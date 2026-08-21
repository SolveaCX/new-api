# Website Portuguese BRL Payment Prices Design

## Goal

Show Brazilian real prices for the amounts a Portuguese-language visitor can pay on the public Pricing page, while leaving USD-denominated usage value, short-term limits, and free-credit explanations unchanged.

## Scope

The `/pt/pricing` page changes these payment-price surfaces:

- Go: `R$ 49,90/mês`
- Pro: `R$ 149,90/mês`
- Max: `R$ 499,90/mês`
- The Pro payment call to action uses `R$ 149,90/mês`.

All other locales keep their current displayed plan prices. Portuguese usage descriptions such as `Até $45 de uso de modelos / mês`, short-term dollar limits, and the `$1` free-credit explanation remain unchanged because they describe account value or limits rather than the checkout amount.

## Chosen Design

Keep the public Pricing page server-rendered and static. Add the configured BRL payment amount to each existing plan definition and choose the rendered payment price from the page locale. Portuguese formats BRL with Brazilian decimal punctuation; every other locale retains the current USD display.

Make the payment-call-to-action copy accept the rendered Pro price instead of embedding `$30`. This keeps the card and CTA on the same price source without changing the visible wording for non-Portuguese locales.

The BRL amounts match the existing subscription price contract used by the authenticated wallet: Go `49.9`, Pro `149.9`, and Max `499.9` BRL.

## Alternatives Rejected

- Add a new anonymous subscription-pricing API: rejected because the current plans endpoint requires authentication, and exposing a new billing endpoint is unnecessary for three stable marketing-page prices.
- Convert USD in the browser using an exchange rate: rejected because converted values could differ from the configured payable Stripe/Pix prices.
- Rely on Stripe Adaptive Pricing alone: rejected because Stripe only localizes Checkout and cannot rewrite the server-rendered public Pricing page.

## Error Handling

No network dependency is introduced, so the page cannot fail because Stripe or the console API is unavailable. The locale selection has an explicit Portuguese branch and preserves the current USD string for every other locale.

## Testing and Acceptance Criteria

- A server-rendering regression test proves `/pt/pricing` renders `R$ 49,90`, `R$ 149,90`, and `R$ 499,90` in the three plan cards.
- The same test proves the Portuguese Pro payment CTA uses `R$ 149,90/mês`.
- Tests prove English plan prices remain `$10`, `$30`, and `$100`.
- Tests preserve dollar-denominated Portuguese usage and free-credit copy.
- Website unit tests, lint, typecheck, and build pass.
- A local production preview at `/pt/pricing` shows the BRL payment prices before a deployment is considered ready.

## Deployment Impact

This is a website-only change. It requires deploying `newapi-web` (and staging `newapi-web-staging` for acceptance) but does not require deploying console or router services, changing Stripe configuration, or changing database state.
