package controller

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	usageReportDefaultDays = 30
	usageReportMaxDays     = 180
	usageReportDateLayout  = "2006-01-02"
)

// GetUsageReport serves the NEW user-side usage report board (usage_report_*),
// separate from the existing ops daily report. It returns one UTC+0 day row
// per requested date plus the per-model slices, optionally as CSV.
//
// Params:
//   - days: trailing window length (default 30, cap 180)
//   - format: "csv" for CSV output, otherwise JSON
//   - dim:    CSV dimension — "daily" (default) or "models"
func GetUsageReport(c *gin.Context) {
	days, _ := strconv.Atoi(c.Query("days"))
	if days <= 0 {
		days = usageReportDefaultDays
	}
	if days > usageReportMaxDays {
		days = usageReportMaxDays
	}
	if err := service.EnsureUsageReportRange(days); err != nil {
		common.ApiError(c, err)
		return
	}

	// Build [from..to] date range (inclusive) around today (UTC+0).
	now := time.Now().UTC()
	to := now.Format(usageReportDateLayout)
	from := now.AddDate(0, 0, -(days - 1)).Format(usageReportDateLayout)

	dayRows, err := model.GetUsageReportDays(from, to)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	modelRows, err := model.GetUsageReportDayModels(from, to)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if strings.EqualFold(c.Query("format"), "csv") {
		dim := strings.ToLower(c.Query("dim"))
		if dim == "models" {
			writeUsageReportModelsCSV(c, modelRows)
			return
		}
		writeUsageReportDailyCSV(c, dayRows)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"days":   dayRows,
			"models": modelRows,
		},
	})
}

func writeUsageReportDailyCSV(c *gin.Context, rows []*model.UsageReportDay) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"date", "registered", "activated_key", "first_paid", "paid_usd", "calls", "tokens"})
	for _, r := range rows {
		_ = w.Write([]string{
			r.Date,
			strconv.Itoa(r.Registered),
			strconv.Itoa(r.ActivatedKey),
			strconv.Itoa(r.FirstPaid),
			strconv.FormatFloat(r.PaidUSD, 'f', 2, 64),
			strconv.FormatInt(r.Calls, 10),
			strconv.FormatInt(r.PromptTokens+r.CompletionTokens, 10),
		})
	}
	w.Flush()
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="usage_report_daily.csv"`)
	c.String(http.StatusOK, buf.String())
}

func writeUsageReportModelsCSV(c *gin.Context, rows []*model.UsageReportDayModel) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"date", "model", "calls", "tokens"})
	for _, r := range rows {
		_ = w.Write([]string{
			r.Date,
			r.ModelName,
			strconv.FormatInt(r.Calls, 10),
			strconv.FormatInt(r.PromptTokens+r.CompletionTokens, 10),
		})
	}
	w.Flush()
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="usage_report_models.csv"`)
	c.String(http.StatusOK, buf.String())
}
