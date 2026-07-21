package controller

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupStatusMetricsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(append(model.StatusCenterModels(), &model.PerfMetricAvailability{})...))
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	return db
}

func TestStatusFeatureFlagsGuardPublicStatusRoutesDuringShadowMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/status", RequireStatusCenterPublic, func(c *gin.Context) { c.Status(http.StatusNoContent) })

	t.Setenv("STATUS_CENTER_PUBLIC_ENABLED", "false")
	t.Setenv("STATUS_CENTER_SHADOW_MODE", "false")
	disabled := httptest.NewRecorder()
	engine.ServeHTTP(disabled, httptest.NewRequest(http.MethodGet, "/status", nil))
	require.Equal(t, http.StatusServiceUnavailable, disabled.Code)

	t.Setenv("STATUS_CENTER_PUBLIC_ENABLED", "true")
	enabled := httptest.NewRecorder()
	engine.ServeHTTP(enabled, httptest.NewRequest(http.MethodGet, "/status", nil))
	require.Equal(t, http.StatusNoContent, enabled.Code)

	t.Setenv("STATUS_CENTER_SHADOW_MODE", "true")
	shadow := httptest.NewRecorder()
	engine.ServeHTTP(shadow, httptest.NewRequest(http.MethodGet, "/status", nil))
	require.Equal(t, http.StatusServiceUnavailable, shadow.Code)
}

func TestStatusFeatureFlagsGuardSubscriptionRoutesUntilNotificationsAreEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/subscriptions", RequireStatusCenterPublic, RequireStatusCenterNotifications, func(c *gin.Context) { c.Status(http.StatusNoContent) })

	t.Setenv("STATUS_CENTER_PUBLIC_ENABLED", "true")
	t.Setenv("STATUS_CENTER_NOTIFICATIONS_ENABLED", "false")
	t.Setenv("STATUS_CENTER_SHADOW_MODE", "false")
	disabled := httptest.NewRecorder()
	engine.ServeHTTP(disabled, httptest.NewRequest(http.MethodPost, "/subscriptions", nil))
	require.Equal(t, http.StatusServiceUnavailable, disabled.Code)

	t.Setenv("STATUS_CENTER_NOTIFICATIONS_ENABLED", "true")
	enabled := httptest.NewRecorder()
	engine.ServeHTTP(enabled, httptest.NewRequest(http.MethodPost, "/subscriptions", nil))
	require.Equal(t, http.StatusNoContent, enabled.Code)

	t.Setenv("STATUS_CENTER_SHADOW_MODE", "true")
	shadow := httptest.NewRecorder()
	engine.ServeHTTP(shadow, httptest.NewRequest(http.MethodPost, "/subscriptions", nil))
	require.Equal(t, http.StatusServiceUnavailable, shadow.Code)
}

func TestStatusMetricsExposeAggregatesWithoutSecretsOrInternalIdentities(t *testing.T) {
	db := setupStatusMetricsTestDB(t)
	const now = int64(200_000)

	t.Setenv("STATUS_CENTER_ENABLED", "true")
	t.Setenv("STATUS_CENTER_PUBLIC_ENABLED", "true")
	t.Setenv("STATUS_CENTER_NOTIFICATIONS_ENABLED", "true")
	t.Setenv("STATUS_CENTER_SHADOW_MODE", "false")
	t.Setenv("STATUS_SECRET_KEYS", "active:"+base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	t.Setenv("STATUS_SECRET_ACTIVE_KEY_ID", "active")
	t.Setenv("ROUTER_ORIGIN", "https://router-secret.example/internal")

	require.NoError(t, db.Create(&model.StatusJobLease{
		Name: "status-center-scheduler", Holder: "node-holder-secret", ExpiresAt: now + 45,
		FencingToken: 7, UpdatedAt: now - 5,
	}).Error)
	components := []model.StatusComponent{
		{ComponentKey: "router", Slug: "router", Kind: model.StatusComponentKindRouter, DisplayName: "Router Secret", Lifecycle: model.StatusLifecycleActive, EffectiveStatus: model.StatusUnknown, ModelName: "router-internal-secret", LastEvaluatedAt: now - 20, CoverageMicros: 0, UpdatedAt: now - 11, Version: 1},
		{ComponentKey: "model:secret", Slug: "secret", Kind: model.StatusComponentKindModel, DisplayName: "Model Secret", Lifecycle: model.StatusLifecycleActive, EffectiveStatus: model.StatusUnknown, ModelName: "model-internal-secret", LastEvaluatedAt: now - 30, CoverageMicros: 500_000, UpdatedAt: now - 12, Version: 1},
	}
	require.NoError(t, db.Create(&components).Error)
	require.NoError(t, db.Create(&[]model.StatusProbeResult{
		{ComponentID: components[0].ID, Success: true, DiagnosticType: "ok", TargetRef: "probe-target-secret", LatencyMs: 125, CreatedAt: now - 10},
		{ComponentID: components[1].ID, DiagnosticType: "upstream_failure", TargetRef: "probe-endpoint-secret", LatencyMs: 250, CreatedAt: now - 9},
		{ComponentID: components[1].ID, MonitoringFault: true, DiagnosticType: "probe_key_secret", TargetRef: "probe-key-secret", LatencyMs: 500, CreatedAt: now - 8},
	}).Error)
	require.NoError(t, db.Create(&[]model.StatusPeriod{
		{ComponentID: components[0].ID, Granularity: model.StatusGranularityHour, PeriodStart: now - 3_600, WorstStatus: model.StatusUnknown, CreatedAt: now - 3_500, UpdatedAt: now - 3_500},
		{ComponentID: components[0].ID, Granularity: model.StatusGranularityDay, PeriodStart: now - 86_400, WorstStatus: model.StatusUnknown, CreatedAt: now - 80_000, UpdatedAt: now - 80_000},
		{ComponentID: components[1].ID, Granularity: model.StatusGranularityHour, PeriodStart: now - 3_600, WorstStatus: model.StatusUnknown, CreatedAt: now - 3_500, UpdatedAt: now - 3_500},
		{ComponentID: components[1].ID, Granularity: model.StatusGranularityDay, PeriodStart: now - 86_400, WorstStatus: model.StatusUnknown, CreatedAt: now - 80_000, UpdatedAt: now - 80_000},
	}).Error)
	require.NoError(t, db.Create(&model.StatusIncident{
		PublicID: "private-secret", Kind: model.StatusIncidentKindIncident, Title: "draft-title-secret", Impact: model.StatusDegraded,
		Status: "draft", Visibility: "private", AutomationMode: "automatic", IdempotencyKey: "draft-secret-key", Version: 1, CreatedAt: now - 60, UpdatedAt: now - 50,
	}).Error)
	require.NoError(t, db.Create(&model.StatusSubscriber{
		Kind: model.StatusSubscriberKindWebhook, IdentityHash: "subscriber-secret-hash", DisplayAddress: "private@example.com",
		EncryptedEndpoint: "encrypted-endpoint-secret", EncryptedSigningSecret: "encrypted-signing-secret", Status: model.StatusSubscriberSuspended,
		SuspendedAt: now - 20, CreatedAt: now - 100, UpdatedAt: now - 20,
	}).Error)
	require.NoError(t, db.Create(&model.StatusDeliveryOutbox{
		PublishedUpdateID: 1, DestinationType: model.StatusDestinationWebhook, DestinationID: 1, EventID: "event-secret",
		Payload: `{"endpoint":"outbox-endpoint-secret"}`, Status: model.StatusDeliveryPending, Attempts: 2,
		NextAttemptAt: now + 10, LastError: "outbox-error-secret", Version: 1, CreatedAt: now - 40, UpdatedAt: now - 5,
	}).Error)

	text, err := buildStatusCenterPrometheusText(now)
	require.NoError(t, err)
	for _, expected := range []string{
		"newapi_status_center_metrics_up 1",
		`newapi_status_center_feature_enabled{feature="scheduler"} 1`,
		`newapi_status_center_feature_enabled{feature="public"} 1`,
		`newapi_status_center_feature_enabled{feature="notifications"} 1`,
		`newapi_status_center_feature_enabled{feature="shadow"} 0`,
		"newapi_status_center_scheduler_lease_active 1",
		"newapi_status_center_scheduler_lease_remaining_seconds 45",
		"newapi_status_center_evaluator_lag_seconds 30",
		"newapi_status_center_probe_queue_depth",
		`newapi_status_center_probe_results{result="success"} 1`,
		`newapi_status_center_probe_results{result="failure"} 1`,
		`newapi_status_center_probe_results{result="monitoring_fault"} 1`,
		"newapi_status_center_probe_duration_seconds",
		"newapi_status_center_unknown_models 1",
		`newapi_status_center_coverage_components{coverage="zero"} 1`,
		`newapi_status_center_coverage_components{coverage="partial"} 1`,
		`newapi_status_center_rollup_lag_seconds{granularity="hour"} 3600`,
		`newapi_status_center_rollup_lag_seconds{granularity="day"} 86400`,
		"newapi_status_center_incident_drafts 1",
		`newapi_status_center_outbox_depth{status="pending"} 1`,
		"newapi_status_center_outbox_oldest_age_seconds 40",
		"newapi_status_center_outbox_retry_ratio 1",
		"newapi_status_center_suspended_destinations 1",
		"newapi_status_center_keyring_healthy 1",
	} {
		require.Contains(t, text, expected)
	}

	for _, forbidden := range []string{
		"node-holder-secret", "router-secret.example", "router-internal-secret", "model-internal-secret",
		"probe-target-secret", "probe-endpoint-secret", "probe-key-secret", "draft-title-secret", "draft-secret-key",
		"subscriber-secret-hash", "private@example.com", "encrypted-endpoint-secret", "encrypted-signing-secret",
		"event-secret", "outbox-endpoint-secret", "outbox-error-secret", "0123456789abcdef",
	} {
		require.False(t, strings.Contains(text, forbidden), "metrics leaked %q", forbidden)
	}
}

func TestStatusMetricsReportDisabledKeyringWithoutBreakingCoreMetrics(t *testing.T) {
	setupStatusMetricsTestDB(t)
	t.Setenv("STATUS_SECRET_KEYS", "")
	t.Setenv("STATUS_SECRET_ACTIVE_KEY_ID", "")

	text, err := buildStatusCenterPrometheusText(200_000)
	require.NoError(t, err)
	require.Contains(t, text, "newapi_status_center_metrics_up 1")
	require.Contains(t, text, "newapi_status_center_keyring_healthy 0")
	require.True(t, service.StatusNotificationCapabilitiesFor(nil).Email)
}

func TestStatusMetricsExcludeHighTrafficModelsFromProbeQueue(t *testing.T) {
	db := setupStatusMetricsTestDB(t)
	const now = int64(200_000)
	component := model.StatusComponent{
		ComponentKey: "model:busy", Slug: "busy", Kind: model.StatusComponentKindModel,
		ModelName: "busy-model", DisplayName: "Busy", Lifecycle: model.StatusLifecycleActive,
		EffectiveStatus: model.StatusOperational, Version: 1,
	}
	require.NoError(t, db.Create(&component).Error)
	require.NoError(t, db.Create(&model.StatusProbeResult{
		ComponentID: component.ID, Success: true, CreatedAt: now - 16*60,
	}).Error)
	require.NoError(t, db.Create(&model.PerfMetricAvailability{
		ModelName: "busy-model", Group: service.WebsitePublicGroup,
		BucketTs: now - 400, EligibleCount: 20, SuccessCount: 20,
	}).Error)

	text, err := buildStatusCenterPrometheusText(now)
	require.NoError(t, err)
	require.Contains(t, text, "newapi_status_center_probe_queue_depth 0")
}

func TestStatusMetricsCountSignalConflictsInFastProbeQueue(t *testing.T) {
	db := setupStatusMetricsTestDB(t)
	const now = int64(200_000)
	component := model.StatusComponent{
		ComponentKey: "model:conflict", Slug: "conflict", Kind: model.StatusComponentKindModel,
		ModelName: "conflict-model", DisplayName: "Conflict", Lifecycle: model.StatusLifecycleActive,
		EffectiveStatus: model.StatusOperational, Version: 1,
	}
	require.NoError(t, db.Create(&component).Error)
	require.NoError(t, db.Create(&model.StatusProbeResult{
		ComponentID: component.ID, Success: false, CreatedAt: now - 61,
	}).Error)
	require.NoError(t, db.Create(&model.PerfMetricAvailability{
		ModelName: "conflict-model", Group: service.WebsitePublicGroup,
		BucketTs: now - 400, EligibleCount: 1, SuccessCount: 1,
	}).Error)

	text, err := buildStatusCenterPrometheusText(now)
	require.NoError(t, err)
	require.Contains(t, text, "newapi_status_center_probe_queue_depth 1")
}

func TestStatusMetricsRequireRollupsForEveryActiveComponent(t *testing.T) {
	db := setupStatusMetricsTestDB(t)
	const now = int64(200_000)
	components := []model.StatusComponent{
		{ComponentKey: "router", Slug: "router", Kind: model.StatusComponentKindRouter, DisplayName: "Router", Lifecycle: model.StatusLifecycleActive, Version: 1},
		{ComponentKey: "model:missing", Slug: "missing", Kind: model.StatusComponentKindModel, ModelName: "missing", DisplayName: "Missing", Lifecycle: model.StatusLifecycleActive, Version: 1},
	}
	require.NoError(t, db.Create(&components).Error)
	require.NoError(t, db.Create(&model.StatusPeriod{
		ComponentID: components[0].ID, Granularity: model.StatusGranularityHour,
		PeriodStart: now - 3_600, WorstStatus: model.StatusOperational,
	}).Error)

	text, err := buildStatusCenterPrometheusText(now)
	require.NoError(t, err)
	require.Contains(t, text, `newapi_status_center_rollup_ready{granularity="hour"} 0`)
	require.Contains(t, text, `newapi_status_center_rollup_lag_seconds{granularity="hour"} 200000`)
}

func TestStatusMetricsRejectFutureOnlyRollups(t *testing.T) {
	db := setupStatusMetricsTestDB(t)
	const now = int64(200_000)
	component := model.StatusComponent{
		ComponentKey: "router", Slug: "router", Kind: model.StatusComponentKindRouter,
		DisplayName: "Router", Lifecycle: model.StatusLifecycleActive, Version: 1,
	}
	require.NoError(t, db.Create(&component).Error)
	require.NoError(t, db.Create(&model.StatusPeriod{
		ComponentID: component.ID, Granularity: model.StatusGranularityHour,
		PeriodStart: now + 3_600, WorstStatus: model.StatusOperational,
	}).Error)

	text, err := buildStatusCenterPrometheusText(now)
	require.NoError(t, err)
	require.Contains(t, text, `newapi_status_center_rollup_ready{granularity="hour"} 0`)
	require.Contains(t, text, `newapi_status_center_rollup_lag_seconds{granularity="hour"} 200000`)
}

func TestStatusMetricsUseLatestNonFutureRollup(t *testing.T) {
	db := setupStatusMetricsTestDB(t)
	const now = int64(200_000)
	component := model.StatusComponent{
		ComponentKey: "router", Slug: "router", Kind: model.StatusComponentKindRouter,
		DisplayName: "Router", Lifecycle: model.StatusLifecycleActive, Version: 1,
	}
	require.NoError(t, db.Create(&component).Error)
	require.NoError(t, db.Create(&[]model.StatusPeriod{
		{ComponentID: component.ID, Granularity: model.StatusGranularityHour, PeriodStart: now - 3_600, WorstStatus: model.StatusOperational},
		{ComponentID: component.ID, Granularity: model.StatusGranularityHour, PeriodStart: now + 3_600, WorstStatus: model.StatusOperational},
	}).Error)

	text, err := buildStatusCenterPrometheusText(now)
	require.NoError(t, err)
	require.Contains(t, text, `newapi_status_center_rollup_ready{granularity="hour"} 1`)
	require.Contains(t, text, `newapi_status_center_rollup_lag_seconds{granularity="hour"} 3600`)
}

func TestStatusMetricsBoundProbeAndOutboxSeries(t *testing.T) {
	db := setupStatusMetricsTestDB(t)
	const now = int64(200_000)
	component := model.StatusComponent{
		ComponentKey: "router", Slug: "router", Kind: model.StatusComponentKindRouter,
		DisplayName: "Router", Lifecycle: model.StatusLifecycleActive, Version: 1,
	}
	require.NoError(t, db.Create(&component).Error)
	require.NoError(t, db.Create(&[]model.StatusProbeResult{
		{ComponentID: component.ID, Success: true, LatencyMs: 100, CreatedAt: now - 30},
		{ComponentID: component.ID, Success: false, LatencyMs: 900, CreatedAt: now - 3_601},
		{ComponentID: component.ID, Success: false, LatencyMs: 800, CreatedAt: now + 1},
	}).Error)
	require.NoError(t, db.Create(&[]model.StatusDeliveryOutbox{
		{PublishedUpdateID: 1, DestinationType: model.StatusDestinationEmail, DestinationID: 1, EventID: "pending", Payload: "{}", Status: model.StatusDeliveryPending, Attempts: 1, Version: 1, CreatedAt: now - 40},
		{PublishedUpdateID: 2, DestinationType: model.StatusDestinationEmail, DestinationID: 2, EventID: "delivered", Payload: "{}", Status: model.StatusDeliveryDelivered, Attempts: 3, Version: 1, CreatedAt: now - 80_000},
		{PublishedUpdateID: 3, DestinationType: model.StatusDestinationEmail, DestinationID: 3, EventID: "dead", Payload: "{}", Status: model.StatusDeliveryDead, Attempts: 5, Version: 1, CreatedAt: now - 90_000},
	}).Error)

	text, err := buildStatusCenterPrometheusText(now)
	require.NoError(t, err)
	require.Contains(t, text, "newapi_status_center_probe_requests 1")
	require.Contains(t, text, `newapi_status_center_probe_results{result="success"} 1`)
	require.Contains(t, text, `newapi_status_center_probe_results{result="failure"} 0`)
	require.Contains(t, text, `newapi_status_center_outbox_depth{status="pending"} 1`)
	require.NotContains(t, text, `newapi_status_center_outbox_depth{status="delivered"}`)
	require.NotContains(t, text, `newapi_status_center_outbox_depth{status="dead"}`)
	require.Contains(t, text, "newapi_status_center_outbox_dead 1")
	require.Contains(t, text, "newapi_status_center_outbox_retry_ratio 1")
	require.NotContains(t, text, "newapi_status_center_component_sync_lag_seconds")
}

func TestStatusMetricsAreOnlyEmittedByMasterNodes(t *testing.T) {
	setupStatusMetricsTestDB(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/metrics", GetPrometheusMetrics)

	originalMaster := common.IsMasterNode
	t.Cleanup(func() { common.IsMasterNode = originalMaster })
	t.Setenv("STATUS_CENTER_ENABLED", "true")

	common.IsMasterNode = false
	slave := httptest.NewRecorder()
	engine.ServeHTTP(slave, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, slave.Code)
	require.NotContains(t, slave.Body.String(), "newapi_status_center_feature_enabled")

	common.IsMasterNode = true
	master := httptest.NewRecorder()
	engine.ServeHTTP(master, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, master.Code)
	require.Contains(t, master.Body.String(), "newapi_status_center_feature_enabled")
}
