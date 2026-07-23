package service

import (
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestSupplierAudioHasPositiveFinalUsage(t *testing.T) {
	committed := types.BillingSettlementResult{FinanciallyCommitted: true}
	committedWithSalesQuota := types.BillingSettlementResult{FinanciallyCommitted: true, FinalSalesQuota: 1}
	notCommitted := types.BillingSettlementResult{}

	require.False(t, supplierAudioHasPositiveFinalUsage(1, notCommitted), "uncommitted usage must remain ineligible despite token evidence")
	require.False(t, supplierAudioHasPositiveFinalUsage(0, committed), "fixed/price mode alone must not establish positive usage")
	require.True(t, supplierAudioHasPositiveFinalUsage(1, committed))
	require.True(t, supplierAudioHasPositiveFinalUsage(0, committedWithSalesQuota))
}
