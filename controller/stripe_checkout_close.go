package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type stripeCheckoutCloseRequest struct {
	TradeNo string `json:"trade_no"`
}

type stripeCheckoutCloseRuntime struct {
	GetSession            func(context.Context, service.StripeCheckoutPurchaseKind, string) (*stripeCheckoutSessionSnapshot, error)
	ExpireSession         func(context.Context, service.StripeCheckoutPurchaseKind, string) (*stripeCheckoutSessionSnapshot, error)
	FailTopUp             func(string) error
	FailInvoice           func(string) error
	TerminateSubscription func(context.Context, string, string) error
}

var currentStripeCheckoutCloseRuntime = defaultStripeCheckoutCloseRuntime()

func defaultStripeCheckoutCloseRuntime() stripeCheckoutCloseRuntime {
	return stripeCheckoutCloseRuntime{
		GetSession:    getStripeCheckoutSessionSnapshot,
		ExpireSession: expireStripeCheckoutSessionSnapshot,
		FailTopUp: func(tradeNo string) error {
			return model.UpdatePendingTopUpStatus(tradeNo, model.PaymentProviderStripe, common.TopUpStatusFailed)
		},
		FailInvoice: func(tradeNo string) error {
			return model.UpdatePaymentInvoiceStatus(tradeNo, model.PaymentInvoiceStatusFailed)
		},
		TerminateSubscription: service.TerminatePendingStripePurchase,
	}
}

func replaceStripeCheckoutCloseRuntimeForTest(replacement stripeCheckoutCloseRuntime) func() {
	original := currentStripeCheckoutCloseRuntime
	updated := original
	if replacement.GetSession != nil {
		updated.GetSession = replacement.GetSession
	}
	if replacement.ExpireSession != nil {
		updated.ExpireSession = replacement.ExpireSession
	}
	if replacement.FailTopUp != nil {
		updated.FailTopUp = replacement.FailTopUp
	}
	if replacement.FailInvoice != nil {
		updated.FailInvoice = replacement.FailInvoice
	}
	if replacement.TerminateSubscription != nil {
		updated.TerminateSubscription = replacement.TerminateSubscription
	}
	currentStripeCheckoutCloseRuntime = updated
	return func() { currentStripeCheckoutCloseRuntime = original }
}

// CloseStripeCheckout terminates an unpaid in-console Stripe Checkout purchase.
// Stripe provider state is checked first so a concurrent paid webhook remains
// authoritative. The endpoint is intentionally idempotent for local terminal
// states because the frontend sends it as a best-effort dismissal signal.
func CloseStripeCheckout(c *gin.Context) {
	var request stripeCheckoutCloseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "invalid parameters")
		return
	}
	tradeNo := strings.TrimSpace(request.TradeNo)
	if tradeNo == "" {
		common.ApiErrorMsg(c, "trade_no is required")
		return
	}

	userID := c.GetInt("id")
	if userID <= 0 {
		common.ApiErrorMsg(c, "unauthorized")
		return
	}

	// A one-time subscription is mirrored into top_ups after lifecycle work, so
	// resolve the subscription row first to avoid treating that mirror as a
	// wallet top-up on a repeated close.
	var order model.SubscriptionOrder
	orderErr := model.DB.Where("trade_no = ?", tradeNo).First(&order).Error
	if orderErr == nil {
		if order.UserId != userID || order.PaymentProvider != model.PaymentProviderStripe {
			common.ApiErrorMsg(c, "Stripe checkout order not found")
			return
		}
		status := strings.TrimSpace(order.Status)
		if status == common.TopUpStatusSuccess {
			writeStripeCheckoutCloseSuccess(c, "paid")
			return
		}
		if status != common.TopUpStatusPending {
			writeStripeCheckoutCloseSuccess(c, "failed")
			return
		}
		kind := service.StripeCheckoutPurchaseOneTimeSubscription
		if strings.EqualFold(strings.TrimSpace(order.PaymentMethod), model.PaymentMethodStripe) {
			kind = service.StripeCheckoutPurchaseRecurringSubscription
		}
		if err := closePendingStripeCheckout(c.Request.Context(), kind, tradeNo, strings.TrimSpace(order.ProviderSessionId), false); err != nil {
			writeStripeCheckoutCloseError(c, err)
			return
		}
		writeStripeCheckoutCloseSuccess(c, "failed")
		return
	}
	if !errors.Is(orderErr, gorm.ErrRecordNotFound) {
		writeStripeCheckoutCloseError(c, orderErr)
		return
	}

	topUp, topUpErr := model.GetTopUpByTradeNoWithError(tradeNo)
	if errors.Is(topUpErr, model.ErrTopUpNotFound) {
		common.ApiErrorMsg(c, "Stripe checkout order not found")
		return
	}
	if topUpErr != nil {
		writeStripeCheckoutCloseError(c, topUpErr)
		return
	}
	if topUp == nil || topUp.UserId != userID || topUp.PaymentProvider != model.PaymentProviderStripe {
		common.ApiErrorMsg(c, "Stripe checkout order not found")
		return
	}
	if topUp.Status == common.TopUpStatusSuccess {
		writeStripeCheckoutCloseSuccess(c, "paid")
		return
	}
	if topUp.Status != common.TopUpStatusPending {
		writeStripeCheckoutCloseSuccess(c, "failed")
		return
	}
	if err := closePendingStripeCheckout(c.Request.Context(), service.StripeCheckoutPurchaseTopUp, tradeNo, strings.TrimSpace(topUp.GatewayTradeNo), true); err != nil {
		writeStripeCheckoutCloseError(c, err)
		return
	}
	writeStripeCheckoutCloseSuccess(c, "failed")
}

func closePendingStripeCheckout(ctx context.Context, kind service.StripeCheckoutPurchaseKind, tradeNo string, sessionID string, topUp bool) error {
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("Stripe checkout session is missing")
	}
	snapshot, err := currentStripeCheckoutCloseRuntime.GetSession(ctx, kind, sessionID)
	if err != nil {
		return fmt.Errorf("Stripe checkout session lookup failed: %w", err)
	}
	if stripeCheckoutSessionCompleted(snapshot) {
		return errStripeCheckoutAlreadyCompleted
	}
	if !stripeCheckoutSessionExpired(snapshot) {
		snapshot, err = currentStripeCheckoutCloseRuntime.ExpireSession(ctx, kind, sessionID)
		if err != nil {
			return fmt.Errorf("Stripe checkout session expiration failed: %w", err)
		}
		if stripeCheckoutSessionCompleted(snapshot) {
			return errStripeCheckoutAlreadyCompleted
		}
		if !stripeCheckoutSessionExpired(snapshot) {
			return errors.New("Stripe checkout session did not confirm unpaid expiration")
		}
	}

	if topUp {
		if err := currentStripeCheckoutCloseRuntime.FailTopUp(tradeNo); err != nil {
			return reconcileStripeCheckoutCloseRace(tradeNo, err)
		}
		if err := currentStripeCheckoutCloseRuntime.FailInvoice(tradeNo); err != nil && !errors.Is(err, model.ErrPaymentInvoiceNotFound) {
			return err
		}
		return nil
	}
	if err := currentStripeCheckoutCloseRuntime.TerminateSubscription(ctx, tradeNo, model.SubscriptionChangeIntentStatusFailed); err != nil {
		return reconcileStripeCheckoutCloseRace(tradeNo, err)
	}
	return nil
}

func reconcileStripeCheckoutCloseRace(tradeNo string, original error) error {
	var order model.SubscriptionOrder
	if err := model.DB.Where("trade_no = ?", tradeNo).First(&order).Error; err == nil {
		if order.Status == common.TopUpStatusSuccess {
			return errStripeCheckoutAlreadyCompleted
		}
		if order.Status != common.TopUpStatusPending {
			return nil
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return original
	}
	if topUp, err := model.GetTopUpByTradeNoWithError(tradeNo); err == nil && topUp != nil {
		if topUp.Status == common.TopUpStatusSuccess {
			return errStripeCheckoutAlreadyCompleted
		}
		if topUp.Status != common.TopUpStatusPending {
			return nil
		}
	}
	return original
}

func writeStripeCheckoutCloseSuccess(c *gin.Context, status string) {
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "success", "data": gin.H{"status": status}})
}

func writeStripeCheckoutCloseError(c *gin.Context, err error) {
	if errors.Is(err, errStripeCheckoutAlreadyCompleted) {
		writeStripeCheckoutCloseSuccess(c, "paid")
		return
	}
	common.ApiError(c, err)
}
