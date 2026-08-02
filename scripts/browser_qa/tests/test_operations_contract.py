import pathlib
import re
import unittest


REPO_ROOT = pathlib.Path(__file__).resolve().parents[3]
OPERATIONS = REPO_ROOT / "deploy" / "gcp" / "docs" / "OPERATIONS.md"
PROD_ROOT = "deploy/gcp/envs/prod"
BROWSER_QA_ROOT = "deploy/gcp/envs/browser-qa-staging"
BROWSER_QA_BACKEND_PREFIX = "envs/browser-qa-staging"
BROWSER_QA_REQUIRED_APIS = {
    "artifactregistry.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "iam.googleapis.com",
    "iamcredentials.googleapis.com",
    "logging.googleapis.com",
    "run.googleapis.com",
    "secretmanager.googleapis.com",
    "serviceusage.googleapis.com",
    "sts.googleapis.com",
    "storage.googleapis.com",
}
BROWSER_QA_LIVE_RESOURCES = {
    "flatkey-staging-browser-qa",
    "flatkey-staging-browser-qa-broker",
    "flatkey-staging-browser-qa-cleanup",
    "flatkey-browser-qa-runtime@vocai-gemini-prod.iam.gserviceaccount.com",
    "flatkey-browser-qa-broker@vocai-gemini-prod.iam.gserviceaccount.com",
    "flatkey-browser-qa-cleanup@vocai-gemini-prod.iam.gserviceaccount.com",
    "flatkey-browser-qa-deployer@vocai-gemini-prod.iam.gserviceaccount.com",
    "flatkey-browser-qa-codex-api-key",
    "flatkey-browser-qa-identity-seed",
    "flatkey-browser-qa-gmail-oauth",
    "gs://vocai-gemini-prod-flatkey-browser-qa-reports",
    "flatkey-browser-qa-github",
    "staging",
}
ABSENCE_ONLY_DIAGNOSTICS = {"404", "NOT_FOUND", "does\\ not\\ exist"}
UNKNOWN_ABSENCE_DIAGNOSTICS = {
    "PERMISSION_DENIED",
    "UNAUTHENTICATED",
    "UNAVAILABLE",
    '""',
}
UNKNOWN_STATE_DIAGNOSTICS = UNKNOWN_ABSENCE_DIAGNOSTICS - {'""'}


OUTPUT_BACKED_VARIABLES = {
    "GCP_BROWSER_QA_AR_REPO_URL": "browser_qa_artifact_registry_url",
    "GCP_BROWSER_QA_WIF_PROVIDER": "browser_qa_wif_provider",
    "GCP_BROWSER_QA_DEPLOYER_SA": "browser_qa_deployer_sa_email",
    "GCP_BROWSER_QA_GCS_BUCKET": "browser_qa_report_bucket",
}


def operations_text():
    return OPERATIONS.read_text(encoding="utf-8")


def browser_qa_section():
    text = operations_text()
    match = re.search(
        r"(?ms)^## Flatkey staging browser QA first-run and recovery runbook\n"
        r"(?P<body>.*?)(?=^## [^\n]+\n|\Z)",
        text,
    )
    if not match:
        raise AssertionError("Browser QA runbook section not found")
    return match.group("body")


def fenced_blocks(text):
    return re.findall(r"(?ms)^```(?:bash|sh)?\n(.*?)^```", text)


def heading_section(text, heading):
    match = re.search(
        rf"(?ms)^### {re.escape(heading)}\n(?P<body>.*?)(?=^### \d+\. |\Z)",
        text,
    )
    if not match:
        raise AssertionError(f"section not found: {heading}")
    return match.group("body")


def output_backed_section():
    return heading_section(browser_qa_section(), "2. Set output-backed GitHub repository variables")


def output_backed_command_block():
    blocks = fenced_blocks(output_backed_section())
    if len(blocks) != 1:
        raise AssertionError(f"expected one output-backed command block, found {len(blocks)}")
    return blocks[0]


def plan_review_command_block():
    section = heading_section(browser_qa_section(), "1. Authenticated refreshing Terraform plan review")
    blocks = fenced_blocks(section)
    if len(blocks) != 1:
        raise AssertionError(f"expected one plan review command block, found {len(blocks)}")
    return blocks[0]


class BrowserQaOperationsContractTests(unittest.TestCase):
    def test_project_environment_lists_prod_and_browser_qa_roots_and_state_prefixes(self):
        text = operations_text()
        project_environment = re.search(
            r"(?ms)^## Project / environment\n(?P<body>.*?)(?=^## [^\n]+\n|\Z)",
            text,
        )
        if not project_environment:
            raise AssertionError("Project / environment section not found")
        section = project_environment.group("body")

        self.assertIn(PROD_ROOT, section)
        self.assertIn("prefix `envs/prod`", section)
        self.assertIn(BROWSER_QA_ROOT, section)
        self.assertIn(f"prefix `{BROWSER_QA_BACKEND_PREFIX}`", section)

    def test_browser_qa_section_uses_only_independent_terraform_root(self):
        section = browser_qa_section()

        self.assertIn(BROWSER_QA_ROOT, section)
        self.assertIn(f"Backend prefix | `{BROWSER_QA_BACKEND_PREFIX}`", section)
        self.assertNotIn(PROD_ROOT, section)
        self.assertNotIn("enable_browser_qa = true", section)

    def test_resource_table_lists_all_non_committed_github_variables(self):
        text = operations_text()
        expected_variables = set(OUTPUT_BACKED_VARIABLES) | {"GCP_BROWSER_QA_GMAIL_BASE"}

        self.assertIn("Non-committed GitHub variable", text)
        for variable in expected_variables:
            with self.subTest(variable=variable):
                self.assertRegex(text, rf"\|\s*[^|\n]*GitHub variable[^|\n]*\|\s*`?{variable}`?")

    def test_output_backed_repo_variables_are_mapped_from_terraform_outputs(self):
        text = operations_text()

        for variable, output in OUTPUT_BACKED_VARIABLES.items():
            with self.subTest(variable=variable):
                self.assertRegex(
                    text,
                    rf"{variable}[\s\S]{{0,240}}terraform output -raw {output}",
                )
                self.assertRegex(
                    text,
                    rf"{variable}[\s\S]{{0,240}}{output}",
                )

    def test_output_backed_section_is_after_apply_and_before_secrets_and_dispatch(self):
        text = operations_text()
        apply_instruction = "Apply only from the same review shell"
        output_heading = "### 2. Set output-backed GitHub repository variables"
        secret_heading = "### 3. Add Secret Manager versions without leaking values"
        dispatch_heading = "### 7. Dispatch core or normal and capture the exact run id"

        for marker in [apply_instruction, output_heading, secret_heading, dispatch_heading]:
            self.assertIn(marker, text)

        self.assertLess(text.index(apply_instruction), text.index(output_heading))
        self.assertLess(text.index(output_heading), text.index(secret_heading))
        self.assertLess(text.index(output_heading), text.index(dispatch_heading))
        self.assertIn("Terraform output", output_backed_section())

    def test_output_backed_repo_variables_are_set_via_stdin_not_argv(self):
        block = output_backed_command_block()
        self.assertNotIn("--body", block)

        for variable, output in OUTPUT_BACKED_VARIABLES.items():
            with self.subTest(variable=variable):
                assignment_pattern = rf'{variable}="\$\(terraform output -raw {output}\)"'
                self.assertRegex(block, assignment_pattern)
                self.assertRegex(block, rf'test -n "\$\{{{variable}\}}"')
                self.assertRegex(block, rf'test "\$\{{{variable}\}}" != "null"')
                self.assertRegex(
                    block,
                    rf'printf \'%s\' "\$\{{{variable}\}}" \| gh variable set {variable}\s+--repo SolveaCX/new-api',
                )
                self.assertNotRegex(block, rf"gh variable set {variable}[^\n]*\$\{{{variable}\}}")

    def test_output_backed_section_anchors_repo_root_before_browser_qa_cd(self):
        block = output_backed_command_block()
        self.assertRegex(block, r'repo_root="\$\(git rev-parse --show-toplevel\)"')
        self.assertRegex(block, rf'cd "\$repo_root/{BROWSER_QA_ROOT}"')
        self.assertLess(
            block.index('repo_root="$(git rev-parse --show-toplevel)"'),
            block.index(f'cd "$repo_root/{BROWSER_QA_ROOT}"'),
        )

    def test_plan_review_uses_fail_closed_preflight_and_independent_state(self):
        section = browser_qa_section()

        self.assertIn('repo_root="$(git rev-parse --show-toplevel)"', section)
        self.assertIn(f'qa_root="$repo_root/{BROWSER_QA_ROOT}"', section)
        self.assertIn("gcloud auth login", section)
        self.assertIn("gcloud auth application-default login", section)
        self.assertLess(section.index("gcloud auth login"), section.index("gcloud services list --enabled"))
        self.assertLess(section.index("gcloud auth application-default login"), section.index("terraform init -reconfigure"))
        self.assertIn('active_project="$(gcloud config get-value project 2>/dev/null)"', section)
        self.assertIn('ABORT: active GCP project must be vocai-gemini-prod', section)
        self.assertIn('active_region="$(gcloud config get-value run/region 2>/dev/null)"', section)
        self.assertIn('ABORT: active run region must be us-west1', section)
        self.assertIn('gcloud services list --enabled', section)
        self.assertIn("--format='value(config.name)'", section)
        self.assertIn("missing_apis", section)
        self.assertIn("comm -23", section)
        self.assertIn("ABORT: required APIs are not enabled", section)
        for api in sorted(BROWSER_QA_REQUIRED_APIS):
            with self.subTest(api=api):
                self.assertIn(api, section)
        self.assertIn('cd "$qa_root"', section)
        self.assertIn("terraform init -reconfigure", section)
        self.assertIn("terraform state list", section)
        self.assertIn("ABORT: independent Browser QA state is not empty", section)
        self.assertLess(section.index("terraform state list"), section.index('describe_absent "Artifact Registry repository'))

    def test_state_gate_handles_absent_state_without_masking_unknown_read_errors(self):
        block = plan_review_command_block()

        self.assertIn('state_stdout="$review_dir/state-list.stdout"', block)
        self.assertIn('state_stderr="$review_dir/state-list.stderr"', block)
        self.assertIn('if terraform state list >"$state_stdout" 2>"$state_stderr"; then', block)
        self.assertIn("state_status=0", block)
        self.assertRegex(block, r"else\s+state_status=\"\$\?\"")
        self.assertNotIn("if ! terraform state list", block)
        self.assertIn('state_addresses="$(cat "$state_stdout")"', block)
        self.assertIn('state_diagnostic="$(cat "$state_stderr" "$state_stdout")"', block)
        self.assertIn("No state file was found!", block)
        self.assertIn("ABORT: unable to read independent Browser QA state", block)
        self.assertIn("ABORT: independent Browser QA state is not empty", block)
        self.assertLess(block.index("terraform state list"), block.index('describe_absent "Artifact Registry repository'))
        self.assertLess(block.index("No state file was found!"), block.index('describe_absent "Artifact Registry repository'))

    def test_absent_state_requires_empty_stdout_before_accepting_no_state_diagnostic(self):
        block = plan_review_command_block()

        self.assertRegex(
            block,
            r'(?ms)if terraform state list >"\$state_stdout" 2>"\$state_stderr"; then\s+'
            r"state_status=0\s+"
            r'state_addresses="\$\(cat "\$state_stdout"\)"',
        )
        self.assertRegex(
            block,
            r'(?ms)\*"No state file was found!"\*\)\s+'
            r'if \[ -s "\$state_stdout" \]; then\s+'
            r'echo "ABORT: unable to read independent Browser QA state" >&2\s+'
            r'printf \'%s\\n\' "\$state_diagnostic" >&2\s+'
            r"exit 1\s+"
            r"fi\s+"
            r'state_addresses=""',
        )

    def test_state_gate_only_allows_exact_terraform_no_state_diagnostic(self):
        block = plan_review_command_block()
        no_state_case = re.search(
            r"(?ms)case \"\$state_diagnostic\" in(?P<body>.*?)esac",
            block,
        )
        if not no_state_case:
            raise AssertionError("state diagnostic case block not found")
        body = no_state_case.group("body")

        self.assertIn("No state file was found!", body)
        for diagnostic in UNKNOWN_STATE_DIAGNOSTICS:
            with self.subTest(diagnostic=diagnostic):
                self.assertNotIn(diagnostic, body)

    def test_live_absence_preflight_lists_resources_and_fails_closed_on_unknown_errors(self):
        section = browser_qa_section()

        for resource_name in sorted(BROWSER_QA_LIVE_RESOURCES):
            with self.subTest(resource_name=resource_name):
                self.assertIn(resource_name, section)
        for command_family in [
            "gcloud artifacts repositories describe",
            "gcloud run services describe",
            "gcloud run jobs describe",
            "gcloud iam service-accounts describe",
            "gcloud secrets describe",
            "gcloud storage buckets describe",
            "gcloud iam workload-identity-pools describe",
            "gcloud iam workload-identity-pools providers describe",
        ]:
            with self.subTest(command_family=command_family):
                self.assertIn(command_family, section)
        self.assertIn("ABORT: unable to prove", section)
        self.assertRegex(section, r"404|NOT_FOUND|does not exist")
        self.assertIn("Stop and design an import/migration before creating Browser QA resources", section)
        self.assertIn("Do not use -target", section)

    def test_live_absence_probe_allows_only_absence_diagnostics(self):
        block = plan_review_command_block()
        describe_function = re.search(
            r"(?ms)^describe_absent\(\) \{\n(?P<body>.*?)^\}\n\n",
            block,
        )
        if not describe_function:
            raise AssertionError("describe_absent function not found")
        function_body = describe_function.group("body")
        case_match = re.search(r"(?ms)case \"\$diagnostic\" in(?P<body>.*?)esac", function_body)
        if not case_match:
            raise AssertionError("describe_absent diagnostic case not found")
        case_body = case_match.group("body")

        for diagnostic in ABSENCE_ONLY_DIAGNOSTICS:
            with self.subTest(diagnostic=diagnostic):
                self.assertIn(diagnostic, case_body)
        for diagnostic in UNKNOWN_ABSENCE_DIAGNOSTICS:
            with self.subTest(diagnostic=diagnostic):
                self.assertNotIn(diagnostic, case_body)
        self.assertIn('*)', case_body)
        self.assertIn("ABORT: unable to prove", case_body)

    def test_live_absence_preflight_binds_each_resource_to_exact_describe(self):
        section = browser_qa_section()

        specific_probes = [
            (
                "Artifact Registry repository flatkey-staging-browser-qa",
                r"gcloud artifacts repositories describe flatkey-staging-browser-qa\s+\\\s+--project=\"\$project_id\" --location=\"\$region\"",
            ),
            (
                "Cloud Run broker service flatkey-staging-browser-qa-broker",
                r"gcloud run services describe flatkey-staging-browser-qa-broker\s+\\\s+--project=\"\$project_id\" --region=\"\$region\"",
            ),
            (
                "Cloud Run job flatkey-staging-browser-qa",
                r"gcloud run jobs describe flatkey-staging-browser-qa\s+\\\s+--project=\"\$project_id\" --region=\"\$region\"",
            ),
            (
                "Cloud Run job flatkey-staging-browser-qa-cleanup",
                r"gcloud run jobs describe flatkey-staging-browser-qa-cleanup\s+\\\s+--project=\"\$project_id\" --region=\"\$region\"",
            ),
            (
                "runtime service account flatkey-browser-qa-runtime@vocai-gemini-prod.iam.gserviceaccount.com",
                r"gcloud iam service-accounts describe flatkey-browser-qa-runtime@vocai-gemini-prod\.iam\.gserviceaccount\.com\s+\\\s+--project=\"\$project_id\"",
            ),
            (
                "broker service account flatkey-browser-qa-broker@vocai-gemini-prod.iam.gserviceaccount.com",
                r"gcloud iam service-accounts describe flatkey-browser-qa-broker@vocai-gemini-prod\.iam\.gserviceaccount\.com\s+\\\s+--project=\"\$project_id\"",
            ),
            (
                "cleanup service account flatkey-browser-qa-cleanup@vocai-gemini-prod.iam.gserviceaccount.com",
                r"gcloud iam service-accounts describe flatkey-browser-qa-cleanup@vocai-gemini-prod\.iam\.gserviceaccount\.com\s+\\\s+--project=\"\$project_id\"",
            ),
            (
                "deployer service account flatkey-browser-qa-deployer@vocai-gemini-prod.iam.gserviceaccount.com",
                r"gcloud iam service-accounts describe flatkey-browser-qa-deployer@vocai-gemini-prod\.iam\.gserviceaccount\.com\s+\\\s+--project=\"\$project_id\"",
            ),
            (
                "Secret container flatkey-browser-qa-codex-api-key",
                r"gcloud secrets describe flatkey-browser-qa-codex-api-key\s+\\\s+--project=\"\$project_id\"",
            ),
            (
                "Secret container flatkey-browser-qa-identity-seed",
                r"gcloud secrets describe flatkey-browser-qa-identity-seed\s+\\\s+--project=\"\$project_id\"",
            ),
            (
                "Secret container flatkey-browser-qa-gmail-oauth",
                r"gcloud secrets describe flatkey-browser-qa-gmail-oauth\s+\\\s+--project=\"\$project_id\"",
            ),
            (
                "GCS bucket gs://vocai-gemini-prod-flatkey-browser-qa-reports",
                r"gcloud storage buckets describe gs://vocai-gemini-prod-flatkey-browser-qa-reports\s+\\\s+--project=\"\$project_id\"",
            ),
            (
                "WIF pool flatkey-browser-qa-github",
                r"gcloud iam workload-identity-pools describe flatkey-browser-qa-github\s+\\\s+--project=\"\$project_id\" --location=global",
            ),
            (
                "WIF provider staging",
                r"gcloud iam workload-identity-pools providers describe staging\s+\\\s+--project=\"\$project_id\" --location=global --workload-identity-pool=flatkey-browser-qa-github",
            ),
        ]
        for label, command_pattern in specific_probes:
            with self.subTest(label=label):
                self.assertRegex(
                    section,
                    rf'describe_absent "{re.escape(label)}"\s+\\\s+{command_pattern}',
                )

    def test_plan_guard_uses_versioned_guard_before_review_and_apply_confirmation(self):
        section = browser_qa_section()

        self.assertIn('terraform plan -out="$plan_path"', section)
        self.assertEqual(section.count('terraform plan -out="$plan_path"'), 1)
        self.assertIn('terraform show -json "$plan_path" > "$plan_json"', section)
        self.assertIn('terraform show -no-color "$plan_path"', section)
        guard = f'python3 "$repo_root/scripts/browser_qa/terraform_plan_guard.py" "$plan_json"'
        self.assertIn(guard, section)
        self.assertNotIn('if "browser_qa" not in address', section)
        self.assertLess(section.index(guard), section.index("Human-readable plan review"))
        self.assertLess(section.index(guard), section.index("APPLY_BROWSER_QA_SAVED_PLAN"))
        self.assertNotRegex(section, r"(?m)^\s*terraform\s+(?:plan|apply)\b[^\n]*\s-target(?:=|\s)")

    def test_recovery_table_names_exact_guard_and_state_live_preflight(self):
        section = browser_qa_section()
        table_start = section.index("### Recovery and abort rules")
        recovery = section[table_start:]

        self.assertIn("versioned Terraform plan guard", recovery)
        self.assertIn("state/live preflight", recovery)
        self.assertNotIn("[0]", recovery)

    def test_output_backed_values_are_all_read_and_validated_before_first_github_write(self):
        block = output_backed_command_block()
        first_write_index = block.index("gh variable set")

        for variable, output in OUTPUT_BACKED_VARIABLES.items():
            with self.subTest(variable=variable):
                assignment = f'{variable}="$(terraform output -raw {output})"'
                nonempty = f'test -n "${{{variable}}}"'
                not_null = f'test "${{{variable}}}" != "null"'
                self.assertIn(assignment, block)
                self.assertIn(nonempty, block)
                self.assertIn(not_null, block)
                self.assertLess(block.index(assignment), first_write_index)
                self.assertLess(block.index(nonempty), first_write_index)
                self.assertLess(block.index(not_null), first_write_index)

    def test_output_backed_section_documents_non_atomic_safe_rerun(self):
        section = output_backed_section()
        self.assertRegex(section, r"(?i)sequential")
        self.assertRegex(section, r"(?i)non-atomic|not atomic")
        self.assertRegex(section, r"(?i)safe to rerun")
        self.assertRegex(section, r"(?i)partial `gh` failure|partial gh failure")

    def test_output_backed_section_does_not_print_or_change_gmail_base(self):
        block = output_backed_command_block()

        self.assertNotRegex(block, r"(?m)^\s*echo\b")
        self.assertNotRegex(block, r"(?m)^\s*printf\b(?! '%s' \"\$\{GCP_BROWSER_QA_[A-Z0-9_]+\}\" \| gh variable set GCP_BROWSER_QA_[A-Z0-9_]+\b)")
        self.assertNotRegex(block, r"(?m)^\s*(?:export\s+)?GCP_BROWSER_QA_GMAIL_BASE=")
        self.assertNotRegex(block, r"(?m)\b(?:gh variable set|unset|read)\s+GCP_BROWSER_QA_GMAIL_BASE\b")
        self.assertNotIn("FLATKEY_QA_GMAIL_BASE", block)

    def test_gmail_base_stays_stdin_only_and_never_uses_body_argument(self):
        text = browser_qa_section()
        self.assertIn("GCP_BROWSER_QA_GMAIL_BASE", text)
        self.assertIn('IFS= read -rsp "Base Gmail address without plus tag: " GCP_BROWSER_QA_GMAIL_BASE', text)
        self.assertRegex(
            text,
            r"gh variable set GCP_BROWSER_QA_GMAIL_BASE\s+\\\s+--repo SolveaCX/new-api\s+\\\s+< \"\$tmp_var\"",
        )
        self.assertNotIn("--body", "\n".join(fenced_blocks(text)))

    def test_output_and_report_lookup_use_independent_root(self):
        section = browser_qa_section()

        self.assertIn(f'terraform -chdir="$repo_root/{BROWSER_QA_ROOT}" output -raw browser_qa_report_bucket', section)
        self.assertIn(f'cd "$repo_root/{BROWSER_QA_ROOT}"', output_backed_command_block())
        self.assertIn('BROKER_URI="$(terraform output -raw browser_qa_broker_uri)"', section)


if __name__ == "__main__":
    unittest.main()
