package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

const (
	supplierRouteRootMutation   = "root_mutation"
	supplierRouteGateTransition = "gate_transition"
	supplierRouteScheduler      = "scheduler"
	supplierRouteReportRead     = "report_read"
	supplierRouteCommandRead    = "command_read"
	supplierRouteGeneralRead    = "general_read"
)

func TestSupplyChainRouteClassificationMatrixIsComplete(t *testing.T) {
	classified := map[string]string{
		"GET /api/supply-chain/commands/result":                      supplierRouteCommandRead,
		"GET /api/supply-chain/suppliers":                            supplierRouteGeneralRead,
		"GET /api/supply-chain/suppliers/:id":                        supplierRouteGeneralRead,
		"POST /api/supply-chain/suppliers":                           supplierRouteRootMutation,
		"PATCH /api/supply-chain/suppliers/:id":                      supplierRouteRootMutation,
		"POST /api/supply-chain/suppliers/:id/inactivate":            supplierRouteRootMutation,
		"GET /api/supply-chain/contracts":                            supplierRouteGeneralRead,
		"GET /api/supply-chain/contracts/:id":                        supplierRouteGeneralRead,
		"POST /api/supply-chain/contracts":                           supplierRouteRootMutation,
		"PATCH /api/supply-chain/contracts/:id":                      supplierRouteRootMutation,
		"POST /api/supply-chain/contracts/:id/inactivate":            supplierRouteRootMutation,
		"GET /api/supply-chain/contracts/:id/rates":                  supplierRouteGeneralRead,
		"POST /api/supply-chain/contracts/:id/rates":                 supplierRouteRootMutation,
		"GET /api/supply-chain/contracts/:id/inventory-adjustments":  supplierRouteGeneralRead,
		"POST /api/supply-chain/contracts/:id/inventory-adjustments": supplierRouteRootMutation,
		"GET /api/supply-chain/exclusions":                           supplierRouteGeneralRead,
		"POST /api/supply-chain/exclusions":                          supplierRouteRootMutation,
		"GET /api/supply-chain/channel-bindings":                     supplierRouteGeneralRead,
		"GET /api/supply-chain/channel-bindings/:channel_id":         supplierRouteGeneralRead,
		"PUT /api/supply-chain/channel-bindings/:channel_id":         supplierRouteRootMutation,
		"DELETE /api/supply-chain/channel-bindings/:channel_id":      supplierRouteRootMutation,
		"GET /api/supply-chain/reports/overview":                     supplierRouteReportRead,
		"GET /api/supply-chain/reports/trend":                        supplierRouteReportRead,
		"GET /api/supply-chain/reports/contracts":                    supplierRouteReportRead,
		"GET /api/supply-chain/reports/contracts/:id":                supplierRouteReportRead,
		"GET /api/supply-chain/reports/channels":                     supplierRouteReportRead,
		"GET /api/supply-chain/reports/breakdown":                    supplierRouteReportRead,
		"GET /api/supply-chain/reports/freshness":                    supplierRouteReportRead,
		"GET /api/supply-chain/reports/daily":                        supplierRouteReportRead,
		"POST /api/supply-chain/reports/daily/:date/rerun":           supplierRouteRootMutation,
		"GET /api/supply-chain/accounting/status":                    supplierRouteGeneralRead,
		"GET /api/supply-chain/accounting/readiness":                 supplierRouteGeneralRead,
		"POST /api/supply-chain/accounting/mutation-gate":            supplierRouteGateTransition,
		"POST /api/supply-chain/accounting/prepare":                  supplierRouteRootMutation,
		"POST /api/supply-chain/accounting/arm":                      supplierRouteRootMutation,
		"POST /api/supply-chain/accounting/activate":                 supplierRouteRootMutation,
		"POST /api/supply-chain/accounting/disable":                  supplierRouteRootMutation,
		"POST /api/supply-chain/accounting/degrade":                  supplierRouteRootMutation,
		"POST /api/supply-chain/accounting/gaps/resolve":             supplierRouteRootMutation,
		"POST /api/supply-chain/accounting/reactivate":               supplierRouteRootMutation,
		"POST /api/supply-chain/accounting/adopt-legacy":             supplierRouteRootMutation,
		"POST /api/supply-chain/daily-batches/catch-up":              supplierRouteScheduler,
		"GET /api/supply-chain/daily-batches/status":                 supplierRouteScheduler,
	}
	engine := newSupplyChainRouteTestEngine(t)
	found := make(map[string]bool, len(classified))
	for _, route := range engine.Routes() {
		if !strings.HasPrefix(route.Path, "/api/supply-chain") {
			continue
		}
		key := route.Method + " " + route.Path
		_, ok := classified[key]
		require.True(t, ok, "unclassified supply-chain route: %s", key)
		found[key] = true
	}
	require.Len(t, found, len(classified), "classification entries must exactly match registered routes")

	source, err := os.ReadFile("api-router.go")
	require.NoError(t, err)
	text := string(source)
	require.Contains(t, text, `supplyChainRoute.Use(middleware.FinanceAuth())`)
	require.Contains(t, text, `supplyChainRoute.GET("/reports/daily", controller.GetSupplyChainDailyReports)`)
	require.Contains(t, text, `supplyChainRoute.POST("/reports/daily/:date/rerun", supplierSupplyChainMutation(controller.RerunSupplyChainDailyReport)...)`)
	require.Contains(t, text, `accountingRoute.Use(middleware.FinanceAuth())`)
	require.Contains(t, text, `supplierBatchRoute.Use(middleware.SupplierBatchAuth())`)
}

func TestSupplyChainDailyReportReadFinanceAuthMatrix(t *testing.T) {
	engine := newSupplyChainRouteTestEngine(t)
	path := "/api/supply-chain/reports/daily?start_date=2026-07-20&end_date=2026-07-20"
	for _, role := range []int{common.RoleCommonUser, common.RoleAdminUser} {
		response := performSupplyChainRouteTestRequestAt(engine, supplyChainRouteTestCookies(t, engine, role), http.MethodGet, path, "")
		require.Equal(t, http.StatusOK, response.Code)
		require.Contains(t, response.Body.String(), `"success":false`)
	}
	root := performSupplyChainRouteTestRequestAt(engine, supplyChainRouteTestCookies(t, engine, common.RoleRootUser), http.MethodGet, path, "")
	require.Equal(t, http.StatusOK, root.Code, "Root report reads must not require fresh verification")
	require.Contains(t, root.Body.String(), `"success":true`)

	token := supplierBatchRouteTestToken(22)
	require.NoError(t, model.DB.AutoMigrate(&model.User{}))
	t.Setenv(middleware.SupplierBatchCurrentVerifierHashEnv, supplierBatchRouteTestVerifier(token))
	t.Setenv(middleware.SupplierBatchNextVerifierHashEnv, "")
	t.Setenv(middleware.SupplierBatchTrustedIdentityEnv, "supplier-route-runner")
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	scheduler := httptest.NewRecorder()
	engine.ServeHTTP(scheduler, request)
	require.Equal(t, http.StatusOK, scheduler.Code)
	require.Contains(t, scheduler.Body.String(), `"success":false`, "scheduler credentials must not cross the FinanceAuth boundary")
}
