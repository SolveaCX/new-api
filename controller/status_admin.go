package controller

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const statusAdminMaxBodySize = int64(64 * 1024)

type statusAdminIncidentRecord struct {
	Incident     model.StatusIncident         `json:"incident"`
	Updates      []model.StatusIncidentUpdate `json:"updates"`
	ComponentIDs []int64                      `json:"component_ids"`
}

type statusAdminSetting struct {
	Key        string `json:"key"`
	Value      string `json:"value,omitempty"`
	Sensitive  bool   `json:"sensitive"`
	Configured bool   `json:"configured"`
	Version    int64  `json:"version"`
	UpdatedBy  int    `json:"updated_by"`
	UpdatedAt  int64  `json:"updated_at"`
}

func ListAdminStatusIncidents(c *gin.Context) {
	adminStatusIncidentList(c, "")
}

func GetAdminStatusIncident(c *gin.Context) {
	id, err := statusAdminID(c.Param("id"))
	if err != nil {
		statusAdminError(c, http.StatusBadRequest, err)
		return
	}
	record, err := service.GetAdminStatusIncidentRecord(id)
	if err != nil {
		statusAdminServiceError(c, err)
		return
	}
	common.ApiSuccess(c, adminStatusIncidentRecord(record))
}

func PublishAdminStatusIncident(c *gin.Context) {
	id, err := statusAdminID(c.Param("id"))
	if err != nil {
		statusAdminError(c, http.StatusBadRequest, err)
		return
	}
	var request struct {
		ExpectedVersion int64                               `json:"expected_version"`
		State           string                              `json:"state"`
		Body            string                              `json:"body"`
		EventID         string                              `json:"event_id"`
		Reason          string                              `json:"reason"`
		Destinations    []service.StatusDeliveryDestination `json:"destinations"`
	}
	if err := decodeStatusAdminBody(c, &request); err != nil {
		statusAdminDecodeError(c, err)
		return
	}
	result, err := service.PublishStatusIncidentUpdate(service.StatusIncidentPublishInput{
		IncidentID: id, ExpectedVersion: request.ExpectedVersion, State: request.State,
		Body: request.Body, EventID: request.EventID, Destinations: request.Destinations,
		Actor: statusAdminActor(c), Reason: request.Reason, Now: time.Now().Unix(),
	})
	if err != nil {
		statusAdminServiceError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func ListAdminStatusMaintenance(c *gin.Context) {
	adminStatusIncidentList(c, model.StatusIncidentKindMaintenance)
}

func CreateAdminStatusMaintenance(c *gin.Context) {
	var request struct {
		Title            string  `json:"title"`
		Body             string  `json:"body"`
		Impact           string  `json:"impact"`
		IdempotencyKey   string  `json:"idempotency_key"`
		ComponentIDs     []int64 `json:"component_ids"`
		ScheduledStartAt int64   `json:"scheduled_start_at"`
		ScheduledEndAt   int64   `json:"scheduled_end_at"`
		Reason           string  `json:"reason"`
	}
	if err := decodeStatusAdminBody(c, &request); err != nil {
		statusAdminDecodeError(c, err)
		return
	}
	incident, err := service.CreateStatusMaintenanceDraft(service.StatusMaintenanceDraftInput{
		Title: request.Title, Body: request.Body, Impact: request.Impact,
		IdempotencyKey: request.IdempotencyKey, ComponentIDs: request.ComponentIDs,
		ScheduledStartAt: request.ScheduledStartAt, ScheduledEndAt: request.ScheduledEndAt,
		Actor: statusAdminActor(c), Reason: request.Reason, Now: time.Now().Unix(),
	})
	if err != nil {
		statusAdminServiceError(c, err)
		return
	}
	common.ApiSuccess(c, incident)
}

func ReconcileAdminStatusMaintenance(c *gin.Context) {
	id, err := statusAdminID(c.Param("id"))
	if err != nil {
		statusAdminError(c, http.StatusBadRequest, err)
		return
	}
	var request struct {
		ExpectedVersion int64 `json:"expected_version"`
	}
	if err := decodeStatusAdminBody(c, &request); err != nil {
		statusAdminDecodeError(c, err)
		return
	}
	incident, err := service.ReconcileStatusMaintenance(service.StatusMaintenanceTransitionInput{
		IncidentID: id, ExpectedVersion: request.ExpectedVersion, Now: time.Now().Unix(),
	})
	if err != nil {
		statusAdminServiceError(c, err)
		return
	}
	common.ApiSuccess(c, incident)
}

func CreateAdminStatusOverride(c *gin.Context) {
	createAdminStatusOverride(c, false)
}

func CreateAdminStatusForceGreen(c *gin.Context) {
	createAdminStatusOverride(c, true)
}

func ListAdminStatusSettings(c *gin.Context) {
	settings, err := service.ListStatusSettings(statusAdminLimit(c))
	if err != nil {
		statusAdminServiceError(c, err)
		return
	}
	response := make([]statusAdminSetting, 0, len(settings))
	for _, setting := range settings {
		value := setting.Value
		if setting.Sensitive {
			value = ""
		}
		response = append(response, statusAdminSetting{
			Key: setting.Key, Value: value, Sensitive: setting.Sensitive,
			Configured: setting.Value != "", Version: setting.Version,
			UpdatedBy: setting.UpdatedBy, UpdatedAt: setting.UpdatedAt,
		})
	}
	common.ApiSuccess(c, response)
}

func UpdateAdminStatusSetting(c *gin.Context) {
	var request struct {
		Value           string `json:"value"`
		ExpectedVersion int64  `json:"expected_version"`
	}
	if err := decodeStatusAdminBody(c, &request); err != nil {
		statusAdminDecodeError(c, err)
		return
	}
	setting, err := service.UpdateStatusSetting(
		statusAdminActor(c), c.Param("key"), request.Value, request.ExpectedVersion, time.Now().Unix(),
	)
	if err != nil {
		statusAdminServiceError(c, err)
		return
	}
	common.ApiSuccess(c, statusAdminSetting{
		Key: setting.Key, Value: setting.Value, Sensitive: setting.Sensitive, Configured: setting.Value != "",
		Version: setting.Version, UpdatedBy: setting.UpdatedBy, UpdatedAt: setting.UpdatedAt,
	})
}

func ConfigureAdminStatusDiscord(c *gin.Context) {
	var request struct {
		Endpoint string `json:"endpoint"`
	}
	if err := decodeStatusAdminBody(c, &request); err != nil {
		statusAdminDecodeError(c, err)
		return
	}
	keyring, err := service.LoadStatusSecretKeyringFromEnvironment()
	if err != nil {
		statusAdminServiceError(c, err)
		return
	}
	setting, err := service.ConfigureStatusDiscordEndpoint(statusAdminActor(c), request.Endpoint, keyring, time.Now().Unix())
	if err != nil {
		statusAdminServiceError(c, err)
		return
	}
	common.ApiSuccess(c, statusAdminSetting{
		Key: setting.Key, Sensitive: true, Configured: true, Version: setting.Version,
		UpdatedBy: setting.UpdatedBy, UpdatedAt: setting.UpdatedAt,
	})
}

func TestAdminStatusDiscord(c *gin.Context) {
	keyring, err := service.LoadStatusSecretKeyringFromEnvironment()
	if err != nil {
		statusAdminServiceError(c, err)
		return
	}
	result, err := service.SendStatusDiscordTest(
		c.Request.Context(), statusAdminActor(c), keyring, service.NewStatusSafeWebhookClient(), time.Now().Unix(),
	)
	if err != nil {
		statusAdminServiceError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func ListAdminStatusSubscribers(c *gin.Context) {
	subscribers, err := service.ListStatusSubscribers(statusAdminLimit(c))
	if err != nil {
		statusAdminServiceError(c, err)
		return
	}
	common.ApiSuccess(c, subscribers)
}

func ListAdminStatusDeliveries(c *gin.Context) {
	deliveries, err := service.ListStatusDeliveries(statusAdminLimit(c))
	if err != nil {
		statusAdminServiceError(c, err)
		return
	}
	common.ApiSuccess(c, deliveries)
}

func ListAdminStatusAudit(c *gin.Context) {
	events, err := service.ListStatusAuditEvents(statusAdminLimit(c))
	if err != nil {
		statusAdminServiceError(c, err)
		return
	}
	common.ApiSuccess(c, events)
}

func adminStatusIncidentList(c *gin.Context, kind string) {
	records, err := service.ListStatusIncidentRecords(kind, false, statusAdminLimit(c))
	if err != nil {
		statusAdminServiceError(c, err)
		return
	}
	response := make([]statusAdminIncidentRecord, 0, len(records))
	for _, record := range records {
		response = append(response, adminStatusIncidentRecord(record))
	}
	common.ApiSuccess(c, response)
}

func adminStatusIncidentRecord(record service.StatusIncidentRecord) statusAdminIncidentRecord {
	return statusAdminIncidentRecord{
		Incident: record.Incident, Updates: record.Updates, ComponentIDs: record.ComponentIDs,
	}
}

func createAdminStatusOverride(c *gin.Context, forceGreen bool) {
	var request struct {
		ComponentID     int64  `json:"component_id"`
		ExpectedVersion int64  `json:"expected_version"`
		Status          string `json:"status"`
		Reason          string `json:"reason"`
		ExpiresAt       int64  `json:"expires_at"`
	}
	if err := decodeStatusAdminBody(c, &request); err != nil {
		statusAdminDecodeError(c, err)
		return
	}
	if forceGreen {
		request.Status = model.StatusOperational
	}
	component, err := service.ApplyStatusOverride(service.StatusOverrideInput{
		ComponentID: request.ComponentID, ExpectedVersion: request.ExpectedVersion,
		Status: request.Status, Reason: request.Reason, ExpiresAt: request.ExpiresAt,
		Actor: statusAdminActor(c), Now: time.Now().Unix(),
	})
	if err != nil {
		statusAdminServiceError(c, err)
		return
	}
	common.ApiSuccess(c, component)
}

func statusAdminActor(c *gin.Context) service.StatusMutationActor {
	return service.StatusMutationActor{
		ID: c.GetInt("id"), Role: c.GetInt("role"), SecureVerified: c.GetBool("secure_verified"),
	}
}

func statusAdminID(value string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return 0, service.ErrStatusInvalidMutation
	}
	return id, nil
}

func statusAdminLimit(c *gin.Context) int {
	limit, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "100")))
	if err != nil || limit <= 0 || limit > 200 {
		return 100
	}
	return limit
}

func decodeStatusAdminBody(c *gin.Context, target any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, statusAdminMaxBodySize)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	return common.Unmarshal(body, target)
}

func statusAdminDecodeError(c *gin.Context, err error) {
	if statusBodyTooLarge(err) {
		statusAdminError(c, http.StatusRequestEntityTooLarge, errors.New("status request body is too large"))
		return
	}
	statusAdminError(c, http.StatusBadRequest, service.ErrStatusInvalidMutation)
}

func statusAdminServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrStatusVersionConflict),
		errors.Is(err, service.ErrStatusMaintenanceOverlap),
		errors.Is(err, service.ErrStatusMaintenanceRequiresTransition),
		errors.Is(err, service.ErrStatusMaintenanceNotPublished):
		statusAdminError(c, http.StatusConflict, err)
	case errors.Is(err, service.ErrStatusInvalidMutation):
		statusAdminError(c, http.StatusBadRequest, err)
	case errors.Is(err, service.ErrStatusAdminRequired),
		errors.Is(err, service.ErrStatusRootRequired),
		errors.Is(err, service.ErrStatusSecureVerificationRequired):
		statusAdminError(c, http.StatusForbidden, err)
	case errors.Is(err, gorm.ErrRecordNotFound):
		statusAdminError(c, http.StatusNotFound, errors.New("status object not found"))
	default:
		statusAdminError(c, http.StatusInternalServerError, err)
	}
}

func statusAdminError(c *gin.Context, status int, err error) {
	c.Status(status)
	c.Writer.WriteHeaderNow()
	common.ApiError(c, err)
}
