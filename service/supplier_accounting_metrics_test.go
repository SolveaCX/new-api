package service

import (
	"context"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func supplierAccountingMetricValue(t *testing.T, sample string) uint64 {
	t.Helper()
	text, err := perfmetrics.BuildPrometheusText(context.Background())
	require.NoError(t, err)
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, sample+" ") {
			continue
		}
		value, parseErr := strconv.ParseUint(strings.TrimPrefix(line, sample+" "), 10, 64)
		require.NoError(t, parseErr)
		return value
	}
	return 0
}

func TestSupplierAccountingSettlementMetricPrecedesConsumeLogObserver(t *testing.T) {
	settlementSample := `newapi_supplier_accounting_settlements_total{disposition="captured"}`
	writeSample := `newapi_supplier_accounting_consume_log_writes_total{disposition="captured",outcome="disabled"}`
	settlementsBefore := supplierAccountingMetricValue(t, settlementSample)
	writesBefore := supplierAccountingMetricValue(t, writeSample)

	other := map[string]any{}
	envelope := InjectSupplierAccountingEnvelopeV1(other, supplierEnvelopeTestInput())
	require.Equal(t, types.SupplierAccountingDispositionCaptured, envelope.Disposition)
	require.Equal(t, settlementsBefore+1, supplierAccountingMetricValue(t, settlementSample))
	require.Equal(t, writesBefore, supplierAccountingMetricValue(t, writeSample), "write outcome must not be emitted before RecordConsumeLog")

	originalEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = false
	t.Cleanup(func() { common.LogConsumeEnabled = originalEnabled })
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	model.RecordConsumeLog(ctx, 0, model.RecordConsumeLogParams{Other: other})
	require.Equal(t, writesBefore+1, supplierAccountingMetricValue(t, writeSample))
}

func TestSupplierAccountingBuildFailureMetricsOnlyForProducerError(t *testing.T) {
	missingSample := `newapi_supplier_accounting_snapshot_build_failures_total{reason="missing"}`
	missingBefore := supplierAccountingMetricValue(t, missingSample)

	uncommitted := supplierEnvelopeTestInput()
	uncommitted.Settlement.FinanciallyCommitted = false
	InjectSupplierAccountingEnvelopeV1(map[string]any{}, uncommitted)
	require.Equal(t, missingBefore, supplierAccountingMetricValue(t, missingSample))

	invalidSnapshot := supplierEnvelopeTestInput()
	invalidSnapshot.Capture.OfficialListUSD = nil
	envelope := InjectSupplierAccountingEnvelopeV1(map[string]any{}, invalidSnapshot)
	require.Equal(t, types.SupplierAccountingDispositionProducerError, envelope.Disposition)
	require.Equal(t, missingBefore+1, supplierAccountingMetricValue(t, missingSample))
}

func TestSupplierAccountingBuildFailureReasonMappingIsBounded(t *testing.T) {
	tests := []struct {
		reason SupplierAccountingFailureReason
		want   perfmetrics.SupplierAccountingBuildFailureReason
	}{
		{SupplierAccountingReasonNegative, perfmetrics.SupplierAccountingBuildFailureNegative},
		{SupplierAccountingReasonNonFinite, perfmetrics.SupplierAccountingBuildFailureNonFinite},
		{SupplierAccountingReasonInvalidDivisor, perfmetrics.SupplierAccountingBuildFailureInvalidDivisor},
		{SupplierAccountingReasonOverflow, perfmetrics.SupplierAccountingBuildFailureOverflow},
		{SupplierAccountingReasonMissing, perfmetrics.SupplierAccountingBuildFailureMissing},
		{SupplierAccountingReasonUnknown, perfmetrics.SupplierAccountingBuildFailureUnknown},
	}
	for _, testCase := range tests {
		err := newSupplierAccountingError("field", testCase.reason, nil)
		require.Equal(t, testCase.want, supplierAccountingBuildFailureMetricReason(err))
	}
	require.Equal(t, perfmetrics.SupplierAccountingBuildFailureUnknown, supplierAccountingBuildFailureMetricReason(nil))
}
