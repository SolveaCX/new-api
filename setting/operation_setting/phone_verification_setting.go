package operation_setting

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

// PhoneVerificationSetting holds operator-controlled phone-verification
// behaviour. The feature flag SMS_VERIFICATION_ENABLED (env) still decides
// whether phone verification exists at all; these options only tune what the
// deployment does on top of it.
type PhoneVerificationSetting struct {
	// APIReminderEnabled serves legacy PLG accounts without a verified phone a
	// synthesized "please bind a phone" reply once every 24 hours on their
	// first text-generation API call. Ships off; enable through
	// `phone_verification_setting.api_reminder_enabled`.
	APIReminderEnabled bool `json:"api_reminder_enabled"`
}

var phoneVerificationSetting = PhoneVerificationSetting{
	APIReminderEnabled: false,
}

func init() {
	config.GlobalConfig.Register("phone_verification_setting", &phoneVerificationSetting)
}

func GetPhoneVerificationSetting() *PhoneVerificationSetting {
	return &phoneVerificationSetting
}

// PhoneVerificationAPIReminderEnabled reports whether the API reminder should
// run: the SMS feature must exist and the operator must have opted in.
func PhoneVerificationAPIReminderEnabled() bool {
	return common.SMSVerificationEnabled && GetPhoneVerificationSetting().APIReminderEnabled
}
