# Console CLI Header CTA Design

## Goal

Add a prominent Flatkey CLI entry to the authenticated Console header so users can discover the product from any Console screen. Home and Rankings remain absent from the Console top navigation.

## Selected approach

Add a dedicated `ConsoleCliCta` component to the default `AppHeader` action group, immediately after desktop website navigation and before notifications. The CTA is not represented as a `TopNavLink`: `useTopNavLinks()` is also consumed by public navigation, while the Console CTA must remain authenticated-shell-only and visible on mobile.

The CTA opens the official website `/cli` landing page using `officialWebsiteUrl('/cli')`. It does not link to `/cli/authorize`, because that route is a device-flow callback that requires `user_code`.

## Alternatives considered

1. A normal `CLI` text link inside `TopNav`. Rejected because it is not visually prominent and the current `AppHeader` hides the `TopNav` wrapper below `lg`.
2. A static non-clickable CLI badge. Rejected because it creates emphasis without giving users a useful next action.
3. A direct GitHub or npm link. Rejected as the primary header destination because the official `/cli` landing page already provides localized product context, install commands, and downstream links.

## Visual and responsive behavior

- Use the existing compact button geometry with a terminal icon, `Flatkey CLI` on `sm` and wider screens, and `CLI` below `sm`.
- Use the existing Flatkey violet visual language with a restrained violet-to-fuchsia background, white text, and modest shadow.
- Keep the control inside the 48px header and visible down to 360px.
- Use color and the persistent text label together; do not add pulsing, flashing, or continuous animation.

## Accessibility and link behavior

- Render a semantic anchor with `target="_blank"` and `rel="noopener noreferrer"`.
- Keep the visible label as the accessible name and mark terminal/external-link icons decorative.
- Preserve an obvious `focus-visible` ring and AA-readable text contrast in light and dark themes.

## Implementation boundary

- Create `web/default/src/components/layout/components/console-cli-cta.tsx`.
- Render it only from the default action group in `app-header.tsx`, before notifications.
- Do not extend `TopNavLink`, modify `useTopNavLinks()`, change public navigation, add dependencies, or alter CLI authorization behavior.

## Verification

- A server-rendered component test proves the `/cli` destination, safe new-tab attributes, both responsive labels, and hidden decorative icons.
- Targeted test, ESLint, Prettier, TypeScript, production build, and `git diff --check` must pass.
- Browser checks at 1440px and 390px verify placement, emphasis, no header overflow, focus visibility, and destination behavior.
