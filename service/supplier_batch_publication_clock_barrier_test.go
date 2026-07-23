package service

import (
	"context"
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

func installSupplierLogScanBarrier(t *testing.T, logDB *gorm.DB) (<-chan struct{}, func()) {
	t.Helper()
	scanReached := make(chan struct{})
	releaseScan := make(chan struct{})
	var blockOnce sync.Once
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseScan) }) }
	callbackName := fmt.Sprintf("test:publication_clock_scan:%s", t.Name())
	blockScan := func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "logs" {
			return
		}
		blockOnce.Do(func() {
			close(scanReached)
			<-releaseScan
		})
	}
	require.NoError(t, logDB.Callback().Query().Before("gorm:query").Register(callbackName+":query", blockScan))
	require.NoError(t, logDB.Callback().Row().Before("gorm:row").Register(callbackName+":row", blockScan))
	require.NoError(t, logDB.Callback().Raw().Before("gorm:raw").Register(callbackName+":raw", blockScan))
	t.Cleanup(func() {
		release()
		_ = logDB.Callback().Query().Remove(callbackName + ":query")
		_ = logDB.Callback().Row().Remove(callbackName + ":row")
		_ = logDB.Callback().Raw().Remove(callbackName + ":raw")
	})
	return scanReached, release
}

func releaseSupplierLogScanAfterClockAdvances(t *testing.T, scanReached <-chan struct{}, releaseScan func()) int64 {
	t.Helper()
	select {
	case <-scanReached:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "supplier log scan did not reach the barrier")
	}
	reachedAt := time.Now().Unix()
	require.Eventually(t, func() bool { return time.Now().Unix() > reachedAt }, 2*time.Second, 5*time.Millisecond)
	releasedAt := time.Now().Unix()
	releaseScan()
	return releasedAt
}

func requireSupplierPublicationAfterScanRelease(t *testing.T, mainDB *gorm.DB, runID int64, releasedAt int64) {
	t.Helper()
	var run model.SupplierUsageDailyBatchRun
	require.NoError(t, mainDB.First(&run, runID).Error)
	require.NotNil(t, run.CompletedAt)
	require.NotNil(t, run.PublishedAt)
	require.Equal(t, *run.CompletedAt, *run.PublishedAt)
	require.GreaterOrEqual(t, *run.CompletedAt, releasedAt)
}

func TestSupplierBatchSchedulerPublishesAfterBlockedLogScanIsReleased(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	now := time.Now().In(location)
	day := beginningOfSupplierDay(now).AddDate(0, 0, -1)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())
	valid := supplierDailySnapshot(day, 700_000)
	require.NoError(t, logDB.Create(&model.Log{
		Type: model.LogTypeConsume, CreatedAt: day.Add(time.Hour).Unix(), ChannelId: 7, ModelName: "scheduler-clock-barrier",
		Other: supplierDailyLogOther(t, valid),
	}).Error)

	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "scheduler-clock-barrier-job", AuditSlot: dto.SupplierBatchAuditSlotCurrent}
	digest, err := model.DigestSupplierBatchTrustedIdentity(principal.TrustedJobIdentity)
	require.NoError(t, err)
	claim, err := model.ClaimSupplierBatchSchedulerCommand(context.Background(), mainDB, digest, "scheduler-clock-barrier", supplierBatchCatchUpSemanticsV1{SchemaVersion: 1, Operation: "catch_up"}, principal.AuditSlot)
	require.NoError(t, err)
	lease, err := model.AcquireSupplierDailyBatch(context.Background(), mainDB, day.Format("2006-01-02"), day.Unix(), day.AddDate(0, 0, 1).Unix(), "scheduler-clock-barrier", now, supplierDailyLeaseDuration, false)
	require.NoError(t, err)
	lockedUntil := now.Add(supplierDailyLeaseDuration).Truncate(time.Second).UTC().Format(time.RFC3339)
	require.NoError(t, model.StoreSupplierBatchSchedulerCommandState(context.Background(), mainDB, claim, types.SupplierBatchCommandStateV1{
		SchemaVersion: types.SupplierBatchCommandSchemaVersion,
		State:         types.SupplierBatchCommandStateRunning,
		Response: &types.SupplierBatchCommandStatusV1{
			RequestID: "scheduler-clock-barrier", BatchDate: stringPointer(lease.BatchDate), RunID: int64Pointer(lease.RunId),
			Status: types.SupplierBatchCommandStateRunning, FenceToken: lease.FenceToken, LockedUntil: &lockedUntil,
			ErrorCategory: types.SupplierBatchErrorNone,
		},
	}))
	prepared := &supplierBatchPreparedRequest{
		claim: claim, lease: lease, day: day, cutoverAt: day.Unix(),
		startDate: day.Format("2006-01-02"), targetDate: day.Format("2006-01-02"),
	}

	scanReached, releaseScan := installSupplierLogScanBarrier(t, logDB)
	result := make(chan supplierBatchAsyncResult, 1)
	go func() {
		response, runErr := executeSupplierBatchCatchUpRequest(context.Background(), mainDB, logDB, "scheduler-clock-barrier", now, prepared)
		result <- supplierBatchAsyncResult{response: response, err: runErr}
	}()
	releasedAt := releaseSupplierLogScanAfterClockAdvances(t, scanReached, releaseScan)
	completed := <-result
	require.NoError(t, completed.err)
	require.Equal(t, dto.SupplierBatchStatusCompleted, completed.response.Status)
	requireSupplierPublicationAfterScanRelease(t, mainDB, lease.RunId, releasedAt)
}

func TestSupplierDailyReportRerunPublishesAfterBlockedLogScanIsReleased(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	now := time.Now()
	day, published, _ := seedIncompleteSupplierDailyPublication(t, mainDB, logDB, now)
	request := dto.SupplierDailyReportRerunRequest{
		Reason:                      "prove publication clock follows scan completion",
		ExpectedPublishedFenceToken: published.PublishedFenceToken,
	}

	scanReached, releaseScan := installSupplierLogScanBarrier(t, logDB)
	result := make(chan supplierBatchAsyncResult, 1)
	go func() {
		response, runErr := RerunSupplierDailyReport(context.Background(), mainDB, logDB, 9, day.Format("2006-01-02"), "rerun-clock-barrier", request, now)
		result <- supplierBatchAsyncResult{response: response, err: runErr}
	}()
	releasedAt := releaseSupplierLogScanAfterClockAdvances(t, scanReached, releaseScan)
	completed := <-result
	require.NoError(t, completed.err)
	require.Equal(t, dto.SupplierBatchStatusCompleted, completed.response.Status)
	require.NotNil(t, completed.response.RunID)
	requireSupplierPublicationAfterScanRelease(t, mainDB, *completed.response.RunID, releasedAt)
}
