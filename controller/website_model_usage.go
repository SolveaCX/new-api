package controller

import (
	"crypto/subtle"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// WEBSITE_METRICS_KEY gates the public website's read-only usage feed. The
// website calls this server-side (never from the browser) with the shared key.
//
// The dashboard's own /api/data routes are admin/user scoped and expose quota,
// usernames, and per-channel spend. This endpoint deliberately exposes only the
// daily call count for one model: no money, no users, no channels. Even so it
// is not left open, because per-model call volume is competitive information.
//
// Unset key => endpoint disabled (404). A deployment that has not configured a
// key must not silently serve the data.
const websiteMetricsKeyEnv = "WEBSITE_METRICS_KEY"

// Bounds the scan: the website charts a month, and an unbounded range would let
// a caller walk the whole table one request at a time.
const websiteModelUsageMaxDays = 90

type websiteModelUsagePoint struct {
	// Unix seconds at UTC day start.
	Date  int64 `json:"date"`
	Count int   `json:"count"`
}

type websiteModelUsageResponse struct {
	Success bool                     `json:"success"`
	Model   string                   `json:"model"`
	Days    int                      `json:"days"`
	Total   int                      `json:"total"`
	Points  []websiteModelUsagePoint `json:"points"`
}

// GetWebsiteModelUsage returns the daily request count for a single model.
//
// GET /api/website/model-usage?model=<name>&days=<n>
// Header: X-Website-Metrics-Key: <WEBSITE_METRICS_KEY>
func GetWebsiteModelUsage(c *gin.Context) {
	configuredKey := strings.TrimSpace(common.GetEnvOrDefaultString(websiteMetricsKeyEnv, ""))
	if configuredKey == "" {
		// 404 rather than 503: an unconfigured deployment should not advertise
		// that this endpoint exists.
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "not found"})
		return
	}

	presented := strings.TrimSpace(c.GetHeader("X-Website-Metrics-Key"))
	// Constant-time compare so a caller cannot recover the key byte by byte.
	if subtle.ConstantTimeCompare([]byte(presented), []byte(configuredKey)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "unauthorized"})
		return
	}

	modelName := strings.TrimSpace(c.Query("model"))
	if modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "model is required"})
		return
	}

	days := common.String2Int(c.Query("days"))
	if days <= 0 {
		days = 30
	}
	if days > websiteModelUsageMaxDays {
		days = websiteModelUsageMaxDays
	}

	// quota_data rows are bucketed hourly; align the window to UTC day starts so
	// the first and last buckets are whole days rather than partial ones.
	endOfToday := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	start := endOfToday.Add(-time.Duration(days) * 24 * time.Hour)

	rows, err := model.GetModelDailyUsage(modelName, start.Unix(), endOfToday.Unix())
	if err != nil {
		common.SysError("failed to load website model usage: " + err.Error())
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "usage temporarily unavailable",
		})
		return
	}

	points := make([]websiteModelUsagePoint, 0, len(rows))
	total := 0
	for _, row := range rows {
		points = append(points, websiteModelUsagePoint{Date: row.Date, Count: row.Count})
		total += row.Count
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Date < points[j].Date })

	c.Header("Cache-Control", "public, max-age=300, must-revalidate")
	c.JSON(http.StatusOK, websiteModelUsageResponse{
		Success: true,
		Model:   modelName,
		Days:    days,
		Total:   total,
		Points:  points,
	})
}
