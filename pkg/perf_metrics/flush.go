package perfmetrics

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
)

func flushLoop() {
	for {
		interval := perf_metrics_setting.GetFlushIntervalMinutes()
		time.Sleep(time.Duration(interval) * time.Minute)
		setting := perf_metrics_setting.GetSetting()
		if !setting.Enabled {
			continue
		}
		flushCompletedBuckets()
		cleanupExpiredMetrics(setting.RetentionDays)
	}
}

func redisFlushLoop() {
	for {
		interval := perf_metrics_setting.GetRedisFlushIntervalSeconds()
		time.Sleep(time.Duration(interval) * time.Second)
		if !perf_metrics_setting.GetSetting().Enabled {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(interval)*time.Second)
		if err := flushRedisMetricsOnce(ctx); err != nil {
			common.SysError("failed to flush redis perf metrics: " + err.Error())
		}
		cancel()
	}
}

func flushCompletedBuckets() {
	currentBucket := bucketStart(time.Now().Unix())
	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.bucketTs >= currentBucket {
			return true
		}

		bucket := value.(*atomicBucket)
		drained := bucket.drain()
		if drained.requestCount == 0 {
			deleteOldEmptyBucket(k, key)
			return true
		}

		err := model.UpsertPerfMetric(&model.PerfMetric{
			ModelName:      k.model,
			Group:          k.group,
			BucketTs:       k.bucketTs,
			RequestCount:   drained.requestCount,
			SuccessCount:   drained.successCount,
			TotalLatencyMs: drained.totalLatencyMs,
			TtftSumMs:      drained.ttftSumMs,
			TtftCount:      drained.ttftCount,
			OutputTokens:   drained.outputTokens,
			GenerationMs:   drained.generationMs,
		})
		if err != nil {
			bucket.addCounters(drained)
			common.SysError(fmt.Sprintf("failed to flush perf metric bucket model=%s group=%s bucket=%d: %s", k.model, k.group, k.bucketTs, err.Error()))
			return true
		}

		deleteOldEmptyBucket(k, key)
		return true
	})
}

func deleteOldEmptyBucket(k bucketKey, rawKey any) {
	if k.bucketTs < bucketStart(time.Now().Add(-24*time.Hour).Unix()) {
		hotBuckets.Delete(rawKey)
	}
}

func cleanupExpiredMetrics(retentionDays int) {
	if retentionDays <= 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	if err := model.DeletePerfMetricsBefore(cutoff); err != nil {
		common.SysError("failed to cleanup expired perf metrics: " + err.Error())
	}
}

type drainedRedisBucket struct {
	rawKey  any
	key     bucketKey
	bucket  *atomicBucket
	counter counters
}

type drainedPrometheusBucket struct {
	rawKey  any
	key     prometheusSeriesKey
	bucket  *prometheusAtomicBucket
	counter prometheusCounters
}

func flushRedisMetricsOnce(ctx context.Context) error {
	if !common.RedisEnabled || common.RDB == nil {
		return nil
	}

	activeBucket := bucketStart(time.Now().Unix())
	redisDrained := make([]drainedRedisBucket, 0)
	redisPendingBuckets.Range(func(key, value any) bool {
		bucketKey := key.(bucketKey)
		bucket := value.(*atomicBucket)
		counter := bucket.drain()
		if counter.requestCount == 0 {
			deleteHistoricalRedisPendingBucket(key, bucketKey, bucket, activeBucket)
			return true
		}
		redisDrained = append(redisDrained, drainedRedisBucket{
			rawKey:  key,
			key:     bucketKey,
			bucket:  bucket,
			counter: counter,
		})
		return true
	})

	prometheusDrained := make([]drainedPrometheusBucket, 0)
	prometheusPendingBuckets.Range(func(key, value any) bool {
		bucket := value.(*prometheusAtomicBucket)
		counter := bucket.drain()
		if counter.isZero() {
			deleteEmptyPrometheusPendingBucket(key, bucket)
			return true
		}
		prometheusDrained = append(prometheusDrained, drainedPrometheusBucket{
			rawKey:  key,
			key:     key.(prometheusSeriesKey),
			bucket:  bucket,
			counter: counter,
		})
		return true
	})

	if len(redisDrained) == 0 && len(prometheusDrained) == 0 {
		return nil
	}

	pipe := common.RDB.TxPipeline()
	for _, item := range redisDrained {
		redisKey := redisBucketKey(item.key)
		pipe.HIncrBy(ctx, redisKey, "req", item.counter.requestCount)
		if item.counter.successCount != 0 {
			pipe.HIncrBy(ctx, redisKey, "ok", item.counter.successCount)
		}
		if item.counter.totalLatencyMs != 0 {
			pipe.HIncrBy(ctx, redisKey, "lat", item.counter.totalLatencyMs)
		}
		if item.counter.ttftSumMs != 0 {
			pipe.HIncrBy(ctx, redisKey, "ttft", item.counter.ttftSumMs)
		}
		if item.counter.ttftCount != 0 {
			pipe.HIncrBy(ctx, redisKey, "ttft_n", item.counter.ttftCount)
		}
		if item.counter.outputTokens != 0 {
			pipe.HIncrBy(ctx, redisKey, "out", item.counter.outputTokens)
		}
		if item.counter.generationMs != 0 {
			pipe.HIncrBy(ctx, redisKey, "gen_ms", item.counter.generationMs)
		}
		pipe.Expire(ctx, redisKey, 2*time.Hour)
	}

	seriesMembers := make([]interface{}, 0, len(prometheusDrained))
	for _, item := range prometheusDrained {
		addPrometheusInflightCounter(item.key, item.counter)
		seriesMembers = append(seriesMembers, encodePrometheusSeriesKey(item.key))
	}
	if len(seriesMembers) > 0 {
		pipe.SAdd(ctx, prometheusSeriesSetKey, seriesMembers...)
		pipe.Expire(ctx, prometheusSeriesSetKey, prometheusRedisTTL)
	}

	for _, item := range prometheusDrained {
		redisKey := prometheusRedisKey(item.key)
		pipe.HIncrBy(ctx, redisKey, prometheusCountField, item.counter.count)
		if item.counter.sumMs != 0 {
			pipe.HIncrBy(ctx, redisKey, prometheusSumMsField, item.counter.sumMs)
		}
		for i, value := range item.counter.buckets {
			if value != 0 {
				pipe.HIncrBy(ctx, redisKey, prometheusBucketField(i), value)
			}
		}
		pipe.Expire(ctx, redisKey, prometheusRedisTTL)
	}

	if err := ctx.Err(); err != nil {
		requeueDrainedRedisMetrics(redisDrained, prometheusDrained)
		clearPrometheusInflightCounters(prometheusDrained)
		return err
	}
	if _, err := pipe.Exec(ctx); err != nil {
		clearPrometheusInflightCounters(prometheusDrained)
		for _, item := range prometheusDrained {
			deleteEmptyPrometheusPendingBucket(item.rawKey, item.bucket)
		}
		return err
	}
	clearPrometheusInflightCounters(prometheusDrained)
	for _, item := range redisDrained {
		deleteHistoricalRedisPendingBucket(item.rawKey, item.key, item.bucket, activeBucket)
	}
	for _, item := range prometheusDrained {
		deleteEmptyPrometheusPendingBucket(item.rawKey, item.bucket)
	}
	return nil
}

func requeueDrainedRedisMetrics(redisDrained []drainedRedisBucket, prometheusDrained []drainedPrometheusBucket) {
	for _, item := range redisDrained {
		item.bucket.addCounters(item.counter)
	}
	for _, item := range prometheusDrained {
		item.bucket.addCounters(item.counter)
	}
}

func clearPrometheusInflightCounters(prometheusDrained []drainedPrometheusBucket) {
	for _, item := range prometheusDrained {
		prometheusInflightBuckets.Delete(item.key)
	}
}

func deleteHistoricalRedisPendingBucket(rawKey any, key bucketKey, bucket *atomicBucket, activeBucket int64) {
	if key.bucketTs >= activeBucket || bucket.snapshot().requestCount != 0 {
		return
	}
	redisPendingBuckets.Delete(rawKey)
}

func redisCounters(values map[string]string) counters {
	return counters{
		requestCount:   parseRedisInt(values["req"]),
		successCount:   parseRedisInt(values["ok"]),
		totalLatencyMs: parseRedisInt(values["lat"]),
		ttftSumMs:      parseRedisInt(values["ttft"]),
		ttftCount:      parseRedisInt(values["ttft_n"]),
		outputTokens:   parseRedisInt(values["out"]),
		generationMs:   parseRedisInt(values["gen_ms"]),
	}
}

func parseRedisInt(value string) int64 {
	if value == "" {
		return 0
	}
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}
