package auto_model

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func DisabledError(c *gin.Context) *types.NewAPIError {
	return newLocalError(c, i18n.MsgAutoModelDisabled, types.ErrorCodeAutoModelDisabled, http.StatusBadRequest)
}

func UnsupportedRequestError(c *gin.Context) *types.NewAPIError {
	return newLocalError(c, i18n.MsgAutoModelUnsupportedRequest, types.ErrorCodeAutoModelUnsupportedRequest, http.StatusBadRequest)
}

func NoEligibleCandidateError(c *gin.Context) *types.NewAPIError {
	return newLocalError(c, i18n.MsgAutoModelNoEligibleCandidate, types.ErrorCodeAutoModelNoEligibleCandidate, http.StatusBadRequest)
}

func ConfigInvalidError(c *gin.Context) *types.NewAPIError {
	return newLocalError(c, i18n.MsgAutoModelConfigInvalid, types.ErrorCodeAutoModelConfigInvalid, http.StatusServiceUnavailable)
}

func newLocalError(c *gin.Context, messageKey string, code types.ErrorCode, status int) *types.NewAPIError {
	return types.NewErrorWithStatusCode(
		errors.New(i18n.T(c, messageKey)),
		code,
		status,
		types.ErrOptionWithSkipRetry(),
	)
}
