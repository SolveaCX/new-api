import json
import os
import sys
import tempfile
import time

from .mcp import McpServer
from .mcp import Tool
from .mcp import ToolExecutionError
from .mcp import run_jsonrpc_server


STATE_FILE_NAME = "control_state.json"
_ENV_RUNTIME_DIR = "FLATKEY_BROWSER_QA_RUNTIME_DIR"
_ENV_MODE = "FLATKEY_BROWSER_QA_MODE"


def run(stdin=None, stdout=None, *, env=None, clock=None, max_line_bytes=1024 * 1024):
    stdin = stdin or sys.stdin
    stdout = stdout or sys.stdout
    env = env or os.environ
    clock = clock or time
    runtime_dir = env.get(_ENV_RUNTIME_DIR)
    mode = env.get(_ENV_MODE, "normal")
    server = McpServer(
        "flatkey-browser-qa-control",
        [
            Tool("qa_replay_checkpoint", "Mark the replay checkpoint as complete.", lambda: _mark(runtime_dir, "replay_checkpoint", clock)),
            Tool("qa_start_exploration", "Start the limited exploration phase.", lambda: _start_exploration(runtime_dir, clock, mode)),
        ],
    )
    run_jsonrpc_server(stdin, stdout, server, max_line_bytes=max_line_bytes)


def _start_exploration(runtime_dir, clock, mode):
    if mode == "core":
        raise ToolExecutionError("core mode forbids exploration")
    if mode != "normal":
        raise ToolExecutionError("control mode unavailable")
    return _mark(runtime_dir, "exploration", clock)


def _mark(runtime_dir, phase, clock):
    state = {"phase": phase, "updated_at": int(clock.time())}
    if phase == "exploration":
        state["monotonic_started_at"] = float(clock.monotonic())
    write_control_state(runtime_dir, state)
    return {"status": "ok", "phase": phase}


def write_control_state(runtime_dir, state):
    directory = _runtime_dir(runtime_dir)
    state_path = os.path.join(directory, STATE_FILE_NAME)
    if os.path.lexists(state_path) and (os.path.islink(state_path) or not os.path.isfile(state_path)):
        raise ToolExecutionError("control state unavailable")
    raw = json.dumps(state, separators=(",", ":"), sort_keys=True).encode("utf-8")
    fd = None
    tmp_path = None
    try:
        fd, tmp_path = tempfile.mkstemp(prefix=".control_state.", suffix=".tmp", dir=directory)
        with os.fdopen(fd, "wb") as handle:
            fd = None
            handle.write(raw)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(tmp_path, state_path)
        tmp_path = None
        _fsync_dir(directory)
    except OSError as exc:
        raise ToolExecutionError("control state unavailable") from exc
    finally:
        if fd is not None:
            os.close(fd)
        if tmp_path and os.path.exists(tmp_path):
            try:
                os.unlink(tmp_path)
            except OSError:
                pass


def _runtime_dir(runtime_dir):
    if not isinstance(runtime_dir, str) or not runtime_dir:
        raise ToolExecutionError("control runtime missing")
    real = os.path.realpath(runtime_dir)
    if not os.path.isdir(real):
        raise ToolExecutionError("control runtime invalid")
    return real


def _fsync_dir(directory):
    if os.name == "nt":
        return
    fd = os.open(directory, os.O_RDONLY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


if __name__ == "__main__":
    run()
