package model

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SupplierAdminCommandScopeCreateSupplier  = "supplier.create"
	SupplierAdminCommandScopeCreateContract  = "supplier_contract.create"
	SupplierAdminCommandScopeCreateRate      = "supplier_rate.create"
	SupplierAdminCommandScopeCreateInventory = "supplier_inventory.create"
	SupplierAdminCommandScopeCreateExclusion = "supplier_exclusion.create"

	supplierAdminCommandResourceSupplier  = "supplier"
	supplierAdminCommandResourceContract  = "supplier_contract"
	supplierAdminCommandResourceRate      = "supplier_rate"
	supplierAdminCommandResourceInventory = "supplier_inventory_adjustment"
	supplierAdminCommandResourceExclusion = "supplier_exclusion_rule"

	supplierAdminCommandPayloadVersion  = 1
	maxSupplierAdminIdempotencyKeyBytes = 128
)

var (
	ErrSupplierAdminIdempotencyKeyRequired = errors.New("supplier admin idempotency key is required")
	ErrSupplierAdminIdempotencyConflict    = errors.New("supplier admin idempotency key payload conflict")
	ErrSupplierAdminCommandIncomplete      = errors.New("supplier admin idempotency command is incomplete")
)

type SupplierAdminCommand struct {
	Id             int    `json:"id"`
	ActorId        int    `json:"-" gorm:"not null;default:0;index:idx_supplier_admin_command_actor_scope_key,priority:1"`
	Scope          string `json:"scope" gorm:"type:varchar(64);not null;uniqueIndex:ux_supplier_admin_command_scope_key,priority:1;index:idx_supplier_admin_command_actor_scope_key,priority:2"`
	IdempotencyKey string `json:"idempotency_key" gorm:"type:varchar(128);not null;uniqueIndex:ux_supplier_admin_command_scope_key,priority:2;index:idx_supplier_admin_command_actor_scope_key,priority:3"`
	PayloadVersion int    `json:"payload_version" gorm:"not null;default:1"`
	PayloadDigest  string `json:"payload_digest" gorm:"type:varchar(64);not null"`
	ResourceType   string `json:"resource_type" gorm:"type:varchar(32);not null;index:idx_supplier_admin_command_resource,priority:1"`
	ResourceId     int    `json:"resource_id" gorm:"not null;default:0;index:idx_supplier_admin_command_resource,priority:2"`
	ClaimToken     string `json:"-" gorm:"type:varchar(32);not null"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (c *SupplierAdminCommand) BeforeCreate(_ *gorm.DB) error {
	c.Scope = strings.TrimSpace(c.Scope)
	c.IdempotencyKey = strings.TrimSpace(c.IdempotencyKey)
	c.PayloadDigest = strings.TrimSpace(c.PayloadDigest)
	c.ResourceType = strings.TrimSpace(c.ResourceType)
	if c.Scope == "" || c.IdempotencyKey == "" || len(c.IdempotencyKey) > maxSupplierAdminIdempotencyKeyBytes || c.PayloadVersion != supplierAdminCommandPayloadVersion || len(c.PayloadDigest) != sha256.Size*2 || c.ResourceType == "" || len(c.ClaimToken) != 32 {
		return ErrSupplierAdminIdempotencyKeyRequired
	}
	return nil
}

func (c *SupplierAdminCommand) BeforeUpdate(_ *gorm.DB) error {
	return ErrSupplierAppendOnly
}

func (c *SupplierAdminCommand) BeforeDelete(_ *gorm.DB) error {
	return ErrSupplierAppendOnly
}

type supplierAdminCommandClaim struct {
	Command  SupplierAdminCommand
	Claimed  bool
	Replayed bool
}

func claimSupplierAdminCommand(tx *gorm.DB, actorId int, scope string, idempotencyKey string, payloadDigest string, resourceType string) (*supplierAdminCommandClaim, error) {
	key := strings.TrimSpace(idempotencyKey)
	if actorId < 0 || key == "" || len(key) > maxSupplierAdminIdempotencyKeyBytes {
		return nil, ErrSupplierAdminIdempotencyKeyRequired
	}
	claimToken, err := newSupplierAdminClaimToken()
	if err != nil {
		return nil, err
	}
	candidate := SupplierAdminCommand{
		ActorId:        actorId,
		Scope:          scope,
		IdempotencyKey: key,
		PayloadVersion: supplierAdminCommandPayloadVersion,
		PayloadDigest:  payloadDigest,
		ResourceType:   resourceType,
		ClaimToken:     claimToken,
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "scope"}, {Name: "idempotency_key"}},
		DoNothing: true,
	}).Create(&candidate).Error; err != nil {
		return nil, err
	}
	var persisted SupplierAdminCommand
	if err := tx.Where("scope = ? AND idempotency_key = ?", scope, key).First(&persisted).Error; err != nil {
		return nil, err
	}
	if persisted.ActorId != actorId || persisted.PayloadVersion != supplierAdminCommandPayloadVersion || persisted.PayloadDigest != payloadDigest || persisted.ResourceType != resourceType {
		return nil, ErrSupplierAdminIdempotencyConflict
	}
	claimed := persisted.ClaimToken == claimToken
	if !claimed && persisted.ResourceId <= 0 {
		return nil, ErrSupplierAdminCommandIncomplete
	}
	return &supplierAdminCommandClaim{Command: persisted, Claimed: claimed, Replayed: !claimed}, nil
}

type SupplierAdminCommandResult struct {
	Scope          string `json:"scope"`
	IdempotencyKey string `json:"idempotency_key"`
	ResourceType   string `json:"resource_type"`
	ResourceId     int    `json:"resource_id"`
	CreatedAt      int64  `json:"created_at"`
}

func IsSupplierAdminCommandScope(scope string) bool {
	_, _, ok := parseSupplierAdminCommandScope(scope)
	return ok
}

func SupplierInventoryCommandScope(contractId int) string {
	return SupplierAdminCommandScopeCreateInventory + "/" + strconv.Itoa(contractId)
}

func parseSupplierAdminCommandScope(scope string) (string, int, bool) {
	scope = strings.TrimSpace(scope)
	switch scope {
	case SupplierAdminCommandScopeCreateSupplier, SupplierAdminCommandScopeCreateContract, SupplierAdminCommandScopeCreateRate:
		return scope, 0, true
	case SupplierAdminCommandScopeCreateExclusion:
		return scope, 0, true
	}
	prefix := SupplierAdminCommandScopeCreateInventory + "/"
	if !strings.HasPrefix(scope, prefix) {
		return "", 0, false
	}
	contractId, err := strconv.Atoi(strings.TrimPrefix(scope, prefix))
	if err != nil || contractId <= 0 || scope != SupplierInventoryCommandScope(contractId) {
		return "", 0, false
	}
	return SupplierAdminCommandScopeCreateInventory, contractId, true
}

// GetSupplierAdminCommandResult returns only completed commands owned by the
// requesting administrator. Actor mismatches intentionally have the same
// not-found result as missing keys to avoid leaking another admin's commands.
func GetSupplierAdminCommandResult(actorId int, scope string, idempotencyKey string) (*SupplierAdminCommandResult, error) {
	if DB == nil {
		return nil, fmt.Errorf("get supplier admin command result: %w", ErrDatabase)
	}
	scope = strings.TrimSpace(scope)
	key := strings.TrimSpace(idempotencyKey)
	baseScope, subjectId, validScope := parseSupplierAdminCommandScope(scope)
	if actorId <= 0 || !validScope || key == "" || len(key) > maxSupplierAdminIdempotencyKeyBytes {
		return nil, ErrSupplierAdminIdempotencyKeyRequired
	}
	if baseScope == SupplierAdminCommandScopeCreateInventory {
		var adjustment SupplierInventoryAdjustment
		if err := DB.Where("created_by = ? AND contract_id = ? AND idempotency_key = ?", actorId, subjectId, key).Take(&adjustment).Error; err != nil {
			return nil, err
		}
		return &SupplierAdminCommandResult{Scope: scope, IdempotencyKey: adjustment.IdempotencyKey, ResourceType: supplierAdminCommandResourceInventory, ResourceId: adjustment.Id, CreatedAt: adjustment.CreatedAt}, nil
	}
	if baseScope == SupplierAdminCommandScopeCreateExclusion {
		var rule SupplierStatisticsExclusionRule
		if err := DB.Where("created_by = ? AND idempotency_key = ?", actorId, key).Take(&rule).Error; err != nil {
			return nil, err
		}
		return &SupplierAdminCommandResult{Scope: scope, IdempotencyKey: rule.IdempotencyKey, ResourceType: supplierAdminCommandResourceExclusion, ResourceId: rule.Id, CreatedAt: rule.CreatedAt}, nil
	}
	var command SupplierAdminCommand
	if err := DB.Where("actor_id = ? AND scope = ? AND idempotency_key = ? AND resource_id > 0", actorId, scope, key).Take(&command).Error; err != nil {
		return nil, err
	}
	return &SupplierAdminCommandResult{
		Scope: command.Scope, IdempotencyKey: command.IdempotencyKey,
		ResourceType: command.ResourceType, ResourceId: command.ResourceId, CreatedAt: command.CreatedAt,
	}, nil
}

func completeSupplierAdminCommand(tx *gorm.DB, claim *supplierAdminCommandClaim, resourceId int) error {
	if claim == nil || !claim.Claimed || claim.Command.Id <= 0 || resourceId <= 0 {
		return ErrSupplierAdminCommandIncomplete
	}
	result := tx.Model(&SupplierAdminCommand{}).
		Where("id = ? AND claim_token = ? AND resource_id = 0", claim.Command.Id, claim.Command.ClaimToken).
		UpdateColumn("resource_id", resourceId)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrSupplierAdminCommandIncomplete
	}
	claim.Command.ResourceId = resourceId
	return nil
}

func supplierAdminPayloadDigest(payload any) (string, error) {
	encoded, err := common.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func newSupplierAdminClaimToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate supplier admin command claim token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

type createUpstreamSupplierCommandPayload struct {
	Name   string `json:"name"`
	Remark string `json:"remark"`
}

func CreateUpstreamSupplierIdempotent(supplier *UpstreamSupplier, idempotencyKey string) (*UpstreamSupplier, bool, error) {
	return CreateUpstreamSupplierIdempotentForActor(supplier, idempotencyKey, 0)
}

func CreateUpstreamSupplierIdempotentForActor(supplier *UpstreamSupplier, idempotencyKey string, actorId int) (*UpstreamSupplier, bool, error) {
	if DB == nil {
		return nil, false, fmt.Errorf("create upstream supplier idempotently: %w", ErrDatabase)
	}
	if supplier == nil {
		return nil, false, ErrSupplierInvalidStatus
	}
	payload := createUpstreamSupplierCommandPayload{Name: strings.TrimSpace(supplier.Name), Remark: strings.TrimSpace(supplier.Remark)}
	digest, err := supplierAdminPayloadDigest(payload)
	if err != nil {
		return nil, false, err
	}
	var result UpstreamSupplier
	replayed := false
	err = DB.Transaction(func(tx *gorm.DB) error {
		claim, err := claimSupplierAdminCommand(tx, actorId, SupplierAdminCommandScopeCreateSupplier, idempotencyKey, digest, supplierAdminCommandResourceSupplier)
		if err != nil {
			return err
		}
		replayed = claim.Replayed
		if replayed {
			return tx.First(&result, claim.Command.ResourceId).Error
		}
		result = UpstreamSupplier{Name: payload.Name, Remark: payload.Remark, Status: SupplierStatusActive}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		return completeSupplierAdminCommand(tx, claim, result.Id)
	})
	if err != nil {
		return nil, false, err
	}
	*supplier = result
	if !replayed {
		refreshLocalChannelCacheAndPublishChanged()
	}
	return &result, replayed, nil
}

type createSupplierContractCommandPayload struct {
	SupplierId     int    `json:"supplier_id"`
	Name           string `json:"name"`
	ContractNo     string `json:"contract_no"`
	Remark         string `json:"remark"`
	RpmLimit       int64  `json:"rpm_limit"`
	TpmLimit       int64  `json:"tpm_limit"`
	MaxConcurrency int    `json:"max_concurrency"`
}

func CreateSupplierContractIdempotent(contract *SupplierContract, idempotencyKey string) (*SupplierContract, bool, error) {
	return CreateSupplierContractIdempotentForActor(contract, idempotencyKey, 0)
}

func CreateSupplierContractIdempotentForActor(contract *SupplierContract, idempotencyKey string, actorId int) (*SupplierContract, bool, error) {
	if DB == nil {
		return nil, false, fmt.Errorf("create supplier contract idempotently: %w", ErrDatabase)
	}
	if contract == nil {
		return nil, false, ErrSupplierInvalidContract
	}
	payload := createSupplierContractCommandPayload{
		SupplierId:     contract.SupplierId,
		Name:           strings.TrimSpace(contract.Name),
		ContractNo:     strings.TrimSpace(contract.ContractNo),
		Remark:         strings.TrimSpace(contract.Remark),
		RpmLimit:       contract.RpmLimit,
		TpmLimit:       contract.TpmLimit,
		MaxConcurrency: contract.MaxConcurrency,
	}
	digest, err := supplierAdminPayloadDigest(payload)
	if err != nil {
		return nil, false, err
	}
	var result SupplierContract
	replayed := false
	err = DB.Transaction(func(tx *gorm.DB) error {
		claim, err := claimSupplierAdminCommand(tx, actorId, SupplierAdminCommandScopeCreateContract, idempotencyKey, digest, supplierAdminCommandResourceContract)
		if err != nil {
			return err
		}
		replayed = claim.Replayed
		if replayed {
			return tx.First(&result, claim.Command.ResourceId).Error
		}
		var upstreamSupplier UpstreamSupplier
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&upstreamSupplier, payload.SupplierId).Error; err != nil {
			return err
		}
		if upstreamSupplier.Status != SupplierStatusActive {
			return ErrSupplierInactive
		}
		result = SupplierContract{
			SupplierId:     payload.SupplierId,
			Name:           payload.Name,
			ContractNo:     payload.ContractNo,
			Remark:         payload.Remark,
			Status:         SupplierContractStatusActive,
			RpmLimit:       payload.RpmLimit,
			TpmLimit:       payload.TpmLimit,
			MaxConcurrency: payload.MaxConcurrency,
		}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		return completeSupplierAdminCommand(tx, claim, result.Id)
	})
	if err != nil {
		return nil, false, err
	}
	*contract = result
	if !replayed {
		refreshLocalChannelCacheAndPublishChanged()
	}
	return &result, replayed, nil
}

type createSupplierRateCommandPayload struct {
	ContractId               int    `json:"contract_id"`
	ProcurementMultiplierPpm int64  `json:"procurement_multiplier_ppm"`
	CreatedBy                int    `json:"created_by"`
	Reason                   string `json:"reason"`
}

func CreateAndActivateSupplierContractRateVersionIdempotent(contractId int, procurementMultiplierPpm int64, createdBy int, reason string, idempotencyKey string) (*SupplierContractRateVersion, bool, error) {
	if DB == nil {
		return nil, false, fmt.Errorf("create supplier rate version idempotently: %w", ErrDatabase)
	}
	payload := createSupplierRateCommandPayload{
		ContractId:               contractId,
		ProcurementMultiplierPpm: procurementMultiplierPpm,
		CreatedBy:                createdBy,
		Reason:                   strings.TrimSpace(reason),
	}
	digest, err := supplierAdminPayloadDigest(payload)
	if err != nil {
		return nil, false, err
	}
	var result SupplierContractRateVersion
	replayed := false
	err = DB.Transaction(func(tx *gorm.DB) error {
		claim, err := claimSupplierAdminCommand(tx, createdBy, SupplierAdminCommandScopeCreateRate, idempotencyKey, digest, supplierAdminCommandResourceRate)
		if err != nil {
			return err
		}
		replayed = claim.Replayed
		if replayed {
			return tx.First(&result, claim.Command.ResourceId).Error
		}
		version, err := createAndActivateSupplierContractRateVersionTx(tx, payload.ContractId, payload.ProcurementMultiplierPpm, payload.CreatedBy, payload.Reason)
		if err != nil {
			return err
		}
		result = *version
		return completeSupplierAdminCommand(tx, claim, result.Id)
	})
	if err != nil {
		return nil, false, err
	}
	if !replayed {
		refreshLocalChannelCacheAndPublishChanged()
	}
	return &result, replayed, nil
}
