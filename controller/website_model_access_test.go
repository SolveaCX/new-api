package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWebsiteModelAccessMatchesPublicAccountAndIgnoresRequestedIdentity(t *testing.T) {
	withSelfUseModeEnabled(t)
	withControllerHiddenPricingModels(t, "secret-*")
	withControllerModelAccessGroups(t, map[string]float64{"plg": 1, "vip": 2}, map[string]string{"vip": "Private"}, []string{"vip"})
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.ModelAvailabilityState{}))
	require.NoError(t, db.Create(&model.User{Id: 507, Username: "public-catalog", Password: "password", Group: "plg", Status: common.UserStatusEnabled}).Error)
	createAvailableModelFixture(t, db, 507, common.ChannelStatusEnabled, map[string][]string{
		"plg": {"public-model", "secret-model"},
		"vip": {"private-model"},
	})
	_, expected := requestUserModelAccessAtPath(t, 507, "/api/user/model-access?view=available_models")

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/website/model-access?group=vip&user_id=999", nil)
	GetWebsiteModelAccess(ctx)
	var actual userModelAccessResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &actual))
	require.True(t, actual.Success)
	require.Equal(t, expected.Data, actual.Data)
	require.Equal(t, []string{"public-model"}, actual.Data.AccountModelIDs)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
}
