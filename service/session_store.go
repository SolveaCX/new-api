package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service/sessioncapture"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const sessionStoreMaxBodyBytes = 10 * 1024 * 1024

var (
	sessionCaptureEnabled     = sessioncapture.Enabled()
	sessionCaptureTenantID    = sessionStoreEnvOrDefault("SESSION_CAPTURE_TENANT_ID", "flatkey")
	sessionCaptureAllowedUser = sessionStoreOptionalUserID("SESSION_CAPTURE_USER_ID")
	sessionCapturePublish     = sessioncapture.Capture
)

var errSessionCaptureTruncated = errors.New("session capture body was truncated")

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
	SessionID             string
	TenantID              string
	StartedAt             time.Time
	EndedAt               time.Time
	UserID                int
	Model                 string
	Status                string
	PromptTokens          int
	CompletionTokens      int
	Quota                 int
	HTTPStatus            int
	RequestBody           []byte
	ResponseBody          []byte
	RequestBodyTruncated  bool
	ResponseBodyTruncated bool
}

type claudeSessionRecord struct {
	SessionID          string            `json:"session_id"`
	CapturedAt         string            `json:"captured_at"`
	Provider           string            `json:"provider"`
	Model              string            `json:"model"`
	Request            json.RawMessage   `json:"request"`
	Response           json.RawMessage   `json:"response"`
	TokenLength        claudeTokenInfo   `json:"token_length"`
	SignaturePreserved bool              `json:"signature_preserved"`
	Meta               claudeSessionMeta `json:"meta"`
}

type claudeTokenInfo struct {
	InputTokens   int64 `json:"input_tokens"`
	OutputTokens  int64 `json:"output_tokens"`
	TotalTokens   int64 `json:"total_tokens"`
	CacheCreation int64 `json:"cache_creation"`
	CacheRead     int64 `json:"cache_read"`
}

type claudeSessionMeta struct {
	TenantID  string  `json:"tenant_id"`
	CostUSD   float64 `json:"cost_usd"`
	StartedAt string  `json:"started_at"`
	EndedAt   string  `json:"ended_at"`
	Turns     int     `json:"turns"`
}

type claudeRequestSummary struct {
	Model    string `json:"model"`
	Messages []struct {
		Role string `json:"role"`
	} `json:"messages"`
}

func sessionStoreEnvOrDefault(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func sessionStoreOptionalUserID(key string) *int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}
	userID, err := strconv.Atoi(value)
	if err != nil || userID <= 0 {
		invalidUserID := 0
		return &invalidUserID
	}
	return &userID
}

// SessionStoreCaptureEnabledForRequest keeps the response-copying middleware
// off unless the new capture pipeline is enabled and the request is a native
// Anthropic Messages request for a customer-selected Claude family.
func SessionStoreCaptureEnabledForRequest(method string, path string, model string, userID int) bool {
	if !sessionCaptureEnabled || method != http.MethodPost || path != "/v1/messages" || !isClaudeSessionCaptureModel(model) {
		return false
	}
	return sessionCaptureAllowedUser == nil || *sessionCaptureAllowedUser == userID
}

func isClaudeSessionCaptureModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if slash := strings.LastIndex(model, "/"); slash >= 0 {
		model = model[slash+1:]
	}
	if strings.HasPrefix(model, "claude-sonnet-") {
		return true
	}
	const opusPrefix = "claude-opus-"
	if !strings.HasPrefix(model, opusPrefix) {
		return false
	}
	version := strings.ReplaceAll(strings.TrimPrefix(model, opusPrefix), ".", "-")
	parts := strings.Split(version, "-")
	if len(parts) == 0 {
		return false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	if major > 4 {
		return true
	}
	if major < 4 || len(parts) < 2 {
		return false
	}
	minor, err := strconv.Atoi(parts[1])
	return err == nil && minor >= 6
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

// FinishSessionStoreCapture snapshots the complete request and client-visible
// Claude response, builds the canonical JSONL record, then hands it to the
// best-effort GCS/PubSub publisher. Storage failures never propagate upward.
func FinishSessionStoreCapture(c *gin.Context, responseBody []byte, responseBodyTruncated bool) {
	if c == nil || c.Request == nil || !sessionCaptureEnabled {
		return
	}
	state, ok := common.GetContextKeyType[*sessionStoreCaptureState](c, constant.ContextKeySessionStoreCapture)
	if !ok || state == nil {
		return
	}

	requestBody, requestBodyTruncated, err := readSessionStoreRequestBody(c)
	if err != nil {
		common.SysError("session capture failed to snapshot request body: " + err.Error())
		return
	}

	state.mu.Lock()
	payload := sessionStorePayload{
		SessionID:             c.GetString(common.RequestIdKey),
		TenantID:              sessionCaptureTenantID,
		StartedAt:             state.startedAt,
		EndedAt:               time.Now(),
		UserID:                state.userID,
		Model:                 state.model,
		PromptTokens:          state.promptTokens,
		CompletionTokens:      state.completionTokens,
		Quota:                 state.quota,
		HTTPStatus:            c.Writer.Status(),
		RequestBody:           requestBody,
		ResponseBody:          append([]byte(nil), responseBody...),
		RequestBodyTruncated:  requestBodyTruncated,
		ResponseBodyTruncated: responseBodyTruncated,
	}
	state.mu.Unlock()

	if payload.SessionID == "" {
		payload.SessionID = uuid.NewString()
	}
	if payload.HTTPStatus == 0 {
		payload.HTTPStatus = http.StatusOK
	}
	payload.Status = "error"
	if payload.HTTPStatus < http.StatusBadRequest {
		payload.Status = "ok"
	}

	transcript, complete, err := buildSessionStoreTranscript(payload)
	if err != nil {
		common.SysError("session capture failed to build transcript request_id=" + payload.SessionID + ": " + err.Error())
		return
	}
	if payload.Status == "ok" && !complete {
		payload.Status = "partial"
	}

	sessionCapturePublish(sessioncapture.Meta{
		SessionID: payload.SessionID,
		TenantID:  payload.TenantID,
		UserID:    strconv.Itoa(payload.UserID),
		Model:     payload.Model,
		Status:    payload.Status,
		StartedAt: payload.StartedAt,
		EndedAt:   payload.EndedAt,
		TokensIn:  int64(payload.PromptTokens),
		TokensOut: int64(payload.CompletionTokens),
		CostUSD:   sessionStoreCostUSD(payload.Quota),
	}, transcript)
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

func buildSessionStoreTranscript(payload sessionStorePayload) ([]byte, bool, error) {
	if payload.RequestBodyTruncated || payload.ResponseBodyTruncated {
		return nil, false, errSessionCaptureTruncated
	}

	var requestSummary claudeRequestSummary
	if err := common.Unmarshal(payload.RequestBody, &requestSummary); err != nil {
		return nil, false, fmt.Errorf("invalid Claude request JSON: %w", err)
	}
	model := requestSummary.Model
	if model == "" {
		model = payload.Model
	}

	responseBody, response, complete, err := canonicalClaudeResponse(payload.ResponseBody)
	if err != nil {
		return nil, false, err
	}
	usage := claudeUsageFromResponse(response)
	if usage.InputTokens == 0 {
		usage.InputTokens = int64(payload.PromptTokens)
	}
	if usage.OutputTokens == 0 {
		usage.OutputTokens = int64(payload.CompletionTokens)
	}
	usage.TotalTokens = usage.InputTokens + usage.OutputTokens

	turns := 0
	for _, message := range requestSummary.Messages {
		if message.Role == "user" {
			turns++
		}
	}
	endedAt := payload.EndedAt
	if endedAt.IsZero() {
		endedAt = time.Now()
	}
	startedAt := payload.StartedAt
	if startedAt.IsZero() {
		startedAt = endedAt
	}
	record := claudeSessionRecord{
		SessionID:          payload.SessionID,
		CapturedAt:         endedAt.UTC().Format(time.RFC3339Nano),
		Provider:           "anthropic",
		Model:              model,
		Request:            append(json.RawMessage(nil), payload.RequestBody...),
		Response:           append(json.RawMessage(nil), responseBody...),
		TokenLength:        usage,
		SignaturePreserved: true,
		Meta: claudeSessionMeta{
			TenantID:  payload.TenantID,
			CostUSD:   sessionStoreCostUSD(payload.Quota),
			StartedAt: startedAt.UTC().Format(time.RFC3339Nano),
			EndedAt:   endedAt.UTC().Format(time.RFC3339Nano),
			Turns:     turns,
		},
	}
	transcript, err := common.Marshal(record)
	if err != nil {
		return nil, false, err
	}
	return append(transcript, '\n'), complete, nil
}

func canonicalClaudeResponse(body []byte) ([]byte, map[string]any, bool, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, nil, false, errors.New("empty Claude response")
	}
	if trimmed[0] == '{' {
		var response map[string]any
		if err := common.Unmarshal(trimmed, &response); err != nil {
			return nil, nil, false, fmt.Errorf("invalid Claude response JSON: %w", err)
		}
		return append([]byte(nil), trimmed...), response, true, nil
	}
	response, complete, err := assembleClaudeStream(trimmed)
	if err != nil {
		return nil, nil, false, err
	}
	data, err := common.Marshal(response)
	if err != nil {
		return nil, nil, false, err
	}
	return data, response, complete, nil
}

func assembleClaudeStream(body []byte) (map[string]any, bool, error) {
	var message map[string]any
	blocks := make(map[int]map[string]any)
	partialJSON := make(map[int]string)
	sawMessageStop := false

	for _, eventData := range sessionStoreSSEData(body) {
		if eventData == "" || eventData == "[DONE]" {
			continue
		}
		var event map[string]any
		if err := common.Unmarshal([]byte(eventData), &event); err != nil {
			return nil, false, fmt.Errorf("invalid Claude SSE event: %w", err)
		}
		eventType, _ := event["type"].(string)
		switch eventType {
		case "message_start":
			message, _ = event["message"].(map[string]any)
			if message == nil {
				return nil, false, errors.New("Claude message_start is missing message")
			}
		case "content_block_start":
			index, err := sessionStoreEventIndex(event)
			if err != nil {
				return nil, false, err
			}
			block, _ := event["content_block"].(map[string]any)
			if block == nil {
				return nil, false, errors.New("Claude content_block_start is missing content_block")
			}
			blocks[index] = block
		case "content_block_delta":
			index, err := sessionStoreEventIndex(event)
			if err != nil {
				return nil, false, err
			}
			block := blocks[index]
			if block == nil {
				block = make(map[string]any)
				blocks[index] = block
			}
			delta, _ := event["delta"].(map[string]any)
			mergeClaudeContentDelta(block, delta, partialJSON, index)
		case "content_block_stop":
			index, err := sessionStoreEventIndex(event)
			if err != nil {
				return nil, false, err
			}
			if err := finalizeClaudeToolInput(blocks[index], partialJSON[index]); err != nil {
				return nil, false, err
			}
		case "message_delta":
			if message == nil {
				message = make(map[string]any)
			}
			if delta, ok := event["delta"].(map[string]any); ok {
				for key, value := range delta {
					message[key] = value
				}
			}
			mergeClaudeUsage(message, event["usage"])
		case "message_stop":
			sawMessageStop = true
		case "error":
			return event, true, nil
		}
	}
	if message == nil {
		return nil, false, errors.New("Claude stream did not contain message_start")
	}
	indices := make([]int, 0, len(blocks))
	for index := range blocks {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	content := make([]any, 0, len(indices))
	for _, index := range indices {
		if err := finalizeClaudeToolInput(blocks[index], partialJSON[index]); err != nil {
			return nil, false, err
		}
		content = append(content, blocks[index])
	}
	message["content"] = content
	return message, sawMessageStop, nil
}

func sessionStoreSSEData(body []byte) []string {
	lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	result := make([]string, 0)
	dataLines := make([]string, 0, 1)
	flush := func() {
		if len(dataLines) > 0 {
			result = append(result, strings.Join(dataLines, "\n"))
			dataLines = dataLines[:0]
		}
	}
	for _, line := range lines {
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	flush()
	return result
}

func sessionStoreEventIndex(event map[string]any) (int, error) {
	switch value := event["index"].(type) {
	case float64:
		return int(value), nil
	case int:
		return value, nil
	default:
		return 0, errors.New("Claude content event is missing index")
	}
}

func mergeClaudeContentDelta(block map[string]any, delta map[string]any, partialJSON map[int]string, index int) {
	for key, value := range delta {
		if key == "type" {
			continue
		}
		if key == "partial_json" {
			if chunk, ok := value.(string); ok {
				partialJSON[index] += chunk
			}
			continue
		}
		if chunk, ok := value.(string); ok {
			if existing, ok := block[key].(string); ok {
				block[key] = existing + chunk
			} else {
				block[key] = chunk
			}
			continue
		}
		block[key] = value
	}
}

func finalizeClaudeToolInput(block map[string]any, partialJSON string) error {
	if block == nil || partialJSON == "" {
		return nil
	}
	var input any
	if err := common.UnmarshalJsonStr(partialJSON, &input); err != nil {
		return fmt.Errorf("invalid Claude tool input delta: %w", err)
	}
	block["input"] = input
	return nil
}

func mergeClaudeUsage(message map[string]any, deltaUsage any) {
	if deltaUsage == nil {
		return
	}
	usage, _ := message["usage"].(map[string]any)
	if usage == nil {
		usage = make(map[string]any)
		message["usage"] = usage
	}
	if delta, ok := deltaUsage.(map[string]any); ok {
		for key, value := range delta {
			usage[key] = value
		}
	}
}

func claudeUsageFromResponse(response map[string]any) claudeTokenInfo {
	var result claudeTokenInfo
	if response == nil {
		return result
	}
	usage, _ := response["usage"].(map[string]any)
	result.InputTokens = sessionStoreJSONInt64(usage["input_tokens"])
	result.OutputTokens = sessionStoreJSONInt64(usage["output_tokens"])
	result.CacheCreation = sessionStoreJSONInt64(usage["cache_creation_input_tokens"])
	result.CacheRead = sessionStoreJSONInt64(usage["cache_read_input_tokens"])
	return result
}

func sessionStoreJSONInt64(value any) int64 {
	switch value := value.(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	case json.Number:
		parsed, _ := value.Int64()
		return parsed
	default:
		return 0
	}
}

func sessionStoreCostUSD(quota int) float64 {
	if common.QuotaPerUnit <= 0 {
		return 0
	}
	return float64(quota) / common.QuotaPerUnit
}
