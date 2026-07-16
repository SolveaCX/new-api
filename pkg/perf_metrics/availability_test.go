package perfmetrics

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestClassifyAvailabilityOutcome(t *testing.T) {
	tests := []struct {
		name    string
		success bool
		err     *types.NewAPIError
		want    AvailabilityOutcome
	}{
		{name: "success", success: true, want: AvailabilityEligibleSuccess},
		{name: "final failure without detail", want: AvailabilityEligibleFailure},
		{name: "upstream timeout", err: types.NewError(context.DeadlineExceeded, types.ErrorCodeDoRequestFailed), want: AvailabilityEligibleFailure},
		{name: "upstream server error", err: types.NewErrorWithStatusCode(errors.New("unavailable"), types.ErrorCodeBadResponseStatusCode, http.StatusServiceUnavailable), want: AvailabilityEligibleFailure},
		{name: "upstream bad response", err: types.NewError(errors.New("bad body"), types.ErrorCodeBadResponseBody), want: AvailabilityEligibleFailure},
		{name: "router exhausted", err: types.NewError(errors.New("no channel"), types.ErrorCodeGetChannelFailed), want: AvailabilityEligibleFailure},
		{name: "channel auth", err: types.NewErrorWithStatusCode(errors.New("bad upstream key"), types.ErrorCodeChannelInvalidKey, http.StatusUnauthorized), want: AvailabilityEligibleFailure},
		{name: "upstream rate limit", err: types.NewErrorWithStatusCode(errors.New("limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests), want: AvailabilityEligibleFailure},
		{name: "client cancellation", err: types.NewError(context.Canceled, types.ErrorCodeDoRequestFailed), want: AvailabilityExcluded},
		{name: "invalid request", err: types.NewErrorWithStatusCode(errors.New("invalid"), types.ErrorCodeInvalidRequest, http.StatusBadRequest), want: AvailabilityExcluded},
		{name: "user quota", err: types.NewErrorWithStatusCode(errors.New("quota"), types.ErrorCodeInsufficientUserQuota, http.StatusForbidden), want: AvailabilityExcluded},
		{name: "policy rejection", err: types.NewErrorWithStatusCode(errors.New("blocked"), types.ErrorCodeAccessDenied, http.StatusForbidden), want: AvailabilityExcluded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ClassifyAvailabilityOutcome(tt.success, tt.err))
		})
	}
}

func TestRecordRelaySampleCountsAvailabilityOnce(t *testing.T) {
	resetPerfMetricsStateForTest(t)

	RecordRelaySample(&relaycommon.RelayInfo{
		OriginModelName: "gpt-5",
		UsingGroup:      "default",
		StartTime:       time.Now().Add(-time.Second),
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 42},
	}, false, 0, types.NewError(context.DeadlineExceeded, types.ErrorCodeDoRequestFailed))

	var snapshot counters
	hotBuckets.Range(func(_, value any) bool {
		snapshot = value.(*atomicBucket).snapshot()
		return false
	})
	require.EqualValues(t, 1, snapshot.availabilityEligibleCount)
	require.Zero(t, snapshot.availabilitySuccessCount)
}

func TestRecordRelaySampleExcludesClientGoneFromAvailability(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	streamStatus := relaycommon.NewStreamStatus()
	streamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, nil)

	RecordRelaySample(&relaycommon.RelayInfo{
		OriginModelName: "gpt-5",
		UsingGroup:      "default",
		StartTime:       time.Now().Add(-time.Second),
		IsStream:        true,
		StreamStatus:    streamStatus,
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 42},
	}, true, 0, nil)

	var snapshot counters
	hotBuckets.Range(func(_, value any) bool {
		snapshot = value.(*atomicBucket).snapshot()
		return false
	})
	require.Zero(t, snapshot.availabilityEligibleCount)
	require.Zero(t, snapshot.availabilitySuccessCount)
}
