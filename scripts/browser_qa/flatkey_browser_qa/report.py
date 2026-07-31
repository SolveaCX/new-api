import json
import os
import time

from .cleanup import CleanupResult
from .redaction import Redactor


class ResultValidationError(ValueError):
    pass


SEVERITIES = {"critical", "high", "medium", "low", "info"}
CONFIDENCE = {"low", "medium", "high"}


def validate_result(payload):
    _require_object(payload, {"replay", "exploration", "budgets", "findings"}, {"infrastructure"})
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

    if "infrastructure" in payload:
        _require_object(payload["infrastructure"], {"status"}, {"classification"})
        _enum(payload["infrastructure"]["status"], {"failed"}, "infrastructure.status")
        if "classification" in payload["infrastructure"]:
            _enum(
                payload["infrastructure"]["classification"],
                {"alias_restriction", "preflight_failed", "codex_timeout", "upload_failed", "invalid_result"},
                "infrastructure.classification",
            )
    return payload


def classify_status(payload, *, cleanup_result=None, codex_returncode=0, upload_failed=False, invalid_result=False):
    if cleanup_result is not None and cleanup_result.cleanup_failed:
        return "cleanup_failed"
    if upload_failed or invalid_result or payload.get("infrastructure", {}).get("status") == "failed":
        return "infrastructure_failed"
    if codex_returncode != 0 or payload["replay"]["status"] == "failed" or not payload["replay"]["checkpoint_reached"]:
        return "replay_failed"
    if any(item["severity"] != "info" for item in payload["findings"]):
        return "findings_detected"
    return "passed"


def build_manifest(payload, *, cleanup_result, redactor=None, model_manifest=None, codex_returncode=0, upload_failed=False, invalid_result=False):
    redactor = redactor or Redactor()
    validate_result(payload)
    cleanup = _cleanup_to_dict(cleanup_result)
    status = classify_status(payload, cleanup_result=cleanup_result, codex_returncode=codex_returncode, upload_failed=upload_failed, invalid_result=invalid_result)
    manifest = {
        "status": status,
        "created_at": int(time.time()),
        "result": redactor.clean(payload),
        "cleanup": redactor.clean(cleanup),
    }
    return manifest


def write_report(result_path, manifest_path, *, cleanup_result, redactor=None, codex_returncode=0, upload_failed=False, invalid_result=False):
    with open(result_path, encoding="utf-8") as handle:
        payload = json.load(handle)
    manifest = build_manifest(
        payload,
        cleanup_result=cleanup_result,
        redactor=redactor,
        codex_returncode=codex_returncode,
        upload_failed=upload_failed,
        invalid_result=invalid_result,
    )
    os.makedirs(os.path.dirname(manifest_path), exist_ok=True)
    _write_json_private(manifest_path, manifest)
    return manifest


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


def _write_json_private(path, payload):
    flags = os.O_WRONLY | os.O_CREAT | os.O_TRUNC
    fd = os.open(path, flags, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        json.dump(payload, handle, sort_keys=True)
