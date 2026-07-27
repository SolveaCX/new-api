package controller

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// Data-tool marketplace endpoints — the "more tools" half of
// "one key. more models, more tools, less spend."
//
// The same key and the same quota that buy model tokens buy tool calls, so
// there is no separate wallet, plan or invoice for tools.

// GetDataToolCatalogue lists tools with their platform/category facets. This is
// the marketplace's primary read and is intentionally cheap: the registry is
// in memory and this only filters and slices it.
func GetDataToolCatalogue(c *gin.Context) {
	reg, _ := service.GetToolRegistry()

	q := strings.ToLower(strings.TrimSpace(c.Query("q")))
	platform := c.Query("platform")
	category := c.Query("category")
	mode := c.Query("mode")

	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 24
	}

	all := reg.All()
	matched := make([]gin.H, 0, pageSize)
	total := 0

	for _, s := range all {
		if platform != "" && platform != "all" && s.Platform() != platform {
			continue
		}
		if mode != "" && mode != "all" && string(s.Mode) != mode {
			continue
		}
		if category != "" && category != "all" && !hasDataToolCategory(s, category) {
			continue
		}
		if q != "" && !matchesDataToolQuery(s, q) {
			continue
		}
		total++
		// Collect only the requested page, but keep counting so the UI can
		// show an honest total rather than "24+".
		if total <= (page-1)*pageSize || len(matched) >= pageSize {
			continue
		}
		matched = append(matched, dataToolSummary(reg, s))
	}

	common.ApiSuccess(c, gin.H{
		"total":       len(all),
		"matched":     total,
		"page":        page,
		"page_size":   pageSize,
		"tools":       matched,
		"platforms":   reg.Platforms(),
		"categories":  reg.Categories(),
		"providers":   reg.Providers(),
		"margin_rate": service.ToolMinMarginRate,
		// Browsing the catalogue is open to everyone; running is gated. The UI
		// uses this to show an upsell instead of letting a Go-plan user click
		// Run and only then discover they cannot.
		"access": toolAccessFor(c),
	})
}

func dataToolSummary(reg *service.ToolRegistry, s *service.ToolSpec) gin.H {
	return gin.H{
		"id":          s.Id,
		"name":        s.Name,
		"provider":    s.Provider,
		"platform":    s.Platform(),
		"mode":        s.Mode,
		"categories":  s.Categories,
		"description": s.Description,
		"method":      s.Adapter.Method,
		"endpoint":    dataToolEndpointLabel(s),
		// price_usd is the real charge including the margin floor, not the raw
		// catalogue number, so what the card shows is what the caller pays.
		"price_usd":   s.PriceUSD(1),
		"free":        s.Pricing.Model == service.ToolFree,
		"pay_on_match": s.Pricing.PayOnMatch,
		"quarantined": reg.QuarantineReason(s) != "",
	}
}

// dataToolEndpointLabel exposes the upstream path for display only. Native
// adapters show their real path; federated ones show the tool id so we never
// leak an aggregator's internal routing.
func dataToolEndpointLabel(s *service.ToolSpec) string {
	if s.Adapter.Kind != "http" || s.Adapter.URL == "" {
		return s.Id
	}
	u := s.Adapter.URL
	if i := strings.Index(u, "://"); i >= 0 {
		if j := strings.Index(u[i+3:], "/"); j >= 0 {
			return u[i+3+j:]
		}
	}
	return s.Id
}

func hasDataToolCategory(s *service.ToolSpec, cat string) bool {
	for _, c := range s.Categories {
		if c == cat {
			return true
		}
	}
	return false
}

func matchesDataToolQuery(s *service.ToolSpec, q string) bool {
	return strings.Contains(strings.ToLower(s.Name), q) ||
		strings.Contains(strings.ToLower(s.Id), q) ||
		strings.Contains(strings.ToLower(s.Description), q) ||
		strings.Contains(strings.ToLower(s.Adapter.URL), q)
}

// SearchDataTools is the natural-language `discover` primitive. Exposing a
// search instead of thousands of tool definitions is what lets the catalogue
// grow without overrunning an agent's context window.
func SearchDataTools(c *gin.Context) {
	var body struct {
		Query    string  `json:"query"`
		Limit    int     `json:"limit"`
		MinScore float64 `json:"min_score"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Query == "" {
		body.Query = c.Query("query")
	}
	if body.Query == "" || len(body.Query) > 1000 {
		common.ApiErrorMsg(c, "query must be 1-1000 characters")
		return
	}

	reg, _ := service.GetToolRegistry()
	hits := reg.Discover(body.Query, body.Limit, body.MinScore)
	common.ApiSuccess(c, gin.H{"query": body.Query, "count": len(hits), "results": hits})
}

// InspectDataTool returns the schema and the exact price. The adapter block is
// withheld: it carries upstream URLs and credential names, which are ours.
func InspectDataTool(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		var body struct {
			Id string `json:"id"`
		}
		_ = c.ShouldBindJSON(&body)
		id = body.Id
	}

	reg, _ := service.GetToolRegistry()
	spec, ok := reg.Get(id)
	if !ok {
		common.ApiErrorMsg(c, "unknown tool: "+id)
		return
	}

	common.ApiSuccess(c, gin.H{
		"id":          spec.Id,
		"name":        spec.Name,
		"provider":    spec.Provider,
		"platform":    spec.Platform(),
		"mode":        spec.Mode,
		"categories":  spec.Categories,
		"description": spec.Description,
		"input":       spec.Input,
		"method":      spec.Adapter.Method,
		"endpoint":    dataToolEndpointLabel(spec),
		"pricing": gin.H{
			"model":        spec.Pricing.Model,
			"price_usd":    spec.PriceUSD(1),
			"max_price":    spec.MaxPriceUSD(),
			"pay_on_match": spec.Pricing.PayOnMatch,
			"currency":     "USD",
		},
		"settlement":  dataToolSettlement(spec),
		"docs_url":    spec.DocsURL,
		"quarantined": reg.QuarantineReason(spec),
	})
}

func dataToolSettlement(s *service.ToolSpec) string {
	if s.Settlement == "" {
		return "balance"
	}
	return s.Settlement
}

// RunDataTool executes a tool and charges it against the caller's quota.
//
// Order matters and is the whole safety story: refuse before dispatch when the
// quota cannot cover the worst case, and charge only after a successful call,
// so a failure is always free.
func RunDataTool(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		common.ApiErrorMsg(c, "unauthorized")
		return
	}

	var body struct {
		Id    string         `json:"id"`
		Input map[string]any `json:"input"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Id == "" {
		common.ApiErrorMsg(c, `body must be {"id": "...", "input": {...}}`)
		return
	}

	// Plan entitlement is enforced here, on the server: the entry-level Go plan
	// buys model tokens only, tools unlock from Pro upwards. Hiding the button
	// in the console would not stop an agent calling this endpoint directly.
	access, err := model.CheckToolAccess(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !access.Allowed {
		common.ApiErrorMsg(c, "Your current plan does not include tool calls. Upgrade to Pro or above to use the API Marketplace.")
		return
	}

	reg, _ := service.GetToolRegistry()
	spec, ok := reg.Get(body.Id)
	if !ok {
		common.ApiErrorMsg(c, "unknown tool: "+body.Id)
		return
	}
	if reason := reg.QuarantineReason(spec); reason != "" {
		common.ApiErrorMsg(c, "tool is temporarily unavailable: "+reason)
		return
	}

	groupRatio := service.DataToolGroupRatio(c)
	if _, err := service.CheckDataToolQuota(userId, spec, groupRatio); err != nil {
		if errors.Is(err, service.ErrDataToolInsufficientQuota) {
			common.ApiErrorMsg(c, err.Error())
			return
		}
		common.ApiError(c, err)
		return
	}

	res := service.RunTool(c.Request.Context(), spec, body.Input)
	charge := service.SettleDataToolCall(
		c, userId, c.GetInt("token_id"), c.GetString("token_key"), spec, res, groupRatio)

	if !res.OK {
		common.ApiSuccess(c, gin.H{
			"ok":                 false,
			"tool":               res.ToolId,
			"error":              res.Error,
			"missing_credential": res.MissingEnv,
			"charged_quota":      0,
			"remaining_quota":    charge.RemainingQuota,
			"latency_ms":         res.LatencyMs,
		})
		return
	}

	common.ApiSuccess(c, gin.H{
		"ok":              true,
		"tool":            res.ToolId,
		"output":          res.Output,
		"result_count":    res.ResultCount,
		"charged_quota":   charge.Quota,
		"charged_usd":     charge.USD,
		"remaining_quota": charge.RemainingQuota,
		"latency_ms":      res.LatencyMs,
		"settlement":      dataToolSettlement(spec),
	})
}

// GetDataToolRuns backs the recent-runs table.
func GetDataToolRuns(c *gin.Context) {
	userId := c.GetInt("id")
	limit, _ := strconv.Atoi(c.Query("limit"))
	calls, err := model.GetToolCalls(userId, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"count": len(calls), "runs": calls})
}

// GetDataToolStats backs the stat cards and the provider table.
//
// Success rates here are measured from our own calls. Upstream catalogues have
// been observed reporting providers as connected whose execute call then
// answers "no credentials configured", so a provider's self-reported
// availability is never shown.
func GetDataToolStats(c *gin.Context) {
	userId := c.GetInt("id")

	days, _ := strconv.Atoi(c.Query("days"))
	if days <= 0 || days > 365 {
		days = 30
	}
	since := time.Now().AddDate(0, 0, -days).Unix()

	summary, err := model.GetToolCallSummary(userId, since)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	health, err := model.GetToolProviderHealth(userId, since)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	top, err := model.GetToolTopUsage(userId, since, 10)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	reg, _ := service.GetToolRegistry()
	native := 0
	for _, s := range reg.All() {
		if s.Mode == service.ToolModeNative {
			native++
		}
	}

	quota, _ := model.GetUserQuota(userId, false)
	access, _ := model.CheckToolAccess(userId)

	common.ApiSuccess(c, gin.H{
		"days":            days,
		"remaining_quota": quota,
		"access":          access,
		"catalogue": gin.H{
			"tools":     reg.Len(),
			"native":    native,
			"federated": reg.Len() - native,
			"providers": len(reg.Providers()),
			"platforms": len(reg.Platforms()),
		},
		"summary":   summary,
		"providers": health,
		"top_tools": top,
	})
}


// toolAccessFor resolves entitlement without failing the read when the lookup
// errors — a catalogue that renders is better than a 500.
func toolAccessFor(c *gin.Context) model.ToolAccess {
	access, err := model.CheckToolAccess(c.GetInt("id"))
	if err != nil {
		return model.ToolAccess{Reason: "unknown"}
	}
	return access
}
