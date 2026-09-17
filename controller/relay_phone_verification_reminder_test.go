package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
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

func newPhoneReminderRecorder(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}"))
	return c, recorder
}

func sseDataLines(t *testing.T, body string) []string {
	t.Helper()
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "data: ") {
			lines = append(lines, strings.TrimPrefix(line, "data: "))
		}
	}
	return lines
}

const reminderText = "please bind your phone"

func TestWritePhoneVerificationReminderOpenAIChat(t *testing.T) {
	t.Run("non-stream", func(t *testing.T) {
		c, rec := newPhoneReminderRecorder(t)
		require.NoError(t, writePhoneVerificationReminder(c, phoneReminderKindOpenAIChat, false, "gpt-x", reminderText))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Header().Get("Content-Type"), "application/json")
		var resp dto.OpenAITextResponse
		require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, "chat.completion", resp.Object)
		require.Equal(t, "gpt-x", resp.Model)
		require.True(t, strings.HasPrefix(resp.Id, "chatcmpl-"))
		require.Len(t, resp.Choices, 1)
		require.Equal(t, "assistant", resp.Choices[0].Message.Role)
		require.Equal(t, reminderText, resp.Choices[0].Message.StringContent())
		require.Equal(t, "stop", resp.Choices[0].FinishReason)
		require.Equal(t, 0, resp.Usage.TotalTokens)
	})
	t.Run("stream", func(t *testing.T) {
		c, rec := newPhoneReminderRecorder(t)
		require.NoError(t, writePhoneVerificationReminder(c, phoneReminderKindOpenAIChat, true, "gpt-x", reminderText))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
		lines := sseDataLines(t, rec.Body.String())
		require.Len(t, lines, 3)
		var first, second dto.ChatCompletionsStreamResponse
		require.NoError(t, common.Unmarshal([]byte(lines[0]), &first))
		require.NoError(t, common.Unmarshal([]byte(lines[1]), &second))
		require.Equal(t, "chat.completion.chunk", first.Object)
		require.Equal(t, "assistant", first.Choices[0].Delta.Role)
		require.Equal(t, reminderText, first.Choices[0].Delta.GetContentString())
		require.Equal(t, first.Id, second.Id)
		require.NotNil(t, second.Choices[0].FinishReason)
		require.Equal(t, "stop", *second.Choices[0].FinishReason)
		require.Equal(t, "[DONE]", lines[2])
	})
}

func TestWritePhoneVerificationReminderClaude(t *testing.T) {
	t.Run("non-stream", func(t *testing.T) {
		c, rec := newPhoneReminderRecorder(t)
		require.NoError(t, writePhoneVerificationReminder(c, phoneReminderKindClaude, false, "claude-x", reminderText))
		require.Equal(t, http.StatusOK, rec.Code)
		var resp dto.ClaudeResponse
		require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, "message", resp.Type)
		require.Equal(t, "assistant", resp.Role)
		require.Equal(t, "claude-x", resp.Model)
		require.True(t, strings.HasPrefix(resp.Id, "msg_"))
		require.Len(t, resp.Content, 1)
		require.Equal(t, "text", resp.Content[0].Type)
		require.Equal(t, reminderText, resp.Content[0].GetText())
		require.Equal(t, "end_turn", resp.StopReason)
		require.NotNil(t, resp.Usage)
		require.Equal(t, 0, resp.Usage.OutputTokens)
	})
	t.Run("stream", func(t *testing.T) {
		c, rec := newPhoneReminderRecorder(t)
		require.NoError(t, writePhoneVerificationReminder(c, phoneReminderKindClaude, true, "claude-x", reminderText))
		require.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()
		for _, event := range []string{"message_start", "content_block_start", "content_block_delta", "content_block_stop", "message_delta", "message_stop"} {
			require.Contains(t, body, "event: "+event+"\n", body)
		}
		require.Less(t, strings.Index(body, "event: message_start"), strings.Index(body, "event: content_block_delta"))
		require.Less(t, strings.Index(body, "event: content_block_delta"), strings.Index(body, "event: message_stop"))
		lines := sseDataLines(t, body)
		require.Len(t, lines, 6)
		var delta dto.ClaudeResponse
		require.NoError(t, common.Unmarshal([]byte(lines[2]), &delta))
		require.Equal(t, "content_block_delta", delta.Type)
		require.Equal(t, "text_delta", delta.Delta.Type)
		require.Equal(t, reminderText, delta.Delta.GetText())
		var messageDelta dto.ClaudeResponse
		require.NoError(t, common.Unmarshal([]byte(lines[4]), &messageDelta))
		require.Equal(t, "end_turn", *messageDelta.Delta.StopReason)
	})
}

func TestWritePhoneVerificationReminderResponses(t *testing.T) {
	t.Run("non-stream", func(t *testing.T) {
		c, rec := newPhoneReminderRecorder(t)
		require.NoError(t, writePhoneVerificationReminder(c, phoneReminderKindResponses, false, "gpt-x", reminderText))
		require.Equal(t, http.StatusOK, rec.Code)
		var resp dto.OpenAIResponsesResponse
		require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, "response", resp.Object)
		require.Equal(t, `"completed"`, string(resp.Status))
		require.True(t, strings.HasPrefix(resp.ID, "resp_"))
		require.Len(t, resp.Output, 1)
		require.Equal(t, "message", resp.Output[0].Type)
		require.Equal(t, "assistant", resp.Output[0].Role)
		require.Len(t, resp.Output[0].Content, 1)
		require.Equal(t, "output_text", resp.Output[0].Content[0].Type)
		require.Equal(t, reminderText, resp.Output[0].Content[0].Text)
		require.NotNil(t, resp.Usage)
	})
	t.Run("stream", func(t *testing.T) {
		c, rec := newPhoneReminderRecorder(t)
		require.NoError(t, writePhoneVerificationReminder(c, phoneReminderKindResponses, true, "gpt-x", reminderText))
		require.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()
		events := []string{"response.created", "response.output_item.added", "response.content_part.added", "response.output_text.delta", "response.output_text.done", "response.content_part.done", "response.output_item.done", "response.completed"}
		last := -1
		for _, event := range events {
			idx := strings.Index(body, "event: "+event+"\n")
			require.Greater(t, idx, last, "event %s missing or out of order", event)
			last = idx
		}
		lines := sseDataLines(t, body)
		require.Len(t, lines, len(events))
		var delta map[string]any
		require.NoError(t, common.Unmarshal([]byte(lines[3]), &delta))
		require.Equal(t, reminderText, delta["delta"])
		var completed struct {
			Response dto.OpenAIResponsesResponse `json:"response"`
		}
		require.NoError(t, common.Unmarshal([]byte(lines[7]), &completed))
		require.Equal(t, `"completed"`, string(completed.Response.Status))
		require.Equal(t, reminderText, completed.Response.Output[0].Content[0].Text)
		require.NotContains(t, body, "[DONE]")
	})
}

func TestWritePhoneVerificationReminderGemini(t *testing.T) {
	t.Run("non-stream", func(t *testing.T) {
		c, rec := newPhoneReminderRecorder(t)
		require.NoError(t, writePhoneVerificationReminder(c, phoneReminderKindGemini, false, "gemini-x", reminderText))
		require.Equal(t, http.StatusOK, rec.Code)
		var resp dto.GeminiChatResponse
		require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
		require.Len(t, resp.Candidates, 1)
		require.Equal(t, "model", resp.Candidates[0].Content.Role)
		require.Equal(t, reminderText, resp.Candidates[0].Content.Parts[0].Text)
		require.Equal(t, "STOP", *resp.Candidates[0].FinishReason)
		require.Equal(t, 0, resp.UsageMetadata.TotalTokenCount)
	})
	t.Run("stream", func(t *testing.T) {
		c, rec := newPhoneReminderRecorder(t)
		require.NoError(t, writePhoneVerificationReminder(c, phoneReminderKindGemini, true, "gemini-x", reminderText))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
		lines := sseDataLines(t, rec.Body.String())
		require.Len(t, lines, 1)
		var resp dto.GeminiChatResponse
		require.NoError(t, common.Unmarshal([]byte(lines[0]), &resp))
		require.Equal(t, reminderText, resp.Candidates[0].Content.Parts[0].Text)
		require.NotContains(t, rec.Body.String(), "[DONE]")
	})
}

func stubPhoneReminderClaim(t *testing.T, claimed bool, err error) *int {
	t.Helper()
	calls := 0
	original := claimPhoneVerificationReminder
	claimPhoneVerificationReminder = func(userId int) (bool, error) {
		calls++
		return claimed, err
	}
	t.Cleanup(func() { claimPhoneVerificationReminder = original })
	return &calls
}

func newPhoneReminderRelayContext(t *testing.T, path string, eligible bool) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, rec := newPhoneReminderRecorder(t)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}"))
	c.Set("id", 4242)
	common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{Language: "zh-CN"})
	if eligible {
		common.SetContextKey(c, constant.ContextKeyPhoneVerificationReminderEligible, true)
	}
	return c, rec
}

func TestMaybeServePhoneVerificationReminderServesEligibleChatRequest(t *testing.T) {
	require.NoError(t, i18n.Init())
	calls := stubPhoneReminderClaim(t, true, nil)
	c, rec := newPhoneReminderRelayContext(t, "/v1/chat/completions", true)
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI, RelayMode: relayconstant.RelayModeChatCompletions, OriginModelName: "gpt-x", UserId: 4242}
	request := chatReq(t, `{"model":"gpt-x","messages":[{"role":"user","content":"hi"}]}`)

	served := maybeServePhoneVerificationReminder(c, types.RelayFormatOpenAI, info, request)

	require.True(t, served)
	require.Equal(t, 1, *calls)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, phoneVerificationNoticeValue, rec.Header().Get(phoneVerificationNoticeHeader))
	var resp dto.OpenAITextResponse
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
	text := resp.Choices[0].Message.StringContent()
	require.Contains(t, text, "尚未绑定手机号")
	require.Contains(t, text, "http://localhost:3000/")
	require.Contains(t, text, common.SystemName)
}

func TestMaybeServePhoneVerificationReminderIsNoopWithoutFlag(t *testing.T) {
	calls := stubPhoneReminderClaim(t, true, nil)
	c, rec := newPhoneReminderRelayContext(t, "/v1/chat/completions", false)
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI, RelayMode: relayconstant.RelayModeChatCompletions, OriginModelName: "gpt-x", UserId: 4242}
	request := chatReq(t, `{"model":"gpt-x","messages":[]}`)

	require.False(t, maybeServePhoneVerificationReminder(c, types.RelayFormatOpenAI, info, request))
	require.Equal(t, 0, *calls)
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func TestMaybeServePhoneVerificationReminderDoesNotClaimUnsupportedRequests(t *testing.T) {
	calls := stubPhoneReminderClaim(t, true, nil)
	c, rec := newPhoneReminderRelayContext(t, "/v1/embeddings", true)
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatEmbedding, RelayMode: relayconstant.RelayModeEmbeddings, OriginModelName: "emb", UserId: 4242}

	require.False(t, maybeServePhoneVerificationReminder(c, types.RelayFormatEmbedding, info, &dto.EmbeddingRequest{}))
	require.Equal(t, 0, *calls, "unsupported requests must not consume the daily slot")
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func TestMaybeServePhoneVerificationReminderFallsThroughWhenClaimLostOrFails(t *testing.T) {
	for name, tc := range map[string]struct {
		claimed bool
		err     error
	}{
		"lost":  {claimed: false, err: nil},
		"error": {claimed: false, err: errors.New("redis down")},
	} {
		t.Run(name, func(t *testing.T) {
			calls := stubPhoneReminderClaim(t, tc.claimed, tc.err)
			c, rec := newPhoneReminderRelayContext(t, "/v1/messages", true)
			info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatClaude, RelayMode: relayconstant.RelayModeChatCompletions, OriginModelName: "claude-x", UserId: 4242}
			request := claudeReq(t, `{"model":"claude-x","max_tokens":5,"messages":[{"role":"user","content":"hi"}]}`)

			require.False(t, maybeServePhoneVerificationReminder(c, types.RelayFormatClaude, info, request))
			require.Equal(t, 1, *calls)
			require.False(t, c.Writer.Written())
			require.Empty(t, rec.Body.String())
			require.Empty(t, rec.Header().Get(phoneVerificationNoticeHeader))
		})
	}
}

func TestMaybeServePhoneVerificationReminderHonoursStreamFlagAndGeminiPath(t *testing.T) {
	require.NoError(t, i18n.Init())
	stubPhoneReminderClaim(t, true, nil)
	c, rec := newPhoneReminderRelayContext(t, "/v1beta/models/gemini-x:streamGenerateContent", true)
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatGemini, RelayMode: relayconstant.RelayModeGemini, IsStream: true, OriginModelName: "gemini-x", UserId: 4242}
	request := geminiReq(t, `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`)

	require.True(t, maybeServePhoneVerificationReminder(c, types.RelayFormatGemini, info, request))
	require.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
	require.Len(t, sseDataLines(t, rec.Body.String()), 1)
}

func TestClaimPhoneVerificationReminderDefaultsToServiceImplementation(t *testing.T) {
	require.NotNil(t, claimPhoneVerificationReminder)
	// The indirection exists only so tests can stub the 24h slot; production
	// must keep pointing at the service implementation.
	_ = service.ClaimPhoneVerificationReminder
}

func TestRelayCallsPhoneVerificationReminderBeforeBilling(t *testing.T) {
	source, err := os.ReadFile("relay.go")
	require.NoError(t, err)
	text := string(source)
	hook := strings.Index(text, "maybeServePhoneVerificationReminder(c, relayFormat, relayInfo, request)")
	require.Greater(t, hook, 0, "Relay must call the reminder hook")
	require.Less(t, strings.Index(text, "IsModelOfficiallyUnsupported(relayInfo.OriginModelName)"), hook)
	require.Less(t, hook, strings.Index(text, "needSensitiveCheck := setting.ShouldCheckPromptSensitive()"))
	require.Less(t, hook, strings.Index(text, "service.PreConsumeBilling(c, priceData.QuotaToPreConsume, relayInfo)"))
}

// TestRelayServesPhoneVerificationReminderWithoutBillingOrUpstream drives the
// real controller.Relay: an eligible legacy PLG account gets the synthesized
// reminder as a 200, the wallet is untouched and no channel is used. The
// "claim lost -> relay normally" branch is covered by the unit tests above;
// exercising it here would need a full fake upstream.
func TestRelayServesPhoneVerificationReminderWithoutBillingOrUpstream(t *testing.T) {
	require.NoError(t, i18n.Init())
	restoreDB := useControllerAssetTaskDBForTest(t)
	defer restoreDB()
	restorePricing := useControllerAssetTaskPricingForTest(t)
	defer restorePricing()
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"gpt-reminder-test":1}`))
	model.InitChannelCache()

	seedControllerRelayUserToken(t, 7, 11, 10000, 10000)
	const startingQuota = 10000

	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		common.RDB = oldRDB
	})
	claims := 0
	originalClaim := claimPhoneVerificationReminder
	claimPhoneVerificationReminder = func(userId int) (bool, error) {
		claims++
		require.Equal(t, 7, userId)
		return true, nil
	}
	t.Cleanup(func() { claimPhoneVerificationReminder = originalClaim })

	newContext := func() (*gin.Context, *httptest.ResponseRecorder) {
		c, rec := newControllerRelayTaskContext(`{"model":"gpt-reminder-test","messages":[{"role":"user","content":"hi"}]}`)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-reminder-test","messages":[{"role":"user","content":"hi"}]}`))
		c.Request.Header.Set("Content-Type", "application/json")
		common.SetContextKey(c, constant.ContextKeyUserId, 7)
		c.Set("id", 7)
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenId, 11)
		common.SetContextKey(c, constant.ContextKeyTokenKey, "sk-task-token-11")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUserQuota, startingQuota)
		common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: "wallet_only", Language: "en"})
		common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-reminder-test")
		common.SetContextKey(c, constant.ContextKeyPhoneVerificationReminderEligible, true)
		c.Set("token_name", "task-token")
		return c, rec
	}

	c, rec := newContext()
	Relay(c, types.RelayFormatOpenAI)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, phoneVerificationNoticeValue, rec.Header().Get(phoneVerificationNoticeHeader))
	var resp dto.OpenAITextResponse
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "gpt-reminder-test", resp.Model)
	require.Contains(t, resp.Choices[0].Message.StringContent(), "has not bound a phone number")
	require.Equal(t, startingQuota, getControllerUserQuota(t, 7), "reminder must not bill the wallet")
	require.Empty(t, c.GetStringSlice("use_channel"), "reminder must not touch any channel")
	require.Equal(t, 1, claims)
}
