package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/stripe/stripe-go/v81"
	stripecoupon "github.com/stripe/stripe-go/v81/coupon"
	stripecustomer "github.com/stripe/stripe-go/v81/customer"
	stripeprice "github.com/stripe/stripe-go/v81/price"
	stripepromotioncode "github.com/stripe/stripe-go/v81/promotioncode"
)

const (
	topUpRecallTickInterval = 10 * time.Minute
	topUpRecallBatchSize    = 50
	topUpRecallRefundDays   = 7
	topUpRecallAmountOffUSD = int64(200)
	topUpRecallPackageUSD   = int64(5)
	topUpRecallMinAmountUSD = int64(500)
	topUpRecallPromoTTL     = 7 * 24 * time.Hour
)

var (
	topUpRecallOnce                 sync.Once
	topUpRecallRunning              atomic.Bool
	topUpRecallCouponCreator        = createStripeTopUpRecallCoupon
	topUpRecallPromotionCodeCreator = createStripeTopUpRecallPromotionCode
	topUpRecallPriceGetter          = getStripeTopUpRecallPrice
	topUpRecallCustomerCreator      = createStripeTopUpRecallCustomer
	topUpRecallEmailSender          = common.SendEmail
)

func createStripeTopUpRecallCoupon(params *stripe.CouponParams) (*stripe.Coupon, error) {
	return stripecoupon.Client{
		B:   stripe.GetBackend(stripe.APIBackend),
		Key: strings.TrimSpace(setting.StripeApiSecret),
	}.New(params)
}

func createStripeTopUpRecallPromotionCode(params *stripe.PromotionCodeParams) (*stripe.PromotionCode, error) {
	return stripepromotioncode.Client{
		B:   stripe.GetBackend(stripe.APIBackend),
		Key: strings.TrimSpace(setting.StripeApiSecret),
	}.New(params)
}

func getStripeTopUpRecallPrice(priceId string, params *stripe.PriceParams) (*stripe.Price, error) {
	return stripeprice.Client{
		B:   stripe.GetBackend(stripe.APIBackend),
		Key: strings.TrimSpace(setting.StripeApiSecret),
	}.Get(priceId, params)
}

func createStripeTopUpRecallCustomer(params *stripe.CustomerParams) (*stripe.Customer, error) {
	return stripecustomer.Client{
		B:   stripe.GetBackend(stripe.APIBackend),
		Key: strings.TrimSpace(setting.StripeApiSecret),
	}.New(params)
}

func StartTopUpRecallTask() {
	topUpRecallOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("top-up recall task started: tick=%s", topUpRecallTickInterval))
			ticker := time.NewTicker(topUpRecallTickInterval)
			defer ticker.Stop()

			runTopUpRecallOnce()
			for range ticker.C {
				runTopUpRecallOnce()
			}
		})
	})
}

func runTopUpRecallOnce() {
	if !topUpRecallEnabled() {
		return
	}
	if !topUpRecallRunning.CompareAndSwap(false, true) {
		return
	}
	defer topUpRecallRunning.Store(false)

	candidates, err := model.GetEligibleTopUpRecallCandidates(common.GetTimestamp(), topUpRecallBatchSize)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("top-up recall scan failed: %v", err))
		return
	}

	for _, candidate := range candidates {
		if err := processTopUpRecallCandidate(candidate); err != nil {
			logger.LogWarn(context.Background(), fmt.Sprintf("top-up recall failed trade_no=%s user_id=%d error=%v", candidate.TradeNo, candidate.UserId, err))
		}
	}
}

func processTopUpRecallCandidate(candidate model.TopUpRecallCandidate) error {
	recall, reserved, err := model.ReserveTopUpRecall(candidate)
	if err != nil || !reserved {
		return err
	}

	code, promoID, err := createTopUpRecallPromotionCode(recall)
	if err != nil {
		_ = model.MarkTopUpRecallFailed(recall.Id, err)
		return err
	}

	subject, content := renderTopUpRecallEmail(candidate, code)
	if err := topUpRecallEmailSender(subject, recall.Email, content); err != nil {
		_ = model.MarkTopUpRecallFailed(recall.Id, err)
		return err
	}

	return model.MarkTopUpRecallSent(recall.Id, code, promoID)
}

func topUpRecallEnabled() bool {
	return strings.HasPrefix(strings.TrimSpace(setting.StripeApiSecret), "sk_") ||
		strings.HasPrefix(strings.TrimSpace(setting.StripeApiSecret), "rk_")
}

func createTopUpRecallPromotionCode(recall *model.TopUpRecall) (string, string, error) {
	if recall == nil {
		return "", "", errors.New("top-up recall is nil")
	}
	if !topUpRecallEnabled() {
		return "", "", errors.New("Stripe API secret is not configured")
	}

	code := buildTopUpRecallPromotionCode(recall)
	productId, err := topUpRecallProductId()
	if err != nil {
		return "", "", err
	}
	customerId, err := ensureTopUpRecallStripeCustomer(recall.UserId)
	if err != nil {
		return "", "", err
	}
	expiresAt := time.Now().Add(topUpRecallPromoTTL).Unix()

	couponParams := &stripe.CouponParams{
		AmountOff:      stripe.Int64(topUpRecallAmountOffUSD),
		AppliesTo:      &stripe.CouponAppliesToParams{Products: []*string{stripe.String(productId)}},
		Currency:       stripe.String(strings.ToLower(string(stripe.CurrencyUSD))),
		Duration:       stripe.String(string(stripe.CouponDurationOnce)),
		MaxRedemptions: stripe.Int64(1),
		Name:           stripe.String("$2 abandoned top-up recovery"),
		RedeemBy:       stripe.Int64(expiresAt),
	}
	couponParams.AddMetadata("source", "topup_recall")
	couponParams.AddMetadata("trade_no", recall.TradeNo)
	couponParams.AddMetadata("user_id", fmt.Sprintf("%d", recall.UserId))

	coupon, err := topUpRecallCouponCreator(couponParams)
	if err != nil {
		return "", "", err
	}
	if coupon == nil || strings.TrimSpace(coupon.ID) == "" {
		return "", "", errors.New("Stripe coupon creation returned empty ID")
	}

	promotionCodeParams := &stripe.PromotionCodeParams{
		Code:           stripe.String(code),
		Coupon:         stripe.String(coupon.ID),
		Customer:       stripe.String(customerId),
		ExpiresAt:      stripe.Int64(expiresAt),
		MaxRedemptions: stripe.Int64(1),
		Restrictions: &stripe.PromotionCodeRestrictionsParams{
			MinimumAmount:         stripe.Int64(topUpRecallMinAmountUSD),
			MinimumAmountCurrency: stripe.String(strings.ToLower(string(stripe.CurrencyUSD))),
		},
	}
	promotionCodeParams.AddMetadata("source", "topup_recall")
	promotionCodeParams.AddMetadata("trade_no", recall.TradeNo)
	promotionCodeParams.AddMetadata("user_id", fmt.Sprintf("%d", recall.UserId))

	promotionCode, err := topUpRecallPromotionCodeCreator(promotionCodeParams)
	if err != nil {
		return "", "", err
	}
	if promotionCode == nil || strings.TrimSpace(promotionCode.ID) == "" {
		return "", "", errors.New("Stripe promotion code creation returned empty ID")
	}

	return code, promotionCode.ID, nil
}

func topUpRecallProductId() (string, error) {
	priceId := setting.StripeTopUpPriceIDForAmount(topUpRecallPackageUSD)
	if strings.TrimSpace(priceId) == "" {
		return "", fmt.Errorf("Stripe $%d top-up Price ID is not configured", topUpRecallPackageUSD)
	}

	params := &stripe.PriceParams{}
	params.AddExpand("product")
	price, err := topUpRecallPriceGetter(strings.TrimSpace(priceId), params)
	if err != nil {
		return "", err
	}
	if price == nil || price.Product == nil || strings.TrimSpace(price.Product.ID) == "" {
		return "", fmt.Errorf("Stripe Price %s does not have a product", strings.TrimSpace(priceId))
	}
	return strings.TrimSpace(price.Product.ID), nil
}

func ensureTopUpRecallStripeCustomer(userId int) (string, error) {
	if userId == 0 {
		return "", errors.New("top-up recall user id is empty")
	}
	user, err := model.GetUserById(userId, false)
	if err != nil || user == nil {
		return "", err
	}
	if customerId := strings.TrimSpace(user.StripeCustomer); customerId != "" {
		return customerId, nil
	}

	params := topUpRecallCustomerParams(user)
	params.SetIdempotencyKey(fmt.Sprintf("topup_recall_customer_%d", userId))
	customer, err := topUpRecallCustomerCreator(params)
	if err != nil {
		return "", err
	}
	if customer == nil || strings.TrimSpace(customer.ID) == "" {
		return "", errors.New("Stripe customer creation returned empty ID")
	}
	return model.SetUserStripeCustomerIfEmpty(userId, customer.ID)
}

func topUpRecallCustomerParams(user *model.User) *stripe.CustomerParams {
	params := &stripe.CustomerParams{}
	if user == nil {
		return params
	}
	if email := strings.TrimSpace(user.Email); email != "" {
		params.Email = stripe.String(email)
	}
	if name := strings.TrimSpace(user.DisplayName); name != "" {
		params.Name = stripe.String(name)
	} else if name := strings.TrimSpace(user.Username); name != "" {
		params.Name = stripe.String(name)
	}
	params.AddMetadata("source", "topup_recall")
	params.AddMetadata("user_id", fmt.Sprintf("%d", user.Id))
	return params
}

func buildTopUpRecallPromotionCode(recall *model.TopUpRecall) string {
	randomSuffix := strings.ToUpper(common.GetRandomString(6))
	return fmt.Sprintf("SAVE2-%d-%s", recall.Id, randomSuffix)
}

func renderTopUpRecallEmail(candidate model.TopUpRecallCandidate, code string) (string, string) {
	data := map[string]any{
		"SystemName": common.SystemName,
		"Amount":     fmt.Sprintf("$%d", candidate.Amount),
		"Code":       code,
		"Link":       topUpRecallLink(),
		"RefundDays": topUpRecallRefundDays,
	}
	return i18n.Translate(candidate.Language, i18n.MsgEmailTopUpRecallSubject, data),
		i18n.Translate(candidate.Language, i18n.MsgEmailTopUpRecallContent, data)
}

func topUpRecallLink() string {
	if link := topUpURL(); strings.TrimSpace(link) != "" {
		return link
	}
	return "https://console.flatkey.ai/wallet"
}
