package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetSupplyChainDailyReportsContract(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SupplierUsageDailyBatchRun{}, &model.SupplierAccountingCoverageGap{}))
	location, err := time.LoadLocation(service.SupplierReportTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	publishedAt := day.AddDate(0, 0, 1).Unix()
	evidence := types.SupplierPublishedEvidenceV1{
		SchemaVersion: types.SupplierPublishedEvidenceSchemaVersion, LogsScanned: 1,
		ProducerMarkersPresent: 0, CapturedSnapshotCount: 0,
		FailureCounts:                    types.SupplierPublishedFailureCountsV1{AbsentMarkerAfterCutover: 1},
		PersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessIncomplete,
		Warnings:                         []types.SupplierPublishedWarningV1{{Code: types.SupplierPublishedWarningAbsentMarker, MessageKey: "supply_chain.warning.absent_marker_after_cutover", Count: 1}},
	}
	rawEvidence, err := types.EncodeSupplierPublishedEvidenceV1(evidence)
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.SupplierUsageDailyBatchRun{
		BatchDate: day.Format("2006-01-02"), DayStart: day.Unix(), DayEnd: day.AddDate(0, 0, 1).Unix(),
		FenceToken: 8, PublishedFenceToken: 7, PublishedAt: &publishedAt,
		PublishedPersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessIncomplete,
		PublishedEvidenceV1:                       rawEvidence,
	}).Error)

	previousFactory := newSupplyChainReportService
	newSupplyChainReportService = func() *service.SupplierReportService {
		reports := service.NewSupplierReportService(model.NewSupplierReportStore(db))
		return reports
	}
	t.Cleanup(func() { newSupplyChainReportService = previousFactory })
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/reports/daily", GetSupplyChainDailyReports)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/reports/daily?start_date=2026-07-20&end_date=2026-07-20", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"persisted_log_universe":"`+service.SupplierReportPersistedLogUniverse+`"`)
	require.Contains(t, recorder.Body.String(), `"batch_date":"2026-07-20"`)
	require.Contains(t, recorder.Body.String(), `"published_fence_token":7`)
	require.Contains(t, recorder.Body.String(), `"finance_attention_required":true`)
}

func TestGetSupplyChainDailyReportsRejectsInvalidRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/reports/daily", GetSupplyChainDailyReports)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/reports/daily?start_date=2026-07-20", nil))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
