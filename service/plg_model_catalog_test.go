package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func withServiceHiddenPricingModels(t *testing.T, hiddenModels string) {
	t.Helper()
	visibility := operation_setting.GetPricingVisibilitySetting()
	original := visibility.HiddenModels
	visibility.HiddenModels = hiddenModels
	t.Cleanup(func() {
		visibility.HiddenModels = original
	})
}

func TestFilterHiddenModelsFromUserAccessDropsHiddenEntriesEverywhere(t *testing.T) {
	withServiceHiddenPricingModels(t, "hidden-exact, secret-*")
	access := &UserModelAccess{
		ScopeMode:           ModelAccessScopeSelectableGroup,
		IdentityModelIDs:    []string{"hidden-exact", "visible"},
		IdentityModelRatios: map[string]float64{"hidden-exact": 0.5, "visible": 0.6},
		AccountModelIDs:     []string{"secret-account", "visible"},
		AccountModelRatios:  map[string]float64{"secret-account": 0.7},
		Groups: []ModelAccessScope{{
			ID:          "default",
			ModelIDs:    []string{"secret-default", "visible"},
			ModelRatios: map[string]float64{"secret-default": 0.8, "visible": 0.9},
		}},
		Models: []ModelAccessModel{{ID: "hidden-exact"}, {ID: "secret-default"}, {ID: "visible"}},
	}

	FilterHiddenModelsFromUserAccess(access)

	require.Equal(t, []string{"visible"}, access.IdentityModelIDs)
	require.Equal(t, map[string]float64{"visible": 0.6}, access.IdentityModelRatios)
	require.Equal(t, []string{"visible"}, access.AccountModelIDs)
	require.Empty(t, access.AccountModelRatios)
	require.Equal(t, []string{"visible"}, access.Groups[0].ModelIDs)
	require.Equal(t, map[string]float64{"visible": 0.9}, access.Groups[0].ModelRatios)
	require.Len(t, access.Models, 1)
	require.Equal(t, "visible", access.Models[0].ID)
}

func TestFilterHiddenModelsFromUserAccessIsNoOpWithoutPatterns(t *testing.T) {
	withServiceHiddenPricingModels(t, "")
	access := &UserModelAccess{
		IdentityModelIDs: []string{"a", "b"},
		Models:           []ModelAccessModel{{ID: "a"}, {ID: "b"}},
	}

	FilterHiddenModelsFromUserAccess(access)
	FilterHiddenModelsFromUserAccess(nil)

	require.Equal(t, []string{"a", "b"}, access.IdentityModelIDs)
	require.Len(t, access.Models, 2)
}

func TestResolvePLGCatalogModelNamesMatchesConsoleAvailableModelsView(t *testing.T) {
	db, _ := setupServiceModelAccessDB(t)
	withServiceHiddenPricingModels(t, "hidden-*")
	seedModelAccessScope(t, db, 201, "plg", constant.ChannelTypeOpenAI,
		"plg-visible", "plg-unpriced", "hidden-plg", "plg-probe-failed", "plg-unsupported", "plg-legacy-unknown")
	seedModelAccessScope(t, db, 202, "default", constant.ChannelTypeOpenAI, "default-only")
	setModelAccessBilling(t, map[string]float64{
		"plg-visible": 1, "hidden-plg": 1, "plg-probe-failed": 1, "plg-unsupported": 1, "plg-legacy-unknown": 1, "default-only": 1,
	}, nil, nil)
	require.NoError(t, db.Create(&model.ModelAvailabilityState{ModelName: "plg-probe-failed", Status: model.ModelAvailabilityUnknownFailure}).Error)
	require.NoError(t, db.Create(&model.ModelAvailabilityState{ModelName: "plg-unsupported", Status: model.ModelAvailabilityOfficialUnsupported}).Error)
	require.NoError(t, db.Create(&model.ModelAvailabilityState{ModelName: "plg-legacy-unknown", Status: "unknown"}).Error)

	names, err := ResolvePLGCatalogModelNames()
	require.NoError(t, err)
	require.Equal(t, []string{"plg-legacy-unknown", "plg-visible"}, names)
}

func TestResolvePLGCatalogModelNamesIgnoresDisabledChannels(t *testing.T) {
	db, _ := setupServiceModelAccessDB(t)
	seedModelAccessScope(t, db, 211, "plg", constant.ChannelTypeOpenAI, "plg-on")
	seedModelAccessScope(t, db, 212, "plg", constant.ChannelTypeOpenAI, "plg-off")
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 212).Update("status", common.ChannelStatusManuallyDisabled).Error)
	setModelAccessBilling(t, map[string]float64{"plg-on": 1, "plg-off": 1}, nil, nil)

	names, err := ResolvePLGCatalogModelNames()
	require.NoError(t, err)
	require.Equal(t, []string{"plg-on"}, names)
}

func TestDiffModelNamesReportsAddedAndRemovedSorted(t *testing.T) {
	added, removed := DiffModelNames(
		[]string{"b", "a", "keep"},
		[]string{"keep", "z", "c"},
	)
	require.Equal(t, []string{"c", "z"}, added)
	require.Equal(t, []string{"a", "b"}, removed)

	added, removed = DiffModelNames([]string{"same"}, []string{"same"})
	require.Empty(t, added)
	require.Empty(t, removed)
}
