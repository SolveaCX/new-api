package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegistrationEmailVerificationStatusRateLimitUsesSeparateBucket(t *testing.T) {
	previousRedisEnabled := common.RedisEnabled
	previousCriticalEnabled := common.CriticalRateLimitEnable
	previousCriticalLimit := common.CriticalRateLimitNum
	common.RedisEnabled = false
	common.CriticalRateLimitEnable = true
	common.CriticalRateLimitNum = 1
	t.Cleanup(func() {
		common.RedisEnabled = previousRedisEnabled
		common.CriticalRateLimitEnable = previousCriticalEnabled
		common.CriticalRateLimitNum = previousCriticalLimit
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/status", RegistrationEmailVerificationStatusRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	engine.POST("/critical", CriticalRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	statusRequest := httptest.NewRequest(http.MethodPost, "/status", nil)
	statusRequest.RemoteAddr = "192.0.2.10:1234"
	statusResponse := httptest.NewRecorder()
	engine.ServeHTTP(statusResponse, statusRequest)
	require.Equal(t, http.StatusNoContent, statusResponse.Code)

	criticalRequest := httptest.NewRequest(http.MethodPost, "/critical", nil)
	criticalRequest.RemoteAddr = "192.0.2.10:1234"
	criticalResponse := httptest.NewRecorder()
	engine.ServeHTTP(criticalResponse, criticalRequest)
	require.Equal(t, http.StatusNoContent, criticalResponse.Code)
}
