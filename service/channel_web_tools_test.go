package service

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func webToolContext(t *testing.T, path, body string) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	t.Cleanup(func() { common.CleanupBodyStorage(c) })
	return c
}

func TestRequestRequiresServerWebTools(t *testing.T) {
	for _, version := range []string{
		"web_search_20250305", "web_search_20260209", "web_search_20260318",
		"web_fetch_20250910", "web_fetch_20260209", "web_fetch_20260309", "web_fetch_20260318",
		"web_search_20270101", "web_fetch_20270101", // Preserve the versioned family on upgrades.
		"web_search", "web_search_preview",
		"web_search_2025_08_26", "web_search_preview_2025_03_11",
		"web_search_2027_01_01", "web_search_preview_2027_01_01",
	} {
		t.Run(version, func(t *testing.T) {
			body := fmt.Sprintf(`{"tools":[{"name":"Read","input_schema":{}},{"type":%q,"name":"web_search","max_uses":8,"allowed_domains":["example.com"],"allowed_callers":["code_execution_20260120"],"max_content_tokens":0,"citations":{"enabled":false}}],"tool_choice":{"type":"auto"}}`, version)
			for _, path := range []string{"/v1/messages", "/v1/chat/completions", "/v1/responses", "/pg/chat/completions", "/pg/responses"} {
				ctx := webToolContext(t, path, body)
				require.True(t, RequestRequiresServerWebTools(ctx))
				require.True(t, RequestRequiresServerWebTools(ctx)) // Cached detection must not consume the body.
				require.False(t, ChannelSupportsServerWebTools(ctx, &model.Channel{Type: constant.ChannelTypeCopilot}))
				got, err := io.ReadAll(ctx.Request.Body)
				require.NoError(t, err)
				require.Equal(t, body, string(got))
			}
		})
	}
	for _, body := range []string{`{"web_search_options":{}}`, `{"web_search_options":{"search_context_size":"low"}}`} {
		require.True(t, RequestRequiresServerWebTools(webToolContext(t, "/v1/chat/completions", body)))
	}
	for name, body := range map[string]string{
		"ordinary CC declarations":    `{"tools":[{"name":"WebSearch","input_schema":{"type":"object"}},{"name":"WebFetch","input_schema":{"type":"object"}}]}`,
		"custom function names":       `{"tools":[{"type":"function","function":{"name":"web_search"}},{"type":"function","name":"WebSearch"},{"type":"custom","name":"web_fetch"}]}`,
		"tool discovery":              `{"tools":[{"type":"tool_search_tool_regex_20251119","name":"tool_search_tool_regex"},{"type":"tool_search_tool_bm25_20251119","name":"tool_search_tool_bm25"}]}`,
		"MCP search":                  `{"tools":[{"name":"mcp__browser__web_search","input_schema":{}}]}`,
		"tool description and schema": `{"tools":[{"name":"example","description":"web_search_20250305","input_schema":{"properties":{"type":{"const":"web_fetch_20260318"}}}}]}`,
		"message content":             `{"messages":[{"role":"user","content":"web_search_20250305 请联网查询"},{"role":"assistant","content":[{"type":"tool_use","name":"WebSearch","input":{"query":"weather"}}]}]}`,
		"null":                        `{"tools":null,"web_search_options":null}`,
		"similar unknown types":       `{"tools":[{"type":"web_search_custom"},{"type":"web_fetch_20260318_extra"}]}`,
		"malformed OpenAI versions":   `{"tools":[{"type":"web_search_2025_8_26"},{"type":"web_search_preview_2025_03_11_extra"},{"type":"web_search_preview_20250311"},{"type":"web_search_2025_aa_26"},{"type":"web_fetch_2025_08_26"}]}`,
		"dated function names":        `{"tools":[{"type":"function","function":{"name":"web_search_2025_08_26"}},{"type":"function","function":{"name":"web_search_preview_2025_03_11"}}]}`,
		"no tool":                     `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			require.False(t, RequestRequiresServerWebTools(webToolContext(t, "/v1/messages", body)))
		})
	}
	require.False(t, RequestRequiresServerWebTools(nil))
	for _, path := range []string{"/v1/videos", "/v1/embeddings", "/v1/messages/count_tokens"} {
		require.False(t, RequestRequiresServerWebTools(webToolContext(t, path, `{"tools":[{"type":"web_search_20250305"}]}`)))
	}
}

func TestServerWebToolsFilterBeforePriorityAndRetries(t *testing.T) {
	for _, memoryCache := range []bool{false, true} {
		t.Run(fmt.Sprintf("memory_cache_%v", memoryCache), func(t *testing.T) {
			setupChannelSelectEndpointTestDB(t)
			common.MemoryCacheEnabled = memoryCache
			for i, channelType := range []int{constant.ChannelTypeCopilot, constant.ChannelTypeCopilot, constant.ChannelTypeAnthropic, constant.ChannelTypeOpenAI} {
				priority := int64(100 - i*10)
				channel := &model.Channel{Id: 9800 + i, Type: channelType, Key: "test-key", Name: "web-tools-test", Status: common.ChannelStatusEnabled, Group: "default", Models: "claude-test", Priority: &priority}
				require.NoError(t, model.DB.Create(channel).Error)
				require.NoError(t, model.DB.Create(&model.Ability{ChannelId: channel.Id, Group: "default", Model: "claude-test", Enabled: true, Priority: &priority}).Error)
			}
			model.InitChannelCache()
			ctx := webToolContext(t, "/v1/messages", `{"model":"claude-test","tools":[{"type":"web_search_20250305","name":"web_search"}]}`)
			for retry, want := range []int{9802, 9803} {
				param := &RetryParam{Ctx: ctx, TokenGroup: "default", ModelName: "claude-test", Retry: common.GetPointer(retry)}
				channel, group, err := CacheGetRandomSatisfiedChannel(param)
				require.NoError(t, err)
				require.Equal(t, "default", group)
				require.NotNil(t, channel)
				require.Equal(t, want, channel.Id)
				require.Equal(t, retry, param.GetRetry())
				require.NoError(t, ReleaseChannelConcurrencyForContext(ctx))
			}
			ordinary := webToolContext(t, "/v1/messages", `{"tools":[{"name":"WebSearch","input_schema":{}}]}`)
			channel, _, err := CacheGetRandomSatisfiedChannel(&RetryParam{Ctx: ordinary, TokenGroup: "default", ModelName: "claude-test", Retry: common.GetPointer(0)})
			require.NoError(t, err)
			require.NotNil(t, channel)
			require.Equal(t, 9800, channel.Id)
			require.NoError(t, ReleaseChannelConcurrencyForContext(ordinary))
		})
	}
}
