package model

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSupplierDailySummaryIncrementExpressionByDialect(t *testing.T) {
	tests := map[string]string{
		"postgres": `"supplier_usage_daily_summaries"."request_count" + EXCLUDED."request_count"`,
		"mysql":    "`request_count` + VALUES(`request_count`)",
		"sqlite":   "request_count + excluded.request_count",
	}
	for dialect, expected := range tests {
		t.Run(dialect, func(t *testing.T) {
			require.Equal(t, expected, supplierDailySummaryIncrementExpression(dialect, "request_count"))
		})
	}
}

func TestSupplierDailySummaryUpsertPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("set TEST_POSTGRES_DSN to run the PostgreSQL upsert regression")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { require.NoError(t, tx.Rollback().Error) })
	require.NoError(t, tx.AutoMigrate(&SupplierUsageDailySummary{}))

	fenceToken := time.Now().UnixNano()
	summary := SupplierUsageDailySummary{
		BatchDate:       "1900-01-01",
		BatchFenceToken: fenceToken,
		DimensionKey:    "g010-postgres-upsert",
		RequestCount:    1,
	}
	require.NoError(t, upsertSupplierDailySummaries(tx, []SupplierUsageDailySummary{summary}))
	summary.RequestCount = 2
	require.NoError(t, upsertSupplierDailySummaries(tx, []SupplierUsageDailySummary{summary}))

	var persisted SupplierUsageDailySummary
	require.NoError(t, tx.Where("batch_date = ? AND batch_fence_token = ? AND dimension_key = ?", summary.BatchDate, fenceToken, summary.DimensionKey).First(&persisted).Error)
	require.Equal(t, int64(3), persisted.RequestCount)
}
