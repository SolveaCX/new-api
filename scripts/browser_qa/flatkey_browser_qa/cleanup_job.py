import json
import re
import sys
import time

from .api import StagingApiClient
from .cleanup import CleanupRunner
from .config import load_cleanup_config
from .gcp import GcpClient, GcsObjectAlreadyExists, GcsUploadUncertain, read_gcs_json_object, upload_gcs_object
from .identity import derive_identity


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
    main_record = _main_record(cfg, access_token)
    records = list(root["executions"])
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
    status = manifest.get("status")
    created_at = manifest.get("created_at", 0)
    return _execution_record("main", cfg.main_execution_id, object_name, status, created_at)


def _append_or_validate_record(records, record):
    for existing in records:
        if existing["kind"] == record["kind"] and existing["execution_id"] == record["execution_id"]:
            if existing != record:
                raise ValueError("conflicting execution record")
            return records
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


def _execution_record(kind, execution_id, manifest, status, created_at):
    return {
        "kind": kind,
        "execution_id": execution_id,
        "manifest": manifest,
        "status": status,
        "created_at": created_at,
    }


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
    if set(record) != {"kind", "execution_id", "manifest", "status", "created_at"}:
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


def _validate_main_manifest(manifest, run_id, execution_id):
    if not isinstance(manifest, dict):
        raise ValueError("main manifest must be an object")
    allowed = {"schema_version", "kind", "run_id", "execution_id", "status", "created_at", "result", "cleanup", "infrastructure"}
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
    if "infrastructure" in manifest and not isinstance(manifest["infrastructure"], dict):
        raise ValueError("main manifest infrastructure is invalid")


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
