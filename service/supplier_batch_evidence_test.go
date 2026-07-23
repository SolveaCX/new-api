package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestClassifySupplierAccountingLogCurrentEnvelope(t *testing.T) {
	envelope := BuildSupplierAccountingEnvelopeV1(supplierEnvelopeTestInput())
	payload, err := common.Marshal(map[string]any{types.SupplierAccountingEnvelopeKeyV1: envelope})
	require.NoError(t, err)

	classified := classifySupplierAccountingLog(string(payload))
	require.True(t, classified.producerMarkerPresent)
	require.Equal(t, types.SupplierAccountingDispositionCaptured, classified.disposition)
	require.Equal(t, supplierBatchLogFailureNone, classified.failure)
	require.NotNil(t, classified.snapshot)
	mode, err := supplierPricingModeFromProvenance(classified.snapshot.PricingProvenance)
	require.NoError(t, err)
	require.Equal(t, string(types.SupplierPricingModeRatio), mode)
}

func TestClassifySupplierAccountingLogLegacyEnvelopeIsEvidenceGap(t *testing.T) {
	const legacy = `{"supplier_accounting_v1":{"bv":8,"s":12,"c":13,"rv":14,"pm":650000,"sm":700000,"ol":1000000,"sa":700000,"pc":650000,"gp":50000,"ss":"business","ed":"included","q":"500000","p":"ratio","fc":1784801200}}`

	classified := classifySupplierAccountingLog(legacy)
	require.True(t, classified.producerMarkerPresent)
	require.Equal(t, supplierBatchLogFailureUnknownProducer, classified.failure)
	require.Nil(t, classified.snapshot)

	// The compatibility decoder remains available for historical inspection;
	// current publication classification never treats it as captured evidence.
	snapshot, ok, err := parseSupplierAccountingLog(legacy)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 12, snapshot.SupplierId)
}

func TestClassifySupplierAccountingLogDefectsAreIncompleteEvidenceNotScanErrors(t *testing.T) {
	tests := map[string]struct {
		other       string
		failure     supplierBatchLogFailure
		disposition types.SupplierAccountingDisposition
		marker      bool
	}{
		"absent marker": {
			other: `{}`, failure: supplierBatchLogFailureAbsentMarker,
		},
		"unknown capability": {
			other:   `{"supplier_accounting_v1":{"v":1,"a":1,"d":"captured","s":"AQ"}}`,
			failure: supplierBatchLogFailureUnknownProducer, marker: true,
		},
		"incompatible capability": {
			other:   `{"supplier_accounting_v1":{"v":1,"c":2,"a":1,"d":"captured","s":"AQ"}}`,
			failure: supplierBatchLogFailureIncompatibleProducer, marker: true,
		},
		"invalid captured payload": {
			other:   `{"supplier_accounting_v1":{"v":1,"c":1,"a":1,"d":"captured","s":"AQ"}}`,
			failure: supplierBatchLogFailureInvalidCaptured, disposition: types.SupplierAccountingDispositionCaptured, marker: true,
		},
	}
	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			classified := classifySupplierAccountingLog(testCase.other)
			require.Equal(t, testCase.marker, classified.producerMarkerPresent)
			require.Equal(t, testCase.failure, classified.failure)
			require.Equal(t, testCase.disposition, classified.disposition)
			require.Nil(t, classified.snapshot)
		})
	}
}

func TestUnknownOfficialAmountPublishesValidIncompleteEvidence(t *testing.T) {
	const legacyUnknownOfficial = `{"supplier_accounting_v1":{"bv":8,"s":12,"c":13,"rv":14,"pm":650000,"sm":700000,"ol":1000000,"sa":700000,"pc":650000,"gp":50000,"ss":"business","ed":"included","fc":1784801200,"uo":1}}`
	classified := classifySupplierAccountingLog(legacyUnknownOfficial)
	require.Equal(t, supplierBatchLogFailureUnknownOfficial, classified.failure)
	require.Nil(t, classified.snapshot, "legacy diagnostics must never become accounting-eligible money")
	accumulator := newSupplierBatchEvidenceAccumulator()
	accumulator.observe(classified)
	evidence, err := accumulator.publishedEvidence()
	require.NoError(t, err)
	require.Equal(t, types.SupplierPersistedLogCompletenessIncomplete, evidence.PersistedLogSnapshotCompleteness)
	require.EqualValues(t, 1, evidence.FailureCounts.InvalidCapturedSnapshot)
	require.EqualValues(t, 1, evidence.FailureCounts.UnknownOfficialAmount)
	require.ElementsMatch(t, []string{
		types.SupplierPublishedWarningInvalidCaptured,
		types.SupplierPublishedWarningUnknownOfficialAmount,
	}, []string{evidence.Warnings[0].Code, evidence.Warnings[1].Code})
}

func TestCurrentProducerErrorRemainsDispositionEvidence(t *testing.T) {
	input := supplierEnvelopeTestInput()
	input.Capture.UnknownOfficialAmountCount = 1
	envelope := BuildSupplierAccountingEnvelopeV1(input)
	require.Equal(t, types.SupplierAccountingDispositionProducerError, envelope.Disposition)
	require.Nil(t, envelope.Captured)
	payload, err := common.Marshal(map[string]any{types.SupplierAccountingEnvelopeKeyV1: envelope})
	require.NoError(t, err)
	classified := classifySupplierAccountingLog(string(payload))
	require.Equal(t, types.SupplierAccountingDispositionProducerError, classified.disposition)
	require.Equal(t, supplierBatchLogFailureNone, classified.failure)
	require.Nil(t, classified.snapshot)
}

func TestSupplierPricingModeComesOnlyFromStrictProvenanceUnion(t *testing.T) {
	tests := map[string]struct {
		provenance *types.SupplierPricingProvenanceV1
		want       string
		wantErr    bool
	}{
		"ratio":   {provenance: &types.SupplierPricingProvenanceV1{Ratio: &types.SupplierRatioPricingProvenanceV1{}}, want: "ratio"},
		"fixed":   {provenance: &types.SupplierPricingProvenanceV1{Fixed: &types.SupplierFixedPricingProvenanceV1{}}, want: "fixed"},
		"tiered":  {provenance: &types.SupplierPricingProvenanceV1{Tiered: &types.SupplierTieredPricingProvenanceV1{}}, want: "tiered"},
		"missing": {wantErr: true},
		"ambiguous": {
			provenance: &types.SupplierPricingProvenanceV1{Ratio: &types.SupplierRatioPricingProvenanceV1{}, Fixed: &types.SupplierFixedPricingProvenanceV1{}},
			wantErr:    true,
		},
	}
	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := supplierPricingModeFromProvenance(testCase.provenance)
			if testCase.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, testCase.want, got)
		})
	}
}
