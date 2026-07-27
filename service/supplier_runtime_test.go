package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	require.NoError(t, model.EnsureSupplierAccountingFactSchema(logDB))
	return mainDB, logDB
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

func setSupplierDailyFactPreparedAt(t *testing.T, db *gorm.DB, fact *model.SupplierAccountingFact, preparedAt time.Time) {
	t.Helper()
	preparedDay := preparedAt.In(preparedAt.Location()).Format("2006-01-02")
	require.NoError(t, db.Model(&model.SupplierAccountingFact{}).Where("id = ?", fact.Id).
		Updates(map[string]any{"prepared_at": preparedAt.Unix(), "prepared_day": preparedDay}).Error)
	fact.PreparedAt = preparedAt.Unix()
	fact.PreparedDay = preparedDay
}

func TestRunSupplierDailyBatchAggregatesCapturedSnapshot(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	fact, err := model.PrepareSupplierAccountingFact(context.Background(), logDB, model.SupplierAccountingFactPrepare{
		AttemptId: "018f843e-7e3a-7f61-a0a0-000000000101", ParentRequestId: "req-1", RetryIndex: 0,
		SupplierId: 1, ContractId: 2, BindingVersionId: 8, RateVersionId: 3, ChannelId: 4, ModelName: "gpt-test",
		CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
	})
	require.NoError(t, err)
	setSupplierDailyFactPreparedAt(t, logDB, &fact, day.Add(time.Hour))
	require.NoError(t, model.FinalizeSupplierAccountingFactCaptured(context.Background(), logDB, fact.AttemptId,
		types.SupplierAccountingEnvelopeV1{EnvelopeSchemaVersion: types.SupplierAccountingEnvelopeSchemaVersionV1,
			Disposition: types.SupplierAccountingDispositionCaptured, Captured: ptrSupplierDailySnapshot(supplierDailySnapshot(day, 1_500_000))}, day.Add(time.Hour).Unix()))

	require.NoError(t, RunSupplierDailyBatch(context.Background(), mainDB, logDB, day.Format("2006-01-02"), "console", day.AddDate(0, 0, 2)))
	var summary model.SupplierUsageDailySummary
	require.NoError(t, mainDB.First(&summary).Error)
	require.EqualValues(t, 1, summary.RequestCount)
	require.EqualValues(t, 1_500_000, *summary.SalesMultiplierPpm)
	var run model.SupplierUsageDailyBatchRun
	require.NoError(t, mainDB.First(&run).Error)
	require.Equal(t, run.FenceToken, run.PublishedFenceToken)
	require.Equal(t, fact.Id, run.SourceMaxFactId)
	require.Equal(t, string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1), run.CoverageScope)
}

func ptrSupplierDailySnapshot(snapshot types.SupplierAccountingLogSnapshotV1) *types.SupplierAccountingLogSnapshotV1 {
	return &snapshot
}

func TestRunSupplierDailyBatchDoesNotPublishPendingOrLateFacts(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	fact, err := model.PrepareSupplierAccountingFact(context.Background(), logDB, model.SupplierAccountingFactPrepare{
		AttemptId: "018f843e-7e3a-7f61-a0a0-000000000102", ParentRequestId: "req-pending", ChannelId: 4,
		SupplierId: 1, ContractId: 2, BindingVersionId: 8, RateVersionId: 3, ModelName: "gpt-test",
		CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
	})
	require.NoError(t, err)
	setSupplierDailyFactPreparedAt(t, logDB, &fact, day.Add(time.Hour))

	err = RunSupplierDailyBatch(context.Background(), mainDB, logDB, day.Format("2006-01-02"), "console", day.AddDate(0, 0, 2))
	require.ErrorIs(t, err, model.ErrSupplierAccountingFactsPending)
	var run model.SupplierUsageDailyBatchRun
	require.NoError(t, mainDB.First(&run).Error)
	require.Zero(t, run.PublishedFenceToken)

	require.NoError(t, model.FinalizeSupplierAccountingFactVoid(context.Background(), logDB, fact.AttemptId, day.Add(2*time.Hour).Unix()))
	require.NoError(t, RunSupplierDailyBatch(context.Background(), mainDB, logDB, day.Format("2006-01-02"), "console", day.AddDate(0, 0, 2)))
	require.NoError(t, mainDB.First(&run).Error)
	require.NotZero(t, run.PublishedFenceToken)
}

func TestRunSupplierDailyBatchClosesExactCapturedPageAndTrailingVoid(t *testing.T) {
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	payload, err := common.Marshal(types.SupplierAccountingEnvelopeV1{
		EnvelopeSchemaVersion: types.SupplierAccountingEnvelopeSchemaVersionV1,
		Disposition:           types.SupplierAccountingDispositionCaptured,
		Captured:              ptrSupplierDailySnapshot(supplierDailySnapshot(day, 1_500_000)),
	})
	require.NoError(t, err)
	payloadDigest := sha256.Sum256(payload)
	payloadHash := hex.EncodeToString(payloadDigest[:])
	for _, trailingVoid := range []bool{false, true} {
		t.Run(fmt.Sprintf("trailing_void_%t", trailingVoid), func(t *testing.T) {
			mainDB, logDB := supplierDailyTestDBs(t)
			facts := make([]model.SupplierAccountingFact, model.SupplierAccountingFactPageSize)
			for index := range facts {
				terminalAt := day.Add(time.Hour).Unix()
				facts[index] = model.SupplierAccountingFact{
					AttemptId: fmt.Sprintf("%036d", index+1), ParentRequestId: "page-boundary", PreparedAt: terminalAt,
					PreparedDay: day.Format("2006-01-02"), SupplierId: 1, ContractId: 2, BindingVersionId: 8, RateVersionId: 3,
					ChannelId: 4, ModelName: "gpt-test", CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
					Status: model.SupplierAccountingFactStatusCaptured, Payload: string(payload), PayloadHash: payloadHash, TerminalAt: &terminalAt,
				}
			}
			require.NoError(t, logDB.CreateInBatches(&facts, 500).Error)
			if trailingVoid {
				terminalAt := day.Add(2 * time.Hour).Unix()
				require.NoError(t, logDB.Create(&model.SupplierAccountingFact{
					AttemptId: fmt.Sprintf("%036d", len(facts)+1), ParentRequestId: "trailing-void", PreparedAt: terminalAt,
					PreparedDay: day.Format("2006-01-02"), SupplierId: 1, ContractId: 2, BindingVersionId: 8, RateVersionId: 3,
					ChannelId: 4, ModelName: "gpt-test", CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
					Status: model.SupplierAccountingFactStatusVoid, TerminalAt: &terminalAt,
				}).Error)
			}
			require.NoError(t, RunSupplierDailyBatch(context.Background(), mainDB, logDB, day.Format("2006-01-02"), "console", day.AddDate(0, 0, 2)))
			var run model.SupplierUsageDailyBatchRun
			require.NoError(t, mainDB.First(&run).Error)
			require.EqualValues(t, model.SupplierAccountingFactPageSize, run.SnapshotCount)
			require.NotZero(t, run.PublishedFenceToken)
		})
	}
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
		fact, err := model.PrepareSupplierAccountingFact(context.Background(), logDB, model.SupplierAccountingFactPrepare{
			AttemptId: "018f843e-7e3a-7f61-a0a0-00000000020" + strconv.Itoa(index), ParentRequestId: "internal-" + strconv.Itoa(index),
			SupplierId: snapshot.SupplierId, ContractId: snapshot.ContractId, BindingVersionId: snapshot.BindingVersionId,
			RateVersionId: snapshot.RateVersionId, ChannelId: channelID, ModelName: "internal-model-" + strconv.Itoa(index),
			CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
		})
		require.NoError(t, err)
		setSupplierDailyFactPreparedAt(t, logDB, &fact, day.Add(time.Duration(index+1)*time.Hour))
		require.NoError(t, model.FinalizeSupplierAccountingFactCaptured(context.Background(), logDB, fact.AttemptId,
			types.SupplierAccountingEnvelopeV1{EnvelopeSchemaVersion: types.SupplierAccountingEnvelopeSchemaVersionV1,
				Disposition: types.SupplierAccountingDispositionCaptured, Captured: &snapshot}, day.Add(time.Duration(index+1)*time.Hour).Unix()))
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

func TestCatchUpSupplierDailyBatchesIsDisabledWithoutCutover(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", "")
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, location)

	result, err := CatchUpSupplierDailyBatches(context.Background(), mainDB, logDB, "console", now)
	require.NoError(t, err)
	require.Equal(t, SupplierDailyBatchCatchUpResult{}, result)
	var count int64
	require.NoError(t, mainDB.Model(&model.SupplierUsageDailyBatchRun{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestSupplierAccountingCutoverRejectsPartialShanghaiDay(t *testing.T) {
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	nonMidnight := time.Date(2026, 7, 25, 0, 0, 1, 0, location)
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", strconv.FormatInt(nonMidnight.Unix(), 10))

	_, configured, err := configuredSupplierAccountingCutover()
	require.Error(t, err)
	require.False(t, configured)
}

func TestCatchUpSupplierDailyBatchesStartsAtCutoverDayOnlyAfterTPlusOne(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	cutover := time.Date(2026, 7, 25, 0, 0, 0, 0, location)
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", strconv.FormatInt(cutover.Unix(), 10))

	result, err := CatchUpSupplierDailyBatches(context.Background(), mainDB, logDB, "console", cutover.Add(SupplierDailyCloseGrace))
	require.NoError(t, err)
	require.Equal(t, SupplierDailyBatchCatchUpResult{}, result)
	result, err = CatchUpSupplierDailyBatches(context.Background(), mainDB, logDB, "console", cutover.AddDate(0, 0, 1).Add(SupplierDailyCloseGrace))
	require.NoError(t, err)
	require.Equal(t, 1, result.ProcessedDays)
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
