package model

import (
	"time"
)

// ToolCall records one data-tool execution.
//
// Billing itself does NOT live here — a tool call deducts from the same
// `users.quota` as a token call and writes the same consume log, so plans,
// redemption codes and subscriptions cover tool usage without any new billing
// path. This table exists for the marketplace surfaces that the token logs
// cannot answer: per-provider health, latency and realised margin.
//
// Columns are deliberately flat (no JSON) so the same schema works on SQLite,
// MySQL and PostgreSQL without dialect-specific operators.
type ToolCall struct {
	Id       int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId   int    `json:"user_id" gorm:"index;not null"`
	TokenId  int    `json:"token_id" gorm:"index"`
	ToolId   string `json:"tool_id" gorm:"size:160;index;not null"`
	Provider string `json:"provider" gorm:"size:120;index;not null"`
	// native = direct upstream contract (full margin)
	// federated = resold through an aggregator (thin margin)
	Mode        string `json:"mode" gorm:"size:16"`
	Success     bool   `json:"success" gorm:"default:false"`
	ErrorMsg    string `json:"error_msg" gorm:"type:text"`
	ResultCount int    `json:"result_count" gorm:"default:0"`
	// Quota charged to the user, in the same unit as token quota.
	Quota int `json:"quota" gorm:"default:0"`
	// Micro-USD (USD * 1e6) so margin reporting stays exact in integer maths.
	ChargedMicroUsd int `json:"charged_micro_usd" gorm:"default:0"`
	CostMicroUsd    int `json:"cost_micro_usd" gorm:"default:0"`
	LatencyMs       int `json:"latency_ms" gorm:"default:0"`

	CreatedAt int64 `json:"created_at" gorm:"bigint;index"`
}

func (ToolCall) TableName() string {
	return "tool_calls"
}

// RecordToolCall persists one execution. Failures are recorded too: they are
// the only source of the measured success rate the marketplace shows, and
// upstream catalogues cannot be trusted to report their own availability.
func RecordToolCall(call *ToolCall) error {
	if call.Mode == "" {
		call.Mode = "native"
	}
	if call.CreatedAt == 0 {
		call.CreatedAt = time.Now().Unix()
	}
	call.ErrorMsg = truncateToolError(call.ErrorMsg)
	return DB.Create(call).Error
}

// maxToolErrorLen keeps one chatty upstream from writing an essay per row.
const maxToolErrorLen = 500

// truncateToolError strips what a text column cannot hold. Upstream error
// bodies are arbitrary bytes — some scrapers echo raw page content back — and
// a NUL byte makes the INSERT fail on PostgreSQL with "invalid byte sequence
// for encoding". Losing the row would be worse than losing the exact bytes.
func truncateToolError(s string) string {
	if s == "" {
		return ""
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == 0 || r == '�' || (r < 0x20 && r != '\n' && r != '\t') {
			continue
		}
		out = append(out, r)
		if len(out) >= maxToolErrorLen {
			break
		}
	}
	return string(out)
}

// ToolCallSummary powers the stat cards above the marketplace.
type ToolCallSummary struct {
	Calls        int64 `json:"calls"`
	Succeeded    int64 `json:"succeeded"`
	Quota        int64 `json:"quota"`
	ChargedMicro int64 `json:"charged_micro_usd"`
	CostMicro    int64 `json:"cost_micro_usd"`
	AvgLatencyMs int64 `json:"avg_latency_ms"`
}

// GetToolCallSummary aggregates a user's calls since the given unix second.
// Uses only portable aggregates so the query runs identically on all three
// supported databases.
func GetToolCallSummary(userId int, since int64) (*ToolCallSummary, error) {
	var row struct {
		Calls        int64
		Succeeded    int64
		Quota        int64
		ChargedMicro int64
		CostMicro    int64
		AvgLatency   float64
	}
	q := DB.Model(&ToolCall{}).Where("created_at >= ?", since)
	if userId > 0 {
		q = q.Where("user_id = ?", userId)
	}
	err := q.Select(`
		COUNT(*) AS calls,
		COALESCE(SUM(CASE WHEN success THEN 1 ELSE 0 END), 0) AS succeeded,
		COALESCE(SUM(quota), 0) AS quota,
		COALESCE(SUM(charged_micro_usd), 0) AS charged_micro,
		COALESCE(SUM(cost_micro_usd), 0) AS cost_micro,
		COALESCE(AVG(latency_ms), 0) AS avg_latency`).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	return &ToolCallSummary{
		Calls:        row.Calls,
		Succeeded:    row.Succeeded,
		Quota:        row.Quota,
		ChargedMicro: row.ChargedMicro,
		CostMicro:    row.CostMicro,
		AvgLatencyMs: int64(row.AvgLatency),
	}, nil
}

// ToolProviderHealth is measured from our own calls. Upstream catalogues have
// been observed reporting providers as connected whose execute call then
// answers "no credentials configured", so the marketplace never shows a
// provider's self-reported availability.
type ToolProviderHealth struct {
	Provider     string  `json:"provider"`
	Calls        int64   `json:"calls"`
	Succeeded    int64   `json:"succeeded"`
	SuccessRate  float64 `json:"success_rate"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	ChargedMicro int64   `json:"charged_micro_usd"`
	CostMicro    int64   `json:"cost_micro_usd"`
	MarginMicro  int64   `json:"margin_micro_usd"`
}

func GetToolProviderHealth(userId int, since int64) ([]ToolProviderHealth, error) {
	var rows []struct {
		Provider     string
		Calls        int64
		Succeeded    int64
		ChargedMicro int64
		CostMicro    int64
		AvgLatency   float64
	}
	q := DB.Model(&ToolCall{}).Where("created_at >= ?", since)
	if userId > 0 {
		q = q.Where("user_id = ?", userId)
	}
	err := q.Select(`
		provider,
		COUNT(*) AS calls,
		COALESCE(SUM(CASE WHEN success THEN 1 ELSE 0 END), 0) AS succeeded,
		COALESCE(SUM(charged_micro_usd), 0) AS charged_micro,
		COALESCE(SUM(cost_micro_usd), 0) AS cost_micro,
		COALESCE(AVG(latency_ms), 0) AS avg_latency`).
		Group("provider").
		Order("calls DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]ToolProviderHealth, 0, len(rows))
	for _, r := range rows {
		h := ToolProviderHealth{
			Provider:     r.Provider,
			Calls:        r.Calls,
			Succeeded:    r.Succeeded,
			AvgLatencyMs: int64(r.AvgLatency),
			ChargedMicro: r.ChargedMicro,
			CostMicro:    r.CostMicro,
			MarginMicro:  r.ChargedMicro - r.CostMicro,
		}
		if r.Calls > 0 {
			h.SuccessRate = float64(r.Succeeded) / float64(r.Calls)
		}
		out = append(out, h)
	}
	return out, nil
}

// GetToolCalls lists a user's recent executions for the Runs table.
func GetToolCalls(userId int, limit int) ([]*ToolCall, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var calls []*ToolCall
	q := DB.Model(&ToolCall{}).Order("id DESC").Limit(limit)
	if userId > 0 {
		q = q.Where("user_id = ?", userId)
	}
	err := q.Find(&calls).Error
	return calls, err
}

// ToolTopUsage ranks tools by call volume. This ranking is the promotion list:
// the federated tools at the top are the ones worth replacing with a direct
// upstream contract.
type ToolTopUsage struct {
	ToolId string `json:"tool_id"`
	Mode   string `json:"mode"`
	Calls  int64  `json:"calls"`
}

func GetToolTopUsage(userId int, since int64, limit int) ([]ToolTopUsage, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	var rows []ToolTopUsage
	q := DB.Model(&ToolCall{}).Where("created_at >= ?", since)
	if userId > 0 {
		q = q.Where("user_id = ?", userId)
	}
	err := q.Select("tool_id, mode, COUNT(*) AS calls").
		Group("tool_id, mode").
		Order("calls DESC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}
