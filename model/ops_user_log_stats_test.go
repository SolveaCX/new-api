package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newOpsLogStatsTestDB opens an isolated in-memory SQLite database wired as
// both DB and LOG_DB, migrating just the tables the aggregation touches.
func newOpsLogStatsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}, &OpsUserLogStatsRow{}, &OpsUserLogStatsMeta{}))

	oldDB, oldLogDB := DB, LOG_DB
	DB = db
	LOG_DB = db
	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLogDB
	})
	return db
}

func TestSyncOpsUserLogStatsAggregatesPlaygroundAndAPIKeys(t *testing.T) {
	db := newOpsLogStatsTestDB(t)
	now := common.GetTimestamp()

	logs := []*Log{
		// playground: auto-fired onboarding call (token_name playground-x)
		{Id: 1, UserId: 100, CreatedAt: now - 1000, Type: LogTypeConsume, TokenName: "playground-abc"},
		{Id: 2, UserId: 100, CreatedAt: now - 900, Type: LogTypeConsume, TokenName: "playground-abc"},
		// real API key usage (token_id > 0, non-playground name)
		{Id: 3, UserId: 100, CreatedAt: now - 800, Type: LogTypeConsume, TokenName: "main-key", TokenId: 7},
		{Id: 4, UserId: 100, CreatedAt: now - 700, Type: LogTypeConsume, TokenName: "main-key", TokenId: 7},
		// second user, API key only
		{Id: 5, UserId: 200, CreatedAt: now - 600, Type: LogTypeConsume, TokenName: "cli-key", TokenId: 8},
		// non-consume rows must be ignored
		{Id: 6, UserId: 100, CreatedAt: now - 500, Type: LogTypeTopup, TokenName: ""},
		// token_id=0 consume rows (playground-name check still holds)
		{Id: 7, UserId: 200, CreatedAt: now - 400, Type: LogTypeConsume, TokenName: "playground-x", TokenId: 0},
	}
	require.NoError(t, db.Create(&logs).Error)

	require.NoError(t, SyncOpsUserLogStats())

	meta, err := getOpsUserLogStatsMeta()
	require.NoError(t, err)
	require.True(t, meta.Backfilled, "first pass that drains the tail must mark the table ready")
	require.EqualValues(t, 7, meta.LastLogId)

	rows, err := GetOpsUserLogStats([]int{100, 200})
	require.NoError(t, err)
	byUser := map[int]*OpsUserLogStats{}
	for _, r := range rows {
		byUser[r.UserId] = r
	}

	u100 := byUser[100]
	require.NotNil(t, u100)
	require.Equal(t, 2, u100.PlaygroundCount)
	require.Equal(t, now-1000, u100.FirstPlaygroundAt)
	require.Equal(t, 2, u100.ApiKeyCount)
	require.Equal(t, now-800, u100.FirstApiKeyAt)
	require.Equal(t, now-700, u100.LastRequestAt)

	u200 := byUser[200]
	require.NotNil(t, u200)
	require.Equal(t, 1, u200.PlaygroundCount)
	require.Equal(t, 1, u200.ApiKeyCount, "cli-key with token_id>0 is a real API-key call")
	require.Equal(t, now-600, u200.FirstApiKeyAt)
	require.Equal(t, now-400, u200.LastRequestAt)
}

func TestSyncOpsUserLogStatsIncrementalAccumulates(t *testing.T) {
	db := newOpsLogStatsTestDB(t)
	now := common.GetTimestamp()

	// First pass: one user, one playground log.
	require.NoError(t, db.Create(&Log{Id: 1, UserId: 100, CreatedAt: now - 1000, Type: LogTypeConsume, TokenName: "playground-a"}).Error)
	require.NoError(t, SyncOpsUserLogStats())

	// Second pass: a newer API-key log for the same user must accumulate
	// (not overwrite) the earlier playground count, and a new user appears.
	require.NoError(t, db.Create(&Log{Id: 2, UserId: 100, CreatedAt: now - 500, Type: LogTypeConsume, TokenName: "main-key", TokenId: 3}).Error)
	require.NoError(t, db.Create(&Log{Id: 3, UserId: 300, CreatedAt: now - 400, Type: LogTypeConsume, TokenName: "cli-key", TokenId: 4}).Error)
	require.NoError(t, SyncOpsUserLogStats())

	rows, err := GetOpsUserLogStats([]int{100, 300})
	require.NoError(t, err)
	byUser := map[int]*OpsUserLogStats{}
	for _, r := range rows {
		byUser[r.UserId] = r
	}

	u100 := byUser[100]
	require.NotNil(t, u100)
	require.Equal(t, 1, u100.PlaygroundCount)
	require.Equal(t, 1, u100.ApiKeyCount)
	require.Equal(t, now-1000, u100.FirstPlaygroundAt, "first playground must survive the incremental pass")
	require.Equal(t, now-500, u100.FirstApiKeyAt)
	require.Equal(t, now-500, u100.LastRequestAt)

	u300 := byUser[300]
	require.NotNil(t, u300)
	require.Equal(t, 1, u300.ApiKeyCount)
}

func TestGetOpsUserLogStatsFallsBackBeforeBackfill(t *testing.T) {
	db := newOpsLogStatsTestDB(t)
	now := common.GetTimestamp()
	require.NoError(t, db.Create(&Log{Id: 1, UserId: 100, CreatedAt: now - 1000, Type: LogTypeConsume, TokenName: "playground-a"}).Error)

	// Table exists but backfill has not completed: must use the direct logs
	// scan so the report is never empty.
	rows, err := GetOpsUserLogStats([]int{100})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 1, rows[0].PlaygroundCount)
	require.Equal(t, 0, rows[0].ApiKeyCount)
}
