package controller

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	stripecoupon "github.com/stripe/stripe-go/v81/coupon"
	"github.com/thanhpk/randstr"
)

type SubscriptionStripePayRequest struct {
	PlanId      int    `json:"plan_id"`
	RecallClaim string `json:"recall_claim,omitempty"`
}

func SubscriptionRequestStripePay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req SubscriptionStripePayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !plan.Enabled {
		common.ApiErrorMsg(c, "套餐未启用")
		return
	}
	if plan.StripePriceId == "" {
		common.ApiErrorMsg(c, "该套餐未配置 StripePriceId")
		return
	}
	if !strings.HasPrefix(setting.StripeApiSecret, "sk_") && !strings.HasPrefix(setting.StripeApiSecret, "rk_") {
		common.ApiErrorMsg(c, "Stripe 未配置或密钥无效")
		return
	}
	if setting.StripeWebhookSecret == "" {
		common.ApiErrorMsg(c, "Stripe Webhook 未配置")
		return
	}

	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}

	if plan.MaxPurchasePerUser > 0 {
		count, err := model.CountUserSubscriptionsByPlan(userId, plan.Id)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			common.ApiErrorMsg(c, "已达到该套餐购买上限")
			return
		}
	}

	reference := fmt.Sprintf("sub-stripe-ref-%d-%d-%s", user.Id, time.Now().UnixMilli(), randstr.String(4))
	referenceId := "sub_ref_" + common.Sha1([]byte(reference))

	// 被邀用户首次订阅：首月立减（前端与邀请页承诺的口径）。折扣判定与
	// 订单创建同事务、锁用户行——并发多开 checkout 只有一单能占到折扣。
	order := &model.SubscriptionOrder{
		UserId:          userId,
		PlanId:          plan.Id,
		TradeNo:         referenceId,
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := model.CreateSubscriptionOrderWithInviteDiscount(order, plan.PriceAmount, 0); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	var recallDiscount *service.RecallCheckoutDiscount
	if strings.TrimSpace(req.RecallClaim) != "" {
		recallDiscount, err = service.GetRecallRuntime().Claims.BuildCheckoutDiscount(
			c.Request.Context(),
			userId,
			req.RecallClaim,
			service.RecallPurchaseKindSubscription,
			plan.StripePriceId,
		)
		if err != nil {
			logger.LogWarn(c.Request.Context(), fmt.Sprintf("Stripe subscription recall claim rejected user_id=%d trade_no=%s plan_id=%d error=%q", userId, referenceId, plan.Id, err.Error()))
			order.Status = common.TopUpStatusFailed
			_ = order.Update()
			c.JSON(http.StatusOK, gin.H{"message": "error", "data": i18n.T(c, i18n.MsgPaymentRecallClaimUnavailable)})
			return
		}
	}

	// Stripe Checkout accepts only one discount. A targeted recall promotion
	// takes precedence over the generic invitee first-subscription discount;
	// release the invite slot and keep the recorded amount aligned with Stripe.
	if recallDiscount != nil && order.DiscountUSD > 0 {
		order.Money = plan.PriceAmount
		order.DiscountUSD = 0
		if err := order.Update(); err != nil {
			order.Status = common.TopUpStatusFailed
			_ = order.Update()
			c.JSON(http.StatusOK, gin.H{"message": "error", "data": "更新订单折扣失败"})
			return
		}
	}

	payLink, err := genStripeSubscriptionLink(referenceId, user.StripeCustomer, user.Email, plan.StripePriceId, recallDiscount, order.DiscountUSD)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 订阅支付链接创建失败 trade_no=%s plan_id=%d error=%q", referenceId, plan.Id, err.Error()))
		order.Status = common.TopUpStatusFailed
		_ = order.Update()
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"pay_link": payLink,
		},
	})
}

func genStripeSubscriptionLink(referenceId string, customerId string, email string, priceId string, recall *service.RecallCheckoutDiscount, discountUSD float64) (string, error) {
	stripe.Key = setting.StripeApiSecret

	params := &stripe.CheckoutSessionParams{
		ClientReferenceID: stripe.String(referenceId),
		SuccessURL:        stripe.String(consolePaymentReturnPath("/console/topup")),
		CancelURL:         stripe.String(consolePaymentReturnPath("/console/topup")),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceId),
				Quantity: stripe.Int64(1),
			},
		},
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		// Same 3DS posture as top-up checkouts (see buildStripeCheckoutSessionParams):
		// request the cardholder challenge whenever the card is enrolled so stolen-card
		// traffic can't open subscriptions, with issuer-side liability shift.
		PaymentMethodOptions: &stripe.CheckoutSessionPaymentMethodOptionsParams{
			Card: &stripe.CheckoutSessionPaymentMethodOptionsCardParams{
				RequestThreeDSecure: stripe.String(string(stripe.CheckoutSessionPaymentMethodOptionsCardRequestThreeDSecureAny)),
			},
		},
	}
	if recall != nil {
		params.Discounts = []*stripe.CheckoutSessionDiscountParams{{
			PromotionCode: stripe.String(recall.PromotionCodeID),
		}}
		params.Metadata = map[string]string{
			"recall_campaign_id":  strconv.FormatInt(recall.CampaignID, 10),
			"recall_recipient_id": strconv.FormatInt(recall.RecipientID, 10),
		}
	} else if discountUSD <= 0 {
		params.AllowPromotionCodes = stripe.Bool(true)
	}

	if "" == customerId {
		if "" != email {
			params.CustomerEmail = stripe.String(email)
		}
		// Do NOT set CustomerCreation here: Stripe rejects it outside payment
		// mode ("customer_creation can only be used in payment mode"), and
		// subscription-mode checkouts always create a customer anyway.
	} else {
		params.Customer = stripe.String(customerId)
	}

	// 被邀首订折扣：一次性 coupon 只作用于首期账单，续费恢复原价。
	if recall == nil && discountUSD > 0 {
		couponParams := &stripe.CouponParams{
			AmountOff: stripe.Int64(int64(math.Round(discountUSD * 100))),
			Currency:  stripe.String(string(stripe.CurrencyUSD)),
			Duration:  stripe.String(string(stripe.CouponDurationOnce)),
			Name:      stripe.String("Invite first-month discount"),
		}
		cp, err := stripecoupon.New(couponParams)
		if err != nil {
			return "", fmt.Errorf("create invite discount coupon: %w", err)
		}
		params.Discounts = []*stripe.CheckoutSessionDiscountParams{
			{Coupon: stripe.String(cp.ID)},
		}
	}

	result, err := session.New(params)
	if err != nil {
		return "", err
	}
	return result.URL, nil
}
