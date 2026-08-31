package dto

import (
	"bytes"
	"encoding/json"
	"regexp"

	"github.com/QuantumNous/new-api/common"
)

var claudeWebToolTypePattern = regexp.MustCompile(`^web_(search|fetch)_[0-9]{8}$`)

// Use a distinct type to avoid recursively invoking the JSON methods below.
type toolCallRequestJSON ToolCallRequest

func (t *ToolCallRequest) UnmarshalJSON(data []byte) error {
	var decoded toolCallRequestJSON
	if err := common.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if claudeWebToolTypePattern.MatchString(decoded.Type) {
		// Keep version-specific options, unknown extensions, and explicit zero values.
		decoded.claudeWebTool = bytes.Clone(data)
	}
	*t = ToolCallRequest(decoded)
	return nil
}

func (t ToolCallRequest) MarshalJSON() ([]byte, error) {
	if raw := t.ClaudeWebTool(); len(raw) > 0 {
		return common.Marshal(raw)
	}
	return common.Marshal(toolCallRequestJSON(t))
}

// ClaudeWebTool returns only native server web tools, never client function names.
func (t ToolCallRequest) ClaudeWebTool() json.RawMessage {
	if !claudeWebToolTypePattern.MatchString(t.Type) {
		return nil
	}
	return t.claudeWebTool
}
