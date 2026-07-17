package controller_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func statusAuthCookie(t *testing.T, engine *gin.Engine, role int, secure bool) *http.Cookie {
	t.Helper()
	path := fmt.Sprintf("/__status_test_login/%d/%t", role, secure)
	engine.GET(path, func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "status-admin")
		session.Set("role", role)
		session.Set("id", 7001)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		if secure {
			session.Set(middleware.SecureVerificationSessionKey, time.Now().Unix())
		}
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusNoContent, recorder.Code)
	response := recorder.Result()
	cookies := response.Cookies()
	require.NotEmpty(t, cookies)
	return cookies[0]
}

func performStatusAuthenticatedRequest(engine http.Handler, method string, target string, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("New-Api-User", "7001")
	request.AddCookie(cookie)
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestStatusAdminOrdinaryRoutesRequireAdminAndSensitiveRoutesRequireRootSecureVerification(t *testing.T) {
	engine, _ := setupStatusHTTPTest(t)

	unauthenticated := performStatusRequest(engine, http.MethodGet, "/api/status/admin/incidents", "", nil)
	require.Equal(t, http.StatusUnauthorized, unauthenticated.Code)

	adminCookie := statusAuthCookie(t, engine, common.RoleAdminUser, false)
	ordinary := performStatusAuthenticatedRequest(engine, http.MethodGet, "/api/status/admin/incidents", "", adminCookie)
	require.Equal(t, http.StatusOK, ordinary.Code, ordinary.Body.String())
	require.Equal(t, true, decodeStatusResponse(t, ordinary)["success"])

	adminForceGreen := performStatusAuthenticatedRequest(engine, http.MethodPost, "/api/status/admin/overrides/force-green", `{}`, adminCookie)
	require.Equal(t, http.StatusOK, adminForceGreen.Code)
	require.Equal(t, false, decodeStatusResponse(t, adminForceGreen)["success"])

	rootCookie := statusAuthCookie(t, engine, common.RoleRootUser, false)
	rootWithoutVerification := performStatusAuthenticatedRequest(engine, http.MethodPost, "/api/status/admin/discord/test", `{}`, rootCookie)
	require.Equal(t, http.StatusForbidden, rootWithoutVerification.Code)
	require.NotContains(t, rootWithoutVerification.Body.String(), "webhook_endpoint")

	verifiedRootCookie := statusAuthCookie(t, engine, common.RoleRootUser, true)
	verifiedRoot := performStatusAuthenticatedRequest(engine, http.MethodPost, "/api/status/admin/discord/test", `{}`, verifiedRootCookie)
	require.NotEqual(t, http.StatusNotFound, verifiedRoot.Code)
	require.NotContains(t, verifiedRoot.Body.String(), "STATUS_SECRET_KEYS")
	require.NotContains(t, verifiedRoot.Body.String(), "webhook_endpoint")
}

func TestStatusAdminIncidentAndOverrideVersionConflictsReturn409(t *testing.T) {
	engine, db := setupStatusHTTPTest(t)
	now := time.Now().Unix()
	component := model.StatusComponent{
		ComponentKey: "model:gpt-conflict", Slug: "gpt-conflict", Kind: model.StatusComponentKindModel,
		DisplayName: "GPT Conflict", Lifecycle: model.StatusLifecycleActive,
		ObservedStatus: model.StatusDegraded, EffectiveStatus: model.StatusDegraded,
		LastTrustworthyUpdateAt: now, LastEvaluatedAt: now, CoverageMicros: 1_000_000, Version: 2,
	}
	require.NoError(t, db.Create(&component).Error)
	incident := model.StatusIncident{
		PublicID: "inc_conflict", Kind: model.StatusIncidentKindIncident, Title: "Conflict",
		Impact: "degraded", Status: "draft", Visibility: "private", AutomationMode: "manual",
		IdempotencyKey: "incident-conflict", Version: 2, CreatedAt: now - 10, UpdatedAt: now - 10,
	}
	require.NoError(t, db.Create(&incident).Error)
	adminCookie := statusAuthCookie(t, engine, common.RoleAdminUser, false)

	publishBody := `{"expected_version":1,"state":"investigating","body":"Investigating safely.","event_id":"evt-conflict","reason":"operator publish","destinations":[]}`
	publish := performStatusAuthenticatedRequest(engine, http.MethodPost, "/api/status/admin/incidents/"+strconv.FormatInt(incident.ID, 10)+"/publish", publishBody, adminCookie)
	require.Equal(t, http.StatusConflict, publish.Code, publish.Body.String())
	require.Equal(t, false, decodeStatusResponse(t, publish)["success"])

	overrideBody := fmt.Sprintf(`{"component_id":%d,"expected_version":1,"status":"degraded","reason":"operator assessment","expires_at":%d}`, component.ID, now+600)
	override := performStatusAuthenticatedRequest(engine, http.MethodPost, "/api/status/admin/overrides", overrideBody, adminCookie)
	require.Equal(t, http.StatusConflict, override.Code, override.Body.String())
	require.Equal(t, false, decodeStatusResponse(t, override)["success"])
}

func TestStatusAdminMaintenanceValidationAndReadSurfaces(t *testing.T) {
	engine, _ := setupStatusHTTPTest(t)
	adminCookie := statusAuthCookie(t, engine, common.RoleAdminUser, false)

	invalidMaintenance := performStatusAuthenticatedRequest(engine, http.MethodPost, "/api/status/admin/maintenance", `{"title":"missing window"}`, adminCookie)
	require.Equal(t, http.StatusBadRequest, invalidMaintenance.Code, invalidMaintenance.Body.String())

	for _, path := range []string{
		"/api/status/admin/incidents",
		"/api/status/admin/maintenance",
		"/api/status/admin/settings",
		"/api/status/admin/subscribers",
		"/api/status/admin/deliveries",
		"/api/status/admin/audit",
	} {
		response := performStatusAuthenticatedRequest(engine, http.MethodGet, path, "", adminCookie)
		require.Equal(t, http.StatusOK, response.Code, "%s: %s", path, response.Body.String())
		require.Equal(t, true, decodeStatusResponse(t, response)["success"], path)
	}
}

func TestStatusAdminSettingMutationRejectsStaleVersionAndReservedDiscordKey(t *testing.T) {
	engine, db := setupStatusHTTPTest(t)
	now := time.Now().Unix()
	require.NoError(t, db.Create(&model.StatusSetting{
		Key: "status.evidence_max_age_seconds", Value: "1200", Version: 2, UpdatedBy: 1, UpdatedAt: now - 10,
	}).Error)
	verifiedRootCookie := statusAuthCookie(t, engine, common.RoleRootUser, true)

	stale := performStatusAuthenticatedRequest(
		engine, http.MethodPut, "/api/status/admin/settings/status.evidence_max_age_seconds",
		`{"value":"600","expected_version":1}`, verifiedRootCookie,
	)
	require.Equal(t, http.StatusConflict, stale.Code, stale.Body.String())

	reserved := performStatusAuthenticatedRequest(
		engine, http.MethodPut, "/api/status/admin/settings/status.discord.webhook_endpoint",
		`{"value":"plaintext-secret","expected_version":1}`, verifiedRootCookie,
	)
	require.Equal(t, http.StatusBadRequest, reserved.Code, reserved.Body.String())
	require.NotContains(t, reserved.Body.String(), "plaintext-secret")
}
