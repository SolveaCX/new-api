package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCustomerPaymentRoutesRequirePLGAndHistoryRemainsAccessible(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	var handlers []string
	engine.Use(func(c *gin.Context) { handlers = c.HandlerNames(); c.AbortWithStatus(http.StatusNoContent) })
	SetApiRouter(engine)
	guarded := []string{
		"/api/user/pay", "/api/user/stripe/pay", "/api/user/creem/pay",
		"/api/user/waffo/pay", "/api/user/waffo-pancake/pay", "/api/user/paddle/pay",
		"/api/user/topup/test/resume", "/api/user/stripe/checkout/discount",
		"/api/subscription/self/quote", "/api/subscription/self/purchase",
		"/api/subscription/self/change-plan", "/api/subscription/self/renewal/resume",
		"/api/subscription/balance/pay", "/api/subscription/epay/pay",
		"/api/subscription/stripe/pay", "/api/subscription/creem/pay",
		"/api/subscription/waffo-pancake/pay",
	}
	for _, path := range guarded {
		t.Run(path, func(t *testing.T) {
			handlers = nil
			engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, path, nil))
			require.Contains(t, strings.Join(handlers, "\n"), "RequireSelfServicePayment", path)
		})
	}
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/user/topup/self"}, {"GET", "/api/subscription/self"},
		{"POST", "/api/user/topup/test/invoice"}, {"POST", "/api/user/stripe/checkout/close"},
		{"POST", "/api/subscription/self/renewal/cancel"},
		{"POST", "/api/user/epay/notify"}, {"POST", "/api/subscription/epay/notify"},
	} {
		handlers = nil
		engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(route.method, route.path, nil))
		require.Greater(t, len(handlers), 1, route.path)
		require.NotContains(t, strings.Join(handlers, "\n"), "RequireSelfServicePayment", route.path)
	}
}
