# PLG Model Catalog Change DingTalk Notification Design

## Goal

Send a DingTalk group robot message whenever the set of models that a `plg` user sees on the Console "Available Models" page changes, so operators learn about accidental removals (missing ratios, disabled channels, failed probes) and new launches without polling the page by hand.

## Scope and definition of "available"

The monitored set is exactly what the Console page renders for a `plg` account:

1. `service.ResolveUserModelAccess` for a synthetic `UserBase{Group: "plg"}` with an empty setting string. The resolver takes the fixed-account branch, so only `abilities` rows for group `plg` on enabled channels with a visible billing configuration are returned.
2. The pricing hidden-model patterns are applied, the same filter the page requests with `view=available_models`.
3. Only models whose availability status is `available` or `unknown` are kept, mirroring the frontend `isAvailableCatalogModel` rule. Models with probe failures are hidden on the page and therefore excluded here.

The public website payload (`/api/website/pricing?group=plg`) is deliberately not the reference: it does not drop unpriced models or disabled channels, so it would miss the incidents this feature exists to catch.

Because the resolver already treats an empty group as `plg`, the task passes the literal `"plg"` group to avoid semantic drift if that default ever changes.

## Architecture

A new background task `StartPLGModelCatalogWatchTask` runs on master (Console) nodes only, registered in `main.go` next to the model availability detection task. Each cycle:

1. **Resolve** the catalog through the shared service helper `service.ResolvePLGCatalogModelNames()`, which performs the three steps above. The hidden-model filter previously lived in `controller/model_access.go`; it moves to `service.FilterHiddenModelsFromUserAccess` so the controller and the task share one implementation.
2. **Fingerprint** the sorted model names with SHA-256.
3. **Compare and claim** against the single snapshot row in `plg_model_catalog_snapshots`. A missing row is written as the baseline without notifying. A different fingerprint is claimed with a conditional update (`WHERE fingerprint = <previous>`); only the node whose update affects one row proceeds. Nodes that lose the race skip silently.
4. **Notify** with the added and removed model names through `service.SendDingTalkText` using the dedicated webhook. Send failures are logged and not retried: the snapshot has already advanced, so the next cycle does not resend, and the change is still visible in the system log.

## Configuration

New operation setting block `plg_catalog_notify_setting`, mirroring `payment_notify_setting`:

| key | type | default |
| --- | --- | --- |
| `dingtalk_alert_enabled` | bool | `false` |
| `dingtalk_alert_webhook_url` | string | `""` |
| `dingtalk_alert_secret` | string | `""` |
| `check_interval_minutes` | float64 | `5` (values below 1 are clamped to 1) |

The block is exposed through the existing config manager, whitelisted in `isBulkOptionUpdateKey`, and edited in a new "PLG Catalog Notifications" section on the Console operations settings page. The task keeps polling even while disabled so the baseline snapshot stays current; it only skips sending when the switch is off or the webhook is empty.

## Data model

`PLGModelCatalogSnapshot`, table `plg_model_catalog_snapshots`:

| column | type | note |
| --- | --- | --- |
| `group_name` | varchar(64) primary key | always `plg`; keyed so the table can serve other groups later |
| `fingerprint` | varchar(64) | hex SHA-256 of the sorted names |
| `model_names` | text | JSON array of sorted names, source of the diff |
| `model_count` | int | convenience for dashboards |
| `updated_at` | bigint | unix seconds |

Migration is add-only and uses portable column types, so it works on MySQL, PostgreSQL, and SQLite.

## Multi-node behavior

Console runs 1 to 5 Cloud Run instances, all `NODE_TYPE=master`. Correctness does not depend on process-local state:

- the baseline insert uses `ON CONFLICT DO NOTHING`; a losing insert simply reads the row next cycle;
- change detection uses a compare-and-swap update on `fingerprint`; exactly one node wins per change;
- a node that crashes after winning the CAS but before sending loses that one notification, which is accepted because the alternative (send before commit) risks duplicate messages.

Router (slave) nodes never start the task, so no router deployment is required for this change.

## Notification content

Plain text, keyed so DingTalk keyword security works with the word `PLG`:

```
PLG 可用模型变更
时间：2026-09-10 12:00:00 (UTC+8)
新增 2 个：gpt-6-astra, claude-opus-5
移除 1 个：apodex-1.1
当前共 86 个模型
```

Lists are capped at 20 names per direction with a "其余 N 个已省略" suffix to stay under DingTalk limits.

## Testing

- `service`: hidden-model filter moved without behavior change; `ResolvePLGCatalogModelNames` drops unpriced, hidden, probe-failed models and keeps `available`/`unknown`; `DiffModelNames` and content builder.
- `model`: baseline insert, CAS claim succeeds once and fails for stale fingerprints, sqlite in-memory.
- `service` task: end-to-end run with an httptest DingTalk server asserting one message on change, none on first run, none when unchanged, none when disabled.
- Frontend: schema/normalization utils (webhook required when enabled, http-only URL, interval clamp, blank secret never re-posted) plus a static render test of the settings section built on the shared `useSettingsForm` hook.
