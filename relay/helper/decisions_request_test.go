package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newDecisionsRequestContext(body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/alpha/decisions", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx
}

func TestGetAndValidateTextRequestDecisionsRequiresStateAndQuestions(t *testing.T) {
	valid := `{"model":"typesafe/jev-1.13","state":{"text":"hello"},"questions":{"route":{"type":"choice","instructions":"Pick one","criteria":{"a":"A"}}}}`

	t.Run("accepts valid decision request", func(t *testing.T) {
		request, err := GetAndValidateTextRequest(newDecisionsRequestContext(valid), relayconstant.RelayModeDecisions)
		require.NoError(t, err)
		require.Equal(t, "typesafe/jev-1.13", request.Model)
		require.JSONEq(t, `{"text":"hello"}`, string(request.State))
		require.Contains(t, string(request.Questions), `"route"`)
	})

	t.Run("rejects missing state", func(t *testing.T) {
		_, err := GetAndValidateTextRequest(newDecisionsRequestContext(`{"model":"typesafe/jev-1.13","questions":{}}`), relayconstant.RelayModeDecisions)
		require.EqualError(t, err, "field state is required")
	})

	t.Run("rejects missing questions", func(t *testing.T) {
		_, err := GetAndValidateTextRequest(newDecisionsRequestContext(`{"model":"typesafe/jev-1.13","state":{}}`), relayconstant.RelayModeDecisions)
		require.EqualError(t, err, "field questions is required")
	})

	t.Run("rejects explicit null fields", func(t *testing.T) {
		_, err := GetAndValidateTextRequest(newDecisionsRequestContext(`{"model":"typesafe/jev-1.13","state":null,"questions":null}`), relayconstant.RelayModeDecisions)
		require.EqualError(t, err, "field state is required")
	})
}
