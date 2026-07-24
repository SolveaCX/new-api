package router

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSupplyChainRouteRegistryMatchesCurrentCRUDAndReportSurface(t *testing.T) {
	expected := map[string]bool{
		"GET /api/supply-chain/suppliers":                            true,
		"GET /api/supply-chain/suppliers/:id":                        true,
		"POST /api/supply-chain/suppliers":                           true,
		"PATCH /api/supply-chain/suppliers/:id":                      true,
		"POST /api/supply-chain/suppliers/:id/inactivate":            true,
		"GET /api/supply-chain/contracts":                            true,
		"GET /api/supply-chain/contracts/:id":                        true,
		"POST /api/supply-chain/contracts":                           true,
		"PATCH /api/supply-chain/contracts/:id":                      true,
		"POST /api/supply-chain/contracts/:id/inactivate":            true,
		"GET /api/supply-chain/contracts/:id/rates":                  true,
		"POST /api/supply-chain/contracts/:id/rates":                 true,
		"GET /api/supply-chain/contracts/:id/inventory-adjustments":  true,
		"POST /api/supply-chain/contracts/:id/inventory-adjustments": true,
		"GET /api/supply-chain/exclusions":                           true,
		"POST /api/supply-chain/exclusions":                          true,
		"GET /api/supply-chain/channel-bindings":                     true,
		"GET /api/supply-chain/channel-bindings/:channel_id":         true,
		"PUT /api/supply-chain/channel-bindings/:channel_id":         true,
		"DELETE /api/supply-chain/channel-bindings/:channel_id":      true,
		"GET /api/supply-chain/reports/overview":                     true,
		"GET /api/supply-chain/reports/trend":                        true,
		"GET /api/supply-chain/reports/contracts":                    true,
		"GET /api/supply-chain/reports/contracts/:id":                true,
		"GET /api/supply-chain/reports/channels":                     true,
		"GET /api/supply-chain/reports/breakdown":                    true,
	}
	engine := newSupplyChainRouteTestEngine(t)
	found := make(map[string]bool, len(expected))
	for _, route := range engine.Routes() {
		if !strings.HasPrefix(route.Path, "/api/supply-chain") {
			continue
		}
		key := route.Method + " " + route.Path
		require.True(t, expected[key], "obsolete or unexpected supply-chain route: %s", key)
		found[key] = true
	}
	require.Equal(t, expected, found)
}

func TestSupplyChainRouteUsesFinanceAuthBoundary(t *testing.T) {
	source, err := os.ReadFile("api-router.go")
	require.NoError(t, err)
	text := string(source)
	require.Contains(t, text, `supplyChainRoute.Use(middleware.FinanceAuth())`)
}
