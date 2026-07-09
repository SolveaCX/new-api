package perfmetrics

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func resetPerfMetricsStateForTest(t *testing.T) {
	t.Helper()
	hotBuckets = sync.Map{}
	redisPendingBuckets = sync.Map{}
	prometheusPendingBuckets = sync.Map{}
	prometheusInflightBuckets = sync.Map{}
}

func setupMiniRedisForPerfMetrics(t *testing.T) {
	t.Helper()
	mr := miniredis.RunT(t)
	origClient := common.RDB
	origEnabled := common.RedisEnabled
	common.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	common.RedisEnabled = true
	t.Cleanup(func() {
		common.RDB.Close()
		common.RDB = origClient
		common.RedisEnabled = origEnabled
	})
}

func disableRedisForPerfMetrics(t *testing.T) {
	t.Helper()
	origClient := common.RDB
	origEnabled := common.RedisEnabled
	common.RDB = nil
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.RDB = origClient
		common.RedisEnabled = origEnabled
	})
}

func TestPrometheusMetricsAreQueuedLocallyUntilRedisFlush(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	setupMiniRedisForPerfMetrics(t)

	Record(Sample{
		Model:     "gpt-5",
		Group:     "default",
		ChannelID: 7,
		LatencyMs: 1200,
		Success:   true,
	})

	ctx := context.Background()
	exists, err := common.RDB.Exists(ctx, prometheusSeriesSetKey).Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), exists, "Record must not write Redis on the request path")

	require.NoError(t, flushRedisMetricsOnce(ctx))

	members, err := common.RDB.SMembers(ctx, prometheusSeriesSetKey).Result()
	require.NoError(t, err)
	require.Len(t, members, 1)
}

func TestBuildPrometheusTextEmitsHistogramAndRequestCounters(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	setupMiniRedisForPerfMetrics(t)

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
	require.NoError(t, flushRedisMetricsOnce(context.Background()))

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)

	require.Contains(t, text, "# TYPE newapi_model_request_duration_seconds histogram")
	require.Contains(t, text, `newapi_model_request_duration_seconds_bucket{model="gpt-5",channel_id="7",status="success",le="2"} 1`)
	require.Contains(t, text, `newapi_model_request_duration_seconds_bucket{model="gpt-5",channel_id="7",status="error",le="5"} 1`)
	require.Contains(t, text, `newapi_model_request_duration_seconds_sum{model="gpt-5",channel_id="7",status="success"} 1.200`)
	require.Contains(t, text, `newapi_model_request_duration_seconds_count{model="gpt-5",channel_id="7",status="success"} 1`)
	require.Contains(t, text, `newapi_model_requests_total{model="gpt-5",channel_id="7",status="success"} 1`)
	require.Contains(t, text, `newapi_model_requests_total{model="gpt-5",channel_id="7",status="error"} 1`)
	require.Contains(t, text, "newapi_perf_metrics_redis_available 1")
}

func TestBuildPrometheusTextEmitsLocalMetricsWhenRedisUnavailable(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	disableRedisForPerfMetrics(t)

	Record(Sample{
		Model:     "gpt-5",
		Group:     "default",
		ChannelID: 7,
		LatencyMs: 1200,
		Success:   true,
	})

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)

	require.Contains(t, text, "newapi_perf_metrics_redis_available 0")
	require.Contains(t, text, `newapi_model_request_duration_seconds_count{model="gpt-5",channel_id="7",status="success"} 1`)
}

func TestRecordRelaySampleUsesChannelIDLabel(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	disableRedisForPerfMetrics(t)

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
	setupMiniRedisForPerfMetrics(t)

	Record(Sample{
		Model:     "gpt\"5\\mini\nv2",
		Group:     "default",
		ChannelID: 9,
		LatencyMs: 100,
		Success:   true,
	})
	require.NoError(t, flushRedisMetricsOnce(context.Background()))

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

func TestFlushRedisMetricsDeletesFlushedHistoricalBuckets(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	setupMiniRedisForPerfMetrics(t)

	key := bucketKey{
		model:    "gpt-5",
		group:    "default",
		bucketTs: bucketStart(time.Now().Add(-time.Hour).Unix()),
	}
	bucket := &atomicBucket{}
	bucket.add(Sample{Model: "gpt-5", Group: "default", LatencyMs: 100, Success: true})
	redisPendingBuckets.Store(key, bucket)

	require.NoError(t, flushRedisMetricsOnce(context.Background()))
	require.Equal(t, 0, syncMapLen(redisPendingBuckets))
}

func TestFlushRedisMetricsDeletesFlushedPrometheusBuckets(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	setupMiniRedisForPerfMetrics(t)

	Record(Sample{
		Model:     "gpt-5",
		Group:     "default",
		ChannelID: 7,
		LatencyMs: 100,
		Success:   true,
	})
	require.Equal(t, 1, syncMapLen(prometheusPendingBuckets))

	require.NoError(t, flushRedisMetricsOnce(context.Background()))
	require.Equal(t, 0, syncMapLen(prometheusPendingBuckets))
}

func TestFlushRedisMetricsSetsPrometheusSeriesTTL(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	setupMiniRedisForPerfMetrics(t)

	key := prometheusSeriesKey{model: "gpt-5", channelID: 7, status: "success"}
	Record(Sample{
		Model:     key.model,
		Group:     "default",
		ChannelID: key.channelID,
		LatencyMs: 100,
		Success:   true,
	})

	require.NoError(t, flushRedisMetricsOnce(context.Background()))

	seriesTTL, err := common.RDB.TTL(context.Background(), prometheusSeriesSetKey).Result()
	require.NoError(t, err)
	require.Greater(t, seriesTTL, time.Duration(0))

	hashTTL, err := common.RDB.TTL(context.Background(), prometheusRedisKey(key)).Result()
	require.NoError(t, err)
	require.Greater(t, hashTTL, time.Duration(0))
}

func TestBuildPrometheusTextPrunesStaleSeriesMembers(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	setupMiniRedisForPerfMetrics(t)

	member := encodePrometheusSeriesKey(prometheusSeriesKey{
		model:     "deleted-model",
		channelID: 9,
		status:    "success",
	})
	require.NoError(t, common.RDB.SAdd(context.Background(), prometheusSeriesSetKey, member).Err())

	_, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)

	isMember, err := common.RDB.SIsMember(context.Background(), prometheusSeriesSetKey, member).Result()
	require.NoError(t, err)
	require.False(t, isMember)
}

func TestRedisFlushFailureRequeuesDrainedCounters(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	setupMiniRedisForPerfMetrics(t)

	Record(Sample{
		Model:     "gpt-5",
		Group:     "default",
		ChannelID: 7,
		LatencyMs: 100,
		Success:   true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, flushRedisMetricsOnce(ctx))

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	require.Contains(t, text, `newapi_model_requests_total{model="gpt-5",channel_id="7",status="success"} 1`)
}

func TestRedisExecErrorDoesNotRequeueAmbiguousFlush(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	setupMiniRedisForPerfMetrics(t)

	require.NoError(t, common.RDB.Set(context.Background(), prometheusSeriesSetKey, "not-a-set", 0).Err())
	Record(Sample{
		Model:     "gpt-5",
		Group:     "default",
		ChannelID: 7,
		LatencyMs: 100,
		Success:   true,
	})

	require.Error(t, flushRedisMetricsOnce(context.Background()))
	require.Equal(t, 0, syncMapLen(prometheusPendingBuckets))
	require.Equal(t, 0, syncMapLen(prometheusInflightBuckets))
}

func TestBuildPrometheusTextIncludesInflightFlushCounters(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	disableRedisForPerfMetrics(t)

	key := prometheusSeriesKey{model: "gpt-5", channelID: 7, status: "success"}
	counter := prometheusCounters{
		count: 1,
		sumMs: 1200,
	}
	counter.buckets[prometheusInfBucketIndex] = 1
	addPrometheusInflightCounter(key, counter)

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	require.Contains(t, text, `newapi_model_request_duration_seconds_count{model="gpt-5",channel_id="7",status="success"} 1`)
	require.Contains(t, text, `newapi_model_request_duration_seconds_sum{model="gpt-5",channel_id="7",status="success"} 1.200`)
}

func TestPrometheusSeriesKeyEncodingAcceptsDelimiterInModel(t *testing.T) {
	key := prometheusSeriesKey{
		model:     "gpt\x1f5",
		channelID: 7,
		status:    "success",
	}

	decoded, ok := decodePrometheusSeriesKey(encodePrometheusSeriesKey(key))
	require.True(t, ok)
	require.Equal(t, key, decoded)
}

func TestPrometheusChannelLabelCanBeDisabled(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	disableRedisForPerfMetrics(t)
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

func TestPrometheusSeriesScanLimitFailsClosed(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	setupMiniRedisForPerfMetrics(t)
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
	require.NoError(t, flushRedisMetricsOnce(context.Background()))

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	require.Contains(t, text, "newapi_perf_metrics_redis_available 0")
	require.Contains(t, text, "newapi_perf_metrics_series 0")
}

func syncMapLen(m sync.Map) int {
	count := 0
	m.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}
