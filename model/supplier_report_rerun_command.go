package model

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	supplierDailyReportRerunScopePrefix     = "supplier_report.rerun/"
	supplierDailyReportRerunCommandResource = "supplier_report_rerun"
)

type SupplierDailyReportRerunCommandClaim struct {
	Command  SupplierAdminCommand
	State    types.SupplierBatchCommandStateV1
	Claimed  bool
	Replayed bool
}

func SupplierDailyReportRerunCommandScope(batchDate string) (string, error) {
	parsed, err := time.Parse("2006-01-02", batchDate)
	if err != nil || parsed.Format("2006-01-02") != batchDate {
		return "", ErrSupplierAdminIdempotencyKeyRequired
	}
	return supplierDailyReportRerunScopePrefix + batchDate, nil
}

func isSupplierDailyReportRerunScope(scope string) bool {
	if !strings.HasPrefix(scope, supplierDailyReportRerunScopePrefix) {
		return false
	}
	date := strings.TrimPrefix(scope, supplierDailyReportRerunScopePrefix)
	parsed, err := time.Parse("2006-01-02", date)
	return err == nil && parsed.Format("2006-01-02") == date
}

// ClaimSupplierDailyReportRerunCommand commits the Root actor-local command
// before the separate LOG_DB scan begins. It intentionally permits resource_id
// zero because the batch run is acquired only after this transaction commits.
func ClaimSupplierDailyReportRerunCommand(ctx context.Context, db *gorm.DB, actorID int, batchDate, idempotencyKey string, payload any) (*SupplierDailyReportRerunCommandClaim, error) {
	if db == nil || actorID <= 0 {
		return nil, ErrSupplierAdminIdempotencyKeyRequired
	}
	scope, err := SupplierDailyReportRerunCommandScope(batchDate)
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(idempotencyKey)
	if key == "" || len(key) > maxSupplierAdminIdempotencyKeyBytes {
		return nil, ErrSupplierAdminIdempotencyKeyRequired
	}
	payloadDigest, err := supplierAdminPayloadDigest(payload)
	if err != nil {
		return nil, err
	}
	claimedState := types.SupplierBatchCommandStateV1{SchemaVersion: types.SupplierBatchCommandSchemaVersion, State: types.SupplierBatchCommandStateClaimed}
	statusJSON, err := types.EncodeSupplierBatchCommandStateV1(claimedState)
	if err != nil {
		return nil, err
	}
	var claim *SupplierDailyReportRerunCommandClaim
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		claimToken, tokenErr := newSupplierAdminClaimToken()
		if tokenErr != nil {
			return tokenErr
		}
		keyDigest := supplierAdminIdempotencyKeyDigest(key)
		candidate := SupplierAdminCommand{
			ActorId: actorID, Scope: scope, IdempotencyKey: key, IdempotencyKeyDigest: keyDigest,
			PayloadVersion: supplierAdminCommandPayloadVersion, PayloadDigest: payloadDigest,
			ResourceType: supplierDailyReportRerunCommandResource, StatusJson: statusJSON, ClaimToken: claimToken,
		}
		if createErr := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "actor_id"}, {Name: "scope"}, {Name: "idempotency_key_digest"}}, DoNothing: true,
		}).Create(&candidate).Error; createErr != nil {
			return createErr
		}
		var persisted SupplierAdminCommand
		if queryErr := tx.Where("actor_id = ? AND scope = ? AND idempotency_key_digest = ?", actorID, scope, keyDigest).First(&persisted).Error; queryErr != nil {
			return queryErr
		}
		if persisted.IdempotencyKey != key || persisted.PayloadVersion != supplierAdminCommandPayloadVersion || persisted.PayloadDigest != payloadDigest || persisted.ResourceType != supplierDailyReportRerunCommandResource {
			return ErrSupplierAdminIdempotencyConflict
		}
		state, parseErr := types.ParseSupplierBatchCommandStateV1(persisted.StatusJson)
		if parseErr != nil {
			return ErrSupplierAdminCommandIncomplete
		}
		claimed := persisted.ClaimToken == claimToken
		claim = &SupplierDailyReportRerunCommandClaim{Command: persisted, State: state, Claimed: claimed, Replayed: !claimed}
		return nil
	})
	return claim, err
}

func GetSupplierDailyReportRerunCommand(ctx context.Context, db *gorm.DB, actorID int, batchDate, idempotencyKey string) (*SupplierDailyReportRerunCommandClaim, error) {
	scope, err := SupplierDailyReportRerunCommandScope(batchDate)
	key := strings.TrimSpace(idempotencyKey)
	if db == nil || actorID <= 0 || err != nil || key == "" || len(key) > maxSupplierAdminIdempotencyKeyBytes {
		return nil, ErrSupplierAdminIdempotencyKeyRequired
	}
	var command SupplierAdminCommand
	if err = db.WithContext(ctx).Where("actor_id = ? AND scope = ? AND idempotency_key_digest = ?", actorID, scope, supplierAdminIdempotencyKeyDigest(key)).First(&command).Error; err != nil {
		return nil, err
	}
	if command.IdempotencyKey != key {
		return nil, ErrSupplierAdminIdempotencyConflict
	}
	state, err := types.ParseSupplierBatchCommandStateV1(command.StatusJson)
	if err != nil {
		return nil, ErrSupplierAdminCommandIncomplete
	}
	return &SupplierDailyReportRerunCommandClaim{Command: command, State: state, Replayed: true}, nil
}

func StoreSupplierDailyReportRerunCommandState(ctx context.Context, db *gorm.DB, claim *SupplierDailyReportRerunCommandClaim, state types.SupplierBatchCommandStateV1) error {
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

// ReconcileSupplierDailyReportRerunCommandState closes a persisted running
// actor-local rerun command without requiring the original in-memory claim.
// The CAS binds the actor, date scope, request/payload identity, claim token,
// and exact running state. Identical terminal races replay; divergent races do
// not overwrite the winner.
func ReconcileSupplierDailyReportRerunCommandState(ctx context.Context, db *gorm.DB, replay *SupplierDailyReportRerunCommandClaim, terminal types.SupplierBatchCommandStateV1) (*SupplierDailyReportRerunCommandClaim, error) {
	if db == nil || !validSupplierDailyReportRerunRunningReplay(replay) || !validSupplierBatchReconciledTerminal(replay.State, terminal) {
		return nil, ErrSupplierAdminCommandIncomplete
	}
	terminalJSON, err := types.EncodeSupplierBatchCommandStateV1(terminal)
	if err != nil {
		return nil, err
	}
	updatedAt := time.Now().Unix()
	result := supplierDailyReportRerunCommandAnchorQuery(db.WithContext(ctx), replay.Command).
		Where("status_json = ?", replay.Command.StatusJson).
		UpdateColumns(map[string]any{"status_json": terminalJSON, "updated_at": updatedAt})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 1 {
		command := replay.Command
		command.StatusJson = terminalJSON
		command.UpdatedAt = updatedAt
		return &SupplierDailyReportRerunCommandClaim{Command: command, State: terminal}, nil
	}

	var persisted SupplierAdminCommand
	err = supplierDailyReportRerunCommandAnchorQuery(db.WithContext(ctx), replay.Command).First(&persisted).Error
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
	return &SupplierDailyReportRerunCommandClaim{Command: persisted, State: persistedState, Replayed: true}, nil
}

func validSupplierDailyReportRerunRunningReplay(replay *SupplierDailyReportRerunCommandClaim) bool {
	if replay == nil || replay.Claimed || !replay.Replayed {
		return false
	}
	command := replay.Command
	if command.Id <= 0 || command.ActorId <= 0 || !isSupplierDailyReportRerunScope(command.Scope) ||
		command.IdempotencyKey == "" || len(command.IdempotencyKey) > maxSupplierAdminIdempotencyKeyBytes || strings.TrimSpace(command.IdempotencyKey) != command.IdempotencyKey ||
		!equalDigest(command.IdempotencyKeyDigest, supplierAdminIdempotencyKeyDigest(command.IdempotencyKey)) ||
		command.TrustedJobIdentityDigest != nil || command.SchedulerRequestDigest != nil || command.SchedulerScopeCode != nil || command.SchedulerSlot != "" ||
		command.PayloadVersion != supplierAdminCommandPayloadVersion || !validSupplierBatchSchedulerSHA256Hex(command.PayloadDigest) ||
		command.ResourceType != supplierDailyReportRerunCommandResource || command.ResourceId != 0 || command.ResultJson != "" || len(command.ClaimToken) != 32 || command.StatusJson == "" {
		return false
	}
	canonicalRunning, err := types.EncodeSupplierBatchCommandStateV1(replay.State)
	if err != nil || canonicalRunning != command.StatusJson || replay.State.State != types.SupplierBatchCommandStateRunning || replay.State.Response == nil ||
		replay.State.Response.PublishedFenceToken >= replay.State.Response.FenceToken || replay.State.Response.RequestID != command.IdempotencyKey || replay.State.Response.BatchDate == nil {
		return false
	}
	scope, err := SupplierDailyReportRerunCommandScope(*replay.State.Response.BatchDate)
	return err == nil && command.Scope == scope
}

func supplierDailyReportRerunCommandAnchorQuery(db *gorm.DB, command SupplierAdminCommand) *gorm.DB {
	return db.Model(&SupplierAdminCommand{}).
		Where("id = ? AND actor_id = ? AND scope = ?", command.Id, command.ActorId, command.Scope).
		Where("idempotency_key = ? AND idempotency_key_digest = ?", command.IdempotencyKey, command.IdempotencyKeyDigest).
		Where("trusted_job_identity_digest IS NULL AND scheduler_request_digest IS NULL AND scheduler_scope_code IS NULL AND scheduler_slot = ?", "").
		Where("payload_version = ? AND payload_digest = ?", command.PayloadVersion, command.PayloadDigest).
		Where("resource_type = ? AND resource_id = ? AND result_json = ?", command.ResourceType, command.ResourceId, command.ResultJson).
		Where("claim_token = ?", command.ClaimToken)
}

func TakeoverSupplierDailyReportRerunCommand(ctx context.Context, db *gorm.DB, replay *SupplierDailyReportRerunCommandClaim, now time.Time, claimedTimeout ...time.Duration) (*SupplierDailyReportRerunCommandClaim, error) {
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
