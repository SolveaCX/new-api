package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestSupplierBatchExpiredDatabaseLeaseIsDerivedAndSameRequestTakesOver(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	requestDay := time.Now().In(location).AddDate(0, 0, 1)
	now := time.Date(requestDay.Year(), requestDay.Month(), requestDay.Day(), 12, 0, 0, 0, location)
	day := beginningOfSupplierDay(now).AddDate(0, 0, -1)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())

	activeSlot := 1
	run := model.SupplierUsageDailyBatchRun{
		BatchDate: day.Format("2006-01-02"), DayStart: day.Unix(), DayEnd: day.AddDate(0, 0, 1).Unix(),
		Status: model.SupplierDailyBatchStatusRunning, LeaseOwner: "crashed-node", FenceToken: 1,
		LockedUntil: time.Now().Add(-time.Minute).Unix(), ActiveLeaseSlot: &activeSlot,
	}
	require.NoError(t, mainDB.Create(&run).Error)
	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "expired-lease-job", AuditSlot: dto.SupplierBatchAuditSlotCurrent}
	requestID := "expired-database-lease"
	seedSupplierSchedulerRunningCommand(t, mainDB, principal, requestID, run.BatchDate, run.Id, run.FenceToken, now.Add(20*time.Minute))

	status, err := getSupplierDailyBatchRequestStatus(context.Background(), mainDB, logDB, principal, requestID, now)
	require.NoError(t, err)
	require.NoError(t, status.Validate())
	require.Equal(t, dto.SupplierBatchStatusFailed, status.Status)
	require.Equal(t, dto.SupplierBatchErrorLeaseExpired, status.ErrorCategory)
	require.Nil(t, status.LockedUntil)
	require.True(t, status.Result.RemainingWork)

	digest, err := model.DigestSupplierBatchTrustedIdentity(principal.TrustedJobIdentity)
	require.NoError(t, err)
	derivedOnly, err := model.GetSupplierBatchSchedulerCommand(context.Background(), mainDB, digest, requestID)
	require.NoError(t, err)
	require.Equal(t, types.SupplierBatchCommandStateRunning, derivedOnly.State.State, "GET derives expiry without terminalizing the command ledger")

	replayed, err := CatchUpSupplierDailyBatchesByRequest(context.Background(), mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: requestID}, now)
	require.NoError(t, err)
	require.NoError(t, replayed.Validate())
	require.Equal(t, dto.SupplierBatchStatusCompleted, replayed.Status)
	require.Equal(t, run.BatchDate, *replayed.BatchDate)
	require.Greater(t, replayed.FenceToken, run.FenceToken)
	require.Equal(t, replayed.FenceToken, replayed.PublishedFenceToken)

	terminal, err := model.GetSupplierBatchSchedulerCommand(context.Background(), mainDB, digest, requestID)
	require.NoError(t, err)
	require.Equal(t, types.SupplierBatchCommandStateCompleted, terminal.State.State)
}

func TestSupplierDailyReportRerunExpiredDatabaseLeaseSameRequestTakesOver(t *testing.T) {
	mainDB, logDB := supplierBatchProtocolDBs(t)
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	require.NoError(t, err)
	requestDay := time.Now().In(location).AddDate(0, 0, 1)
	now := time.Date(requestDay.Year(), requestDay.Month(), requestDay.Day(), 12, 0, 0, 0, location)
	day := beginningOfSupplierDay(now).AddDate(0, 0, -1)
	activateSupplierAccountingForBatch(t, mainDB, day.Unix())
	require.NoError(t, logDB.Create(&model.Log{Type: model.LogTypeConsume, CreatedAt: day.Add(time.Hour).Unix(), Other: `{}`}).Error)

	principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: "rerun-expiry-bootstrap", AuditSlot: dto.SupplierBatchAuditSlotCurrent}
	published, err := CatchUpSupplierDailyBatchesByRequest(context.Background(), mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: "rerun-expiry-bootstrap"}, now)
	require.NoError(t, err)
	publishedRun, publishedEvidence, err := model.LoadSupplierPublishedDailyBatch(context.Background(), mainDB, *published.BatchDate)
	require.NoError(t, err)
	require.Equal(t, types.SupplierPersistedLogCompletenessIncomplete, publishedEvidence.PersistedLogSnapshotCompleteness)

	actorID := 9
	requestID := "rerun-expired-database-lease"
	request := dto.SupplierDailyReportRerunRequest{Reason: "recover expired rerun", ExpectedPublishedFenceToken: publishedRun.PublishedFenceToken}
	lease, err := model.AcquireSupplierDailyBatchRerun(
		context.Background(), mainDB, publishedRun.BatchDate, day.Unix(), day.AddDate(0, 0, 1).Unix(),
		supplierDailyReportRerunLeaseOwner(actorID, requestID), now, time.Minute, publishedRun.PublishedFenceToken,
	)
	require.NoError(t, err)
	claim, err := model.ClaimSupplierDailyReportRerunCommand(context.Background(), mainDB, actorID, publishedRun.BatchDate, requestID, request)
	require.NoError(t, err)
	ledgerLockedUntil := now.Add(20 * time.Minute).UTC().Format(time.RFC3339)
	require.NoError(t, model.StoreSupplierDailyReportRerunCommandState(context.Background(), mainDB, claim, types.SupplierBatchCommandStateV1{
		SchemaVersion: types.SupplierBatchCommandSchemaVersion, State: types.SupplierBatchCommandStateRunning,
		Response: &types.SupplierBatchCommandStatusV1{
			RequestID: requestID, BatchDate: stringPointer(publishedRun.BatchDate), RunID: int64Pointer(lease.RunId), Status: types.SupplierBatchCommandStateRunning,
			FenceToken: lease.FenceToken, PublishedFenceToken: publishedRun.PublishedFenceToken, LockedUntil: &ledgerLockedUntil, ErrorCategory: types.SupplierBatchErrorNone,
		},
	}))
	require.NoError(t, mainDB.Model(&model.SupplierUsageDailyBatchRun{}).Where("id = ?", lease.RunId).UpdateColumn("locked_until", time.Now().Add(-time.Minute).Unix()).Error)

	replayed, err := RerunSupplierDailyReport(context.Background(), mainDB, logDB, actorID, publishedRun.BatchDate, requestID, request, now)
	require.NoError(t, err)
	require.NoError(t, replayed.Validate())
	require.Equal(t, dto.SupplierBatchStatusCompleted, replayed.Status)
	require.Greater(t, replayed.PublishedFenceToken, publishedRun.PublishedFenceToken)
	require.Equal(t, replayed.FenceToken, replayed.PublishedFenceToken)

	terminal, err := model.GetSupplierDailyReportRerunCommand(context.Background(), mainDB, actorID, publishedRun.BatchDate, requestID)
	require.NoError(t, err)
	require.Equal(t, types.SupplierBatchCommandStateCompleted, terminal.State.State)
}
