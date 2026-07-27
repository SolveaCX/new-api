package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

var (
	ErrToolUnknown    = errors.New("unknown tool")
	ErrToolValidation = errors.New("invalid tool input")
)

// ToolMissingCredentialError names the environment variable that would unblock
// the call, so an operator is told exactly what to configure rather than being
// handed a generic upstream failure.
type ToolMissingCredentialError struct{ EnvKey string }

func (e *ToolMissingCredentialError) Error() string {
	return "missing upstream credential: " + e.EnvKey
}

// ToolResult is what one execution produced. Charging is decided by the caller
// so that a failed call can never cost anything.
type ToolResult struct {
	ToolId      string `json:"tool"`
	OK          bool   `json:"ok"`
	Output      any    `json:"output,omitempty"`
	Error       string `json:"error,omitempty"`
	MissingEnv  string `json:"missing_credential,omitempty"`
	ResultCount int    `json:"result_count"`
	LatencyMs   int    `json:"latency_ms"`
}

func toolUpstreamTimeout() time.Duration {
	if v := os.Getenv("TOOL_UPSTREAM_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 30 * time.Second
}

// RunTool validates the input, dispatches to the adapter and reports what came
// back. It never touches quota: keeping the charge decision outside this
// function is what makes "a failure is free" hard to get wrong.
func RunTool(ctx context.Context, spec *ToolSpec, raw map[string]any) *ToolResult {
	input, err := ValidateToolInput(spec, raw)
	if err != nil {
		return &ToolResult{ToolId: spec.Id, Error: err.Error()}
	}

	ctx, cancel := context.WithTimeout(ctx, toolUpstreamTimeout())
	defer cancel()

	started := time.Now()
	out, err := dispatchTool(ctx, spec, input)
	res := &ToolResult{ToolId: spec.Id, LatencyMs: int(time.Since(started).Milliseconds())}

	if err != nil {
		var mc *ToolMissingCredentialError
		if errors.As(err, &mc) {
			res.MissingEnv = mc.EnvKey
		}
		res.Error = err.Error()
		return res
	}

	res.OK = true
	res.Output = out
	res.ResultCount = countToolResults(out)
	return res
}

func dispatchTool(ctx context.Context, spec *ToolSpec, input map[string]any) (any, error) {
	switch spec.Adapter.Kind {
	case "http":
		return execToolHTTP(ctx, spec, input)
	case "mcp":
		return execToolMCP(ctx, spec, input)
	default:
		// monid / deepline / waterfall land here until their executors ship.
		// A clear error beats pretending the call was made.
		return nil, fmt.Errorf("tool adapter %q is not wired up yet", spec.Adapter.Kind)
	}
}

func execToolHTTP(ctx context.Context, spec *ToolSpec, input map[string]any) (any, error) {
	a := spec.Adapter

	u, err := url.Parse(templateToolStr(a.URL, input))
	if err != nil {
		return nil, fmt.Errorf("bad upstream url: %w", err)
	}

	q := u.Query()
	for k, tpl := range a.Query {
		if v := templateToolStr(tpl, input); v != "" {
			q.Set(k, v)
		}
	}

	header := http.Header{}
	header.Set("Accept", "application/json")
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
		case "header":
			name := a.Auth.Name
			if name == "" {
				name = "Authorization"
			}
			header.Set(name, secret)
		case "query":
			name := a.Auth.Name
			if name == "" {
				name = "api_key"
			}
			q.Set(name, secret)
		}
	}
	u.RawQuery = q.Encode()

	method := strings.ToUpper(a.Method)
	if method == "" {
		method = http.MethodGet
	}

	var body io.Reader
	if method == http.MethodPost {
		payload := templateToolAny(a.Body, input)
		buf, err := common.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode upstream body: %w", err)
		}
		body = bytes.NewReader(buf)
		header.Set("Content-Type", "application/json")
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header = header

	// requireHttpClient rather than GetHttpClient: the shared client is built
	// during init, so anything running before that (tests, early startup) would
	// otherwise dereference nil.
	client, err := requireHttpClient()
	if err != nil {
		client = &http.Client{Timeout: toolUpstreamTimeout()}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream unreachable: %w", err)
	}
	defer resp.Body.Close()

	// Cap the read so one pathological upstream cannot exhaust memory.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read upstream: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, truncateToolText(string(raw), 300))
	}

	var parsed any
	if err := common.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("upstream returned non-JSON: %s", truncateToolText(string(raw), 200))
	}
	return pickToolPath(parsed, a.ResultPath), nil
}

// pickToolPath unwraps a response along resultPath. An unresolvable path yields
// the whole body rather than nothing: an upstream that wraps most endpoints in
// "data" still answers a few utility endpoints at the top level, and failing
// those would be a mapping opinion, not a real error.
func pickToolPath(v any, path string) any {
	if path == "" {
		return v
	}
	cur := v
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return v
		}
		next, ok := m[part]
		if !ok {
			return v
		}
		cur = next
	}
	return cur
}

// toolListKeys are the wrappers upstreams commonly put a result list behind.
// "reviews" is here because the VOC Amazon endpoints name their array that;
// without it a 50-review response is logged as a single result.
var toolListKeys = []string{"results", "items", "hits", "data", "records", "rows", "matches", "output", "reviews"}

// countToolResults drives per-result pricing and pay-on-match, so an empty
// envelope must count as zero: upstreams routinely answer "nothing found" with
// {"count":0,"results":[]}, which is a non-empty object and would otherwise be
// billed as one hit.
func countToolResults(v any) int {
	switch t := v.(type) {
	case nil:
		return 0
	case []any:
		return len(t)
	case map[string]any:
		if len(t) == 0 {
			return 0
		}
		for _, k := range toolListKeys {
			if arr, ok := t[k].([]any); ok {
				return len(arr)
			}
		}
		if n, ok := t["count"].(float64); ok {
			return int(n)
		}
		if n, ok := t["total"].(float64); ok {
			return int(n)
		}
		if found, ok := t["found"].(bool); ok && !found {
			return 0
		}
		if s, ok := t["status"].(string); ok && s == "not_found" {
			return 0
		}
		if _, bad := t["error"]; bad {
			return 0
		}
		return 1
	default:
		return 1
	}
}

var toolPlaceholder = regexp.MustCompile(`\{\{(\w+)\}\}`)

func templateToolStr(tpl string, input map[string]any) string {
	return toolPlaceholder.ReplaceAllStringFunc(tpl, func(m string) string {
		key := toolPlaceholder.FindStringSubmatch(m)[1]
		v, ok := input[key]
		if !ok || v == nil {
			return ""
		}
		return fmt.Sprint(v)
	})
}

// templateToolAny substitutes into a JSON body. A value that is exactly one
// placeholder keeps its original type, so a number stays a number.
func templateToolAny(v any, input map[string]any) any {
	switch t := v.(type) {
	case string:
		if m := toolPlaceholder.FindStringSubmatch(t); m != nil && m[0] == t {
			return input[m[1]]
		}
		return templateToolStr(t, input)
	case []any:
		out := make([]any, 0, len(t))
		for _, e := range t {
			out = append(out, templateToolAny(e, input))
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, e := range t {
			tv := templateToolAny(e, input)
			if tv == nil || tv == "" {
				continue
			}
			out[k] = tv
		}
		return out
	default:
		return v
	}
}

// ValidateToolInput coerces raw input against the spec's schema, fills defaults
// and rejects anything the upstream would reject anyway — cheaper to catch here
// than to pay for a 400 round trip.
func ValidateToolInput(spec *ToolSpec, raw map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(spec.Input.Properties))

	for key, field := range spec.Input.Properties {
		v, present := raw[key]
		if !present || v == nil || v == "" {
			if field.Default == nil {
				continue
			}
			v = field.Default
		}

		switch field.Type {
		case "number":
			n, err := toToolFloat(v)
			if err != nil {
				return nil, fmt.Errorf("%w: %s must be a number", ErrToolValidation, key)
			}
			v = n
		case "boolean":
			v = toToolBool(v)
		case "array":
			if s, ok := v.(string); ok {
				parts := strings.Split(s, ",")
				arr := make([]any, 0, len(parts))
				for _, p := range parts {
					if p = strings.TrimSpace(p); p != "" {
						arr = append(arr, p)
					}
				}
				v = arr
			}
		}

		if len(field.Enum) > 0 && !containsToolStr(field.Enum, fmt.Sprint(v)) {
			return nil, fmt.Errorf("%w: %s must be one of: %s",
				ErrToolValidation, key, strings.Join(field.Enum, ", "))
		}
		out[key] = v
	}

	for _, req := range spec.Input.Required {
		if _, ok := out[req]; !ok {
			return nil, fmt.Errorf("%w: missing required field: %s", ErrToolValidation, req)
		}
	}
	return out, nil
}

func toToolFloat(v any) (float64, error) {
	switch t := v.(type) {
	case float64:
		return t, nil
	case float32:
		return float64(t), nil
	case int:
		return float64(t), nil
	case int64:
		return float64(t), nil
	case string:
		return strconv.ParseFloat(strings.TrimSpace(t), 64)
	default:
		return 0, fmt.Errorf("not a number")
	}
}

func toToolBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1"
	case float64:
		return t != 0
	default:
		return false
	}
}

func truncateToolText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
