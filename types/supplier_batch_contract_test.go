package types

import (
	"math"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestSupplierPublishedEvidenceV1StrictRoundTrip(t *testing.T) {
	evidence := SupplierPublishedEvidenceV1{
		SchemaVersion: SupplierPublishedEvidenceSchemaVersion, LogsScanned: 6, ProducerMarkersPresent: 6, CapturedSnapshotCount: 1,
		DispositionCounts:                SupplierPublishedDispositionCountsV1{Captured: 1, UnsupportedPath: 1, NotFinanciallyCommitted: 1, ZeroUsage: 1, Unbound: 1, ProducerError: 1},
		PersistedLogSnapshotCompleteness: SupplierPersistedLogCompletenessIncomplete,
		Warnings:                         []SupplierPublishedWarningV1{{Code: SupplierPublishedWarningProducerError, Count: 1, MessageKey: "supply_chain.warning.producer_error"}},
	}
	raw, err := EncodeSupplierPublishedEvidenceV1(evidence)
	require.NoError(t, err)
	parsed, err := ParseSupplierPublishedEvidenceV1(raw)
	require.NoError(t, err)
	require.Equal(t, evidence, parsed)

	invalid := []string{
		strings.Replace(raw, `"warnings":`, `"unknown":1,"warnings":`, 1),
		strings.Replace(raw, `"captured":1,`, "", 1),
		strings.Replace(raw, `"producer_error":1`, `"producer_error":-1`, 1),
		raw + ` {}`,
		strings.Repeat(" ", SupplierPublishedEvidenceMaxBytes+1),
	}
	for _, value := range invalid {
		_, err = ParseSupplierPublishedEvidenceV1(value)
		require.Error(t, err)
	}
}

func TestSupplierPublishedEvidenceV1IncompleteUnclassifiedMarkers(t *testing.T) {
	evidence := SupplierPublishedEvidenceV1{
		SchemaVersion: SupplierPublishedEvidenceSchemaVersion, LogsScanned: 3, ProducerMarkersPresent: 2, CapturedSnapshotCount: 1,
		DispositionCounts:                SupplierPublishedDispositionCountsV1{Captured: 1},
		FailureCounts:                    SupplierPublishedFailureCountsV1{UnknownProducerCapability: 1, AbsentMarkerAfterCutover: 1},
		PersistedLogSnapshotCompleteness: SupplierPersistedLogCompletenessIncomplete,
		Warnings: []SupplierPublishedWarningV1{
			{Code: SupplierPublishedWarningUnknownProducer, Count: 1, MessageKey: "supply_chain.warning.unknown_producer_capability"},
			{Code: SupplierPublishedWarningAbsentMarker, Count: 1, MessageKey: "supply_chain.warning.absent_marker_after_cutover"},
		},
	}
	_, err := EncodeSupplierPublishedEvidenceV1(evidence)
	require.NoError(t, err)
}

func TestSupplierPublishedEvidenceV1RequiresExactWarningSet(t *testing.T) {
	base := SupplierPublishedEvidenceV1{
		SchemaVersion: SupplierPublishedEvidenceSchemaVersion, LogsScanned: 1, ProducerMarkersPresent: 1,
		DispositionCounts:                SupplierPublishedDispositionCountsV1{ProducerError: 1},
		PersistedLogSnapshotCompleteness: SupplierPersistedLogCompletenessIncomplete,
		Warnings: []SupplierPublishedWarningV1{{
			Code: SupplierPublishedWarningProducerError, Count: 1, MessageKey: "supply_chain.warning.producer_error",
		}},
	}
	require.NoError(t, ValidateSupplierPublishedEvidenceV1(base))

	tests := []struct {
		name   string
		mutate func(*SupplierPublishedEvidenceV1)
	}{
		{name: "missing", mutate: func(evidence *SupplierPublishedEvidenceV1) { evidence.Warnings = []SupplierPublishedWarningV1{} }},
		{name: "null", mutate: func(evidence *SupplierPublishedEvidenceV1) { evidence.Warnings = nil }},
		{name: "mismatched count", mutate: func(evidence *SupplierPublishedEvidenceV1) { evidence.Warnings[0].Count = 2 }},
		{name: "duplicate", mutate: func(evidence *SupplierPublishedEvidenceV1) {
			evidence.Warnings = append(evidence.Warnings, evidence.Warnings[0])
		}},
		{name: "extra", mutate: func(evidence *SupplierPublishedEvidenceV1) {
			evidence.Warnings = append(evidence.Warnings, SupplierPublishedWarningV1{
				Code: SupplierPublishedWarningAbsentMarker, Count: 1, MessageKey: "supply_chain.warning.absent_marker_after_cutover",
			})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidence := base
			evidence.Warnings = append([]SupplierPublishedWarningV1(nil), base.Warnings...)
			test.mutate(&evidence)
			require.ErrorIs(t, ValidateSupplierPublishedEvidenceV1(evidence), ErrInvalidSupplierPublishedEvidence)
		})
	}

	raw, err := EncodeSupplierPublishedEvidenceV1(SupplierPublishedEvidenceV1{
		SchemaVersion:                    SupplierPublishedEvidenceSchemaVersion,
		PersistedLogSnapshotCompleteness: SupplierPersistedLogCompletenessComplete,
		Warnings:                         []SupplierPublishedWarningV1{},
	})
	require.NoError(t, err)
	_, err = ParseSupplierPublishedEvidenceV1(strings.Replace(raw, `"warnings":[]`, `"warnings":null`, 1))
	require.ErrorIs(t, err, ErrInvalidSupplierPublishedEvidence)
}

func TestSupplierPublishedEvidenceV1RejectsCountOverflow(t *testing.T) {
	tests := []SupplierPublishedEvidenceV1{
		{
			SchemaVersion: SupplierPublishedEvidenceSchemaVersion, LogsScanned: math.MaxInt64, ProducerMarkersPresent: math.MaxInt64,
			CapturedSnapshotCount:            math.MaxInt64,
			DispositionCounts:                SupplierPublishedDispositionCountsV1{Captured: math.MaxInt64, UnsupportedPath: 1},
			PersistedLogSnapshotCompleteness: SupplierPersistedLogCompletenessComplete,
			Warnings:                         []SupplierPublishedWarningV1{},
		},
		{
			SchemaVersion: SupplierPublishedEvidenceSchemaVersion, LogsScanned: math.MaxInt64, ProducerMarkersPresent: math.MaxInt64,
			FailureCounts: SupplierPublishedFailureCountsV1{
				UnknownProducerCapability:      math.MaxInt64,
				IncompatibleProducerCapability: math.MaxInt64,
			},
			PersistedLogSnapshotCompleteness: SupplierPersistedLogCompletenessIncomplete,
			Warnings:                         []SupplierPublishedWarningV1{},
		},
	}
	for _, evidence := range tests {
		require.ErrorIs(t, ValidateSupplierPublishedEvidenceV1(evidence), ErrInvalidSupplierPublishedEvidence)
	}
}

func TestSupplierBatchCommandStateV1StrictTerminalContracts(t *testing.T) {
	noWork := SupplierBatchCommandStateV1{SchemaVersion: 1, State: SupplierBatchCommandStateCompleted, Response: &SupplierBatchCommandStatusV1{
		RequestID: "request-1", Status: SupplierBatchCommandStateCompleted, ErrorCategory: SupplierBatchErrorNone, Result: &SupplierBatchCommandResultV1{},
	}}
	raw, err := EncodeSupplierBatchCommandStateV1(noWork)
	require.NoError(t, err)
	parsed, err := ParseSupplierBatchCommandStateV1(raw)
	require.NoError(t, err)
	require.Equal(t, noWork, parsed)

	date, runID := "2026-07-22", int64(9)
	failed := SupplierBatchCommandStateV1{SchemaVersion: 1, State: SupplierBatchCommandStateFailed, Response: &SupplierBatchCommandStatusV1{
		RequestID: "request-2", BatchDate: &date, RunID: &runID, Status: SupplierBatchCommandStateFailed, FenceToken: 2,
		PublishedFenceToken: 3, ErrorCategory: SupplierBatchErrorPublicationFailed, Result: &SupplierBatchCommandResultV1{},
	}}
	failedRaw, err := EncodeSupplierBatchCommandStateV1(failed)
	require.NoError(t, err)
	parsed, err = ParseSupplierBatchCommandStateV1(failedRaw)
	require.NoError(t, err, "stale-fence failures may observe a publication newer than the command fence")
	require.Equal(t, failed, parsed)
	_, err = ParseSupplierBatchCommandStateV1(strings.Replace(raw, `"result":`, `"extra":1,"result":`, 1))
	require.Error(t, err)
}

func TestSupplierBatchCommandStateV1RejectsInvariantViolationsDuringExactParse(t *testing.T) {
	date, nextDate, runID := "2026-07-22", "2026-07-23", int64(9)
	lockedUntil := "2026-07-23T03:00:00Z"
	tests := []struct {
		name  string
		state SupplierBatchCommandStateV1
	}{
		{
			name: "date-bearing completion processed no days",
			state: SupplierBatchCommandStateV1{SchemaVersion: 1, State: SupplierBatchCommandStateCompleted, Response: &SupplierBatchCommandStatusV1{
				RequestID: "completed-zero", BatchDate: &date, RunID: &runID, Status: SupplierBatchCommandStateCompleted, FenceToken: 2, PublishedFenceToken: 2,
				ErrorCategory: SupplierBatchErrorNone, Result: &SupplierBatchCommandResultV1{},
			}},
		},
		{
			name: "completion publication differs from command fence",
			state: SupplierBatchCommandStateV1{SchemaVersion: 1, State: SupplierBatchCommandStateCompleted, Response: &SupplierBatchCommandStatusV1{
				RequestID: "completed-fence", BatchDate: &date, RunID: &runID, Status: SupplierBatchCommandStateCompleted, FenceToken: 2, PublishedFenceToken: 1,
				ErrorCategory: SupplierBatchErrorNone, Result: &SupplierBatchCommandResultV1{ProcessedDays: 1},
			}},
		},
		{
			name: "no-work completion carries remaining hint",
			state: SupplierBatchCommandStateV1{SchemaVersion: 1, State: SupplierBatchCommandStateCompleted, Response: &SupplierBatchCommandStatusV1{
				RequestID: "no-work-hint", Status: SupplierBatchCommandStateCompleted, ErrorCategory: SupplierBatchErrorNone,
				Result: &SupplierBatchCommandResultV1{RemainingWork: true, NextBatchDate: &nextDate},
			}},
		},
		{
			name: "failed command reports processed day",
			state: SupplierBatchCommandStateV1{SchemaVersion: 1, State: SupplierBatchCommandStateFailed, Response: &SupplierBatchCommandStatusV1{
				RequestID: "failed-processed", BatchDate: &date, RunID: &runID, Status: SupplierBatchCommandStateFailed, FenceToken: 2,
				ErrorCategory: SupplierBatchErrorExecutionFailed, Result: &SupplierBatchCommandResultV1{ProcessedDays: 1},
			}},
		},
		{
			name: "failed command has no error",
			state: SupplierBatchCommandStateV1{SchemaVersion: 1, State: SupplierBatchCommandStateFailed, Response: &SupplierBatchCommandStatusV1{
				RequestID: "failed-none", BatchDate: &date, RunID: &runID, Status: SupplierBatchCommandStateFailed, FenceToken: 2,
				ErrorCategory: SupplierBatchErrorNone, Result: &SupplierBatchCommandResultV1{},
			}},
		},
		{
			name: "running command already published its fence",
			state: SupplierBatchCommandStateV1{SchemaVersion: 1, State: SupplierBatchCommandStateRunning, Response: &SupplierBatchCommandStatusV1{
				RequestID: "running-published", BatchDate: &date, RunID: &runID, Status: SupplierBatchCommandStateRunning, FenceToken: 2, PublishedFenceToken: 2,
				LockedUntil: &lockedUntil, ErrorCategory: SupplierBatchErrorNone,
			}},
		},
		{
			name: "remaining false carries next date",
			state: SupplierBatchCommandStateV1{SchemaVersion: 1, State: SupplierBatchCommandStateFailed, Response: &SupplierBatchCommandStatusV1{
				RequestID: "false-with-next", BatchDate: &date, RunID: &runID, Status: SupplierBatchCommandStateFailed, FenceToken: 2,
				ErrorCategory: SupplierBatchErrorExecutionFailed, Result: &SupplierBatchCommandResultV1{NextBatchDate: &nextDate},
			}},
		},
		{
			name: "remaining true omits next date",
			state: SupplierBatchCommandStateV1{SchemaVersion: 1, State: SupplierBatchCommandStateFailed, Response: &SupplierBatchCommandStatusV1{
				RequestID: "true-without-next", BatchDate: &date, RunID: &runID, Status: SupplierBatchCommandStateFailed, FenceToken: 2,
				ErrorCategory: SupplierBatchErrorExecutionFailed, Result: &SupplierBatchCommandResultV1{RemainingWork: true},
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.ErrorIs(t, ValidateSupplierBatchCommandStateV1(test.state), ErrInvalidSupplierBatchCommand)
			raw, err := common.Marshal(test.state)
			require.NoError(t, err)
			_, err = ParseSupplierBatchCommandStateV1(string(raw))
			require.ErrorIs(t, err, ErrInvalidSupplierBatchCommand, "exact storage replay must fail before DTO conversion")
		})
	}
}
