package apodex

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type Adaptor struct{}

var _ channel.Adaptor = (*Adaptor)(nil)

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", errors.New("apodex: relay info is nil")
	}
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		return "", errors.New("apodex: responses compact is not supported")
	}
	if info.RelayMode != relayconstant.RelayModeChatCompletions && info.RelayMode != relayconstant.RelayModeResponses {
		return "", fmt.Errorf("apodex: relay mode %d is not supported", info.RelayMode)
	}
	base := ""
	channelType := 0
	if info.ChannelMeta != nil {
		base = strings.TrimRight(info.ChannelBaseUrl, "/")
		channelType = info.ChannelType
	}
	if base == "" {
		base = constant.ChannelBaseURLs[constant.ChannelTypeApodex]
	}
	path := info.RequestURLPath
	if path == "" {
		if info.RelayMode == relayconstant.RelayModeResponses {
			path = "/v1/responses"
		} else {
			path = "/v1/chat/completions"
		}
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if strings.HasPrefix(path, "/v1/responses/compact") {
		return "", errors.New("apodex: responses compact is not supported")
	}
	if strings.HasSuffix(base, "/v1") && strings.HasPrefix(path, "/v1/") {
		path = strings.TrimPrefix(path, "/v1")
	}
	return relaycommon.GetFullRequestURL(base, path, channelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, h *http.Header, info *relaycommon.RelayInfo) error {
	if info == nil || info.ChannelMeta == nil || c == nil || c.Request == nil || h == nil {
		return errors.New("apodex: invalid request info or headers")
	}
	if *h == nil {
		*h = make(http.Header)
	}
	channel.SetupApiRequestHeader(info, c, h)
	h.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(_ *gin.Context, info *relaycommon.RelayInfo, req *dto.GeneralOpenAIRequest) (any, error) {
	if req == nil {
		return nil, errors.New("apodex: request is nil")
	}
	if info != nil && info.RelayMode != relayconstant.RelayModeChatCompletions {
		return nil, fmt.Errorf("apodex: relay mode %d is not supported", info.RelayMode)
	}
	// Leave an omitted stream field untouched so Apodex can apply its model
	// specific default. Deep research chat models default to SSE; reflect that
	// upstream behavior in the relay before the response arrives. Explicit
	// stream=true/false remains authoritative through RelayInfo initialization.
	if req.Stream == nil && info != nil && isDeepModel(req.Model) {
		info.IsStream = true
	}
	if isDeepModel(req.Model) {
		req.StreamOptions = nil
	}
	return req, nil
}

func isDeepModel(model string) bool {
	return strings.Contains(model, "deep-") || strings.Contains(model, "deep_")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(_ *gin.Context, info *relaycommon.RelayInfo, req dto.OpenAIResponsesRequest) (any, error) {
	if info != nil && info.RelayMode != relayconstant.RelayModeResponses {
		return nil, fmt.Errorf("apodex: relay mode %d is not supported", info.RelayMode)
	}
	// Apodex Responses defaults to a streaming SSE response when stream is
	// omitted. Keep the field omitted and prime relay response dispatch with
	// that default; an explicit stream value remains unchanged.
	if req.Stream == nil && info != nil {
		info.IsStream = true
	}
	return (&openai.Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, req)
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, body io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, body)
}
func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	return (&openai.Adaptor{}).DoResponse(c, resp, info)
}
func (a *Adaptor) GetModelList() []string { return ModelList }
func (a *Adaptor) GetChannelName() string { return ChannelName }

func unsupported(name string) error { return fmt.Errorf("apodex: %s is not supported", name) }
func (a *Adaptor) ConvertRerankRequest(*gin.Context, int, dto.RerankRequest) (any, error) {
	return nil, unsupported("rerank")
}
func (a *Adaptor) ConvertEmbeddingRequest(*gin.Context, *relaycommon.RelayInfo, dto.EmbeddingRequest) (any, error) {
	return nil, unsupported("embeddings")
}
func (a *Adaptor) ConvertAudioRequest(*gin.Context, *relaycommon.RelayInfo, dto.AudioRequest) (io.Reader, error) {
	return nil, unsupported("audio")
}
func (a *Adaptor) ConvertImageRequest(*gin.Context, *relaycommon.RelayInfo, dto.ImageRequest) (any, error) {
	return nil, unsupported("images")
}
func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	return nil, unsupported("Claude API")
}
func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, unsupported("Gemini API")
}
