package oxalpha

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

var _ channel.Adaptor = (*Adaptor)(nil)

func testContext(method, path string) *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, nil)
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestGetRequestURLUsesDefaultAndNormalizesV1(t *testing.T) {
	cases := []struct {
		name string
		base string
		want string
	}{
		{
			name: "default",
			want: "https://oxalpha.run/api/v1/chat/completions",
		},
		{
			name: "api",
			base: "https://proxy.example/api",
			want: "https://proxy.example/api/v1/chat/completions",
		},
		{
			name: "api-v1",
			base: "https://proxy.example/api/v1",
			want: "https://proxy.example/api/v1/chat/completions",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				RelayMode: relayconstant.RelayModeChatCompletions,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:    constant.ChannelTypeOxAlpha,
					ChannelBaseUrl: tc.base,
				},
				RequestURLPath: "/v1/chat/completions",
			}
			got, err := (&Adaptor{}).GetRequestURL(info)
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("GetRequestURL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGetRequestURLRejectsUnsupportedRelayMode(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeOxAlpha,
			ChannelBaseUrl: "https://oxalpha.run/api",
		},
		RequestURLPath: "/v1/embeddings",
	}

	if _, err := (&Adaptor{}).GetRequestURL(info); err == nil {
		t.Fatal("expected unsupported relay mode error")
	}
}

func TestSetupRequestHeaderSetsBearerAndPreservesContentType(t *testing.T) {
	c := testContext(http.MethodPost, "/v1/chat/completions")
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "test-key"},
	}
	headers := make(http.Header)

	if err := (&Adaptor{}).SetupRequestHeader(c, &headers, info); err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}
	if got := headers.Get("Authorization"); got != "Bearer test-key" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer test-key")
	}
	if got := headers.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
}

func TestConvertOpenAIRequestPassesThroughPointer(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{Model: "model-chosen-later"}
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	got, ok := converted.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("converted request type = %T, want *dto.GeneralOpenAIRequest", converted)
	}
	if got != request {
		t.Fatalf("ConvertOpenAIRequest returned a different pointer")
	}
	if got.Model != "model-chosen-later" {
		t.Fatalf("model changed to %q", got.Model)
	}
}

func TestConvertOpenAIRequestDropsUnsupportedStreamOptions(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "model-chosen-later",
		StreamOptions: &dto.StreamOptions{
			IncludeUsage: true,
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeChatCompletions,
	}, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	got, ok := converted.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("converted request type = %T, want *dto.GeneralOpenAIRequest", converted)
	}
	if got != request {
		t.Fatalf("ConvertOpenAIRequest returned a different pointer")
	}
	if got.StreamOptions != nil {
		t.Fatalf("StreamOptions = %#v, want nil for unsupported channel capability", got.StreamOptions)
	}
}

func TestConvertOpenAIRequestRejectsNilAndUnsupportedMode(t *testing.T) {
	adaptor := &Adaptor{}
	if _, err := adaptor.ConvertOpenAIRequest(nil, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeChatCompletions,
	}, nil); err == nil {
		t.Fatal("expected nil request error")
	}
	if _, err := adaptor.ConvertOpenAIRequest(nil, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
	}, &dto.GeneralOpenAIRequest{}); err == nil {
		t.Fatal("expected unsupported mode error")
	}
}

func TestUnsupportedConversionsReturnErrors(t *testing.T) {
	adaptor := &Adaptor{}
	c := testContext(http.MethodPost, "/v1/chat/completions")
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions}

	if _, err := adaptor.ConvertRerankRequest(c, relayconstant.RelayModeRerank, dto.RerankRequest{}); err == nil {
		t.Fatal("expected rerank unsupported error")
	}
	if _, err := adaptor.ConvertEmbeddingRequest(c, info, dto.EmbeddingRequest{}); err == nil {
		t.Fatal("expected embedding unsupported error")
	}
	if _, err := adaptor.ConvertAudioRequest(c, info, dto.AudioRequest{}); err == nil {
		t.Fatal("expected audio unsupported error")
	}
	if _, err := adaptor.ConvertImageRequest(c, info, dto.ImageRequest{}); err == nil {
		t.Fatal("expected image unsupported error")
	}
	if _, err := adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{}); err == nil {
		t.Fatal("expected responses unsupported error")
	}
	if _, err := adaptor.ConvertClaudeRequest(c, info, &dto.ClaudeRequest{}); err == nil {
		t.Fatal("expected Claude unsupported error")
	}
	if _, err := adaptor.ConvertGeminiRequest(c, info, &dto.GeminiChatRequest{}); err == nil {
		t.Fatal("expected Gemini unsupported error")
	}
}

func TestAdaptorMetadata(t *testing.T) {
	adaptor := &Adaptor{}
	if got := adaptor.GetModelList(); len(got) != 1 || got[0] != "ox-alpha" {
		t.Fatalf("GetModelList = %#v, want [ox-alpha]", got)
	}
	if got := adaptor.GetChannelName(); got != "oxalpha" {
		t.Fatalf("GetChannelName = %q, want oxalpha", got)
	}
}

func TestDoResponseDelegatesOpenAIHandler(t *testing.T) {
	c := testContext(http.MethodPost, "/v1/chat/completions")
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeChatCompletions,
		IsStream:  false,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOxAlpha,
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl-test","object":"chat.completion","model":"ox-alpha","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`)),
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	if apiErr != nil {
		t.Fatalf("DoResponse returned error: %v", apiErr)
	}
	got, ok := usage.(*dto.Usage)
	if !ok {
		t.Fatalf("usage type = %T, want *dto.Usage", usage)
	}
	if got.TotalTokens != 3 {
		t.Fatalf("TotalTokens = %d, want 3", got.TotalTokens)
	}
}

func TestGetRequestURLRejectsNilInfo(t *testing.T) {
	if _, err := (&Adaptor{}).GetRequestURL(nil); err == nil {
		t.Fatal("expected nil relay info error")
	}
}
