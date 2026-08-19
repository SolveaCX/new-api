package console_setting

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
)

func TestValidateOfficialWebsiteBannerHrefAcceptsSafeTargets(t *testing.T) {
	for _, href := range []string{
		"",
		"   ",
		"/campaigns/summer",
		"/models/seedance-api?ref=banner",
		"https://console.example.com/sign-up",
		"http://localhost:3000/promo",
	} {
		if err := ValidateOfficialWebsiteBannerHref(href); err != nil {
			t.Fatalf("expected href %q to be accepted, got %v", href, err)
		}
	}
}

func TestValidateOfficialWebsiteBannerHrefRejectsUnsafeTargets(t *testing.T) {
	for _, href := range []string{
		"javascript:alert(1)",
		"JavaScript:alert(1)",
		"data:text/html;base64,PHNjcmlwdD4=",
		"vbscript:msgbox(1)",
		"campaigns/summer",
		"//evil.example.com",
		"/campaigns/<script>alert(1)</script>",
		"/" + strings.Repeat("a", 600),
	} {
		if err := ValidateOfficialWebsiteBannerHref(href); err == nil {
			t.Fatalf("expected href %q to be rejected", href)
		}
	}
}

func TestValidateOfficialWebsiteBannerIconAcceptsSupportedImageDataURLs(t *testing.T) {
	for _, icon := range []string{
		"",
		"   ",
		"data:image/png;base64,iVBORw0KGgo=",
		"data:image/jpeg;base64,/9j/4AAQSkZJRg==",
		"data:image/webp;base64,UklGRg==",
		"data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=",
		"data:image/gif;base64,R0lGODlhAQABAA==",
	} {
		if err := ValidateOfficialWebsiteBannerIcon(icon); err != nil {
			t.Fatalf("expected icon %q to be accepted, got %v", icon, err)
		}
	}
}

func TestValidateOfficialWebsiteBannerIconRejectsUnsupportedPayloads(t *testing.T) {
	oversized := "data:image/png;base64," + strings.Repeat("A", officialWebsiteBannerIconMaxBytes+1)
	for _, icon := range []string{
		"https://cdn.example.com/icon.png",
		"data:text/html;base64,PHNjcmlwdD4=",
		"data:image/png,iVBORw0KGgo=",
		"data:image/svg+xml;utf8,<svg onload=alert(1)></svg>",
		"data:image/png;base64,not base64!!",
		oversized,
	} {
		if err := ValidateOfficialWebsiteBannerIcon(icon); err == nil {
			t.Fatalf("expected icon %q to be rejected", icon)
		}
	}
}

func TestValidateOfficialWebsiteBannerContentAcceptsLocaleMaps(t *testing.T) {
	for _, content := range []string{
		"",
		"   ",
		"{}",
		// 只填了空白字符等同于未配置，官网回退到内置默认横幅。
		`{"en":"   "}`,
		`{"en":"Launch credits are live."}`,
		`{"en":"Launch credits are live.","zh":"上线额度已开放。","ja":"クレジットを配布中。"}`,
	} {
		if err := ValidateOfficialWebsiteBannerContent(content); err != nil {
			t.Fatalf("expected content %q to be accepted, got %v", content, err)
		}
	}
}

func TestValidateOfficialWebsiteBannerContentRejectsInvalidMaps(t *testing.T) {
	tooLong := `{"en":"` + strings.Repeat("a", officialWebsiteBannerContentMaxRunes+1) + `"}`
	for _, content := range []string{
		"not json",
		`["Launch credits are live."]`,
		`{"en":42}`,
		`{"klingon":"nuqneH"}`,
		`{"zh":"上线额度已开放。"}`,
		`{"en":"   ","zh":"上线额度已开放。"}`,
		`{"en":"<script>alert(1)</script>"}`,
		tooLong,
	} {
		if err := ValidateOfficialWebsiteBannerContent(content); err == nil {
			t.Fatalf("expected content %q to be rejected", content)
		}
	}
}

func TestOfficialWebsiteBannerContentMapParsesConfiguredLocales(t *testing.T) {
	setting := GetConsoleSetting()
	original := setting.OfficialWebsiteBannerContent
	t.Cleanup(func() {
		setting.OfficialWebsiteBannerContent = original
	})

	setting.OfficialWebsiteBannerContent = `{"en":" Launch credits are live. ","zh":"上线额度已开放。","de":""}`
	got := GetOfficialWebsiteBannerContentMap()
	if len(got) != 2 {
		t.Fatalf("expected two configured locales, got %d (%v)", len(got), got)
	}
	if got["en"] != "Launch credits are live." {
		t.Fatalf("unexpected en copy: %q", got["en"])
	}
	if got["zh"] != "上线额度已开放。" {
		t.Fatalf("unexpected zh copy: %q", got["zh"])
	}
}

func TestOfficialWebsiteBannerContentMapIsEmptyForUnsetOrInvalidValues(t *testing.T) {
	setting := GetConsoleSetting()
	original := setting.OfficialWebsiteBannerContent
	t.Cleanup(func() {
		setting.OfficialWebsiteBannerContent = original
	})

	for _, content := range []string{"", "   ", "not json", "[]", `{"en":"   "}`} {
		setting.OfficialWebsiteBannerContent = content
		if got := GetOfficialWebsiteBannerContentMap(); len(got) != 0 {
			t.Fatalf("expected empty map for %q, got %v", content, got)
		}
	}
}

func TestOfficialWebsiteBannerKeysAreExportedToTheOptionMap(t *testing.T) {
	all := config.GlobalConfig.ExportAllConfigs()
	for _, key := range []string{
		"console_setting.official_website_banner_enabled",
		"console_setting.official_website_banner_content",
		"console_setting.official_website_banner_href",
		"console_setting.official_website_banner_icon",
	} {
		if _, ok := all[key]; !ok {
			t.Fatalf("expected %s to be exported to the option map", key)
		}
	}
	if all["console_setting.official_website_banner_enabled"] != "true" {
		t.Fatalf("expected the banner to default to enabled, got %q", all["console_setting.official_website_banner_enabled"])
	}
}
