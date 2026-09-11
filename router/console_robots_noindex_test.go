package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestConsoleMarketplaceRetainsNoindex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetWebRouter(engine, ThemeAssets{
		DefaultIndexPage: []byte(`<!doctype html><html><head></head><body></body></html>`),
		ClassicIndexPage: []byte(`<!doctype html><html><head></head><body></body></html>`),
	})
	for _, path := range []string{"/robots.txt", "/api-marketplace", "/api-marketplace/", "/api-marketplace?from=search"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "https://console.flatkey.ai"+path, nil)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("X-Robots-Tag"); got != "noindex, nofollow" {
				t.Fatalf("X-Robots-Tag=%q, want noindex, nofollow", got)
			}
		})
	}
}
