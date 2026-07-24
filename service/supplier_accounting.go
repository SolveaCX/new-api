package service

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/shopspring/decimal"
)

const supplierMicroUSDScale int64 = 1_000_000

const supplierPricingInputVersionV1 int64 = 1

const supplierExpressionFingerprintTailMaxV1 uint64 = 1<<48 - 1

type SupplierAccountingCaptureInputV1 struct {
	OfficialListUSD            *decimal.Decimal
	OfficialEvidenceReason     string
	PricingMode                string
	TieredTokenParams          *billingexpr.TokenParams
	AudioPricingApplied        bool
	ToolPricingApplied         bool
	ImagePricingApplied        bool
	UnknownOfficialAmountCount uint32
}

type SupplierAccountingEnvelopeInputV1 struct {
	RelayInfo  *relaycommon.RelayInfo
	Settlement types.BillingSettlementResult
	// HasPositiveFinalUsage is final billable-usage evidence for the selected
	// path. It is not the settlement adjustment delta and is not necessarily a
	// token count: explicit image/tool calls or a positive final charge may be
	// evidence with zero tokens, but selecting fixed pricing alone is not.
	HasPositiveFinalUsage bool
	Capture               SupplierAccountingCaptureInputV1
}

type SupplierAccountingFailureReason string

const (
	SupplierAccountingReasonNegative       SupplierAccountingFailureReason = "negative"
	SupplierAccountingReasonNonFinite      SupplierAccountingFailureReason = "non_finite"
	SupplierAccountingReasonInvalidDivisor SupplierAccountingFailureReason = "invalid_divisor"
	SupplierAccountingReasonOverflow       SupplierAccountingFailureReason = "overflow"
	SupplierAccountingReasonUnknown        SupplierAccountingFailureReason = "unknown"
	SupplierAccountingReasonMissing        SupplierAccountingFailureReason = "missing"
)

type SupplierAccountingError struct {
	Field  string
	Reason SupplierAccountingFailureReason
	Err    error
}

func (e *SupplierAccountingError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return fmt.Sprintf("supplier accounting %s: %s", e.Field, e.Reason)
	}
	return fmt.Sprintf("supplier accounting %s: %s: %v", e.Field, e.Reason, e.Err)
}

func (e *SupplierAccountingError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *SupplierAccountingError) QualityReason() string {
	if e == nil {
		return ""
	}
	return "supplier_accounting." + e.Field + "." + string(e.Reason)
}

func calculateSupplierTieredResult(relayInfo *relaycommon.RelayInfo, params billingexpr.TokenParams) (*billingexpr.TieredResult, error) {
	if relayInfo == nil || relayInfo.SupplierOfficialPricingSnapshot.TieredBillingSnapshot == nil {
		return nil, newSupplierAccountingError("tiered_pricing_snapshot", SupplierAccountingReasonMissing, nil)
	}
	pricing := relayInfo.SupplierOfficialPricingSnapshot
	requestInput := billingexpr.RequestInput{}
	if pricing.BillingRequestInput != nil {
		requestInput = *pricing.BillingRequestInput
	}
	result, err := billingexpr.ComputeTieredQuotaWithRequest(pricing.TieredBillingSnapshot, params, requestInput)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SupplierOfficialUSDToMicro converts one frozen decimal USD amount to micro-USD.
// Nil means unknown. Rounding happens exactly once using ROUND_HALF_UP.
func SupplierOfficialUSDToMicro(officialUSD *decimal.Decimal) (*int64, error) {
	if officialUSD == nil {
		return nil, nil
	}
	if officialUSD.IsNegative() {
		return nil, newSupplierAccountingError("official_list_usd", SupplierAccountingReasonNegative, nil)
	}
	return supplierDecimalToMicro("official_list_usd", officialUSD.Mul(decimal.NewFromInt(supplierMicroUSDScale)))
}

// SupplierProcurementMicro computes official_micro * ppm / 1e6 with one final
// ROUND_HALF_UP operation. Nil official amount remains unknown.
func SupplierProcurementMicro(officialMicroUSD *int64, procurementMultiplierPpm int64) (*int64, error) {
	if officialMicroUSD == nil {
		return nil, nil
	}
	if *officialMicroUSD < 0 || procurementMultiplierPpm < 0 {
		return nil, newSupplierAccountingError("procurement_cost_micro_usd", SupplierAccountingReasonNegative, nil)
	}
	value, ok := types.CalculateSupplierProcurementMicroV1(*officialMicroUSD, procurementMultiplierPpm)
	if !ok {
		return nil, newSupplierAccountingError("procurement_cost_micro_usd", SupplierAccountingReasonOverflow, nil)
	}
	return &value, nil
}

// SupplierSalesMicro converts final quota to micro-USD using the frozen quota
// divisor. No binary floating-point arithmetic is used.
func SupplierSalesMicro(finalSalesQuota *int64, quotaPerUnitSnapshot *string) (*int64, error) {
	if finalSalesQuota == nil {
		return nil, nil
	}
	if *finalSalesQuota < 0 {
		return nil, newSupplierAccountingError("sales_micro_usd", SupplierAccountingReasonNegative, nil)
	}
	quotaPerUnit, err := parseSupplierQuotaPerUnit(quotaPerUnitSnapshot)
	if err != nil {
		return nil, err
	}
	value := decimal.NewFromInt(*finalSalesQuota).
		Div(quotaPerUnit).
		Mul(decimal.NewFromInt(supplierMicroUSDScale))
	return supplierDecimalToMicro("sales_micro_usd", value)
}

// SupplierGrossMicro subtracts frozen procurement cost from frozen sales. Nil
// operands preserve partial-known state; negative gross profit is valid.
func SupplierGrossMicro(salesMicroUSD *int64, procurementMicroUSD *int64) (*int64, error) {
	if salesMicroUSD == nil || procurementMicroUSD == nil {
		return nil, nil
	}
	if *salesMicroUSD < 0 || *procurementMicroUSD < 0 {
		return nil, newSupplierAccountingError("gross_profit_micro_usd", SupplierAccountingReasonNegative, nil)
	}
	value := decimal.NewFromInt(*salesMicroUSD).Sub(decimal.NewFromInt(*procurementMicroUSD))
	return supplierDecimalToMicro("gross_profit_micro_usd", value)
}

// BuildSupplierAccountingLogSnapshotV1 freezes the supplier-side financial
// values into the same durable consume log as the customer settlement.
func BuildSupplierAccountingLogSnapshotV1(
	relayInfo *relaycommon.RelayInfo,
	settlement types.BillingSettlementResult,
	officialListUSD *decimal.Decimal,
	officialEvidenceReason string,
	pricingMode string,
) *types.SupplierAccountingLogSnapshotV1 {
	if relayInfo == nil || !settlement.FinanciallyCommitted || settlement.Err != nil || settlement.FinanciallyCommittedAt <= 0 || !relayInfo.SupplierCostSnapshot.IsBound() {
		return nil
	}
	cost := relayInfo.SupplierCostSnapshot
	scope := relayInfo.SupplierStatisticsScopeSnapshot
	if scope.Scope == "" {
		scope = types.BusinessSupplierStatisticsScopeSnapshot()
	}
	decision := "included"
	var exclusionRuleID *int
	if scope.Scope == types.SupplierStatisticsScopeInternal {
		decision = "excluded"
		if scope.ExclusionRuleId > 0 {
			exclusionRuleID = cloneSupplierInt(scope.ExclusionRuleId)
		}
	}

	officialMicroUSD, officialErr := SupplierOfficialUSDToMicro(officialListUSD)
	procurementMicroUSD, procurementErr := SupplierProcurementMicro(officialMicroUSD, cost.ProcurementMultiplierPpm)
	var salesMultiplierPpm *int64
	var salesMultiplierErr error
	var salesMicroUSD *int64
	var salesErr error
	var quotaPerUnit *string
	if relayInfo.SupplierOfficialPricingSnapshot.Loaded {
		quotaPerUnit = cloneSupplierString(&relayInfo.SupplierOfficialPricingSnapshot.QuotaPerUnit)
	}
	if scope.Scope != types.SupplierStatisticsScopeInternal {
		salesMultiplierPpm, salesMultiplierErr = supplierSalesMultiplierPpm(supplierRequestTimeSalesMultiplier(relayInfo))
		finalQuota := int64(settlement.FinalSalesQuota)
		salesMicroUSD, salesErr = SupplierSalesMicro(&finalQuota, quotaPerUnit)
	} else {
		quotaPerUnit = nil
	}
	grossMicroUSD, grossErr := SupplierGrossMicro(salesMicroUSD, procurementMicroUSD)

	var pricingModeSnapshot *string
	if scope.Scope != types.SupplierStatisticsScopeInternal {
		if normalized := strings.TrimSpace(pricingMode); normalized != "" {
			pricingModeSnapshot = &normalized
		}
	}
	reasons := make([]string, 0, 5)
	for _, err := range []error{officialErr, procurementErr, salesMultiplierErr, salesErr, grossErr} {
		if err != nil {
			reasons = append(reasons, supplierAccountingErrorQualityReason(err))
		}
	}
	if reason := strings.TrimSpace(officialEvidenceReason); reason != "" {
		reasons = append(reasons, reason)
	}

	return &types.SupplierAccountingLogSnapshotV1{
		BindingVersionId:         cost.BindingVersionId,
		SupplierId:               cost.SupplierId,
		ContractId:               cost.ContractId,
		RateVersionId:            cost.RateVersionId,
		ProcurementMultiplierPpm: cost.ProcurementMultiplierPpm,
		SalesMultiplierPpm:       cloneSupplierInt64(salesMultiplierPpm),
		OfficialListMicroUsd:     cloneSupplierInt64(officialMicroUSD),
		SalesMicroUsd:            cloneSupplierInt64(salesMicroUSD),
		ProcurementCostMicroUsd:  cloneSupplierInt64(procurementMicroUSD),
		GrossProfitMicroUsd:      cloneSupplierInt64(grossMicroUSD),
		StatisticsScope:          string(scope.Scope),
		ExclusionDecision:        decision,
		ExclusionRuleId:          exclusionRuleID,
		QuotaPerUnit:             quotaPerUnit,
		PricingMode:              pricingModeSnapshot,
		FinanciallyCommittedAt:   settlement.FinanciallyCommittedAt,
		QualityReason:            strings.Join(reasons, ";"),
	}
}

func supplierRequestTimeSalesMultiplier(relayInfo *relaycommon.RelayInfo) float64 {
	// PriceData is updated after every retry selection and is the settlement
	// source for ratio, fixed-price, and audio billing. Tiered snapshots are kept
	// in lockstep by ApplyResolvedGroupRatio before the successful attempt.
	return relayInfo.PriceData.GroupRatioInfo.GroupRatio
}

func supplierSalesMultiplierPpm(multiplier float64) (*int64, error) {
	if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) {
		return nil, newSupplierAccountingError("sales_multiplier_ppm", SupplierAccountingReasonNonFinite, nil)
	}
	if multiplier < 0 {
		return nil, newSupplierAccountingError("sales_multiplier_ppm", SupplierAccountingReasonNegative, nil)
	}
	// Existing group/model ratio validation permits more than six decimal
	// places. Canonicalize to the persisted ppm scale with deterministic
	// ROUND_HALF_UP instead of turning otherwise valid traffic into a gap.
	ppm := decimal.NewFromFloat(multiplier).
		Mul(decimal.NewFromInt(supplierMicroUSDScale)).
		Round(0)
	if ppm.GreaterThan(decimal.NewFromInt(math.MaxInt64)) {
		return nil, newSupplierAccountingError("sales_multiplier_ppm", SupplierAccountingReasonOverflow, nil)
	}
	value := ppm.IntPart()
	return &value, nil
}

// BuildSupplierAccountingEnvelopeV1 applies the fixed mutually-exclusive
// disposition order. Snapshot failures are converted to a snapshot-free
// producer_error only after all captured preconditions have passed.
func BuildSupplierAccountingEnvelopeV1(input SupplierAccountingEnvelopeInputV1) types.SupplierAccountingEnvelopeV1 {
	envelope, _ := buildSupplierAccountingEnvelopeV1(input)
	return envelope
}

func buildSupplierAccountingEnvelopeV1(input SupplierAccountingEnvelopeInputV1) (types.SupplierAccountingEnvelopeV1, error) {
	envelope := newSupplierAccountingDispositionEnvelope(types.SupplierAccountingDispositionNotFinanciallyCommitted)
	if !input.Settlement.FinanciallyCommitted || input.Settlement.Err != nil {
		return envelope, nil
	}
	if !input.HasPositiveFinalUsage {
		envelope.Disposition = types.SupplierAccountingDispositionZeroUsage
		return envelope, nil
	}
	if input.RelayInfo != nil && input.RelayInfo.SupplierCostSnapshot.CacheUnavailable {
		envelope.Disposition = types.SupplierAccountingDispositionProducerError
		return envelope, newSupplierAccountingError("supplier_cache", SupplierAccountingReasonMissing, nil)
	}
	if input.RelayInfo == nil || supplierAccountingBindingAbsent(input.RelayInfo.SupplierCostSnapshot) {
		envelope.Disposition = types.SupplierAccountingDispositionUnbound
		return envelope, nil
	}
	if !supplierAccountingBindingValid(input.RelayInfo.SupplierCostSnapshot) {
		envelope.Disposition = types.SupplierAccountingDispositionProducerError
		return envelope, newSupplierAccountingError("supplier_binding", SupplierAccountingReasonUnknown, nil)
	}
	internal := input.RelayInfo.SupplierStatisticsScopeSnapshot.Scope == types.SupplierStatisticsScopeInternal
	pricingMode := ""
	if !internal {
		var err error
		pricingMode, err = resolvedSupplierAccountingPricingModeV1(input.RelayInfo, input.Capture.PricingMode)
		if err != nil {
			envelope.Disposition = types.SupplierAccountingDispositionProducerError
			return envelope, err
		}
		input.Capture.PricingMode = pricingMode
	}

	snapshot := BuildSupplierAccountingLogSnapshotV1(
		input.RelayInfo,
		input.Settlement,
		input.Capture.OfficialListUSD,
		input.Capture.OfficialEvidenceReason,
		pricingMode,
	)
	var buildErr error
	if snapshot == nil {
		buildErr = newSupplierAccountingError("captured_snapshot", SupplierAccountingReasonMissing, nil)
	} else {
		buildErr = supplierAccountingErrorFromQualityReason(snapshot.QualityReason)
		snapshot.QualityReason = ""
		snapshot.QuotaPerUnit = nil
		snapshot.PricingMode = nil
		snapshot.UnknownOfficialCount = input.Capture.UnknownOfficialAmountCount
		if buildErr == nil {
			if !internal {
				provenance, err := buildSupplierPricingProvenanceV1(input.RelayInfo, input.Capture)
				if err != nil {
					buildErr = err
				} else {
					snapshot.PricingProvenance = provenance
				}
			}
			if buildErr == nil {
				envelope.Disposition = types.SupplierAccountingDispositionCaptured
				envelope.Captured = snapshot
				if validationErr := ValidateSupplierAccountingEnvelopeV1(envelope); validationErr == nil {
					return envelope, nil
				} else {
					buildErr = validationErr
				}
			}
		}
	}
	envelope.Disposition = types.SupplierAccountingDispositionProducerError
	envelope.Captured = nil
	return envelope, buildErr
}

func supplierAccountingErrorFromQualityReason(reason string) error {
	if strings.TrimSpace(reason) == "" {
		return nil
	}
	for _, candidate := range []SupplierAccountingFailureReason{
		SupplierAccountingReasonNegative,
		SupplierAccountingReasonNonFinite,
		SupplierAccountingReasonInvalidDivisor,
		SupplierAccountingReasonOverflow,
		SupplierAccountingReasonMissing,
		SupplierAccountingReasonUnknown,
	} {
		if strings.Contains(reason, "."+string(candidate)) {
			return newSupplierAccountingError("captured_snapshot", candidate, nil)
		}
	}
	return newSupplierAccountingError("captured_snapshot", SupplierAccountingReasonUnknown, nil)
}

func InjectSupplierAccountingEnvelopeV1(other map[string]any, input SupplierAccountingEnvelopeInputV1) types.SupplierAccountingEnvelopeV1 {
	envelope, _ := buildSupplierAccountingEnvelopeV1(input)
	if other != nil {
		delete(other, types.SupplierAccountingEnvelopeKeyV1)
		if envelope.Disposition == types.SupplierAccountingDispositionCaptured && ValidateSupplierAccountingEnvelopeV1(envelope) == nil {
			other[types.SupplierAccountingEnvelopeKeyV1] = envelope
		}
	}
	return envelope
}

func InjectUnsupportedSupplierAccountingEnvelopeV1(other map[string]any) types.SupplierAccountingEnvelopeV1 {
	envelope := newSupplierAccountingDispositionEnvelope(types.SupplierAccountingDispositionUnsupportedPath)
	if other != nil {
		delete(other, types.SupplierAccountingEnvelopeKeyV1)
	}
	return envelope
}

func newSupplierAccountingDispositionEnvelope(disposition types.SupplierAccountingDisposition) types.SupplierAccountingEnvelopeV1 {
	return types.SupplierAccountingEnvelopeV1{
		EnvelopeSchemaVersion: types.SupplierAccountingEnvelopeSchemaVersionV1,
		Disposition:           disposition,
	}
}

func supplierAccountingBindingValid(cost types.SupplierCostSnapshot) bool {
	return cost.BindingVersionId > 0 && cost.SupplierId > 0 && cost.ContractId > 0 && cost.RateVersionId > 0
}

func supplierAccountingBindingAbsent(cost types.SupplierCostSnapshot) bool {
	return cost.BindingVersionId == 0 && cost.SupplierId == 0 && cost.ContractId == 0 && cost.RateVersionId == 0
}

func supplierAccountingOfficialPricingModeV1(relayInfo *relaycommon.RelayInfo) string {
	if relayInfo == nil {
		return "ratio"
	}
	pricing := relayInfo.SupplierOfficialPricingSnapshot
	if pricing.TieredBillingSnapshot != nil {
		return "tiered_expr"
	}
	if pricing.PriceData.UsePrice {
		return "fixed"
	}
	return "ratio"
}

func resolvedSupplierAccountingPricingModeV1(relayInfo *relaycommon.RelayInfo, claimed string) (string, error) {
	actual := supplierAccountingOfficialPricingModeV1(relayInfo)
	normalizedClaim := normalizeSupplierAccountingPricingModeV1(claimed)
	if strings.TrimSpace(claimed) != "" && normalizedClaim != actual {
		return "", newSupplierAccountingError("pricing_mode", SupplierAccountingReasonUnknown, nil)
	}
	return actual, nil
}

func normalizeSupplierAccountingPricingModeV1(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "ratio":
		return "ratio"
	case "price", "fixed":
		return "fixed"
	case "tiered", "tiered_expr":
		return "tiered_expr"
	default:
		return ""
	}
}

func buildSupplierPricingProvenanceV1(relayInfo *relaycommon.RelayInfo, input SupplierAccountingCaptureInputV1) (*types.SupplierPricingProvenanceV1, error) {
	internal := relayInfo.SupplierStatisticsScopeSnapshot.Scope == types.SupplierStatisticsScopeInternal
	var groupMultiplier, groupRatioVersion int64
	if !internal {
		var err error
		groupMultiplier, err = requiredSupplierMultiplierPpm("group_ratio", relayInfo.PriceData.GroupRatioInfo.GroupRatio)
		if err != nil {
			return nil, err
		}
		groupRatioVersion = supplierPricingInputVersionV1
	}
	provenance := &types.SupplierPricingProvenanceV1{}
	switch strings.ToLower(strings.TrimSpace(input.PricingMode)) {
	case "ratio":
		modelMultiplier, multiplierErr := requiredSupplierMultiplierPpm("model_ratio", relayInfo.SupplierOfficialPricingSnapshot.PriceData.ModelRatio)
		if multiplierErr != nil {
			return nil, multiplierErr
		}
		provenance.Ratio = &types.SupplierRatioPricingProvenanceV1{
			ModelRatioPpm: modelMultiplier, GroupRatioPpm: groupMultiplier,
			ModelRatioVersion: supplierPricingInputVersionV1, GroupRatioVersion: groupRatioVersion,
		}
	case "price", "fixed":
		provenance.Fixed = &types.SupplierFixedPricingProvenanceV1{
			Source: "price_data", Key: "model_price", PriceVersion: supplierPricingInputVersionV1,
			GroupMultiplierPpm: groupMultiplier, GroupRatioVersion: groupRatioVersion,
		}
	case "tiered", "tiered_expr":
		tiered, tieredErr := buildSupplierTieredPricingProvenanceV1(relayInfo, groupMultiplier, groupRatioVersion, input.TieredTokenParams)
		if tieredErr != nil {
			return nil, tieredErr
		}
		provenance.Tiered = tiered
	default:
		return nil, newSupplierAccountingError("pricing_mode", SupplierAccountingReasonUnknown, nil)
	}
	if input.AudioPricingApplied || input.ToolPricingApplied || input.ImagePricingApplied {
		provenance.Dimensions = &types.SupplierPricingDimensionsV1{
			Audio: input.AudioPricingApplied, Tool: input.ToolPricingApplied, Image: input.ImagePricingApplied,
		}
	}
	return provenance, nil
}

func buildSupplierTieredPricingProvenanceV1(relayInfo *relaycommon.RelayInfo, groupMultiplier int64, groupRatioVersion int64, params *billingexpr.TokenParams) (*types.SupplierTieredPricingProvenanceV1, error) {
	if relayInfo.SupplierOfficialPricingSnapshot.TieredBillingSnapshot == nil || params == nil {
		return nil, newSupplierAccountingError("tiered_provenance", SupplierAccountingReasonMissing, nil)
	}
	snapshot := relayInfo.SupplierOfficialPricingSnapshot.TieredBillingSnapshot
	if strings.TrimSpace(snapshot.ExprString) == "" {
		return nil, newSupplierAccountingError("tiered_expression", SupplierAccountingReasonMissing, nil)
	}
	digest := sha256.Sum256([]byte(snapshot.ExprString))
	canonicalHash := hex.EncodeToString(digest[:])
	if snapshot.ExprHash != "" && snapshot.ExprHash != canonicalHash {
		return nil, newSupplierAccountingError("tiered_expression", SupplierAccountingReasonUnknown, nil)
	}
	fingerprint := binary.BigEndian.Uint64(digest[:8])
	fingerprintTail := uint64(digest[8])<<40 |
		uint64(digest[9])<<32 |
		uint64(digest[10])<<24 |
		uint64(digest[11])<<16 |
		uint64(digest[12])<<8 |
		uint64(digest[13])
	if (fingerprint == 0 && fingerprintTail == 0) || int64(snapshot.ExprVersion) != supplierPricingInputVersionV1 {
		return nil, newSupplierAccountingError("tiered_expression", SupplierAccountingReasonUnknown, nil)
	}
	normalized, err := supplierTieredNormalizedInputsV1(*params)
	if err != nil {
		return nil, err
	}
	return &types.SupplierTieredPricingProvenanceV1{
		ExpressionFingerprint:     fingerprint,
		ExpressionFingerprintTail: fingerprintTail,
		ExpressionVersion:         int64(snapshot.ExprVersion),
		GroupMultiplierPpm:        groupMultiplier,
		GroupRatioVersion:         groupRatioVersion,
		NormalizedInputs:          normalized,
	}, nil
}

func supplierTieredNormalizedInputsV1(params billingexpr.TokenParams) (types.SupplierTieredNormalizedInputsV1, error) {
	values := []struct {
		field string
		value float64
	}{
		{"tiered_prompt", params.P}, {"tiered_completion", params.C}, {"tiered_context_length", params.Len},
		{"tiered_cache_read", params.CR}, {"tiered_cache_create", params.CC}, {"tiered_cache_create_1h", params.CC1h},
		{"tiered_image_input", params.Img}, {"tiered_image_output", params.ImgO},
		{"tiered_audio_input", params.AI}, {"tiered_audio_output", params.AO},
	}
	converted := make([]int64, len(values))
	for index, item := range values {
		value, err := requiredSupplierWholeNonNegative(item.field, item.value)
		if err != nil {
			return types.SupplierTieredNormalizedInputsV1{}, err
		}
		converted[index] = value
	}
	return types.SupplierTieredNormalizedInputsV1{
		Prompt: converted[0], Completion: converted[1], ContextLength: converted[2], CacheRead: converted[3],
		CacheCreate: converted[4], CacheCreate1H: converted[5], ImageInput: converted[6], ImageOutput: converted[7],
		AudioInput: converted[8], AudioOutput: converted[9],
	}, nil
}

func requiredSupplierMultiplierPpm(field string, multiplier float64) (int64, error) {
	value, err := supplierSalesMultiplierPpm(multiplier)
	if err != nil || value == nil {
		if err != nil {
			return 0, err
		}
		return 0, newSupplierAccountingError(field, SupplierAccountingReasonMissing, nil)
	}
	return *value, nil
}

func requiredSupplierWholeNonNegative(field string, value float64) (int64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, newSupplierAccountingError(field, SupplierAccountingReasonNonFinite, nil)
	}
	if value < 0 {
		return 0, newSupplierAccountingError(field, SupplierAccountingReasonNegative, nil)
	}
	if value > math.MaxInt64 || value != math.Trunc(value) {
		return 0, newSupplierAccountingError(field, SupplierAccountingReasonOverflow, nil)
	}
	return int64(value), nil
}

func ValidateSupplierAccountingEnvelopeV1(envelope types.SupplierAccountingEnvelopeV1) error {
	if envelope.EnvelopeSchemaVersion != types.SupplierAccountingEnvelopeSchemaVersionV1 {
		return newSupplierAccountingError("envelope_version", SupplierAccountingReasonUnknown, nil)
	}
	switch envelope.Disposition {
	case types.SupplierAccountingDispositionCaptured:
		if envelope.Captured == nil {
			return newSupplierAccountingError("captured_snapshot", SupplierAccountingReasonMissing, nil)
		}
		return validateSupplierAccountingCapturedV1(envelope.Captured)
	case types.SupplierAccountingDispositionUnsupportedPath,
		types.SupplierAccountingDispositionNotFinanciallyCommitted,
		types.SupplierAccountingDispositionZeroUsage,
		types.SupplierAccountingDispositionUnbound,
		types.SupplierAccountingDispositionProducerError:
		if envelope.Captured != nil {
			return newSupplierAccountingError("captured_snapshot", SupplierAccountingReasonUnknown, nil)
		}
		return nil
	default:
		return newSupplierAccountingError("disposition", SupplierAccountingReasonUnknown, nil)
	}
}

func validateSupplierAccountingCapturedV1(snapshot *types.SupplierAccountingLogSnapshotV1) error {
	if snapshot == nil || snapshot.BindingVersionId <= 0 || snapshot.SupplierId <= 0 || snapshot.ContractId <= 0 ||
		snapshot.RateVersionId <= 0 || snapshot.ProcurementMultiplierPpm < 0 || snapshot.FinanciallyCommittedAt <= 0 {
		return newSupplierAccountingError("captured_identity", SupplierAccountingReasonMissing, nil)
	}
	if snapshot.OfficialListMicroUsd == nil || *snapshot.OfficialListMicroUsd < 0 ||
		snapshot.ProcurementCostMicroUsd == nil || *snapshot.ProcurementCostMicroUsd < 0 {
		return newSupplierAccountingError("captured_cost", SupplierAccountingReasonMissing, nil)
	}
	expectedProcurement, validProcurement := types.CalculateSupplierProcurementMicroV1(*snapshot.OfficialListMicroUsd, snapshot.ProcurementMultiplierPpm)
	if !validProcurement || expectedProcurement != *snapshot.ProcurementCostMicroUsd {
		return newSupplierAccountingError("procurement_formula", SupplierAccountingReasonUnknown, nil)
	}
	if snapshot.QualityReason != "" || snapshot.UnknownOfficialCount != 0 || snapshot.QuotaPerUnit != nil || snapshot.PricingMode != nil {
		return newSupplierAccountingError("captured_forbidden_field", SupplierAccountingReasonUnknown, nil)
	}
	switch types.SupplierStatisticsScope(snapshot.StatisticsScope) {
	case types.SupplierStatisticsScopeBusiness:
		groupMultiplier, err := validateSupplierPricingProvenanceV1(snapshot.PricingProvenance)
		if err != nil {
			return err
		}
		if snapshot.ExclusionDecision != "included" || snapshot.ExclusionRuleId != nil || snapshot.SalesMultiplierPpm == nil ||
			snapshot.SalesMicroUsd == nil || snapshot.GrossProfitMicroUsd == nil || *snapshot.SalesMultiplierPpm < 0 || *snapshot.SalesMicroUsd < 0 {
			return newSupplierAccountingError("business_snapshot", SupplierAccountingReasonMissing, nil)
		}
		if *snapshot.SalesMultiplierPpm != groupMultiplier || !supplierGrossProfitFormulaValid(*snapshot.SalesMicroUsd, *snapshot.ProcurementCostMicroUsd, *snapshot.GrossProfitMicroUsd) {
			return newSupplierAccountingError("business_formula", SupplierAccountingReasonUnknown, nil)
		}
	case types.SupplierStatisticsScopeInternal:
		if snapshot.ExclusionDecision != "excluded" || snapshot.ExclusionRuleId == nil || *snapshot.ExclusionRuleId <= 0 ||
			snapshot.SalesMultiplierPpm != nil || snapshot.SalesMicroUsd != nil || snapshot.GrossProfitMicroUsd != nil ||
			snapshot.PricingProvenance != nil {
			return newSupplierAccountingError("internal_snapshot", SupplierAccountingReasonUnknown, nil)
		}
	default:
		return newSupplierAccountingError("statistics_scope", SupplierAccountingReasonUnknown, nil)
	}
	return nil
}

func validateSupplierPricingProvenanceV1(provenance *types.SupplierPricingProvenanceV1) (int64, error) {
	if provenance == nil {
		return 0, newSupplierAccountingError("pricing_provenance", SupplierAccountingReasonMissing, nil)
	}
	memberCount := 0
	if provenance.Ratio != nil {
		memberCount++
	}
	if provenance.Fixed != nil {
		memberCount++
	}
	if provenance.Tiered != nil {
		memberCount++
	}
	if memberCount != 1 {
		return 0, newSupplierAccountingError("pricing_provenance_union", SupplierAccountingReasonUnknown, nil)
	}
	if provenance.Dimensions != nil && !provenance.Dimensions.Audio && !provenance.Dimensions.Tool && !provenance.Dimensions.Image {
		return 0, newSupplierAccountingError("pricing_dimensions", SupplierAccountingReasonUnknown, nil)
	}
	if ratio := provenance.Ratio; ratio != nil {
		if ratio.ModelRatioPpm < 0 || ratio.ModelRatioVersion != supplierPricingInputVersionV1 ||
			!supplierGroupPricingEvidenceValidV1(ratio.GroupRatioPpm, ratio.GroupRatioVersion) {
			return 0, newSupplierAccountingError("ratio_provenance", SupplierAccountingReasonUnknown, nil)
		}
		return ratio.GroupRatioPpm, nil
	}
	if fixed := provenance.Fixed; fixed != nil {
		if fixed.Source != "price_data" || fixed.Key != "model_price" ||
			fixed.PriceVersion != supplierPricingInputVersionV1 ||
			!supplierGroupPricingEvidenceValidV1(fixed.GroupMultiplierPpm, fixed.GroupRatioVersion) {
			return 0, newSupplierAccountingError("fixed_provenance", SupplierAccountingReasonUnknown, nil)
		}
		return fixed.GroupMultiplierPpm, nil
	}
	tiered := provenance.Tiered
	if (tiered.ExpressionFingerprint == 0 && tiered.ExpressionFingerprintTail == 0) ||
		tiered.ExpressionFingerprintTail > supplierExpressionFingerprintTailMaxV1 ||
		tiered.ExpressionVersion != 1 || !supplierGroupPricingEvidenceValidV1(tiered.GroupMultiplierPpm, tiered.GroupRatioVersion) ||
		!supplierTieredInputsNonNegative(tiered.NormalizedInputs) {
		return 0, newSupplierAccountingError("tiered_provenance", SupplierAccountingReasonUnknown, nil)
	}
	return tiered.GroupMultiplierPpm, nil
}

func supplierGroupPricingEvidenceValidV1(multiplier, version int64) bool {
	return multiplier >= 0 && version == supplierPricingInputVersionV1
}

func supplierTieredInputsNonNegative(input types.SupplierTieredNormalizedInputsV1) bool {
	return input.Prompt >= 0 && input.Completion >= 0 && input.ContextLength >= 0 && input.CacheRead >= 0 &&
		input.CacheCreate >= 0 && input.CacheCreate1H >= 0 && input.ImageInput >= 0 && input.ImageOutput >= 0 &&
		input.AudioInput >= 0 && input.AudioOutput >= 0
}

func supplierGrossProfitFormulaValid(sales int64, procurement int64, gross int64) bool {
	// Both inputs are non-negative, so subtraction cannot overflow below
	// MinInt64. Compare without performing an overflowing addition.
	return sales-procurement == gross
}

func appendSupplierQualityReason(current string, reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return current
	}
	if strings.TrimSpace(current) == "" {
		return reason
	}
	return current + ";" + reason
}

func parseSupplierQuotaPerUnit(value *string) (decimal.Decimal, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return decimal.Zero, newSupplierAccountingError("quota_per_unit_snapshot", SupplierAccountingReasonInvalidDivisor, nil)
	}
	normalized := strings.ToLower(strings.TrimSpace(*value))
	switch normalized {
	case "nan", "+nan", "-nan", "inf", "+inf", "-inf", "infinity", "+infinity", "-infinity":
		return decimal.Zero, newSupplierAccountingError("quota_per_unit_snapshot", SupplierAccountingReasonNonFinite, nil)
	}
	parsed, err := decimal.NewFromString(normalized)
	if err != nil {
		return decimal.Zero, newSupplierAccountingError("quota_per_unit_snapshot", SupplierAccountingReasonInvalidDivisor, err)
	}
	if parsed.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, newSupplierAccountingError("quota_per_unit_snapshot", SupplierAccountingReasonInvalidDivisor, nil)
	}
	return parsed, nil
}

func supplierDecimalToMicro(field string, value decimal.Decimal) (*int64, error) {
	rounded := value.Round(0)
	maxInt64 := decimal.NewFromInt(math.MaxInt64)
	minInt64 := decimal.NewFromInt(math.MinInt64)
	if rounded.GreaterThan(maxInt64) || rounded.LessThan(minInt64) {
		return nil, newSupplierAccountingError(field, SupplierAccountingReasonOverflow, nil)
	}
	result := rounded.IntPart()
	return &result, nil
}

func newSupplierAccountingError(field string, reason SupplierAccountingFailureReason, err error) error {
	return &SupplierAccountingError{Field: field, Reason: reason, Err: err}
}

func supplierAccountingErrorQualityReason(err error) string {
	var accountingErr *SupplierAccountingError
	if errors.As(err, &accountingErr) {
		return accountingErr.QualityReason()
	}
	return supplierAccountingQualityReason("unknown", SupplierAccountingReasonUnknown)
}

func supplierAccountingQualityReason(field string, reason SupplierAccountingFailureReason) string {
	return "supplier_accounting." + field + "." + string(reason)
}

func cloneSupplierInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func cloneSupplierInt(value int) *int {
	copyValue := value
	return &copyValue
}

func cloneSupplierString(value *string) *string {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}
