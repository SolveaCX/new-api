package dto

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	SupplierBatchStatusRunning   = "running"
	SupplierBatchStatusCompleted = "completed"
	SupplierBatchStatusFailed    = "failed"

	SupplierBatchErrorNone              = "none"
	SupplierBatchErrorFenceLost         = "fence_lost"
	SupplierBatchErrorLeaseExpired      = "lease_expired"
	SupplierBatchErrorExecutionFailed   = "execution_failed"
	SupplierBatchErrorReadFailed        = "read_failed"
	SupplierBatchErrorPublicationFailed = "publication_failed"

	SupplierBatchAuditSlotCurrent = "current"
	SupplierBatchAuditSlotNext    = "next"
)

var ErrSupplierBatchStatusInvalid = errors.New("invalid supplier batch status response")

// SupplierBatchSchedulerPrincipal is derived only from a matched server-side
// verifier slot. It is deliberately excluded from JSON responses.
type SupplierBatchSchedulerPrincipal struct {
	TrustedJobIdentity string `json:"-"`
	AuditSlot          string `json:"-"`
}

// SupplierBatchCatchUpRequest carries the stable scheduler request anchor.
// RequestID is the verbatim validated Idempotency-Key header.
type SupplierBatchCatchUpRequest struct {
	RequestID string `json:"-"`
}

type SupplierBatchStatusResult struct {
	ProcessedDays int     `json:"processed_days"`
	RemainingWork bool    `json:"remaining_work"`
	NextBatchDate *string `json:"next_batch_date"`
}

// SupplierBatchStatusResponse is the fixed scheduler compatibility payload.
// It never contains credentials, verifier material, or lease-owner identity.
type SupplierBatchStatusResponse struct {
	RequestID           string                     `json:"request_id"`
	BatchDate           *string                    `json:"batch_date"`
	RunID               *int64                     `json:"run_id"`
	Status              string                     `json:"status"`
	FenceToken          int64                      `json:"fence_token"`
	PublishedFenceToken int64                      `json:"published_fence_token"`
	LockedUntil         *string                    `json:"locked_until"`
	ErrorCategory       string                     `json:"error_category"`
	Result              *SupplierBatchStatusResult `json:"result"`
}

func (r SupplierBatchStatusResponse) Validate() error {
	if strings.TrimSpace(r.RequestID) == "" || r.RequestID != strings.TrimSpace(r.RequestID) || len(r.RequestID) > 128 || !utf8.ValidString(r.RequestID) || r.FenceToken < 0 || r.PublishedFenceToken < 0 {
		return ErrSupplierBatchStatusInvalid
	}
	if !validSupplierBatchErrorCategory(r.ErrorCategory) {
		return ErrSupplierBatchStatusInvalid
	}
	switch r.Status {
	case SupplierBatchStatusRunning:
		if r.BatchDate == nil || !validSupplierBatchDate(*r.BatchDate) || r.RunID == nil || *r.RunID <= 0 || r.FenceToken <= 0 || r.PublishedFenceToken >= r.FenceToken || r.LockedUntil == nil || r.Result != nil {
			return ErrSupplierBatchStatusInvalid
		}
		if parsed, err := time.Parse(time.RFC3339, *r.LockedUntil); err != nil || parsed.Format(time.RFC3339) != *r.LockedUntil {
			return ErrSupplierBatchStatusInvalid
		}
		if r.ErrorCategory != SupplierBatchErrorNone {
			return ErrSupplierBatchStatusInvalid
		}
	case SupplierBatchStatusCompleted, SupplierBatchStatusFailed:
		if r.LockedUntil != nil || r.Result == nil || r.Result.ProcessedDays < 0 || r.Result.ProcessedDays > 1 {
			return ErrSupplierBatchStatusInvalid
		}
		if r.Result.NextBatchDate != nil && !validSupplierBatchDate(*r.Result.NextBatchDate) {
			return ErrSupplierBatchStatusInvalid
		}
		if r.Result.RemainingWork != (r.Result.NextBatchDate != nil) {
			return ErrSupplierBatchStatusInvalid
		}
		if (r.BatchDate == nil) != (r.RunID == nil) {
			return ErrSupplierBatchStatusInvalid
		}
		if r.BatchDate == nil {
			if r.Status != SupplierBatchStatusCompleted || r.FenceToken != 0 || r.PublishedFenceToken != 0 || r.ErrorCategory != SupplierBatchErrorNone || r.Result.ProcessedDays != 0 || r.Result.RemainingWork || r.Result.NextBatchDate != nil {
				return ErrSupplierBatchStatusInvalid
			}
		} else {
			if !validSupplierBatchDate(*r.BatchDate) || r.RunID == nil || *r.RunID <= 0 || r.FenceToken <= 0 {
				return ErrSupplierBatchStatusInvalid
			}
			if r.Status == SupplierBatchStatusCompleted && (r.ErrorCategory != SupplierBatchErrorNone || r.Result.ProcessedDays != 1 || r.PublishedFenceToken != r.FenceToken) {
				return ErrSupplierBatchStatusInvalid
			}
			if r.Status == SupplierBatchStatusFailed && (r.ErrorCategory == SupplierBatchErrorNone || r.Result.ProcessedDays != 0) {
				return ErrSupplierBatchStatusInvalid
			}
		}
	default:
		return ErrSupplierBatchStatusInvalid
	}
	return nil
}

type SupplierDailyReportRerunRequest struct {
	Reason                      string `json:"reason"`
	ExpectedPublishedFenceToken int64  `json:"expected_published_fence_token"`
}

func validSupplierBatchErrorCategory(value string) bool {
	switch value {
	case SupplierBatchErrorNone, SupplierBatchErrorFenceLost, SupplierBatchErrorLeaseExpired, SupplierBatchErrorExecutionFailed, SupplierBatchErrorReadFailed, SupplierBatchErrorPublicationFailed:
		return true
	default:
		return false
	}
}

func validSupplierBatchDate(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}
