package model

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestSupplierAdminMassAssignmentCannotMoveContractOrCurrentRate(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-mass-assignment")
	createdSupplier := UpstreamSupplier{Id: 999, Name: "allowlisted create", Status: SupplierStatusInactive, CreatedAt: 1, UpdatedAt: 1}
	require.NoError(t, CreateUpstreamSupplier(&createdSupplier))
	require.NotEqual(t, 999, createdSupplier.Id)
	require.Equal(t, SupplierStatusActive, createdSupplier.Status)
	require.NotEqual(t, int64(1), createdSupplier.CreatedAt)
	createdContract := SupplierContract{Id: 999, SupplierId: createdSupplier.Id, Name: "allowlisted contract", ContractNo: "allowlisted-1", Status: SupplierContractStatusInactive, CurrentRateVersionId: intPointerForSupplierAdminTest(999)}
	require.NoError(t, CreateSupplierContract(&createdContract))
	require.NotEqual(t, 999, createdContract.Id)
	require.Equal(t, SupplierContractStatusActive, createdContract.Status)
	require.Nil(t, createdContract.CurrentRateVersionId)

	contract := createSupplierContractFixture(t, db, "immutable-supplier", "immutable-contract")
	other := createSupplierContractFixture(t, db, "other-supplier", "other-contract")
	rate, err := CreateAndActivateSupplierContractRateVersion(contract.Id, 650_000, 1, "initial")
	require.NoError(t, err)

	err = db.Model(&contract).Updates(map[string]any{"supplier_id": other.SupplierId}).Error
	require.ErrorIs(t, err, ErrSupplierImmutableField)
	err = db.Model(&contract).Updates(map[string]any{"current_rate_version_id": 999}).Error
	require.ErrorIs(t, err, ErrSupplierImmutableField)
	err = db.Model(&contract).Updates(map[string]any{"status": SupplierContractStatusInactive}).Error
	require.ErrorIs(t, err, ErrSupplierImmutableField)

	var persisted SupplierContract
	require.NoError(t, db.First(&persisted, contract.Id).Error)
	require.Equal(t, contract.SupplierId, persisted.SupplierId)
	require.NotNil(t, persisted.CurrentRateVersionId)
	require.Equal(t, rate.Id, *persisted.CurrentRateVersionId)
	require.Equal(t, SupplierContractStatusActive, persisted.Status)

	supplier := UpstreamSupplier{Id: contract.SupplierId}
	err = db.Model(&supplier).Update("status", SupplierStatusInactive).Error
	require.ErrorIs(t, err, ErrSupplierImmutableField)
}

func intPointerForSupplierAdminTest(value int) *int {
	return &value
}

func TestSupplierAdminBindingAndInactivationInvariants(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-binding-inactivation")
	contract := createSupplierContractFixture(t, db, "binding-supplier", "binding-contract")
	channel := Channel{Name: "binding channel", Key: "key", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&channel).Error)

	require.ErrorIs(t, BindChannelSupplierContract(channel.Id, contract.Id), ErrSupplierCurrentRateRequired)
	_, err := CreateAndActivateSupplierContractRateVersion(contract.Id, 650_000, 1, "initial")
	require.NoError(t, err)
	require.NoError(t, BindChannelSupplierContract(channel.Id, contract.Id))
	binding, err := GetChannelSupplierContractBinding(channel.Id)
	require.NoError(t, err)
	require.Equal(t, channel.Id, binding.ChannelId)
	require.NotNil(t, binding.SupplierContractId)
	bindings, total, err := ListSupplierChannelBindings(contract.Id, SupplierPage{})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, bindings, 1)

	var rebound Channel
	require.NoError(t, db.Select("id", "supplier_contract_id").First(&rebound, channel.Id).Error)
	require.NotNil(t, rebound.SupplierContractId)
	require.Equal(t, contract.Id, *rebound.SupplierContractId)
	require.ErrorIs(t, InactivateSupplierContract(contract.Id), ErrSupplierContractBound)
	require.ErrorIs(t, InactivateUpstreamSupplier(contract.SupplierId), ErrSupplierHasActiveContracts)

	require.NoError(t, UnbindChannelSupplierContract(channel.Id))
	require.NoError(t, InactivateSupplierContract(contract.Id))
	require.ErrorIs(t, BindChannelSupplierContract(channel.Id, contract.Id), ErrSupplierContractInactive)
	require.NoError(t, InactivateUpstreamSupplier(contract.SupplierId))
	require.ErrorIs(t, CreateSupplierContract(&SupplierContract{SupplierId: contract.SupplierId, Name: "late", ContractNo: "late"}), ErrSupplierInactive)
}

func TestSupplierAdminInactivateSupplierRejectsIllegalBindingOnInactiveContract(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-illegal-binding")
	contract := createSupplierContractFixture(t, db, "illegal-binding-supplier", "illegal-binding-contract")
	_, err := CreateAndActivateSupplierContractRateVersion(contract.Id, 650_000, 1, "initial")
	require.NoError(t, err)
	channel := Channel{Name: "illegal bound", Key: "key", Status: common.ChannelStatusEnabled, SupplierContractId: &contract.Id}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Model(&SupplierContract{}).Where("id = ?", contract.Id).UpdateColumn("status", SupplierContractStatusInactive).Error)
	require.NoError(t, db.Model(&UpstreamSupplier{}).Where("id = ?", contract.SupplierId).UpdateColumn("status", SupplierStatusInactive).Error)

	require.ErrorIs(t, InactivateUpstreamSupplier(contract.SupplierId), ErrSupplierHasChannelBindings)
	require.ErrorIs(t, InactivateSupplierContract(contract.Id), ErrSupplierContractBound)
	require.NoError(t, UnbindChannelSupplierContract(channel.Id))
	require.NoError(t, InactivateUpstreamSupplier(contract.SupplierId))
}

func TestSupplierAdminConcurrentRateAdvancePreservesEveryVersion(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-concurrent-rate")
	contract := createSupplierContractFixture(t, db, "concurrent-rate-supplier", "concurrent-rate-contract")
	initial, err := CreateAndActivateSupplierContractRateVersion(contract.Id, 600_000, 1, "initial")
	require.NoError(t, err)

	multipliers := []int64{650_000, 700_000}
	versions := make([]*SupplierContractRateVersion, len(multipliers))
	errs := make([]error, len(multipliers))
	var wg sync.WaitGroup
	for i := range multipliers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			versions[i], errs[i] = CreateAndActivateSupplierContractRateVersion(contract.Id, multipliers[i], i+1, "advance")
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	require.NotEqual(t, versions[0].Id, versions[1].Id)
	require.NotEqual(t, initial.Id, versions[0].Id)

	var count int64
	require.NoError(t, db.Model(&SupplierContractRateVersion{}).Where("contract_id = ?", contract.Id).Count(&count).Error)
	require.Equal(t, int64(3), count)
	var persisted SupplierContract
	require.NoError(t, db.First(&persisted, contract.Id).Error)
	require.NotNil(t, persisted.CurrentRateVersionId)
	require.Contains(t, []int{versions[0].Id, versions[1].Id}, *persisted.CurrentRateVersionId)
}

func TestSupplierAdminAppendOnlyCommandsAreIdempotentByPayload(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-idempotency")
	contract := createSupplierContractFixture(t, db, "idempotent-supplier", "idempotent-contract")
	_, err := CreateAndActivateSupplierContractRateVersion(contract.Id, 650_000, 1, "initial")
	require.NoError(t, err)

	first, err := CreateSupplierInventoryAdjustment(&SupplierInventoryAdjustment{ContractId: contract.Id, DeltaMicroUsd: 200_000_000_000, Type: SupplierInventoryAdjustmentTypeReplenishment, Reason: "add stock", IdempotencyKey: "inventory-1", CreatedBy: 1})
	require.NoError(t, err)
	replayed, err := CreateSupplierInventoryAdjustment(&SupplierInventoryAdjustment{ContractId: contract.Id, DeltaMicroUsd: 200_000_000_000, Type: SupplierInventoryAdjustmentTypeReplenishment, Reason: "add stock", IdempotencyKey: "inventory-1", CreatedBy: 1})
	require.NoError(t, err)
	require.Equal(t, first.Id, replayed.Id)
	_, err = CreateSupplierInventoryAdjustment(&SupplierInventoryAdjustment{ContractId: contract.Id, DeltaMicroUsd: 1, Type: SupplierInventoryAdjustmentTypeReplenishment, Reason: "different", IdempotencyKey: "inventory-1", CreatedBy: 1})
	require.ErrorIs(t, err, ErrSupplierIdempotencyConflict)

	rule, err := CreateSupplierStatisticsExclusionRule(101, SupplierStatisticsActionExclude, 1, "internal", "rule-1")
	require.NoError(t, err)
	replayedRule, err := CreateSupplierStatisticsExclusionRule(101, SupplierStatisticsActionExclude, 1, "internal", "rule-1")
	require.NoError(t, err)
	require.Equal(t, rule.Id, replayedRule.Id)
	_, err = CreateSupplierStatisticsExclusionRule(102, SupplierStatisticsActionExclude, 1, "different", "rule-1")
	require.ErrorIs(t, err, ErrSupplierIdempotencyConflict)
}

func TestSupplierCacheRejectsIllegalOldBindingsAndRetainsPreviousIndex(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-cache-illegal-old-data")
	validContract := createSupplierContractFixture(t, db, "valid-cache-supplier", "valid-cache-contract")
	validRate, err := CreateAndActivateSupplierContractRateVersion(validContract.Id, 650_000, 1, "initial")
	require.NoError(t, err)
	validChannel := Channel{Name: "valid cache channel", Key: "key-1", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&validChannel).Error)
	require.NoError(t, SetChannelSupplierContractCASForActor(validChannel.Id, 0, &validContract.Id, 1))
	require.NoError(t, RefreshSupplierCache())
	validSnapshot, ok := GetSupplierCostSnapshot(validChannel.Id)
	require.True(t, ok)
	require.Equal(t, validRate.Id, validSnapshot.RateVersionId)

	inactiveContract := createSupplierContractFixture(t, db, "inactive-cache-supplier", "inactive-cache-contract")
	_, err = CreateAndActivateSupplierContractRateVersion(inactiveContract.Id, 700_000, 1, "initial")
	require.NoError(t, err)
	require.NoError(t, db.Model(&SupplierContract{}).Where("id = ?", inactiveContract.Id).UpdateColumn("status", SupplierContractStatusInactive).Error)
	inactiveChannel := Channel{Name: "inactive cache channel", Key: "key-2", Status: common.ChannelStatusEnabled, SupplierContractId: &inactiveContract.Id}
	require.NoError(t, db.Create(&inactiveChannel).Error)

	missingRateContract := createSupplierContractFixture(t, db, "missing-rate-supplier", "missing-rate-contract")
	missingRateChannel := Channel{Name: "missing rate channel", Key: "key-3", Status: common.ChannelStatusEnabled, SupplierContractId: &missingRateContract.Id}
	require.NoError(t, db.Create(&missingRateChannel).Error)

	missingContractId := 999999
	missingContractChannel := Channel{Name: "missing contract channel", Key: "key-4", Status: common.ChannelStatusEnabled, SupplierContractId: &missingContractId}
	require.NoError(t, db.Create(&missingContractChannel).Error)

	require.Error(t, RefreshSupplierCache())
	retainedSnapshot, ok := GetSupplierCostSnapshot(validChannel.Id)
	require.True(t, ok)
	require.Equal(t, validSnapshot, retainedSnapshot)
	for _, channelId := range []int{inactiveChannel.Id, missingRateChannel.Id, missingContractChannel.Id} {
		_, ok := GetSupplierCostSnapshot(channelId)
		require.False(t, ok)
	}
	health := GetSupplierCacheHealth()
	require.True(t, health.Blocking)
	require.Equal(t, 3, health.IllegalBindingCount)
	require.Len(t, health.Issues, 3)
	require.NotEmpty(t, health.RefreshError)
}

func TestSupplierAdminIndexesSupportInvariantQueries(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-indexes")
	require.Equal(t, []string{"supplier_id", "status"}, supplierSQLiteIndexColumns(t, db, "idx_supplier_contracts_supplier_status"))
	require.Equal(t, []string{"contract_id", "id"}, supplierSQLiteIndexColumns(t, db, "idx_supplier_inventory_contract_id"))
}
