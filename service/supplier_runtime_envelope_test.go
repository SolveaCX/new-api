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

func TestParseSupplierAccountingLogNeverFallsBackForMalformedCurrentEnvelope(t *testing.T) {
	tests := map[string]string{
		"unsupported version":   `{"supplier_accounting_v1":{"v":2,"d":"captured","s":"AQ"}}`,
		"missing disposition":   `{"supplier_accounting_v1":{"v":1,"s":"AQ"}}`,
		"non canonical payload": `{"supplier_accounting_v1":{"v":1,"d":"captured","s":"not+raw/base64="}}`,
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			_, ok, err := parseSupplierAccountingLog(payload)
			require.Error(t, err)
			require.False(t, ok)
		})
	}
}
