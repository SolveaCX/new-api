package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
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
		Other:     supplierAccountingControllerTestOther(t),
	}).Error)

	for _, testCase := range []struct {
		name           string
		role           int
		wantRaw        bool
		wantProjection bool
	}{
		{name: "user", role: common.RoleCommonUser, wantRaw: false, wantProjection: false},
		{name: "admin", role: common.RoleAdminUser, wantRaw: false, wantProjection: false},
		{name: "root", role: common.RoleRootUser, wantRaw: true, wantProjection: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log?type=2&p=1&page_size=20", nil, 1)
			ctx.Set("role", testCase.role)
			GetAllLogs(ctx)

			response := decodeAPIResponse(t, recorder)
			require.True(t, response.Success, response.Message)
			if testCase.wantProjection {
				require.Contains(t, string(response.Data), `"official_list_micro_usd":"100000000"`)
			}
			var page struct {
				Items []*model.Log `json:"items"`
			}
			require.NoError(t, common.Unmarshal(response.Data, &page))
			require.Len(t, page.Items, 1)
			other, err := common.StrToMap(page.Items[0].Other)
			require.NoError(t, err)
			_, hasSupplier := other[types.SupplierAccountingEnvelopeKeyV1]
			require.Equal(t, testCase.wantRaw, hasSupplier)
			require.Equal(t, "standard", other["matched_tier"])
			if testCase.wantProjection {
				require.NotNil(t, page.Items[0].SupplierAccounting)
				require.Equal(t, 12, page.Items[0].SupplierAccounting.SupplierId)
				require.EqualValues(t, 100_000_000, *page.Items[0].SupplierAccounting.OfficialListMicroUsd)
				require.Equal(t, types.SupplierPricingModeRatio, page.Items[0].SupplierAccounting.PricingEvidence.Mode)
			} else {
				require.Nil(t, page.Items[0].SupplierAccounting)
			}
		})
	}
}

func TestGetAllLogsRootIgnoresMalformedSupplierAccountingProjection(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId: 100, CreatedAt: 1000, Type: model.LogTypeConsume, ModelName: "gpt-4o",
		Other: `{"supplier_accounting_v1":{"v":1,"d":"captured","s":"invalid"},"matched_tier":"standard"}`,
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log?type=2&p=1&page_size=20", nil, 1)
	ctx.Set("role", common.RoleRootUser)
	GetAllLogs(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var page struct {
		Items []*model.Log `json:"items"`
	}
	require.NoError(t, common.Unmarshal(response.Data, &page))
	require.Len(t, page.Items, 1)
	require.Nil(t, page.Items[0].SupplierAccounting)
	require.Contains(t, page.Items[0].Other, types.SupplierAccountingEnvelopeKeyV1)
}

func supplierAccountingControllerTestOther(t *testing.T) string {
	t.Helper()
	official := int64(100_000_000)
	procurement := int64(65_000_000)
	salesMultiplier := int64(700_000)
	sales := int64(70_000_000)
	grossProfit := int64(5_000_000)
	envelope := types.SupplierAccountingEnvelopeV1{
		EnvelopeSchemaVersion: types.SupplierAccountingEnvelopeSchemaVersionV1,
		Disposition:           types.SupplierAccountingDispositionCaptured,
		Captured: &types.SupplierAccountingLogSnapshotV1{
			BindingVersionId: 11, SupplierId: 12, ContractId: 13, RateVersionId: 14,
			ProcurementMultiplierPpm: 650_000, SalesMultiplierPpm: &salesMultiplier,
			OfficialListMicroUsd: &official, SalesMicroUsd: &sales, ProcurementCostMicroUsd: &procurement,
			GrossProfitMicroUsd: &grossProfit, StatisticsScope: string(types.SupplierStatisticsScopeBusiness),
			ExclusionDecision: "included", FinanciallyCommittedAt: 1_784_801_200,
			PricingProvenance: &types.SupplierPricingProvenanceV1{Ratio: &types.SupplierRatioPricingProvenanceV1{
				ModelRatioPpm: 2_500_000, GroupRatioPpm: salesMultiplier, ModelRatioVersion: 1, GroupRatioVersion: 1,
			}},
		},
	}
	payload, err := common.Marshal(map[string]interface{}{
		types.SupplierAccountingEnvelopeKeyV1: envelope,
		"matched_tier":                        "standard",
	})
	require.NoError(t, err)
	return string(payload)
}
