package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SupplierAccountingActivationOptionKey = "supplier_accounting_v1.activation"
	SupplierAccountingMutationOptionKey   = "supplier_accounting_v1.mutations_enabled"

	supplierAccountingOptionSchemaVersion  = 1
	maxSupplierAccountingOptionReasonBytes = 1024
)

var (
	ErrSupplierAccountingReservedOption     = errors.New("supplier accounting reserved option cannot be changed through generic option APIs")
	ErrSupplierAccountingOptionMalformed    = errors.New("supplier accounting option is malformed")
	ErrSupplierAccountingOptionConflict     = errors.New("supplier accounting option state version conflict")
	ErrSupplierAccountingTransition         = errors.New("supplier accounting activation transition is not allowed")
	ErrSupplierAccountingMutationTransition = errors.New("supplier accounting mutation-gate transition is not allowed")
)

type SupplierAccountingActivationPhase string

const (
	SupplierAccountingActivationDisabled SupplierAccountingActivationPhase = "disabled"
	SupplierAccountingActivationShadow   SupplierAccountingActivationPhase = "shadow"
	SupplierAccountingActivationArmed    SupplierAccountingActivationPhase = "armed"
	SupplierAccountingActivationActive   SupplierAccountingActivationPhase = "active"
	SupplierAccountingActivationDegraded SupplierAccountingActivationPhase = "degraded"
	SupplierAccountingActivationRetired  SupplierAccountingActivationPhase = "retired"
)

type SupplierAccountingActivationState struct {
	SchemaVersion              int                               `json:"schema_version"`
	StateVersion               int64                             `json:"state_version"`
	Phase                      SupplierAccountingActivationPhase `json:"phase"`
	CutoverAt                  *int64                            `json:"cutover_at"`
	AcceptedCapabilityVersions []int                             `json:"accepted_capability_versions"`
	PreparedAt                 *int64                            `json:"prepared_at"`
	PreparedBy                 *int                              `json:"prepared_by"`
	ActivatedAt                *int64                            `json:"activated_at"`
	DegradedAt                 *int64                            `json:"degraded_at"`
	Reason                     string                            `json:"reason"`
}

type SupplierAccountingMutationState struct {
	SchemaVersion int    `json:"schema_version"`
	StateVersion  int64  `json:"state_version"`
	Enabled       bool   `json:"enabled"`
	UpdatedBy     *int   `json:"updated_by,omitempty"`
	UpdatedAt     *int64 `json:"updated_at,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

func IsSupplierAccountingReservedOptionKey(key string) bool {
	return key == SupplierAccountingActivationOptionKey ||
		key == SupplierAccountingMutationOptionKey ||
		key == SupplierAccountingCoverageStartOptionKey
}

func SyntheticSupplierAccountingActivationState() SupplierAccountingActivationState {
	return SupplierAccountingActivationState{
		SchemaVersion: supplierAccountingOptionSchemaVersion,
		StateVersion:  0,
		Phase:         SupplierAccountingActivationDisabled,
	}
}

func SyntheticSupplierAccountingMutationState() SupplierAccountingMutationState {
	return SupplierAccountingMutationState{
		SchemaVersion: supplierAccountingOptionSchemaVersion,
		StateVersion:  0,
		Enabled:       false,
	}
}

func ParseSupplierAccountingActivationState(raw string) (SupplierAccountingActivationState, error) {
	allowed := map[string]struct{}{
		"schema_version": {}, "state_version": {}, "phase": {}, "cutover_at": {},
		"accepted_capability_versions": {}, "prepared_at": {}, "prepared_by": {},
		"activated_at": {}, "degraded_at": {}, "reason": {},
	}
	fields, err := strictSupplierAccountingJSONFields(raw, allowed)
	if err != nil {
		return SupplierAccountingActivationState{}, err
	}
	for _, required := range []string{"schema_version", "state_version", "phase", "cutover_at", "accepted_capability_versions", "prepared_at", "prepared_by", "activated_at", "degraded_at", "reason"} {
		if _, ok := fields[required]; !ok {
			return SupplierAccountingActivationState{}, malformedSupplierAccountingOption(fmt.Errorf("missing field %q", required))
		}
	}
	var state SupplierAccountingActivationState
	if err := common.UnmarshalJsonStr(raw, &state); err != nil {
		return SupplierAccountingActivationState{}, malformedSupplierAccountingOption(err)
	}
	if err := validateSupplierAccountingActivationState(state); err != nil {
		return SupplierAccountingActivationState{}, err
	}
	state.AcceptedCapabilityVersions = normalizeCapabilityVersions(state.AcceptedCapabilityVersions)
	state.Reason = strings.TrimSpace(state.Reason)
	return state, nil
}

func ParseSupplierAccountingMutationState(raw string) (SupplierAccountingMutationState, error) {
	allowed := map[string]struct{}{
		"schema_version": {}, "state_version": {}, "enabled": {}, "updated_by": {},
		"updated_at": {}, "reason": {},
	}
	fields, err := strictSupplierAccountingJSONFields(raw, allowed)
	if err != nil {
		return SupplierAccountingMutationState{}, err
	}
	for _, required := range []string{"schema_version", "state_version", "enabled"} {
		if _, ok := fields[required]; !ok {
			return SupplierAccountingMutationState{}, malformedSupplierAccountingOption(fmt.Errorf("missing field %q", required))
		}
	}
	var state SupplierAccountingMutationState
	if err := common.UnmarshalJsonStr(raw, &state); err != nil {
		return SupplierAccountingMutationState{}, malformedSupplierAccountingOption(err)
	}
	if err := validateSupplierAccountingMutationState(state); err != nil {
		return SupplierAccountingMutationState{}, err
	}
	state.Reason = strings.TrimSpace(state.Reason)
	return state, nil
}

func ReadSupplierAccountingActivationState(db *gorm.DB) (SupplierAccountingActivationState, error) {
	option, found, err := readSupplierAccountingOption(db, SupplierAccountingActivationOptionKey)
	if err != nil {
		return SupplierAccountingActivationState{}, err
	}
	if !found {
		return SyntheticSupplierAccountingActivationState(), nil
	}
	return ParseSupplierAccountingActivationState(option.Value)
}

func ReadSupplierAccountingMutationState(db *gorm.DB) (SupplierAccountingMutationState, error) {
	option, found, err := readSupplierAccountingOption(db, SupplierAccountingMutationOptionKey)
	if err != nil {
		return SupplierAccountingMutationState{}, err
	}
	if !found {
		return SyntheticSupplierAccountingMutationState(), nil
	}
	return ParseSupplierAccountingMutationState(option.Value)
}

// CASSupplierAccountingActivationState changes only the reserved activation
// document. Pass a transaction to compose this CAS with command and gap writes.
// StateVersion and SchemaVersion are assigned here and cannot be supplied by
// callers. The compare uses both the decoded version and the exact prior value,
// so stale callers fail even after a phase ABA cycle.
func CASSupplierAccountingActivationState(db *gorm.DB, expectedStateVersion int64, desired SupplierAccountingActivationState, now int64) (SupplierAccountingActivationState, error) {
	if db == nil || expectedStateVersion < 0 || now <= 0 {
		return SupplierAccountingActivationState{}, malformedSupplierAccountingOption(errors.New("invalid CAS arguments"))
	}
	currentOption, found, err := readSupplierAccountingOption(db, SupplierAccountingActivationOptionKey)
	if err != nil {
		return SupplierAccountingActivationState{}, err
	}
	current := SyntheticSupplierAccountingActivationState()
	if found {
		current, err = ParseSupplierAccountingActivationState(currentOption.Value)
		if err != nil {
			return SupplierAccountingActivationState{}, err
		}
	}
	if current.StateVersion != expectedStateVersion {
		return SupplierAccountingActivationState{}, ErrSupplierAccountingOptionConflict
	}

	desired.SchemaVersion = supplierAccountingOptionSchemaVersion
	desired.StateVersion = expectedStateVersion + 1
	desired.Reason = strings.TrimSpace(desired.Reason)
	desired.AcceptedCapabilityVersions = normalizeCapabilityVersions(desired.AcceptedCapabilityVersions)
	if err := ValidateSupplierAccountingActivationTransition(current, desired, now); err != nil {
		return SupplierAccountingActivationState{}, err
	}
	encoded, err := common.Marshal(desired)
	if err != nil {
		return SupplierAccountingActivationState{}, err
	}
	if err := casSupplierAccountingOption(db, SupplierAccountingActivationOptionKey, found, currentOption.Value, string(encoded)); err != nil {
		return SupplierAccountingActivationState{}, err
	}
	return desired, nil
}

// CASSupplierAccountingMutationState toggles the strongly consistent mutation
// gate. Missing state is synthetic version zero and is inserted only by a
// successful expected-version-zero transition.
func CASSupplierAccountingMutationState(db *gorm.DB, expectedStateVersion int64, enabled bool, actor int, reason string, now int64) (SupplierAccountingMutationState, error) {
	if db == nil || expectedStateVersion < 0 || actor <= 0 || now <= 0 {
		return SupplierAccountingMutationState{}, malformedSupplierAccountingOption(errors.New("invalid CAS arguments"))
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > maxSupplierAccountingOptionReasonBytes {
		return SupplierAccountingMutationState{}, malformedSupplierAccountingOption(errors.New("invalid reason"))
	}
	currentOption, found, err := readSupplierAccountingOption(db, SupplierAccountingMutationOptionKey)
	if err != nil {
		return SupplierAccountingMutationState{}, err
	}
	current := SyntheticSupplierAccountingMutationState()
	if found {
		current, err = ParseSupplierAccountingMutationState(currentOption.Value)
		if err != nil {
			return SupplierAccountingMutationState{}, err
		}
	}
	if current.StateVersion != expectedStateVersion {
		return SupplierAccountingMutationState{}, ErrSupplierAccountingOptionConflict
	}
	if found && current.Enabled == enabled {
		return SupplierAccountingMutationState{}, ErrSupplierAccountingMutationTransition
	}
	next := SupplierAccountingMutationState{
		SchemaVersion: supplierAccountingOptionSchemaVersion,
		StateVersion:  expectedStateVersion + 1,
		Enabled:       enabled,
		UpdatedBy:     &actor,
		UpdatedAt:     &now,
		Reason:        reason,
	}
	if err := validateSupplierAccountingMutationState(next); err != nil {
		return SupplierAccountingMutationState{}, err
	}
	encoded, err := common.Marshal(next)
	if err != nil {
		return SupplierAccountingMutationState{}, err
	}
	if err := casSupplierAccountingOption(db, SupplierAccountingMutationOptionKey, found, currentOption.Value, string(encoded)); err != nil {
		return SupplierAccountingMutationState{}, err
	}
	return next, nil
}

func ValidateSupplierAccountingActivationTransition(current, next SupplierAccountingActivationState, now int64) error {
	if now <= 0 || next.StateVersion != current.StateVersion+1 {
		return ErrSupplierAccountingTransition
	}
	if current.StateVersion == 0 {
		if current.SchemaVersion != supplierAccountingOptionSchemaVersion || current.Phase != SupplierAccountingActivationDisabled ||
			current.CutoverAt != nil || len(current.AcceptedCapabilityVersions) != 0 || current.PreparedAt != nil ||
			current.PreparedBy != nil || current.ActivatedAt != nil || current.DegradedAt != nil || current.Reason != "" {
			return malformedSupplierAccountingOption(errors.New("invalid synthetic activation state"))
		}
	} else if err := validateSupplierAccountingActivationState(current); err != nil {
		return err
	}
	if err := validateSupplierAccountingActivationState(next); err != nil {
		return err
	}
	allowed := false
	switch current.Phase {
	case SupplierAccountingActivationDisabled:
		allowed = next.Phase == SupplierAccountingActivationShadow
	case SupplierAccountingActivationShadow:
		allowed = next.Phase == SupplierAccountingActivationArmed || next.Phase == SupplierAccountingActivationDisabled
	case SupplierAccountingActivationArmed:
		allowed = next.Phase == SupplierAccountingActivationActive || next.Phase == SupplierAccountingActivationDisabled
	case SupplierAccountingActivationActive:
		allowed = next.Phase == SupplierAccountingActivationDegraded || next.Phase == SupplierAccountingActivationRetired
	case SupplierAccountingActivationDegraded:
		allowed = next.Phase == SupplierAccountingActivationDegraded || next.Phase == SupplierAccountingActivationActive || next.Phase == SupplierAccountingActivationRetired
	}
	if !allowed {
		return ErrSupplierAccountingTransition
	}
	if next.Phase == SupplierAccountingActivationArmed && (next.CutoverAt == nil || *next.CutoverAt <= now) {
		return fmt.Errorf("%w: armed cutover must be future-dated", ErrSupplierAccountingTransition)
	}
	if (current.Phase == SupplierAccountingActivationShadow || current.Phase == SupplierAccountingActivationArmed) && next.Phase == SupplierAccountingActivationDisabled && current.CutoverAt != nil && *current.CutoverAt <= now {
		return fmt.Errorf("%w: rollback after cutover", ErrSupplierAccountingTransition)
	}
	if current.Phase == SupplierAccountingActivationArmed && next.Phase == SupplierAccountingActivationActive {
		if current.CutoverAt == nil || *current.CutoverAt > now || next.CutoverAt == nil || *next.CutoverAt != *current.CutoverAt {
			return fmt.Errorf("%w: activation cutover mismatch", ErrSupplierAccountingTransition)
		}
	}
	if (current.Phase == SupplierAccountingActivationActive || current.Phase == SupplierAccountingActivationDegraded) && (next.Phase == SupplierAccountingActivationActive || next.Phase == SupplierAccountingActivationDegraded || next.Phase == SupplierAccountingActivationRetired) {
		if current.CutoverAt == nil || next.CutoverAt == nil || *current.CutoverAt != *next.CutoverAt {
			return fmt.Errorf("%w: active cutover is immutable", ErrSupplierAccountingTransition)
		}
	}
	return nil
}

func validateSupplierAccountingActivationState(state SupplierAccountingActivationState) error {
	if state.SchemaVersion != supplierAccountingOptionSchemaVersion || state.StateVersion <= 0 {
		return malformedSupplierAccountingOption(errors.New("unsupported schema or invalid state version"))
	}
	state.Reason = strings.TrimSpace(state.Reason)
	if state.Reason == "" || len(state.Reason) > maxSupplierAccountingOptionReasonBytes {
		return malformedSupplierAccountingOption(errors.New("invalid reason"))
	}
	versions := normalizeCapabilityVersions(state.AcceptedCapabilityVersions)
	if len(versions) != len(state.AcceptedCapabilityVersions) {
		return malformedSupplierAccountingOption(errors.New("capability versions must be positive and unique"))
	}
	for _, version := range versions {
		if version <= 0 {
			return malformedSupplierAccountingOption(errors.New("invalid capability version"))
		}
	}
	positiveTime := func(value *int64) bool { return value != nil && *value > 0 }
	positiveActor := func(value *int) bool { return value != nil && *value > 0 }
	for _, value := range []*int64{state.CutoverAt, state.PreparedAt, state.ActivatedAt, state.DegradedAt} {
		if value != nil && *value <= 0 {
			return malformedSupplierAccountingOption(errors.New("invalid timestamp"))
		}
	}
	if state.PreparedBy != nil && *state.PreparedBy <= 0 {
		return malformedSupplierAccountingOption(errors.New("invalid prepared_by"))
	}
	switch state.Phase {
	case SupplierAccountingActivationDisabled:
		if state.CutoverAt != nil || len(versions) != 0 || state.PreparedAt != nil || state.PreparedBy != nil || state.ActivatedAt != nil || state.DegradedAt != nil {
			return malformedSupplierAccountingOption(errors.New("disabled state contains activation fields"))
		}
	case SupplierAccountingActivationShadow:
		if len(versions) == 0 || !positiveTime(state.PreparedAt) || !positiveActor(state.PreparedBy) || state.CutoverAt != nil || state.ActivatedAt != nil || state.DegradedAt != nil {
			return malformedSupplierAccountingOption(errors.New("invalid shadow state"))
		}
	case SupplierAccountingActivationArmed:
		if len(versions) == 0 || !positiveTime(state.PreparedAt) || !positiveActor(state.PreparedBy) || !positiveTime(state.CutoverAt) || state.ActivatedAt != nil || state.DegradedAt != nil {
			return malformedSupplierAccountingOption(errors.New("invalid armed state"))
		}
	case SupplierAccountingActivationActive:
		if len(versions) == 0 || !positiveTime(state.PreparedAt) || !positiveActor(state.PreparedBy) || !positiveTime(state.CutoverAt) || !positiveTime(state.ActivatedAt) || state.DegradedAt != nil || *state.ActivatedAt < *state.CutoverAt {
			return malformedSupplierAccountingOption(errors.New("invalid active state"))
		}
	case SupplierAccountingActivationDegraded:
		if len(versions) == 0 || !positiveTime(state.PreparedAt) || !positiveActor(state.PreparedBy) || !positiveTime(state.CutoverAt) || !positiveTime(state.ActivatedAt) || !positiveTime(state.DegradedAt) || *state.ActivatedAt < *state.CutoverAt || *state.DegradedAt < *state.ActivatedAt {
			return malformedSupplierAccountingOption(errors.New("invalid degraded state"))
		}
	case SupplierAccountingActivationRetired:
		if len(versions) == 0 || !positiveTime(state.PreparedAt) || !positiveActor(state.PreparedBy) || !positiveTime(state.CutoverAt) || !positiveTime(state.ActivatedAt) {
			return malformedSupplierAccountingOption(errors.New("invalid retired state"))
		}
	default:
		return malformedSupplierAccountingOption(errors.New("unknown phase"))
	}
	return nil
}

func validateSupplierAccountingMutationState(state SupplierAccountingMutationState) error {
	if state.SchemaVersion != supplierAccountingOptionSchemaVersion || state.StateVersion <= 0 {
		return malformedSupplierAccountingOption(errors.New("unsupported schema or invalid state version"))
	}
	if state.UpdatedBy != nil && *state.UpdatedBy <= 0 {
		return malformedSupplierAccountingOption(errors.New("invalid updated_by"))
	}
	if state.UpdatedAt != nil && *state.UpdatedAt <= 0 {
		return malformedSupplierAccountingOption(errors.New("invalid updated_at"))
	}
	reason := strings.TrimSpace(state.Reason)
	if len(reason) > maxSupplierAccountingOptionReasonBytes {
		return malformedSupplierAccountingOption(errors.New("invalid reason"))
	}
	return nil
}

func strictSupplierAccountingJSONFields(raw string, allowed map[string]struct{}) (map[string]json.RawMessage, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, malformedSupplierAccountingOption(errors.New("empty document"))
	}
	fields, err := common.UnmarshalJsonObjectStrict(raw, allowed)
	if err != nil {
		return nil, malformedSupplierAccountingOption(err)
	}
	return fields, nil
}

func normalizeCapabilityVersions(versions []int) []int {
	if len(versions) == 0 {
		return []int{}
	}
	seen := make(map[int]struct{}, len(versions))
	normalized := make([]int, 0, len(versions))
	for _, version := range versions {
		if version <= 0 {
			continue
		}
		if _, exists := seen[version]; exists {
			continue
		}
		seen[version] = struct{}{}
		normalized = append(normalized, version)
	}
	sort.Ints(normalized)
	return normalized
}

func readSupplierAccountingOption(db *gorm.DB, key string) (Option, bool, error) {
	if db == nil {
		return Option{}, false, ErrDatabase
	}
	var option Option
	err := db.Where(&Option{Key: key}).Take(&option).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Option{}, false, nil
	}
	return option, err == nil, err
}

func casSupplierAccountingOption(db *gorm.DB, key string, found bool, previousValue, nextValue string) error {
	if !found {
		candidate := Option{Key: key, Value: nextValue}
		result := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoNothing: true,
		}).Create(&candidate)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrSupplierAccountingOptionConflict
		}
		return nil
	}
	result := db.Model(&Option{}).Where(map[string]any{"key": key, "value": previousValue}).UpdateColumn("value", nextValue)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrSupplierAccountingOptionConflict
	}
	return nil
}

func malformedSupplierAccountingOption(cause error) error {
	return fmt.Errorf("%w: %v", ErrSupplierAccountingOptionMalformed, cause)
}
