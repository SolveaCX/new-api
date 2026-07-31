import json
import sys

from .api import StagingApiClient
from .cleanup import CleanupRunner
from .config import load_cleanup_config
from .identity import derive_identity


def main(argv=None):
    argv = [] if argv is None else argv
    if argv:
        raise SystemExit("cleanup_job does not accept command line arguments")
    cfg = load_cleanup_config()
    identity = derive_identity(cfg.identity_seed, cfg.run_id)
    client = StagingApiClient(cfg.console_origin)
    result = CleanupRunner(client).run(identity)
    print(
        json.dumps(
            {
                "cleanup_failed": result.cleanup_failed,
                "deleted_token_count": result.deleted_token_count,
                "account_deleted": result.account_deleted,
                "login_rejected_after_delete": result.login_rejected_after_delete,
                "reason": result.reason,
            },
            sort_keys=True,
        )
    )
    return 1 if result.cleanup_failed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
