package prommetrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

func TestRecordRelaySampleExportsSafeRelayMetrics(t *testing.T) {
	recorder := NewRecorder("router")
	start := time.Now().Add(-1500 * time.Millisecond)
	info := &relaycommon.RelayInfo{
		OriginModelName:   "gpt-5.4",
		UsingGroup:        "paid",
		StartTime:         start,
		FirstResponseTime: start.Add(250 * time.Millisecond),
		IsStream:          true,
		RelayFormat:       types.RelayFormatOpenAI,
	}
	info.InitRequestConversionChain()

	recorder.RecordRelaySample(info, true, http.StatusOK, 42)

	body := scrapeMetrics(t, recorder)
	for _, want := range []string{
		"flatkey_relay_requests_total",
		"flatkey_relay_latency_seconds_bucket",
		"flatkey_relay_ttft_seconds_bucket",
		"flatkey_relay_output_tokens_total",
		`service_role="router"`,
		`model="gpt-5.4"`,
		`group="paid"`,
		`relay_format="openai"`,
		`status_class="2xx"`,
		`success="true"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected metrics output to contain %q:\n%s", want, body)
		}
	}

	for _, forbidden := range []string{
		"user_id=",
		"token_id=",
		"api_key=",
		"request_id=",
		"email=",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("metrics output contains forbidden high-cardinality or sensitive label %q:\n%s", forbidden, body)
		}
	}
}

func TestRecordRelaySampleGroupsFailureStatusClass(t *testing.T) {
	recorder := NewRecorder("router")
	recorder.RecordRelaySample(&relaycommon.RelayInfo{
		OriginModelName: "gpt-5.4-mini",
		UsingGroup:      "",
		StartTime:       time.Now().Add(-250 * time.Millisecond),
		RelayFormat:     types.RelayFormatClaude,
	}, false, http.StatusTooManyRequests, 0)

	body := scrapeMetrics(t, recorder)
	for _, want := range []string{
		`model="gpt-5.4-mini"`,
		`group="default"`,
		`relay_format="claude"`,
		`status_class="4xx"`,
		`success="false"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected metrics output to contain %q:\n%s", want, body)
		}
	}
}

func scrapeMetrics(t *testing.T, recorder *Recorder) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	recorder.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics scrape returned status %d", rec.Code)
	}
	return rec.Body.String()
}
