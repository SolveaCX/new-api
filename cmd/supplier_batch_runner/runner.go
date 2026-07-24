package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

const (
	consoleURLEnv    = "SUPPLIER_BATCH_CONSOLE_URL"
	tokenEnv         = "SUPPLIER_BATCH_TOKEN"
	pollIntervalEnv  = "SUPPLIER_BATCH_POLL_INTERVAL"
	runnerTimeout    = 55 * time.Minute
	defaultPoll      = 5 * time.Second
	minimumPoll      = time.Second
	maximumPoll      = time.Minute
	maximumBodyBytes = 8 * 1024
)

var (
	errProtocol             = errors.New("supplier batch protocol error")
	errAuthentication       = errors.New("supplier batch authentication failed")
	errVerifierUnavailable  = errors.New("supplier batch verifier unavailable")
	errConfiguration        = errors.New("supplier batch configuration unavailable")
	errIdempotencyConflict  = errors.New("supplier batch idempotency conflict")
	errRemoteInternal       = errors.New("supplier batch remote internal error")
	errTerminalBatchFailure = errors.New("supplier batch completed with failure")
	errUnexpectedHTTPStatus = errors.New("unexpected supplier batch HTTP status")
)

type config struct {
	consoleURL   *url.URL
	token        string
	pollInterval time.Duration
}

type runner struct {
	config  config
	client  *http.Client
	newID   func() (string, error)
	wait    func(context.Context, time.Duration) error
	timeout func(context.Context, time.Duration) (context.Context, context.CancelFunc)
	runFor  time.Duration
}

type schedulerErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

func loadConfigFromEnv() (config, error) {
	return parseConfig(os.Getenv(consoleURLEnv), os.Getenv(tokenEnv), os.Getenv(pollIntervalEnv))
}

func parseConfig(rawConsoleURL, token, rawPollInterval string) (config, error) {
	consoleURL, err := url.Parse(strings.TrimSpace(rawConsoleURL))
	if err != nil || consoleURL.Scheme != "https" || consoleURL.Host == "" || consoleURL.User != nil || consoleURL.RawQuery != "" || consoleURL.Fragment != "" || (consoleURL.Path != "" && consoleURL.Path != "/") {
		return config{}, fmt.Errorf("%s must be an HTTPS origin", consoleURLEnv)
	}
	consoleURL.Path = ""
	consoleURL.RawPath = ""

	if strings.TrimSpace(token) != token || token == "" {
		return config{}, fmt.Errorf("%s must contain one raw base64url token", tokenEnv)
	}
	decodedToken, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decodedToken) != 32 {
		return config{}, fmt.Errorf("%s must be a 32-byte raw base64url token", tokenEnv)
	}

	pollInterval := defaultPoll
	if strings.TrimSpace(rawPollInterval) != "" {
		pollInterval, err = time.ParseDuration(strings.TrimSpace(rawPollInterval))
		if err != nil || pollInterval < minimumPoll || pollInterval > maximumPoll {
			return config{}, fmt.Errorf("%s must be between %s and %s", pollIntervalEnv, minimumPoll, maximumPoll)
		}
	}
	return config{consoleURL: consoleURL, token: token, pollInterval: pollInterval}, nil
}

func newRunner(config config) *runner {
	return &runner{
		config: config,
		client: &http.Client{
			// Every request is governed by the one Run-scoped deadline below.
			// A per-request client timeout would incorrectly restart the 55-minute
			// budget during status reconciliation or same-key takeover.
			Timeout: 0,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		newID:   newRequestID,
		wait:    waitFor,
		timeout: context.WithTimeout,
		runFor:  runnerTimeout,
	}
}

func newRequestID() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate request id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func waitFor(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (r *runner) Run(ctx context.Context) (dto.SupplierBatchStatusResponse, error) {
	runCtx, cancel := r.timeout(ctx, r.runFor)
	defer cancel()

	requestID, err := r.newID()
	if err != nil {
		return dto.SupplierBatchStatusResponse{}, err
	}
	if err := validateRequestID(requestID); err != nil {
		return dto.SupplierBatchStatusResponse{}, err
	}

	postRequired := true
	for {
		if err := runCtx.Err(); err != nil {
			return dto.SupplierBatchStatusResponse{}, err
		}

		var status dto.SupplierBatchStatusResponse
		if postRequired {
			status, err = r.catchUp(runCtx, requestID)
			if err != nil {
				switch {
				case errors.Is(err, errVerifierUnavailable), errors.Is(err, errConfiguration), errors.Is(err, errAuthentication), errors.Is(err, errIdempotencyConflict):
					return dto.SupplierBatchStatusResponse{}, err
				default:
					// A POST transport error or unexpected 5xx is ambiguous. Resolve
					// the durable request anchor before another POST is allowed.
					postRequired = false
					continue
				}
			}
		} else {
			status, err = r.status(runCtx, requestID)
			if err != nil {
				switch {
				case errors.Is(err, errVerifierUnavailable), errors.Is(err, errConfiguration), errors.Is(err, errAuthentication), errors.Is(err, errIdempotencyConflict):
					return dto.SupplierBatchStatusResponse{}, err
				case errors.Is(err, errProtocol):
					return dto.SupplierBatchStatusResponse{}, err
				case errors.Is(err, errRequestNotFound):
					// 404 proves no command is anchored for this key. Retrying the
					// POST with the same key is therefore safe.
					postRequired = true
				}
				if waitErr := r.wait(runCtx, r.config.pollInterval); waitErr != nil {
					return dto.SupplierBatchStatusResponse{}, waitErr
				}
				continue
			}
		}

		if status.RequestID != requestID {
			return dto.SupplierBatchStatusResponse{}, fmt.Errorf("%w: response request_id mismatch", errProtocol)
		}
		switch status.Status {
		case dto.SupplierBatchStatusCompleted:
			return status, nil
		case dto.SupplierBatchStatusRunning:
			postRequired = false
		case dto.SupplierBatchStatusFailed:
			if status.ErrorCategory == dto.SupplierBatchErrorLeaseExpired {
				// Status has proven the former lease expired. The server owns the
				// atomic takeover decision; use the original request key verbatim.
				postRequired = true
			} else {
				return dto.SupplierBatchStatusResponse{}, fmt.Errorf("%w: %s", errTerminalBatchFailure, status.ErrorCategory)
			}
		default:
			return dto.SupplierBatchStatusResponse{}, fmt.Errorf("%w: invalid status", errProtocol)
		}
		if waitErr := r.wait(runCtx, r.config.pollInterval); waitErr != nil {
			return dto.SupplierBatchStatusResponse{}, waitErr
		}
	}
}

var errRequestNotFound = errors.New("supplier batch request not found")

func (r *runner) catchUp(ctx context.Context, requestID string) (dto.SupplierBatchStatusResponse, error) {
	request, err := r.newRequest(ctx, http.MethodPost, "/api/supply-chain/daily-batches/catch-up", requestID)
	if err != nil {
		return dto.SupplierBatchStatusResponse{}, err
	}
	request.Header.Set("Idempotency-Key", requestID)
	return r.do(request, requestID)
}

func (r *runner) status(ctx context.Context, requestID string) (dto.SupplierBatchStatusResponse, error) {
	path := "/api/supply-chain/daily-batches/status?request_id=" + url.QueryEscape(requestID)
	request, err := r.newRequest(ctx, http.MethodGet, path, requestID)
	if err != nil {
		return dto.SupplierBatchStatusResponse{}, err
	}
	return r.do(request, requestID)
}

func (r *runner) newRequest(ctx context.Context, method, path, requestID string) (*http.Request, error) {
	if err := validateRequestID(requestID); err != nil {
		return nil, err
	}
	endpoint := r.config.consoleURL.ResolveReference(&url.URL{Path: path})
	if strings.Contains(path, "?") {
		parsed, err := url.Parse(path)
		if err != nil {
			return nil, fmt.Errorf("build scheduler URL: %w", err)
		}
		endpoint = r.config.consoleURL.ResolveReference(parsed)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build scheduler request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+r.config.token)
	request.Header.Set("Accept", "application/json")
	return request, nil
}

func validateRequestID(requestID string) error {
	decoded, err := base64.RawURLEncoding.DecodeString(requestID)
	if err != nil || len(decoded) != 32 || len(requestID) > 128 || strings.TrimSpace(requestID) != requestID {
		return fmt.Errorf("%w: request id must be 32-byte raw base64url", errProtocol)
	}
	return nil
}

func (r *runner) do(request *http.Request, requestID string) (dto.SupplierBatchStatusResponse, error) {
	response, err := r.client.Do(request)
	if err != nil {
		return dto.SupplierBatchStatusResponse{}, fmt.Errorf("scheduler request transport: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maximumBodyBytes+1))
	if err != nil {
		return dto.SupplierBatchStatusResponse{}, fmt.Errorf("read scheduler response: %w", err)
	}
	if len(body) > maximumBodyBytes {
		return dto.SupplierBatchStatusResponse{}, fmt.Errorf("%w: response exceeds %d bytes", errProtocol, maximumBodyBytes)
	}
	if response.StatusCode == http.StatusOK {
		return parseStatusResponse(body, requestID)
	}
	return dto.SupplierBatchStatusResponse{}, parseErrorResponse(response.StatusCode, body)
}

func parseStatusResponse(body []byte, requestID string) (dto.SupplierBatchStatusResponse, error) {
	allowed := map[string]struct{}{
		"request_id": {}, "batch_date": {}, "run_id": {}, "status": {}, "fence_token": {},
		"published_fence_token": {}, "locked_until": {}, "error_category": {}, "result": {},
	}
	fields, err := common.UnmarshalJsonObjectStrict(string(body), allowed)
	if err != nil || len(fields) != len(allowed) {
		return dto.SupplierBatchStatusResponse{}, fmt.Errorf("%w: invalid status object", errProtocol)
	}
	for field := range allowed {
		if _, ok := fields[field]; !ok {
			return dto.SupplierBatchStatusResponse{}, fmt.Errorf("%w: missing status field %s", errProtocol, field)
		}
	}
	if string(fields["result"]) != "null" {
		resultAllowed := map[string]struct{}{"processed_days": {}, "remaining_work": {}, "next_batch_date": {}}
		resultFields, strictErr := common.UnmarshalJsonObjectStrict(string(fields["result"]), resultAllowed)
		if strictErr != nil || len(resultFields) != len(resultAllowed) {
			return dto.SupplierBatchStatusResponse{}, fmt.Errorf("%w: invalid result object", errProtocol)
		}
		for field := range resultAllowed {
			if _, ok := resultFields[field]; !ok {
				return dto.SupplierBatchStatusResponse{}, fmt.Errorf("%w: missing result field %s", errProtocol, field)
			}
		}
	}
	var status dto.SupplierBatchStatusResponse
	if err := common.Unmarshal(body, &status); err != nil {
		return dto.SupplierBatchStatusResponse{}, fmt.Errorf("%w: decode status", errProtocol)
	}
	if err := status.Validate(); err != nil || status.RequestID != requestID {
		return dto.SupplierBatchStatusResponse{}, fmt.Errorf("%w: invalid status semantics", errProtocol)
	}
	return status, nil
}

func parseErrorResponse(statusCode int, body []byte) error {
	allowed := map[string]struct{}{"success": {}, "message": {}, "code": {}}
	fields, err := common.UnmarshalJsonObjectStrict(string(body), allowed)
	if err != nil || len(fields) != len(allowed) {
		return fmt.Errorf("%w: HTTP %d invalid error object", errProtocol, statusCode)
	}
	for field := range allowed {
		if _, ok := fields[field]; !ok {
			return fmt.Errorf("%w: HTTP %d missing error field %s", errProtocol, statusCode, field)
		}
	}
	var response schedulerErrorResponse
	if err := common.Unmarshal(body, &response); err != nil || response.Success || strings.TrimSpace(response.Message) == "" {
		return fmt.Errorf("%w: HTTP %d invalid error semantics", errProtocol, statusCode)
	}
	switch {
	case statusCode == http.StatusNotFound && response.Code == "not_found":
		return errRequestNotFound
	case statusCode == http.StatusConflict && response.Code == "busy":
		return fmt.Errorf("%w: busy", errUnexpectedHTTPStatus)
	case statusCode == http.StatusConflict && response.Code == "idempotency_conflict":
		return errIdempotencyConflict
	case statusCode == http.StatusServiceUnavailable && response.Code == "verifier_unavailable":
		return errVerifierUnavailable
	case statusCode == http.StatusServiceUnavailable && response.Code == "config_unavailable":
		return errConfiguration
	case statusCode == http.StatusUnauthorized && response.Code == "unauthorized":
		return errAuthentication
	case statusCode >= 500 && response.Code == "internal_error":
		return errRemoteInternal
	default:
		return fmt.Errorf("%w: HTTP %d unrecognized error contract", errUnexpectedHTTPStatus, statusCode)
	}
}
