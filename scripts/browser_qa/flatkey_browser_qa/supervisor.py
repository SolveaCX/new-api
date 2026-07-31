import json
import os
import tempfile

from . import report
from .cleanup import CleanupResult
from .config import load_config
from .egress_proxy import EgressProxy
from .identity import derive_identity
from .redaction import Redactor


INTERNAL_DEADLINE_SECONDS = 840


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
        preflight,
        clock,
    ):
        self.env = dict(env)
        self.runtime_root = runtime_root
        self.subprocess_runner = subprocess_runner
        self.uploader = uploader
        self.cleanup_runner = cleanup_runner
        self.proxy_factory = proxy_factory
        self.preflight = preflight
        self.clock = clock
        self.events = []
        self.codex_home = None

    def run(self, *, initial_result=None):
        os.makedirs(self.runtime_root, exist_ok=True)
        cfg = load_config({key: value for key, value in self.env.items() if key.startswith("FLATKEY_QA_")})
        identity = derive_identity(cfg.identity_seed, cfg.run_id)
        redactor = Redactor(
            email=f"{cfg.gmail_base.split('@', 1)[0]}+{identity.email_tag}@{cfg.gmail_base.split('@', 1)[1]}",
            password=identity.password,
            extra_secrets=(self.env.get("CODEX_API_KEY", ""), cfg.gmail_base, "password", "Cookie: secret", "Authorization: Bearer", "sk-xSECRET"),
        )
        result_path = os.path.join(self.runtime_root, "result.json")
        manifest_path = os.path.join(self.runtime_root, "manifest.json")
        artifact_paths = [result_path, manifest_path]
        proxy = self.proxy_factory()
        cleanup_result = CleanupResult(0, False, False, True, "cleanup was not attempted")
        upload_failed = False
        invalid_result = False
        codex_returncode = 0

        payload = initial_result if initial_result is not None else _empty_infrastructure_result("invalid_result")
        try:
            preflight_payload = self.preflight()
            if preflight_payload.get("data", {}).get("email_verification") is not True or preflight_payload.get("data", {}).get("turnstile_check") is not False:
                payload = _empty_infrastructure_result("preflight_failed")
                invalid_result = False
            else:
                proxy.start()
                process = self._start_codex(proxy, result_path)
                codex_returncode = self._wait_for_codex(process, redactor)
                if initial_result is None and os.path.exists(result_path):
                    with open(result_path, encoding="utf-8") as handle:
                        payload = json.load(handle)
        except TimeoutError:
            payload = _empty_infrastructure_result("codex_timeout")
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
        _write_private_text(qa_config_path, _qa_config(proxy))
        empty_workspace = tempfile.mkdtemp(prefix="empty-workspace-", dir=self.runtime_root)
        api_key = self.env.pop("CODEX_API_KEY")
        child_env = {
            "CODEX_HOME": self.codex_home,
            "CODEX_API_KEY": api_key,
            "HTTPS_PROXY": f"http://{proxy.host}:{proxy.port}",
            "HTTP_PROXY": f"http://{proxy.host}:{proxy.port}",
            "NO_PROXY": "",
        }
        args = [
            "codex",
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
        return self.subprocess_runner.popen(args, env=child_env, stdin="qa", text=True, stdout=-1, stderr=-1)

    def _wait_for_codex(self, process, redactor):
        deadline = self.clock.monotonic() + INTERNAL_DEADLINE_SECONDS
        while True:
            remaining = deadline - self.clock.monotonic()
            if remaining <= 0:
                process.terminate()
                try:
                    process.communicate(timeout=2)
                except Exception:
                    pass
                raise TimeoutError("codex internal deadline exceeded")
            try:
                code = process.wait(timeout=min(1, remaining))
                stdout, stderr = process.communicate(timeout=1)
                self._event("codex_stdout", stdout, redactor)
                self._event("codex_stderr", stderr, redactor)
                return code
            except TimeoutError:
                if hasattr(self.clock, "advance"):
                    self.clock.advance(min(1, remaining))

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


def _codex_config(proxy):
    return f"""[sandbox_workspace_write]
network_access = false

[profiles.qa]
config = "qa.config.toml"
proxy = "http://{proxy.host}:{proxy.port}"
proxy_bypass = ""
web_search = false
"""


def _qa_config(proxy):
    return f"""disable_response_storage = true
model = "gpt-5.4"
proxy = "http://{proxy.host}:{proxy.port}"
proxy_bypass = ""
approval_policy = "never"

[mcp_servers.playwright]
command = "python"
args = ["-m", "scripts.browser_qa.flatkey_browser_qa.mcp_budget_wrapper", "--headless", "--output-mode=file", "--output-dir", "runtime", "--disable-quic", "--force-webrtc-ip-handling-policy=disable_non_proxied_udp", "--disable-features=ServiceWorker"]
required = true
enabled_tools = ["browser_navigate", "browser_navigate_back", "browser_tabs", "browser_click", "browser_type", "browser_fill_form", "browser_select_option", "browser_snapshot", "browser_find", "browser_wait_for", "browser_take_screenshot", "browser_console_messages", "browser_network_requests", "browser_network_request"]

[mcp_servers.broker]
command = "python"
args = ["-m", "scripts.browser_qa.flatkey_browser_qa.broker_mcp"]
required = true
enabled_tools = ["get_current_verification_code"]

[mcp_servers.control]
command = "python"
args = ["-m", "scripts.browser_qa.flatkey_browser_qa.control_mcp"]
required = true
enabled_tools = ["qa_replay_checkpoint", "qa_start_exploration"]
"""


def _write_private_json(path, payload):
    _write_private_text(path, json.dumps(payload, sort_keys=True))


def _write_private_text(path, text):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        handle.write(text)
