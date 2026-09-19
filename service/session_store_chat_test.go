package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service/sessioncapture"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const chatCaptureRequest = `{"model":"claude-sonnet-4-5","messages":[{"role":"system","content":"keep me"},{"role":"user","content":"hello"}]}`
const chatCaptureResponse = `{"id":"chatcmpl-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"hello","reasoning_content":"reason"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":7,"prompt_tokens_details":{"cached_tokens":3}}}`

func chatCaptureStream() string {
	return strings.Join([]string{
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":null,"reasoning_content":"rea"},"finish_reason":null}]}`,
		`data: {"choices":[{"index":0,"delta":{"content":"hel","reasoning_content":"son"}}]}`,
		`data: {"choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":"stop"}]}`,
		`data: {"choices":[],"usage":{"prompt_tokens":12,"completion_tokens":7,"prompt_tokens_details":{"cached_tokens":3}}}`,
		`data: [DONE]`,
		``,
	}, "\n\n")
}

func TestSessionStoreChatCompletionLocalHTTP(t *testing.T) {
	originalEnabled, originalUser, originalPublish := sessionCaptureEnabled, sessionCaptureAllowedUser, sessionCapturePublish
	sessionCaptureEnabled, sessionCaptureAllowedUser = true, nil
	t.Cleanup(func() {
		sessionCaptureEnabled, sessionCaptureAllowedUser, sessionCapturePublish = originalEnabled, originalUser, originalPublish
	})
	for _, tc := range []struct {
		name, response, status string
	}{
		{"json", chatCaptureResponse, "ok"},
		{"stream", chatCaptureStream(), "ok"},
		{"interrupted", strings.ReplaceAll(chatCaptureStream(), "data: [DONE]", ""), "partial"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			type capture struct {
				meta sessioncapture.Meta
				body []byte
			}
			published := make(chan capture, 1)
			sessionCapturePublish = func(meta sessioncapture.Meta, body []byte) { published <- capture{meta, body} }
			router := gin.New()
			router.POST("/v1/chat/completions", func(c *gin.Context) {
				if !SessionStoreCaptureEnabledForRequest(c.Request.Method, c.Request.URL.Path, "claude-sonnet-4-5", 42) {
					c.AbortWithStatus(http.StatusForbidden)
					return
				}
				common.SetContextKey(c, constant.ContextKeyUserId, 42)
				common.SetContextKey(c, constant.ContextKeyOriginalModel, "claude-sonnet-4-5")
				// Relay reads and caches the request before writing the response.
				if _, err := common.GetBodyStorage(c); err != nil {
					c.AbortWithStatus(http.StatusBadRequest)
					return
				}
				BeginSessionStoreCapture(c)
				recordSessionStoreUsage(c, 42, "claude-sonnet-4-5", 12, 7, 0)
				_, _ = c.Writer.WriteString(tc.response)
				c.Writer.Flush()
				FinishSessionStoreCapture(c, []byte(tc.response), false)
			})
			server := httptest.NewServer(router)
			defer server.Close()
			t.Logf("本地 HTTP 验证地址：%s/v1/chat/completions", server.URL)
			resp, err := http.Post(server.URL+"/v1/chat/completions", "application/json", strings.NewReader(chatCaptureRequest))
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, tc.response, string(body))
			select {
			case got := <-published:
				require.Equal(t, tc.status, got.meta.Status)
				require.Equal(t, "42", got.meta.UserID)
				var record claudeSessionRecord
				require.NoError(t, common.Unmarshal(got.body, &record))
				require.JSONEq(t, chatCaptureRequest, string(record.Request))
				require.JSONEq(t, chatCaptureResponse, string(record.Response))
				require.False(t, record.SignaturePreserved)
				require.EqualValues(t, 12, record.TokenLength.InputTokens)
				require.EqualValues(t, 7, record.TokenLength.OutputTokens)
				require.EqualValues(t, 19, record.TokenLength.TotalTokens)
				require.EqualValues(t, 3, record.TokenLength.CacheRead)
				require.Equal(t, 1, record.Meta.Turns)
			default:
				t.Fatal("request was not captured")
			}
		})
	}
}

func TestChatCaptureInterleavedChoicesAndTools(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"choices":[{"index":1,"delta":{"role":"assistant","tool_calls":[{"index":1,"id":"call_b","type":"function","function":{"name":"second","arguments":"{\"b\":"}},{"index":0,"id":"call_a","type":"function","function":{"name":"first","arguments":"{\"a\":"}}]}},{"index":0,"delta":{"role":"assistant","content":"hello"}}]}`,
		`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"},{"index":1,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}},{"index":1,"function":{"arguments":"2}"}}]},"finish_reason":"tool_calls"}]}`,
		`data: [DONE]`,
	}, "\r\n\r\n")
	_, response, complete, err := canonicalChatCompletionResponse([]byte(stream))
	require.NoError(t, err)
	require.True(t, complete)
	choices := response["choices"].([]any)
	require.Len(t, choices, 2)
	require.EqualValues(t, 0, choices[0].(map[string]any)["index"])
	message := choices[1].(map[string]any)["message"].(map[string]any)
	tools := message["tool_calls"].([]any)
	require.Len(t, tools, 2)
	require.Equal(t, "call_a", tools[0].(map[string]any)["id"])
	require.Equal(t, `{"a":1}`, tools[0].(map[string]any)["function"].(map[string]any)["arguments"])
	require.Equal(t, "call_b", tools[1].(map[string]any)["id"])
	require.Equal(t, `{"b":2}`, tools[1].(map[string]any)["function"].(map[string]any)["arguments"])
}

func TestChatCaptureMalformedAndIncompleteStreams(t *testing.T) {
	for _, body := range []string{"", "data: [DONE]\n\n", "data: {invalid}\n\n", `data: {"choices":[{"index":-1,"delta":{}}]}`, `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0.5}]}}]}`} {
		_, _, _, err := canonicalChatCompletionResponse([]byte(body))
		require.Error(t, err, body)
	}
	_, _, complete, err := canonicalChatCompletionResponse([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"}}]}\n\ndata: [DONE]\n\n"))
	require.NoError(t, err)
	require.False(t, complete)
}

func TestChatCaptureUsageFallbackAndError(t *testing.T) {
	for _, body := range []string{
		`{"choices":[{"index":0,"message":{"content":"ok"},"finish_reason":"stop"}]}`,
		`{"error":{"message":"failed","type":"api_error"}}`,
		"data: {\"error\":{\"message\":\"failed\",\"type\":\"api_error\"}}\n\n",
	} {
		transcript, complete, err := buildSessionStoreTranscript(sessionStorePayload{
			RequestPath: "/v1/chat/completions", RequestBody: []byte(chatCaptureRequest), ResponseBody: []byte(body), PromptTokens: 15, CompletionTokens: 8,
		})
		require.NoError(t, err)
		require.True(t, complete)
		var record claudeSessionRecord
		require.NoError(t, common.Unmarshal(transcript, &record))
		require.EqualValues(t, 15, record.TokenLength.InputTokens)
		require.EqualValues(t, 8, record.TokenLength.OutputTokens)
	}
}
