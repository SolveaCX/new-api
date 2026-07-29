package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const supplierAccountingPendingPageLimit = 200

type supplierAccountingFactResolveRequest struct {
	Action      string `json:"action"`
	SourceLogId int    `json:"source_log_id"`
	Reason      string `json:"reason"`
	Evidence    string `json:"evidence"`
}

func ListPendingSupplyChainAccountingFacts(c *gin.Context) {
	day := strings.TrimSpace(c.Query("date"))
	cursorID, err := strconv.ParseInt(strings.TrimSpace(c.DefaultQuery("cursor_id", "0")), 10, 64)
	if err != nil || cursorID < 0 {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return
	}
	limit, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "100")))
	if err != nil || limit <= 0 || limit > supplierAccountingPendingPageLimit {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return
	}
	facts, err := service.ListPendingSupplierAccountingFacts(c.Request.Context(), day, cursorID, limit+1)
	if err != nil {
		supplyChainAccountingFactError(c, err)
		return
	}
	hasMore := len(facts) > limit
	if hasMore {
		facts = facts[:limit]
	}
	nextCursorID := cursorID
	if len(facts) > 0 {
		nextCursorID = facts[len(facts)-1].Id
	}
	common.ApiSuccess(c, gin.H{"items": facts, "next_cursor_id": nextCursorID, "has_more": hasMore})
}

func ResolvePendingSupplyChainAccountingFact(c *gin.Context) {
	attemptID := strings.TrimSpace(c.Param("attempt_id"))
	var request supplierAccountingFactResolveRequest
	if c.ShouldBindJSON(&request) != nil || len(attemptID) > 36 || strings.TrimSpace(request.Reason) == "" ||
		len(strings.TrimSpace(request.Reason)) > 255 || strings.TrimSpace(request.Evidence) == "" || len(strings.TrimSpace(request.Evidence)) > 4000 {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return
	}
	fact, err := service.ResolvePendingSupplierAccountingFact(c.Request.Context(), service.SupplierAccountingFactResolveInput{
		AttemptId: attemptID, Action: request.Action, SourceLogId: request.SourceLogId,
		Actor:  fmt.Sprintf("root:%d:%s", c.GetInt("id"), c.GetString("username")),
		Reason: request.Reason, Evidence: request.Evidence,
	})
	if err != nil {
		supplyChainAccountingFactError(c, err)
		return
	}
	common.ApiSuccess(c, fact)
}

func supplyChainAccountingFactError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrSupplierAccountingFactOperationInvalid), errors.Is(err, model.ErrSupplierAccountingFactResolutionInvalid):
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
	case errors.Is(err, model.ErrSupplierAccountingFactNotFound), errors.Is(err, model.ErrLogNotFound):
		supplyChainError(c, http.StatusNotFound, i18n.MsgSupplyChainNotFound)
	case errors.Is(err, service.ErrSupplierAccountingFactSourceLogMismatch),
		errors.Is(err, service.ErrSupplierAccountingFactSourceLogInvalid), errors.Is(err, model.ErrSupplierAccountingFactTerminalConflict):
		supplyChainError(c, http.StatusConflict, i18n.MsgSupplyChainConflict)
	default:
		logger.LogError(c.Request.Context(), fmt.Sprintf("supplier accounting fact operation failed: %v", err))
		supplyChainError(c, http.StatusInternalServerError, i18n.MsgSupplyChainInternalError)
	}
}
