package types

import (
	"encoding/base64"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestSupplierAccountingEnvelopeV1CodecRoundTripsEveryScopeAndMode(t *testing.T) {
	for _, internal := range []bool{false, true} {
		for _, mode := range []SupplierPricingModeV1{SupplierPricingModeRatio, SupplierPricingModeFixed, SupplierPricingModeTiered} {
			name := string(mode) + "/business"
			if internal {
				name = string(mode) + "/internal"
			}
			t.Run(name, func(t *testing.T) {
				envelope := supplierAccountingCodecEnvelopeV1(internal, mode)
				payload, err := common.Marshal(envelope)
				require.NoError(t, err)
				encoded := supplierAccountingCodecCapturedTextV1(t, payload)
				decodedBinary, err := base64.RawURLEncoding.DecodeString(encoded)
				require.NoError(t, err)
				require.Len(t, decodedBinary, supplierAccountingCapturedBinarySizeV1(internal, mode))

				var decoded SupplierAccountingEnvelopeV1
				require.NoError(t, common.Unmarshal(payload, &decoded))
				require.Equal(t, envelope, decoded)
				if internal {
					require.Len(t, decodedBinary, 1+8*9, "internal layout must contain only identities, procurement amounts, commit time, and exclusion rule")
					require.Equal(t, supplierAccountingCapturedLayoutVersionV1<<supplierAccountingCapturedLayoutShift|supplierAccountingCapturedInternalFlag, decodedBinary[0])
					require.Nil(t, decoded.Captured.PricingProvenance)
				}

				reencoded, err := common.Marshal(decoded)
				require.NoError(t, err)
				require.Equal(t, payload, reencoded, "the persisted protocol must be byte-canonical")
			})
		}
	}
}

func TestSupplierAccountingEnvelopeV1CodecStrictOuterObject(t *testing.T) {
	valid := `{"v":1,"d":"unbound"}`
	tests := map[string]string{
		"missing schema":           `{"d":"unbound"}`,
		"missing disposition":      `{"v":1}`,
		"wrong schema":             `{"v":2,"d":"unbound"}`,
		"unknown disposition":      `{"v":1,"d":"request_controlled"}`,
		"captured without payload": `{"v":1,"d":"captured"}`,
		"payload on exclusion":     `{"v":1,"d":"unbound","s":"AA"}`,
		"trailing document":        valid + `{}`,
		"array":                    `[]`,
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			var decoded SupplierAccountingEnvelopeV1
			require.Error(t, common.Unmarshal([]byte(payload), &decoded))
		})
	}
}

func TestSupplierAccountingEnvelopeV1CodecRejectsMalformedCapturedPayload(t *testing.T) {
	envelope := supplierAccountingCodecEnvelopeV1(true, SupplierPricingModeTiered)
	payload, err := common.Marshal(envelope)
	require.NoError(t, err)
	encoded := supplierAccountingCodecCapturedTextV1(t, payload)

	decodedBytes, err := base64.RawURLEncoding.DecodeString(encoded)
	require.NoError(t, err)
	wrongLayout := append([]byte(nil), decodedBytes...)
	wrongLayout[0] = (wrongLayout[0] &^ supplierAccountingCapturedLayoutMask) | byte(2<<supplierAccountingCapturedLayoutShift)
	wrongMode := append([]byte(nil), decodedBytes...)
	wrongMode[0] = (wrongMode[0] &^ supplierAccountingCapturedModeMask) | byte(3<<supplierAccountingCapturedModeShift)

	tests := map[string]string{
		"invalid alphabet": strings.Repeat("!", len(encoded)),
		"truncated":        encoded[:len(encoded)-1],
		"padded":           encoded + "=",
		"oversized":        strings.Repeat("A", 1<<20),
		"wrong layout":     base64.RawURLEncoding.EncodeToString(wrongLayout),
		"wrong mode":       base64.RawURLEncoding.EncodeToString(wrongMode),
	}
	for name, captured := range tests {
		t.Run(name, func(t *testing.T) {
			raw := `{"v":1,"d":"captured","s":"` + captured + `"}`
			var decoded SupplierAccountingEnvelopeV1
			require.Error(t, common.Unmarshal([]byte(raw), &decoded))
		})
	}
}

func TestSupplierAccountingEnvelopeV1CodecTieredFingerprintUses112BitBigEndianLayout(t *testing.T) {
	envelope := supplierAccountingCodecEnvelopeV1(false, SupplierPricingModeTiered)
	tiered := envelope.Captured.PricingProvenance.Tiered
	tiered.ExpressionFingerprint = 0xba7816bf8f01cfea
	tiered.ExpressionFingerprintTail = 0x414140de5dae

	payload, err := common.Marshal(envelope)
	require.NoError(t, err)
	encoded := supplierAccountingCodecCapturedTextV1(t, payload)
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	require.NoError(t, err)
	fingerprintOffset := 1 + 8*8 + 8*3
	require.Equal(t, uint64(0xba7816bf8f01cfea), binary.BigEndian.Uint64(data[fingerprintOffset:fingerprintOffset+8]))
	require.Equal(t, []byte{0x41, 0x41, 0x40, 0xde, 0x5d, 0xae}, data[fingerprintOffset+8:fingerprintOffset+14])

	var decoded SupplierAccountingEnvelopeV1
	require.NoError(t, common.Unmarshal(payload, &decoded))
	require.Equal(t, tiered.ExpressionFingerprint, decoded.Captured.PricingProvenance.Tiered.ExpressionFingerprint)
	require.Equal(t, tiered.ExpressionFingerprintTail, decoded.Captured.PricingProvenance.Tiered.ExpressionFingerprintTail)
}

func TestSupplierAccountingEnvelopeV1CodecRejectsTieredFingerprintTailOverflow(t *testing.T) {
	envelope := supplierAccountingCodecEnvelopeV1(false, SupplierPricingModeTiered)
	envelope.Captured.PricingProvenance.Tiered.ExpressionFingerprintTail = 1 << 48
	_, err := common.Marshal(envelope)
	require.Error(t, err)
}

func TestSupplierAccountingEnvelopeV1CodecAllowsZeroHeadWhen112BitFingerprintIsNonZero(t *testing.T) {
	envelope := supplierAccountingCodecEnvelopeV1(false, SupplierPricingModeTiered)
	envelope.Captured.PricingProvenance.Tiered.ExpressionFingerprint = 0
	envelope.Captured.PricingProvenance.Tiered.ExpressionFingerprintTail = 1
	payload, err := common.Marshal(envelope)
	require.NoError(t, err)

	var decoded SupplierAccountingEnvelopeV1
	require.NoError(t, common.Unmarshal(payload, &decoded))
	require.Equal(t, envelope, decoded)
}

func TestSupplierAccountingEnvelopeV1CodecRejectsUnrepresentableSemanticVersions(t *testing.T) {
	tests := map[string]func(*SupplierAccountingLogSnapshotV1){
		"ratio model version": func(snapshot *SupplierAccountingLogSnapshotV1) {
			snapshot.PricingProvenance.Ratio.ModelRatioVersion = 2
		},
		"fixed source": func(snapshot *SupplierAccountingLogSnapshotV1) {
			snapshot.PricingProvenance.Fixed.Source = "future_source"
		},
		"tiered expression version": func(snapshot *SupplierAccountingLogSnapshotV1) {
			snapshot.PricingProvenance.Tiered.ExpressionVersion = 2
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			mode := SupplierPricingModeRatio
			switch name {
			case "fixed source":
				mode = SupplierPricingModeFixed
			case "tiered expression version":
				mode = SupplierPricingModeTiered
			}
			envelope := supplierAccountingCodecEnvelopeV1(false, mode)
			mutate(envelope.Captured)
			_, err := common.Marshal(envelope)
			require.Error(t, err, "schema V1 must reject unrepresentable semantic versions")
		})
	}
}

func TestSupplierAccountingCapturedV1OversizeGuardPrecedesBase64Decode(t *testing.T) {
	oversized := strings.Repeat("A", 1<<20)
	allocations := testing.AllocsPerRun(100, func() {
		_, _ = decodeSupplierAccountingCapturedV1(oversized)
	})
	require.LessOrEqual(t, allocations, float64(3), "oversize rejection must not allocate proportional decoded storage")
}

func supplierAccountingCodecEnvelopeV1(internal bool, mode SupplierPricingModeV1) SupplierAccountingEnvelopeV1 {
	maxInt := int(^uint(0) >> 1)
	official := int64(math.MaxInt64)
	procurement := int64(math.MaxInt64)
	snapshot := &SupplierAccountingLogSnapshotV1{
		BindingVersionId:         maxInt,
		SupplierId:               maxInt,
		ContractId:               maxInt,
		RateVersionId:            maxInt,
		ProcurementMultiplierPpm: 1_000_000,
		OfficialListMicroUsd:     &official,
		ProcurementCostMicroUsd:  &procurement,
		FinanciallyCommittedAt:   math.MaxInt64,
	}
	groupMultiplier := int64(math.MaxInt64)
	groupRatioVersion := int64(1)
	if internal {
		exclusionRuleID := maxInt
		snapshot.StatisticsScope = string(SupplierStatisticsScopeInternal)
		snapshot.ExclusionDecision = "excluded"
		snapshot.ExclusionRuleId = &exclusionRuleID
		return SupplierAccountingEnvelopeV1{
			EnvelopeSchemaVersion: SupplierAccountingEnvelopeSchemaVersionV1,
			Disposition:           SupplierAccountingDispositionCaptured,
			Captured:              snapshot,
		}
	} else {
		sales := int64(0)
		grossProfit := -int64(math.MaxInt64)
		snapshot.StatisticsScope = string(SupplierStatisticsScopeBusiness)
		snapshot.ExclusionDecision = "included"
		snapshot.SalesMultiplierPpm = &groupMultiplier
		snapshot.SalesMicroUsd = &sales
		snapshot.GrossProfitMicroUsd = &grossProfit
	}
	snapshot.PricingProvenance = &SupplierPricingProvenanceV1{}

	switch mode {
	case SupplierPricingModeRatio:
		snapshot.PricingProvenance.Ratio = &SupplierRatioPricingProvenanceV1{
			ModelRatioPpm: math.MaxInt64, GroupRatioPpm: groupMultiplier,
			ModelRatioVersion: 1, GroupRatioVersion: groupRatioVersion,
		}
	case SupplierPricingModeFixed:
		snapshot.PricingProvenance.Fixed = &SupplierFixedPricingProvenanceV1{
			Source: supplierAccountingFixedSourceV1, Key: supplierAccountingFixedKeyV1,
			PriceVersion: 1, GroupMultiplierPpm: groupMultiplier, GroupRatioVersion: groupRatioVersion,
		}
	case SupplierPricingModeTiered:
		snapshot.PricingProvenance.Tiered = &SupplierTieredPricingProvenanceV1{
			ExpressionFingerprint:     math.MaxUint64,
			ExpressionFingerprintTail: supplierAccountingFingerprintTailMaxV1,
			ExpressionVersion:         supplierAccountingTieredExpressionVersionV1,
			GroupMultiplierPpm:        groupMultiplier,
			GroupRatioVersion:         groupRatioVersion,
			NormalizedInputs: SupplierTieredNormalizedInputsV1{
				Prompt: math.MaxInt64, Completion: math.MaxInt64, ContextLength: math.MaxInt64,
				CacheRead: math.MaxInt64, CacheCreate: math.MaxInt64, CacheCreate1H: math.MaxInt64,
				ImageInput: math.MaxInt64, ImageOutput: math.MaxInt64, AudioInput: math.MaxInt64, AudioOutput: math.MaxInt64,
			},
		}
	}
	snapshot.PricingProvenance.Dimensions = &SupplierPricingDimensionsV1{Audio: true, Tool: true, Image: true}
	return SupplierAccountingEnvelopeV1{
		EnvelopeSchemaVersion: SupplierAccountingEnvelopeSchemaVersionV1,
		Disposition:           SupplierAccountingDispositionCaptured,
		Captured:              snapshot,
	}
}

func TestCalculateSupplierProcurementMicroV1UsesOverflowSafeHalfUp(t *testing.T) {
	tests := []struct {
		name       string
		official   int64
		multiplier int64
		expected   int64
		ok         bool
	}{
		{name: "below half", official: 1, multiplier: 499_999, expected: 0, ok: true},
		{name: "exact half rounds up", official: 1, multiplier: 500_000, expected: 1, ok: true},
		{name: "above one multiplier", official: 2_000_000, multiplier: 1_250_000, expected: 2_500_000, ok: true},
		{name: "max exact", official: math.MaxInt64, multiplier: 1_000_000, expected: math.MaxInt64, ok: true},
		{name: "rounded overflow", official: math.MaxInt64, multiplier: 1_000_001, ok: false},
		{name: "product overflow", official: math.MaxInt64, multiplier: math.MaxInt64, ok: false},
		{name: "negative official", official: -1, multiplier: 1_000_000, ok: false},
		{name: "negative multiplier", official: 1, multiplier: -1, ok: false},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actual, ok := CalculateSupplierProcurementMicroV1(testCase.official, testCase.multiplier)
			require.Equal(t, testCase.ok, ok)
			if ok {
				require.Equal(t, testCase.expected, actual)
			}
		})
	}
}

func supplierAccountingCodecCapturedTextV1(t *testing.T, payload []byte) string {
	t.Helper()
	var fields map[string]any
	require.NoError(t, common.Unmarshal(payload, &fields))
	captured, ok := fields["s"].(string)
	require.True(t, ok)
	return captured
}
