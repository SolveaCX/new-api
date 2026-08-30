package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegistrationCaptchaRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	wanted := map[string]string{
		"/api/registration/captcha":        http.MethodGet,
		"/api/registration/captcha/verify": http.MethodPost,
	}
	for _, route := range engine.Routes() {
		if method, ok := wanted[route.Path]; ok && method == route.Method {
			delete(wanted, route.Path)
		}
	}
	if len(wanted) != 0 {
		t.Fatalf("registration captcha routes were not registered: %v", wanted)
	}
}
