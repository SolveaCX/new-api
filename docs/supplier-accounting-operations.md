# Supplier accounting operations

The canonical accounting contract is [Upstream supply chain and profit accounting V1](./supplier-supply-chain-v1.md). GCP ownership and traffic caveats are in [`deploy/gcp/docs/OPERATIONS.md`](../deploy/gcp/docs/OPERATIONS.md).

## Runtime topology and deadlines

- Terraform uses a fail-closed two-phase rollout: with `supplier_batch_job_enabled=false` it creates identities, empty Secret containers, and IAM; after both token versions and Console hashes are ready, `enabled=true` creates the one-shot Cloud Run Job `newapi-supplier-batch` and Cloud Scheduler job `newapi-supplier-batch-daily` from a required digest-pinned image.
- Ordinary production/staging application deploys and ordinary rollback never inspect, mutate, or execute the supplier Job/Scheduler. Runner image ownership belongs only to the explicit `GCP Promote Supplier Runner` workflow, which uses its fixed WIF provider/service account and Terraform-owned Job name after its finalized-ledger, immutable-binding, association, manual-execution, and log-proof gates pass. Promotion never mutates Scheduler and leaves it paused for a separately authorized operator.
- Scheduler starts the Job at `02:05 Asia/Shanghai`. It never calls Console directly. One successful Job processes at most one oldest never-published Shanghai day; additional backlog is drained by additional serial executions.
- The server batch hard stop is 45 minutes, the runner HTTP timeout is 55 minutes, the Cloud Run Job timeout is at least 60 minutes, and the Console Cloud Run request timeout is 60 minutes. Do not shorten or invert this order.
- `08:00 Asia/Shanghai` is the completion SLO, not the schedule. Alert when the prior day is still unpublished at 08:00, pending never-published days are at least two, or the oldest never-published day is older than 24 hours.
- The runner receives only `SUPPLIER_BATCH_CONSOLE_URL`, `SUPPLIER_BATCH_TOKEN`, and optional `SUPPLIER_BATCH_POLL_INTERVAL`. It uses a stable `Idempotency-Key`/`request_id`; after a lost or ambiguous POST response it polls scheduler-only status by that same request ID and never invents a second key.

## Identity and least privilege

Terraform creates separate identities:

| Identity | Allowed | Explicitly not allowed |
| --- | --- | --- |
| `newapi-supplier-batch-runner` | Read the two supplier-batch token Secret containers; write logs; call the scheduler-only Console endpoints with the selected raw token | Root/Admin APIs, report rerun, Cloud Run administration |
| `newapi-supplier-scheduler` | `roles/run.invoker` on `newapi-supplier-batch` only | Read secrets, call Console, invoke other Jobs/services |
| `newapi-supplier-promoter` | Fixed Job `run.jobs.get/update/run`; project-level read-only Run/log observer; fixed Artifact Registry repository reader; `actAs` only on `newapi-supplier-batch-runner` | Every `cloudscheduler.*`, other Jobs, other repositories/runtime SAs |

The historical `newapi-ci-deployer` is build-only and has only repository-scoped Artifact Registry writer. Production Console/Router/website and staging deploy use fixed deploy identities; Console/Router rollback use separate fixed identities with no Artifact Registry access and no `actAs`. Each privileged workflow lane has its own workflow/ref/event/subject-pinned WIF pool. Never add or consume `GCP_DEPLOYER_SA` as a generic escape hatch.

Bootstrap this split with a reviewed local Owner-ADC refreshing plan/apply before any workflow references the new identities. CI `gcp-infra.yml` has no GCP auth or production backend and only runs `terraform init -backend=false`, validate, and static contracts; it never plans or applies production.

GitHub protections and credentials are external fail-closed prerequisites. Current evidence shows reviewers but no deployment branch policy and `prevent_self_review=false` on `production`, `production-console`, and `production-router`; an unprotected `staging` branch; no `production-infra` Environment or CI infra identity; and missing `SUPPLIER_DEPLOY_ROOT_ACCESS_TOKEN` / `SUPPLIER_DEPLOY_ROOT_USER_ID`. This document does not claim those settings were changed. Configure and independently verify them before the relevant workflow; missing Root inputs must stop promotion/rollback, and production Root credentials must never be copied to staging.

The Secret Manager containers are `newapi-supplier-batch-token-current` and `newapi-supplier-batch-token-next`. Terraform creates containers and IAM only: it never contains a token value, a concrete secret version, or a verifier hash. Only the runner service account has raw current/next accessor permission. Console receives these hashes only:

```text
SUPPLIER_BATCH_TOKEN_CURRENT_SHA256
SUPPLIER_BATCH_TOKEN_NEXT_SHA256
SUPPLIER_BATCH_TRUSTED_JOB_IDENTITY
```

The trusted identity is stable across slot rotation. A verifier hash is not a bearer credential; sending a SHA-256 hash as `Authorization: Bearer ...` is rejected. Never reuse or migrate a Root access token for the scheduler.

## Dual-slot token rotation

Perform rotation from a controlled operator shell. Do not paste the raw token into Terraform variables, GitHub Actions inputs, tickets, logs, or command history.

1. Select the inactive slot (`next` when `current` is active, otherwise `current`). Generate exactly 32 random bytes and encode with unpadded base64url. A safe generator is:

   ```bash
   umask 077
   openssl rand 32 | openssl base64 -A | tr '+/' '-_' | tr -d '=' > /tmp/supplier-batch-token
   test "$(wc -c < /tmp/supplier-batch-token | tr -d ' ')" -eq 43
   shasum -a 256 /tmp/supplier-batch-token | awk '{print $1}' > /tmp/supplier-batch-token.sha256
   ```

2. On first setup, run the first reviewed Terraform plan/apply with `supplier_batch_job_enabled=false`; this creates both empty Secret containers/IAM but no Job/Scheduler. Add raw 32-byte tokens to both containers out of band. On later rotations, add only the inactive slot. Use `--data-file`; never use Terraform `secret_data` and never print the token.
3. Keep the active Console verifier hash and add the inactive slot's new hash. Keep `SUPPLIER_BATCH_TRUSTED_JOB_IDENTITY` unchanged. Roll Console and verify the verifier configuration is available before switching the Job.
4. For first enablement, set `supplier_batch_job_enabled=true` and supply the verified `supplier_batch_runner_image=image@sha256:...`; for later rotation change only `supplier_batch_active_token_slot`. Run the second reviewed plan/apply. The Job reads `latest` from the selected container; Terraform state still contains no raw value or concrete version. Execute one manual Job and confirm its command audit slot is the selected slot.
5. Monitor through the agreed rotation deadline: no `unauthorized`/`verifier_unavailable`, no active old-slot commands, scheduler status replay works across slots, and the 08:00 SLO remains healthy.
6. After the deadline, clear the old slot hash from Console, deploy Console, disable/destroy the old Secret version out of band, and delete `/tmp/supplier-batch-token*`. Keep both empty Secret containers for the next rotation.

Rollback during rotation is the inverse slot switch while both hashes are still accepted. After the old hash/version is revoked, restoring it requires a new controlled rotation; never substitute the hash as bearer.

## Manual Scheduler activation after promotion

Terraform creates/re-creates `newapi-supplier-batch-daily` paused, and `GCP Promote Supplier Runner` leaves it paused even after the fixed Job passes both association checks and its manual execution proof. A separately authorized operator must run this exact sequence from the repository root:

```bash
set -euo pipefail
project=vocai-gemini-prod
region=us-west1
scheduler=newapi-supplier-batch-daily
job=newapi-supplier-batch
scheduler_sa=newapi-supplier-scheduler@vocai-gemini-prod.iam.gserviceaccount.com

verify_scheduler() {
  bash .github/scripts/supplier-resource-association-verify.sh scheduler \
    "$1" "$project" "$region" "$scheduler" "$job" "$scheduler_sa" "$2"
}

gcloud scheduler jobs describe "$scheduler" \
  --project="$project" --location="$region" --format=json \
  > /tmp/newapi-supplier-batch-daily.paused.json
verify_scheduler /tmp/newapi-supplier-batch-daily.paused.json PAUSED

if ! gcloud scheduler jobs resume "$scheduler" \
  --project="$project" --location="$region"; then
  gcloud scheduler jobs pause "$scheduler" \
    --project="$project" --location="$region"
  gcloud scheduler jobs describe "$scheduler" \
    --project="$project" --location="$region" --format=json \
    > /tmp/newapi-supplier-batch-daily.reverted.json
  verify_scheduler /tmp/newapi-supplier-batch-daily.reverted.json PAUSED
  exit 1
fi

if ! gcloud scheduler jobs describe "$scheduler" \
    --project="$project" --location="$region" --format=json \
    > /tmp/newapi-supplier-batch-daily.enabled.json || \
   ! verify_scheduler /tmp/newapi-supplier-batch-daily.enabled.json ENABLED; then
  gcloud scheduler jobs pause "$scheduler" \
    --project="$project" --location="$region"
  gcloud scheduler jobs describe "$scheduler" \
    --project="$project" --location="$region" --format=json \
    > /tmp/newapi-supplier-batch-daily.reverted.json
  verify_scheduler /tmp/newapi-supplier-batch-daily.reverted.json PAUSED
  exit 1
fi
```

The verifier checks the exact target Job, OAuth service account/scope, schedule/timezone, retry/deadline/state, empty body, and safe headers. The sequence returns to and re-verifies `PAUSED` when resume or the `ENABLED` read-back fails. If that recovery verification also fails, treat it as a production incident; do not infer safety from a successful `resume` or `pause` command alone.

## Scheduler result handling

- A completed scan containing malformed persisted supplier rows publishes the computable amounts with bounded warnings and `persisted_log_snapshot_completeness=incomplete`, then advances the never-published backlog. Row defects are finance evidence, not an execution retry loop.
- Log-page read/scan errors, fence loss, lease failure, main-DB transaction failure, and other execution errors publish nothing. They retain the prior published generation, if any, and remain eligible for bounded reconciliation/retry under a new fence.
- `completed` with `processed_days=0`, null date/run, fence zero, null lease, and `remaining_work=false` is the exact durable no-work result.
- `running` means poll the status route using the same request ID.
- `404 not_found` after an ambiguous POST is retried only under the runner's bounded reconciliation policy with the same key.
- `409 busy` means another globally fenced catch-up or Root rerun owns work; do not start a competing key.
- `409 idempotency_conflict` is an operator-visible protocol error, not a retry-with-new-key signal.
- `503 verifier_unavailable` or `config_unavailable` is fail closed; repair Console configuration before retrying.
- HTTP 408, 429, network errors, and 5xx are ambiguous until status reconciliation completes. Never mark the day complete from transport status alone.

## Compatibility-bridge finalization

`FinalizeSupplierAdminCommandLedgerMigration` is a post-drain protocol boundary, not a startup migration. During a mixed-version rollout both legacy uniqueness bridges must remain:

- command-ledger legacy indexes `ux_supplier_admin_command_scope_key` and `idx_supplier_admin_command_actor_scope_key`;
- inventory legacy index `ux_supplier_inventory_contract_idempotency`.

The new actor-local indexes must already exist: `ux_supplier_admin_command_actor_scope_digest`, `ux_supplier_batch_scheduler_identity_scope_digest`, and `ux_supplier_inventory_actor_idempotency`.

Use this order exactly:

1. Keep the versioned supplier mutation gate disabled.
2. Deploy bridge-compatible Console and Router revisions. If legacy `newapi` still serves any traffic, deploy it too.
3. Preflight the new Console at a 0%-traffic tagged URL, switch Console directly to 100%, and wait the full 60-minute maximum management-request timeout.
4. Prove every incompatible old Console/Router/legacy writer revision has drained. Do not infer drain from a new revision merely being ready.
5. Run `/app/supplier_admin_finalize verify`; failure with legacy bridges present is expected before finalization.
6. Set `SUPPLIER_ADMIN_FINALIZE_EXPECTED_DB_IDENTITY` to the exact reported physical identity (`mysql:<database>` or `postgres:<database>/<schema>`) and `SUPPLIER_ADMIN_FINALIZE_DRAIN_EVIDENCE_REF` to a non-secret, bounded change/ticket/evidence reference. A missing or mismatched identity, missing options/command/inventory prerequisite, malformed mutation state, or enabled mutation gate fails before bridge removal.
7. Run `/app/supplier_admin_finalize finalize` once, then run `verify` twice. The command revalidates the disabled gate after destructive bridge removal and exits non-zero unless both command-ledger bridges and the inventory bridge are absent, all three new indexes have exact safe definitions, scheduler columns exist, and every digest is valid.
8. Re-run authenticated status/readiness/preflight and require `admin_command_ledger_state=finalized`. `bridge` means all three legacy bridges and all replacement invariants are intact; `invalid` means a missing/partial/unknown prerequisite and blocks promotion. Only then may Root enable expected-version mutations through secure verification and expected-version CAS.

The CLI requires explicit `SQL_DSN`, expected database identity, and drain-evidence reference, refuses SQLite fallback, opens one database connection, and does not call application `InitDB`, AutoMigrate the whole application, expose an HTTP route, or remain resident. Its JSON audit output contains the dialect, verified database identity, evidence reference, prerequisite booleans, and exact ledger state; it never prints the DSN or credentials. Run it from the digest-pinned release image as an auditable one-off Cloud Run Job using the existing runtime service account, Cloud SQL attachment, and `newapi-sql-dsn` secret; set command `/app/supplier_admin_finalize` and args `finalize` or `verify`, `max-retries=0`, then remove the one-off Job after retaining execution logs.

If finalization fails, keep the mutation gate disabled. The validator reports which bridge/index/digest invariant is unsafe; repair that condition and rerun the same command. Never manually drop only one bridge and never enable the gate based on partial success.

## Rollback and disaster order

- Before finalization: keep the gate disabled and roll Console/Router/legacy traffic back to a bridge-compatible digest normally.
- After finalization: old writers that require either legacy bridge may not return. Keep/return the gate to disabled and roll only to a revision proven compatible with the finalized indexes. Recreating legacy uniqueness is a separate reviewed data migration because actor-local duplicate keys may now exist.
- If accounting is already active and an incompatible emergency shift is unavoidable, first atomically enter degraded state and open a named coverage-gap epoch, then shift traffic. Do not shift first and document later.
- Stop/disable the Scheduler trigger before database repair, preserve additive schema and published fence evidence, restore an accepted Console/Router capability, reconcile the stable request ID, then resume Scheduler serially.
- Website, Cloudflare, and public DNS are not part of this rollout or rollback.

## Exact backlog observability

The existing Router Managed Prometheus `/metrics` surface exports one shared, DB-derived backlog snapshot as low-cardinality gauges:

| Gauge | Meaning and alert contract |
| --- | --- |
| `newapi_supplier_accounting_never_published_days` | Current number of eligible Shanghai dates with `published_fence_token = 0`; alert when >1 for 60 seconds (current backlog >=2). |
| `newapi_supplier_accounting_oldest_never_published_age_seconds` | Current age of the oldest never-published date; alert only when strictly >86400 for 60 seconds. |
| `newapi_supplier_accounting_prior_day_unpublished_after_0800` | At/after 08:00 Asia/Shanghai, `1` when the prior day is still unpublished; alert when >0 for 60 seconds. Before 08:00 it remains `0`. |
| `newapi_supplier_accounting_backlog_observer_up` | Snapshot health; alert when the max across all Router instances remains <1 for 120 seconds, or when the metric is absent for 120 seconds. |
| `newapi_supplier_accounting_backlog_observed_at_seconds` | Database timestamp of the last successful observation; use it to corroborate freshness and fire/resolve evidence. |

Terraform filters Managed Prometheus `prometheus_target` series to `service=newapi-router`/the configured `router_service_name`. It reduces instance/revision duplicates with max and never sum: each Router observes the same database facts, so a sum would amplify values as instances scale. The normal threshold duration covers at least two 30-second scrapes; health uses 120 seconds, which is strictly beyond the 90-second stale boundary.

With `supplier_batch_job_enabled=false`, Terraform creates no supplier alert policies. That valid phase-one state must remain quiet; do not add an absence policy outside the same enable gate. The monitoring path reuses the Router sidecar and adds no observer Job, Scheduler, identity, secret, log metric, table, or index.

Every Router instance is scraped every 30 seconds, so snapshot read load grows with instance count. G006 must measure production-equivalent maximum-scale scrape/query amplification and DB latency/CPU/connections/locks before release. G006 must also retain approved staging/live evidence showing each threshold fires and resolves, plus observer-down and metric-absence fire/resolve timelines checked against `observed_at_seconds`. Until those artifacts exist, the configuration is implemented but production observability evidence remains incomplete.

## Release verification

- Terraform: `terraform fmt -check -recursive deploy/gcp` and `terraform -chdir=deploy/gcp/envs/prod validate`.
- IAM: Scheduler can start only `newapi-supplier-batch`; Scheduler cannot access either secret; runner can access only the two batch-token secrets and has no Root/Admin authority.
- Deadlines: server 45m, runner 55m, Job >=60m, Console 60m.
- Deployment targets: Terraform, `newapi-console`, `newapi-router`, and legacy `newapi` only if it still serves traffic. `newapi-web` and Cloudflare are uninvolved.
- No raw token, secret payload, concrete Secret Manager version, Root token, or hash-as-bearer appears in Terraform state, build metadata, application env intended for the runner, or logs.
