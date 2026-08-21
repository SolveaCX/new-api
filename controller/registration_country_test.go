package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegistrationCountryDecisionRecognizesMorocco(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest("POST", "/api/user/register", nil)
	req.RemoteAddr = "105.154.0.1:443" // embedded iploc resolves this to MA
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	country, blocked, autoDisable := registrationCountryDecision(c)
	require.Equal(t, "MA", country)
	require.True(t, blocked)
	require.False(t, autoDisable)
}
