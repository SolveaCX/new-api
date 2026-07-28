package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const autoModelTestCandidate = "auto-discovery-candidate"

func withEnabledAutoModelSnapshot(t *testing.T) {
	t.Helper()
	credentialVersion := "0123456789abcdef0123456789abcdef"
	config := model_setting.AutoModelConfig{
		Version:                 model_setting.AutoModelConfigVersion,
		Enabled:                 true,
		ClassifierBaseURL:       "https://classifier.example.com/v1",
		ClassifierModel:         "classifier-model",
		ClassifierTimeoutMS:     800,
		ClassifierInputMaxChars: 8000,
		DefaultModel:            autoModelTestCandidate,
		CredentialVersion:       credentialVersion,
		Routes: map[string][]string{
			"general":     {autoModelTestCandidate, "auto-candidate-two"},
			"coding":      {"auto-candidate-three"},
			"reasoning":   {"auto-candidate-four"},
			"translation": {"auto-candidate-five"},
		},
	}
	configRaw, err := common.Marshal(config)
	require.NoError(t, err)
	credentialRaw, err := common.Marshal(model_setting.AutoModelCredential{
		Version: credentialVersion,
		APIKey:  "sk-auto-discovery-test",
	})
	require.NoError(t, err)
	require.NoError(t, model_setting.ReloadAutoModelSnapshot(string(configRaw), string(credentialRaw)))
	originalMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCache
		require.NoError(t, model_setting.ReloadAutoModelSnapshot("", ""))
	})
}

func setupAutoModelDiscoveryFixture(t *testing.T, userID int) {
	t.Helper()
	withSelfUseModeEnabled(t)
	withEnabledAutoModelSnapshot(t)
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id: userID, Username: "auto-discovery-user", Password: "password",
		Group: "default", Status: common.UserStatusEnabled,
	}).Error)
	createAvailableModelFixture(t, db, userID, common.ChannelStatusEnabled, map[string][]string{
		"default": {autoModelTestCandidate},
	})
}

func newAutoModelTokenContext(t *testing.T, target string, limitsEnabled bool, limits map[string]bool) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, limitsEnabled)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, limits)
	return ctx, recorder
}

func TestAutoModelDiscoveryProtocolShapesAndGeminiExclusion(t *testing.T) {
	setupAutoModelDiscoveryFixture(t, 94001)

	openAIContext, openAIRecorder := newAutoModelTokenContext(t, "/v1/models", false, nil)
	ListModels(openAIContext, constant.ChannelTypeOpenAI)
	var openAI listModelsResponse
	require.NoError(t, common.Unmarshal(openAIRecorder.Body.Bytes(), &openAI))
	require.Len(t, openAI.Data, 2)
	var virtual dto.OpenAIModels
	for _, item := range openAI.Data {
		if item.Id == autoModelID {
			virtual = item
		}
	}
	require.Equal(t, "new-api", virtual.OwnedBy)
	require.Equal(t, autoModelSupportedEndpointTypes, virtual.SupportedEndpointTypes)

	anthropicContext, anthropicRecorder := newAutoModelTokenContext(t, "/v1/models", false, nil)
	ListModels(anthropicContext, constant.ChannelTypeAnthropic)
	var anthropic struct {
		Data    []dto.AnthropicModel `json:"data"`
		FirstID string               `json:"first_id"`
		LastID  string               `json:"last_id"`
	}
	require.NoError(t, common.Unmarshal(anthropicRecorder.Body.Bytes(), &anthropic))
	require.Equal(t, autoModelID, anthropic.Data[len(anthropic.Data)-1].ID)
	require.NotEmpty(t, anthropic.FirstID)
	require.Equal(t, autoModelID, anthropic.LastID)

	geminiContext, geminiRecorder := newAutoModelTokenContext(t, "/v1beta/models", false, nil)
	ListModels(geminiContext, constant.ChannelTypeGemini)
	require.NotContains(t, geminiRecorder.Body.String(), `"displayName":"auto"`)
}

func TestAutoModelDiscoveryRequiresVirtualAndRealTokenPermissions(t *testing.T) {
	setupAutoModelDiscoveryFixture(t, 94002)

	tests := []struct {
		name    string
		limits  map[string]bool
		visible bool
	}{
		{name: "both", limits: map[string]bool{autoModelID: true, autoModelTestCandidate: true}, visible: true},
		{name: "virtual denied", limits: map[string]bool{autoModelTestCandidate: true}, visible: false},
		{name: "real denied", limits: map[string]bool{autoModelID: true}, visible: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, recorder := newAutoModelTokenContext(t, "/v1/available_models", true, test.limits)
			AvailableModels(ctx)
			ids := decodeAvailableModelsResponse(t, recorder)
			if test.visible {
				require.Contains(t, ids, autoModelID)
			} else {
				require.NotContains(t, ids, autoModelID)
			}
		})
	}
}

func TestAutoModelRetrieveUsesListVisibility(t *testing.T) {
	setupAutoModelDiscoveryFixture(t, 94003)

	visibleContext, visibleRecorder := newAutoModelTokenContext(t, "/v1/models/auto", true, map[string]bool{
		autoModelID:            true,
		autoModelTestCandidate: true,
	})
	visibleContext.Params = gin.Params{{Key: "model", Value: autoModelID}}
	RetrieveModel(visibleContext, constant.ChannelTypeOpenAI)
	var visible dto.OpenAIModels
	require.NoError(t, common.Unmarshal(visibleRecorder.Body.Bytes(), &visible))
	require.Equal(t, autoModelID, visible.Id)
	require.Equal(t, "new-api", visible.OwnedBy)

	hiddenContext, hiddenRecorder := newAutoModelTokenContext(t, "/v1/models/auto", true, map[string]bool{autoModelID: true})
	hiddenContext.Params = gin.Params{{Key: "model", Value: autoModelID}}
	RetrieveModel(hiddenContext, constant.ChannelTypeOpenAI)
	var hidden struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(hiddenRecorder.Body.Bytes(), &hidden))
	require.Equal(t, "model_not_found", hidden.Error.Code)
}

func TestAutoModelDiscoveryFailsClosedOnRealNameConflict(t *testing.T) {
	setupAutoModelDiscoveryFixture(t, 94005)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group: "default", Model: autoModelID, ChannelId: 94005, Enabled: true,
	}).Error)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 94005).
		Update("models", autoModelTestCandidate+","+autoModelID).Error)

	ctx, recorder := newAutoModelTokenContext(t, "/v1/available_models", false, nil)
	AvailableModels(ctx)
	var payload availableModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	autoItems := make([]dto.OpenAIModels, 0, 1)
	for _, item := range payload.Data {
		if item.Id == autoModelID {
			autoItems = append(autoItems, item)
		}
	}
	require.Len(t, autoItems, 1, "the real model remains visible without a duplicate virtual model")
	require.NotEqual(t, "new-api", autoItems[0].OwnedBy)
}

func TestAutoModelDiscoveryUsesConcreteAutoGroups(t *testing.T) {
	withSelfUseModeEnabled(t)
	withEnabledAutoModelSnapshot(t)
	withModelListGroupSettings(t,
		map[string]string{"default": "Default", "vip": "VIP", "auto": "Automatic"},
		[]string{"vip"},
	)
	db := setupModelListControllerTestDB(t)
	createAvailableModelFixture(t, db, 94006, common.ChannelStatusEnabled, map[string][]string{
		"vip": {autoModelTestCandidate},
	})

	ctx, recorder := newAutoModelTokenContext(t, "/v1/available_models", true, map[string]bool{
		autoModelID:            true,
		autoModelTestCandidate: true,
	})
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, autoModelID)
	AvailableModels(ctx)
	require.Contains(t, decodeAvailableModelsResponse(t, recorder), autoModelID)
}

func TestAutoModelDiscoveryRejectsOfficiallyUnsupportedCandidate(t *testing.T) {
	setupAutoModelDiscoveryFixture(t, 94007)
	require.NoError(t, model.DB.Create(&model.ModelAvailabilityState{
		ModelName: autoModelTestCandidate,
		Status:    model.ModelAvailabilityOfficialUnsupported,
	}).Error)

	ctx, recorder := newAutoModelTokenContext(t, "/v1/available_models", false, nil)
	AvailableModels(ctx)
	require.NotContains(t, decodeAvailableModelsResponse(t, recorder), autoModelID)
}

func TestAutoModelUserDiscoveryAddsVirtualAccessWithoutTokenContext(t *testing.T) {
	setupAutoModelDiscoveryFixture(t, 94004)

	models := requestUserModels(t, "/api/user/models?group=default", 94004)
	require.Contains(t, models, autoModelID)

	_, payload := requestUserModelAccess(t, 94004)
	require.Contains(t, payload.Data.IdentityModelIDs, autoModelID)
	metadata := make(map[string]service.ModelAccessModel, len(payload.Data.Models))
	for _, item := range payload.Data.Models {
		metadata[item.ID] = item
	}
	require.Equal(t, autoModelSupportedEndpointTypes, metadata[autoModelID].SupportedEndpointTypes)
	require.Nil(t, metadata[autoModelID].Vendor)
}

func TestAnthropicModelDiscoveryHandlesEmptyList(t *testing.T) {
	withSelfUseModeEnabled(t)
	setupModelListControllerTestDB(t)
	originalMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCache })
	require.NoError(t, model_setting.ReloadAutoModelSnapshot("", ""))

	ctx, recorder := newAutoModelTokenContext(t, "/v1/models", true, map[string]bool{})
	ListModels(ctx, constant.ChannelTypeAnthropic)
	var payload struct {
		Data    []dto.AnthropicModel `json:"data"`
		FirstID string               `json:"first_id"`
		LastID  string               `json:"last_id"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Empty(t, payload.Data)
	require.Empty(t, payload.FirstID)
	require.Empty(t, payload.LastID)
}
