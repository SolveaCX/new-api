package router

import (
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionCatalogMigrationRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := make(map[string]string)
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = route.Handler
	}
	expected := map[string]string{
		"POST /api/subscription/admin/catalog-migrations/preview":    "controller.AdminPreviewSubscriptionCatalogMigration",
		"POST /api/subscription/admin/catalog-migrations/apply":      "controller.AdminApplySubscriptionCatalogMigration",
		"GET /api/subscription/admin/catalog-migrations/:id":         "controller.AdminGetSubscriptionCatalogMigration",
		"POST /api/subscription/admin/catalog-migrations/:id/cancel": "controller.AdminCancelSubscriptionCatalogMigration",
	}
	for route, handler := range expected {
		registered, ok := routes[route]
		require.True(t, ok, "missing %s", route)
		require.Contains(t, registered, handler)
	}
}

func TestSubscriptionCatalogMigrationRoutesUseRootAndCriticalMiddleware(t *testing.T) {
	source, err := os.ReadFile("api-router.go")
	require.NoError(t, err)
	routerSource := string(source)

	require.Contains(t, routerSource, `subscriptionAdminRoute.Use(middleware.AdminAuth())`)
	require.Contains(t, routerSource, `subscriptionAdminRoute.POST("/catalog-migrations/preview", middleware.RootAuth(), middleware.CriticalRateLimit(), controller.AdminPreviewSubscriptionCatalogMigration)`)
	require.Contains(t, routerSource, `subscriptionAdminRoute.POST("/catalog-migrations/apply", middleware.RootAuth(), middleware.CriticalRateLimit(), controller.AdminApplySubscriptionCatalogMigration)`)
	require.Contains(t, routerSource, `subscriptionAdminRoute.GET("/catalog-migrations/:id", middleware.RootAuth(), controller.AdminGetSubscriptionCatalogMigration)`)
	require.Contains(t, routerSource, `subscriptionAdminRoute.POST("/catalog-migrations/:id/cancel", middleware.RootAuth(), middleware.CriticalRateLimit(), controller.AdminCancelSubscriptionCatalogMigration)`)
}
