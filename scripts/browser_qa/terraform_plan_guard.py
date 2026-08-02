import argparse
import json
import sys
from pathlib import Path


ALLOWED_RESOURCE_ADDRESSES = frozenset(
    {
        "google_artifact_registry_repository.browser_qa",
        "google_artifact_registry_repository_iam_member.browser_qa_deployer_writer",
        "google_cloud_run_v2_job.browser_qa_cleanup",
        "google_cloud_run_v2_job.browser_qa_main",
        "google_cloud_run_v2_job_iam_member.browser_qa_cleanup_deployer_developer",
        "google_cloud_run_v2_job_iam_member.browser_qa_cleanup_deployer_invoker",
        "google_cloud_run_v2_job_iam_member.browser_qa_main_deployer_developer",
        "google_cloud_run_v2_job_iam_member.browser_qa_main_deployer_invoker",
        "google_cloud_run_v2_service.browser_qa_broker",
        "google_cloud_run_v2_service_iam_member.browser_qa_broker_deployer_developer",
        "google_cloud_run_v2_service_iam_member.browser_qa_broker_invoker",
        "google_iam_workload_identity_pool.browser_qa_github",
        "google_iam_workload_identity_pool_provider.browser_qa_github",
        "google_project_iam_member.browser_qa_broker_log_writer",
        "google_project_iam_member.browser_qa_cleanup_log_writer",
        "google_project_iam_member.browser_qa_runtime_log_writer",
        "google_secret_manager_secret.browser_qa_codex_api_key",
        "google_secret_manager_secret.browser_qa_gmail_oauth",
        "google_secret_manager_secret.browser_qa_identity_seed",
        "google_secret_manager_secret_iam_member.browser_qa_broker_gmail_oauth",
        "google_secret_manager_secret_iam_member.browser_qa_cleanup_identity_seed",
        "google_secret_manager_secret_iam_member.browser_qa_runtime_codex_api_key",
        "google_secret_manager_secret_iam_member.browser_qa_runtime_identity_seed",
        "google_service_account.browser_qa_broker",
        "google_service_account.browser_qa_cleanup",
        "google_service_account.browser_qa_deployer",
        "google_service_account.browser_qa_runtime",
        "google_service_account_iam_member.browser_qa_broker_user",
        "google_service_account_iam_member.browser_qa_cleanup_user",
        "google_service_account_iam_member.browser_qa_runtime_user",
        "google_service_account_iam_member.browser_qa_wif_deployer",
        "google_storage_bucket.browser_qa_reports",
        "google_storage_bucket_iam_member.browser_qa_cleanup_report_admin",
        "google_storage_bucket_iam_member.browser_qa_deployer_report_viewer",
        "google_storage_bucket_iam_member.browser_qa_runtime_report_creator",
    }
)

ALLOWED_OUTPUTS = frozenset(
    {
        "browser_qa_artifact_registry_url",
        "browser_qa_broker_service_name",
        "browser_qa_broker_uri",
        "browser_qa_cleanup_job_name",
        "browser_qa_deployer_sa_email",
        "browser_qa_main_job_name",
        "browser_qa_report_bucket",
        "browser_qa_wif_provider",
    }
)


class PlanValidationError(ValueError):
    pass


def _actions(change):
    if not isinstance(change, dict):
        return None
    actions = change.get("actions")
    if actions is None:
        return None
    return tuple(actions)


def _meaningful_resource_changes(plan):
    changes = {}
    for resource_change in plan.get("resource_changes") or []:
        actions = _actions(resource_change.get("change", {}))
        if actions in (None, (), ("no-op",)):
            continue
        changes[resource_change.get("address")] = actions
    return changes


def _meaningful_output_changes(plan):
    changes = {}
    for name, output_change in (plan.get("output_changes") or {}).items():
        actions = _actions(output_change)
        if actions in (None, (), ("no-op",)):
            continue
        changes[name] = actions
    return changes


def _first_sorted(values):
    return sorted(values)[0]


def validate_bootstrap_plan(plan):
    changes = _meaningful_resource_changes(plan)
    if not changes:
        raise PlanValidationError("saved plan has no resource changes")

    resource_addresses = frozenset(changes)
    unexpected_resources = resource_addresses - ALLOWED_RESOURCE_ADDRESSES
    if unexpected_resources:
        raise PlanValidationError(f"unexpected resource: {_first_sorted(unexpected_resources)}")

    missing_resources = ALLOWED_RESOURCE_ADDRESSES - resource_addresses
    if missing_resources:
        raise PlanValidationError(f"missing resource: {_first_sorted(missing_resources)}")

    for address in sorted(changes):
        if changes[address] != ("create",):
            raise PlanValidationError(f"bootstrap plan is not create-only: {address}")

    output_changes = _meaningful_output_changes(plan)
    output_names = frozenset(output_changes)
    unexpected_outputs = output_names - ALLOWED_OUTPUTS
    if unexpected_outputs:
        raise PlanValidationError(f"unexpected output: {_first_sorted(unexpected_outputs)}")

    missing_outputs = ALLOWED_OUTPUTS - output_names
    if missing_outputs:
        raise PlanValidationError(f"missing output: {_first_sorted(missing_outputs)}")

    for output in sorted(output_changes):
        if output_changes[output] != ("create",):
            raise PlanValidationError(f"output changes are not create-only: {output}")

    return changes


def main(argv=None):
    parser = argparse.ArgumentParser(
        description="Validate an exact Browser QA Terraform bootstrap saved plan JSON."
    )
    parser.add_argument("plan_json", type=Path)
    args = parser.parse_args(argv)

    try:
        plan = json.loads(args.plan_json.read_text(encoding="utf-8"))
        changes = validate_bootstrap_plan(plan)
    except (OSError, json.JSONDecodeError, PlanValidationError) as exc:
        print(f"ABORT: {exc}", file=sys.stderr)
        return 1

    print("OK: exact Browser QA create-only bootstrap plan")
    for address in sorted(changes):
        print(f"  {address}: create")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
