# Wallet coupon presentation and Stripe promotion-code entry

## Goal

Make wallet discounts easier to scan and allow the custom Stripe Elements dialog to accept a customer-entered promotion code.

## Design

### Wallet discount row

- Remove the campaign/source label from subscription and top-up cards.
- Render the discount badge as a percentage label such as `20% OFF` when the quote has a measurable percentage; keep a generic `OFF` fallback for discounts whose percentage cannot be derived.
- Place the badge, `Save …`, and `Expires …` in one wrapping flex row beneath the discounted price. Recall expiry is rendered from the offer expiry timestamp; invitation discounts have no expiry label.
- Keep the original amount struck through and preserve the existing mobile wrapping behavior.

### Stripe promotion-code entry

- Expose Stripe Checkout Elements actions for applying and removing promotion codes through the existing `MountedStripeCheckoutElements` adapter.
- Add a controlled promotion-code input with Apply/Remove controls to the custom Stripe checkout form. The dialog updates its session view model from the action result so totals and discount lines refresh immediately.
- Show Stripe's returned error message inline and prevent empty submissions. After a successful application, keep the code visible and offer Remove.
- Enable `allow_promotion_codes` on new subscription Checkout Sessions only when no server-selected Recall or invitation discount is present. Stripe Checkout supports one coupon/promotion code per session; automatic discounts remain authoritative and are not stacked with a manually entered code.

## Error handling

- Empty code: show a local validation message and do not call Stripe.
- Stripe rejection: show the returned message inline and keep the current session totals unchanged.
- Mount/confirm behavior remains unchanged.

## Verification

- Frontend tests cover percentage/expiry markup and Stripe action exposure, plus promotion-code success/error UI contracts.
- Go tests assert ordinary subscription sessions enable promotion codes while Recall/invitation sessions continue to send their single automatic discount.
- Run targeted Bun tests, Go package tests, TypeScript typecheck, and the frontend build/check before delivery.
