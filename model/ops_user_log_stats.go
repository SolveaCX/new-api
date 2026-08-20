package model

import (
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OpsUserLogStatsRow is the pre-aggregated per-user consume-log statistics that
// back GetOpsUserLogStats. The alternative — scanning the whole logs history
// per user on every report rebuild — full-scans a ~45M-row table on prod
// (measured 100s+), so the report now reads this small table instead.
//
// The table lives in LOG_DB next to logs (a separate database when
// LOG_SQL_DSN is set) and is maintained by StartOpsUserLogStatsSyncTask:
// an incremental pass aggregates every consume log newer than the stored
// cursor and upserts per-user rows. The first run backfills from the whole
// logs history (one-time, in the background, never on the request path).
//
// Semantics mirror GetOpsUserLogStats exactly:
//   - playground = token_name LIKE 'playground%' (auto-fired onboarding call)
//   - api key    = token_id > 0 AND NOT playground (opsExternalAPIKeyLogPredicate)
//
// Deleting old logs (DeleteOldLog) does not retroactively shrink these rows:
// the cursor only moves forward, and first_* values were captured from the
// earliest matching log seen so far.

type OpsUserLogStatsRow struct {
	UserId            int   `gorm:"column:user_id;primaryKey;autoIncrement:false"`
	FirstPlaygroundAt int64 `gorm:"column:first_playground_at;default:0"`
	PlaygroundCount   int   `gorm:"column:playground_count;default:0"`
	FirstApiKeyAt     int64 `gorm:"column:first_api_key_at;default:0"`
	ApiKeyCount       int   `gorm:"column:api_key_count;default:0"`
	LastRequestAt     int64 `gorm:"column:last_request_at;default:0"`
	UpdatedAt         int64 `gorm:"column:updated_at;default:0"`
}

func (OpsUserLogStatsRow) TableName() string {
	return "ops_user_log_stats"
}

// OpsUserLogStatsMeta is the single-row sync cursor for the aggregation task:
// LastLogId is the highest logs.id already folded into ops_user_log_stats.
// Backfilled flips true only after the first full pass has caught up with the
// log tail — until then the report must keep using the slow direct scan rather
// than reading a half-populated table.
type OpsUserLogStatsMeta struct {
	Id         int   `gorm:"primaryKey;autoIncrement:false"` // always 1
	LastLogId  int64 `gorm:"column:last_log_id;default:0"`
	Backfilled bool  `gorm:"column:backfilled;default:false"`
	UpdatedAt  int64 `gorm:"column:updated_at;default:0"`
}

func (OpsUserLogStatsMeta) TableName() string {
	return "ops_user_log_stats_meta"
}

const (
	opsUserLogStatsSyncBatch = 50000
	opsUserLogStatsSyncEvery = 5 * time.Minute
)

// getOpsUserLogStatsMeta reads the single cursor row, creating it on first use.
func getOpsUserLogStatsMeta() (*OpsUserLogStatsMeta, error) {
	var meta OpsUserLogStatsMeta
	err := LOG_DB.Where("id = 1").First(&meta).Error
	if err == nil {
		return &meta, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	meta = OpsUserLogStatsMeta{Id: 1}
	if err := LOG_DB.Create(&meta).Error; err != nil {
		return nil, err
	}
	return &meta, nil
}

// opsUserLogStatsReady reports whether the aggregate table is fully usable:
// the cursor has advanced past the first backfill (a full pass that caught up
// with the log tail). Single-row primary-key read, cheap.
func opsUserLogStatsReady() bool {
	meta, err := getOpsUserLogStatsMeta()
	return err == nil && meta.Backfilled
}

// SyncOpsUserLogStats runs one incremental aggregation pass: consume logs with
// id > cursor are aggregated per user and upserted into ops_user_log_stats,
// then the cursor advances. The first pass (cursor == 0) backfills the whole
// logs history in batches. Safe on multi-node: only the master node runs the
// task, and per-user upserts are idempotent by primary key.
func SyncOpsUserLogStats() error {
	meta, err := getOpsUserLogStatsMeta()
	if err != nil {
		return err
	}
	cursor := meta.LastLogId
	reachedTail := false
	for {
		var logs []*Log
		if err := LOG_DB.Select("id", "user_id", "created_at", "token_name", "token_id").
			Where("type = ? AND id > ?", LogTypeConsume, cursor).
			Order("id").
			Limit(opsUserLogStatsSyncBatch).
			Find(&logs).Error; err != nil {
			return err
		}
		if len(logs) == 0 {
			// No rows newer than the cursor: the pass reached the log tail.
			reachedTail = true
			break
		}
		agg := map[int]*OpsUserLogStatsRow{}
		for _, l := range logs {
			row, ok := agg[l.UserId]
			if !ok {
				row = &OpsUserLogStatsRow{UserId: l.UserId}
				agg[l.UserId] = row
			}
			if strings.HasPrefix(l.TokenName, "playground") {
				row.PlaygroundCount++
				if row.FirstPlaygroundAt == 0 || l.CreatedAt < row.FirstPlaygroundAt {
					row.FirstPlaygroundAt = l.CreatedAt
				}
			} else if l.TokenId > 0 {
				row.ApiKeyCount++
				if row.FirstApiKeyAt == 0 || l.CreatedAt < row.FirstApiKeyAt {
					row.FirstApiKeyAt = l.CreatedAt
				}
			}
			if l.CreatedAt > row.LastRequestAt {
				row.LastRequestAt = l.CreatedAt
			}
		}

		// Merge with the rows already in the table (the same user can appear
		// across batches): counts accumulate, first_* keep the earliest
		// non-zero value, last_request_at takes the max. Then overwrite the
		// whole row so the upsert is portable across SQLite/MySQL/PostgreSQL.
		ids := make([]int, 0, len(agg))
		for uid := range agg {
			ids = append(ids, uid)
		}
		var existing []OpsUserLogStatsRow
		if err := LOG_DB.Where("user_id IN ?", ids).Find(&existing).Error; err != nil {
			return err
		}
		byId := map[int]*OpsUserLogStatsRow{}
		for i := range existing {
			byId[existing[i].UserId] = &existing[i]
		}
		now := common.GetTimestamp()
		rows := make([]OpsUserLogStatsRow, 0, len(agg))
		for _, row := range agg {
			if old, ok := byId[row.UserId]; ok {
				row.PlaygroundCount += old.PlaygroundCount
				row.ApiKeyCount += old.ApiKeyCount
				if old.FirstPlaygroundAt > 0 && (row.FirstPlaygroundAt == 0 || old.FirstPlaygroundAt < row.FirstPlaygroundAt) {
					row.FirstPlaygroundAt = old.FirstPlaygroundAt
				}
				if old.FirstApiKeyAt > 0 && (row.FirstApiKeyAt == 0 || old.FirstApiKeyAt < row.FirstApiKeyAt) {
					row.FirstApiKeyAt = old.FirstApiKeyAt
				}
				if old.LastRequestAt > row.LastRequestAt {
					row.LastRequestAt = old.LastRequestAt
				}
			}
			row.UpdatedAt = now
			rows = append(rows, *row)
		}
		if err := LOG_DB.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"first_playground_at", "playground_count", "first_api_key_at",
				"api_key_count", "last_request_at", "updated_at",
			}),
		}).Create(&rows).Error; err != nil {
			return err
		}
		cursor = int64(logs[len(logs)-1].Id)
		// Persist the cursor after every batch so a crash resumes from here.
		if err := LOG_DB.Model(&OpsUserLogStatsMeta{}).Where("id = 1").
			Updates(map[string]interface{}{"last_log_id": cursor, "updated_at": now}).Error; err != nil {
			return err
		}
		if len(logs) < opsUserLogStatsSyncBatch {
			// A short final batch means we drained the tail in this pass.
			reachedTail = true
			break
		}
	}
	// Only a pass that actually reached the log tail completes the backfill;
	// a partial first pass must not mark the table ready.
	if reachedTail && !meta.Backfilled {
		if err := LOG_DB.Model(&OpsUserLogStatsMeta{}).Where("id = 1").
			Update("backfilled", true).Error; err != nil {
			return err
		}
	}
	return nil
}

var opsUserLogStatsTaskOnce sync.Once

// StartOpsUserLogStatsSyncTask runs the incremental aggregation every
// opsUserLogStatsSyncEvery on the master node (single writer per Rule 11; the
// report reads are multi-node safe because the table is shared and upserts are
// idempotent). The first pass is a full backfill, so the aggregate table
// becomes available within one batch cycle of startup.
func StartOpsUserLogStatsSyncTask() {
	if !common.IsMasterNode {
		return
	}
	opsUserLogStatsTaskOnce.Do(func() {
		go func() {
			for {
				if err := SyncOpsUserLogStats(); err != nil {
					common.SysError("ops user log stats sync: " + err.Error())
				}
				time.Sleep(opsUserLogStatsSyncEvery)
			}
		}()
	})
}
