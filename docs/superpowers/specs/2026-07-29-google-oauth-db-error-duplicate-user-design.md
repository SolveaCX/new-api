# OAuth identity lookup failure design

## Problem

OAuth login and binding currently treat a failed database lookup as “identity not found”. A transient database failure can therefore enter registration and create another user for the same provider identity. The lookup also checks `RowsAffected == 1`, so an identity that already has multiple rows is incorrectly treated as absent.

## Constraints

- Support SQLite, MySQL, and PostgreSQL.
- Production runs multiple application nodes; an in-process lock is insufficient.
- Preserve the existing soft-delete rule: a deleted user's provider identity remains reserved.
- Do not add a unique index in this change. Production already contains duplicate Google IDs, so an automatic unique-index migration could prevent startup.
- Do not mutate production data or deploy from this branch.

## Decision

Change OAuth provider identity checks to return `(bool, error)` and propagate database errors through login and binding controllers. A successful query returns `true` for one or more matches, rather than only exactly one match.

Provider-backed user fill operations will continue to represent a scoped “not found” result as a zero-value user so callers can report a deleted account, but they will return all other database errors. Custom OAuth binding checks and writes will follow the same error handling.

This is preferable to returning `true` on every database error: that would prevent registration but misreport an outage as “already bound” and would still allow later fill queries to hide database failures.

## Verification

- A forced provider identity lookup error returns an error from `findOrCreateOAuthUser` and creates no user.
- Zero, one, and multiple matching Google IDs produce false, true, and true respectively without error.
- Existing and soft-deleted user behavior remains unchanged.
- All provider implementations and controller call sites compile with the error-aware interface.

## Deferred database invariant

After duplicate production identities are reconciled, add cross-database-safe unique constraints for built-in OAuth identity columns in a separately staged migration. That is the durable protection against ordinary concurrent first-login races; this change specifically closes the observed database-error path and makes existing duplicate identities detectable.
