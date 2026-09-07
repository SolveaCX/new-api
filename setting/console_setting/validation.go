package console_setting

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

var (
	urlRegex       = regexp.MustCompile(`^https?://(?:(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?|(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?))(?:\:[0-9]{1,5})?(?:/.*)?$`)
	dangerousChars = []string{"<script", "<iframe", "javascript:", "onload=", "onerror=", "onclick="}
	validColors    = map[string]bool{
		"blue": true, "green": true, "cyan": true, "purple": true, "pink": true,
		"red": true, "orange": true, "amber": true, "yellow": true, "lime": true,
		"light-green": true, "teal": true, "light-blue": true, "indigo": true,
		"violet": true, "grey": true, "slate": true,
	}
	slugRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

func parseJSONArray(jsonStr string, typeName string) ([]map[string]interface{}, error) {
	var list []map[string]interface{}
	if err := common.Unmarshal([]byte(jsonStr), &list); err != nil {
		return nil, fmt.Errorf("%s格式错误：%s", typeName, err.Error())
	}
	return list, nil
}

func validateURL(urlStr string, index int, itemType string) error {
	if !urlRegex.MatchString(urlStr) {
		return fmt.Errorf("第%d个%s的URL格式不正确", index, itemType)
	}
	if _, err := url.Parse(urlStr); err != nil {
		return fmt.Errorf("第%d个%s的URL无法解析：%s", index, itemType, err.Error())
	}
	return nil
}

func checkDangerousContent(content string, index int, itemType string) error {
	lower := strings.ToLower(content)
	for _, d := range dangerousChars {
		if strings.Contains(lower, d) {
			return fmt.Errorf("第%d个%s包含不允许的内容", index, itemType)
		}
	}
	return nil
}

func getJSONList(jsonStr string) []map[string]interface{} {
	if jsonStr == "" {
		return []map[string]interface{}{}
	}
	var list []map[string]interface{}
	_ = common.Unmarshal([]byte(jsonStr), &list)
	return list
}

func ValidateConsoleSettings(settingsStr string, settingType string) error {
	if settingsStr == "" {
		return nil
	}

	switch settingType {
	case "ApiInfo":
		return validateApiInfo(settingsStr)
	case "Announcements":
		return validateAnnouncements(settingsStr)
	case "WelcomePromo":
		return validateWelcomePromo(settingsStr)
	case "FAQ":
		return validateFAQ(settingsStr)
	case "UptimeKumaGroups":
		return validateUptimeKumaGroups(settingsStr)
	default:
		return fmt.Errorf("未知的设置类型：%s", settingType)
	}
}

func validateWelcomePromo(welcomePromoStr string) error {
	var models []map[string]interface{}
	if err := common.Unmarshal([]byte(welcomePromoStr), &models); err != nil {
		return fmt.Errorf("首屏弹窗模型配置格式错误：%s", err.Error())
	}
	if len(models) != 3 {
		return fmt.Errorf("首屏弹窗模型配置必须包含3个模型")
	}
	seen := make(map[string]struct{}, len(models))
	for i, item := range models {
		modelName, ok := item["model_name"].(string)
		modelName = strings.TrimSpace(modelName)
		if !ok || modelName == "" {
			return fmt.Errorf("第%d个首屏弹窗模型缺少模型名称", i+1)
		}
		if _, exists := seen[modelName]; exists {
			return fmt.Errorf("第%d个首屏弹窗模型与其他模型重复", i+1)
		}
		seen[modelName] = struct{}{}
		description, ok := item["description"].(string)
		if !ok || strings.TrimSpace(description) == "" {
			return fmt.Errorf("第%d个首屏弹窗模型缺少描述", i+1)
		}
		if utf8.RuneCountInString(strings.TrimSpace(description)) > 200 {
			return fmt.Errorf("第%d个首屏弹窗模型描述不能超过200字符", i+1)
		}
		offer, ok := item["offer"].(string)
		if !ok || strings.TrimSpace(offer) == "" {
			return fmt.Errorf("第%d个首屏弹窗模型缺少优惠文案", i+1)
		}
		if utf8.RuneCountInString(strings.TrimSpace(offer)) > 100 {
			return fmt.Errorf("第%d个首屏弹窗模型优惠文案不能超过100字符", i+1)
		}
		if err := checkDangerousContent(description, i+1, "首屏弹窗模型描述"); err != nil {
			return err
		}
		if err := checkDangerousContent(offer, i+1, "首屏弹窗模型优惠文案"); err != nil {
			return err
		}
	}
	return nil
}

func validateApiInfo(apiInfoStr string) error {
	apiInfoList, err := parseJSONArray(apiInfoStr, "API信息")
	if err != nil {
		return err
	}

	if len(apiInfoList) > 50 {
		return fmt.Errorf("API信息数量不能超过50个")
	}

	for i, apiInfo := range apiInfoList {
		urlStr, ok := apiInfo["url"].(string)
		if !ok || urlStr == "" {
			return fmt.Errorf("第%d个API信息缺少URL字段", i+1)
		}
		route, ok := apiInfo["route"].(string)
		if !ok || route == "" {
			return fmt.Errorf("第%d个API信息缺少线路描述字段", i+1)
		}
		description, ok := apiInfo["description"].(string)
		if !ok || description == "" {
			return fmt.Errorf("第%d个API信息缺少说明字段", i+1)
		}
		color, ok := apiInfo["color"].(string)
		if !ok || color == "" {
			return fmt.Errorf("第%d个API信息缺少颜色字段", i+1)
		}

		if err := validateURL(urlStr, i+1, "API信息"); err != nil {
			return err
		}

		if len(urlStr) > 500 {
			return fmt.Errorf("第%d个API信息的URL长度不能超过500字符", i+1)
		}
		if len(route) > 100 {
			return fmt.Errorf("第%d个API信息的线路描述长度不能超过100字符", i+1)
		}
		if len(description) > 200 {
			return fmt.Errorf("第%d个API信息的说明长度不能超过200字符", i+1)
		}

		if !validColors[color] {
			return fmt.Errorf("第%d个API信息的颜色值不合法", i+1)
		}

		if err := checkDangerousContent(description, i+1, "API信息"); err != nil {
			return err
		}
		if err := checkDangerousContent(route, i+1, "API信息"); err != nil {
			return err
		}
	}
	return nil
}

func GetApiInfo() []map[string]interface{} {
	return getJSONList(GetConsoleSetting().ApiInfo)
}

func validateAnnouncements(announcementsStr string) error {
	list, err := parseJSONArray(announcementsStr, "系统公告")
	if err != nil {
		return err
	}
	if len(list) > 100 {
		return fmt.Errorf("系统公告数量不能超过100个")
	}
	validTypes := map[string]bool{
		"default": true, "ongoing": true, "success": true, "warning": true, "error": true,
	}
	for i, ann := range list {
		content, contentPresent, err := announcementStringField(ann, "content", i+1, "公告内容")
		if err != nil {
			return err
		}
		contentMapPresent, contentMapNonEmpty, err := validateAnnouncementLocalizedField(
			ann,
			"content_i18n",
			announcementContentMaxRunes,
			i+1,
			"公告内容",
		)
		if err != nil {
			return err
		}
		for _, field := range []string{"contentI18n", "contentByLocale"} {
			aliasPresent, aliasNonEmpty, err := validateAnnouncementLocalizedField(
				ann,
				field,
				announcementContentMaxRunes,
				i+1,
				"公告内容",
			)
			if err != nil {
				return err
			}
			contentMapPresent = contentMapPresent || aliasPresent
			contentMapNonEmpty = contentMapNonEmpty || aliasNonEmpty
		}
		if !contentPresent || strings.TrimSpace(content) == "" {
			if !contentMapPresent || !contentMapNonEmpty {
				return fmt.Errorf("第%d个公告缺少内容字段", i+1)
			}
		}
		publishDateAny, exists := ann["publishDate"]
		if !exists {
			return fmt.Errorf("第%d个公告缺少发布日期字段", i+1)
		}
		publishDateStr, ok := publishDateAny.(string)
		if !ok || publishDateStr == "" {
			return fmt.Errorf("第%d个公告的发布日期不能为空", i+1)
		}
		if _, err := time.Parse(time.RFC3339, publishDateStr); err != nil {
			return fmt.Errorf("第%d个公告的发布日期格式错误", i+1)
		}
		if t, exists := ann["type"]; exists {
			if typeStr, ok := t.(string); ok {
				if !validTypes[typeStr] {
					return fmt.Errorf("第%d个公告的类型值不合法", i+1)
				}
			}
		}
		// Keep the legacy scalar limit for backwards compatibility. The
		// localized field above uses a rune limit so CJK text is counted as
		// users perceive it rather than by its UTF-8 byte length.
		// The scalar is a compatibility fallback emitted alongside the
		// localized map by the console. Once at least one localized copy is
		// present, enforce the rune limit above and do not reject a UTF-8
		// fallback merely because its byte length exceeds the old scalar limit.
		if contentPresent && !contentMapNonEmpty && len(content) > announcementLegacyContentMaxBytes {
			return fmt.Errorf("第%d个公告的内容长度不能超过500字符", i+1)
		}

		for _, field := range []string{
			"intro_i18n",
			"introI18n",
			"summary_i18n",
			"summaryI18n",
			"extra_i18n",
			"extraI18n",
		} {
			if _, _, err := validateAnnouncementLocalizedField(
				ann,
				field,
				announcementIntroMaxRunes,
				i+1,
				"公告简介",
			); err != nil {
				return err
			}
		}
		for _, field := range []string{"intro", "extra", "summary"} {
			value, present, err := announcementStringField(ann, field, i+1, "公告简介")
			if err != nil {
				return err
			}
			if present {
				if utf8.RuneCountInString(strings.TrimSpace(value)) > announcementIntroMaxRunes {
					return fmt.Errorf("第%d个公告的说明长度不能超过200字符", i+1)
				}
				if err := checkDangerousContent(value, i+1, "公告简介"); err != nil {
					return err
				}
			}
		}

		for _, field := range []string{
			"link_label_i18n",
			"linkLabelI18n",
			"link_text_i18n",
			"linkTextI18n",
		} {
			if _, _, err := validateAnnouncementLocalizedField(
				ann,
				field,
				announcementLinkLabelMaxRunes,
				i+1,
				"公告链接文案",
			); err != nil {
				return err
			}
		}
		for _, field := range []string{"link_label", "linkLabel", "link_text", "linkText"} {
			value, present, err := announcementStringField(ann, field, i+1, "公告链接文案")
			if err != nil {
				return err
			}
			if present {
				if utf8.RuneCountInString(strings.TrimSpace(value)) > announcementLinkLabelMaxRunes {
					return fmt.Errorf("第%d个公告的链接文案长度不能超过100字符", i+1)
				}
				if err := checkDangerousContent(value, i+1, "公告链接文案"); err != nil {
					return err
				}
			}
		}

		for _, field := range []string{"link", "href", "url"} {
			link, present, err := announcementStringField(ann, field, i+1, "公告链接")
			if err != nil {
				return err
			}
			if present {
				if err := validateAnnouncementLink(link, i+1); err != nil {
					return err
				}
			}
		}

		for _, field := range []string{"logo", "icon", "logo_url", "icon_url"} {
			logo, present, err := announcementStringField(ann, field, i+1, "公告 Logo")
			if err != nil {
				return err
			}
			if present {
				if err := validateAnnouncementLogo(logo, i+1); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validateFAQ(faqStr string) error {
	list, err := parseJSONArray(faqStr, "FAQ信息")
	if err != nil {
		return err
	}
	if len(list) > 100 {
		return fmt.Errorf("FAQ数量不能超过100个")
	}
	for i, faq := range list {
		question, ok := faq["question"].(string)
		if !ok || question == "" {
			return fmt.Errorf("第%d个FAQ缺少问题字段", i+1)
		}
		answer, ok := faq["answer"].(string)
		if !ok || answer == "" {
			return fmt.Errorf("第%d个FAQ缺少答案字段", i+1)
		}
		if len(question) > 200 {
			return fmt.Errorf("第%d个FAQ的问题长度不能超过200字符", i+1)
		}
		if len(answer) > 1000 {
			return fmt.Errorf("第%d个FAQ的答案长度不能超过1000字符", i+1)
		}
	}
	return nil
}

func getPublishTime(item map[string]interface{}) time.Time {
	if v, ok := item["publishDate"]; ok {
		if s, ok2 := v.(string); ok2 {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

func GetAnnouncements() []map[string]interface{} {
	list := getJSONList(GetConsoleSetting().Announcements)
	sort.SliceStable(list, func(i, j int) bool {
		return getPublishTime(list[i]).After(getPublishTime(list[j]))
	})
	return list
}

func GetFAQ() []map[string]interface{} {
	return getJSONList(GetConsoleSetting().FAQ)
}

func validateUptimeKumaGroups(groupsStr string) error {
	groups, err := parseJSONArray(groupsStr, "Uptime Kuma分组配置")
	if err != nil {
		return err
	}

	if len(groups) > 20 {
		return fmt.Errorf("Uptime Kuma分组数量不能超过20个")
	}

	nameSet := make(map[string]bool)

	for i, group := range groups {
		categoryName, ok := group["categoryName"].(string)
		if !ok || categoryName == "" {
			return fmt.Errorf("第%d个分组缺少分类名称字段", i+1)
		}
		if nameSet[categoryName] {
			return fmt.Errorf("第%d个分组的分类名称与其他分组重复", i+1)
		}
		nameSet[categoryName] = true
		urlStr, ok := group["url"].(string)
		if !ok || urlStr == "" {
			return fmt.Errorf("第%d个分组缺少URL字段", i+1)
		}
		slug, ok := group["slug"].(string)
		if !ok || slug == "" {
			return fmt.Errorf("第%d个分组缺少Slug字段", i+1)
		}
		description, ok := group["description"].(string)
		if !ok {
			description = ""
		}

		if err := validateURL(urlStr, i+1, "分组"); err != nil {
			return err
		}

		if len(categoryName) > 50 {
			return fmt.Errorf("第%d个分组的分类名称长度不能超过50字符", i+1)
		}
		if len(urlStr) > 500 {
			return fmt.Errorf("第%d个分组的URL长度不能超过500字符", i+1)
		}
		if len(slug) > 100 {
			return fmt.Errorf("第%d个分组的Slug长度不能超过100字符", i+1)
		}
		if len(description) > 200 {
			return fmt.Errorf("第%d个分组的描述长度不能超过200字符", i+1)
		}

		if !slugRegex.MatchString(slug) {
			return fmt.Errorf("第%d个分组的Slug只能包含字母、数字、下划线和连字符", i+1)
		}

		if err := checkDangerousContent(description, i+1, "分组"); err != nil {
			return err
		}
		if err := checkDangerousContent(categoryName, i+1, "分组"); err != nil {
			return err
		}
	}
	return nil
}

func GetUptimeKumaGroups() []map[string]interface{} {
	return getJSONList(GetConsoleSetting().UptimeKumaGroups)
}
