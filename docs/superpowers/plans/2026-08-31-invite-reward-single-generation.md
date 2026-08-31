# Invite Reward Single-Generation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve valid zero-price subscription orders while preventing any order funded by an inviter reward from generating another upstream inviter reward.

**Architecture:** Persist a validated funding-origin marker on subscription orders and discount-ledger reservation/terminal rows. Resolve the marker inside the locked reservation transaction using the user’s inviter-grant history; treat mixed or missing provenance as ineligible for a new reward, while leaving entitlement creation and zero-price checkout unchanged. The existing direct-inviter idempotency flow remains the only positive reward path.

**Tech Stack:** Go, GORM, SQLite/MySQL/PostgreSQL model migrations, `testify/require`, existing subscription purchase and invoice lifecycle services.

---

## File Map

- Modify `model/subscription_discount_credit.go`: funding-origin constants, ledger fields, reservation input, validation, and terminal-row propagation.
- Modify `model/subscription.go`: persist the order funding-origin marker and include it in the subscription-order schema.
- Modify `service/subscription_purchase.go`: resolve and persist provenance for pending and balance purchases, and include it in the audited pricing snapshot.
- Modify `service/subscription_discount_invoice.go`: pass provenance for recurring invoice reservations.
- Modify `model/invite_subscription_reward.go`: gate positive inviter grants, create idempotent blocked rows, and preserve direct-inviter behavior.
- Test in `model/subscription_discount_credit_test.go`, `model/invite_subscription_reward_test.go`, `model/subscription_balance_purchase_test.go`, `model/subscription_recurring_test.go`, `service/subscription_purchase_test.go`, and `service/subscription_discount_invoice_test.go`.

**Execution order:** The sections below were assembled from the red-test, model, service, reward, and compatibility lanes. Execute them in dependency order: Task 1 red tests → Task 2 model contract → Task 3 service wiring → Task 4 reward gate → Task 5 compatibility and full verification. Do not start implementation edits until the Task 1 red tests have been run.

## Task 1: Lock the funding-origin contract with failing tests

**Files:**
- Test: `model/subscription_discount_credit_test.go`
- Test: `model/invite_subscription_reward_test.go`

- [ ] **Step 1: Add a failing ledger-source propagation test.**

Use the existing SQLite setup. Reserve an invitee grant with `FundingSource: SubscriptionDiscountFundingSourceInvitee`; assert the reserve row and the later commit row retain that exact value.

```go
func TestSubscriptionDiscountReservationPreservesFundingSource(t *testing.T) {
	setupSubscriptionDiscountCreditMemoryDB(t)
	const key = "subscription-order:funding-source:reserve"
	_, err := GrantSubscriptionDiscountTx(DB, SubscriptionDiscountGrantInput{
		UserID: 111, USDMinor: 1000,
		EntryType: SubscriptionDiscountEntryTypeGrantInvitee,
		SourceType: "invitee_registration", SourceKey: "invitee:111",
		IdempotencyKey: "invitee:111", PricingSnapshot: `{}`,
	})
	require.NoError(t, err)
	created, err := ReserveSubscriptionDiscountTx(DB, SubscriptionDiscountReservationInput{
		UserID: 111, USDMinor: 500, TradeNo: "funding-source-order",
		PaymentCurrency: "USD", AppliedAmountMinor: 500, PricingSnapshot: `{}`,
		IdempotencyKey: key, FundingSource: SubscriptionDiscountFundingSourceInvitee,
		ExpiresAt: common.GetTimestamp() + 3600,
	})
	require.NoError(t, err)
	require.True(t, created)
	var reserve SubscriptionDiscountEntry
	require.NoError(t, DB.Where("idempotency_key = ?", key).First(&reserve).Error)
	require.Equal(t, SubscriptionDiscountFundingSourceInvitee, reserve.FundingSource)
	_, err = CommitSubscriptionDiscountTx(DB, key)
	require.NoError(t, err)
	var terminal SubscriptionDiscountEntry
	require.NoError(t, DB.Where("terminal_reservation_key = ?", key).First(&terminal).Error)
	require.Equal(t, SubscriptionDiscountFundingSourceInvitee, terminal.FundingSource)
}
```

## Task 2: Add validated provenance to ledger and order models

**Files:**
- Modify: `model/subscription_discount_credit.go`
- Modify: `model/subscription.go`
- Test: `model/subscription_discount_credit_test.go`

- [ ] **Step 1: Define the source constants and normalization.**

Add:

```go
const (
	SubscriptionDiscountFundingSourceNone    = "none"
	SubscriptionDiscountFundingSourceInvitee = "invitee"
	SubscriptionDiscountFundingSourceInviter = "inviter"
	SubscriptionDiscountFundingSourceMixed   = "mixed"
	SubscriptionDiscountFundingSourceUnknown = "unknown"
)

func normalizeSubscriptionDiscountFundingSource(source string) string {
	switch strings.TrimSpace(source) {
	case SubscriptionDiscountFundingSourceNone,
		SubscriptionDiscountFundingSourceInvitee,
		SubscriptionDiscountFundingSourceInviter,
		SubscriptionDiscountFundingSourceMixed:
		return strings.TrimSpace(source)
	default:
		return SubscriptionDiscountFundingSourceUnknown
	}
}
```

- [ ] **Step 2: Add persisted fields.**

```go
// SubscriptionDiscountEntry
FundingSource string `json:"funding_source" gorm:"type:varchar(16);not null;default:'unknown';index"`

// SubscriptionDiscountReservationInput
FundingSource string

// SubscriptionOrder
InvitationFundingSource string `json:"invitation_funding_source" gorm:"type:varchar(16);not null;default:'unknown';index"`
```

- [ ] **Step 3: Thread the normalized source through reserve and terminal rows.**

Extend `normalizedSubscriptionDiscountReservationInput` with `fundingSource`, normalize it before mutation, assign it to the reserve entry in `ReserveSubscriptionDiscountTx`, and copy it to commit/release entries in `closeSubscriptionDiscountReservationTx`. Empty legacy inputs normalize to `unknown`.

- [ ] **Step 4: Run the model ledger suite.**

```powershell
go test ./model -run 'TestSubscriptionDiscountReservationPreservesFundingSource|TestSubscriptionDiscount(Grant|Reserve|Commit|Release)' -count=1
```

Expected result: all selected tests pass.

- [ ] **Step 5: Commit the model contract.**

```powershell
git add model/subscription_discount_credit.go model/subscription.go model/subscription_discount_credit_test.go
git commit -m "Persist subscription discount funding provenance" -m "Keep inviter-origin information through reservation and terminal ledger rows so zero-price orders remain valid without recursive referral rewards.\n\nConstraint: Existing databases and zero-price checkout paths must remain compatible.\nRejected: Payment-amount gating, which would reject valid zero-price orders.\nConfidence: high\nScope-risk: moderate\nDirective: Treat missing provenance as reward-ineligible and preserve it across every terminal transition.\nTested: targeted model ledger tests\nNot-tested: service integration paths pending"
```

## Task 3: Resolve provenance during purchase reservations

**Files:**
- Modify: `service/subscription_purchase.go`
- Modify: `service/subscription_discount_invoice.go`
- Test: `service/subscription_purchase_test.go`
- Test: `service/subscription_discount_invoice_test.go`

- [ ] **Step 1: Add a failing mixed-credit reservation test.**

Create one account with both a `grant_invitee` and a `grant_inviter` grant, reserve an invitation discount, and assert that the saved order is marked `mixed` while the reservation still succeeds even when its final payment is zero.

```go
func TestReserveSubscriptionDiscountForOrderMarksMixedFunding(t *testing.T) {
	setupSubscriptionPurchaseTest(t)
	user := createSubscriptionPurchaseUser(t)
	require.True(t, grantSubscriptionDiscountForTest(t, user.Id, 500, "invitee:source"))
	require.True(t, grantSubscriptionDiscountForTest(t, user.Id, 3000, "inviter:source"))
	order := &model.SubscriptionOrder{UserId: user.Id, TradeNo: "mixed-order"}
	quote := SubscriptionPurchaseQuote{
		Currency: "USD", UnitAmountMinor: 3000, OriginalTotalAmountMinor: 3000,
		PaymentAmountMinor: 0, DiscountKind: SubscriptionDiscountKindInvitation,
		InvitationDiscountUSDMinor: 3000, InvitationDiscountAmountMinor: 3000,
	}
	require.NoError(t, reserveSubscriptionDiscountForOrderTx(
		model.DB, order, testPlan(t), testBalanceCommand(), quote,
		subscriptionReservationDiscountFacts{}, common.GetTimestamp()+3600,
	))
	require.Equal(t, model.SubscriptionDiscountFundingSourceMixed, order.InvitationFundingSource)
}
```

- [ ] **Step 2: Run the new service test and observe the red result.**

```powershell
go test ./service -run TestReserveSubscriptionDiscountForOrderMarksMixedFunding -count=1
```

Expected result: compilation or assertion failure because reservation code does not yet resolve or persist funding provenance.

- [ ] **Step 3: Implement the locked resolver.**

Add `ResolveSubscriptionDiscountFundingSourceTx` in `model/subscription_discount_credit.go`. It must return `none` for a zero requested discount, `invitee` when no positive inviter-origin grant exists, `inviter` when the available balance is attributable only to inviter grants, `mixed` when both sources may contribute, and `unknown` when historical or migration data cannot prove the source. The resolver runs inside the reservation transaction and never trusts a client-provided source.

- [ ] **Step 4: Wire both one-time purchase paths.**

In `reserveSubscriptionDiscountForOrderTx`, resolve the source before writing the reservation and persist it in the order, snapshot, and `SubscriptionDiscountReservationInput`:

```go
source := model.SubscriptionDiscountFundingSourceNone
if quote.DiscountKind == SubscriptionDiscountKindInvitation && quote.InvitationDiscountUSDMinor > 0 {
	var err error
	source, err = model.ResolveSubscriptionDiscountFundingSourceTx(
		tx, order.UserId, quote.InvitationDiscountUSDMinor,
	)
	if err != nil {
		return err
	}
}
order.InvitationFundingSource = source
```

Rebuild `subscriptionDiscountPricingSnapshot` after the source is resolved so the audit snapshot and reserve row agree. The signed client quote must not be allowed to override the source.

- [ ] **Step 5: Wire recurring invoice reservations.**

In `buildStripeSubscriptionDiscountInvoicePrepareTx`, call the same resolver while the account is locked, add the result to `stripeSubscriptionDiscountInvoiceSnapshot`, and pass it to `SubscriptionDiscountReservationInput`. Retry paths must read the persisted reserve row instead of recomputing a different source.

- [ ] **Step 6: Run purchase and invoice tests.**

```powershell
go test ./service -run 'TestReserveSubscriptionDiscountForOrderMarksMixedFunding|TestSubscriptionDiscountInvoice|TestPurchaseSubscription' -count=1
```

Expected result: selected purchase and invoice tests pass, including zero-final-payment quote cases.

## Task 4: Gate recursive rewards without touching entitlement success

**Files:**
- Modify: `model/invite_subscription_reward.go`
- Test: `model/invite_subscription_reward_test.go`
- Test: `model/subscription_balance_purchase_test.go`
- Test: `model/subscription_recurring_test.go`

- [ ] **Step 1: Add the nested-reward reason and blocked-row helper.**

Define:

```go
const InviteSubscriptionRewardReasonInviterRewardReentry = "inviter_reward_reentry"
```

Add `blockInviteSubscriptionRewardForFundingSourceTx(tx, order, inviteeID, inviterID, now)`. It inserts one `InviteSubscriptionReward` row with `Status=blocked`, `RewardQuota=0`, and the reason above using `OnConflict(DoNothing)`. It then calls `finalizeInviteSubscriptionRewardInviteeTx`; it never calls `GrantSubscriptionDiscountTx` and never increments `aff_count`.

- [ ] **Step 2: Add the source gate before positive grant creation.**

After the existing direct-inviter lookup and idempotency repair check in `grantInviteSubscriptionDiscountAfterPaidOrderTx`, read the persisted order/reservation source:

```go
source := normalizeSubscriptionDiscountFundingSource(order.InvitationFundingSource)
if source == SubscriptionDiscountFundingSourceInviter ||
	source == SubscriptionDiscountFundingSourceMixed ||
	source == SubscriptionDiscountFundingSourceUnknown {
	return blockInviteSubscriptionRewardForFundingSourceTx(
		tx, order, invitee.Id, inviter.Id, now,
	)
}
```

For legacy orders with an empty marker, inspect the linked reservation row; if it is absent, use the conservative ledger-history fallback and return `unknown` whenever inviter-origin evidence cannot be excluded. Orders marked `none` or `invitee` continue through the existing direct-inviter grant path.

- [ ] **Step 3: Keep post-create logging distinct.**

Extend `inviteSubRewardCreateResult` with `reason`, and make `runInviteSubRewardPostCreateHooks` log `inviter_reward_reentry` separately from the configured cap message. Cache invalidation and invitee finalization stay unchanged.

- [ ] **Step 4: Run reward tests red-to-green.**

First run the new tests:

```powershell
go test ./model -run 'TestInviteSubRewardAllowsZeroPriceInviteeFunding|TestInviteSubRewardBlocksInviterFundingWithoutClawback' -count=1
```

Then run the focused existing suites:

```powershell
go test ./model -run 'Invite(Sub|Subscription)Reward|SubscriptionBalancePurchase|CompleteSubscriptionOrderWithProviderBinding' -count=1
```

Expected result: invitee-source zero-price orders grant once; inviter/mixed/unknown orders create exactly one blocked row and no positive ledger grant; cap, idempotency, balance, and provider tests remain green.

- [ ] **Step 5: Commit the reward gate.**

```powershell
git add model/invite_subscription_reward.go model/invite_subscription_reward_test.go model/subscription_balance_purchase_test.go model/subscription_recurring_test.go
git commit -m "Stop recursive inviter reward generation" -m "Keep zero-price subscriptions valid while blocking orders funded by inviter rewards from minting another upstream reward.\n\nConstraint: Entitlement creation and direct-inviter zero-price rewards remain supported.\nRejected: Blocking all zero-price orders or all second-level inviters.\nConfidence: high\nScope-risk: moderate\nDirective: Never let inviter-origin, mixed, or unknown funding enter the positive grant path.\nTested: model reward, balance, and recurring suites\nNot-tested: full repository verification pending"
```

- [ ] **Step 2: Add failing reward-lineage tests.**

Add tests that set `order.InvitationFundingSource` before calling the existing reward entry point:

```go
func TestInviteSubRewardAllowsZeroPriceInviteeFunding(t *testing.T) {
	setupInviteSubRewardTest(t)
	inviter := createInviteRewardUser(t, "root", 0)
	invitee := createInviteRewardUser(t, "invitee", inviter.Id)
	order := createCompletedSubscriptionOrder(t, invitee.Id, 0, "sub-zero-invitee")
	order.InvitationFundingSource = SubscriptionDiscountFundingSourceInvitee
	require.NoError(t, order.Update())
	require.NoError(t, TryGrantInviteSubscriptionRewardAfterOrderCompleted(order.TradeNo))
	var reward InviteSubscriptionReward
	require.NoError(t, DB.Where("invitee_id = ?", invitee.Id).First(&reward).Error)
	require.Equal(t, InviteSubRewardStatusGranted, reward.Status)
}

func TestInviteSubRewardBlocksInviterFundingWithoutClawback(t *testing.T) {
	setupInviteSubRewardTest(t)
	inviter := createInviteRewardUser(t, "root", 0)
	invitee := createInviteRewardUser(t, "invitee", inviter.Id)
	order := createCompletedSubscriptionOrder(t, invitee.Id, 0, "sub-zero-inviter")
	order.InvitationFundingSource = SubscriptionDiscountFundingSourceInviter
	require.NoError(t, order.Update())
	require.NoError(t, TryGrantInviteSubscriptionRewardAfterOrderCompleted(order.TradeNo))
	var reward InviteSubscriptionReward
	require.NoError(t, DB.Where("invitee_id = ?", invitee.Id).First(&reward).Error)
	require.Equal(t, InviteSubRewardStatusBlocked, reward.Status)
	require.Equal(t, InviteSubscriptionRewardReasonInviterRewardReentry, reward.Reason)
	var entries int64
	require.NoError(t, DB.Model(&SubscriptionDiscountEntry{}).Where("user_id = ?", inviter.Id).Count(&entries).Error)
	require.Zero(t, entries)
}
```

- [ ] **Step 3: Run only these tests and verify the red result.**

```powershell
go test ./model -run 'TestSubscriptionDiscountReservationPreservesFundingSource|TestInviteSubRewardAllowsZeroPriceInviteeFunding|TestInviteSubRewardBlocksInviterFundingWithoutClawback' -count=1
```

Expected result: compilation fails because the new source fields and nested-reward reason do not yet exist.

## Task 5: Compatibility and full verification

**Files:**
- Modify: `model/main.go` only if the ordered migration list does not already include the changed models.
- Test: `model/invite_subscription_reward_test.go`
- Test: `service/subscription_discount_invoice_test.go`

- [ ] **Step 1: Add legacy and retry regression tests.**

Cover an old order with an empty marker and prior inviter-origin evidence: the order remains successful, while reconciliation creates one blocked reward row. Cover an old order with no inviter-origin evidence: the direct reward remains allowed. Cover reserve followed by release and a second reservation to ensure source markers do not leak between orders.

- [ ] **Step 2: Verify schema migration on SQLite.**

Run the existing migration/model tests and assert `AutoMigrate` creates `funding_source` on `subscription_discount_entries` and `invitation_funding_source` on `subscription_orders` without removing existing indexes:

```powershell
go test ./model -run 'Test.*(Migration|AutoMigrate|SubscriptionDiscount)' -count=1
```

- [ ] **Step 3: Run formatting and static checks.**

```powershell
gofmt -w model/subscription_discount_credit.go model/subscription.go model/invite_subscription_reward.go service/subscription_purchase.go service/subscription_discount_invoice.go
go vet ./model ./service
```

- [ ] **Step 4: Run the complete Go test suite.**

```powershell
go test ./... -count=1
```

Expected result: exit code 0 with no failing packages. If an unrelated pre-existing failure appears, record its exact package and failure before changing this feature.

- [ ] **Step 5: Inspect the final diff and commit verification.**

```powershell
git diff --check origin/main...HEAD
git status --short --branch
git log --oneline --decorate -5
```

Confirm that only planned model/service/test/docs files changed, no production credentials or data dumps are present, and the design document still matches the implementation. Commit any final adjustment with the Lore trailers required by `AGENTS.md`.

