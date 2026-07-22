package service

import (
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
	if procurementMultiplierPpm > supplierMicroUSDScale {
		return nil, newSupplierAccountingError("procurement_multiplier_ppm", SupplierAccountingReasonInvalidDivisor, nil)
	}
	value := decimal.NewFromInt(*officialMicroUSD).
		Mul(decimal.NewFromInt(procurementMultiplierPpm)).
		Div(decimal.NewFromInt(supplierMicroUSDScale))
	return supplierDecimalToMicro("procurement_cost_micro_usd", value)
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
	if relayInfo == nil || !settlement.FinanciallyCommitted || settlement.FinanciallyCommittedAt <= 0 || !relayInfo.SupplierCostSnapshot.IsBound() {
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
	ppm := decimal.NewFromFloat(multiplier).Mul(decimal.NewFromInt(supplierMicroUSDScale))
	if !ppm.Equal(ppm.Truncate(0)) {
		return nil, newSupplierAccountingError("sales_multiplier_ppm", SupplierAccountingReasonUnknown, nil)
	}
	if ppm.GreaterThan(decimal.NewFromInt(math.MaxInt64)) {
		return nil, newSupplierAccountingError("sales_multiplier_ppm", SupplierAccountingReasonOverflow, nil)
	}
	value := ppm.IntPart()
	return &value, nil
}

func InjectSupplierAccountingLogSnapshotV1(other map[string]interface{}, snapshot *types.SupplierAccountingLogSnapshotV1) {
	if other != nil && snapshot != nil {
		other["supplier_accounting_v1"] = snapshot
	}
}

func buildSupplierAccountingSnapshotForFinalUsage(
	relayInfo *relaycommon.RelayInfo,
	settlement types.BillingSettlementResult,
	officialListUSD *decimal.Decimal,
	officialEvidenceReason string,
	pricingMode string,
	totalTokens int,
) *types.SupplierAccountingLogSnapshotV1 {
	if totalTokens <= 0 {
		return nil
	}
	return BuildSupplierAccountingLogSnapshotV1(relayInfo, settlement, officialListUSD, officialEvidenceReason, pricingMode)
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
