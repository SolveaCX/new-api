package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userModelsResponse struct {
	Success bool     `json:"success"`
	Data    []string `json:"data"`
}

func decodeUserModelsResponse(t *testing.T, recorder *httptest.ResponseRecorder) []string {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload userModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	return payload.Data
}

func requestUserModels(t *testing.T, target string, userId int) []string {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
	ctx.Set("id", userId)

	GetUserModels(ctx)

	return decodeUserModelsResponse(t, recorder)
}

func TestGetUserModelsFiltersByGroup(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	originalUserGroups := setting.UserUsableGroups2JSONString()
	originalAutoGroups := setting.AutoGroups2JsonString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUserGroups))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{
		"default": "Default group",
		"vip": "VIP group",
		"auto": "Auto group"
	}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["default","vip"]`))

	require.NoError(t, db.Create(&model.User{
		Id:       3001,
		Username: "playground-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "default-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "shared-model", ChannelId: 1, Enabled: true},
		{Group: "vip", Model: "vip-model", ChannelId: 2, Enabled: true},
		{Group: "vip", Model: "shared-model", ChannelId: 2, Enabled: true},
		{Group: "blocked", Model: "blocked-model", ChannelId: 3, Enabled: true},
		{Group: "default", Model: "disabled-model", ChannelId: 4, Enabled: false},
	}).Error)

	require.ElementsMatch(t, []string{"default-model", "shared-model"}, requestUserModels(t, "/api/user/models?group=default", 3001))
	require.ElementsMatch(t, []string{"vip-model", "shared-model"}, requestUserModels(t, "/api/user/models?group=vip", 3001))
	require.ElementsMatch(t, []string{"default-model", "shared-model", "vip-model"}, requestUserModels(t, "/api/user/models?group=auto", 3001))
	require.Empty(t, requestUserModels(t, "/api/user/models?group=blocked", 3001))
	require.ElementsMatch(t, []string{"default-model", "shared-model", "vip-model"}, requestUserModels(t, "/api/user/models", 3001))
}

func TestGetUserModelsCanExcludeAdministrativelyHiddenModels(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	originalUserGroups := setting.UserUsableGroups2JSONString()
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalHiddenModels := operation_setting.GetPricingVisibilitySetting().HiddenModels
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUserGroups))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		operation_setting.GetPricingVisibilitySetting().HiddenModels = originalHiddenModels
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{
		"default": "Default group",
		"vip": "VIP group",
		"auto": "Auto group"
	}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["default","vip"]`))
	operation_setting.GetPricingVisibilitySetting().HiddenModels = "gpt-image-2,seedance-*"

	require.NoError(t, db.Create(&model.User{
		Id:       3002,
		Username: "playground-visibility-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "gpt-4o", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "gpt-image-2", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "seedance-2.5", ChannelId: 1, Enabled: true},
		{Group: "vip", Model: "visible-vip", ChannelId: 2, Enabled: true},
		{Group: "vip", Model: "seedance-2.0-fast", ChannelId: 2, Enabled: true},
	}).Error)

	require.ElementsMatch(t,
		[]string{"gpt-4o", "gpt-image-2", "seedance-2.5"},
		requestUserModels(t, "/api/user/models?group=default", 3002),
	)
	require.ElementsMatch(t,
		[]string{"gpt-4o", "gpt-image-2", "seedance-2.5"},
		requestUserModels(t, "/api/user/models?group=default&exclude_hidden=false", 3002),
	)
	require.ElementsMatch(t,
		[]string{"gpt-4o"},
		requestUserModels(t, "/api/user/models?group=default&exclude_hidden=true", 3002),
	)
	require.Empty(t, requestUserModels(t, "/api/user/models?group=blocked&exclude_hidden=true", 3002))
	require.ElementsMatch(t,
		[]string{"gpt-4o", "visible-vip"},
		requestUserModels(t, "/api/user/models?group=auto&exclude_hidden=true", 3002),
	)
	require.ElementsMatch(t,
		[]string{"gpt-4o", "visible-vip"},
		requestUserModels(t, "/api/user/models?exclude_hidden=true", 3002),
	)
}

func TestGetUserModelsSortsModelsWithinFamilyByCreatedTime(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	originalUserGroups := setting.UserUsableGroups2JSONString()
	originalAutoGroups := setting.AutoGroups2JsonString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUserGroups))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{
		"default": "Default group",
		"vip": "VIP group",
		"auto": "Auto group"
	}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["default","vip"]`))

	require.NoError(t, db.Create(&model.User{
		Id:       3003,
		Username: "playground-order-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "gpt-5.4", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "claude-sonnet-4", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "gpt-5.5", ChannelId: 1, Enabled: true},
		{Group: "vip", Model: "gpt-5.4", ChannelId: 2, Enabled: true},
		{Group: "vip", Model: "gpt-5.5", ChannelId: 2, Enabled: true},
	}).Error)
	require.NoError(t, db.Create(&[]model.Model{
		{ModelName: "gpt-5.4", CreatedTime: 100},
		{ModelName: "gpt-5.5", CreatedTime: 200},
		{ModelName: "claude-sonnet-4", CreatedTime: 300},
	}).Error)

	for _, target := range []string{
		"/api/user/models?group=default",
		"/api/user/models?group=auto",
		"/api/user/models",
	} {
		models := requestUserModels(t, target, 3003)
		newerIndex := indexOfModel(t, models, "gpt-5.5")
		olderIndex := indexOfModel(t, models, "gpt-5.4")
		require.Less(t, newerIndex, olderIndex, "expected newer model first for %s", target)
		require.Contains(t, models, "claude-sonnet-4")
	}
}

func indexOfModel(t *testing.T, models []string, name string) int {
	t.Helper()
	for index, modelName := range models {
		if modelName == name {
			return index
		}
	}
	t.Fatalf("model %q not found in %v", name, models)
	return -1
}
