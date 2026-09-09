package service

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type walletRenewalTxOptionsRecorder struct {
	*sql.DB
	options *sql.TxOptions
}

func (recorder *walletRenewalTxOptionsRecorder) BeginTx(ctx context.Context, options *sql.TxOptions) (*sql.Tx, error) {
	if options != nil {
		copied := *options
		recorder.options = &copied
	}
	return recorder.DB.BeginTx(ctx, options)
}

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

func seedWalletRenewalSourceOrder(t *testing.T, contract model.UserSubscriptionContract, entitlement *model.UserSubscription, plan model.SubscriptionPlan) model.SubscriptionOrder {
	t.Helper()
	tradeNo := fmt.Sprintf("WALLETSOURCE%d", contract.Id)
	grantKey := "prepaid:" + tradeNo
	require.NoError(t, model.DB.Model(entitlement).Where("id = ?", entitlement.Id).Update("grant_key", grantKey).Error)
	entitlement.GrantKey = &grantKey
	planSnapshot, err := subscriptionPurchasePlanSnapshot(&plan)
	require.NoError(t, err)
	order := model.SubscriptionOrder{
		UserId:          contract.UserId,
		PlanId:          plan.Id,
		Money:           plan.PriceAmount,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodBalance,
		PaymentProvider: model.PaymentProviderBalance,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      entitlement.StartTime,
		CompleteTime:    entitlement.StartTime,
		PurchaseMonths:  1,
		UnitPrice:       plan.PriceAmount,
		PaymentCurrency: plan.Currency,
		PlanSnapshot:    planSnapshot,
		ProviderPayload: fmt.Sprintf("charged_quota=700;contract_id=%d", contract.Id),
		RenewalSource:   model.SubscriptionRenewalSourceWallet,
	}
	require.NoError(t, model.DB.Create(&order).Error)
	return order
}

func seedReachedWalletCatalogMigration(t *testing.T, contract *model.UserSubscriptionContract, targetPlanID int, rawSnapshot string) model.SubscriptionChangeIntent {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.SubscriptionCatalogMigrationBatch{}))
	t.Setenv("FLATKEY_DEPLOYMENT_ENV", "staging")
	t.Setenv("K_SERVICE", "newapi-staging")
	t.Setenv("SUBSCRIPTION_CATALOG_MIGRATION_ENABLED", "true")
	t.Setenv("SUBSCRIPTION_CATALOG_MIGRATION_CONTRACT_ALLOWLIST", fmt.Sprint(contract.Id))
	batch := model.SubscriptionCatalogMigrationBatch{
		Id: "catmig_wallet_test", RequestId: "wallet-test-batch", CohortDigest: fmt.Sprintf("%064d", 1),
		Status: model.SubscriptionCatalogMigrationBatchStatusScheduled, ManifestSnapshot: "{}", RequestedBy: 1,
		DeploymentEnvironment: "staging", ServiceName: "newapi-staging", SandboxOnly: true,
	}
	require.NoError(t, model.DB.Create(&batch).Error)
	batchID := batch.Id
	intent := model.SubscriptionChangeIntent{
		ContractId: contract.Id, UserId: contract.UserId, RequestId: "wallet-catalog-intent", ChangeVersion: contract.ChangeVersion,
		Kind: model.SubscriptionChangeIntentKindCatalogMigration, PaymentMode: contract.PaymentMode,
		Status: model.SubscriptionChangeIntentStatusScheduled, FromPlanId: contract.CurrentPlanId, ToPlanId: targetPlanID,
		CatalogMigrationBatchId: &batchID, TargetPlanSnapshot: rawSnapshot, EffectiveAt: contract.CurrentPeriodEnd,
	}
	require.NoError(t, model.DB.Create(&intent).Error)
	require.NoError(t, model.DB.Model(&model.UserSubscriptionContract{}).Where("id = ?", contract.Id).Updates(map[string]interface{}{
		"latest_change_intent_id": intent.Id,
		"pending_plan_id":         targetPlanID,
		"pending_effective_at":    contract.CurrentPeriodEnd,
	}).Error)
	contract.LatestChangeIntentId = intent.Id
	contract.PendingPlanId = targetPlanID
	contract.PendingEffectiveAt = contract.CurrentPeriodEnd
	return intent
}

func walletCatalogSnapshot(t *testing.T, planID int) string {
	t.Helper()
	raw, err := EncodeRecurringPlanSnapshotV1(RecurringPlanSnapshotV1{
		Version: RecurringPlanSnapshotVersionV1, PlanID: planID, StripePriceID: "price_wallet_catalog",
		Currency: "USD", BasePriceMinor: 1000, DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1,
		QuotaResetPeriod: model.SubscriptionResetNever, TotalAmount: 13, MediaCreditsMonthly: 7,
		Window5hAmount: 0, WindowWeekAmount: 0, UpgradeGroup: "catalog_target",
	})
	require.NoError(t, err)
	return raw
}

func configureWalletCatalogTarget(t *testing.T, plan *model.SubscriptionPlan) {
	t.Helper()
	plan.PriceAmount = 10
	plan.Currency = "USD"
	plan.StripePriceId = "price_wallet_catalog"
	plan.DurationUnit = model.SubscriptionDurationMonth
	plan.DurationValue = 1
	plan.CustomSeconds = 0
	plan.QuotaResetPeriod = model.SubscriptionResetNever
	plan.QuotaResetCustomSeconds = 0
	plan.TotalAmount = 13
	plan.MediaCreditsMonthly = 7
	plan.Window5hAmount = 0
	plan.WindowWeekAmount = 0
	plan.UpgradeGroup = "catalog_target"
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"price_amount": plan.PriceAmount, "currency": plan.Currency, "stripe_price_id": plan.StripePriceId,
		"duration_unit": plan.DurationUnit, "duration_value": plan.DurationValue, "custom_seconds": plan.CustomSeconds,
		"quota_reset_period": plan.QuotaResetPeriod, "quota_reset_custom_seconds": plan.QuotaResetCustomSeconds,
		"total_amount": plan.TotalAmount, "media_credits_monthly": plan.MediaCreditsMonthly,
		"window_5h_amount": plan.Window5hAmount, "window_week_amount": plan.WindowWeekAmount,
		"upgrade_group": plan.UpgradeGroup,
	}).Error)
}

func assertWalletCatalogRenewalUnchanged(t *testing.T, contract model.UserSubscriptionContract, entitlement model.UserSubscription, quota int) {
	t.Helper()
	var storedContract model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&storedContract, "id = ?", contract.Id).Error)
	require.Equal(t, contract.CurrentPlanId, storedContract.CurrentPlanId)
	require.Equal(t, contract.CurrentPeriodEnd, storedContract.CurrentPeriodEnd)
	require.Equal(t, entitlement.Id, storedContract.CurrentEntitlementId)
	var storedEntitlement model.UserSubscription
	require.NoError(t, model.DB.First(&storedEntitlement, "id = ?", entitlement.Id).Error)
	require.Equal(t, model.SubscriptionEntitlementStatusActive, storedEntitlement.Status)
	require.NotNil(t, storedEntitlement.CurrentSlot)
	var user model.User
	require.NoError(t, model.DB.First(&user, "id = ?", contract.UserId).Error)
	require.Equal(t, quota, user.Quota)
	var count int64
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("user_id = ? AND trade_no LIKE ?", contract.UserId, "SUBRENEW%").Count(&count).Error)
	require.Zero(t, count)
}

func TestRenewWalletSubscriptionContractAppliesReachedCatalogSnapshotAtomically(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	source := insertPurchaseServicePlan(t, 7830, 1, 7, 700)
	periodEnd := common.GetTimestamp() - 15
	contract, oldEntitlement := seedWalletRenewalContract(t, 7930, 1500, source, periodEnd)
	sourceOrder := seedWalletRenewalSourceOrder(t, contract, &oldEntitlement, source)
	target := insertPurchaseServicePlan(t, 7831, 2, 99, 999)
	configureWalletCatalogTarget(t, &target)
	intent := seedReachedWalletCatalogMigration(t, &contract, target.Id, walletCatalogSnapshot(t, target.Id))
	assertWalletCatalogRenewalUnchanged(t, contract, oldEntitlement, 1500)

	result, err := RenewWalletSubscriptionContract(contract.Id)

	require.NoError(t, err)
	require.True(t, result.Renewed)
	require.Equal(t, int64(1000), result.ChargedQuota)
	var stored model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&stored, "id = ?", contract.Id).Error)
	require.Equal(t, target.Id, stored.CurrentPlanId)
	require.Equal(t, intent.Id, stored.LatestChangeIntentId)
	require.Zero(t, stored.PendingPlanId)
	require.Zero(t, stored.PendingEffectiveAt)
	var renewed model.UserSubscription
	require.NoError(t, model.DB.First(&renewed, "id = ?", stored.CurrentEntitlementId).Error)
	require.Equal(t, int64(13), renewed.AmountTotal)
	require.Equal(t, int64(7), renewed.MediaCreditsTotal)
	require.Equal(t, int64(0), *renewed.Window5hAmount)
	require.Equal(t, int64(0), *renewed.WindowWeekAmount)
	var historical model.UserSubscription
	require.NoError(t, model.DB.First(&historical, "id = ?", oldEntitlement.Id).Error)
	require.Equal(t, model.SubscriptionEntitlementStatusHistorical, historical.Status)
	var applied model.SubscriptionChangeIntent
	require.NoError(t, model.DB.First(&applied, "id = ?", intent.Id).Error)
	require.Equal(t, model.SubscriptionChangeIntentStatusApplied, applied.Status)
	require.NotEmpty(t, applied.WalletDebitTradeNo)
	var renewalOrder model.SubscriptionOrder
	require.NoError(t, model.DB.First(&renewalOrder, "id = ?", result.OrderID).Error)
	require.Equal(t, target.Id, renewalOrder.PlanId)
	require.Equal(t, float64(10), renewalOrder.Money)
	require.Equal(t, float64(10), renewalOrder.UnitPrice)
	require.Equal(t, "USD", renewalOrder.PaymentCurrency)
	require.Contains(t, renewalOrder.PlanSnapshot, `"plan_id":7831`)
	require.Contains(t, renewalOrder.PlanSnapshot, `"price_amount":10`)
	require.Contains(t, renewalOrder.PlanSnapshot, `"total_amount":13`)
	require.Contains(t, renewalOrder.PlanSnapshot, `"media_credits_monthly":7`)
	require.Contains(t, renewalOrder.PlanSnapshot, `"window_5h_amount":0`)
	require.Contains(t, renewalOrder.PlanSnapshot, `"window_week_amount":0`)
	require.Contains(t, renewalOrder.PlanSnapshot, `"upgrade_group":"catalog_target"`)
	var unchangedSourceOrder model.SubscriptionOrder
	require.NoError(t, model.DB.First(&unchangedSourceOrder, "id = ?", sourceOrder.Id).Error)
	require.Equal(t, sourceOrder.Status, unchangedSourceOrder.Status)
	require.Equal(t, sourceOrder.PlanSnapshot, unchangedSourceOrder.PlanSnapshot)
	require.Equal(t, sourceOrder.TradeNo, unchangedSourceOrder.TradeNo)
	var user model.User
	require.NoError(t, model.DB.First(&user, "id = ?", contract.UserId).Error)
	require.Equal(t, 500, user.Quota)

	replay, err := RenewWalletSubscriptionContract(contract.Id)
	require.NoError(t, err)
	require.False(t, replay.Renewed)
	var ledgerCount, entitlementCount int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).Where("user_id = ?", contract.UserId).Count(&ledgerCount).Error)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("contract_id = ?", contract.Id).Count(&entitlementCount).Error)
	require.Equal(t, int64(1), ledgerCount)
	require.Equal(t, int64(2), entitlementCount)
}

func TestRenewWalletSubscriptionContractCatalogFailuresNeverFallback(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, intent model.SubscriptionChangeIntent)
	}{
		{name: "wrong snapshot", mutate: func(t *testing.T, intent model.SubscriptionChangeIntent) {
			require.NoError(t, model.DB.Model(&model.SubscriptionChangeIntent{}).Where("id = ?", intent.Id).Update("target_plan_snapshot", `{"version":1}`).Error)
		}},
		{name: "guard off", mutate: func(t *testing.T, _ model.SubscriptionChangeIntent) {
			t.Setenv("SUBSCRIPTION_CATALOG_MIGRATION_ENABLED", "false")
		}},
		{name: "sandbox drift", mutate: func(t *testing.T, intent model.SubscriptionChangeIntent) {
			require.NotNil(t, intent.CatalogMigrationBatchId)
			require.NoError(t, model.DB.Exec("UPDATE subscription_catalog_migration_batches SET service_name = ? WHERE id = ?", "unexpected-service", *intent.CatalogMigrationBatchId).Error)
		}},
		{name: "plan drift", mutate: func(t *testing.T, intent model.SubscriptionChangeIntent) {
			require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", intent.ToPlanId).Update("price_amount", 11).Error)
		}},
		{name: "balance payment disabled", mutate: func(t *testing.T, intent model.SubscriptionChangeIntent) {
			require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", intent.ToPlanId).Update("allow_balance_pay", false).Error)
		}},
		{name: "provider binding present", mutate: func(t *testing.T, intent model.SubscriptionChangeIntent) {
			require.NoError(t, model.DB.Model(&model.UserSubscriptionContract{}).Where("id = ?", intent.ContractId).Update("current_provider_binding_id", 99).Error)
		}},
		{name: "external payment mode", mutate: func(t *testing.T, intent model.SubscriptionChangeIntent) {
			require.NoError(t, model.DB.Model(&model.UserSubscriptionContract{}).Where("id = ?", intent.ContractId).Update("payment_mode", model.SubscriptionPaymentModeExternalOnePeriod).Error)
		}},
		{name: "needs attention", mutate: func(t *testing.T, intent model.SubscriptionChangeIntent) {
			require.NoError(t, model.DB.Model(&model.SubscriptionChangeIntent{}).Where("id = ?", intent.Id).Update("status", model.SubscriptionChangeIntentStatusNeedsAttention).Error)
		}},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupSubscriptionPurchaseServiceTestDB(t)
			source := insertPurchaseServicePlan(t, 7840+index*2, 1, 7, 700)
			periodEnd := common.GetTimestamp() - 15
			contract, entitlement := seedWalletRenewalContract(t, 7940+index, 1500, source, periodEnd)
			target := insertPurchaseServicePlan(t, 7841+index*2, 2, 99, 999)
			configureWalletCatalogTarget(t, &target)
			intent := seedReachedWalletCatalogMigration(t, &contract, target.Id, walletCatalogSnapshot(t, target.Id))
			test.mutate(t, intent)

			result, err := RenewWalletSubscriptionContract(contract.Id)

			require.Nil(t, result)
			require.Error(t, err)
			assertWalletCatalogRenewalUnchanged(t, contract, entitlement, 1500)
		})
	}
}

func TestRunWalletSubscriptionRenewalOnceRetriesCatalogAfterBalanceTopUp(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	source := insertPurchaseServicePlan(t, 7850, 1, 7, 700)
	periodEnd := common.GetTimestamp() - 15
	contract, oldEntitlement := seedWalletRenewalContract(t, 7950, 999, source, periodEnd)
	target := insertPurchaseServicePlan(t, 7851, 2, 99, 999)
	configureWalletCatalogTarget(t, &target)
	intent := seedReachedWalletCatalogMigration(t, &contract, target.Id, walletCatalogSnapshot(t, target.Id))

	result, err := RenewWalletSubscriptionContract(contract.Id)
	require.NoError(t, err)
	require.False(t, result.Renewed)
	require.Equal(t, model.SubscriptionRenewalStatusPausedInsufficientBalance, result.PausedStatus)
	assertWalletCatalogRenewalUnchanged(t, contract, oldEntitlement, 999)
	var pending model.SubscriptionChangeIntent
	require.NoError(t, model.DB.First(&pending, "id = ?", intent.Id).Error)
	require.Equal(t, model.SubscriptionChangeIntentStatusScheduled, pending.Status)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", contract.UserId).Update("quota", 1000).Error)

	renewed, err := RunWalletSubscriptionRenewalOnce(10)
	require.NoError(t, err)
	require.Equal(t, 1, renewed)
	var user model.User
	require.NoError(t, model.DB.First(&user, "id = ?", contract.UserId).Error)
	require.Zero(t, user.Quota)
	var ledgerCount int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).Where("user_id = ?", contract.UserId).Count(&ledgerCount).Error)
	require.Equal(t, int64(1), ledgerCount)
}

func TestRenewWalletSubscriptionContractConcurrentCatalogReplayChargesAndGrantsOnce(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	source := insertPurchaseServicePlan(t, 7860, 1, 7, 700)
	periodEnd := common.GetTimestamp() - 15
	contract, _ := seedWalletRenewalContract(t, 7960, 1000, source, periodEnd)
	target := insertPurchaseServicePlan(t, 7861, 2, 99, 999)
	configureWalletCatalogTarget(t, &target)
	seedReachedWalletCatalogMigration(t, &contract, target.Id, walletCatalogSnapshot(t, target.Id))

	results := make([]*WalletSubscriptionRenewalResult, 2)
	errs := make([]error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	start := make(chan struct{})
	var calls sync.WaitGroup
	calls.Add(2)
	for index := range results {
		go func(index int) {
			defer calls.Done()
			ready.Done()
			<-start
			results[index], errs[index] = RenewWalletSubscriptionContract(contract.Id)
		}(index)
	}
	ready.Wait()
	close(start)
	calls.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	renewed := 0
	for _, result := range results {
		require.NotNil(t, result)
		if result.Renewed {
			renewed++
		}
	}
	require.Equal(t, 1, renewed)
	var user model.User
	require.NoError(t, model.DB.First(&user, "id = ?", contract.UserId).Error)
	require.Zero(t, user.Quota)
	var ledgerCount, orderCount, entitlementCount int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).Where("user_id = ?", contract.UserId).Count(&ledgerCount).Error)
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("user_id = ?", contract.UserId).Count(&orderCount).Error)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("contract_id = ?", contract.Id).Count(&entitlementCount).Error)
	require.Equal(t, int64(1), ledgerCount)
	require.Equal(t, int64(1), orderCount)
	require.Equal(t, int64(2), entitlementCount)
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
	require.Contains(t, order.PlanSnapshot, `"media_credits_monthly":25`)
	var entitlement model.UserSubscription
	require.NoError(t, model.DB.First(&entitlement, "id = ?", stored.CurrentEntitlementId).Error)
	require.Equal(t, int64(25), entitlement.MediaCreditsTotal)
	require.Zero(t, entitlement.MediaCreditsUsed)

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

func TestRenewWalletSubscriptionContractDoesNotPersistSuccessFactsWhenConditionalDebitLoses(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	plan := insertPurchaseServicePlan(t, 7807, 1, 7, 700)
	periodEnd := common.GetTimestamp() - 15
	contract, entitlement := seedWalletRenewalContract(t, 7908, 700, plan, periodEnd)
	renewalKey := walletRenewalKey(contract.Id, periodEnd, plan.Id)
	tradeNo := walletRenewalTradeNo(contract.Id, periodEnd, plan.Id)

	callbackName := "test:wallet_renewal_conditional_debit_loses"
	fired := false
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if fired || tx.Statement == nil || tx.Statement.Table != "users" {
			return
		}
		fired = true
		require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).
			Model(&model.User{}).
			Where("id = ?", contract.UserId).
			Update("quota", 0).Error)
	}))
	t.Cleanup(func() {
		require.NoError(t, model.DB.Callback().Update().Remove(callbackName))
	})

	result, err := RenewWalletSubscriptionContract(contract.Id)

	require.NoError(t, err)
	require.True(t, fired)
	require.False(t, result.Renewed)
	require.Equal(t, model.SubscriptionRenewalStatusPausedInsufficientBalance, result.PausedStatus)
	var orderCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("trade_no = ?", tradeNo).Count(&orderCount).Error)
	require.Zero(t, orderCount)
	var ledgerCount int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).Where("entry_key = ?", renewalKey).Count(&ledgerCount).Error)
	require.Zero(t, ledgerCount)
	var renewalEntitlementCount int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("grant_key = ?", renewalKey).Count(&renewalEntitlementCount).Error)
	require.Zero(t, renewalEntitlementCount)
	var storedContract model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&storedContract, "id = ?", contract.Id).Error)
	require.Equal(t, model.SubscriptionContractStatusEnded, storedContract.Status)
	var storedEntitlement model.UserSubscription
	require.NoError(t, model.DB.First(&storedEntitlement, "id = ?", entitlement.Id).Error)
	require.Equal(t, model.SubscriptionEntitlementStatusHistorical, storedEntitlement.Status)
}

func TestRenewWalletSubscriptionContractRenewsRetiredCurrentPlanWithEntitlementLimits(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	plan := insertPurchaseServicePlan(t, 7805, 1, 7, 700)
	plan.UpgradeGroup = "legacy_group"
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("upgrade_group", plan.UpgradeGroup).Error)
	periodEnd := common.GetTimestamp() - 15
	contract, oldEntitlement := seedWalletRenewalContract(t, 7905, 700, plan, periodEnd)
	sourceOrder := seedWalletRenewalSourceOrder(t, contract, &oldEntitlement, plan)
	var sourceSnapshot purchasePlanSnapshot
	require.NoError(t, common.Unmarshal([]byte(sourceOrder.PlanSnapshot), &sourceSnapshot))
	sourceSnapshot.PriceAmount = 7.001
	sourceSnapshotJSON, err := common.Marshal(sourceSnapshot)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("id = ?", sourceOrder.Id).Update("plan_snapshot", string(sourceSnapshotJSON)).Error)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("id = ?", oldEntitlement.Id).Updates(map[string]interface{}{
		"media_credits_total": plan.MediaCreditsMonthly,
		"window_5h_amount":    plan.Window5hAmount,
		"window_week_amount":  plan.WindowWeekAmount,
		"upgrade_group":       plan.UpgradeGroup,
	}).Error)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"enabled":               false,
		"price_amount":          13,
		"currency":              "EUR",
		"allow_balance_pay":     false,
		"total_amount":          1300,
		"window_5h_amount":      0,
		"window_week_amount":    0,
		"media_credits_monthly": 99,
		"upgrade_group":         "new_group",
	}).Error)

	result, err := RenewWalletSubscriptionContract(contract.Id)

	require.NoError(t, err)
	require.True(t, result.Renewed)
	require.Equal(t, int64(700), result.ChargedQuota)
	var stored model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&stored, "id = ?", contract.Id).Error)
	require.Equal(t, model.SubscriptionRenewalStatusEnabled, stored.RenewalStatus)
	require.Equal(t, model.SubscriptionContractStatusActive, stored.Status)
	require.NotEqual(t, oldEntitlement.Id, stored.CurrentEntitlementId)
	var renewed model.UserSubscription
	require.NoError(t, model.DB.First(&renewed, "id = ?", stored.CurrentEntitlementId).Error)
	require.Equal(t, int64(700), renewed.AmountTotal)
	require.Equal(t, int64(25), renewed.MediaCreditsTotal)
	require.Equal(t, int64(50), *renewed.Window5hAmount)
	require.Equal(t, int64(500), *renewed.WindowWeekAmount)
	require.Equal(t, "legacy_group", renewed.UpgradeGroup)
	var order model.SubscriptionOrder
	require.NoError(t, model.DB.First(&order, "id = ?", result.OrderID).Error)
	require.Contains(t, order.PlanSnapshot, `"total_amount":700`)
	require.Contains(t, order.PlanSnapshot, `"window_5h_amount":50`)
	require.Contains(t, order.PlanSnapshot, `"window_week_amount":500`)
	require.Contains(t, order.PlanSnapshot, `"media_credits_monthly":25`)
	require.Contains(t, order.PlanSnapshot, `"upgrade_group":"legacy_group"`)
	require.Contains(t, order.PlanSnapshot, `"price_amount":7`)
	require.Contains(t, order.PlanSnapshot, `"currency":"USD"`)
	require.Equal(t, float64(7), order.UnitPrice)
	require.Equal(t, "USD", order.PaymentCurrency)
	var ledger model.WalletLedgerEntry
	require.NoError(t, model.DB.First(&ledger, "order_id = ?", result.OrderID).Error)
	require.Equal(t, float64(7), ledger.MoneyAmount)
}

func TestRenewWalletSubscriptionContractRestoresLegacyNilWindowsFromSourceOrderSnapshot(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	plan := insertPurchaseServicePlan(t, 7821, 1, 7, 700)
	periodEnd := common.GetTimestamp() - 15
	contract, oldEntitlement := seedWalletRenewalContract(t, 7921, 700, plan, periodEnd)
	seedWalletRenewalSourceOrder(t, contract, &oldEntitlement, plan)
	require.Nil(t, oldEntitlement.Window5hAmount)
	require.Nil(t, oldEntitlement.WindowWeekAmount)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"enabled":            false,
		"price_amount":       13,
		"window_5h_amount":   0,
		"window_week_amount": 0,
	}).Error)

	result, err := RenewWalletSubscriptionContract(contract.Id)

	require.NoError(t, err)
	require.True(t, result.Renewed)
	require.Equal(t, int64(700), result.ChargedQuota)
	var stored model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&stored, "id = ?", contract.Id).Error)
	var renewed model.UserSubscription
	require.NoError(t, model.DB.First(&renewed, "id = ?", stored.CurrentEntitlementId).Error)
	require.Equal(t, int64(50), *renewed.Window5hAmount)
	require.Equal(t, int64(500), *renewed.WindowWeekAmount)
}

func TestRenewWalletSubscriptionContractPausesRetiredPlanWithoutTrustedSourceOrder(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	plan := insertPurchaseServicePlan(t, 7822, 1, 7, 700)
	periodEnd := common.GetTimestamp() - 15
	contract, entitlement := seedWalletRenewalContract(t, 7922, 700, plan, periodEnd)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("enabled", false).Error)

	result, err := RenewWalletSubscriptionContract(contract.Id)

	require.NoError(t, err)
	require.False(t, result.Renewed)
	require.Equal(t, model.SubscriptionRenewalStatusPausedPlanUnavailable, result.PausedStatus)
	var stored model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&stored, "id = ?", contract.Id).Error)
	require.Equal(t, model.SubscriptionContractStatusEnded, stored.Status)
	require.Equal(t, model.SubscriptionRenewalStatusPausedPlanUnavailable, stored.RenewalStatus)
	require.Equal(t, periodEnd, stored.CurrentPeriodEnd)
	require.Equal(t, entitlement.Id, stored.CurrentEntitlementId)
	var storedEntitlement model.UserSubscription
	require.NoError(t, model.DB.First(&storedEntitlement, "id = ?", entitlement.Id).Error)
	require.Equal(t, model.SubscriptionEntitlementStatusHistorical, storedEntitlement.Status)
}

func TestRenewWalletSubscriptionContractRejectsRetiredPlanMismatchedWithCurrentEntitlement(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	currentPlan := insertPurchaseServicePlan(t, 7818, 1, 7, 700)
	otherPlan := insertPurchaseServicePlan(t, 7819, 2, 9, 900)
	periodEnd := common.GetTimestamp() - 15
	contract, entitlement := seedWalletRenewalContract(t, 7918, 900, currentPlan, periodEnd)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", otherPlan.Id).Update("enabled", false).Error)
	require.NoError(t, model.DB.Model(&model.UserSubscriptionContract{}).Where("id = ?", contract.Id).Update("current_plan_id", otherPlan.Id).Error)

	result, err := RenewWalletSubscriptionContract(contract.Id)

	require.NoError(t, err)
	require.False(t, result.Renewed)
	require.Equal(t, model.SubscriptionRenewalStatusPausedPlanUnavailable, result.PausedStatus)
	var orderCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("user_id = ?", contract.UserId).Count(&orderCount).Error)
	require.Zero(t, orderCount)
	var ledgerCount int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).Where("user_id = ?", contract.UserId).Count(&ledgerCount).Error)
	require.Zero(t, ledgerCount)
	var storedEntitlement model.UserSubscription
	require.NoError(t, model.DB.First(&storedEntitlement, "id = ?", entitlement.Id).Error)
	require.Equal(t, model.SubscriptionEntitlementStatusHistorical, storedEntitlement.Status)
}

func TestQuoteSubscriptionPurchaseStillRejectsRetiredPlan(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	plan := insertPurchaseServicePlan(t, 7820, 1, 7, 700)
	insertPurchaseServiceUser(t, 7920, 700)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("enabled", false).Error)

	result, err := QuoteSubscriptionPurchase(purchaseBalanceCommand(7920, plan.Id, 1, "retired-plan-purchase"))

	require.Nil(t, result)
	require.ErrorContains(t, err, "subscription plan is disabled")
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
	cached, err := model.GetUserCache(7907)
	require.NoError(t, err)
	require.Equal(t, 300, cached.Quota)
	require.Eventually(t, func() bool {
		return len(mr.Keys()) == 1
	}, time.Second, 10*time.Millisecond)
	userCacheKey := mr.Keys()[0]
	require.Contains(t, userCacheKey, "7907")

	_, err = RenewWalletSubscriptionContract(contract.Id)

	require.NoError(t, err)
	require.False(t, mr.Exists(userCacheKey))
	refreshed, err := model.GetUserCache(7907)
	require.NoError(t, err)
	require.Zero(t, refreshed.Quota)
	require.Eventually(t, func() bool {
		return mr.Exists(userCacheKey)
	}, time.Second, 10*time.Millisecond)
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

func TestRunWalletSubscriptionRenewalOnceRecoversCompletedPostgresDuplicateAndContinues(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	plan := insertPurchaseServicePlan(t, 7808, 1, 7, 700)
	originalPeriodEnd := common.GetTimestamp() - 15
	staleContract, _ := seedWalletRenewalContract(t, 7909, 1400, plan, originalPeriodEnd)

	winner, err := RenewWalletSubscriptionContract(staleContract.Id)
	require.NoError(t, err)
	require.True(t, winner.Renewed)
	var committedContract model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&committedContract, "id = ?", staleContract.Id).Error)
	require.Greater(t, committedContract.CurrentPeriodEnd, originalPeriodEnd)

	followerContract, _ := seedWalletRenewalContract(t, 7910, 700, plan, originalPeriodEnd)
	common.UsingPostgreSQL = true

	callbackName := "test:wallet_renewal_postgres_stale_snapshot"
	batchInjected := false
	contractInjected := false
	require.NoError(t, model.DB.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "user_subscription_contracts" {
			return
		}
		switch destination := tx.Statement.Dest.(type) {
		case *[]model.UserSubscriptionContract:
			if !batchInjected {
				*destination = append([]model.UserSubscriptionContract{staleContract}, (*destination)...)
				batchInjected = true
			}
		case *model.UserSubscriptionContract:
			if batchInjected && !contractInjected && destination.Id == staleContract.Id {
				*destination = staleContract
				contractInjected = true
			}
		}
	}))
	t.Cleanup(func() {
		require.NoError(t, model.DB.Callback().Query().Remove(callbackName))
	})

	renewed, err := RunWalletSubscriptionRenewalOnce(10)

	require.NoError(t, err)
	require.True(t, batchInjected)
	require.True(t, contractInjected)
	require.Equal(t, 1, renewed)
	var winnerUser model.User
	require.NoError(t, model.DB.First(&winnerUser, "id = ?", staleContract.UserId).Error)
	require.Equal(t, 700, winnerUser.Quota)
	var winnerOrderCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("trade_no = ?", walletRenewalTradeNo(staleContract.Id, originalPeriodEnd, plan.Id)).Count(&winnerOrderCount).Error)
	require.Equal(t, int64(1), winnerOrderCount)
	var winnerLedgerCount int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).Where("entry_key = ?", walletRenewalKey(staleContract.Id, originalPeriodEnd, plan.Id)).Count(&winnerLedgerCount).Error)
	require.Equal(t, int64(1), winnerLedgerCount)
	var followerStored model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&followerStored, "id = ?", followerContract.Id).Error)
	require.Greater(t, followerStored.CurrentPeriodEnd, originalPeriodEnd)
}

func TestRenewWalletSubscriptionContractReportsIncompletePostgresDuplicateFacts(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	plan := insertPurchaseServicePlan(t, 7809, 1, 7, 700)
	periodEnd := common.GetTimestamp() - 15
	contract, _ := seedWalletRenewalContract(t, 7911, 700, plan, periodEnd)
	renewalKey := walletRenewalKey(contract.Id, periodEnd, plan.Id)
	tradeNo := walletRenewalTradeNo(contract.Id, periodEnd, plan.Id)
	planSnapshot, err := subscriptionPurchasePlanSnapshot(&plan)
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.SubscriptionOrder{
		UserId:          contract.UserId,
		PlanId:          plan.Id,
		Money:           plan.PriceAmount,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodBalance,
		PaymentProvider: model.PaymentProviderBalance,
		Status:          common.TopUpStatusSuccess,
		PurchaseMonths:  1,
		UnitPrice:       plan.PriceAmount,
		PaymentCurrency: plan.Currency,
		PlanSnapshot:    planSnapshot,
		ProviderPayload: fmt.Sprintf("charged_quota=700;contract_id=%d;renewal_key=%s", contract.Id, renewalKey),
		RenewalSource:   model.SubscriptionRenewalSourceWallet,
	}).Error)
	require.NoError(t, model.DB.Create(&model.UserSubscription{
		UserId:            contract.UserId,
		PlanId:            plan.Id,
		ContractId:        contract.Id,
		ProviderBindingId: 0,
		GrantKey:          &renewalKey,
		AmountTotal:       plan.TotalAmount,
		MediaCreditsTotal: plan.MediaCreditsMonthly,
		Window5hAmount:    common.GetPointer(plan.Window5hAmount),
		WindowWeekAmount:  common.GetPointer(plan.WindowWeekAmount),
		StartTime:         periodEnd,
		EndTime:           time.Unix(periodEnd, 0).AddDate(0, 1, 0).Unix(),
		AccessEndTime:     time.Unix(periodEnd, 0).AddDate(0, 1, 0).Unix(),
		Status:            model.SubscriptionEntitlementStatusHistorical,
		PaymentMode:       model.SubscriptionPaymentModePrepaid,
		Source:            model.PaymentMethodBalance,
		UpgradeGroup:      plan.UpgradeGroup,
	}).Error)
	common.UsingPostgreSQL = true

	_, err = RenewWalletSubscriptionContract(contract.Id)

	require.ErrorContains(t, err, "wallet renewal duplicate facts are incomplete")
	require.ErrorContains(t, err, "debit ledger")
}

func TestRecoverPostgresWalletRenewalDuplicateRejectsGrantMismatchDespiteCorrectLedger(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	plan := insertPurchaseServicePlan(t, 7810, 1, 7, 700)
	plan.MediaCreditsMonthly = 35
	plan.Window5hAmount = 75
	plan.WindowWeekAmount = 650
	plan.UpgradeGroup = "renewal_group"
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"media_credits_monthly": plan.MediaCreditsMonthly,
		"window_5h_amount":      plan.Window5hAmount,
		"window_week_amount":    plan.WindowWeekAmount,
		"upgrade_group":         plan.UpgradeGroup,
	}).Error)
	periodEnd := common.GetTimestamp() - 15
	contract, _ := seedWalletRenewalContract(t, 7912, 700, plan, periodEnd)
	attempt := seedCompletedWalletRenewalDuplicateFacts(t, contract, plan, periodEnd)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).
		Where("grant_key = ?", attempt.RenewalKey).
		Update("amount_total", plan.TotalAmount+1).Error)

	_, err := recoverPostgresWalletRenewalDuplicate(attempt)

	require.ErrorContains(t, err, "wallet renewal duplicate facts are incomplete")
	require.ErrorContains(t, err, "entitlement grant")
}

func TestRecoverPostgresWalletRenewalDuplicateRejectsContractThatOnlyAdvancesPeriodEnd(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	plan := insertPurchaseServicePlan(t, 7811, 1, 7, 700)
	periodEnd := common.GetTimestamp() - 15
	contract, oldEntitlement := seedWalletRenewalContract(t, 7913, 700, plan, periodEnd)
	attempt := seedCompletedWalletRenewalDuplicateFacts(t, contract, plan, periodEnd)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).
		Where("grant_key = ?", attempt.RenewalKey).
		Updates(map[string]interface{}{
			"current_slot": nil,
			"status":       model.SubscriptionEntitlementStatusHistorical,
			"end_reason":   model.SubscriptionEntitlementEndReasonRenewed,
		}).Error)
	require.NoError(t, model.DB.Model(&model.UserSubscriptionContract{}).Where("id = ?", contract.Id).Updates(map[string]interface{}{
		"current_period_start":   contract.CurrentPeriodStart,
		"current_period_end":     time.Unix(attempt.PeriodEnd, 0).AddDate(0, 1, 0).Unix(),
		"current_entitlement_id": oldEntitlement.Id,
	}).Error)

	_, err := recoverPostgresWalletRenewalDuplicate(attempt)

	require.ErrorContains(t, err, "wallet renewal duplicate facts are incomplete")
	require.ErrorContains(t, err, "contract")
}

func TestRecoverPostgresWalletRenewalDuplicateRejectsSamePeriodHistoricalGrantWithoutCurrentSlot(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	plan := insertPurchaseServicePlan(t, 7812, 1, 7, 700)
	periodStart := common.GetTimestamp() - 15
	contract, _ := seedWalletRenewalContract(t, 7914, 700, plan, periodStart)
	attempt := seedCompletedWalletRenewalDuplicateFacts(t, contract, plan, periodStart)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).
		Where("grant_key = ?", attempt.RenewalKey).
		Updates(map[string]interface{}{
			"current_slot": nil,
			"status":       model.SubscriptionEntitlementStatusHistorical,
			"end_reason":   model.SubscriptionEntitlementEndReasonRenewed,
		}).Error)

	_, err := recoverPostgresWalletRenewalDuplicate(attempt)

	require.ErrorContains(t, err, "wallet renewal duplicate facts are incomplete")
	require.ErrorContains(t, err, "entitlement grant")
}

func TestRecoverPostgresWalletRenewalDuplicateAcceptsHistoricalGrantAfterContractAdvances(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	plan := insertPurchaseServicePlan(t, 7813, 1, 7, 700)
	periodStart := common.GetTimestamp() - 15
	contract, _ := seedWalletRenewalContract(t, 7915, 700, plan, periodStart)
	attempt := seedCompletedWalletRenewalDuplicateFacts(t, contract, plan, periodStart)
	nextGrantInput := attempt.GrantInput
	nextGrantInput.GrantKey = walletRenewalKey(contract.Id, attempt.PeriodEnd, plan.Id)
	nextGrantInput.PeriodStart = attempt.PeriodEnd
	nextGrantInput.PeriodEnd = time.Unix(nextGrantInput.PeriodStart, 0).AddDate(0, 1, 0).Unix()
	var nextGrant *model.GrantEntitlementResult
	require.NoError(t, model.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		nextGrant, err = model.RotateCurrentEntitlementTx(tx, nextGrantInput)
		return err
	}))
	var attemptedEntitlement model.UserSubscription
	require.NoError(t, model.DB.First(&attemptedEntitlement, "grant_key = ?", attempt.RenewalKey).Error)
	require.Equal(t, model.SubscriptionEntitlementStatusHistorical, attemptedEntitlement.Status)
	require.Nil(t, attemptedEntitlement.CurrentSlot)

	result, err := recoverPostgresWalletRenewalDuplicate(attempt)

	require.NoError(t, err)
	require.Equal(t, attemptedEntitlement.Id, result.EntitlementID)
	require.NotNil(t, nextGrant)
	require.NotEqual(t, attemptedEntitlement.Id, nextGrant.Entitlement.Id)
}

func TestRecoverPostgresWalletRenewalDuplicateUsesReadOnlyRepeatableReadTransaction(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	plan := insertPurchaseServicePlan(t, 7814, 1, 7, 700)
	periodStart := common.GetTimestamp() - 15
	contract, _ := seedWalletRenewalContract(t, 7916, 700, plan, periodStart)
	attempt := seedCompletedWalletRenewalDuplicateFacts(t, contract, plan, periodStart)
	sqlDB, err := model.DB.DB()
	require.NoError(t, err)
	recorder := &walletRenewalTxOptionsRecorder{DB: sqlDB}
	originalConnPool := model.DB.Statement.ConnPool
	model.DB.Statement.ConnPool = recorder
	t.Cleanup(func() {
		model.DB.Statement.ConnPool = originalConnPool
	})

	_, err = recoverPostgresWalletRenewalDuplicate(attempt)

	require.NoError(t, err)
	require.NotNil(t, recorder.options)
	require.True(t, recorder.options.ReadOnly)
	require.Equal(t, sql.LevelRepeatableRead, recorder.options.Isolation)
}

func TestRecoverPostgresWalletRenewalDuplicateRejectsContractBehindAttemptPeriodWithCompleteWinnerFacts(t *testing.T) {
	setupSubscriptionPurchaseServiceTestDB(t)
	plan := insertPurchaseServicePlan(t, 7815, 1, 7, 700)
	periodStart := common.GetTimestamp() - 15
	contract, _ := seedWalletRenewalContract(t, 7917, 700, plan, periodStart)
	attempt := seedCompletedWalletRenewalDuplicateFacts(t, contract, plan, periodStart)
	require.NoError(t, model.DB.Model(&model.UserSubscriptionContract{}).
		Where("id = ?", contract.Id).
		Update("current_period_end", attempt.PeriodEnd-1).Error)

	_, err := recoverPostgresWalletRenewalDuplicate(attempt)

	require.ErrorContains(t, err, "wallet renewal duplicate facts are incomplete")
	require.ErrorContains(t, err, "was not advanced through")
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

func seedCompletedWalletRenewalDuplicateFacts(t *testing.T, contract model.UserSubscriptionContract, plan model.SubscriptionPlan, periodStart int64) walletRenewalAttempt {
	t.Helper()
	renewalKey := walletRenewalKey(contract.Id, periodStart, plan.Id)
	tradeNo := walletRenewalTradeNo(contract.Id, periodStart, plan.Id)
	periodEnd := time.Unix(periodStart, 0).AddDate(0, 1, 0).Unix()
	planSnapshot, err := subscriptionPurchasePlanSnapshot(&plan)
	require.NoError(t, err)
	attempt := walletRenewalAttempt{
		ContractID:      contract.Id,
		UserID:          contract.UserId,
		PlanID:          plan.Id,
		PeriodStart:     periodStart,
		PeriodEnd:       periodEnd,
		RenewalKey:      renewalKey,
		TradeNo:         tradeNo,
		RequiredQuota:   700,
		PriceAmount:     plan.PriceAmount,
		PaymentCurrency: plan.Currency,
		PlanSnapshot:    planSnapshot,
		GrantInput: model.GrantEntitlementInput{
			ContractId:           contract.Id,
			UserId:               contract.UserId,
			PlanId:               plan.Id,
			ProviderBindingId:    0,
			GrantKey:             renewalKey,
			PaymentMode:          model.SubscriptionPaymentModePrepaid,
			AmountTotal:          plan.TotalAmount,
			MediaCreditsTotal:    plan.MediaCreditsMonthly,
			Window5hAmount:       common.GetPointer(plan.Window5hAmount),
			WindowWeekAmount:     common.GetPointer(plan.WindowWeekAmount),
			UpgradeGroup:         common.GetPointer(plan.UpgradeGroup),
			PeriodStart:          periodStart,
			PeriodEnd:            periodEnd,
			EndReasonForPrevious: model.SubscriptionEntitlementEndReasonRenewed,
			Source:               model.PaymentMethodBalance,
		},
	}
	require.NoError(t, model.DB.Create(&model.SubscriptionOrder{
		UserId:          contract.UserId,
		PlanId:          plan.Id,
		Money:           plan.PriceAmount,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodBalance,
		PaymentProvider: model.PaymentProviderBalance,
		Status:          common.TopUpStatusSuccess,
		PurchaseMonths:  1,
		UnitPrice:       plan.PriceAmount,
		PaymentCurrency: plan.Currency,
		PlanSnapshot:    planSnapshot,
		ProviderPayload: fmt.Sprintf("charged_quota=700;contract_id=%d;renewal_key=%s", contract.Id, renewalKey),
		RenewalSource:   model.SubscriptionRenewalSourceWallet,
	}).Error)
	var order model.SubscriptionOrder
	require.NoError(t, model.DB.First(&order, "trade_no = ?", tradeNo).Error)
	require.NoError(t, model.DB.Create(&model.WalletLedgerEntry{
		UserId:      contract.UserId,
		EntryKey:    renewalKey,
		QuotaDelta:  -700,
		MoneyAmount: plan.PriceAmount,
		EntryType:   model.WalletLedgerEntryTypePrepaidDebit,
		OrderId:     order.Id,
	}).Error)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).
		Where("contract_id = ? AND current_slot = ?", contract.Id, 1).
		Updates(map[string]interface{}{
			"current_slot": nil,
			"status":       model.SubscriptionEntitlementStatusHistorical,
			"end_reason":   model.SubscriptionEntitlementEndReasonRenewed,
		}).Error)
	currentSlot := 1
	require.NoError(t, model.DB.Create(&model.UserSubscription{
		UserId:            contract.UserId,
		PlanId:            plan.Id,
		ContractId:        contract.Id,
		ProviderBindingId: 0,
		GrantKey:          &renewalKey,
		CurrentSlot:       &currentSlot,
		AmountTotal:       plan.TotalAmount,
		MediaCreditsTotal: plan.MediaCreditsMonthly,
		Window5hAmount:    common.GetPointer(plan.Window5hAmount),
		WindowWeekAmount:  common.GetPointer(plan.WindowWeekAmount),
		StartTime:         periodStart,
		EndTime:           periodEnd,
		AccessEndTime:     periodEnd,
		Status:            model.SubscriptionEntitlementStatusActive,
		Source:            model.PaymentMethodBalance,
		PaymentMode:       model.SubscriptionPaymentModePrepaid,
		UpgradeGroup:      plan.UpgradeGroup,
	}).Error)
	var entitlement model.UserSubscription
	require.NoError(t, model.DB.First(&entitlement, "grant_key = ?", renewalKey).Error)
	require.NoError(t, model.DB.Model(&model.UserSubscriptionContract{}).Where("id = ?", contract.Id).Updates(map[string]interface{}{
		"current_period_start":   periodStart,
		"current_period_end":     periodEnd,
		"current_entitlement_id": entitlement.Id,
		"current_plan_id":        plan.Id,
		"payment_mode":           model.SubscriptionPaymentModePrepaid,
	}).Error)
	return attempt
}
