# Stripe Checkout Promotion Code Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add one promotion-code control to every Flatkey Stripe Checkout Elements purchase, replacing invitation/recall discounts with a buyer-entered Stripe Promotion Code and restoring the canonical automatic discount when removed.

**Architecture:** Initial Checkout Elements responses carry a signed purchase context, revision number, and normalized discount state. One authenticated endpoint prepares a candidate Session for the same internal order, lets Stripe validate it, expires the previous Session, atomically promotes the new revision, and returns a new client secret; the shared React dialog remounts Checkout Elements while keeping the modal open and fencing stale responses.

**Tech Stack:** Go 1.25.1, Gin, GORM, stripe-go v86, React 19, TypeScript 6, Bun test, Stripe.js Checkout Elements, Tailwind CSS 4.

## Global Constraints

- Stripe Checkout permits at most one coupon or promotion code per Session; manual and automatic discounts never stack.
- Applies to wallet top-ups, recurring subscriptions, and one-time subscription purchases using Checkout Elements.
- Applying a manual code directly replaces invitation/recall; removing it restores the original server-side discount.
- Stripe is authoritative for promotion eligibility; candidate Session creation is the final validation.
- The browser never supplies amount, currency, product, customer, or original-discount identity.
- Invalid/ineligible codes leave the current payable Session and totals unchanged.
- Mutations use signed context, expected revision, request idempotency, and database compare-and-swap.
- The paid Session's discount controls fulfillment attribution; manual must not consume invitation credit or record recall conversion.
- Raw buyer-entered promotion codes never appear in logs or idempotency keys.
- No new dependency and no private-preview Stripe dynamic-discount API.
- Desktop 1440x900 and mobile 390x844 must have no horizontal overflow.
- Delivery is guarded by setting.StripePromotionCodeEnabled; the client shows the input only when a complete revision contract exists.

**Design source:** docs/superpowers/specs/2026-08-20-stripe-promotion-code-checkout-design.md at commit 91ce9ca00.

---

## Locked API Contract

Endpoint:

~~~http
POST /api/user/stripe/checkout/discount
~~~

Request:

~~~go
type StripeCheckoutDiscountRequest struct {
    CheckoutContext  string `json:"checkout_context" binding:"required"`
    ExpectedRevision int64  `json:"expected_revision" binding:"required,min=1"`
    RequestID        string `json:"request_id" binding:"required"`
    Action           string `json:"action" binding:"required,oneof=apply restore"`
    PromotionCode    string `json:"promotion_code,omitempty"`
}
~~~

Apply includes action=apply and promotion_code. Restore includes action=restore and omits promotion_code. request_id is a browser-generated UUID/nanoid reused only for exact retry.

Successful response uses the repository envelope:

~~~json
{
  "success": true,
  "message": "success",
  "data": {
    "client_secret": "cs_test_next_secret_xxx",
    "publishable_key": "pk_test_xxx",
    "fallback_url": "https://checkout.stripe.test/session",
    "checkout_context": "base64url-payload.hex-signature",
    "checkout_revision": 2,
    "discount_state": {
      "source": "manual",
      "display_name": "SAVE20",
      "promotion_code_masked": "SAVE20",
      "replaced_source": "invitation"
    },
    "topup_summary": {
      "pay_amount": 30,
      "bonus_amount": 0,
      "credit_amount": 30,
      "show_amounts": true
    }
  }
}
~~~

Stable error codes:

~~~text
promotion_code_invalid
promotion_code_ineligible
promotion_code_ambiguous
checkout_context_invalid
checkout_context_expired
checkout_revision_conflict
checkout_already_completed
checkout_replacement_failed
stripe_promotion_disabled
~~~

`checkout_revision_conflict` returns HTTP 409 and includes `data` in the same `StripeCheckoutRevisionResponse` shape for the latest active Session. Other validation errors omit `data`, so the client can leave its current mounted Session untouched.

---

## File Structure

### Create

| File | Responsibility |
| --- | --- |
| service/stripe_checkout_context.go | Sign/verify short-lived mutation contexts and define purchase kinds. |
| service/stripe_checkout_discount.go | Shared discount source/selection type and exactly-one-discount parameter helper. |
| service/stripe_checkout_promotion.go | Resolve a unique active Stripe Promotion Code. |
| model/stripe_checkout_revision.go | Cross-purchase revision ledger and preparing/active/superseded/abandoned CAS. |
| controller/stripe_checkout_discount.go | Unified endpoint and shared candidate→expire→activate coordinator. |
| web/default/src/features/wallet/components/dialogs/stripe-promotion-code-control.tsx | Accessible apply/remove form and status UI. |
| web/default/src/features/wallet/components/dialogs/stripe-checkout-dialog.interaction.test.tsx | Remount, stale-response, apply/remove behavior. |

### Modify

| File | Responsibility |
| --- | --- |
| setting/payment_stripe.go, model/option.go | Server rollout flag. |
| model/main.go, model/topup.go, model/subscription.go | Migration and current revision pointers. |
| router/api-router.go | Authenticated/rate-limited route. |
| controller/topup_stripe.go | Initial context, top-up candidates, webhook identity. |
| controller/subscription_payment_stripe.go | One-time candidates and revision metadata. |
| controller/subscription_self_purchase.go | Flexible purchase response propagation. |
| service/subscription_invoice.go, service/subscription_contract.go | Recurring candidates and invoice validation. |
| service/recall_attribution.go | Attribute only the paid discount source. |
| wallet/types.ts, wallet/api.ts, stripe-checkout-opening.ts | Frontend API/revision contract. |
| stripe-checkout-dialog.tsx, stripe-checkout-layout.tsx | Revision lifecycle and input placement. |
| subscriptions/types.ts, subscription-purchase-dialog.tsx | Direct subscription propagation. |
| web/default/src/i18n/locales/*.json | New checkout copy. |

---

### Task 1: Feature Gate and Signed Checkout Context

**Files:**
- Modify: setting/payment_stripe.go
- Modify: model/option.go
- Create: service/stripe_checkout_context.go
- Test: service/stripe_checkout_context_test.go
- Test: model/option_test.go

**Interfaces:**
- Produces: StripeCheckoutPurchaseKind, StripeCheckoutContextClaims, SignStripeCheckoutContext, VerifyStripeCheckoutContext.
- Consumed by: initial response builders and UpdateStripeCheckoutDiscount.

- [ ] **Step 1: Write failing tests**

Create service/stripe_checkout_context_test.go:

~~~go
func TestStripeCheckoutContextRoundTripAndTamperFence(t *testing.T) {
    now := time.Unix(1_787_200_000, 0)
    claims := StripeCheckoutContextClaims{
        UserID: 19, PurchaseKind: StripeCheckoutPurchaseTopUp,
        TradeNo: "topup-19", Revision: 3,
        ExpiresAt: now.Add(15 * time.Minute).Unix(),
    }
    token, err := SignStripeCheckoutContext(claims)
    require.NoError(t, err)
    got, err := VerifyStripeCheckoutContext(token, now)
    require.NoError(t, err)
    require.Equal(t, claims, got)

    _, err = VerifyStripeCheckoutContext(token+"x", now)
    require.ErrorIs(t, err, ErrStripeCheckoutContextInvalid)
}

func TestStripeCheckoutContextRejectsExpired(t *testing.T) {
    now := time.Unix(1_787_200_000, 0)
    token, err := SignStripeCheckoutContext(StripeCheckoutContextClaims{
        UserID: 19, PurchaseKind: StripeCheckoutPurchaseOneTimeSubscription,
        TradeNo: "sub-19", Revision: 1, ExpiresAt: now.Add(-time.Second).Unix(),
    })
    require.NoError(t, err)
    _, err = VerifyStripeCheckoutContext(token, now)
    require.ErrorIs(t, err, ErrStripeCheckoutContextExpired)
}
~~~

Append TestStripePromotionCodeOption to model/option_test.go: call updateOptionMap(), updateOption("StripePromotionCodeEnabled", "true"), then assert the setting and OptionMap value.

- [ ] **Step 2: Run tests and verify red**

~~~powershell
go test ./service/ -run TestStripeCheckoutContext -v
go test ./model/ -run TestStripePromotionCodeOption -v
~~~

Expected: compilation fails because the types and flag do not exist.

- [ ] **Step 3: Implement minimal production code**

Add to setting/payment_stripe.go:

~~~go
var StripePromotionCodeEnabled = false
~~~

Wire it into updateOptionMap() and updateOption() in model/option.go using strconv.FormatBool and value == "true".

Create service/stripe_checkout_context.go with:

~~~go
type StripeCheckoutPurchaseKind string

const (
    StripeCheckoutPurchaseTopUp StripeCheckoutPurchaseKind = "topup"
    StripeCheckoutPurchaseRecurringSubscription StripeCheckoutPurchaseKind = "recurring_subscription"
    StripeCheckoutPurchaseOneTimeSubscription StripeCheckoutPurchaseKind = "one_time_subscription"
)

type StripeCheckoutContextClaims struct {
    UserID       int                        `json:"uid"`
    PurchaseKind StripeCheckoutPurchaseKind `json:"kind"`
    TradeNo      string                     `json:"trade_no"`
    Revision     int64                      `json:"revision"`
    ExpiresAt    int64                      `json:"expires_at"`
}
~~~

Sign base64url JSON with common.GenerateHMAC. Verify with hmac.Equal, reject unknown kind/empty trade/non-positive user or revision, and return ErrStripeCheckoutContextExpired when ExpiresAt <= now.Unix().

- [ ] **Step 4: Run tests and verify green**

~~~powershell
go test ./service/ -run TestStripeCheckoutContext -v
go test ./model/ -run TestStripePromotionCodeOption -v
~~~

Expected: PASS.

- [ ] **Step 5: Commit**

~~~powershell
git add setting/payment_stripe.go model/option.go model/option_test.go service/stripe_checkout_context.go service/stripe_checkout_context_test.go
git commit -m "Gate and authenticate Stripe checkout revisions" -m "Constraint: Checkout mutation identity remains server-authoritative." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: Context token and option round-trip tests."
~~~

---

### Task 2: Durable Revision Ledger and Cross-node CAS

**Files:**
- Create: model/stripe_checkout_revision.go
- Modify: model/topup.go
- Modify: model/subscription.go
- Modify: model/main.go
- Test: model/stripe_checkout_revision_test.go

**Interfaces:**
- Produces: PrepareStripeCheckoutRevision, RecordStripeCheckoutCandidate, ActivateStripeCheckoutRevision, AbandonStripeCheckoutRevision, GetStripeCheckoutRevisionByRequestID, ErrStripeCheckoutRevisionConflict.

- [ ] **Step 1: Write failing ledger tests**

Create model/stripe_checkout_revision_test.go:

~~~go
func TestStripeCheckoutRevisionPrepareIsIdempotentAndFenced(t *testing.T) {
    setupTopUpLifecycleTestDB(t, 1)
    topup := TopUp{
        UserId: 7, TradeNo: "t-7", GatewayTradeNo: "cs_old",
        Status: common.TopUpStatusPending, CheckoutRevision: 1,
    }
    require.NoError(t, DB.Create(&topup).Error)

    first, replay, err := PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
        OrderType: StripeCheckoutOrderTopUp, TradeNo: "t-7", UserID: 7,
        ExpectedRevision: 1, RequestID: "req-1",
        SelectionDigest: "sha256:manual-promo-1",
    })
    require.NoError(t, err)
    require.False(t, replay)
    require.Equal(t, int64(2), first.Revision)

    second, replay, err := PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
        OrderType: StripeCheckoutOrderTopUp, TradeNo: "t-7", UserID: 7,
        ExpectedRevision: 1, RequestID: "req-1",
        SelectionDigest: "sha256:manual-promo-1",
    })
    require.NoError(t, err)
    require.True(t, replay)
    require.Equal(t, first.Id, second.Id)

    _, _, err = PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
        OrderType: StripeCheckoutOrderTopUp, TradeNo: "t-7", UserID: 7,
        ExpectedRevision: 1, RequestID: "req-2",
        SelectionDigest: "sha256:manual-promo-2",
    })
    require.ErrorIs(t, err, ErrStripeCheckoutRevisionConflict)
}
~~~

Add TestActivateStripeCheckoutRevisionMovesPointerExactlyOnce: seed preparing revision 2 with cs_new, activate once, assert TopUp.GatewayTradeNo=cs_new and CheckoutRevision=2, then assert second/stale activation conflicts.

Add TestStripeCheckoutRevisionSkipsAbandonedRevision: abandon revision 2 while the active order remains revision 1, prepare a new request with expected revision 1, and assert the new row is revision 3 while the abandoned revision 2 remains immutable for audit.

- [ ] **Step 2: Run the focused test**

~~~powershell
go test ./model/ -run TestStripeCheckoutRevision -v
~~~

Expected: compilation fails because the ledger does not exist.

- [ ] **Step 3: Implement schema and CAS**

Add CheckoutRevision int64 with `gorm:"not null;default:0"` to TopUp and SubscriptionOrder. Existing pending Sessions remain revision `0` and keep the legacy webhook path; feature-capable initial Sessions are explicitly initialized to revision `1`. Register StripeCheckoutRevision in orderedMigrationModels().

Core schema:

~~~go
type StripeCheckoutRevision struct {
    Id                 int64  `gorm:"primaryKey"`
    OrderType          string `gorm:"type:varchar(32);not null;uniqueIndex:idx_stripe_checkout_revision;uniqueIndex:idx_stripe_checkout_request"`
    TradeNo            string `gorm:"type:varchar(128);not null;uniqueIndex:idx_stripe_checkout_revision;uniqueIndex:idx_stripe_checkout_request;index"`
    Revision           int64  `gorm:"not null;uniqueIndex:idx_stripe_checkout_revision"`
    UserId             int    `gorm:"not null;index"`
    RequestId          string `gorm:"type:varchar(64);not null;uniqueIndex:idx_stripe_checkout_request"`
    SelectionDigest    string `gorm:"type:varchar(96);not null"`
    State              string `gorm:"type:varchar(16);not null;index"`
    DiscountSource     string `gorm:"type:varchar(24);not null"`
    ReplacedSource     string `gorm:"type:varchar(24);not null;default:''"`
    CouponId           string `gorm:"type:varchar(128);default:''"`
    PromotionCodeId    string `gorm:"type:varchar(128);default:''"`
    PromotionCodeMask  string `gorm:"type:varchar(64);default:''"`
    DiscountPayload    string `gorm:"type:text"`
    Currency           string `gorm:"type:varchar(8);not null;default:''"`
    SubtotalMinor      int64  `gorm:"not null;default:0"`
    DiscountMinor      int64  `gorm:"not null;default:0"`
    TotalMinor         int64  `gorm:"not null;default:0"`
    ProviderSessionId  *string `gorm:"type:varchar(128);uniqueIndex"`
    ProviderSessionURL string `gorm:"type:text"`
    SummaryPayload     string `gorm:"type:text"`
    CreatedAt          int64  `gorm:"autoCreateTime"`
    UpdatedAt          int64  `gorm:"autoUpdateTime"`
}
~~~

States are `preparing`, `active`, `superseded`, and `abandoned`. Prepare runs in DB.Transaction, locks the owning order row with clause.Locking{Strength:"UPDATE"}, checks owner/status/current active revision, replays an exact order+request-id+digest match, and inserts `max(existing revision)+1`. Revision numbers are monotonic but may have gaps after abandonment; an abandoned row is immutable audit history and must never block a later request whose expected active revision is still valid. Activate uses WHERE checkout_revision=expected AND current session=old session, updates the order pointer/revision, marks the previous active row superseded, and marks the candidate active in one transaction. Abandon updates only preparing rows after candidate expiration. Do not persist Stripe client secrets; an exact replay retrieves the active Session by ProviderSessionId and reads its current client secret.

Revision `1` stores the canonical original selection and its signed server metadata. Restore always reconstructs from revision `1`, which is required for top-up recall because TopUp itself does not persist recall promotion identity.

- [ ] **Step 4: Run model regression**

~~~powershell
go test ./model/ -run "TestStripeCheckoutRevision|TestTopUpLifecycle|TestSubscriptionLifecycle" -v
~~~

Expected: PASS including stale CAS and exact replay.

- [ ] **Step 5: Commit**

~~~powershell
git add model/stripe_checkout_revision.go model/stripe_checkout_revision_test.go model/topup.go model/subscription.go model/main.go
git commit -m "Fence Stripe checkout replacements with durable revisions" -m "Constraint: Process-local locks cannot protect multi-node mutation." -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: Revision idempotency, CAS, and lifecycle tests."
~~~

---

### Task 3: Promotion Code Resolver

**Files:**
- Create: service/stripe_checkout_promotion.go
- Test: service/stripe_checkout_promotion_test.go

**Interfaces:**
- Produces: StripeCheckoutPromotionResolver.ResolveManualPromotion and StripeCheckoutResolvedPromotion.

- [ ] **Step 1: Write failing resolver tests**

Use a fake StripeCheckoutPromotionClient and cover: global match, customer-restricted preference, two eligible globals ambiguous, inactive code, wrong customer, minimum/currency mismatch, and product restriction.

~~~go
func TestResolveManualPromotionPrefersCurrentCustomer(t *testing.T) {
    resolver := StripeCheckoutPromotionResolver{Client: &fakePromotionClient{
        promotions: []*stripe.PromotionCode{
            globalPromotion("promo_global", "SAVE20", 20),
            customerPromotion("promo_customer", "SAVE20", "cus_7", 25),
        },
    }}
    got, err := resolver.ResolveManualPromotion(context.Background(), StripeCheckoutPromotionQuery{
        Code: " save20 ", CustomerID: "cus_7", ProductID: "prod_pro",
        Currency: stripe.CurrencyUSD, Subtotal: 3000,
    })
    require.NoError(t, err)
    require.Equal(t, "promo_customer", got.PromotionCodeID)
    require.Equal(t, "SAVE20", got.MaskedCode)
}

func TestResolveManualPromotionRejectsAmbiguousGlobalMatch(t *testing.T) {
    resolver := StripeCheckoutPromotionResolver{Client: fakeWithTwoEligibleGlobalCodes("SAVE20")}
    _, err := resolver.ResolveManualPromotion(context.Background(), StripeCheckoutPromotionQuery{
        Code: "SAVE20", CustomerID: "cus_7", ProductID: "prod_pro",
        Currency: stripe.CurrencyUSD, Subtotal: 3000,
    })
    require.ErrorIs(t, err, ErrStripePromotionAmbiguous)
}
~~~

- [ ] **Step 2: Run test and verify red**

~~~powershell
go test ./service/ -run TestResolveManualPromotion -v
~~~

Expected: compilation failure.

- [ ] **Step 3: Implement resolver**

~~~go
type StripeCheckoutPromotionClient interface {
    ListPromotionCodes(context.Context, string) ([]*stripe.PromotionCode, error)
}

type StripeCheckoutPromotionQuery struct {
    Code string
    CustomerID string
    ProductID string
    Currency stripe.Currency
    Subtotal int64
}

type StripeCheckoutResolvedPromotion struct {
    PromotionCodeID string
    CouponID string
    MaskedCode string
}
~~~

Production adapter calls promotioncode.List with Code and Active=true and consumes all pages. Resolver performs case-insensitive exact match; filters customer, coupon validity/expiry/max redemptions, minimum amount/currency, and coupon.AppliesTo.Products; customer-specific matches win over global. More than one winning match returns ErrStripePromotionAmbiguous. Return Stripe's normalized code; never put submitted raw code into logs/errors.

- [ ] **Step 4: Run resolver tests**

~~~powershell
go test ./service/ -run TestResolveManualPromotion -v
~~~

Expected: PASS.

- [ ] **Step 5: Commit**

~~~powershell
git add service/stripe_checkout_promotion.go service/stripe_checkout_promotion_test.go
git commit -m "Resolve buyer promotion codes against Stripe restrictions" -m "Constraint: Stripe remains final eligibility authority." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: Customer, ambiguity, amount, currency, and product restrictions."
~~~

---

### Task 4: Revision-aware Session Builders

**Files:**
- Create: service/stripe_checkout_discount.go
- Test: service/stripe_checkout_discount_test.go
- Modify: controller/topup_stripe.go
- Modify: controller/topup_stripe_test.go
- Modify: controller/subscription_payment_stripe.go
- Modify: controller/subscription_one_time_stripe_test.go
- Modify: service/subscription_invoice.go
- Modify: service/subscription_invoice_test.go
- Modify: service/subscription_contract.go
- Modify: service/subscription_contract_test.go

**Interfaces:**
- Consumes: target revision and StripeCheckoutDiscountSelection.
- Produces: candidate Session for the same internal order with revision metadata and revisioned idempotency.

- [ ] **Step 1: Write failing parameter tests**

For top-up, recurring, and one-time builders, feed:

~~~go
selection := service.StripeCheckoutDiscountSelection{
    Source: service.StripeCheckoutDiscountManual,
    PromotionCodeID: "promo_manual_7",
}
~~~

Assert one promotion discount, checkout_revision=2, discount_selection=manual on Session/PaymentIntent/Subscription metadata, no recall_claim metadata, and idempotency contains :rev:2:. Add restore cases: canonical invitation uses CouponID, recall uses PromotionCodeID, none has no Discounts.

- [ ] **Step 2: Run focused tests**

~~~powershell
go test ./controller/ -run "TestStripeCheckoutSessionRevision|TestOneTimePlanStripeRevision" -v
go test ./service/ -run TestStripeSubscriptionCheckoutRevision -v
~~~

Expected: failure because builders accept neither revision nor explicit selection.

- [ ] **Step 3: Parameterize builders**

Define in service/stripe_checkout_discount.go:

~~~go
type StripeCheckoutDiscountSource string

const (
    StripeCheckoutDiscountNone StripeCheckoutDiscountSource = "none"
    StripeCheckoutDiscountInvitation StripeCheckoutDiscountSource = "invitation"
    StripeCheckoutDiscountRecall StripeCheckoutDiscountSource = "recall"
    StripeCheckoutDiscountManual StripeCheckoutDiscountSource = "manual"
)

type StripeCheckoutDiscountSelection struct {
    Source StripeCheckoutDiscountSource
    CouponID string
    PromotionCodeID string
    MaskedCode string
    ReplacedSource StripeCheckoutDiscountSource
}
~~~

Implement ApplyStripeCheckoutDiscount in the same file and unit-test that invitation sets Coupon, recall/manual set PromotionCode, and none clears Discounts. Extend StripeSubscriptionCheckoutInput with CheckoutRevision and DiscountSelection. Parameterize top-up genStripeLink/build params, recurring createStripeSubscriptionCheckout, and one-time create/build functions. Add trade_no, checkout_revision, discount_selection to Session, PaymentIntentData, and SubscriptionData. Include recall metadata only when Source==recall. Build idempotency from existing prefix + revision + selection identity hash, never the submitted code.

- [ ] **Step 4: Run builder regressions**

~~~powershell
go test ./controller/ -run "TestStripeCheckoutSession|TestOneTimePlanStripe" -v
go test ./service/ -run "TestStripeSubscriptionCheckout|TestStripeRecurringChangePlan" -v
~~~

Expected: PASS.

- [ ] **Step 5: Commit**

~~~powershell
git add service/stripe_checkout_discount.go service/stripe_checkout_discount_test.go controller/topup_stripe.go controller/topup_stripe_test.go controller/subscription_payment_stripe.go controller/subscription_one_time_stripe_test.go service/subscription_invoice.go service/subscription_invoice_test.go service/subscription_contract.go service/subscription_contract_test.go
git commit -m "Build revisioned Stripe Sessions with one explicit discount" -m "Constraint: Replacements preserve one internal order and never stack." -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: Top-up, recurring, and one-time Session builders."
~~~

---

### Task 5: Unified Mutation Endpoint and Candidate Transition

**Files:**
- Create: controller/stripe_checkout_discount.go
- Modify: model/stripe_checkout_revision.go
- Modify: model/stripe_checkout_revision_test.go
- Modify: router/api-router.go
- Modify: controller/topup_stripe.go
- Modify: controller/subscription_payment_stripe.go
- Modify: controller/subscription_self_purchase.go
- Modify: service/subscription_invoice.go
- Test: controller/stripe_checkout_discount_test.go
- Test: controller/topup_stripe_test.go
- Test: controller/subscription_self_purchase_test.go
- Test: router/subscription_routes_test.go

**Interfaces:**
- Consumes: signed context, expected revision, request ID, apply/restore.
- Produces: StripeCheckoutRevisionResponse using the locked envelope.

- [ ] **Step 0: Add ledger read APIs required by restore and conflict responses**

Add model-layer read-only getters for an exact revision and the current active revision by `(order_type, trade_no)`. They must not return superseded or abandoned rows as active, and missing rows must preserve `gorm.ErrRecordNotFound`. Cover revision 1, current active, excluded terminal states, and not-found behavior. Controllers must not query `model.DB` directly.

- [ ] **Step 1: Write failing endpoint tests**

Inject fake get/create/expire functions and cover:

~~~text
valid apply: candidate created -> old expired -> CAS active -> secret returned
invalid apply: candidate rejected -> old untouched
restore: canonical none/invitation/recall reconstructed from order
exact request retry: persisted result returned, no second Stripe create
same expected revision with new request: 409 checkout_revision_conflict plus latest active revision payload
old payment wins expire race: candidate expired, checkout_already_completed
activation CAS loss: candidate expired, 409 conflict
transient activation failure after old expiration: candidate remains preparing; exact retry promotes it
flag false: stripe_promotion_disabled
wrong user/tampered/expired context: stable context error
~~~

The happy-path handler test posts the locked request and asserts revision 2/manual response plus no raw code in captured logs.

- [ ] **Step 2: Run tests and verify red**

~~~powershell
go test ./controller/ -run TestUpdateStripeCheckoutDiscount -v
go test ./router/ -run TestStripeCheckoutDiscountRoute -v
~~~

Expected: route and handler undefined.

- [ ] **Step 3: Implement route, response, and transition**

Register:

~~~go
selfRoute.POST("/stripe/checkout/discount", middleware.CriticalRateLimit(), controller.UpdateStripeCheckoutDiscount)
~~~

Define StripeCheckoutDiscountState and StripeCheckoutRevisionResponse with client_secret, publishable_key, fallback_url, checkout_context, checkout_revision, discount_state, and optional topup_summary. The stable-error writer accepts an optional latest response and includes it only for revision conflict. Define the previously anonymous top-up map as:

~~~go
type StripeTopUpSummary struct {
    PayAmount    float64 `json:"pay_amount"`
    BonusAmount  float64 `json:"bonus_amount"`
    CreditAmount float64 `json:"credit_amount"`
    ShowAmounts  bool    `json:"show_amounts"`
}
~~~

Coordinator order:

~~~text
verify context, user, revision
resolve manual selection or canonical restore selection
prepare ledger revision / replay exact request
create unexposed candidate Session (Stripe validates)
record candidate before changing active Session
expire old Session
if payment won: expire candidate + abandon revision + completed/conflict
activate candidate with DB CAS
if CAS proves another revision won: expire candidate + abandon revision + conflict
if activation has a transient database error after old expiration: keep recorded candidate preparing; exact retry reconciles and promotes it
sign next context and return persisted candidate response
~~~

Dispatch candidate construction by claims.PurchaseKind to top-up, recurring, or one-time adapters; state transition remains shared. A replay of preparing retrieves both Sessions, then either abandons an unpaid candidate while the old Session is still payable, promotes the candidate when the old Session is confirmed expired, or expires the candidate when the old Session completed. It never creates another order/Session.

Initial top-up, resume, legacy subscription, and self-purchase Elements responses create revision 1 idempotently and add context/revision/discount state only when the flag is on and client_secret exists. Hosted responses remain unchanged.

- [ ] **Step 4: Run endpoint and response tests**

~~~powershell
go test ./controller/ -run "TestUpdateStripeCheckoutDiscount|TestStripe.*Checkout.*Context|TestSubscriptionSelfPurchase.*Revision" -v
go test ./router/ -run "TestStripeCheckoutDiscountRoute|TestSubscriptionRoutes" -v
~~~

Expected: PASS.

- [ ] **Step 5: Commit**

~~~powershell
git add controller/stripe_checkout_discount.go controller/stripe_checkout_discount_test.go controller/topup_stripe.go controller/topup_stripe_test.go controller/subscription_payment_stripe.go controller/subscription_self_purchase.go controller/subscription_self_purchase_test.go service/subscription_invoice.go router/api-router.go router/subscription_routes_test.go
git commit -m "Reissue Stripe checkout Sessions through one fenced endpoint" -m "Constraint: Invalid codes cannot invalidate the payable Session." -m "Confidence: high" -m "Scope-risk: broad" -m "Tested: Apply, restore, replay, CAS loss, payment race, and route tests."
~~~

---

### Task 6: Paid Revision Authority and Attribution

**Files:**
- Modify: controller/topup_stripe.go
- Modify: controller/topup_stripe_test.go
- Modify: controller/subscription_payment_stripe.go
- Modify: controller/subscription_one_time_stripe_test.go
- Modify: service/subscription_invoice.go
- Modify: service/subscription_invoice_test.go
- Modify: service/recall_attribution.go
- Modify: service/recall_attribution_test.go

**Interfaces:**
- Consumes: paid checkout_revision and discount_selection metadata.
- Produces: fulfillment only for active revision and attribution only for winning source.

- [ ] **Step 1: Write failing webhook tests**

Cover: stale top-up revision no credit; stale one-time revision no term; stale recurring invoice no activation; manual win over canonical invite/recall consumes neither; restored recall converts once; duplicate active webhook remains single-shot.

Fixtures include:

~~~go
metadata := map[string]string{
    "trade_no": order.TradeNo,
    "checkout_revision": "2",
    "discount_selection": "manual",
}
~~~

- [ ] **Step 2: Run tests and verify red**

~~~powershell
go test ./controller/ -run "TestStripe.*StaleRevision|TestOneTime.*StaleRevision" -v
go test ./service/ -run "TestReconcilePaidInvoice.*Revision|TestRecallAttribution.*Manual" -v
~~~

Expected: recurring accepts stale same-trade facts and canonical metadata can misattribute manual payment.

- [ ] **Step 3: Enforce active revision**

Top-up and one-time validators require provider Session ID and checkout_revision equal the order pointer/revision.

Extend recurring paidInvoiceFacts with CheckoutRevision and DiscountSelection. Parse Subscription/Invoice metadata and compare in validateLocalInvoiceFacts. Permit legacy absence only when migrated order CheckoutRevision is zero.

Pass winning DiscountSelection into invitation/recall finalization: invitation consumes invitation only; recall records conversion only; manual/none cannot classify as assisted recall even while canonical facts stay stored for restore.

- [ ] **Step 4: Run fulfillment regressions**

~~~powershell
go test ./controller/ -run "TestStripe.*PaymentContract|TestStripe.*StaleRevision|TestOneTimePlanStripe" -v
go test ./service/ -run "TestReconcilePaidInvoice|TestRecallAttribution|TestSubscriptionInvitation" -v
~~~

Expected: PASS.

- [ ] **Step 5: Commit**

~~~powershell
git add controller/topup_stripe.go controller/topup_stripe_test.go controller/subscription_payment_stripe.go controller/subscription_one_time_stripe_test.go service/subscription_invoice.go service/subscription_invoice_test.go service/recall_attribution.go service/recall_attribution_test.go
git commit -m "Make the paid Stripe revision authoritative for fulfillment" -m "Constraint: Canonical discounts remain restorable without implying attribution." -m "Confidence: high" -m "Scope-risk: broad" -m "Tested: Stale webhook, duplicate fulfillment, invitation, and recall cases."
~~~

---

### Task 7: Frontend Revision Contract and API Helper

**Files:**
- Modify: web/default/src/features/wallet/types.ts
- Modify: web/default/src/features/wallet/api.ts
- Modify: web/default/src/features/wallet/lib/stripe-checkout-opening.ts
- Modify: web/default/src/features/wallet/lib/stripe-payment-request.test.ts
- Create or modify: web/default/src/features/wallet/api.test.ts
- Modify: web/default/src/features/subscriptions/types.ts
- Modify: web/default/src/features/subscriptions/components/dialogs/subscription-purchase-dialog.tsx
- Modify: web/default/src/features/subscriptions/components/dialogs/subscription-purchase-dialog.test.tsx

**Interfaces:**
- Produces: StripeCheckoutDiscountRequest, StripeCheckoutRevisionData, and updateStripeCheckoutDiscount.

- [ ] **Step 1: Write failing propagation tests**

Assert resolveStripeCheckoutOpening preserves checkout_context/revision/discount_state/topup_summary. Assert updateStripeCheckoutDiscount posts the union request to the locked route with skipBusinessError and skipErrorHandler. Assert direct subscription dialog forwards the revision contract.

- [ ] **Step 2: Run tests and verify red**

~~~powershell
cd web/default
bun test src/features/wallet/lib/stripe-payment-request.test.ts src/features/wallet/api.test.ts src/features/subscriptions/components/dialogs/subscription-purchase-dialog.test.tsx
~~~

Expected: revision fields are stripped and helper missing.

- [ ] **Step 3: Add canonical types and adapters**

~~~ts
export type StripeCheckoutDiscountSource =
  | 'none'
  | 'invitation'
  | 'recall'
  | 'manual'

export interface StripeCheckoutDiscountState {
  source: StripeCheckoutDiscountSource
  display_name?: string
  promotion_code_masked?: string
  replaced_source?: Exclude<StripeCheckoutDiscountSource, 'manual'>
}

export type StripeCheckoutDiscountRequest =
  | { checkout_context: string; expected_revision: number; request_id: string; action: 'apply'; promotion_code: string }
  | { checkout_context: string; expected_revision: number; request_id: string; action: 'restore' }
~~~

Extend StripeCheckoutData/Opening. Expose revision capability only when context is nonempty, revision is a positive integer, and discount_state exists.

~~~ts
export async function updateStripeCheckoutDiscount(
  request: StripeCheckoutDiscountRequest
): Promise<ApiResponse<StripeCheckoutRevisionData>> {
  const res = await api.post('/api/user/stripe/checkout/discount', request, {
    skipBusinessError: true,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}
~~~

Propagate fields through subscription response types and subscription-purchase-dialog. Shared top-up/flexible callers already use the opener.

- [ ] **Step 4: Run tests and typecheck**

~~~powershell
bun test src/features/wallet/lib/stripe-payment-request.test.ts src/features/wallet/api.test.ts src/features/subscriptions/components/dialogs/subscription-purchase-dialog.test.tsx
bun run typecheck
~~~

Expected: PASS.

- [ ] **Step 5: Commit**

~~~powershell
git add src/features/wallet/types.ts src/features/wallet/api.ts src/features/wallet/api.test.ts src/features/wallet/lib/stripe-checkout-opening.ts src/features/wallet/lib/stripe-payment-request.test.ts src/features/subscriptions/types.ts src/features/subscriptions/components/dialogs/subscription-purchase-dialog.tsx src/features/subscriptions/components/dialogs/subscription-purchase-dialog.test.tsx
git commit -m "Preserve Stripe checkout revision state in the console" -m "Constraint: Partial legacy responses cannot expose a broken input." -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: Opener, API, subscription propagation, and typecheck."
~~~

---

### Task 8: Promotion UI, Elements Remount, and Responsive Layout

**Files:**
- Create: web/default/src/features/wallet/components/dialogs/stripe-promotion-code-control.tsx
- Modify: web/default/src/features/wallet/components/dialogs/stripe-checkout-dialog.tsx
- Modify: web/default/src/features/wallet/components/dialogs/stripe-checkout-layout.tsx
- Modify: web/default/src/features/wallet/components/dialogs/stripe-checkout-layout.test.tsx
- Create: web/default/src/features/wallet/components/dialogs/stripe-checkout-dialog.interaction.test.tsx
- Modify: web/default/src/features/wallet/lib/wallet-i18n.test.ts
- Modify: web/default/src/i18n/locales/*.json

**Interfaces:**
- Consumes: revision-capable StripeCheckoutDialogSession and updateStripeCheckoutDiscount.
- Produces: always-visible apply/remove UX and atomic remount.

- [ ] **Step 1: Write failing layout and interaction tests**

Layout asserts summary card < promotion control < Continue in rendered markup and Continue disabled while switching.

Interaction tests mock API/mount and cover: Enter/Apply submit trimmed once; invalid keeps old mount; success destroys once and mounts new secret once; Remove sends restore without canonical fields; older overlapping response ignored; Continue disabled through remount; replacing invitation shows direct replacement copy.

- [ ] **Step 2: Run tests and verify red**

~~~powershell
bun test src/features/wallet/components/dialogs/stripe-checkout-layout.test.tsx src/features/wallet/components/dialogs/stripe-checkout-dialog.interaction.test.tsx
~~~

Expected: missing slot/control/lifecycle.

- [ ] **Step 3: Implement form and remount fence**

Control props:

~~~ts
interface StripePromotionCodeControlProps {
  value: string
  discountState: StripeCheckoutDiscountState
  busy: boolean
  message: { kind: 'success' | 'error'; text: string } | null
  onValueChange: (value: string) => void
  onApply: () => void
  onRemove: () => void
}
~~~

Use form submit for Enter/Apply; disable empty/busy; show returned masked code and Remove for manual; keep input available for replacing the manual code; use aria-live=polite and role=alert.

Dialog local state:

~~~ts
const requestGenerationRef = useRef(0)
const mutationInFlightRef = useRef(false)
const [current, setCurrent] = useState(() => props.session)
const [switching, setSwitching] = useState(false)
~~~

On success: fence generation, destroy old mount, atomically replace secret/context/revision/discount/summary, then remount from current.clientSecret/current.checkoutRevision. Keep switching until mount ready. On ordinary rejection: keep current and old mount. On checkout_revision_conflict with a complete latest `data` payload: install that latest payload through the same destroy/remount path and show the conflict copy. Guard handleConfirm as well as disabled button.

Place promotionControl between a data-slot=stripe-checkout-summary-card block and data-slot=stripe-checkout-continue. Use minmax(0,1fr), min-w-0, wrapping, and max-[520px]:grid-cols-1; add no fixed width.

- [ ] **Step 4: Add copy and run frontend regression**

Required source keys:

~~~text
Promotion code
Enter promotion code
Apply
Applying promotion code...
Promotion code applied.
Promotion code applied. Previous discount replaced.
Remove promotion code
Restoring previous discount...
Previous discount restored.
This promotion code is invalid.
This promotion code is not eligible for this purchase.
Multiple promotion codes match. Contact support.
Checkout changed in another request. The latest checkout was restored.
Unable to update the checkout. Please try again.
~~~

Run:

~~~powershell
bun run i18n:sync
bun test src/features/wallet/components/dialogs/stripe-checkout-layout.test.tsx src/features/wallet/components/dialogs/stripe-checkout-dialog.test.ts src/features/wallet/components/dialogs/stripe-checkout-dialog.interaction.test.tsx src/features/wallet/lib/stripe-payment-request.test.ts src/features/wallet/lib/stripe-checkout-elements.test.ts src/features/wallet/lib/stripe-checkout-view-model.test.ts src/features/wallet/api.test.ts src/features/subscriptions/components/dialogs/subscription-purchase-dialog.test.tsx src/features/wallet/lib/wallet-i18n.test.ts
bun run typecheck
~~~

Expected: PASS and every registered locale contains keys.

- [ ] **Step 5: Commit**

~~~powershell
git add src/features/wallet/components/dialogs/stripe-promotion-code-control.tsx src/features/wallet/components/dialogs/stripe-checkout-dialog.tsx src/features/wallet/components/dialogs/stripe-checkout-layout.tsx src/features/wallet/components/dialogs/stripe-checkout-layout.test.tsx src/features/wallet/components/dialogs/stripe-checkout-dialog.interaction.test.tsx src/features/wallet/lib/wallet-i18n.test.ts src/i18n/locales/
git commit -m "Add promotion-code replacement to the Stripe modal" -m "Constraint: Desktop and mobile share one control and reading order." -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: Apply, remove, race, remount, i18n, layout, and type tests."
~~~

---

### Task 9: Full Verification and Responsive Stripe Test Matrix

**Files:**
- Modify only files owned by Tasks 1-8 if verification exposes a defect.
- Do not commit generated build output or secret-bearing screenshots.

- [ ] **Step 1: Format and static checks**

~~~powershell
gofmt -w service/stripe_checkout_context.go service/stripe_checkout_discount.go service/stripe_checkout_promotion.go model/stripe_checkout_revision.go controller/stripe_checkout_discount.go
git diff --check
~~~

Expected: no errors.

- [ ] **Step 2: Backend verification**

~~~powershell
go test ./controller/ -run "TestUpdateStripeCheckoutDiscount|TestStripeCheckoutSession|TestOneTimePlanStripe|TestSubscriptionSelfPurchase" -v
go test ./service/ -run "TestStripeCheckout|TestReconcilePaidInvoice|TestRecallAttribution|TestStripeSubscription" -v
go test ./model/ -run "TestStripeCheckoutRevision|TestTopUpLifecycle|TestSubscriptionLifecycle" -v
go test ./router/ -run "TestStripeCheckoutDiscountRoute|TestSubscriptionRoutes" -v
go test -race ./model/ ./service/ ./controller/ -run "StripeCheckoutRevision|UpdateStripeCheckoutDiscount|StaleRevision" -count=1
~~~

Expected: PASS and no race report.

- [ ] **Step 3: Frontend/build verification**

From web/default:

~~~powershell
bun test src/features/wallet/components/dialogs/stripe-checkout-layout.test.tsx src/features/wallet/components/dialogs/stripe-checkout-dialog.test.ts src/features/wallet/components/dialogs/stripe-checkout-dialog.interaction.test.tsx src/features/wallet/lib/stripe-payment-request.test.ts src/features/wallet/lib/stripe-checkout-elements.test.ts src/features/wallet/lib/stripe-checkout-view-model.test.ts src/features/wallet/api.test.ts src/features/subscriptions/components/dialogs/subscription-purchase-dialog.test.tsx src/features/wallet/lib/wallet-i18n.test.ts
bun run typecheck
bun run lint
bun run build:check
~~~

Expected: PASS.

- [ ] **Step 4: Browser and Stripe test-mode matrix**

With StripePromotionCodeEnabled=true in staging/test configuration, verify at 1440x900 and 390x844:

~~~text
no document/modal horizontal overflow
top-up original none -> valid apply -> remove -> none
subscription invitation -> manual replaces -> remove restores invitation
subscription recall -> manual replaces -> remove restores recall
invalid/expired/product/minimum failures keep the current payment form usable
totals update before Continue re-enables
only the winning Session fulfills the order
~~~

Record viewport, overflow result, Session IDs/revisions, and order/webhook outcome without raw code/customer secrets.

- [ ] **Step 5: Review and completion**

Run superpowers:requesting-code-review, fix every blocking finding with its focused test, confirm git status --short is clean, and report exact commits/commands.

---

## Spec Coverage Self-review

| Requirement | Task |
| --- | --- |
| One input for every purchase kind | 5, 7, 8 |
| Stripe-authoritative restrictions | 3, 4 |
| Direct replacement and restore | 4, 5, 8 |
| Remount without closing modal | 8 |
| Stale request / duplicate protection | 2, 5, 8 |
| Paid revision controls attribution | 6 |
| Inline stable errors | 5, 7, 8 |
| Server rollout flag | 1, 5, 9 |
| Desktop/mobile no overflow | 8, 9 |

## Stop Condition

Complete only when all tasks are committed, focused tests pass, race/static/type/lint/build checks pass, the responsive Stripe test-mode matrix passes, review findings are resolved, and the working tree is clean.
