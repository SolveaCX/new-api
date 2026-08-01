import json
import os
import subprocess
import sys
import tempfile
import unittest
from unittest import mock
import io

from scripts.browser_qa.flatkey_browser_qa import browser_evidence_mcp
from scripts.browser_qa.flatkey_browser_qa import supervisor


class BrowserEvidenceTests(unittest.TestCase):
    def test_evidence_mcp_rejects_invalid_frames_and_extra_screenshot_arguments(self):
        stdin = io.StringIO(
            "\n".join([
                json.dumps("not-object"),
                json.dumps({
                    "jsonrpc": "2.0",
                    "id": "extra",
                    "method": "tools/call",
                    "params": {"name": "qa_capture_screenshot", "arguments": {"name": "checkpoint", "filename": "leak.png"}},
                }),
                json.dumps({
                    "jsonrpc": "2.0",
                    "id": "ok",
                    "method": "tools/call",
                    "params": {"name": "qa_capture_screenshot", "arguments": {"name": "checkpoint"}},
                }),
            ])
            + "\n"
        )
        stdout = io.StringIO()
        with mock.patch.dict(os.environ, {"FLATKEY_BROWSER_QA_RUNTIME_DIR": "runtime", "FLATKEY_BROWSER_QA_CDP_ENDPOINT": "http://127.0.0.1:9222"}, clear=True), \
            mock.patch.object(sys, "stdin", stdin), \
            mock.patch.object(sys, "stdout", stdout), \
            mock.patch.object(browser_evidence_mcp, "_capture_with_node", return_value="screenshots/checkpoint.png") as capture:
            browser_evidence_mcp.main()

        responses = [json.loads(line) for line in stdout.getvalue().splitlines()]
        self.assertEqual(responses[0]["error"]["code"], -32600)
        self.assertEqual(responses[1]["id"], "extra")
        self.assertEqual(responses[1]["error"]["code"], -32602)
        self.assertEqual(responses[2]["id"], "ok")
        self.assertEqual(responses[2]["result"]["content"][0]["text"], "screenshots/checkpoint.png")
        capture.assert_called_once_with("runtime", "http://127.0.0.1:9222", "checkpoint")

    def test_node_helper_contract_rejects_paths_masks_buffer_screenshot_and_projects_events(self):
        helper_test = os.path.join(
            os.path.dirname(__file__),
            "..",
            "flatkey_browser_qa",
            "browser_evidence_helper.test.cjs",
        )
        result = subprocess.run(
            ["node", helper_test],
            cwd=os.path.realpath(os.path.join(os.path.dirname(__file__), "..", "..", "..")),
            text=True,
            capture_output=True,
            timeout=20,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_browser_event_writer_redacts_projects_and_fails_closed_on_unbounded_input(self):
        with tempfile.TemporaryDirectory() as runtime_root:
            browser_dir = os.path.join(runtime_root, "browser")
            raw = {
                "console": [{"type": "log", "text": "code 654321 sk-12345678"}],
                "network": [{
                    "url": "https://staging-console.flatkey.ai/api?token=secret",
                    "method": "POST",
                    "status": 200,
                    "headers": {"cookie": "secret"},
                    "postData": "secret",
                    "timing": {"startTime": 1},
                }],
            }
            redactor = supervisor.Redactor(extra_secrets=("654321",))
            supervisor.write_browser_evidence_artifacts(runtime_root, raw, redactor)

            with open(os.path.join(browser_dir, "console.jsonl"), encoding="utf-8") as handle:
                console_event = json.loads(handle.readline())
            with open(os.path.join(browser_dir, "network.jsonl"), encoding="utf-8") as handle:
                network_event = json.loads(handle.readline())
            self.assertNotIn("654321", json.dumps(console_event))
            self.assertNotIn("sk-12345678", json.dumps(console_event))
            self.assertEqual(set(network_event), {"url", "method", "status", "timing"})
            self.assertNotIn("secret", json.dumps(network_event))

            too_many = {"console": [{"text": "x"}] * (supervisor.MAX_BROWSER_EVIDENCE_EVENTS + 1), "network": []}
            with self.assertRaises(RuntimeError):
                supervisor.write_browser_evidence_artifacts(runtime_root, too_many, redactor)


if __name__ == "__main__":
    unittest.main()
