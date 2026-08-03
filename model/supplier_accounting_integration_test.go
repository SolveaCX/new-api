package model

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const supplierAccountingIntegrationConfirmation = "isolated-empty-database"

type supplierAccountingIntegrationDatabase struct {
	name   string
	dsnEnv string
	open   func(string) gorm.Dialector
}

// TestSupplierAccountingRealDatabaseIntegration is intentionally opt-in. Each
// DSN must point to a disposable database/schema containing no tables. The
// additional confirmation prevents an accidentally exported production DSN
// from authorizing schema creation or cleanup by itself.
func TestSupplierAccountingRealDatabaseIntegration(t *testing.T) {
	databases := []supplierAccountingIntegrationDatabase{
		{
			name:   "mysql",
			dsnEnv: "TEST_SUPPLIER_ACCOUNTING_MYSQL_DSN",
			open: func(dsn string) gorm.Dialector {
				return mysql.Open(ensureMySQLDSNDefaults(dsn))
			},
		},
		{
			name:   "postgres",
			dsnEnv: "TEST_SUPPLIER_ACCOUNTING_POSTGRES_DSN",
			open:   func(dsn string) gorm.Dialector { return postgres.Open(dsn) },
		},
		{
			name:   "sqlite",
			dsnEnv: "TEST_SUPPLIER_ACCOUNTING_SQLITE_DSN",
			open: func(dsn string) gorm.Dialector {
				separator := "?"
				if strings.Contains(dsn, "?") {
					separator = "&"
				}
				return sqlite.Open(dsn + separator + "_pragma=busy_timeout(30000)")
			},
		},
	}

	for _, database := range databases {
		database := database
		t.Run(database.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(database.dsnEnv))
			if dsn == "" {
				t.Skipf("set %s to an isolated empty test database to run this real-%s integration test", database.dsnEnv, database.name)
			}
			require.Equal(t, supplierAccountingIntegrationConfirmation, strings.TrimSpace(os.Getenv("TEST_SUPPLIER_ACCOUNTING_ALLOW_SCHEMA_MUTATION")),
				"refusing schema mutation: set TEST_SUPPLIER_ACCOUNTING_ALLOW_SCHEMA_MUTATION=%s only after verifying the DSN is an isolated empty test database", supplierAccountingIntegrationConfirmation)

			db, err := gorm.Open(database.open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

			assertSupplierAccountingIntegrationDatabaseVersion(t, db)
			assertSupplierAccountingIntegrationDatabaseIsEmpty(t, db)
			migrateSupplierAccountingIntegrationSchema(t, db)
			t.Cleanup(func() { dropSupplierAccountingIntegrationSchema(t, db) })

			t.Run("migrates all supplier tables", func(t *testing.T) {
				for _, table := range supplierAccountingIntegrationTableNames() {
					require.True(t, db.Migrator().HasTable(table), "missing migrated table %s", table)
				}
			})

			t.Run("transitions fact pending to captured with CAS idempotency", func(t *testing.T) {
				testSupplierAccountingIntegrationFactLifecycle(t, db, database.name)
			})

			t.Run("increments a daily summary through a real upsert", func(t *testing.T) {
				testSupplierAccountingIntegrationDailySummaryUpsert(t, db, database.name)
			})

			t.Run("rejects a stale historical import lease fence", func(t *testing.T) {
				testSupplierAccountingIntegrationHistoricalFence(t, db, database.name)
			})

			t.Run("serializes concurrent overlapping historical imports", func(t *testing.T) {
				testSupplierAccountingIntegrationHistoricalOverlap(t, db, database.name)
			})

			t.Run("publishes and replaces historical report baselines", func(t *testing.T) {
				testSupplierAccountingIntegrationHistoricalPublication(t, db, database.name)
			})

			t.Run("serializes concurrent channel policy writers", func(t *testing.T) {
				testSupplierAccountingIntegrationPolicyCAS(t, db)
			})
		})
	}
}

func assertSupplierAccountingIntegrationDatabaseVersion(t *testing.T, db *gorm.DB) {
	t.Helper()
	switch db.Dialector.Name() {
	case "mysql":
		var version string
		require.NoError(t, db.Raw("SELECT VERSION()").Scan(&version).Error)
		var major, minor, patch int
		_, err := fmt.Sscanf(version, "%d.%d.%d", &major, &minor, &patch)
		require.NoError(t, err, "cannot parse MySQL server version %q", version)
		require.True(t, major > 5 || major == 5 && (minor > 7 || minor == 7 && patch >= 8), "MySQL 5.7.8+ is required, got %s", version)
	case "postgres":
		var versionText string
		require.NoError(t, db.Raw("SHOW server_version_num").Scan(&versionText).Error)
		version, err := strconv.Atoi(versionText)
		require.NoError(t, err)
		require.GreaterOrEqual(t, version, 90600, "PostgreSQL 9.6+ is required")
	case "sqlite":
		return
	default:
		t.Fatalf("unsupported integration database dialect %q", db.Dialector.Name())
	}
}

func assertSupplierAccountingIntegrationDatabaseIsEmpty(t *testing.T, db *gorm.DB) {
	t.Helper()
	tables, err := db.Migrator().GetTables()
	require.NoError(t, err)
	sort.Strings(tables)
	require.Empty(t, tables,
		"integration test refuses to mutate a database/schema that already contains tables; use a disposable empty database, found %v", tables)
}

func migrateSupplierAccountingIntegrationSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(
		&Option{}, &Channel{},
		&UpstreamSupplier{}, &SupplierContract{}, &SupplierContractRateVersion{},
		&SupplierChannelBindingVersion{}, &SupplierInventoryAdjustment{}, &SupplierStatisticsExclusionRule{},
		&SupplierUsageDailySummary{}, &SupplierUsageDailyBatchRun{},
		&SupplierHistoricalImport{}, &SupplierHistoricalDailySummary{},
	))
	require.NoError(t, MigrateSupplierHistoricalPublishedDaySchema(db))
	require.NoError(t, EnsureSupplierAccountingFactSchema(db))
	require.NoError(t, EnsureSupplierUsageGenerationSchema(db))
}

func dropSupplierAccountingIntegrationSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Migrator().DropTable(
		&SupplierHistoricalPublishedDay{}, &SupplierHistoricalDailySummary{}, &SupplierHistoricalImport{},
		&SupplierUsageDailySummary{}, &SupplierUsageDailyBatchRun{},
		&SupplierAccountingFact{}, &SupplierStatisticsExclusionRule{}, &SupplierInventoryAdjustment{},
		&SupplierChannelBindingVersion{}, &SupplierContractRateVersion{}, &SupplierContract{}, &UpstreamSupplier{},
		&Channel{}, &Option{},
	))
}

func supplierAccountingIntegrationTableNames() []string {
	return []string{
		"channels",
		"options",
		"supplier_accounting_facts",
		"supplier_channel_binding_versions",
		"supplier_contract_rate_versions",
		"supplier_contracts",
		"supplier_historical_daily_summaries",
		"supplier_historical_imports",
		"supplier_historical_published_days",
		"supplier_inventory_adjustments",
		"supplier_statistics_exclusion_rules",
		"supplier_usage_daily_batch_runs",
		"supplier_usage_daily_summaries",
		"upstream_suppliers",
	}
}

func testSupplierAccountingIntegrationPolicyCAS(t *testing.T, db *gorm.DB) {
	t.Helper()
	supplier := UpstreamSupplier{Name: "integration policy supplier"}
	require.NoError(t, db.Create(&supplier).Error)
	contract := SupplierContract{SupplierId: supplier.Id, Name: "integration policy contract", ContractNo: "integration-policy"}
	require.NoError(t, db.Create(&contract).Error)
	rate := SupplierContractRateVersion{ContractId: contract.Id, ProcurementMultiplierPpm: 650_000, CreatedBy: 1}
	require.NoError(t, db.Create(&rate).Error)
	require.NoError(t, db.Model(&contract).UpdateColumn("current_rate_version_id", rate.Id).Error)
	channel := Channel{Name: "integration policy channel", Key: "integration-policy-key", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&channel).Error)
	_, err := setChannelSupplierContractPolicyCASWithRetry(db, channel.Id, 0, false, &contract.Id, false, 1)
	require.NoError(t, err)

	results := make(chan error, 2)
	for actor := 2; actor <= 3; actor++ {
		actor := actor
		go func() {
			_, err := setChannelSupplierContractPolicyCASWithRetry(db, channel.Id, contract.Id, false, &contract.Id, true, actor)
			results <- err
		}()
	}
	successes, conflicts := 0, 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrSupplierBindingChanged):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent policy CAS result: %v", err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)

	index, health, err := loadSupplierRuntimeIndexSnapshot(db)
	require.NoError(t, err)
	require.False(t, health.Blocking)
	require.True(t, index.channelCosts[channel.Id].SkipInternalAccounting)
}

func testSupplierAccountingIntegrationFactLifecycle(t *testing.T, db *gorm.DB, dialect string) {
	t.Helper()
	ctx := context.Background()
	attemptID := supplierAccountingIntegrationAttemptID(dialect)
	input := SupplierAccountingFactPrepare{
		AttemptId: attemptID, ParentRequestId: "integration-" + dialect, RetryIndex: 1,
		SupplierId: 12, ContractId: 13, BindingVersionId: 11, RateVersionId: 14,
		ChannelId: 15, ModelName: "gpt-integration", CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
	}
	prepared, err := PrepareSupplierAccountingFact(ctx, db, input)
	require.NoError(t, err)
	require.Equal(t, SupplierAccountingFactStatusPending, prepared.Status)

	official, procurement, sales, gross, salesMultiplier := int64(1_000), int64(650), int64(700), int64(50), int64(700_000)
	envelope := types.SupplierAccountingEnvelopeV1{
		EnvelopeSchemaVersion: types.SupplierAccountingEnvelopeSchemaVersionV1,
		Disposition:           types.SupplierAccountingDispositionCaptured,
		Captured: &types.SupplierAccountingLogSnapshotV1{
			BindingVersionId: 11, SupplierId: 12, ContractId: 13, RateVersionId: 14,
			ProcurementMultiplierPpm: 650_000, SalesMultiplierPpm: &salesMultiplier,
			OfficialListMicroUsd: &official, ProcurementCostMicroUsd: &procurement,
			SalesMicroUsd: &sales, GrossProfitMicroUsd: &gross,
			StatisticsScope: string(types.SupplierStatisticsScopeBusiness), ExclusionDecision: "included",
			FinanciallyCommittedAt: time.Now().Unix(),
			PricingProvenance: &types.SupplierPricingProvenanceV1{Ratio: &types.SupplierRatioPricingProvenanceV1{
				ModelRatioPpm: 1_000_000, GroupRatioPpm: salesMultiplier, ModelRatioVersion: 1, GroupRatioVersion: 1,
			}},
		},
	}
	require.NoError(t, FinalizeSupplierAccountingFactCaptured(ctx, db, attemptID, envelope, time.Now().Unix()))
	require.NoError(t, FinalizeSupplierAccountingFactCaptured(ctx, db, attemptID, envelope, time.Now().Unix()))
	require.ErrorIs(t, FinalizeSupplierAccountingFactVoid(ctx, db, attemptID, time.Now().Unix()), ErrSupplierAccountingFactTerminalConflict)

	stored, err := GetSupplierAccountingFactByAttemptID(ctx, db, attemptID)
	require.NoError(t, err)
	require.Equal(t, SupplierAccountingFactStatusCaptured, stored.Status)
}

func supplierAccountingIntegrationAttemptID(dialect string) string {
	suffix := map[string]string{"mysql": "0001", "postgres": "0002", "sqlite": "0003"}[dialect]
	return "018f843e-7e3a-7f61-a0a0-00000000" + suffix
}

func testSupplierAccountingIntegrationDailySummaryUpsert(t *testing.T, db *gorm.DB, dialect string) {
	t.Helper()
	summary := SupplierUsageDailySummary{
		BatchDate: "2098-01-01", BatchFenceToken: 1, DimensionKey: "integration-" + dialect,
		BucketStart: 4_039_372_800, SupplierId: 12, ContractId: 13, BindingVersionId: 11,
		RateVersionId: 14, ChannelId: 15, ModelName: "gpt-integration",
		StatisticsScope: string(types.SupplierStatisticsScopeBusiness), DataQuality: "actual", RequestCount: 1,
	}
	require.NoError(t, upsertSupplierDailySummaries(db, []SupplierUsageDailySummary{summary}))
	summary.RequestCount = 2
	require.NoError(t, upsertSupplierDailySummaries(db, []SupplierUsageDailySummary{summary}))

	var stored SupplierUsageDailySummary
	require.NoError(t, db.Where("batch_date = ? AND batch_fence_token = ? AND dimension_key = ?", summary.BatchDate, summary.BatchFenceToken, summary.DimensionKey).First(&stored).Error)
	require.Equal(t, int64(3), stored.RequestCount)
}

func testSupplierAccountingIntegrationHistoricalFence(t *testing.T, db *gorm.DB, dialect string) {
	t.Helper()
	ctx := context.Background()
	input := supplierAccountingIntegrationHistoricalInput(dialect+"-fence", 100, 200)
	item, err := CreateSupplierHistoricalImport(ctx, db, input)
	require.NoError(t, err)

	stale, err := AcquireSupplierHistoricalImport(ctx, db, item.Id, "node-a", time.Minute)
	require.NoError(t, err)
	require.NoError(t, db.Model(&SupplierHistoricalImport{}).Where("id = ?", item.Id).Update("locked_until", int64(1)).Error)
	current, err := AcquireSupplierHistoricalImport(ctx, db, item.Id, "node-b", time.Minute)
	require.NoError(t, err)
	require.Greater(t, current.FenceToken, stale.FenceToken)
	require.ErrorIs(t, FreezeSupplierHistoricalImport(ctx, db, stale, 10, 2, 0), ErrSupplierHistoricalImportFenceLost)
	require.NoError(t, FreezeSupplierHistoricalImport(ctx, db, current, 10, 2, 0))
}

func testSupplierAccountingIntegrationHistoricalOverlap(t *testing.T, db *gorm.DB, dialect string) {
	t.Helper()
	inputs := []SupplierHistoricalImportCreate{
		supplierAccountingIntegrationHistoricalInput(dialect+"-overlap-a", 1_000, 2_000),
		supplierAccountingIntegrationHistoricalInput(dialect+"-overlap-b", 1_500, 2_500),
	}
	start := make(chan struct{})
	errs := make(chan error, len(inputs))
	var workers sync.WaitGroup
	for _, input := range inputs {
		input := input
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, err := CreateSupplierHistoricalImport(context.Background(), db, input)
			errs <- err
		}()
	}
	close(start)
	workers.Wait()
	close(errs)

	var accepted, rejected int
	for err := range errs {
		switch {
		case err == nil:
			accepted++
		case errors.Is(err, ErrSupplierHistoricalImportOverlap):
			rejected++
		default:
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, accepted)
	require.Equal(t, 1, rejected)
}

func testSupplierAccountingIntegrationHistoricalPublication(t *testing.T, db *gorm.DB, dialect string) {
	t.Helper()
	ctx := context.Background()
	const (
		dayStart = int64(4_075_027_200)
		dayEnd   = dayStart + 86_400
	)
	baseInput := supplierAccountingIntegrationHistoricalInput(dialect+"-published", dayStart, dayEnd)
	baseInput.StartDate = "2099-02-18"
	baseInput.EndDate = "2099-02-19"
	base, err := CreateSupplierHistoricalImport(ctx, db, baseInput)
	require.NoError(t, err)
	require.NoError(t, db.Model(&SupplierHistoricalImport{}).Where("id = ?", base.Id).UpdateColumn("status", SupplierHistoricalImportStatusCompleted).Error)
	require.NoError(t, db.Create(&SupplierHistoricalDailySummary{
		ImportId: base.Id, Date: base.StartDate, DimensionKey: "integration-published", BucketStart: dayStart,
		SupplierId: 12, ContractId: 13, RateVersionId: 14, ChannelId: 15, ModelName: "historical",
		StatisticsScope: string(types.SupplierStatisticsScopeBusiness), DataQuality: SupplierHistoricalDataQualityEstimated,
		SourceRequestCount: 2, SalesKnownCount: 2, SalesMicroUsd: 700, ProcurementCostKnownCount: 2,
		ProcurementCostMicroUsd: 650, GrossProfitKnownCount: 2, GrossProfitMicroUsd: 50,
		GrossMarginEligibleCount: 2, GrossMarginEligibleSalesMicroUsd: 700,
	}).Error)

	initialResults := make(chan error, 2)
	for range 2 {
		go func() {
			_, publishErr := PublishSupplierHistoricalImport(ctx, db, base.Id, 700)
			initialResults <- publishErr
		}()
	}
	for range 2 {
		require.NoError(t, <-initialResults)
	}

	replacements := make([]SupplierHistoricalImport, 2)
	for index := range replacements {
		replacements[index] = newSupplierHistoricalImport(baseInput)
		replacements[index].CommandHash = strings.Repeat(strconv.Itoa(index+2), 64)
		replacements[index].IdempotencyKey = fmt.Sprintf("%s-replacement-%d", dialect, index)
		replacements[index].Reason = "concurrent replacement"
		replacements[index].Status = SupplierHistoricalImportStatusCompleted
		replacements[index].SupersedesImportId = &base.Id
	}
	require.NoError(t, db.Create(&replacements).Error)

	replacementResults := make(chan error, len(replacements))
	for index := range replacements {
		item := replacements[index]
		go func() {
			_, publishErr := PublishSupplierHistoricalImport(ctx, db, item.Id, item.CreatedBy)
			replacementResults <- publishErr
		}()
	}
	var publishedReplacement int64
	for range replacements {
		publishErr := <-replacementResults
		if publishErr == nil {
			continue
		}
		require.True(t, errors.Is(publishErr, ErrSupplierHistoricalPublicationConflict) || errors.Is(publishErr, ErrSupplierHistoricalReplacementInvalid), publishErr)
	}
	var publishedDay SupplierHistoricalPublishedDay
	require.NoError(t, db.First(&publishedDay, "date = ?", base.StartDate).Error)
	publishedReplacement = publishedDay.ImportId
	require.Contains(t, []int64{replacements[0].Id, replacements[1].Id}, publishedReplacement)

	require.NoError(t, db.Create(&SupplierHistoricalDailySummary{
		ImportId: publishedReplacement, Date: base.StartDate, DimensionKey: "replacement", BucketStart: dayStart,
		SupplierId: 12, ContractId: 13, RateVersionId: 14, ChannelId: 15, ModelName: "historical-v2",
		StatisticsScope: string(types.SupplierStatisticsScopeBusiness), DataQuality: SupplierHistoricalDataQualityEstimated,
		SourceRequestCount: 3, GrossMarginEligibleCount: 3,
	}).Error)
	store := NewSupplierReportStore(db)
	filter := SupplierReportFilter{StartAt: dayStart, EndAt: dayEnd}
	usage, err := store.QueryBusinessUsage(ctx, filter, false)
	require.NoError(t, err)
	require.Equal(t, int64(3), usage[0].BusinessRequestCount)

	publishedAt := dayEnd
	require.NoError(t, db.Create(&SupplierUsageDailyBatchRun{
		BatchDate: base.StartDate, DayStart: dayStart, DayEnd: dayEnd, Status: SupplierDailyBatchStatusCompleted,
		FenceToken: 9, PublishedFenceToken: 9, PublishedAt: &publishedAt,
	}).Error)
	require.NoError(t, db.Create(&SupplierUsageDailySummary{
		BatchDate: base.StartDate, BatchFenceToken: 9, DimensionKey: "authoritative", BucketStart: dayStart,
		SupplierId: 12, ContractId: 13, RateVersionId: 14, ChannelId: 15, ModelName: "authoritative",
		StatisticsScope: string(types.SupplierStatisticsScopeBusiness), DataQuality: "authoritative", RequestCount: 5,
	}).Error)
	usage, err = store.QueryBusinessUsage(ctx, filter, false)
	require.NoError(t, err)
	require.Equal(t, int64(5), usage[0].BusinessRequestCount)
}

func supplierAccountingIntegrationHistoricalInput(key string, dayStart, dayEnd int64) SupplierHistoricalImportCreate {
	hashCharacter := "a"
	if strings.HasSuffix(key, "b") {
		hashCharacter = "b"
	}
	return SupplierHistoricalImportCreate{
		CommandHash: strings.Repeat(hashCharacter, 64), CommandJSON: `{}`,
		IdempotencyKey: key, CreatedBy: 700, Method: SupplierHistoricalMethodLogEstimateV1, Reason: "real database integration test",
		StartDate: "2099-01-01", EndDate: "2099-02-01", DayStart: dayStart, DayEnd: dayEnd,
		QuotaPerUnit: "500000", ExcludedUserIdsJSON: `[]`, ChannelMappingsJSON: `[]`,
	}
}
