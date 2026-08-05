import json
import re
import sys
import time
import urllib.parse

from .api import StagingApiClient
from .cleanup import CleanupRunner
from .config import load_cleanup_config
from .gcp import GcpClient, GcsObjectAlreadyExists, GcsUploadUncertain, read_gcs_json_object, upload_gcs_object
from .identity import derive_identity
from . import report


ROOT_MANIFEST_SCHEMA_VERSION = 1
_SAFE_GCS_COMPONENT = re.compile(r"^[A-Za-z0-9._-]{1,128}$")
_STATUS_PRIORITY = {
    "passed": 0,
    "findings_detected": 1,
    "replay_failed": 2,
    "infrastructure_failed": 3,
    "cleanup_failed": 4,
}


def main(argv=None):
    argv = [] if argv is None else argv
    if argv:
        raise SystemExit("cleanup_job does not accept command line arguments")
    cfg = load_cleanup_config()
    identity = derive_identity(cfg.identity_seed, cfg.run_id)
    client = StagingApiClient(cfg.console_origin)
    result = CleanupRunner(client).run(identity)
    cleanup_payload = _build_cleanup_payload(cfg, result, int(time.time()))
    persistence_failed = False
    try:
        cleanup_payload = _persist_cleanup(cfg, cleanup_payload)
    except Exception as exc:
        persistence_failed = True
        cleanup_payload = dict(cleanup_payload)
        cleanup_payload["persistence_error"] = type(exc).__name__
    print(json.dumps(cleanup_payload["cleaned"], sort_keys=True))
    return 1 if cleanup_payload["cleaned"]["cleanup_failed"] or persistence_failed else 0


def _persist_cleanup(cfg, cleanup_payload):
    access_token = GcpClient().access_token()
    cleanup_object = f"runs/{cfg.run_id}/cleanup/{cfg.cleanup_execution_id}/cleanup.json"
    root_object = f"runs/{cfg.run_id}/manifest.json"
    cleanup_payload = _write_or_read_cleanup_payload(cfg, cleanup_object, cleanup_payload, access_token)
    cleanup_record = _execution_record(
        "cleanup",
        cfg.cleanup_execution_id,
        cleanup_object,
        cleanup_payload["status"],
        cleanup_payload["created_at"],
        summary=_cleanup_summary(cleanup_payload),
    )
    for _attempt in range(3):
        root, generation = _read_root_or_bootstrap(cfg, access_token)
        merged = _merge_root_manifest(root, cfg, cleanup_record, access_token)
        try:
            upload_gcs_object(
                cfg.gcs_bucket,
                root_object,
                _json_bytes(merged),
                "application/json",
                access_token,
                if_generation_match=generation,
            )
            return cleanup_payload
        except GcsUploadUncertain:
            if _root_matches_desired_state(cfg, root_object, merged, access_token):
                return cleanup_payload
        except GcsObjectAlreadyExists:
            pass
    raise RuntimeError("root manifest CAS failed")


def _write_or_read_cleanup_payload(cfg, cleanup_object, cleanup_payload, access_token):
    try:
        upload_gcs_object(
            cfg.gcs_bucket,
            cleanup_object,
            _json_bytes(cleanup_payload),
            "application/json",
            access_token,
            if_generation_match=0,
        )
        return cleanup_payload
    except (GcsObjectAlreadyExists, GcsUploadUncertain):
        persisted, _generation = read_gcs_json_object(cfg.gcs_bucket, cleanup_object, access_token)
        _validate_cleanup_manifest(persisted, cfg)
        return persisted


def _root_matches_desired_state(cfg, root_object, desired, access_token):
    current, _generation = read_gcs_json_object(cfg.gcs_bucket, root_object, access_token)
    _validate_root_manifest(current, cfg.run_id)
    return current == desired


def _read_root_or_bootstrap(cfg, access_token):
    root_object = f"runs/{cfg.run_id}/manifest.json"
    try:
        root, generation = read_gcs_json_object(cfg.gcs_bucket, root_object, access_token)
    except FileNotFoundError:
        root = {
            "schema_version": ROOT_MANIFEST_SCHEMA_VERSION,
            "run_id": cfg.run_id,
            "status": "passed",
            "latest": {"main_execution_id": cfg.main_execution_id, "cleanup_execution_id": None},
            "executions": [],
        }
        generation = 0
    _validate_root_manifest(root, cfg.run_id)
    return root, generation


def _merge_root_manifest(root, cfg, cleanup_record, access_token):
    records = list(root["executions"])
    existing_main = _find_execution_record(records, "main", cfg.main_execution_id)
    try:
        main_record = _main_record(cfg, access_token)
    except FileNotFoundError:
        if existing_main is not None and not _is_missing_main_placeholder(existing_main):
            main_record = existing_main
        else:
            main_record = _missing_main_record(cfg)
    records = _append_or_validate_record(records, main_record)
    records = _append_or_validate_record(records, cleanup_record)
    status = _aggregate_status(records)
    latest = _latest_by_kind(records)
    return {
        "schema_version": ROOT_MANIFEST_SCHEMA_VERSION,
        "run_id": cfg.run_id,
        "status": status,
        "latest": latest,
        "executions": records,
    }


def _main_record(cfg, access_token):
    object_name = f"runs/{cfg.run_id}/main/{cfg.main_execution_id}/manifest.json"
    manifest, _generation = read_gcs_json_object(cfg.gcs_bucket, object_name, access_token)
    _validate_main_manifest(manifest, cfg.run_id, cfg.main_execution_id)
    summary = _main_summary(manifest["result"])
    status = manifest.get("status")
    created_at = manifest.get("created_at", 0)
    return _execution_record("main", cfg.main_execution_id, object_name, status, created_at, summary=summary)


def _missing_main_record(cfg):
    return _execution_record(
        "main",
        cfg.main_execution_id,
        f"runs/{cfg.run_id}/main/{cfg.main_execution_id}/manifest.json",
        "infrastructure_failed",
        0,
        summary={
            "replay_status": "failed",
            "exploration_status": "not_started",
            "exploration_actions": 0,
            "finding_count": 0,
            "finding_summaries": [],
        },
    )


def _find_execution_record(records, kind, execution_id):
    for record in records:
        if record["kind"] == kind and record["execution_id"] == execution_id:
            return record
    return None


def _is_missing_main_placeholder(record):
    return (
        record.get("kind") == "main"
        and record.get("status") == "infrastructure_failed"
        and record.get("created_at") == 0
        and record.get("summary")
        == {
            "replay_status": "failed",
            "exploration_status": "not_started",
            "exploration_actions": 0,
            "finding_count": 0,
            "finding_summaries": [],
        }
    )


def _append_or_validate_record(records, record):
    updated = []
    found = False
    for existing in records:
        if existing["kind"] == record["kind"] and existing["execution_id"] == record["execution_id"]:
            found = True
            if existing != record:
                if _is_missing_main_placeholder(existing) and record["kind"] == "main":
                    updated.append(record)
                    continue
                legacy_record = dict(record)
                legacy_record.pop("summary", None)
                if existing == legacy_record:
                    updated.append(record)
                    continue
                raise ValueError("conflicting execution record")
            updated.append(existing)
        else:
            updated.append(existing)
    if found:
        return updated
    return [*records, record]


def _aggregate_status(records):
    status = "passed"
    for record in records:
        candidate = record["status"]
        if _STATUS_PRIORITY[candidate] > _STATUS_PRIORITY[status]:
            status = candidate
    return status


def _latest_by_kind(records):
    latest = {"main": None, "cleanup": None}
    for record in records:
        current = latest[record["kind"]]
        if current is None or (record["created_at"], record["execution_id"]) > (current["created_at"], current["execution_id"]):
            latest[record["kind"]] = record
    return {
        "main_execution_id": latest["main"]["execution_id"],
        "cleanup_execution_id": None if latest["cleanup"] is None else latest["cleanup"]["execution_id"],
    }


def _build_cleanup_payload(cfg, result, created_at):
    status = "cleanup_failed" if result.cleanup_failed else "passed"
    return {
        "schema_version": ROOT_MANIFEST_SCHEMA_VERSION,
        "kind": "cleanup",
        "run_id": cfg.run_id,
        "execution_id": cfg.cleanup_execution_id,
        "main_execution_id": cfg.main_execution_id,
        "created_at": created_at,
        "status": status,
        "cleaned": {
            "cleanup_failed": result.cleanup_failed,
            "deleted_token_count": result.deleted_token_count,
            "account_deleted": result.account_deleted,
            "login_rejected_after_delete": result.login_rejected_after_delete,
            "reason": result.reason,
        },
    }


def _execution_record(kind, execution_id, manifest, status, created_at, *, summary=None):
    record = {
        "kind": kind,
        "execution_id": execution_id,
        "manifest": manifest,
        "status": status,
        "created_at": created_at,
    }
    if summary is not None:
        record["summary"] = summary
    return record


def _validate_root_manifest(root, run_id):
    if not isinstance(root, dict):
        raise ValueError("root manifest must be an object")
    if root.get("schema_version") != ROOT_MANIFEST_SCHEMA_VERSION or root.get("run_id") != run_id:
        raise ValueError("root manifest identity mismatch")
    latest = root.get("latest")
    if not isinstance(latest, dict) or set(latest) != {"main_execution_id", "cleanup_execution_id"}:
        raise ValueError("root manifest latest is invalid")
    if not _safe_gcs_component(latest["main_execution_id"]):
        raise ValueError("root manifest latest main execution is invalid")
    if latest["cleanup_execution_id"] is not None and not _safe_gcs_component(latest["cleanup_execution_id"]):
        raise ValueError("root manifest latest cleanup execution is invalid")
    executions = root.get("executions")
    if not isinstance(executions, list):
        raise ValueError("root manifest executions is invalid")
    seen = set()
    seen_by_kind = {"main": set(), "cleanup": set()}
    for record in executions:
        _validate_record(record, run_id)
        identity = (record["kind"], record["execution_id"])
        if identity in seen:
            raise ValueError("root manifest contains duplicate execution record")
        seen.add(identity)
        seen_by_kind[record["kind"]].add(record["execution_id"])
    status = root.get("status")
    if not isinstance(status, str) or status not in _STATUS_PRIORITY:
        raise ValueError("root manifest status is invalid")
    if seen_by_kind["main"] and latest["main_execution_id"] not in seen_by_kind["main"]:
        raise ValueError("root manifest latest main execution is inconsistent")
    if (
        latest["cleanup_execution_id"] is not None
        and seen_by_kind["cleanup"]
        and latest["cleanup_execution_id"] not in seen_by_kind["cleanup"]
    ):
        raise ValueError("root manifest latest cleanup execution is inconsistent")


def _validate_record(record, run_id):
    if not isinstance(record, dict):
        raise ValueError("execution record must be an object")
    if not {"kind", "execution_id", "manifest", "status", "created_at"} <= set(record):
        raise ValueError("execution record fields are invalid")
    if set(record) - {"kind", "execution_id", "manifest", "status", "created_at", "summary"}:
        raise ValueError("execution record fields are invalid")
    if record["kind"] not in {"main", "cleanup"}:
        raise ValueError("execution record kind is invalid")
    if not _safe_gcs_component(record["execution_id"]):
        raise ValueError("execution record execution_id is invalid")
    if record["manifest"] != _canonical_manifest_path(run_id, record["kind"], record["execution_id"]):
        raise ValueError("execution record manifest path is invalid")
    if not isinstance(record["status"], str) or not record["status"]:
        raise ValueError("execution record status is invalid")
    if record["status"] not in _STATUS_PRIORITY:
        raise ValueError("execution record status is invalid")
    if not isinstance(record["created_at"], int) or isinstance(record["created_at"], bool) or record["created_at"] < 0:
        raise ValueError("execution record created_at is invalid")
    if "summary" in record:
        _validate_record_summary(record["kind"], record["summary"])


def _validate_main_manifest(manifest, run_id, execution_id):
    if not isinstance(manifest, dict):
        raise ValueError("main manifest must be an object")
    allowed = {
        "schema_version",
        "kind",
        "run_id",
        "execution_id",
        "status",
        "created_at",
        "result",
        "cleanup",
        "provenance",
        "infrastructure",
    }
    required = allowed - {"infrastructure"}
    if set(manifest) - allowed or required - set(manifest):
        raise ValueError("main manifest fields are invalid")
    if manifest["schema_version"] != ROOT_MANIFEST_SCHEMA_VERSION:
        raise ValueError("main manifest schema is invalid")
    if manifest["kind"] != "main" or manifest["run_id"] != run_id or manifest["execution_id"] != execution_id:
        raise ValueError("main manifest identity is invalid")
    if not _safe_gcs_component(manifest["execution_id"]):
        raise ValueError("main manifest execution_id is invalid")
    if not isinstance(manifest["status"], str) or manifest["status"] not in _STATUS_PRIORITY:
        raise ValueError("main manifest status is invalid")
    if not isinstance(manifest["created_at"], int) or isinstance(manifest["created_at"], bool) or manifest["created_at"] < 0:
        raise ValueError("main manifest created_at is invalid")
    if not isinstance(manifest["result"], dict) or not isinstance(manifest["cleanup"], dict):
        raise ValueError("main manifest payload is invalid")
    try:
        report.validate_result(manifest["result"])
        report.validate_provenance(manifest["provenance"])
    except report.ResultValidationError as exc:
        raise ValueError("main manifest payload is invalid") from exc
    if "infrastructure" in manifest and not isinstance(manifest["infrastructure"], dict):
        raise ValueError("main manifest infrastructure is invalid")


def _main_summary(result):
    try:
        validated = report.validate_result(result)
    except report.ResultValidationError as exc:
        raise ValueError("main manifest result is invalid") from exc
    return {
        "replay_status": validated["replay"]["status"],
        "exploration_status": validated["exploration"]["status"],
        "exploration_actions": validated["exploration"]["actions_used"],
        "finding_count": len(validated["findings"]),
        "finding_summaries": _finding_summaries(validated["findings"]),
    }


def _cleanup_summary(cleanup_payload):
    return {"cleanup_failed": cleanup_payload["cleaned"]["cleanup_failed"]}


def _validate_record_summary(kind, summary):
    if not isinstance(summary, dict):
        raise ValueError("execution record summary is invalid")
    if kind == "main":
        if set(summary) != {
            "replay_status",
            "exploration_status",
            "exploration_actions",
            "finding_count",
            "finding_summaries",
        }:
            raise ValueError("main execution summary fields are invalid")
        if summary["replay_status"] not in {"passed", "failed"}:
            raise ValueError("main execution replay summary is invalid")
        if summary["exploration_status"] not in {"passed", "failed", "not_started"}:
            raise ValueError("main execution exploration summary is invalid")
        for key in ("exploration_actions", "finding_count"):
            if not isinstance(summary[key], int) or isinstance(summary[key], bool) or summary[key] < 0:
                raise ValueError("main execution summary count is invalid")
        _validate_finding_summaries(summary["finding_summaries"])
        return
    if set(summary) != {"cleanup_failed"} or not isinstance(summary["cleanup_failed"], bool):
        raise ValueError("cleanup execution summary is invalid")


def _validate_cleanup_manifest(manifest, cfg):
    if not isinstance(manifest, dict):
        raise ValueError("cleanup manifest must be an object")
    if set(manifest) != {
        "schema_version",
        "kind",
        "run_id",
        "execution_id",
        "main_execution_id",
        "created_at",
        "status",
        "cleaned",
    }:
        raise ValueError("cleanup manifest fields are invalid")
    if manifest["schema_version"] != ROOT_MANIFEST_SCHEMA_VERSION:
        raise ValueError("cleanup manifest schema is invalid")
    if (
        manifest["kind"] != "cleanup"
        or manifest["run_id"] != cfg.run_id
        or manifest["execution_id"] != cfg.cleanup_execution_id
        or manifest["main_execution_id"] != cfg.main_execution_id
    ):
        raise ValueError("cleanup manifest identity is invalid")
    if not _safe_gcs_component(manifest["execution_id"]) or not _safe_gcs_component(manifest["main_execution_id"]):
        raise ValueError("cleanup manifest execution_id is invalid")
    if not isinstance(manifest["created_at"], int) or isinstance(manifest["created_at"], bool) or manifest["created_at"] < 0:
        raise ValueError("cleanup manifest created_at is invalid")
    if not isinstance(manifest["status"], str) or manifest["status"] not in {"passed", "cleanup_failed"}:
        raise ValueError("cleanup manifest status is invalid")
    cleaned = manifest["cleaned"]
    if not isinstance(cleaned, dict) or set(cleaned) != {
        "cleanup_failed",
        "deleted_token_count",
        "account_deleted",
        "login_rejected_after_delete",
        "reason",
    }:
        raise ValueError("cleanup manifest cleaned payload is invalid")
    if cleaned["cleanup_failed"] is not (manifest["status"] == "cleanup_failed"):
        raise ValueError("cleanup manifest status is inconsistent")
    if not isinstance(cleaned["deleted_token_count"], int) or isinstance(cleaned["deleted_token_count"], bool) or cleaned["deleted_token_count"] < 0:
        raise ValueError("cleanup manifest deleted token count is invalid")
    if not isinstance(cleaned["account_deleted"], bool) or not isinstance(cleaned["login_rejected_after_delete"], bool):
        raise ValueError("cleanup manifest booleans are invalid")
    if not isinstance(cleaned["reason"], str) or not cleaned["reason"]:
        raise ValueError("cleanup manifest reason is invalid")


def _json_bytes(payload):
    return json.dumps(payload, sort_keys=True, separators=(",", ":")).encode("utf-8")


def _finding_summaries(findings):
    summaries = []
    for finding in findings:
        if finding["severity"] == "info":
            continue
        summaries.append(
            {
                "severity": finding["severity"],
                "title": _summary_title(finding["title"]),
                "confidence": finding["confidence"],
                "page_path": _summary_page_path(finding["target_url"]),
            }
        )
        if len(summaries) == 3:
            break
    return summaries


def _validate_finding_summaries(summaries):
    if not isinstance(summaries, list) or len(summaries) > 3:
        raise ValueError("main execution finding summaries are invalid")
    for item in summaries:
        if not isinstance(item, dict) or set(item) != {"severity", "title", "confidence", "page_path"}:
            raise ValueError("main execution finding summary fields are invalid")
        if item["severity"] not in (report.SEVERITIES - {"info"}):
            raise ValueError("main execution finding summary severity is invalid")
        if item["confidence"] not in report.CONFIDENCE:
            raise ValueError("main execution finding summary confidence is invalid")
        title = item["title"]
        if (
            not isinstance(title, str)
            or not title
            or len(title) > 160
            or title != " ".join(title.split())
            or _has_control_character(title)
        ):
            raise ValueError("main execution finding summary title is invalid")
        page_path = item["page_path"]
        if (
            not isinstance(page_path, str)
            or not page_path.startswith("/")
            or "?" in page_path
            or "#" in page_path
            or _has_control_character(page_path)
        ):
            raise ValueError("main execution finding summary page path is invalid")


def _summary_title(title):
    folded = "".join(" " if _is_control_character(char) else char for char in title).split()
    return " ".join(folded)[:160]


def _summary_page_path(target_url):
    path = urllib.parse.urlsplit(target_url).path
    return path if path.startswith("/") else "/"


def _has_control_character(value):
    return any(_is_control_character(char) for char in value)


def _is_control_character(char):
    return ord(char) < 32 or ord(char) == 127


def _canonical_manifest_path(run_id, kind, execution_id):
    filename = "cleanup.json" if kind == "cleanup" else "manifest.json"
    return f"runs/{run_id}/{kind}/{execution_id}/{filename}"


def _safe_gcs_component(value):
    return (
        isinstance(value, str)
        and _SAFE_GCS_COMPONENT.fullmatch(value) is not None
        and value not in {".", ".."}
        and ".." not in value
        and "/" not in value
        and "\\" not in value
    )


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
