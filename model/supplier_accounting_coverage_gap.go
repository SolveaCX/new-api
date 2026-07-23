package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SupplierCoverageGapReasonLogWriteFailure            = "log_write_failure"
	SupplierCoverageGapReasonLoggingDisabled            = "logging_disabled"
	SupplierCoverageGapReasonProducerCapabilityMismatch = "producer_capability_mismatch"
	SupplierCoverageGapReasonEmergencyRollback          = "emergency_rollback"
	SupplierCoverageGapReasonOperatorDeclared           = "operator_declared"

	SupplierCoverageGapFinancePending      = "pending"
	SupplierCoverageGapFinanceNoImpact     = "no_impact"
	SupplierCoverageGapFinanceReconciled   = "reconciled"
	SupplierCoverageGapFinanceAcceptedLoss = "accepted_loss"

	SupplierCoverageGapMaxReasonTextBytes   = 1024
	SupplierCoverageGapMaxEvidenceRefsBytes = 4096
	SupplierCoverageGapMaxEvidenceRefCount  = 16
	SupplierCoverageGapMaxEvidenceRefBytes  = 256
	SupplierCoverageGapMaxCommandIDBytes    = 128
	SupplierCoverageGapMaxOCIDigestBytes    = 71
	SupplierCoverageGapMaxBuildCommitBytes  = 64
	SupplierCoverageGapInitialRecordVersion = int64(1)
)

var (
	ErrSupplierCoverageGapInvalid             = errors.New("invalid supplier accounting coverage gap")
	ErrSupplierCoverageGapIdempotencyConflict = errors.New("supplier accounting coverage gap idempotency conflict")
	ErrSupplierCoverageGapNotFound            = errors.New("supplier accounting coverage gap not found")
	ErrSupplierCoverageGapCASConflict         = errors.New("supplier accounting coverage gap version conflict")
)

// SupplierAccountingCoverageGap records one known accounting-coverage epoch.
// Epochs may overlap and are never merged so incident and finance evidence
// remains independently auditable.
type SupplierAccountingCoverageGap struct {
	Id                           int64    `json:"id"`
	StartAt                      int64    `json:"start_at" gorm:"not null;index:idx_supplier_accounting_coverage_gaps_start_at"`
	EndAt                        *int64   `json:"end_at"`
	ReasonCategory               string   `json:"reason_category" gorm:"type:varchar(64);not null"`
	ReasonText                   string   `json:"reason_text" gorm:"type:text;not null"`
	ExpectedCapabilityVersion    int64    `json:"expected_capability_version" gorm:"not null"`
	AffectedCapabilityVersion    *int64   `json:"affected_capability_version"`
	AffectedOCIDigest            *string  `json:"affected_oci_digest" gorm:"column:affected_oci_digest;type:varchar(71)"`
	AffectedBuildCommit          *string  `json:"affected_build_commit" gorm:"type:varchar(64)"`
	ActivationStateVersionBefore int64    `json:"activation_state_version_before" gorm:"not null"`
	ActivationStateVersionAfter  int64    `json:"activation_state_version_after" gorm:"not null"`
	OpenCommandID                string   `json:"open_command_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_supplier_coverage_gap_open_command"`
	CloseCommandID               *string  `json:"close_command_id" gorm:"type:varchar(128);uniqueIndex:ux_supplier_coverage_gap_close_command"`
	OpenedBy                     int      `json:"opened_by" gorm:"not null"`
	ClosedBy                     *int     `json:"closed_by"`
	FinanceDisposition           string   `json:"finance_disposition" gorm:"type:varchar(32);not null"`
	EvidenceRefs                 []string `json:"evidence_refs" gorm:"type:text;not null;serializer:json"`
	RecordVersion                int64    `json:"record_version" gorm:"not null;default:1"`
	CreatedAt                    int64    `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                    int64    `json:"updated_at" gorm:"autoUpdateTime"`
}

type OpenSupplierAccountingCoverageGapInput struct {
	StartAt                      int64
	ReasonCategory               string
	ReasonText                   string
	ExpectedCapabilityVersion    int64
	AffectedCapabilityVersion    *int64
	AffectedOCIDigest            *string
	AffectedBuildCommit          *string
	ActivationStateVersionBefore int64
	ActivationStateVersionAfter  int64
	OpenCommandID                string
	OpenedBy                     int
	EvidenceRefs                 []string
}

type CloseSupplierAccountingCoverageGapInput struct {
	ID                 int64
	EndAt              int64
	CloseCommandID     string
	ClosedBy           int
	FinanceDisposition string
	ExpectedVersion    int64
}

func (g *SupplierAccountingCoverageGap) BeforeCreate(_ *gorm.DB) error {
	return g.validate()
}

func (g *SupplierAccountingCoverageGap) validate() error {
	if g == nil || g.StartAt <= 0 || g.ExpectedCapabilityVersion <= 0 || g.ActivationStateVersionBefore < 0 ||
		g.ActivationStateVersionAfter != g.ActivationStateVersionBefore+1 || g.OpenedBy <= 0 ||
		g.RecordVersion < SupplierCoverageGapInitialRecordVersion {
		return ErrSupplierCoverageGapInvalid
	}
	if !isSupplierCoverageGapReason(g.ReasonCategory) || strings.TrimSpace(g.ReasonText) != g.ReasonText ||
		g.ReasonText == "" || len(g.ReasonText) > SupplierCoverageGapMaxReasonTextBytes || !utf8.ValidString(g.ReasonText) {
		return ErrSupplierCoverageGapInvalid
	}
	if g.AffectedCapabilityVersion != nil && *g.AffectedCapabilityVersion <= 0 {
		return ErrSupplierCoverageGapInvalid
	}
	if !validSupplierCoverageGapOCIDigest(g.AffectedOCIDigest) || !validSupplierCoverageGapBuildCommit(g.AffectedBuildCommit) ||
		!validSupplierCoverageGapCommandID(g.OpenCommandID) || !validSupplierCoverageGapEvidenceRefs(g.EvidenceRefs) ||
		!isSupplierCoverageGapFinanceDisposition(g.FinanceDisposition) {
		return ErrSupplierCoverageGapInvalid
	}
	if g.EndAt == nil {
		if g.CloseCommandID != nil || g.ClosedBy != nil || g.FinanceDisposition != SupplierCoverageGapFinancePending {
			return ErrSupplierCoverageGapInvalid
		}
		return nil
	}
	if *g.EndAt <= g.StartAt || g.CloseCommandID == nil || g.ClosedBy == nil || *g.ClosedBy <= 0 ||
		!validSupplierCoverageGapCommandID(*g.CloseCommandID) || !isSupplierCoverageGapFinalFinanceDisposition(g.FinanceDisposition) {
		return ErrSupplierCoverageGapInvalid
	}
	return nil
}

// OpenSupplierAccountingCoverageGap is transaction-aware: callers may pass a
// main-DB transaction that also updates activation and command-ledger state.
func OpenSupplierAccountingCoverageGap(tx *gorm.DB, input OpenSupplierAccountingCoverageGapInput) (*SupplierAccountingCoverageGap, error) {
	if tx == nil {
		return nil, ErrDatabase
	}
	candidate := SupplierAccountingCoverageGap{
		StartAt:                      input.StartAt,
		ReasonCategory:               strings.TrimSpace(input.ReasonCategory),
		ReasonText:                   strings.TrimSpace(input.ReasonText),
		ExpectedCapabilityVersion:    input.ExpectedCapabilityVersion,
		AffectedCapabilityVersion:    cloneInt64(input.AffectedCapabilityVersion),
		AffectedOCIDigest:            cloneTrimmedString(input.AffectedOCIDigest),
		AffectedBuildCommit:          cloneTrimmedString(input.AffectedBuildCommit),
		ActivationStateVersionBefore: input.ActivationStateVersionBefore,
		ActivationStateVersionAfter:  input.ActivationStateVersionAfter,
		OpenCommandID:                strings.TrimSpace(input.OpenCommandID),
		OpenedBy:                     input.OpenedBy,
		FinanceDisposition:           SupplierCoverageGapFinancePending,
		EvidenceRefs:                 cloneStrings(input.EvidenceRefs),
		RecordVersion:                SupplierCoverageGapInitialRecordVersion,
	}
	if err := candidate.validate(); err != nil {
		return nil, err
	}
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "open_command_id"}},
		DoNothing: true,
	}).Create(&candidate)
	if result.Error != nil {
		return nil, result.Error
	}
	var persisted SupplierAccountingCoverageGap
	if err := tx.Where("open_command_id = ?", candidate.OpenCommandID).First(&persisted).Error; err != nil {
		return nil, err
	}
	if !sameSupplierCoverageGapOpenPayload(&persisted, &candidate) {
		return nil, ErrSupplierCoverageGapIdempotencyConflict
	}
	return &persisted, nil
}

// CloseSupplierAccountingCoverageGap closes exactly one named open row via an
// id + open-state + record-version CAS. Historical open fields are untouched.
func CloseSupplierAccountingCoverageGap(tx *gorm.DB, input CloseSupplierAccountingCoverageGapInput) (*SupplierAccountingCoverageGap, error) {
	if tx == nil {
		return nil, ErrDatabase
	}
	input.CloseCommandID = strings.TrimSpace(input.CloseCommandID)
	input.FinanceDisposition = strings.TrimSpace(input.FinanceDisposition)
	if input.ID <= 0 || input.EndAt <= 0 || input.ClosedBy <= 0 || input.ExpectedVersion < SupplierCoverageGapInitialRecordVersion ||
		!validSupplierCoverageGapCommandID(input.CloseCommandID) || !isSupplierCoverageGapFinalFinanceDisposition(input.FinanceDisposition) {
		return nil, ErrSupplierCoverageGapInvalid
	}
	var named SupplierAccountingCoverageGap
	if err := tx.First(&named, input.ID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSupplierCoverageGapNotFound
	} else if err != nil {
		return nil, err
	}
	if input.EndAt <= named.StartAt {
		return nil, ErrSupplierCoverageGapInvalid
	}
	if replay, err := findSupplierCoverageGapCloseReplay(tx, input); err != nil || replay != nil {
		return replay, err
	}
	updatedAt, err := getSupplierDBTimestamp(tx)
	if err != nil {
		return nil, err
	}
	result := tx.Model(&SupplierAccountingCoverageGap{}).
		Where("id = ? AND end_at IS NULL AND record_version = ?", input.ID, input.ExpectedVersion).
		UpdateColumns(map[string]any{
			"end_at":              input.EndAt,
			"close_command_id":    input.CloseCommandID,
			"closed_by":           input.ClosedBy,
			"finance_disposition": input.FinanceDisposition,
			"record_version":      gorm.Expr("record_version + ?", 1),
			"updated_at":          updatedAt,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		if replay, replayErr := findSupplierCoverageGapCloseReplay(tx, input); replayErr != nil || replay != nil {
			return replay, replayErr
		}
		var current SupplierAccountingCoverageGap
		if err := tx.First(&current, input.ID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSupplierCoverageGapNotFound
		} else if err != nil {
			return nil, err
		}
		return nil, ErrSupplierCoverageGapCASConflict
	}
	var closed SupplierAccountingCoverageGap
	if err := tx.First(&closed, input.ID).Error; err != nil {
		return nil, err
	}
	return &closed, nil
}

// QuerySupplierAccountingCoverageGapsOverlapping returns all epochs that
// intersect the half-open interval [startAt, endAt).
func QuerySupplierAccountingCoverageGapsOverlapping(db *gorm.DB, startAt, endAt int64) ([]SupplierAccountingCoverageGap, error) {
	if db == nil {
		return nil, ErrDatabase
	}
	if startAt < 0 || endAt <= startAt {
		return nil, ErrSupplierCoverageGapInvalid
	}
	var gaps []SupplierAccountingCoverageGap
	err := db.Where("start_at < ? AND (end_at IS NULL OR end_at > ?)", endAt, startAt).
		Order("start_at ASC, id ASC").Find(&gaps).Error
	return gaps, err
}

func findSupplierCoverageGapCloseReplay(tx *gorm.DB, input CloseSupplierAccountingCoverageGapInput) (*SupplierAccountingCoverageGap, error) {
	var gap SupplierAccountingCoverageGap
	err := tx.Where("close_command_id = ?", input.CloseCommandID).First(&gap).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if gap.Id != input.ID || gap.EndAt == nil || *gap.EndAt != input.EndAt || gap.ClosedBy == nil || *gap.ClosedBy != input.ClosedBy ||
		gap.FinanceDisposition != input.FinanceDisposition || gap.RecordVersion != input.ExpectedVersion+1 {
		return nil, ErrSupplierCoverageGapIdempotencyConflict
	}
	return &gap, nil
}

func sameSupplierCoverageGapOpenPayload(left, right *SupplierAccountingCoverageGap) bool {
	return left.StartAt == right.StartAt && left.ReasonCategory == right.ReasonCategory && left.ReasonText == right.ReasonText &&
		int64PointersEqual(left.AffectedCapabilityVersion, right.AffectedCapabilityVersion) &&
		stringPointersEqual(left.AffectedOCIDigest, right.AffectedOCIDigest) && stringPointersEqual(left.AffectedBuildCommit, right.AffectedBuildCommit) &&
		left.ExpectedCapabilityVersion == right.ExpectedCapabilityVersion &&
		left.ActivationStateVersionBefore == right.ActivationStateVersionBefore && left.ActivationStateVersionAfter == right.ActivationStateVersionAfter &&
		left.OpenedBy == right.OpenedBy && slices.Equal(left.EvidenceRefs, right.EvidenceRefs)
}

func isSupplierCoverageGapReason(value string) bool {
	switch value {
	case SupplierCoverageGapReasonLogWriteFailure, SupplierCoverageGapReasonLoggingDisabled,
		SupplierCoverageGapReasonProducerCapabilityMismatch, SupplierCoverageGapReasonEmergencyRollback,
		SupplierCoverageGapReasonOperatorDeclared:
		return true
	default:
		return false
	}
}

func isSupplierCoverageGapFinanceDisposition(value string) bool {
	switch value {
	case SupplierCoverageGapFinancePending, SupplierCoverageGapFinanceNoImpact,
		SupplierCoverageGapFinanceReconciled, SupplierCoverageGapFinanceAcceptedLoss:
		return true
	default:
		return false
	}
}

func isSupplierCoverageGapFinalFinanceDisposition(value string) bool {
	return value != SupplierCoverageGapFinancePending && isSupplierCoverageGapFinanceDisposition(value)
}

func validSupplierCoverageGapEvidenceRefs(refs []string) bool {
	if len(refs) > SupplierCoverageGapMaxEvidenceRefCount {
		return false
	}
	for _, ref := range refs {
		if ref == "" || strings.TrimSpace(ref) != ref || len(ref) > SupplierCoverageGapMaxEvidenceRefBytes || !utf8.ValidString(ref) || containsControlCharacter(ref) {
			return false
		}
	}
	encoded, err := common.Marshal(refs)
	return err == nil && len(encoded) <= SupplierCoverageGapMaxEvidenceRefsBytes
}

func validSupplierCoverageGapCommandID(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && len(value) <= SupplierCoverageGapMaxCommandIDBytes && utf8.ValidString(value) && !containsControlCharacter(value)
}

func validSupplierCoverageGapOCIDigest(value *string) bool {
	if value == nil {
		return true
	}
	if len(*value) != SupplierCoverageGapMaxOCIDigestBytes || !strings.HasPrefix(*value, "sha256:") {
		return false
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(*value, "sha256:"))
	return err == nil && len(decoded) == sha256.Size && strings.ToLower(*value) == *value
}

func validSupplierCoverageGapBuildCommit(value *string) bool {
	if value == nil {
		return true
	}
	if (len(*value) != 40 && len(*value) != SupplierCoverageGapMaxBuildCommitBytes) || strings.ToLower(*value) != *value {
		return false
	}
	_, err := hex.DecodeString(*value)
	return err == nil
}

func containsControlCharacter(value string) bool {
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return true
		}
	}
	return false
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneTrimmedString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := strings.TrimSpace(*value)
	return &cloned
}

func cloneStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return slices.Clone(values)
}

func int64PointersEqual(left, right *int64) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func stringPointersEqual(left, right *string) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}
