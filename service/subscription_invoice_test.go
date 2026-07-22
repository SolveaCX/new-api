package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
	"gorm.io/gorm"
)

func setupSubscriptionInvoiceServiceTestDB(t *testing.T) {
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
	sqlDB.SetMaxOpenConns(1)

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
		&model.UserSubscriptionContract{},
		&model.SubscriptionChangeIntent{},
	))
}

func seedStripeInvoicePurchase(t *testing.T, userID int, planID int, tradeNo string) (model.UserSubscriptionContract, model.SubscriptionChangeIntent) {
	t.Helper()
	rank := 1
	require.NoError(t, model.DB.Create(&model.User{
		Id:       userID,
		Username: "invoice_user",
		Email:    "invoice-user@example.com",
		Status:   common.UserStatusEnabled,
		Group:    "plg",
		AffCode:  "invoice_aff",
	}).Error)
	require.NoError(t, model.DB.Create(&model.SubscriptionPlan{
		Id:              planID,
		Title:           "Invoice Plan",
		PriceAmount:     12.34,
		Currency:        "USD",
		DurationUnit:    model.SubscriptionDurationMonth,
		DurationValue:   1,
		Enabled:         true,
		TierRank:        &rank,
		AllowBalancePay: common.GetPointer(true),
		TotalAmount:     1234,
		StripePriceId:   "price_invoice_plan",
	}).Error)
	contract := model.UserSubscriptionContract{
		UserId:      userID,
		Status:      model.SubscriptionContractStatusEnded,
		PaymentMode: model.SubscriptionPaymentModeExternalOnePeriod,
	}
	require.NoError(t, model.DB.Create(&contract).Error)
	intent := model.SubscriptionChangeIntent{
		ContractId:    contract.Id,
		UserId:        userID,
		RequestId:     "550e8400-e29b-41d4-a716-446655440100",
		Kind:          model.SubscriptionChangeIntentKindPurchase,
		PaymentMode:   model.SubscriptionPaymentModeStripeRecurring,
		Status:        model.SubscriptionChangeIntentStatusAwaitingPayment,
		ToPlanId:      planID,
		ChangeVersion: contract.ChangeVersion + 1,
	}
	require.NoError(t, model.DB.Create(&intent).Error)
	require.NoError(t, model.DB.Model(&model.UserSubscriptionContract{}).Where("id = ?", contract.Id).Update("latest_change_intent_id", intent.Id).Error)
	require.NoError(t, model.DB.Create(&model.SubscriptionOrder{
		UserId:          userID,
		PlanId:          planID,
		Money:           12.34,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusPending,
		CreateTime:      common.GetTimestamp(),
		ProviderPayload: fmt.Sprintf("change_intent_id=%d", intent.Id),
	}).Error)
	return contract, intent
}

func stripeInvoiceFixture(invoiceID string, subscriptionID string) *stripe.Invoice {
	return &stripe.Invoice{
		ID:         invoiceID,
		Paid:       true,
		Status:     stripe.InvoiceStatusPaid,
		AmountPaid: 1234,
		Total:      1234,
		Currency:   stripe.CurrencyUSD,
		Customer:   &stripe.Customer{ID: "cus_invoice"},
		Livemode:   false,
		Subscription: &stripe.Subscription{
			ID: subscriptionID,
		},
		Lines: &stripe.InvoiceLineItemList{Data: []*stripe.InvoiceLineItem{
			{
				Amount:   1234,
				Currency: stripe.CurrencyUSD,
				Price:    &stripe.Price{ID: "price_invoice_plan"},
				Period:   &stripe.Period{Start: 1700000000, End: 1702592000},
			},
		}},
	}
}

func stripeSubscriptionFixture(subscriptionID string, metadata map[string]string) *stripe.Subscription {
	return &stripe.Subscription{
		ID:                 subscriptionID,
		Customer:           &stripe.Customer{ID: "cus_invoice"},
		Status:             stripe.SubscriptionStatusActive,
		Livemode:           false,
		CurrentPeriodStart: 1700000000,
		CurrentPeriodEnd:   1702592000,
		Metadata:           metadata,
		Items: &stripe.SubscriptionItemList{Data: []*stripe.SubscriptionItem{
			{
				ID:    "si_invoice",
				Price: &stripe.Price{ID: "price_invoice_plan"},
			},
		}},
		LatestInvoice: &stripe.Invoice{ID: "in_first"},
	}
}

func TestReconcilePaidInvoiceGrantsInvoiceFirstPurchase(t *testing.T) {
	setupSubscriptionInvoiceServiceTestDB(t)
	contract, intent := seedStripeInvoicePurchase(t, 8101, 8201, "sub_invoice_first")
	restore := replaceStripeInvoiceReconcilers(t, stripeInvoiceFixture("in_first", "sub_invoice_first"), stripeSubscriptionFixture("sub_invoice_first", map[string]string{
		"trade_no":         "sub_invoice_first",
		"user_id":          "8101",
		"plan_id":          "8201",
		"contract_id":      strconv.FormatInt(contract.Id, 10),
		"change_intent_id": strconv.FormatInt(intent.Id, 10),
	}))
	defer restore()

	result, err := ReconcilePaidInvoice(context.Background(), "in_first")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Applied)
	var entitlement model.UserSubscription
	require.NoError(t, model.DB.First(&entitlement, "user_id = ?", 8101).Error)
	require.Equal(t, "stripe:in_first", *entitlement.GrantKey)
	require.Equal(t, int64(1234), entitlement.AmountTotal)
	require.Equal(t, model.SubscriptionPaymentModeStripeRecurring, entitlement.PaymentMode)
	var bindingCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionProviderBinding{}).Where("provider_subscription_id = ?", "sub_invoice_first").Count(&bindingCount).Error)
	require.Equal(t, int64(1), bindingCount)
	var order model.SubscriptionOrder
	require.NoError(t, model.DB.First(&order, "trade_no = ?", "sub_invoice_first").Error)
	require.Equal(t, common.TopUpStatusSuccess, order.Status)
	var applied model.SubscriptionChangeIntent
	require.NoError(t, model.DB.First(&applied, "id = ?", intent.Id).Error)
	require.Equal(t, model.SubscriptionChangeIntentStatusApplied, applied.Status)
	require.Equal(t, "in_first", applied.ProviderInvoiceId)
}

func TestReconcilePaidInvoiceIsIdempotentForDuplicateAndCheckoutFirst(t *testing.T) {
	setupSubscriptionInvoiceServiceTestDB(t)
	contract, intent := seedStripeInvoicePurchase(t, 8102, 8202, "sub_duplicate_invoice")
	restore := replaceStripeInvoiceReconcilers(t, stripeInvoiceFixture("in_duplicate", "sub_duplicate_invoice"), stripeSubscriptionFixture("sub_duplicate_invoice", map[string]string{
		"trade_no":         "sub_duplicate_invoice",
		"user_id":          "8102",
		"plan_id":          "8202",
		"contract_id":      strconv.FormatInt(contract.Id, 10),
		"change_intent_id": strconv.FormatInt(intent.Id, 10),
	}))
	defer restore()

	first, err := ReconcilePaidInvoice(context.Background(), "in_duplicate")
	require.NoError(t, err)
	second, err := ReconcilePaidInvoice(context.Background(), "in_duplicate")
	require.NoError(t, err)

	require.True(t, first.Applied)
	require.False(t, second.Applied)
	var entitlementCount int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("user_id = ?", 8102).Count(&entitlementCount).Error)
	require.Equal(t, int64(1), entitlementCount)
	var bindingCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionProviderBinding{}).Where("user_id = ?", 8102).Count(&bindingCount).Error)
	require.Equal(t, int64(1), bindingCount)
}

func TestReconcilePaidInvoiceRejectsLocalStripeCustomerMismatch(t *testing.T) {
	setupSubscriptionInvoiceServiceTestDB(t)
	contract, intent := seedStripeInvoicePurchase(t, 8105, 8205, "sub_customer_mismatch")
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 8105).Update("stripe_customer", "cus_expected").Error)
	restore := replaceStripeInvoiceReconcilers(t, stripeInvoiceFixture("in_customer_mismatch", "sub_customer_mismatch"), stripeSubscriptionFixture("sub_customer_mismatch", map[string]string{
		"trade_no":         "sub_customer_mismatch",
		"user_id":          "8105",
		"plan_id":          "8205",
		"contract_id":      strconv.FormatInt(contract.Id, 10),
		"change_intent_id": strconv.FormatInt(intent.Id, 10),
	}))
	defer restore()

	_, err := ReconcilePaidInvoice(context.Background(), "in_customer_mismatch")

	require.Error(t, err)
	require.True(t, IsPermanentPaidInvoiceError(err))
	require.Contains(t, err.Error(), "customer mismatch")
}

func TestReconcilePaidInvoiceRejectsInvoiceSubscriptionLivemodeMismatch(t *testing.T) {
	setupSubscriptionInvoiceServiceTestDB(t)
	contract, intent := seedStripeInvoicePurchase(t, 8106, 8206, "sub_livemode_mismatch")
	invoice := stripeInvoiceFixture("in_livemode_mismatch", "sub_livemode_mismatch")
	subscription := stripeSubscriptionFixture("sub_livemode_mismatch", map[string]string{
		"trade_no":         "sub_livemode_mismatch",
		"user_id":          "8106",
		"plan_id":          "8206",
		"contract_id":      strconv.FormatInt(contract.Id, 10),
		"change_intent_id": strconv.FormatInt(intent.Id, 10),
	})
	subscription.Livemode = true
	restore := replaceStripeInvoiceReconcilers(t, invoice, subscription)
	defer restore()

	_, err := ReconcilePaidInvoice(context.Background(), "in_livemode_mismatch")

	require.Error(t, err)
	require.True(t, IsPermanentPaidInvoiceError(err))
	require.Contains(t, err.Error(), "livemode mismatch")
}

func TestReconcilePaidInvoiceRejectsMissingStripeCustomer(t *testing.T) {
	setupSubscriptionInvoiceServiceTestDB(t)
	contract, intent := seedStripeInvoicePurchase(t, 8107, 8207, "sub_missing_customer")
	invoice := stripeInvoiceFixture("in_missing_customer", "sub_missing_customer")
	invoice.Customer = nil
	subscription := stripeSubscriptionFixture("sub_missing_customer", map[string]string{
		"trade_no":         "sub_missing_customer",
		"user_id":          "8107",
		"plan_id":          "8207",
		"contract_id":      strconv.FormatInt(contract.Id, 10),
		"change_intent_id": strconv.FormatInt(intent.Id, 10),
	})
	subscription.Customer = nil
	restore := replaceStripeInvoiceReconcilers(t, invoice, subscription)
	defer restore()

	_, err := ReconcilePaidInvoice(context.Background(), "in_missing_customer")

	require.Error(t, err)
	require.True(t, IsPermanentPaidInvoiceError(err))
	require.Contains(t, err.Error(), "customer")
	var entitlementCount int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("user_id = ?", 8107).Count(&entitlementCount).Error)
	require.Zero(t, entitlementCount)
	var bindingCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionProviderBinding{}).Where("user_id = ?", 8107).Count(&bindingCount).Error)
	require.Zero(t, bindingCount)
	var order model.SubscriptionOrder
	require.NoError(t, model.DB.First(&order, "trade_no = ?", "sub_missing_customer").Error)
	require.Equal(t, common.TopUpStatusPending, order.Status)
	var reloadedIntent model.SubscriptionChangeIntent
	require.NoError(t, model.DB.First(&reloadedIntent, "id = ?", intent.Id).Error)
	require.Equal(t, model.SubscriptionChangeIntentStatusAwaitingPayment, reloadedIntent.Status)
}

func TestReconcilePaidInvoiceValidatesLivemodeAgainstStripeKeyMode(t *testing.T) {
	testCases := []struct {
		name        string
		key         string
		livemode    bool
		expectError bool
	}{
		{name: "test key accepts test invoice", key: "sk_test_subscription", livemode: false},
		{name: "test key rejects live invoice", key: "sk_test_subscription", livemode: true, expectError: true},
		{name: "live key accepts live invoice", key: "sk_live_subscription", livemode: true},
		{name: "live key rejects test invoice", key: "sk_live_subscription", livemode: false, expectError: true},
	}
	for index, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setupSubscriptionInvoiceServiceTestDB(t)
			restoreKey := replaceStripeAPISecretForInvoiceTest(t, tc.key)
			defer restoreKey()
			userID := 8110 + index
			planID := 8210 + index
			tradeNo := fmt.Sprintf("sub_livemode_key_%d", index)
			contract, intent := seedStripeInvoicePurchase(t, userID, planID, tradeNo)
			invoice := stripeInvoiceFixture("in_"+tradeNo, tradeNo)
			invoice.Livemode = tc.livemode
			subscription := stripeSubscriptionFixture(tradeNo, map[string]string{
				"trade_no":         tradeNo,
				"user_id":          strconv.Itoa(userID),
				"plan_id":          strconv.Itoa(planID),
				"contract_id":      strconv.FormatInt(contract.Id, 10),
				"change_intent_id": strconv.FormatInt(intent.Id, 10),
			})
			subscription.Livemode = tc.livemode
			restoreReconcilers := replaceStripeInvoiceReconcilers(t, invoice, subscription)
			defer restoreReconcilers()

			_, err := ReconcilePaidInvoice(context.Background(), "in_"+tradeNo)

			if tc.expectError {
				require.Error(t, err)
				require.True(t, IsPermanentPaidInvoiceError(err))
				require.Contains(t, err.Error(), "livemode")
				var entitlementCount int64
				require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("user_id = ?", userID).Count(&entitlementCount).Error)
				require.Zero(t, entitlementCount)
				var bindingCount int64
				require.NoError(t, model.DB.Model(&model.SubscriptionProviderBinding{}).Where("user_id = ?", userID).Count(&bindingCount).Error)
				require.Zero(t, bindingCount)
				var order model.SubscriptionOrder
				require.NoError(t, model.DB.First(&order, "trade_no = ?", tradeNo).Error)
				require.Equal(t, common.TopUpStatusPending, order.Status)
				var reloadedIntent model.SubscriptionChangeIntent
				require.NoError(t, model.DB.First(&reloadedIntent, "id = ?", intent.Id).Error)
				require.Equal(t, model.SubscriptionChangeIntentStatusAwaitingPayment, reloadedIntent.Status)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestTerminatePendingStripePurchaseOnlyClearsMatchingLatestIntent(t *testing.T) {
	setupSubscriptionInvoiceServiceTestDB(t)
	contract, oldIntent := seedStripeInvoicePurchase(t, 8103, 8203, "sub_expired_old")
	newIntent := model.SubscriptionChangeIntent{
		ContractId:    contract.Id,
		UserId:        8103,
		RequestId:     "550e8400-e29b-41d4-a716-446655440101",
		Kind:          model.SubscriptionChangeIntentKindPurchase,
		PaymentMode:   model.SubscriptionPaymentModeStripeRecurring,
		Status:        model.SubscriptionChangeIntentStatusAwaitingPayment,
		ToPlanId:      8203,
		ChangeVersion: contract.ChangeVersion + 2,
	}
	require.NoError(t, model.DB.Create(&newIntent).Error)
	require.NoError(t, model.DB.Model(&model.UserSubscriptionContract{}).Where("id = ?", contract.Id).Update("latest_change_intent_id", newIntent.Id).Error)

	require.NoError(t, TerminatePendingStripePurchase(context.Background(), "sub_expired_old", model.SubscriptionChangeIntentStatusExpired))

	var reloadedContract model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&reloadedContract, "id = ?", contract.Id).Error)
	require.Equal(t, newIntent.Id, reloadedContract.LatestChangeIntentId)
	var expired model.SubscriptionChangeIntent
	require.NoError(t, model.DB.First(&expired, "id = ?", oldIntent.Id).Error)
	require.Equal(t, model.SubscriptionChangeIntentStatusExpired, expired.Status)
}

func TestStripeRecurringChangePlanCreatesAndReplaysCheckoutSession(t *testing.T) {
	setupSubscriptionInvoiceServiceTestDB(t)
	insertContractServiceUser(t, 8104, 0)
	plan := insertContractServicePlan(t, 8204, 1, 12.34, 1234)
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", plan.Id).Update("stripe_price_id", "price_invoice_plan").Error)
	restore := replaceStripeCheckoutCreator(t, "cs_replay", "https://checkout.example/session")
	defer restore()

	first, err := ChangeSubscriptionPlan(ChangePlanCommand{
		UserID:      8104,
		PlanID:      8204,
		PaymentMode: model.SubscriptionPaymentModeStripeRecurring,
		RequestID:   "550e8400-e29b-41d4-a716-446655440102",
	})
	require.NoError(t, err)
	second, err := ChangeSubscriptionPlan(ChangePlanCommand{
		UserID:      8104,
		PlanID:      8204,
		PaymentMode: model.SubscriptionPaymentModeStripeRecurring,
		RequestID:   "550e8400-e29b-41d4-a716-446655440102",
	})
	require.NoError(t, err)

	require.Equal(t, ChangePlanStatusCheckoutRequired, first.Status)
	require.Equal(t, "https://checkout.example/session", first.CheckoutURL)
	require.Equal(t, first.Intent.Id, second.Intent.Id)
	require.Equal(t, first.CheckoutURL, second.CheckoutURL)
	var orderCount int64
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("user_id = ?", 8104).Count(&orderCount).Error)
	require.Equal(t, int64(1), orderCount)
}

func TestStripeMinorUnitAmountForSubscriptionUsesDecimalRounding(t *testing.T) {
	actual, err := stripeMinorUnitAmountForSubscription(1.005, "USD")

	require.NoError(t, err)
	require.Equal(t, int64(101), actual)
}

func replaceStripeInvoiceReconcilers(t *testing.T, invoice *stripe.Invoice, subscription *stripe.Subscription) func() {
	t.Helper()
	originalInvoiceGetter := stripeInvoiceGetter
	originalSubscriptionGetter := stripeSubscriptionGetter
	stripeInvoiceGetter = func(ctx context.Context, invoiceID string) (*stripe.Invoice, error) {
		return invoice, nil
	}
	stripeSubscriptionGetter = func(ctx context.Context, subscriptionID string) (*stripe.Subscription, error) {
		return subscription, nil
	}
	return func() {
		stripeInvoiceGetter = originalInvoiceGetter
		stripeSubscriptionGetter = originalSubscriptionGetter
	}
}

func replaceStripeCheckoutCreator(t *testing.T, sessionID string, checkoutURL string) func() {
	t.Helper()
	originalCreator := stripeSubscriptionCheckoutCreator
	stripeSubscriptionCheckoutCreator = func(ctx context.Context, input StripeSubscriptionCheckoutInput) (*StripeSubscriptionCheckoutSession, error) {
		return &StripeSubscriptionCheckoutSession{
			ID:  sessionID,
			URL: checkoutURL,
		}, nil
	}
	return func() {
		stripeSubscriptionCheckoutCreator = originalCreator
	}
}

func replaceStripeAPISecretForInvoiceTest(t *testing.T, secret string) func() {
	t.Helper()
	originalSecret := setting.StripeApiSecret
	setting.StripeApiSecret = secret
	return func() {
		setting.StripeApiSecret = originalSecret
	}
}
