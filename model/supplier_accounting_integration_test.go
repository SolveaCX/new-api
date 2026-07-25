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
			open:   func(dsn string) gorm.Dialector { return sqlite.Open(dsn) },
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
		&UpstreamSupplier{}, &SupplierContract{}, &SupplierContractRateVersion{},
		&SupplierChannelBindingVersion{}, &SupplierInventoryAdjustment{}, &SupplierStatisticsExclusionRule{},
		&SupplierUsageDailySummary{}, &SupplierUsageDailyBatchRun{},
		&SupplierHistoricalImport{}, &SupplierHistoricalDailySummary{},
	))
	require.NoError(t, EnsureSupplierAccountingFactSchema(db))
	require.NoError(t, EnsureSupplierUsageGenerationSchema(db))
}

func dropSupplierAccountingIntegrationSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Migrator().DropTable(
		&SupplierHistoricalDailySummary{}, &SupplierHistoricalImport{},
		&SupplierUsageDailySummary{}, &SupplierUsageDailyBatchRun{},
		&SupplierAccountingFact{}, &SupplierStatisticsExclusionRule{}, &SupplierInventoryAdjustment{},
		&SupplierChannelBindingVersion{}, &SupplierContractRateVersion{}, &SupplierContract{}, &UpstreamSupplier{},
	))
}

func supplierAccountingIntegrationTableNames() []string {
	return []string{
		"supplier_accounting_facts",
		"supplier_channel_binding_versions",
		"supplier_contract_rate_versions",
		"supplier_contracts",
		"supplier_historical_daily_summaries",
		"supplier_historical_imports",
		"supplier_inventory_adjustments",
		"supplier_statistics_exclusion_rules",
		"supplier_usage_daily_batch_runs",
		"supplier_usage_daily_summaries",
		"upstream_suppliers",
	}
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
	require.ErrorIs(t, FreezeSupplierHistoricalImport(ctx, db, stale, 10, 2), ErrSupplierHistoricalImportFenceLost)
	require.NoError(t, FreezeSupplierHistoricalImport(ctx, db, current, 10, 2))
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
