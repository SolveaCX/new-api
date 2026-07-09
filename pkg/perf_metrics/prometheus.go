package perfmetrics

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	prometheusFiniteBucketCount  = 9
	prometheusLatencyBucketCount = prometheusFiniteBucketCount + 1
	prometheusInfBucketIndex     = prometheusLatencyBucketCount - 1

	prometheusMaxSeriesPerScrapeEnv     = "PROMETHEUS_MAX_SERIES_PER_SCRAPE"
	prometheusChannelLabelEnabledEnv    = "PROMETHEUS_METRICS_CHANNEL_LABEL_ENABLED"
	defaultPrometheusMaxSeriesPerScrape = 5000
	prometheusSeriesIdleRetention       = 6 * time.Hour
)

var prometheusLatencyBucketUpperBoundsMs = [prometheusFiniteBucketCount]int64{
	500,
	1000,
	2000,
	5000,
	10000,
	30000,
	60000,
	120000,
	300000,
}

func BuildPrometheusText(_ context.Context) (string, error) {
	series := map[prometheusSeriesKey]prometheusCounters{}
	mergePrometheusPendingSnapshots(series)
	if maxSeries := prometheusMaxSeriesPerScrape(); maxSeries > 0 && len(series) > maxSeries {
		return "", fmt.Errorf("prometheus series limit exceeded: %d > %d", len(series), maxSeries)
	}

	var b strings.Builder
	b.WriteString("# HELP newapi_perf_metrics_series Number of model/channel/status series exposed by this endpoint.\n")
	b.WriteString("# TYPE newapi_perf_metrics_series gauge\n")
	b.WriteString(fmt.Sprintf("newapi_perf_metrics_series %d\n", len(series)))

	b.WriteString("# HELP newapi_model_request_duration_seconds Model request latency in seconds.\n")
	b.WriteString("# TYPE newapi_model_request_duration_seconds histogram\n")
	keys := sortedPrometheusSeriesKeys(series)
	for _, key := range keys {
		counter := series[key]
		if counter.count == 0 {
			continue
		}
		for i, count := range counter.buckets {
			b.WriteString("newapi_model_request_duration_seconds_bucket{")
			writePrometheusLabels(&b, key)
			b.WriteString(`,le="`)
			b.WriteString(prometheusBucketLabel(i))
			b.WriteString(`"} `)
			b.WriteString(strconv.FormatInt(count, 10))
			b.WriteByte('\n')
		}
		b.WriteString("newapi_model_request_duration_seconds_sum{")
		writePrometheusLabels(&b, key)
		b.WriteString("} ")
		b.WriteString(formatSeconds(counter.sumMs))
		b.WriteByte('\n')
		b.WriteString("newapi_model_request_duration_seconds_count{")
		writePrometheusLabels(&b, key)
		b.WriteString("} ")
		b.WriteString(strconv.FormatInt(counter.count, 10))
		b.WriteByte('\n')
	}

	b.WriteString("# HELP newapi_model_requests_total Total model requests by model, channel and status.\n")
	b.WriteString("# TYPE newapi_model_requests_total counter\n")
	for _, key := range keys {
		counter := series[key]
		if counter.count == 0 {
			continue
		}
		b.WriteString("newapi_model_requests_total{")
		writePrometheusLabels(&b, key)
		b.WriteString("} ")
		b.WriteString(strconv.FormatInt(counter.count, 10))
		b.WriteByte('\n')
	}

	return b.String(), nil
}

func mergePrometheusPendingSnapshots(series map[prometheusSeriesKey]prometheusCounters) {
	idleCutoff := time.Now().Add(-prometheusSeriesIdleRetention).UnixNano()
	prometheusPendingBuckets.Range(func(key, value any) bool {
		bucket := value.(*prometheusLockedBucket)
		if bucket.retireIfIdle(idleCutoff) {
			prometheusPendingBuckets.CompareAndDelete(key, value)
			return true
		}
		seriesKey := normalizePrometheusSeriesKey(key.(prometheusSeriesKey))
		current := series[seriesKey]
		current.add(bucket.snapshot())
		series[seriesKey] = current
		return true
	})
}

func normalizePrometheusSeriesKey(key prometheusSeriesKey) prometheusSeriesKey {
	if !prometheusChannelLabelEnabled() {
		key.channelID = 0
	}
	return key
}

func sortedPrometheusSeriesKeys(series map[prometheusSeriesKey]prometheusCounters) []prometheusSeriesKey {
	keys := make([]prometheusSeriesKey, 0, len(series))
	for key := range series {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].model != keys[j].model {
			return keys[i].model < keys[j].model
		}
		if keys[i].channelID != keys[j].channelID {
			return keys[i].channelID < keys[j].channelID
		}
		return keys[i].status < keys[j].status
	})
	return keys
}

func writePrometheusLabels(b *strings.Builder, key prometheusSeriesKey) {
	b.WriteString(`model="`)
	b.WriteString(escapePrometheusLabelValue(key.model))
	b.WriteString(`",channel_id="`)
	b.WriteString(escapePrometheusLabelValue(prometheusChannelLabel(key.channelID)))
	b.WriteString(`",status="`)
	b.WriteString(escapePrometheusLabelValue(key.status))
	b.WriteByte('"')
}

func prometheusChannelLabel(channelID int) string {
	if channelID <= 0 {
		return "unknown"
	}
	return strconv.Itoa(channelID)
}

func prometheusBucketLabel(index int) string {
	if index == prometheusInfBucketIndex {
		return "+Inf"
	}
	return strconv.FormatFloat(float64(prometheusLatencyBucketUpperBoundsMs[index])/1000, 'f', -1, 64)
}

func formatSeconds(milliseconds int64) string {
	return strconv.FormatFloat(float64(milliseconds)/1000, 'f', 3, 64)
}

func escapePrometheusLabelValue(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\n", "\\n",
		"\"", "\\\"",
	)
	return replacer.Replace(value)
}

func prometheusMaxSeriesPerScrape() int {
	return common.GetEnvOrDefault(prometheusMaxSeriesPerScrapeEnv, defaultPrometheusMaxSeriesPerScrape)
}

func prometheusChannelLabelEnabled() bool {
	return common.GetEnvOrDefaultBool(prometheusChannelLabelEnabledEnv, true)
}
