package dto

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGeneralOpenAIRequestNativeWebToolRoundTrip(t *testing.T) {
	for _, toolType := range []string{
		"web_search_20250305", "web_search_20260209", "web_search_20260318",
		"web_fetch_20250910", "web_fetch_20260209", "web_fetch_20260309", "web_fetch_20260318",
	} {
		t.Run(toolType, func(t *testing.T) {
			name := "web_search"
			if toolType[:9] == "web_fetch" {
				name = "web_fetch"
			}
			tool := fmt.Sprintf(`{
				"type":%q,"name":%q,"max_uses":0,
				"allowed_domains":["weather.com.cn"],"blocked_domains":[],
				"user_location":{"type":"approximate","city":"Beijing","timezone":"Asia/Shanghai"},
				"citations":{"enabled":false},"max_content_tokens":0,
				"allowed_callers":["direct"],"defer_loading":false,
				"cache_control":{"type":"ephemeral"},
				"future_option":{"enabled":false,"limit":0,"id":9007199254740993}
			}`, toolType, name)
			var request GeneralOpenAIRequest
			require.NoError(t, common.UnmarshalJsonStr(`{"model":"claude-opus-5","tools":[`+tool+`]}`, &request))

			// TextHelper deep-copies the parsed request before invoking the adaptor.
			copied, err := common.DeepCopy(&request)
			require.NoError(t, err)
			encoded, err := common.Marshal(copied)
			require.NoError(t, err)
			actual := gjson.GetBytes(encoded, "tools.0")
			require.JSONEq(t, tool, actual.Raw)
			require.Equal(t, "9007199254740993", actual.Get("future_option.id").Raw)
			require.False(t, actual.Get("function").Exists(), "native tools must not gain an empty function")
		})
	}
}

func TestToolCallRequestOrdinaryToolsUnchanged(t *testing.T) {
	for _, raw := range []string{
		`{"type":"function","function":{"name":"WebSearch","parameters":{"type":"object","properties":{}}}}`,
		`{"type":"function","function":{"name":"WebFetch","description":"web_search_20250305","parameters":{"type":"object"}}}`,
		`{"type":"custom","function":{"name":""},"custom":{"name":"grammar","format":{"type":"text"}}}`,
	} {
		var tool ToolCallRequest
		require.NoError(t, common.UnmarshalJsonStr(raw, &tool))
		require.Empty(t, tool.ClaudeWebTool())
		encoded, err := common.Marshal(tool)
		require.NoError(t, err)
		require.JSONEq(t, raw, string(encoded))
	}
	for _, toolType := range []string{"web_search", "web_search_preview", "tool_search", "web_search_2025030", "web_fetch_20260318_extra", "web_search_abcdefgh"} {
		var tool ToolCallRequest
		require.NoError(t, common.UnmarshalJsonStr(fmt.Sprintf(`{"type":%q,"name":"web_search"}`, toolType), &tool))
		require.Empty(t, tool.ClaudeWebTool(), toolType)
	}
}

func TestToolCallRequestNativeWebToolOwnsInputAndResetsOnReuse(t *testing.T) {
	raw := []byte(`{"type":"web_search_20250305","name":"web_search"}`)
	var tool ToolCallRequest
	require.NoError(t, common.Unmarshal(raw, &tool))
	expected := string(raw)
	for i := range raw {
		raw[i] = ' '
	}
	encoded, err := common.Marshal(tool)
	require.NoError(t, err)
	require.JSONEq(t, expected, string(encoded))

	function := `{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}`
	require.NoError(t, common.UnmarshalJsonStr(function, &tool))
	require.Empty(t, tool.ClaudeWebTool())
	encoded, err = common.Marshal(tool)
	require.NoError(t, err)
	require.JSONEq(t, function, string(encoded))
}
