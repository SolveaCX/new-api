package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func catalogMigrationMappingForControllerTest() service.CatalogMigrationMappingExpectation {
	return service.CatalogMigrationMappingExpectation{SourcePlanID: 1, TargetPlanID: 5}
}

func catalogMigrationControllerContext(method string, path string, body string, rootUserID int) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", rootUserID)
	return ctx, recorder
}

func catalogMigrationControllerResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestAdminPreviewSubscriptionCatalogMigrationRejectsUnknownFieldsBeforeServiceCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := previewSubscriptionCatalogMigrationForAdmin
	t.Cleanup(func() { previewSubscriptionCatalogMigrationForAdmin = original })
	calls := 0
	previewSubscriptionCatalogMigrationForAdmin = func(context.Context, AdminSubscriptionCatalogMigrationPreviewRequest) (any, error) {
		calls++
		return map[string]any{"unexpected": true}, nil
	}
	ctx, recorder := catalogMigrationControllerContext(http.MethodPost, "/api/subscription/admin/catalog-migrations/preview", `{"request_id":"request-1","mappings":[{"source_plan_id":1,"target_plan_id":5}],"contract_ids":[7],"allow_live":true}`, 1)

	AdminPreviewSubscriptionCatalogMigration(ctx)

	require.Zero(t, calls)
	require.Equal(t, false, catalogMigrationControllerResponse(t, recorder)["success"])
}

func TestAdminPreviewSubscriptionCatalogMigrationDelegatesValidatedRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := previewSubscriptionCatalogMigrationForAdmin
	t.Cleanup(func() { previewSubscriptionCatalogMigrationForAdmin = original })
	var received AdminSubscriptionCatalogMigrationPreviewRequest
	previewSubscriptionCatalogMigrationForAdmin = func(_ context.Context, request AdminSubscriptionCatalogMigrationPreviewRequest) (any, error) {
		received = request
		return map[string]any{"cohort_digest": strings.Repeat("a", 64)}, nil
	}
	ctx, recorder := catalogMigrationControllerContext(http.MethodPost, "/api/subscription/admin/catalog-migrations/preview", `{"request_id":" request-1 ","mappings":[{"source_plan_id":1,"target_plan_id":5}],"contract_ids":[7]}`, 1)

	AdminPreviewSubscriptionCatalogMigration(ctx)

	require.Equal(t, "request-1", received.RequestID)
	require.Equal(t, []int64{7}, received.ContractIDs)
	require.Equal(t, true, catalogMigrationControllerResponse(t, recorder)["success"])
}

func TestAdminApplySubscriptionCatalogMigrationRejectsMalformedDigestBeforeServiceCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := applySubscriptionCatalogMigrationForAdmin
	t.Cleanup(func() { applySubscriptionCatalogMigrationForAdmin = original })
	calls := 0
	applySubscriptionCatalogMigrationForAdmin = func(context.Context, int, AdminSubscriptionCatalogMigrationApplyRequest) (any, error) {
		calls++
		return map[string]any{"unexpected": true}, nil
	}
	ctx, recorder := catalogMigrationControllerContext(http.MethodPost, "/api/subscription/admin/catalog-migrations/apply", `{"request_id":"request-1","cohort_digest":"short","mappings":[{"source_plan_id":1,"target_plan_id":5}],"contract_ids":[7]}`, 41)

	AdminApplySubscriptionCatalogMigration(ctx)

	require.Zero(t, calls)
	require.Equal(t, false, catalogMigrationControllerResponse(t, recorder)["success"])
}

func TestAdminApplySubscriptionCatalogMigrationPassesRootIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := applySubscriptionCatalogMigrationForAdmin
	t.Cleanup(func() { applySubscriptionCatalogMigrationForAdmin = original })
	receivedRootID := 0
	var received AdminSubscriptionCatalogMigrationApplyRequest
	applySubscriptionCatalogMigrationForAdmin = func(_ context.Context, rootID int, request AdminSubscriptionCatalogMigrationApplyRequest) (any, error) {
		receivedRootID = rootID
		received = request
		return map[string]any{"batch_id": "catmig_test"}, nil
	}
	digest := strings.Repeat("A", 64)
	ctx, recorder := catalogMigrationControllerContext(http.MethodPost, "/api/subscription/admin/catalog-migrations/apply", `{"request_id":"request-1","cohort_digest":"`+digest+`","mappings":[{"source_plan_id":1,"target_plan_id":5}],"contract_ids":[7]}`, 42)

	AdminApplySubscriptionCatalogMigration(ctx)

	require.Equal(t, 42, receivedRootID)
	require.Equal(t, strings.ToLower(digest), received.CohortDigest)
	require.Equal(t, true, catalogMigrationControllerResponse(t, recorder)["success"])
}

func TestAdminGetSubscriptionCatalogMigrationDelegatesBatchID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := getSubscriptionCatalogMigrationForAdmin
	t.Cleanup(func() { getSubscriptionCatalogMigrationForAdmin = original })
	receivedBatchID := ""
	getSubscriptionCatalogMigrationForAdmin = func(_ context.Context, batchID string) (any, error) {
		receivedBatchID = batchID
		return map[string]any{"batch_id": batchID}, nil
	}
	ctx, recorder := catalogMigrationControllerContext(http.MethodGet, "/api/subscription/admin/catalog-migrations/catmig_test", "", 43)
	ctx.Params = gin.Params{{Key: "id", Value: "catmig_test"}}

	AdminGetSubscriptionCatalogMigration(ctx)

	require.Equal(t, "catmig_test", receivedBatchID)
	require.Equal(t, true, catalogMigrationControllerResponse(t, recorder)["success"])
}

func TestAdminCancelSubscriptionCatalogMigrationPassesRootIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := cancelSubscriptionCatalogMigrationForAdmin
	t.Cleanup(func() { cancelSubscriptionCatalogMigrationForAdmin = original })
	receivedRootID := 0
	receivedBatchID := ""
	cancelSubscriptionCatalogMigrationForAdmin = func(_ context.Context, rootID int, batchID string) (any, error) {
		receivedRootID = rootID
		receivedBatchID = batchID
		return map[string]any{"status": "cancelled"}, nil
	}
	ctx, recorder := catalogMigrationControllerContext(http.MethodPost, "/api/subscription/admin/catalog-migrations/catmig_test/cancel", "", 44)
	ctx.Params = gin.Params{{Key: "id", Value: "catmig_test"}}

	AdminCancelSubscriptionCatalogMigration(ctx)

	require.Equal(t, 44, receivedRootID)
	require.Equal(t, "catmig_test", receivedBatchID)
	require.Equal(t, true, catalogMigrationControllerResponse(t, recorder)["success"])
}
