package service

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/shopspring/decimal"
)

const websitePricingV2Group = "plg"

type WebsitePricePair struct {
	Configured string `json:"configured"`
	PLG        string `json:"plg"`
}

type WebsiteDisplayPrices struct {
	Input         *WebsitePricePair `json:"input"`
	Output        *WebsitePricePair `json:"output"`
	Cache         *WebsitePricePair `json:"cache"`
	CacheCreation *WebsitePricePair `json:"cache_creation"`
	Image         *WebsitePricePair `json:"image"`
	AudioInput    *WebsitePricePair `json:"audio_input"`
	AudioOutput   *WebsitePricePair `json:"audio_output"`
	Request       *WebsitePricePair `json:"request"`
}

type WebsitePricingModel struct {
	ModelName              string                  `json:"model_name"`
	Description            string                  `json:"description,omitempty"`
	Icon                   string                  `json:"icon,omitempty"`
	Tags                   string                  `json:"tags,omitempty"`
	VendorID               int                     `json:"vendor_id,omitempty"`
	QuotaType              int                     `json:"quota_type"`
	EnableGroups           []string                `json:"enable_groups"`
	SupportedEndpointTypes []constant.EndpointType `json:"supported_endpoint_types"`
	BillingKind            string                  `json:"billing_kind"`
	DisplayLabel           string                  `json:"display_label,omitempty"`
	EffectiveGroupRatio    string                  `json:"effective_group_ratio"`
	RatioSource            string                  `json:"ratio_source"`
	Prices                 WebsiteDisplayPrices    `json:"prices"`
	AvailabilityStatus     string                  `json:"availability_status,omitempty"`
	AvailabilityReason     string                  `json:"availability_reason,omitempty"`
	AvailabilityDetectedAt int64                   `json:"availability_detected_at,omitempty"`
	AvailabilityCheckedAt  int64                   `json:"availability_checked_at,omitempty"`
}

type WebsitePricingV2 struct {
	Success           bool                           `json:"success"`
	SchemaVersion     string                         `json:"schema_version"`
	Group             string                         `json:"group"`
	GeneratedAt       int64                          `json:"generated_at"`
	Models            []WebsitePricingModel          `json:"models"`
	Vendors           []model.PricingVendor          `json:"vendors"`
	SupportedEndpoint map[string]common.EndpointInfo `json:"supported_endpoint"`
	AutoGroups        []string                       `json:"auto_groups"`
}

type websitePricingSource interface {
	BillingMode(string) string
	EffectiveGroupRatio(string) types.GroupRatioInfo
	HasGroup(string) bool
	QuotaPerUnit() float64
}

type liveWebsitePricingSource struct{}

func (liveWebsitePricingSource) BillingMode(modelName string) string {
	return billing_setting.GetBillingMode(modelName)
}

func (liveWebsitePricingSource) EffectiveGroupRatio(modelName string) types.GroupRatioInfo {
	return ratio_setting.GetEffectiveGroupRatio(websitePricingV2Group, websitePricingV2Group, modelName)
}

func (liveWebsitePricingSource) HasGroup(group string) bool {
	_, ok := ratio_setting.GetGroupRatioCopy()[group]
	return ok
}

func (liveWebsitePricingSource) QuotaPerUnit() float64 { return common.QuotaPerUnit }

func BuildWebsitePricingV2(
	pricing []model.Pricing,
	vendors []model.PricingVendor,
	supportedEndpoint map[string]common.EndpointInfo,
	autoGroups []string,
	group string,
	generatedAt time.Time,
) (WebsitePricingV2, error) {
	return buildWebsitePricingV2(pricing, vendors, supportedEndpoint, autoGroups, group, generatedAt, liveWebsitePricingSource{})
}

func buildWebsitePricingV2(
	pricing []model.Pricing,
	vendors []model.PricingVendor,
	supportedEndpoint map[string]common.EndpointInfo,
	autoGroups []string,
	group string,
	generatedAt time.Time,
	source websitePricingSource,
) (WebsitePricingV2, error) {
	if group != websitePricingV2Group {
		return WebsitePricingV2{}, fmt.Errorf("unsupported website pricing group %q", group)
	}
	if !source.HasGroup(group) {
		return WebsitePricingV2{}, errors.New("public website group is not configured")
	}
	quotaPerUnit := source.QuotaPerUnit()
	if !validWebsitePrice(quotaPerUnit) || quotaPerUnit == 0 {
		return WebsitePricingV2{}, errors.New("quota per unit must be a positive finite number")
	}

	models := make([]WebsitePricingModel, 0, len(pricing))
	for _, item := range pricing {
		if !websiteModelVisibleToGroup(item, group) {
			continue
		}
		ratioInfo := source.EffectiveGroupRatio(item.ModelName)
		if !validWebsitePrice(ratioInfo.GroupRatio) {
			return WebsitePricingV2{}, fmt.Errorf("invalid group ratio for model %q", item.ModelName)
		}

		row := WebsitePricingModel{
			ModelName:              item.ModelName,
			Description:            item.Description,
			Icon:                   item.Icon,
			Tags:                   item.Tags,
			VendorID:               item.VendorID,
			QuotaType:              item.QuotaType,
			EnableGroups:           []string{group},
			SupportedEndpointTypes: item.SupportedEndpointTypes,
			EffectiveGroupRatio:    decimal.NewFromFloat(ratioInfo.GroupRatio).String(),
			RatioSource:            websiteRatioSource(ratioInfo),
			AvailabilityStatus:     item.AvailabilityStatus,
			AvailabilityReason:     item.AvailabilityReason,
			AvailabilityDetectedAt: item.AvailabilityDetectedAt,
			AvailabilityCheckedAt:  item.AvailabilityCheckedAt,
		}

		switch source.BillingMode(item.ModelName) {
		case billing_setting.BillingModeTieredExpr:
			row.BillingKind = billing_setting.BillingModeTieredExpr
			row.DisplayLabel = "Variable pricing"
		case billing_setting.BillingModeRatio:
			if item.QuotaType == 1 {
				row.BillingKind = "request_base"
				row.Prices.Request = websitePricePair(item.ModelPrice, ratioInfo.GroupRatio)
				break
			}
			row.BillingKind = "token_ratio"
			input := decimal.NewFromInt(1_000_000).
				Mul(decimal.NewFromFloat(item.ModelRatio)).
				Div(decimal.NewFromFloat(quotaPerUnit)).InexactFloat64()
			if !validWebsitePrice(input) {
				return WebsitePricingV2{}, fmt.Errorf("invalid input price for model %q", item.ModelName)
			}
			row.Prices.Input = websitePricePair(input, ratioInfo.GroupRatio)
			row.Prices.Output = websitePricePair(input*item.CompletionRatio, ratioInfo.GroupRatio)
			row.Prices.Cache = websiteOptionalPricePair(input, item.CacheRatio, ratioInfo.GroupRatio)
			row.Prices.CacheCreation = websiteOptionalPricePair(input, item.CreateCacheRatio, ratioInfo.GroupRatio)
			row.Prices.Image = websiteOptionalPricePair(input, item.ImageRatio, ratioInfo.GroupRatio)
			row.Prices.AudioInput = websiteOptionalPricePair(input, item.AudioRatio, ratioInfo.GroupRatio)
			row.Prices.AudioOutput = websiteOptionalPricePair(input, item.AudioCompletionRatio, ratioInfo.GroupRatio)
		default:
			return WebsitePricingV2{}, fmt.Errorf("unsupported billing mode for model %q", item.ModelName)
		}
		models = append(models, row)
	}

	sort.Slice(models, func(i, j int) bool { return models[i].ModelName < models[j].ModelName })
	return WebsitePricingV2{
		Success:           true,
		SchemaVersion:     "website-public-plg-v2",
		Group:             group,
		GeneratedAt:       generatedAt.Unix(),
		Models:            models,
		Vendors:           append([]model.PricingVendor(nil), vendors...),
		SupportedEndpoint: supportedEndpoint,
		AutoGroups:        append([]string(nil), autoGroups...),
	}, nil
}

func websitePricePair(configured, groupRatio float64) *WebsitePricePair {
	if !validWebsitePrice(configured) {
		return nil
	}
	return &WebsitePricePair{
		Configured: decimal.NewFromFloat(configured).String(),
		PLG:        decimal.NewFromFloat(configured).Mul(decimal.NewFromFloat(groupRatio)).String(),
	}
}

func websiteOptionalPricePair(base float64, ratio *float64, groupRatio float64) *WebsitePricePair {
	if ratio == nil {
		return nil
	}
	return websitePricePair(base**ratio, groupRatio)
}

func websiteModelVisibleToGroup(item model.Pricing, group string) bool {
	for _, enabledGroup := range item.EnableGroup {
		if enabledGroup == "all" || enabledGroup == group {
			return true
		}
	}
	return false
}

func websiteRatioSource(info types.GroupRatioInfo) string {
	if info.HasGroupModelRatio {
		return "group_model"
	}
	if info.HasSpecialRatio {
		return "group_group"
	}
	return "group"
}

func validWebsitePrice(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
