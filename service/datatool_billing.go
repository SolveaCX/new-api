package service

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

// Data-tool billing deliberately reuses the token money path instead of
// inventing a second one: a tool call deducts from `users.quota` through
// model.DecreaseUserQuota and writes the same consume log a model call writes.
//
// That is what makes plans, redemption codes and subscriptions cover tool usage
// with no new billing concept — top up once, spend it on either half of
// "one key. more models, more tools, less spend."
//
// The tool_calls table alongside it carries only what the shared consume log
// cannot express (provider, latency, realised margin) and never participates in
// charging.
//
// Naming note: `tool_billing.go` in this package already bills OpenAI-style
// in-request tool calls (web_search / file_search / image_generation). This
// file is about the data-tool marketplace and is kept separate on purpose.
//
// Multi-node (Rule 11): the deduction is a single `quota = quota - ?` UPDATE,
// atomic per row, and nothing here depends on process-local state.

var ErrDataToolInsufficientQuota = errors.New("insufficient quota for this tool call")

// DataToolQuota converts a USD price into the quota unit token billing uses,
// applying the caller's group ratio exactly as ComputeToolCallQuota does — so a
// discounted group is discounted on tools too, not just on models.
func DataToolQuota(usd float64, groupRatio float64) int {
	if usd <= 0 {
		return 0
	}
	if groupRatio <= 0 {
		groupRatio = 1
	}
	return int(math.Ceil(usd * common.QuotaPerUnit * groupRatio))
}

// DataToolGroupRatio resolves the ratio for the caller's group, defaulting to
// 1 when the group is unknown.
func DataToolGroupRatio(c *gin.Context) float64 {
	group := c.GetString("group")
	if group == "" {
		group = c.GetString("user_group")
	}
	if group == "" {
		return 1
	}
	ratio := ratio_setting.GetGroupRatio(group)
	if ratio <= 0 {
		return 1
	}
	return ratio
}

// CheckDataToolQuota refuses before dispatch when the caller cannot cover the
// worst case for this tool. Refusing first and charging last is what makes a
// failed call free.
func CheckDataToolQuota(userId int, spec *ToolSpec, groupRatio float64) (int, error) {
	quota, err := model.GetUserQuota(userId, false)
	if err != nil {
		return 0, err
	}
	need := DataToolQuota(spec.MaxPriceUSD(), groupRatio)
	if need > 0 && quota < need {
		return quota, fmt.Errorf("%w: remaining quota %d is below the %d this call may cost",
			ErrDataToolInsufficientQuota, quota, need)
	}
	return quota, nil
}

// DataToolCharge reports what a completed call cost.
type DataToolCharge struct {
	Quota          int     `json:"quota"`
	USD            float64 `json:"usd"`
	RemainingQuota int     `json:"remaining_quota"`
}

// SettleDataToolCall charges a completed call and records it.
//
// A failed call still gets a tool_calls row — it is the only source of the
// measured provider success rate the marketplace shows — but is charged nothing
// and writes no consume log.
func SettleDataToolCall(
	c *gin.Context,
	userId int,
	tokenId int,
	tokenKey string,
	spec *ToolSpec,
	res *ToolResult,
	groupRatio float64,
) *DataToolCharge {
	usd := 0.0
	quota := 0
	if res.OK {
		usd = spec.PriceUSD(res.ResultCount)
		quota = DataToolQuota(usd, groupRatio)
	}

	upstreamCost := 0.0
	if res.OK && spec.Pricing.Cost > 0 {
		upstreamCost = spec.Pricing.Cost
		if spec.Pricing.Model == ToolPerResult {
			upstreamCost *= float64(res.ResultCount)
		}
	}

	// x402-settled tools are paid on-chain from the caller's own wallet, so the
	// prepaid quota is untouched; the row is still recorded for reporting.
	if spec.Settlement != "" && spec.Settlement != "balance" {
		quota = 0
	}

	if quota > 0 {
		if err := model.DecreaseUserQuota(userId, quota, false); err != nil {
			common.SysError("data tool: failed to decrease user quota: " + err.Error())
		} else {
			if tokenId > 0 && tokenKey != "" {
				if err := model.DecreaseTokenQuota(tokenId, tokenKey, quota); err != nil {
					common.SysError("data tool: failed to decrease token quota: " + err.Error())
				}
			}
			// The same consume log a model call writes, so tool spend appears
			// in the existing usage views without a parallel reporting path.
			model.RecordConsumeLog(c, userId, model.RecordConsumeLogParams{
				ModelName:      DataToolLogModelName(spec.Id),
				TokenName:      c.GetString("token_name"),
				TokenId:        tokenId,
				Quota:          quota,
				Content:        fmt.Sprintf("Tool call · %d result(s) · $%.6f", res.ResultCount, usd),
				UseTimeSeconds: res.LatencyMs / 1000,
				Group:          c.GetString("group"),
				Other: map[string]interface{}{
					"tool_id":       spec.Id,
					"tool_provider": spec.Provider,
					"tool_mode":     string(spec.Mode),
					"result_count":  res.ResultCount,
					"group_ratio":   groupRatio,
				},
			})
		}
	}

	if err := model.RecordToolCall(&model.ToolCall{
		UserId:          userId,
		TokenId:         tokenId,
		ToolId:          spec.Id,
		Provider:        spec.Provider,
		Mode:            string(spec.Mode),
		Success:         res.OK,
		ErrorMsg:        res.Error,
		ResultCount:     res.ResultCount,
		Quota:           quota,
		ChargedMicroUsd: int(math.Round(usd * 1e6)),
		CostMicroUsd:    int(math.Round(upstreamCost * 1e6)),
		LatencyMs:       res.LatencyMs,
		CreatedAt:       time.Now().Unix(),
	}); err != nil {
		// The upstream call already happened and the caller was already
		// charged; losing the analytics row must not turn a successful call
		// into an error for the caller.
		common.SysError("data tool: failed to record tool call: " + err.Error())
	}

	remaining, _ := model.GetUserQuota(userId, false)
	return &DataToolCharge{Quota: quota, USD: usd, RemainingQuota: remaining}
}

// DataToolLogModelName namespaces tool spend in the shared consume log so it is
// obvious in usage views which rows are tools rather than models.
func DataToolLogModelName(toolId string) string {
	return "tool/" + toolId
}
