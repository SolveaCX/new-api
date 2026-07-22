# Single Subscription Lifecycle — Non-blocking Follow-ups

This file records review findings intentionally deferred to keep the release path moving. Items here must not be treated as release blockers unless later evidence raises their severity.

## Task 5

- Legacy `POST /api/subscription/balance/pay` generates a server-side random request ID when the old UI omits one. Transaction serialization and same-plan/rank rejection prevent a second debit, but a retry returns `plan unchanged` instead of the original terminal result. Remove this compatibility path after the frontend fully migrates to `/api/subscription/self/change-plan` with a stable client UUID.
- `router/subscription_routes_test.go` primarily asserts Gin handler names for blocked legacy subscription purchase routes. Add an authenticated runtime router test later to cover middleware and route wiring end to end.

## Task 6

- `service/subscription_invoice.go` falls back to `now + 30 days` when Stripe invoice/subscription period facts are absent. Normal subscription invoices provide these fields, but a later hardening pass should fail closed or add explicit fallback coverage.
- If a Stripe Checkout Session was created but its URL was not persisted, replay uses the stored Stripe idempotency key with current user email/customer inputs. Freeze the original Checkout parameters later to eliminate possible Stripe idempotency-parameter drift.
