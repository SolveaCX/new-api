import json
import unittest
import urllib.error

from scripts.browser_qa.flatkey_browser_qa.gcp import (
    GcpClient,
    GcpConfigError,
    GcpTransientError,
    upload_gcs_object,
)


class FakeResponse:
    def __init__(self, status, payload):
        self.status = status
        self.payload = payload

    def read(self, _limit=-1):
        if isinstance(self.payload, bytes):
            return self.payload
        return json.dumps(self.payload).encode("utf-8")

    def __enter__(self):
        return self

    def __exit__(self, *_):
        return False


class RecordingOpener:
    def __init__(self, responses):
        self.responses = list(responses)
        self.requests = []

    def open(self, request, timeout=0):
        self.requests.append((request, timeout))
        response = self.responses.pop(0)
        if isinstance(response, Exception):
            raise response
        return response


class GcpTests(unittest.TestCase):
    def test_metadata_identity_token_uses_exact_host_path_header_and_https_audience(self):
        opener = RecordingOpener([FakeResponse(200, b"id-token")])
        client = GcpClient(opener=opener, retry_base_delay=0)

        token = client.identity_token("https://flatkey-staging-browser-qa-broker-abc-uw.a.run.app")

        self.assertEqual(token, "id-token")
        request, timeout = opener.requests[0]
        self.assertEqual(request.host, "metadata.google.internal")
        self.assertEqual(request.selector.split("?", 1)[0], "/computeMetadata/v1/instance/service-accounts/default/identity")
        self.assertIn("audience=https%3A%2F%2Fflatkey-staging-browser-qa-broker-abc-uw.a.run.app", request.selector)
        self.assertEqual(request.headers["Metadata-flavor"], "Google")
        self.assertEqual(timeout, 5)

    def test_metadata_rejects_non_https_audience_and_retries_transient_only(self):
        client = GcpClient(opener=RecordingOpener([]), retry_base_delay=0)
        with self.assertRaises(GcpConfigError):
            client.identity_token("http://service")

        opener = RecordingOpener(
            [
                urllib.error.HTTPError("http://metadata.google.internal/x", 503, "busy", {}, io_bytes({})),
                urllib.error.URLError("timeout"),
                urllib.error.URLError("timeout"),
            ]
        )
        client = GcpClient(opener=opener, retry_base_delay=0)
        with self.assertRaises(GcpTransientError):
            client.access_token()
        self.assertEqual(len(opener.requests), 3)

    def test_access_token_and_gcs_upload_use_bounded_google_origins_without_secret_repr(self):
        opener = RecordingOpener(
            [
                FakeResponse(200, {"access_token": "access-secret", "expires_in": 3600}),
                FakeResponse(200, {"name": "reports/run/result.json"}),
            ]
        )
        client = GcpClient(opener=opener, retry_base_delay=0)

        token = client.access_token()
        upload_gcs_object(
            "flatkey-browser-qa-reports",
            "reports/123/result.json",
            b"{}",
            "application/json",
            token,
            opener=opener,
        )

        self.assertEqual(token, "access-secret")
        self.assertNotIn("access-secret", repr(client))
        upload_request = opener.requests[1][0]
        self.assertEqual(upload_request.host, "storage.googleapis.com")
        self.assertIn("/upload/storage/v1/b/flatkey-browser-qa-reports/o", upload_request.selector)
        self.assertIn("ifGenerationMatch=0", upload_request.selector)
        self.assertEqual(upload_request.headers["Authorization"], "Bearer access-secret")
        self.assertEqual(upload_request.headers["Content-type"], "application/json")

    def test_gcs_upload_validates_bucket_object_and_rejects_redirects(self):
        token = "access-secret"
        for bucket, name in [("Bad_Bucket", "x"), ("ok-bucket", "/absolute"), ("ok-bucket", "../escape")]:
            with self.subTest(bucket=bucket, name=name):
                with self.assertRaises(GcpConfigError):
                    upload_gcs_object(bucket, name, b"x", "text/plain", token, opener=RecordingOpener([]))

        opener = RecordingOpener([urllib.error.HTTPError("https://storage.googleapis.com/x", 302, "redirect", {}, io_bytes({}))])
        with self.assertRaises(GcpConfigError):
            upload_gcs_object("ok-bucket", "reports/x", b"x", "text/plain", token, opener=opener)


def io_bytes(payload):
    class Body:
        def read(self, _limit=-1):
            return json.dumps(payload).encode("utf-8")

        def close(self):
            return None

    return Body()


if __name__ == "__main__":
    unittest.main()
