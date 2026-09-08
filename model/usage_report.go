package model

// UsageReportDay is one UTC+0 calendar day of user-side reporting metrics.
// This is a NEW reporting board, intentionally separate from the existing
// ops daily report (controller/ops_report.go). It never replaces it.
//
// Metric definitions (mirror the approved user-side design):
//   - Registered:     new users created that UTC day with status=enabled AND
//     email_verified_at > 0.
//   - ActivatedKey:   users whose FIRST API token (tokens.created_time) falls
//     inside that UTC day.
//   - FirstPaid:      users whose FIRST successful top-up (top_ups,
//     status=success) falls inside that UTC day.
//   - PaidUSD:        total successful top-up money settled that UTC day
//     (repeat top-ups included; one-time subscription plans are
//     mirrored into top_ups by the subscription sync pipeline,
//     so they are covered there as well).
//   - Calls/Tokens:   consumption log_requests rows of that UTC day
//     (Log.Type = consume), totalled across the log DB.
type UsageReportDay struct {
	Date             string  `gorm:"primaryKey;type:char(10)" json:"date"` // UTC+0 "2006-01-02"
	Registered       int     `json:"registered"`
	ActivatedKey     int     `json:"activated_key"`
	FirstPaid        int     `json:"first_paid"`
	PaidUSD          float64 `json:"paid_usd" gorm:"type:decimal(14,2);default:0"`
	Calls            int64   `json:"calls"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	BuiltAt          int64   `json:"built_at" gorm:"bigint;default:0"` // unix seconds of last compute
}

func (UsageReportDay) TableName() string {
	return "usage_report_daily"
}

// UsageReportDayModel is the per-model slice of a UTC+0 usage day. Together
// with UsageReportDay it powers the "model stacked area + daily detail"
// views of the usage report board.
type UsageReportDayModel struct {
	Date             string `gorm:"primaryKey;type:char(10)" json:"date"`
	ModelName        string `gorm:"primaryKey;type:varchar(191)" json:"model_name"`
	Calls            int64  `json:"calls"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
}

func (UsageReportDayModel) TableName() string {
	return "usage_report_daily_model"
}

// GetUsageReportDays returns stored daily rows within [from, to] (inclusive),
// ordered by date ascending. Missing dates simply do not appear.
func GetUsageReportDays(from string, to string) ([]*UsageReportDay, error) {
	var rows []*UsageReportDay
	err := DB.Where("date >= ? AND date <= ?", from, to).Order("date ASC").Find(&rows).Error
	return rows, err
}

// GetUsageReportDayModels returns stored per-model rows within [from, to]
// (inclusive), ordered by date then by tokens desc so the stacked-area series
// has a stable order.
func GetUsageReportDayModels(from string, to string) ([]*UsageReportDayModel, error) {
	var rows []*UsageReportDayModel
	err := DB.Where("date >= ? AND date <= ?", from, to).
		Order("date ASC, (prompt_tokens + completion_tokens) DESC, model_name ASC").Find(&rows).Error
	return rows, err
}

// GetUsageReportDayExists reports whether a daily row is already stored.
func GetUsageReportDayExists(date string) (bool, error) {
	var count int64
	err := DB.Model(&UsageReportDay{}).Where("date = ?", date).Count(&count).Error
	return count > 0, err
}
