package service

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

// UsageReport computes the user-side daily usage report board. It is a NEW,
// independent reporting module: it never modifies nor replaces the existing
// ops daily report.
//
// Design (see docs/usage-report.md):
//   - One compute per UTC+0 day, stored into usage_report_daily(_model).
//   - Past dates are computed once, lazily, on first read (idempotent).
//   - The current UTC day is recomputed at most every usageReportTodayFresh
//     seconds from the (indexed) day slice of the log/users tables, so admin
//     reads never scan history. A later phase will move the "today" slice to
//     Redis minute-level counters (documented, not yet wired to the hot path).

const usageReportTodayFresh = 5 * time.Minute

var usageReportMu sync.Mutex

// utcDateBounds converts "2006-01-02" (UTC+0) to [start, end) unix seconds.
func utcDateBounds(date string) (int64, int64, error) {
	start, err := time.ParseInLocation("2006-01-02", date, time.UTC)
	if err != nil {
		return 0, 0, err
	}
	s := start.Unix()
	return s, s + 24*60*60, nil
}

// utcToday returns the UTC+0 calendar date string of "now".
func utcToday(now time.Time) string {
	return now.UTC().Format("2006-01-02")
}

// EnsureUsageReportRange ensures every UTC day in the trailing window
// [today-(days-1) .. today] has fresh daily rows. days is capped by the caller.
func EnsureUsageReportRange(days int) error {
	now := time.Now()
	today := utcToday(now)
	for i := days - 1; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		if err := EnsureUsageReportDate(utcToday(d)); err != nil {
			return err
		}
	}
	_ = today
	return nil
}

// EnsureUsageReportDate guarantees the row for one UTC date exists and is
// fresh enough for today. Past dates are computed once; today is refreshed at
// most every usageReportTodayFresh seconds.
func EnsureUsageReportDate(date string) error {
	usageReportMu.Lock()
	defer usageReportMu.Unlock()

	exists, err := model.GetUsageReportDayExists(date)
	if err != nil {
		return err
	}
	if !exists {
		return computeUsageReportDate(date)
	}
	if date == utcToday(time.Now()) {
		var row model.UsageReportDay
		if err := model.DB.Where("date = ?", date).First(&row).Error; err != nil {
			return err
		}
		if time.Since(time.Unix(row.BuiltAt, 0)) >= usageReportTodayFresh {
			return computeUsageReportDate(date)
		}
	}
	return nil
}

// computeUsageReportDate aggregates one UTC day from the source tables and
// stores it (idempotent delete + insert).
func computeUsageReportDate(date string) error {
	start, end, err := utcDateBounds(date)
	if err != nil {
		return err
	}

	day, modelStats, err := aggregateUsageReportDate(date, start, end)
	if err != nil {
		return err
	}
	day.BuiltAt = common.GetTimestamp()

	return model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("date = ?", date).Delete(&model.UsageReportDay{}).Error; err != nil {
			return err
		}
		if err := tx.Create(day).Error; err != nil {
			return err
		}
		if err := tx.Where("date = ?", date).Delete(&model.UsageReportDayModel{}).Error; err != nil {
			return err
		}
		if len(modelStats) == 0 {
			return nil
		}
		return tx.Create(modelStats).Error
	})
}

type usageModelRow struct {
	ModelName        string `gorm:"column:model_name"`
	Calls            int64  `gorm:"column:calls"`
	PromptTokens     int64  `gorm:"column:prompt_tokens"`
	CompletionTokens int64  `gorm:"column:completion_tokens"`
}

// aggregateUsageReportDate reads one UTC day of facts from the source tables.
// It never scans history: every query is bounded to [start, end).
func aggregateUsageReportDate(date string, start, end int64) (*model.UsageReportDay, []*model.UsageReportDayModel, error) {
	day := &model.UsageReportDay{Date: date}

	// 1) Registrations: created that UTC day, enabled + email verified.
	var registered int64
	if err := model.DB.Model(&model.User{}).
		Where("status = ?", common.UserStatusEnabled).
		Where("email_verified_at > 0").
		Where("created_at >= ? AND created_at < ?", start, end).
		Count(&registered).Error; err != nil {
		return nil, nil, fmt.Errorf("usage_report registrations: %w", err)
	}
	day.Registered = int(registered)

	// 2) Activation: users whose FIRST API token was created that UTC day.
	if err := model.DB.Raw(`
		SELECT COUNT(*) FROM (
			SELECT user_id, MIN(created_time) AS ft
			FROM tokens
			WHERE created_time < ?
			GROUP BY user_id
		) AS t
		WHERE t.ft >= ?`, end, start).Scan(&day.ActivatedKey).Error; err != nil {
		return nil, nil, fmt.Errorf("usage_report activated keys: %w", err)
	}

	// 3) First paid + 4) paid amount: successful top_ups, bucketed by the
	// completion time (falling back to creation time when never completed).
	paymentTime := "COALESCE(NULLIF(complete_time, 0), create_time)"
	if err := model.DB.Raw(fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT user_id, MIN(%s) AS ft
			FROM top_ups
			WHERE status = ? AND (money > 0 OR payment_amount_minor > 0)
			GROUP BY user_id
		) AS t
		WHERE t.ft >= ? AND t.ft < ?`, paymentTime),
		common.TopUpStatusSuccess, start, end).Scan(&day.FirstPaid).Error; err != nil {
		return nil, nil, fmt.Errorf("usage_report first paid: %w", err)
	}

	var paidUSD float64
	if err := model.DB.Raw(fmt.Sprintf(`
		SELECT COALESCE(SUM(money), 0)
		FROM top_ups
		WHERE status = ? AND (money > 0 OR payment_amount_minor > 0)
		  AND %s >= ? AND %s < ?`, paymentTime, paymentTime),
		common.TopUpStatusSuccess, start, end).Scan(&paidUSD).Error; err != nil {
		return nil, nil, fmt.Errorf("usage_report paid usd: %w", err)
	}
	day.PaidUSD = math.Round(paidUSD*100) / 100

	// 5) Usage: consumption log rows of that day (Log.Type = consume),
	// grouped by model. All queries are range-bounded by the log table's
	// created_at index.
	rows, err := aggregateUsageLogs(start, end)
	if err != nil {
		return nil, nil, err
	}
	modelStats := make([]*model.UsageReportDayModel, 0, len(rows))
	for _, r := range rows {
		if r.ModelName == "" {
			continue
		}
		day.Calls += r.Calls
		day.PromptTokens += r.PromptTokens
		day.CompletionTokens += r.CompletionTokens
		modelStats = append(modelStats, &model.UsageReportDayModel{
			Date:             date,
			ModelName:        r.ModelName,
			Calls:            r.Calls,
			PromptTokens:     r.PromptTokens,
			CompletionTokens: r.CompletionTokens,
		})
	}
	return day, modelStats, nil
}

func aggregateUsageLogs(start, end int64) ([]usageModelRow, error) {
	var rows []usageModelRow
	err := model.LOG_DB.Raw(`
		SELECT model_name, COUNT(*) AS calls,
		       COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
		       COALESCE(SUM(completion_tokens), 0) AS completion_tokens
		FROM logs
		WHERE type = ? AND created_at >= ? AND created_at < ?
		GROUP BY model_name`,
		model.LogTypeConsume, start, end).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("usage_report usage logs: %w", err)
	}
	return rows, nil
}
