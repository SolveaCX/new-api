package service

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

type supplierBatchLogFailure string

const (
	supplierBatchLogFailureNone                 supplierBatchLogFailure = ""
	supplierBatchLogFailureUnknownProducer      supplierBatchLogFailure = "unknown_producer"
	supplierBatchLogFailureIncompatibleProducer supplierBatchLogFailure = "incompatible_producer"
	supplierBatchLogFailureAbsentMarker         supplierBatchLogFailure = "absent_marker"
	supplierBatchLogFailureInvalidCaptured      supplierBatchLogFailure = "invalid_captured_snapshot"
	supplierBatchLogFailureUnknownOfficial      supplierBatchLogFailure = "unknown_official_amount"
)

type supplierBatchLogClassification struct {
	producerMarkerPresent bool
	disposition           types.SupplierAccountingDisposition
	snapshot              *types.SupplierAccountingLogSnapshotV1
	failure               supplierBatchLogFailure
}

type supplierBatchEvidenceAccumulator struct {
	evidence types.SupplierPublishedEvidenceV1
}

func newSupplierBatchEvidenceAccumulator() *supplierBatchEvidenceAccumulator {
	return &supplierBatchEvidenceAccumulator{evidence: types.SupplierPublishedEvidenceV1{
		SchemaVersion:                    types.SupplierPublishedEvidenceSchemaVersion,
		PersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessComplete,
		Warnings:                         []types.SupplierPublishedWarningV1{},
	}}
}

func (a *supplierBatchEvidenceAccumulator) observe(classification supplierBatchLogClassification) {
	a.evidence.LogsScanned++
	if classification.producerMarkerPresent {
		a.evidence.ProducerMarkersPresent++
	}
	if classification.snapshot != nil {
		a.evidence.CapturedSnapshotCount++
		a.evidence.DispositionCounts.Captured++
	} else if classification.failure == supplierBatchLogFailureNone {
		switch classification.disposition {
		case types.SupplierAccountingDispositionUnsupportedPath:
			a.evidence.DispositionCounts.UnsupportedPath++
		case types.SupplierAccountingDispositionNotFinanciallyCommitted:
			a.evidence.DispositionCounts.NotFinanciallyCommitted++
		case types.SupplierAccountingDispositionZeroUsage:
			a.evidence.DispositionCounts.ZeroUsage++
		case types.SupplierAccountingDispositionUnbound:
			a.evidence.DispositionCounts.Unbound++
		case types.SupplierAccountingDispositionProducerError:
			a.evidence.DispositionCounts.ProducerError++
		}
	}
	switch classification.failure {
	case supplierBatchLogFailureUnknownProducer:
		a.evidence.FailureCounts.UnknownProducerCapability++
	case supplierBatchLogFailureIncompatibleProducer:
		a.evidence.FailureCounts.IncompatibleProducerCapability++
	case supplierBatchLogFailureAbsentMarker:
		a.evidence.FailureCounts.AbsentMarkerAfterCutover++
	case supplierBatchLogFailureInvalidCaptured:
		a.evidence.FailureCounts.InvalidCapturedSnapshot++
	case supplierBatchLogFailureUnknownOfficial:
		a.evidence.FailureCounts.InvalidCapturedSnapshot++
		a.evidence.FailureCounts.UnknownOfficialAmount++
	}
}

func (a *supplierBatchEvidenceAccumulator) publishedEvidence() (types.SupplierPublishedEvidenceV1, error) {
	evidence := a.evidence
	warnings := make([]types.SupplierPublishedWarningV1, 0, 5)
	appendWarning := func(code string, count int64, messageKey string) {
		if count > 0 {
			warnings = append(warnings, types.SupplierPublishedWarningV1{Code: code, Count: count, MessageKey: messageKey})
		}
	}
	appendWarning(types.SupplierPublishedWarningProducerError, evidence.DispositionCounts.ProducerError, "supply_chain.warning.producer_error")
	appendWarning(types.SupplierPublishedWarningUnknownProducer, evidence.FailureCounts.UnknownProducerCapability, "supply_chain.warning.unknown_producer_capability")
	appendWarning(types.SupplierPublishedWarningAbsentMarker, evidence.FailureCounts.AbsentMarkerAfterCutover, "supply_chain.warning.absent_marker_after_cutover")
	appendWarning(types.SupplierPublishedWarningIncompatibleProducer, evidence.FailureCounts.IncompatibleProducerCapability, "supply_chain.warning.incompatible_producer")
	appendWarning(types.SupplierPublishedWarningInvalidCaptured, evidence.FailureCounts.InvalidCapturedSnapshot, "supply_chain.warning.invalid_captured_snapshot")
	appendWarning(types.SupplierPublishedWarningUnknownOfficialAmount, evidence.FailureCounts.UnknownOfficialAmount, "supply_chain.warning.unknown_official_amount")
	evidence.Warnings = warnings
	failures := evidence.FailureCounts
	if evidence.DispositionCounts.ProducerError > 0 || failures.UnknownProducerCapability > 0 || failures.IncompatibleProducerCapability > 0 ||
		failures.AbsentMarkerAfterCutover > 0 || failures.InvalidCapturedSnapshot > 0 || failures.UnknownOfficialAmount > 0 {
		evidence.PersistedLogSnapshotCompleteness = types.SupplierPersistedLogCompletenessIncomplete
	}
	if err := types.ValidateSupplierPublishedEvidenceV1(evidence); err != nil {
		return types.SupplierPublishedEvidenceV1{}, err
	}
	return evidence, nil
}

// classifySupplierAccountingLog turns persisted-row defects into evidence,
// not execution errors. Only a validated captured envelope contributes money.
func classifySupplierAccountingLog(other string) supplierBatchLogClassification {
	trimmed := strings.TrimSpace(other)
	if trimmed == "" || !strings.Contains(trimmed, `"`+types.SupplierAccountingEnvelopeKeyV1+`"`) {
		return supplierBatchLogClassification{failure: supplierBatchLogFailureAbsentMarker}
	}

	var outer supplierAccountingLogEnvelope
	if err := common.Unmarshal([]byte(trimmed), &outer); err != nil || len(outer.SupplierAccountingV1) == 0 || string(outer.SupplierAccountingV1) == "null" {
		return supplierBatchLogClassification{producerMarkerPresent: true, failure: supplierBatchLogFailureUnknownProducer}
	}

	control, controlOK := supplierAccountingEnvelopeControl(outer.SupplierAccountingV1)
	if !controlOK {
		// Legacy snapshot objects deliberately decode through the compatibility
		// helper, but they do not have a numeric producer capability and are not
		// accounting-eligible for the current publication contract.
		if supplierLegacyUnknownOfficialAmount(outer.SupplierAccountingV1) {
			return supplierBatchLogClassification{producerMarkerPresent: true, failure: supplierBatchLogFailureUnknownOfficial}
		}
		return supplierBatchLogClassification{producerMarkerPresent: true, failure: supplierBatchLogFailureUnknownProducer}
	}
	if control.schemaVersion != types.SupplierAccountingEnvelopeSchemaVersionV1 ||
		control.capabilityVersion != types.SupplierAccountingProducerCapabilityV1 {
		return supplierBatchLogClassification{producerMarkerPresent: true, failure: supplierBatchLogFailureIncompatibleProducer}
	}

	envelope, err := types.ParseSupplierAccountingEnvelopeV1JSON(outer.SupplierAccountingV1)
	if err != nil {
		failure := supplierBatchLogFailureUnknownProducer
		if control.disposition == types.SupplierAccountingDispositionCaptured {
			failure = supplierBatchLogFailureInvalidCaptured
		}
		return supplierBatchLogClassification{producerMarkerPresent: true, disposition: control.disposition, failure: failure}
	}
	if err := ValidateSupplierAccountingEnvelopeV1(envelope); err != nil {
		failure := supplierBatchLogFailureUnknownProducer
		if envelope.Disposition == types.SupplierAccountingDispositionCaptured {
			failure = supplierBatchLogFailureInvalidCaptured
		}
		return supplierBatchLogClassification{producerMarkerPresent: true, disposition: envelope.Disposition, failure: failure}
	}
	if envelope.Disposition == types.SupplierAccountingDispositionProducerError {
		return supplierBatchLogClassification{producerMarkerPresent: true, disposition: envelope.Disposition}
	}
	if envelope.Disposition != types.SupplierAccountingDispositionCaptured {
		return supplierBatchLogClassification{producerMarkerPresent: true, disposition: envelope.Disposition}
	}
	if envelope.Captured.UnknownOfficialCount != 0 {
		return supplierBatchLogClassification{producerMarkerPresent: true, disposition: envelope.Disposition, failure: supplierBatchLogFailureUnknownOfficial}
	}
	return supplierBatchLogClassification{producerMarkerPresent: true, disposition: envelope.Disposition, snapshot: envelope.Captured}
}

func supplierLegacyUnknownOfficialAmount(raw json.RawMessage) bool {
	var fields map[string]json.RawMessage
	if common.Unmarshal(raw, &fields) != nil || common.GetJsonType(fields["uo"]) != "number" {
		return false
	}
	var count uint32
	return common.Unmarshal(fields["uo"], &count) == nil && count > 0
}

type supplierAccountingEnvelopeControlFields struct {
	schemaVersion     int
	capabilityVersion int
	disposition       types.SupplierAccountingDisposition
}

func supplierAccountingEnvelopeControl(raw json.RawMessage) (supplierAccountingEnvelopeControlFields, bool) {
	var fields map[string]json.RawMessage
	if err := common.Unmarshal(raw, &fields); err != nil {
		return supplierAccountingEnvelopeControlFields{}, false
	}
	var control supplierAccountingEnvelopeControlFields
	if common.GetJsonType(fields["v"]) != "number" || common.Unmarshal(fields["v"], &control.schemaVersion) != nil {
		return supplierAccountingEnvelopeControlFields{}, false
	}
	if common.GetJsonType(fields["c"]) != "number" || common.Unmarshal(fields["c"], &control.capabilityVersion) != nil {
		return supplierAccountingEnvelopeControlFields{}, false
	}
	if common.GetJsonType(fields["d"]) != "string" || common.Unmarshal(fields["d"], &control.disposition) != nil {
		return supplierAccountingEnvelopeControlFields{}, false
	}
	return control, true
}
