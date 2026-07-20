package model

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCountLogsUpToGeneratesLimitedSubquery(t *testing.T) {
	dryRunDB := LOG_DB.Session(&gorm.Session{DryRun: true})
	filteredQuery := dryRunDB.Where("logs.user_id = ?", 42)

	statement := limitedLogCountQuery(dryRunDB, filteredQuery, logSearchCountLimit).Count(new(int64)).Statement
	query := strings.ToUpper(statement.SQL.String())

	require.Contains(t, query, "SELECT COUNT(*) FROM (SELECT LOGS.ID FROM")
	require.Contains(t, query, "WHERE LOGS.USER_ID = ?")
	require.Contains(t, query, "LIMIT 10000")
}

func TestGetUserLogsCapsTotalAtSearchLimit(t *testing.T) {
	truncateTables(t)

	logs := make([]Log, logSearchCountLimit+1)
	for i := range logs {
		logs[i] = Log{UserId: 42, Type: LogTypeConsume, CreatedAt: int64(i + 1)}
	}
	require.NoError(t, LOG_DB.CreateInBatches(&logs, 500).Error)

	got, total, err := GetUserLogs(42, LogTypeUnknown, 0, 0, "", "", 0, 2, "", "", "")
	require.NoError(t, err)
	require.Equal(t, int64(logSearchCountLimit), total)
	require.Len(t, got, 2)
	require.Equal(t, 1, got[0].Id)
	require.Equal(t, 2, got[1].Id)
}

func TestGetCodexChannelUsageStatsHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := GetCodexChannelUsageStats(ctx, []int{1}, 0, 0)
	require.Error(t, err)
}
