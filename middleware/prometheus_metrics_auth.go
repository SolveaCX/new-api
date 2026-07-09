package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

const PrometheusMetricsTokenEnv = "PROMETHEUS_METRICS_TOKEN"

func PrometheusMetricsAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		want := strings.TrimSpace(common.GetEnvOrDefaultString(PrometheusMetricsTokenEnv, ""))
		if want == "" {
			c.String(http.StatusServiceUnavailable, "prometheus metrics token not configured")
			c.Abort()
			return
		}
		got := prometheusMetricsBearer(c.GetHeader("Authorization"))
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			c.String(http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}

func prometheusMetricsBearer(header string) string {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}
