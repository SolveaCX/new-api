import re
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[3]
QA_ROOT = REPO_ROOT / "deploy" / "gcp" / "envs" / "browser-qa-staging"
PROD_ROOT = REPO_ROOT / "deploy" / "gcp" / "envs" / "prod"

EXPECTED_PROJECT = "vocai-gemini-prod"
EXPECTED_REGION = "us-west1"
EXPECTED_BUCKET = "vocai-gemini-prod-newapi-tfstate"
EXPECTED_PREFIX = "envs/browser-qa-staging"


def _read(path):
    return path.read_text(encoding="utf-8")


def _strip_comments(text):
    result = []
    i = 0
    in_string = False
    in_line_comment = False
    in_block_comment = False
    while i < len(text):
        current = text[i]
        nxt = text[i + 1] if i + 1 < len(text) else ""
        if in_line_comment:
            if current == "\n":
                in_line_comment = False
                result.append(current)
            i += 1
            continue
        if in_block_comment:
            if current == "*" and nxt == "/":
                in_block_comment = False
                i += 2
            else:
                if current == "\n":
                    result.append("\n")
                i += 1
            continue
        if not in_string and current == "/" and nxt == "/":
            in_line_comment = True
            i += 2
            continue
        if not in_string and current == "#":
            in_line_comment = True
            i += 1
            continue
        if not in_string and current == "/" and nxt == "*":
            in_block_comment = True
            i += 2
            continue
        if current == '"' and (i == 0 or text[i - 1] != "\\"):
            in_string = not in_string
        result.append(current)
        i += 1
    return "".join(result)


def _find_matching_brace(text, open_index):
    depth = 0
    in_string = False
    for i in range(open_index, len(text)):
        current = text[i]
        if current == '"' and (i == 0 or text[i - 1] != "\\"):
            in_string = not in_string
        if in_string:
            continue
        if current == "{":
            depth += 1
        elif current == "}":
            depth -= 1
            if depth == 0:
                return i
    raise AssertionError("unclosed HCL block")


def _named_blocks(text, kind):
    clean = _strip_comments(text)
    pattern = re.compile(rf'(?m)^\s*{kind}\s+"([^"]+)"\s*\{{')
    blocks = {}
    for match in pattern.finditer(clean):
        close = _find_matching_brace(clean, match.end() - 1)
        blocks[match.group(1)] = clean[match.end():close]
    return blocks


def _assigned_strings(text):
    clean = _strip_comments(text)
    return dict(re.findall(r'(?m)^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*"([^"]*)"\s*$', clean))


class BrowserQaTerraformStateIsolationContractTest(unittest.TestCase):
    def setUp(self):
        self.assertTrue(PROD_ROOT.exists(), "prod root must exist as the isolation comparison root")
        self.assertTrue(QA_ROOT.exists(), "browser QA staging Terraform root must exist")
        self.tf_files = sorted(QA_ROOT.glob("*.tf"))
        self.assertTrue(self.tf_files, "browser QA staging Terraform root must contain Terraform files")
        self.all_tf = "\n".join(_read(path) for path in self.tf_files)

    def test_backend_uses_dedicated_browser_qa_state_prefix(self):
        backend = _read(QA_ROOT / "backend.tf")
        values = _assigned_strings(backend)

        self.assertEqual(values.get("bucket"), EXPECTED_BUCKET)
        self.assertEqual(values.get("prefix"), EXPECTED_PREFIX)
        self.assertNotEqual(values.get("prefix"), "envs/prod")
        self.assertIn("envs/prod", _read(PROD_ROOT / "backend.tf"))

    def test_variables_accept_only_locked_non_sensitive_project_and_region(self):
        variables = _read(QA_ROOT / "variables.tf")
        blocks = _named_blocks(variables, "variable")

        self.assertEqual(set(blocks), {"project_id", "region"})
        expected_values = {"project_id": EXPECTED_PROJECT, "region": EXPECTED_REGION}
        for name, value in expected_values.items():
            with self.subTest(variable=name):
                body = blocks[name]
                self.assertRegex(body, r"\btype\s*=\s*string\b")
                self.assertRegex(body, rf"\bcondition\s*=\s*var\.{name}\s*==\s*\"{re.escape(value)}\"")
                self.assertRegex(body, r"\berror_message\s*=\s*\"[^\"]+\"")

        self.assertNotRegex(variables, r"(?i)\b(secret|token|oauth|gmail|api_key)\b")

    def test_tfvars_contains_only_fixed_project_and_region_values(self):
        tfvars = _read(QA_ROOT / "terraform.tfvars")
        values = _assigned_strings(tfvars)

        self.assertEqual(values, {"project_id": EXPECTED_PROJECT, "region": EXPECTED_REGION})
        self.assertNotRegex(tfvars, r"(?i)\b(secret|token|oauth|gmail|api_key)\b")

    def test_root_does_not_own_shared_project_api_enablement(self):
        self.assertNotRegex(self.all_tf, r'\bresource\s+"google_project_service"\b')
        self.assertNotRegex(self.all_tf, r'\bmodule\s+"apis"\b')


if __name__ == "__main__":
    unittest.main()
