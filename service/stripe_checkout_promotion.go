package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting"
	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/promotioncode"
)

var (
	ErrStripePromotionUnavailable = errors.New("promotion code is unavailable")
	ErrStripePromotionAmbiguous   = errors.New("promotion code is ambiguous")
	ErrStripePromotionLookup      = errors.New("promotion code lookup failed")
)

type StripeCheckoutPromotionClient interface {
	ListPromotionCodes(context.Context, string) ([]*stripe.PromotionCode, error)
}

type StripeCheckoutPromotionQuery struct {
	Code       string
	CustomerID string
	ProductID  string
	Currency   stripe.Currency
	Subtotal   int64
}

type StripeCheckoutResolvedPromotion struct {
	PromotionCodeID string
	CouponID        string
	MaskedCode      string
}

type StripeCheckoutPromotionResolver struct {
	Client StripeCheckoutPromotionClient
}

func (r StripeCheckoutPromotionResolver) ResolveManualPromotion(ctx context.Context, query StripeCheckoutPromotionQuery) (StripeCheckoutResolvedPromotion, error) {
	code := strings.TrimSpace(query.Code)
	if code == "" {
		return StripeCheckoutResolvedPromotion{}, ErrStripePromotionUnavailable
	}

	client := r.Client
	if client == nil {
		client = stripeCheckoutPromotionListClient{}
	}
	promotions, err := client.ListPromotionCodes(ctx, code)
	if err != nil {
		return StripeCheckoutResolvedPromotion{}, ErrStripePromotionLookup
	}

	now := time.Now().Unix()
	customerID := strings.TrimSpace(query.CustomerID)
	var customerMatches []*stripe.PromotionCode
	var globalMatches []*stripe.PromotionCode
	for _, promotion := range promotions {
		if !stripeCheckoutPromotionEligible(promotion, code, customerID, query, now) {
			continue
		}
		if promotion.Customer != nil {
			customerMatches = append(customerMatches, promotion)
		} else {
			globalMatches = append(globalMatches, promotion)
		}
	}

	matches := globalMatches
	if len(customerMatches) > 0 {
		matches = customerMatches
	}
	if len(matches) == 0 {
		return StripeCheckoutResolvedPromotion{}, ErrStripePromotionUnavailable
	}
	if len(matches) > 1 {
		return StripeCheckoutResolvedPromotion{}, ErrStripePromotionAmbiguous
	}

	match := matches[0]
	return StripeCheckoutResolvedPromotion{
		PromotionCodeID: match.ID,
		CouponID:        match.Promotion.Coupon.ID,
		MaskedCode:      match.Code,
	}, nil
}

type stripeCheckoutPromotionListClient struct{}

func (stripeCheckoutPromotionListClient) ListPromotionCodes(ctx context.Context, code string) ([]*stripe.PromotionCode, error) {
	params := &stripe.PromotionCodeListParams{
		Active: stripe.Bool(true),
		Code:   stripe.String(strings.TrimSpace(code)),
	}
	params.AddExpand("data.promotion.coupon")
	params.Context = ctx
	client := promotioncode.Client{B: stripe.GetBackend(stripe.APIBackend), Key: setting.StripeApiSecret}
	iter := client.List(params)
	promotions := make([]*stripe.PromotionCode, 0)
	for iter.Next() {
		promotions = append(promotions, iter.PromotionCode())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return promotions, nil
}

func stripeCheckoutPromotionEligible(promotion *stripe.PromotionCode, code, customerID string, query StripeCheckoutPromotionQuery, now int64) bool {
	if promotion == nil || !promotion.Active || !strings.EqualFold(promotion.Code, code) || strings.TrimSpace(promotion.ID) == "" {
		return false
	}
	if promotion.Customer != nil && promotion.Customer.ID != customerID {
		return false
	}
	if stripeCheckoutPromotionExpiredOrExhausted(promotion.ExpiresAt, promotion.MaxRedemptions, promotion.TimesRedeemed, now) {
		return false
	}
	if promotion.Promotion == nil || promotion.Promotion.Type != stripe.PromotionCodePromotionTypeCoupon || promotion.Promotion.Coupon == nil {
		return false
	}

	coupon := promotion.Promotion.Coupon
	if strings.TrimSpace(coupon.ID) == "" || !coupon.Valid || stripeCheckoutPromotionExpiredOrExhausted(coupon.RedeemBy, coupon.MaxRedemptions, coupon.TimesRedeemed, now) {
		return false
	}
	if !stripeCheckoutPromotionMeetsMinimum(promotion.Restrictions, query.Currency, query.Subtotal) {
		return false
	}
	return stripeCheckoutPromotionAppliesToProduct(coupon.AppliesTo, strings.TrimSpace(query.ProductID))
}

func stripeCheckoutPromotionExpiredOrExhausted(expiresAt, maxRedemptions, timesRedeemed, now int64) bool {
	return (expiresAt > 0 && expiresAt <= now) || (maxRedemptions > 0 && timesRedeemed >= maxRedemptions)
}

func stripeCheckoutPromotionMeetsMinimum(restrictions *stripe.PromotionCodeRestrictions, currency stripe.Currency, subtotal int64) bool {
	if restrictions == nil {
		return true
	}
	currencyCode := strings.ToLower(string(currency))
	if len(restrictions.CurrencyOptions) > 0 {
		option, ok := restrictions.CurrencyOptions[currencyCode]
		if !ok || option == nil {
			return false
		}
		return subtotal >= option.MinimumAmount
	}
	if restrictions.MinimumAmount == 0 {
		return true
	}
	return strings.EqualFold(string(restrictions.MinimumAmountCurrency), string(currency)) && subtotal >= restrictions.MinimumAmount
}

func stripeCheckoutPromotionAppliesToProduct(appliesTo *stripe.CouponAppliesTo, productID string) bool {
	if appliesTo == nil {
		return true
	}
	for _, allowedProductID := range appliesTo.Products {
		if allowedProductID == productID {
			return true
		}
	}
	return false
}
