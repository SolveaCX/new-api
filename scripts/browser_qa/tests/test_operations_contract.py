import pathlib
import re
import unittest


REPO_ROOT = pathlib.Path(__file__).resolve().parents[3]
OPERATIONS = REPO_ROOT / "deploy" / "gcp" / "docs" / "OPERATIONS.md"


OUTPUT_BACKED_VARIABLES = {
    "GCP_BROWSER_QA_AR_REPO_URL": "browser_qa_artifact_registry_url",
    "GCP_BROWSER_QA_WIF_PROVIDER": "browser_qa_wif_provider",
    "GCP_BROWSER_QA_DEPLOYER_SA": "browser_qa_deployer_sa_email",
    "GCP_BROWSER_QA_GCS_BUCKET": "browser_qa_report_bucket",
}


def operations_text():
    return OPERATIONS.read_text(encoding="utf-8")


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
    return heading_section(operations_text(), "2. Set output-backed GitHub repository variables")


def output_backed_command_block():
    blocks = fenced_blocks(output_backed_section())
    if len(blocks) != 1:
        raise AssertionError(f"expected one output-backed command block, found {len(blocks)}")
    return blocks[0]


class BrowserQaOperationsContractTests(unittest.TestCase):
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

    def test_output_backed_section_anchors_repo_root_before_prod_cd(self):
        block = output_backed_command_block()
        self.assertRegex(block, r'repo_root="\$\(git rev-parse --show-toplevel\)"')
        self.assertRegex(block, r'cd "\$repo_root/deploy/gcp/envs/prod"')
        self.assertLess(block.index('repo_root="$(git rev-parse --show-toplevel)"'), block.index('cd "$repo_root/deploy/gcp/envs/prod"'))

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
        text = operations_text()
        self.assertIn("GCP_BROWSER_QA_GMAIL_BASE", text)
        self.assertIn('IFS= read -rsp "Base Gmail address without plus tag: " GCP_BROWSER_QA_GMAIL_BASE', text)
        self.assertRegex(
            text,
            r"gh variable set GCP_BROWSER_QA_GMAIL_BASE\s+\\\s+--repo SolveaCX/new-api\s+\\\s+< \"\$tmp_var\"",
        )
        self.assertNotIn("--body", "\n".join(fenced_blocks(text)))


if __name__ == "__main__":
    unittest.main()
