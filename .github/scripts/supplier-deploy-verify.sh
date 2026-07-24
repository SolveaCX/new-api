#!/usr/bin/env bash
set -euo pipefail

die() {
  echo "$*" >&2
  exit 1
}

require_file() {
  [[ -f "$1" && -r "$1" ]] || die "required evidence is missing or unreadable: $1"
}

validate_sha256() {
  [[ "$1" =~ ^[a-f0-9]{64}$ ]] || die "invalid lowercase SHA-256: $1"
}

validate_oci_digest() {
  [[ "$1" =~ ^sha256:[a-f0-9]{64}$ ]] || die "invalid OCI digest: $1"
}

verify_manifest_files() {
  local payload_file=$1 sha_file=$2 payload expected actual canonical
  require_file "${payload_file}"
  require_file "${sha_file}"
  payload=$(cat "${payload_file}")
  expected=$(tr -d '\r\n' < "${sha_file}")
  validate_sha256 "${expected}"
  actual=$(printf '%s' "${payload}" | sha256sum | awk '{print $1}')
  [[ "${actual}" == "${expected}" ]] || die "build manifest payload SHA-256 mismatch"
  jq -e '
    type == "object" and
    (keys == ["build_commit","build_job_identity","build_provenance_id","github_run_id","producer_capabilities","repo","run_attempt","schema_version","supplier_admin_schema_capabilities"]) and
    .schema_version == 1 and
    (.repo | type == "string" and length > 0) and
    (.build_commit | type == "string" and length > 0) and
    (.github_run_id | type == "string" and length > 0) and
    (.run_attempt | type == "string" and length > 0) and
    (.build_job_identity | type == "string" and length > 0) and
    (.producer_capabilities | type == "array" and length > 0 and all(.[]; type == "number" and . > 0 and floor == .)) and
    (.producer_capabilities == (.producer_capabilities | sort | unique)) and
    (.supplier_admin_schema_capabilities | type == "array" and length > 0 and all(.[]; type == "number" and . > 0 and floor == .)) and
    (.supplier_admin_schema_capabilities == (.supplier_admin_schema_capabilities | sort | unique)) and
    (.supplier_admin_schema_capabilities | index(1) != null) and
    .build_provenance_id == (.repo + "@" + .github_run_id + "." + .run_attempt + ":" + .build_job_identity)
  ' "${payload_file}" >/dev/null || die "build manifest payload is invalid"
  canonical=$(jq -c '{schema_version,repo,build_commit,github_run_id,run_attempt,build_job_identity,producer_capabilities,supplier_admin_schema_capabilities,build_provenance_id}' "${payload_file}")
  [[ "${payload}" == "${canonical}" ]] || die "build manifest payload is not in the exact canonical byte representation"
  grep -Eq '"producer_capabilities":\[[1-9][0-9]*(,[1-9][0-9]*)*\],"supplier_admin_schema_capabilities":\[[1-9][0-9]*(,[1-9][0-9]*)*\]' "${payload_file}" ||
    die "capability arrays must use canonical decimal integer representations"
}

verify_embedded_status_manifest() {
  local status_file=$1 payload sha temp expanded canonical_expanded
  require_file "${status_file}"
  jq -e '.success == true' "${status_file}" >/dev/null || die "status response is unsuccessful"
  payload=$(jq -er '.data.build_manifest.manifest_hash_payload' "${status_file}") || die "status manifest payload is missing"
  sha=$(jq -er '.data.build_manifest.manifest_sha256' "${status_file}") || die "status manifest SHA-256 is missing"
  temp=$(mktemp -d)
  printf '%s' "${payload}" > "${temp}/payload.json"
  printf '%s\n' "${sha}" > "${temp}/sha.txt"
  if ! verify_manifest_files "${temp}/payload.json" "${temp}/sha.txt"; then
    rm -rf "${temp}"
    return 1
  fi
  expanded=$(jq -S -c '.data.build_manifest | del(.manifest_hash_payload,.manifest_sha256)' "${status_file}")
  canonical_expanded=$(jq -S -c '.' "${temp}/payload.json")
  rm -rf "${temp}"
  [[ "${expanded}" == "${canonical_expanded}" ]] || die "expanded status manifest must exactly equal the verified manifest_hash_payload object"
  jq -e '.data.build_manifest | has("oci_digest") | not' "${status_file}" >/dev/null || die "status must not contain an OCI digest"
}

verify_status() {
  local payload_file=$1 sha_file=$2 status_file=$3 expected_payload expected_sha status_payload status_sha
  verify_manifest_files "${payload_file}" "${sha_file}"
  require_file "${status_file}"
  expected_payload=$(cat "${payload_file}")
  expected_sha=$(tr -d '\r\n' < "${sha_file}")
  status_payload=$(jq -er '.data.build_manifest.manifest_hash_payload' "${status_file}") || die "status manifest payload is missing"
  status_sha=$(jq -er '.data.build_manifest.manifest_sha256' "${status_file}") || die "status manifest SHA-256 is missing"
  [[ "${status_payload}" == "${expected_payload}" ]] || die "running revision manifest payload differs from build artifact"
  [[ "${status_sha}" == "${expected_sha}" ]] || die "running revision manifest SHA-256 differs from build artifact"
  verify_embedded_status_manifest "${status_file}"
}

verify_capabilities() {
  local status_file=$1 accepted_csv=$2 field=$3 normalized accepted_json
  require_file "${status_file}"
  [[ "${accepted_csv}" =~ ^[1-9][0-9]*(,[1-9][0-9]*)*$ ]] || die "accepted capabilities must be a numeric CSV"
  normalized=$(printf '%s' "${accepted_csv}" | tr ',' '\n' | sort -n -u | paste -sd, -)
  [[ "${accepted_csv}" == "${normalized}" ]] || die "accepted capabilities must be sorted unique canonical positive integers"
  case "${field}" in
    producer) field=producer_capabilities ;;
    admin-schema) field=supplier_admin_schema_capabilities ;;
    *) die "unknown capability class: ${field}" ;;
  esac
  verify_embedded_status_manifest "${status_file}"
  accepted_json="[${accepted_csv}]"
  jq -e --arg field "${field}" --argjson accepted "${accepted_json}" '
    (.data.build_manifest.manifest_hash_payload | fromjson)[$field] as $caps |
    ($caps | type == "array" and length > 0) and all($caps[]; . as $cap | $accepted | index($cap) != null)
  ' "${status_file}" >/dev/null || die "running revision has an unaccepted numeric ${field} capability"
}

fetch_control_plane_status() {
  local console_origin=$1 access_token=$2 user_id=$3 output_file=$4 temp_file
  [[ "${console_origin}" =~ ^https://[^[:space:]]+$ ]] || die "Console origin must be a non-empty HTTPS URL"
  [[ -n "${access_token}" ]] || die "supplier deploy root access token is required"
  [[ "${user_id}" =~ ^[1-9][0-9]*$ ]] || die "supplier deploy root user ID must be a positive integer"
  mkdir -p "$(dirname "${output_file}")"
  temp_file=$(mktemp "${output_file}.tmp.XXXXXX")
  if ! curl --fail --silent --show-error \
    -H "Authorization: ${access_token}" \
    -H "New-Api-User: ${user_id}" \
    "${console_origin%/}/api/supply-chain/accounting/status" > "${temp_file}"; then
    rm -f "${temp_file}"
    die "authenticated Console accounting status request failed"
  fi
  jq -e '.success == true and (.data.activation | type == "object")' "${temp_file}" >/dev/null || {
    rm -f "${temp_file}"
    die "authenticated Console accounting status response is malformed"
  }
  mv "${temp_file}" "${output_file}"
}

verify_control_plane_capabilities() {
  local status_file=$1 accounting_file=$2 current_producer_csv=$3 current_admin_schema_csv=$4 phase accepted_csv
  require_file "${accounting_file}"
  jq -e '.success == true and (.data.activation | type == "object")' "${accounting_file}" >/dev/null ||
    die "control-plane accounting status response is unsuccessful or malformed"
  phase=$(jq -er '.data.activation.phase | select(type == "string" and length > 0)' "${accounting_file}") ||
    die "control-plane activation phase is missing or malformed"
  case "${phase}" in
    disabled)
      jq -e '.data.activation.accepted_capability_versions == []' "${accounting_file}" >/dev/null ||
        die "disabled control-plane state must declare an exact empty accepted capability set"
      accepted_csv="${current_producer_csv}"
      ;;
    shadow|armed|active|degraded)
      accepted_csv=$(jq -er '
        .data.activation.accepted_capability_versions as $caps |
        if ($caps | type) != "array" or ($caps | length) == 0 or
           (any($caps[]; type != "number" or floor != . or . <= 0)) or
           $caps != ($caps | sort | unique)
        then error("accepted capability versions are missing or malformed")
        else ($caps | map(tostring) | join(","))
        end
      ' "${accounting_file}") || die "accepted capability versions must be non-empty sorted unique positive integers"
      [[ -n "${accepted_csv}" ]] || die "accepted capability versions are empty"
      ;;
    *) die "unknown control-plane activation phase: ${phase}" ;;
  esac
  verify_capabilities "${status_file}" "${accepted_csv}" producer
  verify_capabilities "${status_file}" "${current_admin_schema_csv}" admin-schema
}

write_binding() {
  local oci_digest=$1 manifest_sha=$2 output=$3
  validate_oci_digest "${oci_digest}"
  validate_sha256 "${manifest_sha}"
  mkdir -p "$(dirname "${output}")"
  printf '{"oci_digest":"%s","manifest_sha256":"%s"}\n' "${oci_digest}" "${manifest_sha}" > "${output}"
}

verify_binding() {
  local binding_file=$1 expected_oci=$2 expected_manifest_sha=$3
  require_file "${binding_file}"
  validate_oci_digest "${expected_oci}"
  validate_sha256 "${expected_manifest_sha}"
  jq -e --arg oci "${expected_oci}" --arg manifest "${expected_manifest_sha}" '
    type == "object" and keys == ["manifest_sha256","oci_digest"] and
    .oci_digest == $oci and .manifest_sha256 == $manifest
  ' "${binding_file}" >/dev/null || die "deployment binding is missing, unreadable, or tampered"
}

verify_oci() {
  local expected=$1 deployed=$2
  validate_oci_digest "${expected}"
  validate_oci_digest "${deployed}"
  [[ "${expected}" == "${deployed}" ]] || die "Artifact Registry digest differs from the Cloud Run deployed digest"
}

command=${1:-}
case "${command}" in
  manifest) shift; [[ $# -eq 2 ]] || die "manifest requires PAYLOAD_FILE SHA_FILE"; verify_manifest_files "$@" ;;
  status) shift; [[ $# -eq 3 ]] || die "status requires PAYLOAD_FILE SHA_FILE STATUS_FILE"; verify_status "$@" ;;
  capabilities) shift; [[ $# -eq 2 ]] || die "capabilities requires STATUS_FILE ACCEPTED_CSV"; verify_capabilities "$@" producer ;;
  admin-schema-capabilities) shift; [[ $# -eq 2 ]] || die "admin-schema-capabilities requires STATUS_FILE ACCEPTED_CSV"; verify_capabilities "$@" admin-schema ;;
  control-plane-fetch) shift; [[ $# -eq 4 ]] || die "control-plane-fetch requires CONSOLE_ORIGIN ACCESS_TOKEN USER_ID OUTPUT_FILE"; fetch_control_plane_status "$@" ;;
  control-plane-capabilities) shift; [[ $# -eq 4 ]] || die "control-plane-capabilities requires STATUS_FILE ACCOUNTING_FILE CURRENT_PRODUCER_CSV CURRENT_ADMIN_SCHEMA_CSV"; verify_control_plane_capabilities "$@" ;;
  binding-write) shift; [[ $# -eq 3 ]] || die "binding-write requires OCI_DIGEST MANIFEST_SHA OUTPUT"; write_binding "$@" ;;
  binding-verify) shift; [[ $# -eq 3 ]] || die "binding-verify requires BINDING_FILE OCI_DIGEST MANIFEST_SHA"; verify_binding "$@" ;;
  oci) shift; [[ $# -eq 2 ]] || die "oci requires EXPECTED_DIGEST DEPLOYED_DIGEST"; verify_oci "$@" ;;
  *) die "unknown verification command: ${command}" ;;
esac
