package service

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type supplierAccountingDispositionBilling struct {
	committed bool
	err       error
}

func (billing *supplierAccountingDispositionBilling) Settle(int) error { return billing.err }

func (billing *supplierAccountingDispositionBilling) SettleWithResult(actualQuota int) types.BillingSettlementResult {
	result := types.BillingSettlementResult{FinalSalesQuota: actualQuota, Err: billing.err}
	if billing.committed && billing.err == nil {
		result.FinanciallyCommitted = true
		result.FinanciallyCommittedAt = 1_784_801_200
	}
	return result
}

func (*supplierAccountingDispositionBilling) Refund(*gin.Context)      {}
func (*supplierAccountingDispositionBilling) NeedsRefund() bool        { return false }
func (*supplierAccountingDispositionBilling) GetPreConsumedQuota() int { return 0 }
func (*supplierAccountingDispositionBilling) Reserve(int) error        { return nil }

func TestDynamicConsumeWritersPersistEverySupplierDisposition(t *testing.T) {
	originalLogConsumeEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() { common.LogConsumeEnabled = originalLogConsumeEnabled })

	paths := []struct {
		name string
		run  func(*gin.Context, *relaycommon.RelayInfo, bool)
	}{
		{name: "text", run: func(ctx *gin.Context, info *relaycommon.RelayInfo, positiveUsage bool) {
			usage := &dto.Usage{}
			if positiveUsage {
				usage.PromptTokens = 1
				usage.TotalTokens = 1
			}
			PostTextConsumeQuota(ctx, info, usage, nil)
		}},
		{name: "audio", run: func(ctx *gin.Context, info *relaycommon.RelayInfo, positiveUsage bool) {
			usage := &dto.Usage{}
			if positiveUsage {
				usage.PromptTokens = 1
				usage.TotalTokens = 1
				usage.PromptTokensDetails.TextTokens = 1
			}
			PostAudioConsumeQuota(ctx, info, usage, "")
		}},
		{name: "wss", run: func(ctx *gin.Context, info *relaycommon.RelayInfo, positiveUsage bool) {
			usage := &dto.RealtimeUsage{}
			if positiveUsage {
				usage.InputTokens = 1
				usage.TotalTokens = 1
				usage.InputTokenDetails.TextTokens = 1
			}
			PostWssConsumeQuota(ctx, info, info.OriginModelName, usage, "")
		}},
	}

	tests := []struct {
		name          string
		disposition   types.SupplierAccountingDisposition
		fixedPrice    bool
		binding       string
		committed     bool
		settleErr     error
		positiveUsage bool
	}{
		{name: "not_financially_committed", disposition: types.SupplierAccountingDispositionNotFinanciallyCommitted, fixedPrice: true, binding: "valid", settleErr: errors.New("settlement rejected"), positiveUsage: true},
		{name: "zero_usage", disposition: types.SupplierAccountingDispositionZeroUsage, fixedPrice: true, binding: "valid", committed: true},
		{name: "unbound", disposition: types.SupplierAccountingDispositionUnbound, fixedPrice: true, binding: "absent", committed: true, positiveUsage: true},
		{name: "captured", disposition: types.SupplierAccountingDispositionCaptured, fixedPrice: true, binding: "valid", committed: true, positiveUsage: true},
		{name: "producer_error", disposition: types.SupplierAccountingDispositionProducerError, fixedPrice: true, binding: "partial", committed: true, positiveUsage: true},
	}

	const tokenBase = 9_870_000
	caseNumber := 0
	for _, path := range paths {
		for _, testCase := range tests {
			caseNumber++
			t.Run(path.name+"/"+testCase.name, func(t *testing.T) {
				tokenID := tokenBase + caseNumber
				require.NoError(t, model.LOG_DB.Where("token_id = ?", tokenID).Delete(&model.Log{}).Error)
				t.Cleanup(func() { _ = model.LOG_DB.Where("token_id = ?", tokenID).Delete(&model.Log{}).Error })

				gin.SetMode(gin.TestMode)
				ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
				ctx.Set("username", "supplier-disposition-test")
				ctx.Set("token_name", "supplier-disposition-token")
				info := supplierAccountingDynamicWriterInfo(tokenID, testCase.fixedPrice, testCase.binding)
				info.Billing = &supplierAccountingDispositionBilling{committed: testCase.committed, err: testCase.settleErr}

				path.run(ctx, info, testCase.positiveUsage)

				var persisted model.Log
				require.NoError(t, model.LOG_DB.Where("token_id = ?", tokenID).First(&persisted).Error)
				require.Equal(t, model.LogTypeConsume, persisted.Type)
				envelope := decodePersistedSupplierAccountingEnvelope(t, persisted.Other)
				require.Equal(t, testCase.disposition, envelope.Disposition)
				if testCase.disposition == types.SupplierAccountingDispositionCaptured {
					require.NotNil(t, envelope.Captured)
					require.NoError(t, ValidateSupplierAccountingEnvelopeV1(envelope))
				} else {
					require.Nil(t, envelope.Captured)
				}
			})
		}
	}
}

func TestDynamicConsumeWritersRejectNonAuthoritativeOfficialEvidence(t *testing.T) {
	originalLogConsumeEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() { common.LogConsumeEnabled = originalLogConsumeEnabled })

	paths := []struct {
		name string
		run  func(*gin.Context, *relaycommon.RelayInfo)
	}{
		{name: "text", run: func(ctx *gin.Context, info *relaycommon.RelayInfo) {
			PostTextConsumeQuota(ctx, info, &dto.Usage{PromptTokens: 1, TotalTokens: 1}, nil)
		}},
		{name: "audio", run: func(ctx *gin.Context, info *relaycommon.RelayInfo) {
			PostAudioConsumeQuota(ctx, info, &dto.Usage{PromptTokens: 1, TotalTokens: 1, PromptTokensDetails: dto.InputTokenDetails{TextTokens: 1}}, "")
		}},
		{name: "wss", run: func(ctx *gin.Context, info *relaycommon.RelayInfo) {
			PostWssConsumeQuota(ctx, info, info.OriginModelName, &dto.RealtimeUsage{InputTokens: 1, TotalTokens: 1, InputTokenDetails: dto.InputTokenDetails{TextTokens: 1}}, "")
		}},
	}

	const tokenBase = 9_880_000
	for index, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			tokenID := tokenBase + index
			require.NoError(t, model.LOG_DB.Where("token_id = ?", tokenID).Delete(&model.Log{}).Error)
			t.Cleanup(func() { _ = model.LOG_DB.Where("token_id = ?", tokenID).Delete(&model.Log{}).Error })

			gin.SetMode(gin.TestMode)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Set("username", "supplier-estimate-test")
			ctx.Set("token_name", "supplier-estimate-token")
			ctx.Set(string(constant.ContextKeyLocalCountTokens), true)
			info := supplierAccountingDynamicWriterInfo(tokenID, true, "valid")
			info.Billing = &supplierAccountingDispositionBilling{committed: true}

			path.run(ctx, info)

			var persisted model.Log
			require.NoError(t, model.LOG_DB.Where("token_id = ?", tokenID).First(&persisted).Error)
			envelope := decodePersistedSupplierAccountingEnvelope(t, persisted.Other)
			require.Equal(t, types.SupplierAccountingDispositionProducerError, envelope.Disposition)
			require.Nil(t, envelope.Captured)
		})
	}
}

func supplierAccountingDynamicWriterInfo(tokenID int, fixedPrice bool, binding string) *relaycommon.RelayInfo {
	priceData := types.PriceData{
		ModelPrice:           1,
		ModelRatio:           1,
		CompletionRatio:      1,
		CacheRatio:           1,
		CacheCreationRatio:   1,
		ImageRatio:           1,
		AudioRatio:           1,
		AudioCompletionRatio: 1,
		UsePrice:             fixedPrice,
		GroupRatioInfo:       types.GroupRatioInfo{GroupRatio: 0.7},
	}
	info := supplierAccountingTestRelayInfo()
	info.StartTime = time.Now()
	info.UserId = 1_234_567
	info.TokenId = tokenID
	info.TokenKey = "supplier-disposition-key"
	info.OriginModelName = "supplier-disposition-model"
	info.UsingGroup = "default"
	info.ChannelMeta = &relaycommon.ChannelMeta{ChannelId: 7_654_321}
	info.PriceData = priceData
	info.SupplierOfficialPricingSnapshot.Loaded = true
	info.SupplierOfficialPricingSnapshot.QuotaPerUnit = "500000"
	info.SupplierOfficialPricingSnapshot.PriceData = priceData
	switch binding {
	case "absent":
		info.SupplierCostSnapshot = types.SupplierCostSnapshot{}
	case "partial":
		info.SupplierCostSnapshot = types.SupplierCostSnapshot{SupplierId: 12}
	}
	return info
}

func decodePersistedSupplierAccountingEnvelope(t *testing.T, other string) types.SupplierAccountingEnvelopeV1 {
	t.Helper()
	var payload struct {
		Envelope types.SupplierAccountingEnvelopeV1 `json:"supplier_accounting_v1"`
	}
	require.NoError(t, common.UnmarshalJsonStr(other, &payload))
	return payload.Envelope
}
