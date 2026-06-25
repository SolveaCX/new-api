package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllLangsHaveAllSteps(t *testing.T) {
	langs := []string{"en", "zh", "pt", "es", "ja"}
	for _, lang := range langs {
		for step := 1; step <= 4; step++ {
			tpl, ok := getEmailTemplate(lang, step)
			require.True(t, ok, "缺失 lang=%s step=%d", lang, step)
			require.NotEmpty(t, tpl.Subject, "lang=%s step=%d 主题为空", lang, step)
			require.NotEmpty(t, tpl.BodyHTML, "lang=%s step=%d 正文为空", lang, step)
		}
	}
}

func TestRenderEmailFallsBackToEn(t *testing.T) {
	// de 不支持 → 回退 en
	data := EmailRenderData{SystemName: "TestAPI", QuickstartLink: "http://x/quickstart"}
	subject, body, err := RenderEmail("de", 1, data)
	require.NoError(t, err)
	require.NotEmpty(t, subject)
	require.Contains(t, body, "TestAPI")
}

func TestRenderEmailSubjectUsesPlainTextEscaping(t *testing.T) {
	data := EmailRenderData{SystemName: "Foo & Bar", QuickstartLink: "http://x/quickstart"}
	subject, _, err := RenderEmail("en", 1, data)
	require.NoError(t, err)
	require.Contains(t, subject, "Foo & Bar")
	require.NotContains(t, subject, "&amp;")
}

func TestRenderEmailErrorsWhenFallbackStepMissing(t *testing.T) {
	data := EmailRenderData{SystemName: "TestAPI"}
	subject, body, err := RenderEmail("de", 99, data)
	require.Error(t, err)
	require.Empty(t, subject)
	require.Empty(t, body)
}

func TestRenderEmailInjectsBonus(t *testing.T) {
	data := EmailRenderData{SystemName: "TestAPI", BonusText: "Top up $50 get $30 free", TopupLink: "http://x/wallet"}
	_, body, err := RenderEmail("en", 3, data)
	require.NoError(t, err)
	require.Contains(t, body, "Top up $50 get $30 free")
}

func TestRenderEmailInjectsUnsubscribe(t *testing.T) {
	data := EmailRenderData{SystemName: "TestAPI", UnsubscribeURL: "http://x/api/email/unsubscribe?uid=1&token=abc"}
	// html/template 会把属性值里的 & 转义成 &amp;(合法 HTML,浏览器点击时还原为 &)。
	// 断言转义后的形式,确认退订链接确实注入到 href 中。
	const escaped = "http://x/api/email/unsubscribe?uid=1&amp;token=abc"
	for _, lang := range []string{"en", "zh", "pt", "es", "ja"} {
		for step := 1; step <= 4; step++ {
			_, body, err := RenderEmail(lang, step, data)
			require.NoError(t, err)
			require.Contains(t, body, escaped, "lang=%s step=%d 缺退订链接", lang, step)
		}
	}
}

// TestNoEnglishLeakInOtherLangs 防止把英文原文复制为其他语言(CLAUDE.md i18n 铁律)。
// 检查各非英语语言的主题不与英语主题完全相同。
func TestNoEnglishLeakInOtherLangs(t *testing.T) {
	for step := 1; step <= 4; step++ {
		en, _ := getEmailTemplate("en", step)
		for _, lang := range []string{"zh", "pt", "es", "ja"} {
			other, _ := getEmailTemplate(lang, step)
			require.NotEqual(t, en.Subject, other.Subject,
				"lang=%s step=%d 主题与英文相同,疑似漏翻", lang, step)
		}
	}
}
