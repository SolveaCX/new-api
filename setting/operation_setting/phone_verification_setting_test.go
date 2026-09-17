package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestPhoneVerificationAPIReminderEnabledRequiresFeatureFlagAndToggle(t *testing.T) {
	prevEnabled := common.SMSVerificationEnabled
	prevSetting := phoneVerificationSetting.APIReminderEnabled
	t.Cleanup(func() {
		common.SMSVerificationEnabled = prevEnabled
		phoneVerificationSetting.APIReminderEnabled = prevSetting
	})

	// Default ships off even with the feature flag on.
	common.SMSVerificationEnabled = true
	phoneVerificationSetting.APIReminderEnabled = false
	require.False(t, PhoneVerificationAPIReminderEnabled())

	// Both on -> enabled.
	phoneVerificationSetting.APIReminderEnabled = true
	require.True(t, PhoneVerificationAPIReminderEnabled())

	// Feature flag off -> disabled regardless of the toggle.
	common.SMSVerificationEnabled = false
	require.False(t, PhoneVerificationAPIReminderEnabled())
}

func TestPhoneVerificationSettingDefaultsToReminderOff(t *testing.T) {
	require.False(t, GetPhoneVerificationSetting().APIReminderEnabled)
}
