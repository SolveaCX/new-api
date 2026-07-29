package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func restoreGroupRatioSettings(t *testing.T) {
	t.Helper()

	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalGroupGroupRatio := ratio_setting.GroupGroupRatio2JSONString()
	originalGroupModelRatio := ratio_setting.GroupModelRatio2JSONString()

	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalGroupGroupRatio))
		require.NoError(t, ratio_setting.UpdateGroupModelRatioByJSONString(originalGroupModelRatio))
	})
}

func TestHandleGroupRatioUsesGroupModelRatioBeforeUserGroupOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restoreGroupRatioSettings(t)

	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"plg":0.9}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"vip":{"plg":0.8}}`))
	require.NoError(t, ratio_setting.UpdateGroupModelRatioByJSONString(`{"plg":{"gpt-5.5":0.3}}`))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.5",
		UserGroup:       "vip",
		UsingGroup:      "plg",
	}

	groupRatioInfo := HandleGroupRatio(ctx, info)

	require.Equal(t, 0.3, groupRatioInfo.GroupRatio)
	require.True(t, groupRatioInfo.HasGroupModelRatio)
	require.Equal(t, "plg", groupRatioInfo.GroupModelRatioGroup)
	require.Equal(t, "gpt-5.5", groupRatioInfo.GroupModelRatioModel)
	require.False(t, groupRatioInfo.HasSpecialRatio)
}

func TestApplyResolvedGroupRatioFreezesCrossGroupRetryForRatioAndTieredSettlement(t *testing.T) {
	info := &relaycommon.RelayInfo{
		PriceData: types.PriceData{GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.3}},
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			GroupRatio: 0.3, EstimatedQuotaBeforeGroup: 100, EstimatedQuotaAfterGroup: 30,
		},
		SupplierOfficialPricingSnapshot: types.SupplierOfficialPricingSnapshot{
			Loaded:    true,
			PriceData: types.PriceData{GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.3}},
			TieredBillingSnapshot: &billingexpr.BillingSnapshot{
				GroupRatio: 0.3, EstimatedQuotaBeforeGroup: 100, EstimatedQuotaAfterGroup: 30,
			},
		},
	}
	finalGroup := types.GroupRatioInfo{GroupRatio: 0.9, HasSpecialRatio: true, GroupSpecialRatio: 0.8}

	ApplyResolvedGroupRatio(info, finalGroup)

	require.Equal(t, finalGroup, info.PriceData.GroupRatioInfo)
	require.Equal(t, 0.9, info.TieredBillingSnapshot.GroupRatio)
	require.Equal(t, 90, info.TieredBillingSnapshot.EstimatedQuotaAfterGroup)
	require.Equal(t, finalGroup, info.SupplierOfficialPricingSnapshot.PriceData.GroupRatioInfo)
	require.Equal(t, 0.9, info.SupplierOfficialPricingSnapshot.TieredBillingSnapshot.GroupRatio)
	require.Equal(t, 90, info.SupplierOfficialPricingSnapshot.TieredBillingSnapshot.EstimatedQuotaAfterGroup)
}

func TestFreezeSupplierOfficialPricingSnapshotCopiesRequestPricing(t *testing.T) {
	priceData := types.PriceData{
		ModelRatio: 2,
		OtherRatios: map[string]float64{
			"n": 2,
		},
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "gemini-2.5-flash",
		SupplierCostSnapshot: types.SupplierCostSnapshot{
			SupplierId: 1, ContractId: 2, RateVersionId: 3,
		},
	}

	FreezeSupplierOfficialPricingSnapshot(info, priceData)
	priceData.OtherRatios["n"] = 9

	require.True(t, info.SupplierOfficialPricingSnapshot.Loaded)
	require.NotEmpty(t, info.SupplierOfficialPricingSnapshot.QuotaPerUnit)
	require.Equal(t, float64(2), info.SupplierOfficialPricingSnapshot.PriceData.OtherRatios["n"])
	require.Equal(t, float64(1), info.SupplierOfficialPricingSnapshot.GeminiInputAudioPricePerMillionTokens)
	require.Equal(t, operation_setting.GPTImage1Low1024x1024, info.SupplierOfficialPricingSnapshot.ImageGenerationCallPrices.Price("low", "1024x1024"))

	FreezeSupplierOfficialPricingSnapshot(info, types.PriceData{ModelRatio: 99})
	require.Equal(t, float64(2), info.SupplierOfficialPricingSnapshot.PriceData.ModelRatio, "response-time helper calls must not overwrite the request snapshot")
}

func TestFreezeSupplierOfficialPricingSnapshotUnboundHasZeroAllocations(t *testing.T) {
	info := &relaycommon.RelayInfo{OriginModelName: "unbound"}
	allocations := testing.AllocsPerRun(1000, func() {
		FreezeSupplierOfficialPricingSnapshot(info, types.PriceData{ModelRatio: 2})
	})
	require.Zero(t, allocations)
	require.False(t, info.SupplierOfficialPricingSnapshot.Loaded)
}

func TestFreezeSupplierOfficialPricingSnapshotBoundHasBoundedAllocations(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5.5",
		SupplierCostSnapshot: types.SupplierCostSnapshot{
			SupplierId: 1, ContractId: 2, RateVersionId: 3,
		},
	}
	allocations := testing.AllocsPerRun(1000, func() {
		info.SupplierOfficialPricingSnapshot = types.SupplierOfficialPricingSnapshot{}
		FreezeSupplierOfficialPricingSnapshot(info, types.PriceData{ModelRatio: 2})
	})
	require.LessOrEqual(t, allocations, float64(2))
}

func TestGenRelayInfoLoadsSupplierSnapshotBeforeInitialPricing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "request-model")
	common.SetContextKey(ctx, constant.ContextKeySupplierCostSnapshot, types.SupplierCostSnapshot{
		SupplierId: 1, ContractId: 2, RateVersionId: 3,
	})
	common.SetContextKey(ctx, constant.ContextKeySupplierStatsScope, types.BusinessSupplierStatisticsScopeSnapshot())

	info := relaycommon.GenRelayInfoOpenAI(ctx, nil)
	FreezeSupplierOfficialPricingSnapshot(info, types.PriceData{ModelRatio: 2})

	require.True(t, info.SupplierCostSnapshot.IsBound())
	require.True(t, info.SupplierOfficialPricingSnapshot.Loaded)
}

func TestRefreshSupplierOfficialPricingSnapshotUsesResolvedCompactModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalModelPrice := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalModelPrice))
	})
	modelPrices, err := common.Marshal(map[string]float64{ratio_setting.CompactWildcardModelKey: 7.25})
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(modelPrices)))

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	ctx.Set("group", "default")
	info := &relaycommon.RelayInfo{
		OriginModelName: ratio_setting.WithCompactModelSuffix("mapped-upstream-model"),
		UsingGroup:      "default",
		UserGroup:       "default",
		RelayMode:       relayconstant.RelayModeResponsesCompact,
		SupplierCostSnapshot: types.SupplierCostSnapshot{
			SupplierId: 1, ContractId: 2, RateVersionId: 3,
		},
		SupplierOfficialPricingSnapshot: types.SupplierOfficialPricingSnapshot{
			Loaded: true,
			PriceData: types.PriceData{
				UsePrice:   true,
				ModelPrice: 1,
			},
		},
	}

	require.NoError(t, RefreshSupplierOfficialPricingSnapshotForCurrentModel(ctx, info, 0, &types.TokenCountMeta{}))
	require.True(t, info.SupplierOfficialPricingSnapshot.Loaded)
	require.True(t, info.SupplierOfficialPricingSnapshot.PriceData.UsePrice)
	require.Equal(t, 7.25, info.SupplierOfficialPricingSnapshot.PriceData.ModelPrice)
}

func TestCompactTieredSupplierSnapshotSurvivesConfigChangeBeforeSettlement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	modelName := ratio_setting.WithCompactModelSuffix("supplier-compact-tiered")
	oldExpr := `tier("old", p * 2)`
	newExpr := `tier("new", p * 9)`
	modeJSON, err := common.Marshal(map[string]string{modelName: "tiered_expr"})
	require.NoError(t, err)
	oldExprJSON, err := common.Marshal(map[string]string{modelName: oldExpr})
	require.NoError(t, err)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": string(modeJSON),
		"billing_setting.billing_expr": string(oldExprJSON),
	}))

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	ctx.Set("group", "default")
	info := &relaycommon.RelayInfo{
		OriginModelName: modelName,
		UsingGroup:      "default",
		UserGroup:       "default",
		SupplierCostSnapshot: types.SupplierCostSnapshot{
			SupplierId: 1, ContractId: 2, RateVersionId: 3,
		},
	}
	require.NoError(t, RefreshSupplierOfficialPricingSnapshotForCurrentModel(ctx, info, 100, &types.TokenCountMeta{}))
	require.True(t, info.SupplierOfficialPricingSnapshot.CaptureAttempted)
	require.Equal(t, oldExpr, info.SupplierOfficialPricingSnapshot.TieredBillingSnapshot.ExprString)

	newExprJSON, err := common.Marshal(map[string]string{modelName: newExpr})
	require.NoError(t, err)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": string(modeJSON),
		"billing_setting.billing_expr": string(newExprJSON),
	}))
	_, err = ModelPriceHelper(ctx, info, 100, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.Equal(t, newExpr, info.TieredBillingSnapshot.ExprString, "customer settlement keeps its existing response-time path")
	require.Equal(t, oldExpr, info.SupplierOfficialPricingSnapshot.TieredBillingSnapshot.ExprString, "supplier settlement must retain request-time config")
}

func TestModelPriceHelperTieredUsesPreloadedRequestInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"tiered-test-model":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"tiered-test-model":"param(\"stream\") == true ? tier(\"stream\", p * 3) : tier(\"base\", p * 2)"}`,
	}))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/channel/test/1", nil)
	req.Body = nil
	req.ContentLength = 0
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		OriginModelName: "tiered-test-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		RequestHeaders:  map[string]string{"Content-Type": "application/json"},
		BillingRequestInput: &billingexpr.RequestInput{
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    []byte(`{"stream":true}`),
		},
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.Equal(t, 1500, priceData.QuotaToPreConsume)
	require.NotNil(t, info.TieredBillingSnapshot)
	require.Equal(t, "stream", info.TieredBillingSnapshot.EstimatedTier)
	require.Equal(t, billing_setting.BillingModeTieredExpr, info.TieredBillingSnapshot.BillingMode)
	require.Equal(t, common.QuotaPerUnit, info.TieredBillingSnapshot.QuotaPerUnit)
}
