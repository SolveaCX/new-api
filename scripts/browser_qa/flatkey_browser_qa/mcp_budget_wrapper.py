import json
import os
import signal
import subprocess
import sys
import threading
import time

from .control_mcp import STATE_FILE_NAME
from .control_mcp import write_control_state


SUBPROCESS_SHELL = False
_MAX_ACTIONS = 30
_MAX_SECONDS = 300


def playwright_child_command(runtime_dir):
    directory = _runtime_dir(runtime_dir)
    output_dir = os.path.join(directory, "playwright-output")
    os.makedirs(output_dir, exist_ok=True)
    return ["npx", "-y", "@playwright/mcp@0.0.78", "--output-mode", "file", "--output-dir", output_dir]


def main():
    runtime_dir = os.environ["FLATKEY_BROWSER_QA_RUNTIME_DIR"]
    child = subprocess.Popen(
        playwright_child_command(runtime_dir),
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        encoding="utf-8",
        errors="strict",
        bufsize=1,
        shell=SUBPROCESS_SHELL,
    )
    wrapper = BudgetedMcpWrapper(runtime_dir, child=child)
    signal.signal(signal.SIGTERM, wrapper.handle_parent_signal)
    signal.signal(signal.SIGINT, wrapper.handle_parent_signal)
    stdout_thread = threading.Thread(target=wrapper.proxy_child_stdout, args=(sys.stdout,), daemon=True)
    stderr_thread = threading.Thread(target=wrapper.proxy_child_stderr, args=(sys.stderr,), daemon=True)
    stdout_thread.start()
    stderr_thread.start()
    wrapper.proxy_client_requests(sys.stdin, sys.stdout)


class BudgetedMcpWrapper:
    def __init__(self, runtime_dir, *, child, clock=None):
        self.runtime_dir = _runtime_dir(runtime_dir)
        self.child = child
        self.clock = clock or time
        self._exploration_started_at = None
        self._actions = 0
        self._closed = False

    def proxy_client_requests(self, client_input, client_output, *, after_first_request=None):
        try:
            for index, line in enumerate(client_input):
                request = self._parse_client_line(line, client_output)
                if request is None:
                    break
                if self._is_counted_tool_call(request):
                    if not self._allow_action(request, client_output):
                        break
                self._write_child_stdin(line)
                if index == 0 and after_first_request:
                    after_first_request()
        finally:
            self.terminate_child()

    def proxy_child_stdout(self, client_output):
        for line in self.child.stdout:
            try:
                json.loads(line)
            except json.JSONDecodeError:
                self.terminate_child()
                break
            client_output.write(line)
            client_output.flush()

    def proxy_child_stderr(self, diagnostic_output):
        for line in self.child.stderr:
            diagnostic_output.write(line)
            diagnostic_output.flush()

    def terminate_child(self):
        if self._closed:
            return
        self._closed = True
        try:
            self.child.terminate()
        except Exception:
            pass
        try:
            self.child.wait(timeout=5)
        except Exception:
            pass

    def handle_parent_signal(self, _signum, _frame):
        self.terminate_child()
        raise SystemExit(128)

    def _parse_client_line(self, line, client_output):
        try:
            return json.loads(line)
        except json.JSONDecodeError:
            _write_error(client_output, None, -32700, "parse error")
            self.terminate_child()
            return None

    def _is_counted_tool_call(self, request):
        return isinstance(request, dict) and request.get("method") == "tools/call" and "id" in request

    def _allow_action(self, request, client_output):
        try:
            phase = _read_phase(self.runtime_dir)
        except RuntimeError:
            _write_error(client_output, request.get("id"), -32001, "exploration budget unavailable")
            return False
        if phase != "exploration":
            return True
        if self._exploration_started_at is None:
            self._exploration_started_at = self.clock.monotonic()
        if self.clock.monotonic() > self._exploration_started_at + _MAX_SECONDS:
            _write_error(client_output, request.get("id"), -32001, "exploration budget exceeded")
            return False
        if self._actions >= _MAX_ACTIONS:
            _write_error(client_output, request.get("id"), -32001, "exploration budget exceeded")
            return False
        self._actions += 1
        return True

    def _write_child_stdin(self, line):
        self.child.stdin.write(line)
        self.child.stdin.flush()


def _read_phase(runtime_dir):
    path = os.path.join(runtime_dir, STATE_FILE_NAME)
    if not os.path.exists(path):
        return None
    if os.path.islink(path):
        raise RuntimeError("control state unavailable")
    try:
        with open(path, encoding="utf-8") as state_file:
            payload = json.load(state_file)
    except (OSError, json.JSONDecodeError) as exc:
        raise RuntimeError("control state unavailable") from exc
    phase = payload.get("phase") if isinstance(payload, dict) else None
    if phase not in (None, "replay_checkpoint", "exploration"):
        raise RuntimeError("control state unavailable")
    return phase


def _runtime_dir(runtime_dir):
    if not isinstance(runtime_dir, str) or not runtime_dir:
        raise RuntimeError("runtime dir missing")
    real = os.path.realpath(runtime_dir)
    if not os.path.isdir(real):
        raise RuntimeError("runtime dir invalid")
    return real


def _write_error(stdout, request_id, code, message):
    stdout.write(json.dumps({"jsonrpc": "2.0", "id": request_id, "error": {"code": code, "message": message}}, sort_keys=True) + "\n")
    stdout.flush()


if __name__ == "__main__":
    main()
