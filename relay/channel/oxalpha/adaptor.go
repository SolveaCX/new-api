// Package oxalpha implements the OpenAI-compatible Ox Alpha preview channel.
package oxalpha

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// Adaptor implements channel.Adaptor for Ox Alpha's OpenAI-compatible chat API.
type Adaptor struct{}

var _ channel.Adaptor = (*Adaptor)(nil)

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", errors.New("oxalpha: relay info is nil")
	}
	if info.RelayMode != relayconstant.RelayModeChatCompletions {
		return "", fmt.Errorf(
			"oxalpha: relay mode %d is not supported; use /v1/chat/completions",
			info.RelayMode,
		)
	}

	baseURL := strings.TrimRight(info.ChannelBaseUrl, "/")
	if baseURL == "" {
		if len(channelconstant.ChannelBaseURLs) <= channelconstant.ChannelTypeOxAlpha {
			return "", errors.New("oxalpha: default base URL is not configured")
		}
		baseURL = strings.TrimRight(
			channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeOxAlpha],
			"/",
		)
	}

	requestPath := info.RequestURLPath
	if requestPath == "" {
		requestPath = "/v1/chat/completions"
	}
	if !strings.HasPrefix(requestPath, "/") {
		requestPath = "/" + requestPath
	}
	if strings.HasSuffix(baseURL, "/v1") && strings.HasPrefix(requestPath, "/v1/") {
		requestPath = strings.TrimPrefix(requestPath, "/v1")
	}

	return relaycommon.GetFullRequestURL(baseURL, requestPath, info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(
	c *gin.Context,
	req *http.Header,
	info *relaycommon.RelayInfo,
) error {
	if info == nil {
		return errors.New("oxalpha: relay info is nil")
	}
	if c == nil || c.Request == nil {
		return errors.New("oxalpha: request context is nil")
	}
	if req == nil {
		return errors.New("oxalpha: request headers are nil")
	}
	if info.RelayMode != relayconstant.RelayModeChatCompletions {
		return fmt.Errorf("oxalpha: relay mode %d is not supported", info.RelayMode)
	}

	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	request *dto.GeneralOpenAIRequest,
) (any, error) {
	if request == nil {
		return nil, errors.New("oxalpha: request is nil")
	}
	if info != nil && info.RelayMode != relayconstant.RelayModeChatCompletions {
		return nil, fmt.Errorf("oxalpha: relay mode %d is not supported", info.RelayMode)
	}
	// Ox Alpha's documented API does not advertise stream_options support.
	// Keep the request model and all other OpenAI-compatible fields unchanged,
	// but prevent this optional field from reaching the upstream endpoint.
	request.StreamOptions = nil
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(
	c *gin.Context,
	relayMode int,
	request dto.RerankRequest,
) (any, error) {
	return nil, errors.New("oxalpha: rerank is not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	request dto.EmbeddingRequest,
) (any, error) {
	return nil, errors.New("oxalpha: embeddings are not supported")
}

func (a *Adaptor) ConvertAudioRequest(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	request dto.AudioRequest,
) (io.Reader, error) {
	return nil, errors.New("oxalpha: audio is not supported")
}

func (a *Adaptor) ConvertImageRequest(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	request dto.ImageRequest,
) (any, error) {
	return nil, errors.New("oxalpha: images are not supported")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	request dto.OpenAIResponsesRequest,
) (any, error) {
	return nil, errors.New("oxalpha: Responses API is not supported")
}

func (a *Adaptor) ConvertClaudeRequest(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	request *dto.ClaudeRequest,
) (any, error) {
	return nil, errors.New("oxalpha: Claude API is not supported")
}

func (a *Adaptor) ConvertGeminiRequest(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	request *dto.GeminiChatRequest,
) (any, error) {
	return nil, errors.New("oxalpha: Gemini API is not supported")
}

func (a *Adaptor) DoRequest(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	requestBody io.Reader,
) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(
	c *gin.Context,
	resp *http.Response,
	info *relaycommon.RelayInfo,
) (usage any, err *types.NewAPIError) {
	adaptor := openai.Adaptor{}
	return adaptor.DoResponse(c, resp, info)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
