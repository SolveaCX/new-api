package controller

import (
	"bytes"
	"encoding/csv"
	"fmt"
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
//   - group: "plg" (default, only users whose users.group = 'plg') or "all"
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
	group, err := usageReportGroupParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// Build [from..to] date range (inclusive) around today (UTC+0).
	now := time.Now().UTC()
	to := now.Format(usageReportDateLayout)
	from := now.AddDate(0, 0, -(days - 1)).Format(usageReportDateLayout)

	// CSV exports need a complete window: fill synchronously (rare, admin-only).
	if strings.EqualFold(c.Query("format"), "csv") {
		if err := service.EnsureUsageReportRange(days); err != nil {
			common.ApiError(c, err)
			return
		}
		dim := strings.ToLower(c.Query("dim"))
		if dim == "models" {
			modelRows, err := model.GetUsageReportDayModels(from, to, group)
			if err != nil {
				common.ApiError(c, err)
				return
			}
			writeUsageReportModelsCSV(c, modelRows)
			return
		}
		dayRows, err := model.GetUsageReportDays(from, to, group)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		writeUsageReportDailyCSV(c, dayRows)
		return
	}

	// Interactive view: never block the request on a historical backfill.
	// Kick off the warm fill in the background and serve what is already
	// persisted; the response carries a "filling" flag until the window is
	// complete so the front-end can poll.
	service.EnsureUsageReportRangeAsync(days)

	dayRows, err := model.GetUsageReportDays(from, to, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	modelRows, err := model.GetUsageReportDayModels(from, to, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"group":  group,
			"days":   dayRows,
			"models": modelRows,
			// Combine data completeness with the runner state: if the background
			// fill finished right between query and response the rows may still be
			// short, so the front-end keeps polling until the window is complete.
			"filling": len(dayRows) < days || service.UsageReportFillRunning(),
		},
	})
}

func writeUsageReportDailyCSV(c *gin.Context, rows []*model.UsageReportDay) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"date", "registered", "activated_day", "paid_day", "paid_usd", "calls", "tokens"})
	for _, r := range rows {
		_ = w.Write([]string{
			r.Date,
			strconv.Itoa(r.Registered),
			strconv.Itoa(r.ActivatedDay),
			strconv.Itoa(r.PaidDay),
			strconv.FormatFloat(r.PaidUSD, 'f', 2, 64),
			strconv.FormatInt(r.Calls, 10),
			strconv.FormatInt(r.PromptTokens+r.CompletionTokens, 10),
		})
	}
	w.Flush()
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="usage_report_daily.csv"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
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
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}

// usageReportBackfillResult is one date's outcome in the manual backfill API.
type usageReportBackfillResult struct {
	Date  string `json:"date"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// BackfillUsageReport force-recomputes stored days from the source tables.
// Admin-only escape hatch for "a date is missing / looks stale" on production,
// complementing the automatic 00:00 UTC job.
//
// Params (UTC+0 dates, exactly one form):
//   - date=YYYY-MM-DD        single day
//   - from=YYYY-MM-DD&to=YYYY-MM-DD   inclusive range (<= 180 days)
//   - days=N                 trailing N days ending today
//
// The result lists every date with ok/error so a partial failure is visible.
func BackfillUsageReport(c *gin.Context) {
	dates, err := usageReportBackfillDates(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	results := make([]usageReportBackfillResult, 0, len(dates))
	okCount := 0
	for _, d := range dates {
		if err := service.RecomputeUsageReportDateAllGroups(d); err != nil {
			results = append(results, usageReportBackfillResult{Date: d, OK: false, Error: err.Error()})
			continue
		}
		okCount++
		results = append(results, usageReportBackfillResult{Date: d, OK: true})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"results": results,
			"ok":      okCount,
			"failed":  len(dates) - okCount,
		},
	})
}

// usageReportGroupParam validates ?group= (plg | all); default plg.
func usageReportGroupParam(c *gin.Context) (string, error) {
	group := strings.ToLower(strings.TrimSpace(c.Query("group")))
	if group == "" {
		return service.UsageReportGroupPLG, nil
	}
	if group != service.UsageReportGroupPLG && group != service.UsageReportGroupAll {
		return "", fmt.Errorf("invalid group %q, expected plg or all", group)
	}
	return group, nil
}

// usageReportBackfillDates parses and validates the backfill query params.
func usageReportBackfillDates(c *gin.Context) ([]string, error) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	parse := func(v string) (time.Time, error) {
		t, err := time.ParseInLocation(usageReportDateLayout, v, time.UTC)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid date %q, expected YYYY-MM-DD", v)
		}
		if t.After(today) {
			return time.Time{}, fmt.Errorf("date %s is in the future (UTC+0 today is %s)", v, today.Format(usageReportDateLayout))
		}
		return t, nil
	}

	if v := strings.TrimSpace(c.Query("date")); v != "" {
		t, err := parse(v)
		if err != nil {
			return nil, err
		}
		return []string{t.Format(usageReportDateLayout)}, nil
	}

	fromStr := strings.TrimSpace(c.Query("from"))
	toStr := strings.TrimSpace(c.Query("to"))
	if fromStr != "" || toStr != "" {
		if fromStr == "" || toStr == "" {
			return nil, fmt.Errorf("both from and to are required (YYYY-MM-DD)")
		}
		from, err := parse(fromStr)
		if err != nil {
			return nil, err
		}
		to, err := parse(toStr)
		if err != nil {
			return nil, err
		}
		if to.Before(from) {
			return nil, fmt.Errorf("to (%s) must not be earlier than from (%s)", toStr, fromStr)
		}
		span := int(to.Sub(from).Hours()/24) + 1
		if span > usageReportMaxDays {
			return nil, fmt.Errorf("range spans %d days, max is %d", span, usageReportMaxDays)
		}
		dates := make([]string, 0, span)
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			dates = append(dates, d.Format(usageReportDateLayout))
		}
		return dates, nil
	}

	if v := strings.TrimSpace(c.Query("days")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("days must be a positive integer")
		}
		if n > usageReportMaxDays {
			n = usageReportMaxDays
		}
		dates := make([]string, 0, n)
		for i := n - 1; i >= 0; i-- {
			dates = append(dates, today.AddDate(0, 0, -i).Format(usageReportDateLayout))
		}
		return dates, nil
	}

	return nil, fmt.Errorf("specify date=YYYY-MM-DD, from=&to=, or days=N")
}
