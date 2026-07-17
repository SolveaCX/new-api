package service

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

type statusIncidentDraftAuditSnapshot struct {
	Incident    *model.StatusIncident          `json:"incident"`
	Draft       *model.StatusIncidentUpdate    `json:"draft"`
	Association *model.StatusIncidentComponent `json:"component_association"`
}

type statusIncidentPublishAuditDestination struct {
	Type    string `json:"type"`
	ID      int64  `json:"id"`
	EventID string `json:"event_id"`
}

type statusIncidentPublishAuditSnapshot struct {
	Incident     model.StatusIncident                    `json:"incident"`
	Update       *model.StatusIncidentUpdate             `json:"update"`
	Destinations []statusIncidentPublishAuditDestination `json:"destinations"`
}

type statusMaintenanceAuditSnapshot struct {
	Incident     *model.StatusIncident           `json:"incident"`
	Draft        *model.StatusIncidentUpdate     `json:"draft"`
	Associations []model.StatusIncidentComponent `json:"component_associations"`
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
		PreviousObservedStatus: model.StatusOperational,
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

func TestStatusIncidentAutomationRedactsCredentialsFromDraftAndAudit(t *testing.T) {
	tests := []struct {
		name     string
		evidence string
		secret   string
	}{
		{name: "authorization without space", evidence: "request failed Authorization:Bearer sentinel-auth-tight", secret: "sentinel-auth-tight"},
		{name: "authorization with space", evidence: "request failed Authorization: Bearer sentinel-auth-spaced", secret: "sentinel-auth-spaced"},
		{name: "secret colon", evidence: "request failed secret: sentinel-secret-colon", secret: "sentinel-secret-colon"},
		{name: "token equals", evidence: "request failed token=sentinel-token-equals", secret: "sentinel-token-equals"},
		{name: "api key header", evidence: "request failed x-api-key: sentinel-api-key", secret: "sentinel-api-key"},
		{name: "access token", evidence: "request failed access_token=sentinel-access-token", secret: "sentinel-access-token"},
		{name: "password", evidence: "request failed password: sentinel-password", secret: "sentinel-password"},
		{name: "quoted fragment", evidence: `request failed "token=sentinel-quoted-fragment"`, secret: "sentinel-quoted-fragment"},
		{name: "json object", evidence: `{"message":"request failed","token":"sentinel-json-object"}`, secret: "sentinel-json-object"},
		{name: "nested json object", evidence: `{"message":"request failed","nested":{"Authorization":"Bearer sentinel-json-nested"}}`, secret: "sentinel-json-nested"},
		{name: "json array", evidence: `[{"message":"request failed","x-api-key":"sentinel-json-array"}]`, secret: "sentinel-json-array"},
		{name: "mixed case nested json key", evidence: `{"message":"request failed","nested":[{"PaSsWoRd":"sentinel-json-mixed-case"}]}`, secret: "sentinel-json-mixed-case"},
		{name: "quoted sk token in json string", evidence: `{"message":"request failed sk-sentinel-json-string"}`, secret: "sk-sentinel-json-string"},
		{name: "authorization in json message", evidence: `{"message":"request failed Authorization:Bearer sentinel-json-message-authorization"}`, secret: "sentinel-json-message-authorization"},
		{name: "secret in json error", evidence: `{"error":"request failed secret: sentinel-json-error-secret"}`, secret: "sentinel-json-error-secret"},
		{name: "token in json message", evidence: `{"message":"request failed token=sentinel-json-message-token"}`, secret: "sentinel-json-message-token"},
		{name: "quoted authorization fragment in json string", evidence: `{"message":"request failed \"Authorization\":\"Bearer sentinel-json-quoted-authorization\""}`, secret: "sentinel-json-quoted-authorization"},
		{name: "quoted token fragment in json string", evidence: `{"message":"request failed \"token\":\"sentinel-json-quoted-token\""}`, secret: "sentinel-json-quoted-token"},
		{name: "scalar json string", evidence: `"request failed secret: sentinel-json-scalar"`, secret: "sentinel-json-scalar"},
		{name: "refresh token scalar json string", evidence: `"request failed refresh_token=rt-live-secret"`, secret: "rt-live-secret"},
		{name: "quoted client secret fragment in json string", evidence: `{"message":"request failed \"client_secret\":\"cs-live-secret\""}`, secret: "cs-live-secret"},
		{name: "quoted sk token", evidence: `request failed "sk-sentinel-quoted-key"`, secret: "sk-sentinel-quoted-key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupStatusServiceTestDB(t)
			component := createStatusIncidentTestComponent(t, model.StatusOperational)
			result, err := ReconcileStatusIncidentAutomation(StatusIncidentAutomationInput{
				ComponentID:            component.ID,
				PreviousObservedStatus: model.StatusOperational,
				ObservedStatus:         model.StatusOutage,
				EvidenceSummary:        tt.evidence,
				IdempotencyKey:         "credential-redaction-" + common.GetUUID(),
				Now:                    2_500,
			})
			require.NoError(t, err)
			require.NotNil(t, result.Draft)
			require.NotContains(t, result.Draft.Body, tt.secret)
			require.Contains(t, result.Draft.Body, "request failed")

			var audit model.StatusAuditEvent
			require.NoError(t, db.Where("action = ?", "status.incident.draft.auto").First(&audit).Error)
			require.NotContains(t, audit.AfterJSON, tt.secret)
			require.Contains(t, audit.AfterJSON, "request failed")
		})
	}
}

func TestStatusIncidentEvidenceSanitizerPreservesUTF8WithinByteLimit(t *testing.T) {
	sanitized := sanitizeStatusEvidence(strings.Repeat("a", 999) + "界")
	require.LessOrEqual(t, len(sanitized), 1_000)
	require.True(t, utf8.ValidString(sanitized), "sanitization must not split a multibyte rune")
}

func TestStatusIncidentAutomationAuditCapturesCompleteAggregate(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOperational)
	result, err := ReconcileStatusIncidentAutomation(StatusIncidentAutomationInput{
		ComponentID: component.ID, PreviousObservedStatus: model.StatusOperational, ObservedStatus: model.StatusDegraded,
		EvidenceSummary: "request failed safely", IdempotencyKey: "automation-audit-aggregate", Now: 2_600,
	})
	require.NoError(t, err)

	var audit model.StatusAuditEvent
	require.NoError(t, db.Where("action = ?", "status.incident.draft.auto").First(&audit).Error)
	require.NotEqual(t, "null", audit.BeforeJSON)
	var before statusIncidentDraftAuditSnapshot
	var after statusIncidentDraftAuditSnapshot
	require.NoError(t, common.Unmarshal([]byte(audit.BeforeJSON), &before))
	require.NoError(t, common.Unmarshal([]byte(audit.AfterJSON), &after))
	require.Nil(t, before.Incident)
	require.Nil(t, before.Draft)
	require.Nil(t, before.Association)
	require.NotNil(t, after.Incident)
	require.NotNil(t, after.Draft)
	require.NotNil(t, after.Association)
	require.Equal(t, result.Incident.ID, after.Incident.ID)
	require.Equal(t, result.Draft.ID, after.Draft.ID)
	require.NotZero(t, after.Association.ID)
	require.Equal(t, result.Incident.ID, after.Association.IncidentID)
	require.Equal(t, component.ID, after.Association.ComponentID)
}

func TestStatusIncidentAutomationIgnoresNonTransitions(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusUnknown)
	tests := []struct {
		name     string
		previous string
		observed string
	}{
		{name: "repeated outage", previous: model.StatusOutage, observed: model.StatusOutage},
		{name: "repeated degraded", previous: model.StatusDegraded, observed: model.StatusDegraded},
		{name: "unknown to degraded", previous: model.StatusUnknown, observed: model.StatusDegraded},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ReconcileStatusIncidentAutomation(StatusIncidentAutomationInput{
				ComponentID:            component.ID,
				PreviousObservedStatus: tt.previous,
				ObservedStatus:         tt.observed,
				EvidenceSummary:        "must not create a draft",
				IdempotencyKey:         "non-transition-" + common.GetUUID(),
				Now:                    2_100 + int64(i),
			})
			require.NoError(t, err)
			require.Nil(t, result.Incident)
			require.Nil(t, result.Draft)
		})
	}

	var incidentCount int64
	require.NoError(t, db.Model(&model.StatusIncident{}).Count(&incidentCount).Error)
	require.Zero(t, incidentCount)
	var auditCount int64
	require.NoError(t, db.Model(&model.StatusAuditEvent{}).Count(&auditCount).Error)
	require.Zero(t, auditCount)
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

func TestStatusIncidentPublishAuditCapturesUpdateAndSafeDestinations(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOperational)
	created, err := ReconcileStatusIncidentAutomation(StatusIncidentAutomationInput{
		ComponentID: component.ID, PreviousObservedStatus: model.StatusOperational, ObservedStatus: model.StatusDegraded,
		EvidenceSummary: "degraded traffic", IdempotencyKey: "publish-audit-aggregate", Now: 4_500,
	})
	require.NoError(t, err)

	published, err := PublishStatusIncidentUpdate(StatusIncidentPublishInput{
		IncidentID: created.Incident.ID, ExpectedVersion: created.Incident.Version, State: "monitoring",
		Body: "Public evidence only.", EventID: "publish-audit-event",
		Destinations: []StatusDeliveryDestination{{Type: model.StatusDestinationWebhook, ID: 41}, {Type: model.StatusDestinationEmail, ID: 42}},
		Actor:        statusAdminActor(), Reason: "publish safe update", Now: 4_510,
	})
	require.NoError(t, err)

	var audit model.StatusAuditEvent
	require.NoError(t, db.Where("action = ?", "status.incident.publish").First(&audit).Error)
	var before statusIncidentPublishAuditSnapshot
	var after statusIncidentPublishAuditSnapshot
	require.NoError(t, common.Unmarshal([]byte(audit.BeforeJSON), &before))
	require.NoError(t, common.Unmarshal([]byte(audit.AfterJSON), &after))
	require.Equal(t, "draft", before.Incident.Status)
	require.Nil(t, before.Update)
	require.Empty(t, before.Destinations)
	require.Equal(t, published.Incident.ID, after.Incident.ID)
	require.NotNil(t, after.Update)
	require.Equal(t, published.Update.ID, after.Update.ID)
	require.Equal(t, "monitoring", after.Update.State)
	require.Equal(t, "Public evidence only.", after.Update.Body)
	require.Equal(t, "publish-audit-event", after.Update.EventID)
	require.Len(t, after.Destinations, 2)
	require.ElementsMatch(t, []statusIncidentPublishAuditDestination{
		{Type: model.StatusDestinationWebhook, ID: 41, EventID: statusDeliveryEventID("publish-audit-event", model.StatusDestinationWebhook, 41)},
		{Type: model.StatusDestinationEmail, ID: 42, EventID: statusDeliveryEventID("publish-audit-event", model.StatusDestinationEmail, 42)},
	}, after.Destinations)
	require.NotContains(t, strings.ToLower(audit.AfterJSON), "payload")
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
	require.EqualValues(t, 7_100, started.StartedAt)
	var during model.StatusComponent
	require.NoError(t, db.First(&during, component.ID).Error)
	require.Equal(t, model.StatusOutage, during.ObservedStatus, "observed health must continue during maintenance")
	require.Equal(t, model.StatusMaintenance, during.EffectiveStatus)
	require.Equal(t, "maintenance", during.StatusSource)
	lease := acquireStatusServiceLease(t, "maintenance-evaluation", "node-a", 7_110)
	staleEvaluation := component
	staleEvaluation.ObservedStatus = model.StatusDegraded
	staleEvaluation.EffectiveStatus = model.StatusDegraded
	staleEvaluation.StatusSource = "traffic"
	staleEvaluation.UpdatedAt = 7_110
	versionBeforeEvaluation := during.Version
	require.NoError(t, model.CommitStatusEvaluationWithFence("maintenance-evaluation", "node-a", lease.FencingToken, 7_110, &staleEvaluation))
	require.NoError(t, db.First(&during, component.ID).Error)
	require.Equal(t, model.StatusDegraded, during.ObservedStatus, "observed health must continue changing during maintenance")
	require.Equal(t, model.StatusMaintenance, during.EffectiveStatus, "an evaluator write must not end published maintenance")
	require.Equal(t, "maintenance", during.StatusSource)
	require.EqualValues(t, versionBeforeEvaluation+1, during.Version, "fenced evaluation must advance the component version")

	ended, err := ReconcileStatusMaintenance(StatusMaintenanceTransitionInput{
		IncidentID: maintenance.ID, ExpectedVersion: started.Version, Now: 7_200,
	})
	require.NoError(t, err)
	require.Equal(t, "resolved", ended.Status)
	require.Equal(t, started.StartedAt, ended.StartedAt, "maintenance end must preserve the actual start timestamp")
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

func TestStatusMaintenanceRejectsOverlappingComponentsBeforeCreatingRecords(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	firstComponent := createStatusIncidentTestComponent(t, model.StatusOperational)
	secondComponent := createStatusIncidentTestComponent(t, model.StatusOperational)

	first, err := CreateStatusMaintenanceDraft(StatusMaintenanceDraftInput{
		Title: "First maintenance", Body: "First planned window.", IdempotencyKey: "maintenance-overlap-first",
		ComponentIDs:     []int64{secondComponent.ID, firstComponent.ID, secondComponent.ID},
		ScheduledStartAt: 15_100, ScheduledEndAt: 15_200,
		Actor: statusAdminActor(), Reason: "first planned work", Now: 15_000,
	})
	require.NoError(t, err)

	var associations []model.StatusIncidentComponent
	require.NoError(t, db.Where("incident_id = ?", first.ID).Order("id ASC").Find(&associations).Error)
	require.Len(t, associations, 2)
	require.Equal(t, []int64{firstComponent.ID, secondComponent.ID}, []int64{associations[0].ComponentID, associations[1].ComponentID})

	_, err = CreateStatusMaintenanceDraft(StatusMaintenanceDraftInput{
		Title: "Overlapping maintenance", Body: "This must be rejected.", IdempotencyKey: "maintenance-overlap-second",
		ComponentIDs: []int64{firstComponent.ID}, ScheduledStartAt: 15_150, ScheduledEndAt: 15_250,
		Actor: statusAdminActor(), Reason: "overlapping planned work", Now: 15_010,
	})
	require.True(t, errors.Is(err, ErrStatusMaintenanceOverlap))

	var incidentCount, draftCount, associationCount, auditCount int64
	require.NoError(t, db.Model(&model.StatusIncident{}).Count(&incidentCount).Error)
	require.NoError(t, db.Model(&model.StatusIncidentUpdate{}).Count(&draftCount).Error)
	require.NoError(t, db.Model(&model.StatusIncidentComponent{}).Count(&associationCount).Error)
	require.NoError(t, db.Model(&model.StatusAuditEvent{}).Count(&auditCount).Error)
	require.EqualValues(t, 1, incidentCount)
	require.EqualValues(t, 1, draftCount)
	require.EqualValues(t, 2, associationCount)
	require.EqualValues(t, 1, auditCount)

	adjacent, err := CreateStatusMaintenanceDraft(StatusMaintenanceDraftInput{
		Title: "Adjacent maintenance", Body: "A non-overlapping boundary is allowed.", IdempotencyKey: "maintenance-overlap-adjacent",
		ComponentIDs: []int64{firstComponent.ID}, ScheduledStartAt: 15_200, ScheduledEndAt: 15_300,
		Actor: statusAdminActor(), Reason: "adjacent planned work", Now: 15_020,
	})
	require.NoError(t, err)
	require.NotZero(t, adjacent.ID)
}

func TestStatusMaintenanceProgressPublishPreservesActiveTransition(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOperational)
	maintenance, err := CreateStatusMaintenanceDraft(StatusMaintenanceDraftInput{
		Title: "Progress maintenance", Body: "Planned work.", IdempotencyKey: "maintenance-progress",
		ComponentIDs: []int64{component.ID}, ScheduledStartAt: 16_100, ScheduledEndAt: 16_300,
		Actor: statusAdminActor(), Reason: "planned work", Now: 16_000,
	})
	require.NoError(t, err)
	published, err := PublishStatusIncidentUpdate(StatusIncidentPublishInput{
		IncidentID: maintenance.ID, ExpectedVersion: maintenance.Version, State: "identified",
		Body: "Work is scheduled.", EventID: "maintenance-progress-initial", Actor: statusAdminActor(),
		Reason: "publish maintenance", Now: 16_050,
	})
	require.NoError(t, err)
	started, err := ReconcileStatusMaintenance(StatusMaintenanceTransitionInput{
		IncidentID: maintenance.ID, ExpectedVersion: published.Incident.Version, Now: 16_100,
	})
	require.NoError(t, err)
	require.Equal(t, "monitoring", started.Status)
	require.EqualValues(t, 16_100, started.StartedAt)

	progress, err := PublishStatusIncidentUpdate(StatusIncidentPublishInput{
		IncidentID: maintenance.ID, ExpectedVersion: started.Version, State: "monitoring",
		Body: "Work is progressing normally.", EventID: "maintenance-progress-update", Actor: statusAdminActor(),
		Reason: "publish progress", Now: 16_150,
	})
	require.NoError(t, err)
	require.Equal(t, "monitoring", progress.Incident.Status)
	require.Equal(t, started.StartedAt, progress.Incident.StartedAt)

	reconciled, err := ReconcileStatusMaintenance(StatusMaintenanceTransitionInput{
		IncidentID: maintenance.ID, ExpectedVersion: progress.Incident.Version, Now: 16_160,
	})
	require.NoError(t, err)
	require.Equal(t, "monitoring", reconciled.Status)
	require.Equal(t, progress.Incident.Version, reconciled.Version)

	var startAuditCount, componentStartAuditCount int64
	require.NoError(t, db.Model(&model.StatusAuditEvent{}).Where("action = ?", "status.maintenance.start").Count(&startAuditCount).Error)
	require.NoError(t, db.Model(&model.StatusAuditEvent{}).Where("action = ?", "status.maintenance.component.start").Count(&componentStartAuditCount).Error)
	require.EqualValues(t, 1, startAuditCount)
	require.EqualValues(t, 1, componentStartAuditCount)
}

func TestStatusMaintenanceRejectsGenericResolutionWhileActive(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOperational)
	maintenance, err := CreateStatusMaintenanceDraft(StatusMaintenanceDraftInput{
		Title: "Active maintenance", Body: "Planned work.", IdempotencyKey: "maintenance-active-resolution",
		ComponentIDs: []int64{component.ID}, ScheduledStartAt: 16_600, ScheduledEndAt: 16_800,
		Actor: statusAdminActor(), Reason: "planned work", Now: 16_500,
	})
	require.NoError(t, err)
	published, err := PublishStatusIncidentUpdate(StatusIncidentPublishInput{
		IncidentID: maintenance.ID, ExpectedVersion: maintenance.Version, State: "identified",
		Body: "Work is scheduled.", EventID: "maintenance-active-resolution-published", Actor: statusAdminActor(),
		Reason: "publish maintenance", Now: 16_550,
	})
	require.NoError(t, err)
	started, err := ReconcileStatusMaintenance(StatusMaintenanceTransitionInput{
		IncidentID: maintenance.ID, ExpectedVersion: published.Incident.Version, Now: 16_600,
	})
	require.NoError(t, err)
	require.Equal(t, "monitoring", started.Status)

	var updateCountBefore, outboxCountBefore, auditCountBefore int64
	require.NoError(t, db.Model(&model.StatusIncidentUpdate{}).Count(&updateCountBefore).Error)
	require.NoError(t, db.Model(&model.StatusDeliveryOutbox{}).Count(&outboxCountBefore).Error)
	require.NoError(t, db.Model(&model.StatusAuditEvent{}).Count(&auditCountBefore).Error)

	_, err = PublishStatusIncidentUpdate(StatusIncidentPublishInput{
		IncidentID: maintenance.ID, ExpectedVersion: started.Version, State: "resolved",
		Body: "Work is complete.", EventID: "maintenance-active-resolution-generic",
		Destinations: []StatusDeliveryDestination{{Type: model.StatusDestinationWebhook, ID: 71}},
		Actor:        statusAdminActor(), Reason: "generic resolution must not bypass maintenance end", Now: 16_700,
	})
	require.ErrorIs(t, err, ErrStatusMaintenanceRequiresTransition)

	var storedIncident model.StatusIncident
	require.NoError(t, db.First(&storedIncident, maintenance.ID).Error)
	require.Equal(t, "monitoring", storedIncident.Status)
	require.Equal(t, started.Version, storedIncident.Version)
	require.Zero(t, storedIncident.ResolvedAt)
	var storedComponent model.StatusComponent
	require.NoError(t, db.First(&storedComponent, component.ID).Error)
	require.Equal(t, model.StatusMaintenance, storedComponent.EffectiveStatus)
	require.Equal(t, "maintenance", storedComponent.StatusSource)

	var updateCountAfter, outboxCountAfter, auditCountAfter int64
	require.NoError(t, db.Model(&model.StatusIncidentUpdate{}).Count(&updateCountAfter).Error)
	require.NoError(t, db.Model(&model.StatusDeliveryOutbox{}).Count(&outboxCountAfter).Error)
	require.NoError(t, db.Model(&model.StatusAuditEvent{}).Count(&auditCountAfter).Error)
	require.Equal(t, updateCountBefore, updateCountAfter)
	require.Equal(t, outboxCountBefore, outboxCountAfter)
	require.Equal(t, auditCountBefore, auditCountAfter)
}

func TestStatusMaintenanceTransitionLocksComponentsBeforeIncident(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOperational)
	maintenance, err := CreateStatusMaintenanceDraft(StatusMaintenanceDraftInput{
		Title: "Lock order maintenance", Body: "Planned work.", IdempotencyKey: "maintenance-lock-order",
		ComponentIDs: []int64{component.ID}, ScheduledStartAt: 16_600, ScheduledEndAt: 16_700,
		Actor: statusAdminActor(), Reason: "planned work", Now: 16_500,
	})
	require.NoError(t, err)
	published, err := PublishStatusIncidentUpdate(StatusIncidentPublishInput{
		IncidentID: maintenance.ID, ExpectedVersion: maintenance.Version, State: "identified",
		Body: "Work is scheduled.", EventID: "maintenance-lock-order-published", Actor: statusAdminActor(),
		Reason: "publish maintenance", Now: 16_550,
	})
	require.NoError(t, err)

	queriedTables := make([]string, 0, 3)
	const callbackName = "status-maintenance-transition-lock-order"
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "status_components" || tx.Statement.Table == "status_incidents" {
			queriedTables = append(queriedTables, tx.Statement.Table)
		}
	}))
	t.Cleanup(func() {
		require.NoError(t, db.Callback().Query().Remove(callbackName))
	})

	_, err = ReconcileStatusMaintenance(StatusMaintenanceTransitionInput{
		IncidentID: maintenance.ID, ExpectedVersion: published.Incident.Version, Now: 16_600,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(queriedTables), 2)
	require.Equal(t, []string{"status_components", "status_incidents"}, queriedTables[:2],
		"maintenance create and transition must share the global components-then-incident lock order")
}

func TestStatusMaintenanceAuditCapturesDraftAndAssociations(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	firstComponent := createStatusIncidentTestComponent(t, model.StatusOperational)
	secondComponent := createStatusIncidentTestComponent(t, model.StatusOperational)
	maintenance, err := CreateStatusMaintenanceDraft(StatusMaintenanceDraftInput{
		Title: "Audited maintenance", Body: "Audited planned work.", IdempotencyKey: "maintenance-audit-aggregate",
		ComponentIDs:     []int64{secondComponent.ID, firstComponent.ID, secondComponent.ID},
		ScheduledStartAt: 17_100, ScheduledEndAt: 17_200,
		Actor: statusAdminActor(), Reason: "audit planned work", Now: 17_000,
	})
	require.NoError(t, err)

	var audit model.StatusAuditEvent
	require.NoError(t, db.Where("action = ?", "status.maintenance.create").First(&audit).Error)
	require.NotEqual(t, "null", audit.BeforeJSON)
	var before statusMaintenanceAuditSnapshot
	var after statusMaintenanceAuditSnapshot
	require.NoError(t, common.Unmarshal([]byte(audit.BeforeJSON), &before))
	require.NoError(t, common.Unmarshal([]byte(audit.AfterJSON), &after))
	require.Nil(t, before.Incident)
	require.Nil(t, before.Draft)
	require.Empty(t, before.Associations)
	require.NotNil(t, after.Incident)
	require.NotNil(t, after.Draft)
	require.Equal(t, maintenance.ID, after.Incident.ID)
	require.Equal(t, maintenance.ID, after.Draft.IncidentID)
	require.Len(t, after.Associations, 2)
	require.NotZero(t, after.Associations[0].ID)
	require.NotZero(t, after.Associations[1].ID)
	require.Equal(t, []int64{firstComponent.ID, secondComponent.ID}, []int64{after.Associations[0].ComponentID, after.Associations[1].ComponentID})
}

func TestStatusMaintenanceMissedWindowDoesNotFabricateStartTimestamp(t *testing.T) {
	setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOperational)
	maintenance, err := CreateStatusMaintenanceDraft(StatusMaintenanceDraftInput{
		Title: "Missed maintenance", Body: "Scheduled work.", IdempotencyKey: "maintenance:missed-window",
		ComponentIDs: []int64{component.ID}, ScheduledStartAt: 12_100, ScheduledEndAt: 12_200,
		Actor: statusAdminActor(), Reason: "planned work", Now: 12_000,
	})
	require.NoError(t, err)
	published, err := PublishStatusIncidentUpdate(StatusIncidentPublishInput{
		IncidentID: maintenance.ID, ExpectedVersion: maintenance.Version, State: "identified",
		Body: "Scheduled work.", EventID: "maintenance-missed-published", Actor: statusAdminActor(),
		Reason: "approve maintenance", Now: 12_050,
	})
	require.NoError(t, err)

	ended, err := ReconcileStatusMaintenance(StatusMaintenanceTransitionInput{
		IncidentID: maintenance.ID, ExpectedVersion: published.Incident.Version, Now: 12_300,
	})
	require.NoError(t, err)
	require.Equal(t, "resolved", ended.Status)
	require.Zero(t, ended.StartedAt, "ending an unstarted window must not invent a start time")
}

func TestStatusMaintenanceEndRestoresUnexpiredOverrideBeforeObserved(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOutage)
	overridden, err := ApplyStatusOverride(StatusOverrideInput{
		ComponentID: component.ID, ExpectedVersion: component.Version, Status: model.StatusUnknown,
		Reason: "monitoring uncertainty", ExpiresAt: 13_500, Actor: statusAdminActor(), Now: 13_000,
	})
	require.NoError(t, err)

	maintenance, err := CreateStatusMaintenanceDraft(StatusMaintenanceDraftInput{
		Title: "Override precedence", Body: "Scheduled work.", IdempotencyKey: "maintenance:override-precedence",
		ComponentIDs: []int64{component.ID}, ScheduledStartAt: 13_100, ScheduledEndAt: 13_200,
		Actor: statusAdminActor(), Reason: "planned work", Now: 13_010,
	})
	require.NoError(t, err)
	published, err := PublishStatusIncidentUpdate(StatusIncidentPublishInput{
		IncidentID: maintenance.ID, ExpectedVersion: maintenance.Version, State: "identified",
		Body: "Scheduled work.", EventID: "maintenance-override-published", Actor: statusAdminActor(),
		Reason: "approve maintenance", Now: 13_050,
	})
	require.NoError(t, err)
	started, err := ReconcileStatusMaintenance(StatusMaintenanceTransitionInput{
		IncidentID: maintenance.ID, ExpectedVersion: published.Incident.Version, Now: 13_100,
	})
	require.NoError(t, err)

	ended, err := ReconcileStatusMaintenance(StatusMaintenanceTransitionInput{
		IncidentID: maintenance.ID, ExpectedVersion: started.Version, Now: 13_200,
	})
	require.NoError(t, err)
	require.Equal(t, "resolved", ended.Status)
	var after model.StatusComponent
	require.NoError(t, db.First(&after, component.ID).Error)
	require.Equal(t, model.StatusOutage, after.ObservedStatus)
	require.Equal(t, model.StatusUnknown, after.EffectiveStatus)
	require.Equal(t, "override", after.StatusSource)
	require.EqualValues(t, overridden.OverrideExpiresAt, after.OverrideExpiresAt)

	var fallbackCount int64
	require.NoError(t, db.Model(&model.StatusIncident{}).
		Joins("JOIN status_incident_components sic ON sic.incident_id = status_incidents.id").
		Where("sic.component_id = ? AND status_incidents.kind = ? AND status_incidents.automation_mode = ?", component.ID, model.StatusIncidentKindIncident, "automatic").
		Count(&fallbackCount).Error)
	require.EqualValues(t, 1, fallbackCount, "fallback drafting must use unhealthy observed status, not the override")
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

func TestStatusOverrideStaleEvaluatorPreservesOverrideAndAdvancesVersion(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	component := createStatusIncidentTestComponent(t, model.StatusOperational)
	staleEvaluation := component
	staleEvaluation.ObservedStatus = model.StatusOutage
	staleEvaluation.EffectiveStatus = model.StatusOutage
	staleEvaluation.StatusSource = "traffic"
	staleEvaluation.LastEvaluatedAt = 14_100
	staleEvaluation.UpdatedAt = 14_100

	overridden, err := ApplyStatusOverride(StatusOverrideInput{
		ComponentID: component.ID, ExpectedVersion: component.Version, Status: model.StatusUnknown,
		Reason: "hold public status while investigating", ExpiresAt: 14_500, Actor: statusAdminActor(), Now: 14_000,
	})
	require.NoError(t, err)
	lease := acquireStatusServiceLease(t, "stale-override-evaluation", "node-a", 14_100)
	require.NoError(t, model.CommitStatusEvaluationWithFence(
		"stale-override-evaluation", "node-a", lease.FencingToken, 14_100, &staleEvaluation,
	))

	var after model.StatusComponent
	require.NoError(t, db.First(&after, component.ID).Error)
	require.Equal(t, model.StatusOutage, after.ObservedStatus)
	require.Equal(t, model.StatusUnknown, after.EffectiveStatus)
	require.Equal(t, "override", after.StatusSource)
	require.EqualValues(t, overridden.Version+1, after.Version)

	_, err = ApplyStatusOverride(StatusOverrideInput{
		ComponentID: component.ID, ExpectedVersion: overridden.Version, Status: model.StatusDegraded,
		Reason: "stale administrator mutation", ExpiresAt: 14_600, Actor: statusAdminActor(), Now: 14_110,
	})
	require.True(t, errors.Is(err, model.ErrStatusVersionConflict), "the evaluator version advance must reject stale admin writes")
}
