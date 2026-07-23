package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestRefundSubscriptionTermSegmentCreditsOnlyNotStartedOneTimeCanonicalValue(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	insertPurchaseServiceUser(t, 7601, 25)
	plan := insertPurchaseServicePlan(t, 7701, 1, 12, 1200)
	contract := model.UserSubscriptionContract{
		UserId:        7601,
		Status:        model.SubscriptionContractStatusActive,
		PaymentMode:   model.SubscriptionPaymentModePrepaid,
		CurrentPlanId: plan.Id,
	}
	require.NoError(t, model.DB.Create(&contract).Error)
	order := model.SubscriptionOrder{
		UserId:             7601,
		PlanId:             plan.Id,
		Money:              99,
		TradeNo:            "term-refund-local-currency",
		PaymentMethod:      SubscriptionPaymentChoicePix,
		PaymentProvider:    model.PaymentProviderStripe,
		Status:             common.TopUpStatusSuccess,
		CreateTime:         common.GetTimestamp(),
		PurchaseMonths:     2,
		UnitPrice:          49.5,
		PaymentCurrency:    "BRL",
		PaymentAmountMinor: 9900,
	}
	require.NoError(t, model.DB.Create(&order).Error)
	term := model.SubscriptionTermSegment{
		ContractId:     contract.Id,
		OrderId:        order.Id,
		PlanId:         plan.Id,
		SegmentIndex:   1,
		StartTime:      common.GetTimestamp() + 3600,
		EndTime:        common.GetTimestamp() + 7200,
		AllocatedMoney: plan.PriceAmount,
		Status:         model.SubscriptionTermStatusNotStarted,
	}
	require.NoError(t, model.DB.Create(&term).Error)

	result, err := RefundSubscriptionTermSegment(7601, term.Id)

	require.NoError(t, err)
	require.Equal(t, int64(1200), result.RefundedQuota)
	require.Equal(t, float64(12), result.RefundedMoney)
	var user model.User
	require.NoError(t, model.DB.First(&user, "id = ?", 7601).Error)
	require.Equal(t, 1225, user.Quota)
	var stored model.SubscriptionTermSegment
	require.NoError(t, model.DB.First(&stored, "id = ?", term.Id).Error)
	require.Equal(t, model.SubscriptionTermStatusRefunded, stored.Status)
	require.NotNil(t, stored.RefundKey)
	var ledger model.WalletLedgerEntry
	require.NoError(t, model.DB.Where("entry_key = ?", *stored.RefundKey).First(&ledger).Error)
	require.Equal(t, int64(1200), ledger.QuotaDelta)
	require.Equal(t, float64(12), ledger.MoneyAmount)
	require.Equal(t, model.WalletLedgerEntryTypePrepaidRefund, ledger.EntryType)
}

func TestRefundSubscriptionTermSegmentRejectsNonNotStartedStatuses(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	insertPurchaseServiceUser(t, 7602, 0)
	plan := insertPurchaseServicePlan(t, 7702, 1, 5, 500)
	contract := model.UserSubscriptionContract{UserId: 7602, Status: model.SubscriptionContractStatusActive, PaymentMode: model.SubscriptionPaymentModePrepaid, CurrentPlanId: plan.Id}
	require.NoError(t, model.DB.Create(&contract).Error)
	order := model.SubscriptionOrder{UserId: 7602, PlanId: plan.Id, TradeNo: "term-refund-reject", PaymentProvider: model.PaymentProviderBalance, Status: common.TopUpStatusSuccess}
	require.NoError(t, model.DB.Create(&order).Error)
	for index, status := range []string{
		model.SubscriptionTermStatusActive,
		model.SubscriptionTermStatusReplaced,
		"completed",
		"cancelled",
		model.SubscriptionTermStatusRefunded,
	} {
		t.Run(status, func(t *testing.T) {
			term := model.SubscriptionTermSegment{
				ContractId:     contract.Id,
				OrderId:        order.Id,
				PlanId:         plan.Id,
				SegmentIndex:   index,
				AllocatedMoney: plan.PriceAmount,
				Status:         status,
			}
			require.NoError(t, model.DB.Create(&term).Error)

			_, err := RefundSubscriptionTermSegment(7602, term.Id)

			require.Error(t, err)
			require.Contains(t, err.Error(), "not_started")
		})
	}
}
