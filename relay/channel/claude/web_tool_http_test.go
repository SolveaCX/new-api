package claude

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Verify the outgoing wire request through the real adaptor/HTTP transport.
// The upstream echoes the received body; this does not exercise live web search.
func TestChatNativeWebToolsReachClaudeHTTPUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if service.GetHttpClient() == nil {
		service.InitHttpClient()
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			http.Error(w, "unexpected upstream endpoint", http.StatusBadRequest)
			return
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			http.Error(w, "missing upstream version header", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.Copy(w, r.Body)
	}))
	defer upstream.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()

	router := gin.New()
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		fail := func(err error) {
			c.String(http.StatusInternalServerError, "%s", err.Error())
		}
		var request dto.GeneralOpenAIRequest
		if err := common.DecodeJson(c.Request.Body, &request); err != nil {
			fail(err)
			return
		}
		copied, err := common.DeepCopy(&request)
		if err != nil {
			fail(err)
			return
		}
		info := &relaycommon.RelayInfo{
			OriginModelName: request.Model,
			IsStream:        request.IsStream(c),
			DisablePing:     true,
			ChannelMeta:     &relaycommon.ChannelMeta{ChannelBaseUrl: upstream.URL},
		}
		adaptor := &Adaptor{}
		converted, err := adaptor.ConvertOpenAIRequest(c, info, copied)
		if err != nil {
			fail(err)
			return
		}
		body, err := common.Marshal(converted)
		if err != nil {
			fail(err)
			return
		}
		response, err := channel.DoApiRequest(adaptor, c, info, bytes.NewReader(body))
		if err != nil {
			fail(err)
			return
		}
		defer response.Body.Close()
		body, err = io.ReadAll(response.Body)
		if err != nil {
			fail(err)
			return
		}
		c.Data(response.StatusCode, "application/json", body)
	})
	gateway := httptest.NewServer(router)
	defer gateway.Close()
	t.Logf("local HTTP verification: %s/v1/chat/completions -> %s/v1/messages", gateway.URL, upstream.URL)

	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", stream), func(t *testing.T) {
			tools := `[{"type":"web_search_20250305","name":"web_search","allowed_domains":["weather.com.cn"],"max_uses":0}]`
			body := fmt.Sprintf(`{"model":"claude-opus-5","max_tokens":1024,"stream":%t,"tools":%s,"messages":[{"role":"user","content":"今天北京天气如何？请你搜索后回答"}]}`, stream, tools)
			response, err := client.Post(gateway.URL+"/v1/chat/completions", "application/json", strings.NewReader(body))
			require.NoError(t, err)
			defer response.Body.Close()
			wire, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode, string(wire))
			require.JSONEq(t, tools, gjson.GetBytes(wire, "tools").Raw)
			require.Equal(t, stream, gjson.GetBytes(wire, "stream").Bool())
			require.Equal(t, "claude-opus-5", gjson.GetBytes(wire, "model").String())
			require.Equal(t, int64(1024), gjson.GetBytes(wire, "max_tokens").Int())
		})
	}
}
