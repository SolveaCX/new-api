# Susciyuan Virtual-Character Real-Person Provider Design

## Goal

Make the susciyuan (涑元智算 SusToken) `virtual_character` Doubao-video channel (production channel 272) usable as a **real-person** material source in flatkey (new-api), and let it participate in **automatic group-based channel routing** alongside native BytePlus, not only as an admin-pinned `-<channelID>` channel.

Today channel 272 is unusable for real-person material for two independent reasons:

1. `realPersonProviderForChannel` (`service/real_person_provider.go`) accepts an explicit `asset_materialization.provider` only when it equals `tokenspace_material`; `virtual_character` is rejected with `real person provider unavailable`.
2. `realPersonChannelIsAutomaticCandidate` admits only native BytePlus (type 107) into the weighted-random automatic pool; any channel with an explicit provider config is excluded.

The generalized `virtual_character` materializer already merged (PR #1144) handles **ordinary** reusable assets only. It never enters the real-person verification/asset lifecycle. This design adds the missing real-person half.

## Non-goals

- No database schema change. No new column, no new table, no `ALTER` on the large `byte_plus_assets` / `abilities` / `users` tables.
- No change to the real-person state machine bodies (`createBytePlusVisualValidation`, `SyncBytePlusRealPersonVerification`, `expireBytePlusRealPersonCurrentSessionIfDue`) or the three background jobs — they already operate through the `binding.Provider` abstraction.
- No end-to-end real-person face-scan acceptance in this deliverable (requires a live human at the H5 flow and burns generation cost). Acceptance stops at unit tests + the real "create session → query session → query character → cancel" pre-live path, which needs no human.
- No production deployment. Development happens on a worktree branch off `origin/main`; PR to `SolveaCX/new-api`; cherry-pick to staging for verification only.

## Upstream contract (verified 2026-09-10 against susciyuan.com)

Susciyuan is a new-api-family instance. Auth is `Authorization: Bearer <API_KEY>`; the character belongs to that key's user. The real-person flow is **session + character_id + human H5 liveness**, distinct from BytePlus's `BytedToken`/`GroupId` model and from TokenSpace's Action+JSON protocol.

- **Create session**: `POST /v1/virtual-characters/validation-sessions` `{name, description?, tags?, language: zh|en}` → `data.id` (session hash), `data.character_id` (integer), `data.launch_url` (H5, returned **only at creation**), `data.expires_at` (~30 min), `data.status = pending`.
- **Query session**: `GET /v1/virtual-characters/validation-sessions/{id}` → `data.status ∈ {pending, succeeded, failed, expired, cancelled}`, `data.character_id`, `data.last_error`. `launch_url` may be absent on query.
- **Cancel session**: `DELETE /v1/virtual-characters/validation-sessions/{id}` → `data.cancelled`.
- **Character detail**: `GET /v1/virtual-characters/{id}` where `{id}` is the integer character id. Ready-to-generate is four conditions together: `status = active` + `validation_status = accepted` + `authorization.status = active` + `provider_asset_id` non-empty.
- **Portrait upload**: `POST /v1/virtual-characters/{id}/asset` multipart `file`, once; then `POST /v1/virtual-characters/{id}/sync`.
- **Generation reference**: `character_id` (integer) **or** `content[].image_url.url = asset://<provider_asset_id>`.
- **Error codes**: `real_person_disabled`, `character_limit_reached`, `validation_required`, `asset_already_uploaded`, `authorization_invalid`.
- **Two known upstream quirks** (from the probe suite): non-ASCII `name`/`description` are stored as U+FFFD by the upstream, so integration must send ASCII-only name/description; `provider_asset_id` only appears after a human completes H5 verification and portrait upload.

Account `User_090402` has sufficient real-person permission (`real_person_limit=5`, `real_person_used=0`); session creation returns 200 + `launch_url`; the pre-live path (create → query → cancel) works and restores quota. Test suite: `E:\workspace\susciyuan-realperson-test\`.

## The central constraint: two IDs, one legacy field

A ready real-person character has **two distinct upstream identifiers**:

- the **management id** (integer, `data.id` of the character, equal to `character_id`) — used to poll status (`GET /v1/virtual-characters/{id}`) and to delete;
- the **`provider_asset_id`** (opaque string, appears only after the human flow completes) — the value that must go on the wire as `asset://<provider_asset_id>` for generation.

The generalized materializer (PR #1144) already stores both, because `model.AssetBinding` has **two** upstream fields (`UpstreamAssetId` + `UpstreamGroupId`) and its rewrite goes through the scope-aware `assetBindingRewriteURIForScope`, whose `virtual-character:v1:` branch keeps the management id in `UpstreamAssetId` and returns `asset://` + `UpstreamGroupId` (the provider_asset_id).

The legacy real-person material table `model.BytePlusAsset` has **only one** upstream field, `UpstreamAssetId`. Worse, the legacy real-person rewrite in `ResolveLegacyBytePlusAssetBindingReferences` (`service/asset_reference.go`, the path the distributor uses for video-submit) is hardcoded to emit `"asset://" + asset.UpstreamAssetId` and never consults scope or a second field. Legacy real-person assets get a synthesized binding with an empty `BindingScope` and no `UpstreamGroupID` (`asset_reference.go:465-481`).

So on the legacy real-person path, a single field cannot satisfy two contradictory needs (poll/delete want the management id; the wire wants provider_asset_id). Adding a second column to `byte_plus_assets` is ruled out by the no-schema-change non-goal (and by the 148M-row `ALTER` risk recorded in prod incident history).

### Chosen resolution (Approach A, refined)

Route susciyuan real-person **assets** through the **generalized `AssetBinding` pipeline** (which already has the two fields and the scope-aware rewrite), instead of forcing a second id into the legacy `BytePlusAsset` table. Concretely:

- Real-person **verification** (session/profile lifecycle) stays on the existing `binding.Provider` real-person state machine — susciyuan implements the 8-method `realPersonProvider` interface. This part maps cleanly (session id ↔ BytedToken, launch_url ↔ H5Link, character_id ↔ GroupID) and needs no schema.
- Real-person **material** (the ready character used for `asset://`) is represented as a `virtual-character:v1:` scoped `AssetBinding` row: `UpstreamAssetId` = management id (poll/delete), `UpstreamGroupId` = provider_asset_id (wire). The existing `assetBindingRewriteURIForScope` then emits the correct `asset://<provider_asset_id>` with no change to that function, and status polling/delete use the management id.

This is preferred over Approach B (store `provider_asset_id` in the single legacy `UpstreamAssetId` and have `GetAsset`/`DeleteAsset` operate by `provider_asset_id`) because susciyuan's query/delete endpoints are keyed by the integer management id, not by `provider_asset_id` — Approach B depends on an upstream capability that was never verified and that the observed character schema contradicts.

### Phasing: what ships now vs. what is deferred (resolved)

The open implementation detail — where the real-person → generalized-binding bridge is anchored — resolves to a **phase boundary**, because the two legs of the flow have unequal buildability under the chosen acceptance ("unit tests + the H5 pre-live path"):

**Phase A (this plan, fully verifiable now):** the verification/routing leg plus a complete provider.
- The `virtualCharacterRealPersonProvider` implements all 8 `realPersonProvider` methods with real susciyuan REST calls. `CreateAsset`/`GetAsset`/`DeleteAsset` are keyed by the integer **management id**, which is the correct contract for the provider's own poll/delete responsibilities and for the `BytePlusAsset` single upstream field.
- susciyuan enters automatic group routing; sessions are created, polled to `character_id`, and the profile lifecycle runs on the existing state machine.
- Verified by unit tests + the live `create session → query session → query character → cancel` path. No human, no generation cost.

**Phase B (deferred, its own follow-up):** the material → `asset://<provider_asset_id>` wire bridge.
- The legacy real-person rewrite `ResolveLegacyBytePlusAssetBindingReferences` emits `"asset://" + asset.UpstreamAssetId` from the single-column `BytePlusAsset`. Susciyuan's wire requires `asset://<provider_asset_id>`, a **second** upstream value distinct from the management id used for poll/delete. Representing both requires the two-field scoped `AssetBinding` path, i.e. registering the ready character as a `virtual-character:v1:` scoped binding (`UpstreamAssetId`=management id, `UpstreamGroupId`=provider_asset_id) and teaching the video-submit resolver to prefer that scoped binding over the synthesized single-field legacy binding.
- This is deferred for two independent reasons: (1) its payoff (`provider_asset_id`) appears **only after a live human completes H5 verification + portrait upload**, which the agreed acceptance explicitly excludes — the round-trip cannot be verified now without burning a real human flow; (2) bridging the `BytePlus`-table-wired real-person asset subsystem to the generalized `AssetBinding` pipeline is a separate design with dual-write correctness hazards that conflicts with the "no state-machine-body change" non-goal, and deserves its own spec.
- **Safety of shipping Phase A without Phase B:** the channel is enabled in **staging only** (per Deployment) and acceptance is the pre-live path, so no production user reaches a wire that would emit `asset://<management_id>`. The PR description must state that production real-person **generation** is not complete until Phase B lands.

The plan below is Phase A only.

## Provider implementation

New file `service/virtual_character_real_person.go`, modeled on `service/tokenspace_real_person.go`:

```go
type virtualCharacterRealPersonProvider struct {
    channel       *model.Channel
    apiKey        string
    gatewayOrigin string
}
```

Implements all 8 `realPersonProvider` methods:

- `RequiresCallback() bool` → `false` (polling, no HTTPS callback).
- `VerificationTTLSeconds() int64` → new constant `virtualCharacterRealPersonSessionTTLSeconds` (~30 min, matching susciyuan session TTL).
- `CreateVisualValidateSession(ctx, callbackURL)` → `POST /validation-sessions`; encode `session_id` into `BytedToken` (stored encrypted by the state machine), `launch_url` into `H5Link`, upstream request id into `RequestID`.
- `GetVisualValidateResult(ctx, bytedToken)` → `GET /validation-sessions/{session_id}`. `succeeded` → `GroupID = character_id`. `pending` → **return a processing-class error** (drives the state machine's 60s backoff) — never return a fake or empty-yet-nil GroupID that would be treated as success. `failed/expired/cancelled` → definitive terminal error.
- `CreateAsset` / `GetAsset` / `DeleteAsset` → susciyuan REST calls; status maps to `model.BytePlusAssetStatusActive/Processing/Failed` (copy the mapping shape from `tokenspace_real_person.go`).
- `ListAssets` → minimal implementation; there is no production caller (`ListBytePlusRealPersonAssets` reads the local table).

HTTP transport reuses the **existing** virtual_character helpers in the same package — `virtualCharacterAssetHTTPClientFactory`, `readVirtualCharacterResponse`, `virtualCharacterHTTPFailure`, `virtualCharacterProcessingFailure`, `virtualCharacterDefinitiveFailure`, plus the error base `assetMaterializeClassForHTTPStatus` / `parseAssetMaterializeRetryAfter` / `newAssetMaterializeFailure`. It does **not** reuse `tokenSpaceMaterialDo` (different protocol). A small `virtualCharacterRealPersonDo` wraps request building/redirect-blocking, shaped like `tokenSpaceMaterialDo` but REST.

## Routing: susciyuan into automatic group routing

Per the confirmed decision, susciyuan real-person channels enter the automatic weighted-random pool, not only admin-pinned selection.

1. `service/real_person_provider.go` `realPersonProviderForChannel` — the `explicit` branch changes from an equality check to a `switch config.Provider`: `tokenspace_material` (unchanged), **new** `virtual_character` (returns the new provider binding; keeps the "exactly one enabled key" constraint via `enabledAssetMaterializeKeys`), `default` → unavailable.
2. `service/real_person_provider.go` `realPersonChannelIsAutomaticCandidate` — change from "native BytePlus only" to "native BytePlus (existing conditions) **or** `realPersonProviderForChannel(channel)` succeeds". Update the test `TestRealPersonAutomaticCandidateExcludesTokenSpace` that pins the old exclusion.
3. `service/byteplus_real_person.go` `validateRealPersonCreateCallbackRequirement` — today it unconditionally requires a callback base URL when `specificChannelID <= 0`. Since automatic routing may now select a no-callback provider, relax it to validate the callback requirement against the **selected** binding, not before selection.
4. Guard `selectBytePlusRealPersonChannel` / `loadUsableBytePlusRealPersonChannel` (and any wrapper that errors with "channel does not use native BytePlus credentials" when `StorageCredentials == nil`) — confirm no auto-selected non-native channel reaches those native-only paths.

### Multi-node safety (AGENTS.md Rule 11)

- Automatic routing applies **only at "create a new real-person verification session."** Once a profile exists it is pinned to `profile.ChannelId`; all later upload/poll/reference resolve that channel by id from the DB. Same-group mixing of susciyuan + native BytePlus is therefore safe — no second random draw.
- PR #1067 already established that automatic selection, after picking a candidate from the process-local channel cache, reloads the channel from the DB via `loadUsableRealPersonProviderBinding(channel.Id, group)` before constructing the provider. The new provider inherits this: any channel-config read goes through `loadUsableRealPersonProviderBinding` (`GetChannelById(id, true)` + ability check), never the cached `*Channel`.
- Real-person concurrency control is **not** AssetBinding CAS (that path is the generalized materializer's). It is the session lease claim (`ClaimBytePlusVisualValidationSession`) plus terminal profile transitions that tolerate `model.ErrAPIIdempotencyCASLost` (lost optimistic lock = another replica already handled it). The provider must not fabricate a GroupID; terminal writes are idempotent, so the provider need not reason about concurrency.

## Channel-type gating (distributor / relay)

DoubaoVideo (type 54) real-person video-submit requests enter the real-person link via `middleware/distributor.go` `selectBytePlusPinnedAssetChannel` + `TokenSpaceRealPersonChannelIsUsable`, triggered when `relay_mode == RelayModeVideoSubmit` and the resolved reference set has `HasRealPersonReference`.

- **First priority**: generalize `TokenSpaceRealPersonChannelIsUsable` (rename to `ExplicitRealPersonChannelIsUsable`, or widen it to accept any DoubaoVideo channel whose `realPersonProviderForChannel` succeeds). Update its call sites `service/asset_reference.go` `legacyRealPersonAssetCanUseChannel` and `middleware/distributor.go` `selectBytePlusPinnedAssetChannel`. Without this, the pinned-channel check `channel.Type != ChannelTypeBytePlus && !tokenSpaceRealPersonChannel` returns 503.
- `service/byteplus_real_person_asset.go` `realPersonAssetMultipartStore` — the type assertion that currently allowlists only `tokenSpaceRealPersonProvider` for GCS temp storage when `StorageCredentials == nil` must also admit the new provider, or multipart portrait upload 503s.
- **Deferred / defensive only**: the three `channel.Type != constant.ChannelTypeBytePlus` hardcodes in `controller/relay.go` (`lockBytePlusAssetPinnedChannel`, `validateBytePlusAssetPinnedLock`) are dormant — they activate only on the `ResolveBytePlusAssetReferences` path, which production video-submit does not take (it uses `ResolveLegacyBytePlusAssetBindingReferences`, which never sets `ContextKeyBytePlusAssetPinnedChannelID`). Widen them in the same spirit as PR #829 as a defensive follow-up, clearly marked as not on the hot path.

## Data-plane configuration (non-code)

Channel 272 needs, in staging first:

- `other_settings.asset_materialization = {"provider":"virtual_character","gateway_base_url":"https://susciyuan.com"}`;
- exactly one enabled key (the susciyuan API key);
- an `abilities(group, model='seedance-2.0', channel_id=272, enabled=1)` row for each group that should route to it;
- `status = enabled` (it is currently disabled, status=2).

## Testing and acceptance

- **Unit / protocol tests** (`service/virtual_character_real_person_test.go`, new): copy the recipe from `real_person_provider_test.go` (package-var client-factory replacement + `channelWithAssetMaterializationSettings`) and `tokenspace_real_person_test.go` (`httptest.NewTLSServer` + `withVirtualCharacterTestClients` + `newBytePlusRealPersonServiceTestDB` / `installBytePlusRealPersonServiceTestDeps`). Add `insertVirtualCharacterRealPersonChannel` / `virtualCharacterRealPersonTestBinding` helpers. **Must include an assertion that error strings never leak the API key.** Cover: provider dispatch (explicit `virtual_character` → new binding), pending→60s-backoff (GroupID-not-ready returns a processing error), succeeded→activate, automatic-candidate now admits the channel, and the scoped-binding rewrite emits `asset://<provider_asset_id>`.
- **Live pre-verification path** (the `E:\workspace\susciyuan-realperson-test\` suite): re-run `create session → query session → query character → cancel` against the real upstream. No human, no generation cost.
- **Regression**: keep `TestBytePlusRealPersonAutomaticSelectionRefreshesCachedChannelBeforeBinding` green; revise `TestRealPersonAutomaticCandidateExcludesTokenSpace` to match the new admission rule.
- **i18n**: any new user-facing error message gets all 8 languages.

## Deployment

- Develop on worktree `E:\workspace\new-api-worktrees\susciyuan-real-person`, branch `feat/susciyuan-real-person-20260911`, off `origin/main`.
- PR to `SolveaCX/new-api`. Do **not** push `main` or `staging` without explicit approval.
- Cherry-pick to staging for verification. Router deploy recommendation (AGENTS.md Rule 12): master node runs no migration for this change (no schema change); the only ordering requirement is that the new code is deployed before channel 272 is enabled in staging. No rollback data hazard because no table is written that old revisions cannot read.

## Files touched (summary)

New:
- `service/virtual_character_real_person.go`
- `service/virtual_character_real_person_test.go`

Modified (Phase A):
- `service/real_person_provider.go` (provider switch; automatic-candidate admission; generalize `TokenSpaceRealPersonChannelIsUsable`)
- `service/byteplus_real_person.go` (relax `validateRealPersonCreateCallbackRequirement`; confirm native-only wrappers)
- `service/byteplus_real_person_asset.go` (multipart temp-store allowlist)
- `service/asset_reference.go` (call-site of the generalized usability check only — `legacyRealPersonAssetCanUseChannel`)
- `service/real_person_provider_test.go` (revise the exclusion test)
- possibly `controller/relay.go` (defensive dormant-gate widening)
- i18n message files (8 languages) for any new error string

Deferred to Phase B (not in this plan):
- `service/asset_reference.go` real-person material → scoped-binding rewrite bridge (the `asset://<provider_asset_id>` wire leg)
