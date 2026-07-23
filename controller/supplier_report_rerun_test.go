package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	backendi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRerunSupplyChainDailyReportContract(t *testing.T) {
	require.NoError(t, backendi18n.Init())
	gin.SetMode(gin.TestMode)
	mainDB := &gorm.DB{}
	logDB := &gorm.DB{}
	now := time.Date(2026, time.July, 23, 3, 0, 0, 0, time.UTC)
	called := false
	engine := gin.New()
	engine.POST("/reports/daily/:date/rerun", func(c *gin.Context) {
		c.Set("id", 17)
		rerunSupplyChainDailyReport(c, func(_ context.Context, gotMainDB, gotLogDB *gorm.DB, actorID int, batchDate, key string, request dto.SupplierDailyReportRerunRequest, gotNow time.Time) (dto.SupplierBatchStatusResponse, error) {
			called = true
			require.Same(t, mainDB, gotMainDB)
			require.Same(t, logDB, gotLogDB)
			require.Equal(t, 17, actorID)
			require.Equal(t, "2026-07-22", batchDate)
			require.Equal(t, "rerun-key", key)
			require.Equal(t, "retry incomplete persisted scan", request.Reason)
			require.Equal(t, int64(7), request.ExpectedPublishedFenceToken)
			require.Equal(t, now, gotNow)
			runID := int64(43)
			return dto.SupplierBatchStatusResponse{
				RequestID: key, BatchDate: &batchDate, RunID: &runID, Status: dto.SupplierBatchStatusCompleted,
				FenceToken: 8, PublishedFenceToken: 8, ErrorCategory: dto.SupplierBatchErrorNone,
				Result: &dto.SupplierBatchStatusResult{ProcessedDays: 1, RemainingWork: false},
			}, nil
		}, mainDB, logDB, now)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/reports/daily/2026-07-22/rerun", strings.NewReader(`{"reason":"retry incomplete persisted scan","expected_published_fence_token":7}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "rerun-key")
	engine.ServeHTTP(recorder, request)
	require.True(t, called)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"published_fence_token":8`)
}

func TestRerunSupplyChainDailyReportStableErrors(t *testing.T) {
	require.NoError(t, backendi18n.Init())
	tests := []struct {
		name       string
		runErr     error
		wantStatus int
		wantCode   string
	}{
		{name: "not found", runErr: service.ErrSupplierDailyReportNotFound, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "not eligible", runErr: service.ErrSupplierDailyReportNotEligible, wantStatus: http.StatusConflict, wantCode: "not_eligible"},
		{name: "stale generation", runErr: service.ErrSupplierDailyReportVersionConflict, wantStatus: http.StatusConflict, wantCode: "version_conflict"},
		{name: "busy", runErr: service.ErrSupplierBatchBusy, wantStatus: http.StatusConflict, wantCode: "busy"},
		{name: "idempotency conflict", runErr: service.ErrSupplierBatchIdempotencyConflict, wantStatus: http.StatusConflict, wantCode: "idempotency_conflict"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine := gin.New()
			engine.POST("/reports/daily/:date/rerun", func(c *gin.Context) {
				c.Set("id", 17)
				rerunSupplyChainDailyReport(c, func(context.Context, *gorm.DB, *gorm.DB, int, string, string, dto.SupplierDailyReportRerunRequest, time.Time) (dto.SupplierBatchStatusResponse, error) {
					return dto.SupplierBatchStatusResponse{}, test.runErr
				}, &gorm.DB{}, &gorm.DB{}, time.Now())
			})
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/reports/daily/2026-07-22/rerun", strings.NewReader(`{"reason":"retry","expected_published_fence_token":7}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", "rerun-key")
			engine.ServeHTTP(recorder, request)
			require.Equal(t, test.wantStatus, recorder.Code)
			require.Contains(t, recorder.Body.String(), `"code":"`+test.wantCode+`"`)
		})
	}
}

func TestRerunSupplyChainDailyReportRejectsInvalidInput(t *testing.T) {
	require.NoError(t, backendi18n.Init())
	tests := []struct{ name, date, key, body string }{
		{name: "invalid date", date: "2026-02-30", key: "rerun-key", body: `{"reason":"retry","expected_published_fence_token":7}`},
		{name: "missing key", date: "2026-07-22", body: `{"reason":"retry","expected_published_fence_token":7}`},
		{name: "missing reason", date: "2026-07-22", key: "rerun-key", body: `{"expected_published_fence_token":7}`},
		{name: "missing fence", date: "2026-07-22", key: "rerun-key", body: `{"reason":"retry"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			engine := gin.New()
			engine.POST("/reports/daily/:date/rerun", func(c *gin.Context) {
				c.Set("id", 17)
				rerunSupplyChainDailyReport(c, func(context.Context, *gorm.DB, *gorm.DB, int, string, string, dto.SupplierDailyReportRerunRequest, time.Time) (dto.SupplierBatchStatusResponse, error) {
					called = true
					return dto.SupplierBatchStatusResponse{}, nil
				}, &gorm.DB{}, &gorm.DB{}, time.Now())
			})
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/reports/daily/"+test.date+"/rerun", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			if test.key != "" {
				request.Header.Set("Idempotency-Key", test.key)
			}
			engine.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.False(t, called)
		})
	}
}
