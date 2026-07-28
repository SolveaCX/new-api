package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	backendi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	automodel "github.com/QuantumNous/new-api/service/auto_model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type countingAutoModelClassifier struct {
	calls int
	route automodel.Route
	err   error
}

func (c *countingAutoModelClassifier) Classify(context.Context, *model_setting.AutoModelSnapshot, string) (automodel.Route, error) {
	c.calls++
	return c.route, c.err
}

func TestValidateAutoModelTopLevelFailClosed(t *testing.T) {
	tests := []struct {
		name     string
		protocol automodel.Protocol
		body     string
		wantErr  bool
	}{
		{name: "chat allowed explicit zero", protocol: automodel.ProtocolChatCompletions, body: `{"model":"auto","messages":[{"role":"user","content":"hi"}],"temperature":0,"stream":false}`},
		{name: "chat unknown", protocol: automodel.ProtocolChatCompletions, body: `{"model":"auto","messages":[],"reasoning_effort":"high"}`, wantErr: true},
		{name: "responses store false", protocol: automodel.ProtocolResponses, body: `{"model":"auto","input":"hi","store":false}`},
		{name: "responses store true", protocol: automodel.ProtocolResponses, body: `{"model":"auto","input":"hi","store":true}`, wantErr: true},
		{name: "responses store null", protocol: automodel.ProtocolResponses, body: `{"model":"auto","input":"hi","store":null}`, wantErr: true},
		{name: "messages tool", protocol: automodel.ProtocolMessages, body: `{"model":"auto","messages":[],"tools":[]}`, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateAutoModelTopLevel(test.protocol, []byte(test.body))
			if test.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAutoModelSensitiveTextMatchesResponsesTokenMeta(t *testing.T) {
	text, err := autoModelSensitiveText(automodel.ProtocolResponses, []byte(`{
		"model":"auto","input":"safe","metadata":{"note":"test_sensitive"}
	}`))
	require.NoError(t, err)
	require.Contains(t, text, "test_sensitive")
}

func TestApplyAutoModelResolutionPublishesOnlyFinalFixedGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newContext := func(t *testing.T) (*gin.Context, *ModelRequest, []byte) {
		t.Helper()
		raw := []byte(`{"model":"auto","messages":[{"role":"user","content":"hi"}],"temperature":0,"stream":false,"future":{"enabled":false}}`)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(raw)))
		c.Request.Header.Set("Content-Type", "application/json")
		_, err := common.GetBodyStorage(c)
		require.NoError(t, err)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "auto")
		return c, &ModelRequest{Model: "auto"}, raw
	}

	c, request, raw := newContext(t)
	err := applyAutoModelResolution(c, request, raw, &autoModelResolution{
		pair:     autoModelPair{model: "real-model", group: "vip", groupIndex: 1},
		protocol: autoModelProtocol{metric: "chat"}, decisionSource: "classifier",
	})
	require.Nil(t, err)
	require.Equal(t, "real-model", request.Model)
	require.Equal(t, 1, common.GetContextKeyInt(c, constant.ContextKeyAutoGroupIndex))
	_, exists := common.GetContextKey(c, constant.ContextKeyAutoGroup)
	require.False(t, exists, "non-fixed selection must publish the actual group only after channel selection")
	rewrittenStorage, bodyErr := common.GetBodyStorage(c)
	require.NoError(t, bodyErr)
	rewritten, bodyErr := rewrittenStorage.Bytes()
	require.NoError(t, bodyErr)
	require.JSONEq(t, `{"model":"real-model","messages":[{"role":"user","content":"hi"}],"temperature":0,"stream":false,"future":{"enabled":false}}`, string(rewritten))

	fixedContext, fixedRequest, fixedRaw := newContext(t)
	err = applyAutoModelResolution(fixedContext, fixedRequest, fixedRaw, &autoModelResolution{
		pair:     autoModelPair{model: "real-model", group: "vip", groupIndex: 1},
		protocol: autoModelProtocol{metric: "chat"}, decisionSource: "classifier", fixedChannel: true,
	})
	require.Nil(t, err)
	require.Equal(t, "vip", common.GetContextKeyString(fixedContext, constant.ContextKeyAutoGroup))
}

func TestAbortWithAutoModelErrorUsesAnthropicEnvelope(t *testing.T) {
	require.NoError(t, backendi18n.Init())
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	abortWithAutoModelError(c, autoModelProtocol{metric: "messages", messages: true}, automodel.UnsupportedRequestError(c))

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var response struct {
		Type  string `json:"type"`
		Error struct {
			Type string `json:"type"`
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "error", response.Type)
	require.Equal(t, "invalid_request_error", response.Error.Type)
	require.Equal(t, string(types.ErrorCodeAutoModelUnsupportedRequest), response.Error.Code)
}

func TestGetModelRequestCompactPreservesVirtualAutoBeforeResolution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", strings.NewReader(`{"model":"auto","input":"hi"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	request, selectChannel, err := getModelRequest(c)
	require.NoError(t, err)
	require.True(t, selectChannel)
	require.Equal(t, "auto", request.Model)
}

func setupAutoModelResolverTest(t *testing.T, status int, abilityEnabled bool) {
	t.Helper()
	require.NoError(t, backendi18n.Init())
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalIsMaster := common.IsMasterNode
	originalSQLitePath := common.SQLitePath
	originalRedis := common.RedisEnabled
	originalMemoryCache := common.MemoryCacheEnabled
	originalSQLite := common.UsingSQLite
	originalMySQL := common.UsingMySQL
	originalPostgreSQL := common.UsingPostgreSQL
	originalRatios := ratio_setting.ModelRatio2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	originalUsable := setting.UserUsableGroups2JSONString()
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalClassifier := defaultAutoModelClassifier
	originalSensitiveWords := append([]string(nil), setting.SensitiveWords...)
	originalCheckSensitive := setting.CheckSensitiveEnabled
	originalCheckPrompt := setting.CheckSensitiveOnPromptEnabled
	originalDSN, hadDSN := os.LookupEnv("SQL_DSN")

	common.IsMasterNode = false
	common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	require.NoError(t, os.Setenv("SQL_DSN", "local"))
	require.NoError(t, model.InitDB())
	db := model.DB
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}, &model.ModelAvailabilityState{}))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`[]`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1}`))
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"model-a":1,"model-b":1,"model-c":1,"model-d":1,"model-e":1}`))
	priority := int64(0)
	weight := uint(100)
	require.NoError(t, db.Create(&model.Channel{
		Id: 99501, Type: constant.ChannelTypeOpenAI, Status: status, Key: "test-key",
		Models: "model-a,model-b,model-c,model-d,model-e", Group: "default", Priority: &priority, Weight: &weight,
	}).Error)
	abilities := make([]model.Ability, 0, 5)
	for _, modelName := range []string{"model-a", "model-b", "model-c", "model-d", "model-e"} {
		abilities = append(abilities, model.Ability{Group: "default", Model: modelName, ChannelId: 99501, Enabled: abilityEnabled, Priority: &priority, Weight: weight})
	}
	require.NoError(t, db.Create(&abilities).Error)

	config := model_setting.AutoModelConfig{
		Version: model_setting.AutoModelConfigVersion, Enabled: true,
		ClassifierBaseURL: "https://classifier.example.com/v1", ClassifierModel: "classifier",
		ClassifierTimeoutMS: 800, ClassifierInputMaxChars: 8000, DefaultModel: "model-a",
		Routes: map[string][]string{
			"general": {"model-a", "model-b"}, "coding": {"model-c"},
			"reasoning": {"model-d"}, "translation": {"model-e"},
		},
		CredentialVersion: "test-version",
	}
	configRaw, err := common.Marshal(config)
	require.NoError(t, err)
	credentialRaw, err := model_setting.MarshalAutoModelCredential(model_setting.AutoModelCredential{Version: "test-version", APIKey: "test-key"})
	require.NoError(t, err)
	require.NoError(t, model_setting.ReloadAutoModelSnapshot(string(configRaw), credentialRaw))

	t.Cleanup(func() {
		defaultAutoModelClassifier = originalClassifier
		setting.SensitiveWords = originalSensitiveWords
		setting.CheckSensitiveEnabled = originalCheckSensitive
		setting.CheckSensitiveOnPromptEnabled = originalCheckPrompt
		require.NoError(t, model_setting.ReloadAutoModelSnapshot("", ""))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalRatios))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsable))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.IsMasterNode = originalIsMaster
		common.SQLitePath = originalSQLitePath
		common.RedisEnabled = originalRedis
		common.MemoryCacheEnabled = originalMemoryCache
		common.UsingSQLite = originalSQLite
		common.UsingMySQL = originalMySQL
		common.UsingPostgreSQL = originalPostgreSQL
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
		if hadDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	})
}

func newAutoModelResolverContext(path, body string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
	return c
}

func TestResolveAutoModelDeclaredButDisabledReturns503WithoutClassifier(t *testing.T) {
	setupAutoModelResolverTest(t, common.ChannelStatusManuallyDisabled, false)
	classifier := &countingAutoModelClassifier{route: automodel.RouteGeneral}
	defaultAutoModelClassifier = classifier
	body := `{"model":"auto","messages":[{"role":"user","content":"safe"}]}`
	c := newAutoModelResolverContext("/v1/chat/completions", body)

	resolution, virtual, apiErr := resolveAutoModel(c, []byte(body), nil)
	require.True(t, virtual)
	require.Nil(t, resolution)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
	require.Equal(t, types.ErrorCodeModelNotFound, apiErr.GetErrorCode())
	require.Zero(t, classifier.calls)
}

func TestResolveAutoModelResponsesMetadataSensitiveSkipsClassifier(t *testing.T) {
	setupAutoModelResolverTest(t, common.ChannelStatusEnabled, true)
	classifier := &countingAutoModelClassifier{route: automodel.RouteGeneral}
	defaultAutoModelClassifier = classifier
	setting.SensitiveWords = []string{"test_sensitive"}
	setting.CheckSensitiveEnabled = true
	setting.CheckSensitiveOnPromptEnabled = true
	body := `{"model":"auto","input":"safe","metadata":{"note":"test_sensitive"},"store":false}`
	c := newAutoModelResolverContext("/v1/responses", body)

	resolution, virtual, apiErr := resolveAutoModel(c, []byte(body), nil)
	require.True(t, virtual)
	require.Nil(t, resolution)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeSensitiveWordsDetected, apiErr.GetErrorCode())
	require.Zero(t, classifier.calls)
}

func TestAutoModelDisabledAndUnsupportedProtocolEnvelopesSkipClassifier(t *testing.T) {
	protocols := []struct {
		name     string
		path     string
		body     string
		messages bool
	}{
		{name: "chat", path: "/v1/chat/completions", body: `{"model":"auto","messages":[{"role":"user","content":"safe"}]}`},
		{name: "responses", path: "/v1/responses", body: `{"model":"auto","input":"safe"}`},
		{name: "messages", path: "/v1/messages", body: `{"model":"auto","max_tokens":16,"messages":[{"role":"user","content":"safe"}]}`, messages: true},
	}
	for _, test := range protocols {
		t.Run(test.name+" disabled", func(t *testing.T) {
			setupAutoModelResolverTest(t, common.ChannelStatusEnabled, true)
			require.NoError(t, model_setting.ReloadAutoModelSnapshot("", ""))
			classifier := &countingAutoModelClassifier{route: automodel.RouteGeneral}
			defaultAutoModelClassifier = classifier
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
			common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
			common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
			_, virtual, apiErr := resolveAutoModel(c, []byte(test.body), nil)
			require.True(t, virtual)
			require.Equal(t, types.ErrorCodeAutoModelDisabled, apiErr.GetErrorCode())
			abortWithAutoModelError(c, autoModelProtocolForError(c.Request), apiErr)
			require.Zero(t, classifier.calls)
			assertAutoModelErrorEnvelope(t, recorder, test.messages, types.ErrorCodeAutoModelDisabled)
		})
		t.Run(test.name+" unsupported", func(t *testing.T) {
			setupAutoModelResolverTest(t, common.ChannelStatusEnabled, true)
			classifier := &countingAutoModelClassifier{route: automodel.RouteGeneral}
			defaultAutoModelClassifier = classifier
			body := strings.TrimSuffix(test.body, "}") + `,"tools":[]}`
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(body))
			common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
			common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
			common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
			_, virtual, apiErr := resolveAutoModel(c, []byte(body), nil)
			require.True(t, virtual)
			require.Equal(t, types.ErrorCodeAutoModelUnsupportedRequest, apiErr.GetErrorCode())
			abortWithAutoModelError(c, autoModelProtocolForError(c.Request), apiErr)
			require.Zero(t, classifier.calls)
			assertAutoModelErrorEnvelope(t, recorder, test.messages, types.ErrorCodeAutoModelUnsupportedRequest)
		})
	}
}

func TestAutoModelColdStartConfigMismatchReturnsConfigInvalid(t *testing.T) {
	setupAutoModelResolverTest(t, common.ChannelStatusEnabled, true)
	require.NoError(t, model_setting.ReloadAutoModelSnapshot("", ""))
	cfg := model_setting.AutoModelConfig{
		Version: model_setting.AutoModelConfigVersion, Enabled: true,
		ClassifierBaseURL: "https://classifier.example.com/v1", ClassifierModel: "classifier",
		ClassifierTimeoutMS: 800, ClassifierInputMaxChars: 8000, DefaultModel: "model-a",
		Routes: map[string][]string{
			"general": {"model-a", "model-b"}, "coding": {"model-c"},
			"reasoning": {"model-d"}, "translation": {"model-e"},
		},
		CredentialVersion: "config-version",
	}
	configRaw, err := common.Marshal(cfg)
	require.NoError(t, err)
	credentialRaw, err := model_setting.MarshalAutoModelCredential(model_setting.AutoModelCredential{
		Version: "credential-version",
		APIKey:  "sk-mismatch",
	})
	require.NoError(t, err)
	require.Error(t, model_setting.ReloadAutoModelSnapshot(string(configRaw), credentialRaw))

	classifier := &countingAutoModelClassifier{route: automodel.RouteGeneral}
	defaultAutoModelClassifier = classifier
	body := `{"model":"auto","messages":[{"role":"user","content":"safe"}]}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")

	resolution, virtual, apiErr := resolveAutoModel(c, []byte(body), nil)
	require.True(t, virtual)
	require.Nil(t, resolution)
	require.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
	require.Equal(t, types.ErrorCodeAutoModelConfigInvalid, apiErr.GetErrorCode())
	require.Zero(t, classifier.calls)
}

func TestAutoModelNestedExtensionsAreRejectedBeforeClassification(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		body        string
		channelType int
	}{
		{
			name: "chat message provider extension",
			path: "/v1/chat/completions",
			body: `{"model":"auto","messages":[{"role":"user","content":"hello","provider":"custom"}]}`,
		},
		{
			name: "responses content cache extension",
			path: "/v1/responses",
			body: `{"model":"auto","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello","cache_control":{"type":"ephemeral"}}]}]}`,
		},
		{
			name:        "messages content cache extension",
			path:        "/v1/messages",
			body:        `{"model":"auto","max_tokens":16,"messages":[{"role":"user","content":[{"type":"text","text":"hello","cache_control":{"type":"ephemeral"}}]}]}`,
			channelType: constant.ChannelTypeAnthropic,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAutoModelResolverTest(t, common.ChannelStatusEnabled, true)
			if test.channelType != 0 {
				require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 99501).Update("type", test.channelType).Error)
			}
			classifier := &countingAutoModelClassifier{route: automodel.RouteGeneral}
			defaultAutoModelClassifier = classifier
			c := newAutoModelResolverContext(test.path, test.body)

			resolution, virtual, apiErr := resolveAutoModel(c, []byte(test.body), nil)
			require.True(t, virtual)
			require.Nil(t, resolution)
			require.Equal(t, types.ErrorCodeAutoModelUnsupportedRequest, apiErr.GetErrorCode())
			require.Zero(t, classifier.calls)
		})
	}
}

func assertAutoModelErrorEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, messages bool, code types.ErrorCode) {
	t.Helper()
	var payload map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	errorPayload, ok := payload["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, string(code), errorPayload["code"])
	if messages {
		require.Equal(t, "error", payload["type"])
		require.Equal(t, "invalid_request_error", errorPayload["type"])
	} else {
		require.Equal(t, "new_api_error", errorPayload["type"])
	}
}

func TestResolveAutoModelClassificationAndFallbackMatrix(t *testing.T) {
	tests := []struct {
		name         string
		classifier   *countingAutoModelClassifier
		disableModel string
		wantModel    string
		wantDecision string
		wantCode     types.ErrorCode
	}{
		{name: "route success", classifier: &countingAutoModelClassifier{route: automodel.RouteCoding}, wantModel: "model-c", wantDecision: "classifier"},
		{name: "classifier error defaults", classifier: &countingAutoModelClassifier{err: errors.New("classifier unavailable")}, wantModel: "model-a", wantDecision: "default"},
		{name: "route unavailable defaults", classifier: &countingAutoModelClassifier{route: automodel.RouteCoding}, disableModel: "model-c", wantModel: "model-a", wantDecision: "default"},
		{name: "default unavailable rejects", classifier: &countingAutoModelClassifier{err: errors.New("classifier unavailable")}, disableModel: "model-a", wantCode: types.ErrorCodeAutoModelNoEligibleCandidate},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAutoModelResolverTest(t, common.ChannelStatusEnabled, true)
			if test.disableModel != "" {
				require.NoError(t, model.DB.Model(&model.Ability{}).Where("model = ?", test.disableModel).Update("enabled", false).Error)
			}
			defaultAutoModelClassifier = test.classifier
			body := `{"model":"auto","messages":[{"role":"user","content":"safe"}]}`
			resolution, virtual, apiErr := resolveAutoModel(newAutoModelResolverContext("/v1/chat/completions", body), []byte(body), nil)
			require.True(t, virtual)
			require.Equal(t, 1, test.classifier.calls)
			if test.wantCode != "" {
				require.Nil(t, resolution)
				require.Equal(t, test.wantCode, apiErr.GetErrorCode())
				return
			}
			require.Nil(t, apiErr)
			require.Equal(t, test.wantModel, resolution.pair.model)
			require.Equal(t, test.wantDecision, resolution.decisionSource)
		})
	}
}

func TestResolveAutoModelTokenPermissionMatrixIncludingFixedChannel(t *testing.T) {
	tests := []struct {
		name      string
		limits    map[string]bool
		fixed     bool
		wantCode  types.ErrorCode
		wantCalls int
	}{
		{name: "only virtual", limits: map[string]bool{"auto": true}, wantCode: types.ErrorCodeAutoModelNoEligibleCandidate},
		{name: "only real", limits: map[string]bool{"model-a": true, "model-b": true, "model-c": true, "model-d": true, "model-e": true}, wantCode: types.ErrorCodeAccessDenied},
		{name: "both", limits: map[string]bool{"auto": true, "model-a": true, "model-b": true, "model-c": true, "model-d": true, "model-e": true}, wantCalls: 1},
		{name: "fixed both", fixed: true, limits: map[string]bool{"auto": true, "model-a": true, "model-b": true, "model-c": true, "model-d": true, "model-e": true}, wantCalls: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAutoModelResolverTest(t, common.ChannelStatusEnabled, true)
			classifier := &countingAutoModelClassifier{route: automodel.RouteGeneral}
			defaultAutoModelClassifier = classifier
			body := `{"model":"auto","messages":[{"role":"user","content":"safe"}]}`
			c := newAutoModelResolverContext("/v1/chat/completions", body)
			common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
			common.SetContextKey(c, constant.ContextKeyTokenModelLimit, test.limits)
			var fixedChannel *model.Channel
			if test.fixed {
				var err error
				fixedChannel, err = model.GetChannelById(99501, true)
				require.NoError(t, err)
			}
			resolution, virtual, apiErr := resolveAutoModel(c, []byte(body), fixedChannel)
			require.True(t, virtual)
			require.Equal(t, test.wantCalls, classifier.calls)
			if test.wantCode != "" {
				require.Nil(t, resolution)
				require.Equal(t, test.wantCode, apiErr.GetErrorCode())
				return
			}
			require.Nil(t, apiErr)
			require.NotNil(t, resolution)
			require.Equal(t, test.fixed, resolution.fixedChannel)
		})
	}
}

func TestResolveAutoModelAutoGroupPreselectsAndSelectorNeverMovesBackward(t *testing.T) {
	setupAutoModelResolverTest(t, common.ChannelStatusManuallyDisabled, true)
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP","auto":"Auto"}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["default","vip"]`))
	priority := int64(0)
	weight := uint(100)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 99502, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "vip-key", Models: "model-a,model-b,model-c,model-d,model-e", Group: "vip", Priority: &priority, Weight: &weight}).Error)
	for _, modelName := range []string{"model-a", "model-b", "model-c", "model-d", "model-e"} {
		require.NoError(t, model.DB.Create(&model.Ability{Group: "vip", Model: modelName, ChannelId: 99502, Enabled: true, Priority: &priority, Weight: weight}).Error)
	}
	classifier := &countingAutoModelClassifier{route: automodel.RouteGeneral}
	defaultAutoModelClassifier = classifier
	body := `{"model":"auto","messages":[{"role":"user","content":"safe"}]}`
	c := newAutoModelResolverContext("/v1/chat/completions", body)
	common.SetContextKey(c, constant.ContextKeyTokenGroup, "auto")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "auto")
	resolution, virtual, apiErr := resolveAutoModel(c, []byte(body), nil)
	require.True(t, virtual)
	require.Nil(t, apiErr)
	require.Equal(t, "vip", resolution.pair.group)
	require.Equal(t, 1, resolution.pair.groupIndex)
	require.Nil(t, applyAutoModelResolution(c, &ModelRequest{Model: "auto"}, []byte(body), resolution))
	require.Equal(t, 1, common.GetContextKeyInt(c, constant.ContextKeyAutoGroupIndex))

	// Make the earlier group healthy after the preselection. The existing
	// selector must still start at index 1 and never move backward to default.
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 99501).Update("status", common.ChannelStatusEnabled).Error)
	channel, selectedGroup, err := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{Ctx: c, ModelName: resolution.pair.model, TokenGroup: "auto", Retry: common.GetPointer(0)})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "vip", selectedGroup)
	require.Equal(t, 99502, channel.Id)
	require.NoError(t, service.ReleaseChannelConcurrencyForContext(c))
}

func TestDistributeNonAutoLeavesBodyUnchanged(t *testing.T) {
	setupAutoModelResolverTest(t, common.ChannelStatusEnabled, true)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
	}, Distribute(), func(c *gin.Context) {
		storage, err := common.GetBodyStorage(c)
		require.NoError(t, err)
		body, err := storage.Bytes()
		require.NoError(t, err)
		c.Data(http.StatusOK, "application/json", body)
	})
	body := `{"model":"model-a","messages":[{"role":"user","content":"safe"}],"temperature":0,"stream":false,"future":{"enabled":false}}`
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, body, recorder.Body.String())
}

func TestResolveAutoModelRejectsRecursionHeaderBeforeClassifier(t *testing.T) {
	setupAutoModelResolverTest(t, common.ChannelStatusEnabled, true)
	classifier := &countingAutoModelClassifier{route: automodel.RouteGeneral}
	defaultAutoModelClassifier = classifier
	body := `{"model":"auto","messages":[{"role":"user","content":"safe"}]}`
	c := newAutoModelResolverContext("/v1/chat/completions", body)
	c.Request.Header.Set(automodel.AutoHopHeader, automodel.AutoHopValue)
	resolution, virtual, apiErr := resolveAutoModel(c, []byte(body), nil)
	require.True(t, virtual)
	require.Nil(t, resolution)
	require.Equal(t, types.ErrorCodeAutoModelUnsupportedRequest, apiErr.GetErrorCode())
	require.Zero(t, classifier.calls)
}
