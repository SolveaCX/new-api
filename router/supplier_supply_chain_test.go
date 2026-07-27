package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	backendi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newSupplyChainRouteTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	require.NoError(t, backendi18n.Init())
	gin.SetMode(gin.TestMode)
	previousRateLimit := common.GlobalApiRateLimitEnable
	common.GlobalApiRateLimitEnable = false
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Option{}, &model.Channel{}, &model.Log{},
		&model.UpstreamSupplier{}, &model.SupplierContract{}, &model.SupplierContractRateVersion{},
		&model.SupplierChannelBindingVersion{}, &model.SupplierInventoryAdjustment{}, &model.SupplierStatisticsExclusionRule{},
		&model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{},
	))
	require.NoError(t, model.EnsureSupplierAccountingFactSchema(db))
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		common.GlobalApiRateLimitEnable = previousRateLimit
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("supply-chain-route-test"))))
	engine.GET("/test-login/:role/:verified", func(c *gin.Context) {
		role, _ := strconv.Atoi(c.Param("role"))
		session := sessions.Default(c)
		session.Set("username", "route-tester")
		session.Set("role", role)
		session.Set("id", 17)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		if c.Param("verified") == "true" {
			session.Set(middleware.SecureVerificationSessionKey, time.Now().Unix())
		}
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	SetApiRouter(engine)
	return engine
}

func TestPendingAccountingFactRoutesRequireRoot(t *testing.T) {
	engine := newSupplyChainRouteTestEngine(t)
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	day := time.Now().In(location).Format("2006-01-02")
	path := "/api/supply-chain/accounting-facts/pending?date=" + day + "&limit=10"

	admin := performSupplyChainRouteTestRequestAt(engine, supplyChainRouteTestCookies(t, engine, common.RoleAdminUser), http.MethodGet, path, "")
	require.Equal(t, http.StatusForbidden, admin.Code)
	rootCookies := supplyChainRouteTestCookies(t, engine, common.RoleRootUser)
	root := performSupplyChainRouteTestRequestAt(engine, rootCookies, http.MethodGet, path, "")
	require.Equal(t, http.StatusOK, root.Code)
	require.Contains(t, root.Body.String(), `"items"`)

	fact, err := model.PrepareSupplierAccountingFact(context.Background(), model.LOG_DB, model.SupplierAccountingFactPrepare{
		ParentRequestId: "req-route-resolve", SupplierId: 1, ContractId: 2, BindingVersionId: 3, RateVersionId: 4,
		ChannelId: 5, ModelName: "gpt-test", CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1), CutoverAt: 1,
	})
	require.NoError(t, err)
	resolvePath := "/api/supply-chain/accounting-facts/" + fact.AttemptId + "/resolve"
	body := `{"action":"void","reason":"verified rejection","evidence":"ticket-route"}`
	resolved := performSupplyChainRouteTestRequestAt(engine, rootCookies, http.MethodPost, resolvePath, body)
	require.Equal(t, http.StatusOK, resolved.Code)
}

func supplyChainRouteTestCookies(t *testing.T, engine *gin.Engine, role int) []*http.Cookie {
	return supplyChainRouteTestCookiesWithVerification(t, engine, role, false)
}

func supplyChainRouteTestCookiesWithVerification(t *testing.T, engine *gin.Engine, role int, verified bool) []*http.Cookie {
	t.Helper()
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/test-login/"+strconv.Itoa(role)+"/"+strconv.FormatBool(verified), nil))
	require.Equal(t, http.StatusNoContent, recorder.Code)
	return recorder.Result().Cookies()
}

func performSupplyChainRouteTestRequestAt(engine *gin.Engine, cookies []*http.Cookie, method, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("New-Api-User", "17")
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestSupplyChainRoutesRequireFinanceAccess(t *testing.T) {
	engine := newSupplyChainRouteTestEngine(t)
	path := "/api/supply-chain/suppliers/not-an-id"
	require.Equal(t, http.StatusUnauthorized, performSupplyChainRouteTestRequestAt(engine, nil, http.MethodGet, path, "").Code)
	for _, role := range []int{common.RoleCommonUser, common.RoleAdminUser} {
		response := performSupplyChainRouteTestRequestAt(engine, supplyChainRouteTestCookies(t, engine, role), http.MethodGet, path, "")
		require.Equal(t, http.StatusForbidden, response.Code)
		require.Contains(t, response.Body.String(), `"success":false`)
	}
	root := performSupplyChainRouteTestRequestAt(engine, supplyChainRouteTestCookies(t, engine, common.RoleRootUser), http.MethodGet, path, "")
	require.Equal(t, http.StatusBadRequest, root.Code)
}

func TestSupplyChainRouteRegistryHasNoHardDeletes(t *testing.T) {
	for _, route := range newSupplyChainRouteTestEngine(t).Routes() {
		if route.Method == http.MethodDelete && (strings.Contains(route.Path, "/suppliers") || strings.Contains(route.Path, "/contracts") || strings.Contains(route.Path, "/exclusions")) {
			t.Fatalf("append-only supply-chain resource exposes hard delete: %s %s", route.Method, route.Path)
		}
	}
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
	cookies := supplyChainRouteTestCookies(t, engine, common.RoleRootUser)
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
	require.Equal(t, http.StatusOK, statuses[0])
	require.Equal(t, http.StatusTooManyRequests, statuses[1])
}
