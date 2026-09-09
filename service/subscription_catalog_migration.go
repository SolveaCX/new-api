package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const catalogMigrationLifecycleReservationTTLSeconds int64 = 10 * 60

type CatalogMigrationApplyCommand struct {
	PreviewRequest CatalogMigrationPreviewRequest `json:"preview_request"`
	CohortDigest   string                         `json:"cohort_digest"`
	RequestedBy    int                            `json:"-"`
}

type CatalogMigrationCancelCommand struct {
	BatchID     string `json:"batch_id"`
	RequestedBy int    `json:"-"`
}

type CatalogMigrationOperationResult struct {
	Batch   model.SubscriptionCatalogMigrationBatch `json:"batch"`
	Summary CatalogMigrationOperationSummary        `json:"summary"`
	Items   []CatalogMigrationOperationItem         `json:"items"`
}

type CatalogMigrationOperationSummary struct {
	Total          int `json:"total"`
	Scheduled      int `json:"scheduled"`
	Applied        int `json:"applied"`
	Skipped        int `json:"skipped"`
	Failed         int `json:"failed"`
	Superseded     int `json:"superseded"`
	NeedsAttention int `json:"needs_attention"`
	Irreversible   int `json:"irreversible"`
}

type CatalogMigrationOperationItem struct {
	ContractID int64  `json:"contract_id"`
	IntentID   int64  `json:"intent_id,omitempty"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
}

type catalogMigrationStoredManifest struct {
	Version   int                                      `json:"version"`
	Preview   CatalogMigrationPreviewResult            `json:"preview"`
	Snapshots []catalogMigrationStoredContractSnapshot `json:"snapshots"`
}

type catalogMigrationStoredContractSnapshot struct {
	ContractID          int64  `json:"contract_id"`
	CurrentPlanSnapshot string `json:"current_plan_snapshot"`
	TargetPlanSnapshot  string `json:"target_plan_snapshot"`
}

type CatalogMigrationProviderScheduleRequest struct {
	BatchID                      string
	IntentID                     int64
	ContractID                   int64
	UserID                       int
	ChangeVersion                int64
	BindingID                    int64
	ProviderSubscriptionID       string
	ProviderSubscriptionItemID   string
	ProviderCustomerID           string
	CurrentPriceID               string
	TargetPriceID                string
	CurrentPeriodStart           int64
	CurrentPeriodEnd             int64
	IdempotencyKey               string
	ExpectedOwnershipFingerprint string
	ExpectedProviderFingerprint  string
	CurrentPlanSnapshot          string
	TargetPlanSnapshot           string
	LifecycleActionSeq           int64
	LifecycleReservationToken    string
	LifecycleReservationUntil    int64
	PreviousScheduleSnapshot     string
}

type CatalogMigrationProviderScheduleResult struct {
	ScheduleID               string
	OwnershipFingerprint     string
	PreviousScheduleSnapshot string
}

type CatalogMigrationProviderScheduler interface {
	ScheduleCatalogMigration(ctx context.Context, request CatalogMigrationProviderScheduleRequest) (CatalogMigrationProviderScheduleResult, error)
	RestoreCatalogMigration(ctx context.Context, request CatalogMigrationProviderScheduleRequest) error
}

type CatalogMigrationService struct {
	DB        *gorm.DB
	Preview   func(context.Context, CatalogMigrationPreviewRequest) (CatalogMigrationPreviewResult, error)
	Scheduler CatalogMigrationProviderScheduler
	Sandbox   CatalogMigrationSandboxConfig
	// AfterProviderSchedule is a deterministic failure seam for compensation tests.
	AfterProviderSchedule func(CatalogMigrationProviderScheduleResult) error
	AfterProviderRestore  func() error
}

var catalogMigrationRuntimeProviderScheduler CatalogMigrationProviderScheduler

func ApplySubscriptionCatalogMigration(ctx context.Context, command CatalogMigrationApplyCommand) (CatalogMigrationOperationResult, error) {
	service := runtimeCatalogMigrationService()
	return service.Apply(ctx, command)
}

func GetSubscriptionCatalogMigration(ctx context.Context, batchID string) (CatalogMigrationOperationResult, error) {
	service := runtimeCatalogMigrationService()
	return service.Get(ctx, batchID)
}

func CancelSubscriptionCatalogMigration(ctx context.Context, command CatalogMigrationCancelCommand) (CatalogMigrationOperationResult, error) {
	service := runtimeCatalogMigrationService()
	return service.Cancel(ctx, command)
}

func runtimeCatalogMigrationService() CatalogMigrationService {
	return CatalogMigrationService{
		DB:        model.DB,
		Preview:   PreviewSubscriptionCatalogMigration,
		Scheduler: catalogMigrationRuntimeProviderScheduler,
		Sandbox:   catalogMigrationRuntimeSandboxConfig(),
	}
}

func (s CatalogMigrationService) Apply(ctx context.Context, command CatalogMigrationApplyCommand) (CatalogMigrationOperationResult, error) {
	if s.DB == nil {
		return CatalogMigrationOperationResult{}, errors.New("catalog migration database is unavailable")
	}
	command.CohortDigest = strings.ToLower(strings.TrimSpace(command.CohortDigest))
	command.PreviewRequest.RequestID = strings.TrimSpace(command.PreviewRequest.RequestID)
	if command.RequestedBy <= 0 || command.PreviewRequest.RequestID == "" || len(command.CohortDigest) != 64 {
		return CatalogMigrationOperationResult{}, errors.New("catalog migration apply command is invalid")
	}
	if existing, found, err := findCatalogMigrationBatchByRequest(ctx, s.DB, command.PreviewRequest.RequestID); err != nil {
		return CatalogMigrationOperationResult{}, err
	} else if found {
		if existing.CohortDigest != command.CohortDigest {
			return CatalogMigrationOperationResult{}, errors.New("catalog migration request_id was already used with a different digest")
		}
		preview, manifestErr := decodeCatalogMigrationStoredManifestForBatch(existing)
		if manifestErr != nil {
			return CatalogMigrationOperationResult{}, manifestErr
		}
		actionable, actionErr := s.catalogMigrationHasActionable(ctx, existing.Id, preview)
		if actionErr != nil {
			return CatalogMigrationOperationResult{}, actionErr
		}
		if !actionable {
			return s.Get(ctx, existing.Id)
		}
		if err := validateCatalogMigrationPreviewSandbox(s.Sandbox, preview); err != nil {
			return CatalogMigrationOperationResult{}, err
		}
		if err := s.processCatalogMigrationPreview(ctx, &existing, preview); err != nil {
			return CatalogMigrationOperationResult{}, err
		}
		return s.Get(ctx, existing.Id)
	}
	if len(command.PreviewRequest.ContractIDs) == 0 {
		return CatalogMigrationOperationResult{}, errors.New("catalog migration contract allowlist is empty")
	}
	for _, contractID := range canonicalContractIDs(command.PreviewRequest.ContractIDs) {
		if err := ValidateCatalogMigrationCommonSandbox(s.Sandbox, contractID); err != nil {
			return CatalogMigrationOperationResult{}, err
		}
	}
	if s.Preview == nil {
		return CatalogMigrationOperationResult{}, errors.New("catalog migration preview dependency is unavailable")
	}
	preview, err := s.Preview(ctx, command.PreviewRequest)
	if err != nil {
		return CatalogMigrationOperationResult{}, err
	}
	recomputedDigest, digestErr := catalogMigrationPreviewDigest(preview)
	if digestErr != nil || preview.CohortDigest != command.CohortDigest || recomputedDigest != command.CohortDigest {
		return CatalogMigrationOperationResult{}, errors.New("catalog migration cohort digest changed")
	}
	manifest, err := encodeCatalogMigrationStoredManifest(preview)
	if err != nil {
		return CatalogMigrationOperationResult{}, fmt.Errorf("encode catalog migration manifest: %w", err)
	}
	batch := model.SubscriptionCatalogMigrationBatch{
		RequestId: command.PreviewRequest.RequestID, CohortDigest: command.CohortDigest,
		Status: model.SubscriptionCatalogMigrationBatchStatusApplying, ManifestSnapshot: string(manifest),
		RequestedBy: command.RequestedBy, DeploymentEnvironment: strings.TrimSpace(s.Sandbox.DeploymentEnvironment),
		ServiceName: strings.TrimSpace(s.Sandbox.ServiceName), SandboxOnly: true, Livemode: false,
	}
	if err := s.DB.WithContext(ctx).Create(&batch).Error; err != nil {
		if existing, found, lookupErr := findCatalogMigrationBatchByRequest(ctx, s.DB, command.PreviewRequest.RequestID); lookupErr == nil && found {
			if existing.CohortDigest != command.CohortDigest {
				return CatalogMigrationOperationResult{}, errors.New("catalog migration request_id was already used with a different digest")
			}
			return s.Get(ctx, existing.Id)
		}
		return CatalogMigrationOperationResult{}, err
	}

	if err := s.processCatalogMigrationPreview(ctx, &batch, preview); err != nil {
		return CatalogMigrationOperationResult{}, err
	}
	return s.Get(ctx, batch.Id)
}

func (s CatalogMigrationService) Get(ctx context.Context, batchID string) (CatalogMigrationOperationResult, error) {
	if s.DB == nil {
		return CatalogMigrationOperationResult{}, errors.New("catalog migration database is unavailable")
	}
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return CatalogMigrationOperationResult{}, errors.New("catalog migration batch_id is required")
	}
	var batch model.SubscriptionCatalogMigrationBatch
	if err := s.DB.WithContext(ctx).Where("id = ?", batchID).First(&batch).Error; err != nil {
		return CatalogMigrationOperationResult{}, err
	}
	preview, err := decodeCatalogMigrationStoredManifestForBatch(batch)
	if err != nil {
		return CatalogMigrationOperationResult{}, err
	}
	var intents []model.SubscriptionChangeIntent
	if err := s.DB.WithContext(ctx).Where("catalog_migration_batch_id = ?", batch.Id).Order("contract_id asc").Find(&intents).Error; err != nil {
		return CatalogMigrationOperationResult{}, err
	}
	byContract := make(map[int64]model.SubscriptionChangeIntent, len(intents))
	for _, intent := range intents {
		byContract[intent.ContractId] = intent
	}
	result := CatalogMigrationOperationResult{Batch: batch, Items: make([]CatalogMigrationOperationItem, 0, len(preview.Contracts))}
	for _, previewItem := range preview.Contracts {
		item := CatalogMigrationOperationItem{ContractID: previewItem.ContractID}
		if !previewItem.Eligible {
			item.Status = "skipped"
			item.Reason = previewItem.Reason
		} else if intent, ok := byContract[previewItem.ContractID]; ok {
			item.IntentID = intent.Id
			item.Status = intent.Status
			item.Reason = publicCatalogMigrationReason(intent.Status)
		} else {
			item.Status = model.SubscriptionChangeIntentStatusFailed
			item.Reason = "eligible contract was not scheduled"
		}
		result.Items = append(result.Items, item)
		addCatalogMigrationSummaryItem(&result.Summary, item)
	}
	result.Batch.Status = catalogMigrationBatchStatus(result.Summary)
	return result, nil
}

func (s CatalogMigrationService) Cancel(ctx context.Context, command CatalogMigrationCancelCommand) (CatalogMigrationOperationResult, error) {
	if command.RequestedBy <= 0 {
		return CatalogMigrationOperationResult{}, errors.New("catalog migration cancel command is invalid")
	}
	current, err := s.Get(ctx, command.BatchID)
	if err != nil {
		return CatalogMigrationOperationResult{}, err
	}
	preview, err := decodeCatalogMigrationStoredManifestForBatch(current.Batch)
	if err != nil {
		return CatalogMigrationOperationResult{}, err
	}
	if err := validateCatalogMigrationPreviewSandbox(s.Sandbox, preview); err != nil {
		return CatalogMigrationOperationResult{}, err
	}
	var intents []model.SubscriptionChangeIntent
	if err := s.DB.WithContext(ctx).Where("catalog_migration_batch_id = ?", current.Batch.Id).Order("contract_id asc").Find(&intents).Error; err != nil {
		return CatalogMigrationOperationResult{}, err
	}
	var cancelErrors []error
	for index := range intents {
		intent := intents[index]
		var itemErr error
		switch intent.Status {
		case model.SubscriptionChangeIntentStatusApplied, model.SubscriptionChangeIntentStatusSuperseded:
			continue
		case model.SubscriptionChangeIntentStatusScheduled,
			model.SubscriptionChangeIntentStatusSyncing,
			model.SubscriptionChangeIntentStatusCompensationRequired,
			model.SubscriptionChangeIntentStatusNeedsAttention:
			if intent.PaymentMode == model.SubscriptionPaymentModeStripeRecurring {
				itemErr = s.cancelStripeIntent(ctx, &current.Batch, &intent)
			} else {
				itemErr = s.cancelLocalIntent(ctx, &intent)
			}
		default:
			itemErr = s.markIntentNeedsAttention(ctx, intent.Id, "catalog migration cancellation requires provider review")
		}
		if itemErr != nil {
			if persistErr := s.markIntentNeedsAttention(ctx, intent.Id, "catalog migration cancellation failed"); persistErr != nil {
				cancelErrors = append(cancelErrors, errors.Join(itemErr, persistErr))
			} else {
				cancelErrors = append(cancelErrors, itemErr)
			}
		}
	}
	if len(cancelErrors) != 0 {
		return CatalogMigrationOperationResult{}, errors.Join(cancelErrors...)
	}
	if err := s.refreshBatchSummary(ctx, current.Batch.Id); err != nil {
		return CatalogMigrationOperationResult{}, err
	}
	result, err := s.Get(ctx, current.Batch.Id)
	if err != nil {
		return CatalogMigrationOperationResult{}, err
	}
	for index := range result.Items {
		if result.Items[index].Status == model.SubscriptionChangeIntentStatusApplied {
			result.Items[index].Reason = "paid migration is irreversible by batch cancellation"
			result.Summary.Irreversible++
		}
	}
	return result, nil
}

func (s CatalogMigrationService) applyWalletContract(ctx context.Context, batch *model.SubscriptionCatalogMigrationBatch, item CatalogMigrationContractPreviewResult) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		contract, targetSnapshot, targetSnapshotJSON, err := lockCatalogMigrationContractForApply(tx, batch.Id, item)
		if err != nil {
			return err
		}
		_ = targetSnapshot
		intent, err := createCatalogMigrationIntentTx(tx, batch.Id, contract, item, targetSnapshotJSON, model.SubscriptionChangeIntentStatusScheduled, 0)
		if err != nil {
			return err
		}
		return scheduleCatalogMigrationContractTx(tx, contract, intent)
	})
}

func (s CatalogMigrationService) processCatalogMigrationPreview(ctx context.Context, batch *model.SubscriptionCatalogMigrationBatch, preview CatalogMigrationPreviewResult) error {
	for _, item := range preview.Contracts {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !item.Eligible {
			continue
		}
		var intents []model.SubscriptionChangeIntent
		if err := s.DB.WithContext(ctx).
			Where("catalog_migration_batch_id = ? AND contract_id = ?", batch.Id, item.ContractID).
			Order("id desc").Find(&intents).Error; err != nil {
			return err
		}
		if len(intents) != 0 {
			intent := intents[0]
			if item.PaymentMode == model.SubscriptionPaymentModeStripeRecurring &&
				(intent.Status == model.SubscriptionChangeIntentStatusSyncing || intent.Status == model.SubscriptionChangeIntentStatusCompensationRequired) {
				_ = s.resumeStripeIntent(ctx, batch, item, &intent)
			}
			continue
		}
		if item.PaymentMode == model.SubscriptionPaymentModeStripeRecurring {
			_ = s.applyStripeContract(ctx, batch, item)
		} else {
			_ = s.applyWalletContract(ctx, batch, item)
		}
	}
	return s.refreshBatchSummary(ctx, batch.Id)
}

func (s CatalogMigrationService) catalogMigrationHasActionable(ctx context.Context, batchID string, preview CatalogMigrationPreviewResult) (bool, error) {
	var intents []model.SubscriptionChangeIntent
	if err := s.DB.WithContext(ctx).Where("catalog_migration_batch_id = ?", batchID).Find(&intents).Error; err != nil {
		return false, err
	}
	byContract := make(map[int64]model.SubscriptionChangeIntent, len(intents))
	for _, intent := range intents {
		byContract[intent.ContractId] = intent
	}
	for _, item := range preview.Contracts {
		if !item.Eligible {
			continue
		}
		intent, exists := byContract[item.ContractID]
		if !exists || intent.Status == model.SubscriptionChangeIntentStatusSyncing || intent.Status == model.SubscriptionChangeIntentStatusCompensationRequired {
			return true, nil
		}
	}
	return false, nil
}

type preparedStripeCatalogMigration struct {
	Intent      model.SubscriptionChangeIntent
	Contract    model.UserSubscriptionContract
	Binding     model.SubscriptionProviderBinding
	Reservation *model.SubscriptionProviderLifecycleReservation
	Request     CatalogMigrationProviderScheduleRequest
}

func (s CatalogMigrationService) applyStripeContract(ctx context.Context, batch *model.SubscriptionCatalogMigrationBatch, item CatalogMigrationContractPreviewResult) error {
	prepared, err := s.prepareStripeContract(ctx, batch, item)
	if err != nil {
		return err
	}
	if s.Scheduler == nil {
		err = errors.New("catalog migration provider scheduler is unavailable")
		return errors.Join(err, s.markPreparedStripeFailure(ctx, prepared, err, model.SubscriptionChangeIntentStatusNeedsAttention))
	}
	result, err := s.Scheduler.ScheduleCatalogMigration(ctx, prepared.Request)
	if err != nil {
		return errors.Join(err, s.markPreparedStripeFailure(ctx, prepared, err, model.SubscriptionChangeIntentStatusCompensationRequired))
	}
	if validateCatalogMigrationScheduleResult(prepared.Request, result) != nil {
		err = validateCatalogMigrationScheduleResult(prepared.Request, result)
		return errors.Join(err, s.markPreparedStripeFailure(ctx, prepared, err, model.SubscriptionChangeIntentStatusNeedsAttention))
	}
	if s.AfterProviderSchedule != nil {
		err = s.AfterProviderSchedule(result)
	} else {
		err = s.finalizeStripeContract(ctx, prepared, result)
	}
	if err == nil {
		return nil
	}
	if persistErr := s.markPreparedStripeFailure(ctx, prepared, err, model.SubscriptionChangeIntentStatusCompensationRequired); persistErr != nil {
		return errors.Join(err, persistErr)
	}
	restoreRequest := prepared.Request
	restoreRequest.PreviousScheduleSnapshot = result.PreviousScheduleSnapshot
	if restoreErr := s.Scheduler.RestoreCatalogMigration(ctx, restoreRequest); restoreErr != nil {
		combined := fmt.Errorf("%v; provider restore failed: %w", err, restoreErr)
		return errors.Join(err, s.markPreparedStripeFailure(ctx, prepared, combined, model.SubscriptionChangeIntentStatusNeedsAttention))
	}
	return err
}

func (s CatalogMigrationService) prepareStripeContract(ctx context.Context, batch *model.SubscriptionCatalogMigrationBatch, item CatalogMigrationContractPreviewResult) (*preparedStripeCatalogMigration, error) {
	var prepared preparedStripeCatalogMigration
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var binding model.SubscriptionProviderBinding
		if err := subscriptionCommandLock(tx).Where("id = ?", itemBindingIDFromPreview(tx, item.ContractID)).First(&binding).Error; err != nil {
			return err
		}
		contract, _, targetSnapshotJSON, err := lockCatalogMigrationContractForApply(tx, batch.Id, item)
		if err != nil {
			return err
		}
		if binding.Id != contract.CurrentProviderBindingId || binding.ContractId != contract.Id || binding.UserId != contract.UserId || binding.PlanId != contract.CurrentPlanId {
			return model.ErrSubscriptionProviderBindingConflict
		}
		if fingerprintCatalogMigrationBinding(binding) != item.BindingFingerprint {
			return model.ErrSubscriptionProviderBindingConflict
		}
		if !catalogMigrationHasTestStripeCredentials(s.Sandbox) {
			return errors.New("catalog migration requires Stripe test credentials")
		}
		if err := freezeCatalogMigrationBindingCurrentSnapshotTx(tx, &binding, item); err != nil {
			return err
		}
		reservationToken := stableCatalogMigrationKey("reserve", batch.RequestId, contract.Id, contract.ChangeVersion)
		reservation, reservedBinding, err := model.ReserveSubscriptionProviderLifecycleExactTx(tx, &binding, model.SubscriptionProviderLifecycleActionCatalogMigration, reservationToken, catalogMigrationLifecycleReservationTTLSeconds)
		if err != nil {
			return err
		}
		intent, err := createCatalogMigrationIntentTx(tx, batch.Id, contract, item, targetSnapshotJSON, model.SubscriptionChangeIntentStatusSyncing, binding.Id)
		if err != nil {
			return err
		}
		if err := setCatalogMigrationLatestIntentTx(tx, contract, intent.Id); err != nil {
			return err
		}
		var target model.SubscriptionPlan
		if err := tx.Where("id = ?", item.TargetPlanID).First(&target).Error; err != nil {
			return err
		}
		fingerprint := stableCatalogMigrationKey("ownership", batch.Id, contract.Id, intent.Id, contract.ChangeVersion, binding.ProviderSubscriptionId, binding.ProviderSubscriptionItemId, binding.ProviderPriceId, target.StripePriceId, item.EffectiveAt)
		providerKey := stableCatalogMigrationKey("provider", batch.RequestId, contract.Id, item.SourcePlanID, item.TargetPlanID, contract.ChangeVersion)
		if err := tx.Model(intent).Updates(map[string]any{
			"provider_idempotency_key":      providerKey,
			"provider_schedule_fingerprint": fingerprint,
		}).Error; err != nil {
			return err
		}
		prepared = preparedStripeCatalogMigration{
			Intent: *intent, Contract: *contract, Binding: *reservedBinding, Reservation: reservation,
			Request: CatalogMigrationProviderScheduleRequest{
				BatchID: batch.Id, IntentID: intent.Id, ContractID: contract.Id, UserID: contract.UserId,
				ChangeVersion: contract.ChangeVersion, BindingID: binding.Id,
				ProviderSubscriptionID: binding.ProviderSubscriptionId, ProviderSubscriptionItemID: binding.ProviderSubscriptionItemId,
				ProviderCustomerID: binding.ProviderCustomerId, CurrentPriceID: binding.ProviderPriceId, TargetPriceID: target.StripePriceId,
				CurrentPeriodStart: contract.CurrentPeriodStart, CurrentPeriodEnd: contract.CurrentPeriodEnd,
				IdempotencyKey: providerKey, ExpectedOwnershipFingerprint: fingerprint, ExpectedProviderFingerprint: item.ProviderFingerprint,
				CurrentPlanSnapshot: item.CurrentPlanSnapshot, TargetPlanSnapshot: item.TargetPlanSnapshot,
				LifecycleActionSeq: reservation.LifecycleActionSeq, LifecycleReservationToken: reservation.Token, LifecycleReservationUntil: reservation.ExpiresAt,
			},
		}
		prepared.Intent.ProviderIdempotencyKey = providerKey
		prepared.Intent.ProviderScheduleFingerprint = fingerprint
		return nil
	})
	return &prepared, err
}

func (s CatalogMigrationService) resumeStripeIntent(ctx context.Context, batch *model.SubscriptionCatalogMigrationBatch, item CatalogMigrationContractPreviewResult, expected *model.SubscriptionChangeIntent) error {
	if s.Scheduler == nil || !catalogMigrationHasTestStripeCredentials(s.Sandbox) {
		return errors.New("catalog migration provider reconciliation is unavailable")
	}
	var prepared preparedStripeCatalogMigration
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var binding model.SubscriptionProviderBinding
		if err := subscriptionCommandLock(tx).Where("id = ?", expected.ProviderBindingId).First(&binding).Error; err != nil {
			return err
		}
		var contract model.UserSubscriptionContract
		if err := subscriptionCommandLock(tx).Where("id = ?", expected.ContractId).First(&contract).Error; err != nil {
			return err
		}
		var intent model.SubscriptionChangeIntent
		if err := subscriptionCommandLock(tx).Where("id = ?", expected.Id).First(&intent).Error; err != nil {
			return err
		}
		if intent.CatalogMigrationBatchId == nil || *intent.CatalogMigrationBatchId != batch.Id || contract.LatestChangeIntentId != intent.Id ||
			(intent.Status != model.SubscriptionChangeIntentStatusSyncing && intent.Status != model.SubscriptionChangeIntentStatusCompensationRequired) ||
			strings.TrimSpace(intent.ProviderIdempotencyKey) == "" || strings.TrimSpace(intent.ProviderScheduleFingerprint) == "" {
			return ErrSubscriptionChangeInProgress
		}
		if strings.TrimSpace(binding.LifecycleReservationToken) == "" || binding.LifecycleReservationAction != model.SubscriptionProviderLifecycleActionCatalogMigration {
			return model.ErrSubscriptionProviderLifecycleConflict
		}
		reservation, reservedBinding, err := model.ReserveSubscriptionProviderLifecycleExactTx(tx, &binding, model.SubscriptionProviderLifecycleActionCatalogMigration, binding.LifecycleReservationToken, catalogMigrationLifecycleReservationTTLSeconds)
		if err != nil {
			return err
		}
		binding = *reservedBinding
		target, err := DecodeRecurringPlanSnapshotV1(intent.TargetPlanSnapshot)
		if err != nil {
			return err
		}
		prepared = preparedStripeCatalogMigration{Intent: intent, Contract: contract, Binding: binding, Reservation: reservation}
		prepared.Contract.ChangeVersion = intent.ChangeVersion
		prepared.Request = CatalogMigrationProviderScheduleRequest{BatchID: batch.Id, IntentID: intent.Id, ContractID: contract.Id, UserID: contract.UserId, ChangeVersion: intent.ChangeVersion, BindingID: binding.Id, ProviderSubscriptionID: binding.ProviderSubscriptionId, ProviderSubscriptionItemID: binding.ProviderSubscriptionItemId, ProviderCustomerID: binding.ProviderCustomerId, CurrentPriceID: binding.ProviderPriceId, TargetPriceID: target.StripePriceID, CurrentPeriodStart: contract.CurrentPeriodStart, CurrentPeriodEnd: intent.EffectiveAt, IdempotencyKey: intent.ProviderIdempotencyKey, ExpectedOwnershipFingerprint: intent.ProviderScheduleFingerprint, ExpectedProviderFingerprint: item.ProviderFingerprint, CurrentPlanSnapshot: item.CurrentPlanSnapshot, TargetPlanSnapshot: item.TargetPlanSnapshot, LifecycleActionSeq: reservation.LifecycleActionSeq, LifecycleReservationToken: reservation.Token, LifecycleReservationUntil: reservation.ExpiresAt, PreviousScheduleSnapshot: intent.PreviousScheduleSnapshot}
		return nil
	})
	if err != nil {
		return err
	}
	result, err := s.Scheduler.ScheduleCatalogMigration(ctx, prepared.Request)
	if err != nil {
		return errors.Join(err, s.markPreparedStripeFailure(ctx, &prepared, err, model.SubscriptionChangeIntentStatusCompensationRequired))
	}
	if err := validateCatalogMigrationScheduleResult(prepared.Request, result); err != nil {
		return errors.Join(err, s.markPreparedStripeFailure(ctx, &prepared, err, model.SubscriptionChangeIntentStatusNeedsAttention))
	}
	return s.finalizeStripeContract(ctx, &prepared, result)
}

func itemBindingIDFromPreview(tx *gorm.DB, contractID int64) int64 {
	var contract model.UserSubscriptionContract
	if tx.Where("id = ?", contractID).Select("current_provider_binding_id").First(&contract).Error != nil {
		return 0
	}
	return contract.CurrentProviderBindingId
}

func (s CatalogMigrationService) finalizeStripeContract(ctx context.Context, prepared *preparedStripeCatalogMigration, result CatalogMigrationProviderScheduleResult) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var binding model.SubscriptionProviderBinding
		if err := subscriptionCommandLock(tx).Where("id = ?", prepared.Binding.Id).First(&binding).Error; err != nil {
			return err
		}
		if !catalogMigrationReservationMatches(&binding, prepared.Reservation) {
			return model.ErrSubscriptionProviderLifecycleConflict
		}
		var contract model.UserSubscriptionContract
		if err := subscriptionCommandLock(tx).Where("id = ?", prepared.Contract.Id).First(&contract).Error; err != nil {
			return err
		}
		if contract.ChangeVersion != prepared.Contract.ChangeVersion+1 || contract.LatestChangeIntentId != prepared.Intent.Id || contract.PendingPlanId != 0 || contract.PendingEffectiveAt != 0 {
			return ErrSubscriptionChangeInProgress
		}
		intentUpdate := tx.Model(&model.SubscriptionChangeIntent{}).
			Where("id = ? AND status IN ? AND provider_schedule_fingerprint = ?", prepared.Intent.Id, []string{model.SubscriptionChangeIntentStatusSyncing, model.SubscriptionChangeIntentStatusCompensationRequired}, prepared.Request.ExpectedOwnershipFingerprint).
			Updates(map[string]any{
				"status": model.SubscriptionChangeIntentStatusScheduled, "provider_schedule_id": strings.TrimSpace(result.ScheduleID),
				"provider_schedule_fingerprint": strings.TrimSpace(result.OwnershipFingerprint), "previous_schedule_snapshot": result.PreviousScheduleSnapshot,
				"last_error": "", "updated_at": common.GetTimestamp(),
			})
		if intentUpdate.Error != nil || intentUpdate.RowsAffected != 1 {
			return firstError(intentUpdate.Error, ErrSubscriptionChangeInProgress)
		}
		contractUpdate := tx.Model(&model.UserSubscriptionContract{}).
			Where("id = ? AND change_version = ? AND latest_change_intent_id = ? AND pending_plan_id = ? AND pending_effective_at = ?", contract.Id, contract.ChangeVersion, prepared.Intent.Id, 0, 0).
			Updates(map[string]any{"pending_plan_id": prepared.Intent.ToPlanId, "pending_effective_at": prepared.Intent.EffectiveAt, "change_version": gorm.Expr("change_version + 1"), "updated_at": common.GetTimestamp()})
		if contractUpdate.Error != nil || contractUpdate.RowsAffected != 1 {
			return firstError(contractUpdate.Error, ErrSubscriptionChangeInProgress)
		}
		bindingUpdate := tx.Model(&model.SubscriptionProviderBinding{}).
			Where("id = ? AND lifecycle_action_seq = ? AND lifecycle_reservation_token = ? AND lifecycle_reservation_action = ? AND lifecycle_reservation_until = ?", binding.Id, prepared.Reservation.LifecycleActionSeq, prepared.Reservation.Token, prepared.Reservation.Action, prepared.Reservation.ExpiresAt).
			Updates(map[string]any{"provider_schedule_id": strings.TrimSpace(result.ScheduleID), "lifecycle_action_seq": gorm.Expr("lifecycle_action_seq + 1"), "lifecycle_reservation_token": "", "lifecycle_reservation_action": "", "lifecycle_reservation_until": 0, "updated_at": common.GetTimestamp()})
		if bindingUpdate.Error != nil || bindingUpdate.RowsAffected != 1 {
			return firstError(bindingUpdate.Error, model.ErrSubscriptionProviderLifecycleConflict)
		}
		return nil
	})
}

func lockCatalogMigrationContractForApply(tx *gorm.DB, batchID string, item CatalogMigrationContractPreviewResult) (*model.UserSubscriptionContract, RecurringPlanSnapshotV1, string, error) {
	var contract model.UserSubscriptionContract
	if err := subscriptionCommandLock(tx).Where("id = ?", item.ContractID).First(&contract).Error; err != nil {
		return nil, RecurringPlanSnapshotV1{}, "", err
	}
	if contract.ChangeVersion != item.ExpectedChangeVersion || fingerprintCatalogMigrationContract(contract) != item.ContractFingerprint || contract.CurrentPlanId != item.SourcePlanID || contract.CurrentPeriodEnd != item.EffectiveAt || contract.PendingPlanId != 0 || contract.PendingEffectiveAt != 0 || contract.LatestChangeIntentId != item.PriorLatestChangeIntentID || contract.Status != model.SubscriptionContractStatusActive || contract.RenewalStatus != model.SubscriptionRenewalStatusEnabled {
		return nil, RecurringPlanSnapshotV1{}, "", ErrSubscriptionChangeInProgress
	}
	if err := validatePriorCatalogMigrationIntentTx(tx, &contract); err != nil {
		return nil, RecurringPlanSnapshotV1{}, "", err
	}
	var entitlement model.UserSubscription
	if err := subscriptionCommandLock(tx).Where("id = ?", contract.CurrentEntitlementId).First(&entitlement).Error; err != nil || fingerprintJSON(entitlement) != item.EntitlementFingerprint || !catalogMigrationPaymentShapeMatches(contract, entitlement) {
		return nil, RecurringPlanSnapshotV1{}, "", errors.New("catalog migration entitlement facts changed")
	}
	var source, target model.SubscriptionPlan
	if err := tx.Where("id = ?", item.SourcePlanID).First(&source).Error; err != nil || fingerprintPlan(source) != item.SourcePlanFingerprint {
		return nil, RecurringPlanSnapshotV1{}, "", errors.New("catalog migration source plan facts changed")
	}
	if err := tx.Where("id = ? AND enabled = ?", item.TargetPlanID, true).First(&target).Error; err != nil {
		return nil, RecurringPlanSnapshotV1{}, "", err
	}
	if fingerprintPlan(target) != item.TargetPlanFingerprint {
		return nil, RecurringPlanSnapshotV1{}, "", errors.New("catalog migration target plan facts changed")
	}
	targetSnapshot, err := DecodeRecurringPlanSnapshotV1(item.TargetPlanSnapshot)
	if err != nil {
		return nil, RecurringPlanSnapshotV1{}, "", err
	}
	if err := ValidateRecurringPlanSnapshotV1AgainstPlan(targetSnapshot, &target); err != nil {
		return nil, RecurringPlanSnapshotV1{}, "", err
	}
	targetHash, err := RecurringPlanSnapshotV1Fingerprint(targetSnapshot)
	if err != nil || targetHash != item.TargetSnapshotHash {
		return nil, RecurringPlanSnapshotV1{}, "", errors.New("catalog migration target snapshot changed")
	}
	return &contract, targetSnapshot, item.TargetPlanSnapshot, nil
}

func validatePriorCatalogMigrationIntentTx(tx *gorm.DB, contract *model.UserSubscriptionContract) error {
	if contract.LatestChangeIntentId == 0 {
		return nil
	}
	var prior model.SubscriptionChangeIntent
	if err := tx.Where("id = ? AND contract_id = ? AND user_id = ?", contract.LatestChangeIntentId, contract.Id, contract.UserId).First(&prior).Error; err != nil {
		return ErrSubscriptionChangeInProgress
	}
	switch prior.Status {
	case model.SubscriptionChangeIntentStatusApplied,
		model.SubscriptionChangeIntentStatusFailed,
		model.SubscriptionChangeIntentStatusExpired,
		model.SubscriptionChangeIntentStatusSuperseded:
		return nil
	default:
		return ErrSubscriptionChangeInProgress
	}
}

func freezeCatalogMigrationBindingCurrentSnapshotTx(tx *gorm.DB, binding *model.SubscriptionProviderBinding, item CatalogMigrationContractPreviewResult) error {
	if tx == nil || binding == nil || strings.TrimSpace(item.CurrentPlanSnapshot) == "" {
		return errors.New("catalog migration current plan snapshot is missing")
	}
	snapshot, err := DecodeRecurringPlanSnapshotV1(item.CurrentPlanSnapshot)
	if err != nil {
		return err
	}
	fingerprint, err := RecurringPlanSnapshotV1Fingerprint(snapshot)
	if err != nil || fingerprint != item.CurrentSnapshotHash || snapshot.PlanID != binding.PlanId || snapshot.StripePriceID != strings.TrimSpace(binding.ProviderPriceId) {
		return errors.New("catalog migration current plan snapshot changed")
	}
	if strings.TrimSpace(binding.CurrentPlanSnapshot) != "" {
		if binding.CurrentPlanSnapshot != item.CurrentPlanSnapshot {
			return errors.New("catalog migration binding current plan snapshot mismatch")
		}
		return nil
	}
	update := tx.Model(&model.SubscriptionProviderBinding{}).
		Where("id = ? AND current_plan_snapshot = ?", binding.Id, "").
		Update("current_plan_snapshot", item.CurrentPlanSnapshot)
	if update.Error != nil || update.RowsAffected != 1 {
		return firstError(update.Error, model.ErrSubscriptionProviderBindingConflict)
	}
	binding.CurrentPlanSnapshot = item.CurrentPlanSnapshot
	return nil
}

func createCatalogMigrationIntentTx(tx *gorm.DB, batchID string, contract *model.UserSubscriptionContract, item CatalogMigrationContractPreviewResult, targetSnapshot string, status string, bindingID int64) (*model.SubscriptionChangeIntent, error) {
	requestID := stableCatalogMigrationKey("intent", batchID, contract.Id, item.SourcePlanID, item.TargetPlanID, contract.ChangeVersion)
	batchIDCopy := batchID
	intent := &model.SubscriptionChangeIntent{
		ContractId: contract.Id, UserId: contract.UserId, RequestId: requestID, ChangeVersion: contract.ChangeVersion,
		Kind: model.SubscriptionChangeIntentKindCatalogMigration, PaymentMode: contract.PaymentMode, Status: status,
		FromPlanId: item.SourcePlanID, ToPlanId: item.TargetPlanID, ProviderBindingId: bindingID,
		CatalogMigrationBatchId: &batchIDCopy, TargetPlanSnapshot: targetSnapshot, EffectiveAt: item.EffectiveAt,
		PreviousChangeIntentId: item.PriorLatestChangeIntentID,
	}
	if err := tx.Create(intent).Error; err != nil {
		return nil, err
	}
	return intent, nil
}

func scheduleCatalogMigrationContractTx(tx *gorm.DB, contract *model.UserSubscriptionContract, intent *model.SubscriptionChangeIntent) error {
	update := tx.Model(&model.UserSubscriptionContract{}).
		Where("id = ? AND change_version = ? AND latest_change_intent_id = ? AND pending_plan_id = ? AND pending_effective_at = ?", contract.Id, contract.ChangeVersion, contract.LatestChangeIntentId, 0, 0).
		Updates(map[string]any{"latest_change_intent_id": intent.Id, "pending_plan_id": intent.ToPlanId, "pending_effective_at": intent.EffectiveAt, "change_version": gorm.Expr("change_version + 1"), "updated_at": common.GetTimestamp()})
	if update.Error != nil || update.RowsAffected != 1 {
		return firstError(update.Error, ErrSubscriptionChangeInProgress)
	}
	return nil
}

func setCatalogMigrationLatestIntentTx(tx *gorm.DB, contract *model.UserSubscriptionContract, intentID int64) error {
	update := tx.Model(&model.UserSubscriptionContract{}).
		Where("id = ? AND change_version = ? AND latest_change_intent_id = ? AND pending_plan_id = ? AND pending_effective_at = ?", contract.Id, contract.ChangeVersion, contract.LatestChangeIntentId, 0, 0).
		Updates(map[string]any{"latest_change_intent_id": intentID, "change_version": gorm.Expr("change_version + 1"), "updated_at": common.GetTimestamp()})
	if update.Error != nil || update.RowsAffected != 1 {
		return firstError(update.Error, ErrSubscriptionChangeInProgress)
	}
	return nil
}

func (s CatalogMigrationService) markPreparedStripeFailure(ctx context.Context, prepared *preparedStripeCatalogMigration, cause error, status string) error {
	message := "catalog migration provider operation failed"
	if cause != nil {
		message = cause.Error()
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		intentUpdate := tx.Model(&model.SubscriptionChangeIntent{}).Where("id = ? AND status IN ?", prepared.Intent.Id, []string{model.SubscriptionChangeIntentStatusSyncing, model.SubscriptionChangeIntentStatusCompensationRequired, model.SubscriptionChangeIntentStatusNeedsAttention}).Updates(map[string]any{"status": status, "last_error": message, "updated_at": common.GetTimestamp()})
		if intentUpdate.Error != nil {
			return intentUpdate.Error
		}
		if intentUpdate.RowsAffected != 1 {
			return ErrSubscriptionChangeInProgress
		}
		contractUpdate := tx.Model(&model.UserSubscriptionContract{}).Where("id = ? AND latest_change_intent_id = ?", prepared.Contract.Id, prepared.Intent.Id).Updates(map[string]any{"status": conditionalNeedsAttentionStatus(status), "updated_at": common.GetTimestamp()})
		if contractUpdate.Error != nil {
			return contractUpdate.Error
		}
		if contractUpdate.RowsAffected != 1 {
			return ErrSubscriptionChangeInProgress
		}
		return nil
	})
}

func conditionalNeedsAttentionStatus(intentStatus string) string {
	if intentStatus == model.SubscriptionChangeIntentStatusNeedsAttention {
		return model.SubscriptionContractStatusNeedsAttention
	}
	return model.SubscriptionContractStatusActive
}

func (s CatalogMigrationService) cancelLocalIntent(ctx context.Context, intent *model.SubscriptionChangeIntent) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var contract model.UserSubscriptionContract
		if err := subscriptionCommandLock(tx).Where("id = ?", intent.ContractId).First(&contract).Error; err != nil {
			return err
		}
		if contract.LatestChangeIntentId != intent.Id || !catalogMigrationPendingStateCanClear(contract, intent) {
			return ErrSubscriptionChangeInProgress
		}
		update := tx.Model(&model.SubscriptionChangeIntent{}).Where("id = ? AND status IN ?", intent.Id, cancellableCatalogMigrationIntentStatuses()).Updates(map[string]any{"status": model.SubscriptionChangeIntentStatusSuperseded, "last_error": "", "updated_at": common.GetTimestamp()})
		if update.Error != nil || update.RowsAffected != 1 {
			return firstError(update.Error, ErrSubscriptionChangeInProgress)
		}
		return clearCatalogMigrationPendingTx(tx, &contract, intent)
	})
}

func (s CatalogMigrationService) cancelStripeIntent(ctx context.Context, batch *model.SubscriptionCatalogMigrationBatch, intent *model.SubscriptionChangeIntent) error {
	if s.Scheduler == nil {
		restoreErr := errors.New("catalog migration provider restore is unavailable")
		return errors.Join(restoreErr, s.markIntentNeedsAttention(ctx, intent.Id, restoreErr.Error()))
	}
	var request CatalogMigrationProviderScheduleRequest
	var reservation *model.SubscriptionProviderLifecycleReservation
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var binding model.SubscriptionProviderBinding
		if err := subscriptionCommandLock(tx).Where("id = ?", intent.ProviderBindingId).First(&binding).Error; err != nil {
			return err
		}
		var contract model.UserSubscriptionContract
		if err := subscriptionCommandLock(tx).Where("id = ?", intent.ContractId).First(&contract).Error; err != nil {
			return err
		}
		var lockedIntent model.SubscriptionChangeIntent
		if err := subscriptionCommandLock(tx).Where("id = ?", intent.Id).First(&lockedIntent).Error; err != nil {
			return err
		}
		intent = &lockedIntent
		if contract.LatestChangeIntentId != intent.Id || (strings.TrimSpace(binding.ProviderScheduleId) != "" && strings.TrimSpace(intent.ProviderScheduleId) != "" && binding.ProviderScheduleId != intent.ProviderScheduleId) || strings.TrimSpace(intent.ProviderScheduleFingerprint) == "" || !catalogMigrationPendingStateCanClear(contract, intent) {
			return ErrSubscriptionChangeInProgress
		}
		if !catalogMigrationHasTestStripeCredentials(s.Sandbox) {
			return errors.New("catalog migration requires Stripe test credentials")
		}
		var targetSnapshot RecurringPlanSnapshotV1
		targetSnapshot, err := DecodeRecurringPlanSnapshotV1(intent.TargetPlanSnapshot)
		if err != nil {
			return err
		}
		token := stableCatalogMigrationKey("cancel", batch.Id, contract.Id, intent.Id, contract.ChangeVersion)
		reservation, _, err = model.ReserveSubscriptionProviderLifecycleExactTx(tx, &binding, model.SubscriptionProviderLifecycleActionCatalogMigration, token, catalogMigrationLifecycleReservationTTLSeconds)
		if err != nil {
			return err
		}
		request = CatalogMigrationProviderScheduleRequest{BatchID: batch.Id, IntentID: intent.Id, ContractID: contract.Id, UserID: contract.UserId, ChangeVersion: contract.ChangeVersion, BindingID: binding.Id, ProviderSubscriptionID: binding.ProviderSubscriptionId, ProviderSubscriptionItemID: binding.ProviderSubscriptionItemId, ProviderCustomerID: binding.ProviderCustomerId, CurrentPriceID: binding.ProviderPriceId, TargetPriceID: targetSnapshot.StripePriceID, CurrentPeriodStart: contract.CurrentPeriodStart, CurrentPeriodEnd: contract.CurrentPeriodEnd, IdempotencyKey: intent.ProviderIdempotencyKey, ExpectedOwnershipFingerprint: intent.ProviderScheduleFingerprint, CurrentPlanSnapshot: binding.CurrentPlanSnapshot, TargetPlanSnapshot: intent.TargetPlanSnapshot, LifecycleActionSeq: reservation.LifecycleActionSeq, LifecycleReservationToken: reservation.Token, LifecycleReservationUntil: reservation.ExpiresAt, PreviousScheduleSnapshot: intent.PreviousScheduleSnapshot}
		return nil
	})
	if err != nil {
		return err
	}
	if err := s.Scheduler.RestoreCatalogMigration(ctx, request); err != nil {
		return errors.Join(err, s.markIntentNeedsAttention(ctx, intent.Id, err.Error()))
	}
	if s.AfterProviderRestore != nil {
		if err := s.AfterProviderRestore(); err != nil {
			_ = s.markIntentNeedsAttention(ctx, intent.Id, "catalog migration provider restore requires reconciliation")
			return err
		}
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var binding model.SubscriptionProviderBinding
		if err := subscriptionCommandLock(tx).Where("id = ?", intent.ProviderBindingId).First(&binding).Error; err != nil {
			return err
		}
		var contract model.UserSubscriptionContract
		if err := subscriptionCommandLock(tx).Where("id = ?", intent.ContractId).First(&contract).Error; err != nil {
			return err
		}
		var lockedIntent model.SubscriptionChangeIntent
		if err := subscriptionCommandLock(tx).Where("id = ?", intent.Id).First(&lockedIntent).Error; err != nil {
			return err
		}
		intent = &lockedIntent
		if !catalogMigrationReservationMatches(&binding, reservation) || contract.LatestChangeIntentId != intent.Id {
			return model.ErrSubscriptionProviderLifecycleConflict
		}
		intentUpdate := tx.Model(&model.SubscriptionChangeIntent{}).Where("id = ? AND status IN ?", intent.Id, cancellableCatalogMigrationIntentStatuses()).Updates(map[string]any{"status": model.SubscriptionChangeIntentStatusSuperseded, "provider_schedule_id": "", "last_error": "", "updated_at": common.GetTimestamp()})
		if intentUpdate.Error != nil || intentUpdate.RowsAffected != 1 {
			return firstError(intentUpdate.Error, ErrSubscriptionChangeInProgress)
		}
		if err := clearCatalogMigrationPendingTx(tx, &contract, intent); err != nil {
			return err
		}
		update := tx.Model(&model.SubscriptionProviderBinding{}).Where("id = ? AND lifecycle_action_seq = ? AND lifecycle_reservation_token = ?", binding.Id, reservation.LifecycleActionSeq, reservation.Token).Updates(map[string]any{"provider_schedule_id": "", "lifecycle_action_seq": gorm.Expr("lifecycle_action_seq + 1"), "lifecycle_reservation_token": "", "lifecycle_reservation_action": "", "lifecycle_reservation_until": 0, "updated_at": common.GetTimestamp()})
		if update.Error != nil || update.RowsAffected != 1 {
			return firstError(update.Error, model.ErrSubscriptionProviderLifecycleConflict)
		}
		return nil
	})
}

func clearCatalogMigrationPendingTx(tx *gorm.DB, contract *model.UserSubscriptionContract, intent *model.SubscriptionChangeIntent) error {
	if !catalogMigrationPendingStateCanClear(*contract, intent) {
		return ErrSubscriptionChangeInProgress
	}
	update := tx.Model(&model.UserSubscriptionContract{}).Where("id = ? AND change_version = ? AND latest_change_intent_id = ? AND pending_plan_id = ? AND pending_effective_at = ?", contract.Id, contract.ChangeVersion, intent.Id, contract.PendingPlanId, contract.PendingEffectiveAt).Updates(map[string]any{"latest_change_intent_id": intent.PreviousChangeIntentId, "pending_plan_id": 0, "pending_effective_at": 0, "status": model.SubscriptionContractStatusActive, "change_version": gorm.Expr("change_version + 1"), "updated_at": common.GetTimestamp()})
	if update.Error != nil || update.RowsAffected != 1 {
		return firstError(update.Error, ErrSubscriptionChangeInProgress)
	}
	return nil
}

func catalogMigrationPendingStateCanClear(contract model.UserSubscriptionContract, intent *model.SubscriptionChangeIntent) bool {
	return intent != nil && ((contract.PendingPlanId == intent.ToPlanId && contract.PendingEffectiveAt == intent.EffectiveAt) || (contract.PendingPlanId == 0 && contract.PendingEffectiveAt == 0))
}

func cancellableCatalogMigrationIntentStatuses() []string {
	return []string{model.SubscriptionChangeIntentStatusScheduled, model.SubscriptionChangeIntentStatusSyncing, model.SubscriptionChangeIntentStatusCompensationRequired, model.SubscriptionChangeIntentStatusNeedsAttention}
}

func (s CatalogMigrationService) markIntentNeedsAttention(ctx context.Context, intentID int64, message string) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var intent model.SubscriptionChangeIntent
		if err := subscriptionCommandLock(tx).Where("id = ?", intentID).First(&intent).Error; err != nil {
			return err
		}
		if intent.Status == model.SubscriptionChangeIntentStatusApplied || intent.Status == model.SubscriptionChangeIntentStatusSuperseded {
			return nil
		}
		intentUpdate := tx.Model(&intent).Where("status NOT IN ?", []string{model.SubscriptionChangeIntentStatusApplied, model.SubscriptionChangeIntentStatusSuperseded}).Updates(map[string]any{"status": model.SubscriptionChangeIntentStatusNeedsAttention, "last_error": strings.TrimSpace(message), "updated_at": common.GetTimestamp()})
		if intentUpdate.Error != nil || intentUpdate.RowsAffected != 1 {
			return firstError(intentUpdate.Error, ErrSubscriptionChangeInProgress)
		}
		return tx.Model(&model.UserSubscriptionContract{}).Where("id = ? AND latest_change_intent_id = ?", intent.ContractId, intent.Id).Updates(map[string]any{"status": model.SubscriptionContractStatusNeedsAttention, "updated_at": common.GetTimestamp()}).Error
	})
}

func (s CatalogMigrationService) refreshBatchSummary(ctx context.Context, batchID string) error {
	result, err := s.Get(ctx, batchID)
	if err != nil {
		return err
	}
	status := catalogMigrationBatchStatus(result.Summary)
	summaryJSON, err := json.Marshal(result.Summary)
	if err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Model(&model.SubscriptionCatalogMigrationBatch{}).Where("id = ?", batchID).Updates(map[string]any{"status": status, "summary_snapshot": string(summaryJSON), "updated_at": common.GetTimestamp()}).Error
}

func findCatalogMigrationBatchByRequest(ctx context.Context, db *gorm.DB, requestID string) (model.SubscriptionCatalogMigrationBatch, bool, error) {
	var batch model.SubscriptionCatalogMigrationBatch
	err := db.WithContext(ctx).Where("request_id = ?", strings.TrimSpace(requestID)).First(&batch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return batch, false, nil
	}
	return batch, err == nil, err
}

func encodeCatalogMigrationStoredManifest(preview CatalogMigrationPreviewResult) (string, error) {
	stored := catalogMigrationStoredManifest{Version: 1, Preview: preview, Snapshots: make([]catalogMigrationStoredContractSnapshot, 0, len(preview.Contracts))}
	for _, item := range preview.Contracts {
		if !item.Eligible {
			continue
		}
		if strings.TrimSpace(item.CurrentPlanSnapshot) == "" || strings.TrimSpace(item.TargetPlanSnapshot) == "" {
			return "", errors.New("catalog migration eligible preview snapshots are missing")
		}
		stored.Snapshots = append(stored.Snapshots, catalogMigrationStoredContractSnapshot{ContractID: item.ContractID, CurrentPlanSnapshot: item.CurrentPlanSnapshot, TargetPlanSnapshot: item.TargetPlanSnapshot})
	}
	payload, err := json.Marshal(stored)
	if err != nil {
		return "", fmt.Errorf("encode catalog migration manifest: %w", err)
	}
	return string(payload), nil
}

func decodeCatalogMigrationStoredManifest(raw string) (CatalogMigrationPreviewResult, error) {
	var stored catalogMigrationStoredManifest
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return CatalogMigrationPreviewResult{}, fmt.Errorf("decode catalog migration manifest: %w", err)
	}
	if stored.Version != 1 || strings.TrimSpace(stored.Preview.RequestID) == "" {
		return CatalogMigrationPreviewResult{}, errors.New("catalog migration manifest version is invalid")
	}
	snapshots := make(map[int64]catalogMigrationStoredContractSnapshot, len(stored.Snapshots))
	for _, snapshot := range stored.Snapshots {
		if snapshot.ContractID <= 0 {
			return CatalogMigrationPreviewResult{}, errors.New("catalog migration manifest snapshot identity is invalid")
		}
		if _, exists := snapshots[snapshot.ContractID]; exists {
			return CatalogMigrationPreviewResult{}, errors.New("catalog migration manifest contains duplicate snapshots")
		}
		snapshots[snapshot.ContractID] = snapshot
	}
	for index := range stored.Preview.Contracts {
		item := &stored.Preview.Contracts[index]
		if !item.Eligible {
			continue
		}
		snapshot, exists := snapshots[item.ContractID]
		if !exists {
			return CatalogMigrationPreviewResult{}, errors.New("catalog migration manifest eligible snapshot is missing")
		}
		item.CurrentPlanSnapshot = snapshot.CurrentPlanSnapshot
		item.TargetPlanSnapshot = snapshot.TargetPlanSnapshot
		current, err := DecodeRecurringPlanSnapshotV1(item.CurrentPlanSnapshot)
		if err != nil {
			return CatalogMigrationPreviewResult{}, err
		}
		currentHash, err := RecurringPlanSnapshotV1Fingerprint(current)
		if err != nil || currentHash != item.CurrentSnapshotHash {
			return CatalogMigrationPreviewResult{}, errors.New("catalog migration manifest current snapshot hash mismatch")
		}
		target, err := DecodeRecurringPlanSnapshotV1(item.TargetPlanSnapshot)
		if err != nil {
			return CatalogMigrationPreviewResult{}, err
		}
		targetHash, err := RecurringPlanSnapshotV1Fingerprint(target)
		if err != nil || targetHash != item.TargetSnapshotHash {
			return CatalogMigrationPreviewResult{}, errors.New("catalog migration manifest target snapshot hash mismatch")
		}
	}
	return stored.Preview, nil
}

func decodeCatalogMigrationStoredManifestForBatch(batch model.SubscriptionCatalogMigrationBatch) (CatalogMigrationPreviewResult, error) {
	preview, err := decodeCatalogMigrationStoredManifest(batch.ManifestSnapshot)
	if err != nil {
		return CatalogMigrationPreviewResult{}, err
	}
	if strings.TrimSpace(preview.RequestID) != strings.TrimSpace(batch.RequestId) {
		return CatalogMigrationPreviewResult{}, errors.New("catalog migration manifest request identity mismatch")
	}
	digest, err := catalogMigrationPreviewDigest(preview)
	if err != nil || digest != strings.ToLower(strings.TrimSpace(batch.CohortDigest)) {
		return CatalogMigrationPreviewResult{}, errors.New("catalog migration manifest cohort digest mismatch")
	}
	return preview, nil
}

func validateCatalogMigrationPreviewSandbox(config CatalogMigrationSandboxConfig, preview CatalogMigrationPreviewResult) error {
	for _, item := range preview.Contracts {
		if item.Eligible {
			if err := ValidateCatalogMigrationCommonSandbox(config, item.ContractID); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateCatalogMigrationScheduleResult(request CatalogMigrationProviderScheduleRequest, result CatalogMigrationProviderScheduleResult) error {
	if strings.TrimSpace(result.ScheduleID) == "" || strings.TrimSpace(result.OwnershipFingerprint) == "" || strings.TrimSpace(result.OwnershipFingerprint) != strings.TrimSpace(request.ExpectedOwnershipFingerprint) {
		return errors.New("catalog migration provider schedule ownership mismatch")
	}
	return nil
}

func catalogMigrationReservationMatches(binding *model.SubscriptionProviderBinding, reservation *model.SubscriptionProviderLifecycleReservation) bool {
	return binding != nil && reservation != nil && binding.Id == reservation.BindingId && binding.UserId == reservation.UserId && binding.ContractId == reservation.ContractId && binding.LifecycleActionSeq == reservation.LifecycleActionSeq && binding.LifecycleReservationToken == reservation.Token && binding.LifecycleReservationAction == reservation.Action && binding.LifecycleReservationUntil == reservation.ExpiresAt
}

func stableCatalogMigrationKey(parts ...any) string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		values = append(values, fmt.Sprint(part))
	}
	digest := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return "catmig_" + hex.EncodeToString(digest[:])
}

func firstError(actual error, fallback error) error {
	if actual != nil {
		return actual
	}
	return fallback
}

func addCatalogMigrationSummaryItem(summary *CatalogMigrationOperationSummary, item CatalogMigrationOperationItem) {
	summary.Total++
	switch item.Status {
	case "skipped":
		summary.Skipped++
	case model.SubscriptionChangeIntentStatusScheduled:
		summary.Scheduled++
	case model.SubscriptionChangeIntentStatusApplied:
		summary.Applied++
	case model.SubscriptionChangeIntentStatusSuperseded:
		summary.Superseded++
	case model.SubscriptionChangeIntentStatusNeedsAttention, model.SubscriptionChangeIntentStatusCompensationRequired:
		summary.NeedsAttention++
	default:
		summary.Failed++
	}
}

func publicCatalogMigrationReason(status string) string {
	switch status {
	case model.SubscriptionChangeIntentStatusCompensationRequired:
		return "provider_reconciliation_required"
	case model.SubscriptionChangeIntentStatusNeedsAttention:
		return "manual_review_required"
	case model.SubscriptionChangeIntentStatusFailed:
		return "operation_failed"
	default:
		return ""
	}
}

func catalogMigrationBatchStatus(summary CatalogMigrationOperationSummary) string {
	switch {
	case summary.NeedsAttention > 0:
		return model.SubscriptionCatalogMigrationBatchStatusNeedsAttention
	case summary.Applied > 0 && summary.Applied+summary.Skipped == summary.Total:
		return model.SubscriptionCatalogMigrationBatchStatusApplied
	case summary.Superseded > 0 && summary.Superseded+summary.Skipped == summary.Total:
		return model.SubscriptionCatalogMigrationBatchStatusCancelled
	case summary.Failed > 0 || summary.Superseded > 0 || summary.Applied > 0:
		return model.SubscriptionCatalogMigrationBatchStatusPartial
	default:
		return model.SubscriptionCatalogMigrationBatchStatusScheduled
	}
}
