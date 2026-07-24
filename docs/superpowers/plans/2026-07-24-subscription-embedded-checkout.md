# Subscription Embedded Checkout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route every Stripe-backed wallet plan purchase through the wallet's existing Stripe Embedded Checkout dialog while preserving balance payments, webhook settlement, idempotent replay, pending-order supersession, and hosted redirect fallback.

**Architecture:** The backend treats `ui_mode` as presentation-only request data and returns a Checkout client secret plus the existing Stripe publishable key for embedded sessions. The frontend extends the existing `usePayment` state and `StripeEmbeddedCheckoutDialog`; `SubscriptionPlansCard` delegates all Stripe response handling to that shared opener, so `stripe-embedded-checkout-dialog.tsx` remains the only Stripe.js mount/unmount implementation. Embedded sessions persist only the Stripe Session ID when Stripe supplies no hosted URL, and same-request replay reconstructs the response by retrieving that Session.

**Tech Stack:** Go 1.22+, Gin, GORM, stripe-go v86, React 19, TypeScript 6, Bun test, Stripe.js Embedded Checkout.

---

## File Structure

- Modify `service/subscription_invoice.go`: define the subscription Checkout presentation value, build hosted versus embedded recurring Session parameters, return client secret, and expose the existing Session getter to replay code.
- Modify `service/subscription_contract.go`: carry presentation through recurring checkout creation, allow Session-ID-only persistence, and hydrate embedded replay from Stripe by Session ID.
- Modify `service/subscription_purchase.go`: carry presentation and embedded result fields between the self-purchase service and controller.
- Modify `service/subscription_invoice_test.go`: lock recurring embedded parameters, hosted compatibility, URL-less persistence, and replay retrieval.
- Modify `service/subscription_purchase_test.go`: lock propagation of embedded presentation/results through the purchase facade.
- Modify `controller/subscription_payment_stripe.go`: create and retrieve one-time Alipay/Pix/UPI embedded Sessions while preserving method and local-currency quote snapshots.
- Modify `controller/subscription_self_purchase.go`: accept `ui_mode`, return embedded response fields, and replay a persisted one-time Session instead of creating another.
- Modify `controller/subscription_one_time_stripe_test.go`: lock one-time hosted/embedded parameter behavior for Alipay, Pix, and UPI.
- Modify `controller/subscription_self_purchase_test.go`: lock embedded API response, same-request replay, hosted fallback, and direct balance behavior.
- Modify `web/default/src/features/subscriptions/types.ts`: add the optional embedded request/response fields.
- Modify `web/default/src/features/wallet/types.ts`: define payment-purpose-neutral Stripe Checkout response/presentation types while retaining the top-up summary type.
- Modify `web/default/src/features/wallet/hooks/use-payment.ts`: add one generic Stripe Checkout opener backed by the existing `embeddedCheckout` state; keep top-up as a thin adapter.
- Modify `web/default/src/features/wallet/components/dialogs/stripe-embedded-checkout-dialog.tsx`: make title/description/fallback presentation configurable without changing its single mount/destroy lifecycle.
- Modify `web/default/src/features/wallet/components/subscription-plans-card.tsx`: request embedded mode for non-balance choices and delegate responses to the shared opener.
- Modify `web/default/src/features/wallet/index.tsx`: pass the existing shared opener to `SubscriptionPlansCard`; keep one dialog render.
- Modify `web/default/src/features/wallet/lib/subscription-plan-lifecycle.ts`: include `ui_mode: 'embedded'` only for Stripe-backed flexible purchase requests.
- Modify `web/default/src/features/wallet/components/subscription-plans-card.test.tsx`: lock request routing, embedded preference, and hosted fallback delegation.
- Modify `web/default/src/features/wallet/wallet-layout.test.ts`: lock the single renderer/mount architecture.

### Task 1: Recurring Stripe Checkout presentation and replay

**Files:**
- Modify: `service/subscription_invoice.go`
- Modify: `service/subscription_contract.go`
- Modify: `service/subscription_purchase.go`
- Test: `service/subscription_invoice_test.go`
- Test: `service/subscription_purchase_test.go`

- [ ] **Step 1: Write failing recurring embedded parameter tests**

Add tests that call a testable recurring parameter builder with an embedded presentation and assert:

```go
require.Equal(t, string(stripe.CheckoutSessionUIModeEmbeddedPage), *params.UIMode)
require.Contains(t, *params.ReturnURL, "session_id={CHECKOUT_SESSION_ID}")
require.Contains(t, *params.ReturnURL, "trade_no="+input.TradeNo)
require.Nil(t, params.SuccessURL)
require.Nil(t, params.CancelURL)
```

Add the hosted counterpart:

```go
require.Nil(t, params.UIMode)
require.Nil(t, params.ReturnURL)
require.NotNil(t, params.SuccessURL)
require.NotNil(t, params.CancelURL)
```

- [ ] **Step 2: Run the new recurring tests and verify RED**

Run:

```powershell
go test ./service -run 'Test.*StripeSubscriptionCheckout.*(Embedded|Hosted)' -count=1
```

Expected: FAIL because the presentation field/builder and embedded response fields do not exist yet.

- [ ] **Step 3: Implement the minimal recurring presentation contract**

Add a normalized value carried by `PurchaseSubscriptionCommand`, `ChangePlanCommand`, and `StripeSubscriptionCheckoutInput`:

```go
type StripeCheckoutPresentation struct {
    UIMode string
}

func (p StripeCheckoutPresentation) Embedded() bool {
    return strings.EqualFold(strings.TrimSpace(p.UIMode), "embedded")
}
```

Build recurring parameters so embedded sessions set `UIMode` and `ReturnURL`, hosted sessions keep `SuccessURL` and `CancelURL`. Extend `StripeSubscriptionCheckoutSession`, `ChangePlanResult`, and `PurchaseSubscriptionResult` with `ClientSecret`. Require `ClientSecret` only for embedded creation; allow an empty embedded `URL`.

- [ ] **Step 4: Write failing recurring replay and persistence tests**

Create an awaiting-payment recurring order with `ProviderSessionId` set and `ProviderSessionURL` empty. Stub `stripeCheckoutSessionGetter` to return:

```go
&stripe.CheckoutSession{
    ID: "cs_existing",
    ClientSecret: "cs_existing_secret_replay",
    Status: stripe.CheckoutSessionStatusOpen,
}
```

Assert same-request replay returns that client secret and does not call `stripeSubscriptionCheckoutCreator`. Add a persistence test proving `persistStripeCheckoutSession(intentID, "cs_existing", "")` succeeds and retains the Session ID.

- [ ] **Step 5: Run replay tests and verify RED**

Run:

```powershell
go test ./service -run 'Test.*(EmbeddedReplay|PersistStripeCheckoutSessionWithoutURL)' -count=1
```

Expected: FAIL because current replay relies on `ProviderSessionURL` and persistence requires both ID and URL.

- [ ] **Step 6: Implement Session-ID replay hydration**

Persist a non-empty Session ID even when URL is empty. On embedded replay, retrieve the persisted Session ID through `stripeCheckoutSessionGetter`, require a matching ID and non-empty client secret, and copy the secret plus any returned URL into the result. Keep all authority in the database intent/order and Stripe; do not store the client secret or use process-local state.

- [ ] **Step 7: Run recurring service tests and verify GREEN**

Run:

```powershell
go test ./service -run 'Test.*(StripeSubscriptionCheckout|EmbeddedReplay|PersistStripeCheckoutSession|PurchaseSubscriptionStripeRecurring)' -count=1
```

Expected: PASS.

### Task 2: One-time Alipay, Pix, and UPI embedded Checkout

**Files:**
- Modify: `controller/subscription_payment_stripe.go`
- Modify: `controller/subscription_self_purchase.go`
- Test: `controller/subscription_one_time_stripe_test.go`
- Test: `controller/subscription_self_purchase_test.go`

- [ ] **Step 1: Write failing one-time embedded parameter tests**

For Alipay, Pix, and UPI, build an order with the authoritative method/currency/minor amount and assert embedded presentation changes only Checkout UI fields:

```go
require.Equal(t, string(stripe.CheckoutSessionUIModeEmbeddedPage), *params.UIMode)
require.NotNil(t, params.ReturnURL)
require.Nil(t, params.SuccessURL)
require.Nil(t, params.CancelURL)
require.Equal(t, expectedMethod, *params.PaymentMethodTypes[0])
require.Equal(t, expectedCurrency, *params.LineItems[0].PriceData.Currency)
require.Equal(t, expectedAmount, *params.LineItems[0].PriceData.UnitAmount)
```

- [ ] **Step 2: Run one-time parameter tests and verify RED**

Run:

```powershell
go test ./controller -run 'TestBuildOneTimePlanCheckoutSessionParams.*Embedded' -count=1
```

Expected: FAIL because the builder does not accept presentation and always sets hosted URLs.

- [ ] **Step 3: Implement one-time embedded creation**

Pass the shared `service.StripeCheckoutPresentation` into `buildOneTimePlanCheckoutSessionParams` and `createOneTimeStripeCheckoutSession`. For embedded mode with a configured publishable key, set embedded Stripe fields and return `ClientSecret`; otherwise build the existing hosted Session. Keep payment method, quote currency, amount, metadata, and idempotency key unchanged.

- [ ] **Step 4: Write failing self-purchase response and replay tests**

Post a request containing:

```json
{"plan_id":2,"payment_choice":"pix","months":3,"request_id":"req-embedded","quote_id":"signed","ui_mode":"embedded"}
```

Assert the response contains `client_secret` and `publishable_key`. Seed a second same-request attempt whose order already has a Session ID but no URL; stub the one-time Session getter and assert the creator is not called. Add hosted-mode and balance cases proving the former still returns `checkout_url` and the latter returns no Stripe fields.

- [ ] **Step 5: Run self-purchase tests and verify RED**

Run:

```powershell
go test ./controller -run 'TestPurchaseSubscriptionSelf.*(Embedded|Replay|Hosted|Balance)' -count=1
```

Expected: FAIL because `ui_mode` and embedded response fields are absent.

- [ ] **Step 6: Implement controller request/response and replay**

Add `UIMode`, `ClientSecret`, and `PublishableKey` to the self-purchase boundary. Resolve embedded mode only when `StripePublishableKey` is configured, retrieve an existing persisted Session before creating a new one, and allow an embedded session whose URL is empty. Continue calling `SyncSubscriptionOrderTopUpHistory`; webhook settlement remains unchanged.

- [ ] **Step 7: Run one-time controller tests and verify GREEN**

Run:

```powershell
go test ./controller -run 'Test.*(OneTimePlanCheckout|PurchaseSubscriptionSelf)' -count=1
```

Expected: PASS.

### Task 3: One shared frontend opener and dialog

**Files:**
- Modify: `web/default/src/features/subscriptions/types.ts`
- Modify: `web/default/src/features/wallet/types.ts`
- Modify: `web/default/src/features/wallet/hooks/use-payment.ts`
- Modify: `web/default/src/features/wallet/components/dialogs/stripe-embedded-checkout-dialog.tsx`
- Modify: `web/default/src/features/wallet/lib/subscription-plan-lifecycle.ts`
- Modify: `web/default/src/features/wallet/components/subscription-plans-card.tsx`
- Modify: `web/default/src/features/wallet/index.tsx`
- Test: `web/default/src/features/wallet/components/subscription-plans-card.test.tsx`
- Test: `web/default/src/features/wallet/wallet-layout.test.ts`

- [ ] **Step 1: Write failing request and response-routing tests**

Extend the pure request-builder tests:

```ts
expect(buildFlexiblePurchaseRequest({ paymentChoice: 'pix', ...base })).toMatchObject({
  payment_choice: 'pix',
  ui_mode: 'embedded',
})
expect(buildFlexiblePurchaseRequest({ paymentChoice: 'balance', ...base })).not.toHaveProperty('ui_mode')
```

Add tests for the shared Stripe action resolver:

```ts
expect(resolveStripeCheckoutAction({ client_secret: 'cs_secret', publishable_key: 'pk_test' }).kind).toBe('embedded')
expect(resolveStripeCheckoutAction({ checkout_url: 'https://checkout.stripe.test/x' }).kind).toBe('hosted')
expect(resolveStripeCheckoutAction({ hosted_invoice_url: 'https://invoice.stripe.test/x' }).kind).toBe('hosted')
```

- [ ] **Step 2: Run the frontend tests and verify RED**

Run from `web/default`:

```powershell
bun test src/features/wallet/components/subscription-plans-card.test.tsx src/features/wallet/wallet-layout.test.ts
```

Expected: FAIL because flexible requests lack `ui_mode` and no shared flat-response resolver exists.

- [ ] **Step 3: Implement payment-purpose-neutral shared state**

Add `ui_mode?: 'embedded'` and optional `client_secret` / `publishable_key` response fields. Generalize the existing session without creating another hook:

```ts
export interface StripeEmbeddedCheckoutSession {
  clientSecret: string
  publishableKey: string
  title: string
  description: string
  summary: StripeTopupSummary | null
  fallbackUrl: string | null
}
```

The existing `usePayment` owns the only `embeddedCheckout` state and exposes one generic opener. Top-up remains a thin adapter that supplies its bonus summary; subscriptions supply neutral title/description and no summary. The opener prefers `client_secret` plus `publishable_key`, then accepts `pay_link`, `checkout_url`, or `hosted_invoice_url` as hosted fallback.

- [ ] **Step 4: Wire subscription purchase to the shared opener**

Make `SubscriptionPlansCard` receive the opener from `Wallet`. For `checkout_required` or `payment_action_required`, remember the return poll, close the selection dialog, and call the shared opener. Remove direct `window.location.assign` from `SubscriptionPlansCard`; hosted navigation now stays in the shared opener. Balance success continues through refresh/success handling and never sends `ui_mode`.

- [ ] **Step 5: Generalize only the existing dialog**

Read title and description from the shared session. Preserve the existing `loadStripe -> createEmbeddedCheckoutPage -> mount -> destroy` effect. If mounting fails and `fallbackUrl` exists, navigate to it; otherwise close and show the existing recoverable error. Do not create another renderer, dialog, or Stripe hook.

- [ ] **Step 6: Add the single-mount architecture regression**

Extend the source-level wallet test to scan `src/features/wallet` and assert:

```ts
expect(filesContaining('createEmbeddedCheckoutPage')).toEqual([
  'components/dialogs/stripe-embedded-checkout-dialog.tsx',
])
expect(subscriptionPlansSource).not.toContain('window.location.assign')
expect(walletSource).toContain('openStripeCheckout={openStripeCheckout}')
```

- [ ] **Step 7: Run frontend tests and typecheck and verify GREEN**

Run from `web/default`:

```powershell
bun test src/features/wallet/components/subscription-plans-card.test.tsx src/features/wallet/wallet-layout.test.ts src/features/wallet/lib/stripe-payment-request.test.ts
bun run typecheck
```

Expected: PASS.

### Task 4: Integrated verification and delivery

**Files:**
- Review all files changed by Tasks 1-3.

- [ ] **Step 1: Run targeted backend regression suites**

```powershell
go test ./controller -run 'Test.*(Stripe|SubscriptionSelf|OneTimePlan)' -count=1
go test ./service -run 'Test.*(Stripe|Subscription)' -count=1
```

Expected: PASS.

- [ ] **Step 2: Run the wallet regression suite and build checks**

From `web/default`:

```powershell
bun test src/features/wallet/api.test.ts src/features/wallet/lib/stripe-payment-request.test.ts src/features/wallet/lib/subscription-plan-lifecycle.test.ts src/features/wallet/components/subscription-plans-card.test.tsx src/features/wallet/wallet-layout.test.ts
bun run typecheck
bun run build
```

Expected: PASS.

- [ ] **Step 3: Verify the single Stripe renderer invariant**

```powershell
rg -n "createEmbeddedCheckoutPage|\.mount\(|\.destroy\(" web/default/src/features/wallet
```

Expected: Stripe Embedded Checkout lifecycle calls appear only in `components/dialogs/stripe-embedded-checkout-dialog.tsx`.

- [ ] **Step 4: Review the diff for scope and secrets**

```powershell
git diff --check
git status --short
git diff --stat
```

Expected: no whitespace errors; `.gitnexus/` remains untracked and is not staged; no Stripe secret key or client secret is logged or persisted.

- [ ] **Step 5: Commit with the Lore protocol**

Stage only the plan and implementation files, then commit with an intent-first message and `Constraint`, `Rejected`, `Confidence`, `Scope-risk`, `Directive`, `Tested`, and `Not-tested` trailers. Do not stage `.gitnexus/`.

- [ ] **Step 6: Push for staging and production review**

Merge/push the verified commit to the remote `staging` branch to trigger staging deployment. Keep the feature branch and open or update the production PR against `main`; do not create new Stripe Prices or change webhook configuration.
