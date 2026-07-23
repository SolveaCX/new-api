package perfmetrics

import (
	"context"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestSupplierAccountingMetricsRejectUnboundedLabels(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	require.False(t, RecordSupplierAccountingSettlement("tenant-controlled"))
	require.False(t, RecordSupplierAccountingSnapshotBuildFailure("request-specific"))
	require.False(t, RecordSupplierAccountingConsumeLogWrite(types.SupplierAccountingDispositionCaptured, "maybe"))
	require.Zero(t, supplierAccountingPrometheusSeriesCount())
}

func TestSupplierAccountingMetricsRenderExactNamesAndLabels(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	require.True(t, RecordSupplierAccountingSettlement(types.SupplierAccountingDispositionCaptured))
	require.True(t, RecordSupplierAccountingSnapshotBuildFailure(SupplierAccountingBuildFailureMissing))
	require.True(t, RecordSupplierAccountingConsumeLogWrite(types.SupplierAccountingDispositionCaptured, model.SupplierAccountingConsumeLogWriteSuccess))

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	requirePrometheusSampleLine(t, text, `newapi_supplier_accounting_settlements_total{disposition="captured"} 1`)
	requirePrometheusSampleLine(t, text, `newapi_supplier_accounting_snapshot_build_failures_total{reason="missing"} 1`)
	requirePrometheusSampleLine(t, text, `newapi_supplier_accounting_consume_log_writes_total{disposition="captured",outcome="success"} 1`)
	requirePrometheusSeriesGaugeMatchesRenderedSamples(t, text)
}

func TestSupplierAccountingMetricsFixedUniverseAndConcurrency(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	const goroutines = 24
	const iterations = 500
	var wait sync.WaitGroup
	for index := 0; index < goroutines; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for iteration := 0; iteration < iterations; iteration++ {
				for _, disposition := range supplierAccountingDispositions {
					RecordSupplierAccountingSettlement(disposition)
					RecordSupplierAccountingConsumeLogWrite(disposition, model.SupplierAccountingConsumeLogWriteFailure)
				}
				for _, reason := range supplierAccountingBuildFailureReasons {
					RecordSupplierAccountingSnapshotBuildFailure(reason)
				}
			}
		}()
	}
	wait.Wait()

	expected := uint64(goroutines * iterations)
	for index := range supplierAccountingDispositions {
		require.Equal(t, expected, supplierAccountingMetricCounters.settlements[index].Load())
		require.Equal(t, expected, supplierAccountingMetricCounters.writes[index][supplierAccountingWriteOutcomeIndex(model.SupplierAccountingConsumeLogWriteFailure)].Load())
	}
	for index := range supplierAccountingBuildFailureReasons {
		require.Equal(t, expected, supplierAccountingMetricCounters.buildFailures[index].Load())
	}
}

func TestSupplierAccountingMetricsRespectPrometheusSeriesLimit(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	t.Setenv(prometheusMaxSeriesPerScrapeEnv, "1")
	require.True(t, RecordSupplierAccountingSettlement(types.SupplierAccountingDispositionCaptured))
	_, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)

	require.True(t, RecordSupplierAccountingSnapshotBuildFailure(SupplierAccountingBuildFailureMissing))
	_, err = BuildPrometheusText(context.Background())
	require.ErrorContains(t, err, "prometheus series limit exceeded: 2 > 1")
}
