# Subscription Embedded Checkout Design

Date: 2026-07-24
Status: Approved
Scope: Customer wallet subscription purchases only

## Goal

Keep customers on the Flatkey wallet page while they pay for a subscription plan. Reuse the existing Stripe Embedded Checkout implementation used by wallet top-ups instead of introducing a second Stripe mounting component or a custom payment form.

## Decisions

- Stripe recurring, Alipay, Pix, and UPI plan purchases use Stripe Embedded Checkout inside the existing wallet dialog system.
- Flatkey balance purchases remain immediate server-side payments and never open Stripe.
- Existing recurring-plan upgrades that return `hosted_invoice_url` keep the current full-page redirect as a compatibility fallback.
- A Stripe-hosted `checkout_url` remains the recovery fallback when the shared embedded renderer cannot initialize or when billing history reopens an unpaid order.
- Redirect-based authorization required by Alipay, Pix, or UPI may temporarily leave the page or open an external application. Stripe returns the buyer to the wallet afterward.
- Stripe webhooks remain the only authority that activates a paid plan. Closing or completing a frontend dialog never grants entitlement directly.
- There will be one shared Stripe Embedded Checkout renderer and one Stripe.js mount/unmount lifecycle. Top-ups and subscription purchases provide presentation text and optional summary data to that renderer; no subscription-specific renderer, mount hook, or legacy parallel dialog remains.

## Non-goals

- Replacing Stripe Checkout with Payment Element.
- Embedding Stripe Hosted Invoice pages.
- Changing configured plan prices, currencies, Price IDs, webhook signing, or entitlement settlement rules.
- Adding subscription cancellation or future-month refund controls.
- Automatically deleting payment history rows. Superseded orders remain auditable with terminal statuses.

## Existing Reusable Implementation

The current top-up flow already provides the required pattern:

1. The client requests `ui_mode: "embedded"`.
2. The backend creates a Checkout Session with `ui_mode=embedded_page` and a `return_url`.
3. The backend returns `client_secret` and `publishable_key`.
4. The wallet stores an embedded-session view model.
5. `StripeEmbeddedCheckoutDialog` loads Stripe.js, creates the Embedded Checkout instance, mounts it, and destroys it when the dialog closes.
6. If embedded mode is unavailable, the top-up flow can still consume a hosted URL.

Subscription purchases will use the same renderer and the same Stripe publishable-key setting.

## API Contract

### Request

`POST /api/subscription/self/purchase` adds an optional presentation field:

```json
{
  "plan_id": 2,
  "payment_choice": "pix",
  "months": 3,
  "request_id": "...",
  "quote_id": "...",
  "ui_mode": "embedded"
}
```

`ui_mode` affects only Checkout presentation. It must not affect pricing, idempotency, intent kind, settlement, or entitlement snapshots.

### Embedded response

For Stripe recurring and Stripe-backed one-time methods, a successful embedded session response contains:

```json
{
  "status": "checkout_required",
  "client_secret": "cs_..._secret_...",
  "publishable_key": "pk_...",
  "checkout_url": "https://checkout.stripe.com/..."
}
```

`checkout_url` remains available as a recovery fallback and for billing-history reopening. The frontend prefers `client_secret` plus `publishable_key` whenever both exist.

For `payment_action_required` responses containing only `hosted_invoice_url`, the frontend keeps the existing full-page redirect.

### Balance response

Balance payments do not return Stripe fields. The existing applied/success response remains unchanged.

## Backend Design

### Shared presentation model

Introduce a small Checkout presentation value shared by recurring and one-time subscription session creators:

- requested UI mode;
- return URL for embedded sessions;
- hosted success and cancel URLs for hosted sessions.

Do not share top-up pricing or order builders with subscription code. Only the Stripe Checkout presentation behavior is shared or mirrored through one narrowly named helper.

### Recurring Checkout

When `ui_mode=embedded` and `StripePublishableKey` is configured:

- set `UIMode` to `embedded_page`;
- set `ReturnURL` to the subscription wallet return route;
- omit `SuccessURL` and `CancelURL` because Stripe rejects them for embedded sessions;
- accept a Session with a non-empty ID and client secret;
- persist Session ID and hosted URL exactly as today;
- return client secret, publishable key, and hosted URL.

Hosted mode keeps the current subscription-mode Checkout behavior.

### One-time Alipay, Pix, and UPI Checkout

Use the same presentation decision while preserving:

- `mode=payment`;
- exactly one configured payment method;
- quote-backed BRL or INR amount snapshots;
- existing authoritative order metadata and webhook settlement validation.

The embedded `return_url` includes Stripe's `{CHECKOUT_SESSION_ID}` token and the Flatkey trade number so the wallet can recover state after an external authorization round trip.

### Replay and supersession

- Replaying the same `request_id` returns the existing session's presentation data instead of creating another Session.
- A different request supersedes every replaceable unpaid Checkout Session before creating the new one.
- The existing transitions remain authoritative: old Stripe Session `expired`, order `expired`, intent `superseded`, and related history synchronized.

Persist the Stripe Session client secret only if replay requires it and storage is permitted by the existing data model. Prefer retrieving the open Session by its persisted Session ID when reconstructing an embedded replay response, so no new secret-bearing database column is introduced.

### Multi-node behavior

Presentation mode is request data, not process-local state. Replay and supersession continue to use the existing database-backed intent and order transactions, so concurrent console instances cannot rely on in-memory dialog/session ownership. Stripe Session retrieval is keyed by the persisted Session ID, and webhook reconciliation remains idempotent across nodes.

## Frontend Design

### One renderer

Generalize the existing `StripeEmbeddedCheckoutDialog` rather than copying it. Its session model becomes payment-purpose neutral:

- `clientSecret`;
- `publishableKey`;
- configurable title;
- configurable description;
- optional summary content.

Top-up continues to supply its bonus summary. Subscription purchase supplies plan/payment context or no summary. Stripe.js loading, mounting, cleanup, responsive width, and error handling stay in the shared renderer.

### Subscription purchase flow

1. The buyer selects a plan, payment choice, and month count.
2. The wallet sends `ui_mode: "embedded"` for Stripe recurring, Alipay, Pix, and UPI.
3. Balance payments use the existing direct path.
4. When the response has `client_secret` and `publishable_key`, close the purchase-selection dialog and open the shared Embedded Checkout dialog.
5. When the response has only `hosted_invoice_url`, remember the external return and use the existing full-page redirect.
6. When embedded initialization fails but a hosted Checkout URL exists, show a clear error with a retry/open-payment action; do not silently grant or mutate the plan.

### Completion and return

- Checkout completion and external-method returns land on the customer wallet route.
- The wallet refreshes the current subscription and billing history using the existing return polling behavior.
- A completed webhook changes the plan state; the frontend only reflects the refreshed server state.

### Closing the dialog

Closing the dialog destroys only the Stripe.js UI instance. It does not mark the order paid or cancel a potentially active external authorization. The order remains pending until Stripe expiration/webhook reconciliation or until a newer request supersedes it.

## Error Handling

- Missing Stripe publishable key: backend returns hosted Checkout presentation, preserving the ability to pay.
- Missing embedded client secret: return a payment creation error; do not return a false success.
- Stripe.js load or mount failure: close the broken renderer and present a retry/open-hosted-payment action.
- Paid or completed prior Session: reject supersession to prevent double purchase.
- Expired prior Session: synchronize terminal local state and allow a new request.
- `hosted_invoice_url`: use the approved full-page redirect fallback.

## Security and Correctness

- Never expose the Stripe secret key.
- The publishable key and Checkout client secret are safe client-facing Stripe values but must not be logged unnecessarily.
- Preserve request-id idempotency and signed quote validation.
- Preserve immutable currency, amount, plan snapshot, and purchase intent metadata.
- Never activate entitlements from a redirect query parameter or dialog callback.
- Webhook validation and database reconciliation remain unchanged and authoritative.

## Testing

### Backend

- Recurring embedded Session uses `UIMode=embedded_page`, `ReturnURL`, and no success/cancel URLs.
- One-time Alipay, Pix, and UPI embedded Sessions preserve payment method and local-currency amounts.
- Embedded API response returns client secret, publishable key, and hosted recovery URL.
- Hosted response remains backward compatible.
- Same-request replay returns the existing embedded session.
- New-request supersession expires the previous pending session.
- Missing publishable key falls back to hosted presentation.

### Frontend

- Subscription purchase requests embedded mode for every Stripe-backed choice and not for balance.
- A client-secret response opens the shared dialog and does not call `window.location.assign`.
- A hosted-invoice response uses the existing redirect fallback.
- Top-up still uses the same generalized dialog and preserves its bonus banner.
- Closing destroys the mounted Checkout instance without changing entitlement state.
- Mount failures show a recoverable error.

### Staging smoke test

- Stripe recurring opens inside the wallet.
- Alipay, Pix, and UPI open inside the wallet and can return from required external authorization.
- Closing an unpaid dialog and starting a new purchase leaves only the newest order pending.
- Balance purchase remains immediate.
- Billing history can reopen the hosted recovery URL for an unpaid order.

## Deployment and Configuration

- No new Stripe Price IDs are required.
- No database migration is expected unless replay cannot reconstruct the client secret from the persisted Session ID.
- `StripePublishableKey` must be configured; the current wallet top-up implementation already uses this setting.
- Deploy the console/backend service because it serves both the API and embedded wallet frontend.
- Validate on staging before merging the production PR.

## Acceptance Criteria

- All ordinary Stripe-backed plan purchases open the existing shared Stripe dialog inside the wallet.
- No second Stripe Embedded Checkout mounting component exists.
- Top-up behavior remains unchanged after the renderer is generalized.
- Balance payment remains direct.
- Stripe-hosted Checkout or Invoice URLs remain the only approved full-page payment fallbacks; no second in-app payment implementation is retained.
- Webhooks remain the sole source of paid-plan activation.
