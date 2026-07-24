package service

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

const supplierDailyTaskInterval = time.Minute

var (
	supplierDailyTaskOnce    sync.Once
	supplierDailyTaskRunning atomic.Bool
)

func StartSupplierDailyAggregationTask() {
	supplierDailyTaskOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("supplier daily aggregation task started: tick=%s", supplierDailyTaskInterval))
			ticker := time.NewTicker(supplierDailyTaskInterval)
			defer ticker.Stop()
			runSupplierDailyAggregationOnce()
			for range ticker.C {
				runSupplierDailyAggregationOnce()
			}
		})
	})
}

func runSupplierDailyAggregationOnce() {
	if !supplierDailyTaskRunning.CompareAndSwap(false, true) {
		return
	}
	defer supplierDailyTaskRunning.Store(false)
	if model.DB == nil || model.LOG_DB == nil {
		return
	}
	hostname, _ := os.Hostname()
	owner := fmt.Sprintf("%s:%d", hostname, os.Getpid())
	result, err := CatchUpSupplierDailyBatches(context.Background(), model.DB, model.LOG_DB, owner, time.Now())
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("supplier daily aggregation failed: %v", err))
		return
	}
	if common.DebugEnabled && result.ProcessedDays > 0 {
		logger.LogDebug(context.Background(), "supplier daily aggregation completed: processed_days=%d, remaining=%t", result.ProcessedDays, result.RemainingWork)
	}
}
