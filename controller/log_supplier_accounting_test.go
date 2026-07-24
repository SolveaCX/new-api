package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestGetAllLogsRedactsSupplierAccountingForAdminButKeepsRootView(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId:    100,
		CreatedAt: 1000,
		Type:      model.LogTypeConsume,
		ModelName: "gpt-4o",
		ChannelId: 0,
		Other: common.MapToJsonStr(map[string]interface{}{
			"supplier_accounting_v1": map[string]interface{}{
				"official_list_micro_usd":    "100000000",
				"procurement_cost_micro_usd": "65000000",
				"sales_micro_usd":            "70000000",
				"gross_profit_micro_usd":     "5000000",
			},
			"matched_tier": "standard",
		}),
	}).Error)

	for _, testCase := range []struct {
		name         string
		role         int
		wantSupplier bool
	}{
		{name: "admin", role: common.RoleAdminUser, wantSupplier: false},
		{name: "root", role: common.RoleRootUser, wantSupplier: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log?type=2&p=1&page_size=20", nil, 1)
			ctx.Set("role", testCase.role)
			GetAllLogs(ctx)

			response := decodeAPIResponse(t, recorder)
			require.True(t, response.Success, response.Message)
			var page struct {
				Items []*model.Log `json:"items"`
			}
			require.NoError(t, common.Unmarshal(response.Data, &page))
			require.Len(t, page.Items, 1)
			other, err := common.StrToMap(page.Items[0].Other)
			require.NoError(t, err)
			_, hasSupplier := other["supplier_accounting_v1"]
			require.Equal(t, testCase.wantSupplier, hasSupplier)
			require.Equal(t, "standard", other["matched_tier"])
		})
	}
}
