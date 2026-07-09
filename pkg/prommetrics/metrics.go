package prommetrics

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var labelNames = []string{
	"service_role",
	"model",
	"group",
	"relay_format",
	"status_class",
	"success",
}

type Recorder struct {
	registry      *prometheus.Registry
	serviceRole   string
	requests      *prometheus.CounterVec
	latency       *prometheus.HistogramVec
	ttft          *prometheus.HistogramVec
	outputTokens  *prometheus.CounterVec
	metricsHandle http.Handler
}

func NewRecorder(serviceRole string) *Recorder {
	serviceRole = strings.TrimSpace(serviceRole)
	if serviceRole == "" {
		serviceRole = "router"
	}

	recorder := &Recorder{
		registry:    prometheus.NewRegistry(),
		serviceRole: serviceRole,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flatkey_relay_requests_total",
			Help: "Total Flatkey relay requests grouped by safe routing labels.",
		}, labelNames),
		latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "flatkey_relay_latency_seconds",
			Help:    "End-to-end Flatkey relay latency in seconds.",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60},
		}, labelNames),
		ttft: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "flatkey_relay_ttft_seconds",
			Help:    "Flatkey streaming relay time to first token in seconds.",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60},
		}, labelNames),
		outputTokens: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flatkey_relay_output_tokens_total",
			Help: "Total Flatkey relay output tokens reported by upstream usage.",
		}, labelNames),
	}
	recorder.registry.MustRegister(
		recorder.requests,
		recorder.latency,
		recorder.ttft,
		recorder.outputTokens,
	)
	recorder.metricsHandle = promhttp.HandlerFor(recorder.registry, promhttp.HandlerOpts{})
	return recorder
}

func (r *Recorder) Handler() http.Handler {
	return r.metricsHandle
}

func (r *Recorder) RecordRelaySample(info *relaycommon.RelayInfo, success bool, statusCode int, outputTokens int64) {
	if r == nil || info == nil {
		return
	}
	model := strings.TrimSpace(info.OriginModelName)
	if model == "" {
		return
	}
	group := strings.TrimSpace(info.UsingGroup)
	if group == "" {
		group = "default"
	}
	relayFormat := string(info.GetFinalRequestRelayFormat())
	if relayFormat == "" {
		relayFormat = "unknown"
	}

	labels := prometheus.Labels{
		"service_role": r.serviceRole,
		"model":        model,
		"group":        group,
		"relay_format": relayFormat,
		"status_class": statusClass(statusCode, success),
		"success":      fmt.Sprintf("%t", success),
	}
	r.requests.With(labels).Inc()

	if latency := latencySeconds(info.StartTime, time.Now()); latency >= 0 {
		r.latency.With(labels).Observe(latency)
	}
	if info.IsStream && info.HasSendResponse() {
		if ttft := latencySeconds(info.StartTime, info.FirstResponseTime); ttft >= 0 {
			r.ttft.With(labels).Observe(ttft)
		}
	}
	if outputTokens > 0 {
		r.outputTokens.With(labels).Add(float64(outputTokens))
	}
}

func statusClass(statusCode int, success bool) string {
	if statusCode >= 100 && statusCode <= 599 {
		return fmt.Sprintf("%dxx", statusCode/100)
	}
	if success {
		return "2xx"
	}
	return "error"
}

func latencySeconds(start time.Time, end time.Time) float64 {
	if start.IsZero() || end.IsZero() {
		return 0
	}
	duration := end.Sub(start)
	if duration < 0 {
		return 0
	}
	return duration.Seconds()
}

var (
	defaultMu       sync.RWMutex
	defaultRecorder = NewRecorder(defaultServiceRole())
)

func Handler() http.Handler {
	defaultMu.RLock()
	recorder := defaultRecorder
	defaultMu.RUnlock()
	return recorder.Handler()
}

func RecordRelaySample(info *relaycommon.RelayInfo, success bool, statusCode int, outputTokens int64) {
	defaultMu.RLock()
	recorder := defaultRecorder
	defaultMu.RUnlock()
	recorder.RecordRelaySample(info, success, statusCode, outputTokens)
}

func ResetDefaultForTest() {
	defaultMu.Lock()
	defaultRecorder = NewRecorder(defaultServiceRole())
	defaultMu.Unlock()
}

func defaultServiceRole() string {
	return common.GetEnvOrDefaultString("PROMETHEUS_METRICS_SERVICE_ROLE", "router")
}
