# Design

## Source of truth

- Status: Active
- Last refreshed: 2026-07-17
- Primary product surfaces: NewAPI Console administration, the public Flatkey website, and the native Router/model status center.
- Evidence reviewed: `docs/superpowers/specs/2026-07-16-native-status-center-design.md`, `docs/superpowers/plans/2026-07-16-native-status-center.md`, `web/default/AGENTS.md`, existing authenticated routes and sidebar data, `web/default` Base UI/Tailwind components, theme tokens, and system-settings patterns.

## Brand

- Personality: Calm, precise, operational, and candid about incomplete evidence.
- Trust signals: Explicit timestamps, coverage, immutable public updates, clear permission boundaries, and visible stale/unknown states.
- Avoid: Decorative dashboard chrome, celebratory all-green presentation, hidden uncertainty, provider/channel disclosure, and alarmist color without supporting text.

## Product goals

- Goals: Let administrators operate incidents, maintenance, subscribers, deliveries, settings, overrides, and audit history from one guarded workspace; let the public understand Router and model health without false certainty.
- Non-goals: A channel/provider diagnostic console, a new design system, a generic monitoring builder, or a replacement for existing global Console navigation.
- Success signals: Administrators can identify current risk, complete permitted actions, recover from version conflicts, and understand delivery/configuration health without exposing secrets.

## Personas and jobs

- Primary personas: NewAPI administrators and Root operators; public API customers are the audience for the later website status pages.
- User jobs: Triage health, publish incident updates, schedule maintenance, apply expiring overrides, inspect delivery failures, manage guarded settings, and audit changes.
- Key contexts of use: Time-sensitive desktop operations, occasional tablet/mobile inspection, degraded backend or stale data, and concurrent edits by multiple administrators.

## Information architecture

- Primary navigation: An authenticated, admin-only `Status Center` sidebar entry.
- Core routes/screens: Console `/status-center` with Overview, Incidents, Maintenance, Subscribers, Deliveries, Settings, and Audit tabs; public `/status` and `/status/models/:slug` are separate website surfaces.
- Content hierarchy: Overall health and freshness first, actionable exceptions second, operational records and settings after; Router remains distinct from model components.

## Design principles

- Principle 1: Evidence before reassurance—unknown, stale, maintenance, and partial coverage remain visible and are never styled as operational.
- Principle 2: Guard actions at the point of use—permission, secure-verification, expiry, reason, and optimistic-version requirements are explicit before submission.
- Tradeoffs: Prefer information density and scannability over spacious marketing presentation; prefer compact responsive stacking over hiding operational fields.

## Visual language

- Color: Reuse semantic theme tokens and existing badge variants; never communicate state by color alone.
- Typography: Reuse the Console type scale; use tabular numerals where existing utilities support operational metrics and timestamps.
- Spacing/layout rhythm: Follow existing card, table, form, and page spacing; use compact but readable controls.
- Shape/radius/elevation: Reuse existing Base UI wrappers and theme radius/elevation tokens.
- Motion: Minimal feedback-only motion; respect reduced-motion preferences.
- Imagery/iconography: Reuse `lucide-react`; pair every status icon with a translated text label.

## Components

- Existing components to reuse: Authenticated page layout, tabs, cards, badges, alerts, tables, dialogs, forms, buttons, skeletons, empty states, and toast/error handling already present in `web/default`.
- New/changed components: Status-center feature components and pure validation/rendering helpers scoped under `src/features/status-center`.
- Variants and states: Operational, degraded, outage, unknown, and maintenance; loading, empty, error, stale, disabled, forbidden, and HTTP 409 conflict/reload states.
- Token/component ownership: Existing Console tokens and shared components remain authoritative; no parallel token layer or design-system abstraction is introduced.

## Accessibility

- Target standard: WCAG 2.1 AA.
- Keyboard/focus behavior: Tabs, dialogs, menus, forms, and tables remain keyboard operable with visible focus and logical focus return.
- Contrast/readability: Semantic colors must meet AA contrast and always include text/icon equivalents.
- Screen-reader semantics: Use semantic headings, landmarks, table headers, form labels, live status/error messaging, and descriptive action names.
- Reduced motion and sensory considerations: Avoid essential animation and do not rely on color, position, or motion as the sole signal.

## Responsive behavior

- Supported breakpoints/devices: Existing Tailwind mobile-first breakpoints and supported Console browsers.
- Layout adaptations: Stack summaries and forms on narrow screens; preserve tab access; allow tables to scroll or collapse into readable labeled rows without dropping critical fields.
- Touch/hover differences: Maintain adequate touch targets and expose all information without hover-only interactions.

## Interaction states

- Loading: Use existing skeleton/progress patterns without presenting cached values as fresh.
- Empty: Explain what is absent and whether the user can create or configure it.
- Error: Keep last trustworthy context where available, identify unavailable data, and offer retry.
- Success: Confirm the completed operation and refresh affected queries.
- Disabled: Explain missing permission, secure-verification requirement, unavailable secret/keyring state, or invalid form condition.
- Offline/slow network, if applicable: Preserve visible stale timestamps and avoid optimistic green; HTTP 409 responses require a reload/conflict message before editing continues.

## Content voice

- Tone: Direct, concise, factual, and non-blaming.
- Terminology: Use Router, model, incident, maintenance, subscriber, delivery, override, audit, operational, degraded, outage, unknown, and maintenance consistently.
- Microcopy rules: Route all visible Console copy through `t()` with real `en`, `zh`, `es`, `fr`, `pt`, `ru`, `ja`, and `vi` translations; state the recovery action for errors; never render a saved secret again.

## Implementation constraints

- Framework/styling system: React, TanStack Router, React Query, Base UI wrappers, Tailwind CSS, and the repository's existing API/i18n utilities.
- Design-token constraints: Reuse current semantic CSS variables, variants, and spacing; do not introduce a new design-system layer or dependency.
- Performance constraints: Keep data fetching tab-scoped where practical, invalidate only affected query keys, and avoid large eager client bundles or render-time object churn.
- Compatibility constraints: Preserve permission patterns, immutable published updates, optimistic versions, eight Console locales, responsive layouts, and secret non-disclosure.
- Test/screenshot expectations: Unit/component tests cover state labels, expiring/force-green validation, immutable rendering, 409 messaging, and permission controls; run i18n synchronization and TypeScript checks.

## Open questions

- [ ] Validate final information density with live production-like data after the first integrated Console build / product owner / may tune spacing without changing the interaction contract.
