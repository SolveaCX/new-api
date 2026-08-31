package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDistributeServerWebToolsHTTP(t *testing.T) {
	require.NoError(t, i18n.Init())
	defer useMiddlewareMemoryChannelConcurrencyForTest(t)()
	defer useMiddlewareChannelSelectionDBForTest(t)()
	defer useMiddlewareChannelAffinityRuleForTest(t)()
	setting := operation_setting.GetChannelAffinitySetting()
	setting.SwitchOnSuccess = true
	setting.Rules[0].SkipRetryOnFailure = true
	setting.Rules[0].ParamOverrideTemplate = map[string]any{"temperature": 0.1}
	setting.Rules[0].PathRegex = []string{"/v1/(chat/completions|messages)"}
	const modelName = "gpt-affinity-wait"
	for i, channelType := range []int{constant.ChannelTypeCopilot, constant.ChannelTypeAnthropic} {
		priority := int64(100 - i*90)
		channel := &model.Channel{Id: 9850 + i, Type: channelType, Key: "test-key", Name: "web-tools-test", Status: common.ChannelStatusEnabled, Models: modelName, Group: "default", Priority: &priority}
		require.NoError(t, model.DB.Create(channel).Error)
		require.NoError(t, model.DB.Create(&model.Ability{ChannelId: channel.Id, Group: "default", Model: modelName, Enabled: true, Priority: &priority}).Error)
	}
	model.InitChannelCache()
	affinityValue := t.Name()
	seed, _ := gin.CreateTestContext(httptest.NewRecorder())
	seed.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"gpt-affinity-wait"}`))
	seed.Request.Header.Set("X-Test-Affinity", affinityValue)
	_, found := service.GetPreferredChannelByAffinity(seed, modelName, "default")
	require.False(t, found)
	service.RecordChannelAffinity(seed, 9850)
	t.Cleanup(func() { common.CleanupBodyStorage(seed) })

	router := gin.New()
	router.Use(BodyStorageCleanup())
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		if c.GetHeader("X-Test-Pinned") != "" {
			common.SetContextKey(c, constant.ContextKeyTokenSpecificChannelId, "9850")
		}
		c.Next()
	})
	router.Use(Distribute())
	router.POST("/v1/messages", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Header("X-Selected-Channel", fmt.Sprint(common.GetContextKeyInt(c, constant.ContextKeyChannelId)))
		c.Header("X-Skip-Retry", fmt.Sprint(service.ShouldSkipRetryAfterChannelAffinityFailure(c)))
		c.Header("X-Param-Override", fmt.Sprint(common.GetContextKey(c, constant.ContextKeyChannelParamOverride)))
		c.Data(http.StatusOK, "application/json", body)
	})
	server := httptest.NewServer(router)
	defer server.Close()
	t.Logf("路由本地 HTTP 验证地址：%s/v1/messages", server.URL)
	request := func(body string, pinned bool) (*http.Response, string) {
		req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/messages", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Affinity", affinityValue)
		if pinned {
			req.Header.Set("X-Test-Pinned", "yes")
		}
		resp, err := server.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		got, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp, string(got)
	}
	for _, stream := range []bool{false, true} {
		body := fmt.Sprintf(`{"model":"gpt-affinity-wait","stream":%v,"tools":[{"type":"web_search_20250305","name":"web_search","max_uses":8,"blocked_domains":["example.com"]}],"tool_choice":{"type":"tool","name":"web_search"}}`, stream)
		resp, got := request(body, false)
		require.Equal(t, http.StatusOK, resp.StatusCode, got)
		require.Equal(t, "9851", resp.Header.Get("X-Selected-Channel"))
		require.Equal(t, "false", resp.Header.Get("X-Skip-Retry"))
		require.NotContains(t, resp.Header.Get("X-Param-Override"), "temperature")
		require.Equal(t, body, got)
	}
	// A successful web request must not overwrite ordinary CC chat affinity.
	plain := `{"model":"gpt-affinity-wait","tools":[{"name":"WebSearch","input_schema":{"type":"object"}},{"name":"WebFetch","input_schema":{"type":"object"}}]}`
	resp, got := request(plain, false)
	require.Equal(t, http.StatusOK, resp.StatusCode, got)
	require.Equal(t, "9850", resp.Header.Get("X-Selected-Channel"))
	require.Equal(t, "true", resp.Header.Get("X-Skip-Retry"))
	require.Contains(t, resp.Header.Get("X-Param-Override"), "temperature")
	web := `{"model":"gpt-affinity-wait","tools":[{"type":"web_fetch_20260318","name":"web_fetch"}]}`
	resp, got = request(web, true)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, got)
	require.Contains(t, got, string(types.ErrorCodeUnsupportedWebTools))
	require.Empty(t, resp.Header.Get("X-Selected-Channel"))
	// With only Copilot left, fail locally instead of silently dropping tools.
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 9851).Update("status", common.ChannelStatusManuallyDisabled).Error)
	require.NoError(t, model.DB.Model(&model.Ability{}).Where("channel_id = ?", 9851).Update("enabled", false).Error)
	model.InitChannelCache()
	resp, got = request(web, false)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, got)
	require.Empty(t, resp.Header.Get("X-Selected-Channel"))
}

func TestSetupContextRejectsUnsupportedServerWebTools(t *testing.T) {
	i18n.Init()
	for _, lang := range []string{"en", "zh-CN", "zh-TW", "pt"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"tools":[{"type":"web_search_20260318","name":"web_search"}]}`))
		c.Request.Header.Set("Accept-Language", lang)
		err := SetupContextForSelectedChannel(c, &model.Channel{Id: 1, Type: constant.ChannelTypeCopilot}, "claude-test")
		require.NotNil(t, err)
		require.Equal(t, http.StatusBadRequest, err.StatusCode)
		require.Equal(t, types.ErrorCodeUnsupportedWebTools, err.GetErrorCode())
		require.True(t, types.IsSkipRetryError(err))
		require.False(t, types.IsRecordErrorLog(err))
		require.NotContains(t, err.Error(), "distributor.unsupported_web_tools")
		require.Zero(t, common.GetContextKeyInt(c, constant.ContextKeyChannelId))
		common.CleanupBodyStorage(c)
	}
}
