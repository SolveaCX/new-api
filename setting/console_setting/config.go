package console_setting

import "github.com/QuantumNous/new-api/setting/config"

type ConsoleSetting struct {
	ApiInfo              string `json:"api_info"`              // 控制台 API 信息 (JSON 数组字符串)
	UptimeKumaGroups     string `json:"uptime_kuma_groups"`    // Uptime Kuma 分组配置 (JSON 数组字符串)
	Announcements        string `json:"announcements"`         // 系统公告 (JSON 数组字符串)
	FAQ                  string `json:"faq"`                   // 常见问题 (JSON 数组字符串)
	ApiInfoEnabled       bool   `json:"api_info_enabled"`      // 是否启用 API 信息面板
	UptimeKumaEnabled    bool   `json:"uptime_kuma_enabled"`   // 是否启用 Uptime Kuma 面板
	AnnouncementsEnabled bool   `json:"announcements_enabled"` // 是否启用系统公告面板
	FAQEnabled           bool   `json:"faq_enabled"`           // 是否启用常见问答面板

	// 官网（website/ 独立 Next 应用）全局横幅配置。这些值通过 /api/status 暴露给官网，
	// 官网侧再按访问语种取文案；内容为空时官网回退到内置的默认横幅。
	OfficialWebsiteBannerEnabled bool   `json:"official_website_banner_enabled"` // 是否展示官网全局横幅
	OfficialWebsiteBannerContent string `json:"official_website_banner_content"` // 横幅文案 (语种 -> 文案 的 JSON 对象字符串)
	OfficialWebsiteBannerHref    string `json:"official_website_banner_href"`    // 横幅跳转链接 (站内绝对路径或 http/https 链接)
	OfficialWebsiteBannerIcon    string `json:"official_website_banner_icon"`    // 横幅图标 (上传图片生成的 data URL)
}

// 默认配置
var defaultConsoleSetting = ConsoleSetting{
	ApiInfo:              "",
	UptimeKumaGroups:     "",
	Announcements:        "",
	FAQ:                  "",
	ApiInfoEnabled:       true,
	UptimeKumaEnabled:    true,
	AnnouncementsEnabled: true,
	FAQEnabled:           true,

	OfficialWebsiteBannerEnabled: true,
	OfficialWebsiteBannerContent: "",
	OfficialWebsiteBannerHref:    "",
	OfficialWebsiteBannerIcon:    "",
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
