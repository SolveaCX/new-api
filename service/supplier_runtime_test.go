package service

import (
	"context"
	"errors"
	"fmt"
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
	require.NoError(t, mainDB.AutoMigrate(&model.Option{}, &model.SupplierAccountingCoverageGap{}, &model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{}))
	require.NoError(t, logDB.AutoMigrate(&model.Log{}))
	return mainDB, logDB
}

func persistLegacySupplierAccountingCoverageStart(t *testing.T, db *gorm.DB, cutoverAt int64) {
	t.Helper()
	require.Greater(t, cutoverAt, int64(0))
	require.NoError(t, db.Create(&model.Option{
		Key:   model.SupplierAccountingCoverageStartOptionKey,
		Value: fmt.Sprint(cutoverAt),
	}).Error)
}

func armSupplierAccountingForBatch(t *testing.T, db *gorm.DB, cutoverAt int64) model.SupplierAccountingActivationState {
	t.Helper()
	preparedAt := cutoverAt - 2
	preparedBy := 7
	shadow, err := model.CASSupplierAccountingActivationState(db, 0, model.SupplierAccountingActivationState{
		Phase: model.SupplierAccountingActivationShadow, AcceptedCapabilityVersions: []int{1},
		PreparedAt: &preparedAt, PreparedBy: &preparedBy, Reason: "prepare supplier batch test",
	}, preparedAt)
	require.NoError(t, err)
	armed := shadow
	armed.Phase = model.SupplierAccountingActivationArmed
	armed.CutoverAt = &cutoverAt
	armed.Reason = "arm supplier batch test"
	armed, err = model.CASSupplierAccountingActivationState(db, shadow.StateVersion, armed, cutoverAt-1)
	require.NoError(t, err)
	return armed
}

func activateSupplierAccountingForBatch(t *testing.T, db *gorm.DB, cutoverAt int64) model.SupplierAccountingActivationState {
	t.Helper()
	armed := armSupplierAccountingForBatch(t, db, cutoverAt)
	activatedAt := cutoverAt
	active := armed
	active.Phase = model.SupplierAccountingActivationActive
	active.ActivatedAt = &activatedAt
	active.Reason = "activate supplier batch test"
	active, err := model.CASSupplierAccountingActivationState(db, armed.StateVersion, active, activatedAt)
	require.NoError(t, err)
	return active
}

func degradeSupplierAccountingForBatch(t *testing.T, db *gorm.DB, active model.SupplierAccountingActivationState) model.SupplierAccountingActivationState {
	t.Helper()
	degradedAt := *active.ActivatedAt + 1
	degraded := active
	degraded.Phase = model.SupplierAccountingActivationDegraded
	degraded.DegradedAt = &degradedAt
	degraded.Reason = "degrade supplier batch test"
	degraded, err := model.CASSupplierAccountingActivationState(db, active.StateVersion, degraded, degradedAt)
	require.NoError(t, err)
	return degraded
}

func supplierDailyLogOther(t *testing.T, snapshot types.SupplierAccountingLogSnapshotV1) string {
	t.Helper()
	payload, err := common.Marshal(map[string]any{types.SupplierAccountingEnvelopeKeyV1: types.SupplierAccountingEnvelopeV1{
		EnvelopeSchemaVersion: types.SupplierAccountingEnvelopeSchemaVersionV1, ProducerCapabilityVersion: types.SupplierAccountingProducerCapabilityV1,
		ActivationStateVersion: 1, Disposition: types.SupplierAccountingDispositionCaptured, Captured: &snapshot,
	}})
	require.NoError(t, err)
	return string(payload)
}

func supplierDailySnapshot(day time.Time, multiplier int64) types.SupplierAccountingLogSnapshotV1 {
	official, sales, procurement, gross := int64(1_000), int64(2_000), int64(700), int64(1_300)
	return types.SupplierAccountingLogSnapshotV1{
		BindingVersionId: 8, SupplierId: 1, ContractId: 2, RateVersionId: 3,
		ProcurementMultiplierPpm: 700_000,
		SalesMultiplierPpm:       &multiplier, OfficialListMicroUsd: &official, SalesMicroUsd: &sales,
		ProcurementCostMicroUsd: &procurement, GrossProfitMicroUsd: &gross,
		StatisticsScope: string(types.SupplierStatisticsScopeBusiness), ExclusionDecision: "included",
		FinanciallyCommittedAt: day.Add(time.Hour).Unix(),
		PricingProvenance: &types.SupplierPricingProvenanceV1{Ratio: &types.SupplierRatioPricingProvenanceV1{
			ModelRatioPpm: 1_000_000, GroupRatioPpm: multiplier, ModelRatioVersion: 1, GroupRatioVersion: 1,
		}},
	}
}

func TestRunSupplierDailyBatchAggregatesSalesMultiplierAsDimension(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())
	var legacyCoverageRows int64
	require.NoError(t, mainDB.Model(&model.Option{}).Where("key = ?", model.SupplierAccountingCoverageStartOptionKey).Count(&legacyCoverageRows).Error)
	require.Zero(t, legacyCoverageRows, "prepare-to-arm-to-active must not require legacy coverage evidence")
	for index, multiplier := range []int64{300_000, 900_000} {
		snapshot := supplierDailySnapshot(day, multiplier)
		require.NoError(t, logDB.Create(&model.Log{
			Type: model.LogTypeConsume, CreatedAt: day.Add(time.Duration(index+1) * time.Hour).Unix(),
			ChannelId: 4, ModelName: "gpt-test", Other: supplierDailyLogOther(t, snapshot),
		}).Error)
	}

	require.NoError(t, RunSupplierDailyBatch(context.Background(), mainDB, logDB, day.Format("2006-01-02"), "console-a", day.AddDate(0, 0, 2), false))
	var rows []model.SupplierUsageDailySummary
	require.NoError(t, mainDB.Order("sales_multiplier_ppm").Find(&rows).Error)
	require.Len(t, rows, 2)
	require.EqualValues(t, 300_000, *rows[0].SalesMultiplierPpm)
	require.EqualValues(t, 900_000, *rows[1].SalesMultiplierPpm)
}

func TestRunSupplierDailyBatchAttributesAccountingDayByConsumeLogCreatedAt(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())

	rows := []struct {
		createdAt              int64
		financiallyCommittedAt int64
	}{
		{createdAt: day.Add(-time.Second).Unix(), financiallyCommittedAt: day.Add(time.Hour).Unix()},
		{createdAt: day.Unix(), financiallyCommittedAt: day.Add(-time.Second).Unix()},
		{createdAt: day.AddDate(0, 0, 1).Add(-time.Second).Unix(), financiallyCommittedAt: day.AddDate(0, 0, 1).Add(time.Second).Unix()},
		{createdAt: day.AddDate(0, 0, 1).Unix(), financiallyCommittedAt: day.Add(2 * time.Hour).Unix()},
	}
	for _, row := range rows {
		snapshot := supplierDailySnapshot(day, 300_000)
		snapshot.FinanciallyCommittedAt = row.financiallyCommittedAt
		require.NoError(t, logDB.Create(&model.Log{
			Type: model.LogTypeConsume, CreatedAt: row.createdAt, ChannelId: 4,
			ModelName: "created-at-contract", Other: supplierDailyLogOther(t, snapshot),
		}).Error)
	}

	// Contract: accounting-day attribution follows the immutable consume log's
	// Asia/Shanghai created_at [start,end). FinanciallyCommittedAt remains audit
	// evidence and may legitimately fall on the adjacent calendar day.
	require.NoError(t, RunSupplierDailyBatch(context.Background(), mainDB, logDB, day.Format("2006-01-02"), "console-a", day.AddDate(0, 0, 2), false))
	var summary model.SupplierUsageDailySummary
	require.NoError(t, mainDB.Where("batch_date = ?", day.Format("2006-01-02")).First(&summary).Error)
	require.EqualValues(t, 2, summary.RequestCount)
	require.EqualValues(t, 2_000, summary.OfficialListMicroUsd)
	require.EqualValues(t, 4_000, summary.SalesMicroUsd)
	require.EqualValues(t, 1_400, summary.ProcurementCostMicroUsd)
	require.EqualValues(t, 2_600, summary.GrossProfitMicroUsd)
}

func TestRunSupplierDailyBatchInternalSnapshotRetainsInventoryOnly(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())
	snapshot := supplierDailySnapshot(day, 300_000)
	snapshot.StatisticsScope = string(types.SupplierStatisticsScopeInternal)
	snapshot.ExclusionDecision = "excluded"
	ruleID := 9
	snapshot.ExclusionRuleId = &ruleID
	snapshot.SalesMultiplierPpm = nil
	snapshot.SalesMicroUsd = nil
	snapshot.GrossProfitMicroUsd = nil
	require.NoError(t, logDB.Create(&model.Log{
		Type: model.LogTypeConsume, CreatedAt: day.Add(time.Hour).Unix(), ChannelId: 4,
		ModelName: "must-not-be-dimensional", Other: supplierDailyLogOther(t, snapshot),
	}).Error)

	require.NoError(t, RunSupplierDailyBatch(context.Background(), mainDB, logDB, day.Format("2006-01-02"), "console-a", day.AddDate(0, 0, 2), false))
	var summary model.SupplierUsageDailySummary
	require.NoError(t, mainDB.Where("batch_date = ?", day.Format("2006-01-02")).First(&summary).Error)
	require.Equal(t, string(types.SupplierStatisticsScopeInternal), summary.StatisticsScope)
	require.Empty(t, summary.ModelName)
	require.EqualValues(t, 1, summary.RequestCount)
	require.EqualValues(t, 1, summary.OfficialListKnownCount)
	require.EqualValues(t, 1_000, summary.OfficialListMicroUsd)
	require.EqualValues(t, 1, summary.ProcurementCostKnownCount)
	require.EqualValues(t, 700, summary.ProcurementCostMicroUsd)
	require.Zero(t, summary.SalesKnownCount)
	require.Zero(t, summary.SalesMicroUsd)
	require.Zero(t, summary.GrossProfitKnownCount)
	require.Zero(t, summary.GrossProfitMicroUsd)
	require.Zero(t, summary.GrossMarginEligibleCount)
	require.Zero(t, summary.GrossMarginEligibleSalesMicroUsd)
}

func TestSupplierDailyBatchUsesStableExactCutoverAndConsumeLogsOnly(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	cutover := day.Add(90 * time.Minute).Unix()

	snapshot := supplierDailySnapshot(day, 300_000)
	for _, row := range []model.Log{
		{Type: model.LogTypeConsume, CreatedAt: cutover - 1, ChannelId: 4, Other: supplierDailyLogOther(t, snapshot)},
		{Type: model.LogTypeConsume, CreatedAt: cutover + 1, ChannelId: 4, Other: supplierDailyLogOther(t, snapshot)},
		{Type: model.LogTypeRefund, CreatedAt: cutover + 2, ChannelId: 4, Other: supplierDailyLogOther(t, snapshot)},
		{Type: model.LogTypeError, CreatedAt: cutover + 3, ChannelId: 4, Other: supplierDailyLogOther(t, snapshot)},
	} {
		require.NoError(t, logDB.Create(&row).Error)
	}
	active := activateSupplierAccountingForBatch(t, mainDB, cutover)
	legacyCutover := day.Add(3 * time.Hour).Unix()
	persistLegacySupplierAccountingCoverageStart(t, mainDB, legacyCutover)
	globalDB := supplierDailyTestDB(t, t.Name()+"-disabled-global")
	require.NoError(t, globalDB.AutoMigrate(&model.Option{}))
	originalDB := model.DB
	model.DB = globalDB
	t.Cleanup(func() { model.DB = originalDB })
	catchUp, err := CatchUpSupplierDailyBatches(context.Background(), mainDB, logDB, "console", day.AddDate(0, 0, 2).Add(12*time.Hour))
	require.NoError(t, err)
	require.Equal(t, SupplierDailyBatchCatchUpResult{ProcessedDays: 1, RemainingWork: true, NextBatchDate: day.AddDate(0, 0, 1).Format("2006-01-02")}, catchUp)

	var summary model.SupplierUsageDailySummary
	require.NoError(t, mainDB.First(&summary).Error)
	require.EqualValues(t, 1, summary.RequestCount)
	persisted, err := model.ReadSupplierAccountingActivationState(mainDB)
	require.NoError(t, err)
	require.Equal(t, active.StateVersion, persisted.StateVersion)
	require.Equal(t, cutover, *persisted.CutoverAt)
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", fmt.Sprint(day.Add(3*time.Hour).Unix()))
	stable, err := model.SupplierAccountingCoverageStart(context.Background(), mainDB)
	require.NoError(t, err)
	require.Equal(t, legacyCutover, stable)
}

func TestSupplierDailyBatchBlocksLegacyOnlyAndPreActivePhases(t *testing.T) {
	for _, phase := range []string{"legacy-only", "shadow", "armed", "retired"} {
		t.Run(phase, func(t *testing.T) {
			mainDB, logDB := supplierDailyTestDBs(t)
			location, err := time.LoadLocation(SupplierDailyBatchTimezone)
			require.NoError(t, err)
			day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
			persistLegacySupplierAccountingCoverageStart(t, mainDB, day.Unix())

			switch phase {
			case "shadow":
				preparedAt := day.Unix() - 2
				preparedBy := 7
				_, err = model.CASSupplierAccountingActivationState(mainDB, 0, model.SupplierAccountingActivationState{
					Phase: model.SupplierAccountingActivationShadow, AcceptedCapabilityVersions: []int{1},
					PreparedAt: &preparedAt, PreparedBy: &preparedBy, Reason: "prepare without activate",
				}, preparedAt)
				require.NoError(t, err)
			case "armed":
				armSupplierAccountingForBatch(t, mainDB, day.Unix())
			case "retired":
				active := activateSupplierAccountingForBatch(t, mainDB, day.Unix())
				retired := active
				retired.Phase = model.SupplierAccountingActivationRetired
				retired.Reason = "retire supplier batch test"
				_, err = model.CASSupplierAccountingActivationState(mainDB, active.StateVersion, retired, day.Unix()+1)
				require.NoError(t, err)
			}

			err = RunSupplierDailyBatch(context.Background(), mainDB, logDB, day.Format("2006-01-02"), "blocked", day.AddDate(0, 0, 2), false)
			require.ErrorIs(t, err, ErrSupplierAccountingNotActive)
			var runs int64
			require.NoError(t, mainDB.Model(&model.SupplierUsageDailyBatchRun{}).Count(&runs).Error)
			require.Zero(t, runs, "phase eligibility must be checked before acquiring a batch lease")
		})
	}
}

func TestSupplierDailyBatchAllowsDegradedCanonicalActivation(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	active := activateSupplierAccountingForBatch(t, mainDB, day.Unix())
	degradeSupplierAccountingForBatch(t, mainDB, active)

	require.NoError(t, RunSupplierDailyBatch(context.Background(), mainDB, logDB, day.Format("2006-01-02"), "degraded", day.AddDate(0, 0, 2), false))
}

func TestInitializeSupplierAccountingCoverageStartDoesNotInventDBTime(t *testing.T) {
	mainDB, _ := supplierDailyTestDBs(t)
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", "1800000000")
	coverageStartAt, err := InitializeSupplierAccountingCoverageStart(context.Background(), mainDB)
	require.NoError(t, err)
	require.Zero(t, coverageStartAt)

	var optionCount int64
	require.NoError(t, mainDB.Model(&model.Option{}).Count(&optionCount).Error)
	require.Zero(t, optionCount)
}

func TestCatchUpSupplierDailyBatchesWaitsForCloseGrace(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	today := time.Date(2026, 7, 22, 0, 0, 0, 0, location)
	activateSupplierAccountingForBatch(t, mainDB, today.AddDate(0, 0, -1).Unix())
	result, err := CatchUpSupplierDailyBatches(context.Background(), mainDB, logDB, "console", today.Add(SupplierDailyCloseGrace-time.Second))
	require.NoError(t, err)
	require.Equal(t, SupplierDailyBatchCatchUpResult{}, result, "before close grace there is no eligible work and no next batch date")
	var count int64
	require.NoError(t, mainDB.Model(&model.SupplierUsageDailyBatchRun{}).Count(&count).Error)
	require.Zero(t, count)
	result, err = CatchUpSupplierDailyBatches(context.Background(), mainDB, logDB, "console", today.Add(SupplierDailyCloseGrace))
	require.NoError(t, err)
	require.Equal(t, SupplierDailyBatchCatchUpResult{ProcessedDays: 1}, result)
	require.NoError(t, mainDB.Model(&model.SupplierUsageDailyBatchRun{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestCatchUpSupplierDailyBatchesProcessesOneMissingDayPerInvocation(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	today := time.Date(2026, 7, 23, 3, 0, 0, 0, location)
	firstMissing := today.AddDate(0, 0, -10)
	activateSupplierAccountingForBatch(t, mainDB, firstMissing.Unix())

	result, err := CatchUpSupplierDailyBatches(context.Background(), mainDB, logDB, "bounded-call-1", today)
	require.NoError(t, err)
	require.Equal(t, SupplierDailyBatchCatchUpResult{
		ProcessedDays: 1, RemainingWork: true,
		NextBatchDate: firstMissing.AddDate(0, 0, 1).Format("2006-01-02"),
	}, result)
	var runs []model.SupplierUsageDailyBatchRun
	require.NoError(t, mainDB.Order("batch_date ASC").Find(&runs).Error)
	require.Len(t, runs, SupplierDailyCatchUpMaxDays)
	require.Equal(t, firstMissing.Format("2006-01-02"), runs[0].BatchDate)

	result, err = CatchUpSupplierDailyBatches(context.Background(), mainDB, logDB, "bounded-call-2", today)
	require.NoError(t, err)
	require.Equal(t, SupplierDailyBatchCatchUpResult{
		ProcessedDays: 1, RemainingWork: true,
		NextBatchDate: firstMissing.AddDate(0, 0, 2).Format("2006-01-02"),
	}, result)
	runs = nil
	require.NoError(t, mainDB.Order("batch_date ASC").Find(&runs).Error)
	require.Len(t, runs, 2*SupplierDailyCatchUpMaxDays)
}

func TestAsyncTaskSuccessAndFailureDoNotCreateSupplierSnapshotsOrSummaries(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())
	for index, status := range []model.TaskStatus{model.TaskStatusSuccess, model.TaskStatusFailure} {
		task := &model.Task{TaskID: fmt.Sprintf("async-%d", index), Status: status, ChannelId: 4, Properties: model.Properties{OriginModelName: "video"}}
		other := taskBillingOther(task)
		require.NotContains(t, other, "supplier_accounting_v1")
		require.NoError(t, logDB.Create(&model.Log{
			Type: model.LogTypeConsume, CreatedAt: day.Add(time.Duration(index+1) * time.Hour).Unix(),
			ChannelId: task.ChannelId, ModelName: "video", Other: common.MapToJsonStr(other),
		}).Error)
	}
	require.NoError(t, RunSupplierDailyBatch(context.Background(), mainDB, logDB, day.Format("2006-01-02"), "console", day.AddDate(0, 0, 2), false))
	var summaryCount int64
	require.NoError(t, mainDB.Model(&model.SupplierUsageDailySummary{}).Count(&summaryCount).Error)
	require.Zero(t, summaryCount)
}

func TestSupplierFreshnessExposesSyncOnlyAndCanonicalCutover(t *testing.T) {
	mainDB, _ := supplierDailyTestDBs(t)
	coverageStartAt := time.Now().Add(-time.Hour).Unix()
	activateSupplierAccountingForBatch(t, mainDB, coverageStartAt)
	result, err := NewSupplierReportService(model.NewSupplierReportStore(mainDB)).GetFreshness(context.Background())
	require.NoError(t, err)
	require.True(t, result.SyncOnly)
	require.Equal(t, coverageStartAt, result.CoverageStartAt)
}

func TestSupplierFreshnessDoesNotInventDMinusOneCoverage(t *testing.T) {
	mainDB, _ := supplierDailyTestDBs(t)
	result, err := NewSupplierReportService(model.NewSupplierReportStore(mainDB)).GetFreshness(context.Background())
	require.NoError(t, err)
	require.Zero(t, result.CoverageStartAt)
}

func TestSupplierDailyBatchLeaseUsesDatabaseTimeAndFencesStaleOwner(t *testing.T) {
	db := supplierDailyTestDB(t, t.Name())
	require.NoError(t, db.AutoMigrate(&model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{}))
	now := time.Now()
	first, err := model.AcquireSupplierDailyBatch(context.Background(), db, "2026-12-01", now.Add(-24*time.Hour).Unix(), now.Unix(), "node-a", time.Unix(1, 0), time.Minute, false)
	require.NoError(t, err)
	_, err = model.AcquireSupplierDailyBatch(context.Background(), db, "2026-12-01", now.Add(-24*time.Hour).Unix(), now.Unix(), "node-b", time.Unix(mathMaxUnixForTest, 0), time.Minute, false)
	require.ErrorIs(t, err, model.ErrSupplierDailyBatchBusy)
	require.NoError(t, db.Model(&model.SupplierUsageDailyBatchRun{}).Where("id = ?", first.RunId).Update("locked_until", 0).Error)
	second, err := model.AcquireSupplierDailyBatch(context.Background(), db, "2026-12-01", now.Add(-24*time.Hour).Unix(), now.Unix(), "node-b", time.Time{}, time.Minute, false)
	require.NoError(t, err)
	require.Greater(t, second.FenceToken, first.FenceToken)
	require.ErrorIs(t, model.RenewSupplierDailyBatchLease(context.Background(), db, first, time.Minute), model.ErrSupplierDailyBatchFenceLost)
}

func TestSupplierDailyBatchLeaseRenewAllowsSameSecondUnchangedValue(t *testing.T) {
	db := supplierDailyTestDB(t, t.Name())
	require.NoError(t, db.AutoMigrate(&model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{}))
	now := time.Now()
	lease, err := model.AcquireSupplierDailyBatch(context.Background(), db, "2026-12-02", now.Add(-24*time.Hour).Unix(), now.Unix(), "node-a", time.Time{}, time.Minute, false)
	require.NoError(t, err)
	require.NoError(t, model.RenewSupplierDailyBatchLease(context.Background(), db, lease, time.Minute))
	require.NoError(t, model.RenewSupplierDailyBatchLease(context.Background(), db, lease, time.Minute))
}

func TestSupplierDailyBatchStaleOwnerCannotMutateTakeover(t *testing.T) {
	db := supplierDailyTestDB(t, t.Name())
	require.NoError(t, db.AutoMigrate(&model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{}))
	assertSupplierStaleLeaseCannotMutateNewOwner(t, db, "2026-12-03")
}

func TestSupplierDailyBatchPageCursorIsAtomicAndRetrySafe(t *testing.T) {
	db := supplierDailyTestDB(t, t.Name())
	require.NoError(t, db.AutoMigrate(&model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{}))
	ctx := context.Background()
	day := time.Date(2026, 12, 4, 0, 0, 0, 0, time.UTC)
	lease, err := model.AcquireSupplierDailyBatch(ctx, db, "2026-12-04", day.Unix(), day.AddDate(0, 0, 1).Unix(), "node-a", time.Time{}, time.Minute, false)
	require.NoError(t, err)
	summary := supplierFencingSummary(lease.BatchDate, "page-1", 7)
	require.NoError(t, model.PersistSupplierDailyBatchPage(ctx, db, lease, []model.SupplierUsageDailySummary{summary}, 100, 10, 4, 3, time.Minute))

	// Replaying the page with the stale expected cursor cannot double count: the
	// summary upsert and cursor CAS share one transaction.
	require.ErrorIs(t, model.PersistSupplierDailyBatchPage(ctx, db, lease, []model.SupplierUsageDailySummary{summary}, 100, 10, 4, 3, time.Minute), model.ErrSupplierDailyBatchFenceLost)
	var persisted model.SupplierUsageDailySummary
	require.NoError(t, db.Where("batch_date = ? AND batch_fence_token = ?", lease.BatchDate, lease.FenceToken).First(&persisted).Error)
	require.EqualValues(t, 7, persisted.RequestCount)
	var run model.SupplierUsageDailyBatchRun
	require.NoError(t, db.First(&run, lease.RunId).Error)
	require.EqualValues(t, 100, run.CursorCreatedAt)
	require.EqualValues(t, 10, run.CursorId)
	require.EqualValues(t, 4, run.LogsScanned)
	require.EqualValues(t, 3, run.SnapshotCount)
}

func TestSupplierDailyBatchPageWithoutSnapshotsStillAdvancesCursor(t *testing.T) {
	db := supplierDailyTestDB(t, t.Name())
	require.NoError(t, db.AutoMigrate(&model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{}))
	ctx := context.Background()
	day := time.Date(2026, 12, 7, 0, 0, 0, 0, time.UTC)
	lease, err := model.AcquireSupplierDailyBatch(ctx, db, "2026-12-07", day.Unix(), day.AddDate(0, 0, 1).Unix(), "node-a", time.Time{}, time.Minute, false)
	require.NoError(t, err)
	require.NoError(t, model.PersistSupplierDailyBatchPage(ctx, db, lease, nil, 300, 30, 5, 0, time.Minute))
	var run model.SupplierUsageDailyBatchRun
	require.NoError(t, db.First(&run, lease.RunId).Error)
	require.EqualValues(t, 300, run.CursorCreatedAt)
	require.EqualValues(t, 30, run.CursorId)
	require.EqualValues(t, 5, run.LogsScanned)
	require.Zero(t, run.SnapshotCount)
	var summaryCount int64
	require.NoError(t, db.Model(&model.SupplierUsageDailySummary{}).Count(&summaryCount).Error)
	require.Zero(t, summaryCount)
}

func TestSupplierDailyBatchTakeoverUsesFreshGenerationAndCleansPartialRows(t *testing.T) {
	db := supplierDailyTestDB(t, t.Name())
	require.NoError(t, db.AutoMigrate(&model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{}))
	ctx := context.Background()
	day := time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC)
	leaseA, err := model.AcquireSupplierDailyBatch(ctx, db, "2026-12-05", day.Unix(), day.AddDate(0, 0, 1).Unix(), "node-a", time.Time{}, time.Minute, false)
	require.NoError(t, err)
	require.NoError(t, model.PersistSupplierDailyBatchPage(ctx, db, leaseA, []model.SupplierUsageDailySummary{supplierFencingSummary(leaseA.BatchDate, "partial-a", 1)}, 100, 1, 1, 1, time.Minute))
	require.NoError(t, db.Model(&model.SupplierUsageDailyBatchRun{}).Where("id = ?", leaseA.RunId).Update("locked_until", 0).Error)

	leaseB, err := model.AcquireSupplierDailyBatch(ctx, db, leaseA.BatchDate, day.Unix(), day.AddDate(0, 0, 1).Unix(), "node-b", time.Time{}, time.Minute, false)
	require.NoError(t, err)
	require.Greater(t, leaseB.FenceToken, leaseA.FenceToken)
	require.Zero(t, leaseB.CursorCreatedAt)
	require.Zero(t, leaseB.CursorId)
	var partialCount int64
	require.NoError(t, db.Model(&model.SupplierUsageDailySummary{}).Where("batch_date = ?", leaseA.BatchDate).Count(&partialCount).Error)
	require.Zero(t, partialCount)
}

func TestSupplierDailyBatchForceRerunKeepsPublishedGenerationUntilAtomicPublish(t *testing.T) {
	db := supplierDailyTestDB(t, t.Name())
	require.NoError(t, db.AutoMigrate(&model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{}))
	ctx := context.Background()
	day := time.Date(2026, 12, 6, 0, 0, 0, 0, time.UTC)
	leaseA, err := model.AcquireSupplierDailyBatch(ctx, db, "2026-12-06", day.Unix(), day.AddDate(0, 0, 1).Unix(), "node-a", time.Time{}, time.Minute, false)
	require.NoError(t, err)
	require.NoError(t, model.PersistSupplierDailyBatchPage(ctx, db, leaseA, []model.SupplierUsageDailySummary{supplierFencingSummary(leaseA.BatchDate, "published-a", 11)}, 100, 1, 2, 1, time.Minute))
	leaseA.CursorCreatedAt, leaseA.CursorId = 100, 1
	require.NoError(t, model.PublishSupplierDailyBatch(ctx, db, leaseA, time.Now(), types.SupplierPublishedEvidenceV1{
		SchemaVersion: types.SupplierPublishedEvidenceSchemaVersion, LogsScanned: 2,
		ProducerMarkersPresent: 1, CapturedSnapshotCount: 1,
		DispositionCounts:                types.SupplierPublishedDispositionCountsV1{Captured: 1},
		FailureCounts:                    types.SupplierPublishedFailureCountsV1{AbsentMarkerAfterCutover: 1},
		PersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessIncomplete,
		Warnings: []types.SupplierPublishedWarningV1{{
			Code: types.SupplierPublishedWarningAbsentMarker, MessageKey: "supply_chain.warning.absent_marker_after_cutover", Count: 1,
		}},
	}))

	leaseB, err := model.AcquireSupplierDailyBatchRerun(ctx, db, leaseA.BatchDate, day.Unix(), day.AddDate(0, 0, 1).Unix(), "node-b", time.Time{}, time.Minute, leaseA.FenceToken)
	require.NoError(t, err)
	require.Greater(t, leaseB.FenceToken, leaseA.FenceToken)
	require.NoError(t, model.PersistSupplierDailyBatchPage(ctx, db, leaseB, []model.SupplierUsageDailySummary{supplierFencingSummary(leaseB.BatchDate, "candidate-b", 22)}, 200, 2, 1, 1, time.Minute))

	var run model.SupplierUsageDailyBatchRun
	require.NoError(t, db.First(&run, leaseB.RunId).Error)
	require.Equal(t, leaseA.FenceToken, run.PublishedFenceToken)
	var generations []int64
	require.NoError(t, db.Model(&model.SupplierUsageDailySummary{}).Where("batch_date = ?", leaseA.BatchDate).Distinct("batch_fence_token").Order("batch_fence_token").Scan(&generations).Error)
	require.Equal(t, []int64{leaseA.FenceToken, leaseB.FenceToken}, generations)

	leaseB.CursorCreatedAt, leaseB.CursorId = 200, 2
	require.NoError(t, model.CompleteSupplierDailyBatch(ctx, db, leaseB, time.Now()))
	require.NoError(t, db.First(&run, leaseB.RunId).Error)
	require.Equal(t, leaseB.FenceToken, run.PublishedFenceToken)
	generations = nil
	require.NoError(t, db.Model(&model.SupplierUsageDailySummary{}).Where("batch_date = ?", leaseA.BatchDate).Distinct("batch_fence_token").Scan(&generations).Error)
	require.Equal(t, []int64{leaseB.FenceToken}, generations)
}

func TestCatchUpSupplierDailyBatchesSkipsCompletedHistory(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	today := time.Date(2026, 7, 23, 3, 0, 0, 0, location)
	firstDay := today.AddDate(0, 0, -201)
	activateSupplierAccountingForBatch(t, mainDB, firstDay.Unix())
	runs := make([]model.SupplierUsageDailyBatchRun, 0, 200)
	for offset := 0; offset < 200; offset++ {
		day := firstDay.AddDate(0, 0, offset)
		completedAt := day.AddDate(0, 0, 1).Unix()
		runs = append(runs, supplierReportPublishedRun(t, day.Format("2006-01-02"), day.Unix(), completedAt, 1, 0))
	}
	require.NoError(t, mainDB.CreateInBatches(runs, 100).Error)

	var invokedDates []string
	runner := func(ctx context.Context, mainDB, logDB *gorm.DB, batchDate, owner string, now time.Time, force bool) error {
		invokedDates = append(invokedDates, batchDate)
		return RunSupplierDailyBatch(ctx, mainDB, logDB, batchDate, owner, now, force)
	}
	result, err := catchUpSupplierDailyBatches(context.Background(), mainDB, logDB, "one-shot", today, runner)
	require.NoError(t, err)
	require.Equal(t, SupplierDailyBatchCatchUpResult{ProcessedDays: 1}, result)
	require.Equal(t, []string{today.AddDate(0, 0, -1).Format("2006-01-02")}, invokedDates, "selection must start at latest completed + 1")
	var count int64
	require.NoError(t, mainDB.Model(&model.SupplierUsageDailyBatchRun{}).Count(&count).Error)
	require.EqualValues(t, 201, count, "only D-1 after the latest completed date should be acquired")
	var historicalChanged int64
	require.NoError(t, mainDB.Model(&model.SupplierUsageDailyBatchRun{}).Where("batch_date < ? AND fence_token <> published_fence_token", today.AddDate(0, 0, -1).Format("2006-01-02")).Count(&historicalChanged).Error)
	require.Zero(t, historicalChanged, "completed history must not be reacquired")
}

func TestCatchUpSupplierDailyBatchesDoesNotWalkCompletedHistoryAfterFailedDate(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	today := time.Date(2026, 7, 23, 3, 0, 0, 0, location)
	firstDay := today.AddDate(0, 0, -201)
	activateSupplierAccountingForBatch(t, mainDB, firstDay.Unix())
	runs := []model.SupplierUsageDailyBatchRun{{
		BatchDate: firstDay.Format("2006-01-02"), DayStart: firstDay.Unix(), DayEnd: firstDay.AddDate(0, 0, 1).Unix(),
		Status: model.SupplierDailyBatchStatusFailed,
	}}
	for offset := 1; offset <= 200; offset++ {
		day := firstDay.AddDate(0, 0, offset)
		completedAt := day.AddDate(0, 0, 1).Unix()
		runs = append(runs, supplierReportPublishedRun(t, day.Format("2006-01-02"), day.Unix(), completedAt, 1, 0))
	}
	require.NoError(t, mainDB.CreateInBatches(runs, 100).Error)

	var invokedDates []string
	runner := func(ctx context.Context, mainDB, logDB *gorm.DB, batchDate, owner string, now time.Time, force bool) error {
		invokedDates = append(invokedDates, batchDate)
		return RunSupplierDailyBatch(ctx, mainDB, logDB, batchDate, owner, now, force)
	}
	result, err := catchUpSupplierDailyBatches(context.Background(), mainDB, logDB, "one-shot", today, runner)
	require.NoError(t, err)
	require.Equal(t, SupplierDailyBatchCatchUpResult{ProcessedDays: 1}, result)
	require.Equal(t, []string{firstDay.Format("2006-01-02")}, invokedDates, "selection must jump over completed history after repairing the failed date")
	var completedReacquired int64
	require.NoError(t, mainDB.Model(&model.SupplierUsageDailyBatchRun{}).
		Where("batch_date > ? AND fence_token <> published_fence_token", firstDay.Format("2006-01-02")).Count(&completedReacquired).Error)
	require.Zero(t, completedReacquired)
	var repaired model.SupplierUsageDailyBatchRun
	require.NoError(t, mainDB.Where("batch_date = ?", firstDay.Format("2006-01-02")).First(&repaired).Error)
	require.Equal(t, model.SupplierDailyBatchStatusCompleted, repaired.Status)
	require.EqualValues(t, 1, repaired.FenceToken)
}

func assertSupplierStaleLeaseCannotMutateNewOwner(t *testing.T, db *gorm.DB, batchDate string) {
	t.Helper()
	ctx := context.Background()
	day, err := time.Parse("2006-01-02", batchDate)
	require.NoError(t, err)
	require.NoError(t, db.Where("batch_date = ?", batchDate).Delete(&model.SupplierUsageDailySummary{}).Error)
	require.NoError(t, db.Where("batch_date = ?", batchDate).Delete(&model.SupplierUsageDailyBatchRun{}).Error)

	leaseA, err := model.AcquireSupplierDailyBatch(ctx, db, batchDate, day.Unix(), day.AddDate(0, 0, 1).Unix(), "node-a", time.Time{}, time.Minute, false)
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.SupplierUsageDailyBatchRun{}).Where("id = ?", leaseA.RunId).Update("locked_until", 0).Error)
	leaseB, err := model.AcquireSupplierDailyBatch(ctx, db, batchDate, day.Unix(), day.AddDate(0, 0, 1).Unix(), "node-b", time.Time{}, time.Minute, false)
	require.NoError(t, err)
	require.Greater(t, leaseB.FenceToken, leaseA.FenceToken)

	sentinel := supplierFencingSummary(batchDate, "node-b-sentinel", 11)
	sentinel.BatchFenceToken = leaseB.FenceToken
	require.NoError(t, db.Create(&sentinel).Error)
	staleReplacement := supplierFencingSummary(batchDate, "stale-a", 12)
	require.ErrorIs(t, model.PersistSupplierDailyBatchPage(ctx, db, leaseA, []model.SupplierUsageDailySummary{staleReplacement}, 100, 1, 1, 1, time.Minute), model.ErrSupplierDailyBatchFenceLost)
	require.ErrorIs(t, model.FailSupplierDailyBatch(ctx, db, leaseA, errors.New("stale owner failure")), model.ErrSupplierDailyBatchFenceLost)
	var summaries []model.SupplierUsageDailySummary
	require.NoError(t, db.Where("batch_date = ?", batchDate).Find(&summaries).Error)
	require.Len(t, summaries, 1)
	require.Equal(t, sentinel.DimensionKey, summaries[0].DimensionKey)

	require.Error(t, model.PersistSupplierDailyBatchPage(ctx, db, leaseB, nil, 100, 1, model.SupplierDailyLogPageSize+1, 0, time.Minute))
	require.NoError(t, db.Where("batch_date = ?", batchDate).Find(&summaries).Error)
	require.Len(t, summaries, 1, "invalid page must not mutate the current generation")
	require.Equal(t, sentinel.DimensionKey, summaries[0].DimensionKey)
	var run model.SupplierUsageDailyBatchRun
	require.NoError(t, db.First(&run, leaseB.RunId).Error)
	require.Equal(t, model.SupplierDailyBatchStatusRunning, run.Status)
	require.Equal(t, leaseB.Owner, run.LeaseOwner)
	require.Equal(t, leaseB.FenceToken, run.FenceToken)

	winner := supplierFencingSummary(batchDate, "node-b-winner", 31)
	require.NoError(t, model.PersistSupplierDailyBatchPage(ctx, db, leaseB, []model.SupplierUsageDailySummary{winner}, 100, 1, 3, 2, time.Minute))
	leaseB.CursorCreatedAt, leaseB.CursorId = 100, 1
	require.NoError(t, model.PublishSupplierDailyBatch(ctx, db, leaseB, time.Now(), types.SupplierPublishedEvidenceV1{
		SchemaVersion:                    types.SupplierPublishedEvidenceSchemaVersion,
		LogsScanned:                      3,
		ProducerMarkersPresent:           2,
		CapturedSnapshotCount:            2,
		DispositionCounts:                types.SupplierPublishedDispositionCountsV1{Captured: 2},
		FailureCounts:                    types.SupplierPublishedFailureCountsV1{AbsentMarkerAfterCutover: 1},
		PersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessIncomplete,
		Warnings: []types.SupplierPublishedWarningV1{{
			Code: types.SupplierPublishedWarningAbsentMarker, Count: 1, MessageKey: "supply_chain.warning.absent_marker_after_cutover",
		}},
	}))
	require.NoError(t, db.Where("batch_date = ?", batchDate).Find(&summaries).Error)
	require.Len(t, summaries, 2)
	dimensionKeys := []string{summaries[0].DimensionKey, summaries[1].DimensionKey}
	require.ElementsMatch(t, []string{sentinel.DimensionKey, winner.DimensionKey}, dimensionKeys)
	require.NoError(t, db.First(&run, leaseB.RunId).Error)
	require.Equal(t, model.SupplierDailyBatchStatusCompleted, run.Status)
	require.EqualValues(t, 2, run.SummaryCount)
	require.EqualValues(t, 3, run.LogsScanned)
	require.EqualValues(t, 2, run.SnapshotCount)
}

func supplierFencingSummary(batchDate, dimensionKey string, requestCount int64) model.SupplierUsageDailySummary {
	return model.SupplierUsageDailySummary{
		BatchDate: batchDate, DimensionKey: dimensionKey, BucketStart: 1,
		SupplierId: 1, ContractId: 2, RateVersionId: 3, ChannelId: 4,
		StatisticsScope: string(types.SupplierStatisticsScopeBusiness), DataQuality: SupplierDataQualityAuthoritative,
		RequestCount: requestCount,
	}
}

const mathMaxUnixForTest = int64(1<<62 - 1)

func TestScanSupplierAccountingLogsUsesConsumeTypeAndCreatedAtIDKeyset(t *testing.T) {
	db := supplierDailyTestDB(t, t.Name())
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	for index := range 7 {
		require.NoError(t, db.Create(&model.Log{Type: model.LogTypeConsume, CreatedAt: 100, Other: fmt.Sprintf(`{"i":%d}`, index)}).Error)
	}
	require.NoError(t, db.Create(&model.Log{Type: model.LogTypeError, CreatedAt: 100}).Error)
	var ids []int
	scanned, err := model.ScanSupplierAccountingLogs(context.Background(), db, 99, 101, 2, func(rows []model.SupplierAccountingLogRow) error {
		for _, row := range rows {
			ids = append(ids, row.Id)
		}
		return nil
	})
	require.NoError(t, err)
	require.EqualValues(t, 7, scanned)
	require.Len(t, ids, 7)
	for index := 1; index < len(ids); index++ {
		require.Greater(t, ids[index], ids[index-1])
	}
}
