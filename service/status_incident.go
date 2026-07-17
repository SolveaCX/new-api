package service

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	statusForceGreenMaxSeconds                  = int64(60 * 60)
	statusEvidenceAuthorizationLabelPattern     = `(?:authorization|proxy[-_ ]*authorization)`
	statusEvidenceCredentialLabelPattern        = `(?:secret|client[-_ ]*secret|token|access[-_ ]*token|refresh[-_ ]*token|api[-_ ]*key|x[-_ ]*api[-_ ]*key|password)`
	statusEvidenceSensitiveNormalizedKeyPattern = `(?:` + statusEvidenceAuthorizationLabelPattern + `|` + statusEvidenceCredentialLabelPattern + `)`
)

var (
	statusEvidenceAuthorizationPattern = regexp.MustCompile(`(?i)(\b` + statusEvidenceAuthorizationLabelPattern + `\b["']?\s*[:=]\s*(?:(?:bearer|basic|token)\s*)?)(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s,;"'}\]]+)`)
	statusEvidenceCredentialPattern    = regexp.MustCompile(`(?i)(\b` + statusEvidenceCredentialLabelPattern + `\b["']?\s*[:=]\s*)(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s,;"'}\]]+)`)
	statusEvidenceSensitiveKeyPattern  = regexp.MustCompile(`(?i)^` + statusEvidenceSensitiveNormalizedKeyPattern + `$`)
	statusEvidenceSecretKeyPattern     = regexp.MustCompile(`(?i)\bsk-[[:alnum:]][[:alnum:]_.-]*`)
)

var (
	ErrStatusAdminRequired                 = errors.New("status mutation requires an administrator")
	ErrStatusRootRequired                  = errors.New("force-green status override requires root authorization")
	ErrStatusSecureVerificationRequired    = errors.New("force-green status override requires secure verification")
	ErrStatusForceGreenTooLong             = errors.New("force-green status override cannot exceed one hour")
	ErrStatusInvalidMutation               = errors.New("invalid status mutation")
	ErrStatusMaintenanceNotPublished       = model.ErrStatusMaintenanceNotPublished
	ErrStatusMaintenanceOverlap            = model.ErrStatusMaintenanceOverlap
	ErrStatusMaintenanceRequiresTransition = model.ErrStatusMaintenanceRequiresTransition
)

type StatusMutationActor struct {
	ID             int
	Role           int
	ActorType      string
	SecureVerified bool
}

type StatusIncidentAutomationInput struct {
	ComponentID            int64
	PreviousObservedStatus string
	ObservedStatus         string
	EvidenceSummary        string
	IdempotencyKey         string
	Now                    int64
}

type StatusIncidentAutomationResult struct {
	Incident *model.StatusIncident
	Draft    *model.StatusIncidentUpdate
}

type StatusDeliveryDestination struct {
	Type string
	ID   int64
}

type StatusIncidentPublishInput struct {
	IncidentID      int64
	ExpectedVersion int64
	State           string
	Body            string
	EventID         string
	Destinations    []StatusDeliveryDestination
	Actor           StatusMutationActor
	Reason          string
	Now             int64
}

type StatusIncidentPublishResult struct {
	Incident model.StatusIncident
	Update   model.StatusIncidentUpdate
}

type StatusMaintenanceDraftInput struct {
	Title            string
	Body             string
	Impact           string
	IdempotencyKey   string
	ComponentIDs     []int64
	ScheduledStartAt int64
	ScheduledEndAt   int64
	Actor            StatusMutationActor
	Reason           string
	Now              int64
}

type StatusMaintenanceTransitionInput struct {
	IncidentID      int64
	ExpectedVersion int64
	Now             int64
}

type StatusOverrideInput struct {
	ComponentID     int64
	ExpectedVersion int64
	Status          string
	Reason          string
	ExpiresAt       int64
	Actor           StatusMutationActor
	Now             int64
}

func ReconcileStatusIncidentAutomation(input StatusIncidentAutomationInput) (StatusIncidentAutomationResult, error) {
	if input.ComponentID <= 0 || input.Now <= 0 || strings.TrimSpace(input.IdempotencyKey) == "" {
		return StatusIncidentAutomationResult{}, ErrStatusInvalidMutation
	}

	unhealthy := input.PreviousObservedStatus == model.StatusOperational &&
		(input.ObservedStatus == model.StatusDegraded || input.ObservedStatus == model.StatusOutage)
	recovered := isUnhealthyStatus(input.PreviousObservedStatus) && input.ObservedStatus == model.StatusOperational
	if !unhealthy && !recovered {
		return StatusIncidentAutomationResult{}, nil
	}

	state := "investigating"
	impact := input.ObservedStatus
	title := "Automated component status review"
	bodyPrefix := "Automated evidence: "
	if recovered {
		state = "resolved"
		impact = "none"
		title = "Automated recovery review"
		bodyPrefix = "Recovery observed; review this resolution suggestion: "
	}
	body := bodyPrefix + sanitizeStatusEvidence(input.EvidenceSummary)

	incident, draft, err := model.UpsertStatusIncidentDraft(model.StatusIncidentDraftMutation{
		Incident: model.StatusIncident{
			PublicID:       "inc_" + common.GetUUID(),
			Kind:           model.StatusIncidentKindIncident,
			Title:          title,
			Impact:         impact,
			Status:         "draft",
			Visibility:     "private",
			AutomationMode: "automatic",
			IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
			Version:        1,
			CreatedAt:      input.Now,
			UpdatedAt:      input.Now,
		},
		ComponentID: input.ComponentID,
		Draft: model.StatusIncidentUpdate{
			EventID:   "draft_" + common.GetUUID(),
			State:     state,
			Body:      body,
			Published: false,
			CreatedAt: input.Now,
		},
		Audit: model.StatusAuditMutation{
			ActorType: "automation",
			Action:    "status.incident.draft.auto",
			Reason:    "observed component status transition",
			CreatedAt: input.Now,
		},
	})
	if err != nil {
		return StatusIncidentAutomationResult{}, err
	}
	return StatusIncidentAutomationResult{Incident: &incident, Draft: &draft}, nil
}

func PublishStatusIncidentUpdate(input StatusIncidentPublishInput) (StatusIncidentPublishResult, error) {
	actor, err := requireStatusAdmin(input.Actor)
	if err != nil {
		return StatusIncidentPublishResult{}, err
	}
	if input.IncidentID <= 0 || input.ExpectedVersion <= 0 || input.Now <= 0 ||
		strings.TrimSpace(input.Body) == "" || strings.TrimSpace(input.EventID) == "" || strings.TrimSpace(input.Reason) == "" ||
		!validPublishedIncidentState(input.State) {
		return StatusIncidentPublishResult{}, ErrStatusInvalidMutation
	}

	destinations := make([]model.StatusDeliveryDestinationMutation, 0, len(input.Destinations))
	seen := make(map[string]struct{}, len(input.Destinations))
	for _, destination := range input.Destinations {
		if destination.ID <= 0 || !validStatusDestination(destination.Type) {
			return StatusIncidentPublishResult{}, ErrStatusInvalidMutation
		}
		key := fmt.Sprintf("%s:%d", destination.Type, destination.ID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		destinations = append(destinations, model.StatusDeliveryDestinationMutation{
			Type:    destination.Type,
			ID:      destination.ID,
			EventID: statusDeliveryEventID(input.EventID, destination.Type, destination.ID),
		})
	}

	incident, update, err := model.PublishStatusIncidentUpdate(model.StatusIncidentPublishMutation{
		IncidentID:      input.IncidentID,
		ExpectedVersion: input.ExpectedVersion,
		Update: model.StatusIncidentUpdate{
			EventID:     strings.TrimSpace(input.EventID),
			State:       input.State,
			Body:        strings.TrimSpace(input.Body),
			Published:   true,
			PublishedAt: input.Now,
			ActorID:     actor.ID,
			CreatedAt:   input.Now,
		},
		Destinations: destinations,
		Audit: model.StatusAuditMutation{
			ActorID:   actor.ID,
			ActorType: actor.ActorType,
			Action:    "status.incident.publish",
			Reason:    strings.TrimSpace(input.Reason),
			CreatedAt: input.Now,
		},
	})
	if err != nil {
		return StatusIncidentPublishResult{}, err
	}
	return StatusIncidentPublishResult{Incident: incident, Update: update}, nil
}

func CreateStatusMaintenanceDraft(input StatusMaintenanceDraftInput) (model.StatusIncident, error) {
	actor, err := requireStatusAdmin(input.Actor)
	if err != nil {
		return model.StatusIncident{}, err
	}
	if strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.Body) == "" || strings.TrimSpace(input.Reason) == "" ||
		strings.TrimSpace(input.IdempotencyKey) == "" || len(input.ComponentIDs) == 0 || input.Now <= 0 ||
		input.ScheduledStartAt <= input.Now || input.ScheduledEndAt <= input.ScheduledStartAt {
		return model.StatusIncident{}, ErrStatusInvalidMutation
	}
	for _, componentID := range input.ComponentIDs {
		if componentID <= 0 {
			return model.StatusIncident{}, ErrStatusInvalidMutation
		}
	}
	impact := strings.TrimSpace(input.Impact)
	if impact == "" {
		impact = "maintenance"
	}
	return model.CreateStatusMaintenanceDraft(model.StatusMaintenanceDraftMutation{
		Incident: model.StatusIncident{
			PublicID:         "mnt_" + common.GetUUID(),
			Kind:             model.StatusIncidentKindMaintenance,
			Title:            strings.TrimSpace(input.Title),
			Impact:           impact,
			Status:           "draft",
			Visibility:       "private",
			AutomationMode:   "manual",
			IdempotencyKey:   strings.TrimSpace(input.IdempotencyKey),
			ScheduledStartAt: input.ScheduledStartAt,
			ScheduledEndAt:   input.ScheduledEndAt,
			Version:          1,
			CreatedBy:        actor.ID,
			CreatedAt:        input.Now,
			UpdatedAt:        input.Now,
		},
		ComponentIDs: input.ComponentIDs,
		Draft: model.StatusIncidentUpdate{
			EventID:   "draft_" + common.GetUUID(),
			State:     "identified",
			Body:      strings.TrimSpace(input.Body),
			Published: false,
			ActorID:   actor.ID,
			CreatedAt: input.Now,
		},
		Audit: model.StatusAuditMutation{
			ActorID:   actor.ID,
			ActorType: actor.ActorType,
			Action:    "status.maintenance.create",
			Reason:    strings.TrimSpace(input.Reason),
			CreatedAt: input.Now,
		},
	})
}

func ReconcileStatusMaintenance(input StatusMaintenanceTransitionInput) (model.StatusIncident, error) {
	if input.IncidentID <= 0 || input.ExpectedVersion <= 0 || input.Now <= 0 {
		return model.StatusIncident{}, ErrStatusInvalidMutation
	}
	return model.TransitionStatusMaintenance(model.StatusMaintenanceTransitionMutation{
		IncidentID:      input.IncidentID,
		ExpectedVersion: input.ExpectedVersion,
		Now:             input.Now,
	})
}

func ApplyStatusOverride(input StatusOverrideInput) (model.StatusComponent, error) {
	actor, err := requireStatusAdmin(input.Actor)
	if err != nil {
		return model.StatusComponent{}, err
	}
	if input.ComponentID <= 0 || input.ExpectedVersion <= 0 || input.Now <= 0 || input.ExpiresAt <= input.Now ||
		strings.TrimSpace(input.Reason) == "" || !validOverrideStatus(input.Status) {
		return model.StatusComponent{}, ErrStatusInvalidMutation
	}
	if input.Status == model.StatusOperational {
		if actor.Role < common.RoleRootUser {
			return model.StatusComponent{}, ErrStatusRootRequired
		}
		if !actor.SecureVerified {
			return model.StatusComponent{}, ErrStatusSecureVerificationRequired
		}
		if input.ExpiresAt-input.Now > statusForceGreenMaxSeconds {
			return model.StatusComponent{}, ErrStatusForceGreenTooLong
		}
	}
	return model.SetStatusComponentOverride(model.StatusOverrideMutation{
		ComponentID:     input.ComponentID,
		ExpectedVersion: input.ExpectedVersion,
		Status:          input.Status,
		Reason:          strings.TrimSpace(input.Reason),
		ExpiresAt:       input.ExpiresAt,
		ActorID:         actor.ID,
		Now:             input.Now,
		Audit: model.StatusAuditMutation{
			ActorID:   actor.ID,
			ActorType: actor.ActorType,
			Action:    "status.override.set",
			Reason:    strings.TrimSpace(input.Reason),
			CreatedAt: input.Now,
		},
	})
}

func ExpireStatusOverrides(now int64) ([]model.StatusComponent, error) {
	if now <= 0 {
		return nil, ErrStatusInvalidMutation
	}
	return model.ExpireStatusComponentOverrides(now)
}

func requireStatusAdmin(actor StatusMutationActor) (StatusMutationActor, error) {
	if actor.ID <= 0 || actor.Role < common.RoleAdminUser {
		return StatusMutationActor{}, ErrStatusAdminRequired
	}
	if strings.TrimSpace(actor.ActorType) == "" {
		if actor.Role >= common.RoleRootUser {
			actor.ActorType = "root"
		} else {
			actor.ActorType = "admin"
		}
	}
	return actor, nil
}

func validPublishedIncidentState(state string) bool {
	switch state {
	case "investigating", "identified", "monitoring", "resolved":
		return true
	default:
		return false
	}
}

func validStatusDestination(destinationType string) bool {
	switch destinationType {
	case model.StatusDestinationEmail, model.StatusDestinationWebhook, model.StatusDestinationDiscord:
		return true
	default:
		return false
	}
}

func validOverrideStatus(status string) bool {
	switch status {
	case model.StatusOperational, model.StatusDegraded, model.StatusOutage, model.StatusUnknown, model.StatusMaintenance:
		return true
	default:
		return false
	}
}

func statusDeliveryEventID(eventID string, destinationType string, destinationID int64) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", eventID, destinationType, destinationID)))
	return fmt.Sprintf("delivery_%x", digest[:24])
}

func sanitizeStatusEvidence(summary string) string {
	trimmed := strings.TrimSpace(strings.ToValidUTF8(summary, "�"))
	if trimmed == "" {
		return "No public-safe evidence detail was available."
	}

	var document any
	if err := common.Unmarshal([]byte(trimmed), &document); err == nil {
		document = redactStatusEvidenceJSON(document)
		if sanitized, err := common.Marshal(document); err == nil {
			return truncateStatusEvidence(string(sanitized))
		}
	}

	sanitized := redactStatusEvidenceText(trimmed)
	return truncateStatusEvidence(strings.Join(strings.Fields(sanitized), " "))
}

func redactStatusEvidenceText(value string) string {
	redacted := statusEvidenceAuthorizationPattern.ReplaceAllString(value, "${1}[REDACTED]")
	redacted = statusEvidenceCredentialPattern.ReplaceAllString(redacted, "${1}[REDACTED]")
	return statusEvidenceSecretKeyPattern.ReplaceAllString(redacted, "[REDACTED]")
}

func redactStatusEvidenceJSON(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if isSensitiveStatusEvidenceKey(key) {
				typed[key] = "[REDACTED]"
				continue
			}
			typed[key] = redactStatusEvidenceJSON(child)
		}
	case []any:
		for i, child := range typed {
			typed[i] = redactStatusEvidenceJSON(child)
		}
	case string:
		return redactStatusEvidenceText(typed)
	}
	return value
}

func isSensitiveStatusEvidenceKey(key string) bool {
	normalized := strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(key)))
	return statusEvidenceSensitiveKeyPattern.MatchString(normalized)
}

func truncateStatusEvidence(summary string) string {
	if len(summary) <= 1_000 {
		return summary
	}
	boundary := 1_000
	for boundary > 0 && !utf8.ValidString(summary[:boundary]) {
		boundary--
	}
	return summary[:boundary]
}
