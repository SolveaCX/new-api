package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// phoneVerificationReminderKind names the response shape the gateway can
// synthesize for a request. Only text-generation shapes are supported; every
// other request is relayed untouched.
type phoneVerificationReminderKind int

const (
	phoneReminderKindOpenAIChat phoneVerificationReminderKind = iota + 1
	phoneReminderKindClaude
	phoneReminderKindResponses
	phoneReminderKindGemini
)

// phoneVerificationReminderKindFor decides whether the request can be answered
// with a synthesized reminder and, if so, in which shape. It returns false for
// unsupported entry points and for requests that ask for machine-readable
// output (JSON modes, forced tool calls), where a prose reply would break the
// caller's pipeline on the spot.
func phoneVerificationReminderKindFor(relayFormat types.RelayFormat, info *relaycommon.RelayInfo, path string, request dto.Request) (phoneVerificationReminderKind, bool) {
	if info == nil || request == nil {
		return 0, false
	}
	switch relayFormat {
	case types.RelayFormatOpenAI:
		req, ok := request.(*dto.GeneralOpenAIRequest)
		if !ok || info.RelayMode != relayconstant.RelayModeChatCompletions {
			return 0, false
		}
		if openAIChatWantsMachineReadableOutput(req) {
			return 0, false
		}
		return phoneReminderKindOpenAIChat, true
	case types.RelayFormatClaude:
		req, ok := request.(*dto.ClaudeRequest)
		if !ok || claudeWantsMachineReadableOutput(req) {
			return 0, false
		}
		return phoneReminderKindClaude, true
	case types.RelayFormatOpenAIResponses:
		req, ok := request.(*dto.OpenAIResponsesRequest)
		if !ok || responsesWantsMachineReadableOutput(req) {
			return 0, false
		}
		return phoneReminderKindResponses, true
	case types.RelayFormatGemini:
		req, ok := request.(*dto.GeminiChatRequest)
		if !ok || !isGeminiGenerateContentPath(path) || len(req.Requests) > 0 {
			return 0, false
		}
		if geminiWantsMachineReadableOutput(req) {
			return 0, false
		}
		return phoneReminderKindGemini, true
	}
	return 0, false
}

// isGeminiGenerateContentPath matches /v1beta/models/{m}:generateContent and
// :streamGenerateContent (also under /v1/models/). Embeddings, countTokens
// and batch endpoints share the route but are not text generation.
func isGeminiGenerateContentPath(path string) bool {
	if !strings.HasPrefix(path, "/v1beta/models/") && !strings.HasPrefix(path, "/v1/models/") {
		return false
	}
	actionIndex := strings.LastIndex(path, ":")
	if actionIndex < 0 {
		return false
	}
	action := path[actionIndex+1:]
	return action == "generateContent" || action == "streamGenerateContent"
}

func toolChoiceForcesCall(toolChoice any) bool {
	switch v := toolChoice.(type) {
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "required")
	case map[string]any:
		return len(v) > 0
	}
	return false
}

func openAIChatWantsMachineReadableOutput(req *dto.GeneralOpenAIRequest) bool {
	if req.ResponseFormat != nil {
		switch strings.ToLower(strings.TrimSpace(req.ResponseFormat.Type)) {
		case "json_object", "json_schema":
			return true
		}
	}
	return toolChoiceForcesCall(req.ToolChoice)
}

func claudeWantsMachineReadableOutput(req *dto.ClaudeRequest) bool {
	if len(req.OutputFormat) > 0 && string(req.OutputFormat) != "null" {
		return true
	}
	choice, ok := req.ToolChoice.(map[string]any)
	if !ok {
		return false
	}
	switch strings.ToLower(common.Interface2String(choice["type"])) {
	case "any", "tool":
		return true
	}
	return false
}

func responsesWantsMachineReadableOutput(req *dto.OpenAIResponsesRequest) bool {
	if len(req.Text) > 0 {
		var text struct {
			Format struct {
				Type string `json:"type"`
			} `json:"format"`
		}
		if err := common.Unmarshal(req.Text, &text); err == nil {
			switch strings.ToLower(strings.TrimSpace(text.Format.Type)) {
			case "json_object", "json_schema":
				return true
			}
		}
	}
	if len(req.ToolChoice) > 0 {
		var choice any
		if err := common.Unmarshal(req.ToolChoice, &choice); err == nil && toolChoiceForcesCall(choice) {
			return true
		}
	}
	return false
}

func geminiWantsMachineReadableOutput(req *dto.GeminiChatRequest) bool {
	cfg := req.GenerationConfig
	if strings.EqualFold(strings.TrimSpace(cfg.ResponseMimeType), "application/json") {
		return true
	}
	if cfg.ResponseSchema != nil {
		return true
	}
	if len(cfg.ResponseJsonSchema) > 0 && string(cfg.ResponseJsonSchema) != "null" {
		return true
	}
	if req.ToolConfig != nil && req.ToolConfig.FunctionCallingConfig != nil &&
		strings.EqualFold(string(req.ToolConfig.FunctionCallingConfig.Mode), "ANY") {
		return true
	}
	return false
}

// phoneVerificationNoticeHeader lets integrations recognise a synthesized
// reminder deterministically instead of pattern-matching the text.
const (
	phoneVerificationNoticeHeader = "X-Flatkey-Notice"
	phoneVerificationNoticeValue  = "phone_verification_reminder"
)

// writePhoneVerificationReminder writes a complete, successful response in the
// requested shape whose only content is the reminder text. Usage is zero and
// the finish reason is a normal end of turn.
func writePhoneVerificationReminder(c *gin.Context, kind phoneVerificationReminderKind, stream bool, modelName string, text string) error {
	c.Header(phoneVerificationNoticeHeader, phoneVerificationNoticeValue)
	now := common.GetTimestamp()
	switch kind {
	case phoneReminderKindOpenAIChat:
		return writePhoneReminderOpenAIChat(c, stream, modelName, text, now)
	case phoneReminderKindClaude:
		return writePhoneReminderClaude(c, stream, modelName, text)
	case phoneReminderKindResponses:
		return writePhoneReminderResponses(c, stream, modelName, text, now)
	case phoneReminderKindGemini:
		return writePhoneReminderGemini(c, stream, text)
	}
	return fmt.Errorf("unsupported phone verification reminder kind %d", kind)
}

func writePhoneReminderOpenAIChat(c *gin.Context, stream bool, modelName string, text string, now int64) error {
	id := "chatcmpl-" + common.GetRandomString(29)
	if !stream {
		c.JSON(http.StatusOK, dto.OpenAITextResponse{
			Id:      id,
			Model:   modelName,
			Object:  "chat.completion",
			Created: now,
			Choices: []dto.OpenAITextResponseChoice{{
				Index:        0,
				Message:      dto.Message{Role: "assistant", Content: text},
				FinishReason: constant.FinishReasonStop,
			}},
			Usage: dto.Usage{},
		})
		return nil
	}
	helper.SetEventStreamHeaders(c)
	content := dto.ChatCompletionsStreamResponse{
		Id:      id,
		Object:  "chat.completion.chunk",
		Created: now,
		Model:   modelName,
		Choices: []dto.ChatCompletionsStreamResponseChoice{{Index: 0}},
	}
	content.Choices[0].Delta.Role = "assistant"
	content.Choices[0].Delta.SetContentString(text)
	if err := helper.ObjectData(c, content); err != nil {
		return err
	}
	stop := helper.GenerateStopResponse(id, now, modelName, constant.FinishReasonStop)
	stop.Usage = &dto.Usage{}
	if err := helper.ObjectData(c, stop); err != nil {
		return err
	}
	helper.Done(c)
	return nil
}

func writePhoneReminderClaude(c *gin.Context, stream bool, modelName string, text string) error {
	id := "msg_" + common.GetRandomString(24)
	textBlock := dto.ClaudeMediaMessage{Type: "text"}
	textBlock.SetText(text)
	if !stream {
		c.JSON(http.StatusOK, dto.ClaudeResponse{
			Id:         id,
			Type:       "message",
			Role:       "assistant",
			Model:      modelName,
			Content:    []dto.ClaudeMediaMessage{textBlock},
			StopReason: "end_turn",
			Usage:      &dto.ClaudeUsage{},
		})
		return nil
	}
	helper.SetEventStreamHeaders(c)
	start := &dto.ClaudeMediaMessage{Id: id, Type: "message", Role: "assistant", Model: modelName, Usage: &dto.ClaudeUsage{}}
	start.SetContent(make([]any, 0))
	emptyText := dto.ClaudeMediaMessage{Type: "text"}
	emptyText.SetText("")
	delta := dto.ClaudeMediaMessage{Type: "text_delta"}
	delta.SetText(text)
	events := []dto.ClaudeResponse{
		{Type: "message_start", Message: start},
		{Type: "content_block_start", Index: common.GetPointer(0), ContentBlock: &emptyText},
		{Type: "content_block_delta", Index: common.GetPointer(0), Delta: &delta},
		{Type: "content_block_stop", Index: common.GetPointer(0)},
		{Type: "message_delta", Usage: &dto.ClaudeUsage{}, Delta: &dto.ClaudeMediaMessage{StopReason: common.GetPointer("end_turn")}},
		{Type: "message_stop"},
	}
	for _, event := range events {
		if err := helper.ClaudeData(c, event); err != nil {
			return err
		}
	}
	return nil
}

func phoneReminderResponsesBody(id string, itemID string, modelName string, now int64, status string, content []dto.ResponsesOutputContent) dto.OpenAIResponsesResponse {
	output := []dto.ResponsesOutput{}
	if content != nil {
		output = append(output, dto.ResponsesOutput{Type: "message", ID: itemID, Status: "completed", Role: "assistant", Content: content})
	}
	return dto.OpenAIResponsesResponse{
		ID:                 id,
		Object:             "response",
		CreatedAt:          int(now),
		Status:             json.RawMessage(`"` + status + `"`),
		Instructions:       json.RawMessage(`null`),
		Model:              modelName,
		Output:             output,
		PreviousResponseID: json.RawMessage(`null`),
		ToolChoice:         json.RawMessage(`"auto"`),
		Tools:              []map[string]any{},
		Truncation:         json.RawMessage(`"disabled"`),
		Usage:              &dto.Usage{},
		User:               json.RawMessage(`null`),
		Metadata:           json.RawMessage(`{}`),
	}
}

func writePhoneReminderResponses(c *gin.Context, stream bool, modelName string, text string, now int64) error {
	id := "resp_" + common.GetRandomString(24)
	itemID := "msg_" + common.GetRandomString(24)
	content := []dto.ResponsesOutputContent{{Type: "output_text", Text: text, Annotations: []interface{}{}}}
	if !stream {
		c.JSON(http.StatusOK, phoneReminderResponsesBody(id, itemID, modelName, now, "completed", content))
		return nil
	}
	helper.SetEventStreamHeaders(c)
	inProgress := phoneReminderResponsesBody(id, itemID, modelName, now, "in_progress", nil)
	completed := phoneReminderResponsesBody(id, itemID, modelName, now, "completed", content)
	emptyItem := map[string]any{"type": "message", "id": itemID, "status": "in_progress", "role": "assistant", "content": []any{}}
	doneItem := map[string]any{"type": "message", "id": itemID, "status": "completed", "role": "assistant", "content": content}
	emptyPart := map[string]any{"type": "output_text", "text": "", "annotations": []any{}}
	donePart := map[string]any{"type": "output_text", "text": text, "annotations": []any{}}
	events := []map[string]any{
		{"type": "response.created", "response": inProgress},
		{"type": "response.output_item.added", "output_index": 0, "item": emptyItem},
		{"type": "response.content_part.added", "item_id": itemID, "output_index": 0, "content_index": 0, "part": emptyPart},
		{"type": "response.output_text.delta", "item_id": itemID, "output_index": 0, "content_index": 0, "delta": text},
		{"type": "response.output_text.done", "item_id": itemID, "output_index": 0, "content_index": 0, "text": text},
		{"type": "response.content_part.done", "item_id": itemID, "output_index": 0, "content_index": 0, "part": donePart},
		{"type": "response.output_item.done", "output_index": 0, "item": doneItem},
		{"type": "response.completed", "response": completed},
	}
	for index, event := range events {
		event["sequence_number"] = index
		data, err := common.Marshal(event)
		if err != nil {
			return err
		}
		helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: event["type"].(string)}, string(data))
	}
	return nil
}

func writePhoneReminderGemini(c *gin.Context, stream bool, text string) error {
	body := dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{{
			Content:       dto.GeminiChatContent{Role: "model", Parts: []dto.GeminiPart{{Text: text}}},
			FinishReason:  common.GetPointer("STOP"),
			Index:         0,
			SafetyRatings: []dto.GeminiChatSafetyRating{},
		}},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokensDetails:        []dto.GeminiPromptTokensDetails{},
			ToolUsePromptTokensDetails: []dto.GeminiPromptTokensDetails{},
			CandidatesTokensDetails:    []dto.GeminiPromptTokensDetails{},
		},
	}
	if !stream {
		c.JSON(http.StatusOK, body)
		return nil
	}
	helper.SetEventStreamHeaders(c)
	return helper.ObjectData(c, body)
}
