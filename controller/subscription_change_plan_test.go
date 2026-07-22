package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChangeSubscriptionPlanRejectsInvalidRequestID(t *testing.T) {
	paymentSetting := operation_setting.GetPaymentSetting()
	originalConfirmed := paymentSetting.ComplianceConfirmed
	originalVersion := paymentSetting.ComplianceTermsVersion
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = originalConfirmed
		paymentSetting.ComplianceTermsVersion = originalVersion
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 901)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/subscription/self/change-plan",
		strings.NewReader(`{"plan_id":1,"payment_mode":"balance_one_period","request_id":"bad id"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	ChangeSubscriptionPlan(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":false`)
	require.Contains(t, recorder.Body.String(), "request_id")
}
