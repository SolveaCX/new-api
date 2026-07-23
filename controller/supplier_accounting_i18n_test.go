package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	backendi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type supplierAccountingErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

type supplierAccountingControlErrorCase struct {
	name    string
	err     error
	status  int
	code    string
	english string
	chinese string
}

func TestSupplierAccountingControlErrorsAreLocalizedWithoutChangingMachineContract(t *testing.T) {
	require.NoError(t, backendi18n.Init())
	gin.SetMode(gin.TestMode)
	tests := []supplierAccountingControlErrorCase{
		{name: "idempotency conflict", err: model.ErrSupplierAdminIdempotencyConflict, status: http.StatusConflict, code: "idempotency_conflict", english: "idempotency key payload conflict", chinese: "幂等键对应的请求负载冲突"},
		{name: "version conflict", err: model.ErrSupplierAccountingOptionConflict, status: http.StatusConflict, code: "version_conflict", english: "supplier accounting version conflict", chinese: "供应商核算版本冲突"},
		{name: "invalid transition", err: model.ErrSupplierAccountingTransition, status: http.StatusConflict, code: "invalid_transition", english: "supplier accounting transition is not allowed", chinese: "不允许执行该供应商核算状态转换"},
		{name: "coverage unresolved", err: model.ErrSupplierAccountingCoverageUnresolved, status: http.StatusConflict, code: "coverage_unresolved", english: "supplier accounting coverage gaps remain unresolved", chinese: "供应商核算覆盖缺口仍未解决"},
		{name: "gap not found", err: model.ErrSupplierCoverageGapNotFound, status: http.StatusNotFound, code: "not_found", english: "supplier accounting coverage gap was not found", chinese: "未找到供应商核算覆盖缺口"},
		{name: "invalid request", err: model.ErrSupplierAccountingCommandInvalid, status: http.StatusBadRequest, code: "invalid_request", english: "invalid supplier accounting request", chinese: "无效的供应商核算请求"},
		{name: "state malformed", err: model.ErrSupplierAccountingOptionMalformed, status: http.StatusServiceUnavailable, code: "state_malformed", english: "supplier accounting state is malformed", chinese: "供应商核算状态格式无效"},
		{name: "control plane unavailable", err: errors.New("database failure"), status: http.StatusServiceUnavailable, code: "control_plane_unavailable", english: "supplier accounting control plane is unavailable", chinese: "供应商核算控制面不可用"},
	}
	languages := []struct {
		name string
		code string
		pick func(supplierAccountingControlErrorCase) string
	}{
		{name: "English", code: backendi18n.LangEn, pick: func(test supplierAccountingControlErrorCase) string { return test.english }},
		{name: "Simplified Chinese", code: backendi18n.LangZhCN, pick: func(test supplierAccountingControlErrorCase) string { return test.chinese }},
	}

	for _, testCase := range tests {
		for _, language := range languages {
			t.Run(testCase.name+"/"+language.name, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(recorder)
				ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
				ctx.Request.Header.Set("Accept-Language", language.code)

				supplierAccountingControlError(ctx, testCase.err)

				var response supplierAccountingErrorResponse
				require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
				require.Equal(t, testCase.status, recorder.Code)
				require.False(t, response.Success)
				require.Equal(t, testCase.code, response.Code)
				require.Equal(t, language.pick(testCase), response.Message)
			})
		}
	}
}

func TestSupplierAccountingRequestValidationAndReadinessErrorsAreLocalized(t *testing.T) {
	require.NoError(t, backendi18n.Init())
	setupSupplierAccountingControllerTestDB(t)
	engine := supplierAccountingControllerEngine()

	request := func(method, path, body, idempotencyKey string) supplierAccountingErrorResponse {
		t.Helper()
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Language", backendi18n.LangZhCN)
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}
		engine.ServeHTTP(recorder, req)
		var response supplierAccountingErrorResponse
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		return response
	}

	invalidJSON := request(http.MethodPost, "/prepare", `{`, "invalid-json")
	require.Equal(t, "invalid_request", invalidJSON.Code)
	require.Equal(t, "无效的供应商核算请求", invalidJSON.Message)

	missingFields := request(http.MethodPost, "/prepare", `{}`, "missing-fields")
	require.Equal(t, "invalid_request", missingFields.Code)
	require.Equal(t, "expected_state_version 和 reason 为必填项", missingFields.Message)

	missingIdempotencyKey := request(http.MethodPost, "/prepare", `{"expected_state_version":0,"reason":"prepare","accepted_capability_versions":[1]}`, "")
	require.Equal(t, "idempotency_key_required", missingIdempotencyKey.Code)
	require.Equal(t, "必须提供有效的 Idempotency-Key", missingIdempotencyKey.Message)

	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", "not-a-timestamp")
	notReady := request(http.MethodGet, "/readiness", "", "")
	require.Equal(t, "supplier_accounting_not_ready", notReady.Code)
	require.Equal(t, "供应商核算尚未就绪", notReady.Message)
}
