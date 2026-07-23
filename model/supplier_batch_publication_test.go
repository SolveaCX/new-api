package model

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var supplierBatchTestDBSequence atomic.Uint64

func newSupplierBatchTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	databaseName := fmt.Sprintf("%s-%d", t.Name(), supplierBatchTestDBSequence.Add(1))
	db, err := gorm.Open(sqlite.Open("file:"+databaseName+"?mode=memory&cache=shared&_pragma=busy_timeout(10000)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SupplierUsageDailySummary{}, &SupplierUsageDailyBatchRun{}, &SupplierAdminCommand{}))
	require.NoError(t, MigrateSupplierAdminCommandLedger(db))
	return db
}

func completePublishedEvidence(logs int64) types.SupplierPublishedEvidenceV1 {
	return types.SupplierPublishedEvidenceV1{
		SchemaVersion: 1, LogsScanned: logs, ProducerMarkersPresent: logs, CapturedSnapshotCount: logs,
		DispositionCounts:                types.SupplierPublishedDispositionCountsV1{Captured: logs},
		PersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessComplete,
		Warnings:                         []types.SupplierPublishedWarningV1{},
	}
}

func supplierBatchTestSummary(batchDate string, fence int64, key string, bucketStart int64) SupplierUsageDailySummary {
	return SupplierUsageDailySummary{
		BatchDate: batchDate, BatchFenceToken: fence, DimensionKey: key, BucketStart: bucketStart,
		SupplierId: 1, ContractId: 1, RateVersionId: 1, ChannelId: 1,
		StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 1,
	}
}

func TestSupplierDailyPublicationAtomicReplacementAndFailureRetention(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	ctx := context.Background()
	lease, err := AcquireSupplierDailyBatch(ctx, db, "2026-07-20", 1784476800, 1784563200, "first", time.Now(), time.Minute, false)
	require.NoError(t, err)
	summary := supplierBatchTestSummary(lease.BatchDate, lease.FenceToken, "published", 1784476800)
	require.NoError(t, PersistSupplierDailyBatchPage(ctx, db, lease, []SupplierUsageDailySummary{summary}, 1, 1, 1, 1, time.Minute))
	require.NoError(t, PublishSupplierDailyBatch(ctx, db, lease, time.Unix(1784563201, 0), completePublishedEvidence(1)))
	oldRun, oldEvidence, err := LoadSupplierPublishedDailyBatch(ctx, db, "2026-07-20")
	require.NoError(t, err)

	// Install a valid incomplete old view directly to exercise expected-fence rerun CAS.
	incomplete := completePublishedEvidence(1)
	incomplete.DispositionCounts.Captured = 0
	incomplete.CapturedSnapshotCount = 0
	incomplete.ProducerMarkersPresent = 0
	incomplete.FailureCounts.AbsentMarkerAfterCutover = 1
	incomplete.PersistedLogSnapshotCompleteness = types.SupplierPersistedLogCompletenessIncomplete
	incomplete.Warnings = []types.SupplierPublishedWarningV1{{Code: types.SupplierPublishedWarningAbsentMarker, Count: 1, MessageKey: "supply_chain.warning.absent_marker_after_cutover"}}
	raw, encodeErr := types.EncodeSupplierPublishedEvidenceV1(incomplete)
	require.NoError(t, encodeErr)
	require.NoError(t, db.Model(&SupplierUsageDailyBatchRun{}).Where("id = ?", oldRun.Id).Updates(map[string]any{"published_evidence_v1": raw, "published_persisted_log_snapshot_completeness": types.SupplierPersistedLogCompletenessIncomplete}).Error)

	rerun, err := AcquireSupplierDailyBatchRerun(ctx, db, oldRun.BatchDate, oldRun.DayStart, oldRun.DayEnd, "rerun", time.Now(), time.Minute, oldRun.PublishedFenceToken)
	require.NoError(t, err)
	err = PublishSupplierDailyBatch(ctx, db, rerun, time.Now(), completePublishedEvidence(1))
	require.ErrorIs(t, err, ErrSupplierDailyBatchPublicationInvalid)
	retained, evidence, err := LoadSupplierPublishedDailyBatch(ctx, db, oldRun.BatchDate)
	require.NoError(t, err)
	require.Equal(t, oldRun.PublishedFenceToken, retained.PublishedFenceToken)
	require.Equal(t, incomplete, *evidence)
	require.NotEqual(t, oldEvidence.PersistedLogSnapshotCompleteness, evidence.PersistedLogSnapshotCompleteness)
}

func TestOldestNeverPublishedRequiresExactZeroPublishedFence(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	activeSlot := 1
	runs := []SupplierUsageDailyBatchRun{
		{BatchDate: "2026-07-20", DayStart: 1, DayEnd: 2, Status: SupplierDailyBatchStatusFailed, FenceToken: 2, PublishedFenceToken: 1},
		{BatchDate: "2026-07-21", DayStart: 2, DayEnd: 3, Status: SupplierDailyBatchStatusFailed, FenceToken: 2, PublishedFenceToken: -1},
		{BatchDate: "2026-07-22", DayStart: 3, DayEnd: 4, Status: SupplierDailyBatchStatusCompleted, FenceToken: 5, PublishedFenceToken: 0},
		{BatchDate: "2026-07-23", DayStart: 4, DayEnd: 5, Status: SupplierDailyBatchStatusRunning, FenceToken: 6, PublishedFenceToken: 0, ActiveLeaseSlot: &activeSlot, LockedUntil: 1<<62 - 1},
		{BatchDate: "2026-07-24", DayStart: 5, DayEnd: 6, Status: SupplierDailyBatchStatusRunning, FenceToken: 7, PublishedFenceToken: 0, LockedUntil: 1},
		{BatchDate: "2026-07-25", DayStart: 6, DayEnd: 7, Status: SupplierDailyBatchStatusFailed, FenceToken: 0, PublishedFenceToken: 0},
	}
	require.NoError(t, db.Create(&runs).Error)
	date, ok, err := OldestNeverPublishedSupplierDailyBatchDate(context.Background(), db, "2026-07-20", "2026-07-25")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "2026-07-22", date, "mutable completed status cannot hide an exact-zero published fence")

	require.NoError(t, db.Model(&SupplierUsageDailyBatchRun{}).Where("batch_date = ?", date).Updates(map[string]any{
		"published_fence_token": 5,
	}).Error)
	date, ok, err = OldestNeverPublishedSupplierDailyBatchDate(context.Background(), db, "2026-07-20", "2026-07-25")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "2026-07-23", date, "an active lease remains backlog and is rejected as busy during acquisition")
}

func TestEnsureSupplierDailyBatchCandidatesMaterializesShanghaiDates(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	require.NoError(t, EnsureSupplierDailyBatchCandidates(context.Background(), db, "2026-07-20", "2026-07-22"))

	var runs []SupplierUsageDailyBatchRun
	require.NoError(t, db.Order("batch_date ASC").Find(&runs).Error)
	require.Len(t, runs, 3)
	location, err := time.LoadLocation(supplierDailyBatchTimezone)
	require.NoError(t, err)
	for index := range runs {
		day := time.Date(2026, 7, 20+index, 0, 0, 0, 0, location)
		require.Equal(t, day.Format("2006-01-02"), runs[index].BatchDate)
		require.Equal(t, day.Unix(), runs[index].DayStart)
		require.Equal(t, day.AddDate(0, 0, 1).Unix(), runs[index].DayEnd)
		require.Equal(t, SupplierDailyBatchStatusFailed, runs[index].Status)
		require.Zero(t, runs[index].FenceToken)
		require.Zero(t, runs[index].PublishedFenceToken)
		require.Zero(t, runs[index].LockedUntil)
		require.Nil(t, runs[index].ActiveLeaseSlot)
	}
	date, found, err := OldestNeverPublishedSupplierDailyBatchDate(context.Background(), db, "2026-07-20", "2026-07-22")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "2026-07-20", date)
}

func TestEnsureSupplierDailyBatchCandidatesNoWorkDoesNotInsertRows(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	ctx := context.Background()
	require.NoError(t, EnsureSupplierDailyBatchCandidates(ctx, db, "2026-07-20", "2026-07-21"))
	require.NoError(t, db.Model(&SupplierUsageDailyBatchRun{}).Where("batch_date >= ? AND batch_date <= ?", "2026-07-20", "2026-07-21").Updates(map[string]any{
		"status": SupplierDailyBatchStatusCompleted, "fence_token": 1, "published_fence_token": 1,
	}).Error)

	var before int64
	require.NoError(t, db.Model(&SupplierUsageDailyBatchRun{}).Count(&before).Error)
	require.NoError(t, EnsureSupplierDailyBatchCandidates(ctx, db, "2026-07-20", "2026-07-21"))
	var after int64
	require.NoError(t, db.Model(&SupplierUsageDailyBatchRun{}).Count(&after).Error)
	require.Equal(t, before, after)
	date, found, err := OldestNeverPublishedSupplierDailyBatchDate(ctx, db, "2026-07-20", "2026-07-21")
	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, date)
}

func TestEnsureSupplierDailyBatchCandidatesConcurrentMaterialization(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	const callers = 16
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for index := range errs {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			errs[index] = EnsureSupplierDailyBatchCandidates(context.Background(), db, "2026-07-20", "2026-07-29")
		}(index)
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	var count int64
	require.NoError(t, db.Model(&SupplierUsageDailyBatchRun{}).Count(&count).Error)
	require.EqualValues(t, 10, count)
}

func TestEnsureSupplierDailyBatchCandidatesRejectsCorruptFences(t *testing.T) {
	t.Run("negative published fence", func(t *testing.T) {
		db := newSupplierBatchTestDB(t)
		require.NoError(t, db.Create(&SupplierUsageDailyBatchRun{
			BatchDate: "2026-07-20", DayStart: 1, DayEnd: 2, Status: SupplierDailyBatchStatusFailed, FenceToken: 1, PublishedFenceToken: -1,
		}).Error)
		err := EnsureSupplierDailyBatchCandidates(context.Background(), db, "2026-07-20", "2026-07-21")
		require.ErrorIs(t, err, ErrSupplierDailyBatchPublicationInvalid)
		var count int64
		require.NoError(t, db.Model(&SupplierUsageDailyBatchRun{}).Count(&count).Error)
		require.EqualValues(t, 1, count, "validation must fail before materializing another date")
	})

	t.Run("published fence exceeds candidate fence", func(t *testing.T) {
		db := newSupplierBatchTestDB(t)
		require.NoError(t, db.Create(&SupplierUsageDailyBatchRun{
			BatchDate: "2026-07-20", DayStart: 1, DayEnd: 2, Status: SupplierDailyBatchStatusCompleted, FenceToken: 1, PublishedFenceToken: 2,
		}).Error)
		err := EnsureSupplierDailyBatchCandidates(context.Background(), db, "2026-07-20", "2026-07-20")
		require.ErrorIs(t, err, ErrSupplierDailyBatchPublicationInvalid)
	})
}

func TestOldestNeverPublishedSelectorSourceGuard(t *testing.T) {
	source, err := os.ReadFile("supplier_usage.go")
	require.NoError(t, err)
	text := string(source)
	queryStart := strings.Index(text, "func oldestNeverPublishedSupplierDailyBatchQuery")
	require.NotEqual(t, -1, queryStart)
	queryEnd := strings.Index(text[queryStart:], "\nfunc supplierDailyBatchDateRange")
	require.NotEqual(t, -1, queryEnd)
	querySource := text[queryStart : queryStart+queryEnd]
	require.Contains(t, querySource, `Where("published_fence_token = ?", 0)`)
	require.NotContains(t, querySource, `published_fence_token >`)
	require.NotContains(t, querySource, `status`)
	require.NotContains(t, querySource, `locked_until`)
	require.NotContains(t, querySource, `active_lease_slot`)

	selectorStart := strings.Index(text, "func OldestNeverPublishedSupplierDailyBatchDate")
	require.NotEqual(t, -1, selectorStart)
	selectorEnd := strings.Index(text[selectorStart:], "\nfunc oldestNeverPublishedSupplierDailyBatchQuery")
	require.NotEqual(t, -1, selectorEnd)
	selectorSource := text[selectorStart : selectorStart+selectorEnd]
	require.Contains(t, selectorSource, "oldestNeverPublishedSupplierDailyBatchQuery")
	require.NotContains(t, selectorSource, "Pluck(")
	require.NotContains(t, selectorSource, "make(map[")
	require.NotContains(t, selectorSource, "for day :=")
}

func TestOldestNeverPublishedSelectorSQLIsCrossDatabaseReady(t *testing.T) {
	sqliteDB := newSupplierBatchTestDB(t)
	connection, err := sqliteDB.DB()
	require.NoError(t, err)
	dialectors := map[string]gorm.Dialector{
		"sqlite":   sqlite.Open("file:" + t.Name() + "-dry-run?mode=memory&cache=shared"),
		"mysql":    mysql.New(mysql.Config{Conn: connection, SkipInitializeWithVersion: true}),
		"postgres": postgres.New(postgres.Config{Conn: connection, WithoutReturning: true}),
	}
	for name, dialector := range dialectors {
		t.Run(name, func(t *testing.T) {
			dryRun, openErr := gorm.Open(dialector, &gorm.Config{DryRun: true, DisableAutomaticPing: true})
			require.NoError(t, openErr)
			var candidate struct{ BatchDate string }
			statement := oldestNeverPublishedSupplierDailyBatchQuery(dryRun, "2026-07-20", "2026-07-22").Scan(&candidate).Statement
			normalized := strings.NewReplacer("`", "", `"`, "").Replace(strings.ToLower(statement.SQL.String()))
			require.Contains(t, normalized, "published_fence_token =")
			require.NotContains(t, normalized, "published_fence_token >")
			require.NotContains(t, normalized, "status")
			require.NotContains(t, normalized, "locked_until")
			require.NotContains(t, normalized, "active_lease_slot")
			require.Contains(t, normalized, "order by batch_date asc")
			require.Contains(t, normalized, "limit")
		})
	}
}

func TestSupplierReportNeverReadsZeroFenceCandidate(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	run := SupplierUsageDailyBatchRun{BatchDate: "2026-07-20", DayStart: 100, DayEnd: 200, Status: SupplierDailyBatchStatusRunning, FenceToken: 0, PublishedFenceToken: 0}
	require.NoError(t, db.Create(&run).Error)
	summary := SupplierUsageDailySummary{BatchDate: run.BatchDate, BatchFenceToken: 0, DimensionKey: "candidate", BucketStart: 100, SupplierId: 1, ContractId: 1, RateVersionId: 1, ChannelId: 1, StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 99}
	require.NoError(t, db.Create(&summary).Error)
	rows, err := NewSupplierReportStore(db).QueryBusinessUsage(context.Background(), SupplierReportFilter{StartAt: 100, EndAt: 200}, false)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestSupplierDailyFirstAcquisitionRaceOneWinner(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	const callers = 24
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = AcquireSupplierDailyBatch(context.Background(), db, "2026-07-20", 1784476800, 1784563200, fmt.Sprintf("owner-%d", i), time.Now(), time.Minute, false)
		}(i)
	}
	wg.Wait()
	winners := 0
	for _, err := range errs {
		if err == nil {
			winners++
		} else {
			require.True(t, errors.Is(err, ErrSupplierDailyBatchBusy), err)
		}
	}
	require.Equal(t, 1, winners)
}

func TestSupplierSchedulerCommandReplayNoWorkAndAdminIndexCoexist(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	ctx := context.Background()
	identity, err := DigestSupplierBatchTrustedIdentity("trusted-job")
	require.NoError(t, err)
	claim, err := ClaimSupplierBatchSchedulerCommand(ctx, db, identity, "request-1", struct {
		Mode string `json:"mode"`
	}{"catch_up"}, SupplierBatchSchedulerAuditSlotCurrent)
	require.NoError(t, err)
	require.True(t, claim.Claimed)
	state := types.SupplierBatchCommandStateV1{SchemaVersion: 1, State: types.SupplierBatchCommandStateCompleted, Response: &types.SupplierBatchCommandStatusV1{
		RequestID: "request-1", Status: types.SupplierBatchCommandStateCompleted, ErrorCategory: types.SupplierBatchErrorNone, Result: &types.SupplierBatchCommandResultV1{},
	}}
	require.NoError(t, StoreSupplierBatchSchedulerCommandState(ctx, db, claim, state))
	replay, err := GetSupplierBatchSchedulerCommand(ctx, db, identity, "request-1")
	require.NoError(t, err)
	require.Equal(t, state, replay.State)
	_, err = ClaimSupplierBatchSchedulerCommand(ctx, db, identity, "request-1", struct {
		Mode string `json:"mode"`
	}{"different"}, SupplierBatchSchedulerAuditSlotNext)
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyConflict)

	payload := struct {
		Value int `json:"value"`
	}{1}
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		claim, claimErr := ClaimSupplierAdminCommandTx(tx, 7, "admin.scope", "same-key", payload, "admin")
		if claimErr != nil {
			return claimErr
		}
		return CompleteSupplierAdminCommandTx(tx, claim, 1, payload)
	}))
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		claim, claimErr := ClaimSupplierAdminCommandTx(tx, 8, "admin.scope", "same-key", payload, "admin")
		if claimErr != nil {
			return claimErr
		}
		return CompleteSupplierAdminCommandTx(tx, claim, 2, payload)
	}))
}

func TestSupplierUsagePublicationMigrationAdoptsOnlyProvenLegacyRows(t *testing.T) {
	t.Run("adopts committed generation", func(t *testing.T) {
		db := newSupplierBatchTestDB(t)
		completedAt := int64(1784563201)
		run := SupplierUsageDailyBatchRun{BatchDate: "2026-07-20", DayStart: 1784476800, DayEnd: 1784563200, Status: SupplierDailyBatchStatusCompleted, FenceToken: 7, LogsScanned: 2, SnapshotCount: 2, SummaryCount: 1, CompletedAt: &completedAt}
		require.NoError(t, db.Create(&run).Error)
		require.NoError(t, db.Create(&SupplierUsageDailySummary{
			BatchDate: run.BatchDate, BatchFenceToken: run.FenceToken, DimensionKey: "legacy", BucketStart: run.DayStart,
			SupplierId: 1, ContractId: 1, RateVersionId: 1, ChannelId: 1, StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 2,
		}).Error)
		require.NoError(t, MigrateSupplierUsageDailyBatchPublication(db))
		published, evidence, err := LoadSupplierPublishedDailyBatch(context.Background(), db, run.BatchDate)
		require.NoError(t, err)
		require.Equal(t, int64(7), published.PublishedFenceToken)
		require.Equal(t, types.SupplierPersistedLogCompletenessComplete, evidence.PersistedLogSnapshotCompleteness)
	})
	t.Run("blocks unreconstructable completion", func(t *testing.T) {
		db := newSupplierBatchTestDB(t)
		completedAt := int64(1784563201)
		run := SupplierUsageDailyBatchRun{BatchDate: "2026-07-20", DayStart: 1784476800, DayEnd: 1784563200, Status: SupplierDailyBatchStatusCompleted, FenceToken: 0, CompletedAt: &completedAt}
		require.NoError(t, db.Create(&run).Error)
		require.ErrorIs(t, MigrateSupplierUsageDailyBatchPublication(db), ErrSupplierDailyBatchPublicationInvalid)
		var persisted SupplierUsageDailyBatchRun
		require.NoError(t, db.First(&persisted, run.Id).Error)
		require.Zero(t, persisted.PublishedFenceToken)
		require.Empty(t, persisted.PublishedEvidenceV1)
	})
	t.Run("blocks mismatched legacy counts without publication metadata", func(t *testing.T) {
		db := newSupplierBatchTestDB(t)
		completedAt := int64(1784563201)
		run := SupplierUsageDailyBatchRun{
			BatchDate: "2026-07-20", DayStart: 1784476800, DayEnd: 1784563200,
			Status: SupplierDailyBatchStatusCompleted, FenceToken: 7,
			LogsScanned: 2, SnapshotCount: 1, CompletedAt: &completedAt,
		}
		require.NoError(t, db.Create(&run).Error)

		var before SupplierUsageDailyBatchRun
		require.NoError(t, db.First(&before, run.Id).Error)
		err := MigrateSupplierUsageDailyBatchPublication(db)
		require.ErrorIs(t, err, ErrSupplierDailyBatchPublicationInvalid)

		var after SupplierUsageDailyBatchRun
		require.NoError(t, db.First(&after, run.Id).Error)
		require.Equal(t, before.PublishedFenceToken, after.PublishedFenceToken)
		require.Equal(t, before.PublishedAt, after.PublishedAt)
		require.Equal(t, before.PublishedPersistedLogSnapshotCompleteness, after.PublishedPersistedLogSnapshotCompleteness)
		require.Equal(t, before.PublishedEvidenceV1, after.PublishedEvidenceV1)
	})
}

func TestSupplierUsagePublicationMigrationPreservesPublishedGenerationAcrossReruns(t *testing.T) {
	for _, status := range []string{SupplierDailyBatchStatusRunning, SupplierDailyBatchStatusFailed} {
		t.Run(status, func(t *testing.T) {
			db := newSupplierBatchTestDB(t)
			publishedAt := int64(1784563201)
			evidenceRaw, err := types.EncodeSupplierPublishedEvidenceV1(completePublishedEvidence(1))
			require.NoError(t, err)
			var activeSlot *int
			lockedUntil := int64(0)
			leaseOwner := ""
			if status == SupplierDailyBatchStatusRunning {
				slot := 1
				activeSlot = &slot
				lockedUntil = 1<<62 - 1
				leaseOwner = "newer-rerun"
			}
			run := SupplierUsageDailyBatchRun{
				BatchDate: "2026-07-20", DayStart: 1784476800, DayEnd: 1784563200,
				Status: status, LeaseOwner: leaseOwner, FenceToken: 2, PublishedFenceToken: 1,
				PublishedAt: &publishedAt, PublishedPersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessComplete,
				PublishedEvidenceV1: evidenceRaw, ActiveLeaseSlot: activeSlot, LockedUntil: lockedUntil,
				LogsScanned: 99, SnapshotCount: 0, SummaryCount: 777, ErrorMessage: "mutable rerun state",
				CompletedAt: nil, UpdatedAt: 12345,
			}
			require.NoError(t, db.Create(&run).Error)
			require.NoError(t, db.Create(&[]SupplierUsageDailySummary{
				supplierBatchTestSummary(run.BatchDate, 1, "published", run.DayStart),
				supplierBatchTestSummary(run.BatchDate, 2, "candidate", run.DayStart),
			}).Error)

			var before SupplierUsageDailyBatchRun
			require.NoError(t, db.First(&before, run.Id).Error)
			require.NoError(t, MigrateSupplierUsageDailyBatchPublication(db))
			var after SupplierUsageDailyBatchRun
			require.NoError(t, db.First(&after, run.Id).Error)
			require.Equal(t, before.PublishedFenceToken, after.PublishedFenceToken)
			require.Equal(t, before.PublishedAt, after.PublishedAt)
			require.Equal(t, before.PublishedPersistedLogSnapshotCompleteness, after.PublishedPersistedLogSnapshotCompleteness)
			require.Equal(t, before.PublishedEvidenceV1, after.PublishedEvidenceV1)
			require.Equal(t, before.UpdatedAt, after.UpdatedAt, "valid publication migration must be a byte-for-byte no-op")
		})
	}
}

func TestSupplierUsagePublicationMigrationRejectsPartialOrCorruptPublishedState(t *testing.T) {
	const (
		publishedFenceBit = 1 << iota
		publishedAtBit
		publishedEvidenceBit
		publishedCompletenessBit
		allPublishedMetadataBits = publishedFenceBit | publishedAtBit | publishedEvidenceBit | publishedCompletenessBit
	)
	for metadataBits := 1; metadataBits < allPublishedMetadataBits; metadataBits++ {
		t.Run(fmt.Sprintf("partial metadata %04b", metadataBits), func(t *testing.T) {
			db := newSupplierBatchTestDB(t)
			publishedAt := ptrInt64(1784563201)
			evidenceRaw, err := types.EncodeSupplierPublishedEvidenceV1(completePublishedEvidence(0))
			require.NoError(t, err)
			run := SupplierUsageDailyBatchRun{
				BatchDate: "2026-07-20", DayStart: 1784476800, DayEnd: 1784563200,
				Status: SupplierDailyBatchStatusFailed, FenceToken: 1,
			}
			if metadataBits&publishedFenceBit != 0 {
				run.PublishedFenceToken = 1
			}
			if metadataBits&publishedAtBit != 0 {
				run.PublishedAt = publishedAt
			}
			if metadataBits&publishedEvidenceBit != 0 {
				run.PublishedEvidenceV1 = evidenceRaw
			}
			if metadataBits&publishedCompletenessBit != 0 {
				run.PublishedPersistedLogSnapshotCompleteness = types.SupplierPersistedLogCompletenessComplete
			}
			require.NoError(t, db.Create(&run).Error)

			var before SupplierUsageDailyBatchRun
			require.NoError(t, db.First(&before, run.Id).Error)
			err = MigrateSupplierUsageDailyBatchPublication(db)
			require.ErrorIs(t, err, ErrSupplierDailyBatchPublicationInvalid)
			var after SupplierUsageDailyBatchRun
			require.NoError(t, db.First(&after, run.Id).Error)
			require.Equal(t, before, after)
		})
	}

	tests := []struct {
		name          string
		fence         int64
		published     int64
		publishedAt   *int64
		completeness  string
		evidence      types.SupplierPublishedEvidenceV1
		summaryFences []int64
	}{
		{name: "published fence exceeds fence", fence: 1, published: 2, publishedAt: ptrInt64(1784563201), completeness: types.SupplierPersistedLogCompletenessComplete, evidence: completePublishedEvidence(1), summaryFences: []int64{2}},
		{name: "captured generation missing", fence: 1, published: 1, publishedAt: ptrInt64(1784563201), completeness: types.SupplierPersistedLogCompletenessComplete, evidence: completePublishedEvidence(1)},
		{name: "zero capture has summary", fence: 1, published: 1, publishedAt: ptrInt64(1784563201), completeness: types.SupplierPersistedLogCompletenessComplete, evidence: completePublishedEvidence(0), summaryFences: []int64{1}},
		{name: "summary rows exceed captured", fence: 1, published: 1, publishedAt: ptrInt64(1784563201), completeness: types.SupplierPersistedLogCompletenessComplete, evidence: completePublishedEvidence(1), summaryFences: []int64{1, 1}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			db := newSupplierBatchTestDB(t)
			raw := ""
			if testCase.completeness != "" {
				var err error
				raw, err = types.EncodeSupplierPublishedEvidenceV1(testCase.evidence)
				require.NoError(t, err)
			}
			run := SupplierUsageDailyBatchRun{
				BatchDate: "2026-07-20", DayStart: 1784476800, DayEnd: 1784563200,
				Status: SupplierDailyBatchStatusFailed, FenceToken: testCase.fence, PublishedFenceToken: testCase.published,
				PublishedAt: testCase.publishedAt, PublishedPersistedLogSnapshotCompleteness: testCase.completeness, PublishedEvidenceV1: raw,
			}
			require.NoError(t, db.Create(&run).Error)
			for index, fence := range testCase.summaryFences {
				require.NoError(t, db.Create(&SupplierUsageDailySummary{
					BatchDate: run.BatchDate, BatchFenceToken: fence, DimensionKey: fmt.Sprintf("row-%d", index), BucketStart: run.DayStart,
					SupplierId: 1, ContractId: 1, RateVersionId: 1, ChannelId: 1, StatisticsScope: "business", DataQuality: "authoritative",
				}).Error)
			}
			var before SupplierUsageDailyBatchRun
			require.NoError(t, db.First(&before, run.Id).Error)
			err := MigrateSupplierUsageDailyBatchPublication(db)
			require.ErrorIs(t, err, ErrSupplierDailyBatchPublicationInvalid)
			var after SupplierUsageDailyBatchRun
			require.NoError(t, db.First(&after, run.Id).Error)
			require.Equal(t, before, after)
		})
	}
}

func TestSupplierUsagePublicationMigrationLegacySummaryProofIsExactAndAtomic(t *testing.T) {
	t.Run("exact generation adopted and rerunnable", func(t *testing.T) {
		db := newSupplierBatchTestDB(t)
		completedAt := int64(1784563201)
		run := SupplierUsageDailyBatchRun{
			BatchDate: "2026-07-20", DayStart: 1784476800, DayEnd: 1784563200,
			Status: SupplierDailyBatchStatusCompleted, FenceToken: 3, LogsScanned: 2, SnapshotCount: 2, SummaryCount: 1, CompletedAt: &completedAt,
		}
		require.NoError(t, db.Create(&run).Error)
		require.NoError(t, db.Create(&SupplierUsageDailySummary{
			BatchDate: run.BatchDate, BatchFenceToken: run.FenceToken, DimensionKey: "legacy", BucketStart: run.DayStart,
			SupplierId: 1, ContractId: 1, RateVersionId: 1, ChannelId: 1, StatisticsScope: "business", DataQuality: "authoritative",
		}).Error)
		require.NoError(t, MigrateSupplierUsageDailyBatchPublication(db))
		var once SupplierUsageDailyBatchRun
		require.NoError(t, db.First(&once, run.Id).Error)
		require.Equal(t, int64(3), once.PublishedFenceToken)
		require.NoError(t, MigrateSupplierUsageDailyBatchPublication(db))
		var twice SupplierUsageDailyBatchRun
		require.NoError(t, db.First(&twice, run.Id).Error)
		require.Equal(t, once, twice)
	})

	t.Run("mismatch rolls back earlier adoption", func(t *testing.T) {
		db := newSupplierBatchTestDB(t)
		completedAt := int64(1784563201)
		runs := []SupplierUsageDailyBatchRun{
			{BatchDate: "2026-07-20", DayStart: 1784476800, DayEnd: 1784563200, Status: SupplierDailyBatchStatusCompleted, FenceToken: 3, LogsScanned: 1, SnapshotCount: 1, SummaryCount: 1, CompletedAt: &completedAt},
			{BatchDate: "2026-07-21", DayStart: 1784563200, DayEnd: 1784649600, Status: SupplierDailyBatchStatusCompleted, FenceToken: 4, LogsScanned: 1, SnapshotCount: 1, SummaryCount: 2, CompletedAt: &completedAt},
		}
		require.NoError(t, db.Create(&runs).Error)
		for index := range runs {
			require.NoError(t, db.Create(&SupplierUsageDailySummary{
				BatchDate: runs[index].BatchDate, BatchFenceToken: runs[index].FenceToken, DimensionKey: "legacy", BucketStart: runs[index].DayStart,
				SupplierId: 1, ContractId: 1, RateVersionId: 1, ChannelId: 1, StatisticsScope: "business", DataQuality: "authoritative",
			}).Error)
		}
		err := MigrateSupplierUsageDailyBatchPublication(db)
		require.ErrorIs(t, err, ErrSupplierDailyBatchPublicationInvalid)
		var after []SupplierUsageDailyBatchRun
		require.NoError(t, db.Order("id ASC").Find(&after).Error)
		require.Zero(t, after[0].PublishedFenceToken, "the all-row plan must validate before the first write")
		require.Empty(t, after[0].PublishedEvidenceV1)
	})
}

func TestSupplierUsagePublicationMigrationRejectsStableDoubleActive(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	runs := []SupplierUsageDailyBatchRun{
		{BatchDate: "2026-07-20", DayStart: 1784476800, DayEnd: 1784563200, Status: SupplierDailyBatchStatusRunning, LeaseOwner: "one", FenceToken: 1, LockedUntil: 1<<62 - 1},
		{BatchDate: "2026-07-21", DayStart: 1784563200, DayEnd: 1784649600, Status: SupplierDailyBatchStatusRunning, LeaseOwner: "two", FenceToken: 1, LockedUntil: 1<<62 - 1},
	}
	require.NoError(t, db.Create(&runs).Error)
	require.ErrorIs(t, MigrateSupplierUsageDailyBatchPublication(db), ErrSupplierDailyBatchPublicationInvalid)
	var after []SupplierUsageDailyBatchRun
	require.NoError(t, db.Order("id ASC").Find(&after).Error)
	require.Nil(t, after[0].ActiveLeaseSlot)
	require.Nil(t, after[1].ActiveLeaseSlot)
}

func TestSupplierUsagePublicationMigrationConcurrentCallsConverge(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	completedAt := int64(1784563201)
	run := SupplierUsageDailyBatchRun{
		BatchDate: "2026-07-20", DayStart: 1784476800, DayEnd: 1784563200,
		Status: SupplierDailyBatchStatusCompleted, FenceToken: 5,
		LogsScanned: 1, SnapshotCount: 1, SummaryCount: 1, CompletedAt: &completedAt,
	}
	require.NoError(t, db.Create(&run).Error)
	require.NoError(t, db.Create(&SupplierUsageDailySummary{
		BatchDate: run.BatchDate, BatchFenceToken: run.FenceToken, DimensionKey: "legacy", BucketStart: run.DayStart,
		SupplierId: 1, ContractId: 1, RateVersionId: 1, ChannelId: 1, StatisticsScope: "business", DataQuality: "authoritative",
	}).Error)
	const callers = 12
	errs := make(chan error, callers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- MigrateSupplierUsageDailyBatchPublication(db)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var migrated SupplierUsageDailyBatchRun
	require.NoError(t, db.First(&migrated, run.Id).Error)
	require.Equal(t, run.FenceToken, migrated.PublishedFenceToken)
	_, evidence, err := LoadSupplierPublishedDailyBatch(context.Background(), db, run.BatchDate)
	require.NoError(t, err)
	require.Equal(t, completePublishedEvidence(1), *evidence)
}

func TestSupplierUsagePublicationMigrationDoesNotRegressConcurrentPublish(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	ctx := context.Background()
	lease, err := AcquireSupplierDailyBatch(ctx, db, "2026-07-20", 1784476800, 1784563200, "publisher", time.Now(), time.Minute, false)
	require.NoError(t, err)
	summary := supplierBatchTestSummary(lease.BatchDate, lease.FenceToken, "published", 1784476800)
	require.NoError(t, PersistSupplierDailyBatchPage(ctx, db, lease, []SupplierUsageDailySummary{summary}, 1, 1, 1, 1, time.Minute))

	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		results <- MigrateSupplierUsageDailyBatchPublication(db)
	}()
	go func() {
		<-start
		results <- PublishSupplierDailyBatch(ctx, db, lease, time.Unix(1784563201, 0), completePublishedEvidence(1))
	}()
	close(start)
	require.NoError(t, <-results)
	require.NoError(t, <-results)
	published, evidence, err := LoadSupplierPublishedDailyBatch(ctx, db, lease.BatchDate)
	require.NoError(t, err)
	require.Equal(t, lease.FenceToken, published.PublishedFenceToken)
	require.Equal(t, completePublishedEvidence(1), *evidence)
	require.Nil(t, published.ActiveLeaseSlot)
}

func TestSupplierUsagePublicationMigrationDoesNotClearConcurrentAcquisition(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	ctx := context.Background()
	start := make(chan struct{})
	migrationResult := make(chan error, 1)
	leaseResult := make(chan struct {
		lease SupplierDailyBatchLease
		err   error
	}, 1)
	go func() {
		<-start
		migrationResult <- MigrateSupplierUsageDailyBatchPublication(db)
	}()
	go func() {
		<-start
		lease, err := AcquireSupplierDailyBatch(ctx, db, "2026-07-20", 1784476800, 1784563200, "acquirer", time.Now(), time.Minute, false)
		leaseResult <- struct {
			lease SupplierDailyBatchLease
			err   error
		}{lease: lease, err: err}
	}()
	close(start)
	require.NoError(t, <-migrationResult)
	result := <-leaseResult
	if errors.Is(result.err, ErrSupplierDailyBatchBusy) {
		result.lease, result.err = AcquireSupplierDailyBatch(ctx, db, "2026-07-20", 1784476800, 1784563200, "acquirer", time.Now(), time.Minute, false)
	}
	require.NoError(t, result.err)
	var run SupplierUsageDailyBatchRun
	require.NoError(t, db.First(&run, result.lease.RunId).Error)
	require.Equal(t, SupplierDailyBatchStatusRunning, run.Status)
	require.Equal(t, result.lease.FenceToken, run.FenceToken)
	require.NotNil(t, run.ActiveLeaseSlot)
	require.Equal(t, 1, *run.ActiveLeaseSlot)
}

func TestPublishSupplierDailyBatchRejectsMissingOrInvalidGenerationRows(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		evidence  types.SupplierPublishedEvidenceV1
		summaries int
		wantErr   bool
	}{
		{name: "captured missing", evidence: completePublishedEvidence(1), summaries: 0, wantErr: true},
		{name: "zero capture with row", evidence: completePublishedEvidence(0), summaries: 1, wantErr: true},
		{name: "rows exceed captured", evidence: completePublishedEvidence(1), summaries: 2, wantErr: true},
		{name: "bounded rows", evidence: completePublishedEvidence(2), summaries: 1},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			db := newSupplierBatchTestDB(t)
			ctx := context.Background()
			lease, err := AcquireSupplierDailyBatch(ctx, db, "2026-07-20", 1784476800, 1784563200, "publisher", time.Now(), time.Minute, false)
			require.NoError(t, err)
			summaries := make([]SupplierUsageDailySummary, 0, testCase.summaries)
			for index := 0; index < testCase.summaries; index++ {
				summaries = append(summaries, supplierBatchTestSummary(lease.BatchDate, lease.FenceToken, fmt.Sprintf("row-%d", index), 1784476800))
			}
			if testCase.evidence.LogsScanned > 0 {
				require.NoError(t, PersistSupplierDailyBatchPage(ctx, db, lease, summaries, 1, 1, testCase.evidence.LogsScanned, testCase.evidence.CapturedSnapshotCount, time.Minute))
			} else {
				require.NoError(t, db.Create(&summaries).Error)
			}
			err = PublishSupplierDailyBatch(ctx, db, lease, time.Unix(1784563201, 0), testCase.evidence)
			if testCase.wantErr {
				require.ErrorIs(t, err, ErrSupplierDailyBatchPublicationInvalid)
				var run SupplierUsageDailyBatchRun
				require.NoError(t, db.First(&run, lease.RunId).Error)
				require.Zero(t, run.PublishedFenceToken)
				return
			}
			require.NoError(t, err)
		})
	}
}

func ptrInt64(value int64) *int64 { return &value }

func TestSupplierUsagePublicationMigrationCrossDB(t *testing.T) {
	tests := []struct {
		name, env, expectedDatabase string
		open                        func(string) gorm.Dialector
	}{
		{name: "mysql8", env: "TEST_MYSQL_DSN", expectedDatabase: "supplier_g009_mysql", open: func(dsn string) gorm.Dialector { return mysql.Open(dsn) }},
		{name: "postgres15", env: "TEST_POSTGRES_DSN", expectedDatabase: "supplier_g009_postgres", open: func(dsn string) gorm.Dialector { return postgres.Open(dsn) }},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(testCase.env))
			if dsn == "" {
				t.Skipf("set %s to run the supplier publication migration matrix", testCase.env)
			}
			db, err := gorm.Open(testCase.open(dsn), &gorm.Config{})
			require.NoError(t, err)
			var databaseName string
			if db.Dialector.Name() == "mysql" {
				require.NoError(t, db.Raw("SELECT DATABASE()").Scan(&databaseName).Error)
			} else {
				require.NoError(t, db.Raw("SELECT current_database()").Scan(&databaseName).Error)
			}
			require.Equal(t, testCase.expectedDatabase, databaseName, "cross-DB test is destructive only inside the dedicated supplier test database")
			require.NoError(t, db.AutoMigrate(&SupplierUsageDailySummary{}, &SupplierUsageDailyBatchRun{}))

			const batchDate = "2099-01-01"
			location, loadErr := time.LoadLocation(supplierDailyBatchTimezone)
			require.NoError(t, loadErr)
			day, parseErr := time.ParseInLocation("2006-01-02", batchDate, location)
			require.NoError(t, parseErr)
			cleanup := func() {
				require.NoError(t, db.Where("batch_date = ?", batchDate).Delete(&SupplierUsageDailySummary{}).Error)
				require.NoError(t, db.Where("batch_date = ?", batchDate).Delete(&SupplierUsageDailyBatchRun{}).Error)
			}
			cleanup()
			t.Cleanup(cleanup)

			completedAt := day.AddDate(0, 0, 1).Unix() + 1
			run := SupplierUsageDailyBatchRun{
				BatchDate: batchDate, DayStart: day.Unix(), DayEnd: day.AddDate(0, 0, 1).Unix(),
				Status: SupplierDailyBatchStatusCompleted, FenceToken: 9,
				LogsScanned: 1, SnapshotCount: 1, SummaryCount: 1, CompletedAt: &completedAt,
			}
			require.NoError(t, db.Create(&run).Error)
			require.NoError(t, db.Create(&SupplierUsageDailySummary{
				BatchDate: batchDate, BatchFenceToken: run.FenceToken, DimensionKey: "crossdb", BucketStart: day.Unix(),
				SupplierId: 1, ContractId: 1, RateVersionId: 1, ChannelId: 1, StatisticsScope: "business", DataQuality: "authoritative",
			}).Error)
			require.NoError(t, MigrateSupplierUsageDailyBatchPublication(db))
			var once SupplierUsageDailyBatchRun
			require.NoError(t, db.First(&once, run.Id).Error)
			require.Equal(t, int64(9), once.PublishedFenceToken)
			require.NoError(t, MigrateSupplierUsageDailyBatchPublication(db))
			var twice SupplierUsageDailyBatchRun
			require.NoError(t, db.First(&twice, run.Id).Error)
			require.Equal(t, once, twice)
		})
	}
}

func TestCompleteSupplierDailyBatchRejectsMismatchedLegacyCountsAndRetainsPublishedView(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	ctx := context.Background()
	publishedAt := int64(1784563201)
	incomplete := completePublishedEvidence(1)
	incomplete.ProducerMarkersPresent = 0
	incomplete.CapturedSnapshotCount = 0
	incomplete.DispositionCounts.Captured = 0
	incomplete.FailureCounts.AbsentMarkerAfterCutover = 1
	incomplete.PersistedLogSnapshotCompleteness = types.SupplierPersistedLogCompletenessIncomplete
	incomplete.Warnings = []types.SupplierPublishedWarningV1{{Code: types.SupplierPublishedWarningAbsentMarker, Count: 1, MessageKey: "supply_chain.warning.absent_marker_after_cutover"}}
	evidenceRaw, err := types.EncodeSupplierPublishedEvidenceV1(incomplete)
	require.NoError(t, err)
	activeSlot := 1
	run := SupplierUsageDailyBatchRun{
		BatchDate: "2026-07-20", DayStart: 1784476800, DayEnd: 1784563200,
		Status: SupplierDailyBatchStatusRunning, LeaseOwner: "rerun", FenceToken: 2,
		PublishedFenceToken: 1, PublishedAt: &publishedAt,
		PublishedPersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessIncomplete,
		PublishedEvidenceV1:                       evidenceRaw, ActiveLeaseSlot: &activeSlot, LockedUntil: 1 << 62,
		LogsScanned: 2, SnapshotCount: 1,
	}
	require.NoError(t, db.Create(&run).Error)
	require.NoError(t, db.Create(&SupplierUsageDailySummary{
		BatchDate: run.BatchDate, BatchFenceToken: 1, DimensionKey: "published",
		BucketStart: run.DayStart, SupplierId: 1, ContractId: 1, RateVersionId: 1,
		ChannelId: 1, StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 1,
	}).Error)

	lease := SupplierDailyBatchLease{RunId: run.Id, BatchDate: run.BatchDate, Owner: run.LeaseOwner, FenceToken: run.FenceToken}
	err = CompleteSupplierDailyBatch(ctx, db, lease, time.Unix(1784563202, 0))
	require.ErrorIs(t, err, ErrSupplierDailyBatchPublicationInvalid)

	retained, retainedEvidence, err := LoadSupplierPublishedDailyBatch(ctx, db, run.BatchDate)
	require.NoError(t, err)
	require.Equal(t, int64(1), retained.PublishedFenceToken)
	require.Equal(t, &publishedAt, retained.PublishedAt)
	require.Equal(t, incomplete, *retainedEvidence)
	require.Equal(t, SupplierDailyBatchStatusRunning, retained.Status)
	require.Equal(t, "rerun", retained.LeaseOwner)
	var publishedSummaryCount int64
	require.NoError(t, db.Model(&SupplierUsageDailySummary{}).
		Where("batch_date = ? AND batch_fence_token = ?", run.BatchDate, 1).
		Count(&publishedSummaryCount).Error)
	require.Equal(t, int64(1), publishedSummaryCount)
}

func TestLegacySupplierPublishedEvidenceRequiresExactCapturedCount(t *testing.T) {
	tests := []struct {
		name        string
		logsScanned int64
		captured    int64
		valid       bool
	}{
		{name: "zero", logsScanned: 0, captured: 0, valid: true},
		{name: "all captured", logsScanned: 2, captured: 2, valid: true},
		{name: "negative logs", logsScanned: -1, captured: 0},
		{name: "negative captured", logsScanned: 1, captured: -1},
		{name: "captured greater than logs", logsScanned: 1, captured: 2},
		{name: "partial capture", logsScanned: 2, captured: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidence, err := legacySupplierPublishedEvidence(test.logsScanned, test.captured)
			if !test.valid {
				require.ErrorIs(t, err, ErrSupplierDailyBatchPublicationInvalid)
				require.Equal(t, types.SupplierPublishedEvidenceV1{}, evidence)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.logsScanned, evidence.LogsScanned)
			require.Equal(t, test.captured, evidence.ProducerMarkersPresent)
			require.Equal(t, test.captured, evidence.CapturedSnapshotCount)
			require.Equal(t, test.captured, evidence.DispositionCounts.Captured)
			require.Equal(t, types.SupplierPersistedLogCompletenessComplete, evidence.PersistedLogSnapshotCompleteness)
			require.Empty(t, evidence.Warnings)
			require.Equal(t, types.SupplierPublishedFailureCountsV1{}, evidence.FailureCounts)
		})
	}
}

func TestSupplierSchedulerAndRerunClaimedTakeover(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	identity, err := DigestSupplierBatchTrustedIdentity("trusted-job")
	require.NoError(t, err)
	scheduler, err := ClaimSupplierBatchSchedulerCommand(ctx, db, identity, "takeover-scheduler", map[string]any{"mode": "catch_up"}, SupplierBatchSchedulerAuditSlotCurrent)
	require.NoError(t, err)
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Where("id = ?", scheduler.Command.Id).UpdateColumn("updated_at", now.Add(-10*time.Minute).Unix()).Error)
	scheduler, err = GetSupplierBatchSchedulerCommand(ctx, db, identity, "takeover-scheduler")
	require.NoError(t, err)
	scheduler, err = TakeoverSupplierBatchSchedulerCommand(ctx, db, scheduler, now, time.Minute)
	require.NoError(t, err)
	require.True(t, scheduler.Claimed)

	payload := map[string]any{"date": "2026-07-20", "reason": "repair", "expected_fence": 3}
	rerun, err := ClaimSupplierDailyReportRerunCommand(ctx, db, 7, "2026-07-20", "takeover-rerun", payload)
	require.NoError(t, err)
	require.NoError(t, db.Model(&SupplierAdminCommand{}).Where("id = ?", rerun.Command.Id).UpdateColumn("updated_at", now.Add(-10*time.Minute).Unix()).Error)
	rerun, err = GetSupplierDailyReportRerunCommand(ctx, db, 7, "2026-07-20", "takeover-rerun")
	require.NoError(t, err)
	rerun, err = TakeoverSupplierDailyReportRerunCommand(ctx, db, rerun, now, time.Minute)
	require.NoError(t, err)
	require.True(t, rerun.Claimed)
	_, err = ClaimSupplierDailyReportRerunCommand(ctx, db, 7, "2026-07-20", "takeover-rerun", map[string]any{"date": "2026-07-20", "reason": "different", "expected_fence": 3})
	require.ErrorIs(t, err, ErrSupplierAdminIdempotencyConflict)
}

func TestAdoptSupplierBatchSchedulerClaimCASAndIdentity(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	identity, err := DigestSupplierBatchTrustedIdentity("trusted-adoption-job")
	require.NoError(t, err)
	semantics := struct {
		Mode string `json:"mode"`
	}{Mode: "catch_up"}
	original, err := ClaimSupplierBatchSchedulerCommand(ctx, db, identity, "adopt-request", semantics, SupplierBatchSchedulerAuditSlotCurrent)
	require.NoError(t, err)
	originalToken := original.Command.ClaimToken
	originalPayload := original.Command.PayloadDigest
	originalIdentity := *original.Command.TrustedJobIdentityDigest

	// Adoption deliberately opens no inner transaction: an outer rollback must
	// roll the token rotation back with the rest of the scheduling decision.
	tx := db.Begin()
	require.NoError(t, tx.Error)
	rolledBack, err := GetSupplierBatchSchedulerCommand(ctx, tx, identity, "adopt-request")
	require.NoError(t, err)
	rolledBack, err = AdoptSupplierBatchSchedulerClaim(ctx, tx, rolledBack, now)
	require.NoError(t, err)
	require.NotEqual(t, originalToken, rolledBack.Command.ClaimToken)
	require.NoError(t, tx.Rollback().Error)
	persisted, err := GetSupplierBatchSchedulerCommand(ctx, db, identity, "adopt-request")
	require.NoError(t, err)
	require.Equal(t, originalToken, persisted.Command.ClaimToken)

	left, err := GetSupplierBatchSchedulerCommand(ctx, db, identity, "adopt-request")
	require.NoError(t, err)
	right, err := GetSupplierBatchSchedulerCommand(ctx, db, identity, "adopt-request")
	require.NoError(t, err)
	claims := []*SupplierBatchSchedulerCommandClaim{left, right}
	errs := make([]error, len(claims))
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range claims {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			claims[i], errs[i] = AdoptSupplierBatchSchedulerClaim(ctx, db, claims[i], now.Add(time.Second))
		}(i)
	}
	close(start)
	wg.Wait()
	winners := 0
	for i := range errs {
		if errs[i] == nil {
			winners++
			require.True(t, claims[i].Claimed)
			require.False(t, claims[i].Replayed)
		} else {
			require.ErrorIs(t, errs[i], ErrSupplierDailyBatchBusy)
		}
	}
	require.Equal(t, 1, winners)

	persisted, err = GetSupplierBatchSchedulerCommand(ctx, db, identity, "adopt-request")
	require.NoError(t, err)
	require.Equal(t, SupplierBatchSchedulerCommandScopeCatchUp, persisted.Command.Scope)
	require.Equal(t, "adopt-request", persisted.Command.IdempotencyKey)
	require.Equal(t, originalPayload, persisted.Command.PayloadDigest)
	require.NotNil(t, persisted.Command.TrustedJobIdentityDigest)
	require.Equal(t, originalIdentity, *persisted.Command.TrustedJobIdentityDigest)
	require.Equal(t, types.SupplierBatchCommandStateClaimed, persisted.State.State)
	terminalJSON, err := types.EncodeSupplierBatchCommandStateV1(types.SupplierBatchCommandStateV1{SchemaVersion: 1, State: types.SupplierBatchCommandStateCompleted, Response: &types.SupplierBatchCommandStatusV1{
		RequestID: "adopt-request", Status: types.SupplierBatchCommandStateCompleted, ErrorCategory: types.SupplierBatchErrorNone, Result: &types.SupplierBatchCommandResultV1{},
	}})
	require.NoError(t, err)
	forged := *persisted
	forged.Command.StatusJson = terminalJSON
	_, err = AdoptSupplierBatchSchedulerClaim(ctx, db, &forged, now.Add(2*time.Second))
	require.ErrorIs(t, err, ErrSupplierAdminCommandIncomplete)
}

func TestAdoptSupplierBatchSchedulerClaimOuterTransactionStoresTerminal(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	ctx := context.Background()
	identity, err := DigestSupplierBatchTrustedIdentity("trusted-terminal-job")
	require.NoError(t, err)
	_, err = ClaimSupplierBatchSchedulerCommand(ctx, db, identity, "terminal-request", map[string]string{"mode": "catch_up"}, SupplierBatchSchedulerAuditSlotCurrent)
	require.NoError(t, err)

	err = db.Transaction(func(tx *gorm.DB) error {
		replay, getErr := GetSupplierBatchSchedulerCommand(ctx, tx, identity, "terminal-request")
		if getErr != nil {
			return getErr
		}
		adopted, adoptErr := AdoptSupplierBatchSchedulerClaim(ctx, tx, replay, time.Now().UTC())
		if adoptErr != nil {
			return adoptErr
		}
		terminal := types.SupplierBatchCommandStateV1{SchemaVersion: 1, State: types.SupplierBatchCommandStateCompleted, Response: &types.SupplierBatchCommandStatusV1{
			RequestID: "terminal-request", Status: types.SupplierBatchCommandStateCompleted, ErrorCategory: types.SupplierBatchErrorNone, Result: &types.SupplierBatchCommandResultV1{},
		}}
		return StoreSupplierBatchSchedulerCommandState(ctx, tx, adopted, terminal)
	})
	require.NoError(t, err)
	stored, err := GetSupplierBatchSchedulerCommand(ctx, db, identity, "terminal-request")
	require.NoError(t, err)
	require.Equal(t, types.SupplierBatchCommandStateCompleted, stored.State.State)
}

func TestReconcileSupplierBatchSchedulerCommandStateConcurrentOneWinner(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	ctx := context.Background()
	identity, _, running := newRunningSupplierBatchSchedulerReplay(t, db, "reconcile-race-job", "reconcile-race", 9)
	left, err := GetSupplierBatchSchedulerCommand(ctx, db, identity, "reconcile-race")
	require.NoError(t, err)
	right, err := GetSupplierBatchSchedulerCommand(ctx, db, identity, "reconcile-race")
	require.NoError(t, err)
	terminal := supplierBatchReconciledTerminalState(running, false)

	replays := []*SupplierBatchSchedulerCommandClaim{left, right}
	results := make([]*SupplierBatchSchedulerCommandClaim, len(replays))
	errs := make([]error, len(replays))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for index := range replays {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			results[index], errs[index] = ReconcileSupplierBatchSchedulerCommandState(ctx, db, replays[index], terminal)
		}(index)
	}
	close(start)
	wg.Wait()

	winners := 0
	replayed := 0
	for index := range errs {
		require.NoError(t, errs[index])
		require.NotNil(t, results[index])
		if results[index].Replayed {
			replayed++
		} else {
			winners++
		}
		require.Equal(t, terminal, results[index].State)
	}
	require.Equal(t, 1, winners)
	require.Equal(t, 1, replayed)

	stored, err := GetSupplierBatchSchedulerCommand(ctx, db, identity, "reconcile-race")
	require.NoError(t, err)
	require.Equal(t, terminal, stored.State)
	require.Equal(t, left.Command.ClaimToken, stored.Command.ClaimToken)
	require.Equal(t, left.Command.PayloadDigest, stored.Command.PayloadDigest)
	require.Equal(t, left.Command.TrustedJobIdentityDigest, stored.Command.TrustedJobIdentityDigest)
}

func TestReconcileSupplierBatchSchedulerCommandStateRejectsWrongAnchorsAndFence(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	ctx := context.Background()
	identity, replay, running := newRunningSupplierBatchSchedulerReplay(t, db, "reconcile-guard-job", "reconcile-guard", 11)
	terminal := supplierBatchReconciledTerminalState(running, false)

	t.Run("identity", func(t *testing.T) {
		forged := *replay
		forged.Command = replay.Command
		wrongIdentity := strings.Repeat("f", 64)
		forged.Command.TrustedJobIdentityDigest = &wrongIdentity
		_, err := ReconcileSupplierBatchSchedulerCommandState(ctx, db, &forged, terminal)
		require.ErrorIs(t, err, ErrSupplierAdminCommandIncomplete)
	})

	t.Run("claim token", func(t *testing.T) {
		forged := *replay
		forged.Command = replay.Command
		forged.Command.ClaimToken = strings.Repeat("0", 32)
		_, err := ReconcileSupplierBatchSchedulerCommandState(ctx, db, &forged, terminal)
		require.ErrorIs(t, err, ErrSupplierAdminCommandIncomplete)
	})

	t.Run("old status", func(t *testing.T) {
		forged := *replay
		forged.State = types.SupplierBatchCommandStateV1{SchemaVersion: types.SupplierBatchCommandSchemaVersion, State: types.SupplierBatchCommandStateClaimed}
		_, err := ReconcileSupplierBatchSchedulerCommandState(ctx, db, &forged, terminal)
		require.ErrorIs(t, err, ErrSupplierAdminCommandIncomplete)
	})

	t.Run("terminal fence", func(t *testing.T) {
		wrongFence := supplierBatchReconciledTerminalState(running, false)
		wrongFence.Response.FenceToken++
		wrongFence.Response.PublishedFenceToken++
		_, err := ReconcileSupplierBatchSchedulerCommandState(ctx, db, replay, wrongFence)
		require.ErrorIs(t, err, ErrSupplierAdminCommandIncomplete)
	})

	t.Run("completed published fence", func(t *testing.T) {
		wrongPublishedFence := supplierBatchReconciledTerminalState(running, false)
		wrongPublishedFence.Response.PublishedFenceToken++
		_, err := ReconcileSupplierBatchSchedulerCommandState(ctx, db, replay, wrongPublishedFence)
		require.ErrorIs(t, err, ErrSupplierAdminCommandIncomplete)
	})

	stored, err := GetSupplierBatchSchedulerCommand(ctx, db, identity, "reconcile-guard")
	require.NoError(t, err)
	require.Equal(t, types.SupplierBatchCommandStateRunning, stored.State.State)
	require.Equal(t, replay.Command.StatusJson, stored.Command.StatusJson)
}

func TestReconcileSupplierBatchSchedulerCommandStateIdempotentReplayAndNoBlindOverwrite(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	ctx := context.Background()
	identity, replay, running := newRunningSupplierBatchSchedulerReplay(t, db, "reconcile-replay-job", "reconcile-replay", 13)
	terminal := supplierBatchReconciledTerminalState(running, false)

	applied, err := ReconcileSupplierBatchSchedulerCommandState(ctx, db, replay, terminal)
	require.NoError(t, err)
	require.False(t, applied.Replayed)
	require.Equal(t, terminal, applied.State)

	replayed, err := ReconcileSupplierBatchSchedulerCommandState(ctx, db, replay, terminal)
	require.NoError(t, err)
	require.True(t, replayed.Replayed)
	require.Equal(t, terminal, replayed.State)

	divergent := supplierBatchReconciledTerminalState(running, true)
	_, err = ReconcileSupplierBatchSchedulerCommandState(ctx, db, replay, divergent)
	require.ErrorIs(t, err, ErrSupplierDailyBatchBusy)
	stored, err := GetSupplierBatchSchedulerCommand(ctx, db, identity, "reconcile-replay")
	require.NoError(t, err)
	require.Equal(t, terminal, stored.State)
}

func TestReconcileSupplierBatchSchedulerCommandStateAcceptsStrictFailedCategories(t *testing.T) {
	categories := []string{
		types.SupplierBatchErrorFenceLost,
		types.SupplierBatchErrorLeaseExpired,
		types.SupplierBatchErrorExecutionFailed,
		types.SupplierBatchErrorReadFailed,
		types.SupplierBatchErrorPublicationFailed,
	}
	for index, category := range categories {
		t.Run(category, func(t *testing.T) {
			db := newSupplierBatchTestDB(t)
			requestID := fmt.Sprintf("reconcile-failed-%d", index)
			identity, replay, running := newRunningSupplierBatchSchedulerReplay(t, db, "reconcile-failed-job", requestID, 17)
			terminal := supplierBatchReconciledFailedState(running, category, true)

			reconciled, err := ReconcileSupplierBatchSchedulerCommandState(context.Background(), db, replay, terminal)
			require.NoError(t, err)
			require.False(t, reconciled.Replayed)
			require.Equal(t, terminal, reconciled.State)

			stored, err := GetSupplierBatchSchedulerCommand(context.Background(), db, identity, requestID)
			require.NoError(t, err)
			require.Equal(t, terminal, stored.State)
		})
	}
}

func TestReconcileSupplierBatchSchedulerCommandStateFailedTerminalPreservesNewerPublishedFence(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	identity, replay, running := newRunningSupplierBatchSchedulerReplay(t, db, "reconcile-newer-fence-job", "reconcile-newer-fence", 17)
	terminal := supplierBatchReconciledFailedState(running, types.SupplierBatchErrorFenceLost, true)
	terminal.Response.PublishedFenceToken = 23

	reconciled, err := ReconcileSupplierBatchSchedulerCommandState(context.Background(), db, replay, terminal)
	require.NoError(t, err)
	require.Equal(t, int64(23), reconciled.State.Response.PublishedFenceToken)
	stored, err := GetSupplierBatchSchedulerCommand(context.Background(), db, identity, running.Response.RequestID)
	require.NoError(t, err)
	require.Equal(t, int64(23), stored.State.Response.PublishedFenceToken)
}

func TestPublishSupplierDailyBatchTxRollsBackWithCommandCASFailure(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	ctx := context.Background()
	lease, identity, replay, running := newSupplierBatchSchedulerLeaseFixture(t, db, "publish-rollback-job", "publish-rollback")
	var before SupplierUsageDailyBatchRun
	require.NoError(t, db.First(&before, lease.RunId).Error)
	terminal := supplierBatchReconciledTerminalState(running, false)
	forged := *replay
	forged.Command = replay.Command
	forged.Command.ClaimToken = strings.Repeat("0", 32)

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if publishErr := PublishSupplierDailyBatchTx(ctx, tx, lease, time.Unix(1784563201, 0), completePublishedEvidence(1)); publishErr != nil {
			return publishErr
		}
		_, reconcileErr := ReconcileSupplierBatchSchedulerCommandState(ctx, tx, &forged, terminal)
		return reconcileErr
	})
	require.ErrorIs(t, err, ErrSupplierAdminCommandIncomplete)

	var after SupplierUsageDailyBatchRun
	require.NoError(t, db.First(&after, lease.RunId).Error)
	require.Equal(t, before.Status, after.Status)
	require.Equal(t, before.PublishedFenceToken, after.PublishedFenceToken)
	require.Equal(t, before.PublishedAt, after.PublishedAt)
	require.Equal(t, before.PublishedPersistedLogSnapshotCompleteness, after.PublishedPersistedLogSnapshotCompleteness)
	require.Equal(t, before.PublishedEvidenceV1, after.PublishedEvidenceV1)
	require.Equal(t, before.LeaseOwner, after.LeaseOwner)
	require.Equal(t, before.ActiveLeaseSlot, after.ActiveLeaseSlot)
	var candidateCount int64
	require.NoError(t, db.Model(&SupplierUsageDailySummary{}).
		Where("batch_date = ? AND batch_fence_token = ?", lease.BatchDate, lease.FenceToken).
		Count(&candidateCount).Error)
	require.Equal(t, int64(1), candidateCount, "generated summary remains an unpublished candidate after rollback")
	stored, err := GetSupplierBatchSchedulerCommand(ctx, db, identity, running.Response.RequestID)
	require.NoError(t, err)
	require.Equal(t, running, stored.State)
}

func TestPublishSupplierDailyBatchTxCommitsWithCommandTerminal(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	ctx := context.Background()
	lease, identity, replay, running := newSupplierBatchSchedulerLeaseFixture(t, db, "publish-commit-job", "publish-commit")
	terminal := supplierBatchReconciledTerminalState(running, false)
	completedAt := time.Unix(1784563201, 0)
	evidence := completePublishedEvidence(1)

	require.NoError(t, db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := PublishSupplierDailyBatchTx(ctx, tx, lease, completedAt, evidence); err != nil {
			return err
		}
		_, err := ReconcileSupplierBatchSchedulerCommandState(ctx, tx, replay, terminal)
		return err
	}))

	published, storedEvidence, err := LoadSupplierPublishedDailyBatch(ctx, db, lease.BatchDate)
	require.NoError(t, err)
	require.Equal(t, SupplierDailyBatchStatusCompleted, published.Status)
	require.Equal(t, lease.FenceToken, published.PublishedFenceToken)
	require.NotNil(t, published.PublishedAt)
	require.Equal(t, completedAt.Unix(), *published.PublishedAt)
	require.Equal(t, evidence, *storedEvidence)
	stored, err := GetSupplierBatchSchedulerCommand(ctx, db, identity, running.Response.RequestID)
	require.NoError(t, err)
	require.Equal(t, terminal, stored.State)
	require.Equal(t, replay.Command.ClaimToken, stored.Command.ClaimToken)
}

func TestFailSupplierDailyBatchTxRollsBackWithCommandCASFailure(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	ctx := context.Background()
	lease, identity, replay, running := newSupplierBatchSchedulerLeaseFixture(t, db, "fail-rollback-job", "fail-rollback")
	terminal := supplierBatchReconciledFailedState(running, types.SupplierBatchErrorExecutionFailed, true)
	forged := *replay
	forged.Command = replay.Command
	forged.Command.ClaimToken = strings.Repeat("0", 32)

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if failErr := FailSupplierDailyBatchTx(ctx, tx, lease, errors.New("scan failed")); failErr != nil {
			return failErr
		}
		_, reconcileErr := ReconcileSupplierBatchSchedulerCommandState(ctx, tx, &forged, terminal)
		return reconcileErr
	})
	require.ErrorIs(t, err, ErrSupplierAdminCommandIncomplete)

	var run SupplierUsageDailyBatchRun
	require.NoError(t, db.First(&run, lease.RunId).Error)
	require.Equal(t, SupplierDailyBatchStatusRunning, run.Status)
	require.Equal(t, lease.Owner, run.LeaseOwner)
	require.NotNil(t, run.ActiveLeaseSlot)
	require.Positive(t, run.LockedUntil)
	require.Empty(t, run.ErrorMessage)
	var candidateCount int64
	require.NoError(t, db.Model(&SupplierUsageDailySummary{}).
		Where("batch_date = ? AND batch_fence_token = ?", lease.BatchDate, lease.FenceToken).
		Count(&candidateCount).Error)
	require.Equal(t, int64(1), candidateCount)
	stored, err := GetSupplierBatchSchedulerCommand(ctx, db, identity, running.Response.RequestID)
	require.NoError(t, err)
	require.Equal(t, running, stored.State)
}

func TestFailSupplierDailyBatchTxCommitsWithTerminalAfterRequestCancellation(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	requestCtx, cancel := context.WithCancel(context.Background())
	cancel()
	recoveryCtx := context.WithoutCancel(requestCtx)
	require.ErrorIs(t, requestCtx.Err(), context.Canceled)
	require.NoError(t, recoveryCtx.Err())
	lease, identity, replay, running := newSupplierBatchSchedulerLeaseFixture(t, db, "fail-cancel-job", "fail-cancel")
	terminal := supplierBatchReconciledFailedState(running, types.SupplierBatchErrorReadFailed, true)
	cause := errors.New("scan supplier accounting: canceled upstream read")

	require.NoError(t, db.WithContext(recoveryCtx).Transaction(func(tx *gorm.DB) error {
		if err := FailSupplierDailyBatchTx(recoveryCtx, tx, lease, cause); err != nil {
			return err
		}
		_, err := ReconcileSupplierBatchSchedulerCommandState(recoveryCtx, tx, replay, terminal)
		return err
	}))

	var run SupplierUsageDailyBatchRun
	require.NoError(t, db.First(&run, lease.RunId).Error)
	require.Equal(t, SupplierDailyBatchStatusFailed, run.Status)
	require.Empty(t, run.LeaseOwner)
	require.Nil(t, run.ActiveLeaseSlot)
	require.Zero(t, run.LockedUntil)
	require.Equal(t, cause.Error(), run.ErrorMessage)
	var candidateCount int64
	require.NoError(t, db.Model(&SupplierUsageDailySummary{}).
		Where("batch_date = ? AND batch_fence_token = ?", lease.BatchDate, lease.FenceToken).
		Count(&candidateCount).Error)
	require.Zero(t, candidateCount)
	stored, err := GetSupplierBatchSchedulerCommand(context.Background(), db, identity, running.Response.RequestID)
	require.NoError(t, err)
	require.Equal(t, terminal, stored.State)
}

func TestReconcileSupplierDailyReportRerunCommandStateAnchoredReplay(t *testing.T) {
	terminalCases := []struct {
		name  string
		state func(types.SupplierBatchCommandStateV1) types.SupplierBatchCommandStateV1
	}{
		{name: "completed", state: func(running types.SupplierBatchCommandStateV1) types.SupplierBatchCommandStateV1 {
			return supplierBatchReconciledTerminalState(running, false)
		}},
		{name: "failed", state: func(running types.SupplierBatchCommandStateV1) types.SupplierBatchCommandStateV1 {
			terminal := supplierBatchReconciledFailedState(running, types.SupplierBatchErrorPublicationFailed, false)
			terminal.Response.PublishedFenceToken = running.Response.FenceToken + 1
			return terminal
		}},
	}
	for index, terminalCase := range terminalCases {
		t.Run(terminalCase.name, func(t *testing.T) {
			db := newSupplierBatchTestDB(t)
			requestID := fmt.Sprintf("rerun-reconcile-%d", index)
			replay, running := newRunningSupplierDailyReportRerunReplay(t, db, 7, "2026-07-20", requestID, 5, 4)
			terminal := terminalCase.state(running)

			forged := *replay
			forged.Command = replay.Command
			forged.Command.ActorId++
			_, err := ReconcileSupplierDailyReportRerunCommandState(context.Background(), db, &forged, terminal)
			require.ErrorIs(t, err, ErrSupplierAdminCommandIncomplete)

			applied, err := ReconcileSupplierDailyReportRerunCommandState(context.Background(), db, replay, terminal)
			require.NoError(t, err)
			require.False(t, applied.Replayed)
			require.Equal(t, terminal, applied.State)
			replayed, err := ReconcileSupplierDailyReportRerunCommandState(context.Background(), db, replay, terminal)
			require.NoError(t, err)
			require.True(t, replayed.Replayed)

			divergent := supplierBatchReconciledFailedState(running, types.SupplierBatchErrorExecutionFailed, true)
			if terminalCase.name == "failed" {
				divergent = supplierBatchReconciledTerminalState(running, false)
			}
			_, err = ReconcileSupplierDailyReportRerunCommandState(context.Background(), db, replay, divergent)
			require.ErrorIs(t, err, ErrSupplierDailyBatchBusy)
		})
	}
}

func TestReconcileSupplierDailyReportRerunCommandStateRejectsPublishedFenceRegression(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	replay, running := newRunningSupplierDailyReportRerunReplay(t, db, 7, "2026-07-20", "rerun-fence-regression", 5, 4)
	terminal := supplierBatchReconciledFailedState(running, types.SupplierBatchErrorFenceLost, true)
	terminal.Response.PublishedFenceToken = 3

	_, err := ReconcileSupplierDailyReportRerunCommandState(context.Background(), db, replay, terminal)
	require.ErrorIs(t, err, ErrSupplierAdminCommandIncomplete)
	stored, err := GetSupplierDailyReportRerunCommand(context.Background(), db, 7, "2026-07-20", running.Response.RequestID)
	require.NoError(t, err)
	require.Equal(t, running, stored.State)
}

func TestReconcileSupplierDailyReportRerunCommandStateConcurrentOneWinner(t *testing.T) {
	db := newSupplierBatchTestDB(t)
	_, running := newRunningSupplierDailyReportRerunReplay(t, db, 7, "2026-07-20", "rerun-reconcile-race", 5, 4)
	left, err := GetSupplierDailyReportRerunCommand(context.Background(), db, 7, "2026-07-20", running.Response.RequestID)
	require.NoError(t, err)
	right, err := GetSupplierDailyReportRerunCommand(context.Background(), db, 7, "2026-07-20", running.Response.RequestID)
	require.NoError(t, err)
	terminal := supplierBatchReconciledFailedState(running, types.SupplierBatchErrorFenceLost, true)
	terminal.Response.PublishedFenceToken = 6
	replays := []*SupplierDailyReportRerunCommandClaim{left, right}
	results := make([]*SupplierDailyReportRerunCommandClaim, len(replays))
	errs := make([]error, len(replays))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for index := range replays {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			results[index], errs[index] = ReconcileSupplierDailyReportRerunCommandState(context.Background(), db, replays[index], terminal)
		}(index)
	}
	close(start)
	wg.Wait()

	winners := 0
	replayed := 0
	for index := range errs {
		require.NoError(t, errs[index])
		require.NotNil(t, results[index])
		if results[index].Replayed {
			replayed++
		} else {
			winners++
		}
		require.Equal(t, terminal, results[index].State)
	}
	require.Equal(t, 1, winners)
	require.Equal(t, 1, replayed)
}

func newRunningSupplierBatchSchedulerReplay(t *testing.T, db *gorm.DB, identityName, requestID string, fence int64) ([]byte, *SupplierBatchSchedulerCommandClaim, types.SupplierBatchCommandStateV1) {
	t.Helper()
	ctx := context.Background()
	identity, err := DigestSupplierBatchTrustedIdentity(identityName)
	require.NoError(t, err)
	claim, err := ClaimSupplierBatchSchedulerCommand(ctx, db, identity, requestID, map[string]string{"mode": "catch_up"}, SupplierBatchSchedulerAuditSlotCurrent)
	require.NoError(t, err)
	batchDate := "2026-07-20"
	runID := int64(41)
	lockedUntil := time.Now().UTC().Add(time.Hour).Truncate(time.Second).Format(time.RFC3339)
	running := types.SupplierBatchCommandStateV1{SchemaVersion: types.SupplierBatchCommandSchemaVersion, State: types.SupplierBatchCommandStateRunning, Response: &types.SupplierBatchCommandStatusV1{
		RequestID: requestID, BatchDate: &batchDate, RunID: &runID, Status: types.SupplierBatchCommandStateRunning,
		FenceToken: fence, LockedUntil: &lockedUntil, ErrorCategory: types.SupplierBatchErrorNone,
	}}
	require.NoError(t, StoreSupplierBatchSchedulerCommandState(ctx, db, claim, running))
	replay, err := GetSupplierBatchSchedulerCommand(ctx, db, identity, requestID)
	require.NoError(t, err)
	require.False(t, replay.Claimed)
	require.True(t, replay.Replayed)
	return identity, replay, running
}

func newSupplierBatchSchedulerLeaseFixture(t *testing.T, db *gorm.DB, identityName, requestID string) (SupplierDailyBatchLease, []byte, *SupplierBatchSchedulerCommandClaim, types.SupplierBatchCommandStateV1) {
	t.Helper()
	ctx := context.Background()
	lease, err := AcquireSupplierDailyBatch(ctx, db, "2026-07-20", 1784476800, 1784563200, "owner-"+requestID, time.Now(), time.Minute, false)
	require.NoError(t, err)
	summary := SupplierUsageDailySummary{
		DimensionKey: "candidate", BucketStart: 1784476800, SupplierId: 1, ContractId: 1, RateVersionId: 1, ChannelId: 1,
		StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 1,
	}
	require.NoError(t, PersistSupplierDailyBatchPage(ctx, db, lease, []SupplierUsageDailySummary{summary}, 1, 1, 1, 1, time.Minute))
	identity, err := DigestSupplierBatchTrustedIdentity(identityName)
	require.NoError(t, err)
	claim, err := ClaimSupplierBatchSchedulerCommand(ctx, db, identity, requestID, map[string]string{"mode": "catch_up"}, SupplierBatchSchedulerAuditSlotCurrent)
	require.NoError(t, err)
	lockedUntil := time.Now().UTC().Add(time.Hour).Truncate(time.Second).Format(time.RFC3339)
	batchDate := lease.BatchDate
	runID := lease.RunId
	running := types.SupplierBatchCommandStateV1{SchemaVersion: types.SupplierBatchCommandSchemaVersion, State: types.SupplierBatchCommandStateRunning, Response: &types.SupplierBatchCommandStatusV1{
		RequestID: requestID, BatchDate: &batchDate, RunID: &runID, Status: types.SupplierBatchCommandStateRunning,
		FenceToken: lease.FenceToken, PublishedFenceToken: 0, LockedUntil: &lockedUntil, ErrorCategory: types.SupplierBatchErrorNone,
	}}
	require.NoError(t, StoreSupplierBatchSchedulerCommandState(ctx, db, claim, running))
	replay, err := GetSupplierBatchSchedulerCommand(ctx, db, identity, requestID)
	require.NoError(t, err)
	return lease, identity, replay, running
}

func newRunningSupplierDailyReportRerunReplay(t *testing.T, db *gorm.DB, actorID int, batchDate, requestID string, fence, publishedFence int64) (*SupplierDailyReportRerunCommandClaim, types.SupplierBatchCommandStateV1) {
	t.Helper()
	ctx := context.Background()
	claim, err := ClaimSupplierDailyReportRerunCommand(ctx, db, actorID, batchDate, requestID, map[string]any{
		"date": batchDate, "expected_fence": publishedFence,
	})
	require.NoError(t, err)
	runID := int64(73)
	lockedUntil := time.Now().UTC().Add(time.Hour).Truncate(time.Second).Format(time.RFC3339)
	running := types.SupplierBatchCommandStateV1{SchemaVersion: types.SupplierBatchCommandSchemaVersion, State: types.SupplierBatchCommandStateRunning, Response: &types.SupplierBatchCommandStatusV1{
		RequestID: requestID, BatchDate: &batchDate, RunID: &runID, Status: types.SupplierBatchCommandStateRunning,
		FenceToken: fence, PublishedFenceToken: publishedFence, LockedUntil: &lockedUntil, ErrorCategory: types.SupplierBatchErrorNone,
	}}
	require.NoError(t, StoreSupplierDailyReportRerunCommandState(ctx, db, claim, running))
	replay, err := GetSupplierDailyReportRerunCommand(ctx, db, actorID, batchDate, requestID)
	require.NoError(t, err)
	return replay, running
}

func supplierBatchReconciledTerminalState(running types.SupplierBatchCommandStateV1, remaining bool) types.SupplierBatchCommandStateV1 {
	batchDate := *running.Response.BatchDate
	runID := *running.Response.RunID
	var nextDate *string
	if remaining {
		value := "2026-07-21"
		nextDate = &value
	}
	return types.SupplierBatchCommandStateV1{SchemaVersion: types.SupplierBatchCommandSchemaVersion, State: types.SupplierBatchCommandStateCompleted, Response: &types.SupplierBatchCommandStatusV1{
		RequestID: running.Response.RequestID, BatchDate: &batchDate, RunID: &runID, Status: types.SupplierBatchCommandStateCompleted,
		FenceToken: running.Response.FenceToken, PublishedFenceToken: running.Response.FenceToken, ErrorCategory: types.SupplierBatchErrorNone,
		Result: &types.SupplierBatchCommandResultV1{ProcessedDays: 1, RemainingWork: remaining, NextBatchDate: nextDate},
	}}
}

func supplierBatchReconciledFailedState(running types.SupplierBatchCommandStateV1, category string, remaining bool) types.SupplierBatchCommandStateV1 {
	batchDate := *running.Response.BatchDate
	runID := *running.Response.RunID
	var nextDate *string
	if remaining {
		value := batchDate
		nextDate = &value
	}
	return types.SupplierBatchCommandStateV1{SchemaVersion: types.SupplierBatchCommandSchemaVersion, State: types.SupplierBatchCommandStateFailed, Response: &types.SupplierBatchCommandStatusV1{
		RequestID: running.Response.RequestID, BatchDate: &batchDate, RunID: &runID, Status: types.SupplierBatchCommandStateFailed,
		FenceToken: running.Response.FenceToken, PublishedFenceToken: running.Response.PublishedFenceToken, ErrorCategory: category,
		Result: &types.SupplierBatchCommandResultV1{ProcessedDays: 0, RemainingWork: remaining, NextBatchDate: nextDate},
	}}
}
