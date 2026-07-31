import json
import base64
import contextlib
import io
import threading
import unittest
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from unittest import mock
from urllib.parse import parse_qs, urlparse

from scripts.browser_qa.flatkey_browser_qa import cleanup_job
from scripts.browser_qa.flatkey_browser_qa.cleanup import CleanupRunner
from scripts.browser_qa.flatkey_browser_qa.cleanup import CleanupResult
from scripts.browser_qa.flatkey_browser_qa.api import LoginResult
from scripts.browser_qa.flatkey_browser_qa.api import StagingApiClient
from scripts.browser_qa.flatkey_browser_qa.identity import DerivedIdentity


IDENTITY = DerivedIdentity(
    run_id="123456789",
    username="qa12345678abcdef12",
    email_tag="flatkey-qa-123456789-abcdef1234",
    password="Aa1!secret-password-for-tests",
    key_name="cloud-qa-123456789",
)


class FakeStagingApi:
    def __init__(self, tokens=None):
        self.tokens = list(tokens if tokens is not None else [1])
        self.account_exists = True
        self.sessions = {}
        self.list_calls = 0
        self.delete_calls = {}
        self.login_calls = 0
        self.self_delete_calls = 0
        self.malformed_list = False
        self.duplicate_pages = False
        self.total_override = None
        self.one_delete_404 = set()
        self.transient_delete_503 = set()
        self.lost_delete_response = set()
        self.record_not_found_after_lost = set()
        self.record_not_found_without_delete = set()
        self.persistent_delete_5xx = set()
        self.account_delete_failure = False
        self.account_delete_lost_then_503 = False
        self.sequence = []
        self.login_response = None
        self.login_redirect_to = None
        self.duplicate_first_page_item = False

    def start(self):
        fake = self

        class Handler(BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"

            def log_message(self, *_):
                return

            def _read_json(self):
                length = int(self.headers.get("Content-Length", "0"))
                body = self.rfile.read(length) if length else b"{}"
                return json.loads(body.decode("utf-8"))

            def _send_json(self, status, payload, extra_headers=None):
                raw = json.dumps(payload).encode("utf-8")
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(raw)))
                for key, value in (extra_headers or {}).items():
                    self.send_header(key, value)
                self.end_headers()
                self.wfile.write(raw)

            def _authenticated_user(self):
                cookie = self.headers.get("Cookie", "")
                user_id = self.headers.get("New-Api-User")
                if not user_id or user_id not in fake.sessions.values():
                    return None
                if f"session={user_id}" not in cookie:
                    return None
                return user_id

            def do_POST(self):
                if self.path != "/api/user/login":
                    self._send_json(HTTPStatus.NOT_FOUND, {"success": False, "message": "not found"})
                    return
                fake.login_calls += 1
                fake.sequence.append(("login", None))
                body = self._read_json()
                if fake.login_redirect_to is not None:
                    self.send_response(HTTPStatus.FOUND)
                    self.send_header("Location", fake.login_redirect_to)
                    self.send_header("Content-Length", "0")
                    self.end_headers()
                    return
                if fake.login_response is not None:
                    status, payload = fake.login_response
                    self._send_json(status, payload)
                    return
                if not fake.account_exists:
                    self._send_json(
                        HTTPStatus.OK,
                        {"success": False, "message": "Username or password is incorrect, or user has been banned"},
                    )
                    return
                if body.get("username") != IDENTITY.username or body.get("password") != IDENTITY.password:
                    self._send_json(HTTPStatus.OK, {"success": False, "message": "invalid username or password"})
                    return
                user_id = "123"
                fake.sessions[f"session={user_id}"] = user_id
                self._send_json(
                    HTTPStatus.OK,
                    {"success": True, "data": {"id": int(user_id)}},
                    {"Set-Cookie": f"session={user_id}; Path=/; HttpOnly"},
                )

            def do_GET(self):
                parsed = urlparse(self.path)
                if parsed.path != "/api/token/":
                    self._send_json(HTTPStatus.NOT_FOUND, {"success": False, "message": "not found"})
                    return
                if self._authenticated_user() is None:
                    self._send_json(HTTPStatus.UNAUTHORIZED, {"success": False, "message": "unauthorized"})
                    return
                fake.list_calls += 1
                fake.sequence.append(("list", fake.list_calls))
                if fake.malformed_list:
                    self._send_json(HTTPStatus.OK, {"success": True, "data": {"items": "not-a-list", "total": 1}})
                    return
                query = parse_qs(parsed.query)
                page = int(query.get("p", ["1"])[0])
                size = int(query.get("size", ["100"])[0])
                if fake.duplicate_pages and page > 1:
                    items = fake.tokens[: min(size, len(fake.tokens))]
                else:
                    items = fake.tokens[(page - 1) * size : page * size]
                if fake.duplicate_first_page_item and page == 1 and items:
                    items = [items[0], *items]
                payload_items = [{"id": token_id} for token_id in items]
                total = fake.total_override if fake.total_override is not None else len(fake.tokens)
                self._send_json(HTTPStatus.OK, {"success": True, "data": {"items": payload_items, "total": total}})

            def do_DELETE(self):
                if self._authenticated_user() is None:
                    self._send_json(HTTPStatus.UNAUTHORIZED, {"success": False, "message": "unauthorized"})
                    return
                if self.path == "/api/user/self":
                    fake.self_delete_calls += 1
                    if fake.account_delete_lost_then_503 and fake.self_delete_calls == 1:
                        fake.account_exists = False
                        fake.tokens.clear()
                        self.close_connection = True
                        return
                    if fake.account_delete_lost_then_503:
                        self._send_json(HTTPStatus.SERVICE_UNAVAILABLE, {"success": False, "message": "busy"})
                        return
                    if fake.account_delete_failure:
                        self._send_json(HTTPStatus.INTERNAL_SERVER_ERROR, {"success": False, "message": "failed"})
                        return
                    fake.account_exists = False
                    fake.tokens.clear()
                    self._send_json(HTTPStatus.OK, {"success": True, "data": True})
                    return
                if not self.path.startswith("/api/token/"):
                    self._send_json(HTTPStatus.NOT_FOUND, {"success": False, "message": "not found"})
                    return
                try:
                    token_id = int(self.path.rsplit("/", 1)[-1])
                except ValueError:
                    self._send_json(HTTPStatus.NOT_FOUND, {"success": False, "message": "not found"})
                    return
                fake.delete_calls[token_id] = fake.delete_calls.get(token_id, 0) + 1
                fake.sequence.append(("delete", token_id))
                if token_id in fake.persistent_delete_5xx:
                    self._send_json(HTTPStatus.SERVICE_UNAVAILABLE, {"success": False, "message": "busy"})
                    return
                if token_id in fake.transient_delete_503 and fake.delete_calls[token_id] == 1:
                    self._send_json(HTTPStatus.SERVICE_UNAVAILABLE, {"success": False, "message": "busy"})
                    return
                if token_id in fake.record_not_found_without_delete:
                    self._send_json(HTTPStatus.OK, {"success": False, "message": "record not found"})
                    return
                if token_id in fake.one_delete_404:
                    fake.one_delete_404.remove(token_id)
                    self._send_json(HTTPStatus.NOT_FOUND, {"success": False, "message": "not found"})
                    return
                if token_id in fake.tokens:
                    fake.tokens.remove(token_id)
                if token_id in fake.lost_delete_response and fake.delete_calls[token_id] == 1:
                    self.close_connection = True
                    return
                if token_id in fake.record_not_found_after_lost and token_id not in fake.tokens:
                    self._send_json(HTTPStatus.OK, {"success": False, "message": "record not found"})
                    return
                self._send_json(HTTPStatus.OK, {"success": True, "data": True})

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        return self

    @property
    def origin(self):
        host, port = self.server.server_address
        return f"http://{host}:{port}"

    def stop(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=2)


class TargetServer:
    def __init__(self):
        self.hits = 0

    def start(self):
        target = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_):
                return

            def do_GET(self):
                target.hits += 1
                raw = b"redirect target"
                self.send_response(HTTPStatus.OK)
                self.send_header("Content-Length", str(len(raw)))
                self.end_headers()
                self.wfile.write(raw)

            def do_POST(self):
                target.hits += 1
                self.do_GET()

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        return self

    @property
    def url(self):
        host, port = self.server.server_address
        return f"http://{host}:{port}/redirect-target"

    def stop(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=2)


class StubCleanupClient:
    def __init__(self, list_tokens, *, delete_result=True):
        self._list_tokens = list_tokens
        self.delete_result = delete_result
        self.list_calls = 0
        self.deleted = []
        self.user_id = None
        self.cookies = []

    def login(self, _username, _password):
        self.user_id = "123"
        return LoginResult(user_id="123")

    def list_tokens(self, *, page, size):
        self.list_calls += 1
        return self._list_tokens(page, size)

    def delete_token(self, token_id):
        self.deleted.append(token_id)
        return self.delete_result

    def delete_self(self):
        return True

    def clear_cookies(self):
        self.user_id = None


class CleanupTests(unittest.TestCase):
    def make_client(self, fake):
        return StagingApiClient(fake.origin, allow_test_origin=True, retry_base_delay=0)

    def run_cleanup(self, fake, page_size=100, max_attempts=3):
        client = self.make_client(fake)
        return CleanupRunner(client, page_size=page_size, max_attempts=max_attempts).run(IDENTITY)

    def test_deletes_205_tokens_across_pages_and_verifies_account_absence(self):
        fake = FakeStagingApi(list(range(1, 206))).start()
        fake.one_delete_404.add(17)
        fake.transient_delete_503.add(18)
        try:
            result = self.run_cleanup(fake)
        finally:
            fake.stop()

        self.assertFalse(result.cleanup_failed, result.reason)
        self.assertEqual(result.deleted_token_count, 205)
        self.assertTrue(result.account_deleted)
        self.assertTrue(result.login_rejected_after_delete)
        self.assertGreaterEqual(fake.list_calls, 4)
        self.assertNotIn(18, fake.tokens)
        first_delete_index = next(index for index, item in enumerate(fake.sequence) if item[0] == "delete")
        self.assertEqual(
            [item for item in fake.sequence[:first_delete_index] if item[0] == "list"],
            [("list", 1), ("list", 2), ("list", 3)],
        )

    def test_constructor_accepts_only_exact_production_staging_origin_by_default(self):
        StagingApiClient("https://staging-console.flatkey.ai")
        with self.assertRaises(ValueError):
            StagingApiClient("https://staging-console.flatkey.ai/")
        with self.assertRaises(ValueError):
            StagingApiClient("http://127.0.0.1:1234")
        for origin in [
            "https://staging-console.flatkey.ai/path",
            "https://staging-console.flatkey.ai?x=1",
            "https://staging-console.flatkey.ai#frag",
            "https://user:pass@staging-console.flatkey.ai",
        ]:
            with self.subTest(origin=origin):
                with self.assertRaises(ValueError):
                    StagingApiClient(origin, allow_test_origin=True)

    def test_redirects_fail_closed_without_visiting_target_origin(self):
        target = TargetServer().start()
        fake = FakeStagingApi([]).start()
        fake.login_redirect_to = target.url
        try:
            result = self.run_cleanup(fake)
        finally:
            fake.stop()
            target.stop()

        self.assertTrue(result.cleanup_failed)
        self.assertEqual(target.hits, 0)

    def test_already_absent_requires_explicit_authentication_rejection(self):
        fake = FakeStagingApi([]).start()
        fake.account_exists = False
        try:
            result = self.run_cleanup(fake)
        finally:
            fake.stop()

        self.assertFalse(result.cleanup_failed, result.reason)
        self.assertEqual(result.deleted_token_count, 0)
        self.assertFalse(result.account_deleted)
        self.assertTrue(result.login_rejected_after_delete)

    def test_real_i18n_auth_rejection_is_accepted_directly(self):
        fake = FakeStagingApi([]).start()
        fake.account_exists = False
        client = self.make_client(fake)
        try:
            login = client.login(IDENTITY.username, IDENTITY.password)
        finally:
            fake.stop()

        self.assertTrue(login.auth_rejected)
        self.assertIsNone(client.user_id)

    def test_malformed_pagination_fails_closed(self):
        fake = FakeStagingApi(["token-1"]).start()
        fake.malformed_list = True
        try:
            result = self.run_cleanup(fake)
        finally:
            fake.stop()

        self.assertTrue(result.cleanup_failed)
        self.assertIn("pagination", result.reason)

    def test_bool_total_fails_closed(self):
        fake = FakeStagingApi([1]).start()
        fake.total_override = True
        try:
            result = self.run_cleanup(fake)
        finally:
            fake.stop()

        self.assertTrue(result.cleanup_failed)
        self.assertIn("pagination", result.reason)
        self.assertEqual(fake.self_delete_calls, 0)

    def test_page_size_must_match_service_contract_limit(self):
        fake = FakeStagingApi([]).start()
        client = self.make_client(fake)
        try:
            with self.assertRaises(ValueError):
                CleanupRunner(client, page_size=0)
            with self.assertRaises(ValueError):
                CleanupRunner(client, page_size=101)
            with self.assertRaises(ValueError):
                CleanupRunner(client, max_pages=0)
            with self.assertRaises(ValueError):
                CleanupRunner(client, max_pages=True)
        finally:
            fake.stop()

    def test_repeated_page_with_huge_total_fails_without_unbounded_paging(self):
        client = StubCleanupClient(lambda _page, _size: (["1", "2"], 10**12))

        result = CleanupRunner(client, page_size=2, max_pages=3).run(IDENTITY)

        self.assertTrue(result.cleanup_failed)
        self.assertIn("pagination", result.reason)
        self.assertEqual(client.list_calls, 2)
        self.assertEqual(client.deleted, [])

    def test_new_ids_beyond_max_pages_fail_closed(self):
        client = StubCleanupClient(lambda page, _size: ([str(page)], 10**12))

        result = CleanupRunner(client, page_size=1, max_pages=3).run(IDENTITY)

        self.assertTrue(result.cleanup_failed)
        self.assertIn("pagination", result.reason)
        self.assertEqual(client.list_calls, 3)
        self.assertEqual(client.deleted, [])

    def test_repeated_ids_across_real_pages_converge_by_observation(self):
        fake = FakeStagingApi([1, 2, 3]).start()
        fake.duplicate_first_page_item = True
        try:
            result = self.run_cleanup(fake, page_size=2)
        finally:
            fake.stop()

        self.assertFalse(result.cleanup_failed, result.reason)
        self.assertEqual(result.deleted_token_count, 3)
        self.assertEqual(fake.tokens, [])

    def test_empty_items_with_positive_total_fails_closed_without_deleting_account(self):
        fake = FakeStagingApi([1, 2]).start()
        fake.total_override = 3
        try:
            result = self.run_cleanup(fake, page_size=2)
        finally:
            fake.stop()

        self.assertTrue(result.cleanup_failed)
        self.assertIn("pagination", result.reason)
        self.assertFalse(result.account_deleted)
        self.assertEqual(fake.self_delete_calls, 0)

    def test_lost_delete_response_is_reentered_until_empty_listing(self):
        fake = FakeStagingApi([99]).start()
        fake.lost_delete_response.add(99)
        fake.record_not_found_after_lost.add(99)
        try:
            result = self.run_cleanup(fake)
        finally:
            fake.stop()

        self.assertFalse(result.cleanup_failed, result.reason)
        self.assertEqual(result.deleted_token_count, 0)
        self.assertGreaterEqual(fake.delete_calls[99], 1)
        last_delete_index = max(index for index, item in enumerate(fake.sequence) if item[0] == "delete")
        self.assertIn(("list", fake.list_calls), fake.sequence[last_delete_index + 1 :])

    def test_delete_false_without_absence_proof_reports_zero_deleted(self):
        client = StubCleanupClient(lambda _page, _size: (["77"], 1), delete_result=False)

        result = CleanupRunner(client, max_attempts=2).run(IDENTITY)

        self.assertTrue(result.cleanup_failed)
        self.assertIn("token", result.reason)
        self.assertEqual(result.deleted_token_count, 0)
        self.assertEqual(client.deleted, ["77", "77"])

    def test_record_not_found_without_absence_proof_fails_after_relisting(self):
        fake = FakeStagingApi([77]).start()
        fake.record_not_found_without_delete.add(77)
        try:
            result = self.run_cleanup(fake)
        finally:
            fake.stop()

        self.assertTrue(result.cleanup_failed)
        self.assertIn("token", result.reason)
        first_delete_index = next(index for index, item in enumerate(fake.sequence) if item[0] == "delete")
        self.assertTrue(any(item[0] == "list" for item in fake.sequence[first_delete_index + 1 :]))

    def test_persistent_5xx_leaves_cleanup_failed(self):
        fake = FakeStagingApi([500]).start()
        fake.persistent_delete_5xx.add(500)
        client = self.make_client(fake)
        try:
            result = CleanupRunner(client).run(IDENTITY)
        finally:
            fake.stop()

        self.assertTrue(result.cleanup_failed)
        self.assertIn("token", result.reason)
        self.assertEqual(fake.delete_calls[500], 3)
        self.assertIsNone(client.user_id)
        self.assertEqual(len(list(client.cookies)), 0)

    def test_account_delete_failure_is_cleanup_failed(self):
        fake = FakeStagingApi([]).start()
        fake.account_delete_failure = True
        try:
            result = self.run_cleanup(fake)
        finally:
            fake.stop()

        self.assertTrue(result.cleanup_failed)
        self.assertIn("account", result.reason)
        self.assertFalse(result.login_rejected_after_delete)

    def test_lost_account_delete_response_is_verified_by_rejected_login(self):
        fake = FakeStagingApi([]).start()
        fake.account_delete_lost_then_503 = True
        try:
            result = self.run_cleanup(fake)
        finally:
            fake.stop()

        self.assertFalse(result.cleanup_failed, result.reason)
        self.assertTrue(result.account_deleted)
        self.assertTrue(result.login_rejected_after_delete)
        self.assertGreaterEqual(fake.self_delete_calls, 2)

    def test_second_cleanup_run_observes_already_absent(self):
        fake = FakeStagingApi([1, 2]).start()
        try:
            first = self.run_cleanup(fake)
            second = self.run_cleanup(fake)
        finally:
            fake.stop()

        self.assertFalse(first.cleanup_failed, first.reason)
        self.assertFalse(second.cleanup_failed, second.reason)
        self.assertTrue(second.login_rejected_after_delete)
        self.assertEqual(second.deleted_token_count, 0)

    def test_client_rejects_non_integer_user_and_token_ids(self):
        bad_user = FakeStagingApi([]).start()
        bad_user.login_response = (HTTPStatus.OK, {"success": True, "data": {"id": "123"}})
        try:
            bad_user_result = self.run_cleanup(bad_user)
        finally:
            bad_user.stop()

        bad_token = FakeStagingApi(["1"]).start()
        try:
            bad_token_result = self.run_cleanup(bad_token)
        finally:
            bad_token.stop()

        self.assertTrue(bad_user_result.cleanup_failed)
        self.assertTrue(bad_token_result.cleanup_failed)
        self.assertIn("pagination", bad_token_result.reason)

    def test_200_success_false_that_is_not_auth_rejection_fails_closed(self):
        fake = FakeStagingApi([]).start()
        fake.login_response = (HTTPStatus.OK, {"success": False, "message": "password login disabled"})
        try:
            result = self.run_cleanup(fake)
        finally:
            fake.stop()

        self.assertTrue(result.cleanup_failed)
        self.assertFalse(result.login_rejected_after_delete)

    def test_malformed_login_success_and_login_5xx_fail_closed(self):
        malformed = FakeStagingApi([]).start()
        malformed.login_response = (HTTPStatus.OK, {"success": True, "data": {}})
        try:
            malformed_result = self.run_cleanup(malformed)
        finally:
            malformed.stop()

        server_error = FakeStagingApi([]).start()
        server_error.login_response = (HTTPStatus.INTERNAL_SERVER_ERROR, {"success": False, "message": "busy"})
        try:
            server_error_result = self.run_cleanup(server_error)
        finally:
            server_error.stop()

        self.assertTrue(malformed_result.cleanup_failed)
        self.assertTrue(server_error_result.cleanup_failed)
        self.assertFalse(malformed_result.login_rejected_after_delete)
        self.assertFalse(server_error_result.login_rejected_after_delete)

    def test_cleanup_job_output_omits_username_password_and_seed(self):
        seed = b"seed-with-32-bytes-minimum-value"
        env = {
            "FLATKEY_QA_RUN_ID": IDENTITY.run_id,
            "FLATKEY_QA_IDENTITY_SEED_B64": base64.b64encode(seed).decode("ascii"),
            "FLATKEY_QA_CONSOLE_ORIGIN": "https://staging-console.flatkey.ai",
        }

        class FakeRunner:
            def __init__(self, _client):
                pass

            def run(self, _identity):
                return CleanupResult(3, True, True, False, "cleanup verified")

        stdout = io.StringIO()
        with mock.patch.dict("os.environ", env, clear=True):
            with mock.patch.object(cleanup_job, "StagingApiClient", lambda _origin: object()):
                with mock.patch.object(cleanup_job, "CleanupRunner", FakeRunner):
                    with contextlib.redirect_stdout(stdout):
                        exit_code = cleanup_job.main([])

        output = stdout.getvalue()
        self.assertEqual(exit_code, 0)
        for secret in [IDENTITY.username, IDENTITY.password, "owner", "gmail", "seed-with"]:
            self.assertNotIn(secret, output)


if __name__ == "__main__":
    unittest.main()
