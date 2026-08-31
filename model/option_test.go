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
