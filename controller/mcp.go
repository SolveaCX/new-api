package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// MCP server endpoint — the agent-facing face of the tool marketplace.
//
// It deliberately exposes three tools and not 1,053. An agent that has to load
// every endpoint definition to decide which one to call spends its whole
// context on the catalogue; discover → inspect → run keeps that cost flat no
// matter how large the catalogue grows.
//
// Transport is Streamable HTTP, stateless: no mcp-session-id is issued, so any
// node in the fleet can serve any request. Authentication is the caller's
// normal API key, the same one that buys model tokens.

const (
	mcpProtocolVersion = "2025-06-18"
	mcpServerName      = "flatkey"
)

// JSON-RPC error codes used here. -32000 is the generic implementation-defined
// server error the spec reserves for applications.
const (
	mcpErrParse          = -32700
	mcpErrInvalidRequest = -32600
	mcpErrMethodNotFound = -32601
	mcpErrInvalidParams  = -32602
)

type mcpRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	Id      any            `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params"`
}

func mcpResult(c *gin.Context, id any, result any) {
	c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "id": id, "result": result})
}

func mcpError(c *gin.Context, id any, code int, message string) {
	// Transport stays 200: a JSON-RPC error is a valid response, and clients
	// read the body, not the status.
	c.JSON(http.StatusOK, gin.H{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   gin.H{"code": code, "message": message},
	})
}

// mcpToolText wraps a plain string result.
func mcpToolText(c *gin.Context, id any, text string) {
	mcpResult(c, id, gin.H{"content": []gin.H{{"type": "text", "text": text}}})
}

// mcpToolData returns both a text rendering and structuredContent. Clients that
// understand structured output get typed data; the rest still get the JSON as
// text, so no client is left with an empty result.
func mcpToolData(c *gin.Context, id any, data any) {
	encoded, err := common.Marshal(data)
	if err != nil {
		mcpToolFailure(c, id, "could not encode result: "+err.Error())
		return
	}
	mcpResult(c, id, gin.H{
		"content":           []gin.H{{"type": "text", "text": string(encoded)}},
		"structuredContent": data,
	})
}

// mcpToolFailure reports a *tool* failure. Per MCP this is a successful
// JSON-RPC response carrying isError, not a protocol error — the call reached
// the tool and the tool has something to say, which the model should see and
// can act on.
func mcpToolFailure(c *gin.Context, id any, message string) {
	mcpResult(c, id, gin.H{
		"isError": true,
		"content": []gin.H{{"type": "text", "text": message}},
	})
}

// HandleMCP serves one JSON-RPC message.
func HandleMCP(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 4<<20))
	if err != nil {
		mcpError(c, nil, mcpErrParse, "could not read request body")
		return
	}

	// Batching was removed in protocol 2025-06-18. Say so rather than failing
	// with an opaque parse error.
	if trimmed := strings.TrimSpace(string(raw)); strings.HasPrefix(trimmed, "[") {
		mcpError(c, nil, mcpErrInvalidRequest, "batch requests are not supported in protocol "+mcpProtocolVersion)
		return
	}

	var req mcpRequest
	if err := common.Unmarshal(raw, &req); err != nil {
		mcpError(c, nil, mcpErrParse, "invalid JSON-RPC body")
		return
	}

	// A message without an id is a notification: acknowledge with no body.
	// Returning a response to one is a protocol violation.
	if req.Id == nil {
		c.Status(http.StatusAccepted)
		return
	}

	switch req.Method {
	case "initialize":
		mcpResult(c, req.Id, gin.H{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":    gin.H{"tools": gin.H{"listChanged": false}},
			"serverInfo":      gin.H{"name": mcpServerName, "version": common.Version},
			"instructions": "Call discover first with a natural-language description of the data you need, " +
				"then inspect the tool id it returns to read the exact input schema and price, then run it. " +
				"Never guess a tool id.",
		})
	case "ping":
		mcpResult(c, req.Id, gin.H{})
	case "tools/list":
		mcpResult(c, req.Id, gin.H{"tools": mcpToolDefinitions()})
	case "tools/call":
		handleMCPToolCall(c, req)
	default:
		mcpError(c, req.Id, mcpErrMethodNotFound, "unknown method: "+req.Method)
	}
}

func mcpToolDefinitions() []gin.H {
	reg, _ := service.GetToolRegistry()
	return []gin.H{
		{
			"name": "discover",
			"description": fmt.Sprintf(
				"Search %d data endpoints across TikTok, Instagram, Amazon reviews, YouTube, Reddit, LinkedIn and more, "+
					"using a natural-language description of what you need. Returns candidate tool ids ranked by relevance. "+
					"Always start here — tool ids are not guessable.", reg.Len()),
			"inputSchema": gin.H{
				"type": "object",
				"properties": gin.H{
					"query": gin.H{
						"type":        "string",
						"description": "What you want, in plain language. Example: 'trending TikTok hashtags in the US'.",
					},
					"limit": gin.H{
						"type":        "integer",
						"description": "How many candidates to return (default 10, max 50).",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			"name": "inspect",
			"description": "Read one endpoint's exact input schema, price per call and settlement before running it. " +
				"Run without inspecting only when you already know the schema.",
			"inputSchema": gin.H{
				"type": "object",
				"properties": gin.H{
					"id": gin.H{
						"type":        "string",
						"description": "Tool id returned by discover, e.g. 'tikhub.tiktok.get_trends_hashtag_list'.",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			"name": "run",
			"description": "Execute an endpoint and return its data. Charged against the same balance as your model calls; " +
				"a failed call is free. Requires a Pro plan or above.",
			"inputSchema": gin.H{
				"type": "object",
				"properties": gin.H{
					"id": gin.H{
						"type":        "string",
						"description": "Tool id to execute.",
					},
					"input": gin.H{
						"type":        "object",
						"description": "Arguments matching the schema returned by inspect.",
					},
				},
				"required": []string{"id"},
			},
		},
	}
}

func handleMCPToolCall(c *gin.Context, req mcpRequest) {
	name, _ := req.Params["name"].(string)
	args, _ := req.Params["arguments"].(map[string]any)
	if args == nil {
		args = map[string]any{}
	}

	switch name {
	case "discover":
		mcpDiscover(c, req.Id, args)
	case "inspect":
		mcpInspect(c, req.Id, args)
	case "run":
		mcpRun(c, req.Id, args)
	default:
		mcpError(c, req.Id, mcpErrInvalidParams, "unknown tool: "+name)
	}
}

func mcpDiscover(c *gin.Context, id any, args map[string]any) {
	query, _ := args["query"].(string)
	query = strings.TrimSpace(query)
	if query == "" || len(query) > 1000 {
		mcpToolFailure(c, id, "query must be 1-1000 characters")
		return
	}

	limit := 10
	if n, ok := toMcpInt(args["limit"]); ok && n > 0 {
		limit = n
	}
	if limit > 50 {
		limit = 50
	}

	reg, _ := service.GetToolRegistry()
	hits := reg.Discover(query, limit, 0)
	results := make([]gin.H, 0, len(hits))
	for _, h := range hits {
		results = append(results, gin.H{
			"id":          h.Id,
			"name":        h.Name,
			"platform":    h.Platform,
			"description": h.Description,
			"score":       h.Score,
		})
	}
	mcpToolData(c, id, gin.H{"query": query, "count": len(results), "results": results})
}

func mcpInspect(c *gin.Context, id any, args map[string]any) {
	toolId, _ := args["id"].(string)
	if strings.TrimSpace(toolId) == "" {
		mcpToolFailure(c, id, "id is required")
		return
	}

	reg, _ := service.GetToolRegistry()
	spec, ok := reg.Get(toolId)
	if !ok {
		mcpToolFailure(c, id, "unknown tool: "+toolId+" — call discover to find a valid id")
		return
	}

	// The adapter block stays server-side: it carries upstream URLs and
	// credential names, which are ours and not the caller's.
	mcpToolData(c, id, gin.H{
		"id":          spec.Id,
		"name":        spec.Name,
		"platform":    spec.Platform(),
		"description": spec.Description,
		"input":       spec.Input,
		"pricing": gin.H{
			"price_usd":    spec.PriceUSD(1),
			"max_price":    spec.MaxPriceUSD(),
			"pay_on_match": spec.Pricing.PayOnMatch,
			"currency":     "USD",
		},
		"settlement":  dataToolSettlement(spec),
		"quarantined": reg.QuarantineReason(spec),
	})
}

// mcpRun mirrors RunDataTool exactly: same plan gate, same pre-flight quota
// check, same settlement. Two code paths to one wallet would eventually
// disagree, so the money logic lives in service and both callers use it.
func mcpRun(c *gin.Context, id any, args map[string]any) {
	userId := c.GetInt("id")
	if userId <= 0 {
		mcpToolFailure(c, id, "unauthorized")
		return
	}

	toolId, _ := args["id"].(string)
	if strings.TrimSpace(toolId) == "" {
		mcpToolFailure(c, id, "id is required")
		return
	}
	input, _ := args["input"].(map[string]any)
	if input == nil {
		input = map[string]any{}
	}

	access, err := model.CheckToolAccess(userId)
	if err != nil {
		mcpToolFailure(c, id, "could not resolve plan entitlement: "+err.Error())
		return
	}
	if !access.Allowed {
		mcpToolFailure(c, id, "Your current plan does not include tool calls. Upgrade to Pro or above to run endpoints.")
		return
	}

	reg, _ := service.GetToolRegistry()
	spec, ok := reg.Get(toolId)
	if !ok {
		mcpToolFailure(c, id, "unknown tool: "+toolId+" — call discover to find a valid id")
		return
	}
	if reason := reg.QuarantineReason(spec); reason != "" {
		mcpToolFailure(c, id, "tool is temporarily unavailable: "+reason)
		return
	}

	groupRatio := service.DataToolGroupRatio(c)
	if _, err := service.CheckDataToolQuota(userId, spec, groupRatio); err != nil {
		if errors.Is(err, service.ErrDataToolInsufficientQuota) {
			mcpToolFailure(c, id, err.Error())
			return
		}
		mcpToolFailure(c, id, "quota check failed: "+err.Error())
		return
	}

	res := service.RunTool(c.Request.Context(), spec, input)
	charge := service.SettleDataToolCall(
		c, userId, c.GetInt("token_id"), c.GetString("token_key"), spec, res, groupRatio)

	if !res.OK {
		msg := res.Error
		if res.MissingEnv != "" {
			msg = "this endpoint needs a credential that is not configured: " + res.MissingEnv
		}
		// Not charged — say so, so the model does not retry believing it paid.
		mcpToolFailure(c, id, fmt.Sprintf("%s (not charged)", msg))
		return
	}

	mcpToolData(c, id, gin.H{
		"tool":            res.ToolId,
		"output":          res.Output,
		"result_count":    res.ResultCount,
		"charged_usd":     charge.USD,
		"remaining_quota": charge.RemainingQuota,
		"latency_ms":      res.LatencyMs,
	})
}

func toMcpInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	}
	return 0, false
}
