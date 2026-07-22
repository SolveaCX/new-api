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

func setupStripeSubscriptionLifecycleTestDB(t *testing.T) {
	t.Helper()
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedisEnabled := common.RedisEnabled
	originalUsingSQLite := common.UsingSQLite
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalUsingMySQL := common.UsingMySQL

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(5)

	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.UsingMySQL = false

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedisEnabled
		common.UsingSQLite = originalUsingSQLite
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.UsingMySQL = originalUsingMySQL
		require.NoError(t, sqlDB.Close())
	})

	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Log{},
		&model.SubscriptionPlan{},
		&model.SubscriptionOrder{},
		&model.UserSubscription{},
		&model.SubscriptionProviderBinding{},
	))
}

func insertStripeLifecycleBinding(t *testing.T, userID int, status string, cancelAtPeriodEnd bool) *model.SubscriptionProviderBinding {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:       userID,
		Username: "stripe_lifecycle_user",
		Status:   common.UserStatusEnabled,
		AffCode:  "stripe_lifecycle_aff",
	}).Error)
	require.NoError(t, model.DB.Create(&model.SubscriptionPlan{
		Id:            900 + userID,
		Title:         "Lifecycle Plan",
		PriceAmount:   9.99,
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   1000,
		StripePriceId: "price_lifecycle",
	}).Error)
	binding := &model.SubscriptionProviderBinding{
		UserId:                 userID,
		PlanId:                 900 + userID,
		InitialOrderId:         1000 + userID,
		Provider:               model.PaymentProviderStripe,
		ProviderSubscriptionId: "sub_lifecycle",
		ProviderCustomerId:     "cus_lifecycle",
		ProviderPriceId:        "price_lifecycle",
		ProviderStatus:         status,
		CancelAtPeriodEnd:      cancelAtPeriodEnd,
		CurrentPeriodStart:     1000,
		CurrentPeriodEnd:       2000,
	}
	require.NoError(t, model.DB.Create(binding).Error)
	require.NoError(t, model.DB.Create(&model.UserSubscription{
		UserId:            userID,
		PlanId:            binding.PlanId,
		ProviderBindingId: binding.Id,
		AmountTotal:       1000,
		StartTime:         1000,
		EndTime:           2000,
		Status:            "active",
		Source:            "order",
	}).Error)
	return binding
}

func TestStripeSubscriptionLifecycleCancelMarksPeriodEnd(t *testing.T) {
	setupStripeSubscriptionLifecycleTestDB(t)
	binding := insertStripeLifecycleBinding(t, 801, "active", false)
	originalCancel := stripeUpdateSubscriptionCancelAtPeriodEnd
	t.Cleanup(func() { stripeUpdateSubscriptionCancelAtPeriodEnd = originalCancel })
	var gotSubscriptionID string
	var gotIdempotencyKey string
	stripeUpdateSubscriptionCancelAtPeriodEnd = func(providerSubscriptionID string, cancelAtPeriodEnd bool, idempotencyKey string) (model.ProviderSubscriptionSnapshot, error) {
		gotSubscriptionID = providerSubscriptionID
		gotIdempotencyKey = idempotencyKey
		require.True(t, cancelAtPeriodEnd)
		return model.ProviderSubscriptionSnapshot{
			ProviderSubscriptionId: providerSubscriptionID,
			ProviderCustomerId:     "cus_lifecycle",
			ProviderPriceId:        "price_lifecycle",
			ProviderStatus:         "active",
			CancelAtPeriodEnd:      true,
			CurrentPeriodStart:     1000,
			CurrentPeriodEnd:       2000,
		}, nil
	}

	updated, err := CancelStripeRecurringSubscription(801, binding.Id)

	require.NoError(t, err)
	require.Equal(t, "sub_lifecycle", gotSubscriptionID)
	require.Contains(t, gotIdempotencyKey, "binding_")
	require.True(t, updated.CancelAtPeriodEnd)
}

func TestStripeSubscriptionLifecycleResumeClearsPeriodEnd(t *testing.T) {
	setupStripeSubscriptionLifecycleTestDB(t)
	binding := insertStripeLifecycleBinding(t, 802, "active", true)
	originalUpdate := stripeUpdateSubscriptionCancelAtPeriodEnd
	t.Cleanup(func() { stripeUpdateSubscriptionCancelAtPeriodEnd = originalUpdate })
	stripeUpdateSubscriptionCancelAtPeriodEnd = func(providerSubscriptionID string, cancelAtPeriodEnd bool, idempotencyKey string) (model.ProviderSubscriptionSnapshot, error) {
		require.False(t, cancelAtPeriodEnd)
		return model.ProviderSubscriptionSnapshot{
			ProviderSubscriptionId: providerSubscriptionID,
			ProviderCustomerId:     "cus_lifecycle",
			ProviderPriceId:        "price_lifecycle",
			ProviderStatus:         "active",
			CancelAtPeriodEnd:      false,
			CurrentPeriodStart:     1000,
			CurrentPeriodEnd:       2000,
		}, nil
	}

	updated, err := ResumeStripeRecurringSubscription(802, binding.Id)

	require.NoError(t, err)
	require.False(t, updated.CancelAtPeriodEnd)
}

func TestStripeSubscriptionLifecycleRejectsForeignBinding(t *testing.T) {
	setupStripeSubscriptionLifecycleTestDB(t)
	binding := insertStripeLifecycleBinding(t, 803, "active", false)

	_, err := CancelStripeRecurringSubscription(804, binding.Id)

	require.Error(t, err)
}

func TestStripeSubscriptionLifecyclePastDueCancelTerminatesLocalEntitlement(t *testing.T) {
	setupStripeSubscriptionLifecycleTestDB(t)
	binding := insertStripeLifecycleBinding(t, 805, "past_due", false)
	originalCancelNow := stripeCancelSubscriptionNow
	t.Cleanup(func() { stripeCancelSubscriptionNow = originalCancelNow })
	stripeCancelSubscriptionNow = func(providerSubscriptionID string, idempotencyKey string) (model.ProviderSubscriptionSnapshot, error) {
		return model.ProviderSubscriptionSnapshot{
			ProviderSubscriptionId: providerSubscriptionID,
			ProviderCustomerId:     "cus_lifecycle",
			ProviderPriceId:        "price_lifecycle",
			ProviderStatus:         "canceled",
			EndedAt:                common.GetTimestamp(),
		}, nil
	}

	updated, err := CancelStripeRecurringSubscription(805, binding.Id)

	require.NoError(t, err)
	require.Equal(t, "canceled", updated.ProviderStatus)
	var sub model.UserSubscription
	require.NoError(t, model.DB.Where("provider_binding_id = ?", binding.Id).First(&sub).Error)
	require.Equal(t, "cancelled", sub.Status)
}

func TestAdminInvalidateStripeRecurringSubscriptionCancelsRemoteBeforeLocal(t *testing.T) {
	setupStripeSubscriptionLifecycleTestDB(t)
	binding := insertStripeLifecycleBinding(t, 806, "active", false)
	sub := stripeLifecycleUserSubscriptionForBinding(t, binding.Id)
	originalCancelNow := stripeCancelSubscriptionNow
	t.Cleanup(func() { stripeCancelSubscriptionNow = originalCancelNow })
	var called bool
	stripeCancelSubscriptionNow = func(providerSubscriptionID string, idempotencyKey string) (model.ProviderSubscriptionSnapshot, error) {
		called = true
		require.Equal(t, "sub_lifecycle", providerSubscriptionID)
		require.Contains(t, idempotencyKey, "admin_invalidate")
		var before model.UserSubscription
		require.NoError(t, model.DB.First(&before, sub.Id).Error)
		require.Equal(t, "active", before.Status)
		return model.ProviderSubscriptionSnapshot{
			ProviderSubscriptionId: providerSubscriptionID,
			ProviderStatus:         "canceled",
			EndedAt:                common.GetTimestamp(),
		}, nil
	}

	_, err := AdminInvalidateUserSubscriptionWithRecurringPolicy(sub.Id)

	require.NoError(t, err)
	require.True(t, called)
	var updated model.UserSubscription
	require.NoError(t, model.DB.First(&updated, sub.Id).Error)
	require.Equal(t, "cancelled", updated.Status)
	var updatedBinding model.SubscriptionProviderBinding
	require.NoError(t, model.DB.First(&updatedBinding, binding.Id).Error)
	require.Equal(t, "canceled", updatedBinding.ProviderStatus)
}

func TestAdminInvalidateStripeRecurringSubscriptionRemoteFailureKeepsLocalActive(t *testing.T) {
	setupStripeSubscriptionLifecycleTestDB(t)
	binding := insertStripeLifecycleBinding(t, 807, "active", false)
	sub := stripeLifecycleUserSubscriptionForBinding(t, binding.Id)
	originalCancelNow := stripeCancelSubscriptionNow
	t.Cleanup(func() { stripeCancelSubscriptionNow = originalCancelNow })
	stripeCancelSubscriptionNow = func(providerSubscriptionID string, idempotencyKey string) (model.ProviderSubscriptionSnapshot, error) {
		return model.ProviderSubscriptionSnapshot{}, assertAnErrorForAdminLifecycleTest
	}

	_, err := AdminInvalidateUserSubscriptionWithRecurringPolicy(sub.Id)

	require.ErrorIs(t, err, assertAnErrorForAdminLifecycleTest)
	var updated model.UserSubscription
	require.NoError(t, model.DB.First(&updated, sub.Id).Error)
	require.Equal(t, "active", updated.Status)
	var updatedBinding model.SubscriptionProviderBinding
	require.NoError(t, model.DB.First(&updatedBinding, binding.Id).Error)
	require.Equal(t, "active", updatedBinding.ProviderStatus)
}

func TestAdminInvalidateNonStripeSubscriptionKeepsLocalBehavior(t *testing.T) {
	setupStripeSubscriptionLifecycleTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       808,
		Username: "local_admin_user",
		Status:   common.UserStatusEnabled,
		AffCode:  "local_admin_aff",
	}).Error)
	require.NoError(t, model.DB.Create(&model.SubscriptionPlan{
		Id:            1708,
		Title:         "Local Admin Plan",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   1000,
	}).Error)
	sub := &model.UserSubscription{
		UserId:      808,
		PlanId:      1708,
		AmountTotal: 1000,
		StartTime:   1000,
		EndTime:     2000,
		Status:      "active",
		Source:      "admin",
	}
	require.NoError(t, model.DB.Create(sub).Error)
	originalCancelNow := stripeCancelSubscriptionNow
	t.Cleanup(func() { stripeCancelSubscriptionNow = originalCancelNow })
	stripeCancelSubscriptionNow = func(providerSubscriptionID string, idempotencyKey string) (model.ProviderSubscriptionSnapshot, error) {
		t.Fatal("non-Stripe admin invalidate must not call Stripe")
		return model.ProviderSubscriptionSnapshot{}, nil
	}

	_, err := AdminInvalidateUserSubscriptionWithRecurringPolicy(sub.Id)

	require.NoError(t, err)
	var updated model.UserSubscription
	require.NoError(t, model.DB.First(&updated, sub.Id).Error)
	require.Equal(t, "cancelled", updated.Status)
}

func TestAdminDeleteStripeRecurringSubscriptionHistoryIsRejected(t *testing.T) {
	setupStripeSubscriptionLifecycleTestDB(t)
	binding := insertStripeLifecycleBinding(t, 809, "active", false)
	sub := stripeLifecycleUserSubscriptionForBinding(t, binding.Id)

	_, err := AdminDeleteUserSubscriptionWithRecurringPolicy(sub.Id)

	require.Error(t, err)
	var existing model.UserSubscription
	require.NoError(t, model.DB.First(&existing, sub.Id).Error)
}

var assertAnErrorForAdminLifecycleTest = errAdminLifecycleTest{}

type errAdminLifecycleTest struct{}

func (errAdminLifecycleTest) Error() string {
	return "admin lifecycle failure"
}

func stripeLifecycleUserSubscriptionForBinding(t *testing.T, bindingID int64) model.UserSubscription {
	t.Helper()
	var sub model.UserSubscription
	require.NoError(t, model.DB.Where("provider_binding_id = ?", bindingID).First(&sub).Error)
	return sub
}
