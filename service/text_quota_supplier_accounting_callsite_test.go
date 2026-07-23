package service

import (
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestSupplierTextHasPositiveFinalUsage(t *testing.T) {
	committed := types.BillingSettlementResult{FinanciallyCommitted: true}
	committedWithSalesQuota := types.BillingSettlementResult{FinanciallyCommitted: true, FinalSalesQuota: 1}
	notCommitted := types.BillingSettlementResult{}
	allEvidence := textQuotaSummary{
		TotalTokens:                1,
		WebSearchCallCount:         1,
		ClaudeWebSearchCallCount:   1,
		FileSearchCallCount:        1,
		ImageGenerationCallApplied: true,
	}

	require.False(t, supplierTextHasPositiveFinalUsage(allEvidence, notCommitted), "uncommitted usage must remain ineligible despite explicit evidence")
	require.False(t, supplierTextHasPositiveFinalUsage(textQuotaSummary{}, committed), "fixed/price mode alone must not establish positive usage")
	require.True(t, supplierTextHasPositiveFinalUsage(textQuotaSummary{TotalTokens: 1}, committed))
	require.True(t, supplierTextHasPositiveFinalUsage(textQuotaSummary{}, committedWithSalesQuota))
	require.True(t, supplierTextHasPositiveFinalUsage(textQuotaSummary{WebSearchCallCount: 1}, committed))
	require.True(t, supplierTextHasPositiveFinalUsage(textQuotaSummary{ClaudeWebSearchCallCount: 1}, committed))
	require.True(t, supplierTextHasPositiveFinalUsage(textQuotaSummary{FileSearchCallCount: 1}, committed))
	require.True(t, supplierTextHasPositiveFinalUsage(textQuotaSummary{ImageGenerationCallApplied: true}, committed))
}
