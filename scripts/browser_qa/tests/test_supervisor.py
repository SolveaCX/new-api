import json
import os
import signal
import subprocess
import sys
import tempfile
import unittest

from scripts.browser_qa.flatkey_browser_qa.cleanup import CleanupResult
from scripts.browser_qa.flatkey_browser_qa import supervisor


class FakeClock:
    def __init__(self):
        self.now = 100.0

    def monotonic(self):
        return self.now

    def time(self):
        return 1700000000

    def advance(self, seconds):
        self.now += seconds


class FakeProcess:
    def __init__(self, returncode=0, stdout="", stderr="", finish_after=0, write_result=None, on_communicate=None):
        self.returncode = returncode
        self.stdout = stdout
        self.stderr = stderr
        self.finish_after = finish_after
        self.write_result = write_result
        self.on_communicate = on_communicate
        self.terminated = False
        self.killed = False
        self.communicated_input = None
        self.wait_calls = 0

    def poll(self):
        return self.returncode if self.wait_calls >= self.finish_after else None

    def wait(self, timeout=None):
        self.wait_calls += 1
        if self.wait_calls >= self.finish_after:
            return self.returncode
        raise TimeoutError("still running")

    def terminate(self):
        self.terminated = True
        self.returncode = -15

    def kill(self):
        self.killed = True

    def communicate(self, input=None, timeout=None):
        self.communicated_input = input
        if self.on_communicate:
            self.on_communicate()
        if self.write_result:
            self.write_result()
        return self.stdout, self.stderr


class FakeSubprocess:
    def __init__(self, process):
        self.process = process
        self.calls = []

    def popen(self, args, **kwargs):
        self.calls.append((args, kwargs))
        return self.process


class FakeSignals:
    SIGTERM = signal.SIGTERM
    SIGINT = signal.SIGINT

    def __init__(self):
        self.handlers = {}
        self.installed = []

    def signal(self, signum, handler):
        previous = self.handlers.get(signum, "previous")
        self.handlers[signum] = handler
        self.installed.append((signum, handler))
        return previous


class FakeUploader:
    def __init__(self, fail=False):
        self.fail = fail
        self.uploaded = []

    def upload(self, manifest_path, artifact_paths):
        self.uploaded.append((manifest_path, list(artifact_paths)))
        if self.fail:
            raise RuntimeError("Authorization: Bearer sk-uploadSECRET")
        return {"uploaded": True}


class FakeStatusResponse:
    status = 200

    def __init__(self, payload):
        self.payload = payload

    def read(self, _limit=-1):
        return json.dumps(self.payload).encode("utf-8")

    def __enter__(self):
        return self

    def __exit__(self, *_):
        return False


class FakeStatusOpener:
    def __init__(self, response):
        self.response = response
        self.requests = []

    def open(self, request, timeout=0):
        self.requests.append((request, timeout))
        return self.response


class FakeCleanup:
    def __init__(self, result=None):
        self.result = result or CleanupResult(0, True, True, False, "cleanup verified")
        self.calls = []

    def run(self, identity):
        self.calls.append(identity)
        return self.result


class FakeProxy:
    def __init__(self):
        self.started = False
        self.stopped = False
        self.host = "127.0.0.1"
        self.port = 4567

    def start(self):
        self.started = True
        return self

    def stop(self):
        self.stopped = True


def env():
    return {
        "CODEX_API_KEY": "sk-parentSECRET",
        "FLATKEY_QA_RUN_ID": "12345",
        "FLATKEY_QA_IDENTITY_SEED_B64": "c2VlZC13aXRoLTMyLWJ5dGVzLW1pbmltdW0tdmFsdWU=",
        "FLATKEY_QA_GMAIL_BASE": "owner@gmail.com",
        "FLATKEY_QA_WEBSITE_ORIGIN": "https://staging-website.flatkey.ai",
        "FLATKEY_QA_CONSOLE_ORIGIN": "https://staging-console.flatkey.ai",
        "FLATKEY_QA_DOCS_ORIGIN": "https://docs.flatkey.ai",
        "FLATKEY_BROWSER_QA_BROKER_URL": "https://flatkey-staging-browser-qa-broker-abc-uw.a.run.app/",
    }


def valid_result(**overrides):
    payload = {
        "replay": {"status": "passed", "checkpoint_reached": True},
        "exploration": {"status": "passed", "actions_used": 0},
        "budgets": {"replay_seconds": 300, "exploration_seconds": 300, "max_actions": 30},
        "findings": [],
    }
    payload.update(overrides)
    return payload


class SupervisorTests(unittest.TestCase):
    def test_prompt_file_declares_skill_policy_runtime_contract_and_forbidden_scope(self):
        prompt_path = os.path.join(os.path.dirname(__file__), "..", "config", "qa-prompt.md")
        with open(prompt_path, encoding="utf-8") as handle:
            prompt = handle.read()
        for required in [
            "$flatkey-new-user-onboarding",
            "staging-cloud-qa-policy.md",
            "run id",
            "qa_replay_checkpoint",
            "qa_start_exploration",
            "5 minutes",
            "30 actions",
            "production",
            "payment",
            "subscription",
            "invite",
            "admin",
            "global settings",
            "real model calls",
            "CAPTCHA",
            "runtime cleanup",
            "cookie-free docs",
        ]:
            self.assertIn(required.lower(), prompt.lower())

    def run_supervisor(self, process, *, result_payload=None, cleanup=None, uploader=None, preflight=None, clock=None, input_env=None):
        tmp = tempfile.mkdtemp()
        sup = supervisor.Supervisor(
            env=input_env or env(),
            runtime_root=tmp,
            subprocess_runner=FakeSubprocess(process),
            uploader=uploader or FakeUploader(),
            cleanup_runner=cleanup or FakeCleanup(),
            proxy_factory=lambda: FakeProxy(),
            preflight=preflight or (lambda: {"data": {"register_enabled": True, "password_register_enabled": True, "email_verification": True, "turnstile_check": False}}),
            clock=clock or FakeClock(),
        )
        outcome = sup.run(initial_result=result_payload)
        return outcome, sup

    def test_normal_run_uses_owner_only_codex_home_profile_schema_proxy_cleanup_upload_and_pops_api_key(self):
        process = FakeProcess(0)
        outcome, sup = self.run_supervisor(process, result_payload=valid_result())

        self.assertEqual(outcome.status, "passed")
        self.assertEqual(len(sup.cleanup_runner.calls), 1)
        self.assertEqual(len(sup.uploader.uploaded), 1)
        args, kwargs = sup.subprocess_runner.calls[0]
        self.assertIn("--strict-config", args)
        self.assertIn("--profile", args)
        self.assertIn("qa", args)
        self.assertIn("--output-schema", args)
        self.assertIn("--sandbox", args)
        self.assertIn("workspace-write", args)
        self.assertIn("--model", args)
        self.assertIn("gpt-5.4", args)
        self.assertEqual(kwargs["env"]["CODEX_API_KEY"], "sk-parentSECRET")
        self.assertNotIn("HTTP_PROXY", kwargs["env"])
        self.assertNotIn("HTTPS_PROXY", kwargs["env"])
        self.assertNotIn("ALL_PROXY", kwargs["env"])
        self.assertIs(kwargs["stdin"], subprocess.PIPE)
        self.assertIn("$flatkey-new-user-onboarding", process.communicated_input)
        self.assertIn("staging-cloud-qa-policy.md", process.communicated_input)
        self.assertIn("Run ID: 12345", process.communicated_input)
        self.assertIn("Disposable email:", process.communicated_input)
        self.assertIn("Disposable password:", process.communicated_input)
        self.assertNotIn("CODEX_API_KEY", process.communicated_input)
        self.assertNotIn("owner@gmail.com", config_text if "config_text" in locals() else "")
        self.assertNotIn("CODEX_API_KEY", sup.env)
        if os.name != "nt":
            self.assertEqual(oct(os.stat(sup.codex_home).st_mode & 0o777), "0o700")
        else:
            self.assertTrue(os.path.isdir(sup.codex_home))
        config_text = ""
        for path in [os.path.join(sup.codex_home, "config.toml"), os.path.join(sup.codex_home, "qa.config.toml")]:
            if os.name != "nt":
                self.assertEqual(oct(os.stat(path).st_mode & 0o777), "0o600")
            else:
                self.assertTrue(os.path.isfile(path))
            with open(path, encoding="utf-8") as handle:
                data = handle.read()
            config_text += data
        self.assertNotIn("proxy_bypass", config_text)
        self.assertIn("network_access = false", config_text)
        self.assertIn("[shell_environment_policy]", config_text)
        self.assertIn("inherit = \"none\"", config_text)
        self.assertIn("web_search = \"disabled\"", config_text)
        self.assertNotIn("[profiles.qa]", config_text)
        self.assertNotIn("config = \"qa.config.toml\"", config_text)
        self.assertIn("FLATKEY_BROWSER_QA_RUNTIME_DIR", config_text)
        self.assertIn("FLATKEY_BROWSER_QA_BROKER_URL", config_text)
        self.assertIn("PATH", config_text)
        self.assertIn("PYTHONPATH", config_text)
        self.assertNotIn("${FLATKEY_BROWSER_QA_RUN_ID}", config_text)
        self.assertIn(sys.executable.replace("\\", "\\\\"), config_text)
        self.assertNotIn("owner+", config_text)
        self.assertNotIn(sup._identity.password, config_text)
        for data in [config_text]:
            self.assertNotIn("browser_evaluate", data)
            self.assertNotIn("plugins = false", data)

    def test_nonzero_invalid_timeout_sigterm_upload_and_cleanup_failures_still_cleanup_then_upload_with_priority(self):
        self.assertEqual(self.run_supervisor(FakeProcess(7), result_payload=valid_result())[0].status, "infrastructure_failed")
        self.assertEqual(self.run_supervisor(FakeProcess(0), result_payload={"bad": "shape"})[0].status, "infrastructure_failed")

        clock = FakeClock()
        process = FakeProcess(0, on_communicate=lambda: (_ for _ in ()).throw(subprocess.TimeoutExpired("codex", 840)))
        outcome, sup = self.run_supervisor(process, result_payload=valid_result(), clock=clock)
        self.assertEqual(outcome.status, "infrastructure_failed")
        self.assertTrue(process.terminated)
        self.assertEqual(len(sup.cleanup_runner.calls), 1)
        self.assertEqual(len(sup.uploader.uploaded), 1)

        outcome, sup = self.run_supervisor(FakeProcess(0), result_payload=valid_result(), uploader=FakeUploader(fail=True))
        self.assertEqual(outcome.status, "infrastructure_failed")

        cleanup = FakeCleanup(CleanupResult(0, False, False, True, "password cleanup failed"))
        outcome, sup = self.run_supervisor(FakeProcess(7), result_payload=valid_result(), cleanup=cleanup)
        self.assertEqual(outcome.status, "cleanup_failed")
        self.assertEqual(len(sup.uploader.uploaded), 1)

    def test_preflight_and_alias_restriction_classification_are_precise(self):
        bad_status = lambda: {"data": {"register_enabled": True, "password_register_enabled": True, "email_verification": False, "turnstile_check": False}}
        self.assertEqual(self.run_supervisor(FakeProcess(0), result_payload=valid_result(), preflight=bad_status)[0].status, "infrastructure_failed")

        alias_failure = valid_result(
            replay={"status": "failed", "checkpoint_reached": False},
            infrastructure={"classification": "alias_restriction", "status": "failed"},
        )
        self.assertEqual(self.run_supervisor(FakeProcess(0), result_payload=alias_failure)[0].status, "infrastructure_failed")

        missing_broker_env = env()
        del missing_broker_env["FLATKEY_BROWSER_QA_BROKER_URL"]
        self.assertEqual(self.run_supervisor(FakeProcess(0), result_payload=valid_result(), input_env=missing_broker_env)[0].status, "infrastructure_failed")

    def test_default_preflight_client_uses_exact_staging_status_contract(self):
        payload = {"data": {"register_enabled": True, "password_register_enabled": True, "email_verification": True, "turnstile_check": False}}
        opener = FakeStatusOpener(FakeStatusResponse(payload))
        preflight = supervisor.StagingStatusPreflight("https://staging-console.flatkey.ai", opener=opener)

        self.assertEqual(preflight(), payload)
        request, timeout = opener.requests[0]
        self.assertEqual(request.full_url, "https://staging-console.flatkey.ai/api/status")
        self.assertEqual(request.get_method(), "GET")
        self.assertEqual(timeout, 5)

        with self.assertRaises(ValueError):
            supervisor.StagingStatusPreflight("https://flatkey.ai", opener=opener)
        with self.assertRaises(RuntimeError):
            supervisor.StagingStatusPreflight(
                "https://staging-console.flatkey.ai",
                opener=FakeStatusOpener(FakeStatusResponse({"data": {"registration": True, "password_login": True}})),
            )()

    def test_stdout_stderr_artifacts_and_outcome_do_not_leak_secrets(self):
        process = FakeProcess(1, stdout="owner@gmail.com+flatkey password sk-parentSECRET 654321", stderr="Cookie: secret Authorization: Bearer sk-xSECRET")
        outcome, sup = self.run_supervisor(process, result_payload=valid_result())
        rendered = json.dumps(outcome.to_dict()) + json.dumps(sup.events)

        for secret in ["owner@gmail.com", "password", "sk-parentSECRET", "sk-xSECRET", "654321", "Cookie: secret", "Authorization: Bearer"]:
            self.assertNotIn(secret, rendered)

    def test_quarantine_result_is_redacted_validated_atomically_written_and_not_uploaded(self):
        raw_alias = "owner+flatkey-qa-12345-secret@gmail.com"
        code = "654321"

        with tempfile.TemporaryDirectory() as tmp:
            process_holder = {}

            def write_raw_result():
                args, _kwargs = process_holder["sup"].subprocess_runner.calls[0]
                raw_path = args[args.index("--output-last-message") + 1]
                raw_password = process_holder["sup"]._identity.password
                with open(raw_path, "w", encoding="utf-8") as handle:
                    json.dump(valid_result(findings=[{
                        "severity": "low",
                        "title": "leak",
                        "target_url": "https://staging-console.flatkey.ai",
                        "steps": [raw_alias],
                        "expected": "no leak",
                        "actual": raw_password + " " + code,
                        "evidence_paths": ["artifact.txt"],
                        "confidence": "high",
                    }]), handle)

            process = FakeProcess(0, write_result=write_raw_result)
            uploader = FakeUploader()
            sup = supervisor.Supervisor(
                env=env(),
                runtime_root=tmp,
                subprocess_runner=FakeSubprocess(process),
                uploader=uploader,
                cleanup_runner=FakeCleanup(),
                proxy_factory=lambda: FakeProxy(),
                preflight=lambda: {"data": {"register_enabled": True, "password_register_enabled": True, "email_verification": True, "turnstile_check": False}},
                clock=FakeClock(),
            )
            process_holder["sup"] = sup
            outcome = sup.run()

            self.assertEqual(outcome.status, "findings_detected")
            uploaded = json.dumps(uploader.uploaded)
            self.assertNotIn("quarantine", uploaded)
            with open(os.path.join(tmp, "result.json"), encoding="utf-8") as handle:
                final_result = json.load(handle)
            rendered = json.dumps(final_result) + json.dumps(outcome.to_dict())
            for secret in [raw_alias, sup._identity.password, code, "owner@gmail.com", "flatkey-qa-12345-secret"]:
                self.assertNotIn(secret, rendered)
            self.assertFalse(os.path.exists(os.path.join(tmp, "quarantine-result.json")))

    def test_signal_handler_terminates_active_codex_then_cleanup_upload_and_restores_handlers(self):
        signals = FakeSignals()
        cleanup = FakeCleanup()
        uploader = FakeUploader()
        with tempfile.TemporaryDirectory() as tmp:
            process = FakeProcess(0, on_communicate=lambda: signals.handlers[signal.SIGTERM](signal.SIGTERM, None))
            sup = supervisor.Supervisor(
                env=env(),
                runtime_root=tmp,
                subprocess_runner=FakeSubprocess(process),
                uploader=uploader,
                cleanup_runner=cleanup,
                proxy_factory=lambda: FakeProxy(),
                preflight=lambda: {"data": {"register_enabled": True, "password_register_enabled": True, "email_verification": True, "turnstile_check": False}},
                clock=FakeClock(),
                signal_module=signals,
            )
            outcome = sup.run(initial_result=valid_result())

            self.assertEqual(outcome.status, "infrastructure_failed")
            self.assertTrue(process.terminated)
            self.assertEqual(len(cleanup.calls), 1)
            self.assertEqual(len(uploader.uploaded), 1)
            self.assertEqual(signals.handlers[signal.SIGTERM], "previous")
            self.assertEqual(signals.handlers[signal.SIGINT], "previous")


if __name__ == "__main__":
    unittest.main()
