package model

import (
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type panicReadCloser struct {
	readCalled bool
}

func (p *panicReadCloser) Read(_ []byte) (int, error) {
	p.readCalled = true
	return 0, errors.New("body should not be read")
}

func (p *panicReadCloser) Close() error {
	return nil
}

func configureRequestSamplingForTest(t *testing.T, enabled bool, rate float64, groups []string) {
	t.Helper()
	original := operation_setting.GetLogRequestSamplingSetting()
	operation_setting.UpdateLogRequestSamplingSetting(func(setting *operation_setting.LogRequestSamplingSetting) {
		setting.Enabled = enabled
		setting.SampleRate = rate
		setting.Groups = append([]string(nil), groups...)
	})
	t.Cleanup(func() {
		operation_setting.UpdateLogRequestSamplingSetting(func(setting *operation_setting.LogRequestSamplingSetting) {
			*setting = original
		})
	})

	oldRunner := logRequestSampleAsyncRunner
	oldRandom := logRequestSampleRandomFloat
	logRequestSampleAsyncRunner = func(fn func()) {
		fn()
	}
	logRequestSampleRandomFloat = func() float64 {
		return 0
	}
	t.Cleanup(func() {
		logRequestSampleAsyncRunner = oldRunner
		logRequestSampleRandomFloat = oldRandom
	})
}

func newRequestSamplingContext(method string, path string, body io.Reader) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, body)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("username", "sample-user")
	c.Set(common.RequestIdKey, "req_sample_1")
	common.SetContextKey(c, constant.ContextKeyRequestSamplingEligible, true)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "plg")
	return c
}

func countRequestSamples(t *testing.T) int64 {
	t.Helper()
	var count int64
	require.NoError(t, LOG_DB.Model(&LogRequestSample{}).Count(&count).Error)
	return count
}

func TestRecordConsumeLogRequestSamplingDisabledDoesNotReadBody(t *testing.T) {
	truncateTables(t)
	configureRequestSamplingForTest(t, false, 1, []string{"plg"})
	body := &panicReadCloser{}
	c := newRequestSamplingContext("POST", "/v1/chat/completions", body)
	c.Request.Body = body

	RecordConsumeLog(c, 123, RecordConsumeLogParams{
		ModelName:      "gpt-4o",
		TokenName:      "api-token",
		TokenId:        9,
		UseTimeSeconds: 1,
		Group:          "enterprise",
		Other:          map[string]interface{}{"note": "keep"},
	})

	if body.readCalled {
		t.Fatal("disabled sampling path read request body")
	}
	require.Equal(t, int64(0), countRequestSamples(t))
}

func TestRecordConsumeLogRequestSamplingWritesRedactedSampleForEligiblePLG(t *testing.T) {
	truncateTables(t)
	configureRequestSamplingForTest(t, true, 1, []string{"plg"})
	body := strings.NewReader(`{
		"model":"gpt-4o",
		"messages":[{"role":"user","content":"hello"}],
		"api_key":"sk-secret",
		"x-api-key":"AIzaSyA000000000000000000000000000000",
		"proxy-authorization":"Bearer plain-oauth-token-value",
		"aws_key":"AKIAABCDEFGHIJKLMNOP",
		"private-key":"BEGIN PRIVATE KEY",
		"accessToken":"opaque-access-token",
		"clientSecret":"plain-client-secret",
		"privateKey":"plain-private-key",
		"cookie":"sid=secret",
		"session_id":"session-secret",
		"service_credentials":"credential-secret",
		"note":"Authorization Bearer abcdefghijklmnop and jwt eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature",
		"url":"https://user:pass@example.com/path?token=abc&safe=ok",
		"image_url":"data:image/png;base64,AAAA"
	}`)
	c := newRequestSamplingContext("POST", "/v1/chat/completions", body)

	RecordConsumeLog(c, 123, RecordConsumeLogParams{
		ModelName:        "gpt-4o",
		TokenName:        "api-token",
		TokenId:          9,
		PromptTokens:     3,
		CompletionTokens: 4,
		UseTimeSeconds:   1,
		Group:            "enterprise",
		Other:            map[string]interface{}{"note": "keep"},
	})

	var log Log
	require.NoError(t, LOG_DB.First(&log).Error)
	require.NotZero(t, log.Id)
	require.NotContains(t, log.Other, "hello")

	var sample LogRequestSample
	require.NoError(t, LOG_DB.Where("log_id = ?", log.Id).First(&sample).Error)
	require.Equal(t, log.Id, sample.LogId)
	require.Equal(t, "req_sample_1", sample.RequestId)
	require.Equal(t, 123, sample.UserId)
	require.Equal(t, 9, sample.TokenId)
	require.Equal(t, "plg", sample.UserGroup)
	require.Equal(t, "/v1/chat/completions", sample.RequestPath)
	require.Equal(t, "gpt-4o", sample.ModelName)
	require.NotZero(t, sample.RequestBodySize)
	require.Equal(t, 1.0, sample.SampleRate)
	require.NotEmpty(t, sample.RedactionVersion)

	var payload map[string]interface{}
	require.NoError(t, common.Unmarshal([]byte(sample.RequestParams), &payload))
	require.Equal(t, "[redacted]", payload["api_key"])
	require.Equal(t, "[redacted]", payload["x-api-key"])
	require.Equal(t, "[redacted]", payload["proxy-authorization"])
	require.Equal(t, "[redacted]", payload["private-key"])
	require.Equal(t, "[redacted]", payload["accessToken"])
	require.Equal(t, "[redacted]", payload["clientSecret"])
	require.Equal(t, "[redacted]", payload["privateKey"])
	require.Equal(t, "[redacted]", payload["cookie"])
	require.Equal(t, "[redacted]", payload["session_id"])
	require.Equal(t, "[redacted]", payload["service_credentials"])
	require.Contains(t, sample.RequestParams, "hello")
	require.NotContains(t, sample.RequestParams, "sk-secret")
	require.NotContains(t, sample.RequestParams, "AIza")
	require.NotContains(t, sample.RequestParams, "AKIA")
	require.NotContains(t, sample.RequestParams, "plain-oauth-token-value")
	require.NotContains(t, sample.RequestParams, "BEGIN PRIVATE KEY")
	require.NotContains(t, sample.RequestParams, "opaque-access-token")
	require.NotContains(t, sample.RequestParams, "plain-client-secret")
	require.NotContains(t, sample.RequestParams, "plain-private-key")
	require.NotContains(t, sample.RequestParams, "sid=secret")
	require.NotContains(t, sample.RequestParams, "session-secret")
	require.NotContains(t, sample.RequestParams, "credential-secret")
	require.NotContains(t, sample.RequestParams, "abcdefghijklmnop")
	require.NotContains(t, sample.RequestParams, "eyJhbGci")
	require.NotContains(t, sample.RequestParams, "user:pass")
	require.NotContains(t, sample.RequestParams, "token=abc")
	require.NotContains(t, sample.RequestParams, "example.com")
	require.NotContains(t, sample.RequestParams, "safe=ok")
	require.NotContains(t, sample.RequestParams, "data:image/png;base64")
}

func TestRecordConsumeLogRequestSamplingSkipsNonMatchingGroupInvalidJSONAndViolationFee(t *testing.T) {
	truncateTables(t)
	configureRequestSamplingForTest(t, true, 1, []string{"plg"})

	c := newRequestSamplingContext("POST", "/v1/chat/completions", strings.NewReader(`{"messages":[{"content":"hi"}]}`))
	common.SetContextKey(c, constant.ContextKeyUserGroup, "enterprise")
	RecordConsumeLog(c, 123, RecordConsumeLogParams{ModelName: "gpt-4o", Other: map[string]interface{}{}})

	c = newRequestSamplingContext("POST", "/v1/chat/completions", strings.NewReader(`{bad json`))
	RecordConsumeLog(c, 123, RecordConsumeLogParams{ModelName: "gpt-4o", Other: map[string]interface{}{}})

	c = newRequestSamplingContext("POST", "/v1/chat/completions", strings.NewReader(`{"messages":[{"content":"hi"}]}`))
	RecordConsumeLog(c, 123, RecordConsumeLogParams{ModelName: "gpt-4o", Other: map[string]interface{}{"violation_fee": true}})

	require.Equal(t, int64(0), countRequestSamples(t))
}

func TestRecordConsumeLogRequestSamplingSkipsGeminiNonTextActions(t *testing.T) {
	truncateTables(t)
	configureRequestSamplingForTest(t, true, 1, []string{"plg"})

	c := newRequestSamplingContext("POST", "/v1beta/models/gemini-2.5-pro:embedContent", strings.NewReader(`{"content":{"parts":[{"text":"hi"}]}}`))
	RecordConsumeLog(c, 123, RecordConsumeLogParams{ModelName: "gemini-2.5-pro", Other: map[string]interface{}{}})
	require.Equal(t, int64(0), countRequestSamples(t))

	c = newRequestSamplingContext("POST", "/v1beta/models/gemini-2.5-pro:generateContent", strings.NewReader(`{"contents":[{"parts":[{"text":"hi"}]}]}`))
	RecordConsumeLog(c, 123, RecordConsumeLogParams{ModelName: "gemini-2.5-pro", Other: map[string]interface{}{}})
	require.Equal(t, int64(1), countRequestSamples(t))

	c = newRequestSamplingContext("POST", "/v1/models/gemini-2.5-pro:embedContent", strings.NewReader(`{"content":{"parts":[{"text":"hi"}]}}`))
	RecordConsumeLog(c, 123, RecordConsumeLogParams{ModelName: "gemini-2.5-pro", Other: map[string]interface{}{}})
	require.Equal(t, int64(1), countRequestSamples(t))

	c = newRequestSamplingContext("POST", "/v1/models/gemini-2.5-pro:generateContent", strings.NewReader(`{"contents":[{"parts":[{"text":"hi"}]}]}`))
	RecordConsumeLog(c, 123, RecordConsumeLogParams{ModelName: "gemini-2.5-pro", Other: map[string]interface{}{}})
	require.Equal(t, int64(2), countRequestSamples(t))
}

func TestRecordConsumeLogRequestSamplingRestoresBodyPosition(t *testing.T) {
	truncateTables(t)
	configureRequestSamplingForTest(t, true, 1, []string{"plg"})
	c := newRequestSamplingContext("POST", "/v1/responses", strings.NewReader(`{"input":"hello"}`))

	RecordConsumeLog(c, 123, RecordConsumeLogParams{ModelName: "gpt-4o", Other: map[string]interface{}{}})

	storage, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	body, err := storage.Bytes()
	require.NoError(t, err)
	require.JSONEq(t, `{"input":"hello"}`, string(body))
	pos, err := storage.Seek(0, io.SeekCurrent)
	require.NoError(t, err)
	require.Equal(t, int64(0), pos)
}

func TestCleanupOldLogRequestSamplesDeletesSamplesOnly(t *testing.T) {
	truncateTables(t)
	now := time.Now().Unix()
	require.NoError(t, LOG_DB.Create(&Log{Id: 1, CreatedAt: now, Type: LogTypeConsume}).Error)
	require.NoError(t, LOG_DB.Create(&LogRequestSample{LogId: 1, CreatedAt: now - 100, RequestParams: `{}`}).Error)
	require.NoError(t, LOG_DB.Create(&LogRequestSample{LogId: 2, CreatedAt: now, RequestParams: `{}`}).Error)

	deleted, err := CleanupOldLogRequestSamples(now-10, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)

	var logCount int64
	require.NoError(t, LOG_DB.Model(&Log{}).Count(&logCount).Error)
	require.Equal(t, int64(1), logCount)
	require.Equal(t, int64(1), countRequestSamples(t))
}

func TestCleanupOldLogRequestSamplesWithBatchLimitStopsAfterLimit(t *testing.T) {
	truncateTables(t)
	now := time.Now().Unix()
	for i := 1; i <= 5; i++ {
		require.NoError(t, LOG_DB.Create(&LogRequestSample{LogId: i, CreatedAt: now - 100, RequestParams: `{}`}).Error)
	}

	deleted, err := CleanupOldLogRequestSamplesWithBatchLimit(now-10, 2, 2)
	require.NoError(t, err)
	require.Equal(t, int64(4), deleted)
	require.Equal(t, int64(1), countRequestSamples(t))
}

func TestDeleteOldLogDeletesMatchingRequestSamples(t *testing.T) {
	truncateTables(t)
	require.NoError(t, LOG_DB.Create(&Log{Id: 1, CreatedAt: 100, Type: LogTypeConsume}).Error)
	require.NoError(t, LOG_DB.Create(&Log{Id: 2, CreatedAt: 300, Type: LogTypeConsume}).Error)
	require.NoError(t, LOG_DB.Create(&LogRequestSample{LogId: 1, CreatedAt: 100, RequestParams: `{}`}).Error)
	require.NoError(t, LOG_DB.Create(&LogRequestSample{LogId: 2, CreatedAt: 300, RequestParams: `{}`}).Error)

	deleted, err := DeleteOldLog(t.Context(), 200, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)

	var oldSampleCount int64
	require.NoError(t, LOG_DB.Model(&LogRequestSample{}).Where("log_id = ?", 1).Count(&oldSampleCount).Error)
	require.Equal(t, int64(0), oldSampleCount)
	require.Equal(t, int64(1), countRequestSamples(t))
}
