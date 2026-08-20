# Task 4 Report: Revision-aware Session Builders

## Status

Implemented revision-aware Stripe Checkout builders for top-up, recurring subscription, and one-time subscription purchases.

## Changes

- Added `StripeCheckoutDiscountSelection` and the explicit `none`, `invitation`, `recall`, and `manual` sources.
- Added normalized one-discount application: invitation selects one Coupon, recall/manual select one Promotion Code, and none clears discounts.
- Added revisioned idempotency keys using a SHA-256-derived selection identity; buyer-visible/raw code values are not present in the key.
- Added `trade_no`, `checkout_revision`, and `discount_selection` to Session plus PaymentIntent/Subscription metadata as applicable.
- Gated recall attribution metadata on `Source == recall`.
- Carried order checkout revision and the canonical persisted discount selection into recurring production checkout input.
- Kept the controller-only `genStripeSubscriptionLink` test path out of the recurring production implementation.

## TDD Evidence

RED:

```powershell
go test ./service/ -run "TestApplyStripeCheckoutDiscount|TestStripeCheckoutIdempotencyKey|TestStripeSubscriptionCheckoutRevision|TestStripeSubscriptionCheckoutInputRestores" -count=1 -v
# FAIL: StripeCheckoutDiscountSelection, ApplyStripeCheckoutDiscount, and revision fields were undefined.

$controllerFiles = <all controller production .go + owned tests>
go test $controllerFiles -run "TestStripeCheckoutSessionRevision|TestOneTimePlanStripeRevision" -count=1 -v
# FAIL: selection types were undefined and the builders did not accept revision/selection.
```

GREEN:

```powershell
go test ./service/ -run "TestApplyStripeCheckoutDiscount|TestStripeCheckoutIdempotencyKey|TestCreateStripeSubscriptionCheckout|TestStripeSubscriptionCheckout|TestStripeRecurringChangePlan" -count=1 -v
# PASS

$controllerFiles = <all controller production .go + owned tests and their existing helper test sources; customer_usage_reconciliation_test.go excluded>
go test $controllerFiles -run "TestStripeCheckoutSession(Keeps|Ordinary|RecallPromotion|Revision|Requests|Carries|Embedded|Elements|Passes)|TestBuildOneTimePlanCheckout|TestOneTimePlanStripe" -count=1 -v
# PASS

go vet ./service/
go vet $controllerFiles
git diff --check
# PASS
```

## Verification Gap

`go test ./service/ -count=1` produced no output for about two minutes and was stopped to avoid an unbounded wait. The focused Task 4 suites, related builder regressions, and vet checks passed. The known unrelated `controller/customer_usage_reconciliation_test.go` compile failure was not modified and was excluded through the documented file-list command.

## Commit

This report is included in the single Task 4 Lore commit; the resolved SHA is recorded in the parent handoff.

## Remaining Risk

The revision-aware one-time and top-up entry points retain compatibility wrappers for existing initial checkout callers. Replacement orchestration must call the explicit `ForRevision` builders/creator (or the parameterized top-up creator) with its prepared target revision and canonical selection.

## Review Fix Round 1

Resolved four findings from the formal review:

- One-time invitation orders now reconstruct Stripe's gross subtotal from the persisted net payment plus the canonical invitation discount. Invitation restore, manual replacement, and none restoration therefore apply at most one discount against gross.
- The initial one-time invitation compatibility path creates its explicit Coupon idempotently before building against gross; it then persists the created Session as before.
- `createOneTimeStripeCheckoutSessionForRevision` now creates a candidate without mutating the order's active Session pointer or checkout revision. The initial compatibility wrapper retains the persistence side effect.
- Unknown discount sources and invitation/manual/recall selections missing their required Stripe ID are rejected before metadata, Session idempotency, or Session creation. `none` remains the only zero-discount selection.
- One-time manual promotion candidates resolve the snapshot Stripe Price's stable Product and bind it to inline `price_data`; lookup failures stop before Checkout creation.

Round 1 RED evidence:

```powershell
go test $controllerFiles -run "TestOneTimePlanInvitationReplacementBuildsDiscountAgainstGrossAmount" -count=1 -v
# FAIL: invitation net 1300 was sent as subtotal instead of gross 2000.

go test $controllerFiles -run "TestCreateOneTimePlanCheckout(ForRevisionDoesNotReplaceActiveOrderPointer|InitialWrapperPersistsActiveOrderPointer)" -count=1 -v
# FAIL: candidate creation returned "Stripe checkout session mismatch"; initial wrapper passed.

go test ./service/ -run "TestApplyStripeCheckoutDiscount|TestValidateStripeCheckoutDiscountSelection|TestStripeCheckoutIdempotencyKey" -count=1 -v
# FAIL: validation/error-returning helper contracts were absent.

go test $controllerFiles -run "TestCreateOneTimePlanManualRevision" -count=1 -v
# FAIL: manual PriceData.Product was nil and Product lookup errors were ignored.

go test $controllerFiles -run "TestOneTimePlanMetadataIncludesInvitationDiscountSnapshot|TestCreateOneTimePlanCheckoutInitialInvitationCreatesCouponAndPersists" -count=1 -v
# FAIL: the compatibility path rejected the persisted invitation because no explicit Coupon had been created yet.
```

Round 1 GREEN evidence:

```powershell
go test ./service/ -run "TestApplyStripeCheckoutDiscount|TestValidateStripeCheckoutDiscountSelection|TestStripeCheckoutIdempotencyKey|TestCreateStripeSubscriptionCheckout|TestStripeSubscriptionCheckout|TestStripeRecurringChangePlan" -count=1
# PASS

go test $controllerFiles -run "TestStripeCheckoutSession|TestBuildOneTimePlanCheckout|TestOneTimePlanStripe|TestCreateOneTimePlan" -count=1
# PASS
```
