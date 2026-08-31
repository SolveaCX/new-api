package service

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// Claude uses YYYYMMDD; OpenAI search and search preview use YYYY_MM_DD.
var serverWebToolVersion = regexp.MustCompile(`^(web_(search|fetch)_[0-9]{8}|web_search(_preview)?_[0-9]{4}_[0-9]{2}_[0-9]{2})$`)

// RequestRequiresServerWebTools inspects only top-level tool declarations. CC's
// client-side WebSearch/WebFetch and MCP functions must remain ordinary tools:
// CC sends a separate request with a versioned server tool when executing search.
// The request-scoped cache avoids decoding large bodies once per candidate and
// has no cross-node state. The original body and tool parameters remain intact.
func RequestRequiresServerWebTools(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil || c.Request.Method != http.MethodPost {
		return false
	}
	switch normalizePlaygroundRelayPath(c.Request.URL.Path) {
	case "/v1/messages", "/v1/chat/completions", "/v1/responses":
	default:
		return false
	}
	if cached, ok := common.GetContextKeyType[bool](c, constant.ContextKeyRequestServerWebTools); ok {
		return cached
	}
	required := false
	defer func() { common.SetContextKey(c, constant.ContextKeyRequestServerWebTools, required) }()
	if c.Request.Body == nil {
		return false
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return false // Existing request validation reports body read/parse errors.
	}
	defer func() {
		_, _ = storage.Seek(0, io.SeekStart)
		c.Request.Body = io.NopCloser(storage)
	}()
	var request struct {
		Tools []struct {
			Type string `json:"type"`
		} `json:"tools"`
		WebSearchOptions json.RawMessage `json:"web_search_options"`
	}
	if err := common.DecodeJson(storage, &request); err != nil {
		return false
	}
	options := bytes.TrimSpace(request.WebSearchOptions)
	if len(options) > 0 && options[0] == '{' {
		required = true
	}
	for _, tool := range request.Tools {
		if serverWebToolVersion.MatchString(tool.Type) ||
			tool.Type == "web_search" || tool.Type == "web_search_preview" {
			required = true
			break
		}
	}
	return required
}

// ChannelSupportsServerWebTools is a negative capability rule for Copilot,
// not a claim that every other provider implements every server tool version.
func ChannelSupportsServerWebTools(c *gin.Context, channel *model.Channel) bool {
	return channel == nil || channel.Type != constant.ChannelTypeCopilot || !RequestRequiresServerWebTools(c)
}
