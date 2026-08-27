package console_setting

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// announcementLegacyContentMaxBytes preserves the limit that applied to
	// the original scalar `content` field. New localized copies use rune limits
	// below so non-ASCII text is counted in a user-visible way.
	announcementLegacyContentMaxBytes = 500
	announcementContentMaxRunes       = 500
	announcementIntroMaxRunes         = 200
	announcementLinkLabelMaxRunes     = 100
	// Logos are included in every public status response, so keep each one
	// small enough that a configured announcement cannot inflate that payload.
	announcementLogoMaxBytes = 64 * 1024
)

// Keep this list in sync with website/src/lib/locales.ts. The console may
// configure any of the locales served by the public website, even though the
// authenticated console itself has a smaller translation set.
var announcementLocales = map[string]struct{}{
	"en": {}, "zh": {}, "es": {}, "fr": {}, "pt": {},
	"ru": {}, "ja": {}, "vi": {}, "de": {}, "id": {},
}

var announcementLogoMediaTypes = map[string]struct{}{
	"image/png":     {},
	"image/jpeg":    {},
	"image/webp":    {},
	"image/svg+xml": {},
	"image/gif":     {},
}

var announcementLogoPathRegex = regexp.MustCompile(`^/(?:assets/)?logos/[A-Za-z0-9._/-]+$`)

// announcementStringField reads an optional scalar field while distinguishing
// an absent field from an explicitly empty one. Explicit non-string values are
// rejected instead of silently disappearing from the public banner.
func announcementStringField(
	item map[string]interface{},
	field string,
	index int,
	label string,
) (string, bool, error) {
	raw, exists := item[field]
	if !exists {
		return "", false, nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", true, fmt.Errorf("第%d个%s字段必须是字符串", index, label)
	}
	return value, true, nil
}

// validateAnnouncementLocalizedField validates a locale -> copy object. It
// returns whether the field was present and whether it contained at least one
// non-empty copy; callers use those signals to implement fallback semantics.
func validateAnnouncementLocalizedField(
	item map[string]interface{},
	field string,
	maxRunes int,
	index int,
	label string,
) (present bool, nonEmpty bool, err error) {
	raw, exists := item[field]
	if !exists {
		return false, false, nil
	}

	values, err := announcementLocalizedMap(raw)
	if err != nil {
		return true, false, fmt.Errorf("第%d个公告的%s必须是语种到文案的对象", index, field)
	}

	for locale, value := range values {
		if _, ok := announcementLocales[locale]; !ok {
			return true, false, fmt.Errorf("第%d个公告的%s包含不支持的语种：%s", index, field, locale)
		}
		trimmed := strings.TrimSpace(value)
		if utf8.RuneCountInString(trimmed) > maxRunes {
			return true, false, fmt.Errorf("第%d个公告的%s（%s）长度不能超过%d字符", index, label, locale, maxRunes)
		}
		if trimmed == "" {
			continue
		}
		nonEmpty = true
		if err := checkDangerousContent(trimmed, index, label); err != nil {
			return true, false, err
		}
	}

	return true, nonEmpty, nil
}

func announcementLocalizedMap(value interface{}) (map[string]string, error) {
	switch values := value.(type) {
	case map[string]interface{}:
		result := make(map[string]string, len(values))
		for locale, raw := range values {
			text, ok := raw.(string)
			if !ok {
				return nil, fmt.Errorf("locale %s copy is not a string", locale)
			}
			result[locale] = text
		}
		return result, nil
	case map[string]string:
		// This branch is useful to callers/tests that construct the item in Go
		// rather than passing it through encoding/json first.
		result := make(map[string]string, len(values))
		for locale, text := range values {
			result[locale] = text
		}
		return result, nil
	default:
		return nil, fmt.Errorf("localized copy is not an object")
	}
}

func validateAnnouncementLink(link string, index int) error {
	trimmed := strings.TrimSpace(link)
	if trimmed == "" {
		return nil
	}
	if len(trimmed) > 500 {
		return fmt.Errorf("第%d个公告链接长度不能超过500字符", index)
	}
	if err := checkDangerousContent(trimmed, index, "公告链接"); err != nil {
		return err
	}

	if strings.HasPrefix(trimmed, "//") {
		return fmt.Errorf("第%d个公告链接不支持协议相对地址", index)
	}
	if strings.HasPrefix(trimmed, "/") {
		if _, err := url.Parse(trimmed); err != nil {
			return fmt.Errorf("第%d个公告链接无法解析：%s", index, err.Error())
		}
		return nil
	}
	return validateURL(trimmed, index, "公告链接")
}

func validateAnnouncementLogo(logo string, index int) error {
	trimmed := strings.TrimSpace(logo)
	if trimmed == "" {
		return nil
	}
	if len(trimmed) > announcementLogoMaxBytes {
		return fmt.Errorf("第%d个公告 Logo 不能超过64KB", index)
	}
	// Built-in model logos are served by the official website. Store only a
	// relative path from the two public logo directories; arbitrary remote
	// image URLs would create an uncontrolled third-party request on every page.
	if strings.HasPrefix(trimmed, "/") && !strings.HasPrefix(trimmed, "//") {
		if !strings.HasPrefix(trimmed, "/logos/") && !strings.HasPrefix(trimmed, "/assets/logos/") {
			return fmt.Errorf("第%d个公告 Logo 路径不受支持", index)
		}
		if strings.Contains(trimmed, "..") || strings.ContainsAny(trimmed, "?#") {
			return fmt.Errorf("第%d个公告 Logo 路径不安全", index)
		}
		if !announcementLogoPathRegex.MatchString(trimmed) {
			return fmt.Errorf("第%d个公告 Logo 路径格式不正确", index)
		}
		return nil
	}
	if !strings.HasPrefix(strings.ToLower(trimmed), "data:") {
		return fmt.Errorf("第%d个公告 Logo 必须是 data:image 图片", index)
	}

	headerAndPayload := trimmed[len("data:"):]
	header, payload, found := strings.Cut(headerAndPayload, ",")
	if !found || strings.TrimSpace(payload) == "" {
		return fmt.Errorf("第%d个公告 Logo 的 data URL 格式不正确", index)
	}
	parts := strings.Split(header, ";")
	if len(parts) == 0 {
		return fmt.Errorf("第%d个公告 Logo 的媒体类型不正确", index)
	}
	mediaType := strings.ToLower(strings.TrimSpace(parts[0]))
	if _, ok := announcementLogoMediaTypes[mediaType]; !ok {
		return fmt.Errorf("第%d个公告 Logo 只支持 PNG、JPEG、WebP、SVG、GIF 格式", index)
	}
	hasBase64Encoding := false
	for _, parameter := range parts[1:] {
		if strings.EqualFold(strings.TrimSpace(parameter), "base64") {
			hasBase64Encoding = true
			break
		}
	}
	if !hasBase64Encoding {
		return fmt.Errorf("第%d个公告 Logo 必须是 base64 编码的图片", index)
	}

	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return fmt.Errorf("第%d个公告 Logo 数据损坏，请重新上传", index)
	}
	// SVG is rendered as an image by the website, but reject obvious active
	// content as a defense in depth measure before it reaches a browser.
	if mediaType == "image/svg+xml" {
		if err := checkDangerousContent(string(decoded), index, "公告 Logo"); err != nil {
			return err
		}
	}
	return nil
}
