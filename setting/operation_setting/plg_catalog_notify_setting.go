package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// PLGCatalogNotifySetting configures the DingTalk robot that announces
// changes to the plg "Available Models" catalog. It deliberately does not
// share the monitoring robot so catalog changes land in their own group.
type PLGCatalogNotifySetting struct {
	DingTalkAlertEnabled    bool    `json:"dingtalk_alert_enabled"`
	DingTalkAlertWebhookURL string  `json:"dingtalk_alert_webhook_url"`
	DingTalkAlertSecret     string  `json:"dingtalk_alert_secret"`
	CheckIntervalMinutes    float64 `json:"check_interval_minutes"`
}

const (
	DefaultPLGCatalogCheckIntervalMinutes = 5.0
	MinPLGCatalogCheckIntervalMinutes     = 1.0
)

var plgCatalogNotifySetting = PLGCatalogNotifySetting{
	DingTalkAlertEnabled:    false,
	DingTalkAlertWebhookURL: "",
	DingTalkAlertSecret:     "",
	CheckIntervalMinutes:    DefaultPLGCatalogCheckIntervalMinutes,
}

func init() {
	config.GlobalConfig.Register("plg_catalog_notify_setting", &plgCatalogNotifySetting)
}

func GetPLGCatalogNotifySetting() *PLGCatalogNotifySetting {
	return &plgCatalogNotifySetting
}

// EffectiveCheckIntervalMinutes clamps operator input so a zero or negative
// value cannot turn the watcher into a busy loop.
func (s *PLGCatalogNotifySetting) EffectiveCheckIntervalMinutes() float64 {
	if s == nil || s.CheckIntervalMinutes < MinPLGCatalogCheckIntervalMinutes {
		return MinPLGCatalogCheckIntervalMinutes
	}
	return s.CheckIntervalMinutes
}
