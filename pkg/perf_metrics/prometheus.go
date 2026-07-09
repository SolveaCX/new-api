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
	if maxSeries := prometheusMaxSeriesPerScrape(); maxSeries > 0 && len(series) > maxSeries {
		return "", fmt.Errorf("prometheus series limit exceeded: %d > %d", len(series), maxSeries)
	}

	var b strings.Builder
	b.WriteString("# HELP newapi_perf_metrics_series Number of model/status series exposed by this endpoint.\n")
	b.WriteString("# TYPE newapi_perf_metrics_series gauge\n")
	b.WriteString(fmt.Sprintf("newapi_perf_metrics_series %d\n", len(series)))

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
