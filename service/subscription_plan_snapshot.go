package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stripe/stripe-go/v86"
)

const RecurringPlanSnapshotVersionV1 = 1

// RecurringPlanSnapshotV1 is the immutable billing and entitlement contract
// accepted for recurring renewals. Money is stored only in Stripe minor units.
type RecurringPlanSnapshotV1 struct {
	Version                 int    `json:"version"`
	PlanID                  int    `json:"plan_id"`
	StripePriceID           string `json:"stripe_price_id"`
	Currency                string `json:"currency"`
	BasePriceMinor          int64  `json:"base_price_minor"`
	DurationUnit            string `json:"duration_unit"`
	DurationValue           int    `json:"duration_value"`
	DurationCustomSeconds   int64  `json:"duration_custom_seconds"`
	QuotaResetPeriod        string `json:"quota_reset_period"`
	QuotaResetCustomSeconds int64  `json:"quota_reset_custom_seconds"`
	TotalAmount             int64  `json:"total_amount"`
	MediaCreditsMonthly     int64  `json:"media_credits_monthly"`
	Window5hAmount          int64  `json:"window_5h_amount"`
	WindowWeekAmount        int64  `json:"window_week_amount"`
	UpgradeGroup            string `json:"upgrade_group"`
}

func EncodeRecurringPlanSnapshotV1(snapshot RecurringPlanSnapshotV1) (string, error) {
	snapshot = normalizeRecurringPlanSnapshotV1(snapshot)
	if err := ValidateRecurringPlanSnapshotV1(snapshot); err != nil {
		return "", err
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func DecodeRecurringPlanSnapshotV1(raw string) (RecurringPlanSnapshotV1, error) {
	if strings.TrimSpace(raw) == "" {
		return RecurringPlanSnapshotV1{}, errors.New("recurring plan snapshot is missing")
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	var snapshot RecurringPlanSnapshotV1
	if err := decoder.Decode(&snapshot); err != nil {
		return RecurringPlanSnapshotV1{}, fmt.Errorf("decode recurring plan snapshot: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return RecurringPlanSnapshotV1{}, err
	}
	snapshot = normalizeRecurringPlanSnapshotV1(snapshot)
	if err := ValidateRecurringPlanSnapshotV1(snapshot); err != nil {
		return RecurringPlanSnapshotV1{}, err
	}
	return snapshot, nil
}

func RecurringPlanSnapshotV1Fingerprint(snapshot RecurringPlanSnapshotV1) (string, error) {
	encoded, err := EncodeRecurringPlanSnapshotV1(snapshot)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(encoded))
	return hex.EncodeToString(digest[:]), nil
}

func ValidateRecurringPlanSnapshotV1(snapshot RecurringPlanSnapshotV1) error {
	if snapshot.Version != RecurringPlanSnapshotVersionV1 {
		return fmt.Errorf("unsupported recurring plan snapshot version: %d", snapshot.Version)
	}
	if snapshot.PlanID <= 0 {
		return errors.New("recurring plan snapshot plan_id is invalid")
	}
	if strings.TrimSpace(snapshot.StripePriceID) == "" {
		return errors.New("recurring plan snapshot stripe_price_id is missing")
	}
	if snapshot.Currency == "" || snapshot.Currency != strings.ToUpper(snapshot.Currency) || len(snapshot.Currency) != 3 {
		return errors.New("recurring plan snapshot currency is invalid")
	}
	if snapshot.BasePriceMinor <= 0 {
		return errors.New("recurring plan snapshot base price is invalid")
	}
	if snapshot.DurationUnit != model.SubscriptionDurationMonth || snapshot.DurationValue != 1 || snapshot.DurationCustomSeconds != 0 {
		return errors.New("recurring plan snapshot must describe one monthly period")
	}
	if err := validateRecurringPlanSnapshotReset(snapshot); err != nil {
		return err
	}
	if snapshot.TotalAmount < 0 || snapshot.MediaCreditsMonthly < 0 || snapshot.Window5hAmount < 0 || snapshot.WindowWeekAmount < 0 {
		return errors.New("recurring plan snapshot entitlement values are invalid")
	}
	return nil
}

func FreezeRecurringPlanSnapshotV1(plan *model.SubscriptionPlan, price *stripe.Price) (RecurringPlanSnapshotV1, error) {
	if plan == nil {
		return RecurringPlanSnapshotV1{}, errors.New("subscription plan is missing")
	}
	minor, err := stripeMinorUnitAmountForSubscription(plan.PriceAmount, plan.Currency)
	if err != nil {
		return RecurringPlanSnapshotV1{}, err
	}
	snapshot := normalizeRecurringPlanSnapshotV1(RecurringPlanSnapshotV1{
		Version:                 RecurringPlanSnapshotVersionV1,
		PlanID:                  plan.Id,
		StripePriceID:           plan.StripePriceId,
		Currency:                plan.Currency,
		BasePriceMinor:          minor,
		DurationUnit:            plan.DurationUnit,
		DurationValue:           plan.DurationValue,
		DurationCustomSeconds:   plan.CustomSeconds,
		QuotaResetPeriod:        plan.QuotaResetPeriod,
		QuotaResetCustomSeconds: plan.QuotaResetCustomSeconds,
		TotalAmount:             plan.TotalAmount,
		MediaCreditsMonthly:     plan.MediaCreditsMonthly,
		Window5hAmount:          plan.Window5hAmount,
		WindowWeekAmount:        plan.WindowWeekAmount,
		UpgradeGroup:            plan.UpgradeGroup,
	})
	if err := ValidateRecurringPlanSnapshotV1AgainstPlan(snapshot, plan); err != nil {
		return RecurringPlanSnapshotV1{}, err
	}
	if err := ValidateRecurringPlanSnapshotV1AgainstStripePrice(snapshot, price); err != nil {
		return RecurringPlanSnapshotV1{}, err
	}
	return snapshot, nil
}

func ValidateRecurringPlanSnapshotV1AgainstPlan(snapshot RecurringPlanSnapshotV1, plan *model.SubscriptionPlan) error {
	if err := ValidateRecurringPlanSnapshotV1(snapshot); err != nil {
		return err
	}
	if plan == nil {
		return errors.New("subscription plan is missing")
	}
	expectedMinor, err := stripeMinorUnitAmountForSubscription(plan.PriceAmount, plan.Currency)
	if err != nil {
		return err
	}
	expected := normalizeRecurringPlanSnapshotV1(RecurringPlanSnapshotV1{
		Version:                 RecurringPlanSnapshotVersionV1,
		PlanID:                  plan.Id,
		StripePriceID:           plan.StripePriceId,
		Currency:                plan.Currency,
		BasePriceMinor:          expectedMinor,
		DurationUnit:            plan.DurationUnit,
		DurationValue:           plan.DurationValue,
		DurationCustomSeconds:   plan.CustomSeconds,
		QuotaResetPeriod:        plan.QuotaResetPeriod,
		QuotaResetCustomSeconds: plan.QuotaResetCustomSeconds,
		TotalAmount:             plan.TotalAmount,
		MediaCreditsMonthly:     plan.MediaCreditsMonthly,
		Window5hAmount:          plan.Window5hAmount,
		WindowWeekAmount:        plan.WindowWeekAmount,
		UpgradeGroup:            plan.UpgradeGroup,
	})
	if snapshot != expected {
		return errors.New("recurring plan snapshot does not match subscription plan")
	}
	return nil
}

func ValidateRecurringPlanSnapshotV1AgainstStripePrice(snapshot RecurringPlanSnapshotV1, price *stripe.Price) error {
	if err := ValidateRecurringPlanSnapshotV1(snapshot); err != nil {
		return err
	}
	if price == nil || strings.TrimSpace(price.ID) == "" {
		return errors.New("Stripe Price is missing")
	}
	if price.Livemode {
		return errors.New("Stripe Price must be test mode")
	}
	if !price.Active || price.Deleted {
		return errors.New("Stripe Price is inactive")
	}
	if strings.TrimSpace(price.ID) != snapshot.StripePriceID {
		return errors.New("Stripe Price id does not match recurring plan snapshot")
	}
	if strings.ToUpper(strings.TrimSpace(string(price.Currency))) != snapshot.Currency {
		return errors.New("Stripe Price currency does not match recurring plan snapshot")
	}
	if price.UnitAmount != snapshot.BasePriceMinor {
		return errors.New("Stripe Price amount does not match recurring plan snapshot")
	}
	if price.Type != stripe.PriceTypeRecurring || price.Recurring == nil ||
		price.Recurring.Interval != stripe.PriceRecurringIntervalMonth || price.Recurring.IntervalCount != 1 {
		return errors.New("Stripe Price must recur monthly")
	}
	return nil
}

// FreezeLegacyRecurringPlanSnapshotV1 accepts an old order as a temporary
// source only when every local and Stripe ownership/money fact still agrees.
func FreezeLegacyRecurringPlanSnapshotV1(order *model.SubscriptionOrder, binding *model.SubscriptionProviderBinding, plan *model.SubscriptionPlan, price *stripe.Price) (RecurringPlanSnapshotV1, error) {
	if order == nil || binding == nil || plan == nil {
		return RecurringPlanSnapshotV1{}, errors.New("legacy recurring source facts are incomplete")
	}
	if order.Id <= 0 || order.Id != binding.InitialOrderId || order.UserId != binding.UserId || order.PlanId != binding.PlanId || order.PlanId != plan.Id {
		return RecurringPlanSnapshotV1{}, errors.New("legacy recurring source ownership mismatch")
	}
	if order.Status != common.TopUpStatusSuccess || order.CompleteTime <= 0 || strings.TrimSpace(order.PaymentProvider) != model.PaymentProviderStripe || strings.TrimSpace(order.PaymentMethod) != model.PaymentMethodStripe {
		return RecurringPlanSnapshotV1{}, errors.New("legacy recurring source order is not a completed Stripe purchase")
	}
	switch strings.TrimSpace(order.PurchaseIntent) {
	case model.SubscriptionChangeIntentKindPurchase, model.SubscriptionChangeIntentKindRepurchase, model.SubscriptionChangeIntentKindUpgrade:
	default:
		return RecurringPlanSnapshotV1{}, errors.New("legacy recurring source purchase intent is invalid")
	}
	if renewalSource := strings.TrimSpace(order.RenewalSource); renewalSource != "" && renewalSource != model.SubscriptionRenewalSourceProvider {
		return RecurringPlanSnapshotV1{}, errors.New("legacy recurring source renewal semantics are invalid")
	}
	if err := validateLegacyRecurringProviderPayload(order.ProviderPayload, binding); err != nil {
		return RecurringPlanSnapshotV1{}, err
	}
	var legacy purchasePlanSnapshot
	if strings.TrimSpace(order.PlanSnapshot) == "" || json.Unmarshal([]byte(order.PlanSnapshot), &legacy) != nil {
		return RecurringPlanSnapshotV1{}, errors.New("legacy recurring order plan snapshot is invalid")
	}
	legacyCurrency := strings.ToUpper(strings.TrimSpace(legacy.Currency))
	orderCurrency := strings.ToUpper(strings.TrimSpace(order.PaymentCurrency))
	if legacy.PlanID != order.PlanId || legacyCurrency == "" || legacyCurrency != orderCurrency || strings.TrimSpace(legacy.StripePriceID) == "" ||
		strings.TrimSpace(legacy.StripePriceID) != strings.TrimSpace(binding.ProviderPriceId) || strings.TrimSpace(legacy.StripePriceID) != strings.TrimSpace(plan.StripePriceId) {
		return RecurringPlanSnapshotV1{}, errors.New("legacy recurring order snapshot identity mismatch")
	}
	legacyMinor, err := stripeMinorUnitAmountForSubscription(legacy.PriceAmount, legacyCurrency)
	if err != nil {
		return RecurringPlanSnapshotV1{}, err
	}
	orderMinor, err := stripeMinorUnitAmountForSubscription(order.UnitPrice, orderCurrency)
	if err != nil {
		return RecurringPlanSnapshotV1{}, err
	}
	if legacyMinor != orderMinor {
		return RecurringPlanSnapshotV1{}, errors.New("legacy recurring order amount mismatch")
	}
	snapshot := normalizeRecurringPlanSnapshotV1(RecurringPlanSnapshotV1{
		Version:             RecurringPlanSnapshotVersionV1,
		PlanID:              legacy.PlanID,
		StripePriceID:       legacy.StripePriceID,
		Currency:            legacyCurrency,
		BasePriceMinor:      legacyMinor,
		DurationUnit:        legacy.DurationUnit,
		DurationValue:       legacy.DurationValue,
		QuotaResetPeriod:    legacy.QuotaResetPeriod,
		TotalAmount:         legacy.TotalAmount,
		MediaCreditsMonthly: legacy.MediaCreditsMonthly,
		Window5hAmount:      legacy.Window5hAmount,
		WindowWeekAmount:    legacy.WindowWeekAmount,
		UpgradeGroup:        legacy.UpgradeGroup,
	})
	if err := ValidateRecurringPlanSnapshotV1AgainstPlan(snapshot, plan); err != nil {
		return RecurringPlanSnapshotV1{}, err
	}
	if err := ValidateRecurringPlanSnapshotV1AgainstStripePrice(snapshot, price); err != nil {
		return RecurringPlanSnapshotV1{}, err
	}
	return snapshot, nil
}

type CatalogMigrationSandboxConfig struct {
	DeploymentEnvironment string
	ServiceName           string
	FeatureEnabled        bool
	AllowedContractIDs    []int64
	StripeSecret          string
	StripePublishableKey  string
}

type CatalogMigrationStripeSandboxFacts struct {
	ContractID             int64
	BindingLivemode        bool
	SubscriptionLivemode   bool
	CurrentPriceLivemode   bool
	TargetPriceLivemode    bool
	BindingSubscriptionID  string
	SubscriptionID         string
	BindingCustomerID      string
	SubscriptionCustomerID string
	BindingCurrentPriceID  string
	SubscriptionPriceID    string
	ExpectedTargetPriceID  string
	TargetPriceID          string
}

func ValidateCatalogMigrationCommonSandbox(config CatalogMigrationSandboxConfig, contractID int64) error {
	if strings.TrimSpace(config.DeploymentEnvironment) != "staging" {
		return errors.New("catalog migration requires staging deployment environment")
	}
	if strings.TrimSpace(config.ServiceName) != "newapi-staging" {
		return errors.New("catalog migration requires newapi-staging service")
	}
	if !config.FeatureEnabled {
		return errors.New("catalog migration feature is disabled")
	}
	if contractID <= 0 || !containsContractID(config.AllowedContractIDs, contractID) {
		return errors.New("catalog migration contract is not allowlisted")
	}
	return nil
}

func ValidateCatalogMigrationStripeSandbox(config CatalogMigrationSandboxConfig, facts CatalogMigrationStripeSandboxFacts) error {
	if err := ValidateCatalogMigrationCommonSandbox(config, facts.ContractID); err != nil {
		return err
	}
	secret := strings.TrimSpace(config.StripeSecret)
	publishable := strings.TrimSpace(config.StripePublishableKey)
	if (!strings.HasPrefix(secret, "sk_test_") && !strings.HasPrefix(secret, "rk_test_")) || !strings.HasPrefix(publishable, "pk_test_") {
		return errors.New("catalog migration requires Stripe test credentials")
	}
	if facts.BindingLivemode || facts.SubscriptionLivemode || facts.CurrentPriceLivemode || facts.TargetPriceLivemode {
		return errors.New("catalog migration rejects Stripe live-mode facts")
	}
	if !sameRequiredIdentifier(facts.BindingSubscriptionID, facts.SubscriptionID) {
		return errors.New("catalog migration Stripe subscription mismatch")
	}
	if !sameRequiredIdentifier(facts.BindingCustomerID, facts.SubscriptionCustomerID) {
		return errors.New("catalog migration Stripe customer mismatch")
	}
	if !sameRequiredIdentifier(facts.BindingCurrentPriceID, facts.SubscriptionPriceID) {
		return errors.New("catalog migration current Stripe Price mismatch")
	}
	if !sameRequiredIdentifier(facts.ExpectedTargetPriceID, facts.TargetPriceID) {
		return errors.New("catalog migration target Stripe Price mismatch")
	}
	return nil
}

func normalizeRecurringPlanSnapshotV1(snapshot RecurringPlanSnapshotV1) RecurringPlanSnapshotV1 {
	snapshot.StripePriceID = strings.TrimSpace(snapshot.StripePriceID)
	snapshot.Currency = strings.ToUpper(strings.TrimSpace(snapshot.Currency))
	snapshot.DurationUnit = strings.TrimSpace(snapshot.DurationUnit)
	snapshot.QuotaResetPeriod = strings.TrimSpace(snapshot.QuotaResetPeriod)
	snapshot.UpgradeGroup = strings.TrimSpace(snapshot.UpgradeGroup)
	return snapshot
}

func validateRecurringPlanSnapshotReset(snapshot RecurringPlanSnapshotV1) error {
	switch snapshot.QuotaResetPeriod {
	case model.SubscriptionResetNever, model.SubscriptionResetDaily, model.SubscriptionResetWeekly, model.SubscriptionResetMonthly:
		if snapshot.QuotaResetCustomSeconds != 0 {
			return errors.New("recurring plan snapshot reset custom seconds are invalid")
		}
	case model.SubscriptionResetCustom:
		if snapshot.QuotaResetCustomSeconds <= 0 {
			return errors.New("recurring plan snapshot custom reset is invalid")
		}
	default:
		return errors.New("recurring plan snapshot reset period is invalid")
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode recurring plan snapshot: %w", err)
	}
	return errors.New("decode recurring plan snapshot: trailing JSON value")
}

func containsContractID(allowed []int64, contractID int64) bool {
	for _, candidate := range allowed {
		if candidate == contractID {
			return true
		}
	}
	return false
}

func sameRequiredIdentifier(expected string, observed string) bool {
	expected = strings.TrimSpace(expected)
	observed = strings.TrimSpace(observed)
	return expected != "" && expected == observed
}

func validateLegacyRecurringProviderPayload(raw string, binding *model.SubscriptionProviderBinding) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	values := make(map[string]string)
	for _, part := range strings.Split(raw, ";") {
		key, value, ok := strings.Cut(part, "=")
		if ok {
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	if value := values["subscription_id"]; value != "" && value != strings.TrimSpace(binding.ProviderSubscriptionId) {
		return errors.New("legacy recurring source subscription identity mismatch")
	}
	if value := values["customer_id"]; value != "" && value != strings.TrimSpace(binding.ProviderCustomerId) {
		return errors.New("legacy recurring source customer identity mismatch")
	}
	return nil
}
