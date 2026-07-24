package dto

// SupplierCreateRequest intentionally exposes only mutable supplier fields.
// Status and identifiers are assigned by the server.
type SupplierCreateRequest struct {
	Name   string `json:"name"`
	Remark string `json:"remark"`
}

type SupplierUpdateRequest struct {
	Name            *string `json:"name,omitempty"`
	Remark          *string `json:"remark,omitempty"`
	ExpectedVersion *int64  `json:"expected_version"`
}

type SupplierInactivateRequest struct {
	ExpectedVersion *int64 `json:"expected_version"`
}

// SupplierContractCreateRequest does not expose status or
// current_rate_version_id. The latter advances only through the dedicated
// append-only rate-version command.
type SupplierContractCreateRequest struct {
	SupplierId     int    `json:"supplier_id"`
	Name           string `json:"name"`
	ContractNo     string `json:"contract_no"`
	Remark         string `json:"remark"`
	RpmLimit       int64  `json:"rpm_limit"`
	TpmLimit       int64  `json:"tpm_limit"`
	MaxConcurrency int    `json:"max_concurrency"`
}

// SupplierContractUpdateRequest deliberately omits supplier_id, status, and
// current_rate_version_id to prevent mass assignment of invariant fields.
type SupplierContractUpdateRequest struct {
	Name            *string `json:"name,omitempty"`
	ContractNo      *string `json:"contract_no,omitempty"`
	Remark          *string `json:"remark,omitempty"`
	RpmLimit        *int64  `json:"rpm_limit,omitempty"`
	TpmLimit        *int64  `json:"tpm_limit,omitempty"`
	MaxConcurrency  *int    `json:"max_concurrency,omitempty"`
	ExpectedVersion *int64  `json:"expected_version"`
}

type SupplierContractInactivateRequest struct {
	ExpectedVersion *int64 `json:"expected_version"`
}

type SupplierRateVersionCreateRequest struct {
	ProcurementMultiplierPpm *int64 `json:"procurement_multiplier_ppm"`
	Reason                   string `json:"reason"`
}

type SupplierInventoryAdjustmentCreateRequest struct {
	DeltaMicroUsd *int64 `json:"delta_micro_usd"`
	Type          string `json:"type"`
	Reason        string `json:"reason"`
}

type SupplierExclusionRuleCreateRequest struct {
	UserId int    `json:"user_id"`
	Action string `json:"action"`
	Reason string `json:"reason"`
}

// SupplierChannelBindingRequest is the only JSON surface that can change a
// channel's supplier contract. The generic Channel DTO remains unchanged.
type SupplierChannelBindingRequest struct {
	ContractId         *int `json:"contract_id"`
	ExpectedContractId *int `json:"expected_contract_id"`
}
