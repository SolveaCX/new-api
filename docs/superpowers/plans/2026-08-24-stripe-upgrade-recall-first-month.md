# Stripe Upgrade Recall First-Month Discount Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make active Stripe recurring upgrades apply the verified Recall promotion to the immediate full-period invoice and persist/reconcile the same discounted amount without weakening quote validation.

**Architecture:** The verified one-month quote is normalized at the contract boundary, persisted into the deterministic upgrade snapshot order, and reused by the Stripe executor on every retry. Paid upgrade reconciliation validates the first invoice against that order while later renewals continue validating against the target plan's undiscounted snapshot.

**Tech Stack:** Go 1.25, GORM, stripe-go v86, Testify, SQLite-backed service tests.

---

## File structure

| File | Responsibility |
| --- | --- |
| `service/subscription_upgrade.go` | Persist authoritative upgrade pricing, apply the Recall promotion in the Stripe update, validate the first paid invoice, and record Recall conversion. |
| `service/subscription_contract.go` | Require/normalize the verified quote for a fresh active upgrade and pass it through fresh and replay executor inputs. |
| `service/subscription_upgrade_test.go` | Request-form, deterministic snapshot, paid invoice, conversion, and renewal regression coverage. |
| `service/subscription_purchase_test.go` | End-to-end service proof that a verified Recall quote reaches the upgrade executor and returns a quote-matching order. |
| `controller/subscription_self_purchase_test.go` | Existing quote-mismatch guard; run unchanged as a regression fence. |

## Global constraints

- Keep the same Stripe subscription and subscription item.
- Keep `billing_cycle_anchor=now`, `proration_behavior=none`, and the existing provider idempotency key.
- Do not rewrite historical paid orders or amounts.
- Do not remove or relax `validateSubscriptionSelfPurchaseResultQuote`.
- Persist the quote before the external Stripe mutation.
- Replays use the snapshot order and do not re-resolve the Recall campaign.
- Only Recall promotion-code discounts are added to active recurring upgrades in this change.

---

### Task 1: Authoritative discounted upgrade snapshot and Stripe request

**Files:**
- Modify: `service/subscription_upgrade.go`
- Test: `service/subscription_upgrade_test.go`

- [ ] **Step 1: Write the failing request and snapshot tests**

Add a test that seeds an active recurring contract and upgrade intent, supplies this quote, executes against the existing Stripe test server, and asserts the posted form and stored order:

```go
quote := SubscriptionPurchaseQuote{
    Currency: "USD", UnitPrice: 25, UnitAmountMinor: 2500,
    OriginalTotal: 25, OriginalTotalAmountMinor: 2500,
    DiscountKind: SubscriptionDiscountKindRecall,
    DiscountAmount: 5, DiscountAmountMinor: 500,
    Total: 20, PaymentAmountMinor: 2000,
    OtherDiscountKind: SubscriptionDiscountKindRecall,
    OtherDiscountAmountMinor: 500,
    RecallCampaignID: 91, RecallRecipientID: 92,
    RecallPromotionCodeID: "promo_upgrade_recall",
}
```

The assertions must include:

```go
require.Equal(t, "promo_upgrade_recall", updateForm.Get("discounts[0][promotion_code]"))
require.Equal(t, float64(20), order.Money)
require.Equal(t, int64(2000), order.PaymentAmountMinor)
require.Equal(t, SubscriptionDiscountKindRecall, order.DiscountKind)
require.Equal(t, int64(91), order.RecallCampaignId)
require.Equal(t, int64(92), order.RecallRecipientId)
require.Equal(t, "promo_upgrade_recall", order.RecallPromotionCodeId)
require.Equal(t, int64(500), order.RecallDiscountAmountMinor)
require.Contains(t, order.DiscountPricingSnapshot, `"payment_amount_minor":2000`)
```

Add a second test that first calls `ensureStripeSubscriptionUpgradeSnapshotOrder` without a quote and expects `ErrSubscriptionPurchaseQuoteRequired`, then creates the order with the quote, and finally replays with `VerifiedQuote=nil` and asserts the same order ID and pricing.

- [ ] **Step 2: Run the tests and verify red**

```powershell
go test ./service/ -run "TestStripeUpgrade.*Recall|TestStripeUpgrade.*Quote" -count=1 -v
```

Expected: compilation or assertion failure because `StripeSubscriptionUpgradeInput` has no quote and the Stripe request/order use the plan price.

- [ ] **Step 3: Implement the minimal quote/snapshot contract**

Add the input field:

```go
VerifiedQuote *SubscriptionPurchaseQuote
```

Before building a new order, normalize the quote and validate it against the target plan:

```go
quote, err := recurringStripeCheckoutQuote(*plan, input.VerifiedQuote)
if err != nil {
    return nil, err
}
if err := validateSubscriptionPurchaseQuoteMatchesPlan(*plan, PurchaseSubscriptionCommand{
    UserID: input.UserID, PlanID: plan.Id,
    PaymentChoice: SubscriptionPaymentChoiceStripeRecurring,
    Months: 1, VerifiedQuote: &quote,
}, quote); err != nil {
    return nil, err
}
if quote.DiscountKind != SubscriptionDiscountKindNone && quote.DiscountKind != SubscriptionDiscountKindRecall {
    return nil, fmt.Errorf("%w: active Stripe upgrade discount kind is unsupported", ErrSubscriptionPurchaseQuoteInvalid)
}
if quote.DiscountKind == SubscriptionDiscountKindRecall && strings.TrimSpace(quote.RecallPromotionCodeID) == "" {
    return nil, fmt.Errorf("%w: Recall promotion code is required", ErrSubscriptionPurchaseQuoteInvalid)
}
```

Existing-order lookup must stay before this validation so quote-free replay remains possible.

Build the order from the normalized quote, call `subscriptionDiscountSnapshotJSON(quote, "")`, and persist the Recall attribution returned by `subscriptionPurchaseRecallAttribution(quote)`.

- [ ] **Step 4: Apply the persisted promotion in the same Stripe update**

Keep the returned snapshot order from `ensureStripeSubscriptionUpgradeSnapshotOrder`. For a Recall order add:

```go
params.Discounts = []*stripe.SubscriptionDiscountParams{{
    PromotionCode: stripe.String(strings.TrimSpace(snapshotOrder.RecallPromotionCodeId)),
}}
```

Do not resolve the campaign or read a new promotion code at this point.

- [ ] **Step 5: Run focused tests and format**

```powershell
gofmt -w service/subscription_upgrade.go service/subscription_upgrade_test.go
go test ./service/ -run "TestStripeUpgrade.*Recall|TestStripeUpgrade.*Quote|TestStripeUpgradeExecuteWritesAuthoritativeMetadata|TestStripeUpgradeSnapshotOrderPersistsPendingLifecycleOnce" -count=1 -v
```

Expected: PASS.

- [ ] **Step 6: Commit Task 1**

Stage only the two service files and commit with Lore trailers. Do not stage `AGENTS.md` or `CLAUDE.md`.

---

### Task 2: Pass the quote through purchase, reconcile the discounted invoice, and preserve full-price renewal

**Files:**
- Modify: `service/subscription_contract.go`
- Modify: `service/subscription_upgrade.go`
- Test: `service/subscription_purchase_test.go`
- Test: `service/subscription_upgrade_test.go`

- [ ] **Step 1: Write failing purchase and invoice tests**

Add a purchase test using `setupSubscriptionRecallPurchaseTestDB`, `createRecallClaimFixture`, an active Stripe contract, and target price `price_subscription`. The fake upgrade executor calls `ensureStripeSubscriptionUpgradeSnapshotOrder(input, &targetPlan)`. Assert:

```go
require.NotNil(t, captured.VerifiedQuote)
require.Equal(t, SubscriptionDiscountKindRecall, captured.VerifiedQuote.DiscountKind)
require.Equal(t, int64(2000), result.Order.PaymentAmountMinor)
require.Equal(t, captured.VerifiedQuote.PaymentAmountMinor, result.Order.PaymentAmountMinor)
require.Equal(t, captured.VerifiedQuote.RecallCampaignID, result.Order.RecallCampaignId)
require.Equal(t, captured.VerifiedQuote.RecallRecipientID, result.Order.RecallRecipientId)
```

Add an upgrade paid-invoice test with a persisted Recall snapshot order whose target plan price is 2500 and payment amount is 2000. The first invoice amount is 2000 and must reconcile successfully, close the order, and create one Recall conversion. Replay the same invoice and assert the conversion count remains one.

Add a later renewal validation assertion for the same binding using a new invoice ID and amount 2500. It must pass normal renewal validation and must not create another Recall conversion.

- [ ] **Step 2: Run the tests and verify red**

```powershell
go test ./service/ -run "TestPurchaseSubscriptionStripeRecurringUpgrade.*Recall|TestStripeUpgradePaidInvoice.*Recall|TestStripeUpgrade.*Renewal" -count=1 -v
```

Expected: the executor input lacks the quote and/or the paid upgrade validator expects 2500 instead of 2000.

- [ ] **Step 3: Require and pass the quote at the contract boundary**

In the fresh recurring plan-change preflight, validate a quote for `purchase` and every `upgrade`, including an active recurring upgrade. Assign the normalized result back to `cmd.VerifiedQuote`.

Set both fresh and replay `StripeSubscriptionUpgradeInput` values to:

```go
VerifiedQuote: cmd.VerifiedQuote,
```

Do not require the quote inside `StripeSubscriptionUpgradeInput.validate`; the snapshot-order lookup is authoritative for replay.

- [ ] **Step 4: Validate the first invoice against the order and record conversion**

When `recurringPlanSnapshotForUpgradeIntentTx` finds an order, load that order before amount validation and pass it into `validateStripeUpgradePaidInvoiceFacts`. For a snapshot order:

```go
expectedCurrency := strings.ToUpper(strings.TrimSpace(upgradeOrder.PaymentCurrency))
expectedMinor := upgradeOrder.PaymentAmountMinor
```

For a legacy intent with no snapshot order, retain the current plan/snapshot price fallback.

After the paid lifecycle transition wins, call:

```go
if applied {
    if err := recordRecurringInvoiceRecallConversionTx(tx, &upgradeOrder, facts, facts.InvoiceID); err != nil {
        return true, err
    }
}
```

Keep renewal validation unchanged so later invoices continue to use `planSnapshot.Snapshot.PriceAmount` rather than the upgrade order's one-time payment amount.

- [ ] **Step 5: Run focused and guard regressions**

```powershell
gofmt -w service/subscription_contract.go service/subscription_upgrade.go service/subscription_purchase_test.go service/subscription_upgrade_test.go
go test ./service/ -run "TestPurchaseSubscriptionStripeRecurring|TestStripeUpgrade|TestReconcilePaidInvoice" -count=1
go test ./controller/ -run "TestValidateSubscriptionSelfPurchaseResultQuote|TestSubscriptionSelfPurchase.*Quote" -count=1
```

Expected: PASS and the existing controller mismatch guard remains active.

- [ ] **Step 6: Commit Task 2**

Stage only Task 2 files and commit with Lore trailers.

---

### Task 3: Repository verification, review, and staging delivery

**Files:**
- Modify only Task 1-2 files if verification exposes a defect.

- [ ] **Step 1: Static checks and GitNexus impact**

```powershell
git diff --check
npx -y gitnexus@1.6.9 detect-changes --repo 'E:\workspace\staging-deploy'
```

Expected: no whitespace errors and no unexpected high-risk change outside the planned service surface.

- [ ] **Step 2: Backend verification**

```powershell
go test ./service/... -count=1
go test ./controller/... -count=1
go build ./...
```

Expected: all commands exit 0.

- [ ] **Step 3: Independent review**

Run a spec-compliance review, then a code-quality review, fix every blocking issue, and rerun its focused test before continuing.

- [ ] **Step 4: Commit final fixes and push**

Confirm `git status --short` contains only the planned changes plus the pre-existing GitNexus count edits in `AGENTS.md` and `CLAUDE.md`. Push `fix/stripe-upgrade-recall-first-month` without force.

- [ ] **Step 5: Deploy staging and smoke test**

Use the repository's existing staging deployment path. Verify the deployment reaches ready state, then repeat the upgrade test with a one-time Recall promotion and confirm:

```text
order payment amount == signed quote total
first upgrade invoice == discounted total
following renewal amount == target plan full price
no subscription purchase quote mismatch
one Recall conversion only
```

## Self-review

- Spec coverage: quote propagation, Stripe discount, snapshot economics, replay, first invoice, renewal, conversion, and controller guard are each mapped to Tasks 1-3.
- Placeholder scan: no deferred implementation markers remain.
- Type consistency: all snippets use existing `SubscriptionPurchaseQuote`, `StripeSubscriptionUpgradeInput`, `model.SubscriptionOrder`, and Stripe v86 parameter types.

## Stop condition

Complete only when focused tests, full service/controller tests, build, GitNexus change detection, independent reviews, push, staging readiness, and the test-mode smoke flow all succeed.
