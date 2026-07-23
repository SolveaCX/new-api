package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	backendi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSupplierMutationGateErrorsAreLocalizedWithoutChangingMachineContract(t *testing.T) {
	require.NoError(t, backendi18n.Init())
	db := setupSupplierMutationGateTestDB(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/mutation", SupplierMutationGate(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := func(language string) (int, supplierAccountingErrorResponse) {
		t.Helper()
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/mutation", nil)
		req.Header.Set("Accept-Language", language)
		engine.ServeHTTP(recorder, req)
		var response supplierAccountingErrorResponse
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		return recorder.Code, response
	}

	status, disabledEnglish := request(backendi18n.LangEn)
	require.Equal(t, http.StatusLocked, status)
	require.Equal(t, SupplierMutationGateDisabledCode, disabledEnglish.Code)
	require.Equal(t, "supplier mutations are disabled", disabledEnglish.Message)

	status, disabledChinese := request(backendi18n.LangZhCN)
	require.Equal(t, http.StatusLocked, status)
	require.Equal(t, SupplierMutationGateDisabledCode, disabledChinese.Code)
	require.Equal(t, "供应商变更已被禁用", disabledChinese.Message)

	require.NoError(t, db.Create(&model.Option{Key: model.SupplierAccountingMutationOptionKey, Value: `{}`}).Error)
	status, unavailableEnglish := request(backendi18n.LangEn)
	require.Equal(t, http.StatusServiceUnavailable, status)
	require.Equal(t, SupplierMutationGateUnavailableCode, unavailableEnglish.Code)
	require.Equal(t, "supplier mutation gate is unavailable", unavailableEnglish.Message)

	status, unavailableChinese := request(backendi18n.LangZhCN)
	require.Equal(t, http.StatusServiceUnavailable, status)
	require.Equal(t, SupplierMutationGateUnavailableCode, unavailableChinese.Code)
	require.Equal(t, "供应商变更控制不可用", unavailableChinese.Message)
}

type supplierAccountingErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    string `json:"code"`
}
