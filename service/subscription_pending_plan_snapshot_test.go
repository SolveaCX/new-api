package service

import (
	"context"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v86"
)

func TestAdaptivePricingDowngradeUsesTargetGrantSnapshotOnLaterRenewal(t *testing.T) {
	setupSubscriptionInvoiceServiceTestDB(t)
	contract, purchase := seedStripeInvoicePurchase(t, 9199, 9299, "sub_downgrade_snapshot")
	originalSnapshot, err := common.Marshal(purchasePlanSnapshot{
		PlanID: 9299, Title: "Original", PriceAmount: 12.34, Currency: "USD",
		StripePriceID: "price_invoice_plan", TotalAmount: 1234,
		MediaCreditsMonthly: 100, Window5hAmount: 200, WindowWeekAmount: 900,
		UpgradeGroup: "original_group",
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).
		Where("trade_no = ?", "sub_downgrade_snapshot").Update("plan_snapshot", string(originalSnapshot)).Error)
	invoice := stripeInvoiceFixture("in_original_snapshot", "sub_downgrade_snapshot")
	subscription := stripeSubscriptionFixture("sub_downgrade_snapshot", map[string]string{
		"trade_no": "sub_downgrade_snapshot", "user_id": "9199", "plan_id": "9299",
		"contract_id": strconv.FormatInt(contract.Id, 10), "change_intent_id": strconv.FormatInt(purchase.Id, 10),
	})
	t.Cleanup(replaceStripeInvoiceReconcilers(t, invoice, subscription))
	first, err := ReconcilePaidInvoice(context.Background(), invoice.ID)
	require.NoError(t, err)
	require.True(t, first.Applied)
	require.Positive(t, first.Binding.InitialOrderId)
	originalOrderID := first.Binding.InitialOrderId
	boundary := first.Entitlement.EndTime

	target := model.SubscriptionPlan{
		Id: 9300, Title: "Lower Plan", PriceAmount: 9.99, Currency: "USD",
		DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true,
		TotalAmount: 700, MediaCreditsMonthly: 11, Window5hAmount: 15, WindowWeekAmount: 40,
		UpgradeGroup: "lower_group", StripePriceId: "price_lower_snapshot",
	}
	require.NoError(t, model.DB.Create(&target).Error)
	downgrade := model.SubscriptionChangeIntent{
		ContractId: contract.Id, UserId: 9199, RequestId: "scheduled-snapshot-downgrade",
		Kind: model.SubscriptionChangeIntentKindDowngrade, PaymentMode: model.SubscriptionPaymentModeStripeRecurring,
		Status: model.SubscriptionChangeIntentStatusScheduled, FromPlanId: 9299, ToPlanId: target.Id,
		ProviderBindingId: first.Binding.Id, EffectiveAt: boundary,
	}
	require.NoError(t, model.DB.Create(&downgrade).Error)
	require.NoError(t, model.DB.Model(&model.UserSubscriptionContract{}).Where("id = ?", contract.Id).
		Updates(map[string]interface{}{"pending_plan_id": target.Id, "pending_effective_at": boundary, "latest_change_intent_id": downgrade.Id}).Error)
	invoice.ID = "in_downgrade_snapshot"
	setStripeInvoiceFixtureAmountAndPrice(invoice, subscription, 4990, stripe.CurrencyBRL, target.StripePriceId)
	invoice.Lines.Data[0].Period = &stripe.Period{Start: boundary, End: boundary + 2592000}
	setStripeSubscriptionCurrentPeriod(subscription, boundary, boundary+2592000)
	downgraded, err := ReconcilePaidInvoice(context.Background(), invoice.ID)
	require.NoError(t, err)
	require.True(t, downgraded.Applied)
	assertTargetGrant := func(grant *model.UserSubscription) {
		t.Helper()
		require.Equal(t, target.Id, grant.PlanId)
		require.Equal(t, int64(700), grant.AmountTotal)
		require.Equal(t, int64(11), grant.MediaCreditsTotal)
		require.NotNil(t, grant.Window5hAmount)
		require.NotNil(t, grant.WindowWeekAmount)
		require.Equal(t, int64(15), *grant.Window5hAmount)
		require.Equal(t, int64(40), *grant.WindowWeekAmount)
		require.Equal(t, "lower_group", grant.UpgradeGroup)
	}
	assertTargetGrant(downgraded.Entitlement)
	var activeBinding model.SubscriptionProviderBinding
	require.NoError(t, model.DB.First(&activeBinding, first.Binding.Id).Error)
	limitsSnapshot, err := recurringPlanSnapshotFromBindingTx(model.DB, &activeBinding)
	require.NoError(t, err)
	require.False(t, limitsSnapshot.Found, "entitlement limits must not masquerade as a payment/price snapshot")
	require.NotNil(t, limitsSnapshot.GrantLimits)
	canonicalPrice, err := canonicalRenewalUSDMinor(&target, limitsSnapshot)
	require.NoError(t, err)
	require.Equal(t, int64(999), canonicalPrice, "discount pricing must use the target plan, not zero or the original plan price")

	// The original payment remains immutable for attribution. Later edits to
	// the catalog must not override the limits frozen by the downgrade grant.
	require.NoError(t, model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", target.Id).
		Updates(map[string]interface{}{"total_amount": 9999, "media_credits_monthly": 999, "window_5h_amount": 999, "window_week_amount": 999, "upgrade_group": "edited_group"}).Error)
	invoice.ID = "in_after_downgrade_snapshot"
	invoice.Lines.Data[0].Period = &stripe.Period{Start: boundary + 2592000, End: boundary + 5184000}
	setStripeSubscriptionCurrentPeriod(subscription, boundary+2592000, boundary+5184000)
	renewed, err := ReconcilePaidInvoice(context.Background(), invoice.ID)
	require.NoError(t, err)
	require.True(t, renewed.Applied)
	assertTargetGrant(renewed.Entitlement)
	require.Equal(t, originalOrderID, renewed.Binding.InitialOrderId)
	replay, err := ReconcilePaidInvoice(context.Background(), invoice.ID)
	require.NoError(t, err)
	require.False(t, replay.Applied)
	var originalOrder model.SubscriptionOrder
	require.NoError(t, model.DB.First(&originalOrder, originalOrderID).Error)
	require.Equal(t, 9299, originalOrder.PlanId)
	require.Equal(t, string(originalSnapshot), originalOrder.PlanSnapshot)
	var grants int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("contract_id = ?", contract.Id).Count(&grants).Error)
	require.Equal(t, int64(3), grants)
}

func TestRecurringPlanGrantLimitsRejectMismatchedEntitlement(t *testing.T) {
	for _, column := range []string{"user_id", "contract_id", "provider_binding_id", "plan_id", "current_slot"} {
		t.Run(column, func(t *testing.T) {
			setupSubscriptionInvoiceServiceTestDB(t)
			_, binding, entitlement := seedStripeRenewalContract(t, 9200, 9301, "sub_grant_limits_owner")
			require.NoError(t, model.DB.Create(&model.SubscriptionPlan{
				Id: binding.PlanId + 1, Title: "Original Plan", PriceAmount: 12.34, Currency: "USD",
			}).Error)
			order := model.SubscriptionOrder{
				UserId: binding.UserId, PlanId: binding.PlanId + 1, TradeNo: "original-before-downgrade",
				Status: common.TopUpStatusSuccess, PaymentProvider: model.PaymentProviderStripe,
			}
			require.NoError(t, model.DB.Create(&order).Error)
			binding.InitialOrderId = order.Id
			snapshot, err := recurringPlanSnapshotFromBindingTx(model.DB, &binding)
			require.NoError(t, err)
			require.NotNil(t, snapshot.GrantLimits)
			require.False(t, snapshot.Found)
			// No entitlement from another user, contract, binding, plan, or an old
			// historical slot may become the source of the current grant's limits.
			require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("id = ?", entitlement.Id).Update(column, 999999).Error)
			_, err = recurringPlanSnapshotFromBindingTx(model.DB, &binding)
			require.Error(t, err)
		})
	}
}
