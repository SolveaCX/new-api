# Stripe Checkout Promotion Code Design

**Status:** Approved direction; ready for implementation planning

**Date:** 2026-08-20

**Scope:** Flatkey wallet top-ups and subscription purchases that use Stripe Checkout Elements

## Context

Flatkey currently creates Stripe Checkout Sessions with either no discount or one automatic discount supplied by an invitation or recall campaign. Stripe Checkout supports only one coupon or promotion code on a Session. A buyer-entered promotion code therefore cannot be stacked with the existing automatic discount.

The checkout must expose one promotion-code input for every Stripe purchase type while preserving the existing invitation and recall behavior. Applying a manual code replaces the automatic discount immediately; removing it restores the original automatic discount.

## Goals

- Show one always-visible promotion-code input for Stripe wallet top-ups, recurring subscriptions, and one-time subscription purchases.
- Let Stripe remain authoritative for promotion-code eligibility, including product, customer, minimum amount, first-time customer, expiry, and redemption limits.
- Replace, never stack, the existing invitation or recall discount when a manual code is applied.
- Restore the original automatic discount when the manual code is removed.
- Refresh Checkout Elements and the displayed totals without closing the Flatkey modal.
- Prevent stale requests, duplicate orders, duplicate charges, and discount-attribution errors.
- Keep the layout usable without horizontal overflow at desktop and 390 px mobile widths.

## Non-goals

- An admin interface for creating Stripe coupons or promotion codes.
- Discount stacking or choosing between multiple simultaneous discounts.
- Stripe's private-preview dynamic Checkout Session discount API.
- Changes to the hosted Stripe Checkout page.
- Changes to refund, invoice, or subscription-cancellation policy.

## Confirmed UX

The promotion-code control is part of the shared Stripe Checkout summary pane:

- Desktop: below the amount summary card and above the **Continue** button in the right pane.
- Mobile: below the amount summary and above **Continue**, in the same reading order.
- The control contains a `Promotion code` label, text input, and **Apply** action. Pressing Enter also applies the code.
- While applying or restoring, the input, action, and **Continue** button are disabled and an inline progress indicator is shown.
- A successful apply shows the masked or normalized code, the refreshed discount and total, a **Remove** action, and the message `Promotion code applied. Previous discount replaced.` when an automatic discount was replaced.
- Removing a manual code restores the original invitation or recall discount and refreshes the totals.
- An invalid, expired, ineligible, or ambiguous code produces an inline error. The current Session, discount, and totals remain unchanged.
- The buyer is not shown a second replacement-confirmation dialog.

## Discount Precedence and State

Each order has a canonical original discount derived on the server when the order is created:

1. Eligible recall promotion code.
2. Eligible invitation coupon.
3. No automatic discount.

The active discount is one of:

```text
original none/invitation/recall --apply valid manual--> manual
manual --apply another valid manual---------------> manual
manual --remove-----------------------------------> original none/invitation/recall
```

Only the active discount is attached to the replacement Stripe Checkout Session. The client never decides the original discount, amount, currency, product, customer, or order identity.

If a manual code wins, invitation credit is not consumed and recall attribution is not recorded. The discount on the successfully paid Stripe Checkout Session is the authoritative source for downstream accounting and campaign attribution.

## Architecture

### Initial Checkout Creation

Existing top-up and subscription endpoints continue to create the internal order and initial Stripe Checkout Session. Checkout Elements responses are extended with:

- `checkout_context`: a short-lived, opaque, server-signed token.
- `checkout_revision`: a monotonically increasing integer, starting at `1`.
- `discount_state`: normalized display metadata for the active discount.

The signed context binds at least:

- authenticated user ID;
- purchase kind;
- internal order/trade ID;
- current revision;
- expiry time.

The context must not contain a reusable Stripe secret or trust client-provided amount, currency, product, customer, or original-discount data.

### Unified Discount Endpoint

Add an authenticated, rate-limited endpoint:

```http
POST /api/user/stripe/checkout/discount
```

Apply request:

```json
{
  "checkout_context": "opaque-signed-token",
  "expected_revision": 1,
  "action": "apply",
  "promotion_code": "SAVE20"
}
```

Restore request:

```json
{
  "checkout_context": "opaque-signed-token",
  "expected_revision": 2,
  "action": "restore"
}
```

Successful response:

```json
{
  "client_secret": "cs_..._secret_...",
  "publishable_key": "pk_...",
  "checkout_context": "next-opaque-signed-token",
  "checkout_revision": 2,
  "discount_state": {
    "source": "manual",
    "display_name": "SAVE20",
    "promotion_code_masked": "SAVE20",
    "replaced_source": "invitation"
  },
  "summary": {}
}
```

`summary` reuses the purchase-specific summary shape already returned to the checkout dialog. Fields that do not apply to a purchase kind are omitted.

### Server-side Session Revision

For `apply`, the backend:

1. Authenticates the user and verifies the signed context, order ownership, expiry, and expected revision.
2. Normalizes the submitted code by trimming surrounding whitespace without logging the raw value.
3. Resolves a unique active Stripe Promotion Code server-side.
4. Reserves the next checkout revision with a compare-and-swap against the order's current revision and records it as `preparing`.
5. Checks that the current Stripe Session is still unpaid and replaceable.
6. Creates a candidate replacement Session for the same internal order with only the selected manual promotion code. Stripe validates the code and coupon restrictions during this creation.
7. Rejects missing, inactive, ineligible, or ambiguous codes without changing or expiring the current payable Session.
8. Persists the candidate Session ID and target discount on the preparing revision before changing the active Session.
9. Expires the old Session. If payment wins this race, the candidate is expired and the paid old Session remains authoritative.
10. Promotes the prepared revision and Session to active with a compare-and-swap.
11. Returns a new client secret, signed context, revision, discount state, and refreshed summary.

For `restore`, the backend follows the same revision flow but reconstructs the original automatic discount from canonical server-side order data. It never restores from a discount supplied by the client.

Applying a second manual code replaces the first manual code through the same flow. A replacement Session contains exactly one `discounts` entry, or none when restoring an order with no original discount.

## Promotion-code Resolution

- Match the buyer-entered code case-insensitively after trimming whitespace.
- Prefer an active code explicitly restricted to the current Stripe customer; otherwise use an active global code.
- If multiple eligible results remain, return an ambiguous-code error rather than choosing nondeterministically.
- Let Stripe validate promotion-code and coupon restrictions. Flatkey may preflight obvious mismatches but must not independently reimplement Stripe's eligibility rules as the final authority.
- Return stable application error codes with localized UI messages; do not expose raw Stripe errors.

## Concurrency, Idempotency, and Recovery

- Every mutation includes `expected_revision`. A mismatch returns `409 checkout_revision_conflict` plus enough state for the client to reload the latest checkout.
- The client also keeps a local request generation. Responses from an older generation are ignored even if they arrive later.
- Stripe replacement creation uses a revisioned idempotency key derived from internal order ID, target revision, and a hash of the chosen discount identity. The raw promotion code is not included in the key or logs.
- The **Continue** action is disabled while a replacement is in flight.
- A candidate Session is not returned to the client until the previous Session has been expired and the candidate revision has become active.
- If the old Session has already completed or cannot be expired because payment won the race, the backend expires the candidate and returns a completed/conflict result.
- If candidate creation or Stripe validation fails, the preparing revision is abandoned and the previous Session remains payable.
- If the process fails after the candidate is recorded, a reconciliation path finishes the state transition deterministically: expire the candidate while the old Session remains active, or promote the candidate after confirming that the old Session is expired. It must not create another internal order.
- Webhook fulfillment remains idempotent by internal order ID. A completed Session is accepted only when its order metadata and persisted revision/session relationship are valid. A superseded Session must not produce a second fulfillment.

## Persistence and Accounting

The internal order or a dedicated checkout-revision record stores:

- current Stripe Checkout Session ID;
- checkout revision;
- prepared Stripe Checkout Session ID and transition state, when a revision is in flight;
- canonical original discount source and identity;
- active discount source and identity;
- masked display code, when needed for audit/UI;
- replacement timestamps and terminal state.

Do not store the buyer-entered raw code in application logs. The stable Stripe Promotion Code ID is sufficient for reconciliation.

Invitation consumption, recall conversion attribution, top-up crediting, and subscription fulfillment use the successfully paid Session's actual discount identity. They must not assume that the original automatic discount remained active.

## Frontend Design

### Components and State

The shared Stripe Checkout dialog owns:

- current checkout client secret and revision context;
- promotion-code input value;
- normalized discount state;
- apply/restore loading state;
- inline success or error message;
- request generation used to ignore stale responses.

The summary layout receives a promotion-code control slot so desktop and mobile use one implementation and one ordering rule. The existing view model continues to derive amounts from Stripe Checkout Session data and the returned purchase summary.

### Session Remount

After a successful revision response, the dialog:

1. Destroys the mounted Checkout Elements instance.
2. Replaces the client secret, signed context, revision, discount state, and summary atomically.
3. Remounts Checkout Elements using a revision-based React key.
4. Keeps the modal open and restores focus to the promotion-code status/control.
5. Re-enables **Continue** only after the new Stripe actions and Payment Element are ready.

If the API rejects the code before replacement, the current mounted Elements instance remains active.

## Error Handling

Stable errors include:

- `promotion_code_invalid`
- `promotion_code_ineligible`
- `promotion_code_ambiguous`
- `checkout_context_invalid`
- `checkout_context_expired`
- `checkout_revision_conflict`
- `checkout_already_completed`
- `checkout_replacement_failed`

Validation errors are shown inline next to the promotion-code control. Revision conflicts trigger one controlled refresh from the returned latest state. Completed checkout closes or transitions the modal to the existing completion flow. Replacement failures keep or recover a payable Session whenever possible and expose a retry action.

## Testing Strategy

### Backend

- Top-up, recurring subscription, and one-time subscription initial responses contain signed context, revision, and discount state.
- Apply a valid global code to orders whose original discount is none, invitation, or recall.
- Apply a valid customer-restricted code and reject it for another customer.
- Reject invalid, inactive, expired, product-ineligible, minimum-amount-ineligible, and ambiguous codes without expiring the current Session.
- Applying a second manual code replaces the first.
- Restore returns to none, invitation, or recall as appropriate.
- Every replacement Session contains at most one discount.
- Stale revision, idempotent retry, payment/expiration race, replacement failure, and recovery paths.
- Webhook fulfillment is single-shot and attributes only the paid Session's active discount.
- Invitation or recall benefits are not consumed when a manual promotion code wins.
- Raw promotion codes do not appear in application logs.

### Frontend

- The control appears between summary and **Continue** for all purchase kinds.
- Enter and **Apply** submit the normalized code once.
- Loading disables input, actions, and **Continue**.
- Apply success updates totals and shows direct-replacement status without a second confirmation.
- Invalid apply leaves the current Session and totals unchanged.
- Remove restores the original automatic discount.
- A successful response destroys and remounts Checkout Elements exactly once.
- Stale responses cannot overwrite the latest revision.
- Checkout recovery and completion errors use the correct UI state.

### Verification

- Targeted Go and frontend unit tests.
- Frontend typecheck and lint for changed files.
- Production frontend build.
- Responsive browser verification at 1440x900 and 390x844 with no horizontal overflow.
- Staging smoke test using Stripe test-mode promotion codes for none, invitation, and recall starting states.

## Acceptance Criteria

- Every Flatkey Stripe Checkout Elements purchase shows one promotion-code input.
- Eligible Stripe promotion codes apply and update the displayed discount and total.
- Ineligible codes show an inline error and leave the payable Session unchanged.
- A manual code replaces rather than stacks with invitation or recall discounts.
- Removing the manual code restores the original automatic discount.
- Session switching cannot create a second internal order or duplicate fulfillment.
- Campaign and invitation accounting reflect the discount on the paid Session.
- Desktop and 390 px mobile layouts remain readable and have no horizontal overflow.

## Rollout

Ship behind a server-controlled feature flag. Enable in Stripe test mode first, run the staging smoke matrix, and monitor checkout-revision failures, Session-expiration failures, promotion-code rejection rates, and webhook/order mismatches. Roll back by disabling the flag; initial checkout creation and existing automatic discounts continue to work without exposing the promotion-code control.
