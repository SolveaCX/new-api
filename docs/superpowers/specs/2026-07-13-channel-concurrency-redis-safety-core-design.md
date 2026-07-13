# Channel Concurrency Redis Safety Core Design

Date: 2026-07-13
Status: Approved design; written-spec review pending
Supersedes: `2026-06-17-sub2-style-channel-concurrency-design.md` for Redis failure policy and the phase-A implementation scope

## Decision

Harden the existing channel concurrency implementation for a production pool of 50 concurrency-limited channels before adding the rest of sub2's scheduler features.

This phase keeps Redis as the multi-node source of truth and adds five safety boundaries:

1. Atomic wait admission and atomic wait release.
2. Retryable slot release that does not discard the request lease before Redis confirms release.
3. Cooldown state and caching separated from concurrency-load reads.
4. One coalesced fresh-load reorder after all immediate acquire attempts fail.
5. A sub2-style acquire fan-out limit of seven candidates per pass so a saturated 50-channel pool cannot amplify every request into 50 serialized Redis acquire attempts.

The fifth item is a necessary refinement to the originally approved phase-A scope. It borrows only sub2's default Top-K safety bound. It does not add sub2 scoring, EWMA, sticky sessions, model-level limits, or a background scheduler.

## Production Constraints and Evidence

The repository describes the production request path as:

- up to 20 `newapi-router` Cloud Run instances;
- request concurrency 60 per router instance;
- a Redis client pool default of 10 connections per process;
- one 1 GB, Basic-tier, single-zone Memorystore for Redis 7.0 instance.

The current code already has useful pressure controls:

- a 200 ms per-process load snapshot cache;
- per-process `singleflight` coalescing for the same candidate set;
- Redis pipelining for load reads;
- exponential wait retry backoff;
- a 30-minute lease TTL with heartbeat renewal;
- fail-closed Redis slot acquisition.

The remaining hot-path risks are concrete:

- A load snapshot currently executes `TIME` plus four commands per limited channel: `ZREMRANGEBYSCORE`, `ZCARD`, waiting `GET`, and cooldown `EXISTS`. Fifty channels therefore cost 201 logical Redis commands on every cache miss.
- Immediate acquisition checks cooldown and then executes the acquire Lua script for every candidate. When all 50 channels are full, one request can issue about 100 serialized Redis calls before it waits or returns. Adding a second fresh pass without a fan-out bound would raise this to about 200.
- Wait admission performs `INCR` and `EXPIRE` separately. A crash or timeout between them can leave an immortal counter, and rejected waiters temporarily increase the shared count.
- Wait release performs `DECR` and conditional `DEL` separately, allowing negative or stale values under races.
- Request-context release clears the lease before `ZREM`. If `ZREM` fails, the request loses the only handle needed to retry release.
- Candidate ordering can remain stale after every real acquire attempt fails; the current fresh retry only handles an all-cooldown snapshot.

## Goals

- Support a stable candidate set of 50 limited channels without per-request O(50) Redis round trips.
- Preserve correct capacity enforcement across all application instances.
- Put explicit, testable upper bounds around snapshot refresh, fresh retry, acquire fan-out, and wait counters.
- Keep Redis acquire and wait operations fail-closed so a Redis outage cannot create cross-node over-allocation.
- Keep external API response shapes and existing priority/weighted tie behavior unchanged.
- Provide command-count tests and a production-like load-test gate before enabling all 50 channels.

## Non-Goals

- Full sub2 scoring or dynamic scheduling weights.
- EWMA error rate, TTFT scoring, sticky affinity, or model-level concurrency limits.
- A global/background scheduler or a new queue service.
- Redis topology or infrastructure changes.
- Increasing `REDIS_POOL_SIZE` without measurements. A larger pool can increase pressure on the single Redis server and is not a substitute for bounded command generation.
- A guarantee for arbitrary request rate. The code will guarantee amplification bounds; the actual Redis instance must still pass a load test at the observed production peak plus safety margin.

## Component Boundaries

### `service/channel_concurrency.go`

Own only distributed concurrency state:

- slot acquire, renewal, and release;
- active and waiting load reads;
- atomic wait admission and release;
- load snapshot cache and fresh-load throttling.

`ChannelConcurrencyLoad` will no longer own cooldown state. This keeps concurrency load refreshes independent from upstream availability signals.

### `service/channel_availability.go`

Own temporary channel availability:

- Redis cooldown marker writes;
- batch cooldown reads;
- a separate short local cache;
- cache update/invalidation when this process marks a channel cooled down;
- per-candidate-set `singleflight` coalescing.

The default availability cache TTL is one second. Another node may therefore route to a newly cooled channel for at most approximately one second, which is acceptable because cooldown is advisory and slot acquisition remains authoritative for concurrency.

### `service/channel_select.go`

Own the scheduling flow:

1. Obtain the model/group candidates using the existing database/cache rules.
2. Batch-filter cooled-down candidates through the availability service.
3. Read concurrency loads for the remaining candidates.
4. Preserve load-rate ordering, priority ordering, and weighted tie-breaking.
5. Attempt at most seven ordered candidates.
6. If all attempts fail, perform at most one throttled fresh load read, rebuild the order, and attempt at most seven candidates again.
7. If the fresh pass also fails, wait on one selected candidate using the existing five-second bounded wait behavior.

Unlimited channels remain eligible and do not touch Redis for load, cooldown, acquire, heartbeat, or release state.

## Detailed Redis Behavior

### Atomic Wait Admission

One Lua script receives the waiting key, maximum waiting count, and TTL in milliseconds.

It must:

1. Read the current count, treating a missing key as zero.
2. Return rejected without modifying Redis when `current >= maxWaiting`.
3. Otherwise increment the counter and set `PEXPIRE` in the same script.
4. Return both the admission result and resulting count.

The wait TTL remains `wait timeout + one minute`. Admission failure is fail-closed and returns the existing concurrency-limit behavior.

### Atomic Wait Release

One Lua script must:

1. Treat a missing key as already released.
2. Delete the key when the count is one or less.
3. Otherwise decrement once.
4. Never create or leave a negative counter.

The waiting lease remains idempotent. A Redis release error resets its local released state so a later cleanup can retry.

### Slot Release State Machine

`ChannelConcurrencyLease` will serialize release attempts with a small lease-local lock and distinguish `held` from `released` instead of using one compare-and-swap as both "in progress" and "complete".

Redis release will:

1. Ignore the caller's canceled request context.
2. Use a detached context with a total deadline of approximately three seconds.
3. Retry `ZREM` a small, fixed number of times with short bounded backoff.
4. Mark the lease released and stop heartbeat only after `ZREM` succeeds; removing an already-missing member is success.
5. On terminal failure, stop heartbeat so the slot can expire by TTL, leave the lease in `held`, and return the error.

`ReleaseChannelConcurrencyForContext` will clear the Gin context only after a successful release. On failure it retains the lease so another cleanup path in the same request can retry.

### Cooldown Availability

Cooldown reads leave the concurrency load pipeline. A stable candidate set uses a one-second batch cache and `singleflight`, so concurrent requests in one process share one Redis `EXISTS` pipeline.

Marking cooldown writes the Redis key and immediately updates the local positive cache. Other processes observe it at their next one-second refresh. A cooldown lookup error is treated as "unknown, not confirmed cooling" and the channel may proceed to real slot acquisition; cooldown does not become a second fail-closed availability system.

### Load Snapshot

For each limited channel the Redis load pipeline contains only:

- `ZREMRANGEBYSCORE` for expired leases;
- `ZCARD` for active leases;
- `GET` for waiting count.

Redis `TIME` remains one separate command before the pipeline. The 200 ms cache and per-key `singleflight` remain.

Normal load, fresh load, and cooldown batch fetches share a per-process batch-fetch semaphore of two. With the default Redis pool of ten this prevents candidate-set cache fragmentation from consuming every connection needed by acquire and release operations.

### Fresh Load Reorder

Fresh load is allowed only after all immediate attempts for the current priority group fail.

- It runs at most once per request selection.
- It uses `singleflight` by candidate-set key.
- It has a separate minimum refresh interval of 500 ms per candidate set, limiting sustained fresh snapshots to two per second per process.
- It replaces both the fresh and normal cached snapshot on success.
- It does not recurse and cannot trigger another fresh read.

After refresh, the scheduler reorders all eligible candidates but executes at most seven acquire scripts. It then chooses a single wait target, preserving the phase-A waiting behavior.

### Acquire Fan-Out Bound

The acquire limit is seven candidates per pass, matching sub2's default load-balancer Top-K. It is a code safety constant in this phase, not a user-facing setting.

For 50 full channels:

- initial acquire scripts: at most 7;
- fresh-pass acquire scripts: at most 7;
- total before entering the wait path: at most 14.

This replaces the unbounded 50-channel scan. The fresh snapshot still reads all candidates, so a channel outside the first stale top seven can move into the fresh top seven when capacity changes.

## Redis Command Budget for 50 Channels

Let `C` be the number of limited candidates in a stable candidate set. For the production target, `C = 50`.

| Operation | Commands per miss | Local maximum rate | 50-channel maximum per process |
| --- | ---: | ---: | ---: |
| Normal load snapshot | `1 + 3C` | 5/s from 200 ms cache | 755 commands/s |
| Cooldown snapshot | `C` | 1/s from 1 s cache | 50 commands/s |
| Fresh load snapshot | `1 + 3C` | 2/s from 500 ms throttle, only after all attempts fail | 302 commands/s |
| Snapshot total, steady success | — | — | 805 commands/s |
| Snapshot total, sustained all-full fallback | — | — | 1,107 commands/s |

The normal and fresh load miss each use two network round trips (`TIME`, then one pipeline). Cooldown uses one pipeline round trip. The corresponding batch round-trip ceiling is 15 per second per process under sustained all-full fallback.

At the configured maximum of 20 router instances, the snapshot-only upper bound for one stable 50-channel set is approximately 22,140 logical Redis commands per second. This is a deliberately conservative code budget; acquire, release, heartbeat, unrelated application Redis usage, and multiple distinct candidate-set keys are additional load. For that reason rollout is gated on measured Redis headroom, not on a theoretical Redis benchmark number.

Per request, the normal success path adds one acquire script and one `ZREM`. The saturated path adds at most 14 pre-wait acquire scripts plus atomic wait admission/release and the already bounded exponential retry loop. It no longer scales linearly to 50 acquire attempts per pass.

## Failure Policy

- Redis slot acquire error: fail closed and return an internal acquisition error. Do not fall back to process memory in a multi-node production path.
- Redis wait admission error: fail closed. Do not create an untracked local waiter while Redis is the source of truth.
- Redis load snapshot error: log and use the existing memory snapshot only for ordering; every actual Redis acquire remains authoritative.
- Redis cooldown lookup error: treat cooldown as unknown and continue to actual acquire.
- Redis release error: retain the request lease, stop heartbeat after bounded retries, log the failure, and rely on lease TTL only as the final safety net.
- Batch-fetch semaphore timeout: treat it like a load snapshot error; it must not borrow all Redis pool connections or start an unbounded goroutine.
- No user-facing response contains Redis keys, lease tokens, or supplier details.

## Multi-Node Behavior

- Slot capacity, wait admission, wait release, and cooldown markers are Redis-backed and shared across nodes.
- All correctness-sensitive mutations are one Lua script or one idempotent Redis command.
- Load and availability caches are process-local performance hints only; stale values can change ordering but cannot bypass the acquire Lua script.
- `singleflight` is intentionally process-local. The explicit per-process refresh rates and the 20-node budget above account for every node refreshing independently.
- A local cooldown mark is immediately visible in the marking process and becomes visible to other nodes within approximately one second.

## Test-First Plan

Implementation begins with failing tests for each behavior group.

### Atomic Waiting

- A rejected admission does not increment the Redis counter.
- An admitted waiter receives a TTL in the same atomic operation.
- Concurrent admissions never exceed `maxWaiting`.
- Release never creates a negative counter.
- Missing-key and duplicate release are safe.

### Reliable Release

- A first `ZREM` failure followed by success releases the lease.
- Gin context retains the lease after terminal release failure.
- Gin context clears the lease only after success.
- Concurrent cleanup calls do not double-release or report false success.
- Heartbeat stops on success and after terminal failure.

### Cooldown Isolation

- The concurrency load pipeline issues zero cooldown `EXISTS` commands.
- Fifty-channel load fetch issues one `TIME`, 50 cleanup commands, 50 cardinality commands, and 50 waiting reads.
- Concurrent availability lookups coalesce into one cooldown pipeline per process.
- Local mark is visible immediately; remote-style cache refresh sees Redis state within the cache TTL.
- Cooldown lookup failure does not bypass the Redis slot limit.

### Fresh Reorder and Fan-Out

- All immediate attempts failing triggers exactly one fresh load operation.
- Fresh load can move a newly released candidate into the attempted set and acquire it.
- Fresh requests for the same candidate set coalesce and respect the 500 ms minimum interval.
- No selection performs more than seven acquire attempts per pass or fourteen before waiting.
- A 50-channel all-full test proves the fan-out bound through Redis command counters.

### Pressure and Race Verification

- A cold-cache burst of at least 100 concurrent selectors for the same 50 channels produces one normal load pipeline and one availability pipeline per process.
- Distinct candidate-set bursts never run more than two batch fetches concurrently.
- `go test -race` covers the concurrency service and selection tests.
- An opt-in real-Redis stress test exercises 50 channels and records operations, errors, pool timeouts, and selection latency. Miniredis tests remain the deterministic CI contract; they are not treated as capacity evidence.

## Rollout Gate

Do not enable concurrency control on all 50 production channels merely because unit tests pass.

Before rollout:

1. Run targeted Go tests, race tests, and command-count tests.
2. Run a real-Redis load test with 50 limited channels for at least ten minutes at two times the observed production peak request rate, using the production router instance/concurrency profile where practical.
3. Verify no Redis pool timeout, no leaked or negative waiting count, no concurrency over-allocation, and no unbounded goroutine growth.
4. Require Redis CPU p95 below 60% and maximum below 75% during the test, leaving headroom below the existing 80% alert.
5. Require channel-selection p99 added latency below 10 ms in the normal non-wait path and no material increase in application 5xx errors.
6. Roll out through staging and then a production canary/revision while watching Redis CPU, Redis operation rate, connection count, router 429/5xx rate, and selection latency.

Stop the rollout and roll back the router revision if any threshold is crossed. Do not compensate by only raising `REDIS_POOL_SIZE`.

## Acceptance Criteria

- Atomic wait scripts prevent over-admission, immortal counters, and negative counts.
- Redis release retries within a fixed deadline and never discards a failed lease from request context.
- Cooldown is absent from the concurrency load pipeline and uses an independent one-second coalesced cache.
- A 50-channel normal load miss executes exactly 151 logical Redis commands, down from 201.
- One request executes no more than seven immediate and seven fresh-pass acquire scripts before waiting.
- Fresh load is performed at most once per request and at most twice per second per candidate set per process.
- At most two batch Redis fetches run concurrently per process, preserving pool capacity for acquire/release.
- Redis acquire and wait failures remain fail-closed across multiple application nodes.
- All focused tests, race tests, and the real-Redis 50-channel load gate pass before production enablement.
- No external API format, billing behavior, or unrelated channel-selection behavior changes.
