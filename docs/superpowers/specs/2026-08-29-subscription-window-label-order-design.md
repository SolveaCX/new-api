# Subscription Window Meter Label and Order Design

## Goal

Make the active subscription card easier to scan by showing the 5-hour and
7-day usage windows before the monthly quota, and remove the redundant
currency qualifier from their labels.

## Scope and constraints

- Change only the wallet current-plan presentation in
  `web/default/src/features/wallet/components/current-plan-card.tsx`.
- Keep the existing `UsageWindowMeter` values, progress bars, reset text, and
  API-backed `window_5h` / `window_7d` data unchanged.
- Keep the monthly quota link and subscription/payment lifecycle unchanged.
- Labels become `5-hour window limit` and `7-day window limit`; no `(USD)` or
  localized equivalent is shown in the label. Numeric quota formatting remains
  the existing configured display format.
- Omit a short-window meter when its API total is zero or absent, as today.

## Layout and data flow

`CurrentPlanCard` renders the optional short-window meter grid first, followed
by the linked monthly quota meter. Both continue to receive their respective
normalized self-subscription windows from `normalizeSelfSubscriptionData`.
No backend or payment code changes are required.

## Verification

- Add a regression assertion that the rendered short-window labels precede the
  monthly label and do not contain `(USD)`.
- Preserve assertions for both meters, their side-by-side grid, and the
  monthly usage-log link.
- Run the focused wallet component test, typecheck, and targeted ESLint.
