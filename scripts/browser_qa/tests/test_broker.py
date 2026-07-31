import http.client
import json
import threading
import time
import unittest
from contextlib import redirect_stdout
from http.server import ThreadingHTTPServer
from io import StringIO

from scripts.browser_qa.flatkey_browser_qa.broker import BrokerConfig, VerificationBroker, make_handler


class FakeGmail:
    base_address = "owner@gmail.com"

    def __init__(self, code=None):
        self.code = code
        self.searches = []

    def find_verification_code(self, search):
        self.searches.append(search)
        return self.code


class BrokerTests(unittest.TestCase):
    def start_broker(self, gmail, *, log=None):
        kwargs = {
            "sender": "noreply@flatkey.ai",
            "subject_marker": "Flatkey Email Verification",
            "now": lambda: 1800000060,
        }
        if log == "default":
            config = BrokerConfig(**kwargs)
        else:
            config = BrokerConfig(**kwargs, log=log or (lambda _event: None))
        broker = VerificationBroker(gmail, config)
        server = ThreadingHTTPServer(("127.0.0.1", 0), make_handler(broker))
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        return server, thread

    def request(self, server, method, path, body=None, headers=None):
        host, port = server.server_address
        conn = http.client.HTTPConnection(host, port, timeout=5)
        raw = None if body is None else json.dumps(body).encode("utf-8")
        conn.request(method, path, body=raw, headers=headers or {"Content-Type": "application/json"})
        response = conn.getresponse()
        payload = response.read()
        conn.close()
        return response.status, json.loads(payload.decode("utf-8"))

    def test_returns_ready_or_pending_only_for_valid_current_code_request(self):
        gmail = FakeGmail("123456")
        server, thread = self.start_broker(gmail)
        try:
            status, payload = self.request(
                server,
                "POST",
                "/v1/current-code",
                {"run_id": "123456789", "email_tag": "flatkey-qa-123456789-abc123def4", "start_time": 1800000000},
            )
            pending_status, pending_payload = self.request(
                server,
                "POST",
                "/v1/current-code",
                {"run_id": "123456789", "email_tag": "flatkey-qa-123456789-abc123def4", "start_time": 1800000000},
            )
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=2)

        self.assertEqual(status, 200)
        self.assertEqual(payload, {"status": "ready", "code": "123456"})
        self.assertEqual(pending_status, 200)
        self.assertEqual(pending_payload, {"status": "ready", "code": "123456"})
        self.assertEqual(gmail.searches[0].email_tag, "flatkey-qa-123456789-abc123def4")
        self.assertEqual(gmail.searches[0].run_start_epoch, 1800000000)

    def test_rejects_method_path_content_type_unknown_fields_and_alias_tampering(self):
        cases = [
            ("GET", "/v1/current-code", None, {"Content-Type": "application/json"}, "method_not_allowed"),
            ("POST", "/wrong", {}, {"Content-Type": "application/json"}, "not_found"),
            ("POST", "/v1/current-code", {}, {"Content-Type": "text/plain"}, "unsupported_media_type"),
            (
                "POST",
                "/v1/current-code",
                {"run_id": "123", "email_tag": "flatkey-qa-123-abcdefghij", "start_time": 1800000000, "base": "owner@gmail.com"},
                {"Content-Type": "application/json"},
                "invalid_fields",
            ),
            (
                "POST",
                "/v1/current-code",
                {"run_id": "123", "email_tag": "flatkey-qa-124-abcdefghij", "start_time": 1800000000},
                {"Content-Type": "application/json"},
                "invalid_email_tag",
            ),
            (
                "POST",
                "/v1/current-code",
                {"run_id": "１２３", "email_tag": "flatkey-qa-123-abcdefghij", "start_time": 1800000000},
                {"Content-Type": "application/json"},
                "invalid_run_id",
            ),
        ]
        server, thread = self.start_broker(FakeGmail())
        try:
            for method, path, body, headers, code in cases:
                with self.subTest(code=code):
                    status, payload = self.request(server, method, path, body, headers)
                    self.assertGreaterEqual(status, 400)
                    self.assertEqual(payload, {"error": code})
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=2)

    def test_rejects_stale_future_oversized_chunked_invalid_json_and_malformed_tags(self):
        server, thread = self.start_broker(FakeGmail())
        host, port = server.server_address
        try:
            stale = self.request(
                server,
                "POST",
                "/v1/current-code",
                {"run_id": "123", "email_tag": "flatkey-qa-123-abcdefghij", "start_time": 1799990000},
            )
            future = self.request(
                server,
                "POST",
                "/v1/current-code",
                {"run_id": "123", "email_tag": "flatkey-qa-123-abcdefghij", "start_time": 1800000400},
            )
            bad_tag = self.request(
                server,
                "POST",
                "/v1/current-code",
                {"run_id": "123", "email_tag": "flatkey-qa-123-ABCDEF1234", "start_time": 1800000000},
            )

            conn = http.client.HTTPConnection(host, port, timeout=5)
            conn.putrequest("POST", "/v1/current-code")
            conn.putheader("Content-Type", "application/json")
            conn.putheader("Transfer-Encoding", "chunked")
            conn.endheaders()
            conn.send(b"0\r\n\r\n")
            chunked_response = conn.getresponse()
            chunked = (chunked_response.status, json.loads(chunked_response.read().decode("utf-8")))
            conn.close()

            conn = http.client.HTTPConnection(host, port, timeout=5)
            conn.request("POST", "/v1/current-code", body=b"x" * 4097, headers={"Content-Type": "application/json"})
            oversized_response = conn.getresponse()
            oversized = (oversized_response.status, json.loads(oversized_response.read().decode("utf-8")))
            conn.close()

            conn = http.client.HTTPConnection(host, port, timeout=5)
            conn.request("POST", "/v1/current-code", body=b"{", headers={"Content-Type": "application/json"})
            invalid_response = conn.getresponse()
            invalid = (invalid_response.status, json.loads(invalid_response.read().decode("utf-8")))
            conn.close()
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=2)

        self.assertEqual(stale[1], {"error": "invalid_time_window"})
        self.assertEqual(future[1], {"error": "invalid_time_window"})
        self.assertEqual(bad_tag[1], {"error": "invalid_email_tag"})
        self.assertEqual(chunked, (411, {"error": "length_required"}))
        self.assertEqual(oversized, (413, {"error": "body_too_large"}))
        self.assertEqual(invalid[1], {"error": "invalid_json"})

    def test_access_logs_and_structured_logs_do_not_include_secrets(self):
        stdout = StringIO()
        server, thread = self.start_broker(FakeGmail("123456"), log="default")
        try:
            with redirect_stdout(stdout):
                self.request(
                    server,
                    "POST",
                    "/v1/current-code",
                    {"run_id": "123", "email_tag": "flatkey-qa-123-abcdefghij", "start_time": 1800000000},
                )
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=2)

        output = stdout.getvalue()
        self.assertIn('"run_id": "123"', output)
        for secret in ["owner", "gmail", "abcdefghij", "123456", "flatkey-qa"]:
            self.assertNotIn(secret, output)


if __name__ == "__main__":
    unittest.main()
