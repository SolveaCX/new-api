package model

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupStripeWalletReconciliationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := DB

	dsn := filepath.Join(t.TempDir(), "stripe-wallet-reconciliation.db") + "?_pragma=busy_timeout(10000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() {
		DB = originalDB
		_ = sqlDB.Close()
	})
	require.NoError(t, db.AutoMigrate(&StripeWalletScanState{}, &StripeWalletPaymentCheck{}, &TopUp{}))
	DB = db
	return db
}

func stripeWalletDBNow(t *testing.T) int64 {
	t.Helper()
	now, err := GetDBTimestampWithContext(context.Background())
	require.NoError(t, err)
	return now
}

func TestStripeWalletPaymentCheckInsertIsStableAndDoesNotReopenClosed(t *testing.T) {
	db := setupStripeWalletReconciliationTestDB(t)
	ctx := context.Background()
	now := stripeWalletDBNow(t)

	first := &StripeWalletPaymentCheck{Scope: "acct:test", SessionID: "cs_1", EventID: "evt_1", TradeNo: "trade-1", NextCheckAt: now, ClosedAt: now}
	require.NoError(t, InsertStripeWalletPaymentCheck(ctx, first))
	require.NotZero(t, first.Id)
	require.NoError(t, InsertStripeWalletPaymentCheck(ctx, &StripeWalletPaymentCheck{
		Scope: "acct:test", SessionID: "cs_1", EventID: "evt_new", TradeNo: "trade-new", NextCheckAt: 0,
	}))

	var rows []StripeWalletPaymentCheck
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, "evt_1", rows[0].EventID)
	require.Equal(t, now, rows[0].ClosedAt)

	claimed, err := ClaimStripeWalletPaymentChecks(ctx, "acct:test", now+100, 120, 10, "worker")
	require.NoError(t, err)
	require.Empty(t, claimed)
}

func TestStripeWalletPaymentCheckConcurrentClaimsAreDisjoint(t *testing.T) {
	setupStripeWalletReconciliationTestDB(t)
	ctx := context.Background()
	now := stripeWalletDBNow(t)
	for i := 0; i < 20; i++ {
		require.NoError(t, InsertStripeWalletPaymentCheck(ctx, &StripeWalletPaymentCheck{
			Scope: "acct:test", SessionID: fmt.Sprintf("cs_%02d", i), NextCheckAt: now,
		}))
	}

	start := make(chan struct{})
	results := make(chan []StripeWalletPaymentCheck, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			<-start
			rows, err := ClaimStripeWalletPaymentChecks(ctx, "acct:test", now, 120, 20, token)
			results <- rows
			errs <- err
		}(fmt.Sprintf("worker-%d", i))
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	seen := make(map[int64]struct{})
	for rows := range results {
		for _, row := range rows {
			_, duplicate := seen[row.Id]
			require.False(t, duplicate, "row %d was claimed twice", row.Id)
			seen[row.Id] = struct{}{}
		}
	}
	require.Len(t, seen, 20)
}

func TestStripeWalletPaymentCheckSaveFencesOldTokenAndReleasesLease(t *testing.T) {
	db := setupStripeWalletReconciliationTestDB(t)
	ctx := context.Background()
	now := stripeWalletDBNow(t)
	require.NoError(t, InsertStripeWalletPaymentCheck(ctx, &StripeWalletPaymentCheck{Scope: "acct:test", SessionID: "cs_1", NextCheckAt: now}))

	claimed, err := ClaimStripeWalletPaymentChecks(ctx, "acct:test", now, 120, 1, "old-token")
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.NoError(t, db.Model(&StripeWalletPaymentCheck{}).Where("id = ?", claimed[0].Id).
		Updates(map[string]any{"lease_token": "new-token", "lease_until": now + 120}).Error)

	claimed[0].LocalStatus = "failed"
	err = SaveStripeWalletPaymentCheck(ctx, &claimed[0], "old-token")
	require.ErrorIs(t, err, ErrStripeWalletReconciliationLeaseLost)

	claimed[0].LeaseToken = "new-token"
	claimed[0].LeaseUntil = now + 120
	claimed[0].FirstAnomalyAt = now
	claimed[0].NextCheckAt = now + 3600
	require.NoError(t, SaveStripeWalletPaymentCheck(ctx, &claimed[0], "new-token"))

	var stored StripeWalletPaymentCheck
	require.NoError(t, db.First(&stored, claimed[0].Id).Error)
	require.Equal(t, "failed", stored.LocalStatus)
	require.Empty(t, stored.LeaseToken)
	require.Zero(t, stored.LeaseUntil)
}

func TestStripeWalletPaymentCheckCheckpointRetainsAndFencesLease(t *testing.T) {
	db := setupStripeWalletReconciliationTestDB(t)
	ctx := context.Background()
	now := stripeWalletDBNow(t)
	require.NoError(t, InsertStripeWalletPaymentCheck(ctx, &StripeWalletPaymentCheck{Scope: "acct:test", SessionID: "cs_checkpoint", NextCheckAt: now}))
	claimed, err := ClaimStripeWalletPaymentChecks(ctx, "acct:test", now, 120, 1, "worker-1")
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	claimed[0].FirstAnomalyAt = now
	claimed[0].LocalStatus = "failed"
	claimed[0].UserID = 42
	require.NoError(t, CheckpointStripeWalletPaymentCheck(ctx, &claimed[0], "worker-1"))
	other, err := ClaimStripeWalletPaymentChecks(ctx, "acct:test", now, 120, 1, "worker-2")
	require.NoError(t, err)
	require.Empty(t, other)

	var stored StripeWalletPaymentCheck
	require.NoError(t, db.First(&stored, claimed[0].Id).Error)
	require.Equal(t, "worker-1", stored.LeaseToken)
	require.Equal(t, "failed", stored.LocalStatus)
	require.Equal(t, 42, stored.UserID)

	require.NoError(t, db.Model(&StripeWalletPaymentCheck{}).Where("id = ?", claimed[0].Id).Update("lease_until", int64(1)).Error)
	err = CheckpointStripeWalletPaymentCheck(ctx, &claimed[0], "worker-1")
	require.ErrorIs(t, err, ErrStripeWalletReconciliationLeaseLost)
}

func TestStripeWalletScanStatePersistsAndFencesStaleOwner(t *testing.T) {
	db := setupStripeWalletReconciliationTestDB(t)
	ctx := context.Background()
	now := stripeWalletDBNow(t)
	state, acquired, err := AcquireStripeWalletScan(ctx, "acct:test", now, 120, "worker-1")
	require.NoError(t, err)
	require.True(t, acquired)
	require.Equal(t, now, state.InitialAt)
	require.Equal(t, now, state.RecentThrough)

	other, acquired, err := AcquireStripeWalletScan(ctx, "acct:test", now+10, 120, "worker-2")
	require.NoError(t, err)
	require.False(t, acquired)
	require.Nil(t, other)

	state.RecentAfter = "cs_cursor"
	state.BackfillAfter = "cs_backfill"
	state.LastSuccessAt = now
	require.NoError(t, SaveStripeWalletScan(ctx, state, "worker-1"))
	require.NoError(t, ReleaseStripeWalletScan(ctx, state.Scope, "worker-1"))
	require.ErrorIs(t, SaveStripeWalletScan(ctx, state, "worker-1"), ErrStripeWalletReconciliationLeaseLost)

	var stored StripeWalletScanState
	require.NoError(t, db.First(&stored, "scope = ?", state.Scope).Error)
	require.Equal(t, "cs_cursor", stored.RecentAfter)
	require.Equal(t, "cs_backfill", stored.BackfillAfter)
	require.Equal(t, now, stored.InitialAt)

	// A fresh GORM session proves the state comes from the database rather than
	// process-local reconciliation memory.
	var restarted StripeWalletScanState
	require.NoError(t, db.Session(&gorm.Session{NewDB: true}).First(&restarted, "scope = ?", state.Scope).Error)
	require.Equal(t, stored.RecentAfter, restarted.RecentAfter)
}

func TestStripeWalletScanStateSurvivesDatabaseRestart(t *testing.T) {
	originalDB := DB
	t.Cleanup(func() { DB = originalDB })
	path := filepath.Join(t.TempDir(), "restart.db")
	open := func() *gorm.DB {
		db, err := gorm.Open(sqlite.Open(path+"?_pragma=busy_timeout(10000)"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, err)
		return db
	}

	db := open()
	require.NoError(t, db.AutoMigrate(&StripeWalletScanState{}))
	DB = db
	now := stripeWalletDBNow(t)
	state, acquired, err := AcquireStripeWalletScan(context.Background(), "acct:restart", now, 120, "worker")
	require.NoError(t, err)
	require.True(t, acquired)
	state.RecentAfter = "cs_persisted"
	state.BackfillDone = true
	require.NoError(t, SaveStripeWalletScan(context.Background(), state, "worker"))
	require.NoError(t, ReleaseStripeWalletScan(context.Background(), state.Scope, "worker"))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	db = open()
	DB = db
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	var restarted StripeWalletScanState
	require.NoError(t, db.First(&restarted, "scope = ?", "acct:restart").Error)
	require.Equal(t, "cs_persisted", restarted.RecentAfter)
	require.True(t, restarted.BackfillDone)
	require.Equal(t, now, restarted.InitialAt)
}

func TestStripeWalletReconciliationStoreNeverMutatesTopUp(t *testing.T) {
	db := setupStripeWalletReconciliationTestDB(t)
	ctx := context.Background()
	now := stripeWalletDBNow(t)
	original := TopUp{TradeNo: "trade-protected", UserId: 9, Amount: 123, Status: "failed", CompleteTime: 17}
	require.NoError(t, db.Create(&original).Error)

	state, acquired, err := AcquireStripeWalletScan(ctx, "acct:test", now, 120, "scan-worker")
	require.NoError(t, err)
	require.True(t, acquired)
	state.RecentThrough = now + 1
	require.NoError(t, SaveStripeWalletScan(ctx, state, "scan-worker"))
	require.NoError(t, ReleaseStripeWalletScan(ctx, state.Scope, "scan-worker"))
	require.NoError(t, InsertStripeWalletPaymentCheck(ctx, &StripeWalletPaymentCheck{
		Scope: "acct:test", SessionID: "cs_protected", TradeNo: original.TradeNo, NextCheckAt: now,
	}))
	claimed, err := ClaimStripeWalletPaymentChecks(ctx, "acct:test", now, 120, 1, "check-worker")
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	claimed[0].LocalStatus = "failed"
	require.NoError(t, SaveStripeWalletPaymentCheck(ctx, &claimed[0], "check-worker"))

	var stored TopUp
	require.NoError(t, db.First(&stored, original.Id).Error)
	require.Equal(t, original.UserId, stored.UserId)
	require.Equal(t, original.Amount, stored.Amount)
	require.Equal(t, original.Status, stored.Status)
	require.Equal(t, original.CompleteTime, stored.CompleteTime)
}

func TestGetStripeWalletTopUpDistinguishesMissingFromDatabaseFailure(t *testing.T) {
	db := setupStripeWalletReconciliationTestDB(t)
	ctx := context.Background()
	require.NoError(t, db.Create(&TopUp{TradeNo: "trade-1", UserId: 7, Amount: 99, Status: "failed"}).Error)

	topUp, err := GetStripeWalletTopUpForReconciliation(ctx, "trade-1")
	require.NoError(t, err)
	require.Equal(t, 7, topUp.UserId)
	_, err = GetStripeWalletTopUpForReconciliation(ctx, "missing")
	require.ErrorIs(t, err, ErrTopUpNotFound)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = GetStripeWalletTopUpForReconciliation(ctx, "trade-1")
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrTopUpNotFound))
}

func TestStripeWalletReconciliationModelsAreMigrated(t *testing.T) {
	names := make(map[string]bool)
	for _, migration := range orderedMigrationModels() {
		names[migration.name] = true
	}
	require.True(t, names["StripeWalletScanState"])
	require.True(t, names["StripeWalletPaymentCheck"])
}
