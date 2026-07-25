package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	SupplierAccountingFactStatusPending  = "pending"
	SupplierAccountingFactStatusCaptured = "captured"
	SupplierAccountingFactStatusVoid     = "void"

	SupplierAccountingFactPageSize           = 5000
	SupplierAccountingFactRetentionChunkSize = 5000
)

var (
	ErrSupplierAccountingFactNotFound          = errors.New("supplier accounting fact not found")
	ErrSupplierAccountingFactBeforeCutover     = errors.New("supplier accounting fact is before cutover")
	ErrSupplierAccountingFactTerminalConflict  = errors.New("supplier accounting fact terminal state conflicts")
	ErrSupplierAccountingFactResolutionInvalid = errors.New("supplier accounting fact resolution is invalid")
	ErrSupplierAccountingFactsPending          = errors.New("supplier accounting facts remain pending")
	ErrSupplierAccountingFactWatermarkChanged  = errors.New("supplier accounting fact watermark changed")
	supplierAccountingFactLocation             = time.FixedZone("Asia/Shanghai", 8*60*60)
)

// SupplierAccountingFact is the durable lifecycle record for one real
// synchronous upstream relay attempt. It lives in LOG_DB independently of the
// optional general-purpose consume log.
type SupplierAccountingFact struct {
	Id                 int64  `json:"id" gorm:"index:idx_supplier_accounting_facts_day_status_id,priority:3"`
	AttemptId          string `json:"attempt_id" gorm:"type:varchar(36);not null;uniqueIndex:ux_supplier_accounting_facts_attempt_id"`
	ParentRequestId    string `json:"parent_request_id" gorm:"type:varchar(191);not null;default:''"`
	RetryIndex         int    `json:"retry_index" gorm:"not null;default:0"`
	PreparedAt         int64  `json:"prepared_at" gorm:"not null"`
	PreparedDay        string `json:"prepared_day" gorm:"type:varchar(10);not null;index:idx_supplier_accounting_facts_day_status_id,priority:1"`
	SupplierId         int    `json:"supplier_id" gorm:"not null;default:0"`
	ContractId         int    `json:"contract_id" gorm:"not null;default:0"`
	BindingVersionId   int    `json:"binding_version_id" gorm:"not null;default:0"`
	RateVersionId      int    `json:"rate_version_id" gorm:"not null;default:0"`
	ChannelId          int    `json:"channel_id" gorm:"not null;default:0"`
	ModelName          string `json:"model_name" gorm:"type:varchar(191);not null;default:''"`
	CoverageScope      string `json:"coverage_scope" gorm:"type:varchar(32);not null"`
	Status             string `json:"status" gorm:"type:varchar(16);not null;index:idx_supplier_accounting_facts_day_status_id,priority:2"`
	Payload            string `json:"payload,omitempty" gorm:"type:text"`
	PayloadHash        string `json:"payload_hash,omitempty" gorm:"type:char(64);not null;default:''"`
	TerminalAt         *int64 `json:"terminal_at"`
	ResolutionActor    string `json:"resolution_actor,omitempty" gorm:"type:varchar(191);not null;default:''"`
	ResolutionReason   string `json:"resolution_reason,omitempty" gorm:"type:varchar(255);not null;default:''"`
	ResolutionEvidence string `json:"resolution_evidence,omitempty" gorm:"type:text"`
	CreatedAt          int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

type SupplierAccountingFactPrepare struct {
	AttemptId        string
	ParentRequestId  string
	RetryIndex       int
	SupplierId       int
	ContractId       int
	BindingVersionId int
	RateVersionId    int
	ChannelId        int
	ModelName        string
	CoverageScope    string
	CutoverAt        int64
}

type SupplierAccountingFactResolution struct {
	AttemptId  string
	Status     string
	Envelope   *types.SupplierAccountingEnvelopeV1
	Actor      string
	Reason     string
	Evidence   string
	TerminalAt int64
}

type SupplierAccountingFactDayWatermark struct {
	PreparedDay     string
	SourceMaxFactId int64
}

type SupplierAccountingFactRetentionChunkResult struct {
	Selected int
	Deleted  int64
}

type SupplierAccountingFactRow struct {
	Id          int64
	ChannelId   int
	ModelName   string
	Payload     string
	PayloadHash string
}

func EnsureSupplierAccountingFactSchema(db *gorm.DB) error {
	if db == nil {
		return ErrDatabase
	}
	migrator := db.Migrator()
	if !migrator.HasTable(&SupplierAccountingFact{}) {
		if err := migrator.CreateTable(&SupplierAccountingFact{}); err != nil {
			return err
		}
	}
	for _, indexName := range []string{"ux_supplier_accounting_facts_attempt_id", "idx_supplier_accounting_facts_day_status_id"} {
		if !migrator.HasIndex(&SupplierAccountingFact{}, indexName) {
			if err := migrator.CreateIndex(&SupplierAccountingFact{}, indexName); err != nil {
				return err
			}
		}
	}
	return nil
}

func PrepareSupplierAccountingFact(ctx context.Context, db *gorm.DB, input SupplierAccountingFactPrepare) (SupplierAccountingFact, error) {
	attemptID, existing, err := prepareSupplierAccountingFactPending(ctx, db, input)
	if err != nil {
		return SupplierAccountingFact{}, err
	}
	if existing != nil {
		return *existing, nil
	}
	var fact SupplierAccountingFact
	if err := db.WithContext(ctx).Where("attempt_id = ?", attemptID).First(&fact).Error; err != nil {
		return SupplierAccountingFact{}, err
	}
	return fact, nil
}

// PrepareSupplierAccountingFactFast durably creates the pending fact with one
// database statement on the successful hot path. It returns only the attempt
// ID because relay settlement does not need the generated row fields.
func PrepareSupplierAccountingFactFast(ctx context.Context, db *gorm.DB, input SupplierAccountingFactPrepare) (string, error) {
	attemptID, _, err := prepareSupplierAccountingFactPending(ctx, db, input)
	return attemptID, err
}

func prepareSupplierAccountingFactPending(ctx context.Context, db *gorm.DB, input SupplierAccountingFactPrepare) (string, *SupplierAccountingFact, error) {
	if db == nil || input.RetryIndex < 0 || input.SupplierId <= 0 || input.ContractId <= 0 || input.BindingVersionId <= 0 ||
		input.RateVersionId <= 0 || input.ChannelId <= 0 || strings.TrimSpace(input.ModelName) == "" || strings.TrimSpace(input.ParentRequestId) == "" ||
		input.CoverageScope != string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1) || input.CutoverAt < 0 {
		return "", nil, ErrSupplierAccountingFactResolutionInvalid
	}
	attemptID := strings.TrimSpace(input.AttemptId)
	if attemptID == "" {
		attemptID = uuid.NewString()
	} else if _, err := uuid.Parse(attemptID); err != nil {
		return "", nil, ErrSupplierAccountingFactResolutionInvalid
	}
	result := db.WithContext(ctx).Exec(supplierAccountingFactPrepareSQL(db),
		attemptID, strings.TrimSpace(input.ParentRequestId), input.RetryIndex,
		input.SupplierId, input.ContractId, input.BindingVersionId, input.RateVersionId,
		input.ChannelId, input.ModelName, input.CoverageScope, input.CutoverAt,
	)
	if result.Error == nil {
		if result.RowsAffected == 0 {
			return "", nil, ErrSupplierAccountingFactBeforeCutover
		}
		return attemptID, nil, nil
	}
	var existing SupplierAccountingFact
	if loadErr := db.WithContext(ctx).Where("attempt_id = ?", attemptID).First(&existing).Error; loadErr != nil {
		return "", nil, fmt.Errorf("create supplier accounting fact: %w; reload: %v", result.Error, loadErr)
	}
	if supplierAccountingFactPrepareIdentityMatches(existing, input) {
		return attemptID, &existing, nil
	}
	return "", nil, ErrSupplierAccountingFactTerminalConflict
}

func supplierAccountingFactPrepareSQL(db *gorm.DB) string {
	databaseUnix, preparedDay := "UNIX_TIMESTAMP()", "DATE_FORMAT(DATE_ADD(UTC_TIMESTAMP(), INTERVAL 8 HOUR), '%Y-%m-%d')"
	switch db.Dialector.Name() {
	case "postgres":
		databaseUnix = "EXTRACT(EPOCH FROM NOW())::bigint"
		preparedDay = "TO_CHAR(NOW() AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD')"
	case "sqlite":
		databaseUnix = "CAST(strftime('%s','now') AS INTEGER)"
		preparedDay = "strftime('%Y-%m-%d','now','+8 hours')"
	}
	return fmt.Sprintf(`INSERT INTO supplier_accounting_facts
		(attempt_id, parent_request_id, retry_index, prepared_at, prepared_day, supplier_id, contract_id, binding_version_id, rate_version_id, channel_id, model_name, coverage_scope, status, payload, payload_hash, terminal_at, resolution_actor, resolution_reason, resolution_evidence, created_at, updated_at)
		SELECT ?, ?, ?, %s, %s, ?, ?, ?, ?, ?, ?, ?, '%s', '', '', NULL, '', '', '', %s, %s
		WHERE %s >= ?`, databaseUnix, preparedDay, SupplierAccountingFactStatusPending, databaseUnix, databaseUnix, databaseUnix)
}

func IsSupplierAccountingCutoverActive(ctx context.Context, db *gorm.DB, cutoverAt int64) (bool, error) {
	if db == nil || cutoverAt <= 0 {
		return false, ErrDatabase
	}
	databaseNow, err := supplierDBUnix(ctx, db)
	if err != nil {
		return false, err
	}
	return databaseNow >= cutoverAt, nil
}

func supplierAccountingFactPrepareIdentityMatches(existing SupplierAccountingFact, expected SupplierAccountingFactPrepare) bool {
	return existing.ParentRequestId == strings.TrimSpace(expected.ParentRequestId) && existing.RetryIndex == expected.RetryIndex &&
		existing.SupplierId == expected.SupplierId && existing.ContractId == expected.ContractId &&
		existing.BindingVersionId == expected.BindingVersionId && existing.RateVersionId == expected.RateVersionId &&
		existing.ChannelId == expected.ChannelId && existing.ModelName == expected.ModelName &&
		existing.CoverageScope == expected.CoverageScope
}

func FinalizeSupplierAccountingFactCaptured(ctx context.Context, db *gorm.DB, attemptID string, envelope types.SupplierAccountingEnvelopeV1, terminalAt int64) error {
	return resolveSupplierAccountingFact(ctx, db, SupplierAccountingFactResolution{
		AttemptId: attemptID, Status: SupplierAccountingFactStatusCaptured, Envelope: &envelope,
		Actor: "system", Reason: "settlement_captured", Evidence: "relay_settlement_v1", TerminalAt: terminalAt,
	}, false)
}

func FinalizeSupplierAccountingFactVoid(ctx context.Context, db *gorm.DB, attemptID string, terminalAt int64) error {
	return resolveSupplierAccountingFact(ctx, db, SupplierAccountingFactResolution{
		AttemptId: attemptID, Status: SupplierAccountingFactStatusVoid,
		Actor: "system", Reason: "attempt_void", Evidence: "relay_attempt_v1", TerminalAt: terminalAt,
	}, false)
}

// ResolveSupplierAccountingFact is the audited Root/manual resolution
// primitive. HTTP authorization belongs in the controller that invokes it.
func ResolveSupplierAccountingFact(ctx context.Context, db *gorm.DB, resolution SupplierAccountingFactResolution) error {
	return resolveSupplierAccountingFact(ctx, db, resolution, true)
}

func resolveSupplierAccountingFact(ctx context.Context, db *gorm.DB, resolution SupplierAccountingFactResolution, requireHumanAudit bool) error {
	if db == nil || strings.TrimSpace(resolution.AttemptId) == "" || resolution.TerminalAt <= 0 {
		return ErrSupplierAccountingFactResolutionInvalid
	}
	actor, reason, evidence := strings.TrimSpace(resolution.Actor), strings.TrimSpace(resolution.Reason), strings.TrimSpace(resolution.Evidence)
	if actor == "" || reason == "" || evidence == "" || (requireHumanAudit && actor == "system") {
		return ErrSupplierAccountingFactResolutionInvalid
	}
	payload, payloadHash := "", ""
	switch resolution.Status {
	case SupplierAccountingFactStatusCaptured:
		if resolution.Envelope == nil || resolution.Envelope.Disposition != types.SupplierAccountingDispositionCaptured || resolution.Envelope.Captured == nil {
			return ErrSupplierAccountingFactResolutionInvalid
		}
		encoded, err := common.Marshal(resolution.Envelope)
		if err != nil {
			return fmt.Errorf("marshal supplier accounting fact payload: %w", err)
		}
		payload = string(encoded)
		digest := sha256.Sum256(encoded)
		payloadHash = hex.EncodeToString(digest[:])
	case SupplierAccountingFactStatusVoid:
		if resolution.Envelope != nil {
			return ErrSupplierAccountingFactResolutionInvalid
		}
	default:
		return ErrSupplierAccountingFactResolutionInvalid
	}
	terminalAt := resolution.TerminalAt
	query := db.WithContext(ctx).Model(&SupplierAccountingFact{}).
		Where("attempt_id = ? AND status = ?", resolution.AttemptId, SupplierAccountingFactStatusPending)
	if resolution.Status == SupplierAccountingFactStatusCaptured {
		query = query.Where("supplier_id = ? AND contract_id = ? AND binding_version_id = ? AND rate_version_id = ?",
			resolution.Envelope.Captured.SupplierId, resolution.Envelope.Captured.ContractId,
			resolution.Envelope.Captured.BindingVersionId, resolution.Envelope.Captured.RateVersionId)
	}
	result := query.
		Updates(map[string]any{
			"status": resolution.Status, "payload": payload, "payload_hash": payloadHash, "terminal_at": terminalAt,
			"resolution_actor": actor, "resolution_reason": reason, "resolution_evidence": evidence,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}
	var existing SupplierAccountingFact
	if err := db.WithContext(ctx).Where("attempt_id = ?", resolution.AttemptId).First(&existing).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrSupplierAccountingFactNotFound
	} else if err != nil {
		return err
	}
	if existing.Status == resolution.Status && existing.PayloadHash == payloadHash &&
		existing.ResolutionActor == actor && existing.ResolutionReason == reason && existing.ResolutionEvidence == evidence {
		return nil
	}
	return ErrSupplierAccountingFactTerminalConflict
}

func FreezeSupplierAccountingFactDay(ctx context.Context, db *gorm.DB, preparedDay string) (SupplierAccountingFactDayWatermark, error) {
	watermark, err := inspectSupplierAccountingFactDay(ctx, db, preparedDay)
	if err != nil {
		return watermark, err
	}
	if watermark.SourceMaxFactId > 0 {
		var pending int64
		if err := db.WithContext(ctx).Model(&SupplierAccountingFact{}).
			Where("prepared_day = ? AND coverage_scope = ? AND id <= ? AND status = ?", preparedDay,
				string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1), watermark.SourceMaxFactId, SupplierAccountingFactStatusPending).
			Count(&pending).Error; err != nil {
			return watermark, err
		}
		if pending > 0 {
			return watermark, ErrSupplierAccountingFactsPending
		}
	}
	return watermark, nil
}

func VerifySupplierAccountingFactDayClosed(ctx context.Context, db *gorm.DB, preparedDay string, sourceMaxFactID int64) error {
	watermark, err := inspectSupplierAccountingFactDay(ctx, db, preparedDay)
	if err != nil {
		return err
	}
	if watermark.SourceMaxFactId != sourceMaxFactID {
		return ErrSupplierAccountingFactWatermarkChanged
	}
	if sourceMaxFactID > 0 {
		var pending int64
		if err := db.WithContext(ctx).Model(&SupplierAccountingFact{}).
			Where("prepared_day = ? AND coverage_scope = ? AND id <= ? AND status = ?", preparedDay,
				string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1), sourceMaxFactID, SupplierAccountingFactStatusPending).
			Count(&pending).Error; err != nil {
			return err
		}
		if pending > 0 {
			return ErrSupplierAccountingFactsPending
		}
	}
	return nil
}

func inspectSupplierAccountingFactDay(ctx context.Context, db *gorm.DB, preparedDay string) (SupplierAccountingFactDayWatermark, error) {
	watermark := SupplierAccountingFactDayWatermark{PreparedDay: preparedDay}
	if db == nil || len(preparedDay) != len("2006-01-02") {
		return watermark, ErrSupplierAccountingFactResolutionInvalid
	}
	if err := db.WithContext(ctx).Model(&SupplierAccountingFact{}).
		Where("prepared_day = ? AND coverage_scope = ?", preparedDay, string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1)).
		Select("COALESCE(MAX(id), 0)").Scan(&watermark.SourceMaxFactId).Error; err != nil {
		return watermark, err
	}
	return watermark, nil
}

func ScanCapturedSupplierAccountingFactPage(ctx context.Context, db *gorm.DB, preparedDay string, sourceMaxFactID, cursorID int64, pageSize int) ([]SupplierAccountingFactRow, error) {
	if db == nil || sourceMaxFactID <= 0 || cursorID < 0 || cursorID >= sourceMaxFactID || pageSize <= 0 || pageSize > SupplierAccountingFactPageSize {
		return nil, ErrDatabase
	}
	rows := make([]SupplierAccountingFactRow, 0, pageSize)
	if err := db.WithContext(ctx).Model(&SupplierAccountingFact{}).
		Select("id", "channel_id", "model_name", "payload", "payload_hash").
		Where("prepared_day = ? AND coverage_scope = ? AND status = ? AND id > ? AND id <= ?", preparedDay,
			string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1), SupplierAccountingFactStatusCaptured, cursorID, sourceMaxFactID).
		Order("id ASC").Limit(pageSize).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func DeleteSupplierAccountingFactRetentionChunk(ctx context.Context, db *gorm.DB, preparedDay string, sourceMaxFactID int64) (SupplierAccountingFactRetentionChunkResult, error) {
	result := SupplierAccountingFactRetentionChunkResult{}
	parsedDay, err := time.ParseInLocation("2006-01-02", preparedDay, supplierAccountingFactLocation)
	if db == nil || sourceMaxFactID < 0 || err != nil || parsedDay.Format("2006-01-02") != preparedDay {
		return result, ErrSupplierAccountingFactResolutionInvalid
	}
	terminalStatuses := []string{SupplierAccountingFactStatusCaptured, SupplierAccountingFactStatusVoid}
	ids := make([]int64, 0, SupplierAccountingFactRetentionChunkSize)
	if err := db.WithContext(ctx).Model(&SupplierAccountingFact{}).
		Where("prepared_day = ? AND coverage_scope = ? AND status IN ? AND id <= ?", preparedDay,
			string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1), terminalStatuses, sourceMaxFactID).
		Order("id ASC").Limit(SupplierAccountingFactRetentionChunkSize).Pluck("id", &ids).Error; err != nil {
		return result, err
	}
	result.Selected = len(ids)
	if len(ids) == 0 {
		return result, nil
	}
	deleted := db.WithContext(ctx).Where("id IN ? AND prepared_day = ? AND coverage_scope = ? AND status IN ? AND id <= ?", ids,
		preparedDay, string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1), terminalStatuses, sourceMaxFactID).
		Delete(&SupplierAccountingFact{})
	if deleted.Error != nil {
		return SupplierAccountingFactRetentionChunkResult{}, deleted.Error
	}
	result.Deleted = deleted.RowsAffected
	return result, nil
}

// ListPendingSupplierAccountingFacts returns one stable, ascending keyset page
// for Root operational review. The date is interpreted in Asia/Shanghai.
func ListPendingSupplierAccountingFacts(ctx context.Context, db *gorm.DB, preparedDay string, cursorID int64, pageSize int) ([]SupplierAccountingFact, error) {
	if db == nil || cursorID < 0 || pageSize <= 0 || pageSize > SupplierAccountingFactPageSize {
		return nil, ErrSupplierAccountingFactResolutionInvalid
	}
	parsedDay, err := time.ParseInLocation("2006-01-02", preparedDay, supplierAccountingFactLocation)
	if err != nil || parsedDay.Format("2006-01-02") != preparedDay {
		return nil, ErrSupplierAccountingFactResolutionInvalid
	}
	facts := make([]SupplierAccountingFact, 0, pageSize)
	if err := db.WithContext(ctx).
		Where("prepared_day = ? AND coverage_scope = ? AND status = ? AND id > ?", preparedDay,
			string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1), SupplierAccountingFactStatusPending, cursorID).
		Order("id ASC").Limit(pageSize).Find(&facts).Error; err != nil {
		return nil, err
	}
	return facts, nil
}

func GetSupplierAccountingFactByAttemptID(ctx context.Context, db *gorm.DB, attemptID string) (SupplierAccountingFact, error) {
	var fact SupplierAccountingFact
	if db == nil || strings.TrimSpace(attemptID) == "" {
		return fact, ErrSupplierAccountingFactResolutionInvalid
	}
	if err := db.WithContext(ctx).Where("attempt_id = ?", strings.TrimSpace(attemptID)).First(&fact).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return SupplierAccountingFact{}, ErrSupplierAccountingFactNotFound
	} else if err != nil {
		return SupplierAccountingFact{}, err
	}
	return fact, nil
}
