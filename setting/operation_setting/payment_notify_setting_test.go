package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestPaymentNotifySettingDefaults(t *testing.T) {
	setting := GetPaymentNotifySetting()

	require.False(t, setting.DingTalkAlertEnabled)
	require.Empty(t, setting.DingTalkAlertWebhookURL)
	require.Empty(t, setting.DingTalkAlertSecret)
}

func TestPaymentNotifySettingLoadsDingTalkFieldsFromConfigMap(t *testing.T) {
	setting := &PaymentNotifySetting{}

	err := config.UpdateConfigFromMap(setting, map[string]string{
		"dingtalk_alert_enabled":     "true",
		"dingtalk_alert_webhook_url": "https://oapi.dingtalk.com/robot/send?access_token=abc",
		"dingtalk_alert_secret":      "secret",
	})

	require.NoError(t, err)
	require.True(t, setting.DingTalkAlertEnabled)
	require.Equal(t, "https://oapi.dingtalk.com/robot/send?access_token=abc", setting.DingTalkAlertWebhookURL)
	require.Equal(t, "secret", setting.DingTalkAlertSecret)
}
