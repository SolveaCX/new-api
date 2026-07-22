package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSubscriptionContractServiceTestDB(t *testing.T) {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedisEnabled := common.RedisEnabled
	originalUsingSQLite := common.UsingSQLite
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalUsingMySQL := common.UsingMySQL
	originalQuotaPerUnit := common.QuotaPerUnit

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.UsingMySQL = false
	common.QuotaPerUnit = 100

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedisEnabled
		common.UsingSQLite = originalUsingSQLite
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.UsingMySQL = originalUsingMySQL
		common.QuotaPerUnit = originalQuotaPerUnit
		require.NoError(t, sqlDB.Close())
	})

	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Log{},
		&model.SubscriptionPlan{},
		&model.SubscriptionOrder{},
		&model.UserSubscription{},
		&model.SubscriptionProviderBinding{},
		&model.UserSubscriptionContract{},
		&model.SubscriptionChangeIntent{},
	))
}

func insertContractServiceUser(t *testing.T, id int, quota int) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:       id,
		Username: "contract_user_" + t.Name(),
		Status:   common.UserStatusEnabled,
		Quota:    quota,
		Group:    "plg",
		AffCode:  "contract_aff_" + t.Name(),
	}).Error)
}

func insertContractServicePlan(t *testing.T, id int, rank int, price float64, total int64) model.SubscriptionPlan {
	t.Helper()
	plan := model.SubscriptionPlan{
		Id:              id,
		Title:           "Contract Plan",
		PriceAmount:     price,
		Currency:        "USD",
		DurationUnit:    model.SubscriptionDurationMonth,
		DurationValue:   1,
		Enabled:         true,
		TierRank:        &rank,
		AllowBalancePay: common.GetPointer(true),
		TotalAmount:     total,
	}
	require.NoError(t, model.DB.Create(&plan).Error)
	return plan
}

func balanceChangeCommand(userID int, planID int, requestID string) ChangePlanCommand {
	return ChangePlanCommand{
		UserID:      userID,
		PlanID:      planID,
		PaymentMode: model.SubscriptionPaymentModeBalanceOnePeriod,
		RequestID:   requestID,
	}
}

func TestBalancePurchaseCreatesOnePeriodWithoutBinding(t *testing.T) {
	setupSubscriptionContractServiceTestDB(t)
	insertContractServiceUser(t, 7101, 1000)
	insertContractServicePlan(t, 7201, 1, 2.25, 2250)

	result, err := ChangeSubscriptionPlan(balanceChangeCommand(7101, 7201, "req-balance-one"))

	require.NoError(t, err)
	require.Equal(t, ChangePlanStatusApplied, result.Status)
	require.NotNil(t, result.Contract)
	require.NotNil(t, result.Intent)
	require.Equal(t, model.SubscriptionContractStatusActive, result.Contract.Status)
	require.Equal(t, model.SubscriptionPaymentModeBalanceOnePeriod, result.Contract.PaymentMode)
	require.Equal(t, 7201, result.Contract.CurrentPlanId)
	require.Zero(t, result.Contract.CurrentProviderBindingId)
	require.Equal(t, result.Intent.Id, result.Contract.LatestChangeIntentId)
	require.Equal(t, model.SubscriptionChangeIntentStatusApplied, result.Intent.Status)
	require.Zero(t, result.Intent.ProviderBindingId)

	var user model.User
	require.NoError(t, model.DB.First(&user, "id = ?", 7101).Error)
	require.Equal(t, 775, user.Quota)

	var orders []model.SubscriptionOrder
	require.NoError(t, model.DB.Where("user_id = ?", 7101).Find(&orders).Error)
	require.Len(t, orders, 1)
	require.Equal(t, common.TopUpStatusSuccess, orders[0].Status)
	require.Equal(t, model.PaymentProviderBalance, orders[0].PaymentProvider)

	var bindingCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionProviderBinding{}).Where("user_id = ?", 7101).Count(&bindingCount).Error)
	require.Zero(t, bindingCount)

	var entitlements []model.UserSubscription
	require.NoError(t, model.DB.Where("user_id = ?", 7101).Find(&entitlements).Error)
	require.Len(t, entitlements, 1)
	require.Equal(t, result.Contract.Id, entitlements[0].ContractId)
	require.Equal(t, "balance", entitlements[0].Source)
	require.Equal(t, model.SubscriptionPaymentModeBalanceOnePeriod, entitlements[0].PaymentMode)
	require.NotNil(t, entitlements[0].CurrentSlot)
}

func TestSameRequestIDReturnsSameIntent(t *testing.T) {
	setupSubscriptionContractServiceTestDB(t)
	insertContractServiceUser(t, 7102, 1000)
	insertContractServicePlan(t, 7202, 1, 1.5, 1500)

	first, err := ChangeSubscriptionPlan(balanceChangeCommand(7102, 7202, "stable-request-id"))
	require.NoError(t, err)
	second, err := ChangeSubscriptionPlan(balanceChangeCommand(7102, 7202, "stable-request-id"))
	require.NoError(t, err)

	require.Equal(t, first.Intent.Id, second.Intent.Id)
	require.Equal(t, first.Contract.Id, second.Contract.Id)
	require.Equal(t, ChangePlanStatusApplied, second.Status)

	var intentCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionChangeIntent{}).Where("user_id = ?", 7102).Count(&intentCount).Error)
	require.Equal(t, int64(1), intentCount)
	var orderCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("user_id = ?", 7102).Count(&orderCount).Error)
	require.Equal(t, int64(1), orderCount)
}

func TestSameRequestIDIgnoresChangedPlanOnRetry(t *testing.T) {
	setupSubscriptionContractServiceTestDB(t)
	insertContractServiceUser(t, 7106, 1000)
	insertContractServicePlan(t, 7209, 1, 1.5, 1500)

	first, err := ChangeSubscriptionPlan(balanceChangeCommand(7106, 7209, "retry-before-plan-validation"))
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", 7209).Update("enabled", false).Error)

	retry := balanceChangeCommand(7106, 999999, "retry-before-plan-validation")
	retry.PaymentMode = "unsupported_mode"
	second, err := ChangeSubscriptionPlan(retry)

	require.NoError(t, err)
	require.Equal(t, first.Intent.Id, second.Intent.Id)
	require.Equal(t, ChangePlanStatusApplied, second.Status)
	require.Equal(t, 7209, second.Intent.ToPlanId)
}

func TestUserPurchasesAreSerializedThroughOneContract(t *testing.T) {
	setupSubscriptionContractServiceTestDB(t)
	insertContractServiceUser(t, 7103, 2000)
	insertContractServicePlan(t, 7203, 1, 1, 1000)
	insertContractServicePlan(t, 7204, 2, 2, 2000)

	first, err := ChangeSubscriptionPlan(balanceChangeCommand(7103, 7203, "first-plan"))
	require.NoError(t, err)
	second, err := ChangeSubscriptionPlan(balanceChangeCommand(7103, 7204, "second-plan"))
	require.NoError(t, err)

	require.Equal(t, first.Contract.Id, second.Contract.Id)
	require.Equal(t, 7204, second.Contract.CurrentPlanId)

	var contractCount int64
	require.NoError(t, model.DB.Model(&model.UserSubscriptionContract{}).Where("user_id = ?", 7103).Count(&contractCount).Error)
	require.Equal(t, int64(1), contractCount)
	var currentCount int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("contract_id = ? AND current_slot = ?", first.Contract.Id, 1).Count(&currentCount).Error)
	require.Equal(t, int64(1), currentCount)
}

func TestSameRankOrSamePlanIsRejected(t *testing.T) {
	setupSubscriptionContractServiceTestDB(t)
	insertContractServiceUser(t, 7104, 2000)
	insertContractServicePlan(t, 7205, 1, 1, 1000)
	insertContractServicePlan(t, 7206, 1, 1, 1000)

	_, err := ChangeSubscriptionPlan(balanceChangeCommand(7104, 7205, "initial"))
	require.NoError(t, err)

	_, err = ChangeSubscriptionPlan(balanceChangeCommand(7104, 7205, "same-plan"))
	require.ErrorIs(t, err, ErrSubscriptionPlanUnchanged)

	_, err = ChangeSubscriptionPlan(balanceChangeCommand(7104, 7206, "same-rank"))
	require.ErrorIs(t, err, ErrSubscriptionPlanUnchanged)
}

func TestBalanceDowngradeDoesNotApplyImmediately(t *testing.T) {
	setupSubscriptionContractServiceTestDB(t)
	insertContractServiceUser(t, 7107, 3000)
	insertContractServicePlan(t, 7210, 1, 1, 1000)
	insertContractServicePlan(t, 7211, 3, 2, 2000)

	current, err := ChangeSubscriptionPlan(balanceChangeCommand(7107, 7211, "start-high-rank"))
	require.NoError(t, err)
	var beforeUser model.User
	require.NoError(t, model.DB.First(&beforeUser, "id = ?", 7107).Error)

	_, err = ChangeSubscriptionPlan(balanceChangeCommand(7107, 7210, "downgrade-low-rank"))

	require.ErrorIs(t, err, ErrSubscriptionDowngradeDeferred)
	var afterUser model.User
	require.NoError(t, model.DB.First(&afterUser, "id = ?", 7107).Error)
	require.Equal(t, beforeUser.Quota, afterUser.Quota)
	var contract model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&contract, "id = ?", current.Contract.Id).Error)
	require.Equal(t, 7211, contract.CurrentPlanId)
	require.Equal(t, current.Contract.CurrentEntitlementId, contract.CurrentEntitlementId)
	var orderCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("user_id = ?", 7107).Count(&orderCount).Error)
	require.Equal(t, int64(1), orderCount)
}

func TestBalancePurchaseEnforcesMaxPurchasePerUser(t *testing.T) {
	setupSubscriptionContractServiceTestDB(t)
	insertContractServiceUser(t, 7108, 3000)
	plan := insertContractServicePlan(t, 7212, 1, 1, 1000)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).
		Where("id = ?", plan.Id).
		Update("max_purchase_per_user", 1).Error)
	require.NoError(t, model.DB.Create(&model.UserSubscription{
		UserId:      7108,
		PlanId:      plan.Id,
		AmountTotal: plan.TotalAmount,
		Status:      "expired",
		Source:      model.PaymentMethodBalance,
		PaymentMode: model.SubscriptionPaymentModeBalanceOnePeriod,
	}).Error)

	_, err := ChangeSubscriptionPlan(balanceChangeCommand(7108, plan.Id, "purchase-limit"))

	require.Error(t, err)
	require.Contains(t, err.Error(), "purchase limit")
	var orderCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("user_id = ?", 7108).Count(&orderCount).Error)
	require.Zero(t, orderCount)
	var intentCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionChangeIntent{}).Where("user_id = ?", 7108).Count(&intentCount).Error)
	require.Zero(t, intentCount)
}

func TestBalancePurchaseRejectsNegativePlanPrice(t *testing.T) {
	setupSubscriptionContractServiceTestDB(t)
	insertContractServiceUser(t, 7109, 3000)
	insertContractServicePlan(t, 7213, 1, -1, 1000)

	_, err := ChangeSubscriptionPlan(balanceChangeCommand(7109, 7213, "negative-price"))

	require.Error(t, err)
	require.Contains(t, err.Error(), "price")
	var orderCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("user_id = ?", 7109).Count(&orderCount).Error)
	require.Zero(t, orderCount)
	var intentCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionChangeIntent{}).Where("user_id = ?", 7109).Count(&intentCount).Error)
	require.Zero(t, intentCount)
}

func TestUnresolvedPurchaseBlocksSecondChange(t *testing.T) {
	setupSubscriptionContractServiceTestDB(t)
	insertContractServiceUser(t, 7105, 2000)
	insertContractServicePlan(t, 7207, 1, 1, 1000)
	insertContractServicePlan(t, 7208, 2, 2, 2000)
	require.NoError(t, model.DB.Create(&model.UserSubscriptionContract{
		UserId:      7105,
		Status:      model.SubscriptionContractStatusEnded,
		PaymentMode: model.SubscriptionPaymentModeExternalOnePeriod,
	}).Error)
	var contract model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&contract, "user_id = ?", 7105).Error)
	require.NoError(t, model.DB.Create(&model.SubscriptionChangeIntent{
		ContractId:    contract.Id,
		UserId:        7105,
		RequestId:     "pending-intent",
		Kind:          model.SubscriptionChangeIntentKindPurchase,
		PaymentMode:   model.SubscriptionPaymentModeStripeRecurring,
		Status:        model.SubscriptionChangeIntentStatusAwaitingPayment,
		FromPlanId:    0,
		ToPlanId:      7207,
		EffectiveAt:   common.GetTimestamp(),
		ChangeVersion: contract.ChangeVersion + 1,
	}).Error)

	_, err := ChangeSubscriptionPlan(balanceChangeCommand(7105, 7208, "blocked-by-pending"))

	require.ErrorIs(t, err, ErrSubscriptionChangeInProgress)
}
