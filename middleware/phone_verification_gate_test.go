package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// The gate rule itself is covered in model/phone_verification_policy_test.go.
// These tests only verify the middleware wiring: it must consult the cached
// user's CreatedAt so accounts created before the rollout start are exempt.
func TestAPIPhoneVerificationGateOnlyAppliesToNewUnverifiedPlgAccounts(t *testing.T) {
	originalSMS := common.SMSVerificationEnabled
	common.SMSVerificationEnabled = true
	t.Cleanup(func() { common.SMSVerificationEnabled = originalSMS })
	start := model.PhoneVerificationRolloutStart().Unix()

	tests := []struct {
		name     string
		user     *model.UserBase
		required bool
	}{
		{name: "old plg account is exempt", user: &model.UserBase{Group: "plg", CreatedAt: start - 1}, required: false},
		{name: "legacy plg account without created_at is exempt", user: &model.UserBase{Group: "plg"}, required: false},
		{name: "new plg account must verify", user: &model.UserBase{Group: "plg", CreatedAt: start}, required: true},
		{name: "new verified plg account passes", user: &model.UserBase{Group: "plg", CreatedAt: start, PhoneNumber: "+14155550123", PhoneVerifiedAt: start + 1}, required: false},
		{name: "new non plg account passes", user: &model.UserBase{Group: "enterprise", CreatedAt: start}, required: false},
		{name: "nil user never gated", user: nil, required: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.required, tt.user.PhoneVerificationRequired())
		})
	}
}

func TestAPIPhoneVerificationGateDisabledWithSMSFeature(t *testing.T) {
	original := common.SMSVerificationEnabled
	t.Cleanup(func() { common.SMSVerificationEnabled = original })
	common.SMSVerificationEnabled = false
	require.False(t, (&model.UserBase{Group: "plg", CreatedAt: model.PhoneVerificationRolloutStart().Unix()}).PhoneVerificationRequired())
}

func TestAPIPhoneVerificationNotifyIsLocalized(t *testing.T) {
	require.NoError(t, i18n.Init())
	require.NotEqual(t, i18n.MsgNotifyPhoneVerificationRequiredForAPI, i18n.Translate(i18n.LangZhCN, i18n.MsgNotifyPhoneVerificationRequiredForAPI))
	require.NotEqual(t, i18n.MsgNotifyPhoneVerificationRequiredForAPI, i18n.Translate(i18n.LangEn, i18n.MsgNotifyPhoneVerificationRequiredForAPI))
	require.NotEqual(t, i18n.Translate(i18n.LangZhCN, i18n.MsgNotifyPhoneVerificationRequiredForAPI), i18n.Translate(i18n.LangEn, i18n.MsgNotifyPhoneVerificationRequiredForAPI))
}

func TestAbortWithOpenAIMessageAndNotifyAddsTopLevelNotify(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set(common.RequestIdKey, "req-phone-gate")

	abortWithOpenAiMessageAndNotify(ctx, http.StatusForbidden, "绑定手机号后使用 API", "请先绑定并验证手机号后再使用 API。")

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.JSONEq(t, `{"error":{"message":"绑定手机号后使用 API (request id: req-phone-gate)","type":"new_api_error","code":""},"notify":"请先绑定并验证手机号后再使用 API。"}`, recorder.Body.String())
}

func TestPhoneVerificationReminderEligibleHonoursToggleAndRule(t *testing.T) {
	originalSMS := common.SMSVerificationEnabled
	originalToggle := operation_setting.GetPhoneVerificationSetting().APIReminderEnabled
	t.Cleanup(func() {
		common.SMSVerificationEnabled = originalSMS
		operation_setting.GetPhoneVerificationSetting().APIReminderEnabled = originalToggle
	})
	common.SMSVerificationEnabled = true
	start := model.PhoneVerificationRolloutStart().Unix()
	legacy := &model.UserBase{Group: "plg", CreatedAt: start - 1}

	operation_setting.GetPhoneVerificationSetting().APIReminderEnabled = false
	require.False(t, phoneVerificationReminderEligible(legacy))

	operation_setting.GetPhoneVerificationSetting().APIReminderEnabled = true
	require.True(t, phoneVerificationReminderEligible(legacy))
	require.False(t, phoneVerificationReminderEligible(&model.UserBase{Group: "plg", CreatedAt: start}))
	require.False(t, phoneVerificationReminderEligible(&model.UserBase{Group: "enterprise", CreatedAt: start - 1}))
	require.False(t, phoneVerificationReminderEligible(nil))
}
