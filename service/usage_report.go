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
const usageReportSchemaV = 4

// usageReportDateLocks serialize per-date ensure/compute so a slow recompute
// of one day never blocks other dates (and admin reads are not globally
// serialized). Cross-node idempotency still relies on the delete+insert
// transaction being safe under concurrent writers.
var usageReportLocksMu sync.Mutex
var usageReportLocks = make(map[string]*sync.Mutex)

func usageReportDateLock(date string) *sync.Mutex {
	usageReportLocksMu.Lock()
	defer usageReportLocksMu.Unlock()
	if m, ok := usageReportLocks[date]; ok {
		return m
	}
	m := &sync.Mutex{}
	usageReportLocks[date] = m
	return m
}

// usageReportBackfillNullColumns fills NULL legacy rows that appear when
// AutoMigrate adds columns without a default; Go int scanning would otherwise
// fail on NULL. Runs per date right before the row is read.
func usageReportBackfillNullColumns(date string) error {
	return model.DB.Exec(`
		UPDATE usage_report_daily SET
			registered = COALESCE(registered, 0),
			activated_key = COALESCE(activated_key, 0),
			first_paid = COALESCE(first_paid, 0),
			paid_usd = COALESCE(paid_usd, 0),
			activated_day = COALESCE(activated_day, 0),
			paid_day = COALESCE(paid_day, 0),
			activated_c7 = COALESCE(activated_c7, 0),
			paid_c14 = COALESCE(paid_c14, 0),
			paid_reg_c14 = COALESCE(paid_reg_c14, 0),
			calls = COALESCE(calls, 0),
			prompt_tokens = COALESCE(prompt_tokens, 0),
			completion_tokens = COALESCE(completion_tokens, 0),
			built_at = COALESCE(built_at, 0),
			schema_v = COALESCE(schema_v, 0)
		WHERE date = ?`, date).Error
}

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
// All date math is done in UTC so a non-UTC server clock cannot shift the
// trailing window by a day.
func EnsureUsageReportRange(days int) error {
	now := time.Now().UTC()
	for i := days - 1; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		if err := EnsureUsageReportDate(utcToday(d)); err != nil {
			return err
		}
	}
	return nil
}

// EnsureUsageReportDate guarantees the row for one UTC date exists and is
// fresh enough for today. Past dates are computed once; today is refreshed at
// most every usageReportTodayFresh seconds. Rows written by an older
// aggregation schema (SchemaV < current) are recomputed once.
func EnsureUsageReportDate(date string) error {
	lock := usageReportDateLock(date)
	lock.Lock()
	defer lock.Unlock()

	if err := usageReportBackfillNullColumns(date); err != nil {
		return err
	}

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
	if date == utcToday(time.Now().UTC()) {
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

	// 3a) 当天口径（主口径，C 端快进快出）：该日注册的人中，注册当天即
	// 首次建 Key / 首次付费的人数。天然 ⊆ Registered（同一天注册队列）。
	actDay, payDay, sameDayErr := aggregateSameDay(start, end, paymentTime)
	if sameDayErr != nil {
		return nil, nil, sameDayErr
	}
	day.ActivatedDay = actDay
	day.PaidDay = payDay

	// 3b) 长窗辅助口径（保留，仅分析用；主界面不再用 7/14 日窗口）：
	// reg -> key(7d)：该日注册者注册后 7 日内首次建 Key 人数。
	var activatedC7 int
	if err := model.DB.Raw(`
		SELECT COUNT(*) FROM (
			SELECT u.id AS uid, u.created_at AS ct, MIN(t.created_time) AS ft
			FROM users u
			JOIN tokens t ON t.user_id = u.id
			WHERE u.status = ? AND u.email_verified_at > 0 AND u.deleted_at IS NULL
			  AND u.created_at >= ? AND u.created_at < ?
			GROUP BY u.id
		) x
		WHERE x.ft >= x.ct AND x.ft <= x.ct + ?`,
		common.UserStatusEnabled, start, end, 7*24*60*60).Scan(&activatedC7).Error; err != nil {
		return nil, nil, fmt.Errorf("usage_report cohort reg->key(7d): %w", err)
	}
	day.ActivatedC7 = activatedC7

	// key -> pay(14d): users whose FIRST key fell in this UTC day and whose
	// FIRST successful top-up happened within 14 days after that first key
	// (first-payment time = MIN over successful top-ups, so repeat top-ups
	// cannot inflate the cohort).
	// Subset of ActivatedKey (same cohort), so the rate is <= 100%.
	var paidC14 int
	if err := model.DB.Raw(fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT t.user_id AS uid, MIN(t.created_time) AS kt
			FROM tokens t
			GROUP BY t.user_id
		) kk
		JOIN (
			SELECT p.user_id AS puid, MIN(%s) AS pt
			FROM top_ups p
			WHERE p.status = ? AND (p.money > 0 OR p.payment_amount_minor > 0)
			GROUP BY p.user_id
		) fp ON fp.puid = kk.uid
		WHERE kk.kt >= ? AND kk.kt < ?
		  AND fp.pt >= kk.kt AND fp.pt <= kk.kt + ?`, paymentTime),
		common.TopUpStatusSuccess, start, end, 14*24*60*60).Scan(&paidC14).Error; err != nil {
		return nil, nil, fmt.Errorf("usage_report cohort key->pay(14d): %w", err)
	}
	day.PaidC14 = paidC14

	// key/reg -> pay(14d) on the REGISTRATION cohort (subset of Registered):
	// users registered this UTC day whose FIRST successful top-up happened
	// within 14 days after their registration.
	var paidRegC14 int
	if err := model.DB.Raw(fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT u.id AS uid, u.created_at AS ct
			FROM users u
			WHERE u.status = ? AND u.email_verified_at > 0 AND u.deleted_at IS NULL
			  AND u.created_at >= ? AND u.created_at < ?
		) uu
		JOIN (
			SELECT p.user_id AS puid, MIN(%s) AS pt
			FROM top_ups p
			WHERE p.status = ? AND (p.money > 0 OR p.payment_amount_minor > 0)
			GROUP BY p.user_id
		) fp ON fp.puid = uu.uid
		WHERE fp.pt >= uu.ct AND fp.pt <= uu.ct + ?`, paymentTime),
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

// aggregateSameDay counts, among users registered in [start, end):
//   - activated:  people who created their first key the SAME UTC day they
//     registered (first token time >= registration time and < end).
//   - paid:       people whose first successful top-up completed the SAME UTC
//     day they registered (settlement time >= registration time and < end).
//
// Both numbers are subsets of that day's registrations (people counted), which
// keeps the same-day funnel columns monotonically non-increasing.
func aggregateSameDay(start, end int64, paymentTime string) (int, int, error) {
	var activated int
	if err := model.DB.Raw(`
		SELECT COUNT(*) FROM (
			SELECT u.id AS uid, u.created_at AS ct
			FROM users u
			WHERE u.status = ? AND u.email_verified_at > 0 AND u.deleted_at IS NULL
			  AND u.created_at >= ? AND u.created_at < ?
		) uu
		WHERE EXISTS (
			SELECT 1 FROM tokens t
			WHERE t.user_id = uu.uid
			  AND t.created_time >= uu.ct AND t.created_time < ?)`,
		common.UserStatusEnabled, start, end, end).Scan(&activated).Error; err != nil {
		return 0, 0, fmt.Errorf("usage_report same-day activated: %w", err)
	}

	var paid int
	if err := model.DB.Raw(fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT u.id AS uid, u.created_at AS ct
			FROM users u
			WHERE u.status = ? AND u.email_verified_at > 0 AND u.deleted_at IS NULL
			  AND u.created_at >= ? AND u.created_at < ?
		) uu
		JOIN (
			SELECT p.user_id AS puid, MIN(%s) AS pt
			FROM top_ups p
			WHERE p.status = ? AND (p.money > 0 OR p.payment_amount_minor > 0)
			GROUP BY p.user_id
		) fp ON fp.puid = uu.uid
		WHERE fp.pt >= uu.ct AND fp.pt < ?`, paymentTime),
		common.UserStatusEnabled, start, end, common.TopUpStatusSuccess, end).Scan(&paid).Error; err != nil {
		return 0, 0, fmt.Errorf("usage_report same-day paid: %w", err)
	}
	return activated, paid, nil
}
