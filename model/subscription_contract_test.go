package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubscriptionContractMigrationCreatesLifecycleTablesAndColumns(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)

	require.NoError(t, migrateDBFast())

	require.True(t, DB.Migrator().HasTable(&UserSubscriptionContract{}))
	require.True(t, DB.Migrator().HasTable(&SubscriptionChangeIntent{}))
	require.True(t, DB.Migrator().HasTable(&SubscriptionTierRankReservation{}))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "contract_id"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "grant_key"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "current_slot"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "access_end_time"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "end_reason"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionProviderBinding{}, "contract_id"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionProviderBinding{}, "provider_subscription_item_id"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionProviderBinding{}, "provider_schedule_id"))
}

func TestSubscriptionContractAllowsOnlyOneContractPerUser(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionContractTestDB(t)

	require.NoError(t, DB.Create(&UserSubscriptionContract{
		UserId:      7001,
		Status:      SubscriptionContractStatusActive,
		PaymentMode: SubscriptionPaymentModeRecurring,
	}).Error)

	err := DB.Create(&UserSubscriptionContract{
		UserId:      7001,
		Status:      SubscriptionContractStatusActive,
		PaymentMode: SubscriptionPaymentModeRecurring,
	}).Error

	require.Error(t, err)
}

func TestOnlyOneCurrentEntitlementPerContract(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionContractTestDB(t)

	currentSlot := 1
	require.NoError(t, DB.Create(&UserSubscription{
		UserId:      7101,
		PlanId:      7201,
		ContractId:  7301,
		CurrentSlot: &currentSlot,
		Status:      "active",
	}).Error)

	err := DB.Create(&UserSubscription{
		UserId:      7101,
		PlanId:      7202,
		ContractId:  7301,
		CurrentSlot: &currentSlot,
		Status:      "active",
	}).Error

	require.Error(t, err)
}

func TestSubscriptionEntitlementNullableCurrentSlotAndGrantKeyConstraints(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionContractTestDB(t)

	require.NoError(t, DB.Create(&UserSubscription{
		UserId:      7201,
		PlanId:      7301,
		ContractId:  7401,
		CurrentSlot: nil,
		GrantKey:    nil,
		Status:      "expired",
	}).Error)
	require.NoError(t, DB.Create(&UserSubscription{
		UserId:      7201,
		PlanId:      7302,
		ContractId:  7401,
		CurrentSlot: nil,
		GrantKey:    nil,
		Status:      "expired",
	}).Error)

	grantKey := "grant-contract-7401"
	require.NoError(t, DB.Create(&UserSubscription{
		UserId:     7201,
		PlanId:     7303,
		ContractId: 7401,
		GrantKey:   &grantKey,
		Status:     "expired",
	}).Error)

	duplicateGrantKey := grantKey
	err := DB.Create(&UserSubscription{
		UserId:     7202,
		PlanId:     7304,
		ContractId: 7402,
		GrantKey:   &duplicateGrantKey,
		Status:     "expired",
	}).Error
	require.Error(t, err)
}

func TestSubscriptionEntitlementBlankGrantKeyPersistsAsNull(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionContractTestDB(t)

	blankGrantKey := "   "
	require.NoError(t, DB.Create(&UserSubscription{
		UserId:     7251,
		PlanId:     7351,
		GrantKey:   &blankGrantKey,
		ContractId: 7451,
		Status:     "expired",
	}).Error)

	var stored UserSubscription
	require.NoError(t, DB.First(&stored, "user_id = ?", 7251).Error)
	require.Nil(t, stored.GrantKey)
}

func TestProviderSubscriptionSnapshotPersistsItemScheduleAndContractFields(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionContractTestDB(t)
	insertUserForSubscriptionRecurringTest(t, 7501)
	insertPlanForSubscriptionRecurringTest(t, 7601, "price_recurring")
	insertOrderForSubscriptionRecurringTest(t, "contract-recurring-order", 7501, 7601)

	binding, err := CompleteSubscriptionOrderWithProviderBinding(
		"contract-recurring-order",
		"{}",
		PaymentProviderStripe,
		PaymentMethodStripe,
		ProviderSubscriptionSnapshot{
			ProviderSubscriptionId:     "sub_contract",
			ProviderSubscriptionItemId: "si_contract",
			ProviderScheduleId:         "sub_sched_contract",
			ProviderCustomerId:         "cus_contract",
			ProviderPriceId:            "price_recurring",
			ProviderStatus:             "active",
			CurrentPeriodStart:         100,
			CurrentPeriodEnd:           200,
		},
	)
	require.NoError(t, err)
	require.Equal(t, int64(0), binding.ContractId)
	require.Equal(t, "si_contract", binding.ProviderSubscriptionItemId)
	require.Equal(t, "sub_sched_contract", binding.ProviderScheduleId)

	updated, err := ApplyProviderSubscriptionSnapshot(binding.Id, ProviderSubscriptionSnapshot{
		ProviderSubscriptionId:     "sub_contract",
		ProviderSubscriptionItemId: "si_contract_updated",
		ProviderScheduleId:         "sub_sched_updated",
		ProviderCustomerId:         "cus_contract",
		ProviderPriceId:            "price_recurring",
		ProviderStatus:             "active",
		CurrentPeriodStart:         300,
		CurrentPeriodEnd:           400,
	})
	require.NoError(t, err)
	require.Equal(t, "si_contract_updated", updated.ProviderSubscriptionItemId)
	require.Equal(t, "sub_sched_updated", updated.ProviderScheduleId)
}

func migrateSubscriptionContractTestDB(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(
		&User{},
		&Log{},
		&TopUp{},
		&SubscriptionPlan{},
		&SubscriptionOrder{},
		&UserSubscription{},
		&SubscriptionProviderBinding{},
		&PaymentWebhookEvent{},
		&UserSubscriptionContract{},
		&SubscriptionChangeIntent{},
		&SubscriptionTierRankReservation{},
	))
}
