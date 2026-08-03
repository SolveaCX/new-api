# Supplier Published-Day Unique Index Repair Design

**Date:** 2026-08-03
**Status:** Approved for staging recovery
**Environment:** Application startup migration, validated first on Flatkey staging
**Incident:** GitHub Actions run `30833847187`, failed Cloud Run revision `newapi-staging-20260803-165345-0aaa6900812f`

## Outcome

Restore staging startup by removing only redundant single-column unique indexes on `supplier_historical_published_days.date`, then make future startup migrations idempotent without weakening the date uniqueness invariant.

This repair is intentionally narrow:

- it does not run Terraform or modify Browser QA infrastructure;
- it does not deploy production or merge to `main`;
- it does not drop unrelated, composite, non-unique, prefix, or primary indexes;
- it preserves exactly one canonical unique index named `ux_supplier_historical_published_day_date`;
- it keeps SQLite, MySQL 5.7.8+, and PostgreSQL compatibility;
- it remains safe when more than one master process starts concurrently.

## Evidence and Root Cause

The staging container exited before binding `PORT=3000`. Cloud Run surfaced a generic startup failure, while the application log contained:

```text
failed to initialize database: Error 1069 (42000): Too many keys specified; max 64 keys allowed
```

The failing statement was:

```sql
ALTER TABLE `supplier_historical_published_days`
MODIFY COLUMN `date` varchar(10) NOT NULL UNIQUE
```

`migrateDB()` includes `SupplierHistoricalPublishedDay` in its main `AutoMigrate` call. The `Date` field currently declares a single-field named `uniqueIndex`. In the repository's GORM version, a single-field unique index also marks the parsed field as unique. MySQL column migration therefore emits `MODIFY COLUMN ... UNIQUE`; repeated cold starts accumulated equivalent keys until the table reached MySQL's 64-key ceiling.

The same container image digest previously started successfully, and earlier cold-start logs showed the same alteration succeeding. The failure is accumulated staging schema damage, not a Browser QA code regression.

## Approaches Considered

### 1. Pre-repair exact duplicates, then explicitly ensure the canonical index — selected

Remove the model tag that makes GORM emit column-level `UNIQUE`. Before `AutoMigrate`, inspect MySQL metadata and remove only exact duplicate unique indexes whose ordered column list is exactly `date`. After `AutoMigrate`, explicitly ensure the canonical unique index exists on all supported databases.

This both repairs the existing staging table and prevents recurrence.

### 2. Manually drop staging indexes once — rejected

A one-time manual cleanup could unblock the next deploy, but the unchanged GORM tag would repeat the column alteration on later cold starts. It also leaves the repair outside versioned, testable application behavior.

### 3. Add only a `HasIndex` guard after `AutoMigrate` — rejected

The failure occurs inside `AutoMigrate`, before a post-migration guard can run. A canonical-name check also cannot identify or remove anonymous or differently named exact duplicates that already consume the key limit.

## Architecture

The repair is split into three small units in the model package.

### Metadata model and pure planner

A pure function receives normalized MySQL index metadata grouped by index name and returns a deterministic repair plan:

- eligible indexes are non-primary, unique, non-prefix indexes with the ordered columns exactly equal to `date`;
- if the canonical index exists, keep it and drop every other eligible index;
- otherwise keep the lexicographically smallest eligible index, rename it to the canonical name, and drop the other eligible indexes;
- ignore every ineligible index;
- reject unsafe identifiers or inconsistent metadata instead of producing DDL.

Keeping the decision logic pure makes the destructive scope easy to test without a live MySQL server.

### Dedicated published-day schema migration

`SupplierHistoricalPublishedDay` is removed from the large shared `AutoMigrate` list and migrated through one dedicated function. On MySQL, that function pins one SQL connection with GORM's `Connection` API so the advisory lock, metadata reads, DDL, `AutoMigrate`, final ensure, and unlock all use the same database session.

The MySQL path:

1. returns immediately if the table does not exist;
2. obtains a connection-scoped advisory lock dedicated to this table repair;
3. reads `information_schema.statistics` for the current database and table, ordered by index name and sequence;
4. builds and validates the pure repair plan;
5. executes only constant-table, safely quoted `DROP INDEX` or `RENAME INDEX` statements from that plan;
6. runs `AutoMigrate` for `SupplierHistoricalPublishedDay` without the model-level unique tag;
7. explicitly ensures the canonical unique index;
8. re-reads metadata and verifies the final canonical definition;
9. releases the advisory lock on the same SQL connection.

MySQL DDL implicitly commits, so the repair does not claim transactional rollback. The advisory lock serializes competing master starts. Every operation is followed by metadata validation, and an ambiguous or unsafe state fails closed.

If the table is already at 64 keys and has no exact unique-date index that this scoped repair may reuse or remove, startup remains failed with an actionable error. The repair must never delete an unrelated index to make room.

### Cross-database post-AutoMigrate ensure

Inside the dedicated schema migration, an explicit ensure function creates the canonical unique index if absent:

```text
ux_supplier_historical_published_day_date(date)
```

The model tag is not used for this index. SQLite and PostgreSQL use their supported idempotent index creation path; MySQL performs a fresh metadata check while holding the same repair boundary or treats a concurrent already-exists result as success only after validating the final index definition.

Fresh databases therefore retain the same uniqueness contract even after the problematic tag is removed.

## Migration Ordering

The normal startup path becomes:

1. Main `DB.AutoMigrate(...)` call for the existing model set, excluding `SupplierHistoricalPublishedDay`.
2. Dedicated published-day schema migration: lock, scoped repair, tag-free `AutoMigrate`, canonical ensure, final validation, unlock.
3. Remaining startup migrations.

`migrateDBFast()` removes the published-day model from its generic per-model list and calls the same dedicated schema migration at the corresponding point. Tests and helpers that directly call `AutoMigrate` must use the dedicated function when they depend on the database constraint.

## Error Handling and Safety

- Metadata query failures stop startup and preserve the original error context.
- Advisory lock timeout stops startup; it never proceeds concurrently without serialization.
- Identifier validation permits only safe MySQL index identifiers before any dynamic DDL is built.
- `PRIMARY`, composite, non-unique, expression, prefix-length, and other-column indexes are never repair candidates.
- A canonical index with the wrong shape is an error, not an instruction to replace it automatically.
- Post-repair verification must prove the canonical index is unique and covers exactly `date`.
- No raw secret, DSN, or database credentials are logged.

## Multi-Node Behavior

Production and staging may start multiple master revisions concurrently. The MySQL repair uses a connection-scoped advisory lock and final-state revalidation so only one process performs DDL at a time. Other processes wait, then observe the canonical state and perform no destructive work. SQLite and PostgreSQL do not need the MySQL duplicate-index cleanup; their canonical index creation remains idempotent.

## Verification

Implementation is complete only when all of the following are proven:

- a regression test shows the current model schema marks `Date` as unique and therefore fails before the tag is removed;
- planner tests keep the canonical index, deterministically rename one eligible noncanonical index when needed, and drop only remaining exact duplicates;
- planner tests ignore `PRIMARY`, composite, non-unique, prefix, expression, and other-column indexes;
- malformed or unsafe metadata fails closed;
- repeated planning and repair are idempotent;
- SQLite creates the canonical unique index and rejects a duplicate date;
- supported-dialect schema tests continue to pass without a model-level unique tag;
- normal and fast migration paths both call the repair and ensure boundaries;
- targeted model tests, full model tests, `go vet` or equivalent static checks, `go build ./...`, and whitespace checks pass;
- an independent code review finds no critical or important issue;
- a fresh staging deployment reaches a healthy backend revision;
- Browser QA main replay and cleanup pass, and the root manifest reports `status=passed`.

Production deployment is explicitly outside this design. Staging success is evidence for a later production plan, not authorization to deploy `main`.
