package console_setting

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const DefaultWelcomePromo = `[{"model_name":"deepseek-v4-pro","description":"DeepSeek reasoning model","offer":"55% off"},{"model_name":"glm-5.3-flash","description":"GLM multimodal model","offer":"55% off"},{"model_name":"gpt-5.6-sol","description":"GPT frontier model","offer":"55% off"}]`

type WelcomePromoModel struct {
	ModelName   string `json:"model_name"`
	Description string `json:"description"`
	Offer       string `json:"offer"`
}

type ConsoleSetting struct {
	ApiInfo              string `json:"api_info"`              // 控制台 API 信息 (JSON 数组字符串)
	UptimeKumaGroups     string `json:"uptime_kuma_groups"`    // Uptime Kuma 分组配置 (JSON 数组字符串)
	Announcements        string `json:"announcements"`         // 系统公告 (JSON 数组字符串)
	WelcomePromo         string `json:"welcome_promo"`         // 官网首屏弹窗右侧模型配置 (JSON 数组字符串)
	WelcomePromoEnabled  bool   `json:"welcome_promo_enabled"` // 是否展示官网首屏弹窗
	FAQ                  string `json:"faq"`                   // 常见问题 (JSON 数组字符串)
	ApiInfoEnabled       bool   `json:"api_info_enabled"`      // 是否启用 API 信息面板
	UptimeKumaEnabled    bool   `json:"uptime_kuma_enabled"`   // 是否启用 Uptime Kuma 面板
	AnnouncementsEnabled bool   `json:"announcements_enabled"` // 是否启用系统公告面板
	FAQEnabled           bool   `json:"faq_enabled"`           // 是否启用常见问答面板
}

// 默认配置
var defaultConsoleSetting = ConsoleSetting{
	ApiInfo:              "",
	UptimeKumaGroups:     "",
	Announcements:        "",
	WelcomePromo:         DefaultWelcomePromo,
	WelcomePromoEnabled:  true,
	FAQ:                  "",
	ApiInfoEnabled:       true,
	UptimeKumaEnabled:    true,
	AnnouncementsEnabled: true,
	FAQEnabled:           true,
}

// 全局实例
var consoleSetting = defaultConsoleSetting

func init() {
	// 注册到全局配置管理器，键名为 console_setting
	config.GlobalConfig.Register("console_setting", &consoleSetting)
}

// GetConsoleSetting 获取 ConsoleSetting 配置实例
func GetConsoleSetting() *ConsoleSetting {
	return &consoleSetting
}

func GetWelcomePromo() []WelcomePromoModel {
	raw := consoleSetting.WelcomePromo
	if raw == "" {
		raw = DefaultWelcomePromo
	}
	var models []WelcomePromoModel
	if err := common.Unmarshal([]byte(raw), &models); err != nil || len(models) == 0 {
		_ = common.Unmarshal([]byte(DefaultWelcomePromo), &models)
	}
	return models
}
