package perfmetrics

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func resetPerfMetricsStateForTest(t *testing.T) {
	t.Helper()
	hotBuckets = sync.Map{}
	prometheusPendingBuckets = sync.Map{}
	prometheusChannelBuckets = sync.Map{}
	prometheusChannelModelBuckets = sync.Map{}
	prometheusModelPerformanceBuckets = sync.Map{}
	prometheusModelAdmissionMu = sync.Mutex{}
	prometheusModelDroppedSamples = prometheusModelDropCounters{}
}

func TestRecordRelaySampleCapturesSuccessfulModelLatencyAndTTFT(t *testing.T) {
	resetPerfMetricsStateForTest(t)

	now := time.Now()
	startedAt := now.Add(-2 * time.Second)
	RecordRelaySample(&relaycommon.RelayInfo{
		OriginModelName:   "gpt-5",
		StartTime:         startedAt,
		FirstResponseTime: startedAt.Add(250 * time.Millisecond),
		IsStream:          true,
		ChannelMeta:       &relaycommon.ChannelMeta{ChannelId: 42},
	}, true, 12, nil)

	snapshots := snapshotPrometheusModelPerformances(time.Now())
	require.Len(t, snapshots, 1)
	snapshot := snapshots[0]
	require.Equal(t, "gpt-5", snapshot.model)
	require.EqualValues(t, 1, snapshot.latencyCount)
	require.InDelta(t, 2, snapshot.latencySumSeconds, 0.1)
	require.EqualValues(t, 1, snapshot.streamSuccess)
	require.EqualValues(t, 1, snapshot.ttftCount)
	require.InDelta(t, 0.25, snapshot.ttftSumSeconds, 0.01)
	require.Empty(t, snapshot.errors)
}

func TestRecordRelaySampleKeepsFinalFailureOutOfLatency(t *testing.T) {
	resetPerfMetricsStateForTest(t)

	RecordRelaySample(&relaycommon.RelayInfo{
		OriginModelName: "gpt-5",
		StartTime:       time.Now().Add(-time.Second),
		IsStream:        true,
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 42},
	}, false, 0, types.NewErrorWithStatusCode(
		context.DeadlineExceeded,
		types.ErrorCodeDoRequestFailed,
		http.StatusGatewayTimeout,
	))

	snapshots := snapshotPrometheusModelPerformances(time.Now())
	require.Len(t, snapshots, 1)
	snapshot := snapshots[0]
	require.Equal(t, map[string]int64{"timeout": 1}, snapshot.errors)
	require.EqualValues(t, 0, snapshot.latencyCount)
	require.Zero(t, snapshot.latencySumSeconds)
	require.EqualValues(t, 0, snapshot.ttftCount)
	require.Zero(t, snapshot.ttftSumSeconds)
	require.EqualValues(t, 0, snapshot.streamSuccess)
}

func TestRecordRelaySampleCountsStreamWithoutFirstToken(t *testing.T) {
	resetPerfMetricsStateForTest(t)

	RecordRelaySample(&relaycommon.RelayInfo{
		OriginModelName:   "gpt-5",
		StartTime:         time.Now().Add(-2 * time.Second),
		FirstResponseTime: time.Time{},
		IsStream:          true,
		ChannelMeta:       &relaycommon.ChannelMeta{ChannelId: 42},
	}, true, 0, nil)

	snapshots := snapshotPrometheusModelPerformances(time.Now())
	require.Len(t, snapshots, 1)
	require.EqualValues(t, 1, snapshots[0].latencyCount)
	require.EqualValues(t, 1, snapshots[0].streamSuccess)
	require.EqualValues(t, 0, snapshots[0].ttftCount)
	require.Zero(t, snapshots[0].ttftSumSeconds)
	require.Empty(t, snapshots[0].errors)
	require.EqualValues(t, 0, prometheusModelDroppedSamples.snapshot()[modelDropInvalidTTFT])
}

func TestRecordRelaySampleCountsLegacySentinelAsMissingFirstToken(t *testing.T) {
	resetPerfMetricsStateForTest(t)

	startedAt := time.Now().Add(-2 * time.Second)
	RecordRelaySample(&relaycommon.RelayInfo{
		OriginModelName:   "gpt-5",
		StartTime:         startedAt,
		FirstResponseTime: startedAt.Add(-time.Second),
		IsStream:          true,
		ChannelMeta:       &relaycommon.ChannelMeta{ChannelId: 42},
	}, true, 0, nil)

	snapshots := snapshotPrometheusModelPerformances(time.Now())
	require.Len(t, snapshots, 1)
	require.EqualValues(t, 1, snapshots[0].latencyCount)
	require.EqualValues(t, 1, snapshots[0].streamSuccess)
	require.EqualValues(t, 0, snapshots[0].ttftCount)
	require.Zero(t, snapshots[0].ttftSumSeconds)
	require.Empty(t, snapshots[0].errors)
	require.EqualValues(t, 0, prometheusModelDroppedSamples.snapshot()[modelDropInvalidTTFT])
}

func TestRecordRelaySampleRejectsInvalidTimestamps(t *testing.T) {
	resetPerfMetricsStateForTest(t)

	now := time.Now()
	RecordRelaySample(&relaycommon.RelayInfo{
		OriginModelName:   "gpt-5",
		StartTime:         now.Add(time.Minute),
		FirstResponseTime: now.Add(-time.Minute),
		IsStream:          true,
		ChannelMeta:       &relaycommon.ChannelMeta{ChannelId: 42},
	}, true, 0, nil)

	snapshots := snapshotPrometheusModelPerformances(time.Now())
	require.Len(t, snapshots, 1)
	require.EqualValues(t, 0, snapshots[0].latencyCount)
	require.Zero(t, snapshots[0].latencySumSeconds)
	require.EqualValues(t, 0, snapshots[0].ttftCount)
	require.Zero(t, snapshots[0].ttftSumSeconds)
	require.EqualValues(t, 1, snapshots[0].streamSuccess)
	require.Empty(t, snapshots[0].errors)
	dropped := prometheusModelDroppedSamples.snapshot()
	require.EqualValues(t, 1, dropped[modelDropInvalidLatency])
	require.EqualValues(t, 1, dropped[modelDropInvalidTTFT])
}

func TestRecordRelaySampleClassifiesNilFinalFailureAsOther(t *testing.T) {
	resetPerfMetricsStateForTest(t)

	RecordRelaySample(&relaycommon.RelayInfo{
		OriginModelName: "gpt-5",
		StartTime:       time.Now().Add(-time.Second),
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 42},
	}, false, 0, nil)

	snapshots := snapshotPrometheusModelPerformances(time.Now())
	require.Len(t, snapshots, 1)
	require.EqualValues(t, 1, snapshots[0].errors["other"])
	require.NotContains(t, snapshots[0].errors, "none")
}

func TestRecordRelaySampleEnforcesConfigurableActiveModelLimit(t *testing.T) {
	t.Run("rejects next active model", func(t *testing.T) {
		resetPerfMetricsStateForTest(t)
		t.Setenv(prometheusMaxModelHistogramModelsEnv, "1")

		RecordRelaySample(&relaycommon.RelayInfo{OriginModelName: "gpt-5", StartTime: time.Now().Add(-time.Second), ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 42}}, true, 0, nil)
		RecordRelaySample(&relaycommon.RelayInfo{OriginModelName: "gpt-5-mini", StartTime: time.Now().Add(-time.Second), ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 42}}, true, 0, nil)

		snapshots := snapshotPrometheusModelPerformances(time.Now())
		require.Len(t, snapshots, 1)
		require.Equal(t, "gpt-5", snapshots[0].model)
		require.EqualValues(t, 1, prometheusModelDroppedSamples.snapshot()[modelDropModelLimit])
	})

	t.Run("non-positive limit disables cap", func(t *testing.T) {
		resetPerfMetricsStateForTest(t)
		t.Setenv(prometheusMaxModelHistogramModelsEnv, "0")

		RecordRelaySample(&relaycommon.RelayInfo{OriginModelName: "gpt-5", StartTime: time.Now().Add(-time.Second), ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 42}}, true, 0, nil)
		RecordRelaySample(&relaycommon.RelayInfo{OriginModelName: "gpt-5-mini", StartTime: time.Now().Add(-time.Second), ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 42}}, true, 0, nil)

		require.Len(t, snapshotPrometheusModelPerformances(time.Now()), 2)
		require.EqualValues(t, 0, prometheusModelDroppedSamples.snapshot()[modelDropModelLimit])
	})
}

func TestPrometheusModelAdmissionHonorsCapConcurrently(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	t.Setenv(prometheusMaxModelHistogramModelsEnv, "3")

	const attempts = 64
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(attempts)
	for i := 0; i < attempts; i++ {
		go func(modelIndex int) {
			defer wg.Done()
			<-start
			RecordRelaySample(&relaycommon.RelayInfo{
				OriginModelName: fmt.Sprintf("model-%d", modelIndex),
				StartTime:       time.Now().Add(-time.Second),
				ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 42},
			}, true, 0, nil)
		}(i)
	}

	close(start)
	wg.Wait()

	snapshots := snapshotPrometheusModelPerformances(time.Now())
	require.LessOrEqual(t, len(snapshots), 3)
	var admittedSamples int64
	for _, snapshot := range snapshots {
		admittedSamples += snapshot.latencyCount
	}
	droppedSamples := prometheusModelDroppedSamples.snapshot()[modelDropModelLimit]
	require.EqualValues(t, attempts, admittedSamples+droppedSamples)
}

func TestPrometheusModelMutationRetriesAfterConcurrentRetirement(t *testing.T) {
	resetPerfMetricsStateForTest(t)

	now := time.Now()
	retiredBucket := newPrometheusModelPerformanceBucket(now)
	retiredBucket.mu.Lock()
	retiredBucket.retired = true
	retiredBucket.mu.Unlock()

	loadCount := 0
	var replacementBucket *prometheusModelPerformanceBucket
	latencySeconds := 0.5
	mutatePrometheusModelPerformanceWithLoader(
		"gpt-5",
		now,
		func(model string, loadTime time.Time) (*prometheusModelPerformanceBucket, bool) {
			loadCount++
			if loadCount == 1 {
				prometheusModelPerformanceBuckets.Store(model, retiredBucket)
				return retiredBucket, true
			}
			bucket, admitted := loadOrCreatePrometheusModelBucket(model, loadTime)
			replacementBucket = bucket
			return bucket, admitted
		},
		func(bucket *prometheusModelPerformanceBucket) bool {
			return bucket.addSuccess(now, &latencySeconds, false, nil)
		},
	)

	require.Equal(t, 2, loadCount)
	require.NotNil(t, replacementBucket)
	require.NotSame(t, retiredBucket, replacementBucket)
	replacementSnapshot, ok := replacementBucket.snapshot("gpt-5")
	require.True(t, ok)
	require.EqualValues(t, 1, replacementSnapshot.latencyCount)
	require.InDelta(t, latencySeconds, replacementSnapshot.latencySumSeconds, 0.001)
	retiredBucket.mu.Lock()
	require.EqualValues(t, 0, retiredBucket.latencyCount)
	retiredBucket.mu.Unlock()
}

func TestSnapshotPrometheusModelPerformanceRetiresIdleModels(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	t.Setenv(prometheusMaxModelHistogramModelsEnv, "1")

	RecordRelaySample(&relaycommon.RelayInfo{
		OriginModelName: "stale-model",
		StartTime:       time.Now().Add(-time.Second),
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 42},
	}, true, 0, nil)
	value, ok := prometheusModelPerformanceBuckets.Load("stale-model")
	require.True(t, ok)
	staleBucket := value.(*prometheusModelPerformanceBucket)
	staleBucket.mu.Lock()
	staleBucket.lastUpdatedAt = time.Now().Add(-prometheusModelIdleRetention - time.Minute).UnixNano()
	staleBucket.mu.Unlock()

	require.Empty(t, snapshotPrometheusModelPerformances(time.Now()))
	_, ok = prometheusModelPerformanceBuckets.Load("stale-model")
	require.False(t, ok)

	RecordRelaySample(&relaycommon.RelayInfo{
		OriginModelName: "replacement-model",
		StartTime:       time.Now().Add(-time.Second),
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 42},
	}, true, 0, nil)
	snapshots := snapshotPrometheusModelPerformances(time.Now())
	require.Len(t, snapshots, 1)
	require.Equal(t, "replacement-model", snapshots[0].model)
	require.EqualValues(t, 0, prometheusModelDroppedSamples.snapshot()[modelDropModelLimit])
}

func TestPrometheusModelDropCountersUseFixedReasons(t *testing.T) {
	var counters prometheusModelDropCounters
	require.Equal(t, []string{modelDropModelLimit, modelDropInvalidLatency, modelDropInvalidTTFT, modelDropSeriesLimit}, modelDropReasons)

	counters.add(modelDropModelLimit, 0)
	counters.add(modelDropInvalidLatency, -1)
	counters.add("unknown", 10)
	require.EqualValues(t, 0, counters.snapshot()[modelDropModelLimit])
	require.EqualValues(t, 0, counters.snapshot()[modelDropInvalidLatency])
	require.NotContains(t, counters.snapshot(), "unknown")

	counters.add(modelDropSeriesLimit, 2)
	require.EqualValues(t, 2, counters.snapshot()[modelDropSeriesLimit])
}

func TestBuildPrometheusTextEmitsChannelHealthMetrics(t *testing.T) {
	resetPerfMetricsStateForTest(t)

	successStartedAt := time.Now().Add(-1500 * time.Millisecond)
	successInfo := &relaycommon.RelayInfo{
		OriginModelName:   "gpt-5",
		StartTime:         successStartedAt,
		FirstResponseTime: successStartedAt.Add(250 * time.Millisecond),
		IsStream:          true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId: 42,
		},
	}
	RecordChannelAttempt(successInfo, 42, "primary\"east", successStartedAt, nil)
	RecordChannelTokens(successInfo, 120, 30)

	failureStartedAt := time.Now().Add(-4 * time.Second)
	failureInfo := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5",
		StartTime:       failureStartedAt,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId: 42,
		},
	}
	RecordChannelAttempt(
		failureInfo,
		42,
		"primary\"east",
		failureStartedAt,
		types.NewErrorWithStatusCode(
			context.DeadlineExceeded,
			types.ErrorCodeDoRequestFailed,
			http.StatusGatewayTimeout,
		),
	)

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)

	require.Contains(t, text, `newapi_channel_info{channel_id="42",channel_name="primary\"east"} 1`)
	require.Contains(t, text, `newapi_channel_attempts_total{channel_id="42",status="success",error_category="none"} 1`)
	require.Contains(t, text, `newapi_channel_attempts_total{channel_id="42",status="error",error_category="timeout"} 1`)
	require.Contains(t, text, `newapi_channel_request_duration_seconds_bucket{channel_id="42",le="2"} 1`)
	require.Contains(t, text, `newapi_channel_request_duration_seconds_bucket{channel_id="42",le="5"} 2`)
	require.Contains(t, text, `newapi_channel_request_duration_seconds_bucket{channel_id="42",le="+Inf"} 2`)
	require.Contains(t, text, `newapi_channel_request_duration_seconds_count{channel_id="42"} 2`)
	require.Contains(t, text, `newapi_channel_ttft_seconds_sum{channel_id="42"} 0.25`)
	require.Contains(t, text, `newapi_channel_ttft_seconds_count{channel_id="42"} 1`)
	require.Contains(t, text, `newapi_channel_model_attempts_total{channel_id="42",model="gpt-5",status="success"} 1`)
	require.Contains(t, text, `newapi_channel_model_attempts_total{channel_id="42",model="gpt-5",status="error"} 1`)
	require.Contains(t, text, `newapi_channel_model_input_tokens_total{channel_id="42",model="gpt-5"} 120`)
	require.Contains(t, text, `newapi_channel_model_output_tokens_total{channel_id="42",model="gpt-5"} 30`)

	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "newapi_channel_") || strings.HasPrefix(line, "newapi_channel_info{") {
			continue
		}
		require.NotContains(t, line, "channel_name", line)
	}
}

func TestRecordChannelMetricsRejectsAbnormalStreamSuccess(t *testing.T) {
	tests := []struct {
		name         string
		endReason    relaycommon.StreamEndReason
		wantStatus   string
		wantCategory string
	}{
		{
			name:         "client disconnect",
			endReason:    relaycommon.StreamEndReasonClientGone,
			wantStatus:   "client_cancel",
			wantCategory: "client_cancel",
		},
		{
			name:         "stream timeout",
			endReason:    relaycommon.StreamEndReasonTimeout,
			wantStatus:   "error",
			wantCategory: "timeout",
		},
		{
			name:         "first response timeout",
			endReason:    relaycommon.StreamEndReasonFirstResponseTimeout,
			wantStatus:   "error",
			wantCategory: "timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetPerfMetricsStateForTest(t)

			startedAt := time.Now().Add(-time.Second)
			streamStatus := relaycommon.NewStreamStatus()
			streamStatus.SetEndReason(tt.endReason, nil)
			info := &relaycommon.RelayInfo{
				OriginModelName: "gpt-5",
				StartTime:       startedAt,
				IsStream:        true,
				StreamStatus:    streamStatus,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelId: 42,
				},
			}

			RecordChannelAttempt(info, 42, "primary", startedAt, nil)
			RecordChannelTokens(info, 120, 30)

			text, err := BuildPrometheusText(context.Background())
			require.NoError(t, err)
			require.Contains(t, text, `newapi_channel_attempts_total{channel_id="42",status="`+tt.wantStatus+`",error_category="`+tt.wantCategory+`"} 1`)
			require.NotContains(t, text, `newapi_channel_attempts_total{channel_id="42",status="success"`)
			require.NotContains(t, text, `newapi_channel_model_attempts_total{channel_id="42",model="gpt-5",status="success"}`)
			require.NotContains(t, text, `newapi_channel_model_input_tokens_total{channel_id="42",model="gpt-5"}`)
			require.NotContains(t, text, `newapi_channel_model_output_tokens_total{channel_id="42",model="gpt-5"}`)
		})
	}
}

func TestClassifyChannelError(t *testing.T) {
	tests := []struct {
		name         string
		err          *types.NewAPIError
		wantStatus   string
		wantCategory string
	}{
		{
			name:         "client cancel",
			err:          types.NewError(context.Canceled, types.ErrorCodeDoRequestFailed),
			wantStatus:   "client_cancel",
			wantCategory: "client_cancel",
		},
		{
			name:         "timeout",
			err:          types.NewError(context.DeadlineExceeded, types.ErrorCodeDoRequestFailed),
			wantStatus:   "error",
			wantCategory: "timeout",
		},
		{
			name:         "rate limit",
			err:          types.NewErrorWithStatusCode(errors.New("limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests),
			wantStatus:   "error",
			wantCategory: "rate_limit",
		},
		{
			name:         "auth",
			err:          types.NewErrorWithStatusCode(errors.New("denied"), types.ErrorCodeBadResponseStatusCode, http.StatusUnauthorized),
			wantStatus:   "error",
			wantCategory: "auth",
		},
		{
			name:         "bad response",
			err:          types.NewError(errors.New("invalid body"), types.ErrorCodeBadResponseBody),
			wantStatus:   "error",
			wantCategory: "bad_response",
		},
		{
			name:         "network",
			err:          types.NewError(errors.New("dial failed"), types.ErrorCodeDoRequestFailed),
			wantStatus:   "error",
			wantCategory: "network",
		},
		{
			name:         "upstream 4xx",
			err:          types.NewErrorWithStatusCode(errors.New("teapot"), types.ErrorCodeBadResponseStatusCode, http.StatusTeapot),
			wantStatus:   "error",
			wantCategory: "upstream_4xx",
		},
		{
			name:         "upstream 5xx",
			err:          types.NewErrorWithStatusCode(errors.New("unavailable"), types.ErrorCodeBadResponseStatusCode, http.StatusServiceUnavailable),
			wantStatus:   "error",
			wantCategory: "upstream_5xx",
		},
		{
			name:         "other",
			err:          types.NewErrorWithStatusCode(errors.New("unknown"), types.ErrorCode("unknown"), 0),
			wantStatus:   "error",
			wantCategory: "other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, category := classifyChannelError(tt.err)
			require.Equal(t, tt.wantStatus, status)
			require.Equal(t, tt.wantCategory, category)
		})
	}
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
	require.Contains(t, text, "# HELP newapi_perf_metrics_series Number of application metric series exposed by this endpoint.")
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
	}, true, 0, nil)

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
