package router

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDataToolsOAuthRoutesUseDedicatedAuthMiddlewareOnly(t *testing.T) {
	source, err := os.ReadFile("api-router.go")
	require.NoError(t, err)
	text := string(source)

	searchRoute := `dataToolRoute.GET("", middleware.DataToolsAuth("tools:search", true), controller.ListDataTools)`
	inspectRoute := `dataToolRoute.GET("/inspect", middleware.DataToolsAuth("tools:read", true), controller.InspectDataTool)`
	runRoute := `dataToolRoute.POST("/run", middleware.DataToolsAuth("tools:execute", false), controller.RunDataTool)`

	require.Contains(t, text, searchRoute)
	require.Contains(t, text, inspectRoute)
	require.Contains(t, text, runRoute)
	require.NotContains(t, text, `dataToolRoute.GET("", middleware.TokenOrUserAuth(), controller.ListDataTools)`)
	require.NotContains(t, text, `dataToolRoute.GET("/inspect", middleware.TokenOrUserAuth(), controller.InspectDataTool)`)
	require.NotContains(t, text, `dataToolRoute.POST("/run", middleware.TokenAuth(), controller.RunDataTool)`)

	dataToolsStart := strings.Index(text, `dataToolRoute := apiRouter.Group("/data-tools")`)
	require.NotEqual(t, -1, dataToolsStart)
	perfMetricsStart := strings.Index(text, `perfMetricsRoute := apiRouter.Group("/perf-metrics")`)
	require.NotEqual(t, -1, perfMetricsStart)
	dataToolsBlock := text[dataToolsStart:perfMetricsStart]
	require.NotContains(t, dataToolsBlock, "middleware.TokenAuth()")
	require.NotContains(t, dataToolsBlock, "middleware.TokenOrUserAuth()")
}

func TestDataToolsOAuthDoesNotAttachToNonDataToolsRoutes(t *testing.T) {
	source, err := os.ReadFile("api-router.go")
	require.NoError(t, err)
	text := string(source)

	dataToolsStart := strings.Index(text, `dataToolRoute := apiRouter.Group("/data-tools")`)
	require.NotEqual(t, -1, dataToolsStart)
	perfMetricsStart := strings.Index(text, `perfMetricsRoute := apiRouter.Group("/perf-metrics")`)
	require.NotEqual(t, -1, perfMetricsStart)
	withoutDataToolsBlock := text[:dataToolsStart] + text[perfMetricsStart:]

	require.NotContains(t, withoutDataToolsBlock, "middleware.DataToolsAuth(")
}
