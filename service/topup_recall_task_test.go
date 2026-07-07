package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
)

func TestProcessTopUpRecallCandidateCreatesPromotionAndSendsEmail(t *testing.T) {
	truncate(t)
	require.NoError(t, i18n.Init())

	originalSystemName := common.SystemName
	originalSecret := setting.StripeApiSecret
	originalTopUpPriceIds := setting.StripeTopUpPriceIds
	originalCouponCreator := topUpRecallCouponCreator
	originalPromotionCodeCreator := topUpRecallPromotionCodeCreator
	originalPriceGetter := topUpRecallPriceGetter
	originalCustomerCreator := topUpRecallCustomerCreator
	originalEmailSender := topUpRecallEmailSender
	originalStripeKey := stripe.Key
	t.Cleanup(func() {
		common.SystemName = originalSystemName
		setting.StripeApiSecret = originalSecret
		setting.StripeTopUpPriceIds = originalTopUpPriceIds
		topUpRecallCouponCreator = originalCouponCreator
		topUpRecallPromotionCodeCreator = originalPromotionCodeCreator
		topUpRecallPriceGetter = originalPriceGetter
		topUpRecallCustomerCreator = originalCustomerCreator
		topUpRecallEmailSender = originalEmailSender
		stripe.Key = originalStripeKey
	})

	common.SystemName = "flatkey"
	setting.StripeApiSecret = "sk_test_topup_recall"
	setting.StripeTopUpPriceIds = `{"5":"price_recall_5"}`
	stripe.Key = "sk_existing_global_key"
	var createdCoupon *stripe.CouponParams
	var createdPromotionCode *stripe.PromotionCodeParams
	var fetchedPriceId string
	var fetchedPriceExpandedProduct bool
	var emailSubject string
	var emailReceiver string
	var emailContent string

	topUpRecallPriceGetter = func(priceId string, params *stripe.PriceParams) (*stripe.Price, error) {
		fetchedPriceId = priceId
		for _, expand := range params.Expand {
			if expand != nil && *expand == "product" {
				fetchedPriceExpandedProduct = true
			}
		}
		return &stripe.Price{
			ID:      priceId,
			Product: &stripe.Product{ID: "prod_wallet_topup"},
		}, nil
	}
	topUpRecallCustomerCreator = func(params *stripe.CustomerParams) (*stripe.Customer, error) {
		t.Fatal("customer should not be created when user already has a Stripe customer")
		return nil, nil
	}
	topUpRecallCouponCreator = func(params *stripe.CouponParams) (*stripe.Coupon, error) {
		createdCoupon = params
		return &stripe.Coupon{ID: "coupon_recall"}, nil
	}
	topUpRecallPromotionCodeCreator = func(params *stripe.PromotionCodeParams) (*stripe.PromotionCode, error) {
		createdPromotionCode = params
		return &stripe.PromotionCode{ID: "promo_recall"}, nil
	}
	topUpRecallEmailSender = func(subject string, receiver string, content string) error {
		emailSubject = subject
		emailReceiver = receiver
		emailContent = content
		return nil
	}

	candidate := model.TopUpRecallCandidate{
		UserId:   909,
		TradeNo:  "expired-909",
		Email:    "buyer@example.com",
		Language: i18n.LangEn,
		Amount:   5,
	}

	require.NoError(t, model.DB.Create(&model.User{
		Id:             909,
		Username:       "topup-recall-909",
		Email:          "buyer@example.com",
		AffCode:        "topup-recall-909",
		StripeCustomer: "cus_recall_909",
	}).Error)

	require.NoError(t, processTopUpRecallCandidate(candidate))

	require.Equal(t, "price_recall_5", fetchedPriceId)
	require.True(t, fetchedPriceExpandedProduct)
	require.NotNil(t, createdCoupon)
	require.Equal(t, int64(200), *createdCoupon.AmountOff)
	require.Equal(t, "usd", *createdCoupon.Currency)
	require.Equal(t, string(stripe.CouponDurationOnce), *createdCoupon.Duration)
	require.Equal(t, int64(1), *createdCoupon.MaxRedemptions)
	require.NotNil(t, createdCoupon.AppliesTo)
	require.Len(t, createdCoupon.AppliesTo.Products, 1)
	require.Equal(t, "prod_wallet_topup", *createdCoupon.AppliesTo.Products[0])
	require.NotNil(t, createdCoupon.RedeemBy)

	require.NotNil(t, createdPromotionCode)
	require.Equal(t, "coupon_recall", *createdPromotionCode.Coupon)
	require.NotNil(t, createdPromotionCode.Customer)
	require.Equal(t, "cus_recall_909", *createdPromotionCode.Customer)
	require.Equal(t, int64(1), *createdPromotionCode.MaxRedemptions)
	require.True(t, strings.HasPrefix(*createdPromotionCode.Code, "SAVE2-"))
	require.NotNil(t, createdPromotionCode.Restrictions)
	require.Equal(t, int64(500), *createdPromotionCode.Restrictions.MinimumAmount)
	require.Equal(t, "usd", *createdPromotionCode.Restrictions.MinimumAmountCurrency)
	require.NotNil(t, createdPromotionCode.ExpiresAt)
	require.Equal(t, *createdPromotionCode.ExpiresAt, *createdCoupon.RedeemBy)
	require.GreaterOrEqual(t, *createdPromotionCode.ExpiresAt, time.Now().Add(7*24*time.Hour-time.Minute).Unix())
	require.LessOrEqual(t, *createdPromotionCode.ExpiresAt, time.Now().Add(7*24*time.Hour+time.Minute).Unix())
	require.Equal(t, "sk_existing_global_key", stripe.Key)

	require.Equal(t, "buyer@example.com", emailReceiver)
	require.Contains(t, emailSubject, "flatkey")
	require.Contains(t, emailContent, "$5")
	require.Contains(t, emailContent, *createdPromotionCode.Code)

	var recall model.TopUpRecall
	require.NoError(t, model.DB.Where("user_id = ?", candidate.UserId).First(&recall).Error)
	require.Equal(t, model.TopUpRecallStatusSent, recall.Status)
	require.Equal(t, *createdPromotionCode.Code, recall.PromotionCode)
	require.Equal(t, "promo_recall", recall.StripePromotionCodeId)
}

func TestCreateTopUpRecallPromotionCodeCreatesCustomerWhenMissing(t *testing.T) {
	truncate(t)

	originalSecret := setting.StripeApiSecret
	originalTopUpPriceIds := setting.StripeTopUpPriceIds
	originalCouponCreator := topUpRecallCouponCreator
	originalPromotionCodeCreator := topUpRecallPromotionCodeCreator
	originalPriceGetter := topUpRecallPriceGetter
	originalCustomerCreator := topUpRecallCustomerCreator
	t.Cleanup(func() {
		setting.StripeApiSecret = originalSecret
		setting.StripeTopUpPriceIds = originalTopUpPriceIds
		topUpRecallCouponCreator = originalCouponCreator
		topUpRecallPromotionCodeCreator = originalPromotionCodeCreator
		topUpRecallPriceGetter = originalPriceGetter
		topUpRecallCustomerCreator = originalCustomerCreator
	})

	setting.StripeApiSecret = "sk_test_topup_recall"
	setting.StripeTopUpPriceIds = `{"5":"price_recall_5"}`
	var createdCustomer *stripe.CustomerParams
	var createdPromotionCode *stripe.PromotionCodeParams

	topUpRecallPriceGetter = func(priceId string, params *stripe.PriceParams) (*stripe.Price, error) {
		return &stripe.Price{
			ID:      priceId,
			Product: &stripe.Product{ID: "prod_wallet_topup"},
		}, nil
	}
	topUpRecallCustomerCreator = func(params *stripe.CustomerParams) (*stripe.Customer, error) {
		createdCustomer = params
		return &stripe.Customer{ID: "cus_created_for_recall"}, nil
	}
	topUpRecallCouponCreator = func(params *stripe.CouponParams) (*stripe.Coupon, error) {
		return &stripe.Coupon{ID: "coupon_recall"}, nil
	}
	topUpRecallPromotionCodeCreator = func(params *stripe.PromotionCodeParams) (*stripe.PromotionCode, error) {
		createdPromotionCode = params
		return &stripe.PromotionCode{ID: "promo_recall"}, nil
	}

	require.NoError(t, model.DB.Create(&model.User{
		Id:       911,
		Username: "topup-recall-911",
		Email:    "buyer3@example.com",
		AffCode:  "topup-recall-911",
	}).Error)

	code, promoId, err := createTopUpRecallPromotionCode(&model.TopUpRecall{
		Id:      9911,
		UserId:  911,
		TradeNo: "expired-911",
		Email:   "buyer3@example.com",
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(code, "SAVE2-"))
	require.Equal(t, "promo_recall", promoId)

	require.NotNil(t, createdCustomer)
	require.NotNil(t, createdCustomer.Email)
	require.Equal(t, "buyer3@example.com", *createdCustomer.Email)
	require.NotNil(t, createdCustomer.Name)
	require.Equal(t, "topup-recall-911", *createdCustomer.Name)

	require.NotNil(t, createdPromotionCode)
	require.NotNil(t, createdPromotionCode.Customer)
	require.Equal(t, "cus_created_for_recall", *createdPromotionCode.Customer)

	var user model.User
	require.NoError(t, model.DB.Select("stripe_customer").Where("id = ?", 911).First(&user).Error)
	require.Equal(t, "cus_created_for_recall", user.StripeCustomer)
}

func TestProcessTopUpRecallCandidateDoesNotEmailWhenPromotionCreationFails(t *testing.T) {
	truncate(t)

	originalSecret := setting.StripeApiSecret
	originalTopUpPriceIds := setting.StripeTopUpPriceIds
	originalCouponCreator := topUpRecallCouponCreator
	originalPromotionCodeCreator := topUpRecallPromotionCodeCreator
	originalPriceGetter := topUpRecallPriceGetter
	originalCustomerCreator := topUpRecallCustomerCreator
	originalEmailSender := topUpRecallEmailSender
	t.Cleanup(func() {
		setting.StripeApiSecret = originalSecret
		setting.StripeTopUpPriceIds = originalTopUpPriceIds
		topUpRecallCouponCreator = originalCouponCreator
		topUpRecallPromotionCodeCreator = originalPromotionCodeCreator
		topUpRecallPriceGetter = originalPriceGetter
		topUpRecallCustomerCreator = originalCustomerCreator
		topUpRecallEmailSender = originalEmailSender
	})

	setting.StripeApiSecret = "sk_test_topup_recall"
	setting.StripeTopUpPriceIds = `{"5":"price_recall_5"}`
	topUpRecallPriceGetter = func(priceId string, params *stripe.PriceParams) (*stripe.Price, error) {
		return &stripe.Price{
			ID:      priceId,
			Product: &stripe.Product{ID: "prod_wallet_topup"},
		}, nil
	}
	topUpRecallCustomerCreator = func(params *stripe.CustomerParams) (*stripe.Customer, error) {
		t.Fatal("customer should not be created when user already has a Stripe customer")
		return nil, nil
	}
	topUpRecallCouponCreator = func(params *stripe.CouponParams) (*stripe.Coupon, error) {
		return &stripe.Coupon{ID: "coupon_recall"}, nil
	}
	topUpRecallPromotionCodeCreator = func(params *stripe.PromotionCodeParams) (*stripe.PromotionCode, error) {
		return nil, errors.New("stripe unavailable")
	}
	topUpRecallEmailSender = func(subject string, receiver string, content string) error {
		t.Fatal("email should not be sent when promotion code creation fails")
		return nil
	}

	candidate := model.TopUpRecallCandidate{
		UserId:  910,
		TradeNo: "expired-910",
		Email:   "buyer2@example.com",
		Amount:  5,
	}

	require.NoError(t, model.DB.Create(&model.User{
		Id:             910,
		Username:       "topup-recall-910",
		Email:          "buyer2@example.com",
		AffCode:        "topup-recall-910",
		StripeCustomer: "cus_recall_910",
	}).Error)

	require.Error(t, processTopUpRecallCandidate(candidate))

	var recall model.TopUpRecall
	require.NoError(t, model.DB.Where("user_id = ?", candidate.UserId).First(&recall).Error)
	require.Equal(t, model.TopUpRecallStatusFailed, recall.Status)
	require.Contains(t, recall.Error, "stripe unavailable")
}
