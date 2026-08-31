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
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

const announcementTranslationMaxResponseBytes = int64(128 * 1024)

var announcementTranslationLocales = []string{"en", "fr", "ja", "ru", "vi", "es", "pt"}

type AnnouncementTranslation struct {
	Content string `json:"content"`
	Extra   string `json:"extra,omitempty"`
}

type announcementTranslationRequest struct {
	Model    string                           `json:"model"`
	Messages []announcementTranslationMessage `json:"messages"`
	Stream   bool                             `json:"stream"`
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
		Stream: false,
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
		return nil, fmt.Errorf("translation provider returned status %d", resp.StatusCode)
	}

	var envelope announcementTranslationResponse
	if err := common.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("translation provider returned an error")
	}
	if len(envelope.Choices) == 0 {
		return nil, fmt.Errorf("translation provider returned no choices")
	}

	translatedJSON := strings.TrimSpace(envelope.Choices[0].Message.Content)
	translatedJSON = strings.TrimPrefix(translatedJSON, "```json")
	translatedJSON = strings.TrimPrefix(translatedJSON, "```")
	translatedJSON = strings.TrimSuffix(strings.TrimSpace(translatedJSON), "```")
	var translations map[string]AnnouncementTranslation
	if err := common.Unmarshal([]byte(strings.TrimSpace(translatedJSON)), &translations); err != nil {
		return nil, fmt.Errorf("translation provider returned invalid JSON")
	}
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
