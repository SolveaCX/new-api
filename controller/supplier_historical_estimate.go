package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func CreateSupplierHistoricalEstimateImport(c *gin.Context) {
	idempotencyKey, ok := supplyChainIdempotencyKey(c)
	if !ok {
		return
	}
	var command service.SupplierHistoricalImportCommand
	if c.ShouldBindJSON(&command) != nil {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return
	}
	item, err := service.CreateSupplierHistoricalEstimate(c.Request.Context(), model.DB, command, c.GetInt("id"), idempotencyKey)
	if err != nil {
		supplierHistoricalEstimateError(c, err)
		return
	}
	view, err := service.BuildSupplierHistoricalImportView(item)
	if err != nil {
		supplierHistoricalEstimateError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func ListSupplierHistoricalEstimateImports(c *gin.Context) {
	page := common.GetPageQuery(c)
	items, total, err := model.ListSupplierHistoricalImports(c.Request.Context(), model.DB, page.GetStartIdx(), page.GetPageSize())
	if err != nil {
		supplierHistoricalEstimateError(c, err)
		return
	}
	views := make([]service.SupplierHistoricalImportView, 0, len(items))
	for _, item := range items {
		view, viewErr := service.BuildSupplierHistoricalImportView(item)
		if viewErr != nil {
			supplierHistoricalEstimateError(c, viewErr)
			return
		}
		views = append(views, view)
	}
	page.SetTotal(int(total))
	page.SetItems(views)
	common.ApiSuccess(c, page)
}

func GetSupplierHistoricalEstimateImport(c *gin.Context) {
	id, ok := supplierHistoricalEstimateID(c)
	if !ok {
		return
	}
	item, err := model.GetSupplierHistoricalImport(c.Request.Context(), model.DB, id)
	if err != nil {
		supplierHistoricalEstimateError(c, err)
		return
	}
	view, err := service.BuildSupplierHistoricalImportView(item)
	if err != nil {
		supplierHistoricalEstimateError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func PublishSupplierHistoricalEstimateImport(c *gin.Context) {
	id, ok := supplierHistoricalEstimateID(c)
	if !ok {
		return
	}
	item, err := service.PublishSupplierHistoricalEstimate(c.Request.Context(), model.DB, id, c.GetInt("id"))
	if err != nil {
		supplierHistoricalEstimateError(c, err)
		return
	}
	view, err := service.BuildSupplierHistoricalImportView(item)
	if err != nil {
		supplierHistoricalEstimateError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func ListSupplierHistoricalEstimateSummaries(c *gin.Context) {
	id, ok := supplierHistoricalEstimateID(c)
	if !ok {
		return
	}
	item, err := model.GetSupplierHistoricalImport(c.Request.Context(), model.DB, id)
	if err != nil {
		supplierHistoricalEstimateError(c, err)
		return
	}
	if item.Status != model.SupplierHistoricalImportStatusCompleted {
		supplierHistoricalEstimateError(c, service.ErrSupplierHistoricalImportNotReady)
		return
	}
	startDate := strings.TrimSpace(c.DefaultQuery("start_date", item.StartDate))
	endDate := strings.TrimSpace(c.DefaultQuery("end_date", item.EndDate))
	limit := 200
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed <= 0 || parsed > 500 {
			supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
			return
		}
		limit = parsed
	}
	cursor := model.SupplierHistoricalSeriesCursor{
		Date: strings.TrimSpace(c.Query("after_date")), StatisticsScope: strings.TrimSpace(c.Query("after_scope")),
	}
	if raw := strings.TrimSpace(c.Query("after_supplier_id")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 0 {
			supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
			return
		}
		cursor.SupplierId = parsed
	}
	if startDate < item.StartDate || endDate > item.EndDate || startDate >= endDate ||
		(cursor.Date != "" && (cursor.Date < startDate || cursor.Date >= endDate)) {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return
	}
	items, hasMore, err := service.ListCompletedSupplierHistoricalSeries(c.Request.Context(), model.DB, id, startDate, endDate, cursor, limit)
	if err != nil {
		supplierHistoricalEstimateError(c, err)
		return
	}
	var next *model.SupplierHistoricalSeriesCursor
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		next = &model.SupplierHistoricalSeriesCursor{Date: last.Date, StatisticsScope: last.StatisticsScope, SupplierId: last.SupplierId}
	}
	common.ApiSuccess(c, gin.H{"items": items, "limit": limit, "has_more": hasMore, "next_cursor": next})
}

func supplierHistoricalEstimateID(c *gin.Context) (int64, bool) {
	raw := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
		return 0, false
	}
	return id, true
}

func supplierHistoricalEstimateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrSupplierHistoricalCommandInvalid), errors.Is(err, service.ErrSupplierHistoricalMappingInvalid),
		errors.Is(err, model.ErrSupplierHistoricalImportInvalid):
		supplyChainError(c, http.StatusBadRequest, i18n.MsgSupplyChainInvalidInput)
	case errors.Is(err, model.ErrSupplierHistoricalImportIdempotencyConflict), errors.Is(err, model.ErrSupplierHistoricalImportOverlap),
		errors.Is(err, model.ErrSupplierHistoricalImportBusy), errors.Is(err, model.ErrSupplierHistoricalImportFenceLost),
		errors.Is(err, model.ErrSupplierHistoricalReplacementInvalid), errors.Is(err, model.ErrSupplierHistoricalPublicationConflict),
		errors.Is(err, model.ErrSupplierHistoricalPublicationNeedsReestimate),
		errors.Is(err, service.ErrSupplierHistoricalImportNotReady):
		supplyChainError(c, http.StatusConflict, i18n.MsgSupplyChainConflict)
	case errors.Is(err, gorm.ErrRecordNotFound):
		supplyChainError(c, http.StatusNotFound, i18n.MsgSupplyChainNotFound)
	default:
		supplyChainError(c, http.StatusInternalServerError, i18n.MsgSupplyChainInternalError)
	}
}
