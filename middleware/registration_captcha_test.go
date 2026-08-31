package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegistrationCaptchaCheckRejectsMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	reached := false
	engine.POST("/register", RegistrationCaptchaCheck(), func(c *gin.Context) {
		reached = true
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader("{}"))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if reached {
		t.Fatal("registration handler must not run without a graphical captcha token")
	}
	if !strings.Contains(response.Body.String(), "registration_captcha.required") &&
		!strings.Contains(response.Body.String(), "图形验证") &&
		!strings.Contains(response.Body.String(), "graphical verification") {
		t.Fatalf("unexpected missing captcha response: %s", response.Body.String())
	}
}
