package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	backendi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newSupplierAccountingRouteTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	require.NoError(t, backendi18n.Init())
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=busy_timeout(5000)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.SupplierAdminCommand{}, &model.SupplierAccountingCoverageGap{}, &model.SupplierUsageDailyBatchRun{}))
	require.NoError(t, model.MigrateSupplierAdminCommandLedger(db))
	require.NoError(t, model.FinalizeSupplierAdminCommandLedgerMigration(db))
	previousDB := model.DB
	model.DB = db
	previousCritical := common.CriticalRateLimitEnable
	common.CriticalRateLimitEnable = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.CriticalRateLimitEnable = previousCritical
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("supplier-accounting-route-test"))))
	engine.GET("/login/:role/:verified", func(c *gin.Context) {
		role, _ := strconv.Atoi(c.Param("role"))
		session := sessions.Default(c)
		session.Set("username", "accounting-route-tester")
		session.Set("role", role)
		session.Set("id", 91)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		if c.Param("verified") == "true" {
			session.Set(middleware.SecureVerificationSessionKey, time.Now().Unix())
		}
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	registerSupplierAccountingControlRoutes(engine.Group("/api"))
	return engine
}

func supplierAccountingRouteCookies(t *testing.T, engine *gin.Engine, role int, verified bool) []*http.Cookie {
	t.Helper()
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/login/"+strconv.Itoa(role)+"/"+strconv.FormatBool(verified), nil))
	require.Equal(t, http.StatusNoContent, recorder.Code)
	return recorder.Result().Cookies()
}

func performSupplierAccountingRouteRequest(engine *gin.Engine, cookies []*http.Cookie, method, path, body, key string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("New-Api-User", "91")
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	for _, value := range cookies {
		request.AddCookie(value)
	}
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestSupplierAccountingRoutesEnforceFinanceGateAndVerificationClasses(t *testing.T) {
	engine := newSupplierAccountingRouteTestEngine(t)
	adminVerified := supplierAccountingRouteCookies(t, engine, common.RoleAdminUser, true)
	rootUnverified := supplierAccountingRouteCookies(t, engine, common.RoleRootUser, false)
	rootVerified := supplierAccountingRouteCookies(t, engine, common.RoleRootUser, true)

	unauthenticated := performSupplierAccountingRouteRequest(engine, nil, http.MethodGet, "/api/supply-chain/accounting/status", "", "")
	require.Equal(t, http.StatusUnauthorized, unauthenticated.Code)
	admin := performSupplierAccountingRouteRequest(engine, adminVerified, http.MethodGet, "/api/supply-chain/accounting/status", "", "")
	require.Equal(t, http.StatusOK, admin.Code)
	require.Contains(t, admin.Body.String(), `"success":false`)
	rootRead := performSupplierAccountingRouteRequest(engine, rootUnverified, http.MethodGet, "/api/supply-chain/accounting/status", "", "")
	require.Equal(t, http.StatusOK, rootRead.Code)
	require.Contains(t, rootRead.Body.String(), `"success":true`)

	prepareBody := `{"expected_state_version":0,"reason":"prepare","accepted_capability_versions":[1]}`
	gateClosed := performSupplierAccountingRouteRequest(engine, rootVerified, http.MethodPost, "/api/supply-chain/accounting/prepare", prepareBody, "prepare-route")
	require.Equal(t, http.StatusLocked, gateClosed.Code)
	require.Contains(t, gateClosed.Body.String(), middleware.SupplierMutationGateDisabledCode)

	gateUnverified := performSupplierAccountingRouteRequest(engine, rootUnverified, http.MethodPost, "/api/supply-chain/accounting/mutation-gate", `{"expected_state_version":0,"reason":"enable","enabled":true}`, "gate-route")
	require.Equal(t, http.StatusForbidden, gateUnverified.Code, "gate transition bypasses the gate but never fresh verification")

	gateAdmin := performSupplierAccountingRouteRequest(engine, adminVerified, http.MethodPost, "/api/supply-chain/accounting/mutation-gate", `{"expected_state_version":0,"reason":"enable","enabled":true}`, "gate-route")
	require.Equal(t, http.StatusOK, gateAdmin.Code)
	require.Contains(t, gateAdmin.Body.String(), `"success":false`)

	gateEnabled := performSupplierAccountingRouteRequest(engine, rootVerified, http.MethodPost, "/api/supply-chain/accounting/mutation-gate", `{"expected_state_version":0,"reason":"enable","enabled":true}`, "gate-route")
	require.Equal(t, http.StatusOK, gateEnabled.Code)
	require.Contains(t, gateEnabled.Body.String(), `"enabled":true`)

	prepareUnverified := performSupplierAccountingRouteRequest(engine, rootUnverified, http.MethodPost, "/api/supply-chain/accounting/prepare", prepareBody, "prepare-route")
	require.Equal(t, http.StatusForbidden, prepareUnverified.Code)
	prepared := performSupplierAccountingRouteRequest(engine, rootVerified, http.MethodPost, "/api/supply-chain/accounting/prepare", prepareBody, "prepare-route")
	require.Equal(t, http.StatusOK, prepared.Code)
	require.Contains(t, prepared.Body.String(), `"phase":"shadow"`)
}

func TestSupplierAccountingRouteRegistryIsCompleteAndNamespaced(t *testing.T) {
	engine := newSupplierAccountingRouteTestEngine(t)
	wanted := map[string]string{
		http.MethodGet + " /api/supply-chain/accounting/status":         "GetSupplierAccountingStatus",
		http.MethodGet + " /api/supply-chain/accounting/readiness":      "GetSupplierAccountingReadiness",
		http.MethodPost + " /api/supply-chain/accounting/mutation-gate": "ToggleSupplierAccountingMutationGate",
		http.MethodPost + " /api/supply-chain/accounting/prepare":       "PrepareSupplierAccounting",
		http.MethodPost + " /api/supply-chain/accounting/arm":           "ArmSupplierAccounting",
		http.MethodPost + " /api/supply-chain/accounting/activate":      "ActivateSupplierAccounting",
		http.MethodPost + " /api/supply-chain/accounting/disable":       "DisableSupplierAccountingBeforeCutover",
		http.MethodPost + " /api/supply-chain/accounting/degrade":       "DegradeSupplierAccounting",
		http.MethodPost + " /api/supply-chain/accounting/gaps/resolve":  "ResolveSupplierAccountingGap",
		http.MethodPost + " /api/supply-chain/accounting/reactivate":    "ReactivateSupplierAccounting",
		http.MethodPost + " /api/supply-chain/accounting/adopt-legacy":  "AdoptLegacySupplierAccounting",
	}
	found := make(map[string]bool, len(wanted))
	for _, route := range engine.Routes() {
		key := route.Method + " " + route.Path
		if handler, ok := wanted[key]; ok {
			require.Contains(t, route.Handler, handler)
			found[key] = true
		}
	}
	require.Len(t, found, len(wanted))
}

func TestSupplierAccountingRouteMiddlewareClassificationIsStatic(t *testing.T) {
	source, err := os.ReadFile("api-router.go")
	require.NoError(t, err)
	text := string(source)
	require.Contains(t, text, `accountingRoute.Use(middleware.FinanceAuth())`)
	require.Contains(t, text, `accountingRoute.POST("/mutation-gate", middleware.CriticalRateLimit(), middleware.SecureVerificationRequired(), controller.ToggleSupplierAccountingMutationGate)`)
	require.Contains(t, text, `mutationRoute.Use(middleware.CriticalRateLimit(), middleware.SupplierMutationGate(), middleware.SecureVerificationRequired())`)
	require.Contains(t, text, `supplyChainRoute.POST("/reports/daily/:date/rerun", supplierSupplyChainMutation(controller.RerunSupplyChainDailyReport)...)`)
	require.Contains(t, text, `supplierBatchRoute.Use(middleware.SupplierBatchAuth())`)
	require.Contains(t, text, `supplierBatchRoute.POST("/catch-up", middleware.CriticalRateLimit(), catchUpHandler)`)
	require.Contains(t, text, `supplierBatchRoute.GET("/status", statusHandler)`)
	require.NotContains(t, text, `supplierBatchRoute.Use(middleware.RootAuth())`)
	require.NotContains(t, text, `accountingRoute.POST("/mutation-gate", middleware.SupplierMutationGate()`)
	require.Equal(t, 13, strings.Count(text, "supplierSupplyChainMutation("), "twelve mutation routes plus the helper definition must remain classified")

	expected := map[string]string{
		http.MethodPost + " /api/supply-chain/suppliers":                           "CreateSupplyChainSupplier",
		http.MethodPatch + " /api/supply-chain/suppliers/:id":                      "UpdateSupplyChainSupplier",
		http.MethodPost + " /api/supply-chain/suppliers/:id/inactivate":            "InactivateSupplyChainSupplier",
		http.MethodPost + " /api/supply-chain/contracts":                           "CreateSupplyChainContract",
		http.MethodPatch + " /api/supply-chain/contracts/:id":                      "UpdateSupplyChainContract",
		http.MethodPost + " /api/supply-chain/contracts/:id/inactivate":            "InactivateSupplyChainContract",
		http.MethodPost + " /api/supply-chain/contracts/:id/rates":                 "CreateSupplyChainRateVersion",
		http.MethodPost + " /api/supply-chain/contracts/:id/inventory-adjustments": "CreateSupplyChainInventoryAdjustment",
		http.MethodPost + " /api/supply-chain/exclusions":                          "CreateSupplyChainExclusionRule",
		http.MethodPut + " /api/supply-chain/channel-bindings/:channel_id":         "BindSupplyChainChannel",
		http.MethodDelete + " /api/supply-chain/channel-bindings/:channel_id":      "UnbindSupplyChainChannel",
		http.MethodPost + " /api/supply-chain/accounting/mutation-gate":            "ToggleSupplierAccountingMutationGate",
		http.MethodPost + " /api/supply-chain/accounting/prepare":                  "PrepareSupplierAccounting",
		http.MethodPost + " /api/supply-chain/accounting/arm":                      "ArmSupplierAccounting",
		http.MethodPost + " /api/supply-chain/accounting/activate":                 "ActivateSupplierAccounting",
		http.MethodPost + " /api/supply-chain/accounting/disable":                  "DisableSupplierAccountingBeforeCutover",
		http.MethodPost + " /api/supply-chain/accounting/degrade":                  "DegradeSupplierAccounting",
		http.MethodPost + " /api/supply-chain/accounting/gaps/resolve":             "ResolveSupplierAccountingGap",
		http.MethodPost + " /api/supply-chain/accounting/reactivate":               "ReactivateSupplierAccounting",
		http.MethodPost + " /api/supply-chain/accounting/adopt-legacy":             "AdoptLegacySupplierAccounting",
		http.MethodPost + " /api/supply-chain/daily-batches/catch-up":              "TriggerSupplierDailyBatchCatchUp",
		http.MethodPost + " /api/supply-chain/reports/daily/:date/rerun":           "RerunSupplyChainDailyReport",
	}
	engine := newSupplyChainRouteTestEngine(t)
	found := make(map[string]bool, len(expected))
	for _, route := range engine.Routes() {
		if !strings.HasPrefix(route.Path, "/api/supply-chain") || route.Method == http.MethodGet || route.Method == http.MethodHead || route.Method == http.MethodOptions {
			continue
		}
		key := route.Method + " " + route.Path
		handler, classified := expected[key]
		require.True(t, classified, "unclassified supply-chain mutation route: %s", key)
		require.Contains(t, route.Handler, handler)
		found[key] = true
	}
	require.Len(t, found, len(expected), "every classified mutation route must remain registered")
}

func TestExistingSupplyChainMutationClassesRequireGateAndFreshVerification(t *testing.T) {
	engine := newSupplyChainRouteTestEngine(t)
	rootVerified := supplyChainRouteTestCookiesWithVerification(t, engine, common.RoleRootUser, true)
	rootUnverified := supplyChainRouteTestCookies(t, engine, common.RoleRootUser)
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/supply-chain/suppliers", body: `{"name":"supplier"}`},
		{method: http.MethodPatch, path: "/api/supply-chain/suppliers/1", body: `{"name":"supplier"}`},
		{method: http.MethodPost, path: "/api/supply-chain/suppliers/1/inactivate", body: `{}`},
		{method: http.MethodPost, path: "/api/supply-chain/contracts", body: `{"supplier_id":1,"name":"contract"}`},
		{method: http.MethodPatch, path: "/api/supply-chain/contracts/1", body: `{"name":"contract"}`},
		{method: http.MethodPost, path: "/api/supply-chain/contracts/1/inactivate", body: `{}`},
		{method: http.MethodPost, path: "/api/supply-chain/contracts/1/rates", body: `{"procurement_multiplier_ppm":1000000,"reason":"rate"}`},
		{method: http.MethodPost, path: "/api/supply-chain/contracts/1/inventory-adjustments", body: `{"delta_micro_usd":1,"type":"initial","reason":"inventory"}`},
		{method: http.MethodPost, path: "/api/supply-chain/exclusions", body: `{"user_id":1,"action":"exclude","reason":"internal"}`},
		{method: http.MethodPut, path: "/api/supply-chain/channel-bindings/1", body: `{"contract_id":1,"expected_contract_id":null}`},
		{method: http.MethodDelete, path: "/api/supply-chain/channel-bindings/1", body: `{"expected_contract_id":1}`},
		{method: http.MethodPost, path: "/api/supply-chain/reports/daily/2026-07-22/rerun", body: `{"reason":"retry incomplete day","expected_published_fence_token":7}`},
	}

	_, err := model.CASSupplierAccountingMutationState(model.DB, 1, false, 91, "test disabled gate", time.Now().Unix())
	require.NoError(t, err)
	for _, testCase := range tests {
		disabled := performSupplyChainRouteTestRequestAt(engine, rootVerified, testCase.method, testCase.path, testCase.body)
		require.Equal(t, http.StatusLocked, disabled.Code, testCase.method+" "+testCase.path)
		require.Contains(t, disabled.Body.String(), middleware.SupplierMutationGateDisabledCode)
	}

	_, err = model.CASSupplierAccountingMutationState(model.DB, 2, true, 91, "test enabled gate", time.Now().Unix())
	require.NoError(t, err)
	for _, testCase := range tests {
		unverified := performSupplyChainRouteTestRequestAt(engine, rootUnverified, testCase.method, testCase.path, testCase.body)
		require.Equal(t, http.StatusForbidden, unverified.Code, testCase.method+" "+testCase.path)
		require.Contains(t, unverified.Body.String(), "VERIFICATION_REQUIRED")
	}
}
