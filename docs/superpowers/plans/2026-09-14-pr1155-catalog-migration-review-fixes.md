# PR #1155 Catalog Migration Review Fixes

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. This plan is executed inline in the authoring session; each task is TDD (failing test → fix → green → commit).

**Goal:** Make PR #1155 (legacy → new catalog cutover at paid renewal) safe to ship by fixing the 15 verified review findings plus the verified notes, without changing the feature's external API.

**Architecture:** Keep the two-phase design (preview/apply → renewal-boundary application). Move policy gates (feature flag, allowlist) to admin operations only; at the renewal boundary validate only the batch's immutable runtime facts (deployment env, service, livemode, credential mode). Never compare Stripe amounts to local prices (repo rule). Make the wallet sweeper resilient to per-contract failures. Persist batch cancellation as a sticky fact.

**Tech Stack:** Go 1.25, GORM, stripe-go v86, testify, sqlite test DB.

**Branch:** `fix/pr1155-catalog-migration-review` (worktree `E:\workspace\new-api-worktrees\pr1155-review-fixes`, based on PR head `39b32eed6`).

---

## Finding → Task map

| # | Finding | Task |
|---|---|---|
| 1 | Snapshot validator rejects live Stripe prices → production impossible | T1 |
| 6 | Released discount reservation turns paid invoice into permanent error | T2 |
| 7 | Exact amount equality (violates "never compare Stripe amounts") | T2 |
| 10 | v1 discount snapshots rejected when typed snapshot present | T2 |
| 12 | Re-prepare fails on subtotal after first item (falls out of #7) | T2 |
| 8 | Downgrade path leaves stale typed snapshot → bricked renewals | T3 |
| 9 | Frozen snapshot never cleared on cancel (sharp edges removed by T2/T3; snapshot itself is a faithful frozen copy, kept) | T3 |
| 2 | Wallet loader lacks Applied guard → error every later cycle | T4 |
| 3 | Boundary hard-errors for flag-off / non-scheduled statuses | T4 |
| 4 | Balance pause + expiry → poison contract | T4 |
| sweeper | First error aborts every wallet renewal + skips expiry/reset | T4 |
| 11 | Cancel not terminal; re-apply re-schedules | T5 |
| note | `syncing` counted as Failed in summary | T5 |
| note | Per-contract apply errors discarded silently | T5 |
| 13 | AddDate month overflow in schedule phase end | T6 |
| 14 | Attention marker has no status CAS; busy check ignores expiry; prepared-failure blocks user actions for TTL | T7 |
| 15 | Cancel path mutates Stripe before ExpectedChangeVersion CAS | T7 |
| note | Wallet contracts never supersede catalog intents on user action | T7 |
| 5 | `type:longtext` breaks PostgreSQL (Rule 2) | T8 |
| note | `encoding/json` in business code (Rule 1); duplicated helpers; dead func; unindexed post-tx lookup | T8 |
| note | Empty allowlist filtered out of `--update-env-vars` | T9 |

Deliberately left as-is (documented in PR): `context.Background()` for user-action supersede (safer for multi-step provider mutation); hard-fail of user action when supersede fails (fail-closed is correct); `walletContractIsRenewable` (covered by inline check + sweeper WHERE); duplicate adapters / N+1 preview lookups (refactor, no behavior change).

---

### Task 1: Snapshot validator must not pin Stripe mode

**Files:** `service/subscription_plan_snapshot.go` (ValidateRecurringPlanSnapshotV1AgainstStripePrice), `service/subscription_plan_snapshot_test.go`

Livemode consistency is already enforced by `ValidateCatalogMigrationStripeSandbox` (facts.*Livemode) at preview and scheduler time. The snapshot validator only checks price identity/amount/interval/active.

- [ ] Test: `TestRecurringPlanSnapshotV1AcceptsLiveModePrice` — live price passes; `FreezeRecurringPlanSnapshotV1(plan, livePrice)` succeeds. Replace the "live" subcase in `TestRecurringPlanSnapshotV1RejectsPlanAndStripeDrift`.
- [ ] Fix: delete the `price.Livemode` check.
- [ ] Commit: `fix(catalog-migration): stop pinning recurring plan snapshots to Stripe test mode`

### Task 2: Never compare Stripe amounts; accept released reservations; accept v1 discount snapshots

**Files:** `service/subscription_invoice.go` (validateRenewalInvoiceFactsTx, stripeSubscriptionDiscountInvoiceExpectedPaymentMinorTx, reconcilePaidInvoiceRenewalTx, recurringInvoiceSnapshotFingerprint), `service/subscription_discount_invoice.go`, `service/subscription_price_authority_test.go`, `service/subscription_discount_invoice_test.go`, `service/subscription_invoice_test.go`

- [ ] Tests (red): restore baseline `TestSubscriptionDiscountInvoicePaidValidationAcceptsStripeAdjustedFinalPayment` and `TestSubscriptionDiscountInvoiceLatePaidAfterReleaseGrantsEntitlementWithoutLedgerMutation`; convert `RejectsWrongAmountWithoutSwitch` → `AcceptsStripeAdjustedAmountAndSwitches`; drop the "amount" subcase from `SecondCycleRejectsBillingDrift`; convert `RejectsLegacyV1DiscountSnapshot` → `AcceptsLegacyV1DiscountSnapshotAndCommits`; add `TestSubscriptionDiscountInvoicePrepareReplayTolerantOfRecomputedSubtotal` (recorder recomputes `inv.Subtotal` after item creation; second prepare succeeds, no double reserve).
- [ ] Fix: remove all `facts.Amount` equality checks and the `amountIsSubtotalOption` variadic (callers updated); `stripeSubscriptionDiscountInvoiceExpectedPaymentMinorTx` accepts version 1 unconditionally and drops `OriginalSubtotalMinor != baseMinor`; `commitStripeSubscriptionDiscountInvoiceForPaidRenewalTx` released → proceed with grant (bool discarded as in baseline); `recurringInvoiceSnapshotFingerprint(planSnapshot, plan)` falls back to a plan-identity fingerprint when no snapshot exists (pending-downgrade invoices).
- [ ] Commit: `fix(subscription): stop comparing Stripe amounts on renewal and honor paid invoices after discount release`

### Task 3: Stale typed snapshot tolerance + downgrade clears it

**Files:** `service/subscription_invoice.go` (recurringPlanSnapshotFromBindingTx, resolveRenewalPlanSnapshotTx, reconcilePaidInvoiceRenewalTx), `service/subscription_invoice_test.go`

- [ ] Test: `TestReconcilePaidInvoiceStaleTypedSnapshotDoesNotBrickRenewal` — binding has typed snapshot for plan A but plan_id = B (as after upgrade/downgrade); ordinary renewal at plan B's catalog price grants.
- [ ] Test: `TestReconcilePaidInvoicePendingDowngradeClearsTypedSnapshot` — after a downgrade renewal, `current_plan_snapshot` is empty.
- [ ] Fix: typed snapshot with `PlanID != binding.PlanId` is treated as absent (fall back to order/entitlement); downgrade (`pendingDowngrade && catalog == nil`) sets `current_plan_snapshot = ""`.
- [ ] Commit: `fix(subscription): tolerate stale typed plan snapshots after plan changes`

### Task 4: Renewal-boundary semantics + resilient wallet sweeper

**Files:** `service/subscription_wallet_renewal.go`, `service/subscription_invoice.go`, `service/subscription_catalog_migration_manifest.go` (new `validateCatalogMigrationBatchRuntimeFacts`), tests in `subscription_wallet_renewal_test.go`, `subscription_invoice_test.go`

Semantics: feature flag / allowlist gate admin operations only. At the boundary: (a) `applied` → no catalog resolution; (b) `scheduled` → strict facts, apply; (c) `compensation_required`/`needs_attention` → apply only if the paid invoice already carries the target price (Stripe executed), else ordinary legacy renewal; (d) any other status → ordinary renewal. Runtime facts (env, service, livemode, credential mode) must match the batch → otherwise a *retryable* error (webhook retried; wallet retried next tick). Wallet sweeper logs and continues past per-contract errors; only the query error propagates.

- [ ] Tests: `TestRenewWalletSubscriptionContractSecondCycleAfterCatalogApplyRenewsOnTargetPlan`; `TestRunWalletSubscriptionRenewalOnceContinuesPastPoisonContract`; `TestRenewWalletSubscriptionContractHonorsScheduledCatalogWhenFlagOff`; `TestRenewWalletSubscriptionContractNonScheduledCatalogIntentRenewsLegacyPlan`; `TestRunWalletSubscriptionRenewalOnceAppliesCatalogAfterBalancePauseExpiryAndTopUp`; `TestReconcilePaidInvoiceCatalogMigrationHonorsScheduledWhenFlagOff`; `TestReconcilePaidInvoiceCompensationRequiredLegacyPriceGrantsOrdinaryRenewal`; `TestReconcilePaidInvoiceCompensationRequiredTargetPriceAppliesMigration`; `TestReconcilePaidInvoiceCatalogRuntimeDriftIsRetryable`. Update `CatalogFailuresNeverFallback` (drop "guard off"/"needs attention" subcases) and `ClosedSandboxMutatesNothing` (drive with service drift, assert retryable).
- [ ] Commit: `fix(catalog-migration): honor scheduled cutovers at the renewal boundary and keep the wallet sweeper alive`

### Task 5: Sticky cancellation, in-flight summary, observable apply errors

**Files:** `service/subscription_catalog_migration.go`, `service/subscription_catalog_migration_apply_test.go`

- [ ] Tests: `TestCatalogMigrationCancelIsStickyAndRefusesReapply` (drifted contract → no intent; cancel; restore; re-apply refused; 0 scheduler calls); `TestCatalogMigrationSummaryCountsSyncingAsInFlight`.
- [ ] Fix: `Cancel` persists `status=cancelled` after all items succeed; `Get`/`refreshBatchSummary` keep `cancelled` sticky; `Apply` on existing cancelled batch → error; summary gains `Syncing`; `processCatalogMigrationPreview` logs each item error with contract id.
- [ ] Commit: `fix(catalog-migration): make batch cancellation terminal and surface per-contract apply errors`

### Task 6: Month arithmetic without overflow

**Files:** `service/subscription_catalog_migration_stripe.go`, `service/subscription_catalog_migration_stripe_test.go`

- [ ] Test: `TestCatalogMigrationNextMonthlyPeriodEndClampsToMonthEnd` (Jan 31 → Feb 28 2026; Mar 31 → Apr 30; May 31 → Jun 30; Jan 15 → Feb 15; time of day preserved, UTC).
- [ ] Fix: `catalogMigrationNextMonthlyPeriodEnd(unix int64) int64` used by both params and ownership check.
- [ ] Commit: `fix(catalog-migration): compute the target phase end without month overflow`

### Task 7: User-action precedence hardening

**Files:** `service/subscription_catalog_migration_stripe.go`, `service/subscription_renewal_lifecycle.go`, `service/subscription_contract.go`, `service/subscription_catalog_migration_manifest.go`, `model/subscription_lifecycle_reservation.go` (exported active predicate), tests in `subscription_catalog_migration_stripe_test.go`

- [ ] Tests: `TestCancelRenewalWithStaleChangeVersionMakesNoStripeCall`; `TestCatalogMigrationUserActionAttentionNeverRelabelsAppliedIntent`; `TestCatalogMigrationUserActionIgnoresExpiredOrConsumedReservation`; `TestCatalogMigrationPreviewIgnoresConsumedReservationTombstone`; `TestCatalogMigrationWalletUserActionSupersedesLocalIntent`.
- [ ] Fix: `supersedeCatalogMigrationForUserAction(ctx, userID, targetPlanID, expectedChangeVersion)` enforces version inside the locked tx when > 0; cancel path passes the client's version; attention marker guarded `status NOT IN (applied, superseded)`; busy checks use `model.SubscriptionProviderLifecycleReservationIsActive(binding, now)`; wallet contracts supersede scheduled local catalog intents (no provider call).
- [ ] Commit: `fix(catalog-migration): validate user preconditions before superseding and respect reservation expiry`

### Task 8: Repo rules and cleanups

**Files:** `model/subscription_catalog_migration.go`, `model/subscription_contract.go`, `model/subscription_recurring.go`, `model/subscription_catalog_migration_test.go`, `service/subscription_catalog_migration*.go`, `service/subscription_plan_snapshot.go`, `service/subscription_invoice.go`

- [ ] Test: `TestSubscriptionCatalogMigrationSnapshotColumnsUseDialectDefaultTextType` (schema parse: no `TYPE` tag on the four snapshot columns).
- [ ] Fix: drop `type:longtext`; `encoding/json` → `common.*`; `containsString` → `common.StringsContains`; `parseCatalogMigrationBool` → `common.GetEnvOrDefaultBool`; delete `isCatalogMigrationTerminalProviderStatus`; `PaidInvoiceReconcileResult.CatalogMigrationBatchID` set inside the tx replaces the post-tx lookup.
- [ ] Commit: `chore(catalog-migration): follow repo JSON and database compatibility rules`

### Task 9: Deploy workflow allowlist

**Files:** `.github/workflows/gcp-deploy.yml`

- [ ] Fix: append `SUBSCRIPTION_CATALOG_MIGRATION_CONTRACT_ALLOWLIST=${...:-}` explicitly after the filtered loop in both prod jobs so an empty allowlist is always written.
- [ ] Commit: `ci: always write the catalog migration allowlist on production deploys`

### Task 10: Verification and hand-off

- [ ] `go vet ./model ./controller ./router ./service`
- [ ] `go test ./service/ -run 'CatalogMigration|PlanSnapshot|Renewal|Invoice|Discount|Wallet' -count=1`; `go test ./model/ ./router/ -count=1`; controller subscription suite (4 pre-existing failures on baseline are known).
- [ ] `git diff --stat pr-1155..HEAD` reviewed for unexpected deletions.
- [ ] Push branch, open PR targeting `feat/staging-legacy-renewal-catalog-migration` with the finding map above.
