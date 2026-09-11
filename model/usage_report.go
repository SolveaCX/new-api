package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

// usageReportGroupCol returns the dialect-quoted `group` column (group is a
// reserved word in MySQL and PostgreSQL).
func usageReportGroupCol() string {
	if common.UsingPostgreSQL {
		return `"group"`
	}
	return "`group`"
}

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
	Date string `gorm:"primaryKey;type:char(10)" json:"date"` // UTC+0 "2006-01-02"
	// Group: 分组维度（users.group / logs.group）。"plg" = 仅 PLG 分组用户，
	// "all" = 不限分组。主键 = (date, group)。
	Group        string  `gorm:"primaryKey;type:varchar(32);default:'plg'" json:"group"`
	Registered   int     `gorm:"not null;default:0" json:"registered"`
	ActivatedKey int     `gorm:"not null;default:0" json:"activated_key"` // 当日首次建 Key 的去重用户数（事件口径）
	FirstPaid    int     `gorm:"not null;default:0" json:"first_paid"`    // 当日首次付费的去重用户数（事件口径）
	PaidUSD      float64 `gorm:"type:decimal(14,2);not null;default:0" json:"paid_usd"`
	// 当天口径（C 端快进快出，主口径；⊆ Registered）：
	//   ActivatedDay: 该日注册的人中，注册当天即首次建 Key 的人数
	//   PaidDay:      该日注册的人中，注册当天即首次付费的人数
	ActivatedDay int `gorm:"not null;default:0" json:"activated_day"`
	PaidDay      int `gorm:"not null;default:0" json:"paid_day"`
	// 长窗辅助字段（人；未用于主表，保留作分析）：
	//   ActivatedC7 / PaidRegC14 / PaidC14 …
	ActivatedC7      int   `gorm:"not null;default:0" json:"activated_c7"`
	PaidC14          int   `gorm:"not null;default:0" json:"paid_c14"`
	PaidRegC14       int   `gorm:"not null;default:0" json:"paid_reg_c14"`
	Calls            int64 `gorm:"not null;default:0" json:"calls"`
	PromptTokens     int64 `gorm:"not null;default:0" json:"prompt_tokens"`
	CompletionTokens int64 `gorm:"not null;default:0" json:"completion_tokens"`
	BuiltAt          int64 `gorm:"bigint;not null;default:0" json:"built_at"` // unix seconds of last compute
	SchemaV          int   `gorm:"not null;default:0" json:"-"`               // aggregation schema version, bump to force one-time recompute
}

func (UsageReportDay) TableName() string {
	return "usage_report_daily"
}

// UsageReportDayModel is the per-model slice of a UTC+0 usage day. Together
// with UsageReportDay it powers the "model stacked area + daily detail"
// views of the usage report board.
type UsageReportDayModel struct {
	Date             string `gorm:"primaryKey;type:char(10)" json:"date"`
	Group            string `gorm:"primaryKey;type:varchar(32);default:'plg'" json:"group"`
	ModelName        string `gorm:"primaryKey;type:varchar(191)" json:"model_name"`
	Calls            int64  `json:"calls"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
}

func (UsageReportDayModel) TableName() string {
	return "usage_report_daily_model"
}

// GetUsageReportDays returns stored daily rows of one group within [from, to]
// (inclusive), ordered by date ascending. Missing dates simply do not appear.
func GetUsageReportDays(from string, to string, group string) ([]*UsageReportDay, error) {
	var rows []*UsageReportDay
	err := DB.Where(fmt.Sprintf("date >= ? AND date <= ? AND %s = ?", usageReportGroupCol()), from, to, group).
		Order("date ASC").Find(&rows).Error
	return rows, err
}

// GetUsageReportDayModels returns stored per-model rows of one group within
// [from, to] (inclusive), ordered by date then by tokens desc so the
// stacked-area series has a stable order.
func GetUsageReportDayModels(from string, to string, group string) ([]*UsageReportDayModel, error) {
	var rows []*UsageReportDayModel
	err := DB.Where(fmt.Sprintf("date >= ? AND date <= ? AND %s = ?", usageReportGroupCol()), from, to, group).
		Order("date ASC, (prompt_tokens + completion_tokens) DESC, model_name ASC").Find(&rows).Error
	return rows, err
}

// GetUsageReportDayExists reports whether a daily row for (date, group) exists.
func GetUsageReportDayExists(date string, group string) (bool, error) {
	var count int64
	err := DB.Model(&UsageReportDay{}).
		Where(fmt.Sprintf("date = ? AND %s = ?", usageReportGroupCol()), date, group).Count(&count).Error
	return count > 0, err
}

// HasUsageReportGroupColumn reports whether usage_report_daily already has the
// group dimension (legacy tables written before it must be rebuilt because the
// primary key changes from date to (date, group)).
func HasUsageReportGroupColumn() bool {
	if !DB.Migrator().HasTable(&UsageReportDay{}) {
		return true // nothing stored yet; AutoMigrate will create it correctly
	}
	return DB.Migrator().HasColumn(&UsageReportDay{}, "group")
}

// DropLegacyUsageReportTables removes the pre-group tables; the data is a
// derived cache and is rebuilt by the aggregation task/reads.
func DropLegacyUsageReportTables() error {
	for _, table := range []interface{}{&UsageReportDay{}, &UsageReportDayModel{}} {
		if DB.Migrator().HasTable(table) {
			if err := DB.Migrator().DropTable(table); err != nil {
				return err
			}
		}
	}
	return nil
}
