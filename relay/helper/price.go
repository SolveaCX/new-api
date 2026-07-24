package helper

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func modelPriceNotConfiguredError(modelName string, userId int) error {
	if model.IsAdmin(userId) {
		return fmt.Errorf(
			"模型 %s 的价格未配置。请前往「系统设置 → 运营设置」开启自用模式，或在「系统设置 → 分组与模型定价设置」中为该模型配置价格；"+
				"Model %s price not configured. Go to System Settings → Operation Settings to enable self-use mode, or configure the model price in System Settings → Group & Model Pricing.",
			modelName, modelName,
		)
	}
	return fmt.Errorf(
		"模型 %s 的价格尚未由管理员配置，暂时无法使用，请联系站点管理员开启该模型；"+
			"Model %s has not been priced by the administrator yet. Please contact the site administrator to enable this model.",
		modelName, modelName,
	)
}

// https://docs.claude.com/en/docs/build-with-claude/prompt-caching#1-hour-cache-duration
const claudeCacheCreation1hMultiplier = 6 / 3.75

// HandleGroupRatio checks for "auto_group" in the context and updates the group ratio and relayInfo.UsingGroup if present
func HandleGroupRatio(ctx *gin.Context, relayInfo *relaycommon.RelayInfo) types.GroupRatioInfo {
	// check auto group
	autoGroup, exists := ctx.Get("auto_group")
	if exists {
		logger.LogDebug(ctx, "final group: %s", autoGroup)
		relayInfo.UsingGroup = autoGroup.(string)
	}

	return ratio_setting.GetEffectiveGroupRatio(relayInfo.UserGroup, relayInfo.UsingGroup, relayInfo.OriginModelName)
}

// ApplyResolvedGroupRatio freezes the group selected for the current relay
// attempt into every settlement snapshot that consumes it. Unit prices and
// tier expressions remain frozen from initial pricing; only the retry-varying
// group multiplier is advanced to the final attempt.
func ApplyResolvedGroupRatio(relayInfo *relaycommon.RelayInfo, groupRatioInfo types.GroupRatioInfo) {
	if relayInfo == nil {
		return
	}
	relayInfo.PriceData.GroupRatioInfo = groupRatioInfo
	applyTieredGroupRatio(relayInfo.TieredBillingSnapshot, groupRatioInfo.GroupRatio)
	if relayInfo.SupplierOfficialPricingSnapshot.Loaded {
		relayInfo.SupplierOfficialPricingSnapshot.PriceData.GroupRatioInfo = groupRatioInfo
		applyTieredGroupRatio(relayInfo.SupplierOfficialPricingSnapshot.TieredBillingSnapshot, groupRatioInfo.GroupRatio)
	}
}

func applyTieredGroupRatio(snapshot *billingexpr.BillingSnapshot, groupRatio float64) {
	if snapshot == nil {
		return
	}
	snapshot.GroupRatio = groupRatio
	snapshot.EstimatedQuotaAfterGroup = billingexpr.QuotaRound(snapshot.EstimatedQuotaBeforeGroup * groupRatio)
}

func ModelPriceHelper(c *gin.Context, info *relaycommon.RelayInfo, promptTokens int, meta *types.TokenCountMeta) (types.PriceData, error) {
	modelPrice, usePrice := ratio_setting.GetModelPrice(info.OriginModelName, false)

	groupRatioInfo := HandleGroupRatio(c, info)

	// Check if this model uses tiered_expr billing
	if billing_setting.GetBillingMode(info.OriginModelName) == billing_setting.BillingModeTieredExpr {
		return modelPriceHelperTiered(c, info, promptTokens, meta, groupRatioInfo)
	}

	var preConsumedQuota int
	var modelRatio float64
	var completionRatio float64
	var cacheRatio float64
	var imageRatio float64
	var cacheCreationRatio float64
	var cacheCreationRatio5m float64
	var cacheCreationRatio1h float64
	var audioRatio float64
	var audioCompletionRatio float64
	var freeModel bool
	if !usePrice {
		preConsumedTokens := common.Max(promptTokens, common.PreConsumedQuota)
		if meta.MaxTokens != 0 {
			preConsumedTokens += meta.MaxTokens
		}
		var success bool
		var matchName string
		modelRatio, success, matchName = ratio_setting.GetModelRatio(info.OriginModelName)
		if !success {
			acceptUnsetRatio := false
			if info.UserSetting.AcceptUnsetRatioModel {
				acceptUnsetRatio = true
			}
			if !acceptUnsetRatio {
				return types.PriceData{}, modelPriceNotConfiguredError(matchName, info.UserId)
			}
		}
		completionRatio = ratio_setting.GetCompletionRatio(info.OriginModelName)
		cacheRatio, _ = ratio_setting.GetCacheRatio(info.OriginModelName)
		cacheCreationRatio, _ = ratio_setting.GetCreateCacheRatio(info.OriginModelName)
		cacheCreationRatio5m = cacheCreationRatio
		// 固定1h和5min缓存写入价格的比例
		cacheCreationRatio1h = cacheCreationRatio * claudeCacheCreation1hMultiplier
		imageRatio, _ = ratio_setting.GetImageRatio(info.OriginModelName)
		audioRatio = ratio_setting.GetAudioRatio(info.OriginModelName)
		audioCompletionRatio = ratio_setting.GetAudioCompletionRatio(info.OriginModelName)
		ratio := modelRatio * groupRatioInfo.GroupRatio
		preConsumedQuota = int(float64(preConsumedTokens) * ratio)
	} else {
		if meta.ImagePriceRatio != 0 {
			modelPrice = modelPrice * meta.ImagePriceRatio
		}
		preConsumedQuota = int(modelPrice * common.QuotaPerUnit * groupRatioInfo.GroupRatio)
	}

	// check if free model pre-consume is disabled
	if !operation_setting.GetQuotaSetting().EnableFreeModelPreConsume {
		// if model price or ratio is 0, do not pre-consume quota
		if groupRatioInfo.GroupRatio == 0 {
			preConsumedQuota = 0
			freeModel = true
		} else if usePrice {
			if modelPrice == 0 {
				preConsumedQuota = 0
				freeModel = true
			}
		} else {
			if modelRatio == 0 {
				preConsumedQuota = 0
				freeModel = true
			}
		}
	}

	priceData := types.PriceData{
		FreeModel:            freeModel,
		ModelPrice:           modelPrice,
		ModelRatio:           modelRatio,
		CompletionRatio:      completionRatio,
		GroupRatioInfo:       groupRatioInfo,
		UsePrice:             usePrice,
		CacheRatio:           cacheRatio,
		ImageRatio:           imageRatio,
		AudioRatio:           audioRatio,
		AudioCompletionRatio: audioCompletionRatio,
		CacheCreationRatio:   cacheCreationRatio,
		CacheCreation5mRatio: cacheCreationRatio5m,
		CacheCreation1hRatio: cacheCreationRatio1h,
		QuotaToPreConsume:    preConsumedQuota,
	}

	if common.DebugEnabled {
		logger.LogDebug(c, "model_price_helper result: %s", priceData.ToSetting())
	}
	info.PriceData = priceData
	FreezeSupplierOfficialPricingSnapshot(info, priceData)
	return priceData, nil
}

// ModelPriceHelperPerCall 按次/按量计费的 PriceHelper (MJ、Task)
func ModelPriceHelperPerCall(c *gin.Context, info *relaycommon.RelayInfo) (types.PriceData, error) {
	groupRatioInfo := HandleGroupRatio(c, info)

	modelPrice, success := ratio_setting.GetModelPrice(info.OriginModelName, true)
	usePrice := success
	var modelRatio float64

	if !success {
		defaultPrice, ok := ratio_setting.GetDefaultModelPriceMap()[info.OriginModelName]
		if ok {
			modelPrice = defaultPrice
			usePrice = true
		} else {
			var ratioSuccess bool
			var matchName string
			modelRatio, ratioSuccess, matchName = ratio_setting.GetModelRatio(info.OriginModelName)
			acceptUnsetRatio := false
			if info.UserSetting.AcceptUnsetRatioModel {
				acceptUnsetRatio = true
			}
			if !ratioSuccess && !acceptUnsetRatio {
				return types.PriceData{}, modelPriceNotConfiguredError(matchName, info.UserId)
			}
		}
	}

	var quota int
	freeModel := false

	if usePrice {
		quota = int(modelPrice * common.QuotaPerUnit * groupRatioInfo.GroupRatio)
		if !operation_setting.GetQuotaSetting().EnableFreeModelPreConsume {
			if groupRatioInfo.GroupRatio == 0 || modelPrice == 0 {
				quota = 0
				freeModel = true
			}
		}
	} else {
		// 按量计费：以模型倍率的一半作为预扣额度
		quota = int(modelRatio / 2 * common.QuotaPerUnit * groupRatioInfo.GroupRatio)
		modelPrice = -1
		if !operation_setting.GetQuotaSetting().EnableFreeModelPreConsume {
			if groupRatioInfo.GroupRatio == 0 || modelRatio == 0 {
				quota = 0
				freeModel = true
			}
		}
	}

	priceData := types.PriceData{
		FreeModel:      freeModel,
		ModelPrice:     modelPrice,
		ModelRatio:     modelRatio,
		UsePrice:       usePrice,
		Quota:          quota,
		GroupRatioInfo: groupRatioInfo,
	}
	FreezeSupplierOfficialPricingSnapshot(info, priceData)
	return priceData, nil
}

func HasModelBillingConfig(modelName string) bool {
	if _, ok := ratio_setting.GetModelPrice(modelName, false); ok {
		return true
	}
	if _, ok, _ := ratio_setting.GetModelRatio(modelName); ok {
		return true
	}
	if billing_setting.GetBillingMode(modelName) != billing_setting.BillingModeTieredExpr {
		return false
	}
	expr, ok := billing_setting.GetBillingExpr(modelName)
	return ok && strings.TrimSpace(expr) != ""
}

func modelPriceHelperTiered(c *gin.Context, info *relaycommon.RelayInfo, promptTokens int, meta *types.TokenCountMeta, groupRatioInfo types.GroupRatioInfo) (types.PriceData, error) {
	exprStr, ok := billing_setting.GetBillingExpr(info.OriginModelName)
	if !ok {
		return types.PriceData{}, fmt.Errorf("model %s is configured as tiered_expr but has no billing expression", info.OriginModelName)
	}

	estimatedCompletionTokens := 0
	if meta.MaxTokens != 0 {
		estimatedCompletionTokens = meta.MaxTokens
	}

	requestInput, err := ResolveIncomingBillingExprRequestInput(c, info)
	if err != nil {
		return types.PriceData{}, err
	}

	rawCost, trace, err := billingexpr.RunExprWithRequest(exprStr, billingexpr.TokenParams{
		P:   float64(promptTokens),
		C:   float64(estimatedCompletionTokens),
		Len: float64(promptTokens),
	}, requestInput)
	if err != nil {
		return types.PriceData{}, fmt.Errorf("model %s tiered expr run failed: %w", info.OriginModelName, err)
	}

	// Expression coefficients are $/1M tokens prices; convert to quota the same way per-call billing does.
	quotaBeforeGroup := rawCost / 1_000_000 * common.QuotaPerUnit
	preConsumedQuota := billingexpr.QuotaRound(quotaBeforeGroup * groupRatioInfo.GroupRatio)

	freeModel := false
	if !operation_setting.GetQuotaSetting().EnableFreeModelPreConsume {
		if groupRatioInfo.GroupRatio == 0 {
			preConsumedQuota = 0
			freeModel = true
		}
	}

	exprHash := billingexpr.ExprHashString(exprStr)
	snapshot := &billingexpr.BillingSnapshot{
		BillingMode:               billing_setting.BillingModeTieredExpr,
		ModelName:                 info.OriginModelName,
		ExprString:                exprStr,
		ExprHash:                  exprHash,
		GroupRatio:                groupRatioInfo.GroupRatio,
		EstimatedPromptTokens:     promptTokens,
		EstimatedCompletionTokens: estimatedCompletionTokens,
		EstimatedQuotaBeforeGroup: quotaBeforeGroup,
		EstimatedQuotaAfterGroup:  preConsumedQuota,
		EstimatedTier:             trace.MatchedTier,
		QuotaPerUnit:              common.QuotaPerUnit,
		ExprVersion:               billingexpr.ExprVersion(exprStr),
	}
	info.TieredBillingSnapshot = snapshot
	info.BillingRequestInput = &requestInput

	priceData := types.PriceData{
		FreeModel:         freeModel,
		GroupRatioInfo:    groupRatioInfo,
		QuotaToPreConsume: preConsumedQuota,
	}

	logger.LogDebug(c, "model_price_helper_tiered result: model=%s preConsume=%d quotaBeforeGroup=%.2f groupRatio=%.2f tier=%s", info.OriginModelName, preConsumedQuota, quotaBeforeGroup, groupRatioInfo.GroupRatio, trace.MatchedTier)

	info.PriceData = priceData
	FreezeSupplierOfficialPricingSnapshot(info, priceData)
	return priceData, nil
}

// FreezeSupplierOfficialPricingSnapshot captures supplier-only unit pricing.
// It never changes customer PriceData and performs no supplier storage I/O.
func FreezeSupplierOfficialPricingSnapshot(info *relaycommon.RelayInfo, priceData types.PriceData) {
	if info == nil || !info.SupplierCostSnapshot.IsBound() || info.SupplierOfficialPricingSnapshot.CaptureAttempted {
		return
	}
	priceData.OtherRatios = cloneSupplierOtherRatios(priceData.OtherRatios)
	info.SupplierOfficialPricingSnapshot = types.SupplierOfficialPricingSnapshot{
		CaptureAttempted:                      true,
		Loaded:                                true,
		QuotaPerUnit:                          strconv.FormatFloat(common.QuotaPerUnit, 'f', -1, 64),
		PriceData:                             priceData,
		TieredBillingSnapshot:                 cloneSupplierBillingSnapshot(info.TieredBillingSnapshot),
		BillingRequestInput:                   supplierBillingRequestInput(info.TieredBillingSnapshot, info.BillingRequestInput),
		WebSearchPreviewPricePerThousandCalls: operation_setting.GetToolPriceForModel("web_search_preview", info.OriginModelName),
		ClaudeWebSearchPricePerThousandCalls:  operation_setting.GetToolPrice("web_search"),
		FileSearchPricePerThousandCalls:       operation_setting.GetToolPrice("file_search"),
		GeminiInputAudioPricePerMillionTokens: operation_setting.GetGeminiInputAudioPricePerMillionTokens(info.OriginModelName),
		ImageGenerationCallPrices: types.SupplierImageGenerationCallPrices{
			Low1024x1024:    operation_setting.GPTImage1Low1024x1024,
			Low1024x1536:    operation_setting.GPTImage1Low1024x1536,
			Low1536x1024:    operation_setting.GPTImage1Low1536x1024,
			Medium1024x1024: operation_setting.GPTImage1Medium1024x1024,
			Medium1024x1536: operation_setting.GPTImage1Medium1024x1536,
			Medium1536x1024: operation_setting.GPTImage1Medium1536x1024,
			High1024x1024:   operation_setting.GPTImage1High1024x1024,
			High1024x1536:   operation_setting.GPTImage1High1024x1536,
			High1536x1024:   operation_setting.GPTImage1High1536x1024,
		},
	}
}

// RefreshSupplierOfficialPricingSnapshotForCurrentModel recalculates only the
// supplier sidecar on a shallow RelayInfo copy. Responses compact resolves its
// effective billing model after initial customer pre-consume, so it calls this
// immediately after model mapping and before the upstream request.
func RefreshSupplierOfficialPricingSnapshotForCurrentModel(c *gin.Context, info *relaycommon.RelayInfo, promptTokens int, meta *types.TokenCountMeta) error {
	if info == nil || !info.SupplierCostSnapshot.IsBound() {
		return nil
	}
	shadow := *info
	shadow.SupplierOfficialPricingSnapshot = types.SupplierOfficialPricingSnapshot{}
	shadow.TieredBillingSnapshot = nil
	if _, err := ModelPriceHelper(c, &shadow, promptTokens, meta); err != nil {
		info.SupplierOfficialPricingSnapshot = types.SupplierOfficialPricingSnapshot{CaptureAttempted: true}
		return err
	}
	info.SupplierOfficialPricingSnapshot = shadow.SupplierOfficialPricingSnapshot
	return nil
}

func cloneSupplierBillingSnapshot(snapshot *billingexpr.BillingSnapshot) *billingexpr.BillingSnapshot {
	if snapshot == nil {
		return nil
	}
	cloned := *snapshot
	return &cloned
}

func supplierBillingRequestInput(snapshot *billingexpr.BillingSnapshot, input *billingexpr.RequestInput) *billingexpr.RequestInput {
	if snapshot == nil || input == nil {
		return nil
	}
	// ResolveIncomingBillingExprRequestInput creates a request-local immutable
	// value. Reusing it avoids a second O(body_size) copy on the hot path.
	return input
}

func cloneSupplierOtherRatios(source map[string]float64) map[string]float64 {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]float64, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
