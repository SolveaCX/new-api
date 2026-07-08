package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	logRequestSampleCleanupTickInterval = 1 * time.Hour
	logRequestSampleCleanupBatchSize    = 1000
	logRequestSampleCleanupMaxBatches   = 0
	logRequestSampleCleanupTimeout      = 30 * time.Second
)

var logRequestSampleCleanupOnce sync.Once

func StartLogRequestSampleCleanupTask() {
	logRequestSampleCleanupOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("log request sample cleanup task started: tick=%s", logRequestSampleCleanupTickInterval))
			ticker := time.NewTicker(logRequestSampleCleanupTickInterval)
			defer ticker.Stop()

			runLogRequestSampleCleanupOnce()
			for range ticker.C {
				runLogRequestSampleCleanupOnce()
			}
		})
	})
}

func runLogRequestSampleCleanupOnce() {
	snapshot := operation_setting.GetLogRequestSamplingSnapshot()
	if snapshot.RetentionDays <= 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(snapshot.RetentionDays) * 24 * time.Hour).Unix()
	ctx, cancel := context.WithTimeout(context.Background(), logRequestSampleCleanupTimeout)
	defer cancel()
	deleted, err := model.CleanupOldLogRequestSamplesWithContext(ctx, cutoff, logRequestSampleCleanupBatchSize, logRequestSampleCleanupMaxBatches)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("log request sample cleanup failed: %v", err))
		return
	}
	if common.DebugEnabled && deleted > 0 {
		logger.LogDebug(context.Background(), "log request sample cleanup: deleted=%d", deleted)
	}
}
