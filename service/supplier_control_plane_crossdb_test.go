package service

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const supplierControlPlaneConcurrentCallers = 24

func TestSupplierAccountingControlPlaneCrossDBConcurrency(t *testing.T) {
	testCases := []struct {
		name             string
		dialect          string
		dsnEnv           string
		expectedDatabase string
	}{
		{name: "sqlite", dialect: "sqlite"},
		{name: "mysql", dialect: "mysql", dsnEnv: "TEST_MYSQL_DSN", expectedDatabase: "supplier_g009_mysql"},
		{name: "postgres", dialect: "postgres", dsnEnv: "TEST_POSTGRES_DSN", expectedDatabase: "supplier_g009_postgres"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var dialector gorm.Dialector
			switch testCase.dialect {
			case "sqlite":
				path := filepath.Join(t.TempDir(), "supplier_g001_sqlite.db")
				dialector = sqlite.Open("file:" + path + "?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)")
			case "mysql":
				dsn := strings.TrimSpace(os.Getenv(testCase.dsnEnv))
				if dsn == "" {
					t.Skipf("set %s to run the isolated %s concurrency matrix", testCase.dsnEnv, testCase.name)
				}
				dialector = mysql.Open(dsn)
			case "postgres":
				dsn := strings.TrimSpace(os.Getenv(testCase.dsnEnv))
				if dsn == "" {
					t.Skipf("set %s to run the isolated %s concurrency matrix", testCase.dsnEnv, testCase.name)
				}
				dialector = postgres.Open(dsn)
			default:
				t.Fatalf("unsupported supplier accounting test dialect %q", testCase.dialect)
			}

			db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			if testCase.expectedDatabase != "" {
				requireIsolatedSupplierDatabase(t, db, testCase.dialect, testCase.expectedDatabase)
			}
			sqlDB, err := db.DB()
			require.NoError(t, err)
			maxOpenConnections := supplierControlPlaneConcurrentCallers
			if testCase.dialect == "sqlite" {
				// SQLite has one physical writer. The same 24 goroutines still race
				// through the pool, while one connection avoids SQLITE_BUSY noise.
				maxOpenConnections = 1
			}
			sqlDB.SetMaxOpenConns(maxOpenConnections)
			sqlDB.SetMaxIdleConns(maxOpenConnections)

			resetSupplierControlPlaneConcurrencyTables(t, db)
			require.NoError(t, db.AutoMigrate(
				&model.Option{},
				&model.SupplierAdminCommand{},
				&model.SupplierAccountingCoverageGap{},
			))
			require.NoError(t, model.MigrateSupplierAdminCommandLedger(db), "bridge must precede test-only finalization")
			require.NoError(t, model.FinalizeSupplierAdminCommandLedgerMigration(db), "test database explicitly finalizes after bridge")
			require.NoError(t, model.ValidateSupplierAdminCommandLedgerFinalized(db))

			previousDB := model.DB
			model.DB = db
			t.Cleanup(func() {
				model.DB = previousDB
				resetSupplierControlPlaneConcurrencyTables(t, db)
				require.NoError(t, sqlDB.Close())
			})

			runSupplierControlPlaneConcurrencyMatrix(t, db, testCase.dialect)
		})
	}
}

func resetSupplierControlPlaneConcurrencyTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Migrator().DropTable(
		&model.SupplierAccountingCoverageGap{},
		&model.SupplierAdminCommand{},
		&model.Option{},
	))
}

func runSupplierControlPlaneConcurrencyMatrix(t *testing.T, db *gorm.DB, dialect string) {
	t.Helper()

	initialMutations := make([]model.SupplierAccountingMutationGateInput, supplierControlPlaneConcurrentCallers)
	for index := range initialMutations {
		initialMutations[index] = model.SupplierAccountingMutationGateInput{
			SupplierAccountingControlCommand: supplierCrossDBControlCommand(101, fmt.Sprintf("g001-mutation-first-%02d", index), 0, "enable mutation gate from synthetic state"),
			Enabled:                          true,
		}
	}
	mutationResults, mutationErrors := runConcurrentSupplierControlCalls(func(index int) (*model.SupplierAccountingControlResult, error) {
		return model.ToggleSupplierAccountingMutationGate(initialMutations[index])
	})
	mutationWinner, mutationCommit := requireSingleSupplierControlCASCommit(t, mutationResults, mutationErrors, model.ErrSupplierAccountingOptionConflict)
	require.NotNil(t, mutationCommit.Mutation)
	require.Equal(t, int64(1), mutationCommit.Mutation.StateVersion)
	require.True(t, mutationCommit.Mutation.Enabled)
	requireSupplierControlReplayWave(t, func(_ int) (*model.SupplierAccountingControlResult, error) {
		return model.ToggleSupplierAccountingMutationGate(initialMutations[mutationWinner])
	})

	conflictingMutation := initialMutations[mutationWinner]
	conflictingMutation.Enabled = false
	_, err := model.ToggleSupplierAccountingMutationGate(conflictingMutation)
	require.ErrorIs(t, err, model.ErrSupplierAdminIdempotencyConflict)

	toggleMutations := make([]model.SupplierAccountingMutationGateInput, supplierControlPlaneConcurrentCallers)
	for index := range toggleMutations {
		toggleMutations[index] = model.SupplierAccountingMutationGateInput{
			SupplierAccountingControlCommand: supplierCrossDBControlCommand(101, fmt.Sprintf("g001-mutation-toggle-%02d", index), 1, "disable mutation gate from version one"),
			Enabled:                          false,
		}
	}
	toggleResults, toggleErrors := runConcurrentSupplierControlCalls(func(index int) (*model.SupplierAccountingControlResult, error) {
		return model.ToggleSupplierAccountingMutationGate(toggleMutations[index])
	})
	toggleWinner, toggleCommit := requireSingleSupplierControlCASCommit(t, toggleResults, toggleErrors, model.ErrSupplierAccountingOptionConflict)
	require.NotNil(t, toggleCommit.Mutation)
	require.Equal(t, int64(2), toggleCommit.Mutation.StateVersion)
	require.False(t, toggleCommit.Mutation.Enabled)
	requireSupplierControlReplayWave(t, func(_ int) (*model.SupplierAccountingControlResult, error) {
		return model.ToggleSupplierAccountingMutationGate(toggleMutations[toggleWinner])
	})
	mutationState, err := model.ReadSupplierAccountingMutationState(db)
	require.NoError(t, err)
	require.Equal(t, int64(2), mutationState.StateVersion)
	require.False(t, mutationState.Enabled)
	requireCompletedSupplierCommandCount(t, db, model.SupplierAccountingCommandScopeMutationGate, 2)

	prepareInputs := make([]model.SupplierAccountingPrepareInput, supplierControlPlaneConcurrentCallers)
	for index := range prepareInputs {
		prepareInputs[index] = model.SupplierAccountingPrepareInput{
			SupplierAccountingControlCommand: supplierCrossDBControlCommand(201, fmt.Sprintf("g001-prepare-first-%02d", index), 0, "prepare activation from synthetic state"),
			AcceptedCapabilityVersions:       []int{1},
		}
	}
	prepareResults, prepareErrors := runConcurrentSupplierControlCalls(func(index int) (*model.SupplierAccountingControlResult, error) {
		return model.PrepareSupplierAccounting(prepareInputs[index])
	})
	prepareWinner, prepareCommit := requireSingleSupplierControlCASCommit(t, prepareResults, prepareErrors, model.ErrSupplierAccountingOptionConflict)
	require.NotNil(t, prepareCommit.Activation)
	require.Equal(t, model.SupplierAccountingActivationShadow, prepareCommit.Activation.Phase)
	require.Equal(t, int64(1), prepareCommit.Activation.StateVersion)
	requireSupplierControlReplayWave(t, func(_ int) (*model.SupplierAccountingControlResult, error) {
		return model.PrepareSupplierAccounting(prepareInputs[prepareWinner])
	})

	conflictingPrepare := prepareInputs[prepareWinner]
	conflictingPrepare.AcceptedCapabilityVersions = []int{1, 2}
	_, err = model.PrepareSupplierAccounting(conflictingPrepare)
	require.ErrorIs(t, err, model.ErrSupplierAdminIdempotencyConflict)

	dbNow := supplierControlPlaneDBUnix(t, db, dialect)
	cutoverAt := dbNow + 3
	armInputs := make([]model.SupplierAccountingArmInput, supplierControlPlaneConcurrentCallers)
	for index := range armInputs {
		armInputs[index] = model.SupplierAccountingArmInput{
			SupplierAccountingControlCommand: supplierCrossDBControlCommand(201, fmt.Sprintf("g001-arm-version-one-%02d", index), 1, "arm one canonical cutover"),
			CutoverAt:                        cutoverAt,
			AcceptedCapabilityVersions:       []int{1},
		}
	}
	armResults, armErrors := runConcurrentSupplierControlCalls(func(index int) (*model.SupplierAccountingControlResult, error) {
		return model.ArmSupplierAccounting(armInputs[index])
	})
	armWinner, armCommit := requireSingleSupplierControlCASCommit(t, armResults, armErrors, model.ErrSupplierAccountingOptionConflict)
	require.NotNil(t, armCommit.Activation)
	require.Equal(t, model.SupplierAccountingActivationArmed, armCommit.Activation.Phase)
	require.Equal(t, int64(2), armCommit.Activation.StateVersion)
	require.Equal(t, cutoverAt, *armCommit.Activation.CutoverAt)
	requireSupplierControlReplayWave(t, func(_ int) (*model.SupplierAccountingControlResult, error) {
		return model.ArmSupplierAccounting(armInputs[armWinner])
	})
	requireCompletedSupplierCommandCount(t, db, model.SupplierAccountingCommandScopePrepare, 1)
	requireCompletedSupplierCommandCount(t, db, model.SupplierAccountingCommandScopeArm, 1)

	waitForSupplierControlPlaneDBUnix(t, db, dialect, cutoverAt+1)
	activated, err := model.ActivateSupplierAccounting(supplierCrossDBControlCommand(201, "g001-activate", 2, "activate after canonical cutover"))
	require.NoError(t, err)
	require.Equal(t, model.SupplierAccountingActivationActive, activated.Activation.Phase)
	require.Equal(t, int64(3), activated.Activation.StateVersion)

	degradeInputs := make([]model.SupplierAccountingDegradeInput, supplierControlPlaneConcurrentCallers)
	for index := range degradeInputs {
		degradeInputs[index] = model.SupplierAccountingDegradeInput{
			SupplierAccountingControlCommand: supplierCrossDBControlCommand(301, fmt.Sprintf("g001-degrade-%02d", index), 3, "open one named accounting gap"),
			StartAt:                          cutoverAt,
			ReasonCategory:                   model.SupplierCoverageGapReasonOperatorDeclared,
			ExpectedCapabilityVersion:        1,
			EvidenceRefs:                     []string{"incident://g001/concurrency"},
		}
	}
	degradeResults, degradeErrors := runConcurrentSupplierControlCalls(func(index int) (*model.SupplierAccountingControlResult, error) {
		return model.DegradeSupplierAccounting(degradeInputs[index])
	})
	degradeWinner, degradeCommit := requireSingleSupplierControlCASCommit(t, degradeResults, degradeErrors, model.ErrSupplierAccountingOptionConflict)
	require.NotNil(t, degradeCommit.Activation)
	require.NotNil(t, degradeCommit.Gap)
	require.Equal(t, model.SupplierAccountingActivationDegraded, degradeCommit.Activation.Phase)
	require.Equal(t, int64(4), degradeCommit.Activation.StateVersion)
	require.NotEmpty(t, degradeCommit.Gap.OpenCommandID)
	requireSupplierControlReplayWave(t, func(_ int) (*model.SupplierAccountingControlResult, error) {
		return model.DegradeSupplierAccounting(degradeInputs[degradeWinner])
	})

	conflictingDegrade := degradeInputs[degradeWinner]
	conflictingDegrade.Reason = "conflicting degrade payload"
	_, err = model.DegradeSupplierAccounting(conflictingDegrade)
	require.ErrorIs(t, err, model.ErrSupplierAdminIdempotencyConflict)

	var gaps []model.SupplierAccountingCoverageGap
	require.NoError(t, db.Order("id ASC").Find(&gaps).Error)
	require.Len(t, gaps, 1, "degrade replays and losing CAS contenders must not duplicate gaps")
	require.Equal(t, degradeCommit.Gap.Id, gaps[0].Id)
	requireCompletedSupplierCommandCount(t, db, model.SupplierAccountingCommandScopeDegrade, 1)

	closeAt := supplierControlPlaneDBUnix(t, db, dialect)
	require.Greater(t, closeAt, gaps[0].StartAt)
	resolveInputs := make([]model.SupplierAccountingResolveGapInput, supplierControlPlaneConcurrentCallers)
	for index := range resolveInputs {
		resolveInputs[index] = model.SupplierAccountingResolveGapInput{
			SupplierAccountingControlCommand: supplierCrossDBControlCommand(301, fmt.Sprintf("g001-resolve-%02d", index), 4, "close the named gap without implicit reactivation"),
			GapID:                            gaps[0].Id,
			ExpectedGapVersion:               model.SupplierCoverageGapInitialRecordVersion,
			EndAt:                            closeAt,
			FinanceDisposition:               model.SupplierCoverageGapFinanceNoImpact,
		}
	}
	resolveResults, resolveErrors := runConcurrentSupplierControlCalls(func(index int) (*model.SupplierAccountingControlResult, error) {
		return model.ResolveSupplierAccountingGap(resolveInputs[index])
	})
	resolveWinner, resolveCommit := requireSingleSupplierControlCASCommit(t, resolveResults, resolveErrors, model.ErrSupplierCoverageGapCASConflict)
	require.NotNil(t, resolveCommit.Activation)
	require.NotNil(t, resolveCommit.Gap)
	require.Equal(t, model.SupplierAccountingActivationDegraded, resolveCommit.Activation.Phase)
	require.Equal(t, int64(5), resolveCommit.Activation.StateVersion)
	require.NotNil(t, resolveCommit.Gap.CloseCommandID)
	require.NotEmpty(t, *resolveCommit.Gap.CloseCommandID)
	require.Equal(t, model.SupplierCoverageGapInitialRecordVersion+1, resolveCommit.Gap.RecordVersion)
	requireSupplierControlReplayWave(t, func(_ int) (*model.SupplierAccountingControlResult, error) {
		return model.ResolveSupplierAccountingGap(resolveInputs[resolveWinner])
	})

	conflictingResolve := resolveInputs[resolveWinner]
	conflictingResolve.FinanceDisposition = model.SupplierCoverageGapFinanceReconciled
	_, err = model.ResolveSupplierAccountingGap(conflictingResolve)
	require.ErrorIs(t, err, model.ErrSupplierAdminIdempotencyConflict)

	gaps = nil
	require.NoError(t, db.Order("id ASC").Find(&gaps).Error)
	require.Len(t, gaps, 1)
	require.NotNil(t, gaps[0].CloseCommandID)
	require.Equal(t, resolveCommit.Gap.CloseCommandID, gaps[0].CloseCommandID)
	requireCompletedSupplierCommandCount(t, db, model.SupplierAccountingCommandScopeResolveGap, 1)

	degradedState, err := model.ReadSupplierAccountingActivationState(db)
	require.NoError(t, err)
	require.Equal(t, model.SupplierAccountingActivationDegraded, degradedState.Phase, "closing the last gap must not reactivate implicitly")
	require.Equal(t, int64(5), degradedState.StateVersion)
	reactivated, err := model.ReactivateSupplierAccounting(supplierCrossDBControlCommand(301, "g001-reactivate", 5, "reactivate in an independent command"))
	require.NoError(t, err)
	require.Equal(t, model.SupplierAccountingActivationActive, reactivated.Activation.Phase)
	require.Equal(t, int64(6), reactivated.Activation.StateVersion)
	requireCompletedSupplierCommandCount(t, db, model.SupplierAccountingCommandScopeReactivate, 1)

	assertSupplierActorLocalDigestConcurrency(t, db)
	t.Logf("%s G001 concurrency matrix: callers=%d mutation=true activation=true gap_deduplicated=true close_completed=true reactivate_independent=true actor_local_digest=true", dialect, supplierControlPlaneConcurrentCallers)
}

func supplierCrossDBControlCommand(actorID int, idempotencyKey string, expectedVersion int64, reason string) model.SupplierAccountingControlCommand {
	return model.SupplierAccountingControlCommand{
		ActorID:              actorID,
		IdempotencyKey:       idempotencyKey,
		ExpectedStateVersion: expectedVersion,
		Reason:               reason,
	}
}

func runConcurrentSupplierControlCalls(call func(int) (*model.SupplierAccountingControlResult, error)) ([]*model.SupplierAccountingControlResult, []error) {
	results := make([]*model.SupplierAccountingControlResult, supplierControlPlaneConcurrentCallers)
	errs := make([]error, supplierControlPlaneConcurrentCallers)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := range supplierControlPlaneConcurrentCallers {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			results[index], errs[index] = call(index)
		}(index)
	}
	close(start)
	wait.Wait()
	return results, errs
}

func requireSingleSupplierControlCASCommit(t *testing.T, results []*model.SupplierAccountingControlResult, errs []error, expectedConflict error) (int, *model.SupplierAccountingControlResult) {
	t.Helper()
	require.Len(t, results, supplierControlPlaneConcurrentCallers)
	require.Len(t, errs, supplierControlPlaneConcurrentCallers)
	commits := 0
	conflicts := 0
	winner := -1
	var committed *model.SupplierAccountingControlResult
	for index := range results {
		if errs[index] != nil {
			require.ErrorIs(t, errs[index], expectedConflict, "concurrent caller %d returned an unexpected error", index)
			require.Nil(t, results[index], "conflicting caller %d must not return a result", index)
			conflicts++
			continue
		}
		require.NotNil(t, results[index], "winning caller %d", index)
		require.False(t, results[index].Replayed, "distinct idempotency keys cannot replay each other")
		commits++
		winner = index
		committed = results[index]
	}
	require.Equal(t, 1, commits, "only one concurrent caller may perform the eligible state transition")
	require.Equal(t, supplierControlPlaneConcurrentCallers-1, conflicts, "all losing distinct commands must return the typed CAS conflict")
	return winner, committed
}

func requireSupplierControlReplayWave(t *testing.T, call func(int) (*model.SupplierAccountingControlResult, error)) {
	t.Helper()
	results, errs := runConcurrentSupplierControlCalls(call)
	for index := range results {
		require.NoError(t, errs[index], "replay caller %d", index)
		require.NotNil(t, results[index], "replay caller %d", index)
		require.True(t, results[index].Replayed, "completed winner must replay for caller %d", index)
	}
}

func requireCompletedSupplierCommandCount(t *testing.T, db *gorm.DB, scope string, expected int) []model.SupplierAdminCommand {
	t.Helper()
	var commands []model.SupplierAdminCommand
	require.NoError(t, db.Where("scope = ?", scope).Order("id ASC").Find(&commands).Error)
	require.Len(t, commands, expected)
	for _, command := range commands {
		require.Positive(t, command.ResourceId)
		require.NotEmpty(t, command.ResultJson)
		require.Len(t, command.IdempotencyKeyDigest, 32)
	}
	return commands
}

func supplierControlPlaneDBUnix(t *testing.T, db *gorm.DB, dialect string) int64 {
	t.Helper()
	var timestamp int64
	query := "SELECT CAST(strftime('%s', 'now') AS INTEGER)"
	switch dialect {
	case "mysql":
		query = "SELECT UNIX_TIMESTAMP()"
	case "postgres":
		query = "SELECT FLOOR(EXTRACT(EPOCH FROM clock_timestamp()))::bigint"
	}
	require.NoError(t, db.Raw(query).Scan(&timestamp).Error)
	require.Positive(t, timestamp)
	return timestamp
}

func waitForSupplierControlPlaneDBUnix(t *testing.T, db *gorm.DB, dialect string, target int64) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for supplierControlPlaneDBUnix(t, db, dialect) < target {
		if time.Now().After(deadline) {
			t.Fatalf("%s database time did not reach %d before timeout", dialect, target)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

type supplierActorLocalDigestResult struct {
	ActorID int    `json:"actor_id"`
	Value   string `json:"value"`
}

type supplierActorLocalDigestOutcome struct {
	Claimed  bool
	Replayed bool
	Result   supplierActorLocalDigestResult
}

func assertSupplierActorLocalDigestConcurrency(t *testing.T, db *gorm.DB) {
	t.Helper()
	const (
		scope        = "supplier_accounting.g001_actor_digest"
		key          = "g001-shared-actor-local-key"
		resourceType = "actor_digest_probe"
	)
	actors := [2]int{401, 402}
	outcomes := make([]supplierActorLocalDigestOutcome, supplierControlPlaneConcurrentCallers)
	errs := make([]error, supplierControlPlaneConcurrentCallers)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := range supplierControlPlaneConcurrentCallers {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			actorID := actors[index%len(actors)]
			payload := supplierActorLocalDigestResult{ActorID: actorID, Value: fmt.Sprintf("actor-%d", actorID)}
			errs[index] = db.Transaction(func(tx *gorm.DB) error {
				claim, err := model.ClaimSupplierAdminCommandTx(tx, actorID, scope, key, payload, resourceType)
				if err != nil {
					return err
				}
				outcomes[index].Claimed = claim.Claimed
				outcomes[index].Replayed = claim.Replayed
				if claim.Replayed {
					return claim.DecodeResult(&outcomes[index].Result)
				}
				outcomes[index].Result = payload
				return model.CompleteSupplierAdminCommandTx(tx, claim, actorID, payload)
			})
		}(index)
	}
	close(start)
	wait.Wait()

	winners := 0
	replays := 0
	for index := range outcomes {
		require.NoError(t, errs[index], "actor-local caller %d", index)
		if outcomes[index].Claimed {
			winners++
		} else if outcomes[index].Replayed {
			replays++
		}
		expectedActor := actors[index%len(actors)]
		require.Equal(t, expectedActor, outcomes[index].Result.ActorID)
		require.Equal(t, fmt.Sprintf("actor-%d", expectedActor), outcomes[index].Result.Value)
	}
	require.Equal(t, len(actors), winners, "the same digest must have one independent winner per actor")
	require.Equal(t, supplierControlPlaneConcurrentCallers-len(actors), replays)

	commands := requireCompletedSupplierCommandCount(t, db, scope, len(actors))
	require.NotEqual(t, commands[0].ActorId, commands[1].ActorId)
	require.True(t, bytes.Equal(commands[0].IdempotencyKeyDigest, commands[1].IdempotencyKeyDigest), "the digest is shared while actor_id provides isolation")

	conflictingPayload := supplierActorLocalDigestResult{ActorID: actors[0], Value: "conflicting-payload"}
	err := db.Transaction(func(tx *gorm.DB) error {
		_, claimErr := model.ClaimSupplierAdminCommandTx(tx, actors[0], scope, key, conflictingPayload, resourceType)
		return claimErr
	})
	require.ErrorIs(t, err, model.ErrSupplierAdminIdempotencyConflict)
}
