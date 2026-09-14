package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSMSVerificationRateLimitReadsJSONBodyAndPreservesIt(t *testing.T) {
	previousEnabled, previousRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = false, nil
	t.Cleanup(func() { common.RedisEnabled, common.RDB = previousEnabled, previousRDB })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/sms", SMSVerificationRateLimit(), func(c *gin.Context) {
		var body struct {
			PhoneNumber string `json:"phone_number"`
		}
		require.NoError(t, common.DecodeJson(c.Request.Body, &body))
		require.Equal(t, "+14155552671", body.PhoneNumber)
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/sms", strings.NewReader(`{"phone_number":"+14155552671"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestSMSVerificationRateLimitFailsClosedWithoutRedisClient(t *testing.T) {
	previousEnabled, previousRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = true, nil
	t.Cleanup(func() { common.RedisEnabled, common.RDB = previousEnabled, previousRDB })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/sms", SMSVerificationRateLimit(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/sms", strings.NewReader(`{"phone_number":"+14155552671"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}
