# BytePlus Credential-Scoped Reusable Assets Design

## Goal

Make reusable asset uploads reliable on BytePlus channel 120 after credential or project changes, without allowing concurrent credentials, old data, or mixed Cloud Run revisions to corrupt each other's upstream asset groups.

## Evidence and root cause

The production smoke test stored the uploaded source successfully, but channel 120 failed materialization after six attempts. The upstream response was HTTP 404 with `NotFound.group_id`. The current group cache is keyed only by user and channel, while a BytePlus group belongs to the credential's AccessKey ID and ProjectName. A request using a newer credential therefore reuses a group created under an older credential.

The existing worktree already adds exact `NotFound.group_id` recovery and credential-scoped `AssetBinding` identities, but static review found four remaining correctness gaps:

1. the upstream group itself is still shared across credentials;
2. normal group-creation contention can be classified as a permanent binding failure;
3. probing a legacy blank-scope binding with the wrong credential can destroy that binding;
4. legacy adoption can race a concurrent scoped binding insert.

## Chosen architecture

Add a new `BytePlusAssetBindingGroup` model backed by an add-only table named `byte_plus_asset_binding_groups`. Only the generalized reusable-asset materializer uses this table. The legacy BytePlus asset API and its `BytePlusAssetGroup` table remain unchanged.

Each new group row is uniquely identified by:

- user ID;
- channel ID;
- a non-secret binding scope: `byteplus:v1:` plus SHA-256 of normalized `AccessKeyID + NUL + ProjectName`.

The video API key and SecretAccessKey are deliberately excluded. Rotating either secret does not change upstream group ownership. AccessKey ID or ProjectName changes do create a distinct group.

This design is preferred over widening the existing table because it isolates the generalized reusable-asset materialization path during a multi-node Cloud Run rollout and rollback. Old revisions use the legacy table for their materialization behavior. New revisions use the new table only for generalized materialization, while their legacy public BytePlus asset API continues to read and write `byte_plus_asset_groups`. The old and new generalized-materialization paths therefore do not share group rows, but new revisions are not globally barred from the legacy table. The only deployment dependency is that the master migration creates the new table before router traffic reaches the new code.

## Data model and concurrency

`BytePlusAssetBindingGroup` carries the same lifecycle fields as the legacy group: upstream group/request IDs, status, lease timestamp, created time, and updated time. Its unique index is `(user_id, channel_id, binding_scope)`.

Claiming uses database uniqueness plus compare-and-swap conditions, not process-local locks:

- first claimant inserts a `Creating` row and owns the lease;
- later claimants read the exact scope row;
- a `Failed` row or stale `Creating` lease can be reclaimed only within the same scope;
- activate/fail operations require the row ID and expected lease timestamp;
- invalidation requires the row ID and expected upstream group ID.

These rules are safe across multiple router instances.

## Materialization flow

The generalized BytePlus materializer parses and validates the effective request credential, computes its binding scope once, and claims the matching `BytePlusAssetBindingGroup`.

When `CreateAsset` returns exact HTTP 404 code `NotFound.group_id`:

1. invalidate only that scoped active group using CAS;
2. claim or wait for a replacement in the same scope;
3. retry `CreateAsset` once with the replacement group;
4. return the second error without another rebuild if it also reports a missing group.

Other 404 codes must not rebuild a group.

If another node is creating the group and the short synchronous wait expires, the materializer returns `ErrAssetBindingInitializing`. Existing binding/readiness/task workers already treat that error as retryable, release their leases, and requeue within the existing preparation window. The provider layer must not return an HTTP-facing `NewAPIError` for this internal contention state.

## Legacy binding compatibility

Blank-scope `AssetBinding` rows have unknown credential ownership.

- A retryable lookup result keeps the request retryable.
- A definitive lookup failure or status/ID mismatch only skips adoption; it must not modify the legacy row.
- Only a verified Active lookup for the same upstream asset ID may adopt the row into the current credential scope.

Adoption runs in one transaction:

1. lock and validate the legacy source row;
2. insert a scoped copy with `ON CONFLICT DO NOTHING`;
3. load the exact scoped winner;
4. delete the legacy row only when the winner is Active and references the same upstream asset.

This avoids cross-database duplicate-key error handling and remains safe if another node creates the target binding concurrently.

## Security and observability

- Persist only the scope digest, never API keys, AccessKey IDs, SecretAccessKeys, or ProjectName.
- Preserve existing sanitized public errors.
- Keep upstream request IDs only in restricted/internal fields and logs.
- No API response schema changes are required.

## Testing

Automated tests must cover:

- same user/channel with different credential scopes creates independent groups;
- stale lease takeover is limited to the exact scope;
- exact `NotFound.group_id` invalidates, recreates, and retries once;
- a second missing-group response stops after one retry;
- other 404 codes do not rebuild;
- group contention returns `ErrAssetBindingInitializing` and does not mark binding/readiness Failed;
- secret/API-key rotation preserves scope; AccessKey ID or ProjectName changes scope;
- failed legacy probing leaves the blank-scope row Active while creating a new scoped binding;
- successful legacy adoption handles a concurrent target insert without a unique-key failure;
- the new model is registered in both normal and SQLite-fast migrations.

Run targeted model/service tests with Go test concurrency capped at one or two, followed by diff checks and the narrowest practical build verification.

## Deployment and production verification

1. Push a feature branch and merge through a reviewed PR into `main`.
2. Deploy and validate staging first.
3. Deploy the master/console revision and verify the new table and unique index exist.
4. Deploy router nodes and verify the serving revision.
5. Upload a fresh 1024×1024 PNG with the supplied token pinned to channel 120.
6. Wait until `seedance2.0-pro` is available for that asset.
7. Submit exactly one 480p, 16:9, 5-second asset-referenced video.
8. Verify the task reaches `completed`, download the MP4, and validate its container, dimensions, duration, frame decoding, size, and SHA-256.

Rollback remains safe because the old binaries ignore the add-only table. If production verification fails, route traffic back to the prior router revision; do not drop the new table during incident response.

## Non-goals

- No changes to the public asset or video API contract.
- No inference or backfill of legacy group credential ownership.
- No deletion of legacy upstream groups.
- No broad refactor of non-BytePlus materializers.
