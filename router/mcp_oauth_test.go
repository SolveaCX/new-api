package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	backendI18n "github.com/QuantumNous/new-api/i18n"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMcpOAuthRoutesAreRegisteredBeforeWebFallbackAndRequireExpectedAuth(t *testing.T) {
	require.NoError(t, backendI18n.Init())
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("mcp-oauth-routes"))))

	SetRouter(engine, ThemeAssets{})

	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	for _, want := range []string{
		http.MethodGet + " /.well-known/oauth-authorization-server",
		http.MethodGet + " /oauth/jwks",
		http.MethodPost + " /oauth/register",
		http.MethodPost + " /oauth/token",
		http.MethodPost + " /oauth/revoke",
		http.MethodGet + " /api/oauth/authorization-details",
		http.MethodPost + " /api/oauth/authorize",
		http.MethodGet + " /api/user/connected-apps",
		http.MethodPost + " /api/user/connected-apps/:grant_id/revoke",
	} {
		require.True(t, routes[want], "route %s missing from %v", want, engine.Routes())
	}
	require.False(t, routes[http.MethodGet+" /oauth/authorize"], "SPA-owned /oauth/authorize must not register a backend GET handler")
	require.False(t, routes[http.MethodGet+" /.well-known/oauth-protected-resource"])

	unauthDetails := httptest.NewRecorder()
	engine.ServeHTTP(unauthDetails, httptest.NewRequest(http.MethodGet, "/api/oauth/authorization-details", nil))
	require.Equal(t, http.StatusUnauthorized, unauthDetails.Code)

	unauthApps := httptest.NewRecorder()
	engine.ServeHTTP(unauthApps, httptest.NewRequest(http.MethodGet, "/api/user/connected-apps", nil))
	require.Equal(t, http.StatusUnauthorized, unauthApps.Code)
}

func TestMcpOAuthStaticApiRoutesPrecedeLegacyOAuthWildcard(t *testing.T) {
	source, err := os.ReadFile("api-router.go")
	require.NoError(t, err)
	text := string(source)

	require.Contains(t, text, `apiRouter.GET("/oauth/authorization-details", middleware.UserAuth(), controller.McpOAuthAuthorizationDetails)`)
	require.Contains(t, text, `apiRouter.POST("/oauth/authorize", middleware.UserAuth(), controller.McpOAuthAuthorize)`)
	require.Contains(t, text, `apiRouter.GET("/oauth/:provider", middleware.CriticalRateLimit(), controller.HandleOAuth)`)
	require.Less(t, strings.Index(text, `apiRouter.GET("/oauth/authorization-details"`), strings.Index(text, `apiRouter.GET("/oauth/:provider"`))
	require.Less(t, strings.Index(text, `apiRouter.POST("/oauth/authorize"`), strings.Index(text, `apiRouter.GET("/oauth/:provider"`))
}

func TestMcpOAuthPublicRoutesUseRootPostMiddlewareAndRegisterBeforeWeb(t *testing.T) {
	sourceMain, err := os.ReadFile("main.go")
	require.NoError(t, err)
	mainText := string(sourceMain)
	require.Contains(t, mainText, "SetMcpOAuthRouter(router)")
	require.Less(t, strings.Index(mainText, "SetMcpOAuthRouter(router)"), strings.Index(mainText, "SetWebRouter(router, assets)"))

	sourceOAuth, err := os.ReadFile("mcp_oauth.go")
	require.NoError(t, err)
	oauthText := string(sourceOAuth)
	require.Contains(t, oauthText, `middleware.RouteTag("mcp_oauth")`)
	require.Contains(t, oauthText, "middleware.BodyStorageCleanup()")
	require.Contains(t, oauthText, "middleware.GlobalAPIRateLimit()")
	require.Contains(t, oauthText, `router.GET("/.well-known/oauth-authorization-server", middleware.RouteTag("mcp_oauth"), middleware.BodyStorageCleanup(), middleware.GlobalAPIRateLimit(), controller.McpOAuthAuthorizationServerMetadata)`)
	require.Contains(t, oauthText, "middleware.AnonymousRequestBodyLimit()")
	require.Contains(t, oauthText, `oauth.POST("/register", anonymousRequestBodyLimit, controller.McpOAuthRegisterClient)`)
	require.Contains(t, oauthText, `oauth.POST("/token", anonymousRequestBodyLimit, controller.McpOAuthToken)`)
	require.Contains(t, oauthText, `oauth.POST("/revoke", anonymousRequestBodyLimit, controller.McpOAuthRevoke)`)
}
