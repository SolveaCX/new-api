package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// Streamable-HTTP MCP servers answer either with a plain JSON body or with an
// SSE stream of `data:` lines. Both are legal per the spec, so accept either.
func parseMcpBody(raw []byte) (map[string]any, error) {
	payload := raw
	if bytes.Contains(raw, []byte("data:")) {
		var b strings.Builder
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(line, "data:") {
				b.WriteString(strings.TrimLeft(strings.TrimPrefix(line, "data:"), " "))
			}
		}
		if b.Len() > 0 {
			payload = []byte(b.String())
		}
	}
	var env map[string]any
	if err := common.Unmarshal(payload, &env); err != nil {
		return nil, fmt.Errorf("mcp upstream returned unparseable body: %s", truncateToolText(string(raw), 200))
	}
	return env, nil
}

// Stateful MCP servers (anything on the Java/Spring SDK, including our own
// open.voc.ai) reject a bare tools/call: every request has to carry the
// mcp-session-id issued by initialize. Sessions are cached per endpoint so the
// handshake is paid once rather than on every call.
//
// Production runs multiple nodes; this cache is deliberately per-process. A
// session is cheap to re-establish and sharing one across nodes would need
// coordination that buys nothing.
type mcpSessionEntry struct {
	id string
	at time.Time
}

var (
	mcpSessionMu sync.Mutex
	mcpSessions  = map[string]mcpSessionEntry{}
)

const mcpSessionTTL = 10 * time.Minute

func mcpCachedSession(url string) (string, bool) {
	mcpSessionMu.Lock()
	defer mcpSessionMu.Unlock()
	e, ok := mcpSessions[url]
	if !ok || time.Since(e.at) > mcpSessionTTL {
		return "", false
	}
	return e.id, true
}

func mcpStoreSession(url, id string) {
	mcpSessionMu.Lock()
	defer mcpSessionMu.Unlock()
	mcpSessions[url] = mcpSessionEntry{id: id, at: time.Now()}
}

func mcpDropSession(url string) {
	mcpSessionMu.Lock()
	defer mcpSessionMu.Unlock()
	delete(mcpSessions, url)
}

func mcpClient() *http.Client {
	client, err := requireHttpClient()
	if err != nil {
		return &http.Client{Timeout: toolUpstreamTimeout()}
	}
	return client
}

func mcpPost(ctx context.Context, url string, header http.Header, payload any) (int, []byte, http.Header, error) {
	buf, err := common.Marshal(payload)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("encode mcp request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header = header.Clone()
	req.Header.Set("Content-Type", "application/json")

	resp, err := mcpClient().Do(req)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("mcp upstream unreachable: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return resp.StatusCode, nil, resp.Header, fmt.Errorf("read mcp upstream: %w", err)
	}
	return resp.StatusCode, raw, resp.Header, nil
}

// mcpOpenSession performs the initialize handshake. An empty id with no error
// means the server is stateless, which is also valid — the caller then just
// omits the header.
func mcpOpenSession(ctx context.Context, url string, header http.Header) (string, error) {
	if id, ok := mcpCachedSession(url); ok {
		return id, nil
	}

	status, raw, respHeader, err := mcpPost(ctx, url, header, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "new-api", "version": "1"},
		},
	})
	if err != nil {
		return "", err
	}
	if status >= 400 {
		return "", fmt.Errorf("mcp initialize failed %d: %s", status, truncateToolText(string(raw), 200))
	}

	id := respHeader.Get("mcp-session-id")
	if id == "" {
		return "", nil
	}

	// The spec requires this notification before the session accepts calls.
	// Best effort: a server that ignores it still answers tools/call.
	notifyHeader := header.Clone()
	notifyHeader.Set("mcp-session-id", id)
	_, _, _, _ = mcpPost(ctx, url, notifyHeader, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})

	mcpStoreSession(url, id)
	return id, nil
}

// execToolMCP calls one tool on an upstream MCP server over JSON-RPC. MCP
// returns results as a content array of text parts; JSON encoded inside those
// parts is decoded back out so callers see structured data either way.
func execToolMCP(ctx context.Context, spec *ToolSpec, input map[string]any) (any, error) {
	a := spec.Adapter
	if a.URL == "" {
		return nil, fmt.Errorf("mcp adapter for %q has no url", spec.Id)
	}
	toolName := a.ToolName
	if toolName == "" {
		toolName = a.Tool
	}
	if toolName == "" {
		return nil, fmt.Errorf("mcp adapter for %q has no toolName", spec.Id)
	}

	header := http.Header{}
	header.Set("Accept", "application/json, text/event-stream")
	for k, v := range a.Headers {
		header.Set(k, v)
	}

	if a.Auth != nil && a.Auth.Type != "" && a.Auth.Type != "none" {
		secret := os.Getenv(a.Auth.EnvKey)
		if secret == "" {
			return nil, &ToolMissingCredentialError{EnvKey: a.Auth.EnvKey}
		}
		switch a.Auth.Type {
		case "bearer":
			header.Set("Authorization", "Bearer "+secret)
		default:
			name := a.Auth.Name
			if name == "" {
				name = "X-API-Key"
			}
			header.Set(name, secret)
		}
	}

	call := func(sessionID string) (int, []byte, error) {
		h := header.Clone()
		if sessionID != "" {
			h.Set("mcp-session-id", sessionID)
		}
		status, raw, _, err := mcpPost(ctx, a.URL, h, map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params":  map[string]any{"name": toolName, "arguments": input},
		})
		return status, raw, err
	}

	sessionID, err := mcpOpenSession(ctx, a.URL, header)
	if err != nil {
		return nil, err
	}
	status, raw, err := call(sessionID)
	if err != nil {
		return nil, err
	}
	// An expired session shows up as a 4xx; re-handshake once before failing.
	if status >= 400 && sessionID != "" {
		mcpDropSession(a.URL)
		if sessionID, err = mcpOpenSession(ctx, a.URL, header); err != nil {
			return nil, err
		}
		if status, raw, err = call(sessionID); err != nil {
			return nil, err
		}
	}
	if status >= 400 {
		return nil, fmt.Errorf("mcp upstream %d: %s", status, truncateToolText(string(raw), 300))
	}

	env, err := parseMcpBody(raw)
	if err != nil {
		return nil, err
	}
	if e, ok := env["error"].(map[string]any); ok {
		return nil, fmt.Errorf("mcp error: %v", e["message"])
	}

	result, _ := env["result"].(map[string]any)
	if result == nil {
		return nil, fmt.Errorf("mcp upstream returned no result: %s", truncateToolText(string(raw), 200))
	}
	// isError means the tool ran and reported failure — surface it as an error
	// so the caller is not charged for a result that is really a message.
	if isErr, _ := result["isError"].(bool); isErr {
		return nil, fmt.Errorf("%s", truncateToolText(mcpFirstText(result), 300))
	}
	if sc, ok := result["structuredContent"]; ok && sc != nil {
		return pickToolPath(sc, a.ResultPath), nil
	}

	texts := mcpTexts(result)
	if len(texts) == 1 {
		var parsed any
		if err := common.Unmarshal([]byte(texts[0]), &parsed); err == nil {
			return pickToolPath(parsed, a.ResultPath), nil
		}
		return texts[0], nil
	}
	if len(texts) > 1 {
		return texts, nil
	}
	return pickToolPath(result, a.ResultPath), nil
}

func mcpTexts(result map[string]any) []string {
	parts, _ := result["content"].([]any)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		part, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := part["type"].(string); t != "text" {
			continue
		}
		if s, _ := part["text"].(string); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func mcpFirstText(result map[string]any) string {
	if texts := mcpTexts(result); len(texts) > 0 {
		return texts[0]
	}
	return "upstream tool reported an error"
}
