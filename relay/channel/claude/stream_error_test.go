package claude

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClaudeStreamHandlerEmbeddedError(t *testing.T) {
	const stream = "event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_overlimit\",\"model\":\"claude-fable-5\",\"usage\":{\"input_tokens\":0,\"output_tokens\":0}}}\n\n" +
		"event: content_block_start\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"\\n\\n[Error: The model returned the following errors: prompt is too long: 1251057 tokens > 1000000 maximum]\"}}\n\n" +
		"event: message_delta\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":1}}\n\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\"}\n\n"

	recorder, apiErr := runClaudeStreamFixture(t, stream)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	require.Equal(t, "invalid_request_error", apiErr.ToClaudeError().Type)
	require.Contains(t, apiErr.ToClaudeError().Message, "prompt is too long")
	require.True(t, types.IsSkipRetryError(apiErr))
	require.False(t, recorder.Flushed, "embedded upstream error must not commit SSE headers")
	require.Empty(t, recorder.Body.String())
}

func TestClaudeStreamHandlerEmbeddedErrorSplitAcrossDeltas(t *testing.T) {
	const stream = "event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"\\n\\n[Error: The model returned the following errors: prompt is too long: 1251057\"}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\" tokens > 1000000 maximum]\"}}\n\n"

	recorder, apiErr := runClaudeStreamFixture(t, stream)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	require.Equal(t, "invalid_request_error", apiErr.ToClaudeError().Type)
	require.Contains(t, apiErr.ToClaudeError().Message, "prompt is too long")
	require.True(t, types.IsSkipRetryError(apiErr))
	require.False(t, recorder.Flushed)
	require.Empty(t, recorder.Body.String())
}

func TestClaudeStreamHandlerEmbeddedErrorAfterNormalPrelude(t *testing.T) {
	const stream = "event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_overlimit_late\",\"model\":\"claude-fable-5\"}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"normal prelude\"}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"\\n\\n[Error: The model returned the following errors: prompt is too long: 1251057 tokens > 1000000 maximum]\"}}\n\n" +
		"data: [DONE]\n\n"

	recorder, apiErr := runClaudeStreamFixture(t, stream)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	require.Equal(t, "invalid_request_error", apiErr.ToClaudeError().Type)
	require.Contains(t, apiErr.ToClaudeError().Message, "prompt is too long")
	require.True(t, types.IsSkipRetryError(apiErr))
	require.False(t, recorder.Flushed, "embedded upstream error must not commit SSE headers")
	require.Empty(t, recorder.Body.String())
}

func TestClaudeStreamHandlerFormalError(t *testing.T) {
	const stream = "event: error\n" +
		"data: {\"type\":\"error\",\"error\":{\"type\":\"invalid_request_error\",\"message\":\"prompt is too long: 1251057 tokens > 1000000 maximum\"}}\n\n"

	recorder, apiErr := runClaudeStreamFixture(t, stream)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	require.Equal(t, "invalid_request_error", apiErr.ToClaudeError().Type)
	require.Contains(t, apiErr.ToClaudeError().Message, "prompt is too long")
	require.True(t, types.IsSkipRetryError(apiErr))
	require.False(t, recorder.Flushed, "formal upstream error must not commit SSE headers")
	require.Empty(t, recorder.Body.String())
}

func TestClaudeStreamHandlerTransientFormalErrorKeepsRetryPolicy(t *testing.T) {
	const stream = "event: error\n" +
		"data: {\"type\":\"error\",\"error\":{\"type\":\"rate_limit_error\",\"message\":\"slow down\"}}\n\n"

	recorder, apiErr := runClaudeStreamFixture(t, stream)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
	require.False(t, types.IsSkipRetryError(apiErr), "transient upstream errors must remain governed by retry policy")
	require.False(t, recorder.Flushed)
}

func TestClaudeStreamHandlerErrorClearsPrecommittedSSEHeaders(t *testing.T) {
	const stream = "event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"[Error: The model returned the following errors: prompt is too long: 1251057 tokens > 1000000 maximum]\"}}\n\n"

	recorder, c, resp, info := newClaudeStreamFixture(t, stream)
	helper.SetEventStreamHeaders(c)
	_, apiErr := ClaudeStreamHandler(c, resp, info)

	require.NotNil(t, apiErr)
	require.False(t, recorder.Flushed)
	require.Empty(t, recorder.Header().Get("Content-Type"))
	require.NotContains(t, c.Keys, "event_stream_headers_set")

	// This is what controller.Relay does after the handler returns. The JSON
	// renderer only sets its content type when the stale SSE header is gone.
	c.JSON(apiErr.StatusCode, gin.H{"type": "error", "error": apiErr.ToClaudeError()})
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
}

func TestClaudeStreamHandlerNormalStream(t *testing.T) {
	const stream = "event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_ok\",\"model\":\"claude-fable-5\",\"usage\":{\"input_tokens\":3,\"output_tokens\":0}}}\n\n" +
		"event: content_block_start\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n" +
		"event: message_delta\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"input_tokens\":3,\"output_tokens\":1}}\n\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\"}\n\n" +
		"data: [DONE]\n\n"

	recorder, apiErr := runClaudeStreamFixture(t, stream)

	require.Nil(t, apiErr)
	require.True(t, recorder.Flushed)
	require.Contains(t, recorder.Body.String(), "hello")
	require.Contains(t, recorder.Body.String(), "message_start")
}

func TestClaudeStreamHandlerFableNormalText(t *testing.T) {
	const stream = "event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello fable\"}}\n\n" +
		"data: [DONE]\n\n"

	recorder, apiErr := runClaudeStreamFixture(t, stream)

	require.Nil(t, apiErr)
	require.True(t, recorder.Flushed)
	require.Contains(t, recorder.Body.String(), "hello fable")
	require.NotContains(t, recorder.Body.String(), "invalid_request_error")
}

func TestClaudeStreamHandlerFableEOFWithoutDone(t *testing.T) {
	const stream = "event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello fable\"}}\n\n"

	recorder, apiErr := runClaudeStreamFixture(t, stream)

	require.Nil(t, apiErr)
	require.True(t, recorder.Flushed)
	require.Contains(t, recorder.Body.String(), "hello fable")
	require.NotContains(t, recorder.Body.String(), "did not terminate")
}

func TestClaudeStreamHandlerGateDoesNotLetPingCommitEarly(t *testing.T) {
	setting := operation_setting.GetGeneralSetting()
	oldEnabled := setting.PingIntervalEnabled
	oldSeconds := setting.PingIntervalSeconds
	setting.PingIntervalEnabled = true
	setting.PingIntervalSeconds = 1
	t.Cleanup(func() {
		setting.PingIntervalEnabled = oldEnabled
		setting.PingIntervalSeconds = oldSeconds
	})

	oldStreamingTimeout := constant.StreamingTimeout
	oldFirstResponseTimeout := constant.StreamingFirstResponseTimeout
	constant.StreamingTimeout = 3
	constant.StreamingFirstResponseTimeout = 3
	t.Cleanup(func() {
		constant.StreamingTimeout = oldStreamingTimeout
		constant.StreamingFirstResponseTimeout = oldFirstResponseTimeout
	})

	pr, pw := io.Pipe()
	started := make(chan struct{})
	go func() {
		defer pw.Close()
		close(started)
		time.Sleep(1200 * time.Millisecond)
		_, _ = fmt.Fprint(pw, "event: content_block_delta\n")
		_, _ = fmt.Fprint(pw, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello gated ping\"}}\n\n")
		time.Sleep(1500 * time.Millisecond)
		_, _ = fmt.Fprint(pw, "data: [DONE]\n\n")
	}()
	<-started

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       pr,
	}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		IsStream:    true,
		DisablePing: false,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-fable-5"},
	}

	done := make(chan struct{})
	var apiErr *types.NewAPIError
	go func() {
		_, apiErr = ClaudeStreamHandler(c, resp, info)
		close(done)
	}()

	time.Sleep(400 * time.Millisecond)
	require.False(t, recorder.Flushed)
	require.Empty(t, recorder.Body.String())

	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("timed out waiting for gated ping stream to finish")
	}

	require.Nil(t, apiErr)
	require.True(t, recorder.Flushed)
	require.Contains(t, recorder.Body.String(), "hello gated ping")
}

func TestClaudeStreamHandlerNonFableEmbeddedTextRemainsNormalStream(t *testing.T) {
	const stream = "event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"[Error: The model returned the following errors: prompt is too long: 1251057 tokens > 1000000 maximum]\"}}\n\n" +
		"data: [DONE]\n\n"

	recorder, c, resp, info := newClaudeStreamFixture(t, stream)
	info.UpstreamModelName = "claude-3-5-sonnet"
	info.ChannelMeta.UpstreamModelName = info.UpstreamModelName
	_, apiErr := ClaudeStreamHandler(c, resp, info)

	require.Nil(t, apiErr)
	require.True(t, recorder.Flushed)
	require.Contains(t, recorder.Body.String(), "prompt is too long")
}

func TestClaudeStreamHandlerRejectsTruncatedFablePrelude(t *testing.T) {
	const stream = "event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_truncated\"}}\n\n"

	recorder, apiErr := runClaudeStreamFixture(t, stream)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.False(t, recorder.Flushed)
	require.Empty(t, recorder.Body.String())
}

func TestClaudeStreamHandlerRejectsUnterminatedEmbeddedError(t *testing.T) {
	var stream strings.Builder
	for i := 0; i < 32; i++ {
		text := "x"
		if i == 0 {
			text = "[Error: The model returned the following errors: prompt is too long: "
		}
		fmt.Fprintf(&stream, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":%q}}\n\n", text)
	}

	recorder, apiErr := runClaudeStreamFixture(t, stream.String())

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Contains(t, apiErr.Error(), "did not terminate")
	require.False(t, recorder.Flushed)
	require.Empty(t, recorder.Body.String())
}

func runClaudeStreamFixture(t *testing.T, stream string) (*httptest.ResponseRecorder, *types.NewAPIError) {
	recorder, c, resp, info := newClaudeStreamFixture(t, stream)
	_, apiErr := ClaudeStreamHandler(c, resp, info)
	return recorder, apiErr
}

func newClaudeStreamFixture(t *testing.T, stream string) (*httptest.ResponseRecorder, *gin.Context, *http.Response, *relaycommon.RelayInfo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	oldStreamingTimeout := constant.StreamingTimeout
	oldFirstResponseTimeout := constant.StreamingFirstResponseTimeout
	constant.StreamingTimeout = 2
	constant.StreamingFirstResponseTimeout = 2
	t.Cleanup(func() {
		constant.StreamingTimeout = oldStreamingTimeout
		constant.StreamingFirstResponseTimeout = oldFirstResponseTimeout
	})

	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		IsStream:    true,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-fable-5",
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(stream)),
	}
	return recorder, c, resp, info
}
