# Stripe recurring upgrade Recall first-month discount

## Goal

Make an active Stripe recurring subscription upgrade charge the authoritative discounted quote on the upgrade invoice, apply a Recall promotion only to that invoice, and keep every retry and webhook reconciliation consistent with the same persisted facts.

## Current failure

The purchase quote can contain a Recall first-month discount, but the active recurring upgrade path currently:

1. creates the plan-change intent without carrying the verified quote into `StripeSubscriptionUpgradeInput`;
2. updates only the Stripe price and omits `discounts[0][promotion_code]`;
3. creates the upgrade snapshot order from the plan's undiscounted price; and
4. validates the paid upgrade invoice against that undiscounted plan price.

Stripe therefore charges the full price while the controller still compares the result against the discounted signed quote and reports `subscription purchase quote mismatch`.

## Selected design

Keep the existing Stripe subscription. The same `subscriptions.update` request that changes the price continues to use `billing_cycle_anchor=now` and `proration_behavior=none`, and adds the Recall promotion code from the authoritative quote. Recall coupons use `duration=once`, so the immediately generated full-period invoice receives the discount and later renewals return to the plan price.

The controller mismatch validation remains unchanged. It is a tamper and drift guard; the service must make the order and provider charge match the quote instead of weakening the guard.

## Quote and snapshot contract

- A fresh active recurring upgrade requires a verified one-month Stripe recurring quote.
- `ChangeSubscriptionPlan` validates the quote against the target plan and passes it to `StripeSubscriptionUpgradeInput`.
- If an upgrade intent exists but no snapshot order exists, an execution attempt without a verified quote returns `ErrSubscriptionPurchaseQuoteRequired`. The controller already verifies the replay token and retries with the quote.
- Once the deterministic upgrade snapshot order exists, retries reuse it without recalculating pricing or requiring another quote.
- The snapshot stores the actual payable amount in `Money` and `PaymentAmountMinor`, the target plan's normal unit price, the Recall campaign/recipient/promotion-code identity, the Recall discount amount, `DiscountKind=recall`, and the discount-pricing JSON snapshot.
- No historical paid order is rewritten or reclassified as unpaid.

## Stripe request

For a Recall snapshot order, `executeStripeSubscriptionUpgrade` sends:

```text
billing_cycle_anchor=now
proration_behavior=none
items[0][id]=<current subscription item>
items[0][price]=<target price>
discounts[0][promotion_code]=<persisted Recall promotion code>
```

The promotion code comes from the persisted snapshot order, not from a newly resolved campaign. This keeps Stripe idempotent retries and cross-node retries economically identical.

For an undiscounted snapshot, the upgrade sends no new discount. Invitation-credit discounts are not introduced into this active-upgrade path by this change because they require a different Stripe coupon lifecycle.

## Invoice reconciliation and attribution

- The upgrade invoice validator uses the snapshot order's payment currency and exact `PaymentAmountMinor` when an upgrade snapshot exists.
- Legacy upgrade intents without a snapshot keep the existing plan-price validation fallback.
- A successful Recall upgrade records conversion from the snapshot order exactly once in the same database transaction as the paid lifecycle transition.
- The binding keeps the target plan snapshot as its initial order. Normal renewal validation continues to derive the later renewal amount from the target plan's undiscounted `PriceAmount`, so the one-time discount does not leak into later invoices.

## Concurrency and replay

- The deterministic trade number remains `SUBUPGINT<change_intent_id>` and stays unique.
- The provider update keeps the intent-scoped Stripe idempotency key.
- Order creation remains transactional and recovers a concurrently inserted order by unique trade number.
- Existing orders are authoritative even when a replay arrives without a quote.

## Error handling

- Missing quote before first snapshot: `ErrSubscriptionPurchaseQuoteRequired`.
- Invalid or plan-mismatched quote: `ErrSubscriptionPurchaseQuoteInvalid` wrapping the existing mismatch reason.
- Recall quote without a persisted promotion-code ID: reject before the Stripe mutation.
- Stripe update failure keeps the existing subscription-schedule restoration behavior.
- Invoice amount drift remains a permanent reconciliation error.

## Verification

- A request-form test proves `discounts[0][promotion_code]` is present with the price update.
- Snapshot tests prove discounted money, Recall identity, pricing snapshot, and quote-free replay.
- Purchase integration proves the verified Recall quote reaches the executor and the returned order matches it.
- Paid-invoice tests prove the discounted first invoice reconciles and records Recall conversion once.
- Renewal validation proves the following invoice is accepted at the normal target-plan price.
- Existing controller quote-mismatch tests remain unchanged and passing.

## Non-goals

- Canceling the existing Stripe subscription and creating another subscription.
- Marking historical paid orders unpaid.
- Globally enabling the embedded dynamic promotion-code feature.
- Adding invitation-credit coupon generation to active recurring upgrades.
