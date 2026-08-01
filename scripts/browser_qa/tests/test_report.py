import json
import os
import tempfile
import unittest

from scripts.browser_qa.flatkey_browser_qa.cleanup import CleanupResult
from scripts.browser_qa.flatkey_browser_qa import report


def valid_result(**overrides):
    payload = {
        "replay": {"status": "passed", "checkpoint_reached": True},
        "exploration": {"status": "passed", "actions_used": 3},
        "budgets": {"replay_seconds": 300, "exploration_seconds": 300, "max_actions": 30},
        "findings": [],
    }
    payload.update(overrides)
    return payload


def finding(**overrides):
    payload = {
        "severity": "medium",
        "title": "registration accepts weak confirmation state",
        "target_url": "https://staging-console.flatkey.ai/register",
        "steps": ["Open registration", "Submit form"],
        "expected": "The flow should reject the state.",
        "actual": "The flow accepted the state.",
        "evidence_paths": ["artifacts/screenshot.png"],
        "confidence": "high",
    }
    payload.update(overrides)
    return payload


def valid_provenance(**overrides):
    payload = {
        "skill_name": "flatkey-new-user-onboarding",
        "skill_content_sha256": "a" * 64,
        "codex_version": "codex 1.2.3",
        "model_config": {
            "model": "gpt-5.4",
            "sandbox": "workspace-write",
            "network_access": False,
        },
        "playwright_mcp_version": "playwright-mcp 1.0.0",
        "playwright_package_version": "1.50.0",
        "chromium_version": "Chromium 123.0.0.0",
    }
    payload.update(overrides)
    return payload


class ReportTests(unittest.TestCase):
    def test_schema_rejects_missing_extra_invalid_type_enum_and_finding_shape(self):
        with self.assertRaises(report.ResultValidationError):
            report.validate_result({"replay": {}})

        extra = valid_result(extra="not allowed")
        with self.assertRaises(report.ResultValidationError):
            report.validate_result(extra)

        bad_type = valid_result(findings="not a list")
        with self.assertRaises(report.ResultValidationError):
            report.validate_result(bad_type)

        bad_enum = valid_result(findings=[finding(severity="warning")])
        with self.assertRaises(report.ResultValidationError):
            report.validate_result(bad_enum)

        missing_finding_field = valid_result(findings=[{k: v for k, v in finding().items() if k != "actual"}])
        with self.assertRaises(report.ResultValidationError):
            report.validate_result(missing_finding_field)

    def test_status_priority_ignores_info_findings_and_cleanup_overrides_manifest(self):
        self.assertEqual(report.classify_status(valid_result(findings=[finding(severity="info")])), "passed")
        self.assertEqual(report.classify_status(valid_result(findings=[finding(severity="low")])), "findings_detected")
        self.assertEqual(report.classify_status(valid_result(replay={"status": "failed", "checkpoint_reached": False})), "replay_failed")
        self.assertEqual(report.classify_status(valid_result(), runtime_classification="codex_nonzero"), "infrastructure_failed")

        cleanup = CleanupResult(1, True, True, True, "token cleanup failed")
        manifest = report.build_manifest(
            valid_result(),
            cleanup_result=cleanup,
            provenance=valid_provenance(),
            run_id="123456789",
            execution_id="main-001",
        )

        self.assertEqual(manifest["status"], "cleanup_failed")
        self.assertEqual(manifest["cleanup"]["cleanup_failed"], True)
        self.assertEqual(manifest["cleanup"]["reason"], "token cleanup failed")

    def test_model_result_schema_rejects_infrastructure_and_alias_restriction_self_report(self):
        with self.assertRaises(report.ResultValidationError):
            report.validate_result(valid_result(infrastructure={"status": "failed"}))
        with self.assertRaises(report.ResultValidationError):
            report.validate_result(valid_result(alias_restriction=True))

    def test_writes_strict_redacted_report_and_schema_file_is_loadable(self):
        with tempfile.TemporaryDirectory() as tmp:
            result_path = os.path.join(tmp, "result.json")
            manifest_path = os.path.join(tmp, "manifest.json")
            with open(result_path, "w", encoding="utf-8") as handle:
                json.dump(valid_result(findings=[finding(actual="Authorization: Bearer sk-secretSECRET")]), handle)

            manifest = report.write_report(
                result_path,
                manifest_path,
                cleanup_result=CleanupResult(0, False, False, False, "not needed"),
                redactor=report.Redactor(extra_secrets=("sk-secretSECRET",)),
                run_id="123456789",
                execution_id="main-001",
                provenance=valid_provenance(),
            )

            with open(manifest_path, encoding="utf-8") as handle:
                written = json.load(handle)
            self.assertEqual(written, manifest)
            rendered = json.dumps(written)
            self.assertNotIn("sk-secretSECRET", rendered)
            self.assertEqual(written["status"], "findings_detected")
            self.assertEqual(written["schema_version"], 1)
            self.assertEqual(written["kind"], "main")
            self.assertEqual(written["run_id"], "123456789")
            self.assertEqual(written["execution_id"], "main-001")

    def test_report_manifest_identity_is_explicit_and_validated(self):
        cleanup = CleanupResult(0, False, False, False, "not needed")

        with self.assertRaises(TypeError):
            report.build_manifest(valid_result(), cleanup_result=cleanup)

        for run_id, execution_id in [
            ("", "main-001"),
            ("not-decimal", "main-001"),
            ("123456789", ""),
            ("123456789", "../main"),
            ("123456789", "main/001"),
        ]:
            with self.subTest(run_id=run_id, execution_id=execution_id):
                with self.assertRaises(report.ResultValidationError):
                    report.build_manifest(
                        valid_result(),
                        cleanup_result=cleanup,
                        run_id=run_id,
                        execution_id=execution_id,
                        provenance=valid_provenance(),
                    )

        schema_path = os.path.join(os.path.dirname(__file__), "..", "config", "result.schema.json")
        with open(schema_path, encoding="utf-8") as handle:
            schema = json.load(handle)
        self.assertFalse(schema["additionalProperties"])

    def test_schema_declares_same_non_empty_contract_as_python_validator(self):
        schema_path = os.path.join(os.path.dirname(__file__), "..", "config", "result.schema.json")
        with open(schema_path, encoding="utf-8") as handle:
            schema = json.load(handle)
        finding_schema = schema["properties"]["findings"]["items"]["properties"]

        for field in ["title", "target_url", "expected", "actual", "confidence", "severity"]:
            with self.subTest(field=field):
                self.assertEqual(finding_schema[field].get("minLength"), 1)

        for field in ["steps", "evidence_paths"]:
            with self.subTest(field=field):
                self.assertEqual(finding_schema[field].get("minItems"), 1)
                self.assertEqual(finding_schema[field]["items"].get("minLength"), 1)

        for bad_payload in [
            valid_result(findings=[finding(title="")]),
            valid_result(findings=[finding(steps=[])]),
            valid_result(findings=[finding(evidence_paths=[""])]),
        ]:
            with self.subTest(payload=bad_payload):
                with self.assertRaises(report.ResultValidationError):
                    report.validate_result(bad_payload)

    def test_main_manifest_requires_strict_runtime_provenance(self):
        cleanup = CleanupResult(0, False, False, False, "not needed")
        manifest = report.build_manifest(
            valid_result(),
            cleanup_result=cleanup,
            run_id="123456789",
            execution_id="main-001",
            provenance=valid_provenance(),
        )

        self.assertEqual(manifest["provenance"], valid_provenance())

        with self.assertRaises(TypeError):
            report.build_manifest(
                valid_result(),
                cleanup_result=cleanup,
                run_id="123456789",
                execution_id="main-001",
                model_manifest=valid_provenance(),
            )

        malformed_cases = [
            None,
            {**valid_provenance(), "extra": "bad"},
            {k: v for k, v in valid_provenance().items() if k != "skill_content_sha256"},
            valid_provenance(skill_content_sha256="A" * 64),
            valid_provenance(skill_content_sha256="a" * 63),
            valid_provenance(model_config={"model": "gpt-5.4", "sandbox": "workspace-write"}),
            valid_provenance(model_config={"model": "gpt-5.4", "sandbox": "workspace-write", "network_access": True}),
            valid_provenance(codex_version=""),
            valid_provenance(playwright_mcp_version="bad\nversion"),
            valid_provenance(playwright_package_version=""),
            valid_provenance(chromium_version=""),
        ]
        for provenance in malformed_cases:
            with self.subTest(provenance=provenance):
                with self.assertRaises(report.ResultValidationError):
                    report.build_manifest(
                        valid_result(),
                        cleanup_result=cleanup,
                        run_id="123456789",
                        execution_id="main-001",
                        provenance=provenance,
                    )


if __name__ == "__main__":
    unittest.main()
