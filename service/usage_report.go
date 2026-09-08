package service

import (
	"errors"
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

// usageReportSchemaV is bumped whenever the daily row gains new aggregated
// columns so existing stored rows are recomputed once (see EnsureUsageReportDate).
const usageReportSchemaV = 2

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
// most every usageReportTodayFresh seconds. Rows written by an older
// aggregation schema (SchemaV < current) are recomputed once.
func EnsureUsageReportDate(date string) error {
	usageReportMu.Lock()
	defer usageReportMu.Unlock()

	var row model.UsageReportDay
	err := model.DB.Where("date = ?", date).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return computeUsageReportDate(date)
		}
		return err
	}
	// One-time recompute after schema additions (cohort fields etc).
	if row.SchemaV < usageReportSchemaV {
		return computeUsageReportDate(date)
	}
	if date == utcToday(time.Now()) {
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
	day.SchemaV = usageReportSchemaV

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

	// 3b) Cohort funnel — people counted (1 user = 1), only completed windows
	// are meaningful; the front-end hides the most recent 7/14 days.
	//
	// reg -> key(7d): users registered this UTC day whose FIRST key was
	// created within 7 days after their registration. Subset of Registered,
	// so the cohort rate can never exceed 100%.
	var activatedC7 int
	if err := model.DB.Raw(`
		SELECT COUNT(*) FROM (
			SELECT u.id AS uid, u.created_at AS ct, MIN(t.created_time) AS ft
			FROM users u
			JOIN tokens t ON t.user_id = u.id
			WHERE u.status = ? AND u.email_verified_at > 0
			  AND u.created_at >= ? AND u.created_at < ?
			GROUP BY u.id
		) x
		WHERE x.ft >= x.ct AND x.ft <= x.ct + ?`,
		common.UserStatusEnabled, start, end, 7*24*60*60).Scan(&activatedC7).Error; err != nil {
		return nil, nil, fmt.Errorf("usage_report cohort reg->key(7d): %w", err)
	}
	day.ActivatedC7 = activatedC7

	// key -> pay(14d): users whose FIRST key fell in this UTC day and who made
	// their first successful top-up within 14 days after that first key.
	// Subset of ActivatedKey (same cohort), so the rate is <= 100%.
	var paidC14 int
	if err := model.DB.Raw(fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT t.user_id AS uid, MIN(t.created_time) AS kt
			FROM tokens t
			GROUP BY t.user_id
		) kk
		WHERE kk.kt >= ? AND kk.kt < ?
		  AND EXISTS (
			SELECT 1 FROM top_ups p
			WHERE p.user_id = kk.uid
			  AND p.status = ?
			  AND (p.money > 0 OR p.payment_amount_minor > 0)
			  AND %s >= kk.kt AND %s <= kk.kt + ?)`,
		paymentTime, paymentTime),
		start, end, common.TopUpStatusSuccess, 14*24*60*60).Scan(&paidC14).Error; err != nil {
		return nil, nil, fmt.Errorf("usage_report cohort key->pay(14d): %w", err)
	}
	day.PaidC14 = paidC14

	// key/reg -> pay(14d) on the REGISTRATION cohort (subset of Registered):
	// users registered this UTC day whose first successful top-up happened
	// within 14 days after their registration. Powers the "首次付费" funnel
	// column that must stay <= the row's Registered.
	var paidRegC14 int
	if err := model.DB.Raw(fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT u.id AS uid, u.created_at AS ct
			FROM users u
			WHERE u.status = ? AND u.email_verified_at > 0
			  AND u.created_at >= ? AND u.created_at < ?
		) uu
		WHERE EXISTS (
			SELECT 1 FROM top_ups p
			WHERE p.user_id = uu.uid
			  AND p.status = ?
			  AND (p.money > 0 OR p.payment_amount_minor > 0)
			  AND %s >= uu.ct AND %s <= uu.ct + ?)`,
		paymentTime, paymentTime),
		common.UserStatusEnabled, start, end, common.TopUpStatusSuccess,
		14*24*60*60).Scan(&paidRegC14).Error; err != nil {
		return nil, nil, fmt.Errorf("usage_report cohort reg->pay(14d): %w", err)
	}
	day.PaidRegC14 = paidRegC14

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
