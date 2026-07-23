package model

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestSupplierAndContractMutationsUseVersionCASAndExactReplay(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-mutation-cas-replay")
	supplier := UpstreamSupplier{Name: "versioned supplier"}
	require.NoError(t, CreateUpstreamSupplier(&supplier))
	require.Equal(t, int64(1), supplier.RowVersion)
	name := "renamed supplier"
	updated, replayed, err := UpdateUpstreamSupplierIdempotentForActor(supplier.Id, UpdateUpstreamSupplierInput{Name: &name, ExpectedVersion: 1}, "supplier-update-1", 7)
	require.NoError(t, err)
	require.False(t, replayed)
	require.Equal(t, int64(2), updated.RowVersion)
	replayedSupplier, replayed, err := UpdateUpstreamSupplierIdempotentForActor(supplier.Id, UpdateUpstreamSupplierInput{Name: &name, ExpectedVersion: 1}, "supplier-update-1", 7)
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, *updated, *replayedSupplier)
	otherName := "conflict"
	_, _, err = UpdateUpstreamSupplierIdempotentForActor(supplier.Id, UpdateUpstreamSupplierInput{Name: &otherName, ExpectedVersion: 1}, "supplier-update-1", 7)
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyConflict)
	_, _, err = UpdateUpstreamSupplierIdempotentForActor(supplier.Id, UpdateUpstreamSupplierInput{Name: &otherName, ExpectedVersion: 1}, "supplier-update-stale", 7)
	require.ErrorIs(t, err, ErrSupplierVersionConflict)

	contract := SupplierContract{SupplierId: supplier.Id, Name: "versioned contract", ContractNo: "VC-1"}
	require.NoError(t, CreateSupplierContract(&contract))
	require.Equal(t, int64(1), contract.RowVersion)
	remark := "updated"
	updatedContract, replayed, err := UpdateSupplierContractIdempotentForActor(contract.Id, UpdateSupplierContractInput{Remark: &remark, ExpectedVersion: 1}, "contract-update-1", 7)
	require.NoError(t, err)
	require.False(t, replayed)
	require.Equal(t, int64(2), updatedContract.RowVersion)
	replayedContract, replayed, err := UpdateSupplierContractIdempotentForActor(contract.Id, UpdateSupplierContractInput{Remark: &remark, ExpectedVersion: 1}, "contract-update-1", 7)
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, *updatedContract, *replayedContract)
	_, _, err = UpdateSupplierContractIdempotentForActor(contract.Id, UpdateSupplierContractInput{Remark: &remark, ExpectedVersion: 1}, "contract-update-stale", 7)
	require.ErrorIs(t, err, ErrSupplierVersionConflict)

	inactivatedContract, replayed, err := InactivateSupplierContractIdempotentForActor(contract.Id, 2, "contract-inactivate-1", 7)
	require.NoError(t, err)
	require.False(t, replayed)
	require.Equal(t, int64(3), inactivatedContract.RowVersion)
	replayedContract, replayed, err = InactivateSupplierContractIdempotentForActor(contract.Id, 2, "contract-inactivate-1", 7)
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, *inactivatedContract, *replayedContract)
	_, _, err = InactivateSupplierContractIdempotentForActor(contract.Id, 3, "contract-inactivate-new-key", 7)
	require.ErrorIs(t, err, ErrSupplierVersionConflict, "a new key cannot record an already-inactive no-op")

	inactivatedSupplier, replayed, err := InactivateUpstreamSupplierIdempotentForActor(supplier.Id, 2, "supplier-inactivate-1", 7)
	require.NoError(t, err)
	require.False(t, replayed)
	require.Equal(t, int64(3), inactivatedSupplier.RowVersion)
	_, replayed, err = InactivateUpstreamSupplierIdempotentForActor(supplier.Id, 2, "supplier-inactivate-1", 7)
	require.NoError(t, err)
	require.True(t, replayed)
	_, _, err = InactivateUpstreamSupplierIdempotentForActor(supplier.Id, 3, "supplier-inactivate-new-key", 7)
	require.ErrorIs(t, err, ErrSupplierVersionConflict)

	var commandCount int64
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Where("actor_id = ?", 7).Count(&commandCount).Error)
	require.Equal(t, int64(4), commandCount, "failed stale/conflicting commands roll back their claims")
}

func TestChannelBindingMutationsAreExactlyOnce(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-binding-command-replay")
	contract := createSupplierContractFixture(t, db, "binding command supplier", "binding command contract")
	_, err := CreateAndActivateSupplierContractRateVersion(contract.Id, 650_000, 7, "initial")
	require.NoError(t, err)
	channel := Channel{Name: "binding command channel", Key: "binding-key", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&channel).Error)

	bound, replayed, err := SetChannelSupplierContractCASIdempotentForActor(channel.Id, 0, &contract.Id, "bind-command", 7)
	require.NoError(t, err)
	require.False(t, replayed)
	require.NotNil(t, bound.SupplierContractId)
	replayedBinding, replayed, err := SetChannelSupplierContractCASIdempotentForActor(channel.Id, 0, &contract.Id, "bind-command", 7)
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, *bound, *replayedBinding)
	_, _, err = SetChannelSupplierContractCASIdempotentForActor(channel.Id, 0, &contract.Id, "bind-stale-new-key", 7)
	require.ErrorIs(t, err, ErrSupplierBindingChanged, "a new key cannot turn a stale expected state into a successful no-op")
	_, _, err = SetChannelSupplierContractCASIdempotentForActor(channel.Id, contract.Id, &contract.Id, "bind-command", 7)
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyConflict)

	unbound, replayed, err := SetChannelSupplierContractCASIdempotentForActor(channel.Id, contract.Id, nil, "unbind-command", 7)
	require.NoError(t, err)
	require.False(t, replayed)
	require.Nil(t, unbound.SupplierContractId)
	replayedUnbound, replayed, err := SetChannelSupplierContractCASIdempotentForActor(channel.Id, contract.Id, nil, "unbind-command", 7)
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, *unbound, *replayedUnbound)
	_, _, err = SetChannelSupplierContractCASIdempotentForActor(channel.Id, contract.Id, nil, "unbind-stale-new-key", 7)
	require.ErrorIs(t, err, ErrSupplierBindingChanged, "only exact command replay may return an already-unbound result")

	var versions int64
	require.NoError(t, db.Model(&SupplierChannelBindingVersion{}).Where("channel_id = ?", channel.Id).Count(&versions).Error)
	require.Equal(t, int64(2), versions)
}

func TestSupplierContractRateAdvanceInvalidatesStaleMutationVersion(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-rate-invalidates-version")
	contract := createSupplierContractFixture(t, db, "rate version supplier", "rate version contract")
	require.Equal(t, int64(1), contract.RowVersion)
	_, replayed, err := CreateAndActivateSupplierContractRateVersionIdempotent(contract.Id, 650_000, 7, "initial", "rate-version-command")
	require.NoError(t, err)
	require.False(t, replayed)
	var advanced SupplierContract
	require.NoError(t, db.First(&advanced, contract.Id).Error)
	require.Equal(t, int64(2), advanced.RowVersion)
	_, replayed, err = CreateAndActivateSupplierContractRateVersionIdempotent(contract.Id, 650_000, 7, "initial", "rate-version-command")
	require.NoError(t, err)
	require.True(t, replayed)
	require.NoError(t, db.First(&advanced, contract.Id).Error)
	require.Equal(t, int64(2), advanced.RowVersion, "rate replay must not increment row version")
	remark := "stale"
	_, _, err = UpdateSupplierContractIdempotentForActor(contract.Id, UpdateSupplierContractInput{Remark: &remark, ExpectedVersion: 1}, "stale-after-rate", 7)
	require.ErrorIs(t, err, ErrSupplierVersionConflict)
}

func TestInventoryAndExclusionCommandsAdoptLegacyRowsActorLocally(t *testing.T) {
	db := setupSupplierTestDB(t, "supplier-append-command-legacy-adoption")
	contract := createSupplierContractFixture(t, db, "legacy append supplier", "legacy append contract")
	_, err := CreateAndActivateSupplierContractRateVersion(contract.Id, 650_000, 7, "initial")
	require.NoError(t, err)
	legacyInventory := SupplierInventoryAdjustment{ContractId: contract.Id, DeltaMicroUsd: 10, Type: SupplierInventoryAdjustmentTypeCorrection, Reason: "legacy", IdempotencyKey: "legacy-inventory", CreatedBy: 7}
	require.NoError(t, db.Create(&legacyInventory).Error)
	adoptedInventory, replayed, err := CreateSupplierInventoryAdjustmentIdempotentForActor(&SupplierInventoryAdjustment{ContractId: contract.Id, DeltaMicroUsd: 10, Type: SupplierInventoryAdjustmentTypeCorrection, Reason: "legacy"}, "legacy-inventory", 7)
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, legacyInventory.Id, adoptedInventory.Id)
	_, _, err = CreateSupplierInventoryAdjustmentIdempotentForActor(&SupplierInventoryAdjustment{ContractId: contract.Id, DeltaMicroUsd: 11, Type: SupplierInventoryAdjustmentTypeCorrection, Reason: "changed"}, "legacy-inventory", 7)
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyConflict)
	otherActorInventory, replayed, err := CreateSupplierInventoryAdjustmentIdempotentForActor(&SupplierInventoryAdjustment{ContractId: contract.Id, DeltaMicroUsd: 12, Type: SupplierInventoryAdjustmentTypeCorrection, Reason: "other actor"}, "legacy-inventory", 8)
	require.NoError(t, err)
	require.False(t, replayed)
	require.NotEqual(t, legacyInventory.Id, otherActorInventory.Id)

	legacyExclusion := SupplierStatisticsExclusionRule{UserId: 91, Action: SupplierStatisticsActionExclude, Reason: "legacy", IdempotencyKey: "legacy-exclusion", CreatedBy: 7}
	require.NoError(t, db.Create(&legacyExclusion).Error)
	adoptedExclusion, replayed, err := CreateSupplierStatisticsExclusionRuleIdempotentForActor(91, SupplierStatisticsActionExclude, 7, "legacy", "legacy-exclusion")
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, legacyExclusion.Id, adoptedExclusion.Id)
	_, _, err = CreateSupplierStatisticsExclusionRuleIdempotentForActor(92, SupplierStatisticsActionExclude, 7, "changed", "legacy-exclusion")
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyConflict)
	otherActorExclusion, replayed, err := CreateSupplierStatisticsExclusionRuleIdempotentForActor(93, SupplierStatisticsActionExclude, 8, "other actor", "legacy-exclusion")
	require.NoError(t, err)
	require.False(t, replayed)
	require.NotEqual(t, legacyExclusion.Id, otherActorExclusion.Id)
}

type legacySupplierWithoutRowVersion struct {
	Id     int `gorm:"primaryKey"`
	Name   string
	Status string
	Remark string
}

func (legacySupplierWithoutRowVersion) TableName() string { return "upstream_suppliers" }

type legacyContractWithoutRowVersion struct {
	Id         int `gorm:"primaryKey"`
	SupplierId int
	Name       string
	ContractNo string
	Status     string
}

func (legacyContractWithoutRowVersion) TableName() string { return "supplier_contracts" }

type legacyInventoryActorIndex struct {
	Id             int `gorm:"primaryKey"`
	ContractId     int `gorm:"uniqueIndex:ux_supplier_inventory_contract_idempotency,priority:1"`
	DeltaMicroUsd  int64
	Type           string
	Reason         string
	IdempotencyKey string `gorm:"uniqueIndex:ux_supplier_inventory_contract_idempotency,priority:2"`
	CreatedBy      int
}

func (legacyInventoryActorIndex) TableName() string { return "supplier_inventory_adjustments" }

func TestSupplierMutationMigrationBackfillsVersionsAndRepairsActorIndexRerunnably(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacySupplierWithoutRowVersion{}, &legacyContractWithoutRowVersion{}, &legacyInventoryActorIndex{}, &SupplierAdminCommand{}))
	require.NoError(t, db.Create(&legacySupplierWithoutRowVersion{Id: 1, Name: "legacy", Status: SupplierStatusActive}).Error)
	require.NoError(t, db.Create(&legacyContractWithoutRowVersion{Id: 1, SupplierId: 1, Name: "legacy", ContractNo: "L-1", Status: SupplierContractStatusActive}).Error)
	require.NoError(t, db.Create(&legacyInventoryActorIndex{ContractId: 1, DeltaMicroUsd: 1, Type: SupplierInventoryAdjustmentTypeCorrection, IdempotencyKey: "legacy", CreatedBy: 7}).Error)
	for range 2 {
		require.NoError(t, MigrateSupplierAdminCommandLedger(db))
	}
	require.Equal(t, []string{"contract_id", "idempotency_key"}, supplierSQLiteIndexColumns(t, db, legacySupplierInventoryContractKeyIndex), "bridge keeps the old conflict target valid for mixed-version writers")
	require.Equal(t, []string{"contract_id", "created_by", "idempotency_key"}, supplierSQLiteIndexColumns(t, db, supplierInventoryActorLocalIndex))
	require.NoError(t, FinalizeSupplierAdminCommandLedgerMigration(db))
	var supplier UpstreamSupplier
	require.NoError(t, db.First(&supplier, 1).Error)
	require.Equal(t, int64(1), supplier.RowVersion)
	var contract SupplierContract
	require.NoError(t, db.First(&contract, 1).Error)
	require.Equal(t, int64(1), contract.RowVersion)
	require.False(t, db.Migrator().HasIndex(&SupplierInventoryAdjustment{}, legacySupplierInventoryContractKeyIndex))
	require.Equal(t, []string{"contract_id", "created_by", "idempotency_key"}, supplierSQLiteIndexColumns(t, db, supplierInventoryActorLocalIndex))
	require.LessOrEqual(t, 2*8+128*4, 767, "actor-local inventory key fits the conservative MySQL 5.7 utf8mb4 index limit")
}

func TestSupplierInventoryIndexBridgePreservesOldWriterUntilFinalization(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacyInventoryActorIndex{}, &SupplierAdminCommand{}))
	require.NoError(t, MigrateSupplierAdminCommandLedger(db))

	oldWriter := func(actorID int) error {
		row := legacyInventoryActorIndex{ContractId: 1, DeltaMicroUsd: int64(actorID), Type: SupplierInventoryAdjustmentTypeCorrection, IdempotencyKey: "mixed-version", CreatedBy: actorID}
		return db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "contract_id"}, {Name: "idempotency_key"}}, DoNothing: true}).Create(&row).Error
	}
	require.NoError(t, oldWriter(7), "the bridge retains the old ON CONFLICT target")
	require.NoError(t, oldWriter(8), "old writer replay remains valid while mixed versions are draining")
	var count int64
	require.NoError(t, db.Model(&legacyInventoryActorIndex{}).Where("contract_id = ? AND idempotency_key = ?", 1, "mixed-version").Count(&count).Error)
	require.Equal(t, int64(1), count, "the legacy stronger key remains authoritative during the bridge")
	require.NoError(t, FinalizeSupplierAdminCommandLedgerMigration(db))
	require.False(t, db.Migrator().HasIndex(&SupplierInventoryAdjustment{}, legacySupplierInventoryContractKeyIndex))
	require.NoError(t, db.Create(&legacyInventoryActorIndex{ContractId: 1, DeltaMicroUsd: 8, Type: SupplierInventoryAdjustmentTypeCorrection, IdempotencyKey: "mixed-version", CreatedBy: 8}).Error)
	require.NoError(t, db.Model(&legacyInventoryActorIndex{}).Where("contract_id = ? AND idempotency_key = ?", 1, "mixed-version").Count(&count).Error)
	require.Equal(t, int64(2), count, "post-drain finalization enables actor-local reuse")
}

func TestEnsureSupplierInventoryActorLocalIndexIsConcurrentAndCrossDialect(t *testing.T) {
	for dialect, expected := range map[string]string{
		"sqlite":   `CREATE UNIQUE INDEX "idx" ON "supplier_inventory_adjustments" ("contract_id", "created_by", "idempotency_key")`,
		"postgres": `CREATE UNIQUE INDEX "idx" ON "supplier_inventory_adjustments" ("contract_id", "created_by", "idempotency_key")`,
		"mysql":    "CREATE UNIQUE INDEX `idx` ON `supplier_inventory_adjustments` (`contract_id`, `created_by`, `idempotency_key`)",
	} {
		statement, err := supplierCreateUniqueIndexStatement(dialect, "supplier_inventory_adjustments", "idx", []string{"contract_id", "created_by", "idempotency_key"})
		require.NoError(t, err)
		require.Equal(t, expected, statement)
	}

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=busy_timeout(5000)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacyInventoryActorIndex{}))
	const workers = 8
	errs := make(chan error, workers)
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errs <- ensureSupplierInventoryActorLocalIndex(db)
		}()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, []string{"contract_id", "created_by", "idempotency_key"}, supplierSQLiteIndexColumns(t, db, supplierInventoryActorLocalIndex))
	require.True(t, db.Migrator().HasIndex(&SupplierInventoryAdjustment{}, legacySupplierInventoryContractKeyIndex), "concurrent bridge never drops the old writer constraint")
}
