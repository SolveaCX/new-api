package copilot

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// FinalizeRequest sets the context-management beta only when the final outbound
// body contains that field; otherwise existing beta headers are preserved.
func (a *Adaptor) FinalizeRequest(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	if info == nil || info.RelayFormat != types.RelayFormatClaude {
		return nil
	}

	var payload struct {
		ContextManagement json.RawMessage `json:"context_management"`
	}
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return channel.MarkDefinitelyNotSent(err)
		}
		// Only inspect the payload; keep the exact bytes and ContentLength.
		req.Body = io.NopCloser(bytes.NewReader(body))
		if err := common.Unmarshal(body, &payload); err != nil {
			return channel.MarkDefinitelyNotSent(err)
		}
	}

	if len(payload.ContextManagement) > 0 {
		req.Header.Set("anthropic-beta", "context-management-2025-06-27")
	} else if len(req.Header.Values("anthropic-beta")) == 0 && c != nil && c.Request != nil {
		// The shared request pipeline does not copy this client header.
		// Preserve all original values without changing the inbound request.
		for _, value := range c.Request.Header.Values("anthropic-beta") {
			req.Header.Add("anthropic-beta", value)
		}
	}
	return nil
}
