package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	supplierHistoricalImportCreateMaxAttempts = 5
	SupplierHistoricalSummarySchemaVersion    = 2
	supplierHistoricalLegacyTestLabel         = "模型测试"
)

const (
	SupplierHistoricalMethodLogEstimateV1  = "log_estimate_v1"
	SupplierHistoricalDataQualityEstimated = "estimated"

	SupplierHistoricalImportStatusPending   = "pending"
	SupplierHistoricalImportStatusRunning   = "running"
	SupplierHistoricalImportStatusCompleted = "completed"
	SupplierHistoricalImportStatusFailed    = "failed"
)

var (
	ErrSupplierHistoricalImportInvalid              = errors.New("invalid supplier historical import")
	ErrSupplierHistoricalImportImmutable            = errors.New("supplier historical import command is immutable")
	ErrSupplierHistoricalImportIdempotencyConflict  = errors.New("supplier historical import idempotency conflict")
	ErrSupplierHistoricalImportBusy                 = errors.New("supplier historical import is leased")
	ErrSupplierHistoricalImportFenceLost            = errors.New("supplier historical import lease fence lost")
	ErrSupplierHistoricalImportOverlap              = errors.New("supplier historical import overlaps a started import")
	ErrSupplierHistoricalImportSourceChanged        = errors.New("supplier historical import source count changed")
	ErrSupplierHistoricalReplacementInvalid         = errors.New("supplier historical replacement is invalid")
	ErrSupplierHistoricalPublicationConflict        = errors.New("supplier historical publication conflicts with the current version")
	ErrSupplierHistoricalPublicationNeedsReestimate = errors.New("supplier historical publication requires re-estimation")
	ErrSupplierHistoricalMoneyOverflow              = errors.New("supplier historical estimate money overflow")
)

type SupplierHistoricalImport struct {
	Id                      int64  `json:"id"`
	CommandHash             string `json:"command_hash" gorm:"type:varchar(64);not null"`
	CommandJSON             string `json:"command_json" gorm:"type:text;not null"`
	IdempotencyKey          string `json:"idempotency_key" gorm:"type:varchar(128);not null;uniqueIndex:ux_supplier_historical_import_actor_key,priority:2"`
	CreatedBy               int    `json:"created_by" gorm:"not null;uniqueIndex:ux_supplier_historical_import_actor_key,priority:1"`
	Method                  string `json:"method" gorm:"type:varchar(32);not null"`
	Reason                  string `json:"reason" gorm:"type:text;not null"`
	StartDate               string `json:"start_date" gorm:"type:varchar(10);not null;index:idx_supplier_historical_import_range,priority:1"`
	EndDate                 string `json:"end_date" gorm:"type:varchar(10);not null;index:idx_supplier_historical_import_range,priority:2"`
	DayStart                int64  `json:"day_start" gorm:"not null"`
	DayEnd                  int64  `json:"day_end" gorm:"not null"`
	QuotaPerUnit            string `json:"quota_per_unit" gorm:"type:varchar(64);not null"`
	ExcludedUserIdsJSON     string `json:"excluded_user_ids_json" gorm:"type:text;not null"`
	ChannelMappingsJSON     string `json:"channel_mappings_json" gorm:"type:text;not null"`
	SummarySchemaVersion    int    `json:"summary_schema_version" gorm:"not null;default:0"`
	SupersedesImportId      *int64 `json:"supersedes_import_id" gorm:"index"`
	SupersededByImportId    *int64 `json:"superseded_by_import_id" gorm:"index"`
	Status                  string `json:"status" gorm:"type:varchar(16);not null;index"`
	SourceMaxLogId          int64  `json:"source_max_log_id" gorm:"not null;default:0"`
	CandidateCount          int64  `json:"candidate_count" gorm:"not null;default:0"`
	ExcludedSystemTestCount int64  `json:"excluded_system_test_count" gorm:"not null;default:0"`
	LeaseOwner              string `json:"lease_owner" gorm:"type:varchar(128);not null;default:''"`
	FenceToken              int64  `json:"fence_token" gorm:"not null;default:0"`
	ActiveLeaseSlot         *int   `json:"-" gorm:"uniqueIndex:ux_supplier_historical_import_active_slot"`
	LockedUntil             int64  `json:"locked_until" gorm:"not null;default:0"`
	CursorCreatedAt         int64  `json:"cursor_created_at" gorm:"not null;default:0"`
	CursorId                int64  `json:"cursor_id" gorm:"not null;default:0"`
	ProcessedCount          int64  `json:"processed_count" gorm:"not null;default:0"`
	SummaryCount            int64  `json:"summary_count" gorm:"not null;default:0"`
	ErrorMessage            string `json:"error_message" gorm:"type:text;not null"`
	StartedAt               *int64 `json:"started_at"`
	CompletedAt             *int64 `json:"completed_at"`
	PublishedAt             *int64 `json:"published_at"`
	PublishedBy             int    `json:"published_by" gorm:"not null;default:0"`
	SupersededAt            *int64 `json:"superseded_at"`
	CreatedAt               int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt               int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (i *SupplierHistoricalImport) BeforeCreate(_ *gorm.DB) error {
	i.CommandHash = strings.TrimSpace(i.CommandHash)
	i.IdempotencyKey = strings.TrimSpace(i.IdempotencyKey)
	i.Method = strings.TrimSpace(i.Method)
	i.Reason = strings.TrimSpace(i.Reason)
	if len(i.CommandHash) != 64 || i.CommandJSON == "" || i.IdempotencyKey == "" || i.CreatedBy <= 0 ||
		i.Method != SupplierHistoricalMethodLogEstimateV1 || i.Reason == "" || i.DayStart <= 0 || i.DayEnd <= i.DayStart ||
		i.StartDate == "" || i.EndDate == "" || i.QuotaPerUnit == "" || i.ExcludedUserIdsJSON == "" || i.ChannelMappingsJSON == "" ||
		(i.SupersedesImportId != nil && *i.SupersedesImportId <= 0) {
		return ErrSupplierHistoricalImportInvalid
	}
	if i.Status == "" {
		i.Status = SupplierHistoricalImportStatusPending
	}
	return nil
}

func (i *SupplierHistoricalImport) BeforeUpdate(tx *gorm.DB) error {
	if tx.Statement.Changed("CommandHash", "CommandJSON", "IdempotencyKey", "CreatedBy", "Method", "Reason", "StartDate", "EndDate", "DayStart", "DayEnd", "QuotaPerUnit", "ExcludedUserIdsJSON", "ChannelMappingsJSON", "SummarySchemaVersion", "SupersedesImportId") {
		return ErrSupplierHistoricalImportImmutable
	}
	return nil
}

func (i *SupplierHistoricalImport) BeforeDelete(_ *gorm.DB) error {
	return ErrSupplierHistoricalImportImmutable
}

type SupplierHistoricalDailySummary struct {
	Id                               int64  `json:"id"`
	ImportId                         int64  `json:"import_id" gorm:"not null;uniqueIndex:ux_supplier_historical_daily_dimension,priority:1;index:idx_supplier_historical_daily_import_date,priority:1"`
	Date                             string `json:"date" gorm:"type:varchar(10);not null;uniqueIndex:ux_supplier_historical_daily_dimension,priority:2;index:idx_supplier_historical_daily_import_date,priority:2"`
	DimensionKey                     string `json:"dimension_key" gorm:"type:varchar(64);not null;uniqueIndex:ux_supplier_historical_daily_dimension,priority:3"`
	BucketStart                      int64  `json:"bucket_start" gorm:"not null"`
	StatisticsScope                  string `json:"statistics_scope" gorm:"type:varchar(16);not null"`
	SupplierId                       int    `json:"supplier_id" gorm:"not null;default:0"`
	ContractId                       int    `json:"contract_id" gorm:"not null;default:0"`
	RateVersionId                    int    `json:"rate_version_id" gorm:"not null;default:0"`
	ChannelId                        int    `json:"channel_id" gorm:"not null;default:0"`
	ModelName                        string `json:"model_name" gorm:"type:varchar(191);not null;default:''"`
	ProcurementMultiplierPpm         *int64 `json:"procurement_multiplier_ppm"`
	DataQuality                      string `json:"data_quality" gorm:"type:varchar(16);not null"`
	SourceRequestCount               int64  `json:"source_request_count" gorm:"not null;default:0"`
	UnassignedRequestCount           int64  `json:"unassigned_request_count" gorm:"not null;default:0"`
	OfficialListKnownCount           int64  `json:"official_list_known_count" gorm:"not null;default:0"`
	OfficialListUnknownCount         int64  `json:"official_list_unknown_count" gorm:"not null;default:0"`
	OfficialListMicroUsd             int64  `json:"official_list_micro_usd,string" gorm:"not null;default:0"`
	SalesKnownCount                  int64  `json:"sales_known_count" gorm:"not null;default:0"`
	SalesUnknownCount                int64  `json:"sales_unknown_count" gorm:"not null;default:0"`
	SalesMicroUsd                    int64  `json:"sales_micro_usd,string" gorm:"not null;default:0"`
	ProcurementCostKnownCount        int64  `json:"procurement_cost_known_count" gorm:"not null;default:0"`
	ProcurementCostUnknownCount      int64  `json:"procurement_cost_unknown_count" gorm:"not null;default:0"`
	ProcurementCostMicroUsd          int64  `json:"procurement_cost_micro_usd,string" gorm:"not null;default:0"`
	GrossProfitKnownCount            int64  `json:"gross_profit_known_count" gorm:"not null;default:0"`
	GrossProfitUnknownCount          int64  `json:"gross_profit_unknown_count" gorm:"not null;default:0"`
	GrossProfitMicroUsd              int64  `json:"gross_profit_micro_usd,string" gorm:"not null;default:0"`
	GrossMarginEligibleCount         int64  `json:"gross_margin_eligible_count" gorm:"not null;default:0"`
	GrossMarginEligibleSalesMicroUsd int64  `json:"gross_margin_eligible_sales_micro_usd,string" gorm:"not null;default:0"`
	CreatedAt                        int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                        int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

type SupplierHistoricalPublishedDay struct {
	Date        string `json:"date" gorm:"type:varchar(10);not null"`
	DayStart    int64  `json:"day_start" gorm:"not null;index"`
	ImportId    int64  `json:"import_id" gorm:"not null;index"`
	PublishedBy int    `json:"published_by" gorm:"not null"`
	PublishedAt int64  `json:"published_at" gorm:"not null"`
}

type SupplierHistoricalImportCreate struct {
	CommandHash, CommandJSON, IdempotencyKey, Method, Reason string
	CreatedBy                                                int
	StartDate, EndDate, QuotaPerUnit                         string
	DayStart, DayEnd                                         int64
	ExcludedUserIdsJSON, ChannelMappingsJSON                 string
	SupersedesImportId                                       *int64
}

type SupplierHistoricalImportLease struct {
	ImportId, FenceToken, SourceMaxLogId, CandidateCount int64
	Owner                                                string
	Started                                              bool
	CursorCreatedAt, CursorId, ProcessedCount            int64
	AlreadyDone                                          bool
}

type SupplierHistoricalSourceStats struct {
	SourceMaxLogId          int64 `gorm:"column:source_max_log_id"`
	CandidateCount          int64 `gorm:"column:candidate_count"`
	ExcludedSystemTestCount int64 `gorm:"column:excluded_system_test_count"`
}

type SupplierHistoricalSourceLog struct {
	Id        int64  `gorm:"column:id"`
	UserId    int    `gorm:"column:user_id"`
	CreatedAt int64  `gorm:"column:created_at"`
	ChannelId int    `gorm:"column:channel_id"`
	ModelName string `gorm:"column:model_name"`
	Quota     int64  `gorm:"column:quota"`
	Other     string `gorm:"column:other"`
}

type SupplierHistoricalRateChain struct {
	RateVersionId            int   `gorm:"column:rate_version_id"`
	ContractId               int   `gorm:"column:contract_id"`
	SupplierId               int   `gorm:"column:supplier_id"`
	ProcurementMultiplierPpm int64 `gorm:"column:procurement_multiplier_ppm"`
}

type SupplierHistoricalSeriesCursor struct {
	Date            string `json:"date"`
	StatisticsScope string `json:"statistics_scope"`
	SupplierId      int    `json:"supplier_id"`
}

type SupplierHistoricalSeriesPoint struct {
	Date                        string `json:"date" gorm:"column:date"`
	BucketStart                 int64  `json:"bucket_start" gorm:"column:bucket_start"`
	StatisticsScope             string `json:"statistics_scope" gorm:"column:statistics_scope"`
	SupplierId                  int    `json:"supplier_id" gorm:"column:supplier_id"`
	DataQuality                 string `json:"data_quality" gorm:"-"`
	SourceRequestCount          int64  `json:"source_request_count" gorm:"column:source_request_count"`
	UnassignedRequestCount      int64  `json:"unassigned_request_count" gorm:"column:unassigned_request_count"`
	OfficialListKnownCount      int64  `json:"official_list_known_count" gorm:"column:official_list_known_count"`
	OfficialListUnknownCount    int64  `json:"official_list_unknown_count" gorm:"column:official_list_unknown_count"`
	OfficialListMicroUsd        int64  `json:"official_list_micro_usd,string" gorm:"column:official_list_micro_usd"`
	SalesKnownCount             int64  `json:"sales_known_count" gorm:"column:sales_known_count"`
	SalesUnknownCount           int64  `json:"sales_unknown_count" gorm:"column:sales_unknown_count"`
	SalesMicroUsd               int64  `json:"sales_micro_usd,string" gorm:"column:sales_micro_usd"`
	ProcurementCostKnownCount   int64  `json:"procurement_cost_known_count" gorm:"column:procurement_cost_known_count"`
	ProcurementCostUnknownCount int64  `json:"procurement_cost_unknown_count" gorm:"column:procurement_cost_unknown_count"`
	ProcurementCostMicroUsd     int64  `json:"procurement_cost_micro_usd,string" gorm:"column:procurement_cost_micro_usd"`
	GrossProfitKnownCount       int64  `json:"gross_profit_known_count" gorm:"column:gross_profit_known_count"`
	GrossProfitUnknownCount     int64  `json:"gross_profit_unknown_count" gorm:"column:gross_profit_unknown_count"`
	GrossProfitMicroUsd         int64  `json:"gross_profit_micro_usd,string" gorm:"column:gross_profit_micro_usd"`
}

func CreateSupplierHistoricalImport(ctx context.Context, db *gorm.DB, input SupplierHistoricalImportCreate) (SupplierHistoricalImport, error) {
	if db == nil {
		return SupplierHistoricalImport{}, ErrDatabase
	}
	for attempt := 0; attempt < supplierHistoricalImportCreateMaxAttempts; attempt++ {
		item := newSupplierHistoricalImport(input)
		overlapped := false
		err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var existing SupplierHistoricalImport
			existingErr := tx.Where("created_by = ? AND idempotency_key = ?", input.CreatedBy, strings.TrimSpace(input.IdempotencyKey)).First(&existing).Error
			if existingErr == nil {
				if existing.CommandHash != strings.TrimSpace(input.CommandHash) {
					return ErrSupplierHistoricalImportIdempotencyConflict
				}
				item = existing
				overlapped = existing.Status == SupplierHistoricalImportStatusFailed && existing.ErrorMessage == ErrSupplierHistoricalImportOverlap.Error()
				return nil
			}
			if !errors.Is(existingErr, gorm.ErrRecordNotFound) {
				return existingErr
			}
			if input.SupersedesImportId != nil {
				var target SupplierHistoricalImport
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&target, *input.SupersedesImportId).Error; err != nil {
					return err
				}
				if target.Status != SupplierHistoricalImportStatusCompleted || target.StartDate != input.StartDate || target.EndDate != input.EndDate ||
					target.DayStart != input.DayStart || target.DayEnd != input.DayEnd || target.SupersededByImportId != nil {
					return ErrSupplierHistoricalReplacementInvalid
				}
			}
			var overlaps int64
			overlapQuery := tx.Model(&SupplierHistoricalImport{}).
				Where("day_start < ? AND day_end > ? AND (status IN ? OR (status = ? AND superseded_at IS NULL))", input.DayEnd, input.DayStart,
					[]string{SupplierHistoricalImportStatusPending, SupplierHistoricalImportStatusRunning}, SupplierHistoricalImportStatusCompleted)
			if input.SupersedesImportId != nil {
				overlapQuery = overlapQuery.Where("id <> ?", *input.SupersedesImportId)
			}
			if err := overlapQuery.Count(&overlaps).Error; err != nil {
				return err
			}
			if overlaps > 0 {
				item.Status = SupplierHistoricalImportStatusFailed
				item.ErrorMessage = ErrSupplierHistoricalImportOverlap.Error()
				overlapped = true
			}
			result := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "created_by"}, {Name: "idempotency_key"}}, DoNothing: true,
			}).Create(&item)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				return nil
			}
			if err := tx.Where("created_by = ? AND idempotency_key = ?", input.CreatedBy, strings.TrimSpace(input.IdempotencyKey)).First(&existing).Error; err != nil {
				return err
			}
			if existing.CommandHash != strings.TrimSpace(input.CommandHash) {
				return ErrSupplierHistoricalImportIdempotencyConflict
			}
			item = existing
			overlapped = existing.Status == SupplierHistoricalImportStatusFailed && existing.ErrorMessage == ErrSupplierHistoricalImportOverlap.Error()
			return nil
		}, &sql.TxOptions{Isolation: sql.LevelSerializable})
		if err == nil {
			if overlapped {
				return item, ErrSupplierHistoricalImportOverlap
			}
			return item, nil
		}
		if !isSupplierHistoricalImportCreateRace(err) || attempt+1 == supplierHistoricalImportCreateMaxAttempts {
			return item, err
		}
		select {
		case <-ctx.Done():
			return item, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 5 * time.Millisecond):
		}
	}
	return SupplierHistoricalImport{}, ErrDatabase
}

func newSupplierHistoricalImport(input SupplierHistoricalImportCreate) SupplierHistoricalImport {
	return SupplierHistoricalImport{
		CommandHash: input.CommandHash, CommandJSON: input.CommandJSON, IdempotencyKey: input.IdempotencyKey,
		CreatedBy: input.CreatedBy, Method: input.Method, Reason: input.Reason, StartDate: input.StartDate, EndDate: input.EndDate,
		DayStart: input.DayStart, DayEnd: input.DayEnd, QuotaPerUnit: input.QuotaPerUnit,
		ExcludedUserIdsJSON: input.ExcludedUserIdsJSON, ChannelMappingsJSON: input.ChannelMappingsJSON,
		SupersedesImportId:   input.SupersedesImportId,
		SummarySchemaVersion: SupplierHistoricalSummarySchemaVersion,
		Status:               SupplierHistoricalImportStatusPending,
	}
}

func isSupplierHistoricalImportCreateRace(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, fragment := range []string{"serialization failure", "could not serialize", "deadlock", "database is locked", "database table is locked", "lock wait timeout", "sqlite_busy"} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}

func AcquireSupplierHistoricalImport(ctx context.Context, db *gorm.DB, importId int64, owner string, leaseDuration time.Duration) (SupplierHistoricalImportLease, error) {
	if db == nil || strings.TrimSpace(owner) == "" || leaseDuration <= 0 {
		return SupplierHistoricalImportLease{}, ErrSupplierHistoricalImportInvalid
	}
	var lease SupplierHistoricalImportLease
	overlapDetected := false
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, err := supplierDBUnix(ctx, tx)
		if err != nil {
			return err
		}
		for {
			query := tx.Clauses(clause.Locking{Strength: "UPDATE"})
			var item SupplierHistoricalImport
			if importId > 0 {
				err = query.First(&item, importId).Error
			} else {
				err = query.Where("status IN ? AND (active_lease_slot IS NULL OR locked_until < ? OR lease_owner = ?)", []string{SupplierHistoricalImportStatusPending, SupplierHistoricalImportStatusRunning}, now, owner).
					Order("id ASC").First(&item).Error
			}
			if importId == 0 && errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			if item.Status == SupplierHistoricalImportStatusCompleted {
				lease = SupplierHistoricalImportLease{ImportId: item.Id, AlreadyDone: true}
				return nil
			}
			if item.Status != SupplierHistoricalImportStatusPending && item.Status != SupplierHistoricalImportStatusRunning {
				return ErrSupplierHistoricalImportInvalid
			}
			if item.ActiveLeaseSlot != nil && item.LockedUntil >= now && item.LeaseOwner != owner {
				return ErrSupplierHistoricalImportBusy
			}
			if item.StartedAt == nil {
				var overlaps int64
				overlapQuery := tx.Model(&SupplierHistoricalImport{}).
					Where("id <> ? AND day_start < ? AND day_end > ? AND (status = ? OR (status = ? AND superseded_at IS NULL) OR (status = ? AND id < ?))",
						item.Id, item.DayEnd, item.DayStart,
						SupplierHistoricalImportStatusRunning, SupplierHistoricalImportStatusCompleted, SupplierHistoricalImportStatusPending, item.Id)
				if item.SupersedesImportId != nil {
					overlapQuery = overlapQuery.Where("id <> ?", *item.SupersedesImportId)
				}
				if err := overlapQuery.Count(&overlaps).Error; err != nil {
					return err
				}
				if overlaps > 0 {
					if updateErr := tx.Model(&SupplierHistoricalImport{}).Where("id = ? AND status = ?", item.Id, SupplierHistoricalImportStatusPending).
						Updates(map[string]any{"status": SupplierHistoricalImportStatusFailed, "error_message": ErrSupplierHistoricalImportOverlap.Error(), "active_lease_slot": nil, "lease_owner": "", "locked_until": 0}).Error; updateErr != nil {
						return updateErr
					}
					if importId > 0 {
						overlapDetected = true
						return nil
					}
					continue
				}
			}
			fence := item.FenceToken + 1
			slot := 1
			result := tx.Model(&SupplierHistoricalImport{}).Where("id = ? AND fence_token = ?", item.Id, item.FenceToken).Updates(map[string]any{
				"lease_owner": owner, "fence_token": fence, "locked_until": now + int64(leaseDuration.Seconds()), "active_lease_slot": slot,
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrSupplierHistoricalImportFenceLost
			}
			lease = SupplierHistoricalImportLease{
				ImportId: item.Id, Owner: owner, FenceToken: fence, Started: item.StartedAt != nil,
				SourceMaxLogId: item.SourceMaxLogId, CandidateCount: item.CandidateCount,
				CursorCreatedAt: item.CursorCreatedAt, CursorId: item.CursorId, ProcessedCount: item.ProcessedCount,
			}
			return nil
		}
	})
	if err != nil {
		return lease, err
	}
	if overlapDetected {
		return lease, ErrSupplierHistoricalImportOverlap
	}
	if importId == 0 && lease.ImportId == 0 {
		return lease, gorm.ErrRecordNotFound
	}
	return lease, nil
}

func FreezeSupplierHistoricalImport(ctx context.Context, db *gorm.DB, lease SupplierHistoricalImportLease, sourceMaxLogId, candidateCount, excludedSystemTestCount int64) error {
	if db == nil || lease.ImportId <= 0 || lease.FenceToken <= 0 || sourceMaxLogId < 0 || candidateCount < 0 || excludedSystemTestCount < 0 {
		return ErrSupplierHistoricalImportInvalid
	}
	now, err := supplierDBUnix(ctx, db)
	if err != nil {
		return err
	}
	result := db.WithContext(ctx).Model(&SupplierHistoricalImport{}).
		Where("id = ? AND status = ? AND started_at IS NULL AND lease_owner = ? AND fence_token = ? AND locked_until >= ?", lease.ImportId, SupplierHistoricalImportStatusPending, lease.Owner, lease.FenceToken, now).
		Updates(map[string]any{"status": SupplierHistoricalImportStatusRunning, "source_max_log_id": sourceMaxLogId, "candidate_count": candidateCount, "excluded_system_test_count": excludedSystemTestCount, "started_at": now, "error_message": ""})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrSupplierHistoricalImportFenceLost
	}
	return nil
}

func CommitSupplierHistoricalImportPage(ctx context.Context, db *gorm.DB, lease SupplierHistoricalImportLease, summaries []SupplierHistoricalDailySummary, cursorCreatedAt, cursorId, processedDelta int64) error {
	if db == nil || lease.ImportId <= 0 || lease.FenceToken <= 0 || processedDelta < 0 {
		return ErrSupplierHistoricalImportInvalid
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := upsertSupplierHistoricalDailySummaries(tx, summaries); err != nil {
			return err
		}
		now, err := supplierDBUnix(ctx, tx)
		if err != nil {
			return err
		}
		result := tx.Model(&SupplierHistoricalImport{}).
			Where("id = ? AND status = ? AND lease_owner = ? AND fence_token = ? AND locked_until >= ?", lease.ImportId, SupplierHistoricalImportStatusRunning, lease.Owner, lease.FenceToken, now).
			Updates(map[string]any{
				"cursor_created_at": cursorCreatedAt, "cursor_id": cursorId,
				"processed_count": gorm.Expr("processed_count + ?", processedDelta),
				"locked_until":    now + 120, "error_message": "",
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrSupplierHistoricalImportFenceLost
		}
		return nil
	})
}

func upsertSupplierHistoricalDailySummaries(tx *gorm.DB, summaries []SupplierHistoricalDailySummary) error {
	if len(summaries) == 0 {
		return nil
	}
	columns := []string{
		"source_request_count", "unassigned_request_count", "official_list_known_count", "official_list_unknown_count", "official_list_micro_usd",
		"sales_known_count", "sales_unknown_count", "sales_micro_usd", "procurement_cost_known_count", "procurement_cost_unknown_count",
		"procurement_cost_micro_usd", "gross_profit_known_count", "gross_profit_unknown_count", "gross_profit_micro_usd",
		"gross_margin_eligible_count", "gross_margin_eligible_sales_micro_usd",
	}
	dimensionKeys := make([]string, 0, len(summaries))
	for index := range summaries {
		dimensionKeys = append(dimensionKeys, summaries[index].DimensionKey)
	}
	var existing []SupplierHistoricalDailySummary
	if err := tx.Where("import_id = ? AND dimension_key IN ?", summaries[0].ImportId, dimensionKeys).Find(&existing).Error; err != nil {
		return err
	}
	byDimension := make(map[string]SupplierHistoricalDailySummary, len(existing))
	for _, item := range existing {
		byDimension[item.DimensionKey] = item
	}
	for index := range summaries {
		current, ok := byDimension[summaries[index].DimensionKey]
		if !ok {
			continue
		}
		if err := mergeSupplierHistoricalSummary(&summaries[index], current); err != nil {
			return err
		}
	}
	assignColumns := append(columns, "updated_at")
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "import_id"}, {Name: "date"}, {Name: "dimension_key"}}, DoUpdates: clause.AssignmentColumns(assignColumns),
	}).CreateInBatches(summaries, 200).Error
}

func mergeSupplierHistoricalSummary(target *SupplierHistoricalDailySummary, existing SupplierHistoricalDailySummary) error {
	pairs := [][2]*int64{
		{&target.SourceRequestCount, &existing.SourceRequestCount}, {&target.UnassignedRequestCount, &existing.UnassignedRequestCount},
		{&target.OfficialListKnownCount, &existing.OfficialListKnownCount}, {&target.OfficialListUnknownCount, &existing.OfficialListUnknownCount},
		{&target.OfficialListMicroUsd, &existing.OfficialListMicroUsd}, {&target.SalesKnownCount, &existing.SalesKnownCount},
		{&target.SalesUnknownCount, &existing.SalesUnknownCount}, {&target.SalesMicroUsd, &existing.SalesMicroUsd},
		{&target.ProcurementCostKnownCount, &existing.ProcurementCostKnownCount}, {&target.ProcurementCostUnknownCount, &existing.ProcurementCostUnknownCount},
		{&target.ProcurementCostMicroUsd, &existing.ProcurementCostMicroUsd}, {&target.GrossProfitKnownCount, &existing.GrossProfitKnownCount},
		{&target.GrossProfitUnknownCount, &existing.GrossProfitUnknownCount}, {&target.GrossProfitMicroUsd, &existing.GrossProfitMicroUsd},
		{&target.GrossMarginEligibleCount, &existing.GrossMarginEligibleCount}, {&target.GrossMarginEligibleSalesMicroUsd, &existing.GrossMarginEligibleSalesMicroUsd},
	}
	for _, pair := range pairs {
		value, ok := supplierHistoricalCheckedAdd(*pair[0], *pair[1])
		if !ok {
			return ErrSupplierHistoricalMoneyOverflow
		}
		*pair[0] = value
	}
	return nil
}

func supplierHistoricalCheckedAdd(left, right int64) (int64, bool) {
	if (right > 0 && left > math.MaxInt64-right) || (right < 0 && left < math.MinInt64-right) {
		return 0, false
	}
	return left + right, true
}

func supplierHistoricalIncrementExpression(dialect, column string) string {
	switch dialect {
	case "postgres":
		return fmt.Sprintf(`"supplier_historical_daily_summaries"."%s" + EXCLUDED."%s"`, column, column)
	case "mysql":
		return fmt.Sprintf("`%s` + VALUES(`%s`)", column, column)
	default:
		return column + " + excluded." + column
	}
}

func CompleteSupplierHistoricalImport(ctx context.Context, db *gorm.DB, lease SupplierHistoricalImportLease, verifiedCount int64) error {
	if db == nil || verifiedCount < 0 {
		return ErrSupplierHistoricalImportInvalid
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, err := supplierDBUnix(ctx, tx)
		if err != nil {
			return err
		}
		var item SupplierHistoricalImport
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, lease.ImportId).Error; err != nil {
			return err
		}
		if item.Status != SupplierHistoricalImportStatusRunning || item.LeaseOwner != lease.Owner || item.FenceToken != lease.FenceToken || item.LockedUntil < now {
			return ErrSupplierHistoricalImportFenceLost
		}
		if verifiedCount != item.ProcessedCount || item.CandidateCount != item.ProcessedCount {
			return ErrSupplierHistoricalImportSourceChanged
		}
		var summaryCount int64
		if err := tx.Model(&SupplierHistoricalDailySummary{}).Where("import_id = ?", item.Id).Count(&summaryCount).Error; err != nil {
			return err
		}
		result := tx.Model(&SupplierHistoricalImport{}).Where("id = ? AND fence_token = ?", item.Id, item.FenceToken).Updates(map[string]any{
			"status": SupplierHistoricalImportStatusCompleted, "summary_count": summaryCount, "completed_at": now,
			"lease_owner": "", "locked_until": 0, "active_lease_slot": nil, "error_message": "",
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrSupplierHistoricalImportFenceLost
		}
		return nil
	})
}

func PublishSupplierHistoricalImport(ctx context.Context, db *gorm.DB, importId int64, actor int) (SupplierHistoricalImport, error) {
	if db == nil || importId <= 0 || actor <= 0 {
		return SupplierHistoricalImport{}, ErrSupplierHistoricalImportInvalid
	}
	var published SupplierHistoricalImport
	for attempt := 0; attempt < supplierHistoricalImportCreateMaxAttempts; attempt++ {
		err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var item SupplierHistoricalImport
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, importId).Error; err != nil {
				return err
			}
			if item.Status != SupplierHistoricalImportStatusCompleted {
				return ErrSupplierHistoricalPublicationConflict
			}
			if item.SummarySchemaVersion != SupplierHistoricalSummarySchemaVersion {
				return ErrSupplierHistoricalPublicationNeedsReestimate
			}
			if item.PublishedAt != nil && item.SupersededAt == nil {
				published = item
				return nil
			}
			if item.SupersededAt != nil {
				return ErrSupplierHistoricalPublicationConflict
			}
			var current []SupplierHistoricalPublishedDay
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("day_start >= ? AND day_start < ?", item.DayStart, item.DayEnd).Order("day_start ASC").Find(&current).Error; err != nil {
				return err
			}
			var target *SupplierHistoricalImport
			if item.SupersedesImportId == nil {
				if len(current) > 0 {
					return ErrSupplierHistoricalPublicationConflict
				}
			} else {
				var previous SupplierHistoricalImport
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&previous, *item.SupersedesImportId).Error; err != nil {
					return err
				}
				if previous.Status != SupplierHistoricalImportStatusCompleted || previous.StartDate != item.StartDate || previous.EndDate != item.EndDate ||
					previous.DayStart != item.DayStart || previous.DayEnd != item.DayEnd ||
					(previous.SupersededByImportId != nil && *previous.SupersededByImportId != item.Id) {
					return ErrSupplierHistoricalReplacementInvalid
				}
				for _, day := range current {
					if day.ImportId != previous.Id {
						return ErrSupplierHistoricalPublicationConflict
					}
				}
				target = &previous
			}
			now, err := supplierDBUnix(ctx, tx)
			if err != nil {
				return err
			}
			days := supplierHistoricalPublicationDays(item, actor, now)
			if target == nil {
				result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "date"}}, DoNothing: true}).CreateInBatches(days, 100)
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != int64(len(days)) {
					return ErrSupplierHistoricalPublicationConflict
				}
			} else {
				if target.PublishedAt != nil && len(current) != len(days) {
					return ErrSupplierHistoricalPublicationConflict
				}
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "date"}},
					DoUpdates: clause.AssignmentColumns([]string{"day_start", "import_id", "published_by", "published_at"}),
				}).CreateInBatches(days, 100).Error; err != nil {
					return err
				}
				result := tx.Model(&SupplierHistoricalImport{}).Where("id = ? AND superseded_by_import_id IS NULL", target.Id).
					Updates(map[string]any{"superseded_by_import_id": item.Id, "superseded_at": now})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return ErrSupplierHistoricalPublicationConflict
				}
			}
			result := tx.Model(&SupplierHistoricalImport{}).Where("id = ? AND published_at IS NULL AND superseded_at IS NULL", item.Id).
				Updates(map[string]any{"published_at": now, "published_by": actor})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrSupplierHistoricalPublicationConflict
			}
			return tx.First(&published, item.Id).Error
		}, &sql.TxOptions{Isolation: sql.LevelSerializable})
		if err == nil {
			return published, nil
		}
		if !isSupplierHistoricalImportCreateRace(err) || attempt+1 == supplierHistoricalImportCreateMaxAttempts {
			return SupplierHistoricalImport{}, err
		}
		select {
		case <-ctx.Done():
			return SupplierHistoricalImport{}, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 5 * time.Millisecond):
		}
	}
	return SupplierHistoricalImport{}, ErrDatabase
}

func supplierHistoricalPublicationDays(item SupplierHistoricalImport, actor int, publishedAt int64) []SupplierHistoricalPublishedDay {
	count := int((item.DayEnd - item.DayStart) / 86_400)
	days := make([]SupplierHistoricalPublishedDay, 0, count)
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	for dayStart := item.DayStart; dayStart < item.DayEnd; dayStart += 86_400 {
		days = append(days, SupplierHistoricalPublishedDay{
			Date: time.Unix(dayStart, 0).In(location).Format("2006-01-02"), DayStart: dayStart,
			ImportId: item.Id, PublishedBy: actor, PublishedAt: publishedAt,
		})
	}
	return days
}

func FailSupplierHistoricalImport(ctx context.Context, db *gorm.DB, lease SupplierHistoricalImportLease, failure error) error {
	if db == nil || lease.ImportId <= 0 || failure == nil {
		return ErrSupplierHistoricalImportInvalid
	}
	result := db.WithContext(ctx).Model(&SupplierHistoricalImport{}).
		Where("id = ? AND lease_owner = ? AND fence_token = ?", lease.ImportId, lease.Owner, lease.FenceToken).
		Updates(map[string]any{"status": SupplierHistoricalImportStatusFailed, "error_message": failure.Error(), "lease_owner": "", "locked_until": 0, "active_lease_slot": nil})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrSupplierHistoricalImportFenceLost
	}
	return nil
}

func GetSupplierHistoricalImport(ctx context.Context, db *gorm.DB, id int64) (SupplierHistoricalImport, error) {
	var item SupplierHistoricalImport
	if db == nil || id <= 0 {
		return item, ErrSupplierHistoricalImportInvalid
	}
	err := db.WithContext(ctx).First(&item, id).Error
	return item, err
}

func ListSupplierHistoricalImports(ctx context.Context, db *gorm.DB, offset, limit int) ([]SupplierHistoricalImport, int64, error) {
	if db == nil || offset < 0 || limit <= 0 || limit > 100 {
		return nil, 0, ErrSupplierHistoricalImportInvalid
	}
	var total int64
	if err := db.WithContext(ctx).Model(&SupplierHistoricalImport{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []SupplierHistoricalImport
	err := db.WithContext(ctx).Order("id DESC").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}

func ListSupplierHistoricalDailySummaries(ctx context.Context, db *gorm.DB, importId int64) ([]SupplierHistoricalDailySummary, error) {
	if db == nil || importId <= 0 {
		return nil, ErrSupplierHistoricalImportInvalid
	}
	var items []SupplierHistoricalDailySummary
	err := db.WithContext(ctx).Where("import_id = ?", importId).Order("date ASC, dimension_key ASC").Find(&items).Error
	return items, err
}

func ListSupplierHistoricalSeries(ctx context.Context, db *gorm.DB, importId int64, startDate, endDate string, cursor SupplierHistoricalSeriesCursor, limit int) ([]SupplierHistoricalSeriesPoint, bool, error) {
	if db == nil || importId <= 0 || startDate == "" || endDate == "" || startDate >= endDate || limit <= 0 || limit > 500 {
		return nil, false, ErrSupplierHistoricalImportInvalid
	}
	query := db.WithContext(ctx).Model(&SupplierHistoricalDailySummary{}).
		Select(`date, MIN(bucket_start) AS bucket_start, statistics_scope, supplier_id,
SUM(source_request_count) AS source_request_count, SUM(unassigned_request_count) AS unassigned_request_count,
SUM(official_list_known_count) AS official_list_known_count, SUM(official_list_unknown_count) AS official_list_unknown_count, SUM(official_list_micro_usd) AS official_list_micro_usd,
SUM(sales_known_count) AS sales_known_count, SUM(sales_unknown_count) AS sales_unknown_count, SUM(sales_micro_usd) AS sales_micro_usd,
SUM(procurement_cost_known_count) AS procurement_cost_known_count, SUM(procurement_cost_unknown_count) AS procurement_cost_unknown_count, SUM(procurement_cost_micro_usd) AS procurement_cost_micro_usd,
SUM(gross_profit_known_count) AS gross_profit_known_count, SUM(gross_profit_unknown_count) AS gross_profit_unknown_count, SUM(gross_profit_micro_usd) AS gross_profit_micro_usd`).
		Where("import_id = ? AND date >= ? AND date < ?", importId, startDate, endDate)
	if cursor.Date != "" {
		query = query.Where("date > ? OR (date = ? AND statistics_scope > ?) OR (date = ? AND statistics_scope = ? AND supplier_id > ?)",
			cursor.Date, cursor.Date, cursor.StatisticsScope, cursor.Date, cursor.StatisticsScope, cursor.SupplierId)
	}
	var rows []SupplierHistoricalSeriesPoint
	err := query.Group("date, statistics_scope, supplier_id").Order("date ASC, statistics_scope ASC, supplier_id ASC").Limit(limit + 1).Scan(&rows).Error
	if err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	for index := range rows {
		rows[index].DataQuality = SupplierHistoricalDataQualityEstimated
	}
	return rows, hasMore, nil
}

func FreezeSupplierHistoricalSourceStats(ctx context.Context, logDB *gorm.DB, dayStart, dayEnd int64) (SupplierHistoricalSourceStats, error) {
	var stats SupplierHistoricalSourceStats
	if logDB == nil || dayStart <= 0 || dayEnd <= dayStart {
		return stats, ErrSupplierHistoricalImportInvalid
	}
	predicate, args := supplierHistoricalSystemTestPredicate()
	err := logDB.WithContext(ctx).Table("logs").
		Select(`COALESCE(MAX(id), 0) AS source_max_log_id,
COALESCE(SUM(CASE WHEN `+predicate+` THEN 0 ELSE 1 END), 0) AS candidate_count,
COALESCE(SUM(CASE WHEN `+predicate+` THEN 1 ELSE 0 END), 0) AS excluded_system_test_count`, append(args, args...)...).
		Where("type = ? AND created_at >= ? AND created_at < ?", LogTypeConsume, dayStart, dayEnd).
		Scan(&stats).Error
	return stats, err
}

func supplierHistoricalSystemTestPredicate() (string, []any) {
	return `COALESCE(token_id, 0) = ? OR (COALESCE(token_id, 0) = 0 AND COALESCE(token_name, '') = ? AND COALESCE(content, '') = ?)`,
		[]any{SystemChannelTestTokenId, supplierHistoricalLegacyTestLabel, supplierHistoricalLegacyTestLabel}
}

func supplierHistoricalSourceQuery(ctx context.Context, logDB *gorm.DB, dayStart, dayEnd, sourceMaxLogId int64) *gorm.DB {
	predicate, args := supplierHistoricalSystemTestPredicate()
	return logDB.WithContext(ctx).Table("logs").
		Where("type = ? AND created_at >= ? AND created_at < ? AND id <= ?", LogTypeConsume, dayStart, dayEnd, sourceMaxLogId).
		Where("NOT ("+predicate+")", args...)
}

func ListSupplierHistoricalSourcePage(ctx context.Context, logDB *gorm.DB, dayStart, dayEnd, sourceMaxLogId, cursorCreatedAt, cursorId int64, limit int) ([]SupplierHistoricalSourceLog, error) {
	if logDB == nil || dayStart <= 0 || dayEnd <= dayStart || sourceMaxLogId < 0 || cursorCreatedAt < 0 || cursorId < 0 || limit <= 0 || limit > 5000 {
		return nil, ErrSupplierHistoricalImportInvalid
	}
	var rows []SupplierHistoricalSourceLog
	query := supplierHistoricalSourceQuery(ctx, logDB, dayStart, dayEnd, sourceMaxLogId).
		Select("id, user_id, created_at, channel_id, model_name, quota, other")
	if cursorCreatedAt > 0 || cursorId > 0 {
		query = query.Where("created_at > ? OR (created_at = ? AND id > ?)", cursorCreatedAt, cursorCreatedAt, cursorId)
	}
	err := query.Order("created_at ASC, id ASC").Limit(limit).Scan(&rows).Error
	return rows, err
}

func CountSupplierHistoricalFrozenSource(ctx context.Context, logDB *gorm.DB, dayStart, dayEnd, sourceMaxLogId int64) (int64, error) {
	if logDB == nil || dayStart <= 0 || dayEnd <= dayStart || sourceMaxLogId < 0 {
		return 0, ErrSupplierHistoricalImportInvalid
	}
	var count int64
	err := supplierHistoricalSourceQuery(ctx, logDB, dayStart, dayEnd, sourceMaxLogId).Count(&count).Error
	return count, err
}

func ListSupplierHistoricalRateChains(ctx context.Context, db *gorm.DB, rateVersionIds []int) ([]SupplierHistoricalRateChain, error) {
	if db == nil || len(rateVersionIds) == 0 {
		return []SupplierHistoricalRateChain{}, nil
	}
	var rows []SupplierHistoricalRateChain
	err := db.WithContext(ctx).Table("supplier_contract_rate_versions AS rv").
		Select("rv.id AS rate_version_id, rv.contract_id, c.supplier_id, rv.procurement_multiplier_ppm").
		Joins("JOIN supplier_contracts AS c ON c.id = rv.contract_id").
		Where("rv.id IN ?", rateVersionIds).Scan(&rows).Error
	return rows, err
}
