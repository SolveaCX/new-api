package auto_model

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLocalErrorsHaveStableCodesStatusAndTranslations(t *testing.T) {
	require.NoError(t, i18n.Init())
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		make   func(*gin.Context) *types.NewAPIError
		code   types.ErrorCode
		status int
	}{
		{"disabled", DisabledError, types.ErrorCodeAutoModelDisabled, 400},
		{"unsupported", UnsupportedRequestError, types.ErrorCodeAutoModelUnsupportedRequest, 400},
		{"no candidate", NoEligibleCandidateError, types.ErrorCodeAutoModelNoEligibleCandidate, 400},
		{"config", ConfigInvalidError, types.ErrorCodeAutoModelConfigInvalid, 503},
	}
	for _, lang := range []string{i18n.LangEn, i18n.LangZhCN} {
		for _, test := range tests {
			t.Run(lang+"/"+test.name, func(t *testing.T) {
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
				c.Request.Header.Set("Accept-Language", lang)
				err := test.make(c)
				require.Equal(t, test.code, err.GetErrorCode())
				require.Equal(t, test.status, err.StatusCode)
				require.True(t, types.IsSkipRetryError(err))
				require.NotEmpty(t, err.Error())
				require.NotEqual(t, string(test.code), err.Error())
			})
		}
	}
}
