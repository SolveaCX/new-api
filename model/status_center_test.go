package model

import (
	"errors"
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
