package service

import (
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestSupplierAccountingCompactRatioUsesRefreshedOfficialModelRatio(t *testing.T) {
	info := supplierAccountingCompactRelayInfo()
	info.PriceData.ModelRatio = 99
	info.SupplierOfficialPricingSnapshot.PriceData = types.PriceData{
		ModelRatio: 2.5, CompletionRatio: 1,
	}
	summary := textQuotaSummary{PromptTokens: 1_000, TotalTokens: 1_000}
	official, known, reason := calculateTextOfficialListUSD(info, summary, nil)
	require.True(t, known)
	require.Empty(t, reason)
	require.True(t, official.Equal(decimal.RequireFromString("0.005")))

	envelope := BuildSupplierAccountingEnvelopeV1(SupplierAccountingEnvelopeInputV1{
		RelayInfo: info,
		Settlement: types.BillingSettlementResult{
			FinanciallyCommitted: true, FinanciallyCommittedAt: 1_784_801_200, FinalSalesQuota: 1_000_000,
		},
		HasPositiveFinalUsage: true,
		Capture: SupplierAccountingCaptureInputV1{
			OfficialListUSD: &official,
			PricingMode:     supplierAccountingOfficialPricingModeV1(info),
		},
	})
	require.Equal(t, types.SupplierAccountingDispositionCaptured, envelope.Disposition)
	require.NotNil(t, envelope.Captured.PricingProvenance.Ratio)
	require.EqualValues(t, 2_500_000, envelope.Captured.PricingProvenance.Ratio.ModelRatioPpm)
	require.EqualValues(t, 666_667, envelope.Captured.PricingProvenance.Ratio.GroupRatioPpm)
	require.EqualValues(t, 666_667, *envelope.Captured.SalesMultiplierPpm)
}

func TestSupplierAccountingCompactFixedAmountAndProvenanceUseRefreshedOfficialSnapshot(t *testing.T) {
	info := supplierAccountingCompactRelayInfo()
	info.PriceData.UsePrice = false
	info.PriceData.ModelRatio = 99
	info.SupplierOfficialPricingSnapshot.PriceData = types.PriceData{UsePrice: true, ModelPrice: 2.25}

	official, known, reason := calculateTextOfficialListUSD(info, textQuotaSummary{}, nil)
	require.True(t, known)
	require.Empty(t, reason)
	require.True(t, official.Equal(decimal.RequireFromString("2.25")))
	envelope := supplierAccountingCompactEnvelope(info, official, nil)

	require.Equal(t, types.SupplierAccountingDispositionCaptured, envelope.Disposition)
	require.Nil(t, envelope.Captured.PricingProvenance.Ratio)
	require.NotNil(t, envelope.Captured.PricingProvenance.Fixed)
	require.Nil(t, envelope.Captured.PricingProvenance.Tiered)
	require.EqualValues(t, 2_250_000, *envelope.Captured.OfficialListMicroUsd)
	require.EqualValues(t, 666_667, envelope.Captured.PricingProvenance.Fixed.GroupMultiplierPpm)
	require.EqualValues(t, 666_667, *envelope.Captured.SalesMultiplierPpm)
}

func TestSupplierAccountingCompactTieredAmountAndProvenanceUseRefreshedOfficialSnapshot(t *testing.T) {
	info := supplierAccountingCompactRelayInfo()
	info.PriceData.UsePrice = true
	info.PriceData.ModelPrice = 99
	expression := `tier("base", p * 2 + c * 4)`
	info.SupplierOfficialPricingSnapshot.TieredBillingSnapshot = &billingexpr.BillingSnapshot{
		ExprString:   expression,
		ExprHash:     billingexpr.ExprHashString(expression),
		ExprVersion:  1,
		QuotaPerUnit: 500_000,
		GroupRatio:   0.1,
	}
	params := billingexpr.TokenParams{P: 1_000_000}
	summary := textQuotaSummary{SupplierTieredParams: &params, TotalTokens: 1_000_000}
	official, known, reason := calculateTextOfficialListUSD(info, summary, nil)
	require.True(t, known)
	require.Empty(t, reason)
	require.True(t, official.Equal(decimal.RequireFromString("2")))
	envelope := supplierAccountingCompactEnvelope(info, official, &params)

	require.Equal(t, types.SupplierAccountingDispositionCaptured, envelope.Disposition)
	require.Nil(t, envelope.Captured.PricingProvenance.Ratio)
	require.Nil(t, envelope.Captured.PricingProvenance.Fixed)
	require.NotNil(t, envelope.Captured.PricingProvenance.Tiered)
	require.EqualValues(t, 2_000_000, *envelope.Captured.OfficialListMicroUsd)
	require.EqualValues(t, 666_667, envelope.Captured.PricingProvenance.Tiered.GroupMultiplierPpm)
	require.EqualValues(t, 666_667, *envelope.Captured.SalesMultiplierPpm)
	require.EqualValues(t, 1_000_000, envelope.Captured.PricingProvenance.Tiered.NormalizedInputs.Prompt)
}

func TestSupplierAccountingMultiplierPPMRoundsExistingHighPrecisionRatiosHalfUp(t *testing.T) {
	tests := []struct {
		name       string
		multiplier float64
		want       int64
	}{
		{name: "below half", multiplier: 0.6666664, want: 666_666},
		{name: "at half", multiplier: 0.6666665, want: 666_667},
		{name: "above half", multiplier: 0.6666667, want: 666_667},
		{name: "sub ppm half", multiplier: 0.0000005, want: 1},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actual, err := supplierSalesMultiplierPpm(testCase.multiplier)
			require.NoError(t, err)
			require.Equal(t, testCase.want, *actual)
		})
	}
}

func supplierAccountingCompactRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		PriceData: types.PriceData{
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.6666667},
		},
		SupplierCostSnapshot: types.SupplierCostSnapshot{
			BindingVersionId: 11, SupplierId: 12, ContractId: 13, RateVersionId: 14, ProcurementMultiplierPpm: 650_000,
		},
		SupplierStatisticsScopeSnapshot: types.BusinessSupplierStatisticsScopeSnapshot(),
		SupplierOfficialPricingSnapshot: types.SupplierOfficialPricingSnapshot{
			CaptureAttempted: true,
			Loaded:           true,
			QuotaPerUnit:     "500000",
		},
	}
}

func supplierAccountingCompactEnvelope(info *relaycommon.RelayInfo, officialListUSD decimal.Decimal, tieredParams *billingexpr.TokenParams) types.SupplierAccountingEnvelopeV1 {
	return BuildSupplierAccountingEnvelopeV1(SupplierAccountingEnvelopeInputV1{
		RelayInfo: info,
		Settlement: types.BillingSettlementResult{
			FinanciallyCommitted: true, FinanciallyCommittedAt: 1_784_801_200, FinalSalesQuota: 1_000_000,
		},
		HasPositiveFinalUsage: true,
		Capture: SupplierAccountingCaptureInputV1{
			OfficialListUSD:   &officialListUSD,
			PricingMode:       supplierAccountingOfficialPricingModeV1(info),
			TieredTokenParams: tieredParams,
		},
	})
}
