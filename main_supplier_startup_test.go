package main

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInitializeSupplierRuntimeAllowsSlaveBeforeMasterMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	oldDB := model.DB
	oldMaster := common.IsMasterNode
	model.DB = db
	common.IsMasterNode = false
	t.Cleanup(func() {
		model.DB = oldDB
		common.IsMasterNode = oldMaster
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	// No supplier tables exist yet. A router must still start and keep supplier
	// accounting fail-closed until its normal cache refresh observes the master
	// migration.
	require.NoError(t, initializeSupplierRuntime())
	require.True(t, model.IsSupplierCacheBlocking())

	// Simulate the master completing its additive migration. The same refresh
	// primitive used by SyncChannelCache/SyncSupplierCache must then recover the
	// router without a restart.
	require.NoError(t, db.AutoMigrate(
		&model.Channel{},
		&model.UpstreamSupplier{},
		&model.SupplierContract{},
		&model.SupplierContractRateVersion{},
		&model.SupplierChannelBindingVersion{},
		&model.SupplierInventoryAdjustment{},
		&model.SupplierStatisticsExclusionRule{},
		&model.SupplierUsageDailySummary{},
		&model.SupplierUsageDailyBatchRun{},
	))
	require.NoError(t, model.RefreshSupplierCache())
	require.False(t, model.IsSupplierCacheBlocking())
}

func TestInitializeSupplierRuntimeKeepsMasterMigrationFailureFatal(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	oldDB := model.DB
	oldMaster := common.IsMasterNode
	model.DB = db
	common.IsMasterNode = true
	t.Cleanup(func() {
		model.DB = oldDB
		common.IsMasterNode = oldMaster
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	require.Error(t, initializeSupplierRuntime())
}
