package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSupplierAccountingReadinessDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Option{},
		&model.SupplierAdminCommand{},
		&model.SupplierInventoryAdjustment{},
	))
	model.DB = db
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", "")
	t.Cleanup(func() { model.DB = originalDB })
	return db
}

func persistSupplierAccountingArmedState(t *testing.T, db *gorm.DB, cutoverAt int64) {
	t.Helper()
	preparedAt := cutoverAt - 60
	preparedBy := 7
	state := model.SupplierAccountingActivationState{
		SchemaVersion:              1,
		StateVersion:               3,
		Phase:                      model.SupplierAccountingActivationArmed,
		CutoverAt:                  &cutoverAt,
		AcceptedCapabilityVersions: []int{1},
		PreparedAt:                 &preparedAt,
		PreparedBy:                 &preparedBy,
		Reason:                     "readiness assertion fixture",
	}
	encoded, err := common.Marshal(state)
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Option{Key: model.SupplierAccountingActivationOptionKey, Value: string(encoded)}).Error)
}

func TestCheckSupplierAccountingReadinessSyntheticDisabledDoesNotPersistState(t *testing.T) {
	db := setupSupplierAccountingReadinessDB(t)
	require.NoError(t, CheckSupplierAccountingReadiness())

	var optionCount int64
	require.NoError(t, db.Model(&model.Option{}).Count(&optionCount).Error)
	require.Zero(t, optionCount)
}

func TestCheckSupplierAccountingReadinessRequiresFinalizedLedgerOnlyWhenGateEnabled(t *testing.T) {
	const (
		legacyScopeIndex     = "ux_supplier_admin_command_scope_key"
		legacyActorIndex     = "idx_supplier_admin_command_actor_scope_key"
		legacyInventoryIndex = "ux_supplier_inventory_contract_idempotency"
		digestIndex          = "ux_supplier_admin_command_actor_scope_digest"
	)
	bridgeLedger := func(t *testing.T, db *gorm.DB) {
		t.Helper()
		require.NoError(t, db.AutoMigrate(&model.SupplierAdminCommand{}))
		require.NoError(t, db.Exec("CREATE UNIQUE INDEX "+legacyScopeIndex+" ON supplier_admin_commands (scope, idempotency_key)").Error)
		require.NoError(t, db.Exec("CREATE INDEX "+legacyActorIndex+" ON supplier_admin_commands (actor_id, scope, idempotency_key)").Error)
		require.NoError(t, db.Exec("CREATE UNIQUE INDEX "+legacyInventoryIndex+" ON supplier_inventory_adjustments (contract_id, idempotency_key)").Error)
		require.NoError(t, model.MigrateSupplierAdminCommandLedger(db))
	}
	enableGate := func(t *testing.T, db *gorm.DB) {
		t.Helper()
		_, err := model.CASSupplierAccountingMutationState(db, 0, true, 7, "readiness enabled fixture", 1_800_000_000)
		require.NoError(t, err)
	}

	t.Run("disabled legacy bridge remains ready", func(t *testing.T) {
		db := setupSupplierAccountingReadinessDB(t)
		bridgeLedger(t, db)
		status, err := model.GetSupplierAdminCommandLedgerMigrationStatus(db)
		require.NoError(t, err)
		require.Equal(t, model.SupplierAdminCommandLedgerStateBridge, status.State())
		require.NoError(t, CheckSupplierAccountingReadiness())
	})

	t.Run("enabled legacy bridge fails closed", func(t *testing.T) {
		db := setupSupplierAccountingReadinessDB(t)
		bridgeLedger(t, db)
		enableGate(t, db)
		require.ErrorIs(t, CheckSupplierAccountingReadiness(), model.ErrSupplierAdminCommandLedgerNotFinalized)
	})

	t.Run("enabled null digest fails closed", func(t *testing.T) {
		db := setupSupplierAccountingReadinessDB(t)
		bridgeLedger(t, db)
		require.NoError(t, model.FinalizeSupplierAdminCommandLedgerMigration(db))
		enableGate(t, db)
		command := model.SupplierAdminCommand{
			ActorId: 7, Scope: "activation.transition", IdempotencyKey: "null-digest", PayloadVersion: 1,
			PayloadDigest: fmt.Sprintf("%064x", 1), ResourceType: "activation", ResourceId: 1, ClaimToken: fmt.Sprintf("%032x", 1),
		}
		require.NoError(t, db.Create(&command).Error)
		require.NoError(t, db.Exec("UPDATE supplier_admin_commands SET idempotency_key_digest = NULL WHERE id = ?", command.Id).Error)
		status, err := model.GetSupplierAdminCommandLedgerMigrationStatus(db)
		require.NoError(t, err)
		require.Equal(t, model.SupplierAdminCommandLedgerStateInvalid, status.State())
		require.ErrorIs(t, CheckSupplierAccountingReadiness(), model.ErrSupplierAdminCommandLedgerNotFinalized)
	})

	t.Run("enabled malformed digest index fails closed", func(t *testing.T) {
		db := setupSupplierAccountingReadinessDB(t)
		bridgeLedger(t, db)
		require.NoError(t, model.FinalizeSupplierAdminCommandLedgerMigration(db))
		enableGate(t, db)
		require.NoError(t, db.Exec("DROP INDEX "+digestIndex).Error)
		require.NoError(t, db.Exec("CREATE UNIQUE INDEX "+digestIndex+" ON supplier_admin_commands (scope, actor_id, idempotency_key_digest)").Error)
		status, err := model.GetSupplierAdminCommandLedgerMigrationStatus(db)
		require.NoError(t, err)
		require.Equal(t, model.SupplierAdminCommandLedgerStateInvalid, status.State())
		require.ErrorIs(t, CheckSupplierAccountingReadiness(), model.ErrSupplierAdminCommandLedgerNotFinalized)
	})

	t.Run("enabled finalized ledger is ready", func(t *testing.T) {
		db := setupSupplierAccountingReadinessDB(t)
		bridgeLedger(t, db)
		require.NoError(t, model.FinalizeSupplierAdminCommandLedgerMigration(db))
		enableGate(t, db)
		status, err := model.GetSupplierAdminCommandLedgerMigrationStatus(db)
		require.NoError(t, err)
		require.Equal(t, model.SupplierAdminCommandLedgerStateFinalized, status.State())
		require.NoError(t, CheckSupplierAccountingReadiness())
	})
}

func TestCheckSupplierAccountingReadinessRejectsMalformedStrictOptions(t *testing.T) {
	for _, key := range []string{model.SupplierAccountingActivationOptionKey, model.SupplierAccountingMutationOptionKey} {
		t.Run(key, func(t *testing.T) {
			db := setupSupplierAccountingReadinessDB(t)
			require.NoError(t, db.Create(&model.Option{Key: key, Value: `{}`}).Error)
			err := CheckSupplierAccountingReadiness()
			require.ErrorIs(t, err, model.ErrSupplierAccountingOptionMalformed)
		})
	}
}

func TestCheckSupplierAccountingReadinessCutoverAssertion(t *testing.T) {
	t.Run("invalid env", func(t *testing.T) {
		for _, value := range []string{"not-a-timestamp", "0", "-1"} {
			t.Run(value, func(t *testing.T) {
				setupSupplierAccountingReadinessDB(t)
				t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", value)
				require.ErrorContains(t, CheckSupplierAccountingReadiness(), "invalid SUPPLIER_ACCOUNTING_CUTOVER_AT")
			})
		}
	})

	t.Run("missing persisted cutover", func(t *testing.T) {
		setupSupplierAccountingReadinessDB(t)
		t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", "1800000000")
		require.ErrorContains(t, CheckSupplierAccountingReadiness(), "requires a persisted activation cutover")
	})

	t.Run("mismatch and equality", func(t *testing.T) {
		db := setupSupplierAccountingReadinessDB(t)
		const cutoverAt = int64(1_800_000_000)
		persistSupplierAccountingArmedState(t, db, cutoverAt)

		t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", fmt.Sprint(cutoverAt+1))
		require.ErrorContains(t, CheckSupplierAccountingReadiness(), "mismatch")
		t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", fmt.Sprint(cutoverAt))
		require.NoError(t, CheckSupplierAccountingReadiness())
	})
}

func TestLegacyCoverageInitializerIsReadOnlyUpgradeEvidence(t *testing.T) {
	db := setupSupplierAccountingReadinessDB(t)
	const legacyCutover = int64(1_790_000_000)
	require.NoError(t, db.Create(&model.Option{Key: model.SupplierAccountingCoverageStartOptionKey, Value: fmt.Sprint(legacyCutover)}).Error)

	read, err := InitializeSupplierAccountingCoverageStart(context.Background(), db)
	require.NoError(t, err)
	require.Equal(t, legacyCutover, read)
	require.NoError(t, CheckSupplierAccountingReadiness(), "legacy evidence does not activate accounting")

	var optionCount int64
	require.NoError(t, db.Model(&model.Option{}).Count(&optionCount).Error)
	require.EqualValues(t, 1, optionCount)

	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", fmt.Sprint(legacyCutover))
	require.ErrorContains(t, CheckSupplierAccountingReadiness(), "requires a persisted activation cutover")
}
