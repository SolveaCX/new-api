package service

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type supplierAccountingProviderMatrixCase struct {
	provider                 string
	model                    string
	scope                    types.SupplierStatisticsScope
	pricingMode              types.SupplierPricingModeV1
	dimension                string
	groupMultiplierPpm       int64
	modelRatioPpm            int64
	expectedOfficialMicroUSD int64
}

type supplierAccountingProviderMatrixFixture struct {
	input       SupplierAccountingEnvelopeInputV1
	textSummary *textQuotaSummary
	audioInput  TokenDetails
	audioOutput TokenDetails
}

func TestSupplierAccountingProviderNamedFieldMatrix(t *testing.T) {
	testCases := []supplierAccountingProviderMatrixCase{
		{
			provider: "Claude", model: "claude-sonnet-4-20250514",
			scope: types.SupplierStatisticsScopeBusiness, pricingMode: types.SupplierPricingModeRatio,
			dimension: "tool", groupMultiplierPpm: 700_000, modelRatioPpm: 3_000_000,
			expectedOfficialMicroUSD: 50_490,
		},
		{
			provider: "Claude", model: "claude-opus-4-20250514",
			scope: types.SupplierStatisticsScopeInternal, pricingMode: types.SupplierPricingModeFixed,
			dimension: "", groupMultiplierPpm: 650_000, expectedOfficialMicroUSD: 2_500_000,
		},
		{
			provider: "GPT", model: "gpt-image-1",
			scope: types.SupplierStatisticsScopeBusiness, pricingMode: types.SupplierPricingModeFixed,
			dimension: "image", groupMultiplierPpm: 670_000, expectedOfficialMicroUSD: 2_667_000,
		},
		{
			provider: "GPT", model: "gpt-4o-audio-preview",
			scope: types.SupplierStatisticsScopeInternal, pricingMode: types.SupplierPricingModeRatio,
			dimension: "audio", groupMultiplierPpm: 720_000, modelRatioPpm: 2_500_000,
			expectedOfficialMicroUSD: 9_250,
		},
		{
			provider: "Gemini", model: "gemini-2.5-pro",
			scope: types.SupplierStatisticsScopeBusiness, pricingMode: types.SupplierPricingModeTiered,
			dimension: "audio", groupMultiplierPpm: 680_000, expectedOfficialMicroUSD: 29_500,
		},
		{
			provider: "Gemini", model: "gemini-2.5-flash-image",
			scope: types.SupplierStatisticsScopeInternal, pricingMode: types.SupplierPricingModeRatio,
			dimension: "image", groupMultiplierPpm: 690_000, modelRatioPpm: 1_250_000,
			expectedOfficialMicroUSD: 46_000,
		},
		{
			provider: "DeepSeek", model: "deepseek-chat",
			scope: types.SupplierStatisticsScopeBusiness, pricingMode: types.SupplierPricingModeRatio,
			dimension: "", groupMultiplierPpm: 710_000, modelRatioPpm: 550_000,
			expectedOfficialMicroUSD: 57_640,
		},
		{
			provider: "DeepSeek", model: "deepseek-reasoner",
			scope: types.SupplierStatisticsScopeInternal, pricingMode: types.SupplierPricingModeTiered,
			dimension: "", groupMultiplierPpm: 660_000, expectedOfficialMicroUSD: 112_000,
		},
	}

	seenProviders := make(map[string]bool, 4)
	seenScopeModes := make(map[string]bool, 6)
	seenDimensions := make(map[string]bool, 3)
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(fmt.Sprintf("%s/%s/%s/%s", testCase.provider, testCase.scope, testCase.pricingMode, testCase.model), func(t *testing.T) {
			fixture := supplierAccountingProviderMatrixProductionFixture(t, testCase)
			input := fixture.input
			require.Equal(t, supplierAccountingProviderMatrixExpectedPricingMode(testCase.pricingMode), input.Capture.PricingMode)
			require.Equal(t, input.Capture.PricingMode, supplierAccountingOfficialPricingModeV1(input.RelayInfo))
			requireSupplierAccountingProviderDimensionSemantics(t, testCase, fixture)

			envelope := BuildSupplierAccountingEnvelopeV1(input)
			require.Equal(t, types.SupplierAccountingDispositionCaptured, envelope.Disposition)
			require.NoError(t, ValidateSupplierAccountingEnvelopeV1(envelope))
			require.NotNil(t, envelope.Captured)
			require.NotNil(t, envelope.Captured.PricingProvenance)
			requireSupplierAccountingProviderPricingUnion(t, testCase, input, envelope.Captured.PricingProvenance)
			requireSupplierAccountingProviderDimensions(t, testCase.dimension, envelope.Captured.PricingProvenance.Dimensions)
			expectedOfficialMicroUSD, err := SupplierOfficialUSDToMicro(input.Capture.OfficialListUSD)
			require.NoError(t, err)
			require.NotNil(t, expectedOfficialMicroUSD)
			require.Positive(t, *expectedOfficialMicroUSD)
			require.Equal(t, testCase.expectedOfficialMicroUSD, *expectedOfficialMicroUSD)
			require.Equal(t, *expectedOfficialMicroUSD, *envelope.Captured.OfficialListMicroUsd)

			payload, err := common.Marshal(envelope)
			require.NoError(t, err)
			require.NotContains(t, string(payload), testCase.model)
			require.NotContains(t, string(payload), testCase.provider+" upstream supplier")
			require.NotContains(t, string(payload), testCase.provider+" procurement contract")

			if testCase.scope == types.SupplierStatisticsScopeBusiness {
				require.Equal(t, "included", envelope.Captured.ExclusionDecision)
				require.Nil(t, envelope.Captured.ExclusionRuleId)
				require.NotNil(t, envelope.Captured.SalesMultiplierPpm)
				require.Equal(t, testCase.groupMultiplierPpm, *envelope.Captured.SalesMultiplierPpm)
				require.NotNil(t, envelope.Captured.SalesMicroUsd)
				require.NotNil(t, envelope.Captured.ProcurementCostMicroUsd)
				require.NotNil(t, envelope.Captured.GrossProfitMicroUsd)
				require.Equal(t, *envelope.Captured.SalesMicroUsd-*envelope.Captured.ProcurementCostMicroUsd, *envelope.Captured.GrossProfitMicroUsd)
				requireSupplierAccountingProviderBusinessAggregation(t, testCase, *envelope.Captured)
			} else {
				require.Equal(t, "excluded", envelope.Captured.ExclusionDecision)
				require.NotNil(t, envelope.Captured.ExclusionRuleId)
				require.Positive(t, *envelope.Captured.ExclusionRuleId)
				require.Nil(t, envelope.Captured.SalesMultiplierPpm)
				require.Nil(t, envelope.Captured.SalesMicroUsd)
				require.Nil(t, envelope.Captured.GrossProfitMicroUsd)
				requireSupplierAccountingProviderInternalAggregation(t, testCase, *envelope.Captured)
			}
		})
		seenProviders[testCase.provider] = true
		seenScopeModes[string(testCase.scope)+"/"+string(testCase.pricingMode)] = true
		if testCase.dimension != "" {
			seenDimensions[testCase.dimension] = true
		}
	}

	require.Equal(t, map[string]bool{"Claude": true, "GPT": true, "Gemini": true, "DeepSeek": true}, seenProviders)
	for _, scope := range []types.SupplierStatisticsScope{types.SupplierStatisticsScopeBusiness, types.SupplierStatisticsScopeInternal} {
		for _, mode := range []types.SupplierPricingModeV1{types.SupplierPricingModeRatio, types.SupplierPricingModeFixed, types.SupplierPricingModeTiered} {
			require.True(t, seenScopeModes[string(scope)+"/"+string(mode)], "missing %s/%s provider matrix row", scope, mode)
		}
	}
	require.Equal(t, map[string]bool{"audio": true, "tool": true, "image": true}, seenDimensions)
}

func TestSupplierAccountingProviderMatrixRejectsForbiddenCrossModeAndDimensionFields(t *testing.T) {
	ratioInput := supplierAccountingProviderMatrixProductionFixture(t, supplierAccountingProviderMatrixCase{
		provider: "Claude", model: "claude-sonnet-4-20250514",
		scope: types.SupplierStatisticsScopeBusiness, pricingMode: types.SupplierPricingModeRatio,
		dimension: "tool", groupMultiplierPpm: 700_000, modelRatioPpm: 3_000_000,
	}).input
	ratio := BuildSupplierAccountingEnvelopeV1(ratioInput)
	fixedInput := supplierAccountingProviderMatrixProductionFixture(t, supplierAccountingProviderMatrixCase{
		provider: "GPT", model: "gpt-image-1",
		scope: types.SupplierStatisticsScopeBusiness, pricingMode: types.SupplierPricingModeFixed,
		dimension: "image", groupMultiplierPpm: 670_000,
	}).input
	fixed := BuildSupplierAccountingEnvelopeV1(fixedInput)
	tieredInput := supplierAccountingProviderMatrixProductionFixture(t, supplierAccountingProviderMatrixCase{
		provider: "Gemini", model: "gemini-2.5-pro",
		scope: types.SupplierStatisticsScopeBusiness, pricingMode: types.SupplierPricingModeTiered,
		dimension: "audio", groupMultiplierPpm: 680_000,
	}).input
	tiered := BuildSupplierAccountingEnvelopeV1(tieredInput)
	deepSeekInternalInput := supplierAccountingProviderMatrixProductionFixture(t, supplierAccountingProviderMatrixCase{
		provider: "DeepSeek", model: "deepseek-reasoner",
		scope: types.SupplierStatisticsScopeInternal, pricingMode: types.SupplierPricingModeTiered,
		groupMultiplierPpm: 660_000,
	}).input
	deepSeekInternal := BuildSupplierAccountingEnvelopeV1(deepSeekInternalInput)
	for _, envelope := range []types.SupplierAccountingEnvelopeV1{ratio, fixed, tiered, deepSeekInternal} {
		require.Equal(t, types.SupplierAccountingDispositionCaptured, envelope.Disposition)
		require.NoError(t, ValidateSupplierAccountingEnvelopeV1(envelope))
	}

	testCases := []struct {
		name   string
		base   types.SupplierAccountingEnvelopeV1
		mutate func(*types.SupplierAccountingLogSnapshotV1)
	}{
		{
			name: "Claude ratio forbids fixed provenance", base: ratio,
			mutate: func(snapshot *types.SupplierAccountingLogSnapshotV1) {
				snapshot.PricingProvenance.Fixed = fixed.Captured.PricingProvenance.Fixed
			},
		},
		{
			name: "GPT fixed forbids tiered provenance", base: fixed,
			mutate: func(snapshot *types.SupplierAccountingLogSnapshotV1) {
				snapshot.PricingProvenance.Tiered = tiered.Captured.PricingProvenance.Tiered
			},
		},
		{
			name: "Gemini tiered forbids ratio provenance", base: tiered,
			mutate: func(snapshot *types.SupplierAccountingLogSnapshotV1) {
				snapshot.PricingProvenance.Ratio = ratio.Captured.PricingProvenance.Ratio
			},
		},
		{
			name: "DeepSeek internal forbids business sales", base: deepSeekInternal,
			mutate: func(snapshot *types.SupplierAccountingLogSnapshotV1) {
				value := int64(1)
				snapshot.SalesMultiplierPpm = &value
				snapshot.SalesMicroUsd = &value
				snapshot.GrossProfitMicroUsd = &value
			},
		},
		{
			name: "dimension marker with no affected dimension is forbidden", base: ratio,
			mutate: func(snapshot *types.SupplierAccountingLogSnapshotV1) {
				snapshot.PricingProvenance.Dimensions = &types.SupplierPricingDimensionsV1{}
			},
		},
		{
			name: "business sales multiplier must match final successful group multiplier", base: fixed,
			mutate: func(snapshot *types.SupplierAccountingLogSnapshotV1) {
				wrong := *snapshot.SalesMultiplierPpm + 1
				snapshot.SalesMultiplierPpm = &wrong
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			broken := supplierAccountingProviderMatrixCloneEnvelope(testCase.base)
			testCase.mutate(broken.Captured)
			require.Error(t, ValidateSupplierAccountingEnvelopeV1(broken))
		})
	}
}

func TestSupplierAccountingProviderMatrixRejectsStaleAuthoritativePricingModeClaims(t *testing.T) {
	testCases := []struct {
		name     string
		fixture  supplierAccountingProviderMatrixCase
		makeMode func(*types.SupplierOfficialPricingSnapshot)
	}{
		{
			name: "Claude ratio claim cannot override frozen fixed mode",
			fixture: supplierAccountingProviderMatrixCase{
				provider: "Claude", model: "claude-sonnet-4-20250514",
				scope: types.SupplierStatisticsScopeBusiness, pricingMode: types.SupplierPricingModeRatio,
				dimension: "tool", groupMultiplierPpm: 700_000, modelRatioPpm: 3_000_000,
			},
			makeMode: func(pricing *types.SupplierOfficialPricingSnapshot) {
				pricing.PriceData.UsePrice = true
				pricing.PriceData.ModelPrice = 2.5
			},
		},
		{
			name: "GPT fixed claim cannot override frozen ratio mode",
			fixture: supplierAccountingProviderMatrixCase{
				provider: "GPT", model: "gpt-image-1",
				scope: types.SupplierStatisticsScopeBusiness, pricingMode: types.SupplierPricingModeFixed,
				dimension: "image", groupMultiplierPpm: 670_000,
			},
			makeMode: func(pricing *types.SupplierOfficialPricingSnapshot) {
				pricing.PriceData.UsePrice = false
				pricing.PriceData.ModelRatio = 2.5
			},
		},
		{
			name: "DeepSeek tiered claim requires frozen expression snapshot",
			fixture: supplierAccountingProviderMatrixCase{
				provider: "DeepSeek", model: "deepseek-reasoner",
				scope: types.SupplierStatisticsScopeInternal, pricingMode: types.SupplierPricingModeTiered,
				groupMultiplierPpm: 660_000,
			},
			makeMode: func(pricing *types.SupplierOfficialPricingSnapshot) {
				pricing.TieredBillingSnapshot = nil
				pricing.PriceData.UsePrice = false
				pricing.PriceData.ModelRatio = 0.55
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			input := supplierAccountingProviderMatrixProductionFixture(t, testCase.fixture).input
			testCase.makeMode(&input.RelayInfo.SupplierOfficialPricingSnapshot)
			envelope := BuildSupplierAccountingEnvelopeV1(input)
			require.Equal(t, types.SupplierAccountingDispositionProducerError, envelope.Disposition)
			require.Nil(t, envelope.Captured)
		})
	}
}

func supplierAccountingProviderMatrixProductionFixture(t *testing.T, testCase supplierAccountingProviderMatrixCase) supplierAccountingProviderMatrixFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	groupMultiplier := float64(testCase.groupMultiplierPpm) / 1_000_000
	priceData := types.PriceData{
		ModelPrice:           2.5,
		ModelRatio:           float64(testCase.modelRatioPpm) / 1_000_000,
		CompletionRatio:      4,
		CacheRatio:           0.1,
		CacheCreationRatio:   1.25,
		CacheCreation5mRatio: 1.25,
		CacheCreation1hRatio: 2,
		ImageRatio:           1.5,
		AudioRatio:           2,
		AudioCompletionRatio: 2.5,
		UsePrice:             testCase.pricingMode == types.SupplierPricingModeFixed,
		GroupRatioInfo:       types.GroupRatioInfo{GroupRatio: groupMultiplier},
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: testCase.model,
		StartTime:       time.Now(),
		PriceData:       priceData,
		SupplierCostSnapshot: types.SupplierCostSnapshot{
			BindingVersionId:         101,
			SupplierId:               102,
			SupplierName:             testCase.provider + " upstream supplier",
			ContractId:               103,
			ContractName:             testCase.provider + " procurement contract",
			RateVersionId:            104,
			ProcurementMultiplierPpm: 650_000,
		},
		SupplierStatisticsScopeSnapshot: types.BusinessSupplierStatisticsScopeSnapshot(),
		SupplierOfficialPricingSnapshot: types.SupplierOfficialPricingSnapshot{
			Loaded:                                true,
			QuotaPerUnit:                          "500000",
			PriceData:                             priceData,
			WebSearchPreviewPricePerThousandCalls: 10,
			ClaudeWebSearchPricePerThousandCalls:  10,
			FileSearchPricePerThousandCalls:       2.5,
			GeminiInputAudioPricePerMillionTokens: 3.5,
			ImageGenerationCallPrices: types.SupplierImageGenerationCallPrices{
				High1024x1024: 0.167,
			},
		},
	}
	if testCase.scope == types.SupplierStatisticsScopeInternal {
		info.SupplierStatisticsScopeSnapshot = types.SupplierStatisticsScopeSnapshot{
			Scope: types.SupplierStatisticsScopeInternal, ExclusionRuleId: 9001,
		}
	}
	if testCase.pricingMode == types.SupplierPricingModeTiered {
		expression := supplierAccountingProviderMatrixTieredExpression(testCase)
		snapshot := &billingexpr.BillingSnapshot{
			BillingMode:   "tiered_expr",
			ModelName:     testCase.model,
			ExprString:    expression,
			ExprHash:      billingexpr.ExprHashString(expression),
			GroupRatio:    groupMultiplier,
			QuotaPerUnit:  common.QuotaPerUnit,
			ExprVersion:   1,
			EstimatedTier: "base",
		}
		info.TieredBillingSnapshot = snapshot
		info.SupplierOfficialPricingSnapshot.TieredBillingSnapshot = snapshot
	}

	if testCase.provider == "GPT" && testCase.dimension == "audio" {
		return supplierAccountingProviderMatrixAudioFixture(t, testCase, info)
	}
	return supplierAccountingProviderMatrixTextFixture(t, testCase, info)
}

func supplierAccountingProviderMatrixTextFixture(t *testing.T, testCase supplierAccountingProviderMatrixCase, info *relaycommon.RelayInfo) supplierAccountingProviderMatrixFixture {
	t.Helper()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	if testCase.provider == "Claude" && testCase.dimension == "tool" {
		ctx.Set("claude_web_search_requests", 3)
	}
	if testCase.provider == "GPT" && testCase.dimension == "image" {
		ctx.Set("image_generation_call", true)
		ctx.Set("image_generation_call_quality", "high")
		ctx.Set("image_generation_call_size", "1024x1024")
	}
	usage := supplierAccountingProviderMatrixUsage(testCase)
	summary := calculateTextQuotaSummary(ctx, info, &usage)
	if testCase.pricingMode == types.SupplierPricingModeTiered {
		require.NotNil(t, summary.SupplierTieredParams)
		ok, quota, tieredResult := TryTieredSettle(info, *summary.SupplierTieredParams)
		require.True(t, ok)
		require.NotNil(t, tieredResult)
		summary.Quota = composeTieredTextQuota(info, &summary, quota, tieredResult)
	}
	require.True(t, summary.OfficialListUSDKnown, summary.OfficialEvidenceReason)
	require.True(t, summary.OfficialListUSD.GreaterThan(decimal.Zero))
	require.Positive(t, summary.Quota)

	pricingMode := supplierAccountingOfficialPricingModeV1(info)
	audioPricingApplied := summary.AudioTokens > 0 && (summary.AudioInputPrice > 0 || info.SupplierOfficialPricingSnapshot.GeminiInputAudioPricePerMillionTokens > 0)
	imagePricingApplied := summary.ImageGenerationCallApplied || (summary.ImageTokens > 0 && pricingMode != "fixed")
	if pricingMode == "tiered_expr" && summary.SupplierTieredParams != nil && info.SupplierOfficialPricingSnapshot.TieredBillingSnapshot != nil {
		usedVars := billingexpr.UsedVars(info.SupplierOfficialPricingSnapshot.TieredBillingSnapshot.ExprString)
		audioPricingApplied = (summary.SupplierTieredParams.AI > 0 && usedVars["ai"]) ||
			(summary.SupplierTieredParams.AO > 0 && usedVars["ao"])
		imagePricingApplied = summary.ImageGenerationCallApplied ||
			(summary.SupplierTieredParams.Img > 0 && usedVars["img"]) ||
			(summary.SupplierTieredParams.ImgO > 0 && usedVars["img_o"])
	}
	toolPricingApplied := summary.WebSearchCallCount > 0 || summary.ClaudeWebSearchCallCount > 0 || summary.FileSearchCallCount > 0
	officialListUSD := summary.OfficialListUSD
	settlement := types.BillingSettlementResult{
		FinanciallyCommitted: true, FinanciallyCommittedAt: 1_784_887_200, FinalSalesQuota: summary.Quota,
	}
	return supplierAccountingProviderMatrixFixture{
		input: SupplierAccountingEnvelopeInputV1{
			RelayInfo:             info,
			Settlement:            settlement,
			HasPositiveFinalUsage: supplierTextHasPositiveFinalUsage(summary, settlement),
			Capture: SupplierAccountingCaptureInputV1{
				OfficialListUSD:     &officialListUSD,
				PricingMode:         pricingMode,
				TieredTokenParams:   summary.SupplierTieredParams,
				AudioPricingApplied: audioPricingApplied,
				ToolPricingApplied:  toolPricingApplied,
				ImagePricingApplied: imagePricingApplied,
			},
		},
		textSummary: &summary,
	}
}

func supplierAccountingProviderMatrixAudioFixture(t *testing.T, testCase supplierAccountingProviderMatrixCase, info *relaycommon.RelayInfo) supplierAccountingProviderMatrixFixture {
	t.Helper()
	inputDetails := TokenDetails{TextTokens: 800, AudioTokens: 200}
	outputDetails := TokenDetails{TextTokens: 100, AudioTokens: 50}
	officialListUSD, officialKnown, reason, pricingMode := calculateSupplierAudioOfficialListUSD(info, inputDetails, outputDetails, nil)
	require.True(t, officialKnown, reason)
	require.True(t, officialListUSD.GreaterThan(decimal.Zero))
	finalSalesQuota := calculateAudioQuota(QuotaInfo{
		InputDetails: inputDetails, OutputDetails: outputDetails, ModelName: testCase.model,
		UsePrice: info.PriceData.UsePrice, ModelPrice: info.PriceData.ModelPrice,
		ModelRatio: info.PriceData.ModelRatio, GroupRatio: info.PriceData.GroupRatioInfo.GroupRatio,
	})
	require.Positive(t, finalSalesQuota)
	settlement := types.BillingSettlementResult{
		FinanciallyCommitted: true, FinanciallyCommittedAt: 1_784_887_200, FinalSalesQuota: finalSalesQuota,
	}
	return supplierAccountingProviderMatrixFixture{
		input: SupplierAccountingEnvelopeInputV1{
			RelayInfo:             info,
			Settlement:            settlement,
			HasPositiveFinalUsage: supplierAudioHasPositiveFinalUsage(1_150, settlement),
			Capture: SupplierAccountingCaptureInputV1{
				OfficialListUSD:     &officialListUSD,
				PricingMode:         pricingMode,
				AudioPricingApplied: pricingMode != "price" && (inputDetails.AudioTokens > 0 || outputDetails.AudioTokens > 0),
			},
		},
		audioInput:  inputDetails,
		audioOutput: outputDetails,
	}
}

func supplierAccountingProviderMatrixUsage(testCase supplierAccountingProviderMatrixCase) dto.Usage {
	usage := dto.Usage{PromptTokens: 4_000, CompletionTokens: 600, TotalTokens: 4_600, UsageSource: "provider_matrix"}
	switch testCase.provider {
	case "Claude":
		usage.PromptTokens = 2_000
		usage.CompletionTokens = 300
		usage.TotalTokens = 2_300
		usage.UsageSemantic = "anthropic"
		usage.PromptTokensDetails.CachedTokens = 500
		usage.PromptTokensDetails.CachedCreationTokens = 120
		usage.ClaudeCacheCreation5mTokens = 100
		usage.ClaudeCacheCreation1hTokens = 20
	case "GPT":
		usage.PromptTokens = 1_000
		usage.CompletionTokens = 100
		usage.TotalTokens = 1_100
	case "Gemini":
		usage.PromptTokens = 12_000
		usage.CompletionTokens = 1_500
		usage.TotalTokens = 13_500
		if testCase.dimension == "audio" {
			usage.PromptTokensDetails.AudioTokens = 2_000
			usage.CompletionTokenDetails.AudioTokens = 500
		}
		if testCase.dimension == "image" {
			usage.PromptTokensDetails.ImageTokens = 800
		}
	case "DeepSeek":
		usage.PromptTokens = 32_000
		usage.CompletionTokens = 6_000
		usage.TotalTokens = 38_000
		usage.PromptTokensDetails.CachedTokens = 4_000
	}
	return usage
}

func supplierAccountingProviderMatrixTieredExpression(testCase supplierAccountingProviderMatrixCase) string {
	switch testCase.dimension {
	case "audio":
		return `tier("audio", p*1.25+c*5+ai*3.5+ao*10)`
	case "image":
		return `tier("image", p*1.25+c*5+img*2+img_o*8)`
	default:
		return `len<=200000?tier("standard",p*2+c*8):tier("long",p*4+c*12)`
	}
}

func supplierAccountingProviderMatrixExpectedPricingMode(mode types.SupplierPricingModeV1) string {
	if mode == types.SupplierPricingModeTiered {
		return "tiered_expr"
	}
	return string(mode)
}

func requireSupplierAccountingProviderDimensionSemantics(t *testing.T, testCase supplierAccountingProviderMatrixCase, fixture supplierAccountingProviderMatrixFixture) {
	t.Helper()
	input := fixture.input
	switch testCase.dimension {
	case "audio":
		require.True(t, input.Capture.AudioPricingApplied)
		require.False(t, input.Capture.ToolPricingApplied)
		require.False(t, input.Capture.ImagePricingApplied)
		require.Positive(t, input.RelayInfo.SupplierOfficialPricingSnapshot.PriceData.AudioRatio)
		if testCase.provider == "Gemini" {
			require.NotNil(t, fixture.textSummary)
			require.Positive(t, fixture.textSummary.AudioTokens)
			require.Positive(t, input.RelayInfo.SupplierOfficialPricingSnapshot.GeminiInputAudioPricePerMillionTokens)
			require.NotNil(t, input.Capture.TieredTokenParams)
			require.Positive(t, input.Capture.TieredTokenParams.AI)
			require.Positive(t, input.Capture.TieredTokenParams.AO)
		} else {
			require.Positive(t, fixture.audioInput.AudioTokens)
			require.Positive(t, fixture.audioOutput.AudioTokens)
		}
	case "tool":
		require.True(t, input.Capture.ToolPricingApplied)
		require.False(t, input.Capture.AudioPricingApplied)
		require.False(t, input.Capture.ImagePricingApplied)
		require.NotNil(t, fixture.textSummary)
		if testCase.provider == "Claude" {
			require.Positive(t, fixture.textSummary.ClaudeWebSearchCallCount)
			require.True(t, fixture.textSummary.OfficialToolSurchargeUSD.GreaterThan(decimal.Zero))
			require.Positive(t, input.RelayInfo.SupplierOfficialPricingSnapshot.ClaudeWebSearchPricePerThousandCalls)
		}
	case "image":
		require.True(t, input.Capture.ImagePricingApplied)
		require.False(t, input.Capture.AudioPricingApplied)
		require.False(t, input.Capture.ToolPricingApplied)
		require.NotNil(t, fixture.textSummary)
		if testCase.provider == "GPT" {
			require.True(t, fixture.textSummary.ImageGenerationCallApplied)
			require.True(t, fixture.textSummary.OfficialToolSurchargeUSD.GreaterThan(decimal.Zero))
			require.Positive(t, input.RelayInfo.SupplierOfficialPricingSnapshot.ImageGenerationCallPrices.High1024x1024)
		} else {
			require.Positive(t, fixture.textSummary.ImageTokens)
			require.Positive(t, input.RelayInfo.SupplierOfficialPricingSnapshot.PriceData.ImageRatio)
		}
	default:
		require.False(t, input.Capture.AudioPricingApplied)
		require.False(t, input.Capture.ToolPricingApplied)
		require.False(t, input.Capture.ImagePricingApplied)
	}
}

func requireSupplierAccountingProviderPricingUnion(t *testing.T, testCase supplierAccountingProviderMatrixCase, input SupplierAccountingEnvelopeInputV1, provenance *types.SupplierPricingProvenanceV1) {
	t.Helper()
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
	require.Equal(t, 1, memberCount)

	switch testCase.pricingMode {
	case types.SupplierPricingModeRatio:
		require.NotNil(t, provenance.Ratio)
		require.Nil(t, provenance.Fixed)
		require.Nil(t, provenance.Tiered)
		require.Equal(t, testCase.modelRatioPpm, provenance.Ratio.ModelRatioPpm)
		require.Equal(t, float64(testCase.modelRatioPpm)/1_000_000, input.RelayInfo.SupplierOfficialPricingSnapshot.PriceData.ModelRatio)
		require.False(t, input.RelayInfo.SupplierOfficialPricingSnapshot.PriceData.UsePrice)
		expectedGroupMultiplier, expectedGroupVersion := supplierAccountingProviderExpectedGroupEvidence(testCase)
		require.Equal(t, expectedGroupMultiplier, provenance.Ratio.GroupRatioPpm)
		require.Positive(t, provenance.Ratio.ModelRatioVersion)
		require.Equal(t, expectedGroupVersion, provenance.Ratio.GroupRatioVersion)
	case types.SupplierPricingModeFixed:
		require.Nil(t, provenance.Ratio)
		require.NotNil(t, provenance.Fixed)
		require.Nil(t, provenance.Tiered)
		require.Equal(t, "price_data", provenance.Fixed.Source)
		require.Equal(t, "model_price", provenance.Fixed.Key)
		require.True(t, input.RelayInfo.SupplierOfficialPricingSnapshot.PriceData.UsePrice)
		require.Positive(t, input.RelayInfo.SupplierOfficialPricingSnapshot.PriceData.ModelPrice)
		expectedGroupMultiplier, expectedGroupVersion := supplierAccountingProviderExpectedGroupEvidence(testCase)
		require.Equal(t, expectedGroupMultiplier, provenance.Fixed.GroupMultiplierPpm)
		require.Positive(t, provenance.Fixed.PriceVersion)
		require.Equal(t, expectedGroupVersion, provenance.Fixed.GroupRatioVersion)
	case types.SupplierPricingModeTiered:
		require.Nil(t, provenance.Ratio)
		require.Nil(t, provenance.Fixed)
		require.NotNil(t, provenance.Tiered)
		require.NotZero(t, provenance.Tiered.ExpressionFingerprint)
		require.Positive(t, provenance.Tiered.ExpressionVersion)
		expectedGroupMultiplier, expectedGroupVersion := supplierAccountingProviderExpectedGroupEvidence(testCase)
		require.Equal(t, expectedGroupMultiplier, provenance.Tiered.GroupMultiplierPpm)
		require.Equal(t, expectedGroupVersion, provenance.Tiered.GroupRatioVersion)
		require.NotNil(t, input.RelayInfo.SupplierOfficialPricingSnapshot.TieredBillingSnapshot)
		require.NotNil(t, input.Capture.TieredTokenParams)
		params := input.Capture.TieredTokenParams
		require.Equal(t, int64(params.P), provenance.Tiered.NormalizedInputs.Prompt)
		require.Equal(t, int64(params.C), provenance.Tiered.NormalizedInputs.Completion)
		require.Equal(t, int64(params.Len), provenance.Tiered.NormalizedInputs.ContextLength)
		require.Equal(t, int64(params.CR), provenance.Tiered.NormalizedInputs.CacheRead)
		require.Equal(t, int64(params.CC), provenance.Tiered.NormalizedInputs.CacheCreate)
		require.Equal(t, int64(params.CC1h), provenance.Tiered.NormalizedInputs.CacheCreate1H)
		require.Equal(t, int64(params.Img), provenance.Tiered.NormalizedInputs.ImageInput)
		require.Equal(t, int64(params.ImgO), provenance.Tiered.NormalizedInputs.ImageOutput)
		require.Equal(t, int64(params.AI), provenance.Tiered.NormalizedInputs.AudioInput)
		require.Equal(t, int64(params.AO), provenance.Tiered.NormalizedInputs.AudioOutput)
	default:
		t.Fatalf("unexpected pricing mode %q", testCase.pricingMode)
	}
}

func supplierAccountingProviderExpectedGroupEvidence(testCase supplierAccountingProviderMatrixCase) (int64, int64) {
	if testCase.scope == types.SupplierStatisticsScopeInternal {
		return 0, 0
	}
	return testCase.groupMultiplierPpm, 1
}

func requireSupplierAccountingProviderDimensions(t *testing.T, dimension string, actual *types.SupplierPricingDimensionsV1) {
	t.Helper()
	if dimension == "" {
		require.Nil(t, actual)
		return
	}
	require.NotNil(t, actual)
	require.Equal(t, dimension == "audio", actual.Audio)
	require.Equal(t, dimension == "tool", actual.Tool)
	require.Equal(t, dimension == "image", actual.Image)
}

func requireSupplierAccountingProviderBusinessAggregation(t *testing.T, testCase supplierAccountingProviderMatrixCase, snapshot types.SupplierAccountingLogSnapshotV1) {
	t.Helper()
	accumulators := make(map[string]*model.SupplierUsageDailySummary)
	err := addSupplierDailySnapshot(accumulators, "2026-07-23", 1_784_764_800, model.SupplierAccountingLogRow{
		ChannelId: 11, ModelName: testCase.model,
	}, snapshot)
	require.NoError(t, err)
	require.Len(t, accumulators, 1)
	for _, summary := range accumulators {
		require.Equal(t, testCase.model, summary.ModelName)
		require.EqualValues(t, 1, summary.SalesKnownCount)
		require.EqualValues(t, 1, summary.GrossProfitKnownCount)
		require.Equal(t, *snapshot.SalesMicroUsd, summary.SalesMicroUsd)
		require.Equal(t, *snapshot.GrossProfitMicroUsd, summary.GrossProfitMicroUsd)
	}
}

func requireSupplierAccountingProviderInternalAggregation(t *testing.T, testCase supplierAccountingProviderMatrixCase, snapshot types.SupplierAccountingLogSnapshotV1) {
	t.Helper()
	accumulators := make(map[string]*model.SupplierUsageDailySummary)
	err := addSupplierDailySnapshot(accumulators, "2026-07-23", 1_784_764_800, model.SupplierAccountingLogRow{
		ChannelId: 11, ModelName: testCase.model,
	}, snapshot)
	require.NoError(t, err)
	require.Len(t, accumulators, 1)
	for _, summary := range accumulators {
		require.Empty(t, summary.ModelName)
		require.EqualValues(t, 1, summary.OfficialListKnownCount)
		require.EqualValues(t, 1, summary.ProcurementCostKnownCount)
		require.Zero(t, summary.SalesKnownCount)
		require.Zero(t, summary.SalesMicroUsd)
		require.Zero(t, summary.GrossProfitKnownCount)
		require.Zero(t, summary.GrossProfitMicroUsd)
		require.Zero(t, summary.GrossMarginEligibleCount)
		require.Zero(t, summary.GrossMarginEligibleSalesMicroUsd)
	}
}

func supplierAccountingProviderMatrixCloneEnvelope(source types.SupplierAccountingEnvelopeV1) types.SupplierAccountingEnvelopeV1 {
	clone := source
	if source.Captured == nil {
		return clone
	}
	snapshot := *source.Captured
	clone.Captured = &snapshot
	if source.Captured.PricingProvenance == nil {
		return clone
	}
	provenance := *source.Captured.PricingProvenance
	clone.Captured.PricingProvenance = &provenance
	if provenance.Dimensions != nil {
		dimensions := *provenance.Dimensions
		clone.Captured.PricingProvenance.Dimensions = &dimensions
	}
	return clone
}
