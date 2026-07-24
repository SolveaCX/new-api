package service

import (
	"encoding/json"
	"math"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func supplierReportJSONFields(t *testing.T, value any) map[string]json.RawMessage {
	t.Helper()
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	fields := make(map[string]json.RawMessage)
	require.NoError(t, common.Unmarshal(encoded, &fields))
	return fields
}

func TestSupplierReportMicroUsdResponsesUseExactDecimalStrings(t *testing.T) {
	maxDecimal := strconv.FormatInt(math.MaxInt64, 10)
	exactMax := `"` + maxDecimal + `"`

	money := supplierReportJSONFields(t, SupplierReportMoney{KnownCount: 7, MicroUsd: math.MaxInt64})
	require.Equal(t, "7", string(money["known_count"]))
	require.Equal(t, exactMax, string(money["micro_usd"]))

	metrics := supplierReportJSONFields(t, SupplierReportMetrics{
		RequestCount:                     11,
		GrossMarginEligibleCount:         13,
		GrossMarginEligibleSalesMicroUsd: math.MaxInt64,
		OfficialList:                     SupplierReportMoney{KnownCount: 17, MicroUsd: math.MaxInt64},
	})
	require.Equal(t, "11", string(metrics["request_count"]))
	require.Equal(t, "13", string(metrics["gross_margin_eligible_count"]))
	require.Equal(t, exactMax, string(metrics["gross_margin_eligible_sales_micro_usd"]))
	var officialList map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(metrics["official_list"], &officialList))
	require.Equal(t, "17", string(officialList["known_count"]))
	require.Equal(t, exactMax, string(officialList["micro_usd"]))

	overview := supplierReportJSONFields(t, SupplierReportOverview{
		TotalInventoryMicroUsd:       math.MaxInt64,
		OfficialListConsumedMicroUsd: math.MaxInt64,
		RemainingInventoryMicroUsd:   math.MaxInt64,
	})
	require.Equal(t, exactMax, string(overview["total_inventory_micro_usd"]))
	require.Equal(t, exactMax, string(overview["official_list_consumed_micro_usd"]))
	require.Equal(t, exactMax, string(overview["remaining_inventory_micro_usd"]))

	contract := supplierReportJSONFields(t, SupplierReportContractRow{
		TotalInventoryMicroUsd:       math.MaxInt64,
		OfficialListConsumedMicroUsd: math.MaxInt64,
		RemainingInventoryMicroUsd:   math.MaxInt64,
		RpmLimit:                     19,
	})
	require.Equal(t, exactMax, string(contract["total_inventory_micro_usd"]))
	require.Equal(t, exactMax, string(contract["official_list_consumed_micro_usd"]))
	require.Equal(t, exactMax, string(contract["remaining_inventory_micro_usd"]))
	require.Equal(t, "19", string(contract["rpm_limit"]))

	adjustment := supplierReportJSONFields(t, SupplierReportInventoryAdjustment{
		DeltaMicroUsd: math.MaxInt64,
		CreatedAt:     23,
	})
	require.Equal(t, exactMax, string(adjustment["delta_micro_usd"]))
	require.Equal(t, "23", string(adjustment["created_at"]))
}
