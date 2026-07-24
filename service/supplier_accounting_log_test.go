package service

import (
	"context"
	"math"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func supplierAccountingTestRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		PriceData: types.PriceData{GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.3}},
		SupplierCostSnapshot: types.SupplierCostSnapshot{
			BindingVersionId: 17, SupplierId: 2, ContractId: 3,
			RateVersionId: 4, ProcurementMultiplierPpm: 500_000,
		},
		SupplierStatisticsScopeSnapshot: types.BusinessSupplierStatisticsScopeSnapshot(),
		SupplierOfficialPricingSnapshot: types.SupplierOfficialPricingSnapshot{
			Loaded: true, QuotaPerUnit: "500000",
			PriceData: types.PriceData{GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 0.3}},
		},
	}
}

func TestSupplierAccountingLogSnapshotFreezesNominalRequestMultiplierAcrossPricingModes(t *testing.T) {
	tests := []struct {
		name        string
		mode        string
		groupRatio  float64
		expectedPpm int64
		configure   func(*relaycommon.RelayInfo)
	}{
		{name: "fixed", mode: "fixed", groupRatio: 0.7, expectedPpm: 700_000},
		{name: "ratio_cache", mode: "ratio", groupRatio: 0.67, expectedPpm: 670_000, configure: func(info *relaycommon.RelayInfo) {
			info.SupplierOfficialPricingSnapshot.PriceData.OtherRatios = map[string]float64{"cache": 0.1}
		}},
		{name: "tiered_final_retry_group", mode: "tiered_expr", groupRatio: 0.9, expectedPpm: 900_000, configure: func(info *relaycommon.RelayInfo) {
			info.PriceData.GroupRatioInfo.GroupRatio = 0.9
			info.SupplierOfficialPricingSnapshot.PriceData.GroupRatioInfo.GroupRatio = 0.8
			info.SupplierOfficialPricingSnapshot.TieredBillingSnapshot = &billingexpr.BillingSnapshot{GroupRatio: 0.6}
		}},
		{name: "other_ratios", mode: "ratio", groupRatio: 0.8, expectedPpm: 800_000, configure: func(info *relaycommon.RelayInfo) {
			info.SupplierOfficialPricingSnapshot.PriceData.OtherRatios = map[string]float64{"duration": 2, "resolution": 1.5}
		}},
		{name: "small_quota_rounding", mode: "ratio", groupRatio: 0.3, expectedPpm: 300_000},
		{name: "seven_decimal_places_round_half_up", mode: "ratio", groupRatio: 0.6666667, expectedPpm: 666_667},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := supplierAccountingTestRelayInfo()
			info.PriceData.GroupRatioInfo.GroupRatio = tt.groupRatio
			info.SupplierOfficialPricingSnapshot.PriceData.GroupRatioInfo.GroupRatio = tt.groupRatio
			if tt.configure != nil {
				tt.configure(info)
			}
			official := decimal.RequireFromString("0.000001")
			snapshot := BuildSupplierAccountingLogSnapshotV1(info, types.BillingSettlementResult{
				FinanciallyCommitted: true, FinanciallyCommittedAt: 123, FinalSalesQuota: 1,
			}, &official, "", tt.mode)
			require.NotNil(t, snapshot)
			require.NotNil(t, snapshot.SalesMultiplierPpm)
			require.EqualValues(t, tt.expectedPpm, *snapshot.SalesMultiplierPpm)
		})
	}
}

func TestSupplierAccountingLogSnapshotSerializationSizeAndInternalOmissions(t *testing.T) {
	official := decimal.RequireFromString("1.25")
	settlement := types.BillingSettlementResult{FinanciallyCommitted: true, FinanciallyCommittedAt: 123, FinalSalesQuota: 1_000_000}
	business := BuildSupplierAccountingLogSnapshotV1(supplierAccountingTestRelayInfo(), settlement, &official, "", "ratio")
	businessJSON, err := common.Marshal(business)
	require.NoError(t, err)
	require.Less(t, len(businessJSON), 260, string(businessJSON))

	internalInfo := supplierAccountingTestRelayInfo()
	internalInfo.SupplierStatisticsScopeSnapshot = types.SupplierStatisticsScopeSnapshot{Scope: types.SupplierStatisticsScopeInternal, ExclusionRuleId: 99}
	internal := BuildSupplierAccountingLogSnapshotV1(internalInfo, settlement, &official, "", "ratio")
	internalJSON, err := common.Marshal(internal)
	require.NoError(t, err)
	require.Less(t, len(internalJSON), 190, string(internalJSON))
	require.NotContains(t, string(internalJSON), `"sm"`)
	require.NotContains(t, string(internalJSON), `"q"`)
	require.NotContains(t, string(internalJSON), `"p"`)
	require.NotContains(t, string(internalJSON), `"sa"`)
	require.NotContains(t, string(internalJSON), `"gp"`)
}

func TestSupplierAccountingLogSnapshotFreezesKnownValuesAndSalesMultiplier(t *testing.T) {
	settlement := types.BillingSettlementResult{FinanciallyCommitted: true, FinanciallyCommittedAt: 123, FinalSalesQuota: 0}
	knownZero := decimal.Zero
	snapshot := BuildSupplierAccountingLogSnapshotV1(supplierAccountingTestRelayInfo(), settlement, &knownZero, "", "fixed")
	require.NotNil(t, snapshot)
	require.NotNil(t, snapshot.OfficialListMicroUsd)
	require.Zero(t, *snapshot.OfficialListMicroUsd)
	require.NotNil(t, snapshot.SalesMicroUsd)
	require.Zero(t, *snapshot.SalesMicroUsd)
	require.NotNil(t, snapshot.ProcurementCostMicroUsd)
	require.Zero(t, *snapshot.ProcurementCostMicroUsd)
	require.NotNil(t, snapshot.GrossProfitMicroUsd)
	require.Zero(t, *snapshot.GrossProfitMicroUsd)
	require.NotNil(t, snapshot.SalesMultiplierPpm)
	require.EqualValues(t, 300_000, *snapshot.SalesMultiplierPpm)
	require.Equal(t, "included", snapshot.ExclusionDecision)
	require.Equal(t, 17, snapshot.BindingVersionId)
}

func TestSupplierAccountingLogSnapshotReportsUnavailableMultiplierWithoutFabrication(t *testing.T) {
	info := supplierAccountingTestRelayInfo()
	info.PriceData.GroupRatioInfo.GroupRatio = math.NaN()
	info.SupplierOfficialPricingSnapshot.PriceData.GroupRatioInfo.GroupRatio = math.NaN()
	official := decimal.RequireFromString("1.25")
	snapshot := BuildSupplierAccountingLogSnapshotV1(info, types.BillingSettlementResult{
		FinanciallyCommitted: true, FinanciallyCommittedAt: 456, FinalSalesQuota: 1_000_000,
	}, &official, "", "ratio")
	require.NotNil(t, snapshot)
	require.Nil(t, snapshot.SalesMultiplierPpm)
	require.Contains(t, snapshot.QualityReason, "supplier_accounting.sales_multiplier_ppm.non_finite")
}

func TestSupplierAccountingLogSnapshotSkipsUncommittedAndFreezesExclusion(t *testing.T) {
	info := supplierAccountingTestRelayInfo()
	require.Nil(t, BuildSupplierAccountingLogSnapshotV1(info, types.BillingSettlementResult{}, nil, "", "ratio"))

	info.SupplierStatisticsScopeSnapshot = types.SupplierStatisticsScopeSnapshot{
		Scope: types.SupplierStatisticsScopeInternal, ExclusionRuleId: 99,
	}
	official := decimal.RequireFromString("1.25")
	snapshot := BuildSupplierAccountingLogSnapshotV1(info, types.BillingSettlementResult{
		FinanciallyCommitted: true, FinanciallyCommittedAt: 456, FinalSalesQuota: 1_000_000,
	}, &official, "", "tiered_expr")
	require.NotNil(t, snapshot)
	require.Equal(t, "excluded", snapshot.ExclusionDecision)
	require.NotNil(t, snapshot.ExclusionRuleId)
	require.Equal(t, 99, *snapshot.ExclusionRuleId)
	require.Nil(t, snapshot.SalesMicroUsd)
	require.Nil(t, snapshot.GrossProfitMicroUsd)
}

func TestAccountingEnvelopeSkipsZeroUsageButKeepsFixedPriceSuccessContract(t *testing.T) {
	info := supplierAccountingTestRelayInfo()
	settlement := types.BillingSettlementResult{FinanciallyCommitted: true, FinanciallyCommittedAt: 123, FinalSalesQuota: 1}
	official := decimal.RequireFromString("1")
	zeroUsage := BuildSupplierAccountingEnvelopeV1(SupplierAccountingEnvelopeInputV1{
		RelayInfo: info, Settlement: settlement,
	})
	require.Equal(t, types.SupplierAccountingDispositionZeroUsage, zeroUsage.Disposition)
	require.Nil(t, zeroUsage.Captured)

	// Fixed-price successful calls are not inferred from token count. Their
	// non-token settlement path continues to use the general snapshot builder.
	fixed := BuildSupplierAccountingLogSnapshotV1(info, settlement, &official, "", "fixed")
	require.NotNil(t, fixed)
	require.Equal(t, "fixed", *fixed.PricingMode)
}

func TestFinalRetrySupplierAndGroupPersistIntoConsumeLogAndDailySummary(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)

	info := supplierAccountingTestRelayInfo()
	info.SupplierCostSnapshot = types.SupplierCostSnapshot{
		BindingVersionId: 101, SupplierId: 11, ContractId: 12,
		RateVersionId: 13, ProcurementMultiplierPpm: 700_000,
	}
	info.SupplierOfficialPricingSnapshot.TieredBillingSnapshot = &billingexpr.BillingSnapshot{
		GroupRatio: 0.3, EstimatedQuotaBeforeGroup: 1_000_000, EstimatedQuotaAfterGroup: 300_000,
		ExprString: "p+c", ExprHash: billingexpr.ExprHashString("p+c"), ExprVersion: 1,
	}

	// Simulate the successful retry selecting a different supplier binding and
	// a different auto-group multiplier than the first attempt.
	info.SupplierCostSnapshot = types.SupplierCostSnapshot{
		BindingVersionId: 201, SupplierId: 21, ContractId: 22,
		RateVersionId: 23, ProcurementMultiplierPpm: 250_000,
	}
	finalGroup := types.GroupRatioInfo{GroupRatio: 0.9}
	info.PriceData.GroupRatioInfo = finalGroup
	info.SupplierOfficialPricingSnapshot.PriceData.GroupRatioInfo = finalGroup
	info.SupplierOfficialPricingSnapshot.TieredBillingSnapshot.GroupRatio = finalGroup.GroupRatio
	info.SupplierOfficialPricingSnapshot.TieredBillingSnapshot.EstimatedQuotaAfterGroup = billingexpr.QuotaRound(
		info.SupplierOfficialPricingSnapshot.TieredBillingSnapshot.EstimatedQuotaBeforeGroup * finalGroup.GroupRatio,
	)
	official := decimal.RequireFromString("2")
	settlement := types.BillingSettlementResult{
		FinanciallyCommitted: true, FinanciallyCommittedAt: day.Add(time.Hour).Unix(), FinalSalesQuota: 900_000,
	}
	envelope := BuildSupplierAccountingEnvelopeV1(SupplierAccountingEnvelopeInputV1{
		RelayInfo: info, Settlement: settlement, HasPositiveFinalUsage: true,
		Capture: SupplierAccountingCaptureInputV1{
			OfficialListUSD: &official, PricingMode: "tiered_expr",
			TieredTokenParams: &billingexpr.TokenParams{P: 1, C: 1, Len: 2},
		},
	})
	require.NoError(t, ValidateSupplierAccountingEnvelopeV1(envelope))
	require.NotNil(t, envelope.Captured)
	other, err := common.Marshal(map[string]any{types.SupplierAccountingEnvelopeKeyV1: envelope, "matched_tier": "final"})
	require.NoError(t, err)
	require.NoError(t, logDB.Create(&model.Log{
		Type: model.LogTypeConsume, CreatedAt: day.Add(time.Hour).Unix(), ChannelId: 99,
		ModelName: "retry-model", Other: string(other),
	}).Error)
	require.NoError(t, RunSupplierDailyBatch(context.Background(), mainDB, logDB, day.Format("2006-01-02"), "node-final", day.AddDate(0, 0, 2)))

	var persisted model.Log
	require.NoError(t, logDB.First(&persisted).Error)
	var persistedEnvelope supplierAccountingLogEnvelope
	require.NoError(t, common.Unmarshal([]byte(persisted.Other), &persistedEnvelope))
	var persistedSnapshot types.SupplierAccountingLogSnapshotV1
	require.NoError(t, common.Unmarshal(persistedEnvelope.SupplierAccountingV1, &envelope))
	require.NotNil(t, envelope.Captured)
	persistedSnapshot = *envelope.Captured
	require.Equal(t, 201, persistedSnapshot.BindingVersionId)
	require.Equal(t, 21, persistedSnapshot.SupplierId)
	require.Equal(t, 22, persistedSnapshot.ContractId)
	require.Equal(t, 23, persistedSnapshot.RateVersionId)
	require.EqualValues(t, 250_000, persistedSnapshot.ProcurementMultiplierPpm)
	require.NotNil(t, persistedSnapshot.SalesMultiplierPpm)
	require.EqualValues(t, 900_000, *persistedSnapshot.SalesMultiplierPpm)

	var summaries []model.SupplierUsageDailySummary
	require.NoError(t, mainDB.Find(&summaries).Error)
	require.Len(t, summaries, 1)
	summary := summaries[0]
	require.Equal(t, 201, summary.BindingVersionId)
	require.Equal(t, 21, summary.SupplierId)
	require.Equal(t, 22, summary.ContractId)
	require.Equal(t, 23, summary.RateVersionId)
	require.NotNil(t, summary.SalesMultiplierPpm)
	require.EqualValues(t, 900_000, *summary.SalesMultiplierPpm)
	require.EqualValues(t, 1, summary.RequestCount)
	require.EqualValues(t, 2_000_000, summary.OfficialListMicroUsd)
	require.EqualValues(t, 500_000, summary.ProcurementCostMicroUsd)
}

func TestPostTextConsumeQuotaOmitsZeroUsageEnvelope(t *testing.T) {
	const tokenID = 987654
	require.NoError(t, model.LOG_DB.Where("token_id = ?", tokenID).Delete(&model.Log{}).Error)
	t.Cleanup(func() { _ = model.LOG_DB.Where("token_id = ?", tokenID).Delete(&model.Log{}).Error })

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("username", "zero-token-user")
	ctx.Set("token_name", "zero-token")
	info := supplierAccountingTestRelayInfo()
	info.StartTime = time.Now()
	info.UserId = 123456
	info.TokenId = tokenID
	info.TokenKey = "test"
	info.OriginModelName = "zero-token-model"
	info.UsingGroup = "default"
	info.ChannelMeta = &relaycommon.ChannelMeta{ChannelId: 765432}

	PostTextConsumeQuota(ctx, info, &dto.Usage{}, nil)

	var persisted model.Log
	require.NoError(t, model.LOG_DB.Where("token_id = ?", tokenID).First(&persisted).Error)
	require.Equal(t, model.LogTypeConsume, persisted.Type)
	var other map[string]any
	require.NoError(t, common.UnmarshalJsonStr(persisted.Other, &other))
	require.NotContains(t, other, types.SupplierAccountingEnvelopeKeyV1)
}
