import http.client
import http.server
import json
import socket
import socketserver
import threading
import unittest

from scripts.browser_qa.flatkey_browser_qa import egress_proxy


class FakeConnector:
    def __init__(self):
        self.calls = []

    def __call__(self, sockaddr, timeout):
        self.calls.append((sockaddr, timeout))
        raise OSError("connect stopped by test")


class ThreadedServer(socketserver.ThreadingMixIn, http.server.HTTPServer):
    daemon_threads = True
    allow_reuse_address = True


class RedirectHandler(http.server.BaseHTTPRequestHandler):
    redirect_to = "https://flatkey.ai/blocked"

    def do_GET(self):
        if self.path == "/redirect":
            self.send_response(302)
            self.send_header("Location", self.redirect_to)
            self.send_header("Content-Length", "0")
            self.end_headers()
            return
        if self.path == "/relative-redirect":
            self.send_response(302)
            self.send_header("Location", "/safe-target")
            self.send_header("Content-Length", "0")
            self.end_headers()
            return
        self.send_response(200)
        self.send_header("Content-Type", "text/plain")
        self.send_header("Content-Length", "7")
        self.end_headers()
        self.wfile.write(b"allowed")

    def log_message(self, _format, *args):
        return


class EgressPolicyTests(unittest.TestCase):
    def test_allowlist_file_is_versioned_deny_by_default_and_blocks_private_literal_hosts(self):
        policy = egress_proxy.EgressPolicy.from_file()

        self.assertTrue(policy.is_allowed_host("staging-website.flatkey.ai"))
        self.assertTrue(policy.is_allowed_host("staging-console.flatkey.ai"))
        self.assertTrue(policy.is_allowed_host("docs.flatkey.ai"))
        self.assertFalse(policy.is_allowed_host("flatkey.ai"))
        self.assertFalse(policy.is_allowed_host("assets.example.com"))
        self.assertFalse(policy.is_allowed_host("localhost"))
        self.assertFalse(policy.is_allowed_host("127.0.0.1"))
        self.assertFalse(policy.is_allowed_host("0177.0.0.1"))
        self.assertFalse(policy.is_allowed_host("169.254.169.254"))
        self.assertFalse(policy.is_allowed_host("http://staging-console.flatkey.ai"))

    def test_dns_resolution_fails_closed_on_any_private_answer_and_uses_verified_sockaddr(self):
        policy = egress_proxy.EgressPolicy({"staging-console.flatkey.ai"})

        def mixed_resolver(host, port, *_):
            return [
                (socket.AF_INET, socket.SOCK_STREAM, 6, "", ("93.184.216.34", port)),
                (socket.AF_INET, socket.SOCK_STREAM, 6, "", ("10.0.0.1", port)),
            ]

        with self.assertRaises(egress_proxy.PolicyDenied):
            policy.resolve("staging-console.flatkey.ai", 443, resolver=mixed_resolver)

        def public_resolver(host, port, *_):
            return [(socket.AF_INET, socket.SOCK_STREAM, 6, "", ("93.184.216.34", port))]

        resolved = policy.resolve("staging-console.flatkey.ai", 443, resolver=public_resolver)
        self.assertEqual(resolved[0].sockaddr, ("93.184.216.34", 443))

    def test_connect_checks_host_before_opening_socket_and_never_logs_secret_headers(self):
        connector = FakeConnector()
        policy = egress_proxy.EgressPolicy({"staging-console.flatkey.ai"})
        proxy = egress_proxy.EgressProxy(policy=policy, resolver=lambda *_: [], connector=connector)
        proxy.start()
        try:
            conn = http.client.HTTPConnection(proxy.host, proxy.port, timeout=2)
            conn.set_tunnel("flatkey.ai", 443, headers={"Authorization": "Bearer sk-secretSECRET", "Cookie": "secret=value"})
            with self.assertRaises((OSError, http.client.HTTPException)):
                conn.request("GET", "/")
            self.assertEqual(connector.calls, [])
            self.assertNotIn("sk-secretSECRET", json.dumps(proxy.events))
            self.assertNotIn("secret=value", json.dumps(proxy.events))
        finally:
            proxy.stop()

    def test_redirect_target_is_rechecked_and_non_http_methods_are_blocked(self):
        policy = egress_proxy.EgressPolicy({"staging-console.flatkey.ai"})
        self.assertEqual(policy.check_url("http://staging-console.flatkey.ai/login").host, "staging-console.flatkey.ai")
        with self.assertRaises(egress_proxy.PolicyDenied):
            policy.check_url("https://flatkey.ai/login")
        with self.assertRaises(egress_proxy.PolicyDenied):
            policy.check_url("ftp://staging-console.flatkey.ai/file")

        response = egress_proxy.filter_response_headers(
            302,
            [("Location", "https://flatkey.ai/blocked"), ("Content-Length", "0")],
            policy,
            base_url="http://staging-console.flatkey.ai/start",
        )
        self.assertEqual(response.status, 502)
        self.assertEqual(response.headers, [])

        response = egress_proxy.filter_response_headers(
            302,
            [("Location", "/safe-target"), ("Content-Length", "0")],
            policy,
            base_url="http://staging-console.flatkey.ai/start",
        )
        self.assertEqual(response.status, 302)

    def test_allowed_http_is_forwarded_to_verified_sockaddr_and_redirect_is_rechecked(self):
        upstream = ThreadedServer(("127.0.0.1", 0), RedirectHandler)
        thread = threading.Thread(target=upstream.serve_forever, daemon=True)
        thread.start()
        port = upstream.server_address[1]
        resolver_calls = []

        def resolver(host, resolved_port, *_):
            resolver_calls.append((host, resolved_port))
            return [(socket.AF_INET, socket.SOCK_STREAM, 6, "", ("93.184.216.34", resolved_port))]

        def connector(sockaddr, timeout):
            self.assertEqual(sockaddr, ("93.184.216.34", 80))
            return socket.create_connection(("127.0.0.1", port), timeout=timeout)

        proxy = egress_proxy.EgressProxy(
            policy=egress_proxy.EgressPolicy({"staging-console.flatkey.ai"}),
            resolver=resolver,
            connector=connector,
        )
        proxy.start()
        try:
            conn = http.client.HTTPConnection(proxy.host, proxy.port, timeout=2)
            conn.request("GET", "http://staging-console.flatkey.ai/")
            response = conn.getresponse()
            self.assertEqual(response.status, 200)
            self.assertEqual(response.read(), b"allowed")
            conn.close()

            conn = http.client.HTTPConnection(proxy.host, proxy.port, timeout=2)
            conn.request("GET", "http://staging-console.flatkey.ai/redirect")
            response = conn.getresponse()
            self.assertEqual(response.status, 502)
            self.assertEqual(response.read(), b"")
            conn.close()
            self.assertEqual(resolver_calls, [("staging-console.flatkey.ai", 80), ("staging-console.flatkey.ai", 80)])
        finally:
            proxy.stop()
            upstream.shutdown()
            upstream.server_close()

    def test_header_body_bounds_and_thread_local_decisions(self):
        proxy = egress_proxy.EgressProxy(policy=egress_proxy.EgressPolicy({"staging-console.flatkey.ai"}), max_header_bytes=32)
        proxy.start()
        try:
            with socket.create_connection((proxy.host, proxy.port), timeout=2) as sock:
                sock.sendall(b"GET http://staging-console.flatkey.ai/ HTTP/1.1\r\nHost: staging-console.flatkey.ai\r\nX-Oversized: " + b"a" * 80 + b"\r\n\r\n")
                data = sock.recv(256)
            self.assertIn(b"431", data)
        finally:
            proxy.stop()

        proxy = egress_proxy.EgressProxy(policy=egress_proxy.EgressPolicy({"staging-console.flatkey.ai"}))
        proxy.start()
        try:
            results = []

            def hit(host):
                conn = http.client.HTTPConnection(proxy.host, proxy.port, timeout=2)
                try:
                    conn.request("GET", f"http://{host}/", headers={"Host": host})
                    results.append(conn.getresponse().status)
                except Exception:
                    results.append("error")
                finally:
                    conn.close()

            threads = [threading.Thread(target=hit, args=("flatkey.ai",)), threading.Thread(target=hit, args=("localhost",))]
            for thread in threads:
                thread.start()
            for thread in threads:
                thread.join()
            self.assertEqual(results.count(403), 2)
        finally:
            proxy.stop()

    def test_proxy_rejects_ports_methods_userinfo_ambiguous_length_and_transfer_encoding(self):
        policy = egress_proxy.EgressPolicy({"staging-console.flatkey.ai"})
        with self.assertRaises(egress_proxy.PolicyDenied):
            policy.check_url("https://staging-console.flatkey.ai/")
        with self.assertRaises(egress_proxy.PolicyDenied):
            policy.check_url("http://staging-console.flatkey.ai:81/")
        with self.assertRaises(egress_proxy.PolicyDenied):
            policy.check_url("http://user@staging-console.flatkey.ai/")
        with self.assertRaises(egress_proxy.PolicyDenied):
            policy.check_connect_host("staging-console.flatkey.ai:444")

        proxy = egress_proxy.EgressProxy(policy=policy)
        proxy.start()
        try:
            conn = http.client.HTTPConnection(proxy.host, proxy.port, timeout=2)
            conn.request("PUT", "http://staging-console.flatkey.ai/")
            self.assertEqual(conn.getresponse().status, 405)
            conn.close()

            with socket.create_connection((proxy.host, proxy.port), timeout=2) as sock:
                sock.sendall(
                    b"POST http://staging-console.flatkey.ai/ HTTP/1.1\r\n"
                    b"Host: staging-console.flatkey.ai\r\n"
                    b"Content-Length: 1\r\n"
                    b"Content-Length: 2\r\n\r\nx"
                )
                self.assertIn(b"400", sock.recv(256))

            with socket.create_connection((proxy.host, proxy.port), timeout=2) as sock:
                sock.sendall(
                    b"POST http://staging-console.flatkey.ai/ HTTP/1.1\r\n"
                    b"Host: staging-console.flatkey.ai\r\n"
                    b"Transfer-Encoding: chunked\r\n\r\n0\r\n\r\n"
                )
                self.assertIn(b"400", sock.recv(256))
        finally:
            proxy.stop()


if __name__ == "__main__":
    unittest.main()
