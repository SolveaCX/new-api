#!/usr/bin/env bash
set -euo pipefail

die() {
  echo "$*" >&2
  exit 1
}

require_file() {
  [[ -f "$1" && -r "$1" ]] || die "resource description is missing or unreadable: $1"
}

verify_job() {
  local file=$1 project_id=$2 project_number=$3 region=$4 job_name=$5 runner_sa=$6 console_url=$7 poll_interval=$8
  require_file "${file}"
  jq -e \
    --arg project_id "${project_id}" \
    --arg project_number "${project_number}" \
    --arg region "${region}" \
    --arg job_name "${job_name}" \
    --arg runner_sa "${runner_sa}" \
    --arg console_url "${console_url}" \
    --arg poll_interval "${poll_interval}" '
    def seconds:
      if type == "number" then .
      elif type == "string" and test("^[0-9]+s?$") then sub("s$"; "") | tonumber
      else -1
      end;
    def is_v2: (.name? | type == "string" and startswith("projects/"));
    def identity_ok:
      if is_v2 then
        .name == ("projects/" + $project_id + "/locations/" + $region + "/jobs/" + $job_name)
      else
        .metadata.name == $job_name and
        ((.metadata.namespace | tostring) == $project_id or (.metadata.namespace | tostring) == $project_number) and
        .metadata.labels["cloud.googleapis.com/location"] == $region
      end;
    def job_template:
      if is_v2 then .template else .spec.template.spec end;
    def task_template:
      if is_v2 then .template.template else .spec.template.spec.template.spec end;
    def env_secret_name:
      .valueSource.secretKeyRef.secret //
      .value_source.secret_key_ref.secret //
      .valueFrom.secretKeyRef.name //
      .value_from.secret_key_ref.name // null;
    def env_secret_version:
      .valueSource.secretKeyRef.version //
      .value_source.secret_key_ref.version //
      .valueFrom.secretKeyRef.key //
      .value_from.secret_key_ref.key // null;
    def env_shape_ok($v2):
      if .name == "SUPPLIER_BATCH_CONSOLE_URL" or .name == "SUPPLIER_BATCH_POLL_INTERVAL" then
        keys == ["name", "value"]
      elif .name == "SUPPLIER_BATCH_TOKEN" and $v2 then
        keys == ["name", "valueSource"] and
        (.valueSource | keys == ["secretKeyRef"]) and
        (.valueSource.secretKeyRef | keys == ["secret", "version"])
      elif .name == "SUPPLIER_BATCH_TOKEN" then
        keys == ["name", "valueFrom"] and
        (.valueFrom | keys == ["secretKeyRef"]) and
        (.valueFrom.secretKeyRef | keys == ["key", "name"])
      else false
      end;
    (is_v2) as $v2 |
    (($v2 and (has("spec") | not)) or (($v2 | not) and (has("template") | not))) and
    identity_ok and (
    (job_template) as $job |
    (task_template) as $task |
    ($job.taskCount == 1) and
    ($job.parallelism == 1) and
    ((($v2 and ($task | has("serviceAccount")) and ($task | has("serviceAccountName") | not) and ($task | has("timeout")) and ($task | has("timeoutSeconds") | not)) or
      (($v2 | not) and ($task | has("serviceAccountName")) and ($task | has("serviceAccount") | not) and ($task | has("timeoutSeconds")) and ($task | has("timeout") | not))) and
     ($task | has("service_account") | not) and ($task | has("service_account_name") | not) and
     ($task | has("maxRetries")) and ($task | has("max_retries") | not) and ($task | has("timeout_seconds") | not)) and
    ((if $v2 then $task.serviceAccount else $task.serviceAccountName end) == $runner_sa) and
    ($task.maxRetries == 0) and
    (((if $v2 then $task.timeout else $task.timeoutSeconds end) | seconds) >= 3600) and
    (($task.containers // []) | length == 1) and
    (($task.containers[0]) as $container |
    ($container.image == ($region + "-docker.pkg.dev/" + $project_id + "/newapi/server@" + ($container.image | capture("@(?<digest>sha256:[0-9a-f]{64})$").digest))) and
    ($container.command == ["/app/supplier_batch_runner"]) and
    (($container.args // []) == []) and
    (($container | has("volume_mounts") | not) and ($container | has("security_context") | not)) and
    (($container.resources.limits.cpu // $container.resources.limits["cpu"]) | tostring | . == "1" or . == "1000m") and
    (($container.resources.limits.memory // $container.resources.limits["memory"]) == "512Mi") and
    (($task.volumes // []) == []) and
    (($container.volumeMounts // $container.volume_mounts // []) == []) and
    ((($container.securityContext.capabilities // $container.security_context.capabilities // {}) | length) == 0) and
    (($container.env // []) | length == 3) and
    (all($container.env[]; env_shape_ok($v2))) and
    (($container.env | map(.name) | sort) == ["SUPPLIER_BATCH_CONSOLE_URL", "SUPPLIER_BATCH_POLL_INTERVAL", "SUPPLIER_BATCH_TOKEN"]) and
    (($container.env | map(select(.name == "SUPPLIER_BATCH_CONSOLE_URL")) | length) == 1) and
    (($container.env | map(select(.name == "SUPPLIER_BATCH_CONSOLE_URL" and .value == $console_url and env_secret_name == null)) | length) == 1) and
    (($container.env | map(select(.name == "SUPPLIER_BATCH_POLL_INTERVAL")) | length) == 1) and
    (($container.env | map(select(.name == "SUPPLIER_BATCH_POLL_INTERVAL" and .value == $poll_interval and env_secret_name == null)) | length) == 1) and
    (($container.env | map(select(.name == "SUPPLIER_BATCH_TOKEN")) | length) == 1) and
    (($container.env | map(select(
      .name == "SUPPLIER_BATCH_TOKEN" and
      (.value? == null) and
      (env_secret_name == "newapi-supplier-batch-token-current" or env_secret_name == "newapi-supplier-batch-token-next") and
      env_secret_version == "latest"
    )) | length) == 1))
    )
  ' "${file}" >/dev/null || die "supplier Cloud Run Job association or shape verification failed"
}

verify_scheduler() {
  local file=$1 project_id=$2 region=$3 scheduler_name=$4 job_name=$5 scheduler_sa=$6 expected_state=$7
  require_file "${file}"
  [[ "${expected_state}" == "PAUSED" || "${expected_state}" == "ENABLED" ]] || die "expected Scheduler state must be PAUSED or ENABLED"
  jq -e \
    --arg project_id "${project_id}" \
    --arg region "${region}" \
    --arg scheduler_name "${scheduler_name}" \
    --arg job_name "${job_name}" \
    --arg scheduler_sa "${scheduler_sa}" \
    --arg expected_state "${expected_state}" '
    def seconds:
      if type == "number" then .
      elif type == "string" and test("^[0-9]+s?$") then sub("s$"; "") | tonumber
      else -1
      end;
    (has("httpTarget")) as $camel |
    (($camel and (has("http_target") | not) and has("retryConfig") and (has("retry_config") | not) and has("attemptDeadline") and (has("attempt_deadline") | not) and has("timeZone") and (has("time_zone") | not) and has("state") and (has("status") | not)) or
     (($camel | not) and has("http_target") and has("retry_config") and (has("retryConfig") | not) and has("attempt_deadline") and (has("attemptDeadline") | not) and has("time_zone") and (has("timeZone") | not) and has("status") and (has("state") | not))) and
    (if $camel then .httpTarget else .http_target end) as $target |
    (($camel and ($target | has("oauthToken")) and ($target | has("oauth_token") | not)) or
     (($camel | not) and ($target | has("oauth_token")) and ($target | has("oauthToken") | not))) and
    (if $camel then $target.oauthToken else $target.oauth_token end) as $oauth |
    (if $camel then .retryConfig else .retry_config end) as $retry |
    (($camel and ($target | has("http_method") | not) and ($oauth | has("service_account_email") | not) and ($retry | has("retry_count") | not)) or
     (($camel | not) and ($target | has("httpMethod") | not) and ($oauth | has("serviceAccountEmail") | not) and ($retry | has("retryCount") | not))) and
    ($target.headers // {}) as $headers |
    .name == ("projects/" + $project_id + "/locations/" + $region + "/jobs/" + $scheduler_name) and
    (if $camel then $target.httpMethod else $target.http_method end) == "POST" and
    $target.uri == ("https://run.googleapis.com/v2/projects/" + $project_id + "/locations/" + $region + "/jobs/" + $job_name + ":run") and
    (($target.body // "") == "") and
    ($headers == {} or $headers == {"User-Agent": "Google-Cloud-Scheduler"}) and
    (if $camel then $oauth.serviceAccountEmail else $oauth.service_account_email end) == $scheduler_sa and
    $oauth.scope == "https://www.googleapis.com/auth/cloud-platform" and
    .schedule == "5 2 * * *" and
    (if $camel then .timeZone else .time_zone end) == "Asia/Shanghai" and
    (if $camel then $retry.retryCount else $retry.retry_count end) == 0 and
    (((if $camel then .attemptDeadline else .attempt_deadline end) | seconds) == 30) and
    (if $camel then .state else .status.state end) == $expected_state
  ' "${file}" >/dev/null || die "supplier Cloud Scheduler association or shape verification failed"
}

command=${1:-}
case "${command}" in
  job)
    shift
    [[ $# -eq 8 ]] || die "job requires FILE PROJECT_ID PROJECT_NUMBER REGION JOB_NAME RUNNER_SA CONSOLE_URL POLL_INTERVAL"
    verify_job "$@"
    ;;
  scheduler)
    shift
    [[ $# -eq 7 ]] || die "scheduler requires FILE PROJECT_ID REGION SCHEDULER_NAME JOB_NAME SCHEDULER_SA EXPECTED_STATE"
    verify_scheduler "$@"
    ;;
  *) die "unknown resource-association command: ${command}" ;;
esac
