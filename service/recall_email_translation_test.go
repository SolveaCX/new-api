package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/require"
)

var recallEmailTranslationTestLanguages = []string{"zh", "es", "fr", "pt", "ru", "ja", "vi"}

func TestRecallEmailTranslatorTranslatesMultipleStagesInOneStructuredRequest(t *testing.T) {
	allowRecallEmailTranslationTestServer(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/custom/responses", r.URL.Path)
		require.Equal(t, "Bearer sk-translation", r.Header.Get("Authorization"))

		var request map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &request))
		require.Equal(t, "gpt-translation", request["model"])
		requestJSON, err := common.Marshal(request)
		require.NoError(t, err)
		requestText := string(requestJSON)
		require.Contains(t, requestText, `"type":"json_schema"`)
		require.Contains(t, requestText, `"strict":true`)
		require.Contains(t, requestText, `"additionalProperties":false`)
		for _, language := range recallEmailTranslationTestLanguages {
			require.Contains(t, requestText, `"`+language+`"`)
		}
		require.Contains(t, requestText, "First subject")
		require.Contains(t, requestText, "Second body")

		writeRecallEmailTranslationResponse(t, w, validRecallEmailTranslationResult([]int{1, 2}))
	}))
	defer server.Close()

	translator := NewRecallEmailTranslator(RecallEmailTranslatorOptions{
		APIKey:  "sk-translation",
		BaseURL: server.URL + "/custom/",
		Model:   "gpt-translation",
		Client:  server.Client(),
	})
	translated, err := translator.Translate(context.Background(), []RecallEmailStage{
		{StageNo: 1, Templates: map[string]RecallEmailTemplate{"en": {Subject: "First subject", BodyText: "First body"}}},
		{StageNo: 2, Templates: map[string]RecallEmailTemplate{"en": {Subject: "Second subject", BodyText: "Second body"}}},
	})

	require.NoError(t, err)
	require.EqualValues(t, 1, requests.Load())
	require.Len(t, translated, 2)
	require.Equal(t, "zh subject 1", translated[1]["zh"].Subject)
	require.Equal(t, "vi body 2", translated[2]["vi"].BodyText)
}

func TestRecallEmailTranslatorRejectsMissingAPIKey(t *testing.T) {
	translator := NewRecallEmailTranslator(RecallEmailTranslatorOptions{})
	_, err := translator.Translate(context.Background(), recallEmailTranslationTestStages())
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "api key")
}

func TestRecallEmailTranslatorDoesNotRetryPermanentHTTPFailuresAndRedactsBody(t *testing.T) {
	allowRecallEmailTranslationTestServer(t)
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"error":"Authorization: Bearer sk-secret provider details"}`))
			}))
			defer server.Close()

			translator := newRecallEmailTranslationTestTranslator(server, RecallEmailTranslatorOptions{})
			_, err := translator.Translate(context.Background(), recallEmailTranslationTestStages())

			require.Error(t, err)
			require.EqualValues(t, 1, requests.Load())
			require.Contains(t, err.Error(), "status "+strconv.Itoa(status))
			require.NotContains(t, err.Error(), "sk-secret")
			require.NotContains(t, err.Error(), "provider details")
		})
	}
}

func TestRecallEmailTranslatorRetriesTemporaryHTTPFailures(t *testing.T) {
	allowRecallEmailTranslationTestServer(t)
	for _, status := range []int{http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempt := requests.Add(1)
				if attempt < 3 {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(status)
					return
				}
				writeRecallEmailTranslationResponse(t, w, validRecallEmailTranslationResult([]int{1}))
			}))
			defer server.Close()

			translator := newRecallEmailTranslationTestTranslator(server, RecallEmailTranslatorOptions{})
			translated, err := translator.Translate(context.Background(), recallEmailTranslationTestStages())

			require.NoError(t, err)
			require.EqualValues(t, 3, requests.Load())
			require.Equal(t, "fr body 1", translated[1]["fr"].BodyText)
		})
	}
}

func TestRecallEmailTranslatorRetriesTemporaryNetworkFailures(t *testing.T) {
	var requests atomic.Int32
	client := &http.Client{Transport: recallEmailTranslationRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
		attempt := requests.Add(1)
		if attempt < 3 {
			return nil, temporaryTranslationNetworkError{}
		}
		payload := recallEmailTranslationHTTPPayload(t, validRecallEmailTranslationResult([]int{1}))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(payload))),
		}, nil
	})}
	translator := NewRecallEmailTranslator(RecallEmailTranslatorOptions{
		APIKey: "sk-test", BaseURL: "https://example.com/v1", Model: "gpt-test", Client: client,
		sleep: func(context.Context, time.Duration) error { return nil },
	})

	_, err := translator.Translate(context.Background(), recallEmailTranslationTestStages())
	require.NoError(t, err)
	require.EqualValues(t, 3, requests.Load())
}

func TestRecallEmailTranslatorRejectsOversizeAndMalformedResponses(t *testing.T) {
	allowRecallEmailTranslationTestServer(t)
	tests := []struct {
		name     string
		maxBytes int64
		body     string
	}{
		{name: "oversize", maxBytes: 16, body: strings.Repeat("x", 17)},
		{name: "malformed envelope", maxBytes: 1024, body: `{not-json`},
		{name: "malformed output", maxBytes: 1024, body: `{"output_text":"not-json"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			translator := newRecallEmailTranslationTestTranslator(server, RecallEmailTranslatorOptions{MaxBytes: test.maxBytes})
			_, err := translator.Translate(context.Background(), recallEmailTranslationTestStages())
			require.Error(t, err)
		})
	}
}

func TestRecallEmailTranslationValidatesStructuredOutput(t *testing.T) {
	allowRecallEmailTranslationTestServer(t)
	tests := []struct {
		name   string
		result map[string]any
	}{
		{name: "missing language", result: func() map[string]any {
			result := validRecallEmailTranslationResult([]int{1})
			delete(result["stages"].([]map[string]any)[0]["translations"].(map[string]RecallEmailTemplate), "vi")
			return result
		}()},
		{name: "extra language", result: func() map[string]any {
			result := validRecallEmailTranslationResult([]int{1})
			result["stages"].([]map[string]any)[0]["translations"].(map[string]RecallEmailTemplate)["de"] = RecallEmailTemplate{Subject: "Hallo", BodyText: "Text"}
			return result
		}()},
		{name: "wrong stage", result: validRecallEmailTranslationResult([]int{2})},
		{name: "duplicate stage", result: func() map[string]any {
			result := validRecallEmailTranslationResult([]int{1, 1})
			return result
		}()},
		{name: "empty field", result: func() map[string]any {
			result := validRecallEmailTranslationResult([]int{1})
			translations := result["stages"].([]map[string]any)[0]["translations"].(map[string]RecallEmailTemplate)
			translations["zh"] = RecallEmailTemplate{Subject: "", BodyText: "body"}
			return result
		}()},
		{name: "multiline subject", result: func() map[string]any {
			result := validRecallEmailTranslationResult([]int{1})
			translations := result["stages"].([]map[string]any)[0]["translations"].(map[string]RecallEmailTemplate)
			translations["zh"] = RecallEmailTemplate{Subject: "line one\r\nline two", BodyText: "body"}
			return result
		}()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRecallEmailTranslationResponse(t, w, test.result)
			}))
			defer server.Close()
			translator := newRecallEmailTranslationTestTranslator(server, RecallEmailTranslatorOptions{})
			_, err := translator.Translate(context.Background(), recallEmailTranslationTestStages())
			require.Error(t, err)
		})
	}
}

func TestRecallEmailTranslationProtectsAndRestoresTokensAndURLs(t *testing.T) {
	allowRecallEmailTranslationTestServer(t)
	sentinelPattern := regexp.MustCompile(`__RECALL_EMAIL_PROTECTED_[0-9]{4}__`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		requestText := string(raw)
		require.NotContains(t, requestText, "{{name}}")
		require.NotContains(t, requestText, "https://example.com/pay?code=${code}")
		require.NotContains(t, requestText, "{{coupon}}")
		sentinels := sentinelPattern.FindAllString(requestText, -1)
		require.GreaterOrEqual(t, len(sentinels), 3)

		translations := make(map[string]RecallEmailTemplate, len(recallEmailTranslationTestLanguages))
		for _, language := range recallEmailTranslationTestLanguages {
			translations[language] = RecallEmailTemplate{
				Subject:  language + " hello " + sentinels[0],
				BodyText: language + " visit " + sentinels[1] + " use " + sentinels[2],
			}
		}
		writeRecallEmailTranslationResponse(t, w, map[string]any{"stages": []map[string]any{{"stage_no": 1, "translations": translations}}})
	}))
	defer server.Close()

	translator := newRecallEmailTranslationTestTranslator(server, RecallEmailTranslatorOptions{})
	translated, err := translator.Translate(context.Background(), []RecallEmailStage{{
		StageNo: 1,
		Templates: map[string]RecallEmailTemplate{"en": {
			Subject:  "Hello {{name}}",
			BodyText: "Visit https://example.com/pay?code=${code} and use {{coupon}}",
		}},
	}})

	require.NoError(t, err)
	require.Equal(t, "zh hello {{name}}", translated[1]["zh"].Subject)
	require.Equal(t, "zh visit https://example.com/pay?code=${code} use {{coupon}}", translated[1]["zh"].BodyText)
}

func TestRecallEmailTranslationRejectsDamagedProtectedSequence(t *testing.T) {
	allowRecallEmailTranslationTestServer(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		translations := make(map[string]RecallEmailTemplate, len(recallEmailTranslationTestLanguages))
		for _, language := range recallEmailTranslationTestLanguages {
			translations[language] = RecallEmailTemplate{Subject: language + " __RECALL_EMAIL_PROTECTED_9999__", BodyText: language + " body"}
		}
		writeRecallEmailTranslationResponse(t, w, map[string]any{"stages": []map[string]any{{"stage_no": 1, "translations": translations}}})
	}))
	defer server.Close()

	translator := newRecallEmailTranslationTestTranslator(server, RecallEmailTranslatorOptions{})
	_, err := translator.Translate(context.Background(), []RecallEmailStage{{StageNo: 1, Templates: map[string]RecallEmailTemplate{
		"en": {Subject: "Hello {{name}}", BodyText: "Body"},
	}}})
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "protected")
}

func TestRecallEmailTranslatorStopsImmediatelyWhenContextCancelled(t *testing.T) {
	var requests atomic.Int32
	client := &http.Client{Transport: recallEmailTranslationRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests.Add(1)
		return nil, r.Context().Err()
	})}
	translator := NewRecallEmailTranslator(RecallEmailTranslatorOptions{
		APIKey: "sk-test", BaseURL: "https://example.com/v1", Model: "gpt-test", Client: client,
		sleep: func(context.Context, time.Duration) error { return nil },
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := translator.Translate(ctx, recallEmailTranslationTestStages())
	require.ErrorIs(t, err, context.Canceled)
	require.LessOrEqual(t, requests.Load(), int32(1))
}

func recallEmailTranslationTestStages() []RecallEmailStage {
	return []RecallEmailStage{{
		StageNo: 1,
		Templates: map[string]RecallEmailTemplate{
			"en": {Subject: "English subject", BodyText: "English body"},
		},
	}}
}

func validRecallEmailTranslationResult(stageNumbers []int) map[string]any {
	stages := make([]map[string]any, 0, len(stageNumbers))
	for _, stageNo := range stageNumbers {
		translations := make(map[string]RecallEmailTemplate, len(recallEmailTranslationTestLanguages))
		for _, language := range recallEmailTranslationTestLanguages {
			translations[language] = RecallEmailTemplate{
				Subject:  language + " subject " + strconv.Itoa(stageNo),
				BodyText: language + " body " + strconv.Itoa(stageNo),
			}
		}
		stages = append(stages, map[string]any{"stage_no": stageNo, "translations": translations})
	}
	return map[string]any{"stages": stages}
}

func writeRecallEmailTranslationResponse(t *testing.T, w http.ResponseWriter, result map[string]any) {
	t.Helper()
	payload := recallEmailTranslationHTTPPayload(t, result)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(payload)
}

func recallEmailTranslationHTTPPayload(t *testing.T, result map[string]any) []byte {
	t.Helper()
	resultJSON, err := common.Marshal(result)
	require.NoError(t, err)
	payload, err := common.Marshal(map[string]any{"output_text": string(resultJSON)})
	require.NoError(t, err)
	return payload
}

func newRecallEmailTranslationTestTranslator(server *httptest.Server, options RecallEmailTranslatorOptions) RecallEmailTranslator {
	options.APIKey = "sk-test"
	options.BaseURL = server.URL + "/v1"
	options.Model = "gpt-test"
	options.Client = server.Client()
	options.sleep = func(context.Context, time.Duration) error { return nil }
	return NewRecallEmailTranslator(options)
}

func allowRecallEmailTranslationTestServer(t *testing.T) {
	t.Helper()
	original := *system_setting.GetFetchSetting()
	t.Cleanup(func() { *system_setting.GetFetchSetting() = original })
	system_setting.GetFetchSetting().EnableSSRFProtection = false
}

type temporaryTranslationNetworkError struct{}

func (temporaryTranslationNetworkError) Error() string   { return "temporary network error" }
func (temporaryTranslationNetworkError) Timeout() bool   { return true }
func (temporaryTranslationNetworkError) Temporary() bool { return true }

var _ interface {
	Timeout() bool
	Temporary() bool
} = temporaryTranslationNetworkError{}

type recallEmailTranslationRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn recallEmailTranslationRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
