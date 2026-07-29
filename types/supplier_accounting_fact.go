package types

// SupplierAccountingCoverageScope identifies which request families are
// represented by an authoritative accounting fact stream.
type SupplierAccountingCoverageScope string

const (
	SupplierAccountingAttemptIDKeyV1 = "supplier_accounting_attempt_id"

	// SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1 covers
	// synchronous relay attempts with a frozen supplier binding. Unbound,
	// async-task, and Midjourney flows are intentionally outside this V1 scope.
	SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1 SupplierAccountingCoverageScope = "bound_supplier_synchronous_relay_v1"
)
