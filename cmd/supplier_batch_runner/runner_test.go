package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

var fixedRequestID = base64.RawURLEncoding.EncodeToString(bytesOf(7, 32))

func TestNewRequestIDIsCSPRNGBase64URL(t *testing.T) {
	first, err := newRequestID()
	require.NoError(t, err)
	second, err := newRequestID()
	require.NoError(t, err)
	require.NotEqual(t, first, second)
	for _, requestID := range []string{first, second} {
		decoded, err := base64.RawURLEncoding.DecodeString(requestID)
		require.NoError(t, err)
		require.Len(t, decoded, 32)
		require.NoError(t, validateRequestID(requestID))
	}
}

func TestParseConfigValidatesControlledInputsAndDeadline(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(bytesOf(11, 32))
	parsed, err := parseConfig("https://console.example.com/", token, "7s")
	require.NoError(t, err)
	require.Equal(t, "https://console.example.com", parsed.consoleURL.String())
	require.Equal(t, 7*time.Second, parsed.pollInterval)
	client := newRunner(parsed).client
	require.Zero(t, client.Timeout, "per-request timeout must not restart the total runner budget")
	require.Equal(t, 55*time.Minute, runnerTimeout)
	jobTimeoutContract := 60 * time.Minute
	consoleRequestTimeoutContract := 60 * time.Minute
	require.Less(t, 45*time.Minute, runnerTimeout, "server hard stop must precede the total runner timeout")
	require.Less(t, runnerTimeout, jobTimeoutContract, "runner timeout must leave recovery time inside the Job deadline")
	require.Less(t, runnerTimeout, consoleRequestTimeoutContract, "runner timeout must be below the Console request timeout")
	require.GreaterOrEqual(t, jobTimeoutContract, 60*time.Minute)
	require.ErrorIs(t, client.CheckRedirect(nil, nil), http.ErrUseLastResponse)

	for _, test := range []struct {
		name, consoleURL, token, poll string
	}{
		{name: "missing URL", token: token},
		{name: "URL credentials", consoleURL: "https://user:pass@console.example.com", token: token},
		{name: "URL path", consoleURL: "https://console.example.com/api", token: token},
		{name: "short token", consoleURL: "https://console.example.com", token: "short"},
		{name: "poll too short", consoleURL: "https://console.example.com", token: token, poll: "100ms"},
		{name: "poll too long", consoleURL: "https://console.example.com", token: token, poll: "2m"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseConfig(test.consoleURL, test.token, test.poll)
			require.Error(t, err)
			require.NotContains(t, err.Error(), token)
		})
	}
}

func TestRunnerCompletesNoWorkWithStableHeaders(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(bytesOf(12, 32))
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, "/api/supply-chain/daily-batches/catch-up", request.URL.Path)
		require.Equal(t, "Bearer "+token, request.Header.Get("Authorization"))
		require.Equal(t, fixedRequestID, request.Header.Get("Idempotency-Key"))
		writeStatus(t, w, completedNoWork(fixedRequestID))
	})

	r := testRunner(t, handler, token)
	status, err := r.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, dto.SupplierBatchStatusCompleted, status.Status)
	require.Zero(t, status.Result.ProcessedDays)
}

func TestRunnerLostPOSTResponseQueriesStatusWithSameKey(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(bytesOf(13, 32))
	var postKey, statusKey string
	r := testRunner(t, http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		statusKey = request.URL.Query().Get("request_id")
		writeStatus(t, w, completedNoWork(statusKey))
	}), token)
	baseTransport := r.client.Transport
	r.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPost {
			postKey = request.Header.Get("Idempotency-Key")
			return nil, errors.New("response lost")
		}
		return baseTransport.RoundTrip(request)
	})
	status, err := r.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, fixedRequestID, status.RequestID)
	require.Equal(t, postKey, statusKey)
}

func TestRunnerAmbiguousPOSTTimeoutQueriesStatusBeforeRetry(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(bytesOf(20, 32))
	var methods []string
	r := testRunner(t, http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		methods = append(methods, request.Method)
		writeStatus(t, w, completedNoWork(request.URL.Query().Get("request_id")))
	}), token)
	baseTransport := r.client.Transport
	postCount := 0
	r.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPost {
			postCount++
			return nil, context.DeadlineExceeded
		}
		return baseTransport.RoundTrip(request)
	})

	status, err := r.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, fixedRequestID, status.RequestID)
	require.Equal(t, 1, postCount)
	require.Equal(t, []string{http.MethodGet}, methods)
}

func TestRunnerSharesOneTotalDeadlineAcrossPollAndTakeover(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(bytesOf(22, 32))
	requestCount := 0
	var requestDeadlines []time.Time
	var requestKeys []string
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requestCount++
		deadline, ok := request.Context().Deadline()
		require.True(t, ok)
		requestDeadlines = append(requestDeadlines, deadline)
		if request.Method == http.MethodPost {
			requestKeys = append(requestKeys, request.Header.Get("Idempotency-Key"))
		} else {
			requestKeys = append(requestKeys, request.URL.Query().Get("request_id"))
		}
		switch requestCount {
		case 1:
			writeStatus(t, w, runningStatus(fixedRequestID))
		case 2:
			writeStatus(t, w, failedStatus(fixedRequestID, dto.SupplierBatchErrorLeaseExpired))
		case 3:
			writeStatus(t, w, runningStatus(fixedRequestID))
		case 4:
			writeStatus(t, w, completedDay(fixedRequestID))
		default:
			t.Fatalf("unexpected request %d", requestCount)
		}
	})
	r := testRunner(t, handler, token)
	r.runFor = time.Hour
	timeoutCalls := 0
	r.timeout = func(parent context.Context, duration time.Duration) (context.Context, context.CancelFunc) {
		timeoutCalls++
		require.Equal(t, time.Hour, duration)
		return context.WithTimeout(parent, duration)
	}
	var waitDeadlines []time.Time
	r.wait = func(ctx context.Context, _ time.Duration) error {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		waitDeadlines = append(waitDeadlines, deadline)
		return nil
	}

	status, err := r.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, dto.SupplierBatchStatusCompleted, status.Status)
	require.Equal(t, 1, timeoutCalls, "polling and takeover must not create a new total deadline")
	require.Len(t, requestDeadlines, 4)
	require.NotEmpty(t, waitDeadlines)
	for _, deadline := range append(requestDeadlines[1:], waitDeadlines...) {
		require.Equal(t, requestDeadlines[0], deadline)
	}
	for _, requestKey := range requestKeys {
		require.Equal(t, fixedRequestID, requestKey)
	}
}

func TestRunnerTotalDeadlineStopsPollingWithoutChangingRequestID(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(bytesOf(23, 32))
	var requestDeadlines []time.Time
	var requestKeys []string
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		deadline, ok := request.Context().Deadline()
		require.True(t, ok)
		requestDeadlines = append(requestDeadlines, deadline)
		if request.Method == http.MethodPost {
			requestKeys = append(requestKeys, request.Header.Get("Idempotency-Key"))
		} else {
			requestKeys = append(requestKeys, request.URL.Query().Get("request_id"))
		}
		writeStatus(t, w, runningStatus(fixedRequestID))
	})
	r := testRunner(t, handler, token)
	r.runFor = 30 * time.Millisecond
	r.config.pollInterval = time.Millisecond
	r.wait = waitFor
	newIDCalls := 0
	r.newID = func() (string, error) {
		newIDCalls++
		return fixedRequestID, nil
	}

	started := time.Now()
	_, err := r.Run(context.Background())
	elapsed := time.Since(started)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, elapsed, 500*time.Millisecond)
	require.Equal(t, 1, newIDCalls)
	require.GreaterOrEqual(t, len(requestKeys), 2, "the test must enter status polling before the shared deadline")
	for index, requestKey := range requestKeys {
		require.Equal(t, fixedRequestID, requestKey)
		require.Equal(t, requestDeadlines[0], requestDeadlines[index])
	}
}

func TestRunnerWaitsForRunningStatusAndReusesCommittedResult(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(bytesOf(14, 32))
	requests := make([]string, 0, 3)
	var lock sync.Mutex
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		lock.Lock()
		defer lock.Unlock()
		requests = append(requests, request.Method+" "+request.URL.RequestURI())
		if len(requests) < 3 {
			writeStatus(t, w, runningStatus(fixedRequestID))
			return
		}
		writeStatus(t, w, completedDay(fixedRequestID))
	})

	r := testRunner(t, handler, token)
	status, err := r.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, status.Result.ProcessedDays)
	require.Equal(t, []string{
		"POST /api/supply-chain/daily-batches/catch-up",
		"GET /api/supply-chain/daily-batches/status?request_id=" + fixedRequestID,
		"GET /api/supply-chain/daily-batches/status?request_id=" + fixedRequestID,
	}, requests)
}

func TestRunnerRetriesExpiredTakeoverWithOriginalKey(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(bytesOf(15, 32))
	var keys []string
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			keys = append(keys, request.Header.Get("Idempotency-Key"))
			if len(keys) == 1 {
				writeStatus(t, w, runningStatus(fixedRequestID))
				return
			}
			writeStatus(t, w, completedDay(fixedRequestID))
			return
		}
		writeStatus(t, w, failedStatus(fixedRequestID, dto.SupplierBatchErrorLeaseExpired))
	})

	r := testRunner(t, handler, token)
	status, err := r.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, dto.SupplierBatchStatusCompleted, status.Status)
	require.Equal(t, []string{fixedRequestID, fixedRequestID}, keys)
}

func TestRunnerNotFoundAndBusyRetrySameKeyWithoutStorm(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(bytesOf(16, 32))
	postCount := 0
	waits := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			postCount++
			require.Equal(t, fixedRequestID, request.Header.Get("Idempotency-Key"))
			if postCount == 1 {
				writeError(t, w, http.StatusConflict, "busy", "another request is active")
				return
			}
			writeStatus(t, w, completedNoWork(fixedRequestID))
			return
		}
		require.Equal(t, fixedRequestID, request.URL.Query().Get("request_id"))
		writeError(t, w, http.StatusNotFound, "not_found", "request does not exist")
	})

	r := testRunner(t, handler, token)
	r.wait = func(context.Context, time.Duration) error { waits++; return nil }
	_, err := r.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, postCount)
	require.GreaterOrEqual(t, waits, 1)
}

func TestRunnerStableTerminalErrorContract(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(bytesOf(17, 32))
	tests := []struct {
		name       string
		statusCode int
		code       string
		want       error
	}{
		{name: "idempotency conflict", statusCode: http.StatusConflict, code: "idempotency_conflict", want: errIdempotencyConflict},
		{name: "verifier", statusCode: http.StatusServiceUnavailable, code: "verifier_unavailable", want: errVerifierUnavailable},
		{name: "configuration", statusCode: http.StatusServiceUnavailable, code: "config_unavailable", want: errConfiguration},
		{name: "authentication", statusCode: http.StatusUnauthorized, code: "unauthorized", want: errAuthentication},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeError(t, w, test.statusCode, test.code, "safe message")
			})
			r := testRunner(t, handler, token)
			_, err := r.Run(context.Background())
			require.ErrorIs(t, err, test.want)
		})
	}
}

func TestRunnerReturnsStableTerminalBatchFailure(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(bytesOf(21, 32))
	r := testRunner(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeStatus(t, w, failedStatus(fixedRequestID, dto.SupplierBatchErrorExecutionFailed))
	}), token)
	_, err := r.Run(context.Background())
	require.ErrorIs(t, err, errTerminalBatchFailure)
	require.Contains(t, err.Error(), dto.SupplierBatchErrorExecutionFailed)
}

func TestRunnerRejectsInvalidDTOAndDoesNotLeakSecret(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(bytesOf(18, 32))
	tests := []struct {
		name, response string
	}{
		{name: "unknown field", response: strings.TrimSuffix(mustStatusJSON(t, completedNoWork(fixedRequestID)), "}") + `,"secret":"` + token + `"}`},
		{name: "invalid enum", response: strings.Replace(mustStatusJSON(t, completedNoWork(fixedRequestID)), `"status":"completed"`, `"status":"queued"`, 1)},
		{name: "missing field", response: `{"request_id":"` + fixedRequestID + `"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			postAttempted := false
			handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.Method == http.MethodPost {
					postAttempted = true
					writeError(t, w, http.StatusInternalServerError, "internal_error", "ambiguous")
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, err := fmt.Fprint(w, test.response)
				require.NoError(t, err)
			})
			r := testRunner(t, handler, token)
			_, err := r.Run(context.Background())
			require.True(t, postAttempted)
			require.ErrorIs(t, err, errProtocol)
			require.NotContains(t, err.Error(), token)
		})
	}
}

func TestRunnerHonorsContextDuringPolling(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(bytesOf(19, 32))
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeStatus(t, w, runningStatus(fixedRequestID))
	})
	r := testRunner(t, handler, token)
	ctx, cancel := context.WithCancel(context.Background())
	r.wait = func(context.Context, time.Duration) error { cancel(); return context.Canceled }
	_, err := r.Run(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func testRunner(t *testing.T, handler http.Handler, token string) *runner {
	t.Helper()
	parsed, err := parseConfig("https://console.test", token, "1s")
	require.NoError(t, err)
	r := newRunner(parsed)
	r.client.Transport = handlerTransport{handler: handler}
	r.newID = func() (string, error) { return fixedRequestID, nil }
	r.wait = func(context.Context, time.Duration) error { return nil }
	return r
}

type handlerTransport struct{ handler http.Handler }

func (transport handlerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	transport.handler.ServeHTTP(recorder, request)
	return recorder.Result(), nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func runningStatus(requestID string) dto.SupplierBatchStatusResponse {
	date := "2026-07-22"
	runID := int64(42)
	lockedUntil := "2026-07-23T03:00:00+08:00"
	return dto.SupplierBatchStatusResponse{
		RequestID: requestID, BatchDate: &date, RunID: &runID, Status: dto.SupplierBatchStatusRunning,
		FenceToken: 9, PublishedFenceToken: 7, LockedUntil: &lockedUntil, ErrorCategory: dto.SupplierBatchErrorNone,
	}
}

func completedNoWork(requestID string) dto.SupplierBatchStatusResponse {
	return dto.SupplierBatchStatusResponse{
		RequestID: requestID, Status: dto.SupplierBatchStatusCompleted, ErrorCategory: dto.SupplierBatchErrorNone,
		Result: &dto.SupplierBatchStatusResult{},
	}
}

func completedDay(requestID string) dto.SupplierBatchStatusResponse {
	date := "2026-07-22"
	runID := int64(42)
	return dto.SupplierBatchStatusResponse{
		RequestID: requestID, BatchDate: &date, RunID: &runID, Status: dto.SupplierBatchStatusCompleted,
		FenceToken: 9, PublishedFenceToken: 9, ErrorCategory: dto.SupplierBatchErrorNone,
		Result: &dto.SupplierBatchStatusResult{ProcessedDays: 1},
	}
}

func failedStatus(requestID, category string) dto.SupplierBatchStatusResponse {
	date := "2026-07-22"
	runID := int64(42)
	return dto.SupplierBatchStatusResponse{
		RequestID: requestID, BatchDate: &date, RunID: &runID, Status: dto.SupplierBatchStatusFailed,
		FenceToken: 9, PublishedFenceToken: 7, ErrorCategory: category,
		Result: &dto.SupplierBatchStatusResult{RemainingWork: true, NextBatchDate: stringPointer("2026-07-22")},
	}
}

func writeStatus(t *testing.T, writer http.ResponseWriter, status dto.SupplierBatchStatusResponse) {
	t.Helper()
	require.NoError(t, status.Validate())
	writer.Header().Set("Content-Type", "application/json")
	_, err := writer.Write([]byte(mustStatusJSON(t, status)))
	require.NoError(t, err)
}

func mustStatusJSON(t *testing.T, status dto.SupplierBatchStatusResponse) string {
	t.Helper()
	encoded, err := common.Marshal(status)
	require.NoError(t, err)
	return string(encoded)
}

func writeError(t *testing.T, writer http.ResponseWriter, statusCode int, code, message string) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)
	encoded, err := common.Marshal(schedulerErrorResponse{Success: false, Message: message, Code: code})
	require.NoError(t, err)
	_, err = writer.Write(encoded)
	require.NoError(t, err)
}

func bytesOf(value byte, count int) []byte {
	result := make([]byte, count)
	for index := range result {
		result[index] = value
	}
	return result
}

func stringPointer(value string) *string { return &value }
