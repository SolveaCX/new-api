package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func supplierDailyTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	return db
}

func supplierDailyTestDBs(t *testing.T) (*gorm.DB, *gorm.DB) {
	t.Helper()
	mainDB := supplierDailyTestDB(t, t.Name()+"-main")
	logDB := supplierDailyTestDB(t, t.Name()+"-log")
	require.NoError(t, mainDB.AutoMigrate(&model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{}))
	require.NoError(t, logDB.AutoMigrate(&model.Log{}))
	return mainDB, logDB
}

func supplierDailyLogOther(t *testing.T, snapshot types.SupplierAccountingLogSnapshotV1) string {
	t.Helper()
	payload, err := common.Marshal(map[string]any{types.SupplierAccountingEnvelopeKeyV1: types.SupplierAccountingEnvelopeV1{
		EnvelopeSchemaVersion: types.SupplierAccountingEnvelopeSchemaVersionV1,
		Disposition:           types.SupplierAccountingDispositionCaptured, Captured: &snapshot,
	}})
	require.NoError(t, err)
	return string(payload)
}

func supplierDailySnapshot(day time.Time, multiplier int64) types.SupplierAccountingLogSnapshotV1 {
	official, sales, procurement, gross := int64(1_000), int64(2_000), int64(700), int64(1_300)
	return types.SupplierAccountingLogSnapshotV1{
		BindingVersionId: 8, SupplierId: 1, ContractId: 2, RateVersionId: 3,
		ProcurementMultiplierPpm: 700_000, SalesMultiplierPpm: &multiplier,
		OfficialListMicroUsd: &official, SalesMicroUsd: &sales,
		ProcurementCostMicroUsd: &procurement, GrossProfitMicroUsd: &gross,
		StatisticsScope: string(types.SupplierStatisticsScopeBusiness), ExclusionDecision: "included",
		FinanciallyCommittedAt: day.Add(time.Hour).Unix(),
		PricingProvenance: &types.SupplierPricingProvenanceV1{Ratio: &types.SupplierRatioPricingProvenanceV1{
			ModelRatioPpm: 1_000_000, GroupRatioPpm: multiplier, ModelRatioVersion: 1, GroupRatioVersion: 1,
		}},
	}
}

func TestRunSupplierDailyBatchAggregatesCapturedSnapshot(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	require.NoError(t, logDB.Create(&model.Log{
		Type: model.LogTypeConsume, CreatedAt: day.Add(time.Hour).Unix(), ChannelId: 4, ModelName: "gpt-test",
		Other: supplierDailyLogOther(t, supplierDailySnapshot(day, 1_500_000)),
	}).Error)

	require.NoError(t, RunSupplierDailyBatch(context.Background(), mainDB, logDB, day.Format("2006-01-02"), "console", day.AddDate(0, 0, 2)))
	var summary model.SupplierUsageDailySummary
	require.NoError(t, mainDB.First(&summary).Error)
	require.EqualValues(t, 1, summary.RequestCount)
	require.EqualValues(t, 1_500_000, *summary.SalesMultiplierPpm)
	var run model.SupplierUsageDailyBatchRun
	require.NoError(t, mainDB.First(&run).Error)
	require.Equal(t, run.FenceToken, run.PublishedFenceToken)
}

func TestRunSupplierDailyBatchNeutralizesInternalCustomerAndRoutingDimensions(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)

	for index, channelID := range []int{41, 42} {
		snapshot := supplierDailySnapshot(day, int64(700_000+index))
		snapshot.BindingVersionId += index
		snapshot.RateVersionId += index
		snapshot.StatisticsScope = string(types.SupplierStatisticsScopeInternal)
		snapshot.ExclusionDecision = "excluded"
		exclusionRuleID := 90 + index
		snapshot.ExclusionRuleId = &exclusionRuleID
		snapshot.SalesMultiplierPpm = nil
		snapshot.SalesMicroUsd = nil
		snapshot.GrossProfitMicroUsd = nil
		snapshot.PricingProvenance = nil
		require.NoError(t, logDB.Create(&model.Log{
			Type: model.LogTypeConsume, CreatedAt: day.Add(time.Duration(index+1) * time.Hour).Unix(),
			ChannelId: channelID, ModelName: "internal-model-" + strconv.Itoa(index), Other: supplierDailyLogOther(t, snapshot),
		}).Error)
	}

	require.NoError(t, RunSupplierDailyBatch(context.Background(), mainDB, logDB, day.Format("2006-01-02"), "console", day.AddDate(0, 0, 2)))
	var summaries []model.SupplierUsageDailySummary
	require.NoError(t, mainDB.Find(&summaries).Error)
	require.Len(t, summaries, 1, "internal rows differing only by customer/routing dimensions must coalesce")
	summary := summaries[0]
	require.EqualValues(t, 2, summary.RequestCount)
	require.Zero(t, summary.BindingVersionId)
	require.Zero(t, summary.RateVersionId)
	require.Zero(t, summary.ChannelId)
	require.Empty(t, summary.ModelName)
	require.Nil(t, summary.SalesMultiplierPpm)
	require.Empty(t, summary.PricingMode)
	require.EqualValues(t, 1, summary.SupplierId)
	require.EqualValues(t, 2, summary.ContractId)
	require.Equal(t, string(types.SupplierStatisticsScopeInternal), summary.StatisticsScope)
	require.Equal(t, SupplierDataQualityAuthoritative, summary.DataQuality)
	require.EqualValues(t, 2, summary.OfficialListKnownCount)
	require.EqualValues(t, 2_000, summary.OfficialListMicroUsd)
	require.EqualValues(t, 2, summary.ProcurementCostKnownCount)
	require.EqualValues(t, 1_400, summary.ProcurementCostMicroUsd)
	require.Zero(t, summary.SalesKnownCount)
	require.Zero(t, summary.GrossProfitKnownCount)
}

func TestCatchUpSupplierDailyBatchesWaitsForCloseGrace(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	today := time.Date(2026, 7, 22, 0, 0, 0, 0, location)
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", strconv.FormatInt(today.AddDate(0, 0, -1).Unix(), 10))

	result, err := CatchUpSupplierDailyBatches(context.Background(), mainDB, logDB, "console", today.Add(SupplierDailyCloseGrace-time.Second))
	require.NoError(t, err)
	require.Equal(t, SupplierDailyBatchCatchUpResult{}, result)
	result, err = CatchUpSupplierDailyBatches(context.Background(), mainDB, logDB, "console", today.Add(SupplierDailyCloseGrace))
	require.NoError(t, err)
	require.Equal(t, SupplierDailyBatchCatchUpResult{ProcessedDays: 1}, result)
}

func TestSupplierDailyBatchLeaseUsesDatabaseTimeAndFencesStaleOwner(t *testing.T) {
	db := supplierDailyTestDB(t, t.Name())
	require.NoError(t, db.AutoMigrate(&model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{}))
	now := time.Now()
	first, err := model.AcquireSupplierDailyBatch(context.Background(), db, "2026-12-01", now.Add(-24*time.Hour).Unix(), now.Unix(), "node-a", time.Minute)
	require.NoError(t, err)
	_, err = model.AcquireSupplierDailyBatch(context.Background(), db, "2026-12-01", now.Add(-24*time.Hour).Unix(), now.Unix(), "node-b", time.Minute)
	require.ErrorIs(t, err, model.ErrSupplierDailyBatchBusy)
	require.NoError(t, db.Model(&model.SupplierUsageDailyBatchRun{}).Where("id = ?", first.RunId).Update("locked_until", 0).Error)
	second, err := model.AcquireSupplierDailyBatch(context.Background(), db, "2026-12-01", now.Add(-24*time.Hour).Unix(), now.Unix(), "node-b", time.Minute)
	require.NoError(t, err)
	require.Greater(t, second.FenceToken, first.FenceToken)
	require.ErrorIs(t, model.RenewSupplierDailyBatchLease(context.Background(), db, first, time.Minute), model.ErrSupplierDailyBatchFenceLost)
}
