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
| `newapi_supplier_accounting_never_published_days` | Current number of eligible Shanghai dates that are absent from the batch-run table or have `published_fence_token = 0`; alert when >1 for 60 seconds (current backlog >=2). Existing rows must form a continuous prefix from cutover; an interior date hole makes the observer unhealthy instead of silently understating backlog. |
| `newapi_supplier_accounting_oldest_never_published_age_seconds` | Current age of the oldest never-published date; alert only when strictly >86400 for 60 seconds. |
| `newapi_supplier_accounting_prior_day_unpublished_after_0800` | At/after 08:00 Asia/Shanghai, `1` when the prior day is still unpublished; alert when >0 for 60 seconds. Before 08:00 it remains `0`. |
| `newapi_supplier_accounting_backlog_observer_up` | Snapshot health; alert when the max across all Router instances remains <1 for 120 seconds, or when the metric is absent for 120 seconds. |
| `newapi_supplier_accounting_backlog_observed_at_seconds` | Database timestamp of the last successful observation; use it to corroborate freshness and fire/resolve evidence. |

Terraform filters Managed Prometheus `prometheus_target` series to `service=newapi-router`/the configured `router_service_name`. It reduces instance/revision duplicates with max and never sum: each Router observes the same database facts, so a sum would amplify values as instances scale. The normal threshold duration covers at least two 30-second scrapes; health uses 120 seconds, which is strictly beyond the 90-second stale boundary.

With `supplier_batch_job_enabled=false`, Terraform creates no supplier alert policies. That valid phase-one state must remain quiet; do not add an absence policy outside the same enable gate. The monitoring path reuses the Router sidecar and adds no observer Job, Scheduler, identity, secret, log metric, table, or index.

Every Router instance is scraped every 30 seconds, so snapshot read load grows with instance count. G006 must measure production-equivalent maximum-scale scrape/query amplification and DB latency/CPU/connections/locks before release. G006 must also retain approved staging/live evidence showing each threshold fires and resolves, plus observer-down and metric-absence fire/resolve timelines checked against `observed_at_seconds`. Until those artifacts exist, the configuration is implemented but production observability evidence remains incomplete.

### G006 local 2x configured-scale evidence and remaining blockers

The opt-in test `TestSupplierAccountingBacklogMaximumRouterBurstCapacity` reads `router_max_instances` from `deploy/gcp/envs/prod/terraform.tfvars`, contract-checks the currently reviewed value `30`, and derives both the 2x burst and series counts from that source. It executes synchronized cold calls to `refreshSupplierAccountingBacklogPrometheusSnapshot` over a 365-day continuous prefix. Each call bypasses only the process-local cache/singleflight and executes the production cold-refresh DB contract: activation option, DB time, and one-row aggregate. Its shared pool emulates aggregate DB concurrency; it is not independent Router pools and is not authenticated HTTP `GET /metrics` or Prometheus render latency.

Run it only against the two enforced isolated databases (`supplier_g009_mysql` and `supplier_g009_postgres`):

```bash
RUN_SUPPLIER_CAPACITY_TEST=1 \
SUPPLIER_CAPACITY_ISOLATION_TOKEN='<token already present in supplier_capacity_test_sentinel in both databases>' \
SUPPLIER_CAPACITY_EVIDENCE_DIR='./artifacts/supplier-capacity' \
TEST_MYSQL_DSN='<isolated MySQL DSN for supplier_g009_mysql>' \
TEST_POSTGRES_DSN='<isolated PostgreSQL DSN for supplier_g009_postgres>' \
go test ./pkg/perf_metrics -run '^TestSupplierAccountingBacklogMaximumRouterBurstCapacity$' -count=1 -v
```

| Local database | Burst wall time | p50 | p95 | p99/max | Pool wait | Connections before/after | Waiting locks before/after |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| MySQL 8.0.46 | 32.615 ms | 25.478 ms | 31.442 ms | 32.175 ms | 0 | 1 / 60 | 0 / 0 |
| PostgreSQL 15.18 | 106.009 ms | 90.109 ms | 103.489 ms | 105.538 ms | 0 | 1 / 60 | 0 / 0 |

Before any migration or cleanup, both DSNs must parse to loopback and each database must contain an externally provisioned `supplier_capacity_test_sentinel` row matching the shared token. A successful run writes current schema-v2 `supplier-observer.json` with generation time, HEAD/dirty state, Terraform source path/hash, real timings, series math, unavailable fields, and release blockers. The workflow never copies the checked-in historical draft; artifact authority is defined by the success attestation below. The table below and [`docs/evidence/supplier-g006-local-capacity.json`](./evidence/supplier-g006-local-capacity.json) are provisional local observations only, not final release evidence. Real-path behavior is covered separately by `TestPrometheusMetricsAuth`, `TestSetPrometheusMetricsRouterRegistersMetricsRoute`, `TestBuildPrometheusTextEmitsSupplierAccountingBacklogFixedGauges`, and `TestSupplierAccountingBacklogObserverSingleflightsConcurrentScrapes`; none turns this DB-contract measurement into HTTP E2E latency evidence.

### Production-p99 T+1 local gate

`TestSupplierT1ProductionP99Capacity` is inert unless `RUN_SUPPLIER_T1_CAPACITY_TEST=1`. Once enabled, missing/invalid `V`, either DSN, the external sentinel token, or provenance fails the test; none may skip. The required provenance is an ordered RFC3339 measurement window, reviewable query/report reference, and 64-character lowercase SHA-256. Relative to validation time, its production measurement end must satisfy the inclusive boundary `now - 7×24h <= end <= now + 5m`. For each isolated MySQL/PostgreSQL database it rounds the target upward to a multiple of 20 after applying `max(1,000,000, 2V)`, preserving both the lower bound and exact 95% internal / 5% business mix, plus the same number of older background rows. A smaller developer smoke still requires `SUPPLIER_T1_CAPACITY_ALLOW_SMALL_SMOKE=1` and remains `smoke_not_release`; the manual workflow never sets it.

```bash
RUN_SUPPLIER_T1_CAPACITY_TEST=1 \
SUPPLIER_PRODUCTION_P99_DAILY_LOGS='<measured production p99 V>' \
SUPPLIER_CAPACITY_MEASUREMENT_WINDOW_START='<RFC3339>' \
SUPPLIER_CAPACITY_MEASUREMENT_WINDOW_END='<RFC3339>' \
SUPPLIER_CAPACITY_SOURCE_REFERENCE='<query/report identifier>' \
SUPPLIER_CAPACITY_SOURCE_SHA256='<64 lowercase hex>' \
SUPPLIER_CAPACITY_ISOLATION_TOKEN='<externally provisioned shared token>' \
SUPPLIER_CAPACITY_EVIDENCE_DIR='./artifacts/supplier-capacity' \
TEST_MYSQL_DSN='<isolated MySQL DSN for supplier_g009_mysql>' \
TEST_POSTGRES_DSN='<isolated PostgreSQL DSN for supplier_g009_postgres>' \
go test ./capacity -run '^TestSupplierT1ProductionP99Capacity$' -count=1 -v -timeout=120m
```

The manual-only GitHub workflow `Supplier accounting capacity evidence` is a required pre-activation release gate, not merge CI. It uses `ubuntu-24.04`, digest-pinned MySQL 8.0.46 and PostgreSQL 15.18-alpine services, provisions both sentinels externally, and retains uploads for 90 days. Its `always()` upload preserves failed or partial artifacts for diagnosis only and must never authorize activation. The local gate passes only for a complete artifact containing a validated, success-only `SUCCESS_MANIFEST.json`: validation binds current `GITHUB_SHA` and a clean working tree; requires exactly `supplier-observer.json`, `supplier-t1-mysql.json`, and `supplier-t1-postgres.json`; checks their schema/evidence class/dialect and workflow-window timestamps; records their SHA-256 digests; and requires complete runner metadata.

`SUCCESS_MANIFEST.json` attests only the integrity and successful execution of this local capacity run. The workflow copies the operator-provided source reference, claimed source SHA-256, measurement window, and `V` into generated evidence, but has no credential or storage trust path with which to authenticate that external production source. Before release approval, the approver must independently retrieve the immutable source, recompute and match its SHA-256, then reconcile its reference, measurement window, and `V` against both T+1 JSON files. Activation remains explicitly blocked until that external verification is recorded. Even after it is recorded, production-distribution `EXPLAIN`, actual rows examined/read <=1.2x, Cloud SQL CPU average/peak, lock-wait distribution, replica lag, and approved live alert fire/resolve remain release blockers.

The envelope golden tests measure a worst-case full `logs.other` payload of 330 bytes for business scope and 320 bytes for internal scope. At the stated 95% internal / 5% business mix, the conservative all-captured mean is 320.5 bytes per successful persisted consume log. With production p99 successful-log volume `V`, raw payload growth is `320.5 * V` bytes/day and `9,615 * V` bytes/30-day month (before database row/page/backup overhead). Examples: one million logs/day is 320.5 MB/day and 9.615 GB/30 days; five million logs/day is 1.6025 GB/day and 48.075 GB/30 days. The actual production p99 `V`, database overhead multiplier, Cloud SQL CPU/connections/locks, MySQL 5.7.8/PostgreSQL 9.6 runtime behavior, and approved staging/live alert fire-resolve timelines remain release blockers because this local task had no production access.

Local deterministic signal-transition tests prove the metric inputs for backlog `2 -> 0`, oldest age `86401 -> 0`, 08:00 miss `1 -> 0`, observer health `0 -> 1`, and configured metric presence -> omission. Terraform contract tests prove the matching `>1`, `>86400`, `>0`, `<1`, and 120-second absence conditions. They do not prove Cloud Monitoring incident creation, notification delivery, or automatic resolution; retain those staging/live artifacts before release.

## Release verification

- Terraform: `terraform fmt -check -recursive deploy/gcp` and `terraform -chdir=deploy/gcp/envs/prod validate`.
- IAM: Scheduler can start only `newapi-supplier-batch`; Scheduler cannot access either secret; runner can access only the two batch-token secrets and has no Root/Admin authority.
- Deadlines: server 45m, runner 55m, Job >=60m, Console 60m.
- Deployment targets: Terraform, `newapi-console`, `newapi-router`, and legacy `newapi` only if it still serves traffic. `newapi-web` and Cloudflare are uninvolved.
- No raw token, secret payload, concrete Secret Manager version, Root token, or hash-as-bearer appears in Terraform state, build metadata, application env intended for the runner, or logs.
