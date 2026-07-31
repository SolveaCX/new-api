import json
import os
import signal
import subprocess
import socket
import sys
import tempfile
import unittest
from unittest import mock
import io
import urllib.request

from scripts.browser_qa.flatkey_browser_qa.cleanup import CleanupResult
from scripts.browser_qa.flatkey_browser_qa import control_mcp
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


class ReadableAfterCloseStringIO(io.StringIO):
    def close(self):
        self.closed_flag = True


class FakeProcess:
    def __init__(self, returncode=0, stdout="", stderr="", finish_after=0, write_result=None, on_communicate=None):
        self.returncode = returncode
        self.stdout = io.StringIO(stdout)
        self.stderr = io.StringIO(stderr)
        self.finish_after = finish_after
        self.write_result = write_result
        self.on_communicate = on_communicate
        self.terminated = False
        self.killed = False
        self.communicated_input = None
        self.stdin = ReadableAfterCloseStringIO()
        self.wait_calls = 0
        self.wait_timeouts = []

    def poll(self):
        return self.returncode if self.wait_calls >= self.finish_after else None

    def wait(self, timeout=None):
        self.wait_timeouts.append(timeout)
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
        raise AssertionError("supervisor must stream stdout/stderr instead of communicate()")

    def finish_streams(self):
        if self.on_communicate:
            self.on_communicate()
        if self.write_result:
            self.write_result()


class RecordingThread:
    join_timeouts = []
    clock = None
    elapsed = 0.0

    def __init__(self, target):
        self.target = target
        self.started = False
        self._alive = False

    def start(self):
        self.started = True
        self._alive = True

    def join(self, timeout=None):
        self.join_timeouts.append(timeout)
        if isinstance(timeout, (int, float)):
            self.elapsed += timeout
            if self.clock is not None:
                self.clock.advance(timeout)

    def is_alive(self):
        return self._alive


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


class StopFailEvidenceSink:
    runtime_classification = None
    url = "http://127.0.0.1:1/runtime-evidence"

    def __init__(self, _redactor):
        self.started = False

    def start(self):
        self.started = True

    def stop(self):
        raise RuntimeError("evidence sink stop failed")


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

    def run_supervisor(self, process, *, result_payload=None, cleanup=None, uploader=None, preflight=None, clock=None, input_env=None, thread_factory=None):
        tmp = tempfile.mkdtemp()
        supervisor_kwargs = {}
        if thread_factory is not None:
            supervisor_kwargs["thread_factory"] = thread_factory
        sup = supervisor.Supervisor(
            env=input_env or env(),
            runtime_root=tmp,
            subprocess_runner=FakeSubprocess(process),
            uploader=uploader or FakeUploader(),
            cleanup_runner=cleanup or FakeCleanup(),
            proxy_factory=lambda: FakeProxy(),
            preflight=preflight or (lambda: {"data": {"register_enabled": True, "password_register_enabled": True, "email_verification": True, "turnstile_check": False}}),
            clock=clock or FakeClock(),
            **supervisor_kwargs,
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
        self.assertIn("--skip-git-repo-check", args)
        self.assertIn("--profile", args)
        self.assertIn("qa", args)
        self.assertIn("--output-schema", args)
        self.assertNotIn("--output-last-message", args)
        self.assertNotIn("--ignore-user-config", args)
        self.assertIn("--sandbox", args)
        self.assertIn("workspace-write", args)
        self.assertIn("--model", args)
        self.assertIn("gpt-5.4", args)
        self.assertEqual(kwargs["env"]["CODEX_API_KEY"], "sk-parentSECRET")
        self.assertNotIn("HTTP_PROXY", kwargs["env"])
        self.assertNotIn("HTTPS_PROXY", kwargs["env"])
        self.assertNotIn("ALL_PROXY", kwargs["env"])
        self.assertIs(kwargs["stdin"], subprocess.PIPE)
        prompt = process.stdin.getvalue()
        self.assertIn("$flatkey-new-user-onboarding", prompt)
        self.assertIn("staging-cloud-qa-policy.md", prompt)
        self.assertIn("Run ID: 12345", prompt)
        self.assertIn("Disposable email:", prompt)
        self.assertIn("Disposable password:", prompt)
        self.assertNotIn("CODEX_API_KEY", prompt)
        self.assertNotIn("owner@gmail.com", config_text if "config_text" in locals() else "")
        self.assertNotIn("CODEX_API_KEY", sup.env)
        self.assertEqual(kwargs["env"]["HOME"], sup.home_dir)
        if os.name == "nt":
            self.assertEqual(kwargs["env"]["USERPROFILE"], sup.home_dir)
        skill_dir = os.path.join(sup.home_dir, ".agents", "skills", "flatkey-new-user-onboarding")
        self.assertTrue(os.path.isdir(skill_dir))
        self.assertFalse(os.path.realpath(skill_dir).startswith(os.path.realpath(kwargs["cwd"]) + os.sep))
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
        self.assertNotIn("disable_response_storage", config_text)
        self.assertIn("FLATKEY_BROWSER_QA_RUNTIME_DIR", config_text)
        self.assertIn("FLATKEY_BROWSER_QA_BROKER_URL", config_text)
        playwright_env = config_text.split("[mcp_servers.playwright.env]", 1)[1].split("[mcp_servers.broker]", 1)[0]
        broker_env = config_text.split("[mcp_servers.broker.env]", 1)[1].split("[mcp_servers.control]", 1)[0]
        self.assertIn("FLATKEY_BROWSER_QA_RUNTIME_EVIDENCE_URL", playwright_env)
        self.assertIn("FLATKEY_BROWSER_QA_RUNTIME_EVIDENCE_URL", broker_env)
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
        process = FakeProcess(0, finish_after=100)
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

    def test_teardown_and_proxy_construction_failures_do_not_skip_cleanup(self):
        cleanup = FakeCleanup()
        with mock.patch.object(supervisor, "RuntimeEvidenceSink", StopFailEvidenceSink):
            outcome, sup = self.run_supervisor(FakeProcess(0), result_payload=valid_result(), cleanup=cleanup)
        self.assertEqual(outcome.status, "infrastructure_failed")
        self.assertEqual(len(cleanup.calls), 1)
        self.assertTrue(any(event["kind"] == "evidence_sink_stop_failed" for event in sup.events))

        cleanup = FakeCleanup()
        with tempfile.TemporaryDirectory() as runtime_root:
            sup = supervisor.Supervisor(
                env=env(),
                runtime_root=runtime_root,
                subprocess_runner=FakeSubprocess(FakeProcess(0)),
                uploader=FakeUploader(),
                cleanup_runner=cleanup,
                proxy_factory=lambda: (_ for _ in ()).throw(RuntimeError("proxy construction failed")),
                preflight=lambda: {"data": {"register_enabled": True, "password_register_enabled": True, "email_verification": True, "turnstile_check": False}},
                clock=FakeClock(),
            )
            outcome = sup.run(initial_result=valid_result())
        self.assertEqual(outcome.status, "infrastructure_failed")
        self.assertEqual(len(cleanup.calls), 1)

    def test_preflight_and_alias_restriction_classification_are_precise(self):
        bad_status = lambda: {"data": {"register_enabled": True, "password_register_enabled": True, "email_verification": False, "turnstile_check": False}}
        self.assertEqual(self.run_supervisor(FakeProcess(0), result_payload=valid_result(), preflight=bad_status)[0].status, "infrastructure_failed")

        alias_self_report = valid_result(
            replay={"status": "failed", "checkpoint_reached": False},
            alias_restriction=True,
        )
        self.assertEqual(self.run_supervisor(FakeProcess(0), result_payload=alias_self_report)[0].status, "infrastructure_failed")

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
        process = FakeProcess(
            1,
            stdout=json.dumps({"type": "item.completed", "item": {"type": "agent_message", "text": json.dumps(valid_result())}}) + "\n",
            stderr="Cookie: secret Authorization: Bearer sk-xSECRET\n",
        )
        outcome, sup = self.run_supervisor(process, result_payload=valid_result())
        rendered = json.dumps(outcome.to_dict()) + json.dumps(sup.events)

        for secret in ["owner@gmail.com", "password", "sk-parentSECRET", "sk-xSECRET", "Cookie: secret", "Authorization: Bearer"]:
            self.assertNotIn(secret, rendered)
        artifact_names = {os.path.basename(path) for _manifest, paths in sup.uploader.uploaded for path in paths}
        self.assertIn("codex-events.jsonl", artifact_names)
        self.assertIn("codex-stderr.txt", artifact_names)

    def test_final_result_is_extracted_from_jsonl_redacted_and_no_raw_quarantine_is_written(self):
        raw_alias = "owner+flatkey-qa-12345-secret@gmail.com"
        code = "654321"

        with tempfile.TemporaryDirectory() as tmp:
            process_holder = {}
            raw_result = valid_result(findings=[{
                "severity": "low",
                "title": "leak",
                "target_url": "https://staging-console.flatkey.ai",
                "steps": [raw_alias],
                "expected": "no leak",
                "actual": code,
                "evidence_paths": ["artifact.txt"],
                "confidence": "high",
            }], exploration={"status": "passed", "actions_used": 999})
            def notify_code():
                control_mcp.write_control_state(
                    tmp,
                    {"phase": "exploration", "monotonic_started_at": 100.0, "actions_used": 7},
                )
                request = urllib.request.Request(
                    process_holder["sup"]._evidence_url,
                    data=json.dumps({"type": "verification_code", "code": code}).encode("utf-8"),
                    headers={"Content-Type": "application/json"},
                    method="POST",
                )
                urllib.request.urlopen(request, timeout=2).close()

            process = FakeProcess(
                0,
                stdout=json.dumps({"type": "item.completed", "item": {"type": "agent_message", "text": json.dumps(raw_result)}}) + "\n",
                on_communicate=notify_code,
            )
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
            self.assertEqual(final_result["exploration"]["actions_used"], 7)
            with open(os.path.join(tmp, "manifest.json"), encoding="utf-8") as handle:
                manifest = json.load(handle)
            self.assertEqual(manifest["result"]["exploration"]["actions_used"], 7)
            self.assertFalse(os.path.exists(os.path.join(tmp, "quarantine-result.json")))

    def test_invalid_or_oversized_codex_jsonl_is_infrastructure_and_kills_process(self):
        process = FakeProcess(0, stdout="{bad-json}\n")
        outcome, _sup = self.run_supervisor(process)
        self.assertEqual(outcome.status, "infrastructure_failed")
        self.assertTrue(process.terminated)

        process = FakeProcess(0, stdout="x" * (supervisor.MAX_CODEX_STDOUT_LINE_BYTES + 1))
        outcome, _sup = self.run_supervisor(process)
        self.assertEqual(outcome.status, "infrastructure_failed")
        self.assertTrue(process.terminated)

        process = FakeProcess(0, stderr="x" * (supervisor.MAX_CODEX_STDERR_LINE_BYTES + 1))
        outcome, _sup = self.run_supervisor(process, result_payload=valid_result())
        self.assertEqual(outcome.status, "infrastructure_failed")
        self.assertTrue(process.terminated)

    def test_wait_for_codex_uses_one_shared_deadline_for_streams_and_process_wait(self):
        process = FakeProcess(0, finish_after=100)
        clock = FakeClock()
        RecordingThread.join_timeouts = []
        RecordingThread.elapsed = 0.0
        RecordingThread.clock = clock
        outcome, _sup = self.run_supervisor(process, result_payload=valid_result(), clock=clock, thread_factory=RecordingThread)

        self.assertEqual(outcome.status, "infrastructure_failed")
        self.assertTrue(process.terminated)
        self.assertTrue(RecordingThread.join_timeouts)
        self.assertLessEqual(RecordingThread.elapsed, supervisor.INTERNAL_DEADLINE_SECONDS + 2)

        process = FakeProcess(0, stdout=("x" * (supervisor.MAX_CODEX_STDOUT_LINE_BYTES + 1)) + "\n")
        outcome, _sup = self.run_supervisor(process)
        self.assertEqual(outcome.status, "infrastructure_failed")
        self.assertTrue(process.terminated)

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

    def test_main_rejects_arguments(self):
        with self.assertRaises(SystemExit):
            supervisor.main(["--unexpected"])

    def test_gcs_uploader_uses_run_execution_prefix_and_manifest_last(self):
        calls = []

        def upload(bucket, object_name, data, content_type, token, **_kwargs):
            calls.append((bucket, object_name, data, content_type, token))
            return {"name": object_name}

        with tempfile.TemporaryDirectory() as tmp:
            result_path = os.path.join(tmp, "result.json")
            events_path = os.path.join(tmp, "codex-events.jsonl")
            manifest_path = os.path.join(tmp, "manifest.json")
            for path in [result_path, events_path, manifest_path]:
                with open(path, "w", encoding="utf-8") as handle:
                    handle.write("{}")

            uploader = supervisor.GcsArtifactUploader(
                bucket="browser-qa-bucket",
                run_id="run-1",
                execution_id="exec-1",
                runtime_root=tmp,
                token_provider=lambda: "access-token",
                upload_func=upload,
            )
            uploader.upload(manifest_path, [result_path, events_path, manifest_path])

        self.assertEqual([call[1] for call in calls], [
            "runs/run-1/main/exec-1/result.json",
            "runs/run-1/main/exec-1/codex-events.jsonl",
            "runs/run-1/main/exec-1/manifest.json",
        ])

    def test_gcs_uploader_rejects_unsafe_artifacts_and_lazy_token_failure_happens_after_cleanup(self):
        with tempfile.TemporaryDirectory() as tmp:
            result_path = os.path.join(tmp, "result.json")
            manifest_path = os.path.join(tmp, "manifest.json")
            outside = os.path.join(tempfile.gettempdir(), "outside-browser-qa-artifact.txt")
            for path in [result_path, manifest_path, outside]:
                with open(path, "w", encoding="utf-8") as handle:
                    handle.write("{}")
            uploader = supervisor.GcsArtifactUploader(
                bucket="browser-qa-bucket",
                run_id="run-1",
                execution_id="exec/../bad",
                runtime_root=tmp,
                token_provider=lambda: "access-token",
                upload_func=lambda *_args, **_kwargs: {},
            )
            with self.assertRaises(ValueError):
                uploader.upload(manifest_path, [result_path, manifest_path])

            duplicate_dir = os.path.join(tmp, "duplicate")
            os.mkdir(duplicate_dir)
            duplicate_result_path = os.path.join(duplicate_dir, "result.json")
            with open(duplicate_result_path, "w", encoding="utf-8") as handle:
                handle.write("{}")
            uploader = supervisor.GcsArtifactUploader(
                bucket="browser-qa-bucket",
                run_id="run-1",
                execution_id="exec-1",
                runtime_root=tmp,
                token_provider=lambda: "access-token",
                upload_func=lambda *_args, **_kwargs: {},
            )
            try:
                with self.assertRaises(ValueError):
                    uploader.upload(manifest_path, [result_path, outside, manifest_path])

                unexpected_path = os.path.join(tmp, "unexpected.txt")
                with open(unexpected_path, "w", encoding="utf-8") as handle:
                    handle.write("unexpected")
                with self.assertRaises(ValueError):
                    uploader.upload(manifest_path, [result_path, unexpected_path, manifest_path])

                with self.assertRaises(ValueError):
                    uploader.upload(manifest_path, [result_path, duplicate_result_path, manifest_path])

                if hasattr(os, "symlink"):
                    symlink_path = os.path.join(tmp, "codex-stderr.txt")
                    try:
                        os.symlink(result_path, symlink_path)
                    except (OSError, NotImplementedError):
                        symlink_path = None
                    if symlink_path is not None:
                        with self.assertRaises(ValueError):
                            uploader.upload(manifest_path, [result_path, symlink_path, manifest_path])

                small_limit_uploader = supervisor.GcsArtifactUploader(
                    bucket="browser-qa-bucket",
                    run_id="run-1",
                    execution_id="exec-1",
                    runtime_root=tmp,
                    token_provider=lambda: "access-token",
                    upload_func=lambda *_args, **_kwargs: {},
                    max_total_bytes=1,
                )
                with self.assertRaises(ValueError):
                    small_limit_uploader.upload(manifest_path, [result_path, manifest_path])
            finally:
                try:
                    os.remove(outside)
                except FileNotFoundError:
                    pass

        with tempfile.TemporaryDirectory() as runtime_root:
            process = FakeProcess(0)
            cleanup = FakeCleanup()
            lazy_uploader = supervisor.GcsArtifactUploader(
                bucket="browser-qa-bucket",
                run_id="run-1",
                execution_id="exec-1",
                runtime_root=runtime_root,
                token_provider=lambda: (_ for _ in ()).throw(RuntimeError("lazy token unavailable")),
                upload_func=lambda *_args, **_kwargs: {},
            )
            sup = supervisor.Supervisor(
                env=env(),
                runtime_root=runtime_root,
                subprocess_runner=FakeSubprocess(process),
                uploader=lazy_uploader,
                cleanup_runner=cleanup,
                proxy_factory=lambda: FakeProxy(),
                preflight=lambda: {"data": {"register_enabled": True, "password_register_enabled": True, "email_verification": True, "turnstile_check": False}},
                clock=FakeClock(),
            )
            outcome = sup.run(initial_result=valid_result())
            self.assertEqual(outcome.status, "infrastructure_failed")
            self.assertEqual(len(cleanup.calls), 1)

    def test_main_defers_gcp_access_token_until_after_supervisor_construction(self):
        calls = []

        class FakeGcp:
            def access_token(self):
                calls.append("access_token")
                return "access-token"

        class FakeSupervisor:
            def __init__(self, **kwargs):
                calls.append("supervisor_init")
                self.kwargs = kwargs

            def run(self):
                calls.append("run")
                return supervisor.Outcome("passed", "manifest.json", [])

        fake_env = env()
        fake_env.update({
            "FLATKEY_BROWSER_QA_GCS_BUCKET": "browser-qa-bucket",
            "FLATKEY_BROWSER_QA_EXECUTION_ID": "exec-1",
        })
        with mock.patch.object(supervisor, "GcpClient", return_value=FakeGcp()), \
            mock.patch.object(supervisor, "Supervisor", FakeSupervisor), \
            mock.patch.object(supervisor, "CleanupRunner", lambda _client: FakeCleanup()), \
            mock.patch.object(supervisor, "StagingApiClient", lambda _origin: object()), \
            mock.patch.object(supervisor.os, "environ", fake_env):
            self.assertEqual(supervisor.main([]), 0)

        self.assertEqual(calls, ["supervisor_init", "run"])

    def test_runtime_evidence_sink_registers_exact_code_and_rejects_bad_events(self):
        redactor = supervisor.Redactor(email="owner+flatkey-qa-1-x@gmail.com")
        sink = supervisor.RuntimeEvidenceSink(redactor)
        sink.start()
        try:
            request = urllib.request.Request(
                sink.url,
                data=json.dumps({"type": "verification_code", "code": "654321"}).encode("utf-8"),
                headers={"Content-Type": "application/json"},
                method="POST",
            )
            with urllib.request.urlopen(request, timeout=2) as response:
                self.assertEqual(response.status, 204)
            self.assertEqual(redactor.clean("123456 654321"), "123456 [REDACTED_CODE]")

            bad = urllib.request.Request(
                sink.url,
                data=json.dumps({"type": "verification_code", "code": "12345"}).encode("utf-8"),
                headers={"Content-Type": "application/json"},
                method="POST",
            )
            with self.assertRaises(urllib.error.HTTPError):
                urllib.request.urlopen(bad, timeout=2)

            alias = urllib.request.Request(
                sink.url,
                data=json.dumps({
                    "type": "alias_restriction",
                    "failed": True,
                    "text": "管理员已启用邮箱地址别名限制，您的邮箱地址由于包含特殊符号而被拒绝。",
                }).encode("utf-8"),
                headers={"Content-Type": "application/json"},
                method="POST",
            )
            with urllib.request.urlopen(alias, timeout=2) as response:
                self.assertEqual(response.status, 204)
            self.assertEqual(sink.runtime_classification, "alias_restriction")

            wrong_path = urllib.request.Request(
                sink.url.replace("/runtime-evidence", "/wrong"),
                data=json.dumps({"type": "verification_code", "code": "111111"}).encode("utf-8"),
                headers={"Content-Type": "application/json"},
                method="POST",
            )
            with self.assertRaises(urllib.error.HTTPError):
                urllib.request.urlopen(wrong_path, timeout=2)

            with socket.create_connection((sink.host, sink.port), timeout=2) as sock:
                sock.sendall(
                    b"POST /runtime-evidence HTTP/1.1\r\n"
                    b"Host: 127.0.0.1\r\n"
                    b"Content-Type: application/json\r\n"
                    b"Transfer-Encoding: chunked\r\n\r\n0\r\n\r\n"
                )
                self.assertIn(b"400", sock.recv(256))

            body = json.dumps({"type": "verification_code", "code": "222222"}).encode("utf-8")
            with socket.create_connection((sink.host, sink.port), timeout=2) as sock:
                sock.sendall(
                    b"POST /runtime-evidence HTTP/1.1\r\n"
                    b"Host: 127.0.0.1\r\n"
                    b"Content-Type: application/json\r\n"
                    b"Content-Length: 1\r\n"
                    b"Content-Length: " + str(len(body)).encode("ascii") + b"\r\n\r\n" + body
                )
                self.assertIn(b"400", sock.recv(256))
        finally:
            sink.stop()


if __name__ == "__main__":
    unittest.main()
