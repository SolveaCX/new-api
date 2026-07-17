package service

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func createStatusIncidentTestComponent(t *testing.T, observed string) model.StatusComponent {
	t.Helper()
	component := model.StatusComponent{
		ComponentKey:    "model:test-" + common.GetUUID(),
		Slug:            "test-" + common.GetUUID(),
		Kind:            model.StatusComponentKindModel,
		DisplayName:     "Test Model",
		Lifecycle:       model.StatusLifecycleActive,
		ObservedStatus:  observed,
		EffectiveStatus: observed,
		StatusSource:    "observed",
		Version:         1,
		CreatedAt:       1_000,
		UpdatedAt:       1_000,
	}
	require.NoError(t, model.DB.Create(&component).Error)
	return component
}

func statusAdminActor() StatusMutationActor {
	return StatusMutationActor{ID: 17, Role: common.RoleAdminUser, ActorType: "admin"}
}

func statusRootActor(verified bool) StatusMutationActor {
	return StatusMutationActor{ID: 1, Role: common.RoleRootUser, ActorType: "root", SecureVerified: verified}
}

func TestStatusIncidentAutomationCreatesAndUpdatesPrivateDraft(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOperational)

	created, err := ReconcileStatusIncidentAutomation(StatusIncidentAutomationInput{
		ComponentID:            component.ID,
		PreviousObservedStatus: model.StatusOperational,
		ObservedStatus:         model.StatusDegraded,
		EvidenceSummary:        "upstream timeout Authorization: Bearer sk-secret-token",
		IdempotencyKey:         "automatic:test-model:episode-1",
		Now:                    2_000,
	})
	require.NoError(t, err)
	require.NotNil(t, created.Incident)
	require.NotNil(t, created.Draft)
	require.Equal(t, "private", created.Incident.Visibility)
	require.Equal(t, "draft", created.Incident.Status)
	require.Equal(t, "automatic", created.Incident.AutomationMode)
	require.False(t, created.Draft.Published)
	require.Contains(t, created.Draft.Body, "upstream timeout")
	require.NotContains(t, created.Draft.Body, "sk-secret-token")

	updated, err := ReconcileStatusIncidentAutomation(StatusIncidentAutomationInput{
		ComponentID:            component.ID,
		PreviousObservedStatus: model.StatusDegraded,
		ObservedStatus:         model.StatusOutage,
		EvidenceSummary:        "probe and traffic signals confirm the outage",
		IdempotencyKey:         "automatic:test-model:episode-1",
		Now:                    2_060,
	})
	require.NoError(t, err)
	require.Equal(t, created.Incident.ID, updated.Incident.ID)
	require.Equal(t, created.Draft.ID, updated.Draft.ID)
	require.Equal(t, "outage", updated.Incident.Impact)
	require.Contains(t, updated.Draft.Body, "confirm the outage")

	var incidentCount int64
	require.NoError(t, db.Model(&model.StatusIncident{}).Count(&incidentCount).Error)
	require.EqualValues(t, 1, incidentCount)
	var publishedCount int64
	require.NoError(t, db.Model(&model.StatusIncidentUpdate{}).Where("published = ?", true).Count(&publishedCount).Error)
	require.Zero(t, publishedCount)
}

func TestStatusIncidentRecoverySuggestsResolutionWithoutEditingPublishedText(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOperational)
	created, err := ReconcileStatusIncidentAutomation(StatusIncidentAutomationInput{
		ComponentID:            component.ID,
		PreviousObservedStatus: model.StatusOperational,
		ObservedStatus:         model.StatusOutage,
		EvidenceSummary:        "traffic failure confirmed",
		IdempotencyKey:         "automatic:test-model:episode-2",
		Now:                    3_000,
	})
	require.NoError(t, err)

	published, err := PublishStatusIncidentUpdate(StatusIncidentPublishInput{
		IncidentID:      created.Incident.ID,
		ExpectedVersion: created.Incident.Version,
		State:           "investigating",
		Body:            "We are investigating elevated errors.",
		EventID:         "incident-published-1",
		Actor:           statusAdminActor(),
		Reason:          "approved public wording",
		Now:             3_010,
	})
	require.NoError(t, err)

	recovery, err := ReconcileStatusIncidentAutomation(StatusIncidentAutomationInput{
		ComponentID:            component.ID,
		PreviousObservedStatus: model.StatusOutage,
		ObservedStatus:         model.StatusOperational,
		EvidenceSummary:        "healthy traffic recovered",
		IdempotencyKey:         "automatic:test-model:episode-2",
		Now:                    3_100,
	})
	require.NoError(t, err)
	require.NotNil(t, recovery.Draft)
	require.False(t, recovery.Draft.Published)
	require.Equal(t, "resolved", recovery.Draft.State)
	require.Contains(t, recovery.Draft.Body, "healthy traffic recovered")

	var immutable model.StatusIncidentUpdate
	require.NoError(t, db.First(&immutable, published.Update.ID).Error)
	require.True(t, immutable.Published)
	require.Equal(t, "We are investigating elevated errors.", immutable.Body)
	var deliveryCount int64
	require.NoError(t, db.Model(&model.StatusDeliveryOutbox{}).Count(&deliveryCount).Error)
	require.Zero(t, deliveryCount, "automation must not queue public notifications")
}

func TestStatusIncidentPublishAppendsUpdatesAndIdempotentDeliveries(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOperational)
	created, err := ReconcileStatusIncidentAutomation(StatusIncidentAutomationInput{
		ComponentID:            component.ID,
		PreviousObservedStatus: model.StatusOperational,
		ObservedStatus:         model.StatusDegraded,
		EvidenceSummary:        "degraded traffic",
		IdempotencyKey:         "automatic:test-model:episode-3",
		Now:                    4_000,
	})
	require.NoError(t, err)

	destinations := []StatusDeliveryDestination{
		{Type: model.StatusDestinationEmail, ID: 10},
		{Type: model.StatusDestinationWebhook, ID: 20},
		{Type: model.StatusDestinationEmail, ID: 10},
	}
	first, err := PublishStatusIncidentUpdate(StatusIncidentPublishInput{
		IncidentID:      created.Incident.ID,
		ExpectedVersion: created.Incident.Version,
		State:           "investigating",
		Body:            "Investigating elevated errors.",
		EventID:         "incident-published-2",
		Destinations:    destinations,
		Actor:           statusAdminActor(),
		Reason:          "first public update",
		Now:             4_010,
	})
	require.NoError(t, err)
	require.True(t, first.Update.Published)

	second, err := PublishStatusIncidentUpdate(StatusIncidentPublishInput{
		IncidentID:      created.Incident.ID,
		ExpectedVersion: first.Incident.Version,
		State:           "identified",
		Body:            "The failing dependency has been identified.",
		EventID:         "incident-published-3",
		Destinations:    destinations,
		Actor:           statusAdminActor(),
		Reason:          "correction and diagnosis",
		Now:             4_020,
	})
	require.NoError(t, err)

	var updates []model.StatusIncidentUpdate
	require.NoError(t, db.Where("incident_id = ? AND published = ?", created.Incident.ID, true).Order("id ASC").Find(&updates).Error)
	require.Len(t, updates, 2)
	require.Equal(t, "Investigating elevated errors.", updates[0].Body)
	require.Equal(t, "The failing dependency has been identified.", updates[1].Body)
	require.NotEqual(t, updates[0].ID, updates[1].ID)

	var deliveries []model.StatusDeliveryOutbox
	require.NoError(t, db.Order("id ASC").Find(&deliveries).Error)
	require.Len(t, deliveries, 4, "duplicate destinations must collapse to one logical delivery per update")
	for _, delivery := range deliveries {
		require.Equal(t, model.StatusDeliveryPending, delivery.Status)
		require.NotEmpty(t, delivery.EventID)
		require.NotEmpty(t, delivery.Payload)
	}
	require.EqualValues(t, 3, second.Incident.Version)
}

func TestStatusIncidentPublishRollsBackWhenDeliveryInsertFails(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOperational)
	created, err := ReconcileStatusIncidentAutomation(StatusIncidentAutomationInput{
		ComponentID:            component.ID,
		PreviousObservedStatus: model.StatusOperational,
		ObservedStatus:         model.StatusOutage,
		EvidenceSummary:        "outage",
		IdempotencyKey:         "automatic:test-model:episode-4",
		Now:                    5_000,
	})
	require.NoError(t, err)

	conflictingEventID := statusDeliveryEventID("incident-published-4", model.StatusDestinationWebhook, 30)
	require.NoError(t, db.Create(&model.StatusDeliveryOutbox{
		PublishedUpdateID: 999,
		DestinationType:   model.StatusDestinationWebhook,
		DestinationID:     999,
		EventID:           conflictingEventID,
		Payload:           "{}",
		Status:            model.StatusDeliveryPending,
		Version:           1,
	}).Error)

	_, err = PublishStatusIncidentUpdate(StatusIncidentPublishInput{
		IncidentID:      created.Incident.ID,
		ExpectedVersion: created.Incident.Version,
		State:           "investigating",
		Body:            "This transaction must roll back.",
		EventID:         "incident-published-4",
		Destinations:    []StatusDeliveryDestination{{Type: model.StatusDestinationWebhook, ID: 30}},
		Actor:           statusAdminActor(),
		Reason:          "test transactional failure",
		Now:             5_010,
	})
	require.Error(t, err)

	var stored model.StatusIncident
	require.NoError(t, db.First(&stored, created.Incident.ID).Error)
	require.Equal(t, created.Incident.Version, stored.Version)
	require.Equal(t, "draft", stored.Status)
	var publishedCount int64
	require.NoError(t, db.Model(&model.StatusIncidentUpdate{}).Where("incident_id = ? AND published = ?", stored.ID, true).Count(&publishedCount).Error)
	require.Zero(t, publishedCount)
}

func TestStatusIncidentPublishRejectsOptimisticConflict(t *testing.T) {
	setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOperational)
	created, err := ReconcileStatusIncidentAutomation(StatusIncidentAutomationInput{
		ComponentID: component.ID, PreviousObservedStatus: model.StatusOperational, ObservedStatus: model.StatusDegraded,
		EvidenceSummary: "degraded", IdempotencyKey: "automatic:test-model:episode-5", Now: 6_000,
	})
	require.NoError(t, err)
	_, err = PublishStatusIncidentUpdate(StatusIncidentPublishInput{
		IncidentID: created.Incident.ID, ExpectedVersion: created.Incident.Version, State: "investigating",
		Body: "First update.", EventID: "incident-published-5", Actor: statusAdminActor(), Reason: "publish", Now: 6_010,
	})
	require.NoError(t, err)
	_, err = PublishStatusIncidentUpdate(StatusIncidentPublishInput{
		IncidentID: created.Incident.ID, ExpectedVersion: created.Incident.Version, State: "identified",
		Body: "Stale update.", EventID: "incident-published-6", Actor: statusAdminActor(), Reason: "stale", Now: 6_020,
	})
	require.True(t, errors.Is(err, model.ErrStatusVersionConflict))
}

func TestStatusMaintenancePublicationAuthorizesStartAndEndFallback(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOutage)
	maintenance, err := CreateStatusMaintenanceDraft(StatusMaintenanceDraftInput{
		Title:            "Database maintenance",
		Body:             "Planned database maintenance.",
		IdempotencyKey:   "maintenance:database:7",
		ComponentIDs:     []int64{component.ID},
		ScheduledStartAt: 7_100,
		ScheduledEndAt:   7_200,
		Actor:            statusAdminActor(),
		Reason:           "database upgrade",
		Now:              7_000,
	})
	require.NoError(t, err)

	_, err = ReconcileStatusMaintenance(StatusMaintenanceTransitionInput{
		IncidentID: maintenance.ID, ExpectedVersion: maintenance.Version, Now: 7_100,
	})
	require.True(t, errors.Is(err, ErrStatusMaintenanceNotPublished))

	published, err := PublishStatusIncidentUpdate(StatusIncidentPublishInput{
		IncidentID: maintenance.ID, ExpectedVersion: maintenance.Version, State: "identified",
		Body: "Maintenance will begin shortly.", EventID: "maintenance-published-1",
		Actor: statusAdminActor(), Reason: "approve maintenance", Now: 7_050,
	})
	require.NoError(t, err)
	require.Equal(t, "scheduled", published.Incident.Status)

	started, err := ReconcileStatusMaintenance(StatusMaintenanceTransitionInput{
		IncidentID: maintenance.ID, ExpectedVersion: published.Incident.Version, Now: 7_100,
	})
	require.NoError(t, err)
	require.Equal(t, "monitoring", started.Status)
	var during model.StatusComponent
	require.NoError(t, db.First(&during, component.ID).Error)
	require.Equal(t, model.StatusOutage, during.ObservedStatus, "observed health must continue during maintenance")
	require.Equal(t, model.StatusMaintenance, during.EffectiveStatus)
	require.Equal(t, "maintenance", during.StatusSource)
	lease := acquireStatusServiceLease(t, "maintenance-evaluation", "node-a", 7_110)
	during.ObservedStatus = model.StatusDegraded
	during.EffectiveStatus = model.StatusDegraded
	during.StatusSource = "traffic"
	during.UpdatedAt = 7_110
	require.NoError(t, model.CommitStatusEvaluationWithFence("maintenance-evaluation", "node-a", lease.FencingToken, 7_110, &during))
	require.NoError(t, db.First(&during, component.ID).Error)
	require.Equal(t, model.StatusDegraded, during.ObservedStatus, "observed health must continue changing during maintenance")
	require.Equal(t, model.StatusMaintenance, during.EffectiveStatus, "an evaluator write must not end published maintenance")
	require.Equal(t, "maintenance", during.StatusSource)

	ended, err := ReconcileStatusMaintenance(StatusMaintenanceTransitionInput{
		IncidentID: maintenance.ID, ExpectedVersion: started.Version, Now: 7_200,
	})
	require.NoError(t, err)
	require.Equal(t, "resolved", ended.Status)
	var after model.StatusComponent
	require.NoError(t, db.First(&after, component.ID).Error)
	require.Equal(t, model.StatusDegraded, after.ObservedStatus)
	require.Equal(t, model.StatusDegraded, after.EffectiveStatus)
	require.Equal(t, "observed", after.StatusSource)

	var fallback model.StatusIncident
	require.NoError(t, db.Joins("JOIN status_incident_components sic ON sic.incident_id = status_incidents.id").
		Where("sic.component_id = ? AND status_incidents.kind = ? AND status_incidents.automation_mode = ?", component.ID, model.StatusIncidentKindIncident, "automatic").
		First(&fallback).Error)
	require.Equal(t, "private", fallback.Visibility)
	require.Equal(t, "draft", fallback.Status)
}

func TestStatusOverrideExpiresAndRestoresObservedStatus(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOutage)
	updated, err := ApplyStatusOverride(StatusOverrideInput{
		ComponentID: component.ID, ExpectedVersion: component.Version, Status: model.StatusUnknown,
		Reason: "monitoring is inconclusive", ExpiresAt: 8_100, Actor: statusAdminActor(), Now: 8_000,
	})
	require.NoError(t, err)
	require.Equal(t, model.StatusUnknown, updated.EffectiveStatus)
	require.Equal(t, "override", updated.StatusSource)

	expired, err := ExpireStatusOverrides(8_101)
	require.NoError(t, err)
	require.Len(t, expired, 1)
	require.Equal(t, model.StatusOutage, expired[0].EffectiveStatus)
	require.Equal(t, "observed", expired[0].StatusSource)
	require.Empty(t, expired[0].OverrideStatus)

	var audit model.StatusAuditEvent
	require.NoError(t, db.Where("action = ?", "status.override.expire").First(&audit).Error)
	require.Equal(t, "automation", audit.ActorType)
}

func TestStatusOverrideForceGreenRequiresVerifiedRootAndOneHourMaximum(t *testing.T) {
	setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOutage)
	base := StatusOverrideInput{
		ComponentID: component.ID, ExpectedVersion: component.Version, Status: model.StatusOperational,
		Reason: "temporary false positive", ExpiresAt: 9_600, Now: 9_000,
	}

	admin := base
	admin.Actor = statusAdminActor()
	_, err := ApplyStatusOverride(admin)
	require.True(t, errors.Is(err, ErrStatusRootRequired))

	unverified := base
	unverified.Actor = statusRootActor(false)
	_, err = ApplyStatusOverride(unverified)
	require.True(t, errors.Is(err, ErrStatusSecureVerificationRequired))

	tooLong := base
	tooLong.Actor = statusRootActor(true)
	tooLong.ExpiresAt = tooLong.Now + 3_601
	_, err = ApplyStatusOverride(tooLong)
	require.True(t, errors.Is(err, ErrStatusForceGreenTooLong))

	valid := base
	valid.Actor = statusRootActor(true)
	valid.ExpiresAt = valid.Now + 3_600
	updated, err := ApplyStatusOverride(valid)
	require.NoError(t, err)
	require.Equal(t, model.StatusOperational, updated.EffectiveStatus)
	require.EqualValues(t, valid.ExpiresAt, updated.OverrideExpiresAt)
}

func TestStatusOverrideUsesOptimisticVersionAndAuditsBeforeAfter(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOperational)
	updated, err := ApplyStatusOverride(StatusOverrideInput{
		ComponentID: component.ID, ExpectedVersion: component.Version, Status: model.StatusDegraded,
		Reason: "customer impact under review", ExpiresAt: 10_600, Actor: statusAdminActor(), Now: 10_000,
	})
	require.NoError(t, err)
	require.EqualValues(t, component.Version+1, updated.Version)

	_, err = ApplyStatusOverride(StatusOverrideInput{
		ComponentID: component.ID, ExpectedVersion: component.Version, Status: model.StatusOutage,
		Reason: "stale update", ExpiresAt: 10_700, Actor: statusAdminActor(), Now: 10_010,
	})
	require.True(t, errors.Is(err, model.ErrStatusVersionConflict))

	var audits []model.StatusAuditEvent
	require.NoError(t, db.Where("action = ?", "status.override.set").Find(&audits).Error)
	require.Len(t, audits, 1, "a stale mutation must not produce an audit row")
	audit := audits[0]
	require.Equal(t, statusAdminActor().ID, audit.ActorID)
	require.Equal(t, "component", audit.ObjectType)
	require.Equal(t, "customer impact under review", audit.Reason)
	require.EqualValues(t, 10_000, audit.CreatedAt)

	var before model.StatusComponent
	var after model.StatusComponent
	require.NoError(t, common.Unmarshal([]byte(audit.BeforeJSON), &before))
	require.NoError(t, common.Unmarshal([]byte(audit.AfterJSON), &after))
	require.Equal(t, model.StatusOperational, before.EffectiveStatus)
	require.Empty(t, before.OverrideStatus)
	require.Equal(t, model.StatusDegraded, after.EffectiveStatus)
	require.Equal(t, model.StatusDegraded, after.OverrideStatus)
}
