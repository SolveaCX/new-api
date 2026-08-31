package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDistributeVersionedWebSearchBypassesChatAffinityHTTP(t *testing.T) {
	require.NoError(t, i18n.Init())
	defer useMiddlewareMemoryChannelConcurrencyForTest(t)()
	defer useMiddlewareChannelSelectionDBForTest(t)()
	defer useMiddlewareChannelAffinityRuleForTest(t)()
	setting := operation_setting.GetChannelAffinitySetting()
	setting.SwitchOnSuccess = true
	setting.Rules[0].PathRegex = []string{"/v1/(chat/completions|responses)"}
	setting.Rules[0].SkipRetryOnFailure = true
	setting.Rules[0].ParamOverrideTemplate = map[string]any{"temperature": 0.1}
	for i, channelType := range []int{constant.ChannelTypeCopilot, constant.ChannelTypeOpenAI, constant.ChannelTypeOpenAI} {
		priority := int64(100 - i*10)
		channel := &model.Channel{Id: 9860 + i, Type: channelType, Key: "test-key", Name: "versioned-web-tools", Status: common.ChannelStatusEnabled, Group: "default", Models: "gpt-affinity-wait", Priority: &priority}
		require.NoError(t, model.DB.Create(channel).Error)
		require.NoError(t, channel.AddAbilities(nil))
	}
	model.InitChannelCache()
	// The chat is pinned to a lower-priority Responses-capable channel. A search
	// must neither inherit its skip-retry/template nor replace the chat binding.
	seed, _ := gin.CreateTestContext(httptest.NewRecorder())
	seed.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-affinity-wait"}`))
	seed.Request.Header.Set("X-Test-Affinity", t.Name())
	_, found := service.GetPreferredChannelByAffinity(seed, "gpt-affinity-wait", "default")
	require.False(t, found)
	service.RecordChannelAffinity(seed, 9862)
	t.Cleanup(func() { common.CleanupBodyStorage(seed) })

	router := gin.New()
	router.Use(BodyStorageCleanup())
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		c.Next()
	})
	router.Use(Distribute())
	handler := func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Header("X-Selected-Channel", fmt.Sprint(common.GetContextKeyInt(c, constant.ContextKeyChannelId)))
		c.Header("X-Skip-Retry", fmt.Sprint(service.ShouldSkipRetryAfterChannelAffinityFailure(c)))
		c.Header("X-Param-Override", fmt.Sprint(common.GetContextKey(c, constant.ContextKeyChannelParamOverride)))
		c.Data(http.StatusOK, "application/json", body)
	}
	router.POST("/v1/responses", handler)
	router.POST("/v1/chat/completions", handler)
	server := httptest.NewServer(router)
	defer server.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	t.Logf("local HTTP verification: %s/v1/responses", server.URL)
	request := func(path, body string) (*http.Response, string) {
		req, err := http.NewRequest(http.MethodPost, server.URL+path, strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Affinity", t.Name())
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		got, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode, string(got))
		require.Equal(t, body, string(got))
		return resp, string(got)
	}
	for _, toolType := range []string{"web_search", "web_search_preview", "web_search_2025_08_26", "web_search_preview_2025_03_11"} {
		for _, stream := range []bool{false, true} {
			body := fmt.Sprintf(`{"model":"gpt-affinity-wait","input":"weather","stream":%t,"tools":[{"type":%q,"search_context_size":"medium"}]}`, stream, toolType)
			resp, _ := request("/v1/responses", body)
			require.Equal(t, "9861", resp.Header.Get("X-Selected-Channel"), toolType)
			require.Equal(t, "false", resp.Header.Get("X-Skip-Retry"), toolType)
			require.NotContains(t, resp.Header.Get("X-Param-Override"), "temperature", toolType)
		}
	}
	resp, _ := request("/v1/chat/completions", `{"model":"gpt-affinity-wait","messages":[{"role":"user","content":"hello"}]}`)
	require.Equal(t, "9862", resp.Header.Get("X-Selected-Channel"), "search must not replace chat affinity")
	require.Equal(t, "true", resp.Header.Get("X-Skip-Retry"))
	require.Contains(t, resp.Header.Get("X-Param-Override"), "temperature")
}
