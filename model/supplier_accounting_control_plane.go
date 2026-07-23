package model

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	SupplierAccountingCommandScopePrepare       = "supplier_accounting.prepare"
	SupplierAccountingCommandScopeArm           = "supplier_accounting.arm"
	SupplierAccountingCommandScopeActivate      = "supplier_accounting.activate"
	SupplierAccountingCommandScopeDisable       = "supplier_accounting.disable"
	SupplierAccountingCommandScopeDegrade       = "supplier_accounting.degrade"
	SupplierAccountingCommandScopeResolveGap    = "supplier_accounting.resolve_gap"
	SupplierAccountingCommandScopeReactivate    = "supplier_accounting.reactivate"
	SupplierAccountingCommandScopeMutationGate  = "supplier_accounting.mutation_gate"
	SupplierAccountingCommandScopeAdoptLegacy   = "supplier_accounting.adopt_legacy"
	SupplierAccountingCommandResourceActivation = "accounting_activation"
	SupplierAccountingCommandResourceMutation   = "accounting_mutation_gate"
)

var (
	ErrSupplierAccountingCommandInvalid     = errors.New("invalid supplier accounting control-plane command")
	ErrSupplierAccountingCoverageUnresolved = errors.New("supplier accounting coverage gaps remain unresolved")
	ErrSupplierAccountingLegacyUnavailable  = errors.New("supplier accounting legacy coverage is unavailable")
)

type SupplierAccountingControlCommand struct {
	ActorID              int    `json:"actor_id"`
	IdempotencyKey       string `json:"-"`
	ExpectedStateVersion int64  `json:"expected_state_version"`
	Reason               string `json:"reason"`
}

type SupplierAccountingPrepareInput struct {
	SupplierAccountingControlCommand
	AcceptedCapabilityVersions []int `json:"accepted_capability_versions"`
}

type SupplierAccountingArmInput struct {
	SupplierAccountingControlCommand
	CutoverAt                  int64 `json:"cutover_at"`
	AcceptedCapabilityVersions []int `json:"accepted_capability_versions"`
}

type SupplierAccountingDegradeInput struct {
	SupplierAccountingControlCommand
	StartAt                   int64    `json:"start_at"`
	ReasonCategory            string   `json:"reason_category"`
	ExpectedCapabilityVersion int64    `json:"expected_capability_version"`
	AffectedCapabilityVersion *int64   `json:"affected_capability_version"`
	AffectedOCIDigest         *string  `json:"affected_oci_digest"`
	AffectedBuildCommit       *string  `json:"affected_build_commit"`
	EvidenceRefs              []string `json:"evidence_refs"`
}

type SupplierAccountingResolveGapInput struct {
	SupplierAccountingControlCommand
	GapID              int64  `json:"gap_id"`
	ExpectedGapVersion int64  `json:"expected_gap_version"`
	EndAt              int64  `json:"end_at"`
	FinanceDisposition string `json:"finance_disposition"`
}

type SupplierAccountingMutationGateInput struct {
	SupplierAccountingControlCommand
	Enabled bool `json:"enabled"`
}

type SupplierAccountingLegacyAdoptionInput struct {
	SupplierAccountingControlCommand
	AcceptedCapabilityVersions []int `json:"accepted_capability_versions"`
}

type SupplierAccountingControlResult struct {
	Activation *SupplierAccountingActivationState `json:"activation,omitempty"`
	Mutation   *SupplierAccountingMutationState   `json:"mutation,omitempty"`
	Gap        *SupplierAccountingCoverageGap     `json:"gap,omitempty"`
	Replayed   bool                               `json:"-"`
}

type SupplierAccountingControlStatus struct {
	Activation      SupplierAccountingActivationState `json:"activation"`
	Mutation        SupplierAccountingMutationState   `json:"mutation"`
	LegacyCutoverAt *int64                            `json:"legacy_cutover_at"`
	UnresolvedGaps  []SupplierAccountingCoverageGap   `json:"unresolved_gaps"`
}

func GetSupplierAccountingControlStatus() (*SupplierAccountingControlStatus, error) {
	if DB == nil {
		return nil, ErrDatabase
	}
	var status SupplierAccountingControlStatus
	txOptions := &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead}
	if DB.Dialector.Name() == "sqlite" {
		txOptions = nil
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		activation, err := ReadSupplierAccountingActivationState(tx)
		if err != nil {
			return err
		}
		mutation, err := ReadSupplierAccountingMutationState(tx)
		if err != nil {
			return err
		}
		legacy, err := SupplierAccountingCoverageStart(context.Background(), tx)
		if err != nil {
			return err
		}
		var unresolved []SupplierAccountingCoverageGap
		if err := tx.Where("end_at IS NULL OR finance_disposition = ?", SupplierCoverageGapFinancePending).
			Order("start_at ASC, id ASC").Find(&unresolved).Error; err != nil {
			return err
		}
		status = SupplierAccountingControlStatus{Activation: activation, Mutation: mutation, UnresolvedGaps: unresolved}
		if legacy > 0 {
			status.LegacyCutoverAt = &legacy
		}
		return nil
	}, txOptions)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func PrepareSupplierAccounting(input SupplierAccountingPrepareInput) (*SupplierAccountingControlResult, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !validSupplierAccountingCapabilities(input.AcceptedCapabilityVersions) {
		return nil, ErrSupplierAccountingCommandInvalid
	}
	input.AcceptedCapabilityVersions = normalizeCapabilityVersions(input.AcceptedCapabilityVersions)
	return runSupplierAccountingActivationCommand(input.SupplierAccountingControlCommand, SupplierAccountingCommandScopePrepare, input, func(tx *gorm.DB, now int64) (SupplierAccountingControlResult, error) {
		actor := input.ActorID
		next := SupplierAccountingActivationState{
			Phase:                      SupplierAccountingActivationShadow,
			AcceptedCapabilityVersions: input.AcceptedCapabilityVersions,
			PreparedAt:                 &now,
			PreparedBy:                 &actor,
			Reason:                     input.Reason,
		}
		state, err := CASSupplierAccountingActivationState(tx, input.ExpectedStateVersion, next, now)
		return SupplierAccountingControlResult{Activation: &state}, err
	})
}

func ArmSupplierAccounting(input SupplierAccountingArmInput) (*SupplierAccountingControlResult, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !validSupplierAccountingCapabilities(input.AcceptedCapabilityVersions) {
		return nil, ErrSupplierAccountingCommandInvalid
	}
	input.AcceptedCapabilityVersions = normalizeCapabilityVersions(input.AcceptedCapabilityVersions)
	return runSupplierAccountingActivationCommand(input.SupplierAccountingControlCommand, SupplierAccountingCommandScopeArm, input, func(tx *gorm.DB, now int64) (SupplierAccountingControlResult, error) {
		current, err := ReadSupplierAccountingActivationState(tx)
		if err != nil {
			return SupplierAccountingControlResult{}, err
		}
		next := current
		next.Phase = SupplierAccountingActivationArmed
		next.CutoverAt = &input.CutoverAt
		next.AcceptedCapabilityVersions = input.AcceptedCapabilityVersions
		next.Reason = input.Reason
		state, err := CASSupplierAccountingActivationState(tx, input.ExpectedStateVersion, next, now)
		return SupplierAccountingControlResult{Activation: &state}, err
	})
}

func ActivateSupplierAccounting(input SupplierAccountingControlCommand) (*SupplierAccountingControlResult, error) {
	return transitionSupplierAccounting(input, SupplierAccountingCommandScopeActivate, SupplierAccountingActivationActive, func(current *SupplierAccountingActivationState, now int64) {
		current.ActivatedAt = &now
	})
}

func DisableSupplierAccountingBeforeCutover(input SupplierAccountingControlCommand) (*SupplierAccountingControlResult, error) {
	return transitionSupplierAccounting(input, SupplierAccountingCommandScopeDisable, SupplierAccountingActivationDisabled, func(current *SupplierAccountingActivationState, _ int64) {
		current.CutoverAt = nil
		current.AcceptedCapabilityVersions = nil
		current.PreparedAt = nil
		current.PreparedBy = nil
		current.ActivatedAt = nil
		current.DegradedAt = nil
	})
}

func DegradeSupplierAccounting(input SupplierAccountingDegradeInput) (*SupplierAccountingControlResult, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.ReasonCategory = strings.TrimSpace(input.ReasonCategory)
	input.FinanceNormalize()
	return runSupplierAccountingActivationCommand(input.SupplierAccountingControlCommand, SupplierAccountingCommandScopeDegrade, input, func(tx *gorm.DB, now int64) (SupplierAccountingControlResult, error) {
		current, err := ReadSupplierAccountingActivationState(tx)
		if err != nil {
			return SupplierAccountingControlResult{}, err
		}
		if current.CutoverAt == nil || input.StartAt < *current.CutoverAt || input.StartAt > now ||
			!supplierAccountingCapabilityAccepted(current.AcceptedCapabilityVersions, input.ExpectedCapabilityVersion) {
			return SupplierAccountingControlResult{}, ErrSupplierCoverageGapInvalid
		}
		next := current
		next.Phase = SupplierAccountingActivationDegraded
		next.DegradedAt = &now
		next.Reason = input.Reason
		state, err := CASSupplierAccountingActivationState(tx, input.ExpectedStateVersion, next, now)
		if err != nil {
			return SupplierAccountingControlResult{}, err
		}
		gap, err := OpenSupplierAccountingCoverageGap(tx, OpenSupplierAccountingCoverageGapInput{
			StartAt: input.StartAt, ReasonCategory: input.ReasonCategory, ReasonText: input.Reason,
			ExpectedCapabilityVersion: input.ExpectedCapabilityVersion, AffectedCapabilityVersion: input.AffectedCapabilityVersion,
			AffectedOCIDigest: input.AffectedOCIDigest, AffectedBuildCommit: input.AffectedBuildCommit,
			ActivationStateVersionBefore: input.ExpectedStateVersion, ActivationStateVersionAfter: state.StateVersion,
			OpenCommandID: supplierAccountingGapCommandID(SupplierAccountingCommandScopeDegrade, input.ActorID, input.IdempotencyKey),
			OpenedBy:      input.ActorID, EvidenceRefs: input.EvidenceRefs,
		})
		return SupplierAccountingControlResult{Activation: &state, Gap: gap}, err
	})
}

func ResolveSupplierAccountingGap(input SupplierAccountingResolveGapInput) (*SupplierAccountingControlResult, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.FinanceDisposition = strings.TrimSpace(input.FinanceDisposition)
	if input.FinanceDisposition == SupplierCoverageGapFinancePending {
		return nil, ErrSupplierAccountingCommandInvalid
	}
	return runSupplierAccountingActivationCommand(input.SupplierAccountingControlCommand, SupplierAccountingCommandScopeResolveGap, input, func(tx *gorm.DB, now int64) (SupplierAccountingControlResult, error) {
		current, err := ReadSupplierAccountingActivationState(tx)
		if err != nil {
			return SupplierAccountingControlResult{}, err
		}
		if current.Phase != SupplierAccountingActivationDegraded {
			return SupplierAccountingControlResult{}, ErrSupplierAccountingTransition
		}
		if input.EndAt > now {
			return SupplierAccountingControlResult{}, ErrSupplierCoverageGapInvalid
		}
		gap, err := CloseSupplierAccountingCoverageGap(tx, CloseSupplierAccountingCoverageGapInput{
			ID: input.GapID, EndAt: input.EndAt,
			CloseCommandID: supplierAccountingGapCommandID(SupplierAccountingCommandScopeResolveGap, input.ActorID, input.IdempotencyKey),
			ClosedBy:       input.ActorID, FinanceDisposition: input.FinanceDisposition, ExpectedVersion: input.ExpectedGapVersion,
		})
		if err != nil {
			return SupplierAccountingControlResult{}, err
		}
		next := current
		next.Reason = input.Reason
		state, err := CASSupplierAccountingActivationState(tx, input.ExpectedStateVersion, next, now)
		return SupplierAccountingControlResult{Activation: &state, Gap: gap}, err
	})
}

func ReactivateSupplierAccounting(input SupplierAccountingControlCommand) (*SupplierAccountingControlResult, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	return runSupplierAccountingActivationCommand(input, SupplierAccountingCommandScopeReactivate, input, func(tx *gorm.DB, now int64) (SupplierAccountingControlResult, error) {
		var count int64
		if err := tx.Model(&SupplierAccountingCoverageGap{}).
			Where("end_at IS NULL OR finance_disposition = ?", SupplierCoverageGapFinancePending).Count(&count).Error; err != nil {
			return SupplierAccountingControlResult{}, err
		}
		if count != 0 {
			return SupplierAccountingControlResult{}, ErrSupplierAccountingCoverageUnresolved
		}
		current, err := ReadSupplierAccountingActivationState(tx)
		if err != nil {
			return SupplierAccountingControlResult{}, err
		}
		next := current
		next.Phase = SupplierAccountingActivationActive
		next.DegradedAt = nil
		next.Reason = input.Reason
		state, err := CASSupplierAccountingActivationState(tx, input.ExpectedStateVersion, next, now)
		return SupplierAccountingControlResult{Activation: &state}, err
	})
}

func ToggleSupplierAccountingMutationGate(input SupplierAccountingMutationGateInput) (*SupplierAccountingControlResult, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if err := validateSupplierAccountingControlCommand(input.SupplierAccountingControlCommand); err != nil {
		return nil, err
	}
	var result SupplierAccountingControlResult
	err := DB.Transaction(func(tx *gorm.DB) error {
		if input.Enabled {
			if err := ValidateSupplierAdminCommandLedgerFinalized(tx); err != nil {
				return err
			}
		}
		claim, err := ClaimSupplierAdminCommandTx(tx, input.ActorID, SupplierAccountingCommandScopeMutationGate, input.IdempotencyKey, input, SupplierAccountingCommandResourceMutation)
		if err != nil {
			return err
		}
		if claim.Replayed {
			if err := claim.DecodeResult(&result); err != nil {
				return err
			}
			result.Replayed = true
			return nil
		}
		now, err := getSupplierDBTimestamp(tx)
		if err != nil {
			return err
		}
		state, err := CASSupplierAccountingMutationState(tx, input.ExpectedStateVersion, input.Enabled, input.ActorID, input.Reason, now)
		if err != nil {
			return err
		}
		result.Mutation = &state
		return CompleteSupplierAdminCommandTx(tx, claim, claim.Command.Id, result)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// AdoptLegacySupplierAccounting is the only migration path from the retired
// coverage_start_at option. It preserves the historical cutover exactly and
// creates an active version-one document through an explicit audited command.
func AdoptLegacySupplierAccounting(input SupplierAccountingLegacyAdoptionInput) (*SupplierAccountingControlResult, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !validSupplierAccountingCapabilities(input.AcceptedCapabilityVersions) {
		return nil, ErrSupplierAccountingCommandInvalid
	}
	input.AcceptedCapabilityVersions = normalizeCapabilityVersions(input.AcceptedCapabilityVersions)
	if err := validateSupplierAccountingControlCommand(input.SupplierAccountingControlCommand); err != nil {
		return nil, err
	}
	var result SupplierAccountingControlResult
	err := DB.Transaction(func(tx *gorm.DB) error {
		claim, err := ClaimSupplierAdminCommandTx(tx, input.ActorID, SupplierAccountingCommandScopeAdoptLegacy, input.IdempotencyKey, input, SupplierAccountingCommandResourceActivation)
		if err != nil {
			return err
		}
		if claim.Replayed {
			if err := claim.DecodeResult(&result); err != nil {
				return err
			}
			result.Replayed = true
			return nil
		}
		now, err := getSupplierDBTimestamp(tx)
		if err != nil {
			return err
		}
		legacyCutover, err := SupplierAccountingCoverageStart(context.Background(), tx)
		if err != nil {
			return err
		}
		if legacyCutover <= 0 || legacyCutover > now || input.ExpectedStateVersion != 0 || len(input.AcceptedCapabilityVersions) == 0 {
			return ErrSupplierAccountingLegacyUnavailable
		}
		option, found, err := readSupplierAccountingOption(tx, SupplierAccountingActivationOptionKey)
		if err != nil {
			return err
		}
		if found {
			return ErrSupplierAccountingOptionConflict
		}
		actor := input.ActorID
		state := SupplierAccountingActivationState{
			SchemaVersion: supplierAccountingOptionSchemaVersion, StateVersion: 1, Phase: SupplierAccountingActivationActive,
			CutoverAt: &legacyCutover, AcceptedCapabilityVersions: input.AcceptedCapabilityVersions,
			PreparedAt: &legacyCutover, PreparedBy: &actor, ActivatedAt: &legacyCutover, Reason: input.Reason,
		}
		if err := validateSupplierAccountingActivationState(state); err != nil {
			return err
		}
		encoded, err := common.Marshal(state)
		if err != nil {
			return err
		}
		if err := casSupplierAccountingOption(tx, SupplierAccountingActivationOptionKey, false, option.Value, string(encoded)); err != nil {
			return err
		}
		result.Activation = &state
		return CompleteSupplierAdminCommandTx(tx, claim, claim.Command.Id, result)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func transitionSupplierAccounting(input SupplierAccountingControlCommand, scope string, phase SupplierAccountingActivationPhase, mutate func(*SupplierAccountingActivationState, int64)) (*SupplierAccountingControlResult, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	return runSupplierAccountingActivationCommand(input, scope, input, func(tx *gorm.DB, now int64) (SupplierAccountingControlResult, error) {
		current, err := ReadSupplierAccountingActivationState(tx)
		if err != nil {
			return SupplierAccountingControlResult{}, err
		}
		next := current
		next.Phase = phase
		next.Reason = input.Reason
		mutate(&next, now)
		state, err := CASSupplierAccountingActivationState(tx, input.ExpectedStateVersion, next, now)
		return SupplierAccountingControlResult{Activation: &state}, err
	})
}

func runSupplierAccountingActivationCommand(command SupplierAccountingControlCommand, scope string, payload any, apply func(*gorm.DB, int64) (SupplierAccountingControlResult, error)) (*SupplierAccountingControlResult, error) {
	if err := validateSupplierAccountingControlCommand(command); err != nil {
		return nil, err
	}
	if DB == nil {
		return nil, ErrDatabase
	}
	var result SupplierAccountingControlResult
	err := DB.Transaction(func(tx *gorm.DB) error {
		claim, err := ClaimSupplierAdminCommandTx(tx, command.ActorID, scope, command.IdempotencyKey, payload, SupplierAccountingCommandResourceActivation)
		if err != nil {
			return err
		}
		if claim.Replayed {
			if err := claim.DecodeResult(&result); err != nil {
				return err
			}
			result.Replayed = true
			return nil
		}
		now, err := getSupplierDBTimestamp(tx)
		if err != nil {
			return err
		}
		result, err = apply(tx, now)
		if err != nil {
			return err
		}
		return CompleteSupplierAdminCommandTx(tx, claim, claim.Command.Id, result)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func validateSupplierAccountingControlCommand(command SupplierAccountingControlCommand) error {
	if DB == nil {
		return ErrDatabase
	}
	if command.ActorID <= 0 || command.ExpectedStateVersion < 0 || strings.TrimSpace(command.IdempotencyKey) == "" || strings.TrimSpace(command.Reason) == "" || len(strings.TrimSpace(command.Reason)) > maxSupplierAccountingOptionReasonBytes {
		return ErrSupplierAccountingCommandInvalid
	}
	return nil
}

func supplierAccountingGapCommandID(scope string, actorID int, idempotencyKey string) string {
	digest, _ := supplierAdminPayloadDigest(struct {
		Scope string `json:"scope"`
		Actor int    `json:"actor"`
		Key   string `json:"key"`
	}{Scope: scope, Actor: actorID, Key: strings.TrimSpace(idempotencyKey)})
	return digest
}

func (input *SupplierAccountingDegradeInput) FinanceNormalize() {
	input.AffectedOCIDigest = cloneTrimmedString(input.AffectedOCIDigest)
	input.AffectedBuildCommit = cloneTrimmedString(input.AffectedBuildCommit)
	input.EvidenceRefs = cloneStrings(input.EvidenceRefs)
}

func validSupplierAccountingCapabilities(versions []int) bool {
	if len(versions) == 0 {
		return false
	}
	seen := make(map[int]struct{}, len(versions))
	for _, version := range versions {
		if version <= 0 {
			return false
		}
		if _, exists := seen[version]; exists {
			return false
		}
		seen[version] = struct{}{}
	}
	return true
}

func supplierAccountingCapabilityAccepted(versions []int, expected int64) bool {
	for _, version := range versions {
		if int64(version) == expected {
			return true
		}
	}
	return false
}
