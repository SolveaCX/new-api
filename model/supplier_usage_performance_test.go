package model

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const supplierAccountingPerfRowsEnv = "SUPPLIER_ACCOUNTING_PERF_ROWS"

func openSupplierAccountingPerfSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?cache=private&_pragma=journal_mode(WAL)&_pragma=synchronous(OFF)&_pragma=temp_store(MEMORY)", t.TempDir()+"/supplier-perf.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	return db
}

func supplierAccountingQueryPlan(t *testing.T, db *gorm.DB, continuation bool) []string {
	t.Helper()
	query := `EXPLAIN QUERY PLAN SELECT id, created_at, channel_id, model_name, other
		FROM logs
		WHERE type = ? AND created_at >= ? AND created_at < ?`
	args := []any{LogTypeConsume, int64(1_700_000_000), int64(1_700_086_400)}
	if continuation {
		query += ` AND (created_at > ? OR (created_at = ? AND id > ?))`
		args = append(args, int64(1_700_000_100), int64(1_700_000_100), 100)
	}
	query += ` ORDER BY created_at ASC, id ASC LIMIT 1000`

	type planRow struct {
		Detail string `gorm:"column:detail"`
	}
	var rows []planRow
	require.NoError(t, db.Raw(query, args...).Scan(&rows).Error)
	require.NotEmpty(t, rows)
	plan := make([]string, 0, len(rows))
	for _, row := range rows {
		plan = append(plan, row.Detail)
	}
	return plan
}

func requireSupplierAccountingPlanUsesTypeCreatedAtIndex(t *testing.T, plan []string) {
	t.Helper()
	joined := strings.ToLower(strings.Join(plan, " | "))
	require.Contains(t, joined, "idx_type_created_at_quota", "query plan must use the existing (type, created_at, quota) index: %s", joined)
	require.NotContains(t, joined, "scan logs", "query must not perform a full logs table scan: %s", joined)
}

func TestScanSupplierAccountingLogsUsesExistingTypeCreatedAtIndex(t *testing.T) {
	db := openSupplierAccountingPerfSQLite(t)

	firstPagePlan := supplierAccountingQueryPlan(t, db, false)
	requireSupplierAccountingPlanUsesTypeCreatedAtIndex(t, firstPagePlan)
	t.Logf("first page query plan: %s", strings.Join(firstPagePlan, " | "))

	continuationPlan := supplierAccountingQueryPlan(t, db, true)
	requireSupplierAccountingPlanUsesTypeCreatedAtIndex(t, continuationPlan)
	t.Logf("continuation query plan: %s", strings.Join(continuationPlan, " | "))
}

func TestLogSchemaKeepsSupplierAccountingOutOfPhysicalColumns(t *testing.T) {
	db := openSupplierAccountingPerfSQLite(t)

	columnTypes, err := db.Migrator().ColumnTypes(&Log{})
	require.NoError(t, err)
	actualColumns := make([]string, 0, len(columnTypes))
	for _, column := range columnTypes {
		actualColumns = append(actualColumns, column.Name())
	}
	sort.Strings(actualColumns)
	expectedColumns := []string{
		"channel_id", "channel_name", "completion_tokens", "content", "created_at", "group", "id", "ip",
		"is_stream", "model_name", "other", "prompt_tokens", "quota", "request_id", "token_id",
		"token_name", "type", "upstream_request_id", "use_time", "user_id", "username",
	}
	sort.Strings(expectedColumns)
	require.Equal(t, expectedColumns, actualColumns)
	for _, column := range actualColumns {
		require.NotContains(t, column, "supplier")
		require.NotContains(t, column, "procurement")
		require.NotContains(t, column, "official")
	}

	type sqliteIndex struct {
		Name string `gorm:"column:name"`
	}
	var indexes []sqliteIndex
	require.NoError(t, db.Raw("PRAGMA index_list('logs')").Scan(&indexes).Error)
	indexNames := make([]string, 0, len(indexes))
	for _, index := range indexes {
		indexNames = append(indexNames, index.Name)
	}
	require.Contains(t, indexNames, "idx_type_created_at_quota")
	require.Contains(t, indexNames, "idx_created_at_id")
}

func TestSupplierAccountingFreshMigrationDoesNotAlterLogs(t *testing.T) {
	db := openSupplierAccountingPerfSQLite(t)
	columnsBefore, indexesBefore := sqliteLogSchema(t, db)

	supplierTables := []any{
		&UpstreamSupplier{},
		&SupplierContract{},
		&SupplierContractRateVersion{},
		&SupplierChannelBindingVersion{},
		&SupplierInventoryAdjustment{},
		&SupplierStatisticsExclusionRule{},
		&SupplierAdminCommand{},
		&SupplierUsageDailySummary{},
		&SupplierUsageDailyBatchRun{},
	}
	require.Len(t, supplierTables, 9)
	require.NoError(t, db.AutoMigrate(supplierTables...))
	for _, table := range supplierTables {
		require.True(t, db.Migrator().HasTable(table))
	}

	columnsAfter, indexesAfter := sqliteLogSchema(t, db)
	require.Equal(t, columnsBefore, columnsAfter, "supplier migration must not add or alter logs columns")
	require.Equal(t, indexesBefore, indexesAfter, "supplier migration must not add or alter logs indexes")
}

func sqliteLogSchema(t *testing.T, db *gorm.DB) ([]string, []string) {
	t.Helper()
	type schemaRow struct {
		SQL string `gorm:"column:sql"`
	}
	var columns []schemaRow
	require.NoError(t, db.Raw("SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'logs'").Scan(&columns).Error)
	require.Len(t, columns, 1)
	var indexes []schemaRow
	require.NoError(t, db.Raw("SELECT sql FROM sqlite_master WHERE type = 'index' AND tbl_name = 'logs' AND sql IS NOT NULL ORDER BY name").Scan(&indexes).Error)
	indexSQL := make([]string, 0, len(indexes))
	for _, index := range indexes {
		indexSQL = append(indexSQL, index.SQL)
	}
	return []string{columns[0].SQL}, indexSQL
}

func TestSupplierAccountingScanConfiguredRowsPerformance(t *testing.T) {
	rawRows := strings.TrimSpace(os.Getenv(supplierAccountingPerfRowsEnv))
	if rawRows == "" {
		t.Skipf("set %s=1000000 to run the million-row performance gate", supplierAccountingPerfRowsEnv)
	}
	totalRows, err := strconv.Atoi(rawRows)
	require.NoError(t, err)
	require.GreaterOrEqual(t, totalRows, 100_000)

	db := openSupplierAccountingPerfSQLite(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	insertSupplierAccountingPerfRows(t, sqlDB, totalRows)

	plan := supplierAccountingQueryPlan(t, db, true)
	requireSupplierAccountingPlanUsesTypeCreatedAtIndex(t, plan)

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	startedAt := time.Now()
	var previousCreatedAt int64
	var previousID int
	var consumedRows int64
	pages := 0
	scanned, err := ScanSupplierAccountingLogs(context.Background(), db, 1_700_000_000, 1_700_086_400, SupplierDailyLogPageSize, func(rows []SupplierAccountingLogRow) error {
		pages++
		require.LessOrEqual(t, len(rows), SupplierDailyLogPageSize)
		for _, row := range rows {
			if row.CreatedAt < previousCreatedAt || (row.CreatedAt == previousCreatedAt && row.Id <= previousID) {
				return fmt.Errorf("non-monotonic keyset: previous=(%d,%d), current=(%d,%d)", previousCreatedAt, previousID, row.CreatedAt, row.Id)
			}
			previousCreatedAt, previousID = row.CreatedAt, row.Id
			consumedRows++
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, int64(totalRows), scanned)
	require.Equal(t, scanned, consumedRows)
	elapsed := time.Since(startedAt)

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	rowsPerSecond := float64(scanned) / elapsed.Seconds()
	totalAllocatedMiB := float64(after.TotalAlloc-before.TotalAlloc) / (1024 * 1024)
	heapDeltaMiB := float64(int64(after.HeapAlloc)-int64(before.HeapAlloc)) / (1024 * 1024)
	t.Logf("supplier accounting scan: rows=%d pages=%d elapsed=%s rows/sec=%.0f total_alloc=%.1fMiB heap_delta=%.1fMiB plan=%s",
		scanned, pages, elapsed.Round(time.Millisecond), rowsPerSecond, totalAllocatedMiB, heapDeltaMiB, strings.Join(plan, " | "))
	require.Greater(t, rowsPerSecond, 10_000.0, "SQLite scan throughput unexpectedly low")
	require.Less(t, heapDeltaMiB, 64.0, "keyset scan must not retain memory proportional to total row count")
}

func insertSupplierAccountingPerfRows(t *testing.T, db *sql.DB, totalRows int) {
	t.Helper()
	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	statement, err := tx.Prepare(`INSERT INTO logs(type, created_at, channel_id, model_name, other) VALUES (?, ?, ?, ?, ?)`)
	require.NoError(t, err)
	defer statement.Close()

	const snapshot = `{"supplier_accounting_v1":{"schema_version":1,"supplier_id":1,"contract_id":1,"rate_version_id":1,"statistics_scope":"business","financially_committed_at":1700000000}}`
	for row := 0; row < totalRows; row++ {
		createdAt := int64(1_700_000_000 + row/1000)
		_, err = statement.Exec(LogTypeConsume, createdAt, row%32+1, "perf-model", snapshot)
		require.NoError(t, err)
	}
	require.NoError(t, tx.Commit())
}
