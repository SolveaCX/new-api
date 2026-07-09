package perfmetrics

import (
	"context"
	"sync"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func resetPerfMetricsStateForTest(t *testing.T) {
	t.Helper()
	hotBuckets = sync.Map{}
	prometheusPendingBuckets = sync.Map{}
}

func TestPrometheusMetricsAreKeptInLocalProcessMemory(t *testing.T) {
	resetPerfMetricsStateForTest(t)

	Record(Sample{
		Model:     "gpt-5",
		Group:     "default",
		ChannelID: 7,
		LatencyMs: 1200,
		Success:   true,
	})

	require.Equal(t, 1, syncMapLen(prometheusPendingBuckets))
}

func TestBuildPrometheusTextEmitsModelStatusRequestCountersOnly(t *testing.T) {
	resetPerfMetricsStateForTest(t)

	Record(Sample{
		Model:     "gpt-5",
		Group:     "default",
		ChannelID: 7,
		LatencyMs: 1200,
		Success:   true,
	})
	Record(Sample{
		Model:     "gpt-5",
		Group:     "default",
		ChannelID: 8,
		LatencyMs: 3200,
		Success:   true,
	})
	Record(Sample{
		Model:     "gpt-5",
		Group:     "default",
		ChannelID: 9,
		LatencyMs: 4500,
		Success:   false,
	})

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)

	require.NotContains(t, text, "newapi_model_request_duration_seconds")
	require.NotContains(t, text, "channel_id")
	require.Contains(t, text, "# HELP newapi_perf_metrics_series Number of model/status series exposed by this endpoint.")
	require.Contains(t, text, `newapi_perf_metrics_series 2`)
	require.Contains(t, text, `newapi_model_requests_total{model="gpt-5",status="success"} 2`)
	require.Contains(t, text, `newapi_model_requests_total{model="gpt-5",status="error"} 1`)
	require.NotContains(t, text, "newapi_perf_metrics_redis_available")
}

func TestRecordRelaySampleUsesModelStatusLabels(t *testing.T) {
	resetPerfMetricsStateForTest(t)

	RecordRelaySample(&relaycommon.RelayInfo{
		OriginModelName: "gpt-5",
		UsingGroup:      "default",
		StartTime:       time.Now().Add(-1200 * time.Millisecond),
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId: 42,
		},
	}, true, 0)

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)

	require.Contains(t, text, `newapi_model_requests_total{model="gpt-5",status="success"} 1`)
	require.NotContains(t, text, "channel_id")
}

func TestBuildPrometheusTextEscapesLabelValues(t *testing.T) {
	resetPerfMetricsStateForTest(t)

	Record(Sample{
		Model:     "gpt\"5\\mini\nv2",
		Group:     "default",
		ChannelID: 9,
		LatencyMs: 100,
		Success:   true,
	})

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	require.Contains(t, text, `model="gpt\"5\\mini\nv2"`)
}

func TestPrometheusCountersIsZeroChecksCount(t *testing.T) {
	counter := prometheusCounters{}
	require.True(t, counter.isZero())

	counter.count = 1
	require.False(t, counter.isZero())
}

func TestPrometheusModelStatusCoalescesChannelIDs(t *testing.T) {
	resetPerfMetricsStateForTest(t)

	Record(Sample{
		Model:     "gpt-5",
		Group:     "default",
		ChannelID: 7,
		LatencyMs: 100,
		Success:   true,
	})
	Record(Sample{
		Model:     "gpt-5",
		Group:     "default",
		ChannelID: 8,
		LatencyMs: 100,
		Success:   true,
	})

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	require.Contains(t, text, `newapi_model_requests_total{model="gpt-5",status="success"} 2`)
	require.NotContains(t, text, "channel_id")
}

func TestPrometheusSeriesScanLimitFailsClosed(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	t.Setenv(prometheusMaxSeriesPerScrapeEnv, "1")

	Record(Sample{
		Model:     "gpt-5",
		Group:     "default",
		ChannelID: 7,
		LatencyMs: 100,
		Success:   true,
	})
	Record(Sample{
		Model:     "gpt-5-mini",
		Group:     "default",
		ChannelID: 8,
		LatencyMs: 100,
		Success:   true,
	})

	text, err := BuildPrometheusText(context.Background())
	require.Error(t, err)
	require.Empty(t, text)
}

func TestBuildPrometheusTextPrunesIdleLocalSeries(t *testing.T) {
	resetPerfMetricsStateForTest(t)

	staleKey := prometheusSeriesKey{model: "stale-model", status: "success"}
	staleBucket := &prometheusLockedBucket{}
	staleBucket.add(Sample{
		Model:     "stale-model",
		ChannelID: 7,
		LatencyMs: 100,
		Success:   true,
	})
	staleBucket.mu.Lock()
	staleBucket.lastUpdatedAt = time.Now().Add(-prometheusSeriesIdleRetention - time.Minute).UnixNano()
	staleBucket.mu.Unlock()
	prometheusPendingBuckets.Store(staleKey, staleBucket)

	Record(Sample{
		Model:     "active-model",
		Group:     "default",
		ChannelID: 8,
		LatencyMs: 200,
		Success:   true,
	})

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	require.Contains(t, text, `newapi_model_requests_total{model="active-model",status="success"} 1`)
	require.NotContains(t, text, "stale-model")

	_, exists := prometheusPendingBuckets.Load(staleKey)
	require.False(t, exists)
}

func syncMapLen(m sync.Map) int {
	count := 0
	m.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}
