package service

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newSupplierReportTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.UpstreamSupplier{}, &model.SupplierContract{}, &model.SupplierContractRateVersion{},
		&model.SupplierInventoryAdjustment{}, &model.Channel{},
		&model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{},
	))
	return db
}

func seedSupplierReportDay(t *testing.T, db *gorm.DB, date string, dayStart, fence int64) {
	t.Helper()
	publishedAt := dayStart + 86_400
	require.NoError(t, db.Create(&model.SupplierUsageDailyBatchRun{
		BatchDate: date, DayStart: dayStart, DayEnd: publishedAt,
		Status: model.SupplierDailyBatchStatusCompleted, FenceToken: fence,
		PublishedFenceToken: fence, PublishedAt: &publishedAt,
	}).Error)
}

func TestSupplierReportServiceComposedReadsStayInsideSnapshot(t *testing.T) {
	db := newSupplierReportTestDB(t)
	location, err := time.LoadLocation(SupplierReportTimezone)
	require.NoError(t, err)
	dayStart := time.Date(2026, 7, 20, 0, 0, 0, 0, location).Unix()

	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.UpstreamSupplier{Id: 1, Name: "supplier", Status: model.SupplierStatusActive}).Error)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.SupplierContract{Id: 2, SupplierId: 1, Name: "contract", ContractNo: "C-1", Status: model.SupplierContractStatusActive}).Error)
	require.NoError(t, db.Create(&model.SupplierContractRateVersion{Id: 3, ContractId: 2, ProcurementMultiplierPpm: 700_000, EffectiveAt: dayStart - 1, CreatedBy: 1}).Error)
	require.NoError(t, db.Create(&model.SupplierInventoryAdjustment{ContractId: 2, DeltaMicroUsd: 5_000, Type: model.SupplierInventoryAdjustmentTypeInitial, IdempotencyKey: "inventory", CreatedBy: 1, CreatedAt: dayStart}).Error)
	contractID := 2
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.Channel{Id: 4, Name: "channel", Status: 1, SupplierContractId: &contractID}).Error)
	seedSupplierReportDay(t, db, "2026-07-20", dayStart, 7)
	require.NoError(t, db.Create(&[]model.SupplierUsageDailySummary{
		{BatchDate: "2026-07-20", BatchFenceToken: 7, DimensionKey: "business", BucketStart: dayStart, SupplierId: 1, ContractId: 2, RateVersionId: 3, ChannelId: 4, ModelName: "gpt-test", PricingMode: "ratio", StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 1, OfficialListKnownCount: 1, OfficialListMicroUsd: 1_000},
		{BatchDate: "2026-07-20", BatchFenceToken: 7, DimensionKey: "internal", BucketStart: dayStart, SupplierId: 1, ContractId: 2, RateVersionId: 3, ChannelId: 4, StatisticsScope: "internal", DataQuality: "authoritative", RequestCount: 1, OfficialListKnownCount: 1, OfficialListMicroUsd: 1_000},
	}).Error)

	queryCount := 0
	escapedSnapshot := false
	checkTransaction := func(tx *gorm.DB) {
		queryCount++
		if _, ok := tx.Statement.ConnPool.(*sql.Tx); !ok {
			escapedSnapshot = true
		}
	}
	callbackName := fmt.Sprintf("test:supplier_report_snapshot:%s", t.Name())
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, checkTransaction))
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register(callbackName, checkTransaction))
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register(callbackName, checkTransaction))
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(callbackName)
		_ = db.Callback().Raw().Remove(callbackName)
		_ = db.Callback().Row().Remove(callbackName)
	})

	reports := NewSupplierReportService(model.NewSupplierReportStore(db))
	query := SupplierReportQuery{StartDate: "2026-07-20", EndDate: "2026-07-20"}
	page := model.SupplierReportPage{Limit: 10}
	operations := []struct {
		name string
		run  func() error
	}{
		{"overview", func() error { _, err := reports.GetOverview(context.Background(), query); return err }},
		{"trend", func() error { _, err := reports.GetTrend(context.Background(), query); return err }},
		{"contracts", func() error { _, err := reports.ListContracts(context.Background(), query, page); return err }},
		{"contract detail", func() error { _, err := reports.GetContractDetail(context.Background(), 2, query, page); return err }},
		{"channels", func() error { _, err := reports.ListChannels(context.Background(), query, page); return err }},
		{"breakdown", func() error { _, err := reports.ListBreakdown(context.Background(), query, page); return err }},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			queryCount = 0
			escapedSnapshot = false
			require.NoError(t, operation.run())
			require.GreaterOrEqual(t, queryCount, 2, "operation must exercise a composed read")
			require.False(t, escapedSnapshot, "every composed query must use the snapshot transaction")
		})
	}
}

func TestSupplierReportTrendDistinguishesPublishedZeroFromIncompleteDays(t *testing.T) {
	db := newSupplierReportTestDB(t)
	location, err := time.LoadLocation(SupplierReportTimezone)
	require.NoError(t, err)
	start := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	day := func(offset int) int64 { return start.AddDate(0, 0, offset).Unix() }

	seedSupplierReportDay(t, db, "2026-07-20", day(0), 1)
	require.NoError(t, db.Create(&model.SupplierUsageDailyBatchRun{BatchDate: "2026-07-21", DayStart: day(1), DayEnd: day(2), Status: model.SupplierDailyBatchStatusRunning, FenceToken: 2}).Error)
	require.NoError(t, db.Create(&model.SupplierUsageDailyBatchRun{BatchDate: "2026-07-22", DayStart: day(2), DayEnd: day(3), Status: model.SupplierDailyBatchStatusFailed, FenceToken: 3}).Error)
	seedSupplierReportDay(t, db, "2026-07-24", day(4), 4)
	require.NoError(t, db.Create(&model.SupplierUsageDailySummary{
		BatchDate: "2026-07-24", BatchFenceToken: 4, DimensionKey: "business", BucketStart: day(4),
		SupplierId: 1, ContractId: 2, ChannelId: 4, StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 9,
	}).Error)

	report, err := NewSupplierReportService(model.NewSupplierReportStore(db)).GetTrend(context.Background(), SupplierReportQuery{StartDate: "2026-07-20", EndDate: "2026-07-24"})
	require.NoError(t, err)
	require.Equal(t, []SupplierReportDayStatus{
		{Date: "2026-07-20", Status: "completed"},
		{Date: "2026-07-21", Status: "running"},
		{Date: "2026-07-22", Status: "failed"},
		{Date: "2026-07-23", Status: "missing"},
		{Date: "2026-07-24", Status: "completed"},
	}, report.DayStatuses)
	require.True(t, report.HasIncompleteDays)
	require.Equal(t, 3, report.IncompleteDayCount)
	require.NotNil(t, report.LatestCompletedDate)
	require.Equal(t, "2026-07-24", *report.LatestCompletedDate)
	require.Len(t, report.Points, 2, "only published days may expose metric points")
	require.Equal(t, "2026-07-20", report.Points[0].Date)
	require.Zero(t, report.Points[0].Business.RequestCount, "published-zero remains an explicit zero point")
	require.Equal(t, "2026-07-24", report.Points[1].Date)
	require.Equal(t, int64(9), report.Points[1].Business.RequestCount)
}
