package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const (
	hiddenGateVisibleModel = "zz-gate-visible"
	hiddenGateHiddenModel  = "zz-gate-hidden"
)

func listModelsForIdentity(t *testing.T, identityGroup string) map[string]struct{} {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, identityGroup)

	ListModels(ctx, constant.ChannelTypeOpenAI)

	return decodeListModelsResponse(t, recorder)
}

func availableModelsForIdentity(t *testing.T, identityGroup string) map[string]struct{} {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/available_models", nil)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, identityGroup)
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, identityGroup)

	AvailableModels(ctx)

	return decodeAvailableModelsResponse(t, recorder)
}

func retrieveModelForIdentity(t *testing.T, identityGroup string, modelID string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models/"+modelID, nil)
	ctx.Params = gin.Params{{Key: "model", Value: modelID}}
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, identityGroup)

	RetrieveModel(ctx, constant.ChannelTypeOpenAI)

	return recorder
}

func TestListModelsExcludesHiddenModelsForPLGIdentity(t *testing.T) {
	withSelfUseModeEnabled(t)
	withControllerHiddenPricingModels(t, hiddenGateHiddenModel)
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "plg", Model: hiddenGateVisibleModel, ChannelId: 1, Enabled: true},
		{Group: "plg", Model: hiddenGateHiddenModel, ChannelId: 1, Enabled: true},
	}).Error)

	ids := listModelsForIdentity(t, "plg")

	require.Contains(t, ids, hiddenGateVisibleModel)
	require.NotContains(t, ids, hiddenGateHiddenModel)
}

func TestListModelsKeepsHiddenModelsForEnterpriseIdentity(t *testing.T) {
	withSelfUseModeEnabled(t)
	withControllerHiddenPricingModels(t, hiddenGateHiddenModel)
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: hiddenGateVisibleModel, ChannelId: 1, Enabled: true},
		{Group: "default", Model: hiddenGateHiddenModel, ChannelId: 1, Enabled: true},
	}).Error)

	ids := listModelsForIdentity(t, "default")

	require.Contains(t, ids, hiddenGateVisibleModel)
	require.Contains(t, ids, hiddenGateHiddenModel)
}

func TestAvailableModelsExcludesHiddenModelsForPLGIdentity(t *testing.T) {
	withSelfUseModeEnabled(t)
	withControllerHiddenPricingModels(t, "zz-gate-hid*")
	db := setupModelListControllerTestDB(t)
	createAvailableModelFixture(t, db, 91101, common.ChannelStatusEnabled, map[string][]string{
		"plg": {hiddenGateVisibleModel, hiddenGateHiddenModel},
	})

	ids := availableModelsForIdentity(t, "plg")

	require.Contains(t, ids, hiddenGateVisibleModel)
	require.NotContains(t, ids, hiddenGateHiddenModel)
}

func TestAvailableModelsKeepsHiddenModelsForEnterpriseIdentity(t *testing.T) {
	withSelfUseModeEnabled(t)
	withControllerHiddenPricingModels(t, "zz-gate-hid*")
	db := setupModelListControllerTestDB(t)
	createAvailableModelFixture(t, db, 91102, common.ChannelStatusEnabled, map[string][]string{
		"default": {hiddenGateVisibleModel, hiddenGateHiddenModel},
	})

	ids := availableModelsForIdentity(t, "default")

	require.Contains(t, ids, hiddenGateVisibleModel)
	require.Contains(t, ids, hiddenGateHiddenModel)
}

func TestRetrieveModelHidesHiddenModelFromPLGIdentity(t *testing.T) {
	withControllerHiddenPricingModels(t, "gpt-4o")

	hidden := retrieveModelForIdentity(t, "plg", "gpt-4o")
	unknown := retrieveModelForIdentity(t, "plg", "zz-no-such-model")

	// A hidden model must be indistinguishable from an unknown one: same status,
	// same envelope, and no trace of the model id in the success shape.
	require.Equal(t, unknown.Code, hidden.Code)
	var hiddenPayload, unknownPayload struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Param   string `json:"param"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(hidden.Body.Bytes(), &hiddenPayload), hidden.Body.String())
	require.NoError(t, common.Unmarshal(unknown.Body.Bytes(), &unknownPayload), unknown.Body.String())
	require.Equal(t, "model_not_found", hiddenPayload.Error.Code)
	require.Equal(t, unknownPayload.Error.Type, hiddenPayload.Error.Type)
	require.Equal(t, unknownPayload.Error.Param, hiddenPayload.Error.Param)
	require.Equal(t, fmt.Sprintf("The model '%s' does not exist", "gpt-4o"), hiddenPayload.Error.Message)
	require.NotContains(t, hidden.Body.String(), `"id"`)
}

func TestRetrieveModelKeepsHiddenModelForEnterpriseIdentity(t *testing.T) {
	withControllerHiddenPricingModels(t, "gpt-4o")

	recorder := retrieveModelForIdentity(t, "default", "gpt-4o")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"id":"gpt-4o"`)
	require.NotContains(t, recorder.Body.String(), "model_not_found")
}
