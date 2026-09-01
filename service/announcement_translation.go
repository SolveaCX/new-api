package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

const announcementTranslationMaxResponseBytes = int64(128 * 1024)

var announcementTranslationLocales = []string{"en", "fr", "ja", "ru", "vi", "es", "pt"}

type LocalizedNotice struct {
	Content      string                             `json:"content"`
	Translations map[string]AnnouncementTranslation `json:"translations,omitempty"`
}

type AnnouncementTranslation struct {
	Content string `json:"content"`
	Extra   string `json:"extra,omitempty"`
}

type announcementTranslationRequest struct {
	Model          string                           `json:"model"`
	Messages       []announcementTranslationMessage `json:"messages"`
	Stream         bool                             `json:"stream"`
	ResponseFormat announcementTranslationFormat    `json:"response_format"`
}

type announcementTranslationFormat struct {
	Type string `json:"type"`
}

type announcementTranslationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type announcementTranslationResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func LocalizeNotice(raw, language string) string {
	notice := LocalizedNotice{}
	if err := common.Unmarshal([]byte(raw), &notice); err != nil || strings.TrimSpace(notice.Content) == "" {
		return raw
	}
	for _, candidate := range []string{language, strings.Split(language, "-")[0], "en", "zh"} {
		if translation, ok := notice.Translations[candidate]; ok && strings.TrimSpace(translation.Content) != "" {
			return translation.Content
		}
	}
	return notice.Content
}

func TranslateAnnouncement(ctx context.Context, content, extra string) (map[string]AnnouncementTranslation, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("announcement content is empty")
	}
	apiKey := operation_setting.GetMonitorAIAnalysisAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("monitor AI analysis API key is not configured")
	}

	baseURL := strings.TrimRight(operation_setting.GetMonitorAIAnalysisBaseURL(), "/")
	endpoint := baseURL + "/chat/completions"
	fetchSetting := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(endpoint, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return nil, fmt.Errorf("translation endpoint rejected: %v", err)
	}

	localeList := strings.Join(announcementTranslationLocales, ", ")
	prompt := fmt.Sprintf("Translate the Chinese announcement into these locales: %s. Return only valid JSON with exactly these keys. Each value must contain content and may contain extra. Preserve Markdown, URLs, placeholders, and product names. Do not translate URLs or placeholders. Source content: %q. Source extra: %q", localeList, content, strings.TrimSpace(extra))
	body, err := common.Marshal(announcementTranslationRequest{
		Model: operation_setting.GetMonitorAIAnalysisModel(),
		Messages: []announcementTranslationMessage{
			{Role: "system", Content: "You are a precise product announcement translator."},
			{Role: "user", Content: prompt},
		},
		Stream:         false,
		ResponseFormat: announcementTranslationFormat{Type: "json_object"},
	})
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "NewAPI-Announcement-Translation/1.0")
	client, err := requireHttpClient()
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, announcementTranslationMaxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > announcementTranslationMaxResponseBytes {
		return nil, fmt.Errorf("translation response is too large")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		logger.LogWarn(ctx, fmt.Sprintf("announcement translation provider response status=%d content_type=%q body_bytes=%d", resp.StatusCode, resp.Header.Get("Content-Type"), len(raw)))
		return nil, fmt.Errorf("translation provider returned status %d", resp.StatusCode)
	}

	var envelope announcementTranslationResponse
	if err := common.Unmarshal(raw, &envelope); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("announcement translation parse failed provider_status=%d content_type=%q body_bytes=%d parser_stage=%q", resp.StatusCode, resp.Header.Get("Content-Type"), len(raw), "provider_envelope_decode"))
		return nil, err
	}
	if envelope.Error != nil {
		logger.LogWarn(ctx, fmt.Sprintf("announcement translation provider returned an error status=%d content_type=%q body_bytes=%d parser_stage=%q", resp.StatusCode, resp.Header.Get("Content-Type"), len(raw), "provider_error"))
		return nil, fmt.Errorf("translation provider returned an error")
	}
	if len(envelope.Choices) == 0 {
		logger.LogWarn(ctx, fmt.Sprintf("announcement translation provider returned no choices status=%d content_type=%q body_bytes=%d parser_stage=%q", resp.StatusCode, resp.Header.Get("Content-Type"), len(raw), "missing_choices"))
		return nil, fmt.Errorf("translation provider returned no choices")
	}

	translations, err := parseAnnouncementTranslationResponse(envelope.Choices[0].Message.Content)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("announcement translation parse failed provider_status=%d content_type=%q body_bytes=%d choices=%d message_content_bytes=%d parser_stage=%q", resp.StatusCode, resp.Header.Get("Content-Type"), len(raw), len(envelope.Choices), len(envelope.Choices[0].Message.Content), err.Error()))
		return nil, fmt.Errorf("translation provider returned invalid JSON")
	}
	logger.LogInfo(ctx, fmt.Sprintf("announcement translation parsed provider_status=%d content_type=%q body_bytes=%d choices=%d message_content_bytes=%d locales=%d", resp.StatusCode, resp.Header.Get("Content-Type"), len(raw), len(envelope.Choices), len(envelope.Choices[0].Message.Content), len(translations)))
	for _, locale := range announcementTranslationLocales {
		translation, ok := translations[locale]
		if !ok || strings.TrimSpace(translation.Content) == "" {
			return nil, fmt.Errorf("translation missing locale %s", locale)
		}
		translation.Content = strings.TrimSpace(translation.Content)
		translation.Extra = strings.TrimSpace(translation.Extra)
		translations[locale] = translation
	}
	return translations, nil
}

func parseAnnouncementTranslationResponse(raw string) (map[string]AnnouncementTranslation, error) {
	raw = strings.TrimSpace(raw)
	for _, candidate := range []string{raw, trimAnnouncementTranslationFence(raw)} {
		if translations, ok := decodeAnnouncementTranslations(candidate, 0); ok {
			return translations, nil
		}
	}

	for _, candidate := range []string{raw, trimAnnouncementTranslationFence(raw)} {
		extracted := extractAnnouncementTranslationJSON(candidate)
		if extracted == candidate {
			continue
		}
		if translations, ok := decodeAnnouncementTranslations(extracted, 0); ok {
			return translations, nil
		}
	}
	return nil, fmt.Errorf("parser_stage=unrecognized_translation_shape")
}

func decodeAnnouncementTranslations(raw string, depth int) (map[string]AnnouncementTranslation, bool) {
	if depth > 1 {
		return nil, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}

	var wrapped struct {
		Translations map[string]AnnouncementTranslation `json:"translations"`
	}
	if err := common.Unmarshal([]byte(raw), &wrapped); err == nil && len(wrapped.Translations) > 0 {
		return wrapped.Translations, true
	}

	var translations map[string]AnnouncementTranslation
	if err := common.Unmarshal([]byte(raw), &translations); err == nil && hasAnnouncementTranslationContent(translations) {
		return translations, true
	}

	var wrappedText struct {
		Translations map[string]string `json:"translations"`
	}
	if err := common.Unmarshal([]byte(raw), &wrappedText); err == nil && len(wrappedText.Translations) > 0 {
		return announcementTranslationsFromText(wrappedText.Translations), true
	}

	var textTranslations map[string]string
	if err := common.Unmarshal([]byte(raw), &textTranslations); err == nil && len(textTranslations) > 0 {
		return announcementTranslationsFromText(textTranslations), true
	}

	var encoded string
	if err := common.Unmarshal([]byte(raw), &encoded); err == nil {
		return decodeAnnouncementTranslations(encoded, depth+1)
	}
	return nil, false
}

func announcementTranslationsFromText(translations map[string]string) map[string]AnnouncementTranslation {
	result := make(map[string]AnnouncementTranslation, len(translations))
	for locale, content := range translations {
		result[locale] = AnnouncementTranslation{Content: content}
	}
	return result
}

func hasAnnouncementTranslationContent(translations map[string]AnnouncementTranslation) bool {
	for _, translation := range translations {
		if strings.TrimSpace(translation.Content) != "" {
			return true
		}
	}
	return false
}

func trimAnnouncementTranslationFence(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "```") || !strings.HasSuffix(raw, "```") {
		return raw
	}
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "```"))
	if newline := strings.IndexByte(raw, '\n'); newline >= 0 {
		language := strings.TrimSpace(raw[:newline])
		if language == "json" || language == "JSON" {
			raw = strings.TrimSpace(raw[newline+1:])
		}
	}
	return strings.TrimSpace(strings.TrimSuffix(raw, "```"))
}

func extractAnnouncementTranslationJSON(raw string) string {
	raw = trimAnnouncementTranslationFence(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end >= start {
		return raw[start : end+1]
	}
	return raw
}
