package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func seedWalletRenewalContract(t *testing.T, userID int, quota int, plan model.SubscriptionPlan, periodEnd int64) (model.UserSubscriptionContract, model.UserSubscription) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:       userID,
		Username: fmt.Sprintf("wallet_renewal_user_%s_%d", t.Name(), userID),
		Status:   common.UserStatusEnabled,
		Quota:    quota,
		Group:    "plg",
		AffCode:  fmt.Sprintf("wallet_renewal_aff_%s_%d", t.Name(), userID),
	}).Error)
	contract := model.UserSubscriptionContract{
		UserId:               userID,
		Status:               model.SubscriptionContractStatusActive,
		PaymentMode:          model.SubscriptionPaymentModePrepaid,
		RenewalSource:        model.SubscriptionRenewalSourceWallet,
		RenewalStatus:        model.SubscriptionRenewalStatusEnabled,
		CurrentPlanId:        plan.Id,
		CurrentPeriodStart:   periodEnd - 30*24*3600,
		CurrentPeriodEnd:     periodEnd,
		CurrentEntitlementId: 0,
	}
	require.NoError(t, model.DB.Create(&contract).Error)
	entitlement := model.UserSubscription{
		UserId:        userID,
		PlanId:        plan.Id,
		ContractId:    contract.Id,
		AmountTotal:   plan.TotalAmount,
		StartTime:     contract.CurrentPeriodStart,
		EndTime:       periodEnd,
		AccessEndTime: periodEnd,
		Status:        model.SubscriptionEntitlementStatusActive,
		PaymentMode:   model.SubscriptionPaymentModePrepaid,
		Source:        model.PaymentMethodBalance,
	}
	currentSlot := 1
	entitlement.CurrentSlot = &currentSlot
	require.NoError(t, model.DB.Create(&entitlement).Error)
	require.NoError(t, model.DB.Model(&contract).Updates(map[string]interface{}{"current_entitlement_id": entitlement.Id}).Error)
	contract.CurrentEntitlementId = entitlement.Id
	return contract, entitlement
}

func TestRenewWalletSubscriptionContractChargesCurrentOneMonthPlanAndExtendsOnce(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	plan := insertPurchaseServicePlan(t, 7801, 1, 7, 700)
	periodEnd := common.GetTimestamp() - 15
	contract, oldEntitlement := seedWalletRenewalContract(t, 7901, 1000, plan, periodEnd)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("price_amount", 9).Error)

	result, err := RenewWalletSubscriptionContract(contract.Id)

	require.NoError(t, err)
	require.True(t, result.Renewed)
	require.Equal(t, int64(900), result.ChargedQuota)
	var user model.User
	require.NoError(t, model.DB.First(&user, "id = ?", 7901).Error)
	require.Equal(t, 100, user.Quota)
	var stored model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&stored, "id = ?", contract.Id).Error)
	require.Equal(t, model.SubscriptionRenewalStatusEnabled, stored.RenewalStatus)
	require.Equal(t, periodEnd, stored.CurrentPeriodStart)
	require.Equal(t, time.Unix(periodEnd, 0).AddDate(0, 1, 0).Unix(), stored.CurrentPeriodEnd)
	require.NotEqual(t, oldEntitlement.Id, stored.CurrentEntitlementId)
	var ledgerCount int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).Where("user_id = ? AND entry_type = ?", 7901, model.WalletLedgerEntryTypePrepaidDebit).Count(&ledgerCount).Error)
	require.Equal(t, int64(1), ledgerCount)
	var order model.SubscriptionOrder
	require.NoError(t, model.DB.First(&order, "id = ?", result.OrderID).Error)
	require.NotEmpty(t, order.PlanSnapshot)
	require.Contains(t, order.PlanSnapshot, `"plan_id":7801`)
	require.Contains(t, order.PlanSnapshot, `"price_amount":9`)

	replay, err := RenewWalletSubscriptionContract(contract.Id)
	require.NoError(t, err)
	require.False(t, replay.Renewed)
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).Where("user_id = ? AND entry_type = ?", 7901, model.WalletLedgerEntryTypePrepaidDebit).Count(&ledgerCount).Error)
	require.Equal(t, int64(1), ledgerCount)
}

func TestRenewWalletSubscriptionContractPausesWithoutExtendingOnInsufficientBalance(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	plan := insertPurchaseServicePlan(t, 7802, 1, 7, 700)
	periodEnd := common.GetTimestamp() - 15
	contract, entitlement := seedWalletRenewalContract(t, 7902, 699, plan, periodEnd)

	result, err := RenewWalletSubscriptionContract(contract.Id)

	require.NoError(t, err)
	require.False(t, result.Renewed)
	require.Equal(t, model.SubscriptionRenewalStatusPausedInsufficientBalance, result.PausedStatus)
	var stored model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&stored, "id = ?", contract.Id).Error)
	require.Equal(t, model.SubscriptionRenewalStatusPausedInsufficientBalance, stored.RenewalStatus)
	require.Equal(t, model.SubscriptionContractStatusEnded, stored.Status)
	require.Equal(t, periodEnd, stored.CurrentPeriodEnd)
	require.Equal(t, entitlement.Id, stored.CurrentEntitlementId)
	var storedEntitlement model.UserSubscription
	require.NoError(t, model.DB.First(&storedEntitlement, "id = ?", entitlement.Id).Error)
	require.Equal(t, model.SubscriptionEntitlementStatusHistorical, storedEntitlement.Status)
}

func TestRenewWalletSubscriptionContractPausesWithoutExtendingWhenPlanUnavailable(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	plan := insertPurchaseServicePlan(t, 7805, 1, 7, 700)
	periodEnd := common.GetTimestamp() - 15
	contract, entitlement := seedWalletRenewalContract(t, 7905, 700, plan, periodEnd)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("enabled", false).Error)

	result, err := RenewWalletSubscriptionContract(contract.Id)

	require.NoError(t, err)
	require.False(t, result.Renewed)
	require.Equal(t, model.SubscriptionRenewalStatusPausedPlanUnavailable, result.PausedStatus)
	var stored model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&stored, "id = ?", contract.Id).Error)
	require.Equal(t, model.SubscriptionRenewalStatusPausedPlanUnavailable, stored.RenewalStatus)
	require.Equal(t, model.SubscriptionContractStatusEnded, stored.Status)
	require.Equal(t, periodEnd, stored.CurrentPeriodEnd)
	require.Equal(t, entitlement.Id, stored.CurrentEntitlementId)
}

func TestRunWalletSubscriptionRenewalOnceSkipsFuturePeriodsAndCatchesUpExpiredPeriods(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	plan := insertPurchaseServicePlan(t, 7803, 1, 3, 300)
	now := common.GetTimestamp()
	futureEnd := now + 30
	futureContract, _ := seedWalletRenewalContract(t, 7903, 300, plan, futureEnd)
	expiredEnd := now - 90
	expiredContract, _ := seedWalletRenewalContract(t, 7906, 300, plan, expiredEnd)

	renewed, err := RunWalletSubscriptionRenewalOnce(10)

	require.NoError(t, err)
	require.Equal(t, 1, renewed)
	var futureStored model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&futureStored, "id = ?", futureContract.Id).Error)
	require.Equal(t, futureEnd, futureStored.CurrentPeriodEnd)
	var expiredStored model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&expiredStored, "id = ?", expiredContract.Id).Error)
	require.Greater(t, expiredStored.CurrentPeriodEnd, expiredEnd)
}

func TestRenewWalletSubscriptionContractInvalidatesUserCacheAfterDebit(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	mr := setupWalletRenewalRedis(t)
	plan := insertPurchaseServicePlan(t, 7806, 1, 3, 300)
	periodEnd := common.GetTimestamp() - 15
	contract, _ := seedWalletRenewalContract(t, 7907, 300, plan, periodEnd)
	cacheUserQuota(t, 7907, 300)
	require.True(t, mr.Exists(fmt.Sprintf("user:v2:%d", 7907)))

	_, err := RenewWalletSubscriptionContract(contract.Id)

	require.NoError(t, err)
	require.False(t, mr.Exists(fmt.Sprintf("user:v2:%d", 7907)))
}

func TestHandleExistingWalletRenewalDoesNotQueryAbortedPostgresTransaction(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalUsingSQLite := common.UsingSQLite
	common.UsingPostgreSQL = true
	common.UsingSQLite = false
	t.Cleanup(func() {
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.UsingSQLite = originalUsingSQLite
	})
	originalErr := &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}

	err := handleExistingWalletRenewalTx(model.DB, &model.UserSubscriptionContract{Id: 1}, "missing-key", nil, originalErr)

	require.ErrorIs(t, err, originalErr)
}

func TestRunSubscriptionTermSegmentAdvanceOnceCompletesExpiredActiveAndActivatesDueTerms(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	insertPurchaseServiceUser(t, 7904, 0)
	plan := insertPurchaseServicePlan(t, 7804, 1, 3, 300)
	contract := model.UserSubscriptionContract{UserId: 7904, Status: model.SubscriptionContractStatusActive, PaymentMode: model.SubscriptionPaymentModePrepaid, CurrentPlanId: plan.Id}
	require.NoError(t, model.DB.Create(&contract).Error)
	order := model.SubscriptionOrder{UserId: 7904, PlanId: plan.Id, TradeNo: "advance-term-state", PaymentProvider: model.PaymentProviderBalance, Status: common.TopUpStatusSuccess}
	require.NoError(t, model.DB.Create(&order).Error)
	now := common.GetTimestamp()
	expiredActive := model.SubscriptionTermSegment{ContractId: contract.Id, OrderId: order.Id, PlanId: plan.Id, SegmentIndex: 0, StartTime: now - 7200, EndTime: now - 3600, AllocatedMoney: plan.PriceAmount, Status: model.SubscriptionTermStatusActive}
	dueNotStarted := model.SubscriptionTermSegment{ContractId: contract.Id, OrderId: order.Id, PlanId: plan.Id, SegmentIndex: 1, StartTime: now - 3600, EndTime: now + 3600, AllocatedMoney: plan.PriceAmount, Status: model.SubscriptionTermStatusNotStarted}
	futureNotStarted := model.SubscriptionTermSegment{ContractId: contract.Id, OrderId: order.Id, PlanId: plan.Id, SegmentIndex: 2, StartTime: now + 3600, EndTime: now + 7200, AllocatedMoney: plan.PriceAmount, Status: model.SubscriptionTermStatusNotStarted}
	require.NoError(t, model.DB.Create(&expiredActive).Error)
	require.NoError(t, model.DB.Create(&dueNotStarted).Error)
	require.NoError(t, model.DB.Create(&futureNotStarted).Error)

	advanced, err := RunSubscriptionTermSegmentAdvanceOnce(10)

	require.NoError(t, err)
	require.Equal(t, 2, advanced)
	var terms []model.SubscriptionTermSegment
	require.NoError(t, model.DB.Where("order_id = ?", order.Id).Order("segment_index asc").Find(&terms).Error)
	require.Equal(t, subscriptionTermStatusCompleted, terms[0].Status)
	require.Equal(t, model.SubscriptionTermStatusActive, terms[1].Status)
	require.Equal(t, model.SubscriptionTermStatusNotStarted, terms[2].Status)
}
