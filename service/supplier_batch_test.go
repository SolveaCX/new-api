package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type supplierBatchAsyncResult struct {
	response dto.SupplierBatchStatusResponse
	err      error
}

func seedSupplierSchedulerRunningCommand(t *testing.T, db *gorm.DB, principal dto.SupplierBatchSchedulerPrincipal, requestID, batchDate string, runID, fence int64, lockedUntil time.Time) {
	t.Helper()
	digest, err := model.DigestSupplierBatchTrustedIdentity(principal.TrustedJobIdentity)
	require.NoError(t, err)
	claim, err := model.ClaimSupplierBatchSchedulerCommand(context.Background(), db, digest, requestID, supplierBatchCatchUpSemanticsV1{SchemaVersion: 1, Operation: "catch_up"}, principal.AuditSlot)
	require.NoError(t, err)
	locked := lockedUntil.UTC().Format(time.RFC3339)
	require.NoError(t, model.StoreSupplierBatchSchedulerCommandState(context.Background(), db, claim, types.SupplierBatchCommandStateV1{
		SchemaVersion: types.SupplierBatchCommandSchemaVersion, State: types.SupplierBatchCommandStateRunning,
		Response: &types.SupplierBatchCommandStatusV1{
			RequestID: requestID, BatchDate: stringPointer(batchDate), RunID: int64Pointer(runID), Status: types.SupplierBatchCommandStateRunning,
			FenceToken: fence, LockedUntil: &locked, ErrorCategory: types.SupplierBatchErrorNone,
		},
	}))
}

func supplierBatchProtocolDBs(t *testing.T) (*gorm.DB, *gorm.DB) {
	t.Helper()
	mainDB, logDB := supplierDailyTestDBs(t)
	require.NoError(t, mainDB.AutoMigrate(&model.SupplierAdminCommand{}))
	require.NoError(t, model.MigrateSupplierAdminCommandLedger(mainDB))
	return mainDB, logDB
}

func seedIncompleteSupplierDailyPublication(t *testing.T, mainDB, logDB *gorm.DB, now time.Time) (time.Time, *model.SupplierUsageDailyBatchRun, model.Log) {
	t.Helper()
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := beginningOfSupplierDay(now.In(location)).AddDate(0, 0, -1)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())
	logEntry := model.Log{Type: model.LogTypeConsume, CreatedAt: day.Add(time.Hour).Unix(), Other: `{}`}
	require.NoError(t, logDB.Create(&logEntry).Error)
	lease, err := model.AcquireSupplierDailyBatch(
		context.Background(), mainDB, day.Format("2006-01-02"), day.Unix(), day.AddDate(0, 0, 1).Unix(),
		"seed-incomplete", now, supplierDailyLeaseDuration, false,
	)
	require.NoError(t, err)
	require.NoError(t, model.PersistSupplierDailyBatchPage(
		context.Background(), mainDB, lease, nil, logEntry.CreatedAt, logEntry.Id, 1, 0, supplierDailyLeaseDuration,
	))
	evidence := types.SupplierPublishedEvidenceV1{
		SchemaVersion:                    types.SupplierPublishedEvidenceSchemaVersion,
		LogsScanned:                      1,
		FailureCounts:                    types.SupplierPublishedFailureCountsV1{AbsentMarkerAfterCutover: 1},
		PersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessIncomplete,
		Warnings: []types.SupplierPublishedWarningV1{{
			Code: types.SupplierPublishedWarningAbsentMarker, Count: 1, MessageKey: "supply_chain.warning.absent_marker_after_cutover",
		}},
	}
	require.NoError(t, model.PublishSupplierDailyBatch(context.Background(), mainDB, lease, time.Now(), evidence))
	run, publishedEvidence, err := model.LoadSupplierPublishedDailyBatch(context.Background(), mainDB, day.Format("2006-01-02"))
	require.NoError(t, err)
	require.Equal(t, types.SupplierPersistedLogCompletenessIncomplete, publishedEvidence.PersistedLogSnapshotCompleteness)
	return day, run, logEntry
}

func staleSupplierBatchRequestNow(t *testing.T) time.Time {
	t.Helper()
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	actual := time.Now().In(location)
	return time.Date(actual.Year(), actual.Month(), actual.Day()-2, 12, 0, 0, 0, location)
}

func requireSupplierPublicationTimestampInWindow(t *testing.T, run model.SupplierUsageDailyBatchRun, windowStart, windowEnd int64) {
	t.Helper()
	require.NotNil(t, run.CompletedAt)
	require.NotNil(t, run.PublishedAt)
	require.Equal(t, *run.CompletedAt, *run.PublishedAt)
	require.GreaterOrEqual(t, *run.CompletedAt, windowStart)
	require.LessOrEqual(t, *run.CompletedAt, windowEnd)
}

func TestSupplierBatchPublicationUsesCompletionClockInsteadOfStaleRequestTime(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	requestNow := staleSupplierBatchRequestNow(t)
	day := beginningOfSupplierDay(requestNow).AddDate(0, 0, -1)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())
	valid := supplierDailySnapshot(day, 700_000)
	require.NoError(t, logDB.Create(&model.Log{
		Type: model.LogTypeConsume, CreatedAt: day.Add(time.Hour).Unix(), ChannelId: 7, ModelName: "completion-clock",
		Other: supplierDailyLogOther(t, valid),
	}).Error)
	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "completion-clock-job", AuditSlot: dto.SupplierBatchAuditSlotCurrent}

	publicationStartedAt := time.Now().Unix()
	response, err := CatchUpSupplierDailyBatchesByRequest(context.Background(), mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "completion-clock"}, requestNow)
	require.NoError(t, err)
	require.Equal(t, dto.SupplierBatchStatusCompleted, response.Status)
	publicationFinishedAt := time.Now().Unix()

	var run model.SupplierUsageDailyBatchRun
	require.NoError(t, mainDB.Where("id = ?", *response.RunID).First(&run).Error)
	requireSupplierPublicationTimestampInWindow(t, run, publicationStartedAt, publicationFinishedAt)
	require.Greater(t, *run.CompletedAt, requestNow.Unix(), "request entry time can be stale while the log scan is running")
}

func TestSupplierDailyReportRerunPublicationUsesCompletionClockAndReplayKeepsTerminal(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	requestNow := staleSupplierBatchRequestNow(t)
	day := beginningOfSupplierDay(requestNow).AddDate(0, 0, -1)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())
	require.NoError(t, logDB.Create(&model.Log{Type: model.LogTypeConsume, CreatedAt: day.Add(time.Hour).Unix(), Other: `{}`}).Error)
	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "rerun-completion-clock-job", AuditSlot: dto.SupplierBatchAuditSlotCurrent}
	published, err := CatchUpSupplierDailyBatchesByRequest(context.Background(), mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "publish-for-rerun-clock"}, requestNow)
	require.NoError(t, err)
	oldRun, _, err := model.LoadSupplierPublishedDailyBatch(context.Background(), mainDB, *published.BatchDate)
	require.NoError(t, err)

	rerunRequest := dto.SupplierDailyReportRerunRequest{Reason: "verify completion timestamp", ExpectedPublishedFenceToken: oldRun.PublishedFenceToken}
	publicationStartedAt := time.Now().Unix()
	first, err := RerunSupplierDailyReport(context.Background(), mainDB, logDB, 9, oldRun.BatchDate, "rerun-completion-clock", rerunRequest, requestNow.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, dto.SupplierBatchStatusCompleted, first.Status)
	publicationFinishedAt := time.Now().Unix()

	newRun, _, err := model.LoadSupplierPublishedDailyBatch(context.Background(), mainDB, oldRun.BatchDate)
	require.NoError(t, err)
	requireSupplierPublicationTimestampInWindow(t, *newRun, publicationStartedAt, publicationFinishedAt)
	require.Greater(t, *newRun.CompletedAt, requestNow.Unix(), "rerun request entry time must not become the publication timestamp")

	replayed, err := RerunSupplierDailyReport(context.Background(), mainDB, logDB, 9, oldRun.BatchDate, "rerun-completion-clock", rerunRequest, requestNow.Add(10*time.Minute))
	require.NoError(t, err)
	require.Equal(t, first, replayed)
	replayedRun, _, err := model.LoadSupplierPublishedDailyBatch(context.Background(), mainDB, oldRun.BatchDate)
	require.NoError(t, err)
	require.Equal(t, newRun.CompletedAt, replayedRun.CompletedAt)
	require.Equal(t, newRun.PublishedAt, replayedRun.PublishedAt)
	require.Equal(t, newRun.PublishedFenceToken, replayedRun.PublishedFenceToken)
}

func TestSupplierBatchSchedulerPersistsExactNoWorkAndReplaysAcrossSlots(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, location)
	activateSupplierAccountingForBatch(t, mainDB, beginningOfSupplierDay(now).Unix())
	request := dto.SupplierBatchCatchUpRequest{RequestID: "no-work-key"}
	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "supplier-job", AuditSlot: dto.SupplierBatchAuditSlotCurrent}

	first, err := CatchUpSupplierDailyBatchesByRequest(context.Background(), mainDB, logDB, principal, request, now)
	require.NoError(t, err)
	require.NoError(t, first.Validate())
	require.Equal(t, dto.SupplierBatchStatusCompleted, first.Status)
	require.Nil(t, first.BatchDate)
	require.Nil(t, first.RunID)
	require.Zero(t, first.FenceToken)
	require.Zero(t, first.PublishedFenceToken)
	require.Equal(t, &dto.SupplierBatchStatusResult{ProcessedDays: 0, RemainingWork: false}, first.Result)

	var runCount int64
	require.NoError(t, mainDB.Model(&model.SupplierUsageDailyBatchRun{}).Count(&runCount).Error)
	require.Zero(t, runCount, "no-work must not create a date-keyed batch row")
	principal.AuditSlot = dto.SupplierBatchAuditSlotNext
	replayed, err := CatchUpSupplierDailyBatchesByRequest(context.Background(), mainDB, logDB, principal, request, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, first, replayed)
	status, err := GetSupplierDailyBatchRequestStatus(context.Background(), mainDB, principal, request.RequestID, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, first, status)
}

func TestSupplierBatchConfigurationStateMapsToUnavailable(t *testing.T) {
	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "configuration-state-job", AuditSlot: dto.SupplierBatchAuditSlotCurrent}
	now := time.Now()

	t.Run("missing activation", func(t *testing.T) {
		mainDB, logDB := supplierBatchProtocolDBs(t)
		_, err := CatchUpSupplierDailyBatchesByRequest(
			context.Background(), mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "missing-activation"}, now,
		)
		require.ErrorIs(t, err, ErrSupplierBatchConfigUnavailable)
	})

	t.Run("non-active phase", func(t *testing.T) {
		mainDB, logDB := supplierBatchProtocolDBs(t)
		location, err := time.LoadLocation(SupplierDailyBatchTimezone)
		require.NoError(t, err)
		armSupplierAccountingForBatch(t, mainDB, beginningOfSupplierDay(now.In(location)).AddDate(0, 0, -1).Unix())
		_, err = CatchUpSupplierDailyBatchesByRequest(
			context.Background(), mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "non-active-phase"}, now,
		)
		require.ErrorIs(t, err, ErrSupplierBatchConfigUnavailable)
	})

	t.Run("database failure remains internal", func(t *testing.T) {
		mainDB, logDB := supplierBatchProtocolDBs(t)
		require.NoError(t, mainDB.Migrator().DropTable(&model.Option{}))
		_, err := CatchUpSupplierDailyBatchesByRequest(
			context.Background(), mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "database-failure"}, now,
		)
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrSupplierBatchConfigUnavailable)
	})
}

func TestSupplierBatchSchedulerClaimAndNoWorkAreExternallyAtomic(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, location)
	activateSupplierAccountingForBatch(t, mainDB, beginningOfSupplierDay(now).Unix())
	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "atomic-no-work-job", AuditSlot: dto.SupplierBatchAuditSlotCurrent}
	request := dto.SupplierBatchCatchUpRequest{RequestID: "atomic-no-work"}

	claimReached := make(chan struct{})
	releaseClaim := make(chan struct{})
	var once sync.Once
	callbackName := fmt.Sprintf("test:block_after_supplier_claim:%s", t.Name())
	require.NoError(t, mainDB.Callback().Create().After("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "supplier_admin_commands" {
			once.Do(func() { close(claimReached) })
			<-releaseClaim
		}
	}))
	t.Cleanup(func() { _ = mainDB.Callback().Create().Remove(callbackName) })

	postResult := make(chan supplierBatchAsyncResult, 1)
	go func() {
		response, runErr := CatchUpSupplierDailyBatchesByRequest(context.Background(), mainDB, logDB, principal, request, now)
		postResult <- supplierBatchAsyncResult{response: response, err: runErr}
	}()
	require.Eventually(t, func() bool {
		select {
		case <-claimReached:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)

	getResult := make(chan supplierBatchAsyncResult, 1)
	go func() {
		response, getErr := getSupplierDailyBatchRequestStatus(context.Background(), mainDB, logDB, principal, request.RequestID, now)
		getResult <- supplierBatchAsyncResult{response: response, err: getErr}
	}()
	select {
	case result := <-getResult:
		require.Failf(t, "status escaped atomic preparation", "response=%+v err=%v", result.response, result.err)
	case <-time.After(25 * time.Millisecond):
	}
	close(releaseClaim)
	posted := <-postResult
	require.NoError(t, posted.err)
	require.NoError(t, posted.response.Validate())
	queried := <-getResult
	require.NoError(t, queried.err)
	require.NoError(t, queried.response.Validate())
	require.Equal(t, posted.response, queried.response)
}

func TestSupplierBatchStatusRecoversOrphanClaimWithoutDoubleExecution(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, location)
	day := beginningOfSupplierDay(now).AddDate(0, 0, -1)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())
	require.NoError(t, logDB.Create(&model.Log{Type: model.LogTypeConsume, CreatedAt: day.Add(time.Hour).Unix(), Other: `{}`}).Error)
	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "orphan-recovery-job", AuditSlot: dto.SupplierBatchAuditSlotCurrent}
	requestID := "orphan-claim"
	digest, err := model.DigestSupplierBatchTrustedIdentity(principal.TrustedJobIdentity)
	require.NoError(t, err)
	claim, err := model.ClaimSupplierBatchSchedulerCommand(context.Background(), mainDB, digest, requestID, supplierBatchCatchUpSemanticsV1{SchemaVersion: 1, Operation: "catch_up"}, principal.AuditSlot)
	require.NoError(t, err)
	require.True(t, claim.Claimed)
	require.NoError(t, mainDB.Model(&model.SupplierAdminCommand{}).Where("id = ?", claim.Command.Id).UpdateColumn("updated_at", now.Add(-10*time.Minute).Unix()).Error)

	scanReached := make(chan struct{})
	releaseScan := make(chan struct{})
	var once sync.Once
	callbackName := fmt.Sprintf("test:block_orphan_recovery_scan:%s", t.Name())
	blockScan := func(tx *gorm.DB) {
		once.Do(func() { close(scanReached) })
		<-releaseScan
	}
	require.NoError(t, logDB.Callback().Query().Before("gorm:query").Register(callbackName+":query", blockScan))
	require.NoError(t, logDB.Callback().Row().Before("gorm:row").Register(callbackName+":row", blockScan))
	require.NoError(t, logDB.Callback().Raw().Before("gorm:raw").Register(callbackName+":raw", blockScan))
	t.Cleanup(func() {
		_ = logDB.Callback().Query().Remove(callbackName + ":query")
		_ = logDB.Callback().Row().Remove(callbackName + ":row")
		_ = logDB.Callback().Raw().Remove(callbackName + ":raw")
	})

	ownerResult := make(chan supplierBatchAsyncResult, 1)
	go func() {
		response, getErr := getSupplierDailyBatchRequestStatus(context.Background(), mainDB, logDB, principal, requestID, now)
		ownerResult <- supplierBatchAsyncResult{response: response, err: getErr}
	}()
	select {
	case <-scanReached:
	case early := <-ownerResult:
		require.Failf(t, "orphan recovery ended before scanning", "response=%+v err=%v", early.response, early.err)
	case <-time.After(time.Second):
		require.Fail(t, "orphan recovery did not reach log scan")
	}

	follower, err := getSupplierDailyBatchRequestStatus(context.Background(), mainDB, logDB, principal, requestID, now)
	require.NoError(t, err)
	require.NoError(t, follower.Validate())
	require.Equal(t, dto.SupplierBatchStatusRunning, follower.Status)
	close(releaseScan)
	owner := <-ownerResult
	require.NoError(t, owner.err)
	require.NoError(t, owner.response.Validate())
	require.Equal(t, dto.SupplierBatchStatusCompleted, owner.response.Status)

	var runCount int64
	require.NoError(t, mainDB.Model(&model.SupplierUsageDailyBatchRun{}).Count(&runCount).Error)
	require.EqualValues(t, 1, runCount, "orphan recovery and concurrent status reads must share one batch execution")
	terminal, err := getSupplierDailyBatchRequestStatus(context.Background(), mainDB, logDB, principal, requestID, now)
	require.NoError(t, err)
	require.Equal(t, owner.response, terminal)
}

func TestSupplierBatchStatusRecoveryCancellationFailsWithoutPublicationOrStrandedLease(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, location)
	day := beginningOfSupplierDay(now).AddDate(0, 0, -1)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())
	require.NoError(t, logDB.Create(&model.Log{Type: model.LogTypeConsume, CreatedAt: day.Add(time.Hour).Unix(), Other: `{}`}).Error)
	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "cancelled-recovery-job", AuditSlot: dto.SupplierBatchAuditSlotCurrent}
	requestID := "cancelled-orphan-claim"
	digest, err := model.DigestSupplierBatchTrustedIdentity(principal.TrustedJobIdentity)
	require.NoError(t, err)
	_, err = model.ClaimSupplierBatchSchedulerCommand(context.Background(), mainDB, digest, requestID, supplierBatchCatchUpSemanticsV1{SchemaVersion: 1, Operation: "catch_up"}, principal.AuditSlot)
	require.NoError(t, err)

	scanReached := make(chan struct{})
	var once sync.Once
	callbackName := fmt.Sprintf("test:cancel_orphan_recovery_scan:%s", t.Name())
	cancelScan := func(tx *gorm.DB) {
		once.Do(func() { close(scanReached) })
		<-tx.Statement.Context.Done()
		tx.AddError(tx.Statement.Context.Err())
	}
	require.NoError(t, logDB.Callback().Query().Before("gorm:query").Register(callbackName+":query", cancelScan))
	require.NoError(t, logDB.Callback().Row().Before("gorm:row").Register(callbackName+":row", cancelScan))
	require.NoError(t, logDB.Callback().Raw().Before("gorm:raw").Register(callbackName+":raw", cancelScan))
	t.Cleanup(func() {
		_ = logDB.Callback().Query().Remove(callbackName + ":query")
		_ = logDB.Callback().Row().Remove(callbackName + ":row")
		_ = logDB.Callback().Raw().Remove(callbackName + ":raw")
	})

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan supplierBatchAsyncResult, 1)
	go func() {
		response, getErr := getSupplierDailyBatchRequestStatus(ctx, mainDB, logDB, principal, requestID, now)
		result <- supplierBatchAsyncResult{response: response, err: getErr}
	}()
	select {
	case <-scanReached:
	case early := <-result:
		require.Failf(t, "cancel recovery ended before scanning", "response=%+v err=%v", early.response, early.err)
	case <-time.After(time.Second):
		require.Fail(t, "cancel recovery did not reach log scan")
	}
	cancel()
	recovered := <-result
	require.NoError(t, recovered.err)
	require.NoError(t, recovered.response.Validate())
	require.Equal(t, dto.SupplierBatchStatusFailed, recovered.response.Status)
	require.Equal(t, dto.SupplierBatchErrorReadFailed, recovered.response.ErrorCategory)

	var run model.SupplierUsageDailyBatchRun
	require.NoError(t, mainDB.Where("batch_date = ?", day.Format("2006-01-02")).First(&run).Error)
	require.Equal(t, model.SupplierDailyBatchStatusFailed, run.Status)
	require.Zero(t, run.PublishedFenceToken)
	require.Nil(t, run.ActiveLeaseSlot)

	require.NoError(t, logDB.Callback().Query().Remove(callbackName+":query"))
	require.NoError(t, logDB.Callback().Row().Remove(callbackName+":row"))
	require.NoError(t, logDB.Callback().Raw().Remove(callbackName+":raw"))
	later, err := CatchUpSupplierDailyBatchesByRequest(context.Background(), mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "later-after-cancel"}, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, dto.SupplierBatchStatusCompleted, later.Status)
	require.Greater(t, later.PublishedFenceToken, recovered.response.FenceToken)
	replayed, err := getSupplierDailyBatchRequestStatus(context.Background(), mainDB, logDB, principal, requestID, now.Add(2*time.Minute))
	require.NoError(t, err)
	require.Equal(t, recovered.response, replayed, "a later fence must not rewrite the canceled request's stored terminal response")
}

func TestSupplierBatchCancellationDuringTerminalStoreStillCommitsExactFailure(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, location)
	day := beginningOfSupplierDay(now).AddDate(0, 0, -1)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())
	require.NoError(t, logDB.Create(&model.Log{Type: model.LogTypeConsume, CreatedAt: day.Add(time.Hour).Unix(), Other: `{}`}).Error)
	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "terminal-cancel-job", AuditSlot: dto.SupplierBatchAuditSlotCurrent}
	requestID := "cancel-during-terminal-store"

	scanReached := make(chan struct{})
	var scanOnce sync.Once
	scanCallback := fmt.Sprintf("test:cancel_terminal_scan:%s", t.Name())
	cancelScan := func(tx *gorm.DB) {
		scanOnce.Do(func() { close(scanReached) })
		<-tx.Statement.Context.Done()
		tx.AddError(tx.Statement.Context.Err())
	}
	require.NoError(t, logDB.Callback().Query().Before("gorm:query").Register(scanCallback+":query", cancelScan))
	require.NoError(t, logDB.Callback().Row().Before("gorm:row").Register(scanCallback+":row", cancelScan))
	require.NoError(t, logDB.Callback().Raw().Before("gorm:raw").Register(scanCallback+":raw", cancelScan))
	t.Cleanup(func() {
		_ = logDB.Callback().Query().Remove(scanCallback + ":query")
		_ = logDB.Callback().Row().Remove(scanCallback + ":row")
		_ = logDB.Callback().Raw().Remove(scanCallback + ":raw")
	})

	terminalStoreReached := make(chan struct{})
	releaseTerminalStore := make(chan struct{})
	var terminalOnce sync.Once
	terminalCallback := fmt.Sprintf("test:block_terminal_store:%s", t.Name())
	require.NoError(t, mainDB.Callback().Update().Before("gorm:update").Register(terminalCallback, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "supplier_admin_commands" {
			return
		}
		updates, ok := tx.Statement.Dest.(map[string]any)
		if !ok {
			return
		}
		statusJSON, ok := updates["status_json"].(string)
		if !ok {
			return
		}
		state, parseErr := types.ParseSupplierBatchCommandStateV1(statusJSON)
		if parseErr != nil || state.State != types.SupplierBatchCommandStateFailed {
			return
		}
		terminalOnce.Do(func() { close(terminalStoreReached) })
		<-releaseTerminalStore
	}))
	t.Cleanup(func() { _ = mainDB.Callback().Update().Remove(terminalCallback) })

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan supplierBatchAsyncResult, 1)
	go func() {
		response, runErr := CatchUpSupplierDailyBatchesByRequest(ctx, mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: requestID}, now)
		result <- supplierBatchAsyncResult{response: response, err: runErr}
	}()
	select {
	case <-scanReached:
	case early := <-result:
		require.Failf(t, "request ended before scanning", "response=%+v err=%v", early.response, early.err)
	case <-time.After(time.Second):
		require.Fail(t, "request did not reach log scan")
	}
	cancel()
	select {
	case <-terminalStoreReached:
	case early := <-result:
		require.Failf(t, "request ended before terminal store", "response=%+v err=%v", early.response, early.err)
	case <-time.After(time.Second):
		require.Fail(t, "request did not reach terminal store")
	}
	close(releaseTerminalStore)
	recovered := <-result
	require.NoError(t, recovered.err)
	require.NoError(t, recovered.response.Validate())
	require.Equal(t, dto.SupplierBatchStatusFailed, recovered.response.Status)

	require.NoError(t, logDB.Callback().Query().Remove(scanCallback+":query"))
	require.NoError(t, logDB.Callback().Row().Remove(scanCallback+":row"))
	require.NoError(t, logDB.Callback().Raw().Remove(scanCallback+":raw"))
	require.NoError(t, mainDB.Callback().Update().Remove(terminalCallback))
	later, err := CatchUpSupplierDailyBatchesByRequest(context.Background(), mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "later-after-terminal-cancel"}, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, dto.SupplierBatchStatusCompleted, later.Status)
	replayed, err := getSupplierDailyBatchRequestStatus(context.Background(), mainDB, logDB, principal, requestID, now.Add(2*time.Minute))
	require.NoError(t, err)
	require.Equal(t, recovered.response, replayed)
}

func TestSupplierBatchTerminalStoreFailureRollsBackPublicationAndReconcilesFailure(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, location)
	day := beginningOfSupplierDay(now).AddDate(0, 0, -1)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())
	valid := supplierDailySnapshot(day, 700_000)
	require.NoError(t, logDB.Create(&model.Log{Type: model.LogTypeConsume, CreatedAt: day.Add(time.Hour).Unix(), ChannelId: 7, ModelName: "atomic-terminal", Other: supplierDailyLogOther(t, valid)}).Error)
	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "terminal-store-failure-job", AuditSlot: dto.SupplierBatchAuditSlotCurrent}
	requestID := "terminal-store-failure"

	callbackName := fmt.Sprintf("test:fail_completed_terminal_once:%s", t.Name())
	var once sync.Once
	require.NoError(t, mainDB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "supplier_admin_commands" {
			return
		}
		updates, ok := tx.Statement.Dest.(map[string]any)
		if !ok {
			return
		}
		statusJSON, ok := updates["status_json"].(string)
		if !ok {
			return
		}
		state, parseErr := types.ParseSupplierBatchCommandStateV1(statusJSON)
		if parseErr == nil && state.State == types.SupplierBatchCommandStateCompleted {
			once.Do(func() { tx.AddError(errors.New("forced terminal command-store failure")) })
		}
	}))
	t.Cleanup(func() { _ = mainDB.Callback().Update().Remove(callbackName) })

	_, err = CatchUpSupplierDailyBatchesByRequest(context.Background(), mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: requestID}, now)
	require.ErrorContains(t, err, "forced terminal command-store failure")
	require.NoError(t, mainDB.Callback().Update().Remove(callbackName))
	var run model.SupplierUsageDailyBatchRun
	require.NoError(t, mainDB.Where("batch_date = ?", day.Format("2006-01-02")).First(&run).Error)
	require.Equal(t, model.SupplierDailyBatchStatusFailed, run.Status)
	require.Zero(t, run.PublishedFenceToken, "publication pointer must roll back with the failed terminal command store")
	require.Empty(t, run.PublishedEvidenceV1, "published evidence must roll back with the publication pointer")
	var candidateSummaries int64
	require.NoError(t, mainDB.Model(&model.SupplierUsageDailySummary{}).Where("batch_date = ? AND batch_fence_token = ?", run.BatchDate, run.FenceToken).Count(&candidateSummaries).Error)
	require.Zero(t, candidateSummaries, "failed-fence candidate summaries must be cleaned before terminalization")
	status, err := getSupplierDailyBatchRequestStatus(context.Background(), mainDB, logDB, principal, requestID, now)
	require.NoError(t, err)
	require.Equal(t, dto.SupplierBatchStatusFailed, status.Status)
	replayed, err := CatchUpSupplierDailyBatchesByRequest(context.Background(), mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: requestID}, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, status, replayed)
}

func TestSupplierBatchRunningCommandReconcilesPublishedRenewedAndFailedRows(t *testing.T) {
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, location)
	day := beginningOfSupplierDay(now).AddDate(0, 0, -1)
	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "reconcile-job", AuditSlot: dto.SupplierBatchAuditSlotCurrent}

	t.Run("published before terminal store", func(t *testing.T) {
		mainDB, logDB := supplierBatchProtocolDBs(t)
		activateSupplierAccountingForBatch(t, mainDB, day.Unix())
		run := supplierReportPublishedRun(t, day.Format("2006-01-02"), day.Unix(), day.AddDate(0, 0, 1).Unix(), 7, 0)
		require.NoError(t, mainDB.Create(&run).Error)
		seedSupplierSchedulerRunningCommand(t, mainDB, principal, "published-lost-response", run.BatchDate, int64(run.Id), 7, now.Add(-time.Minute))

		status, err := getSupplierDailyBatchRequestStatus(context.Background(), mainDB, logDB, principal, "published-lost-response", now)
		require.NoError(t, err)
		require.NoError(t, status.Validate())
		require.Equal(t, dto.SupplierBatchStatusCompleted, status.Status)
		require.Equal(t, run.BatchDate, *status.BatchDate)
		replayed, err := CatchUpSupplierDailyBatchesByRequest(context.Background(), mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "published-lost-response"}, now)
		require.NoError(t, err)
		require.Equal(t, status, replayed, "same-key replay must reconcile the published fence instead of processing another date")
	})

	t.Run("database lease renewal overrides stale ledger lease", func(t *testing.T) {
		mainDB, logDB := supplierBatchProtocolDBs(t)
		activateSupplierAccountingForBatch(t, mainDB, day.Unix())
		lockedUntil := time.Now().Add(20 * time.Minute).Unix()
		activeSlot := 1
		run := model.SupplierUsageDailyBatchRun{
			BatchDate: day.Format("2006-01-02"), DayStart: day.Unix(), DayEnd: day.AddDate(0, 0, 1).Unix(),
			Status: model.SupplierDailyBatchStatusRunning, LeaseOwner: "renewed-owner", FenceToken: 3, LockedUntil: lockedUntil, ActiveLeaseSlot: &activeSlot,
		}
		require.NoError(t, mainDB.Create(&run).Error)
		seedSupplierSchedulerRunningCommand(t, mainDB, principal, "renewed-running", run.BatchDate, int64(run.Id), 3, now.Add(-time.Minute))
		status, err := getSupplierDailyBatchRequestStatus(context.Background(), mainDB, logDB, principal, "renewed-running", now)
		require.NoError(t, err)
		require.NoError(t, status.Validate())
		require.Equal(t, dto.SupplierBatchStatusRunning, status.Status)
		require.Equal(t, time.Unix(lockedUntil, 0).UTC().Format(time.RFC3339), *status.LockedUntil)
	})

	t.Run("failed candidate is terminal retryable status", func(t *testing.T) {
		mainDB, logDB := supplierBatchProtocolDBs(t)
		activateSupplierAccountingForBatch(t, mainDB, day.Unix())
		run := model.SupplierUsageDailyBatchRun{
			BatchDate: day.Format("2006-01-02"), DayStart: day.Unix(), DayEnd: day.AddDate(0, 0, 1).Unix(),
			Status: model.SupplierDailyBatchStatusFailed, FenceToken: 4,
		}
		require.NoError(t, mainDB.Create(&run).Error)
		seedSupplierSchedulerRunningCommand(t, mainDB, principal, "failed-lost-response", run.BatchDate, int64(run.Id), 4, now.Add(-time.Minute))
		status, err := getSupplierDailyBatchRequestStatus(context.Background(), mainDB, logDB, principal, "failed-lost-response", now)
		require.NoError(t, err)
		require.NoError(t, status.Validate())
		require.Equal(t, dto.SupplierBatchStatusFailed, status.Status)
		require.Equal(t, dto.SupplierBatchErrorExecutionFailed, status.ErrorCategory)
		require.True(t, status.Result.RemainingWork)
	})
}

func TestSupplierBatchLostResponseUsesImmutablePublishedFenceBeforeNewCandidate(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, location)
	day := beginningOfSupplierDay(now).AddDate(0, 0, -1)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())
	run := supplierReportPublishedRun(t, day.Format("2006-01-02"), day.Unix(), day.AddDate(0, 0, 1).Unix(), 7, 0)
	run.FenceToken = 8
	run.Status = model.SupplierDailyBatchStatusRunning
	run.LeaseOwner = "rerun-candidate"
	run.LockedUntil = now.Add(10 * time.Minute).Unix()
	activeSlot := 1
	run.ActiveLeaseSlot = &activeSlot
	require.NoError(t, mainDB.Create(&run).Error)
	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "immutable-published-job", AuditSlot: dto.SupplierBatchAuditSlotCurrent}
	seedSupplierSchedulerRunningCommand(t, mainDB, principal, "published-before-new-candidate", run.BatchDate, int64(run.Id), 7, now.Add(-time.Minute))

	status, err := getSupplierDailyBatchRequestStatus(context.Background(), mainDB, logDB, principal, "published-before-new-candidate", now)
	require.NoError(t, err)
	require.NoError(t, status.Validate())
	require.Equal(t, dto.SupplierBatchStatusCompleted, status.Status)
	require.EqualValues(t, 7, status.PublishedFenceToken)
	replayed, err := CatchUpSupplierDailyBatchesByRequest(context.Background(), mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "published-before-new-candidate"}, now)
	require.NoError(t, err)
	require.Equal(t, status, replayed)
	var runCount int64
	require.NoError(t, mainDB.Model(&model.SupplierUsageDailyBatchRun{}).Count(&runCount).Error)
	require.EqualValues(t, 1, runCount, "same-key reconciliation must not select another date")
}

func TestSupplierBatchLostResponseFailsClosedForNewerPublishedFence(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, location)
	day := beginningOfSupplierDay(now).AddDate(0, 0, -1)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())
	run := supplierReportPublishedRun(t, day.Format("2006-01-02"), day.Unix(), day.AddDate(0, 0, 1).Unix(), 8, 0)
	require.NoError(t, mainDB.Create(&run).Error)
	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "newer-published-job", AuditSlot: dto.SupplierBatchAuditSlotCurrent}
	seedSupplierSchedulerRunningCommand(t, mainDB, principal, "newer-published-fence", run.BatchDate, int64(run.Id), 7, now.Add(-time.Minute))
	status, err := getSupplierDailyBatchRequestStatus(context.Background(), mainDB, logDB, principal, "newer-published-fence", now)
	require.NoError(t, err)
	require.NoError(t, status.Validate())
	require.Equal(t, dto.SupplierBatchStatusFailed, status.Status)
	require.Equal(t, dto.SupplierBatchErrorFenceLost, status.ErrorCategory)
	require.EqualValues(t, 8, status.PublishedFenceToken)
	require.False(t, status.Result.RemainingWork)
	require.Nil(t, status.Result.NextBatchDate, "a stale fence must not manufacture a retry hint when no work remains")
}

func TestSupplierBatchLostResponseUsesCurrentLaterBacklogHint(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, location)
	staleDay := beginningOfSupplierDay(now).AddDate(0, 0, -2)
	backlogDay := staleDay.AddDate(0, 0, 1)
	activateSupplierAccountingForBatch(t, mainDB, staleDay.Unix())
	staleRun := supplierReportPublishedRun(t, staleDay.Format("2006-01-02"), staleDay.Unix(), staleDay.AddDate(0, 0, 1).Unix(), 8, 0)
	backlogRun := model.SupplierUsageDailyBatchRun{
		BatchDate: backlogDay.Format("2006-01-02"), DayStart: backlogDay.Unix(), DayEnd: backlogDay.AddDate(0, 0, 1).Unix(),
		Status: model.SupplierDailyBatchStatusFailed, FenceToken: 1,
	}
	require.NoError(t, mainDB.Create(&staleRun).Error)
	require.NoError(t, mainDB.Create(&backlogRun).Error)
	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "later-backlog-job", AuditSlot: dto.SupplierBatchAuditSlotCurrent}
	seedSupplierSchedulerRunningCommand(t, mainDB, principal, "newer-fence-later-backlog", staleRun.BatchDate, int64(staleRun.Id), 7, now.Add(-time.Minute))

	status, err := getSupplierDailyBatchRequestStatus(context.Background(), mainDB, logDB, principal, "newer-fence-later-backlog", now)
	require.NoError(t, err)
	require.NoError(t, status.Validate())
	require.Equal(t, dto.SupplierBatchStatusFailed, status.Status)
	require.Equal(t, dto.SupplierBatchErrorFenceLost, status.ErrorCategory)
	require.True(t, status.Result.RemainingWork)
	require.Equal(t, backlogRun.BatchDate, *status.Result.NextBatchDate, "retry hint must point at the current oldest never-published date")
	require.NotEqual(t, staleRun.BatchDate, *status.Result.NextBatchDate)
}

func TestSupplierDailyReportInvalidEligibilityLeavesNoClaimResidue(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, location)
	day := beginningOfSupplierDay(now).AddDate(0, 0, -1)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())
	run := supplierReportPublishedRun(t, day.Format("2006-01-02"), day.Unix(), day.AddDate(0, 0, 1).Unix(), 5, 0)
	require.NoError(t, mainDB.Create(&run).Error)
	request := dto.SupplierDailyReportRerunRequest{Reason: "not eligible", ExpectedPublishedFenceToken: 5}

	for range 2 {
		_, err = RerunSupplierDailyReport(context.Background(), mainDB, logDB, 9, run.BatchDate, "invalid-eligibility", request, now)
		require.ErrorIs(t, err, ErrSupplierDailyReportNotEligible)
	}
	var commandCount int64
	require.NoError(t, mainDB.Model(&model.SupplierAdminCommand{}).Where("actor_id = ?", 9).Count(&commandCount).Error)
	require.Zero(t, commandCount, "eligibility failures must roll back before an actor command becomes observable")
}

func TestSupplierBatchSchedulerPublishesIncompleteThenAdvancesToLaterDate(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, location)
	firstDay := beginningOfSupplierDay(now).AddDate(0, 0, -2)
	activateSupplierAccountingForBatch(t, mainDB, firstDay.Unix())
	require.NoError(t, logDB.Create(&model.Log{Type: model.LogTypeConsume, CreatedAt: firstDay.Add(time.Hour).Unix(), Other: `{}`}).Error)
	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "supplier-job", AuditSlot: dto.SupplierBatchAuditSlotCurrent}

	first, err := CatchUpSupplierDailyBatchesByRequest(context.Background(), mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "day-one"}, now)
	require.NoError(t, err)
	require.Equal(t, firstDay.Format("2006-01-02"), *first.BatchDate)
	require.True(t, first.Result.RemainingWork)
	require.Equal(t, firstDay.AddDate(0, 0, 1).Format("2006-01-02"), *first.Result.NextBatchDate)
	firstRun, firstEvidence, err := model.LoadSupplierPublishedDailyBatch(context.Background(), mainDB, *first.BatchDate)
	require.NoError(t, err)
	require.Positive(t, firstRun.PublishedFenceToken)
	require.Equal(t, types.SupplierPersistedLogCompletenessIncomplete, firstEvidence.PersistedLogSnapshotCompleteness)

	second, err := CatchUpSupplierDailyBatchesByRequest(context.Background(), mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "day-two"}, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, firstDay.AddDate(0, 0, 1).Format("2006-01-02"), *second.BatchDate, "published incomplete date must remain outside scheduler catch-up")
	require.False(t, second.Result.RemainingWork)
}

func TestSupplierDailyReportRerunFailureRetainsAndSuccessReplacesPublishedView(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, location)
	day := beginningOfSupplierDay(now).AddDate(0, 0, -1)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())
	require.NoError(t, logDB.Create(&model.Log{Type: model.LogTypeConsume, CreatedAt: day.Add(time.Hour).Unix(), Other: `{}`}).Error)
	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "supplier-job", AuditSlot: dto.SupplierBatchAuditSlotCurrent}
	published, err := CatchUpSupplierDailyBatchesByRequest(context.Background(), mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "publish-incomplete"}, now)
	require.NoError(t, err)
	oldRun, oldEvidence, err := model.LoadSupplierPublishedDailyBatch(context.Background(), mainDB, *published.BatchDate)
	require.NoError(t, err)
	require.Equal(t, types.SupplierPersistedLogCompletenessIncomplete, oldEvidence.PersistedLogSnapshotCompleteness)

	require.NoError(t, logDB.Migrator().DropTable(&model.Log{}))
	failed, err := RerunSupplierDailyReport(context.Background(), mainDB, logDB, 9, day.Format("2006-01-02"), "failed-rerun", dto.SupplierDailyReportRerunRequest{
		Reason: "retry missing evidence", ExpectedPublishedFenceToken: oldRun.PublishedFenceToken,
	}, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, dto.SupplierBatchStatusFailed, failed.Status)
	retained, retainedEvidence, err := model.LoadSupplierPublishedDailyBatch(context.Background(), mainDB, day.Format("2006-01-02"))
	require.NoError(t, err)
	require.Equal(t, oldRun.PublishedFenceToken, retained.PublishedFenceToken)
	require.Equal(t, *oldRun.PublishedAt, *retained.PublishedAt)
	require.Equal(t, *oldEvidence, *retainedEvidence)

	require.NoError(t, logDB.AutoMigrate(&model.Log{}))
	valid := supplierDailySnapshot(day, 700_000)
	require.NoError(t, logDB.Create(&model.Log{Type: model.LogTypeConsume, CreatedAt: day.Add(time.Hour).Unix(), ChannelId: 4, ModelName: "rerun-model", Other: supplierDailyLogOther(t, valid)}).Error)
	succeeded, err := RerunSupplierDailyReport(context.Background(), mainDB, logDB, 9, day.Format("2006-01-02"), "successful-rerun", dto.SupplierDailyReportRerunRequest{
		Reason: "evidence restored", ExpectedPublishedFenceToken: oldRun.PublishedFenceToken,
	}, now.Add(2*time.Minute))
	require.NoError(t, err)
	require.Equal(t, dto.SupplierBatchStatusCompleted, succeeded.Status)
	require.Greater(t, succeeded.PublishedFenceToken, oldRun.PublishedFenceToken)
	newRun, newEvidence, err := model.LoadSupplierPublishedDailyBatch(context.Background(), mainDB, day.Format("2006-01-02"))
	require.NoError(t, err)
	require.Equal(t, types.SupplierPersistedLogCompletenessComplete, newEvidence.PersistedLogSnapshotCompleteness)
	require.EqualValues(t, 1, newEvidence.CapturedSnapshotCount)
	require.NotEqual(t, oldRun.PublishedFenceToken, newRun.PublishedFenceToken)

	replay, err := RerunSupplierDailyReport(context.Background(), mainDB, logDB, 9, day.Format("2006-01-02"), "successful-rerun", dto.SupplierDailyReportRerunRequest{
		Reason: "evidence restored", ExpectedPublishedFenceToken: oldRun.PublishedFenceToken,
	}, now.Add(3*time.Minute))
	require.NoError(t, err)
	require.Equal(t, succeeded, replay)
	_, err = RerunSupplierDailyReport(context.Background(), mainDB, logDB, 9, day.Format("2006-01-02"), "successful-rerun", dto.SupplierDailyReportRerunRequest{
		Reason: "changed payload", ExpectedPublishedFenceToken: oldRun.PublishedFenceToken,
	}, now.Add(3*time.Minute))
	require.ErrorIs(t, err, ErrSupplierBatchIdempotencyConflict)
}

func TestSupplierDailyReportRerunTransactionalClaimReplaysAfterStaleMiss(t *testing.T) {
	for _, winnerState := range []string{"running", "completed"} {
		t.Run(winnerState, func(t *testing.T) {
			mainDB, logDB := supplierBatchProtocolDBs(t)
			mainSQL, err := mainDB.DB()
			require.NoError(t, err)
			mainSQL.SetMaxOpenConns(4)
			now := time.Now()
			day, published, logEntry := seedIncompleteSupplierDailyPublication(t, mainDB, logDB, now)
			valid := supplierDailySnapshot(day, 700_000)
			require.NoError(t, logDB.Model(&model.Log{}).Where("id = ?", logEntry.Id).Update("other", supplierDailyLogOther(t, valid)).Error)

			const actorID = 19
			idempotencyKey := "stale-miss-" + winnerState
			request := dto.SupplierDailyReportRerunRequest{
				Reason: "transactional claim replay", ExpectedPublishedFenceToken: published.PublishedFenceToken,
			}
			missReached := make(chan struct{})
			releaseMiss := make(chan struct{})
			var releaseMissOnce sync.Once
			defer releaseMissOnce.Do(func() { close(releaseMiss) })
			var missMu sync.Mutex
			missAvailable := true
			missCallbackName := fmt.Sprintf("test:rerun_stale_miss:%s", t.Name())
			require.NoError(t, mainDB.Callback().Query().After("gorm:query").Register(missCallbackName, func(tx *gorm.DB) {
				if tx.Statement == nil || tx.Statement.Table != "supplier_admin_commands" || tx.RowsAffected != 0 {
					return
				}
				missMu.Lock()
				if !missAvailable {
					missMu.Unlock()
					return
				}
				missAvailable = false
				missMu.Unlock()
				close(missReached)
				<-releaseMiss
			}))
			defer func() { _ = mainDB.Callback().Query().Remove(missCallbackName) }()

			loserResult := make(chan supplierBatchAsyncResult, 1)
			go func() {
				response, runErr := RerunSupplierDailyReport(
					context.Background(), mainDB, logDB, actorID, day.Format("2006-01-02"), idempotencyKey, request, now,
				)
				loserResult <- supplierBatchAsyncResult{response: response, err: runErr}
			}()
			select {
			case <-missReached:
			case early := <-loserResult:
				require.Failf(t, "stale reader returned before its command miss was released", "response=%+v err=%v", early.response, early.err)
			case <-time.After(5 * time.Second):
				require.Fail(t, "stale command miss was not observed")
			}

			if winnerState == "completed" {
				winner, runErr := RerunSupplierDailyReport(
					context.Background(), mainDB, logDB, actorID, day.Format("2006-01-02"), idempotencyKey, request, now,
				)
				require.NoError(t, runErr)
				require.Equal(t, dto.SupplierBatchStatusCompleted, winner.Status)
				releaseMissOnce.Do(func() { close(releaseMiss) })
				loser := <-loserResult
				require.NoError(t, loser.err)
				require.Equal(t, winner, loser.response, "stale miss must exact-replay the terminal command before checking the newer published fence")
				return
			}

			scanReached := make(chan struct{})
			releaseScan := make(chan struct{})
			var releaseScanOnce sync.Once
			defer releaseScanOnce.Do(func() { close(releaseScan) })
			var scanMu sync.Mutex
			scanAvailable := true
			scanCallbackName := fmt.Sprintf("test:rerun_stale_miss_scan:%s", t.Name())
			blockScan := func(tx *gorm.DB) {
				if tx.Statement == nil || tx.Statement.Table != "logs" {
					return
				}
				scanMu.Lock()
				if !scanAvailable {
					scanMu.Unlock()
					return
				}
				scanAvailable = false
				scanMu.Unlock()
				close(scanReached)
				<-releaseScan
			}
			require.NoError(t, logDB.Callback().Query().Before("gorm:query").Register(scanCallbackName+":query", blockScan))
			require.NoError(t, logDB.Callback().Row().Before("gorm:row").Register(scanCallbackName+":row", blockScan))
			require.NoError(t, logDB.Callback().Raw().Before("gorm:raw").Register(scanCallbackName+":raw", blockScan))
			defer func() {
				_ = logDB.Callback().Query().Remove(scanCallbackName + ":query")
				_ = logDB.Callback().Row().Remove(scanCallbackName + ":row")
				_ = logDB.Callback().Raw().Remove(scanCallbackName + ":raw")
			}()
			winnerResult := make(chan supplierBatchAsyncResult, 1)
			go func() {
				response, runErr := RerunSupplierDailyReport(
					context.Background(), mainDB, logDB, actorID, day.Format("2006-01-02"), idempotencyKey, request, now,
				)
				winnerResult <- supplierBatchAsyncResult{response: response, err: runErr}
			}()
			select {
			case <-scanReached:
			case early := <-winnerResult:
				require.Failf(t, "winner returned before reaching the scan barrier", "response=%+v err=%v", early.response, early.err)
			case <-time.After(5 * time.Second):
				require.Fail(t, "winner did not reach the scan barrier")
			}
			stored, err := model.GetSupplierDailyReportRerunCommand(context.Background(), mainDB, actorID, day.Format("2006-01-02"), idempotencyKey)
			require.NoError(t, err)
			storedRunning, err := supplierBatchCommandResponseToDTO(stored.State)
			require.NoError(t, err)
			require.Equal(t, dto.SupplierBatchStatusRunning, storedRunning.Status)

			releaseMissOnce.Do(func() { close(releaseMiss) })
			loser := <-loserResult
			require.NoError(t, loser.err)
			require.NotNil(t, storedRunning.LockedUntil)
			require.NotNil(t, loser.response.LockedUntil)
			storedExpiry, err := time.Parse(time.RFC3339, *storedRunning.LockedUntil)
			require.NoError(t, err)
			replayedExpiry, err := time.Parse(time.RFC3339, *loser.response.LockedUntil)
			require.NoError(t, err)
			require.True(t, storedExpiry.Equal(replayedExpiry))
			storedRunning.LockedUntil = nil
			loser.response.LockedUntil = nil
			require.Equal(t, storedRunning, loser.response, "stale miss must replay the in-flight command instead of attempting a second takeover")
			releaseScanOnce.Do(func() { close(releaseScan) })
			winner := <-winnerResult
			require.NoError(t, winner.err)
			require.Equal(t, dto.SupplierBatchStatusCompleted, winner.response.Status)
		})
	}
}
