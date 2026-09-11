package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
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
//   - One compute per (UTC+0 day, group), stored into
//     usage_report_daily_v2 / usage_report_daily_model_v2.
//   - Past dates are filled by bounded offline tasks and kept as immutable snapshots.
//   - Existing snapshots are immutable; only an explicit manual backfill may
//     force a replacement. Admin reads never scan history.
//     Redis minute-level counters (documented, not yet wired to the hot path).

const (
	usageReportBackfillMaxDays    = 7
	usageReportDistributedLockKey = "new-api:usage-report:fill-lock"
	usageReportDistributedLockTTL = 30 * time.Minute
)

func usageReportBackfillDays(days int) int {
	if days <= 0 {
		return 1
	}
	if days > usageReportBackfillMaxDays {
		return usageReportBackfillMaxDays
	}
	return days
}

var (
	ErrUsageReportFillInProgress  = errors.New("usage report fill already running")
	ErrUsageReportLockUnavailable = errors.New("usage report distributed lock unavailable")
	ErrUsageReportLockLost        = errors.New("usage report distributed lock lost")
)

type usageReportDistributedLock struct{ token string }

func acquireUsageReportDistributedLock(ctx context.Context) (*usageReportDistributedLock, bool, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return nil, false, ErrUsageReportLockUnavailable
	}
	token := common.GetUUID()
	ok, err := common.RDB.SetNX(ctx, usageReportDistributedLockKey, token, usageReportDistributedLockTTL).Result()
	if err != nil {
		return nil, false, fmt.Errorf("acquire usage report distributed lock: %w", err)
	}
	if !ok {
		return nil, false, nil
	}
	return &usageReportDistributedLock{token: token}, true, nil
}

func (l *usageReportDistributedLock) Release() {
	if l == nil || !common.RedisEnabled || common.RDB == nil || l.token == "" {
		return
	}
	const script = `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`
	_, _ = common.RDB.Eval(context.Background(), script, []string{usageReportDistributedLockKey}, l.token).Result()
}

func (l *usageReportDistributedLock) Renew(ctx context.Context) <-chan struct{} {
	lost := make(chan struct{})
	if l == nil || l.token == "" || !common.RedisEnabled || common.RDB == nil {
		close(lost)
		return lost
	}
	go func() {
		defer close(lost)
		ticker := time.NewTicker(usageReportDistributedLockTTL / 3)
		defer ticker.Stop()
		const script = `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("PEXPIRE", KEYS[1], ARGV[2]) else return 0 end`
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				result, err := common.RDB.Eval(ctx, script, []string{usageReportDistributedLockKey}, l.token, usageReportDistributedLockTTL.Milliseconds()).Int()
				if err != nil || result != 1 {
					return
				}
			}
		}
	}()
	return lost
}

var usageReportDaySemaphore = make(chan struct{}, 1)

func withUsageReportDaySlot(fn func() error) error {
	usageReportDaySemaphore <- struct{}{}
	defer func() { <-usageReportDaySemaphore }()
	return fn()
}

// usageReportSchemaV is bumped whenever the daily row gains new aggregated
// columns so existing stored rows are recomputed once (see EnsureUsageReportDate).
const (
	usageReportSchemaV = 5

	// UsageReportGroupPLG / UsageReportGroupAll are the stored group
	// dimensions: "plg" = only users whose users.group = 'plg';
	// "all" = no group filter.
	UsageReportGroupPLG = "plg"
	UsageReportGroupAll = "all"
)

var usageReportGroups = []string{UsageReportGroupPLG, UsageReportGroupAll}

// usageReportGroupColumn returns the dialect-quoted `group` column name for
// raw SQL (logs.group / users.group are reserved words in MySQL/PostgreSQL).
func usageReportGroupColumn() string {
	if common.UsingPostgreSQL {
		return `"group"`
	}
	return "`group`"
}

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

// usageReportNullsMu guards a table-wide NULL backfill that only marks itself
// done after a successful run, so a transient failure is retried on the next
// call instead of being cached forever (see review).
// AutoMigrate can add columns without defaults, leaving historical rows NULL
// which would break Go int scanning; we fill them once (only rows that contain
// NULL are touched), so the report read path never becomes a per-request write.
var usageReportNullsMu sync.Mutex
var usageReportNullsDone bool

func usageReportEnsureColumnDefaults() error {
	usageReportNullsMu.Lock()
	defer usageReportNullsMu.Unlock()
	if usageReportNullsDone {
		return nil
	}
	// Table name is taken from the model so it always follows TableName()
	// (currently usage_report_daily_v2) and can never drift to a legacy table.
	table := model.UsageReportDay{}.TableName()
	if err := model.DB.Exec(fmt.Sprintf(`
		UPDATE %s SET
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
		WHERE registered IS NULL OR activated_key IS NULL OR first_paid IS NULL
		   OR paid_usd IS NULL OR activated_day IS NULL OR paid_day IS NULL
		   OR activated_c7 IS NULL OR paid_c14 IS NULL OR paid_reg_c14 IS NULL
		   OR calls IS NULL OR prompt_tokens IS NULL OR completion_tokens IS NULL
		   OR built_at IS NULL OR schema_v IS NULL`, table)).Error; err != nil {
		return err // transient failure: do not mark done, next call retries
	}
	usageReportNullsDone = true
	return nil
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

// EnsureUsageReportRange fills only missing dates in the bounded trailing
// window [today-(days-1) .. today]. Existing snapshots are never refreshed.
//
// Two deliberate properties (learned from production: rows stopped at an old
// date while newer days stayed empty):
//   - newest first: today/yesterday land first, so a long or interrupted fill
//     still leaves the most relevant days visible;
//   - one failing day never aborts the rest: errors are logged and skipped, and
//     the first error is returned for the caller (CSV) to surface.
func EnsureUsageReportRange(days int) error {
	days = usageReportBackfillDays(days)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	distributedLock, acquired, err := acquireUsageReportDistributedLock(ctx)
	if err != nil {
		return err
	}
	if !acquired {
		return ErrUsageReportFillInProgress
	}
	defer distributedLock.Release()
	leaseCtx, leaseCancel := context.WithCancel(context.Background())
	defer leaseCancel()
	leaseLost := distributedLock.Renew(leaseCtx)
	if err := usageReportEnsureColumnDefaults(); err != nil {
		return err
	}
	now := time.Now().UTC()
	var firstErr error
	for i := 0; i < days; i++ {
		select {
		case <-leaseLost:
			return ErrUsageReportLockLost
		default:
		}
		date := utcToday(now.AddDate(0, 0, -i))
		for _, group := range usageReportGroups {
			select {
			case <-leaseLost:
				return ErrUsageReportLockLost
			default:
			}
			started := time.Now()
			if err := EnsureUsageReportDate(date, group); err != nil {
				common.SysError(fmt.Sprintf("usage_report fill failed: date=%s group=%s: %s", date, group, err.Error()))
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if cost := time.Since(started); cost > 5*time.Second {
				common.SysLog(fmt.Sprintf("usage_report fill slow: date=%s group=%s cost=%s", date, group, cost))
			}
		}
	}
	return firstErr
}

// usageReportFillGuard prevents duplicate background fills across admin reads.
var (
	usageReportFillMu  sync.Mutex
	usageReportFilling bool
)

// usageReportPair is one (date, group) aggregation unit.
type usageReportPair struct {
	Date  string
	Group string
}

// FillUsageReportMissing computes up to `batch` still-missing day
// rows, newest-first, and reports how many units remain.
//
// Each call is bounded so an explicit admin backfill stays well under the
// request timeout.
func FillUsageReportMissing(days int, batch int) ([]string, int, error) {
	days = usageReportBackfillDays(days)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	distributedLock, acquired, err := acquireUsageReportDistributedLock(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !acquired {
		return nil, days * len(usageReportGroups), ErrUsageReportFillInProgress
	}
	defer distributedLock.Release()
	leaseCtx, leaseCancel := context.WithCancel(context.Background())
	defer leaseCancel()
	leaseLost := distributedLock.Renew(leaseCtx)
	if err := usageReportEnsureColumnDefaults(); err != nil {
		return nil, 0, err
	}
	if batch <= 0 {
		batch = 2
	}
	now := time.Now().UTC()
	today := utcToday(now)
	from := utcToday(now.AddDate(0, 0, -(days - 1)))

	existing := map[string]struct{}{}
	var rows []model.UsageReportDay
	if err := model.DB.
		Select(fmt.Sprintf("date, %s", model.UsageReportGroupColumn())).
		Where("date >= ? AND date <= ?", from, today).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	for _, r := range rows {
		existing[r.Date+"|"+r.Group] = struct{}{}
	}

	needed := make([]usageReportPair, 0, days*len(usageReportGroups))
	for i := 0; i < days; i++ {
		date := utcToday(now.AddDate(0, 0, -i))
		for _, group := range usageReportGroups {
			if _, ok := existing[date+"|"+group]; !ok {
				needed = append(needed, usageReportPair{Date: date, Group: group})
			}
		}
	}

	filled := make([]string, 0, batch)
	var firstErr error
	for _, pair := range needed {
		if len(filled) >= batch {
			break
		}
		select {
		case <-leaseLost:
			return filled, len(needed) - len(filled), ErrUsageReportLockLost
		default:
		}
		if err := EnsureUsageReportDate(pair.Date, pair.Group); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		filled = append(filled, pair.Date+"|"+pair.Group)
	}
	remaining := len(needed) - len(filled)
	if remaining < 0 {
		remaining = 0
	}
	return filled, remaining, firstErr
}

// EnsureUsageReportDateAllGroups fills one date in group order while holding
// the distributed lease for the whole date. Existing rows are skipped by
// EnsureUsageReportDate, so a retry cannot re-run a completed group.
func EnsureUsageReportDateAllGroups(date string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	distributedLock, acquired, err := acquireUsageReportDistributedLock(ctx)
	if err != nil {
		return err
	}
	if !acquired {
		return ErrUsageReportFillInProgress
	}
	defer distributedLock.Release()

	leaseCtx, leaseCancel := context.WithCancel(context.Background())
	defer leaseCancel()
	leaseLost := distributedLock.Renew(leaseCtx)
	for _, group := range usageReportGroups {
		select {
		case <-leaseLost:
			return ErrUsageReportLockLost
		default:
		}
		if err := EnsureUsageReportDate(date, group); err != nil {
			return err
		}
	}
	return nil
}

// UsageReportFillRunning reports whether a background window fill is currently
// in progress (the API uses it instead of inferring from row counts).
func UsageReportFillRunning() bool {
	usageReportFillMu.Lock()
	defer usageReportFillMu.Unlock()
	return usageReportFilling
}

// EnsureUsageReportRangeAsync warms the trailing window in the background and
// returns immediately. The report read path therefore never blocks on a full
// historical backfill; each date becomes visible as soon as it is persisted,
// and the API reports a "filling" flag until the whole window is ready.
func EnsureUsageReportRangeAsync(days int) {
	usageReportFillMu.Lock()
	if usageReportFilling {
		usageReportFillMu.Unlock()
		return
	}
	usageReportFilling = true
	usageReportFillMu.Unlock()
	go func() {
		defer func() {
			usageReportFillMu.Lock()
			usageReportFilling = false
			usageReportFillMu.Unlock()
		}()
		if err := EnsureUsageReportRange(days); err != nil {
			common.SysError("usage_report background fill failed: " + err.Error())
		}
	}()
}

// EnsureUsageReportDate guarantees the row for one UTC date exists. Existing
// rows are immutable; use RecomputeUsageReportDate only for an explicit manual
// repair.
func EnsureUsageReportDate(date string, group string) error {
	lock := usageReportDateLock(date + "|" + group)
	lock.Lock()
	defer lock.Unlock()
	return withUsageReportDaySlot(func() error {
		return ensureUsageReportDateLocked(date, group)
	})
}

func ensureUsageReportDateLocked(date string, group string) error {
	// Ensure legacy NULL rows are backfilled before reading, so a single-date
	// request can self-heal even if the range-level call was never hit.
	if err := usageReportEnsureColumnDefaults(); err != nil {
		return err
	}

	var row model.UsageReportDay
	err := model.DB.Where("date = ? AND `group` = ?", date, group).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return computeUsageReportDate(date, group)
		}
		return err
	}
	return nil
}

// RecomputeUsageReportDate force-recomputes one UTC day regardless of its
// stored state. Used by the nightly task so yesterday is finalised and
// late-arriving key/payment events are folded in.
func RecomputeUsageReportDate(date string, group string) error {
	if err := usageReportEnsureColumnDefaults(); err != nil {
		return err
	}
	lock := usageReportDateLock(date + "|" + group)
	lock.Lock()
	defer lock.Unlock()
	return withUsageReportDaySlot(func() error {
		return computeUsageReportDate(date, group)
	})
}

// RecomputeUsageReportDateAllGroups recomputes every stored group for a date
// (used by the nightly task and the manual backfill endpoint).
func RecomputeUsageReportDateAllGroups(date string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	distributedLock, acquired, err := acquireUsageReportDistributedLock(ctx)
	if err != nil {
		return err
	}
	if !acquired {
		return ErrUsageReportFillInProgress
	}
	defer distributedLock.Release()
	return recomputeUsageReportDateAllGroups(date)
}

func recomputeUsageReportDateAllGroups(date string) error {
	var firstErr error
	for _, group := range usageReportGroups {
		if err := RecomputeUsageReportDate(date, group); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			common.SysError(fmt.Sprintf("usage_report recompute failed: date=%s group=%s: %s", date, group, err.Error()))
		}
	}
	return firstErr
}

// computeUsageReportDate aggregates one UTC day from the source tables and
// stores it (idempotent delete + insert).
func computeUsageReportDate(date string, group string) error {
	start, end, err := utcDateBounds(date)
	if err != nil {
		return err
	}

	day, modelStats, err := aggregateUsageReportDate(date, group, start, end)
	if err != nil {
		return err
	}
	day.BuiltAt = common.GetTimestamp()
	day.SchemaV = usageReportSchemaV

	return model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("date = ? AND `group` = ?", date, group).Delete(&model.UsageReportDay{}).Error; err != nil {
			return err
		}
		if err := tx.Create(day).Error; err != nil {
			return err
		}
		if err := tx.Where("date = ? AND `group` = ?", date, group).Delete(&model.UsageReportDayModel{}).Error; err != nil {
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
func aggregateUsageReportDate(date string, group string, start, end int64) (*model.UsageReportDay, []*model.UsageReportDayModel, error) {
	day := &model.UsageReportDay{Date: date, Group: group}
	groupFilter := ""
	if group != UsageReportGroupAll {
		groupFilter = fmt.Sprintf(" AND %s = ?", usageReportGroupColumn())
	}

	// 1) Registrations: created that UTC day, enabled + email verified,
	//    restricted to the requested group (default: PLG only).
	regQuery := model.DB.Model(&model.User{}).
		Where("status = ?", common.UserStatusEnabled).
		Where("email_verified_at > 0").
		Where("created_at >= ? AND created_at < ?", start, end)
	if group != UsageReportGroupAll {
		regQuery = regQuery.Where("`group` = ?", group)
	}
	var registered int64
	if err := regQuery.Count(&registered).Error; err != nil {
		return nil, nil, fmt.Errorf("usage_report registrations: %w", err)
	}
	day.Registered = int(registered)

	// 2) 金额：当日成功 top_up 的实收合计（日历日口径；事件/长窗辅助口径
	// 已从主计算中移除——主界面只展示当天口径，避免每日重算时对
	// tokens/top_ups 做全历史 MIN 分组）。
	paymentTime := "COALESCE(NULLIF(complete_time, 0), create_time)"

	var paidUSD float64
	if group == UsageReportGroupAll {
		if err := model.DB.Raw(fmt.Sprintf(`
			SELECT COALESCE(SUM(money), 0)
			FROM top_ups
			WHERE status = ? AND (money > 0 OR payment_amount_minor > 0)
			  AND %s >= ? AND %s < ?`, paymentTime, paymentTime),
			common.TopUpStatusSuccess, start, end).Scan(&paidUSD).Error; err != nil {
			return nil, nil, fmt.Errorf("usage_report paid usd: %w", err)
		}
	} else {
		if err := model.DB.Raw(fmt.Sprintf(`
			SELECT COALESCE(SUM(p.money), 0)
			FROM top_ups p
			JOIN users u ON u.id = p.user_id AND u.deleted_at IS NULL AND %s = ?
			WHERE p.status = ? AND (p.money > 0 OR p.payment_amount_minor > 0)
			  AND %s >= ? AND %s < ?`, usageReportGroupColumn(), paymentTime, paymentTime),
			group, common.TopUpStatusSuccess, start, end).Scan(&paidUSD).Error; err != nil {
			return nil, nil, fmt.Errorf("usage_report paid usd: %w", err)
		}
	}
	day.PaidUSD = math.Round(paidUSD*100) / 100

	// 3) 当天口径（主口径，C 端快进快出）：该日注册的人中，注册当天即
	// 首次建 Key / 首次付费的人数。天然 ⊆ Registered（同一天注册队列）。
	actDay, payDay, sameDayErr := aggregateSameDay(start, end, paymentTime, groupFilter, group)
	if sameDayErr != nil {
		return nil, nil, sameDayErr
	}
	day.ActivatedDay = actDay
	day.PaidDay = payDay

	// (事件/长窗辅助字段 activated_key / first_paid / activated_c7 /
	//  paid_c14 / paid_reg_c14 已从主计算移除：主界面只展示当天口径，
	//  避免每日重算对 tokens / top_ups 做全历史 MIN 分组。)

	// 5) Usage: consumption log rows of that day (Log.Type = consume),
	// grouped by model. All queries are range-bounded by the log table's
	// created_at index.
	rows, err := aggregateUsageLogs(start, end, groupFilter, group)
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
			Group:            group,
			ModelName:        r.ModelName,
			Calls:            r.Calls,
			PromptTokens:     r.PromptTokens,
			CompletionTokens: r.CompletionTokens,
		})
	}
	return day, modelStats, nil
}

func aggregateUsageLogs(start, end int64, groupFilter string, group string) ([]usageModelRow, error) {
	var rows []usageModelRow
	sql := `
		SELECT model_name, COUNT(*) AS calls,
		       COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
		       COALESCE(SUM(completion_tokens), 0) AS completion_tokens
		FROM logs
		WHERE type = ? AND created_at >= ? AND created_at < ?` + strings.ReplaceAll(groupFilter, "u.", "") + `
		GROUP BY model_name`
	args := []interface{}{model.LogTypeConsume, start, end}
	if group != UsageReportGroupAll {
		args = append(args, group)
	}
	err := model.LOG_DB.Raw(sql, args...).Scan(&rows).Error
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
func aggregateSameDay(start, end int64, paymentTime string, groupFilter string, group string) (int, int, error) {
	var activated int
	sql := `
		SELECT COUNT(*) FROM (
			SELECT u.id AS uid, u.created_at AS ct
			FROM users u
			WHERE u.status = ? AND u.email_verified_at > 0 AND u.deleted_at IS NULL` + groupFilter + `
			  AND u.created_at >= ? AND u.created_at < ?
		) uu
		WHERE EXISTS (
			SELECT 1 FROM tokens t
			WHERE t.user_id = uu.uid
			  AND t.created_time >= uu.ct AND t.created_time < ?)`
	args := []interface{}{common.UserStatusEnabled}
	if group != UsageReportGroupAll {
		args = append(args, group)
	}
	args = append(args, start, end, end)
	if err := model.DB.Raw(sql, args...).Scan(&activated).Error; err != nil {
		return 0, 0, fmt.Errorf("usage_report same-day activated: %w", err)
	}

	// paid_day：该日注册的人中，注册当天完成首笔成功付费（EXISTS 当天有
	// 成功付费 + NOT EXISTS 注册前已有付费，限定首笔；走 user_id 索引）。
	var paid int
	sql = fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT u.id AS uid, u.created_at AS ct
			FROM users u
			WHERE u.status = ? AND u.email_verified_at > 0 AND u.deleted_at IS NULL`+groupFilter+`
			  AND u.created_at >= ? AND u.created_at < ?
		) uu
		WHERE EXISTS (
			SELECT 1 FROM top_ups p
			WHERE p.user_id = uu.uid
			  AND p.status = ?
			  AND (p.money > 0 OR p.payment_amount_minor > 0)
			  AND %s >= uu.ct AND %s < ?
		)
		AND NOT EXISTS (
			SELECT 1 FROM top_ups prev
			WHERE prev.user_id = uu.uid
			  AND prev.status = ?
			  AND (prev.money > 0 OR prev.payment_amount_minor > 0)
			  AND %s < uu.ct
		)`, paymentTime, paymentTime, paymentTime)
	args = []interface{}{common.UserStatusEnabled}
	if group != UsageReportGroupAll {
		args = append(args, group)
	}
	args = append(args, start, end, common.TopUpStatusSuccess, end, common.TopUpStatusSuccess)
	if err := model.DB.Raw(sql, args...).Scan(&paid).Error; err != nil {
		return 0, 0, fmt.Errorf("usage_report same-day paid: %w", err)
	}
	return activated, paid, nil
}
