package controller

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	backendi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTriggerSupplierDailyBatchCatchUpFixedResponse(t *testing.T) {
	require.NoError(t, backendi18n.Init())
	gin.SetMode(gin.TestMode)
	token := configureSupplierBatchControllerAuth(t)
	now := time.Date(2026, time.July, 23, 3, 0, 0, 0, time.UTC)
	mainDB := &gorm.DB{}
	logDB := &gorm.DB{}
	called := false
	engine := gin.New()
	engine.POST("/catch-up", middleware.SupplierBatchAuth(), func(c *gin.Context) {
		triggerSupplierDailyBatchCatchUp(c, func(_ context.Context, gotMainDB, gotLogDB *gorm.DB, principal dto.SupplierBatchSchedulerPrincipal, request dto.SupplierBatchCatchUpRequest, gotNow time.Time) (dto.SupplierBatchStatusResponse, error) {
			called = true
			require.Same(t, mainDB, gotMainDB)
			require.Same(t, logDB, gotLogDB)
			require.Equal(t, "supplier-controller-runner", principal.TrustedJobIdentity)
			require.Equal(t, dto.SupplierBatchAuditSlotCurrent, principal.AuditSlot)
			require.Equal(t, "runner-key", request.RequestID)
			require.Equal(t, now, gotNow)
			return dto.SupplierBatchStatusResponse{
				RequestID: "runner-key", Status: dto.SupplierBatchStatusCompleted, ErrorCategory: dto.SupplierBatchErrorNone,
				Result: &dto.SupplierBatchStatusResult{ProcessedDays: 0, RemainingWork: false},
			}, nil
		}, mainDB, logDB, now)
	})
	recorder := performSupplierBatchControllerRequest(engine, token, http.MethodPost, "/catch-up", "runner-key")
	require.True(t, called)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"request_id":"runner-key","batch_date":null,"run_id":null,"status":"completed","fence_token":0,"published_fence_token":0,"locked_until":null,"error_category":"none","result":{"processed_days":0,"remaining_work":false,"next_batch_date":null}}`, recorder.Body.String())
}

func TestSupplierDailyBatchControllerStableErrors(t *testing.T) {
	token := configureSupplierBatchControllerAuth(t)
	tests := []struct {
		name       string
		runErr     error
		wantStatus int
		wantCode   string
	}{
		{name: "busy", runErr: service.ErrSupplierBatchBusy, wantStatus: http.StatusConflict, wantCode: "busy"},
		{name: "conflict", runErr: service.ErrSupplierBatchIdempotencyConflict, wantStatus: http.StatusConflict, wantCode: "idempotency_conflict"},
		{name: "config", runErr: service.ErrSupplierBatchConfigUnavailable, wantStatus: http.StatusServiceUnavailable, wantCode: "config_unavailable"},
		{name: "internal", runErr: errors.New("database password must not leak"), wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine := gin.New()
			engine.POST("/catch-up", middleware.SupplierBatchAuth(), func(c *gin.Context) {
				triggerSupplierDailyBatchCatchUp(c, func(context.Context, *gorm.DB, *gorm.DB, dto.SupplierBatchSchedulerPrincipal, dto.SupplierBatchCatchUpRequest, time.Time) (dto.SupplierBatchStatusResponse, error) {
					return dto.SupplierBatchStatusResponse{}, test.runErr
				}, &gorm.DB{}, &gorm.DB{}, time.Now())
			})
			recorder := performSupplierBatchControllerRequest(engine, token, http.MethodPost, "/catch-up", "runner-key")
			require.Equal(t, test.wantStatus, recorder.Code)
			require.Contains(t, recorder.Body.String(), `"code":"`+test.wantCode+`"`)
			require.NotContains(t, recorder.Body.String(), "database password")
		})
	}
}

func TestGetSupplierDailyBatchStatusRecoversUnknownRunID(t *testing.T) {
	token := configureSupplierBatchControllerAuth(t)
	engine := gin.New()
	engine.GET("/status", middleware.SupplierBatchAuth(), func(c *gin.Context) {
		getSupplierDailyBatchStatus(c, func(_ context.Context, _ *gorm.DB, principal dto.SupplierBatchSchedulerPrincipal, requestID string, _ time.Time) (dto.SupplierBatchStatusResponse, error) {
			require.Equal(t, "supplier-controller-runner", principal.TrustedJobIdentity)
			require.Equal(t, "lost-response-key", requestID)
			return dto.SupplierBatchStatusResponse{}, service.ErrSupplierBatchRequestNotFound
		}, &gorm.DB{}, time.Now())
	})
	recorder := performSupplierBatchControllerRequest(engine, token, http.MethodGet, "/status?request_id=lost-response-key", "")
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"code":"not_found"`)
}

func TestGetSupplierDailyBatchStatusReturnsActiveRequestByRequestID(t *testing.T) {
	token := configureSupplierBatchControllerAuth(t)
	batchDate := "2026-07-22"
	runID := int64(42)
	lockedUntil := "2026-07-23T03:00:00+08:00"
	engine := gin.New()
	engine.GET("/status", middleware.SupplierBatchAuth(), func(c *gin.Context) {
		getSupplierDailyBatchStatus(c, func(_ context.Context, _ *gorm.DB, _ dto.SupplierBatchSchedulerPrincipal, requestID string, _ time.Time) (dto.SupplierBatchStatusResponse, error) {
			return dto.SupplierBatchStatusResponse{
				RequestID: requestID, BatchDate: &batchDate, RunID: &runID, Status: dto.SupplierBatchStatusRunning,
				FenceToken: 9, PublishedFenceToken: 7, LockedUntil: &lockedUntil, ErrorCategory: dto.SupplierBatchErrorNone,
			}, nil
		}, &gorm.DB{}, time.Now())
	})
	recorder := performSupplierBatchControllerRequest(engine, token, http.MethodGet, "/status?request_id=lost-response-key", "")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"request_id":"lost-response-key","batch_date":"2026-07-22","run_id":42,"status":"running","fence_token":9,"published_fence_token":7,"locked_until":"2026-07-23T03:00:00+08:00","error_category":"none","result":null}`, recorder.Body.String())
}

func TestSupplierDailyBatchControllerRejectsMissingOrDuplicateRequestID(t *testing.T) {
	token := configureSupplierBatchControllerAuth(t)
	called := false
	engine := gin.New()
	engine.POST("/catch-up", middleware.SupplierBatchAuth(), func(c *gin.Context) {
		triggerSupplierDailyBatchCatchUp(c, func(context.Context, *gorm.DB, *gorm.DB, dto.SupplierBatchSchedulerPrincipal, dto.SupplierBatchCatchUpRequest, time.Time) (dto.SupplierBatchStatusResponse, error) {
			called = true
			return dto.SupplierBatchStatusResponse{}, nil
		}, &gorm.DB{}, &gorm.DB{}, time.Now())
	})
	for _, keys := range [][]string{nil, {"key-one", "key-two"}, {" surrounded "}} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/catch-up", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		for _, key := range keys {
			request.Header.Add("Idempotency-Key", key)
		}
		engine.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusBadRequest, recorder.Code)
		require.Contains(t, recorder.Body.String(), `"code":"idempotency_key_required"`)
	}
	require.False(t, called)
}

func configureSupplierBatchControllerAuth(t *testing.T) string {
	t.Helper()
	require.NoError(t, backendi18n.Init())
	raw := make([]byte, 32)
	for index := range raw {
		raw[index] = 21
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256(raw)
	t.Setenv(middleware.SupplierBatchCurrentVerifierHashEnv, hex.EncodeToString(digest[:]))
	t.Setenv(middleware.SupplierBatchNextVerifierHashEnv, "")
	t.Setenv(middleware.SupplierBatchTrustedIdentityEnv, "supplier-controller-runner")
	return token
}

func performSupplierBatchControllerRequest(engine *gin.Engine, token, method, path, requestID string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	if requestID != "" {
		request.Header.Set("Idempotency-Key", requestID)
	}
	engine.ServeHTTP(recorder, request)
	return recorder
}
