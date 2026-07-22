package router

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	backendi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newSupplyChainRouteTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	require.NoError(t, backendi18n.Init())
	gin.SetMode(gin.TestMode)
	previousRateLimit := common.GlobalApiRateLimitEnable
	common.GlobalApiRateLimitEnable = false
	t.Cleanup(func() { common.GlobalApiRateLimitEnable = previousRateLimit })

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("supply-chain-route-test"))))
	engine.GET("/test-login/:role", func(c *gin.Context) {
		role, _ := strconv.Atoi(c.Param("role"))
		session := sessions.Default(c)
		session.Set("username", "route-tester")
		session.Set("role", role)
		session.Set("id", 17)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	SetApiRouter(engine)
	return engine
}

func supplyChainRouteTestCookies(t *testing.T, engine *gin.Engine, role int) []*http.Cookie {
	t.Helper()
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/test-login/"+strconv.Itoa(role), nil))
	require.Equal(t, http.StatusNoContent, recorder.Code)
	return recorder.Result().Cookies()
}

func performSupplyChainRouteTestRequest(engine *gin.Engine, cookies []*http.Cookie) *httptest.ResponseRecorder {
	return performSupplyChainRouteTestRequestAt(engine, cookies, http.MethodGet, "/api/supply-chain/suppliers/not-an-id", "")
}

func performSupplyChainRouteTestRequestAt(engine *gin.Engine, cookies []*http.Cookie, method, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("New-Api-User", "17")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "route-permission-test")
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestSupplyChainSensitiveRoutesRequireAdmin(t *testing.T) {
	engine := newSupplyChainRouteTestEngine(t)
	userCookies := supplyChainRouteTestCookies(t, engine, common.RoleCommonUser)
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/supply-chain/commands/result?scope=supplier.invalid"},
		{method: http.MethodGet, path: "/api/supply-chain/reports/freshness"},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			unauthenticated := performSupplyChainRouteTestRequestAt(engine, nil, test.method, test.path, test.body)
			require.Equal(t, http.StatusUnauthorized, unauthenticated.Code)

			user := performSupplyChainRouteTestRequestAt(engine, userCookies, test.method, test.path, test.body)
			require.Equal(t, http.StatusOK, user.Code)
			require.Contains(t, user.Body.String(), `"success":false`)
		})
	}
}

func TestSupplyChainRoutesRequireAdmin(t *testing.T) {
	engine := newSupplyChainRouteTestEngine(t)

	unauthenticated := httptest.NewRecorder()
	engine.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/api/supply-chain/suppliers/not-an-id", nil))
	require.Equal(t, http.StatusUnauthorized, unauthenticated.Code)

	user := performSupplyChainRouteTestRequest(engine, supplyChainRouteTestCookies(t, engine, common.RoleCommonUser))
	require.Equal(t, http.StatusOK, user.Code)
	require.Contains(t, user.Body.String(), `"success":false`)

	admin := performSupplyChainRouteTestRequest(engine, supplyChainRouteTestCookies(t, engine, common.RoleAdminUser))
	require.Equal(t, http.StatusBadRequest, admin.Code, "admin must reach the supplier controller")
}

func TestSupplierDailyBatchCatchUpRouteRequiresRoot(t *testing.T) {
	require.NoError(t, backendi18n.Init())
	gin.SetMode(gin.TestMode)
	previousRateLimit := common.CriticalRateLimitEnable
	common.CriticalRateLimitEnable = false
	t.Cleanup(func() { common.CriticalRateLimitEnable = previousRateLimit })

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("supplier-batch-trigger-route-test"))))
	engine.GET("/test-login/:role", func(c *gin.Context) {
		role, _ := strconv.Atoi(c.Param("role"))
		session := sessions.Default(c)
		session.Set("username", "route-tester")
		session.Set("role", role)
		session.Set("id", 17)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	var calls int
	registerSupplierDailyBatchCatchUpRoute(engine.Group("/api"), func(c *gin.Context) {
		calls++
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	path := "/api/supply-chain/daily-batches/catch-up"

	unauthenticated := performSupplyChainRouteTestRequestAt(engine, nil, http.MethodPost, path, "")
	require.Equal(t, http.StatusUnauthorized, unauthenticated.Code)

	admin := performSupplyChainRouteTestRequestAt(engine, supplyChainRouteTestCookies(t, engine, common.RoleAdminUser), http.MethodPost, path, "")
	require.Equal(t, http.StatusOK, admin.Code)
	require.Contains(t, admin.Body.String(), `"success":false`)
	require.Zero(t, calls)

	root := performSupplyChainRouteTestRequestAt(engine, supplyChainRouteTestCookies(t, engine, common.RoleRootUser), http.MethodPost, path, "")
	require.Equal(t, http.StatusOK, root.Code)
	require.Equal(t, 1, calls)
}

func TestSupplyChainRouteRegistryHasNoCollisionsOrHardDeletes(t *testing.T) {
	engine := newSupplyChainRouteTestEngine(t)
	routes := engine.Routes()
	wantedHandlers := map[string]string{
		http.MethodPost + " /api/supply-chain/daily-batches/catch-up": "TriggerSupplierDailyBatchCatchUp",
		http.MethodGet + " /api/supply-chain/commands/result":         "GetSupplyChainCommandResult",
		http.MethodGet + " /api/supply-chain/reports/freshness":       "GetSupplyChainReportFreshness",
		http.MethodGet + " /api/supply-chain/reports/contracts/:id":   "GetSupplyChainReportContract",
	}
	found := make(map[string]string)
	for _, route := range routes {
		key := route.Method + " " + route.Path
		if expected, ok := wantedHandlers[key]; ok {
			found[key] = route.Handler
			require.Contains(t, route.Handler, expected)
		}
		if route.Method == http.MethodDelete && (strings.Contains(route.Path, "/suppliers") || strings.Contains(route.Path, "/contracts") || strings.Contains(route.Path, "/exclusions")) {
			t.Fatalf("append-only supply-chain resource unexpectedly exposes hard delete: %s", key)
		}
	}
	require.Len(t, found, len(wantedHandlers))
}

func TestSupplyChainWriteRoutesUseCriticalRateLimit(t *testing.T) {
	previousEnable := common.CriticalRateLimitEnable
	previousLimit := common.CriticalRateLimitNum
	previousDuration := common.CriticalRateLimitDuration
	previousRedis := common.RedisEnabled
	common.CriticalRateLimitEnable = true
	common.CriticalRateLimitNum = 1
	common.CriticalRateLimitDuration = 60
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.CriticalRateLimitEnable = previousEnable
		common.CriticalRateLimitNum = previousLimit
		common.CriticalRateLimitDuration = previousDuration
		common.RedisEnabled = previousRedis
	})

	engine := newSupplyChainRouteTestEngine(t)
	cookies := supplyChainRouteTestCookies(t, engine, common.RoleAdminUser)
	statuses := make([]int, 0, 2)
	for range 2 {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/supply-chain/suppliers", strings.NewReader(`{"name":"rate-limited"}`))
		request.RemoteAddr = "198.51.100.77:1234"
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("New-Api-User", "17")
		for _, cookie := range cookies {
			request.AddCookie(cookie)
		}
		engine.ServeHTTP(recorder, request)
		statuses = append(statuses, recorder.Code)
	}
	require.Equal(t, http.StatusBadRequest, statuses[0])
	require.Equal(t, http.StatusTooManyRequests, statuses[1])
}
