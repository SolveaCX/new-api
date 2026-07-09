package perfmetrics

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	prometheusFiniteBucketCount  = 9
	prometheusLatencyBucketCount = prometheusFiniteBucketCount + 1
	prometheusInfBucketIndex     = prometheusLatencyBucketCount - 1

	prometheusSeriesSetKey = "newapi:perf:prometheus:series"
	prometheusCountField   = "count"
	prometheusSumMsField   = "sum_ms"
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

func BuildPrometheusText(ctx context.Context) (string, error) {
	series := map[prometheusSeriesKey]prometheusCounters{}
	redisAvailable := false

	if common.RedisEnabled && common.RDB != nil {
		members, err := common.RDB.SMembers(ctx, prometheusSeriesSetKey).Result()
		if err == nil {
			redisAvailable = true
			for _, member := range members {
				key, ok := decodePrometheusSeriesKey(member)
				if !ok {
					continue
				}
				values, err := common.RDB.HGetAll(ctx, prometheusRedisKey(key)).Result()
				if err != nil || len(values) == 0 {
					continue
				}
				current := series[key]
				current.add(redisPrometheusCounters(values))
				series[key] = current
			}
		}
	}

	prometheusPendingBuckets.Range(func(key, value any) bool {
		seriesKey := key.(prometheusSeriesKey)
		current := series[seriesKey]
		current.add(value.(*prometheusAtomicBucket).snapshot())
		series[seriesKey] = current
		return true
	})

	var b strings.Builder
	b.WriteString("# HELP newapi_perf_metrics_redis_available Whether Redis-backed metrics aggregation is available.\n")
	b.WriteString("# TYPE newapi_perf_metrics_redis_available gauge\n")
	if redisAvailable {
		b.WriteString("newapi_perf_metrics_redis_available 1\n")
	} else {
		b.WriteString("newapi_perf_metrics_redis_available 0\n")
	}
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

func prometheusBucketField(index int) string {
	if index == prometheusInfBucketIndex {
		return "bucket:+Inf"
	}
	return fmt.Sprintf("bucket:%d", prometheusLatencyBucketUpperBoundsMs[index])
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

func encodePrometheusSeriesKey(key prometheusSeriesKey) string {
	return strings.Join([]string{
		key.model,
		strconv.Itoa(key.channelID),
		key.status,
	}, "\x1f")
}

func decodePrometheusSeriesKey(value string) (prometheusSeriesKey, bool) {
	parts := strings.Split(value, "\x1f")
	if len(parts) != 3 {
		return prometheusSeriesKey{}, false
	}
	channelID, err := strconv.Atoi(parts[1])
	if err != nil {
		return prometheusSeriesKey{}, false
	}
	return prometheusSeriesKey{
		model:     parts[0],
		channelID: channelID,
		status:    parts[2],
	}, true
}

func prometheusRedisKey(key prometheusSeriesKey) string {
	hash := sha1.Sum([]byte(encodePrometheusSeriesKey(key)))
	return "newapi:perf:prometheus:" + hex.EncodeToString(hash[:])
}

func redisPrometheusCounters(values map[string]string) prometheusCounters {
	out := prometheusCounters{
		count: parseRedisInt(values[prometheusCountField]),
		sumMs: parseRedisInt(values[prometheusSumMsField]),
	}
	for i := range out.buckets {
		out.buckets[i] = parseRedisInt(values[prometheusBucketField(i)])
	}
	return out
}
