package service

import (
	"sort"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// FilterHiddenModelsFromUserAccess applies the pricing visibility setting to a
// resolved model access view. It changes what the Console catalog shows; it
// does not change the user's actual access.
func FilterHiddenModelsFromUserAccess(access *UserModelAccess) {
	if access == nil || len(operation_setting.GetPricingHiddenModelPatterns()) == 0 {
		return
	}

	access.IdentityModelIDs = filterVisibleModelIDs(access.IdentityModelIDs)
	access.IdentityModelRatios = filterVisibleModelRatios(access.IdentityModelRatios)
	access.AccountModelIDs = filterVisibleModelIDs(access.AccountModelIDs)
	access.AccountModelRatios = filterVisibleModelRatios(access.AccountModelRatios)
	for i := range access.Groups {
		access.Groups[i].ModelIDs = filterVisibleModelIDs(access.Groups[i].ModelIDs)
		access.Groups[i].ModelRatios = filterVisibleModelRatios(access.Groups[i].ModelRatios)
	}

	models := make([]ModelAccessModel, 0, len(access.Models))
	for _, item := range access.Models {
		if !operation_setting.IsPricingHiddenModel(item.ID) {
			models = append(models, item)
		}
	}
	access.Models = models
}

func filterVisibleModelIDs(modelIDs []string) []string {
	visible := make([]string, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		if !operation_setting.IsPricingHiddenModel(modelID) {
			visible = append(visible, modelID)
		}
	}
	return visible
}

func filterVisibleModelRatios(modelRatios map[string]float64) map[string]float64 {
	visible := make(map[string]float64, len(modelRatios))
	for modelID, ratio := range modelRatios {
		if !operation_setting.IsPricingHiddenModel(modelID) {
			visible[modelID] = ratio
		}
	}
	return visible
}

// ResolvePLGCatalogModelNames returns the sorted model names a plg account
// sees on the Console "Available Models" page: the fixed-account resolver,
// the pricing hidden-model filter, and the catalog's availability rule that
// keeps only `available` and legacy `unknown` records.
func ResolvePLGCatalogModelNames() ([]string, error) {
	// The resolver also maps an empty group to plg; pass the literal group so
	// this task keeps meaning plg even if that default changes.
	access, err := ResolveUserModelAccess(&model.UserBase{Group: modelAccessPLGGroup})
	if err != nil {
		return nil, err
	}
	FilterHiddenModelsFromUserAccess(access)

	names := make([]string, 0, len(access.Models))
	for _, item := range access.Models {
		if isCatalogAvailable(item.AvailabilityStatus) {
			names = append(names, item.ID)
		}
	}
	sort.Strings(names)
	return names, nil
}

func isCatalogAvailable(status string) bool {
	return status == model.ModelAvailabilityAvailable || status == ModelAvailabilityUnknown
}

// DiffModelNames returns the names present only in next (added) and only in
// previous (removed), both sorted.
func DiffModelNames(previous []string, next []string) (added []string, removed []string) {
	previousSet := make(map[string]struct{}, len(previous))
	for _, name := range previous {
		previousSet[name] = struct{}{}
	}
	nextSet := make(map[string]struct{}, len(next))
	for _, name := range next {
		nextSet[name] = struct{}{}
	}
	added = make([]string, 0)
	for name := range nextSet {
		if _, ok := previousSet[name]; !ok {
			added = append(added, name)
		}
	}
	removed = make([]string, 0)
	for name := range previousSet {
		if _, ok := nextSet[name]; !ok {
			removed = append(removed, name)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}
