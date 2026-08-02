import json
import re
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from scripts.browser_qa.terraform_plan_guard import (
    ALLOWED_OUTPUTS,
    ALLOWED_RESOURCE_ADDRESSES,
    PlanValidationError,
    validate_bootstrap_plan,
)


REPO_ROOT = Path(__file__).resolve().parents[3]
GUARD_SCRIPT = REPO_ROOT / "scripts" / "browser_qa" / "terraform_plan_guard.py"
BROWSER_QA_TF = REPO_ROOT / "deploy" / "gcp" / "envs" / "browser-qa-staging" / "browser_qa.tf"
OUTPUTS_TF = REPO_ROOT / "deploy" / "gcp" / "envs" / "browser-qa-staging" / "outputs.tf"

EXPECTED_RESOURCE_ADDRESSES = frozenset(
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

EXPECTED_OUTPUTS = frozenset(
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


def valid_plan():
    return {
        "resource_changes": [
            {"address": address, "change": {"actions": ["create"]}}
            for address in sorted(ALLOWED_RESOURCE_ADDRESSES)
        ],
        "output_changes": {
            output: {"actions": ["create"]}
            for output in sorted(ALLOWED_OUTPUTS)
        },
    }


def assert_rejected(testcase, plan, expected_message):
    with testcase.assertRaisesRegex(PlanValidationError, re.escape(expected_message)):
        validate_bootstrap_plan(plan)


def run_guard(raw_json):
    with tempfile.TemporaryDirectory() as tmp_dir:
        plan_path = Path(tmp_dir) / "plan.json"
        plan_path.write_text(raw_json, encoding="utf-8")
        return subprocess.run(
            [sys.executable, "-B", str(GUARD_SCRIPT), str(plan_path)],
            cwd=REPO_ROOT,
            text=True,
            capture_output=True,
            check=False,
        )


def run_guard_help():
    return subprocess.run(
        [sys.executable, "-B", str(GUARD_SCRIPT), "--help"],
        cwd=REPO_ROOT,
        text=True,
        capture_output=True,
        check=False,
    )


class TerraformPlanGuardTest(unittest.TestCase):
    def test_complete_exact_create_only_plan_is_accepted(self):
        self.assertIsInstance(ALLOWED_RESOURCE_ADDRESSES, frozenset)
        self.assertIsInstance(ALLOWED_OUTPUTS, frozenset)
        self.assertEqual(ALLOWED_RESOURCE_ADDRESSES, EXPECTED_RESOURCE_ADDRESSES)
        self.assertEqual(ALLOWED_OUTPUTS, EXPECTED_OUTPUTS)

        browser_qa_resources = frozenset(
            ".".join(match)
            for match in re.findall(
                r'(?m)^\s*resource\s+"([^"]+)"\s+"([^"]+)"\s*\{',
                BROWSER_QA_TF.read_text(encoding="utf-8"),
            )
        )
        browser_qa_outputs = frozenset(
            re.findall(r'(?m)^\s*output\s+"([^"]+)"\s*\{', OUTPUTS_TF.read_text(encoding="utf-8"))
        )
        self.assertEqual(ALLOWED_RESOURCE_ADDRESSES, browser_qa_resources)
        self.assertEqual(ALLOWED_OUTPUTS, browser_qa_outputs)

        changes = validate_bootstrap_plan(valid_plan())

        self.assertEqual(set(changes), ALLOWED_RESOURCE_ADDRESSES)
        for address, actions in changes.items():
            with self.subTest(address=address):
                self.assertEqual(actions, ("create",))

    def test_extra_non_qa_resource_address_is_rejected_as_unexpected(self):
        plan = valid_plan()
        plan["resource_changes"].append(
            {"address": "google_cloud_run_v2_service.newapi_console", "change": {"actions": ["create"]}}
        )

        assert_rejected(self, plan, "unexpected resource")

    def test_any_resource_change_that_is_not_single_create_is_rejected(self):
        for actions in (["update"], ["delete"], ["create", "delete"], ["delete", "create"], ["read"], ["replace"]):
            with self.subTest(actions=actions):
                plan = valid_plan()
                plan["resource_changes"][0]["change"]["actions"] = actions

                assert_rejected(self, plan, "bootstrap plan is not create-only")

        plan = valid_plan()
        plan["resource_changes"][0]["deposed"] = "deadbeef"
        assert_rejected(self, plan, "bootstrap plan is not create-only")

    def test_missing_resource_is_rejected(self):
        plan = valid_plan()
        plan["resource_changes"].pop()

        assert_rejected(self, plan, "missing resource")

    def test_duplicate_resource_change_keys_are_rejected_before_create_only_checks(self):
        first_address = sorted(ALLOWED_RESOURCE_ADDRESSES)[0]
        for actions in (["create"], ["update"]):
            with self.subTest(actions=actions):
                plan = valid_plan()
                plan["resource_changes"].append(
                    {"address": first_address, "change": {"actions": actions}}
                )

                assert_rejected(self, plan, "duplicate resource change")

        plan = valid_plan()
        original = plan["resource_changes"].pop(0)
        plan["resource_changes"].insert(
            0,
            {"address": original["address"], "deposed": "deadbeef", "change": {"actions": ["update"]}},
        )
        plan["resource_changes"].append(original)

        assert_rejected(self, plan, "bootstrap plan is not create-only")

    def test_non_qa_resource_no_op_is_rejected_as_unexpected(self):
        plan = valid_plan()
        plan["resource_changes"].append(
            {
                "address": "google_sql_database_instance.main",
                "change": {"actions": ["no-op"]},
            }
        )

        assert_rejected(self, plan, "unexpected resource")

    def test_well_formed_allowed_resource_no_op_is_ignored_but_cannot_bypass_duplicates(self):
        plan = valid_plan()
        first_address = sorted(ALLOWED_RESOURCE_ADDRESSES)[0]
        plan["resource_changes"].append(
            {
                "address": first_address,
                "deposed": "deadbeef",
                "change": {"actions": ["no-op"]},
            }
        )

        changes = validate_bootstrap_plan(plan)
        self.assertEqual(set(changes), ALLOWED_RESOURCE_ADDRESSES)

        plan = valid_plan()
        plan["resource_changes"].append(
            {"address": first_address, "change": {"actions": ["no-op"]}}
        )

        assert_rejected(self, plan, "duplicate resource change")

    def test_output_set_must_be_exact(self):
        plan = valid_plan()
        plan["output_changes"]["browser_qa_extra"] = {"actions": ["create"]}
        assert_rejected(self, plan, "unexpected output")

        plan = valid_plan()
        plan["output_changes"].pop(next(iter(ALLOWED_OUTPUTS)))
        assert_rejected(self, plan, "missing output")

    def test_output_changes_must_be_create_only(self):
        plan = valid_plan()
        plan["output_changes"][next(iter(ALLOWED_OUTPUTS))]["actions"] = ["update"]

        assert_rejected(self, plan, "output changes are not create-only")

        plan = valid_plan()
        plan["output_changes"]["browser_qa_extra"] = {"actions": ["no-op"]}

        assert_rejected(self, plan, "unexpected output")

    def test_empty_plan_is_rejected_as_no_resource_changes(self):
        assert_rejected(self, {"resource_changes": [], "output_changes": {}}, "saved plan has no resource changes")
        assert_rejected(
            self,
            {
                "resource_changes": [
                    {"address": sorted(ALLOWED_RESOURCE_ADDRESSES)[0], "change": {"actions": ["no-op"]}}
                ],
                "output_changes": {},
            },
            "saved plan has no resource changes",
        )

    def test_malformed_plan_shapes_are_rejected_with_validation_error(self):
        first_address = sorted(ALLOWED_RESOURCE_ADDRESSES)[0]
        first_output = sorted(ALLOWED_OUTPUTS)[0]
        malformed_plans = [
            [],
            {},
            {"resource_changes": {}},
            {"resource_changes": ["bad"]},
            {"resource_changes": [{"address": "", "change": {"actions": ["create"]}}]},
            {"resource_changes": [{"address": first_address, "change": []}]},
            {"resource_changes": [{"address": first_address, "change": {}}]},
            {"resource_changes": [{"address": first_address, "change": {"actions": "create"}}]},
            {"resource_changes": [{"address": first_address, "change": {"actions": [1]}}]},
            {"resource_changes": [{"address": first_address, "deposed": "", "change": {"actions": ["no-op"]}}]},
            {"resource_changes": [], "output_changes": []},
            {"resource_changes": [], "output_changes": {first_output: []}},
            {"resource_changes": [], "output_changes": {first_output: {}}},
            {"resource_changes": [], "output_changes": {first_output: {"actions": "create"}}},
            {"resource_changes": [], "output_changes": {first_output: {"actions": [1]}}},
        ]
        for plan in malformed_plans:
            with self.subTest(plan=plan):
                with self.assertRaises(PlanValidationError):
                    validate_bootstrap_plan(plan)

    def test_blank_address_deposed_and_output_name_are_malformed(self):
        first_address = sorted(ALLOWED_RESOURCE_ADDRESSES)[0]

        plan = valid_plan()
        plan["resource_changes"][0]["address"] = " "
        assert_rejected(self, plan, "resource change address must be a non-empty string")

        plan = valid_plan()
        plan["resource_changes"].append(
            {"address": first_address, "deposed": " ", "change": {"actions": ["no-op"]}}
        )
        assert_rejected(self, plan, "resource change deposed must be a non-empty string")

        plan = valid_plan()
        plan["output_changes"][" "] = {"actions": ["create"]}
        assert_rejected(self, plan, "output change name must be a non-empty string")

    def test_cli_rejects_invalid_and_wrong_json_without_traceback(self):
        update_plan = valid_plan()
        update_plan["resource_changes"][0]["change"]["actions"] = ["update"]

        for raw_json in ("{", "[]", "{}", json.dumps(update_plan)):
            with self.subTest(raw_json=raw_json):
                result = run_guard(raw_json)

                self.assertEqual(result.returncode, 1)
                self.assertTrue(result.stderr.startswith("ABORT:"), result.stderr)
                self.assertNotIn("Traceback", result.stderr)

    def test_cli_help_describes_terraform_show_json_positional(self):
        result = run_guard_help()

        self.assertEqual(result.returncode, 0)
        self.assertIn("Path produced by terraform show -json", result.stdout)

    def test_cli_rejects_raw_duplicate_output_keys(self):
        output_entries = ",".join(
            f'"{output}": {{"actions": ["create"]}}'
            for output in sorted(ALLOWED_OUTPUTS)
        )
        duplicate_output = sorted(ALLOWED_OUTPUTS)[0]
        raw_json = json.dumps({"resource_changes": valid_plan()["resource_changes"]})[:-1]
        raw_json = (
            raw_json
            + f', "output_changes": {{{output_entries}, '
            + f'"{duplicate_output}": {{"actions": ["create"]}}'
            + "}}"
        )

        result = run_guard(raw_json)

        self.assertEqual(result.returncode, 1)
        self.assertTrue(result.stderr.startswith("ABORT:"), result.stderr)
        self.assertNotIn("Traceback", result.stderr)


if __name__ == "__main__":
    unittest.main()
