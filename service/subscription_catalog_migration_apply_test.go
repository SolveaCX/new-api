package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

type catalogMigrationSchedulerStub struct {
	mu            sync.Mutex
	scheduleCalls int
	restoreCalls  int
	failSchedule  error
	failRestore   error
}

func TestCatalogMigrationOwnershipFingerprintFitsFixedWidthColumn(t *testing.T) {
	first := catalogMigrationOwnershipFingerprint("batch", int64(16), int64(95), "sub_test", "si_test", "price_old", "price_new", int64(123))
	second := catalogMigrationOwnershipFingerprint("batch", int64(16), int64(95), "sub_test", "si_test", "price_old", "price_new", int64(123))
	require.Len(t, first, 64)
	require.Equal(t, first, second)
	require.NotEqual(t, first, stableCatalogMigrationKey("ownership", "batch", int64(16), int64(95)))
}

func (s *catalogMigrationSchedulerStub) ScheduleCatalogMigration(_ context.Context, request CatalogMigrationProviderScheduleRequest) (CatalogMigrationProviderScheduleResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scheduleCalls++
	if s.failSchedule != nil {
		return CatalogMigrationProviderScheduleResult{}, s.failSchedule
	}
	return CatalogMigrationProviderScheduleResult{ScheduleID: "sched_owned", OwnershipFingerprint: request.ExpectedOwnershipFingerprint}, nil
}

func (s *catalogMigrationSchedulerStub) RestoreCatalogMigration(_ context.Context, _ CatalogMigrationProviderScheduleRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restoreCalls++
	return s.failRestore
}

func TestCatalogMigrationApplyWalletIsIdempotentAndDoesNotChangeCurrentRights(t *testing.T) {
	service, command, contract, entitlement := setupCatalogMigrationApplyFixture(t, false)
	historicalOrder := model.SubscriptionOrder{
		UserId: entitlement.UserId, PlanId: entitlement.PlanId, Money: 10, TradeNo: "legacy-order-preserved",
		PaymentMethod: "balance", PaymentProvider: model.PaymentProviderBalance, Status: common.TopUpStatusSuccess,
		CreateTime: entitlement.StartTime - 10, CompleteTime: entitlement.StartTime - 5, PurchaseMonths: 1,
		UnitPrice: 10, PaymentCurrency: "USD", PaymentAmountMinor: 1000, PlanSnapshot: `{"legacy":true}`,
	}
	require.NoError(t, model.DB.Create(&historicalOrder).Error)
	require.NoError(t, model.DB.First(&historicalOrder, historicalOrder.Id).Error)

	first, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	require.Equal(t, 1, first.Summary.Scheduled)
	second, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	require.Equal(t, first.Batch.Id, second.Batch.Id)

	var storedContract model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&storedContract, contract.Id).Error)
	require.Equal(t, contract.CurrentPlanId, storedContract.CurrentPlanId)
	require.Equal(t, entitlement.Id, storedContract.CurrentEntitlementId)
	require.Equal(t, 2, storedContract.PendingPlanId)
	var storedEntitlement model.UserSubscription
	require.NoError(t, model.DB.First(&storedEntitlement, entitlement.Id).Error)
	require.Equal(t, entitlement.AmountTotal, storedEntitlement.AmountTotal)
	require.Equal(t, entitlement.AmountUsed, storedEntitlement.AmountUsed)
	require.Equal(t, entitlement.Status, storedEntitlement.Status)
	require.Equal(t, entitlement.CurrentSlot, storedEntitlement.CurrentSlot)
	require.Equal(t, entitlement.Window5hAmount, storedEntitlement.Window5hAmount)
	require.Equal(t, entitlement.WindowWeekAmount, storedEntitlement.WindowWeekAmount)
	require.Equal(t, entitlement.StartTime, storedEntitlement.StartTime)
	require.Equal(t, entitlement.EndTime, storedEntitlement.EndTime)
	require.Equal(t, entitlement.AccessEndTime, storedEntitlement.AccessEndTime)
	var storedHistoricalOrder model.SubscriptionOrder
	require.NoError(t, model.DB.First(&storedHistoricalOrder, historicalOrder.Id).Error)
	require.Equal(t, historicalOrder, storedHistoricalOrder)
	var count int64
	require.NoError(t, model.DB.Model(&model.SubscriptionChangeIntent{}).Where("contract_id = ?", contract.Id).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestCatalogMigrationApplyCASDriftNeverCallsProviderOrChangesRights(t *testing.T) {
	testCases := []struct {
		name   string
		mutate func(t *testing.T, contract model.UserSubscriptionContract, entitlement model.UserSubscription)
	}{
		{name: "entitlement amount used", mutate: func(t *testing.T, _ model.UserSubscriptionContract, entitlement model.UserSubscription) {
			require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("id = ?", entitlement.Id).UpdateColumn("amount_used", entitlement.AmountUsed+1).Error)
		}},
		{name: "target balance eligibility", mutate: func(t *testing.T, _ model.UserSubscriptionContract, _ model.UserSubscription) {
			var target model.SubscriptionPlan
			require.NoError(t, model.DB.First(&target, 2).Error)
			value := false
			require.NoError(t, model.DB.Model(&target).UpdateColumn("allow_balance_pay", &value).Error)
		}},
		{name: "binding provider status", mutate: func(t *testing.T, contract model.UserSubscriptionContract, _ model.UserSubscription) {
			require.NoError(t, model.DB.Model(&model.SubscriptionProviderBinding{}).Where("id = ?", contract.CurrentProviderBindingId).UpdateColumn("provider_status", "past_due").Error)
		}},
		{name: "binding lifecycle sequence", mutate: func(t *testing.T, contract model.UserSubscriptionContract, _ model.UserSubscription) {
			require.NoError(t, model.DB.Model(&model.SubscriptionProviderBinding{}).Where("id = ?", contract.CurrentProviderBindingId).UpdateColumn("lifecycle_action_seq", 9).Error)
		}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service, command, contract, entitlement := setupCatalogMigrationApplyFixture(t, true)
			scheduler := &catalogMigrationSchedulerStub{}
			service.Scheduler = scheduler
			testCase.mutate(t, contract, entitlement)
			var beforeContract model.UserSubscriptionContract
			var beforeEntitlement model.UserSubscription
			require.NoError(t, model.DB.First(&beforeContract, contract.Id).Error)
			require.NoError(t, model.DB.First(&beforeEntitlement, entitlement.Id).Error)

			result, err := service.Apply(context.Background(), command)
			require.NoError(t, err)
			require.Equal(t, 1, result.Summary.Failed)
			require.Equal(t, 0, scheduler.scheduleCalls)
			require.Equal(t, 0, scheduler.restoreCalls)
			var afterContract model.UserSubscriptionContract
			var afterEntitlement model.UserSubscription
			require.NoError(t, model.DB.First(&afterContract, contract.Id).Error)
			require.NoError(t, model.DB.First(&afterEntitlement, entitlement.Id).Error)
			require.Equal(t, beforeContract, afterContract)
			require.Equal(t, beforeEntitlement, afterEntitlement)
			var intents int64
			require.NoError(t, model.DB.Model(&model.SubscriptionChangeIntent{}).Where("catalog_migration_batch_id = ?", result.Batch.Id).Count(&intents).Error)
			require.Zero(t, intents)
		})
	}
}

func TestCatalogMigrationRuntimeGuardsBlockApplyResumeAndCancelWithoutMutation(t *testing.T) {
	guards := []struct {
		name   string
		mutate func(*CatalogMigrationSandboxConfig, int64)
	}{
		{name: "wrong environment", mutate: func(config *CatalogMigrationSandboxConfig, _ int64) { config.DeploymentEnvironment = "production" }},
		{name: "wrong service", mutate: func(config *CatalogMigrationSandboxConfig, _ int64) { config.ServiceName = "newapi" }},
		{name: "removed allowlist", mutate: func(config *CatalogMigrationSandboxConfig, _ int64) { config.AllowedContractIDs = nil }},
	}
	for _, guard := range guards {
		for _, operation := range []string{"apply", "resume", "cancel"} {
			t.Run(guard.name+"/"+operation, func(t *testing.T) {
				service, command, contract, entitlement := setupCatalogMigrationApplyFixture(t, true)
				scheduler := &catalogMigrationSchedulerStub{}
				service.Scheduler = scheduler
				var batchID string
				if operation != "apply" {
					preview, err := service.Preview(context.Background(), command.PreviewRequest)
					require.NoError(t, err)
					manifest, err := encodeCatalogMigrationStoredManifest(preview)
					require.NoError(t, err)
					batch := model.SubscriptionCatalogMigrationBatch{RequestId: preview.RequestID, CohortDigest: preview.CohortDigest, Status: model.SubscriptionCatalogMigrationBatchStatusApplying, ManifestSnapshot: manifest, RequestedBy: 1, DeploymentEnvironment: "staging", ServiceName: "newapi-staging", SandboxOnly: true}
					require.NoError(t, model.DB.Create(&batch).Error)
					batchID = batch.Id
					if operation == "resume" {
						_, err = service.prepareStripeContract(context.Background(), &batch, preview.Contracts[0])
						require.NoError(t, err)
					} else {
						result, err := service.Apply(context.Background(), command)
						require.NoError(t, err)
						batchID = result.Batch.Id
						require.Equal(t, 1, scheduler.scheduleCalls)
					}
				}
				guard.mutate(&service.Sandbox, contract.Id)
				var beforeContract model.UserSubscriptionContract
				var beforeEntitlement model.UserSubscription
				var beforeBinding model.SubscriptionProviderBinding
				var beforeIntents []model.SubscriptionChangeIntent
				var beforeBatch model.SubscriptionCatalogMigrationBatch
				require.NoError(t, model.DB.First(&beforeContract, contract.Id).Error)
				require.NoError(t, model.DB.First(&beforeEntitlement, entitlement.Id).Error)
				require.NoError(t, model.DB.First(&beforeBinding, contract.CurrentProviderBindingId).Error)
				require.NoError(t, model.DB.Order("id asc").Find(&beforeIntents).Error)
				if batchID != "" {
					require.NoError(t, model.DB.First(&beforeBatch, "id = ?", batchID).Error)
				}
				beforeScheduleCalls, beforeRestoreCalls := scheduler.scheduleCalls, scheduler.restoreCalls

				var err error
				switch operation {
				case "apply":
					_, err = service.Apply(context.Background(), command)
				case "resume":
					_, err = service.Apply(context.Background(), command)
				case "cancel":
					_, err = service.Cancel(context.Background(), CatalogMigrationCancelCommand{BatchID: batchID, RequestedBy: 1})
				}
				require.Error(t, err)
				require.Equal(t, beforeScheduleCalls, scheduler.scheduleCalls)
				require.Equal(t, beforeRestoreCalls, scheduler.restoreCalls)
				var afterContract model.UserSubscriptionContract
				var afterEntitlement model.UserSubscription
				var afterBinding model.SubscriptionProviderBinding
				var afterIntents []model.SubscriptionChangeIntent
				require.NoError(t, model.DB.First(&afterContract, contract.Id).Error)
				require.NoError(t, model.DB.First(&afterEntitlement, entitlement.Id).Error)
				require.NoError(t, model.DB.First(&afterBinding, contract.CurrentProviderBindingId).Error)
				require.NoError(t, model.DB.Order("id asc").Find(&afterIntents).Error)
				require.Equal(t, beforeContract, afterContract)
				require.Equal(t, beforeEntitlement, afterEntitlement)
				require.Equal(t, beforeBinding, afterBinding)
				require.Equal(t, beforeIntents, afterIntents)
				if batchID != "" {
					var afterBatch model.SubscriptionCatalogMigrationBatch
					require.NoError(t, model.DB.First(&afterBatch, "id = ?", batchID).Error)
					require.Equal(t, beforeBatch, afterBatch)
				}
			})
		}
	}
}

func TestCatalogMigrationItemFailureIsPersistedAndObservable(t *testing.T) {
	service, command, contract, _ := setupCatalogMigrationApplyFixture(t, true)
	service.Scheduler = &catalogMigrationSchedulerStub{failSchedule: errors.New("provider unavailable")}

	result, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	require.Equal(t, 1, result.Summary.NeedsAttention)
	require.Equal(t, "provider_reconciliation_required", result.Items[0].Reason)
	var intent model.SubscriptionChangeIntent
	require.NoError(t, model.DB.Where("catalog_migration_batch_id = ? AND contract_id = ?", result.Batch.Id, contract.Id).First(&intent).Error)
	require.Equal(t, model.SubscriptionChangeIntentStatusCompensationRequired, intent.Status)
	require.Contains(t, intent.LastError, "provider unavailable")
	var batch model.SubscriptionCatalogMigrationBatch
	require.NoError(t, model.DB.First(&batch, "id = ?", result.Batch.Id).Error)
	require.Equal(t, model.SubscriptionCatalogMigrationBatchStatusNeedsAttention, batch.Status)
	require.NotEmpty(t, batch.SummarySnapshot)
	reloaded, err := service.Get(context.Background(), result.Batch.Id)
	require.NoError(t, err)
	require.Equal(t, result.Summary, reloaded.Summary)
	require.Equal(t, result.Items, reloaded.Items)
}

func TestCatalogMigrationApplyRejectsDigestDriftBeforeWriting(t *testing.T) {
	service, command, _, _ := setupCatalogMigrationApplyFixture(t, false)
	command.CohortDigest = strings.Repeat("b", 64)
	_, err := service.Apply(context.Background(), command)
	require.ErrorContains(t, err, "digest changed")
	var count int64
	require.NoError(t, model.DB.Model(&model.SubscriptionCatalogMigrationBatch{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestCatalogMigrationApplyStripeSchedulesOnceAndCancelRestoresOwnedSchedule(t *testing.T) {
	service, command, contract, entitlement := setupCatalogMigrationApplyFixture(t, true)
	scheduler := &catalogMigrationSchedulerStub{}
	service.Scheduler = scheduler

	result, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	require.Equal(t, 1, result.Summary.Scheduled)
	service.Sandbox.FeatureEnabled = false
	_, err = service.Apply(context.Background(), command)
	require.NoError(t, err)
	require.Equal(t, 1, scheduler.scheduleCalls)
	service.Sandbox.FeatureEnabled = true

	var unchanged model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&unchanged, contract.Id).Error)
	require.Equal(t, contract.CurrentPlanId, unchanged.CurrentPlanId)
	require.Equal(t, entitlement.Id, unchanged.CurrentEntitlementId)
	var frozenBinding model.SubscriptionProviderBinding
	require.NoError(t, model.DB.First(&frozenBinding, contract.CurrentProviderBindingId).Error)
	require.NotEmpty(t, frozenBinding.CurrentPlanSnapshot)
	frozenSnapshot, err := DecodeRecurringPlanSnapshotV1(frozenBinding.CurrentPlanSnapshot)
	require.NoError(t, err)
	require.Equal(t, contract.CurrentPlanId, frozenSnapshot.PlanID)

	cancelled, err := service.Cancel(context.Background(), CatalogMigrationCancelCommand{BatchID: result.Batch.Id, RequestedBy: 99})
	require.NoError(t, err)
	require.Equal(t, 1, cancelled.Summary.Superseded)
	require.Equal(t, 1, scheduler.restoreCalls)
	require.NoError(t, model.DB.First(&unchanged, contract.Id).Error)
	require.Zero(t, unchanged.PendingPlanId)
	require.Equal(t, contract.CurrentPlanId, unchanged.CurrentPlanId)
}

func TestCatalogMigrationApplyClosedSandboxWritesNothing(t *testing.T) {
	service, command, _, _ := setupCatalogMigrationApplyFixture(t, false)
	service.Sandbox.FeatureEnabled = false
	previewCalled := false
	service.Preview = func(context.Context, CatalogMigrationPreviewRequest) (CatalogMigrationPreviewResult, error) {
		previewCalled = true
		return CatalogMigrationPreviewResult{}, nil
	}
	_, err := service.Apply(context.Background(), command)
	require.ErrorContains(t, err, "disabled")
	require.False(t, previewCalled)
	var count int64
	require.NoError(t, model.DB.Model(&model.SubscriptionCatalogMigrationBatch{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestCatalogMigrationApplyReplayReturnsExistingBatchAfterSandboxCloses(t *testing.T) {
	service, command, _, _ := setupCatalogMigrationApplyFixture(t, true)
	scheduler := &catalogMigrationSchedulerStub{}
	service.Scheduler = scheduler

	first, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	require.Equal(t, 1, scheduler.scheduleCalls)

	service.Sandbox.FeatureEnabled = false
	replayed, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	require.Equal(t, first.Batch.Id, replayed.Batch.Id)
	require.Equal(t, 1, scheduler.scheduleCalls)
}

func TestCatalogMigrationConcurrentApplyCreatesOneBatchAndIntent(t *testing.T) {
	service, command, contract, _ := setupCatalogMigrationApplyFixture(t, false)
	start := make(chan struct{})
	errorsSeen := make(chan error, 2)
	for index := 0; index < 2; index++ {
		go func() {
			<-start
			_, err := service.Apply(context.Background(), command)
			errorsSeen <- err
		}()
	}
	close(start)
	require.NoError(t, <-errorsSeen)
	require.NoError(t, <-errorsSeen)
	var batches, intents int64
	require.NoError(t, model.DB.Model(&model.SubscriptionCatalogMigrationBatch{}).Count(&batches).Error)
	require.NoError(t, model.DB.Model(&model.SubscriptionChangeIntent{}).Where("contract_id = ?", contract.Id).Count(&intents).Error)
	require.Equal(t, int64(1), batches)
	require.Equal(t, int64(1), intents)
}

func TestCatalogMigrationCancelClosedSandboxMakesNoMutation(t *testing.T) {
	service, command, contract, _ := setupCatalogMigrationApplyFixture(t, true)
	scheduler := &catalogMigrationSchedulerStub{}
	service.Scheduler = scheduler
	result, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	service.Sandbox.FeatureEnabled = false
	var beforeContract model.UserSubscriptionContract
	var beforeIntent model.SubscriptionChangeIntent
	require.NoError(t, model.DB.First(&beforeContract, contract.Id).Error)
	require.NoError(t, model.DB.Where("catalog_migration_batch_id = ?", result.Batch.Id).First(&beforeIntent).Error)

	_, err = service.Cancel(context.Background(), CatalogMigrationCancelCommand{BatchID: result.Batch.Id, RequestedBy: 1})
	require.ErrorContains(t, err, "disabled")
	require.Equal(t, 0, scheduler.restoreCalls)
	var afterContract model.UserSubscriptionContract
	var afterIntent model.SubscriptionChangeIntent
	require.NoError(t, model.DB.First(&afterContract, contract.Id).Error)
	require.NoError(t, model.DB.First(&afterIntent, beforeIntent.Id).Error)
	require.Equal(t, beforeContract, afterContract)
	require.Equal(t, beforeIntent, afterIntent)
}

func TestCatalogMigrationExistingApplyingBatchRecoversMissingIntent(t *testing.T) {
	service, command, contract, _ := setupCatalogMigrationApplyFixture(t, false)
	preview, err := service.Preview(context.Background(), command.PreviewRequest)
	require.NoError(t, err)
	manifest, err := encodeCatalogMigrationStoredManifest(preview)
	require.NoError(t, err)
	batch := model.SubscriptionCatalogMigrationBatch{RequestId: preview.RequestID, CohortDigest: preview.CohortDigest, Status: model.SubscriptionCatalogMigrationBatchStatusApplying, ManifestSnapshot: manifest, RequestedBy: 1, DeploymentEnvironment: "staging", ServiceName: "newapi-staging", SandboxOnly: true}
	require.NoError(t, model.DB.Create(&batch).Error)

	result, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	require.Equal(t, batch.Id, result.Batch.Id)
	require.Equal(t, 1, result.Summary.Scheduled)
	var count int64
	require.NoError(t, model.DB.Model(&model.SubscriptionChangeIntent{}).Where("contract_id = ?", contract.Id).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestCatalogMigrationExistingScheduledIntentNeverRetriggersProvider(t *testing.T) {
	service, command, _, _ := setupCatalogMigrationApplyFixture(t, true)
	scheduler := &catalogMigrationSchedulerStub{}
	service.Scheduler = scheduler
	result, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	require.Equal(t, 1, scheduler.scheduleCalls)
	require.NoError(t, model.DB.Model(&model.SubscriptionCatalogMigrationBatch{}).Where("id = ?", result.Batch.Id).Update("status", model.SubscriptionCatalogMigrationBatchStatusApplying).Error)

	replayed, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	require.Equal(t, result.Batch.Id, replayed.Batch.Id)
	require.Equal(t, 1, scheduler.scheduleCalls)
}

func TestCatalogMigrationExistingSyncingIntentReclaimsWithSameProviderKey(t *testing.T) {
	service, command, _, _ := setupCatalogMigrationApplyFixture(t, true)
	preview, err := service.Preview(context.Background(), command.PreviewRequest)
	require.NoError(t, err)
	manifest, err := encodeCatalogMigrationStoredManifest(preview)
	require.NoError(t, err)
	batch := model.SubscriptionCatalogMigrationBatch{RequestId: preview.RequestID, CohortDigest: preview.CohortDigest, Status: model.SubscriptionCatalogMigrationBatchStatusApplying, ManifestSnapshot: manifest, RequestedBy: 1, DeploymentEnvironment: "staging", ServiceName: "newapi-staging", SandboxOnly: true}
	require.NoError(t, model.DB.Create(&batch).Error)
	prepared, err := service.prepareStripeContract(context.Background(), &batch, preview.Contracts[0])
	require.NoError(t, err)
	originalKey := prepared.Request.IdempotencyKey
	scheduler := &catalogMigrationSchedulerStub{}
	service.Scheduler = scheduler

	result, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	require.Equal(t, 1, result.Summary.Scheduled)
	require.Equal(t, 1, scheduler.scheduleCalls)
	var intent model.SubscriptionChangeIntent
	require.NoError(t, model.DB.First(&intent, prepared.Intent.Id).Error)
	require.Equal(t, originalKey, intent.ProviderIdempotencyKey)
}

func TestCatalogMigrationApplyDriftStopsBeforeProvider(t *testing.T) {
	service, command, contract, _ := setupCatalogMigrationApplyFixture(t, true)
	scheduler := &catalogMigrationSchedulerStub{}
	service.Scheduler = scheduler
	require.NoError(t, model.DB.Model(&model.UserSubscriptionContract{}).Where("id = ?", contract.Id).Update("change_version", contract.ChangeVersion+1).Error)
	result, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	require.Equal(t, 0, scheduler.scheduleCalls)
	require.Equal(t, 1, result.Summary.Failed)
	var stored model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&stored, contract.Id).Error)
	require.Zero(t, stored.PendingPlanId)
}

func TestCatalogMigrationOperationResultRedactsManifestAndRawProviderError(t *testing.T) {
	service, command, _, _ := setupCatalogMigrationApplyFixture(t, true)
	service.Scheduler = &catalogMigrationSchedulerStub{failSchedule: errors.New("secret sub_123 cus_123 price_123")}
	result, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	payload, err := json.Marshal(result)
	require.NoError(t, err)
	encoded := string(payload)
	require.NotContains(t, encoded, "manifest_snapshot")
	require.NotContains(t, encoded, "summary_snapshot")
	require.NotContains(t, encoded, "sub_123")
	require.NotContains(t, encoded, "cus_123")
	require.NotContains(t, encoded, "price_123")
	require.Contains(t, encoded, "provider_reconciliation_required")
}

func TestCatalogMigrationPreservesAndRestoresTerminalPriorIntent(t *testing.T) {
	for _, stripeMode := range []bool{false, true} {
		name := "wallet"
		if stripeMode {
			name = "stripe"
		}
		t.Run(name, func(t *testing.T) {
			service, command, contract, _ := setupCatalogMigrationApplyFixture(t, stripeMode)
			prior := model.SubscriptionChangeIntent{ContractId: contract.Id, UserId: contract.UserId, RequestId: "prior-" + name, ChangeVersion: contract.ChangeVersion - 1, Kind: model.SubscriptionChangeIntentKindPurchase, PaymentMode: contract.PaymentMode, Status: model.SubscriptionChangeIntentStatusApplied, FromPlanId: contract.CurrentPlanId, ToPlanId: contract.CurrentPlanId}
			require.NoError(t, model.DB.Create(&prior).Error)
			require.NoError(t, model.DB.Model(&model.UserSubscriptionContract{}).Where("id = ?", contract.Id).Update("latest_change_intent_id", prior.Id).Error)
			require.NoError(t, model.DB.First(&contract, contract.Id).Error)
			preview, err := service.Preview(context.Background(), command.PreviewRequest)
			require.NoError(t, err)
			preview.Contracts[0].PriorLatestChangeIntentID = prior.Id
			preview.Contracts[0].ContractFingerprint = fingerprintCatalogMigrationContract(contract)
			preview.CohortDigest, err = catalogMigrationPreviewDigest(preview)
			require.NoError(t, err)
			service.Preview = func(context.Context, CatalogMigrationPreviewRequest) (CatalogMigrationPreviewResult, error) {
				return preview, nil
			}
			command.CohortDigest = preview.CohortDigest
			scheduler := &catalogMigrationSchedulerStub{}
			if stripeMode {
				service.Scheduler = scheduler
			}
			result, err := service.Apply(context.Background(), command)
			require.NoError(t, err)
			var migration model.SubscriptionChangeIntent
			require.NoError(t, model.DB.Where("catalog_migration_batch_id = ?", result.Batch.Id).First(&migration).Error)
			require.Equal(t, prior.Id, migration.PreviousChangeIntentId)
			_, err = service.Cancel(context.Background(), CatalogMigrationCancelCommand{BatchID: result.Batch.Id, RequestedBy: 1})
			require.NoError(t, err)
			require.NoError(t, model.DB.First(&contract, contract.Id).Error)
			require.Equal(t, prior.Id, contract.LatestChangeIntentId)
		})
	}
}

func TestCatalogMigrationProviderRestoreSuccessLocalFinalizeFailureIsNotReportedAsSuccess(t *testing.T) {
	service, command, _, _ := setupCatalogMigrationApplyFixture(t, true)
	scheduler := &catalogMigrationSchedulerStub{}
	service.Scheduler = scheduler
	result, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	service.AfterProviderRestore = func() error { return errors.New("forced local restore finalize failure") }

	_, err = service.Cancel(context.Background(), CatalogMigrationCancelCommand{BatchID: result.Batch.Id, RequestedBy: 1})
	require.ErrorContains(t, err, "forced local restore finalize failure")
	require.Equal(t, 1, scheduler.restoreCalls)
	var intent model.SubscriptionChangeIntent
	require.NoError(t, model.DB.Where("catalog_migration_batch_id = ?", result.Batch.Id).First(&intent).Error)
	require.Equal(t, model.SubscriptionChangeIntentStatusNeedsAttention, intent.Status)
}

func TestCatalogMigrationLiveStripeCredentialsNeverCallScheduler(t *testing.T) {
	service, command, contract, _ := setupCatalogMigrationApplyFixture(t, true)
	scheduler := &catalogMigrationSchedulerStub{}
	service.Scheduler = scheduler
	service.Sandbox.StripeSecret = "sk_live_forbidden"
	service.Sandbox.StripePublishableKey = "pk_live_forbidden"

	result, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	require.Equal(t, 0, scheduler.scheduleCalls)
	var stored model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&stored, contract.Id).Error)
	require.Zero(t, stored.PendingPlanId)
	require.Equal(t, contract.CurrentPlanId, stored.CurrentPlanId)
	require.Equal(t, 1, result.Summary.Failed)
}

func TestCatalogMigrationStripeFinalizeFailureRecordsCompensationAndRestores(t *testing.T) {
	service, command, _, _ := setupCatalogMigrationApplyFixture(t, true)
	scheduler := &catalogMigrationSchedulerStub{}
	service.Scheduler = scheduler
	service.AfterProviderSchedule = func(CatalogMigrationProviderScheduleResult) error { return errors.New("forced finalize failure") }

	result, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	require.Equal(t, 1, scheduler.scheduleCalls)
	require.Equal(t, 1, scheduler.restoreCalls)
	require.Equal(t, 1, result.Summary.NeedsAttention)
	var intent model.SubscriptionChangeIntent
	require.NoError(t, model.DB.Where("catalog_migration_batch_id = ?", result.Batch.Id).First(&intent).Error)
	require.Equal(t, model.SubscriptionChangeIntentStatusCompensationRequired, intent.Status)
}

func TestCatalogMigrationApplyMixedContractsKeepsSuccessfulWalletSchedule(t *testing.T) {
	service, command, walletContract, walletEntitlement := setupCatalogMigrationApplyFixture(t, false)
	stripeContract := walletContract
	stripeContract.Id = 22
	stripeContract.UserId = 12
	stripeContract.CurrentEntitlementId = 32
	stripeContract.CurrentProviderBindingId = 42
	stripeContract.PaymentMode = model.SubscriptionPaymentModeStripeRecurring
	stripeContract.RenewalSource = model.SubscriptionRenewalSourceProvider
	stripeEntitlement := walletEntitlement
	stripeEntitlement.Id = 32
	stripeEntitlement.UserId = 12
	stripeEntitlement.ContractId = 22
	stripeEntitlement.ProviderBindingId = 42
	stripeEntitlement.PaymentMode = model.SubscriptionPaymentModeStripeRecurring
	require.NoError(t, model.DB.Create(&stripeContract).Error)
	require.NoError(t, model.DB.Create(&stripeEntitlement).Error)
	mixedBinding := model.SubscriptionProviderBinding{Id: 42, UserId: 12, PlanId: stripeContract.CurrentPlanId, ContractId: 22, Provider: model.PaymentProviderStripe, ProviderSubscriptionId: "sub_mixed", ProviderSubscriptionItemId: "si_mixed", ProviderCustomerId: "cus_mixed", ProviderPriceId: "price_old", ProviderStatus: "active", CurrentPeriodStart: stripeContract.CurrentPeriodStart, CurrentPeriodEnd: stripeContract.CurrentPeriodEnd}
	require.NoError(t, model.DB.Create(&mixedBinding).Error)
	require.NoError(t, model.DB.First(&stripeContract, stripeContract.Id).Error)
	require.NoError(t, model.DB.First(&stripeEntitlement, stripeEntitlement.Id).Error)
	require.NoError(t, model.DB.First(&mixedBinding, mixedBinding.Id).Error)
	preview, err := service.Preview(context.Background(), command.PreviewRequest)
	require.NoError(t, err)
	stripeItem := preview.Contracts[0]
	stripeItem.ContractID = stripeContract.Id
	stripeItem.PaymentMode = stripeContract.PaymentMode
	stripeItem.ExpectedChangeVersion = stripeContract.ChangeVersion
	stripeItem.ContractFingerprint = fingerprintCatalogMigrationContract(stripeContract)
	stripeItem.EntitlementFingerprint = fingerprintJSON(stripeEntitlement)
	stripeItem.BindingFingerprint = fingerprintCatalogMigrationBinding(mixedBinding)
	stripeItem.ProviderFingerprint = "mixed-provider-fingerprint"
	preview.Contracts = append(preview.Contracts, stripeItem)
	preview.Summary.Requested = 2
	preview.Summary.Eligible = 2
	preview.CohortDigest, err = catalogMigrationPreviewDigest(preview)
	require.NoError(t, err)
	service.Preview = func(context.Context, CatalogMigrationPreviewRequest) (CatalogMigrationPreviewResult, error) {
		return preview, nil
	}
	service.Sandbox.AllowedContractIDs = []int64{walletContract.Id, stripeContract.Id}
	command.PreviewRequest.ContractIDs = []int64{walletContract.Id, stripeContract.Id}
	command.CohortDigest = preview.CohortDigest
	service.Scheduler = &catalogMigrationSchedulerStub{failSchedule: errors.New("provider unavailable")}

	result, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	require.Equal(t, 1, result.Summary.Scheduled)
	require.Equal(t, 1, result.Summary.NeedsAttention)
	var unchanged model.UserSubscriptionContract
	require.NoError(t, model.DB.First(&unchanged, walletContract.Id).Error)
	require.Equal(t, walletContract.CurrentPlanId, unchanged.CurrentPlanId)
	require.Equal(t, 2, unchanged.PendingPlanId)
}

func setupCatalogMigrationApplyFixture(t *testing.T, stripeMode bool) (CatalogMigrationService, CatalogMigrationApplyCommand, model.UserSubscriptionContract, model.UserSubscription) {
	t.Helper()
	setupSubscriptionContractServiceTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.SubscriptionCatalogMigrationBatch{}))
	now := common.GetTimestamp()
	tier := 1
	source := model.SubscriptionPlan{Id: 1, Title: "Legacy Go", PriceAmount: 10, Currency: "USD", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, TierRank: &tier, StripePriceId: "price_old", TotalAmount: 5000000, Window5hAmount: 100, WindowWeekAmount: 200, QuotaResetPeriod: model.SubscriptionResetNever}
	target := model.SubscriptionPlan{Id: 2, Title: "Go", PriceAmount: 10, Currency: "USD", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, TierRank: &tier, StripePriceId: "price_new", TotalAmount: 6500000, QuotaResetPeriod: model.SubscriptionResetNever}
	require.NoError(t, model.DB.Create(&source).Error)
	require.NoError(t, model.DB.Create(&target).Error)
	currentSlot := 1
	entitlement := model.UserSubscription{Id: 31, UserId: 11, PlanId: source.Id, Status: "active", AmountTotal: source.TotalAmount, AmountUsed: 123, StartTime: now - 100, EndTime: now + 3600, AccessEndTime: now + 7200, CurrentSlot: &currentSlot, Window5hAmount: common.GetPointer(source.Window5hAmount), WindowWeekAmount: common.GetPointer(source.WindowWeekAmount), Source: "order"}
	contract := model.UserSubscriptionContract{Id: 21, UserId: 11, Status: model.SubscriptionContractStatusActive, PaymentMode: model.SubscriptionPaymentModeBalanceOnePeriod, RenewalSource: model.SubscriptionRenewalSourceWallet, RenewalStatus: model.SubscriptionRenewalStatusEnabled, CurrentPlanId: source.Id, CurrentEntitlementId: entitlement.Id, CurrentPeriodStart: entitlement.StartTime, CurrentPeriodEnd: entitlement.EndTime, ChangeVersion: 7}
	if stripeMode {
		contract.PaymentMode = model.SubscriptionPaymentModeStripeRecurring
		contract.RenewalSource = model.SubscriptionRenewalSourceProvider
		contract.CurrentProviderBindingId = 41
		entitlement.ProviderBindingId = 41
	}
	entitlement.PaymentMode = contract.PaymentMode
	entitlement.ContractId = contract.Id
	require.NoError(t, model.DB.Create(&contract).Error)
	require.NoError(t, model.DB.Create(&entitlement).Error)
	if stripeMode {
		binding := model.SubscriptionProviderBinding{Id: 41, UserId: contract.UserId, PlanId: source.Id, ContractId: contract.Id, Provider: model.PaymentProviderStripe, ProviderSubscriptionId: "sub_test", ProviderSubscriptionItemId: "si_test", ProviderCustomerId: "cus_test", ProviderPriceId: source.StripePriceId, ProviderStatus: "active", CurrentPeriodStart: contract.CurrentPeriodStart, CurrentPeriodEnd: contract.CurrentPeriodEnd}
		require.NoError(t, model.DB.Create(&binding).Error)
	}
	require.NoError(t, model.DB.First(&contract, contract.Id).Error)
	require.NoError(t, model.DB.First(&entitlement, entitlement.Id).Error)
	targetSnapshot, err := recurringSnapshotFromPlanOnly(&target)
	require.NoError(t, err)
	targetHash, err := RecurringPlanSnapshotV1Fingerprint(targetSnapshot)
	require.NoError(t, err)
	targetSnapshotJSON, err := EncodeRecurringPlanSnapshotV1(targetSnapshot)
	require.NoError(t, err)
	sourceSnapshot, err := recurringSnapshotFromPlanOnly(&source)
	require.NoError(t, err)
	sourceHash, err := RecurringPlanSnapshotV1Fingerprint(sourceSnapshot)
	require.NoError(t, err)
	sourceSnapshotJSON, err := EncodeRecurringPlanSnapshotV1(sourceSnapshot)
	require.NoError(t, err)
	item := CatalogMigrationContractPreviewResult{ContractID: contract.Id, Eligible: true, Reason: CatalogMigrationReasonEligible, PaymentMode: contract.PaymentMode, SourcePlanID: source.Id, TargetPlanID: target.Id, EffectiveAt: contract.CurrentPeriodEnd, PriorLatestChangeIntentID: contract.LatestChangeIntentId, ExpectedChangeVersion: contract.ChangeVersion, ContractFingerprint: fingerprintCatalogMigrationContract(contract), EntitlementFingerprint: fingerprintJSON(entitlement), SourcePlanFingerprint: fingerprintPlan(source), TargetPlanFingerprint: fingerprintPlan(target), CurrentSnapshotHash: sourceHash, TargetSnapshotHash: targetHash, CurrentPlanSnapshot: sourceSnapshotJSON, TargetPlanSnapshot: targetSnapshotJSON}
	if stripeMode {
		var binding model.SubscriptionProviderBinding
		require.NoError(t, model.DB.First(&binding, contract.CurrentProviderBindingId).Error)
		item.BindingFingerprint = fingerprintCatalogMigrationBinding(binding)
		item.ProviderFingerprint = "provider-fingerprint"
	}
	preview := CatalogMigrationPreviewResult{RequestID: "apply-request", Contracts: []CatalogMigrationContractPreviewResult{item}, Summary: CatalogMigrationPreviewSummary{Requested: 1, Eligible: 1, ByReason: map[string]int{CatalogMigrationReasonEligible: 1}}}
	preview.CohortDigest, err = catalogMigrationPreviewDigest(preview)
	require.NoError(t, err)
	service := CatalogMigrationService{DB: model.DB, Sandbox: CatalogMigrationSandboxConfig{DeploymentEnvironment: "staging", ServiceName: "newapi-staging", FeatureEnabled: true, AllowedContractIDs: []int64{contract.Id}, StripeSecret: "rk_test_catalog", StripePublishableKey: "pk_test_catalog"}, Preview: func(context.Context, CatalogMigrationPreviewRequest) (CatalogMigrationPreviewResult, error) {
		return preview, nil
	}}
	command := CatalogMigrationApplyCommand{PreviewRequest: CatalogMigrationPreviewRequest{RequestID: preview.RequestID, ContractIDs: []int64{contract.Id}}, CohortDigest: preview.CohortDigest, RequestedBy: 1}
	return service, command, contract, entitlement
}
