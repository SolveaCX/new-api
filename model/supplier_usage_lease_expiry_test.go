package model

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSupplierDailyBatchExpiredLeaseCannotMutateBeforeTakeover(t *testing.T) {
	db := setupSupplierTestDB(t, t.Name())
	require.NoError(t, db.AutoMigrate(&SupplierUsageDailySummary{}, &SupplierUsageDailyBatchRun{}))
	ctx := context.Background()
	day := time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		mutate func(*gorm.DB, SupplierDailyBatchLease) error
	}{
		{
			name: "persist page",
			mutate: func(db *gorm.DB, lease SupplierDailyBatchLease) error {
				return PersistSupplierDailyBatchPage(ctx, db, lease, []SupplierUsageDailySummary{{DimensionKey: "expired-page", RequestCount: 1}}, 100, 1, 1, 1, time.Minute)
			},
		},
		{
			name: "renew lease",
			mutate: func(db *gorm.DB, lease SupplierDailyBatchLease) error {
				return RenewSupplierDailyBatchLease(ctx, db, lease, time.Minute)
			},
		},
		{
			name: "publish",
			mutate: func(db *gorm.DB, lease SupplierDailyBatchLease) error {
				return PublishSupplierDailyBatch(ctx, db, lease, time.Now(), types.SupplierPublishedEvidenceV1{
					SchemaVersion:                    types.SupplierPublishedEvidenceSchemaVersion,
					DispositionCounts:                types.SupplierPublishedDispositionCountsV1{},
					PersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessComplete,
					Warnings:                         []types.SupplierPublishedWarningV1{},
				})
			},
		},
		{
			name: "fail",
			mutate: func(db *gorm.DB, lease SupplierDailyBatchLease) error {
				return FailSupplierDailyBatch(ctx, db, lease, errors.New("late cleanup"))
			},
		},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			batchDay := day.AddDate(0, 0, index)
			batchDate := batchDay.Format("2006-01-02")
			lease, err := AcquireSupplierDailyBatch(ctx, db, batchDate, batchDay.Unix(), batchDay.AddDate(0, 0, 1).Unix(), fmt.Sprintf("node-%d", index), time.Time{}, time.Minute, false)
			require.NoError(t, err)
			dbNow, err := supplierDBUnix(ctx, db)
			require.NoError(t, err)
			require.NoError(t, db.Model(&SupplierUsageDailyBatchRun{}).Where("id = ?", lease.RunId).UpdateColumn("locked_until", dbNow-1).Error)

			require.ErrorIs(t, test.mutate(db, lease), ErrSupplierDailyBatchFenceLost)

			var run SupplierUsageDailyBatchRun
			require.NoError(t, db.First(&run, lease.RunId).Error)
			require.Equal(t, SupplierDailyBatchStatusRunning, run.Status)
			require.Equal(t, lease.Owner, run.LeaseOwner)
			require.Equal(t, lease.FenceToken, run.FenceToken)
			require.Equal(t, dbNow-1, run.LockedUntil)
			var summaryCount int64
			require.NoError(t, db.Model(&SupplierUsageDailySummary{}).Where("batch_date = ? AND batch_fence_token = ?", batchDate, lease.FenceToken).Count(&summaryCount).Error)
			require.Zero(t, summaryCount, "an expired writer must not leave candidate mutations before takeover cleanup")
		})
	}
}
