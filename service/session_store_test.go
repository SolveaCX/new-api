package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service/sessioncapture"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSessionStoreCaptureEnabledForRequest(t *testing.T) {
	originalEnabled := sessionCaptureEnabled
	originalAllowedUser := sessionCaptureAllowedUser
	sessionCaptureEnabled = true
	sessionCaptureAllowedUser = nil
	t.Cleanup(func() {
		sessionCaptureEnabled = originalEnabled
		sessionCaptureAllowedUser = originalAllowedUser
	})

	tests := []struct {
		method string
		path   string
		model  string
		want   bool
	}{
		{http.MethodPost, "/v1/messages", "claude-opus-4-6-20260115", true},
		{http.MethodPost, "/v1/messages", "anthropic/claude-opus-4-8-max", true},
		{http.MethodPost, "/v1/messages", "claude-opus-5-20260815", true},
		{http.MethodPost, "/v1/messages", "claude-sonnet-3-5", true},
		{http.MethodPost, "/v1/messages", "claude-opus-4-5", false},
		{http.MethodPost, "/v1/messages", "claude-haiku-4-6", false},
		{http.MethodPost, "/v1/chat/completions", "claude-sonnet-4-5", true},
		{http.MethodPost, "/v1/chat/completions", "anthropic/claude-opus-4.6", true},
		{http.MethodPost, "/v1/chat/completions", "claude-opus-4-5", false},
		{http.MethodPost, "/v1/chat/completions", "claude-haiku-4-6", false},
		{http.MethodPost, "/v1/chat/completions", "gpt-4o", false},
		{http.MethodGet, "/v1/chat/completions", "claude-sonnet-4-5", false},
		{http.MethodPost, "/v1/responses", "claude-sonnet-4-5", false},
		{http.MethodGet, "/v1/messages", "claude-sonnet-4-5", false},
	}
	for _, test := range tests {
		require.Equal(t, test.want, SessionStoreCaptureEnabledForRequest(test.method, test.path, test.model, 42), test.method+" "+test.path+" "+test.model)
	}

	allowedUserID := 42
	sessionCaptureAllowedUser = &allowedUserID
	require.True(t, SessionStoreCaptureEnabledForRequest(http.MethodPost, "/v1/messages", "claude-sonnet-4-5", 42))
	require.False(t, SessionStoreCaptureEnabledForRequest(http.MethodPost, "/v1/messages", "claude-sonnet-4-5", 41))
	require.True(t, SessionStoreCaptureEnabledForRequest(http.MethodPost, "/v1/chat/completions", "claude-sonnet-4-5", 42))
	require.False(t, SessionStoreCaptureEnabledForRequest(http.MethodPost, "/v1/chat/completions", "claude-sonnet-4-5", 41))
	sessionCaptureEnabled = false
	require.False(t, SessionStoreCaptureEnabledForRequest(http.MethodPost, "/v1/chat/completions", "claude-sonnet-4-5", 42))
}

func TestSessionStoreOptionalUserIDFailsClosed(t *testing.T) {
	t.Setenv("SESSION_CAPTURE_USER_ID", "invalid")
	userID := sessionStoreOptionalUserID("SESSION_CAPTURE_USER_ID")
	require.NotNil(t, userID)
	require.Zero(t, *userID)
}

func TestBuildSessionStoreTranscriptCanonicalNonStream(t *testing.T) {
	payload := sessionStorePayload{
		SessionID:        "session-non-stream",
		TenantID:         "flatkey-test",
		StartedAt:        time.Date(2026, 9, 18, 1, 2, 3, 0, time.UTC),
		EndedAt:          time.Date(2026, 9, 18, 1, 2, 8, 0, time.UTC),
		PromptTokens:     100,
		CompletionTokens: 20,
		Quota:            int(common.QuotaPerUnit),
		RequestBody:      []byte(`{"model":"claude-opus-4-8-20260601","system":"keep me","messages":[{"role":"user","content":"first"},{"role":"assistant","content":"answer"},{"role":"user","content":"second"}]}`),
		ResponseBody:     []byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-4-8-20260601","content":[{"type":"thinking","thinking":"reason","signature":"signature-verbatim"},{"type":"text","text":"done"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":101,"output_tokens":21,"cache_creation_input_tokens":9,"cache_read_input_tokens":7}}`),
	}

	transcript, complete, err := buildSessionStoreTranscript(payload)
	require.NoError(t, err)
	require.True(t, complete)
	require.True(t, strings.HasSuffix(string(transcript), "\n"))
	require.NotContains(t, strings.TrimSuffix(string(transcript), "\n"), "\n")

	var record claudeSessionRecord
	require.NoError(t, common.Unmarshal(transcript, &record))
	require.Equal(t, "anthropic", record.Provider)
	require.Equal(t, "claude-opus-4-8-20260601", record.Model)
	require.True(t, record.SignaturePreserved)
	require.EqualValues(t, 101, record.TokenLength.InputTokens)
	require.EqualValues(t, 21, record.TokenLength.OutputTokens)
	require.EqualValues(t, 122, record.TokenLength.TotalTokens)
	require.EqualValues(t, 9, record.TokenLength.CacheCreation)
	require.EqualValues(t, 7, record.TokenLength.CacheRead)
	require.Equal(t, 2, record.Meta.Turns)
	require.InDelta(t, 1, record.Meta.CostUSD, 0.0000001)

	var response map[string]any
	require.NoError(t, common.Unmarshal(record.Response, &response))
	content := response["content"].([]any)
	thinking := content[0].(map[string]any)
	require.Equal(t, "signature-verbatim", thinking["signature"])
}

func TestBuildSessionStoreTranscriptAssemblesClaudeStream(t *testing.T) {
	stream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_stream","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":12,"output_tokens":1,"cache_creation_input_tokens":4,"cache_read_input_tokens":3}}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"reason "}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"continued"}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"signed-"}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"verbatim"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"lookup","input":{}}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"query\":"}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"\"hello\"}"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":1}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":17}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")
	payload := sessionStorePayload{
		SessionID:    "session-stream",
		TenantID:     "flatkey-test",
		RequestBody:  []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"hello"}]}`),
		ResponseBody: []byte(stream),
	}

	transcript, complete, err := buildSessionStoreTranscript(payload)
	require.NoError(t, err)
	require.True(t, complete)
	var record claudeSessionRecord
	require.NoError(t, common.Unmarshal(transcript, &record))
	require.EqualValues(t, 12, record.TokenLength.InputTokens)
	require.EqualValues(t, 17, record.TokenLength.OutputTokens)
	require.EqualValues(t, 4, record.TokenLength.CacheCreation)
	require.EqualValues(t, 3, record.TokenLength.CacheRead)

	var response map[string]any
	require.NoError(t, common.Unmarshal(record.Response, &response))
	require.Equal(t, "tool_use", response["stop_reason"])
	content := response["content"].([]any)
	thinking := content[0].(map[string]any)
	require.Equal(t, "reason continued", thinking["thinking"])
	require.Equal(t, "signed-verbatim", thinking["signature"])
	tool := content[1].(map[string]any)
	require.Equal(t, "hello", tool["input"].(map[string]any)["query"])
}

func TestBuildSessionStoreTranscriptRejectsTruncation(t *testing.T) {
	_, _, err := buildSessionStoreTranscript(sessionStorePayload{RequestBodyTruncated: true})
	require.ErrorIs(t, err, errSessionCaptureTruncated)
	_, _, err = buildSessionStoreTranscript(sessionStorePayload{ResponseBodyTruncated: true})
	require.ErrorIs(t, err, errSessionCaptureTruncated)
}

func TestFinishSessionStoreCapturePublishesCanonicalRecord(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalEnabled := sessionCaptureEnabled
	originalTenant := sessionCaptureTenantID
	originalPublish := sessionCapturePublish
	sessionCaptureEnabled = true
	sessionCaptureTenantID = "flatkey-test"
	var capturedMeta sessioncapture.Meta
	var capturedTranscript []byte
	sessionCapturePublish = func(meta sessioncapture.Meta, transcript []byte) {
		capturedMeta = meta
		capturedTranscript = append([]byte(nil), transcript...)
	}
	t.Cleanup(func() {
		sessionCaptureEnabled = originalEnabled
		sessionCaptureTenantID = originalTenant
		sessionCapturePublish = originalPublish
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"hello"}]}`))
	c.Set(common.RequestIdKey, "request-123")
	common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Unix(1_700_000_000, 0))
	common.SetContextKey(c, constant.ContextKeyUserId, 42)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "claude-sonnet-4-5")

	BeginSessionStoreCapture(c)
	recordSessionStoreUsage(c, 42, "claude-sonnet-4-5", 120, 34, int(common.QuotaPerUnit))
	c.Status(http.StatusOK)
	FinishSessionStoreCapture(c, []byte(`{"id":"msg_1","type":"message","content":[],"usage":{"input_tokens":120,"output_tokens":34}}`), false)

	require.Equal(t, "request-123", capturedMeta.SessionID)
	require.Equal(t, "flatkey-test", capturedMeta.TenantID)
	require.Equal(t, "42", capturedMeta.UserID)
	require.Equal(t, "ok", capturedMeta.Status)
	require.EqualValues(t, 120, capturedMeta.TokensIn)
	require.EqualValues(t, 34, capturedMeta.TokensOut)
	require.InDelta(t, 1, capturedMeta.CostUSD, 0.0000001)
	require.NotEmpty(t, capturedTranscript)

	var record claudeSessionRecord
	require.NoError(t, common.Unmarshal(capturedTranscript, &record))
	require.Equal(t, "request-123", record.SessionID)
}

func TestCanonicalClaudeResponseRejectsMalformedToolDelta(t *testing.T) {
	stream := []byte("data: {\"type\":\"message_start\",\"message\":{\"type\":\"message\"}}\n\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"input\":{}}}\n\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\"}}\n\n" +
		"data: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
	_, _, _, err := canonicalClaudeResponse(stream)
	require.Error(t, err)
	require.False(t, errors.Is(err, errSessionCaptureTruncated))
}
