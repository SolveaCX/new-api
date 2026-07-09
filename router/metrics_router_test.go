package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/pkg/prommetrics"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func TestMetricsRouteDisabledReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("PROMETHEUS_METRICS_ENABLED", "")
	t.Setenv("PROMETHEUS_METRICS_TOKEN", "secret")

	engine := gin.New()
	SetMetricsRouter(engine)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected disabled metrics route to return 404, got %d", rec.Code)
	}
}

func TestMetricsRouteEnabledWithoutTokenReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("PROMETHEUS_METRICS_ENABLED", "true")
	t.Setenv("PROMETHEUS_METRICS_TOKEN", "")

	engine := gin.New()
	SetMetricsRouter(engine)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected metrics route without configured token to return 404, got %d", rec.Code)
	}
}

func TestMetricsRouteRejectsWrongBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("PROMETHEUS_METRICS_ENABLED", "true")
	t.Setenv("PROMETHEUS_METRICS_TOKEN", "secret")

	engine := gin.New()
	SetMetricsRouter(engine)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected wrong metrics bearer token to return 401, got %d", rec.Code)
	}
}

func TestMetricsRouteAcceptsConfiguredBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("PROMETHEUS_METRICS_ENABLED", "true")
	t.Setenv("PROMETHEUS_METRICS_TOKEN", "secret")
	t.Setenv("PROMETHEUS_METRICS_SERVICE_ROLE", "router")
	prommetrics.ResetDefaultForTest()
	prommetrics.RecordRelaySample(&relaycommon.RelayInfo{
		OriginModelName: "gpt-5.4",
		UsingGroup:      "paid",
		StartTime:       time.Now().Add(-100 * time.Millisecond),
		RelayFormat:     types.RelayFormatOpenAI,
	}, true, http.StatusOK, 7)

	engine := gin.New()
	SetMetricsRouter(engine)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer secret")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected correct metrics bearer token to return 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "flatkey_relay_requests_total") {
		t.Fatalf("expected metrics body to include relay counter, got:\n%s", rec.Body.String())
	}
}
