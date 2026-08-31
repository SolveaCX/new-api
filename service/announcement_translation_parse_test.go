package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func parseAnnouncementTranslationJSON(raw string) (map[string]AnnouncementTranslation, error) {
	var translations map[string]AnnouncementTranslation
	if err := common.Unmarshal([]byte(raw), &translations); err == nil {
		return translations, nil
	}
	var encoded string
	if err := common.Unmarshal([]byte(raw), &encoded); err != nil {
		return nil, err
	}
	if err := common.Unmarshal([]byte(encoded), &translations); err != nil {
		return nil, err
	}
	return translations, nil
}

func TestParseAnnouncementTranslationJSON(t *testing.T) {
	plain, err := parseAnnouncementTranslationJSON(`{"en":{"content":"Hello"}}`)
	if err != nil || plain["en"].Content != "Hello" {
		t.Fatalf("plain JSON was not parsed: %#v, %v", plain, err)
	}

	encoded, err := parseAnnouncementTranslationJSON(`"{\"en\":{\"content\":\"Hello\"}}"`)
	if err != nil || encoded["en"].Content != "Hello" {
		t.Fatalf("encoded JSON was not parsed: %#v, %v", encoded, err)
	}
}
