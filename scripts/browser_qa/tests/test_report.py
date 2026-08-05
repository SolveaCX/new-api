import json
import os
import tempfile
import unittest
from unittest import mock

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
    def test_normalize_findings_does_not_downgrade_first_party_denied_target_host(self):
        with tempfile.TemporaryDirectory() as tmp:
            browser_dir = os.path.join(tmp, "browser")
            os.makedirs(browser_dir)
            with open(os.path.join(browser_dir, "network.jsonl"), "w", encoding="utf-8") as handle:
                handle.write("{}\n")
            first_party = finding(
                target_url="https://staging-console.flatkey.ai/register",
                actual="The staging console could not be reached through the QA proxy.",
                evidence_paths=["browser/network.jsonl"],
            )
            third_party = finding(
                title="analytics dependency reported blocked",
                target_url="https://staging-console.flatkey.ai/register",
                actual="The page reported that mixpanel.example was blocked.",
                evidence_paths=["browser/network.jsonl"],
            )

            normalized = report.normalize_findings(
                valid_result(findings=[first_party, third_party]),
                runtime_root=tmp,
                proxy_events=[
                    {"host": "staging-console.flatkey.ai:443", "status": 403, "reason": "denied"},
                    {"host": "mixpanel.example:443", "status": 403, "reason": "denied"},
                ],
            )

            self.assertEqual(normalized["findings"][0]["severity"], "medium")
            self.assertEqual(normalized["findings"][1]["severity"], "info")

    def test_normalize_findings_treats_oversized_evidence_as_unusable_without_unbounded_read(self):
        class GuardedReader:
            def __init__(self, handle, max_read_size):
                self.handle = handle
                self.max_read_size = max_read_size

            def __enter__(self):
                self.handle.__enter__()
                return self

            def __exit__(self, *args):
                return self.handle.__exit__(*args)

            def read(self, size=-1):
                if size < 0 or size > self.max_read_size:
                    raise AssertionError("evidence reader attempted an unbounded read")
                return self.handle.read(size)

            def readline(self, size=-1):
                if size < 0 or size > self.max_read_size:
                    raise AssertionError("evidence reader attempted an unbounded line read")
                return self.handle.readline(size)

        with tempfile.TemporaryDirectory() as tmp:
            screenshot_dir = os.path.join(tmp, "screenshots")
            os.makedirs(screenshot_dir)
            screenshot_path = os.path.join(screenshot_dir, "large.png")
            with open(screenshot_path, "wb") as handle:
                handle.write(b"x" * 17)

            original_open = open

            def guarded_open(path, mode="r", *args, **kwargs):
                handle = original_open(path, mode, *args, **kwargs)
                if os.path.realpath(path) == os.path.realpath(screenshot_path) and "b" in mode:
                    return GuardedReader(handle, 16)
                return handle

            with (
                mock.patch.object(report, "MAX_EVIDENCE_FILE_BYTES", 16, create=True),
                mock.patch("builtins.open", guarded_open),
            ):
                normalized = report.normalize_findings(
                    valid_result(findings=[finding(evidence_paths=["screenshots/large.png"])]),
                    runtime_root=tmp,
                    proxy_events=[],
                )

            self.assertEqual(normalized["findings"][0]["severity"], "info")

    def test_normalize_findings_requires_existing_bounded_runtime_evidence(self):
        with tempfile.TemporaryDirectory() as tmp:
            payload = valid_result(findings=[finding(evidence_paths=["screenshots/missing.png"])])

            normalized = report.normalize_findings(payload, runtime_root=tmp, proxy_events=[])

            self.assertEqual(normalized["findings"][0]["severity"], "info")

            outside = tempfile.NamedTemporaryFile(delete=False)
            outside.close()
            self.addCleanup(lambda: os.path.exists(outside.name) and os.remove(outside.name))
            for evidence_path in [outside.name, "../outside.png", "result.json", "browser/unknown.jsonl"]:
                with self.subTest(evidence_path=evidence_path):
                    normalized = report.normalize_findings(
                        valid_result(findings=[finding(evidence_paths=[evidence_path])]),
                        runtime_root=tmp,
                        proxy_events=[],
                    )
                    self.assertEqual(normalized["findings"][0]["severity"], "info")

    def test_normalize_findings_strips_query_fragment_and_deduplicates_by_evidence_fingerprint(self):
        with tempfile.TemporaryDirectory() as tmp:
            screenshot_dir = os.path.join(tmp, "screenshots")
            os.makedirs(screenshot_dir)
            for name in ["proof-a.png", "proof-b.png"]:
                with open(os.path.join(screenshot_dir, name), "wb") as handle:
                    handle.write(b"same-evidence")
            first = finding(
                target_url="https://staging-console.flatkey.ai/register?email=secret#form",
                evidence_paths=["screenshots/proof-a.png"],
            )
            duplicate = finding(
                target_url="https://staging-console.flatkey.ai/register?other=value#other",
                evidence_paths=["screenshots/proof-b.png"],
            )

            normalized = report.normalize_findings(
                valid_result(findings=[first, duplicate]),
                runtime_root=tmp,
                proxy_events=[],
            )

            self.assertEqual(len(normalized["findings"]), 1)
            self.assertEqual(
                normalized["findings"][0]["target_url"],
                "https://staging-console.flatkey.ai/register",
            )

    def test_normalize_findings_downgrades_runtime_denied_third_party_noise(self):
        with tempfile.TemporaryDirectory() as tmp:
            browser_dir = os.path.join(tmp, "browser")
            os.makedirs(browser_dir)
            with open(os.path.join(browser_dir, "network.jsonl"), "w", encoding="utf-8") as handle:
                handle.write("{}\n")
            payload = valid_result(findings=[finding(
                actual="The page reported that mixpanel.example was blocked.",
                evidence_paths=["browser/network.jsonl"],
            )])

            normalized = report.normalize_findings(
                payload,
                runtime_root=tmp,
                proxy_events=[{"host": "mixpanel.example:443", "status": 403, "reason": "denied"}],
            )

            self.assertEqual(normalized["findings"][0]["severity"], "info")
            self.assertEqual(report.classify_status(normalized), "passed")

    def test_normalize_findings_keeps_evidence_backed_same_origin_product_failure_actionable(self):
        with tempfile.TemporaryDirectory() as tmp:
            browser_dir = os.path.join(tmp, "browser")
            os.makedirs(browser_dir)
            with open(os.path.join(browser_dir, "network.jsonl"), "w", encoding="utf-8") as handle:
                handle.write("{}\n")
            payload = valid_result(findings=[finding(
                target_url="https://staging-console.flatkey.ai/register",
                actual="The staging registration API returned 500.",
                evidence_paths=["browser/network.jsonl"],
            )])

            normalized = report.normalize_findings(
                payload,
                runtime_root=tmp,
                proxy_events=[{"host": "app.solvea.cx", "status": 403, "reason": "denied"}],
            )

            self.assertEqual(normalized["findings"][0]["severity"], "medium")
            self.assertEqual(report.classify_status(normalized), "findings_detected")

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

    def test_manifest_preserves_deleted_token_count_integer_without_leaking_unsafe_values(self):
        redactor = report.Redactor(extra_secrets=("unsafe-token-count",))
        manifest = report.build_manifest(
            valid_result(),
            cleanup_result=CleanupResult(205, True, True, False, "cleanup verified"),
            provenance=valid_provenance(),
            run_id="123456789",
            execution_id="main-001",
            redactor=redactor,
        )

        self.assertEqual(manifest["cleanup"]["deleted_token_count"], 205)
        self.assertIs(type(manifest["cleanup"]["deleted_token_count"]), int)
        self.assertEqual(
            redactor.clean({"deleted_token_count": "unsafe-token-count"}),
            {"deleted_token_count": "[REDACTED_SECRET]"},
        )
        self.assertEqual(
            redactor.clean({"deleted-token-count": 205}),
            {"deleted-token-count": "[REDACTED_SECRET]"},
        )

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
