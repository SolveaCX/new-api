# 充值 Stripe Checkout 与订阅页面统一实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让充值在创建 Stripe Elements 会话后呈现与订阅相同的 Checkout 页面层级和交互，同时保持充值订单语义及订阅支付契约不变。

**Architecture:** 继续使用现有共享 `StripeCheckoutDialog` 作为唯一 Checkout Elements 页面。充值选择弹窗在共享 Checkout 打开期间隐藏，关闭后恢复；后端仅保留已验证的充值 `custom_text` 兼容修复，订阅后端与订阅组件不改。

**Tech Stack:** Go、Stripe Go v86、React 19、TypeScript、Bun/Vitest、Tailwind CSS。

---

### Task 1: 锁定充值与共享 Checkout 的层级契约

**Files:**
- Modify: `web/default/src/features/wallet/index.test.tsx`
- Test: `web/default/src/features/wallet/index.test.tsx`

- [ ] **Step 1: Write the failing test**

在现有 `index.test.tsx` 的导入区增加 `readFileSync`，并在测试套件中加入以下测试。该测试只锁定已批准的渲染契约，不加载需要浏览器和网络的完整 Wallet 组件：

```tsx
import { readFileSync } from 'node:fs'

test('hides the recharge picker while the shared Stripe checkout is open', () => {
  const source = readFileSync(new URL('./index.tsx', import.meta.url), 'utf8')

  expect(source).toContain('open={topupDialogOpen && !checkoutDialog}')
  expect(source).toContain('<StripeCheckoutDialog')
})
```

- [ ] **Step 2: Run test to verify it fails**

Run from `web/default`:

```powershell
bun test src/features/wallet/index.test.tsx
```

Expected: FAIL because `index.tsx` currently renders the recharge picker with `open={topupDialogOpen}` while the shared Checkout dialog can be open at the same time.

### Task 2: Make the recharge flow use the same visible Checkout layer

**Files:**
- Modify: `web/default/src/features/wallet/index.tsx` at the recharge picker `<Dialog>` near the `RechargeFormCard`.
- Reuse: `web/default/src/features/wallet/components/dialogs/stripe-checkout-dialog.tsx` (no changes).

- [ ] **Step 1: Write minimal implementation**

Change only the parent picker visibility expression:

```tsx
<Dialog
  open={topupDialogOpen && !checkoutDialog}
  onOpenChange={setTopupDialogOpen}
>
```

Leave `processPayment`, `openStripeCheckout`, `StripeCheckoutDialog`, top-up summaries, discounts, and all subscription imports untouched. The existing `handleStripeTopUp` already requests `preferElementsCheckout: true`; the existing shared dialog remains the renderer for both flows.

- [ ] **Step 2: Run the focused test to verify it passes**

Run:

```powershell
bun test src/features/wallet/index.test.tsx
```

Expected: PASS, including the two existing currency-resolution tests and the new visibility contract test.

### Task 3: Verify payment contracts and subscription non-regression

**Files:**
- Verify only: `controller/topup_stripe.go`, `controller/topup_stripe_test.go`, `controller/subscription_payment_stripe.go`, `web/default/src/features/subscriptions/components/dialogs/subscription-purchase-dialog.tsx`

- [ ] **Step 1: Run Go payment tests**

From the isolated worktree root:

```powershell
go test ./controller -run 'TestStripeCheckoutSession(ElementsModeOmitsUnsupportedCustomText|CarriesSubmitMessage|EmbeddedModeUsesReturnURL)$'
```

Expected: PASS; Elements has no `CustomText`, hosted mode retains it, and legacy embedded return URL behavior remains valid.

- [ ] **Step 2: Run frontend Stripe and subscription tests**

From `web/default`:

```powershell
bun test src/features/wallet/lib/stripe-payment-request.test.ts src/features/wallet/components/dialogs/stripe-checkout-dialog.test.ts src/features/subscriptions/components/dialogs/subscription-purchase-dialog.test.tsx
```

Expected: PASS; wallet requests `ui_mode: 'elements'`, both flows resolve the same shared dialog contract, and subscription requests still include `ui_mode: 'elements'` with stable request IDs.

- [ ] **Step 3: Run typecheck and diff checks**

```powershell
bun run typecheck
git diff --check
git diff --stat origin/main...HEAD
```

Expected: typecheck and diff checks pass; the only new production-code line is the wallet parent-dialog visibility guard, while the subscription payment files remain unchanged.

### Task 4: Review, commit, and update the PR

**Files:**
- Commit: `web/default/src/features/wallet/index.tsx`, `web/default/src/features/wallet/index.test.tsx`
- Existing PR: `https://github.com/SolveaCX/new-api/pull/937`

- [ ] **Step 1: Inspect the final diff for scope**

```powershell
git status --short
git diff -- controller/topup_stripe.go controller/topup_stripe_test.go web/default/src/features/wallet/index.tsx web/default/src/features/wallet/index.test.tsx
```

Confirm that no subscription payment source file is modified and no production deployment command is run.

- [ ] **Step 2: Run change-scope tooling where available**

```powershell
gitnexus detect-changes --repo 'E:\workspace\new-api' --scope unstaged
```

If the local GitNexus database version mismatch prevents the graph query, record that limitation and rely on the explicit diff plus targeted tests.

- [ ] **Step 3: Commit the implementation**

```powershell
git add -- web/default/src/features/wallet/index.tsx web/default/src/features/wallet/index.test.tsx
git commit -m "Align recharge checkout dialog layering" -m "Keep the recharge picker behind the same shared Stripe Checkout Elements surface used by subscription purchases without changing either order lifecycle.

Constraint: Subscription payment behavior and production deployment must remain untouched.
Rejected: Merge recharge and subscription payment APIs | Expands scope and risks recurring-payment regressions.
Confidence: high
Scope-risk: narrow
Directive: Keep future Stripe UI changes in StripeCheckoutDialog and preserve separate top-up settlement.
Tested: Wallet, Stripe, subscription, Go payment tests and typecheck.
Not-tested: Live production payment confirmation."
```

- [ ] **Step 4: Push and update PR #937**

```powershell
git push origin fix/stripe-elements-custom-text-20260828
```

Update the PR description with the added layering guard and validation results. Do not deploy or restart production from this task.
