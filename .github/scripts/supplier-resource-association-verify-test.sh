#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
verify="${root}/.github/scripts/supplier-resource-association-verify.sh"
fixtures="${root}/.github/scripts/fixtures/supplier-resource-association"
tmp=$(mktemp -d)
trap 'rm -rf "${tmp}"' EXIT

job_args=(vocai-gemini-prod 528088078482 us-west1 newapi-supplier-batch newapi-supplier-batch-runner@vocai-gemini-prod.iam.gserviceaccount.com https://console.flatkey.ai 5s)
scheduler_args=(vocai-gemini-prod us-west1 newapi-supplier-batch-daily newapi-supplier-batch newapi-supplier-scheduler@vocai-gemini-prod.iam.gserviceaccount.com)

"${verify}" job "${fixtures}/job-v1-valid.json" "${job_args[@]}"
"${verify}" job "${fixtures}/job-v2-valid.json" "${job_args[@]}"
"${verify}" scheduler "${fixtures}/scheduler-camel-valid.json" "${scheduler_args[@]}" PAUSED
"${verify}" scheduler "${fixtures}/scheduler-snake-valid.json" "${scheduler_args[@]}" ENABLED

assert_job_rejected() {
  local name=$1 file=$2
  if "${verify}" job "${file}" "${job_args[@]}" 2>/dev/null; then
    echo "malicious Job fixture unexpectedly passed: ${name}" >&2
    exit 1
  fi
}

assert_scheduler_rejected() {
  local name=$1 file=$2 state=${3:-PAUSED}
  if "${verify}" scheduler "${file}" "${scheduler_args[@]}" "${state}" 2>/dev/null; then
    echo "malicious Scheduler fixture unexpectedly passed: ${name}" >&2
    exit 1
  fi
}

assert_job_rejected extra-env "${fixtures}/job-malicious-extra-env.json"
assert_job_rejected secret-alias "${fixtures}/job-malicious-secret-alias.json"
assert_job_rejected task-alias "${fixtures}/job-malicious-task-alias.json"
assert_scheduler_rejected wrong-target "${fixtures}/scheduler-malicious-target.json"
assert_scheduler_rejected target-alias "${fixtures}/scheduler-malicious-target-alias.json"
assert_scheduler_rejected inner-alias "${fixtures}/scheduler-malicious-inner-alias.json"
assert_scheduler_rejected override-body "${fixtures}/scheduler-malicious-body.json"
assert_scheduler_rejected attacker-headers "${fixtures}/scheduler-malicious-headers.json"

jq '.name = "projects/other/locations/us-west1/jobs/newapi-supplier-batch"' "${fixtures}/job-v2-valid.json" > "${tmp}/job-project.json"
assert_job_rejected project "${tmp}/job-project.json"
jq '.template.taskCount = 2' "${fixtures}/job-v2-valid.json" > "${tmp}/job-task-count.json"
assert_job_rejected task-count "${tmp}/job-task-count.json"
jq '.template.template.containers += [.template.template.containers[0]]' "${fixtures}/job-v2-valid.json" > "${tmp}/job-containers.json"
assert_job_rejected containers "${tmp}/job-containers.json"
jq '.template.template.containers[0].args = ["--override"]' "${fixtures}/job-v2-valid.json" > "${tmp}/job-args.json"
assert_job_rejected args "${tmp}/job-args.json"
jq '.template.template.containers[0].volumeMounts = [{"name":"escape","mountPath":"/escape"}]' "${fixtures}/job-v2-valid.json" > "${tmp}/job-volume.json"
assert_job_rejected volume "${tmp}/job-volume.json"
jq '.template.template.containers[0].securityContext.capabilities.add = ["SYS_ADMIN"]' "${fixtures}/job-v2-valid.json" > "${tmp}/job-capability.json"
assert_job_rejected capability "${tmp}/job-capability.json"
jq '.template.template.timeout = "3599s"' "${fixtures}/job-v2-valid.json" > "${tmp}/job-timeout.json"
assert_job_rejected timeout "${tmp}/job-timeout.json"
jq '.template.template.containers[0].resources.limits.memory = "1Gi"' "${fixtures}/job-v2-valid.json" > "${tmp}/job-memory.json"
assert_job_rejected memory "${tmp}/job-memory.json"
jq '.template.template.serviceAccount = "attacker@vocai-gemini-prod.iam.gserviceaccount.com"' "${fixtures}/job-v2-valid.json" > "${tmp}/job-service-account.json"
assert_job_rejected service-account "${tmp}/job-service-account.json"

jq '.schedule = "* * * * *"' "${fixtures}/scheduler-camel-valid.json" > "${tmp}/scheduler-cron.json"
assert_scheduler_rejected cron "${tmp}/scheduler-cron.json"
jq '.httpTarget.oauthToken.scope = "https://www.googleapis.com/auth/userinfo.email"' "${fixtures}/scheduler-camel-valid.json" > "${tmp}/scheduler-scope.json"
assert_scheduler_rejected scope "${tmp}/scheduler-scope.json"
jq '.retryConfig.retryCount = 1' "${fixtures}/scheduler-camel-valid.json" > "${tmp}/scheduler-retry.json"
assert_scheduler_rejected retry "${tmp}/scheduler-retry.json"
jq '.state = "ENABLED"' "${fixtures}/scheduler-camel-valid.json" > "${tmp}/scheduler-state.json"
assert_scheduler_rejected state "${tmp}/scheduler-state.json" PAUSED

echo "supplier resource association verification tests passed"
