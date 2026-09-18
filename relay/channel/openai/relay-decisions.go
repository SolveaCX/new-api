package openai

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/openrouter"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// openRouterDecisionsUsage is kept deliberately small: OpenRouter may add
// provider-specific usage fields, but these are the fields required for
// Flatkey's token settlement.  The legacy prompt/completion names are accepted
// as a compatibility fallback for gateways that normalize the response.
type openRouterDecisionsUsage struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	Cost             any `json:"cost,omitempty"`
}

type openRouterDecisionsResponse struct {
	Answers json.RawMessage          `json:"answers"`
	Usage   openRouterDecisionsUsage `json:"usage"`
	Error   json.RawMessage          `json:"error,omitempty"`
}

// OpenRouterDecisionsHandler passes the native /api/alpha/decisions response
// through unchanged while extracting usage for the normal Flatkey billing
// pipeline.  Decisions responses are not chat completions and must not be
// coerced into an OpenAI choices/message envelope.
func OpenRouterDecisionsHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(errors.New("invalid response or response body"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusBadGateway)
	}
	if info != nil && info.ChannelMeta != nil &&
		info.ChannelType == constant.ChannelTypeOpenRouter &&
		info.ChannelOtherSettings.IsOpenRouterEnterprise() {
		var enterpriseResponse openrouter.OpenRouterEnterpriseResponse
		if err = common.Unmarshal(responseBody, &enterpriseResponse); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
		}
		if !enterpriseResponse.Success {
			return nil, types.NewOpenAIError(errors.New("openrouter response success=false"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
		}
		responseBody = enterpriseResponse.Data
	}

	var envelope openRouterDecisionsResponse
	if err = common.Unmarshal(responseBody, &envelope); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}

	// A few gateways return an error envelope with HTTP 200.  Preserve the
	// regular OpenAI error semantics instead of charging a successful request.
	if len(envelope.Error) > 0 && common.GetJsonType(envelope.Error) != "null" {
		errorStatus := resp.StatusCode
		if errorStatus < http.StatusBadRequest {
			errorStatus = http.StatusBadGateway
		}
		var generalError dto.GeneralErrorResponse
		if unmarshalErr := common.Unmarshal(responseBody, &generalError); unmarshalErr == nil {
			if openAIError := generalError.TryToOpenAIError(); openAIError != nil {
				return nil, types.WithOpenAIError(*openAIError, errorStatus)
			}
			if message := generalError.ToMessage(); message != "" {
				return nil, types.NewOpenAIError(errors.New(message), types.ErrorCodeBadResponse, errorStatus)
			}
		}
		return nil, types.NewOpenAIError(errors.New("OpenRouter decisions returned an error"), types.ErrorCodeBadResponse, errorStatus)
	}
	if len(envelope.Answers) == 0 || common.GetJsonType(envelope.Answers) != "object" {
		return nil, types.NewOpenAIError(errors.New("OpenRouter decisions response is missing answers"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}

	inputTokens := envelope.Usage.InputTokens
	if inputTokens == 0 {
		inputTokens = envelope.Usage.PromptTokens
	}
	outputTokens := envelope.Usage.OutputTokens
	if outputTokens == 0 {
		outputTokens = envelope.Usage.CompletionTokens
	}
	totalTokens := envelope.Usage.TotalTokens
	if inputTokens < 0 || outputTokens < 0 || totalTokens < 0 {
		return nil, types.NewOpenAIError(errors.New("OpenRouter decisions response contains negative usage"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if totalTokens == 0 {
		totalTokens = inputTokens + outputTokens
	}
	if inputTokens == 0 && info != nil {
		// Keep billing deterministic if an upstream-compatible gateway omits
		// usage: the relay already estimated the prompt before dispatch.
		inputTokens = info.GetEstimatePromptTokens()
		if totalTokens == 0 {
			totalTokens = inputTokens + outputTokens
		}
	}

	usage := &dto.Usage{
		PromptTokens:     inputTokens,
		CompletionTokens: outputTokens,
		TotalTokens:      totalTokens,
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
		Cost:             envelope.Usage.Cost,
	}
	// Decisions tokens are text tokens for the purposes of the existing
	// tiered/audio billing summaries.
	usage.PromptTokensDetails.TextTokens = inputTokens
	usage.CompletionTokenDetails.TextTokens = outputTokens

	// Parse first, then write.  This follows the relay convention that a
	// malformed successful body can still be represented as a local error
	// response, while valid native answers remain byte-for-byte compatible.
	service.IOCopyBytesGracefully(c, resp, responseBody)
	return usage, nil
}
