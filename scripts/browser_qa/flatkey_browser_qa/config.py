import base64
import binascii
import os
from dataclasses import dataclass

from .origin_policy import OriginPolicy


_REQUIRED_ENV = {
    "FLATKEY_QA_RUN_ID",
    "FLATKEY_QA_IDENTITY_SEED_B64",
    "FLATKEY_QA_GMAIL_BASE",
    "FLATKEY_QA_WEBSITE_ORIGIN",
    "FLATKEY_QA_CONSOLE_ORIGIN",
    "FLATKEY_QA_DOCS_ORIGIN",
}
_ALLOWED_ORIGINS = {
    "FLATKEY_QA_WEBSITE_ORIGIN": "https://staging-website.flatkey.ai",
    "FLATKEY_QA_CONSOLE_ORIGIN": "https://staging-console.flatkey.ai",
    "FLATKEY_QA_DOCS_ORIGIN": "https://docs.flatkey.ai",
}
_CLEANUP_REQUIRED_ENV = {
    "FLATKEY_QA_RUN_ID",
    "FLATKEY_QA_IDENTITY_SEED_B64",
    "FLATKEY_QA_CONSOLE_ORIGIN",
}


@dataclass(frozen=True, repr=False)
class RuntimeConfig:
    run_id: str
    identity_seed: bytes
    gmail_base: str
    website_origin: str
    console_origin: str
    docs_origin: str
    origin_policy: OriginPolicy

    def __repr__(self):
        return f"RuntimeConfig(run_id={self.run_id!r})"


@dataclass(frozen=True, repr=False)
class CleanupConfig:
    run_id: str
    identity_seed: bytes
    console_origin: str

    def __repr__(self):
        return f"CleanupConfig(run_id={self.run_id!r})"


def load_config(env=None):
    env = os.environ if env is None else env
    missing = sorted(name for name in _REQUIRED_ENV if not env.get(name))
    if missing:
        raise ValueError(f"missing required environment variables: {', '.join(missing)}")

    extras = sorted(name for name in env if name.startswith("FLATKEY_QA_") and name not in _REQUIRED_ENV)
    if extras:
        raise ValueError(f"unknown FLATKEY_QA_ environment variables: {', '.join(extras)}")

    run_id = _validate_run_id(env["FLATKEY_QA_RUN_ID"])

    origins = {name: env[name] for name in _ALLOWED_ORIGINS}
    for name, expected in _ALLOWED_ORIGINS.items():
        if origins[name] != expected:
            raise ValueError(f"{name} must be exactly {expected}")

    seed = _decode_identity_seed(env["FLATKEY_QA_IDENTITY_SEED_B64"])

    policy = OriginPolicy.from_hosts(
        [
            "staging-website.flatkey.ai",
            "staging-console.flatkey.ai",
            "docs.flatkey.ai",
        ]
    )

    return RuntimeConfig(
        run_id=run_id,
        identity_seed=seed,
        gmail_base=env["FLATKEY_QA_GMAIL_BASE"],
        website_origin=origins["FLATKEY_QA_WEBSITE_ORIGIN"],
        console_origin=origins["FLATKEY_QA_CONSOLE_ORIGIN"],
        docs_origin=origins["FLATKEY_QA_DOCS_ORIGIN"],
        origin_policy=policy,
    )


def load_cleanup_config(env=None):
    env = os.environ if env is None else env
    missing = sorted(name for name in _CLEANUP_REQUIRED_ENV if not env.get(name))
    if missing:
        raise ValueError(f"missing required environment variables: {', '.join(missing)}")

    extras = sorted(name for name in env if name.startswith("FLATKEY_QA_") and name not in _CLEANUP_REQUIRED_ENV)
    if extras:
        raise ValueError(f"unknown FLATKEY_QA_ environment variables: {', '.join(extras)}")

    console_origin = env["FLATKEY_QA_CONSOLE_ORIGIN"]
    expected = _ALLOWED_ORIGINS["FLATKEY_QA_CONSOLE_ORIGIN"]
    if console_origin != expected:
        raise ValueError(f"FLATKEY_QA_CONSOLE_ORIGIN must be exactly {expected}")

    return CleanupConfig(
        run_id=_validate_run_id(env["FLATKEY_QA_RUN_ID"]),
        identity_seed=_decode_identity_seed(env["FLATKEY_QA_IDENTITY_SEED_B64"]),
        console_origin=console_origin,
    )


def _validate_run_id(run_id):
    if not run_id.isascii() or not run_id.isdecimal():
        raise ValueError("FLATKEY_QA_RUN_ID must contain only ASCII decimal digits")
    return run_id


def _decode_identity_seed(value):
    try:
        seed = base64.b64decode(value, validate=True)
    except (binascii.Error, ValueError) as exc:
        raise ValueError("FLATKEY_QA_IDENTITY_SEED_B64 must be valid base64") from exc
    if len(seed) < 32:
        raise ValueError("identity seed must be at least 32 bytes")
    return seed
