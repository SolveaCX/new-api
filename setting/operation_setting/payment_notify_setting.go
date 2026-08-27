package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type PaymentNotifySetting struct {
	DingTalkAlertEnabled    bool   `json:"dingtalk_alert_enabled"`
	DingTalkAlertWebhookURL string `json:"dingtalk_alert_webhook_url"`
	DingTalkAlertSecret     string `json:"dingtalk_alert_secret"`
}

var paymentNotifySetting = PaymentNotifySetting{
	DingTalkAlertEnabled:    false,
	DingTalkAlertWebhookURL: "",
	DingTalkAlertSecret:     "",
}

func init() {
	config.GlobalConfig.Register("payment_notify_setting", &paymentNotifySetting)
}

func GetPaymentNotifySetting() *PaymentNotifySetting {
	return &paymentNotifySetting
}
