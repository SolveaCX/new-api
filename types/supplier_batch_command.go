package types

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

const (
	SupplierBatchCommandSchemaVersion = 1
	SupplierBatchCommandMaxBytes      = 8 * 1024

	SupplierBatchCommandStateClaimed   = "claimed"
	SupplierBatchCommandStateRunning   = "running"
	SupplierBatchCommandStateCompleted = "completed"
	SupplierBatchCommandStateFailed    = "failed"

	SupplierBatchErrorNone              = "none"
	SupplierBatchErrorFenceLost         = "fence_lost"
	SupplierBatchErrorLeaseExpired      = "lease_expired"
	SupplierBatchErrorExecutionFailed   = "execution_failed"
	SupplierBatchErrorReadFailed        = "read_failed"
	SupplierBatchErrorPublicationFailed = "publication_failed"
)

var ErrInvalidSupplierBatchCommand = errors.New("invalid supplier batch command")

type SupplierBatchCommandResultV1 struct {
	ProcessedDays int     `json:"processed_days"`
	RemainingWork bool    `json:"remaining_work"`
	NextBatchDate *string `json:"next_batch_date"`
}

type SupplierBatchCommandStatusV1 struct {
	RequestID           string                        `json:"request_id"`
	BatchDate           *string                       `json:"batch_date"`
	RunID               *int64                        `json:"run_id"`
	Status              string                        `json:"status"`
	FenceToken          int64                         `json:"fence_token"`
	PublishedFenceToken int64                         `json:"published_fence_token"`
	LockedUntil         *string                       `json:"locked_until"`
	ErrorCategory       string                        `json:"error_category"`
	Result              *SupplierBatchCommandResultV1 `json:"result"`
}

type SupplierBatchCommandStateV1 struct {
	SchemaVersion int                           `json:"schema_version"`
	State         string                        `json:"state"`
	Response      *SupplierBatchCommandStatusV1 `json:"response"`
}

func ValidateSupplierBatchCommandStateV1(state SupplierBatchCommandStateV1) error {
	if state.SchemaVersion != SupplierBatchCommandSchemaVersion {
		return ErrInvalidSupplierBatchCommand
	}
	switch state.State {
	case SupplierBatchCommandStateClaimed:
		if state.Response != nil {
			return ErrInvalidSupplierBatchCommand
		}
	case SupplierBatchCommandStateRunning, SupplierBatchCommandStateCompleted, SupplierBatchCommandStateFailed:
		if state.Response == nil || state.Response.Status != state.State || validateSupplierBatchCommandStatus(*state.Response) != nil {
			return ErrInvalidSupplierBatchCommand
		}
	default:
		return ErrInvalidSupplierBatchCommand
	}
	return nil
}

func validateSupplierBatchCommandStatus(status SupplierBatchCommandStatusV1) error {
	if status.RequestID == "" || len(status.RequestID) > 128 || strings.TrimSpace(status.RequestID) != status.RequestID || !utf8.ValidString(status.RequestID) ||
		status.FenceToken < 0 || status.PublishedFenceToken < 0 || !validSupplierBatchDatePointer(status.BatchDate) {
		return ErrInvalidSupplierBatchCommand
	}
	switch status.ErrorCategory {
	case SupplierBatchErrorNone, SupplierBatchErrorFenceLost, SupplierBatchErrorLeaseExpired, SupplierBatchErrorExecutionFailed, SupplierBatchErrorReadFailed, SupplierBatchErrorPublicationFailed:
	default:
		return ErrInvalidSupplierBatchCommand
	}
	switch status.Status {
	case SupplierBatchCommandStateRunning:
		if status.BatchDate == nil || status.RunID == nil || *status.RunID <= 0 || status.FenceToken <= 0 || status.PublishedFenceToken >= status.FenceToken || status.LockedUntil == nil || status.Result != nil || status.ErrorCategory != SupplierBatchErrorNone {
			return ErrInvalidSupplierBatchCommand
		}
		if parsed, err := time.Parse(time.RFC3339, *status.LockedUntil); err != nil || parsed.Format(time.RFC3339) != *status.LockedUntil {
			return ErrInvalidSupplierBatchCommand
		}
	case SupplierBatchCommandStateCompleted:
		if status.LockedUntil != nil || status.Result == nil || status.ErrorCategory != SupplierBatchErrorNone || !validSupplierBatchCommandResult(*status.Result) {
			return ErrInvalidSupplierBatchCommand
		}
		if status.Result.ProcessedDays == 0 {
			if status.BatchDate != nil || status.RunID != nil || status.FenceToken != 0 || status.PublishedFenceToken != 0 || status.Result.RemainingWork || status.Result.NextBatchDate != nil {
				return ErrInvalidSupplierBatchCommand
			}
		} else if status.BatchDate == nil || status.RunID == nil || *status.RunID <= 0 || status.FenceToken <= 0 || status.PublishedFenceToken != status.FenceToken {
			return ErrInvalidSupplierBatchCommand
		}
	case SupplierBatchCommandStateFailed:
		if status.LockedUntil != nil || status.BatchDate == nil || status.RunID == nil || *status.RunID <= 0 || status.FenceToken <= 0 || status.ErrorCategory == SupplierBatchErrorNone || status.Result == nil || status.Result.ProcessedDays != 0 || !validSupplierBatchCommandResult(*status.Result) {
			return ErrInvalidSupplierBatchCommand
		}
	default:
		return ErrInvalidSupplierBatchCommand
	}
	return nil
}

func validSupplierBatchCommandResult(result SupplierBatchCommandResultV1) bool {
	if result.ProcessedDays < 0 || result.ProcessedDays > 1 || !validSupplierBatchDatePointer(result.NextBatchDate) {
		return false
	}
	return result.RemainingWork == (result.NextBatchDate != nil)
}

func validSupplierBatchDatePointer(value *string) bool {
	if value == nil {
		return true
	}
	parsed, err := time.Parse("2006-01-02", *value)
	return err == nil && parsed.Format("2006-01-02") == *value
}

func EncodeSupplierBatchCommandStateV1(state SupplierBatchCommandStateV1) (string, error) {
	if err := ValidateSupplierBatchCommandStateV1(state); err != nil {
		return "", err
	}
	encoded, err := common.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("encode supplier batch command: %w", err)
	}
	if len(encoded) == 0 || len(encoded) > SupplierBatchCommandMaxBytes || !utf8.Valid(encoded) {
		return "", ErrInvalidSupplierBatchCommand
	}
	return string(encoded), nil
}

func ParseSupplierBatchCommandStateV1(raw string) (SupplierBatchCommandStateV1, error) {
	var state SupplierBatchCommandStateV1
	if raw == "" || len(raw) > SupplierBatchCommandMaxBytes || strings.TrimSpace(raw) != raw || !utf8.ValidString(raw) {
		return state, ErrInvalidSupplierBatchCommand
	}
	fields, err := requiredStrictObject(raw, []string{"schema_version", "state", "response"})
	if err != nil {
		return state, fmt.Errorf("parse supplier batch command: %w", err)
	}
	if string(fields["response"]) != "null" {
		responseFields, strictErr := requiredStrictRawObject(fields["response"], []string{
			"request_id", "batch_date", "run_id", "status", "fence_token", "published_fence_token", "locked_until", "error_category", "result",
		})
		if strictErr != nil {
			return state, fmt.Errorf("parse supplier batch command response: %w", strictErr)
		}
		if string(responseFields["result"]) != "null" {
			if _, strictErr = requiredStrictRawObject(responseFields["result"], []string{"processed_days", "remaining_work", "next_batch_date"}); strictErr != nil {
				return state, fmt.Errorf("parse supplier batch command result: %w", strictErr)
			}
		}
	}
	if err = common.UnmarshalJsonStr(raw, &state); err != nil {
		return state, fmt.Errorf("parse supplier batch command: %w", err)
	}
	if err = ValidateSupplierBatchCommandStateV1(state); err != nil {
		return state, err
	}
	return state, nil
}
