import copy
import hashlib
import json
import os
import re
import time
import urllib.parse

from .cleanup import CleanupResult
from .redaction import Redactor


class ResultValidationError(ValueError):
    pass


SEVERITIES = {"critical", "high", "medium", "low", "info"}
CONFIDENCE = {"low", "medium", "high"}
MANIFEST_SCHEMA_VERSION = 1
_SAFE_GCS_COMPONENT = re.compile(r"^[A-Za-z0-9._-]{1,128}$")
_SHA256_HEX = re.compile(r"^[0-9a-f]{64}$")
_PROVENANCE_FIELDS = {
    "skill_name",
    "skill_content_sha256",
    "codex_version",
    "model_config",
    "playwright_mcp_version",
    "playwright_package_version",
    "chromium_version",
}
_MODEL_CONFIG = {"model": "gpt-5.4", "sandbox": "workspace-write", "network_access": False}
_BROWSER_EVIDENCE_PATHS = {"browser/console.jsonl", "browser/network.jsonl"}
MAX_EVIDENCE_FILE_BYTES = 16 * 1024 * 1024
_EVIDENCE_READ_CHUNK_BYTES = 64 * 1024


def validate_result(payload):
    _require_object(payload, {"replay", "exploration", "budgets", "findings"}, set())
    _require_object(payload["replay"], {"status", "checkpoint_reached"}, set())
    _enum(payload["replay"]["status"], {"passed", "failed"}, "replay.status")
    _boolean(payload["replay"]["checkpoint_reached"], "replay.checkpoint_reached")

    _require_object(payload["exploration"], {"status", "actions_used"}, set())
    _enum(payload["exploration"]["status"], {"passed", "failed", "not_started"}, "exploration.status")
    _integer(payload["exploration"]["actions_used"], "exploration.actions_used", minimum=0)

    _require_object(payload["budgets"], {"replay_seconds", "exploration_seconds", "max_actions"}, set())
    for key in ("replay_seconds", "exploration_seconds", "max_actions"):
        _integer(payload["budgets"][key], f"budgets.{key}", minimum=1)

    if not isinstance(payload["findings"], list):
        raise ResultValidationError("findings must be an array")
    for index, item in enumerate(payload["findings"]):
        _validate_finding(item, index)

    return payload


def normalize_findings(payload, *, runtime_root, proxy_events):
    validate_result(payload)
    normalized = copy.deepcopy(payload)
    validate_result(normalized)
    runtime_real = os.path.realpath(runtime_root)
    denied_hosts = _denied_proxy_hosts(proxy_events)
    findings = []
    seen = set()
    for finding in normalized["findings"]:
        finding["target_url"] = _strip_url_query_fragment(finding["target_url"])
        evidence = _finding_evidence(runtime_real, finding["evidence_paths"])
        if not evidence["usable"]:
            finding["severity"] = "info"
        if (
            finding["severity"] != "info"
            and _matches_denied_proxy_host(finding, denied_hosts)
            and not _has_independent_product_evidence(finding, evidence)
        ):
            finding["severity"] = "info"
        dedupe_key = (
            finding["target_url"],
            _normalized_title(finding["title"]),
            tuple(sorted(evidence["hashes"])),
        )
        if dedupe_key in seen:
            continue
        seen.add(dedupe_key)
        findings.append(finding)
    normalized["findings"] = findings
    validate_result(normalized)
    return normalized


def classify_status(payload, *, cleanup_result=None, codex_returncode=0, upload_failed=False, invalid_result=False, runtime_classification=None):
    if cleanup_result is not None and cleanup_result.cleanup_failed:
        return "cleanup_failed"
    if codex_returncode != 0 or upload_failed or invalid_result or runtime_classification:
        return "infrastructure_failed"
    if payload["replay"]["status"] == "failed" or not payload["replay"]["checkpoint_reached"]:
        return "replay_failed"
    if any(item["severity"] != "info" for item in payload["findings"]):
        return "findings_detected"
    return "passed"


def build_manifest(
    payload,
    *,
    cleanup_result,
    run_id,
    execution_id,
    provenance,
    redactor=None,
    codex_returncode=0,
    upload_failed=False,
    invalid_result=False,
    runtime_classification=None,
):
    redactor = redactor or Redactor()
    validate_result(payload)
    validate_provenance(provenance)
    cleanup = _cleanup_to_dict(cleanup_result)
    status = classify_status(
        payload,
        cleanup_result=cleanup_result,
        codex_returncode=codex_returncode,
        upload_failed=upload_failed,
        invalid_result=invalid_result,
        runtime_classification=runtime_classification,
    )
    manifest = {
        "schema_version": MANIFEST_SCHEMA_VERSION,
        "kind": "main",
        "run_id": run_id,
        "execution_id": execution_id,
        "status": status,
        "created_at": int(time.time()),
        "result": redactor.clean(payload),
        "cleanup": redactor.clean(cleanup),
        "provenance": redactor.clean(provenance),
    }
    _validate_manifest_identity(run_id, execution_id)
    if runtime_classification:
        manifest["infrastructure"] = {"status": "failed", "classification": runtime_classification}
    return manifest


def write_report(
    result_path,
    manifest_path,
    *,
    cleanup_result,
    run_id,
    execution_id,
    provenance,
    redactor=None,
    codex_returncode=0,
    upload_failed=False,
    invalid_result=False,
    runtime_classification=None,
):
    with open(result_path, encoding="utf-8") as handle:
        payload = json.load(handle)
    manifest = build_manifest(
        payload,
        cleanup_result=cleanup_result,
        run_id=run_id,
        execution_id=execution_id,
        provenance=provenance,
        redactor=redactor,
        codex_returncode=codex_returncode,
        upload_failed=upload_failed,
        invalid_result=invalid_result,
        runtime_classification=runtime_classification,
    )
    os.makedirs(os.path.dirname(manifest_path), exist_ok=True)
    _write_json_private(manifest_path, manifest)
    return manifest


def validate_provenance(provenance):
    _require_object(provenance, _PROVENANCE_FIELDS, set(), path="provenance")
    if provenance["skill_name"] != "flatkey-new-user-onboarding":
        raise ResultValidationError("provenance.skill_name has invalid value")
    if not isinstance(provenance["skill_content_sha256"], str) or not _SHA256_HEX.fullmatch(provenance["skill_content_sha256"]):
        raise ResultValidationError("provenance.skill_content_sha256 must be lowercase sha256 hex")
    _version_string(provenance["codex_version"], "provenance.codex_version")
    _version_string(provenance["playwright_mcp_version"], "provenance.playwright_mcp_version")
    _version_string(provenance["playwright_package_version"], "provenance.playwright_package_version")
    _version_string(provenance["chromium_version"], "provenance.chromium_version")
    model_config = provenance["model_config"]
    if not isinstance(model_config, dict) or model_config != _MODEL_CONFIG:
        raise ResultValidationError("provenance.model_config has invalid value")
    return provenance


def _cleanup_to_dict(cleanup_result):
    if not isinstance(cleanup_result, CleanupResult):
        raise TypeError("cleanup_result must be CleanupResult")
    return {
        "deleted_token_count": cleanup_result.deleted_token_count,
        "account_deleted": cleanup_result.account_deleted,
        "login_rejected_after_delete": cleanup_result.login_rejected_after_delete,
        "cleanup_failed": cleanup_result.cleanup_failed,
        "reason": cleanup_result.reason,
    }


def _strip_url_query_fragment(url):
    parsed = urllib.parse.urlsplit(url)
    return urllib.parse.urlunsplit((parsed.scheme, parsed.netloc, parsed.path, "", ""))


def _normalized_title(title):
    return " ".join(title.casefold().split())


def _denied_proxy_hosts(proxy_events):
    hosts = set()
    if not isinstance(proxy_events, list):
        return hosts
    for event in proxy_events:
        if not isinstance(event, dict) or event.get("reason") != "denied":
            continue
        host = _host_without_port(event.get("host"))
        if host:
            hosts.add(host)
    return hosts


def _matches_denied_proxy_host(finding, denied_hosts):
    if not denied_hosts:
        return False
    target_host = _url_host(finding["target_url"])
    text = " ".join([finding["title"], finding["expected"], finding["actual"]]).casefold()
    for host in denied_hosts:
        if host != target_host and _host_appears_in_text(host, text):
            return True
    return False


def _finding_evidence(runtime_root, paths):
    hashes = []
    network_events = []
    console_events = []
    has_visual_evidence = False
    usable = True
    for path in paths:
        evidence_path = _resolve_evidence_path(runtime_root, path)
        if evidence_path is None:
            usable = False
            continue
        evidence = _read_evidence(
            evidence_path,
            event_stream=_event_stream_for_evidence_path(path),
        )
        if evidence is None:
            usable = False
            continue
        hashes.append(evidence["sha256"])
        network_events.extend(evidence["network_events"])
        console_events.extend(evidence["console_events"])
        has_visual_evidence = has_visual_evidence or path.startswith("screenshots/")
    return {
        "usable": usable,
        "hashes": hashes,
        "network_events": network_events,
        "console_events": console_events,
        "has_visual_evidence": has_visual_evidence,
    }


def _event_stream_for_evidence_path(path):
    if path == "browser/network.jsonl":
        return "network"
    if path == "browser/console.jsonl":
        return "console"
    return None


def _read_evidence(path, *, event_stream):
    digest = hashlib.sha256()
    total = 0
    pending = b""
    network_events = []
    console_events = []
    with open(path, "rb") as handle:
        while True:
            read_size = (
                min(_EVIDENCE_READ_CHUNK_BYTES, MAX_EVIDENCE_FILE_BYTES - total)
                if total < MAX_EVIDENCE_FILE_BYTES
                else 1
            )
            chunk = handle.read(read_size)
            if not chunk:
                break
            total += len(chunk)
            if total > MAX_EVIDENCE_FILE_BYTES:
                return None
            digest.update(chunk)
            if event_stream:
                pending = _parse_jsonl_chunk(pending + chunk, _events_for_stream(event_stream, network_events, console_events))
    if event_stream and pending.strip():
        _append_jsonl_event(pending, _events_for_stream(event_stream, network_events, console_events))
    return {
        "sha256": digest.hexdigest(),
        "network_events": network_events,
        "console_events": console_events,
    }


def _events_for_stream(event_stream, network_events, console_events):
    return network_events if event_stream == "network" else console_events


def _parse_jsonl_chunk(data, events):
    lines = data.splitlines(keepends=True)
    if lines and not lines[-1].endswith((b"\n", b"\r")):
        pending = lines.pop()
    else:
        pending = b""
    for line in lines:
        _append_jsonl_event(line, events)
    return pending


def _resolve_evidence_path(runtime_root, logical_path):
    if (
        not isinstance(logical_path, str)
        or not logical_path
        or os.path.isabs(logical_path)
        or "\\" in logical_path
        or logical_path.startswith("/")
        or logical_path.startswith("//")
    ):
        return None
    parts = logical_path.split("/")
    if any(part in ("", ".", "..") for part in parts):
        return None
    if not _allowed_evidence_logical_path(logical_path):
        return None
    current = runtime_root
    for part in parts:
        current = os.path.join(current, part)
        try:
            os.lstat(current)
        except OSError:
            return None
        if os.path.islink(current):
            return None
    real = os.path.realpath(os.path.join(runtime_root, *parts))
    if real != runtime_root and not real.startswith(runtime_root + os.sep):
        return None
    if not os.path.isfile(real):
        return None
    return real


def _allowed_evidence_logical_path(logical_path):
    if logical_path in _BROWSER_EVIDENCE_PATHS:
        return True
    if logical_path.startswith("screenshots/") and logical_path.count("/") == 1:
        return logical_path.endswith(".png")
    return False


def _append_jsonl_event(line, events):
    text = line.decode("utf-8", "replace").strip()
    if not text:
        return
    try:
        event = json.loads(text)
    except json.JSONDecodeError:
        return
    if isinstance(event, dict):
        events.append(event)


def _has_independent_product_evidence(finding, evidence):
    return (
        evidence["has_visual_evidence"]
        or _has_same_origin_console_error_evidence(finding, evidence)
        or _has_same_origin_5xx_network_evidence(finding, evidence)
    )


def _has_same_origin_console_error_evidence(finding, evidence):
    target_host = _url_host(finding["target_url"])
    if not target_host:
        return False
    for event in evidence["console_events"]:
        event_type = event.get("type")
        if event_type not in {"error", "assert"}:
            continue
        location = event.get("location")
        if isinstance(location, dict) and _url_host(location.get("url")) == target_host:
            return True
    return False


def _has_same_origin_5xx_network_evidence(finding, evidence):
    target_host = _url_host(finding["target_url"])
    if not target_host:
        return False
    for event in evidence["network_events"]:
        status = event.get("status")
        if isinstance(status, int) and 500 <= status <= 599 and _url_host(event.get("url")) == target_host:
            return True
    return False


def _url_host(url):
    if not isinstance(url, str):
        return ""
    return (urllib.parse.urlsplit(url).hostname or "").casefold()


def _host_without_port(value):
    if not isinstance(value, str) or not value:
        return ""
    host = value.strip().casefold()
    if host.startswith("["):
        end = host.find("]")
        return host[1:end] if end >= 0 else ""
    if ":" in host:
        host = host.rsplit(":", 1)[0]
    return host.rstrip(".")


def _host_appears_in_text(host, text):
    pattern = rf"(?<![A-Za-z0-9.-]){re.escape(host)}(?![A-Za-z0-9.-])"
    return re.search(pattern, text) is not None


def _validate_finding(item, index):
    required = {"severity", "title", "target_url", "steps", "expected", "actual", "evidence_paths", "confidence"}
    _require_object(item, required, set(), path=f"findings[{index}]")
    _enum(item["severity"], SEVERITIES, f"findings[{index}].severity")
    _string(item["title"], f"findings[{index}].title")
    _string(item["target_url"], f"findings[{index}].target_url")
    _string_list(item["steps"], f"findings[{index}].steps")
    _string(item["expected"], f"findings[{index}].expected")
    _string(item["actual"], f"findings[{index}].actual")
    _string_list(item["evidence_paths"], f"findings[{index}].evidence_paths")
    _enum(item["confidence"], CONFIDENCE, f"findings[{index}].confidence")


def _require_object(value, required, optional, path="result"):
    if not isinstance(value, dict):
        raise ResultValidationError(f"{path} must be an object")
    keys = set(value)
    missing = required - keys
    extra = keys - required - optional
    if missing:
        raise ResultValidationError(f"{path} missing required fields: {', '.join(sorted(missing))}")
    if extra:
        raise ResultValidationError(f"{path} contains extra fields: {', '.join(sorted(extra))}")


def _enum(value, allowed, path):
    if not isinstance(value, str) or value not in allowed:
        raise ResultValidationError(f"{path} has invalid value")


def _string(value, path):
    if not isinstance(value, str) or not value:
        raise ResultValidationError(f"{path} must be a non-empty string")


def _string_list(value, path):
    if not isinstance(value, list) or not value:
        raise ResultValidationError(f"{path} must be a non-empty string array")
    for item in value:
        _string(item, path)


def _integer(value, path, *, minimum):
    if not isinstance(value, int) or isinstance(value, bool) or value < minimum:
        raise ResultValidationError(f"{path} must be an integer >= {minimum}")


def _boolean(value, path):
    if not isinstance(value, bool):
        raise ResultValidationError(f"{path} must be boolean")


def _version_string(value, path):
    if not isinstance(value, str) or not value or "\n" in value or "\r" in value:
        raise ResultValidationError(f"{path} must be a non-empty single-line string")
    if len(value.encode("utf-8", "replace")) > 256:
        raise ResultValidationError(f"{path} must be at most 256 bytes")


def _write_json_private(path, payload):
    flags = os.O_WRONLY | os.O_CREAT | os.O_TRUNC
    fd = os.open(path, flags, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        json.dump(payload, handle, sort_keys=True)


def _validate_manifest_identity(run_id, execution_id):
    if not isinstance(run_id, str) or not run_id.isascii() or not run_id.isdecimal():
        raise ResultValidationError("FLATKEY_QA_RUN_ID must contain only ASCII decimal digits")
    if (
        not isinstance(execution_id, str)
        or not _SAFE_GCS_COMPONENT.fullmatch(execution_id)
        or execution_id in {".", ".."}
        or ".." in execution_id
        or "/" in execution_id
        or "\\" in execution_id
    ):
        raise ResultValidationError("FLATKEY_BROWSER_QA_EXECUTION_ID must be a safe GCS object path component")
