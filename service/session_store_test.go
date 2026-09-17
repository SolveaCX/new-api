package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSessionStoreCaptureEnabledForRequest(t *testing.T) {
	originalURL := sessionStoreURL
	sessionStoreURL = "http://session-store.test/v1/sessions"
	t.Cleanup(func() { sessionStoreURL = originalURL })

	tests := []struct {
		method string
		path   string
		want   bool
	}{
		{http.MethodPost, "/v1/messages", true},
		{http.MethodPost, "/v1/chat/completions", true},
		{http.MethodPost, "/v1/responses", true},
		{http.MethodPost, "/v1beta/models/gemini-2.5-pro:generateContent", true},
		{http.MethodPost, "/v1/models/gemini-2.5-pro:streamGenerateContent", true},
		{http.MethodPost, "/v1beta/models/gemini-2.5-pro:embedContent", false},
		{http.MethodPost, "/v1/images/generations", false},
		{http.MethodGet, "/v1/messages", false},
	}
	for _, test := range tests {
		require.Equal(t, test.want, SessionStoreCaptureEnabledForRequest(test.method, test.path), test.method+" "+test.path)
	}
}

func TestFinishSessionStoreCaptureSnapshotsUsageAndBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalURL := sessionStoreURL
	originalTenant := sessionStoreTenantID
	originalRunner := sessionStoreAsyncRunner
	sessionStoreURL = "http://session-store.test/v1/sessions"
	sessionStoreTenantID = "flatkey-test"
	var captured sessionStorePayload
	sessionStoreAsyncRunner = func(payload sessionStorePayload) { captured = payload }
	t.Cleanup(func() {
		sessionStoreURL = originalURL
		sessionStoreTenantID = originalTenant
		sessionStoreAsyncRunner = originalRunner
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages?key=client-secret&beta=true", strings.NewReader(`{"model":"claude-test","messages":[{"role":"user","content":"hello"}]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer client-secret")
	c.Set(common.RequestIdKey, "request-123")
	common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Unix(1_700_000_000, 0))
	common.SetContextKey(c, constant.ContextKeyUserId, 42)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "claude-test")

	BeginSessionStoreCapture(c)
	recordSessionStoreUsage(c, 42, "claude-test", 120, 34, int(common.QuotaPerUnit))
	c.Status(http.StatusOK)
	FinishSessionStoreCapture(c, []byte(`{"id":"msg_1","content":[{"type":"text","text":"world"}]}`), false)

	require.Equal(t, "request-123", captured.RequestID)
	require.Equal(t, "flatkey-test", captured.TenantID)
	require.Equal(t, 42, captured.UserID)
	require.Equal(t, "claude-test", captured.Model)
	require.Equal(t, "ok", captured.Status)
	require.Equal(t, 120, captured.PromptTokens)
	require.Equal(t, 34, captured.CompletionTokens)
	require.JSONEq(t, `{"model":"claude-test","messages":[{"role":"user","content":"hello"}]}`, string(captured.RequestBody))
	require.JSONEq(t, `{"id":"msg_1","content":[{"type":"text","text":"world"}]}`, string(captured.ResponseBody))
}

func TestFinishSessionStoreCaptureKeepsSuccessfulZeroUsageSessionOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalURL := sessionStoreURL
	originalRunner := sessionStoreAsyncRunner
	sessionStoreURL = "http://session-store.test/v1/sessions"
	var captured sessionStorePayload
	sessionStoreAsyncRunner = func(payload sessionStorePayload) { captured = payload }
	t.Cleanup(func() {
		sessionStoreURL = originalURL
		sessionStoreAsyncRunner = originalRunner
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-test","messages":[]}`))
	BeginSessionStoreCapture(c)
	c.Status(http.StatusOK)
	FinishSessionStoreCapture(c, []byte(`{"content":[]}`), false)

	require.Equal(t, "ok", captured.Status)
	require.Zero(t, captured.PromptTokens)
	require.Zero(t, captured.CompletionTokens)
}

func TestBuildSessionStoreTranscriptRedactsCredentials(t *testing.T) {
	payload := sessionStorePayload{
		RequestID:       "req-redact",
		StartedAt:       time.Unix(1_700_000_000, 0),
		EndedAt:         time.Unix(1_700_000_001, 0),
		Method:          http.MethodPost,
		Path:            "/v1/messages",
		RawQuery:        "key=query-secret&beta=true",
		RequestHeaders:  http.Header{"Authorization": []string{"Bearer header-secret"}, "Anthropic-Version": []string{"2023-06-01"}},
		ResponseHeaders: http.Header{"Set-Cookie": []string{"session=secret"}, "Content-Type": []string{"text/event-stream"}},
		HTTPStatus:      http.StatusOK,
		RequestBody:     []byte(`{"messages":[{"role":"user","content":"hello"}]}`),
		ResponseBody:    []byte("event: message\ndata: {\"text\":\"world\"}\n\n"),
	}

	transcript, err := buildSessionStoreTranscript(payload)
	require.NoError(t, err)
	require.NotContains(t, string(transcript), "header-secret")
	require.NotContains(t, string(transcript), "query-secret")
	require.NotContains(t, string(transcript), "session=secret")
	require.Contains(t, string(transcript), "[redacted]")
	require.Contains(t, string(transcript), "hello")
	require.Contains(t, string(transcript), "world")

	lines := strings.Split(strings.TrimSpace(string(transcript)), "\n")
	require.Len(t, lines, 2)
	var requestEvent map[string]any
	require.NoError(t, common.Unmarshal([]byte(lines[0]), &requestEvent))
	require.Equal(t, "request", requestEvent["type"])
	var responseEvent map[string]any
	require.NoError(t, common.Unmarshal([]byte(lines[1]), &responseEvent))
	require.Equal(t, "response", responseEvent["type"])
}

func TestUploadSessionStorePayloadMultipartContract(t *testing.T) {
	requestReceived := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, r.ParseMultipartForm(12*1024*1024))
		require.Equal(t, "flatkey-test", r.FormValue("tenant_id"))
		require.Equal(t, "7", r.FormValue("user_id"))
		require.Equal(t, "claude-test", r.FormValue("model"))
		require.Equal(t, "ok", r.FormValue("status"))
		require.Equal(t, "10", r.FormValue("tokens_in"))
		require.Equal(t, "20", r.FormValue("tokens_out"))
		require.NotEmpty(t, r.FormValue("session_id"))
		file, _, err := r.FormFile("transcript")
		require.NoError(t, err)
		defer file.Close()
		body, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Contains(t, string(body), `"type":"request"`)
		require.Contains(t, string(body), `"type":"response"`)
		requestReceived <- struct{}{}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"session_id":"stored"}`))
	}))
	defer server.Close()

	payload := sessionStorePayload{
		Endpoint:         server.URL,
		TenantID:         "flatkey-test",
		RequestID:        "request-upload",
		StartedAt:        time.Unix(1_700_000_000, 0),
		EndedAt:          time.Unix(1_700_000_001, 0),
		UserID:           7,
		Model:            "claude-test",
		Status:           "ok",
		PromptTokens:     10,
		CompletionTokens: 20,
		Quota:            int(common.QuotaPerUnit),
		Method:           http.MethodPost,
		Path:             "/v1/messages",
		HTTPStatus:       http.StatusOK,
		RequestBody:      []byte(`{"messages":[]}`),
		ResponseBody:     []byte(`{"content":[]}`),
	}
	require.NoError(t, uploadSessionStorePayload(context.Background(), payload))
	select {
	case <-requestReceived:
	case <-time.After(time.Second):
		t.Fatal("session store server did not receive multipart upload")
	}
}

func TestDeliverSessionStorePayloadAttemptsOnce(t *testing.T) {
	for _, test := range []struct {
		name       string
		statusCode int
		err        error
	}{
		{name: "success", statusCode: http.StatusCreated},
		{name: "server failure", statusCode: http.StatusServiceUnavailable},
		{name: "network failure", err: io.ErrUnexpectedEOF},
		{name: "timeout", err: context.DeadlineExceeded},
	} {
		t.Run(test.name, func(t *testing.T) {
			originalClient := sessionStoreHTTPClient
			t.Cleanup(func() { sessionStoreHTTPClient = originalClient })
			attempts := 0
			sessionStoreHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				attempts++
				if test.err != nil {
					return nil, test.err
				}
				return &http.Response{
					StatusCode: test.statusCode,
					Body:       io.NopCloser(strings.NewReader("session store response")),
					Header:     make(http.Header),
				}, nil
			})}

			deliverSessionStorePayload(sessionStorePayload{
				Endpoint:  "http://session-store.test/v1/sessions",
				TenantID:  "flatkey-test",
				RequestID: "single-attempt-" + test.name,
			})

			require.Equal(t, 1, attempts, "session uploads must not retry after a failure")
		})
	}
}
