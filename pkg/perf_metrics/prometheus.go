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
	prometheusMaxSeriesPerScrapeEnv     = "PROMETHEUS_MAX_SERIES_PER_SCRAPE"
	defaultPrometheusMaxSeriesPerScrape = 5000
	prometheusSeriesIdleRetention       = 6 * time.Hour
)

func BuildPrometheusText(_ context.Context) (string, error) {
	series := map[prometheusSeriesKey]prometheusCounters{}
	mergePrometheusPendingSnapshots(series)
	channelSnapshots := snapshotPrometheusChannels()
	channelModelSnapshots := snapshotPrometheusChannelModels()
	seriesCount := len(series)
	for _, snapshot := range channelSnapshots {
		seriesCount += snapshot.seriesCount()
	}
	for _, snapshot := range channelModelSnapshots {
		seriesCount += snapshot.seriesCount()
	}
	if maxSeries := prometheusMaxSeriesPerScrape(); maxSeries > 0 && seriesCount > maxSeries {
		return "", fmt.Errorf("prometheus series limit exceeded: %d > %d", seriesCount, maxSeries)
	}

	var b strings.Builder
	b.WriteString("# HELP newapi_perf_metrics_series Number of application metric series exposed by this endpoint.\n")
	b.WriteString("# TYPE newapi_perf_metrics_series gauge\n")
	b.WriteString(fmt.Sprintf("newapi_perf_metrics_series %d\n", seriesCount))

	keys := sortedPrometheusSeriesKeys(series)
	b.WriteString("# HELP newapi_model_requests_total Total model requests by model and status.\n")
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

	writePrometheusChannelMetrics(&b, channelSnapshots)
	writePrometheusChannelModelMetrics(&b, channelModelSnapshots)

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
		seriesKey := key.(prometheusSeriesKey)
		current := series[seriesKey]
		current.add(bucket.snapshot())
		series[seriesKey] = current
		return true
	})
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
		return keys[i].status < keys[j].status
	})
	return keys
}

func snapshotPrometheusChannels() []prometheusChannelSnapshot {
	idleCutoff := time.Now().Add(-prometheusSeriesIdleRetention).UnixNano()
	snapshots := make([]prometheusChannelSnapshot, 0)
	prometheusChannelBuckets.Range(func(key, value any) bool {
		bucket := value.(*prometheusChannelBucket)
		if bucket.retireIfIdle(idleCutoff) {
			prometheusChannelBuckets.CompareAndDelete(key, value)
			return true
		}
		snapshot := bucket.snapshot(key.(int))
		if snapshot.durationCount > 0 {
			snapshots = append(snapshots, snapshot)
		}
		return true
	})
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].channelID < snapshots[j].channelID
	})
	return snapshots
}

func snapshotPrometheusChannelModels() []prometheusChannelModelSnapshot {
	idleCutoff := time.Now().Add(-prometheusSeriesIdleRetention).UnixNano()
	snapshots := make([]prometheusChannelModelSnapshot, 0)
	prometheusChannelModelBuckets.Range(func(key, value any) bool {
		bucket := value.(*prometheusChannelModelBucket)
		if bucket.retireIfIdle(idleCutoff) {
			prometheusChannelModelBuckets.CompareAndDelete(key, value)
			return true
		}
		seriesKey := key.(prometheusChannelModelKey)
		snapshot := bucket.snapshot(seriesKey)
		if snapshot.seriesCount() > 0 {
			snapshots = append(snapshots, snapshot)
		}
		return true
	})
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].channelID != snapshots[j].channelID {
			return snapshots[i].channelID < snapshots[j].channelID
		}
		return snapshots[i].model < snapshots[j].model
	})
	return snapshots
}

func writePrometheusChannelMetrics(b *strings.Builder, snapshots []prometheusChannelSnapshot) {
	if len(snapshots) == 0 {
		return
	}

	b.WriteString("# HELP newapi_channel_info Current display name for a stable channel ID.\n")
	b.WriteString("# TYPE newapi_channel_info gauge\n")
	for _, snapshot := range snapshots {
		b.WriteString("newapi_channel_info{")
		writePrometheusChannelIDLabel(b, snapshot.channelID)
		b.WriteString(`,channel_name="`)
		b.WriteString(escapePrometheusLabelValue(snapshot.channelName))
		b.WriteString(`"} 1`)
		b.WriteByte('\n')
	}

	b.WriteString("# HELP newapi_channel_attempts_total Total real upstream channel attempts, including retries.\n")
	b.WriteString("# TYPE newapi_channel_attempts_total counter\n")
	for _, snapshot := range snapshots {
		keys := sortedPrometheusChannelAttemptKeys(snapshot.attempts)
		for _, key := range keys {
			b.WriteString("newapi_channel_attempts_total{")
			writePrometheusChannelIDLabel(b, snapshot.channelID)
			b.WriteString(`,status="`)
			b.WriteString(escapePrometheusLabelValue(key.status))
			b.WriteString(`",error_category="`)
			b.WriteString(escapePrometheusLabelValue(key.errorCategory))
			b.WriteString(`"} `)
			b.WriteString(strconv.FormatInt(snapshot.attempts[key], 10))
			b.WriteByte('\n')
		}
	}

	b.WriteString("# HELP newapi_channel_request_duration_seconds Total duration of real upstream channel attempts.\n")
	b.WriteString("# TYPE newapi_channel_request_duration_seconds histogram\n")
	for _, snapshot := range snapshots {
		for i, upperBound := range prometheusChannelDurationBucketsSeconds {
			b.WriteString("newapi_channel_request_duration_seconds_bucket{")
			writePrometheusChannelIDLabel(b, snapshot.channelID)
			b.WriteString(`,le="`)
			b.WriteString(formatPrometheusFloat(upperBound))
			b.WriteString(`"} `)
			b.WriteString(strconv.FormatInt(snapshot.durationBuckets[i], 10))
			b.WriteByte('\n')
		}
		b.WriteString("newapi_channel_request_duration_seconds_bucket{")
		writePrometheusChannelIDLabel(b, snapshot.channelID)
		b.WriteString(`,le="+Inf"} `)
		b.WriteString(strconv.FormatInt(snapshot.durationCount, 10))
		b.WriteByte('\n')
		b.WriteString("newapi_channel_request_duration_seconds_sum{")
		writePrometheusChannelIDLabel(b, snapshot.channelID)
		b.WriteString("} ")
		b.WriteString(formatPrometheusFloat(snapshot.durationSumSeconds))
		b.WriteByte('\n')
		b.WriteString("newapi_channel_request_duration_seconds_count{")
		writePrometheusChannelIDLabel(b, snapshot.channelID)
		b.WriteString("} ")
		b.WriteString(strconv.FormatInt(snapshot.durationCount, 10))
		b.WriteByte('\n')
	}

	b.WriteString("# HELP newapi_channel_ttft_seconds Time to first token for streaming channel attempts.\n")
	b.WriteString("# TYPE newapi_channel_ttft_seconds summary\n")
	for _, snapshot := range snapshots {
		if snapshot.ttftCount == 0 {
			continue
		}
		b.WriteString("newapi_channel_ttft_seconds_sum{")
		writePrometheusChannelIDLabel(b, snapshot.channelID)
		b.WriteString("} ")
		b.WriteString(formatPrometheusFloat(snapshot.ttftSumSeconds))
		b.WriteByte('\n')
		b.WriteString("newapi_channel_ttft_seconds_count{")
		writePrometheusChannelIDLabel(b, snapshot.channelID)
		b.WriteString("} ")
		b.WriteString(strconv.FormatInt(snapshot.ttftCount, 10))
		b.WriteByte('\n')
	}
}

func writePrometheusChannelModelMetrics(b *strings.Builder, snapshots []prometheusChannelModelSnapshot) {
	if len(snapshots) == 0 {
		return
	}

	b.WriteString("# HELP newapi_channel_model_attempts_total Total channel attempts by model and outcome.\n")
	b.WriteString("# TYPE newapi_channel_model_attempts_total counter\n")
	for _, snapshot := range snapshots {
		statuses := make([]string, 0, len(snapshot.attempts))
		for status := range snapshot.attempts {
			statuses = append(statuses, status)
		}
		sort.Strings(statuses)
		for _, status := range statuses {
			b.WriteString("newapi_channel_model_attempts_total{")
			writePrometheusChannelModelLabels(b, snapshot.channelID, snapshot.model)
			b.WriteString(`,status="`)
			b.WriteString(escapePrometheusLabelValue(status))
			b.WriteString(`"} `)
			b.WriteString(strconv.FormatInt(snapshot.attempts[status], 10))
			b.WriteByte('\n')
		}
	}

	b.WriteString("# HELP newapi_channel_model_input_tokens_total Actual input tokens settled by channel and model.\n")
	b.WriteString("# TYPE newapi_channel_model_input_tokens_total counter\n")
	for _, snapshot := range snapshots {
		if snapshot.inputTokens == 0 {
			continue
		}
		b.WriteString("newapi_channel_model_input_tokens_total{")
		writePrometheusChannelModelLabels(b, snapshot.channelID, snapshot.model)
		b.WriteString("} ")
		b.WriteString(strconv.FormatInt(snapshot.inputTokens, 10))
		b.WriteByte('\n')
	}

	b.WriteString("# HELP newapi_channel_model_output_tokens_total Actual output tokens settled by channel and model.\n")
	b.WriteString("# TYPE newapi_channel_model_output_tokens_total counter\n")
	for _, snapshot := range snapshots {
		if snapshot.outputTokens == 0 {
			continue
		}
		b.WriteString("newapi_channel_model_output_tokens_total{")
		writePrometheusChannelModelLabels(b, snapshot.channelID, snapshot.model)
		b.WriteString("} ")
		b.WriteString(strconv.FormatInt(snapshot.outputTokens, 10))
		b.WriteByte('\n')
	}
}

func sortedPrometheusChannelAttemptKeys(attempts map[prometheusChannelAttemptKey]int64) []prometheusChannelAttemptKey {
	keys := make([]prometheusChannelAttemptKey, 0, len(attempts))
	for key := range attempts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].status != keys[j].status {
			return keys[i].status < keys[j].status
		}
		return keys[i].errorCategory < keys[j].errorCategory
	})
	return keys
}

func writePrometheusChannelIDLabel(b *strings.Builder, channelID int) {
	b.WriteString(`channel_id="`)
	b.WriteString(strconv.Itoa(channelID))
	b.WriteByte('"')
}

func writePrometheusChannelModelLabels(b *strings.Builder, channelID int, modelName string) {
	writePrometheusChannelIDLabel(b, channelID)
	b.WriteString(`,model="`)
	b.WriteString(escapePrometheusLabelValue(modelName))
	b.WriteByte('"')
}

func formatPrometheusFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func writePrometheusLabels(b *strings.Builder, key prometheusSeriesKey) {
	b.WriteString(`model="`)
	b.WriteString(escapePrometheusLabelValue(key.model))
	b.WriteString(`",status="`)
	b.WriteString(escapePrometheusLabelValue(key.status))
	b.WriteByte('"')
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
