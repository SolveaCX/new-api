package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

const (
	SupplierPublishedEvidenceSchemaVersion = 1
	SupplierPublishedEvidenceMaxBytes      = 16 * 1024
	SupplierPublishedEvidenceMaxWarnings   = 16

	SupplierPersistedLogCompletenessComplete   = "complete"
	SupplierPersistedLogCompletenessIncomplete = "incomplete"
	SupplierPersistedLogCompletenessNotScanned = "not_scanned"

	SupplierPublishedWarningProducerError         = "producer_error"
	SupplierPublishedWarningUnknownProducer       = "unknown_producer_capability"
	SupplierPublishedWarningAbsentMarker          = "absent_marker_after_cutover"
	SupplierPublishedWarningIncompatibleProducer  = "incompatible_producer"
	SupplierPublishedWarningInvalidCaptured       = "invalid_captured_snapshot"
	SupplierPublishedWarningUnknownOfficialAmount = "unknown_official_amount"
)

var ErrInvalidSupplierPublishedEvidence = errors.New("invalid supplier published evidence")

type SupplierPublishedDispositionCountsV1 struct {
	Captured                int64 `json:"captured"`
	UnsupportedPath         int64 `json:"unsupported_path"`
	NotFinanciallyCommitted int64 `json:"not_financially_committed"`
	ZeroUsage               int64 `json:"zero_usage"`
	Unbound                 int64 `json:"unbound"`
	ProducerError           int64 `json:"producer_error"`
}

type SupplierPublishedFailureCountsV1 struct {
	UnknownProducerCapability      int64 `json:"unknown_producer_capability"`
	IncompatibleProducerCapability int64 `json:"incompatible_producer_capability"`
	AbsentMarkerAfterCutover       int64 `json:"absent_marker_after_cutover"`
	InvalidCapturedSnapshot        int64 `json:"invalid_captured_snapshot"`
	UnknownOfficialAmount          int64 `json:"unknown_official_amount"`
}

type SupplierPublishedWarningV1 struct {
	Code       string `json:"code"`
	Count      int64  `json:"count"`
	MessageKey string `json:"message_key"`
}

type SupplierPublishedEvidenceV1 struct {
	SchemaVersion                    int                                  `json:"schema_version"`
	LogsScanned                      int64                                `json:"logs_scanned"`
	ProducerMarkersPresent           int64                                `json:"producer_markers_present"`
	CapturedSnapshotCount            int64                                `json:"captured_snapshot_count"`
	DispositionCounts                SupplierPublishedDispositionCountsV1 `json:"disposition_counts"`
	FailureCounts                    SupplierPublishedFailureCountsV1     `json:"failure_counts"`
	PersistedLogSnapshotCompleteness string                               `json:"persisted_log_snapshot_completeness"`
	Warnings                         []SupplierPublishedWarningV1         `json:"warnings"`
}

var supplierPublishedWarningMessageKeys = map[string]string{
	SupplierPublishedWarningProducerError:         "supply_chain.warning.producer_error",
	SupplierPublishedWarningUnknownProducer:       "supply_chain.warning.unknown_producer_capability",
	SupplierPublishedWarningAbsentMarker:          "supply_chain.warning.absent_marker_after_cutover",
	SupplierPublishedWarningIncompatibleProducer:  "supply_chain.warning.incompatible_producer",
	SupplierPublishedWarningInvalidCaptured:       "supply_chain.warning.invalid_captured_snapshot",
	SupplierPublishedWarningUnknownOfficialAmount: "supply_chain.warning.unknown_official_amount",
}

func ValidateSupplierPublishedEvidenceV1(evidence SupplierPublishedEvidenceV1) error {
	if evidence.SchemaVersion != SupplierPublishedEvidenceSchemaVersion || evidence.LogsScanned < 0 ||
		evidence.ProducerMarkersPresent < 0 || evidence.ProducerMarkersPresent > evidence.LogsScanned ||
		evidence.CapturedSnapshotCount < 0 || evidence.CapturedSnapshotCount > evidence.LogsScanned {
		return ErrInvalidSupplierPublishedEvidence
	}
	counts := evidence.DispositionCounts
	if counts.Captured < 0 || counts.UnsupportedPath < 0 || counts.NotFinanciallyCommitted < 0 ||
		counts.ZeroUsage < 0 || counts.Unbound < 0 || counts.ProducerError < 0 || counts.Captured != evidence.CapturedSnapshotCount {
		return ErrInvalidSupplierPublishedEvidence
	}
	dispositionTotal, ok := checkedSupplierPublishedCountSum(
		counts.Captured,
		counts.UnsupportedPath,
		counts.NotFinanciallyCommitted,
		counts.ZeroUsage,
		counts.Unbound,
		counts.ProducerError,
	)
	if !ok || dispositionTotal > evidence.ProducerMarkersPresent {
		return ErrInvalidSupplierPublishedEvidence
	}
	failures := evidence.FailureCounts
	if failures.UnknownProducerCapability < 0 || failures.IncompatibleProducerCapability < 0 ||
		failures.AbsentMarkerAfterCutover < 0 || failures.InvalidCapturedSnapshot < 0 || failures.UnknownOfficialAmount < 0 {
		return ErrInvalidSupplierPublishedEvidence
	}
	failureTotal, ok := checkedSupplierPublishedCountSum(
		failures.UnknownProducerCapability,
		failures.IncompatibleProducerCapability,
		failures.AbsentMarkerAfterCutover,
		failures.InvalidCapturedSnapshot,
		failures.UnknownOfficialAmount,
	)
	classifiedFailureTotal, classifiedOK := checkedSupplierPublishedCountSum(
		failures.UnknownProducerCapability,
		failures.IncompatibleProducerCapability,
		failures.InvalidCapturedSnapshot,
	)
	if failures.UnknownProducerCapability > evidence.LogsScanned || failures.IncompatibleProducerCapability > evidence.LogsScanned ||
		failures.AbsentMarkerAfterCutover > evidence.LogsScanned || failures.InvalidCapturedSnapshot > evidence.LogsScanned || failures.UnknownOfficialAmount > evidence.LogsScanned ||
		!ok || !classifiedOK ||
		failures.AbsentMarkerAfterCutover != evidence.LogsScanned-evidence.ProducerMarkersPresent ||
		classifiedFailureTotal != evidence.ProducerMarkersPresent-dispositionTotal ||
		failures.UnknownOfficialAmount > failures.InvalidCapturedSnapshot {
		return ErrInvalidSupplierPublishedEvidence
	}
	switch evidence.PersistedLogSnapshotCompleteness {
	case SupplierPersistedLogCompletenessComplete:
		if failureTotal != 0 || counts.ProducerError != 0 || evidence.ProducerMarkersPresent != evidence.LogsScanned {
			return ErrInvalidSupplierPublishedEvidence
		}
	case SupplierPersistedLogCompletenessIncomplete:
		if failureTotal == 0 && counts.ProducerError == 0 && evidence.ProducerMarkersPresent == evidence.LogsScanned {
			return ErrInvalidSupplierPublishedEvidence
		}
	case SupplierPersistedLogCompletenessNotScanned:
		if evidence.LogsScanned != 0 || evidence.ProducerMarkersPresent != 0 || failureTotal != 0 {
			return ErrInvalidSupplierPublishedEvidence
		}
	default:
		return ErrInvalidSupplierPublishedEvidence
	}
	if evidence.Warnings == nil || len(evidence.Warnings) > SupplierPublishedEvidenceMaxWarnings {
		return ErrInvalidSupplierPublishedEvidence
	}
	expectedWarningCounts := map[string]int64{
		SupplierPublishedWarningProducerError:         counts.ProducerError,
		SupplierPublishedWarningUnknownProducer:       failures.UnknownProducerCapability,
		SupplierPublishedWarningAbsentMarker:          failures.AbsentMarkerAfterCutover,
		SupplierPublishedWarningIncompatibleProducer:  failures.IncompatibleProducerCapability,
		SupplierPublishedWarningInvalidCaptured:       failures.InvalidCapturedSnapshot,
		SupplierPublishedWarningUnknownOfficialAmount: failures.UnknownOfficialAmount,
	}
	seen := make(map[string]struct{}, len(evidence.Warnings))
	for _, warning := range evidence.Warnings {
		expectedKey, ok := supplierPublishedWarningMessageKeys[warning.Code]
		expectedCount := expectedWarningCounts[warning.Code]
		if !ok || expectedCount == 0 || warning.Count != expectedCount || warning.MessageKey != expectedKey || warning.Count > evidence.LogsScanned ||
			!utf8.ValidString(warning.MessageKey) || strings.TrimSpace(warning.MessageKey) != warning.MessageKey {
			return ErrInvalidSupplierPublishedEvidence
		}
		if _, duplicate := seen[warning.Code]; duplicate {
			return ErrInvalidSupplierPublishedEvidence
		}
		seen[warning.Code] = struct{}{}
	}
	for code, count := range expectedWarningCounts {
		if count == 0 {
			continue
		}
		if _, present := seen[code]; !present {
			return ErrInvalidSupplierPublishedEvidence
		}
	}
	return nil
}

func checkedSupplierPublishedCountSum(values ...int64) (int64, bool) {
	var total int64
	for _, value := range values {
		if value < 0 || value > math.MaxInt64-total {
			return 0, false
		}
		total += value
	}
	return total, true
}

func EncodeSupplierPublishedEvidenceV1(evidence SupplierPublishedEvidenceV1) (string, error) {
	if err := ValidateSupplierPublishedEvidenceV1(evidence); err != nil {
		return "", err
	}
	encoded, err := common.Marshal(evidence)
	if err != nil {
		return "", fmt.Errorf("encode supplier published evidence: %w", err)
	}
	if len(encoded) == 0 || len(encoded) > SupplierPublishedEvidenceMaxBytes || !utf8.Valid(encoded) {
		return "", ErrInvalidSupplierPublishedEvidence
	}
	return string(encoded), nil
}

func ParseSupplierPublishedEvidenceV1(raw string) (SupplierPublishedEvidenceV1, error) {
	var evidence SupplierPublishedEvidenceV1
	if raw == "" || len(raw) > SupplierPublishedEvidenceMaxBytes || !utf8.ValidString(raw) || strings.TrimSpace(raw) != raw {
		return evidence, ErrInvalidSupplierPublishedEvidence
	}
	top, err := requiredStrictObject(raw, []string{
		"schema_version", "logs_scanned", "producer_markers_present", "captured_snapshot_count",
		"disposition_counts", "failure_counts", "persisted_log_snapshot_completeness", "warnings",
	})
	if err != nil {
		return evidence, fmt.Errorf("parse supplier published evidence: %w", err)
	}
	if _, err = requiredStrictRawObject(top["disposition_counts"], []string{
		"captured", "unsupported_path", "not_financially_committed", "zero_usage", "unbound", "producer_error",
	}); err != nil {
		return evidence, fmt.Errorf("parse supplier published disposition counts: %w", err)
	}
	if _, err = requiredStrictRawObject(top["failure_counts"], []string{
		"unknown_producer_capability", "incompatible_producer_capability", "absent_marker_after_cutover",
		"invalid_captured_snapshot", "unknown_official_amount",
	}); err != nil {
		return evidence, fmt.Errorf("parse supplier published failure counts: %w", err)
	}
	var warnings []json.RawMessage
	if common.GetJsonType(top["warnings"]) != "array" {
		return evidence, ErrInvalidSupplierPublishedEvidence
	}
	if err = common.Unmarshal(top["warnings"], &warnings); err != nil || len(warnings) > SupplierPublishedEvidenceMaxWarnings {
		return evidence, ErrInvalidSupplierPublishedEvidence
	}
	for _, warning := range warnings {
		if _, err = requiredStrictRawObject(warning, []string{"code", "count", "message_key"}); err != nil {
			return evidence, fmt.Errorf("parse supplier published warning: %w", err)
		}
	}
	if err = common.UnmarshalJsonStr(raw, &evidence); err != nil {
		return evidence, fmt.Errorf("parse supplier published evidence: %w", err)
	}
	if err = ValidateSupplierPublishedEvidenceV1(evidence); err != nil {
		return evidence, err
	}
	return evidence, nil
}

func requiredStrictRawObject(raw json.RawMessage, fields []string) (map[string]json.RawMessage, error) {
	return requiredStrictObject(string(raw), fields)
}

func requiredStrictObject(raw string, fields []string) (map[string]json.RawMessage, error) {
	allowed := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		allowed[field] = struct{}{}
	}
	values, err := common.UnmarshalJsonObjectStrict(raw, allowed)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		if _, ok := values[field]; !ok {
			return nil, fmt.Errorf("missing field %q", field)
		}
	}
	return values, nil
}
