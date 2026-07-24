#!/bin/sh
set -eu

if [ "$#" -ne 8 ]; then
  echo "usage: $0 OUTPUT_DIR REPO COMMIT RUN_ID RUN_ATTEMPT JOB_IDENTITY PRODUCER_CAPABILITIES_CSV ADMIN_SCHEMA_CAPABILITIES_CSV" >&2
  exit 2
fi

output_dir=$1
repo=$2
commit=$3
run_id=$4
run_attempt=$5
job_identity=$6
capabilities_csv=$7
admin_schema_capabilities_csv=$8

validate_text() {
  field=$1
  text=$2
  case "${text}" in
    ''|*[!A-Za-z0-9._/-]*) echo "invalid ${field}" >&2; exit 1 ;;
  esac
  [ "${#text}" -le 160 ] || { echo "invalid ${field}" >&2; exit 1; }
}

validate_text repo "${repo}"
validate_text build_commit "${commit}"
validate_text github_run_id "${run_id}"
validate_text run_attempt "${run_attempt}"
validate_text build_job_identity "${job_identity}"

normalize_capabilities() {
  field=$1
  raw_csv=$2
  raw=$(printf '%s' "${raw_csv}" | tr ',' '\n')
  [ -n "${raw}" ] || { echo "${field} are required" >&2; exit 1; }
  printf '%s\n' "${raw}" | while IFS= read -r capability; do
    case "${capability}" in
      ''|0|0[0-9]*|*[!0-9]*) echo "${field} must use canonical positive integers" >&2; exit 1 ;;
    esac
  done
  sorted=$(printf '%s\n' "${raw}" | sort -n -u)
  raw_count=$(printf '%s\n' "${raw}" | awk 'NF { count++ } END { print count+0 }')
  sorted_count=$(printf '%s\n' "${sorted}" | awk 'NF { count++ } END { print count+0 }')
  if [ "${raw_count}" -ne "${sorted_count}" ]; then
    echo "${field} must be unique" >&2
    exit 1
  fi
  printf '%s\n' "${sorted}" | paste -sd, -
}

capabilities_json=$(normalize_capabilities "producer capabilities" "${capabilities_csv}")
admin_schema_capabilities_json=$(normalize_capabilities "supplier admin schema capabilities" "${admin_schema_capabilities_csv}")
case ",${admin_schema_capabilities_json}," in
  *,1,*) ;;
  *) echo "supplier admin schema capabilities must include current capability 1" >&2; exit 1 ;;
esac
build_provenance_id="${repo}@${run_id}.${run_attempt}:${job_identity}"
payload=$(printf '{"schema_version":1,"repo":"%s","build_commit":"%s","github_run_id":"%s","run_attempt":"%s","build_job_identity":"%s","producer_capabilities":[%s],"supplier_admin_schema_capabilities":[%s],"build_provenance_id":"%s"}' \
  "${repo}" "${commit}" "${run_id}" "${run_attempt}" "${job_identity}" "${capabilities_json}" "${admin_schema_capabilities_json}" "${build_provenance_id}")
manifest_sha256=$(printf '%s' "${payload}" | sha256sum | awk '{print $1}')

mkdir -p "${output_dir}"
printf '%s' "${payload}" > "${output_dir}/manifest_hash_payload.json"
printf '%s\n' "${manifest_sha256}" > "${output_dir}/manifest_sha256.txt"
printf 'BUILD_MANIFEST_SHA256=%s\nBUILD_PROVENANCE_ID=%s\n' "${manifest_sha256}" "${build_provenance_id}" > "${output_dir}/manifest.env"
