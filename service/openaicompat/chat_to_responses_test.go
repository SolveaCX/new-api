package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestChatCompletionsRequestToResponsesRequestPreservesFileURLInputs(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-5.5",
		Messages: []dto.Message{
			{
				Role: "user",
				Content: []any{
					map[string]any{
						"type": "file",
						"file": map[string]any{
							"filename": "report.pdf",
							"file_url": "https://cdn.example.com/report.pdf",
						},
					},
				},
			},
		},
	}

	resp, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	var input []map[string]any
	require.NoError(t, common.Unmarshal(resp.Input, &input))
	require.Len(t, input, 1)
	content := input[0]["content"].([]any)
	require.Len(t, content, 1)
	part := content[0].(map[string]any)
	require.Equal(t, "input_file", part["type"])
	require.Equal(t, "https://cdn.example.com/report.pdf", part["file_url"])
	require.NotContains(t, part, "file")
}
