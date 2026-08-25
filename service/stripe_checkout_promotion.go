package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting"
	"github.com/stripe/stripe-go/v86"
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
	query := url.Values{}
	query.Set("active", "true")
	query.Set("code", strings.TrimSpace(code))
	query.Add("expand[]", "data.promotion.coupon")

	promotions := make([]*stripe.PromotionCode, 0)
	for {
		params := &stripe.RawParams{}
		params.Context = ctx
		backend, err := stripe.GetRawRequestBackend(stripe.APIBackend)
		if err != nil {
			return nil, err
		}
		response, err := backend.RawRequest(http.MethodGet, "/v1/promotion_codes?"+query.Encode(), setting.StripeApiSecret, "", params)
		if err != nil {
			return nil, err
		}
		var page stripeCheckoutPromotionListResponse
		if err := json.Unmarshal(response.RawJSON, &page); err != nil {
			return nil, err
		}
		for _, item := range page.Data {
			promotions = append(promotions, item.PromotionCode())
		}
		if !page.HasMore {
			return promotions, nil
		}
		if len(page.Data) == 0 || strings.TrimSpace(page.Data[len(page.Data)-1].ID) == "" {
			return nil, fmt.Errorf("Stripe promotion code list has_more response is missing a last id")
		}
		query.Set("starting_after", strings.TrimSpace(page.Data[len(page.Data)-1].ID))
	}
}

type stripeCheckoutPromotionListResponse struct {
	Data    []stripeCheckoutPromotionListItem `json:"data"`
	HasMore bool                              `json:"has_more"`
}

type stripeCheckoutPromotionListItem struct {
	ID             string                            `json:"id"`
	Active         bool                              `json:"active"`
	Code           string                            `json:"code"`
	Created        int64                             `json:"created"`
	Customer       *stripe.Customer                  `json:"customer"`
	ExpiresAt      int64                             `json:"expires_at"`
	MaxRedemptions int64                             `json:"max_redemptions"`
	Promotion      *stripe.PromotionCodePromotion    `json:"promotion"`
	Coupon         *stripe.Coupon                    `json:"coupon"`
	Restrictions   *stripe.PromotionCodeRestrictions `json:"restrictions"`
	TimesRedeemed  int64                             `json:"times_redeemed"`
}

func (item stripeCheckoutPromotionListItem) PromotionCode() *stripe.PromotionCode {
	promotion := item.Promotion
	if promotion == nil && item.Coupon != nil {
		promotion = &stripe.PromotionCodePromotion{
			Type:   stripe.PromotionCodePromotionTypeCoupon,
			Coupon: item.Coupon,
		}
	}
	return &stripe.PromotionCode{
		Active:         item.Active,
		Code:           item.Code,
		Created:        item.Created,
		Customer:       item.Customer,
		ExpiresAt:      item.ExpiresAt,
		ID:             item.ID,
		MaxRedemptions: item.MaxRedemptions,
		Promotion:      promotion,
		Restrictions:   item.Restrictions,
		TimesRedeemed:  item.TimesRedeemed,
	}
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
