package controller

import (
	"errors"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const supplierAccountingIdempotencyKeyMaxBytes = 128

func GetSupplierAccountingStatus(c *gin.Context) {
	status, err := service.GetSupplierAccountingControlStatus()
	if err != nil {
		supplierAccountingControlError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}

func GetSupplierAccountingReadiness(c *gin.Context) {
	if err := service.CheckSupplierAccountingReadiness(); err != nil {
		supplierAccountingHTTPError(c, http.StatusServiceUnavailable, "supplier_accounting_not_ready", i18n.MsgSupplierAccountingNotReady)
		return
	}
	status, err := service.GetSupplierAccountingControlStatus()
	if err != nil {
		supplierAccountingHTTPError(c, http.StatusServiceUnavailable, "supplier_accounting_not_ready", i18n.MsgSupplierAccountingNotReady)
		return
	}
	common.ApiSuccess(c, gin.H{"ready": true, "status": status})
}

func PrepareSupplierAccounting(c *gin.Context) {
	handleSupplierAccountingMutation(c, func(request dto.SupplierAccountingPrepareRequest, actorID int, key string) (*model.SupplierAccountingControlResult, error) {
		return service.PrepareSupplierAccounting(actorID, key, request)
	})
}

func ArmSupplierAccounting(c *gin.Context) {
	handleSupplierAccountingMutation(c, func(request dto.SupplierAccountingArmRequest, actorID int, key string) (*model.SupplierAccountingControlResult, error) {
		return service.ArmSupplierAccounting(actorID, key, request)
	})
}

func ActivateSupplierAccounting(c *gin.Context) {
	handleSupplierAccountingMutation(c, func(request dto.SupplierAccountingCommandRequest, actorID int, key string) (*model.SupplierAccountingControlResult, error) {
		return service.ActivateSupplierAccounting(actorID, key, request)
	})
}

func DisableSupplierAccountingBeforeCutover(c *gin.Context) {
	handleSupplierAccountingMutation(c, func(request dto.SupplierAccountingCommandRequest, actorID int, key string) (*model.SupplierAccountingControlResult, error) {
		return service.DisableSupplierAccountingBeforeCutover(actorID, key, request)
	})
}

func DegradeSupplierAccounting(c *gin.Context) {
	handleSupplierAccountingMutation(c, func(request dto.SupplierAccountingDegradeRequest, actorID int, key string) (*model.SupplierAccountingControlResult, error) {
		return service.DegradeSupplierAccounting(actorID, key, request)
	})
}

func ResolveSupplierAccountingGap(c *gin.Context) {
	handleSupplierAccountingMutation(c, func(request dto.SupplierAccountingResolveGapRequest, actorID int, key string) (*model.SupplierAccountingControlResult, error) {
		return service.ResolveSupplierAccountingGap(actorID, key, request)
	})
}

func ReactivateSupplierAccounting(c *gin.Context) {
	handleSupplierAccountingMutation(c, func(request dto.SupplierAccountingCommandRequest, actorID int, key string) (*model.SupplierAccountingControlResult, error) {
		return service.ReactivateSupplierAccounting(actorID, key, request)
	})
}

func AdoptLegacySupplierAccounting(c *gin.Context) {
	handleSupplierAccountingMutation(c, func(request dto.SupplierAccountingLegacyAdoptionRequest, actorID int, key string) (*model.SupplierAccountingControlResult, error) {
		return service.AdoptLegacySupplierAccounting(actorID, key, request)
	})
}

func ToggleSupplierAccountingMutationGate(c *gin.Context) {
	handleSupplierAccountingMutation(c, func(request dto.SupplierAccountingMutationGateRequest, actorID int, key string) (*model.SupplierAccountingControlResult, error) {
		if request.Enabled == nil {
			return nil, model.ErrSupplierAccountingCommandInvalid
		}
		return service.ToggleSupplierAccountingMutationGate(actorID, key, request)
	})
}

func handleSupplierAccountingMutation[T any](c *gin.Context, execute func(T, int, string) (*model.SupplierAccountingControlResult, error)) {
	var request T
	if err := c.ShouldBindJSON(&request); err != nil {
		supplierAccountingHTTPError(c, http.StatusBadRequest, "invalid_request", i18n.MsgSupplierAccountingInvalidRequest)
		return
	}
	command, ok := supplierAccountingCommandRequest(any(request))
	if !ok || command.ExpectedStateVersion == nil || strings.TrimSpace(command.Reason) == "" {
		supplierAccountingHTTPError(c, http.StatusBadRequest, "invalid_request", i18n.MsgSupplierAccountingCommandFieldsRequired)
		return
	}
	key, ok := supplierAccountingIdempotencyKey(c)
	if !ok {
		return
	}
	result, err := execute(request, c.GetInt("id"), key)
	if err != nil {
		supplierAccountingControlError(c, err)
		return
	}
	if result.Replayed {
		c.Header("Idempotent-Replayed", "true")
	}
	common.ApiSuccess(c, result)
}

func supplierAccountingCommandRequest(request any) (dto.SupplierAccountingCommandRequest, bool) {
	switch value := request.(type) {
	case dto.SupplierAccountingCommandRequest:
		return value, true
	case dto.SupplierAccountingPrepareRequest:
		return value.SupplierAccountingCommandRequest, true
	case dto.SupplierAccountingArmRequest:
		return value.SupplierAccountingCommandRequest, true
	case dto.SupplierAccountingDegradeRequest:
		return value.SupplierAccountingCommandRequest, true
	case dto.SupplierAccountingResolveGapRequest:
		return value.SupplierAccountingCommandRequest, true
	case dto.SupplierAccountingMutationGateRequest:
		return value.SupplierAccountingCommandRequest, true
	case dto.SupplierAccountingLegacyAdoptionRequest:
		return value.SupplierAccountingCommandRequest, true
	default:
		return dto.SupplierAccountingCommandRequest{}, false
	}
}

func supplierAccountingIdempotencyKey(c *gin.Context) (string, bool) {
	raw := c.GetHeader("Idempotency-Key")
	key := strings.TrimSpace(raw)
	if raw != key || key == "" || len(key) > supplierAccountingIdempotencyKeyMaxBytes || !utf8.ValidString(key) || strings.ContainsFunc(key, unicode.IsControl) {
		supplierAccountingHTTPError(c, http.StatusBadRequest, "idempotency_key_required", i18n.MsgSupplierAccountingIdempotencyKeyRequired)
		return "", false
	}
	return key, true
}

func supplierAccountingControlError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrSupplierAdminIdempotencyConflict), errors.Is(err, model.ErrSupplierCoverageGapIdempotencyConflict):
		supplierAccountingHTTPError(c, http.StatusConflict, "idempotency_conflict", i18n.MsgSupplierAccountingIdempotencyConflict)
	case errors.Is(err, model.ErrSupplierAccountingOptionConflict), errors.Is(err, model.ErrSupplierCoverageGapCASConflict):
		supplierAccountingHTTPError(c, http.StatusConflict, "version_conflict", i18n.MsgSupplierAccountingVersionConflict)
	case errors.Is(err, model.ErrSupplierAccountingTransition), errors.Is(err, model.ErrSupplierAccountingMutationTransition):
		supplierAccountingHTTPError(c, http.StatusConflict, "invalid_transition", i18n.MsgSupplierAccountingInvalidTransition)
	case errors.Is(err, model.ErrSupplierAccountingCoverageUnresolved):
		supplierAccountingHTTPError(c, http.StatusConflict, "coverage_unresolved", i18n.MsgSupplierAccountingCoverageUnresolved)
	case errors.Is(err, model.ErrSupplierCoverageGapNotFound):
		supplierAccountingHTTPError(c, http.StatusNotFound, "not_found", i18n.MsgSupplierAccountingCoverageGapNotFound)
	case errors.Is(err, model.ErrSupplierAccountingCommandInvalid), errors.Is(err, model.ErrSupplierCoverageGapInvalid),
		errors.Is(err, model.ErrSupplierAccountingLegacyUnavailable), errors.Is(err, model.ErrSupplierAdminIdempotencyKeyRequired):
		supplierAccountingHTTPError(c, http.StatusBadRequest, "invalid_request", i18n.MsgSupplierAccountingInvalidRequest)
	case errors.Is(err, model.ErrSupplierAccountingOptionMalformed):
		supplierAccountingHTTPError(c, http.StatusServiceUnavailable, "state_malformed", i18n.MsgSupplierAccountingStateMalformed)
	default:
		supplierAccountingHTTPError(c, http.StatusServiceUnavailable, "control_plane_unavailable", i18n.MsgSupplierAccountingControlPlaneUnavailable)
	}
}

func supplierAccountingHTTPError(c *gin.Context, status int, code, messageKey string) {
	c.JSON(status, gin.H{"success": false, "message": i18n.T(c, messageKey), "code": code})
}
