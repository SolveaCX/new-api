package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	supplierBatchRequestIDMaxBytes     = 128
	supplierBatchExecutionHardDeadline = 45 * time.Minute
)

type supplierDailyBatchCatchUpByRequestFunc func(context.Context, *gorm.DB, *gorm.DB, dto.SupplierBatchSchedulerPrincipal, dto.SupplierBatchCatchUpRequest, time.Time) (dto.SupplierBatchStatusResponse, error)
type supplierDailyBatchStatusFunc func(context.Context, *gorm.DB, dto.SupplierBatchSchedulerPrincipal, string, time.Time) (dto.SupplierBatchStatusResponse, error)

var catchUpSupplierDailyBatchesByRequest = service.CatchUpSupplierDailyBatchesByRequest
var getSupplierDailyBatchRequestStatus supplierDailyBatchStatusFunc = supplierDailyBatchRequestStatusFromService

func supplierDailyBatchRequestStatusFromService(ctx context.Context, mainDB *gorm.DB, principal dto.SupplierBatchSchedulerPrincipal, requestID string, now time.Time) (dto.SupplierBatchStatusResponse, error) {
	return service.GetSupplierDailyBatchRequestStatus(ctx, mainDB, principal, requestID, now)
}

func TriggerSupplierDailyBatchCatchUp(c *gin.Context) {
	triggerSupplierDailyBatchCatchUp(c, catchUpSupplierDailyBatchesByRequest, model.DB, model.LOG_DB, time.Now())
}

func GetSupplierDailyBatchStatus(c *gin.Context) {
	getSupplierDailyBatchStatus(c, getSupplierDailyBatchRequestStatus, model.DB, time.Now())
}

func triggerSupplierDailyBatchCatchUp(c *gin.Context, catchUp supplierDailyBatchCatchUpByRequestFunc, mainDB, logDB *gorm.DB, now time.Time) {
	triggerSupplierDailyBatchCatchUpWithin(c, catchUp, mainDB, logDB, now, supplierBatchExecutionHardDeadline)
}

func triggerSupplierDailyBatchCatchUpWithin(c *gin.Context, catchUp supplierDailyBatchCatchUpByRequestFunc, mainDB, logDB *gorm.DB, now time.Time, hardDeadline time.Duration) {
	principal, ok := middleware.SupplierBatchPrincipalFromContext(c)
	if !ok {
		supplierBatchHTTPError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	requestID, ok := supplierBatchIdempotencyKey(c)
	if !ok {
		return
	}
	if mainDB == nil || logDB == nil || catchUp == nil {
		supplierBatchHTTPError(c, http.StatusServiceUnavailable, "config_unavailable")
		return
	}
	executionContext, cancel := context.WithTimeout(c.Request.Context(), hardDeadline)
	defer cancel()
	result, err := catchUp(executionContext, mainDB, logDB, principal, dto.SupplierBatchCatchUpRequest{RequestID: requestID}, now)
	supplierDailyBatchResult(c, result, err)
}

func getSupplierDailyBatchStatus(c *gin.Context, getStatus supplierDailyBatchStatusFunc, mainDB *gorm.DB, now time.Time) {
	principal, ok := middleware.SupplierBatchPrincipalFromContext(c)
	if !ok {
		supplierBatchHTTPError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	requestID, ok := supplierBatchStatusRequestID(c)
	if !ok {
		return
	}
	if mainDB == nil || getStatus == nil {
		supplierBatchHTTPError(c, http.StatusServiceUnavailable, "config_unavailable")
		return
	}
	result, err := getStatus(c.Request.Context(), mainDB, principal, requestID, now)
	supplierDailyBatchResult(c, result, err)
}

func supplierDailyBatchResult(c *gin.Context, result dto.SupplierBatchStatusResponse, err error) {
	if err == nil {
		if validationErr := result.Validate(); validationErr != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("supplier batch service returned an invalid status payload: %v", validationErr))
			supplierBatchHTTPError(c, http.StatusInternalServerError, "internal_error")
			return
		}
		c.JSON(http.StatusOK, result)
		return
	}
	switch {
	case errors.Is(err, service.ErrSupplierBatchRequestNotFound):
		supplierBatchHTTPError(c, http.StatusNotFound, "not_found")
	case errors.Is(err, service.ErrSupplierBatchBusy):
		supplierBatchHTTPError(c, http.StatusConflict, "busy")
	case errors.Is(err, service.ErrSupplierBatchIdempotencyConflict):
		supplierBatchHTTPError(c, http.StatusConflict, "idempotency_conflict")
	case errors.Is(err, service.ErrSupplierBatchConfigUnavailable):
		supplierBatchHTTPError(c, http.StatusServiceUnavailable, "config_unavailable")
	default:
		logger.LogError(c.Request.Context(), fmt.Sprintf("supplier batch request failed: %v", err))
		supplierBatchHTTPError(c, http.StatusInternalServerError, "internal_error")
	}
}

func supplierBatchIdempotencyKey(c *gin.Context) (string, bool) {
	values := c.Request.Header.Values("Idempotency-Key")
	if len(values) != 1 || !validSupplierBatchRequestID(values[0]) {
		supplierBatchHTTPError(c, http.StatusBadRequest, "idempotency_key_required")
		return "", false
	}
	return values[0], true
}

func supplierBatchStatusRequestID(c *gin.Context) (string, bool) {
	values := c.Request.URL.Query()["request_id"]
	if len(values) != 1 || !validSupplierBatchRequestID(values[0]) {
		supplierBatchHTTPError(c, http.StatusBadRequest, "request_id_required")
		return "", false
	}
	return values[0], true
}

func validSupplierBatchRequestID(value string) bool {
	return value != "" && len(value) <= supplierBatchRequestIDMaxBytes && utf8.ValidString(value) && value == strings.TrimSpace(value) && !strings.ContainsFunc(value, unicode.IsControl)
}

func supplierBatchHTTPError(c *gin.Context, status int, code string) {
	messageKey := i18n.MsgSupplyChainInternalError
	switch code {
	case "idempotency_key_required", "request_id_required", "invalid_request":
		messageKey = i18n.MsgSupplyChainInvalidInput
	case "not_found":
		messageKey = i18n.MsgSupplyChainNotFound
	case "busy", "idempotency_conflict", "not_eligible", "not_rerunnable", "version_conflict":
		messageKey = i18n.MsgSupplyChainConflict
	}
	c.JSON(status, gin.H{"success": false, "message": common.TranslateMessage(c, messageKey), "code": code})
}
