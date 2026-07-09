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
	key     bucketKey
	bucket  *atomicBucket
	counter counters
}

type drainedPrometheusBucket struct {
	key     prometheusSeriesKey
	bucket  *prometheusAtomicBucket
	counter prometheusCounters
}

func flushRedisMetricsOnce(ctx context.Context) error {
	if !common.RedisEnabled || common.RDB == nil {
		return nil
	}

	redisDrained := make([]drainedRedisBucket, 0)
	redisPendingBuckets.Range(func(key, value any) bool {
		bucket := value.(*atomicBucket)
		counter := bucket.drain()
		if counter.requestCount == 0 {
			return true
		}
		redisDrained = append(redisDrained, drainedRedisBucket{
			key:     key.(bucketKey),
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
			return true
		}
		prometheusDrained = append(prometheusDrained, drainedPrometheusBucket{
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

	for _, item := range prometheusDrained {
		member := encodePrometheusSeriesKey(item.key)
		redisKey := prometheusRedisKey(item.key)
		pipe.SAdd(ctx, prometheusSeriesSetKey, member)
		pipe.HIncrBy(ctx, redisKey, prometheusCountField, item.counter.count)
		if item.counter.sumMs != 0 {
			pipe.HIncrBy(ctx, redisKey, prometheusSumMsField, item.counter.sumMs)
		}
		for i, value := range item.counter.buckets {
			if value != 0 {
				pipe.HIncrBy(ctx, redisKey, prometheusBucketField(i), value)
			}
		}
	}

	if _, err := pipe.Exec(ctx); err != nil {
		for _, item := range redisDrained {
			item.bucket.addCounters(item.counter)
		}
		for _, item := range prometheusDrained {
			item.bucket.addCounters(item.counter)
		}
		return err
	}
	return nil
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
