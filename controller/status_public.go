package controller

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	statusPublicCacheSeconds      = int64(30)
	statusPublicEvidenceMaxAge    = int64(20 * 60)
	statusSubscriptionMaxBodySize = int64(4 * 1024)
	statusPublicMaxFilterLength   = 100
)

type statusPublicMetadata struct {
	GeneratedAt             int64 `json:"generated_at"`
	LastTrustworthyUpdateAt int64 `json:"last_trustworthy_update_at"`
	Coverage                int64 `json:"coverage"`
}

type statusPublicComponent struct {
	ID                      int64  `json:"id"`
	Slug                    string `json:"slug"`
	Kind                    string `json:"kind"`
	DisplayName             string `json:"display_name"`
	Capability              string `json:"capability,omitempty"`
	Lifecycle               string `json:"lifecycle"`
	Status                  string `json:"status"`
	LastTrustworthyUpdateAt int64  `json:"last_trustworthy_update_at"`
	Coverage                int64  `json:"coverage"`
}

type statusPublicPeriod struct {
	PeriodStart      int64  `json:"period_start"`
	Availability     int64  `json:"availability"`
	Coverage         int64  `json:"coverage"`
	Status           string `json:"status"`
	MaintenanceCount int64  `json:"maintenance_count,omitempty"`
}

type statusPublicIncidentUpdate struct {
	State       string `json:"state"`
	Body        string `json:"body"`
	PublishedAt int64  `json:"published_at"`
}

type statusPublicIncident struct {
	ID               string                       `json:"id"`
	Kind             string                       `json:"kind"`
	Title            string                       `json:"title"`
	Impact           string                       `json:"impact"`
	Status           string                       `json:"status"`
	ScheduledStartAt int64                        `json:"scheduled_start_at,omitempty"`
	ScheduledEndAt   int64                        `json:"scheduled_end_at,omitempty"`
	StartedAt        int64                        `json:"started_at,omitempty"`
	ResolvedAt       int64                        `json:"resolved_at,omitempty"`
	UpdatedAt        int64                        `json:"updated_at"`
	Components       []int64                      `json:"component_ids"`
	Updates          []statusPublicIncidentUpdate `json:"updates"`
}

func GetPublicStatusSummary(c *gin.Context) {
	now := time.Now().Unix()
	generatedAt := statusGeneratedAt(now)
	components, err := service.QueryStatusComponents(service.StatusComponentFilter{})
	if err != nil {
		statusPublicError(c, http.StatusServiceUnavailable, errors.New("status data unavailable"))
		return
	}
	publicComponents, honestComponents := publicStatusComponents(components, now)
	metadata := publicStatusMetadata(honestComponents, generatedAt)
	overall := service.OverallStatus(honestComponents)
	message := ""
	if overall == service.OverallMonitoringIncomplete {
		message = "monitoring unavailable"
	}
	statusPublicSuccess(c, struct {
		statusPublicMetadata
		Status     string                  `json:"status"`
		Message    string                  `json:"message"`
		Components []statusPublicComponent `json:"components"`
	}{metadata, overall, message, publicComponents})
}

func GetPublicStatusComponents(c *gin.Context) {
	filter, err := statusComponentFilter(c)
	if err != nil {
		statusPublicError(c, http.StatusBadRequest, err)
		return
	}
	publicStatusFilter := filter.Status
	filter.Status = ""
	components, err := service.QueryStatusComponents(filter)
	if err != nil {
		statusPublicError(c, http.StatusServiceUnavailable, errors.New("status data unavailable"))
		return
	}
	now := time.Now().Unix()
	generatedAt := statusGeneratedAt(now)
	publicComponents, honestComponents := publicStatusComponents(components, now)
	if publicStatusFilter != "" {
		filteredPublic := make([]statusPublicComponent, 0, len(publicComponents))
		filteredHonest := make([]model.StatusComponent, 0, len(honestComponents))
		for index, component := range honestComponents {
			if component.EffectiveStatus != publicStatusFilter {
				continue
			}
			filteredPublic = append(filteredPublic, publicComponents[index])
			filteredHonest = append(filteredHonest, component)
		}
		publicComponents = filteredPublic
		honestComponents = filteredHonest
	}
	statusPublicSuccess(c, struct {
		statusPublicMetadata
		Components []statusPublicComponent `json:"components"`
	}{publicStatusMetadata(honestComponents, generatedAt), publicComponents})
}

func GetPublicStatusComponent(c *gin.Context) {
	component, err := service.GetStatusComponent(c.Param("slug"))
	if err != nil {
		statusPublicQueryError(c, err)
		return
	}
	now := time.Now().Unix()
	generatedAt := statusGeneratedAt(now)
	publicComponents, honestComponents := publicStatusComponents([]model.StatusComponent{component}, now)
	statusPublicSuccess(c, struct {
		statusPublicMetadata
		Component statusPublicComponent `json:"component"`
	}{publicStatusMetadata(honestComponents, generatedAt), publicComponents[0]})
}

func GetPublicStatusComponentHistory(c *gin.Context) {
	rangeName := strings.TrimSpace(c.DefaultQuery("range", "24h"))
	now := time.Now().Unix()
	generatedAt := statusGeneratedAt(now)
	start, granularity, ok := statusHistoryRange(rangeName, now)
	if !ok {
		statusPublicError(c, http.StatusBadRequest, errors.New("range must be one of 24h, 7d, 30d, or 90d"))
		return
	}
	component, periods, err := service.GetStatusComponentHistory(c.Param("slug"), granularity, start, now+1)
	if err != nil {
		statusPublicQueryError(c, err)
		return
	}
	publicComponents, honestComponents := publicStatusComponents([]model.StatusComponent{component}, now)
	publicPeriods := make([]statusPublicPeriod, 0, len(periods))
	availability := service.AggregateStatusPeriods(periods)
	for _, period := range periods {
		periodAvailability := service.AggregateStatusPeriods([]model.StatusPeriod{period})
		publicPeriods = append(publicPeriods, statusPublicPeriod{
			PeriodStart: period.PeriodStart, Availability: periodAvailability.AvailabilityMicros,
			Coverage: periodAvailability.CoverageMicros, Status: period.WorstStatus,
			MaintenanceCount: period.MaintenanceBucketCount,
		})
	}
	metadata := publicStatusMetadata(honestComponents, generatedAt)
	metadata.Coverage = availability.CoverageMicros
	statusPublicSuccess(c, struct {
		statusPublicMetadata
		Component    statusPublicComponent      `json:"component"`
		Range        string                     `json:"range"`
		Availability service.StatusAvailability `json:"availability"`
		Periods      []statusPublicPeriod       `json:"periods"`
	}{metadata, publicComponents[0], rangeName, availability, publicPeriods})
}

func GetPublicStatusIncidents(c *gin.Context) {
	getPublicStatusIncidentList(c, model.StatusIncidentKindIncident, "incidents")
}

func GetPublicStatusIncident(c *gin.Context) {
	record, err := service.GetPublicStatusIncidentRecord(c.Param("id"))
	if err != nil {
		statusPublicQueryError(c, err)
		return
	}
	metadata, err := currentPublicStatusMetadata()
	if err != nil {
		statusPublicError(c, http.StatusServiceUnavailable, errors.New("status data unavailable"))
		return
	}
	statusPublicSuccess(c, struct {
		statusPublicMetadata
		Incident statusPublicIncident `json:"incident"`
	}{metadata, publicStatusIncident(record)})
}

func GetPublicStatusMaintenance(c *gin.Context) {
	getPublicStatusIncidentList(c, model.StatusIncidentKindMaintenance, "maintenance")
}

func CreatePublicStatusSubscription(c *gin.Context) {
	var request struct {
		Email        string  `json:"email"`
		ComponentIDs []int64 `json:"component_ids"`
	}
	if err := decodeStatusPublicBody(c, &request); err != nil {
		if statusBodyTooLarge(err) {
			statusGenericSubscriptionResponse(c, http.StatusRequestEntityTooLarge)
			return
		}
		statusGenericSubscriptionResponse(c, http.StatusOK)
		return
	}
	if len(request.ComponentIDs) > 100 {
		statusGenericSubscriptionResponse(c, http.StatusOK)
		return
	}
	for _, componentID := range request.ComponentIDs {
		if componentID <= 0 {
			statusGenericSubscriptionResponse(c, http.StatusOK)
			return
		}
	}
	_, _ = (service.StatusEmailSubscriptionService{}).Subscribe(c.Request.Context(), request.Email, request.ComponentIDs)
	statusGenericSubscriptionResponse(c, http.StatusOK)
}

func VerifyPublicStatusSubscription(c *gin.Context) {
	token := boundedStatusToken(c.Query("token"))
	_, _ = (service.StatusEmailSubscriptionService{}).Verify(token, time.Now().Unix())
	statusGenericSubscriptionResponse(c, http.StatusOK)
}

func PreviewPublicStatusUnsubscribe(c *gin.Context) {
	preview, err := (service.StatusEmailSubscriptionService{}).PreviewUnsubscribe(boundedStatusToken(c.Query("token")))
	if err != nil {
		preview = service.StatusUnsubscribePreview{Message: service.StatusSubscriptionGenericMessage}
	}
	common.ApiSuccess(c, preview)
}

func UnsubscribePublicStatus(c *gin.Context) {
	var request struct {
		Token string `json:"token"`
	}
	if err := decodeStatusPublicBody(c, &request); err == nil {
		_, _ = (service.StatusEmailSubscriptionService{}).Unsubscribe(boundedStatusToken(request.Token), time.Now().Unix())
	}
	statusGenericSubscriptionResponse(c, http.StatusOK)
}

func getPublicStatusIncidentList(c *gin.Context, kind string, field string) {
	records, err := service.ListStatusIncidentRecords(kind, true, 100)
	if err != nil {
		statusPublicError(c, http.StatusServiceUnavailable, errors.New("status data unavailable"))
		return
	}
	incidents := make([]statusPublicIncident, 0, len(records))
	for _, record := range records {
		incidents = append(incidents, publicStatusIncident(record))
	}
	metadata, err := currentPublicStatusMetadata()
	if err != nil {
		statusPublicError(c, http.StatusServiceUnavailable, errors.New("status data unavailable"))
		return
	}
	if field == "maintenance" {
		statusPublicSuccess(c, struct {
			statusPublicMetadata
			Maintenance []statusPublicIncident `json:"maintenance"`
		}{metadata, incidents})
		return
	}
	statusPublicSuccess(c, struct {
		statusPublicMetadata
		Incidents []statusPublicIncident `json:"incidents"`
	}{metadata, incidents})
}

func publicStatusIncident(record service.StatusIncidentRecord) statusPublicIncident {
	updates := make([]statusPublicIncidentUpdate, 0, len(record.Updates))
	for _, update := range record.Updates {
		if !update.Published {
			continue
		}
		updates = append(updates, statusPublicIncidentUpdate{
			State: update.State, Body: update.Body, PublishedAt: update.PublishedAt,
		})
	}
	incident := record.Incident
	return statusPublicIncident{
		ID: incident.PublicID, Kind: incident.Kind, Title: incident.Title, Impact: incident.Impact,
		Status: incident.Status, ScheduledStartAt: incident.ScheduledStartAt, ScheduledEndAt: incident.ScheduledEndAt,
		StartedAt: incident.StartedAt, ResolvedAt: incident.ResolvedAt, UpdatedAt: incident.UpdatedAt,
		Components: append([]int64(nil), record.ComponentIDs...), Updates: updates,
	}
}

func publicStatusComponents(components []model.StatusComponent, now int64) ([]statusPublicComponent, []model.StatusComponent) {
	publicComponents := make([]statusPublicComponent, 0, len(components))
	honestComponents := service.ProjectStatusComponentFreshness(components, now, statusPublicEvidenceMaxAge)
	for index, component := range components {
		honest := honestComponents[index]
		publicComponents = append(publicComponents, statusPublicComponent{
			ID: component.ID, Slug: component.Slug, Kind: component.Kind, DisplayName: component.DisplayName,
			Capability: component.Capability, Lifecycle: component.Lifecycle, Status: honest.EffectiveStatus,
			LastTrustworthyUpdateAt: component.LastTrustworthyUpdateAt, Coverage: honest.CoverageMicros,
		})
	}
	return publicComponents, honestComponents
}

func publicStatusMetadata(components []model.StatusComponent, now int64) statusPublicMetadata {
	metadata := statusPublicMetadata{GeneratedAt: now}
	var coverageTotal int64
	var coverageCount int64
	for _, component := range components {
		if component.Lifecycle == model.StatusLifecycleRetired {
			continue
		}
		if component.LastTrustworthyUpdateAt > metadata.LastTrustworthyUpdateAt {
			metadata.LastTrustworthyUpdateAt = component.LastTrustworthyUpdateAt
		}
		coverageTotal += component.CoverageMicros
		coverageCount++
	}
	if coverageCount > 0 {
		metadata.Coverage = coverageTotal / coverageCount
	}
	return metadata
}

func currentPublicStatusMetadata() (statusPublicMetadata, error) {
	now := time.Now().Unix()
	generatedAt := statusGeneratedAt(now)
	components, err := service.QueryStatusComponents(service.StatusComponentFilter{})
	if err != nil {
		return statusPublicMetadata{}, err
	}
	_, honest := publicStatusComponents(components, now)
	return publicStatusMetadata(honest, generatedAt), nil
}

func statusComponentFilter(c *gin.Context) (service.StatusComponentFilter, error) {
	filter := service.StatusComponentFilter{
		Kind: strings.TrimSpace(c.Query("kind")), Query: strings.TrimSpace(c.Query("query")),
		Capability: strings.TrimSpace(c.Query("capability")), Status: strings.TrimSpace(c.Query("status")),
	}
	if len(filter.Query) > statusPublicMaxFilterLength || len(filter.Capability) > statusPublicMaxFilterLength {
		return service.StatusComponentFilter{}, errors.New("status filter is too long")
	}
	if filter.Kind != "" && filter.Kind != model.StatusComponentKindRouter && filter.Kind != model.StatusComponentKindModel {
		return service.StatusComponentFilter{}, errors.New("invalid component kind")
	}
	if filter.Status != "" {
		switch filter.Status {
		case model.StatusOperational, model.StatusDegraded, model.StatusOutage, model.StatusUnknown, model.StatusMaintenance:
		default:
			return service.StatusComponentFilter{}, errors.New("invalid component status")
		}
	}
	return filter, nil
}

func statusHistoryRange(value string, now int64) (int64, string, bool) {
	switch value {
	case "24h":
		return now - 24*60*60, model.StatusGranularityHour, true
	case "7d":
		return now - 7*24*60*60, model.StatusGranularityHour, true
	case "30d":
		return now - 30*24*60*60, model.StatusGranularityDay, true
	case "90d":
		return now - 90*24*60*60, model.StatusGranularityDay, true
	default:
		return 0, "", false
	}
}

func statusPublicSuccess(c *gin.Context, data any) {
	payload, err := common.Marshal(data)
	if err != nil {
		statusPublicError(c, http.StatusInternalServerError, errors.New("status response unavailable"))
		return
	}
	digest := sha256.Sum256(payload)
	etag := fmt.Sprintf(`"%x"`, digest)
	c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", statusPublicCacheSeconds))
	c.Header("ETag", etag)
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	common.ApiSuccess(c, data)
}

func statusPublicError(c *gin.Context, status int, err error) {
	c.Status(status)
	c.Writer.WriteHeaderNow()
	common.ApiError(c, err)
}

func statusPublicQueryError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		statusPublicError(c, http.StatusNotFound, errors.New("status object not found"))
		return
	}
	statusPublicError(c, http.StatusServiceUnavailable, errors.New("status data unavailable"))
}

func decodeStatusPublicBody(c *gin.Context, target any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, statusSubscriptionMaxBodySize)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	return common.Unmarshal(body, target)
}

func statusBodyTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}

func statusGenericSubscriptionResponse(c *gin.Context, status int) {
	c.Status(status)
	c.Writer.WriteHeaderNow()
	common.ApiSuccess(c, service.StatusSubscriptionResponse{Message: service.StatusSubscriptionGenericMessage})
}

func boundedStatusToken(token string) string {
	token = strings.TrimSpace(token)
	if len(token) > 512 {
		return ""
	}
	return token
}

func statusGeneratedAt(now int64) int64 {
	return now - now%statusPublicCacheSeconds
}
