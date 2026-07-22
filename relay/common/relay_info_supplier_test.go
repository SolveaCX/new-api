package common

import (
	"net/http/httptest"
	"testing"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRelayInfoInitChannelMetaCopiesAndClearsSupplierSnapshots(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)

	cost := types.SupplierCostSnapshot{
		SupplierId:               1,
		SupplierName:             "supplier",
		ContractId:               2,
		ContractName:             "contract",
		RateVersionId:            3,
		ProcurementMultiplierPpm: 650_000,
	}
	scope := types.SupplierStatisticsScopeSnapshot{
		Scope:           types.SupplierStatisticsScopeInternal,
		ExclusionRuleId: 4,
	}
	rootcommon.SetContextKey(c, constant.ContextKeySupplierCostSnapshot, cost)
	rootcommon.SetContextKey(c, constant.ContextKeySupplierStatsScope, scope)

	info := &RelayInfo{}
	info.InitChannelMeta(c)
	require.Equal(t, cost, info.SupplierCostSnapshot)
	require.Equal(t, scope, info.SupplierStatisticsScopeSnapshot)

	rootcommon.SetContextKey(c, constant.ContextKeySupplierCostSnapshot, types.SupplierCostSnapshot{})
	rootcommon.SetContextKey(c, constant.ContextKeySupplierStatsScope, types.BusinessSupplierStatisticsScopeSnapshot())
	info.InitChannelMeta(c)
	require.Equal(t, types.SupplierCostSnapshot{}, info.SupplierCostSnapshot)
	require.Equal(t, types.BusinessSupplierStatisticsScopeSnapshot(), info.SupplierStatisticsScopeSnapshot)
}

func TestRelayInfoInitChannelMetaDefaultsMissingSupplierSnapshots(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)

	info := &RelayInfo{
		SupplierCostSnapshot: types.SupplierCostSnapshot{ContractId: 99},
		SupplierStatisticsScopeSnapshot: types.SupplierStatisticsScopeSnapshot{
			Scope:           types.SupplierStatisticsScopeInternal,
			ExclusionRuleId: 100,
		},
	}
	info.InitChannelMeta(c)
	require.Equal(t, types.SupplierCostSnapshot{}, info.SupplierCostSnapshot)
	require.Equal(t, types.SupplierStatisticsScopeBusiness, info.SupplierStatisticsScopeSnapshot.Scope)
	require.Zero(t, info.SupplierStatisticsScopeSnapshot.ExclusionRuleId)
}
