package types

import "github.com/QuantumNous/new-api/pkg/billingexpr"

type SupplierStatisticsScope string

const (
	SupplierStatisticsScopeBusiness SupplierStatisticsScope = "business"
	SupplierStatisticsScopeInternal SupplierStatisticsScope = "internal"
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
	BindingVersionId         int     `json:"bv"`
	SupplierId               int     `json:"s"`
	ContractId               int     `json:"c"`
	RateVersionId            int     `json:"rv"`
	ProcurementMultiplierPpm int64   `json:"pm"`
	SalesMultiplierPpm       *int64  `json:"sm,omitempty"`
	OfficialListMicroUsd     *int64  `json:"ol,omitempty"`
	SalesMicroUsd            *int64  `json:"sa,omitempty"`
	ProcurementCostMicroUsd  *int64  `json:"pc,omitempty"`
	GrossProfitMicroUsd      *int64  `json:"gp,omitempty"`
	StatisticsScope          string  `json:"ss"`
	ExclusionDecision        string  `json:"ed"`
	ExclusionRuleId          *int    `json:"er,omitempty"`
	QuotaPerUnit             *string `json:"q,omitempty"`
	PricingMode              *string `json:"p,omitempty"`
	FinanciallyCommittedAt   int64   `json:"fc"`
	QualityReason            string  `json:"qr,omitempty"`
}

func BusinessSupplierStatisticsScopeSnapshot() SupplierStatisticsScopeSnapshot {
	return SupplierStatisticsScopeSnapshot{Scope: SupplierStatisticsScopeBusiness}
}
