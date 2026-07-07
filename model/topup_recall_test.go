package model

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

type topUpRecallQueryCounter struct {
	count int
}

func (l *topUpRecallQueryCounter) LogMode(gormlogger.LogLevel) gormlogger.Interface {
	return l
}

func (l *topUpRecallQueryCounter) Info(context.Context, string, ...interface{})  {}
func (l *topUpRecallQueryCounter) Warn(context.Context, string, ...interface{})  {}
func (l *topUpRecallQueryCounter) Error(context.Context, string, ...interface{}) {}

func (l *topUpRecallQueryCounter) Trace(context.Context, time.Time, func() (string, int64), error) {
	l.count++
}

func setupTopUpRecallTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()

	originalDB := DB
	t.Cleanup(func() {
		DB = originalDB
	})

	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}, &TopUpRecall{}))
	DB = db
	return db
}

func TestTopUpRecallCandidatesExcludeInternalDomainsAndRecoveredUsers(t *testing.T) {
	db := setupTopUpRecallTestDB(t, "topup-recall-candidates")
	now := time.Now().Unix()

	users := []User{
		{Id: 1, Username: "public", Email: "buyer@example.com", AffCode: "recall-1"},
		{Id: 2, Username: "internal", Email: "staff@solvea.cx", AffCode: "recall-2"},
		{Id: 3, Username: "recovered", Email: "recovered@example.com", AffCode: "recall-3"},
		{Id: 4, Username: "fresh", Email: "fresh@example.com", AffCode: "recall-4"},
	}
	for _, user := range users {
		require.NoError(t, db.Create(&user).Error)
	}

	topups := []TopUp{
		{UserId: 1, Amount: 5, TradeNo: "expired-public", PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusExpired, CreateTime: now - int64(2*time.Hour.Seconds())},
		{UserId: 2, Amount: 5, TradeNo: "expired-internal", PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusExpired, CreateTime: now - int64(2*time.Hour.Seconds())},
		{UserId: 3, Amount: 5, TradeNo: "expired-recovered", PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusExpired, CreateTime: now - int64(2*time.Hour.Seconds())},
		{UserId: 3, Amount: 20, TradeNo: "success-recovered", PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusSuccess, CreateTime: now - int64(30*time.Minute.Seconds())},
		{UserId: 4, Amount: 5, TradeNo: "expired-fresh", PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusExpired, CreateTime: now - int64(30*time.Minute.Seconds())},
	}
	for _, topUp := range topups {
		require.NoError(t, db.Create(&topUp).Error)
	}

	candidates, err := GetEligibleTopUpRecallCandidates(now, 10)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, "expired-public", candidates[0].TradeNo)
	require.Equal(t, "buyer@example.com", candidates[0].Email)
}

func TestTopUpRecallCandidatesAllowRetryAfterFailedRecall(t *testing.T) {
	db := setupTopUpRecallTestDB(t, "topup-recall-failed-retry")
	now := time.Now().Unix()

	require.NoError(t, db.Create(&User{Id: 31, Username: "failed-retry", Email: "retry@example.com", AffCode: "recall-31"}).Error)
	require.NoError(t, db.Create(&TopUp{
		UserId:          31,
		Amount:          5,
		TradeNo:         "expired-failed-retry",
		PaymentProvider: PaymentProviderStripe,
		Status:          common.TopUpStatusExpired,
		CreateTime:      now - int64(2*time.Hour.Seconds()),
	}).Error)
	require.NoError(t, db.Create(&TopUpRecall{
		UserId:  31,
		TradeNo: "expired-failed-retry",
		Email:   "retry@example.com",
		Amount:  5,
		Status:  TopUpRecallStatusFailed,
		Error:   "smtp unavailable",
	}).Error)

	candidates, err := GetEligibleTopUpRecallCandidates(now, 10)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, "expired-failed-retry", candidates[0].TradeNo)
}

func TestTopUpRecallCandidateScanBatchesRelatedLookups(t *testing.T) {
	db := setupTopUpRecallTestDB(t, "topup-recall-batched-scan")
	now := time.Now().Unix()

	for i := 0; i < 6; i++ {
		userId := 40 + i
		require.NoError(t, db.Create(&User{
			Id:       userId,
			Username: "batch-user-" + string(rune('a'+i)),
			Email:    "batch-" + string(rune('a'+i)) + "@example.com",
			AffCode:  "batch-recall-" + string(rune('a'+i)),
		}).Error)
		require.NoError(t, db.Create(&TopUp{
			UserId:          userId,
			Amount:          5,
			TradeNo:         "expired-batched-" + string(rune('a'+i)),
			PaymentProvider: PaymentProviderStripe,
			Status:          common.TopUpStatusExpired,
			CreateTime:      now - int64(2*time.Hour.Seconds()),
		}).Error)
	}

	counter := &topUpRecallQueryCounter{}
	originalLogger := db.Config.Logger
	db.Config.Logger = counter
	t.Cleanup(func() {
		db.Config.Logger = originalLogger
	})

	candidates, err := GetEligibleTopUpRecallCandidates(now, 6)
	require.NoError(t, err)
	require.Len(t, candidates, 6)
	require.LessOrEqual(t, counter.count, 5, "candidate scan should batch recall, user, and recovery lookups")
}

func TestReserveTopUpRecallIsUniquePerUser(t *testing.T) {
	setupTopUpRecallTestDB(t, "topup-recall-reserve")

	first, reserved, err := ReserveTopUpRecall(TopUpRecallCandidate{
		UserId:  12,
		TradeNo: "expired-first",
		Email:   "buyer@example.com",
		Amount:  5,
	})
	require.NoError(t, err)
	require.True(t, reserved)
	require.NotNil(t, first)
	require.Equal(t, TopUpRecallStatusPending, first.Status)

	second, reserved, err := ReserveTopUpRecall(TopUpRecallCandidate{
		UserId:  12,
		TradeNo: "expired-second",
		Email:   "buyer@example.com",
		Amount:  5,
	})
	require.NoError(t, err)
	require.False(t, reserved)
	require.Nil(t, second)
}

func TestReserveTopUpRecallSkipsRecoveredUser(t *testing.T) {
	db := setupTopUpRecallTestDB(t, "topup-recall-reserve-recovered")
	now := time.Now().Unix()
	require.NoError(t, db.Create(&TopUp{
		UserId:          61,
		Amount:          5,
		TradeNo:         "success-after-expired",
		PaymentProvider: PaymentProviderStripe,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      now - int64(30*time.Minute.Seconds()),
	}).Error)

	recall, reserved, err := ReserveTopUpRecall(TopUpRecallCandidate{
		UserId:     61,
		TradeNo:    "expired-before-success",
		Email:      "recovered@example.com",
		Amount:     5,
		CreateTime: now - int64(2*time.Hour.Seconds()),
	})

	require.NoError(t, err)
	require.False(t, reserved)
	require.Nil(t, recall)
}

func TestReserveTopUpRecallReactivatesFailedRecall(t *testing.T) {
	db := setupTopUpRecallTestDB(t, "topup-recall-reserve-failed")
	require.NoError(t, db.Create(&TopUpRecall{
		UserId:        62,
		TradeNo:       "expired-reactivate",
		Email:         "retry@example.com",
		Amount:        5,
		Status:        TopUpRecallStatusFailed,
		PromotionCode: "SAVE2-old",
		Error:         "smtp unavailable",
	}).Error)

	recall, reserved, err := ReserveTopUpRecall(TopUpRecallCandidate{
		UserId:  62,
		TradeNo: "expired-reactivate",
		Email:   "retry@example.com",
		Amount:  5,
	})

	require.NoError(t, err)
	require.True(t, reserved)
	require.NotNil(t, recall)
	require.Equal(t, TopUpRecallStatusPending, recall.Status)
	require.Empty(t, recall.Error)
	require.Empty(t, recall.PromotionCode)
}

func TestTopUpRecallCandidatesWaitOneHourAfterExpiration(t *testing.T) {
	db := setupTopUpRecallTestDB(t, "topup-recall-expiration-delay")
	now := time.Now().Unix()

	users := []User{
		{Id: 21, Username: "recent-expired", Email: "recent@example.com", AffCode: "recall-21"},
		{Id: 22, Username: "old-expired", Email: "old@example.com", AffCode: "recall-22"},
	}
	for _, user := range users {
		require.NoError(t, db.Create(&user).Error)
	}

	topups := []TopUp{
		{UserId: 21, Amount: 5, TradeNo: "recent-expired", PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusExpired, CreateTime: now - int64(3*time.Hour.Seconds()), CompleteTime: now - int64(30*time.Minute.Seconds())},
		{UserId: 22, Amount: 5, TradeNo: "old-expired", PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusExpired, CreateTime: now - int64(3*time.Hour.Seconds()), CompleteTime: now - int64(2*time.Hour.Seconds())},
	}
	for _, topUp := range topups {
		require.NoError(t, db.Create(&topUp).Error)
	}

	candidates, err := GetEligibleTopUpRecallCandidates(now, 10)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, "old-expired", candidates[0].TradeNo)
}

func TestMigrateDBFastCreatesTopUpRecallTable(t *testing.T) {
	originalDB := DB
	originalUsingSQLite := common.UsingSQLite
	t.Cleanup(func() {
		DB = originalDB
		common.UsingSQLite = originalUsingSQLite
	})

	db, err := gorm.Open(sqlite.Open("file:topup-recall-fast-migrate?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	DB = db
	common.UsingSQLite = true

	require.NoError(t, migrateDBFast())
	require.True(t, db.Migrator().HasTable(&TopUpRecall{}))
}

func TestTopUpRecallUniqueIndexesDoNotDuplicateColumns(t *testing.T) {
	parsed, err := schema.Parse(&TopUpRecall{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)

	indexes := parsed.ParseIndexes()
	for name, column := range map[string]string{
		"idx_top_up_recalls_user_id":  "user_id",
		"idx_top_up_recalls_trade_no": "trade_no",
	} {
		index, ok := indexes[name]
		require.True(t, ok, "missing index %s", name)
		require.Equal(t, "UNIQUE", index.Class)
		require.Len(t, index.Fields, 1, "index %s must not repeat columns", name)
		require.Equal(t, column, index.Fields[0].DBName)
	}
}
