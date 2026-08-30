# Wallet Plan Limit Summary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the authenticated wallet's plan-card short-window limits use the approved pricing-card panel and copy while keeping live usage meters unchanged.

**Architecture:** Keep `PlanLimitSummary` as the single presentation boundary for `window_5h_amount` and `window_week_amount`. Convert quota units to fixed-USD strings, choose a combined or single translated sentence from the available positive windows, and render one responsive tinted panel. Reuse the existing `All models` key and add the three short-term sentence keys to every console locale; no plan values are hardcoded in the UI.

**Tech Stack:** React 19, TypeScript, react-i18next, Tailwind CSS, Bun test, ESLint, TypeScript project references.

---

### Task 1: Lock the approved summary contract with failing tests

**Files:**
- Modify: `web/default/src/features/wallet/components/subscription-plans-card.test.tsx:404-451`
- Modify: `web/default/src/features/wallet/components/subscription-plans-card.test.tsx` (add a focused direct-render helper near the existing wallet render helpers)

- [x] **Step 1: Replace old independent-box assertions with panel assertions**

In the existing `renders migrated monthly quota values as strike-through prices for staging plans` test, keep the six dollar-value assertions and replace the old label assertions with:

~~~tsx
expect(html).toContain('data-plan-limit-summary="true"')
expect(html).toContain('data-plan-limit-label="all-models"')
expect(html).toContain('Short-term caps: $8 / 5h · $12 / 7d')
expect(html).toContain('Short-term caps: $18 / 5h · $45 / 7d')
expect(html).toContain('Short-term caps: $78 / 5h · $220 / 7d')
expect(html).not.toContain('5-hour window limit (USD)')
expect(html).not.toContain('7-day window limit (USD)')
~~~

The existing `data-plan-limit="5h"` and `data-plan-limit="7d"` checks should be removed because the compact summary has one sentence rather than two independent boxes.

Also update the later `renders current monthly and short-window usage meters with short windows side by side` test: keep its `data-wallet-usage-meter` assertions (those protect the live meters), but replace its two `data-plan-limit` assertions with one `data-plan-limit-summary="true"` assertion so the test does not depend on the removed box instrumentation.

- [x] **Step 2: Add a direct renderer and single-window test**

Import `PlanLimitSummary` beside the other wallet components and add this helper beside `renderWalletCardWithPlans`:

~~~tsx
function renderPlanLimitSummary(plan: {
  window_5h_amount?: number
  window_week_amount?: number
}) {
  return renderToStaticMarkup(
    <I18nextProvider i18n={testI18n}>
      <PlanLimitSummary plan={plan} />
    </I18nextProvider>
  )
}
~~~

Add this test in the wallet-card describe block:

~~~tsx
test('renders only the configured window in a compact summary', () => {
  const html = renderPlanLimitSummary({ window_5h_amount: 4_000_000 })

  expect(html).toContain('data-plan-limit-summary="true"')
  expect(html).toContain('Short-term cap: $8 / 5h')
  expect(html).not.toContain('7d')
  expect(html).not.toContain('5-hour window limit (USD)')
})
~~~

- [x] **Step 3: Run the focused test and verify the expected red state**

From `web/default`, run:

~~~bash
bun test src/features/wallet/components/subscription-plans-card.test.tsx
~~~

Expected: the new assertions fail because the current renderer emits two labeled boxes and no compact-panel marker or `Short-term ...` sentence. Fix only test setup errors; do not change production code before observing this feature failure.

### Task 2: Implement the compact, data-driven summary and localized copy

**Files:**
- Modify: `web/default/src/features/wallet/components/plan-limit-summary.tsx:24-77`
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/vi.json`
- Modify: `web/default/src/i18n/locales/es.json`
- Modify: `web/default/src/i18n/locales/pt.json`

- [x] **Step 1: Reuse the existing label key and add three real translations**

The `All models` key already exists in every locale and should be reused. Add these three entries to every locale's `translation` object. Keep placeholder names exactly `fiveHour`, `week`, and `value`:

~~~json
"Short-term cap: {{value}} / 5h": "Short-term cap: {{value}} / 5h",
"Short-term cap: {{value}} / 7d": "Short-term cap: {{value}} / 7d",
"Short-term caps: {{fiveHour}} / 5h · {{week}} / 7d": "Short-term caps: {{fiveHour}} / 5h · {{week}} / 7d"
~~~

Use these translated values in the non-English files:

~~~text
zh: 短期上限：{{value}} / 5 小时; 短期上限：{{value}} / 7 天; 短期上限：{{fiveHour}} / 5 小时 · {{week}} / 7 天
fr: Plafond à court terme : {{value}} / 5 h; Plafond à court terme : {{value}} / 7 j; Plafonds à court terme : {{fiveHour}} / 5 h · {{week}} / 7 j
ru: Краткосрочный лимит: {{value}} / 5 ч; Краткосрочный лимит: {{value}} / 7 д; Краткосрочные лимиты: {{fiveHour}} / 5 ч · {{week}} / 7 д
ja: 短期上限：{{value}} / 5時間; 短期上限：{{value}} / 7日; 短期上限：{{fiveHour}} / 5時間 · {{week}} / 7日
vi: Hạn mức ngắn hạn: {{value}} / 5 giờ; Hạn mức ngắn hạn: {{value}} / 7 ngày; Hạn mức ngắn hạn: {{fiveHour}} / 5 giờ · {{week}} / 7 ngày
es: Límite a corto plazo: {{value}} / 5 h; Límite a corto plazo: {{value}} / 7 d; Límites a corto plazo: {{fiveHour}} / 5 h · {{week}} / 7 d
pt: Limite de curto prazo: {{value}} / 5 h; Limite de curto prazo: {{value}} / 7 d; Limites de curto prazo: {{fiveHour}} / 5 h · {{week}} / 7 d
~~~

The three values in each line are ordered as singular 5h, singular 7d, combined.

- [x] **Step 2: Refactor `PlanLimitSummary` to one responsive panel**

Keep `formatUSDQuota` and positive-value filtering. Build a typed `windows` array, return `null` when it is empty, choose the combined key when both windows exist and the matching singular key otherwise, then render one panel:

~~~tsx
const windows = [
  { key: '5h' as const, amount: Number(props.plan.window_5h_amount || 0) },
  { key: '7d' as const, amount: Number(props.plan.window_week_amount || 0) },
].filter((window) => window.amount > 0)

if (windows.length === 0) return null

let summary: string
if (windows.length === 2) {
  summary = t('Short-term caps: {{fiveHour}} / 5h · {{week}} / 7d', {
    fiveHour: formatUSDQuota(windows[0].amount),
    week: formatUSDQuota(windows[1].amount),
  })
} else if (windows[0].key === '5h') {
  summary = t('Short-term cap: {{value}} / 5h', {
    value: formatUSDQuota(windows[0].amount),
  })
} else {
  summary = t('Short-term cap: {{value}} / 7d', {
    value: formatUSDQuota(windows[0].amount),
  })
}

return (
  <div
    data-plan-limit-summary='true'
    className={cn(
      'border-primary/10 bg-primary/10 rounded-lg border px-4 py-3',
      props.className
    )}
  >
    <div
      data-plan-limit-label='all-models'
      className='text-muted-foreground font-mono text-[10px] font-semibold tracking-[0.14em] uppercase'
    >
      {t('All models')}
    </div>
    <p className='text-foreground mt-2 text-sm leading-relaxed wrap-break-word'>
      {summary}
    </p>
  </div>
)
~~~

Do not change `CurrentPlanCard`, `UsageWindowMeter`, or the `PlanLimitSummary` call site.

- [x] **Step 3: Run the focused test and verify green**

~~~bash
bun test src/features/wallet/components/subscription-plans-card.test.tsx
~~~

Expected: all tests in the file pass, including combined Go/Pro/Max values, single-window rendering, fixed-USD formatting, and existing current-plan meter assertions.

### Task 3: Synchronize translations and verify the repository

**Files:**
- Generated/modified: `web/default/src/i18n/locales/*.json`
- Generated/modified: `web/default/src/i18n/locales/_reports/*` and `_extras/*` as produced by the sync script

- [x] **Step 1: Normalize locale ordering and inspect the report**

~~~bash
bun run i18n:sync
~~~

Expected: exit code 0, no missing keys in `_sync-report.json`, and no newly flagged untranslated values for the three added keys. Correct a translation before continuing if the report flags one.

- [x] **Step 2: Run typecheck and touched-file lint**

~~~bash
bun run typecheck
bunx eslint src/features/wallet/components/plan-limit-summary.tsx src/features/wallet/components/subscription-plans-card.test.tsx
~~~

Expected: both commands exit 0 with no TypeScript or ESLint errors.

- [x] **Step 3: Check the diff and inspect a local preview**

~~~bash
git diff --check
git status --short
bun run dev --host 127.0.0.1
~~~

Inspect the wallet route at desktop and narrow widths. Each plan card must have one lavender panel with `ALL MODELS` and one caps sentence; the current-plan monthly/5-hour/7-day meters must remain side-by-side and unchanged.

Observed in this run: Rsbuild's dev compiler rebuilt the console successfully, and the static wallet render/tests verified the panel structure. The connected local browser kept an empty React root, so an interactive screenshot comparison remains a preview-environment gap.

- [x] **Step 4: Commit the implementation with the Lore protocol**

After verification:

~~~bash
git add web/default/src/features/wallet/components/plan-limit-summary.tsx web/default/src/features/wallet/components/subscription-plans-card.test.tsx web/default/src/i18n/locales
git commit -m "Match wallet plan limits to pricing-card copy" -m "Present short-window caps in one localized pricing-style panel while preserving live usage meters.

Constraint: all user-visible console copy must exist in all eight locales
Rejected: adding a Tools Credits section | wallet plan data has no matching entitlement
Confidence: high
Scope-risk: narrow
Directive: keep current-plan UsageWindowMeter layout unchanged
Tested: focused Bun test; i18n sync; typecheck; ESLint; git diff --check
Not-tested: production deployment"
~~~
