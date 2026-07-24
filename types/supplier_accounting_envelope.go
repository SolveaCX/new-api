package types

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

const (
	supplierAccountingCapturedLayoutVersionV1 = byte(1)
	supplierAccountingCapturedLayoutShift     = 6
	supplierAccountingCapturedLayoutMask      = byte(0xc0)
	supplierAccountingCapturedInternalFlag    = byte(1 << 0)
	supplierAccountingCapturedModeShift       = 1
	supplierAccountingCapturedModeMask        = byte(0x06)
	supplierAccountingCapturedAudioFlag       = byte(1 << 3)
	supplierAccountingCapturedToolFlag        = byte(1 << 4)
	supplierAccountingCapturedImageFlag       = byte(1 << 5)

	supplierAccountingCapturedModeRatioCode  = byte(0)
	supplierAccountingCapturedModeFixedCode  = byte(1)
	supplierAccountingCapturedModeTieredCode = byte(2)

	supplierAccountingFixedSourceV1        = "price_data"
	supplierAccountingFixedKeyV1           = "model_price"
	supplierAccountingInputVersionV1       = int64(1)
	supplierAccountingFingerprintTailMaxV1 = uint64(1<<48 - 1)
	// This is a persisted protocol value, not the process default. Never bind
	// schema-V1 history to billingexpr.DefaultExprVersion.
	supplierAccountingTieredExpressionVersionV1 = int64(1)
)

type supplierAccountingEnvelopeJSONV1 struct {
	EnvelopeSchemaVersion int                           `json:"v"`
	Disposition           SupplierAccountingDisposition `json:"d"`
	Captured              string                        `json:"s,omitempty"`
}

// MarshalJSON keeps the envelope metadata readable while encoding the
// captured numeric snapshot as a schema-versioned, fixed-width binary
// payload. Raw URL base64 is canonical and avoids padding variance.
func (envelope SupplierAccountingEnvelopeV1) MarshalJSON() ([]byte, error) {
	if envelope.EnvelopeSchemaVersion != SupplierAccountingEnvelopeSchemaVersionV1 {
		return nil, errors.New("invalid supplier accounting envelope version")
	}
	if !isSupplierAccountingDispositionV1(envelope.Disposition) {
		return nil, fmt.Errorf("invalid supplier accounting disposition %q", envelope.Disposition)
	}

	wire := supplierAccountingEnvelopeJSONV1{
		EnvelopeSchemaVersion: envelope.EnvelopeSchemaVersion,
		Disposition:           envelope.Disposition,
	}
	if envelope.Disposition == SupplierAccountingDispositionCaptured {
		captured, err := encodeSupplierAccountingCapturedV1(envelope.Captured)
		if err != nil {
			return nil, err
		}
		wire.Captured = captured
	} else if envelope.Captured != nil {
		return nil, errors.New("non-captured supplier accounting envelope contains a snapshot")
	}
	return common.Marshal(wire)
}

// UnmarshalJSON applies the envelope wire decoder before the service-layer
// field validator checks the captured accounting snapshot.
func (envelope *SupplierAccountingEnvelopeV1) UnmarshalJSON(data []byte) error {
	if envelope == nil {
		return errors.New("nil supplier accounting envelope destination")
	}
	parsed, err := ParseSupplierAccountingEnvelopeV1JSON(data)
	if err != nil {
		return err
	}
	*envelope = parsed
	return nil
}

// ParseSupplierAccountingEnvelopeV1JSON decodes one persisted envelope value.
// Callers must run their current field validator after decoding.
func ParseSupplierAccountingEnvelopeV1JSON(data []byte) (SupplierAccountingEnvelopeV1, error) {
	var wire supplierAccountingEnvelopeJSONV1
	if err := common.Unmarshal(data, &wire); err != nil {
		return SupplierAccountingEnvelopeV1{}, fmt.Errorf("invalid supplier accounting envelope object: %w", err)
	}
	envelope := SupplierAccountingEnvelopeV1{
		EnvelopeSchemaVersion: wire.EnvelopeSchemaVersion,
		Disposition:           wire.Disposition,
	}
	if envelope.EnvelopeSchemaVersion != SupplierAccountingEnvelopeSchemaVersionV1 ||
		!isSupplierAccountingDispositionV1(envelope.Disposition) {
		return SupplierAccountingEnvelopeV1{}, errors.New("unsupported supplier accounting envelope semantics")
	}

	if envelope.Disposition != SupplierAccountingDispositionCaptured {
		if wire.Captured != "" {
			return SupplierAccountingEnvelopeV1{}, errors.New("non-captured supplier accounting envelope contains field s")
		}
		return envelope, nil
	}
	if wire.Captured == "" {
		return SupplierAccountingEnvelopeV1{}, errors.New("captured supplier accounting envelope is missing string field s")
	}
	captured, err := decodeSupplierAccountingCapturedV1(wire.Captured)
	if err != nil {
		return SupplierAccountingEnvelopeV1{}, err
	}
	envelope.Captured = captured
	return envelope, nil
}

func isSupplierAccountingDispositionV1(disposition SupplierAccountingDisposition) bool {
	switch disposition {
	case SupplierAccountingDispositionUnsupportedPath,
		SupplierAccountingDispositionNotFinanciallyCommitted,
		SupplierAccountingDispositionZeroUsage,
		SupplierAccountingDispositionUnbound,
		SupplierAccountingDispositionCaptured,
		SupplierAccountingDispositionProducerError:
		return true
	default:
		return false
	}
}

func encodeSupplierAccountingCapturedV1(snapshot *SupplierAccountingLogSnapshotV1) (string, error) {
	plan, err := supplierAccountingCapturedEncodingPlanV1(snapshot)
	if err != nil {
		return "", err
	}
	data := make([]byte, supplierAccountingCapturedBinarySizeV1(plan.internal, plan.mode))
	data[0] = plan.flags
	offset := 1
	putInt64 := func(value int64) {
		binary.BigEndian.PutUint64(data[offset:offset+8], uint64(value))
		offset += 8
	}

	putInt64(int64(snapshot.BindingVersionId))
	putInt64(int64(snapshot.SupplierId))
	putInt64(int64(snapshot.ContractId))
	putInt64(int64(snapshot.RateVersionId))
	putInt64(snapshot.ProcurementMultiplierPpm)
	putInt64(*snapshot.OfficialListMicroUsd)
	putInt64(*snapshot.ProcurementCostMicroUsd)
	putInt64(snapshot.FinanciallyCommittedAt)
	if plan.internal {
		putInt64(int64(*snapshot.ExclusionRuleId))
	} else {
		putInt64(*snapshot.SalesMultiplierPpm)
		putInt64(*snapshot.SalesMicroUsd)
		putInt64(*snapshot.GrossProfitMicroUsd)
	}

	if !plan.internal {
		switch plan.mode {
		case SupplierPricingModeRatio:
			putInt64(snapshot.PricingProvenance.Ratio.ModelRatioPpm)
		case SupplierPricingModeTiered:
			binary.BigEndian.PutUint64(data[offset:offset+8], snapshot.PricingProvenance.Tiered.ExpressionFingerprint)
			offset += 8
			supplierAccountingPutUint48BigEndianV1(data[offset:offset+6], snapshot.PricingProvenance.Tiered.ExpressionFingerprintTail)
			offset += 6
			inputs := snapshot.PricingProvenance.Tiered.NormalizedInputs
			for _, value := range []int64{
				inputs.Prompt, inputs.Completion, inputs.ContextLength, inputs.CacheRead, inputs.CacheCreate,
				inputs.CacheCreate1H, inputs.ImageInput, inputs.ImageOutput, inputs.AudioInput, inputs.AudioOutput,
			} {
				putInt64(value)
			}
		}
	}
	if offset != len(data) {
		return "", errors.New("supplier accounting captured encoder length mismatch")
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

type supplierAccountingCapturedPlanV1 struct {
	flags           byte
	mode            SupplierPricingModeV1
	internal        bool
	groupMultiplier int64
}

func supplierAccountingCapturedEncodingPlanV1(snapshot *SupplierAccountingLogSnapshotV1) (supplierAccountingCapturedPlanV1, error) {
	if snapshot == nil {
		return supplierAccountingCapturedPlanV1{}, errors.New("missing supplier accounting captured snapshot")
	}
	if snapshot.BindingVersionId <= 0 || snapshot.SupplierId <= 0 || snapshot.ContractId <= 0 || snapshot.RateVersionId <= 0 ||
		snapshot.ProcurementMultiplierPpm < 0 || snapshot.FinanciallyCommittedAt <= 0 ||
		snapshot.OfficialListMicroUsd == nil || *snapshot.OfficialListMicroUsd < 0 ||
		snapshot.ProcurementCostMicroUsd == nil || *snapshot.ProcurementCostMicroUsd < 0 {
		return supplierAccountingCapturedPlanV1{}, errors.New("invalid supplier accounting captured common fields")
	}
	expectedProcurement, validProcurement := CalculateSupplierProcurementMicroV1(*snapshot.OfficialListMicroUsd, snapshot.ProcurementMultiplierPpm)
	if !validProcurement || expectedProcurement != *snapshot.ProcurementCostMicroUsd {
		return supplierAccountingCapturedPlanV1{}, errors.New("invalid supplier accounting procurement formula")
	}
	if snapshot.QuotaPerUnit != nil || snapshot.PricingMode != nil || snapshot.QualityReason != "" || snapshot.UnknownOfficialCount != 0 {
		return supplierAccountingCapturedPlanV1{}, errors.New("unsupported supplier accounting captured compatibility field")
	}
	plan := supplierAccountingCapturedPlanV1{flags: supplierAccountingCapturedLayoutVersionV1 << supplierAccountingCapturedLayoutShift}
	switch SupplierStatisticsScope(snapshot.StatisticsScope) {
	case SupplierStatisticsScopeBusiness:
		if snapshot.ExclusionDecision != "included" || snapshot.ExclusionRuleId != nil || snapshot.SalesMultiplierPpm == nil ||
			snapshot.SalesMicroUsd == nil || snapshot.GrossProfitMicroUsd == nil || *snapshot.SalesMultiplierPpm < 0 ||
			*snapshot.SalesMicroUsd < 0 || *snapshot.SalesMicroUsd-*snapshot.ProcurementCostMicroUsd != *snapshot.GrossProfitMicroUsd {
			return supplierAccountingCapturedPlanV1{}, errors.New("invalid supplier accounting business fields")
		}
	case SupplierStatisticsScopeInternal:
		if snapshot.ExclusionDecision != "excluded" || snapshot.ExclusionRuleId == nil || *snapshot.ExclusionRuleId <= 0 ||
			snapshot.SalesMultiplierPpm != nil || snapshot.SalesMicroUsd != nil || snapshot.GrossProfitMicroUsd != nil {
			return supplierAccountingCapturedPlanV1{}, errors.New("invalid supplier accounting internal fields")
		}
		plan.internal = true
		plan.flags |= supplierAccountingCapturedInternalFlag
		if snapshot.PricingProvenance != nil {
			return supplierAccountingCapturedPlanV1{}, errors.New("internal supplier accounting snapshot contains pricing provenance")
		}
		return plan, nil
	default:
		return supplierAccountingCapturedPlanV1{}, errors.New("invalid supplier accounting statistics scope")
	}
	if snapshot.PricingProvenance == nil {
		return supplierAccountingCapturedPlanV1{}, errors.New("missing supplier accounting pricing provenance")
	}

	provenance := snapshot.PricingProvenance
	memberCount := 0
	if provenance.Ratio != nil {
		memberCount++
		plan.mode = SupplierPricingModeRatio
	}
	if provenance.Fixed != nil {
		memberCount++
		plan.mode = SupplierPricingModeFixed
	}
	if provenance.Tiered != nil {
		memberCount++
		plan.mode = SupplierPricingModeTiered
	}
	if memberCount != 1 {
		return supplierAccountingCapturedPlanV1{}, errors.New("invalid supplier accounting pricing provenance union")
	}
	if dimensions := provenance.Dimensions; dimensions != nil {
		if !dimensions.Audio && !dimensions.Tool && !dimensions.Image {
			return supplierAccountingCapturedPlanV1{}, errors.New("empty supplier accounting pricing dimensions")
		}
		if dimensions.Audio {
			plan.flags |= supplierAccountingCapturedAudioFlag
		}
		if dimensions.Tool {
			plan.flags |= supplierAccountingCapturedToolFlag
		}
		if dimensions.Image {
			plan.flags |= supplierAccountingCapturedImageFlag
		}
	}

	switch plan.mode {
	case SupplierPricingModeRatio:
		ratio := provenance.Ratio
		if ratio.ModelRatioPpm < 0 || ratio.ModelRatioVersion != supplierAccountingInputVersionV1 ||
			!supplierAccountingGroupEvidenceValidV1(ratio.GroupRatioPpm, ratio.GroupRatioVersion) {
			return supplierAccountingCapturedPlanV1{}, errors.New("invalid supplier accounting ratio provenance")
		}
		plan.flags |= supplierAccountingCapturedModeRatioCode << supplierAccountingCapturedModeShift
		plan.groupMultiplier = ratio.GroupRatioPpm
	case SupplierPricingModeFixed:
		fixed := provenance.Fixed
		if fixed.Source != supplierAccountingFixedSourceV1 || fixed.Key != supplierAccountingFixedKeyV1 || fixed.PriceVersion != supplierAccountingInputVersionV1 ||
			!supplierAccountingGroupEvidenceValidV1(fixed.GroupMultiplierPpm, fixed.GroupRatioVersion) {
			return supplierAccountingCapturedPlanV1{}, errors.New("invalid supplier accounting fixed provenance")
		}
		plan.flags |= supplierAccountingCapturedModeFixedCode << supplierAccountingCapturedModeShift
		plan.groupMultiplier = fixed.GroupMultiplierPpm
	case SupplierPricingModeTiered:
		tiered := provenance.Tiered
		if (tiered.ExpressionFingerprint == 0 && tiered.ExpressionFingerprintTail == 0) ||
			tiered.ExpressionFingerprintTail > supplierAccountingFingerprintTailMaxV1 ||
			tiered.ExpressionVersion != supplierAccountingTieredExpressionVersionV1 ||
			!supplierAccountingGroupEvidenceValidV1(tiered.GroupMultiplierPpm, tiered.GroupRatioVersion) ||
			!supplierAccountingTieredInputsNonNegativeV1(tiered.NormalizedInputs) {
			return supplierAccountingCapturedPlanV1{}, errors.New("invalid supplier accounting tiered provenance")
		}
		plan.flags |= supplierAccountingCapturedModeTieredCode << supplierAccountingCapturedModeShift
		plan.groupMultiplier = tiered.GroupMultiplierPpm
	default:
		return supplierAccountingCapturedPlanV1{}, errors.New("invalid supplier accounting pricing mode")
	}
	if *snapshot.SalesMultiplierPpm != plan.groupMultiplier {
		return supplierAccountingCapturedPlanV1{}, errors.New("supplier accounting business group multiplier mismatch")
	}
	return plan, nil
}

func supplierAccountingGroupEvidenceValidV1(multiplier, version int64) bool {
	return multiplier >= 0 && version == supplierAccountingInputVersionV1
}

func decodeSupplierAccountingCapturedV1(encoded string) (*SupplierAccountingLogSnapshotV1, error) {
	if !supplierAccountingCapturedEncodedLengthV1(len(encoded)) {
		return nil, fmt.Errorf("invalid supplier accounting captured base64 length %d", len(encoded))
	}
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(data) == 0 || base64.RawURLEncoding.EncodeToString(data) != encoded {
		return nil, errors.New("invalid or non-canonical supplier accounting captured base64")
	}
	flags := data[0]
	if (flags&supplierAccountingCapturedLayoutMask)>>supplierAccountingCapturedLayoutShift != supplierAccountingCapturedLayoutVersionV1 {
		return nil, errors.New("unsupported supplier accounting captured layout version")
	}
	internal := flags&supplierAccountingCapturedInternalFlag != 0
	if internal {
		if flags != supplierAccountingCapturedLayoutVersionV1<<supplierAccountingCapturedLayoutShift|supplierAccountingCapturedInternalFlag {
			return nil, errors.New("internal supplier accounting captured payload contains pricing flags")
		}
		if len(data) != supplierAccountingCapturedBinarySizeV1(true, "") {
			return nil, fmt.Errorf("invalid supplier accounting captured payload length %d", len(data))
		}
	}
	modeCode := (flags & supplierAccountingCapturedModeMask) >> supplierAccountingCapturedModeShift
	var mode SupplierPricingModeV1
	if !internal {
		switch modeCode {
		case supplierAccountingCapturedModeRatioCode:
			mode = SupplierPricingModeRatio
		case supplierAccountingCapturedModeFixedCode:
			mode = SupplierPricingModeFixed
		case supplierAccountingCapturedModeTieredCode:
			mode = SupplierPricingModeTiered
		default:
			return nil, errors.New("invalid supplier accounting captured pricing mode")
		}
		if len(data) != supplierAccountingCapturedBinarySizeV1(false, mode) {
			return nil, fmt.Errorf("invalid supplier accounting captured payload length %d", len(data))
		}
	}

	offset := 1
	readInt64 := func() int64 {
		value := int64(binary.BigEndian.Uint64(data[offset : offset+8]))
		offset += 8
		return value
	}
	readInt := func(field string) (int, error) {
		value := readInt64()
		converted := int(value)
		if int64(converted) != value {
			return 0, fmt.Errorf("supplier accounting %s overflows int", field)
		}
		return converted, nil
	}

	snapshot := &SupplierAccountingLogSnapshotV1{}
	if snapshot.BindingVersionId, err = readInt("binding version"); err != nil {
		return nil, err
	}
	if snapshot.SupplierId, err = readInt("supplier id"); err != nil {
		return nil, err
	}
	if snapshot.ContractId, err = readInt("contract id"); err != nil {
		return nil, err
	}
	if snapshot.RateVersionId, err = readInt("rate version"); err != nil {
		return nil, err
	}
	snapshot.ProcurementMultiplierPpm = readInt64()
	official := readInt64()
	procurement := readInt64()
	snapshot.OfficialListMicroUsd = &official
	snapshot.ProcurementCostMicroUsd = &procurement
	snapshot.FinanciallyCommittedAt = readInt64()

	var groupMultiplier int64
	if internal {
		snapshot.StatisticsScope = string(SupplierStatisticsScopeInternal)
		snapshot.ExclusionDecision = "excluded"
		exclusionRuleID, readErr := readInt("exclusion rule id")
		if readErr != nil {
			return nil, readErr
		}
		snapshot.ExclusionRuleId = &exclusionRuleID
	} else {
		snapshot.StatisticsScope = string(SupplierStatisticsScopeBusiness)
		snapshot.ExclusionDecision = "included"
		salesMultiplier := readInt64()
		sales := readInt64()
		grossProfit := readInt64()
		snapshot.SalesMultiplierPpm = &salesMultiplier
		snapshot.SalesMicroUsd = &sales
		snapshot.GrossProfitMicroUsd = &grossProfit
		groupMultiplier = salesMultiplier
	}

	if internal {
		if offset != len(data) {
			return nil, errors.New("supplier accounting captured decoder length mismatch")
		}
		return snapshot, nil
	}

	provenance := &SupplierPricingProvenanceV1{Dimensions: supplierAccountingDimensionsFromFlagsV1(flags)}
	switch mode {
	case SupplierPricingModeRatio:
		provenance.Ratio = &SupplierRatioPricingProvenanceV1{
			ModelRatioPpm: readInt64(), GroupRatioPpm: groupMultiplier,
			ModelRatioVersion: supplierAccountingInputVersionV1, GroupRatioVersion: supplierAccountingInputVersionV1,
		}
	case SupplierPricingModeFixed:
		provenance.Fixed = &SupplierFixedPricingProvenanceV1{
			Source: supplierAccountingFixedSourceV1, Key: supplierAccountingFixedKeyV1,
			PriceVersion: supplierAccountingInputVersionV1, GroupMultiplierPpm: groupMultiplier, GroupRatioVersion: supplierAccountingInputVersionV1,
		}
	case SupplierPricingModeTiered:
		fingerprint := binary.BigEndian.Uint64(data[offset : offset+8])
		offset += 8
		fingerprintTail := supplierAccountingUint48BigEndianV1(data[offset : offset+6])
		offset += 6
		values := make([]int64, 10)
		for index := range values {
			values[index] = readInt64()
		}
		provenance.Tiered = &SupplierTieredPricingProvenanceV1{
			ExpressionFingerprint:     fingerprint,
			ExpressionFingerprintTail: fingerprintTail,
			ExpressionVersion:         supplierAccountingTieredExpressionVersionV1,
			GroupMultiplierPpm:        groupMultiplier,
			GroupRatioVersion:         supplierAccountingInputVersionV1,
			NormalizedInputs: SupplierTieredNormalizedInputsV1{
				Prompt: values[0], Completion: values[1], ContextLength: values[2], CacheRead: values[3], CacheCreate: values[4],
				CacheCreate1H: values[5], ImageInput: values[6], ImageOutput: values[7], AudioInput: values[8], AudioOutput: values[9],
			},
		}
	}
	if offset != len(data) {
		return nil, errors.New("supplier accounting captured decoder length mismatch")
	}
	snapshot.PricingProvenance = provenance
	return snapshot, nil
}

func supplierAccountingCapturedEncodedLengthV1(length int) bool {
	for _, candidate := range []int{
		base64.RawURLEncoding.EncodedLen(supplierAccountingCapturedBinarySizeV1(false, SupplierPricingModeRatio)),
		base64.RawURLEncoding.EncodedLen(supplierAccountingCapturedBinarySizeV1(false, SupplierPricingModeFixed)),
		base64.RawURLEncoding.EncodedLen(supplierAccountingCapturedBinarySizeV1(false, SupplierPricingModeTiered)),
		base64.RawURLEncoding.EncodedLen(supplierAccountingCapturedBinarySizeV1(true, "")),
	} {
		if length == candidate {
			return true
		}
	}
	return false
}

func supplierAccountingCapturedBinarySizeV1(internal bool, mode SupplierPricingModeV1) int {
	size := 1 + 8*8
	if internal {
		return size + 8
	} else {
		size += 8 * 3
	}
	switch mode {
	case SupplierPricingModeRatio:
		size += 8
	case SupplierPricingModeTiered:
		size += 8 + 6 + 8*10
	}
	return size
}

func supplierAccountingPutUint48BigEndianV1(destination []byte, value uint64) {
	destination[0] = byte(value >> 40)
	destination[1] = byte(value >> 32)
	destination[2] = byte(value >> 24)
	destination[3] = byte(value >> 16)
	destination[4] = byte(value >> 8)
	destination[5] = byte(value)
}

func supplierAccountingUint48BigEndianV1(source []byte) uint64 {
	return uint64(source[0])<<40 |
		uint64(source[1])<<32 |
		uint64(source[2])<<24 |
		uint64(source[3])<<16 |
		uint64(source[4])<<8 |
		uint64(source[5])
}

func supplierAccountingDimensionsFromFlagsV1(flags byte) *SupplierPricingDimensionsV1 {
	dimensions := &SupplierPricingDimensionsV1{
		Audio: flags&supplierAccountingCapturedAudioFlag != 0,
		Tool:  flags&supplierAccountingCapturedToolFlag != 0,
		Image: flags&supplierAccountingCapturedImageFlag != 0,
	}
	if !dimensions.Audio && !dimensions.Tool && !dimensions.Image {
		return nil
	}
	return dimensions
}

func supplierAccountingTieredInputsNonNegativeV1(inputs SupplierTieredNormalizedInputsV1) bool {
	return inputs.Prompt >= 0 && inputs.Completion >= 0 && inputs.ContextLength >= 0 && inputs.CacheRead >= 0 &&
		inputs.CacheCreate >= 0 && inputs.CacheCreate1H >= 0 && inputs.ImageInput >= 0 && inputs.ImageOutput >= 0 &&
		inputs.AudioInput >= 0 && inputs.AudioOutput >= 0
}
