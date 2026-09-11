package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const hiddenModelGateModel = "gpt-gate-model"

func useMiddlewareHiddenPricingModelsForTest(t *testing.T, hiddenModels string) {
	t.Helper()
	visibility := operation_setting.GetPricingVisibilitySetting()
	original := visibility.HiddenModels
	visibility.HiddenModels = hiddenModels
	t.Cleanup(func() {
		visibility.HiddenModels = original
	})
}

// newHiddenModelGateRouter wires Distribute() behind a stub auth layer so the
// test can pick the identity group per request. It seeds one enabled channel
// for the hidden model in both plg and default so a request that gets past the
// gate always finds a channel and reaches the handler.
func newHiddenModelGateRouter(t *testing.T) *gin.Engine {
	t.Helper()
	require.NoError(t, i18n.Init())
	t.Cleanup(useMiddlewareMemoryChannelConcurrencyForTest(t))
	t.Cleanup(useMiddlewareChannelSelectionDBForTest(t))

	priority := int64(0)
	channel := &model.Channel{
		Id:     9870,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "test-key",
		Name:   "hidden-gate",
		Status: common.ChannelStatusEnabled,
		Models: hiddenModelGateModel,
		Group:  "plg,default",
	}
	require.NoError(t, model.DB.Create(channel).Error)
	for _, group := range []string{"plg", "default"} {
		require.NoError(t, model.DB.Create(&model.Ability{
			ChannelId: channel.Id, Group: group, Model: hiddenModelGateModel, Enabled: true, Priority: &priority,
		}).Error)
	}
	model.InitChannelCache()

	router := gin.New()
	router.Use(BodyStorageCleanup())
	router.Use(func(c *gin.Context) {
		group := c.GetHeader("X-Test-User-Group")
		common.SetContextKey(c, constant.ContextKeyUserGroup, group)
		usingGroup := group
		if usingGroup == "" {
			usingGroup = plgGroup
		}
		common.SetContextKey(c, constant.ContextKeyUsingGroup, usingGroup)
		c.Next()
	})
	router.Use(Distribute())
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Header("X-Selected-Channel", fmt.Sprint(common.GetContextKeyInt(c, constant.ContextKeyChannelId)))
		c.Status(http.StatusOK)
	})
	return router
}

func doHiddenModelGateRequest(t *testing.T, router *gin.Engine, userGroup string, modelName string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(fmt.Sprintf(`{"model":%q}`, modelName)))
	request.Header.Set("Content-Type", "application/json")
	if userGroup != "" {
		request.Header.Set("X-Test-User-Group", userGroup)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func decodeOpenAIErrorEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) (string, string) {
	t.Helper()
	var payload struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload), recorder.Body.String())
	return payload.Error.Message, payload.Error.Code
}

func TestDistributeBlocksPLGIdentityOnHiddenModel(t *testing.T) {
	useMiddlewareHiddenPricingModelsForTest(t, "gpt-gate-*")
	router := newHiddenModelGateRouter(t)

	recorder := doHiddenModelGateRequest(t, router, plgGroup, hiddenModelGateModel)

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	message, code := decodeOpenAIErrorEnvelope(t, recorder)
	require.Equal(t, string(types.ErrorCodeModelNotFound), code)
	require.Contains(t, message, hiddenModelGateModel)
	require.NotContains(t, strings.ToLower(message), "hidden")
	require.Empty(t, recorder.Header().Get("X-Selected-Channel"), "hidden model must be rejected before channel selection")
}

func TestDistributeTreatsEmptyIdentityGroupAsPLGForHiddenModel(t *testing.T) {
	useMiddlewareHiddenPricingModelsForTest(t, hiddenModelGateModel)
	router := newHiddenModelGateRouter(t)

	recorder := doHiddenModelGateRequest(t, router, "", hiddenModelGateModel)

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	_, code := decodeOpenAIErrorEnvelope(t, recorder)
	require.Equal(t, string(types.ErrorCodeModelNotFound), code)
}

func TestDistributeAllowsEnterpriseIdentityOnHiddenModel(t *testing.T) {
	useMiddlewareHiddenPricingModelsForTest(t, "gpt-gate-*")
	router := newHiddenModelGateRouter(t)

	recorder := doHiddenModelGateRequest(t, router, "default", hiddenModelGateModel)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "9870", recorder.Header().Get("X-Selected-Channel"))
}

func TestDistributeAllowsPLGIdentityOnVisibleModel(t *testing.T) {
	useMiddlewareHiddenPricingModelsForTest(t, "some-other-model, secret-*")
	router := newHiddenModelGateRouter(t)

	recorder := doHiddenModelGateRequest(t, router, plgGroup, hiddenModelGateModel)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "9870", recorder.Header().Get("X-Selected-Channel"))
}

func TestDistributeHiddenModelGateIsInertWhenListEmpty(t *testing.T) {
	useMiddlewareHiddenPricingModelsForTest(t, "")
	router := newHiddenModelGateRouter(t)

	recorder := doHiddenModelGateRequest(t, router, plgGroup, hiddenModelGateModel)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
}

// newHiddenModelGatePlaygroundRouter mirrors the /pg/ route stack: UserAuth
// leaves only the session user id (and a possibly stale session group) on the
// context, so the gate must resolve the identity group from the database.
func newHiddenModelGatePlaygroundRouter(t *testing.T, sessionUserID int, sessionGroup string) *gin.Engine {
	t.Helper()
	require.NoError(t, i18n.Init())
	t.Cleanup(useMiddlewareMemoryChannelConcurrencyForTest(t))
	t.Cleanup(useMiddlewareChannelSelectionDBForTest(t))
	require.NoError(t, model.DB.AutoMigrate(&model.User{}))

	priority := int64(0)
	channel := &model.Channel{
		Id:     9871,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "test-key",
		Name:   "hidden-gate-pg",
		Status: common.ChannelStatusEnabled,
		Models: hiddenModelGateModel,
		Group:  "plg,default",
	}
	require.NoError(t, model.DB.Create(channel).Error)
	for _, group := range []string{"plg", "default"} {
		require.NoError(t, model.DB.Create(&model.Ability{
			ChannelId: channel.Id, Group: group, Model: hiddenModelGateModel, Enabled: true, Priority: &priority,
		}).Error)
	}
	model.InitChannelCache()

	router := gin.New()
	router.Use(BodyStorageCleanup())
	router.Use(func(c *gin.Context) {
		c.Set("id", sessionUserID)
		c.Set("group", sessionGroup)
		c.Next()
	})
	router.Use(Distribute())
	router.POST("/pg/chat/completions", func(c *gin.Context) {
		c.Header("X-Selected-Channel", fmt.Sprint(common.GetContextKeyInt(c, constant.ContextKeyChannelId)))
		c.Status(http.StatusOK)
	})
	return router
}

func doHiddenModelGatePlaygroundRequest(t *testing.T, router *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestDistributePlaygroundBlocksPLGUserOnHiddenModel(t *testing.T) {
	useMiddlewareHiddenPricingModelsForTest(t, hiddenModelGateModel)
	common.RedisEnabled = false
	// Session group claims enterprise but the database says plg: the database wins.
	router := newHiddenModelGatePlaygroundRouter(t, 9101, "default")
	require.NoError(t, model.DB.Create(&model.User{
		Id: 9101, Username: "pg-plg-user", Password: "password123", DisplayName: "PLG", Group: plgGroup, Status: common.UserStatusEnabled,
	}).Error)

	recorder := doHiddenModelGatePlaygroundRequest(t, router, fmt.Sprintf(`{"model":%q,"group":"plg"}`, hiddenModelGateModel))

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	_, code := decodeOpenAIErrorEnvelope(t, recorder)
	require.Equal(t, string(types.ErrorCodeModelNotFound), code)
	require.Empty(t, recorder.Header().Get("X-Selected-Channel"))
}

func TestDistributePlaygroundAllowsEnterpriseUserOnHiddenModel(t *testing.T) {
	useMiddlewareHiddenPricingModelsForTest(t, hiddenModelGateModel)
	common.RedisEnabled = false
	// Session group is stale (plg) but the database says enterprise: allowed.
	router := newHiddenModelGatePlaygroundRouter(t, 9102, plgGroup)
	require.NoError(t, model.DB.Create(&model.User{
		Id: 9102, Username: "pg-enterprise-user", Password: "password123", DisplayName: "Enterprise", Group: "default", Status: common.UserStatusEnabled,
	}).Error)

	recorder := doHiddenModelGatePlaygroundRequest(t, router, fmt.Sprintf(`{"model":%q,"group":"default"}`, hiddenModelGateModel))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "9871", recorder.Header().Get("X-Selected-Channel"))
}

func TestDistributePlaygroundFailsClosedWhenUserGroupLookupFails(t *testing.T) {
	useMiddlewareHiddenPricingModelsForTest(t, hiddenModelGateModel)
	common.RedisEnabled = false
	// No user row for the session id: the gate must treat the identity as plg.
	router := newHiddenModelGatePlaygroundRouter(t, 9103, "default")

	recorder := doHiddenModelGatePlaygroundRequest(t, router, fmt.Sprintf(`{"model":%q,"group":"default"}`, hiddenModelGateModel))

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	require.Empty(t, recorder.Header().Get("X-Selected-Channel"))
}
