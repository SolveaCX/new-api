#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
production="${root}/.github/workflows/gcp-deploy.yml"
staging="${root}/.github/workflows/gcp-deploy-staging.yml"
rollback="${root}/.github/workflows/gcp-rollback.yml"
promotion="${root}/.github/workflows/gcp-promote-supplier-runner.yml"
website="${root}/.github/workflows/gcp-deploy-website.yml"
website_static="${root}/.github/workflows/gcp-deploy-website-static.yml"
website_staging="${root}/.github/workflows/gcp-deploy-website-staging.yml"
infra="${root}/.github/workflows/gcp-infra.yml"
association_test="${root}/.github/scripts/supplier-resource-association-verify-test.sh"

require_pattern() {
  local pattern=$1 file=$2
  grep -Eq -- "${pattern}" "${file}" || {
    echo "missing workflow contract ${pattern} in ${file}" >&2
    exit 1
  }
}

reject_pattern() {
  local pattern=$1 file=$2
  if grep -Eq -- "${pattern}" "${file}"; then
    echo "forbidden workflow contract ${pattern} in ${file}" >&2
    exit 1
  else
    local status=$?
    if [[ "${status}" -ne 1 ]]; then
      echo "invalid workflow contract check ${pattern} in ${file}" >&2
      exit "${status}"
    fi
  fi
}

section() {
  local start=$1 end=$2 file=$3
  if [[ -n "${end}" ]]; then
    sed -n "/^  ${start}:/,/^  ${end}:/p" "${file}"
  else
    sed -n "/^  ${start}:/,$ p" "${file}"
  fi
}

require_auth_pair() {
  local content=$1 provider=$2 account=$3 label=$4
  grep -Fq -- "workload_identity_provider: ${provider}" <<<"${content}" || {
    echo "${label} does not use its fixed WIF provider" >&2
    exit 1
  }
  grep -Fq -- "service_account: ${account}" <<<"${content}" || {
    echo "${label} does not use its fixed service account" >&2
    exit 1
  }
}

builder_provider='projects/528088078482/locations/global/workloadIdentityPools/github-actions/providers/github'
prod_provider='projects/528088078482/locations/global/workloadIdentityPools/github-prod-app-deploy/providers/github'
rollback_provider='projects/528088078482/locations/global/workloadIdentityPools/github-prod-rollback/providers/github'
web_provider='projects/528088078482/locations/global/workloadIdentityPools/github-prod-web-deploy/providers/github'
staging_provider='projects/528088078482/locations/global/workloadIdentityPools/github-staging-deploy/providers/github'
promoter_provider='projects/528088078482/locations/global/workloadIdentityPools/github-supplier-promote/providers/github'
builder_sa='newapi-ci-deployer@vocai-gemini-prod.iam.gserviceaccount.com'

all_workflows=(
  "${production}" "${staging}" "${rollback}" "${promotion}"
  "${website}" "${website_static}" "${website_staging}" "${infra}"
)

for workflow in "${all_workflows[@]}"; do
  reject_pattern 'GCP_DEPLOYER_SA' "${workflow}"
  reject_pattern 'GCP_WIF_PROVIDER' "${workflow}"
done

# The shared github-actions trust lane is build-only. Every mutation lane has a
# fixed, narrower provider and service account.
production_build=$(section build deploy-console "${production}")
production_console=$(section deploy-console deploy-router "${production}")
production_router=$(section deploy-router '' "${production}")
staging_build=$(section build deploy "${staging}")
staging_deploy=$(section deploy '' "${staging}")
require_auth_pair "${production_build}" "${builder_provider}" "${builder_sa}" 'production build'
require_auth_pair "${production_console}" "${prod_provider}" 'newapi-prod-console-deployer@vocai-gemini-prod.iam.gserviceaccount.com' 'production Console deploy'
require_auth_pair "${production_router}" "${prod_provider}" 'newapi-prod-router-deployer@vocai-gemini-prod.iam.gserviceaccount.com' 'production Router deploy'
require_auth_pair "${staging_build}" "${builder_provider}" "${builder_sa}" 'staging build'
require_auth_pair "${staging_deploy}" "${staging_provider}" 'newapi-staging-deployer@vocai-gemini-prod.iam.gserviceaccount.com' 'staging deploy'

for mutation_section in "${production_console}" "${production_router}" "${staging_deploy}"; do
  if grep -Fq -- "${builder_provider}" <<<"${mutation_section}" || grep -Fq -- "${builder_sa}" <<<"${mutation_section}"; then
    echo 'application mutation lane must not use the shared build identity' >&2
    exit 1
  fi
done

require_auth_pair "$(section rollback '' "${rollback}")" "${rollback_provider}" \
  "\${{ inputs.rollback_target == 'router' && 'newapi-prod-router-rollback@vocai-gemini-prod.iam.gserviceaccount.com' || 'newapi-prod-console-rollback@vocai-gemini-prod.iam.gserviceaccount.com' }}" \
  'production rollback'
require_auth_pair "$(section promote-runner '' "${promotion}")" "${promoter_provider}" \
  'newapi-supplier-promoter@vocai-gemini-prod.iam.gserviceaccount.com' 'supplier promotion'

for workflow in "${production}" "${staging}"; do
  require_pattern 'steps\.push\.outputs\.digest' "${workflow}"
  require_pattern 'server@\$\{digest\}' "${workflow}"
  require_pattern 'supplier-deploy-verify\.sh status' "${workflow}"
  require_pattern 'supplier-deploy-verify\.sh oci' "${workflow}"
  require_pattern 'supplier-deploy-verify\.sh binding-verify' "${workflow}"
  require_pattern 'retention-days: 90' "${workflow}"
  reject_pattern '--image=.*image_uri' "${workflow}"
done

for workflow in "${production}" "${staging}" "${rollback}"; do
  reject_pattern 'gcloud run jobs|gcloud scheduler jobs' "${workflow}"
done

grep -q -- '--no-traffic' <<<"${production_console}"
grep -q -- '--to-revisions=.*=100' <<<"${production_console}"
grep -q -- 'SUPPLIER_MANAGEMENT_DRAIN_SECONDS' <<<"${production_console}"
grep -q -- 'drain_seconds >= 3600' <<<"${production_console}"
grep -q -- '/api/supply-chain/accounting/readiness' <<<"${production_console}"
grep -q -- 'supplier-deploy-verify\.sh control-plane-capabilities' <<<"${production_console}"

router_header=$(sed -n '/^  deploy-router:/,/^    runs-on:/p' "${production}")
grep -Fq -- 'needs: [build, deploy-console]' <<<"${router_header}"
grep -Fq -- 'always()' <<<"${router_header}"
grep -Fq -- '!cancelled()' <<<"${router_header}"
grep -Fq -- "needs.build.result == 'success'" <<<"${router_header}"
grep -Fq -- "needs.deploy-console.result == 'success'" <<<"${router_header}"
grep -Fq -- 'APP_CONSOLE_ORIGIN: https://console.flatkey.ai' <<<"${production_router}"
grep -Fq -- 'ROOT_ACCESS_TOKEN: ${{ secrets.SUPPLIER_DEPLOY_ROOT_ACCESS_TOKEN }}' <<<"${production_router}"
grep -Fq -- 'ROOT_USER_ID: ${{ vars.SUPPLIER_DEPLOY_ROOT_USER_ID }}' <<<"${production_router}"
grep -q -- 'supplier-deploy-verify\.sh control-plane-capabilities' <<<"${production_router}"

# Root-token operations are pinned to the production Console control plane.
for workflow in "${production}" "${rollback}" "${promotion}"; do
  require_pattern '^          CONTROL_PLANE_URL: https://console\.flatkey\.ai$' "${workflow}"
  require_pattern 'supplier-deploy-verify\.sh control-plane-capabilities' "${workflow}"
done
for workflow in "${production}" "${rollback}" "${promotion}"; do
  reject_pattern 'CONTROL_PLANE_URL:.*\$\{\{' "${workflow}"
done
require_pattern '^          APP_CONSOLE_ORIGIN: https://console\.flatkey\.ai$' "${production}"
reject_pattern '\$\{(tag_url|current_url)\}/api/supply-chain/' "${production}"

python3 - "${production}" <<'PY'
import re
import sys

text = open(sys.argv[1], encoding="utf-8").read()
logical = re.sub(r"\\\n[ \t]*", " ", text)
allowed_origins = ("${CONTROL_PLANE_URL}", "${CONTROL_PLANE_URL%/}", "${APP_CONSOLE_ORIGIN}", "${APP_CONSOLE_ORIGIN%/}")
for line in logical.splitlines():
    if "ROOT_ACCESS_TOKEN" not in line:
        continue
    if "curl " not in line and "supplier-deploy-verify.sh control-plane-fetch" not in line:
        continue
    if not any(origin in line for origin in allowed_origins):
        raise SystemExit(f"production Root-bearing command is not pinned to the fixed Console origin: {line.strip()}")
PY

# Infra CI is static-only: local backend-disabled init and validate, with no
# OIDC authentication, remote backend, plan, or apply authority.
reject_pattern 'id-token:|google-github-actions/auth|workload_identity_provider:|service_account:' "${infra}"
require_pattern 'terraform init -backend=false -input=false' "${infra}"
reject_pattern '-backend-config|terraform (plan|apply)' "${infra}"
while IFS= read -r init_line; do
  if [[ "${init_line}" != *'terraform init -backend=false -input=false'* ]]; then
    echo "infra workflow contains a backend-capable Terraform init: ${init_line}" >&2
    exit 1
  fi
done < <(grep -E -- 'terraform init' "${infra}")
require_pattern 'supplier-resource-association-verify-test\.sh' "${infra}"
require_pattern 'actions/setup-go@v5' "${infra}"
require_pattern 'go-version-file: go\.mod' "${infra}"
require_pattern 'TestSupplierReleaseManifestCapabilitiesTrackSourceV1' "${infra}"

# The association suite is part of infra validation and must retain its hostile
# alias/body/header/environment coverage.
for fixture in \
  job-malicious-extra-env.json \
  job-malicious-secret-alias.json \
  job-malicious-task-alias.json \
  scheduler-malicious-target.json \
  scheduler-malicious-target-alias.json \
  scheduler-malicious-inner-alias.json \
  scheduler-malicious-body.json \
  scheduler-malicious-headers.json; do
  require_pattern "${fixture}" "${association_test}"
done

# Promotion verifies the fixed Job shape immediately before and after its only
# mutation. Scheduler mutation is deliberately outside the WIF promotion lane.
require_pattern 'name: GCP Promote Supplier Runner' "${promotion}"
require_pattern '^    environment: production$' "${promotion}"
require_pattern '^      JOB_NAME: newapi-supplier-batch$' "${promotion}"
reject_pattern 'inputs\.(job_name|scheduler_name)|production-supplier-runner' "${promotion}"
reject_pattern 'gcloud scheduler' "${promotion}"
require_pattern 'gcloud run jobs update' "${promotion}"
require_pattern 'gcloud run jobs execute' "${promotion}"
association_count=$(grep -c -- 'supplier-resource-association-verify\.sh job' "${promotion}")
[[ "${association_count}" -eq 2 ]] || {
  echo "promotion must run exactly two Job association checks, found ${association_count}" >&2
  exit 1
}
first_association=$(grep -n -- 'supplier-resource-association-verify\.sh job' "${promotion}" | sed -n '1s/:.*//p')
job_update=$(grep -n -- 'gcloud run jobs update' "${promotion}" | sed -n '1s/:.*//p')
second_association=$(grep -n -- 'supplier-resource-association-verify\.sh job' "${promotion}" | sed -n '2s/:.*//p')
if ! (( first_association < job_update && job_update < second_association )); then
  echo 'Job association checks must bracket the promotion update' >&2
  exit 1
fi
require_pattern 'admin_command_ledger_state == "finalized"' "${promotion}"
require_pattern 'mutation\.enabled == false' "${promotion}"
require_pattern 'supplier batch request \[A-Za-z0-9_-' "${promotion}"

# Website build jobs may push images only as the builder; deploy jobs mutate the
# pre-created service only as the dedicated web/staging deployer.
for workflow in "${website}" "${website_static}"; do
  web_build=$(section build deploy "${workflow}")
  web_deploy=$(section deploy '' "${workflow}")
  require_auth_pair "${web_build}" "${builder_provider}" "${builder_sa}" "$(basename "${workflow}") build"
  require_auth_pair "${web_deploy}" "${web_provider}" 'newapi-prod-web-deployer@vocai-gemini-prod.iam.gserviceaccount.com' "$(basename "${workflow}") deploy"
  if grep -Fq -- "${builder_sa}" <<<"${web_deploy}" || grep -Fq -- 'newapi-prod-web-deployer@' <<<"${web_build}"; then
    echo "website build/deploy identities are not separated in ${workflow}" >&2
    exit 1
  fi
done
web_staging_build=$(section build deploy "${website_staging}")
web_staging_deploy=$(section deploy '' "${website_staging}")
require_auth_pair "${web_staging_build}" "${builder_provider}" "${builder_sa}" 'staging website build'
require_auth_pair "${web_staging_deploy}" "${staging_provider}" 'newapi-staging-deployer@vocai-gemini-prod.iam.gserviceaccount.com' 'staging website deploy'
reject_pattern 'newapi-ci-deployer@' <(printf '%s\n' "${web_staging_deploy}")

for source in "${production}" "${staging}" "${root}/Dockerfile"; do
  reject_pattern 'SUPPLIER_(PRODUCER|ADMIN_SCHEMA)_CAPABILITIES' "${source}"
done
require_pattern '"gcp-deploy-build" "1" "1"' "${production}"
require_pattern '"gcp-deploy-staging-build" "1" "1"' "${staging}"
require_pattern 'supplier-deploy-verify\.sh capabilities /tmp/status\.json 1' "${staging}"
require_pattern 'BUILD_JOB_IDENTITY.*"1" "1"' "${root}/Dockerfile"

require_pattern 'actions/download-artifact@v4' "${rollback}"
require_pattern 'activation\.phase == "degraded"' "${rollback}"
require_pattern 'reason_category == "emergency_rollback"' "${rollback}"
require_pattern 'affected_oci_digest == \$digest' "${rollback}"
require_pattern 'admin-schema-capabilities /tmp/status\.json 1' "${rollback}"
reject_pattern 'revision.*(>=|<=|greater|newer|older)|creation.*time.*revision' "${rollback}"

require_pattern 'supplier_batch_runner' "${root}/Dockerfile"
require_pattern 'supplier_admin_finalize' "${root}/Dockerfile"
for artifact in new-api supplier_batch_runner supplier_admin_finalize; do
  if [[ -e "${root}/${artifact}" ]]; then
    echo "repository-root build artifact must not exist: ${artifact}" >&2
    exit 1
  fi
done

echo 'supplier workflow contract tests passed'
