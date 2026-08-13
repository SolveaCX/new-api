# Grok Video Upstream-Cost Settlement Design

## Decision

`xaigrok` settles on the cost the upstream reports for the completed task,
instead of a fixed per-second rate configured locally.

`ModelPrice` changes meaning for this channel: it becomes a markup multiplier
applied to the upstream cost, not a price per second.

## Why

xAI prices Grok Imagine video by output resolution:

| Model | 480P | 720P | 1080P |
| --- | ---: | ---: | ---: |
| `grok-imagine-video` | $0.05 | $0.07 | — |
| `grok-imagine-video-1.5` | $0.08 | $0.14 | $0.25 |

**The API accepts no resolution parameter.** The request carries only model,
prompt, optional image, and duration; the model chooses the output tier. So at
submission time — when billing is computed — the tier that determines the price
is unknowable.

A fixed local rate therefore cannot track the published price. It either
overcharges the common case or loses money on the expensive one. Today's flat
`$0.09` / `$0.11` would lose `$0.14` per second on a 1080P render.

The upstream closes the gap. Its poll response carries the exact charge:

```json
{
  "model": "grok-imagine-video",
  "usage": { "cost_in_usd_ticks": 500000000 },
  "video": { "url": "...", "duration": 1 },
  "status": "done"
}
```

`cost_in_usd_ticks / 1e10` is the amount in USD.

### The reported cost matches the published price

Five production tasks, both models, one- and six-second durations:

| Model | Duration | Reported | Per second | Published 480P |
| --- | ---: | ---: | ---: | ---: |
| `grok-imagine-video` | 1s | $0.05 | $0.05 | $0.05 |
| `grok-imagine-video` | 6s | $0.30 | $0.05 | $0.05 |
| `grok-imagine-video-1.5` | 1s | $0.08 | $0.08 | $0.08 |
| `grok-imagine-video-1.5` | 6s | $0.48 | $0.08 | $0.08 |

Exact, so no supplier discount is in play. Every observation lands on the 480P
tier, which is consistent with that being the only tier produced so far.

That evidence covers 480P only; whether 720P and 1080P are likewise
undiscounted is unknown. It does not need to be known — following the reported
cost is correct under a discount, a price change, or a new tier, none of which
require a config edit.

## Mechanism

```
submit    reserve at the configured worst case
          ...task runs...
complete  AdjustPerCallBillingOnComplete reads cost_in_usd_ticks
          final quota = upstream cost x ModelPrice
          difference settled against the reservation
```

### Settlement runs at completion, not submit

`cost_in_usd_ticks` appears only in the poll response; the submit response
carries just `request_id`. This differs from `modelapiseedance`, whose upstream
returns `usage.estimated_usd` at submit and which therefore uses
`AdjustBillingOnSubmit`.

### No change to shared billing code

`settleTaskBillingOnComplete` short-circuits when `PerCallBilling` is set, which
it always is once a model has a `ModelPrice`. But that branch already offers an
opt-in escape:

```go
if bc := task.PrivateData.BillingContext; bc != nil && bc.PerCallBilling {
    if adjuster, ok := adaptor.(perCallTaskBillingAdjuster); ok {
        if actualQuota := adjuster.AdjustPerCallBillingOnComplete(task, taskResult); actualQuota > 0 {
            RecalculateTaskQuota(ctx, task, actualQuota, "adaptor计费调整")
        }
    }
    ...
}
```

`xaigrok` implements `perCallTaskBillingAdjuster`. `hailuo_v2` already uses the
same seam, so this is an established path rather than a new one.

## ModelPrice becomes a markup multiplier

```
final quota = upstream cost x ModelPrice
```

| Value | Meaning |
| ---: | --- |
| `1.0` | sell at cost |
| `1.3` | 30% markup |
| `1.5` | 50% markup |

Reusing `ModelPrice` keeps repricing a one-number edit and matches how the field
reads elsewhere — a factor applied to a base.

### This is a breaking configuration change

For this channel `ModelPrice` currently means dollars per second
(`grok-imagine-video` = `0.09`). After this change it means a multiplier.
Deploying one without the other misprices badly in both directions:

| Order | Result |
| --- | --- |
| Code first, config unchanged | `0.09` read as a multiplier — charges 9% of cost |
| Config first, code unchanged | `1.3` read as dollars per second — charges $1.30/s, ~26x |

**Deploy the code, then change the configuration immediately.** The PR must say
so, and the two values to write are `grok-imagine-video` and
`grok-imagine-video-1.5`.

## Reservation

Reserve at the worst supported tier: `$0.07/s` for `grok-imagine-video`,
`$0.25/s` for `grok-imagine-video-1.5`, times duration.

Under-reserving would let a user start an expensive render they cannot pay for,
and the shortfall surfaces only at settlement. Over-reserving is visible and
self-correcting: completion refunds the difference, which today is most of it,
since every observed render has been 480P.

## Error handling

| Case | Behaviour |
| --- | --- |
| `usage` absent, or `cost_in_usd_ticks` missing | Keep the reservation. Log once. |
| Cost is zero, negative, or non-finite | Keep the reservation. Log once. |
| Computed quota is not positive-finite | Keep the reservation. |

Never guess a cost. A missing field most likely means the upstream renamed it,
and inventing a number would misprice silently — the failure this design exists
to remove. The log line is what makes that discoverable.

## Multi-node behaviour

Settlement already runs on the polling path, which is multi-node today. This
adds no state and no coordination: the computation is a pure function of the
task's stored upstream response and its billing snapshot, so any node reaches
the same result. `RecalculateTaskQuota` is the existing, concurrency-safe
settlement primitive.

## Test contract

- `cost_in_usd_ticks` converts at `1e10`, verified against the observed pairs
  (`500000000` → `$0.05`, `800000000` → `$0.08`).
- Final quota equals upstream cost times `ModelPrice`; a markup of `1.0` charges
  cost exactly.
- Absent `usage`, absent field, zero, negative, and non-finite each keep the
  reservation and return 0.
- **A cost below the reservation refunds the difference.** This is the case that
  touches user balances, so it is tested explicitly rather than inferred.
- A cost above the reservation charges the difference.
- The adaptor satisfies `perCallTaskBillingAdjuster` at compile time, asserted
  structurally since the interface is unexported.

## Deployment impact

`Router deploy: required` — the change is in `relay/channel/task/xaigrok/`, on
the `/v1/videos` settlement path.

`newapi-console` needs the same build so the configuration can be edited. No DB
migration, no new environment variable, no frontend change.

**Rollout risk is in the configuration, not the code.** Ship the code, then
immediately set both models' `ModelPrice` to the intended markup. Until that
second step, billing for this channel is wrong by roughly 10x in the safe
direction (undercharging).
