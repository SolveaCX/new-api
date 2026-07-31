import json
import math
import os
import signal
import subprocess
import sys
import threading
import time
import ctypes
from ctypes import wintypes

from .budget import BudgetExceeded
from .budget import ExplorationBudget
from .budget import ReplayBudget
from .control_mcp import STATE_FILE_NAME
from .control_mcp import write_control_state


SUBPROCESS_SHELL = False
EXPLORATION_MAX_ACTIONS = 30
EXPLORATION_SECONDS = 300
PLAYWRIGHT_MCP_EXECUTABLE = "/usr/local/bin/playwright-mcp"


def playwright_child_command(runtime_dir, proxy_url=None, *, executable=PLAYWRIGHT_MCP_EXECUTABLE):
    if not proxy_url:
        raise RuntimeError("proxy url is required")
    directory = _runtime_dir(runtime_dir)
    output_dir = os.path.join(directory, "playwright-output")
    os.makedirs(output_dir, exist_ok=True)
    config_path = os.path.join(directory, "playwright-mcp-config.json")
    _write_playwright_config(config_path)
    command = [
        executable,
        "--browser",
        "chromium",
        "--headless",
        "--output-mode",
        "file",
        "--output-dir",
        output_dir,
        "--block-service-workers",
        "--config",
        config_path,
        "--proxy-server",
        proxy_url,
        "--proxy-bypass",
        "<-loopback>",
    ]
    return command


def _write_playwright_config(path):
    payload = {
        "browser": {
            "launchOptions": {
                "args": [
                    "--disable-quic",
                    "--force-webrtc-ip-handling-policy=disable_non_proxied_udp",
                ]
            }
        }
    }
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        json.dump(payload, handle, sort_keys=True)


def main():
    runtime_dir = os.environ["FLATKEY_BROWSER_QA_RUNTIME_DIR"]
    proxy_url = os.environ["FLATKEY_BROWSER_QA_PROXY_URL"]
    wrapper = launch_wrapper(runtime_dir, proxy_url=proxy_url)
    signal.signal(signal.SIGTERM, wrapper.handle_parent_signal)
    signal.signal(signal.SIGINT, wrapper.handle_parent_signal)
    stdout_thread = threading.Thread(target=wrapper.proxy_child_stdout, args=(sys.stdout,), daemon=True)
    stderr_thread = threading.Thread(target=wrapper.proxy_child_stderr, args=(sys.stderr,), daemon=True)
    stdout_thread.start()
    stderr_thread.start()
    wrapper.proxy_client_requests(sys.stdin, sys.stdout)


def launch_wrapper(
    runtime_dir,
    *,
    popen_factory=subprocess.Popen,
    tree_attach=None,
    subprocess_run=subprocess.run,
    killpg=os.killpg if hasattr(os, "killpg") else None,
    proxy_url=None,
    parent_env=None,
):
    selected_tree_attach = tree_attach or ProcessTreeTerminator.attach
    child = popen_factory(
        playwright_child_command(runtime_dir, proxy_url=proxy_url or os.environ.get("FLATKEY_BROWSER_QA_PROXY_URL")),
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        encoding="utf-8",
        errors="strict",
        bufsize=1,
        shell=SUBPROCESS_SHELL,
        start_new_session=(os.name != "nt"),
        creationflags=(subprocess.CREATE_NEW_PROCESS_GROUP if os.name == "nt" else 0),
        env=_minimal_child_env(parent_env or os.environ),
    )
    try:
        terminator = selected_tree_attach(child)
    except Exception:
        _best_effort_kill_launch_tree(child, subprocess_run=subprocess_run, killpg=killpg)
        _bounded_wait(child)
        raise
    return BudgetedMcpWrapper(runtime_dir, child=child, tree_terminator=terminator)


class ProcessTreeTerminator:
    @classmethod
    def attach(cls, child):
        if os.name == "nt":
            return WindowsJobProcessContainment.attach(child)
        return PosixProcessGroupContainment.attach(child)


class PosixProcessGroupContainment:
    def __init__(self, pgid):
        self.pgid = pgid

    @classmethod
    def attach(cls, child):
        pid = getattr(child, "pid", None)
        if not isinstance(pid, int) or pid <= 0:
            raise RuntimeError("child pid unavailable for process-group containment")
        return cls(pid)

    def terminate_tree(self, _child):
        os.killpg(self.pgid, signal.SIGTERM)

    def kill_tree(self, _child):
        os.killpg(self.pgid, signal.SIGKILL)

    def close(self):
        return None


class WindowsJobProcessContainment:
    def __init__(self, job_handle, kernel32):
        self.job_handle = job_handle
        self.kernel32 = kernel32

    @classmethod
    def attach(cls, child, *, kernel32=None):
        selected_kernel32 = kernel32 or _kernel32()
        job = selected_kernel32.CreateJobObjectW(None, None)
        if not job:
            raise RuntimeError("windows job creation failed")
        try:
            info = _job_kill_on_close_info()
            if not selected_kernel32.SetInformationJobObject(job, 9, ctypes.byref(info), ctypes.sizeof(info)):
                raise RuntimeError("windows job limit configuration failed")
            process_handle = getattr(child, "_handle", None)
            if process_handle is None:
                raise RuntimeError("child process handle unavailable for job containment")
            if not selected_kernel32.AssignProcessToJobObject(job, process_handle):
                code = _kernel_last_error(selected_kernel32)
                raise RuntimeError(f"windows job assignment failed: {code}")
        except Exception:
            selected_kernel32.CloseHandle(job)
            raise
        return cls(job, selected_kernel32)

    def terminate_tree(self, _child):
        self.close()

    def kill_tree(self, _child):
        self.close()

    def close(self):
        if self.job_handle:
            handle = self.job_handle
            self.job_handle = None
            self.kernel32.CloseHandle(handle)


class BudgetedMcpWrapper:
    def __init__(self, runtime_dir, *, child, clock=None, tree_terminator=None, output_lock=None):
        self.runtime_dir = _runtime_dir(runtime_dir)
        self.child = child
        self.clock = clock or time
        self.tree_terminator = tree_terminator or ProcessTreeTerminator.attach(child)
        self.output_lock = output_lock or threading.Lock()
        self.exploration_budget = None
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
                response = json.loads(line)
            except json.JSONDecodeError:
                self.terminate_child()
                break
            if not _is_jsonrpc_frame(response):
                self.terminate_child()
                break
            self._write_client_line(client_output, line)

    def proxy_child_stderr(self, diagnostic_output):
        for line in self.child.stderr:
            diagnostic_output.write(line)
            diagnostic_output.flush()

    def terminate_child(self):
        if self._closed:
            return
        self._closed = True
        try:
            self.tree_terminator.terminate_tree(self.child)
        except Exception:
            pass
        try:
            self.child.wait(timeout=5)
        except subprocess.TimeoutExpired:
            try:
                self.tree_terminator.kill_tree(self.child)
            except Exception:
                pass
            try:
                self.child.wait(timeout=5)
            except Exception:
                pass
        except Exception:
            pass
        try:
            self.tree_terminator.close()
        except Exception:
            pass

    def handle_parent_signal(self, _signum, _frame):
        self.terminate_child()
        raise SystemExit(128)

    def _parse_client_line(self, line, client_output):
        try:
            request = json.loads(line)
        except json.JSONDecodeError:
            self._write_error(client_output, None, -32700, "parse error")
            self.terminate_child()
            return None
        if not _is_jsonrpc_frame(request):
            request_id = request.get("id") if isinstance(request, dict) else None
            self._write_error(client_output, request_id, -32600, "invalid request")
            self.terminate_child()
            return None
        return request

    def _is_counted_tool_call(self, request):
        return isinstance(request, dict) and request.get("method") == "tools/call"

    def _allow_action(self, request, client_output):
        try:
            phase = _read_phase(self.runtime_dir)
        except RuntimeError:
            self._write_error(client_output, request.get("id"), -32001, "exploration budget unavailable")
            return False
        if phase != "exploration":
            return True
        try:
            if self.exploration_budget is None:
                replay_budget = ReplayBudget(EXPLORATION_SECONDS, self.clock)
                replay_budget.mark_checkpoint()
                self.exploration_budget = ExplorationBudget(EXPLORATION_SECONDS, EXPLORATION_MAX_ACTIONS, self.clock)
                self.exploration_budget.start(replay_budget, started_at=_marker_started_at(self.runtime_dir, self.clock))
        except (RuntimeError, ValueError, BudgetExceeded):
            self._write_error(client_output, request.get("id"), -32001, "exploration budget unavailable")
            return False
        try:
            self.exploration_budget.consume_action()
        except BudgetExceeded:
            if "id" in request:
                self._write_error(client_output, request.get("id"), -32001, "exploration budget exceeded")
            return False
        return True

    def _write_child_stdin(self, line):
        self.child.stdin.write(line)
        self.child.stdin.flush()

    def _write_error(self, stdout, request_id, code, message):
        _write_error(stdout, request_id, code, message, lock=self.output_lock)

    def _write_client_line(self, stdout, line):
        with self.output_lock:
            stdout.write(line)
            stdout.flush()


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


def _marker_started_at(runtime_dir, clock):
    path = os.path.join(runtime_dir, STATE_FILE_NAME)
    try:
        with open(path, encoding="utf-8") as state_file:
            payload = json.load(state_file)
    except (OSError, json.JSONDecodeError) as exc:
        raise RuntimeError("control state unavailable") from exc
    marker = payload.get("monotonic_started_at") if isinstance(payload, dict) else None
    now = clock.monotonic()
    if not isinstance(marker, (int, float)) or isinstance(marker, bool) or not math.isfinite(marker):
        raise RuntimeError("control state unavailable")
    if marker < 0 or marker > now + 1:
        raise RuntimeError("control state unavailable")
    return float(marker)


def _runtime_dir(runtime_dir):
    if not isinstance(runtime_dir, str) or not runtime_dir:
        raise RuntimeError("runtime dir missing")
    real = os.path.realpath(runtime_dir)
    if not os.path.isdir(real):
        raise RuntimeError("runtime dir invalid")
    return real


def _is_jsonrpc_frame(payload):
    return isinstance(payload, dict) and payload.get("jsonrpc") == "2.0"


def _write_error(stdout, request_id, code, message, *, lock=None):
    def write():
        stdout.write(json.dumps({"jsonrpc": "2.0", "id": request_id, "error": {"code": code, "message": message}}, sort_keys=True) + "\n")
        stdout.flush()

    if lock is None:
        write()
    else:
        with lock:
            write()


def _best_effort_kill_launch_tree(child, *, subprocess_run=subprocess.run, killpg=None):
    pid = getattr(child, "pid", None)
    if isinstance(pid, int) and pid > 0:
        if os.name == "nt":
            subprocess_run(
                ["taskkill", "/PID", str(pid), "/T", "/F"],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                shell=False,
                check=False,
            )
            return
        if killpg is not None:
            killpg(pid, signal.SIGKILL)
            return
    child.kill()


def _bounded_wait(child):
    try:
        child.wait(timeout=5)
    except Exception:
        pass


def _minimal_child_env(parent_env):
    child_env = {}
    if parent_env.get("PATH"):
        child_env["PATH"] = parent_env["PATH"]
    if os.name == "nt" and parent_env.get("SystemRoot"):
        child_env["SystemRoot"] = parent_env["SystemRoot"]
    return child_env


def _kernel32():
    kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)
    _declare_kernel32_job_prototypes(kernel32)
    return kernel32


def _declare_kernel32_job_prototypes(kernel32):
    _set_prototype(kernel32.CreateJobObjectW, [wintypes.LPVOID, wintypes.LPCWSTR], wintypes.HANDLE)
    _set_prototype(kernel32.SetInformationJobObject, [wintypes.HANDLE, ctypes.c_int, wintypes.LPVOID, wintypes.DWORD], wintypes.BOOL)
    _set_prototype(kernel32.AssignProcessToJobObject, [wintypes.HANDLE, wintypes.HANDLE], wintypes.BOOL)
    _set_prototype(kernel32.CloseHandle, [wintypes.HANDLE], wintypes.BOOL)
    _set_prototype(kernel32.GetLastError, [], wintypes.DWORD)


def _set_prototype(function, argtypes, restype):
    try:
        function.argtypes = argtypes
        function.restype = restype
    except AttributeError:
        pass


def _kernel_last_error(kernel32):
    try:
        return kernel32.GetLastError()
    except AttributeError:
        return ctypes.get_last_error()


def _job_kill_on_close_info():
    info = JOBOBJECT_EXTENDED_LIMIT_INFORMATION()
    info.BasicLimitInformation.LimitFlags = 0x00002000
    return info


class IO_COUNTERS(ctypes.Structure):
    _fields_ = [
        ("ReadOperationCount", ctypes.c_uint64),
        ("WriteOperationCount", ctypes.c_uint64),
        ("OtherOperationCount", ctypes.c_uint64),
        ("ReadTransferCount", ctypes.c_uint64),
        ("WriteTransferCount", ctypes.c_uint64),
        ("OtherTransferCount", ctypes.c_uint64),
    ]


class JOBOBJECT_BASIC_LIMIT_INFORMATION(ctypes.Structure):
    _fields_ = [
        ("PerProcessUserTimeLimit", ctypes.c_int64),
        ("PerJobUserTimeLimit", ctypes.c_int64),
        ("LimitFlags", ctypes.c_uint32),
        ("MinimumWorkingSetSize", ctypes.c_size_t),
        ("MaximumWorkingSetSize", ctypes.c_size_t),
        ("ActiveProcessLimit", ctypes.c_uint32),
        ("Affinity", ctypes.c_size_t),
        ("PriorityClass", ctypes.c_uint32),
        ("SchedulingClass", ctypes.c_uint32),
    ]


class JOBOBJECT_EXTENDED_LIMIT_INFORMATION(ctypes.Structure):
    _fields_ = [
        ("BasicLimitInformation", JOBOBJECT_BASIC_LIMIT_INFORMATION),
        ("IoInfo", IO_COUNTERS),
        ("ProcessMemoryLimit", ctypes.c_size_t),
        ("JobMemoryLimit", ctypes.c_size_t),
        ("PeakProcessMemoryUsed", ctypes.c_size_t),
        ("PeakJobMemoryUsed", ctypes.c_size_t),
    ]


if __name__ == "__main__":
    main()
