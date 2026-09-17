package controller

import (
	"context"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type AdminSubscriptionCatalogMigrationPreviewRequest struct {
	RequestID   string                                       `json:"request_id"`
	Mappings    []service.CatalogMigrationMappingExpectation `json:"mappings"`
	ContractIDs []int64                                      `json:"contract_ids"`
}

type AdminSubscriptionCatalogMigrationApplyRequest struct {
	RequestID    string                                       `json:"request_id"`
	CohortDigest string                                       `json:"cohort_digest"`
	Mappings     []service.CatalogMigrationMappingExpectation `json:"mappings"`
	ContractIDs  []int64                                      `json:"contract_ids"`
}

var previewSubscriptionCatalogMigrationForAdmin = func(ctx context.Context, request AdminSubscriptionCatalogMigrationPreviewRequest) (any, error) {
	return service.PreviewSubscriptionCatalogMigration(ctx, service.CatalogMigrationPreviewRequest{
		RequestID:   request.RequestID,
		Mappings:    request.Mappings,
		ContractIDs: request.ContractIDs,
	})
}

var applySubscriptionCatalogMigrationForAdmin = func(ctx context.Context, requestedBy int, request AdminSubscriptionCatalogMigrationApplyRequest) (any, error) {
	return service.ApplySubscriptionCatalogMigration(ctx, service.CatalogMigrationApplyCommand{
		PreviewRequest: service.CatalogMigrationPreviewRequest{
			RequestID:   request.RequestID,
			Mappings:    request.Mappings,
			ContractIDs: request.ContractIDs,
		},
		CohortDigest: request.CohortDigest,
		RequestedBy:  requestedBy,
	})
}

var getSubscriptionCatalogMigrationForAdmin = func(ctx context.Context, batchID string) (any, error) {
	return service.GetSubscriptionCatalogMigration(ctx, batchID)
}

var cancelSubscriptionCatalogMigrationForAdmin = func(ctx context.Context, requestedBy int, batchID string) (any, error) {
	return service.CancelSubscriptionCatalogMigration(ctx, service.CatalogMigrationCancelCommand{
		BatchID:     batchID,
		RequestedBy: requestedBy,
	})
}

func AdminPreviewSubscriptionCatalogMigration(c *gin.Context) {
	var request AdminSubscriptionCatalogMigrationPreviewRequest
	if !decodeAdminSubscriptionCatalogMigrationRequest(c, &request) {
		return
	}
	request.RequestID = strings.TrimSpace(request.RequestID)
	if request.RequestID == "" || len(request.Mappings) == 0 || len(request.ContractIDs) == 0 {
		common.ApiErrorMsg(c, "catalog migration preview request is invalid")
		return
	}
	result, err := previewSubscriptionCatalogMigrationForAdmin(c.Request.Context(), request)
	adminSubscriptionCatalogMigrationResult(c, result, err)
}

func AdminApplySubscriptionCatalogMigration(c *gin.Context) {
	var request AdminSubscriptionCatalogMigrationApplyRequest
	if !decodeAdminSubscriptionCatalogMigrationRequest(c, &request) {
		return
	}
	request.RequestID = strings.TrimSpace(request.RequestID)
	request.CohortDigest = strings.ToLower(strings.TrimSpace(request.CohortDigest))
	if request.RequestID == "" || len(request.CohortDigest) != 64 || len(request.Mappings) == 0 || len(request.ContractIDs) == 0 {
		common.ApiErrorMsg(c, "catalog migration apply request is invalid")
		return
	}
	result, err := applySubscriptionCatalogMigrationForAdmin(c.Request.Context(), c.GetInt("id"), request)
	adminSubscriptionCatalogMigrationResult(c, result, err)
}

func AdminGetSubscriptionCatalogMigration(c *gin.Context) {
	batchID := strings.TrimSpace(c.Param("id"))
	if batchID == "" {
		common.ApiErrorMsg(c, "catalog migration batch id is required")
		return
	}
	result, err := getSubscriptionCatalogMigrationForAdmin(c.Request.Context(), batchID)
	adminSubscriptionCatalogMigrationResult(c, result, err)
}

func AdminCancelSubscriptionCatalogMigration(c *gin.Context) {
	batchID := strings.TrimSpace(c.Param("id"))
	if batchID == "" {
		common.ApiErrorMsg(c, "catalog migration batch id is required")
		return
	}
	result, err := cancelSubscriptionCatalogMigrationForAdmin(c.Request.Context(), c.GetInt("id"), batchID)
	adminSubscriptionCatalogMigrationResult(c, result, err)
}

func decodeAdminSubscriptionCatalogMigrationRequest(c *gin.Context, target any) bool {
	if err := common.DecodeJsonDisallowUnknownFields(c.Request.Body, target); err != nil {
		common.ApiErrorMsg(c, "catalog migration request is invalid")
		return false
	}
	return true
}

func adminSubscriptionCatalogMigrationResult(c *gin.Context, result any, err error) {
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if result == nil {
		common.ApiError(c, errors.New("subscription catalog migration result is missing"))
		return
	}
	common.ApiSuccess(c, result)
}
