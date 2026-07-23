package model

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSupplierAccountingOptionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=busy_timeout(5000)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func int64Pointer(value int64) *int64 { return &value }
func intPointer(value int) *int       { return &value }

func activationState(version int64, phase SupplierAccountingActivationPhase, now int64) SupplierAccountingActivationState {
	state := SupplierAccountingActivationState{
		SchemaVersion: supplierAccountingOptionSchemaVersion,
		StateVersion:  version,
		Phase:         phase,
		Reason:        "test transition",
	}
	switch phase {
	case SupplierAccountingActivationDisabled:
		return state
	case SupplierAccountingActivationShadow:
		state.AcceptedCapabilityVersions = []int{1}
		state.PreparedAt = int64Pointer(now - 20)
		state.PreparedBy = intPointer(7)
		return state
	case SupplierAccountingActivationArmed:
		state.AcceptedCapabilityVersions = []int{1}
		state.PreparedAt = int64Pointer(now - 20)
		state.PreparedBy = intPointer(7)
		state.CutoverAt = int64Pointer(now + 10)
		return state
	case SupplierAccountingActivationActive, SupplierAccountingActivationDegraded, SupplierAccountingActivationRetired:
		state.AcceptedCapabilityVersions = []int{1}
		state.PreparedAt = int64Pointer(now - 30)
		state.PreparedBy = intPointer(7)
		state.CutoverAt = int64Pointer(now - 20)
		state.ActivatedAt = int64Pointer(now - 20)
		if phase == SupplierAccountingActivationDegraded {
			state.DegradedAt = int64Pointer(now - 10)
		}
		return state
	default:
		return state
	}
}

func TestGenericOptionWritesRejectSupplierAccountingReservedKeys(t *testing.T) {
	db := setupSupplierAccountingOptionTestDB(t)
	originalDB := DB
	DB = db
	t.Cleanup(func() { DB = originalDB })

	for _, key := range []string{
		SupplierAccountingActivationOptionKey,
		SupplierAccountingMutationOptionKey,
		SupplierAccountingCoverageStartOptionKey,
	} {
		t.Run(key, func(t *testing.T) {
			err := UpdateOption(key, `{}`)
			require.ErrorIs(t, err, ErrSupplierAccountingReservedOption)
			var count int64
			require.NoError(t, db.Model(&Option{}).Where(map[string]any{"key": key}).Count(&count).Error)
			require.Zero(t, count)
		})
	}

	err := UpdateOptionsBulk(map[string]string{
		"SystemName":                        "unchanged",
		SupplierAccountingMutationOptionKey: `{}`,
	})
	require.ErrorIs(t, err, ErrSupplierAccountingReservedOption)
	var count int64
	require.NoError(t, db.Model(&Option{}).Count(&count).Error)
	require.Zero(t, count, "bulk validation must reject before any write")
}

func TestMutationOptionStrictParsingAndSyntheticAbsence(t *testing.T) {
	db := setupSupplierAccountingOptionTestDB(t)
	state, err := ReadSupplierAccountingMutationState(db)
	require.NoError(t, err)
	require.Equal(t, SyntheticSupplierAccountingMutationState(), state)
	var count int64
	require.NoError(t, db.Model(&Option{}).Count(&count).Error)
	require.Zero(t, count, "synthetic reads must not insert")

	valid := `{"schema_version":1,"state_version":3,"enabled":false,"updated_by":9,"updated_at":1700000000,"reason":"maintenance"}`
	parsed, err := ParseSupplierAccountingMutationState(valid)
	require.NoError(t, err)
	require.Equal(t, int64(3), parsed.StateVersion)
	require.False(t, parsed.Enabled)

	invalid := []string{
		``, `null`, `[]`, `{}`,
		`{"schema_version":2,"state_version":1,"enabled":false}`,
		`{"schema_version":1,"state_version":0,"enabled":false}`,
		`{"schema_version":1,"state_version":1}`,
		`{"schema_version":1,"state_version":1,"enabled":false,"unknown":1}`,
		`{"schema_version":1,"state_version":1,"enabled":"false"}`,
		`{"schema_version":1,"state_version":1,"enabled":false,"updated_by":0}`,
		`{"schema_version":1,"state_version":1,"enabled":false,"updated_at":-1}`,
	}
	for _, raw := range invalid {
		_, err := ParseSupplierAccountingMutationState(raw)
		require.ErrorIs(t, err, ErrSupplierAccountingOptionMalformed, raw)
	}
}

func TestActivationOptionStrictParsingAndSyntheticAbsence(t *testing.T) {
	db := setupSupplierAccountingOptionTestDB(t)
	state, err := ReadSupplierAccountingActivationState(db)
	require.NoError(t, err)
	require.Equal(t, SyntheticSupplierAccountingActivationState(), state)

	valid := `{"schema_version":1,"state_version":7,"phase":"armed","cutover_at":1785000000,"accepted_capability_versions":[1],"prepared_at":1784990000,"prepared_by":123,"activated_at":null,"degraded_at":null,"reason":"initial production rollout"}`
	parsed, err := ParseSupplierAccountingActivationState(valid)
	require.NoError(t, err)
	require.Equal(t, SupplierAccountingActivationArmed, parsed.Phase)

	invalid := []string{
		`{"schema_version":1,"state_version":7,"phase":"armed","cutover_at":1785000000,"accepted_capability_versions":[1],"prepared_at":1784990000,"prepared_by":123,"activated_at":null,"degraded_at":null,"reason":"x","extra":true}`,
		`{"schema_version":1,"state_version":7,"phase":"armed","cutover_at":1785000000,"accepted_capability_versions":[1],"prepared_at":1784990000,"prepared_by":123,"activated_at":null,"reason":"x"}`,
		`{"schema_version":1,"state_version":7,"phase":"armed","cutover_at":1785000000,"accepted_capability_versions":[1,1],"prepared_at":1784990000,"prepared_by":123,"activated_at":null,"degraded_at":null,"reason":"x"}`,
		`{"schema_version":1,"state_version":7,"phase":"unknown","cutover_at":null,"accepted_capability_versions":[],"prepared_at":null,"prepared_by":null,"activated_at":null,"degraded_at":null,"reason":"x"}`,
	}
	for _, raw := range invalid {
		_, err := ParseSupplierAccountingActivationState(raw)
		require.ErrorIs(t, err, ErrSupplierAccountingOptionMalformed, raw)
	}
}

func TestSupplierAccountingOptionStrictParsingRejectsDuplicateTopLevelFields(t *testing.T) {
	testCases := []struct {
		name  string
		raw   string
		parse func(string) error
	}{
		{
			name: "activation schema version",
			raw:  `{"schema_version":1,"schema_version":1,"state_version":7,"phase":"armed","cutover_at":1785000000,"accepted_capability_versions":[1],"prepared_at":1784990000,"prepared_by":123,"activated_at":null,"degraded_at":null,"reason":"x"}`,
			parse: func(raw string) error {
				_, err := ParseSupplierAccountingActivationState(raw)
				return err
			},
		},
		{
			name: "activation unknown field",
			raw:  `{"schema_version":1,"state_version":7,"phase":"armed","cutover_at":1785000000,"accepted_capability_versions":[1],"prepared_at":1784990000,"prepared_by":123,"activated_at":null,"degraded_at":null,"reason":"x","unknown":1,"unknown":2}`,
			parse: func(raw string) error {
				_, err := ParseSupplierAccountingActivationState(raw)
				return err
			},
		},
		{
			name: "mutation schema version",
			raw:  `{"schema_version":1,"schema_version":1,"state_version":3,"enabled":false}`,
			parse: func(raw string) error {
				_, err := ParseSupplierAccountingMutationState(raw)
				return err
			},
		},
		{
			name: "mutation enabled",
			raw:  `{"schema_version":1,"state_version":3,"enabled":false,"enabled":true}`,
			parse: func(raw string) error {
				_, err := ParseSupplierAccountingMutationState(raw)
				return err
			},
		},
		{
			name: "mutation unknown field",
			raw:  `{"schema_version":1,"state_version":3,"enabled":false,"unknown":1,"unknown":2}`,
			parse: func(raw string) error {
				_, err := ParseSupplierAccountingMutationState(raw)
				return err
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.parse(testCase.raw)
			require.ErrorIs(t, err, ErrSupplierAccountingOptionMalformed)
			require.Contains(t, err.Error(), "duplicate field")
		})
	}
}

func TestActivationTransitionMatrixAndCutoverRules(t *testing.T) {
	const now int64 = 1_800_000_000
	phases := []SupplierAccountingActivationPhase{
		SupplierAccountingActivationDisabled, SupplierAccountingActivationShadow,
		SupplierAccountingActivationArmed, SupplierAccountingActivationActive,
		SupplierAccountingActivationDegraded, SupplierAccountingActivationRetired,
	}
	allowed := map[string]bool{
		"disabled>shadow":   true,
		"shadow>armed":      true,
		"shadow>disabled":   true,
		"armed>active":      true,
		"armed>disabled":    true,
		"active>degraded":   true,
		"active>retired":    true,
		"degraded>degraded": true,
		"degraded>active":   true,
		"degraded>retired":  true,
	}
	for _, from := range phases {
		for _, to := range phases {
			current := activationState(1, from, now)
			next := activationState(2, to, now)
			if from == SupplierAccountingActivationArmed && to == SupplierAccountingActivationActive {
				next.CutoverAt = int64Pointer(now - 20)
				current.CutoverAt = int64Pointer(now - 20)
			}
			if (from == SupplierAccountingActivationActive || from == SupplierAccountingActivationDegraded) && (to == SupplierAccountingActivationActive || to == SupplierAccountingActivationDegraded || to == SupplierAccountingActivationRetired) {
				next.CutoverAt = int64Pointer(*current.CutoverAt)
			}
			err := ValidateSupplierAccountingActivationTransition(current, next, now)
			key := string(from) + ">" + string(to)
			if allowed[key] {
				require.NoError(t, err, key)
			} else {
				require.Error(t, err, key)
			}
		}
	}

	shadow := activationState(1, SupplierAccountingActivationShadow, now)
	armed := activationState(2, SupplierAccountingActivationArmed, now)
	armed.CutoverAt = int64Pointer(now)
	require.ErrorIs(t, ValidateSupplierAccountingActivationTransition(shadow, armed, now), ErrSupplierAccountingTransition)

	active := activationState(2, SupplierAccountingActivationActive, now)
	active.CutoverAt = int64Pointer(now - 21)
	currentArmed := activationState(1, SupplierAccountingActivationArmed, now)
	currentArmed.CutoverAt = int64Pointer(now - 20)
	require.ErrorIs(t, ValidateSupplierAccountingActivationTransition(currentArmed, active, now), ErrSupplierAccountingTransition)
}

func TestMutationCASRejectsStaleVersionAndABA(t *testing.T) {
	db := setupSupplierAccountingOptionTestDB(t)
	first, err := CASSupplierAccountingMutationState(db, 0, true, 7, "enable", 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), first.StateVersion)
	_, err = CASSupplierAccountingMutationState(db, 1, true, 7, "not a toggle", 101)
	require.ErrorIs(t, err, ErrSupplierAccountingMutationTransition)
	_, err = CASSupplierAccountingMutationState(db, 0, false, 7, "stale", 101)
	require.ErrorIs(t, err, ErrSupplierAccountingOptionConflict)
	second, err := CASSupplierAccountingMutationState(db, 1, false, 7, "disable", 102)
	require.NoError(t, err)
	require.Equal(t, int64(2), second.StateVersion)
	require.False(t, second.Enabled)
	_, err = CASSupplierAccountingMutationState(db, 1, true, 7, "ABA stale", 103)
	require.ErrorIs(t, err, ErrSupplierAccountingOptionConflict)
	read, err := ReadSupplierAccountingMutationState(db)
	require.NoError(t, err)
	require.Equal(t, second, read)
}

func TestConcurrentFirstMutationOptionInsertHasSingleWinner(t *testing.T) {
	db := setupSupplierAccountingOptionTestDB(t)
	const callers = 24
	var winners atomic.Int32
	var conflicts atomic.Int32
	var unexpectedMu sync.Mutex
	var unexpected []error
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := CASSupplierAccountingMutationState(db, 0, true, i+1, fmt.Sprintf("caller %d", i), 100)
			switch {
			case err == nil:
				winners.Add(1)
			case errors.Is(err, ErrSupplierAccountingOptionConflict):
				conflicts.Add(1)
			default:
				unexpectedMu.Lock()
				unexpected = append(unexpected, err)
				unexpectedMu.Unlock()
			}
		}(i)
	}
	close(start)
	wg.Wait()
	require.Empty(t, unexpected)
	require.Equal(t, int32(1), winners.Load())
	require.Equal(t, int32(callers-1), conflicts.Load())
}

func TestConcurrentMutationToggleHasSingleWinner(t *testing.T) {
	db := setupSupplierAccountingOptionTestDB(t)
	_, err := CASSupplierAccountingMutationState(db, 0, false, 1, "initialize", 99)
	require.NoError(t, err)
	const callers = 24
	var winners atomic.Int32
	var conflicts atomic.Int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := CASSupplierAccountingMutationState(db, 1, true, i+1, fmt.Sprintf("toggle %d", i), 100)
			if err == nil {
				winners.Add(1)
			} else if errors.Is(err, ErrSupplierAccountingOptionConflict) {
				conflicts.Add(1)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	require.Equal(t, int32(1), winners.Load())
	require.Equal(t, int32(callers-1), conflicts.Load())
}

func TestSupplierAccountingOptionCASComposesWithTransactionRollback(t *testing.T) {
	db := setupSupplierAccountingOptionTestDB(t)
	rollback := errors.New("rollback injected after CAS")
	err := db.Transaction(func(tx *gorm.DB) error {
		_, err := CASSupplierAccountingMutationState(tx, 0, true, 7, "enable", 100)
		require.NoError(t, err)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	state, err := ReadSupplierAccountingMutationState(db)
	require.NoError(t, err)
	require.Equal(t, SyntheticSupplierAccountingMutationState(), state)
}

func TestConcurrentFirstActivationOptionInsertHasSingleWinner(t *testing.T) {
	db := setupSupplierAccountingOptionTestDB(t)
	const callers = 24
	const now int64 = 1_800_000_000
	var winners atomic.Int32
	var conflicts atomic.Int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			desired := activationState(1, SupplierAccountingActivationShadow, now)
			_, err := CASSupplierAccountingActivationState(db, 0, desired, now)
			if err == nil {
				winners.Add(1)
			} else if errors.Is(err, ErrSupplierAccountingOptionConflict) {
				conflicts.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	require.Equal(t, int32(1), winners.Load())
	require.Equal(t, int32(callers-1), conflicts.Load())
}

func TestMalformedPersistedStateFailsClosed(t *testing.T) {
	db := setupSupplierAccountingOptionTestDB(t)
	require.NoError(t, db.Create(&Option{Key: SupplierAccountingMutationOptionKey, Value: `{"schema_version":1,"state_version":1,"enabled":false,"extra":true}`}).Error)
	_, err := ReadSupplierAccountingMutationState(db)
	require.ErrorIs(t, err, ErrSupplierAccountingOptionMalformed)
	_, err = CASSupplierAccountingMutationState(db, 1, true, 7, "must fail", 100)
	require.ErrorIs(t, err, ErrSupplierAccountingOptionMalformed)
}
