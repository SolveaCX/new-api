package model

import (
	"sort"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSupplierTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalUsingSQLite := common.UsingSQLite
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalSupplierIndex := supplierRuntimeIndexPointer.Load()
	originalSupplierHealth := supplierCacheHealthPointer.Load()
	t.Cleanup(func() {
		DB = originalDB
		common.UsingSQLite = originalUsingSQLite
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		supplierRuntimeIndexPointer.Store(originalSupplierIndex)
		supplierCacheHealthPointer.Store(originalSupplierHealth)
	})

	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&Option{},
		&Channel{},
		&Ability{},
		&UpstreamSupplier{},
		&SupplierContract{},
		&SupplierContractRateVersion{},
		&SupplierChannelBindingVersion{},
		&SupplierInventoryAdjustment{},
		&SupplierStatisticsExclusionRule{},
	))
	DB = db
	common.UsingSQLite = true
	common.MemoryCacheEnabled = false
	supplierRuntimeIndexPointer.Store(emptySupplierRuntimeIndex())
	supplierCacheHealthPointer.Store(nil)
	return db
}

func createSupplierContractFixture(t *testing.T, db *gorm.DB, supplierName string, contractNo string) SupplierContract {
	t.Helper()
	supplier := UpstreamSupplier{Name: supplierName}
	require.NoError(t, db.Create(&supplier).Error)
	contract := SupplierContract{
		SupplierId: supplier.Id,
		Name:       supplierName + " contract",
		ContractNo: contractNo,
	}
	require.NoError(t, db.Create(&contract).Error)
	return contract
}

func TestSupplierMainDBUniqueConstraints(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-unique-constraints")
	contractA := createSupplierContractFixture(t, db, "supplier-a", "contract-a")
	contractB := createSupplierContractFixture(t, db, "supplier-b", "contract-b")

	require.Error(t, db.Create(&UpstreamSupplier{Name: "supplier-a"}).Error)

	adjustment := SupplierInventoryAdjustment{
		ContractId:     contractA.Id,
		DeltaMicroUsd:  100_000_000,
		Type:           SupplierInventoryAdjustmentTypeInitial,
		IdempotencyKey: "inventory-request-1",
		CreatedBy:      1,
	}
	require.NoError(t, db.Create(&adjustment).Error)
	duplicateAdjustment := adjustment
	duplicateAdjustment.Id = 0
	require.Error(t, db.Create(&duplicateAdjustment).Error)
	otherContractAdjustment := adjustment
	otherContractAdjustment.Id = 0
	otherContractAdjustment.ContractId = contractB.Id
	require.NoError(t, db.Create(&otherContractAdjustment).Error)

	rule := SupplierStatisticsExclusionRule{
		UserId:         101,
		Action:         SupplierStatisticsActionExclude,
		IdempotencyKey: "stats-request-1",
		CreatedBy:      1,
	}
	require.NoError(t, db.Create(&rule).Error)
	duplicateRule := rule
	duplicateRule.Id = 0
	duplicateRule.UserId = 102
	require.Error(t, db.Create(&duplicateRule).Error)
	otherCreatorRule := rule
	otherCreatorRule.Id = 0
	otherCreatorRule.CreatedBy = 2
	require.NoError(t, db.Create(&otherCreatorRule).Error)

	require.NoError(t, db.Model(&UpstreamSupplier{}).Where("id = ?", contractA.SupplierId).UpdateColumn("status", SupplierStatusInactive).Error)
	require.ErrorIs(t, db.Delete(&UpstreamSupplier{Id: contractA.SupplierId}).Error, ErrSupplierHardDeleteForbidden)
	require.NoError(t, db.Model(&SupplierContract{}).Where("id = ?", contractA.Id).UpdateColumn("status", SupplierContractStatusInactive).Error)
	require.ErrorIs(t, db.Delete(&SupplierContract{Id: contractA.Id}).Error, ErrSupplierHardDeleteForbidden)
}

func TestChannelSupplierContractBindingIsNotJSONAssignableOrExposed(t *testing.T) {
	var channel Channel
	require.NoError(t, common.Unmarshal([]byte(`{"name":"channel","supplier_contract_id":123}`), &channel))
	require.Equal(t, "channel", channel.Name)
	require.Nil(t, channel.SupplierContractId)

	contractId := 123
	channel.SupplierContractId = &contractId
	payload, err := common.Marshal(channel)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "supplier_contract_id")
}

func TestMigrateDBFastRegistersSupplierModels(t *testing.T) {
	originalDB := DB
	originalUsingSQLite := common.UsingSQLite
	t.Cleanup(func() {
		DB = originalDB
		common.UsingSQLite = originalUsingSQLite
	})
	t.Setenv("LOG_SQL_DSN", "")

	db, err := gorm.Open(sqlite.Open("file:supplier-fast-migration?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	DB = db
	common.UsingSQLite = true

	require.NoError(t, migrateDBFast())
	for _, model := range []any{
		&UpstreamSupplier{},
		&SupplierContract{},
		&SupplierContractRateVersion{},
		&SupplierChannelBindingVersion{},
		&SupplierInventoryAdjustment{},
		&SupplierStatisticsExclusionRule{},
		&SupplierUsageDailySummary{},
		&SupplierUsageDailyBatchRun{},
	} {
		require.True(t, db.Migrator().HasTable(model))
	}
	require.True(t, db.Migrator().HasColumn(&Channel{}, "SupplierContractId"))
}

func TestSupplierAppendOnlyRecordsRejectUpdateAndDelete(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-append-only")
	contract := createSupplierContractFixture(t, db, "append-only-supplier", "append-only-contract")

	rate := SupplierContractRateVersion{
		ContractId:               contract.Id,
		ProcurementMultiplierPpm: 650_000,
		CreatedBy:                1,
	}
	adjustment := SupplierInventoryAdjustment{
		ContractId:     contract.Id,
		DeltaMicroUsd:  100_000_000,
		Type:           SupplierInventoryAdjustmentTypeInitial,
		IdempotencyKey: "append-only-adjustment",
		CreatedBy:      1,
	}
	rule := SupplierStatisticsExclusionRule{
		UserId:         101,
		Action:         SupplierStatisticsActionExclude,
		IdempotencyKey: "append-only-rule",
		CreatedBy:      1,
	}
	require.NoError(t, db.Create(&rate).Error)
	require.NoError(t, db.Create(&adjustment).Error)
	require.NoError(t, db.Create(&rule).Error)

	require.ErrorIs(t, db.Model(&rate).Update("procurement_multiplier_ppm", 700_000).Error, ErrSupplierAppendOnly)
	require.ErrorIs(t, db.Delete(&rate).Error, ErrSupplierAppendOnly)
	require.ErrorIs(t, db.Model(&adjustment).Update("delta_micro_usd", 1).Error, ErrSupplierAppendOnly)
	require.ErrorIs(t, db.Delete(&adjustment).Error, ErrSupplierAppendOnly)
	require.ErrorIs(t, db.Model(&rule).Update("action", SupplierStatisticsActionInclude).Error, ErrSupplierAppendOnly)
	require.ErrorIs(t, db.Delete(&rule).Error, ErrSupplierAppendOnly)

	var persistedRate SupplierContractRateVersion
	require.NoError(t, db.First(&persistedRate, rate.Id).Error)
	require.Equal(t, int64(650_000), persistedRate.ProcurementMultiplierPpm)
	var count int64
	require.NoError(t, db.Model(&SupplierInventoryAdjustment{}).Where("id = ?", adjustment.Id).Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.NoError(t, db.Model(&SupplierStatisticsExclusionRule{}).Where("id = ?", rule.Id).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestSupplierContractRateVersionActivationPreservesHistory(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-rate-versioning")
	contract := createSupplierContractFixture(t, db, "versioned-supplier", "versioned-contract")
	channel := Channel{Name: "versioned channel", Key: "key", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&channel).Error)

	first, err := CreateAndActivateSupplierContractRateVersion(contract.Id, 650_000, 1, "initial")
	require.NoError(t, err)
	require.NoError(t, SetChannelSupplierContractCASForActor(channel.Id, 0, &contract.Id, 1))
	firstSnapshot, ok := GetSupplierCostSnapshot(channel.Id)
	require.True(t, ok)
	require.Equal(t, first.Id, firstSnapshot.RateVersionId)
	second, err := CreateAndActivateSupplierContractRateVersion(contract.Id, 700_000, 1, "new rate")
	require.NoError(t, err)
	require.NotEqual(t, first.Id, second.Id)
	secondSnapshot, ok := GetSupplierCostSnapshot(channel.Id)
	require.True(t, ok)
	require.Equal(t, second.Id, secondSnapshot.RateVersionId)

	var refreshedContract SupplierContract
	require.NoError(t, db.First(&refreshedContract, contract.Id).Error)
	require.NotNil(t, refreshedContract.CurrentRateVersionId)
	require.Equal(t, second.Id, *refreshedContract.CurrentRateVersionId)
	require.Equal(t, int64(3), refreshedContract.RowVersion, "each non-idempotent current-rate advance increments the contract CAS version")

	var persistedFirst SupplierContractRateVersion
	require.NoError(t, db.First(&persistedFirst, first.Id).Error)
	require.Equal(t, int64(650_000), persistedFirst.ProcurementMultiplierPpm)

	third := SupplierContractRateVersion{ContractId: contract.Id, ProcurementMultiplierPpm: 750_000, CreatedBy: 1}
	require.NoError(t, db.Create(&third).Error)
	require.NoError(t, db.Model(&SupplierContract{}).Where("id = ?", contract.Id).UpdateColumn("current_rate_version_id", third.Id).Error)
	_, err = CreateAndActivateSupplierContractRateVersion(contract.Id, -1, 1, "invalid rate must roll back")
	require.ErrorIs(t, err, ErrSupplierInvalidRate)
	_, err = CreateAndActivateSupplierContractRateVersion(contract.Id, 1_000_001, 1, "markup is outside V1 discount bounds")
	require.ErrorIs(t, err, ErrSupplierInvalidRate)
	retainedAfterRollback, ok := GetSupplierCostSnapshot(channel.Id)
	require.True(t, ok)
	require.Equal(t, second.Id, retainedAfterRollback.RateVersionId)
}

func TestSupplierEffectiveTimesComeFromDatabase(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-database-effective-time")
	contract := createSupplierContractFixture(t, db, "database-time-supplier", "database-time-contract")
	before, err := getSupplierDBTimestamp(db)
	require.NoError(t, err)

	rate := SupplierContractRateVersion{
		ContractId:               contract.Id,
		ProcurementMultiplierPpm: 650_000,
		EffectiveAt:              1,
		CreatedBy:                1,
	}
	require.NoError(t, db.Create(&rate).Error)
	after, err := getSupplierDBTimestamp(db)
	require.NoError(t, err)
	require.GreaterOrEqual(t, rate.EffectiveAt, before)
	require.LessOrEqual(t, rate.EffectiveAt, after)
	require.NotEqual(t, int64(1), rate.EffectiveAt)

	rule, err := CreateSupplierStatisticsExclusionRule(301, SupplierStatisticsActionExclude, 1, "database time", "database-time-rule")
	require.NoError(t, err)
	require.GreaterOrEqual(t, rule.EffectiveAt, before)
	require.LessOrEqual(t, rule.EffectiveAt, common.GetTimestamp())
}

func supplierSQLiteIndexColumns(t *testing.T, db *gorm.DB, indexName string) []string {
	t.Helper()
	type indexColumn struct {
		Seqno int
		Name  string
	}
	var columns []indexColumn
	require.NoError(t, db.Raw("PRAGMA index_info('"+indexName+"')").Scan(&columns).Error)
	sort.Slice(columns, func(i, j int) bool { return columns[i].Seqno < columns[j].Seqno })
	names := make([]string, len(columns))
	for index := range columns {
		names[index] = columns[index].Name
	}
	return names
}
