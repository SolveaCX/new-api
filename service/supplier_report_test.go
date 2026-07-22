package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSupplierReportsReadDailyBatchSummariesAcrossPreservedSurfaces(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Option{},
		&model.UpstreamSupplier{}, &model.SupplierContract{}, &model.SupplierContractRateVersion{},
		&model.SupplierInventoryAdjustment{}, &model.Channel{}, &model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{},
	))
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.UpstreamSupplier{Id: 1, Name: "supplier", Status: model.SupplierStatusActive}).Error)
	rateID := 3
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.SupplierContract{Id: 2, SupplierId: 1, Name: "contract", ContractNo: "C-1", Status: model.SupplierContractStatusActive, CurrentRateVersionId: &rateID}).Error)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.SupplierContractRateVersion{Id: rateID, ContractId: 2, ProcurementMultiplierPpm: 700_000, EffectiveAt: 1, CreatedBy: 1}).Error)
	contractID := 2
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.Channel{Id: 4, Name: "channel", SupplierContractId: &contractID}).Error)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.SupplierInventoryAdjustment{ContractId: 2, DeltaMicroUsd: 5_000, Type: model.SupplierInventoryAdjustmentTypeInitial, IdempotencyKey: "inventory", CreatedBy: 1}).Error)

	location, err := time.LoadLocation(SupplierReportTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	rows := []model.SupplierUsageDailySummary{
		{BatchDate: "2026-07-20", DimensionKey: "business", BucketStart: day.Unix(), SupplierId: 1, ContractId: 2, RateVersionId: 3, ChannelId: 4, ModelName: "gpt-test", PricingMode: "ratio", StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 1, OfficialListKnownCount: 1, OfficialListMicroUsd: 1_000, SalesKnownCount: 1, SalesMicroUsd: 2_000, ProcurementCostKnownCount: 1, ProcurementCostMicroUsd: 700, GrossProfitKnownCount: 1, GrossProfitMicroUsd: 1_300, GrossMarginEligibleCount: 1, GrossMarginEligibleSalesMicroUsd: 2_000},
		{BatchDate: "2026-07-20", DimensionKey: "internal", BucketStart: day.Unix(), SupplierId: 1, ContractId: 2, RateVersionId: 3, ChannelId: 4, StatisticsScope: "internal", DataQuality: "authoritative", RequestCount: 1, OfficialListKnownCount: 1, OfficialListMicroUsd: 1_000, ProcurementCostKnownCount: 1, ProcurementCostMicroUsd: 700},
	}
	require.NoError(t, db.Create(&rows).Error)
	completed := day.AddDate(0, 0, 1).Unix()
	_, err = model.GetOrCreateSupplierAccountingCoverageStart(context.Background(), db, day.Unix())
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.SupplierUsageDailyBatchRun{BatchDate: "2026-07-20", DayStart: day.Unix(), DayEnd: completed, Status: model.SupplierDailyBatchStatusCompleted, CompletedAt: &completed}).Error)

	reports := NewSupplierReportService(model.NewSupplierReportStore(db))
	query := SupplierReportQuery{StartDate: "2026-07-20", EndDate: "2026-07-20"}
	overview, err := reports.GetOverview(context.Background(), query)
	require.NoError(t, err)
	require.Equal(t, int64(1), overview.Business.RequestCount)
	require.Equal(t, int64(1), overview.Internal.RequestCount)
	require.Equal(t, int64(1_300), overview.Business.GrossProfit.MicroUsd)
	require.Zero(t, overview.Internal.GrossProfit.MicroUsd)
	require.Equal(t, int64(1_400), overview.TotalProcurementCost.MicroUsd)
	require.Equal(t, int64(3_000), overview.RemainingInventoryMicroUsd)
	freshness, err := reports.GetFreshness(context.Background())
	require.NoError(t, err)
	require.Equal(t, "2026-07-20", freshness.LatestBatchDate)
	require.True(t, freshness.SyncOnly)
	require.Equal(t, day.Unix(), freshness.CoverageStartAt)

	trend, err := reports.GetTrend(context.Background(), query)
	require.NoError(t, err)
	require.Len(t, trend.Points, 1)
	contracts, err := reports.ListContracts(context.Background(), query, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Len(t, contracts.Items, 1)
	channels, err := reports.ListChannels(context.Background(), query, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Len(t, channels.Items, 1)
	breakdown, err := reports.ListBreakdown(context.Background(), query, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Len(t, breakdown.Items, 1)
	detail, err := reports.GetContractDetail(context.Background(), 2, query, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 2, detail.Summary.ContractId)
}

func TestSupplierReportHistoricalChannelOwnershipSurvivesRebindAndUnbind(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.UpstreamSupplier{}, &model.SupplierContract{}, &model.SupplierContractRateVersion{},
		&model.SupplierInventoryAdjustment{}, &model.Channel{}, &model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{},
	))
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.UpstreamSupplier{Id: 1, Name: "supplier", Status: model.SupplierStatusActive}).Error)
	contracts := []model.SupplierContract{
		{Id: 10, SupplierId: 1, Name: "historical contract", ContractNo: "C-OLD", Status: model.SupplierContractStatusActive},
		{Id: 20, SupplierId: 1, Name: "current contract", ContractNo: "C-NEW", Status: model.SupplierContractStatusActive},
	}
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&contracts).Error)
	currentContractID := 20
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.Channel{Id: 30, Name: "rebound channel", Status: 1, SupplierContractId: &currentContractID}).Error)

	location, err := time.LoadLocation(SupplierReportTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	require.NoError(t, db.Create(&model.SupplierUsageDailySummary{
		BatchDate: "2026-07-20", DimensionKey: "historical-business", BucketStart: day.Unix(),
		SupplierId: 1, ContractId: 10, ChannelId: 30, ModelName: "gpt-test",
		StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 2,
		OfficialListKnownCount: 2, OfficialListMicroUsd: 2_000,
	}).Error)
	completed := day.AddDate(0, 0, 1).Unix()
	require.NoError(t, db.Create(&model.SupplierUsageDailyBatchRun{
		BatchDate: "2026-07-20", DayStart: day.Unix(), DayEnd: completed,
		Status: model.SupplierDailyBatchStatusCompleted, CompletedAt: &completed,
	}).Error)

	reports := NewSupplierReportService(model.NewSupplierReportStore(db))
	query := SupplierReportQuery{StartDate: "2026-07-20", EndDate: "2026-07-20"}
	filteredQuery := query
	filteredQuery.ChannelIds = []int{30}

	contractsResult, err := reports.ListContracts(context.Background(), filteredQuery, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Len(t, contractsResult.Items, 1)
	require.Equal(t, 10, contractsResult.Items[0].ContractId, "channel filtering must use the historical daily-summary ownership")

	channels, err := reports.ListChannels(context.Background(), query, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Len(t, channels.Items, 2, "historical ownership and the no-usage current catalog entry must both remain visible")
	require.Equal(t, 10, channels.Items[0].ContractId)
	require.Equal(t, int64(2), channels.Items[0].Business.RequestCount)
	require.Equal(t, 20, channels.Items[1].ContractId)
	require.Zero(t, channels.Items[1].Business.RequestCount)

	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Model(&model.Channel{}).Where("id = ?", 30).UpdateColumn("supplier_contract_id", nil).Error)
	channels, err = reports.ListChannels(context.Background(), query, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Len(t, channels.Items, 1, "unbinding current state must not remove historical ownership")
	require.Equal(t, 10, channels.Items[0].ContractId)
	require.Equal(t, 30, channels.Items[0].ChannelId)
	require.Equal(t, "rebound channel", channels.Items[0].ChannelName)
	require.Equal(t, int64(2), channels.Items[0].Business.RequestCount)

	contractsResult, err = reports.ListContracts(context.Background(), filteredQuery, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Len(t, contractsResult.Items, 1)
	require.Equal(t, 10, contractsResult.Items[0].ContractId)
}

func TestBuildContractRowZeroRemainingInventoryIsNotOversold(t *testing.T) {
	row, err := buildContractRow(
		model.SupplierReportContractCatalogRow{ContractId: 1},
		contractRuntime{inventory: 1_000, consumed: 1_000},
		nil,
	)
	require.NoError(t, err)
	require.Zero(t, row.RemainingInventoryMicroUsd)
	require.False(t, row.Oversold)

	row, err = buildContractRow(
		model.SupplierReportContractCatalogRow{ContractId: 1},
		contractRuntime{inventory: 1_000, consumed: 1_001},
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(-1), row.RemainingInventoryMicroUsd)
	require.True(t, row.Oversold)
}
