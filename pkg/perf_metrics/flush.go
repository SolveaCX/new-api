package perfmetrics

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
)

const availabilityFlushInterval = 5 * time.Second

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

func flushAvailabilityLoop() {
	ticker := time.NewTicker(availabilityFlushInterval)
	defer ticker.Stop()
	for range ticker.C {
		if !perf_metrics_setting.GetSetting().Enabled {
			continue
		}
		flushCompletedAvailabilityBuckets(time.Now().Unix())
	}
}

func flushCompletedAvailabilityBuckets(now int64) {
	currentBucket := fixedAvailabilityBucketStart(now)
	availabilityHotBuckets.Range(func(rawKey, value any) bool {
		key := rawKey.(availabilityBucketKey)
		if key.bucketTs >= currentBucket {
			return true
		}

		bucket := value.(*atomicAvailabilityBucket)
		drained := bucket.drain()
		if drained.eligible == 0 {
			deleteOldEmptyAvailabilityBucket(key, rawKey, bucket, now)
			return true
		}
		if err := model.UpsertPerfMetricAvailability(&model.PerfMetricAvailability{
			ModelName:     key.model,
			Group:         key.group,
			BucketTs:      key.bucketTs,
			EligibleCount: drained.eligible,
			SuccessCount:  drained.success,
		}); err != nil {
			bucket.addCounters(drained)
			common.SysError(fmt.Sprintf("failed to flush availability metric bucket model=%s group=%s bucket=%d: %s", key.model, key.group, key.bucketTs, err.Error()))
			return true
		}
		deleteOldEmptyAvailabilityBucket(key, rawKey, bucket, now)
		return true
	})
}

func deleteOldEmptyAvailabilityBucket(key availabilityBucketKey, rawKey any, bucket *atomicAvailabilityBucket, now int64) {
	if key.bucketTs < fixedAvailabilityBucketStart(now-24*60*60) && bucket.snapshot().eligible == 0 {
		availabilityHotBuckets.Delete(rawKey)
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
			ModelName:                 k.model,
			Group:                     k.group,
			BucketTs:                  k.bucketTs,
			RequestCount:              drained.requestCount,
			SuccessCount:              drained.successCount,
			TotalLatencyMs:            drained.totalLatencyMs,
			TtftSumMs:                 drained.ttftSumMs,
			TtftCount:                 drained.ttftCount,
			OutputTokens:              drained.outputTokens,
			GenerationMs:              drained.generationMs,
			AvailabilityEligibleCount: drained.availabilityEligibleCount,
			AvailabilitySuccessCount:  drained.availabilitySuccessCount,
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
	if err := model.DeletePerfMetricAvailabilityBefore(cutoff); err != nil {
		common.SysError("failed to cleanup expired availability metrics: " + err.Error())
	}
}
