package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestParseSupplierAccountingLogReadsCurrentCapturedEnvelope(t *testing.T) {
	envelope := BuildSupplierAccountingEnvelopeV1(supplierEnvelopeTestInput())
	require.Equal(t, types.SupplierAccountingDispositionCaptured, envelope.Disposition)
	payload, err := common.Marshal(map[string]any{types.SupplierAccountingEnvelopeKeyV1: envelope})
	require.NoError(t, err)

	snapshot, ok, err := parseSupplierAccountingLog(string(payload))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, envelope.Captured.SupplierId, snapshot.SupplierId)
	require.Equal(t, envelope.Captured.ContractId, snapshot.ContractId)
	require.Equal(t, envelope.Captured.RateVersionId, snapshot.RateVersionId)
	require.Equal(t, envelope.Captured.PricingProvenance, snapshot.PricingProvenance)
}

func TestParseSupplierAccountingLogReadsLegacySnapshotWithOverlappingShortKeys(t *testing.T) {
	const legacy = `{"supplier_accounting_v1":{"bv":8,"s":12,"c":13,"rv":14,"pm":650000,"sm":700000,"ol":1000000,"sa":700000,"pc":650000,"gp":50000,"ss":"business","ed":"included","q":"500000","p":"ratio","fc":1784801200}}`

	snapshot, ok, err := parseSupplierAccountingLog(legacy)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 8, snapshot.BindingVersionId)
	require.Equal(t, 12, snapshot.SupplierId)
	require.Equal(t, 13, snapshot.ContractId)
	require.Equal(t, 14, snapshot.RateVersionId)
	require.EqualValues(t, 650_000, snapshot.ProcurementMultiplierPpm)
}

func TestParseSupplierAccountingLogNeverFallsBackForMalformedCurrentEnvelope(t *testing.T) {
	tests := map[string]string{
		"unsupported version":   `{"supplier_accounting_v1":{"v":2,"c":1,"a":1,"d":"captured","s":"AQ"}}`,
		"missing disposition":   `{"supplier_accounting_v1":{"v":1,"c":1,"a":1,"s":"AQ"}}`,
		"non canonical payload": `{"supplier_accounting_v1":{"v":1,"c":1,"a":1,"d":"captured","s":"not+raw/base64="}}`,
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			_, ok, err := parseSupplierAccountingLog(payload)
			require.Error(t, err)
			require.False(t, ok)
		})
	}
}
