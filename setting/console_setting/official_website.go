package console_setting

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

const (
	// officialWebsiteBannerContentMaxRunes 限制单条横幅文案长度，避免顶部横幅在窄屏被撑破。
	officialWebsiteBannerContentMaxRunes = 300
	// officialWebsiteBannerHrefMaxLength 与其它控制台链接配置保持一致。
	officialWebsiteBannerHrefMaxLength = 500
	// officialWebsiteBannerIconMaxBytes 限制上传图标体积（base64 编码后的字节数）。
	// 图标存在 option 表里并随每次 /api/status 下发，必须保持很小。
	officialWebsiteBannerIconMaxBytes = 64 * 1024
)

// officialWebsiteLocales 与官网 website/src/lib/locales.ts 的 LOCALES 保持一致。
// 新增官网语种时两边都要更新。
var officialWebsiteLocales = map[string]bool{
	"en": true, "zh": true, "es": true, "fr": true, "pt": true,
	"ru": true, "ja": true, "vi": true, "de": true, "id": true,
}

// officialWebsiteBannerIconMediaTypes 是允许上传的图标类型。
// 只接受 base64 编码的 data URL，不接受远程 URL——远程图会给官网引入
// 一个不受控的第三方请求。
var officialWebsiteBannerIconMediaTypes = map[string]bool{
	"image/png":     true,
	"image/jpeg":    true,
	"image/webp":    true,
	"image/svg+xml": true,
	"image/gif":     true,
}

// ValidateOfficialWebsiteBannerHref 校验官网横幅跳转链接。
// 允许留空、站内绝对路径（`/` 开头，但不能是 `//` 协议相对地址），或 http/https 链接。
func ValidateOfficialWebsiteBannerHref(href string) error {
	trimmed := strings.TrimSpace(href)
	if trimmed == "" {
		return nil
	}
	if len(trimmed) > officialWebsiteBannerHrefMaxLength {
		return fmt.Errorf("官网横幅链接长度不能超过%d字符", officialWebsiteBannerHrefMaxLength)
	}
	if err := checkDangerousContent(trimmed, 1, "官网横幅链接"); err != nil {
		return err
	}

	if strings.HasPrefix(trimmed, "//") {
		return fmt.Errorf("官网横幅链接不支持协议相对地址，请填写完整的 http/https 链接")
	}
	if strings.HasPrefix(trimmed, "/") {
		if _, err := url.Parse(trimmed); err != nil {
			return fmt.Errorf("官网横幅链接无法解析：%s", err.Error())
		}
		return nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("官网横幅链接无法解析：%s", err.Error())
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("官网横幅链接只支持站内路径（以 / 开头）或 http/https 链接")
	}
	if parsed.Host == "" {
		return fmt.Errorf("官网横幅链接缺少域名")
	}
	return nil
}

// ValidateOfficialWebsiteBannerIcon 校验上传的横幅图标。
// 允许留空，否则必须是受支持图片类型的 base64 data URL。
func ValidateOfficialWebsiteBannerIcon(icon string) error {
	trimmed := strings.TrimSpace(icon)
	if trimmed == "" {
		return nil
	}
	if len(trimmed) > officialWebsiteBannerIconMaxBytes {
		return fmt.Errorf("官网横幅图标不能超过%dKB，请压缩后重新上传", officialWebsiteBannerIconMaxBytes/1024)
	}
	if !strings.HasPrefix(trimmed, "data:") {
		return fmt.Errorf("官网横幅图标必须是上传的图片")
	}

	header, payload, found := strings.Cut(strings.TrimPrefix(trimmed, "data:"), ",")
	if !found {
		return fmt.Errorf("官网横幅图标格式不正确")
	}
	mediaType, encoding, hasEncoding := strings.Cut(header, ";")
	if !hasEncoding || !strings.EqualFold(strings.TrimSpace(encoding), "base64") {
		return fmt.Errorf("官网横幅图标必须是 base64 编码的图片")
	}
	if !officialWebsiteBannerIconMediaTypes[strings.ToLower(strings.TrimSpace(mediaType))] {
		return fmt.Errorf("官网横幅图标只支持 PNG、JPEG、WebP、SVG、GIF 格式")
	}
	if _, err := base64.StdEncoding.DecodeString(payload); err != nil {
		return fmt.Errorf("官网横幅图标数据损坏，请重新上传")
	}
	return nil
}

// ValidateOfficialWebsiteBannerContent 校验按语种配置的横幅文案。
// 允许留空（官网回退到内置默认横幅）；一旦配置了任意语种，就必须包含英文兜底文案。
func ValidateOfficialWebsiteBannerContent(content string) error {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}

	copies, err := parseOfficialWebsiteBannerContent(trimmed)
	if err != nil {
		return err
	}

	nonEmpty := 0
	for locale, text := range copies {
		if !officialWebsiteLocales[locale] {
			return fmt.Errorf("官网横幅文案包含不支持的语种：%s", locale)
		}
		value := strings.TrimSpace(text)
		if value == "" {
			continue
		}
		nonEmpty++
		if utf8.RuneCountInString(value) > officialWebsiteBannerContentMaxRunes {
			return fmt.Errorf("%s 语种的官网横幅文案长度不能超过%d字符", locale, officialWebsiteBannerContentMaxRunes)
		}
		if err := checkDangerousContent(value, 1, "官网横幅文案"); err != nil {
			return err
		}
	}

	if nonEmpty > 0 && strings.TrimSpace(copies["en"]) == "" {
		return fmt.Errorf("官网横幅文案必须填写英文（en），其它语种缺失时会回退到英文")
	}
	return nil
}

// GetOfficialWebsiteBannerContentMap 返回已配置的横幅文案（语种 -> 文案）。
// 值非法或未配置时返回空 map，调用方据此回退到官网内置默认横幅。
func GetOfficialWebsiteBannerContentMap() map[string]string {
	copies, err := parseOfficialWebsiteBannerContent(GetConsoleSetting().OfficialWebsiteBannerContent)
	if err != nil {
		return map[string]string{}
	}

	result := make(map[string]string, len(copies))
	for locale, text := range copies {
		if !officialWebsiteLocales[locale] {
			continue
		}
		value := strings.TrimSpace(text)
		if value == "" {
			continue
		}
		result[locale] = value
	}
	return result
}

func parseOfficialWebsiteBannerContent(content string) (map[string]string, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return map[string]string{}, nil
	}
	var copies map[string]string
	if err := common.Unmarshal([]byte(trimmed), &copies); err != nil {
		return nil, fmt.Errorf("官网横幅文案格式错误，应为 语种->文案 的 JSON 对象：%s", err.Error())
	}
	if copies == nil {
		return map[string]string{}, nil
	}
	return copies, nil
}
