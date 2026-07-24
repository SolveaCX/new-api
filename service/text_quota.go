package service

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type textQuotaSummary struct {
	PromptTokens               int
	CompletionTokens           int
	TotalTokens                int
	CacheTokens                int
	CacheCreationTokens        int
	CacheCreationTokens5m      int
	CacheCreationTokens1h      int
	ImageTokens                int
	AudioTokens                int
	ModelName                  string
	TokenName                  string
	UseTimeSeconds             int64
	CompletionRatio            float64
	CacheRatio                 float64
	ImageRatio                 float64
	ModelRatio                 float64
	GroupRatio                 float64
	ModelPrice                 float64
	CacheCreationRatio         float64
	CacheCreationRatio5m       float64
	CacheCreationRatio1h       float64
	Quota                      int
	IsClaudeUsageSemantic      bool
	UsageSemantic              string
	WebSearchPrice             float64
	WebSearchCallCount         int
	ClaudeWebSearchPrice       float64
	ClaudeWebSearchCallCount   int
	FileSearchPrice            float64
	FileSearchCallCount        int
	AudioInputPrice            float64
	ImageGenerationCallPrice   float64
	ImageGenerationCallApplied bool
	ToolCallSurchargeQuota     decimal.Decimal
	OfficialToolSurchargeUSD   decimal.Decimal
	OfficialAudioInputUSD      decimal.Decimal
	OfficialListUSD            decimal.Decimal
	OfficialListUSDKnown       bool
	OfficialEvidenceReason     string
	UsedHeuristicCacheTokens   bool
	UsageWasEstimated          bool
	LegacyClaudeDerived        bool
	PricingMode                string
	SupplierTieredParams       *billingexpr.TokenParams
}

func supplierTextHasPositiveFinalUsage(summary textQuotaSummary, settlement types.BillingSettlementResult) bool {
	if !settlement.FinanciallyCommitted {
		return false
	}
	return summary.TotalTokens > 0 || settlement.FinalSalesQuota > 0 ||
		summary.WebSearchCallCount > 0 || summary.ClaudeWebSearchCallCount > 0 ||
		summary.FileSearchCallCount > 0 || summary.ImageGenerationCallApplied
}

func cacheWriteTokensTotal(summary textQuotaSummary) int {
	if summary.CacheCreationTokens5m > 0 || summary.CacheCreationTokens1h > 0 {
		splitCacheWriteTokens := summary.CacheCreationTokens5m + summary.CacheCreationTokens1h
		if summary.CacheCreationTokens > splitCacheWriteTokens {
			return summary.CacheCreationTokens
		}
		return splitCacheWriteTokens
	}
	return summary.CacheCreationTokens
}

func isLegacyClaudeDerivedOpenAIUsage(relayInfo *relaycommon.RelayInfo, usage *dto.Usage) bool {
	if relayInfo == nil || usage == nil {
		return false
	}
	if relayInfo.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		return false
	}
	if usage.UsageSource != "" || usage.UsageSemantic != "" {
		return false
	}
	return usage.ClaudeCacheCreation5mTokens > 0 || usage.ClaudeCacheCreation1hTokens > 0
}

func calculateTextToolCallSurcharge(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, summary *textQuotaSummary) decimal.Decimal {
	dGroupRatio := decimal.NewFromFloat(summary.GroupRatio)
	dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
	officialPricing := relayInfo.SupplierOfficialPricingSnapshot

	var surcharge decimal.Decimal

	if relayInfo.ResponsesUsageInfo != nil {
		if webSearchTool, exists := relayInfo.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview]; exists && webSearchTool.CallCount > 0 {
			summary.WebSearchCallCount = webSearchTool.CallCount
			summary.WebSearchPrice = operation_setting.GetToolPriceForModel("web_search_preview", summary.ModelName)
			summary.OfficialToolSurchargeUSD = summary.OfficialToolSurchargeUSD.Add(decimal.NewFromFloat(officialPricing.WebSearchPreviewPricePerThousandCalls).
				Mul(decimal.NewFromInt(int64(webSearchTool.CallCount))).
				Div(decimal.NewFromInt(1000)))
			markMissingOfficialPricingSnapshot(summary, officialPricing.Loaded)
			surcharge = surcharge.Add(decimal.NewFromFloat(summary.WebSearchPrice).
				Mul(decimal.NewFromInt(int64(webSearchTool.CallCount))).
				Div(decimal.NewFromInt(1000)).
				Mul(dGroupRatio).
				Mul(dQuotaPerUnit))
		}
	} else if strings.HasSuffix(summary.ModelName, "search-preview") {
		summary.WebSearchCallCount = 1
		summary.WebSearchPrice = operation_setting.GetToolPriceForModel("web_search_preview", summary.ModelName)
		summary.OfficialToolSurchargeUSD = summary.OfficialToolSurchargeUSD.Add(decimal.NewFromFloat(officialPricing.WebSearchPreviewPricePerThousandCalls).
			Div(decimal.NewFromInt(1000)))
		markMissingOfficialPricingSnapshot(summary, officialPricing.Loaded)
		surcharge = surcharge.Add(decimal.NewFromFloat(summary.WebSearchPrice).
			Div(decimal.NewFromInt(1000)).
			Mul(dGroupRatio).
			Mul(dQuotaPerUnit))
	}

	summary.ClaudeWebSearchCallCount = ctx.GetInt("claude_web_search_requests")
	if summary.ClaudeWebSearchCallCount > 0 {
		summary.ClaudeWebSearchPrice = operation_setting.GetToolPrice("web_search")
		summary.OfficialToolSurchargeUSD = summary.OfficialToolSurchargeUSD.Add(decimal.NewFromFloat(officialPricing.ClaudeWebSearchPricePerThousandCalls).
			Div(decimal.NewFromInt(1000)).
			Mul(decimal.NewFromInt(int64(summary.ClaudeWebSearchCallCount))))
		markMissingOfficialPricingSnapshot(summary, officialPricing.Loaded)
		surcharge = surcharge.Add(decimal.NewFromFloat(summary.ClaudeWebSearchPrice).
			Div(decimal.NewFromInt(1000)).
			Mul(dGroupRatio).
			Mul(dQuotaPerUnit).
			Mul(decimal.NewFromInt(int64(summary.ClaudeWebSearchCallCount))))
	}

	if relayInfo.ResponsesUsageInfo != nil {
		if fileSearchTool, exists := relayInfo.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolFileSearch]; exists && fileSearchTool.CallCount > 0 {
			summary.FileSearchCallCount = fileSearchTool.CallCount
			summary.FileSearchPrice = operation_setting.GetToolPrice("file_search")
			summary.OfficialToolSurchargeUSD = summary.OfficialToolSurchargeUSD.Add(decimal.NewFromFloat(officialPricing.FileSearchPricePerThousandCalls).
				Mul(decimal.NewFromInt(int64(fileSearchTool.CallCount))).
				Div(decimal.NewFromInt(1000)))
			markMissingOfficialPricingSnapshot(summary, officialPricing.Loaded)
			surcharge = surcharge.Add(decimal.NewFromFloat(summary.FileSearchPrice).
				Mul(decimal.NewFromInt(int64(fileSearchTool.CallCount))).
				Div(decimal.NewFromInt(1000)).
				Mul(dGroupRatio).
				Mul(dQuotaPerUnit))
		}
	}

	if ctx.GetBool("image_generation_call") {
		summary.ImageGenerationCallApplied = true
		quality := ctx.GetString("image_generation_call_quality")
		size := ctx.GetString("image_generation_call_size")
		summary.ImageGenerationCallPrice = operation_setting.GetGPTImage1PriceOnceCall(quality, size)
		officialImagePrice := officialPricing.ImageGenerationCallPrices.Price(quality, size)
		summary.OfficialToolSurchargeUSD = summary.OfficialToolSurchargeUSD.Add(decimal.NewFromFloat(officialImagePrice))
		markMissingOfficialPricingSnapshot(summary, officialPricing.Loaded)
		surcharge = surcharge.Add(decimal.NewFromFloat(summary.ImageGenerationCallPrice).
			Mul(dGroupRatio).
			Mul(dQuotaPerUnit))
	}

	return surcharge
}

func markMissingOfficialPricingSnapshot(summary *textQuotaSummary, loaded bool) {
	if !loaded && summary.OfficialEvidenceReason == "" {
		summary.OfficialEvidenceReason = "supplier_accounting.official_pricing_snapshot.missing"
	}
}

func composeTieredTextQuota(relayInfo *relaycommon.RelayInfo, summary *textQuotaSummary, tieredQuota int, tieredResult *billingexpr.TieredResult) int {
	summary.PricingMode = "tiered_expr"
	summary.OfficialListUSD, summary.OfficialListUSDKnown, summary.OfficialEvidenceReason = calculateTextOfficialListUSD(relayInfo, *summary, tieredResult)

	if summary.ToolCallSurchargeQuota.IsZero() {
		return tieredQuota
	}

	if tieredResult != nil {
		if snap := relayInfo.TieredBillingSnapshot; snap != nil {
			return int(decimal.NewFromFloat(tieredResult.ActualQuotaBeforeGroup).
				Mul(decimal.NewFromFloat(snap.GroupRatio)).
				Add(summary.ToolCallSurchargeQuota).
				Round(0).
				IntPart())
		}
	}

	return tieredQuota + int(summary.ToolCallSurchargeQuota.Round(0).IntPart())
}

func calculateTextQuotaSummary(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage) textQuotaSummary {
	summary := textQuotaSummary{
		ModelName:            relayInfo.OriginModelName,
		TokenName:            ctx.GetString("token_name"),
		UseTimeSeconds:       time.Now().Unix() - relayInfo.StartTime.Unix(),
		CompletionRatio:      relayInfo.PriceData.CompletionRatio,
		CacheRatio:           relayInfo.PriceData.CacheRatio,
		ImageRatio:           relayInfo.PriceData.ImageRatio,
		ModelRatio:           relayInfo.PriceData.ModelRatio,
		GroupRatio:           relayInfo.PriceData.GroupRatioInfo.GroupRatio,
		ModelPrice:           relayInfo.PriceData.ModelPrice,
		CacheCreationRatio:   relayInfo.PriceData.CacheCreationRatio,
		CacheCreationRatio5m: relayInfo.PriceData.CacheCreation5mRatio,
		CacheCreationRatio1h: relayInfo.PriceData.CacheCreation1hRatio,
		UsageSemantic:        usageSemanticFromUsage(relayInfo, usage),
		PricingMode:          "ratio",
	}
	if relayInfo.PriceData.UsePrice {
		summary.PricingMode = "price"
	}
	summary.IsClaudeUsageSemantic = summary.UsageSemantic == "anthropic"

	if usage == nil {
		summary.UsageWasEstimated = true
		usage = &dto.Usage{
			PromptTokens:     relayInfo.GetEstimatePromptTokens(),
			CompletionTokens: 0,
			TotalTokens:      relayInfo.GetEstimatePromptTokens(),
		}
	}
	if common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens) {
		summary.UsageWasEstimated = true
	}
	if supplierTieredSnapshot := relayInfo.SupplierOfficialPricingSnapshot.TieredBillingSnapshot; supplierTieredSnapshot != nil {
		params := BuildTieredTokenParams(usage, summary.IsClaudeUsageSemantic, billingexpr.UsedVars(supplierTieredSnapshot.ExprString))
		summary.SupplierTieredParams = &params
	}

	summary.PromptTokens = usage.PromptTokens
	summary.CompletionTokens = usage.CompletionTokens
	summary.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	summary.CacheTokens = usage.PromptTokensDetails.CachedTokens
	summary.CacheCreationTokens = usage.PromptTokensDetails.CacheCreationTokensTotal()
	summary.CacheCreationTokens5m = usage.ClaudeCacheCreation5mTokens
	summary.CacheCreationTokens1h = usage.ClaudeCacheCreation1hTokens
	summary.ImageTokens = usage.PromptTokensDetails.ImageTokens
	summary.AudioTokens = usage.PromptTokensDetails.AudioTokens
	legacyClaudeDerived := isLegacyClaudeDerivedOpenAIUsage(relayInfo, usage)
	summary.LegacyClaudeDerived = legacyClaudeDerived
	isOpenRouterClaudeBilling := relayInfo.ChannelMeta != nil &&
		relayInfo.ChannelType == constant.ChannelTypeOpenRouter &&
		summary.IsClaudeUsageSemantic

	if isOpenRouterClaudeBilling {
		summary.PromptTokens -= summary.CacheTokens
		isUsingCustomSettings := relayInfo.PriceData.UsePrice || hasCustomModelRatio(summary.ModelName, relayInfo.PriceData.ModelRatio)
		if summary.CacheCreationTokens == 0 && relayInfo.PriceData.CacheCreationRatio != 1 && usage.Cost != 0 && !isUsingCustomSettings {
			maybeCacheCreationTokens := CalcOpenRouterCacheCreateTokens(*usage, relayInfo.PriceData)
			if maybeCacheCreationTokens >= 0 && summary.PromptTokens >= maybeCacheCreationTokens {
				summary.CacheCreationTokens = maybeCacheCreationTokens
				summary.UsedHeuristicCacheTokens = true
			}
		}
		summary.PromptTokens -= summary.CacheCreationTokens
	}

	dPromptTokens := decimal.NewFromInt(int64(summary.PromptTokens))
	dCacheTokens := decimal.NewFromInt(int64(summary.CacheTokens))
	dImageTokens := decimal.NewFromInt(int64(summary.ImageTokens))
	dAudioTokens := decimal.NewFromInt(int64(summary.AudioTokens))
	dCompletionTokens := decimal.NewFromInt(int64(summary.CompletionTokens))
	dCachedCreationTokens := decimal.NewFromInt(int64(summary.CacheCreationTokens))
	dCompletionRatio := decimal.NewFromFloat(summary.CompletionRatio)
	dCacheRatio := decimal.NewFromFloat(summary.CacheRatio)
	dImageRatio := decimal.NewFromFloat(summary.ImageRatio)
	dModelRatio := decimal.NewFromFloat(summary.ModelRatio)
	dGroupRatio := decimal.NewFromFloat(summary.GroupRatio)
	dModelPrice := decimal.NewFromFloat(summary.ModelPrice)
	dCacheCreationRatio := decimal.NewFromFloat(summary.CacheCreationRatio)
	dCacheCreationRatio5m := decimal.NewFromFloat(summary.CacheCreationRatio5m)
	dCacheCreationRatio1h := decimal.NewFromFloat(summary.CacheCreationRatio1h)
	dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)

	ratio := dModelRatio.Mul(dGroupRatio)
	summary.ToolCallSurchargeQuota = calculateTextToolCallSurcharge(ctx, relayInfo, &summary)

	var audioInputQuota decimal.Decimal
	salesAudioInputPrice := operation_setting.GetGeminiInputAudioPricePerMillionTokens(summary.ModelName)
	var officialAudioInputPrice float64
	if !dAudioTokens.IsZero() {
		officialAudioInputPrice = relayInfo.SupplierOfficialPricingSnapshot.GeminiInputAudioPricePerMillionTokens
		if officialAudioInputPrice > 0 {
			summary.OfficialAudioInputUSD = decimal.NewFromFloat(officialAudioInputPrice).
				Div(decimal.NewFromInt(1000000)).Mul(dAudioTokens)
		}
	}
	if !relayInfo.PriceData.UsePrice {
		baseTokens := dPromptTokens

		var cachedTokensWithRatio decimal.Decimal
		if !dCacheTokens.IsZero() {
			if !summary.IsClaudeUsageSemantic && !legacyClaudeDerived {
				baseTokens = baseTokens.Sub(dCacheTokens)
			}
			cachedTokensWithRatio = dCacheTokens.Mul(dCacheRatio)
		}

		var cachedCreationTokensWithRatio decimal.Decimal
		hasSplitCacheCreationTokens := summary.CacheCreationTokens5m > 0 || summary.CacheCreationTokens1h > 0
		if !dCachedCreationTokens.IsZero() || hasSplitCacheCreationTokens {
			if !summary.IsClaudeUsageSemantic && !legacyClaudeDerived {
				baseTokens = baseTokens.Sub(dCachedCreationTokens)
				cachedCreationTokensWithRatio = dCachedCreationTokens.Mul(dCacheCreationRatio)
			} else {
				remaining := summary.CacheCreationTokens - summary.CacheCreationTokens5m - summary.CacheCreationTokens1h
				if remaining < 0 {
					remaining = 0
				}
				cachedCreationTokensWithRatio = decimal.NewFromInt(int64(remaining)).Mul(dCacheCreationRatio)
				cachedCreationTokensWithRatio = cachedCreationTokensWithRatio.Add(decimal.NewFromInt(int64(summary.CacheCreationTokens5m)).Mul(dCacheCreationRatio5m))
				cachedCreationTokensWithRatio = cachedCreationTokensWithRatio.Add(decimal.NewFromInt(int64(summary.CacheCreationTokens1h)).Mul(dCacheCreationRatio1h))
			}
		}

		var imageTokensWithRatio decimal.Decimal
		if !dImageTokens.IsZero() {
			baseTokens = baseTokens.Sub(dImageTokens)
			imageTokensWithRatio = dImageTokens.Mul(dImageRatio)
		}

		if !dAudioTokens.IsZero() {
			if salesAudioInputPrice > 0 {
				summary.AudioInputPrice = salesAudioInputPrice
				baseTokens = baseTokens.Sub(dAudioTokens)
				audioInputQuota = decimal.NewFromFloat(salesAudioInputPrice).
					Div(decimal.NewFromInt(1000000)).Mul(dAudioTokens).Mul(dGroupRatio).Mul(dQuotaPerUnit)
			}
		}

		// OpenAI reports cache reads and writes as unadjusted prefix counts, so
		// their sum can exceed prompt_tokens. Overlap must not create a negative
		// uncached remainder.
		if baseTokens.IsNegative() {
			baseTokens = decimal.Zero
		}

		promptQuota := baseTokens.Add(cachedTokensWithRatio).Add(imageTokensWithRatio).Add(cachedCreationTokensWithRatio)
		completionQuota := dCompletionTokens.Mul(dCompletionRatio)
		quotaCalculateDecimal := promptQuota.Add(completionQuota).Mul(ratio)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(summary.ToolCallSurchargeQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(audioInputQuota)
		if len(relayInfo.PriceData.OtherRatios) > 0 {
			for _, otherRatio := range relayInfo.PriceData.OtherRatios {
				quotaCalculateDecimal = quotaCalculateDecimal.Mul(decimal.NewFromFloat(otherRatio))
			}
		}

		if !ratio.IsZero() && quotaCalculateDecimal.LessThanOrEqual(decimal.Zero) {
			quotaCalculateDecimal = decimal.NewFromInt(1)
		}
		summary.Quota = int(quotaCalculateDecimal.Round(0).IntPart())
	} else {
		quotaCalculateDecimal := dModelPrice.Mul(dQuotaPerUnit).Mul(dGroupRatio)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(summary.ToolCallSurchargeQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(audioInputQuota)
		if len(relayInfo.PriceData.OtherRatios) > 0 {
			for _, otherRatio := range relayInfo.PriceData.OtherRatios {
				quotaCalculateDecimal = quotaCalculateDecimal.Mul(decimal.NewFromFloat(otherRatio))
			}
		}
		summary.Quota = int(quotaCalculateDecimal.Round(0).IntPart())
	}

	if summary.TotalTokens == 0 {
		summary.Quota = 0
	} else if !ratio.IsZero() && summary.Quota == 0 {
		summary.Quota = 1
	}
	if relayInfo.TieredBillingSnapshot == nil {
		summary.OfficialListUSD, summary.OfficialListUSDKnown, summary.OfficialEvidenceReason = calculateTextOfficialListUSD(relayInfo, summary, nil)
	}

	return summary
}

func calculateTextOfficialListUSD(relayInfo *relaycommon.RelayInfo, summary textQuotaSummary, tieredResult *billingexpr.TieredResult) (decimal.Decimal, bool, string) {
	if relayInfo == nil || !relayInfo.SupplierOfficialPricingSnapshot.Loaded {
		return decimal.Zero, false, "supplier_accounting.official_pricing_snapshot.missing"
	}
	pricing := relayInfo.SupplierOfficialPricingSnapshot

	reasons := make([]string, 0, 3)
	if summary.OfficialEvidenceReason != "" {
		reasons = append(reasons, summary.OfficialEvidenceReason)
	}
	if summary.UsageWasEstimated {
		reasons = append(reasons, "supplier_accounting.usage.local_estimate")
	}
	if summary.UsedHeuristicCacheTokens {
		reasons = append(reasons, "supplier_accounting.cache_creation_tokens.heuristic")
	}

	if supplierTieredSnapshot := pricing.TieredBillingSnapshot; supplierTieredSnapshot != nil {
		if summary.SupplierTieredParams == nil {
			return decimal.Zero, false, strings.Join(append(reasons, "supplier_accounting.tiered_settlement.params_missing"), ";")
		}
		supplierTieredResult, err := calculateSupplierTieredResult(relayInfo, *summary.SupplierTieredParams)
		if err != nil || math.IsNaN(supplierTieredResult.ActualQuotaBeforeGroup) || math.IsInf(supplierTieredResult.ActualQuotaBeforeGroup, 0) {
			return decimal.Zero, false, strings.Join(append(reasons, "supplier_accounting.tiered_settlement.fallback"), ";")
		}
		quotaPerUnit, err := decimal.NewFromString(pricing.QuotaPerUnit)
		if err != nil || quotaPerUnit.LessThanOrEqual(decimal.Zero) {
			return decimal.Zero, false, strings.Join(append(reasons, "supplier_accounting.quota_per_unit_snapshot.invalid_divisor"), ";")
		}
		amount := decimal.NewFromFloat(supplierTieredResult.ActualQuotaBeforeGroup).
			Div(quotaPerUnit).
			Add(summary.OfficialToolSurchargeUSD)
		return amount, true, strings.Join(reasons, ";")
	}

	if relayInfo.TieredBillingSnapshot != nil {
		return decimal.Zero, false, strings.Join(append(reasons, "supplier_accounting.tiered_pricing_snapshot.missing"), ";")
	}

	priceData := pricing.PriceData
	var amount decimal.Decimal
	if priceData.UsePrice {
		amount = decimal.NewFromFloat(priceData.ModelPrice).
			Add(summary.OfficialToolSurchargeUSD).
			Add(summary.OfficialAudioInputUSD)
	} else {
		quotaPerUnit, err := decimal.NewFromString(pricing.QuotaPerUnit)
		if err != nil || quotaPerUnit.LessThanOrEqual(decimal.Zero) {
			return decimal.Zero, false, strings.Join(append(reasons, "supplier_accounting.quota_per_unit_snapshot.invalid_divisor"), ";")
		}
		baseTokens := decimal.NewFromInt(int64(summary.PromptTokens))
		cacheTokens := decimal.NewFromInt(int64(summary.CacheTokens))
		cacheCreationTokens := decimal.NewFromInt(int64(summary.CacheCreationTokens))
		imageTokens := decimal.NewFromInt(int64(summary.ImageTokens))
		audioTokens := decimal.NewFromInt(int64(summary.AudioTokens))

		weightedCache := decimal.Zero
		if !cacheTokens.IsZero() {
			if !summary.IsClaudeUsageSemantic && !summary.LegacyClaudeDerived {
				baseTokens = baseTokens.Sub(cacheTokens)
			}
			weightedCache = cacheTokens.Mul(decimal.NewFromFloat(priceData.CacheRatio))
		}

		weightedCacheCreation := decimal.Zero
		hasSplitCacheCreation := summary.CacheCreationTokens5m > 0 || summary.CacheCreationTokens1h > 0
		if !cacheCreationTokens.IsZero() || hasSplitCacheCreation {
			if !summary.IsClaudeUsageSemantic && !summary.LegacyClaudeDerived {
				baseTokens = baseTokens.Sub(cacheCreationTokens)
				weightedCacheCreation = cacheCreationTokens.Mul(decimal.NewFromFloat(priceData.CacheCreationRatio))
			} else {
				remaining := summary.CacheCreationTokens - summary.CacheCreationTokens5m - summary.CacheCreationTokens1h
				if remaining < 0 {
					remaining = 0
				}
				weightedCacheCreation = decimal.NewFromInt(int64(remaining)).Mul(decimal.NewFromFloat(priceData.CacheCreationRatio))
				weightedCacheCreation = weightedCacheCreation.Add(decimal.NewFromInt(int64(summary.CacheCreationTokens5m)).Mul(decimal.NewFromFloat(priceData.CacheCreation5mRatio)))
				weightedCacheCreation = weightedCacheCreation.Add(decimal.NewFromInt(int64(summary.CacheCreationTokens1h)).Mul(decimal.NewFromFloat(priceData.CacheCreation1hRatio)))
			}
		}

		weightedImage := decimal.Zero
		if !imageTokens.IsZero() {
			baseTokens = baseTokens.Sub(imageTokens)
			weightedImage = imageTokens.Mul(decimal.NewFromFloat(priceData.ImageRatio))
		}
		if !audioTokens.IsZero() && pricing.GeminiInputAudioPricePerMillionTokens > 0 {
			baseTokens = baseTokens.Sub(audioTokens)
		}

		promptWeighted := baseTokens.Add(weightedCache).Add(weightedCacheCreation).Add(weightedImage)
		completionWeighted := decimal.NewFromInt(int64(summary.CompletionTokens)).Mul(decimal.NewFromFloat(priceData.CompletionRatio))
		amount = promptWeighted.Add(completionWeighted).
			Mul(decimal.NewFromFloat(priceData.ModelRatio)).
			Div(quotaPerUnit).
			Add(summary.OfficialToolSurchargeUSD).
			Add(summary.OfficialAudioInputUSD)
	}

	// OtherRatios are request-local multiplicities (for example requested or
	// returned image count), not mutable unit-price configuration. They may be
	// learned from the upstream response, so apply the final request-local values
	// to the unit prices frozen before the upstream call.
	for _, otherRatio := range relayInfo.PriceData.OtherRatios {
		amount = amount.Mul(decimal.NewFromFloat(otherRatio))
	}
	return amount, true, strings.Join(reasons, ";")
}

func usageSemanticFromUsage(relayInfo *relaycommon.RelayInfo, usage *dto.Usage) string {
	if usage != nil && usage.UsageSemantic != "" {
		return usage.UsageSemantic
	}
	if relayInfo != nil && relayInfo.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		return "anthropic"
	}
	return "openai"
}

func PostTextConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, extraContent []string) {
	originUsage := usage
	if usage == nil {
		extraContent = append(extraContent, "上游无计费信息")
	}
	if originUsage != nil {
		ObserveChannelAffinityUsageCacheByRelayFormat(ctx, usage, relayInfo.GetFinalRequestRelayFormat())
	}

	adminRejectReason := common.GetContextKeyString(ctx, constant.ContextKeyAdminRejectReason)
	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)

	var tieredResult *billingexpr.TieredResult
	tieredBillingApplied := false
	if originUsage != nil {
		var tieredUsedVars map[string]bool
		if snap := relayInfo.TieredBillingSnapshot; snap != nil {
			tieredUsedVars = billingexpr.UsedVars(snap.ExprString)
		}
		tieredOk, tieredQuota, tieredRes := TryTieredSettle(relayInfo, BuildTieredTokenParams(usage, summary.IsClaudeUsageSemantic, tieredUsedVars))
		if tieredOk {
			tieredBillingApplied = true
			tieredResult = tieredRes
			summary.Quota = composeTieredTextQuota(relayInfo, &summary, tieredQuota, tieredRes)
		}
	}

	if summary.WebSearchCallCount > 0 {
		extraContent = append(extraContent, fmt.Sprintf("Web Search 调用 %d 次，调用花费 %s", summary.WebSearchCallCount, decimal.NewFromFloat(summary.WebSearchPrice).Mul(decimal.NewFromInt(int64(summary.WebSearchCallCount))).Div(decimal.NewFromInt(1000)).Mul(decimal.NewFromFloat(summary.GroupRatio)).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).String()))
	}
	if summary.ClaudeWebSearchCallCount > 0 {
		extraContent = append(extraContent, fmt.Sprintf("Claude Web Search 调用 %d 次，调用花费 %s", summary.ClaudeWebSearchCallCount, decimal.NewFromFloat(summary.ClaudeWebSearchPrice).Div(decimal.NewFromInt(1000)).Mul(decimal.NewFromFloat(summary.GroupRatio)).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Mul(decimal.NewFromInt(int64(summary.ClaudeWebSearchCallCount))).String()))
	}
	if summary.FileSearchCallCount > 0 {
		extraContent = append(extraContent, fmt.Sprintf("File Search 调用 %d 次，调用花费 %s", summary.FileSearchCallCount, decimal.NewFromFloat(summary.FileSearchPrice).Mul(decimal.NewFromInt(int64(summary.FileSearchCallCount))).Div(decimal.NewFromInt(1000)).Mul(decimal.NewFromFloat(summary.GroupRatio)).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).String()))
	}
	if summary.AudioInputPrice > 0 && summary.AudioTokens > 0 {
		extraContent = append(extraContent, fmt.Sprintf("Audio Input 花费 %s", decimal.NewFromFloat(summary.AudioInputPrice).Div(decimal.NewFromInt(1000000)).Mul(decimal.NewFromInt(int64(summary.AudioTokens))).Mul(decimal.NewFromFloat(summary.GroupRatio)).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).String()))
	}
	if summary.ImageGenerationCallPrice > 0 {
		extraContent = append(extraContent, fmt.Sprintf("Image Generation Call 花费 %s", decimal.NewFromFloat(summary.ImageGenerationCallPrice).Mul(decimal.NewFromFloat(summary.GroupRatio)).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).String()))
	}

	if summary.TotalTokens == 0 {
		extraContent = append(extraContent, "上游没有返回计费信息，无法扣费（可能是上游超时）")
		logger.LogError(ctx, fmt.Sprintf("total tokens is 0, cannot consume quota, userId %d, channelId %d, tokenId %d, model %s， pre-consumed quota %d", relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, summary.ModelName, relayInfo.FinalPreConsumedQuota))
	} else {
		model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, summary.Quota)
		model.UpdateChannelUsedQuota(relayInfo.ChannelId, summary.Quota)
	}

	settlement := SettleBillingResult(ctx, relayInfo, summary.Quota)
	if settlement.Err != nil {
		logger.LogError(ctx, "error settling billing: "+settlement.Err.Error())
	}
	var officialListUSD *decimal.Decimal
	if summary.OfficialListUSDKnown {
		frozenOfficialListUSD := summary.OfficialListUSD
		officialListUSD = &frozenOfficialListUSD
	}
	logModel := summary.ModelName
	if strings.HasPrefix(logModel, "gpt-4-gizmo") {
		logModel = "gpt-4-gizmo-*"
		extraContent = append(extraContent, fmt.Sprintf("模型 %s", summary.ModelName))
	}
	if strings.HasPrefix(logModel, "gpt-4o-gizmo") {
		logModel = "gpt-4o-gizmo-*"
		extraContent = append(extraContent, fmt.Sprintf("模型 %s", summary.ModelName))
	}

	logContent := strings.Join(extraContent, ", ")
	var other map[string]interface{}
	if summary.IsClaudeUsageSemantic {
		other = GenerateClaudeOtherInfo(ctx, relayInfo,
			summary.ModelRatio, summary.GroupRatio, summary.CompletionRatio,
			summary.CacheTokens, summary.CacheRatio,
			summary.CacheCreationTokens, summary.CacheCreationRatio,
			summary.CacheCreationTokens5m, summary.CacheCreationRatio5m,
			summary.CacheCreationTokens1h, summary.CacheCreationRatio1h,
			summary.ModelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)
		other["usage_semantic"] = "anthropic"
	} else {
		other = GenerateTextOtherInfo(ctx, relayInfo, summary.ModelRatio, summary.GroupRatio, summary.CompletionRatio, summary.CacheTokens, summary.CacheRatio, summary.ModelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)
	}
	if adminRejectReason != "" {
		other["reject_reason"] = adminRejectReason
	}
	if summary.ImageTokens != 0 {
		other["image"] = true
		other["image_ratio"] = summary.ImageRatio
		other["image_output"] = summary.ImageTokens
	}
	if summary.WebSearchCallCount > 0 {
		other["web_search"] = true
		other["web_search_call_count"] = summary.WebSearchCallCount
		other["web_search_price"] = summary.WebSearchPrice
	} else if summary.ClaudeWebSearchCallCount > 0 {
		other["web_search"] = true
		other["web_search_call_count"] = summary.ClaudeWebSearchCallCount
		other["web_search_price"] = summary.ClaudeWebSearchPrice
	}
	if summary.FileSearchCallCount > 0 {
		other["file_search"] = true
		other["file_search_call_count"] = summary.FileSearchCallCount
		other["file_search_price"] = summary.FileSearchPrice
	}
	if summary.AudioInputPrice > 0 && summary.AudioTokens > 0 {
		other["audio_input_seperate_price"] = true
		other["audio_input_token_count"] = summary.AudioTokens
		other["audio_input_price"] = summary.AudioInputPrice
	}
	if summary.ImageGenerationCallPrice > 0 {
		other["image_generation_call"] = true
		other["image_generation_call_price"] = summary.ImageGenerationCallPrice
	}
	if summary.CacheCreationTokens > 0 {
		other["cache_creation_tokens"] = summary.CacheCreationTokens
		other["cache_creation_ratio"] = summary.CacheCreationRatio
	}
	if summary.CacheCreationTokens5m > 0 {
		other["cache_creation_tokens_5m"] = summary.CacheCreationTokens5m
		other["cache_creation_ratio_5m"] = summary.CacheCreationRatio5m
	}
	if summary.CacheCreationTokens1h > 0 {
		other["cache_creation_tokens_1h"] = summary.CacheCreationTokens1h
		other["cache_creation_ratio_1h"] = summary.CacheCreationRatio1h
	}
	cacheWriteTokens := cacheWriteTokensTotal(summary)
	if cacheWriteTokens > 0 {
		// cache_write_tokens: normalized cache creation total for UI display.
		// If split 5m/1h values are present, this is their sum; otherwise it falls back
		// to cache_creation_tokens.
		other["cache_write_tokens"] = cacheWriteTokens
	}
	if relayInfo.GetFinalRequestRelayFormat() != types.RelayFormatClaude && usage != nil && usage.UsageSource != "" && usage.InputTokens > 0 {
		// input_tokens_total: explicit normalized total input used by the usage log UI.
		// Only write this field when upstream/current conversion has already provided a
		// reliable total input value and tagged the usage source. Do not infer it from
		// prompt/cache fields here, otherwise old upstream payloads may be double-counted.
		other["input_tokens_total"] = usage.InputTokens
	}
	if tieredBillingApplied {
		InjectTieredBillingInfo(other, relayInfo, tieredResult)
	}
	unknownOfficialAmountCount := uint32(0)
	if !summary.OfficialListUSDKnown {
		unknownOfficialAmountCount = 1
	}
	supplierPricingMode := supplierAccountingOfficialPricingModeV1(relayInfo)
	audioPricingApplied := summary.AudioTokens > 0 && (summary.AudioInputPrice > 0 || relayInfo.SupplierOfficialPricingSnapshot.GeminiInputAudioPricePerMillionTokens > 0)
	imagePricingApplied := summary.ImageGenerationCallApplied || (summary.ImageTokens > 0 && supplierPricingMode != "fixed")
	if supplierPricingMode == "tiered_expr" && summary.SupplierTieredParams != nil && relayInfo.SupplierOfficialPricingSnapshot.TieredBillingSnapshot != nil {
		usedVars := billingexpr.UsedVars(relayInfo.SupplierOfficialPricingSnapshot.TieredBillingSnapshot.ExprString)
		audioPricingApplied = (summary.SupplierTieredParams.AI > 0 && usedVars["ai"]) ||
			(summary.SupplierTieredParams.AO > 0 && usedVars["ao"])
		imagePricingApplied = summary.ImageGenerationCallApplied ||
			(summary.SupplierTieredParams.Img > 0 && usedVars["img"]) ||
			(summary.SupplierTieredParams.ImgO > 0 && usedVars["img_o"])
	}
	toolPricingApplied := summary.WebSearchCallCount > 0 || summary.ClaudeWebSearchCallCount > 0 || summary.FileSearchCallCount > 0
	InjectSupplierAccountingEnvelopeV1(other, SupplierAccountingEnvelopeInputV1{
		RelayInfo:             relayInfo,
		Settlement:            settlement,
		HasPositiveFinalUsage: supplierTextHasPositiveFinalUsage(summary, settlement),
		Capture: SupplierAccountingCaptureInputV1{
			OfficialListUSD:            officialListUSD,
			OfficialEvidenceReason:     summary.OfficialEvidenceReason,
			PricingMode:                supplierPricingMode,
			TieredTokenParams:          summary.SupplierTieredParams,
			AudioPricingApplied:        audioPricingApplied,
			ToolPricingApplied:         toolPricingApplied,
			ImagePricingApplied:        imagePricingApplied,
			UnknownOfficialAmountCount: unknownOfficialAmountCount,
		},
	})

	model.RecordConsumeLog(ctx, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     summary.PromptTokens,
		CompletionTokens: summary.CompletionTokens,
		ModelName:        logModel,
		TokenName:        summary.TokenName,
		Quota:            summary.Quota,
		Content:          logContent,
		TokenId:          relayInfo.TokenId,
		UseTimeSeconds:   int(summary.UseTimeSeconds),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
	})
	perfmetrics.RecordChannelTokens(relayInfo, int64(summary.PromptTokens), int64(summary.CompletionTokens))
	perfmetrics.RecordRelaySample(relayInfo, true, int64(summary.CompletionTokens), nil)
}
