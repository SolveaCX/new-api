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
	originalCouponCreator := topUpRecallCouponCreator
	originalPromotionCodeCreator := topUpRecallPromotionCodeCreator
	originalEmailSender := topUpRecallEmailSender
	t.Cleanup(func() {
		common.SystemName = originalSystemName
		setting.StripeApiSecret = originalSecret
		topUpRecallCouponCreator = originalCouponCreator
		topUpRecallPromotionCodeCreator = originalPromotionCodeCreator
		topUpRecallEmailSender = originalEmailSender
	})

	common.SystemName = "flatkey"
	setting.StripeApiSecret = "sk_test_topup_recall"
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

	require.NoError(t, processTopUpRecallCandidate(candidate))

	require.NotNil(t, createdCoupon)
	require.Equal(t, int64(200), *createdCoupon.AmountOff)
	require.Equal(t, "usd", *createdCoupon.Currency)
	require.Equal(t, string(stripe.CouponDurationOnce), *createdCoupon.Duration)
	require.Equal(t, int64(1), *createdCoupon.MaxRedemptions)

	require.NotNil(t, createdPromotionCode)
	require.Equal(t, "coupon_recall", *createdPromotionCode.Coupon)
	require.Equal(t, int64(1), *createdPromotionCode.MaxRedemptions)
	require.True(t, strings.HasPrefix(*createdPromotionCode.Code, "SAVE2-"))

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
	originalCouponCreator := topUpRecallCouponCreator
	originalPromotionCodeCreator := topUpRecallPromotionCodeCreator
	originalEmailSender := topUpRecallEmailSender
	t.Cleanup(func() {
		setting.StripeApiSecret = originalSecret
		topUpRecallCouponCreator = originalCouponCreator
		topUpRecallPromotionCodeCreator = originalPromotionCodeCreator
		topUpRecallEmailSender = originalEmailSender
	})

	setting.StripeApiSecret = "sk_test_topup_recall"
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

	require.Error(t, processTopUpRecallCandidate(candidate))

	var recall model.TopUpRecall
	require.NoError(t, model.DB.Where("user_id = ?", candidate.UserId).First(&recall).Error)
	require.Equal(t, model.TopUpRecallStatusFailed, recall.Status)
	require.Contains(t, recall.Error, "stripe unavailable")
}
