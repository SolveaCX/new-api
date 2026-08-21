# Wallet locale pricing and subscription short-window removal

## Context

The wallet currently defaults checkout currency from IP region and still renders base top-up amounts as USD in several paths. The backend already contains the authoritative code-defined Stripe top-up contract for USD, JPY, BRL, and INR, including the allowed credit tiers and presentment amounts.

Flatkey subscriptions also enforce rolling 5-hour and 7-day limits in Redis in addition to monthly quota and media credits. Those short windows are exposed in wallet, profile, subscription administration, and self-service API projections.

## Goals

1. Make wallet top-up prices follow the active interface language by default.
2. Use the backend's code-defined Stripe currency contract as the price source of truth.
3. Fall back to USD whenever the language has no configured currency.
4. Remove real 5-hour and 7-day subscription enforcement.
5. Remove the short-window controls and user-facing meters while preserving monthly quota and image/video media credits.

## Non-goals

- Do not query Stripe to discover supported currencies at wallet page load.
- Do not change the USD credit credited to the wallet or how model usage is deducted.
- Do not remove Codex channel administration's upstream 5-hour/weekly usage report. Those windows describe upstream provider capacity, not Flatkey subscription entitlements.
- Do not drop legacy database columns in this change.
- Do not add or change payment currencies outside the existing code-defined contract.

## Design

### 1. Code-defined top-up price contract

Refactor the existing backend top-up contract into one package-private data structure keyed by checkout currency and USD-credit tier. Each entry stores the expected Stripe minor-unit amount. Existing Stripe checkout validation and the wallet top-up-info response must both derive from this structure.

The top-up-info response will expose only public pricing data for enabled preset tiers. It will not expose Stripe secrets. A representative shape is:

```json
{
  "stripe_currency_prices": {
    "USD": { "10": 1000, "20": 2000 },
    "BRL": { "10": 4990, "20": 9990 },
    "JPY": { "10": 1500, "20": 3000 },
    "INR": { "10": 89900, "20": 179900 }
  }
}
```

Values are minor units. JPY uses zero decimal places; USD, BRL, and INR use two decimal places. The response is filtered to preset amounts that have a configured Stripe Price ID so the wallet does not advertise an unpurchasable tier.

### 2. Wallet currency selection

The wallet normalizes the active i18n language to its base language. Automatic mapping is:

| Interface language | Preferred checkout currency |
| --- | --- |
| `pt`, including `pt-BR` | `BRL` |
| `ja`, including `ja-JP` | `JPY` |
| every other language | `USD` |

The preferred currency is accepted only when it exists in the backend price contract for the displayed tiers. Otherwise the wallet uses USD. INR remains available through the existing explicit/manual checkout-currency path, but is not selected automatically because the console has no mapped Hindi interface locale.

Selection precedence is:

1. a valid explicit checkout currency carried by an inbound pricing link;
2. a currency manually selected by the user;
3. the active interface-language mapping;
4. USD.

The current IP-region default must no longer override the interface-language default. The region may still control whether the manual currency selector is shown.

The selected currency drives all payment-price rendering on the recharge card and the `stripe_currency` sent to checkout. Wallet balance and credited quota remain USD-denominated credit. Recall discounts continue to use their signed backend quote currency.

If a selected currency lacks a price for a tier, the wallet falls back to USD before rendering and before creating checkout. It must not display one currency and submit another.

### 3. Remove subscription short-window enforcement

Subscription funding will stop reserving, settling, refunding, or snapshotting the Redis 5-hour and 7-day counters. Requests funded by a subscription will be gated only by the monthly subscription quota and, for applicable media operations, image/video media credits.

The billing error mapping for short-window exhaustion becomes unreachable and will be removed from the active billing path. New asynchronous task billing snapshots will not contain short-window accounting state. Existing in-flight or historical snapshots remain readable for compatibility but no longer determine whether a request is allowed.

The self-service subscription response will stop advertising 5-hour and 7-day usage buckets. Wallet current-plan cards, plan cards, profile summaries, and purchase reviews will remove the corresponding labels, meters, and reset times. Subscription administration will remove the two configuration inputs and will ignore or normalize legacy values so an older client cannot re-enable enforcement.

### 4. Data compatibility and cleanup

The existing `window_5h_amount` and `window_week_amount` database columns, historical plan snapshots, and expired Redis keys are retained. Their values are inert. This avoids a destructive cross-database migration and permits rollback during the initial release.

No Redis cleanup job is needed; old keys already expire. No process-local state is introduced.

Translations used only by Flatkey's subscription windows will be removed from all eight console locale files and the static-key registry when no other call site remains. Generic weekly reset labels and Codex upstream-window labels remain.

## Error handling

- Missing or malformed currency-price configuration falls back to USD.
- A tier missing from both the selected currency and USD is not rendered as purchasable.
- Stripe remains the final checkout validator; existing amount/currency contract validation is preserved.
- Removing short-window enforcement must not turn monthly quota exhaustion or media-credit exhaustion into a generic internal error.

## Tests

### Backend

- Top-up info returns currency prices derived from the same contract used by Stripe validation.
- Only enabled preset tiers with configured Stripe Price IDs are returned.
- USD, BRL, JPY, and INR minor-unit formatting data remains exact.
- A subscription with exhausted legacy 5-hour and 7-day values can still fund a request when monthly quota is available.
- Monthly quota exhaustion still rejects or falls back according to the existing funding preference.
- Media-credit enforcement remains active for image/video operations.
- New asynchronous task billing snapshots contain no active short-window reservation.

### Frontend

- `pt` and `pt-BR` default to BRL; `ja` and `ja-JP` default to JPY; all other supported locales default to USD.
- A missing configured currency falls back to USD.
- Recharge tiers, CTA/payment summaries, and checkout requests use the same currency and amount.
- Explicit/manual currency selection still works and is not overwritten by language changes during the same selection lifecycle.
- Wallet, profile, purchase review, and subscription administration render no Flatkey 5-hour or 7-day limit controls.
- Monthly quota and media-credit displays remain present and correct.

### Verification

- Run targeted Go tests for top-up info, Stripe top-up contract, subscription funding, billing session, and asynchronous task billing.
- Run targeted Bun tests for recharge card, wallet currency resolution, subscription plan cards, profile summary/header, and subscription administration.
- Run `bun run typecheck`, relevant lint checks, and a production frontend build.
- Run focused Go package builds/tests; the initial broad `go test ./service ./model ./controller` baseline hung without output and must not be treated as passing evidence.

## Multi-node deployment

Router deployment is required because subscription funding behavior changes on the relay billing path. Console deployment is required for the wallet, profile, and administration UI. The website is not changed.

During a rolling deployment, old router instances can still enforce legacy short windows while new instances do not. Completion requires all router instances to run the new version. No database migration, Terraform change, or Cloudflare change is required.

## Acceptance criteria

1. Portuguese wallet sessions default to BRL prices, Japanese sessions default to JPY prices, and every unmapped language defaults to USD.
2. Displayed top-up price and submitted Stripe currency always match the backend code-defined contract.
3. Exhausting stored 5-hour or 7-day counters never blocks or redirects a new subscription-funded request.
4. Monthly quota and image/video media-credit limits continue to work.
5. Flatkey subscription short-window controls and meters are absent from wallet, profile, purchase, and administration surfaces.
6. No legacy database column is dropped and no old Redis key needs manual deletion.
