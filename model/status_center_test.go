package model

import (
	"errors"
	"strconv"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupStatusCenterStoreTest(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := DB
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=busy_timeout(5000)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(StatusCenterModels()...))
	DB = db
	t.Cleanup(func() { DB = originalDB })
	return db
}

func TestStatusCenterUniqueConstraints(t *testing.T) {
	db := setupStatusCenterStoreTest(t)
	component := StatusComponent{ComponentKey: "router", Slug: "router", Kind: StatusComponentKindRouter, DisplayName: "Router", Lifecycle: StatusLifecycleActive, ObservedStatus: StatusUnknown, EffectiveStatus: StatusUnknown, Version: 1}
	require.NoError(t, db.Create(&component).Error)
	require.Error(t, db.Create(&StatusComponent{ComponentKey: "router", Slug: "router-2", Kind: StatusComponentKindRouter}).Error)
	require.Error(t, db.Create(&StatusComponent{ComponentKey: "model:gpt-5", Slug: "router", Kind: StatusComponentKindModel}).Error)

	require.NoError(t, db.Create(&StatusPeriod{ComponentID: component.ID, Granularity: StatusGranularityFiveMinutes, PeriodStart: 100}).Error)
	require.Error(t, db.Create(&StatusPeriod{ComponentID: component.ID, Granularity: StatusGranularityFiveMinutes, PeriodStart: 100}).Error)

	require.NoError(t, db.Create(&StatusIncident{PublicID: "inc-1", Kind: StatusIncidentKindIncident, IdempotencyKey: "router:outage:100", Version: 1}).Error)
	require.Error(t, db.Create(&StatusIncident{PublicID: "inc-2", Kind: StatusIncidentKindIncident, IdempotencyKey: "router:outage:100", Version: 1}).Error)

	require.NoError(t, db.Create(&StatusSubscriber{Kind: StatusSubscriberKindEmail, IdentityHash: "hash", Status: StatusSubscriberPending}).Error)
	require.Error(t, db.Create(&StatusSubscriber{Kind: StatusSubscriberKindEmail, IdentityHash: "hash", Status: StatusSubscriberPending}).Error)

	require.NoError(t, db.Create(&StatusDeliveryOutbox{PublishedUpdateID: 7, DestinationType: StatusDestinationWebhook, DestinationID: 9, EventID: "evt-1", Status: StatusDeliveryPending}).Error)
	require.Error(t, db.Create(&StatusDeliveryOutbox{PublishedUpdateID: 7, DestinationType: StatusDestinationWebhook, DestinationID: 9, EventID: "evt-2", Status: StatusDeliveryPending}).Error)
}

func TestStatusJobLeaseTakeoverIncrementsFence(t *testing.T) {
	setupStatusCenterStoreTest(t)

	first, acquired, err := AcquireStatusJobLease("evaluate", "node-a", 100, 10)
	require.NoError(t, err)
	require.True(t, acquired)
	require.EqualValues(t, 1, first.FencingToken)

	held, acquired, err := AcquireStatusJobLease("evaluate", "node-b", 105, 10)
	require.NoError(t, err)
	require.False(t, acquired)
	require.Equal(t, "node-a", held.Holder)

	taken, acquired, err := AcquireStatusJobLease("evaluate", "node-b", 111, 10)
	require.NoError(t, err)
	require.True(t, acquired)
	require.Equal(t, "node-b", taken.Holder)
	require.EqualValues(t, 2, taken.FencingToken)
}

func TestStatusJobLeaseRenewalUsesCurrentFence(t *testing.T) {
	setupStatusCenterStoreTest(t)
	first, acquired, err := AcquireStatusJobLease("evaluate", "node-a", 100, 10)
	require.NoError(t, err)
	require.True(t, acquired)

	renewed, err := RenewStatusJobLease("evaluate", "node-a", first.FencingToken, 105, 10)
	require.NoError(t, err)
	require.True(t, renewed)
	var lease StatusJobLease
	require.NoError(t, DB.First(&lease, "name = ?", "evaluate").Error)
	require.EqualValues(t, first.FencingToken, lease.FencingToken)
	require.EqualValues(t, 115, lease.ExpiresAt)

	_, acquired, err = AcquireStatusJobLease("evaluate", "node-b", 116, 10)
	require.NoError(t, err)
	require.True(t, acquired)
	renewed, err = RenewStatusJobLease("evaluate", "node-a", first.FencingToken, 116, 10)
	require.NoError(t, err)
	require.False(t, renewed)
}

func TestStatusComponentCommitRejectsStaleFence(t *testing.T) {
	setupStatusCenterStoreTest(t)
	first, acquired, err := AcquireStatusJobLease("evaluate", "node-a", 100, 10)
	require.NoError(t, err)
	require.True(t, acquired)

	component := StatusComponent{ComponentKey: "router", Slug: "router", Kind: StatusComponentKindRouter, Version: 1}
	require.NoError(t, DB.Create(&component).Error)
	component.DisplayName = "stale write"

	_, acquired, err = AcquireStatusJobLease("evaluate", "node-b", 111, 10)
	require.NoError(t, err)
	require.True(t, acquired)
	require.Error(t, CommitStatusComponentWithFence("evaluate", "node-a", first.FencingToken, 111, &component))

	var stored StatusComponent
	require.NoError(t, DB.First(&stored, component.ID).Error)
	require.NotEqual(t, "stale write", stored.DisplayName)
}

func TestModelAvailabilitySaveRejectsStaleStatusFence(t *testing.T) {
	db := setupStatusCenterStoreTest(t)
	require.NoError(t, db.AutoMigrate(&ModelAvailabilityState{}))
	first, acquired, err := AcquireStatusJobLease("evaluate", "node-a", 100, 10)
	require.NoError(t, err)
	require.True(t, acquired)

	require.NoError(t, SaveModelAvailabilityStateWithFence("evaluate", "node-a", first.FencingToken, 100, &ModelAvailabilityState{
		ModelName:     "gpt-test",
		Status:        ModelAvailabilityAvailable,
		LastCheckedAt: 100,
	}))

	_, acquired, err = AcquireStatusJobLease("evaluate", "node-b", 111, 10)
	require.NoError(t, err)
	require.True(t, acquired)
	require.Error(t, SaveModelAvailabilityStateWithFence("evaluate", "node-a", first.FencingToken, 111, &ModelAvailabilityState{
		ModelName:     "gpt-test",
		Status:        ModelAvailabilityTemporaryFailure,
		LastCheckedAt: 111,
	}))

	var stored ModelAvailabilityState
	require.NoError(t, db.First(&stored, "model_name = ?", "gpt-test").Error)
	require.Equal(t, ModelAvailabilityAvailable, stored.Status)
	require.EqualValues(t, 100, stored.LastCheckedAt)
}

func TestStatusComponentVersionConflict(t *testing.T) {
	setupStatusCenterStoreTest(t)
	component := StatusComponent{ComponentKey: "router", Slug: "router", Kind: StatusComponentKindRouter, Version: 1}
	require.NoError(t, DB.Create(&component).Error)

	updated, err := UpdateStatusComponentVersion(component.ID, 1, map[string]any{"display_name": "Public Router"})
	require.NoError(t, err)
	require.EqualValues(t, 2, updated.Version)
	require.Equal(t, "Public Router", updated.DisplayName)

	_, err = UpdateStatusComponentVersion(component.ID, 1, map[string]any{"display_name": "stale"})
	require.True(t, errors.Is(err, ErrStatusVersionConflict))
}

func TestRetryDeadStatusDeliveryRequeuesAndReactivatesSubscriber(t *testing.T) {
	db := setupStatusCenterStoreTest(t)
	subscriber := StatusSubscriber{
		Kind: StatusSubscriberKindWebhook, IdentityHash: "retry-webhook", Status: StatusSubscriberSuspended,
		FailureCount: 3, SuspendedAt: 1_900, CreatedAt: 1_000, UpdatedAt: 1_900,
	}
	require.NoError(t, db.Create(&subscriber).Error)
	delivery := StatusDeliveryOutbox{
		PublishedUpdateID: 11, DestinationType: StatusDestinationWebhook, DestinationID: subscriber.ID,
		EventID: "delivery-retry-webhook", Payload: `{"event":"preserved"}`, Status: StatusDeliveryDead,
		LockToken: "stale-lock", LockedUntil: 9_999, Attempts: 3, NextAttemptAt: 8_888,
		LastError: "endpoint gone", Version: 4, CreatedAt: 1_000, UpdatedAt: 1_900,
	}
	require.NoError(t, db.Create(&delivery).Error)

	retried, err := RetryDeadStatusDelivery(StatusDeliveryRetryMutation{
		ID: delivery.ID, ExpectedVersion: 4, Now: 2_000,
		Audit: StatusAuditMutation{ActorID: 1, ActorType: "root", Action: "status.delivery.retry", Reason: "operator retry", CreatedAt: 2_000},
	})
	require.NoError(t, err)
	require.Equal(t, StatusDeliveryPending, retried.Status)
	require.EqualValues(t, 5, retried.Version)
	require.Empty(t, retried.LockToken)
	require.Zero(t, retried.LockedUntil)
	require.EqualValues(t, 2_000, retried.NextAttemptAt)
	require.Empty(t, retried.LastError)
	require.EqualValues(t, 3, retried.Attempts)
	require.Equal(t, delivery.PublishedUpdateID, retried.PublishedUpdateID)
	require.Equal(t, delivery.DestinationType, retried.DestinationType)
	require.Equal(t, delivery.DestinationID, retried.DestinationID)
	require.Equal(t, delivery.EventID, retried.EventID)
	require.Equal(t, delivery.Payload, retried.Payload)

	require.NoError(t, db.First(&subscriber, subscriber.ID).Error)
	require.Equal(t, StatusSubscriberActive, subscriber.Status)
	require.Zero(t, subscriber.FailureCount)
	require.Zero(t, subscriber.SuspendedAt)

	var audit StatusAuditEvent
	require.NoError(t, db.Where("action = ?", "status.delivery.retry").First(&audit).Error)
	require.Equal(t, "delivery", audit.ObjectType)
	require.Equal(t, strconv.FormatInt(delivery.ID, 10), audit.ObjectID)
	require.Equal(t, "operator retry", audit.Reason)
	require.NotContains(t, audit.BeforeJSON, delivery.Payload)
	require.NotContains(t, audit.AfterJSON, delivery.Payload)
	require.NotContains(t, audit.BeforeJSON, delivery.LastError)
	require.NotContains(t, audit.AfterJSON, delivery.LastError)

	_, err = RetryDeadStatusDelivery(StatusDeliveryRetryMutation{
		ID: delivery.ID, ExpectedVersion: 4, Now: 2_001,
		Audit: StatusAuditMutation{ActorID: 1, ActorType: "root", Action: "status.delivery.retry", Reason: "stale retry", CreatedAt: 2_001},
	})
	require.ErrorIs(t, err, ErrStatusVersionConflict)
}

func TestRetryDeadStatusDeliveryReactivatesDiscordState(t *testing.T) {
	db := setupStatusCenterStoreTest(t)
	require.NoError(t, db.Create(&StatusSetting{
		Key: statusDiscordDeliveryStateSettingKey, Value: `{"failure_count":3,"suspended_at":1900}`,
		Version: 2, UpdatedAt: 1_900,
	}).Error)
	delivery := StatusDeliveryOutbox{
		PublishedUpdateID: 12, DestinationType: StatusDestinationDiscord, DestinationID: 0,
		EventID: "delivery-retry-discord", Payload: `{"event":"preserved"}`, Status: StatusDeliveryDead,
		LastError: "discord gone", Version: 2, CreatedAt: 1_000, UpdatedAt: 1_900,
	}
	require.NoError(t, db.Create(&delivery).Error)

	_, err := RetryDeadStatusDelivery(StatusDeliveryRetryMutation{
		ID: delivery.ID, ExpectedVersion: 2, Now: 2_000,
		Audit: StatusAuditMutation{ActorID: 1, ActorType: "root", Action: "status.delivery.retry", Reason: "operator retry", CreatedAt: 2_000},
	})
	require.NoError(t, err)
	state, err := GetStatusDiscordDeliveryState()
	require.NoError(t, err)
	require.Equal(t, StatusDiscordDeliveryState{}, state)
}

func TestRetryDeadStatusDeliveryRejectsUndeliverableDestinationAtomically(t *testing.T) {
	db := setupStatusCenterStoreTest(t)
	subscriber := StatusSubscriber{
		Kind: StatusSubscriberKindEmail, IdentityHash: "retry-unsubscribed", Status: StatusSubscriberUnsubscribed,
		FailureCount: 3, SuspendedAt: 1_900, CreatedAt: 1_000, UpdatedAt: 1_900,
	}
	require.NoError(t, db.Create(&subscriber).Error)
	delivery := StatusDeliveryOutbox{
		PublishedUpdateID: 13, DestinationType: StatusDestinationEmail, DestinationID: subscriber.ID,
		EventID: "delivery-retry-unsubscribed", Payload: `{"event":"preserved"}`, Status: StatusDeliveryDead,
		LastError: "failed", Version: 2, CreatedAt: 1_000, UpdatedAt: 1_900,
	}
	require.NoError(t, db.Create(&delivery).Error)

	_, err := RetryDeadStatusDelivery(StatusDeliveryRetryMutation{
		ID: delivery.ID, ExpectedVersion: 2, Now: 2_000,
		Audit: StatusAuditMutation{ActorID: 1, ActorType: "root", Action: "status.delivery.retry", Reason: "operator retry", CreatedAt: 2_000},
	})
	require.ErrorIs(t, err, ErrStatusInvalidDeliveryMutation)

	var stored StatusDeliveryOutbox
	require.NoError(t, db.First(&stored, delivery.ID).Error)
	require.Equal(t, StatusDeliveryDead, stored.Status)
	require.EqualValues(t, 2, stored.Version)
	require.Equal(t, "failed", stored.LastError)
	require.NoError(t, db.First(&subscriber, subscriber.ID).Error)
	require.Equal(t, StatusSubscriberUnsubscribed, subscriber.Status)
	var audits int64
	require.NoError(t, db.Model(&StatusAuditEvent{}).Where("action = ?", "status.delivery.retry").Count(&audits).Error)
	require.Zero(t, audits)
}

func TestRetryDeadStatusDeliveryRejectsNonDeadDelivery(t *testing.T) {
	db := setupStatusCenterStoreTest(t)
	delivery := StatusDeliveryOutbox{
		PublishedUpdateID: 14, DestinationType: StatusDestinationDiscord,
		EventID: "delivery-retry-pending", Payload: `{"event":"preserved"}`, Status: StatusDeliveryPending,
		Version: 2, CreatedAt: 1_000, UpdatedAt: 1_900,
	}
	require.NoError(t, db.Create(&delivery).Error)

	_, err := RetryDeadStatusDelivery(StatusDeliveryRetryMutation{
		ID: delivery.ID, ExpectedVersion: 2, Now: 2_000,
		Audit: StatusAuditMutation{ActorID: 1, ActorType: "root", Action: "status.delivery.retry", Reason: "operator retry", CreatedAt: 2_000},
	})
	require.ErrorIs(t, err, ErrStatusInvalidDeliveryMutation)

	var stored StatusDeliveryOutbox
	require.NoError(t, db.First(&stored, delivery.ID).Error)
	require.Equal(t, StatusDeliveryPending, stored.Status)
	require.EqualValues(t, 2, stored.Version)
}

func TestUpsertStatusPeriodReplacesComputedAggregate(t *testing.T) {
	setupStatusCenterStoreTest(t)
	component := StatusComponent{ComponentKey: "router", Slug: "router", Kind: StatusComponentKindRouter, Version: 1}
	require.NoError(t, DB.Create(&component).Error)

	require.NoError(t, UpsertStatusPeriod(&StatusPeriod{ComponentID: component.ID, Granularity: StatusGranularityHour, PeriodStart: 3600, ScoreSumMicros: 900_000, KnownBucketCount: 1, WorstStatus: StatusDegraded}))
	require.NoError(t, UpsertStatusPeriod(&StatusPeriod{ComponentID: component.ID, Granularity: StatusGranularityHour, PeriodStart: 3600, ScoreSumMicros: 1_900_000, KnownBucketCount: 2, WorstStatus: StatusOperational}))

	var periods []StatusPeriod
	require.NoError(t, DB.Find(&periods).Error)
	require.Len(t, periods, 1)
	require.EqualValues(t, 1_900_000, periods[0].ScoreSumMicros)
	require.EqualValues(t, 2, periods[0].KnownBucketCount)
	require.Equal(t, StatusOperational, periods[0].WorstStatus)
}
