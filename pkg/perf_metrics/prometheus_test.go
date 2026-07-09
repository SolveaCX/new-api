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

func TestBuildPrometheusTextEmitsHistogramAndRequestCounters(t *testing.T) {
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
		ChannelID: 7,
		LatencyMs: 4500,
		Success:   false,
	})

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)

	require.Contains(t, text, "# TYPE newapi_model_request_duration_seconds histogram")
	require.Contains(t, text, `newapi_model_request_duration_seconds_bucket{model="gpt-5",channel_id="7",status="success",le="2"} 1`)
	require.Contains(t, text, `newapi_model_request_duration_seconds_bucket{model="gpt-5",channel_id="7",status="error",le="5"} 1`)
	require.Contains(t, text, `newapi_model_request_duration_seconds_sum{model="gpt-5",channel_id="7",status="success"} 1.200`)
	require.Contains(t, text, `newapi_model_request_duration_seconds_count{model="gpt-5",channel_id="7",status="success"} 1`)
	require.Contains(t, text, `newapi_model_requests_total{model="gpt-5",channel_id="7",status="success"} 1`)
	require.Contains(t, text, `newapi_model_requests_total{model="gpt-5",channel_id="7",status="error"} 1`)
	require.NotContains(t, text, "newapi_perf_metrics_redis_available")
}

func TestRecordRelaySampleUsesChannelIDLabel(t *testing.T) {
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

	require.Contains(t, text, `newapi_model_requests_total{model="gpt-5",channel_id="42",status="success"} 1`)
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

func TestPrometheusCountersIsZeroChecksBuckets(t *testing.T) {
	counter := prometheusCounters{}
	require.True(t, counter.isZero())

	counter.buckets[prometheusInfBucketIndex] = 1
	require.False(t, counter.isZero())
}

func TestPrometheusChannelLabelCanBeDisabled(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	t.Setenv(prometheusChannelLabelEnabledEnv, "false")

	Record(Sample{
		Model:     "gpt-5",
		Group:     "default",
		ChannelID: 7,
		LatencyMs: 100,
		Success:   true,
	})

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	require.Contains(t, text, `newapi_model_requests_total{model="gpt-5",channel_id="unknown",status="success"} 1`)
	require.NotContains(t, text, `channel_id="7"`)
}

func TestPrometheusChannelLabelDisableCoalescesLocalSeries(t *testing.T) {
	resetPerfMetricsStateForTest(t)

	Record(Sample{
		Model:     "gpt-5",
		Group:     "default",
		ChannelID: 7,
		LatencyMs: 100,
		Success:   true,
	})
	t.Setenv(prometheusChannelLabelEnabledEnv, "false")

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	require.Contains(t, text, `newapi_model_requests_total{model="gpt-5",channel_id="unknown",status="success"} 1`)
	require.NotContains(t, text, `channel_id="7"`)
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

	staleKey := prometheusSeriesKey{model: "stale-model", channelID: 7, status: "success"}
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
	require.Contains(t, text, `newapi_model_requests_total{model="active-model",channel_id="8",status="success"} 1`)
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
