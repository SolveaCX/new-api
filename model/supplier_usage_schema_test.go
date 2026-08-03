package model

import (
	"sort"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSupplierV1SchemaHasExactlyTwelveTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&Option{},
		&UpstreamSupplier{}, &SupplierContract{}, &SupplierContractRateVersion{},
		&SupplierChannelBindingVersion{}, &SupplierInventoryAdjustment{}, &SupplierStatisticsExclusionRule{},
		&SupplierUsageDailySummary{}, &SupplierUsageDailyBatchRun{},
		&SupplierHistoricalImport{}, &SupplierHistoricalDailySummary{},
	))
	require.NoError(t, MigrateSupplierHistoricalPublishedDaySchema(db))
	require.NoError(t, EnsureSupplierAccountingFactSchema(db))
	var names []string
	require.NoError(t, db.Raw(`SELECT name FROM sqlite_master WHERE type = 'table' AND (name = 'upstream_suppliers' OR name LIKE 'supplier_%')`).Scan(&names).Error)
	sort.Strings(names)
	require.Equal(t, []string{
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
	}, names)
}

func TestSupplierV1DoesNotAddLogColumnsOrIndexes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	require.NoError(t, EnsureSupplierAccountingFactSchema(db))
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
