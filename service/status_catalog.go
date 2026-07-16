package service

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const statusCatalogJobName = "status-center-catalog"

func FilterPricingByUsableGroups(pricing []model.Pricing, usableGroup map[string]string) []model.Pricing {
	if len(pricing) == 0 {
		return pricing
	}
	if len(usableGroup) == 0 {
		return []model.Pricing{}
	}

	usableGroupNames := sortedUsableGroupNames(usableGroup)
	filtered := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		enableGroups := visiblePricingGroups(item.EnableGroup, usableGroup, usableGroupNames)
		if len(enableGroups) == 0 {
			continue
		}
		item.EnableGroup = enableGroups
		filtered = append(filtered, item)
	}
	return filtered
}

func GetWebsiteVisiblePricing() []model.Pricing {
	return FilterPricingByUsableGroups(model.GetPricing(), GetUserUsableGroups(""))
}

func SyncStatusCatalog(jobName string, holder string, fencingToken int64, now int64, pricing []model.Pricing, usableGroup map[string]string) error {
	desired := []model.StatusComponent{
		{
			ComponentKey:    "router",
			Slug:            "router",
			Kind:            model.StatusComponentKindRouter,
			DisplayName:     "Router",
			Lifecycle:       model.StatusLifecycleActive,
			ObservedStatus:  model.StatusUnknown,
			EffectiveStatus: model.StatusUnknown,
			StatusSource:    "observed",
			Version:         1,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}

	visiblePricing := FilterPricingByUsableGroups(pricing, usableGroup)
	sort.SliceStable(visiblePricing, func(i, j int) bool {
		return visiblePricing[i].ModelName < visiblePricing[j].ModelName
	})
	seen := make(map[string]struct{}, len(visiblePricing))
	for _, item := range visiblePricing {
		modelName := strings.TrimSpace(item.ModelName)
		if modelName == "" {
			continue
		}
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		capability := ""
		if len(item.SupportedEndpointTypes) > 0 {
			capability = string(item.SupportedEndpointTypes[0])
		}
		desired = append(desired, model.StatusComponent{
			ComponentKey:    "model:" + modelName,
			Slug:            statusModelSlug(modelName),
			Kind:            model.StatusComponentKindModel,
			ModelName:       modelName,
			DisplayName:     modelName,
			Capability:      capability,
			Lifecycle:       model.StatusLifecycleActive,
			ObservedStatus:  model.StatusUnknown,
			EffectiveStatus: model.StatusUnknown,
			StatusSource:    "observed",
			Version:         1,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}

	return model.SyncStatusCatalogWithFence(jobName, holder, fencingToken, now, desired)
}

func sortedUsableGroupNames(usableGroup map[string]string) []string {
	groups := make([]string, 0, len(usableGroup))
	for group := range usableGroup {
		if group != "" {
			groups = append(groups, group)
		}
	}
	sort.Strings(groups)
	return groups
}

func visiblePricingGroups(enableGroups []string, usableGroup map[string]string, usableGroupNames []string) []string {
	if common.StringsContains(enableGroups, "all") {
		return append([]string(nil), usableGroupNames...)
	}

	groups := make([]string, 0, len(enableGroups))
	seen := make(map[string]struct{}, len(enableGroups))
	for _, group := range enableGroups {
		if group == "" {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		if _, ok := usableGroup[group]; !ok {
			continue
		}
		seen[group] = struct{}{}
		groups = append(groups, group)
	}
	return groups
}

func statusModelSlug(modelName string) string {
	var slug strings.Builder
	lastWasSeparator := false
	for _, r := range strings.ToLower(modelName) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			slug.WriteRune(r)
			lastWasSeparator = false
			continue
		}
		if slug.Len() > 0 && !lastWasSeparator {
			slug.WriteByte('-')
			lastWasSeparator = true
		}
	}
	base := strings.Trim(slug.String(), "-")
	if base == "" {
		base = "model"
	}
	digest := sha256.Sum256([]byte(modelName))
	return "model-" + base + "-" + hex.EncodeToString(digest[:4])
}
