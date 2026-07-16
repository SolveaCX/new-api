# Native Status Center Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a native NewAPI status center that honestly reports Router and every public model's current health, 90-day history, incidents, maintenance, and email/Discord/webhook subscriptions.

**Architecture:** Extend `pkg/perf_metrics` with availability counters, then evaluate persisted traffic and synthetic probes into database-backed five-state components. A lease/fencing scheduler owns catalog sync, probes, rollups, retention, incident drafts, and outbox delivery across nodes; Gin exposes APIs, `website` renders public SSR pages, and `web/default` provides the admin workspace.

**Tech Stack:** Go 1.22, Gin, GORM v2, SQLite/MySQL/PostgreSQL, React 19, TanStack Router/Query, Rsbuild, Next.js 16, Bun, AES-256-GCM, HMAC-SHA256.

---

## File map

- `pkg/perf_metrics/availability.go`: final-request availability classifier.
- `pkg/perf_metrics/{types,metrics,flush}.go`, `model/perf_metric.go`: lock-free collection and persisted eligible/success counters.
- `model/status_center.go`: entities, constants, migrations, and public-safe projections.
- `model/status_center_store.go`: cross-database upserts, CAS leases/fencing, outbox claims, and transactions.
- `service/status_engine.go`: pure state transitions, hysteresis, banners, and availability/coverage math.
- `service/status_catalog.go`: Router plus public-pricing-model synchronization.
- `service/status_{scheduler,probe}.go`: leased evaluation, adaptive probes, rollups, retention, and delivery.
- `service/status_{incident,subscription,secret,webhook}.go`: publish workflows, maintenance, overrides, encryption, SSRF defense, subscribers, and outbox adapters.
- `controller/status_{public,admin}.go`, `router/api-router.go`: public/admin API surfaces.
- `web/default/src/features/status-center/*`: authenticated admin UI.
- `website/src/lib/status*.ts`, `website/src/components/status-*.tsx`, `website/src/app/**/status/**`: public status UI, 90-day model history, localization, and same-origin proxy.

### Task 1: Availability traffic signal

**Files:**
- Create: `pkg/perf_metrics/availability.go`
- Test: `pkg/perf_metrics/availability_test.go`
- Modify: `pkg/perf_metrics/types.go`, `pkg/perf_metrics/metrics.go`, `pkg/perf_metrics/flush.go`
- Modify: `model/perf_metric.go`, `model/main.go`

- [ ] **Step 1: Write failing tests** covering final success, upstream 5xx/timeout/network/bad response/router exhaustion/channel auth as eligible, and client key/quota/invalid request/policy/cancellation as excluded. Assert one availability outcome per final relay request and persisted counts.
- [ ] **Step 2: Run RED:** `go test ./pkg/perf_metrics ./model -run 'TestAvailability|TestPerfMetricAvailability'` → FAIL because the enum/fields do not exist.
- [ ] **Step 3: Implement minimal code** with:

```go
type AvailabilityOutcome uint8
const (
	AvailabilityExcluded AvailabilityOutcome = iota
	AvailabilityEligibleFailure
	AvailabilityEligibleSuccess
)
func ClassifyAvailabilityOutcome(success bool, relayErr *types.NewAPIError) AvailabilityOutcome
```

Add atomic-bucket and `PerfMetric` integer counts; do not add synchronous DB/network work to `RecordRelaySample`.
- [ ] **Step 4: Run GREEN:** `go test ./pkg/perf_metrics ./model -run 'TestAvailability|TestPerfMetricAvailability|TestRecordRelay'` → PASS.
- [ ] **Step 5: Commit:** `measure platform availability from final relay outcomes`.

### Task 2: Status persistence and cross-database constraints

**Files:**
- Create: `model/status_center.go`, `model/status_center_store.go`
- Test: `model/status_center_test.go`
- Modify: `model/main.go`

- [ ] **Step 1: Write failing SQLite migration tests** for unique component key/slug, period `(component_id, granularity, period_start)`, incident transition key, normalized subscriber identity hash, outbox logical destination, optimistic versions, lease takeover, and fencing increments.
- [ ] **Step 2: Run RED:** `go test ./model -run 'TestStatus'` → FAIL because entities/store functions do not exist.
- [ ] **Step 3: Implement additive schema/store.** Use `varchar` states (`operational`, `degraded`, `outage`, `unknown`, `maintenance`), UTC Unix seconds, `TEXT` JSON via `common.Marshal`/`common.Unmarshal`, GORM transactions, portable unique indexes, `AcquireStatusJobLease`, `UpsertStatusPeriod`, `ClaimStatusDeliveries`, and fencing-conditioned commits.
- [ ] **Step 4: Run GREEN:** `go test ./model -run 'TestStatus'` → PASS without dialect-specific SQL.
- [ ] **Step 5: Commit:** `persist status history with portable multi-node coordination`.

### Task 3: Pure state engine and 90-day math

**Files:**
- Create: `service/status_engine.go`
- Test: `service/status_engine_test.go`

- [ ] **Step 1: Write table-driven failing tests** for ≥20-request traffic thresholds 99.5/95, two/three probe failures, conflicts, monitoring faults, 20-minute evidence expiry, three-probe/two-99.9%-bucket recovery, maintenance/override precedence, banner priority, and exact micro-score availability/coverage.
- [ ] **Step 2: Run RED:** `go test ./service -run 'TestStatusEngine|TestStatusAvailability'` → FAIL.
- [ ] **Step 3: Implement pure interfaces:**

```go
type StatusEvidence struct { Eligible, Success, ProbeSuccess, ProbeFailure int64; LastTrustworthyAt int64; MonitoringFault bool }
type StatusTransition struct { Observed, Effective, Source string; ScoreMicros *int64; TrustworthyAt int64 }
func EvaluateStatus(previous model.StatusComponent, evidence StatusEvidence, now int64) StatusTransition
func AggregateStatusPeriods(periods []model.StatusPeriod) StatusAvailability
func OverallStatus(components []model.StatusComponent) string
```

- [ ] **Step 4: Run GREEN:** `go test ./service -run 'TestStatusEngine|TestStatusAvailability'` → PASS.
- [ ] **Step 5: Commit:** `make degraded recovery and unknown coverage explicit`.

### Task 4: Catalog, probes, leases, evaluation, and rollups

**Files:**
- Create: `service/status_catalog.go`, `service/status_probe.go`, `service/status_scheduler.go`
- Test: `service/status_catalog_test.go`, `service/status_probe_test.go`, `service/status_scheduler_test.go`
- Modify: `controller/model_availability_task.go`, `main.go`

- [ ] **Step 1: Write failing tests** asserting one Router plus exactly website-pricing-visible models, retirement, 15-minute idle probes, one-minute Router/fault/conflict probes, high-traffic probe skips, monitoring-fault semantics, stale-fence rejection, idempotent rollups, and retention.
- [ ] **Step 2: Run RED:** `go test ./service -run 'TestStatusCatalog|TestStatusProbe|TestStatusScheduler'` → FAIL.
- [ ] **Step 3: Extract the existing minimal-model probe behind an injectable service adapter** and add `StartStatusCenterTasks()` on master. Correctness-sensitive jobs acquire DB leases, carry fencing tokens into writes, and use unique upserts.
- [ ] **Step 4: Run GREEN/race:** `go test -race ./service -run 'TestStatusCatalog|TestStatusProbe|TestStatusScheduler'` → PASS with competing workers producing one logical result.
- [ ] **Step 5: Commit:** `evaluate every public component with fenced adaptive probes`.

### Task 5: Incidents, maintenance, overrides, and audit

**Files:**
- Create: `service/status_incident.go`
- Test: `service/status_incident_test.go`
- Modify: `model/status_center_store.go`

- [ ] **Step 1: Write failing tests** for automatic private drafts, recovery suggestions, append-only published updates, transactionally coupled outbox rows, maintenance start/end fallback, expiring overrides, root-only force-green ≤1 hour, optimistic conflicts, and audit before/after values.
- [ ] **Step 2: Run RED:** `go test ./service -run 'TestStatusIncident|TestStatusMaintenance|TestStatusOverride'` → FAIL.
- [ ] **Step 3: Implement transactional workflows.** Automation edits drafts only; each publish creates a new immutable update and idempotent delivery rows in one transaction.
- [ ] **Step 4: Run GREEN:** same command → PASS.
- [ ] **Step 5: Commit:** `keep public incident communication deliberate and auditable`.

### Task 6: Secret keyring, webhook SSRF defense, subscriptions, and outbox delivery

**Files:**
- Create/Test: `service/status_secret.go`, `service/status_secret_test.go`
- Create/Test: `service/status_webhook.go`, `service/status_webhook_test.go`
- Create/Test: `service/status_subscription.go`, `service/status_subscription_test.go`

- [ ] **Step 1: Write failing security tests** for AES-GCM active/old keys and tamper rejection, token hashes, HMAC vectors, HTTPS/public-IP enforcement, loopback/link-local/private/metadata IPv4/IPv6, dial-time DNS rebinding, redirect/port/size/time bounds, generic subscription responses, 24-hour email verification, POST-only unsubscribe mutation, webhook challenge activation, and retry classification.
- [ ] **Step 2: Run RED:** `go test ./service -run 'TestStatusSecret|TestStatusWebhook|TestStatusSubscription|TestStatusDelivery'` → FAIL.
- [ ] **Step 3: Implement bounded surfaces.** Parse `STATUS_SECRET_KEYS` as `key_id:base64-32-byte-key`, write with `STATUS_SECRET_ACTIVE_KEY_ID`, validate resolved IP again in a custom dialer, reuse SMTP, and support one encrypted root-configured Discord webhook.
- [ ] **Step 4: Run GREEN/race:** `go test -race ./service -run 'TestStatusSecret|TestStatusWebhook|TestStatusSubscription|TestStatusDelivery'` → PASS.
- [ ] **Step 5: Commit:** `deliver status notifications without exposing internal networks or secrets`.

### Task 7: Public and admin HTTP APIs

**Files:**
- Create/Test: `controller/status_public.go`, `controller/status_public_test.go`
- Create/Test: `controller/status_admin.go`, `controller/status_admin_test.go`
- Modify: `router/api-router.go`

- [ ] **Step 1: Write failing tests** for summary/components/detail/history/incidents/maintenance, range validation, ETag/304, short Cache-Control, public field allowlists, stale/unknown, subscription body limits/generic replies, Admin/Root/SecureVerification route placement, and 409 versions.
- [ ] **Step 2: Run RED:** `go test ./controller ./router -run 'TestStatus'` → FAIL.
- [ ] **Step 3: Add thin handlers/routes.** Controllers bind/validate/call services and use common response helpers. Public DTOs include `generated_at`, `last_trustworthy_update_at`, and coverage; exclude channel/provider/raw error/secret/traffic/customer data.
- [ ] **Step 4: Run GREEN:** same command → PASS.
- [ ] **Step 5: Commit:** `expose honest public health and guarded admin workflows`.

### Task 8: Console Status Center admin workspace

**Files:**
- Create: `web/default/src/features/status-center/{types,api,index}.ts*`
- Test: `web/default/src/features/status-center/status-center.test.ts`
- Create: `web/default/src/routes/_authenticated/status-center/index.tsx`
- Modify: sidebar data and `web/default/src/i18n/locales/{en,zh,es,fr,pt,ru,ja,vi}.json`
- Regenerate: `web/default/src/routeTree.gen.ts`

- [ ] **Step 1: Write failing tests** for status labels, override expiry/force-green validation, immutable update rendering, 409 reload messaging, and permission-based controls.
- [ ] **Step 2: Run RED:** `bun test src/features/status-center/status-center.test.ts` → FAIL.
- [ ] **Step 3: Implement Overview, Incidents, Maintenance, Subscribers, Deliveries, Settings, and Audit tabs** using existing query/Base UI/Tailwind patterns. Route every visible string through `t()` with real translations in eight locale files and never re-render saved secrets.
- [ ] **Step 4: Run GREEN/checks:** `bun test ...`, `bun run i18n:sync`, `bun run typecheck` → exit 0 with no new untranslated keys.
- [ ] **Step 5: Commit:** `give administrators one guarded status operations workspace`.

### Task 9: Website same-origin proxy and resilient client

**Files:**
- Create/Test: `website/src/app/api/status/[...path]/route.ts`, `route.test.ts`
- Create/Test: `website/src/lib/status.ts`, `status.test.ts`

- [ ] **Step 1: Write failing tests** for allowlisted public paths/subscription POST, query/body/status/cache forwarding through `APP_CONSOLE_ORIGIN`, no cookie/auth forwarding, 502 fallback, 60-second revalidation, and stale/unknown without fabricated green.
- [ ] **Step 2: Run RED:** `bun test src/app/api/status/[...path]/route.test.ts src/lib/status.test.ts` → FAIL.
- [ ] **Step 3: Implement allowlisted proxy/fetchers** rejecting admin paths and unknown methods.
- [ ] **Step 4: Run GREEN:** same command → PASS.
- [ ] **Step 5: Commit:** `keep public status data same-origin and fail visibly stale`.

### Task 10: Public overview and model 90-day history

**Files:**
- Create/Test: `website/src/lib/status-copy.ts`, `status-copy.test.ts`
- Create/Test: `website/src/components/status-history-bars.tsx`, `status-page.tsx`, `status-page.test.tsx`, `status-model-page.tsx`, `status-model-page.test.tsx`, `status-subscribe.tsx`
- Create: symmetric `website/src/app/(en)/status/**` and `website/src/app/[locale]/status/**` routes
- Modify: `website/src/app/sitemap.ts`

- [ ] **Step 1: Write failing tests** for pinned Router, all model rows, filters, text+icon+color states, unknown/maintenance/retired/stale, 90 daily bars and ranges, availability/coverage/incidents, related timeline, accessible subscription form, route symmetry, and all nine translations.
- [ ] **Step 2: Run RED:** `bun test src/lib/status-copy.test.ts src/components/status-page.test.tsx src/components/status-model-page.test.tsx` → FAIL.
- [ ] **Step 3: Implement SSR-first pages** with `SiteShell`, `buildMetadata`, `localizePath`, semantic controls, visible focus, responsive layout, 60-second refresh, and textual/tooltipped history bars.
- [ ] **Step 4: Run GREEN/checks:** targeted tests, `bun run lint`, `bun run typecheck` → exit 0.
- [ ] **Step 5: Commit:** `show router and every model with transparent 90-day evidence`.

### Task 11: Lifecycle, observability, flags, and operations docs

**Files:**
- Modify: `main.go`, `controller/prometheus_metrics.go`, `.env.example`, `README.md`
- Create: `docs/status-center-operations.md`

- [ ] **Step 1: Write failing tests** for independent scheduler/public/notification/shadow flags and secret-free lease/evaluator/probe/unknown/rollup/draft/outbox/keyring metrics.
- [ ] **Step 2: Run RED:** `go test ./controller ./service -run 'TestStatusFeatureFlags|TestStatusMetrics'` → FAIL.
- [ ] **Step 3: Wire/document** `STATUS_CENTER_ENABLED`, `STATUS_CENTER_PUBLIC_ENABLED`, `STATUS_CENTER_NOTIFICATIONS_ENABLED`, `STATUS_CENTER_SHADOW_MODE`, keyring values, Router canary config, staged rollout, retention, key rotation, and rollback. Missing optional notification secrets must not stop status reads/probes/email.
- [ ] **Step 4: Run GREEN:** same command → PASS.
- [ ] **Step 5: Commit:** `make status rollout observable reversible and key-rotation ready`.

### Task 12: Full verification and scope review

- [ ] **Step 1: Backend:** `go test ./pkg/perf_metrics ./model ./service ./controller ./router -run 'TestStatus|TestAvailability'` and `go test -race ./pkg/perf_metrics ./model ./service -run 'TestStatus|TestAvailability'` → PASS.
- [ ] **Step 2: Console:** from `web/default`, run targeted tests, `bun run i18n:sync`, `bun run typecheck`, `bun run lint`, `bun run build:check` → PASS.
- [ ] **Step 3: Website:** from `website`, run `bun test`, `bun run lint`, `bun run typecheck`, `bun run build` → PASS.
- [ ] **Step 4: Security/scope review:** search for direct JSON marshal/unmarshal, exposed secrets/internal identities, unsafe URL clients, process-local correctness, database-specific SQL, untranslated keys, unbounded bodies; run `git diff --check`, `git status --short`, and `git diff --stat origin/main...HEAD`.
- [ ] **Step 5: Final commit:** `prove the native status center across backend and both frontends`.

## Self-review

- Spec coverage: Tasks 1-4 cover hybrid signals, Router/every-public-model inventory, hysteresis, leases/fencing, 90-day history, retention, and multi-node behavior. Tasks 5-7 cover incidents, maintenance, overrides, audit, encryption, SSRF, subscriptions, outbox, permissions, and APIs. Tasks 8-10 cover both frontends and all locales. Tasks 11-12 cover flags, observability, rollout, privacy, performance, and verification.
- Placeholder scan: every task names files, failing behavior, commands, implementation interfaces, and expected results; no deferred or incomplete instruction remains.
- Type consistency: public states, granularity, micro-score sums, versions, event IDs, fencing tokens, and feature-flag names match the approved design.
