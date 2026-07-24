package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

type fakeWebsitePricingSource struct {
	modes           map[string]string
	ratios          map[string]types.GroupRatioInfo
	quotaPerUnit    float64
	configuredGroup bool
}

func (source fakeWebsitePricingSource) BillingMode(modelName string) string {
	if mode := source.modes[modelName]; mode != "" {
		return mode
	}
	return billing_setting.BillingModeRatio
}

func (source fakeWebsitePricingSource) EffectiveGroupRatio(modelName string) types.GroupRatioInfo {
	return source.ratios[modelName]
}

func (source fakeWebsitePricingSource) HasGroup(string) bool  { return source.configuredGroup }
func (source fakeWebsitePricingSource) QuotaPerUnit() float64 { return source.quotaPerUnit }

func TestBuildWebsitePricingV2ReturnsDisplayReadyPLGPricesOnly(t *testing.T) {
	cacheRatio := 0.25
	pricing := []model.Pricing{
		{
			ModelName: "token-model", QuotaType: 0, ModelRatio: 2, CompletionRatio: 3,
			CacheRatio: &cacheRatio, EnableGroup: []string{"plg", "internal"}, BillingExpr: "secret()",
		},
		{ModelName: "request-model", QuotaType: 1, ModelPrice: 0.02, EnableGroup: []string{"all"}},
		{ModelName: "tiered-model", QuotaType: 0, EnableGroup: []string{"plg"}, BillingExpr: "tier(secret)"},
		{ModelName: "internal-model", QuotaType: 0, ModelRatio: 9, EnableGroup: []string{"internal"}},
	}
	source := fakeWebsitePricingSource{
		modes: map[string]string{"tiered-model": billing_setting.BillingModeTieredExpr},
		ratios: map[string]types.GroupRatioInfo{
			"token-model":   {GroupRatio: 0.5, HasGroupModelRatio: true},
			"request-model": {GroupRatio: 0.5},
			"tiered-model":  {GroupRatio: 0.5},
		},
		quotaPerUnit: 500_000, configuredGroup: true,
	}

	payload, err := buildWebsitePricingV2(pricing, nil, nil, nil, "plg", time.Unix(100, 0), source)
	require.NoError(t, err)
	require.Len(t, payload.Models, 3)
	require.Equal(t, "request-model", payload.Models[0].ModelName)
	require.Equal(t, "0.02", payload.Models[0].Prices.Request.Configured)
	require.Equal(t, "0.01", payload.Models[0].Prices.Request.PLG)
	require.Equal(t, "token-model", payload.Models[2].ModelName)
	require.Equal(t, "4", payload.Models[2].Prices.Input.Configured)
	require.Equal(t, "2", payload.Models[2].Prices.Input.PLG)
	require.Equal(t, "12", payload.Models[2].Prices.Output.Configured)
	require.Equal(t, "6", payload.Models[2].Prices.Output.PLG)
	require.Equal(t, "1", payload.Models[2].Prices.Cache.Configured)
	require.Equal(t, "0.5", payload.Models[2].Prices.Cache.PLG)
	require.Equal(t, "group_model", payload.Models[2].RatioSource)
	require.Equal(t, billing_setting.BillingModeTieredExpr, payload.Models[1].BillingKind)
	require.Nil(t, payload.Models[1].Prices.Input)
	require.Equal(t, "Variable pricing", payload.Models[1].DisplayLabel)

	body, err := common.Marshal(payload)
	require.NoError(t, err)
	require.NotContains(t, string(body), "model_ratio")
	require.NotContains(t, string(body), "billing_expr")
	require.NotContains(t, string(body), "secret")
	require.NotContains(t, string(body), "internal-model")
}

func TestBuildWebsitePricingV2FailsClosedOutsidePLG(t *testing.T) {
	source := fakeWebsitePricingSource{quotaPerUnit: 500_000, configuredGroup: true}
	_, err := buildWebsitePricingV2(nil, nil, nil, nil, "vip", time.Unix(100, 0), source)
	require.ErrorContains(t, err, "unsupported website pricing group")

	source.configuredGroup = false
	_, err = buildWebsitePricingV2(nil, nil, nil, nil, "plg", time.Unix(100, 0), source)
	require.ErrorContains(t, err, "public website group is not configured")
}
