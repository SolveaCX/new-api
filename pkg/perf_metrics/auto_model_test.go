package perfmetrics

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func resetAutoModelMetricsForTest() {
	autoModelMetrics = autoModelMetricStore{
		requests:          make(map[autoModelRequestKey]uint64),
		classifierBuckets: make([]uint64, len(autoModelClassifierBuckets)),
		classifierErrors:  make(map[string]uint64),
		noEligibleByProto: make(map[string]uint64),
	}
}

func TestAutoModelMetricsUseBoundedLabelsAndRender(t *testing.T) {
	resetAutoModelMetricsForTest()
	t.Cleanup(resetAutoModelMetricsForTest)
	RecordAutoModelRequest("responses", "coding", "gpt-5-mini", "selected")
	RecordAutoModelRequest("unexpected", "unexpected", "", "unexpected")
	ObserveAutoModelClassifierDuration(175 * time.Millisecond)
	RecordAutoModelClassifierError("timeout")
	RecordAutoModelClassifierError("secret-value")
	RecordAutoModelNoEligibleCandidate("messages")

	output, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	require.Contains(t, output, `newapi_auto_model_requests_total{protocol="responses",route="coding",outcome="selected"} 1`)
	require.Contains(t, output, `newapi_auto_model_requests_total{protocol="unknown",route="unknown",outcome="rejected"} 1`)
	require.Contains(t, output, `newapi_auto_model_classifier_errors_total{reason="timeout"} 1`)
	require.Contains(t, output, `newapi_auto_model_classifier_errors_total{reason="config"} 1`)
	require.Contains(t, output, `newapi_auto_model_no_eligible_candidate_total{protocol="messages"} 1`)
	require.Contains(t, output, "newapi_auto_model_classifier_duration_seconds_count 1")
	require.False(t, strings.Contains(output, "secret-value"))
	require.NotContains(t, output, "selected_model")
}

func TestAutoModelRequestSeriesIgnoreSelectedModel(t *testing.T) {
	resetAutoModelMetricsForTest()
	t.Cleanup(resetAutoModelMetricsForTest)
	for i := 0; i < 100; i++ {
		RecordAutoModelRequest("chat", "general", "model-"+strings.Repeat("x", i+1), "selected")
	}
	snapshot := snapshotAutoModelMetrics()
	require.Len(t, snapshot.requests, 1)
	for _, count := range snapshot.requests {
		require.Equal(t, uint64(100), count)
	}
}
