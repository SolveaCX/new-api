# Visual Ralph reference: Stripe Checkout layout

- Source URL: `http://127.0.0.1:61895/reference-exact-v2.html`
- Source file: `E:/workspace/.superpowers/brainstorm/flatkey-stripe-layout-20260819-1/content/reference-exact-v2.html`
- Scope/permission: local reference created for this Flatkey task and explicitly approved by the user with “照着这个干”.
- Desktop reference: `reference-1440x900.png`
- Mobile reference: `reference-390x844.png`
- Target surface: authenticated console Stripe payment dialog shared by wallet top-ups and subscription purchases.
- Seed/login assumptions: implementation preview requires an authenticated local console user and a Stripe test-mode Checkout Session client secret.
- Interaction parity: close, responsive reflow, Stripe-rendered payment fields, dynamic order summary, confirm button, loading/error/disabled states.
- Exclusions: no change to Stripe account configuration, order state machines, webhook behavior, payment method availability, authentication, or third-party hosted redirect pages.

## Capture procedure

1. Open the reference URL.
2. Capture the viewport at 1440×900 to `reference-1440x900.png`.
3. Capture the full mobile page at 390×844 to `reference-390x844.png`.
4. For the implementation, open the wallet Stripe dialog with a test-mode Checkout Session in the same viewports and save `actual-1440x900.png` and `actual-390x844.png`.
5. Run the Visual Ralph verdict before every subsequent visual edit. Passing score: 90 or higher.

