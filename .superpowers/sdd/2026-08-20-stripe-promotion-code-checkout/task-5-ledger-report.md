# Task 5 Step 0 — model-owned ledger reads

## Scope

Implemented model-layer, read-only access to Stripe checkout revision history:

- `GetStripeCheckoutRevision(orderType, tradeNo string, revision int64)` returns the exact row in any lifecycle state.
- `GetActiveStripeCheckoutRevision(orderType, tradeNo string)` returns only a row whose state is `active`.
- Both APIs trim `orderType` and `tradeNo`, reject unsupported/blank inputs, and preserve database errors including `gorm.ErrRecordNotFound`.

Only the assigned model source, model tests, and this report were changed. Concurrent controller/service edits were left untouched and will not be staged in this commit.

## TDD evidence

### RED

Command:

```powershell
go test ./model/ -run 'TestGetStripeCheckoutRevision|TestGetActiveStripeCheckoutRevision' -count=1
```

Observed expected compile failure: `undefined: GetStripeCheckoutRevision` and `undefined: GetActiveStripeCheckoutRevision`. No fixture or syntax failures occurred.

### GREEN

The same command passed after the minimal implementation:

```text
ok  github.com/QuantumNous/new-api/model  6.063s
```

## Coverage

- Exact revision 1 lookup returns superseded and abandoned history rows.
- Exact and active lookups normalize surrounding whitespace.
- Active lookup returns the current active revision while ignoring superseded history.
- Superseded, abandoned, and preparing rows never satisfy the active lookup.
- Missing exact and active rows remain `errors.Is(err, gorm.ErrRecordNotFound)` compatible.
- Identical trade numbers remain isolated between top-up and subscription order types.
- Unsupported order types, blank trade numbers, and non-positive exact revision numbers are rejected before querying.

Mutation review confirms the tests fail behaviorally if the active-state predicate, order-type predicate, trade-number predicate, or exact revision predicate is removed or changed.

## Verification

```powershell
go test ./model/ -run 'TestGetStripeCheckoutRevision|TestGetActiveStripeCheckoutRevision|TestStripeCheckoutRevision' -count=1
# ok  github.com/QuantumNous/new-api/model  25.437s

go vet ./model/
# exit 0, no findings

git diff --check -- model/stripe_checkout_revision.go model/stripe_checkout_revision_test.go
# exit 0, no whitespace errors
```

## Concerns

No model-layer blockers. The active getter deliberately includes an explicit `state = active` predicate and orders matching rows by descending revision as defensive behavior if legacy data ever contains more than one active row.

An additional broad `go test ./model/ -count=1` was attempted but stopped after roughly 150 seconds without output, per the task leader's direction not to delay this bounded Step 0. The required targeted model suite and `go vet ./model/` both completed successfully; the broad package suite remains a documented verification gap for this commit.
