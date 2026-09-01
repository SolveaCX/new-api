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
