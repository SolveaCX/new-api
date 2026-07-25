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
	payload, err := common.Marshal(envelope)
	require.NoError(t, err)

	snapshot, err := parseSupplierAccountingFactPayload(string(payload))
	require.NoError(t, err)
	require.Equal(t, envelope.Captured.SupplierId, snapshot.SupplierId)
	require.Equal(t, envelope.Captured.ContractId, snapshot.ContractId)
	require.Equal(t, envelope.Captured.RateVersionId, snapshot.RateVersionId)
	require.Equal(t, envelope.Captured.PricingProvenance, snapshot.PricingProvenance)
}

func TestParseSupplierAccountingLogNeverFallsBackForMalformedCurrentEnvelope(t *testing.T) {
	tests := map[string]string{
		"unsupported version":   `{"v":2,"d":"captured","s":"AQ"}`,
		"missing disposition":   `{"v":1,"s":"AQ"}`,
		"non canonical payload": `{"v":1,"d":"captured","s":"not+raw/base64="}`,
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := parseSupplierAccountingFactPayload(payload)
			require.Error(t, err)
		})
	}
}
