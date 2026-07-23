package controller

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const supplierReportMaxFilterIDs = 200

const supplierDailyReportRerunReasonMaxBytes = 512

var newSupplyChainReportService = service.DefaultSupplierReportService

type supplierDailyReportRerunFunc func(context.Context, *gorm.DB, *gorm.DB, int, string, string, dto.SupplierDailyReportRerunRequest, time.Time) (dto.SupplierBatchStatusResponse, error)

var rerunSupplierDailyReport = service.RerunSupplierDailyReport

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

func GetSupplyChainReportFreshness(c *gin.Context) {
	result, err := newSupplyChainReportService().GetFreshness(c.Request.Context())
	supplyChainReportResult(c, result, err)
}

func GetSupplyChainDailyReports(c *gin.Context) {
	query := service.SupplierReportQuery{
		Month:     strings.TrimSpace(c.Query("month")),
		StartDate: strings.TrimSpace(c.Query("start_date")),
		EndDate:   strings.TrimSpace(c.Query("end_date")),
	}
	if _, err := service.ParseSupplierReportRange(query.Month, query.StartDate, query.EndDate); err != nil {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidReportRange)
		return
	}
	result, err := newSupplyChainReportService().GetDaily(c.Request.Context(), query)
	supplyChainReportResult(c, result, err)
}

func RerunSupplyChainDailyReport(c *gin.Context) {
	rerunSupplyChainDailyReport(c, rerunSupplierDailyReport, model.DB, model.LOG_DB, time.Now())
}

func rerunSupplyChainDailyReport(c *gin.Context, rerun supplierDailyReportRerunFunc, mainDB, logDB *gorm.DB, now time.Time) {
	batchDate := c.Param("date")
	location, err := time.LoadLocation(service.SupplierDailyBatchTimezone)
	if err != nil {
		supplierBatchHTTPError(c, http.StatusServiceUnavailable, "config_unavailable")
		return
	}
	parsedDate, err := time.ParseInLocation("2006-01-02", batchDate, location)
	if err != nil || parsedDate.Format("2006-01-02") != batchDate {
		supplierBatchHTTPError(c, http.StatusBadRequest, "invalid_request")
		return
	}
	idempotencyKey, ok := supplierBatchIdempotencyKey(c)
	if !ok {
		return
	}
	request := dto.SupplierDailyReportRerunRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		supplierBatchHTTPError(c, http.StatusBadRequest, "invalid_request")
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if request.Reason == "" || len(request.Reason) > supplierDailyReportRerunReasonMaxBytes || request.ExpectedPublishedFenceToken <= 0 {
		supplierBatchHTTPError(c, http.StatusBadRequest, "invalid_request")
		return
	}
	actorID := c.GetInt("id")
	if actorID <= 0 {
		supplierBatchHTTPError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	if mainDB == nil || logDB == nil || rerun == nil {
		supplierBatchHTTPError(c, http.StatusServiceUnavailable, "config_unavailable")
		return
	}
	result, err := rerun(c.Request.Context(), mainDB, logDB, actorID, batchDate, idempotencyKey, request, now)
	switch {
	case errors.Is(err, service.ErrSupplierDailyReportNotFound):
		supplierBatchHTTPError(c, http.StatusNotFound, "not_found")
		return
	case errors.Is(err, service.ErrSupplierDailyReportInvalid):
		supplierBatchHTTPError(c, http.StatusBadRequest, "invalid_request")
		return
	case errors.Is(err, service.ErrSupplierDailyReportNotEligible):
		supplierBatchHTTPError(c, http.StatusConflict, "not_eligible")
		return
	case errors.Is(err, service.ErrSupplierDailyReportVersionConflict):
		supplierBatchHTTPError(c, http.StatusConflict, "version_conflict")
		return
	}
	supplierDailyBatchResult(c, result, err)
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
		supplyChainError(c, http.StatusInternalServerError, i18n.MsgSupplyChainInternalError)
	}
}
