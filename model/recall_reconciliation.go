package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	recallAttributionProgressSource   = "recall_reconciliation"
	recallAttributionProgressLeased   = "reconciliation_leased"
	recallAttributionProgressRetry    = "reconciliation_retry"
	recallAttributionProgressTerminal = "reconciliation_terminal"
	recallAttributionOutcomeMaxLength = 64
)

type RecallAttributionLease struct {
	SourceEventId  string
	LeaseExpiresAt int64
	Attempt        int
}

type recallAttributionProgressData struct {
	Attempt     int    `json:"attempt"`
	OutcomeCode string `json:"outcome_code,omitempty"`
}

func filterRecallAttributionCandidatesWithContext(ctx context.Context, candidates []RecallAttributionCandidate, nowUnix int64, limit int) ([]RecallAttributionCandidate, error) {
	if limit <= 0 || len(candidates) == 0 {
		return []RecallAttributionCandidate{}, nil
	}
	eligible := make([]RecallAttributionCandidate, 0, min(limit, len(candidates)))

	sourceEventIDs := make([]string, len(candidates))
	for i := range candidates {
		sourceEventIDs[i] = recallAttributionSourceEventID(candidates[i])
	}
	progressBySourceEventID := make(map[string]RecallEvent)
	for start := 0; start < len(sourceEventIDs); start += recallRunBatchSize {
		end := min(start+recallRunBatchSize, len(sourceEventIDs))
		var progress []RecallEvent
		if err := DB.WithContext(ctx).
			Select("source_event_id", "event_type", "created_at").
			Where("source = ? AND source_event_id IN ?", recallAttributionProgressSource, sourceEventIDs[start:end]).
			Find(&progress).Error; err != nil {
			return nil, err
		}
		for _, event := range progress {
			progressBySourceEventID[event.SourceEventId] = event
		}
	}

	for i, candidate := range candidates {
		if progress, exists := progressBySourceEventID[sourceEventIDs[i]]; exists {
			if progress.EventType == recallAttributionProgressTerminal {
				continue
			}
			if (progress.EventType == recallAttributionProgressLeased || progress.EventType == recallAttributionProgressRetry) && progress.CreatedAt > nowUnix {
				continue
			}
		}
		eligible = append(eligible, candidate)
		if len(eligible) == limit {
			break
		}
	}
	return eligible, nil
}

func LeaseRecallAttributionCandidateWithContext(ctx context.Context, candidate RecallAttributionCandidate, nowUnix int64, leaseUntil int64) (RecallAttributionLease, bool, error) {
	if leaseUntil <= nowUnix {
		return RecallAttributionLease{}, false, errors.New("recall attribution lease expiry must be after acquisition time")
	}
	sourceEventID := recallAttributionSourceEventID(candidate)
	lease := RecallAttributionLease{SourceEventId: sourceEventID, LeaseExpiresAt: leaseUntil, Attempt: 1}
	eventData, err := marshalRecallAttributionProgressData(lease.Attempt, "")
	if err != nil {
		return RecallAttributionLease{}, false, err
	}
	event := RecallEvent{
		EventType: recallAttributionProgressLeased, Source: recallAttributionProgressSource,
		SourceEventId: sourceEventID, EventData: eventData, CreatedAt: leaseUntil,
	}
	result := insertRecallRunEvent(DB.WithContext(ctx), &event)
	if result.Error != nil {
		return RecallAttributionLease{}, false, result.Error
	}
	if result.RowsAffected == 1 {
		return lease, true, nil
	}

	stored := RecallEvent{}
	if err := DB.WithContext(ctx).
		Select("event_type", "event_data", "created_at").
		Where("source = ? AND source_event_id = ?", recallAttributionProgressSource, sourceEventID).
		First(&stored).Error; err != nil {
		return RecallAttributionLease{}, false, err
	}
	if stored.EventType == recallAttributionProgressTerminal || stored.CreatedAt > nowUnix {
		return RecallAttributionLease{}, false, nil
	}
	progressData := recallAttributionProgressData{}
	if strings.TrimSpace(stored.EventData) != "" {
		if err := common.Unmarshal([]byte(stored.EventData), &progressData); err != nil {
			return RecallAttributionLease{}, false, fmt.Errorf("unmarshal recall attribution progress: %w", err)
		}
	}
	lease.Attempt = progressData.Attempt + 1
	if lease.Attempt < 1 {
		lease.Attempt = 1
	}
	eventData, err = marshalRecallAttributionProgressData(lease.Attempt, "")
	if err != nil {
		return RecallAttributionLease{}, false, err
	}
	result = DB.WithContext(ctx).Model(&RecallEvent{}).
		Where("source = ? AND source_event_id = ? AND event_type = ? AND created_at = ? AND created_at <= ?",
			recallAttributionProgressSource, sourceEventID, stored.EventType, stored.CreatedAt, nowUnix).
		Updates(map[string]any{
			"event_type": recallAttributionProgressLeased,
			"event_data": eventData,
			"created_at": leaseUntil,
		})
	if result.Error != nil {
		return RecallAttributionLease{}, false, result.Error
	}
	return lease, result.RowsAffected == 1, nil
}

func CompleteRecallAttributionCandidateWithContext(ctx context.Context, candidate RecallAttributionCandidate, lease RecallAttributionLease, completedAt int64, outcomeCode string) (bool, error) {
	return updateRecallAttributionProgressWithContext(ctx, candidate, lease, recallAttributionProgressTerminal, completedAt, outcomeCode)
}

func RetryRecallAttributionCandidateWithContext(ctx context.Context, candidate RecallAttributionCandidate, lease RecallAttributionLease, nextAttemptAt int64, outcomeCode string) (bool, error) {
	return updateRecallAttributionProgressWithContext(ctx, candidate, lease, recallAttributionProgressRetry, nextAttemptAt, outcomeCode)
}

func updateRecallAttributionProgressWithContext(ctx context.Context, candidate RecallAttributionCandidate, lease RecallAttributionLease, eventType string, dueAt int64, outcomeCode string) (bool, error) {
	sourceEventID := recallAttributionSourceEventID(candidate)
	if lease.SourceEventId != sourceEventID || lease.LeaseExpiresAt <= 0 || lease.Attempt <= 0 {
		return false, errors.New("invalid recall attribution lease")
	}
	eventData, err := marshalRecallAttributionProgressData(lease.Attempt, outcomeCode)
	if err != nil {
		return false, err
	}
	result := DB.WithContext(ctx).Model(&RecallEvent{}).
		Where("source = ? AND source_event_id = ? AND event_type = ? AND created_at = ?",
			recallAttributionProgressSource, sourceEventID, recallAttributionProgressLeased, lease.LeaseExpiresAt).
		Updates(map[string]any{
			"event_type": eventType,
			"event_data": eventData,
			"created_at": dueAt,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func recallAttributionSourceEventID(candidate RecallAttributionCandidate) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s\x00%s", candidate.UserId, strings.TrimSpace(candidate.TradeNo), strings.TrimSpace(candidate.CheckoutSessionId))))
	return "order:" + hex.EncodeToString(digest[:])
}

func marshalRecallAttributionProgressData(attempt int, outcomeCode string) (string, error) {
	outcomeCode = strings.TrimSpace(outcomeCode)
	if len(outcomeCode) > recallAttributionOutcomeMaxLength {
		return "", fmt.Errorf("recall attribution outcome code exceeds %d bytes", recallAttributionOutcomeMaxLength)
	}
	data, err := common.Marshal(recallAttributionProgressData{Attempt: attempt, OutcomeCode: outcomeCode})
	if err != nil {
		return "", fmt.Errorf("marshal recall attribution progress: %w", err)
	}
	return string(data), nil
}
