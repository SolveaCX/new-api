package service

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSupplierAccountingCrossDBIntegration(t *testing.T) {
	testCases := []struct {
		name     string
		dialect  string
		dsnEnv   string
		database string
		open     func(string) gorm.Dialector
	}{
		{name: "mysql", dialect: "mysql", dsnEnv: "TEST_MYSQL_DSN", database: "supplier_g009_mysql", open: func(dsn string) gorm.Dialector { return mysql.Open(dsn) }},
		{name: "postgres", dialect: "postgres", dsnEnv: "TEST_POSTGRES_DSN", database: "supplier_g009_postgres", open: func(dsn string) gorm.Dialector { return postgres.Open(dsn) }},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(testCase.dsnEnv))
			if dsn == "" {
				t.Skipf("set %s to run the isolated %s integration gate", testCase.dsnEnv, testCase.name)
			}
			db, err := gorm.Open(testCase.open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			requireIsolatedSupplierDatabase(t, db, testCase.dialect, testCase.database)
			runSupplierAccountingCrossDBGate(t, db, testCase.dialect)
		})
	}
}

func requireIsolatedSupplierDatabase(t *testing.T, db *gorm.DB, dialect, expected string) {
	t.Helper()
	var current string
	query := "SELECT DATABASE()"
	if dialect == "postgres" {
		query = "SELECT current_database()"
	}
	require.NoError(t, db.Raw(query).Scan(&current).Error)
	require.Equal(t, expected, current, "integration test refuses to mutate a database without the isolated G009 name")
}

func runSupplierAccountingCrossDBGate(t *testing.T, db *gorm.DB, dialect string) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	columnsBefore, indexesBefore := crossDBLogSchema(t, db, dialect)

	supplierTables := []any{
		&model.UpstreamSupplier{},
		&model.SupplierContract{},
		&model.SupplierContractRateVersion{},
		&model.SupplierChannelBindingVersion{},
		&model.SupplierInventoryAdjustment{},
		&model.SupplierStatisticsExclusionRule{},
		&model.SupplierAdminCommand{},
		&model.SupplierAccountingCoverageGap{},
		&model.SupplierUsageDailySummary{},
		&model.SupplierUsageDailyBatchRun{},
	}
	require.Len(t, supplierTables, 10)
	require.NoError(t, db.AutoMigrate(append([]any{&model.Option{}, &model.Channel{}}, supplierTables...)...))
	require.NoError(t, model.EnsureSupplierUsageGenerationSchema(db))
	for _, table := range supplierTables {
		require.True(t, db.Migrator().HasTable(table))
	}
	columnsAfter, indexesAfter := crossDBLogSchema(t, db, dialect)
	require.Equal(t, columnsBefore, columnsAfter, "supplier migration must not alter logs columns")
	require.Equal(t, indexesBefore, indexesAfter, "supplier migration must not alter logs indexes")
	t.Logf("%s migration: supplier_tables=10 logs_columns=%d logs_indexes=%d unchanged=true", dialect, len(columnsAfter), len(indexesAfter))
	cleanupSupplierAccountingCrossDBGate(t, db)
	t.Cleanup(func() { cleanupSupplierAccountingCrossDBGate(t, db) })

	beforeDBTime := crossDBUnix(t, db, dialect)
	leaseDay := time.Date(1999, time.January, 1, 0, 0, 0, 0, time.UTC)
	lease, err := model.AcquireSupplierDailyBatch(ctx, db, "1999-01-01", leaseDay.Unix(), leaseDay.Add(24*time.Hour).Unix(), "db-time-owner", time.Unix(1, 0), time.Minute, false)
	require.NoError(t, err)
	require.False(t, lease.AlreadyDone)
	var leaseRun model.SupplierUsageDailyBatchRun
	require.NoError(t, db.First(&leaseRun, lease.RunId).Error)
	afterDBTime := crossDBUnix(t, db, dialect)
	require.GreaterOrEqual(t, leaseRun.StartedAt, beforeDBTime)
	require.LessOrEqual(t, leaseRun.StartedAt, afterDBTime)
	require.GreaterOrEqual(t, leaseRun.LockedUntil, leaseRun.StartedAt+59)
	_, err = model.AcquireSupplierDailyBatch(ctx, db, "1999-01-01", leaseDay.Unix(), leaseDay.Add(24*time.Hour).Unix(), "second-node", time.Now().Add(100*365*24*time.Hour), time.Minute, false)
	require.ErrorIs(t, err, model.ErrSupplierDailyBatchBusy)
	require.NoError(t, model.FailSupplierDailyBatch(ctx, db, lease, fmt.Errorf("integration lease cleanup")))
	t.Logf("%s DB-time lease: started_at=%d db_window=[%d,%d] second_owner_busy=true", dialect, leaseRun.StartedAt, beforeDBTime, afterDBTime)
	assertSupplierStaleLeaseCannotMutateNewOwner(t, db, "1999-01-02")
	t.Logf("%s destructive fencing: stale_complete_rejected=true stale_fail_rejected=true winner_atomic=true", dialect)

	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := beginningOfSupplierDay(time.Now().In(location)).AddDate(0, 0, -2)
	startAt, endAt := day.Unix(), day.AddDate(0, 0, 1).Unix()
	persistLegacySupplierAccountingCoverageStart(t, db, startAt)
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", fmt.Sprintf("%d", startAt+1))
	coverageStartAt, err := InitializeSupplierAccountingCoverageStart(ctx, db)
	require.NoError(t, err)
	require.Equal(t, startAt, coverageStartAt, "legacy initializer is read-only and ignores assertion env")
	createdIDs := insertCrossDBKeysetRows(t, db, startAt)
	var scannedIDs []int
	var pageSizes []int
	require.Equal(t, 5000, model.SupplierDailyLogPageSize)
	scanned, err := model.ScanSupplierAccountingLogs(ctx, db, startAt, endAt, model.SupplierDailyLogPageSize, func(rows []model.SupplierAccountingLogRow) error {
		pageSizes = append(pageSizes, len(rows))
		for _, row := range rows {
			scannedIDs = append(scannedIDs, row.Id)
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, int64(len(createdIDs)), scanned)
	require.Equal(t, []int{model.SupplierDailyLogPageSize, 2}, pageSizes)
	require.Equal(t, createdIDs, scannedIDs, "consume-only keyset must preserve created_at,id order without gaps or duplicates")
	t.Logf("%s consume keyset: rows=%d page_sizes=%v ordered_without_gaps=true", dialect, scanned, pageSizes)

	salesMultiplier, officialList, sales, procurement, profit := int64(700_000), int64(1_000_000), int64(700_000), int64(650_000), int64(50_000)
	snapshot := types.SupplierAccountingLogSnapshotV1{
		BindingVersionId: 4, SupplierId: 1, ContractId: 2, RateVersionId: 3,
		ProcurementMultiplierPpm: 650_000, SalesMultiplierPpm: &salesMultiplier,
		OfficialListMicroUsd: &officialList, SalesMicroUsd: &sales,
		ProcurementCostMicroUsd: &procurement, GrossProfitMicroUsd: &profit,
		StatisticsScope: string(types.SupplierStatisticsScopeBusiness), ExclusionDecision: "included",
		FinanciallyCommittedAt: startAt + 3,
		PricingProvenance: &types.SupplierPricingProvenanceV1{Ratio: &types.SupplierRatioPricingProvenanceV1{
			ModelRatioPpm: 1_000_000, GroupRatioPpm: salesMultiplier, ModelRatioVersion: 1, GroupRatioVersion: 1,
		}},
	}
	require.NoError(t, db.Create(&model.Log{Type: model.LogTypeConsume, CreatedAt: startAt + 3, ChannelId: 7, ModelName: "cross-db-model", Other: supplierDailyLogOther(t, snapshot)}).Error)
	stableCoverageStart, err := model.SupplierAccountingCoverageStart(ctx, db)
	require.NoError(t, err)
	require.Equal(t, startAt, stableCoverageStart, "coverage cutover must remain first-writer-wins")
	activateSupplierAccountingForBatch(t, db, startAt)
	require.NoError(t, RunSupplierDailyBatch(ctx, db, db, day.Format("2006-01-02"), dialect+"-summary-owner", day.AddDate(0, 0, 2), false))

	var summary model.SupplierUsageDailySummary
	require.NoError(t, db.Where("batch_date = ?", day.Format("2006-01-02")).First(&summary).Error)
	require.Equal(t, int64(1), summary.RequestCount)
	require.Equal(t, officialList, summary.OfficialListMicroUsd)
	require.Equal(t, sales, summary.SalesMicroUsd)
	require.Equal(t, procurement, summary.ProcurementCostMicroUsd)
	require.Equal(t, profit, summary.GrossProfitMicroUsd)
	require.NotNil(t, summary.SalesMultiplierPpm)
	require.Equal(t, salesMultiplier, *summary.SalesMultiplierPpm)
	t.Logf("%s T+1 summary: requests=%d official=%d sales=%d procurement=%d profit=%d sales_multiplier_ppm=%d", dialect, summary.RequestCount, summary.OfficialListMicroUsd, summary.SalesMicroUsd, summary.ProcurementCostMicroUsd, summary.GrossProfitMicroUsd, *summary.SalesMultiplierPpm)

	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&model.UpstreamSupplier{
		Id: 1, Name: dialect + " report supplier", Status: model.SupplierStatusActive,
	}).Error)
	futureRateID := 30
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&[]model.SupplierContract{
		{Id: 2, SupplierId: 1, Name: "historical contract", ContractNo: dialect + "-historical", Status: model.SupplierContractStatusActive, CurrentRateVersionId: &futureRateID},
		{Id: 20, SupplierId: 1, Name: "current contract", ContractNo: dialect + "-current", Status: model.SupplierContractStatusActive},
	}).Error)
	reportNow := day.AddDate(0, 0, 2).Add(3 * time.Hour)
	openDay := beginningOfSupplierDay(reportNow.In(location))
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&[]model.SupplierContractRateVersion{
		{Id: 3, ContractId: 2, ProcurementMultiplierPpm: 650_000, EffectiveAt: startAt + 2, CreatedBy: 1, CreatedAt: startAt + 2},
		{Id: futureRateID, ContractId: 2, ProcurementMultiplierPpm: 900_000, EffectiveAt: openDay.Unix() + 2, CreatedBy: 1, CreatedAt: openDay.Unix() + 2},
	}).Error)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&[]model.SupplierInventoryAdjustment{
		{ContractId: 2, DeltaMicroUsd: 5_000_000, Type: model.SupplierInventoryAdjustmentTypeInitial, IdempotencyKey: dialect + "-closed-inventory", CreatedBy: 1, CreatedAt: startAt + 2},
		{ContractId: 2, DeltaMicroUsd: 9_000_000, Type: model.SupplierInventoryAdjustmentTypeReplenishment, IdempotencyKey: dialect + "-open-inventory", CreatedBy: 1, CreatedAt: openDay.Unix() + 2},
	}).Error)
	currentContractId := 20
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&[]model.Channel{
		{Id: 7, Name: dialect + " rebound channel", Status: common.ChannelStatusEnabled, SupplierContractId: &currentContractId},
		{Id: 8, Name: dialect + " future history only", Status: common.ChannelStatusEnabled},
	}).Error)
	const futureFence = int64(9007)
	openDayEnd := openDay.AddDate(0, 0, 1)
	require.NoError(t, db.Create(&model.SupplierUsageDailySummary{
		BatchDate: openDay.Format("2006-01-02"), BatchFenceToken: futureFence, DimensionKey: dialect + "-open-day",
		BucketStart: openDay.Unix(), SupplierId: 1, ContractId: 2, RateVersionId: futureRateID, ChannelId: 8,
		ModelName: "open-day-model", PricingMode: "ratio", StatisticsScope: "business", DataQuality: "authoritative",
		RequestCount: 9, OfficialListKnownCount: 9, OfficialListMicroUsd: 9_000_000,
	}).Error)
	futureRun := supplierReportPublishedRun(t, openDay.Format("2006-01-02"), openDay.Unix(), openDayEnd.Unix(), futureFence, 9)
	require.NoError(t, db.Create(&futureRun).Error)

	reportQuery := SupplierReportQuery{StartDate: day.Format("2006-01-02"), EndDate: day.Format("2006-01-02")}
	reports := NewSupplierReportService(model.NewSupplierReportStore(db))
	reports.now = func() time.Time { return reportNow }
	reportFilter := model.SupplierReportFilter{StartAt: startAt, EndAt: endAt, ContractIds: []int{2, 20}}
	reportStore := model.NewSupplierReportStore(db)
	catalog, _, err := reportStore.ListContractCatalog(ctx, reportFilter, nil)
	require.NoError(t, err)
	require.Len(t, catalog, 2)
	require.NotNil(t, catalog[0].CurrentRateVersionId)
	require.Equal(t, 3, *catalog[0].CurrentRateVersionId, "correlated catalog join must select the latest rate before the closed end")
	rates, err := reportStore.ListRateVersions(ctx, 2, endAt)
	require.NoError(t, err)
	require.Len(t, rates, 1)
	require.Equal(t, 3, rates[0].Id)
	adjustments, err := reportStore.ListInventoryAdjustments(ctx, []int{2}, endAt)
	require.NoError(t, err)
	require.Len(t, adjustments, 1)
	require.Equal(t, int64(5_000_000), adjustments[0].DeltaMicroUsd)
	consumption, err := reportStore.QueryInventoryConsumption(ctx, []int{2}, endAt)
	require.NoError(t, err)
	require.Len(t, consumption, 1)
	require.Equal(t, officialList, consumption[0].InventoryAffectingOfficialListMicroUsd)
	catalogChannels, _, err := reportStore.ListChannelCatalog(ctx, reportFilter, nil)
	require.NoError(t, err)
	require.Len(t, catalogChannels, 2, "closed history plus current rebind must exclude the published open-day channel history")
	for _, row := range catalogChannels {
		require.NotEqual(t, 8, row.ChannelId)
	}
	t.Logf("%s closed-day store cutoff: eligible_rate=3 rate_rows=1 adjustment_rows=1 consumption=%d channel_rows=%d", dialect, consumption[0].InventoryAffectingOfficialListMicroUsd, len(catalogChannels))
	contractReport, err := reports.ListContracts(ctx, SupplierReportQuery{
		StartDate: reportQuery.StartDate, EndDate: reportQuery.EndDate, ChannelIds: []int{7},
	}, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Len(t, contractReport.Items, 1)
	require.Equal(t, 2, contractReport.Items[0].ContractId, "channel filtering must use the historical daily-summary contract")
	require.NotNil(t, contractReport.Items[0].CurrentRateVersionId)
	require.Equal(t, 3, *contractReport.Items[0].CurrentRateVersionId)
	require.Equal(t, int64(5_000_000), contractReport.Items[0].TotalInventoryMicroUsd)
	require.Equal(t, officialList, contractReport.Items[0].OfficialListConsumedMicroUsd)

	channelReport, err := reports.ListChannels(ctx, reportQuery, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Len(t, channelReport.Items, 2)
	require.Equal(t, 2, channelReport.Items[0].ContractId)
	require.Equal(t, int64(1), channelReport.Items[0].Business.RequestCount)
	require.Equal(t, 20, channelReport.Items[1].ContractId)
	require.Zero(t, channelReport.Items[1].Business.RequestCount)

	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Model(&model.Channel{}).
		Where("id = ?", 7).UpdateColumn("supplier_contract_id", nil).Error)
	channelReport, err = reports.ListChannels(ctx, reportQuery, model.SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.Len(t, channelReport.Items, 1, "unbinding current state must retain historical ownership")
	require.Equal(t, 2, channelReport.Items[0].ContractId)
	require.Equal(t, 7, channelReport.Items[0].ChannelId)
	t.Logf("%s historical report ownership: rebound_rows=2 unbound_rows=1 historical_contract=2", dialect)
}

func cleanupSupplierAccountingCrossDBGate(t *testing.T, db *gorm.DB) {
	t.Helper()
	tables := []any{
		&model.SupplierUsageDailySummary{},
		&model.SupplierUsageDailyBatchRun{},
		&model.SupplierAccountingCoverageGap{},
		&model.SupplierAdminCommand{},
		&model.SupplierStatisticsExclusionRule{},
		&model.SupplierInventoryAdjustment{},
		&model.SupplierChannelBindingVersion{},
		&model.Channel{},
		&model.SupplierContractRateVersion{},
		&model.SupplierContract{},
		&model.UpstreamSupplier{},
		&model.Option{},
		&model.Log{},
	}
	for _, table := range tables {
		require.NoError(t, db.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(table).Error)
	}
}

func insertCrossDBKeysetRows(t *testing.T, db *gorm.DB, startAt int64) []int {
	t.Helper()
	rows := make([]*model.Log, 0, model.SupplierDailyLogPageSize+3)
	rows = append(rows, &model.Log{Type: model.LogTypeManage, CreatedAt: startAt + 1, ChannelId: 1, ModelName: "m", Other: "{}"})
	for index := 0; index < model.SupplierDailyLogPageSize+2; index++ {
		rows = append(rows, &model.Log{
			Type: model.LogTypeConsume, CreatedAt: startAt + 1 + int64(index/1000),
			ChannelId: 1, ModelName: "m", Other: "{}",
		})
	}
	require.NoError(t, db.CreateInBatches(rows, 1000).Error)
	consumeIDs := make([]int, 0, model.SupplierDailyLogPageSize+2)
	for _, row := range rows {
		if row.Type == model.LogTypeConsume {
			consumeIDs = append(consumeIDs, row.Id)
		}
	}
	return consumeIDs
}

func crossDBUnix(t *testing.T, db *gorm.DB, dialect string) int64 {
	t.Helper()
	var timestamp int64
	query := "SELECT UNIX_TIMESTAMP()"
	if dialect == "postgres" {
		query = "SELECT EXTRACT(EPOCH FROM NOW())::bigint"
	}
	require.NoError(t, db.Raw(query).Scan(&timestamp).Error)
	return timestamp
}

func crossDBLogSchema(t *testing.T, db *gorm.DB, dialect string) ([]string, []string) {
	t.Helper()
	var columns []string
	var indexes []string
	if dialect == "mysql" {
		require.NoError(t, db.Raw(`SELECT CONCAT_WS('|', column_name, column_type, is_nullable, COALESCE(column_default, '<NULL>'), extra)
			FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'logs' ORDER BY ordinal_position`).Scan(&columns).Error)
		require.NoError(t, db.Raw(`SELECT CONCAT_WS('|', index_name, non_unique, seq_in_index, column_name, COALESCE(collation, ''))
			FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'logs' ORDER BY index_name, seq_in_index`).Scan(&indexes).Error)
	} else {
		require.NoError(t, db.Raw(`SELECT concat_ws('|', column_name, data_type, udt_name, is_nullable, COALESCE(column_default, '<NULL>'))
			FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'logs' ORDER BY ordinal_position`).Scan(&columns).Error)
		require.NoError(t, db.Raw(`SELECT indexname || '|' || indexdef FROM pg_indexes WHERE schemaname = current_schema() AND tablename = 'logs' ORDER BY indexname`).Scan(&indexes).Error)
	}
	require.NotEmpty(t, columns)
	require.NotEmpty(t, indexes)
	sort.Strings(indexes)
	return columns, indexes
}
