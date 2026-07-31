import io
import json
import os
import tempfile
import unittest
import urllib.error

from scripts.browser_qa.flatkey_browser_qa import broker_mcp
from scripts.browser_qa.flatkey_browser_qa import control_mcp


class FakeResponse:
    def __init__(self, status, payload, headers=None):
        self.status = status
        self.payload = payload
        self.headers = headers or {}

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


class FakeClock:
    def __init__(self):
        self.now = 100.0
        self.sleeps = []

    def monotonic(self):
        return self.now

    def sleep(self, seconds):
        self.sleeps.append(seconds)
        self.now += seconds


def frames(*payloads):
    return "".join(json.dumps(payload) + "\n" for payload in payloads)


def decode_frames(raw):
    if not raw:
        return []
    return [json.loads(line) for line in raw.splitlines()]


class BrokerMcpTests(unittest.TestCase):
    def env(self):
        return {
            "FLATKEY_BROWSER_QA_RUN_ID": "1700000000",
            "FLATKEY_BROWSER_QA_EMAIL_TAG": "flatkey-qa-1700000000-abc123def4",
            "FLATKEY_BROWSER_QA_START_TIME": "1700000000",
            "FLATKEY_BROWSER_QA_BROKER_URL": "https://flatkey-staging-browser-qa-broker-abc-uw.a.run.app/",
        }

    def run_server(self, requests, opener=None, clock=None, env=None):
        stdin = io.StringIO(frames(*requests))
        stdout = io.StringIO()
        broker_mcp.run(stdin, stdout, env=env or self.env(), opener=opener or RecordingOpener([]), clock=clock or FakeClock())
        return decode_frames(stdout.getvalue())

    def test_initialize_lifecycle_tool_list_and_notifications(self):
        responses = self.run_server(
            [
                {"jsonrpc": "2.0", "id": "init-1", "method": "initialize", "params": {"protocolVersion": "2025-06-18"}},
                {"jsonrpc": "2.0", "method": "notifications/initialized"},
                {"jsonrpc": "2.0", "id": 22, "method": "tools/list"},
            ]
        )

        self.assertEqual([response["id"] for response in responses], ["init-1", 22])
        self.assertEqual(responses[0]["result"]["protocolVersion"], "2025-06-18")
        tools = responses[1]["result"]["tools"]
        self.assertEqual([tool["name"] for tool in tools], ["get_current_verification_code"])
        self.assertEqual(
            tools[0]["inputSchema"],
            {"type": "object", "properties": {}, "additionalProperties": False},
        )

    def test_broker_tool_rejects_any_arguments_missing_unknown_or_malformed_requests(self):
        responses = self.run_server(
            [
                {"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "get_current_verification_code"}},
                {
                    "jsonrpc": "2.0",
                    "id": 2,
                    "method": "tools/call",
                    "params": {"name": "get_current_verification_code", "arguments": {"mailbox": "attacker"}},
                },
                {"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {"name": "unknown", "arguments": {}}},
                {"jsonrpc": "2.0", "id": 4, "method": "unknown"},
            ]
        )

        for response in responses:
            self.assertIn("error", response)
            self.assertIn(response["error"]["code"], {-32601, -32602})
            self.assertNotIn("flatkey-qa-1700000000-abc123def4", json.dumps(response))

        stdin = io.StringIO("{not-json}\n")
        stdout = io.StringIO()
        broker_mcp.run(stdin, stdout, env=self.env(), opener=RecordingOpener([]), clock=FakeClock())
        malformed = decode_frames(stdout.getvalue())
        self.assertEqual(malformed[0]["error"]["code"], -32700)

        stdin = io.StringIO(json.dumps({"jsonrpc": "2.0", "id": 9, "method": "tools/list"}) + ("x" * 1048577) + "\n")
        stdout = io.StringIO()
        broker_mcp.run(stdin, stdout, env=self.env(), opener=RecordingOpener([]), clock=FakeClock(), max_line_bytes=1024)
        oversized = decode_frames(stdout.getvalue())
        self.assertEqual(oversized[0]["error"]["code"], -32700)

    def test_broker_env_http_metadata_and_polling_contract(self):
        opener = RecordingOpener(
            [
                FakeResponse(200, b"id-token", {"Metadata-Flavor": "Google"}),
                FakeResponse(200, {"status": "pending"}),
                FakeResponse(200, b"id-token-2", {"Metadata-Flavor": "Google"}),
                FakeResponse(200, {"status": "ready", "code": "654321"}),
            ]
        )
        clock = FakeClock()

        responses = self.run_server(
            [
                {
                    "jsonrpc": "2.0",
                    "id": "call",
                    "method": "tools/call",
                    "params": {"name": "get_current_verification_code", "arguments": {}},
                }
            ],
            opener=opener,
            clock=clock,
        )

        result_text = responses[0]["result"]["content"][0]["text"]
        self.assertEqual(json.loads(result_text), {"status": "ready", "code": "654321"})
        self.assertEqual(clock.sleeps, [5])
        metadata_request = opener.requests[0][0]
        self.assertEqual(metadata_request.host, "metadata.google.internal")
        self.assertIn("audience=https%3A%2F%2Fflatkey-staging-browser-qa-broker-abc-uw.a.run.app", metadata_request.selector)
        broker_request = opener.requests[1][0]
        self.assertEqual(broker_request.full_url, "https://flatkey-staging-browser-qa-broker-abc-uw.a.run.app/v1/current-code")
        self.assertEqual(broker_request.headers["Authorization"], "Bearer id-token")
        body = json.loads(broker_request.data.decode("utf-8"))
        self.assertEqual(body["run_id"], "1700000000")
        self.assertEqual(body["email_tag"], "flatkey-qa-1700000000-abc123def4")
        self.assertEqual(body["start_time"], 1700000000)

    def test_broker_env_redirect_and_deadline_fail_closed_without_secrets(self):
        bad_env = dict(self.env(), FLATKEY_BROWSER_QA_BROKER_URL="https://example.com/")
        responses = self.run_server(
            [
                {
                    "jsonrpc": "2.0",
                    "id": 1,
                    "method": "tools/call",
                    "params": {"name": "get_current_verification_code", "arguments": {}},
                }
            ],
            env=bad_env,
        )
        self.assertTrue(responses[0]["result"]["isError"])

        opener = RecordingOpener(
            [
                FakeResponse(200, b"id-token", {"Metadata-Flavor": "Google"}),
                urllib.error.HTTPError("https://flatkey-staging-browser-qa-broker-abc-uw.a.run.app/v1/current-code", 302, "redirect", {}, None),
            ]
        )
        responses = self.run_server(
            [
                {
                    "jsonrpc": "2.0",
                    "id": 2,
                    "method": "tools/call",
                    "params": {"name": "get_current_verification_code", "arguments": {}},
                }
            ],
            opener=opener,
            clock=FakeClock(),
        )
        rendered = json.dumps(responses[0])
        self.assertTrue(responses[0]["result"]["isError"])
        self.assertNotIn("abc123def4", rendered)

        pending_pairs = []
        for _ in range(25):
            pending_pairs.append(FakeResponse(200, b"id-token", {"Metadata-Flavor": "Google"}))
            pending_pairs.append(FakeResponse(200, {"status": "pending"}))
        responses = self.run_server(
            [
                {
                    "jsonrpc": "2.0",
                    "id": 3,
                    "method": "tools/call",
                    "params": {"name": "get_current_verification_code", "arguments": {}},
                }
            ],
            opener=RecordingOpener(pending_pairs),
            clock=FakeClock(),
        )
        self.assertTrue(responses[0]["result"]["isError"])
        self.assertIn("deadline", responses[0]["result"]["content"][0]["text"])


class ControlMcpTests(unittest.TestCase):
    def run_server(self, requests, runtime_dir):
        stdin = io.StringIO(frames(*requests))
        stdout = io.StringIO()
        control_mcp.run(stdin, stdout, env={"FLATKEY_BROWSER_QA_RUNTIME_DIR": runtime_dir})
        return decode_frames(stdout.getvalue())

    def test_control_exposes_only_checkpoint_and_exploration_and_writes_atomic_state(self):
        with tempfile.TemporaryDirectory() as runtime_dir:
            responses = self.run_server(
                [
                    {"jsonrpc": "2.0", "id": 1, "method": "tools/list"},
                    {
                        "jsonrpc": "2.0",
                        "id": 2,
                        "method": "tools/call",
                        "params": {"name": "qa_replay_checkpoint", "arguments": {}},
                    },
                    {
                        "jsonrpc": "2.0",
                        "id": 3,
                        "method": "tools/call",
                        "params": {"name": "qa_start_exploration", "arguments": {}},
                    },
                ],
                runtime_dir,
            )

            self.assertEqual([tool["name"] for tool in responses[0]["result"]["tools"]], ["qa_replay_checkpoint", "qa_start_exploration"])
            self.assertFalse(responses[1]["result"]["isError"])
            self.assertFalse(responses[2]["result"]["isError"])
            state_path = os.path.join(runtime_dir, "control_state.json")
            with open(state_path, encoding="utf-8") as state_file:
                state = json.load(state_file)
            self.assertEqual(state["phase"], "exploration")
            leftovers = [name for name in os.listdir(runtime_dir) if name.startswith(".control_state.")]
            self.assertEqual(leftovers, [])

    def test_control_state_path_stays_in_runtime_dir_and_fails_closed_on_symlink_or_bad_env(self):
        responses = self.run_server(
            [
                {
                    "jsonrpc": "2.0",
                    "id": 1,
                    "method": "tools/call",
                    "params": {"name": "qa_replay_checkpoint", "arguments": {}},
                }
            ],
            os.path.join(tempfile.gettempdir(), "missing-runtime-dir-for-mcp-test"),
        )
        self.assertTrue(responses[0]["result"]["isError"])

        with tempfile.TemporaryDirectory() as runtime_dir:
            state_path = os.path.join(runtime_dir, "control_state.json")
            try:
                os.symlink(os.path.join(tempfile.gettempdir(), "outside-state.json"), state_path)
            except (OSError, NotImplementedError):
                self.skipTest("symlink creation unavailable")
            responses = self.run_server(
                [
                    {
                        "jsonrpc": "2.0",
                        "id": 2,
                        "method": "tools/call",
                        "params": {"name": "qa_replay_checkpoint", "arguments": {}},
                    }
                ],
                runtime_dir,
            )
            self.assertTrue(responses[0]["result"]["isError"])


if __name__ == "__main__":
    unittest.main()
