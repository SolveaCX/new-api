package model

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SupplierDailyBatchStatusRunning   = "running"
	SupplierDailyBatchStatusCompleted = "completed"
	SupplierDailyBatchStatusFailed    = "failed"
)

var (
	ErrSupplierDailyBatchBusy               = errors.New("supplier daily batch is already leased")
	ErrSupplierDailyBatchFenceLost          = errors.New("supplier daily batch lease fence lost")
	ErrSupplierDailyBatchPublicationInvalid = errors.New("supplier daily batch publication is invalid")
)

type SupplierUsageDailySummary struct {
	Id                               int64  `json:"id"`
	BatchDate                        string `json:"batch_date" gorm:"type:varchar(10);not null;index:idx_supplier_daily_date_contract,priority:1;uniqueIndex:ux_supplier_daily_dimension,priority:1"`
	BatchFenceToken                  int64  `json:"batch_fence_token" gorm:"not null;default:0;uniqueIndex:ux_supplier_daily_dimension,priority:2"`
	DimensionKey                     string `json:"dimension_key" gorm:"type:varchar(64);not null;uniqueIndex:ux_supplier_daily_dimension,priority:3"`
	BucketStart                      int64  `json:"bucket_start" gorm:"not null;index"`
	SupplierId                       int    `json:"supplier_id" gorm:"not null;index"`
	ContractId                       int    `json:"contract_id" gorm:"not null;index:idx_supplier_daily_date_contract,priority:2"`
	BindingVersionId                 int    `json:"binding_version_id" gorm:"not null;default:0"`
	RateVersionId                    int    `json:"rate_version_id" gorm:"not null"`
	ChannelId                        int    `json:"channel_id" gorm:"not null;index"`
	ModelName                        string `json:"model_name" gorm:"type:varchar(191);not null;default:''"`
	SalesMultiplierPpm               *int64 `json:"sales_multiplier_ppm"`
	PricingMode                      string `json:"pricing_mode" gorm:"type:varchar(32);not null;default:''"`
	StatisticsScope                  string `json:"statistics_scope" gorm:"type:varchar(16);not null"`
	DataQuality                      string `json:"data_quality" gorm:"type:varchar(32);not null"`
	RequestCount                     int64  `json:"request_count" gorm:"not null"`
	UnattributedRequestCount         int64  `json:"unattributed_request_count" gorm:"not null"`
	OfficialListKnownCount           int64  `json:"official_list_known_count" gorm:"not null"`
	OfficialListMicroUsd             int64  `json:"official_list_micro_usd" gorm:"not null"`
	SalesKnownCount                  int64  `json:"sales_known_count" gorm:"not null"`
	SalesMicroUsd                    int64  `json:"sales_micro_usd" gorm:"not null"`
	ProcurementCostKnownCount        int64  `json:"procurement_cost_known_count" gorm:"not null"`
	ProcurementCostMicroUsd          int64  `json:"procurement_cost_micro_usd" gorm:"not null"`
	GrossProfitKnownCount            int64  `json:"gross_profit_known_count" gorm:"not null"`
	GrossProfitMicroUsd              int64  `json:"gross_profit_micro_usd" gorm:"not null"`
	GrossMarginEligibleCount         int64  `json:"gross_margin_eligible_count" gorm:"not null"`
	GrossMarginEligibleSalesMicroUsd int64  `json:"gross_margin_eligible_sales_micro_usd" gorm:"not null"`
	CreatedAt                        int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                        int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

type SupplierUsageDailyBatchRun struct {
	Id                  int64  `json:"id"`
	BatchDate           string `json:"batch_date" gorm:"type:varchar(10);not null;uniqueIndex"`
	DayStart            int64  `json:"day_start" gorm:"not null"`
	DayEnd              int64  `json:"day_end" gorm:"not null"`
	Status              string `json:"status" gorm:"type:varchar(16);not null;index"`
	LeaseOwner          string `json:"lease_owner" gorm:"type:varchar(128);not null;default:''"`
	FenceToken          int64  `json:"fence_token" gorm:"not null;default:0"`
	PublishedFenceToken int64  `json:"published_fence_token" gorm:"not null;default:0"`
	PublishedAt         *int64 `json:"published_at"`
	SourceMaxFactId     int64  `json:"source_max_fact_id" gorm:"not null;default:0"`
	CoverageScope       string `json:"coverage_scope" gorm:"type:varchar(32);not null;default:''"`
	ActiveLeaseSlot     *int   `json:"-" gorm:"uniqueIndex:ux_supplier_daily_active_lease_slot"`
	LockedUntil         int64  `json:"locked_until" gorm:"not null;default:0"`
	CursorCreatedAt     int64  `json:"cursor_created_at" gorm:"not null;default:0"`
	CursorId            int64  `json:"cursor_id" gorm:"not null;default:0"`
	FactsScanned        int64  `json:"facts_scanned" gorm:"column:logs_scanned;not null;default:0"`
	SnapshotCount       int64  `json:"snapshot_count" gorm:"not null;default:0"`
	SummaryCount        int64  `json:"summary_count" gorm:"not null;default:0"`
	ErrorMessage        string `json:"error_message" gorm:"type:text"`
	StartedAt           int64  `json:"started_at" gorm:"not null;default:0"`
	CompletedAt         *int64 `json:"completed_at"`
	CreatedAt           int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

type SupplierDailyBatchLease struct {
	RunId           int64
	BatchDate       string
	Owner           string
	FenceToken      int64
	SourceMaxFactId int64
	CoverageScope   string
	CursorId        int64
	AlreadyDone     bool
}

type SupplierAccountingFactRetentionBatch struct {
	PreparedDay     string
	SourceMaxFactId int64
}

func EnsureSupplierUsageGenerationSchema(db *gorm.DB) error {
	if db == nil {
		return ErrDatabase
	}
	const indexName = "ux_supplier_daily_dimension"
	expected := []string{"batch_date", "batch_fence_token", "dimension_key"}
	columns, err := supplierUsageIndexColumns(db, indexName)
	if err != nil {
		return err
	}
	if slices.Equal(columns, expected) {
		return nil
	}
	if len(columns) > 0 {
		if err := db.Migrator().DropIndex(&SupplierUsageDailySummary{}, indexName); err != nil {
			return err
		}
	}
	return db.Migrator().CreateIndex(&SupplierUsageDailySummary{}, indexName)
}

func supplierUsageIndexColumns(db *gorm.DB, indexName string) ([]string, error) {
	const tableName = "supplier_usage_daily_summaries"
	var columns []string
	switch db.Dialector.Name() {
	case "sqlite":
		var rows []struct{ Name string }
		if err := db.Raw("PRAGMA index_info('ux_supplier_daily_dimension')").Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			columns = append(columns, row.Name)
		}
	case "mysql":
		if err := db.Raw("SELECT column_name FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ? ORDER BY seq_in_index", tableName, indexName).Scan(&columns).Error; err != nil {
			return nil, err
		}
	case "postgres":
		query := `SELECT a.attname
			FROM pg_class t
			JOIN pg_index ix ON t.oid = ix.indrelid
			JOIN pg_class i ON i.oid = ix.indexrelid
			JOIN unnest(ix.indkey) WITH ORDINALITY AS keys(attnum, ord) ON true
			JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = keys.attnum
			WHERE t.relname = ? AND i.relname = ?
			ORDER BY keys.ord`
		if err := db.Raw(query, tableName, indexName).Scan(&columns).Error; err != nil {
			return nil, err
		}
	default:
		return nil, ErrDatabase
	}
	return columns, nil
}

func AcquireSupplierDailyBatch(ctx context.Context, db *gorm.DB, batchDate string, dayStart, dayEnd int64, owner string, leaseDuration time.Duration) (SupplierDailyBatchLease, error) {
	if db == nil || batchDate == "" || dayStart <= 0 || dayEnd <= dayStart || owner == "" || leaseDuration <= 0 {
		return SupplierDailyBatchLease{}, ErrSupplierDailyBatchPublicationInvalid
	}
	var lease SupplierDailyBatchLease
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		nowUnix, err := supplierDBUnix(ctx, tx)
		if err != nil {
			return err
		}
		var active SupplierUsageDailyBatchRun
		query := tx
		if tx.Dialector.Name() != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		err = query.Where("active_lease_slot = ?", 1).First(&active).Error
		if err == nil {
			if active.LockedUntil >= nowUnix {
				return ErrSupplierDailyBatchBusy
			}
			if err := invalidateExpiredSupplierDailyBatch(tx, active); err != nil {
				return err
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var run SupplierUsageDailyBatchRun
		query = tx
		if tx.Dialector.Name() != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		err = query.Where("batch_date = ?", batchDate).First(&run).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			run = SupplierUsageDailyBatchRun{BatchDate: batchDate, DayStart: dayStart, DayEnd: dayEnd, Status: SupplierDailyBatchStatusFailed}
			if err = tx.Create(&run).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if run.PublishedFenceToken > 0 {
			lease = SupplierDailyBatchLease{RunId: run.Id, BatchDate: batchDate, FenceToken: run.PublishedFenceToken,
				SourceMaxFactId: run.SourceMaxFactId, CoverageScope: run.CoverageScope, AlreadyDone: true}
			return nil
		}
		if run.Status == SupplierDailyBatchStatusRunning && run.LockedUntil >= nowUnix {
			return ErrSupplierDailyBatchBusy
		}
		fence := run.FenceToken + 1
		slot := 1
		result := tx.Model(&SupplierUsageDailyBatchRun{}).Where("id = ? AND fence_token = ?", run.Id, run.FenceToken).Updates(map[string]any{
			"day_start": dayStart, "day_end": dayEnd, "status": SupplierDailyBatchStatusRunning,
			"lease_owner": owner, "fence_token": fence, "locked_until": nowUnix + int64(leaseDuration/time.Second),
			"active_lease_slot": &slot, "source_max_fact_id": 0, "coverage_scope": "", "cursor_created_at": 0, "cursor_id": 0, "logs_scanned": 0,
			"snapshot_count": 0, "summary_count": 0, "error_message": "", "started_at": nowUnix, "completed_at": nil,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrSupplierDailyBatchFenceLost
		}
		lease = SupplierDailyBatchLease{RunId: run.Id, BatchDate: batchDate, Owner: owner, FenceToken: fence}
		return nil
	})
	if isSupplierDailyBatchAcquireRace(err) {
		return SupplierDailyBatchLease{}, ErrSupplierDailyBatchBusy
	}
	return lease, err
}

func AttachSupplierDailyBatchSource(ctx context.Context, db *gorm.DB, lease *SupplierDailyBatchLease, sourceMaxFactID int64, coverageScope string) error {
	if db == nil || lease == nil || lease.RunId <= 0 || lease.FenceToken <= 0 || lease.Owner == "" || sourceMaxFactID < 0 || strings.TrimSpace(coverageScope) == "" {
		return ErrSupplierDailyBatchPublicationInvalid
	}
	result := db.WithContext(ctx).Model(&SupplierUsageDailyBatchRun{}).
		Where("id = ? AND status = ? AND lease_owner = ? AND fence_token = ? AND source_max_fact_id = ? AND coverage_scope = ?",
			lease.RunId, SupplierDailyBatchStatusRunning, lease.Owner, lease.FenceToken, 0, "").
		Updates(map[string]any{"source_max_fact_id": sourceMaxFactID, "coverage_scope": coverageScope})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrSupplierDailyBatchFenceLost
	}
	lease.SourceMaxFactId = sourceMaxFactID
	lease.CoverageScope = coverageScope
	return nil
}

func invalidateExpiredSupplierDailyBatch(tx *gorm.DB, run SupplierUsageDailyBatchRun) error {
	result := tx.Model(&SupplierUsageDailyBatchRun{}).Where("id = ? AND fence_token = ? AND active_lease_slot = ?", run.Id, run.FenceToken, 1).Updates(map[string]any{
		"status": SupplierDailyBatchStatusFailed, "active_lease_slot": nil, "locked_until": 0, "lease_owner": "", "error_message": "lease expired",
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrSupplierDailyBatchBusy
	}
	return tx.Where("batch_date = ? AND batch_fence_token = ?", run.BatchDate, run.FenceToken).Delete(&SupplierUsageDailySummary{}).Error
}

func isSupplierDailyBatchAcquireRace(err error) bool {
	if err == nil || errors.Is(err, ErrSupplierDailyBatchBusy) {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, fragment := range []string{"unique constraint", "duplicate key", "duplicate entry", "serialization failure", "could not serialize", "deadlock", "database is locked", "database table is locked"} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}

func PersistSupplierDailyBatchPage(ctx context.Context, db *gorm.DB, lease SupplierDailyBatchLease, summaries []SupplierUsageDailySummary, nextCursorId, factsScanned, snapshotCount int64, leaseDuration time.Duration) error {
	if db == nil || lease.RunId <= 0 || lease.FenceToken <= 0 || lease.Owner == "" {
		return ErrSupplierDailyBatchFenceLost
	}
	if lease.SourceMaxFactId <= 0 || strings.TrimSpace(lease.CoverageScope) == "" || nextCursorId <= lease.CursorId || nextCursorId > lease.SourceMaxFactId ||
		factsScanned <= 0 || factsScanned > SupplierAccountingFactPageSize || snapshotCount < 0 || snapshotCount > factsScanned {
		return ErrDatabase
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		nowUnix, err := supplierDBUnix(ctx, tx)
		if err != nil {
			return err
		}
		for i := range summaries {
			summaries[i].BatchDate = lease.BatchDate
			summaries[i].BatchFenceToken = lease.FenceToken
		}
		if err := upsertSupplierDailySummaries(tx, summaries); err != nil {
			return err
		}
		result := tx.Model(&SupplierUsageDailyBatchRun{}).
			Where("id = ? AND status = ? AND lease_owner = ? AND fence_token = ? AND source_max_fact_id = ? AND coverage_scope = ? AND cursor_id = ? AND locked_until >= "+supplierDBUnixSQL(tx),
				lease.RunId, SupplierDailyBatchStatusRunning, lease.Owner, lease.FenceToken, lease.SourceMaxFactId, lease.CoverageScope, lease.CursorId).
			Updates(map[string]any{
				"cursor_id":    nextCursorId,
				"logs_scanned": gorm.Expr("logs_scanned + ?", factsScanned), "snapshot_count": gorm.Expr("snapshot_count + ?", snapshotCount),
				"locked_until": nowUnix + int64(leaseDuration/time.Second),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrSupplierDailyBatchFenceLost
		}
		return nil
	})
}

func upsertSupplierDailySummaries(tx *gorm.DB, summaries []SupplierUsageDailySummary) error {
	if len(summaries) == 0 {
		return nil
	}
	numeric := []string{"request_count", "unattributed_request_count", "official_list_known_count", "official_list_micro_usd", "sales_known_count", "sales_micro_usd", "procurement_cost_known_count", "procurement_cost_micro_usd", "gross_profit_known_count", "gross_profit_micro_usd", "gross_margin_eligible_count", "gross_margin_eligible_sales_micro_usd"}
	assignments := make([]clause.Assignment, 0, len(numeric)+1)
	for _, column := range numeric {
		assignments = append(assignments, clause.Assignment{Column: clause.Column{Name: column}, Value: gorm.Expr(supplierDailySummaryIncrementExpression(tx.Dialector.Name(), column))})
	}
	assignments = append(assignments, clause.Assignment{Column: clause.Column{Name: "updated_at"}, Value: gorm.Expr("?", time.Now().Unix())})
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "batch_date"}, {Name: "batch_fence_token"}, {Name: "dimension_key"}},
		DoUpdates: clause.Set(assignments),
	}).CreateInBatches(summaries, 200).Error
}

func supplierDailySummaryIncrementExpression(dialect, column string) string {
	switch dialect {
	case "postgres":
		return `"supplier_usage_daily_summaries"."` + column + `" + EXCLUDED."` + column + `"`
	case "mysql":
		return "`" + column + "` + VALUES(`" + column + "`)"
	default:
		return column + " + excluded." + column
	}
}

func CompleteSupplierDailyBatch(ctx context.Context, db *gorm.DB, lease SupplierDailyBatchLease, completedAt time.Time) error {
	if db == nil || lease.RunId <= 0 || lease.FenceToken <= 0 || lease.Owner == "" || completedAt.Unix() <= 0 {
		return ErrSupplierDailyBatchFenceLost
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		nowUnix, err := supplierDBUnix(ctx, tx)
		if err != nil {
			return err
		}
		var summaryCount int64
		if err := tx.Model(&SupplierUsageDailySummary{}).Where("batch_date = ? AND batch_fence_token = ?", lease.BatchDate, lease.FenceToken).Count(&summaryCount).Error; err != nil {
			return err
		}
		completedUnix := completedAt.Unix()
		result := tx.Model(&SupplierUsageDailyBatchRun{}).
			Where("id = ? AND status = ? AND lease_owner = ? AND fence_token = ? AND source_max_fact_id = ? AND coverage_scope = ? AND locked_until >= ?",
				lease.RunId, SupplierDailyBatchStatusRunning, lease.Owner, lease.FenceToken, lease.SourceMaxFactId, lease.CoverageScope, nowUnix).
			Updates(map[string]any{
				"status": SupplierDailyBatchStatusCompleted, "published_fence_token": lease.FenceToken, "published_at": completedUnix,
				"active_lease_slot": nil, "locked_until": 0, "lease_owner": "", "summary_count": summaryCount, "completed_at": completedUnix,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrSupplierDailyBatchFenceLost
		}
		return tx.Where("batch_date = ? AND batch_fence_token <> ?", lease.BatchDate, lease.FenceToken).Delete(&SupplierUsageDailySummary{}).Error
	})
}

func RenewSupplierDailyBatchLease(ctx context.Context, db *gorm.DB, lease SupplierDailyBatchLease, leaseDuration time.Duration) error {
	nowUnix, err := supplierDBUnix(ctx, db)
	if err != nil {
		return err
	}
	result := db.WithContext(ctx).Model(&SupplierUsageDailyBatchRun{}).
		Where("id = ? AND status = ? AND lease_owner = ? AND fence_token = ? AND locked_until >= ?", lease.RunId, SupplierDailyBatchStatusRunning, lease.Owner, lease.FenceToken, nowUnix).
		Update("locked_until", nowUnix+int64(leaseDuration/time.Second))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrSupplierDailyBatchFenceLost
	}
	return nil
}

func FailSupplierDailyBatch(ctx context.Context, db *gorm.DB, lease SupplierDailyBatchLease, cause error) error {
	if db == nil {
		return ErrSupplierDailyBatchFenceLost
	}
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&SupplierUsageDailyBatchRun{}).
			Where("id = ? AND status = ? AND lease_owner = ? AND fence_token = ?", lease.RunId, SupplierDailyBatchStatusRunning, lease.Owner, lease.FenceToken).
			Updates(map[string]any{"status": SupplierDailyBatchStatusFailed, "active_lease_slot": nil, "locked_until": 0, "lease_owner": "", "error_message": message})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrSupplierDailyBatchFenceLost
		}
		return tx.Where("batch_date = ? AND batch_fence_token = ?", lease.BatchDate, lease.FenceToken).Delete(&SupplierUsageDailySummary{}).Error
	})
}

func LatestCompletedSupplierDailyBatch(ctx context.Context, db *gorm.DB) (*SupplierUsageDailyBatchRun, error) {
	var run SupplierUsageDailyBatchRun
	err := db.WithContext(ctx).Where("published_fence_token > ?", 0).Order("batch_date DESC").First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &run, err
}

func SelectSupplierAccountingFactRetentionBatch(ctx context.Context, db *gorm.DB, retentionDays int, afterPreparedDay string) (*SupplierAccountingFactRetentionBatch, error) {
	if db == nil || retentionDays <= 0 {
		return nil, ErrDatabase
	}
	if afterPreparedDay != "" {
		parsed, err := time.ParseInLocation("2006-01-02", afterPreparedDay, supplierAccountingFactLocation)
		if err != nil || parsed.Format("2006-01-02") != afterPreparedDay {
			return nil, ErrSupplierAccountingFactResolutionInvalid
		}
	}
	nowUnix, err := supplierDBUnix(ctx, db)
	if err != nil {
		return nil, err
	}
	localNow := time.Unix(nowUnix, 0).In(supplierAccountingFactLocation)
	cutoff := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, supplierAccountingFactLocation).
		AddDate(0, 0, -retentionDays).Format("2006-01-02")
	query := db.WithContext(ctx).Model(&SupplierUsageDailyBatchRun{}).
		Select("batch_date AS prepared_day", "source_max_fact_id").
		Where("status = ? AND published_fence_token > ? AND coverage_scope = ? AND batch_date < ?",
			SupplierDailyBatchStatusCompleted, 0, string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1), cutoff)
	if afterPreparedDay != "" {
		query = query.Where("batch_date > ?", afterPreparedDay)
	}
	var batch SupplierAccountingFactRetentionBatch
	if err := query.Order("batch_date ASC").First(&batch).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &batch, nil
}

func EarliestIncompleteSupplierDailyBatch(ctx context.Context, db *gorm.DB) (*SupplierUsageDailyBatchRun, error) {
	var run SupplierUsageDailyBatchRun
	err := db.WithContext(ctx).Where("published_fence_token = ?", 0).Order("batch_date ASC").First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &run, err
}

func supplierDBUnix(ctx context.Context, db *gorm.DB) (int64, error) {
	var timestamp int64
	var err error
	switch db.Dialector.Name() {
	case "postgres":
		err = db.WithContext(ctx).Raw("SELECT EXTRACT(EPOCH FROM NOW())::bigint").Scan(&timestamp).Error
	case "sqlite":
		err = db.WithContext(ctx).Raw("SELECT strftime('%s','now')").Scan(&timestamp).Error
	default:
		err = db.WithContext(ctx).Raw("SELECT UNIX_TIMESTAMP()").Scan(&timestamp).Error
	}
	if err != nil || timestamp <= 0 {
		return 0, fmt.Errorf("read supplier database time: %w", ErrDatabase)
	}
	return timestamp, nil
}

func supplierDBUnixSQL(db *gorm.DB) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "EXTRACT(EPOCH FROM NOW())::bigint"
	case "sqlite":
		return "CAST(strftime('%s','now') AS INTEGER)"
	default:
		return "UNIX_TIMESTAMP()"
	}
}
