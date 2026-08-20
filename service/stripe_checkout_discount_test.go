package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v86"
)

func TestApplyStripeCheckoutDiscountUsesExactlyOneExplicitSelection(t *testing.T) {
	tests := []struct {
		name              string
		selection         StripeCheckoutDiscountSelection
		wantCoupon        string
		wantPromotionCode string
		wantCount         int
	}{
		{
			name: "invitation coupon",
			selection: StripeCheckoutDiscountSelection{
				Source:   StripeCheckoutDiscountInvitation,
				CouponID: " coupon_invitation_7 ",
			},
			wantCoupon: "coupon_invitation_7",
			wantCount:  1,
		},
		{
			name: "recall promotion code",
			selection: StripeCheckoutDiscountSelection{
				Source:          StripeCheckoutDiscountRecall,
				PromotionCodeID: " promo_recall_7 ",
			},
			wantPromotionCode: "promo_recall_7",
			wantCount:         1,
		},
		{
			name: "manual promotion code",
			selection: StripeCheckoutDiscountSelection{
				Source:          StripeCheckoutDiscountManual,
				PromotionCodeID: " promo_manual_7 ",
			},
			wantPromotionCode: "promo_manual_7",
			wantCount:         1,
		},
		{
			name:      "none clears previous discount",
			selection: StripeCheckoutDiscountSelection{Source: StripeCheckoutDiscountNone},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := &stripe.CheckoutSessionParams{
				Discounts: []*stripe.CheckoutSessionDiscountParams{{Coupon: stripe.String("coupon_stale")}},
			}

			ApplyStripeCheckoutDiscount(params, test.selection)

			require.Len(t, params.Discounts, test.wantCount)
			if test.wantCount == 0 {
				return
			}
			if test.wantCoupon != "" {
				require.NotNil(t, params.Discounts[0].Coupon)
				require.Equal(t, test.wantCoupon, *params.Discounts[0].Coupon)
				require.Nil(t, params.Discounts[0].PromotionCode)
				return
			}
			require.Nil(t, params.Discounts[0].Coupon)
			require.NotNil(t, params.Discounts[0].PromotionCode)
			require.Equal(t, test.wantPromotionCode, *params.Discounts[0].PromotionCode)
		})
	}
}

func TestStripeCheckoutIdempotencyKeyUsesRevisionAndHashedSelectionIdentity(t *testing.T) {
	selection := StripeCheckoutDiscountSelection{
		Source:          StripeCheckoutDiscountManual,
		PromotionCodeID: " promo_manual_secret_7 ",
		MaskedCode:      "MAN***-7",
	}

	key := StripeCheckoutIdempotencyKey("topup-stripe:trade_7", 2, selection)

	require.Contains(t, key, "topup-stripe:trade_7:rev:2:")
	require.NotContains(t, key, "promo_manual_secret_7")
	require.NotContains(t, key, "MAN***-7")
	require.Equal(t, key, StripeCheckoutIdempotencyKey(" topup-stripe:trade_7 ", 2, selection))
	require.NotEqual(t, key, StripeCheckoutIdempotencyKey("topup-stripe:trade_7", 3, selection))
	require.NotEqual(t, key, StripeCheckoutIdempotencyKey("topup-stripe:trade_7", 2, StripeCheckoutDiscountSelection{Source: StripeCheckoutDiscountNone}))
}
