package service

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClaudeMessageParseContent_PreservesVideoURL(t *testing.T) {
	msg := dto.ClaudeMessage{
		Role: "user",
		Content: []any{
			map[string]any{
				"type":      "video",
				"video_url": "https://example.com/demo.mp4",
			},
		},
	}

	parts, err := msg.ParseContent()
	require.NoError(t, err)
	require.Len(t, parts, 1)
	require.Equal(t, "video", parts[0].Type)
	require.NotNil(t, parts[0].Source)
	require.Equal(t, "https://example.com/demo.mp4", parts[0].Source.Url)
	require.Equal(t, "https://example.com/demo.mp4", parts[0].ToFileSource().GetRawData())
}

func TestClaudeToOpenAIRequest_TransfersVideoURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
		},
	}
	req := dto.ClaudeRequest{
		Model: "kimi-k3",
		Messages: []dto.ClaudeMessage{
			{
				Role: "user",
				Content: []any{
					map[string]any{
						"type":      "video",
						"video_url": "https://example.com/demo.mp4",
					},
				},
			},
		},
	}

	openAIReq, err := ClaudeToOpenAIRequest(req, info)
	require.NoError(t, err)
	require.Len(t, openAIReq.Messages, 1)

	parts := openAIReq.Messages[0].ParseContent()
	require.Len(t, parts, 1)
	require.Equal(t, dto.ContentTypeVideoUrl, parts[0].Type)
	require.NotNil(t, parts[0].VideoUrl)
	require.Equal(t, "https://example.com/demo.mp4", parts[0].VideoUrl.(*dto.MessageVideoUrl).Url)
}
