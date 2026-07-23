package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func supplierReportCompletePublishedBatches(count int) []model.SupplierPublishedDailyBatch {
	batches := make([]model.SupplierPublishedDailyBatch, count)
	for index := range batches {
		batches[index].Evidence = types.SupplierPublishedEvidenceV1{
			PersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessComplete,
			Warnings:                         []types.SupplierPublishedWarningV1{},
		}
	}
	return batches
}

func TestSupplierReportPublishedEvidenceUsesClosedShanghaiDays(t *testing.T) {
	location, err := time.LoadLocation(SupplierReportTimezone)
	require.NoError(t, err)
	monthRange, err := ParseSupplierReportRange("2026-07", "", "")
	require.NoError(t, err)

	t.Run("current month excludes today and future days", func(t *testing.T) {
		now := time.Date(2026, 7, 15, 3, 0, 0, 0, location)
		evidence, err := aggregateSupplierReportPublishedEvidenceAt(monthRange, supplierReportCompletePublishedBatches(14), now)
		require.NoError(t, err)
		require.Equal(t, 14, evidence.ExpectedDays)
		require.Equal(t, 14, evidence.PublishedDays)
		require.Equal(t, types.SupplierPersistedLogCompletenessComplete, evidence.PersistedLogSnapshotCompleteness)
		require.False(t, supplierReportNeedsFinanceAttention(evidence, nil))
	})

	t.Run("close grace boundary", func(t *testing.T) {
		beforeGrace := time.Date(2026, 7, 15, 1, 59, 59, 0, location)
		before, err := aggregateSupplierReportPublishedEvidenceAt(monthRange, supplierReportCompletePublishedBatches(13), beforeGrace)
		require.NoError(t, err)
		require.Equal(t, 13, before.ExpectedDays)
		require.Equal(t, types.SupplierPersistedLogCompletenessComplete, before.PersistedLogSnapshotCompleteness)

		atGrace := time.Date(2026, 7, 15, 2, 0, 0, 0, location)
		after, err := aggregateSupplierReportPublishedEvidenceAt(monthRange, supplierReportCompletePublishedBatches(14), atGrace)
		require.NoError(t, err)
		require.Equal(t, 14, after.ExpectedDays)
		require.Equal(t, types.SupplierPersistedLogCompletenessComplete, after.PersistedLogSnapshotCompleteness)
	})

	t.Run("month start before grace has no eligible day", func(t *testing.T) {
		augustRange, err := ParseSupplierReportRange("2026-08", "", "")
		require.NoError(t, err)
		now := time.Date(2026, 8, 1, 1, 0, 0, 0, location)
		evidence, err := aggregateSupplierReportPublishedEvidenceAt(augustRange, nil, now)
		require.NoError(t, err)
		require.Zero(t, evidence.ExpectedDays)
		require.Zero(t, evidence.PublishedDays)
		require.Equal(t, types.SupplierPersistedLogCompletenessNotScanned, evidence.PersistedLogSnapshotCompleteness)
		require.False(t, supplierReportNeedsFinanceAttention(evidence, nil))
	})

	t.Run("past complete month is unchanged", func(t *testing.T) {
		juneRange, err := ParseSupplierReportRange("2026-06", "", "")
		require.NoError(t, err)
		now := time.Date(2026, 7, 15, 3, 0, 0, 0, location)
		evidence, err := aggregateSupplierReportPublishedEvidenceAt(juneRange, supplierReportCompletePublishedBatches(30), now)
		require.NoError(t, err)
		require.Equal(t, 30, evidence.ExpectedDays)
		require.Equal(t, types.SupplierPersistedLogCompletenessComplete, evidence.PersistedLogSnapshotCompleteness)
		require.False(t, supplierReportNeedsFinanceAttention(evidence, nil))
	})

	t.Run("missing closed day still requires attention", func(t *testing.T) {
		now := time.Date(2026, 7, 15, 3, 0, 0, 0, location)
		evidence, err := aggregateSupplierReportPublishedEvidenceAt(monthRange, supplierReportCompletePublishedBatches(13), now)
		require.NoError(t, err)
		require.Equal(t, 14, evidence.ExpectedDays)
		require.Equal(t, types.SupplierPersistedLogCompletenessIncomplete, evidence.PersistedLogSnapshotCompleteness)
		require.True(t, supplierReportNeedsFinanceAttention(evidence, nil))
	})

	t.Run("unexpected publication with no eligible day requires attention", func(t *testing.T) {
		futureRange, err := ParseSupplierReportRange("2026-08", "", "")
		require.NoError(t, err)
		now := time.Date(2026, 7, 15, 3, 0, 0, 0, location)
		evidence, err := aggregateSupplierReportPublishedEvidenceAt(futureRange, supplierReportCompletePublishedBatches(1), now)
		require.NoError(t, err)
		require.Zero(t, evidence.ExpectedDays)
		require.Equal(t, 1, evidence.PublishedDays)
		require.Equal(t, types.SupplierPersistedLogCompletenessIncomplete, evidence.PersistedLogSnapshotCompleteness)
		require.True(t, supplierReportNeedsFinanceAttention(evidence, nil))
	})
}

func TestSupplierReportFreshnessOnlyReadsEligiblePublishedDays(t *testing.T) {
	mainDB, _ := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierReportTimezone)
	require.NoError(t, err)
	monthStart := time.Date(2026, 7, 1, 0, 0, 0, 0, location)
	activateSupplierAccountingForBatch(t, mainDB, monthStart.Unix())

	runs := make([]model.SupplierUsageDailyBatchRun, 0, 15)
	for day := 1; day <= 15; day++ {
		start := time.Date(2026, 7, day, 0, 0, 0, 0, location)
		runs = append(runs, supplierReportPublishedRun(t, start.Format("2006-01-02"), start.Unix(), start.AddDate(0, 0, 1).Unix(), int64(day), 1))
	}
	require.NoError(t, mainDB.Create(&runs).Error)

	now := time.Date(2026, 7, 15, 3, 0, 0, 0, location)
	freshness, err := NewSupplierReportService(model.NewSupplierReportStore(mainDB)).getFreshnessAt(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, 14, freshness.PublishedEvidence.ExpectedDays)
	require.Equal(t, 14, freshness.PublishedEvidence.PublishedDays)
	require.Equal(t, "2026-07-14", freshness.LatestBatchDate)
	require.Equal(t, types.SupplierPersistedLogCompletenessComplete, freshness.PublishedEvidence.PersistedLogSnapshotCompleteness)
	require.False(t, freshness.FinanceAttentionRequired)
	require.NotNil(t, freshness.FreshThrough)
	require.Equal(t, time.Date(2026, 7, 15, 0, 0, 0, 0, location).Unix(), *freshness.FreshThrough)
	require.NotNil(t, freshness.FreshnessLagSeconds)
	require.Equal(t, int64(3*time.Hour/time.Second), *freshness.FreshnessLagSeconds)
}

func TestSupplierReportSurfacesUseOneEligibleClosedDayRange(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Option{}, &model.UpstreamSupplier{}, &model.SupplierContract{}, &model.SupplierContractRateVersion{},
		&model.SupplierInventoryAdjustment{}, &model.Channel{}, &model.SupplierAccountingCoverageGap{},
		&model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{},
	))
	location, err := time.LoadLocation(SupplierReportTimezone)
	require.NoError(t, err)
	closedDay := time.Date(2026, 7, 14, 0, 0, 0, 0, location)
	openDay := closedDay.AddDate(0, 0, 1)
	now := openDay.Add(3 * time.Hour)

	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.UpstreamSupplier{Id: 1, Name: "supplier", Status: model.SupplierStatusActive}).Error)
	futureRateID := 12
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.SupplierContract{
		Id: 2, SupplierId: 1, Name: "contract", ContractNo: "C-closed", Status: model.SupplierContractStatusActive, CurrentRateVersionId: &futureRateID,
	}).Error)
	rates := []model.SupplierContractRateVersion{
		{Id: 11, ContractId: 2, ProcurementMultiplierPpm: 700_000, EffectiveAt: closedDay.Add(time.Hour).Unix(), CreatedBy: 1, CreatedAt: closedDay.Add(time.Hour).Unix()},
		{Id: futureRateID, ContractId: 2, ProcurementMultiplierPpm: 900_000, EffectiveAt: openDay.Add(time.Hour).Unix(), CreatedBy: 1, CreatedAt: openDay.Add(time.Hour).Unix()},
	}
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&rates).Error)
	contractID := 2
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&[]model.Channel{
		{Id: 4, Name: "closed-channel", SupplierContractId: &contractID},
		{Id: 5, Name: "future-history-only"},
	}).Error)
	adjustments := []model.SupplierInventoryAdjustment{
		{ContractId: 2, DeltaMicroUsd: 5_000, Type: model.SupplierInventoryAdjustmentTypeInitial, IdempotencyKey: "closed-inventory", CreatedBy: 1, CreatedAt: closedDay.Add(time.Hour).Unix()},
		{ContractId: 2, DeltaMicroUsd: 9_000, Type: model.SupplierInventoryAdjustmentTypeReplenishment, IdempotencyKey: "open-day-inventory", CreatedBy: 1, CreatedAt: openDay.Add(time.Hour).Unix()},
	}
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&adjustments).Error)

	summaries := []model.SupplierUsageDailySummary{
		{BatchDate: "2026-07-14", BatchFenceToken: 14, DimensionKey: "closed-business", BucketStart: closedDay.Unix(), SupplierId: 1, ContractId: 2, RateVersionId: 11, ChannelId: 4, ModelName: "closed-model", PricingMode: "ratio", StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 1, OfficialListKnownCount: 1, OfficialListMicroUsd: 1_000, SalesKnownCount: 1, SalesMicroUsd: 2_000, ProcurementCostKnownCount: 1, ProcurementCostMicroUsd: 700, GrossProfitKnownCount: 1, GrossProfitMicroUsd: 1_300, GrossMarginEligibleCount: 1, GrossMarginEligibleSalesMicroUsd: 2_000},
		{BatchDate: "2026-07-14", BatchFenceToken: 14, DimensionKey: "closed-internal", BucketStart: closedDay.Unix(), SupplierId: 1, ContractId: 2, RateVersionId: 11, ChannelId: 4, StatisticsScope: "internal", DataQuality: "authoritative", RequestCount: 1, OfficialListKnownCount: 1, OfficialListMicroUsd: 1_000, ProcurementCostKnownCount: 1, ProcurementCostMicroUsd: 700},
		{BatchDate: "2026-07-15", BatchFenceToken: 15, DimensionKey: "open-business", BucketStart: openDay.Unix(), SupplierId: 1, ContractId: 2, RateVersionId: futureRateID, ChannelId: 5, ModelName: "future-model", PricingMode: "ratio", StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 90, OfficialListKnownCount: 90, OfficialListMicroUsd: 90_000, SalesKnownCount: 90, SalesMicroUsd: 180_000, ProcurementCostKnownCount: 90, ProcurementCostMicroUsd: 81_000, GrossProfitKnownCount: 90, GrossProfitMicroUsd: 99_000, GrossMarginEligibleCount: 90, GrossMarginEligibleSalesMicroUsd: 180_000},
		{BatchDate: "2026-07-15", BatchFenceToken: 15, DimensionKey: "open-internal", BucketStart: openDay.Unix(), SupplierId: 1, ContractId: 2, RateVersionId: futureRateID, ChannelId: 5, StatisticsScope: "internal", DataQuality: "authoritative", RequestCount: 90, OfficialListKnownCount: 90, OfficialListMicroUsd: 90_000, ProcurementCostKnownCount: 90, ProcurementCostMicroUsd: 81_000},
	}
	require.NoError(t, db.Create(&summaries).Error)
	require.NoError(t, db.Create(&[]model.SupplierUsageDailyBatchRun{
		supplierReportPublishedRun(t, "2026-07-14", closedDay.Unix(), openDay.Unix(), 14, 2),
		supplierReportPublishedRun(t, "2026-07-15", openDay.Unix(), openDay.AddDate(0, 0, 1).Unix(), 15, 2),
	}).Error)
	activateSupplierAccountingForBatch(t, db, closedDay.Unix())
	_ = createSupplierReportCoverageGap(t, db, "open-day-only", openDay.Add(30*time.Minute).Unix(), openDay.Add(time.Hour).Unix())

	reports := NewSupplierReportService(model.NewSupplierReportStore(db))
	reports.now = func() time.Time { return now }
	query := SupplierReportQuery{StartDate: "2026-07-14", EndDate: "2026-07-15"}

	overview, err := reports.GetOverview(context.Background(), query)
	require.NoError(t, err)
	require.Equal(t, openDay.AddDate(0, 0, 1).Unix(), overview.Range.EndAt, "response keeps the requested inclusive end day")
	require.Equal(t, int64(1), overview.Business.RequestCount)
	require.Equal(t, int64(1_400), overview.TotalProcurementCost.MicroUsd)
	require.Equal(t, int64(5_000), overview.TotalInventoryMicroUsd)
	require.Equal(t, int64(2_000), overview.OfficialListConsumedMicroUsd)
	require.Equal(t, int64(3_000), overview.RemainingInventoryMicroUsd)
	require.Empty(t, overview.KnownCoverageGaps)
	require.False(t, overview.FinanceAttentionRequired)
	require.Equal(t, 1, overview.PublishedEvidence.ExpectedDays)
	require.Equal(t, 1, overview.PublishedEvidence.PublishedDays)

	trend, err := reports.GetTrend(context.Background(), query)
	require.NoError(t, err)
	require.Len(t, trend.Points, 2)
	require.Equal(t, int64(1), trend.Points[0].Business.RequestCount)
	require.Zero(t, trend.Points[1].Business.RequestCount)

	contracts, err := reports.ListContracts(context.Background(), query, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Len(t, contracts.Items, 1)
	require.Equal(t, 11, *contracts.Items[0].CurrentRateVersionId)
	require.Equal(t, int64(2_000), contracts.Items[0].OfficialListConsumedMicroUsd)

	channels, err := reports.ListChannels(context.Background(), query, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Len(t, channels.Items, 1)
	require.Equal(t, 4, channels.Items[0].ChannelId)

	breakdown, err := reports.ListBreakdown(context.Background(), query, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Len(t, breakdown.Items, 1)
	require.Equal(t, "closed-model", breakdown.Items[0].ModelName)
	require.Equal(t, int64(1), breakdown.TotalBusinessCount)

	detail, err := reports.GetContractDetail(context.Background(), 2, query, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Len(t, detail.RateVersions, 1)
	require.Equal(t, 11, detail.RateVersions[0].Id)
	require.Len(t, detail.InventoryAdjustments, 1)
	require.Equal(t, "closed-inventory", detail.InventoryAdjustments[0].IdempotencyKey)
	require.Len(t, detail.Channels.Items, 1)
	require.Len(t, detail.Breakdown.Items, 1)
	require.Len(t, detail.InternalTrend, 2)
	require.Zero(t, detail.InternalTrend[1].Internal.RequestCount)
}
