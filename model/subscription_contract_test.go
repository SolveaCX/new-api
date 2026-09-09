package model

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacyUserSubscriptionWithoutWindowScope struct {
	Id          int
	UserId      int
	ContractId  int64
	GrantKey    *string `gorm:"type:varchar(255);uniqueIndex"`
	CurrentSlot *int
}

func (legacyUserSubscriptionWithoutWindowScope) TableName() string {
	return "user_subscriptions"
}

func TestSubscriptionContractMigrationCreatesLifecycleTablesAndColumns(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)

	require.NoError(t, migrateDBFast())

	require.True(t, DB.Migrator().HasTable(&UserSubscriptionContract{}))
	require.True(t, DB.Migrator().HasTable(&SubscriptionChangeIntent{}))
	require.True(t, DB.Migrator().HasTable(&SubscriptionTierRankReservation{}))
	require.True(t, DB.Migrator().HasTable(&SubscriptionTermSegment{}))
	require.True(t, DB.Migrator().HasTable(&WalletLedgerEntry{}))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionOrder{}, "purchase_months"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionOrder{}, "unit_price"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionOrder{}, "plan_snapshot"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionOrder{}, "purchase_intent"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionOrder{}, "renewal_source"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscriptionContract{}, "grace_period_end"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscriptionContract{}, "renewal_source"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscriptionContract{}, "renewal_status"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "contract_id"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "grant_key"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "current_slot"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "access_end_time"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "end_reason"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "window_5h_amount"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "window_week_amount"))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "window_scope_version"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionProviderBinding{}, "contract_id"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionProviderBinding{}, "provider_subscription_item_id"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionProviderBinding{}, "provider_schedule_id"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionProviderBinding{}, "lifecycle_reservation_token"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionProviderBinding{}, "lifecycle_reservation_action"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionProviderBinding{}, "lifecycle_reservation_until"))
}

func TestSubscriptionWindowScopeMigrationKeepsExistingRowsAndSchemaDefaultLegacy(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	require.NoError(t, db.AutoMigrate(&legacyUserSubscriptionWithoutWindowScope{}))
	currentSlot := 1
	require.NoError(t, db.Create(&legacyUserSubscriptionWithoutWindowScope{
		Id:          838,
		UserId:      12462,
		ContractId:  132,
		CurrentSlot: &currentSlot,
	}).Error)

	require.NoError(t, db.AutoMigrate(&UserSubscription{}))

	var version int16
	require.NoError(t, db.Table("user_subscriptions").
		Select("window_scope_version").
		Where("id = ?", 838).
		Scan(&version).Error)
	require.Equal(t, SubscriptionWindowScopeVersionLegacy, version)

	// A writer that does not know about the column must continue to create a
	// legacy row. Activation is a creator decision, not a schema-default switch.
	require.NoError(t, db.Create(&legacyUserSubscriptionWithoutWindowScope{
		Id:         839,
		UserId:     12463,
		ContractId: 133,
	}).Error)
	require.NoError(t, db.Table("user_subscriptions").
		Select("window_scope_version").
		Where("id = ?", 839).
		Scan(&version).Error)
	require.Equal(t, SubscriptionWindowScopeVersionLegacy, version)

	// Explicitly activated creators may write version 1. Re-running the migration
	// must preserve both the existing legacy rows and the activated row.
	require.NoError(t, db.Table("user_subscriptions").Create(map[string]interface{}{
		"id":                   840,
		"user_id":              12464,
		"contract_id":          134,
		"window_scope_version": SubscriptionWindowScopeVersionEntitlement,
	}).Error)
	require.NoError(t, db.AutoMigrate(&UserSubscription{}))

	var versions []int16
	require.NoError(t, db.Table("user_subscriptions").
		Where("id IN ?", []int{838, 839, 840}).
		Order("id ASC").
		Pluck("window_scope_version", &versions).Error)
	require.Equal(t, []int16{
		SubscriptionWindowScopeVersionLegacy,
		SubscriptionWindowScopeVersionLegacy,
		SubscriptionWindowScopeVersionEntitlement,
	}, versions)
}

func TestSubscriptionContractAllowsOnlyOneContractPerUser(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionContractTestDB(t)

	require.NoError(t, DB.Create(&UserSubscriptionContract{
		UserId:      7001,
		Status:      SubscriptionContractStatusActive,
		PaymentMode: SubscriptionPaymentModeStripeRecurring,
	}).Error)

	err := DB.Create(&UserSubscriptionContract{
		UserId:      7001,
		Status:      SubscriptionContractStatusActive,
		PaymentMode: SubscriptionPaymentModeStripeRecurring,
	}).Error

	require.Error(t, err)
}

func TestSubscriptionContractEnumValuesRoundTripAndDefaults(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionContractTestDB(t)

	contractStatuses := []string{
		SubscriptionContractStatusActive,
		SubscriptionContractStatusGrace,
		SubscriptionContractStatusEnded,
		SubscriptionContractStatusNeedsAttention,
	}
	paymentModes := []string{
		SubscriptionPaymentModeStripeRecurring,
		SubscriptionPaymentModeBalanceOnePeriod,
		SubscriptionPaymentModeExternalOnePeriod,
	}

	for i, status := range contractStatuses {
		for j, paymentMode := range paymentModes {
			contract := &UserSubscriptionContract{
				UserId:      8000 + i*10 + j,
				Status:      status,
				PaymentMode: paymentMode,
			}
			require.NoError(t, DB.Create(contract).Error)

			var stored UserSubscriptionContract
			require.NoError(t, DB.First(&stored, "id = ?", contract.Id).Error)
			require.Equal(t, status, stored.Status)
			require.Equal(t, paymentMode, stored.PaymentMode)
		}
	}

	defaultContract := &UserSubscriptionContract{UserId: 8099}
	require.NoError(t, DB.Create(defaultContract).Error)

	var storedDefault UserSubscriptionContract
	require.NoError(t, DB.First(&storedDefault, "id = ?", defaultContract.Id).Error)
	require.Equal(t, SubscriptionContractStatusEnded, storedDefault.Status)
	require.Equal(t, SubscriptionPaymentModeExternalOnePeriod, storedDefault.PaymentMode)
}

func TestSubscriptionContractWalletRenewalSourceAndStatusPersist(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionContractTestDB(t)

	contract := &UserSubscriptionContract{
		UserId:        8110,
		Status:        SubscriptionContractStatusActive,
		PaymentMode:   SubscriptionPaymentModeBalanceOnePeriod,
		RenewalSource: SubscriptionRenewalSourceWallet,
		RenewalStatus: SubscriptionRenewalStatusPausedInsufficientBalance,
	}
	require.NoError(t, DB.Create(contract).Error)

	var stored UserSubscriptionContract
	require.NoError(t, DB.First(&stored, "id = ?", contract.Id).Error)
	require.Equal(t, SubscriptionRenewalSourceWallet, stored.RenewalSource)
	require.Equal(t, SubscriptionRenewalStatusPausedInsufficientBalance, stored.RenewalStatus)
}

func TestSubscriptionChangeIntentEnumValuesRoundTripAndDefaults(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionContractTestDB(t)

	intentKinds := []string{
		SubscriptionChangeIntentKindPurchase,
		SubscriptionChangeIntentKindUpgrade,
		SubscriptionChangeIntentKindDowngrade,
		SubscriptionChangeIntentKindCancel,
		SubscriptionChangeIntentKindResume,
		SubscriptionChangeIntentKindTerminate,
	}
	intentStatuses := []string{
		SubscriptionChangeIntentStatusCreated,
		SubscriptionChangeIntentStatusSyncing,
		SubscriptionChangeIntentStatusAwaitingPayment,
		SubscriptionChangeIntentStatusScheduled,
		SubscriptionChangeIntentStatusApplied,
		SubscriptionChangeIntentStatusFailed,
		SubscriptionChangeIntentStatusExpired,
		SubscriptionChangeIntentStatusSuperseded,
		SubscriptionChangeIntentStatusCompensationRequired,
	}

	for i, kind := range intentKinds {
		for j, status := range intentStatuses {
			intent := &SubscriptionChangeIntent{
				ContractId:  9001,
				UserId:      8100 + i,
				RequestId:   "intent-round-trip",
				Kind:        kind,
				PaymentMode: SubscriptionPaymentModeBalanceOnePeriod,
				Status:      status,
			}
			intent.RequestId = intent.RequestId + "-" + kind + "-" + status + "-" + string(rune('a'+j))
			require.NoError(t, DB.Create(intent).Error)

			var stored SubscriptionChangeIntent
			require.NoError(t, DB.First(&stored, "id = ?", intent.Id).Error)
			require.Equal(t, kind, stored.Kind)
			require.Equal(t, SubscriptionPaymentModeBalanceOnePeriod, stored.PaymentMode)
			require.Equal(t, status, stored.Status)
		}
	}

	defaultIntent := &SubscriptionChangeIntent{
		ContractId: 9002,
		UserId:     8199,
		RequestId:  "intent-defaults",
	}
	require.NoError(t, DB.Create(defaultIntent).Error)

	var storedDefault SubscriptionChangeIntent
	require.NoError(t, DB.First(&storedDefault, "id = ?", defaultIntent.Id).Error)
	require.Equal(t, SubscriptionChangeIntentStatusCreated, storedDefault.Status)
	require.Equal(t, SubscriptionPaymentModeExternalOnePeriod, storedDefault.PaymentMode)
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

func TestSubscriptionEntitlementGrantKeySupportsDesignLengthAndUniqueConstraint(t *testing.T) {
	setupSubscriptionRecurringTestDB(t)
	migrateSubscriptionContractTestDB(t)

	columnTypes, err := DB.Migrator().ColumnTypes(&UserSubscription{})
	require.NoError(t, err)
	grantKeyDeclaredAsVarchar255 := false
	for _, columnType := range columnTypes {
		length, hasLength := columnType.Length()
		if columnType.Name() == "grant_key" &&
			strings.Contains(strings.ToLower(columnType.DatabaseTypeName()), "varchar") &&
			hasLength && length == 255 {
			grantKeyDeclaredAsVarchar255 = true
		}
	}
	require.True(t, grantKeyDeclaredAsVarchar255)

	grantKey := strings.Repeat("g", 200)
	require.NoError(t, DB.Create(&UserSubscription{
		UserId:     7271,
		PlanId:     7371,
		ContractId: 7471,
		GrantKey:   &grantKey,
		Status:     "expired",
	}).Error)

	var stored UserSubscription
	require.NoError(t, DB.First(&stored, "user_id = ?", 7271).Error)
	require.NotNil(t, stored.GrantKey)
	require.Equal(t, grantKey, *stored.GrantKey)

	duplicateGrantKey := grantKey
	err = DB.Create(&UserSubscription{
		UserId:     7272,
		PlanId:     7372,
		ContractId: 7472,
		GrantKey:   &duplicateGrantKey,
		Status:     "expired",
	}).Error
	require.Error(t, err)
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
		ProviderScheduleIdObserved: true,
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
		&SubscriptionTermSegment{},
		&WalletLedgerEntry{},
		&SubscriptionPreConsumeRecord{},
		&RecallLifecycleEvent{},
		&QuotaLifecycleState{},
	))
}
