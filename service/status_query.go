package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type StatusComponentFilter struct {
	Kind       string
	Query      string
	Capability string
	Status     string
}

type StatusIncidentRecord struct {
	Incident     model.StatusIncident
	Updates      []model.StatusIncidentUpdate
	ComponentIDs []int64
}

func QueryStatusComponents(filter StatusComponentFilter) ([]model.StatusComponent, error) {
	components, err := model.GetStatusComponents()
	if err != nil {
		return nil, err
	}
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	result := make([]model.StatusComponent, 0, len(components))
	for _, component := range components {
		if filter.Kind != "" && component.Kind != filter.Kind {
			continue
		}
		if filter.Capability != "" && component.Capability != filter.Capability {
			continue
		}
		if filter.Status != "" && component.EffectiveStatus != filter.Status {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(component.DisplayName+" "+component.Slug), query) {
			continue
		}
		result = append(result, component)
	}
	return result, nil
}

func ProjectStatusComponentFreshness(components []model.StatusComponent, now int64, maxAge int64) []model.StatusComponent {
	projected := make([]model.StatusComponent, 0, len(components))
	for _, component := range components {
		if component.EffectiveStatus != model.StatusMaintenance &&
			(component.LastTrustworthyUpdateAt <= 0 || now-component.LastTrustworthyUpdateAt >= maxAge) {
			component.EffectiveStatus = model.StatusUnknown
			component.CoverageMicros = 0
		}
		projected = append(projected, component)
	}
	return projected
}

func GetStatusComponent(slug string) (model.StatusComponent, error) {
	return model.GetStatusComponentBySlug(strings.TrimSpace(slug))
}

func GetStatusComponentHistory(slug string, granularity string, start int64, end int64) (model.StatusComponent, []model.StatusPeriod, error) {
	component, err := GetStatusComponent(slug)
	if err != nil {
		return model.StatusComponent{}, nil, err
	}
	periods, err := model.GetStatusPeriodsForComponentInRange(component.ID, granularity, start, end)
	return component, periods, err
}

func ListStatusIncidentRecords(kind string, publicOnly bool, limit int) ([]StatusIncidentRecord, error) {
	incidents, err := model.GetStatusIncidents(kind, publicOnly, limit)
	if err != nil {
		return nil, err
	}
	records := make([]StatusIncidentRecord, 0, len(incidents))
	if len(incidents) == 0 {
		return records, nil
	}
	incidentIDs := make([]int64, 0, len(incidents))
	for _, incident := range incidents {
		incidentIDs = append(incidentIDs, incident.ID)
	}
	updates, err := model.GetStatusIncidentUpdatesForIncidentIDs(incidentIDs, publicOnly)
	if err != nil {
		return nil, err
	}
	associations, err := model.GetStatusIncidentComponentsForIncidentIDs(incidentIDs)
	if err != nil {
		return nil, err
	}
	updatesByIncident := make(map[int64][]model.StatusIncidentUpdate, len(incidents))
	for _, update := range updates {
		updatesByIncident[update.IncidentID] = append(updatesByIncident[update.IncidentID], update)
	}
	componentIDsByIncident := make(map[int64][]int64, len(incidents))
	for _, association := range associations {
		componentIDsByIncident[association.IncidentID] = append(componentIDsByIncident[association.IncidentID], association.ComponentID)
	}
	for _, incident := range incidents {
		incidentUpdates := updatesByIncident[incident.ID]
		if incidentUpdates == nil {
			incidentUpdates = make([]model.StatusIncidentUpdate, 0)
		}
		componentIDs := componentIDsByIncident[incident.ID]
		if componentIDs == nil {
			componentIDs = make([]int64, 0)
		}
		records = append(records, StatusIncidentRecord{
			Incident: incident, Updates: incidentUpdates, ComponentIDs: componentIDs,
		})
	}
	return records, nil
}

func GetPublicStatusIncidentRecord(publicID string) (StatusIncidentRecord, error) {
	incident, err := model.GetStatusIncidentByPublicID(strings.TrimSpace(publicID), true)
	if err != nil {
		return StatusIncidentRecord{}, err
	}
	return statusIncidentRecord(incident, true)
}

func GetAdminStatusIncidentRecord(id int64) (StatusIncidentRecord, error) {
	incident, err := model.GetStatusIncidentByID(id)
	if err != nil {
		return StatusIncidentRecord{}, err
	}
	return statusIncidentRecord(incident, false)
}

func statusIncidentRecord(incident model.StatusIncident, publishedOnly bool) (StatusIncidentRecord, error) {
	updates, err := model.GetStatusIncidentUpdates(incident.ID, publishedOnly)
	if err != nil {
		return StatusIncidentRecord{}, err
	}
	componentIDs, err := model.GetStatusIncidentComponentIDs(incident.ID)
	if err != nil {
		return StatusIncidentRecord{}, err
	}
	return StatusIncidentRecord{Incident: incident, Updates: updates, ComponentIDs: componentIDs}, nil
}

func ListStatusSettings(limit int) ([]model.StatusSetting, error) {
	return model.GetStatusSettings(limit)
}

func ListStatusSubscribers(limit int) ([]model.StatusSubscriber, error) {
	return model.GetStatusSubscribers(limit)
}

func ListStatusDeliveries(limit int) ([]model.StatusDeliveryOutbox, error) {
	return model.GetStatusDeliveries(limit)
}

func ListStatusAuditEvents(limit int) ([]model.StatusAuditEvent, error) {
	return model.GetStatusAuditEvents(limit)
}

func UpdateStatusSetting(actor StatusMutationActor, key string, value string, expectedVersion int64, now int64) (model.StatusSetting, error) {
	actor, err := requireStatusAdmin(actor)
	if err != nil {
		return model.StatusSetting{}, err
	}
	if actor.Role < common.RoleRootUser {
		return model.StatusSetting{}, ErrStatusRootRequired
	}
	if !actor.SecureVerified {
		return model.StatusSetting{}, ErrStatusSecureVerificationRequired
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if !validStatusSettingKey(key) || strings.HasPrefix(key, "status.discord.") || value == "" || len(value) > 4096 || expectedVersion < 0 || now <= 0 {
		return model.StatusSetting{}, ErrStatusInvalidMutation
	}
	return model.UpdateStatusSettingVersion(model.StatusSetting{
		Key: key, Value: value, Version: 1, UpdatedBy: actor.ID, UpdatedAt: now,
	}, expectedVersion)
}

func validStatusSettingKey(key string) bool {
	if key == "" || len(key) > 128 {
		return false
	}
	for _, character := range key {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}
