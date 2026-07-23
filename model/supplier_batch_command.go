package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SupplierBatchSchedulerCommandScopeCatchUp = "supplier_batch.catch_up"
	SupplierBatchSchedulerAuditSlotCurrent    = "current"
	SupplierBatchSchedulerAuditSlotNext       = "next"

	supplierBatchSchedulerCommandResource  = "supplier_batch_request"
	supplierBatchSchedulerScopeCodeCatchUp = 1
)

type SupplierBatchSchedulerCommandClaim struct {
	Command  SupplierAdminCommand
	State    types.SupplierBatchCommandStateV1
	Claimed  bool
	Replayed bool
}

func DigestSupplierBatchTrustedIdentity(identity string) ([]byte, error) {
	identity = strings.TrimSpace(identity)
	if identity == "" || len(identity) > 256 {
		return nil, ErrSupplierAdminIdempotencyKeyRequired
	}
	digest := sha256.Sum256([]byte(identity))
	return digest[:], nil
}

// ClaimSupplierBatchSchedulerCommand commits a date-independent request claim.
// Callers must complete this operation before selecting or acquiring a batch
// date so a lost response and the no-work outcome remain request-addressable.
func ClaimSupplierBatchSchedulerCommand(ctx context.Context, db *gorm.DB, trustedIdentityDigest []byte, requestID string, requestSemantics any, auditSlot string) (*SupplierBatchSchedulerCommandClaim, error) {
	if db == nil || len(trustedIdentityDigest) != sha256.Size || !validSupplierBatchSchedulerAuditSlot(auditSlot) {
		return nil, ErrSupplierAdminIdempotencyKeyRequired
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > maxSupplierAdminIdempotencyKeyBytes {
		return nil, ErrSupplierAdminIdempotencyKeyRequired
	}
	payloadDigest, err := supplierAdminPayloadDigest(requestSemantics)
	if err != nil {
		return nil, err
	}
	claimedState := types.SupplierBatchCommandStateV1{SchemaVersion: types.SupplierBatchCommandSchemaVersion, State: types.SupplierBatchCommandStateClaimed}
	statusJSON, err := types.EncodeSupplierBatchCommandStateV1(claimedState)
	if err != nil {
		return nil, err
	}
	var claim *SupplierBatchSchedulerCommandClaim
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		claimToken, tokenErr := newSupplierAdminClaimToken()
		if tokenErr != nil {
			return tokenErr
		}
		requestDigest := supplierAdminIdempotencyKeyDigest(requestID)
		identityDigestHex := hex.EncodeToString(trustedIdentityDigest)
		requestDigestHex := hex.EncodeToString(requestDigest)
		scopeCode := supplierBatchSchedulerScopeCodeCatchUp
		candidate := SupplierAdminCommand{
			ActorId: 0, Scope: SupplierBatchSchedulerCommandScopeCatchUp, IdempotencyKey: requestID,
			IdempotencyKeyDigest:     requestDigest,
			TrustedJobIdentityDigest: &identityDigestHex, SchedulerRequestDigest: &requestDigestHex, SchedulerScopeCode: &scopeCode, SchedulerSlot: auditSlot,
			PayloadVersion: supplierAdminCommandPayloadVersion, PayloadDigest: payloadDigest,
			ResourceType: supplierBatchSchedulerCommandResource, StatusJson: statusJSON, ClaimToken: claimToken,
		}
		if createErr := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "trusted_job_identity_digest"}, {Name: "scheduler_scope_code"}, {Name: "scheduler_request_digest"}}, DoNothing: true,
		}).Create(&candidate).Error; createErr != nil {
			return createErr
		}
		var persisted SupplierAdminCommand
		if queryErr := tx.Where("trusted_job_identity_digest = ? AND scheduler_scope_code = ? AND scheduler_request_digest = ?", identityDigestHex, scopeCode, requestDigestHex).First(&persisted).Error; queryErr != nil {
			return queryErr
		}
		if persisted.IdempotencyKey != requestID || persisted.PayloadVersion != supplierAdminCommandPayloadVersion || persisted.PayloadDigest != payloadDigest || persisted.ResourceType != supplierBatchSchedulerCommandResource {
			return ErrSupplierAdminIdempotencyConflict
		}
		state, parseErr := types.ParseSupplierBatchCommandStateV1(persisted.StatusJson)
		if parseErr != nil {
			return ErrSupplierAdminCommandIncomplete
		}
		isClaimed := persisted.ClaimToken == claimToken
		claim = &SupplierBatchSchedulerCommandClaim{Command: persisted, State: state, Claimed: isClaimed, Replayed: !isClaimed}
		return nil
	})
	return claim, err
}

func GetSupplierBatchSchedulerCommand(ctx context.Context, db *gorm.DB, trustedIdentityDigest []byte, requestID string) (*SupplierBatchSchedulerCommandClaim, error) {
	if db == nil || len(trustedIdentityDigest) != sha256.Size {
		return nil, ErrSupplierAdminIdempotencyKeyRequired
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > maxSupplierAdminIdempotencyKeyBytes {
		return nil, ErrSupplierAdminIdempotencyKeyRequired
	}
	var command SupplierAdminCommand
	requestDigest := supplierAdminIdempotencyKeyDigest(requestID)
	if err := db.WithContext(ctx).Where("trusted_job_identity_digest = ? AND scheduler_scope_code = ? AND scheduler_request_digest = ?", hex.EncodeToString(trustedIdentityDigest), supplierBatchSchedulerScopeCodeCatchUp, hex.EncodeToString(requestDigest)).First(&command).Error; err != nil {
		return nil, err
	}
	if command.IdempotencyKey != requestID {
		return nil, ErrSupplierAdminIdempotencyConflict
	}
	state, err := types.ParseSupplierBatchCommandStateV1(command.StatusJson)
	if err != nil {
		return nil, ErrSupplierAdminCommandIncomplete
	}
	return &SupplierBatchSchedulerCommandClaim{Command: command, State: state, Replayed: true}, nil
}

func StoreSupplierBatchSchedulerCommandState(ctx context.Context, db *gorm.DB, claim *SupplierBatchSchedulerCommandClaim, state types.SupplierBatchCommandStateV1) error {
	if db == nil || claim == nil || !claim.Claimed || claim.Command.Id <= 0 || claim.Command.ClaimToken == "" {
		return ErrSupplierAdminCommandIncomplete
	}
	statusJSON, err := types.EncodeSupplierBatchCommandStateV1(state)
	if err != nil {
		return err
	}
	result := db.WithContext(ctx).Model(&SupplierAdminCommand{}).
		Where("id = ? AND claim_token = ?", claim.Command.Id, claim.Command.ClaimToken).
		UpdateColumns(map[string]any{"status_json": statusJSON, "updated_at": time.Now().Unix()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrSupplierAdminCommandIncomplete
	}
	claim.Command.StatusJson = statusJSON
	claim.State = state
	return nil
}

// ReconcileSupplierBatchSchedulerCommandState closes a persisted running
// scheduler command after the caller has independently proven the terminal
// outcome for the same batch fence. It does not rotate claim ownership or
// mutate the request/payload anchors. An identical lost CAS race returns the
// stored terminal state as a replay; a divergent race remains busy for caller
// re-read.
func ReconcileSupplierBatchSchedulerCommandState(ctx context.Context, db *gorm.DB, replay *SupplierBatchSchedulerCommandClaim, terminal types.SupplierBatchCommandStateV1) (*SupplierBatchSchedulerCommandClaim, error) {
	if db == nil || !validSupplierBatchSchedulerRunningReplay(replay) || !validSupplierBatchReconciledTerminal(replay.State, terminal) {
		return nil, ErrSupplierAdminCommandIncomplete
	}
	terminalJSON, err := types.EncodeSupplierBatchCommandStateV1(terminal)
	if err != nil {
		return nil, err
	}
	updatedAt := time.Now().Unix()
	result := supplierBatchSchedulerCommandAnchorQuery(db.WithContext(ctx), replay.Command).
		Where("status_json = ?", replay.Command.StatusJson).
		UpdateColumns(map[string]any{"status_json": terminalJSON, "updated_at": updatedAt})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 1 {
		command := replay.Command
		command.StatusJson = terminalJSON
		command.UpdatedAt = updatedAt
		return &SupplierBatchSchedulerCommandClaim{Command: command, State: terminal}, nil
	}

	var persisted SupplierAdminCommand
	err = supplierBatchSchedulerCommandAnchorQuery(db.WithContext(ctx), replay.Command).First(&persisted).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSupplierAdminCommandIncomplete
	}
	if err != nil {
		return nil, err
	}
	persistedState, err := types.ParseSupplierBatchCommandStateV1(persisted.StatusJson)
	if err != nil {
		return nil, ErrSupplierAdminCommandIncomplete
	}
	if persisted.StatusJson != terminalJSON {
		return nil, ErrSupplierDailyBatchBusy
	}
	return &SupplierBatchSchedulerCommandClaim{Command: persisted, State: persistedState, Replayed: true}, nil
}

func validSupplierBatchSchedulerRunningReplay(replay *SupplierBatchSchedulerCommandClaim) bool {
	if replay == nil || replay.Claimed || !replay.Replayed {
		return false
	}
	command := replay.Command
	if command.Id <= 0 || command.ActorId != 0 || command.Scope != SupplierBatchSchedulerCommandScopeCatchUp ||
		command.IdempotencyKey == "" || len(command.IdempotencyKey) > maxSupplierAdminIdempotencyKeyBytes || strings.TrimSpace(command.IdempotencyKey) != command.IdempotencyKey ||
		!equalDigest(command.IdempotencyKeyDigest, supplierAdminIdempotencyKeyDigest(command.IdempotencyKey)) ||
		command.TrustedJobIdentityDigest == nil || !validSupplierBatchSchedulerSHA256Hex(*command.TrustedJobIdentityDigest) ||
		command.SchedulerRequestDigest == nil || *command.SchedulerRequestDigest != hex.EncodeToString(supplierAdminIdempotencyKeyDigest(command.IdempotencyKey)) ||
		command.SchedulerScopeCode == nil || *command.SchedulerScopeCode != supplierBatchSchedulerScopeCodeCatchUp || !validSupplierBatchSchedulerAuditSlot(command.SchedulerSlot) ||
		command.PayloadVersion != supplierAdminCommandPayloadVersion || !validSupplierBatchSchedulerSHA256Hex(command.PayloadDigest) ||
		command.ResourceType != supplierBatchSchedulerCommandResource || command.ResourceId != 0 || command.ResultJson != "" || len(command.ClaimToken) != 32 || command.StatusJson == "" {
		return false
	}
	canonicalRunning, err := types.EncodeSupplierBatchCommandStateV1(replay.State)
	if err != nil || canonicalRunning != command.StatusJson || replay.State.State != types.SupplierBatchCommandStateRunning || replay.State.Response == nil ||
		replay.State.Response.PublishedFenceToken >= replay.State.Response.FenceToken {
		return false
	}
	return replay.State.Response.RequestID == command.IdempotencyKey
}

func validSupplierBatchReconciledTerminal(running, terminal types.SupplierBatchCommandStateV1) bool {
	if running.State != types.SupplierBatchCommandStateRunning || running.Response == nil || terminal.Response == nil ||
		(terminal.State != types.SupplierBatchCommandStateCompleted && terminal.State != types.SupplierBatchCommandStateFailed) {
		return false
	}
	if _, err := types.EncodeSupplierBatchCommandStateV1(terminal); err != nil {
		return false
	}
	oldResponse := running.Response
	newResponse := terminal.Response
	anchorsMatch := oldResponse.BatchDate != nil && newResponse.BatchDate != nil && *oldResponse.BatchDate == *newResponse.BatchDate &&
		oldResponse.RunID != nil && newResponse.RunID != nil && *oldResponse.RunID == *newResponse.RunID &&
		newResponse.RequestID == oldResponse.RequestID && newResponse.FenceToken == oldResponse.FenceToken
	if !anchorsMatch {
		return false
	}
	if terminal.State == types.SupplierBatchCommandStateCompleted {
		return newResponse.PublishedFenceToken == oldResponse.FenceToken
	}
	return newResponse.PublishedFenceToken >= oldResponse.PublishedFenceToken
}

func validSupplierBatchSchedulerSHA256Hex(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func supplierBatchSchedulerCommandAnchorQuery(db *gorm.DB, command SupplierAdminCommand) *gorm.DB {
	return db.Model(&SupplierAdminCommand{}).
		Where("id = ? AND actor_id = ? AND scope = ?", command.Id, 0, SupplierBatchSchedulerCommandScopeCatchUp).
		Where("idempotency_key = ? AND idempotency_key_digest = ?", command.IdempotencyKey, command.IdempotencyKeyDigest).
		Where("trusted_job_identity_digest = ? AND scheduler_request_digest = ? AND scheduler_scope_code = ? AND scheduler_slot = ?",
			*command.TrustedJobIdentityDigest, *command.SchedulerRequestDigest, *command.SchedulerScopeCode, command.SchedulerSlot).
		Where("payload_version = ? AND payload_digest = ?", command.PayloadVersion, command.PayloadDigest).
		Where("resource_type = ? AND resource_id = ? AND result_json = ?", command.ResourceType, command.ResourceId, command.ResultJson).
		Where("claim_token = ?", command.ClaimToken)
}

// AdoptSupplierBatchSchedulerClaim rotates ownership of a persisted claimed
// command without opening an inner transaction. Callers may pass an outer
// transaction and atomically adopt, select/acquire work, and store either a
// valid running state or the terminal no-work result before commit.
func AdoptSupplierBatchSchedulerClaim(ctx context.Context, db *gorm.DB, replay *SupplierBatchSchedulerCommandClaim, now time.Time) (*SupplierBatchSchedulerCommandClaim, error) {
	if db == nil || replay == nil || replay.Command.Id <= 0 || replay.Command.ClaimToken == "" || replay.Command.StatusJson == "" ||
		replay.State.State != types.SupplierBatchCommandStateClaimed || replay.State.Response != nil || now.Unix() <= 0 {
		return nil, ErrSupplierAdminCommandIncomplete
	}
	persistedState, err := types.ParseSupplierBatchCommandStateV1(replay.Command.StatusJson)
	if err != nil || persistedState.State != types.SupplierBatchCommandStateClaimed || persistedState.Response != nil {
		return nil, ErrSupplierAdminCommandIncomplete
	}
	claimToken, err := newSupplierAdminClaimToken()
	if err != nil {
		return nil, err
	}
	result := db.WithContext(ctx).Model(&SupplierAdminCommand{}).
		Where("id = ? AND claim_token = ? AND status_json = ?", replay.Command.Id, replay.Command.ClaimToken, replay.Command.StatusJson).
		UpdateColumns(map[string]any{"claim_token": claimToken, "updated_at": now.Unix()})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, ErrSupplierDailyBatchBusy
	}
	replay.Command.ClaimToken = claimToken
	replay.Command.UpdatedAt = now.Unix()
	replay.Claimed = true
	replay.Replayed = false
	return replay, nil
}

// TakeoverSupplierBatchSchedulerCommand reclaims only an expired running
// command under the same trusted identity and request anchor. Its CAS prevents
// parallel takeovers across application nodes.
func TakeoverSupplierBatchSchedulerCommand(ctx context.Context, db *gorm.DB, replay *SupplierBatchSchedulerCommandClaim, now time.Time, claimedTimeout ...time.Duration) (*SupplierBatchSchedulerCommandClaim, error) {
	if db == nil || replay == nil || replay.Command.Id <= 0 {
		return nil, ErrSupplierAdminCommandIncomplete
	}
	expired := false
	switch replay.State.State {
	case types.SupplierBatchCommandStateRunning:
		if replay.State.Response == nil || replay.State.Response.LockedUntil == nil {
			return nil, ErrSupplierAdminCommandIncomplete
		}
		lockedUntil, err := time.Parse(time.RFC3339, *replay.State.Response.LockedUntil)
		if err != nil {
			return nil, ErrSupplierAdminCommandIncomplete
		}
		expired = lockedUntil.Before(now)
	case types.SupplierBatchCommandStateClaimed:
		timeout := 5 * time.Minute
		if len(claimedTimeout) > 0 {
			timeout = claimedTimeout[0]
		}
		anchor := replay.Command.UpdatedAt
		if anchor <= 0 {
			anchor = replay.Command.CreatedAt
		}
		expired = timeout > 0 && anchor > 0 && time.Unix(anchor, 0).Add(timeout).Before(now)
	default:
		return nil, ErrSupplierAdminCommandIncomplete
	}
	if !expired {
		return nil, ErrSupplierDailyBatchBusy
	}
	claimToken, err := newSupplierAdminClaimToken()
	if err != nil {
		return nil, err
	}
	claimedState := types.SupplierBatchCommandStateV1{SchemaVersion: types.SupplierBatchCommandSchemaVersion, State: types.SupplierBatchCommandStateClaimed}
	statusJSON, err := types.EncodeSupplierBatchCommandStateV1(claimedState)
	if err != nil {
		return nil, err
	}
	result := db.WithContext(ctx).Model(&SupplierAdminCommand{}).
		Where("id = ? AND claim_token = ? AND status_json = ?", replay.Command.Id, replay.Command.ClaimToken, replay.Command.StatusJson).
		UpdateColumns(map[string]any{"claim_token": claimToken, "status_json": statusJSON, "updated_at": now.Unix()})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, ErrSupplierDailyBatchBusy
	}
	replay.Command.ClaimToken = claimToken
	replay.Command.StatusJson = statusJSON
	replay.State = claimedState
	replay.Claimed = true
	replay.Replayed = false
	return replay, nil
}

func validSupplierBatchSchedulerAuditSlot(slot string) bool {
	return slot == SupplierBatchSchedulerAuditSlotCurrent || slot == SupplierBatchSchedulerAuditSlotNext
}
