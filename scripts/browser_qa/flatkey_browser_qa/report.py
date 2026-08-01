import json
import os
import re
import time

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
