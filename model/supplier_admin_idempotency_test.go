package model

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSupplierAdminCreateSupplierIdempotentReplayConflictAndConcurrency(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-command-supplier")

	const callers = 8
	results := make([]*UpstreamSupplier, callers)
	replayed := make([]bool, callers)
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], replayed[i], errs[i] = CreateUpstreamSupplierIdempotent(&UpstreamSupplier{Name: "  durable supplier  ", Remark: " durable remark "}, "supplier-command-1")
		}(i)
	}
	wg.Wait()
	createdCount := 0
	for i := range errs {
		require.NoError(t, errs[i])
		require.NotNil(t, results[i])
		require.Equal(t, results[0].Id, results[i].Id)
		if !replayed[i] {
			createdCount++
		}
	}
	require.Equal(t, 1, createdCount)
	require.Equal(t, "durable supplier", results[0].Name)
	require.Equal(t, "durable remark", results[0].Remark)

	var supplierCount int64
	require.NoError(t, db.Model(&UpstreamSupplier{}).Where("name = ?", "durable supplier").Count(&supplierCount).Error)
	require.Equal(t, int64(1), supplierCount)
	var commandCount int64
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Where("scope = ? AND idempotency_key = ?", SupplierAdminCommandScopeCreateSupplier, "supplier-command-1").Count(&commandCount).Error)
	require.Equal(t, int64(1), commandCount)

	_, _, err := CreateUpstreamSupplierIdempotent(&UpstreamSupplier{Name: "different supplier"}, "supplier-command-1")
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyConflict)
	_, _, err = CreateUpstreamSupplierIdempotent(&UpstreamSupplier{Name: "missing key"}, "")
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyKeyRequired)
}

func TestSupplierAdminCreateContractIdempotentUsesScopeAndPayload(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-command-contract")
	supplier, replayed, err := CreateUpstreamSupplierIdempotent(&UpstreamSupplier{Name: "contract command supplier"}, "shared-key")
	require.NoError(t, err)
	require.False(t, replayed)

	input := &SupplierContract{SupplierId: supplier.Id, Name: " contract ", ContractNo: " contract-001 ", Remark: " remark ", RpmLimit: 10, TpmLimit: 20, MaxConcurrency: 3}
	contract, replayed, err := CreateSupplierContractIdempotent(input, "shared-key")
	require.NoError(t, err)
	require.False(t, replayed, "the same key is independent across command scopes")
	require.Equal(t, "contract", contract.Name)
	require.Equal(t, "contract-001", contract.ContractNo)

	replayedContract, replayed, err := CreateSupplierContractIdempotent(&SupplierContract{SupplierId: supplier.Id, Name: "contract", ContractNo: "contract-001", Remark: "remark", RpmLimit: 10, TpmLimit: 20, MaxConcurrency: 3}, "shared-key")
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, contract.Id, replayedContract.Id)

	_, _, err = CreateSupplierContractIdempotent(&SupplierContract{SupplierId: supplier.Id, Name: "changed", ContractNo: "contract-001"}, "shared-key")
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyConflict)
	var count int64
	require.NoError(t, db.Model(&SupplierContract{}).Where("supplier_id = ?", supplier.Id).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestSupplierAdminCreateRateReplayReturnsOriginalWithoutReactivatingIt(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-command-rate")
	contract := createSupplierContractFixture(t, db, "rate command supplier", "rate-command-contract")

	first, replayed, err := CreateAndActivateSupplierContractRateVersionIdempotent(contract.Id, 650_000, 1, " first ", "rate-command-1")
	require.NoError(t, err)
	require.False(t, replayed)
	second, replayed, err := CreateAndActivateSupplierContractRateVersionIdempotent(contract.Id, 700_000, 1, "second", "rate-command-2")
	require.NoError(t, err)
	require.False(t, replayed)
	require.NotEqual(t, first.Id, second.Id)

	replayedFirst, replayed, err := CreateAndActivateSupplierContractRateVersionIdempotent(contract.Id, 650_000, 1, "first", "rate-command-1")
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, first.Id, replayedFirst.Id)
	var persisted SupplierContract
	require.NoError(t, db.First(&persisted, contract.Id).Error)
	require.NotNil(t, persisted.CurrentRateVersionId)
	require.Equal(t, second.Id, *persisted.CurrentRateVersionId, "replaying an old command must not roll back the current rate")

	_, _, err = CreateAndActivateSupplierContractRateVersionIdempotent(contract.Id, 600_000, 1, "different", "rate-command-1")
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyConflict)
	var count int64
	require.NoError(t, db.Model(&SupplierContractRateVersion{}).Where("contract_id = ?", contract.Id).Count(&count).Error)
	require.Equal(t, int64(2), count)
}

func TestSupplierAdminCommandAndDomainCreateRollbackTogether(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-command-crash")
	payload := createUpstreamSupplierCommandPayload{Name: "crash-safe supplier", Remark: ""}
	digest, err := supplierAdminPayloadDigest(payload)
	require.NoError(t, err)
	injected := errors.New("injected crash before command completion")
	err = db.Transaction(func(tx *gorm.DB) error {
		claim, err := claimSupplierAdminCommand(tx, 0, SupplierAdminCommandScopeCreateSupplier, "crash-command-1", digest, supplierAdminCommandResourceSupplier)
		if err != nil {
			return err
		}
		require.True(t, claim.Claimed)
		if err := tx.Create(&UpstreamSupplier{Name: payload.Name}).Error; err != nil {
			return err
		}
		return injected
	})
	require.ErrorIs(t, err, injected)

	var count int64
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Model(&UpstreamSupplier{}).Where("name = ?", payload.Name).Count(&count).Error)
	require.Zero(t, count)

	supplier, replayed, err := CreateUpstreamSupplierIdempotent(&UpstreamSupplier{Name: payload.Name}, "crash-command-1")
	require.NoError(t, err)
	require.False(t, replayed)
	require.Positive(t, supplier.Id)
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestSupplierAdminCommandLedgerIsAppendOnlyAndIndexed(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-command-ledger")
	_, _, err := CreateUpstreamSupplierIdempotent(&UpstreamSupplier{Name: "ledger supplier"}, "ledger-command-1")
	require.NoError(t, err)
	var command SupplierAdminCommand
	require.NoError(t, db.First(&command).Error)
	require.Positive(t, command.ResourceId)
	require.ErrorIs(t, db.Model(&command).Update("payload_digest", "different").Error, ErrSupplierAppendOnly)
	require.ErrorIs(t, db.Delete(&command).Error, ErrSupplierAppendOnly)
	require.Equal(t, []string{"scope", "idempotency_key"}, supplierSQLiteIndexColumns(t, db, "ux_supplier_admin_command_scope_key"))
	require.Equal(t, []string{"actor_id", "scope", "idempotency_key"}, supplierSQLiteIndexColumns(t, db, "idx_supplier_admin_command_actor_scope_key"))
	require.Equal(t, []string{"resource_type", "resource_id"}, supplierSQLiteIndexColumns(t, db, "idx_supplier_admin_command_resource"))
}

func TestGetSupplierAdminCommandResultIsExactAndActorScoped(t *testing.T) {
	setupSupplierTestDB(t, "supplier-admin-command-result")
	created, replayed, err := CreateUpstreamSupplierIdempotentForActor(&UpstreamSupplier{Name: "lookup supplier"}, "lookup-key", 7)
	require.NoError(t, err)
	require.False(t, replayed)

	result, err := GetSupplierAdminCommandResult(7, SupplierAdminCommandScopeCreateSupplier, " lookup-key ")
	require.NoError(t, err)
	require.Equal(t, SupplierAdminCommandScopeCreateSupplier, result.Scope)
	require.Equal(t, "lookup-key", result.IdempotencyKey)
	require.Equal(t, supplierAdminCommandResourceSupplier, result.ResourceType)
	require.Equal(t, created.Id, result.ResourceId)
	require.Positive(t, result.CreatedAt)

	_, err = GetSupplierAdminCommandResult(8, SupplierAdminCommandScopeCreateSupplier, "lookup-key")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound, "another administrator must not discover the command")
	_, err = GetSupplierAdminCommandResult(7, SupplierAdminCommandScopeCreateContract, "lookup-key")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound, "a coincident key in another scope is not the same command")
	_, err = GetSupplierAdminCommandResult(7, SupplierAdminCommandScopeCreateSupplier, "missing-key")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = GetSupplierAdminCommandResult(7, "supplier.invalid", "lookup-key")
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyKeyRequired)
	_, err = GetSupplierAdminCommandResult(7, SupplierAdminCommandScopeCreateSupplier, "")
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyKeyRequired)
}

func TestSupplierAdminCommandActorMismatchCannotReplayOrLeakPayload(t *testing.T) {
	setupSupplierTestDB(t, "supplier-admin-command-actor-conflict")
	_, _, err := CreateUpstreamSupplierIdempotentForActor(&UpstreamSupplier{Name: "actor seven supplier"}, "shared-actor-key", 7)
	require.NoError(t, err)
	_, _, err = CreateUpstreamSupplierIdempotentForActor(&UpstreamSupplier{Name: "actor eight supplier"}, "shared-actor-key", 8)
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyConflict)
	_, err = GetSupplierAdminCommandResult(8, SupplierAdminCommandScopeCreateSupplier, "shared-actor-key")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestGetSupplierAdminCommandResultCoversInventoryAndExclusionLedgers(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-admin-command-append-ledgers")
	contract := createSupplierContractFixture(t, db, "append result supplier", "append-result-contract")
	adjustment := &SupplierInventoryAdjustment{
		ContractId: contract.Id, DeltaMicroUsd: 200_000_000, Type: SupplierInventoryAdjustmentTypeReplenishment,
		IdempotencyKey: "inventory-result-key", CreatedBy: 7,
	}
	require.NoError(t, db.Create(adjustment).Error)
	rule, err := CreateSupplierStatisticsExclusionRule(99, SupplierStatisticsActionExclude, 7, "company account", "exclusion-result-key")
	require.NoError(t, err)

	inventoryScope := SupplierInventoryCommandScope(contract.Id)
	inventoryResult, err := GetSupplierAdminCommandResult(7, inventoryScope, "inventory-result-key")
	require.NoError(t, err)
	require.Equal(t, inventoryScope, inventoryResult.Scope)
	require.Equal(t, supplierAdminCommandResourceInventory, inventoryResult.ResourceType)
	require.Equal(t, adjustment.Id, inventoryResult.ResourceId)

	exclusionResult, err := GetSupplierAdminCommandResult(7, SupplierAdminCommandScopeCreateExclusion, "exclusion-result-key")
	require.NoError(t, err)
	require.Equal(t, supplierAdminCommandResourceExclusion, exclusionResult.ResourceType)
	require.Equal(t, rule.Id, exclusionResult.ResourceId)

	_, err = GetSupplierAdminCommandResult(7, SupplierInventoryCommandScope(contract.Id+1), "inventory-result-key")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = GetSupplierAdminCommandResult(8, inventoryScope, "inventory-result-key")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = GetSupplierAdminCommandResult(8, SupplierAdminCommandScopeCreateExclusion, "exclusion-result-key")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	for _, malformed := range []string{SupplierAdminCommandScopeCreateInventory, SupplierAdminCommandScopeCreateInventory + "/0", SupplierAdminCommandScopeCreateInventory + "/01", SupplierAdminCommandScopeCreateInventory + "/not-a-number"} {
		_, err = GetSupplierAdminCommandResult(7, malformed, "inventory-result-key")
		require.ErrorIs(t, err, ErrSupplierAdminIdempotencyKeyRequired, malformed)
	}
}

func TestGetSupplierAdminCommandResultCoversCommandLedgerScopes(t *testing.T) {
	setupSupplierTestDB(t, "supplier-admin-command-result-ledger-scopes")
	supplier, _, err := CreateUpstreamSupplierIdempotentForActor(&UpstreamSupplier{Name: "all scopes supplier"}, "all-scopes-supplier", 7)
	require.NoError(t, err)
	contract, _, err := CreateSupplierContractIdempotentForActor(&SupplierContract{SupplierId: supplier.Id, Name: "all scopes contract", ContractNo: "all-scopes-contract"}, "all-scopes-contract", 7)
	require.NoError(t, err)
	rate, _, err := CreateAndActivateSupplierContractRateVersionIdempotent(contract.Id, 650_000, 7, "initial rate", "all-scopes-rate")
	require.NoError(t, err)

	tests := []struct {
		scope        string
		key          string
		resourceType string
		resourceId   int
	}{
		{SupplierAdminCommandScopeCreateSupplier, "all-scopes-supplier", supplierAdminCommandResourceSupplier, supplier.Id},
		{SupplierAdminCommandScopeCreateContract, "all-scopes-contract", supplierAdminCommandResourceContract, contract.Id},
		{SupplierAdminCommandScopeCreateRate, "all-scopes-rate", supplierAdminCommandResourceRate, rate.Id},
	}
	for _, test := range tests {
		result, err := GetSupplierAdminCommandResult(7, test.scope, test.key)
		require.NoError(t, err, test.scope)
		require.Equal(t, test.resourceType, result.ResourceType)
		require.Equal(t, test.resourceId, result.ResourceId)
	}
}
