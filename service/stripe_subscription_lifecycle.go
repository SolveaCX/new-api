package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stripe/stripe-go/v81"
	stripesubscription "github.com/stripe/stripe-go/v81/subscription"
)

var stripeUpdateSubscriptionCancelAtPeriodEnd = updateStripeSubscriptionCancelAtPeriodEnd
var stripeCancelSubscriptionNow = cancelStripeSubscriptionNow

func CancelStripeRecurringSubscription(userID int, bindingID int64) (*model.SubscriptionProviderBinding, error) {
	binding, err := recurringBindingForUser(userID, bindingID)
	if err != nil {
		return nil, err
	}
	idempotencyKey := recurringLifecycleIdempotencyKey(binding, "cancel")
	if strings.EqualFold(binding.ProviderStatus, "past_due") {
		snapshot, err := stripeCancelSubscriptionNow(binding.ProviderSubscriptionId, idempotencyKey)
		if err != nil {
			return nil, err
		}
		return model.ApplyProviderSubscriptionTermination(binding.Id, snapshot)
	}
	if binding.CancelAtPeriodEnd {
		return binding, nil
	}
	snapshot, err := stripeUpdateSubscriptionCancelAtPeriodEnd(binding.ProviderSubscriptionId, true, idempotencyKey)
	if err != nil {
		return nil, err
	}
	return model.ApplyProviderSubscriptionSnapshot(binding.Id, snapshot)
}

func ResumeStripeRecurringSubscription(userID int, bindingID int64) (*model.SubscriptionProviderBinding, error) {
	binding, err := recurringBindingForUser(userID, bindingID)
	if err != nil {
		return nil, err
	}
	if isTerminalStripeSubscriptionStatus(binding.ProviderStatus) || binding.EndedAt > 0 {
		return nil, errors.New("terminal Stripe subscription cannot be resumed")
	}
	if !binding.CancelAtPeriodEnd {
		return binding, nil
	}
	snapshot, err := stripeUpdateSubscriptionCancelAtPeriodEnd(binding.ProviderSubscriptionId, false, recurringLifecycleIdempotencyKey(binding, "resume"))
	if err != nil {
		return nil, err
	}
	return model.ApplyProviderSubscriptionSnapshot(binding.Id, snapshot)
}

func AdminInvalidateUserSubscriptionWithRecurringPolicy(userSubscriptionID int) (string, error) {
	sub, binding, managed, err := adminRecurringPolicyTarget(userSubscriptionID)
	if err != nil {
		return "", err
	}
	if !managed {
		return model.AdminInvalidateUserSubscription(userSubscriptionID)
	}
	snapshot, err := stripeCancelSubscriptionNow(binding.ProviderSubscriptionId, recurringLifecycleIdempotencyKey(binding, "admin_invalidate"))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(snapshot.ProviderSubscriptionId) == "" {
		snapshot.ProviderSubscriptionId = binding.ProviderSubscriptionId
	}
	if _, err := model.ApplyProviderSubscriptionTermination(binding.Id, snapshot); err != nil {
		return "", err
	}
	return model.AdminInvalidateUserSubscription(sub.Id)
}

func AdminDeleteUserSubscriptionWithRecurringPolicy(userSubscriptionID int) (string, error) {
	_, _, managed, err := adminRecurringPolicyTarget(userSubscriptionID)
	if err != nil {
		return "", err
	}
	if managed {
		return "", errors.New("Stripe recurring subscription history cannot be deleted")
	}
	return model.AdminDeleteUserSubscription(userSubscriptionID)
}

func recurringBindingForUser(userID int, bindingID int64) (*model.SubscriptionProviderBinding, error) {
	binding, err := model.FindBindingByIDForUser(bindingID, userID)
	if err != nil {
		return nil, err
	}
	if binding.Provider != model.PaymentProviderStripe {
		return nil, errors.New("recurring subscription is not managed by Stripe")
	}
	if strings.TrimSpace(binding.ProviderSubscriptionId) == "" {
		return nil, errors.New("Stripe subscription binding is incomplete")
	}
	return binding, nil
}

func adminRecurringPolicyTarget(userSubscriptionID int) (*model.UserSubscription, *model.SubscriptionProviderBinding, bool, error) {
	if userSubscriptionID <= 0 {
		return nil, nil, false, errors.New("invalid userSubscriptionId")
	}
	var sub model.UserSubscription
	if err := model.DB.Where("id = ?", userSubscriptionID).First(&sub).Error; err != nil {
		return nil, nil, false, err
	}
	if sub.ProviderBindingId <= 0 {
		return &sub, nil, false, nil
	}
	var binding model.SubscriptionProviderBinding
	if err := model.DB.Where("id = ?", sub.ProviderBindingId).First(&binding).Error; err != nil {
		return &sub, nil, false, err
	}
	managed := binding.Provider == model.PaymentProviderStripe && strings.TrimSpace(binding.ProviderSubscriptionId) != ""
	return &sub, &binding, managed, nil
}

func recurringLifecycleIdempotencyKey(binding *model.SubscriptionProviderBinding, action string) string {
	currentPeriodEnd := int64(0)
	bindingID := int64(0)
	actionSeq := int64(0)
	if binding != nil {
		currentPeriodEnd = binding.CurrentPeriodEnd
		bindingID = binding.Id
		actionSeq = binding.LifecycleActionSeq
	}
	return fmt.Sprintf("newapi_subscription_binding_%d_%s_%d_%d", bindingID, action, currentPeriodEnd, actionSeq)
}

func isTerminalStripeSubscriptionStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "canceled", "incomplete_expired", "unpaid":
		return true
	default:
		return false
	}
}

func updateStripeSubscriptionCancelAtPeriodEnd(providerSubscriptionID string, cancelAtPeriodEnd bool, idempotencyKey string) (model.ProviderSubscriptionSnapshot, error) {
	if err := ensureStripeLifecycleKey(); err != nil {
		return model.ProviderSubscriptionSnapshot{}, err
	}
	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(cancelAtPeriodEnd),
	}
	params.SetIdempotencyKey(idempotencyKey)
	params.AddExpand("latest_invoice")
	params.AddExpand("items.data.price")
	sub, err := stripesubscription.Update(strings.TrimSpace(providerSubscriptionID), params)
	if err != nil {
		return model.ProviderSubscriptionSnapshot{}, err
	}
	return providerSubscriptionSnapshotFromStripe(sub), nil
}

func cancelStripeSubscriptionNow(providerSubscriptionID string, idempotencyKey string) (model.ProviderSubscriptionSnapshot, error) {
	if err := ensureStripeLifecycleKey(); err != nil {
		return model.ProviderSubscriptionSnapshot{}, err
	}
	params := &stripe.SubscriptionCancelParams{}
	params.SetIdempotencyKey(idempotencyKey)
	sub, err := stripesubscription.Cancel(strings.TrimSpace(providerSubscriptionID), params)
	if err != nil {
		return model.ProviderSubscriptionSnapshot{}, err
	}
	return providerSubscriptionSnapshotFromStripe(sub), nil
}

func ensureStripeLifecycleKey() error {
	if !strings.HasPrefix(setting.StripeApiSecret, "sk_") && !strings.HasPrefix(setting.StripeApiSecret, "rk_") {
		return errors.New("invalid Stripe API key")
	}
	stripe.Key = setting.StripeApiSecret
	return nil
}

func providerSubscriptionSnapshotFromStripe(sub *stripe.Subscription) model.ProviderSubscriptionSnapshot {
	if sub == nil {
		return model.ProviderSubscriptionSnapshot{}
	}
	customerID := ""
	if sub.Customer != nil {
		customerID = strings.TrimSpace(sub.Customer.ID)
	}
	priceID := ""
	if sub.Items != nil && len(sub.Items.Data) > 0 && sub.Items.Data[0] != nil && sub.Items.Data[0].Price != nil {
		priceID = strings.TrimSpace(sub.Items.Data[0].Price.ID)
	}
	itemID := ""
	if sub.Items != nil && len(sub.Items.Data) > 0 && sub.Items.Data[0] != nil {
		itemID = strings.TrimSpace(sub.Items.Data[0].ID)
	}
	scheduleID := ""
	if sub.Schedule != nil {
		scheduleID = strings.TrimSpace(sub.Schedule.ID)
	}
	latestInvoiceID := ""
	if sub.LatestInvoice != nil {
		latestInvoiceID = strings.TrimSpace(sub.LatestInvoice.ID)
	}
	return model.ProviderSubscriptionSnapshot{
		ProviderSubscriptionId:     strings.TrimSpace(sub.ID),
		ProviderSubscriptionItemId: itemID,
		ProviderScheduleId:         scheduleID,
		ProviderScheduleIdObserved: true,
		ProviderCustomerId:         customerID,
		ProviderPriceId:            priceID,
		ProviderLatestInvoiceId:    latestInvoiceID,
		ProviderStatus:             string(sub.Status),
		CancelAtPeriodEnd:          sub.CancelAtPeriodEnd,
		CurrentPeriodStart:         sub.CurrentPeriodStart,
		CurrentPeriodEnd:           sub.CurrentPeriodEnd,
		CanceledAt:                 sub.CanceledAt,
		EndedAt:                    sub.EndedAt,
		Livemode:                   sub.Livemode,
	}
}
