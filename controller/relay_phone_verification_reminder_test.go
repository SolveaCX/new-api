package controller

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func chatReq(t *testing.T, body string) *dto.GeneralOpenAIRequest {
	t.Helper()
	req := &dto.GeneralOpenAIRequest{}
	require.NoError(t, common.Unmarshal([]byte(body), req))
	return req
}

func claudeReq(t *testing.T, body string) *dto.ClaudeRequest {
	t.Helper()
	req := &dto.ClaudeRequest{}
	require.NoError(t, common.Unmarshal([]byte(body), req))
	return req
}

func responsesReq(t *testing.T, body string) *dto.OpenAIResponsesRequest {
	t.Helper()
	req := &dto.OpenAIResponsesRequest{}
	require.NoError(t, common.Unmarshal([]byte(body), req))
	return req
}

func geminiReq(t *testing.T, body string) *dto.GeminiChatRequest {
	t.Helper()
	req := &dto.GeminiChatRequest{}
	require.NoError(t, json.Unmarshal([]byte(body), req))
	return req
}

func TestPhoneVerificationReminderKindSelectsOnlyTextGeneration(t *testing.T) {
	tests := []struct {
		name    string
		format  types.RelayFormat
		mode    int
		path    string
		request dto.Request
		want    phoneVerificationReminderKind
		ok      bool
	}{
		{name: "chat completions", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: chatReq(t, `{"model":"gpt-x","messages":[{"role":"user","content":"hi"}]}`), want: phoneReminderKindOpenAIChat, ok: true},
		{name: "legacy completions", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeCompletions, path: "/v1/completions",
			request: chatReq(t, `{"model":"gpt-x","prompt":"hi"}`), ok: false},
		{name: "moderations", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeModerations, path: "/v1/moderations",
			request: chatReq(t, `{"model":"omni","input":"hi"}`), ok: false},
		{name: "embeddings", format: types.RelayFormatEmbedding, mode: relayconstant.RelayModeEmbeddings, path: "/v1/embeddings",
			request: &dto.EmbeddingRequest{}, ok: false},
		{name: "claude messages", format: types.RelayFormatClaude, mode: relayconstant.RelayModeChatCompletions, path: "/v1/messages",
			request: claudeReq(t, `{"model":"claude-x","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}`), want: phoneReminderKindClaude, ok: true},
		{name: "responses", format: types.RelayFormatOpenAIResponses, mode: relayconstant.RelayModeResponses, path: "/v1/responses",
			request: responsesReq(t, `{"model":"gpt-x","input":"hi"}`), want: phoneReminderKindResponses, ok: true},
		{name: "responses compact", format: types.RelayFormatOpenAIResponsesCompaction, mode: relayconstant.RelayModeResponsesCompact, path: "/v1/responses/compact",
			request: &dto.OpenAIResponsesCompactionRequest{}, ok: false},
		{name: "gemini generateContent", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1beta/models/gemini-x:generateContent",
			request: geminiReq(t, `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`), want: phoneReminderKindGemini, ok: true},
		{name: "gemini streamGenerateContent under /v1", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1/models/gemini-x:streamGenerateContent",
			request: geminiReq(t, `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`), want: phoneReminderKindGemini, ok: true},
		{name: "gemini countTokens", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1beta/models/gemini-x:countTokens",
			request: geminiReq(t, `{"contents":[]}`), ok: false},
		{name: "gemini batch", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1beta/models/gemini-x:generateContent",
			request: geminiReq(t, `{"requests":[{"contents":[]}]}`), ok: false},
		{name: "image", format: types.RelayFormatOpenAIImage, mode: relayconstant.RelayModeImagesGenerations, path: "/v1/images/generations",
			request: &dto.ImageRequest{}, ok: false},
		{name: "mismatched request type", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: &dto.ClaudeRequest{}, ok: false},
		{name: "nil request", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: nil, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{RelayFormat: tt.format, RelayMode: tt.mode}
			got, ok := phoneVerificationReminderKindFor(tt.format, info, tt.path, tt.request)
			require.Equal(t, tt.ok, ok)
			if tt.ok {
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPhoneVerificationReminderSkipsMachineReadableRequests(t *testing.T) {
	tests := []struct {
		name    string
		format  types.RelayFormat
		mode    int
		path    string
		request dto.Request
		skip    bool
	}{
		{name: "chat json_object", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: chatReq(t, `{"model":"gpt-x","messages":[],"response_format":{"type":"json_object"}}`), skip: true},
		{name: "chat json_schema", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: chatReq(t, `{"model":"gpt-x","messages":[],"response_format":{"type":"json_schema","json_schema":{"name":"x"}}}`), skip: true},
		{name: "chat text format", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: chatReq(t, `{"model":"gpt-x","messages":[],"response_format":{"type":"text"}}`), skip: false},
		{name: "chat tool_choice required", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: chatReq(t, `{"model":"gpt-x","messages":[],"tool_choice":"required"}`), skip: true},
		{name: "chat tool_choice named", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: chatReq(t, `{"model":"gpt-x","messages":[],"tool_choice":{"type":"function","function":{"name":"f"}}}`), skip: true},
		{name: "chat tool_choice auto", format: types.RelayFormatOpenAI, mode: relayconstant.RelayModeChatCompletions, path: "/v1/chat/completions",
			request: chatReq(t, `{"model":"gpt-x","messages":[],"tools":[{"type":"function","function":{"name":"f"}}],"tool_choice":"auto"}`), skip: false},
		{name: "claude tool_choice any", format: types.RelayFormatClaude, mode: relayconstant.RelayModeChatCompletions, path: "/v1/messages",
			request: claudeReq(t, `{"model":"c","max_tokens":1,"messages":[],"tool_choice":{"type":"any"}}`), skip: true},
		{name: "claude tool_choice tool", format: types.RelayFormatClaude, mode: relayconstant.RelayModeChatCompletions, path: "/v1/messages",
			request: claudeReq(t, `{"model":"c","max_tokens":1,"messages":[],"tool_choice":{"type":"tool","name":"f"}}`), skip: true},
		{name: "claude tool_choice auto", format: types.RelayFormatClaude, mode: relayconstant.RelayModeChatCompletions, path: "/v1/messages",
			request: claudeReq(t, `{"model":"c","max_tokens":1,"messages":[],"tool_choice":{"type":"auto"}}`), skip: false},
		{name: "claude output_format", format: types.RelayFormatClaude, mode: relayconstant.RelayModeChatCompletions, path: "/v1/messages",
			request: claudeReq(t, `{"model":"c","max_tokens":1,"messages":[],"output_format":{"type":"json_schema","schema":{}}}`), skip: true},
		{name: "responses text json_schema", format: types.RelayFormatOpenAIResponses, mode: relayconstant.RelayModeResponses, path: "/v1/responses",
			request: responsesReq(t, `{"model":"gpt-x","input":"hi","text":{"format":{"type":"json_schema","name":"x","schema":{}}}}`), skip: true},
		{name: "responses text json_object", format: types.RelayFormatOpenAIResponses, mode: relayconstant.RelayModeResponses, path: "/v1/responses",
			request: responsesReq(t, `{"model":"gpt-x","input":"hi","text":{"format":{"type":"json_object"}}}`), skip: true},
		{name: "responses text plain", format: types.RelayFormatOpenAIResponses, mode: relayconstant.RelayModeResponses, path: "/v1/responses",
			request: responsesReq(t, `{"model":"gpt-x","input":"hi","text":{"format":{"type":"text"},"verbosity":"low"}}`), skip: false},
		{name: "responses tool_choice required", format: types.RelayFormatOpenAIResponses, mode: relayconstant.RelayModeResponses, path: "/v1/responses",
			request: responsesReq(t, `{"model":"gpt-x","input":"hi","tool_choice":"required"}`), skip: true},
		{name: "responses tool_choice object", format: types.RelayFormatOpenAIResponses, mode: relayconstant.RelayModeResponses, path: "/v1/responses",
			request: responsesReq(t, `{"model":"gpt-x","input":"hi","tool_choice":{"type":"function","name":"f"}}`), skip: true},
		{name: "responses tool_choice auto", format: types.RelayFormatOpenAIResponses, mode: relayconstant.RelayModeResponses, path: "/v1/responses",
			request: responsesReq(t, `{"model":"gpt-x","input":"hi","tool_choice":"auto"}`), skip: false},
		{name: "gemini json mime", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1beta/models/g:generateContent",
			request: geminiReq(t, `{"contents":[],"generationConfig":{"responseMimeType":"application/json"}}`), skip: true},
		{name: "gemini responseSchema", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1beta/models/g:generateContent",
			request: geminiReq(t, `{"contents":[],"generationConfig":{"responseSchema":{"type":"OBJECT"}}}`), skip: true},
		{name: "gemini responseJsonSchema", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1beta/models/g:generateContent",
			request: geminiReq(t, `{"contents":[],"generationConfig":{"responseJsonSchema":{"type":"object"}}}`), skip: true},
		{name: "gemini function mode ANY", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1beta/models/g:generateContent",
			request: geminiReq(t, `{"contents":[],"toolConfig":{"functionCallingConfig":{"mode":"ANY"}}}`), skip: true},
		{name: "gemini function mode AUTO", format: types.RelayFormatGemini, mode: relayconstant.RelayModeGemini, path: "/v1beta/models/g:generateContent",
			request: geminiReq(t, `{"contents":[],"toolConfig":{"functionCallingConfig":{"mode":"AUTO"}}}`), skip: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{RelayFormat: tt.format, RelayMode: tt.mode}
			_, ok := phoneVerificationReminderKindFor(tt.format, info, tt.path, tt.request)
			require.Equal(t, !tt.skip, ok)
		})
	}
}
