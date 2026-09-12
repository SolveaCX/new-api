package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v86"
)

func TestRecurringPlanSnapshotV1RoundTripAndFingerprint(t *testing.T) {
	plan, price := recurringSnapshotFixture()
	snapshot, err := FreezeRecurringPlanSnapshotV1(plan, price)
	require.NoError(t, err)
	require.Equal(t, int64(1000), snapshot.BasePriceMinor)

	encoded, err := EncodeRecurringPlanSnapshotV1(snapshot)
	require.NoError(t, err)
	require.NotContains(t, encoded, "price_amount")
	parsed, err := DecodeRecurringPlanSnapshotV1(encoded)
	require.NoError(t, err)
	require.Equal(t, snapshot, parsed)

	first, err := RecurringPlanSnapshotV1Fingerprint(snapshot)
	require.NoError(t, err)
	second, err := RecurringPlanSnapshotV1Fingerprint(parsed)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Len(t, first, 64)
}

func TestDecodeRecurringPlanSnapshotV1RejectsMalformedOrUnexpectedData(t *testing.T) {
	plan, price := recurringSnapshotFixture()
	snapshot, err := FreezeRecurringPlanSnapshotV1(plan, price)
	require.NoError(t, err)
	valid, err := EncodeRecurringPlanSnapshotV1(snapshot)
	require.NoError(t, err)

	tests := []struct {
		name string
		raw  string
	}{
		{name: "missing", raw: ""},
		{name: "malformed", raw: "{"},
		{name: "unknown field", raw: strings.TrimSuffix(valid, "}") + `,"price_amount":13}`},
		{name: "trailing value", raw: valid + `{}`},
		{name: "unsupported version", raw: strings.Replace(valid, `"version":1`, `"version":2`, 1)},
		{name: "negative entitlement", raw: strings.Replace(valid, `"total_amount":6500000`, `"total_amount":-1`, 1)},
		{name: "non monthly", raw: strings.Replace(valid, `"duration_unit":"month"`, `"duration_unit":"year"`, 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeRecurringPlanSnapshotV1(test.raw)
			require.Error(t, err)
		})
	}
}

func TestRecurringPlanSnapshotV1RejectsPlanAndStripeDrift(t *testing.T) {
	plan, price := recurringSnapshotFixture()
	snapshot, err := FreezeRecurringPlanSnapshotV1(plan, price)
	require.NoError(t, err)

	t.Run("plan id", func(t *testing.T) {
		changed := snapshot
		changed.PlanID++
		require.ErrorContains(t, ValidateRecurringPlanSnapshotV1AgainstPlan(changed, plan), "does not match")
	})
	t.Run("price id", func(t *testing.T) {
		changed := snapshot
		changed.StripePriceID = "price_other"
		require.ErrorContains(t, ValidateRecurringPlanSnapshotV1AgainstStripePrice(changed, price), "id does not match")
	})
	t.Run("currency", func(t *testing.T) {
		changed := *price
		changed.Currency = stripe.CurrencyEUR
		require.ErrorContains(t, ValidateRecurringPlanSnapshotV1AgainstStripePrice(snapshot, &changed), "currency")
	})
	t.Run("amount", func(t *testing.T) {
		changed := *price
		changed.UnitAmount++
		require.ErrorContains(t, ValidateRecurringPlanSnapshotV1AgainstStripePrice(snapshot, &changed), "amount")
	})
	t.Run("live", func(t *testing.T) {
		changed := *price
		changed.Livemode = true
		require.ErrorContains(t, ValidateRecurringPlanSnapshotV1AgainstStripePrice(snapshot, &changed), "test mode")
	})
	t.Run("interval", func(t *testing.T) {
		changed := *price
		recurring := *price.Recurring
		recurring.Interval = stripe.PriceRecurringIntervalYear
		changed.Recurring = &recurring
		require.ErrorContains(t, ValidateRecurringPlanSnapshotV1AgainstStripePrice(snapshot, &changed), "monthly")
	})
}

func TestFreezeLegacyRecurringPlanSnapshotV1RequiresExactOwnershipAndMoney(t *testing.T) {
	plan, price := recurringSnapshotFixture()
	legacyJSON, err := subscriptionPurchasePlanSnapshot(plan)
	require.NoError(t, err)
	order := &model.SubscriptionOrder{
		Id:              101,
		UserId:          42,
		PlanId:          plan.Id,
		UnitPrice:       plan.PriceAmount,
		PaymentCurrency: plan.Currency,
		PlanSnapshot:    legacyJSON,
		Status:          common.TopUpStatusSuccess,
		CompleteTime:    100,
		PaymentProvider: model.PaymentProviderStripe,
		PaymentMethod:   model.PaymentMethodStripe,
		PurchaseIntent:  model.SubscriptionChangeIntentKindPurchase,
	}
	binding := &model.SubscriptionProviderBinding{
		Id:                     201,
		UserId:                 order.UserId,
		PlanId:                 order.PlanId,
		InitialOrderId:         order.Id,
		ProviderPriceId:        plan.StripePriceId,
		ProviderSubscriptionId: "sub_legacy",
		ProviderCustomerId:     "cus_legacy",
	}

	snapshot, err := FreezeLegacyRecurringPlanSnapshotV1(order, binding, plan, price)
	require.NoError(t, err)
	require.Equal(t, int64(1000), snapshot.BasePriceMinor)

	t.Run("binding plan changed", func(t *testing.T) {
		changed := *binding
		changed.PlanId++
		_, err := FreezeLegacyRecurringPlanSnapshotV1(order, &changed, plan, price)
		require.ErrorContains(t, err, "ownership mismatch")
	})
	t.Run("order amount changed", func(t *testing.T) {
		changed := *order
		changed.UnitPrice = 12
		_, err := FreezeLegacyRecurringPlanSnapshotV1(&changed, binding, plan, price)
		require.ErrorContains(t, err, "amount mismatch")
	})
	t.Run("binding price changed", func(t *testing.T) {
		changed := *binding
		changed.ProviderPriceId = "price_other"
		_, err := FreezeLegacyRecurringPlanSnapshotV1(order, &changed, plan, price)
		require.ErrorContains(t, err, "identity mismatch")
	})
	t.Run("pending order", func(t *testing.T) {
		changed := *order
		changed.Status = common.TopUpStatusPending
		_, err := FreezeLegacyRecurringPlanSnapshotV1(&changed, binding, plan, price)
		require.ErrorContains(t, err, "completed Stripe purchase")
	})
	t.Run("non Stripe order", func(t *testing.T) {
		changed := *order
		changed.PaymentProvider = model.PaymentProviderPaddle
		_, err := FreezeLegacyRecurringPlanSnapshotV1(&changed, binding, plan, price)
		require.ErrorContains(t, err, "completed Stripe purchase")
	})
	t.Run("provider payload subscription mismatch", func(t *testing.T) {
		changed := *order
		changed.ProviderPayload = "invoice_id=in_test;subscription_id=sub_other"
		_, err := FreezeLegacyRecurringPlanSnapshotV1(&changed, binding, plan, price)
		require.ErrorContains(t, err, "subscription identity mismatch")
	})
}

func TestCatalogMigrationSandboxGuards(t *testing.T) {
	config := CatalogMigrationSandboxConfig{
		DeploymentEnvironment: "staging",
		ServiceName:           "newapi-staging",
		FeatureEnabled:        true,
		AllowedContractIDs:    []int64{17},
		StripeSecret:          "rk_test_catalog",
		StripePublishableKey:  "pk_test_catalog",
	}
	facts := CatalogMigrationStripeSandboxFacts{
		ContractID:             17,
		BindingSubscriptionID:  "sub_current",
		SubscriptionID:         "sub_current",
		BindingCustomerID:      "cus_current",
		SubscriptionCustomerID: "cus_current",
		BindingCurrentPriceID:  "price_old",
		SubscriptionPriceID:    "price_old",
		ExpectedTargetPriceID:  "price_new",
		TargetPriceID:          "price_new",
	}
	require.NoError(t, ValidateCatalogMigrationCommonSandbox(config, 17))
	require.NoError(t, ValidateCatalogMigrationStripeSandbox(config, facts))

	commonCases := []struct {
		name   string
		mutate func(*CatalogMigrationSandboxConfig)
	}{
		{name: "wrong environment", mutate: func(v *CatalogMigrationSandboxConfig) { v.DeploymentEnvironment = "production" }},
		{name: "wrong service", mutate: func(v *CatalogMigrationSandboxConfig) { v.ServiceName = "newapi-router" }},
		{name: "disabled", mutate: func(v *CatalogMigrationSandboxConfig) { v.FeatureEnabled = false }},
		{name: "empty allowlist", mutate: func(v *CatalogMigrationSandboxConfig) { v.AllowedContractIDs = nil }},
	}
	for _, test := range commonCases {
		t.Run(test.name, func(t *testing.T) {
			changed := config
			test.mutate(&changed)
			require.Error(t, ValidateCatalogMigrationCommonSandbox(changed, 17))
		})
	}

	t.Run("wallet needs no Stripe facts", func(t *testing.T) {
		walletConfig := config
		walletConfig.StripeSecret = ""
		walletConfig.StripePublishableKey = ""
		require.NoError(t, ValidateCatalogMigrationCommonSandbox(walletConfig, 17))
	})

	stripeCases := []struct {
		name         string
		mutateConfig func(*CatalogMigrationSandboxConfig)
		mutateFacts  func(*CatalogMigrationStripeSandboxFacts)
	}{
		{name: "live secret", mutateConfig: func(v *CatalogMigrationSandboxConfig) { v.StripeSecret = "sk_live_catalog" }},
		{name: "live publishable", mutateConfig: func(v *CatalogMigrationSandboxConfig) { v.StripePublishableKey = "pk_live_catalog" }},
		{name: "live binding", mutateFacts: func(v *CatalogMigrationStripeSandboxFacts) { v.BindingLivemode = true }},
		{name: "live subscription", mutateFacts: func(v *CatalogMigrationStripeSandboxFacts) { v.SubscriptionLivemode = true }},
		{name: "live current price", mutateFacts: func(v *CatalogMigrationStripeSandboxFacts) { v.CurrentPriceLivemode = true }},
		{name: "live target price", mutateFacts: func(v *CatalogMigrationStripeSandboxFacts) { v.TargetPriceLivemode = true }},
		{name: "subscription mismatch", mutateFacts: func(v *CatalogMigrationStripeSandboxFacts) { v.SubscriptionID = "sub_other" }},
		{name: "customer mismatch", mutateFacts: func(v *CatalogMigrationStripeSandboxFacts) { v.SubscriptionCustomerID = "cus_other" }},
		{name: "current price mismatch", mutateFacts: func(v *CatalogMigrationStripeSandboxFacts) { v.SubscriptionPriceID = "price_other" }},
		{name: "target price mismatch", mutateFacts: func(v *CatalogMigrationStripeSandboxFacts) { v.TargetPriceID = "price_other" }},
	}
	for _, test := range stripeCases {
		t.Run(test.name, func(t *testing.T) {
			changedConfig := config
			changedFacts := facts
			if test.mutateConfig != nil {
				test.mutateConfig(&changedConfig)
			}
			if test.mutateFacts != nil {
				test.mutateFacts(&changedFacts)
			}
			require.Error(t, ValidateCatalogMigrationStripeSandbox(changedConfig, changedFacts))
		})
	}
}

func recurringSnapshotFixture() (*model.SubscriptionPlan, *stripe.Price) {
	plan := &model.SubscriptionPlan{
		Id:                      5,
		Title:                   "Go",
		PriceAmount:             10,
		Currency:                "USD",
		DurationUnit:            model.SubscriptionDurationMonth,
		DurationValue:           1,
		StripePriceId:           "price_new_go",
		TotalAmount:             6500000,
		MediaCreditsMonthly:     25,
		Window5hAmount:          0,
		WindowWeekAmount:        0,
		QuotaResetPeriod:        model.SubscriptionResetNever,
		QuotaResetCustomSeconds: 0,
		UpgradeGroup:            "go",
	}
	price := &stripe.Price{
		ID:         plan.StripePriceId,
		Active:     true,
		Currency:   stripe.CurrencyUSD,
		Livemode:   false,
		Type:       stripe.PriceTypeRecurring,
		UnitAmount: 1000,
		Recurring: &stripe.PriceRecurring{
			Interval:      stripe.PriceRecurringIntervalMonth,
			IntervalCount: 1,
		},
	}
	return plan, price
}
