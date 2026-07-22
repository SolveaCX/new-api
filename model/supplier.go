package model

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SupplierStatusActive   = "active"
	SupplierStatusInactive = "inactive"

	SupplierContractStatusActive   = "active"
	SupplierContractStatusInactive = "inactive"

	SupplierInventoryAdjustmentTypeInitial       = "initial"
	SupplierInventoryAdjustmentTypeReplenishment = "replenishment"
	SupplierInventoryAdjustmentTypeCorrection    = "correction"
	SupplierInventoryAdjustmentTypeReversal      = "reversal"

	SupplierStatisticsActionExclude = "exclude"
	SupplierStatisticsActionInclude = "include"
)

var (
	ErrSupplierAppendOnly          = errors.New("supplier record is append-only")
	ErrSupplierImmutableField      = errors.New("supplier immutable field cannot be changed")
	ErrSupplierInvalidStatus       = errors.New("invalid supplier status")
	ErrSupplierInvalidContract     = errors.New("invalid supplier contract")
	ErrSupplierInvalidRate         = errors.New("invalid supplier contract rate")
	ErrSupplierInvalidInventory    = errors.New("invalid supplier inventory adjustment")
	ErrSupplierInvalidStatsRule    = errors.New("invalid supplier statistics exclusion rule")
	ErrSupplierHardDeleteForbidden = errors.New("supplier configuration cannot be hard deleted")
	ErrSupplierInactive            = errors.New("supplier is inactive")
	ErrSupplierContractInactive    = errors.New("supplier contract is inactive")
	ErrSupplierContractBound       = errors.New("supplier contract is still bound to a channel")
	ErrSupplierHasActiveContracts  = errors.New("supplier still has active contracts")
	ErrSupplierHasChannelBindings  = errors.New("supplier still has channel bindings")
	ErrSupplierCurrentRateRequired = errors.New("supplier contract current rate is required")
	ErrSupplierIdempotencyConflict = errors.New("supplier idempotency key payload conflict")
)

type UpstreamSupplier struct {
	Id        int    `json:"id"`
	Name      string `json:"name" gorm:"type:varchar(128);not null;uniqueIndex:ux_upstream_suppliers_name"`
	Status    string `json:"status" gorm:"type:varchar(32);not null;default:'active'"`
	Remark    string `json:"remark" gorm:"type:text"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (s *UpstreamSupplier) BeforeCreate(_ *gorm.DB) error {
	s.Name = strings.TrimSpace(s.Name)
	s.Status = strings.TrimSpace(s.Status)
	if s.Status == "" {
		s.Status = SupplierStatusActive
	}
	if s.Name == "" || !isSupplierStatus(s.Status) {
		return ErrSupplierInvalidStatus
	}
	return nil
}

func (s *UpstreamSupplier) BeforeUpdate(tx *gorm.DB) error {
	if tx.Statement.Changed("Id", "Status") {
		return ErrSupplierImmutableField
	}
	if tx.Statement.Changed("Name") && strings.TrimSpace(s.Name) == "" {
		return ErrSupplierInvalidStatus
	}
	return nil
}

func (s *UpstreamSupplier) BeforeDelete(_ *gorm.DB) error {
	return ErrSupplierHardDeleteForbidden
}

type SupplierContract struct {
	Id                   int    `json:"id"`
	SupplierId           int    `json:"supplier_id" gorm:"not null;index:idx_supplier_contracts_supplier;index:idx_supplier_contracts_supplier_status,priority:1"`
	Name                 string `json:"name" gorm:"type:varchar(128);not null"`
	ContractNo           string `json:"contract_no" gorm:"type:varchar(128);not null"`
	Remark               string `json:"remark" gorm:"type:text"`
	Status               string `json:"status" gorm:"type:varchar(32);not null;default:'active';index:idx_supplier_contracts_supplier_status,priority:2"`
	CurrentRateVersionId *int   `json:"current_rate_version_id"`
	RpmLimit             int64  `json:"rpm_limit" gorm:"not null;default:0"`
	TpmLimit             int64  `json:"tpm_limit" gorm:"not null;default:0"`
	MaxConcurrency       int    `json:"max_concurrency" gorm:"not null;default:0"`
	CreatedAt            int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (c *SupplierContract) BeforeCreate(_ *gorm.DB) error {
	c.Name = strings.TrimSpace(c.Name)
	c.ContractNo = strings.TrimSpace(c.ContractNo)
	c.Status = strings.TrimSpace(c.Status)
	if c.Status == "" {
		c.Status = SupplierContractStatusActive
	}
	if c.SupplierId <= 0 || c.Name == "" || c.ContractNo == "" || !isSupplierContractStatus(c.Status) || c.RpmLimit < 0 || c.TpmLimit < 0 || c.MaxConcurrency < 0 {
		return ErrSupplierInvalidContract
	}
	return nil
}

func (c *SupplierContract) BeforeUpdate(tx *gorm.DB) error {
	if tx.Statement.Changed("Id", "SupplierId", "Status", "CurrentRateVersionId") {
		return ErrSupplierImmutableField
	}
	if (tx.Statement.Changed("Name") && strings.TrimSpace(c.Name) == "") ||
		(tx.Statement.Changed("ContractNo") && strings.TrimSpace(c.ContractNo) == "") ||
		(tx.Statement.Changed("RpmLimit") && c.RpmLimit < 0) ||
		(tx.Statement.Changed("TpmLimit") && c.TpmLimit < 0) ||
		(tx.Statement.Changed("MaxConcurrency") && c.MaxConcurrency < 0) {
		return ErrSupplierInvalidContract
	}
	return nil
}

func (c *SupplierContract) BeforeDelete(_ *gorm.DB) error {
	return ErrSupplierHardDeleteForbidden
}

type SupplierContractRateVersion struct {
	Id                       int    `json:"id" gorm:"index:idx_supplier_rate_versions_contract_effective,priority:3"`
	ContractId               int    `json:"contract_id" gorm:"not null;index:idx_supplier_rate_versions_contract_effective,priority:1"`
	ProcurementMultiplierPpm int64  `json:"procurement_multiplier_ppm" gorm:"not null"`
	EffectiveAt              int64  `json:"effective_at" gorm:"not null;index:idx_supplier_rate_versions_contract_effective,priority:2"`
	CreatedBy                int    `json:"created_by" gorm:"not null"`
	Reason                   string `json:"reason" gorm:"type:text"`
	CreatedAt                int64  `json:"created_at" gorm:"autoCreateTime"`
}

// SupplierChannelBindingVersion is the immutable evidence for a channel's
// supplier-contract assignment at a point in time. Channel.SupplierContractId
// remains the current-state projection used by routing and admin queries.
type SupplierChannelBindingVersion struct {
	Id                         int   `json:"id" gorm:"index:idx_supplier_channel_binding_history,priority:3"`
	ChannelId                  int   `json:"channel_id" gorm:"not null;index:idx_supplier_channel_binding_history,priority:1"`
	PreviousSupplierContractId *int  `json:"previous_supplier_contract_id"`
	SupplierContractId         *int  `json:"supplier_contract_id"`
	EffectiveAt                int64 `json:"effective_at" gorm:"not null;index:idx_supplier_channel_binding_history,priority:2"`
	CreatedBy                  int   `json:"created_by" gorm:"not null;default:0"`
	CreatedAt                  int64 `json:"created_at" gorm:"autoCreateTime"`
}

func (v *SupplierChannelBindingVersion) BeforeCreate(tx *gorm.DB) error {
	if v.ChannelId <= 0 || supplierContractIdsEqual(v.PreviousSupplierContractId, v.SupplierContractId) {
		return ErrSupplierInvalidContract
	}
	effectiveAt, err := getSupplierDBTimestamp(tx)
	if err != nil {
		return fmt.Errorf("assign supplier channel binding effective time: %w", err)
	}
	v.EffectiveAt = effectiveAt
	return nil
}

func (v *SupplierChannelBindingVersion) BeforeUpdate(_ *gorm.DB) error {
	return ErrSupplierAppendOnly
}

func (v *SupplierChannelBindingVersion) BeforeDelete(_ *gorm.DB) error {
	return ErrSupplierAppendOnly
}

func (v *SupplierContractRateVersion) BeforeCreate(tx *gorm.DB) error {
	v.Reason = strings.TrimSpace(v.Reason)
	effectiveAt, err := getSupplierDBTimestamp(tx)
	if err != nil {
		return fmt.Errorf("assign supplier rate effective time: %w", err)
	}
	v.EffectiveAt = effectiveAt
	if v.ContractId <= 0 || v.ProcurementMultiplierPpm < 0 || v.ProcurementMultiplierPpm > 1_000_000 || v.CreatedBy <= 0 {
		return ErrSupplierInvalidRate
	}
	return nil
}

func (v *SupplierContractRateVersion) BeforeUpdate(_ *gorm.DB) error {
	return ErrSupplierAppendOnly
}

func (v *SupplierContractRateVersion) BeforeDelete(_ *gorm.DB) error {
	return ErrSupplierAppendOnly
}

type SupplierInventoryAdjustment struct {
	Id             int    `json:"id" gorm:"index:idx_supplier_inventory_contract_id,priority:2"`
	ContractId     int    `json:"contract_id" gorm:"not null;uniqueIndex:ux_supplier_inventory_contract_idempotency,priority:1;index:idx_supplier_inventory_contract_id,priority:1"`
	DeltaMicroUsd  int64  `json:"delta_micro_usd" gorm:"not null"`
	Type           string `json:"type" gorm:"type:varchar(32);not null"`
	Reason         string `json:"reason" gorm:"type:text"`
	IdempotencyKey string `json:"idempotency_key" gorm:"type:varchar(128);not null;uniqueIndex:ux_supplier_inventory_contract_idempotency,priority:2"`
	CreatedBy      int    `json:"created_by" gorm:"not null"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (a *SupplierInventoryAdjustment) BeforeCreate(_ *gorm.DB) error {
	a.Type = strings.TrimSpace(a.Type)
	a.Reason = strings.TrimSpace(a.Reason)
	a.IdempotencyKey = strings.TrimSpace(a.IdempotencyKey)
	if a.ContractId <= 0 || a.DeltaMicroUsd == 0 || !isSupplierInventoryAdjustmentType(a.Type) || a.IdempotencyKey == "" || a.CreatedBy <= 0 {
		return ErrSupplierInvalidInventory
	}
	return nil
}

func (a *SupplierInventoryAdjustment) BeforeUpdate(_ *gorm.DB) error {
	return ErrSupplierAppendOnly
}

func (a *SupplierInventoryAdjustment) BeforeDelete(_ *gorm.DB) error {
	return ErrSupplierAppendOnly
}

type SupplierStatisticsExclusionRule struct {
	Id             int    `json:"id" gorm:"index:idx_supplier_stats_rules_user_effective,priority:3"`
	UserId         int    `json:"user_id" gorm:"not null;index:idx_supplier_stats_rules_user_effective,priority:1"`
	Action         string `json:"action" gorm:"type:varchar(16);not null"`
	EffectiveAt    int64  `json:"effective_at" gorm:"not null;index:idx_supplier_stats_rules_user_effective,priority:2"`
	Reason         string `json:"reason" gorm:"type:text"`
	IdempotencyKey string `json:"idempotency_key" gorm:"type:varchar(128);not null;uniqueIndex:ux_supplier_stats_rule_creator_idempotency,priority:2"`
	CreatedBy      int    `json:"created_by" gorm:"not null;uniqueIndex:ux_supplier_stats_rule_creator_idempotency,priority:1"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (r *SupplierStatisticsExclusionRule) BeforeCreate(tx *gorm.DB) error {
	r.Action = strings.TrimSpace(r.Action)
	r.Reason = strings.TrimSpace(r.Reason)
	r.IdempotencyKey = strings.TrimSpace(r.IdempotencyKey)
	effectiveAt, err := getSupplierDBTimestamp(tx)
	if err != nil {
		return fmt.Errorf("assign supplier exclusion effective time: %w", err)
	}
	r.EffectiveAt = effectiveAt
	if r.UserId <= 0 || !isSupplierStatisticsAction(r.Action) || r.IdempotencyKey == "" || r.CreatedBy <= 0 {
		return ErrSupplierInvalidStatsRule
	}
	return nil
}

func (r *SupplierStatisticsExclusionRule) BeforeUpdate(_ *gorm.DB) error {
	return ErrSupplierAppendOnly
}

func (r *SupplierStatisticsExclusionRule) BeforeDelete(_ *gorm.DB) error {
	return ErrSupplierAppendOnly
}

func CreateAndActivateSupplierContractRateVersion(contractId int, procurementMultiplierPpm int64, createdBy int, reason string) (*SupplierContractRateVersion, error) {
	if DB == nil {
		return nil, fmt.Errorf("create supplier rate version: %w", ErrDatabase)
	}
	var version *SupplierContractRateVersion
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		version, err = createAndActivateSupplierContractRateVersionTx(tx, contractId, procurementMultiplierPpm, createdBy, reason)
		return err
	})
	if err != nil {
		return nil, err
	}
	refreshLocalChannelCacheAndPublishChanged()
	return version, nil
}

func createAndActivateSupplierContractRateVersionTx(tx *gorm.DB, contractId int, procurementMultiplierPpm int64, createdBy int, reason string) (*SupplierContractRateVersion, error) {
	if tx == nil {
		return nil, ErrDatabase
	}
	if _, _, _, err := lockActiveSupplierContractChain(tx, contractId, false); err != nil {
		return nil, err
	}
	version := &SupplierContractRateVersion{
		ContractId:               contractId,
		ProcurementMultiplierPpm: procurementMultiplierPpm,
		CreatedBy:                createdBy,
		Reason:                   reason,
	}
	if err := tx.Create(version).Error; err != nil {
		return nil, err
	}
	if err := tx.Model(&SupplierContract{}).Where("id = ?", contractId).UpdateColumns(map[string]any{
		"current_rate_version_id": version.Id,
		"updated_at":              version.EffectiveAt,
	}).Error; err != nil {
		return nil, err
	}
	return version, nil
}

func CreateSupplierStatisticsExclusionRule(userId int, action string, createdBy int, reason string, idempotencyKey string) (*SupplierStatisticsExclusionRule, error) {
	if DB == nil {
		return nil, fmt.Errorf("create supplier exclusion rule: %w", ErrDatabase)
	}
	rule := &SupplierStatisticsExclusionRule{
		UserId:         userId,
		Action:         action,
		Reason:         reason,
		IdempotencyKey: idempotencyKey,
		CreatedBy:      createdBy,
	}
	created := false
	if err := DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "created_by"}, {Name: "idempotency_key"}},
			DoNothing: true,
		}).Create(rule)
		if result.Error != nil {
			return result.Error
		}
		created = result.RowsAffected == 1 && rule.Id > 0
		var existing SupplierStatisticsExclusionRule
		if err := tx.Where("created_by = ? AND idempotency_key = ?", createdBy, strings.TrimSpace(idempotencyKey)).First(&existing).Error; err != nil {
			return err
		}
		if existing.UserId != userId || existing.Action != strings.TrimSpace(action) || existing.Reason != strings.TrimSpace(reason) {
			return ErrSupplierIdempotencyConflict
		}
		*rule = existing
		return nil
	}); err != nil {
		return nil, err
	}
	if created {
		refreshLocalChannelCacheAndPublishChanged()
	}
	return rule, nil
}

func isSupplierStatus(status string) bool {
	return status == SupplierStatusActive || status == SupplierStatusInactive
}

func isSupplierContractStatus(status string) bool {
	return status == SupplierContractStatusActive || status == SupplierContractStatusInactive
}

func isSupplierInventoryAdjustmentType(adjustmentType string) bool {
	switch adjustmentType {
	case SupplierInventoryAdjustmentTypeInitial,
		SupplierInventoryAdjustmentTypeReplenishment,
		SupplierInventoryAdjustmentTypeCorrection,
		SupplierInventoryAdjustmentTypeReversal:
		return true
	default:
		return false
	}
}

func isSupplierStatisticsAction(action string) bool {
	return action == SupplierStatisticsActionExclude || action == SupplierStatisticsActionInclude
}

func supplierContractIdsEqual(left *int, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func getSupplierDBTimestamp(tx *gorm.DB) (int64, error) {
	if tx == nil {
		return 0, ErrDatabase
	}
	var timestamp int64
	var err error
	switch tx.Dialector.Name() {
	case "postgres":
		err = tx.Raw("SELECT EXTRACT(EPOCH FROM clock_timestamp())::bigint").Scan(&timestamp).Error
	case "sqlite":
		err = tx.Raw("SELECT CAST(strftime('%s', 'now') AS INTEGER)").Scan(&timestamp).Error
	case "mysql":
		err = tx.Raw("SELECT UNIX_TIMESTAMP()").Scan(&timestamp).Error
	default:
		return 0, fmt.Errorf("unsupported supplier database dialect %q", tx.Dialector.Name())
	}
	if err != nil {
		return 0, err
	}
	if timestamp <= 0 {
		return 0, errors.New("database returned invalid supplier timestamp")
	}
	return timestamp, nil
}
