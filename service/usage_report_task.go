package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	// trailing window warmed at startup and kept warm by the nightly job
	usageReportWindowDays = 7
)

var usageReportTaskOnce sync.Once

// StartUsageReportDailyTask keeps the usage-report tables warm so admin reads
// are pure table lookups instead of on-demand aggregation:
//   - at startup: warm the trailing window in the background (catch-up after a
//     deploy/restart), without blocking boot;
//   - every day at 00:00 UTC: fill only missing dates in the trailing window.
//
// Runs on the master node only; the distributed lock and one-slot query gate
// keep duplicate workers from stressing the source tables.
func StartUsageReportDailyTask() {
	usageReportTaskOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			common.SysLog("usage report daily task started (UTC 00:00)")
			go func() {
				if err := EnsureUsageReportRange(usageReportWindowDays); err != nil {
					common.SysError("usage_report startup backfill failed: " + err.Error())
				}
			}()
			for {
				now := time.Now().UTC()
				next := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).
					Add(24 * time.Hour)
				time.Sleep(time.Until(next))
				runUsageReportDailyJob()
			}
		})
	})
}

func runUsageReportDailyJob() {
	started := time.Now()
	if err := EnsureUsageReportRange(usageReportWindowDays); err != nil {
		common.SysError("usage_report nightly window fill failed: " + err.Error())
	}
	common.SysLog(fmt.Sprintf("usage report daily job finished in %s", time.Since(started)))
}
