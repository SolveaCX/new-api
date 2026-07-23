package controller

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

func TestPrepareOptionUpdateRejectsSupplierAccountingReservedKeys(t *testing.T) {
	require.NoError(t, backendi18n.Init())
	gin.SetMode(gin.TestMode)
	translations := []struct {
		language string
		message  string
	}{
		{language: backendi18n.LangEn, message: "This setting cannot be modified through the general settings API"},
		{language: backendi18n.LangZhCN, message: "该配置项不允许通过通用设置接口修改"},
	}
	keys := []string{
		model.SupplierAccountingActivationOptionKey,
		model.SupplierAccountingMutationOptionKey,
		model.SupplierAccountingCoverageStartOptionKey,
	}
	for _, translation := range translations {
		for _, key := range keys {
			t.Run(translation.language+"/"+key, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(recorder)
				ctx.Request = httptest.NewRequest(http.MethodPost, "/", nil)
				ctx.Request.Header.Set("Accept-Language", translation.language)
				request := OptionUpdateRequest{Key: key, Value: `{}`}
				require.False(t, prepareOptionUpdate(ctx, &request))

				var response struct {
					Success bool   `json:"success"`
					Message string `json:"message"`
				}
				require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
				require.False(t, response.Success)
				require.Equal(t, translation.message, response.Message)
			})
		}
	}
}
