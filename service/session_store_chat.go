package service

import (
	"bytes"
	"errors"
	"fmt"
	"sort"

	"github.com/QuantumNous/new-api/common"
)

// Keep the client-facing Chat Completions format, including reasoning and tool
// arguments, rather than manufacturing Anthropic content or thinking signatures.
func canonicalChatCompletionResponse(body []byte) ([]byte, map[string]any, bool, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, nil, false, errors.New("empty Chat Completions response")
	}
	if trimmed[0] == '{' {
		var response map[string]any
		if err := common.Unmarshal(trimmed, &response); err != nil {
			return nil, nil, false, fmt.Errorf("invalid Chat Completions response JSON: %w", err)
		}
		return append([]byte(nil), trimmed...), response, true, nil
	}

	response := make(map[string]any)
	choices := make(map[int]map[string]any)
	sawDone := false
	for _, data := range sessionStoreSSEData(trimmed) {
		if data == "[DONE]" {
			sawDone = true
			break
		}
		var chunk map[string]any
		if err := common.UnmarshalJsonStr(data, &chunk); err != nil {
			return nil, nil, false, fmt.Errorf("invalid Chat Completions SSE event: %w", err)
		}
		if chunk["error"] != nil {
			encoded, err := common.Marshal(chunk)
			return encoded, chunk, true, err
		}
		for key, value := range chunk {
			if key != "choices" && value != nil {
				response[key] = value
			}
		}
		items, _ := chunk["choices"].([]any)
		for _, item := range items {
			incoming, ok := item.(map[string]any)
			if !ok {
				return nil, nil, false, errors.New("invalid Chat Completions choice")
			}
			index, err := chatCaptureIndex(incoming)
			if err != nil {
				return nil, nil, false, err
			}
			choice := choices[index]
			if choice == nil {
				choice = map[string]any{"index": index, "message": map[string]any{}, "finish_reason": nil}
				choices[index] = choice
			}
			for key, value := range incoming {
				if key != "delta" && value != nil {
					choice[key] = value
				}
			}
			delta, _ := incoming["delta"].(map[string]any)
			if err := mergeChatCaptureDelta(choice["message"].(map[string]any), delta); err != nil {
				return nil, nil, false, err
			}
		}
	}
	if len(choices) == 0 {
		return nil, nil, false, errors.New("Chat Completions stream did not contain choices")
	}
	ordered := make([]any, 0, len(choices))
	complete := sawDone
	indices := make([]int, 0, len(choices))
	for index := range choices {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	for _, index := range indices {
		choice := choices[index]
		if reason, _ := choice["finish_reason"].(string); reason == "" {
			complete = false
		}
		ordered = append(ordered, choice)
	}
	response["object"] = "chat.completion"
	response["choices"] = ordered
	encoded, err := common.Marshal(response)
	return encoded, response, complete, err
}

func chatCaptureIndex(item map[string]any) (int, error) {
	value, ok := item["index"].(float64)
	if !ok || value < 0 || value > float64(sessionStoreMaxBodyBytes) || value != float64(int(value)) {
		return 0, errors.New("invalid Chat Completions stream index")
	}
	return int(value), nil
}

// Strings in deltas are fragments; role/id/type are identifiers. Indexed tool
// calls are merged separately so interleaved calls never share arguments.
func mergeChatCaptureDelta(target, delta map[string]any) error {
	for key, value := range delta {
		if value == nil {
			continue
		}
		switch value := value.(type) {
		case string:
			if key == "role" || key == "id" || key == "type" {
				target[key] = value
			} else {
				existing, _ := target[key].(string)
				target[key] = existing + value
			}
		case map[string]any:
			existing, _ := target[key].(map[string]any)
			if existing == nil {
				existing = make(map[string]any)
				target[key] = existing
			}
			if err := mergeChatCaptureDelta(existing, value); err != nil {
				return err
			}
		case []any:
			existing, _ := target[key].([]any)
			if key != "tool_calls" {
				target[key] = append(existing, value...)
				continue
			}
			for _, item := range value {
				tool, ok := item.(map[string]any)
				if !ok {
					return errors.New("invalid Chat Completions tool call")
				}
				index, err := chatCaptureIndex(tool)
				if err != nil {
					return err
				}
				var merged map[string]any
				for _, candidate := range existing {
					call := candidate.(map[string]any)
					if call["index"] == float64(index) {
						merged = call
						break
					}
				}
				if merged == nil {
					merged = make(map[string]any)
					existing = append(existing, merged)
				}
				if err := mergeChatCaptureDelta(merged, tool); err != nil {
					return err
				}
			}
			sort.Slice(existing, func(i, j int) bool {
				return existing[i].(map[string]any)["index"].(float64) < existing[j].(map[string]any)["index"].(float64)
			})
			target[key] = existing
		default:
			target[key] = value
		}
	}
	return nil
}
