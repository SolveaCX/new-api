package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
	"github.com/stripe/stripe-go/v86"
)

func stripeWalletRepairableStatus(status string) bool {
	switch status {
	case common.TopUpStatusPending, common.TopUpStatusFailed, common.TopUpStatusExpired, "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func (r *stripeWalletReconciler) repairPaidOrder(ctx context.Context, check *model.StripeWalletPaymentCheck) error {
	// Cached paid observations are adequate for alert follow-ups, never for a new
	// balance mutation. Always read current charge/refund evidence before credit.
	session, err := r.gateway.GetRepairSession(ctx, check.SessionID)
	if err != nil {
		return fmt.Errorf("automatic repair deferred: %w", err)
	}
	proof, err := stripeWalletRepairProof(check, session)
	if err != nil {
		return err
	}
	fulfill := r.repair
	if fulfill == nil {
		fulfill = model.RechargeStripeWalletForReconciliation
	}
	_, err = fulfill(ctx, check, proof)
	return err
}

func stripeWalletRepairProof(check *model.StripeWalletPaymentCheck, session *stripe.CheckoutSession) (model.StripeWalletRepairPayment, error) {
	proof := model.StripeWalletRepairPayment{}
	if session == nil || check.UserID <= 0 || check.DiscoveryError != "" || check.VerificationError != "" ||
		session.ID != check.SessionID || stripeWalletSubscriptionSession(session) ||
		session.Status != stripe.CheckoutSessionStatusComplete || session.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid ||
		session.Livemode != strings.HasSuffix(check.Scope, ":live") {
		return proof, errors.New("automatic repair blocked: current paid wallet session identity is not verified")
	}
	tradeNo, valid := stripeWalletReference(session)
	if !valid || tradeNo != check.TradeNo {
		return proof, errors.New("automatic repair blocked: current session order reference conflicts with payment event")
	}
	if check.Amount <= 0 || session.AmountTotal != check.Amount || len(session.Currency) != 3 ||
		!strings.EqualFold(string(session.Currency), check.Currency) {
		return proof, errors.New("automatic repair blocked: current payment amount/currency conflicts with payment event")
	}
	items := session.LineItems
	if items == nil || items.HasMore || len(items.Data) != 1 || items.Data[0] == nil ||
		items.Data[0].Price == nil || items.Data[0].Price.ID == "" || items.Data[0].Quantity != 1 {
		return proof, errors.New("automatic repair blocked: expected one wallet price with quantity one")
	}
	pi := session.PaymentIntent
	if pi == nil || pi.ID == "" || pi.Status != stripe.PaymentIntentStatusSucceeded || pi.Livemode != session.Livemode ||
		pi.Amount != session.AmountTotal || pi.AmountReceived != session.AmountTotal || pi.Currency != session.Currency {
		return proof, errors.New("automatic repair blocked: payment intent is incomplete or conflicts with the paid session")
	}
	charge := pi.LatestCharge
	if charge == nil || charge.ID == "" || charge.Status != stripe.ChargeStatusSucceeded || !charge.Paid || !charge.Captured ||
		charge.Livemode != session.Livemode || charge.PaymentIntent == nil || charge.PaymentIntent.ID != pi.ID ||
		charge.Amount != session.AmountTotal || charge.AmountCaptured != session.AmountTotal || charge.Currency != session.Currency ||
		charge.AmountRefunded != 0 || charge.Refunded || charge.Disputed {
		return proof, errors.New("automatic repair blocked: charge is incomplete, refunded, disputed, or conflicts with the payment")
	}
	if session.Customer == nil || strings.TrimSpace(session.Customer.ID) == "" {
		return proof, errors.New("automatic repair blocked: session customer identity is missing")
	}
	customerID := strings.TrimSpace(session.Customer.ID)
	for _, customer := range []*stripe.Customer{pi.Customer, charge.Customer} {
		if customer == nil || strings.TrimSpace(customer.ID) != customerID {
			return proof, errors.New("automatic repair blocked: payment customer identity is missing or mismatched")
		}
	}
	for _, metadata := range []map[string]string{session.Metadata, pi.Metadata, charge.Metadata} {
		if userID := strings.TrimSpace(metadata["user_id"]); userID != "" && userID != strconv.Itoa(check.UserID) {
			return proof, errors.New("automatic repair blocked: payment metadata user identity mismatch")
		}
		if reference := strings.TrimSpace(metadata["trade_no"]); reference != "" && reference != check.TradeNo {
			return proof, errors.New("automatic repair blocked: payment metadata order identity mismatch")
		}
	}
	revision := int64(0)
	if raw := strings.TrimSpace(session.Metadata["checkout_revision"]); raw != "" {
		var err error
		revision, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || revision < 0 {
			return proof, errors.New("automatic repair blocked: invalid checkout revision")
		}
	} else if strings.TrimSpace(session.Metadata["discount_selection"]) != "" {
		return proof, errors.New("automatic repair blocked: checkout revision missing")
	}
	// The immutable contract is the local price ID + quantity, not the wallet
	// face value: discounts, tax and Adaptive Pricing can change actual charges.
	proof = model.StripeWalletRepairPayment{
		UserID: check.UserID, SessionID: session.ID, TradeNo: tradeNo,
		PriceID: items.Data[0].Price.ID, CustomerID: customerID,
		AmountMinor: session.AmountTotal, Currency: strings.ToUpper(string(session.Currency)),
		CheckoutRevision: revision, DiscountSelection: strings.TrimSpace(session.Metadata["discount_selection"]),
		Snapshot: stripeWalletRepairPaymentSnapshot(session.AmountTotal, string(session.Currency)),
	}
	return proof, nil
}

func stripeWalletRepairPaymentSnapshot(amount int64, currency string) model.PaymentSnapshot {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	divisor := int64(100)
	switch currency {
	case "BIF", "CLP", "DJF", "GNF", "JPY", "KMF", "KRW", "MGA", "PYG", "RWF", "VND", "VUV", "XAF", "XOF", "XPF":
		divisor = 1
	}
	// Stripe retains two-decimal API representation for ISK and UGX. HUF/TWD
	// zero-decimal rules apply to payouts, not these charges.
	money, _ := decimal.NewFromInt(amount).Div(decimal.NewFromInt(divisor)).Float64()
	return model.PaymentSnapshot{Money: money, Currency: currency}
}
