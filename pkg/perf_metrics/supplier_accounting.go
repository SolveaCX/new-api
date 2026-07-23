package perfmetrics

import (
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
)

type SupplierAccountingBuildFailureReason string

const (
	SupplierAccountingBuildFailureNegative       SupplierAccountingBuildFailureReason = "negative"
	SupplierAccountingBuildFailureNonFinite      SupplierAccountingBuildFailureReason = "non_finite"
	SupplierAccountingBuildFailureInvalidDivisor SupplierAccountingBuildFailureReason = "invalid_divisor"
	SupplierAccountingBuildFailureOverflow       SupplierAccountingBuildFailureReason = "overflow"
	SupplierAccountingBuildFailureUnknown        SupplierAccountingBuildFailureReason = "unknown"
	SupplierAccountingBuildFailureMissing        SupplierAccountingBuildFailureReason = "missing"
)

var supplierAccountingDispositions = [...]types.SupplierAccountingDisposition{
	types.SupplierAccountingDispositionUnsupportedPath,
	types.SupplierAccountingDispositionNotFinanciallyCommitted,
	types.SupplierAccountingDispositionZeroUsage,
	types.SupplierAccountingDispositionUnbound,
	types.SupplierAccountingDispositionCaptured,
	types.SupplierAccountingDispositionProducerError,
}

var supplierAccountingBuildFailureReasons = [...]SupplierAccountingBuildFailureReason{
	SupplierAccountingBuildFailureNegative,
	SupplierAccountingBuildFailureNonFinite,
	SupplierAccountingBuildFailureInvalidDivisor,
	SupplierAccountingBuildFailureOverflow,
	SupplierAccountingBuildFailureUnknown,
	SupplierAccountingBuildFailureMissing,
}

var supplierAccountingWriteOutcomes = [...]model.SupplierAccountingConsumeLogWriteOutcome{
	model.SupplierAccountingConsumeLogWriteSuccess,
	model.SupplierAccountingConsumeLogWriteFailure,
	model.SupplierAccountingConsumeLogWriteDisabled,
}

type supplierAccountingCounters struct {
	settlements   [len(supplierAccountingDispositions)]atomic.Uint64
	buildFailures [len(supplierAccountingBuildFailureReasons)]atomic.Uint64
	writes        [len(supplierAccountingDispositions)][len(supplierAccountingWriteOutcomes)]atomic.Uint64
}

var supplierAccountingMetricCounters supplierAccountingCounters

func RecordSupplierAccountingSettlement(disposition types.SupplierAccountingDisposition) bool {
	index := supplierAccountingDispositionIndex(disposition)
	if index < 0 {
		return false
	}
	supplierAccountingMetricCounters.settlements[index].Add(1)
	return true
}

func RecordSupplierAccountingSnapshotBuildFailure(reason SupplierAccountingBuildFailureReason) bool {
	index := supplierAccountingBuildFailureReasonIndex(reason)
	if index < 0 {
		return false
	}
	supplierAccountingMetricCounters.buildFailures[index].Add(1)
	return true
}

func RecordSupplierAccountingConsumeLogWrite(disposition types.SupplierAccountingDisposition, outcome model.SupplierAccountingConsumeLogWriteOutcome) bool {
	dispositionIndex := supplierAccountingDispositionIndex(disposition)
	outcomeIndex := supplierAccountingWriteOutcomeIndex(outcome)
	if dispositionIndex < 0 || outcomeIndex < 0 {
		return false
	}
	supplierAccountingMetricCounters.writes[dispositionIndex][outcomeIndex].Add(1)
	return true
}

func supplierAccountingDispositionIndex(disposition types.SupplierAccountingDisposition) int {
	for index, candidate := range supplierAccountingDispositions {
		if candidate == disposition {
			return index
		}
	}
	return -1
}

func supplierAccountingBuildFailureReasonIndex(reason SupplierAccountingBuildFailureReason) int {
	for index, candidate := range supplierAccountingBuildFailureReasons {
		if candidate == reason {
			return index
		}
	}
	return -1
}

func supplierAccountingWriteOutcomeIndex(outcome model.SupplierAccountingConsumeLogWriteOutcome) int {
	for index, candidate := range supplierAccountingWriteOutcomes {
		if candidate == outcome {
			return index
		}
	}
	return -1
}

func supplierAccountingPrometheusSeriesCount() int {
	count := 0
	for index := range supplierAccountingDispositions {
		if supplierAccountingMetricCounters.settlements[index].Load() > 0 {
			count++
		}
		for outcomeIndex := range supplierAccountingWriteOutcomes {
			if supplierAccountingMetricCounters.writes[index][outcomeIndex].Load() > 0 {
				count++
			}
		}
	}
	for index := range supplierAccountingBuildFailureReasons {
		if supplierAccountingMetricCounters.buildFailures[index].Load() > 0 {
			count++
		}
	}
	return count
}

func writeSupplierAccountingPrometheusMetrics(b *strings.Builder) {
	b.WriteString("# HELP newapi_supplier_accounting_settlements_total Supplier accounting settlements by fixed V1 disposition.\n")
	b.WriteString("# TYPE newapi_supplier_accounting_settlements_total counter\n")
	for index, disposition := range supplierAccountingDispositions {
		value := supplierAccountingMetricCounters.settlements[index].Load()
		if value == 0 {
			continue
		}
		b.WriteString(`newapi_supplier_accounting_settlements_total{disposition="`)
		b.WriteString(escapePrometheusLabelValue(string(disposition)))
		b.WriteString(`"} `)
		b.WriteString(strconv.FormatUint(value, 10))
		b.WriteByte('\n')
	}

	b.WriteString("# HELP newapi_supplier_accounting_snapshot_build_failures_total Otherwise-capturable supplier snapshot build failures by bounded reason.\n")
	b.WriteString("# TYPE newapi_supplier_accounting_snapshot_build_failures_total counter\n")
	for index, reason := range supplierAccountingBuildFailureReasons {
		value := supplierAccountingMetricCounters.buildFailures[index].Load()
		if value == 0 {
			continue
		}
		b.WriteString(`newapi_supplier_accounting_snapshot_build_failures_total{reason="`)
		b.WriteString(escapePrometheusLabelValue(string(reason)))
		b.WriteString(`"} `)
		b.WriteString(strconv.FormatUint(value, 10))
		b.WriteByte('\n')
	}

	b.WriteString("# HELP newapi_supplier_accounting_consume_log_writes_total Supplier consume-log write outcomes by fixed V1 disposition.\n")
	b.WriteString("# TYPE newapi_supplier_accounting_consume_log_writes_total counter\n")
	for dispositionIndex, disposition := range supplierAccountingDispositions {
		for outcomeIndex, outcome := range supplierAccountingWriteOutcomes {
			value := supplierAccountingMetricCounters.writes[dispositionIndex][outcomeIndex].Load()
			if value == 0 {
				continue
			}
			b.WriteString(`newapi_supplier_accounting_consume_log_writes_total{disposition="`)
			b.WriteString(escapePrometheusLabelValue(string(disposition)))
			b.WriteString(`",outcome="`)
			b.WriteString(escapePrometheusLabelValue(string(outcome)))
			b.WriteString(`"} `)
			b.WriteString(strconv.FormatUint(value, 10))
			b.WriteByte('\n')
		}
	}
}
