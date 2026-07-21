package service

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupStatusServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=busy_timeout(5000)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.StatusCenterModels()...))
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	return db
}

func acquireStatusServiceLease(t *testing.T, name string, holder string, now int64) model.StatusJobLease {
	t.Helper()
	lease, acquired, err := model.AcquireStatusJobLease(name, holder, now, 60)
	require.NoError(t, err)
	require.True(t, acquired)
	return lease
}

func TestStatusCatalogMatchesWebsiteVisiblePricing(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	lease := acquireStatusServiceLease(t, statusCatalogJobName, "node-a", 1_000)
	pricing := []model.Pricing{
		{ModelName: "gpt-public", EnableGroup: []string{"public"}, SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAI}},
		{ModelName: "gpt-all", EnableGroup: []string{"all"}, SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAIResponse}},
		{ModelName: "gpt-private", EnableGroup: []string{"private"}},
	}
	usableGroups := map[string]string{"public": "Public"}

	visible := FilterPricingByUsableGroups(pricing, usableGroups)
	require.Equal(t, []string{"gpt-public", "gpt-all"}, []string{visible[0].ModelName, visible[1].ModelName})
	require.NoError(t, SyncStatusCatalog(statusCatalogJobName, "node-a", lease.FencingToken, 1_000, pricing, usableGroups))

	var components []model.StatusComponent
	require.NoError(t, db.Order("kind DESC, model_name ASC").Find(&components).Error)
	require.Len(t, components, 3)
	require.Equal(t, model.StatusComponentKindRouter, components[0].Kind)
	require.Equal(t, "router", components[0].ComponentKey)
	require.Equal(t, "router", components[0].Slug)
	require.Equal(t, []string{"gpt-all", "gpt-public"}, []string{components[1].ModelName, components[2].ModelName})
	for _, component := range components {
		require.Equal(t, model.StatusLifecycleActive, component.Lifecycle)
	}
}

func TestStatusCatalogKeepsStableModelIdentityAndRetiresRemovedModels(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	firstLease := acquireStatusServiceLease(t, statusCatalogJobName, "node-a", 2_000)
	pricing := []model.Pricing{
		{ModelName: "GPT 5.4/Latest", Description: "First label", EnableGroup: []string{"public"}},
		{ModelName: "retired-model", EnableGroup: []string{"public"}},
	}
	require.NoError(t, SyncStatusCatalog(statusCatalogJobName, "node-a", firstLease.FencingToken, 2_000, pricing, map[string]string{"public": "Public"}))

	var original model.StatusComponent
	require.NoError(t, db.Where("model_name = ?", "GPT 5.4/Latest").First(&original).Error)
	require.NotEmpty(t, original.Slug)
	require.Equal(t, "model:GPT 5.4/Latest", original.ComponentKey)

	secondLease, acquired, err := model.AcquireStatusJobLease(statusCatalogJobName, "node-b", 2_061, 60)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NoError(t, SyncStatusCatalog(statusCatalogJobName, "node-b", secondLease.FencingToken, 2_061, []model.Pricing{
		{ModelName: "GPT 5.4/Latest", Description: "Renamed label", EnableGroup: []string{"public"}},
	}, map[string]string{"public": "Public"}))

	var current model.StatusComponent
	require.NoError(t, db.Where("model_name = ?", "GPT 5.4/Latest").First(&current).Error)
	require.Equal(t, original.ID, current.ID)
	require.Equal(t, original.ComponentKey, current.ComponentKey)
	require.Equal(t, original.Slug, current.Slug)
	require.Equal(t, model.StatusLifecycleActive, current.Lifecycle)

	var retired model.StatusComponent
	require.NoError(t, db.Where("model_name = ?", "retired-model").First(&retired).Error)
	require.Equal(t, model.StatusLifecycleRetired, retired.Lifecycle)
}
