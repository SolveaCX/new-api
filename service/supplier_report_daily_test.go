package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSupplierDailyReportProjectionUsesPublishedEvidenceAndFreshGaps(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SupplierUsageDailyBatchRun{}, &model.SupplierAccountingCoverageGap{}))
	location, err := time.LoadLocation(SupplierReportTimezone)
	require.NoError(t, err)
	firstDay := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	secondDay := firstDay.AddDate(0, 0, 1)
	publishedAt := firstDay.AddDate(0, 0, 1).Add(time.Second).Unix()
	evidence := types.SupplierPublishedEvidenceV1{
		SchemaVersion: types.SupplierPublishedEvidenceSchemaVersion, LogsScanned: 2,
		ProducerMarkersPresent: 1, CapturedSnapshotCount: 1,
		DispositionCounts:                types.SupplierPublishedDispositionCountsV1{Captured: 1},
		FailureCounts:                    types.SupplierPublishedFailureCountsV1{AbsentMarkerAfterCutover: 1},
		PersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessIncomplete,
		Warnings:                         []types.SupplierPublishedWarningV1{{Code: types.SupplierPublishedWarningAbsentMarker, MessageKey: "supply_chain.warning.absent_marker_after_cutover", Count: 1}},
	}
	rawEvidence, err := types.EncodeSupplierPublishedEvidenceV1(evidence)
	require.NoError(t, err)
	require.NoError(t, db.Create(&[]model.SupplierUsageDailyBatchRun{
		{
			BatchDate: firstDay.Format("2006-01-02"), DayStart: firstDay.Unix(), DayEnd: secondDay.Unix(),
			Status: model.SupplierDailyBatchStatusFailed, FenceToken: 8, LogsScanned: 999, SnapshotCount: 998,
			PublishedFenceToken: 7, PublishedAt: &publishedAt,
			PublishedPersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessIncomplete,
			PublishedEvidenceV1:                       rawEvidence,
		},
		{
			BatchDate: secondDay.Format("2006-01-02"), DayStart: secondDay.Unix(), DayEnd: secondDay.AddDate(0, 0, 1).Unix(),
			Status: model.SupplierDailyBatchStatusRunning, FenceToken: 9, LogsScanned: 777,
		},
	}).Error)
	firstGap := createSupplierReportCoverageGap(t, db, "daily-first", firstDay.Add(time.Hour).Unix(), firstDay.Add(2*time.Hour).Unix())

	reports := NewSupplierReportService(model.NewSupplierReportStore(db))
	reports.now = func() time.Time { return secondDay.AddDate(0, 0, 2).Add(12 * time.Hour) }
	query := SupplierReportQuery{StartDate: firstDay.Format("2006-01-02"), EndDate: secondDay.Format("2006-01-02")}
	projection, err := reports.GetDaily(context.Background(), query)
	require.NoError(t, err)
	require.Equal(t, SupplierReportPersistedLogUniverse, projection.PersistedLogUniverse)
	require.Len(t, projection.Days, 2)

	published := projection.Days[0]
	require.True(t, published.Published)
	require.Equal(t, int64(7), published.PublishedFenceToken, "rerun CAS must use the immutable published fence")
	require.NotNil(t, published.PublishedAt)
	require.Equal(t, int64(2), published.LogsScanned, "candidate counters must never enter the projection")
	require.Equal(t, int64(1), published.FailureCounts.AbsentMarkerAfterCutover)
	require.Len(t, published.Warnings, 1)
	require.Equal(t, []int64{firstGap.Id}, supplierReportGapIDs(published.KnownCoverageGaps))
	require.True(t, published.FinanceAttentionRequired)

	outstanding := projection.Days[1]
	require.False(t, outstanding.Published)
	require.Zero(t, outstanding.PublishedFenceToken)
	require.Nil(t, outstanding.PublishedAt)
	require.Equal(t, types.SupplierPersistedLogCompletenessNotScanned, outstanding.PersistedLogSnapshotCompleteness)
	require.Zero(t, outstanding.LogsScanned, "mutable scheduler state must remain invisible")
	require.True(t, outstanding.FinanceAttentionRequired)

	secondGap := createSupplierReportCoverageGap(t, db, "daily-second", secondDay.Add(time.Hour).Unix(), secondDay.Add(2*time.Hour).Unix())
	refreshed, err := reports.GetDaily(context.Background(), query)
	require.NoError(t, err)
	require.Equal(t, []int64{secondGap.Id}, supplierReportGapIDs(refreshed.Days[1].KnownCoverageGaps), "typed gaps are queried fresh for each report snapshot")
}

func TestSupplierDailyReportProjectionUsesOneRepeatableReadSnapshot(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "supplier-daily-report-snapshot.db") + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	require.NoError(t, db.AutoMigrate(&model.SupplierUsageDailyBatchRun{}, &model.SupplierAccountingCoverageGap{}))
	location, err := time.LoadLocation(SupplierReportTimezone)
	require.NoError(t, err)
	day := time.Date(2026, 7, 20, 0, 0, 0, 0, location)
	run := supplierReportPublishedRun(t, day.Format("2006-01-02"), day.Unix(), day.AddDate(0, 0, 1).Unix(), 7, 1)
	require.NoError(t, db.Create(&run).Error)

	evidenceRead := make(chan struct{})
	releaseRead := make(chan struct{})
	var blockOnce sync.Once
	callbackName := fmt.Sprintf("test:supplier_daily_report_snapshot:%s", t.Name())
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "supplier_usage_daily_batch_runs" || !strings.Contains(strings.ToLower(tx.Statement.SQL.String()), "published_fence_token") {
			return
		}
		blockOnce.Do(func() {
			close(evidenceRead)
			<-releaseRead
		})
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callbackName) })

	reports := NewSupplierReportService(model.NewSupplierReportStore(db))
	reports.now = func() time.Time { return day.AddDate(0, 0, 2).Add(12 * time.Hour) }
	type dailyResult struct {
		projection SupplierDailyReportProjection
		err        error
	}
	result := make(chan dailyResult, 1)
	go func() {
		projection, reportErr := reports.GetDaily(context.Background(), SupplierReportQuery{StartDate: day.Format("2006-01-02"), EndDate: day.Format("2006-01-02")})
		result <- dailyResult{projection: projection, err: reportErr}
	}()
	select {
	case <-evidenceRead:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "daily report did not reach the evidence read barrier")
	}
	created := createSupplierReportCoverageGap(t, db, "snapshot-late", day.Add(time.Hour).Unix(), day.Add(2*time.Hour).Unix())
	close(releaseRead)
	first := <-result
	require.NoError(t, first.err)
	require.Empty(t, first.projection.Days[0].KnownCoverageGaps, "a gap committed after the snapshot began must not tear into the response")
	second, err := reports.GetDaily(context.Background(), SupplierReportQuery{StartDate: day.Format("2006-01-02"), EndDate: day.Format("2006-01-02")})
	require.NoError(t, err)
	require.Equal(t, []int64{created.Id}, supplierReportGapIDs(second.Days[0].KnownCoverageGaps))
}
