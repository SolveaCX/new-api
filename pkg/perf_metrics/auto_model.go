package perfmetrics

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var autoModelClassifierBuckets = []float64{0.1, 0.25, 0.5, 1, 2}

type autoModelRequestKey struct {
	protocol string
	route    string
	outcome  string
}

type autoModelMetricStore struct {
	mu                sync.Mutex
	requests          map[autoModelRequestKey]uint64
	classifierBuckets []uint64
	classifierCount   uint64
	classifierSum     float64
	classifierErrors  map[string]uint64
	noEligibleByProto map[string]uint64
}

var autoModelMetrics = autoModelMetricStore{
	requests:          make(map[autoModelRequestKey]uint64),
	classifierBuckets: make([]uint64, len(autoModelClassifierBuckets)),
	classifierErrors:  make(map[string]uint64),
	noEligibleByProto: make(map[string]uint64),
}

func RecordAutoModelRequest(protocol, route, _ string, outcome string) {
	key := autoModelRequestKey{
		protocol: normalizeAutoModelProtocol(protocol),
		route:    normalizeAutoModelRoute(route),
		outcome:  normalizeAutoModelOutcome(outcome),
	}
	autoModelMetrics.mu.Lock()
	autoModelMetrics.requests[key]++
	autoModelMetrics.mu.Unlock()
}

func ObserveAutoModelClassifierDuration(duration time.Duration) {
	seconds := duration.Seconds()
	if seconds < 0 {
		seconds = 0
	}
	autoModelMetrics.mu.Lock()
	for i, upperBound := range autoModelClassifierBuckets {
		if seconds <= upperBound {
			autoModelMetrics.classifierBuckets[i]++
		}
	}
	autoModelMetrics.classifierCount++
	autoModelMetrics.classifierSum += seconds
	autoModelMetrics.mu.Unlock()
}

func RecordAutoModelClassifierError(reason string) {
	reason = normalizeAutoModelErrorReason(reason)
	autoModelMetrics.mu.Lock()
	autoModelMetrics.classifierErrors[reason]++
	autoModelMetrics.mu.Unlock()
}

func RecordAutoModelNoEligibleCandidate(protocol string) {
	protocol = normalizeAutoModelProtocol(protocol)
	autoModelMetrics.mu.Lock()
	autoModelMetrics.noEligibleByProto[protocol]++
	autoModelMetrics.mu.Unlock()
}

type autoModelMetricSnapshot struct {
	requests          map[autoModelRequestKey]uint64
	classifierBuckets []uint64
	classifierCount   uint64
	classifierSum     float64
	classifierErrors  map[string]uint64
	noEligibleByProto map[string]uint64
}

func snapshotAutoModelMetrics() autoModelMetricSnapshot {
	autoModelMetrics.mu.Lock()
	defer autoModelMetrics.mu.Unlock()
	snapshot := autoModelMetricSnapshot{
		requests:          make(map[autoModelRequestKey]uint64, len(autoModelMetrics.requests)),
		classifierBuckets: append([]uint64(nil), autoModelMetrics.classifierBuckets...),
		classifierCount:   autoModelMetrics.classifierCount,
		classifierSum:     autoModelMetrics.classifierSum,
		classifierErrors:  make(map[string]uint64, len(autoModelMetrics.classifierErrors)),
		noEligibleByProto: make(map[string]uint64, len(autoModelMetrics.noEligibleByProto)),
	}
	for key, value := range autoModelMetrics.requests {
		snapshot.requests[key] = value
	}
	for key, value := range autoModelMetrics.classifierErrors {
		snapshot.classifierErrors[key] = value
	}
	for key, value := range autoModelMetrics.noEligibleByProto {
		snapshot.noEligibleByProto[key] = value
	}
	return snapshot
}

func (s autoModelMetricSnapshot) seriesCount() int {
	count := len(s.requests) + len(s.classifierErrors) + len(s.noEligibleByProto)
	if s.classifierCount > 0 {
		count += len(autoModelClassifierBuckets) + 3
	}
	return count
}

func writePrometheusAutoModelMetrics(b *strings.Builder, snapshot autoModelMetricSnapshot) {
	if len(snapshot.requests) > 0 {
		b.WriteString("# HELP newapi_auto_model_requests_total Total Auto Model requests by bounded protocol, route, and outcome.\n")
		b.WriteString("# TYPE newapi_auto_model_requests_total counter\n")
		keys := make([]autoModelRequestKey, 0, len(snapshot.requests))
		for key := range snapshot.requests {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			left := keys[i].protocol + "\x00" + keys[i].route + "\x00" + keys[i].outcome
			right := keys[j].protocol + "\x00" + keys[j].route + "\x00" + keys[j].outcome
			return left < right
		})
		for _, key := range keys {
			b.WriteString(`newapi_auto_model_requests_total{protocol="`)
			b.WriteString(escapePrometheusLabelValue(key.protocol))
			b.WriteString(`",route="`)
			b.WriteString(escapePrometheusLabelValue(key.route))
			b.WriteString(`",outcome="`)
			b.WriteString(escapePrometheusLabelValue(key.outcome))
			b.WriteString(`"} `)
			b.WriteString(strconv.FormatUint(snapshot.requests[key], 10))
			b.WriteByte('\n')
		}
	}
	if snapshot.classifierCount > 0 {
		b.WriteString("# HELP newapi_auto_model_classifier_duration_seconds Auto Model classifier request duration.\n")
		b.WriteString("# TYPE newapi_auto_model_classifier_duration_seconds histogram\n")
		for i, upperBound := range autoModelClassifierBuckets {
			b.WriteString(`newapi_auto_model_classifier_duration_seconds_bucket{le="`)
			b.WriteString(formatPrometheusFloat(upperBound))
			b.WriteString(`"} `)
			b.WriteString(strconv.FormatUint(snapshot.classifierBuckets[i], 10))
			b.WriteByte('\n')
		}
		b.WriteString(`newapi_auto_model_classifier_duration_seconds_bucket{le="+Inf"} `)
		b.WriteString(strconv.FormatUint(snapshot.classifierCount, 10))
		b.WriteByte('\n')
		b.WriteString("newapi_auto_model_classifier_duration_seconds_sum ")
		b.WriteString(formatPrometheusFloat(snapshot.classifierSum))
		b.WriteByte('\n')
		b.WriteString("newapi_auto_model_classifier_duration_seconds_count ")
		b.WriteString(strconv.FormatUint(snapshot.classifierCount, 10))
		b.WriteByte('\n')
	}
	writeAutoModelCounterMap(b, "newapi_auto_model_classifier_errors_total", "Auto Model classifier errors by bounded reason.", "reason", snapshot.classifierErrors)
	writeAutoModelCounterMap(b, "newapi_auto_model_no_eligible_candidate_total", "Auto Model requests with no eligible real model.", "protocol", snapshot.noEligibleByProto)
}

func writeAutoModelCounterMap(b *strings.Builder, name, help, label string, values map[string]uint64) {
	if len(values) == 0 {
		return
	}
	b.WriteString("# HELP " + name + " " + help + "\n")
	b.WriteString("# TYPE " + name + " counter\n")
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		b.WriteString(name + `{` + label + `="`)
		b.WriteString(escapePrometheusLabelValue(key))
		b.WriteString(`"} `)
		b.WriteString(strconv.FormatUint(values[key], 10))
		b.WriteByte('\n')
	}
}

func normalizeAutoModelProtocol(value string) string {
	switch value {
	case "chat", "responses", "messages":
		return value
	default:
		return "unknown"
	}
}

func normalizeAutoModelRoute(value string) string {
	switch value {
	case "general", "coding", "reasoning", "translation":
		return value
	default:
		return "unknown"
	}
}

func normalizeAutoModelOutcome(value string) string {
	switch value {
	case "selected", "fallback", "rejected":
		return value
	default:
		return "rejected"
	}
}

func normalizeAutoModelErrorReason(value string) string {
	switch value {
	case "timeout", "http_status", "invalid_json", "invalid_route", "response_too_large", "config":
		return value
	default:
		return "config"
	}
}
