# Wallet plan limit summary aligned with pricing-card copy

## Context

The authenticated wallet page currently renders each plan's 5-hour and 7-day
limits as two separate bordered boxes. The public pricing cards already use a
compact purple information panel with an `ALL MODELS` label and one
short-term-cap sentence. The wallet should use the same visual language while
preserving its live usage meters.

## Goals

1. Make the Go, Pro, and Max plan-card limit summary visually match the pricing
   card reference: tinted panel, compact hierarchy, and a single short-term
   caps line.
2. Keep the limits data-driven from `window_5h_amount` and
   `window_week_amount`; do not duplicate plan values in the frontend.
3. Localize the new user-visible copy in all eight console locales.
4. Leave the current-plan monthly, 5-hour, and 7-day progress meters unchanged.

## Non-goals

- Do not change backend quota enforcement or API responses.
- Do not change the current-plan progress-bar layout.
- Do not add the website card's Tools Credits section to the wallet; the wallet
  plan projection does not expose a corresponding entitlement.
- Do not change plan prices, discounts, or checkout behavior.

## Design

`PlanLimitSummary` remains the sole renderer for short-window limits on the
plan cards. It will:

- filter out zero/negative windows as it does today;
- render one `bg-*`/bordered rounded panel when at least one limit exists;
- show a compact uppercase `All models` label;
- render a localized sentence with each available limit in USD and its period
  (`5h` and/or `7d`), separated by `·`;
- allow the sentence to wrap on narrow screens without introducing a second
  grid row.

When only one window is present, the sentence contains only that window. The
current-plan card continues to use `UsageWindowMeter` for live usage and reset
times; this change does not touch that component or its layout.

## Internationalization

Add the following source keys to all eight locale files (English is the source
language; each other locale receives a real translation):

- `All models`
- `Short-term caps: {{fiveHour}} / 5h · {{week}} / 7d`
- `Short-term cap: {{value}} / 5h`
- `Short-term cap: {{value}} / 7d`

The component chooses the singular key when only one window is available and
the combined key when both are present. Existing limit-label keys remain in
the locale files for the current-plan meter and other consumers.

## Verification

- Update `subscription-plans-card.test.tsx` to assert the compact panel and
  combined/single-window copy, and to reject the old independent-box labels.
- Run the focused wallet component tests.
- Run `bun run typecheck` and the touched-file lint check from `web/default`.
- Run `bun run i18n:sync` and inspect the generated untranslated reports.
- Start the console preview and verify desktop and narrow layouts against the
  supplied reference image.

## Multi-node/deployment impact

This is console-only presentation code. It does not alter router billing,
Redis counters, database state, or API contracts. A console frontend build is
required; router deployment is not required.
