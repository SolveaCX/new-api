#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
module="${root}/deploy/gcp/modules/supplier-batch-job"
github_wif="${root}/deploy/gcp/modules/github-wif"
service_accounts="${root}/deploy/gcp/modules/service-accounts"
monitoring="${root}/deploy/gcp/modules/monitoring/main.tf"
monitoring_variables="${root}/deploy/gcp/modules/monitoring/variables.tf"
prod="${root}/deploy/gcp/envs/prod"
infrastructure_doc="${root}/deploy/gcp/docs/INFRASTRUCTURE.md"
deployment_doc="${root}/deploy/gcp/docs/DEPLOYMENT.md"
operations_doc="${root}/docs/supplier-accounting-operations.md"

require_pattern() {
  local pattern=$1 file=$2
  grep -Eq -- "${pattern}" "${file}" || { echo "missing Terraform contract ${pattern} in ${file}" >&2; exit 1; }
}

forbid_pattern() {
  local pattern=$1 file=$2
  if grep -Eq -- "${pattern}" "${file}"; then
    echo "forbidden Terraform contract ${pattern} in ${file}" >&2
    exit 1
  fi
}

require_pattern_count() {
  local expected=$1 pattern=$2 file=$3 actual
  actual=$(grep -Ec -- "${pattern}" "${file}" || true)
  test "${actual}" -eq "${expected}" || { echo "expected ${expected} matches for ${pattern} in ${file}, got ${actual}" >&2; exit 1; }
}

resource_block() {
  local resource_type=$1 resource_name=$2 file=$3
  awk -v start="resource \"${resource_type}\" \"${resource_name}\" {" '
    $0 == start { active = 1 }
    active && /^resource "/ && $0 != start { exit }
    active { print }
  ' "${file}"
}

require_resource_pattern() {
  local resource_type=$1 resource_name=$2 pattern=$3 file=$4 block
  block=$(resource_block "${resource_type}" "${resource_name}" "${file}")
  test -n "${block}" || { echo "missing Terraform resource ${resource_type}.${resource_name} in ${file}" >&2; exit 1; }
  grep -Eq -- "${pattern}" <<<"${block}" || { echo "missing ${pattern} in ${resource_type}.${resource_name}" >&2; exit 1; }
}

forbid_resource_pattern() {
  local resource_type=$1 resource_name=$2 pattern=$3 file=$4 block
  block=$(resource_block "${resource_type}" "${resource_name}" "${file}")
  if grep -Eq -- "${pattern}" <<<"${block}"; then
    echo "forbidden ${pattern} in ${resource_type}.${resource_name}" >&2
    exit 1
  fi
}

require_resource_count() {
  local resource_type=$1 resource_name=$2 expected=$3 pattern=$4 file=$5 block actual
  block=$(resource_block "${resource_type}" "${resource_name}" "${file}")
  actual=$(grep -Ec -- "${pattern}" <<<"${block}" || true)
  test "${actual}" -eq "${expected}" || { echo "expected ${expected} matches for ${pattern} in ${resource_type}.${resource_name}, got ${actual}" >&2; exit 1; }
}

assignment_block() {
  local assignment=$1 file=$2
  awk -v assignment="${assignment}" '
    index($0, assignment " = {") { active = 1 }
    active {
      print
      opens = gsub(/{/, "{")
      closes = gsub(/}/, "}")
      depth += opens - closes
      if (depth == 0) exit
    }
  ' "${file}"
}

require_exact_permissions() {
  local resource_name=$1 expected=$2 file=$3 actual
  actual=$(resource_block google_project_iam_custom_role "${resource_name}" "${file}" |
    sed -n '/permissions = \[/,/^  \]/p' | grep -Eo '"[^"]+"' | tr -d '"' | sort)
  if [[ "${actual}" != "${expected}" ]]; then
    echo "${resource_name} custom role permissions are not the exact approved minimum" >&2
    diff -u <(printf '%s\n' "${expected}") <(printf '%s\n' "${actual}") || true
    exit 1
  fi
}

require_pattern 'count = var\.enabled \? 1 : 0' "${module}/main.tf"
require_pattern 'var\.job_timeout_seconds >= 3600' "${module}/variables.tf"
require_pattern 'image@sha256' "${module}/main.tf"
require_pattern '^supplier_batch_job_enabled = false$' "${prod}/terraform.tfvars"
require_pattern 'supplier_batch_monitoring_enabled = var\.supplier_batch_job_enabled' "${prod}/main.tf"
require_resource_pattern google_cloud_scheduler_job daily 'paused[[:space:]]+= true' "${module}/main.tf"
require_resource_pattern google_cloud_scheduler_job daily 'ignore_changes = \[paused\]' "${module}/main.tf"

require_resource_pattern google_service_account deployer 'account_id[[:space:]]+= "newapi-ci-deployer"' "${service_accounts}/main.tf"
require_resource_pattern google_artifact_registry_repository_iam_member builder_writer 'role[[:space:]]+= "roles/artifactregistry\.writer"' "${service_accounts}/main.tf"
require_resource_pattern google_artifact_registry_repository_iam_member builder_writer 'repository = var\.artifact_registry_repository_id' "${service_accounts}/main.tf"
require_resource_pattern google_artifact_registry_repository_iam_member builder_writer 'member[[:space:]]+= "serviceAccount:\$\{google_service_account\.deployer\.email\}"' "${service_accounts}/main.tf"
require_pattern_count 1 'google_service_account\.deployer\.email' "${service_accounts}/main.tf"
forbid_pattern 'resource "google_project_iam_member" "deployer|roles/run\.' "${service_accounts}/main.tf"
require_pattern_count 2 'roles/iam\.serviceAccountUser' "${service_accounts}/main.tf"

for identity in \
  'production_console_deployer:newapi-prod-console-deployer' \
  'production_router_deployer:newapi-prod-router-deployer' \
  'production_console_rollback:newapi-prod-console-rollback' \
  'production_router_rollback:newapi-prod-router-rollback' \
  'production_website_deployer:newapi-prod-web-deployer' \
  'staging_deployer:newapi-staging-deployer'; do
  resource_name=${identity%%:*}
  account_id=${identity#*:}
  require_resource_pattern google_service_account "${resource_name}" "account_id[[:space:]]+= \"${account_id}\"" "${service_accounts}/main.tf"
done

expected_service_observer_permissions=$(printf '%s\n' \
  run.locations.list \
  run.operations.get \
  run.revisions.get \
  run.services.get | sort)
expected_service_mutator_permissions=$(printf '%s\n' \
  run.services.get \
  run.services.update | sort)
require_exact_permissions service_deployer_observer "${expected_service_observer_permissions}" "${service_accounts}/main.tf"
require_exact_permissions service_deployer_mutator "${expected_service_mutator_permissions}" "${service_accounts}/main.tf"
forbid_pattern '"run\.jobs\.|"cloudscheduler\.|newapi-supplier-batch-runner' "${service_accounts}/main.tf"
forbid_pattern 'resource "google_project_iam_member" "service_deployer_mutator"' "${service_accounts}/main.tf"

image_deployers=$(assignment_block image_deployers "${service_accounts}/main.tf")
for allowed_image_deployer in production_console production_router production_website staging; do
  grep -Eq "^[[:space:]]+${allowed_image_deployer}[[:space:]]+=" <<<"${image_deployers}" || {
    echo "missing ${allowed_image_deployer} from image_deployers" >&2
    exit 1
  }
done
if grep -Eq 'rollback|google_service_account\.deployer' <<<"${image_deployers}"; then
  echo "builder or rollback identity must not receive Artifact Registry reader through image_deployers" >&2
  exit 1
fi
require_resource_pattern google_artifact_registry_repository_iam_member service_deployer_reader 'role[[:space:]]+= "roles/artifactregistry\.reader"' "${service_accounts}/main.tf"
forbid_pattern 'member[[:space:]]+= "serviceAccount:\$\{google_service_account\.production_(console|router)_rollback\.email\}"' "${service_accounts}/main.tf"

for resource_name in production_console_deployer production_router_deployer production_console_rollback production_router_rollback production_website_deployer; do
  require_resource_pattern google_cloud_run_v2_service_iam_member "${resource_name}" 'role[[:space:]]+= module\.service_accounts\.service_deployer_mutator_custom_role_name' "${prod}/main.tf"
done
for resource_name in staging_backend_deployer staging_website_deployer; do
  require_resource_pattern google_cloud_run_v2_service_iam_member "${resource_name}" 'role[[:space:]]+= module\.service_accounts\.service_deployer_mutator_custom_role_name' "${prod}/staging.tf"
done
require_resource_pattern google_service_account_iam_member production_console_runtime_sa_user 'service_account_id = google_service_account\.runtime\.name' "${service_accounts}/main.tf"
require_resource_pattern google_service_account_iam_member production_router_runtime_sa_user 'service_account_id = google_service_account\.runtime\.name' "${service_accounts}/main.tf"
require_resource_pattern google_service_account_iam_member website_deployer_runtime_sa_user 'service_account_id = google_service_account\.web_runtime\[0\]\.name' "${prod}/main.tf"
require_resource_pattern google_service_account_iam_member staging_deployer_runtime_sa_user 'service_account_id = google_service_account\.staging_runtime\[0\]\.name' "${prod}/staging.tf"
require_resource_pattern google_service_account_iam_member staging_deployer_web_runtime_sa_user 'service_account_id = google_service_account\.staging_web_runtime\[0\]\.name' "${prod}/staging.tf"
require_pattern_count 1 'roles/iam\.serviceAccountUser' "${prod}/main.tf"
require_pattern_count 2 'roles/iam\.serviceAccountUser' "${prod}/staging.tf"

require_resource_pattern google_service_account promoter 'account_id[[:space:]]+= local\.promoter_service_account_id' "${module}/main.tf"
forbid_resource_pattern google_service_account promoter 'count[[:space:]]+=' "${module}/main.tf"
require_pattern 'promoter_service_account_id[[:space:]]+= "newapi-supplier-promoter"' "${module}/main.tf"
require_resource_pattern google_cloud_run_v2_job_iam_member promoter_job 'role[[:space:]]+= google_project_iam_custom_role\.promoter_job\.name' "${module}/main.tf"
require_resource_pattern google_cloud_run_v2_job_iam_member promoter_job 'member[[:space:]]+= "serviceAccount:\$\{google_service_account\.promoter\.email\}"' "${module}/main.tf"
require_resource_pattern google_service_account_iam_member promoter_runner_act_as 'service_account_id = google_service_account\.runner\.name' "${module}/main.tf"
require_resource_pattern google_service_account_iam_member promoter_runner_act_as 'role[[:space:]]+= "roles/iam\.serviceAccountUser"' "${module}/main.tf"
require_resource_pattern google_service_account_iam_member promoter_runner_act_as 'member[[:space:]]+= "serviceAccount:\$\{google_service_account\.promoter\.email\}"' "${module}/main.tf"
require_resource_pattern google_artifact_registry_repository_iam_member promoter_reader 'repository = var\.artifact_registry_repository_id' "${module}/main.tf"
require_resource_pattern google_artifact_registry_repository_iam_member promoter_reader 'role[[:space:]]+= "roles/artifactregistry\.reader"' "${module}/main.tf"
require_resource_pattern google_artifact_registry_repository_iam_member promoter_reader 'member[[:space:]]+= "serviceAccount:\$\{google_service_account\.promoter\.email\}"' "${module}/main.tf"
require_pattern_count 1 'roles/artifactregistry\.reader' "${module}/main.tf"
require_pattern_count 1 'roles/iam\.serviceAccountUser' "${module}/main.tf"

expected_promoter_job_permissions=$(printf '%s\n' \
  run.jobs.get \
  run.jobs.run \
  run.jobs.update | sort)
expected_promoter_observer_permissions=$(printf '%s\n' \
  logging.logEntries.list \
  run.executions.get \
  run.operations.get \
  run.revisions.get \
  run.services.get | sort)
require_exact_permissions promoter_job "${expected_promoter_job_permissions}" "${module}/main.tf"
require_exact_permissions promoter_observer "${expected_promoter_observer_permissions}" "${module}/main.tf"
forbid_pattern 'roles/(run\.(admin|developer)|cloudscheduler\.(admin|jobRunner)|logging\.(admin|viewer))' "${module}/main.tf"
forbid_pattern 'cloudscheduler\.' "${module}/main.tf"

for pool_id in github-actions github-prod-app-deploy github-prod-rollback github-prod-web-deploy github-staging-deploy github-supplier-promote; do
  require_pattern "(workload_identity_pool_id|pool_id)[[:space:]]+= \"${pool_id}\"" "${github_wif}/main.tf"
done
require_pattern 'assertion\.repository_id == '\''\$\{var\.github_repository_id\}'\''' "${github_wif}/main.tf"
require_pattern 'assertion\.repository_owner_id == '\''\$\{var\.github_repository_owner_id\}'\''' "${github_wif}/main.tf"
for workflow in gcp-deploy.yml gcp-rollback.yml gcp-deploy-website.yml gcp-deploy-website-static.yml gcp-deploy-staging.yml gcp-deploy-website-staging.yml gcp-promote-supplier-runner.yml; do
  require_pattern "assertion\\.workflow_ref.*${workflow}" "${github_wif}/main.tf"
done
require_pattern "assertion\.event_name == 'workflow_dispatch'" "${github_wif}/main.tf"
require_pattern "assertion\.ref == 'refs/heads/main'" "${github_wif}/main.tf"
require_pattern "assertion\.ref == 'refs/heads/staging'" "${github_wif}/main.tf"
require_resource_pattern google_service_account_iam_member wif_supplier_runner_promoter 'service_account_id = var\.supplier_runner_promoter_sa_name' "${github_wif}/main.tf"
require_resource_pattern google_service_account_iam_member wif_supplier_runner_promoter 'google_iam_workload_identity_pool\.privileged\["supplier_promotion"\]\.name' "${github_wif}/main.tf"
require_resource_pattern google_service_account_iam_member wif_supplier_runner_promoter 'subject/\$\{local\.production_subject\}' "${github_wif}/main.tf"
require_pattern '^github_repository[[:space:]]+= "SolveaCX/new-api"$' "${prod}/terraform.tfvars"
require_pattern '^github_repository_id[[:space:]]+= "1236600074"$' "${prod}/terraform.tfvars"
require_pattern '^github_repository_owner_id[[:space:]]+= "279667167"$' "${prod}/terraform.tfvars"
require_pattern 'supplier_runner_promoter_sa_name[[:space:]]+= module\.supplier_batch_job\.promoter_service_account_name' "${prod}/main.tf"

for supplier_policy in \
  supplier_batch_never_published_days \
  supplier_batch_oldest_never_published_age \
  supplier_batch_prior_day_unpublished_after_0800 \
  supplier_batch_backlog_observer_unhealthy; do
  require_resource_pattern google_monitoring_alert_policy "${supplier_policy}" 'count = var\.supplier_batch_monitoring_enabled && local\.alerts_enabled \? 1 : 0' "${monitoring}"
  require_resource_pattern google_monitoring_alert_policy "${supplier_policy}" 'resource\.type=\\"prometheus_target\\"' "${monitoring}"
  require_resource_pattern google_monitoring_alert_policy "${supplier_policy}" 'metric\.label\.service=\\"\$\{var\.router_service_name\}\\"' "${monitoring}"
  require_resource_pattern google_monitoring_alert_policy "${supplier_policy}" 'cross_series_reducer = "REDUCE_MAX"' "${monitoring}"
  require_resource_pattern google_monitoring_alert_policy "${supplier_policy}" 'group_by_fields[[:space:]]+= \["metric\.label\.service"\]' "${monitoring}"
  forbid_resource_pattern google_monitoring_alert_policy "${supplier_policy}" 'REDUCE_SUM' "${monitoring}"
done

require_resource_pattern google_monitoring_alert_policy supplier_batch_never_published_days 'newapi_supplier_accounting_never_published_days/gauge' "${monitoring}"
require_resource_pattern google_monitoring_alert_policy supplier_batch_never_published_days 'comparison[[:space:]]+= "COMPARISON_GT"' "${monitoring}"
require_resource_pattern google_monitoring_alert_policy supplier_batch_never_published_days 'threshold_value = 1' "${monitoring}"
require_resource_pattern google_monitoring_alert_policy supplier_batch_never_published_days 'duration[[:space:]]+= "60s"' "${monitoring}"

require_resource_pattern google_monitoring_alert_policy supplier_batch_oldest_never_published_age 'newapi_supplier_accounting_oldest_never_published_age_seconds/gauge' "${monitoring}"
require_resource_pattern google_monitoring_alert_policy supplier_batch_oldest_never_published_age 'comparison[[:space:]]+= "COMPARISON_GT"' "${monitoring}"
require_resource_pattern google_monitoring_alert_policy supplier_batch_oldest_never_published_age 'threshold_value = 86400' "${monitoring}"
require_resource_pattern google_monitoring_alert_policy supplier_batch_oldest_never_published_age 'duration[[:space:]]+= "60s"' "${monitoring}"

require_resource_pattern google_monitoring_alert_policy supplier_batch_prior_day_unpublished_after_0800 'newapi_supplier_accounting_prior_day_unpublished_after_0800/gauge' "${monitoring}"
require_resource_pattern google_monitoring_alert_policy supplier_batch_prior_day_unpublished_after_0800 'comparison[[:space:]]+= "COMPARISON_GT"' "${monitoring}"
require_resource_pattern google_monitoring_alert_policy supplier_batch_prior_day_unpublished_after_0800 'threshold_value = 0' "${monitoring}"
require_resource_pattern google_monitoring_alert_policy supplier_batch_prior_day_unpublished_after_0800 'duration[[:space:]]+= "60s"' "${monitoring}"

require_resource_pattern google_monitoring_alert_policy supplier_batch_backlog_observer_unhealthy 'newapi_supplier_accounting_backlog_observer_up/gauge' "${monitoring}"
require_resource_pattern google_monitoring_alert_policy supplier_batch_backlog_observer_unhealthy 'comparison[[:space:]]+= "COMPARISON_LT"' "${monitoring}"
require_resource_pattern google_monitoring_alert_policy supplier_batch_backlog_observer_unhealthy 'threshold_value = 1' "${monitoring}"
require_resource_pattern google_monitoring_alert_policy supplier_batch_backlog_observer_unhealthy 'condition_absent' "${monitoring}"
require_resource_count google_monitoring_alert_policy supplier_batch_backlog_observer_unhealthy 2 'duration[[:space:]]+= "120s"' "${monitoring}"

forbid_pattern 'google_logging_metric|newapi_supplier_batch_remaining_work_total|processed_days=1 remaining_work=true' "${monitoring}"
forbid_pattern 'supplier_batch_job_name' "${monitoring_variables}"
forbid_pattern 'supplier_batch_job_name' "${prod}/main.tf"
forbid_pattern 'resource "(google_cloud_run_v2_job|google_cloud_scheduler_job|google_service_account|google_secret_manager_secret)' "${monitoring}"

for metric in \
  newapi_supplier_accounting_never_published_days \
  newapi_supplier_accounting_oldest_never_published_age_seconds \
  newapi_supplier_accounting_prior_day_unpublished_after_0800 \
  newapi_supplier_accounting_backlog_observer_up \
  newapi_supplier_accounting_backlog_observed_at_seconds; do
  require_pattern "${metric}" "${infrastructure_doc}"
  require_pattern "${metric}" "${deployment_doc}"
  require_pattern "${metric}" "${operations_doc}"
done

require_pattern 'G006' "${deployment_doc}"
require_pattern 'G006' "${operations_doc}"

if grep -Rq -- 'google_secret_manager_secret_version' "${module}"; then
  echo "supplier batch Terraform must not create raw token secret versions" >&2
  exit 1
fi

terraform fmt -check \
  "${module}/main.tf" "${module}/variables.tf" "${module}/outputs.tf" \
  "${github_wif}/main.tf" "${github_wif}/variables.tf" "${github_wif}/outputs.tf" \
  "${service_accounts}/main.tf" "${service_accounts}/variables.tf" "${service_accounts}/outputs.tf" \
  "${root}/deploy/gcp/modules/monitoring/main.tf" "${root}/deploy/gcp/modules/monitoring/variables.tf" \
  "${prod}/main.tf" "${prod}/staging.tf" "${prod}/variables.tf" "${prod}/outputs.tf" "${prod}/terraform.tfvars"
