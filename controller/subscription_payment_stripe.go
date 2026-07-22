package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
)

type SubscriptionStripePayRequest struct {
	PlanId    int    `json:"plan_id"`
	RequestId string `json:"request_id"`
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

	common.ApiErrorMsg(c, "stripe subscription checkout is pending migration")
}

func genStripeSubscriptionLink(referenceId string, customerId string, email string, priceId string, userId int, planId int) (*stripe.CheckoutSession, error) {
	stripe.Key = setting.StripeApiSecret

	params := buildStripeSubscriptionCheckoutSessionParams(referenceId, customerId, email, priceId, userId, planId)

	result, err := session.New(params)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func buildStripeSubscriptionCheckoutSessionParams(referenceId string, customerId string, email string, priceId string, userId int, planId int) *stripe.CheckoutSessionParams {
	metadata := map[string]string{
		"newapi_trade_no": strings.TrimSpace(referenceId),
		"newapi_user_id":  strconv.Itoa(userId),
		"newapi_plan_id":  strconv.Itoa(planId),
	}
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
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		Metadata: metadata,
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: metadata,
		},
	}

	if "" == customerId {
		if "" != email {
			params.CustomerEmail = stripe.String(email)
		}
		params.CustomerCreation = stripe.String(string(stripe.CheckoutSessionCustomerCreationAlways))
	} else {
		params.Customer = stripe.String(customerId)
	}

	return params
}
