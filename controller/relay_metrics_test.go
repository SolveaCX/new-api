package controller

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestSnapshotRelayErrorForMetricsPreservesClassificationIdentity(t *testing.T) {
	relayErr := types.NewErrorWithStatusCode(
		context.DeadlineExceeded,
		types.ErrorCodeDoRequestFailed,
		http.StatusGatewayTimeout,
	)

	snapshot := snapshotRelayErrorForMetrics(relayErr)
	require.NotSame(t, relayErr, snapshot)
	require.True(t, errors.Is(snapshot, context.DeadlineExceeded))

	relayErr.SetMessage("sanitized client message")
	require.False(t, errors.Is(relayErr, context.DeadlineExceeded))
	require.True(t, errors.Is(snapshot, context.DeadlineExceeded))
}
