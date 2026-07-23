package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupWalletRenewalRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr := miniredis.RunT(t)
	prevRDB := common.RDB
	prevRedisEnabled := common.RedisEnabled
	common.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	common.RedisEnabled = true
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RDB = prevRDB
		common.RedisEnabled = prevRedisEnabled
	})
	return mr
}

func cacheUserQuota(t *testing.T, userID int, quota int) {
	t.Helper()
	require.NoError(t, common.RedisHSetObj(
		fmt.Sprintf("user:v2:%d", userID),
		&model.UserBase{Id: userID, Quota: quota, Status: common.UserStatusEnabled, Group: "plg"},
		0,
	))
}

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

func TestRefundSubscriptionTermSegmentInvalidatesUserCacheAfterCredit(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	mr := setupWalletRenewalRedis(t)
	insertPurchaseServiceUser(t, 7603, 25)
	plan := insertPurchaseServicePlan(t, 7703, 1, 12, 1200)
	contract := model.UserSubscriptionContract{UserId: 7603, Status: model.SubscriptionContractStatusActive, PaymentMode: model.SubscriptionPaymentModePrepaid, CurrentPlanId: plan.Id}
	require.NoError(t, model.DB.Create(&contract).Error)
	order := model.SubscriptionOrder{UserId: 7603, PlanId: plan.Id, TradeNo: "term-refund-cache", PaymentProvider: model.PaymentProviderBalance, Status: common.TopUpStatusSuccess}
	require.NoError(t, model.DB.Create(&order).Error)
	term := model.SubscriptionTermSegment{ContractId: contract.Id, OrderId: order.Id, PlanId: plan.Id, SegmentIndex: 0, AllocatedMoney: plan.PriceAmount, Status: model.SubscriptionTermStatusNotStarted}
	require.NoError(t, model.DB.Create(&term).Error)
	cacheUserQuota(t, 7603, 25)
	require.True(t, mr.Exists(fmt.Sprintf("user:v2:%d", 7603)))

	_, err := RefundSubscriptionTermSegment(7603, term.Id)

	require.NoError(t, err)
	require.False(t, mr.Exists(fmt.Sprintf("user:v2:%d", 7603)))
}

func TestRefundSubscriptionTermSegmentDoesNotLedgerOrCreditWhenStatusCASLoses(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	insertPurchaseServiceUser(t, 7604, 25)
	plan := insertPurchaseServicePlan(t, 7704, 1, 12, 1200)
	contract := model.UserSubscriptionContract{UserId: 7604, Status: model.SubscriptionContractStatusActive, PaymentMode: model.SubscriptionPaymentModePrepaid, CurrentPlanId: plan.Id}
	require.NoError(t, model.DB.Create(&contract).Error)
	order := model.SubscriptionOrder{UserId: 7604, PlanId: plan.Id, TradeNo: "term-refund-cas", PaymentProvider: model.PaymentProviderBalance, Status: common.TopUpStatusSuccess}
	require.NoError(t, model.DB.Create(&order).Error)
	term := model.SubscriptionTermSegment{ContractId: contract.Id, OrderId: order.Id, PlanId: plan.Id, SegmentIndex: 0, AllocatedMoney: plan.PriceAmount, Status: model.SubscriptionTermStatusNotStarted}
	require.NoError(t, model.DB.Create(&term).Error)

	callbackName := "test:term_refund_cas_loses"
	fired := false
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if fired || tx.Statement == nil || tx.Statement.Table != "subscription_term_segments" {
			return
		}
		fired = true
		require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).
			Model(&model.SubscriptionTermSegment{}).
			Where("id = ?", term.Id).
			Update("status", model.SubscriptionTermStatusActive).Error)
	}))
	t.Cleanup(func() {
		require.NoError(t, model.DB.Callback().Update().Remove(callbackName))
	})

	_, err := RefundSubscriptionTermSegment(7604, term.Id)

	require.Error(t, err)
	var ledgerCount int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).Where("term_segment_id = ?", term.Id).Count(&ledgerCount).Error)
	require.Equal(t, int64(0), ledgerCount)
	var user model.User
	require.NoError(t, model.DB.First(&user, "id = ?", 7604).Error)
	require.Equal(t, 25, user.Quota)
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
