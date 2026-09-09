package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v86"
)

type catalogMigrationPreviewFakeStripe struct {
	prices       map[string]*stripe.Price
	subscription CatalogMigrationStripeSubscription
}

func TestCatalogMigrationPreviewDigestCanonicalOrderAndMaterialDrift(t *testing.T) {
	base := CatalogMigrationPreviewResult{
		RequestID: "operator-request-a",
		Mappings: []CatalogMigrationMappingExpectation{
			{SourcePlanID: 2, TargetPlanID: 20, ExpectedTierRank: 2, ExpectedBasePriceMinor: 3000, ExpectedCurrency: "USD"},
			{SourcePlanID: 1, TargetPlanID: 10, ExpectedTierRank: 1, ExpectedBasePriceMinor: 1000, ExpectedCurrency: "USD"},
		},
		Contracts: []CatalogMigrationContractPreviewResult{
			{ContractID: 22, Eligible: true, SourcePlanID: 2, TargetPlanID: 20, EntitlementFingerprint: "ent-22", BindingFingerprint: "bind-22", ProviderFingerprint: "provider-22", TargetPlanFingerprint: "target-22"},
			{ContractID: 11, Eligible: true, SourcePlanID: 1, TargetPlanID: 10, EntitlementFingerprint: "ent-11", BindingFingerprint: "bind-11", ProviderFingerprint: "provider-11", TargetPlanFingerprint: "target-11"},
		},
		Summary: CatalogMigrationPreviewSummary{Requested: 2, Eligible: 2, ByReason: map[string]int{CatalogMigrationReasonEligible: 2}},
	}
	first, err := catalogMigrationPreviewDigest(base)
	require.NoError(t, err)
	reordered := base
	reordered.RequestID = "operator-request-b"
	reordered.Mappings = slices.Clone(base.Mappings)
	reordered.Contracts = slices.Clone(base.Contracts)
	slices.Reverse(reordered.Mappings)
	slices.Reverse(reordered.Contracts)
	second, err := catalogMigrationPreviewDigest(reordered)
	require.NoError(t, err)
	require.Equal(t, first, second)

	mutations := []struct {
		name   string
		mutate func(*CatalogMigrationPreviewResult)
	}{
		{name: "entitlement", mutate: func(value *CatalogMigrationPreviewResult) { value.Contracts[0].EntitlementFingerprint = "ent-drift" }},
		{name: "source", mutate: func(value *CatalogMigrationPreviewResult) { value.Contracts[0].SourcePlanFingerprint = "source-drift" }},
		{name: "target", mutate: func(value *CatalogMigrationPreviewResult) { value.Contracts[0].TargetPlanFingerprint = "target-drift" }},
		{name: "binding", mutate: func(value *CatalogMigrationPreviewResult) { value.Contracts[0].BindingFingerprint = "binding-drift" }},
		{name: "provider", mutate: func(value *CatalogMigrationPreviewResult) { value.Contracts[0].ProviderFingerprint = "provider-drift" }},
	}
	for _, testCase := range mutations {
		t.Run(testCase.name, func(t *testing.T) {
			drifted := base
			drifted.Mappings = slices.Clone(base.Mappings)
			drifted.Contracts = slices.Clone(base.Contracts)
			testCase.mutate(&drifted)
			digest, digestErr := catalogMigrationPreviewDigest(drifted)
			require.NoError(t, digestErr)
			require.NotEqual(t, first, digest)
		})
	}
}

func (f catalogMigrationPreviewFakeStripe) GetPrice(_ context.Context, priceID string) (*stripe.Price, error) {
	price := f.prices[priceID]
	if price == nil {
		return nil, errors.New("price not found")
	}
	return price, nil
}

func (f catalogMigrationPreviewFakeStripe) GetSubscription(_ context.Context, _ string) (CatalogMigrationStripeSubscription, error) {
	return f.subscription, nil
}

func TestCatalogMigrationPreviewWalletIsReadOnlyAndDeterministic(t *testing.T) {
	setupSubscriptionContractServiceTestDB(t)
	contractID, request, sandbox := seedCatalogMigrationWalletPreview(t)

	var beforeContracts, beforeEntitlements, beforeIntents int64
	require.NoError(t, model.DB.Model(&model.UserSubscriptionContract{}).Count(&beforeContracts).Error)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Count(&beforeEntitlements).Error)
	require.NoError(t, model.DB.Model(&model.SubscriptionChangeIntent{}).Count(&beforeIntents).Error)

	first, err := previewSubscriptionCatalogMigrationWithDependencies(context.Background(), model.DB, nil, sandbox, request)
	require.NoError(t, err)
	require.Len(t, first.Contracts, 1)
	require.True(t, first.Contracts[0].Eligible)
	require.Equal(t, CatalogMigrationReasonEligible, first.Contracts[0].Reason)
	require.Len(t, first.CohortDigest, 64)
	require.NotEmpty(t, first.Contracts[0].CurrentSnapshotHash)
	require.NotEmpty(t, first.Contracts[0].TargetSnapshotHash)

	request.ContractIDs = []int64{contractID, contractID}
	request.Mappings = append([]CatalogMigrationMappingExpectation(nil), request.Mappings...)
	second, err := previewSubscriptionCatalogMigrationWithDependencies(context.Background(), model.DB, nil, sandbox, request)
	require.NoError(t, err)
	require.Equal(t, first.CohortDigest, second.CohortDigest)
	request.RequestID = "same-cohort-different-operator-request"
	third, err := previewSubscriptionCatalogMigrationWithDependencies(context.Background(), model.DB, nil, sandbox, request)
	require.NoError(t, err)
	require.Equal(t, first.CohortDigest, third.CohortDigest)

	var afterContracts, afterEntitlements, afterIntents int64
	require.NoError(t, model.DB.Model(&model.UserSubscriptionContract{}).Count(&afterContracts).Error)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Count(&afterEntitlements).Error)
	require.NoError(t, model.DB.Model(&model.SubscriptionChangeIntent{}).Count(&afterIntents).Error)
	require.Equal(t, beforeContracts, afterContracts)
	require.Equal(t, beforeEntitlements, afterEntitlements)
	require.Equal(t, beforeIntents, afterIntents)

	require.NoError(t, model.DB.Model(&model.UserSubscriptionContract{}).Where("id = ?", contractID).UpdateColumn("change_version", 2).Error)
	drifted, err := previewSubscriptionCatalogMigrationWithDependencies(context.Background(), model.DB, nil, sandbox, request)
	require.NoError(t, err)
	require.NotEqual(t, first.CohortDigest, drifted.CohortDigest)
}

func TestCatalogMigrationPreviewRejectsTargetStripePriceDrift(t *testing.T) {
	setupSubscriptionContractServiceTestDB(t)
	contractID, request, sandbox := seedCatalogMigrationWalletPreview(t)
	var contract model.UserSubscriptionContract
	var source, target model.SubscriptionPlan
	require.NoError(t, model.DB.First(&contract, "id = ?", contractID).Error)
	require.NoError(t, model.DB.First(&source, "id = ?", request.Mappings[0].SourcePlanID).Error)
	require.NoError(t, model.DB.First(&target, "id = ?", request.Mappings[0].TargetPlanID).Error)
	sourceSnapshot, err := recurringSnapshotFromPlanOnly(&source)
	require.NoError(t, err)
	encodedSource, err := EncodeRecurringPlanSnapshotV1(sourceSnapshot)
	require.NoError(t, err)
	binding := &model.SubscriptionProviderBinding{
		UserId: contract.UserId, PlanId: source.Id, ContractId: contract.Id,
		Provider: model.PaymentProviderStripe, ProviderSubscriptionId: "sub_preview",
		ProviderSubscriptionItemId: "si_preview", ProviderCustomerId: "cus_preview",
		ProviderPriceId: source.StripePriceId, ProviderStatus: "active",
		CurrentPeriodStart: contract.CurrentPeriodStart, CurrentPeriodEnd: contract.CurrentPeriodEnd,
		CurrentPlanSnapshot: encodedSource,
	}
	require.NoError(t, model.DB.Create(binding).Error)
	require.NoError(t, model.DB.Model(&contract).Updates(map[string]any{
		"payment_mode": model.SubscriptionPaymentModeStripeRecurring, "renewal_source": model.SubscriptionRenewalSourceProvider,
		"current_provider_binding_id": binding.Id,
	}).Error)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("id = ?", contract.CurrentEntitlementId).Updates(map[string]any{"provider_binding_id": binding.Id, "payment_mode": model.SubscriptionPaymentModeStripeRecurring}).Error)
	sandbox.StripeSecret = "rk_test_preview"
	sandbox.StripePublishableKey = "pk_test_preview"
	price := func(id string, amount int64) *stripe.Price {
		return &stripe.Price{ID: id, Active: true, Currency: stripe.CurrencyUSD, UnitAmount: amount, Type: stripe.PriceTypeRecurring, Recurring: &stripe.PriceRecurring{Interval: stripe.PriceRecurringIntervalMonth, IntervalCount: 1}}
	}
	baseInventory := catalogMigrationPreviewFakeStripe{
		prices: map[string]*stripe.Price{
			source.StripePriceId: price(source.StripePriceId, 1000),
			target.StripePriceId: price(target.StripePriceId, 1000),
		},
		subscription: CatalogMigrationStripeSubscription{
			ID: binding.ProviderSubscriptionId, CustomerID: binding.ProviderCustomerId,
			ItemIDs: []string{binding.ProviderSubscriptionItemId}, PriceIDs: []string{binding.ProviderPriceId},
			Status: "active", CurrentPeriodStart: binding.CurrentPeriodStart, CurrentPeriodEnd: binding.CurrentPeriodEnd,
		},
	}
	for _, testCase := range []struct {
		name   string
		mutate func(*stripe.Price)
	}{
		{name: "amount", mutate: func(value *stripe.Price) { value.UnitAmount = 999 }},
		{name: "currency", mutate: func(value *stripe.Price) { value.Currency = stripe.CurrencyEUR }},
		{name: "interval", mutate: func(value *stripe.Price) { value.Recurring.Interval = stripe.PriceRecurringIntervalYear }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			inventory := baseInventory
			inventory.prices = map[string]*stripe.Price{
				source.StripePriceId: price(source.StripePriceId, 1000),
				target.StripePriceId: price(target.StripePriceId, 1000),
			}
			testCase.mutate(inventory.prices[target.StripePriceId])
			result, previewErr := previewSubscriptionCatalogMigrationWithDependencies(context.Background(), model.DB, inventory, sandbox, request)
			require.NoError(t, previewErr)
			require.False(t, result.Contracts[0].Eligible)
			require.Equal(t, CatalogMigrationReasonTargetPlanMismatch, result.Contracts[0].Reason)
		})
	}
}

func TestCatalogMigrationPreviewUsesStableGuardAndMappingReasons(t *testing.T) {
	setupSubscriptionContractServiceTestDB(t)
	contractID, request, sandbox := seedCatalogMigrationWalletPreview(t)

	closed := sandbox
	closed.AllowedContractIDs = []int64{contractID + 1}
	result, err := previewSubscriptionCatalogMigrationWithDependencies(context.Background(), model.DB, nil, closed, request)
	require.NoError(t, err)
	require.Equal(t, CatalogMigrationReasonContractNotAllowlisted, result.Contracts[0].Reason)

	request.Mappings = append(request.Mappings, request.Mappings[0])
	result, err = previewSubscriptionCatalogMigrationWithDependencies(context.Background(), model.DB, nil, sandbox, request)
	require.NoError(t, err)
	require.Equal(t, CatalogMigrationReasonMappingAmbiguous, result.Contracts[0].Reason)
}

func TestCatalogMigrationPreviewRejectsEmptyExplicitCohort(t *testing.T) {
	setupSubscriptionContractServiceTestDB(t)
	_, request, sandbox := seedCatalogMigrationWalletPreview(t)
	request.ContractIDs = nil
	_, err := previewSubscriptionCatalogMigrationWithDependencies(context.Background(), model.DB, nil, sandbox, request)
	require.ErrorContains(t, err, "contract allowlist is empty")
}

func seedCatalogMigrationWalletPreview(t *testing.T) (int64, CatalogMigrationPreviewRequest, CatalogMigrationSandboxConfig) {
	t.Helper()
	rank := 1
	source := &model.SubscriptionPlan{
		Title: "Legacy Go", PriceAmount: 10, Currency: "USD", DurationUnit: model.SubscriptionDurationMonth,
		DurationValue: 1, Enabled: false, TierRank: &rank, StripePriceId: "price_test_legacy_go",
		TotalAmount: 5_000_000, QuotaResetPeriod: model.SubscriptionResetMonthly,
	}
	target := &model.SubscriptionPlan{
		Title: "Go", PriceAmount: 10, Currency: "USD", DurationUnit: model.SubscriptionDurationMonth,
		DurationValue: 1, Enabled: true, TierRank: &rank, StripePriceId: "price_test_go",
		TotalAmount: 13 * 100, QuotaResetPeriod: model.SubscriptionResetMonthly,
	}
	require.NoError(t, model.DB.Create(source).Error)
	require.NoError(t, model.DB.Create(target).Error)

	window := int64(0)
	currentSlot := 1
	entitlement := &model.UserSubscription{
		UserId: 95101, PlanId: source.Id, ContractId: 95201, CurrentSlot: &currentSlot,
		AmountTotal: source.TotalAmount, Window5hAmount: &window, WindowWeekAmount: &window,
		StartTime: 1_700_000_000, EndTime: 1_702_678_400, Status: "active",
		PaymentMode: model.SubscriptionPaymentModeBalanceOnePeriod,
	}
	require.NoError(t, model.DB.Create(entitlement).Error)
	contract := &model.UserSubscriptionContract{
		Id: entitlement.ContractId, UserId: entitlement.UserId, Status: model.SubscriptionContractStatusActive,
		PaymentMode: model.SubscriptionPaymentModeBalanceOnePeriod, RenewalSource: model.SubscriptionRenewalSourceWallet,
		RenewalStatus: model.SubscriptionRenewalStatusEnabled, CurrentPlanId: source.Id,
		CurrentEntitlementId: entitlement.Id, CurrentPeriodStart: entitlement.StartTime, CurrentPeriodEnd: entitlement.EndTime,
		ChangeVersion: 1,
	}
	require.NoError(t, model.DB.Create(contract).Error)

	mapping := CatalogMigrationMappingExpectation{
		SourcePlanID: source.Id, TargetPlanID: target.Id, ExpectedTierRank: rank,
		ExpectedBasePriceMinor: 1000, ExpectedCurrency: "USD", ExpectedDurationUnit: model.SubscriptionDurationMonth,
		ExpectedDurationValue: 1, ExpectedQuotaResetPeriod: model.SubscriptionResetMonthly,
		ExpectedTargetTotalAmount: target.TotalAmount,
	}
	request := CatalogMigrationPreviewRequest{RequestID: "catalog-preview-wallet", Mappings: []CatalogMigrationMappingExpectation{mapping}, ContractIDs: []int64{contract.Id}}
	sandbox := CatalogMigrationSandboxConfig{DeploymentEnvironment: "staging", ServiceName: "newapi-staging", FeatureEnabled: true, AllowedContractIDs: []int64{contract.Id}}
	return contract.Id, request, sandbox
}
