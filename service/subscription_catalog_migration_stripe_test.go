package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v86"
)

type fakeCatalogMigrationStripeProvider struct {
	subscription  *stripe.Subscription
	prices        map[string]*stripe.Price
	schedule      *stripe.SubscriptionSchedule
	createErr     error
	createCommits bool
	createErrOnce bool
	updateErr     error
	releaseErr    error
	createCalls   int
	updateCalls   int
	releaseCalls  int
	createKeys    []string
	updateKeys    []string
	releaseKeys   []string
	updateParams  *stripe.SubscriptionScheduleParams
}

func (f *fakeCatalogMigrationStripeProvider) GetSubscription(context.Context, string) (*stripe.Subscription, error) {
	return f.subscription, nil
}
func (f *fakeCatalogMigrationStripeProvider) GetPrice(_ context.Context, id string) (*stripe.Price, error) {
	price := f.prices[id]
	if price == nil {
		return nil, errors.New("price missing")
	}
	return price, nil
}
func (f *fakeCatalogMigrationStripeProvider) GetSchedule(context.Context, string) (*stripe.SubscriptionSchedule, error) {
	if f.schedule == nil {
		return nil, errors.New("schedule missing")
	}
	return f.schedule, nil
}
func (f *fakeCatalogMigrationStripeProvider) CreateSchedule(_ context.Context, subscriptionID string, _ map[string]string, key string) (*stripe.SubscriptionSchedule, error) {
	f.createCalls++
	f.createKeys = append(f.createKeys, key)
	if f.createErr == nil && f.schedule != nil && f.subscription.Schedule != nil && f.subscription.Schedule.ID == f.schedule.ID {
		// Stripe returns the original object when the same idempotency key is
		// replayed after a response loss.
		return f.schedule, nil
	}
	if f.createErr == nil || f.createCommits {
		// A real from_subscription create starts with one current phase and no
		// metadata; the scheduler adds the migration marker and target phase in
		// UpdateSchedule.
		f.schedule = &stripe.SubscriptionSchedule{
			ID:           "sched_catalog",
			Subscription: &stripe.Subscription{ID: subscriptionID},
			Phases: []*stripe.SubscriptionSchedulePhase{{
				StartDate:         f.subscription.Items.Data[0].CurrentPeriodStart,
				EndDate:           f.subscription.Items.Data[0].CurrentPeriodEnd,
				ProrationBehavior: stripe.SubscriptionSchedulePhaseProrationBehaviorCreateProrations,
				Items:             []*stripe.SubscriptionSchedulePhaseItem{{Price: &stripe.Price{ID: f.subscription.Items.Data[0].Price.ID}, Quantity: 1}},
			}},
		}
		f.subscription.Schedule = &stripe.SubscriptionSchedule{ID: f.schedule.ID}
	}
	if f.createErr != nil {
		err := f.createErr
		if f.createErrOnce {
			f.createErr = nil
		}
		return nil, err
	}
	return f.schedule, nil
}
func (f *fakeCatalogMigrationStripeProvider) UpdateSchedule(_ context.Context, id string, params *stripe.SubscriptionScheduleParams, key string) (*stripe.SubscriptionSchedule, error) {
	f.updateCalls++
	f.updateKeys = append(f.updateKeys, key)
	f.updateParams = params
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.schedule = scheduleFromCatalogMigrationParams(id, f.subscription.ID, params)
	return f.schedule, nil
}
func (f *fakeCatalogMigrationStripeProvider) ReleaseSchedule(_ context.Context, id, key string) (*stripe.SubscriptionSchedule, error) {
	f.releaseCalls++
	f.releaseKeys = append(f.releaseKeys, key)
	if f.releaseErr != nil {
		return nil, f.releaseErr
	}
	f.subscription.Schedule = nil
	return &stripe.SubscriptionSchedule{ID: id, Status: stripe.SubscriptionScheduleStatusReleased}, nil
}

func TestStripeCatalogMigrationSchedulerBuildsExactOwnedTwoPhaseSchedule(t *testing.T) {
	scheduler, provider, request := setupStripeCatalogMigrationSchedulerTest(t)
	result, err := scheduler.ScheduleCatalogMigration(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, "sched_catalog", result.ScheduleID)
	require.Equal(t, request.ExpectedOwnershipFingerprint, result.OwnershipFingerprint)
	require.Equal(t, 1, provider.createCalls)
	require.Equal(t, 1, provider.updateCalls)
	require.Equal(t, []string{request.IdempotencyKey + ":create"}, provider.createKeys)
	require.Equal(t, []string{request.IdempotencyKey + ":configure"}, provider.updateKeys)
	require.Equal(t, "release", stripe.StringValue(provider.updateParams.EndBehavior))
	require.Equal(t, "none", stripe.StringValue(provider.updateParams.ProrationBehavior))
	require.Len(t, provider.updateParams.Phases, 2)
	require.Equal(t, request.CurrentPeriodStart, stripe.Int64Value(provider.updateParams.Phases[0].StartDate))
	require.Equal(t, request.CurrentPeriodEnd, stripe.Int64Value(provider.updateParams.Phases[0].EndDate))
	require.Equal(t, request.CurrentPriceID, stripe.StringValue(provider.updateParams.Phases[0].Items[0].Price))
	require.Equal(t, request.CurrentPeriodEnd, stripe.Int64Value(provider.updateParams.Phases[1].StartDate))
	require.Equal(t, time.Unix(request.CurrentPeriodEnd, 0).UTC().AddDate(0, 1, 0).Unix(), stripe.Int64Value(provider.updateParams.Phases[1].EndDate))
	require.Equal(t, request.TargetPriceID, stripe.StringValue(provider.updateParams.Phases[1].Items[0].Price))
	require.Equal(t, request.BatchID, provider.updateParams.Metadata[catalogMigrationMetadataBatch])
	require.Equal(t, request.ExpectedOwnershipFingerprint, provider.updateParams.Metadata[catalogMigrationMetadataOwnership])

	replayed, err := scheduler.ScheduleCatalogMigration(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, result, replayed)
	require.Equal(t, 1, provider.createCalls)
	require.Equal(t, 1, provider.updateCalls)
	require.Equal(t, []string{request.IdempotencyKey + ":create"}, provider.createKeys)
	require.Equal(t, []string{request.IdempotencyKey + ":configure"}, provider.updateKeys)
}

func TestStripeCatalogMigrationSchedulerAdoptsCommittedCreateAfterResponseLoss(t *testing.T) {
	scheduler, provider, request := setupStripeCatalogMigrationSchedulerTest(t)
	provider.createErr = errors.New("response lost")
	provider.createCommits = true
	provider.createErrOnce = true

	result, err := scheduler.ScheduleCatalogMigration(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, "sched_catalog", result.ScheduleID)
	require.Equal(t, request.ExpectedOwnershipFingerprint, result.OwnershipFingerprint)
	require.Equal(t, 2, provider.createCalls, "the same idempotency key must be replayed after a response loss")
	require.Equal(t, 1, provider.updateCalls)
	require.Equal(t, []string{request.IdempotencyKey + ":create", request.IdempotencyKey + ":create"}, provider.createKeys)
	require.Equal(t, []string{request.IdempotencyKey + ":configure"}, provider.updateKeys)

	replayed, err := scheduler.ScheduleCatalogMigration(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, result, replayed)
	require.Equal(t, 2, provider.createCalls, "replay must not create a duplicate schedule")
	require.Equal(t, 1, provider.updateCalls, "an exact replay must not rewrite the schedule")
}

func TestStripeCatalogMigrationSchedulerRejectsTamperedOwnedSchedule(t *testing.T) {
	tests := map[string]func(*stripe.SubscriptionSchedule){
		"phase count": func(schedule *stripe.SubscriptionSchedule) {
			schedule.Phases = nil
		},
		"current price": func(schedule *stripe.SubscriptionSchedule) {
			schedule.Phases[0].Items[0].Price.ID = "price_tampered"
		},
		"target price": func(schedule *stripe.SubscriptionSchedule) {
			schedule.Phases[1].Items[0].Price.ID = "price_tampered"
		},
		"period": func(schedule *stripe.SubscriptionSchedule) {
			schedule.Phases[1].EndDate++
		},
		"ownership fingerprint": func(schedule *stripe.SubscriptionSchedule) {
			schedule.Metadata[catalogMigrationMetadataOwnership] = "tampered-fingerprint"
		},
	}
	for name, tamper := range tests {
		t.Run(name, func(t *testing.T) {
			scheduler, provider, request := setupStripeCatalogMigrationSchedulerTest(t)
			provider.schedule = ownedCatalogMigrationSchedule(request)
			provider.subscription.Schedule = &stripe.SubscriptionSchedule{ID: provider.schedule.ID}
			require.NoError(t, model.DB.Model(&model.SubscriptionProviderBinding{}).Where("id = ?", request.BindingID).Update("provider_schedule_id", provider.schedule.ID).Error)
			tamper(provider.schedule)

			result, err := scheduler.ScheduleCatalogMigration(context.Background(), request)
			require.ErrorContains(t, err, "unrelated")
			require.Empty(t, result.ScheduleID)
			require.Zero(t, provider.createCalls)
			require.Zero(t, provider.updateCalls)
		})
	}
}

func TestStripeCatalogMigrationSchedulerUpdateFailureIsFailClosedAndRetryable(t *testing.T) {
	scheduler, provider, request := setupStripeCatalogMigrationSchedulerTest(t)
	provider.updateErr = errors.New("update unavailable")

	result, err := scheduler.ScheduleCatalogMigration(context.Background(), request)
	require.ErrorContains(t, err, "update unavailable")
	require.Empty(t, result.ScheduleID)
	require.Equal(t, 1, provider.createCalls)
	require.Equal(t, 1, provider.updateCalls)

	provider.updateErr = nil
	result, err = scheduler.ScheduleCatalogMigration(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, "sched_catalog", result.ScheduleID)
	require.Equal(t, 2, provider.createCalls, "retry must replay the idempotent from_subscription create")
	require.Equal(t, 2, provider.updateCalls)
	require.Equal(t, []string{request.IdempotencyKey + ":configure", request.IdempotencyKey + ":configure"}, provider.updateKeys)
}

func TestStripeCatalogMigrationSchedulerRejectsFreshDriftBeforeMutation(t *testing.T) {
	tests := map[string]func(*fakeCatalogMigrationStripeProvider, CatalogMigrationProviderScheduleRequest){
		"subscription live": func(p *fakeCatalogMigrationStripeProvider, _ CatalogMigrationProviderScheduleRequest) {
			p.subscription.Livemode = true
		},
		"amount": func(p *fakeCatalogMigrationStripeProvider, r CatalogMigrationProviderScheduleRequest) {
			p.prices[r.TargetPriceID].UnitAmount++
		},
		"currency": func(p *fakeCatalogMigrationStripeProvider, r CatalogMigrationProviderScheduleRequest) {
			p.prices[r.TargetPriceID].Currency = stripe.CurrencyEUR
		},
		"price inactive": func(p *fakeCatalogMigrationStripeProvider, r CatalogMigrationProviderScheduleRequest) {
			p.prices[r.TargetPriceID].Active = false
		},
		"period": func(p *fakeCatalogMigrationStripeProvider, _ CatalogMigrationProviderScheduleRequest) {
			p.subscription.Items.Data[0].CurrentPeriodEnd++
		},
		"status": func(p *fakeCatalogMigrationStripeProvider, _ CatalogMigrationProviderScheduleRequest) {
			p.subscription.Status = stripe.SubscriptionStatusPastDue
		},
	}
	for name, drift := range tests {
		t.Run(name, func(t *testing.T) {
			scheduler, provider, request := setupStripeCatalogMigrationSchedulerTest(t)
			drift(provider, request)
			_, err := scheduler.ScheduleCatalogMigration(context.Background(), request)
			require.Error(t, err)
			require.Zero(t, provider.createCalls)
			require.Zero(t, provider.updateCalls)
			require.Zero(t, provider.releaseCalls)
		})
	}
}

func TestStripeCatalogMigrationSchedulerNeverAdoptsOrRestoresUnrelatedSchedule(t *testing.T) {
	scheduler, provider, request := setupStripeCatalogMigrationSchedulerTest(t)
	provider.subscription.Schedule = &stripe.SubscriptionSchedule{ID: "sched_other"}
	provider.schedule = ownedCatalogMigrationSchedule(request)
	provider.schedule.ID = "sched_other"
	provider.schedule.Metadata[catalogMigrationMetadataIntent] = "999"
	require.NoError(t, model.DB.Model(&model.SubscriptionProviderBinding{}).Where("id = ?", request.BindingID).Update("provider_schedule_id", "sched_other").Error)

	_, err := scheduler.ScheduleCatalogMigration(context.Background(), request)
	require.ErrorContains(t, err, "unrelated")
	require.Zero(t, provider.updateCalls)
	err = scheduler.RestoreCatalogMigration(context.Background(), request)
	require.ErrorContains(t, err, "not exactly owned")
	require.Zero(t, provider.releaseCalls)
}

func TestStripeCatalogMigrationRestoreReleasesExactOwnedAndAbsentReplayIsSafe(t *testing.T) {
	scheduler, provider, request := setupStripeCatalogMigrationSchedulerTest(t)
	provider.schedule = ownedCatalogMigrationSchedule(request)
	provider.subscription.Schedule = &stripe.SubscriptionSchedule{ID: provider.schedule.ID}
	require.NoError(t, model.DB.Model(&model.SubscriptionProviderBinding{}).Where("id = ?", request.BindingID).Update("provider_schedule_id", provider.schedule.ID).Error)
	require.NoError(t, scheduler.RestoreCatalogMigration(context.Background(), request))
	require.Equal(t, 1, provider.releaseCalls)
	require.NoError(t, scheduler.RestoreCatalogMigration(context.Background(), request))
	require.Equal(t, 1, provider.releaseCalls)
	require.Equal(t, []string{request.IdempotencyKey + ":restore-release"}, provider.releaseKeys)
}

func TestStripeCatalogMigrationRestoreReleaseFailureIsFailClosedAndRetryable(t *testing.T) {
	scheduler, provider, request := setupStripeCatalogMigrationSchedulerTest(t)
	provider.schedule = ownedCatalogMigrationSchedule(request)
	provider.subscription.Schedule = &stripe.SubscriptionSchedule{ID: provider.schedule.ID}
	require.NoError(t, model.DB.Model(&model.SubscriptionProviderBinding{}).Where("id = ?", request.BindingID).Update("provider_schedule_id", provider.schedule.ID).Error)
	provider.releaseErr = errors.New("release unavailable")

	err := scheduler.RestoreCatalogMigration(context.Background(), request)
	require.ErrorContains(t, err, "release unavailable")
	require.Equal(t, 1, provider.releaseCalls)
	require.NotNil(t, provider.subscription.Schedule, "failed release must not be reported as local success")

	provider.releaseErr = nil
	require.NoError(t, scheduler.RestoreCatalogMigration(context.Background(), request))
	require.Equal(t, 2, provider.releaseCalls)
	require.Equal(t, []string{request.IdempotencyKey + ":restore-release", request.IdempotencyKey + ":restore-release"}, provider.releaseKeys)
}

func TestStripeCatalogMigrationRestoreAbsentScheduleRequiresExactOldSubscriptionState(t *testing.T) {
	tests := map[string]func(*fakeCatalogMigrationStripeProvider, CatalogMigrationProviderScheduleRequest){
		"target price": func(provider *fakeCatalogMigrationStripeProvider, request CatalogMigrationProviderScheduleRequest) {
			provider.subscription.Items.Data[0].Price = provider.prices[request.TargetPriceID]
		},
		"other price": func(provider *fakeCatalogMigrationStripeProvider, _ CatalogMigrationProviderScheduleRequest) {
			provider.subscription.Items.Data[0].Price = &stripe.Price{ID: "price_other"}
		},
		"period drift": func(provider *fakeCatalogMigrationStripeProvider, _ CatalogMigrationProviderScheduleRequest) {
			provider.subscription.Items.Data[0].CurrentPeriodEnd++
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			scheduler, provider, request := setupStripeCatalogMigrationSchedulerTest(t)
			mutate(provider, request)

			err := scheduler.RestoreCatalogMigration(context.Background(), request)
			require.Error(t, err)
			require.Zero(t, provider.releaseCalls)
		})
	}
}

func TestCatalogMigrationUserActionSupersedesOnlyExactOwnedSchedule(t *testing.T) {
	service, command, contract, _ := setupCatalogMigrationApplyFixture(t, true)
	service.Scheduler = &catalogMigrationSchedulerStub{}
	result, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	var intent model.SubscriptionChangeIntent
	require.NoError(t, model.DB.Where("catalog_migration_batch_id = ?", result.Batch.Id).First(&intent).Error)
	var binding model.SubscriptionProviderBinding
	require.NoError(t, model.DB.First(&binding, contract.CurrentProviderBindingId).Error)
	request := catalogMigrationRequestFromStoredState(t, result.Batch.Id, contract.Id, intent.Id)
	provider := fakeProviderForStoredCatalogMigration(t, request)
	provider.schedule = ownedCatalogMigrationSchedule(request)
	provider.schedule.ID = binding.ProviderScheduleId
	provider.subscription.Schedule = &stripe.SubscriptionSchedule{ID: binding.ProviderScheduleId}
	sandbox := service.Sandbox
	scheduler := &stripeCatalogMigrationScheduler{provider: provider, sandbox: func() CatalogMigrationSandboxConfig { return sandbox }}
	original := catalogMigrationRuntimeProviderScheduler
	catalogMigrationRuntimeProviderScheduler = scheduler
	t.Cleanup(func() { catalogMigrationRuntimeProviderScheduler = original })

	version, superseded, err := supersedeCatalogMigrationForUserAction(context.Background(), contract.UserId, 0)
	require.NoError(t, err)
	require.True(t, superseded)
	require.Greater(t, version, contract.ChangeVersion)
	require.Equal(t, 1, provider.releaseCalls)
	require.NoError(t, model.DB.First(&contract, contract.Id).Error)
	require.Zero(t, contract.PendingPlanId)
	require.Zero(t, contract.PendingEffectiveAt)
	require.Equal(t, intent.PreviousChangeIntentId, contract.LatestChangeIntentId)
	require.NoError(t, model.DB.First(&intent, intent.Id).Error)
	require.Equal(t, model.SubscriptionChangeIntentStatusSuperseded, intent.Status)
	require.NoError(t, model.DB.First(&binding, binding.Id).Error)
	require.Empty(t, binding.ProviderScheduleId)
}

func TestCatalogMigrationUserActionBlocksAndMarksAttentionWhenOwnershipIsUnclear(t *testing.T) {
	service, command, contract, _ := setupCatalogMigrationApplyFixture(t, true)
	service.Scheduler = &catalogMigrationSchedulerStub{}
	result, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	var intent model.SubscriptionChangeIntent
	require.NoError(t, model.DB.Where("catalog_migration_batch_id = ?", result.Batch.Id).First(&intent).Error)
	require.NoError(t, model.DB.Model(&model.SubscriptionProviderBinding{}).Where("id = ?", contract.CurrentProviderBindingId).Update("provider_schedule_id", "sched_unrelated").Error)

	_, superseded, err := supersedeCatalogMigrationForUserAction(context.Background(), contract.UserId, 0)
	require.ErrorContains(t, err, "ownership is unclear")
	require.False(t, superseded)
	require.NoError(t, model.DB.First(&intent, intent.Id).Error)
	require.Equal(t, model.SubscriptionChangeIntentStatusNeedsAttention, intent.Status)
	require.NoError(t, model.DB.First(&contract, contract.Id).Error)
	require.Equal(t, model.SubscriptionContractStatusNeedsAttention, contract.Status)
}

func TestCatalogMigrationUserActionRestoreFailureDoesNotReportSuperseded(t *testing.T) {
	service, command, contract, _ := setupCatalogMigrationApplyFixture(t, true)
	service.Scheduler = &catalogMigrationSchedulerStub{}
	result, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	var intent model.SubscriptionChangeIntent
	require.NoError(t, model.DB.Where("catalog_migration_batch_id = ?", result.Batch.Id).First(&intent).Error)
	var binding model.SubscriptionProviderBinding
	require.NoError(t, model.DB.First(&binding, contract.CurrentProviderBindingId).Error)
	require.NoError(t, model.DB.First(&contract, contract.Id).Error)
	pendingPlanID := contract.PendingPlanId
	request := catalogMigrationRequestFromStoredState(t, result.Batch.Id, contract.Id, intent.Id)
	provider := fakeProviderForStoredCatalogMigration(t, request)
	provider.schedule = ownedCatalogMigrationSchedule(request)
	provider.schedule.ID = binding.ProviderScheduleId
	provider.subscription.Schedule = &stripe.SubscriptionSchedule{ID: binding.ProviderScheduleId}
	provider.releaseErr = errors.New("release unavailable")
	sandbox := service.Sandbox
	original := catalogMigrationRuntimeProviderScheduler
	catalogMigrationRuntimeProviderScheduler = &stripeCatalogMigrationScheduler{provider: provider, sandbox: func() CatalogMigrationSandboxConfig { return sandbox }}
	t.Cleanup(func() { catalogMigrationRuntimeProviderScheduler = original })

	version, superseded, err := supersedeCatalogMigrationForUserAction(context.Background(), contract.UserId, 0)
	require.ErrorContains(t, err, "release unavailable")
	require.False(t, superseded)
	require.Equal(t, contract.ChangeVersion, version)
	require.Equal(t, 1, provider.releaseCalls)
	require.NoError(t, model.DB.First(&intent, intent.Id).Error)
	require.Equal(t, model.SubscriptionChangeIntentStatusNeedsAttention, intent.Status)
	require.NoError(t, model.DB.First(&contract, contract.Id).Error)
	require.Equal(t, model.SubscriptionContractStatusNeedsAttention, contract.Status)
	require.Equal(t, pendingPlanID, contract.PendingPlanId)
	require.NoError(t, model.DB.First(&binding, binding.Id).Error)
	require.Equal(t, provider.schedule.ID, binding.ProviderScheduleId)
}

func TestCancelCurrentSubscriptionRenewalPreemptsCatalogMigrationThenCancels(t *testing.T) {
	service, command, contract, _ := setupCatalogMigrationApplyFixture(t, true)
	service.Scheduler = &catalogMigrationSchedulerStub{}
	result, err := service.Apply(context.Background(), command)
	require.NoError(t, err)
	var intent model.SubscriptionChangeIntent
	require.NoError(t, model.DB.Where("catalog_migration_batch_id = ?", result.Batch.Id).First(&intent).Error)
	request := catalogMigrationRequestFromStoredState(t, result.Batch.Id, contract.Id, intent.Id)
	provider := fakeProviderForStoredCatalogMigration(t, request)
	provider.schedule = ownedCatalogMigrationSchedule(request)
	provider.subscription.Schedule = &stripe.SubscriptionSchedule{ID: intent.ProviderScheduleId}
	provider.schedule.ID = intent.ProviderScheduleId
	sandbox := service.Sandbox
	originalScheduler := catalogMigrationRuntimeProviderScheduler
	catalogMigrationRuntimeProviderScheduler = &stripeCatalogMigrationScheduler{provider: provider, sandbox: func() CatalogMigrationSandboxConfig { return sandbox }}
	originalCancel := cancelCurrentStripeRecurringSubscription
	t.Cleanup(func() {
		catalogMigrationRuntimeProviderScheduler = originalScheduler
		cancelCurrentStripeRecurringSubscription = originalCancel
	})
	cancelCurrentStripeRecurringSubscription = func(_ int, bindingID int64, _ *currentStripeRenewalLifecycleMutationGuard) (*model.SubscriptionProviderBinding, error) {
		var binding model.SubscriptionProviderBinding
		require.NoError(t, model.DB.First(&binding, bindingID).Error)
		binding.CancelAtPeriodEnd = true
		return &binding, nil
	}
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("id = ?", contract.CurrentEntitlementId).Update("access_end_time", contract.CurrentPeriodEnd).Error)
	require.NoError(t, model.DB.First(&contract, contract.Id).Error)

	resultState, err := CancelCurrentSubscriptionRenewal(contract.UserId, renewalLifecyclePrecondition(contract, model.SubscriptionRenewalStatusEnabled))
	require.NoError(t, err)
	require.True(t, resultState.CancelAtPeriodEnd)
	require.Equal(t, 1, provider.releaseCalls)
	require.NoError(t, model.DB.First(&intent, intent.Id).Error)
	require.Equal(t, model.SubscriptionChangeIntentStatusSuperseded, intent.Status)
	require.NoError(t, model.DB.First(&contract, contract.Id).Error)
	require.Zero(t, contract.PendingPlanId)
}

func setupStripeCatalogMigrationSchedulerTest(t *testing.T) (*stripeCatalogMigrationScheduler, *fakeCatalogMigrationStripeProvider, CatalogMigrationProviderScheduleRequest) {
	t.Helper()
	_, _, contract, _ := setupCatalogMigrationApplyFixture(t, true)
	var source, target model.SubscriptionPlan
	require.NoError(t, model.DB.First(&source, contract.CurrentPlanId).Error)
	require.NoError(t, model.DB.First(&target, 2).Error)
	currentSnapshot, err := recurringSnapshotFromPlanOnly(&source)
	require.NoError(t, err)
	targetSnapshot, err := recurringSnapshotFromPlanOnly(&target)
	require.NoError(t, err)
	currentRaw, err := EncodeRecurringPlanSnapshotV1(currentSnapshot)
	require.NoError(t, err)
	targetRaw, err := EncodeRecurringPlanSnapshotV1(targetSnapshot)
	require.NoError(t, err)
	request := CatalogMigrationProviderScheduleRequest{BatchID: "batch_test", IntentID: 71, ContractID: contract.Id, UserID: contract.UserId, ChangeVersion: contract.ChangeVersion, BindingID: contract.CurrentProviderBindingId, ProviderSubscriptionID: "sub_test", ProviderSubscriptionItemID: "si_test", ProviderCustomerID: "cus_test", CurrentPriceID: source.StripePriceId, TargetPriceID: target.StripePriceId, CurrentPeriodStart: contract.CurrentPeriodStart, CurrentPeriodEnd: contract.CurrentPeriodEnd, IdempotencyKey: "catalog-idempotency", ExpectedOwnershipFingerprint: "ownership-fingerprint", CurrentPlanSnapshot: currentRaw, TargetPlanSnapshot: targetRaw}
	request.LifecycleActionSeq = 1
	request.LifecycleReservationToken = "catalog-reservation"
	request.LifecycleReservationUntil = time.Now().Unix() + 3600
	require.NoError(t, model.DB.Model(&model.SubscriptionProviderBinding{}).Where("id = ?", request.BindingID).Updates(map[string]any{"lifecycle_action_seq": request.LifecycleActionSeq, "lifecycle_reservation_action": model.SubscriptionProviderLifecycleActionCatalogMigration, "lifecycle_reservation_token": request.LifecycleReservationToken, "lifecycle_reservation_until": request.LifecycleReservationUntil}).Error)
	provider := &fakeCatalogMigrationStripeProvider{prices: map[string]*stripe.Price{
		request.CurrentPriceID: testCatalogMigrationPrice(request.CurrentPriceID, currentSnapshot),
		request.TargetPriceID:  testCatalogMigrationPrice(request.TargetPriceID, targetSnapshot),
	}}
	provider.subscription = &stripe.Subscription{ID: request.ProviderSubscriptionID, Customer: &stripe.Customer{ID: request.ProviderCustomerID}, Status: stripe.SubscriptionStatusActive, Items: &stripe.SubscriptionItemList{Data: []*stripe.SubscriptionItem{{ID: request.ProviderSubscriptionItemID, Price: provider.prices[request.CurrentPriceID], CurrentPeriodStart: request.CurrentPeriodStart, CurrentPeriodEnd: request.CurrentPeriodEnd}}}}
	sandbox := CatalogMigrationSandboxConfig{DeploymentEnvironment: "staging", ServiceName: "newapi-staging", FeatureEnabled: true, AllowedContractIDs: []int64{request.ContractID}, StripeSecret: "sk_test_fake", StripePublishableKey: "pk_test_fake"}
	return &stripeCatalogMigrationScheduler{provider: provider, sandbox: func() CatalogMigrationSandboxConfig { return sandbox }}, provider, request
}

func catalogMigrationRequestFromStoredState(t *testing.T, batchID string, contractID, intentID int64) CatalogMigrationProviderScheduleRequest {
	t.Helper()
	var contract model.UserSubscriptionContract
	var intent model.SubscriptionChangeIntent
	var binding model.SubscriptionProviderBinding
	require.NoError(t, model.DB.First(&contract, contractID).Error)
	require.NoError(t, model.DB.First(&intent, intentID).Error)
	require.NoError(t, model.DB.First(&binding, intent.ProviderBindingId).Error)
	target, err := DecodeRecurringPlanSnapshotV1(intent.TargetPlanSnapshot)
	require.NoError(t, err)
	return CatalogMigrationProviderScheduleRequest{BatchID: batchID, IntentID: intent.Id, ContractID: contract.Id, UserID: contract.UserId, ChangeVersion: contract.ChangeVersion, BindingID: binding.Id, ProviderSubscriptionID: binding.ProviderSubscriptionId, ProviderSubscriptionItemID: binding.ProviderSubscriptionItemId, ProviderCustomerID: binding.ProviderCustomerId, CurrentPriceID: binding.ProviderPriceId, TargetPriceID: target.StripePriceID, CurrentPeriodStart: binding.CurrentPeriodStart, CurrentPeriodEnd: binding.CurrentPeriodEnd, IdempotencyKey: intent.ProviderIdempotencyKey, ExpectedOwnershipFingerprint: intent.ProviderScheduleFingerprint, CurrentPlanSnapshot: binding.CurrentPlanSnapshot, TargetPlanSnapshot: intent.TargetPlanSnapshot}
}

func fakeProviderForStoredCatalogMigration(t *testing.T, request CatalogMigrationProviderScheduleRequest) *fakeCatalogMigrationStripeProvider {
	t.Helper()
	current, err := DecodeRecurringPlanSnapshotV1(request.CurrentPlanSnapshot)
	require.NoError(t, err)
	target, err := DecodeRecurringPlanSnapshotV1(request.TargetPlanSnapshot)
	require.NoError(t, err)
	prices := map[string]*stripe.Price{request.CurrentPriceID: testCatalogMigrationPrice(request.CurrentPriceID, current), request.TargetPriceID: testCatalogMigrationPrice(request.TargetPriceID, target)}
	return &fakeCatalogMigrationStripeProvider{prices: prices, subscription: &stripe.Subscription{ID: request.ProviderSubscriptionID, Customer: &stripe.Customer{ID: request.ProviderCustomerID}, Status: stripe.SubscriptionStatusActive, Items: &stripe.SubscriptionItemList{Data: []*stripe.SubscriptionItem{{ID: request.ProviderSubscriptionItemID, Price: prices[request.CurrentPriceID], CurrentPeriodStart: request.CurrentPeriodStart, CurrentPeriodEnd: request.CurrentPeriodEnd}}}}}
}

func testCatalogMigrationPrice(id string, snapshot RecurringPlanSnapshotV1) *stripe.Price {
	return &stripe.Price{ID: id, Active: true, Currency: stripe.Currency(snapshot.Currency), UnitAmount: snapshot.BasePriceMinor, Type: stripe.PriceTypeRecurring, Recurring: &stripe.PriceRecurring{Interval: stripe.PriceRecurringIntervalMonth, IntervalCount: 1}}
}

func ownedCatalogMigrationSchedule(request CatalogMigrationProviderScheduleRequest) *stripe.SubscriptionSchedule {
	params := catalogMigrationScheduleParams(request, catalogMigrationScheduleMetadata(request), &stripe.Price{ID: request.TargetPriceID})
	return scheduleFromCatalogMigrationParams("sched_catalog", request.ProviderSubscriptionID, params)
}

func scheduleFromCatalogMigrationParams(id, subscriptionID string, params *stripe.SubscriptionScheduleParams) *stripe.SubscriptionSchedule {
	result := &stripe.SubscriptionSchedule{ID: id, Subscription: &stripe.Subscription{ID: subscriptionID}, Metadata: params.Metadata, EndBehavior: stripe.SubscriptionScheduleEndBehavior(stripe.StringValue(params.EndBehavior)), Status: stripe.SubscriptionScheduleStatusActive}
	for _, phase := range params.Phases {
		result.Phases = append(result.Phases, &stripe.SubscriptionSchedulePhase{StartDate: stripe.Int64Value(phase.StartDate), EndDate: stripe.Int64Value(phase.EndDate), ProrationBehavior: stripe.SubscriptionSchedulePhaseProrationBehavior(stripe.StringValue(phase.ProrationBehavior)), Items: []*stripe.SubscriptionSchedulePhaseItem{{Price: &stripe.Price{ID: stripe.StringValue(phase.Items[0].Price)}, Quantity: stripe.Int64Value(phase.Items[0].Quantity)}}})
	}
	return result
}
