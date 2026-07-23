package service

import (
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

var (
	supplierAccountingContractEnvelopeSink types.SupplierAccountingEnvelopeV1
	supplierAccountingContractBytesSink    []byte
)

func TestSupplierAccountingEnvelopeGoldenPayloadBudgets(t *testing.T) {
	setSupplierAccountingContractActivationCache(t)

	business := maximizeSupplierAccountingContractEnvelope(BuildSupplierAccountingEnvelopeV1(supplierAccountingContractCapturedInput(false)))
	require.Equal(t, types.SupplierAccountingDispositionCaptured, business.Disposition)
	require.NoError(t, ValidateSupplierAccountingEnvelopeV1(business))
	businessJSON := marshalSupplierAccountingContractOuter(t, business)
	require.Equal(t, 330, len(businessJSON))
	require.LessOrEqual(t, len(businessJSON), 384, string(businessJSON))
	requireSupplierAccountingContractPayloadHasNoDeploymentIdentity(t, businessJSON)

	internal := maximizeSupplierAccountingContractEnvelope(BuildSupplierAccountingEnvelopeV1(supplierAccountingContractCapturedInput(true)))
	require.Equal(t, types.SupplierAccountingDispositionCaptured, internal.Disposition)
	require.NoError(t, ValidateSupplierAccountingEnvelopeV1(internal))
	internalJSON := marshalSupplierAccountingContractOuter(t, internal)
	require.Equal(t, 320, len(internalJSON))
	require.LessOrEqual(t, len(internalJSON), 320, string(internalJSON))
	requireSupplierAccountingContractPayloadHasNoDeploymentIdentity(t, internalJSON)
	require.NotNil(t, internal.Captured)
	require.Nil(t, internal.Captured.SalesMultiplierPpm)
	require.Nil(t, internal.Captured.SalesMicroUsd)
	require.Nil(t, internal.Captured.GrossProfitMicroUsd)
	require.Empty(t, internal.Captured.QualityReason)

	dispositionInputs := []struct {
		name         string
		envelope     types.SupplierAccountingEnvelopeV1
		expectedJSON string
	}{
		{name: "unsupported", envelope: InjectUnsupportedSupplierAccountingEnvelopeV1(nil), expectedJSON: `{"supplier_accounting_v1":{"v":1,"c":1,"a":0,"d":"unsupported_path"}}`},
		{name: "not_financially_committed", envelope: BuildSupplierAccountingEnvelopeV1(SupplierAccountingEnvelopeInputV1{}), expectedJSON: `{"supplier_accounting_v1":{"v":1,"c":1,"a":0,"d":"not_financially_committed"}}`},
		{name: "zero_usage", envelope: BuildSupplierAccountingEnvelopeV1(SupplierAccountingEnvelopeInputV1{Settlement: types.BillingSettlementResult{FinanciallyCommitted: true, FinanciallyCommittedAt: 1}}), expectedJSON: `{"supplier_accounting_v1":{"v":1,"c":1,"a":0,"d":"zero_usage"}}`},
		{name: "unbound", envelope: BuildSupplierAccountingEnvelopeV1(SupplierAccountingEnvelopeInputV1{RelayInfo: &relaycommon.RelayInfo{}, Settlement: types.BillingSettlementResult{FinanciallyCommitted: true, FinanciallyCommittedAt: 1}, HasPositiveFinalUsage: true}), expectedJSON: `{"supplier_accounting_v1":{"v":1,"c":1,"a":0,"d":"unbound"}}`},
		{name: "producer_error", envelope: newSupplierAccountingDispositionEnvelope(types.SupplierAccountingDispositionProducerError), expectedJSON: `{"supplier_accounting_v1":{"v":1,"c":1,"a":0,"d":"producer_error"}}`},
	}
	for _, testCase := range dispositionInputs {
		t.Run(testCase.name, func(t *testing.T) {
			require.NotEqual(t, types.SupplierAccountingDispositionCaptured, testCase.envelope.Disposition)
			require.Nil(t, testCase.envelope.Captured)
			require.NoError(t, ValidateSupplierAccountingEnvelopeV1(testCase.envelope))
			payload := marshalSupplierAccountingContractOuter(t, testCase.envelope)
			require.Equal(t, testCase.expectedJSON, string(payload))
			require.LessOrEqual(t, len(payload), 160, string(payload))
			requireSupplierAccountingContractPayloadHasNoDeploymentIdentity(t, payload)
		})
	}
}

func TestSupplierAccountingEnvelopeOuterAndInternalFieldContract(t *testing.T) {
	setSupplierAccountingContractActivationCache(t)
	internal := BuildSupplierAccountingEnvelopeV1(supplierAccountingContractCapturedInput(true))
	payload := marshalSupplierAccountingContractOuter(t, internal)

	var outer map[string]any
	require.NoError(t, common.Unmarshal(payload, &outer))
	require.Equal(t, []string{types.SupplierAccountingEnvelopeKeyV1}, sortedSupplierAccountingContractKeys(outer))
	envelope, ok := outer[types.SupplierAccountingEnvelopeKeyV1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, []string{"a", "c", "d", "s", "v"}, sortedSupplierAccountingContractKeys(envelope))
	_, ok = envelope["s"].(string)
	require.True(t, ok)
	var decodedOuter map[string]types.SupplierAccountingEnvelopeV1
	require.NoError(t, common.Unmarshal(payload, &decodedOuter))
	decoded := decodedOuter[types.SupplierAccountingEnvelopeKeyV1]
	require.NotNil(t, decoded.Captured)
	require.Nil(t, decoded.Captured.SalesMultiplierPpm)
	require.Nil(t, decoded.Captured.SalesMicroUsd)
	require.Nil(t, decoded.Captured.GrossProfitMicroUsd)
	require.Nil(t, decoded.Captured.QuotaPerUnit)
	require.Nil(t, decoded.Captured.PricingMode)
	require.Empty(t, decoded.Captured.QualityReason)
}

func TestSupplierAccountingEnvelopeBuildUsesOnlyCachedActivationState(t *testing.T) {
	setSupplierAccountingContractActivationCache(t)
	originalDB := model.DB
	originalRedisEnabled, originalRDB := common.RedisEnabled, common.RDB
	model.DB = nil
	common.RedisEnabled, common.RDB = true, nil
	t.Cleanup(func() {
		model.DB = originalDB
		common.RedisEnabled, common.RDB = originalRedisEnabled, originalRDB
	})

	require.NotPanics(t, func() {
		require.Zero(t, CurrentSupplierAccountingActivationStateVersion())
		envelope := BuildSupplierAccountingEnvelopeV1(supplierAccountingContractCapturedInput(false))
		require.Equal(t, types.SupplierAccountingDispositionCaptured, envelope.Disposition)
	})
}

func TestSupplierAccountingEnvelopeHotPathBudgets(t *testing.T) {
	setSupplierAccountingContractActivationCache(t)
	result := testing.Benchmark(benchmarkSupplierAccountingEnvelopeHotPath)
	t.Logf("supplier envelope hot path: %d ns/op, %d B/op, %d allocs/op", result.NsPerOp(), result.AllocedBytesPerOp(), result.AllocsPerOp())
	require.LessOrEqual(t, result.NsPerOp(), int64(100_000))
	require.LessOrEqual(t, result.AllocedBytesPerOp(), int64(8*1024))
}

func BenchmarkSupplierAccountingEnvelopeHotPath(b *testing.B) {
	setSupplierAccountingContractActivationCache(b)
	benchmarkSupplierAccountingEnvelopeHotPath(b)
}

func benchmarkSupplierAccountingEnvelopeHotPath(b *testing.B) {
	input := supplierAccountingContractCapturedInput(false)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		envelope := BuildSupplierAccountingEnvelopeV1(input)
		payload, err := common.Marshal(map[string]any{types.SupplierAccountingEnvelopeKeyV1: envelope})
		if err != nil {
			b.Fatal(err)
		}
		supplierAccountingContractEnvelopeSink = envelope
		supplierAccountingContractBytesSink = payload
	}
}

func supplierAccountingContractCapturedInput(internal bool) SupplierAccountingEnvelopeInputV1 {
	official := decimal.RequireFromString("0.000001")
	expression := `tier("base", p * 0.000001 + c * 0.000001)`
	info := &relaycommon.RelayInfo{
		PriceData: types.PriceData{
			ModelRatio:     0.000001,
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.000001},
		},
		SupplierCostSnapshot: types.SupplierCostSnapshot{
			BindingVersionId: 1, SupplierId: 1, ContractId: 1, RateVersionId: 1, ProcurementMultiplierPpm: 1,
		},
		SupplierStatisticsScopeSnapshot: types.BusinessSupplierStatisticsScopeSnapshot(),
		SupplierOfficialPricingSnapshot: types.SupplierOfficialPricingSnapshot{
			Loaded: true, QuotaPerUnit: "1000000",
			TieredBillingSnapshot: &billingexpr.BillingSnapshot{
				ExprString: expression, ExprHash: billingexpr.ExprHashString(expression), ExprVersion: 1,
			},
		},
	}
	if internal {
		info.SupplierStatisticsScopeSnapshot = types.SupplierStatisticsScopeSnapshot{Scope: types.SupplierStatisticsScopeInternal, ExclusionRuleId: 1}
	}
	tieredInputs := &billingexpr.TokenParams{P: 1, C: 1, Len: 1, CR: 1, CC: 1, CC1h: 1, Img: 1, ImgO: 1, AI: 1, AO: 1}
	return SupplierAccountingEnvelopeInputV1{
		RelayInfo: info,
		Settlement: types.BillingSettlementResult{
			FinanciallyCommitted: true, FinanciallyCommittedAt: 1, FinalSalesQuota: 1,
		},
		HasPositiveFinalUsage: true,
		Capture: SupplierAccountingCaptureInputV1{
			OfficialListUSD: &official, PricingMode: "tiered", TieredTokenParams: tieredInputs,
			AudioPricingApplied: true, ToolPricingApplied: true, ImagePricingApplied: true,
		},
	}
}

func maximizeSupplierAccountingContractEnvelope(envelope types.SupplierAccountingEnvelopeV1) types.SupplierAccountingEnvelopeV1 {
	envelope.ActivationStateVersion = math.MaxInt64
	snapshot := envelope.Captured
	maxInt := int(^uint(0) >> 1)
	snapshot.BindingVersionId = maxInt
	snapshot.SupplierId = maxInt
	snapshot.ContractId = maxInt
	snapshot.RateVersionId = maxInt
	snapshot.ProcurementMultiplierPpm = 1_000_000
	*snapshot.OfficialListMicroUsd = math.MaxInt64
	*snapshot.ProcurementCostMicroUsd = math.MaxInt64
	snapshot.FinanciallyCommittedAt = math.MaxInt64
	groupMultiplier := int64(math.MaxInt64)
	if snapshot.StatisticsScope == string(types.SupplierStatisticsScopeInternal) {
		exclusionRuleID := maxInt
		snapshot.ExclusionRuleId = &exclusionRuleID
	} else {
		sales := int64(0)
		grossProfit := -int64(math.MaxInt64)
		snapshot.SalesMultiplierPpm = &groupMultiplier
		snapshot.SalesMicroUsd = &sales
		snapshot.GrossProfitMicroUsd = &grossProfit
	}
	tiered := snapshot.PricingProvenance.Tiered
	tiered.ExpressionFingerprint = math.MaxUint64
	tiered.ExpressionFingerprintTail = supplierExpressionFingerprintTailMaxV1
	tiered.ExpressionVersion = 1
	tiered.GroupMultiplierPpm = groupMultiplier
	tiered.GroupRatioVersion = 1
	tiered.NormalizedInputs = types.SupplierTieredNormalizedInputsV1{
		Prompt: math.MaxInt64, Completion: math.MaxInt64, ContextLength: math.MaxInt64,
		CacheRead: math.MaxInt64, CacheCreate: math.MaxInt64, CacheCreate1H: math.MaxInt64,
		ImageInput: math.MaxInt64, ImageOutput: math.MaxInt64, AudioInput: math.MaxInt64, AudioOutput: math.MaxInt64,
	}
	return envelope
}

func marshalSupplierAccountingContractOuter(t testing.TB, envelope types.SupplierAccountingEnvelopeV1) []byte {
	t.Helper()
	payload, err := common.Marshal(map[string]any{types.SupplierAccountingEnvelopeKeyV1: envelope})
	require.NoError(t, err)
	require.Contains(t, string(payload), `"`+types.SupplierAccountingEnvelopeKeyV1+`"`)
	return payload
}

func requireSupplierAccountingContractPayloadHasNoDeploymentIdentity(t testing.TB, payload []byte) {
	t.Helper()
	lower := strings.ToLower(string(payload))
	for _, forbidden := range []string{"artifact", "manifest", "build_commit", "build_provenance", "oci", "model_name", `"model"`} {
		require.NotContains(t, lower, forbidden)
	}
}

func setSupplierAccountingContractActivationCache(t testing.TB) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	original := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = original
		common.OptionMapRWMutex.Unlock()
	})
}

func sortedSupplierAccountingContractKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
