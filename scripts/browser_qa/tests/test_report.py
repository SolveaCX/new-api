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
        self.assertEqual(report.classify_status(valid_result(infrastructure={"status": "failed"})), "infrastructure_failed")

        cleanup = CleanupResult(1, True, True, True, "token cleanup failed")
        manifest = report.build_manifest(valid_result(), cleanup_result=cleanup, model_manifest={"cleanup": {"cleanup_failed": False}})

        self.assertEqual(manifest["status"], "cleanup_failed")
        self.assertEqual(manifest["cleanup"]["cleanup_failed"], True)
        self.assertEqual(manifest["cleanup"]["reason"], "token cleanup failed")

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
            )

            with open(manifest_path, encoding="utf-8") as handle:
                written = json.load(handle)
            self.assertEqual(written, manifest)
            rendered = json.dumps(written)
            self.assertNotIn("sk-secretSECRET", rendered)
            self.assertEqual(written["status"], "findings_detected")

        schema_path = os.path.join(os.path.dirname(__file__), "..", "config", "result.schema.json")
        with open(schema_path, encoding="utf-8") as handle:
            schema = json.load(handle)
        self.assertFalse(schema["additionalProperties"])


if __name__ == "__main__":
    unittest.main()
