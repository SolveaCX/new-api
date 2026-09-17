package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
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
