package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestTopUpBonusAmountUsesRequestedPreset(t *testing.T) {
	paymentSetting := operation_setting.GetPaymentSetting()
	originalDisplayType := operation_setting.GetQuotaDisplayType()
	originalBonus := paymentSetting.AmountBonus
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalDisplayType
		paymentSetting.AmountBonus = originalBonus
	})

	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	paymentSetting.AmountBonus = map[int]int64{
		20: 5,
	}

	require.Equal(t, int64(20), normalizeTopUpAmount(20))
	require.Equal(t, int64(5), configuredTopUpBonusAmount(20))
	require.Equal(t, int64(0), configuredTopUpBonusAmount(33))
}

func TestConfiguredTopUpAmountsReturnsBaseAndBonusSeparately(t *testing.T) {
	paymentSetting := operation_setting.GetPaymentSetting()
	originalDisplayType := operation_setting.GetQuotaDisplayType()
	originalBonus := paymentSetting.AmountBonus
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalDisplayType
		paymentSetting.AmountBonus = originalBonus
	})

	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	paymentSetting.AmountBonus = map[int]int64{20: 5}

	amount, bonus, tier := configuredTopUpAmounts(0, 20)

	require.Equal(t, int64(20), amount) // Amount 只存本金，赠送是否发放推迟到回调判次
	require.Equal(t, int64(5), bonus)
	require.Equal(t, 20, tier) // 无阶段命中时 tier = 充值金额
}

func TestTopUpBonusAmountNormalizesTokenDisplay(t *testing.T) {
	paymentSetting := operation_setting.GetPaymentSetting()
	originalDisplayType := operation_setting.GetQuotaDisplayType()
	originalBonus := paymentSetting.AmountBonus
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalDisplayType
		paymentSetting.AmountBonus = originalBonus
	})

	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeTokens
	requestAmount := int64(20 * common.QuotaPerUnit)
	paymentSetting.AmountBonus = map[int]int64{
		int(requestAmount): int64(5 * common.QuotaPerUnit),
	}

	require.Equal(t, int64(20), normalizeTopUpAmount(requestAmount))
	require.Equal(t, int64(5), configuredTopUpBonusAmount(requestAmount))
}

func TestConfiguredBonusDoesNotChangeChannelPayMoney(t *testing.T) {
	paymentSetting := operation_setting.GetPaymentSetting()
	originalDisplayType := operation_setting.GetQuotaDisplayType()
	originalPrice := operation_setting.Price
	originalStripeUnitPrice := setting.StripeUnitPrice
	originalPaddleUnitPrice := setting.PaddleUnitPrice
	originalWaffoUnitPrice := setting.WaffoUnitPrice
	originalWaffoPancakeUnitPrice := setting.WaffoPancakeUnitPrice
	originalDiscount := paymentSetting.AmountDiscount
	originalBonus := paymentSetting.AmountBonus
	originalTopupGroupRatio := common.TopupGroupRatio2JSONString()
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalDisplayType
		operation_setting.Price = originalPrice
		setting.StripeUnitPrice = originalStripeUnitPrice
		setting.PaddleUnitPrice = originalPaddleUnitPrice
		setting.WaffoUnitPrice = originalWaffoUnitPrice
		setting.WaffoPancakeUnitPrice = originalWaffoPancakeUnitPrice
		paymentSetting.AmountDiscount = originalDiscount
		paymentSetting.AmountBonus = originalBonus
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(originalTopupGroupRatio))
	})

	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	operation_setting.Price = 1
	setting.StripeUnitPrice = 1
	setting.PaddleUnitPrice = 1
	setting.WaffoUnitPrice = 1
	setting.WaffoPancakeUnitPrice = 1
	paymentSetting.AmountDiscount = map[int]float64{}
	paymentSetting.AmountBonus = map[int]int64{20: 5}
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"default":1}`))

	require.Equal(t, 20.0, getPayMoney(20, "default"))
	require.Equal(t, 20.0, getStripePayMoney(20, "default"))
	require.Equal(t, 20.0, getPaddlePayMoney(20, "default"))
	require.Equal(t, 20.0, getWaffoPayMoney(20, "default"))
	require.Equal(t, 20.0, getWaffoPancakePayMoney(20, "default"))
}

func TestResolveStageBonus_OverridesGlobal(t *testing.T) {
	originalDisplayType := operation_setting.GetQuotaDisplayType()
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalDisplayType
	})
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD

	// 用户处于 E3 窗口内,充 50 享阶段档 ($50 送 $30)
	amount, bonus, tier := resolveStageBonus(50, []stageWindowHit{{Step: 3, Amount: 50, Bonus: 30}})
	require.Equal(t, int64(50), amount)
	require.Equal(t, int64(30), bonus, "阶段 bonus 应取代全局")
	require.Equal(t, 50, tier)
}

func TestResolveStageBonus_BelowThreshold(t *testing.T) {
	originalDisplayType := operation_setting.GetQuotaDisplayType()
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalDisplayType
	})
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD

	// 阶段要求充 50,用户只充 20 → 不享阶段
	_, bonus, tier := resolveStageBonus(20, []stageWindowHit{{Step: 3, Amount: 50, Bonus: 30}})
	require.Equal(t, int64(0), bonus, "低于阶段档位不享阶段 bonus")
	require.Equal(t, 0, tier)
}

func TestResolveStageBonus_PicksHighest(t *testing.T) {
	originalDisplayType := operation_setting.GetQuotaDisplayType()
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalDisplayType
	})
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD

	// 同时命中 E3($50送$30) 和 E4($100送$80),充 $100 → 取最高 E4
	hits := []stageWindowHit{
		{Step: 3, Amount: 50, Bonus: 30},
		{Step: 4, Amount: 100, Bonus: 80},
	}
	amount, bonus, tier := resolveStageBonus(100, hits)
	require.Equal(t, int64(100), amount)
	require.Equal(t, int64(80), bonus)
	require.Equal(t, 100, tier)
}

func TestResolveStageBonus_NoHits(t *testing.T) {
	_, bonus, tier := resolveStageBonus(50, nil)
	require.Equal(t, int64(0), bonus)
	require.Equal(t, 0, tier)
}
