package perfmetrics

import (
	"context"
	"errors"
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
