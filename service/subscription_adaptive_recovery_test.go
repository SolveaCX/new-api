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

func TestAdaptivePricingPendingPurchaseRecoveryRequiresKnownInvoice(t *testing.T) {
	for _, withInvoicePointer := range []bool{false, true} {
		t.Run(strconv.FormatBool(withInvoicePointer), func(t *testing.T) {
			setupSubscriptionInvoiceServiceTestDB(t)
			contract, intent := seedStripeInvoicePurchase(t, 9198, 9298, "sub_adaptive_recovery")
			if withInvoicePointer {
				require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).
					Where("trade_no = ?", "sub_adaptive_recovery").
					Update("provider_payload", "invoice_id=in_adaptive_recovery;change_intent_id="+strconv.FormatInt(intent.Id, 10)).Error)
			}
			invoice := stripeInvoiceFixture("in_adaptive_recovery", "sub_adaptive_recovery")
			subscription := stripeSubscriptionFixture("sub_adaptive_recovery", map[string]string{
				"trade_no":         "sub_adaptive_recovery",
				"user_id":          "9198",
				"plan_id":          "9298",
				"contract_id":      strconv.FormatInt(contract.Id, 10),
				"change_intent_id": strconv.FormatInt(intent.Id, 10),
			})
			setStripeInvoiceFixtureAmountAndPrice(invoice, subscription, 4990, stripe.CurrencyBRL, "price_invoice_plan")
			t.Cleanup(replaceStripeInvoiceReconcilers(t, invoice, subscription))

			count, err := reconcileStalePendingStripePurchases(context.Background())
			require.NoError(t, err)
			if withInvoicePointer {
				require.Equal(t, 1, count)
			} else {
				// An old rejection before persisting invoice_id is deliberately
				// invisible to the scanner. Deploying alone cannot repair it;
				// replay the authenticated original invoice event instead.
				require.Zero(t, count)
				var pending model.SubscriptionOrder
				require.NoError(t, model.DB.Where("trade_no = ?", "sub_adaptive_recovery").First(&pending).Error)
				require.Equal(t, common.TopUpStatusPending, pending.Status)
				result, err := ReconcilePaidInvoice(context.Background(), invoice.ID)
				require.NoError(t, err)
				require.True(t, result.Applied)
			}

			var order model.SubscriptionOrder
			require.NoError(t, model.DB.Where("trade_no = ?", "sub_adaptive_recovery").First(&order).Error)
			require.Equal(t, common.TopUpStatusSuccess, order.Status)
			count, err = reconcileStalePendingStripePurchases(context.Background())
			require.NoError(t, err)
			require.Zero(t, count)
			replay, err := ReconcilePaidInvoice(context.Background(), invoice.ID)
			require.NoError(t, err)
			require.False(t, replay.Applied)
			var grants int64
			require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("contract_id = ?", contract.Id).Count(&grants).Error)
			require.Equal(t, int64(1), grants)
		})
	}
}
