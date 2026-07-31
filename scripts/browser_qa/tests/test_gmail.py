import copy
import json
import os
import unittest
import urllib.error
from pathlib import Path
from unittest import mock

from scripts.browser_qa.flatkey_browser_qa.gmail import (
    GmailClient,
    GmailConfigError,
    GmailInvalidGrant,
    GmailTransientError,
    OAuthCredentials,
    VerificationSearch,
    parse_verification_code,
)


FIXTURE = Path(__file__).parent / "fixtures" / "gmail_flatkey_message.json"
ALIAS = "owner+flatkey-qa-123456789-abc123def4@gmail.com"


def load_message():
    return json.loads(FIXTURE.read_text(encoding="utf-8"))


class FakeResponse:
    def __init__(self, status, payload):
        self.status = status
        self.payload = payload
        self.headers = {}

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


class GmailParserTests(unittest.TestCase):
    def test_extracts_single_code_from_matching_plain_and_html_message(self):
        code = parse_verification_code(
            load_message(),
            alias=ALIAS,
            sender="noreply@flatkey.ai",
            subject_marker="Flatkey Email Verification",
            run_start_epoch=1800000000,
            now_epoch=1800000030,
        )

        self.assertEqual(code, "654321")

    def test_accepts_real_html_only_email_shape(self):
        message = load_message()
        message["payload"]["mimeType"] = "text/html"
        message["payload"]["parts"] = []
        message["payload"]["body"] = {"data": "PGRpdj5Zb3VyIGNvZGUgaXMgNjU0MzIxPC9kaXY-"}

        code = parse_verification_code(
            message,
            alias=ALIAS,
            sender="noreply@flatkey.ai",
            subject_marker="Flatkey Email Verification",
            run_start_epoch=1800000000,
            now_epoch=1800000030,
        )

        self.assertEqual(code, "654321")

    def test_rejects_wrong_alias_old_time_sender_subject_and_unrelated_mail(self):
        cases = []
        wrong_alias = load_message()
        wrong_alias["payload"]["headers"][1]["value"] = "owner+other@gmail.com"
        cases.append(wrong_alias)

        old = load_message()
        old["internalDate"] = "1799999999000"
        cases.append(old)

        future = load_message()
        future["internalDate"] = "1800004000000"
        cases.append(future)

        wrong_sender = load_message()
        wrong_sender["payload"]["headers"][0]["value"] = "Flatkey <support@example.com>"
        cases.append(wrong_sender)

        wrong_subject = load_message()
        wrong_subject["payload"]["headers"][2]["value"] = "Welcome"
        cases.append(wrong_subject)

        unrelated = {"id": "x", "internalDate": "1800000005000", "payload": {"headers": [], "body": {}}}
        cases.append(unrelated)

        for message in cases:
            with self.subTest(message=message.get("id")):
                self.assertIsNone(
                    parse_verification_code(
                        message,
                        alias=ALIAS,
                        sender="noreply@flatkey.ai",
                        subject_marker="Flatkey Email Verification",
                        run_start_epoch=1800000000,
                        now_epoch=1800000030,
                    )
                )

    def test_rejects_distinct_two_codes_attachment_only_and_malformed_payloads(self):
        two_codes = load_message()
        two_codes["payload"]["parts"][0]["body"]["data"] = "Q29kZXM6IDExMTExMSBhbmQgMjIyMjIy"
        two_codes["payload"]["parts"][1]["body"]["data"] = "Q29kZXM6IDExMTExMSBhbmQgMjIyMjIy"
        attachment_only = load_message()
        attachment_only["payload"]["parts"] = [{"mimeType": "application/pdf", "body": {"data": "NjU0MzIx"}}]
        malformed = load_message()
        malformed["payload"]["parts"][0]["body"]["data"] = "not base64!!!"
        malformed["payload"]["parts"][1]["body"]["data"] = "not base64!!!"

        for message in [two_codes, attachment_only, malformed]:
            self.assertIsNone(
                parse_verification_code(
                    message,
                    alias=ALIAS,
                    sender="noreply@flatkey.ai",
                    subject_marker="Flatkey Email Verification",
                    run_start_epoch=1800000000,
                    now_epoch=1800000030,
                )
            )


class GmailOAuthAndClientTests(unittest.TestCase):
    def valid_oauth_json(self):
        return json.dumps(
            {
                "refresh_token": "refresh-secret",
                "token_uri": "https://oauth2.googleapis.com/token",
                "client_id": "client-id",
                "client_secret": "client-secret",
                "scopes": ["https://www.googleapis.com/auth/gmail.readonly"],
            }
        )

    def test_oauth_credentials_load_only_from_env_and_do_not_leak_repr(self):
        with mock.patch.dict(os.environ, {"GMAIL_OAUTH_JSON": self.valid_oauth_json()}, clear=True):
            creds = OAuthCredentials.from_env()

        self.assertNotIn("refresh-secret", repr(creds))
        self.assertNotIn("client-secret", repr(creds))

    def test_oauth_credentials_reject_missing_broad_scope_and_untrusted_token_uri(self):
        bad_values = [
            {},
            {
                "refresh_token": "r",
                "token_uri": "https://oauth2.googleapis.com/token",
                "client_id": "c",
                "client_secret": "s",
                "scopes": ["https://mail.google.com/"],
            },
            {
                "refresh_token": "r",
                "token_uri": "https://accounts.google.com/o/oauth2/token",
                "client_id": "c",
                "client_secret": "s",
                "scopes": ["https://www.googleapis.com/auth/gmail.readonly"],
            },
        ]
        for value in bad_values:
            with self.subTest(value=value):
                with mock.patch.dict(os.environ, {"GMAIL_OAUTH_JSON": json.dumps(value)}, clear=True):
                    with self.assertRaises(GmailConfigError):
                        OAuthCredentials.from_env()

    def test_refresh_posts_form_data_and_classifies_invalid_grant_without_retry(self):
        opener = RecordingOpener(
            [
                urllib.error.HTTPError(
                    "https://oauth2.googleapis.com/token",
                    400,
                    "bad",
                    {},
                    io_bytes({"error": "invalid_grant"}),
                )
            ]
        )
        creds = OAuthCredentials.from_json(self.valid_oauth_json())
        client = GmailClient(creds, opener=opener, retry_base_delay=0)

        with self.assertRaises(GmailInvalidGrant):
            client.refresh_access_token()

        self.assertEqual(len(opener.requests), 1)
        request, timeout = opener.requests[0]
        self.assertEqual(request.full_url, "https://oauth2.googleapis.com/token")
        self.assertEqual(request.get_method(), "POST")
        self.assertEqual(request.headers["Content-type"], "application/x-www-form-urlencoded")
        self.assertIn(b"grant_type=refresh_token", request.data)
        self.assertEqual(timeout, 10)

    def test_refresh_retries_transient_responses_only_three_times(self):
        opener = RecordingOpener(
            [
                urllib.error.HTTPError("https://oauth2.googleapis.com/token", 503, "busy", {}, io_bytes({})),
                urllib.error.HTTPError("https://oauth2.googleapis.com/token", 429, "busy", {}, io_bytes({})),
                urllib.error.URLError("timeout"),
            ]
        )
        client = GmailClient(OAuthCredentials.from_json(self.valid_oauth_json()), opener=opener, retry_base_delay=0)

        with self.assertRaises(GmailTransientError):
            client.refresh_access_token()

        self.assertEqual(len(opener.requests), 3)

    def test_search_uses_profile_base_alias_bounded_query_and_full_message_get(self):
        opener = RecordingOpener(
            [
                FakeResponse(200, {"access_token": "access-secret", "expires_in": 3600, "token_type": "Bearer"}),
                FakeResponse(200, {"emailAddress": "owner@gmail.com"}),
                FakeResponse(200, {"messages": [{"id": "msg-1"}]}),
                FakeResponse(200, load_message()),
            ]
        )
        client = GmailClient(OAuthCredentials.from_json(self.valid_oauth_json()), opener=opener, retry_base_delay=0)

        result = client.find_verification_code(
            VerificationSearch(
                email_tag="flatkey-qa-123456789-abc123def4",
                run_start_epoch=1800000000,
                sender="noreply@flatkey.ai",
                subject_marker="Flatkey Email Verification",
                now_epoch=1800000030,
            )
        )

        self.assertEqual(result, "654321")
        urls = [request.full_url for request, _ in opener.requests]
        self.assertIn("https://gmail.googleapis.com/gmail/v1/users/me/profile", urls[1])
        self.assertIn("q=to%3Aowner%2Bflatkey-qa-123456789-abc123def4%40gmail.com+after%3A1800000000", urls[2])
        self.assertIn("maxResults=10", urls[2])
        self.assertIn("format=full", urls[3])
        for request, _ in opener.requests[1:]:
            self.assertEqual(request.headers["Authorization"], "Bearer access-secret")


def io_bytes(payload):
    class Body:
        def read(self, _limit=-1):
            return json.dumps(payload).encode("utf-8")

        def close(self):
            return None

    return Body()


if __name__ == "__main__":
    unittest.main()
