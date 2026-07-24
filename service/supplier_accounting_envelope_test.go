package service

import (
	"errors"
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func supplierEnvelopeTestRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		PriceData: types.PriceData{
			ModelRatio:     2.5,
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.7},
		},
		SupplierCostSnapshot: types.SupplierCostSnapshot{
			BindingVersionId: 11, SupplierId: 12, ContractId: 13, RateVersionId: 14,
			ProcurementMultiplierPpm: 650_000,
		},
		SupplierStatisticsScopeSnapshot: types.BusinessSupplierStatisticsScopeSnapshot(),
		SupplierOfficialPricingSnapshot: types.SupplierOfficialPricingSnapshot{
			Loaded: true, QuotaPerUnit: "500000",
			PriceData: types.PriceData{ModelRatio: 2.5},
		},
	}
}

func supplierEnvelopeTestInput() SupplierAccountingEnvelopeInputV1 {
	official := decimal.NewFromInt(100)
	return SupplierAccountingEnvelopeInputV1{
		RelayInfo: supplierEnvelopeTestRelayInfo(),
		Settlement: types.BillingSettlementResult{
			FinanciallyCommitted: true, FinanciallyCommittedAt: 1_784_801_200, FinalSalesQuota: 35_000_000,
		},
		HasPositiveFinalUsage: true,
		Capture:               SupplierAccountingCaptureInputV1{OfficialListUSD: &official, PricingMode: "ratio"},
	}
}

func TestSupplierAccountingEnvelopeDispositionOrder(t *testing.T) {
	other := map[string]any{}
	unsupported := InjectUnsupportedSupplierAccountingEnvelopeV1(other)
	require.Equal(t, types.SupplierAccountingDispositionUnsupportedPath, unsupported.Disposition)
	require.Nil(t, unsupported.Captured)
	require.NotContains(t, other, types.SupplierAccountingEnvelopeKeyV1)

	uncommittedInput := supplierEnvelopeTestInput()
	uncommittedInput.Settlement.FinanciallyCommitted = false
	uncommittedInput.HasPositiveFinalUsage = false
	uncommittedInput.RelayInfo.SupplierCostSnapshot = types.SupplierCostSnapshot{}
	require.Equal(t, types.SupplierAccountingDispositionNotFinanciallyCommitted, BuildSupplierAccountingEnvelopeV1(uncommittedInput).Disposition)

	zeroInput := supplierEnvelopeTestInput()
	zeroInput.HasPositiveFinalUsage = false
	zeroInput.RelayInfo.SupplierCostSnapshot = types.SupplierCostSnapshot{}
	require.Equal(t, types.SupplierAccountingDispositionZeroUsage, BuildSupplierAccountingEnvelopeV1(zeroInput).Disposition)

	unboundInput := supplierEnvelopeTestInput()
	unboundInput.RelayInfo.SupplierCostSnapshot = types.SupplierCostSnapshot{}
	require.Equal(t, types.SupplierAccountingDispositionUnbound, BuildSupplierAccountingEnvelopeV1(unboundInput).Disposition)

	captured := BuildSupplierAccountingEnvelopeV1(supplierEnvelopeTestInput())
	require.Equal(t, types.SupplierAccountingDispositionCaptured, captured.Disposition)
	require.NotNil(t, captured.Captured)
	require.NoError(t, ValidateSupplierAccountingEnvelopeV1(captured))

	producerErrorInput := supplierEnvelopeTestInput()
	producerErrorInput.Capture.OfficialListUSD = nil
	producerError := BuildSupplierAccountingEnvelopeV1(producerErrorInput)
	require.Equal(t, types.SupplierAccountingDispositionProducerError, producerError.Disposition)
	require.Nil(t, producerError.Captured)
	for _, input := range []SupplierAccountingEnvelopeInputV1{uncommittedInput, zeroInput, unboundInput, producerErrorInput} {
		other := map[string]any{types.SupplierAccountingEnvelopeKeyV1: "stale"}
		InjectSupplierAccountingEnvelopeV1(other, input)
		require.NotContains(t, other, types.SupplierAccountingEnvelopeKeyV1)
	}
}

func TestSupplierAccountingEnvelopeDoesNotCaptureFailedCommittedSettlement(t *testing.T) {
	input := supplierEnvelopeTestInput()
	input.Settlement.Err = errors.New("settlement failed")
	envelope := BuildSupplierAccountingEnvelopeV1(input)
	require.Equal(t, types.SupplierAccountingDispositionNotFinanciallyCommitted, envelope.Disposition)
	require.Nil(t, envelope.Captured)
}

func TestSupplierAccountingEnvelopeCacheUnavailableFailsClosedAfterFinancialDispositionChecks(t *testing.T) {
	unavailable := supplierEnvelopeTestInput()
	unavailable.RelayInfo.SupplierCostSnapshot = types.SupplierCostSnapshot{CacheUnavailable: true}
	envelope := BuildSupplierAccountingEnvelopeV1(unavailable)
	require.Equal(t, types.SupplierAccountingDispositionProducerError, envelope.Disposition)
	require.Nil(t, envelope.Captured)

	uncommitted := unavailable
	uncommitted.Settlement.FinanciallyCommitted = false
	require.Equal(t, types.SupplierAccountingDispositionNotFinanciallyCommitted, BuildSupplierAccountingEnvelopeV1(uncommitted).Disposition)

	zeroUsage := unavailable
	zeroUsage.HasPositiveFinalUsage = false
	require.Equal(t, types.SupplierAccountingDispositionZeroUsage, BuildSupplierAccountingEnvelopeV1(zeroUsage).Disposition)
}

func TestSupplierAccountingEnvelopeCapturedScopeAndFormulaContracts(t *testing.T) {
	business := BuildSupplierAccountingEnvelopeV1(supplierEnvelopeTestInput())
	require.NoError(t, ValidateSupplierAccountingEnvelopeV1(business))
	require.EqualValues(t, 700_000, *business.Captured.SalesMultiplierPpm)
	require.EqualValues(t, 70_000_000, *business.Captured.SalesMicroUsd)
	require.EqualValues(t, 65_000_000, *business.Captured.ProcurementCostMicroUsd)
	require.EqualValues(t, 5_000_000, *business.Captured.GrossProfitMicroUsd)
	require.Nil(t, business.Captured.QuotaPerUnit)
	require.Nil(t, business.Captured.PricingMode)
	require.Empty(t, business.Captured.QualityReason)

	internalInput := supplierEnvelopeTestInput()
	internalInput.RelayInfo.SupplierStatisticsScopeSnapshot = types.SupplierStatisticsScopeSnapshot{
		Scope: types.SupplierStatisticsScopeInternal, ExclusionRuleId: 91,
	}
	internal := BuildSupplierAccountingEnvelopeV1(internalInput)
	require.Equal(t, types.SupplierAccountingDispositionCaptured, internal.Disposition)
	require.NoError(t, ValidateSupplierAccountingEnvelopeV1(internal))
	require.Nil(t, internal.Captured.SalesMultiplierPpm)
	require.Nil(t, internal.Captured.SalesMicroUsd)
	require.Nil(t, internal.Captured.GrossProfitMicroUsd)
	require.Nil(t, internal.Captured.PricingProvenance)

	internalInput.Capture.PricingMode = "not-a-pricing-mode"
	internalInput.Capture.AudioPricingApplied = true
	internalInput.Capture.ToolPricingApplied = true
	internalInput.Capture.ImagePricingApplied = true
	internalInput.RelayInfo.SupplierOfficialPricingSnapshot.PriceData.ModelRatio = math.NaN()
	internalWithoutPricingEvidence := BuildSupplierAccountingEnvelopeV1(internalInput)
	require.Equal(t, types.SupplierAccountingDispositionCaptured, internalWithoutPricingEvidence.Disposition)
	require.Nil(t, internalWithoutPricingEvidence.Captured.PricingProvenance)
	require.NoError(t, ValidateSupplierAccountingEnvelopeV1(internalWithoutPricingEvidence))

	badFormula := *business.Captured
	wrong := *badFormula.GrossProfitMicroUsd + 1
	badFormula.GrossProfitMicroUsd = &wrong
	broken := business
	broken.Captured = &badFormula
	require.Error(t, ValidateSupplierAccountingEnvelopeV1(broken))

	badProcurement := *business.Captured
	wrongProcurement := *badProcurement.ProcurementCostMicroUsd + 1
	badProcurement.ProcurementCostMicroUsd = &wrongProcurement
	consistentGross := *badProcurement.SalesMicroUsd - wrongProcurement
	badProcurement.GrossProfitMicroUsd = &consistentGross
	brokenProcurement := business
	brokenProcurement.Captured = &badProcurement
	require.Error(t, ValidateSupplierAccountingEnvelopeV1(brokenProcurement), "gross consistency must not hide a procurement-formula mismatch")
	_, err := common.Marshal(brokenProcurement)
	require.Error(t, err, "compact encoding must reject the same procurement-formula mismatch")

	unknownInput := supplierEnvelopeTestInput()
	unknownInput.Capture.UnknownOfficialAmountCount = 1
	require.Equal(t, types.SupplierAccountingDispositionProducerError, BuildSupplierAccountingEnvelopeV1(unknownInput).Disposition)
}

func TestSupplierAccountingEnvelopeRejectsNonAuthoritativeOfficialEvidence(t *testing.T) {
	for _, reason := range []string{
		"supplier_accounting.usage.local_estimate",
		"supplier_accounting.cache_creation_tokens.heuristic",
		"future.non_authoritative_evidence",
	} {
		t.Run(reason, func(t *testing.T) {
			input := supplierEnvelopeTestInput()
			input.Capture.OfficialEvidenceReason = reason
			envelope := BuildSupplierAccountingEnvelopeV1(input)
			require.Equal(t, types.SupplierAccountingDispositionProducerError, envelope.Disposition)
			require.Nil(t, envelope.Captured)
		})
	}
}

func TestSupplierAccountingEnvelopePricingProvenanceUnion(t *testing.T) {
	ratio := BuildSupplierAccountingEnvelopeV1(supplierEnvelopeTestInput())
	require.NotNil(t, ratio.Captured.PricingProvenance.Ratio)
	require.Nil(t, ratio.Captured.PricingProvenance.Fixed)
	require.Nil(t, ratio.Captured.PricingProvenance.Tiered)

	fixedInput := supplierEnvelopeTestInput()
	fixedInput.Capture.PricingMode = "fixed"
	fixedInput.RelayInfo.SupplierOfficialPricingSnapshot.PriceData.UsePrice = true
	fixed := BuildSupplierAccountingEnvelopeV1(fixedInput)
	require.Equal(t, types.SupplierAccountingDispositionCaptured, fixed.Disposition)
	require.NotNil(t, fixed.Captured.PricingProvenance.Fixed)

	tieredInput := supplierEnvelopeTestInput()
	tieredInput.Capture.PricingMode = "tiered_expr"
	tieredInput.Capture.TieredTokenParams = &billingexpr.TokenParams{P: 100, C: 20, Len: 120, CR: 10, Img: 3, AI: 4}
	tieredInput.Capture.AudioPricingApplied = true
	tieredInput.Capture.ImagePricingApplied = true
	expression := "tier(\"base\", p*2.5+c*15)"
	tieredInput.RelayInfo.SupplierOfficialPricingSnapshot.TieredBillingSnapshot = &billingexpr.BillingSnapshot{
		ExprString: expression, ExprHash: billingexpr.ExprHashString(expression), ExprVersion: 1,
	}
	tiered := BuildSupplierAccountingEnvelopeV1(tieredInput)
	require.Equal(t, types.SupplierAccountingDispositionCaptured, tiered.Disposition)
	require.NotNil(t, tiered.Captured.PricingProvenance.Tiered)
	require.True(t, tiered.Captured.PricingProvenance.Dimensions.Audio)
	require.True(t, tiered.Captured.PricingProvenance.Dimensions.Image)

	badUnion := *ratio.Captured
	badProvenance := *badUnion.PricingProvenance
	badProvenance.Fixed = fixed.Captured.PricingProvenance.Fixed
	badUnion.PricingProvenance = &badProvenance
	broken := ratio
	broken.Captured = &badUnion
	require.Error(t, ValidateSupplierAccountingEnvelopeV1(broken))

	nonFinite := supplierEnvelopeTestInput()
	nonFinite.RelayInfo.SupplierOfficialPricingSnapshot.PriceData.ModelRatio = math.NaN()
	require.Equal(t, types.SupplierAccountingDispositionProducerError, BuildSupplierAccountingEnvelopeV1(nonFinite).Disposition)
}

func TestSupplierAccountingEnvelopeRejectsClaimedPricingModeMismatch(t *testing.T) {
	ratioAsFixed := supplierEnvelopeTestInput()
	ratioAsFixed.Capture.PricingMode = "fixed"
	require.Equal(t, types.SupplierAccountingDispositionProducerError, BuildSupplierAccountingEnvelopeV1(ratioAsFixed).Disposition)

	fixedAsRatio := supplierEnvelopeTestInput()
	fixedAsRatio.RelayInfo.SupplierOfficialPricingSnapshot.PriceData.UsePrice = true
	fixedAsRatio.Capture.PricingMode = "ratio"
	require.Equal(t, types.SupplierAccountingDispositionProducerError, BuildSupplierAccountingEnvelopeV1(fixedAsRatio).Disposition)

	tieredAsRatio := supplierEnvelopeTestInput()
	expression := `tier("base", p * 2)`
	tieredAsRatio.RelayInfo.SupplierOfficialPricingSnapshot.TieredBillingSnapshot = &billingexpr.BillingSnapshot{
		ExprString: expression, ExprHash: billingexpr.ExprHashString(expression), ExprVersion: 1,
	}
	tieredAsRatio.Capture.PricingMode = "ratio"
	require.Equal(t, types.SupplierAccountingDispositionProducerError, BuildSupplierAccountingEnvelopeV1(tieredAsRatio).Disposition)
}

func TestSupplierAccountingTieredProvenanceRequiresCanonicalFrozenExpression(t *testing.T) {
	expression := `tier("base", p * 2.5 + c * 15)`
	tests := []struct {
		name        string
		exprString  string
		exprHash    string
		disposition types.SupplierAccountingDisposition
	}{
		{name: "matching frozen hash", exprString: expression, exprHash: billingexpr.ExprHashString(expression), disposition: types.SupplierAccountingDispositionCaptured},
		{name: "empty frozen hash recomputes from expression", exprString: expression, disposition: types.SupplierAccountingDispositionCaptured},
		{name: "stale frozen hash", exprString: expression, exprHash: billingexpr.ExprHashString(`tier("other", p)`), disposition: types.SupplierAccountingDispositionProducerError},
		{name: "malformed frozen hash", exprString: expression, exprHash: "not-a-sha256", disposition: types.SupplierAccountingDispositionProducerError},
		{name: "hash with surrounding whitespace", exprString: expression, exprHash: " " + billingexpr.ExprHashString(expression) + " ", disposition: types.SupplierAccountingDispositionProducerError},
		{name: "missing frozen expression", exprHash: billingexpr.ExprHashString(expression), disposition: types.SupplierAccountingDispositionProducerError},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			input := supplierEnvelopeTestInput()
			input.RelayInfo.SupplierOfficialPricingSnapshot.TieredBillingSnapshot = &billingexpr.BillingSnapshot{
				ExprString:  testCase.exprString,
				ExprHash:    testCase.exprHash,
				ExprVersion: 1,
			}
			params := billingexpr.TokenParams{P: 10, C: 2, Len: 10}
			input.Capture.PricingMode = "tiered_expr"
			input.Capture.TieredTokenParams = &params

			envelope := BuildSupplierAccountingEnvelopeV1(input)
			require.Equal(t, testCase.disposition, envelope.Disposition)
			if testCase.disposition == types.SupplierAccountingDispositionCaptured {
				require.NotNil(t, envelope.Captured)
				require.NotNil(t, envelope.Captured.PricingProvenance.Tiered)
				require.NoError(t, ValidateSupplierAccountingEnvelopeV1(envelope))
			} else {
				require.Nil(t, envelope.Captured)
			}
		})
	}
}

func TestSupplierAccountingTieredProvenanceUsesExact112BitSHA256Prefix(t *testing.T) {
	build := func(expression string) *types.SupplierTieredPricingProvenanceV1 {
		input := supplierEnvelopeTestInput()
		input.RelayInfo.SupplierOfficialPricingSnapshot.TieredBillingSnapshot = &billingexpr.BillingSnapshot{
			ExprString:  expression,
			ExprHash:    billingexpr.ExprHashString(expression),
			ExprVersion: 1,
		}
		params := billingexpr.TokenParams{P: 1}
		input.Capture.PricingMode = "tiered_expr"
		input.Capture.TieredTokenParams = &params
		envelope := BuildSupplierAccountingEnvelopeV1(input)
		require.Equal(t, types.SupplierAccountingDispositionCaptured, envelope.Disposition)
		return envelope.Captured.PricingProvenance.Tiered
	}

	known := build("abc")
	require.Equal(t, uint64(0xba7816bf8f01cfea), known.ExpressionFingerprint)
	require.Equal(t, uint64(0x414140de5dae), known.ExpressionFingerprintTail)

	original := build(`tier("base", p * 2)`)
	changed := build(`tier("base", p * 3)`)
	require.NotEqual(t,
		[2]uint64{original.ExpressionFingerprint, original.ExpressionFingerprintTail},
		[2]uint64{changed.ExpressionFingerprint, changed.ExpressionFingerprintTail},
	)

	plain := build(`tier("base", p * 2)`)
	whitespaceChanged := build("  tier(\"base\", p * 2)\n")
	require.NotEqual(t,
		[2]uint64{plain.ExpressionFingerprint, plain.ExpressionFingerprintTail},
		[2]uint64{whitespaceChanged.ExpressionFingerprint, whitespaceChanged.ExpressionFingerprintTail},
		"the frozen UTF-8 expression must be hashed without whitespace normalization",
	)
}

func TestValidateSupplierAccountingEnvelopeV1RejectsTieredFingerprintTailOverflow(t *testing.T) {
	input := supplierEnvelopeTestInput()
	expression := `tier("base", p * 2)`
	input.RelayInfo.SupplierOfficialPricingSnapshot.TieredBillingSnapshot = &billingexpr.BillingSnapshot{
		ExprString: expression, ExprHash: billingexpr.ExprHashString(expression), ExprVersion: 1,
	}
	params := billingexpr.TokenParams{P: 1}
	input.Capture.PricingMode = "tiered_expr"
	input.Capture.TieredTokenParams = &params
	envelope := BuildSupplierAccountingEnvelopeV1(input)
	require.Equal(t, types.SupplierAccountingDispositionCaptured, envelope.Disposition)

	envelope.Captured.PricingProvenance.Tiered.ExpressionFingerprintTail = 1 << 48
	require.Error(t, ValidateSupplierAccountingEnvelopeV1(envelope))
}

func TestSupplierAccountingEnvelopeDistinguishesAbsentAndCorruptBindings(t *testing.T) {
	absent := supplierEnvelopeTestInput()
	absent.RelayInfo.SupplierCostSnapshot = types.SupplierCostSnapshot{}
	require.Equal(t, types.SupplierAccountingDispositionUnbound, BuildSupplierAccountingEnvelopeV1(absent).Disposition)

	partialBindings := []types.SupplierCostSnapshot{
		{SupplierId: 12, ContractId: 13, RateVersionId: 14},
		{BindingVersionId: 11, ContractId: 13, RateVersionId: 14},
		{BindingVersionId: 11, SupplierId: 12, RateVersionId: 14},
		{BindingVersionId: 11, SupplierId: 12, ContractId: 13},
		{BindingVersionId: -1, SupplierId: 12, ContractId: 13, RateVersionId: 14},
	}
	for _, binding := range partialBindings {
		input := supplierEnvelopeTestInput()
		input.RelayInfo.SupplierCostSnapshot = binding
		envelope := BuildSupplierAccountingEnvelopeV1(input)
		require.Equal(t, types.SupplierAccountingDispositionProducerError, envelope.Disposition, "%+v", binding)
		require.Nil(t, envelope.Captured)
	}
}

func TestSupplierAccountingEnvelopePayloadCeilings(t *testing.T) {
	businessInput := supplierEnvelopeTestInput()
	businessInput.Capture.AudioPricingApplied = true
	businessInput.Capture.ToolPricingApplied = true
	businessInput.Capture.ImagePricingApplied = true
	business := BuildSupplierAccountingEnvelopeV1(businessInput)
	requireEnvelopePayloadAtMost(t, business, 384)

	internalInput := businessInput
	internalInput.RelayInfo = supplierEnvelopeTestRelayInfo()
	internalInput.RelayInfo.SupplierStatisticsScopeSnapshot = types.SupplierStatisticsScopeSnapshot{
		Scope: types.SupplierStatisticsScopeInternal, ExclusionRuleId: math.MaxInt32,
	}
	internal := BuildSupplierAccountingEnvelopeV1(internalInput)
	requireEnvelopePayloadAtMost(t, internal, 320)

	disposition := newSupplierAccountingDispositionEnvelope(types.SupplierAccountingDispositionNotFinanciallyCommitted)
	requireEnvelopePayloadAtMost(t, disposition, 160)
}

func requireEnvelopePayloadAtMost(t *testing.T, envelope types.SupplierAccountingEnvelopeV1, maximum int) {
	t.Helper()
	payload, err := common.Marshal(map[string]any{types.SupplierAccountingEnvelopeKeyV1: envelope})
	require.NoError(t, err)
	require.LessOrEqual(t, len(payload), maximum, string(payload))
}

func TestSupplierProcurementAllowsAboveOneMultiplier(t *testing.T) {
	official := int64(2_000_000)
	cost, err := SupplierProcurementMicro(&official, 1_250_000)
	require.NoError(t, err)
	require.EqualValues(t, 2_500_000, *cost)
}

func TestSupplierProcurementMicroHalfUpAndOverflow(t *testing.T) {
	official := int64(1)
	cost, err := SupplierProcurementMicro(&official, 500_000)
	require.NoError(t, err)
	require.EqualValues(t, 1, *cost)

	maxOfficial := int64(math.MaxInt64)
	_, err = SupplierProcurementMicro(&maxOfficial, 1_000_001)
	require.Error(t, err)
}
