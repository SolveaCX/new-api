package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestParseMcpBodyAcceptsPlainJSONAndSSE(t *testing.T) {
	plain, err := parseMcpBody([]byte(`{"jsonrpc":"2.0","result":{"ok":true}}`))
	if err != nil {
		t.Fatalf("plain json: %v", err)
	}
	if plain["result"] == nil {
		t.Fatal("plain json lost the result")
	}

	// Streamable HTTP splits one JSON document across data: lines.
	sse := "event: message\ndata: {\"jsonrpc\":\"2.0\",\"result\":\n" +
		"data: {\"ok\":true}}\n\n"
	parsed, err := parseMcpBody([]byte(sse))
	if err != nil {
		t.Fatalf("sse: %v", err)
	}
	if parsed["result"] == nil {
		t.Fatal("sse lost the result")
	}

	if _, err := parseMcpBody([]byte("<html>not json</html>")); err == nil {
		t.Fatal("expected an error for a non-JSON body")
	}
}

// mcpTestServer answers initialize with a session id and then serves one
// tools/call reply, recording whether the session header came back.
func mcpTestServer(t *testing.T, reply string, status int) (*httptest.Server, *int32, *int32) {
	t.Helper()
	var calls, withSession int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		switch {
		case strings.Contains(string(body), `"initialize"`):
			w.Header().Set("mcp-session-id", "sess-1")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{}}`))
		case strings.Contains(string(body), `"notifications/initialized"`):
			w.WriteHeader(http.StatusAccepted)
		default:
			atomic.AddInt32(&calls, 1)
			if r.Header.Get("mcp-session-id") != "" {
				atomic.AddInt32(&withSession, 1)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(reply))
		}
	}))
	t.Cleanup(func() {
		srv.Close()
		mcpDropSession(srv.URL)
	})
	return srv, &calls, &withSession
}

func mcpSpec(url string) *ToolSpec {
	return &ToolSpec{
		Id:      "voc.test",
		Name:    "Test",
		Input:   ToolInputSchema{Type: "object", Properties: map[string]ToolField{}},
		Adapter: ToolAdapter{Kind: "mcp", URL: url, ToolName: "amazon_reviews_fetch_history"},
	}
}

func TestExecToolMCPUnwrapsJSONEncodedTextContent(t *testing.T) {
	// The common shape: a single text part whose body is itself JSON.
	reply := `{"jsonrpc":"2.0","result":{"content":[{"type":"text","text":"{\"reviews\":[{\"id\":1},{\"id\":2}]}"}]}}`
	srv, calls, withSession := mcpTestServer(t, reply, http.StatusOK)

	out, err := execToolMCP(context.Background(), mcpSpec(srv.URL), map[string]any{})
	if err != nil {
		t.Fatalf("execToolMCP: %v", err)
	}
	obj, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected decoded JSON, got %T", out)
	}
	if list, _ := obj["reviews"].([]any); len(list) != 2 {
		t.Fatalf("expected 2 reviews, got %#v", obj["reviews"])
	}
	// Result counting is what billing keys off; a wrong count charges wrongly.
	if n := countToolResults(out); n != 2 {
		t.Fatalf("expected result count 2, got %d", n)
	}
	if *calls != 1 {
		t.Fatalf("expected 1 tools/call, got %d", *calls)
	}
	if *withSession != 1 {
		t.Fatal("tools/call did not carry the mcp-session-id from initialize")
	}
}

func TestExecToolMCPPrefersStructuredContent(t *testing.T) {
	reply := `{"jsonrpc":"2.0","result":{"structuredContent":{"count":0,"results":[]},"content":[{"type":"text","text":"ignored"}]}}`
	srv, _, _ := mcpTestServer(t, reply, http.StatusOK)

	out, err := execToolMCP(context.Background(), mcpSpec(srv.URL), map[string]any{})
	if err != nil {
		t.Fatalf("execToolMCP: %v", err)
	}
	// An empty envelope must count as zero so pay-on-match does not charge.
	if n := countToolResults(out); n != 0 {
		t.Fatalf("expected empty envelope to count 0, got %d", n)
	}
}

func TestExecToolMCPSurfacesToolLevelError(t *testing.T) {
	// isError means the call reached the tool and the tool refused. Returning
	// it as success would bill the caller for an error message.
	reply := `{"jsonrpc":"2.0","result":{"isError":true,"content":[{"type":"text","text":"INSUFFICIENT_CREDITS"}]}}`
	srv, _, _ := mcpTestServer(t, reply, http.StatusOK)

	_, err := execToolMCP(context.Background(), mcpSpec(srv.URL), map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "INSUFFICIENT_CREDITS") {
		t.Fatalf("expected the upstream error surfaced, got %v", err)
	}
}

func TestExecToolMCPSurfacesJSONRPCError(t *testing.T) {
	reply := `{"jsonrpc":"2.0","error":{"code":-32602,"message":"unknown tool"}}`
	srv, _, _ := mcpTestServer(t, reply, http.StatusOK)

	_, err := execToolMCP(context.Background(), mcpSpec(srv.URL), map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Fatalf("expected the JSON-RPC error surfaced, got %v", err)
	}
}

func TestExecToolMCPRequiresCredentialBeforeCalling(t *testing.T) {
	srv, calls, _ := mcpTestServer(t, `{"jsonrpc":"2.0","result":{}}`, http.StatusOK)
	spec := mcpSpec(srv.URL)
	spec.Adapter.Auth = &ToolAuth{Type: "header", Name: "X-API-Key", EnvKey: "TOOL_TEST_MISSING_KEY"}

	_, err := execToolMCP(context.Background(), spec, map[string]any{})
	var missing *ToolMissingCredentialError
	if err == nil {
		t.Fatal("expected a missing-credential error")
	}
	if !errors.As(err, &missing) {
		t.Fatalf("expected ToolMissingCredentialError, got %v", err)
	}
	if *calls != 0 {
		t.Fatal("upstream was called despite the missing credential")
	}
}

func TestExecToolMCPRejectsSpecWithoutToolName(t *testing.T) {
	spec := mcpSpec("http://127.0.0.1:1")
	spec.Adapter.ToolName = ""
	if _, err := execToolMCP(context.Background(), spec, map[string]any{}); err == nil {
		t.Fatal("expected an error when toolName is absent")
	}
}
