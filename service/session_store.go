package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	sessionStoreQueueSize          = 256
	sessionStoreWorkerCount        = 4
	sessionStoreUploadAttempts     = 3
	sessionStoreUploadTimeout      = 15 * time.Second
	sessionStoreMaxTranscriptBytes = 10 * 1024 * 1024
	sessionStoreMaxBodyBytes       = sessionStoreMaxTranscriptBytes
)

var (
	sessionStoreURL      = strings.TrimSpace(os.Getenv("SESSION_STORE_URL"))
	sessionStoreTenantID = sessionStoreEnvOrDefault("SESSION_STORE_TENANT_ID", "flatkey")

	sessionStoreQueue       = make(chan sessionStorePayload, sessionStoreQueueSize)
	sessionStoreWorkers     sync.Once
	sessionStoreHTTPClient  = newSessionStoreHTTPClient()
	sessionStoreAsyncRunner = enqueueSessionStorePayload
)

type sessionStoreCaptureState struct {
	mu               sync.Mutex
	startedAt        time.Time
	userID           int
	model            string
	promptTokens     int
	completionTokens int
	quota            int
}

type sessionStorePayload struct {
	Endpoint              string
	TenantID              string
	RequestID             string
	StartedAt             time.Time
	EndedAt               time.Time
	UserID                int
	Model                 string
	Status                string
	PromptTokens          int
	CompletionTokens      int
	Quota                 int
	Method                string
	Path                  string
	RawQuery              string
	RequestHeaders        http.Header
	ResponseHeaders       http.Header
	HTTPStatus            int
	RequestBody           []byte
	ResponseBody          []byte
	RequestBodyTruncated  bool
	ResponseBodyTruncated bool
}

type sessionStoreTranscriptEvent struct {
	Type          string              `json:"type"`
	Timestamp     string              `json:"timestamp"`
	RequestID     string              `json:"request_id"`
	Method        string              `json:"method,omitempty"`
	Path          string              `json:"path,omitempty"`
	Query         map[string][]string `json:"query,omitempty"`
	StatusCode    int                 `json:"status_code,omitempty"`
	Headers       map[string][]string `json:"headers,omitempty"`
	Body          any                 `json:"body,omitempty"`
	BodyTruncated bool                `json:"body_truncated,omitempty"`
}

func sessionStoreEnvOrDefault(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func newSessionStoreHTTPClient() *http.Client {
	return &http.Client{Timeout: sessionStoreUploadTimeout}
}

// SessionStoreCaptureEnabledForRequest limits transcript capture to text-model
// endpoints. Binary/image/audio requests can exceed the sink's 10 MiB limit and
// are not Claude-style session transcripts.
func SessionStoreCaptureEnabledForRequest(method string, path string) bool {
	if sessionStoreURL == "" || method != http.MethodPost {
		return false
	}
	switch path {
	case "/v1/messages",
		"/v1/completions",
		"/v1/chat/completions",
		"/v1/responses",
		"/v1/responses/compact",
		"/pg/chat/completions":
		return true
	}
	if !strings.HasPrefix(path, "/v1beta/models/") && !strings.HasPrefix(path, "/v1/models/") {
		return false
	}
	actionIndex := strings.LastIndex(path, ":")
	if actionIndex < 0 || actionIndex == len(path)-1 {
		return false
	}
	action := path[actionIndex+1:]
	return action == "generateContent" || action == "streamGenerateContent"
}

func BeginSessionStoreCapture(c *gin.Context) {
	if c == nil {
		return
	}
	startedAt := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	state := &sessionStoreCaptureState{
		startedAt: startedAt,
		userID:    common.GetContextKeyInt(c, constant.ContextKeyUserId),
		model:     common.GetContextKeyString(c, constant.ContextKeyOriginalModel),
	}
	common.SetContextKey(c, constant.ContextKeySessionStoreCapture, state)
}

func recordSessionStoreUsage(c *gin.Context, userID int, model string, promptTokens int, completionTokens int, quota int) {
	state, ok := common.GetContextKeyType[*sessionStoreCaptureState](c, constant.ContextKeySessionStoreCapture)
	if !ok || state == nil {
		return
	}
	state.mu.Lock()
	state.userID = userID
	state.model = model
	state.promptTokens = promptTokens
	state.completionTokens = completionTokens
	state.quota = quota
	state.mu.Unlock()
}

// FinishSessionStoreCapture snapshots the request after the relay handler has
// completely finished. The external HTTP request is queued and never blocks the
// user-facing relay response.
func FinishSessionStoreCapture(c *gin.Context, responseBody []byte, responseBodyTruncated bool) {
	if c == nil || c.Request == nil || sessionStoreURL == "" {
		return
	}
	state, ok := common.GetContextKeyType[*sessionStoreCaptureState](c, constant.ContextKeySessionStoreCapture)
	if !ok || state == nil {
		return
	}

	requestBody, requestBodyTruncated, err := readSessionStoreRequestBody(c)
	if err != nil {
		common.SysError("session store failed to snapshot request body: " + err.Error())
		return
	}

	state.mu.Lock()
	startedAt := state.startedAt
	userID := state.userID
	model := state.model
	promptTokens := state.promptTokens
	completionTokens := state.completionTokens
	quota := state.quota
	state.mu.Unlock()

	statusCode := c.Writer.Status()
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	status := "error"
	if statusCode < http.StatusBadRequest {
		status = "ok"
	}
	requestID := c.GetString(common.RequestIdKey)
	if requestID == "" {
		requestID = uuid.NewString()
	}

	payload := sessionStorePayload{
		Endpoint:              sessionStoreURL,
		TenantID:              sessionStoreTenantID,
		RequestID:             requestID,
		StartedAt:             startedAt,
		EndedAt:               time.Now(),
		UserID:                userID,
		Model:                 model,
		Status:                status,
		PromptTokens:          promptTokens,
		CompletionTokens:      completionTokens,
		Quota:                 quota,
		Method:                c.Request.Method,
		Path:                  c.Request.URL.Path,
		RawQuery:              c.Request.URL.RawQuery,
		RequestHeaders:        c.Request.Header.Clone(),
		ResponseHeaders:       c.Writer.Header().Clone(),
		HTTPStatus:            statusCode,
		RequestBody:           requestBody,
		ResponseBody:          append([]byte(nil), responseBody...),
		RequestBodyTruncated:  requestBodyTruncated,
		ResponseBodyTruncated: responseBodyTruncated,
	}
	sessionStoreAsyncRunner(payload)
}

func readSessionStoreRequestBody(c *gin.Context) ([]byte, bool, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, false, err
	}
	if _, err := storage.Seek(0, io.SeekStart); err != nil {
		return nil, false, err
	}
	data, err := io.ReadAll(io.LimitReader(storage, sessionStoreMaxBodyBytes+1))
	_, seekErr := storage.Seek(0, io.SeekStart)
	if err != nil {
		return nil, false, err
	}
	if seekErr != nil {
		return nil, false, seekErr
	}
	truncated := len(data) > sessionStoreMaxBodyBytes
	if truncated {
		data = data[:sessionStoreMaxBodyBytes]
	}
	return data, truncated, nil
}

func enqueueSessionStorePayload(payload sessionStorePayload) {
	sessionStoreWorkers.Do(startSessionStoreWorkers)
	select {
	case sessionStoreQueue <- payload:
	default:
		common.SysError("session store async queue full; dropping transcript request_id=" + payload.RequestID)
	}
}

func startSessionStoreWorkers() {
	for i := 0; i < sessionStoreWorkerCount; i++ {
		gopool.Go(func() {
			for payload := range sessionStoreQueue {
				deliverSessionStorePayload(payload)
			}
		})
	}
}

func deliverSessionStorePayload(payload sessionStorePayload) {
	var lastErr error
	for attempt := 1; attempt <= sessionStoreUploadAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), sessionStoreUploadTimeout)
		lastErr = uploadSessionStorePayload(ctx, payload)
		cancel()
		if lastErr == nil {
			return
		}
		if attempt < sessionStoreUploadAttempts {
			time.Sleep(time.Duration(attempt*attempt) * 250 * time.Millisecond)
		}
	}
	common.SysError(fmt.Sprintf("session store upload failed after %d attempts request_id=%s: %v", sessionStoreUploadAttempts, payload.RequestID, lastErr))
}

func uploadSessionStorePayload(ctx context.Context, payload sessionStorePayload) error {
	transcript, err := buildSessionStoreTranscript(payload)
	if err != nil {
		return err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"session_id": sessionStoreSessionID(payload.TenantID, payload.RequestID),
		"tenant_id":  payload.TenantID,
		"started_at": payload.StartedAt.UTC().Format(time.RFC3339Nano),
		"ended_at":   payload.EndedAt.UTC().Format(time.RFC3339Nano),
		"user_id":    strconv.Itoa(payload.UserID),
		"model":      payload.Model,
		"status":     payload.Status,
		"tokens_in":  strconv.Itoa(payload.PromptTokens),
		"tokens_out": strconv.Itoa(payload.CompletionTokens),
		"cost_usd":   sessionStoreCostUSD(payload.Quota),
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return err
		}
	}
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="transcript"; filename="session.jsonl"`)
	partHeader.Set("Content-Type", "application/x-ndjson")
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return err
	}
	if _, err := part.Write(transcript); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, payload.Endpoint, bytes.NewReader(body.Bytes()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := sessionStoreHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	responsePreview, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(responsePreview)))
	}
	return nil
}

func buildSessionStoreTranscript(payload sessionStorePayload) ([]byte, error) {
	requestBody := append([]byte(nil), payload.RequestBody...)
	responseBody := append([]byte(nil), payload.ResponseBody...)
	requestTruncated := payload.RequestBodyTruncated
	responseTruncated := payload.ResponseBodyTruncated

	for attempt := 0; attempt < 64; attempt++ {
		requestEvent := sessionStoreTranscriptEvent{
			Type:          "request",
			Timestamp:     payload.StartedAt.UTC().Format(time.RFC3339Nano),
			RequestID:     payload.RequestID,
			Method:        payload.Method,
			Path:          payload.Path,
			Query:         redactSessionStoreQuery(payload.RawQuery),
			Headers:       redactSessionStoreHeaders(payload.RequestHeaders),
			Body:          sessionStoreTranscriptBody(requestBody, requestTruncated),
			BodyTruncated: requestTruncated,
		}
		responseEvent := sessionStoreTranscriptEvent{
			Type:          "response",
			Timestamp:     payload.EndedAt.UTC().Format(time.RFC3339Nano),
			RequestID:     payload.RequestID,
			StatusCode:    payload.HTTPStatus,
			Headers:       redactSessionStoreHeaders(payload.ResponseHeaders),
			Body:          sessionStoreTranscriptBody(responseBody, responseTruncated),
			BodyTruncated: responseTruncated,
		}
		requestLine, err := common.Marshal(requestEvent)
		if err != nil {
			return nil, err
		}
		responseLine, err := common.Marshal(responseEvent)
		if err != nil {
			return nil, err
		}
		transcript := make([]byte, 0, len(requestLine)+len(responseLine)+2)
		transcript = append(transcript, requestLine...)
		transcript = append(transcript, '\n')
		transcript = append(transcript, responseLine...)
		transcript = append(transcript, '\n')
		if len(transcript) <= sessionStoreMaxTranscriptBytes {
			return transcript, nil
		}

		overflow := len(transcript) - sessionStoreMaxTranscriptBytes
		trimBytes := overflow + 64*1024
		if len(responseBody) >= len(requestBody) && len(responseBody) > 0 {
			if trimBytes >= len(responseBody) {
				responseBody = nil
			} else {
				responseBody = responseBody[:len(responseBody)-trimBytes]
			}
			responseTruncated = true
		} else if len(requestBody) > 0 {
			if trimBytes >= len(requestBody) {
				requestBody = nil
			} else {
				requestBody = requestBody[:len(requestBody)-trimBytes]
			}
			requestTruncated = true
		} else {
			return nil, fmt.Errorf("session store transcript metadata exceeds %d bytes", sessionStoreMaxTranscriptBytes)
		}
	}
	return nil, fmt.Errorf("unable to fit session store transcript within %d bytes", sessionStoreMaxTranscriptBytes)
}

func sessionStoreTranscriptBody(body []byte, truncated bool) any {
	if len(body) == 0 {
		return nil
	}
	if !truncated && json.Valid(body) {
		return json.RawMessage(body)
	}
	return string(body)
}

func redactSessionStoreHeaders(headers http.Header) map[string][]string {
	if len(headers) == 0 {
		return nil
	}
	result := make(map[string][]string, len(headers))
	for key, values := range headers {
		if isSessionStoreSecretName(key) {
			result[key] = []string{"[redacted]"}
			continue
		}
		result[key] = append([]string(nil), values...)
	}
	return result
}

func redactSessionStoreQuery(rawQuery string) map[string][]string {
	if rawQuery == "" {
		return nil
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return map[string][]string{"_raw": {"[invalid query]"}}
	}
	for key := range values {
		if isSessionStoreSecretName(key) {
			values[key] = []string{"[redacted]"}
		}
	}
	return values
}

func isSessionStoreSecretName(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	switch normalized {
	case "authorization", "proxy_authorization", "cookie", "set_cookie", "x_api_key", "api_key", "apikey", "x_goog_api_key", "key", "access_token", "refresh_token", "id_token", "password", "secret":
		return true
	default:
		return strings.HasSuffix(normalized, "_api_key") || strings.HasSuffix(normalized, "_token") || strings.HasSuffix(normalized, "_secret")
	}
}

func sessionStoreSessionID(tenantID string, requestID string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(tenantID+"\x00"+requestID)).String()
}

func sessionStoreCostUSD(quota int) string {
	if common.QuotaPerUnit <= 0 {
		return "0"
	}
	return strconv.FormatFloat(float64(quota)/common.QuotaPerUnit, 'f', 8, 64)
}
