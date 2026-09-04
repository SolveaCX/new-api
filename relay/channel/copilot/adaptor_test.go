package copilot

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func chatInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:            42,
			ChannelMultiKeyIndex: 3,
			ChannelBaseUrl:       "https://attacker.example",
			ApiKey:               "gho_github-credential",
			ChannelSetting:       dto.ChannelSettings{Proxy: "socks5://proxy.example:1080"},
		},
	}
}

func TestGetRequestURLUsesOfficialChatEndpoint(t *testing.T) {
	got, err := (&Adaptor{}).GetRequestURL(chatInfo())
	if err != nil {
		t.Fatal(err)
	}
	if got != chatCompletionsURL {
		t.Fatalf("URL = %q, want %q", got, chatCompletionsURL)
	}

	unsupported := chatInfo()
	unsupported.RelayMode = relayconstant.RelayModeResponses
	if _, err := (&Adaptor{}).GetRequestURL(unsupported); !errors.Is(err, errUnsupportedEndpoint) {
		t.Fatalf("unsupported URL error = %v", err)
	}
}

func TestGetRequestURLUsesNativeClaudeMessagesEndpoint(t *testing.T) {
	info := chatInfo()
	info.RelayFormat = types.RelayFormatClaude
	info.IsStream = true
	got, err := (&Adaptor{}).GetRequestURL(info)
	if err != nil {
		t.Fatal(err)
	}
	if got != claudeMessagesURL {
		t.Fatalf("URL = %q, want %q", got, claudeMessagesURL)
	}
}

func TestSetupRequestHeaderUsesDeviceFlowCredentialDirectly(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	header := http.Header{}
	if err := (&Adaptor{}).SetupRequestHeader(c, &header, chatInfo()); err != nil {
		t.Fatal(err)
	}

	wants := map[string]string{
		"Authorization":        "Bearer gho_github-credential",
		"Accept":               "application/json",
		"Content-Type":         "application/json",
		"User-Agent":           userAgent,
		"Openai-Intent":        openAIIntent,
		"X-GitHub-Api-Version": githubAPIVersion,
		"x-initiator":          "user",
	}
	for name, want := range wants {
		if got := header.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if header.Get("X-Request-Id") == "" {
		t.Fatal("X-Request-Id is missing")
	}
}

func TestSetupRequestHeaderUsesClaudeCopilotHeaders(t *testing.T) {
	info := chatInfo()
	info.RelayFormat = types.RelayFormatClaude
	info.IsStream = true
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader("{}"))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	header := http.Header{}
	require.NoError(t, (&Adaptor{}).SetupRequestHeader(c, &header, info))
	require.Equal(t, "text/event-stream", header.Get("Accept"))
	require.Equal(t, "2023-06-01", header.Get("anthropic-version"))
	require.Equal(t, "Bearer gho_github-credential", header.Get("Authorization"))
}

func TestSetupRequestHeaderUsesJSONForNonStreamingClaude(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	info := chatInfo()
	info.RelayFormat = types.RelayFormatClaude
	info.IsStream = false
	var header http.Header = make(http.Header)
	require.NoError(t, (&Adaptor{}).SetupRequestHeader(c, &header, info))
	require.Equal(t, "application/json", header.Get("Accept"))
}

func TestConvertClaudeRequestUsesNativePassthrough(t *testing.T) {
	info := chatInfo()
	info.RelayFormat = types.RelayFormatClaude
	request := &dto.ClaudeRequest{Model: "claude-sonnet-4"}
	converted, err := (&Adaptor{}).ConvertClaudeRequest(nil, info, request)
	require.NoError(t, err)
	require.Same(t, request, converted)
}

func TestUnsupportedConversionsFailClearly(t *testing.T) {
	a := &Adaptor{}
	if _, err := a.ConvertEmbeddingRequest(nil, chatInfo(), dto.EmbeddingRequest{}); !errors.Is(err, errUnsupportedEndpoint) {
		t.Fatalf("embedding error = %v", err)
	}
	if _, err := a.ConvertOpenAIResponsesRequest(nil, chatInfo(), dto.OpenAIResponsesRequest{}); !errors.Is(err, errUnsupportedEndpoint) {
		t.Fatalf("responses error = %v", err)
	}
}

func TestScrubCopilotResponseHeaders(t *testing.T) {
	resp := &http.Response{Header: http.Header{
		"X-GitHub-Request-Id":          []string{"github-request"},
		"X-GitHub-Copilot-Request-Te":  []string{"true"},
		"X-Copilot-Service-Request-Id": []string{"service-request"},
		"X-Request-Id":                 []string{"local-request"},
	}}

	scrubCopilotResponseHeaders(resp)

	for _, name := range []string{
		"x-github-request-id",
		"x-github-copilot-request-te",
		"x-copilot-service-request-id",
	} {
		if got := resp.Header.Get(name); got != "" {
			t.Fatalf("%s leaked: %q", name, got)
		}
	}
	if got := resp.Header.Get("X-Request-Id"); got != "local-request" {
		t.Fatalf("unrelated response header = %q, want local-request", got)
	}
}

func TestScrubCopilotResponseBodyRemovesUsage(t *testing.T) {
	body, err := newCopilotResponseBody(io.NopCloser(strings.NewReader(`{"id":"resp","copilot_usage":{"nano_aiu":1},"result":{"copilot_usage":{"nano_aiu":2},"text":"ok"}}`)), false)
	require.NoError(t, err)
	defer body.Close()

	got, err := io.ReadAll(body)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"resp","result":{"text":"ok"}}`, string(got))
}

func TestScrubCopilotResponseStreamRemovesUsage(t *testing.T) {
	input := "event: message\ndata: {\"id\":\"resp\",\"copilot_usage\":{\"nano_aiu\":1},\"text\":\"ok\"}\n\ndata: [DONE]\n"
	body, err := newCopilotResponseBody(io.NopCloser(strings.NewReader(input)), true)
	require.NoError(t, err)
	defer body.Close()

	got, err := io.ReadAll(body)
	require.NoError(t, err)
	if strings.Contains(string(got), "copilot_usage") || strings.Contains(string(got), "nano_aiu") {
		t.Fatalf("Copilot usage leaked from stream: %s", got)
	}
	if !strings.Contains(string(got), "\"text\":\"ok\"") || !strings.Contains(string(got), "[DONE]") {
		t.Fatalf("stream content was not preserved: %s", got)
	}
}

func TestDoResponseScrubsCopilotMetadataBeforeDelegating(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := chatInfo()
	info.ChannelMeta.ChannelType = constant.ChannelTypeCopilot
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":                 []string{"application/json"},
			"X-GitHub-Request-Id":          []string{"github-request"},
			"X-GitHub-Copilot-Request-Te":  []string{"true"},
			"X-Copilot-Service-Request-Id": []string{"service-request"},
		},
		Body: io.NopCloser(strings.NewReader(`{"id":"resp","object":"chat.completion","model":"model","choices":[],"copilot_usage":{"nano_aiu":1}}`)),
	}

	_, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	for _, name := range []string{
		"x-github-request-id",
		"x-github-copilot-request-te",
		"x-copilot-service-request-id",
	} {
		require.Empty(t, recorder.Header().Get(name), "%s should not be returned", name)
	}
	require.NotContains(t, recorder.Body.String(), "copilot_usage")
	require.NotContains(t, recorder.Body.String(), "nano_aiu")
}
