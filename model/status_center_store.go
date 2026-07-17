package model

import (
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrStatusVersionConflict               = errors.New("status object version conflict")
	ErrStatusMaintenanceNotPublished       = errors.New("status maintenance is not published")
	ErrStatusMaintenanceOverlap            = errors.New("status maintenance overlaps an existing window")
	ErrStatusMaintenanceRequiresTransition = errors.New("active status maintenance must be resolved through its transition")
)

type StatusAuditMutation struct {
	ActorID   int
	ActorType string
	Action    string
	Reason    string
	CreatedAt int64
}

type StatusIncidentDraftMutation struct {
	Incident    StatusIncident
	ComponentID int64
	Draft       StatusIncidentUpdate
	Audit       StatusAuditMutation
}

type StatusDeliveryDestinationMutation struct {
	Type    string
	ID      int64
	EventID string
}

type statusDeliveryAuditDestination struct {
	Type    string `json:"type"`
	ID      int64  `json:"id"`
	EventID string `json:"event_id"`
}

type StatusIncidentPublishMutation struct {
	IncidentID      int64
	ExpectedVersion int64
	Update          StatusIncidentUpdate
	Destinations    []StatusDeliveryDestinationMutation
	Audit           StatusAuditMutation
}

type StatusMaintenanceDraftMutation struct {
	Incident     StatusIncident
	ComponentIDs []int64
	Draft        StatusIncidentUpdate
	Audit        StatusAuditMutation
}

type StatusMaintenanceTransitionMutation struct {
	IncidentID      int64
	ExpectedVersion int64
	Now             int64
}

type StatusOverrideMutation struct {
	ComponentID     int64
	ExpectedVersion int64
	Status          string
	Reason          string
	ExpiresAt       int64
	ActorID         int
	Now             int64
	Audit           StatusAuditMutation
}

func AcquireStatusJobLease(name string, holder string, now int64, leaseSeconds int64) (StatusJobLease, bool, error) {
	if DB == nil {
		return StatusJobLease{}, false, errors.New("database is not initialized")
	}
	if name == "" || holder == "" || leaseSeconds <= 0 {
		return StatusJobLease{}, false, errors.New("invalid status job lease request")
	}

	lease := StatusJobLease{
		Name:         name,
		Holder:       holder,
		ExpiresAt:    now + leaseSeconds,
		FencingToken: 1,
		UpdatedAt:    now,
	}
	created := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&lease)
	if created.Error != nil {
		return StatusJobLease{}, false, created.Error
	}
	if created.RowsAffected == 1 {
		return lease, true, nil
	}

	result := DB.Model(&StatusJobLease{}).
		Where("name = ? AND expires_at <= ?", name, now).
		Updates(map[string]any{
			"holder":        holder,
			"expires_at":    now + leaseSeconds,
			"fencing_token": gorm.Expr("fencing_token + 1"),
			"updated_at":    now,
		})
	if result.Error != nil {
		return StatusJobLease{}, false, result.Error
	}
	var current StatusJobLease
	if err := DB.Where("name = ?", name).First(&current).Error; err != nil {
		return StatusJobLease{}, false, err
	}
	return current, result.RowsAffected == 1, nil
}

func RenewStatusJobLease(name string, holder string, fencingToken int64, now int64, leaseSeconds int64) (bool, error) {
	if DB == nil {
		return false, errors.New("database is not initialized")
	}
	if name == "" || holder == "" || fencingToken <= 0 || leaseSeconds <= 0 {
		return false, errors.New("invalid status job lease renewal")
	}
	result := DB.Model(&StatusJobLease{}).
		Where("name = ? AND holder = ? AND fencing_token = ? AND expires_at > ?", name, holder, fencingToken, now).
		Updates(map[string]any{
			"expires_at": now + leaseSeconds,
			"updated_at": now,
		})
	return result.RowsAffected == 1, result.Error
}

func ReleaseStatusJobLease(name string, holder string, fencingToken int64, now int64) (bool, error) {
	if DB == nil {
		return false, errors.New("database is not initialized")
	}
	if name == "" || holder == "" || fencingToken <= 0 {
		return false, errors.New("invalid status job lease release")
	}
	result := DB.Model(&StatusJobLease{}).
		Where("name = ? AND holder = ? AND fencing_token = ?", name, holder, fencingToken).
		Updates(map[string]any{
			"expires_at": now,
			"updated_at": now,
		})
	return result.RowsAffected == 1, result.Error
}

func CommitStatusComponentWithFence(jobName string, holder string, fencingToken int64, now int64, component *StatusComponent) error {
	if component == nil {
		return errors.New("status component is nil")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var lease StatusJobLease
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("name = ?", jobName).First(&lease).Error; err != nil {
			return err
		}
		if lease.Holder != holder || lease.FencingToken != fencingToken || lease.ExpiresAt <= now {
			return fmt.Errorf("status job lease is no longer owned")
		}
		return tx.Save(component).Error
	})
}

func CommitStatusEvaluationWithFence(jobName string, holder string, fencingToken int64, now int64, component *StatusComponent) error {
	if component == nil || component.ID == 0 {
		return errors.New("status component is nil")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := validateStatusJobFence(tx, jobName, holder, fencingToken, now); err != nil {
			return err
		}
		var current StatusComponent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, component.ID).Error; err != nil {
			return err
		}
		effectiveStatus := component.ObservedStatus
		statusSource := component.StatusSource
		if statusSource == "maintenance" || statusSource == "override" {
			statusSource = "observed"
		}
		switch {
		case current.StatusSource == "maintenance":
			effectiveStatus = StatusMaintenance
			statusSource = "maintenance"
		case current.OverrideStatus != "" && current.OverrideExpiresAt > now:
			effectiveStatus = current.OverrideStatus
			statusSource = "override"
		}
		result := tx.Model(&StatusComponent{}).Where("id = ? AND version = ?", component.ID, current.Version).Updates(map[string]any{
			"observed_status":              component.ObservedStatus,
			"effective_status":             effectiveStatus,
			"status_source":                statusSource,
			"last_evidence_at":             component.LastEvidenceAt,
			"last_trustworthy_update_at":   component.LastTrustworthyUpdateAt,
			"last_evaluated_at":            component.LastEvaluatedAt,
			"coverage_micros":              component.CoverageMicros,
			"consecutive_probe_failures":   component.ConsecutiveProbeFailures,
			"consecutive_probe_successes":  component.ConsecutiveProbeSuccesses,
			"consecutive_traffic_recovery": component.ConsecutiveTrafficRecovery,
			"last_traffic_bucket_start":    component.LastTrafficBucketStart,
			"updated_at":                   component.UpdatedAt,
			"version":                      gorm.Expr("version + 1"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrStatusVersionConflict
		}
		return nil
	})
}

func SyncStatusCatalogWithFence(jobName string, holder string, fencingToken int64, now int64, desired []StatusComponent) error {
	if DB == nil {
		return errors.New("database is not initialized")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var lease StatusJobLease
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("name = ?", jobName).First(&lease).Error; err != nil {
			return err
		}
		if lease.Holder != holder || lease.FencingToken != fencingToken || lease.ExpiresAt <= now {
			return fmt.Errorf("status job lease is no longer owned")
		}

		activeModelKeys := make([]string, 0, len(desired))
		for i := range desired {
			next := desired[i]
			if next.ComponentKey == "" || next.Slug == "" || next.Kind == "" {
				return errors.New("invalid status catalog component")
			}
			if next.Kind == StatusComponentKindModel {
				activeModelKeys = append(activeModelKeys, next.ComponentKey)
			}

			var existing StatusComponent
			err := tx.Where("component_key = ?", next.ComponentKey).First(&existing).Error
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound):
				if err := tx.Create(&next).Error; err != nil {
					return err
				}
			case err != nil:
				return err
			default:
				updates := map[string]any{
					"display_name": next.DisplayName,
					"model_name":   next.ModelName,
					"capability":   next.Capability,
					"lifecycle":    StatusLifecycleActive,
					"updated_at":   now,
				}
				if err := tx.Model(&StatusComponent{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
					return err
				}
			}
		}

		retired := tx.Model(&StatusComponent{}).Where("kind = ?", StatusComponentKindModel)
		if len(activeModelKeys) > 0 {
			retired = retired.Where("component_key NOT IN ?", activeModelKeys)
		}
		return retired.Updates(map[string]any{
			"lifecycle":  StatusLifecycleRetired,
			"updated_at": now,
		}).Error
	})
}

func UpdateStatusComponentVersion(id int64, expectedVersion int64, updates map[string]any) (StatusComponent, error) {
	if len(updates) == 0 {
		return StatusComponent{}, errors.New("status component update is empty")
	}
	clean := make(map[string]any, len(updates)+1)
	for key, value := range updates {
		if key == "id" || key == "version" || key == "component_key" {
			continue
		}
		clean[key] = value
	}
	clean["version"] = gorm.Expr("version + 1")
	result := DB.Model(&StatusComponent{}).Where("id = ? AND version = ?", id, expectedVersion).Updates(clean)
	if result.Error != nil {
		return StatusComponent{}, result.Error
	}
	if result.RowsAffected == 0 {
		return StatusComponent{}, ErrStatusVersionConflict
	}
	var component StatusComponent
	if err := DB.First(&component, id).Error; err != nil {
		return StatusComponent{}, err
	}
	return component, nil
}

func UpsertStatusIncidentDraft(input StatusIncidentDraftMutation) (StatusIncident, StatusIncidentUpdate, error) {
	if DB == nil {
		return StatusIncident{}, StatusIncidentUpdate{}, errors.New("database is not initialized")
	}
	if input.ComponentID <= 0 || input.Incident.IdempotencyKey == "" || input.Draft.EventID == "" || input.Draft.Body == "" {
		return StatusIncident{}, StatusIncidentUpdate{}, errors.New("invalid status incident draft")
	}
	var incident StatusIncident
	var draft StatusIncidentUpdate
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		incident, draft, err = upsertStatusIncidentDraftTx(tx, input)
		return err
	})
	return incident, draft, err
}

func PublishStatusIncidentUpdate(input StatusIncidentPublishMutation) (StatusIncident, StatusIncidentUpdate, error) {
	if DB == nil {
		return StatusIncident{}, StatusIncidentUpdate{}, errors.New("database is not initialized")
	}
	if input.IncidentID <= 0 || input.ExpectedVersion <= 0 || input.Update.EventID == "" || input.Update.Body == "" || !input.Update.Published {
		return StatusIncident{}, StatusIncidentUpdate{}, errors.New("invalid published status incident update")
	}

	var incident StatusIncident
	var update StatusIncidentUpdate
	err := DB.Transaction(func(tx *gorm.DB) error {
		var before StatusIncident
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&before, input.IncidentID).Error; err != nil {
			return err
		}
		if before.Version != input.ExpectedVersion {
			return ErrStatusVersionConflict
		}
		if before.Kind == StatusIncidentKindMaintenance && before.Status == "monitoring" && input.Update.State == "resolved" {
			return ErrStatusMaintenanceRequiresTransition
		}

		nextStatus := input.Update.State
		if before.Kind == StatusIncidentKindMaintenance && input.Update.State != "resolved" {
			if before.Visibility == "public" {
				nextStatus = before.Status
			} else {
				nextStatus = "scheduled"
			}
		}
		updates := map[string]any{
			"status":     nextStatus,
			"visibility": "public",
			"version":    gorm.Expr("version + 1"),
			"updated_at": input.Update.PublishedAt,
		}
		if input.Update.State == "resolved" {
			updates["resolved_at"] = input.Update.PublishedAt
		}
		result := tx.Model(&StatusIncident{}).
			Where("id = ? AND version = ?", input.IncidentID, input.ExpectedVersion).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrStatusVersionConflict
		}

		update = input.Update
		update.IncidentID = input.IncidentID
		if err := tx.Create(&update).Error; err != nil {
			return err
		}

		payload, err := common.Marshal(struct {
			EventID     string `json:"event_id"`
			IncidentID  int64  `json:"incident_id"`
			UpdateID    int64  `json:"update_id"`
			State       string `json:"state"`
			Body        string `json:"body"`
			PublishedAt int64  `json:"published_at"`
		}{
			EventID: input.Update.EventID, IncidentID: input.IncidentID, UpdateID: update.ID,
			State: input.Update.State, Body: input.Update.Body, PublishedAt: input.Update.PublishedAt,
		})
		if err != nil {
			return err
		}
		auditDestinations := make([]statusDeliveryAuditDestination, 0, len(input.Destinations))
		for _, destination := range input.Destinations {
			outbox := StatusDeliveryOutbox{
				PublishedUpdateID: update.ID,
				DestinationType:   destination.Type,
				DestinationID:     destination.ID,
				EventID:           destination.EventID,
				Payload:           string(payload),
				Status:            StatusDeliveryPending,
				NextAttemptAt:     input.Update.PublishedAt,
				Version:           1,
				CreatedAt:         input.Update.PublishedAt,
				UpdatedAt:         input.Update.PublishedAt,
			}
			if err := tx.Create(&outbox).Error; err != nil {
				return err
			}
			auditDestinations = append(auditDestinations, statusDeliveryAuditDestination{
				Type: destination.Type, ID: destination.ID, EventID: destination.EventID,
			})
		}

		if err := tx.First(&incident, input.IncidentID).Error; err != nil {
			return err
		}
		beforeSnapshot := struct {
			Incident     StatusIncident                   `json:"incident"`
			Update       *StatusIncidentUpdate            `json:"update"`
			Destinations []statusDeliveryAuditDestination `json:"destinations"`
		}{Incident: before, Destinations: []statusDeliveryAuditDestination{}}
		afterSnapshot := struct {
			Incident     StatusIncident                   `json:"incident"`
			Update       StatusIncidentUpdate             `json:"update"`
			Destinations []statusDeliveryAuditDestination `json:"destinations"`
		}{Incident: incident, Update: update, Destinations: auditDestinations}
		return createStatusAuditEvent(tx, input.Audit, "incident", strconv.FormatInt(input.IncidentID, 10), beforeSnapshot, afterSnapshot)
	})
	return incident, update, err
}

func CreateStatusMaintenanceDraft(input StatusMaintenanceDraftMutation) (StatusIncident, error) {
	if DB == nil {
		return StatusIncident{}, errors.New("database is not initialized")
	}
	if input.Incident.IdempotencyKey == "" || input.Incident.Kind != StatusIncidentKindMaintenance || len(input.ComponentIDs) == 0 || input.Draft.Body == "" {
		return StatusIncident{}, errors.New("invalid status maintenance draft")
	}

	componentIDs := append([]int64(nil), input.ComponentIDs...)
	sort.Slice(componentIDs, func(i, j int) bool { return componentIDs[i] < componentIDs[j] })
	deduplicated := componentIDs[:0]
	for _, componentID := range componentIDs {
		if len(deduplicated) == 0 || deduplicated[len(deduplicated)-1] != componentID {
			deduplicated = append(deduplicated, componentID)
		}
	}
	componentIDs = deduplicated

	incident := input.Incident
	err := DB.Transaction(func(tx *gorm.DB) error {
		var components []StatusComponent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id IN ?", componentIDs).Order("id ASC").Find(&components).Error; err != nil {
			return err
		}
		if len(components) != len(componentIDs) {
			return gorm.ErrRecordNotFound
		}

		var overlapping StatusIncident
		overlapResult := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Model(&StatusIncident{}).
			Select("status_incidents.*").
			Joins("JOIN status_incident_components ON status_incident_components.incident_id = status_incidents.id").
			Where("status_incident_components.component_id IN ?", componentIDs).
			Where("status_incidents.kind = ? AND status_incidents.status <> ?", StatusIncidentKindMaintenance, "resolved").
			Where("status_incidents.scheduled_start_at < ? AND status_incidents.scheduled_end_at > ?", incident.ScheduledEndAt, incident.ScheduledStartAt).
			Order("status_incidents.id ASC").Limit(1).Find(&overlapping)
		if overlapResult.Error != nil {
			return overlapResult.Error
		}
		if overlapResult.RowsAffected > 0 {
			return ErrStatusMaintenanceOverlap
		}

		if err := tx.Create(&incident).Error; err != nil {
			return err
		}
		associations := make([]StatusIncidentComponent, 0, len(componentIDs))
		for _, componentID := range componentIDs {
			association := StatusIncidentComponent{IncidentID: incident.ID, ComponentID: componentID}
			if err := tx.Create(&association).Error; err != nil {
				return err
			}
			associations = append(associations, association)
		}
		draft := input.Draft
		draft.IncidentID = incident.ID
		if err := tx.Create(&draft).Error; err != nil {
			return err
		}
		beforeSnapshot := struct {
			Incident     *StatusIncident           `json:"incident"`
			Draft        *StatusIncidentUpdate     `json:"draft"`
			Associations []StatusIncidentComponent `json:"component_associations"`
		}{Associations: []StatusIncidentComponent{}}
		afterSnapshot := struct {
			Incident     StatusIncident            `json:"incident"`
			Draft        StatusIncidentUpdate      `json:"draft"`
			Associations []StatusIncidentComponent `json:"component_associations"`
		}{Incident: incident, Draft: draft, Associations: associations}
		return createStatusAuditEvent(tx, input.Audit, "maintenance", strconv.FormatInt(incident.ID, 10), beforeSnapshot, afterSnapshot)
	})
	return incident, err
}

func TransitionStatusMaintenance(input StatusMaintenanceTransitionMutation) (StatusIncident, error) {
	if DB == nil {
		return StatusIncident{}, errors.New("database is not initialized")
	}
	if input.IncidentID <= 0 || input.ExpectedVersion <= 0 || input.Now <= 0 {
		return StatusIncident{}, errors.New("invalid status maintenance transition")
	}

	var incident StatusIncident
	err := DB.Transaction(func(tx *gorm.DB) error {
		var componentIDs []int64
		if err := tx.Model(&StatusIncidentComponent{}).
			Where("incident_id = ?", input.IncidentID).
			Order("component_id ASC").
			Pluck("component_id", &componentIDs).Error; err != nil {
			return err
		}
		var components []StatusComponent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id IN ?", componentIDs).Order("id ASC").Find(&components).Error; err != nil {
			return err
		}

		var before StatusIncident
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&before, input.IncidentID).Error; err != nil {
			return err
		}
		if before.Version != input.ExpectedVersion {
			return ErrStatusVersionConflict
		}
		if before.Kind != StatusIncidentKindMaintenance || before.Visibility != "public" || before.Status == "draft" {
			return ErrStatusMaintenanceNotPublished
		}
		if input.Now < before.ScheduledStartAt || before.Status == "resolved" {
			incident = before
			return nil
		}

		ending := before.ScheduledEndAt > 0 && input.Now >= before.ScheduledEndAt
		nextStatus := "monitoring"
		action := "status.maintenance.start"
		incidentUpdates := map[string]any{
			"status":     nextStatus,
			"updated_at": input.Now,
			"version":    gorm.Expr("version + 1"),
		}
		if ending {
			nextStatus = "resolved"
			action = "status.maintenance.end"
			incidentUpdates["status"] = nextStatus
			incidentUpdates["resolved_at"] = input.Now
		} else if before.StartedAt == 0 {
			incidentUpdates["started_at"] = input.Now
		}
		if before.Status == nextStatus {
			incident = before
			return nil
		}
		result := tx.Model(&StatusIncident{}).
			Where("id = ? AND version = ?", before.ID, before.Version).
			Updates(incidentUpdates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrStatusVersionConflict
		}

		for _, componentBefore := range components {
			componentUpdates := map[string]any{
				"effective_status": StatusMaintenance,
				"status_source":    "maintenance",
				"updated_at":       input.Now,
				"version":          gorm.Expr("version + 1"),
			}
			componentAction := "status.maintenance.component.start"
			if ending {
				if componentBefore.OverrideStatus != "" && componentBefore.OverrideExpiresAt > input.Now {
					componentUpdates["effective_status"] = componentBefore.OverrideStatus
					componentUpdates["status_source"] = "override"
				} else {
					componentUpdates["effective_status"] = componentBefore.ObservedStatus
					componentUpdates["status_source"] = "observed"
				}
				componentAction = "status.maintenance.component.end"
			}
			componentResult := tx.Model(&StatusComponent{}).
				Where("id = ? AND version = ?", componentBefore.ID, componentBefore.Version).
				Updates(componentUpdates)
			if componentResult.Error != nil {
				return componentResult.Error
			}
			if componentResult.RowsAffected == 0 {
				return ErrStatusVersionConflict
			}
			var componentAfter StatusComponent
			if err := tx.First(&componentAfter, componentBefore.ID).Error; err != nil {
				return err
			}
			if err := createStatusAuditEvent(tx, StatusAuditMutation{
				ActorType: "automation", Action: componentAction, Reason: "published maintenance window transition", CreatedAt: input.Now,
			}, "component", strconv.FormatInt(componentBefore.ID, 10), componentBefore, componentAfter); err != nil {
				return err
			}

			if ending && (componentAfter.ObservedStatus == StatusDegraded || componentAfter.ObservedStatus == StatusOutage) {
				fallbackKey := fmt.Sprintf("maintenance:%d:component:%d:end:%d", before.ID, componentAfter.ID, before.ScheduledEndAt)
				_, _, err := upsertStatusIncidentDraftTx(tx, StatusIncidentDraftMutation{
					Incident: StatusIncident{
						PublicID:       "inc_" + common.GetUUID(),
						Kind:           StatusIncidentKindIncident,
						Title:          "Post-maintenance status review",
						Impact:         componentAfter.ObservedStatus,
						Status:         "draft",
						Visibility:     "private",
						AutomationMode: "automatic",
						IdempotencyKey: fallbackKey,
						Version:        1,
						CreatedAt:      input.Now,
						UpdatedAt:      input.Now,
					},
					ComponentID: componentAfter.ID,
					Draft: StatusIncidentUpdate{
						EventID: "draft_" + common.GetUUID(), State: "investigating",
						Body: "Maintenance ended while observed health remained unhealthy; review before publishing.", CreatedAt: input.Now,
					},
					Audit: StatusAuditMutation{
						ActorType: "automation", Action: "status.incident.draft.auto",
						Reason: "maintenance ended with unhealthy observed status", CreatedAt: input.Now,
					},
				})
				if err != nil {
					return err
				}
			}
		}

		if err := tx.First(&incident, before.ID).Error; err != nil {
			return err
		}
		return createStatusAuditEvent(tx, StatusAuditMutation{
			ActorType: "automation", Action: action, Reason: "published maintenance window transition", CreatedAt: input.Now,
		}, "maintenance", strconv.FormatInt(before.ID, 10), before, incident)
	})
	return incident, err
}

func SetStatusComponentOverride(input StatusOverrideMutation) (StatusComponent, error) {
	if DB == nil {
		return StatusComponent{}, errors.New("database is not initialized")
	}
	if input.ComponentID <= 0 || input.ExpectedVersion <= 0 || input.Status == "" || input.ExpiresAt <= input.Now {
		return StatusComponent{}, errors.New("invalid status override")
	}

	var component StatusComponent
	err := DB.Transaction(func(tx *gorm.DB) error {
		var before StatusComponent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&before, input.ComponentID).Error; err != nil {
			return err
		}
		if before.Version != input.ExpectedVersion {
			return ErrStatusVersionConflict
		}
		updates := map[string]any{
			"override_status":     input.Status,
			"override_reason":     input.Reason,
			"override_expires_at": input.ExpiresAt,
			"override_by":         input.ActorID,
			"override_created_at": input.Now,
			"updated_at":          input.Now,
			"version":             gorm.Expr("version + 1"),
		}
		if before.StatusSource != "maintenance" {
			updates["effective_status"] = input.Status
			updates["status_source"] = "override"
		}
		result := tx.Model(&StatusComponent{}).
			Where("id = ? AND version = ?", before.ID, before.Version).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrStatusVersionConflict
		}
		if err := tx.First(&component, before.ID).Error; err != nil {
			return err
		}
		return createStatusAuditEvent(tx, input.Audit, "component", strconv.FormatInt(before.ID, 10), before, component)
	})
	return component, err
}

func ExpireStatusComponentOverrides(now int64) ([]StatusComponent, error) {
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}
	if now <= 0 {
		return nil, errors.New("invalid status override expiry time")
	}

	updated := make([]StatusComponent, 0)
	err := DB.Transaction(func(tx *gorm.DB) error {
		var candidates []StatusComponent
		if err := tx.Where("override_expires_at > 0 AND override_expires_at <= ?", now).Order("id ASC").Find(&candidates).Error; err != nil {
			return err
		}
		for _, before := range candidates {
			updates := map[string]any{
				"override_status":     "",
				"override_reason":     "",
				"override_expires_at": int64(0),
				"override_by":         0,
				"override_created_at": int64(0),
				"updated_at":          now,
				"version":             gorm.Expr("version + 1"),
			}
			if before.StatusSource == "override" {
				updates["effective_status"] = before.ObservedStatus
				updates["status_source"] = "observed"
			}
			result := tx.Model(&StatusComponent{}).
				Where("id = ? AND version = ? AND override_expires_at > 0 AND override_expires_at <= ?", before.ID, before.Version, now).
				Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				continue
			}
			var after StatusComponent
			if err := tx.First(&after, before.ID).Error; err != nil {
				return err
			}
			if err := createStatusAuditEvent(tx, StatusAuditMutation{
				ActorType: "automation", Action: "status.override.expire", Reason: "manual override expired", CreatedAt: now,
			}, "component", strconv.FormatInt(before.ID, 10), before, after); err != nil {
				return err
			}
			updated = append(updated, after)
		}
		return nil
	})
	return updated, err
}

func UpsertStatusPeriod(period *StatusPeriod) error {
	if period == nil || period.ComponentID == 0 || period.Granularity == "" {
		return errors.New("invalid status period")
	}
	return upsertStatusPeriod(DB, period)
}

func UpsertStatusPeriodWithFence(jobName string, holder string, fencingToken int64, now int64, period *StatusPeriod) error {
	if period == nil || period.ComponentID == 0 || period.Granularity == "" {
		return errors.New("invalid status period")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := validateStatusJobFence(tx, jobName, holder, fencingToken, now); err != nil {
			return err
		}
		return upsertStatusPeriod(tx, period)
	})
}

func CreateStatusProbeResultWithFence(jobName string, holder string, fencingToken int64, now int64, result *StatusProbeResult) error {
	if result == nil || result.ComponentID == 0 {
		return errors.New("invalid status probe result")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := validateStatusJobFence(tx, jobName, holder, fencingToken, now); err != nil {
			return err
		}
		return tx.Create(result).Error
	})
}

func ValidateStatusJobFence(jobName string, holder string, fencingToken int64, now int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return validateStatusJobFence(tx, jobName, holder, fencingToken, now)
	})
}

func GetStatusComponents() ([]StatusComponent, error) {
	var components []StatusComponent
	err := DB.Order("kind DESC, model_name ASC").Find(&components).Error
	return components, err
}

func GetLatestStatusProbeResults(componentIDs []int64, since int64) (map[int64]StatusProbeResult, error) {
	results := make(map[int64]StatusProbeResult, len(componentIDs))
	if len(componentIDs) == 0 {
		return results, nil
	}
	var probes []StatusProbeResult
	if err := DB.Where("component_id IN ? AND created_at >= ?", componentIDs, since).Order("created_at DESC, id DESC").Find(&probes).Error; err != nil {
		return nil, err
	}
	for _, probe := range probes {
		if _, ok := results[probe.ComponentID]; !ok {
			results[probe.ComponentID] = probe
		}
	}
	return results, nil
}

func GetStatusPeriodsInRange(granularity string, start int64, end int64) ([]StatusPeriod, error) {
	var periods []StatusPeriod
	err := DB.Where("granularity = ? AND period_start >= ? AND period_start < ?", granularity, start, end).
		Order("component_id ASC, period_start ASC").Find(&periods).Error
	return periods, err
}

func DeleteStatusHistoryWithFence(jobName string, holder string, fencingToken int64, now int64, rawCutoff int64, aggregateCutoff int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := validateStatusJobFence(tx, jobName, holder, fencingToken, now); err != nil {
			return err
		}
		if err := tx.Where("created_at < ?", rawCutoff).Delete(&StatusProbeResult{}).Error; err != nil {
			return err
		}
		if err := tx.Where("granularity = ? AND period_start < ?", StatusGranularityFiveMinutes, rawCutoff).Delete(&StatusPeriod{}).Error; err != nil {
			return err
		}
		return tx.Where("granularity IN ? AND period_start < ?", []string{StatusGranularityHour, StatusGranularityDay}, aggregateCutoff).
			Delete(&StatusPeriod{}).Error
	})
}

func upsertStatusIncidentDraftTx(tx *gorm.DB, input StatusIncidentDraftMutation) (StatusIncident, StatusIncidentUpdate, error) {
	var incident StatusIncident
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("idempotency_key = ?", input.Incident.IdempotencyKey).
		First(&incident).Error
	created := false
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		incident = input.Incident
		if err := tx.Create(&incident).Error; err != nil {
			return StatusIncident{}, StatusIncidentUpdate{}, err
		}
		created = true
	case err != nil:
		return StatusIncident{}, StatusIncidentUpdate{}, err
	}

	var beforeIncident *StatusIncident
	if !created {
		copyBefore := incident
		beforeIncident = &copyBefore
	}
	var beforeDraft *StatusIncidentUpdate
	if !created && incident.Visibility != "public" {
		result := tx.Model(&StatusIncident{}).
			Where("id = ? AND version = ?", incident.ID, incident.Version).
			Updates(map[string]any{
				"title":      input.Incident.Title,
				"impact":     input.Incident.Impact,
				"status":     "draft",
				"visibility": "private",
				"updated_at": input.Incident.UpdatedAt,
				"version":    gorm.Expr("version + 1"),
			})
		if result.Error != nil {
			return StatusIncident{}, StatusIncidentUpdate{}, result.Error
		}
		if result.RowsAffected == 0 {
			return StatusIncident{}, StatusIncidentUpdate{}, ErrStatusVersionConflict
		}
		if err := tx.First(&incident, incident.ID).Error; err != nil {
			return StatusIncident{}, StatusIncidentUpdate{}, err
		}
	}

	var association StatusIncidentComponent
	var beforeAssociation *StatusIncidentComponent
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("incident_id = ? AND component_id = ?", incident.ID, input.ComponentID).
		First(&association).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		association = StatusIncidentComponent{IncidentID: incident.ID, ComponentID: input.ComponentID}
		if err := tx.Create(&association).Error; err != nil {
			return StatusIncident{}, StatusIncidentUpdate{}, err
		}
	case err != nil:
		return StatusIncident{}, StatusIncidentUpdate{}, err
	default:
		copyBefore := association
		beforeAssociation = &copyBefore
	}

	var draft StatusIncidentUpdate
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("incident_id = ? AND published = ?", incident.ID, false).
		Order("id DESC").First(&draft).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		draft = input.Draft
		draft.IncidentID = incident.ID
		if err := tx.Create(&draft).Error; err != nil {
			return StatusIncident{}, StatusIncidentUpdate{}, err
		}
	case err != nil:
		return StatusIncident{}, StatusIncidentUpdate{}, err
	default:
		copyBefore := draft
		beforeDraft = &copyBefore
		if err := tx.Model(&StatusIncidentUpdate{}).Where("id = ? AND published = ?", draft.ID, false).Updates(map[string]any{
			"state":      input.Draft.State,
			"body":       input.Draft.Body,
			"actor_id":   input.Draft.ActorID,
			"created_at": input.Draft.CreatedAt,
		}).Error; err != nil {
			return StatusIncident{}, StatusIncidentUpdate{}, err
		}
		if err := tx.First(&draft, draft.ID).Error; err != nil {
			return StatusIncident{}, StatusIncidentUpdate{}, err
		}
	}

	before := struct {
		Incident    *StatusIncident          `json:"incident"`
		Draft       *StatusIncidentUpdate    `json:"draft"`
		Association *StatusIncidentComponent `json:"component_association"`
	}{Incident: beforeIncident, Draft: beforeDraft, Association: beforeAssociation}
	after := struct {
		Incident    StatusIncident          `json:"incident"`
		Draft       StatusIncidentUpdate    `json:"draft"`
		Association StatusIncidentComponent `json:"component_association"`
	}{Incident: incident, Draft: draft, Association: association}
	if err := createStatusAuditEvent(tx, input.Audit, "incident", strconv.FormatInt(incident.ID, 10), before, after); err != nil {
		return StatusIncident{}, StatusIncidentUpdate{}, err
	}
	return incident, draft, nil
}

func createStatusAuditEvent(tx *gorm.DB, input StatusAuditMutation, objectType string, objectID string, before any, after any) error {
	beforeJSON, err := common.Marshal(before)
	if err != nil {
		return err
	}
	afterJSON, err := common.Marshal(after)
	if err != nil {
		return err
	}
	return tx.Create(&StatusAuditEvent{
		ActorID:    input.ActorID,
		ActorType:  input.ActorType,
		Action:     input.Action,
		ObjectType: objectType,
		ObjectID:   objectID,
		BeforeJSON: string(beforeJSON),
		AfterJSON:  string(afterJSON),
		Reason:     input.Reason,
		CreatedAt:  input.CreatedAt,
	}).Error
}

func validateStatusJobFence(tx *gorm.DB, jobName string, holder string, fencingToken int64, now int64) error {
	var lease StatusJobLease
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("name = ?", jobName).First(&lease).Error; err != nil {
		return err
	}
	if lease.Holder != holder || lease.FencingToken != fencingToken || lease.ExpiresAt <= now {
		return fmt.Errorf("status job lease is no longer owned")
	}
	return nil
}

func upsertStatusPeriod(db *gorm.DB, period *StatusPeriod) error {
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "component_id"}, {Name: "granularity"}, {Name: "period_start"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"score_sum_micros", "known_bucket_count", "unknown_bucket_count", "maintenance_bucket_count",
			"worst_status", "eligible_count", "success_count", "probe_success_count", "probe_failure_count",
			"latency_sum_ms", "latency_count", "ttft_sum_ms", "ttft_count", "updated_at",
		}),
	}).Create(period).Error
}
