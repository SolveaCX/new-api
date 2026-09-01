package service

import (
	"testing"
)

func TestParseAnnouncementTranslationResponse(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "plain JSON", input: `{"en":{"content":"Hello"}}`},
		{name: "encoded JSON", input: `"{\"en\":{\"content\":\"Hello\"}}"`},
		{name: "encoded JSON in fence", input: "```json\n\"{\\\"en\\\":{\\\"content\\\":\\\"Hello\\\"}}\"\n```"},
		{name: "wrapped translations", input: `{"translations":{"en":{"content":"Hello"}}}`},
		{name: "string values", input: `{"en":"Hello"}`},
		{name: "wrapped string values", input: `{"translations":{"en":"Hello"}}`},
		{name: "surrounding text", input: "Here is the translation:\n{\"en\":{\"content\":\"Hello\"}}\nDone."},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			translations, err := parseAnnouncementTranslationResponse(test.input)
			if err != nil || translations["en"].Content != "Hello" {
				t.Fatalf("parseAnnouncementTranslationResponse() = %#v, %v", translations, err)
			}
		})
	}
}

func TestLocalizeNoticeUsesChineseSourceForChineseLanguage(t *testing.T) {
	raw := `{"content":"中文公告","translations":{"en":{"content":"English notice"}}}`

	if got := LocalizeNotice(raw, "zh-CN"); got != "中文公告" {
		t.Fatalf("LocalizeNotice() = %q, want Chinese source", got)
	}
	if got := LocalizeNotice(raw, "en-US"); got != "English notice" {
		t.Fatalf("LocalizeNotice() = %q, want English translation", got)
	}
}

func TestLocalizeAnnouncementsUsesChineseSourceForChineseLanguage(t *testing.T) {
	announcements := []map[string]interface{}{{
		"content": "中文公告",
		"extra":   "中文说明",
		"translations": map[string]interface{}{
			"en": map[string]interface{}{
				"content": "English notice",
				"extra":   "English details",
			},
		},
	}}

	zh := LocalizeAnnouncements(announcements, "zh-CN")
	if got := zh[0]["content"]; got != "中文公告" {
		t.Fatalf("Chinese announcement content = %v, want Chinese source", got)
	}

	en := LocalizeAnnouncements(announcements, "en-US")
	if got := en[0]["content"]; got != "English notice" {
		t.Fatalf("English announcement content = %v, want English translation", got)
	}
	if got := en[0]["extra"]; got != "English details" {
		t.Fatalf("English announcement extra = %v, want English translation", got)
	}
}
