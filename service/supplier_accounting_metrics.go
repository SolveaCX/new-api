package service

import (
	"errors"

	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/types"
)

func init() {
	model.InstallSupplierAccountingConsumeLogWriteObserver(func(disposition types.SupplierAccountingDisposition, outcome model.SupplierAccountingConsumeLogWriteOutcome) {
		perfmetrics.RecordSupplierAccountingConsumeLogWrite(disposition, outcome)
	})
}

func recordSupplierAccountingSettlementMetrics(disposition types.SupplierAccountingDisposition, buildErr error) {
	perfmetrics.RecordSupplierAccountingSettlement(disposition)
	if disposition != types.SupplierAccountingDispositionProducerError {
		return
	}
	perfmetrics.RecordSupplierAccountingSnapshotBuildFailure(supplierAccountingBuildFailureMetricReason(buildErr))
}

func supplierAccountingBuildFailureMetricReason(err error) perfmetrics.SupplierAccountingBuildFailureReason {
	var accountingErr *SupplierAccountingError
	if !errors.As(err, &accountingErr) {
		return perfmetrics.SupplierAccountingBuildFailureUnknown
	}
	switch accountingErr.Reason {
	case SupplierAccountingReasonNegative:
		return perfmetrics.SupplierAccountingBuildFailureNegative
	case SupplierAccountingReasonNonFinite:
		return perfmetrics.SupplierAccountingBuildFailureNonFinite
	case SupplierAccountingReasonInvalidDivisor:
		return perfmetrics.SupplierAccountingBuildFailureInvalidDivisor
	case SupplierAccountingReasonOverflow:
		return perfmetrics.SupplierAccountingBuildFailureOverflow
	case SupplierAccountingReasonMissing:
		return perfmetrics.SupplierAccountingBuildFailureMissing
	default:
		return perfmetrics.SupplierAccountingBuildFailureUnknown
	}
}
