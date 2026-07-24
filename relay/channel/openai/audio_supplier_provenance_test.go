package openai

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenaiTTSHandlerMarksLocallyCountedUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
	info := &relaycommon.RelayInfo{Request: &dto.AudioRequest{ResponseFormat: "pcm"}}
	info.SetEstimatePromptTokens(10)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(make([]byte, 960))),
	}

	usage := OpenaiTTSHandler(ctx, resp, info)

	require.NotNil(t, usage)
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens))
}

func TestOpenaiSTTHandlerMarksFallbackButNotUpstreamUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fallbackCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	fallbackCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)
	fallbackInfo := &relaycommon.RelayInfo{}
	fallbackInfo.SetEstimatePromptTokens(12)
	_, fallbackUsage := OpenaiSTTHandler(fallbackCtx, &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"text":"ok"}`)),
	}, fallbackInfo, "json")
	require.NotNil(t, fallbackUsage)
	require.True(t, common.GetContextKeyBool(fallbackCtx, constant.ContextKeyLocalCountTokens))

	upstreamCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	upstreamCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)
	_, upstreamUsage := OpenaiSTTHandler(upstreamCtx, &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"usage":{"total_tokens":9,"input_tokens":7,"output_tokens":2}}`)),
	}, &relaycommon.RelayInfo{}, "json")
	require.NotNil(t, upstreamUsage)
	require.False(t, common.GetContextKeyBool(upstreamCtx, constant.ContextKeyLocalCountTokens))
}
