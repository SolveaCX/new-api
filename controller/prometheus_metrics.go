package controller

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GetPrometheusMetrics(c *gin.Context) {
	text, err := perfmetrics.BuildPrometheusText(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	if common.IsMasterNode {
		statusText, statusErr := service.BuildStatusCenterPrometheusText(c.Request.Context(), time.Now().Unix())
		if statusErr != nil {
			statusText = "# HELP newapi_status_center_metrics_up Whether status center metrics were collected successfully.\n" +
				"# TYPE newapi_status_center_metrics_up gauge\n" +
				"newapi_status_center_metrics_up 0\n"
		}
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += statusText
	}
	c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(text))
}

func buildStatusCenterPrometheusText(now int64) (string, error) {
	return service.BuildStatusCenterPrometheusText(context.Background(), now)
}
