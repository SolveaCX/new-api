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


def _validated_actions(change, context):
    if not isinstance(change, dict):
        raise PlanValidationError(f"{context} change must be an object")
    actions = change.get("actions")
    if not isinstance(actions, list):
        raise PlanValidationError(f"{context} actions must be a list of strings")
    if not all(isinstance(action, str) for action in actions):
        raise PlanValidationError(f"{context} actions must be a list of strings")
    return tuple(actions)


def _meaningful_resource_changes(plan):
    resource_changes = plan.get("resource_changes")
    if not isinstance(resource_changes, list):
        raise PlanValidationError("resource_changes must be a list")

    changes = []
    seen_keys = set()
    for resource_change in resource_changes:
        if not isinstance(resource_change, dict):
            raise PlanValidationError("resource change must be an object")

        address = resource_change.get("address")
        if not isinstance(address, str) or not address.strip():
            raise PlanValidationError("resource change address must be a non-empty string")

        deposed = resource_change.get("deposed")
        if deposed is not None and (not isinstance(deposed, str) or not deposed.strip()):
            raise PlanValidationError("resource change deposed must be a non-empty string")

        key = (address, deposed)
        if key in seen_keys:
            raise PlanValidationError(f"duplicate resource change: {address}")
        seen_keys.add(key)

        actions = _validated_actions(resource_change.get("change"), "resource")
        if actions == ("no-op",):
            continue
        if actions == ("create",) and deposed is not None:
            raise PlanValidationError(f"bootstrap plan is not create-only: {address}")
        changes.append((address, deposed, actions))
    return changes


def _meaningful_output_changes(plan):
    output_changes = plan.get("output_changes")
    if not isinstance(output_changes, dict):
        raise PlanValidationError("output_changes must be an object")

    changes = {}
    for name, output_change in output_changes.items():
        if not isinstance(name, str) or not name.strip():
            raise PlanValidationError("output change name must be a non-empty string")
        actions = _validated_actions(output_change, "output")
        changes[name] = actions
    return changes


def _first_sorted(values):
    return sorted(values)[0]


def validate_bootstrap_plan(plan):
    if not isinstance(plan, dict):
        raise PlanValidationError("saved plan must be an object")

    changes = _meaningful_resource_changes(plan)
    output_changes = _meaningful_output_changes(plan)

    if not changes:
        raise PlanValidationError("saved plan has no resource changes")

    resource_addresses = frozenset(address for address, _deposed, _actions in changes)
    unexpected_resources = resource_addresses - ALLOWED_RESOURCE_ADDRESSES
    if unexpected_resources:
        raise PlanValidationError(f"unexpected resource: {_first_sorted(unexpected_resources)}")

    missing_resources = ALLOWED_RESOURCE_ADDRESSES - resource_addresses
    if missing_resources:
        raise PlanValidationError(f"missing resource: {_first_sorted(missing_resources)}")

    for address, deposed, actions in sorted(changes, key=lambda item: (item[0], item[1] or "")):
        if actions != ("create",) or deposed is not None:
            raise PlanValidationError(f"bootstrap plan is not create-only: {address}")

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

    return {address: actions for address, _deposed, actions in changes}


def _reject_duplicate_keys(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise PlanValidationError(f"duplicate JSON object key: {key}")
        result[key] = value
    return result


def main(argv=None):
    parser = argparse.ArgumentParser(
        description="Validate an exact Browser QA Terraform bootstrap saved plan JSON."
    )
    parser.add_argument("plan_json", type=Path, help="Path produced by terraform show -json")
    args = parser.parse_args(argv)

    try:
        plan = json.loads(args.plan_json.read_text(encoding="utf-8"), object_pairs_hook=_reject_duplicate_keys)
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
