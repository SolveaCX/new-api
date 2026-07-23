package model

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSupplierCoverageGapTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=busy_timeout(5000)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SupplierAccountingCoverageGap{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	return db
}

func validSupplierCoverageGapInput(commandID string, startAt int64) OpenSupplierAccountingCoverageGapInput {
	digest := "sha256:" + strings.Repeat("a", 64)
	commit := strings.Repeat("b", 40)
	affectedCapability := int64(6)
	return OpenSupplierAccountingCoverageGapInput{
		StartAt:                      startAt,
		ReasonCategory:               SupplierCoverageGapReasonLogWriteFailure,
		ReasonText:                   "consume log persistence failed",
		ExpectedCapabilityVersion:    7,
		AffectedCapabilityVersion:    &affectedCapability,
		AffectedOCIDigest:            &digest,
		AffectedBuildCommit:          &commit,
		ActivationStateVersionBefore: 10,
		ActivationStateVersionAfter:  11,
		OpenCommandID:                commandID,
		OpenedBy:                     42,
		EvidenceRefs:                 []string{"incident/INC-42", "metric/router-write-failure"},
	}
}

func TestSupplierAccountingCoverageGapSchema(t *testing.T) {
	db := setupSupplierCoverageGapTestDB(t)
	columnTypes, err := db.Migrator().ColumnTypes(&SupplierAccountingCoverageGap{})
	require.NoError(t, err)
	columns := make(map[string]struct{}, len(columnTypes))
	databaseTypes := make(map[string]string, len(columnTypes))
	for _, columnType := range columnTypes {
		columns[columnType.Name()] = struct{}{}
		databaseTypes[columnType.Name()] = strings.ToLower(columnType.DatabaseTypeName())
	}
	require.Contains(t, databaseTypes["evidence_refs"], "text")
	for _, expected := range []string{
		"id", "start_at", "end_at", "reason_category", "reason_text", "expected_capability_version",
		"affected_capability_version", "affected_oci_digest", "affected_build_commit",
		"activation_state_version_before", "activation_state_version_after", "open_command_id", "close_command_id",
		"opened_by", "closed_by", "finance_disposition", "evidence_refs", "record_version", "created_at", "updated_at",
	} {
		_, ok := columns[expected]
		require.True(t, ok, expected)
	}
	require.True(t, db.Migrator().HasIndex(&SupplierAccountingCoverageGap{}, "idx_supplier_accounting_coverage_gaps_start_at"))
	require.True(t, db.Migrator().HasIndex(&SupplierAccountingCoverageGap{}, "ux_supplier_coverage_gap_open_command"))
	require.True(t, db.Migrator().HasIndex(&SupplierAccountingCoverageGap{}, "ux_supplier_coverage_gap_close_command"))
}

func TestOpenSupplierAccountingCoverageGapIdempotencyAndValidation(t *testing.T) {
	db := setupSupplierCoverageGapTestDB(t)
	input := validSupplierCoverageGapInput("open-1", 1_700_000_000)
	opened, err := OpenSupplierAccountingCoverageGap(db, input)
	require.NoError(t, err)
	require.Positive(t, opened.Id)
	require.Equal(t, SupplierCoverageGapFinancePending, opened.FinanceDisposition)
	require.Equal(t, SupplierCoverageGapInitialRecordVersion, opened.RecordVersion)

	replayed, err := OpenSupplierAccountingCoverageGap(db, input)
	require.NoError(t, err)
	require.Equal(t, opened.Id, replayed.Id)

	conflicting := input
	conflicting.ReasonText = "different incident"
	_, err = OpenSupplierAccountingCoverageGap(db, conflicting)
	require.ErrorIs(t, err, ErrSupplierCoverageGapIdempotencyConflict)

	tests := []struct {
		name   string
		mutate func(*OpenSupplierAccountingCoverageGapInput)
	}{
		{name: "unknown reason", mutate: func(value *OpenSupplierAccountingCoverageGapInput) { value.ReasonCategory = "unknown" }},
		{name: "zero expected capability", mutate: func(value *OpenSupplierAccountingCoverageGapInput) { value.ExpectedCapabilityVersion = 0 }},
		{name: "invalid affected capability", mutate: func(value *OpenSupplierAccountingCoverageGapInput) {
			invalid := int64(0)
			value.AffectedCapabilityVersion = &invalid
		}},
		{name: "non monotonic activation version", mutate: func(value *OpenSupplierAccountingCoverageGapInput) {
			value.ActivationStateVersionAfter = value.ActivationStateVersionBefore
		}},
		{name: "malformed digest", mutate: func(value *OpenSupplierAccountingCoverageGapInput) {
			invalid := "sha256:nope"
			value.AffectedOCIDigest = &invalid
		}},
		{name: "malformed build commit", mutate: func(value *OpenSupplierAccountingCoverageGapInput) {
			invalid := "not-a-commit"
			value.AffectedBuildCommit = &invalid
		}},
		{name: "oversized evidence", mutate: func(value *OpenSupplierAccountingCoverageGapInput) {
			value.EvidenceRefs = []string{strings.Repeat("x", SupplierCoverageGapMaxEvidenceRefBytes+1)}
		}},
		{name: "malformed evidence", mutate: func(value *OpenSupplierAccountingCoverageGapInput) { value.EvidenceRefs = []string{"incident\n42"} }},
		{name: "zero opener", mutate: func(value *OpenSupplierAccountingCoverageGapInput) { value.OpenedBy = 0 }},
		{name: "negative opener", mutate: func(value *OpenSupplierAccountingCoverageGapInput) { value.OpenedBy = -1 }},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			invalid := validSupplierCoverageGapInput("invalid-"+string(rune('a'+index)), 1_700_000_100+int64(index))
			test.mutate(&invalid)
			_, err := OpenSupplierAccountingCoverageGap(db, invalid)
			require.ErrorIs(t, err, ErrSupplierCoverageGapInvalid)
		})
	}
}

func TestSupplierAccountingCoverageGapClosedRowValidation(t *testing.T) {
	db := setupSupplierCoverageGapTestDB(t)
	opened, err := OpenSupplierAccountingCoverageGap(db, validSupplierCoverageGapInput("open-closed-validation", 1_700_000_000))
	require.NoError(t, err)

	endAt := opened.StartAt + 60
	closeCommandID := "close-invalid-actor"
	zero := 0
	invalidActor := *opened
	invalidActor.Id = 0
	invalidActor.OpenCommandID = "open-invalid-closed-actor"
	invalidActor.EndAt = &endAt
	invalidActor.CloseCommandID = &closeCommandID
	invalidActor.ClosedBy = &zero
	invalidActor.FinanceDisposition = SupplierCoverageGapFinanceReconciled
	invalidActor.RecordVersion++
	require.ErrorIs(t, db.Create(&invalidActor).Error, ErrSupplierCoverageGapInvalid)

	closeCommandID = "close-invalid-pending"
	closer := 1
	invalidPending := *opened
	invalidPending.Id = 0
	invalidPending.OpenCommandID = "open-invalid-pending"
	invalidPending.EndAt = &endAt
	invalidPending.CloseCommandID = &closeCommandID
	invalidPending.ClosedBy = &closer
	invalidPending.FinanceDisposition = SupplierCoverageGapFinancePending
	invalidPending.RecordVersion++
	require.ErrorIs(t, db.Create(&invalidPending).Error, ErrSupplierCoverageGapInvalid)
}

func TestCloseSupplierAccountingCoverageGapNamedCASAndReplay(t *testing.T) {
	db := setupSupplierCoverageGapTestDB(t)
	input := validSupplierCoverageGapInput("open-close", 1_700_000_000)
	opened, err := OpenSupplierAccountingCoverageGap(db, input)
	require.NoError(t, err)

	closeInput := CloseSupplierAccountingCoverageGapInput{
		ID:                 opened.Id,
		EndAt:              opened.StartAt + 60,
		CloseCommandID:     "close-1",
		ClosedBy:           43,
		FinanceDisposition: SupplierCoverageGapFinanceReconciled,
		ExpectedVersion:    opened.RecordVersion,
	}
	closed, err := CloseSupplierAccountingCoverageGap(db, closeInput)
	require.NoError(t, err)
	require.NotNil(t, closed.EndAt)
	require.Equal(t, closeInput.EndAt, *closed.EndAt)
	require.Equal(t, opened.RecordVersion+1, closed.RecordVersion)
	require.Equal(t, opened.StartAt, closed.StartAt)
	require.Equal(t, opened.ReasonText, closed.ReasonText)
	require.Equal(t, opened.OpenCommandID, closed.OpenCommandID)
	require.Equal(t, opened.EvidenceRefs, closed.EvidenceRefs)

	replayed, err := CloseSupplierAccountingCoverageGap(db, closeInput)
	require.NoError(t, err)
	require.Equal(t, closed.Id, replayed.Id)
	require.Equal(t, closed.RecordVersion, replayed.RecordVersion)

	conflictingReplay := closeInput
	conflictingReplay.FinanceDisposition = SupplierCoverageGapFinanceAcceptedLoss
	_, err = CloseSupplierAccountingCoverageGap(db, conflictingReplay)
	require.ErrorIs(t, err, ErrSupplierCoverageGapIdempotencyConflict)

	stale := closeInput
	stale.CloseCommandID = "close-stale"
	_, err = CloseSupplierAccountingCoverageGap(db, stale)
	require.ErrorIs(t, err, ErrSupplierCoverageGapCASConflict)

	invalidTime := closeInput
	invalidTime.CloseCommandID = "close-invalid-time"
	invalidTime.EndAt = opened.StartAt
	_, err = CloseSupplierAccountingCoverageGap(db, invalidTime)
	require.ErrorIs(t, err, ErrSupplierCoverageGapInvalid)

	invalidFinance := closeInput
	invalidFinance.CloseCommandID = "close-invalid-finance"
	invalidFinance.FinanceDisposition = "unknown"
	_, err = CloseSupplierAccountingCoverageGap(db, invalidFinance)
	require.ErrorIs(t, err, ErrSupplierCoverageGapInvalid)

	pendingFinance := closeInput
	pendingFinance.CloseCommandID = "close-pending-finance"
	pendingFinance.FinanceDisposition = SupplierCoverageGapFinancePending
	_, err = CloseSupplierAccountingCoverageGap(db, pendingFinance)
	require.ErrorIs(t, err, ErrSupplierCoverageGapInvalid)

	zeroCloser := closeInput
	zeroCloser.CloseCommandID = "close-zero-actor"
	zeroCloser.ClosedBy = 0
	_, err = CloseSupplierAccountingCoverageGap(db, zeroCloser)
	require.ErrorIs(t, err, ErrSupplierCoverageGapInvalid)

	negativeCloser := closeInput
	negativeCloser.CloseCommandID = "close-negative-actor"
	negativeCloser.ClosedBy = -1
	_, err = CloseSupplierAccountingCoverageGap(db, negativeCloser)
	require.ErrorIs(t, err, ErrSupplierCoverageGapInvalid)
}

func TestCloseSupplierAccountingCoverageGapConcurrentCAS(t *testing.T) {
	db := setupSupplierCoverageGapTestDB(t)
	opened, err := OpenSupplierAccountingCoverageGap(db, validSupplierCoverageGapInput("open-concurrent", 1_700_100_000))
	require.NoError(t, err)

	start := make(chan struct{})
	errorsSeen := make(chan error, 2)
	var wait sync.WaitGroup
	for index := 0; index < 2; index++ {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, closeErr := CloseSupplierAccountingCoverageGap(db, CloseSupplierAccountingCoverageGapInput{
				ID:                 opened.Id,
				EndAt:              opened.StartAt + int64(60+index),
				CloseCommandID:     []string{"close-concurrent-a", "close-concurrent-b"}[index],
				ClosedBy:           100 + index,
				FinanceDisposition: SupplierCoverageGapFinanceNoImpact,
				ExpectedVersion:    opened.RecordVersion,
			})
			errorsSeen <- closeErr
		}()
	}
	close(start)
	wait.Wait()
	close(errorsSeen)
	var outcomes []error
	for outcome := range errorsSeen {
		outcomes = append(outcomes, outcome)
	}
	require.Len(t, outcomes, 2)
	winners := 0
	conflicts := 0
	for _, outcome := range outcomes {
		switch {
		case outcome == nil:
			winners++
		case errors.Is(outcome, ErrSupplierCoverageGapCASConflict):
			conflicts++
		default:
			require.NoError(t, outcome)
		}
	}
	require.Equal(t, 1, winners)
	require.Equal(t, 1, conflicts)
}

func TestQuerySupplierAccountingCoverageGapsOverlappingSameAndCrossDay(t *testing.T) {
	db := setupSupplierCoverageGapTestDB(t)
	dayStart := int64(1_704_067_200)
	openAndClose := func(command string, startOffset, endOffset int64) SupplierAccountingCoverageGap {
		opened, err := OpenSupplierAccountingCoverageGap(db, validSupplierCoverageGapInput("open-"+command, dayStart+startOffset))
		require.NoError(t, err)
		closed, err := CloseSupplierAccountingCoverageGap(db, CloseSupplierAccountingCoverageGapInput{
			ID:                 opened.Id,
			EndAt:              dayStart + endOffset,
			CloseCommandID:     "close-" + command,
			ClosedBy:           43,
			FinanceDisposition: SupplierCoverageGapFinanceReconciled,
			ExpectedVersion:    opened.RecordVersion,
		})
		require.NoError(t, err)
		return *closed
	}
	first := openAndClose("same-a", 9*3600, 10*3600)
	second := openAndClose("same-b", 12*3600, 13*3600)
	crossDay := openAndClose("cross-day", 23*3600+30*60, 24*3600+30*60)

	sameDay, err := QuerySupplierAccountingCoverageGapsOverlapping(db, dayStart, dayStart+24*3600)
	require.NoError(t, err)
	require.Len(t, sameDay, 3)
	require.Equal(t, []int64{first.Id, second.Id, crossDay.Id}, []int64{sameDay[0].Id, sameDay[1].Id, sameDay[2].Id})

	healthyInterval, err := QuerySupplierAccountingCoverageGapsOverlapping(db, dayStart+10*3600, dayStart+12*3600)
	require.NoError(t, err)
	require.Empty(t, healthyInterval)

	nextDay, err := QuerySupplierAccountingCoverageGapsOverlapping(db, dayStart+24*3600, dayStart+48*3600)
	require.NoError(t, err)
	require.Len(t, nextDay, 1)
	require.Equal(t, crossDay.Id, nextDay[0].Id)
}

func TestQuerySupplierAccountingCoverageGapsAllowsOverlappingEpochs(t *testing.T) {
	db := setupSupplierCoverageGapTestDB(t)
	startAt := int64(1_700_300_000)
	first, err := OpenSupplierAccountingCoverageGap(db, validSupplierCoverageGapInput("overlap-a", startAt))
	require.NoError(t, err)
	second, err := OpenSupplierAccountingCoverageGap(db, validSupplierCoverageGapInput("overlap-b", startAt+10))
	require.NoError(t, err)

	gaps, err := QuerySupplierAccountingCoverageGapsOverlapping(db, startAt+20, startAt+30)
	require.NoError(t, err)
	require.Len(t, gaps, 2)
	require.Equal(t, []int64{first.Id, second.Id}, []int64{gaps[0].Id, gaps[1].Id})
}

func TestSupplierAccountingCoverageGapUniqueCommandIndexes(t *testing.T) {
	db := setupSupplierCoverageGapTestDB(t)
	first, err := OpenSupplierAccountingCoverageGap(db, validSupplierCoverageGapInput("unique-open-a", 1_700_200_000))
	require.NoError(t, err)
	second, err := OpenSupplierAccountingCoverageGap(db, validSupplierCoverageGapInput("unique-open-b", 1_700_200_100))
	require.NoError(t, err)

	_, err = CloseSupplierAccountingCoverageGap(db, CloseSupplierAccountingCoverageGapInput{
		ID: first.Id, EndAt: first.StartAt + 10, CloseCommandID: "unique-close", ClosedBy: 1,
		FinanceDisposition: SupplierCoverageGapFinanceNoImpact, ExpectedVersion: first.RecordVersion,
	})
	require.NoError(t, err)
	_, err = CloseSupplierAccountingCoverageGap(db, CloseSupplierAccountingCoverageGapInput{
		ID: second.Id, EndAt: second.StartAt + 10, CloseCommandID: "unique-close", ClosedBy: 1,
		FinanceDisposition: SupplierCoverageGapFinanceNoImpact, ExpectedVersion: second.RecordVersion,
	})
	require.ErrorIs(t, err, ErrSupplierCoverageGapIdempotencyConflict)

	var ids []int64
	require.NoError(t, db.Model(&SupplierAccountingCoverageGap{}).Order("id").Pluck("id", &ids).Error)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	require.Equal(t, []int64{first.Id, second.Id}, ids)
}
