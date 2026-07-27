package model

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGetDBTimestampWithContextReturnsDatabaseErrorWithoutFallback(t *testing.T) {
	setupRecallEmailQuotaTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	timestamp, err := GetDBTimestampWithContext(ctx)

	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, timestamp)
}

func TestGetDBTimestampWithContextUsesActiveDatabaseDialect(t *testing.T) {
	setupRecallEmailQuotaTestDB(t)
	originalSQLite := common.UsingSQLite
	originalMySQL := common.UsingMySQL
	originalPostgreSQL := common.UsingPostgreSQL
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	t.Cleanup(func() {
		common.UsingSQLite = originalSQLite
		common.UsingMySQL = originalMySQL
		common.UsingPostgreSQL = originalPostgreSQL
	})

	timestamp, err := GetDBTimestampWithContext(context.Background())

	require.NoError(t, err)
	require.Positive(t, timestamp)
}

func TestGetDBTimestampWithContextRejectsUninitializedDatabase(t *testing.T) {
	originalDB := DB
	DB = nil
	t.Cleanup(func() { DB = originalDB })

	timestamp, err := GetDBTimestampWithContext(context.Background())

	require.ErrorContains(t, err, "database is not initialized")
	require.Zero(t, timestamp)

	timestamp, err = getDBTimestamp(nil)
	require.ErrorContains(t, err, "database is not initialized")
	require.Zero(t, timestamp)
}

func TestGetDBTimestampWithContextRejectsNilContext(t *testing.T) {
	setupRecallEmailQuotaTestDB(t)

	timestamp, err := GetDBTimestampWithContext(nil)

	require.ErrorContains(t, err, "context is nil")
	require.Zero(t, timestamp)
}
