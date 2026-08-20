# Task 5 Report: Unified Mutation Endpoint and Candidate Transition

## Status

Complete. The authenticated Stripe checkout discount mutation endpoint, fenced candidate transition, replay/race handling, and initial revision-1 lifecycle are implemented for top-up, recurring subscription, and one-time subscription checkouts.

Step 0 remains provided by commit `f0eda674e`; this task consumes `model.GetStripeCheckoutRevision` and `model.GetActiveStripeCheckoutRevision` without adding controller-side ledger queries.

## Delivered behavior

- Registered `POST /api/user/stripe/checkout/discount` under authenticated self routes with `CriticalRateLimit`.
- Added the locked request/success/error envelopes and typed discount/top-up response payloads.
- Validates the feature flag, signed context, authenticated user, purchase kind, expected revision, trimmed request ID, and request ID length.
- Resolves manual codes through the Task 3 resolver without logging or echoing buyer-entered raw codes.
- Restores the canonical revision-1 selection, including persisted invitation Coupon IDs and top-up recall payloads.
- Runs `Prepare -> create candidate -> Record -> inspect/expire predecessor -> Activate`; only activation changes the order pointer.
- Handles exact active/preparing/abandoned replay, revision gaps, stale requests, payment-wins races, activation CAS loss, and transient activation retry.
- Creates active revision 1 for feature-enabled Elements top-up, recurring, and one-time initial checkouts; feature-disabled or hosted checkout paths retain legacy behavior.
- Adds `checkout_context`, `checkout_revision`, and `discount_state` to feature-enabled Elements responses and top-up resume responses backed by an active ledger revision.
- Persists the recurring invitation Coupon ID actually returned by the builder so later restore reuses the same provider object.

## TDD evidence

RED was observed before implementation for:

- Missing authenticated/rate-limited route.
- Missing coordinator handler/runtime and locked response types.
- Missing exact replay, stale revision, payment race, CAS loss, transient activation retry, and revision-gap behavior.
- Missing initial revision-1 lifecycle helpers for all three checkout kinds.
- Missing subscription initial response revision fields.

Each group was advanced to GREEN with the smallest corresponding production change. Stripe access is injected in controller tests; the final focused runs make no real Stripe requests.

## Verification

All commands exited 0 on 2026-08-20:

```powershell
# All production controller .go files plus owned/required helper tests;
# excludes the known baseline customer_usage_reconciliation_test.go failure.
go test <controller-production-file-list> <owned-and-required-helper-tests> -run 'TestUpdateStripeCheckoutDiscount|TestCreateInitialStripeCheckoutRevision|TestStripeRecurringInitialRevision|TestCreateStripeTopUpCheckoutSession|TestCreateOneTimeStripeCheckoutSessionInitializes|TestSubscriptionSelfPurchase.*Revision|TestSubscriptionSelfPurchaseResponseIncludesClientSecret' -count=1 -v

go test ./router/ -run 'TestStripeCheckoutDiscountRoute|TestSubscriptionRoutes' -count=1 -v
go test ./service/ -run 'TestCreateStripeSubscriptionCheckout|TestStripeSubscriptionCheckout|TestStripeRecurringChangePlan' -count=1 -v
go vet <controller-production-file-list> <owned-and-required-helper-tests>
go vet ./service/
git diff --check
```

## Concerns and exclusions

- The repository's pre-existing `controller/customer_usage_reconciliation_test.go` compile failure was intentionally excluded and not modified, per the task brief.
- No unbounded controller or repository-wide suite was run.
- No real Stripe API call was made; provider behavior is covered through injected creator/getter/expirer fakes and existing parameter-builder tests.
- Hosted checkout responses and feature-disabled flows intentionally remain on their existing compatibility path.

## Fix round 1

Review findings were addressed in a follow-up TDD pass:

- Fresh requests now accept the next monotonic ledger revision after abandoned gaps; exact replay is proven from request digest, row state, current order revision, and provider pointer rather than numeric adjacency.
- The request digest is a domain-separated SHA-256 of the normalized action and case-normalized trimmed input. Raw buyer input is not persisted. Existing requests are looked up before promotion resolution, so exact apply replay does not call the resolver again and stale new requests conflict before resolution.
- A preparing row without a provider Session ID re-enters the same revision-specific candidate creator and records the idempotently returned Session. A durable Record whose response was lost is detected and preserved; failed cleanup never abandons an unconfirmed candidate.
- Activation conflicts now reload the order and active ledger row. Only a proven winner causes loser expiration/abandonment and a latest-revision conflict; no-winner conflicts remain preparing for exact retry.
- Initial revision 1 converges from interruptions after Prepare, provider create, Record, and Activate for top-up, one-time subscription, and recurring subscription checkouts. Recurring invitation replay reads the persisted Coupon ID and does not call the Coupon or Session creator again.
- Invoice top-ups use the invoice snapshot Stripe customer for resolution and candidate construction, and require the predecessor Session to have the same customer identity.
- Completed top-up and subscription orders return `checkout_already_completed`; successful initial and replay envelopes use `message: success`.

Fix-round RED was observed for the abandoned-gap expectation, missing Record recovery seam/customer identity, all shared initial interruption stages, and recurring durable-stage replay. The final focused controller, router, service, vet, and diff-check commands listed above all returned exit 0 after the fixes.
