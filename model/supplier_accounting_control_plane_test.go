package model

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSupplierAccountingControlPlaneTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=busy_timeout(10000)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}, &SupplierAdminCommand{}, &SupplierAccountingCoverageGap{}))
	require.NoError(t, MigrateSupplierAdminCommandLedger(db))
	require.NoError(t, FinalizeSupplierAdminCommandLedgerMigration(db))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	previousDB := DB
	DB = db
	t.Cleanup(func() {
		DB = previousDB
		_ = sqlDB.Close()
	})
	return db
}

func controlCommand(actor int, key string, version int64, reason string) SupplierAccountingControlCommand {
	return SupplierAccountingControlCommand{ActorID: actor, IdempotencyKey: key, ExpectedStateVersion: version, Reason: reason}
}

func adoptLegacyAccountingForTest(t *testing.T, db *gorm.DB) SupplierAccountingActivationState {
	t.Helper()
	legacyCutover := time.Now().Unix() - 3600
	require.NoError(t, db.Create(&Option{Key: SupplierAccountingCoverageStartOptionKey, Value: fmt.Sprint(legacyCutover)}).Error)
	result, err := AdoptLegacySupplierAccounting(SupplierAccountingLegacyAdoptionInput{
		SupplierAccountingControlCommand: controlCommand(7, "adopt-legacy", 0, "adopt proven legacy coverage"),
		AcceptedCapabilityVersions:       []int{1},
	})
	require.NoError(t, err)
	require.NotNil(t, result.Activation)
	require.Equal(t, legacyCutover, *result.Activation.CutoverAt)
	return *result.Activation
}

func TestSupplierAccountingPrepareArmDisableAndReplay(t *testing.T) {
	db := setupSupplierAccountingControlPlaneTestDB(t)
	prepare := SupplierAccountingPrepareInput{
		SupplierAccountingControlCommand: controlCommand(7, "prepare-1", 0, "prepare shadow rollout"),
		AcceptedCapabilityVersions:       []int{2, 1},
	}
	first, err := PrepareSupplierAccounting(prepare)
	require.NoError(t, err)
	require.Equal(t, SupplierAccountingActivationShadow, first.Activation.Phase)
	require.Equal(t, []int{1, 2}, first.Activation.AcceptedCapabilityVersions)

	replay, err := PrepareSupplierAccounting(prepare)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, first.Activation, replay.Activation)

	conflict := prepare
	conflict.Reason = "different payload"
	_, err = PrepareSupplierAccounting(conflict)
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyConflict)

	arm, err := ArmSupplierAccounting(SupplierAccountingArmInput{
		SupplierAccountingControlCommand: controlCommand(7, "arm-1", 1, "arm future cutover"),
		CutoverAt:                        time.Now().Unix() + 300,
		AcceptedCapabilityVersions:       []int{1, 2},
	})
	require.NoError(t, err)
	require.Equal(t, SupplierAccountingActivationArmed, arm.Activation.Phase)

	disabled, err := DisableSupplierAccountingBeforeCutover(controlCommand(7, "disable-1", 2, "cancel before cutover"))
	require.NoError(t, err)
	require.Equal(t, SupplierAccountingActivationDisabled, disabled.Activation.Phase)
	require.Equal(t, int64(3), disabled.Activation.StateVersion)

	status, err := GetSupplierAccountingControlStatus()
	require.NoError(t, err)
	require.Equal(t, disabled.Activation.StateVersion, status.Activation.StateVersion)
	require.Nil(t, status.LegacyCutoverAt)
	require.Empty(t, status.UnresolvedGaps)

	var commandCount int64
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Count(&commandCount).Error)
	require.Equal(t, int64(3), commandCount, "replay and conflict must not create extra commands")
}

func TestSupplierAccountingActivateAndCommandCompletionRollback(t *testing.T) {
	t.Run("activate armed cutover", func(t *testing.T) {
		db := setupSupplierAccountingControlPlaneTestDB(t)
		now := time.Now().Unix()
		armed := activationState(1, SupplierAccountingActivationArmed, now)
		armed.CutoverAt = int64Pointer(now - 1)
		encoded, err := common.Marshal(armed)
		require.NoError(t, err)
		require.NoError(t, db.Create(&Option{Key: SupplierAccountingActivationOptionKey, Value: string(encoded)}).Error)

		result, err := ActivateSupplierAccounting(controlCommand(7, "activate-1", 1, "activate verified rollout"))
		require.NoError(t, err)
		require.Equal(t, SupplierAccountingActivationActive, result.Activation.Phase)
		require.Equal(t, int64(2), result.Activation.StateVersion)
		require.NotNil(t, result.Activation.ActivatedAt)
	})

	t.Run("completion failure rolls back option and claim", func(t *testing.T) {
		db := setupSupplierAccountingControlPlaneTestDB(t)
		const callback = "supplier_control_plane_fail_command_completion"
		require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
			if tx.Statement.Table == "supplier_admin_commands" {
				tx.AddError(errors.New("injected command completion failure"))
			}
		}))
		t.Cleanup(func() { _ = db.Callback().Update().Remove(callback) })

		_, err := PrepareSupplierAccounting(SupplierAccountingPrepareInput{
			SupplierAccountingControlCommand: controlCommand(7, "prepare-rollback", 0, "rollback final stage"),
			AcceptedCapabilityVersions:       []int{1},
		})
		require.ErrorContains(t, err, "injected command completion failure")
		var optionCount, commandCount int64
		require.NoError(t, db.Model(&Option{}).Count(&optionCount).Error)
		require.NoError(t, db.Model(&SupplierAdminCommand{}).Count(&commandCount).Error)
		require.Zero(t, optionCount)
		require.Zero(t, commandCount)
	})
}

func TestSupplierAccountingMutationGateRollbackABAReplayAndConcurrentFirstInsert(t *testing.T) {
	db := setupSupplierAccountingControlPlaneTestDB(t)
	firstInput := SupplierAccountingMutationGateInput{
		SupplierAccountingControlCommand: controlCommand(9, "gate-enable", 0, "enable mutations"), Enabled: true,
	}
	first, err := ToggleSupplierAccountingMutationGate(firstInput)
	require.NoError(t, err)
	require.True(t, first.Mutation.Enabled)
	replay, err := ToggleSupplierAccountingMutationGate(firstInput)
	require.NoError(t, err)
	require.True(t, replay.Replayed)

	second, err := ToggleSupplierAccountingMutationGate(SupplierAccountingMutationGateInput{
		SupplierAccountingControlCommand: controlCommand(9, "gate-disable", 1, "disable mutations"), Enabled: false,
	})
	require.NoError(t, err)
	require.False(t, second.Mutation.Enabled)
	_, err = ToggleSupplierAccountingMutationGate(SupplierAccountingMutationGateInput{
		SupplierAccountingControlCommand: controlCommand(9, "gate-stale-aba", 0, "stale after ABA"), Enabled: true,
	})
	require.ErrorIs(t, err, ErrSupplierAccountingOptionConflict)

	require.NoError(t, db.Exec("DELETE FROM supplier_admin_commands").Error)
	require.NoError(t, db.Exec("DELETE FROM options").Error)
	const callers = 24
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, callErr := ToggleSupplierAccountingMutationGate(SupplierAccountingMutationGateInput{
				SupplierAccountingControlCommand: controlCommand(100+index, fmt.Sprintf("gate-race-%02d", index), 0, "concurrent first insert"), Enabled: true,
			})
			errs <- callErr
		}(i)
	}
	wg.Wait()
	close(errs)
	successes := 0
	for callErr := range errs {
		if callErr == nil {
			successes++
			continue
		}
		require.ErrorIs(t, callErr, ErrSupplierAccountingOptionConflict)
	}
	require.Equal(t, 1, successes)
	var commandCount int64
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Count(&commandCount).Error)
	require.Equal(t, int64(1), commandCount, "losing transactions must roll back their command claims")
}

func TestSupplierAccountingMutationGateRequiresFinalizedCommandLedgerOnlyWhenEnabling(t *testing.T) {
	db := setupSupplierAccountingControlPlaneTestDB(t)
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX "+legacySupplierAdminCommandScopeKeyIndex+" ON supplier_admin_commands (scope, idempotency_key)").Error)

	_, err := ToggleSupplierAccountingMutationGate(SupplierAccountingMutationGateInput{
		SupplierAccountingControlCommand: controlCommand(9, "gate-enable-before-finalize", 0, "enable before ledger finalization"), Enabled: true,
	})
	require.ErrorIs(t, err, ErrSupplierAdminCommandLedgerNotFinalized)
	var commandCount int64
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Count(&commandCount).Error)
	require.Zero(t, commandCount, "failed enable must roll back its command claim")

	disabled, err := ToggleSupplierAccountingMutationGate(SupplierAccountingMutationGateInput{
		SupplierAccountingControlCommand: controlCommand(9, "gate-disable-before-finalize", 0, "recovery disable remains available"), Enabled: false,
	})
	require.NoError(t, err)
	require.False(t, disabled.Mutation.Enabled)

	require.NoError(t, FinalizeSupplierAdminCommandLedgerMigration(db))
	enableInput := SupplierAccountingMutationGateInput{
		SupplierAccountingControlCommand: controlCommand(9, "gate-enable-after-finalize", 1, "enable after ledger finalization"), Enabled: true,
	}
	enabled, err := ToggleSupplierAccountingMutationGate(enableInput)
	require.NoError(t, err)
	require.True(t, enabled.Mutation.Enabled)
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Count(&commandCount).Error)
	commandsAfterEnable := commandCount

	require.NoError(t, db.Exec("CREATE UNIQUE INDEX "+legacySupplierAdminCommandScopeKeyIndex+" ON supplier_admin_commands (scope, idempotency_key)").Error)
	_, err = ToggleSupplierAccountingMutationGate(enableInput)
	require.ErrorIs(t, err, ErrSupplierAdminCommandLedgerNotFinalized, "enable replay must revalidate before reading the old command")
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Count(&commandCount).Error)
	require.Equal(t, commandsAfterEnable, commandCount)
	require.NoError(t, FinalizeSupplierAdminCommandLedgerMigration(db))

	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Model(&SupplierAdminCommand{}).
		Where("idempotency_key = ?", enableInput.IdempotencyKey).UpdateColumn("idempotency_key_digest", []byte("wrong")).Error)
	_, err = ToggleSupplierAccountingMutationGate(enableInput)
	require.ErrorIs(t, err, ErrSupplierAdminCommandLedgerNotFinalized, "enable replay must fail closed on later ledger corruption")
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Count(&commandCount).Error)
	require.Equal(t, commandsAfterEnable, commandCount)

	recoveryDisable, err := ToggleSupplierAccountingMutationGate(SupplierAccountingMutationGateInput{
		SupplierAccountingControlCommand: controlCommand(9, "gate-disable-corrupt-ledger", 2, "disable despite ledger corruption"), Enabled: false,
	})
	require.NoError(t, err)
	require.False(t, recoveryDisable.Mutation.Enabled)
}

func TestSupplierAccountingConcurrentPrepareHasOneAtomicWinner(t *testing.T) {
	db := setupSupplierAccountingControlPlaneTestDB(t)
	const callers = 24
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := PrepareSupplierAccounting(SupplierAccountingPrepareInput{
				SupplierAccountingControlCommand: controlCommand(200+index, fmt.Sprintf("prepare-race-%02d", index), 0, "concurrent prepare"),
				AcceptedCapabilityVersions:       []int{1},
			})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		require.ErrorIs(t, err, ErrSupplierAccountingOptionConflict)
	}
	require.Equal(t, 1, successes)
	var optionCount, commandCount int64
	require.NoError(t, db.Model(&Option{}).Count(&optionCount).Error)
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Count(&commandCount).Error)
	require.Equal(t, int64(1), optionCount)
	require.Equal(t, int64(1), commandCount)
}

func TestSupplierAccountingDegradeRollbackMultiGapResolveAndSeparateReactivate(t *testing.T) {
	db := setupSupplierAccountingControlPlaneTestDB(t)
	active := adoptLegacyAccountingForTest(t, db)

	invalidCases := []struct {
		key        string
		startAt    int64
		category   string
		capability int64
	}{
		{key: "invalid-category", startAt: time.Now().Unix(), category: "not_a_reason", capability: 1},
		{key: "future-start", startAt: time.Now().Unix() + 60, category: SupplierCoverageGapReasonOperatorDeclared, capability: 1},
		{key: "pre-cutover-start", startAt: *active.CutoverAt - 1, category: SupplierCoverageGapReasonOperatorDeclared, capability: 1},
		{key: "unaccepted-capability", startAt: time.Now().Unix(), category: SupplierCoverageGapReasonOperatorDeclared, capability: 99},
	}
	for _, testCase := range invalidCases {
		_, err := DegradeSupplierAccounting(SupplierAccountingDegradeInput{
			SupplierAccountingControlCommand: controlCommand(7, testCase.key, active.StateVersion, "invalid gap must roll back activation"),
			StartAt:                          testCase.startAt, ReasonCategory: testCase.category, ExpectedCapabilityVersion: testCase.capability,
		})
		require.ErrorIs(t, err, ErrSupplierCoverageGapInvalid, testCase.key)
		rolledBack, readErr := ReadSupplierAccountingActivationState(db)
		require.NoError(t, readErr)
		require.Equal(t, active, rolledBack)
	}
	var invalidCommands int64
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Where("scope = ?", SupplierAccountingCommandScopeDegrade).Count(&invalidCommands).Error)
	require.Zero(t, invalidCommands)

	start := time.Now().Unix() - 100
	first, err := DegradeSupplierAccounting(SupplierAccountingDegradeInput{
		SupplierAccountingControlCommand: controlCommand(7, "degrade-a", 1, "first known gap"),
		StartAt:                          start, ReasonCategory: SupplierCoverageGapReasonLogWriteFailure, ExpectedCapabilityVersion: 1,
		EvidenceRefs: []string{"incident://a"},
	})
	require.NoError(t, err)
	require.Equal(t, SupplierAccountingActivationDegraded, first.Activation.Phase)

	second, err := DegradeSupplierAccounting(SupplierAccountingDegradeInput{
		SupplierAccountingControlCommand: controlCommand(7, "degrade-b", 2, "second known gap"),
		StartAt:                          start + 40, ReasonCategory: SupplierCoverageGapReasonEmergencyRollback, ExpectedCapabilityVersion: 1,
		EvidenceRefs: []string{"incident://b"},
	})
	require.NoError(t, err)
	require.NotEqual(t, first.Gap.Id, second.Gap.Id)

	_, err = ReactivateSupplierAccounting(controlCommand(7, "reactivate-too-early", 3, "must not activate with gaps"))
	require.ErrorIs(t, err, ErrSupplierAccountingCoverageUnresolved)
	_, err = ResolveSupplierAccountingGap(SupplierAccountingResolveGapInput{
		SupplierAccountingControlCommand: controlCommand(7, "resolve-future", 3, "future end is invalid"),
		GapID:                            first.Gap.Id, ExpectedGapVersion: 1, EndAt: time.Now().Unix() + 60, FinanceDisposition: SupplierCoverageGapFinanceReconciled,
	})
	require.ErrorIs(t, err, ErrSupplierCoverageGapInvalid)

	resolvedFirst, err := ResolveSupplierAccountingGap(SupplierAccountingResolveGapInput{
		SupplierAccountingControlCommand: controlCommand(7, "resolve-a", 3, "finance reconciled first gap"),
		GapID:                            first.Gap.Id, ExpectedGapVersion: 1, EndAt: start + 20, FinanceDisposition: SupplierCoverageGapFinanceReconciled,
	})
	require.NoError(t, err)
	require.Equal(t, SupplierAccountingActivationDegraded, resolvedFirst.Activation.Phase)
	require.Equal(t, int64(4), resolvedFirst.Activation.StateVersion)

	_, err = ResolveSupplierAccountingGap(SupplierAccountingResolveGapInput{
		SupplierAccountingControlCommand: controlCommand(7, "resolve-pending", 4, "pending is not resolution"),
		GapID:                            second.Gap.Id, ExpectedGapVersion: 1, EndAt: start + 80, FinanceDisposition: SupplierCoverageGapFinancePending,
	})
	require.ErrorIs(t, err, ErrSupplierAccountingCommandInvalid)

	resolvedSecond, err := ResolveSupplierAccountingGap(SupplierAccountingResolveGapInput{
		SupplierAccountingControlCommand: controlCommand(7, "resolve-b", 4, "finance accepted second gap"),
		GapID:                            second.Gap.Id, ExpectedGapVersion: 1, EndAt: start + 80, FinanceDisposition: SupplierCoverageGapFinanceAcceptedLoss,
	})
	require.NoError(t, err)
	require.Equal(t, SupplierAccountingActivationDegraded, resolvedSecond.Activation.Phase)
	require.Equal(t, int64(5), resolvedSecond.Activation.StateVersion)

	replay, err := ResolveSupplierAccountingGap(SupplierAccountingResolveGapInput{
		SupplierAccountingControlCommand: controlCommand(7, "resolve-b", 4, "finance accepted second gap"),
		GapID:                            second.Gap.Id, ExpectedGapVersion: 1, EndAt: start + 80, FinanceDisposition: SupplierCoverageGapFinanceAcceptedLoss,
	})
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, resolvedSecond.Gap, replay.Gap)

	reactivated, err := ReactivateSupplierAccounting(controlCommand(7, "reactivate", 5, "all gaps resolved"))
	require.NoError(t, err)
	require.Equal(t, SupplierAccountingActivationActive, reactivated.Activation.Phase)
	require.Equal(t, int64(6), reactivated.Activation.StateVersion)

	status, err := GetSupplierAccountingControlStatus()
	require.NoError(t, err)
	require.Empty(t, status.UnresolvedGaps)
	require.NotNil(t, status.LegacyCutoverAt)
}

func TestSupplierAccountingResolveCASConflictRollsBackActivationAndCommand(t *testing.T) {
	db := setupSupplierAccountingControlPlaneTestDB(t)
	adoptLegacyAccountingForTest(t, db)
	degraded, err := DegradeSupplierAccounting(SupplierAccountingDegradeInput{
		SupplierAccountingControlCommand: controlCommand(7, "degrade-cas", 1, "gap for CAS test"),
		StartAt:                          time.Now().Unix() - 20, ReasonCategory: SupplierCoverageGapReasonOperatorDeclared, ExpectedCapabilityVersion: 1,
	})
	require.NoError(t, err)

	_, err = ResolveSupplierAccountingGap(SupplierAccountingResolveGapInput{
		SupplierAccountingControlCommand: controlCommand(7, "resolve-stale", 2, "stale gap version"),
		GapID:                            degraded.Gap.Id, ExpectedGapVersion: 99, EndAt: time.Now().Unix(), FinanceDisposition: SupplierCoverageGapFinanceNoImpact,
	})
	require.ErrorIs(t, err, ErrSupplierCoverageGapCASConflict)
	current, readErr := ReadSupplierAccountingActivationState(db)
	require.NoError(t, readErr)
	require.Equal(t, int64(2), current.StateVersion)
	var gap SupplierAccountingCoverageGap
	require.NoError(t, db.First(&gap, degraded.Gap.Id).Error)
	require.Nil(t, gap.EndAt)
	var commandCount int64
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Where("scope = ? AND idempotency_key = ?", SupplierAccountingCommandScopeResolveGap, "resolve-stale").Count(&commandCount).Error)
	require.Zero(t, commandCount)

	_, err = ResolveSupplierAccountingGap(SupplierAccountingResolveGapInput{
		SupplierAccountingControlCommand: controlCommand(7, "resolve-stale", 2, "changed payload"),
		GapID:                            degraded.Gap.Id, ExpectedGapVersion: 1, EndAt: time.Now().Unix(), FinanceDisposition: SupplierCoverageGapFinanceNoImpact,
	})
	require.False(t, errors.Is(err, ErrSupplierAdminIdempotencyConflict), "rolled-back claims must not poison retries")
}
