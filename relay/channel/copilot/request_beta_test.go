package copilot

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestFinalizeRequestClaudeBetaAllowlist(t *testing.T) {
	for _, tc := range []struct {
		name        string
		body        string
		wantContext bool
	}{
		{"absent", `{"model":"claude-sonnet-4","messages":[]}`, false},
		{"context editing", `{"context_management":{"edits":[{"type":"clear_tool_uses_20250919"}]}}`, true},
		{"empty object", `{"context_management":{}}`, true},
		{"explicit null", `{"context_management":null}`, true},
		{"nested field is not a request option", `{"metadata":{"context_management":{}}}`, false},
		{"text is not a request option", `{"messages":[{"role":"user","content":"context_management"}]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := chatInfo()
			info.RelayFormat = types.RelayFormatClaude
			req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(tc.body))
			req.Header.Add("anthropic-beta", "files-api-2025-04-14,unknown-beta")
			req.Header.Add("anthropic-beta", "context-management-2025-06-27")
			length := req.ContentLength

			require.NoError(t, (&Adaptor{}).FinalizeRequest(nil, req, info))
			want := []string{"files-api-2025-04-14,unknown-beta", "context-management-2025-06-27"}
			if tc.wantContext {
				want = []string{"context-management-2025-06-27"}
			}
			require.Equal(t, want, req.Header.Values("anthropic-beta"))
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			require.Equal(t, tc.body, string(body))
			require.Equal(t, length, req.ContentLength)
		})
	}
}

func TestFinalizeRequestDoesNotChangeChatCompletions(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader(`{"context_management":{}}`))
	body := req.Body
	require.NoError(t, (&Adaptor{}).FinalizeRequest(nil, req, chatInfo()))
	require.True(t, body == req.Body)
	require.Empty(t, req.Header.Get("anthropic-beta"))
}

func TestFinalizeRequestRejectsUnreadableOrInvalidBodyBeforeSend(t *testing.T) {
	for _, body := range []io.Reader{
		strings.NewReader(`{"context_management":`),
		betaFailingReader{},
	} {
		info := chatInfo()
		info.RelayFormat = types.RelayFormatClaude
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", body)
		err := (&Adaptor{}).FinalizeRequest(nil, req, info)
		require.Error(t, err)
		require.True(t, channel.IsDefinitelyNotSent(err))
	}
}

type betaFailingReader struct{}

func (betaFailingReader) Read([]byte) (int, error) {
	return 0, errors.New("test read failure")
}

// Keep the real Copilot header/finalization pipeline, replacing only the URL
// with a local upstream so no credential or request reaches GitHub in tests.
type localBetaAdaptor struct {
	Adaptor
	url string
}

func (a *localBetaAdaptor) GetRequestURL(*relaycommon.RelayInfo) (string, error) {
	return a.url + "/v1/messages", nil
}

func TestCopilotBetaHeadersReachLocalUpstream(t *testing.T) {
	service.InitHttpClient()
	for _, stream := range []bool{false, true} {
		for _, withContext := range []bool{false, true} {
			name := "json"
			if stream {
				name = "stream"
			}
			if withContext {
				name += "/context-added-to-final-body"
			} else {
				name += "/context-removed-from-final-body"
			}
			for _, clientBeta := range [][]string{nil, {"interleaved-thinking-2025-05-14", "compact-2026-01-12,unknown-beta"}} {
				headerCase := "/without-beta"
				if len(clientBeta) > 0 {
					headerCase = "/with-beta"
				}
				t.Run(name+headerCase, func(t *testing.T) {
					body := `{"model":"claude-sonnet-4","messages":[]}`
					wantBeta := clientBeta
					if withContext {
						body = `{"model":"claude-sonnet-4","messages":[],"context_management":{"edits":[]}}`
						wantBeta = []string{"context-management-2025-06-27"}
					}
					type capturedRequest struct {
						header http.Header
						body   string
						length int64
						err    error
					}
					captured := make(chan capturedRequest, 1)
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						data, err := io.ReadAll(r.Body)
						captured <- capturedRequest{r.Header.Clone(), string(data), r.ContentLength, err}
						if stream {
							w.Header().Set("Content-Type", "text/event-stream")
							_, _ = io.WriteString(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
						} else {
							w.Header().Set("Content-Type", "application/json")
							_, _ = io.WriteString(w, `{"type":"message","content":[]}`)
						}
					}))
					defer server.Close()
					t.Logf("local upstream: %s/v1/messages", server.URL)

					// Intentionally make the parsed/inbound request disagree with the
					// final body, as can happen after parameter overrides.
					original := &dto.ClaudeRequest{Model: "claude-sonnet-4"}
					if !withContext {
						original.ContextManagement = []byte(`{"edits":[]}`)
					}
					inbound, err := common.Marshal(original)
					require.NoError(t, err)
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(string(inbound)))
					c.Request.Header.Set("Content-Type", "application/json")
					for _, value := range clientBeta {
						c.Request.Header.Add("anthropic-beta", value)
					}
					info := chatInfo()
					info.ApiType = constant.APITypeCopilot
					info.ChannelType = constant.ChannelTypeCopilot
					info.ChannelSetting.Proxy = ""
					info.RelayFormat = types.RelayFormatClaude
					info.IsStream = stream
					info.Request = original
					info.UpstreamRequestBodySize = int64(len(body))
					info.HeadersOverride = map[string]any{"anthropic-beta": "skills-2025-10-02"}

					adaptor := &localBetaAdaptor{url: server.URL}
					// ReaderOnly matches body-storage/pass-through requests, for which
					// http.NewRequest cannot set GetBody or infer ContentLength.
					resp, err := channel.DoApiRequest(adaptor, c, info, common.ReaderOnly(strings.NewReader(body)))
					require.NoError(t, err)
					defer resp.Body.Close()
					require.Equal(t, http.StatusOK, resp.StatusCode)
					got := <-captured
					require.NoError(t, got.err)
					require.Equal(t, wantBeta, got.header.Values("anthropic-beta"))
					require.Equal(t, clientBeta, c.Request.Header.Values("anthropic-beta"))
					require.Equal(t, body, got.body)
					require.Equal(t, int64(len(body)), got.length)
					wantAccept := "application/json"
					if stream {
						wantAccept = "text/event-stream"
					}
					require.Equal(t, wantAccept, got.header.Get("Accept"))
				})
			}
		}
	}
}
