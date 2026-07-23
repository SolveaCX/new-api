package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createSupplierReportCoverageGap(t *testing.T, db *gorm.DB, label string, startAt, endAt int64) model.SupplierAccountingCoverageGap {
	t.Helper()
	opened, err := model.OpenSupplierAccountingCoverageGap(db, model.OpenSupplierAccountingCoverageGapInput{
		StartAt: startAt, ReasonCategory: model.SupplierCoverageGapReasonOperatorDeclared,
		ReasonText: "report gap " + label, ExpectedCapabilityVersion: 1,
		ActivationStateVersionBefore: 3, ActivationStateVersionAfter: 4,
		OpenCommandID: "report-open-" + label, OpenedBy: 1, EvidenceRefs: []string{"incident:" + label},
	})
	require.NoError(t, err)
	closed, err := model.CloseSupplierAccountingCoverageGap(db, model.CloseSupplierAccountingCoverageGapInput{
		ID: opened.Id, EndAt: endAt, CloseCommandID: "report-close-" + label,
		ClosedBy: 1, FinanceDisposition: model.SupplierCoverageGapFinanceNoImpact,
		ExpectedVersion: opened.RecordVersion,
	})
	require.NoError(t, err)
	return *closed
}

func supplierReportGapIDs(gaps []model.SupplierAccountingCoverageGap) []int64 {
	ids := make([]int64, len(gaps))
	for index := range gaps {
		ids[index] = gaps[index].Id
	}
	return ids
}

func supplierReportPublishedRun(t *testing.T, batchDate string, dayStart, dayEnd int64, fence, captured int64) model.SupplierUsageDailyBatchRun {
	t.Helper()
	evidence := types.SupplierPublishedEvidenceV1{
		SchemaVersion: types.SupplierPublishedEvidenceSchemaVersion, LogsScanned: captured,
		ProducerMarkersPresent: captured, CapturedSnapshotCount: captured,
		DispositionCounts:                types.SupplierPublishedDispositionCountsV1{Captured: captured},
		PersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessComplete,
		Warnings:                         []types.SupplierPublishedWarningV1{},
	}
	encoded, err := types.EncodeSupplierPublishedEvidenceV1(evidence)
	require.NoError(t, err)
	publishedAt := dayEnd
	return model.SupplierUsageDailyBatchRun{
		BatchDate: batchDate, DayStart: dayStart, DayEnd: dayEnd, Status: model.SupplierDailyBatchStatusCompleted,
		FenceToken: fence, PublishedFenceToken: fence, PublishedAt: &publishedAt,
		PublishedPersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessComplete,
		PublishedEvidenceV1:                       encoded, CompletedAt: &publishedAt,
	}
}

func TestSupplierReportsReadDailyBatchSummariesAcrossPreservedSurfaces(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Option{},
		&model.UpstreamSupplier{}, &model.SupplierContract{}, &model.SupplierContractRateVersion{},
		&model.SupplierInventoryAdjustment{}, &model.Channel{}, &model.SupplierAccountingCoverageGap{}, &model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{},
	))
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.UpstreamSupplier{Id: 1, Name: "supplier", Status: model.SupplierStatusActive}).Error)
	rateID := 3
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.SupplierContract{Id: 2, SupplierId: 1, Name: "contract", ContractNo: "C-1", Status: model.SupplierContractStatusActive, CurrentRateVersionId: &rateID}).Error)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.SupplierContractRateVersion{Id: rateID, ContractId: 2, ProcurementMultiplierPpm: 700_000, EffectiveAt: 1, CreatedBy: 1}).Error)
	contractID := 2
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.Channel{Id: 4, Name: "channel", SupplierContractId: &contractID}).Error)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.SupplierInventoryAdjustment{ContractId: 2, DeltaMicroUsd: 5_000, Type: model.SupplierInventoryAdjustmentTypeInitial, IdempotencyKey: "inventory", CreatedBy: 1, CreatedAt: 1_784_476_800}).Error)

	location, err := time.LoadLocation(SupplierReportTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	rows := []model.SupplierUsageDailySummary{
		{BatchDate: "2026-07-20", BatchFenceToken: 7, DimensionKey: "business", BucketStart: day.Unix(), SupplierId: 1, ContractId: 2, RateVersionId: 3, ChannelId: 4, ModelName: "gpt-test", PricingMode: "ratio", StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 1, OfficialListKnownCount: 1, OfficialListMicroUsd: 1_000, SalesKnownCount: 1, SalesMicroUsd: 2_000, ProcurementCostKnownCount: 1, ProcurementCostMicroUsd: 700, GrossProfitKnownCount: 1, GrossProfitMicroUsd: 1_300, GrossMarginEligibleCount: 1, GrossMarginEligibleSalesMicroUsd: 2_000},
		{BatchDate: "2026-07-20", BatchFenceToken: 7, DimensionKey: "internal", BucketStart: day.Unix(), SupplierId: 1, ContractId: 2, RateVersionId: 3, ChannelId: 4, StatisticsScope: "internal", DataQuality: "authoritative", RequestCount: 1, OfficialListKnownCount: 1, OfficialListMicroUsd: 1_000, ProcurementCostKnownCount: 1, ProcurementCostMicroUsd: 700},
	}
	require.NoError(t, db.Create(&rows).Error)
	completed := day.AddDate(0, 0, 1).Unix()
	activateSupplierAccountingForBatch(t, db, day.Unix())
	require.NoError(t, db.Create(&[]model.SupplierUsageDailyBatchRun{supplierReportPublishedRun(t, "2026-07-20", day.Unix(), completed, 7, 2)}).Error)
	require.NoError(t, db.Model(&model.SupplierUsageDailyBatchRun{}).Where("batch_date = ?", "2026-07-20").Updates(map[string]any{
		"fence_token": 8, "status": model.SupplierDailyBatchStatusFailed,
		"logs_scanned": 999, "snapshot_count": 998, "error_message": "mutable candidate failure",
	}).Error)
	candidate := rows[0]
	candidate.Id = 0
	candidate.BatchFenceToken = 8
	candidate.DimensionKey = "unpublished-candidate"
	candidate.RequestCount = 900
	candidate.ProcurementCostKnownCount = 900
	candidate.ProcurementCostMicroUsd = 900_000
	require.NoError(t, db.Create(&candidate).Error)
	crossDay := createSupplierReportCoverageGap(t, db, "cross-day", day.Add(-time.Hour).Unix(), day.Add(time.Hour).Unix())
	firstSameDay := createSupplierReportCoverageGap(t, db, "same-day-a", day.Add(2*time.Hour).Unix(), day.Add(3*time.Hour).Unix())
	secondSameDay := createSupplierReportCoverageGap(t, db, "same-day-b", day.Add(5*time.Hour).Unix(), day.Add(6*time.Hour).Unix())
	expectedGapIDs := []int64{crossDay.Id, firstSameDay.Id, secondSameDay.Id}
	gapQueryCount := 0
	callbackName := fmt.Sprintf("test:count_supplier_report_gap_queries:%s", t.Name())
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "supplier_accounting_coverage_gaps" {
			gapQueryCount++
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callbackName) })

	reports := NewSupplierReportService(model.NewSupplierReportStore(db))
	query := SupplierReportQuery{StartDate: "2026-07-20", EndDate: "2026-07-20"}
	beforeGapQueries := gapQueryCount
	overview, err := reports.GetOverview(context.Background(), query)
	require.NoError(t, err)
	require.Equal(t, beforeGapQueries+1, gapQueryCount)
	require.Equal(t, expectedGapIDs, supplierReportGapIDs(overview.KnownCoverageGaps))
	require.Equal(t, int64(1), overview.Business.RequestCount)
	require.Equal(t, int64(1), overview.Internal.RequestCount)
	require.Equal(t, int64(1_300), overview.Business.GrossProfit.MicroUsd)
	require.Zero(t, overview.Internal.GrossProfit.MicroUsd)
	require.Equal(t, int64(1_400), overview.TotalProcurementCost.MicroUsd)
	require.Equal(t, int64(3_000), overview.RemainingInventoryMicroUsd)
	require.Equal(t, int64(2), overview.PublishedEvidence.LogsScanned)
	require.Equal(t, int64(2), overview.PublishedEvidence.CapturedSnapshotCount)
	require.Equal(t, types.SupplierPersistedLogCompletenessComplete, overview.PublishedEvidence.PersistedLogSnapshotCompleteness)
	require.Empty(t, overview.PublishedEvidence.Warnings)
	require.True(t, overview.FinanceAttentionRequired, "typed coverage gaps require finance attention even when published evidence is complete")
	beforeGapQueries = gapQueryCount
	freshness, err := reports.GetFreshness(context.Background())
	require.NoError(t, err)
	require.Equal(t, beforeGapQueries+1, gapQueryCount)
	require.Equal(t, expectedGapIDs, supplierReportGapIDs(freshness.KnownCoverageGaps))
	require.Equal(t, "2026-07-20", freshness.LatestBatchDate)
	require.Equal(t, model.SupplierDailyBatchStatusCompleted, freshness.BatchStatus, "mutable candidate status must not replace published state")
	require.True(t, freshness.SyncOnly)
	require.Equal(t, day.Unix(), freshness.CoverageStartAt)
	require.Equal(t, int64(2), freshness.PublishedEvidence.LogsScanned, "freshness counters must come from immutable published evidence")
	require.True(t, freshness.FinanceAttentionRequired)

	beforeGapQueries = gapQueryCount
	trend, err := reports.GetTrend(context.Background(), query)
	require.NoError(t, err)
	require.Equal(t, beforeGapQueries+1, gapQueryCount)
	require.Equal(t, expectedGapIDs, supplierReportGapIDs(trend.KnownCoverageGaps))
	require.Len(t, trend.Points, 1)
	beforeGapQueries = gapQueryCount
	contracts, err := reports.ListContracts(context.Background(), query, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, beforeGapQueries+1, gapQueryCount)
	require.Equal(t, expectedGapIDs, supplierReportGapIDs(contracts.KnownCoverageGaps))
	require.Len(t, contracts.Items, 1)
	beforeGapQueries = gapQueryCount
	channels, err := reports.ListChannels(context.Background(), query, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, beforeGapQueries+1, gapQueryCount)
	require.Equal(t, expectedGapIDs, supplierReportGapIDs(channels.KnownCoverageGaps))
	require.Len(t, channels.Items, 1)
	beforeGapQueries = gapQueryCount
	breakdown, err := reports.ListBreakdown(context.Background(), query, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, beforeGapQueries+1, gapQueryCount)
	require.Equal(t, expectedGapIDs, supplierReportGapIDs(breakdown.KnownCoverageGaps))
	require.Len(t, breakdown.Items, 1)
	beforeGapQueries = gapQueryCount
	detail, err := reports.GetContractDetail(context.Background(), 2, query, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, beforeGapQueries+1, gapQueryCount, "composed detail response must load the requested gap range once")
	require.Equal(t, expectedGapIDs, supplierReportGapIDs(detail.KnownCoverageGaps))
	require.Equal(t, 2, detail.Summary.ContractId)

	require.NoError(t, db.Where("1 = 1").Delete(&model.SupplierAccountingCoverageGap{}).Error)
	withoutGaps, err := reports.GetOverview(context.Background(), query)
	require.NoError(t, err)
	require.Empty(t, withoutGaps.KnownCoverageGaps, "coverage gaps must be queried independently for each report projection")
	require.Equal(t, int64(1_400), withoutGaps.TotalProcurementCost.MicroUsd, "fresh gap reads must not change published-fence amounts")
	require.Equal(t, types.SupplierPersistedLogCompletenessComplete, withoutGaps.PublishedEvidence.PersistedLogSnapshotCompleteness)
	require.False(t, withoutGaps.FinanceAttentionRequired)

	incompleteEvidence := types.SupplierPublishedEvidenceV1{
		SchemaVersion: types.SupplierPublishedEvidenceSchemaVersion, LogsScanned: 2,
		ProducerMarkersPresent: 1, CapturedSnapshotCount: 1,
		DispositionCounts:                types.SupplierPublishedDispositionCountsV1{Captured: 1},
		FailureCounts:                    types.SupplierPublishedFailureCountsV1{AbsentMarkerAfterCutover: 1},
		PersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessIncomplete,
		Warnings: []types.SupplierPublishedWarningV1{{
			Code: types.SupplierPublishedWarningAbsentMarker, MessageKey: "supply_chain.warning.absent_marker_after_cutover", Count: 1,
		}},
	}
	encodedIncomplete, err := types.EncodeSupplierPublishedEvidenceV1(incompleteEvidence)
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.SupplierUsageDailyBatchRun{}).Where("batch_date = ?", "2026-07-20").Updates(map[string]any{
		"published_evidence_v1":                         encodedIncomplete,
		"published_persisted_log_snapshot_completeness": types.SupplierPersistedLogCompletenessIncomplete,
	}).Error)
	incomplete, err := reports.GetOverview(context.Background(), query)
	require.NoError(t, err)
	require.Empty(t, incomplete.KnownCoverageGaps)
	require.Equal(t, int64(1_400), incomplete.TotalProcurementCost.MicroUsd)
	require.Equal(t, types.SupplierPersistedLogCompletenessIncomplete, incomplete.PublishedEvidence.PersistedLogSnapshotCompleteness)
	require.Equal(t, int64(1), incomplete.PublishedEvidence.FailureCounts.AbsentMarkerAfterCutover)
	require.Len(t, incomplete.PublishedEvidence.Warnings, 1)
	require.True(t, incomplete.FinanceAttentionRequired, "incomplete immutable evidence requires finance attention without coverage gaps")
}

func TestSupplierReportHistoricalChannelOwnershipSurvivesRebindAndUnbind(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.UpstreamSupplier{}, &model.SupplierContract{}, &model.SupplierContractRateVersion{},
		&model.SupplierInventoryAdjustment{}, &model.Channel{}, &model.SupplierAccountingCoverageGap{}, &model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{},
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
		BatchDate: "2026-07-20", BatchFenceToken: 7, DimensionKey: "historical-business", BucketStart: day.Unix(),
		SupplierId: 1, ContractId: 10, ChannelId: 30, ModelName: "gpt-test",
		StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 2,
		OfficialListKnownCount: 2, OfficialListMicroUsd: 2_000,
	}).Error)
	completed := day.AddDate(0, 0, 1).Unix()
	require.NoError(t, db.Create(&[]model.SupplierUsageDailyBatchRun{supplierReportPublishedRun(t, "2026-07-20", day.Unix(), completed, 7, 2)}).Error)

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
