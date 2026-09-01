package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	playgroundRecordBodyLimit = 16 << 20
	playgroundJSONMaxDepth    = 32
)

type savePlaygroundRecordRequest struct {
	RecordID          string          `json:"record_id"`
	ConversationID    string          `json:"conversation_id"`
	UserMessage       json.RawMessage `json:"user_message"`
	RequestMessages   json.RawMessage `json:"request_messages"`
	AssistantMessage  json.RawMessage `json:"assistant_message"`
	ReasoningContent  string          `json:"reasoning_content"`
	InputText         string          `json:"input_text"`
	OutputText        string          `json:"output_text"`
	ModelName         string          `json:"model_name"`
	GroupName         string          `json:"group_name"`
	Parameters        json.RawMessage `json:"parameters"`
	Status            string          `json:"status"`
	ErrorCode         string          `json:"error_code"`
	ErrorMessage      string          `json:"error_message"`
	RelayRequestID    string          `json:"relay_request_id"`
	PromptTokens      int             `json:"prompt_tokens"`
	CompletionTokens  int             `json:"completion_tokens"`
	TotalTokens       int             `json:"total_tokens"`
	LatencyMS         int64           `json:"latency_ms"`
	MessagesSnapshot  json.RawMessage `json:"messages_snapshot"`
	ClientCompletedAt int64           `json:"client_completed_at"`
}

type clearPlaygroundRecordRequest struct {
	RecordID          string `json:"record_id"`
	ConversationID    string `json:"conversation_id"`
	ClientCompletedAt int64  `json:"client_completed_at"`
}

func SavePlaygroundRecord(c *gin.Context) {
	var request savePlaygroundRecordRequest
	if err := decodePlaygroundRecordRequest(c, &request); err != nil {
		playgroundRecordBadRequest(c, err)
		return
	}
	if err := validateSavePlaygroundRecordRequest(&request); err != nil {
		playgroundRecordBadRequest(c, err)
		return
	}
	assetReferences, err := extractPlaygroundAssetReferences(&request)
	if err != nil {
		playgroundRecordBadRequest(c, err)
		return
	}
	if err := model.ValidatePlaygroundAssetReferences(c.GetInt("id"), assetReferences); err != nil {
		playgroundRecordBadRequest(c, err)
		return
	}
	if err := sanitizePlaygroundRecordMedia(&request); err != nil {
		playgroundRecordBadRequest(c, err)
		return
	}

	record := &model.PlaygroundRecord{
		UserID:            c.GetInt("id"),
		RecordID:          request.RecordID,
		RecordType:        model.PlaygroundRecordTypeTurn,
		ConversationID:    request.ConversationID,
		UserMessage:       model.PlaygroundLargeText(request.UserMessage),
		RequestMessages:   model.PlaygroundLargeText(request.RequestMessages),
		AssistantMessage:  model.PlaygroundLargeText(request.AssistantMessage),
		ReasoningContent:  model.PlaygroundLargeText(request.ReasoningContent),
		InputText:         model.PlaygroundLargeText(request.InputText),
		OutputText:        model.PlaygroundLargeText(request.OutputText),
		ModelName:         request.ModelName,
		GroupName:         request.GroupName,
		Parameters:        model.PlaygroundLargeText(request.Parameters),
		Status:            request.Status,
		ErrorCode:         request.ErrorCode,
		ErrorMessage:      model.PlaygroundLargeText(request.ErrorMessage),
		RelayRequestID:    request.RelayRequestID,
		PromptTokens:      request.PromptTokens,
		CompletionTokens:  request.CompletionTokens,
		TotalTokens:       request.TotalTokens,
		LatencyMS:         request.LatencyMS,
		MessagesSnapshot:  model.PlaygroundLargeText(request.MessagesSnapshot),
		ClientCompletedAt: request.ClientCompletedAt,
		AssetReferences:   assetReferences,
	}
	if err := model.SavePlaygroundRecord(record); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func GetCurrentPlaygroundRecord(c *gin.Context) {
	record, err := model.GetCurrentPlaygroundRecord(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if record == nil {
		common.ApiSuccess(c, nil)
		return
	}

	common.ApiSuccess(c, gin.H{
		"conversation_id": record.ConversationID,
		"messages":        json.RawMessage(record.MessagesSnapshot),
	})
}

type renamePlaygroundConversationRequest struct {
	Name string `json:"name"`
}

type deletePlaygroundConversationsRequest struct {
	ConversationIDs []string `json:"conversation_ids"`
}

func ListPlaygroundConversations(c *gin.Context) {
	conversations, err := model.ListPlaygroundConversations(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, conversations)
}

func GetPlaygroundConversation(c *gin.Context) {
	conversationID := c.Param("conversation_id")
	if _, err := uuid.Parse(conversationID); err != nil {
		playgroundRecordBadRequest(c, errors.New("conversation_id must be a UUID"))
		return
	}
	record, err := model.GetPlaygroundConversation(c.GetInt("id"), conversationID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if record == nil {
		common.ApiSuccess(c, nil)
		return
	}
	common.ApiSuccess(c, gin.H{
		"conversation_id": record.ConversationID,
		"messages":        json.RawMessage(record.MessagesSnapshot),
	})
}

func RenamePlaygroundConversation(c *gin.Context) {
	conversationID := c.Param("conversation_id")
	if _, err := uuid.Parse(conversationID); err != nil {
		playgroundRecordBadRequest(c, errors.New("conversation_id must be a UUID"))
		return
	}
	var request renamePlaygroundConversationRequest
	if err := decodePlaygroundRecordRequest(c, &request); err != nil {
		playgroundRecordBadRequest(c, err)
		return
	}
	if strings.TrimSpace(request.Name) == "" {
		playgroundRecordBadRequest(c, errors.New("name must not be empty"))
		return
	}
	if len([]rune(request.Name)) > 120 {
		playgroundRecordBadRequest(c, errors.New("name must not exceed 120 characters"))
		return
	}
	if err := model.RenamePlaygroundConversation(c.GetInt("id"), conversationID, request.Name); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func DeletePlaygroundConversations(c *gin.Context) {
	var request deletePlaygroundConversationsRequest
	if err := decodePlaygroundRecordRequest(c, &request); err != nil {
		playgroundRecordBadRequest(c, err)
		return
	}
	if len(request.ConversationIDs) == 0 || len(request.ConversationIDs) > 100 {
		playgroundRecordBadRequest(c, errors.New("conversation_ids must contain 1 to 100 items"))
		return
	}
	seen := make(map[string]struct{}, len(request.ConversationIDs))
	for _, conversationID := range request.ConversationIDs {
		if _, err := uuid.Parse(conversationID); err != nil {
			playgroundRecordBadRequest(c, errors.New("conversation_id must be a UUID"))
			return
		}
		seen[conversationID] = struct{}{}
	}
	conversationIDs := make([]string, 0, len(seen))
	for conversationID := range seen {
		conversationIDs = append(conversationIDs, conversationID)
	}
	if err := model.DeletePlaygroundConversations(c.GetInt("id"), conversationIDs); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// ExportPlaygroundRecords downloads durable Playground records for
// administrators. Without a user_id query parameter it includes all users;
// otherwise it is restricted to the requested positive user ID. Excel is the
// default format; format=json preserves the legacy JSON download.
func ExportPlaygroundRecords(c *gin.Context) {
	rawUserID, hasUserID := c.GetQuery("user_id")
	userID, err := parsePlaygroundExportUserID(rawUserID, hasUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": common.TranslateMessage(c, i18n.MsgLogInvalidUserId),
		})
		return
	}

	rawFormat, hasFormat := c.GetQuery("format")
	format, err := parsePlaygroundExportFormat(rawFormat, hasFormat)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": common.TranslateMessage(c, i18n.MsgInvalidParams),
		})
		return
	}

	records, err := model.ListPlaygroundRecordsForExport(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if format == "json" {
		exportPlaygroundRecordsJSON(c, records, userID)
		return
	}

	data, err := buildPlaygroundRecordsWorkbook(records)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filename := playgroundExportFilename(userID, "xlsx")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, playgroundExportMimeType, data)
}

func parsePlaygroundExportUserID(raw string, present bool) (*int, error) {
	if !present {
		return nil, nil
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("invalid user_id")
	}
	userID, err := strconv.Atoi(raw)
	if err != nil || userID <= 0 {
		return nil, errors.New("invalid user_id")
	}
	return &userID, nil
}

func ClearPlaygroundRecord(c *gin.Context) {
	var request clearPlaygroundRecordRequest
	if err := decodePlaygroundRecordRequest(c, &request); err != nil {
		playgroundRecordBadRequest(c, err)
		return
	}
	if err := validatePlaygroundRecordIdentity(
		request.RecordID,
		request.ConversationID,
		request.ClientCompletedAt,
	); err != nil {
		playgroundRecordBadRequest(c, err)
		return
	}

	removedAssetIDs, err := model.ClearPlaygroundConversationWithAssets(
		c.GetInt("id"),
		request.RecordID,
		request.ConversationID,
		request.ClientCompletedAt,
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	schedulePlaygroundAttachmentCleanup(c.GetInt("id"), removedAssetIDs)
	common.ApiSuccess(c, nil)
}

func decodePlaygroundRecordRequest(c *gin.Context, target any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, playgroundRecordBodyLimit)
	return common.DecodeJson(c.Request.Body, target)
}

func validateSavePlaygroundRecordRequest(request *savePlaygroundRecordRequest) error {
	if err := validatePlaygroundRecordIdentity(
		request.RecordID,
		request.ConversationID,
		request.ClientCompletedAt,
	); err != nil {
		return err
	}

	switch request.Status {
	case model.PlaygroundStatusComplete, model.PlaygroundStatusError, model.PlaygroundStatusStopped:
	default:
		return fmt.Errorf("invalid status %q", request.Status)
	}
	if request.PromptTokens < 0 || request.CompletionTokens < 0 || request.TotalTokens < 0 || request.LatencyMS < 0 {
		return errors.New("token counts and latency must not be negative")
	}

	fields := []struct {
		name         string
		value        json.RawMessage
		expectedType string
	}{
		{name: "user_message", value: request.UserMessage, expectedType: "object"},
		{name: "request_messages", value: request.RequestMessages, expectedType: "array"},
		{name: "assistant_message", value: request.AssistantMessage, expectedType: "object"},
		{name: "parameters", value: request.Parameters, expectedType: "object"},
		{name: "messages_snapshot", value: request.MessagesSnapshot, expectedType: "array"},
	}
	for _, field := range fields {
		if err := validatePlaygroundJSON(field.name, field.value, field.expectedType); err != nil {
			return err
		}
	}
	return nil
}

func validatePlaygroundRecordIdentity(recordID, conversationID string, clientCompletedAt int64) error {
	if _, err := uuid.Parse(recordID); err != nil {
		return errors.New("record_id must be a UUID")
	}
	if _, err := uuid.Parse(conversationID); err != nil {
		return errors.New("conversation_id must be a UUID")
	}
	if clientCompletedAt <= 0 {
		return errors.New("client_completed_at must be positive")
	}
	return nil
}

func validatePlaygroundJSON(name string, raw json.RawMessage, expectedType string) error {
	if common.GetJsonType(raw) != expectedType {
		return fmt.Errorf("%s must be a JSON %s", name, expectedType)
	}

	var value any
	if err := common.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("%s is invalid JSON: %w", name, err)
	}
	if err := inspectPlaygroundJSON(value, 1); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func inspectPlaygroundJSON(value any, depth int) error {
	switch typed := value.(type) {
	case map[string]any:
		if depth > playgroundJSONMaxDepth {
			return fmt.Errorf("JSON depth exceeds %d", playgroundJSONMaxDepth)
		}
		for key, child := range typed {
			if strings.EqualFold(key, "b64_json") {
				return errors.New("embedded base64 media is not allowed")
			}
			if err := inspectPlaygroundJSON(child, depth+1); err != nil {
				return err
			}
		}
	case []any:
		if depth > playgroundJSONMaxDepth {
			return fmt.Errorf("JSON depth exceeds %d", playgroundJSONMaxDepth)
		}
		for _, child := range typed {
			if err := inspectPlaygroundJSON(child, depth+1); err != nil {
				return err
			}
		}
	case string:
		if isBase64DataURL(typed) {
			return errors.New("embedded base64 media is not allowed")
		}
	}
	return nil
}

func isBase64DataURL(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if !strings.HasPrefix(lower, "data:") {
		return false
	}
	header := lower
	if comma := strings.IndexByte(lower, ','); comma >= 0 {
		header = lower[:comma]
	}
	return strings.Contains(header, ";base64")
}

type playgroundAssetJSONContext struct {
	assetType string
	assetID   string
}

func playgroundAssetTypeFromValue(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "image":
		return model.AssetTypeImage
	case "video":
		return model.AssetTypeVideo
	case "audio":
		return model.AssetTypeAudio
	case "document", "pdf", "file":
		return model.AssetTypeDocument
	default:
		return ""
	}
}

// extractPlaygroundAssetReferences accepts both camelCase (browser message
// shape) and snake_case (API shape), while deriving an expected asset type from
// attachment kind, generated-media type, or multimodal content parts.
func extractPlaygroundAssetReferences(request *savePlaygroundRecordRequest) ([]model.PlaygroundAssetReference, error) {
	if request == nil {
		return nil, nil
	}
	refs := make([]model.PlaygroundAssetReference, 0)
	for _, raw := range []json.RawMessage{
		request.UserMessage,
		request.RequestMessages,
		request.AssistantMessage,
		request.MessagesSnapshot,
	} {
		var value any
		if err := common.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		if err := walkPlaygroundAssetReferences(value, playgroundAssetJSONContext{}, &refs); err != nil {
			return nil, err
		}
	}
	return refs, nil
}

func walkPlaygroundAssetReferences(value any, context playgroundAssetJSONContext, refs *[]model.PlaygroundAssetReference) error {
	switch typed := value.(type) {
	case []any:
		for _, child := range typed {
			if err := walkPlaygroundAssetReferences(child, context, refs); err != nil {
				return err
			}
		}
	case map[string]any:
		local := context
		if kind, ok := stringValueForKey(typed, "kind"); ok {
			if assetType := playgroundAssetTypeFromValue(kind); assetType != "" {
				local.assetType = assetType
			}
		}
		// Generated media uses `type` while input attachments use `kind`.
		if mediaType, ok := stringValueForKey(typed, "type"); ok {
			if assetType := playgroundAssetTypeFromValue(mediaType); assetType != "" {
				local.assetType = assetType
			}
		}
		if assetID, ok := playgroundAssetIDFromMap(typed); ok {
			if assetID == "" {
				return model.ErrPlaygroundAssetInvalidID
			}
			local.assetID = assetID
			*refs = append(*refs, model.PlaygroundAssetReference{AssetID: assetID, AssetType: local.assetType})
		}
		for key, child := range typed {
			childContext := local
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "image_url":
				childContext.assetType = model.AssetTypeImage
			case "video_url":
				childContext.assetType = model.AssetTypeVideo
			case "audio_url", "input_audio":
				childContext.assetType = model.AssetTypeAudio
			case "file", "input_file":
				childContext.assetType = model.AssetTypeDocument
			}
			if keyLower := strings.ToLower(strings.TrimSpace(key)); keyLower == "url" || keyLower == "file_url" {
				if rawURL, ok := child.(string); ok && strings.HasPrefix(strings.TrimSpace(rawURL), "asset://") {
					assetID, err := parsePlaygroundAssetURI(rawURL)
					if err != nil {
						return err
					}
					*refs = append(*refs, model.PlaygroundAssetReference{AssetID: assetID, AssetType: local.assetType})
				}
			}
			if err := walkPlaygroundAssetReferences(child, childContext, refs); err != nil {
				return err
			}
		}
	}
	return nil
}

func stringValueForKey(values map[string]any, wanted string) (string, bool) {
	for key, value := range values {
		if strings.EqualFold(strings.TrimSpace(key), wanted) {
			result, ok := value.(string)
			return result, ok
		}
	}
	return "", false
}

func playgroundAssetIDFromMap(values map[string]any) (string, bool) {
	for key, value := range values {
		normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "_", ""))
		if normalized != "assetid" {
			continue
		}
		assetID, ok := value.(string)
		if !ok {
			return "", true
		}
		return strings.TrimSpace(assetID), true
	}
	return "", false
}

func parsePlaygroundAssetURI(value string) (string, error) {
	assetID := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "asset://"))
	if assetID == "" || strings.ContainsAny(assetID, "/?#") || !strings.HasPrefix(assetID, "ast_") {
		return "", model.ErrPlaygroundAssetInvalidID
	}
	return assetID, nil
}

func sanitizePlaygroundRecordMedia(request *savePlaygroundRecordRequest) error {
	if request == nil {
		return nil
	}
	urlToAsset := make(map[string]string)
	for _, raw := range []json.RawMessage{request.UserMessage, request.RequestMessages, request.AssistantMessage, request.MessagesSnapshot} {
		var value any
		if err := common.Unmarshal(raw, &value); err != nil {
			return err
		}
		collectPlaygroundAssetURLs(value, urlToAsset)
	}
	sanitize := func(raw json.RawMessage) (json.RawMessage, error) {
		var value any
		if err := common.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		sanitizePlaygroundValue(value, urlToAsset, playgroundAssetJSONContext{})
		data, err := common.Marshal(value)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(data), nil
	}
	var err error
	if request.UserMessage, err = sanitize(request.UserMessage); err != nil {
		return err
	}
	if request.RequestMessages, err = sanitize(request.RequestMessages); err != nil {
		return err
	}
	if request.AssistantMessage, err = sanitize(request.AssistantMessage); err != nil {
		return err
	}
	if request.MessagesSnapshot, err = sanitize(request.MessagesSnapshot); err != nil {
		return err
	}
	return nil
}

func collectPlaygroundAssetURLs(value any, urls map[string]string) {
	switch typed := value.(type) {
	case []any:
		for _, child := range typed {
			collectPlaygroundAssetURLs(child, urls)
		}
	case map[string]any:
		assetID, hasAssetID := playgroundAssetIDFromMap(typed)
		if hasAssetID {
			if assetID != "" {
				for _, urlKey := range []string{"url", "file_url"} {
					if rawURL, ok := stringValueForKey(typed, urlKey); ok && rawURL != "" {
						urls[strings.TrimSpace(rawURL)] = assetID
					}
				}
			}
		}
		for _, child := range typed {
			collectPlaygroundAssetURLs(child, urls)
		}
	}
}

func sanitizePlaygroundValue(value any, urls map[string]string, context playgroundAssetJSONContext) {
	switch typed := value.(type) {
	case []any:
		for _, child := range typed {
			sanitizePlaygroundValue(child, urls, context)
		}
	case map[string]any:
		local := context
		if kind, ok := stringValueForKey(typed, "kind"); ok {
			if assetType := playgroundAssetTypeFromValue(kind); assetType != "" {
				local.assetType = assetType
			}
		}
		if mediaType, ok := stringValueForKey(typed, "type"); ok {
			if assetType := playgroundAssetTypeFromValue(mediaType); assetType != "" {
				local.assetType = assetType
			}
		}
		if assetID, ok := playgroundAssetIDFromMap(typed); ok && assetID != "" {
			local.assetID = assetID
		}
		for key, child := range typed {
			keyLower := strings.ToLower(strings.TrimSpace(key))
			if keyLower == "url" || keyLower == "file_url" {
				if rawURL, ok := child.(string); ok {
					if local.assetID != "" && local.assetType != "" {
						delete(typed, key)
						continue
					}
					if assetID := urls[strings.TrimSpace(rawURL)]; assetID != "" {
						typed[key] = "asset://" + assetID
						continue
					}
				}
			}
			childContext := local
			if keyLower == "image_url" {
				childContext.assetType = model.AssetTypeImage
			} else if keyLower == "video_url" {
				childContext.assetType = model.AssetTypeVideo
			} else if keyLower == "audio_url" || keyLower == "input_audio" {
				childContext.assetType = model.AssetTypeAudio
			} else if keyLower == "file" || keyLower == "input_file" {
				childContext.assetType = model.AssetTypeDocument
			}
			sanitizePlaygroundValue(child, urls, childContext)
		}
	}
}

func playgroundRecordBadRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{
		"success": false,
		"message": common.TranslateMessage(c, "distributor.invalid_playground_request", map[string]any{
			"Error": err.Error(),
		}),
	})
}
