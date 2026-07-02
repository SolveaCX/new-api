package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSetApiRouterRegistersWebsiteRankingsRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	for _, route := range engine.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/website/rankings" {
			return
		}
	}

	t.Fatalf("expected GET /api/website/rankings to be registered, routes: %v", engine.Routes())
}
