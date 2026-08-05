import base64
import hashlib
import hmac
import json
import os
import re
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass


FINAL_STATUSES = frozenset(
    {"passed", "findings_detected", "replay_failed", "infrastructure_failed", "cleanup_failed"}
)
REPLAY_STATUSES = frozenset({"passed", "failed", "unknown"})
EXPLORATION_STATUSES = frozenset({"passed", "failed", "not_started", "unknown"})
CLEANUP_STATUSES = frozenset({"passed", "cleanup_failed", "unknown"})
FINDING_SEVERITIES = frozenset({"critical", "high", "medium", "low"})
FINDING_CONFIDENCE = frozenset({"low", "medium", "high"})
GITHUB_RUN_URL = re.compile(
    r"https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+/actions/runs/[1-9][0-9]*"
)
GCS_MANIFEST_URI = re.compile(
    r"gs://[a-z0-9][a-z0-9._-]{1,220}[a-z0-9]/runs/[0-9]+/manifest\.json"
)
URLSAFE_B64 = re.compile(r"[A-Za-z0-9_-]+={0,2}")
EMAIL_ADDRESS = re.compile(r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b")
OPENAI_SECRET_KEY = re.compile(r"\bsk-[A-Za-z0-9_-]{6,}\b")
SECRET_TERMS = re.compile(
    r"\b("
    r"verification|verify code|captcha|password|passcode|cookie|authorization|"
    r"webhook|signing secret|secret"
    r")\b",
    re.IGNORECASE,
)
MAX_RESPONSE_BYTES = 16 * 1024


class DingTalkDeliveryError(RuntimeError):
    pass


@dataclass(frozen=True)
class DingTalkReport:
    final_status: str
    replay_status: str
    exploration_status: str
    exploration_actions: int
    finding_count: int
    cleanup_status: str
    github_run_url: str
    gcs_uri: str
    finding_summaries: tuple[dict, ...] = ()

    def __post_init__(self):
        _require_member("final_status", self.final_status, FINAL_STATUSES)
        _require_member("replay_status", self.replay_status, REPLAY_STATUSES)
        _require_member("exploration_status", self.exploration_status, EXPLORATION_STATUSES)
        _require_count("exploration_actions", self.exploration_actions)
        _require_count("finding_count", self.finding_count)
        _require_member("cleanup_status", self.cleanup_status, CLEANUP_STATUSES)
        if not isinstance(self.github_run_url, str) or GITHUB_RUN_URL.fullmatch(self.github_run_url) is None:
            raise ValueError("github_run_url is invalid")
        if not isinstance(self.gcs_uri, str) or GCS_MANIFEST_URI.fullmatch(self.gcs_uri) is None:
            raise ValueError("gcs_uri is invalid")
        _validate_finding_summaries(self.finding_summaries)

    def markdown(self):
        lines = [
            f"### Staging Browser QA {self._terminal()}",
            f"- Final status: `{self.final_status}`",
            f"- Replay status: `{self.replay_status}`",
            f"- Exploration status: `{self.exploration_status}`",
            f"- Exploration actions: `{self.exploration_actions}`",
            f"- Finding count: `{self.finding_count}`",
            f"- Cleanup status: `{self.cleanup_status}`",
            f"- Run: [Open GitHub Actions]({self.github_run_url})",
            f"- Evidence: `{self.gcs_uri}`",
        ]
        if self.final_status == "findings_detected":
            lines.append("")
            lines.append("### Findings")
            for item in self.finding_summaries:
                lines.append(f"- [{item['severity']}] {item['title']} ({item['confidence']}) {item['page_path']}")
        return "\n".join(lines)

    def payload(self):
        return {
            "msgtype": "markdown",
            "markdown": {
                "title": f"Staging Browser QA {self._terminal()}",
                "text": self.markdown(),
            },
        }

    def _terminal(self):
        if self.final_status == "passed":
            return "PASSED"
        if self.final_status == "findings_detected":
            return "ALERT"
        return "FAILED"


def send_report(webhook, report, *, opener=None, sleeper=time.sleep, signing_secret=None):
    _validate_webhook(webhook)
    if not isinstance(report, DingTalkReport):
        raise TypeError("report must be DingTalkReport")
    if signing_secret is not None:
        _validate_signing_secret(signing_secret)
    if opener is None:
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    body = json.dumps(report.payload(), ensure_ascii=False, separators=(",", ":")).encode("utf-8")

    for attempt in range(3):
        request_url = _signed_webhook(webhook, signing_secret) if signing_secret is not None else webhook
        request = urllib.request.Request(
            request_url,
            data=body,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with opener.open(request, timeout=10) as response:
                status = getattr(response, "status", 200)
                if status == 429 or 500 <= status <= 599:
                    raise _TransientDeliveryError
                if status < 200 or status >= 300:
                    raise DingTalkDeliveryError("DingTalk notification was rejected")
                raw = response.read(MAX_RESPONSE_BYTES + 1)
        except urllib.error.HTTPError as exc:
            if exc.code == 429 or 500 <= exc.code <= 599:
                if attempt < 2:
                    sleeper(2**attempt)
                    continue
                raise DingTalkDeliveryError("DingTalk notification delivery failed after retries") from None
            raise DingTalkDeliveryError("DingTalk notification was rejected") from None
        except (urllib.error.URLError, TimeoutError, _TransientDeliveryError):
            if attempt < 2:
                sleeper(2**attempt)
                continue
            raise DingTalkDeliveryError("DingTalk notification delivery failed after retries") from None

        if len(raw) > MAX_RESPONSE_BYTES:
            raise DingTalkDeliveryError("DingTalk returned an invalid response")
        try:
            payload = json.loads(raw.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError):
            raise DingTalkDeliveryError("DingTalk returned an invalid response") from None
        if not isinstance(payload, dict) or type(payload.get("errcode")) is not int:
            raise DingTalkDeliveryError("DingTalk returned an invalid response")
        if payload["errcode"] != 0:
            raise DingTalkDeliveryError("DingTalk notification was rejected")
        return

    raise DingTalkDeliveryError("DingTalk notification delivery failed after retries")


def main():
    report = DingTalkReport(
        final_status=_required_env("BROWSER_QA_FINAL_STATUS"),
        replay_status=_required_env("BROWSER_QA_REPLAY_STATUS"),
        exploration_status=_required_env("BROWSER_QA_EXPLORATION_STATUS"),
        exploration_actions=_env_count("BROWSER_QA_EXPLORATION_ACTIONS"),
        finding_count=_env_count("BROWSER_QA_FINDING_COUNT"),
        cleanup_status=_required_env("BROWSER_QA_CLEANUP_STATUS"),
        github_run_url=_required_env("BROWSER_QA_GITHUB_RUN_URL"),
        gcs_uri=_required_env("BROWSER_QA_GCS_URI"),
        finding_summaries=_env_finding_summaries("BROWSER_QA_FINDING_SUMMARIES_B64"),
    )
    send_report(_required_env("DINGTALK_WEBHOOK"), report, signing_secret=_required_env("DINGTALK_SIGNING_SECRET"))
    print("DINGTALK_NOTIFICATION_SENT")
    return 0


class _TransientDeliveryError(Exception):
    pass


def _require_member(name, value, allowed):
    if not isinstance(value, str) or value not in allowed:
        raise ValueError(f"{name} is invalid")


def _require_count(name, value):
    if type(value) is not int or value < 0:
        raise ValueError(f"{name} is invalid")


def _required_env(name):
    value = os.environ.get(name)
    if not isinstance(value, str) or not value:
        raise ValueError(f"{name} is required")
    return value


def _env_count(name):
    value = _required_env(name)
    if not value.isascii() or not value.isdecimal():
        raise ValueError(f"{name} is invalid")
    return int(value)


def _env_finding_summaries(name):
    value = os.environ.get(name)
    if value is None:
        return ()
    if not isinstance(value, str) or not value or len(value) % 4 != 0 or URLSAFE_B64.fullmatch(value) is None:
        raise ValueError(f"{name} is invalid")
    try:
        raw = base64.b64decode(value.encode("ascii"), altchars=b"-_", validate=True)
        payload = json.loads(raw.decode("utf-8"))
    except (ValueError, UnicodeDecodeError, json.JSONDecodeError):
        raise ValueError(f"{name} is invalid") from None
    if not isinstance(payload, list):
        raise ValueError(f"{name} is invalid")
    summaries = tuple(payload)
    try:
        _validate_finding_summaries(summaries)
    except ValueError:
        raise ValueError(f"{name} is invalid") from None
    return summaries


def _validate_finding_summaries(summaries):
    if not isinstance(summaries, tuple) or len(summaries) > 3:
        raise ValueError("finding_summaries is invalid")
    for item in summaries:
        if not isinstance(item, dict) or set(item) != {"severity", "title", "confidence", "page_path"}:
            raise ValueError("finding_summaries is invalid")
        _require_member("finding severity", item["severity"], FINDING_SEVERITIES)
        _require_member("finding confidence", item["confidence"], FINDING_CONFIDENCE)
        _validate_finding_title(item["title"])
        _validate_finding_page_path(item["page_path"])


def _validate_finding_title(title):
    if not isinstance(title, str) or not title:
        raise ValueError("finding title is invalid")
    folded = " ".join(title.split())
    if title != folded or len(title) > 160 or _has_control_character(title):
        raise ValueError("finding title is invalid")
    if OPENAI_SECRET_KEY.search(title) or EMAIL_ADDRESS.search(title) or SECRET_TERMS.search(title):
        raise ValueError("finding title is invalid")


def _validate_finding_page_path(page_path):
    if (
        not isinstance(page_path, str)
        or not page_path.startswith("/")
        or "?" in page_path
        or "#" in page_path
        or _has_control_character(page_path)
    ):
        raise ValueError("finding page_path is invalid")


def _has_control_character(value):
    return any(ord(char) < 32 or ord(char) == 127 for char in value)


def _validate_webhook(webhook):
    if not isinstance(webhook, str):
        raise ValueError("DingTalk webhook is invalid")
    parsed = urllib.parse.urlsplit(webhook)
    query = urllib.parse.parse_qs(parsed.query, keep_blank_values=True)
    if (
        parsed.scheme != "https"
        or parsed.hostname != "oapi.dingtalk.com"
        or parsed.port is not None
        or parsed.username is not None
        or parsed.password is not None
        or parsed.path != "/robot/send"
        or parsed.fragment
        or set(query) != {"access_token"}
        or len(query["access_token"]) != 1
        or not query["access_token"][0]
    ):
        raise ValueError("DingTalk webhook is invalid")


def _validate_signing_secret(signing_secret):
    if not isinstance(signing_secret, str) or not signing_secret:
        raise ValueError("DingTalk signing secret is invalid")


def _signed_webhook(webhook, signing_secret):
    timestamp = str(int(time.time() * 1000))
    string_to_sign = f"{timestamp}\n{signing_secret}".encode("utf-8")
    signature = base64.b64encode(
        hmac.new(signing_secret.encode("utf-8"), string_to_sign, hashlib.sha256).digest()
    ).decode("utf-8")
    parsed = urllib.parse.urlsplit(webhook)
    query = urllib.parse.parse_qsl(parsed.query, keep_blank_values=True)
    query.extend([("timestamp", timestamp), ("sign", signature)])
    return urllib.parse.urlunsplit(parsed._replace(query=urllib.parse.urlencode(query)))


if __name__ == "__main__":
    raise SystemExit(main())
