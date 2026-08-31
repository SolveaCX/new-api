package claude

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestRequestOpenAI2ClaudeMessageWebTools(t *testing.T) {
	tests := []struct {
		name    string
		tools   string
		options string
		want    string
	}{
		{
			name:  "user native search request",
			tools: `[{"type":"web_search_20250305","name":"web_search"}]`,
			want:  `[{"type":"web_search_20250305","name":"web_search"}]`,
		},
		{
			name:  "native fetch preserves restrictions and zero values",
			tools: `[{"type":"web_fetch_20260318","name":"web_fetch","max_uses":0,"allowed_domains":["weather.com.cn"],"citations":{"enabled":false},"max_content_tokens":0,"allowed_callers":["direct"]}]`,
			want:  `[{"type":"web_fetch_20260318","name":"web_fetch","max_uses":0,"allowed_domains":["weather.com.cn"],"citations":{"enabled":false},"max_content_tokens":0,"allowed_callers":["direct"]}]`,
		},
		{
			name:    "native search takes precedence over shorthand",
			tools:   `[{"type":"web_search_20260318","name":"web_search","max_uses":0,"blocked_domains":["example.com"],"allowed_callers":["direct"]}]`,
			options: `,"web_search_options":{"search_context_size":"high"}`,
			want:    `[{"type":"web_search_20260318","name":"web_search","max_uses":0,"blocked_domains":["example.com"],"allowed_callers":["direct"]}]`,
		},
		{
			name:    "native omitted max uses is not overwritten by shorthand",
			tools:   `[{"type":"web_search_20250305","name":"web_search"}]`,
			options: `,"web_search_options":{"search_context_size":"medium"}`,
			want:    `[{"type":"web_search_20250305","name":"web_search"}]`,
		},
		{
			name:    "fetch does not suppress shorthand search",
			tools:   `[{"type":"web_fetch_20250910","name":"web_fetch"}]`,
			options: `,"web_search_options":{"search_context_size":"medium"}`,
			want:    `[{"type":"web_fetch_20250910","name":"web_fetch"},{"type":"web_search_20250305","name":"web_search","max_uses":5}]`,
		},
		{
			name:    "existing shorthand and location mapping",
			tools:   `[]`,
			options: `,"web_search_options":{"search_context_size":"low","user_location":{"type":"approximate","approximate":{"city":"Beijing","country":"CN","timezone":"Asia/Shanghai"}}}`,
			want:    `[{"type":"web_search_20250305","name":"web_search","max_uses":1,"user_location":{"type":"approximate","city":"Beijing","country":"CN","timezone":"Asia/Shanghai"}}]`,
		},
		{
			name:  "ordinary functions coexist with native tools",
			tools: `[{"type":"function","function":{"name":"lookup","description":"lookup data","parameters":{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}}},{"type":"web_search_20260209","name":"web_search"}]`,
			want:  `[{"name":"lookup","description":"lookup data","input_schema":{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}},{"type":"web_search_20260209","name":"web_search"}]`,
		},
		{
			name:  "client WebFetch remains a function",
			tools: `[{"type":"function","function":{"name":"WebFetch","description":"fetch a URL","parameters":{"type":"object","properties":{},"required":[]}}}]`,
			want:  `[{"name":"WebFetch","description":"fetch a URL","input_schema":{"type":"object","properties":{},"required":[]}}]`,
		},
		{
			name:  "content alone never adds tools",
			tools: `[]`,
			want:  `[]`,
		},
	}
	for _, tt := range tests {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", tt.name, stream), func(t *testing.T) {
				body := fmt.Sprintf(`{"model":"claude-opus-5","max_tokens":1024,"stream":%t,"messages":[{"role":"user","content":"今天北京天气如何？请你搜索后回答"}],"tools":%s%s}`, stream, tt.tools, tt.options)
				var request dto.GeneralOpenAIRequest
				require.NoError(t, common.UnmarshalJsonStr(body, &request))
				copied, err := common.DeepCopy(&request)
				require.NoError(t, err)
				converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, nil, copied)
				require.NoError(t, err)
				encoded, err := common.Marshal(converted)
				require.NoError(t, err)
				tools := gjson.GetBytes(encoded, "tools")
				if tt.want == "[]" && !tools.Exists() {
					return
				}
				require.JSONEq(t, tt.want, tools.Raw)
				require.Equal(t, stream, gjson.GetBytes(encoded, "stream").Bool())
				require.Equal(t, int64(1024), gjson.GetBytes(encoded, "max_tokens").Int())
			})
		}
	}
}
