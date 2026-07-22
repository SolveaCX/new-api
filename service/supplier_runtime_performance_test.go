package service

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const supplierAccountingE2EPerfRowsEnv = "SUPPLIER_ACCOUNTING_E2E_PERF_ROWS"
const supplierAccountingHighCardinalityRowsEnv = "SUPPLIER_ACCOUNTING_HIGH_CARDINALITY_ROWS"

func TestRunSupplierDailyBatchConfiguredRowsPerformance(t *testing.T) {
	rawRows := strings.TrimSpace(os.Getenv(supplierAccountingE2EPerfRowsEnv))
	if rawRows == "" {
		t.Skipf("set %s=1000000 to run the end-to-end million-row T+1 performance gate", supplierAccountingE2EPerfRowsEnv)
	}
	totalRows, err := strconv.Atoi(rawRows)
	require.NoError(t, err)
	require.GreaterOrEqual(t, totalRows, 1_000_000)

	mainDB := openSupplierAccountingE2EPerfSQLite(t, "main.db")
	logDB := openSupplierAccountingE2EPerfSQLite(t, "log.db")
	require.NoError(t, mainDB.AutoMigrate(&model.Option{}, &model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{}))
	require.NoError(t, logDB.AutoMigrate(&model.Log{}))

	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	_, err = model.GetOrCreateSupplierAccountingCoverageStart(context.Background(), mainDB, day.Unix())
	require.NoError(t, err)
	insertSupplierAccountingE2EPerfRows(t, logDB, day, totalRows)

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	startedAt := time.Now()
	require.NoError(t, RunSupplierDailyBatch(
		context.Background(), mainDB, logDB, day.Format("2006-01-02"),
		"supplier-e2e-performance", day.AddDate(0, 0, 2), false,
	))
	elapsed := time.Since(startedAt)
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	var run model.SupplierUsageDailyBatchRun
	require.NoError(t, mainDB.Where("batch_date = ?", day.Format("2006-01-02")).First(&run).Error)
	require.Equal(t, model.SupplierDailyBatchStatusCompleted, run.Status)
	require.Equal(t, int64(totalRows), run.LogsScanned)
	require.Equal(t, int64(totalRows), run.SnapshotCount)
	require.EqualValues(t, 4, run.SummaryCount)

	var summaries []model.SupplierUsageDailySummary
	require.NoError(t, mainDB.Order("channel_id ASC").Find(&summaries).Error)
	require.Len(t, summaries, 4)
	requireSupplierAccountingE2EPerfTotals(t, summaries, totalRows)

	rowsPerSecond := float64(totalRows) / elapsed.Seconds()
	totalAllocatedMiB := float64(after.TotalAlloc-before.TotalAlloc) / (1024 * 1024)
	retainedHeapMiB := float64(int64(after.HeapAlloc)-int64(before.HeapAlloc)) / (1024 * 1024)
	t.Logf("supplier accounting T+1 end-to-end: rows=%d summaries=%d elapsed=%s rows/sec=%.0f total_alloc=%.1fMiB retained_heap_delta=%.1fMiB",
		totalRows, len(summaries), elapsed.Round(time.Millisecond), rowsPerSecond, totalAllocatedMiB, retainedHeapMiB)
	require.Greater(t, rowsPerSecond, 10_000.0, "end-to-end SQLite batch throughput unexpectedly low")
	require.Less(t, retainedHeapMiB, 64.0, "T+1 batch must retain bounded heap independent of scanned row count")
}

func TestRunSupplierDailyBatchHighCardinalityConfiguredRows(t *testing.T) {
	rawRows := strings.TrimSpace(os.Getenv(supplierAccountingHighCardinalityRowsEnv))
	if rawRows == "" {
		t.Skipf("set %s=1000000 to run the near-unique-dimension bounded-page gate", supplierAccountingHighCardinalityRowsEnv)
	}
	totalRows, err := strconv.Atoi(rawRows)
	require.NoError(t, err)
	require.GreaterOrEqual(t, totalRows, 1_000_000)

	mainDB := openSupplierAccountingE2EPerfSQLite(t, "high-cardinality-main.db")
	logDB := openSupplierAccountingE2EPerfSQLite(t, "high-cardinality-log.db")
	require.NoError(t, mainDB.AutoMigrate(&model.Option{}, &model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{}))
	require.NoError(t, logDB.AutoMigrate(&model.Log{}))
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	_, err = model.GetOrCreateSupplierAccountingCoverageStart(context.Background(), mainDB, day.Unix())
	require.NoError(t, err)
	insertSupplierAccountingHighCardinalityRows(t, logDB, day, totalRows)

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	var peakHeap atomic.Uint64
	peakHeap.Store(before.HeapAlloc)
	stopSampling := make(chan struct{})
	samplingStopped := make(chan struct{})
	go func() {
		defer close(samplingStopped)
		ticker := time.NewTicker(2 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopSampling:
				return
			case <-ticker.C:
				var sample runtime.MemStats
				runtime.ReadMemStats(&sample)
				for current := peakHeap.Load(); sample.HeapAlloc > current && !peakHeap.CompareAndSwap(current, sample.HeapAlloc); current = peakHeap.Load() {
				}
			}
		}
	}()
	require.NoError(t, RunSupplierDailyBatch(context.Background(), mainDB, logDB, day.Format("2006-01-02"), "high-cardinality", day.AddDate(0, 0, 2), false))
	close(stopSampling)
	<-samplingStopped
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	var run model.SupplierUsageDailyBatchRun
	require.NoError(t, mainDB.Where("batch_date = ?", day.Format("2006-01-02")).First(&run).Error)
	require.Equal(t, int64(totalRows), run.LogsScanned)
	require.Equal(t, int64(totalRows), run.SnapshotCount)
	require.Equal(t, int64(totalRows), run.SummaryCount)
	retainedHeapMiB := float64(int64(after.HeapAlloc)-int64(before.HeapAlloc)) / (1024 * 1024)
	peakHeapDeltaMiB := float64(peakHeap.Load()-before.HeapAlloc) / (1024 * 1024)
	// One page contains at most 5000 log strings, aggregate rows, and GORM
	// upsert buffers. A 40 KiB/row budget catches a daily-cardinality map while
	// leaving headroom for SQLite/GORM transient allocations.
	peakHeapLimitMiB := float64(model.SupplierDailyLogPageSize*40*1024) / (1024 * 1024)
	t.Logf("supplier accounting high-cardinality gate: rows=%d summaries=%d peak_heap_delta=%.1fMiB retained_heap_delta=%.1fMiB", totalRows, run.SummaryCount, peakHeapDeltaMiB, retainedHeapMiB)
	require.Less(t, peakHeapDeltaMiB, peakHeapLimitMiB, "peak aggregation heap must be bounded by one 5000-row page")
	require.Less(t, retainedHeapMiB, 96.0, "aggregation heap must be bounded by one 5000-row page, not daily dimensions")
}

func openSupplierAccountingE2EPerfSQLite(t *testing.T, name string) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?cache=private&_pragma=journal_mode(WAL)&_pragma=synchronous(OFF)&_pragma=temp_store(MEMORY)", t.TempDir()+"/"+name)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	return db
}

func insertSupplierAccountingE2EPerfRows(t *testing.T, db *gorm.DB, day time.Time, totalRows int) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	statement, err := tx.Prepare(`INSERT INTO logs(type, created_at, channel_id, model_name, other) VALUES (?, ?, ?, ?, ?)`)
	require.NoError(t, err)
	defer func() { require.NoError(t, statement.Close()) }()

	const businessSnapshot = `{"supplier_accounting_v1":{"bv":1,"s":1,"c":1,"rv":1,"pm":650000,"sm":700000,"ol":1000,"sa":700,"pc":650,"gp":50,"ss":"business","ed":"included","q":"1000000","p":"ratio","fc":1784476801}}`
	const internalSnapshot = `{"supplier_accounting_v1":{"bv":1,"s":1,"c":1,"rv":1,"pm":650000,"ol":1000,"pc":650,"ss":"internal","ed":"excluded","er":99,"fc":1784476801}}`
	for row := 0; row < totalRows; row++ {
		pattern := row % 4
		other := businessSnapshot
		if pattern >= 2 {
			other = internalSnapshot
		}
		_, err = statement.Exec(
			model.LogTypeConsume,
			day.Unix()+1+int64(row/1000),
			pattern+1,
			fmt.Sprintf("perf-model-%d", pattern+1),
			other,
		)
		require.NoError(t, err)
	}
	require.NoError(t, tx.Commit())
}

func insertSupplierAccountingHighCardinalityRows(t *testing.T, db *gorm.DB, day time.Time, totalRows int) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	statement, err := tx.Prepare(`INSERT INTO logs(type, created_at, channel_id, model_name, other) VALUES (?, ?, ?, ?, ?)`)
	require.NoError(t, err)
	defer func() { require.NoError(t, statement.Close()) }()
	const snapshot = `{"supplier_accounting_v1":{"bv":1,"s":1,"c":1,"rv":1,"pm":650000,"sm":700000,"ol":1000,"sa":700,"pc":650,"gp":50,"ss":"business","ed":"included","q":"1000000","p":"ratio","fc":1784476801}}`
	for row := 0; row < totalRows; row++ {
		_, err = statement.Exec(model.LogTypeConsume, day.Unix()+1+int64(row/1000), 1, fmt.Sprintf("high-cardinality-model-%d", row), snapshot)
		require.NoError(t, err)
	}
	require.NoError(t, tx.Commit())
}

func requireSupplierAccountingE2EPerfTotals(t *testing.T, summaries []model.SupplierUsageDailySummary, totalRows int) {
	t.Helper()
	var requestCount int64
	var businessRequests int64
	var internalRequests int64
	for _, summary := range summaries {
		expectedRequests := supplierAccountingE2EPatternCount(totalRows, summary.ChannelId-1)
		require.Equal(t, expectedRequests, summary.RequestCount)
		require.Equal(t, expectedRequests, summary.OfficialListKnownCount)
		require.Equal(t, expectedRequests*1_000, summary.OfficialListMicroUsd)
		require.Equal(t, expectedRequests, summary.ProcurementCostKnownCount)
		require.Equal(t, expectedRequests*650, summary.ProcurementCostMicroUsd)
		requestCount += summary.RequestCount

		switch summary.StatisticsScope {
		case "business":
			require.NotEmpty(t, summary.ModelName)
			require.Equal(t, expectedRequests, summary.SalesKnownCount)
			require.Equal(t, expectedRequests*700, summary.SalesMicroUsd)
			require.Equal(t, expectedRequests, summary.GrossProfitKnownCount)
			require.Equal(t, expectedRequests*50, summary.GrossProfitMicroUsd)
			require.Equal(t, expectedRequests, summary.GrossMarginEligibleCount)
			require.Equal(t, expectedRequests*700, summary.GrossMarginEligibleSalesMicroUsd)
			businessRequests += summary.RequestCount
		case "internal":
			require.Empty(t, summary.ModelName)
			require.Zero(t, summary.SalesKnownCount)
			require.Zero(t, summary.SalesMicroUsd)
			require.Zero(t, summary.GrossProfitKnownCount)
			require.Zero(t, summary.GrossProfitMicroUsd)
			require.Zero(t, summary.GrossMarginEligibleCount)
			require.Zero(t, summary.GrossMarginEligibleSalesMicroUsd)
			internalRequests += summary.RequestCount
		default:
			t.Fatalf("unexpected statistics scope %q", summary.StatisticsScope)
		}
	}
	require.Equal(t, int64(totalRows), requestCount)
	require.Equal(t, supplierAccountingE2EPatternCount(totalRows, 0)+supplierAccountingE2EPatternCount(totalRows, 1), businessRequests)
	require.Equal(t, supplierAccountingE2EPatternCount(totalRows, 2)+supplierAccountingE2EPatternCount(totalRows, 3), internalRequests)
}

func supplierAccountingE2EPatternCount(totalRows, pattern int) int64 {
	if totalRows <= pattern {
		return 0
	}
	return int64((totalRows-1-pattern)/4 + 1)
}
