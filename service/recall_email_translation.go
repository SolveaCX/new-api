package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

const (
	recallEmailTranslationDefaultMaxBytes = int64(256 * 1024)
	recallEmailTranslationDefaultTimeout  = 30 * time.Second
	recallEmailTranslationMaxAttempts     = 3
	recallEmailTranslationMaxRetryAfter   = 2 * time.Second
)

var (
	recallEmailTranslationLanguages = []string{"zh", "es", "fr", "pt", "ru", "ja", "vi"}
	recallEmailProtectedPattern     = regexp.MustCompile(`https?://[^\s<>"']+|\{\{[^{}\r\n]+\}\}|\$\{[^{}\r\n]+\}`)
	recallEmailSentinelPattern      = regexp.MustCompile(`__RECALL_EMAIL_PROTECTED_[0-9]{4}__`)
)

type RecallEmailTranslator interface {
	Translate(ctx context.Context, stages []RecallEmailStage) (map[int]map[string]RecallEmailTemplate, error)
}

type RecallEmailTranslatorOptions struct {
	APIKey   string
	BaseURL  string
	Model    string
	Client   *http.Client
	MaxBytes int64
	Timeout  time.Duration

	sleep func(context.Context, time.Duration) error
}

type recallEmailTranslator struct {
	apiKey   string
	baseURL  string
	model    string
	client   *http.Client
	maxBytes int64
	timeout  time.Duration
	sleep    func(context.Context, time.Duration) error
}

type recallEmailTranslationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type recallEmailTranslationRequest struct {
	Model           string                          `json:"model"`
	Input           []recallEmailTranslationMessage `json:"input"`
	Text            recallEmailTranslationText      `json:"text"`
	MaxOutputTokens int                             `json:"max_output_tokens"`
}

type recallEmailTranslationText struct {
	Format recallEmailTranslationFormat `json:"format"`
}

type recallEmailTranslationFormat struct {
	Type   string         `json:"type"`
	Name   string         `json:"name"`
	Schema map[string]any `json:"schema"`
	Strict bool           `json:"strict"`
}

type recallEmailTranslationEnvelope struct {
	OutputText string `json:"output_text"`
	Output     []struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type recallEmailTranslationResult struct {
	Stages []struct {
		StageNo      int                            `json:"stage_no"`
		Translations map[string]RecallEmailTemplate `json:"translations"`
	} `json:"stages"`
}

type recallEmailProtectedValue struct {
	Sentinel string
	Original string
}

type recallEmailProtectedStage struct {
	StageNo       int    `json:"stage_no"`
	Subject       string `json:"subject"`
	BodyText      string `json:"body_text"`
	subjectValues []recallEmailProtectedValue
	bodyValues    []recallEmailProtectedValue
}

func NewRecallEmailTranslator(options RecallEmailTranslatorOptions) RecallEmailTranslator {
	baseURL := strings.TrimSpace(options.BaseURL)
	if baseURL == "" {
		baseURL = operation_setting.GetMonitorAIAnalysisBaseURL()
	}
	modelName := strings.TrimSpace(options.Model)
	if modelName == "" {
		modelName = operation_setting.GetMonitorAIAnalysisModel()
	}
	maxBytes := options.MaxBytes
	if maxBytes <= 0 {
		maxBytes = recallEmailTranslationDefaultMaxBytes
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = recallEmailTranslationDefaultTimeout
	}
	sleep := options.sleep
	if sleep == nil {
		sleep = recallEmailTranslationSleep
	}
	return &recallEmailTranslator{
		apiKey:   strings.TrimSpace(options.APIKey),
		baseURL:  strings.TrimSpace(baseURL),
		model:    strings.TrimSpace(modelName),
		client:   options.Client,
		maxBytes: maxBytes,
		timeout:  timeout,
		sleep:    sleep,
	}
}

func (t *recallEmailTranslator) Translate(ctx context.Context, stages []RecallEmailStage) (map[int]map[string]RecallEmailTemplate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if t.apiKey == "" {
		return nil, fmt.Errorf("recall email translation API key is empty")
	}
	protectedStages, err := protectRecallEmailTranslationStages(stages)
	if err != nil {
		return nil, err
	}
	endpoint := recallEmailTranslationEndpoint(t.baseURL)
	fetchSetting := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(endpoint, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return nil, fmt.Errorf("recall email translation request rejected: %v", err)
	}
	requestBody, err := common.Marshal(buildRecallEmailTranslationRequest(t.model, protectedStages))
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	client := t.client
	if client == nil {
		client, err = requireHttpClient()
		if err != nil {
			return nil, err
		}
	}

	var raw []byte
	for attempt := 0; attempt < recallEmailTranslationMaxAttempts; attempt++ {
		raw, err = t.request(requestCtx, client, endpoint, requestBody, attempt)
		if err == nil {
			break
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		var retryable *recallEmailTranslationRetryableError
		if !errors.As(err, &retryable) || attempt == recallEmailTranslationMaxAttempts-1 {
			return nil, err
		}
		if err := t.sleep(requestCtx, retryable.delay); err != nil {
			return nil, err
		}
	}

	result, err := parseRecallEmailTranslationResponse(raw)
	if err != nil {
		return nil, err
	}
	return validateAndRestoreRecallEmailTranslations(result, protectedStages)
}

type recallEmailTranslationRetryableError struct {
	err   error
	delay time.Duration
}

func (e *recallEmailTranslationRetryableError) Error() string { return e.err.Error() }
func (e *recallEmailTranslationRetryableError) Unwrap() error { return e.err }

func (t *recallEmailTranslator) request(ctx context.Context, client *http.Client, endpoint string, body []byte, attempt int) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "NewAPI-Recall-Email-Translation/1.0")

	resp, err := client.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if isTemporaryRecallEmailTranslationNetworkError(err) {
			return nil, &recallEmailTranslationRetryableError{err: err, delay: recallEmailTranslationBackoff(attempt)}
		}
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, t.maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > t.maxBytes {
		return nil, fmt.Errorf("recall email translation response exceeds %d bytes", t.maxBytes)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		statusErr := fmt.Errorf("recall email translation returned status %d %s: %s", resp.StatusCode, http.StatusText(resp.StatusCode), recallEmailTranslationErrorBodySummary(raw))
		if recallEmailTranslationRetryableStatus(resp.StatusCode) {
			return nil, &recallEmailTranslationRetryableError{err: statusErr, delay: recallEmailTranslationRetryDelay(resp.Header.Get("Retry-After"), attempt)}
		}
		return nil, statusErr
	}
	return raw, nil
}

func protectRecallEmailTranslationStages(stages []RecallEmailStage) ([]recallEmailProtectedStage, error) {
	if len(stages) < 1 || len(stages) > 3 {
		return nil, fmt.Errorf("recall email translation requires one to three stages")
	}
	seen := make(map[int]struct{}, len(stages))
	protected := make([]recallEmailProtectedStage, 0, len(stages))
	counter := 0
	for _, stage := range stages {
		if stage.StageNo <= 0 {
			return nil, fmt.Errorf("recall email translation stage number must be positive")
		}
		if _, exists := seen[stage.StageNo]; exists {
			return nil, fmt.Errorf("recall email translation stage %d is duplicated", stage.StageNo)
		}
		seen[stage.StageNo] = struct{}{}
		english, exists := stage.Templates["en"]
		if !exists || strings.TrimSpace(english.Subject) == "" || strings.TrimSpace(english.BodyText) == "" {
			return nil, fmt.Errorf("recall email translation stage %d requires a non-empty English template", stage.StageNo)
		}
		if strings.ContainsAny(english.Subject, "\r\n") {
			return nil, fmt.Errorf("recall email translation stage %d English subject must be single-line", stage.StageNo)
		}
		subject, subjectValues := protectRecallEmailValue(english.Subject, &counter)
		body, bodyValues := protectRecallEmailValue(english.BodyText, &counter)
		protected = append(protected, recallEmailProtectedStage{
			StageNo: stage.StageNo, Subject: subject, BodyText: body,
			subjectValues: subjectValues, bodyValues: bodyValues,
		})
	}
	return protected, nil
}

func protectRecallEmailValue(value string, counter *int) (string, []recallEmailProtectedValue) {
	values := make([]recallEmailProtectedValue, 0)
	protected := recallEmailProtectedPattern.ReplaceAllStringFunc(value, func(match string) string {
		*counter++
		sentinel := fmt.Sprintf("__RECALL_EMAIL_PROTECTED_%04d__", *counter)
		values = append(values, recallEmailProtectedValue{Sentinel: sentinel, Original: match})
		return sentinel
	})
	return protected, values
}

func buildRecallEmailTranslationRequest(modelName string, stages []recallEmailProtectedStage) recallEmailTranslationRequest {
	stagesJSON, _ := common.Marshal(stages)
	return recallEmailTranslationRequest{
		Model: modelName,
		Input: []recallEmailTranslationMessage{
			{Role: "system", Content: strings.Join([]string{
				"Translate recall marketing email templates from English into Simplified Chinese, Spanish, French, Portuguese, Russian, Japanese, and Vietnamese.",
				"Preserve meaning, brand names, numbers, amounts, coupon codes, whitespace, and every protected marker exactly and in the same order within its field.",
				"Do not add claims or content. Subjects must be single-line. Return JSON only following the schema.",
			}, " ")},
			{Role: "user", Content: "Translate every stage and target language in this JSON:\n" + string(stagesJSON)},
		},
		Text: recallEmailTranslationText{Format: recallEmailTranslationFormat{
			Type: "json_schema", Name: "recall_email_translations", Schema: buildRecallEmailTranslationSchema(), Strict: true,
		}},
		MaxOutputTokens: 6000,
	}
}

func buildRecallEmailTranslationSchema() map[string]any {
	templateSchema := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"subject":   map[string]any{"type": "string"},
			"body_text": map[string]any{"type": "string"},
		},
		"required": []string{"subject", "body_text"},
	}
	translations := make(map[string]any, len(recallEmailTranslationLanguages))
	for _, language := range recallEmailTranslationLanguages {
		translations[language] = templateSchema
	}
	stageSchema := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"stage_no": map[string]any{"type": "integer"},
			"translations": map[string]any{
				"type": "object", "additionalProperties": false,
				"properties": translations, "required": recallEmailTranslationLanguages,
			},
		},
		"required": []string{"stage_no", "translations"},
	}
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"stages": map[string]any{"type": "array", "items": stageSchema},
		},
		"required": []string{"stages"},
	}
}

func parseRecallEmailTranslationResponse(raw []byte) (recallEmailTranslationResult, error) {
	var envelope recallEmailTranslationEnvelope
	if err := common.Unmarshal(raw, &envelope); err != nil {
		return recallEmailTranslationResult{}, fmt.Errorf("invalid recall email translation response: %w", err)
	}
	if envelope.Error != nil {
		return recallEmailTranslationResult{}, fmt.Errorf("recall email translation provider returned an error")
	}
	outputText := strings.TrimSpace(envelope.OutputText)
	if outputText == "" {
		for _, output := range envelope.Output {
			for _, content := range output.Content {
				if strings.TrimSpace(content.Text) != "" {
					outputText = strings.TrimSpace(content.Text)
					break
				}
			}
			if outputText != "" {
				break
			}
		}
	}
	if outputText == "" {
		return recallEmailTranslationResult{}, fmt.Errorf("recall email translation returned empty output")
	}
	var result recallEmailTranslationResult
	if err := common.Unmarshal([]byte(outputText), &result); err != nil {
		return recallEmailTranslationResult{}, fmt.Errorf("invalid recall email translation output: %w", err)
	}
	return result, nil
}

func validateAndRestoreRecallEmailTranslations(result recallEmailTranslationResult, stages []recallEmailProtectedStage) (map[int]map[string]RecallEmailTemplate, error) {
	expected := make(map[int]recallEmailProtectedStage, len(stages))
	for _, stage := range stages {
		expected[stage.StageNo] = stage
	}
	if len(result.Stages) != len(expected) {
		return nil, fmt.Errorf("recall email translation returned %d stages; expected %d", len(result.Stages), len(expected))
	}
	translated := make(map[int]map[string]RecallEmailTemplate, len(expected))
	for _, stage := range result.Stages {
		protected, exists := expected[stage.StageNo]
		if !exists {
			return nil, fmt.Errorf("recall email translation returned unexpected stage %d", stage.StageNo)
		}
		if _, duplicate := translated[stage.StageNo]; duplicate {
			return nil, fmt.Errorf("recall email translation returned duplicate stage %d", stage.StageNo)
		}
		if len(stage.Translations) != len(recallEmailTranslationLanguages) {
			return nil, fmt.Errorf("recall email translation stage %d must contain exactly seven target languages", stage.StageNo)
		}
		translations := make(map[string]RecallEmailTemplate, len(recallEmailTranslationLanguages))
		for _, language := range recallEmailTranslationLanguages {
			template, exists := stage.Translations[language]
			if !exists {
				return nil, fmt.Errorf("recall email translation stage %d is missing language %s", stage.StageNo, language)
			}
			if strings.TrimSpace(template.Subject) == "" || strings.TrimSpace(template.BodyText) == "" {
				return nil, fmt.Errorf("recall email translation stage %d language %s contains an empty field", stage.StageNo, language)
			}
			if strings.ContainsAny(template.Subject, "\r\n") {
				return nil, fmt.Errorf("recall email translation stage %d language %s subject must be single-line", stage.StageNo, language)
			}
			subject, err := restoreRecallEmailProtectedValue(template.Subject, protected.subjectValues)
			if err != nil {
				return nil, fmt.Errorf("recall email translation stage %d language %s subject: %w", stage.StageNo, language, err)
			}
			body, err := restoreRecallEmailProtectedValue(template.BodyText, protected.bodyValues)
			if err != nil {
				return nil, fmt.Errorf("recall email translation stage %d language %s body: %w", stage.StageNo, language, err)
			}
			translations[language] = RecallEmailTemplate{Subject: strings.TrimSpace(subject), BodyText: strings.TrimSpace(body)}
		}
		translated[stage.StageNo] = translations
	}
	return translated, nil
}

func restoreRecallEmailProtectedValue(value string, protected []recallEmailProtectedValue) (string, error) {
	found := recallEmailSentinelPattern.FindAllString(value, -1)
	if len(found) != len(protected) {
		return "", fmt.Errorf("protected marker sequence changed")
	}
	for index, item := range protected {
		if found[index] != item.Sentinel {
			return "", fmt.Errorf("protected marker sequence changed")
		}
		value = strings.ReplaceAll(value, item.Sentinel, item.Original)
	}
	return value, nil
}

func recallEmailTranslationEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(baseURL, "/responses") {
		return baseURL
	}
	return baseURL + "/responses"
}

func recallEmailTranslationRetryableStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || (status >= 500 && status < 600)
}

func isTemporaryRecallEmailTranslationNetworkError(err error) bool {
	var networkError net.Error
	return errors.As(err, &networkError) && (networkError.Timeout() || networkError.Temporary())
}

func recallEmailTranslationRetryDelay(retryAfter string, attempt int) time.Duration {
	retryAfter = strings.TrimSpace(retryAfter)
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		delay := time.Duration(seconds) * time.Second
		if delay > recallEmailTranslationMaxRetryAfter {
			return recallEmailTranslationMaxRetryAfter
		}
		return delay
	}
	if parsed, err := http.ParseTime(retryAfter); err == nil {
		delay := time.Until(parsed)
		if delay < 0 {
			return 0
		}
		if delay > recallEmailTranslationMaxRetryAfter {
			return recallEmailTranslationMaxRetryAfter
		}
		return delay
	}
	return recallEmailTranslationBackoff(attempt)
}

func recallEmailTranslationBackoff(attempt int) time.Duration {
	return time.Duration(attempt+1) * 100 * time.Millisecond
}

func recallEmailTranslationSleep(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func recallEmailTranslationErrorBodySummary(raw []byte) string {
	body := strings.TrimSpace(string(raw))
	if body == "" {
		return "empty response body"
	}
	sanitized := common.LocalLogPreview(common.MaskSensitiveInfo(body))
	return fmt.Sprintf("response body redacted (sanitized_length=%d)", len([]rune(sanitized)))
}
