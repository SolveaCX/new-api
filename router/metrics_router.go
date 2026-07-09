package router

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/pkg/prommetrics"

	"github.com/gin-gonic/gin"
)

func SetMetricsRouter(router *gin.Engine) {
	if !common.GetEnvOrDefaultBool("PROMETHEUS_METRICS_ENABLED", false) {
		return
	}
	token := strings.TrimSpace(common.GetEnvOrDefaultString("PROMETHEUS_METRICS_TOKEN", ""))
	if token == "" {
		return
	}

	metricsRouter := router.Group("/metrics")
	metricsRouter.Use(middleware.RouteTag("metrics"))
	metricsRouter.GET("", func(c *gin.Context) {
		if !metricsBearerAuthorized(c.GetHeader("Authorization"), token) {
			c.Status(http.StatusUnauthorized)
			return
		}
		prommetrics.Handler().ServeHTTP(c.Writer, c.Request)
	})
}

func metricsBearerAuthorized(header string, token string) bool {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return false
	}
	got := strings.TrimSpace(parts[1])
	if got == "" {
		return false
	}
	gotHash := sha256.Sum256([]byte(got))
	wantHash := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1
}
