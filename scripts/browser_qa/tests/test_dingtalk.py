import io
import json
import os
import unittest
import urllib.error
from contextlib import redirect_stdout
from unittest import mock

try:
    from scripts.browser_qa.flatkey_browser_qa import dingtalk
except ImportError:
    dingtalk = None


WEBHOOK = "https://oapi.dingtalk.com/robot/send?access_token=super-secret-webhook-token"


class FakeResponse:
    def __init__(self, status, payload):
        self.status = status
        self.payload = payload

    def read(self, _limit=-1):
        if isinstance(self.payload, bytes):
            return self.payload
        return json.dumps(self.payload).encode("utf-8")

    def __enter__(self):
        return self

    def __exit__(self, *_):
        return False


class RecordingOpener:
    def __init__(self, responses):
        self.responses = list(responses)
        self.requests = []

    def open(self, request, timeout=0):
        self.requests.append((request, timeout))
        response = self.responses.pop(0)
        if isinstance(response, Exception):
            raise response
        return response


class DingTalkModuleContractTests(unittest.TestCase):
    def test_dingtalk_module_is_available(self):
        self.assertIsNotNone(dingtalk, "DingTalk notification module is missing")


@unittest.skipIf(dingtalk is None, "DingTalk notification module is not implemented yet")
class DingTalkTests(unittest.TestCase):
    def report(self, **overrides):
        values = {
            "final_status": "passed",
            "replay_status": "passed",
            "exploration_status": "not_started",
            "exploration_actions": 0,
            "finding_count": 0,
            "cleanup_status": "passed",
            "github_run_url": "https://github.com/SolveaCX/new-api/actions/runs/12345",
            "gcs_uri": "gs://vocai-gemini-prod-flatkey-browser-qa-reports/runs/12345/manifest.json",
        }
        values.update(overrides)
        return dingtalk.DingTalkReport(**values)

    def test_report_renders_only_the_closed_sanitized_contract(self):
        report = self.report()

        markdown = report.markdown()
        payload = report.payload()

        self.assertEqual(payload["msgtype"], "markdown")
        self.assertEqual(payload["markdown"]["title"], "Staging Browser QA PASSED")
        self.assertEqual(payload["markdown"]["text"], markdown)
        for expected in [
            "Final status: `passed`",
            "Replay status: `passed`",
            "Exploration status: `not_started`",
            "Exploration actions: `0`",
            "Finding count: `0`",
            "Cleanup status: `passed`",
            "https://github.com/SolveaCX/new-api/actions/runs/12345",
            "gs://vocai-gemini-prod-flatkey-browser-qa-reports/runs/12345/manifest.json",
        ]:
            self.assertIn(expected, markdown)
        for forbidden in ["gmail", "password", "verification", "api key", "cookie", "authorization", "super-secret"]:
            self.assertNotIn(forbidden, markdown.lower())

    def test_report_rejects_untrusted_status_counts_and_urls(self):
        invalid_overrides = [
            {"final_status": "passed\npassword=leak"},
            {"replay_status": "unknown-status"},
            {"exploration_status": "running"},
            {"cleanup_status": "deleted"},
            {"exploration_actions": -1},
            {"exploration_actions": True},
            {"finding_count": "0"},
            {"github_run_url": "https://evil.example/actions/runs/12345"},
            {"gcs_uri": "https://storage.googleapis.com/report"},
        ]
        for override in invalid_overrides:
            with self.subTest(override=override):
                with self.assertRaises(ValueError):
                    self.report(**override)

    def test_delivery_retries_transient_failures_and_requires_errcode_zero(self):
        opener = RecordingOpener(
            [
                urllib.error.URLError("temporary timeout"),
                urllib.error.HTTPError(WEBHOOK, 503, "busy", {}, io.BytesIO(b"{}")),
                FakeResponse(200, {"errcode": 0, "errmsg": "ok"}),
            ]
        )
        delays = []

        dingtalk.send_report(WEBHOOK, self.report(), opener=opener, sleeper=delays.append)

        self.assertEqual(len(opener.requests), 3)
        self.assertEqual(delays, [1, 2])
        request, timeout = opener.requests[-1]
        self.assertEqual(request.full_url, WEBHOOK)
        self.assertEqual(request.get_method(), "POST")
        self.assertEqual(request.headers["Content-type"], "application/json")
        self.assertEqual(timeout, 10)
        sent_payload = json.loads(request.data)
        self.assertEqual(sent_payload, self.report().payload())
        self.assertNotIn(WEBHOOK, json.dumps(sent_payload))

    def test_delivery_fails_closed_without_echoing_webhook_or_remote_error(self):
        opener = RecordingOpener(
            [FakeResponse(200, {"errcode": 310000, "errmsg": "super-secret-webhook-token keyword rejected"})]
        )

        with self.assertRaises(dingtalk.DingTalkDeliveryError) as raised:
            dingtalk.send_report(WEBHOOK, self.report(), opener=opener, sleeper=lambda _: None)

        message = str(raised.exception)
        self.assertNotIn(WEBHOOK, message)
        self.assertNotIn("super-secret-webhook-token", message)
        self.assertEqual(len(opener.requests), 1)

    def test_main_reads_environment_and_prints_only_delivery_marker(self):
        env = {
            "DINGTALK_WEBHOOK": WEBHOOK,
            "BROWSER_QA_FINAL_STATUS": "replay_failed",
            "BROWSER_QA_REPLAY_STATUS": "failed",
            "BROWSER_QA_EXPLORATION_STATUS": "not_started",
            "BROWSER_QA_EXPLORATION_ACTIONS": "0",
            "BROWSER_QA_FINDING_COUNT": "1",
            "BROWSER_QA_CLEANUP_STATUS": "passed",
            "BROWSER_QA_GITHUB_RUN_URL": "https://github.com/SolveaCX/new-api/actions/runs/12345",
            "BROWSER_QA_GCS_URI": "gs://vocai-gemini-prod-flatkey-browser-qa-reports/runs/12345/manifest.json",
        }
        stdout = io.StringIO()
        with mock.patch.dict(os.environ, env, clear=True):
            with mock.patch.object(dingtalk, "send_report") as send:
                with redirect_stdout(stdout):
                    exit_code = dingtalk.main()

        self.assertEqual(exit_code, 0)
        send.assert_called_once()
        webhook, report = send.call_args.args
        self.assertEqual(webhook, WEBHOOK)
        self.assertEqual(report.final_status, "replay_failed")
        self.assertEqual(report.finding_count, 1)
        self.assertEqual(stdout.getvalue(), "DINGTALK_NOTIFICATION_SENT\n")
        self.assertNotIn(WEBHOOK, stdout.getvalue())


if __name__ == "__main__":
    unittest.main()
