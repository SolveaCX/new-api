package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v86"
)

func TestPaidRenewalKeepsPurchasedPriceAfterCatalogEdit(t *testing.T) {
	for _, catalogPrice := range []string{"price_new_catalog", ""} {
		t.Run("catalog_"+catalogPrice, func(t *testing.T) {
			setupSubscriptionInvoiceServiceTestDB(t)
			contract, binding, oldGrant := seedStripeRenewalContract(t, 9401, 9402, "sub_bound_price")
			order := model.SubscriptionOrder{
				UserId: binding.UserId, PlanId: binding.PlanId, TradeNo: "purchased-price-order",
				PaymentProvider: model.PaymentProviderStripe, Status: common.TopUpStatusSuccess,
				PlanSnapshot: `{"plan_id":9402,"stripe_price_id":"price_invoice_plan","price_amount":12.34,"currency":"USD","total_amount":1234}`,
			}
			require.NoError(t, model.DB.Create(&order).Error)
			require.NoError(t, model.DB.Model(&binding).Update("initial_order_id", order.Id).Error)
			require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", binding.PlanId).
				Update("stripe_price_id", catalogPrice).Error)
			invoice := stripeInvoiceFixture("in_bound_price", binding.ProviderSubscriptionId)
			invoice.Lines.Data[0].Period = &stripe.Period{Start: oldGrant.EndTime, End: oldGrant.EndTime + 2592000}
			subscription := stripeSubscriptionFixture(binding.ProviderSubscriptionId, map[string]string{})
			setStripeSubscriptionCurrentPeriod(subscription, oldGrant.EndTime, oldGrant.EndTime+2592000)
			t.Cleanup(replaceStripeInvoiceReconcilers(t, invoice, subscription))

			result, err := ReconcilePaidInvoice(context.Background(), invoice.ID)
			require.NoError(t, err)
			require.True(t, result.Applied)
			require.Equal(t, binding.PlanId, result.Entitlement.PlanId)
			require.Equal(t, oldGrant.AmountTotal, result.Entitlement.AmountTotal)
			require.Zero(t, result.Entitlement.AmountUsed)
			replay, err := ReconcilePaidInvoice(context.Background(), invoice.ID)
			require.NoError(t, err)
			require.False(t, replay.Applied)
			var grantCount int64
			require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("contract_id = ?", contract.Id).Count(&grantCount).Error)
			require.Equal(t, int64(2), grantCount)
			var persisted model.SubscriptionProviderBinding
			require.NoError(t, model.DB.First(&persisted, binding.Id).Error)
			require.Equal(t, "price_invoice_plan", persisted.ProviderPriceId)
		})
	}
}

func TestRenewalPriceAuthorityPreservesProductGuard(t *testing.T) {
	for _, tc := range []struct {
		name         string
		boundPrice   string
		catalogPrice string
		invoicePrice string
		frozenPrice  string
		pending      bool
		wantError    bool
	}{
		{"purchased_price_survives_catalog_edit", "price_old", "price_new", "price_old", "price_old", false, false},
		{"catalog_cannot_replace_bound_price", "price_old", "price_new", "price_new", "price_old", false, true},
		{"synced_binding_cannot_override_purchase", "price_new", "price_new", "price_new", "price_old", false, true},
		{"legacy_bound_price_is_not_purchase_authority", "price_new", "price_old", "price_new", "", false, true},
		{"legacy_requires_catalog_match", "price_old", "price_new", "price_old", "", false, true},
		{"legacy_matching_price_accepted", "price_old", "price_old", "price_old", "", false, false},
		{"missing_bound_price", "", "price_new", "price_new", "price_new", false, true},
		{"pending_change_uses_target", "price_old", "price_new", "price_new", "price_old", true, false},
		{"pending_change_rejects_old_price", "price_old", "price_new", "price_old", "price_old", true, true},
		{"pending_change_rejects_unknown_price", "price_old", "price_new", "price_other", "price_old", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binding := &model.SubscriptionProviderBinding{Id: 2, ContractId: 1, UserId: 3, PlanId: 4, ProviderCustomerId: "cus_owner", ProviderPriceId: tc.boundPrice}
			contract := &model.UserSubscriptionContract{Id: 1, UserId: 3, CurrentProviderBindingId: 2, PaymentMode: model.SubscriptionPaymentModeStripeRecurring}
			plan := &model.SubscriptionPlan{Id: 4, StripePriceId: tc.catalogPrice}
			if tc.pending {
				plan.Id = 5
				contract.PendingPlanId = 5
				contract.PendingEffectiveAt = 100
			}
			facts := stripeInvoiceCommonFacts{CustomerID: "cus_owner", PriceID: tc.invoicePrice, Quantity: 1, PeriodStart: 100}
			snapshot := recurringInvoicePlanSnapshot{Found: tc.frozenPrice != "", Snapshot: purchasePlanSnapshot{PlanID: 4, StripePriceID: tc.frozenPrice}}
			err := validateRenewalInvoiceFactsTx(nil, facts, binding, contract, plan, &model.User{}, snapshot)
			if tc.wantError {
				require.ErrorContains(t, err, "Stripe price mismatch")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPaidRenewalRejectsPassivelySyncedPriceChange(t *testing.T) {
	for _, pendingDowngrade := range []bool{false, true} {
		for _, hasPriceSnapshot := range []bool{false, true} {
			name := "ordinary"
			if pendingDowngrade {
				name = "pending_downgrade"
			}
			if hasPriceSnapshot {
				name += "_with_snapshot"
			}
			t.Run(name, func(t *testing.T) {
				setupSubscriptionInvoiceServiceTestDB(t)
				contract, binding, oldGrant := seedStripeRenewalContract(t, 9411, 9412, "sub_synced_price")
				if hasPriceSnapshot {
					order := model.SubscriptionOrder{
						UserId: binding.UserId, PlanId: binding.PlanId, TradeNo: "synced-price-order",
						PaymentProvider: model.PaymentProviderStripe, Status: common.TopUpStatusSuccess,
						PlanSnapshot: `{"plan_id":9412,"stripe_price_id":"price_invoice_plan","price_amount":12.34,"currency":"USD","total_amount":1234}`,
					}
					require.NoError(t, model.DB.Create(&order).Error)
					require.NoError(t, model.DB.Model(&binding).Update("initial_order_id", order.Id).Error)
				}
				if pendingDowngrade {
					target := model.SubscriptionPlan{Id: 9413, Title: "Target", Enabled: true, StripePriceId: "price_target_edited", TotalAmount: 100}
					require.NoError(t, model.DB.Create(&target).Error)
					intent := model.SubscriptionChangeIntent{
						ContractId: contract.Id, UserId: binding.UserId, RequestId: "pending-synced-price",
						Kind: model.SubscriptionChangeIntentKindDowngrade, Status: model.SubscriptionChangeIntentStatusScheduled,
						FromPlanId: binding.PlanId, ToPlanId: target.Id, ProviderBindingId: binding.Id,
					}
					require.NoError(t, model.DB.Create(&intent).Error)
					require.NoError(t, model.DB.Model(&contract).Updates(map[string]interface{}{
						"pending_plan_id": target.Id, "pending_effective_at": oldGrant.EndTime, "latest_change_intent_id": intent.Id,
					}).Error)
				}
				_, err := model.ApplyProviderSubscriptionSnapshot(binding.Id, model.ProviderSubscriptionSnapshot{
					ProviderSubscriptionId: binding.ProviderSubscriptionId, ProviderCustomerId: binding.ProviderCustomerId,
					ProviderPriceId: "price_provider_changed", ProviderStatus: "active",
					CurrentPeriodStart: binding.CurrentPeriodStart, CurrentPeriodEnd: binding.CurrentPeriodEnd,
				})
				require.NoError(t, err)
				var synced model.SubscriptionProviderBinding
				require.NoError(t, model.DB.First(&synced, binding.Id).Error)
				require.Equal(t, "price_provider_changed", synced.ProviderPriceId, "observed provider price is mutable, not purchased authority")
				require.Equal(t, binding.PlanId, synced.PlanId)
				invoice := stripeInvoiceFixture("in_synced_price", binding.ProviderSubscriptionId)
				subscription := stripeSubscriptionFixture(binding.ProviderSubscriptionId, map[string]string{})
				setStripeInvoiceFixtureAmountAndPrice(invoice, subscription, 100, stripe.CurrencyUSD, "price_provider_changed")
				invoice.Lines.Data[0].Period = &stripe.Period{Start: oldGrant.EndTime, End: oldGrant.EndTime + 2592000}
				setStripeSubscriptionCurrentPeriod(subscription, oldGrant.EndTime, oldGrant.EndTime+2592000)
				t.Cleanup(replaceStripeInvoiceReconcilers(t, invoice, subscription))
				_, err = ReconcilePaidInvoice(context.Background(), invoice.ID)
				require.ErrorContains(t, err, "Stripe price mismatch")
				var grantCount int64
				require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("contract_id = ?", contract.Id).Count(&grantCount).Error)
				require.Equal(t, int64(1), grantCount)
				var unchanged model.UserSubscription
				require.NoError(t, model.DB.First(&unchanged, oldGrant.Id).Error)
				require.Equal(t, oldGrant.AmountUsed, unchanged.AmountUsed)
			})
		}
	}
}

func TestUpgradePriceAuthorityUsesPurchaseSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name          string
		snapshotPrice string
		catalogPrice  string
		invoicePrice  string
		wantError     bool
	}{
		{"frozen_price_survives_catalog_edit", "price_bought", "price_new", "price_bought", false},
		{"frozen_price_survives_catalog_removal", "price_bought", "", "price_bought", false},
		{"catalog_cannot_override_snapshot", "price_bought", "price_new", "price_new", true},
		{"unknown_price_rejected", "price_bought", "price_new", "price_other", true},
		{"legacy_snapshot_uses_catalog", "", "price_current", "price_current", false},
		{"legacy_snapshot_rejects_unknown_price", "", "price_current", "price_other", true},
		{"missing_price_rejected", "", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binding := &model.SubscriptionProviderBinding{Id: 2, ContractId: 1, UserId: 3, ProviderSubscriptionId: "sub_owner", ProviderSubscriptionItemId: "si_owner", ProviderCustomerId: "cus_owner"}
			contract := &model.UserSubscriptionContract{Id: 1, UserId: 3, CurrentProviderBindingId: 2, PaymentMode: model.SubscriptionPaymentModeStripeRecurring}
			intent := &model.SubscriptionChangeIntent{ToPlanId: 5}
			plan := &model.SubscriptionPlan{Id: 5, Enabled: true, StripePriceId: tc.catalogPrice}
			facts := paidInvoiceFacts{SubscriptionID: "sub_owner", SubscriptionItemID: "si_owner", CustomerID: "cus_owner", PriceID: tc.invoicePrice, Quantity: 1}
			snapshot := recurringInvoicePlanSnapshot{Found: true, Snapshot: purchasePlanSnapshot{PlanID: 5, StripePriceID: tc.snapshotPrice}}
			err := validateStripeUpgradePaidInvoiceFacts(facts, intent, contract, binding, plan, &model.User{}, snapshot)
			if tc.wantError {
				require.ErrorContains(t, err, "Stripe price mismatch")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
