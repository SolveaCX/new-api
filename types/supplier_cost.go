package types

import (
	"math"
	"math/bits"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
)

type SupplierStatisticsScope string

type SupplierAccountingDisposition string

type SupplierPricingModeV1 string

const (
	SupplierStatisticsScopeBusiness SupplierStatisticsScope = "business"
	SupplierStatisticsScopeInternal SupplierStatisticsScope = "internal"

	SupplierAccountingEnvelopeKeyV1           = "supplier_accounting_v1"
	SupplierAccountingEnvelopeSchemaVersionV1 = 1
	SupplierAccountingProducerCapabilityV1    = 1

	SupplierAccountingDispositionUnsupportedPath         SupplierAccountingDisposition = "unsupported_path"
	SupplierAccountingDispositionNotFinanciallyCommitted SupplierAccountingDisposition = "not_financially_committed"
	SupplierAccountingDispositionZeroUsage               SupplierAccountingDisposition = "zero_usage"
	SupplierAccountingDispositionUnbound                 SupplierAccountingDisposition = "unbound"
	SupplierAccountingDispositionCaptured                SupplierAccountingDisposition = "captured"
	SupplierAccountingDispositionProducerError           SupplierAccountingDisposition = "producer_error"

	SupplierPricingModeRatio  SupplierPricingModeV1 = "ratio"
	SupplierPricingModeFixed  SupplierPricingModeV1 = "fixed"
	SupplierPricingModeTiered SupplierPricingModeV1 = "tiered"
)

// SupplierCostSnapshot is copied by value at channel selection time.
// It intentionally contains only scalar values so a cache refresh cannot mutate
// requests already in flight.
type SupplierCostSnapshot struct {
	BindingVersionId         int
	SupplierId               int
	SupplierName             string
	ContractId               int
	ContractName             string
	RateVersionId            int
	ProcurementMultiplierPpm int64
}

// SupplierOfficialPricingSnapshot freezes the request-time pricing inputs used
// only by the supplier official-list accounting sidecar. Existing customer
// quota settlement continues to use its current code path.
type SupplierOfficialPricingSnapshot struct {
	CaptureAttempted                      bool
	Loaded                                bool
	QuotaPerUnit                          string
	PriceData                             PriceData
	TieredBillingSnapshot                 *billingexpr.BillingSnapshot
	BillingRequestInput                   *billingexpr.RequestInput
	WebSearchPreviewPricePerThousandCalls float64
	ClaudeWebSearchPricePerThousandCalls  float64
	FileSearchPricePerThousandCalls       float64
	GeminiInputAudioPricePerMillionTokens float64
	ImageGenerationCallPrices             SupplierImageGenerationCallPrices
}

type SupplierImageGenerationCallPrices struct {
	Low1024x1024    float64
	Low1024x1536    float64
	Low1536x1024    float64
	Medium1024x1024 float64
	Medium1024x1536 float64
	Medium1536x1024 float64
	High1024x1024   float64
	High1024x1536   float64
	High1536x1024   float64
}

func (p SupplierImageGenerationCallPrices) Price(quality string, size string) float64 {
	switch quality + "|" + size {
	case "low|1024x1024":
		return p.Low1024x1024
	case "low|1024x1536":
		return p.Low1024x1536
	case "low|1536x1024":
		return p.Low1536x1024
	case "medium|1024x1024":
		return p.Medium1024x1024
	case "medium|1024x1536":
		return p.Medium1024x1536
	case "medium|1536x1024":
		return p.Medium1536x1024
	case "high|1024x1536":
		return p.High1024x1536
	case "high|1536x1024":
		return p.High1536x1024
	default:
		return p.High1024x1024
	}
}

func (s SupplierCostSnapshot) IsBound() bool {
	return s.SupplierId > 0 && s.ContractId > 0 && s.RateVersionId > 0
}

// SupplierStatisticsScopeSnapshot freezes the explicit user exclusion decision
// observed by the routing node. A zero value means business traffic with no
// matching exclusion rule.
type SupplierStatisticsScopeSnapshot struct {
	Scope           SupplierStatisticsScope
	ExclusionRuleId int
}

// SupplierAccountingLogSnapshotV1 is the immutable financial envelope stored
// under Log.Other["supplier_accounting_v1"]. Pointer-valued amounts preserve
// the distinction between unknown and a known zero.
type SupplierAccountingLogSnapshotV1 struct {
	BindingVersionId         int                          `json:"bv"`
	SupplierId               int                          `json:"s"`
	ContractId               int                          `json:"c"`
	RateVersionId            int                          `json:"rv"`
	ProcurementMultiplierPpm int64                        `json:"pm"`
	SalesMultiplierPpm       *int64                       `json:"sm,omitempty"`
	OfficialListMicroUsd     *int64                       `json:"ol,omitempty"`
	SalesMicroUsd            *int64                       `json:"sa,omitempty"`
	ProcurementCostMicroUsd  *int64                       `json:"pc,omitempty"`
	GrossProfitMicroUsd      *int64                       `json:"gp,omitempty"`
	StatisticsScope          string                       `json:"ss"`
	ExclusionDecision        string                       `json:"ed"`
	ExclusionRuleId          *int                         `json:"er,omitempty"`
	QuotaPerUnit             *string                      `json:"q,omitempty"`
	PricingMode              *string                      `json:"p,omitempty"`
	FinanciallyCommittedAt   int64                        `json:"fc"`
	QualityReason            string                       `json:"qr,omitempty"`
	PricingProvenance        *SupplierPricingProvenanceV1 `json:"pv,omitempty"`
	UnknownOfficialCount     uint32                       `json:"uo,omitempty"`
}

// SupplierAccountingEnvelopeV1 is the complete bounded producer marker stored
// under logs.other.supplier_accounting_v1. Captured is present only when the
// disposition is captured.
type SupplierAccountingEnvelopeV1 struct {
	EnvelopeSchemaVersion     int                              `json:"v"`
	ProducerCapabilityVersion int                              `json:"c"`
	ActivationStateVersion    int64                            `json:"a"`
	Disposition               SupplierAccountingDisposition    `json:"d"`
	Captured                  *SupplierAccountingLogSnapshotV1 `json:"s,omitempty"`
}

// SupplierPricingProvenanceV1 is a strict union: exactly one member is set.
// Dimensions are present only when the named dimension affected settlement.
type SupplierPricingProvenanceV1 struct {
	Ratio      *SupplierRatioPricingProvenanceV1  `json:"r,omitempty"`
	Fixed      *SupplierFixedPricingProvenanceV1  `json:"f,omitempty"`
	Tiered     *SupplierTieredPricingProvenanceV1 `json:"t,omitempty"`
	Dimensions *SupplierPricingDimensionsV1       `json:"x,omitempty"`
}

type SupplierRatioPricingProvenanceV1 struct {
	ModelRatioPpm     int64 `json:"m"`
	GroupRatioPpm     int64 `json:"g"`
	ModelRatioVersion int64 `json:"mv"`
	GroupRatioVersion int64 `json:"gv"`
}

type SupplierFixedPricingProvenanceV1 struct {
	Source             string `json:"s"`
	Key                string `json:"k"`
	PriceVersion       int64  `json:"v"`
	GroupMultiplierPpm int64  `json:"g"`
	GroupRatioVersion  int64  `json:"gv"`
}

type SupplierTieredPricingProvenanceV1 struct {
	ExpressionFingerprint     uint64                           `json:"e"`
	ExpressionFingerprintTail uint64                           `json:"et"`
	ExpressionVersion         int64                            `json:"v"`
	GroupMultiplierPpm        int64                            `json:"g"`
	GroupRatioVersion         int64                            `json:"gv"`
	NormalizedInputs          SupplierTieredNormalizedInputsV1 `json:"n"`
}

type SupplierTieredNormalizedInputsV1 struct {
	Prompt        int64 `json:"p"`
	Completion    int64 `json:"c"`
	ContextLength int64 `json:"l"`
	CacheRead     int64 `json:"r"`
	CacheCreate   int64 `json:"w"`
	CacheCreate1H int64 `json:"h"`
	ImageInput    int64 `json:"i"`
	ImageOutput   int64 `json:"o"`
	AudioInput    int64 `json:"a"`
	AudioOutput   int64 `json:"b"`
}

type SupplierPricingDimensionsV1 struct {
	Audio bool `json:"a,omitempty"`
	Tool  bool `json:"t,omitempty"`
	Image bool `json:"i,omitempty"`
}

// CalculateSupplierProcurementMicroV1 applies the canonical non-negative
// ROUND_HALF_UP procurement formula without overflowing the int64 product.
func CalculateSupplierProcurementMicroV1(officialMicroUSD int64, procurementMultiplierPpm int64) (int64, bool) {
	if officialMicroUSD < 0 || procurementMultiplierPpm < 0 {
		return 0, false
	}
	const scale uint64 = 1_000_000
	high, low := bits.Mul64(uint64(officialMicroUSD), uint64(procurementMultiplierPpm))
	if high >= scale {
		return 0, false
	}
	quotient, remainder := bits.Div64(high, low, scale)
	if quotient > math.MaxInt64 {
		return 0, false
	}
	if remainder >= scale/2 {
		if quotient == math.MaxInt64 {
			return 0, false
		}
		quotient++
	}
	return int64(quotient), true
}

func BusinessSupplierStatisticsScopeSnapshot() SupplierStatisticsScopeSnapshot {
	return SupplierStatisticsScopeSnapshot{Scope: SupplierStatisticsScopeBusiness}
}
