package service

import "testing"

func TestExtractAnnouncementTranslationJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain", input: `{"en":{"content":"Hello"}}`, want: `{"en":{"content":"Hello"}}`},
		{name: "markdown fence", input: "```json\n{\"en\":{\"content\":\"Hello\"}}\n```", want: `{"en":{"content":"Hello"}}`},
		{name: "surrounding text", input: "Here is the translation:\n{\"en\":{\"content\":\"Hello\"}}\nDone.", want: `{"en":{"content":"Hello"}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := extractAnnouncementTranslationJSON(test.input); got != test.want {
				t.Fatalf("extractAnnouncementTranslationJSON() = %q, want %q", got, test.want)
			}
		})
	}
}
