# Grok Video Upstream-Cost Settlement Design

## Decision

`xaigrok` settles on the cost the upstream reports for the completed task,
instead of a fixed per-second rate configured locally.

The markup over that cost is a code constant. `ModelPrice` keeps its current
meaning — dollars per second — and keeps driving the reservation.

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
require any change on this side.

## Mechanism

```
submit    reserve at the configured worst case
          ...task runs...
complete  AdjustPerCallBillingOnComplete reads cost_in_usd_ticks
          final quota = upstream cost x grokMarkup
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

## Markup is a code constant, and ModelPrice keeps its meaning

```
final quota = upstream cost x grokMarkup
```

`grokMarkup` is a constant in the adaptor. `ModelPrice` is left alone: it keeps
meaning dollars per second and keeps driving the reservation, exactly as today.

The alternative was to reuse `ModelPrice` as the markup multiplier, which would
have made repricing a one-number config edit. It was rejected because the field
would then mean two different things depending on which build is running, with
nothing in the data to say which:

| Order | `0.09` / `1.3` read as | Result |
| --- | --- | --- |
| Code first, config unchanged | multiplier | charges 9% of cost |
| Config first, code unchanged | dollars per second | charges $1.30/s, ~26x |

Both values are ordinary positive floats, so no type, validation, or test could
catch the mismatch. Keeping the markup in code removes the ordering hazard
entirely — the deploy is a plain code deploy with no configuration step, and
nothing is wrong in between.

The cost is that changing the markup needs a release. That is an acceptable
trade for this channel: the markup is a business constant that changes rarely,
and the reservation rates it sits alongside are already code-level values.

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
- Final quota equals upstream cost times `grokMarkup`; a markup of `1.0`
  charges cost exactly.
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

No DB migration, no new environment variable, no frontend change, and nothing
to edit in the console.

**No configuration step.** The markup lives in code, so this is a plain code
deploy with no window in which the two halves disagree. Changing the markup
later needs a release, which is the trade made for removing that hazard.

Minimum validation after deploy: submit one video job per model and confirm the
settled amount equals the upstream's reported cost times the markup, and that a
render cheaper than the reservation refunds the difference.
