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
	usageReportWindowDays = 30
	// days force-recomputed at 00:00 UTC (yesterday finalises; late-arriving
	// key/payment events from the previous days are picked up)
	usageReportNightlyRecomputeDays = 3
)

var usageReportTaskOnce sync.Once

// StartUsageReportDailyTask keeps the usage-report tables warm so admin reads
// are pure table lookups instead of on-demand aggregation:
//   - at startup: warm the trailing window in the background (catch-up after a
//     deploy/restart), without blocking boot;
//   - every day at 00:00 UTC: force-recompute the last few days (so yesterday
//     is finalised and late events land) and then self-heal any missing day in
//     the trailing window.
//
// Runs on the master node only; each recompute is an idempotent delete+insert,
// so a duplicate run from another node or a retry is harmless.
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
	now := time.Now().UTC()
	for i := 1; i <= usageReportNightlyRecomputeDays; i++ {
		date := utcToday(now.AddDate(0, 0, -i))
		if err := RecomputeUsageReportDateAllGroups(date); err != nil {
			common.SysError("usage_report nightly recompute failed for " + date + ": " + err.Error())
		}
	}
	if err := EnsureUsageReportRange(usageReportWindowDays); err != nil {
		common.SysError("usage_report nightly window fill failed: " + err.Error())
	}
	common.SysLog(fmt.Sprintf("usage report daily job finished in %s", time.Since(started)))
}
