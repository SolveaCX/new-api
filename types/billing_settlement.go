package types

// BillingSettlementResult describes the synchronous financial outcome of a
// billing settlement attempt. FinanciallyCommittedAt is a Unix timestamp and
// is set only when FinanciallyCommitted is true.
type BillingSettlementResult struct {
	FinanciallyCommitted   bool
	FinanciallyCommittedAt int64
	FinalSalesQuota        int
	Err                    error
}
