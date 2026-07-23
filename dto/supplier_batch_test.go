package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestSupplierBatchStatusResponseFixedJSON(t *testing.T) {
	batchDate := "2026-07-22"
	runID := int64(42)
	lockedUntil := "2026-07-23T03:00:00+08:00"
	running := SupplierBatchStatusResponse{
		RequestID: "runner-key", BatchDate: &batchDate, RunID: &runID,
		Status: SupplierBatchStatusRunning, FenceToken: 9, PublishedFenceToken: 7,
		LockedUntil: &lockedUntil, ErrorCategory: SupplierBatchErrorNone,
	}
	require.NoError(t, running.Validate())
	payload, err := common.Marshal(running)
	require.NoError(t, err)
	require.JSONEq(t, `{"request_id":"runner-key","batch_date":"2026-07-22","run_id":42,"status":"running","fence_token":9,"published_fence_token":7,"locked_until":"2026-07-23T03:00:00+08:00","error_category":"none","result":null}`, string(payload))
	require.NotContains(t, string(payload), "identity")
	require.NotContains(t, string(payload), "owner")
	require.NotContains(t, string(payload), "bearer")
	require.NotContains(t, string(payload), "verifier")
}

func TestSupplierBatchStatusResponseNoWorkInvariant(t *testing.T) {
	response := SupplierBatchStatusResponse{
		RequestID: "runner-key", Status: SupplierBatchStatusCompleted,
		ErrorCategory: SupplierBatchErrorNone,
		Result:        &SupplierBatchStatusResult{ProcessedDays: 0, RemainingWork: false},
	}
	require.NoError(t, response.Validate())
	payload, err := common.Marshal(response)
	require.NoError(t, err)
	require.JSONEq(t, `{"request_id":"runner-key","batch_date":null,"run_id":null,"status":"completed","fence_token":0,"published_fence_token":0,"locked_until":null,"error_category":"none","result":{"processed_days":0,"remaining_work":false,"next_batch_date":null}}`, string(payload))
}

func TestSupplierBatchStatusResponseRejectsBrokenInvariants(t *testing.T) {
	batchDate := "2026-07-22"
	nextDate := "2026-07-23"
	runID := int64(42)
	lockedUntil := "not-rfc3339"
	tests := []SupplierBatchStatusResponse{
		{RequestID: "runner-key", Status: "queued", ErrorCategory: SupplierBatchErrorNone},
		{RequestID: "runner-key", BatchDate: &batchDate, RunID: &runID, Status: SupplierBatchStatusRunning, FenceToken: 1, LockedUntil: &lockedUntil, ErrorCategory: SupplierBatchErrorNone},
		{RequestID: "runner-key", BatchDate: &batchDate, RunID: &runID, Status: SupplierBatchStatusFailed, FenceToken: 1, ErrorCategory: SupplierBatchErrorNone, Result: &SupplierBatchStatusResult{}},
		{RequestID: "runner-key", BatchDate: &batchDate, RunID: &runID, Status: SupplierBatchStatusCompleted, FenceToken: 1, PublishedFenceToken: 0, ErrorCategory: SupplierBatchErrorNone, Result: &SupplierBatchStatusResult{ProcessedDays: 1}},
		{RequestID: "runner-key", BatchDate: &batchDate, RunID: &runID, Status: SupplierBatchStatusCompleted, FenceToken: 2, PublishedFenceToken: 1, ErrorCategory: SupplierBatchErrorNone, Result: &SupplierBatchStatusResult{ProcessedDays: 1}},
		{RequestID: "runner-key", BatchDate: &batchDate, RunID: &runID, Status: SupplierBatchStatusCompleted, FenceToken: 1, PublishedFenceToken: 1, ErrorCategory: SupplierBatchErrorNone, Result: &SupplierBatchStatusResult{}},
		{RequestID: "runner-key", Status: SupplierBatchStatusCompleted, ErrorCategory: SupplierBatchErrorNone, Result: &SupplierBatchStatusResult{ProcessedDays: 1}},
		{RequestID: "runner-key", Status: SupplierBatchStatusCompleted, ErrorCategory: SupplierBatchErrorNone, Result: &SupplierBatchStatusResult{RemainingWork: true, NextBatchDate: &nextDate}},
		{RequestID: "runner-key", BatchDate: &batchDate, RunID: &runID, Status: SupplierBatchStatusFailed, FenceToken: 1, ErrorCategory: SupplierBatchErrorExecutionFailed, Result: &SupplierBatchStatusResult{ProcessedDays: 1}},
		{RequestID: "runner-key", BatchDate: &batchDate, RunID: &runID, Status: SupplierBatchStatusRunning, FenceToken: 1, PublishedFenceToken: 1, LockedUntil: func() *string { value := "2026-07-23T03:00:00Z"; return &value }(), ErrorCategory: SupplierBatchErrorNone},
		{RequestID: "runner-key", BatchDate: &batchDate, RunID: &runID, Status: SupplierBatchStatusFailed, FenceToken: 1, ErrorCategory: SupplierBatchErrorExecutionFailed, Result: &SupplierBatchStatusResult{NextBatchDate: &nextDate}},
		{RequestID: "runner-key", BatchDate: &batchDate, RunID: &runID, Status: SupplierBatchStatusFailed, FenceToken: 1, ErrorCategory: SupplierBatchErrorExecutionFailed, Result: &SupplierBatchStatusResult{RemainingWork: true}},
	}
	for _, response := range tests {
		require.ErrorIs(t, response.Validate(), ErrSupplierBatchStatusInvalid)
	}
}

func TestSupplierBatchStatusResponseAllowsNewerPublishedFenceForStaleFailure(t *testing.T) {
	batchDate := "2026-07-22"
	runID := int64(42)
	response := SupplierBatchStatusResponse{
		RequestID: "stale-fence", BatchDate: &batchDate, RunID: &runID, Status: SupplierBatchStatusFailed,
		FenceToken: 7, PublishedFenceToken: 8, ErrorCategory: SupplierBatchErrorFenceLost,
		Result: &SupplierBatchStatusResult{ProcessedDays: 0, RemainingWork: false},
	}
	require.NoError(t, response.Validate())
}
