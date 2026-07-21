# Native Status Center Operations

This runbook covers the native NewAPI status center for Router and public-model health, history, incidents, and notification delivery. All four lifecycle switches default to `false`. Roll out the feature in stages; do not enable every surface in one deployment.

## Safety invariants

- The scheduler runs only on the master node and coordinates through a database lease plus fencing token.
- Shadow mode keeps evidence collection and evaluation running while suppressing public status access and all notification delivery.
- Public and notification switches are independent of the scheduler. This allows the public page to serve a last snapshot or the outbox to drain while evaluation is paused. New subscription creation requires both public and notification access.
- A component without trustworthy evidence for 20 minutes becomes `unknown`; stale, retired, and unknown states must never appear green.
- Missing or invalid optional notification encryption keys do not stop status reads, probes, evaluation, or email. They disable encrypted Webhook and Discord capabilities.
- Prometheus labels are fixed and low-cardinality. Metrics never include the scheduler holder, model names, probe targets, subscriber identities, endpoints, secrets, payloads, or raw error text.
- Status-center Prometheus metrics are authoritative only on the Console/master service. Router/slave `/metrics` exposes relay/process metrics, including the availability counters enabled by `STATUS_CENTER_ENABLED`, but never `newapi_status_center_*`. The first successful lease acquisition in each Console process records the holder in the internal application log for diagnosis without adding it to Prometheus.

## Configuration

| Variable | Purpose | Default / rule |
| --- | --- | --- |
| `STATUS_CENTER_ENABLED` | Records relay availability counters on Router/slave; runs catalog sync, Router/model probes, evaluation, rollups, and retention on Console/master. | `false`; set consistently on both services. |
| `STATUS_CENTER_PUBLIC_ENABLED` | Enables public `/api/status/*` reads and subscription entry points. | `false`; ineffective while shadow mode is on. |
| `STATUS_CENTER_NOTIFICATIONS_ENABLED` | Enables new subscription creation and runs the delivery outbox worker. | `false`; may run while the scheduler is off; ineffective in shadow mode. Existing verification/unsubscribe links remain manageable while public access is on. |
| `STATUS_CENTER_SHADOW_MODE` | Collects and evaluates without external public/notification effects. | `false`. |
| `ROUTER_ORIGIN` | Target origin for the real Router canary request to `/api/status`. | Required when the scheduler is enabled. Must be an absolute `http(s)` origin without credentials, path, query, or fragment. |
| `APP_CONSOLE_ORIGIN` | Console/API origin used by the Next.js website same-origin proxy. | Required by the independently deployed `website/` service. |
| `PROMETHEUS_METRICS_TOKEN` | Bearer token for remote `/metrics` scrapes. | Local loopback scrapes follow the existing metrics authentication policy. |
| `STATUS_SECRET_KEYS` | AES-256-GCM keyring in `key_id:base64-32-byte-key` form; entries are comma-separated. | Optional until Webhook or Discord is configured. Keep keys needed by existing envelopes. |
| `STATUS_SECRET_ACTIVE_KEY_ID` | Key ID used for new encrypted values. | Must name an entry in `STATUS_SECRET_KEYS`. |

Generate a 32-byte key with a secrets manager or `openssl rand -base64 32`. Store the keyring in the deployment secret store; never commit it, print it in logs, or place it in Prometheus labels.

## Required deployment targets

1. Router: availability classification and the public `/api/status` canary path.
2. NewAPI Console: additive database schema, scheduler, APIs, admin workspace, metrics, and delivery worker.
3. Website: public SSR status pages and the same-origin `/api/status/*` proxy.

Deploy schema/code with all switches off first. The status schema is additive; disabling the feature does not require dropping tables.

Production deployment wiring:

- configure the four lifecycle values, `STATUS_SECRET_ACTIVE_KEY_ID`, and `STATUS_SECRET_KEYS_SECRET_ID` as variables on the `production-console` GitHub Environment;
- configure the same `STATUS_CENTER_ENABLED` value on the `production-router` GitHub Environment so Router revisions record and flush the real relay availability counters consumed by model health;
- add the keyring value as a version of the Terraform-owned `newapi-status-secret-keys` Secret Manager secret, then set `STATUS_SECRET_KEYS_SECRET_ID=newapi-status-secret-keys`; never inject key material as a plain workflow variable or into `production-router`;
- keep `STATUS_CENTER_PUBLIC_ENABLED`, `STATUS_CENTER_NOTIFICATIONS_ENABLED`, `STATUS_CENTER_SHADOW_MODE`, and keyring variables off Router/slave revisions;
- scrape `newapi-console` through its Managed Prometheus sidecar for authoritative status-center metrics, and keep the Router sidecar for relay/process metrics;
- staging uses the corresponding `STAGING_STATUS_CENTER_*`, `STAGING_STATUS_SECRET_ACTIVE_KEY_ID`, and `STAGING_STATUS_SECRET_KEYS_SECRET_ID=newapi-staging-status-secret-keys` variables on its single master backend after an operator adds the Secret Manager version.

## Staged rollout

### 1. Deploy dark

Set all four Console switches and the Router copy of `STATUS_CENTER_ENABLED` to `false`. Confirm database migration completion, `/metrics` authentication, Router reachability from Console, and the website's `APP_CONSOLE_ORIGIN`.

Stop condition: the application is stable and status-center tables exist, but no status job or public surface is active.

### 2. Run shadow mode for at least 7 days

Set the following on Console, and set `STATUS_CENTER_ENABLED=true` on Router at the same rollout stage:

```dotenv
STATUS_CENTER_ENABLED=true
STATUS_CENTER_SHADOW_MODE=true
STATUS_CENTER_PUBLIC_ENABLED=false
STATUS_CENTER_NOTIFICATIONS_ENABLED=false
ROUTER_ORIGIN=https://router.example.com
```

Review false-positive rates, unknown-model count, coverage distribution, evaluator and rollup lag, probe volume/latency, database growth, and Router canary stability. Shadow mode overrides accidentally enabled public or notification flags.

Stop condition: at least seven complete days of evidence show acceptable false positives, coverage, lag, probe cost, and storage growth.

### 3. Administrator preview for at least 48 hours

Turn shadow mode off but keep public and notifications off. Administrators can inspect components, drafts, maintenance, delivery configuration, and audit history through authenticated status admin APIs/UI.

Stop condition: administrators have reviewed at least 48 hours of non-shadow evaluations and accepted incident-draft quality and recovery behavior.

### 4. Open public status

Set `STATUS_CENTER_PUBLIC_ENABLED=true` and keep notifications off. Verify Router is pinned first, every public website-pricing model appears, model detail/history pages load, new subscription creation returns unavailable, and stale evidence renders `unknown` rather than operational.

Stop condition: public API and website smoke tests pass in every deployed locale with no private fields present.

### 5. Enable notifications

Before setting `STATUS_CENTER_NOTIFICATIONS_ENABLED=true` and exposing the subscription form:

- verify SMTP delivery and the 24-hour email verification flow;
- configure a valid keyring before Webhook/Discord use;
- complete the Webhook challenge and SSRF checks;
- complete the Discord configuration and test delivery;
- confirm retry, dead-letter, and suspended-destination views in the admin workspace.

Stop condition: email, Discord, and generic Webhook paths pass their own validation, and the outbox drains without duplicate logical deliveries.

## Prometheus metrics

Scrape `GET /metrics` with the existing authentication policy. Select the `newapi-console` service for `newapi_status_center_*`; Router/slave intentionally omits those series. `newapi_status_center_metrics_up=0` means the Console status-metrics database collection failed; the endpoint still returns the existing relay/performance metrics.

| Metric | Meaning / alert hint |
| --- | --- |
| `newapi_status_center_feature_enabled{feature=...}` | Effective scheduler, public, notifications, and shadow lifecycle state. |
| `newapi_status_center_scheduler_lease_active` | `1` when the scheduler lease is live. Alert when scheduler is enabled but this remains `0`. |
| `newapi_status_center_scheduler_lease_remaining_seconds` | Lease lifetime without exposing the holder identity. |
| `newapi_status_center_component_inventory_ready` | `1` after at least one active component exists. |
| `newapi_status_center_evaluator_lag_seconds` | Maximum evaluation age. Alert before the 20-minute evidence boundary is reached. |
| `newapi_status_center_probe_queue_depth` | Components selected by the same traffic/conflict/budget rules as the scheduler and currently due for probing. |
| `newapi_status_center_probe_results{result=...}` | Success, failure, and monitoring-fault counts from the most recent one-hour window. |
| `newapi_status_center_probe_requests` | Probe count from the most recent one-hour window for capacity/cost estimation. |
| `newapi_status_center_probe_duration_seconds` | Average probe duration within the most recent one-hour window. |
| `newapi_status_center_unknown_models` | Active public models whose effective status is unknown. |
| `newapi_status_center_coverage_components{coverage=...}` | Active component counts in zero, partial, and full coverage buckets. |
| `newapi_status_center_rollup_ready{granularity=...}` | `1` only when every active component has an hour/day rollup. |
| `newapi_status_center_rollup_lag_seconds{granularity=...}` | Age of the oldest per-component latest rollup. Any missing active-component rollup produces a deliberately large, alertable lag. |
| `newapi_status_center_incident_drafts` | Unpublished incident or maintenance drafts awaiting review. |
| `newapi_status_center_outbox_depth{status=...}` | Active pending and processing delivery rows only. Delivered history is intentionally excluded. |
| `newapi_status_center_outbox_dead` | Dead delivery rows requiring operator review. |
| `newapi_status_center_outbox_oldest_age_seconds` | Age of the oldest pending/processing delivery. |
| `newapi_status_center_outbox_retry_ratio` | Fraction of the active pending/processing queue with at least one retry. |
| `newapi_status_center_suspended_destinations` | Notification destinations suspended after permanent failures. |
| `newapi_status_center_keyring_healthy` | `1` only when the configured keyring parses and has a valid active key. |

Recommended initial alerts:

- metrics collection down for two scrapes;
- enabled scheduler without an active lease for more than two minutes;
- evaluator lag above 10 minutes or any active component nearing 20 minutes without trustworthy evidence;
- growing probe queue for five minutes;
- missing/late hour or day rollups;
- pending outbox age above five minutes, dead rows above zero, or suspended destinations above zero;
- notifications enabled with an unhealthy keyring when Webhook/Discord is in use.

Tune thresholds after the seven-day shadow baseline; do not hide unknown states merely to reduce alerts.

## Key rotation

1. Generate a new 32-byte key, append it to the keyring value, and add that value as a new version of the configured Secret Manager secret. Keep the old active key selected and restart/roll out every Console instance. Confirm `newapi_status_center_keyring_healthy=1` everywhere.
2. Change `STATUS_SECRET_ACTIVE_KEY_ID` to the new key ID and roll out again. New encrypted values now use the new key; old envelopes continue to decrypt through the retained old key.
3. Re-save the Discord endpoint and re-register or rotate Webhook subscriptions so their stored envelopes are rewritten with the new active key. There is no automatic bulk re-encryption command.
4. Remove an old key only after inventory confirms no stored `v1.<old-key-id>.*` envelopes remain. Back up first, perform the check through an authorized database/admin procedure, and never copy envelope contents into tickets or logs.
5. After removal, roll out and verify keyring health plus one Discord/Webhook test.

If a key is lost before all envelopes are rotated, keep status reads/probes/email running, disable notifications, and reconfigure affected Webhook/Discord destinations. Do not delete encrypted records as a first response.

## Retention and history

- Raw probe results and five-minute periods: 7 days.
- Hour and day aggregates: 100 days.
- Public model history: up to 90 days from the time the new measurement definition was enabled.
- Unknown buckets do not enter the availability denominator; coverage remains visible separately.
- The system does not backfill pre-launch success rates. Before sufficient history exists, show not monitored/unknown evidence honestly.
- Incident, subscriber, delivery, settings, and audit rows are not deleted by the history-retention job. Apply a separately reviewed organizational retention policy; do not add ad-hoc production deletes to this rollout.

## Rollback

Use the narrowest reversible switch first:

1. Notification incident: set `STATUS_CENTER_NOTIFICATIONS_ENABLED=false`. Existing outbox rows remain for inspection and later draining.
2. Public-data or presentation incident: set `STATUS_CENTER_PUBLIC_ENABLED=false`. The website must show monitoring unavailable/unknown, never retain a reassuring green state.
3. Evaluator/probe incident: set Console `STATUS_CENTER_ENABLED=false`. This stops new evaluation, rollups, and retention; existing history remains read-only. Set Router `STATUS_CENTER_ENABLED=false` as well only when relay availability collection itself must stop.
4. Full status-center rollback: set public, notifications, scheduler, and shadow to `false`. Keep additive tables and evidence for diagnosis.

Do not drop status tables during an application rollback. If public remains enabled while the scheduler is paused, the API may serve the last snapshot only until freshness checks turn it unknown; this is intentional.

## Troubleshooting

- Scheduler does not start: confirm this is the master node, `STATUS_CENTER_ENABLED=true`, and `ROUTER_ORIGIN` is a bare absolute `http(s)` origin. A path such as `/v1`, credentials, query, or fragment is rejected.
- Lease holder is needed: search the Console internal application log for the process's first `status center scheduler lease acquired` entry. Holder identity is deliberately absent from Prometheus.
- Public API returns `503`: public is disabled or shadow mode is active. Check effective feature metrics, not only raw deployment values.
- Models remain unknown: confirm Router and Console both have `STATUS_CENTER_ENABLED=true`, then inspect evaluator lag, probe queue/results, coverage buckets, Router reachability, traffic eligibility, and monitoring faults. Do not force green to compensate for missing evidence.
- Metrics show `metrics_up=0`: check database connectivity and migration presence. Status metrics failure intentionally does not remove the pre-existing relay metrics from the scrape.
- Keyring health is `0`: configure both key variables together, ensure every decoded key is exactly 32 bytes, and ensure the active ID exists in the key list. Email remains available; encrypted Webhook/Discord does not.
- Outbox is growing: inspect pending age, retry ratio, dead rows, destination suspension, SMTP, and Webhook/Discord test results. The worker may be enabled independently to drain the queue while the scheduler is paused.

Record rollout timestamps, flag values, alert thresholds, key IDs (never key material), validation evidence, and rollback decisions in the change record.
