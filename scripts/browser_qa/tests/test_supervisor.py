import json
import os
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
    def __init__(self, returncode=0, stdout="", stderr="", finish_after=0):
        self.returncode = returncode
        self.stdout = stdout
        self.stderr = stderr
        self.finish_after = finish_after
        self.terminated = False
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

    def communicate(self, timeout=None):
        return self.stdout, self.stderr


class FakeSubprocess:
    def __init__(self, process):
        self.process = process
        self.calls = []

    def popen(self, args, **kwargs):
        self.calls.append((args, kwargs))
        return self.process


class FakeUploader:
    def __init__(self, fail=False):
        self.fail = fail
        self.uploaded = []

    def upload(self, manifest_path, artifact_paths):
        self.uploaded.append((manifest_path, list(artifact_paths)))
        if self.fail:
            raise RuntimeError("Authorization: Bearer sk-uploadSECRET")
        return {"uploaded": True}


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
    def run_supervisor(self, process, *, result_payload=None, cleanup=None, uploader=None, preflight=None, clock=None, input_env=None):
        tmp = tempfile.mkdtemp()
        sup = supervisor.Supervisor(
            env=input_env or env(),
            runtime_root=tmp,
            subprocess_runner=FakeSubprocess(process),
            uploader=uploader or FakeUploader(),
            cleanup_runner=cleanup or FakeCleanup(),
            proxy_factory=lambda: FakeProxy(),
            preflight=preflight or (lambda: {"data": {"email_verification": True, "turnstile_check": False}}),
            clock=clock or FakeClock(),
        )
        outcome = sup.run(initial_result=result_payload)
        return outcome, sup

    def test_normal_run_uses_owner_only_codex_home_profile_schema_proxy_cleanup_upload_and_pops_api_key(self):
        outcome, sup = self.run_supervisor(FakeProcess(0), result_payload=valid_result())

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
        self.assertIn("proxy_bypass = \"\"", config_text)
        self.assertIn("network_access = false", config_text)
        self.assertIn("--headless", config_text)
        self.assertIn("--output-mode=file", config_text)
        self.assertIn("--disable-quic", config_text)
        self.assertIn("--force-webrtc-ip-handling-policy=disable_non_proxied_udp", config_text)
        self.assertIn("--disable-features=ServiceWorker", config_text)
        for data in [config_text]:
            self.assertNotIn("browser_evaluate", data)
            self.assertNotIn("plugins = false", data)

    def test_nonzero_invalid_timeout_sigterm_upload_and_cleanup_failures_still_cleanup_then_upload_with_priority(self):
        self.assertEqual(self.run_supervisor(FakeProcess(7), result_payload=valid_result())[0].status, "replay_failed")
        self.assertEqual(self.run_supervisor(FakeProcess(0), result_payload={"bad": "shape"})[0].status, "infrastructure_failed")

        clock = FakeClock()
        process = FakeProcess(0, finish_after=999)
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
        bad_status = lambda: {"data": {"email_verification": False, "turnstile_check": False}}
        self.assertEqual(self.run_supervisor(FakeProcess(0), result_payload=valid_result(), preflight=bad_status)[0].status, "infrastructure_failed")

        alias_failure = valid_result(
            replay={"status": "failed", "checkpoint_reached": False},
            infrastructure={"classification": "alias_restriction", "status": "failed"},
        )
        self.assertEqual(self.run_supervisor(FakeProcess(0), result_payload=alias_failure)[0].status, "infrastructure_failed")

    def test_stdout_stderr_artifacts_and_outcome_do_not_leak_secrets(self):
        process = FakeProcess(1, stdout="owner@gmail.com+flatkey password sk-parentSECRET", stderr="Cookie: secret Authorization: Bearer sk-xSECRET")
        outcome, sup = self.run_supervisor(process, result_payload=valid_result())
        rendered = json.dumps(outcome.to_dict()) + json.dumps(sup.events)

        for secret in ["owner@gmail.com", "password", "sk-parentSECRET", "sk-xSECRET", "Cookie: secret", "Authorization: Bearer"]:
            self.assertNotIn(secret, rendered)


if __name__ == "__main__":
    unittest.main()
