package model

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestQueryPublishedEvidenceExcludesMutableAndUnpublishedState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SupplierUsageDailyBatchRun{}))

	const dayStart = int64(1_784_476_800)
	publishedAt := dayStart + 86_401
	evidence := completePublishedEvidence(2)
	evidence.PersistedLogSnapshotCompleteness = types.SupplierPersistedLogCompletenessIncomplete
	evidence.ProducerMarkersPresent = 1
	evidence.CapturedSnapshotCount = 1
	evidence.DispositionCounts = types.SupplierPublishedDispositionCountsV1{Captured: 1}
	evidence.FailureCounts = types.SupplierPublishedFailureCountsV1{AbsentMarkerAfterCutover: 1}
	evidence.Warnings = []types.SupplierPublishedWarningV1{{Code: types.SupplierPublishedWarningAbsentMarker, MessageKey: "supply_chain.warning.absent_marker_after_cutover", Count: 1}}
	rawEvidence, err := types.EncodeSupplierPublishedEvidenceV1(evidence)
	require.NoError(t, err)
	require.NoError(t, db.Create(&[]SupplierUsageDailyBatchRun{
		{
			BatchDate: "2026-07-20", DayStart: dayStart, DayEnd: dayStart + 86_400,
			Status: SupplierDailyBatchStatusFailed, FenceToken: 8, LogsScanned: 999, SnapshotCount: 998,
			PublishedFenceToken: 7, PublishedAt: &publishedAt,
			PublishedPersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessIncomplete,
			PublishedEvidenceV1:                       rawEvidence,
		},
		{
			BatchDate: "2026-07-21", DayStart: dayStart + 86_400, DayEnd: dayStart + 172_800,
			Status: SupplierDailyBatchStatusRunning, FenceToken: 9, LogsScanned: 777,
			PublishedFenceToken: 0,
		},
	}).Error)

	rows, err := NewSupplierReportStore(db).QueryPublishedEvidence(context.Background(), dayStart, dayStart+172_800)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "2026-07-20", rows[0].BatchDate)
	require.Equal(t, int64(7), rows[0].PublishedFenceToken)
	require.Equal(t, int64(2), rows[0].Evidence.LogsScanned)
	require.Equal(t, int64(1), rows[0].Evidence.FailureCounts.AbsentMarkerAfterCutover)
	require.Len(t, rows[0].Evidence.Warnings, 1)
}
