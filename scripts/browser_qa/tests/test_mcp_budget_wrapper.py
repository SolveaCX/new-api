import io
import json
import os
import subprocess
import tempfile
import unittest

from scripts.browser_qa.flatkey_browser_qa.budget import ExplorationBudget
from scripts.browser_qa.flatkey_browser_qa import mcp_budget_wrapper


class FakeClock:
    def __init__(self):
        self.now = 100.0

    def monotonic(self):
        return self.now

    def advance(self, seconds):
        self.now += seconds


class FakeChild:
    def __init__(self, *, wait_timeout_once=False):
        self.stdin = io.StringIO()
        self.stdout = io.StringIO()
        self.stderr = io.StringIO()
        self.terminated = False
        self.killed = False
        self.waited = False
        self.wait_timeout_once = wait_timeout_once
        self.wait_calls = 0

    def terminate(self):
        self.terminated = True

    def wait(self, timeout=None):
        self.wait_calls += 1
        self.waited = True
        if self.wait_timeout_once and self.wait_calls == 1:
            raise subprocess.TimeoutExpired("fake-child", timeout)
        return 0

    def kill(self):
        self.killed = True


def frame(payload):
    return json.dumps(payload) + "\n"


def call_frame(identifier):
    return frame({"jsonrpc": "2.0", "id": identifier, "method": "tools/call", "params": {"name": "browser_click", "arguments": {}}})


class WrapperContractTests(unittest.TestCase):
    def test_child_command_is_fixed_no_shell_and_file_output_limited_to_runtime_dir(self):
        with tempfile.TemporaryDirectory() as runtime_dir:
            command = mcp_budget_wrapper.playwright_child_command(runtime_dir)

            self.assertEqual(command[:3], ["npx", "-y", "@playwright/mcp@0.0.78"])
            self.assertIn("--output-mode", command)
            self.assertIn("file", command)
            self.assertIn("--output-dir", command)
            output_dir = command[command.index("--output-dir") + 1]
            self.assertTrue(os.path.realpath(output_dir).startswith(os.path.realpath(runtime_dir) + os.sep))
            self.assertNotIn("@latest", command)
            self.assertIs(mcp_budget_wrapper.SUBPROCESS_SHELL, False)

    def test_wrapper_forwards_calls_before_marker_and_exactly_thirty_after_marker(self):
        clock = FakeClock()
        child = FakeChild()
        with tempfile.TemporaryDirectory() as runtime_dir:
            before = call_frame("before")
            after = "".join(call_frame(i) for i in range(1, 32))
            client_input = io.StringIO(before + after)
            client_output = io.StringIO()

            mcp_budget_wrapper.write_control_state(runtime_dir, {"phase": "replay_checkpoint"})
            wrapper = mcp_budget_wrapper.BudgetedMcpWrapper(runtime_dir, child=child, clock=clock)
            wrapper.proxy_client_requests(client_input, client_output, after_first_request=lambda: mcp_budget_wrapper.write_control_state(runtime_dir, {"phase": "exploration"}))

            forwarded = child.stdin.getvalue().splitlines()
            self.assertEqual(len(forwarded), 31)
            self.assertEqual(json.loads(forwarded[0])["id"], "before")
            self.assertEqual(json.loads(forwarded[-1])["id"], 30)
            budget_response = json.loads(client_output.getvalue().splitlines()[0])
            self.assertEqual(budget_response["id"], 31)
            self.assertEqual(budget_response["error"]["code"], -32001)
            self.assertIsInstance(wrapper.exploration_budget, ExplorationBudget)
            self.assertEqual(wrapper.exploration_budget.actions_consumed, 30)

    def test_initialize_list_responses_and_notifications_do_not_consume_action_budget(self):
        clock = FakeClock()
        child = FakeChild()
        with tempfile.TemporaryDirectory() as runtime_dir:
            mcp_budget_wrapper.write_control_state(runtime_dir, {"phase": "exploration"})
            payloads = [
                {"jsonrpc": "2.0", "id": "init", "method": "initialize"},
                {"jsonrpc": "2.0", "method": "notifications/initialized"},
                {"jsonrpc": "2.0", "id": "list", "method": "tools/list"},
            ]
            client_input = io.StringIO("".join(frame(payload) for payload in payloads) + "".join(call_frame(i) for i in range(1, 31)))

            wrapper = mcp_budget_wrapper.BudgetedMcpWrapper(runtime_dir, child=child, clock=clock)
            wrapper.proxy_client_requests(client_input, io.StringIO())

            forwarded = child.stdin.getvalue().splitlines()
            self.assertEqual(len(forwarded), 33)
            self.assertEqual(json.loads(forwarded[-1])["id"], 30)

    def test_time_cutoff_state_parse_failure_client_parse_failure_and_child_protocol_parse_failure_fail_closed(self):
        clock = FakeClock()
        child = FakeChild()
        with tempfile.TemporaryDirectory() as runtime_dir:
            mcp_budget_wrapper.write_control_state(runtime_dir, {"phase": "exploration"})
            wrapper = mcp_budget_wrapper.BudgetedMcpWrapper(runtime_dir, child=child, clock=clock)
            wrapper.proxy_client_requests(io.StringIO(call_frame("start")), io.StringIO())
            clock.advance(301)
            child = FakeChild()
            wrapper.child = child
            wrapper._closed = False
            output = io.StringIO()
            wrapper.proxy_client_requests(io.StringIO(call_frame("late")), output)
            self.assertEqual(json.loads(output.getvalue().splitlines()[0])["error"]["code"], -32001)
            self.assertEqual(child.stdin.getvalue(), "")

        with tempfile.TemporaryDirectory() as runtime_dir:
            with open(os.path.join(runtime_dir, "control_state.json"), "w", encoding="utf-8") as state_file:
                state_file.write("{partial")
            child = FakeChild()
            wrapper = mcp_budget_wrapper.BudgetedMcpWrapper(runtime_dir, child=child, clock=FakeClock())
            output = io.StringIO()
            wrapper.proxy_client_requests(io.StringIO(call_frame("bad-state")), output)
            self.assertEqual(json.loads(output.getvalue().splitlines()[0])["error"]["code"], -32001)
            self.assertEqual(child.stdin.getvalue(), "")

        with tempfile.TemporaryDirectory() as runtime_dir:
            child = FakeChild()
            wrapper = mcp_budget_wrapper.BudgetedMcpWrapper(runtime_dir, child=child, clock=FakeClock())
            output = io.StringIO()
            wrapper.proxy_client_requests(io.StringIO("{bad-json}\n"), output)
            self.assertEqual(json.loads(output.getvalue().splitlines()[0])["error"]["code"], -32700)
            self.assertTrue(child.terminated)

        for payload in ["\"string\"", "[]", "{\"jsonrpc\":\"1.0\",\"id\":1,\"method\":\"tools/list\"}", "{\"id\":1,\"method\":\"tools/list\"}"]:
            with self.subTest(payload=payload):
                with tempfile.TemporaryDirectory() as runtime_dir:
                    child = FakeChild()
                    wrapper = mcp_budget_wrapper.BudgetedMcpWrapper(runtime_dir, child=child, clock=FakeClock())
                    output = io.StringIO()
                    wrapper.proxy_client_requests(io.StringIO(payload + "\n"), output)
                    self.assertEqual(json.loads(output.getvalue().splitlines()[0])["error"]["code"], -32600)
                    self.assertEqual(child.stdin.getvalue(), "")
                    self.assertTrue(child.terminated)

        with tempfile.TemporaryDirectory() as runtime_dir:
            bad_child_frames = ["non-json", "\"string\"", "[]", "{}", "{\"jsonrpc\":\"1.0\",\"id\":1}"]
            for line in bad_child_frames:
                with self.subTest(child_stdout=line):
                    child = FakeChild()
                    child.stdout = io.StringIO(line + "\n")
                    wrapper = mcp_budget_wrapper.BudgetedMcpWrapper(runtime_dir, child=child, clock=FakeClock())
                    output = io.StringIO()
                    wrapper.proxy_child_stdout(output)
                    self.assertEqual(output.getvalue(), "")
                    self.assertTrue(child.terminated)

    def test_eof_terminates_child_without_orphan(self):
        with tempfile.TemporaryDirectory() as runtime_dir:
            child = FakeChild()
            wrapper = mcp_budget_wrapper.BudgetedMcpWrapper(runtime_dir, child=child, clock=FakeClock())
            wrapper.proxy_client_requests(io.StringIO(""), io.StringIO())
            self.assertTrue(child.terminated)
            self.assertTrue(child.waited)

    def test_parent_signal_terminates_child_without_orphan(self):
        with tempfile.TemporaryDirectory() as runtime_dir:
            child = FakeChild()
            wrapper = mcp_budget_wrapper.BudgetedMcpWrapper(runtime_dir, child=child, clock=FakeClock())

            with self.assertRaises(SystemExit):
                wrapper.handle_parent_signal("TERM", None)

            self.assertTrue(child.terminated)
            self.assertTrue(child.waited)

    def test_terminate_kills_child_when_terminate_is_ignored(self):
        with tempfile.TemporaryDirectory() as runtime_dir:
            child = FakeChild(wait_timeout_once=True)
            wrapper = mcp_budget_wrapper.BudgetedMcpWrapper(runtime_dir, child=child, clock=FakeClock())

            wrapper.terminate_child()

            self.assertTrue(child.terminated)
            self.assertTrue(child.killed)
            self.assertEqual(child.wait_calls, 2)


if __name__ == "__main__":
    unittest.main()
