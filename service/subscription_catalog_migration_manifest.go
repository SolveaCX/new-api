package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stripe/stripe-go/v86"
	stripeprice "github.com/stripe/stripe-go/v86/price"
	stripesubscription "github.com/stripe/stripe-go/v86/subscription"
	"gorm.io/gorm"
)

const (
	CatalogMigrationReasonEligible                     = "eligible"
	CatalogMigrationReasonContractNotFound             = "contract_not_found"
	CatalogMigrationReasonContractNotAllowlisted       = "contract_not_allowlisted"
	CatalogMigrationReasonMappingMissing               = "mapping_missing"
	CatalogMigrationReasonMappingAmbiguous             = "mapping_ambiguous"
	CatalogMigrationReasonMappingInvalid               = "mapping_invalid"
	CatalogMigrationReasonContractState                = "contract_state_invalid"
	CatalogMigrationReasonRenewalDisabled              = "renewal_disabled"
	CatalogMigrationReasonPeriodInvalid                = "period_invalid"
	CatalogMigrationReasonPendingChange                = "pending_change"
	CatalogMigrationReasonEntitlementMismatch          = "entitlement_mismatch"
	CatalogMigrationReasonSourcePlanMismatch           = "source_plan_mismatch"
	CatalogMigrationReasonTargetPlanMismatch           = "target_plan_mismatch"
	CatalogMigrationReasonProviderBindingMismatch      = "provider_binding_mismatch"
	CatalogMigrationReasonProviderLifecycleBusy        = "provider_lifecycle_busy"
	CatalogMigrationReasonProviderScheduleConflict     = "provider_schedule_conflict"
	CatalogMigrationReasonProviderInventoryUnavailable = "provider_inventory_unavailable"
	CatalogMigrationReasonProviderFactsMismatch        = "provider_facts_mismatch"
	CatalogMigrationReasonSnapshotMismatch             = "snapshot_mismatch"
	CatalogMigrationReasonSandboxGuardClosed           = "sandbox_guard_closed"
	CatalogMigrationReasonPaymentShapeMismatch         = "payment_shape_mismatch"
)

type CatalogMigrationMappingExpectation struct {
	SourcePlanID                    int    `json:"source_plan_id"`
	TargetPlanID                    int    `json:"target_plan_id"`
	ExpectedTierRank                int    `json:"expected_tier_rank"`
	ExpectedBasePriceMinor          int64  `json:"expected_base_price_minor"`
	ExpectedCurrency                string `json:"expected_currency"`
	ExpectedDurationUnit            string `json:"expected_duration_unit"`
	ExpectedDurationValue           int    `json:"expected_duration_value"`
	ExpectedDurationCustomSeconds   int64  `json:"expected_duration_custom_seconds"`
	ExpectedQuotaResetPeriod        string `json:"expected_quota_reset_period"`
	ExpectedQuotaResetCustomSeconds int64  `json:"expected_quota_reset_custom_seconds"`
	ExpectedTargetTotalAmount       int64  `json:"expected_target_total_amount"`
	ExpectedMediaCreditsMonthly     int64  `json:"expected_media_credits_monthly"`
	ExpectedWindow5hAmount          int64  `json:"expected_window_5h_amount"`
	ExpectedWindowWeekAmount        int64  `json:"expected_window_week_amount"`
}

type CatalogMigrationPreviewRequest struct {
	RequestID   string                               `json:"request_id"`
	Mappings    []CatalogMigrationMappingExpectation `json:"mappings"`
	ContractIDs []int64                              `json:"contract_ids"`
}

// CatalogMigrationStripeSubscription is the minimum read-only provider view
// needed by preview. It deliberately contains no mutation handle.
type CatalogMigrationStripeSubscription struct {
	ID                 string
	CustomerID         string
	ItemIDs            []string
	PriceIDs           []string
	Status             string
	CurrentPeriodStart int64
	CurrentPeriodEnd   int64
	ScheduleID         string
	CancelAtPeriodEnd  bool
	Livemode           bool
}

type CatalogMigrationStripeInventory interface {
	GetPrice(ctx context.Context, priceID string) (*stripe.Price, error)
	GetSubscription(ctx context.Context, subscriptionID string) (CatalogMigrationStripeSubscription, error)
}

type CatalogMigrationPreviewService struct {
	DB      *gorm.DB
	Stripe  CatalogMigrationStripeInventory
	Sandbox CatalogMigrationSandboxConfig
}

type CatalogMigrationPreviewResult struct {
	RequestID    string                                  `json:"request_id"`
	CohortDigest string                                  `json:"cohort_digest"`
	Mappings     []CatalogMigrationMappingExpectation    `json:"mappings"`
	Contracts    []CatalogMigrationContractPreviewResult `json:"contracts"`
	Summary      CatalogMigrationPreviewSummary          `json:"summary"`
}

type CatalogMigrationPreviewSummary struct {
	Requested  int            `json:"requested"`
	Eligible   int            `json:"eligible"`
	Ineligible int            `json:"ineligible"`
	ByReason   map[string]int `json:"by_reason"`
}

type CatalogMigrationContractPreviewResult struct {
	ContractID                int64  `json:"contract_id"`
	Eligible                  bool   `json:"eligible"`
	Reason                    string `json:"reason"`
	PaymentMode               string `json:"payment_mode,omitempty"`
	SourcePlanID              int    `json:"source_plan_id,omitempty"`
	TargetPlanID              int    `json:"target_plan_id,omitempty"`
	EffectiveAt               int64  `json:"effective_at,omitempty"`
	PriorLatestChangeIntentID int64  `json:"prior_latest_change_intent_id,omitempty"`
	ExpectedChangeVersion     int64  `json:"expected_change_version"`
	ContractFingerprint       string `json:"contract_fingerprint,omitempty"`
	EntitlementFingerprint    string `json:"entitlement_fingerprint,omitempty"`
	SourcePlanFingerprint     string `json:"source_plan_fingerprint,omitempty"`
	TargetPlanFingerprint     string `json:"target_plan_fingerprint,omitempty"`
	BindingFingerprint        string `json:"binding_fingerprint,omitempty"`
	ProviderFingerprint       string `json:"provider_fingerprint,omitempty"`
	CurrentSnapshotHash       string `json:"current_snapshot_hash,omitempty"`
	TargetSnapshotHash        string `json:"target_snapshot_hash,omitempty"`
	// Canonical snapshots are intentionally excluded from preview JSON. Apply
	// consumes the exact same typed/frozen values without rebuilding plan facts.
	CurrentPlanSnapshot string `json:"-"`
	TargetPlanSnapshot  string `json:"-"`
}

func (s CatalogMigrationPreviewService) Preview(ctx context.Context, request CatalogMigrationPreviewRequest) (CatalogMigrationPreviewResult, error) {
	return previewSubscriptionCatalogMigrationWithDependencies(ctx, s.DB, s.Stripe, s.Sandbox, request)
}

// PreviewSubscriptionCatalogMigration is the runtime facade used by the
// operations controller. Its dependency-bearing implementation remains private
// so callers cannot weaken the sandbox configuration through request fields.
func PreviewSubscriptionCatalogMigration(ctx context.Context, request CatalogMigrationPreviewRequest) (CatalogMigrationPreviewResult, error) {
	return previewSubscriptionCatalogMigrationWithDependencies(ctx, model.DB, catalogMigrationRuntimeStripeInventory, catalogMigrationRuntimeSandboxConfig(), request)
}

var catalogMigrationRuntimeStripeInventory CatalogMigrationStripeInventory = stripeCatalogMigrationInventory{}

func previewSubscriptionCatalogMigrationWithDependencies(ctx context.Context, db *gorm.DB, inventory CatalogMigrationStripeInventory, sandbox CatalogMigrationSandboxConfig, request CatalogMigrationPreviewRequest) (CatalogMigrationPreviewResult, error) {
	if db == nil {
		db = model.DB
	}
	request.RequestID = strings.TrimSpace(request.RequestID)
	if request.RequestID == "" {
		return CatalogMigrationPreviewResult{}, errors.New("catalog migration request_id is required")
	}
	if len(request.ContractIDs) == 0 {
		return CatalogMigrationPreviewResult{}, errors.New("catalog migration contract allowlist is empty")
	}
	if len(request.Mappings) == 0 {
		return CatalogMigrationPreviewResult{}, errors.New("catalog migration mappings are empty")
	}

	mappings := canonicalCatalogMigrationMappings(request.Mappings)
	contractIDs := canonicalContractIDs(request.ContractIDs)
	result := CatalogMigrationPreviewResult{
		RequestID: request.RequestID,
		Mappings:  mappings,
		Contracts: make([]CatalogMigrationContractPreviewResult, 0, len(contractIDs)),
		Summary: CatalogMigrationPreviewSummary{
			Requested: len(contractIDs),
			ByReason:  map[string]int{},
		},
	}
	for _, contractID := range contractIDs {
		item := previewCatalogMigrationContract(ctx, db, inventory, sandbox, mappings, contractID)
		result.Contracts = append(result.Contracts, item)
		result.Summary.ByReason[item.Reason]++
		if item.Eligible {
			result.Summary.Eligible++
		} else {
			result.Summary.Ineligible++
		}
	}
	digest, err := catalogMigrationPreviewDigest(result)
	if err != nil {
		return CatalogMigrationPreviewResult{}, err
	}
	result.CohortDigest = digest
	return result, nil
}

type stripeCatalogMigrationInventory struct{}

func (stripeCatalogMigrationInventory) GetPrice(ctx context.Context, priceID string) (*stripe.Price, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ensureStripeSecretForSubscription(); err != nil {
		return nil, err
	}
	stripe.Key = setting.StripeApiSecret
	return stripeprice.Get(strings.TrimSpace(priceID), nil)
}

func (stripeCatalogMigrationInventory) GetSubscription(ctx context.Context, subscriptionID string) (CatalogMigrationStripeSubscription, error) {
	if err := ctx.Err(); err != nil {
		return CatalogMigrationStripeSubscription{}, err
	}
	if err := ensureStripeSecretForSubscription(); err != nil {
		return CatalogMigrationStripeSubscription{}, err
	}
	stripe.Key = setting.StripeApiSecret
	params := &stripe.SubscriptionParams{}
	params.AddExpand("customer")
	params.AddExpand("items.data.price")
	params.AddExpand("schedule")
	subscription, err := stripesubscription.Get(strings.TrimSpace(subscriptionID), params)
	if err != nil {
		return CatalogMigrationStripeSubscription{}, err
	}
	if subscription == nil {
		return CatalogMigrationStripeSubscription{}, errors.New("Stripe subscription is missing")
	}
	result := CatalogMigrationStripeSubscription{
		ID: subscription.ID, Status: string(subscription.Status),
		CancelAtPeriodEnd: subscription.CancelAtPeriodEnd, Livemode: subscription.Livemode,
	}
	if subscription.Customer != nil {
		result.CustomerID = subscription.Customer.ID
	}
	if subscription.Schedule != nil {
		result.ScheduleID = subscription.Schedule.ID
	}
	if subscription.Items != nil {
		for _, item := range subscription.Items.Data {
			if item == nil {
				continue
			}
			result.ItemIDs = append(result.ItemIDs, item.ID)
			if item.Price != nil {
				result.PriceIDs = append(result.PriceIDs, item.Price.ID)
			} else {
				result.PriceIDs = append(result.PriceIDs, "")
			}
			if result.CurrentPeriodStart == 0 {
				result.CurrentPeriodStart = item.CurrentPeriodStart
				result.CurrentPeriodEnd = item.CurrentPeriodEnd
			}
		}
	}
	return result, nil
}

func catalogMigrationRuntimeSandboxConfig() CatalogMigrationSandboxConfig {
	return CatalogMigrationSandboxConfig{
		DeploymentEnvironment: strings.TrimSpace(os.Getenv("FLATKEY_DEPLOYMENT_ENV")),
		ServiceName:           strings.TrimSpace(os.Getenv("K_SERVICE")),
		FeatureEnabled:        parseCatalogMigrationBool(os.Getenv("SUBSCRIPTION_CATALOG_MIGRATION_ENABLED")),
		AllowedContractIDs:    parseCatalogMigrationContractIDs(os.Getenv("SUBSCRIPTION_CATALOG_MIGRATION_CONTRACT_ALLOWLIST")),
		StripeSecret:          setting.StripeApiSecret,
		StripePublishableKey:  setting.StripePublishableKey,
	}
}

func parseCatalogMigrationBool(raw string) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	return err == nil && value
}

func parseCatalogMigrationContractIDs(raw string) []int64 {
	parts := strings.Split(raw, ",")
	result := make([]int64, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err == nil && id > 0 {
			result = append(result, id)
		}
	}
	return canonicalContractIDs(result)
}

func previewCatalogMigrationContract(ctx context.Context, db *gorm.DB, inventory CatalogMigrationStripeInventory, sandbox CatalogMigrationSandboxConfig, mappings []CatalogMigrationMappingExpectation, contractID int64) CatalogMigrationContractPreviewResult {
	item := CatalogMigrationContractPreviewResult{ContractID: contractID, Reason: CatalogMigrationReasonContractNotFound}
	if err := ValidateCatalogMigrationCommonSandbox(sandbox, contractID); err != nil {
		item.Reason = CatalogMigrationReasonSandboxGuardClosed
		if !containsContractID(sandbox.AllowedContractIDs, contractID) {
			item.Reason = CatalogMigrationReasonContractNotAllowlisted
		}
		return item
	}

	var contract model.UserSubscriptionContract
	if err := db.WithContext(ctx).Where("id = ?", contractID).First(&contract).Error; err != nil {
		return item
	}
	item.PaymentMode = contract.PaymentMode
	item.SourcePlanID = contract.CurrentPlanId
	item.EffectiveAt = contract.CurrentPeriodEnd
	item.PriorLatestChangeIntentID = contract.LatestChangeIntentId
	item.ExpectedChangeVersion = contract.ChangeVersion
	item.ContractFingerprint = fingerprintCatalogMigrationContract(contract)

	matches := mappingsForSource(mappings, contract.CurrentPlanId)
	if len(matches) == 0 {
		item.Reason = CatalogMigrationReasonMappingMissing
		return item
	}
	if len(matches) != 1 {
		item.Reason = CatalogMigrationReasonMappingAmbiguous
		return item
	}
	mapping := matches[0]
	item.TargetPlanID = mapping.TargetPlanID
	if err := validateCatalogMigrationMappingExpectation(mapping); err != nil {
		item.Reason = CatalogMigrationReasonMappingInvalid
		return item
	}
	if contract.Status != model.SubscriptionContractStatusActive {
		item.Reason = CatalogMigrationReasonContractState
		return item
	}
	if contract.RenewalStatus != model.SubscriptionRenewalStatusEnabled {
		item.Reason = CatalogMigrationReasonRenewalDisabled
		return item
	}
	if contract.CurrentPeriodStart <= 0 || contract.CurrentPeriodEnd <= contract.CurrentPeriodStart {
		item.Reason = CatalogMigrationReasonPeriodInvalid
		return item
	}
	if contract.PendingPlanId != 0 || contract.PendingEffectiveAt != 0 || hasUnresolvedCatalogMigrationConflict(ctx, db, contract.Id) {
		item.Reason = CatalogMigrationReasonPendingChange
		return item
	}

	var entitlement model.UserSubscription
	if contract.CurrentEntitlementId <= 0 || db.WithContext(ctx).Where("id = ?", contract.CurrentEntitlementId).First(&entitlement).Error != nil ||
		entitlement.ContractId != contract.Id || entitlement.UserId != contract.UserId || entitlement.PlanId != contract.CurrentPlanId || entitlement.CurrentSlot == nil || *entitlement.CurrentSlot != 1 || entitlement.Status != "active" ||
		entitlement.StartTime != contract.CurrentPeriodStart || entitlement.EndTime != contract.CurrentPeriodEnd {
		item.Reason = CatalogMigrationReasonEntitlementMismatch
		return item
	}
	item.EntitlementFingerprint = fingerprintJSON(entitlement)
	if !catalogMigrationPaymentShapeMatches(contract, entitlement) {
		item.Reason = CatalogMigrationReasonPaymentShapeMismatch
		return item
	}

	var sourcePlan, targetPlan model.SubscriptionPlan
	if db.WithContext(ctx).Where("id = ?", contract.CurrentPlanId).First(&sourcePlan).Error != nil {
		item.Reason = CatalogMigrationReasonSourcePlanMismatch
		return item
	}
	if db.WithContext(ctx).Where("id = ?", mapping.TargetPlanID).First(&targetPlan).Error != nil {
		item.Reason = CatalogMigrationReasonTargetPlanMismatch
		return item
	}
	item.SourcePlanFingerprint = fingerprintPlan(sourcePlan)
	item.TargetPlanFingerprint = fingerprintPlan(targetPlan)
	if err := validateCatalogMigrationPlanPair(sourcePlan, targetPlan, mapping); err != nil {
		item.Reason = CatalogMigrationReasonTargetPlanMismatch
		return item
	}

	targetSnapshot, err := recurringSnapshotFromPlanOnly(&targetPlan)
	if err != nil {
		item.Reason = CatalogMigrationReasonTargetPlanMismatch
		return item
	}
	item.TargetSnapshotHash, _ = RecurringPlanSnapshotV1Fingerprint(targetSnapshot)
	item.TargetPlanSnapshot, _ = EncodeRecurringPlanSnapshotV1(targetSnapshot)

	if contract.PaymentMode != model.SubscriptionPaymentModeStripeRecurring {
		if targetPlan.AllowBalancePay != nil && !*targetPlan.AllowBalancePay {
			item.Reason = CatalogMigrationReasonTargetPlanMismatch
			return item
		}
		currentSnapshot, snapshotErr := recurringSnapshotFromPlanOnly(&sourcePlan)
		if snapshotErr != nil || !entitlementMatchesSnapshot(entitlement, currentSnapshot) {
			item.Reason = CatalogMigrationReasonSnapshotMismatch
			return item
		}
		item.CurrentSnapshotHash, _ = RecurringPlanSnapshotV1Fingerprint(currentSnapshot)
		item.CurrentPlanSnapshot, _ = EncodeRecurringPlanSnapshotV1(currentSnapshot)
		item.Eligible = true
		item.Reason = CatalogMigrationReasonEligible
		return item
	}

	var binding model.SubscriptionProviderBinding
	if contract.CurrentProviderBindingId <= 0 || db.WithContext(ctx).Where("id = ?", contract.CurrentProviderBindingId).First(&binding).Error != nil ||
		binding.ContractId != contract.Id || binding.UserId != contract.UserId || binding.PlanId != contract.CurrentPlanId || strings.TrimSpace(binding.Provider) != model.PaymentProviderStripe || binding.EndedAt != 0 {
		item.Reason = CatalogMigrationReasonProviderBindingMismatch
		return item
	}
	item.BindingFingerprint = fingerprintCatalogMigrationBinding(binding)
	if binding.Livemode || binding.CancelAtPeriodEnd || strings.TrimSpace(binding.ProviderStatus) != "active" || binding.CurrentPeriodStart != contract.CurrentPeriodStart || binding.CurrentPeriodEnd != contract.CurrentPeriodEnd {
		item.Reason = CatalogMigrationReasonProviderFactsMismatch
		return item
	}
	if strings.TrimSpace(binding.LifecycleReservationToken) != "" || binding.LifecycleReservationUntil > 0 {
		item.Reason = CatalogMigrationReasonProviderLifecycleBusy
		return item
	}
	if strings.TrimSpace(binding.ProviderScheduleId) != "" {
		item.Reason = CatalogMigrationReasonProviderScheduleConflict
		return item
	}
	if inventory == nil {
		item.Reason = CatalogMigrationReasonProviderInventoryUnavailable
		return item
	}
	if !catalogMigrationHasTestStripeCredentials(sandbox) {
		item.Reason = CatalogMigrationReasonSandboxGuardClosed
		return item
	}
	providerSub, err := inventory.GetSubscription(ctx, binding.ProviderSubscriptionId)
	if err != nil {
		item.Reason = CatalogMigrationReasonProviderInventoryUnavailable
		return item
	}
	currentPrice, err := inventory.GetPrice(ctx, binding.ProviderPriceId)
	if err != nil {
		item.Reason = CatalogMigrationReasonProviderInventoryUnavailable
		return item
	}
	targetPrice, err := inventory.GetPrice(ctx, targetPlan.StripePriceId)
	if err != nil {
		item.Reason = CatalogMigrationReasonProviderInventoryUnavailable
		return item
	}
	if err := ValidateRecurringPlanSnapshotV1AgainstStripePrice(targetSnapshot, targetPrice); err != nil {
		item.Reason = CatalogMigrationReasonTargetPlanMismatch
		return item
	}
	if err := validateCatalogMigrationProviderFacts(sandbox, contract.Id, binding, providerSub, currentPrice, targetPrice, targetPlan.StripePriceId); err != nil {
		item.Reason = CatalogMigrationReasonProviderFactsMismatch
		return item
	}
	item.ProviderFingerprint = fingerprintJSON(struct {
		Subscription, Customer, Item, Schedule, Status string
		PeriodStart, PeriodEnd                         int64
		Livemode, CancelAtPeriodEnd                    bool
		CurrentPrice, TargetPrice                      any
	}{redactIdentifier(providerSub.ID), redactIdentifier(providerSub.CustomerID), redactIdentifier(providerSub.ItemIDs[0]), redactIdentifier(providerSub.ScheduleID), providerSub.Status, providerSub.CurrentPeriodStart, providerSub.CurrentPeriodEnd, providerSub.Livemode, providerSub.CancelAtPeriodEnd, catalogMigrationPriceFingerprintFacts(currentPrice), catalogMigrationPriceFingerprintFacts(targetPrice)})

	var currentSnapshot RecurringPlanSnapshotV1
	if strings.TrimSpace(binding.CurrentPlanSnapshot) != "" {
		currentSnapshot, err = DecodeRecurringPlanSnapshotV1(binding.CurrentPlanSnapshot)
		if err == nil {
			err = ValidateRecurringPlanSnapshotV1AgainstPlan(currentSnapshot, &sourcePlan)
		}
		if err == nil {
			err = ValidateRecurringPlanSnapshotV1AgainstStripePrice(currentSnapshot, currentPrice)
		}
	} else {
		var order model.SubscriptionOrder
		if binding.InitialOrderId <= 0 || db.WithContext(ctx).Where("id = ?", binding.InitialOrderId).First(&order).Error != nil {
			err = errors.New("legacy initial order is missing")
		} else {
			currentSnapshot, err = FreezeLegacyRecurringPlanSnapshotV1(&order, &binding, &sourcePlan, currentPrice)
		}
	}
	if err != nil || !entitlementMatchesSnapshot(entitlement, currentSnapshot) {
		item.Reason = CatalogMigrationReasonSnapshotMismatch
		return item
	}
	item.CurrentSnapshotHash, _ = RecurringPlanSnapshotV1Fingerprint(currentSnapshot)
	item.CurrentPlanSnapshot, _ = EncodeRecurringPlanSnapshotV1(currentSnapshot)
	item.Eligible = true
	item.Reason = CatalogMigrationReasonEligible
	return item
}

func validateCatalogMigrationMappingExpectation(mapping CatalogMigrationMappingExpectation) error {
	if mapping.SourcePlanID <= 0 || mapping.TargetPlanID <= 0 || mapping.SourcePlanID == mapping.TargetPlanID || mapping.ExpectedTierRank <= 0 || mapping.ExpectedBasePriceMinor <= 0 || mapping.ExpectedTargetTotalAmount < 0 || mapping.ExpectedMediaCreditsMonthly < 0 || mapping.ExpectedWindow5hAmount != 0 || mapping.ExpectedWindowWeekAmount != 0 {
		return errors.New("catalog migration mapping expectation is invalid")
	}
	if strings.ToUpper(strings.TrimSpace(mapping.ExpectedCurrency)) != "USD" || strings.TrimSpace(mapping.ExpectedDurationUnit) != model.SubscriptionDurationMonth || mapping.ExpectedDurationValue != 1 || mapping.ExpectedDurationCustomSeconds != 0 {
		return errors.New("catalog migration mapping billing expectation is invalid")
	}
	if strings.TrimSpace(mapping.ExpectedQuotaResetPeriod) == "" {
		return errors.New("catalog migration reset expectation is invalid")
	}
	expectedQuotaUnits, exists := map[int64]int64{1000: 13, 3000: 45, 10000: 170}[mapping.ExpectedBasePriceMinor]
	quotaPerUnit := int64(common.QuotaPerUnit)
	if !exists || int64(mapping.ExpectedTierRank) != map[int64]int64{1000: 1, 3000: 2, 10000: 3}[mapping.ExpectedBasePriceMinor] ||
		quotaPerUnit <= 0 || float64(quotaPerUnit) != common.QuotaPerUnit || mapping.ExpectedTargetTotalAmount != expectedQuotaUnits*quotaPerUnit {
		return errors.New("catalog migration tier price and quota expectation is invalid")
	}
	return nil
}

func validateCatalogMigrationPlanPair(source model.SubscriptionPlan, target model.SubscriptionPlan, expected CatalogMigrationMappingExpectation) error {
	if source.Id != expected.SourcePlanID || target.Id != expected.TargetPlanID || !target.Enabled || source.TierRank == nil || target.TierRank == nil || *source.TierRank != expected.ExpectedTierRank || *target.TierRank != expected.ExpectedTierRank {
		return errors.New("catalog migration plan identity mismatch")
	}
	for _, plan := range []model.SubscriptionPlan{source, target} {
		minor, err := stripeMinorUnitAmountForSubscription(plan.PriceAmount, plan.Currency)
		if err != nil || minor != expected.ExpectedBasePriceMinor || strings.ToUpper(strings.TrimSpace(plan.Currency)) != strings.ToUpper(strings.TrimSpace(expected.ExpectedCurrency)) || plan.DurationUnit != expected.ExpectedDurationUnit || plan.DurationValue != expected.ExpectedDurationValue || plan.CustomSeconds != expected.ExpectedDurationCustomSeconds || plan.QuotaResetPeriod != expected.ExpectedQuotaResetPeriod || plan.QuotaResetCustomSeconds != expected.ExpectedQuotaResetCustomSeconds || strings.TrimSpace(plan.StripePriceId) == "" {
			return errors.New("catalog migration plan billing facts mismatch")
		}
	}
	if target.TotalAmount != expected.ExpectedTargetTotalAmount || target.MediaCreditsMonthly != expected.ExpectedMediaCreditsMonthly || target.Window5hAmount != expected.ExpectedWindow5hAmount || target.WindowWeekAmount != expected.ExpectedWindowWeekAmount {
		return errors.New("catalog migration target benefits mismatch")
	}
	return nil
}

func validateCatalogMigrationProviderFacts(config CatalogMigrationSandboxConfig, contractID int64, binding model.SubscriptionProviderBinding, sub CatalogMigrationStripeSubscription, currentPrice, targetPrice *stripe.Price, targetPriceID string) error {
	if len(sub.ItemIDs) != 1 || len(sub.PriceIDs) != 1 || strings.TrimSpace(sub.ScheduleID) != "" || sub.CancelAtPeriodEnd || strings.TrimSpace(sub.Status) != "active" || strings.TrimSpace(binding.ProviderStatus) != "active" || strings.TrimSpace(sub.Status) != strings.TrimSpace(binding.ProviderStatus) || sub.CurrentPeriodStart != binding.CurrentPeriodStart || sub.CurrentPeriodEnd != binding.CurrentPeriodEnd {
		return errors.New("catalog migration Stripe subscription facts mismatch")
	}
	if strings.TrimSpace(binding.ProviderSubscriptionItemId) == "" || strings.TrimSpace(binding.ProviderSubscriptionItemId) != strings.TrimSpace(sub.ItemIDs[0]) {
		return errors.New("catalog migration Stripe item mismatch")
	}
	if err := ValidateCatalogMigrationStripeSandbox(config, CatalogMigrationStripeSandboxFacts{
		ContractID: contractID, BindingLivemode: binding.Livemode, SubscriptionLivemode: sub.Livemode,
		CurrentPriceLivemode: currentPrice == nil || currentPrice.Livemode, TargetPriceLivemode: targetPrice == nil || targetPrice.Livemode,
		BindingSubscriptionID: binding.ProviderSubscriptionId, SubscriptionID: sub.ID,
		BindingCustomerID: binding.ProviderCustomerId, SubscriptionCustomerID: sub.CustomerID,
		BindingCurrentPriceID: binding.ProviderPriceId, SubscriptionPriceID: sub.PriceIDs[0],
		ExpectedTargetPriceID: targetPriceID, TargetPriceID: priceID(targetPrice),
	}); err != nil {
		return err
	}
	return nil
}

func recurringSnapshotFromPlanOnly(plan *model.SubscriptionPlan) (RecurringPlanSnapshotV1, error) {
	if plan == nil {
		return RecurringPlanSnapshotV1{}, errors.New("subscription plan is missing")
	}
	minor, err := stripeMinorUnitAmountForSubscription(plan.PriceAmount, plan.Currency)
	if err != nil {
		return RecurringPlanSnapshotV1{}, err
	}
	snapshot := RecurringPlanSnapshotV1{
		Version: RecurringPlanSnapshotVersionV1, PlanID: plan.Id, StripePriceID: plan.StripePriceId,
		Currency: strings.ToUpper(strings.TrimSpace(plan.Currency)), BasePriceMinor: minor,
		DurationUnit: plan.DurationUnit, DurationValue: plan.DurationValue, DurationCustomSeconds: plan.CustomSeconds,
		QuotaResetPeriod: plan.QuotaResetPeriod, QuotaResetCustomSeconds: plan.QuotaResetCustomSeconds,
		TotalAmount: plan.TotalAmount, MediaCreditsMonthly: plan.MediaCreditsMonthly,
		Window5hAmount: plan.Window5hAmount, WindowWeekAmount: plan.WindowWeekAmount, UpgradeGroup: plan.UpgradeGroup,
	}
	if err := ValidateRecurringPlanSnapshotV1AgainstPlan(snapshot, plan); err != nil {
		return RecurringPlanSnapshotV1{}, err
	}
	return snapshot, nil
}

func entitlementMatchesSnapshot(entitlement model.UserSubscription, snapshot RecurringPlanSnapshotV1) bool {
	if entitlement.AmountTotal != snapshot.TotalAmount || entitlement.MediaCreditsTotal != snapshot.MediaCreditsMonthly || strings.TrimSpace(entitlement.UpgradeGroup) != snapshot.UpgradeGroup {
		return false
	}
	if entitlement.Window5hAmount != nil && *entitlement.Window5hAmount != snapshot.Window5hAmount {
		return false
	}
	if entitlement.WindowWeekAmount != nil && *entitlement.WindowWeekAmount != snapshot.WindowWeekAmount {
		return false
	}
	return true
}

func hasUnresolvedCatalogMigrationConflict(ctx context.Context, db *gorm.DB, contractID int64) bool {
	var count int64
	err := db.WithContext(ctx).Model(&model.SubscriptionChangeIntent{}).
		Where("contract_id = ? AND status IN ?", contractID, []string{
			model.SubscriptionChangeIntentStatusCreated, model.SubscriptionChangeIntentStatusSyncing,
			model.SubscriptionChangeIntentStatusAwaitingPayment, model.SubscriptionChangeIntentStatusScheduled,
			model.SubscriptionChangeIntentStatusCompensationRequired, model.SubscriptionChangeIntentStatusNeedsAttention,
		}).Count(&count).Error
	return err != nil || count != 0
}

func canonicalCatalogMigrationMappings(input []CatalogMigrationMappingExpectation) []CatalogMigrationMappingExpectation {
	result := append([]CatalogMigrationMappingExpectation(nil), input...)
	for index := range result {
		result[index].ExpectedCurrency = strings.ToUpper(strings.TrimSpace(result[index].ExpectedCurrency))
		result[index].ExpectedDurationUnit = strings.TrimSpace(result[index].ExpectedDurationUnit)
		result[index].ExpectedQuotaResetPeriod = strings.TrimSpace(result[index].ExpectedQuotaResetPeriod)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].SourcePlanID != result[j].SourcePlanID {
			return result[i].SourcePlanID < result[j].SourcePlanID
		}
		if result[i].TargetPlanID != result[j].TargetPlanID {
			return result[i].TargetPlanID < result[j].TargetPlanID
		}
		left, _ := json.Marshal(result[i])
		right, _ := json.Marshal(result[j])
		return string(left) < string(right)
	})
	return result
}

func canonicalContractIDs(input []int64) []int64 {
	seen := make(map[int64]struct{}, len(input))
	result := make([]int64, 0, len(input))
	for _, id := range input {
		if id > 0 {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				result = append(result, id)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func mappingsForSource(mappings []CatalogMigrationMappingExpectation, sourceID int) []CatalogMigrationMappingExpectation {
	result := make([]CatalogMigrationMappingExpectation, 0, 1)
	for _, mapping := range mappings {
		if mapping.SourcePlanID == sourceID {
			result = append(result, mapping)
		}
	}
	return result
}

func catalogMigrationPreviewDigest(result CatalogMigrationPreviewResult) (string, error) {
	result.RequestID = ""
	result.CohortDigest = ""
	result.Mappings = canonicalCatalogMigrationMappings(result.Mappings)
	result.Contracts = append([]CatalogMigrationContractPreviewResult(nil), result.Contracts...)
	sort.Slice(result.Contracts, func(i, j int) bool {
		return result.Contracts[i].ContractID < result.Contracts[j].ContractID
	})
	payload, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("encode catalog migration manifest: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func fingerprintPlan(plan model.SubscriptionPlan) string {
	return fingerprintJSON(struct {
		ID                                                              int
		TierRank                                                        *int
		PriceAmount                                                     float64
		Currency, DurationUnit, PriceID, Reset, Group                   string
		DurationValue                                                   int
		CustomSeconds, ResetSeconds, Total, Media, Window5h, WindowWeek int64
		Enabled                                                         bool
		AllowBalancePay                                                 *bool
	}{plan.Id, plan.TierRank, plan.PriceAmount, plan.Currency, plan.DurationUnit, plan.StripePriceId, plan.QuotaResetPeriod, plan.UpgradeGroup, plan.DurationValue, plan.CustomSeconds, plan.QuotaResetCustomSeconds, plan.TotalAmount, plan.MediaCreditsMonthly, plan.Window5hAmount, plan.WindowWeekAmount, plan.Enabled, plan.AllowBalancePay})
}

func fingerprintCatalogMigrationContract(contract model.UserSubscriptionContract) string {
	return fingerprintJSON(struct {
		ID, Version, EntitlementID, BindingID, LatestIntentID, PeriodStart, PeriodEnd, PendingAt int64
		UserID, PlanID, PendingPlanID                                                            int
		Status, PaymentMode, RenewalSource, RenewalStatus                                        string
	}{contract.Id, contract.ChangeVersion, int64(contract.CurrentEntitlementId), contract.CurrentProviderBindingId, contract.LatestChangeIntentId, contract.CurrentPeriodStart, contract.CurrentPeriodEnd, contract.PendingEffectiveAt, contract.UserId, contract.CurrentPlanId, contract.PendingPlanId, contract.Status, contract.PaymentMode, contract.RenewalSource, contract.RenewalStatus})
}

func fingerprintCatalogMigrationBinding(binding model.SubscriptionProviderBinding) string {
	return fingerprintJSON(struct {
		ID, ContractID, PeriodStart, PeriodEnd, LifecycleSeq, ReservationUntil int64
		UserID, PlanID                                                         int
		Subscription, Customer, Item, Price, Schedule, Status, Reservation     string
		Livemode, CancelAtPeriodEnd                                            bool
	}{binding.Id, binding.ContractId, binding.CurrentPeriodStart, binding.CurrentPeriodEnd, binding.LifecycleActionSeq, binding.LifecycleReservationUntil, binding.UserId, binding.PlanId, redactIdentifier(binding.ProviderSubscriptionId), redactIdentifier(binding.ProviderCustomerId), redactIdentifier(binding.ProviderSubscriptionItemId), redactIdentifier(binding.ProviderPriceId), redactIdentifier(binding.ProviderScheduleId), binding.ProviderStatus, redactIdentifier(binding.LifecycleReservationToken), binding.Livemode, binding.CancelAtPeriodEnd})
}

func catalogMigrationPaymentShapeMatches(contract model.UserSubscriptionContract, entitlement model.UserSubscription) bool {
	switch contract.PaymentMode {
	case model.SubscriptionPaymentModeStripeRecurring:
		return contract.RenewalSource == model.SubscriptionRenewalSourceProvider && contract.CurrentProviderBindingId > 0 && entitlement.PaymentMode == model.SubscriptionPaymentModeStripeRecurring
	case model.SubscriptionPaymentModePrepaid, model.SubscriptionPaymentModeBalanceOnePeriod:
		return contract.RenewalSource == model.SubscriptionRenewalSourceWallet && contract.CurrentProviderBindingId == 0 && entitlement.PaymentMode == contract.PaymentMode
	default:
		return false
	}
}

func catalogMigrationPriceFingerprintFacts(price *stripe.Price) any {
	if price == nil {
		return nil
	}
	interval := ""
	intervalCount := int64(0)
	if price.Recurring != nil {
		interval = string(price.Recurring.Interval)
		intervalCount = price.Recurring.IntervalCount
	}
	return struct {
		ID, Currency, Type, Interval string
		UnitAmount, IntervalCount    int64
		Active, Deleted, Livemode    bool
	}{redactIdentifier(price.ID), strings.ToUpper(strings.TrimSpace(string(price.Currency))), string(price.Type), interval, price.UnitAmount, intervalCount, price.Active, price.Deleted, price.Livemode}
}

func fingerprintJSON(value any) string {
	payload, _ := json.Marshal(value)
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func redactIdentifier(identifier string) string {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return ""
	}
	return fingerprintJSON("provider-identifier:" + identifier)
}

func priceID(price *stripe.Price) string {
	if price == nil {
		return ""
	}
	return strings.TrimSpace(price.ID)
}

func catalogMigrationHasTestStripeCredentials(config CatalogMigrationSandboxConfig) bool {
	secret := strings.TrimSpace(config.StripeSecret)
	publishable := strings.TrimSpace(config.StripePublishableKey)
	return (strings.HasPrefix(secret, "sk_test_") || strings.HasPrefix(secret, "rk_test_")) && strings.HasPrefix(publishable, "pk_test_")
}

func isCatalogMigrationTerminalProviderStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "canceled", "incomplete_expired", "unpaid":
		return true
	default:
		return false
	}
}
