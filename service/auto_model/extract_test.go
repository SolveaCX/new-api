package auto_model

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestExtractTextSupportedProtocols(t *testing.T) {
	tests := []struct {
		name     string
		protocol Protocol
		body     string
		contains []string
	}{
		{
			name:     "chat text blocks",
			protocol: ProtocolChatCompletions,
			body:     `{"messages":[{"role":"system","content":"Be precise"},{"role":"user","content":[{"type":"text","text":"写一个 Go parser"}]}]}`,
			contains: []string{"[system]", "Be precise", "[user]", "写一个 Go parser"},
		},
		{
			name:     "responses string",
			protocol: ProtocolResponses,
			body:     `{"instructions":"Translate carefully","input":"你好"}`,
			contains: []string{"[instructions]", "Translate carefully", "[user]", "你好"},
		},
		{
			name:     "responses simple messages",
			protocol: ProtocolResponses,
			body:     `{"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"Prove this theorem"}]}]}`,
			contains: []string{"[user]", "Prove this theorem"},
		},
		{
			name:     "anthropic text blocks",
			protocol: ProtocolMessages,
			body:     `{"system":[{"type":"text","text":"You are concise"}],"messages":[{"role":"user","content":[{"type":"text","text":"Explain channels"}]}]}`,
			contains: []string{"[system]", "You are concise", "[user]", "Explain channels"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			text, err := ExtractText(test.protocol, []byte(test.body), 8000)
			require.NoError(t, err)
			for _, expected := range test.contains {
				require.Contains(t, text, expected)
			}
		})
	}
}

func TestExtractTextRejectsUnsupportedForms(t *testing.T) {
	tests := []struct {
		protocol Protocol
		body     string
	}{
		{ProtocolChatCompletions, `{"tools":[{"type":"function"}],"messages":[{"role":"user","content":"hello"}]}`},
		{ProtocolChatCompletions, `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.com/a.png"}}]}]}`},
		{ProtocolResponses, `{"previous_response_id":"resp_1","input":"continue"}`},
		{ProtocolResponses, `{"input":[{"type":"function_call","name":"lookup"}]}`},
		{ProtocolResponses, `{"input":"hello","text":{"format":{"type":"json_schema"}}}`},
		{ProtocolMessages, `{"messages":[{"role":"user","content":[{"type":"tool_result","content":"secret"}]}]}`},
		{ProtocolMessages, `{"tools":[{"name":"lookup"}],"messages":[{"role":"user","content":"hello"}]}`},
		{ProtocolChatCompletions, `{"messages":[{"role":"user","content":"hello","provider":"custom"}]}`},
		{ProtocolChatCompletions, `{"messages":[{"role":"user","content":[{"type":"text","text":"hello","cache_control":{"type":"ephemeral"}}]}]}`},
		{ProtocolResponses, `{"input":[{"type":"message","role":"user","content":"hello","provider":"custom"}]}`},
		{ProtocolResponses, `{"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello","cache_control":{"type":"ephemeral"}}]}]}`},
		{ProtocolMessages, `{"system":[{"type":"text","text":"system","cache_control":{"type":"ephemeral"}}],"messages":[{"role":"user","content":"hello"}]}`},
		{ProtocolMessages, `{"messages":[{"role":"user","content":[{"type":"text","text":"hello","cache_control":{"type":"ephemeral"}}]}]}`},
	}
	for _, test := range tests {
		_, err := ExtractText(test.protocol, []byte(test.body), 8000)
		require.True(t, errors.Is(err, ErrUnsupportedRequest), "error: %v", err)
	}
}

func TestExtractTextTruncationPreservesPrefixRecentTextAndUnicode(t *testing.T) {
	body := `{"messages":[{"role":"system","content":"SYSTEM-规则"},{"role":"user","content":"` + strings.Repeat("旧内容", 100) + `"},{"role":"user","content":"RECENT-最近问题"}]}`
	text, err := ExtractText(ProtocolChatCompletions, []byte(body), 80)
	require.NoError(t, err)
	require.LessOrEqual(t, utf8.RuneCountInString(text), 80)
	require.Contains(t, text, "SYSTEM")
	require.Contains(t, text, "RECENT-最近问题")
	require.True(t, utf8.ValidString(text))
}
