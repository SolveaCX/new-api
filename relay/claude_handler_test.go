package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/codex"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestShouldClaudeUseResponsesBridgeForCodexDespiteGlobalPassThrough(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	previous := settings.PassThroughRequestEnabled
	settings.PassThroughRequestEnabled = true
	t.Cleanup(func() {
		settings.PassThroughRequestEnabled = previous
	})

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ApiType: constant.APITypeCodex},
	}

	require.True(t, shouldClaudeUseResponsesBridge(info))
}

type codexClaudeBridgeStub struct {
	codex.Adaptor
	response  *http.Response
	converted map[string]any
}

func (a *codexClaudeBridgeStub) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	converted, err := a.Adaptor.ConvertOpenAIResponsesRequest(c, info, request)
	if err != nil {
		return nil, err
	}
	body, _ := converted.(map[string]any)
	a.converted = body
	return converted, nil
}

func (a *codexClaudeBridgeStub) DoRequest(*gin.Context, *relaycommon.RelayInfo, io.Reader) (any, error) {
	return a.response, nil
}

func TestShouldClaudeUseResponsesBridgeHonorsPassThroughForOtherChannels(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	previous := settings.PassThroughRequestEnabled
	settings.PassThroughRequestEnabled = true
	t.Cleanup(func() {
		settings.PassThroughRequestEnabled = previous
	})

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ApiType: constant.APITypeOpenAI},
	}

	require.False(t, shouldClaudeUseResponsesBridge(info))
}

func TestShouldClaudeUseResponsesBridgeUsesNativeCopilotMessages(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	previousPassThrough := settings.PassThroughRequestEnabled
	previousPolicy := settings.ChatCompletionsToResponsesPolicy
	settings.PassThroughRequestEnabled = false
	settings.ChatCompletionsToResponsesPolicy = model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:      true,
		ChannelTypes: []int{constant.ChannelTypeCopilot},
	}
	t.Cleanup(func() {
		settings.PassThroughRequestEnabled = previousPassThrough
		settings.ChatCompletionsToResponsesPolicy = previousPolicy
	})

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType:     constant.APITypeCopilot,
			ChannelType: constant.ChannelTypeCopilot,
		},
	}

	require.False(t, shouldClaudeUseResponsesBridge(info))
}

func TestShouldClaudeUseResponsesBridgeForCodexDespiteChannelPassThrough(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	previous := settings.PassThroughRequestEnabled
	settings.PassThroughRequestEnabled = false
	t.Cleanup(func() {
		settings.PassThroughRequestEnabled = previous
	})

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType: constant.APITypeCodex,
			ChannelSetting: dto.ChannelSettings{
				PassThroughBodyEnabled: true,
			},
		},
	}

	require.True(t, shouldClaudeUseResponsesBridge(info))
}

func TestShouldClaudeUseResponsesBridgeHonorsChannelPassThroughForOtherChannels(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	previous := settings.PassThroughRequestEnabled
	settings.PassThroughRequestEnabled = false
	t.Cleanup(func() {
		settings.PassThroughRequestEnabled = previous
	})

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType: constant.APITypeOpenAI,
			ChannelSetting: dto.ChannelSettings{
				PassThroughBodyEnabled: true,
			},
		},
	}

	require.False(t, shouldClaudeUseResponsesBridge(info))
}

func TestShouldClaudeUseResponsesBridgePreservesPolicyForOtherChannels(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	previousPassThrough := settings.PassThroughRequestEnabled
	previousPolicy := settings.ChatCompletionsToResponsesPolicy
	settings.PassThroughRequestEnabled = false
	settings.ChatCompletionsToResponsesPolicy = model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:       true,
		ChannelTypes:  []int{constant.ChannelTypeOpenAI},
		ModelPatterns: []string{`^gpt-5\.5$`},
	}
	t.Cleanup(func() {
		settings.PassThroughRequestEnabled = previousPassThrough
		settings.ChatCompletionsToResponsesPolicy = previousPolicy
	})

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType:           constant.APITypeOpenAI,
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "gpt-5.5",
		},
		OriginModelName: "gpt-5.5",
	}

	require.True(t, shouldClaudeUseResponsesBridge(info))
}

func TestCodexClaudeBridgeAppliesChannelSystemPromptOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		clientSystem string
		override     bool
		expected     string
	}{
		{name: "injects when client system is absent", expected: "configured"},
		{name: "preserves client system without override", clientSystem: "client", expected: "client"},
		{name: "overrides once when client system is absent", override: true, expected: "configured"},
		{name: "prefixes once when client system exists", clientSystem: "client", override: true, expected: "configured\nclient"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			request := &dto.ClaudeRequest{Model: "gpt-5.5"}
			if tt.clientSystem != "" {
				request.SetStringSystem(tt.clientSystem)
			}
			info := &relaycommon.RelayInfo{
				RelayFormat:     types.RelayFormatClaude,
				RelayMode:       relayconstant.RelayModeResponses,
				OriginModelName: "gpt-5.5",
				ChannelMeta: &relaycommon.ChannelMeta{
					ApiType:           constant.APITypeCodex,
					ChannelType:       constant.ChannelTypeCodex,
					UpstreamModelName: "gpt-5.5",
					ChannelSetting: dto.ChannelSettings{
						SystemPrompt:         "configured",
						SystemPromptOverride: tt.override,
					},
				},
			}

			applyClaudeChannelSystemPrompt(ctx, info, request, true)
			openAIRequest, err := service.ClaudeToOpenAIRequest(*request, info)
			require.NoError(t, err)
			responsesRequest, err := service.ChatCompletionsRequestToResponsesRequest(openAIRequest)
			require.NoError(t, err)
			converted, err := (&codex.Adaptor{}).ConvertOpenAIResponsesRequest(ctx, info, *responsesRequest)
			require.NoError(t, err)

			body, ok := converted.(map[string]any)
			require.True(t, ok)
			require.Equal(t, tt.expected, body["instructions"])
		})
	}
}

func TestCodexClaudeBridgeReturnsAnthropicResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamingTimeout = previousStreamingTimeout
	})

	tests := []struct {
		name        string
		stream      bool
		contentType string
		upstream    string
	}{
		{
			name:        "non-stream",
			contentType: "application/json",
			upstream:    `{"id":"resp_1","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"pong"}]}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`,
		},
		{
			name:        "stream",
			stream:      true,
			contentType: "text/event-stream",
			upstream: strings.Join([]string{
				`event: response.created`,
				`data: {"type":"response.created","response":{"id":"resp_1","created_at":1700000000,"model":"gpt-5.5"}}`,
				``,
				`event: response.output_text.delta`,
				`data: {"type":"response.output_text.delta","output_index":0,"delta":"pong"}`,
				``,
				`event: response.completed`,
				`data: {"type":"response.completed","response":{"id":"resp_1","status":"completed","usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}`,
				``,
				``,
			}, "\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			ctx.Request.Header.Set("Content-Type", "application/json")

			stream := tt.stream
			maxTokens := uint(16)
			claudeRequest := &dto.ClaudeRequest{
				Model:     "gpt-5.5",
				MaxTokens: &maxTokens,
				Stream:    &stream,
				Messages: []dto.ClaudeMessage{
					{Role: "user", Content: "ping"},
				},
			}
			info := &relaycommon.RelayInfo{
				RelayFormat:            types.RelayFormatClaude,
				RelayMode:              relayconstant.RelayModeUnknown,
				RequestURLPath:         "/v1/messages",
				OriginModelName:        "gpt-5.5",
				IsStream:               tt.stream,
				RequestConversionChain: []types.RelayFormat{types.RelayFormatClaude},
				ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
					LastMessagesType: relaycommon.LastMessageTypeNone,
				},
				ChannelMeta: &relaycommon.ChannelMeta{
					ApiType:           constant.APITypeCodex,
					ChannelType:       constant.ChannelTypeCodex,
					UpstreamModelName: "gpt-5.5",
					ChannelSetting: dto.ChannelSettings{
						SystemPrompt:         "configured",
						SystemPromptOverride: true,
					},
				},
			}

			applyClaudeChannelSystemPrompt(ctx, info, claudeRequest, true)
			openAIRequest, err := service.ClaudeToOpenAIRequest(*claudeRequest, info)
			require.NoError(t, err)
			adaptor := &codexClaudeBridgeStub{
				response: &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{tt.contentType}},
					Body:       io.NopCloser(strings.NewReader(tt.upstream)),
				},
			}

			usage, apiErr := chatCompletionsViaResponses(ctx, info, adaptor, openAIRequest)
			require.Nil(t, apiErr)
			require.Equal(t, 5, usage.TotalTokens)
			require.Equal(t, "configured", adaptor.converted["instructions"])

			responseBody := recorder.Body.String()
			require.Contains(t, responseBody, "pong")
			if tt.stream {
				require.Contains(t, responseBody, "message_start")
				require.Contains(t, responseBody, "content_block_delta")
				require.Contains(t, responseBody, "message_stop")
				require.Contains(t, responseBody, `"input_tokens":3`)
				require.Contains(t, responseBody, `"output_tokens":2`)
			} else {
				var response dto.ClaudeResponse
				require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, "message", response.Type)
				require.Len(t, response.Content, 1)
				require.Equal(t, "pong", response.Content[0].GetText())
			}
		})
	}
}

func TestShouldClaudeUseResponsesBridgeForGrokDespiteGlobalPassThrough(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	previous := settings.PassThroughRequestEnabled
	settings.PassThroughRequestEnabled = true
	t.Cleanup(func() {
		settings.PassThroughRequestEnabled = previous
	})

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ApiType: constant.APITypeGrokSubscription},
	}

	require.True(t, shouldClaudeUseResponsesBridge(info))
}

func TestClaudeHelperRejectsManualFableThinkingAfterModelMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "fable-alias")
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "http://127.0.0.1:1")
	c.Set("model_mapping", `{"fable-alias":"anthropic/claude-fable-5"}`)

	request := &dto.ClaudeRequest{
		Model: "fable-alias",
		Thinking: &dto.Thinking{
			Type:         "enabled",
			BudgetTokens: common.GetPointer(4096),
		},
	}
	info := &relaycommon.RelayInfo{
		Request:         request,
		OriginModelName: "fable-alias",
		ChannelMeta:     &relaycommon.ChannelMeta{},
		RelayFormat:     types.RelayFormatClaude,
	}

	apiErr := ClaudeHelper(c, info)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	require.Equal(t, "invalid_request_error", apiErr.ToClaudeError().Type)
	require.Contains(t, apiErr.ToClaudeError().Message, "thinking.type.enabled")
	require.Equal(t, "anthropic/claude-fable-5", info.UpstreamModelName)
}

func TestValidateResolvedClaudeThinking(t *testing.T) {
	tests := []struct {
		name          string
		upstreamModel string
		requestModel  string
		thinkingType  string
		wantStatus    int
	}{
		{
			name:          "fable enabled",
			upstreamModel: "anthropic/claude-fable-5",
			requestModel:  "fable-alias",
			thinkingType:  "enabled",
			wantStatus:    http.StatusBadRequest,
		},
		{
			name:          "fable disabled",
			upstreamModel: "claude-fable-5:stable",
			requestModel:  "fable-alias",
			thinkingType:  "disabled",
			wantStatus:    http.StatusBadRequest,
		},
		{
			name:          "fable sibling variant enabled",
			upstreamModel: "anthropic/claude-fable-5.1",
			requestModel:  "fable-variant-alias",
			thinkingType:  "enabled",
			wantStatus:    http.StatusBadRequest,
		},
		{
			name:          "fable adaptive remains supported",
			upstreamModel: "claude-fable-5",
			requestModel:  "claude-fable-5",
			thinkingType:  "adaptive",
			wantStatus:    0,
		},
		{
			name:          "other Claude model remains supported",
			upstreamModel: "claude-opus-4-8",
			requestModel:  "claude-opus-4-8",
			thinkingType:  "enabled",
			wantStatus:    0,
		},
		{
			name:          "thinking model suffix is normalized",
			upstreamModel: "anthropic/claude-fable-5-thinking",
			requestModel:  "claude-fable-5-thinking",
			thinkingType:  "disabled",
			wantStatus:    http.StatusBadRequest,
		},
		{
			name:         "request model fallback without channel metadata",
			requestModel: "claude-fable-5",
			thinkingType: "enabled",
			wantStatus:   http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				OriginModelName: tt.requestModel,
				ChannelMeta: &relaycommon.ChannelMeta{
					UpstreamModelName: tt.upstreamModel,
				},
			}
			req := &dto.ClaudeRequest{
				Model: tt.requestModel,
				Thinking: &dto.Thinking{
					Type: tt.thinkingType,
				},
			}

			err := validateResolvedClaudeThinking(info, req)
			if tt.wantStatus == 0 {
				require.Nil(t, err)
				return
			}
			require.NotNil(t, err)
			require.Equal(t, tt.wantStatus, err.StatusCode)
			require.Equal(t, "invalid_request_error", err.ToClaudeError().Type)
		})
	}
}
