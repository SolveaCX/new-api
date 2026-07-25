package service

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func supplierHistoricalServiceDBs(t *testing.T) (*gorm.DB, *gorm.DB) {
	t.Helper()
	mainDB, err := gorm.Open(sqlite.Open("file:"+t.Name()+"-main?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	logDB, err := gorm.Open(sqlite.Open("file:"+t.Name()+"-log?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, mainDB.AutoMigrate(
		&model.UpstreamSupplier{}, &model.SupplierContract{}, &model.SupplierContractRateVersion{},
		&model.SupplierHistoricalImport{}, &model.SupplierHistoricalDailySummary{},
	))
	require.NoError(t, logDB.AutoMigrate(&model.Log{}))
	return mainDB, logDB
}

func supplierHistoricalMappingFixture(t *testing.T, db *gorm.DB) SupplierHistoricalChannelMapping {
	t.Helper()
	supplier := model.UpstreamSupplier{Name: "historical supplier"}
	require.NoError(t, db.Create(&supplier).Error)
	contract := model.SupplierContract{SupplierId: supplier.Id, Name: "historical contract", ContractNo: "H-1"}
	require.NoError(t, db.Create(&contract).Error)
	rate := model.SupplierContractRateVersion{ContractId: contract.Id, ProcurementMultiplierPpm: 650_000, CreatedBy: 7}
	require.NoError(t, db.Create(&rate).Error)
	return SupplierHistoricalChannelMapping{ChannelId: 11, SupplierId: supplier.Id, ContractId: contract.Id, RateVersionId: rate.Id, ProcurementMultiplierPpm: 650_000}
}

func TestSupplierHistoricalImportCanonicalIdempotencyAndValidation(t *testing.T) {
	mainDB, _ := supplierHistoricalServiceDBs(t)
	mapping := supplierHistoricalMappingFixture(t, mainDB)
	command := SupplierHistoricalImportCommand{
		StartDate: "2026-01-01", EndDate: "2026-01-03", QuotaPerUnit: "500000.0",
		ExcludedUserIds: []int{9, 1, 9}, ChannelMappings: []SupplierHistoricalChannelMapping{mapping}, Reason: "legacy estimate",
	}
	first, err := CreateSupplierHistoricalEstimate(context.Background(), mainDB, command, 7, "stable-key")
	require.NoError(t, err)
	command.QuotaPerUnit = "500000"
	command.ExcludedUserIds = []int{1, 9}
	replayed, err := CreateSupplierHistoricalEstimate(context.Background(), mainDB, command, 7, "stable-key")
	require.NoError(t, err)
	require.Equal(t, first.Id, replayed.Id)

	command.Reason = "different"
	_, err = CreateSupplierHistoricalEstimate(context.Background(), mainDB, command, 7, "stable-key")
	require.ErrorIs(t, err, model.ErrSupplierHistoricalImportIdempotencyConflict)

	command.ChannelMappings[0].ProcurementMultiplierPpm = 700_000
	_, err = CreateSupplierHistoricalEstimate(context.Background(), mainDB, command, 7, "mismatch")
	require.ErrorIs(t, err, ErrSupplierHistoricalMappingInvalid)
}

func TestSupplierHistoricalSchedulerTreatsQuarantinedOnlyConflictAsNoWork(t *testing.T) {
	mainDB, logDB := supplierHistoricalServiceDBs(t)
	completed := model.SupplierHistoricalImport{
		CommandHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CommandJSON: `{}`,
		IdempotencyKey: "completed-only", CreatedBy: 7, Method: model.SupplierHistoricalMethodLogEstimateV1, Reason: "completed",
		StartDate: "2026-01-01", EndDate: "2026-02-01", DayStart: 1767196800, DayEnd: 1769875200,
		QuotaPerUnit: "500000", ExcludedUserIdsJSON: `[]`, ChannelMappingsJSON: `[]`, Status: model.SupplierHistoricalImportStatusCompleted,
	}
	require.NoError(t, mainDB.Create(&completed).Error)
	conflicting := model.SupplierHistoricalImport{
		CommandHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", CommandJSON: `{}`,
		IdempotencyKey: "conflict-only", CreatedBy: 7, Method: model.SupplierHistoricalMethodLogEstimateV1, Reason: "conflict",
		StartDate: "2026-01-15", EndDate: "2026-01-20", DayStart: 1768406400, DayEnd: 1768838400,
		QuotaPerUnit: "500000", ExcludedUserIdsJSON: `[]`, ChannelMappingsJSON: `[]`, Status: model.SupplierHistoricalImportStatusPending,
	}
	require.NoError(t, mainDB.Create(&conflicting).Error)

	result, err := RunSupplierHistoricalEstimatePage(context.Background(), mainDB, logDB, "master-1", time.Minute)
	require.NoError(t, err)
	require.True(t, result.NoWork)

	stored, err := model.GetSupplierHistoricalImport(context.Background(), mainDB, conflicting.Id)
	require.NoError(t, err)
	require.Equal(t, model.SupplierHistoricalImportStatusFailed, stored.Status)
	require.Contains(t, stored.ErrorMessage, "overlap")
}

func TestSupplierHistoricalImportProcessesEstimatedKnownUnknownAndUnassigned(t *testing.T) {
	mainDB, logDB := supplierHistoricalServiceDBs(t)
	mapping := supplierHistoricalMappingFixture(t, mainDB)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, location)
	logs := []model.Log{
		{UserId: 10, CreatedAt: day.Add(time.Hour).Unix(), Type: model.LogTypeConsume, ChannelId: 11, ModelName: "claude", Quota: 350_000, Other: `{"group_ratio":0.7}`},
		{UserId: 1, CreatedAt: day.Add(2 * time.Hour).Unix(), Type: model.LogTypeConsume, ChannelId: 11, ModelName: "internal-a", Quota: 350_000, Other: `{"group_ratio":0.7}`},
		{UserId: 1, CreatedAt: day.Add(3 * time.Hour).Unix(), Type: model.LogTypeConsume, ChannelId: 11, ModelName: "internal-b", Quota: 350_000, Other: `{"group_ratio":0.7}`},
		{UserId: 10, CreatedAt: day.Add(4 * time.Hour).Unix(), Type: model.LogTypeConsume, ChannelId: 11, ModelName: "unknown", Quota: 350_000, Other: `{}`},
		{UserId: 10, CreatedAt: day.Add(5 * time.Hour).Unix(), Type: model.LogTypeConsume, ChannelId: 99, ModelName: "unassigned", Quota: 350_000, Other: `{"group_ratio":0.7}`},
	}
	require.NoError(t, logDB.Create(&logs).Error)
	item, err := CreateSupplierHistoricalEstimate(context.Background(), mainDB, SupplierHistoricalImportCommand{
		StartDate: "2026-01-01", EndDate: "2026-01-02", QuotaPerUnit: "500000",
		ExcludedUserIds: []int{1}, ChannelMappings: []SupplierHistoricalChannelMapping{mapping}, Reason: "legacy estimate",
	}, 7, "run")
	require.NoError(t, err)

	result, err := RunSupplierHistoricalEstimatePage(context.Background(), mainDB, logDB, "master-1", time.Minute)
	require.NoError(t, err)
	require.Equal(t, item.Id, result.ImportId)
	require.True(t, result.Completed)

	stored, err := model.GetSupplierHistoricalImport(context.Background(), mainDB, item.Id)
	require.NoError(t, err)
	require.Equal(t, model.SupplierHistoricalImportStatusCompleted, stored.Status)
	require.EqualValues(t, 5, stored.CandidateCount)
	require.EqualValues(t, 5, stored.ProcessedCount)

	summaries, err := model.ListSupplierHistoricalDailySummaries(context.Background(), mainDB, item.Id)
	require.NoError(t, err)
	require.Len(t, summaries, 4, "internal rows differing only by model must coalesce")
	var business, internal, unknown, unassigned *model.SupplierHistoricalDailySummary
	for index := range summaries {
		summary := &summaries[index]
		switch {
		case summary.StatisticsScope == "internal":
			internal = summary
		case summary.UnassignedRequestCount > 0:
			unassigned = summary
		case summary.ModelName == "unknown":
			unknown = summary
		default:
			business = summary
		}
	}
	require.NotNil(t, business)
	require.EqualValues(t, 700_000, business.SalesMicroUsd)
	require.EqualValues(t, 1_000_000, business.OfficialListMicroUsd)
	require.EqualValues(t, 650_000, business.ProcurementCostMicroUsd)
	require.EqualValues(t, 50_000, business.GrossProfitMicroUsd)
	require.NotNil(t, internal)
	require.EqualValues(t, 2, internal.SourceRequestCount)
	require.Zero(t, internal.SalesKnownCount)
	require.EqualValues(t, 2, internal.SalesUnknownCount)
	require.EqualValues(t, 2_000_000, internal.OfficialListMicroUsd)
	require.EqualValues(t, 1_300_000, internal.ProcurementCostMicroUsd)
	require.Empty(t, internal.ModelName)
	require.Zero(t, internal.ChannelId)
	require.NotNil(t, unknown)
	require.EqualValues(t, 1, unknown.SalesKnownCount)
	require.EqualValues(t, 1, unknown.OfficialListUnknownCount)
	require.EqualValues(t, 1, unknown.ProcurementCostUnknownCount)
	require.EqualValues(t, 1, unknown.GrossProfitUnknownCount)
	require.NotNil(t, unassigned)
	require.EqualValues(t, 1, unassigned.UnassignedRequestCount)
	require.EqualValues(t, 1, unassigned.OfficialListKnownCount)
	require.EqualValues(t, 1, unassigned.ProcurementCostUnknownCount)
}

func TestSupplierHistoricalImportResumesFromFrozenCursor(t *testing.T) {
	mainDB, logDB := supplierHistoricalServiceDBs(t)
	mapping := supplierHistoricalMappingFixture(t, mainDB)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, location)
	logs := make([]model.Log, SupplierHistoricalPageSize+1)
	for index := range logs {
		logs[index] = model.Log{UserId: 10, CreatedAt: day.Add(time.Duration(index) * time.Second).Unix(), Type: model.LogTypeConsume, ChannelId: 11, ModelName: "gpt", Quota: 500_000, Other: `{"group_ratio":1}`}
	}
	require.NoError(t, logDB.CreateInBatches(&logs, 500).Error)
	item, err := CreateSupplierHistoricalEstimate(context.Background(), mainDB, SupplierHistoricalImportCommand{
		StartDate: "2026-01-01", EndDate: "2026-01-02", QuotaPerUnit: "500000", ChannelMappings: []SupplierHistoricalChannelMapping{mapping}, Reason: "resume",
	}, 7, "resume")
	require.NoError(t, err)
	first, err := RunSupplierHistoricalEstimatePage(context.Background(), mainDB, logDB, "master-1", time.Minute)
	require.NoError(t, err)
	require.False(t, first.Completed)
	state, err := model.GetSupplierHistoricalImport(context.Background(), mainDB, item.Id)
	require.NoError(t, err)
	require.EqualValues(t, SupplierHistoricalPageSize, state.ProcessedCount)

	second, err := RunSupplierHistoricalEstimatePage(context.Background(), mainDB, logDB, "master-1", time.Minute)
	require.NoError(t, err)
	require.True(t, second.Completed)
	state, err = model.GetSupplierHistoricalImport(context.Background(), mainDB, item.Id)
	require.NoError(t, err)
	require.EqualValues(t, SupplierHistoricalPageSize+1, state.ProcessedCount)
}

func TestSupplierHistoricalEstimateUsesUnroundedOfficialBaseAndCheckedMoney(t *testing.T) {
	command := SupplierHistoricalImportCommand{QuotaPerUnit: "6", ChannelMappings: []SupplierHistoricalChannelMapping{{ChannelId: 1, SupplierId: 1, ContractId: 1, RateVersionId: 1, ProcurementMultiplierPpm: 650_000}}}
	rows := []model.SupplierHistoricalSourceLog{{Id: 1, UserId: 2, CreatedAt: 1767196800, ChannelId: 1, ModelName: "rounding", Quota: 1, Other: `{"group_ratio":0.7}`}}
	summaries, err := aggregateSupplierHistoricalPage(1, rows, command)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	// 1 / 6 / 0.7 * 1e6 = 238095.238...; official is rounded once
	// from that rational, never from rounded sales (166667 / 0.7 = 238095.714...).
	require.EqualValues(t, 238_095, summaries[0].OfficialListMicroUsd)

	overflowRows := []model.SupplierHistoricalSourceLog{{Id: 2, UserId: 2, CreatedAt: 1767196800, ChannelId: 1, ModelName: "overflow", Quota: math.MaxInt64, Other: `{"group_ratio":1}`}}
	_, err = aggregateSupplierHistoricalPage(1, overflowRows, SupplierHistoricalImportCommand{QuotaPerUnit: "0.000001", ChannelMappings: command.ChannelMappings})
	require.ErrorIs(t, err, ErrSupplierHistoricalMoneyOverflow)
}

func TestSupplierHistoricalSeriesServiceRequiresCompletedPublication(t *testing.T) {
	mainDB, _ := supplierHistoricalServiceDBs(t)
	item := model.SupplierHistoricalImport{
		CommandHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CommandJSON: `{}`,
		IdempotencyKey: "series-gate", CreatedBy: 7, Method: model.SupplierHistoricalMethodLogEstimateV1, Reason: "series",
		StartDate: "2026-01-01", EndDate: "2026-01-02", DayStart: 1767196800, DayEnd: 1767283200,
		QuotaPerUnit: "500000", ExcludedUserIdsJSON: `[]`, ChannelMappingsJSON: `[]`, Status: model.SupplierHistoricalImportStatusPending,
	}
	require.NoError(t, mainDB.Create(&item).Error)
	for _, status := range []string{model.SupplierHistoricalImportStatusPending, model.SupplierHistoricalImportStatusRunning, model.SupplierHistoricalImportStatusFailed} {
		require.NoError(t, mainDB.Model(&model.SupplierHistoricalImport{}).Where("id = ?", item.Id).UpdateColumn("status", status).Error)
		_, _, err := ListCompletedSupplierHistoricalSeries(context.Background(), mainDB, item.Id, item.StartDate, item.EndDate, model.SupplierHistoricalSeriesCursor{}, 100)
		require.ErrorIs(t, err, ErrSupplierHistoricalImportNotReady, status)
	}
}
