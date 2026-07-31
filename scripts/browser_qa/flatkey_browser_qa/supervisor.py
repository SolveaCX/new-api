import json
import os
import signal
import shutil
import subprocess
import sys
import tempfile
import urllib.request

from . import report
from .cleanup import CleanupResult
from .config import load_config
from .egress_proxy import EgressProxy
from .identity import derive_identity
from .redaction import Redactor


INTERNAL_DEADLINE_SECONDS = 840
PROMPT_PATH = os.path.join(os.path.dirname(os.path.dirname(__file__)), "config", "qa-prompt.md")
POLICY_PATH = ".agents/skills/flatkey-new-user-onboarding/references/staging-cloud-qa-policy.md"


class _SignalAbort(RuntimeError):
    pass


class Outcome:
    def __init__(self, status, manifest_path, events):
        self.status = status
        self.manifest_path = manifest_path
        self.events = events

    def to_dict(self):
        return {"status": self.status, "manifest_path": self.manifest_path, "events": self.events}


class Supervisor:
    def __init__(
        self,
        *,
        env,
        runtime_root,
        subprocess_runner,
        uploader,
        cleanup_runner,
        proxy_factory=EgressProxy,
        preflight=None,
        clock,
        signal_module=signal,
    ):
        self.env = dict(env)
        self.runtime_root = runtime_root
        self.subprocess_runner = subprocess_runner
        self.uploader = uploader
        self.cleanup_runner = cleanup_runner
        self.proxy_factory = proxy_factory
        self.preflight = preflight
        self.clock = clock
        self.signal_module = signal_module
        self.events = []
        self.codex_home = None
        self._active_process = None

    def run(self, *, initial_result=None):
        os.makedirs(self.runtime_root, exist_ok=True)
        cfg = load_config({key: value for key, value in self.env.items() if key.startswith("FLATKEY_QA_")})
        identity = derive_identity(cfg.identity_seed, cfg.run_id)
        redactor = Redactor(
            email=f"{cfg.gmail_base.split('@', 1)[0]}+{identity.email_tag}@{cfg.gmail_base.split('@', 1)[1]}",
            password=identity.password,
            extra_secrets=(
                self.env.get("CODEX_API_KEY", ""),
                cfg.gmail_base,
                identity.email_tag,
                "password",
                "Cookie: secret",
                "Authorization: Bearer",
                "sk-xSECRET",
            ),
        )
        self._identity = identity
        result_path = os.path.join(self.runtime_root, "result.json")
        quarantine_path = os.path.join(self.runtime_root, "quarantine-result.json")
        manifest_path = os.path.join(self.runtime_root, "manifest.json")
        artifact_paths = [result_path, manifest_path]
        proxy = self.proxy_factory()
        cleanup_result = CleanupResult(0, False, False, True, "cleanup was not attempted")
        upload_failed = False
        invalid_result = False
        codex_returncode = 0

        payload = initial_result if initial_result is not None else _empty_infrastructure_result("invalid_result")
        try:
            selected_preflight = self.preflight or StagingStatusPreflight(cfg.console_origin)
            preflight_payload = selected_preflight()
            if not _preflight_ok(preflight_payload):
                payload = _empty_infrastructure_result("preflight_failed")
                invalid_result = False
            else:
                proxy.start()
                process = self._start_codex(proxy, quarantine_path)
                prompt = build_prompt(cfg, identity)
                previous_handlers = self._install_signal_handlers(process)
                try:
                    codex_returncode = self._wait_for_codex(process, redactor, prompt)
                finally:
                    self._restore_signal_handlers(previous_handlers)
                if initial_result is None and os.path.exists(quarantine_path):
                    with open(quarantine_path, encoding="utf-8") as handle:
                        payload = json.load(handle)
                    payload = redactor.clean(payload)
                    try:
                        os.remove(quarantine_path)
                    except FileNotFoundError:
                        pass
                if codex_returncode != 0:
                    payload = _empty_infrastructure_result("codex_nonzero")
        except TimeoutError:
            payload = _empty_infrastructure_result("codex_timeout")
            codex_returncode = -15
        except _SignalAbort:
            payload = _empty_infrastructure_result("codex_signal")
            codex_returncode = -15
        except Exception as exc:
            payload = _empty_infrastructure_result("invalid_result")
            self._event("infrastructure_error", str(exc), redactor)
            invalid_result = True
        finally:
            try:
                proxy.stop()
            except Exception as exc:
                self._event("proxy_stop_failed", str(exc), redactor)
            try:
                cleanup_result = self.cleanup_runner.run(identity)
            except Exception as exc:
                cleanup_result = CleanupResult(0, False, False, True, redactor.clean(str(exc)))
            try:
                report.validate_result(payload)
            except Exception:
                payload = _empty_infrastructure_result("invalid_result")
                invalid_result = True
            _write_private_json(result_path, redactor.clean(payload))
            manifest = report.write_report(
                result_path,
                manifest_path,
                cleanup_result=cleanup_result,
                redactor=redactor,
                codex_returncode=codex_returncode,
                upload_failed=upload_failed,
                invalid_result=invalid_result,
            )
            try:
                self.uploader.upload(manifest_path, artifact_paths)
            except Exception as exc:
                upload_failed = True
                self._event("upload_failed", str(exc), redactor)
                manifest = report.write_report(
                    result_path,
                    manifest_path,
                    cleanup_result=cleanup_result,
                    redactor=redactor,
                    codex_returncode=codex_returncode,
                    upload_failed=True,
                    invalid_result=invalid_result,
                )
        return Outcome(manifest["status"], manifest_path, self.events)

    def _start_codex(self, proxy, result_path):
        self.codex_home = os.path.join(self.runtime_root, "codex-home")
        os.makedirs(self.codex_home, mode=0o700, exist_ok=True)
        os.chmod(self.codex_home, 0o700)
        config_path = os.path.join(self.codex_home, "config.toml")
        qa_config_path = os.path.join(self.codex_home, "qa.config.toml")
        _write_private_text(config_path, _codex_config(proxy))
        runtime_dir = os.path.realpath(self.runtime_root)
        empty_workspace = tempfile.mkdtemp(prefix="empty-workspace-", dir=self.runtime_root)
        api_key = self.env.pop("CODEX_API_KEY")
        repo_root = _repo_root()
        child_env = {
            "CODEX_HOME": self.codex_home,
            "CODEX_API_KEY": api_key,
            "PYTHONPATH": repo_root,
            "FLATKEY_BROWSER_QA_RUNTIME_DIR": runtime_dir,
            "FLATKEY_BROWSER_QA_RUN_ID": self.env["FLATKEY_QA_RUN_ID"],
            "FLATKEY_BROWSER_QA_EMAIL_TAG": self._identity.email_tag,
            "FLATKEY_BROWSER_QA_START_TIME": str(int(self.clock.time())),
            "FLATKEY_BROWSER_QA_BROKER_URL": self.env["FLATKEY_BROWSER_QA_BROKER_URL"],
            "FLATKEY_BROWSER_QA_PROXY_URL": f"http://{proxy.host}:{proxy.port}",
        }
        if os.environ.get("SystemRoot"):
            child_env["SystemRoot"] = os.environ["SystemRoot"]
        if os.environ.get("PATH"):
            child_env["PATH"] = os.environ["PATH"]
        _write_private_text(qa_config_path, _qa_config(proxy, runtime_dir, child_env))
        args = [
            shutil.which("codex") or "codex",
            "exec",
            "--strict-config",
            "--ignore-rules",
            "--profile",
            "qa",
            "--ephemeral",
            "--json",
            "--output-schema",
            os.path.join(os.path.dirname(os.path.dirname(__file__)), "config", "result.schema.json"),
            "--output-last-message",
            result_path,
            "--sandbox",
            "workspace-write",
            "--model",
            "gpt-5.4",
            "--cd",
            empty_workspace,
            "-",
        ]
        process = self.subprocess_runner.popen(args, env=child_env, cwd=repo_root, stdin=subprocess.PIPE, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self._active_process = process
        return process

    def _wait_for_codex(self, process, redactor, prompt):
        try:
            stdout, stderr = process.communicate(input=prompt, timeout=INTERNAL_DEADLINE_SECONDS)
        except subprocess.TimeoutExpired as exc:
            _terminate_process(process)
            raise TimeoutError("codex internal deadline exceeded") from exc
        self._event("codex_stdout", stdout, redactor)
        self._event("codex_stderr", stderr, redactor)
        return process.returncode

    def _install_signal_handlers(self, process):
        self._active_process = process
        return {
            self.signal_module.SIGTERM: self.signal_module.signal(self.signal_module.SIGTERM, self._handle_signal),
            self.signal_module.SIGINT: self.signal_module.signal(self.signal_module.SIGINT, self._handle_signal),
        }

    def _restore_signal_handlers(self, previous):
        for signum, handler in previous.items():
            self.signal_module.signal(signum, handler)

    def _handle_signal(self, _signum, _frame):
        if self._active_process is not None:
            _terminate_process(self._active_process)
        raise _SignalAbort("codex interrupted by signal")

    def _event(self, kind, text, redactor):
        cleaner = redactor or Redactor(extra_secrets=(self.env.get("CODEX_API_KEY", ""),))
        self.events.append({"kind": kind, "text": cleaner.clean(str(text))})


def _empty_infrastructure_result(classification):
    return {
        "replay": {"status": "failed", "checkpoint_reached": False},
        "exploration": {"status": "not_started", "actions_used": 0},
        "budgets": {"replay_seconds": 300, "exploration_seconds": 300, "max_actions": 30},
        "findings": [],
        "infrastructure": {"status": "failed", "classification": classification},
    }


def _codex_config(_proxy):
    return f"""[sandbox_workspace_write]
network_access = false

[shell_environment_policy]
inherit = "none"
"""


def _qa_config(proxy, runtime_dir, child_env):
    escaped_runtime_dir = _toml_escape(runtime_dir)
    escaped_repo_root = _toml_escape(_repo_root())
    return f"""model = "gpt-5.4"
approval_policy = "never"
web_search = "disabled"

[mcp_servers.playwright]
command = "{_toml_escape(sys.executable)}"
args = ["-m", "scripts.browser_qa.flatkey_browser_qa.mcp_budget_wrapper"]
required = true
enabled_tools = ["browser_navigate", "browser_navigate_back", "browser_tabs", "browser_click", "browser_type", "browser_fill_form", "browser_select_option", "browser_snapshot", "browser_find", "browser_wait_for", "browser_take_screenshot", "browser_console_messages", "browser_network_requests", "browser_network_request"]
[mcp_servers.playwright.env]
PYTHONPATH = "{escaped_repo_root}"
PATH = "{_toml_escape(os.environ.get("PATH", ""))}"
FLATKEY_BROWSER_QA_RUNTIME_DIR = "{escaped_runtime_dir}"
FLATKEY_BROWSER_QA_PROXY_URL = "http://{proxy.host}:{proxy.port}"

[mcp_servers.broker]
command = "{_toml_escape(sys.executable)}"
args = ["-m", "scripts.browser_qa.flatkey_browser_qa.broker_mcp"]
required = true
enabled_tools = ["get_current_verification_code"]
[mcp_servers.broker.env]
PYTHONPATH = "{escaped_repo_root}"
PATH = "{_toml_escape(os.environ.get("PATH", ""))}"
FLATKEY_BROWSER_QA_RUNTIME_DIR = "{escaped_runtime_dir}"
FLATKEY_BROWSER_QA_RUN_ID = "{_toml_escape(child_env["FLATKEY_BROWSER_QA_RUN_ID"])}"
FLATKEY_BROWSER_QA_EMAIL_TAG = "{_toml_escape(child_env["FLATKEY_BROWSER_QA_EMAIL_TAG"])}"
FLATKEY_BROWSER_QA_START_TIME = "{_toml_escape(child_env["FLATKEY_BROWSER_QA_START_TIME"])}"
FLATKEY_BROWSER_QA_BROKER_URL = "{_toml_escape(child_env["FLATKEY_BROWSER_QA_BROKER_URL"])}"

[mcp_servers.control]
command = "{_toml_escape(sys.executable)}"
args = ["-m", "scripts.browser_qa.flatkey_browser_qa.control_mcp"]
required = true
enabled_tools = ["qa_replay_checkpoint", "qa_start_exploration"]
[mcp_servers.control.env]
PYTHONPATH = "{escaped_repo_root}"
PATH = "{_toml_escape(os.environ.get("PATH", ""))}"
FLATKEY_BROWSER_QA_RUNTIME_DIR = "{escaped_runtime_dir}"
"""


def _write_private_json(path, payload):
    _write_private_text(path, json.dumps(payload, sort_keys=True))


def _write_private_text(path, text):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        handle.write(text)


def build_prompt(cfg, identity):
    with open(PROMPT_PATH, encoding="utf-8") as handle:
        prompt = handle.read()
    return (
        prompt
        + "\n\nSkill: $flatkey-new-user-onboarding\n"
        + f"Policy: {POLICY_PATH}\n"
        + f"Run ID: {cfg.run_id}\n"
        + f"Disposable username: {identity.username}\n"
        + f"Disposable email: {cfg.gmail_base.split('@', 1)[0]}+{identity.email_tag}@{cfg.gmail_base.split('@', 1)[1]}\n"
        + f"Disposable password: {identity.password}\n"
        + "The runtime owns verification code, cleanup, result redaction, and upload.\n"
    )


def _preflight_ok(payload):
    data = payload.get("data") if isinstance(payload, dict) else None
    if not isinstance(data, dict):
        return False
    return (
        data.get("register_enabled") is True
        and data.get("password_register_enabled") is True
        and data.get("email_verification") is True
        and data.get("turnstile_check") is False
    )


def _terminate_process(process):
    try:
        process.terminate()
        process.communicate(timeout=2)
    except Exception:
        try:
            process.kill()
        except Exception:
            pass


def _repo_root():
    return os.path.realpath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))


def _toml_escape(value):
    return str(value).replace("\\", "\\\\").replace('"', '\\"')


class StagingStatusPreflight:
    def __init__(self, origin, *, opener=None):
        if origin != "https://staging-console.flatkey.ai":
            raise ValueError("preflight origin must be staging console")
        self.origin = origin
        self.opener = opener or urllib.request.build_opener(_NoRedirectHandler(), urllib.request.ProxyHandler({}))

    def __call__(self):
        request = urllib.request.Request(self.origin + "/api/status", headers={"Accept": "application/json"}, method="GET")
        with self.opener.open(request, timeout=5) as response:
            raw = response.read(65537)
            if len(raw) > 65536:
                raise RuntimeError("status response too large")
            payload = json.loads(raw.decode("utf-8"))
        if not _preflight_ok(payload):
            raise RuntimeError("staging preflight failed")
        return payload


class _NoRedirectHandler(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None
