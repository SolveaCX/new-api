package auto_model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type Protocol string

const (
	ProtocolChatCompletions Protocol = "chat_completions"
	ProtocolResponses       Protocol = "responses"
	ProtocolMessages        Protocol = "messages"
)

var ErrUnsupportedRequest = errors.New("request is not supported by auto model")

type textPart struct {
	role string
	text string
}

// ExtractText validates an Auto Model request and returns only the text needed
// by the classifier. Stateful, tool, and non-text forms are rejected before any
// classifier call can occur.
func ExtractText(protocol Protocol, raw []byte, maxChars int) (string, error) {
	if maxChars <= 0 {
		return "", fmt.Errorf("classification input limit must be positive")
	}
	var body map[string]any
	if err := common.Unmarshal(raw, &body); err != nil {
		return "", fmt.Errorf("invalid request body: %w", err)
	}

	var parts []textPart
	var err error
	switch protocol {
	case ProtocolChatCompletions:
		parts, err = extractChat(body)
	case ProtocolResponses:
		parts, err = extractResponses(body)
	case ProtocolMessages:
		parts, err = extractMessages(body)
	default:
		return "", unsupported("unknown protocol")
	}
	if err != nil {
		return "", err
	}
	return formatAndTruncate(parts, maxChars)
}

func extractChat(body map[string]any) ([]textPart, error) {
	if hasValue(body, "tools") || hasValue(body, "functions") || hasValue(body, "tool_choice") || hasValue(body, "function_call") {
		return nil, unsupported("tools and functions are not supported")
	}
	messages, ok := body["messages"].([]any)
	if !ok || len(messages) == 0 {
		return nil, unsupported("messages must be a non-empty array")
	}
	parts := make([]textPart, 0, len(messages))
	for _, item := range messages {
		message, ok := item.(map[string]any)
		if !ok || !hasOnlyKeys(message, "role", "content") {
			return nil, unsupported("message contains unsupported content")
		}
		role, ok := message["role"].(string)
		if !ok || !oneOf(role, "system", "developer", "user", "assistant") {
			return nil, unsupported("message role is not supported")
		}
		text, err := extractContent(message["content"], "text")
		if err != nil {
			return nil, err
		}
		parts = append(parts, textPart{role: role, text: text})
	}
	return parts, nil
}

func extractResponses(body map[string]any) ([]textPart, error) {
	for _, key := range []string{"tools", "previous_response_id", "conversation", "prompt"} {
		if hasValue(body, key) {
			return nil, unsupported(key + " is not supported")
		}
	}
	if background, ok := body["background"].(bool); ok && background {
		return nil, unsupported("background responses are not supported")
	}
	if textConfig, ok := body["text"].(map[string]any); ok && hasValue(textConfig, "format") {
		return nil, unsupported("structured output is not supported")
	}

	parts := make([]textPart, 0, 8)
	if instructions, exists := body["instructions"]; exists && instructions != nil {
		instructionText, ok := instructions.(string)
		if !ok {
			return nil, unsupported("instructions must be text")
		}
		parts = append(parts, textPart{role: "instructions", text: instructionText})
	}

	input, exists := body["input"]
	if !exists || input == nil {
		return nil, unsupported("input is required")
	}
	if inputText, ok := input.(string); ok {
		parts = append(parts, textPart{role: "user", text: inputText})
		return parts, nil
	}
	items, ok := input.([]any)
	if !ok || len(items) == 0 {
		return nil, unsupported("input must be text or simple messages")
	}
	for _, item := range items {
		message, ok := item.(map[string]any)
		if !ok || !hasOnlyKeys(message, "type", "role", "content") {
			return nil, unsupported("response input item must be a message")
		}
		if itemType, exists := message["type"]; exists && itemType != nil && itemType != "message" {
			return nil, unsupported("response input item type is not supported")
		}
		role, ok := message["role"].(string)
		if !ok || !oneOf(role, "system", "developer", "user", "assistant") {
			return nil, unsupported("response message role is not supported")
		}
		text, err := extractContent(message["content"], "input_text")
		if err != nil {
			return nil, err
		}
		parts = append(parts, textPart{role: role, text: text})
	}
	return parts, nil
}

func extractMessages(body map[string]any) ([]textPart, error) {
	if hasValue(body, "tools") || hasValue(body, "tool_choice") {
		return nil, unsupported("tools are not supported")
	}
	parts := make([]textPart, 0, 8)
	if system, exists := body["system"]; exists && system != nil {
		text, err := extractContent(system, "text")
		if err != nil {
			return nil, err
		}
		parts = append(parts, textPart{role: "system", text: text})
	}
	messages, ok := body["messages"].([]any)
	if !ok || len(messages) == 0 {
		return nil, unsupported("messages must be a non-empty array")
	}
	for _, item := range messages {
		message, ok := item.(map[string]any)
		if !ok || !hasOnlyKeys(message, "role", "content") {
			return nil, unsupported("message must be an object")
		}
		role, ok := message["role"].(string)
		if !ok || !oneOf(role, "user", "assistant") {
			return nil, unsupported("message role is not supported")
		}
		text, err := extractContent(message["content"], "text")
		if err != nil {
			return nil, err
		}
		parts = append(parts, textPart{role: role, text: text})
	}
	return parts, nil
}

func extractContent(value any, allowedBlockType string) (string, error) {
	if text, ok := value.(string); ok {
		return text, nil
	}
	blocks, ok := value.([]any)
	if !ok || len(blocks) == 0 {
		return "", unsupported("content must be text")
	}
	texts := make([]string, 0, len(blocks))
	for _, item := range blocks {
		block, ok := item.(map[string]any)
		if !ok || !hasOnlyKeys(block, "type", "text") || block["type"] != allowedBlockType {
			return "", unsupported("content block type is not supported")
		}
		text, ok := block["text"].(string)
		if !ok {
			return "", unsupported("content block text is invalid")
		}
		texts = append(texts, text)
	}
	return strings.Join(texts, "\n"), nil
}

func formatAndTruncate(parts []textPart, maxChars int) (string, error) {
	formatted := make([]string, 0, len(parts))
	for _, part := range parts {
		text := strings.TrimSpace(part.text)
		if text != "" {
			formatted = append(formatted, "["+part.role+"]\n"+text)
		}
	}
	if len(formatted) == 0 {
		return "", unsupported("request contains no text")
	}
	joined := strings.Join(formatted, "\n\n")
	runes := []rune(joined)
	if len(runes) <= maxChars {
		return joined, nil
	}
	// Preserve the initial instructions/system context and the most recent
	// conversation. The marker makes the omission explicit to the classifier.
	const marker = "\n\n[...truncated...]\n\n"
	markerRunes := []rune(marker)
	if maxChars <= len(markerRunes)+2 {
		return string(runes[len(runes)-maxChars:]), nil
	}
	remaining := maxChars - len(markerRunes)
	prefix := remaining / 2
	suffix := remaining - prefix
	return string(runes[:prefix]) + marker + string(runes[len(runes)-suffix:]), nil
}

func hasValue(object map[string]any, key string) bool {
	value, exists := object[key]
	if !exists || value == nil {
		return false
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) != 0
	case map[string]any:
		return len(typed) != 0
	case bool:
		return typed
	default:
		return true
	}
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func hasOnlyKeys(object map[string]any, allowed ...string) bool {
	allowedKeys := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedKeys[key] = struct{}{}
	}
	for key := range object {
		if _, ok := allowedKeys[key]; !ok {
			return false
		}
	}
	return true
}

func unsupported(reason string) error {
	return fmt.Errorf("%w: %s", ErrUnsupportedRequest, reason)
}
