package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
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

func TestMcpOAuthRoutesResolveToConcreteHandlersAtRuntime(t *testing.T) {
	require.NoError(t, backendI18n.Init())
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("mcp-oauth-runtime"))))

	SetRouter(engine, ThemeAssets{
		DefaultIndexPage: []byte("<html>spa-index</html>"),
		ClassicIndexPage: []byte("<html>spa-index</html>"),
	})

	handlers := map[string]string{}
	for _, route := range engine.Routes() {
		handlers[route.Method+" "+route.Path] = route.Handler
	}
	require.Contains(t, handlers[http.MethodGet+" /.well-known/oauth-authorization-server"], "McpOAuthAuthorizationServerMetadata")
	require.Contains(t, handlers[http.MethodPost+" /oauth/register"], "McpOAuthRegisterClient")
	require.Contains(t, handlers[http.MethodPost+" /oauth/token"], "McpOAuthToken")
	require.Contains(t, handlers[http.MethodPost+" /oauth/revoke"], "McpOAuthRevoke")
	require.Contains(t, handlers[http.MethodGet+" /api/oauth/authorization-details"], "McpOAuthAuthorizationDetails")
	require.Contains(t, handlers[http.MethodPost+" /api/oauth/authorize"], "McpOAuthAuthorize")
	require.NotContains(t, handlers, http.MethodGet+" /oauth/authorize")

	metadata := httptest.NewRecorder()
	engine.ServeHTTP(metadata, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil))
	require.Equal(t, http.StatusOK, metadata.Code)
	require.Contains(t, metadata.Header().Get("Content-Type"), "application/json")
	require.Contains(t, metadata.Body.String(), `"issuer":"https://console.flatkey.ai"`)
	require.NotContains(t, metadata.Body.String(), "spa-index")

	spaAuthorize := httptest.NewRecorder()
	engine.ServeHTTP(spaAuthorize, httptest.NewRequest(http.MethodGet, "/oauth/authorize", nil))
	require.Equal(t, http.StatusOK, spaAuthorize.Code)
	require.Contains(t, spaAuthorize.Body.String(), "spa-index")

	unauthDetails := httptest.NewRecorder()
	engine.ServeHTTP(unauthDetails, httptest.NewRequest(http.MethodGet, "/api/oauth/authorization-details", nil))
	require.Equal(t, http.StatusUnauthorized, unauthDetails.Code)
	require.NotContains(t, unauthDetails.Body.String(), "OAuth login failed")

	unauthAuthorize := httptest.NewRecorder()
	engine.ServeHTTP(unauthAuthorize, httptest.NewRequest(http.MethodPost, "/api/oauth/authorize", strings.NewReader(`{}`)))
	require.Equal(t, http.StatusUnauthorized, unauthAuthorize.Code)
	require.NotContains(t, unauthAuthorize.Body.String(), "OAuth login failed")
}

func TestMcpOAuthPublicPostRoutesUseRuntimeBodyLimitAndContentTypeChain(t *testing.T) {
	require.NoError(t, backendI18n.Init())
	previousLimit := constant.AnonymousRequestBodyLimitKB
	constant.AnonymousRequestBodyLimitKB = 1
	t.Cleanup(func() { constant.AnonymousRequestBodyLimitKB = previousLimit })
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetMcpOAuthRouter(engine)

	oversized := httptest.NewRecorder()
	oversizedReq := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(strings.Repeat("x", int(common.GetAnonymousRequestBodyLimitBytes())+1)))
	oversizedReq.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(oversized, oversizedReq)
	require.Equal(t, http.StatusRequestEntityTooLarge, oversized.Code)

	tokenContentType := httptest.NewRecorder()
	tokenReq := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(`grant_type=authorization_code`))
	tokenReq.Header.Set("Content-Type", "text/plain")
	engine.ServeHTTP(tokenContentType, tokenReq)
	require.Equal(t, http.StatusBadRequest, tokenContentType.Code)
	require.Contains(t, tokenContentType.Body.String(), `"error":"invalid_request"`)
	require.NotContains(t, tokenContentType.Body.String(), "success")
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
