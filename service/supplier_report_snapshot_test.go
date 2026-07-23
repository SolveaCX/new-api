package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type supplierReportSnapshotResult struct {
	overview SupplierReportOverview
	err      error
}

func TestSupplierReportPublishedViewUsesOneSQLiteSnapshot(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "supplier-report-snapshot.db") +
		"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(8)
	var journalMode string
	require.NoError(t, db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error)
	require.Equal(t, "wal", strings.ToLower(journalMode))
	runSupplierReportPublishedViewSnapshotBarrier(t, db)
}

// runSupplierReportPublishedViewSnapshotBarrier is dialect-agnostic so the
// cross-database suite can reuse the same real publish/read interleaving.
func runSupplierReportPublishedViewSnapshotBarrier(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(
		&model.UpstreamSupplier{}, &model.SupplierContract{}, &model.SupplierContractRateVersion{},
		&model.SupplierInventoryAdjustment{}, &model.Channel{}, &model.SupplierAccountingCoverageGap{},
		&model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{},
	))
	location, err := time.LoadLocation(SupplierReportTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	dayEnd := day.AddDate(0, 0, 1)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.UpstreamSupplier{
		Id: 1, Name: "snapshot supplier", Status: model.SupplierStatusActive,
	}).Error)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.SupplierContract{
		Id: 2, SupplierId: 1, Name: "snapshot contract", ContractNo: "SNAPSHOT-1", Status: model.SupplierContractStatusActive,
	}).Error)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.SupplierContractRateVersion{
		Id: 3, ContractId: 2, ProcurementMultiplierPpm: 1_000_000, EffectiveAt: day.Unix() - 1, CreatedBy: 1,
	}).Error)
	contractID := 2
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.Channel{
		Id: 4, Name: "snapshot channel", SupplierContractId: &contractID,
	}).Error)

	ctx := context.Background()
	oldLease, err := model.AcquireSupplierDailyBatch(
		ctx, db, day.Format("2006-01-02"), day.Unix(), dayEnd.Unix(), "snapshot-old", time.Now(), 5*time.Minute, false,
	)
	require.NoError(t, err)
	require.NoError(t, model.PersistSupplierDailyBatchPage(
		ctx, db, oldLease, []model.SupplierUsageDailySummary{supplierReportSnapshotSummary(day, 100)},
		day.Add(time.Hour).Unix(), 1, 2, 1, 5*time.Minute,
	))
	oldEvidence := types.SupplierPublishedEvidenceV1{
		SchemaVersion: types.SupplierPublishedEvidenceSchemaVersion, LogsScanned: 2,
		ProducerMarkersPresent: 2, CapturedSnapshotCount: 1,
		DispositionCounts:                types.SupplierPublishedDispositionCountsV1{Captured: 1, ProducerError: 1},
		PersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessIncomplete,
		Warnings: []types.SupplierPublishedWarningV1{{
			Code: types.SupplierPublishedWarningProducerError, Count: 1, MessageKey: "supply_chain.warning.producer_error",
		}},
	}
	require.NoError(t, model.PublishSupplierDailyBatch(ctx, db, oldLease, dayEnd.Add(time.Second), oldEvidence))

	newLease, err := model.AcquireSupplierDailyBatchRerun(
		ctx, db, oldLease.BatchDate, day.Unix(), dayEnd.Unix(), "snapshot-new", time.Now(), 5*time.Minute, oldLease.FenceToken,
	)
	require.NoError(t, err)
	require.NoError(t, model.PersistSupplierDailyBatchPage(
		ctx, db, newLease, []model.SupplierUsageDailySummary{supplierReportSnapshotSummary(day, 900)},
		day.Add(2*time.Hour).Unix(), 2, 1, 1, 5*time.Minute,
	))
	newEvidence := types.SupplierPublishedEvidenceV1{
		SchemaVersion: types.SupplierPublishedEvidenceSchemaVersion, LogsScanned: 1,
		ProducerMarkersPresent: 1, CapturedSnapshotCount: 1,
		DispositionCounts:                types.SupplierPublishedDispositionCountsV1{Captured: 1},
		PersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessComplete,
		Warnings:                         []types.SupplierPublishedWarningV1{},
	}

	evidenceRead := make(chan struct{})
	releaseReport := make(chan struct{})
	var blockOnce sync.Once
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseReport) }) }
	callbackName := fmt.Sprintf("test:supplier_report_snapshot:%s", t.Name())
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "supplier_usage_daily_batch_runs" {
			return
		}
		query := strings.ToLower(tx.Statement.SQL.String())
		if !strings.Contains(query, "day_start <") || !strings.Contains(query, "day_end >") {
			return
		}
		blockOnce.Do(func() {
			close(evidenceRead)
			<-releaseReport
		})
	}))
	t.Cleanup(func() {
		release()
		_ = db.Callback().Query().Remove(callbackName)
	})

	reports := NewSupplierReportService(model.NewSupplierReportStore(db))
	reports.now = func() time.Time { return dayEnd.AddDate(0, 0, 1).Add(12 * time.Hour) }
	query := SupplierReportQuery{StartDate: day.Format("2006-01-02"), EndDate: day.Format("2006-01-02")}
	result := make(chan supplierReportSnapshotResult, 1)
	go func() {
		overview, reportErr := reports.GetOverview(ctx, query)
		result <- supplierReportSnapshotResult{overview: overview, err: reportErr}
	}()

	select {
	case <-evidenceRead:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "report did not reach the published-evidence barrier")
	}
	require.NoError(t, model.PublishSupplierDailyBatch(ctx, db, newLease, dayEnd.Add(2*time.Second), newEvidence))
	release()

	var duringPublish supplierReportSnapshotResult
	select {
	case duringPublish = <-result:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "report did not finish after publication barrier release")
	}
	require.NoError(t, duringPublish.err)
	require.Equal(t, int64(2), duringPublish.overview.PublishedEvidence.LogsScanned)
	require.Equal(t, int64(100), duringPublish.overview.Business.ProcurementCost.MicroUsd)

	afterPublish, err := reports.GetOverview(ctx, query)
	require.NoError(t, err)
	require.Equal(t, int64(1), afterPublish.PublishedEvidence.LogsScanned)
	require.Equal(t, int64(900), afterPublish.Business.ProcurementCost.MicroUsd)
}

func supplierReportSnapshotSummary(day time.Time, procurementMicroUsd int64) model.SupplierUsageDailySummary {
	return model.SupplierUsageDailySummary{
		DimensionKey: "business", BucketStart: day.Unix(), SupplierId: 1, ContractId: 2,
		RateVersionId: 3, ChannelId: 4, ModelName: "snapshot-model", PricingMode: "ratio",
		StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 1,
		OfficialListKnownCount: 1, OfficialListMicroUsd: procurementMicroUsd,
		SalesKnownCount: 1, SalesMicroUsd: procurementMicroUsd,
		ProcurementCostKnownCount: 1, ProcurementCostMicroUsd: procurementMicroUsd,
		GrossProfitKnownCount: 1, GrossMarginEligibleCount: 1, GrossMarginEligibleSalesMicroUsd: procurementMicroUsd,
	}
}
