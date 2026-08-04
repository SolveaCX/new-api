import io
import json
import os
import tempfile
import threading
import unittest
import urllib.error

from scripts.browser_qa.flatkey_browser_qa import broker_mcp
from scripts.browser_qa.flatkey_browser_qa import control_mcp
from scripts.browser_qa.flatkey_browser_qa import mcp


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


class NoNewlineStream:
    def __init__(self, text):
        self.text = text
        self.offset = 0
        self.read_sizes = []

    def read(self, size=-1):
        self.read_sizes.append(size)
        if self.offset >= len(self.text):
            return ""
        if size is None or size < 0:
            size = len(self.text) - self.offset
        end = min(len(self.text), self.offset + size)
        chunk = self.text[self.offset:end]
        self.offset = end
        return chunk

    def readline(self, size=-1):
        return self.read(size)


class SignalingWriter(io.StringIO):
    def __init__(self):
        super().__init__()
        self.written = threading.Event()

    def write(self, text):
        result = super().write(text)
        self.written.set()
        return result


class FakeClock:
    def __init__(self):
        self.now = 100.0
        self.sleeps = []

    def monotonic(self):
        return self.now

    def sleep(self, seconds):
        self.sleeps.append(seconds)
        self.now += seconds

    def advance(self, seconds):
        self.now += seconds


class ControlClock:
    def __init__(self):
        self.now = 500.25

    def monotonic(self):
        return self.now

    def time(self):
        return 1700000000


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
            "FLATKEY_BROWSER_QA_EMAIL_TAG": "qa-1700000000-abc123de",
            "FLATKEY_BROWSER_QA_START_TIME": "1700000000",
            "FLATKEY_BROWSER_QA_BROKER_URL": "https://flatkey-staging-browser-qa-broker-abc-uw.a.run.app/",
        }

    def run_server(self, requests, opener=None, clock=None, env=None, evidence_notifier=None):
        stdin = io.StringIO(frames(*requests))
        stdout = io.StringIO()
        broker_mcp.run(
            stdin,
            stdout,
            env=env or self.env(),
            opener=opener or RecordingOpener([]),
            clock=clock or FakeClock(),
            evidence_notifier=evidence_notifier,
        )
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

    def test_mcp_rejects_missing_or_wrong_jsonrpc_version_for_requests_and_notifications(self):
        responses = self.run_server(
            [
                {"id": 1, "method": "tools/list"},
                {"jsonrpc": "1.0", "id": 2, "method": "tools/list"},
                {"method": "notifications/initialized"},
            ]
        )

        self.assertEqual(len(responses), 1)
        self.assertEqual(responses[0]["id"], 1)
        self.assertEqual(responses[0]["error"]["code"], -32600)

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
            self.assertNotIn("qa-1700000000-abc123de", json.dumps(response))

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

    def test_jsonrpc_server_rejects_no_newline_oversized_input_without_unbounded_read(self):
        stdin = NoNewlineStream("x" * 1025)
        stdout = io.StringIO()
        server = mcp.McpServer("test", [])

        mcp.run_jsonrpc_server(stdin, stdout, server, max_line_bytes=1024)

        self.assertEqual(decode_frames(stdout.getvalue())[0]["error"]["code"], -32700)
        self.assertLessEqual(max(stdin.read_sizes), 1025)

    def test_jsonrpc_server_responds_to_complete_pipe_frame_before_eof(self):
        read_fd, write_fd = os.pipe()
        stdin = os.fdopen(read_fd, "r", encoding="utf-8")
        client = os.fdopen(write_fd, "w", encoding="utf-8")
        stdout = SignalingWriter()
        server = mcp.McpServer("test", [])
        thread = threading.Thread(target=mcp.run_jsonrpc_server, args=(stdin, stdout, server), daemon=True)
        thread.start()

        try:
            client.write(frames({"jsonrpc": "2.0", "id": "live", "method": "initialize", "params": {}}))
            client.flush()
            self.assertTrue(stdout.written.wait(timeout=1), "initialize response waited for MCP stdin EOF")
        finally:
            client.close()
            thread.join(timeout=2)
            stdin.close()

        self.assertFalse(thread.is_alive())
        self.assertEqual(decode_frames(stdout.getvalue())[0]["id"], "live")

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
        self.assertEqual(body["email_tag"], "qa-1700000000-abc123de")
        self.assertEqual(body["start_time"], 1700000000)

    def test_broker_tool_rejects_malformed_email_tag_env(self):
        malformed_envs = [
            dict(self.env(), FLATKEY_BROWSER_QA_EMAIL_TAG="qa-1700000000-ABC123DE"),
            dict(self.env(), FLATKEY_BROWSER_QA_EMAIL_TAG="qa-1700000001-abc123de"),
            dict(self.env(), FLATKEY_BROWSER_QA_EMAIL_TAG="flatkey-qa-1700000000-abc123def4"),
        ]

        for env in malformed_envs:
            with self.subTest(email_tag=env["FLATKEY_BROWSER_QA_EMAIL_TAG"]):
                responses = self.run_server(
                    [
                        {
                            "jsonrpc": "2.0",
                            "id": "call",
                            "method": "tools/call",
                            "params": {"name": "get_current_verification_code", "arguments": {}},
                        }
                    ],
                    env=env,
                )

                self.assertTrue(responses[0]["result"]["isError"])

    def test_broker_notifies_actual_code_before_returning_it_to_codex(self):
        opener = RecordingOpener(
            [
                FakeResponse(200, b"id-token", {"Metadata-Flavor": "Google"}),
                FakeResponse(200, {"status": "ready", "code": "654321"}),
            ]
        )
        notifications = []

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
            evidence_notifier=lambda event: notifications.append(event),
        )

        self.assertEqual(notifications, [{"type": "verification_code", "code": "654321"}])
        self.assertEqual(json.loads(responses[0]["result"]["content"][0]["text"])["code"], "654321")

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

    def test_broker_deadline_includes_metadata_and_http_in_flight_time(self):
        class SlowPendingOpener:
            def __init__(self, clock):
                self.clock = clock
                self.requests = []

            def open(self, request, timeout=0):
                remaining = 220.0 - self.clock.monotonic()
                self.requests.append((request.full_url, timeout, remaining))
                self.clock.advance(min(4, timeout))
                if request.host == "metadata.google.internal":
                    return FakeResponse(200, b"id-token", {"Metadata-Flavor": "Google"})
                return FakeResponse(200, {"status": "pending"})

        clock = FakeClock()
        opener = SlowPendingOpener(clock)
        responses = self.run_server(
            [
                {
                    "jsonrpc": "2.0",
                    "id": "deadline",
                    "method": "tools/call",
                    "params": {"name": "get_current_verification_code", "arguments": {}},
                }
            ],
            opener=opener,
            clock=clock,
        )

        self.assertTrue(responses[0]["result"]["isError"])
        self.assertLessEqual(clock.monotonic(), 220.0)
        for _url, timeout, remaining in opener.requests:
            self.assertGreater(remaining, 0)
            self.assertLessEqual(timeout, remaining)


class ControlMcpTests(unittest.TestCase):
    def run_server(self, requests, runtime_dir, clock=None, mode=None):
        stdin = io.StringIO(frames(*requests))
        stdout = io.StringIO()
        env = {"FLATKEY_BROWSER_QA_RUNTIME_DIR": runtime_dir}
        if mode is not None:
            env["FLATKEY_BROWSER_QA_MODE"] = mode
        control_mcp.run(stdin, stdout, env=env, clock=clock)
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
            self.assertIsInstance(state["monotonic_started_at"], float)
            leftovers = [name for name in os.listdir(runtime_dir) if name.startswith(".control_state.")]
            self.assertEqual(leftovers, [])

    def test_control_exploration_state_uses_monotonic_marker_timestamp(self):
        with tempfile.TemporaryDirectory() as runtime_dir:
            clock = ControlClock()
            self.run_server(
                [
                    {
                        "jsonrpc": "2.0",
                        "id": 1,
                        "method": "tools/call",
                        "params": {"name": "qa_start_exploration", "arguments": {}},
                    }
                ],
                runtime_dir,
                clock=clock,
            )

            with open(os.path.join(runtime_dir, "control_state.json"), encoding="utf-8") as state_file:
                state = json.load(state_file)
            self.assertEqual(state["updated_at"], 1700000000)
            self.assertEqual(state["monotonic_started_at"], 500.25)

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

    def test_control_core_mode_rejects_exploration_without_writing_exploration_state(self):
        with tempfile.TemporaryDirectory() as runtime_dir:
            responses = self.run_server(
                [
                    {
                        "jsonrpc": "2.0",
                        "id": 1,
                        "method": "tools/call",
                        "params": {"name": "qa_replay_checkpoint", "arguments": {}},
                    },
                    {
                        "jsonrpc": "2.0",
                        "id": 2,
                        "method": "tools/call",
                        "params": {"name": "qa_start_exploration", "arguments": {}},
                    },
                ],
                runtime_dir,
                mode="core",
            )

            self.assertFalse(responses[0]["result"]["isError"])
            self.assertTrue(responses[1]["result"]["isError"])
            self.assertIn("core", responses[1]["result"]["content"][0]["text"])
            with open(os.path.join(runtime_dir, "control_state.json"), encoding="utf-8") as state_file:
                state = json.load(state_file)
            self.assertEqual(state["phase"], "replay_checkpoint")
            self.assertNotIn("monotonic_started_at", state)


if __name__ == "__main__":
    unittest.main()
