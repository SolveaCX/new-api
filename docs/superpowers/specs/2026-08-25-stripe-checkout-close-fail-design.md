# Stripe checkout close terminates unpaid purchase

## Goal

When a user closes the in-console Stripe Checkout Elements dialog, the related
unpaid Stripe purchase must stop being recoverable as `pending`. The wallet no
longer presents pending purchases, so leaving the local order pending strands
the purchase and any locally reserved invitation discount.

## Scope

The behavior applies to all in-console Stripe checkout purchases:

- wallet top-ups;
- recurring subscription purchases; and
- one-time subscription purchases.

Hosted Stripe redirects are unchanged because they do not expose this dialog's
close control.

## Design

The backend exposes an authenticated close endpoint that accepts the purchase
trade number. The frontend carries that trade number in the Stripe checkout
session data and calls the endpoint as a fire-and-forget action when the dialog
closes.

The endpoint is idempotent and performs these checks in order:

1. Load the top-up or subscription order by trade number and verify ownership,
   Stripe as the payment provider, and a `pending` local status.
2. Fetch the provider Checkout Session and re-check provider truth. A paid or
   complete Session is never failed locally; its webhook remains authoritative.
3. If the Session is still open, expire it through Stripe. If it is already
   expired or otherwise unpaid, no second provider mutation is needed.
4. Mark the local order `failed` through the existing purchase lifecycle.
   Subscription termination also releases any invitation-discount reservation,
   updates the pending change intent, and syncs the top-up history mirror.
   Top-up termination also marks a requested payment invoice failed.

If provider lookup or expiration cannot establish that the Session is unpaid,
the endpoint leaves the order pending and returns an error. This avoids turning
a payment race or a provider outage into a false failure. A concurrent paid
webhook wins over a close request.

The frontend closes immediately and does not poll or wait for the order to
become pending/failed. A failed close request is logged through the existing
payment error boundary without blocking the dialog dismissal.

## Coupon guarantees

- Invitation discount credits are released by the subscription termination
  transaction and can be used again.
- Manual Stripe promotion codes are not consumed by an unpaid Checkout Session;
  closing the Session does not redeem or invalidate the code.
- Checkout revision records are abandoned/retired by the existing lifecycle
  paths, so a subsequent purchase can create a fresh revision and reapply a
  discount.

## Verification

- Backend tests cover ownership, missing orders, already-paid sessions, open
  session expiration, idempotent repeated close, all three purchase kinds, and
  discount/invoice cleanup.
- Frontend tests cover passing the trade number, invoking close without blocking
  dismissal, and preserving the existing promotion control state.
- Run targeted Go tests, wallet/subscription Bun tests, typecheck, and
  `git diff --check`.
