package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/require"
)

func TestStripePromotionCodeOption(t *testing.T) {
	setupOptionGroupRenameTestDB(t)
	original := setting.StripePromotionCodeEnabled
	t.Cleanup(func() { setting.StripePromotionCodeEnabled = original })

	InitOptionMap()
	require.NoError(t, UpdateOption("StripePromotionCodeEnabled", "true"))
	require.True(t, setting.StripePromotionCodeEnabled)

	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	require.Equal(t, "true", common.OptionMap["StripePromotionCodeEnabled"])
}

func TestDefaultSystemNoticeIsSeeded(t *testing.T) {
	setupOptionGroupRenameTestDB(t)
	InitOptionMap()

	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	require.Contains(t, common.OptionMap["Notice"], "反滥用/邀请规则")
	require.Contains(t, common.OptionMap["Notice"], "所有支持语言")
}
