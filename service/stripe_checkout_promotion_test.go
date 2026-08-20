package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v86"
)

type fakeStripeCheckoutPromotionClient struct {
	promotions []*stripe.PromotionCode
	err        error
}

func (f *fakeStripeCheckoutPromotionClient) ListPromotionCodes(context.Context, string) ([]*stripe.PromotionCode, error) {
	return f.promotions, f.err
}

func TestResolveManualPromotionReturnsCaseInsensitiveGlobalMatch(t *testing.T) {
	resolver := StripeCheckoutPromotionResolver{Client: &fakeStripeCheckoutPromotionClient{
		promotions: []*stripe.PromotionCode{stripeCheckoutTestGlobalPromotion("promo_global", "Save20", 20)},
	}}

	got, err := resolver.ResolveManualPromotion(context.Background(), StripeCheckoutPromotionQuery{
		Code: " save20 ", CustomerID: "cus_7", ProductID: "prod_pro",
		Currency: stripe.CurrencyUSD, Subtotal: 3000,
	})

	require.NoError(t, err)
	require.Equal(t, StripeCheckoutResolvedPromotion{
		PromotionCodeID: "promo_global",
		CouponID:        "coupon_promo_global",
		MaskedCode:      "Save20",
	}, got)
}

func TestResolveManualPromotionPrefersCurrentCustomer(t *testing.T) {
	resolver := StripeCheckoutPromotionResolver{Client: &fakeStripeCheckoutPromotionClient{
		promotions: []*stripe.PromotionCode{
			stripeCheckoutTestGlobalPromotion("promo_global", "SAVE20", 20),
			stripeCheckoutTestCustomerPromotion("promo_customer", "SAVE20", "cus_7", 25),
		},
	}}

	got, err := resolver.ResolveManualPromotion(context.Background(), StripeCheckoutPromotionQuery{
		Code: " save20 ", CustomerID: "cus_7", ProductID: "prod_pro",
		Currency: stripe.CurrencyUSD, Subtotal: 3000,
	})

	require.NoError(t, err)
	require.Equal(t, "promo_customer", got.PromotionCodeID)
	require.Equal(t, "SAVE20", got.MaskedCode)
}

func TestResolveManualPromotionRejectsAmbiguousGlobalMatch(t *testing.T) {
	resolver := StripeCheckoutPromotionResolver{Client: &fakeStripeCheckoutPromotionClient{
		promotions: []*stripe.PromotionCode{
			stripeCheckoutTestGlobalPromotion("promo_global_1", "SAVE20", 20),
			stripeCheckoutTestGlobalPromotion("promo_global_2", "save20", 25),
		},
	}}

	_, err := resolver.ResolveManualPromotion(context.Background(), StripeCheckoutPromotionQuery{
		Code: "SAVE20", CustomerID: "cus_7", ProductID: "prod_pro",
		Currency: stripe.CurrencyUSD, Subtotal: 3000,
	})

	require.ErrorIs(t, err, ErrStripePromotionAmbiguous)
}

func TestResolveManualPromotionRejectsInactiveCode(t *testing.T) {
	promotion := stripeCheckoutTestGlobalPromotion("promo_inactive", "PRIVATE40", 40)
	promotion.Active = false
	resolver := StripeCheckoutPromotionResolver{Client: &fakeStripeCheckoutPromotionClient{
		promotions: []*stripe.PromotionCode{promotion},
	}}

	_, err := resolver.ResolveManualPromotion(context.Background(), StripeCheckoutPromotionQuery{
		Code: "PRIVATE40", CustomerID: "cus_7", ProductID: "prod_pro",
		Currency: stripe.CurrencyUSD, Subtotal: 5000,
	})

	require.ErrorIs(t, err, ErrStripePromotionUnavailable)
	require.NotContains(t, err.Error(), "PRIVATE40")
}

func TestResolveManualPromotionRejectsCodeForAnotherCustomer(t *testing.T) {
	resolver := StripeCheckoutPromotionResolver{Client: &fakeStripeCheckoutPromotionClient{
		promotions: []*stripe.PromotionCode{
			stripeCheckoutTestCustomerPromotion("promo_customer", "PRIVATE40", "cus_other", 40),
		},
	}}

	_, err := resolver.ResolveManualPromotion(context.Background(), StripeCheckoutPromotionQuery{
		Code: "PRIVATE40", CustomerID: "cus_7", ProductID: "prod_pro",
		Currency: stripe.CurrencyUSD, Subtotal: 5000,
	})

	require.ErrorIs(t, err, ErrStripePromotionUnavailable)
}

func TestResolveManualPromotionRejectsMinimumAmountAndCurrencyMismatch(t *testing.T) {
	tests := []struct {
		name     string
		currency stripe.Currency
		subtotal int64
	}{
		{name: "subtotal below minimum", currency: stripe.CurrencyUSD, subtotal: 2499},
		{name: "currency differs", currency: stripe.CurrencyEUR, subtotal: 3000},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			promotion := stripeCheckoutTestGlobalPromotion("promo_minimum", "SAVE20", 20)
			promotion.Restrictions = &stripe.PromotionCodeRestrictions{
				MinimumAmount:         2500,
				MinimumAmountCurrency: stripe.CurrencyUSD,
			}
			resolver := StripeCheckoutPromotionResolver{Client: &fakeStripeCheckoutPromotionClient{
				promotions: []*stripe.PromotionCode{promotion},
			}}

			_, err := resolver.ResolveManualPromotion(context.Background(), StripeCheckoutPromotionQuery{
				Code: "SAVE20", CustomerID: "cus_7", ProductID: "prod_pro",
				Currency: test.currency, Subtotal: test.subtotal,
			})

			require.ErrorIs(t, err, ErrStripePromotionUnavailable)
		})
	}
}

func TestResolveManualPromotionAcceptsCurrencySpecificMinimum(t *testing.T) {
	promotion := stripeCheckoutTestGlobalPromotion("promo_minimum", "SAVE20", 20)
	promotion.Restrictions = &stripe.PromotionCodeRestrictions{
		CurrencyOptions: map[string]*stripe.PromotionCodeRestrictionsCurrencyOptions{
			"usd": {MinimumAmount: 2500},
			"eur": {MinimumAmount: 2000},
		},
	}
	resolver := StripeCheckoutPromotionResolver{Client: &fakeStripeCheckoutPromotionClient{
		promotions: []*stripe.PromotionCode{promotion},
	}}

	got, err := resolver.ResolveManualPromotion(context.Background(), StripeCheckoutPromotionQuery{
		Code: "SAVE20", CustomerID: "cus_7", ProductID: "prod_pro",
		Currency: stripe.CurrencyEUR, Subtotal: 2000,
	})

	require.NoError(t, err)
	require.Equal(t, "promo_minimum", got.PromotionCodeID)
}

func TestResolveManualPromotionRejectsProductRestriction(t *testing.T) {
	promotion := stripeCheckoutTestGlobalPromotion("promo_product", "PROONLY", 20)
	promotion.Promotion.Coupon.AppliesTo = &stripe.CouponAppliesTo{Products: []string{"prod_other"}}
	resolver := StripeCheckoutPromotionResolver{Client: &fakeStripeCheckoutPromotionClient{
		promotions: []*stripe.PromotionCode{promotion},
	}}

	_, err := resolver.ResolveManualPromotion(context.Background(), StripeCheckoutPromotionQuery{
		Code: "PROONLY", CustomerID: "cus_7", ProductID: "prod_pro",
		Currency: stripe.CurrencyUSD, Subtotal: 3000,
	})

	require.ErrorIs(t, err, ErrStripePromotionUnavailable)
}

func TestResolveManualPromotionRejectsExpiredOrExhaustedPromotionAndCoupon(t *testing.T) {
	now := time.Now().Unix()
	tests := []struct {
		name   string
		mutate func(*stripe.PromotionCode)
	}{
		{name: "promotion expired", mutate: func(p *stripe.PromotionCode) { p.ExpiresAt = now - 1 }},
		{name: "promotion exhausted", mutate: func(p *stripe.PromotionCode) { p.MaxRedemptions, p.TimesRedeemed = 2, 2 }},
		{name: "coupon invalid", mutate: func(p *stripe.PromotionCode) { p.Promotion.Coupon.Valid = false }},
		{name: "coupon expired", mutate: func(p *stripe.PromotionCode) { p.Promotion.Coupon.RedeemBy = now - 1 }},
		{name: "coupon exhausted", mutate: func(p *stripe.PromotionCode) {
			p.Promotion.Coupon.MaxRedemptions, p.Promotion.Coupon.TimesRedeemed = 3, 3
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			promotion := stripeCheckoutTestGlobalPromotion("promo_limited", "LIMIT20", 20)
			test.mutate(promotion)
			resolver := StripeCheckoutPromotionResolver{Client: &fakeStripeCheckoutPromotionClient{
				promotions: []*stripe.PromotionCode{promotion},
			}}

			_, err := resolver.ResolveManualPromotion(context.Background(), StripeCheckoutPromotionQuery{
				Code: "LIMIT20", CustomerID: "cus_7", ProductID: "prod_pro",
				Currency: stripe.CurrencyUSD, Subtotal: 3000,
			})

			require.ErrorIs(t, err, ErrStripePromotionUnavailable)
		})
	}
}

func TestResolveManualPromotionSanitizesLookupFailure(t *testing.T) {
	const submittedCode = "SENSITIVE-CODE"
	resolver := StripeCheckoutPromotionResolver{Client: &fakeStripeCheckoutPromotionClient{
		err: errors.New("upstream rejected code SENSITIVE-CODE"),
	}}

	_, err := resolver.ResolveManualPromotion(context.Background(), StripeCheckoutPromotionQuery{
		Code: submittedCode, CustomerID: "cus_7", ProductID: "prod_pro",
		Currency: stripe.CurrencyUSD, Subtotal: 3000,
	})

	require.ErrorIs(t, err, ErrStripePromotionLookup)
	require.NotContains(t, err.Error(), submittedCode)
}

func TestStripeCheckoutPromotionListClientFiltersAndConsumesAllPages(t *testing.T) {
	originalBackend := stripe.GetBackend(stripe.APIBackend)
	originalKey := stripe.Key
	originalSecret := setting.StripeApiSecret
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/promotion_codes", r.URL.Path)
		require.Equal(t, "Bearer sk_test_promotion_resolver", r.Header.Get("Authorization"))
		queries = append(queries, r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		coupon := `"coupon_first"`
		for key, values := range r.URL.Query() {
			if strings.HasPrefix(key, "expand") && len(values) == 1 && values[0] == "promotion.coupon" {
				coupon = `{"id":"coupon_first","object":"coupon","percent_off":20,"valid":true}`
			}
		}
		if r.URL.Query().Get("starting_after") == "promo_first" {
			body := `{"object":"list","data":[{"id":"promo_second","object":"promotion_code","active":true,"code":"Save20","promotion":{"type":"coupon","coupon":` + strings.Replace(coupon, "coupon_first", "coupon_second", 1) + `}}],"has_more":false,"url":"/v1/promotion_codes"}`
			_, _ = w.Write([]byte(body))
			return
		}
		body := `{"object":"list","data":[{"id":"promo_first","object":"promotion_code","active":true,"code":"Save20","customer":"cus_other","promotion":{"type":"coupon","coupon":` + coupon + `}}],"has_more":true,"url":"/v1/promotion_codes"}`
		_, _ = w.Write([]byte(body))
	}))
	stripe.SetBackend(stripe.APIBackend, stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{
		URL:               stripe.String(server.URL),
		HTTPClient:        server.Client(),
		MaxNetworkRetries: stripe.Int64(0),
		LeveledLogger:     &stripe.LeveledLogger{Level: stripe.LevelNull},
	}))
	stripe.Key = "global-sentinel"
	setting.StripeApiSecret = "sk_test_promotion_resolver"
	t.Cleanup(func() {
		server.Close()
		stripe.SetBackend(stripe.APIBackend, originalBackend)
		stripe.Key = originalKey
		setting.StripeApiSecret = originalSecret
	})

	got, err := (StripeCheckoutPromotionResolver{}).ResolveManualPromotion(context.Background(), StripeCheckoutPromotionQuery{
		Code: " Save20 ", CustomerID: "cus_7", ProductID: "prod_pro",
		Currency: stripe.CurrencyUSD, Subtotal: 3000,
	})

	require.NoError(t, err)
	require.Equal(t, StripeCheckoutResolvedPromotion{
		PromotionCodeID: "promo_second",
		CouponID:        "coupon_second",
		MaskedCode:      "Save20",
	}, got)
	require.Equal(t, "global-sentinel", stripe.Key)
	require.Len(t, queries, 2)
	for _, rawQuery := range queries {
		require.Contains(t, rawQuery, "active=true")
		require.Contains(t, rawQuery, "code=Save20")
		require.Contains(t, rawQuery, "expand")
	}
	require.True(t, strings.Contains(queries[1], "starting_after=promo_first"))
}

func stripeCheckoutTestGlobalPromotion(id, code string, percentOff float64) *stripe.PromotionCode {
	return &stripe.PromotionCode{
		Active: true,
		Code:   code,
		ID:     id,
		Object: "promotion_code",
		Promotion: &stripe.PromotionCodePromotion{
			Type: stripe.PromotionCodePromotionTypeCoupon,
			Coupon: &stripe.Coupon{
				ID:         "coupon_" + id,
				Object:     "coupon",
				PercentOff: percentOff,
				Valid:      true,
			},
		},
		Restrictions: &stripe.PromotionCodeRestrictions{},
	}
}

func stripeCheckoutTestCustomerPromotion(id, code, customerID string, percentOff float64) *stripe.PromotionCode {
	promotion := stripeCheckoutTestGlobalPromotion(id, code, percentOff)
	promotion.Customer = &stripe.Customer{ID: customerID, Object: "customer"}
	return promotion
}
