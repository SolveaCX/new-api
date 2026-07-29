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

const supplierReportMaxFilterIDs = 200

var newSupplyChainReportService = service.DefaultSupplierReportService

func GetSupplyChainReportOverview(c *gin.Context) {
	query, ok := supplyChainReportQuery(c)
	if !ok {
		return
	}
	result, err := newSupplyChainReportService().GetOverview(c.Request.Context(), query)
	supplyChainReportResult(c, result, err)
}

func GetSupplyChainReportTrend(c *gin.Context) {
	query, ok := supplyChainReportQuery(c)
	if !ok {
		return
	}
	result, err := newSupplyChainReportService().GetTrend(c.Request.Context(), query)
	supplyChainReportResult(c, result, err)
}

func ListSupplyChainReportContracts(c *gin.Context) {
	query, ok := supplyChainReportQuery(c)
	if !ok {
		return
	}
	page, ok := supplyChainReportPage(c)
	if !ok {
		return
	}
	result, err := newSupplyChainReportService().ListContracts(c.Request.Context(), query, page)
	supplyChainReportResult(c, result, err)
}

func GetSupplyChainReportContract(c *gin.Context) {
	contractID, ok := supplyChainPositivePathInt(c, "id")
	if !ok {
		return
	}
	query, ok := supplyChainReportQuery(c)
	if !ok {
		return
	}
	page, ok := supplyChainReportPage(c)
	if !ok {
		return
	}
	result, err := newSupplyChainReportService().GetContractDetail(c.Request.Context(), contractID, query, page)
	supplyChainReportResult(c, result, err)
}

func ListSupplyChainReportChannels(c *gin.Context) {
	query, ok := supplyChainReportQuery(c)
	if !ok {
		return
	}
	page, ok := supplyChainReportPage(c)
	if !ok {
		return
	}
	result, err := newSupplyChainReportService().ListChannels(c.Request.Context(), query, page)
	supplyChainReportResult(c, result, err)
}

func ListSupplyChainReportBreakdown(c *gin.Context) {
	query, ok := supplyChainReportQuery(c)
	if !ok {
		return
	}
	page, ok := supplyChainReportPage(c)
	if !ok {
		return
	}
	result, err := newSupplyChainReportService().ListBreakdown(c.Request.Context(), query, page)
	supplyChainReportResult(c, result, err)
}

func supplyChainReportQuery(c *gin.Context) (service.SupplierReportQuery, bool) {
	supplierIDs, ok := supplyChainCSVPositiveInts(c, "supplier_ids")
	if !ok {
		return service.SupplierReportQuery{}, false
	}
	contractIDs, ok := supplyChainCSVPositiveInts(c, "contract_ids")
	if !ok {
		return service.SupplierReportQuery{}, false
	}
	channelIDs, ok := supplyChainCSVPositiveInts(c, "channel_ids")
	if !ok {
		return service.SupplierReportQuery{}, false
	}
	query := service.SupplierReportQuery{
		Month: strings.TrimSpace(c.Query("month")), StartDate: strings.TrimSpace(c.Query("start_date")), EndDate: strings.TrimSpace(c.Query("end_date")),
		SupplierIds: supplierIDs, ContractIds: contractIDs, ChannelIds: channelIDs,
	}
	if _, err := service.ParseSupplierReportRange(query.Month, query.StartDate, query.EndDate); err != nil {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidReportRange)
		return service.SupplierReportQuery{}, false
	}
	return query, true
}

func supplyChainReportPage(c *gin.Context) (model.SupplierReportPage, bool) {
	page := model.SupplierReportPage{}
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 || value > model.SupplierReportMaxPageSize {
			supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
			return page, false
		}
		page.Limit = value
	}
	if raw := strings.TrimSpace(c.Query("offset")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
			return page, false
		}
		page.Offset = value
	}
	return page.Normalize(), true
}

func supplyChainCSVPositiveInts(c *gin.Context, name string) ([]int, bool) {
	parts := supplyChainCSVStrings(c.Query(name))
	if len(parts) > supplierReportMaxFilterIDs {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidReportFilter)
		return nil, false
	}
	result := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))
	for _, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value <= 0 {
			supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidReportFilter)
			return nil, false
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, true
}

func supplyChainCSVStrings(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func supplyChainReportResult(c *gin.Context, result any, err error) {
	if err == nil {
		common.ApiSuccess(c, result)
		return
	}
	switch {
	case errors.Is(err, service.ErrInvalidSupplierReportRange):
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidReportRange)
	case errors.Is(err, model.ErrInvalidSupplierReportFilter):
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidReportFilter)
	case errors.Is(err, service.ErrSupplierReportContractNotFound):
		supplyChainError(c, http.StatusNotFound, i18n.MsgSupplyChainNotFound)
	case errors.Is(err, service.ErrSupplierReportOverflow):
		supplyChainError(c, http.StatusUnprocessableEntity, i18n.MsgSupplyChainReportUnavailable)
	default:
		logger.LogError(c.Request.Context(), fmt.Sprintf("supplier report request failed: %v", err))
		supplyChainError(c, http.StatusInternalServerError, i18n.MsgSupplyChainInternalError)
	}
}
