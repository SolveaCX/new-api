package perfmetrics

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
)

const (
	prometheusFiniteBucketCount  = 9
	prometheusLatencyBucketCount = prometheusFiniteBucketCount + 1
	prometheusInfBucketIndex     = prometheusLatencyBucketCount - 1

	prometheusSeriesSetKey = "newapi:perf:prometheus:series"
	prometheusCountField   = "count"
	prometheusSumMsField   = "sum_ms"
	prometheusRedisTTL     = 24 * time.Hour
	prometheusScanCount    = 100

	prometheusMaxSeriesPerScrapeEnv     = "PROMETHEUS_MAX_SERIES_PER_SCRAPE"
	prometheusChannelLabelEnabledEnv    = "PROMETHEUS_METRICS_CHANNEL_LABEL_ENABLED"
	defaultPrometheusMaxSeriesPerScrape = 5000
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
		redisSeries := map[prometheusSeriesKey]prometheusCounters{}
		redisAvailable = mergeRedisPrometheusSeries(ctx, redisSeries)
		if redisAvailable {
			for key, counter := range redisSeries {
				series[key] = counter
			}
		}
	}

	mergePrometheusPendingSnapshots(series)

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

func mergeRedisPrometheusSeries(ctx context.Context, series map[prometheusSeriesKey]prometheusCounters) bool {
	cursor := uint64(0)
	redisAvailable := true
	staleMembers := make([]string, 0)
	seen := 0
	maxSeries := prometheusMaxSeriesPerScrape()
	for {
		members, nextCursor, err := common.RDB.SScan(ctx, prometheusSeriesSetKey, cursor, "", prometheusScanCount).Result()
		if err != nil {
			return false
		}
		seen += len(members)
		if maxSeries > 0 && seen > maxSeries {
			return false
		}
		if len(members) > 0 {
			if ok := mergeRedisPrometheusSeriesBatch(ctx, members, series, &staleMembers); !ok {
				redisAvailable = false
				break
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	pruneStalePrometheusSeries(ctx, staleMembers)
	return redisAvailable
}

func mergeRedisPrometheusSeriesBatch(ctx context.Context, members []string, series map[prometheusSeriesKey]prometheusCounters, staleMembers *[]string) bool {
	keys := make([]prometheusSeriesKey, 0, len(members))
	validMembers := make([]string, 0, len(members))
	pipe := common.RDB.Pipeline()
	cmds := make([]*redis.StringStringMapCmd, 0, len(members))
	for _, member := range members {
		key, ok := decodePrometheusSeriesKey(member)
		if !ok {
			*staleMembers = append(*staleMembers, member)
			continue
		}
		keys = append(keys, key)
		validMembers = append(validMembers, member)
		cmds = append(cmds, pipe.HGetAll(ctx, prometheusRedisKey(key)))
	}
	if len(cmds) == 0 {
		return true
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return false
	}
	for i, cmd := range cmds {
		values, err := cmd.Result()
		if err != nil || len(values) == 0 {
			*staleMembers = append(*staleMembers, validMembers[i])
			continue
		}
		key := keys[i]
		current := series[key]
		current.add(redisPrometheusCounters(values))
		series[key] = current
	}
	return true
}

func pruneStalePrometheusSeries(ctx context.Context, members []string) {
	if len(members) == 0 {
		return
	}
	pipe := common.RDB.Pipeline()
	for _, member := range members {
		pipe.SRem(ctx, prometheusSeriesSetKey, member)
	}
	_, _ = pipe.Exec(ctx)
}

func mergePrometheusPendingSnapshots(series map[prometheusSeriesKey]prometheusCounters) {
	prometheusPendingBuckets.Range(func(key, value any) bool {
		seriesKey := key.(prometheusSeriesKey)
		current := series[seriesKey]
		current.add(value.(*prometheusAtomicBucket).snapshot())
		series[seriesKey] = current
		return true
	})
	prometheusInflightBuckets.Range(func(key, value any) bool {
		seriesKey := key.(prometheusSeriesKey)
		current := series[seriesKey]
		current.add(value.(*prometheusInflightBucket).snapshot())
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
	payload, err := common.Marshal([]string{
		key.model,
		strconv.Itoa(key.channelID),
		key.status,
	})
	if err != nil {
		return strings.Join([]string{key.model, strconv.Itoa(key.channelID), key.status}, "\x1f")
	}
	return string(payload)
}

func decodePrometheusSeriesKey(value string) (prometheusSeriesKey, bool) {
	parts, ok := decodeJSONPrometheusSeriesKey(value)
	if !ok {
		parts = strings.Split(value, "\x1f")
	}
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

func decodeJSONPrometheusSeriesKey(value string) ([]string, bool) {
	if !strings.HasPrefix(value, "[") {
		return nil, false
	}
	var parts []string
	if err := common.Unmarshal([]byte(value), &parts); err != nil {
		return nil, false
	}
	return parts, true
}

func prometheusRedisKey(key prometheusSeriesKey) string {
	hash := sha1.Sum([]byte(encodePrometheusSeriesKey(key)))
	return "newapi:perf:prometheus:" + hex.EncodeToString(hash[:])
}

func prometheusMaxSeriesPerScrape() int {
	return common.GetEnvOrDefault(prometheusMaxSeriesPerScrapeEnv, defaultPrometheusMaxSeriesPerScrape)
}

func prometheusChannelLabelEnabled() bool {
	return common.GetEnvOrDefaultBool(prometheusChannelLabelEnabledEnv, true)
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
