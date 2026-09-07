package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIPhoneVerificationGateOnlyAppliesToPlgUsersAfterCutoff(t *testing.T) {
	cutoff := apiPhoneVerificationCutoff()
	verifiedAt := cutoff.Unix()

	tests := []struct {
		name     string
		user     *model.UserBase
		now      time.Time
		required bool
	}{
		{
			name:     "before cutoff",
			user:     &model.UserBase{Group: "plg"},
			now:      cutoff.Add(-time.Second),
			required: false,
		},
		{
			name:     "at cutoff",
			user:     &model.UserBase{Group: "plg"},
			now:      cutoff,
			required: true,
		},
		{
			name:     "verified plg user",
			user:     &model.UserBase{Group: "plg", PhoneNumber: "+14155550123", PhoneVerifiedAt: verifiedAt},
			now:      cutoff.Add(time.Hour),
			required: false,
		},
		{
			name:     "non plg user",
			user:     &model.UserBase{Group: "enterprise"},
			now:      cutoff.Add(time.Hour),
			required: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.required, apiPhoneVerificationRequired(tt.now, tt.user))
		})
	}
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
