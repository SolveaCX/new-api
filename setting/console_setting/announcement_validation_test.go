package console_setting

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

const testAnnouncementPublishDate = "2026-08-27T00:00:00Z"

func marshalTestAnnouncements(t *testing.T, announcements []map[string]interface{}) string {
	t.Helper()
	data, err := common.Marshal(announcements)
	if err != nil {
		t.Fatalf("marshal test announcements: %v", err)
	}
	return string(data)
}

func TestValidateAnnouncementsAcceptsLegacyAndLocalizedFields(t *testing.T) {
	logo := "data:image/png;base64,QUJD"
	tests := []struct {
		name string
		item map[string]interface{}
	}{
		{
			name: "legacy scalar fields",
			item: map[string]interface{}{
				"content":     "Legacy announcement",
				"publishDate": testAnnouncementPublishDate,
				"type":        "default",
				"extra":       "Legacy summary",
				"link":        "/pricing",
			},
		},
		{
			name: "canonical localized fields",
			item: map[string]interface{}{
				"content_i18n": map[string]string{
					"en": "Localized announcement",
					"zh": "多语言公告",
				},
				"intro_i18n": map[string]string{
					"en": "A short summary",
					"ja": "短い概要",
				},
				"link_label_i18n": map[string]string{
					"en": "Learn more",
					"zh": "了解更多",
				},
				"logo":        logo,
				"publishDate": testAnnouncementPublishDate,
			},
		},
		{
			name: "localized content without english copy",
			item: map[string]interface{}{
				"content_i18n": map[string]string{"zh": "仅中文公告"},
				"publishDate":  testAnnouncementPublishDate,
			},
		},
		{
			name: "localized content alias without scalar fallback",
			item: map[string]interface{}{
				"contentI18n": map[string]string{"fr": "Annonce française"},
				"publishDate": testAnnouncementPublishDate,
			},
		},
		{
			name: "localized fields with legacy aliases",
			item: map[string]interface{}{
				"content":        "Legacy fallback",
				"content_i18n":   map[string]string{"de": "Deutsche Meldung"},
				"summary_i18n":   map[string]string{"en": "Summary"},
				"extra_i18n":     map[string]string{"fr": "Résumé"},
				"link_text_i18n": map[string]string{"pt": "Saiba mais"},
				"icon":           logo,
				"href":           "https://example.com/announcement",
				"publishDate":    testAnnouncementPublishDate,
			},
		},
		{
			name: "blank localized map with legacy content",
			item: map[string]interface{}{
				"content":         "Still visible",
				"content_i18n":    map[string]string{"en": "   ", "zh": ""},
				"intro_i18n":      map[string]string{"en": ""},
				"link_label_i18n": map[string]string{"en": ""},
				"publishDate":     testAnnouncementPublishDate,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := marshalTestAnnouncements(t, []map[string]interface{}{tt.item})
			if err := ValidateConsoleSettings(settings, "Announcements"); err != nil {
				t.Fatalf("expected announcement to be accepted, got %v", err)
			}
		})
	}
}

func TestValidateAnnouncementsRejectsInvalidLocalizedFields(t *testing.T) {
	tests := []struct {
		name string
		item map[string]interface{}
	}{
		{
			name: "missing all content",
			item: map[string]interface{}{
				"publishDate": testAnnouncementPublishDate,
			},
		},
		{
			name: "empty localized content",
			item: map[string]interface{}{
				"content_i18n": map[string]string{"en": "   "},
				"publishDate":  testAnnouncementPublishDate,
			},
		},
		{
			name: "unsupported locale",
			item: map[string]interface{}{
				"content_i18n": map[string]string{"klingon": "nuqneH"},
				"publishDate":  testAnnouncementPublishDate,
			},
		},
		{
			name: "localized copy is not a string",
			item: map[string]interface{}{
				"content_i18n": map[string]interface{}{"en": 42},
				"publishDate":  testAnnouncementPublishDate,
			},
		},
		{
			name: "localized field is not an object",
			item: map[string]interface{}{
				"content_i18n": "not an object",
				"publishDate":  testAnnouncementPublishDate,
			},
		},
		{
			name: "legacy content is not a string",
			item: map[string]interface{}{
				"content":     42,
				"publishDate": testAnnouncementPublishDate,
			},
		},
		{
			name: "intro is not a string",
			item: map[string]interface{}{
				"content":     "Announcement",
				"intro":       42,
				"publishDate": testAnnouncementPublishDate,
			},
		},
		{
			name: "link label is not a string",
			item: map[string]interface{}{
				"content":     "Announcement",
				"link_label":  42,
				"publishDate": testAnnouncementPublishDate,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := marshalTestAnnouncements(t, []map[string]interface{}{tt.item})
			if err := ValidateConsoleSettings(settings, "Announcements"); err == nil {
				t.Fatal("expected invalid localized announcement to be rejected")
			}
		})
	}
}

func TestValidateAnnouncementsEnforcesLocalizedCopyLengths(t *testing.T) {
	tests := []struct {
		name string
		item map[string]interface{}
	}{
		{
			name: "content",
			item: map[string]interface{}{
				"content_i18n": map[string]string{
					"en": strings.Repeat("中", announcementContentMaxRunes+1),
				},
				"publishDate": testAnnouncementPublishDate,
			},
		},
		{
			name: "intro",
			item: map[string]interface{}{
				"content":     "Announcement",
				"intro_i18n":  map[string]string{"en": strings.Repeat("i", announcementIntroMaxRunes+1)},
				"publishDate": testAnnouncementPublishDate,
			},
		},
		{
			name: "link label",
			item: map[string]interface{}{
				"content":         "Announcement",
				"link_label_i18n": map[string]string{"en": strings.Repeat("l", announcementLinkLabelMaxRunes+1)},
				"publishDate":     testAnnouncementPublishDate,
			},
		},
		{
			name: "legacy content byte limit",
			item: map[string]interface{}{
				"content":     strings.Repeat("x", announcementLegacyContentMaxBytes+1),
				"publishDate": testAnnouncementPublishDate,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := marshalTestAnnouncements(t, []map[string]interface{}{tt.item})
			if err := ValidateConsoleSettings(settings, "Announcements"); err == nil {
				t.Fatal("expected overlong announcement copy to be rejected")
			}
		})
	}
}

func TestValidateAnnouncementsAllowsUtf8LocalizedFallbackBeyondLegacyByteLimit(t *testing.T) {
	localized := strings.Repeat("中", announcementContentMaxRunes)
	settings := marshalTestAnnouncements(t, []map[string]interface{}{{
		// The console keeps this scalar for old readers, but it can be longer
		// than 500 bytes when the canonical localized copy is CJK text.
		"content":      localized,
		"content_i18n": map[string]string{"zh": localized},
		"publishDate":  testAnnouncementPublishDate,
	}})
	if err := ValidateConsoleSettings(settings, "Announcements"); err != nil {
		t.Fatalf("expected localized UTF-8 fallback to be accepted, got %v", err)
	}
}

func TestValidateAnnouncementsRejectsUnsafeLinks(t *testing.T) {
	unsafeLinks := []string{
		"javascript:alert(1)",
		"JavaScript:alert(1)",
		"data:text/html;base64,PHNjcmlwdD4=",
		"//evil.example.com/redirect",
		"campaigns/summer",
		"/campaigns/<script>alert(1)</script>",
		"https://example.com/" + strings.Repeat("a", 500),
	}

	for _, link := range unsafeLinks {
		t.Run(link, func(t *testing.T) {
			settings := marshalTestAnnouncements(t, []map[string]interface{}{{
				"content":     "Announcement",
				"link":        link,
				"publishDate": testAnnouncementPublishDate,
			}})
			if err := ValidateConsoleSettings(settings, "Announcements"); err == nil {
				t.Fatalf("expected unsafe link %q to be rejected", link)
			}
		})
	}
}

func TestValidateAnnouncementsAcceptsSafeLinks(t *testing.T) {
	for _, link := range []string{
		"",
		"   ",
		"/pricing",
		"/models/seedance?ref=banner",
		"https://example.com/announcement",
		"http://localhost:3000/promo",
	} {
		t.Run(fmt.Sprintf("link_%q", link), func(t *testing.T) {
			settings := marshalTestAnnouncements(t, []map[string]interface{}{{
				"content":     "Announcement",
				"link":        link,
				"publishDate": testAnnouncementPublishDate,
			}})
			if err := ValidateConsoleSettings(settings, "Announcements"); err != nil {
				t.Fatalf("expected safe link %q to be accepted, got %v", link, err)
			}
		})
	}
}

func TestValidateAnnouncementsLogoDataURLs(t *testing.T) {
	for _, logo := range []string{
		"",
		"   ",
		"/assets/logos/deepseek.svg",
		"/logos/baidu.svg",
		"data:image/png;base64,QUJD",
		"data:image/jpeg;base64,/9j/4AAQSkZJRg==",
		"data:image/webp;base64,UklGRg==",
		"data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=",
		"data:image/gif;base64,R0lGODlhAQABAA==",
	} {
		t.Run(fmt.Sprintf("logo_%q", logo), func(t *testing.T) {
			settings := marshalTestAnnouncements(t, []map[string]interface{}{{
				"content":     "Announcement",
				"logo":        logo,
				"publishDate": testAnnouncementPublishDate,
			}})
			if err := ValidateConsoleSettings(settings, "Announcements"); err != nil {
				t.Fatalf("expected logo %q to be accepted, got %v", logo, err)
			}
		})
	}
}

func TestValidateAnnouncementsRejectsUnsafeLogoDataURLs(t *testing.T) {
	oversized := "data:image/png;base64," + strings.Repeat("A", announcementLogoMaxBytes)
	for _, logo := range []string{
		"https://cdn.example.com/logo.png",
		"/uploads/logo.svg",
		"/assets/logos/../logo.svg",
		"/assets/logos/deepseek.svg?cache=1",
		"//cdn.example.com/logo.svg",
		"data:text/html;base64,PHNjcmlwdD4=",
		"data:image/png,QUJD",
		"data:image/png;base64,not base64!!",
		"data:image/png;base64,",
		"data:image/svg+xml;base64,PHN2ZyBvbmxvYWQ9YWxlcnQoMSk+PC9zdmc+",
		oversized,
	} {
		t.Run(fmt.Sprintf("logo_%d", len(logo)), func(t *testing.T) {
			settings := marshalTestAnnouncements(t, []map[string]interface{}{{
				"content":     "Announcement",
				"logo":        logo,
				"publishDate": testAnnouncementPublishDate,
			}})
			if err := ValidateConsoleSettings(settings, "Announcements"); err == nil {
				t.Fatalf("expected unsafe logo to be rejected: %q", logo[:minInt(len(logo), 40)])
			}
		})
	}
}

func TestValidateAnnouncementsRejectsMalformedPublishDate(t *testing.T) {
	settings := marshalTestAnnouncements(t, []map[string]interface{}{{
		"content":     "Announcement",
		"publishDate": "not-a-date",
	}})
	if err := ValidateConsoleSettings(settings, "Announcements"); err == nil {
		t.Fatal("expected malformed publish date to be rejected")
	}
}

func TestGetJSONListUsesCommonJSONWrapperAndReturnsEmptyOnMalformedValue(t *testing.T) {
	if got := getJSONList("not json"); len(got) != 0 {
		t.Fatalf("expected malformed list to be empty, got %v", got)
	}
	if got := getJSONList(`[ {"content":"ok"} ]`); len(got) != 1 {
		t.Fatalf("expected one parsed item, got %d", len(got))
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
