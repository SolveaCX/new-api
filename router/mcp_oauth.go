package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func SetMcpOAuthRouter(router *gin.Engine) {
	router.GET("/.well-known/oauth-authorization-server", middleware.RouteTag("mcp_oauth"), middleware.BodyStorageCleanup(), middleware.GlobalAPIRateLimit(), controller.McpOAuthAuthorizationServerMetadata)

	oauth := router.Group("/oauth")
	oauth.Use(middleware.RouteTag("mcp_oauth"))
	oauth.Use(middleware.BodyStorageCleanup())
	oauth.Use(middleware.GlobalAPIRateLimit())
	anonymousRequestBodyLimit := middleware.AnonymousRequestBodyLimit()
	{
		oauth.GET("/jwks", controller.McpOAuthJWKS)
		oauth.POST("/register", anonymousRequestBodyLimit, controller.McpOAuthRegisterClient)
		oauth.POST("/token", anonymousRequestBodyLimit, controller.McpOAuthToken)
		oauth.POST("/revoke", anonymousRequestBodyLimit, controller.McpOAuthRevoke)
	}
}
