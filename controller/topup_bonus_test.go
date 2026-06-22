package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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
	require.Equal(t, model.StageBonusTier(3), tier, "tier 用阶段专用命名空间编码")
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
	require.Equal(t, model.StageBonusTier(4), tier, "取最高阶段 E4 的 tier 编码")
}

func TestResolveStageBonus_NoHits(t *testing.T) {
	_, bonus, tier := resolveStageBonus(50, nil)
	require.Equal(t, int64(0), bonus)
	require.Equal(t, 0, tier)
}

// TestTokensModeCollisionIsHarmless 锁定 C1 复核发现的边界:TOKENS 展示模式下
// req.Amount = 美元 × QuotaPerUnit,充值 >= $2 时数值会落入阶段 tier 命名空间(>=1000000)。
// 验证无阶段命中(userId=0)时,普通路径 bonus 恒为 0(AmountBonus 按美元 key 必然 miss),
// 因此即便 tier 数值与阶段命名空间重叠也不会错误发放——碰撞无害。
func TestTokensModeCollisionIsHarmless(t *testing.T) {
	paymentSetting := operation_setting.GetPaymentSetting()
	originalDisplayType := operation_setting.GetQuotaDisplayType()
	originalBonus := paymentSetting.AmountBonus
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalDisplayType
		paymentSetting.AmountBonus = originalBonus
	})
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeTokens
	paymentSetting.AmountBonus = map[int]int64{} // 美元 key 配置,TOKENS 模式下传入 token 数必然 miss

	// 充值 $2 → req.Amount = 1000000(= StageBonusTierBase),数值落在阶段命名空间
	requestAmount := int64(2 * common.QuotaPerUnit)
	require.True(t, requestAmount >= int64(model.StageBonusTierBase), "前提:该金额数值确实进入阶段命名空间区间")

	// 无阶段命中(userId=0)→ 普通路径 → bonus 必须为 0(不会因 tier 数值碰撞而误发)
	_, bonus, _ := configuredTopUpAmounts(0, requestAmount)
	require.Equal(t, int64(0), bonus, "TOKENS 模式碰撞无害:普通路径 bonus 恒为 0")
}
