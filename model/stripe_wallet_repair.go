package model

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// StripeWalletRepairPayment is a payment proof that has already been verified
// against Stripe by the reconciliation service. The model layer binds that
// proof to immutable local order identity before performing any mutation.
type StripeWalletRepairPayment struct {
	UserID            int
	SessionID         string
	TradeNo           string
	PriceID           string
	CustomerID        string
	AmountMinor       int64
	Currency          string
	CheckoutRevision  int64
	DiscountSelection string
	Snapshot          PaymentSnapshot
}

var ErrStripeWalletRepairRejected = errors.New("stripe wallet repair rejected")

// RefreshStripeWalletRepairAudit reads only authoritative financial audit fields.
// A successful commit whose acknowledgement was lost must still be classified as
// an automatic repair, never closed as an ordinary webhook/manual recovery.
func RefreshStripeWalletRepairAudit(ctx context.Context, check *StripeWalletPaymentCheck, token string) error {
	if check == nil || check.Id <= 0 {
		return ErrStripeWalletReconciliationLeaseLost
	}
	if err := validateStripeWalletLeaseInput(ctx, check.Scope, 1, token); err != nil {
		return err
	}
	now, err := GetDBTimestampWithContext(ctx)
	if err != nil {
		return err
	}
	var stored StripeWalletPaymentCheck
	err = DB.WithContext(ctx).Select("auto_repaired_at", "repair_from_status", "repair_credit").
		Where("id = ? AND scope = ? AND session_id = ? AND lease_token = ? AND lease_until > ?",
			check.Id, check.Scope, check.SessionID, strings.TrimSpace(token), now).Take(&stored).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrStripeWalletReconciliationLeaseLost
	}
	if err != nil {
		return err
	}
	check.AutoRepairedAt, check.RepairFromStatus, check.RepairCredit = stored.AutoRepairedAt, stored.RepairFromStatus, stored.RepairCredit
	return nil
}

// RechargeStripeWalletForReconciliation atomically credits a proven Stripe
// payment, completes the optional invoice, and writes the repair audit marker.
// The caller must still own an unexpired payment-check lease.
func RechargeStripeWalletForReconciliation(ctx context.Context, check *StripeWalletPaymentCheck, proof StripeWalletRepairPayment) (bool, error) {
	if err := validateStripeWalletRepairInput(ctx, check, &proof); err != nil {
		return false, err
	}
	token := strings.TrimSpace(check.LeaseToken)
	options := stripeWalletRechargeOptions{
		ctx:          ctx,
		sourceRef:    "stripe_wallet_reconciliation",
		strictDBTime: true,
		validate: func(tx *gorm.DB, topUp *TopUp, dbNow int64) error {
			return validateStripeWalletRepairTx(tx, check, &proof, topUp, token, dbNow)
		},
		winner: func(tx *gorm.DB, topUp *TopUp, transition *PurchaseLifecycleTransition, fromStatus string) error {
			if err := updateStripeWalletRepairInvoiceTx(tx, topUp, &proof); err != nil {
				return err
			}
			clockQuery, err := dbTimestampQueryForDB(tx)
			if err != nil {
				return err
			}
			// Evaluate the database clock in the write itself. A transaction can
			// outlive the lease after its initial validation and must then roll back.
			result := tx.Model(&StripeWalletPaymentCheck{}).
				Where("id = ? AND scope = ? AND session_id = ? AND trade_no = ? AND lease_token = ? AND lease_until > ("+clockQuery+") AND auto_repaired_at = 0",
					check.Id, strings.TrimSpace(check.Scope), proof.SessionID, proof.TradeNo, token).
				Updates(map[string]any{
					"auto_repaired_at":   transition.OccurredAt,
					"repair_from_status": fromStatus,
					"repair_credit":      transition.Credit,
					"repair_error":       "",
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrStripeWalletReconciliationLeaseLost
			}
			return nil
		},
	}

	credited, result, err := rechargeStripeWallet(proof.TradeNo, proof.CustomerID, "", proof.Snapshot, options)
	if err != nil {
		return false, err
	}
	if result != nil && result.repairedAt > 0 {
		check.AutoRepairedAt = result.repairedAt
		check.RepairFromStatus = result.repairFromStatus
		check.RepairCredit = int64(result.quotaToAdd)
		check.RepairError = ""
	}
	return credited, nil
}

func validateStripeWalletRepairInput(ctx context.Context, check *StripeWalletPaymentCheck, proof *StripeWalletRepairPayment) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if check == nil || check.Id <= 0 {
		return fmt.Errorf("%w: payment check is required", ErrStripeWalletRepairRejected)
	}
	check.Scope = strings.TrimSpace(check.Scope)
	check.SessionID = strings.TrimSpace(check.SessionID)
	check.TradeNo = strings.TrimSpace(check.TradeNo)
	check.LeaseToken = strings.TrimSpace(check.LeaseToken)
	proof.SessionID = strings.TrimSpace(proof.SessionID)
	proof.TradeNo = strings.TrimSpace(proof.TradeNo)
	proof.PriceID = strings.TrimSpace(proof.PriceID)
	proof.CustomerID = strings.TrimSpace(proof.CustomerID)
	proof.Currency = strings.ToUpper(strings.TrimSpace(proof.Currency))
	proof.DiscountSelection = strings.TrimSpace(proof.DiscountSelection)
	proof.Snapshot.Currency = strings.ToUpper(strings.TrimSpace(proof.Snapshot.Currency))
	if check.Scope == "" || check.SessionID == "" || check.TradeNo == "" || check.LeaseToken == "" {
		return fmt.Errorf("%w: payment check identity and lease are required", ErrStripeWalletRepairRejected)
	}
	if proof.UserID <= 0 || proof.SessionID == "" || proof.TradeNo == "" || proof.PriceID == "" || proof.CustomerID == "" || proof.AmountMinor <= 0 || proof.Currency == "" {
		return fmt.Errorf("%w: payment proof is incomplete", ErrStripeWalletRepairRejected)
	}
	if proof.SessionID != check.SessionID || proof.TradeNo != check.TradeNo {
		return fmt.Errorf("%w: payment proof does not match check", ErrStripeWalletRepairRejected)
	}
	if check.UserID != 0 && check.UserID != proof.UserID {
		return fmt.Errorf("%w: payment check user mismatch", ErrStripeWalletRepairRejected)
	}
	if proof.Snapshot.Money <= 0 || proof.Snapshot.Currency != proof.Currency {
		return fmt.Errorf("%w: payment snapshot does not match proof", ErrStripeWalletRepairRejected)
	}
	return nil
}

func validateStripeWalletRepairTx(tx *gorm.DB, check *StripeWalletPaymentCheck, proof *StripeWalletRepairPayment, topUp *TopUp, token string, dbNow int64) error {
	var stored StripeWalletPaymentCheck
	if err := lockQuery(tx).Where("id = ?", check.Id).First(&stored).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrStripeWalletReconciliationLeaseLost
		}
		return err
	}
	if stored.Scope != strings.TrimSpace(check.Scope) || stored.SessionID != proof.SessionID || stored.TradeNo != proof.TradeNo || stored.LeaseToken != token || stored.LeaseUntil <= dbNow {
		return ErrStripeWalletReconciliationLeaseLost
	}
	if stored.ClosedAt != 0 || strings.TrimSpace(stored.DiscoveryError) != "" || strings.TrimSpace(stored.VerificationError) != "" || stored.VerificationPending {
		return fmt.Errorf("%w: payment check is not eligible for repair", ErrStripeWalletRepairRejected)
	}
	if stored.Amount != proof.AmountMinor || !strings.EqualFold(strings.TrimSpace(stored.Currency), proof.Currency) {
		return fmt.Errorf("%w: payment amount snapshot mismatch", ErrStripeWalletRepairRejected)
	}
	if stored.AutoRepairedAt > 0 {
		return ErrStripeWalletRepairRejected
	}
	if topUp.UserId != proof.UserID || (stored.UserID != 0 && stored.UserID != proof.UserID) {
		return fmt.Errorf("%w: user identity mismatch", ErrStripeWalletRepairRejected)
	}
	if topUp.PaymentProvider != PaymentProviderStripe || topUp.Amount <= 0 {
		return fmt.Errorf("%w: local order is not a positive Stripe wallet top-up", ErrStripeWalletRepairRejected)
	}
	if strings.TrimSpace(topUp.GatewayTradeNo) == "" || strings.TrimSpace(topUp.GatewayTradeNo) != proof.SessionID {
		return fmt.Errorf("%w: checkout session mismatch", ErrStripeWalletRepairRejected)
	}
	if strings.TrimSpace(topUp.PaymentPriceId) == "" || strings.TrimSpace(topUp.PaymentPriceId) != proof.PriceID {
		return fmt.Errorf("%w: Stripe price mismatch", ErrStripeWalletRepairRejected)
	}
	if topUp.CheckoutRevision != proof.CheckoutRevision {
		return fmt.Errorf("%w: checkout revision mismatch", ErrStripeWalletRepairRejected)
	}
	var subscriptionOrderCount int64
	if err := tx.Model(&SubscriptionOrder{}).Where("trade_no = ?", proof.TradeNo).Limit(1).Count(&subscriptionOrderCount).Error; err != nil {
		return err
	}
	if subscriptionOrderCount != 0 {
		return fmt.Errorf("%w: trade number belongs to a subscription order", ErrStripeWalletRepairRejected)
	}
	var user User
	if err := lockQuery(tx).Select("id", "stripe_customer").Where("id = ?", proof.UserID).First(&user).Error; err != nil {
		return err
	}
	if customerID := strings.TrimSpace(user.StripeCustomer); customerID != "" && customerID != proof.CustomerID {
		return fmt.Errorf("%w: Stripe customer mismatch", ErrStripeWalletRepairRejected)
	}
	status := normalizePurchaseLifecycleStatus(topUp.Status)
	if !purchaseLifecycleStatusAllowed(status, topUpSuccessFromStatuses()) {
		return fmt.Errorf("%w: local order status %q cannot be repaired", ErrStripeWalletRepairRejected, status)
	}
	alreadyCredited, err := stripeWalletTopUpSuccessEventExistsTx(tx, topUp)
	if err != nil {
		return err
	}
	if alreadyCredited {
		return fmt.Errorf("%w: top-up success ledger already exists", ErrStripeWalletRepairRejected)
	}
	return validateStripeWalletRepairRevisionTx(tx, topUp, proof)
}

func validateStripeWalletRepairRevisionTx(tx *gorm.DB, topUp *TopUp, proof *StripeWalletRepairPayment) error {
	if proof.CheckoutRevision == 0 {
		// Hosted/default checkouts explicitly emit revision 0 + none without
		// creating a revision ledger; older checkouts omit both metadata fields.
		if proof.DiscountSelection == "" || proof.DiscountSelection == "none" {
			return nil
		}
		return fmt.Errorf("%w: unversioned checkout discount requires manual review", ErrStripeWalletRepairRejected)
	}
	switch proof.DiscountSelection {
	case "none", "invitation", "recall", "manual":
	default:
		return fmt.Errorf("%w: unsupported discount selection", ErrStripeWalletRepairRejected)
	}
	var revision StripeCheckoutRevision
	err := lockQuery(tx).Where("order_type = ? AND trade_no = ? AND revision = ?", StripeCheckoutOrderTopUp, topUp.TradeNo, proof.CheckoutRevision).First(&revision).Error
	if err != nil {
		return fmt.Errorf("%w: checkout revision not found", ErrStripeWalletRepairRejected)
	}
	if revision.UserId != proof.UserID || revision.DiscountSource != proof.DiscountSelection || revision.ProviderSessionId == nil || strings.TrimSpace(*revision.ProviderSessionId) != proof.SessionID {
		return fmt.Errorf("%w: checkout revision identity mismatch", ErrStripeWalletRepairRejected)
	}
	if revision.State != StripeCheckoutRevisionStateActive {
		return fmt.Errorf("%w: checkout revision is not active", ErrStripeWalletRepairRejected)
	}
	return nil
}

func stripeWalletTopUpSuccessEventExistsTx(tx *gorm.DB, topUp *TopUp) (bool, error) {
	occurrence, err := NewRecallLifecyclePurchaseOccurrence(RecallLifecycleTriggerPaymentSucceeded, PurchaseLifecycleKindTopUp, topUp.TradeNo, purchaseLifecycleTopUpTable, int64(topUp.Id), topUp.UserId)
	if err != nil {
		return false, err
	}
	var count int64
	err = tx.Model(&RecallLifecycleEvent{}).Where("event_type = ? AND occurrence_key_hash = ?", RecallLifecycleTriggerPaymentSucceeded, occurrence.Hash).Count(&count).Error
	return count > 0, err
}

func updateStripeWalletRepairInvoiceTx(tx *gorm.DB, topUp *TopUp, proof *StripeWalletRepairPayment) error {
	var invoice PaymentInvoice
	result := lockQuery(tx).Where("trade_no = ?", topUp.TradeNo).Limit(1).Find(&invoice)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	if invoice.UserId != proof.UserID || invoice.OrderType != PaymentOrderTypeTopUp || invoice.PaymentProvider != PaymentProviderStripe {
		return fmt.Errorf("%w: invoice identity mismatch", ErrStripeWalletRepairRejected)
	}
	if sessionID := strings.TrimSpace(invoice.StripeCheckoutSessionId); sessionID != "" && sessionID != proof.SessionID {
		return fmt.Errorf("%w: invoice checkout session mismatch", ErrStripeWalletRepairRejected)
	}
	if customerID := strings.TrimSpace(invoice.StripeCustomerId); customerID != "" && customerID != proof.CustomerID {
		return fmt.Errorf("%w: invoice customer mismatch", ErrStripeWalletRepairRejected)
	}
	switch strings.TrimSpace(invoice.InvoiceStatus) {
	case PaymentInvoiceStatusRequested, PaymentInvoiceStatusPending, PaymentInvoiceStatusFailed, PaymentInvoiceStatusExpired, PaymentInvoiceStatusPaid:
	default:
		return fmt.Errorf("%w: invoice status cannot be repaired", ErrStripeWalletRepairRejected)
	}
	updates := map[string]any{
		"stripe_checkout_session_id": proof.SessionID,
		"invoice_status":             PaymentInvoiceStatusPaid,
	}
	if proof.CustomerID != "" {
		updates["stripe_customer_id"] = proof.CustomerID
	}
	return tx.Model(&PaymentInvoice{}).Where("id = ?", invoice.Id).Updates(updates).Error
}
