package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionOrderSnapshotsPersistOneTimeFields(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	require.NoError(t, DB.AutoMigrate(&SubscriptionOrder{}))

	order := &SubscriptionOrder{
		UserId:          10,
		PlanId:          20,
		TradeNo:         "one_time_10",
		PaymentProvider: PaymentProviderStripe,
		PaymentMethod:   SubscriptionPaymentMethodAlipay,
		PurchaseMonths:  3,
		UnitPrice:       30,
		Money:           90,
		PlanSnapshot:    `{"title":"Quarterly","total_amount":3000}`,
		PurchaseIntent:  SubscriptionChangeIntentKindPurchase,
		RenewalSource:   SubscriptionRenewalSourceProvider,
		Status:          common.TopUpStatusPending,
	}

	require.NoError(t, DB.Create(order).Error)

	var stored SubscriptionOrder
	require.NoError(t, DB.Where("trade_no = ?", "one_time_10").First(&stored).Error)
	require.Equal(t, 10, stored.UserId)
	require.Equal(t, 20, stored.PlanId)
	require.Equal(t, "one_time_10", stored.TradeNo)
	require.Equal(t, PaymentProviderStripe, stored.PaymentProvider)
	require.Equal(t, SubscriptionPaymentMethodAlipay, stored.PaymentMethod)
	require.Equal(t, 3, stored.PurchaseMonths)
	require.Equal(t, float64(30), stored.UnitPrice)
	require.Equal(t, float64(90), stored.Money)
	require.Equal(t, `{"title":"Quarterly","total_amount":3000}`, stored.PlanSnapshot)
	require.Equal(t, SubscriptionChangeIntentKindPurchase, stored.PurchaseIntent)
	require.Equal(t, SubscriptionRenewalSourceProvider, stored.RenewalSource)
}

func TestSubscriptionTermSegmentRefundKeyUnique(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	require.NoError(t, DB.AutoMigrate(&SubscriptionTermSegment{}))

	refundKey := "refund-segment-1"
	require.NoError(t, DB.Create(&SubscriptionTermSegment{
		ContractId:     1001,
		OrderId:        2001,
		PlanId:         3001,
		SegmentIndex:   0,
		StartTime:      100,
		EndTime:        200,
		AllocatedMoney: 30,
		Status:         "active",
		RefundKey:      &refundKey,
	}).Error)

	duplicateRefundKey := refundKey
	err := DB.Create(&SubscriptionTermSegment{
		ContractId:     1002,
		OrderId:        2002,
		PlanId:         3002,
		SegmentIndex:   0,
		StartTime:      200,
		EndTime:        300,
		AllocatedMoney: 30,
		Status:         "active",
		RefundKey:      &duplicateRefundKey,
	}).Error
	require.Error(t, err)
}

func TestSubscriptionWalletLedgerEntryKeyUnique(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	require.NoError(t, DB.AutoMigrate(&WalletLedgerEntry{}))

	require.NoError(t, DB.Create(&WalletLedgerEntry{
		UserId:        10,
		EntryKey:      "wallet-entry-1",
		QuotaDelta:    500,
		MoneyAmount:   15,
		EntryType:     "renewal_debit",
		OrderId:       2001,
		TermSegmentId: 3001,
	}).Error)

	err := DB.Create(&WalletLedgerEntry{
		UserId:        11,
		EntryKey:      "wallet-entry-1",
		QuotaDelta:    500,
		MoneyAmount:   15,
		EntryType:     "renewal_debit",
		OrderId:       2002,
		TermSegmentId: 3002,
	}).Error
	require.Error(t, err)
}
