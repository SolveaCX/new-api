package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

func TestOpenRouterDecisionsHandlerPassesThroughAndMapsUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := `{"model":"typesafe/jev-1.13","answers":{"safe":{"type":"noul","noul":0.9}},"usage":{"input_tokens":32,"output_tokens":3,"cost":0.000001}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeDecisions,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenRouter,
			UpstreamModelName: "typesafe/jev-1.13",
		},
	}

	usage, apiErr := OpenRouterDecisionsHandler(ctx, resp, info)
	if apiErr != nil {
		t.Fatalf("OpenRouterDecisionsHandler returned error: %v", apiErr)
	}
	if usage == nil {
		t.Fatal("OpenRouterDecisionsHandler returned nil usage")
	}
	if usage.InputTokens != 32 || usage.OutputTokens != 3 || usage.TotalTokens != 35 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
	if got := recorder.Body.String(); got != body {
		t.Fatalf("response body changed: got %q, want %q", got, body)
	}
}

func TestOpenRouterDecisionsHandlerRejectsMissingAnswers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"model":"typesafe/jev-1.13","usage":{"input_tokens":1}}`)),
	}

	_, apiErr := OpenRouterDecisionsHandler(ctx, resp, &relaycommon.RelayInfo{})
	if apiErr == nil {
		t.Fatal("expected malformed decisions response error")
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("malformed response should not be written, got %q", recorder.Body.String())
	}
}

func TestOpenRouterDecisionsHandlerClampsEmbeddedErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(
			`{"error":{"type":"invalid_request_error","message":"bad decision"}}`,
		)),
	}

	_, apiErr := OpenRouterDecisionsHandler(ctx, resp, &relaycommon.RelayInfo{})
	if apiErr == nil {
		t.Fatal("expected embedded error response")
	}
	if apiErr.StatusCode < http.StatusBadRequest {
		t.Fatalf("embedded error status = %d, want a non-2xx status", apiErr.StatusCode)
	}
}
