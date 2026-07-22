package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/shopspring/decimal"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	stripeinvoice "github.com/stripe/stripe-go/v81/invoice"
	stripesubscription "github.com/stripe/stripe-go/v81/subscription"
	"gorm.io/gorm"
)

type StripeSubscriptionCheckoutInput struct {
	TradeNo        string
	UserID         int
	PlanID         int
	ContractID     int64
	ChangeIntentID int64
	CustomerID     string
	Email          string
	PriceID        string
	IdempotencyKey string
}

type StripeSubscriptionCheckoutSession struct {
	ID  string
	URL string
}

type PaidInvoiceReconcileResult struct {
	Binding     *model.SubscriptionProviderBinding
	Entitlement *model.UserSubscription
	Applied     bool
}

type paidInvoicePermanentError struct {
	err error
}

func (e paidInvoicePermanentError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e paidInvoicePermanentError) Unwrap() error {
	return e.err
}

func PermanentPaidInvoiceError(err error) error {
	if err == nil {
		return nil
	}
	var permanent paidInvoicePermanentError
	if errors.As(err, &permanent) {
		return err
	}
	return paidInvoicePermanentError{err: err}
}

func IsPermanentPaidInvoiceError(err error) bool {
	var permanent paidInvoicePermanentError
	return errors.As(err, &permanent)
}

var stripeInvoiceGetter = getStripeInvoiceForReconcile
var stripeSubscriptionGetter = getStripeSubscriptionForReconcile
var stripeSubscriptionCheckoutCreator = createStripeSubscriptionCheckout

func getStripeInvoiceForReconcile(ctx context.Context, invoiceID string) (*stripe.Invoice, error) {
	if err := ensureStripeSecretForSubscription(); err != nil {
		return nil, err
	}
	stripe.Key = setting.StripeApiSecret
	params := &stripe.InvoiceParams{}
	params.AddExpand("lines.data.price")
	params.AddExpand("subscription")
	params.AddExpand("customer")
	return stripeinvoice.Get(strings.TrimSpace(invoiceID), params)
}

func getStripeSubscriptionForReconcile(ctx context.Context, subscriptionID string) (*stripe.Subscription, error) {
	if err := ensureStripeSecretForSubscription(); err != nil {
		return nil, err
	}
	stripe.Key = setting.StripeApiSecret
	params := &stripe.SubscriptionParams{}
	params.AddExpand("latest_invoice")
	params.AddExpand("items.data.price")
	params.AddExpand("customer")
	return stripesubscription.Get(strings.TrimSpace(subscriptionID), params)
}

func createStripeSubscriptionCheckout(ctx context.Context, input StripeSubscriptionCheckoutInput) (*StripeSubscriptionCheckoutSession, error) {
	if err := ensureStripeSecretForSubscription(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.PriceID) == "" {
		return nil, errors.New("Stripe subscription price id is required")
	}
	stripe.Key = setting.StripeApiSecret
	metadata := stripeSubscriptionAuthoritativeMetadata(input.TradeNo, input.UserID, input.PlanID, input.ContractID, input.ChangeIntentID)
	params := &stripe.CheckoutSessionParams{
		ClientReferenceID: stripe.String(input.TradeNo),
		SuccessURL:        stripe.String(consoleSubscriptionReturnPath()),
		CancelURL:         stripe.String(consoleSubscriptionReturnPath()),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(input.PriceID),
				Quantity: stripe.Int64(1),
			},
		},
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		Metadata: metadata,
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: metadata,
		},
	}
	if strings.TrimSpace(input.CustomerID) != "" {
		params.Customer = stripe.String(strings.TrimSpace(input.CustomerID))
	} else {
		if strings.TrimSpace(input.Email) != "" {
			params.CustomerEmail = stripe.String(strings.TrimSpace(input.Email))
		}
		params.CustomerCreation = stripe.String(string(stripe.CheckoutSessionCustomerCreationAlways))
	}
	if strings.TrimSpace(input.IdempotencyKey) != "" {
		params.SetIdempotencyKey(strings.TrimSpace(input.IdempotencyKey))
	}
	created, err := session.New(params)
	if err != nil {
		return nil, err
	}
	if created == nil || strings.TrimSpace(created.ID) == "" || strings.TrimSpace(created.URL) == "" {
		return nil, errors.New("Stripe checkout session missing id or url")
	}
	return &StripeSubscriptionCheckoutSession{
		ID:  strings.TrimSpace(created.ID),
		URL: strings.TrimSpace(created.URL),
	}, nil
}

func stripeSubscriptionAuthoritativeMetadata(tradeNo string, userID int, planID int, contractID int64, changeIntentID int64) map[string]string {
	return map[string]string{
		"trade_no":         strings.TrimSpace(tradeNo),
		"user_id":          strconv.Itoa(userID),
		"plan_id":          strconv.Itoa(planID),
		"contract_id":      strconv.FormatInt(contractID, 10),
		"change_intent_id": strconv.FormatInt(changeIntentID, 10),
		"newapi_trade_no":  strings.TrimSpace(tradeNo),
		"newapi_user_id":   strconv.Itoa(userID),
		"newapi_plan_id":   strconv.Itoa(planID),
	}
}

func consoleSubscriptionReturnPath() string {
	return strings.TrimRight(system_setting.ServerAddress, "/") + "/console/subscription"
}

func ensureStripeSecretForSubscription() error {
	if !strings.HasPrefix(setting.StripeApiSecret, "sk_") && !strings.HasPrefix(setting.StripeApiSecret, "rk_") {
		return errors.New("invalid Stripe API key")
	}
	return nil
}

func ReconcilePaidInvoice(ctx context.Context, invoiceID string) (*PaidInvoiceReconcileResult, error) {
	invoiceID = strings.TrimSpace(invoiceID)
	if invoiceID == "" {
		return nil, PermanentPaidInvoiceError(errors.New("Stripe invoice id is required"))
	}
	inv, err := stripeInvoiceGetter(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	if inv == nil || strings.TrimSpace(inv.ID) == "" {
		return nil, errors.New("Stripe invoice is missing")
	}
	if !inv.Paid || inv.Status != stripe.InvoiceStatusPaid {
		return nil, errors.New("Stripe invoice is not paid")
	}
	subscriptionID := stripeInvoiceSubscriptionID(inv)
	if subscriptionID == "" {
		return nil, PermanentPaidInvoiceError(errors.New("Stripe invoice subscription id is missing"))
	}
	sub, err := stripeSubscriptionGetter(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	facts, err := validatePaidInvoiceFacts(inv, sub)
	if err != nil {
		return nil, err
	}
	result := &PaidInvoiceReconcileResult{}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		order, intent, contract, plan, user, err := lockInvoicePurchaseFactsTx(tx, facts)
		if err != nil {
			return err
		}
		if err := validateLocalInvoiceFacts(facts, order, intent, contract, plan, user); err != nil {
			return PermanentPaidInvoiceError(err)
		}
		binding, err := createOrLoadStripeInvoiceBindingTx(tx, order, contract.Id, providerSnapshotFromPaidInvoice(facts, invoiceID))
		if err != nil {
			return err
		}
		grant, err := model.RotateCurrentEntitlementTx(tx, model.GrantEntitlementInput{
			ContractId:           contract.Id,
			UserId:               order.UserId,
			PlanId:               order.PlanId,
			ProviderBindingId:    binding.Id,
			GrantKey:             "stripe:" + invoiceID,
			PaymentMode:          model.SubscriptionPaymentModeStripeRecurring,
			AmountTotal:          plan.TotalAmount,
			PeriodStart:          facts.PeriodStart,
			PeriodEnd:            facts.PeriodEnd,
			EndReasonForPrevious: previousEntitlementEndReason(intent.Kind),
			Source:               model.PaymentMethodStripe,
		})
		if err != nil {
			return err
		}
		order.Status = common.TopUpStatusSuccess
		order.CompleteTime = common.GetTimestamp()
		order.ProviderPayload = fmt.Sprintf("invoice_id=%s;subscription_id=%s;change_intent_id=%d", invoiceID, facts.SubscriptionID, intent.Id)
		if err := tx.Save(order).Error; err != nil {
			return err
		}
		intent.Status = model.SubscriptionChangeIntentStatusApplied
		intent.ProviderInvoiceId = invoiceID
		intent.ProviderBindingId = binding.Id
		intent.EffectiveAt = facts.PeriodStart
		if err := tx.Model(intent).Updates(map[string]interface{}{
			"status":              intent.Status,
			"provider_invoice_id": intent.ProviderInvoiceId,
			"provider_binding_id": intent.ProviderBindingId,
			"effective_at":        intent.EffectiveAt,
			"updated_at":          common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(contract).Where("id = ?", contract.Id).Updates(map[string]interface{}{
			"status":                      model.SubscriptionContractStatusActive,
			"payment_mode":                model.SubscriptionPaymentModeStripeRecurring,
			"current_provider_binding_id": binding.Id,
			"latest_change_intent_id":     intent.Id,
			"pending_plan_id":             0,
			"pending_effective_at":        0,
			"updated_at":                  common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		result.Binding = binding
		if grant != nil {
			result.Entitlement = grant.Entitlement
			result.Applied = grant.Applied
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

type paidInvoiceFacts struct {
	InvoiceID          string
	SubscriptionID     string
	SubscriptionItemID string
	CustomerID         string
	PriceID            string
	TradeNo            string
	UserID             int
	PlanID             int
	ContractID         int64
	ChangeIntentID     int64
	AmountPaid         int64
	Currency           string
	Livemode           bool
	ProviderStatus     string
	CancelAtPeriodEnd  bool
	PeriodStart        int64
	PeriodEnd          int64
}

func validatePaidInvoiceFacts(inv *stripe.Invoice, sub *stripe.Subscription) (paidInvoiceFacts, error) {
	if sub == nil || strings.TrimSpace(sub.ID) == "" {
		return paidInvoiceFacts{}, errors.New("Stripe subscription is missing")
	}
	if strings.TrimSpace(inv.ID) == "" || strings.TrimSpace(sub.ID) == "" {
		return paidInvoiceFacts{}, errors.New("Stripe invoice facts are incomplete")
	}
	if stripeInvoiceSubscriptionID(inv) != strings.TrimSpace(sub.ID) {
		return paidInvoiceFacts{}, PermanentPaidInvoiceError(errors.New("Stripe invoice subscription mismatch"))
	}
	invoiceCustomer := stripeCustomerID(inv.Customer)
	subscriptionCustomer := stripeCustomerID(sub.Customer)
	if invoiceCustomer == "" || subscriptionCustomer == "" {
		return paidInvoiceFacts{}, PermanentPaidInvoiceError(errors.New("Stripe invoice customer is missing"))
	}
	if invoiceCustomer != "" && subscriptionCustomer != "" && invoiceCustomer != subscriptionCustomer {
		return paidInvoiceFacts{}, PermanentPaidInvoiceError(errors.New("Stripe invoice customer mismatch"))
	}
	if inv.Livemode != sub.Livemode {
		return paidInvoiceFacts{}, PermanentPaidInvoiceError(errors.New("Stripe invoice subscription livemode mismatch"))
	}
	if err := validateStripeLivemodeForLocalKey(inv.Livemode); err != nil {
		return paidInvoiceFacts{}, PermanentPaidInvoiceError(err)
	}
	metadata := sub.Metadata
	tradeNo := strings.TrimSpace(metadata["trade_no"])
	if tradeNo == "" {
		tradeNo = strings.TrimSpace(metadata["newapi_trade_no"])
	}
	userID, err := strconv.Atoi(strings.TrimSpace(metadata["user_id"]))
	if err != nil || userID <= 0 {
		userID, err = strconv.Atoi(strings.TrimSpace(metadata["newapi_user_id"]))
	}
	if err != nil || userID <= 0 {
		return paidInvoiceFacts{}, PermanentPaidInvoiceError(errors.New("Stripe subscription metadata user_id is invalid"))
	}
	planID, err := strconv.Atoi(strings.TrimSpace(metadata["plan_id"]))
	if err != nil || planID <= 0 {
		planID, err = strconv.Atoi(strings.TrimSpace(metadata["newapi_plan_id"]))
	}
	if err != nil || planID <= 0 {
		return paidInvoiceFacts{}, PermanentPaidInvoiceError(errors.New("Stripe subscription metadata plan_id is invalid"))
	}
	contractID, err := strconv.ParseInt(strings.TrimSpace(metadata["contract_id"]), 10, 64)
	if err != nil || contractID <= 0 {
		return paidInvoiceFacts{}, PermanentPaidInvoiceError(errors.New("Stripe subscription metadata contract_id is invalid"))
	}
	intentID, err := strconv.ParseInt(strings.TrimSpace(metadata["change_intent_id"]), 10, 64)
	if err != nil || intentID <= 0 {
		return paidInvoiceFacts{}, PermanentPaidInvoiceError(errors.New("Stripe subscription metadata change_intent_id is invalid"))
	}
	if tradeNo == "" {
		return paidInvoiceFacts{}, PermanentPaidInvoiceError(errors.New("Stripe subscription metadata trade_no is missing"))
	}
	priceID := stripeSubscriptionFirstPriceID(sub)
	if priceID == "" {
		priceID = stripeInvoiceFirstPriceID(inv)
	}
	periodStart, periodEnd := stripeInvoicePeriod(inv, sub)
	return paidInvoiceFacts{
		InvoiceID:          strings.TrimSpace(inv.ID),
		SubscriptionID:     strings.TrimSpace(sub.ID),
		SubscriptionItemID: stripeSubscriptionFirstItemID(sub),
		CustomerID:         firstNonEmptyString(subscriptionCustomer, invoiceCustomer),
		PriceID:            priceID,
		TradeNo:            tradeNo,
		UserID:             userID,
		PlanID:             planID,
		ContractID:         contractID,
		ChangeIntentID:     intentID,
		AmountPaid:         inv.AmountPaid,
		Currency:           strings.ToUpper(string(inv.Currency)),
		Livemode:           inv.Livemode,
		ProviderStatus:     string(sub.Status),
		CancelAtPeriodEnd:  sub.CancelAtPeriodEnd,
		PeriodStart:        periodStart,
		PeriodEnd:          periodEnd,
	}, nil
}

func stripeInvoiceSubscriptionID(inv *stripe.Invoice) string {
	if inv == nil || inv.Subscription == nil {
		return ""
	}
	return strings.TrimSpace(inv.Subscription.ID)
}

func stripeCustomerID(customer *stripe.Customer) string {
	if customer == nil {
		return ""
	}
	return strings.TrimSpace(customer.ID)
}

func stripeSubscriptionFirstPriceID(sub *stripe.Subscription) string {
	if sub == nil || sub.Items == nil || len(sub.Items.Data) == 0 || sub.Items.Data[0] == nil || sub.Items.Data[0].Price == nil {
		return ""
	}
	return strings.TrimSpace(sub.Items.Data[0].Price.ID)
}

func stripeSubscriptionFirstItemID(sub *stripe.Subscription) string {
	if sub == nil || sub.Items == nil || len(sub.Items.Data) == 0 || sub.Items.Data[0] == nil {
		return ""
	}
	return strings.TrimSpace(sub.Items.Data[0].ID)
}

func stripeInvoiceFirstPriceID(inv *stripe.Invoice) string {
	if inv == nil || inv.Lines == nil {
		return ""
	}
	for _, line := range inv.Lines.Data {
		if line != nil && line.Price != nil && strings.TrimSpace(line.Price.ID) != "" {
			return strings.TrimSpace(line.Price.ID)
		}
	}
	return ""
}

func stripeInvoicePeriod(inv *stripe.Invoice, sub *stripe.Subscription) (int64, int64) {
	if inv != nil && inv.Lines != nil {
		for _, line := range inv.Lines.Data {
			if line != nil && line.Period != nil && line.Period.Start > 0 && line.Period.End > line.Period.Start {
				return line.Period.Start, line.Period.End
			}
		}
	}
	if sub != nil && sub.CurrentPeriodStart > 0 && sub.CurrentPeriodEnd > sub.CurrentPeriodStart {
		return sub.CurrentPeriodStart, sub.CurrentPeriodEnd
	}
	now := common.GetTimestamp()
	return now, now + int64((30 * 24 * time.Hour).Seconds())
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func validateStripeLivemodeForLocalKey(livemode bool) error {
	secret := strings.TrimSpace(setting.StripeApiSecret)
	switch {
	case strings.HasPrefix(secret, "sk_live_"), strings.HasPrefix(secret, "rk_live_"):
		if !livemode {
			return errors.New("Stripe invoice livemode mismatch: live key received test invoice")
		}
	case strings.HasPrefix(secret, "sk_test_"), strings.HasPrefix(secret, "rk_test_"):
		if livemode {
			return errors.New("Stripe invoice livemode mismatch: test key received live invoice")
		}
	}
	return nil
}

func lockInvoicePurchaseFactsTx(tx *gorm.DB, facts paidInvoiceFacts) (*model.SubscriptionOrder, *model.SubscriptionChangeIntent, *model.UserSubscriptionContract, *model.SubscriptionPlan, *model.User, error) {
	var order model.SubscriptionOrder
	if err := subscriptionCommandLock(tx).Where("trade_no = ?", facts.TradeNo).First(&order).Error; err != nil {
		return nil, nil, nil, nil, nil, model.ErrSubscriptionOrderNotFound
	}
	var intent model.SubscriptionChangeIntent
	if err := subscriptionCommandLock(tx).Where("id = ? AND user_id = ? AND contract_id = ?", facts.ChangeIntentID, facts.UserID, facts.ContractID).First(&intent).Error; err != nil {
		return nil, nil, nil, nil, nil, err
	}
	var contract model.UserSubscriptionContract
	if err := subscriptionCommandLock(tx).Where("id = ? AND user_id = ?", facts.ContractID, facts.UserID).First(&contract).Error; err != nil {
		return nil, nil, nil, nil, nil, err
	}
	var plan model.SubscriptionPlan
	if err := tx.Where("id = ?", facts.PlanID).First(&plan).Error; err != nil {
		return nil, nil, nil, nil, nil, err
	}
	plan.NormalizeDefaults()
	var user model.User
	if err := subscriptionCommandLock(tx).Where("id = ?", facts.UserID).First(&user).Error; err != nil {
		return nil, nil, nil, nil, nil, err
	}
	return &order, &intent, &contract, &plan, &user, nil
}

func validateLocalInvoiceFacts(facts paidInvoiceFacts, order *model.SubscriptionOrder, intent *model.SubscriptionChangeIntent, contract *model.UserSubscriptionContract, plan *model.SubscriptionPlan, user *model.User) error {
	if order.UserId != facts.UserID || order.PlanId != facts.PlanID || order.ChangeIntentId != 0 && order.ChangeIntentId != facts.ChangeIntentID {
		return errors.New("local order ownership mismatch")
	}
	if order.PaymentProvider != model.PaymentProviderStripe {
		return errors.New("local order payment provider mismatch")
	}
	if order.Status != common.TopUpStatusPending && order.Status != common.TopUpStatusSuccess {
		return model.ErrSubscriptionOrderStatusInvalid
	}
	if intent.UserId != facts.UserID || intent.ToPlanId != facts.PlanID || intent.PaymentMode != model.SubscriptionPaymentModeStripeRecurring {
		return errors.New("local change intent ownership mismatch")
	}
	if intent.Status != model.SubscriptionChangeIntentStatusAwaitingPayment && intent.Status != model.SubscriptionChangeIntentStatusApplied {
		return errors.New("local change intent status mismatch")
	}
	if contract.UserId != facts.UserID {
		return errors.New("local contract ownership mismatch")
	}
	if strings.TrimSpace(user.StripeCustomer) != "" && strings.TrimSpace(user.StripeCustomer) != facts.CustomerID {
		return errors.New("local Stripe customer mismatch")
	}
	if plan.Id != facts.PlanID || !plan.Enabled {
		return errors.New("local plan is not enabled")
	}
	if strings.TrimSpace(plan.StripePriceId) == "" || strings.TrimSpace(plan.StripePriceId) != facts.PriceID {
		return errors.New("Stripe price mismatch")
	}
	if strings.ToUpper(strings.TrimSpace(plan.Currency)) != facts.Currency {
		return errors.New("Stripe invoice currency mismatch")
	}
	expectedMinor, err := stripeMinorUnitAmountForSubscription(plan.PriceAmount, facts.Currency)
	if err != nil {
		return err
	}
	if expectedMinor != facts.AmountPaid {
		return fmt.Errorf("Stripe invoice amount mismatch: expected %d got %d", expectedMinor, facts.AmountPaid)
	}
	return nil
}

func providerSnapshotFromPaidInvoice(facts paidInvoiceFacts, invoiceID string) model.ProviderSubscriptionSnapshot {
	return model.ProviderSubscriptionSnapshot{
		ProviderSubscriptionId:     facts.SubscriptionID,
		ProviderSubscriptionItemId: facts.SubscriptionItemID,
		ProviderCustomerId:         facts.CustomerID,
		ProviderPriceId:            facts.PriceID,
		ProviderLatestInvoiceId:    invoiceID,
		ProviderStatus:             facts.ProviderStatus,
		CancelAtPeriodEnd:          facts.CancelAtPeriodEnd,
		CurrentPeriodStart:         facts.PeriodStart,
		CurrentPeriodEnd:           facts.PeriodEnd,
		Livemode:                   facts.Livemode,
	}
}

func createOrLoadStripeInvoiceBindingTx(tx *gorm.DB, order *model.SubscriptionOrder, contractID int64, snapshot model.ProviderSubscriptionSnapshot) (*model.SubscriptionProviderBinding, error) {
	if strings.TrimSpace(snapshot.ProviderSubscriptionId) == "" {
		return nil, errors.New("provider subscription id is empty")
	}
	var binding model.SubscriptionProviderBinding
	err := tx.Where("provider = ? AND provider_subscription_id = ?", model.PaymentProviderStripe, snapshot.ProviderSubscriptionId).First(&binding).Error
	if err == nil {
		if binding.UserId != order.UserId || binding.ContractId != contractID {
			return nil, model.ErrSubscriptionProviderBindingConflict
		}
		updates := map[string]interface{}{
			"provider_subscription_item_id": strings.TrimSpace(snapshot.ProviderSubscriptionItemId),
			"provider_customer_id":          strings.TrimSpace(snapshot.ProviderCustomerId),
			"provider_price_id":             strings.TrimSpace(snapshot.ProviderPriceId),
			"provider_latest_invoice_id":    strings.TrimSpace(snapshot.ProviderLatestInvoiceId),
			"provider_status":               strings.TrimSpace(snapshot.ProviderStatus),
			"cancel_at_period_end":          snapshot.CancelAtPeriodEnd,
			"current_period_start":          snapshot.CurrentPeriodStart,
			"current_period_end":            snapshot.CurrentPeriodEnd,
			"livemode":                      snapshot.Livemode,
			"last_synced_at":                common.GetTimestamp(),
			"updated_at":                    common.GetTimestamp(),
		}
		if err := tx.Model(&binding).Updates(updates).Error; err != nil {
			return nil, err
		}
		if err := tx.First(&binding, "id = ?", binding.Id).Error; err != nil {
			return nil, err
		}
		return &binding, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	binding = model.SubscriptionProviderBinding{
		UserId:                     order.UserId,
		PlanId:                     order.PlanId,
		InitialOrderId:             order.Id,
		ContractId:                 contractID,
		Provider:                   model.PaymentProviderStripe,
		ProviderSubscriptionId:     strings.TrimSpace(snapshot.ProviderSubscriptionId),
		ProviderSubscriptionItemId: strings.TrimSpace(snapshot.ProviderSubscriptionItemId),
		ProviderCustomerId:         strings.TrimSpace(snapshot.ProviderCustomerId),
		ProviderPriceId:            strings.TrimSpace(snapshot.ProviderPriceId),
		ProviderLatestInvoiceId:    strings.TrimSpace(snapshot.ProviderLatestInvoiceId),
		ProviderStatus:             strings.TrimSpace(snapshot.ProviderStatus),
		CancelAtPeriodEnd:          snapshot.CancelAtPeriodEnd,
		CurrentPeriodStart:         snapshot.CurrentPeriodStart,
		CurrentPeriodEnd:           snapshot.CurrentPeriodEnd,
		Livemode:                   snapshot.Livemode,
		LastSyncedAt:               common.GetTimestamp(),
	}
	if err := tx.Create(&binding).Error; err != nil {
		var existing model.SubscriptionProviderBinding
		if findErr := tx.Where("provider = ? AND provider_subscription_id = ?", model.PaymentProviderStripe, snapshot.ProviderSubscriptionId).First(&existing).Error; findErr == nil {
			if existing.UserId == order.UserId && existing.ContractId == contractID {
				return &existing, nil
			}
			return nil, model.ErrSubscriptionProviderBindingConflict
		}
		return nil, err
	}
	return &binding, nil
}

func TerminatePendingStripePurchase(ctx context.Context, tradeNo string, intentStatus string) error {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return nil
	}
	switch intentStatus {
	case model.SubscriptionChangeIntentStatusExpired, model.SubscriptionChangeIntentStatusFailed:
	default:
		intentStatus = model.SubscriptionChangeIntentStatusFailed
	}
	orderStatus := common.TopUpStatusFailed
	if intentStatus == model.SubscriptionChangeIntentStatusExpired {
		orderStatus = common.TopUpStatusExpired
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		var order model.SubscriptionOrder
		if err := subscriptionCommandLock(tx).Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if order.PaymentProvider != model.PaymentProviderStripe {
			return nil
		}
		if order.Status == common.TopUpStatusPending {
			if err := tx.Model(&order).Updates(map[string]interface{}{
				"status":        orderStatus,
				"complete_time": common.GetTimestamp(),
			}).Error; err != nil {
				return err
			}
		}
		intentID := order.ChangeIntentId
		if intentID <= 0 {
			intentID = parseChangeIntentIDFromPayload(order.ProviderPayload)
		}
		if intentID <= 0 {
			return nil
		}
		var intent model.SubscriptionChangeIntent
		if err := subscriptionCommandLock(tx).Where("id = ?", intentID).First(&intent).Error; err != nil {
			return err
		}
		if intent.Status == model.SubscriptionChangeIntentStatusAwaitingPayment || intent.Status == model.SubscriptionChangeIntentStatusCreated || intent.Status == model.SubscriptionChangeIntentStatusSyncing {
			if err := tx.Model(&intent).Updates(map[string]interface{}{
				"status":     intentStatus,
				"last_error": "Stripe checkout ended before first invoice was reconciled",
				"updated_at": common.GetTimestamp(),
			}).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.UserSubscriptionContract{}).
			Where("id = ? AND latest_change_intent_id = ?", intent.ContractId, intent.Id).
			Updates(map[string]interface{}{
				"latest_change_intent_id": 0,
				"updated_at":              common.GetTimestamp(),
			}).Error
	})
}

func parseChangeIntentIDFromPayload(payload string) int64 {
	for _, part := range strings.Split(payload, ";") {
		key, value, ok := strings.Cut(part, "=")
		if !ok || strings.TrimSpace(key) != "change_intent_id" {
			continue
		}
		id, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		return id
	}
	return 0
}

func stripeMinorUnitAmountForSubscription(amount float64, currency string) (int64, error) {
	if amount <= 0 {
		return 0, nil
	}
	scale := int32(2)
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "BIF", "CLP", "DJF", "GNF", "JPY", "KMF", "KRW", "MGA", "PYG", "RWF", "UGX", "VND", "VUV", "XAF", "XOF", "XPF":
		scale = 0
	}
	return decimal.NewFromFloat(amount).Shift(scale).Round(0).IntPart(), nil
}
