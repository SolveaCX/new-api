package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	backendi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSupplierAccountingControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	require.NoError(t, backendi18n.Init())
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=busy_timeout(5000)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Option{},
		&model.SupplierAdminCommand{},
		&model.SupplierInventoryAdjustment{},
		&model.SupplierAccountingCoverageGap{},
	))
	require.NoError(t, model.MigrateSupplierAdminCommandLedger(db))
	require.NoError(t, model.FinalizeSupplierAdminCommandLedgerMigration(db))
	previous := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previous
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	return db
}

func supplierAccountingControllerEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("id", 77)
		c.Next()
	})
	engine.GET("/status", GetSupplierAccountingStatus)
	engine.GET("/readiness", GetSupplierAccountingReadiness)
	engine.POST("/prepare", PrepareSupplierAccounting)
	engine.POST("/mutation-gate", ToggleSupplierAccountingMutationGate)
	return engine
}

func TestSupplierAccountingMutationGateRequiresExplicitEnabled(t *testing.T) {
	setupSupplierAccountingControllerTestDB(t)
	engine := supplierAccountingControllerEngine()

	missing := performSupplierAccountingControllerRequest(engine, http.MethodPost, "/mutation-gate", `{"expected_state_version":0,"reason":"missing enabled"}`, "gate-missing")
	require.Equal(t, http.StatusBadRequest, missing.Code)
	require.Contains(t, missing.Body.String(), `"code":"invalid_request"`)

	explicitFalse := performSupplierAccountingControllerRequest(engine, http.MethodPost, "/mutation-gate", `{"expected_state_version":0,"reason":"explicit disable","enabled":false}`, "gate-false")
	require.Equal(t, http.StatusOK, explicitFalse.Code)
	require.Contains(t, explicitFalse.Body.String(), `"enabled":false`)
}

func performSupplierAccountingControllerRequest(engine *gin.Engine, method, path, body, key string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestSupplierAccountingControllerRequiresCommandFieldsAndReturnsAuthoritativeReplay(t *testing.T) {
	db := setupSupplierAccountingControllerTestDB(t)
	engine := supplierAccountingControllerEngine()

	missingVersion := performSupplierAccountingControllerRequest(engine, http.MethodPost, "/prepare", `{"reason":"prepare","accepted_capability_versions":[1]}`, "prepare-key")
	require.Equal(t, http.StatusBadRequest, missingVersion.Code)
	require.Contains(t, missingVersion.Body.String(), `"code":"invalid_request"`)

	missingKey := performSupplierAccountingControllerRequest(engine, http.MethodPost, "/prepare", `{"expected_state_version":0,"reason":"prepare","accepted_capability_versions":[1]}`, "")
	require.Equal(t, http.StatusBadRequest, missingKey.Code)
	require.Contains(t, missingKey.Body.String(), `"code":"idempotency_key_required"`)

	body := `{"expected_state_version":0,"reason":"prepare rollout","accepted_capability_versions":[1]}`
	created := performSupplierAccountingControllerRequest(engine, http.MethodPost, "/prepare", body, "prepare-key")
	require.Equal(t, http.StatusOK, created.Code)
	require.Contains(t, created.Body.String(), `"phase":"shadow"`)
	require.Empty(t, created.Header().Get("Idempotent-Replayed"))
	var command model.SupplierAdminCommand
	require.NoError(t, db.Where("scope = ? AND idempotency_key = ?", model.SupplierAccountingCommandScopePrepare, "prepare-key").Take(&command).Error)
	require.Equal(t, 77, command.ActorId, "the authenticated context actor must own the command")

	replay := performSupplierAccountingControllerRequest(engine, http.MethodPost, "/prepare", body, "prepare-key")
	require.Equal(t, http.StatusOK, replay.Code)
	require.Equal(t, "true", replay.Header().Get("Idempotent-Replayed"))
	require.JSONEq(t, created.Body.String(), replay.Body.String(), "replay body must use the authoritative stored command result")

	conflict := performSupplierAccountingControllerRequest(engine, http.MethodPost, "/prepare", `{"expected_state_version":0,"reason":"changed payload","accepted_capability_versions":[1]}`, "prepare-key")
	require.Equal(t, http.StatusConflict, conflict.Code)
	require.Contains(t, conflict.Body.String(), `"code":"idempotency_conflict"`)
}

func TestSupplierAccountingControllerErrorCodeMapping(t *testing.T) {
	require.NoError(t, backendi18n.Init())
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "idempotency", err: model.ErrSupplierAdminIdempotencyConflict, status: http.StatusConflict, code: "idempotency_conflict"},
		{name: "option version", err: model.ErrSupplierAccountingOptionConflict, status: http.StatusConflict, code: "version_conflict"},
		{name: "gap version", err: model.ErrSupplierCoverageGapCASConflict, status: http.StatusConflict, code: "version_conflict"},
		{name: "transition", err: model.ErrSupplierAccountingTransition, status: http.StatusConflict, code: "invalid_transition"},
		{name: "unresolved", err: model.ErrSupplierAccountingCoverageUnresolved, status: http.StatusConflict, code: "coverage_unresolved"},
		{name: "not found", err: model.ErrSupplierCoverageGapNotFound, status: http.StatusNotFound, code: "not_found"},
		{name: "invalid", err: model.ErrSupplierAccountingCommandInvalid, status: http.StatusBadRequest, code: "invalid_request"},
		{name: "malformed", err: model.ErrSupplierAccountingOptionMalformed, status: http.StatusServiceUnavailable, code: "state_malformed"},
		{name: "database", err: model.ErrDatabase, status: http.StatusServiceUnavailable, code: "control_plane_unavailable"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			supplierAccountingControlError(context, testCase.err)
			require.Equal(t, testCase.status, recorder.Code)
			require.Contains(t, recorder.Body.String(), `"code":"`+testCase.code+`"`)
		})
	}
}

func TestSupplierAccountingStatusAndReadinessFailClosed(t *testing.T) {
	db := setupSupplierAccountingControllerTestDB(t)
	engine := supplierAccountingControllerEngine()

	status := performSupplierAccountingControllerRequest(engine, http.MethodGet, "/status", "", "")
	require.Equal(t, http.StatusOK, status.Code)
	require.Contains(t, status.Body.String(), `"phase":"disabled"`)
	require.Contains(t, status.Body.String(), `"admin_command_ledger_state":"finalized"`)
	ready := performSupplierAccountingControllerRequest(engine, http.MethodGet, "/readiness", "", "")
	require.Equal(t, http.StatusOK, ready.Code)
	require.Contains(t, ready.Body.String(), `"ready":true`)

	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", "not-a-timestamp")
	notReady := performSupplierAccountingControllerRequest(engine, http.MethodGet, "/readiness", "", "")
	require.Equal(t, http.StatusServiceUnavailable, notReady.Code)
	require.Contains(t, notReady.Body.String(), `"code":"supplier_accounting_not_ready"`)

	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", "")
	require.NoError(t, db.Create(&model.Option{Key: model.SupplierAccountingMutationOptionKey, Value: `{}`}).Error)
	malformedMutation := performSupplierAccountingControllerRequest(engine, http.MethodGet, "/readiness", "", "")
	require.Equal(t, http.StatusServiceUnavailable, malformedMutation.Code)
	require.Contains(t, malformedMutation.Body.String(), `"code":"supplier_accounting_not_ready"`)
	require.NoError(t, db.Where("key = ?", model.SupplierAccountingMutationOptionKey).Delete(&model.Option{}).Error)

	require.NoError(t, db.Create(&model.Option{Key: model.SupplierAccountingActivationOptionKey, Value: `{}`}).Error)
	malformedReadiness := performSupplierAccountingControllerRequest(engine, http.MethodGet, "/readiness", "", "")
	require.Equal(t, http.StatusServiceUnavailable, malformedReadiness.Code)
	require.Contains(t, malformedReadiness.Body.String(), `"code":"supplier_accounting_not_ready"`)
	malformed := performSupplierAccountingControllerRequest(engine, http.MethodGet, "/status", "", "")
	require.Equal(t, http.StatusServiceUnavailable, malformed.Code)
	require.Contains(t, malformed.Body.String(), `"code":"state_malformed"`)
}
