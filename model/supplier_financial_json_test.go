package model

import (
	"encoding/json"
	"math"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func supplierFinancialJSONFields(t *testing.T, value any) map[string]json.RawMessage {
	t.Helper()
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	fields := make(map[string]json.RawMessage)
	require.NoError(t, common.Unmarshal(encoded, &fields))
	return fields
}

func TestSupplierAdminMicroUsdResponsesUseExactDecimalStrings(t *testing.T) {
	maxDecimal := strconv.FormatInt(math.MaxInt64, 10)

	supplier := supplierFinancialJSONFields(t, SupplierAdminRow{
		InventoryTotalMicroUsd: math.MaxInt64,
		ContractCount:          7,
		CreatedAt:              11,
	})
	require.Equal(t, `"`+maxDecimal+`"`, string(supplier["inventory_total_micro_usd"]))
	require.Equal(t, "7", string(supplier["contract_count"]))
	require.Equal(t, "11", string(supplier["created_at"]))

	ppm := int64(650_000)
	effectiveAt := int64(13)
	contract := supplierFinancialJSONFields(t, SupplierContractAdminRow{
		InventoryTotalMicroUsd:          math.MaxInt64,
		CurrentProcurementMultiplierPpm: &ppm,
		CurrentRateEffectiveAt:          &effectiveAt,
		RpmLimit:                        17,
	})
	require.Equal(t, `"`+maxDecimal+`"`, string(contract["inventory_total_micro_usd"]))
	require.Equal(t, "650000", string(contract["current_procurement_multiplier_ppm"]))
	require.Equal(t, "13", string(contract["current_rate_effective_at"]))
	require.Equal(t, "17", string(contract["rpm_limit"]))

	adjustment := supplierFinancialJSONFields(t, SupplierInventoryAdjustment{
		DeltaMicroUsd: math.MaxInt64,
		CreatedAt:     19,
	})
	require.Equal(t, `"`+maxDecimal+`"`, string(adjustment["delta_micro_usd"]))
	require.Equal(t, "19", string(adjustment["created_at"]))
}

func TestSupplierInventoryAdjustmentRequestDeltaRemainsNumeric(t *testing.T) {
	payload := []byte(`{"delta_micro_usd":9223372036854775807,"type":"replenishment","reason":"max"}`)
	var request dto.SupplierInventoryAdjustmentCreateRequest
	require.NoError(t, common.Unmarshal(payload, &request))
	require.NotNil(t, request.DeltaMicroUsd)
	require.Equal(t, int64(math.MaxInt64), *request.DeltaMicroUsd)

	encoded, err := common.Marshal(request)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(encoded, &fields))
	require.Equal(t, "number", common.GetJsonType(fields["delta_micro_usd"]))
	require.Equal(t, "9223372036854775807", string(fields["delta_micro_usd"]))
}
