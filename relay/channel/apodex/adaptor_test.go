package apodex

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

func TestApodexURLAndResponses(t *testing.T) {
	a := &Adaptor{}
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions, ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeApodex}}
	got, err := a.GetRequestURL(info)
	if err != nil || got != "https://api.apodex.ai/v1/chat/completions" {
		t.Fatalf("url=%q err=%v", got, err)
	}
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"
	got, err = a.GetRequestURL(info)
	if err != nil || got != "https://api.apodex.ai/v1/responses" {
		t.Fatalf("responses url=%q err=%v", got, err)
	}
}

func TestApodexUsesDefaultURLWithoutChannelMeta(t *testing.T) {
	a := &Adaptor{}
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions}
	got, err := a.GetRequestURL(info)
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://api.apodex.ai/v1/chat/completions" {
		t.Fatalf("url=%q", got)
	}
}

func TestApodexConvertsRequestsAndAuth(t *testing.T) {
	a := &Adaptor{}
	stream := true
	req := &dto.GeneralOpenAIRequest{Model: "apodex-1.1", Stream: &stream}
	out, err := a.ConvertOpenAIRequest(nil, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions}, req)
	if err != nil || out != req {
		t.Fatalf("convert=%T err=%v", out, err)
	}
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	h := make(http.Header)
	if err := a.SetupRequestHeader(c, &h, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions, ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "k"}}); err != nil {
		t.Fatal(err)
	}
	if h.Get("Authorization") != "Bearer k" {
		t.Fatalf("auth=%q", h.Get("Authorization"))
	}
}

func TestApodexDeduplicatesV1InBaseURL(t *testing.T) {
	a := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://example.test/v1/"},
	}
	got, err := a.GetRequestURL(info)
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.test/v1/chat/completions" {
		t.Fatalf("url=%q", got)
	}
}

func TestApodexRejectsResponsesCompactMode(t *testing.T) {
	a := &Adaptor{}
	_, err := a.GetRequestURL(&relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeResponsesCompact})
	if err == nil {
		t.Fatal("expected compact responses mode to be rejected")
	}
}

func TestApodexRejectsUnsupportedRelayMode(t *testing.T) {
	a := &Adaptor{}
	_, err := a.GetRequestURL(&relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeEmbeddings})
	if err == nil {
		t.Fatal("expected unsupported relay mode to be rejected")
	}
}

func TestApodexClearsStreamOptionsForDeepModels(t *testing.T) {
	a := &Adaptor{}
	streamOptions := &dto.StreamOptions{IncludeUsage: true}
	stream := true
	req := &dto.GeneralOpenAIRequest{Model: "apodex-1-1-deep-research", Stream: &stream, StreamOptions: streamOptions}
	got, err := a.ConvertOpenAIRequest(nil, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions}, req)
	if err != nil {
		t.Fatal(err)
	}
	if got != req {
		t.Fatal("expected request to be converted in place")
	}
	if req.StreamOptions != nil {
		t.Fatal("deep model must not receive stream_options")
	}
}

func TestApodexDefaultsOmittedCoreChatStreamToFalse(t *testing.T) {
	a := &Adaptor{}
	req := &dto.GeneralOpenAIRequest{Model: "apodex-1.1"}
	if _, err := a.ConvertOpenAIRequest(nil, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions}, req); err != nil {
		t.Fatal(err)
	}
	if req.Stream == nil || *req.Stream {
		t.Fatalf("omitted stream must be explicit false, got %#v", req.Stream)
	}
}

func TestApodexDefaultsOmittedDeepChatStreamToFalse(t *testing.T) {
	a := &Adaptor{}
	req := &dto.GeneralOpenAIRequest{Model: "apodex-1-1-deep-research"}
	if _, err := a.ConvertOpenAIRequest(nil, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions}, req); err != nil {
		t.Fatal(err)
	}
	if req.Stream == nil || *req.Stream {
		t.Fatalf("omitted stream must be explicit false for deep models, got %#v", req.Stream)
	}
}

func TestApodexPreservesStreamOptionsForCoreModels(t *testing.T) {
	a := &Adaptor{}
	streamOptions := &dto.StreamOptions{IncludeUsage: true}
	req := &dto.GeneralOpenAIRequest{Model: "apodex-1.1", StreamOptions: streamOptions}
	if _, err := a.ConvertOpenAIRequest(nil, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions}, req); err != nil {
		t.Fatal(err)
	}
	if req.StreamOptions != streamOptions {
		t.Fatal("core model stream_options should be preserved")
	}
}

func TestApodexAcceptsResponsesMode(t *testing.T) {
	a := &Adaptor{}
	stream := true
	req := dto.OpenAIResponsesRequest{Model: "apodex-1.1", Stream: &stream}
	if _, err := a.ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeResponses}, req); err != nil {
		t.Fatal(err)
	}
}

func TestApodexDefaultsOmittedResponsesStreamToFalse(t *testing.T) {
	a := &Adaptor{}
	req := dto.OpenAIResponsesRequest{Model: "apodex-1-1-deep-research"}
	converted, err := a.ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeResponses}, req)
	if err != nil {
		t.Fatal(err)
	}
	convertedReq, ok := converted.(dto.OpenAIResponsesRequest)
	if !ok {
		t.Fatalf("converted request type = %T", converted)
	}
	if convertedReq.Stream == nil || *convertedReq.Stream {
		t.Fatalf("omitted stream must be explicit false, got %#v", convertedReq.Stream)
	}
}

func TestApodexRejectsResponsesConversionInChatMode(t *testing.T) {
	a := &Adaptor{}
	req := dto.OpenAIResponsesRequest{Model: "apodex-1.1"}
	if _, err := a.ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions}, req); err == nil {
		t.Fatal("expected responses conversion to reject chat mode")
	}
}

func TestApodexSetupRequestHeaderRejectsNilInfo(t *testing.T) {
	a := &Adaptor{}
	h := make(http.Header)
	if err := a.SetupRequestHeader(nil, &h, nil); err == nil {
		t.Fatal("expected nil relay info to be rejected")
	}
}

func TestApodexSetupRequestHeaderRejectsNilContext(t *testing.T) {
	a := &Adaptor{}
	h := make(http.Header)
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "k"},
	}
	if err := a.SetupRequestHeader(nil, &h, info); err == nil {
		t.Fatal("expected nil gin context to be rejected")
	}
}

func TestApodexSetupRequestHeaderRejectsNilRequest(t *testing.T) {
	a := &Adaptor{}
	h := make(http.Header)
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "k"},
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if err := a.SetupRequestHeader(c, &h, info); err == nil {
		t.Fatal("expected missing request to be rejected")
	}
}
