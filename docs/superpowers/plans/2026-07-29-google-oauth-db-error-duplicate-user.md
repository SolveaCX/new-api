# OAuth identity lookup failure implementation plan

1. Add regression tests for a database error during Google identity lookup, multiple matching identities, normal absent/present identities, and soft-deleted identities.
2. Confirm the new tests fail against the current implementation.
3. Make built-in and custom OAuth identity checks return `(bool, error)` and treat any positive match count as taken.
4. Propagate lookup errors from unified and legacy OAuth login/binding controllers.
5. Return real database errors from provider user fill operations while preserving scoped not-found/deleted-user behavior.
6. Run focused regression tests, affected package tests, formatting, vet/build checks, and review the final diff.
7. Commit the verified branch using the Lore Commit Protocol. Do not deploy, change production data, or modify `main`.
