package service

import (
	"errors"
	"strings"
	"testing"

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
	originalPriceId := setting.StripePriceId
	originalTopUpPriceIds := setting.StripeTopUpPriceIds
	originalCouponCreator := topUpRecallCouponCreator
	originalPromotionCodeCreator := topUpRecallPromotionCodeCreator
	originalPriceGetter := topUpRecallPriceGetter
	originalEmailSender := topUpRecallEmailSender
	originalStripeKey := stripe.Key
	t.Cleanup(func() {
		common.SystemName = originalSystemName
		setting.StripeApiSecret = originalSecret
		setting.StripePriceId = originalPriceId
		setting.StripeTopUpPriceIds = originalTopUpPriceIds
		topUpRecallCouponCreator = originalCouponCreator
		topUpRecallPromotionCodeCreator = originalPromotionCodeCreator
		topUpRecallPriceGetter = originalPriceGetter
		topUpRecallEmailSender = originalEmailSender
		stripe.Key = originalStripeKey
	})

	common.SystemName = "flatkey"
	setting.StripeApiSecret = "sk_test_topup_recall"
	setting.StripePriceId = "price_topup_5"
	setting.StripeTopUpPriceIds = ""
	stripe.Key = "sk_existing_global_key"
	var createdCoupon *stripe.CouponParams
	var createdPromotionCode *stripe.PromotionCodeParams
	var emailSubject string
	var emailReceiver string
	var emailContent string

	topUpRecallCouponCreator = func(params *stripe.CouponParams) (*stripe.Coupon, error) {
		createdCoupon = params
		return &stripe.Coupon{ID: "coupon_recall"}, nil
	}
	topUpRecallPromotionCodeCreator = func(params *stripe.PromotionCodeParams) (*stripe.PromotionCode, error) {
		createdPromotionCode = params
		return &stripe.PromotionCode{ID: "promo_recall"}, nil
	}
	topUpRecallPriceGetter = func(priceId string, params *stripe.PriceParams) (*stripe.Price, error) {
		require.Equal(t, "price_topup_5", priceId)
		return &stripe.Price{
			ID:      priceId,
			Product: &stripe.Product{ID: "prod_topup_5"},
		}, nil
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

	require.NotNil(t, createdCoupon)
	require.Equal(t, int64(200), *createdCoupon.AmountOff)
	require.Equal(t, "usd", *createdCoupon.Currency)
	require.Equal(t, string(stripe.CouponDurationOnce), *createdCoupon.Duration)
	require.Equal(t, int64(1), *createdCoupon.MaxRedemptions)
	require.NotNil(t, createdCoupon.AppliesTo)
	require.Len(t, createdCoupon.AppliesTo.Products, 1)
	require.Equal(t, "prod_topup_5", *createdCoupon.AppliesTo.Products[0])

	require.NotNil(t, createdPromotionCode)
	require.Equal(t, "coupon_recall", *createdPromotionCode.Coupon)
	require.NotNil(t, createdPromotionCode.Customer)
	require.Equal(t, "cus_recall_909", *createdPromotionCode.Customer)
	require.Equal(t, int64(1), *createdPromotionCode.MaxRedemptions)
	require.True(t, strings.HasPrefix(*createdPromotionCode.Code, "SAVE2-"))
	require.NotNil(t, createdPromotionCode.Restrictions)
	require.NotNil(t, createdPromotionCode.Restrictions.MinimumAmount)
	require.Equal(t, int64(500), *createdPromotionCode.Restrictions.MinimumAmount)
	require.NotNil(t, createdPromotionCode.Restrictions.MinimumAmountCurrency)
	require.Equal(t, "usd", *createdPromotionCode.Restrictions.MinimumAmountCurrency)
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

func TestProcessTopUpRecallCandidateDoesNotEmailWhenPromotionCreationFails(t *testing.T) {
	truncate(t)

	originalSecret := setting.StripeApiSecret
	originalPriceId := setting.StripePriceId
	originalTopUpPriceIds := setting.StripeTopUpPriceIds
	originalCouponCreator := topUpRecallCouponCreator
	originalPromotionCodeCreator := topUpRecallPromotionCodeCreator
	originalPriceGetter := topUpRecallPriceGetter
	originalEmailSender := topUpRecallEmailSender
	t.Cleanup(func() {
		setting.StripeApiSecret = originalSecret
		setting.StripePriceId = originalPriceId
		setting.StripeTopUpPriceIds = originalTopUpPriceIds
		topUpRecallCouponCreator = originalCouponCreator
		topUpRecallPromotionCodeCreator = originalPromotionCodeCreator
		topUpRecallPriceGetter = originalPriceGetter
		topUpRecallEmailSender = originalEmailSender
	})

	setting.StripeApiSecret = "sk_test_topup_recall"
	setting.StripePriceId = "price_topup_5"
	setting.StripeTopUpPriceIds = ""
	topUpRecallCouponCreator = func(params *stripe.CouponParams) (*stripe.Coupon, error) {
		return &stripe.Coupon{ID: "coupon_recall"}, nil
	}
	topUpRecallPromotionCodeCreator = func(params *stripe.PromotionCodeParams) (*stripe.PromotionCode, error) {
		return nil, errors.New("stripe unavailable")
	}
	topUpRecallPriceGetter = func(priceId string, params *stripe.PriceParams) (*stripe.Price, error) {
		return &stripe.Price{ID: priceId, Product: &stripe.Product{ID: "prod_topup_5"}}, nil
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

	require.Error(t, processTopUpRecallCandidate(candidate))

	var recall model.TopUpRecall
	require.NoError(t, model.DB.Where("user_id = ?", candidate.UserId).First(&recall).Error)
	require.Equal(t, model.TopUpRecallStatusFailed, recall.Status)
	require.Contains(t, recall.Error, "stripe unavailable")
}
