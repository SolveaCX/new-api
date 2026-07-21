package controller_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	backendI18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/router"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupStatusHTTPTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	t.Setenv("STATUS_CENTER_PUBLIC_ENABLED", "true")
	t.Setenv("STATUS_CENTER_NOTIFICATIONS_ENABLED", "true")
	t.Setenv("STATUS_CENTER_SHADOW_MODE", "false")
	require.NoError(t, backendI18n.Init())
	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.StatusCenterModels()...))
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	engine := gin.New()
	engine.Use(sessions.Sessions("status-test", cookie.NewStore([]byte("status-http-test-secret"))))
	router.SetApiRouter(engine)
	return engine, db
}

func performStatusRequest(engine http.Handler, method string, target string, body string, headers map[string]string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	engine.ServeHTTP(recorder, request)
	return recorder
}

func decodeStatusResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func statusResponseData(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	response := decodeStatusResponse(t, recorder)
	require.Equal(t, true, response["success"])
	data, ok := response["data"].(map[string]any)
	require.True(t, ok, "response data is not an object: %#v", response["data"])
	return data
}

func assertStatusMetadata(t *testing.T, data map[string]any) {
	t.Helper()
	require.NotZero(t, data["generated_at"])
	require.Contains(t, data, "last_trustworthy_update_at")
	require.Contains(t, data, "coverage")
}

func TestStatusPublicSummaryUsesHonestMetadataAllowlistAndConditionalCaching(t *testing.T) {
	engine, db := setupStatusHTTPTest(t)
	now := time.Now().Unix()
	require.NoError(t, db.Create(&model.StatusComponent{
		ComponentKey: "router", Slug: "router", Kind: model.StatusComponentKindRouter,
		DisplayName: "Public Router", Lifecycle: model.StatusLifecycleActive,
		ObservedStatus: model.StatusOperational, EffectiveStatus: model.StatusOperational,
		StatusSource: "traffic", LastEvidenceAt: now - 3600, LastTrustworthyUpdateAt: now - 3600,
		LastEvaluatedAt: now - 3600, CoverageMicros: 1_000_000,
		OverrideReason: "secret customer traffic note", Version: 7, CreatedAt: now - 7200, UpdatedAt: now - 3600,
	}).Error)

	first := performStatusRequest(engine, http.MethodGet, "/api/status/summary", "", nil)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	require.Equal(t, "public, max-age=30", first.Header().Get("Cache-Control"))
	require.NotEmpty(t, first.Header().Get("ETag"))
	data := statusResponseData(t, first)
	assertStatusMetadata(t, data)
	require.Equal(t, service.OverallMonitoringIncomplete, data["status"])
	require.Equal(t, "monitoring unavailable", data["message"])

	body := first.Body.String()
	for _, forbidden := range []string{
		`"model_name"`, `"observed_status"`, `"status_source"`, `"override_reason"`,
		`"override_by"`, `"version"`, `"channel"`, `"provider"`, `"raw_error"`, `"traffic"`, `"customer"`,
	} {
		require.NotContains(t, body, forbidden)
	}
	require.NotContains(t, body, "secret customer traffic note")

	second := performStatusRequest(engine, http.MethodGet, "/api/status/summary", "", map[string]string{
		"If-None-Match": first.Header().Get("ETag"),
	})
	require.Equal(t, http.StatusNotModified, second.Code)
	require.Empty(t, second.Body.String())

	unknown := performStatusRequest(engine, http.MethodGet, "/api/status/components?status=unknown", "", nil)
	require.Equal(t, http.StatusOK, unknown.Code, unknown.Body.String())
	unknownData := statusResponseData(t, unknown)
	unknownComponents := unknownData["components"].([]any)
	require.Len(t, unknownComponents, 1)
	require.Equal(t, "router", unknownComponents[0].(map[string]any)["slug"])
}

func TestStatusPublicComponentsDetailHistoryFiltersAndRangeValidation(t *testing.T) {
	engine, db := setupStatusHTTPTest(t)
	now := time.Now().Unix()
	components := []model.StatusComponent{
		{ComponentKey: "router", Slug: "router", Kind: model.StatusComponentKindRouter, DisplayName: "Router", Lifecycle: model.StatusLifecycleActive, ObservedStatus: model.StatusOperational, EffectiveStatus: model.StatusOperational, LastTrustworthyUpdateAt: now, LastEvaluatedAt: now, CoverageMicros: 1_000_000, Version: 1},
		{ComponentKey: "model:gpt-test", Slug: "gpt-test", Kind: model.StatusComponentKindModel, ModelName: "gpt-test-internal", DisplayName: "GPT Test", Capability: "text", Lifecycle: model.StatusLifecycleActive, ObservedStatus: model.StatusDegraded, EffectiveStatus: model.StatusDegraded, LastTrustworthyUpdateAt: now, LastEvaluatedAt: now, CoverageMicros: 800_000, Version: 1},
		{ComponentKey: "model:image-test", Slug: "image-test", Kind: model.StatusComponentKindModel, ModelName: "image-test-internal", DisplayName: "Image Test", Capability: "image", Lifecycle: model.StatusLifecycleActive, ObservedStatus: model.StatusOperational, EffectiveStatus: model.StatusOperational, LastTrustworthyUpdateAt: now, LastEvaluatedAt: now, CoverageMicros: 900_000, Version: 1},
	}
	for index := range components {
		require.NoError(t, db.Create(&components[index]).Error)
	}
	require.NoError(t, db.Create(&model.StatusPeriod{
		ComponentID: components[1].ID, Granularity: model.StatusGranularityHour,
		PeriodStart: now - 3600, ScoreSumMicros: 990_000, KnownBucketCount: 1,
		UnknownBucketCount: 1, WorstStatus: model.StatusDegraded,
	}).Error)

	list := performStatusRequest(engine, http.MethodGet, "/api/status/components?kind=model&query=gpt&capability=text&status=degraded", "", nil)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	listData := statusResponseData(t, list)
	assertStatusMetadata(t, listData)
	listed := listData["components"].([]any)
	require.Len(t, listed, 1)
	require.Equal(t, "gpt-test", listed[0].(map[string]any)["slug"])
	require.NotContains(t, list.Body.String(), `"model_name"`)

	detail := performStatusRequest(engine, http.MethodGet, "/api/status/components/gpt-test", "", nil)
	require.Equal(t, http.StatusOK, detail.Code, detail.Body.String())
	detailData := statusResponseData(t, detail)
	assertStatusMetadata(t, detailData)
	require.Equal(t, "gpt-test", detailData["component"].(map[string]any)["slug"])

	history := performStatusRequest(engine, http.MethodGet, "/api/status/components/gpt-test/history?range=24h", "", nil)
	require.Equal(t, http.StatusOK, history.Code, history.Body.String())
	historyData := statusResponseData(t, history)
	assertStatusMetadata(t, historyData)
	require.Equal(t, "24h", historyData["range"])
	require.NotEmpty(t, historyData["periods"])

	invalid := performStatusRequest(engine, http.MethodGet, "/api/status/components/gpt-test/history?range=1y", "", nil)
	require.Equal(t, http.StatusBadRequest, invalid.Code, invalid.Body.String())
	require.Contains(t, strings.ToLower(invalid.Body.String()), "range")
}

func TestStatusPublicIncidentsAndMaintenanceExposePublishedSafeFieldsOnly(t *testing.T) {
	engine, db := setupStatusHTTPTest(t)
	now := time.Now().Unix()
	incidents := []model.StatusIncident{
		{PublicID: "inc_public", Kind: model.StatusIncidentKindIncident, Title: "Public incident", Impact: "degraded", Status: "monitoring", Visibility: "public", AutomationMode: "manual", IdempotencyKey: "public-incident", Version: 2, CreatedAt: now - 100, UpdatedAt: now - 10},
		{PublicID: "inc_private", Kind: model.StatusIncidentKindIncident, Title: "Private incident", Impact: "outage", Status: "draft", Visibility: "private", AutomationMode: "automatic", IdempotencyKey: "private-incident", Version: 1, CreatedAt: now - 100, UpdatedAt: now - 10},
		{PublicID: "mnt_public", Kind: model.StatusIncidentKindMaintenance, Title: "Public maintenance", Impact: "maintenance", Status: "identified", Visibility: "public", AutomationMode: "manual", IdempotencyKey: "public-maintenance", ScheduledStartAt: now + 3600, ScheduledEndAt: now + 7200, Version: 1, CreatedAt: now - 100, UpdatedAt: now - 10},
	}
	for index := range incidents {
		require.NoError(t, db.Create(&incidents[index]).Error)
	}
	require.NoError(t, db.Create(&model.StatusIncidentUpdate{
		IncidentID: incidents[0].ID, EventID: "evt_public", State: "monitoring",
		Body: "Service recovery is being monitored.", Published: true, PublishedAt: now - 10, ActorID: 999, CreatedAt: now - 10,
	}).Error)
	require.NoError(t, db.Create(&model.StatusIncidentUpdate{
		IncidentID: incidents[0].ID, EventID: "draft_secret", State: "identified",
		Body: "raw error provider secret", Published: false, ActorID: 999, CreatedAt: now - 20,
	}).Error)

	list := performStatusRequest(engine, http.MethodGet, "/api/status/incidents", "", nil)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	listData := statusResponseData(t, list)
	assertStatusMetadata(t, listData)
	require.Contains(t, list.Body.String(), "inc_public")
	require.NotContains(t, list.Body.String(), "inc_private")
	require.NotContains(t, list.Body.String(), "raw error provider secret")
	require.NotContains(t, list.Body.String(), `"actor_id"`)
	require.NotContains(t, list.Body.String(), `"idempotency_key"`)

	detail := performStatusRequest(engine, http.MethodGet, "/api/status/incidents/inc_public", "", nil)
	require.Equal(t, http.StatusOK, detail.Code, detail.Body.String())
	detailData := statusResponseData(t, detail)
	assertStatusMetadata(t, detailData)
	require.Contains(t, detail.Body.String(), "Service recovery is being monitored.")
	require.NotContains(t, detail.Body.String(), "draft_secret")

	maintenance := performStatusRequest(engine, http.MethodGet, "/api/status/maintenance", "", nil)
	require.Equal(t, http.StatusOK, maintenance.Code, maintenance.Body.String())
	maintenanceData := statusResponseData(t, maintenance)
	assertStatusMetadata(t, maintenanceData)
	require.Contains(t, maintenance.Body.String(), "mnt_public")
	require.NotContains(t, maintenance.Body.String(), "inc_public")
}

func TestStatusPublicSubscriptionsBoundBodiesUseGenericRepliesAndGETDoesNotMutate(t *testing.T) {
	engine, db := setupStatusHTTPTest(t)
	now := time.Now().Unix()
	manageToken := "manage-token-value"
	subscriber := model.StatusSubscriber{
		Kind: model.StatusSubscriberKindEmail, IdentityHash: "subscriber-hash", DisplayAddress: "u***@example.com",
		Status: model.StatusSubscriberActive, ManageTokenHash: service.HashStatusToken(manageToken),
		CreatedAt: now - 100, UpdatedAt: now - 100,
	}
	require.NoError(t, db.Create(&subscriber).Error)

	oversized := fmt.Sprintf(`{"email":"%s"}`, strings.Repeat("a", 8*1024))
	tooLarge := performStatusRequest(engine, http.MethodPost, "/api/status/subscriptions", oversized, nil)
	require.Equal(t, http.StatusRequestEntityTooLarge, tooLarge.Code, tooLarge.Body.String())
	require.Contains(t, tooLarge.Body.String(), service.StatusSubscriptionGenericMessage)
	require.NotContains(t, tooLarge.Body.String(), strings.Repeat("a", 32))

	invalid := performStatusRequest(engine, http.MethodPost, "/api/status/subscriptions", `{"email":"not-an-email"}`, nil)
	require.Equal(t, http.StatusOK, invalid.Code, invalid.Body.String())
	require.Contains(t, invalid.Body.String(), service.StatusSubscriptionGenericMessage)
	require.NotContains(t, invalid.Body.String(), "invalid email")

	preview := performStatusRequest(engine, http.MethodGet, "/api/status/subscriptions/unsubscribe?token="+manageToken, "", nil)
	require.Equal(t, http.StatusOK, preview.Code, preview.Body.String())
	require.Contains(t, preview.Body.String(), service.StatusSubscriptionGenericMessage)
	var stored model.StatusSubscriber
	require.NoError(t, db.First(&stored, subscriber.ID).Error)
	require.Equal(t, model.StatusSubscriberActive, stored.Status)

	unsubscribed := performStatusRequest(engine, http.MethodPost, "/api/status/subscriptions/unsubscribe", fmt.Sprintf(`{"token":%q}`, manageToken), nil)
	require.Equal(t, http.StatusOK, unsubscribed.Code, unsubscribed.Body.String())
	require.Contains(t, unsubscribed.Body.String(), service.StatusSubscriptionGenericMessage)
	require.NoError(t, db.First(&stored, subscriber.ID).Error)
	require.Equal(t, model.StatusSubscriberUnsubscribed, stored.Status)

	verify := performStatusRequest(engine, http.MethodGet, "/api/status/subscriptions/verify?token=unknown", "", nil)
	require.Equal(t, http.StatusOK, verify.Code, verify.Body.String())
	require.Contains(t, verify.Body.String(), service.StatusSubscriptionGenericMessage)

	malformed := performStatusRequest(engine, http.MethodPost, "/api/status/subscriptions/unsubscribe", string(bytes.Repeat([]byte("{"), 20)), nil)
	require.Equal(t, http.StatusOK, malformed.Code, malformed.Body.String())
	require.Contains(t, malformed.Body.String(), service.StatusSubscriptionGenericMessage)
}
