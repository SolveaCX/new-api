# Supplier Published-Day Unique Index Repair Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Repair the accumulated MySQL unique indexes that block staging startup and make the published-day date constraint idempotent on every supported database.

**Architecture:** Remove the single-field GORM unique tag that produces column-level `UNIQUE`, then route this one model through a dedicated schema migration. The MySQL path pins a connection, serializes with `GET_LOCK`, plans repairs from `information_schema.statistics`, mutates only exact unique `date` duplicates, runs tag-free `AutoMigrate`, explicitly creates the canonical index, validates the final state, and unlocks. SQLite and PostgreSQL run the same tag-free migration and explicit idempotent canonical-index ensure without MySQL cleanup.

**Tech Stack:** Go 1.25, GORM v1.25.2, MySQL driver v1.4.3, SQLite test driver, PostgreSQL driver, Testify, GitHub Actions, Cloud Run staging.

---

## File Structure

- Create `model/supplier_historical_published_day_schema.go`: pure repair planner, dialect metadata readers, MySQL advisory-lock migration, explicit canonical-index ensure, and final validation.
- Create `model/supplier_historical_published_day_schema_test.go`: model-tag regression, pure planner edge cases, SQLite idempotence/uniqueness, and nil/unsupported-dialect behavior.
- Modify `model/supplier_historical_estimate.go`: remove only the problematic `uniqueIndex` tag from `SupplierHistoricalPublishedDay.Date`.
- Modify `model/main.go`: remove the model from generic normal/fast migration collections and call `MigrateSupplierHistoricalPublishedDaySchema` in both paths.
- Modify `model/supplier_historical_estimate_test.go`: update the shared fixture and supported-dialect schema expectation.
- Modify `model/supplier_report_test.go`, `model/supplier_accounting_integration_test.go`, `model/supplier_usage_schema_test.go`: use the dedicated schema migration in model-package fixtures.
- Modify `controller/supplier_historical_estimate_test.go` and `service/supplier_report_snapshot_scope_test.go`: use the exported dedicated schema migration in external-package fixtures.
- Modify `model/supplier_test.go`: make the existing fast-migration test prove the published-day table and canonical unique index are created.
- Modify `docs/browser-qa/browser-qa-technical-implementation.html` and `docs/browser-qa/ai-browser-testing-migration-guide.html` only after staging passes, documenting the recovered deployment and portable migration lesson.

### Task 1: Lock the GORM-tag regression with a failing test

**Files:**
- Modify: `model/supplier_historical_estimate_test.go:382-420`

- [ ] **Step 1: Change the published-day schema assertion to the desired tag-free contract**

Replace the published-day part of `TestSupplierHistoricalSchemaIndexesAcrossSupportedDialects` with:

```go
publishedStatement := &gorm.Statement{DB: db}
require.NoError(t, publishedStatement.Parse(&SupplierHistoricalPublishedDay{}))
publishedDateField := publishedStatement.Schema.LookUpField("Date")
require.NotNil(t, publishedDateField)
require.False(t, publishedDateField.Unique)
for _, index := range publishedStatement.Schema.ParseIndexes() {
	require.NotEqual(t, supplierHistoricalPublishedDayDateIndexName, index.Name)
}
```

The index-name constant is introduced by the production file in Task 2. Until then, use the literal `"ux_supplier_historical_published_day_date"` in this first RED run, then switch to the constant during GREEN.

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```powershell
go test ./model -run '^TestSupplierHistoricalSchemaIndexesAcrossSupportedDialects$' -count=1 -v
```

Expected: FAIL because `publishedDateField.Unique` is currently `true` and the parsed canonical index is present.

- [ ] **Step 3: Record the RED evidence before any production edit**

Capture the failing assertion and command in the implementation notes. Do not weaken the assertion.

### Task 2: Remove the unsafe tag and implement the pure repair planner

**Files:**
- Create: `model/supplier_historical_published_day_schema.go`
- Create: `model/supplier_historical_published_day_schema_test.go`
- Modify: `model/supplier_historical_estimate.go:148-154`
- Modify: `model/supplier_historical_estimate_test.go:382-420`

- [ ] **Step 1: Add planner tests before the planner exists**

Create table-driven tests around these internal types and function:

```go
type supplierHistoricalIndexDefinition struct {
	Name          string
	Unique        bool
	Primary       bool
	Columns       []string
	HasPrefix     bool
	HasExpression bool
}

type supplierHistoricalPublishedDayIndexRepairPlan struct {
	RenameFrom string
	Drop       []string
}

func planSupplierHistoricalPublishedDayDateIndexRepair(
	indexes []supplierHistoricalIndexDefinition,
) (supplierHistoricalPublishedDayIndexRepairPlan, error)
```

Cover these exact cases:

```go
func TestPlanSupplierHistoricalPublishedDayDateIndexRepairKeepsCanonicalAndDropsOnlyExactDuplicates(t *testing.T) {
	indexes := []supplierHistoricalIndexDefinition{
		{Name: "PRIMARY", Unique: true, Primary: true, Columns: []string{"id"}},
		{Name: supplierHistoricalPublishedDayDateIndexName, Unique: true, Columns: []string{"date"}},
		{Name: "date", Unique: true, Columns: []string{"date"}},
		{Name: "date_2", Unique: true, Columns: []string{"date"}},
		{Name: "idx_date_nonunique", Unique: false, Columns: []string{"date"}},
		{Name: "ux_date_import", Unique: true, Columns: []string{"date", "import_id"}},
		{Name: "ux_import", Unique: true, Columns: []string{"import_id"}},
		{Name: "ux_date_prefix", Unique: true, Columns: []string{"date"}, HasPrefix: true},
		{Name: "ux_date_expression", Unique: true, HasExpression: true},
	}

	plan, err := planSupplierHistoricalPublishedDayDateIndexRepair(indexes)
	require.NoError(t, err)
	require.Empty(t, plan.RenameFrom)
	require.Equal(t, []string{"date", "date_2"}, plan.Drop)
}

func TestPlanSupplierHistoricalPublishedDayDateIndexRepairRenamesDeterministicCandidate(t *testing.T) {
	indexes := []supplierHistoricalIndexDefinition{
		{Name: "date_9", Unique: true, Columns: []string{"date"}},
		{Name: "date_2", Unique: true, Columns: []string{"date"}},
		{Name: "date_3", Unique: true, Columns: []string{"date"}},
	}

	plan, err := planSupplierHistoricalPublishedDayDateIndexRepair(indexes)
	require.NoError(t, err)
	require.Equal(t, "date_2", plan.RenameFrom)
	require.Equal(t, []string{"date_3", "date_9"}, plan.Drop)
}

func TestPlanSupplierHistoricalPublishedDayDateIndexRepairIsIdempotent(t *testing.T) {
	plan, err := planSupplierHistoricalPublishedDayDateIndexRepair([]supplierHistoricalIndexDefinition{{
		Name: supplierHistoricalPublishedDayDateIndexName, Unique: true, Columns: []string{"date"},
	}})
	require.NoError(t, err)
	require.Equal(t, supplierHistoricalPublishedDayIndexRepairPlan{}, plan)
}

func TestPlanSupplierHistoricalPublishedDayDateIndexRepairRejectsUnsafeCandidate(t *testing.T) {
	_, err := planSupplierHistoricalPublishedDayDateIndexRepair([]supplierHistoricalIndexDefinition{{
		Name: "date` DROP TABLE users", Unique: true, Columns: []string{"date"},
	}})
	require.ErrorContains(t, err, "unsafe index name")
}

func TestPlanSupplierHistoricalPublishedDayDateIndexRepairRejectsWrongCanonicalShape(t *testing.T) {
	_, err := planSupplierHistoricalPublishedDayDateIndexRepair([]supplierHistoricalIndexDefinition{{
		Name: supplierHistoricalPublishedDayDateIndexName, Unique: true, Columns: []string{"date", "import_id"},
	}})
	require.ErrorContains(t, err, "canonical index has unexpected definition")
}
```

- [ ] **Step 2: Run planner tests and verify RED**

Run:

```powershell
go test ./model -run '^TestPlanSupplierHistoricalPublishedDayDateIndexRepair' -count=1 -v
```

Expected: compile failure because the production types/function do not yet exist. This is the wished-for API RED state.

- [ ] **Step 3: Implement the constants, planner types, validation, and deterministic planner**

Start `model/supplier_historical_published_day_schema.go` with:

```go
package model

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
)

const (
	supplierHistoricalPublishedDayTableName     = "supplier_historical_published_days"
	supplierHistoricalPublishedDayDateIndexName = "ux_supplier_historical_published_day_date"
	supplierHistoricalPublishedDayDateColumn    = "date"
)

var supplierHistoricalMySQLIndexNamePattern = regexp.MustCompile(`\A[A-Za-z0-9_$]+\z`)

type supplierHistoricalIndexDefinition struct {
	Name          string
	Unique        bool
	Primary       bool
	Columns       []string
	HasPrefix     bool
	HasExpression bool
}

type supplierHistoricalPublishedDayIndexRepairPlan struct {
	RenameFrom string
	Drop       []string
}

func (index supplierHistoricalIndexDefinition) isExactUniqueDateIndex() bool {
	return !index.Primary && index.Unique && !index.HasPrefix && !index.HasExpression &&
		slices.Equal(index.Columns, []string{supplierHistoricalPublishedDayDateColumn})
}

func planSupplierHistoricalPublishedDayDateIndexRepair(indexes []supplierHistoricalIndexDefinition) (supplierHistoricalPublishedDayIndexRepairPlan, error) {
	var candidates []string
	canonicalPresent := false
	for _, index := range indexes {
		if strings.EqualFold(index.Name, supplierHistoricalPublishedDayDateIndexName) {
			if !index.isExactUniqueDateIndex() {
				return supplierHistoricalPublishedDayIndexRepairPlan{}, fmt.Errorf("canonical index has unexpected definition: %s", index.Name)
			}
			canonicalPresent = true
			continue
		}
		if !index.isExactUniqueDateIndex() {
			continue
		}
		if len(index.Name) == 0 || len(index.Name) > 64 || !supplierHistoricalMySQLIndexNamePattern.MatchString(index.Name) {
			return supplierHistoricalPublishedDayIndexRepairPlan{}, fmt.Errorf("unsafe index name %q", index.Name)
		}
		candidates = append(candidates, index.Name)
	}

	sort.Strings(candidates)
	plan := supplierHistoricalPublishedDayIndexRepairPlan{}
	if canonicalPresent {
		plan.Drop = candidates
		return plan, nil
	}
	if len(candidates) == 0 {
		return plan, nil
	}
	plan.RenameFrom = candidates[0]
	plan.Drop = append(plan.Drop, candidates[1:]...)
	return plan, nil
}
```

- [ ] **Step 4: Remove only the problematic model tag**

Change the field to:

```go
Date string `json:"date" gorm:"type:varchar(10);not null"`
```

- [ ] **Step 5: Switch the Task 1 test from the literal index name to the internal constant**

Use `supplierHistoricalPublishedDayDateIndexName` in the schema assertion.

- [ ] **Step 6: Run the tag and planner tests and verify GREEN**

Run:

```powershell
go test ./model -run '^(TestSupplierHistoricalSchemaIndexesAcrossSupportedDialects|TestPlanSupplierHistoricalPublishedDayDateIndexRepair.*)$' -count=1 -v
```

Expected: PASS.

- [ ] **Step 7: Commit the first behavior slice using Lore format**

Stage only the four task files and commit with a decision-record message whose intent is preventing GORM from re-emitting column-level `UNIQUE`; include the RED/GREEN commands in the `Tested:` trailer.

### Task 3: Implement the dedicated cross-database schema migration

**Files:**
- Modify: `model/supplier_historical_published_day_schema.go`
- Modify: `model/supplier_historical_published_day_schema_test.go`

- [ ] **Step 1: Add the SQLite integration test before the migration function exists**

```go
func TestMigrateSupplierHistoricalPublishedDaySchemaSQLiteIsIdempotentAndUnique(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, MigrateSupplierHistoricalPublishedDaySchema(db))
	require.NoError(t, MigrateSupplierHistoricalPublishedDaySchema(db))
	require.True(t, db.Migrator().HasTable(&SupplierHistoricalPublishedDay{}))

	first := SupplierHistoricalPublishedDay{Date: "2026-08-03", DayStart: 1, ImportId: 1, PublishedBy: 1, PublishedAt: 1}
	second := SupplierHistoricalPublishedDay{Date: "2026-08-03", DayStart: 2, ImportId: 2, PublishedBy: 2, PublishedAt: 2}
	require.NoError(t, db.Create(&first).Error)
	require.Error(t, db.Create(&second).Error)

	definition, found, err := supplierHistoricalPublishedDayCanonicalIndex(db)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, definition.isExactUniqueDateIndex())
}

func TestMigrateSupplierHistoricalPublishedDaySchemaRejectsNilDB(t *testing.T) {
	require.ErrorIs(t, MigrateSupplierHistoricalPublishedDaySchema(nil), ErrDatabase)
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```powershell
go test ./model -run '^TestMigrateSupplierHistoricalPublishedDaySchema' -count=1 -v
```

Expected: compile failure because `MigrateSupplierHistoricalPublishedDaySchema` and canonical metadata lookup do not exist.

- [ ] **Step 3: Add dialect metadata readers and exact canonical validation**

Implement:

```go
func supplierHistoricalPublishedDayCanonicalIndex(db *gorm.DB) (supplierHistoricalIndexDefinition, bool, error)
func loadSupplierHistoricalMySQLIndexes(db *gorm.DB) ([]supplierHistoricalIndexDefinition, error)
func loadSupplierHistoricalSQLiteCanonicalIndex(db *gorm.DB) (supplierHistoricalIndexDefinition, bool, error)
func loadSupplierHistoricalPostgresCanonicalIndex(db *gorm.DB) (supplierHistoricalIndexDefinition, bool, error)
```

The MySQL query must use only MySQL 5.7-compatible columns:

```sql
SELECT index_name, non_unique, seq_in_index, column_name, sub_part
FROM information_schema.statistics
WHERE table_schema = DATABASE() AND table_name = ?
ORDER BY index_name, seq_in_index
```

Normalize rows into one definition per index, validate consistent uniqueness and contiguous `seq_in_index`, represent `column_name IS NULL` as `HasExpression`, and represent non-null `sub_part` as `HasPrefix`.

The SQLite reader must use fixed-name `PRAGMA index_list('supplier_historical_published_days')` and `PRAGMA index_info('ux_supplier_historical_published_day_date')`, validating `unique=1`, `partial=0`, ordered column `date`.

The PostgreSQL reader must query `pg_index`, `pg_class`, `pg_namespace`, `unnest(ix.indkey) WITH ORDINALITY`, and `pg_attribute`, restricted to `current_schema()`, and must reject partial or expression definitions through `ix.indpred` and `ix.indexprs`.

- [ ] **Step 4: Add safe explicit canonical index creation**

Use GORM clause identifiers, never string interpolation:

```go
func createSupplierHistoricalPublishedDayDateIndex(db *gorm.DB) error {
	statement := "CREATE UNIQUE INDEX ? ON ? (?)"
	if db.Dialector.Name() == "sqlite" || db.Dialector.Name() == "postgres" {
		statement = "CREATE UNIQUE INDEX IF NOT EXISTS ? ON ? (?)"
	}
	return db.Exec(
		statement,
		clause.Column{Name: supplierHistoricalPublishedDayDateIndexName},
		clause.Table{Name: supplierHistoricalPublishedDayTableName},
		clause.Column{Name: supplierHistoricalPublishedDayDateColumn},
	).Error
}
```

After creation, always reload the canonical definition and require `isExactUniqueDateIndex()`.

- [ ] **Step 5: Add the MySQL pinned-connection advisory-lock boundary**

Use constants:

```go
const (
	supplierHistoricalPublishedDaySchemaLockName = "newapi:supplier_historical_published_days:date_unique"
	supplierHistoricalPublishedDayLockTimeoutSec = 30
)
```

Acquire and release with scalar scans:

```sql
SELECT GET_LOCK(?, ?)
SELECT RELEASE_LOCK(?)
```

Require a valid return value of `1`. Use `db.Connection(func(tx *gorm.DB) (err error) { ... })` and a deferred release on that same pinned connection. Join a release error to any earlier error with `errors.Join`.

- [ ] **Step 6: Apply the MySQL repair plan using quoted identifiers**

For every drop, execute:

```go
tx.Exec(
	"DROP INDEX ? ON ?",
	clause.Column{Name: indexName},
	clause.Table{Name: supplierHistoricalPublishedDayTableName},
)
```

For a rename, execute MySQL 5.7.8-compatible DDL:

```go
tx.Exec(
	"ALTER TABLE ? RENAME INDEX ? TO ?",
	clause.Table{Name: supplierHistoricalPublishedDayTableName},
	clause.Column{Name: plan.RenameFrom},
	clause.Column{Name: supplierHistoricalPublishedDayDateIndexName},
)
```

Do not catch-and-ignore DDL errors. Reload metadata after repair and require the next plan to be empty before continuing.

- [ ] **Step 7: Implement the public dedicated migration**

The function contract is:

```go
func MigrateSupplierHistoricalPublishedDaySchema(db *gorm.DB) error {
	if db == nil {
		return ErrDatabase
	}
	if db.Dialector.Name() != "mysql" {
		if err := db.AutoMigrate(&SupplierHistoricalPublishedDay{}); err != nil {
			return fmt.Errorf("migrate supplier historical published-day table: %w", err)
		}
		return ensureSupplierHistoricalPublishedDayDateIndex(db)
	}
	return migrateSupplierHistoricalPublishedDaySchemaMySQL(db)
}
```

Inside the MySQL lock:

1. If the table exists, load metadata, plan, and apply the exact repair.
2. Run tag-free `AutoMigrate(&SupplierHistoricalPublishedDay{})`.
3. Ensure the canonical index, creating it only if absent.
4. Reload and verify the canonical exact definition and the absence of remaining repair actions.
5. Release the lock.

- [ ] **Step 8: Run the focused schema tests and verify GREEN**

Run:

```powershell
go test ./model -run '^(TestMigrateSupplierHistoricalPublishedDaySchema.*|TestPlanSupplierHistoricalPublishedDayDateIndexRepair.*|TestSupplierHistoricalSchemaIndexesAcrossSupportedDialects)$' -count=1 -v
```

Expected: PASS.

- [ ] **Step 9: Commit the schema migration slice using Lore format**

The message must record the 64-key constraint, rejection of unrelated-index deletion, multi-node advisory-lock decision, focused tests, and absence of a local live-MySQL test.

### Task 4: Wire normal/fast startup and preserve test fixture constraints

**Files:**
- Modify: `model/main.go:250-490`
- Modify: `model/supplier_historical_estimate_test.go:19-24`
- Modify: `model/supplier_report_test.go:13-17`
- Modify: `model/supplier_accounting_integration_test.go:148-164`
- Modify: `model/supplier_usage_schema_test.go:12-25`
- Modify: `controller/supplier_historical_estimate_test.go:13-23`
- Modify: `service/supplier_report_snapshot_scope_test.go:14-28`
- Modify: `model/supplier_test.go:139-174`

- [ ] **Step 1: Strengthen the existing fast-migration test before wiring the helper**

Add these assertions to `TestMigrateDBFastRegistersSupplierModels`:

```go
require.True(t, db.Migrator().HasTable(&SupplierHistoricalImport{}))
require.True(t, db.Migrator().HasTable(&SupplierHistoricalDailySummary{}))
require.True(t, db.Migrator().HasTable(&SupplierHistoricalPublishedDay{}))
definition, found, err := supplierHistoricalPublishedDayCanonicalIndex(db)
require.NoError(t, err)
require.True(t, found)
require.True(t, definition.isExactUniqueDateIndex())
```

- [ ] **Step 2: Run the fast-migration test and verify RED**

Run:

```powershell
go test ./model -run '^TestMigrateDBFastRegistersSupplierModels$' -count=1 -v
```

Expected: FAIL because generic tag-free `AutoMigrate` creates the table without the canonical unique index.

- [ ] **Step 3: Wire the normal migration path**

Remove `&SupplierHistoricalPublishedDay{}` from the large `DB.AutoMigrate(...)` list. Immediately after that call succeeds, add:

```go
if err := MigrateSupplierHistoricalPublishedDaySchema(DB); err != nil {
	return err
}
```

- [ ] **Step 4: Wire the fast migration path**

Remove the `SupplierHistoricalPublishedDay` entry from the generic `migrations` slice. After the loop succeeds, add:

```go
if err := MigrateSupplierHistoricalPublishedDaySchema(DB); err != nil {
	return fmt.Errorf("failed to migrate SupplierHistoricalPublishedDay: %w", err)
}
```

- [ ] **Step 5: Update every direct historical-schema fixture**

In each listed fixture, remove `SupplierHistoricalPublishedDay` from the generic `AutoMigrate` arguments and immediately call the dedicated migration. External packages use the exported model function:

```go
require.NoError(t, model.MigrateSupplierHistoricalPublishedDaySchema(db))
```

Model-package tests use:

```go
require.NoError(t, MigrateSupplierHistoricalPublishedDaySchema(db))
```

Do not change `DropTable` lists; the physical table name is unchanged.

- [ ] **Step 6: Run the fast migration, historical, report, controller, and service tests**

Run:

```powershell
go test ./model -run '^(TestMigrateDBFastRegistersSupplierModels|TestSupplierHistorical|TestPublishedHistoricalBaseline|TestSupplierV1)' -count=1 -v
go test ./controller -run '^Test.*SupplierHistorical' -count=1 -v
go test ./service -run '^TestSupplierReportService' -count=1 -v
```

Expected: PASS.

- [ ] **Step 7: Commit startup wiring and fixture preservation using Lore format**

The `Directive:` trailer must state that `SupplierHistoricalPublishedDay` must never be re-added to a generic `AutoMigrate` collection while its canonical index is managed explicitly.

### Task 5: Verify locally and request independent review

**Files:**
- Review all files changed since design commit `df68bfd1a`

- [ ] **Step 1: Format the Go files**

Run:

```powershell
gofmt -w model/supplier_historical_published_day_schema.go model/supplier_historical_published_day_schema_test.go model/supplier_historical_estimate.go model/supplier_historical_estimate_test.go model/main.go model/supplier_report_test.go model/supplier_accounting_integration_test.go model/supplier_usage_schema_test.go model/supplier_test.go controller/supplier_historical_estimate_test.go service/supplier_report_snapshot_scope_test.go
```

- [ ] **Step 2: Run targeted tests again after formatting**

Run the exact commands from Tasks 2-4. Expected: PASS.

- [ ] **Step 3: Run full model and affected-package regression**

Run:

```powershell
go test ./model/... -count=1
go test ./controller/... ./service/... -count=1
go vet ./model/... ./controller/... ./service/...
go build ./...
git diff --check
```

Expected: every command exits `0` with no test failures or whitespace errors.

- [ ] **Step 4: Run GitNexus change detection or record the analyzer failure**

Run:

```powershell
gitnexus detect-changes --scope compare --base-ref main
```

If the Windows analyzer remains unavailable, preserve the exact error and use the manual call map plus reviewer evidence; do not claim GitNexus passed.

- [ ] **Step 5: Dispatch an independent code review**

Give the reviewer the design, this plan, `BASE_SHA=df68bfd1a`, current `HEAD_SHA`, and the complete diff. Require findings by severity plus deployment advice:

- `Router deploy`: expected `not required`; production router nodes run with `NODE_TYPE=slave`, skip `migrateDB`, and do not use the supplier historical reporting table in relay paths. Production is not authorized in this task.
- `Other deploy targets`: staging console/backend now; no website, Terraform, Cloudflare, or production action.
- `Risk / validation`: MySQL DDL and multi-node startup serialization; staging is the required live-MySQL validation.

- [ ] **Step 6: Fix every critical or important review finding with a new RED/GREEN cycle**

Re-run the relevant focused test before and after each repair, then repeat the full regression gate.

### Task 6: Commit, push, and validate staging end to end

**Files:**
- Commit only the intended tracked files; exclude `.gitnexus/`, `scripts/browser_qa/**/__pycache__/`, and `tmp_*_diff.txt`.

- [ ] **Step 1: Perform the final staged-file audit**

Run:

```powershell
git status --short
git diff --cached --name-status
git diff --cached --check
```

Expected: only intended design/plan/model/test/doc files are staged; generated caches and temporary diffs are not staged.

- [ ] **Step 2: Create the final Lore-format implementation commit if any task changes remain uncommitted**

The intent line must explain why staging startup needed the change. Trailers must include `Constraint`, `Rejected`, `Confidence`, `Scope-risk`, `Directive`, `Tested`, and `Not-tested`.

- [ ] **Step 3: Push the feature branch, then push the same verified commit to remote `staging`**

Run:

```powershell
git push origin ops/browser-qa-live-acceptance-20260802
git push origin HEAD:staging
```

Expected: both pushes succeed and point to the same implementation commit.

- [ ] **Step 4: Observe the new staging workflow**

Use GitHub CLI to find the run triggered by the pushed commit, then watch it to completion. Required evidence:

- backend image build and push succeed;
- the new Cloud Run revision becomes healthy instead of failing with MySQL error 1069;
- Browser QA core is not skipped;
- main strict replay passes;
- cleanup passes;
- root report manifest has `status=passed`.

- [ ] **Step 5: If Browser QA fails after backend recovery, continue from the safe stage code**

Download the run artifact or read the GCS `codex-events.jsonl`. Use only the fixed helper stage code (`init_connect_failed`, `init_context_failed`, `init_websocket_block_failed`, `init_download_block_failed`, `init_service_worker_block_failed`, `init_page_failed`, `init_service_worker_bypass_failed`, `init_failed`, or `command_failed`) to drive the next systematic debugging cycle. Never expose raw secret-bearing exception text.

### Task 7: Update portable HTML documentation after live success

**Files:**
- Modify: `docs/browser-qa/browser-qa-technical-implementation.html`
- Modify: `docs/browser-qa/ai-browser-testing-migration-guide.html`

- [ ] **Step 1: Add the staging recovery evidence to the technical implementation document**

Document the workflow run, healthy revision, Browser QA result, the GORM/MySQL root cause, the dedicated migration boundary, and the fact that production remains unmodified.

- [ ] **Step 2: Add the portable migration lesson to the cross-project guide**

Explain that record/replay automation depends on a healthy deploy target, startup migrations must be idempotent, single-field GORM unique tags can produce column-level DDL, live test environments need schema-drift observability, and alert-only QA must distinguish deployment failure from replay failure.

- [ ] **Step 3: Validate both HTML files**

Run the repository's existing Browser QA documentation/contract tests that cover these files, plus:

```powershell
git diff --check -- docs/browser-qa/browser-qa-technical-implementation.html docs/browser-qa/ai-browser-testing-migration-guide.html
```

Open both local files in the in-app browser and verify headings, navigation, code blocks, and tables render without overflow or broken markup.

- [ ] **Step 4: Commit and push the documentation evidence using Lore format**

Push the documentation commit to the feature branch and `staging` only if the staging workflow paths do not unnecessarily redeploy the backend for docs-only changes. Otherwise keep the verified implementation commit as the staging evidence and push docs only to the feature branch.

## Stop Condition

Stop only when the backend revision is healthy, strict replay passes, cleanup passes, the root manifest reports `status=passed`, both HTML documents reflect the verified result, all local regression gates pass, independent review has no unresolved critical or important finding, and no production/main deployment has occurred.
