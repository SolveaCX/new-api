package router

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func TestDeleteHistoryLogsRequiresFinanceAuthAndSecureVerification(t *testing.T) {
	engine := newSupplyChainRouteTestEngine(t)
	path := "/api/log/?target_timestamp=0"

	unauthenticated := performSupplyChainRouteTestRequestAt(engine, nil, http.MethodDelete, path, "")
	require.Equal(t, http.StatusUnauthorized, unauthenticated.Code)

	admin := performSupplyChainRouteTestRequestAt(
		engine,
		supplyChainRouteTestCookies(t, engine, common.RoleAdminUser),
		http.MethodDelete,
		path,
		"",
	)
	require.Equal(t, http.StatusForbidden, admin.Code)

	unverifiedRoot := performSupplyChainRouteTestRequestAt(
		engine,
		supplyChainRouteTestCookies(t, engine, common.RoleRootUser),
		http.MethodDelete,
		path,
		"",
	)
	require.Equal(t, http.StatusForbidden, unverifiedRoot.Code)
	require.Contains(t, unverifiedRoot.Body.String(), "VERIFICATION_REQUIRED")

	root := performSupplyChainRouteTestRequestAt(
		engine,
		supplyChainRouteTestCookiesWithVerification(t, engine, common.RoleRootUser, true),
		http.MethodDelete,
		path,
		"",
	)
	require.Equal(t, http.StatusOK, root.Code)
	require.Contains(t, root.Body.String(), "target timestamp is required", "Root must reach the controller")
}
