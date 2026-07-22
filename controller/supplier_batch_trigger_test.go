package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	backendi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTriggerSupplierDailyBatchCatchUpResponses(t *testing.T) {
	require.NoError(t, backendi18n.Init())
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, time.July, 23, 3, 0, 0, 0, time.FixedZone("test", 8*60*60))
	mainDB := &gorm.DB{}
	logDB := &gorm.DB{}

	tests := []struct {
		name       string
		result     service.SupplierDailyBatchCatchUpResult
		runErr     error
		wantStatus int
		wantBody   string
	}{
		{name: "success", result: service.SupplierDailyBatchCatchUpResult{ProcessedDays: 1, RemainingWork: true, NextBatchDate: "2026-07-21"}, wantStatus: http.StatusOK, wantBody: `"processed_days":1,"remaining_work":true,"next_batch_date":"2026-07-21"`},
		{name: "busy", runErr: model.ErrSupplierDailyBatchBusy, wantStatus: http.StatusConflict, wantBody: `"status":"busy"`},
		{name: "failure", runErr: errors.New("database password must not leak"), wantStatus: http.StatusInternalServerError, wantBody: `"status":"error"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var called bool
			engine := gin.New()
			engine.POST("/trigger", func(c *gin.Context) {
				triggerSupplierDailyBatchCatchUp(c, func(_ context.Context, gotMainDB, gotLogDB *gorm.DB, owner string, gotNow time.Time) (service.SupplierDailyBatchCatchUpResult, error) {
					called = true
					require.Same(t, mainDB, gotMainDB)
					require.Same(t, logDB, gotLogDB)
					require.Equal(t, "test-owner", owner)
					require.Equal(t, now, gotNow)
					return test.result, test.runErr
				}, mainDB, logDB, "test-owner", now)
			})

			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/trigger", nil))

			require.True(t, called)
			require.Equal(t, test.wantStatus, recorder.Code)
			require.Contains(t, recorder.Body.String(), test.wantBody)
			require.NotContains(t, recorder.Body.String(), "database password")
		})
	}
}
