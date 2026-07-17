package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStatusRoutesExposePublicAndGuardedAdminSurface(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	want := map[string]string{
		"/api/status/summary":                         http.MethodGet,
		"/api/status/components":                      http.MethodGet,
		"/api/status/components/:slug":                http.MethodGet,
		"/api/status/components/:slug/history":        http.MethodGet,
		"/api/status/incidents":                       http.MethodGet,
		"/api/status/incidents/:id":                   http.MethodGet,
		"/api/status/maintenance":                     http.MethodGet,
		"/api/status/subscriptions":                   http.MethodPost,
		"/api/status/subscriptions/verify":            http.MethodGet,
		"/api/status/subscriptions/unsubscribe":       http.MethodPost,
		"/api/status/admin/incidents":                 http.MethodGet,
		"/api/status/admin/incidents/:id":             http.MethodGet,
		"/api/status/admin/incidents/:id/publish":     http.MethodPost,
		"/api/status/admin/maintenance":               http.MethodPost,
		"/api/status/admin/maintenance/:id/reconcile": http.MethodPost,
		"/api/status/admin/overrides":                 http.MethodPost,
		"/api/status/admin/overrides/force-green":     http.MethodPost,
		"/api/status/admin/settings":                  http.MethodGet,
		"/api/status/admin/settings/discord":          http.MethodPut,
		"/api/status/admin/settings/:key":             http.MethodPut,
		"/api/status/admin/discord/test":              http.MethodPost,
		"/api/status/admin/subscribers":               http.MethodGet,
		"/api/status/admin/deliveries":                http.MethodGet,
		"/api/status/admin/audit":                     http.MethodGet,
	}
	got := map[string]string{}
	for _, route := range engine.Routes() {
		if _, ok := want[route.Path]; ok {
			got[route.Path] = route.Method
		}
	}
	require.Equal(t, want, got)

	getUnsubscribe := false
	for _, route := range engine.Routes() {
		if route.Path == "/api/status/subscriptions/unsubscribe" && route.Method == http.MethodGet {
			getUnsubscribe = true
		}
	}
	require.True(t, getUnsubscribe, "GET unsubscribe must remain confirmation-only")
}
