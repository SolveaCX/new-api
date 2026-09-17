# Stripe wallet payment reconciliation and automatic repair

This background task detects a Stripe wallet payment that succeeded but whose
local top-up is pending, failed, expired, or missing. A verified, matching local
wallet order is now **automatically credited and completed**, followed by a
DingTalk audit warning. Missing or conflicting orders remain alert-only. Stripe
itself is read-only: the task never charges, refunds, or changes a Stripe object.

## Agreed behavior

| Rule | Behavior |
| --- | --- |
| Initial backfill | Successful payment events from the seven days before first activation |
| Ongoing discovery | Stripe Checkout completed/async-success events, not a local pending-order scan |
| Grace period | None; a discovered paid event is immediately eligible for verification and repair |
| Schedule | Every minute; normally next-cycle processing, subject to dependencies and backlog |
| Automatic repair | Matching pending/failed/expired/cancelled/canceled wallet order: atomic credit, success state, optional local invoice update, durable repair audit |
| Repair notification | One `auto_repaired` warning after commit, containing previous status, credited quota and repair time; retry delivery without re-crediting |
| Unresolved anomaly | First notification, then another notification each hour |
| Recovery | One recovery notification when the matching local order becomes successful, then stop |
| Failed first notification | A later recovery still requires a recovery notification; cancel stale failure notices |

Only paid, payment-mode wallet checkouts are candidates. Subscription checkout
metadata and subscription mode are excluded, including one-time subscriptions.
`no_payment_required`, handling refunds/disputes, double-charge resolution, and
auditing a locally successful order's balance/ledger are outside this feature.
Refund/dispute evidence is checked to **prevent** unsafe repair, not to reverse
an existing credit. Removing the grace means a slow normal webhook can lose the
fulfillment race to this task; the resulting repair warning is intentional.

## Automatic repair safety boundary

Immediately before a new credit, retrieve the current Checkout Session with its
line item, PaymentIntent and latest Charge, then check the Charge's refunds.
Require complete/paid payment-mode checkout, succeeded PaymentIntent, a fully
captured succeeded Charge, matching account mode, session/order/customer identity,
and consistent actual amounts/currencies across the event, Session, Intent and
Charge. Session, Intent and Charge must all carry the same non-empty customer;
an existing local user/invoice customer must match it. Missing/null financial
evidence, any refund record (including pending,
failed or canceled), partial/full refund, or dispute blocks automatic repair.
Historical `paid` evidence alone is never sufficient for a new balance mutation.

The local contract is the exact stored Checkout Session, price ID, quantity one,
user and checkout revision/discount selection (positive revisions must be active).
Do not compare the local package's
nominal money/currency with the actual Stripe charge: legitimate coupons, tax and
Adaptive Pricing can change it. The credited wallet amount and eligible bonus
come from the persisted local order and normal recharge rules, never from a
currency-converted Stripe charge. Missing local session/price contracts or
unsupported revision evidence require manual review; this task does not guess
an order or activate a different checkout revision.
Revision zero accepts older absent metadata or the current default `0`/`none`
metadata without a revision row; unversioned non-`none` discount selections
remain manual-review cases.

The normal Stripe recharge implementation is shared with the webhook. Its order
lock and conditional status transition select one winner across nodes. The
winner's balance mutation, bonus/invite effects, lifecycle records, optional
local invoice paid state and reconciliation repair marker commit in one database
transaction. A previously recorded success lifecycle prevents re-crediting an
order whose status was later corrupted. Audit fields cannot be overwritten by
ordinary reconciliation checkpoints. An expired or replaced lease cannot credit.
The audit write evaluates database time in its lease predicate, including when
the lease expires after the transaction's initial validation.

Normal model-level cache updates, logs, purchase analytics and payment-success
notifications are retained; the dedicated repair warning is additional. This
task does not create/finalize Stripe invoices or issue PDFs, bind saved cards, or
replay controller-level card-country/recall attribution hooks. The original
webhook's successful-order replay can repair saved-card binding, invoice sync
and recall attribution without crediting twice. Card-country enrichment runs
only on the original recharge winner, so it requires separate operator backfill
if absent; webhook replay does not restore that field.

Stripe reads and the local database transaction cannot be one atomic transaction.
A refund initiated after the live check remains a normal refund-operations concern.
Account lookup outages permit local alert/recovery follow-ups, but never new credit.

## Configuration and deployment

- The task is registered at server startup on master nodes.
- Set `STRIPE_WALLET_RECONCILIATION_ENABLED=false` to disable it. It is otherwise
  enabled, but an installation without a Stripe API key does not start discovery.
- Uses the existing `StripeApiSecret` and the existing monitor's DingTalk alert
  enable flag, webhook URL and signing secret. A non-empty signing secret is
  required; unsigned delivery is rejected. No new robot is created.
- The Stripe key needs read access to the current account, Events, Checkout
  Sessions/line items, PaymentIntents, Charges and Refunds. No Stripe write
  operation is performed. Restricted-key read failures block repair safely.
- DingTalk disabled, missing configuration, or a failed send is an error, not a
  successful delivery. Notification state remains retryable.
- Existing reconciliation tables gain automatic-repair audit/error fields via
  normal model migration. Pause this task on old master nodes, upgrade all
  masters, then enable it. Although old/new workers cannot credit twice and old
  checkpoints cannot erase the repair marker, an old alert-only worker could
  close a pending repair warning as an ordinary recovery. Coordinated task
  activation preserves the dedicated repair notification during rollout.

Do not run this in a developer preview with production Stripe/DingTalk settings.
Use an isolated test database and mocked endpoints for tests. The feature's
master-only startup guard is not a distributed lock: database leases and fenced
updates are required even when multiple master instances overlap.

The shared Stripe recharge implementation and schema change, so production release
review must include both console and router targets. There is no website change.
Deployment is separate from completing local implementation and tests.

## Discovery, persistence and recovery

The current Stripe account ID and test/live mode define the scope. Rotating a key
for the same account must not reset first activation or historical coverage.
Every activation has a durable seven-day lower boundary; restarts do not slide
that boundary forward and silently lose history.

Recent discovery and historical backfill have separate durable cursors and page
budgets (five pages of up to 100 events each per run). Recent discovery uses a
ten-minute overlap. Each fixed window/page is checkpointed only after all its
candidates are saved; partial failure replays idempotently. Historical progress
does not prevent follow-up checks for previously discovered anomalies.

Discovery saves historical snapshots without per-session HTTP lookups. Session
verification is retried independently on each due work item. A malformed event
or conflicting historical reference is durably quarantined under `event:<id>`
and notified as `discovery_error`; it cannot block later pages. Such quarantines
require manual inspection and explicit closure of their reconciliation record,
not a guessed local-order match. Confirmed session association conflicts are
reported as `stripe_check_failed`, never as a missing order or recovery.
Transient session lookup errors remain retryable and contribute to task-health
alarms, not payment anomalies (a locally successful payment must not generate
an initial anomaly merely because Stripe is briefly unavailable).

Account lookup, recent discovery, backfill and due checks have separate bounded
time budgets (5/8/8/18 seconds within the 45-second run). Due items are claimed
one at a time, with a ten-second item timeout further capped by each lane, so a notification outage cannot
hold an entire unprocessed batch. During an account-lookup outage, previously
verified observations still receive local-order follow-ups; new session
verification waits for a trustworthy account scope.
Repair audit notices already committed with credit also remain deliverable during
Stripe outages; they do not need another Stripe lookup or another recharge.

Due processing reserves six seconds/up to 33 items for each of three lanes:
post-activation first checks, existing anomalies/recoveries, and pre-activation
historical first checks. Thus a large seven-day backfill cannot consume the
capacity reserved for new payments or recovery. These are throughput bounds,
not an unlimited-volume SLA; sustained overload still needs operator attention.
An idle replica does not clear task-health failure while persisted session
verification failures remain unresolved.

Candidates are unique per account/mode and Checkout Session, so replaying an
event or observing two success-related events for the same session cannot open
two notification lifecycles. Due-check ordering and leases bound each batch.
All leases use database time and ownership tokens; stale workers cannot commit
after another instance takes over.

Notifications are synchronous, time-bounded, and recorded only after the sender
acknowledges success. An uncertain remote outcome, such as a response lost after
delivery, can cause a limited duplicate on retry: this is **not an exactly-once
transport guarantee**. Messages carry stable identifiers for correlation.
Failed sends have a durable five-minute retry cooldown separate from successful
hourly reminders. Local recovery is still checked each minute, and switching
from an initial/reminder notice to recovery bypasses that failed-send cooldown.
Switching to `auto_repaired` also bypasses a failed anomaly-send cooldown. A
successfully delivered repair warning closes the work item; a failed send leaves
its durable repair marker pending for retry, even across restarts.

Tracked unresolved anomalies remain eligible for hourly reminders and recovery
checks even after their original event leaves the discovery/history window.

## Classification and failure reporting

Wallet sessions historically do not contain a `kind=wallet` marker. The task
uses the application's wallet order reference plus the configured wallet price
IDs and matching local wallet attributes. A positive local top-up amount is
important: subscription purchases can also have a Stripe TopUp history row.

If a plausible wallet payment has no local order and its price has been retired,
it is retained as `wallet_identity_unknown` for manual inspection rather than
silently discarded or described as a verified wallet missing order. Conflicting
local provider/session/price evidence is an identity issue, not a recovery.
Database errors are never converted to missing-order results.

Repeated task failures or a new-payment scan more than ten minutes behind are
reported separately from individual order anomalies, with hourly notification
throttling after a ten-minute sustained failure. Failed notification delivery
is retained for retry and logged; no channel can notify through DingTalk while
DingTalk itself is unavailable.

## Stripe limitations

Stripe's Events API provides a rolling thirty-day query window. If an unscanned
interval falls outside it, report a coverage gap and arrange a separate manual
backfill; do not silently advance the cursor. Seven-day initial backfill is a
business limit, not the Stripe retention limit.

`event.created` is the event generation time, not a guaranteed exact `paid_at`.
Do not scan only by Checkout Session creation time: an older session can succeed
later. Do not reinterpret an old unpaid `checkout.session.completed` event as
paid because the *current* session is paid; use the corresponding asynchronous
success event. Historical event objects retain their original API version and
are decoded independently of webhook signature/version handling.

Official references (checked 2026-09-09):

- [Events list and retention](https://docs.stripe.com/api/events/list)
- [Checkout event types](https://docs.stripe.com/api/events/types)
- [Event timestamps](https://docs.stripe.com/api/events/object)
- [Checkout Session fields](https://docs.stripe.com/api/checkout/sessions/object)
- [Charge capture/refund/dispute fields](https://docs.stripe.com/api/charges/object)
- [Refund list](https://docs.stripe.com/api/refunds/list)
- [Charge currency units](https://docs.stripe.com/currencies)
- [Rate limits and rolling read allocation](https://docs.stripe.com/rate-limits)

## Verification

Targeted tests cover zero-grace processing, atomic repair, webhook/worker races,
rollback and stale leases, duplicate prevention, unsafe financial evidence,
refund/dispute blocking, repair-warning retry, missing-order alerts, async payment
timing, history/cursor resume, hourly reminders and recovery. Run:

```text
go test ./model ./service -run 'StripeWallet|RechargeStripe' -count=1
go vet ./model ./service
```

No real customer payment or production notification is needed to run these tests.
