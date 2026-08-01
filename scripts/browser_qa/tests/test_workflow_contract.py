import json
import pathlib
import re
import subprocess
import sys
import tempfile
import unittest


REPO_ROOT = pathlib.Path(__file__).resolve().parents[3]
WORKFLOW = REPO_ROOT / ".github" / "workflows" / "gcp-browser-qa.yml"


def workflow_text():
    return WORKFLOW.read_text(encoding="utf-8")


def strip_comments(text):
    return "\n".join(line.split("#", 1)[0].rstrip() for line in text.splitlines())


def step_block(text, name):
    match = re.search(
        rf"(?ms)^      - name: {re.escape(name)}\n(?P<body>.*?)(?=^      - name: |\Z)",
        text,
    )
    if not match:
        raise AssertionError(f"step not found: {name}")
    return match.group("body")


def summary_python_script():
    block = step_block(workflow_text(), "Fetch sanitized manifest and write summary")
    match = re.search(r"(?ms)<<'PY'\n(?P<script>.*?)^          PY\s*$", block)
    if not match:
        raise AssertionError("summary python heredoc not found")
    return re.sub(r"(?m)^          ", "", match.group("script"))


def run_summary_python(manifest, *, main_outcome="success", cleanup_outcome="success"):
    with tempfile.TemporaryDirectory() as tmp:
        root = pathlib.Path(tmp)
        manifest_path = root / "manifest.json"
        output_path = root / "github-output.txt"
        summary_path = root / "github-summary.md"
        manifest_path.write_text(json.dumps(manifest), encoding="utf-8")
        output_path.write_text("", encoding="utf-8")
        summary_path.write_text("", encoding="utf-8")
        env = {
            "EFFECTIVE_RUN_ID": "12345",
            "GITHUB_OUTPUT": str(output_path),
            "GITHUB_STEP_SUMMARY": str(summary_path),
        }
        completed = subprocess.run(
            [
                sys.executable,
                "-c",
                summary_python_script(),
                str(manifest_path),
                "gs://flatkey-browser-qa-reports/runs/12345/manifest.json",
                main_outcome,
                cleanup_outcome,
            ],
            env=env,
            check=True,
            capture_output=True,
            text=True,
        )
        output = output_path.read_text(encoding="utf-8")
        match = re.search(r"(?m)^manifest_status=(?P<status>.+)$", output)
        if not match:
            raise AssertionError(f"manifest_status missing; stdout={completed.stdout!r} stderr={completed.stderr!r}")
        return match.group("status"), summary_path.read_text(encoding="utf-8")


def summarized_root_manifest(status, *, summary=None):
    if summary is None:
        summary = {
            "replay_status": "passed",
            "exploration_status": "not_started",
            "exploration_actions": 0,
            "finding_count": 0,
        }
    return {
        "schema_version": 1,
        "run_id": "12345",
        "status": status,
        "latest": {"main_execution_id": "main-001", "cleanup_execution_id": None},
        "executions": [
            {
                "kind": "main",
                "execution_id": "main-001",
                "manifest": "runs/12345/main/main-001/manifest.json",
                "status": status,
                "created_at": 1,
                "summary": summary,
            }
        ],
    }


class BrowserQaWorkflowContractTests(unittest.TestCase):
    def test_workflow_is_manual_only_with_minimal_permissions_and_serial_concurrency(self):
        text = workflow_text()
        uncommented = strip_comments(text)

        self.assertRegex(uncommented, r"(?m)^on:\n  workflow_dispatch:\n")
        self.assertNotRegex(uncommented, r"(?m)^  (push|pull_request|schedule|workflow_call):\s*$")
        self.assertRegex(uncommented, r"(?ms)^permissions:\n  contents: read\n  id-token: write\b")
        self.assertEqual(len(re.findall(r"(?m)^concurrency:\s*$", uncommented)), 1)
        self.assertRegex(uncommented, r"(?ms)^concurrency:\n  group: .+\n  cancel-in-progress: false\b")

    def test_dispatch_inputs_cover_normal_core_and_cleanup_only_with_original_run_id(self):
        text = workflow_text()
        self.assertRegex(text, r"(?ms)^      mode:\n        .*?type: choice\n        .*?options:\n          - normal\n          - core\n          - cleanup-only")
        self.assertRegex(text, r"(?ms)^      original_run_id:\n        .*?description: .*cleanup-only.*original.*run.*id")
        self.assertRegex(text, r"(?m)^\s*if \[\[ \"\$\{\{ inputs.mode \}\}\" == \"cleanup-only\" && -z \"\$\{\{ inputs.original_run_id \}\}\" \]\]; then")

    def test_workflow_uses_dedicated_qa_identity_and_never_names_production_environment(self):
        text = workflow_text()
        self.assertIn("GCP_BROWSER_QA_WIF_PROVIDER", text)
        self.assertIn("GCP_BROWSER_QA_DEPLOYER_SA", text)
        self.assertNotIn("vars.GCP_WIF_PROVIDER", text)
        self.assertNotIn("vars.GCP_DEPLOYER_SA", text)
        self.assertNotRegex(text, r"(?mi)^\s*environment:\s*(production|prod|staging)\b")
        self.assertNotRegex(text, r"(?i)\bproduction\b|\bprod\b")

    def test_image_is_sha_bound_and_only_qa_resources_are_mutated(self):
        text = workflow_text()
        self.assertRegex(text, r"browser-qa:\$\{\{ github\.sha \}\}-\$\{\{ github\.run_attempt \}\}")
        self.assertNotRegex(text, r"browser-qa:(latest|\$\{\{ github\.ref_name \}\})")
        self.assertIn("flatkey-staging-browser-qa-broker", text)
        self.assertIn("flatkey-staging-browser-qa", text)
        self.assertIn("flatkey-staging-browser-qa-cleanup", text)
        names = sorted(set(re.findall(r"(?:QA_\w+): (flatkey-[A-Za-z0-9-]+)", text)))
        self.assertEqual(
            names,
            [
                "flatkey-staging-browser-qa",
                "flatkey-staging-browser-qa-broker",
                "flatkey-staging-browser-qa-cleanup",
            ],
        )

    def test_normal_path_smokes_image_before_push_and_skips_mutation_for_cleanup_only(self):
        text = workflow_text()
        smoke = step_block(text, "Smoke test browser QA image")
        self.assertIn("docker inspect --format='{{.Config.User}}'", smoke)
        self.assertIn("docker run --rm --entrypoint sh", smoke)
        self.assertIn("id -u", smoke)
        self.assertIn("--entrypoint codex", smoke)
        self.assertIn("--version", smoke)
        self.assertIn("--entrypoint playwright-mcp", smoke)
        self.assertIn("--entrypoint python3", smoke)
        self.assertIn("-m unittest discover -s /opt/flatkey-browser-qa/tests -v", smoke)
        self.assertIn("docker run --rm -i --entrypoint python3", smoke)
        self.assertIn("ChromiumRuntime", smoke)
        self.assertIn("playwright_child_command", smoke)
        self.assertIn("communicate", smoke)
        self.assertIn("timeout=", smoke)
        self.assertIn('"method": "initialize"', smoke)
        self.assertIn('"method": "tools/list"', smoke)
        self.assertNotIn("chromium.launchServer", smoke)
        for step in [
            "Build browser QA image",
            "Smoke test browser QA image",
            "Push browser QA image",
            "Update browser QA Cloud Run resources",
            "Execute main browser QA job",
        ]:
            block = step_block(text, step)
            self.assertRegex(block, r"(?m)^        if: inputs\.mode != 'cleanup-only'$")

    def test_resource_update_step_only_updates_images_without_persistent_env_secret_or_args_mutation(self):
        text = workflow_text()
        update = step_block(text, "Update browser QA Cloud Run resources")
        self.assertNotIn("--update-secrets", update)
        self.assertNotIn("--update-env-vars", update)
        self.assertNotIn("--args=", update)
        self.assertEqual(update.count("gcloud run services update"), 1)
        self.assertEqual(update.count("gcloud run jobs update"), 2)
        for command in re.findall(r"gcloud run (?:services|jobs) update[\s\S]*?(?=\n          gcloud run |\Z)", update):
            self.assertIn("--image=\"${IMAGE_URI}\"", command)
            self.assertNotRegex(command, r"--(update-secrets|update-env-vars|args)=")

    def test_main_and_cleanup_execute_with_wait_and_cleanup_is_unconditional(self):
        text = workflow_text()
        main = step_block(text, "Execute main browser QA job")
        cleanup = step_block(text, "Execute cleanup browser QA job")
        self.assertRegex(main, r"gcloud run jobs execute \"\$\{QA_MAIN_JOB\}\"[\s\S]*--wait")
        self.assertRegex(cleanup, r"gcloud run jobs execute \"\$\{QA_CLEANUP_JOB\}\"[\s\S]*--wait")
        self.assertRegex(cleanup, r"(?m)^        if: \${\{ always\(\) && steps\.qa\.outcome == 'success' \}\}$")
        self.assertIn("MAIN_STATUS", main)
        self.assertIn("main_status", main)
        self.assertIn("EFFECTIVE_RUN_ID", main)
        self.assertIn("EFFECTIVE_RUN_ID", cleanup)
        self.assertIn("FLATKEY_QA_RUN_ID=${EFFECTIVE_RUN_ID}", main)
        self.assertIn("FLATKEY_QA_RUN_ID=${EFFECTIVE_RUN_ID}", cleanup)
        self.assertIn("FLATKEY_BROWSER_QA_MODE=${{ inputs.mode }}", main)

    def test_fetches_only_sanitized_root_manifest_and_summary_stays_non_secret(self):
        text = workflow_text()
        summary = step_block(text, "Fetch sanitized manifest and write summary")
        self.assertIn("runs/${EFFECTIVE_RUN_ID}/manifest.json", summary)
        self.assertNotIn("result.json", summary)
        self.assertNotIn("codex-events.jsonl", summary)
        self.assertNotIn("codex-stderr.txt", summary)
        self.assertNotIn("screenshots", summary)
        for safe_field in ["replay", "exploration", "finding", "cleanup", "status", "gcs_uri"]:
            self.assertIn(safe_field, summary)
        self.assertNotRegex(summary, r"(?i)email|password|cookie|authorization|api[_-]?key|token|verification")
        self.assertIn("latest.main_execution_id", summary)
        self.assertIn("summary", summary)
        self.assertIn("validate_root_manifest", summary)
        self.assertNotIn("main_record.get(\"result\"", summary)

    def test_summary_status_priority_keeps_cleanup_failure_stronger_than_root_status(self):
        text = workflow_text()
        summary = step_block(text, "Fetch sanitized manifest and write summary")
        self.assertIn("cleanup_outcome", summary)
        self.assertIn("main_outcome", summary)
        self.assertRegex(summary, r"if cleanup_outcome != \"success\"[\s\S]*status = \"cleanup_failed\"")
        self.assertRegex(summary, r"elif manifest_error is not None[\s\S]*status = \"infrastructure_failed\"")
        self.assertRegex(summary, r"elif main_outcome == \"failure\" and root_status == \"passed\"[\s\S]*status = \"infrastructure_failed\"")

    def test_summary_python_preserves_trusted_root_status_for_intentional_main_failures(self):
        cases = [
            ("replay_failed", "failure", "success", "replay_failed"),
            ("findings_detected", "failure", "success", "findings_detected"),
            ("passed", "failure", "success", "infrastructure_failed"),
            ("replay_failed", "failure", "failure", "cleanup_failed"),
            ("infrastructure_failed", "failure", "success", "infrastructure_failed"),
            ("replay_failed", "cancelled", "success", "infrastructure_failed"),
        ]
        for root_status, main_outcome, cleanup_outcome, expected in cases:
            with self.subTest(root_status=root_status, main_outcome=main_outcome, cleanup_outcome=cleanup_outcome):
                status, rendered = run_summary_python(
                    summarized_root_manifest(root_status),
                    main_outcome=main_outcome,
                    cleanup_outcome=cleanup_outcome,
                )
                self.assertEqual(status, expected)
                self.assertIn(f"- status: {expected}", rendered)

        status, rendered = run_summary_python(
            {**summarized_root_manifest("passed"), "schema_version": 99},
            main_outcome="success",
            cleanup_outcome="success",
        )
        self.assertEqual(status, "infrastructure_failed")
        self.assertIn("- replay status: unknown", rendered)

    def test_standalone_failures_cover_cleanup_infra_replay_and_findings_states(self):
        text = workflow_text()
        gate = step_block(text, "Fail standalone workflow for actionable QA states")
        for state in ["cleanup_failed", "infrastructure_failed", "replay_failed", "findings_detected"]:
            self.assertIn(state, gate)
        self.assertNotRegex(text, r"(?i)release gate|needs: .*deploy|environment:")

    def test_no_secret_value_is_placed_in_arguments_outputs_inputs_or_summary(self):
        text = workflow_text()
        self.assertNotIn("${{ secrets.", text)
        self.assertNotRegex(text, r"(?i)--(set-env-vars|args|update-env-vars)=[^\n]*(password|cookie|authorization|api[_-]?key|token|secret)")
        self.assertNotRegex(text, r"(?i)GITHUB_OUTPUT[^\n]*(password|cookie|authorization|api[_-]?key|token|secret)")
        self.assertNotRegex(text, r"(?i)GITHUB_STEP_SUMMARY[^\n]*(password|cookie|authorization|api[_-]?key|token|secret|email)")


if __name__ == "__main__":
    unittest.main()
