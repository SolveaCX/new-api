# Grok Upstream-Cost Settlement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Settle `xaigrok` video tasks on the cost the upstream reports for the completed render, instead of a fixed per-second rate that cannot know the output resolution.

**Architecture:** The adapter implements the existing optional `perCallTaskBillingAdjuster` interface. On completion, it reads `usage.cost_in_usd_ticks` out of the stored upstream response, multiplies by a code constant markup, and returns the final quota. No shared billing code changes.

**Tech Stack:** Go 1.25. Tests via `go test`.

**Spec:** `docs/superpowers/specs/2026-08-14-grok-upstream-cost-billing-design.md`

---

## Background For The Implementer

`xaigrok` proxies xAI's Grok Imagine video API. xAI prices by output resolution:

| Model | 480P | 720P | 1080P |
| --- | ---: | ---: | ---: |
| `grok-imagine-video` | $0.05 | $0.07 | — |
| `grok-imagine-video-1.5` | $0.08 | $0.14 | $0.25 |

**The API accepts no resolution parameter.** The request struct is only
`{model, prompt, image?, duration?}` — the model picks the tier. So at submit
time, when billing runs, the tier that sets the price is unknowable. A fixed
local rate either overcharges the common case or loses money on the expensive
one.

The upstream reports the exact charge on completion. Here is a real stored
response from production:

```json
{
  "model": "grok-imagine-video",
  "usage": { "cost_in_usd_ticks": 500000000 },
  "video": { "url": "...", "duration": 1 },
  "status": "done",
  "progress": 100
}
```

`cost_in_usd_ticks / 1e10` is USD. `500000000` → `$0.05`, which is exactly the
published 480P rate.

### Facts already verified — build on these, do not re-derive

- **`task.Data` holds the raw upstream poll response.** `service/task_polling.go:537`
  assigns `redactVideoResponseForChannel(ch.Type, responseBody)`.
- **Redaction does not strip `usage`.** `redactVideoResponseBody` only deletes
  `bytesBase64Encoded` and truncates base64 video strings, and only inside a
  `response` object. `usage.cost_in_usd_ticks` survives — confirmed against real
  stored task rows.
- **The settlement seam exists.** `settleTaskBillingOnComplete`
  (`service/task_polling.go:1000`) short-circuits when `PerCallBilling` is set,
  which it always is once a model has a `ModelPrice`. That branch offers an
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

- **Interface signature** (`service/task_polling.go:48`):

  ```go
  type perCallTaskBillingAdjuster interface {
      AdjustPerCallBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int
  }
  ```

- **`ali` and `hailuo_v2` already implement it.** Read
  `relay/channel/task/hailuo_v2/adaptor.go:674` for the established shape.

**Returning 0 means "keep the reservation."** The caller only recalculates on a
positive value. That is the correct response to any missing or unusable cost —
never invent a number.

**Subscription users are already handled.** `RecalculateTaskQuota` converts the
delta by `SubscriptionWeight` and returns window counts to their original Redis
keys. You do not need to touch any of that; returning a plain quota is enough.

**Rules that apply** (`CLAUDE.md`): Rule 1 — use `common.Unmarshal`, never
`encoding/json` directly. Rule 11 — note multi-node behaviour. Rule 12 — state
router deploy impact.

---

## File Structure

**New**

| File | Responsibility |
| --- | --- |
| `relay/channel/task/xaigrok/cost_billing.go` | Parse the upstream cost, convert ticks to USD, compute the quota |
| `relay/channel/task/xaigrok/cost_billing_test.go` | Tests for the above |

**Modified**

| File | Change |
| --- | --- |
| `relay/channel/task/xaigrok/adaptor.go` | Add `AdjustPerCallBillingOnComplete`, plus a compile-time interface assertion |

Parsing and arithmetic live in their own file so they can be tested without
constructing a full adapter. The adapter method stays a thin delegation, which
is how `hailuo_v2` is arranged.

---

## Task 1: Convert Upstream Ticks To A Quota

**Files:**
- Create: `relay/channel/task/xaigrok/cost_billing.go`
- Test: `relay/channel/task/xaigrok/cost_billing_test.go`

- [ ] **Step 1: Write the failing test**

Create `relay/channel/task/xaigrok/cost_billing_test.go`:

```go
package xaigrok

import (
	"math"
	"testing"
)

// The observed pairs come from real production task rows: grok-imagine-video at
// 500000000 ticks and grok-imagine-video-1.5 at 800000000, which are exactly
// xAI's published 480P rates of $0.05 and $0.08.
func TestUpstreamCostUSD(t *testing.T) {
	tests := []struct {
		name  string
		ticks int64
		want  float64
		ok    bool
	}{
		{"grok-imagine-video 1s at 480P", 500000000, 0.05, true},
		{"grok-imagine-video-1.5 1s at 480P", 800000000, 0.08, true},
		{"six seconds at 480P", 3000000000, 0.30, true},
		{"zero is not a usable cost", 0, 0, false},
		{"negative is not a usable cost", -1, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := upstreamCostUSD(tc.ticks)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if ok && math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("usd = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseUpstreamCost(t *testing.T) {
	tests := []struct {
		name string
		body string
		want float64
		ok   bool
	}{
		{
			name: "real production response",
			body: `{"model":"grok-imagine-video","usage":{"cost_in_usd_ticks":500000000},"video":{"url":"x","duration":1},"status":"done"}`,
			want: 0.05,
			ok:   true,
		},
		{"usage absent", `{"model":"grok-imagine-video","status":"done"}`, 0, false},
		{"cost field absent", `{"usage":{"completion_tokens":5}}`, 0, false},
		{"cost is zero", `{"usage":{"cost_in_usd_ticks":0}}`, 0, false},
		{"cost is negative", `{"usage":{"cost_in_usd_ticks":-5}}`, 0, false},
		{"malformed json", `{not json`, 0, false},
		{"empty body", ``, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseUpstreamCost([]byte(tc.body))
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if ok && math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("usd = %v, want %v", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./relay/channel/task/xaigrok/ -run "TestUpstreamCostUSD|TestParseUpstreamCost" -v`
Expected: FAIL — `undefined: upstreamCostUSD`.

- [ ] **Step 3: Write minimal implementation**

Create `relay/channel/task/xaigrok/cost_billing.go`:

```go
package xaigrok

import (
	"github.com/QuantumNous/new-api/common"
)

// usdTicksPerDollar is the fixed-point scale xAI reports costs in.
// A 480P second bills 500000000 ticks, which is $0.05.
const usdTicksPerDollar = 1e10

// grokMarkup multiplies the upstream cost to reach the customer price.
//
// It lives in code rather than in ModelPrice so that the deploy needs no
// matching configuration change. ModelPrice keeps meaning dollars per second
// for the reservation; reusing it here would give one field two incompatible
// meanings depending on which build is running, with nothing in the data to
// tell them apart.
const grokMarkup = 1.0

// upstreamCostUSD converts xAI's fixed-point cost. A non-positive value is not
// a usable cost: the caller must keep the reservation rather than settle at zero.
func upstreamCostUSD(ticks int64) (float64, bool) {
	if ticks <= 0 {
		return 0, false
	}
	return float64(ticks) / usdTicksPerDollar, true
}

// grokUsage is the subset of the poll response this settlement needs.
type grokUsage struct {
	CostInUSDTicks int64 `json:"cost_in_usd_ticks"`
}

type grokCostEnvelope struct {
	Usage *grokUsage `json:"usage"`
}

// parseUpstreamCost reads the cost out of the stored upstream response.
//
// Returns false for anything unreadable. A missing field most likely means the
// upstream renamed it, and guessing a cost there would misprice silently --
// which is the failure this settlement exists to remove.
func parseUpstreamCost(body []byte) (float64, bool) {
	if len(body) == 0 {
		return 0, false
	}
	var envelope grokCostEnvelope
	if err := common.Unmarshal(body, &envelope); err != nil {
		return 0, false
	}
	if envelope.Usage == nil {
		return 0, false
	}
	return upstreamCostUSD(envelope.Usage.CostInUSDTicks)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./relay/channel/task/xaigrok/ -run "TestUpstreamCostUSD|TestParseUpstreamCost" -v`
Expected: PASS, 12 subtests.

- [ ] **Step 5: Commit**

```bash
git add relay/channel/task/xaigrok/cost_billing.go relay/channel/task/xaigrok/cost_billing_test.go
git commit -m "Read the upstream cost xAI reports for a completed video"
```

---

## Task 2: Turn A Cost Into A Settled Quota

**Files:**
- Modify: `relay/channel/task/xaigrok/cost_billing.go`
- Test: `relay/channel/task/xaigrok/cost_billing_test.go`

- [ ] **Step 1: Read how quota is scaled elsewhere**

Run:
```bash
grep -rn "QuotaPerUnit" relay/channel/task/hailuo_v2/adaptor.go relay/helper/price.go | head -5
```

You will see quota expressed as `usd * common.QuotaPerUnit * groupRatio`.
Match that. Do not invent a different scale.

- [ ] **Step 2: Write the failing test**

Append to `cost_billing_test.go`:

```go
import "github.com/QuantumNous/new-api/common"

func TestSettledQuotaFromCost(t *testing.T) {
	// A 480P second at $0.05, group ratio 1, markup 1: the customer pays cost.
	got := settledQuotaFromCost(0.05, 1.0)
	want := int(0.05 * grokMarkup * common.QuotaPerUnit)
	if got != want {
		t.Fatalf("quota = %d, want %d", got, want)
	}
}

func TestSettledQuotaAppliesGroupRatio(t *testing.T) {
	full := settledQuotaFromCost(0.05, 1.0)
	discounted := settledQuotaFromCost(0.05, 0.9)
	if discounted >= full {
		t.Fatalf("group ratio must reduce the quota: %d vs %d", discounted, full)
	}
	want := int(0.05 * grokMarkup * common.QuotaPerUnit * 0.9)
	if discounted != want {
		t.Fatalf("quota = %d, want %d", discounted, want)
	}
}

func TestSettledQuotaRejectsUnusableInput(t *testing.T) {
	// Returning 0 tells the caller to keep the reservation.
	for _, tc := range []struct {
		name  string
		usd   float64
		ratio float64
	}{
		{"zero cost", 0, 1},
		{"negative cost", -0.05, 1},
		{"zero group ratio", 0.05, 0},
		{"negative group ratio", 0.05, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := settledQuotaFromCost(tc.usd, tc.ratio); got != 0 {
				t.Fatalf("quota = %d, want 0", got)
			}
		})
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./relay/channel/task/xaigrok/ -run TestSettledQuota -v`
Expected: FAIL — `undefined: settledQuotaFromCost`.

- [ ] **Step 4: Write minimal implementation**

Append to `cost_billing.go` (add `math` to the import block):

```go
// settledQuotaFromCost converts an upstream cost into the quota to charge.
//
// Returns 0 for any input that cannot produce a meaningful charge. The caller
// treats 0 as "keep the reservation", so a bad input leaves the customer billed
// at the reserved amount rather than at nothing.
func settledQuotaFromCost(usd, groupRatio float64) int {
	if !isPositiveFinite(usd) || !isPositiveFinite(groupRatio) {
		return 0
	}
	quota := usd * grokMarkup * common.QuotaPerUnit * groupRatio
	if !isPositiveFinite(quota) {
		return 0
	}
	return int(quota)
}

func isPositiveFinite(v float64) bool {
	return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0)
}
```

**Before writing, check whether `xaigrok` already declares `isPositiveFinite`
or an equivalent:**

```bash
grep -rn "isPositiveFinite\|isFinite\|validPositiveFinite" relay/channel/task/xaigrok/
```

A duplicate declaration in the same package will not compile. If one exists,
reuse it and drop mine.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./relay/channel/task/xaigrok/ -v`
Expected: PASS, including Task 1's tests.

- [ ] **Step 6: Commit**

```bash
git add relay/channel/task/xaigrok/cost_billing.go relay/channel/task/xaigrok/cost_billing_test.go
git commit -m "Convert an upstream Grok cost into a settled quota"
```

---

## Task 3: Wire Settlement Into The Adapter

**Files:**
- Modify: `relay/channel/task/xaigrok/cost_billing.go`
- Modify: `relay/channel/task/xaigrok/adaptor.go`
- Test: `relay/channel/task/xaigrok/cost_billing_test.go`

- [ ] **Step 1: Read the reference implementation**

Run:
```bash
sed -n '670,700p' relay/channel/task/hailuo_v2/adaptor.go
grep -n "GroupRatio" model/task.go
```

Note how `hailuo_v2` keeps `AdjustPerCallBillingOnComplete` a one-line
delegation, and find the field name that carries the group ratio on
`TaskBillingContext`. **Use the real field name — do not assume `GroupRatio`.**

- [ ] **Step 2: Write the failing test**

Append to `cost_billing_test.go` (add the `model` import):

```go
import "github.com/QuantumNous/new-api/model"

func newGrokTask(t *testing.T, body string, groupRatio float64) *model.Task {
	t.Helper()
	task := &model.Task{Data: []byte(body)}
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		GroupRatio: groupRatio,
	}
	return task
}

func TestCompletedQuotaFromRealResponse(t *testing.T) {
	task := newGrokTask(t,
		`{"model":"grok-imagine-video","usage":{"cost_in_usd_ticks":500000000},"video":{"url":"x","duration":1},"status":"done"}`,
		1.0)
	got := completedQuota(task)
	want := int(0.05 * grokMarkup * common.QuotaPerUnit)
	if got != want {
		t.Fatalf("quota = %d, want %d", got, want)
	}
}

// The case that touches user balances: a render cheaper than the reservation
// must settle below it so the difference is refunded.
func TestCompletedQuotaBelowReservationRefunds(t *testing.T) {
	// Reserved at the 1080P worst case, rendered at 480P.
	task := newGrokTask(t,
		`{"usage":{"cost_in_usd_ticks":500000000}}`, 1.0)
	reserved := int(0.25 * common.QuotaPerUnit)
	settled := completedQuota(task)
	if settled <= 0 {
		t.Fatalf("settlement must produce a quota, got %d", settled)
	}
	if settled >= reserved {
		t.Fatalf("a 480P render must settle below a 1080P reservation: %d vs %d",
			settled, reserved)
	}
}

func TestCompletedQuotaKeepsReservationWhenUnusable(t *testing.T) {
	for _, tc := range []struct {
		name string
		task *model.Task
	}{
		{"nil task", nil},
		{"no billing snapshot", &model.Task{Data: []byte(`{"usage":{"cost_in_usd_ticks":500000000}}`)}},
		{"usage absent", newGrokTask(t, `{"status":"done"}`, 1.0)},
		{"cost absent", newGrokTask(t, `{"usage":{"completion_tokens":5}}`, 1.0)},
		{"cost zero", newGrokTask(t, `{"usage":{"cost_in_usd_ticks":0}}`, 1.0)},
		{"malformed body", newGrokTask(t, `{not json`, 1.0)},
		{"empty body", newGrokTask(t, ``, 1.0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// 0 tells the caller to keep the reservation.
			if got := completedQuota(tc.task); got != 0 {
				t.Fatalf("quota = %d, want 0", got)
			}
		})
	}
}

// The interface is unexported in package service, so assert the method set.
var _ interface {
	AdjustPerCallBillingOnComplete(*model.Task, *relaycommon.TaskInfo) int
} = (*TaskAdaptor)(nil)
```

Add the `relaycommon` import to the test file, matching the path the adapter
already uses.

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./relay/channel/task/xaigrok/ -run TestCompletedQuota -v`
Expected: FAIL — `undefined: completedQuota`.

- [ ] **Step 4: Write the settlement function**

Append to `cost_billing.go`:

```go
// completedQuota settles a finished task against the cost the upstream reported.
//
// Returns 0 to keep the reservation. That is the right answer whenever the cost
// cannot be read: the alternative is charging a number nobody reported.
func completedQuota(task *model.Task) int {
	if task == nil || task.PrivateData.BillingContext == nil {
		return 0
	}
	usd, ok := parseUpstreamCost(task.Data)
	if !ok {
		return 0
	}
	return settledQuotaFromCost(usd, task.PrivateData.BillingContext.GroupRatio)
}
```

Substitute the real group-ratio field name found in Step 1.

- [ ] **Step 5: Add the adapter method**

In `relay/channel/task/xaigrok/adaptor.go`, add:

```go
// AdjustPerCallBillingOnComplete implements service's perCallTaskBillingAdjuster.
//
// xAI prices by output resolution but accepts no resolution parameter, so the
// reservation cannot know what tier it is paying for. The upstream reports the
// exact charge on completion; settling against it tracks the published price
// through any tier, discount, or repricing without a config change.
func (a *TaskAdaptor) AdjustPerCallBillingOnComplete(task *model.Task, _ *relaycommon.TaskInfo) int {
	return completedQuota(task)
}
```

Add a compile-time assertion near the adapter's other assertions:

```go
// service's perCallTaskBillingAdjuster is unexported, so assert the method set.
// Without this a typo'd name would compile and silently keep the reservation.
var _ interface {
	AdjustPerCallBillingOnComplete(*model.Task, *relaycommon.TaskInfo) int
} = (*TaskAdaptor)(nil)
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./relay/channel/task/xaigrok/ -v`
Expected: PASS, all tests.

- [ ] **Step 7: Verify the wiring is load-bearing**

Delete the `AdjustPerCallBillingOnComplete` method body's call, replacing it
with `return 0`, then run:

Run: `go test ./relay/channel/task/xaigrok/ -run TestCompletedQuota -v`
Expected: the settlement tests still pass, because they call `completedQuota`
directly — **so also confirm the adapter path itself** by checking the compile
assertion is present and the method delegates. Restore the body.

This distinction matters: the unit tests prove the arithmetic, the assertion
proves the adapter is reachable from `service`. Both are needed.

- [ ] **Step 8: Commit**

```bash
git add relay/channel/task/xaigrok/cost_billing.go relay/channel/task/xaigrok/cost_billing_test.go relay/channel/task/xaigrok/adaptor.go
git commit -m "Settle Grok video against the cost the upstream reported"
```

---

## Task 4: Reserve At The Worst Supported Tier

The reservation must cover the most expensive tier the model can produce, or a
user could start a render they cannot pay for and the shortfall would surface
only at settlement.

**Files:**
- Modify: `relay/channel/task/xaigrok/adaptor.go`
- Test: `relay/channel/task/xaigrok/cost_billing_test.go`

- [ ] **Step 1: Find the current reservation**

Run:
```bash
grep -n -B5 -A20 "func (a \*TaskAdaptor) EstimateBilling" relay/channel/task/xaigrok/adaptor.go
```

Note how `ModelPrice` and `resolveDuration` currently produce the reservation,
and which model names the adapter serves.

- [ ] **Step 2: Write the failing test**

Append to `cost_billing_test.go`:

```go
// xAI's published worst-case rates. The reservation must not fall below these,
// or a customer could start a 1080P render they cannot pay for.
func TestWorstCaseRatePerSecond(t *testing.T) {
	tests := map[string]float64{
		"grok-imagine-video":     0.07, // 720P, the highest tier this model has
		"grok-imagine-video-1.5": 0.25, // 1080P
	}
	for name, want := range tests {
		t.Run(name, func(t *testing.T) {
			got, ok := worstCaseRatePerSecond(name)
			if !ok {
				t.Fatalf("no worst-case rate for %s", name)
			}
			if got != want {
				t.Fatalf("rate = %v, want %v", got, want)
			}
		})
	}
}

func TestWorstCaseRateUnknownModel(t *testing.T) {
	// An unknown model must not silently reserve at zero.
	if _, ok := worstCaseRatePerSecond("some-future-model"); ok {
		t.Fatal("an unknown model must not resolve a rate")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./relay/channel/task/xaigrok/ -run TestWorstCase -v`
Expected: FAIL — `undefined: worstCaseRatePerSecond`.

- [ ] **Step 4: Write the implementation**

Append to `cost_billing.go`:

```go
// worstCaseRatePerSecond is xAI's highest published rate for each model.
//
// The reservation uses this because the request cannot specify a resolution and
// the model chooses the tier. Reserving at the cheapest tier would let a
// customer start a render they cannot pay for, and the shortfall would surface
// only at settlement. Over-reserving is self-correcting: completion refunds the
// difference.
//
// Source: https://docs.x.ai/docs/models resolutionPricing.
var worstCaseRates = map[string]float64{
	"grok-imagine-video":     0.07, // 720P
	"grok-imagine-video-1.5": 0.25, // 1080P
}

func worstCaseRatePerSecond(model string) (float64, bool) {
	rate, ok := worstCaseRates[model]
	return rate, ok
}
```

- [ ] **Step 5: Use it in the reservation**

In `EstimateBilling`, where the per-second rate is currently taken from
`info.PriceData.ModelPrice`, prefer the worst-case rate when the model has one:

```go
	rate := info.PriceData.ModelPrice
	if worst, ok := worstCaseRatePerSecond(info.OriginModelName); ok && worst > rate {
		// Reserve at the tier the model could actually produce. Settlement
		// refunds down to the reported cost.
		rate = worst
	}
```

Then use `rate` where `ModelPrice` was used. **Read the surrounding code and
adapt** — the variable names and the exact expression differ from this sketch.

- [ ] **Step 6: Run the full package**

Run: `go test ./relay/channel/task/xaigrok/ -v`
Expected: PASS. If a pre-existing reservation test now fails because the
reserved amount went up, that is the intended change — update the expectation
and note it in the commit message.

- [ ] **Step 7: Commit**

```bash
git add relay/channel/task/xaigrok/
git commit -m "Reserve Grok video at the worst tier the model can produce"
```

---

## Task 5: Full Verification

- [ ] **Step 1: Package tests**

Run: `go test ./relay/channel/task/xaigrok/ -v`
Expected: all pass.

- [ ] **Step 2: Build and vet**

Run:
```bash
go build ./relay/...
go vet ./relay/channel/task/xaigrok/
gofmt -l relay/channel/task/xaigrok/
```
Expected: clean.

- [ ] **Step 3: Confirm no shared code was touched**

Run:
```bash
git diff --name-only origin/main...HEAD | grep -vE '^(relay/channel/task/xaigrok/|docs/)'
```
Expected: no output. This change is one adapter plus docs. Anything in
`service/` or `relay/relay_task.go` is a scope violation — the whole point is
that the settlement seam already exists.

- [ ] **Step 4: Baseline check**

`relay/channel/blockrun`, `relay/channel/claude`, `relay/channel/codex`, and 3
controller tests fail on clean `origin/main`. Ignore them. Note that
`relay/channel/blockrun` is a different package from
`relay/channel/task/blockrun*`.

- [ ] **Step 5: Commit any fixes**

```bash
git add -A
git commit -m "Fix issues found in the full verification pass"
```

---

## Manual Acceptance

Needs a running router and a live Grok channel.

- [ ] Submit a 1-second video on `grok-imagine-video`. Confirm the log shows a
      settled amount of `$0.05 x markup`, matching the upstream's reported cost.
- [ ] Submit a 6-second video. Confirm it settles at `$0.30 x markup` — that
      is, the amount scales with what the upstream reported, not with a local
      per-second rate.
- [ ] Confirm the reservation exceeded the settled amount and the difference was
      refunded. Compare the pre-consume and final quota in the task's billing log.
- [ ] Confirm `grok-imagine-video-1.5` settles at its own reported cost.

## Deployment Impact

`Router deploy: required` — the change is in `relay/channel/task/xaigrok/`, on
the `/v1/videos` settlement path.

**No configuration step.** The markup is a code constant, so there is no window
in which code and config disagree. Changing the markup later needs a release.

No DB migration, no new environment variable, no frontend change, nothing to
edit in the console.

**Multi-node (Rule 11):** settlement already runs on the polling path, which is
multi-node today. This adds no state and no coordination — the computation is a
pure function of the task's stored upstream response and its billing snapshot,
so any node reaches the same result. `RecalculateTaskQuota` is the existing,
concurrency-safe settlement primitive.

**Risk:** the settled amount now depends on an upstream field. If xAI renames
`cost_in_usd_ticks`, settlement returns 0 and every task keeps its reservation —
customers are billed at the worst-case tier until someone notices. The
error-path tests cover the behaviour; the log line is what makes it visible.
