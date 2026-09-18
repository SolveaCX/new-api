package router

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSetRelayRouterRegistersOpenRouterDecisionsRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetRelayRouter(engine)

	for _, route := range engine.Routes() {
		if route.Method == "POST" && route.Path == "/api/alpha/decisions" {
			return
		}
	}

	t.Fatalf("missing route POST /api/alpha/decisions (all routes: %v)", engine.Routes())
}
