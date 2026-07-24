package perfmetrics

import (
	"context"
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/types"
)

type AvailabilityOutcome uint8

const (
	AvailabilityExcluded AvailabilityOutcome = iota
	AvailabilityEligibleFailure
	AvailabilityEligibleSuccess
)

func ClassifyAvailabilityOutcome(success bool, relayErr *types.NewAPIError) AvailabilityOutcome {
	if relayErr == nil {
		if success {
			return AvailabilityEligibleSuccess
		}
		return AvailabilityEligibleFailure
	}
	if errors.Is(relayErr, context.Canceled) {
		return AvailabilityExcluded
	}

	switch relayErr.GetErrorCode() {
	case types.ErrorCodeInvalidRequest,
		types.ErrorCodeSensitiveWordsDetected,
		types.ErrorCodeCountTokenFailed,
		types.ErrorCodeModelPriceError,
		types.ErrorCodeReadRequestBodyFailed,
		types.ErrorCodeConvertRequestFailed,
		types.ErrorCodeAccessDenied,
		types.ErrorCodeBadRequestBody,
		types.ErrorCodePromptBlocked,
		types.ErrorCodeInsufficientUserQuota,
		types.ErrorCodePreConsumeTokenQuotaFailed:
		return AvailabilityExcluded
	case types.ErrorCodeChannelInvalidKey,
		types.ErrorCodeChannelNoAvailableKey,
		types.ErrorCodeGetChannelFailed,
		types.ErrorCodeDoRequestFailed,
		types.ErrorCodeChannelResponseTimeExceeded,
		types.ErrorCodeReadResponseBodyFailed,
		types.ErrorCodeBadResponseStatusCode,
		types.ErrorCodeBadResponse,
		types.ErrorCodeBadResponseBody,
		types.ErrorCodeEmptyResponse,
		types.ErrorCodeAwsInvokeError,
		types.ErrorCodeChannelAwsClientError:
		return AvailabilityEligibleFailure
	}

	if relayErr.StatusCode == http.StatusRequestTimeout ||
		relayErr.StatusCode == http.StatusTooManyRequests ||
		relayErr.StatusCode >= http.StatusInternalServerError {
		return AvailabilityEligibleFailure
	}
	if relayErr.StatusCode >= http.StatusBadRequest && relayErr.StatusCode < http.StatusInternalServerError {
		return AvailabilityExcluded
	}
	return AvailabilityEligibleFailure
}
