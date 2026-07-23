package model

import (
	"sort"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSupplierV1SchemaHasTenTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&UpstreamSupplier{}, &SupplierContract{}, &SupplierContractRateVersion{},
		&SupplierChannelBindingVersion{}, &SupplierInventoryAdjustment{}, &SupplierStatisticsExclusionRule{},
		&SupplierUsageDailySummary{}, &SupplierUsageDailyBatchRun{},
		&SupplierAccountingCoverageGap{},
	))
	require.NoError(t, MigrateSupplierAdminCommandLedger(db))
	require.NoError(t, ValidateSupplierAdminCommandLedgerFinalized(db))
	var names []string
	require.NoError(t, db.Raw(`SELECT name FROM sqlite_master WHERE type = 'table' AND (name = 'upstream_suppliers' OR name LIKE 'supplier_%')`).Scan(&names).Error)
	sort.Strings(names)
	require.Equal(t, []string{
		"supplier_accounting_coverage_gaps",
		"supplier_admin_commands",
		"supplier_channel_binding_versions",
		"supplier_contract_rate_versions",
		"supplier_contracts",
		"supplier_inventory_adjustments",
		"supplier_statistics_exclusion_rules",
		"supplier_usage_daily_batch_runs",
		"supplier_usage_daily_summaries",
		"upstream_suppliers",
	}, names)
}

func TestMigrateDBFastRunsSupplierAdminCommandLedgerMigrationRerunnably(t *testing.T) {
	originalDB := DB
	originalUsingSQLite := common.UsingSQLite
	t.Cleanup(func() {
		DB = originalDB
		common.UsingSQLite = originalUsingSQLite
	})
	t.Setenv("LOG_SQL_DSN", "")

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	DB = db
	common.UsingSQLite = true

	require.NoError(t, db.AutoMigrate(&legacySupplierAdminCommand{}))
	legacy := legacySupplierAdminCommand{
		ActorId: 7, Scope: "activation.transition", IdempotencyKey: "legacy-startup-key",
		PayloadVersion: 1, PayloadDigest: strings.Repeat("1", 64), ResourceType: "activation",
		ResourceId: 3, ClaimToken: strings.Repeat("2", 32),
	}
	require.NoError(t, db.Create(&legacy).Error)
	require.NotEmpty(t, supplierSQLiteIndexColumns(t, db, legacySupplierAdminCommandScopeKeyIndex))

	require.NoError(t, migrateDBFast())
	require.NoError(t, migrateDBFast(), "startup migration must be rerunnable")
	require.Equal(t, []string{"scope", "idempotency_key"}, supplierSQLiteIndexColumns(t, db, legacySupplierAdminCommandScopeKeyIndex))
	require.Equal(t, []string{"actor_id", "scope", "idempotency_key"}, supplierSQLiteIndexColumns(t, db, legacySupplierAdminCommandActorScopeKeyIndex))
	require.Equal(t, []string{"actor_id", "scope", "idempotency_key_digest"}, supplierSQLiteIndexColumns(t, db, supplierAdminCommandActorScopeDigestIndex))
	status, err := GetSupplierAdminCommandLedgerMigrationStatus(db)
	require.NoError(t, err)
	require.False(t, status.Finalized)

	var migrated SupplierAdminCommand
	require.NoError(t, db.First(&migrated, legacy.Id).Error)
	require.Equal(t, supplierAdminIdempotencyKeyDigest(legacy.IdempotencyKey), migrated.IdempotencyKeyDigest)
	require.NoError(t, FinalizeSupplierAdminCommandLedgerMigration(db))
	require.NoError(t, ValidateSupplierAdminCommandLedgerFinalized(db))
	require.NoError(t, migrateDBFast(), "startup after finalization must not recreate legacy indexes")
	require.Empty(t, supplierSQLiteIndexColumns(t, db, legacySupplierAdminCommandScopeKeyIndex))
	require.Empty(t, supplierSQLiteIndexColumns(t, db, legacySupplierAdminCommandActorScopeKeyIndex))
}

func TestMigrateDBProductionRegistersCoverageGapsAndRepairsCommandLedgerRerunnably(t *testing.T) {
	originalDB := DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	t.Cleanup(func() {
		DB = originalDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
	})
	t.Setenv("LOG_SQL_DSN", "")

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	require.NoError(t, db.AutoMigrate(&legacySupplierAdminCommand{}))
	legacy := legacySupplierAdminCommand{
		ActorId: 11, Scope: "activation.transition", IdempotencyKey: "legacy-production-startup-key",
		PayloadVersion: 1, PayloadDigest: strings.Repeat("3", 64), ResourceType: "activation",
		ResourceId: 5, ClaimToken: strings.Repeat("4", 32),
	}
	require.NoError(t, db.Create(&legacy).Error)

	require.NoError(t, migrateDB())
	require.NoError(t, migrateDB(), "production startup migration must be rerunnable")
	require.True(t, db.Migrator().HasTable(&SupplierAccountingCoverageGap{}))
	require.True(t, db.Migrator().HasIndex(&SupplierAccountingCoverageGap{}, "ux_supplier_coverage_gap_open_command"))
	require.Equal(t, []string{"scope", "idempotency_key"}, supplierSQLiteIndexColumns(t, db, legacySupplierAdminCommandScopeKeyIndex))
	require.Equal(t, []string{"actor_id", "scope", "idempotency_key"}, supplierSQLiteIndexColumns(t, db, legacySupplierAdminCommandActorScopeKeyIndex))
	require.Equal(t, []string{"actor_id", "scope", "idempotency_key_digest"}, supplierSQLiteIndexColumns(t, db, supplierAdminCommandActorScopeDigestIndex))
	status, err := GetSupplierAdminCommandLedgerMigrationStatus(db)
	require.NoError(t, err)
	require.False(t, status.Finalized)

	var migrated SupplierAdminCommand
	require.NoError(t, db.First(&migrated, legacy.Id).Error)
	require.Equal(t, supplierAdminIdempotencyKeyDigest(legacy.IdempotencyKey), migrated.IdempotencyKeyDigest)
	require.NoError(t, FinalizeSupplierAdminCommandLedgerMigration(db))
	require.NoError(t, ValidateSupplierAdminCommandLedgerFinalized(db))
	require.NoError(t, migrateDB(), "production startup after finalization must not recreate legacy indexes")
	require.Empty(t, supplierSQLiteIndexColumns(t, db, legacySupplierAdminCommandScopeKeyIndex))
	require.Empty(t, supplierSQLiteIndexColumns(t, db, legacySupplierAdminCommandActorScopeKeyIndex))
}

func TestSupplierV1DoesNotAddLogColumnsOrIndexes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	var columns []struct{ Name string }
	require.NoError(t, db.Raw("PRAGMA table_info('logs')").Scan(&columns).Error)
	for _, column := range columns {
		require.False(t, strings.Contains(column.Name, "supplier"), column.Name)
	}
	var indexes []struct{ Name string }
	require.NoError(t, db.Raw("PRAGMA index_list('logs')").Scan(&indexes).Error)
	for _, index := range indexes {
		require.False(t, strings.Contains(index.Name, "supplier"), index.Name)
	}
}
