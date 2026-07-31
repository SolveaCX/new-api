import json
import re
import time
import urllib.error
import urllib.parse
import urllib.request


METADATA_ORIGIN = "http://metadata.google.internal"
STORAGE_ORIGIN = "https://storage.googleapis.com"
_MAX_RESPONSE_BYTES = 256 * 1024
_BUCKET_RE = re.compile(r"^[a-z0-9][a-z0-9._-]{1,220}[a-z0-9]$")


class GcpError(Exception):
    pass


class GcpConfigError(GcpError):
    pass


class GcpTransientError(GcpError):
    pass


class _NoRedirectHandler(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def _default_opener():
    return urllib.request.build_opener(_NoRedirectHandler(), urllib.request.ProxyHandler({}))


class GcpClient:
    def __init__(self, *, opener=None, retry_base_delay=0.2):
        self.opener = opener or _default_opener()
        self.retry_base_delay = retry_base_delay

    def __repr__(self):
        return "GcpClient()"

    def identity_token(self, audience):
        _validate_https_url(audience, "audience")
        path = "/computeMetadata/v1/instance/service-accounts/default/identity"
        query = urllib.parse.urlencode({"audience": audience, "format": "full"})
        return self._metadata_text(path + "?" + query)

    def access_token(self):
        payload = self._metadata_json("/computeMetadata/v1/instance/service-accounts/default/token")
        token = payload.get("access_token")
        if not isinstance(token, str) or not token:
            raise GcpConfigError("metadata token response malformed")
        return token

    def _metadata_json(self, selector):
        raw = self._metadata_text(selector)
        try:
            payload = json.loads(raw)
        except json.JSONDecodeError as exc:
            raise GcpConfigError("metadata json response malformed") from exc
        if not isinstance(payload, dict):
            raise GcpConfigError("metadata json response malformed")
        return payload

    def _metadata_text(self, selector):
        if not selector.startswith("/computeMetadata/v1/instance/service-accounts/default/"):
            raise GcpConfigError("metadata path outside service-account contract")
        url = METADATA_ORIGIN + selector
        return _request_bytes(
            self.opener,
            "GET",
            url,
            headers={"Metadata-Flavor": "Google"},
            timeout=5,
            retry_base_delay=self.retry_base_delay,
        ).decode("utf-8")


def upload_gcs_object(bucket, object_name, data, content_type, access_token, *, opener=None, retry_base_delay=0.2):
    _validate_bucket(bucket)
    _validate_object_name(object_name)
    if not isinstance(data, bytes):
        raise TypeError("data must be bytes")
    if not isinstance(content_type, str) or not content_type:
        raise GcpConfigError("content type is required")
    if not isinstance(access_token, str) or not access_token:
        raise GcpConfigError("access token is required")
    selected_opener = opener or _default_opener()
    path = f"/upload/storage/v1/b/{urllib.parse.quote(bucket, safe='')}/o"
    query = urllib.parse.urlencode(
        {
            "uploadType": "media",
            "name": object_name,
            "ifGenerationMatch": "0",
        }
    )
    url = STORAGE_ORIGIN + path + "?" + query
    payload = _request_bytes(
        selected_opener,
        "POST",
        url,
        data=data,
        headers={"Authorization": f"Bearer {access_token}", "Content-Type": content_type},
        timeout=15,
        retry_base_delay=retry_base_delay,
    )
    try:
        decoded = json.loads(payload.decode("utf-8")) if payload else {}
    except json.JSONDecodeError as exc:
        raise GcpConfigError("gcs upload response malformed") from exc
    if not isinstance(decoded, dict):
        raise GcpConfigError("gcs upload response malformed")
    return decoded


def _request_bytes(opener, method, url, *, data=None, headers=None, timeout=10, retry_base_delay=0.2, max_attempts=3):
    parsed = urllib.parse.urlparse(url)
    if parsed.netloc == "metadata.google.internal":
        if parsed.scheme != "http":
            raise GcpConfigError("metadata origin must be exact")
    elif parsed.netloc == "storage.googleapis.com":
        if parsed.scheme != "https":
            raise GcpConfigError("storage origin must be exact")
    else:
        raise GcpConfigError("google request origin must be exact")
    for attempt in range(1, max_attempts + 1):
        request = urllib.request.Request(url, data=data, headers=headers or {}, method=method)
        try:
            with opener.open(request, timeout=timeout) as response:
                status = response.status
                raw = response.read(_MAX_RESPONSE_BYTES + 1)
        except urllib.error.HTTPError as exc:
            if 300 <= exc.code <= 399:
                raise GcpConfigError("google redirect blocked") from exc
            if exc.code in (429,) or 500 <= exc.code <= 599:
                if attempt < max_attempts:
                    _sleep(retry_base_delay, attempt)
                    continue
                raise GcpTransientError("google transient failure") from exc
            raise GcpConfigError("google request failed") from exc
        except (urllib.error.URLError, TimeoutError, ConnectionError, OSError) as exc:
            if attempt < max_attempts:
                _sleep(retry_base_delay, attempt)
                continue
            raise GcpTransientError("google connection failed") from exc
        if 300 <= status <= 399:
            raise GcpConfigError("google redirect blocked")
        if status in (429,) or 500 <= status <= 599:
            if attempt < max_attempts:
                _sleep(retry_base_delay, attempt)
                continue
            raise GcpTransientError("google transient failure")
        if not 200 <= status <= 299:
            raise GcpConfigError("google request failed")
        if len(raw) > _MAX_RESPONSE_BYTES:
            raise GcpConfigError("google response too large")
        return raw
    raise GcpTransientError("google retry attempts exhausted")


def _validate_https_url(value, label):
    parsed = urllib.parse.urlparse(value)
    if parsed.scheme != "https" or not parsed.netloc or parsed.username or parsed.password or parsed.fragment:
        raise GcpConfigError(f"{label} must be an exact https url")


def _validate_bucket(bucket):
    if not isinstance(bucket, str) or not _BUCKET_RE.fullmatch(bucket) or ".." in bucket:
        raise GcpConfigError("invalid gcs bucket")


def _validate_object_name(name):
    if not isinstance(name, str) or not name or name.startswith("/") or "\\" in name:
        raise GcpConfigError("invalid gcs object")
    parts = name.split("/")
    if any(part in {"", ".", ".."} for part in parts):
        raise GcpConfigError("invalid gcs object")


def _sleep(base, attempt):
    if base <= 0:
        return
    time.sleep(min(base * (2 ** (attempt - 1)), 2.0))
