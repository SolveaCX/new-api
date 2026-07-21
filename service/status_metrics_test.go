package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestStatusMetricsHonorCanceledScrapeContext(t *testing.T) {
	setupStatusServiceTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := BuildStatusCenterPrometheusText(ctx, 200_000)
	require.ErrorIs(t, err, context.Canceled)
}

func TestPopulateStatusRollupMetricsRejectsFutureRows(t *testing.T) {
	const now = int64(200_000)
	snapshot := newStatusCenterMetricSnapshot(now)
	components := []model.StatusComponent{{ID: 1, Lifecycle: model.StatusLifecycleActive}}
	rollups := []model.StatusRollupMetricRow{{
		ComponentID: 1,
		Granularity: model.StatusGranularityHour,
		Latest:      now + 3_600,
	}}

	populateStatusRollupMetrics(&snapshot, components, rollups, now)

	require.Equal(t, int64(0), snapshot.rollupReady[model.StatusGranularityHour])
	require.Equal(t, now, snapshot.rollupLag[model.StatusGranularityHour])
}
