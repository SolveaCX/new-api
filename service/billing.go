package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const (
	BillingSourceWallet       = "wallet"
	BillingSourceSubscription = "subscription"
)

// PreConsumeBilling 根据用户计费偏好创建 BillingSession 并执行预扣费。
// 会话存储在 relayInfo.Billing 上，供后续 Settle / Refund 使用。
func PreConsumeBilling(c *gin.Context, preConsumedQuota int, relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
	session, apiErr := NewBillingSession(c, relayInfo, preConsumedQuota)
	if apiErr != nil {
		return apiErr
	}
	relayInfo.Billing = session
	return nil
}

// ---------------------------------------------------------------------------
// SettleBilling — 后结算辅助函数
// ---------------------------------------------------------------------------

// SettleBilling 执行计费结算并保留旧的 error 返回约定。
func SettleBilling(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, actualQuota int) error {
	return SettleBillingResult(ctx, relayInfo, actualQuota).Err
}

// SettleBillingResult 执行计费结算并同步返回资金提交状态。如果 RelayInfo 上有
// BillingSession 则通过 session 结算，否则回退到旧的 PostConsumeQuota 路径。
func SettleBillingResult(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, actualQuota int) types.BillingSettlementResult {
	if relayInfo.Billing != nil {
		preConsumed := relayInfo.Billing.GetPreConsumedQuota()
		delta := actualQuota - preConsumed

		if delta > 0 {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费后补扣费：%s（实际消耗：%s，预扣费：%s）",
				logger.FormatQuota(delta),
				logger.FormatQuota(actualQuota),
				logger.FormatQuota(preConsumed),
			))
		} else if delta < 0 {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费后返还扣费：%s（实际消耗：%s，预扣费：%s）",
				logger.FormatQuota(-delta),
				logger.FormatQuota(actualQuota),
				logger.FormatQuota(preConsumed),
			))
		} else {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费与实际消耗一致，无需调整：%s（按次计费）",
				logger.FormatQuota(actualQuota),
			))
		}

		var result types.BillingSettlementResult
		if settler, ok := relayInfo.Billing.(interface {
			SettleWithResult(actualQuota int) types.BillingSettlementResult
		}); ok {
			result = settler.SettleWithResult(actualQuota)
		} else if err := relayInfo.Billing.Settle(actualQuota); err != nil {
			result = types.BillingSettlementResult{FinalSalesQuota: actualQuota, Err: err}
		} else {
			result = types.BillingSettlementResult{
				FinanciallyCommitted:   true,
				FinanciallyCommittedAt: common.GetTimestamp(),
				FinalSalesQuota:        actualQuota,
			}
		}
		if result.Err != nil {
			return result
		}

		// 发送额度通知（订阅计费使用订阅剩余额度）
		if actualQuota != 0 {
			if relayInfo.BillingSource == BillingSourceSubscription {
				checkAndSendSubscriptionQuotaNotify(relayInfo)
			} else {
				checkAndSendQuotaNotify(relayInfo, actualQuota-preConsumed, preConsumed)
			}
		}
		return result
	}

	// 回退：无 BillingSession 时使用旧路径
	quotaDelta := actualQuota - relayInfo.FinalPreConsumedQuota
	if quotaDelta == 0 {
		return types.BillingSettlementResult{
			FinanciallyCommitted:   true,
			FinanciallyCommittedAt: common.GetTimestamp(),
			FinalSalesQuota:        actualQuota,
		}
	}
	result := postConsumeQuotaWithResult(relayInfo, quotaDelta, relayInfo.FinalPreConsumedQuota, true)
	if !result.FundingCommitted {
		return types.BillingSettlementResult{FinalSalesQuota: actualQuota, Err: result.Err}
	}
	return types.BillingSettlementResult{
		FinanciallyCommitted:   true,
		FinanciallyCommittedAt: common.GetTimestamp(),
		FinalSalesQuota:        actualQuota,
		Err:                    result.Err,
	}
}
