package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"gorm.io/gorm"
)

var (
	ErrSupplierBatchRequestNotFound       = errors.New("supplier batch request not found")
	ErrSupplierBatchBusy                  = errors.New("supplier batch busy")
	ErrSupplierBatchIdempotencyConflict   = errors.New("supplier batch idempotency conflict")
	ErrSupplierBatchConfigUnavailable     = errors.New("supplier batch configuration unavailable")
	ErrSupplierDailyReportNotFound        = errors.New("supplier daily report not found")
	ErrSupplierDailyReportInvalid         = errors.New("supplier daily report invalid")
	ErrSupplierDailyReportNotEligible     = errors.New("supplier daily report not eligible")
	ErrSupplierDailyReportVersionConflict = errors.New("supplier daily report version conflict")
)

type supplierBatchCatchUpSemanticsV1 struct {
	SchemaVersion int    `json:"schema_version"`
	Operation     string `json:"operation"`
}

const supplierBatchTerminalRecoveryTimeout = 5 * time.Second

func CatchUpSupplierDailyBatchesByRequest(ctx context.Context, mainDB, logDB *gorm.DB, principal dto.SupplierBatchSchedulerPrincipal, request dto.SupplierBatchCatchUpRequest, now time.Time) (dto.SupplierBatchStatusResponse, error) {
	identityDigest, err := validateSupplierBatchRequest(principal, request.RequestID)
	if err != nil {
		return dto.SupplierBatchStatusResponse{}, err
	}
	prepared, err := prepareSupplierBatchCatchUpRequest(ctx, mainDB, identityDigest, principal.AuditSlot, request.RequestID, now)
	if err != nil {
		return dto.SupplierBatchStatusResponse{}, err
	}
	if prepared.response != nil {
		return *prepared.response, nil
	}
	response, runErr := executeSupplierBatchCatchUpRequest(ctx, mainDB, logDB, request.RequestID, now, prepared)
	if runErr != nil {
		return response, runErr
	}
	return response, nil
}

type supplierBatchPreparedRequest struct {
	claim      *model.SupplierBatchSchedulerCommandClaim
	lease      model.SupplierDailyBatchLease
	day        time.Time
	cutoverAt  int64
	startDate  string
	targetDate string
	response   *dto.SupplierBatchStatusResponse
}

func prepareSupplierBatchCatchUpRequest(ctx context.Context, mainDB *gorm.DB, identityDigest []byte, auditSlot, requestID string, now time.Time) (*supplierBatchPreparedRequest, error) {
	prepared := &supplierBatchPreparedRequest{}
	err := mainDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		claim, err := model.ClaimSupplierBatchSchedulerCommand(ctx, tx, identityDigest, requestID, supplierBatchCatchUpSemanticsV1{SchemaVersion: 1, Operation: "catch_up"}, auditSlot)
		if err != nil {
			return mapSupplierBatchCommandError(err)
		}
		if claim.Replayed {
			switch claim.State.State {
			case types.SupplierBatchCommandStateClaimed:
				claim, err = model.AdoptSupplierBatchSchedulerClaim(ctx, tx, claim, now)
			case types.SupplierBatchCommandStateRunning:
				response, convertErr := reconcileSupplierBatchRunningCommand(ctx, tx, claim, now)
				if convertErr != nil {
					return convertErr
				}
				if response.ErrorCategory != dto.SupplierBatchErrorLeaseExpired && (response.Status != dto.SupplierBatchStatusRunning || !supplierBatchStatusResponseExpired(response, now)) {
					prepared.response = &response
					return nil
				}
				claim, err = model.TakeoverSupplierBatchSchedulerCommand(ctx, tx, claim, now)
			default:
				response, convertErr := supplierBatchCommandResponseToDTO(claim.State)
				if convertErr != nil {
					return convertErr
				}
				prepared.response = &response
				return nil
			}
			if err != nil {
				return mapSupplierBatchCommandError(err)
			}
		}
		prepared.claim = claim

		location, err := time.LoadLocation(SupplierDailyBatchTimezone)
		if err != nil {
			return err
		}
		today := beginningOfSupplierDay(now.In(location))
		target := today.AddDate(0, 0, -1)
		cutoverAt, err := supplierBatchConfiguredCutover(ctx, tx)
		if err != nil {
			return err
		}
		start := beginningOfSupplierDay(time.Unix(cutoverAt, 0).In(location))
		prepared.cutoverAt = cutoverAt
		prepared.startDate = start.Format("2006-01-02")
		prepared.targetDate = target.Format("2006-01-02")
		if now.In(location).Before(today.Add(SupplierDailyCloseGrace)) || start.After(target) {
			response, completeErr := completeSupplierBatchNoWork(ctx, tx, claim, requestID)
			if completeErr != nil {
				return completeErr
			}
			prepared.response = &response
			return nil
		}
		if err := model.EnsureSupplierDailyBatchCandidates(ctx, tx, prepared.startDate, prepared.targetDate); err != nil {
			return err
		}
		batchDate, found, err := model.OldestNeverPublishedSupplierDailyBatchDate(ctx, tx, prepared.startDate, prepared.targetDate)
		if err != nil {
			return err
		}
		if !found {
			response, completeErr := completeSupplierBatchNoWork(ctx, tx, claim, requestID)
			if completeErr != nil {
				return completeErr
			}
			prepared.response = &response
			return nil
		}
		day, err := time.ParseInLocation("2006-01-02", batchDate, location)
		if err != nil {
			return err
		}
		owner := "scheduler:" + hex.EncodeToString(identityDigest[:8])
		lease, err := model.AcquireSupplierDailyBatch(ctx, tx, batchDate, day.Unix(), day.AddDate(0, 0, 1).Unix(), owner, now, supplierDailyLeaseDuration, false)
		if err != nil {
			return mapSupplierBatchCommandError(err)
		}
		lockedUntil := now.Add(supplierDailyLeaseDuration).Truncate(time.Second).Format(time.RFC3339)
		state := types.SupplierBatchCommandStateV1{SchemaVersion: types.SupplierBatchCommandSchemaVersion, State: types.SupplierBatchCommandStateRunning, Response: &types.SupplierBatchCommandStatusV1{
			RequestID: requestID, BatchDate: stringPointer(batchDate), RunID: int64Pointer(lease.RunId), Status: types.SupplierBatchCommandStateRunning,
			FenceToken: lease.FenceToken, PublishedFenceToken: 0, LockedUntil: &lockedUntil, ErrorCategory: types.SupplierBatchErrorNone,
		}}
		if err := model.StoreSupplierBatchSchedulerCommandState(ctx, tx, claim, state); err != nil {
			return err
		}
		prepared.lease = lease
		prepared.day = day
		return nil
	})
	if err != nil {
		return nil, err
	}
	return prepared, nil
}

func GetSupplierDailyBatchRequestStatus(ctx context.Context, mainDB *gorm.DB, principal dto.SupplierBatchSchedulerPrincipal, requestID string, now time.Time) (dto.SupplierBatchStatusResponse, error) {
	return getSupplierDailyBatchRequestStatus(ctx, mainDB, model.LOG_DB, principal, requestID, now)
}

func getSupplierDailyBatchRequestStatus(ctx context.Context, mainDB, recoveryLogDB *gorm.DB, principal dto.SupplierBatchSchedulerPrincipal, requestID string, now time.Time) (dto.SupplierBatchStatusResponse, error) {
	identityDigest, err := validateSupplierBatchRequest(principal, requestID)
	if err != nil {
		return dto.SupplierBatchStatusResponse{}, err
	}
	claim, err := model.GetSupplierBatchSchedulerCommand(ctx, mainDB, identityDigest, requestID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.SupplierBatchStatusResponse{}, ErrSupplierBatchRequestNotFound
	}
	if err != nil {
		return dto.SupplierBatchStatusResponse{}, mapSupplierBatchCommandError(err)
	}
	if claim.State.State == types.SupplierBatchCommandStateClaimed {
		return recoverClaimedSupplierBatchRequest(ctx, mainDB, recoveryLogDB, principal, requestID, now)
	}
	if claim.State.State == types.SupplierBatchCommandStateRunning {
		return reconcileSupplierBatchRunningCommand(ctx, mainDB, claim, now)
	}
	return supplierBatchCommandResponseToDTO(claim.State)
}

func recoverClaimedSupplierBatchRequest(ctx context.Context, mainDB, logDB *gorm.DB, principal dto.SupplierBatchSchedulerPrincipal, requestID string, now time.Time) (dto.SupplierBatchStatusResponse, error) {
	for {
		response, err := CatchUpSupplierDailyBatchesByRequest(ctx, mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: requestID}, now)
		if err == nil {
			return response, response.Validate()
		}
		if !errors.Is(err, ErrSupplierBatchBusy) {
			return dto.SupplierBatchStatusResponse{}, err
		}
		select {
		case <-ctx.Done():
			return dto.SupplierBatchStatusResponse{}, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
		identityDigest, digestErr := validateSupplierBatchRequest(principal, requestID)
		if digestErr != nil {
			return dto.SupplierBatchStatusResponse{}, digestErr
		}
		claim, getErr := model.GetSupplierBatchSchedulerCommand(ctx, mainDB, identityDigest, requestID)
		if getErr != nil {
			return dto.SupplierBatchStatusResponse{}, mapSupplierBatchCommandError(getErr)
		}
		if claim.State.State != types.SupplierBatchCommandStateClaimed {
			return supplierBatchCommandResponseToDTO(claim.State)
		}
	}
}

func reconcileSupplierBatchRunningCommand(ctx context.Context, db *gorm.DB, claim *model.SupplierBatchSchedulerCommandClaim, now time.Time) (dto.SupplierBatchStatusResponse, error) {
	if claim == nil || claim.State.Response == nil || claim.State.Response.BatchDate == nil || claim.State.Response.RunID == nil || claim.State.Response.FenceToken <= 0 {
		return dto.SupplierBatchStatusResponse{}, ErrSupplierBatchRequestNotFound
	}
	command := claim.State.Response
	var run model.SupplierUsageDailyBatchRun
	if err := db.WithContext(ctx).Where("id = ? AND batch_date = ?", *command.RunID, *command.BatchDate).First(&run).Error; err != nil {
		return dto.SupplierBatchStatusResponse{}, err
	}
	lease := model.SupplierDailyBatchLease{RunId: run.Id, BatchDate: run.BatchDate, FenceToken: command.FenceToken}
	if run.PublishedFenceToken == command.FenceToken {
		remaining, nextDate, err := supplierBatchRemainingWork(ctx, db, now)
		if err != nil {
			return dto.SupplierBatchStatusResponse{}, err
		}
		result := &dto.SupplierBatchStatusResult{ProcessedDays: 1, RemainingWork: remaining}
		if remaining {
			result.NextBatchDate = &nextDate
		}
		response := dto.SupplierBatchStatusResponse{
			RequestID: command.RequestID, BatchDate: command.BatchDate, RunID: command.RunID, Status: dto.SupplierBatchStatusCompleted,
			FenceToken: command.FenceToken, PublishedFenceToken: run.PublishedFenceToken, ErrorCategory: dto.SupplierBatchErrorNone, Result: result,
		}
		if err := response.Validate(); err != nil {
			return dto.SupplierBatchStatusResponse{}, err
		}
		reconciled, err := model.ReconcileSupplierBatchSchedulerCommandState(ctx, db, claim, supplierBatchDTOToCommandState(response))
		if err != nil {
			return dto.SupplierBatchStatusResponse{}, mapSupplierBatchCommandError(err)
		}
		return supplierBatchCommandResponseToDTO(reconciled.State)
	}
	if run.PublishedFenceToken > 0 || run.FenceToken != command.FenceToken {
		remaining, nextDate, err := supplierBatchRemainingWork(ctx, db, now)
		if err != nil {
			return dto.SupplierBatchStatusResponse{}, err
		}
		response := failedSupplierBatchCommandResponse(command.RequestID, run.BatchDate, lease, dto.SupplierBatchErrorFenceLost, remaining)
		if remaining {
			response.Result.NextBatchDate = &nextDate
		}
		response.PublishedFenceToken = run.PublishedFenceToken
		return reconcileSupplierBatchSchedulerTerminal(ctx, db, claim, response)
	}
	if run.Status == model.SupplierDailyBatchStatusFailed {
		response := failedSupplierBatchCommandResponse(command.RequestID, run.BatchDate, lease, dto.SupplierBatchErrorExecutionFailed, true)
		response.PublishedFenceToken = command.PublishedFenceToken
		return reconcileSupplierBatchSchedulerTerminal(ctx, db, claim, response)
	}
	if run.Status != model.SupplierDailyBatchStatusRunning || run.LockedUntil <= 0 {
		return dto.SupplierBatchStatusResponse{}, ErrSupplierBatchRequestNotFound
	}
	leaseExpired, err := model.SupplierDailyBatchLeaseExpired(ctx, db, run.LockedUntil)
	if err != nil {
		return dto.SupplierBatchStatusResponse{}, err
	}
	if leaseExpired {
		// Keep the command ledger unchanged so the same request can atomically
		// take over its claim. The database lease is the authoritative expiry
		// proof, including when the ledger's cached timestamp is stale.
		expiredLockedUntil := now.Add(-time.Second).UTC().Format(time.RFC3339)
		command.LockedUntil = &expiredLockedUntil
		response := failedSupplierBatchCommandResponse(command.RequestID, run.BatchDate, lease, dto.SupplierBatchErrorLeaseExpired, true)
		response.PublishedFenceToken = run.PublishedFenceToken
		return response, response.Validate()
	}
	lockedUntil := time.Unix(run.LockedUntil, 0).UTC().Format(time.RFC3339)
	response := dto.SupplierBatchStatusResponse{
		RequestID: command.RequestID, BatchDate: command.BatchDate, RunID: command.RunID, Status: dto.SupplierBatchStatusRunning,
		FenceToken: command.FenceToken, PublishedFenceToken: run.PublishedFenceToken, LockedUntil: &lockedUntil, ErrorCategory: dto.SupplierBatchErrorNone,
	}
	return response, response.Validate()
}

func supplierBatchRemainingWork(ctx context.Context, db *gorm.DB, now time.Time) (bool, string, error) {
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	if err != nil {
		return false, "", err
	}
	today := beginningOfSupplierDay(now.In(location))
	target := today.AddDate(0, 0, -1)
	cutoverAt, err := supplierBatchConfiguredCutover(ctx, db)
	if err != nil {
		return false, "", err
	}
	start := beginningOfSupplierDay(time.Unix(cutoverAt, 0).In(location))
	if now.In(location).Before(today.Add(SupplierDailyCloseGrace)) || start.After(target) {
		return false, "", nil
	}
	nextDate, remaining, err := model.OldestNeverPublishedSupplierDailyBatchDate(ctx, db, start.Format("2006-01-02"), target.Format("2006-01-02"))
	return remaining, nextDate, err
}

func supplierBatchStatusResponseExpired(response dto.SupplierBatchStatusResponse, now time.Time) bool {
	if response.LockedUntil == nil {
		return false
	}
	lockedUntil, err := time.Parse(time.RFC3339, *response.LockedUntil)
	return err == nil && lockedUntil.Before(now)
}

func RerunSupplierDailyReport(ctx context.Context, mainDB, logDB *gorm.DB, actorID int, batchDate, idempotencyKey string, request dto.SupplierDailyReportRerunRequest, now time.Time) (dto.SupplierBatchStatusResponse, error) {
	if actorID <= 0 || strings.TrimSpace(idempotencyKey) == "" || idempotencyKey != strings.TrimSpace(idempotencyKey) || len(idempotencyKey) > 128 ||
		strings.TrimSpace(request.Reason) == "" || request.ExpectedPublishedFenceToken <= 0 {
		return dto.SupplierBatchStatusResponse{}, ErrSupplierDailyReportInvalid
	}
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	if err != nil {
		return dto.SupplierBatchStatusResponse{}, err
	}
	day, err := time.ParseInLocation("2006-01-02", batchDate, location)
	if err != nil || !day.Before(beginningOfSupplierDay(now.In(location))) {
		return dto.SupplierBatchStatusResponse{}, ErrSupplierDailyReportInvalid
	}
	if existing, getErr := model.GetSupplierDailyReportRerunCommand(ctx, mainDB, actorID, batchDate, idempotencyKey); getErr == nil {
		verified, claimErr := model.ClaimSupplierDailyReportRerunCommand(ctx, mainDB, actorID, batchDate, idempotencyKey, request)
		if claimErr != nil {
			return dto.SupplierBatchStatusResponse{}, mapSupplierDailyReportCommandError(claimErr)
		}
		existing = verified
		if existing.State.State == types.SupplierBatchCommandStateCompleted || existing.State.State == types.SupplierBatchCommandStateFailed {
			return supplierBatchCommandResponseToDTO(existing.State)
		}
		if existing.State.State == types.SupplierBatchCommandStateRunning {
			reconciled, reconcileErr := reconcileSupplierDailyReportRerunRunningCommand(ctx, mainDB, existing, now)
			if reconcileErr != nil {
				return dto.SupplierBatchStatusResponse{}, reconcileErr
			}
			takeoverEligible := reconciled.Status == dto.SupplierBatchStatusRunning && supplierBatchStatusResponseExpired(reconciled, now)
			takeoverEligible = takeoverEligible || reconciled.Status == dto.SupplierBatchStatusFailed && reconciled.ErrorCategory == dto.SupplierBatchErrorLeaseExpired
			if !takeoverEligible {
				return reconciled, nil
			}
		}
	} else if !errors.Is(getErr, gorm.ErrRecordNotFound) {
		return dto.SupplierBatchStatusResponse{}, getErr
	}

	var claim *model.SupplierDailyReportRerunCommandClaim
	var lease model.SupplierDailyBatchLease
	var publishedFence int64
	var cutoverAt int64
	var replayedResponse *dto.SupplierBatchStatusResponse
	err = mainDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var loadErr error
		claim, loadErr = model.ClaimSupplierDailyReportRerunCommand(ctx, tx, actorID, batchDate, idempotencyKey, request)
		if loadErr != nil {
			return mapSupplierDailyReportCommandError(loadErr)
		}
		if claim.Replayed {
			switch claim.State.State {
			case types.SupplierBatchCommandStateCompleted, types.SupplierBatchCommandStateFailed:
				response, convertErr := supplierBatchCommandResponseToDTO(claim.State)
				if convertErr != nil {
					return convertErr
				}
				replayedResponse = &response
				return nil
			case types.SupplierBatchCommandStateRunning:
				response, reconcileErr := reconcileSupplierDailyReportRerunRunningCommand(ctx, tx, claim, now)
				if reconcileErr != nil {
					return reconcileErr
				}
				takeoverEligible := response.Status == dto.SupplierBatchStatusRunning && supplierBatchStatusResponseExpired(response, now)
				takeoverEligible = takeoverEligible || response.Status == dto.SupplierBatchStatusFailed && response.ErrorCategory == dto.SupplierBatchErrorLeaseExpired
				if !takeoverEligible {
					replayedResponse = &response
					return nil
				}
				claim, loadErr = model.TakeoverSupplierDailyReportRerunCommand(ctx, tx, claim, now)
			case types.SupplierBatchCommandStateClaimed:
				claim, loadErr = model.TakeoverSupplierDailyReportRerunCommand(ctx, tx, claim, now)
			default:
				return model.ErrSupplierAdminCommandIncomplete
			}
			if loadErr != nil {
				return mapSupplierDailyReportCommandError(loadErr)
			}
		}

		publishedBefore, evidenceBefore, loadErr := model.LoadSupplierPublishedDailyBatch(ctx, tx, batchDate)
		if errors.Is(loadErr, gorm.ErrRecordNotFound) {
			return ErrSupplierDailyReportNotFound
		}
		if loadErr != nil {
			return loadErr
		}
		if publishedBefore.PublishedFenceToken != request.ExpectedPublishedFenceToken {
			return ErrSupplierDailyReportVersionConflict
		}
		if evidenceBefore.PersistedLogSnapshotCompleteness != types.SupplierPersistedLogCompletenessIncomplete {
			return ErrSupplierDailyReportNotEligible
		}
		owner := supplierDailyReportRerunLeaseOwner(actorID, idempotencyKey)
		lease, loadErr = model.AcquireSupplierDailyBatchRerun(ctx, tx, batchDate, day.Unix(), day.AddDate(0, 0, 1).Unix(), owner, now, supplierDailyLeaseDuration, request.ExpectedPublishedFenceToken)
		if loadErr != nil {
			return loadErr
		}
		lockedUntil := now.Add(supplierDailyLeaseDuration).Truncate(time.Second).Format(time.RFC3339)
		state := types.SupplierBatchCommandStateV1{SchemaVersion: types.SupplierBatchCommandSchemaVersion, State: types.SupplierBatchCommandStateRunning, Response: &types.SupplierBatchCommandStatusV1{
			RequestID: idempotencyKey, BatchDate: stringPointer(batchDate), RunID: int64Pointer(lease.RunId), Status: types.SupplierBatchCommandStateRunning,
			FenceToken: lease.FenceToken, PublishedFenceToken: publishedBefore.PublishedFenceToken, LockedUntil: &lockedUntil, ErrorCategory: types.SupplierBatchErrorNone,
		}}
		if loadErr = model.StoreSupplierDailyReportRerunCommandState(ctx, tx, claim, state); loadErr != nil {
			return loadErr
		}
		publishedFence = publishedBefore.PublishedFenceToken
		cutoverAt, loadErr = supplierBatchConfiguredCutover(ctx, tx)
		return loadErr
	})
	if errors.Is(err, model.ErrSupplierDailyBatchBusy) {
		return dto.SupplierBatchStatusResponse{}, fmt.Errorf("%w: %v", ErrSupplierBatchBusy, err)
	}
	if errors.Is(err, model.ErrSupplierDailyBatchNotRerunnable) {
		return dto.SupplierBatchStatusResponse{}, ErrSupplierDailyReportNotEligible
	}
	if err != nil {
		return dto.SupplierBatchStatusResponse{}, err
	}
	if replayedResponse != nil {
		return *replayedResponse, nil
	}
	evidence, scanErr := scanAcquiredSupplierDailyBatch(ctx, mainDB, logDB, lease, day, cutoverAt)
	if scanErr != nil {
		response := failedSupplierBatchCommandResponse(idempotencyKey, batchDate, lease, supplierBatchErrorCategory(scanErr), false)
		response.PublishedFenceToken = publishedFence
		if recoveryErr := finalizeFailedSupplierDailyBatch(ctx, mainDB, lease, scanErr, func(recoveryCtx context.Context, tx *gorm.DB) error {
			return model.StoreSupplierDailyReportRerunCommandState(recoveryCtx, tx, claim, supplierBatchDTOToCommandState(response))
		}); recoveryErr != nil {
			reconciled, reconcileErr := recoverFailedSupplierDailyReportRerunCommand(ctx, mainDB, claim, lease, scanErr, response)
			if reconcileErr != nil {
				return dto.SupplierBatchStatusResponse{}, errors.Join(scanErr, fmt.Errorf("finalize failed supplier daily report rerun: %w", recoveryErr), fmt.Errorf("reconcile failed supplier daily report rerun: %w", reconcileErr))
			}
			return reconciled, nil
		}
		return response, nil
	}
	var response dto.SupplierBatchStatusResponse
	err = mainDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		completedAt := time.Now()
		if publishErr := model.PublishSupplierDailyBatchTx(ctx, tx, lease, completedAt, evidence); publishErr != nil {
			return fmt.Errorf("publish supplier daily publication: %w", publishErr)
		}
		var responseErr error
		response, responseErr = completedSupplierDailyReportRerunResponse(idempotencyKey, &model.SupplierUsageDailyBatchRun{Id: lease.RunId, BatchDate: batchDate}, lease.FenceToken)
		if responseErr != nil {
			return responseErr
		}
		return model.StoreSupplierDailyReportRerunCommandState(ctx, tx, claim, supplierBatchDTOToCommandState(response))
	})
	if err != nil {
		response := failedSupplierBatchCommandResponse(idempotencyKey, batchDate, lease, supplierBatchErrorCategory(err), false)
		response.PublishedFenceToken = publishedFence
		if recoveryErr := finalizeFailedSupplierDailyBatch(ctx, mainDB, lease, err, func(recoveryCtx context.Context, tx *gorm.DB) error {
			return model.StoreSupplierDailyReportRerunCommandState(recoveryCtx, tx, claim, supplierBatchDTOToCommandState(response))
		}); recoveryErr != nil {
			if _, reconcileErr := recoverFailedSupplierDailyReportRerunCommand(ctx, mainDB, claim, lease, err, response); reconcileErr != nil {
				return dto.SupplierBatchStatusResponse{}, errors.Join(err, fmt.Errorf("finalize failed supplier daily report rerun: %w", recoveryErr), fmt.Errorf("reconcile failed supplier daily report rerun: %w", reconcileErr))
			}
		}
		return dto.SupplierBatchStatusResponse{}, err
	}
	return response, nil
}

func supplierDailyReportRerunLeaseOwner(actorID int, idempotencyKey string) string {
	digest := sha256.Sum256([]byte(idempotencyKey))
	return fmt.Sprintf("rerun:%d:sha256:%x", actorID, digest)
}

func completedSupplierDailyReportRerunResponse(requestID string, run *model.SupplierUsageDailyBatchRun, publishedFence int64) (dto.SupplierBatchStatusResponse, error) {
	if run == nil || publishedFence <= 0 {
		return dto.SupplierBatchStatusResponse{}, ErrSupplierDailyReportNotFound
	}
	response := dto.SupplierBatchStatusResponse{
		RequestID: requestID, BatchDate: stringPointer(run.BatchDate), RunID: int64Pointer(run.Id), Status: dto.SupplierBatchStatusCompleted,
		FenceToken: publishedFence, PublishedFenceToken: publishedFence, ErrorCategory: dto.SupplierBatchErrorNone,
		Result: &dto.SupplierBatchStatusResult{ProcessedDays: 1, RemainingWork: false},
	}
	return response, response.Validate()
}

func executeSupplierBatchCatchUpRequest(ctx context.Context, mainDB, logDB *gorm.DB, requestID string, now time.Time, prepared *supplierBatchPreparedRequest) (dto.SupplierBatchStatusResponse, error) {
	batchDate := prepared.lease.BatchDate
	evidence, scanErr := scanAcquiredSupplierDailyBatch(ctx, mainDB, logDB, prepared.lease, prepared.day, prepared.cutoverAt)
	if scanErr != nil {
		response := failedSupplierBatchCommandResponse(requestID, batchDate, prepared.lease, supplierBatchErrorCategory(scanErr), true)
		if recoveryErr := finalizeFailedSupplierDailyBatch(ctx, mainDB, prepared.lease, scanErr, func(recoveryCtx context.Context, tx *gorm.DB) error {
			return model.StoreSupplierBatchSchedulerCommandState(recoveryCtx, tx, prepared.claim, supplierBatchDTOToCommandState(response))
		}); recoveryErr != nil {
			reconciled, reconcileErr := recoverFailedSupplierBatchSchedulerCommand(ctx, mainDB, prepared.claim, prepared.lease, scanErr, response, now)
			if reconcileErr != nil {
				return dto.SupplierBatchStatusResponse{}, errors.Join(scanErr, fmt.Errorf("finalize failed supplier batch scheduler command: %w", recoveryErr), fmt.Errorf("reconcile failed supplier batch scheduler command: %w", reconcileErr))
			}
			return reconciled, nil
		}
		return response, nil
	}

	var response dto.SupplierBatchStatusResponse
	err := mainDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		completedAt := time.Now()
		if publishErr := model.PublishSupplierDailyBatchTx(ctx, tx, prepared.lease, completedAt, evidence); publishErr != nil {
			return fmt.Errorf("publish supplier daily publication: %w", publishErr)
		}
		nextDate, remaining, queryErr := model.OldestNeverPublishedSupplierDailyBatchDate(ctx, tx, prepared.startDate, prepared.targetDate)
		if queryErr != nil {
			return queryErr
		}
		result := &dto.SupplierBatchStatusResult{ProcessedDays: 1, RemainingWork: remaining}
		if remaining {
			result.NextBatchDate = &nextDate
		}
		response = dto.SupplierBatchStatusResponse{
			RequestID: requestID, BatchDate: stringPointer(batchDate), RunID: int64Pointer(prepared.lease.RunId), Status: dto.SupplierBatchStatusCompleted,
			FenceToken: prepared.lease.FenceToken, PublishedFenceToken: prepared.lease.FenceToken, ErrorCategory: dto.SupplierBatchErrorNone, Result: result,
		}
		if validateErr := response.Validate(); validateErr != nil {
			return validateErr
		}
		return model.StoreSupplierBatchSchedulerCommandState(ctx, tx, prepared.claim, supplierBatchDTOToCommandState(response))
	})
	if err != nil {
		response := failedSupplierBatchCommandResponse(requestID, batchDate, prepared.lease, supplierBatchErrorCategory(err), true)
		if recoveryErr := finalizeFailedSupplierDailyBatch(ctx, mainDB, prepared.lease, err, func(recoveryCtx context.Context, tx *gorm.DB) error {
			return model.StoreSupplierBatchSchedulerCommandState(recoveryCtx, tx, prepared.claim, supplierBatchDTOToCommandState(response))
		}); recoveryErr != nil {
			if _, reconcileErr := recoverFailedSupplierBatchSchedulerCommand(ctx, mainDB, prepared.claim, prepared.lease, err, response, now); reconcileErr != nil {
				return dto.SupplierBatchStatusResponse{}, errors.Join(err, fmt.Errorf("finalize failed supplier batch scheduler command: %w", recoveryErr), fmt.Errorf("reconcile failed supplier batch scheduler command: %w", reconcileErr))
			}
		}
		return dto.SupplierBatchStatusResponse{}, err
	}
	return response, nil
}

func finalizeFailedSupplierDailyBatch(requestCtx context.Context, mainDB *gorm.DB, lease model.SupplierDailyBatchLease, cause error, storeTerminal func(context.Context, *gorm.DB) error) error {
	recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(requestCtx), supplierBatchTerminalRecoveryTimeout)
	defer cancel()
	return mainDB.WithContext(recoveryCtx).Transaction(func(tx *gorm.DB) error {
		if err := model.FailSupplierDailyBatchTx(recoveryCtx, tx, lease, cause); err != nil {
			return err
		}
		return storeTerminal(recoveryCtx, tx)
	})
}

func recoverFailedSupplierBatchSchedulerCommand(requestCtx context.Context, mainDB *gorm.DB, claim *model.SupplierBatchSchedulerCommandClaim, lease model.SupplierDailyBatchLease, cause error, failed dto.SupplierBatchStatusResponse, now time.Time) (dto.SupplierBatchStatusResponse, error) {
	recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(requestCtx), supplierBatchTerminalRecoveryTimeout)
	defer cancel()
	failErr := model.FailSupplierDailyBatch(recoveryCtx, mainDB, lease, cause)
	replay, loadErr := reloadSupplierBatchSchedulerCommand(recoveryCtx, mainDB, claim)
	if loadErr != nil {
		return dto.SupplierBatchStatusResponse{}, loadErr
	}
	if failErr == nil {
		return reconcileSupplierBatchSchedulerTerminal(recoveryCtx, mainDB, replay, failed)
	}
	if !errors.Is(failErr, model.ErrSupplierDailyBatchFenceLost) {
		return dto.SupplierBatchStatusResponse{}, failErr
	}
	return reconcileSupplierBatchRunningCommand(recoveryCtx, mainDB, replay, now)
}

func reloadSupplierBatchSchedulerCommand(ctx context.Context, mainDB *gorm.DB, claim *model.SupplierBatchSchedulerCommandClaim) (*model.SupplierBatchSchedulerCommandClaim, error) {
	if claim == nil || claim.Command.TrustedJobIdentityDigest == nil {
		return nil, model.ErrSupplierAdminCommandIncomplete
	}
	digest, err := hex.DecodeString(*claim.Command.TrustedJobIdentityDigest)
	if err != nil {
		return nil, model.ErrSupplierAdminCommandIncomplete
	}
	return model.GetSupplierBatchSchedulerCommand(ctx, mainDB, digest, claim.Command.IdempotencyKey)
}

func reconcileSupplierBatchSchedulerTerminal(ctx context.Context, mainDB *gorm.DB, replay *model.SupplierBatchSchedulerCommandClaim, response dto.SupplierBatchStatusResponse) (dto.SupplierBatchStatusResponse, error) {
	if err := response.Validate(); err != nil {
		return dto.SupplierBatchStatusResponse{}, err
	}
	reconciled, err := model.ReconcileSupplierBatchSchedulerCommandState(ctx, mainDB, replay, supplierBatchDTOToCommandState(response))
	if err != nil {
		return dto.SupplierBatchStatusResponse{}, mapSupplierBatchCommandError(err)
	}
	return supplierBatchCommandResponseToDTO(reconciled.State)
}

func recoverFailedSupplierDailyReportRerunCommand(requestCtx context.Context, mainDB *gorm.DB, claim *model.SupplierDailyReportRerunCommandClaim, lease model.SupplierDailyBatchLease, cause error, failed dto.SupplierBatchStatusResponse) (dto.SupplierBatchStatusResponse, error) {
	recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(requestCtx), supplierBatchTerminalRecoveryTimeout)
	defer cancel()
	failErr := model.FailSupplierDailyBatch(recoveryCtx, mainDB, lease, cause)
	replay, loadErr := model.GetSupplierDailyReportRerunCommand(recoveryCtx, mainDB, claim.Command.ActorId, lease.BatchDate, claim.Command.IdempotencyKey)
	if loadErr != nil {
		return dto.SupplierBatchStatusResponse{}, loadErr
	}
	if failErr == nil {
		return reconcileSupplierDailyReportRerunTerminal(recoveryCtx, mainDB, replay, failed)
	}
	if !errors.Is(failErr, model.ErrSupplierDailyBatchFenceLost) {
		return dto.SupplierBatchStatusResponse{}, failErr
	}
	return reconcileSupplierDailyReportRerunRunningCommand(recoveryCtx, mainDB, replay, time.Now())
}

func reconcileSupplierDailyReportRerunRunningCommand(ctx context.Context, mainDB *gorm.DB, replay *model.SupplierDailyReportRerunCommandClaim, now time.Time) (dto.SupplierBatchStatusResponse, error) {
	if replay == nil || replay.State.Response == nil || replay.State.Response.BatchDate == nil || replay.State.Response.RunID == nil || replay.State.Response.FenceToken <= 0 {
		return dto.SupplierBatchStatusResponse{}, ErrSupplierBatchRequestNotFound
	}
	command := replay.State.Response
	var run model.SupplierUsageDailyBatchRun
	if err := mainDB.WithContext(ctx).Where("id = ? AND batch_date = ?", *command.RunID, *command.BatchDate).First(&run).Error; err != nil {
		return dto.SupplierBatchStatusResponse{}, err
	}
	lease := model.SupplierDailyBatchLease{RunId: run.Id, BatchDate: run.BatchDate, FenceToken: command.FenceToken}
	if run.PublishedFenceToken == command.FenceToken {
		response, err := completedSupplierDailyReportRerunResponse(command.RequestID, &run, command.FenceToken)
		if err != nil {
			return dto.SupplierBatchStatusResponse{}, err
		}
		return reconcileSupplierDailyReportRerunTerminal(ctx, mainDB, replay, response)
	}
	if run.PublishedFenceToken > command.PublishedFenceToken || run.FenceToken != command.FenceToken {
		response := failedSupplierBatchCommandResponse(command.RequestID, run.BatchDate, lease, dto.SupplierBatchErrorFenceLost, false)
		response.PublishedFenceToken = run.PublishedFenceToken
		return reconcileSupplierDailyReportRerunTerminal(ctx, mainDB, replay, response)
	}
	if run.Status == model.SupplierDailyBatchStatusFailed {
		response := failedSupplierBatchCommandResponse(command.RequestID, run.BatchDate, lease, dto.SupplierBatchErrorExecutionFailed, false)
		response.PublishedFenceToken = command.PublishedFenceToken
		return reconcileSupplierDailyReportRerunTerminal(ctx, mainDB, replay, response)
	}
	if run.Status != model.SupplierDailyBatchStatusRunning || run.LockedUntil <= 0 {
		return dto.SupplierBatchStatusResponse{}, ErrSupplierBatchRequestNotFound
	}
	leaseExpired, err := model.SupplierDailyBatchLeaseExpired(ctx, mainDB, run.LockedUntil)
	if err != nil {
		return dto.SupplierBatchStatusResponse{}, err
	}
	if leaseExpired {
		expiredLockedUntil := now.Add(-time.Second).UTC().Format(time.RFC3339)
		command.LockedUntil = &expiredLockedUntil
		response := failedSupplierBatchCommandResponse(command.RequestID, run.BatchDate, lease, dto.SupplierBatchErrorLeaseExpired, false)
		response.PublishedFenceToken = command.PublishedFenceToken
		return response, response.Validate()
	}
	lockedUntil := time.Unix(run.LockedUntil, 0).UTC().Format(time.RFC3339)
	response := dto.SupplierBatchStatusResponse{
		RequestID: command.RequestID, BatchDate: command.BatchDate, RunID: command.RunID, Status: dto.SupplierBatchStatusRunning,
		FenceToken: command.FenceToken, PublishedFenceToken: command.PublishedFenceToken, LockedUntil: &lockedUntil, ErrorCategory: dto.SupplierBatchErrorNone,
	}
	return response, response.Validate()
}

func reconcileSupplierDailyReportRerunTerminal(ctx context.Context, mainDB *gorm.DB, replay *model.SupplierDailyReportRerunCommandClaim, response dto.SupplierBatchStatusResponse) (dto.SupplierBatchStatusResponse, error) {
	if err := response.Validate(); err != nil {
		return dto.SupplierBatchStatusResponse{}, err
	}
	reconciled, err := model.ReconcileSupplierDailyReportRerunCommandState(ctx, mainDB, replay, supplierBatchDTOToCommandState(response))
	if err != nil {
		return dto.SupplierBatchStatusResponse{}, mapSupplierDailyReportCommandError(err)
	}
	return supplierBatchCommandResponseToDTO(reconciled.State)
}

func completeSupplierBatchNoWork(ctx context.Context, mainDB *gorm.DB, claim *model.SupplierBatchSchedulerCommandClaim, requestID string) (dto.SupplierBatchStatusResponse, error) {
	response := dto.SupplierBatchStatusResponse{
		RequestID: requestID, Status: dto.SupplierBatchStatusCompleted, ErrorCategory: dto.SupplierBatchErrorNone,
		Result: &dto.SupplierBatchStatusResult{ProcessedDays: 0, RemainingWork: false},
	}
	if err := response.Validate(); err != nil {
		return dto.SupplierBatchStatusResponse{}, err
	}
	if err := model.StoreSupplierBatchSchedulerCommandState(ctx, mainDB, claim, supplierBatchDTOToCommandState(response)); err != nil {
		return dto.SupplierBatchStatusResponse{}, err
	}
	return response, nil
}

func validateSupplierBatchRequest(principal dto.SupplierBatchSchedulerPrincipal, requestID string) ([]byte, error) {
	if strings.TrimSpace(requestID) == "" || requestID != strings.TrimSpace(requestID) || len(requestID) > 128 ||
		(principal.AuditSlot != dto.SupplierBatchAuditSlotCurrent && principal.AuditSlot != dto.SupplierBatchAuditSlotNext) {
		return nil, ErrSupplierBatchConfigUnavailable
	}
	digest, err := model.DigestSupplierBatchTrustedIdentity(principal.TrustedJobIdentity)
	if err != nil {
		return nil, ErrSupplierBatchConfigUnavailable
	}
	return digest, nil
}

func supplierBatchConfiguredCutover(ctx context.Context, db *gorm.DB) (int64, error) {
	cutoverAt, err := supplierAccountingBatchCutover(ctx, db)
	if errors.Is(err, ErrSupplierAccountingNotActive) {
		return 0, ErrSupplierBatchConfigUnavailable
	}
	return cutoverAt, err
}

func supplierBatchCommandExpired(state types.SupplierBatchCommandStateV1, now time.Time) bool {
	if state.Response == nil || state.Response.LockedUntil == nil {
		return false
	}
	lockedUntil, err := time.Parse(time.RFC3339, *state.Response.LockedUntil)
	return err == nil && lockedUntil.Before(now)
}

func supplierBatchCommandResponseToDTO(state types.SupplierBatchCommandStateV1) (dto.SupplierBatchStatusResponse, error) {
	if state.Response == nil {
		return dto.SupplierBatchStatusResponse{}, ErrSupplierBatchRequestNotFound
	}
	response := dto.SupplierBatchStatusResponse{
		RequestID: state.Response.RequestID, BatchDate: state.Response.BatchDate, RunID: state.Response.RunID, Status: state.Response.Status,
		FenceToken: state.Response.FenceToken, PublishedFenceToken: state.Response.PublishedFenceToken,
		LockedUntil: state.Response.LockedUntil, ErrorCategory: state.Response.ErrorCategory,
	}
	if state.Response.Result != nil {
		response.Result = &dto.SupplierBatchStatusResult{ProcessedDays: state.Response.Result.ProcessedDays, RemainingWork: state.Response.Result.RemainingWork, NextBatchDate: state.Response.Result.NextBatchDate}
	}
	return response, response.Validate()
}

func supplierBatchDTOToCommandState(response dto.SupplierBatchStatusResponse) types.SupplierBatchCommandStateV1 {
	commandResponse := &types.SupplierBatchCommandStatusV1{
		RequestID: response.RequestID, BatchDate: response.BatchDate, RunID: response.RunID, Status: response.Status,
		FenceToken: response.FenceToken, PublishedFenceToken: response.PublishedFenceToken,
		LockedUntil: response.LockedUntil, ErrorCategory: response.ErrorCategory,
	}
	if response.Result != nil {
		commandResponse.Result = &types.SupplierBatchCommandResultV1{ProcessedDays: response.Result.ProcessedDays, RemainingWork: response.Result.RemainingWork, NextBatchDate: response.Result.NextBatchDate}
	}
	return types.SupplierBatchCommandStateV1{SchemaVersion: types.SupplierBatchCommandSchemaVersion, State: response.Status, Response: commandResponse}
}

func failedSupplierBatchCommandResponse(requestID, batchDate string, lease model.SupplierDailyBatchLease, category string, remaining bool) dto.SupplierBatchStatusResponse {
	result := &dto.SupplierBatchStatusResult{ProcessedDays: 0, RemainingWork: remaining}
	if remaining {
		result.NextBatchDate = stringPointer(batchDate)
	}
	return dto.SupplierBatchStatusResponse{
		RequestID: requestID, BatchDate: stringPointer(batchDate), RunID: int64Pointer(lease.RunId), Status: dto.SupplierBatchStatusFailed,
		FenceToken: lease.FenceToken, PublishedFenceToken: 0, ErrorCategory: category, Result: result,
	}
}

func supplierBatchErrorCategory(err error) string {
	switch {
	case errors.Is(err, model.ErrSupplierDailyBatchFenceLost):
		return dto.SupplierBatchErrorFenceLost
	case strings.Contains(strings.ToLower(err.Error()), "scan supplier accounting"):
		return dto.SupplierBatchErrorReadFailed
	case strings.Contains(strings.ToLower(err.Error()), "publication"):
		return dto.SupplierBatchErrorPublicationFailed
	default:
		return dto.SupplierBatchErrorExecutionFailed
	}
}

func mapSupplierBatchCommandError(err error) error {
	switch {
	case errors.Is(err, model.ErrSupplierAdminIdempotencyConflict):
		return fmt.Errorf("%w: %v", ErrSupplierBatchIdempotencyConflict, err)
	case errors.Is(err, model.ErrSupplierDailyBatchBusy):
		return fmt.Errorf("%w: %v", ErrSupplierBatchBusy, err)
	default:
		return err
	}
}

func mapSupplierDailyReportCommandError(err error) error {
	if errors.Is(err, model.ErrSupplierAdminIdempotencyConflict) {
		return fmt.Errorf("%w: %v", ErrSupplierBatchIdempotencyConflict, err)
	}
	return err
}

func stringPointer(value string) *string { return &value }
func int64Pointer(value int64) *int64    { return &value }
