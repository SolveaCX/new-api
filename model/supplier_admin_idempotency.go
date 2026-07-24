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
	"github.com/QuantumNous/new-api/types"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SupplierAdminCommandScopeCreateSupplier     = "supplier.create"
	SupplierAdminCommandScopeCreateContract     = "supplier_contract.create"
	SupplierAdminCommandScopeCreateRate         = "supplier_rate.create"
	SupplierAdminCommandScopeCreateInventory    = "supplier_inventory.create"
	SupplierAdminCommandScopeCreateExclusion    = "supplier_exclusion.create"
	SupplierAdminCommandScopeUpdateSupplier     = "supplier.update"
	SupplierAdminCommandScopeInactivateSupplier = "supplier.inactivate"
	SupplierAdminCommandScopeUpdateContract     = "supplier_contract.update"
	SupplierAdminCommandScopeInactivateContract = "supplier_contract.inactivate"
	SupplierAdminCommandScopeBindChannel        = "supplier_channel.bind"
	SupplierAdminCommandScopeUnbindChannel      = "supplier_channel.unbind"

	supplierAdminCommandResourceSupplier  = "supplier"
	supplierAdminCommandResourceContract  = "supplier_contract"
	supplierAdminCommandResourceRate      = "supplier_rate"
	supplierAdminCommandResourceInventory = "supplier_inventory_adjustment"
	supplierAdminCommandResourceExclusion = "supplier_exclusion_rule"
	supplierAdminCommandResourceBinding   = "supplier_channel_binding"

	supplierAdminCommandPayloadVersion  = 1
	maxSupplierAdminIdempotencyKeyBytes = 128
	maxSupplierAdminCommandResultBytes  = 8 * 1024

	legacySupplierAdminCommandScopeKeyIndex        = "ux_supplier_admin_command_scope_key"
	legacySupplierAdminCommandActorScopeKeyIndex   = "idx_supplier_admin_command_actor_scope_key"
	supplierAdminCommandActorScopeDigestIndex      = "ux_supplier_admin_command_actor_scope_digest"
	supplierBatchSchedulerIdentityScopeDigestIndex = "ux_supplier_batch_scheduler_identity_scope_digest"
	legacySupplierInventoryContractKeyIndex        = "ux_supplier_inventory_contract_idempotency"
	supplierInventoryActorLocalIndex               = "ux_supplier_inventory_actor_idempotency"
	SupplierAdminCommandLedgerStateBridge          = "bridge"
	SupplierAdminCommandLedgerStateFinalized       = "finalized"
	SupplierAdminCommandLedgerStateInvalid         = "invalid"
)

var (
	ErrSupplierAdminIdempotencyKeyRequired    = errors.New("supplier admin idempotency key is required")
	ErrSupplierAdminIdempotencyConflict       = errors.New("supplier admin idempotency key payload conflict")
	ErrSupplierAdminCommandIncomplete         = errors.New("supplier admin idempotency command is incomplete")
	ErrSupplierAdminCommandLedgerNotFinalized = errors.New("supplier admin command ledger migration is not finalized")
	ErrSupplierAdminCommandLedgerGateEnabled  = errors.New("supplier mutation gate must be disabled for command ledger finalization")
)

type SupplierAdminCommand struct {
	Id                       int     `json:"id"`
	ActorId                  int     `json:"-" gorm:"not null;default:0"`
	Scope                    string  `json:"scope" gorm:"type:varchar(64);not null"`
	IdempotencyKey           string  `json:"idempotency_key" gorm:"type:varchar(128);not null"`
	IdempotencyKeyDigest     []byte  `json:"-" gorm:"size:32"`
	TrustedJobIdentityDigest *string `json:"-" gorm:"type:varchar(64)"`
	SchedulerRequestDigest   *string `json:"-" gorm:"type:varchar(64)"`
	SchedulerScopeCode       *int    `json:"-"`
	SchedulerSlot            string  `json:"-" gorm:"type:varchar(16);not null;default:''"`
	PayloadVersion           int     `json:"payload_version" gorm:"not null;default:1"`
	PayloadDigest            string  `json:"payload_digest" gorm:"type:varchar(64);not null"`
	ResourceType             string  `json:"resource_type" gorm:"type:varchar(32);not null;index:idx_supplier_admin_command_resource,priority:1"`
	ResourceId               int     `json:"resource_id" gorm:"not null;default:0;index:idx_supplier_admin_command_resource,priority:2"`
	ResultJson               string  `json:"-" gorm:"type:text"`
	StatusJson               string  `json:"-" gorm:"type:text"`
	ClaimToken               string  `json:"-" gorm:"type:varchar(32);not null"`
	CreatedAt                int64   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                int64   `json:"updated_at" gorm:"autoUpdateTime"`
}

func (c *SupplierAdminCommand) BeforeCreate(_ *gorm.DB) error {
	c.Scope = strings.TrimSpace(c.Scope)
	c.IdempotencyKey = strings.TrimSpace(c.IdempotencyKey)
	c.PayloadDigest = strings.TrimSpace(c.PayloadDigest)
	c.ResourceType = strings.TrimSpace(c.ResourceType)
	keyDigest := supplierAdminIdempotencyKeyDigest(c.IdempotencyKey)
	if len(c.IdempotencyKeyDigest) == 0 {
		c.IdempotencyKeyDigest = keyDigest
	}
	if c.ActorId < 0 || c.Scope == "" || len(c.Scope) > 64 || c.IdempotencyKey == "" || len(c.IdempotencyKey) > maxSupplierAdminIdempotencyKeyBytes || !equalDigest(c.IdempotencyKeyDigest, keyDigest) || c.PayloadVersion != supplierAdminCommandPayloadVersion || len(c.PayloadDigest) != sha256.Size*2 || c.ResourceType == "" || len(c.ResourceType) > 32 || len(c.ClaimToken) != 32 {
		return ErrSupplierAdminIdempotencyKeyRequired
	}
	if c.Scope == SupplierBatchSchedulerCommandScopeCatchUp {
		if c.ActorId != 0 || c.TrustedJobIdentityDigest == nil || len(*c.TrustedJobIdentityDigest) != sha256.Size*2 || c.SchedulerRequestDigest == nil || len(*c.SchedulerRequestDigest) != sha256.Size*2 || c.SchedulerScopeCode == nil || *c.SchedulerScopeCode != supplierBatchSchedulerScopeCodeCatchUp ||
			!validSupplierBatchSchedulerAuditSlot(c.SchedulerSlot) || c.ResourceType != supplierBatchSchedulerCommandResource {
			return ErrSupplierAdminIdempotencyKeyRequired
		}
		if _, err := types.ParseSupplierBatchCommandStateV1(c.StatusJson); err != nil {
			return ErrSupplierAdminCommandIncomplete
		}
	} else if isSupplierDailyReportRerunScope(c.Scope) {
		if c.ActorId <= 0 || c.TrustedJobIdentityDigest != nil || c.SchedulerRequestDigest != nil || c.SchedulerScopeCode != nil || c.SchedulerSlot != "" || c.ResourceType != supplierDailyReportRerunCommandResource {
			return ErrSupplierAdminIdempotencyKeyRequired
		}
		if _, err := types.ParseSupplierBatchCommandStateV1(c.StatusJson); err != nil {
			return ErrSupplierAdminCommandIncomplete
		}
	} else if c.TrustedJobIdentityDigest != nil || c.SchedulerRequestDigest != nil || c.SchedulerScopeCode != nil || c.SchedulerSlot != "" || c.StatusJson != "" {
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

type SupplierAdminCommandClaim struct {
	Command  SupplierAdminCommand
	Claimed  bool
	Replayed bool
}

func (c *SupplierAdminCommandClaim) DecodeResult(out any) error {
	if c == nil || c.Command.ResourceId <= 0 || c.Command.ResultJson == "" || out == nil {
		return ErrSupplierAdminCommandIncomplete
	}
	if err := common.UnmarshalJsonStr(c.Command.ResultJson, out); err != nil {
		return fmt.Errorf("decode supplier admin command result: %w", err)
	}
	return nil
}

// ClaimSupplierAdminCommandTx claims an actor-local command inside the caller's
// transaction. Exact replay returns the stored command and conflicting payloads
// fail without exposing commands owned by another actor.
func ClaimSupplierAdminCommandTx(tx *gorm.DB, actorId int, scope string, idempotencyKey string, payload any, resourceType string) (*SupplierAdminCommandClaim, error) {
	payloadDigest, err := supplierAdminPayloadDigest(payload)
	if err != nil {
		return nil, err
	}
	return claimSupplierAdminCommand(tx, actorId, scope, idempotencyKey, payloadDigest, resourceType)
}

func claimSupplierAdminCommand(tx *gorm.DB, actorId int, scope string, idempotencyKey string, payloadDigest string, resourceType string) (*SupplierAdminCommandClaim, error) {
	if tx == nil {
		return nil, fmt.Errorf("claim supplier admin command: %w", ErrDatabase)
	}
	scope = strings.TrimSpace(scope)
	key := strings.TrimSpace(idempotencyKey)
	resourceType = strings.TrimSpace(resourceType)
	if actorId < 0 || scope == "" || len(scope) > 64 || key == "" || len(key) > maxSupplierAdminIdempotencyKeyBytes || resourceType == "" || len(resourceType) > 32 || len(payloadDigest) != sha256.Size*2 {
		return nil, ErrSupplierAdminIdempotencyKeyRequired
	}
	keyDigest := supplierAdminIdempotencyKeyDigest(key)
	claimToken, err := newSupplierAdminClaimToken()
	if err != nil {
		return nil, err
	}
	candidate := SupplierAdminCommand{
		ActorId:              actorId,
		Scope:                scope,
		IdempotencyKey:       key,
		IdempotencyKeyDigest: keyDigest,
		PayloadVersion:       supplierAdminCommandPayloadVersion,
		PayloadDigest:        payloadDigest,
		ResourceType:         resourceType,
		ClaimToken:           claimToken,
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "actor_id"}, {Name: "scope"}, {Name: "idempotency_key_digest"}},
		DoNothing: true,
	}).Create(&candidate).Error; err != nil {
		return nil, err
	}
	var persisted SupplierAdminCommand
	if err := tx.Where("actor_id = ? AND scope = ? AND idempotency_key_digest = ?", actorId, scope, keyDigest).First(&persisted).Error; err != nil {
		return nil, err
	}
	if persisted.IdempotencyKey != key || persisted.PayloadVersion != supplierAdminCommandPayloadVersion || persisted.PayloadDigest != payloadDigest || persisted.ResourceType != resourceType {
		return nil, ErrSupplierAdminIdempotencyConflict
	}
	claimed := persisted.ClaimToken == claimToken
	if !claimed && persisted.ResourceId <= 0 {
		return nil, ErrSupplierAdminCommandIncomplete
	}
	return &SupplierAdminCommandClaim{Command: persisted, Claimed: claimed, Replayed: !claimed}, nil
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

func SupplierUpdateCommandScope(id int) string {
	return SupplierAdminCommandScopeUpdateSupplier + "/" + strconv.Itoa(id)
}
func SupplierInactivateCommandScope(id int) string {
	return SupplierAdminCommandScopeInactivateSupplier + "/" + strconv.Itoa(id)
}
func SupplierContractUpdateCommandScope(id int) string {
	return SupplierAdminCommandScopeUpdateContract + "/" + strconv.Itoa(id)
}
func SupplierContractInactivateCommandScope(id int) string {
	return SupplierAdminCommandScopeInactivateContract + "/" + strconv.Itoa(id)
}
func SupplierChannelBindCommandScope(id int) string {
	return SupplierAdminCommandScopeBindChannel + "/" + strconv.Itoa(id)
}
func SupplierChannelUnbindCommandScope(id int) string {
	return SupplierAdminCommandScopeUnbindChannel + "/" + strconv.Itoa(id)
}

func parseSupplierAdminCommandScope(scope string) (string, int, bool) {
	scope = strings.TrimSpace(scope)
	switch scope {
	case SupplierAdminCommandScopeCreateSupplier, SupplierAdminCommandScopeCreateContract, SupplierAdminCommandScopeCreateRate:
		return scope, 0, true
	case SupplierAdminCommandScopeCreateExclusion:
		return scope, 0, true
	}
	dynamic := []struct {
		base      string
		canonical func(int) string
	}{
		{SupplierAdminCommandScopeCreateInventory, SupplierInventoryCommandScope},
		{SupplierAdminCommandScopeUpdateSupplier, SupplierUpdateCommandScope},
		{SupplierAdminCommandScopeInactivateSupplier, SupplierInactivateCommandScope},
		{SupplierAdminCommandScopeUpdateContract, SupplierContractUpdateCommandScope},
		{SupplierAdminCommandScopeInactivateContract, SupplierContractInactivateCommandScope},
		{SupplierAdminCommandScopeBindChannel, SupplierChannelBindCommandScope},
		{SupplierAdminCommandScopeUnbindChannel, SupplierChannelUnbindCommandScope},
	}
	for _, candidate := range dynamic {
		prefix := candidate.base + "/"
		if !strings.HasPrefix(scope, prefix) {
			continue
		}
		subjectID, err := strconv.Atoi(strings.TrimPrefix(scope, prefix))
		if err != nil || subjectID <= 0 || scope != candidate.canonical(subjectID) {
			return "", 0, false
		}
		return candidate.base, subjectID, true
	}
	return "", 0, false
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

// CompleteSupplierAdminCommandTx stores the authoritative replay result in the
// same transaction as the domain mutation. A transaction rollback removes both
// the claim and completion, so failed commands never leave partial ledger rows.
func CompleteSupplierAdminCommandTx(tx *gorm.DB, claim *SupplierAdminCommandClaim, resourceId int, commandResult any) error {
	if claim == nil || !claim.Claimed || claim.Command.Id <= 0 || resourceId <= 0 {
		return ErrSupplierAdminCommandIncomplete
	}
	if tx == nil {
		return fmt.Errorf("complete supplier admin command: %w", ErrDatabase)
	}
	resultJson := ""
	if commandResult != nil {
		encoded, err := common.Marshal(commandResult)
		if err != nil {
			return fmt.Errorf("encode supplier admin command result: %w", err)
		}
		if len(encoded) == 0 || len(encoded) > maxSupplierAdminCommandResultBytes {
			return ErrSupplierAdminCommandIncomplete
		}
		resultJson = string(encoded)
	}
	result := tx.Model(&SupplierAdminCommand{}).
		Where("id = ? AND claim_token = ? AND resource_id = 0", claim.Command.Id, claim.Command.ClaimToken).
		UpdateColumns(map[string]any{"resource_id": resourceId, "result_json": resultJson})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrSupplierAdminCommandIncomplete
	}
	claim.Command.ResourceId = resourceId
	claim.Command.ResultJson = resultJson
	return nil
}

func completeSupplierAdminCommand(tx *gorm.DB, claim *SupplierAdminCommandClaim, resourceId int) error {
	return CompleteSupplierAdminCommandTx(tx, claim, resourceId, nil)
}

func supplierAdminPayloadDigest(payload any) (string, error) {
	encoded, err := common.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func supplierAdminIdempotencyKeyDigest(idempotencyKey string) []byte {
	digest := sha256.Sum256([]byte(strings.TrimSpace(idempotencyKey)))
	return digest[:]
}

func equalDigest(left []byte, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var different byte
	for i := range left {
		different |= left[i] ^ right[i]
	}
	return different == 0
}

type SupplierAdminCommandLedgerMigrationStatus struct {
	HasRequiredSupplierSchema bool
	HasControlOptionTable     bool
	HasDigestColumn           bool
	HasResultColumn           bool
	HasSchedulerColumns       bool
	HasActorDigestIndex       bool
	HasSchedulerDigestIndex   bool
	LegacyScopeKeyIndex       bool
	LegacyActorScopeKeyIndex  bool
	HasInventoryActorIndex    bool
	LegacyInventoryKeyIndex   bool
	InvalidDigestRows         int64
	Finalized                 bool
}

func (status SupplierAdminCommandLedgerMigrationStatus) State() string {
	validBase := status.HasRequiredSupplierSchema && status.HasControlOptionTable && status.HasDigestColumn && status.HasResultColumn &&
		status.HasSchedulerColumns && status.HasActorDigestIndex && status.HasSchedulerDigestIndex && status.HasInventoryActorIndex && status.InvalidDigestRows == 0
	if !validBase {
		return SupplierAdminCommandLedgerStateInvalid
	}
	legacyCount := 0
	for _, present := range []bool{status.LegacyScopeKeyIndex, status.LegacyActorScopeKeyIndex, status.LegacyInventoryKeyIndex} {
		if present {
			legacyCount++
		}
	}
	if legacyCount == 3 {
		return SupplierAdminCommandLedgerStateBridge
	}
	if legacyCount == 0 && status.Finalized {
		return SupplierAdminCommandLedgerStateFinalized
	}
	return SupplierAdminCommandLedgerStateInvalid
}

// MigrateSupplierAdminCommandLedger bridges old and new writers. It adds only
// nullable ledger columns, strictly backfills and validates every digest, and
// creates actor-local indexes while retaining legacy indexes for mixed-version
// writers until the explicit post-drain finalization step.
func MigrateSupplierAdminCommandLedger(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate supplier admin command ledger: %w", ErrDatabase)
	}
	if !db.Migrator().HasTable(&SupplierAdminCommand{}) {
		if err := db.Migrator().CreateTable(&SupplierAdminCommand{}); err != nil && !db.Migrator().HasTable(&SupplierAdminCommand{}) {
			return fmt.Errorf("create supplier admin command ledger: %w", err)
		}
	}
	if err := migrateSupplierOptimisticConcurrency(db); err != nil {
		return err
	}
	if err := ensureSupplierInventoryActorLocalIndex(db); err != nil {
		return err
	}
	if err := ensureSupplierAdminCommandLedgerColumn(db, "IdempotencyKeyDigest"); err != nil {
		return err
	}
	if err := ensureSupplierAdminCommandLedgerColumn(db, "ResultJson"); err != nil {
		return err
	}
	for _, field := range []string{"TrustedJobIdentityDigest", "SchedulerRequestDigest", "SchedulerScopeCode", "SchedulerSlot", "StatusJson", "UpdatedAt"} {
		if err := ensureSupplierAdminCommandLedgerColumn(db, field); err != nil {
			return err
		}
	}
	if err := backfillAndValidateSupplierAdminCommandDigests(db); err != nil {
		return err
	}
	if err := ensureSupplierAdminCommandActorDigestIndex(db); err != nil {
		return err
	}
	if err := ensureSupplierBatchSchedulerDigestIndex(db); err != nil {
		return err
	}
	return nil
}

// FinalizeSupplierAdminCommandLedgerMigration is the explicit post-drain
// cutover. It revalidates the bridge, then removes legacy uniqueness and proves
// the resulting schema/data state. It is safe to rerun across application nodes.
func FinalizeSupplierAdminCommandLedgerMigration(db *gorm.DB) error {
	if err := validateSupplierAdminCommandLedgerPrerequisites(db); err != nil {
		return err
	}
	if err := validateSupplierAccountingMutationGateDisabled(db); err != nil {
		return err
	}
	if err := MigrateSupplierAdminCommandLedger(db); err != nil {
		return err
	}
	if err := validateSupplierAccountingMutationGateDisabled(db); err != nil {
		return err
	}
	migrator := db.Migrator()
	for _, legacyIndex := range []string{legacySupplierAdminCommandScopeKeyIndex, legacySupplierAdminCommandActorScopeKeyIndex} {
		if !migrator.HasIndex(&SupplierAdminCommand{}, legacyIndex) {
			continue
		}
		if err := migrator.DropIndex(&SupplierAdminCommand{}, legacyIndex); err != nil && migrator.HasIndex(&SupplierAdminCommand{}, legacyIndex) {
			return fmt.Errorf("drop legacy supplier admin command index %s: %w", legacyIndex, err)
		}
	}
	if err := dropSupplierIndexIfPresent(db, legacySupplierInventoryContractKeyIndex); err != nil {
		return fmt.Errorf("drop legacy supplier inventory index %s: %w", legacySupplierInventoryContractKeyIndex, err)
	}
	if err := ValidateSupplierAdminCommandLedgerFinalized(db); err != nil {
		return err
	}
	return validateSupplierAccountingMutationGateDisabled(db)
}

func GetSupplierAdminCommandLedgerMigrationStatus(db *gorm.DB) (SupplierAdminCommandLedgerMigrationStatus, error) {
	if db == nil {
		return SupplierAdminCommandLedgerMigrationStatus{}, fmt.Errorf("get supplier admin command ledger migration status: %w", ErrDatabase)
	}
	migrator := db.Migrator()
	var err error
	status := SupplierAdminCommandLedgerMigrationStatus{
		HasRequiredSupplierSchema: supplierAdminCommandLedgerRequiredSchemaPresent(db),
		HasControlOptionTable:     migrator.HasTable(&Option{}),
		HasDigestColumn:           migrator.HasColumn(&SupplierAdminCommand{}, "IdempotencyKeyDigest"),
		HasResultColumn:           migrator.HasColumn(&SupplierAdminCommand{}, "ResultJson"),
		HasSchedulerColumns:       migrator.HasColumn(&SupplierAdminCommand{}, "TrustedJobIdentityDigest") && migrator.HasColumn(&SupplierAdminCommand{}, "SchedulerRequestDigest") && migrator.HasColumn(&SupplierAdminCommand{}, "SchedulerScopeCode") && migrator.HasColumn(&SupplierAdminCommand{}, "SchedulerSlot") && migrator.HasColumn(&SupplierAdminCommand{}, "StatusJson"),
		LegacyScopeKeyIndex:       migrator.HasIndex(&SupplierAdminCommand{}, legacySupplierAdminCommandScopeKeyIndex),
		LegacyActorScopeKeyIndex:  migrator.HasIndex(&SupplierAdminCommand{}, legacySupplierAdminCommandActorScopeKeyIndex),
	}
	if migrator.HasTable(&SupplierInventoryAdjustment{}) {
		status.HasInventoryActorIndex, err = supplierUniqueIndexMatches(db, "supplier_inventory_adjustments", supplierInventoryActorLocalIndex, []string{"contract_id", "created_by", "idempotency_key"})
		if err != nil {
			return status, err
		}
		status.LegacyInventoryKeyIndex = migrator.HasIndex(&SupplierInventoryAdjustment{}, legacySupplierInventoryContractKeyIndex)
	}
	if status.HasDigestColumn {
		valid, err := supplierAdminCommandIndexMatches(db, supplierAdminCommandActorScopeDigestIndex, []string{"actor_id", "scope", "idempotency_key_digest"})
		if err != nil {
			return status, err
		}
		status.HasActorDigestIndex = valid
		status.HasSchedulerDigestIndex, err = supplierAdminCommandIndexMatches(db, supplierBatchSchedulerIdentityScopeDigestIndex, []string{"trusted_job_identity_digest", "scheduler_scope_code", "scheduler_request_digest"})
		if err != nil {
			return status, err
		}
		status.InvalidDigestRows, err = countInvalidSupplierAdminCommandDigests(db)
		if err != nil {
			return status, err
		}
	}
	status.Finalized = status.HasRequiredSupplierSchema && status.HasControlOptionTable && status.HasDigestColumn && status.HasResultColumn && status.HasSchedulerColumns && status.HasActorDigestIndex && status.HasSchedulerDigestIndex && status.HasInventoryActorIndex &&
		!status.LegacyScopeKeyIndex && !status.LegacyActorScopeKeyIndex && !status.LegacyInventoryKeyIndex && status.InvalidDigestRows == 0
	return status, nil
}

func ValidateSupplierAdminCommandLedgerFinalized(db *gorm.DB) error {
	status, err := GetSupplierAdminCommandLedgerMigrationStatus(db)
	if err != nil {
		return err
	}
	if !status.Finalized {
		return fmt.Errorf("validate supplier admin command ledger: %+v: %w", status, ErrSupplierAdminCommandLedgerNotFinalized)
	}
	return nil
}

func validateSupplierAdminCommandLedgerPrerequisites(db *gorm.DB) error {
	if db == nil || !supplierAdminCommandLedgerRequiredSchemaPresent(db) || !db.Migrator().HasTable(&Option{}) {
		return fmt.Errorf("validate supplier admin command ledger prerequisites: %w", ErrSupplierAdminCommandLedgerNotFinalized)
	}
	return nil
}

func supplierAdminCommandLedgerRequiredSchemaPresent(db *gorm.DB) bool {
	if db == nil {
		return false
	}
	for _, table := range []any{&SupplierAdminCommand{}, &SupplierInventoryAdjustment{}} {
		if !db.Migrator().HasTable(table) {
			return false
		}
	}
	return true
}

func validateSupplierAccountingMutationGateDisabled(db *gorm.DB) error {
	state, err := ReadSupplierAccountingMutationState(db)
	if err != nil {
		return err
	}
	if state.Enabled {
		return ErrSupplierAdminCommandLedgerGateEnabled
	}
	return nil
}

func SupplierDatabaseIdentity(db *gorm.DB) (string, error) {
	if db == nil {
		return "", fmt.Errorf("read supplier database identity: %w", ErrDatabase)
	}
	switch db.Dialector.Name() {
	case "mysql":
		var databaseName string
		if err := db.Raw("SELECT DATABASE()").Scan(&databaseName).Error; err != nil {
			return "", err
		}
		databaseName = strings.TrimSpace(databaseName)
		if databaseName == "" {
			return "", fmt.Errorf("read supplier database identity: empty MySQL database: %w", ErrDatabase)
		}
		return "mysql:" + databaseName, nil
	case "postgres":
		var identity struct {
			DatabaseName string `gorm:"column:database_name"`
			SchemaName   string `gorm:"column:schema_name"`
		}
		if err := db.Raw("SELECT current_database() AS database_name, current_schema() AS schema_name").Scan(&identity).Error; err != nil {
			return "", err
		}
		identity.DatabaseName = strings.TrimSpace(identity.DatabaseName)
		identity.SchemaName = strings.TrimSpace(identity.SchemaName)
		if identity.DatabaseName == "" || identity.SchemaName == "" {
			return "", fmt.Errorf("read supplier database identity: empty PostgreSQL database/schema: %w", ErrDatabase)
		}
		return "postgres:" + identity.DatabaseName + "/" + identity.SchemaName, nil
	case "sqlite":
		return "sqlite:main", nil
	default:
		return "", fmt.Errorf("read supplier database identity for dialect %q: %w", db.Dialector.Name(), ErrDatabase)
	}
}

func ensureSupplierAdminCommandLedgerColumn(db *gorm.DB, field string) error {
	migrator := db.Migrator()
	if migrator.HasColumn(&SupplierAdminCommand{}, field) {
		return nil
	}
	if err := migrator.AddColumn(&SupplierAdminCommand{}, field); err != nil && !migrator.HasColumn(&SupplierAdminCommand{}, field) {
		return fmt.Errorf("add supplier admin command ledger column %s: %w", field, err)
	}
	return nil
}

func backfillAndValidateSupplierAdminCommandDigests(db *gorm.DB) error {
	lastId := 0
	for {
		var commands []SupplierAdminCommand
		if err := db.Where("id > ?", lastId).Order("id ASC").Limit(500).Find(&commands).Error; err != nil {
			return fmt.Errorf("load supplier admin command digests: %w", err)
		}
		if len(commands) == 0 {
			break
		}
		for i := range commands {
			key := strings.TrimSpace(commands[i].IdempotencyKey)
			if commands[i].Id <= 0 || key == "" || key != commands[i].IdempotencyKey || len(key) > maxSupplierAdminIdempotencyKeyBytes {
				return fmt.Errorf("validate supplier admin command digest source %d: %w", commands[i].Id, ErrSupplierAdminCommandIncomplete)
			}
			expected := supplierAdminIdempotencyKeyDigest(commands[i].IdempotencyKey)
			if !equalDigest(commands[i].IdempotencyKeyDigest, expected) {
				result := db.Session(&gorm.Session{SkipHooks: true}).Model(&SupplierAdminCommand{}).
					Where("id = ?", commands[i].Id).
					UpdateColumn("idempotency_key_digest", expected)
				if result.Error != nil {
					return fmt.Errorf("backfill supplier admin command digest %d: %w", commands[i].Id, result.Error)
				}
				if result.RowsAffected != 1 {
					var persisted SupplierAdminCommand
					if err := db.Select("idempotency_key_digest").First(&persisted, commands[i].Id).Error; err != nil || !equalDigest(persisted.IdempotencyKeyDigest, expected) {
						return fmt.Errorf("backfill supplier admin command digest %d: %w", commands[i].Id, ErrSupplierAdminCommandIncomplete)
					}
				}
			}
			lastId = commands[i].Id
		}
	}
	invalid, err := countInvalidSupplierAdminCommandDigests(db)
	if err != nil {
		return err
	}
	if invalid != 0 {
		return fmt.Errorf("validate supplier admin command digests: %d invalid rows: %w", invalid, ErrSupplierAdminCommandIncomplete)
	}
	return nil
}

func countInvalidSupplierAdminCommandDigests(db *gorm.DB) (int64, error) {
	var commands []SupplierAdminCommand
	if err := db.Select("id", "idempotency_key", "idempotency_key_digest").Order("id ASC").Find(&commands).Error; err != nil {
		return 0, fmt.Errorf("validate supplier admin command digests: %w", err)
	}
	var invalid int64
	for i := range commands {
		key := strings.TrimSpace(commands[i].IdempotencyKey)
		if commands[i].Id <= 0 || key == "" || key != commands[i].IdempotencyKey || len(key) > maxSupplierAdminIdempotencyKeyBytes ||
			!equalDigest(commands[i].IdempotencyKeyDigest, supplierAdminIdempotencyKeyDigest(key)) {
			invalid++
		}
	}
	return invalid, nil
}

func ensureSupplierAdminCommandActorDigestIndex(db *gorm.DB) error {
	expected := []string{"actor_id", "scope", "idempotency_key_digest"}
	matches, err := supplierAdminCommandIndexMatches(db, supplierAdminCommandActorScopeDigestIndex, expected)
	if err != nil {
		return err
	}
	if matches {
		return nil
	}
	if db.Migrator().HasIndex(&SupplierAdminCommand{}, supplierAdminCommandActorScopeDigestIndex) {
		return fmt.Errorf("supplier admin command actor digest index has unexpected definition: %w", ErrSupplierAdminCommandIncomplete)
	}
	quote := `"`
	if db.Dialector.Name() == "mysql" {
		quote = "`"
	}
	identifier := func(value string) string { return quote + value + quote }
	statement := "CREATE UNIQUE INDEX " + identifier(supplierAdminCommandActorScopeDigestIndex) + " ON " + identifier("supplier_admin_commands") +
		" (" + identifier("actor_id") + ", " + identifier("scope") + ", " + identifier("idempotency_key_digest") + ")"
	if err := db.Exec(statement).Error; err != nil {
		matches, verifyErr := supplierAdminCommandIndexMatches(db, supplierAdminCommandActorScopeDigestIndex, expected)
		if verifyErr != nil || !matches {
			return fmt.Errorf("create supplier admin command actor digest index: %w", err)
		}
	}
	return nil
}

func ensureSupplierBatchSchedulerDigestIndex(db *gorm.DB) error {
	expected := []string{"trusted_job_identity_digest", "scheduler_scope_code", "scheduler_request_digest"}
	matches, err := supplierAdminCommandIndexMatches(db, supplierBatchSchedulerIdentityScopeDigestIndex, expected)
	if err != nil || matches {
		return err
	}
	if db.Migrator().HasIndex(&SupplierAdminCommand{}, supplierBatchSchedulerIdentityScopeDigestIndex) {
		return fmt.Errorf("supplier batch scheduler digest index has unexpected definition: %w", ErrSupplierAdminCommandIncomplete)
	}
	quote := `"`
	if db.Dialector.Name() == "mysql" {
		quote = "`"
	}
	identifier := func(value string) string { return quote + value + quote }
	statement := "CREATE UNIQUE INDEX " + identifier(supplierBatchSchedulerIdentityScopeDigestIndex) + " ON " + identifier("supplier_admin_commands") +
		" (" + identifier("trusted_job_identity_digest") + ", " + identifier("scheduler_scope_code") + ", " + identifier("scheduler_request_digest") + ")"
	if err := db.Exec(statement).Error; err != nil {
		matches, verifyErr := supplierAdminCommandIndexMatches(db, supplierBatchSchedulerIdentityScopeDigestIndex, expected)
		if verifyErr != nil || !matches {
			return fmt.Errorf("create supplier batch scheduler digest index: %w", err)
		}
	}
	return nil
}

func supplierAdminCommandIndexMatches(db *gorm.DB, indexName string, expected []string) (bool, error) {
	return supplierUniqueIndexMatches(db, "supplier_admin_commands", indexName, expected)
}

func supplierUniqueIndexMatches(db *gorm.DB, tableName, indexName string, expected []string) (bool, error) {
	columns := make([]string, 0, len(expected))
	switch db.Dialector.Name() {
	case "sqlite":
		var indexes []struct {
			Name    string `gorm:"column:name"`
			Unique  int    `gorm:"column:unique"`
			Partial int    `gorm:"column:partial"`
		}
		if err := db.Raw("PRAGMA index_list('" + tableName + "')").Scan(&indexes).Error; err != nil {
			return false, err
		}
		validDefinition := false
		for _, index := range indexes {
			if index.Name == indexName {
				validDefinition = index.Unique == 1 && index.Partial == 0
				break
			}
		}
		if !validDefinition {
			return false, nil
		}
		var rows []struct {
			Cid  int     `gorm:"column:cid"`
			Name *string `gorm:"column:name"`
		}
		if err := db.Raw("PRAGMA index_info('" + indexName + "')").Scan(&rows).Error; err != nil {
			return false, err
		}
		for _, row := range rows {
			if row.Cid < 0 || row.Name == nil {
				return false, nil
			}
			columns = append(columns, *row.Name)
		}
	case "mysql":
		var rows []struct {
			ColumnName *string `gorm:"column:COLUMN_NAME"`
			NonUnique  int     `gorm:"column:NON_UNIQUE"`
			SubPart    *int64  `gorm:"column:SUB_PART"`
		}
		if err := db.Raw("SELECT column_name, non_unique, sub_part FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ? ORDER BY seq_in_index", tableName, indexName).Scan(&rows).Error; err != nil {
			return false, err
		}
		for _, row := range rows {
			// A NULL column identifies a functional key part on MySQL versions
			// that support expression indexes. sub_part identifies a prefix key.
			if row.NonUnique != 0 || row.ColumnName == nil || row.SubPart != nil {
				return false, nil
			}
			columns = append(columns, *row.ColumnName)
		}
	case "postgres":
		var rows []struct {
			ColumnName string `gorm:"column:column_name"`
		}
		query := `SELECT a.attname AS column_name
FROM pg_class t
JOIN pg_namespace ns ON ns.oid = t.relnamespace
JOIN pg_index ix ON t.oid = ix.indrelid
JOIN pg_class i ON i.oid = ix.indexrelid
JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY AS k(attnum, ord) ON TRUE
JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
WHERE ns.nspname = current_schema() AND t.relname = ? AND i.relname = ?
  AND ix.indisunique = TRUE AND ix.indisvalid = TRUE AND ix.indisready = TRUE
  AND ix.indpred IS NULL AND ix.indexprs IS NULL
ORDER BY k.ord`
		if err := db.Raw(query, tableName, indexName).Scan(&rows).Error; err != nil {
			return false, err
		}
		for _, row := range rows {
			columns = append(columns, row.ColumnName)
		}
	default:
		return false, fmt.Errorf("unsupported supplier admin command ledger dialect %q: %w", db.Dialector.Name(), ErrDatabase)
	}
	if len(columns) != len(expected) {
		return false, nil
	}
	for i := range expected {
		if columns[i] != expected[i] {
			return false, nil
		}
	}
	return true, nil
}

func migrateSupplierOptimisticConcurrency(db *gorm.DB) error {
	for _, target := range []struct {
		model any
		field string
	}{{&UpstreamSupplier{}, "RowVersion"}, {&SupplierContract{}, "RowVersion"}} {
		if !db.Migrator().HasTable(target.model) {
			continue
		}
		if !db.Migrator().HasColumn(target.model, target.field) {
			if err := db.Migrator().AddColumn(target.model, target.field); err != nil && !db.Migrator().HasColumn(target.model, target.field) {
				return err
			}
		}
		if err := db.Session(&gorm.Session{SkipHooks: true}).Model(target.model).Where("row_version IS NULL OR row_version <= ?", 0).UpdateColumn("row_version", 1).Error; err != nil {
			return err
		}
		var invalid int64
		if err := db.Model(target.model).Where("row_version IS NULL OR row_version <= ?", 0).Count(&invalid).Error; err != nil {
			return err
		}
		if invalid != 0 {
			return ErrSupplierVersionConflict
		}
	}
	return nil
}

func ensureSupplierInventoryActorLocalIndex(db *gorm.DB) error {
	if !db.Migrator().HasTable(&SupplierInventoryAdjustment{}) {
		return nil
	}
	expected := []string{"contract_id", "created_by", "idempotency_key"}
	return ensureSupplierInventoryUniqueIndex(db, supplierInventoryActorLocalIndex, expected)
}

func ensureSupplierInventoryUniqueIndex(db *gorm.DB, indexName string, expected []string) error {
	matches, err := supplierUniqueIndexMatches(db, "supplier_inventory_adjustments", indexName, expected)
	if err != nil || matches {
		return err
	}
	statement, err := supplierCreateUniqueIndexStatement(db.Dialector.Name(), "supplier_inventory_adjustments", indexName, expected)
	if err != nil {
		return err
	}
	if err := db.Exec(statement).Error; err != nil {
		verified, verifyErr := supplierUniqueIndexMatches(db, "supplier_inventory_adjustments", indexName, expected)
		if verifyErr != nil || !verified {
			return err
		}
	}
	verified, err := supplierUniqueIndexMatches(db, "supplier_inventory_adjustments", indexName, expected)
	if err != nil || !verified {
		if err != nil {
			return err
		}
		return ErrSupplierAdminCommandIncomplete
	}
	return nil
}

func supplierCreateUniqueIndexStatement(dialect, tableName, indexName string, columns []string) (string, error) {
	quote := `"`
	switch dialect {
	case "mysql":
		quote = "`"
	case "sqlite", "postgres":
	default:
		return "", fmt.Errorf("unsupported supplier inventory index dialect %q: %w", dialect, ErrDatabase)
	}
	q := func(value string) string { return quote + value + quote }
	quotedColumns := make([]string, len(columns))
	for index, column := range columns {
		quotedColumns[index] = q(column)
	}
	return "CREATE UNIQUE INDEX " + q(indexName) + " ON " + q(tableName) + " (" + strings.Join(quotedColumns, ", ") + ")", nil
}

func dropSupplierIndexIfPresent(db *gorm.DB, indexName string) error {
	if !db.Migrator().HasIndex(&SupplierInventoryAdjustment{}, indexName) {
		return nil
	}
	if err := db.Migrator().DropIndex(&SupplierInventoryAdjustment{}, indexName); err != nil && db.Migrator().HasIndex(&SupplierInventoryAdjustment{}, indexName) {
		return err
	}
	return nil
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
