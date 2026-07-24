#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "${tmp}"' EXIT
verify_script="${root}/.github/scripts/supplier-deploy-verify.sh"

"${root}/.github/scripts/supplier-build-manifest.sh" "${tmp}/first" SolveaCX/new-api abc123 100 2 gcp-deploy-build 3,1,2 1
"${root}/.github/scripts/supplier-build-manifest.sh" "${tmp}/second" SolveaCX/new-api abc123 100 2 gcp-deploy-build 3,1,2 1
cmp "${tmp}/first/manifest_hash_payload.json" "${tmp}/second/manifest_hash_payload.json"
cmp "${tmp}/first/manifest_sha256.txt" "${tmp}/second/manifest_sha256.txt"

"${root}/.github/scripts/supplier-build-manifest.sh" "${tmp}/other" SolveaCX/new-api abc123 100 3 gcp-deploy-build 1,2,3 1
if cmp -s "${tmp}/first/manifest_sha256.txt" "${tmp}/other/manifest_sha256.txt"; then
  echo "different attempts must produce different manifests" >&2
  exit 1
fi

if "${root}/.github/scripts/supplier-build-manifest.sh" "${tmp}/missing-admin-arg" SolveaCX/new-api abc123 100 2 gcp-deploy-build 1 2>/dev/null; then
  echo "seven-argument manifest invocation unexpectedly passed" >&2
  exit 1
fi

if "${root}/.github/scripts/supplier-build-manifest.sh" "${tmp}/missing-current-admin" SolveaCX/new-api abc123 100 2 gcp-deploy-build 1 2 2>/dev/null; then
  echo "manifest without current admin schema capability 1 unexpectedly passed" >&2
  exit 1
fi
if "${root}/.github/scripts/supplier-build-manifest.sh" "${tmp}/noncanonical-admin" SolveaCX/new-api abc123 100 2 gcp-deploy-build 1 01 2>/dev/null; then
  echo "non-canonical build capability representation unexpectedly passed" >&2
  exit 1
fi

payload=$(cat "${tmp}/first/manifest_hash_payload.json")
sha=$(cat "${tmp}/first/manifest_sha256.txt")
jq -n --arg payload "${payload}" --arg sha "${sha}" --slurpfile manifest "${tmp}/first/manifest_hash_payload.json" \
  '{success:true,data:{build_manifest:($manifest[0] + {manifest_hash_payload:$payload,manifest_sha256:$sha})}}' > "${tmp}/status.json"
/bin/bash "${verify_script}" manifest "${tmp}/first/manifest_hash_payload.json" "${tmp}/first/manifest_sha256.txt"
/bin/bash "${verify_script}" status "${tmp}/first/manifest_hash_payload.json" "${tmp}/first/manifest_sha256.txt" "${tmp}/status.json"
/bin/bash "${verify_script}" capabilities "${tmp}/status.json" 1,2,3
/bin/bash "${verify_script}" admin-schema-capabilities "${tmp}/status.json" 1

jq '.success = false' "${tmp}/status.json" > "${tmp}/unsuccessful-status.json"
if /bin/bash "${verify_script}" status \
  "${tmp}/first/manifest_hash_payload.json" "${tmp}/first/manifest_sha256.txt" "${tmp}/unsuccessful-status.json" 2>/dev/null; then
  echo "unsuccessful status response unexpectedly passed" >&2
  exit 1
fi

jq '.data.build_manifest.supplier_admin_schema_capabilities = [2]' "${tmp}/status.json" > "${tmp}/expanded-mismatch.json"
if /bin/bash "${verify_script}" admin-schema-capabilities "${tmp}/expanded-mismatch.json" 1,2 2>/dev/null; then
  echo "expanded manifest differing from the canonical payload unexpectedly passed" >&2
  exit 1
fi

sed 's/"supplier_admin_schema_capabilities":\[1\]/"supplier_admin_schema_capabilities":[1.0]/' \
  "${tmp}/first/manifest_hash_payload.json" > "${tmp}/noncanonical-payload.json"
printf '%s' "$(sha256sum "${tmp}/noncanonical-payload.json" | awk '{print $1}')" > "${tmp}/noncanonical-sha.txt"
if /bin/bash "${verify_script}" manifest "${tmp}/noncanonical-payload.json" "${tmp}/noncanonical-sha.txt" 2>/dev/null; then
  echo "numerically equal but non-canonical capability representation unexpectedly passed" >&2
  exit 1
fi

if /bin/bash "${verify_script}" capabilities "${tmp}/status.json" 01 2>/dev/null; then
  echo "non-canonical accepted capability representation unexpectedly passed" >&2
  exit 1
fi

oci="sha256:$(printf 'a%.0s' {1..64})"
/bin/bash "${verify_script}" binding-write "${oci}" "${sha}" "${tmp}/binding.json"
/bin/bash "${verify_script}" binding-verify "${tmp}/binding.json" "${oci}" "${sha}"
/bin/bash "${verify_script}" oci "${oci}" "${oci}"

jq '.manifest_sha256 = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"' "${tmp}/binding.json" > "${tmp}/tampered.json"
if /bin/bash "${verify_script}" binding-verify "${tmp}/tampered.json" "${oci}" "${sha}" 2>/dev/null; then
  echo "tampered binding unexpectedly passed" >&2
  exit 1
fi

"${root}/.github/scripts/supplier-build-manifest.sh" "${tmp}/v1" SolveaCX/new-api abc123 100 2 gcp-deploy-build 1 1
v1_payload=$(cat "${tmp}/v1/manifest_hash_payload.json")
v1_sha=$(cat "${tmp}/v1/manifest_sha256.txt")
jq -n --arg payload "${v1_payload}" --arg sha "${v1_sha}" --slurpfile manifest "${tmp}/v1/manifest_hash_payload.json" \
  '{success:true,data:{build_manifest:($manifest[0] + {manifest_hash_payload:$payload,manifest_sha256:$sha})}}' > "${tmp}/v1-status.json"

verify_control_plane_passes() {
  local name=$1 phase=$2 accepted=$3
  jq -n --arg phase "${phase}" --argjson accepted "${accepted}" \
    '{success:true,data:{activation:{phase:$phase,accepted_capability_versions:$accepted}}}' > "${tmp}/${name}.json"
  /bin/bash "${verify_script}" control-plane-capabilities \
    "${tmp}/v1-status.json" "${tmp}/${name}.json" 1 1
}

verify_control_plane_fails() {
  local name=$1 fixture=$2
  printf '%s\n' "${fixture}" > "${tmp}/${name}.json"
  if /bin/bash "${verify_script}" control-plane-capabilities \
    "${tmp}/v1-status.json" "${tmp}/${name}.json" 1 1 2>/dev/null; then
    echo "${name} control-plane fixture unexpectedly passed" >&2
    exit 1
  fi
}

verify_control_plane_passes disabled-bootstrap disabled '[]'
verify_control_plane_fails disabled-missing '{"success":true,"data":{"activation":{"phase":"disabled"}}}'
verify_control_plane_fails disabled-nonempty '{"success":true,"data":{"activation":{"phase":"disabled","accepted_capability_versions":[1]}}}'
verify_control_plane_fails disabled-malformed '{"success":true,"data":{"activation":{"phase":"disabled","accepted_capability_versions":""}}}'
verify_control_plane_passes shadow shadow '[1]'
verify_control_plane_passes armed armed '[1]'
verify_control_plane_passes active active '[1]'
verify_control_plane_passes degraded degraded '[1,2]'
printf '%s\n' '{"success":true,"data":{"activation":{"phase":"disabled","accepted_capability_versions":[1,2]}}}' > "${tmp}/disabled-expanded-bootstrap.json"
if /bin/bash "${verify_script}" control-plane-capabilities \
  "${tmp}/status.json" "${tmp}/disabled-expanded-bootstrap.json" 1 1 2>/dev/null; then
  echo "disabled bootstrap accepted a non-V1 producer manifest" >&2
  exit 1
fi
verify_control_plane_fails active-mismatch '{"success":true,"data":{"activation":{"phase":"active","accepted_capability_versions":[2]}}}'
verify_control_plane_fails active-duplicate '{"success":true,"data":{"activation":{"phase":"active","accepted_capability_versions":[1,1]}}}'
verify_control_plane_fails active-unsorted '{"success":true,"data":{"activation":{"phase":"active","accepted_capability_versions":[2,1]}}}'
verify_control_plane_fails active-empty '{"success":true,"data":{"activation":{"phase":"active","accepted_capability_versions":[]}}}'
verify_control_plane_fails shadow-empty '{"success":true,"data":{"activation":{"phase":"shadow","accepted_capability_versions":[]}}}'
verify_control_plane_fails shadow-missing '{"success":true,"data":{"activation":{"phase":"shadow"}}}'
verify_control_plane_fails active-missing '{"success":true,"data":{"activation":{"phase":"active"}}}'
verify_control_plane_fails active-malformed '{"success":true,"data":{"activation":{"phase":"active","accepted_capability_versions":[1,"2"]}}}'
verify_control_plane_fails unsuccessful-response '{"success":false,"data":{"activation":{"phase":"active","accepted_capability_versions":[1]}}}'
verify_control_plane_fails unknown-phase '{"success":true,"data":{"activation":{"phase":"retired","accepted_capability_versions":[1]}}}'
verify_control_plane_fails missing-phase '{"success":true,"data":{"activation":{"accepted_capability_versions":[1]}}}'

mkdir -p "${tmp}/bin"
cat > "${tmp}/bin/curl" <<'MOCK_CURL'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" > "${MOCK_CURL_ARGS}"
printf '%s\n' '{"success":true,"data":{"activation":{"phase":"active","accepted_capability_versions":[1]}}}'
MOCK_CURL
chmod +x "${tmp}/bin/curl"
MOCK_CURL_ARGS="${tmp}/curl-args.txt" PATH="${tmp}/bin:${PATH}" \
  /bin/bash "${verify_script}" control-plane-fetch \
  https://console.example.test root-token 123 "${tmp}/fetched-accounting.json"
grep -Fxq 'Authorization: root-token' "${tmp}/curl-args.txt"
grep -Fxq 'New-Api-User: 123' "${tmp}/curl-args.txt"
grep -Fxq 'https://console.example.test/api/supply-chain/accounting/status' "${tmp}/curl-args.txt"
if PATH="${tmp}/bin:${PATH}" /bin/bash "${verify_script}" control-plane-fetch \
  https://console.example.test '' 123 "${tmp}/missing-token.json" 2>/dev/null; then
  echo "control-plane fetch without credentials unexpectedly passed" >&2
  exit 1
fi
