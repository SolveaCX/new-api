package model

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPaymentAnalyticsOutboxTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "payment-analytics.db")+"?_pragma=busy_timeout(5000)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&PaymentAnalyticsOutbox{}))
	DB = db
	t.Cleanup(func() {
		DB = originalDB
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func paymentAnalyticsTestEvent() *PaymentAnalyticsEvent {
	return &PaymentAnalyticsEvent{
		EventID: "flatkey:ga4:purchase:topup:order-1", TransactionID: "order-1", UserID: 7,
		Value: 10, Currency: "usd", PaymentProvider: PaymentProviderStripe, PaymentMethod: PaymentMethodStripe,
		ProductType: "top_up", ItemID: "wallet_top_up", ItemName: "Wallet top-up", ClientID: "123.456", SessionID: "789", OccurredAt: 1_800_000_000,
	}
}

func TestPaymentAnalyticsOutboxEnqueueIsIdempotentAndClaimsExclusively(t *testing.T) {
	db := setupPaymentAnalyticsOutboxTestDB(t)
	EnqueuePaymentAnalyticsBestEffort(paymentAnalyticsTestEvent())
	EnqueuePaymentAnalyticsBestEffort(paymentAnalyticsTestEvent())
	var count int64
	require.NoError(t, db.Model(&PaymentAnalyticsOutbox{}).Count(&count).Error)
	require.EqualValues(t, 1, count)

	const now = int64(1_800_000_100)
	claimed, err := ClaimPaymentAnalyticsOutbox(1, now)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	second, err := ClaimPaymentAnalyticsOutbox(1, now)
	require.NoError(t, err)
	require.Empty(t, second)

	recovered, err := ClaimPaymentAnalyticsOutbox(1, now+10*60+1)
	require.NoError(t, err)
	require.Len(t, recovered, 1)
	require.Equal(t, 2, recovered[0].Attempts)
}

func TestPaymentAnalyticsOutboxRejectsStaleWorkerCompletion(t *testing.T) {
	setupPaymentAnalyticsOutboxTestDB(t)
	EnqueuePaymentAnalyticsBestEffort(paymentAnalyticsTestEvent())
	const firstClaimAt = int64(1_800_000_100)
	first, err := ClaimPaymentAnalyticsOutbox(1, firstClaimAt)
	require.NoError(t, err)
	require.Len(t, first, 1)
	second, err := ClaimPaymentAnalyticsOutbox(1, firstClaimAt+10*60+1)
	require.NoError(t, err)
	require.Len(t, second, 1)

	require.NoError(t, CompletePaymentAnalyticsOutbox(first[0].Id, first[0].ClaimedAt, firstClaimAt+10*60+2))
	var outbox PaymentAnalyticsOutbox
	require.NoError(t, DB.First(&outbox, first[0].Id).Error)
	require.Equal(t, PaymentAnalyticsOutboxDelivering, outbox.Status)
	require.Equal(t, second[0].ClaimedAt, outbox.ClaimedAt)
}

func TestRechargeSucceedsWhenPaymentAnalyticsEnqueueIsUnavailable(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 9123, 0)
	topup := &TopUp{
		UserId: 9123, Amount: 2, Money: 9.99, PaymentCurrency: "USD", TradeNo: "analytics-unavailable",
		PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe, GAClientID: "123.456", GASessionID: "789",
		Status: common.TopUpStatusPending, CreateTime: common.GetTimestamp(),
	}
	require.NoError(t, topup.Insert())

	original := paymentAnalyticsOutboxCreate
	paymentAnalyticsOutboxCreate = func(*PaymentAnalyticsOutbox) error {
		return errors.New("outbox unavailable")
	}
	t.Cleanup(func() { paymentAnalyticsOutboxCreate = original })

	credited, err := Recharge("analytics-unavailable", "cus_analytics", "127.0.0.1")
	require.NoError(t, err)
	require.True(t, credited)
	require.Equal(t, common.TopUpStatusSuccess, getTopUpStatusForPaymentGuardTest(t, "analytics-unavailable"))
	require.Equal(t, int(2*common.QuotaPerUnit), getUserQuotaForPaymentGuardTest(t, 9123))
}
