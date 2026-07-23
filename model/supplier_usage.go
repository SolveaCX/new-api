package model

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SupplierDailyBatchStatusRunning          = "running"
	SupplierDailyBatchStatusCompleted        = "completed"
	SupplierDailyBatchStatusFailed           = "failed"
	SupplierDailyLogPageSize                 = 5000
	SupplierAccountingCoverageStartOptionKey = "supplier_accounting_v1.coverage_start_at"
)

var (
	ErrSupplierDailyBatchBusy      = errors.New("supplier daily batch is already leased")
	ErrSupplierDailyBatchFenceLost = errors.New("supplier daily batch lease fence lost")
)

// SupplierUsageDailySummary is the only supplier accounting aggregate table.
// Rows are dimensional daily aggregates; raw immutable evidence remains in
// logs.other.supplier_accounting_v1.
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

// SupplierUsageDailyBatchRun provides one unique, fenced, cross-node lease per
// Asia/Shanghai accounting date.
type SupplierUsageDailyBatchRun struct {
	Id                  int64  `json:"id"`
	BatchDate           string `json:"batch_date" gorm:"type:varchar(10);not null;uniqueIndex"`
	DayStart            int64  `json:"day_start" gorm:"not null"`
	DayEnd              int64  `json:"day_end" gorm:"not null"`
	Status              string `json:"status" gorm:"type:varchar(16);not null;index"`
	LeaseOwner          string `json:"lease_owner" gorm:"type:varchar(128);not null;default:''"`
	FenceToken          int64  `json:"fence_token" gorm:"not null;default:0"`
	PublishedFenceToken int64  `json:"published_fence_token" gorm:"not null;default:0"`
	LockedUntil         int64  `json:"locked_until" gorm:"not null;default:0"`
	CursorCreatedAt     int64  `json:"cursor_created_at" gorm:"not null;default:0"`
	CursorId            int    `json:"cursor_id" gorm:"not null;default:0"`
	LogsScanned         int64  `json:"logs_scanned" gorm:"not null;default:0"`
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
	CursorCreatedAt int64
	CursorId        int
	AlreadyDone     bool
}

// EnsureSupplierUsageGenerationSchema repairs the pre-generation unique index
// left by an earlier draft migration. AutoMigrate does not replace an existing
// same-named index when its column list changes.
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
			current, readErr := supplierUsageIndexColumns(db, indexName)
			if readErr != nil || !slices.Equal(current, expected) {
				return err
			}
			return nil
		}
	}
	if err := db.Migrator().CreateIndex(&SupplierUsageDailySummary{}, indexName); err != nil {
		current, readErr := supplierUsageIndexColumns(db, indexName)
		if readErr != nil || !slices.Equal(current, expected) {
			return err
		}
	}
	return nil
}

func supplierUsageIndexColumns(db *gorm.DB, indexName string) ([]string, error) {
	tableName := "supplier_usage_daily_summaries"
	var columns []string
	switch db.Dialector.Name() {
	case "sqlite":
		type sqliteIndexColumn struct {
			Name string
		}
		var rows []sqliteIndexColumn
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

func AcquireSupplierDailyBatch(ctx context.Context, db *gorm.DB, batchDate string, dayStart, dayEnd int64, owner string, _ time.Time, leaseDuration time.Duration, force bool) (SupplierDailyBatchLease, error) {
	if db == nil || batchDate == "" || dayStart <= 0 || dayEnd <= dayStart || owner == "" || leaseDuration <= 0 {
		return SupplierDailyBatchLease{}, ErrDatabase
	}
	var lease SupplierDailyBatchLease
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		nowUnix, err := supplierDBUnix(ctx, tx)
		if err != nil {
			return err
		}
		var run SupplierUsageDailyBatchRun
		query := tx
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
		if run.Status == SupplierDailyBatchStatusCompleted && !force {
			lease = SupplierDailyBatchLease{RunId: run.Id, BatchDate: batchDate, FenceToken: run.PublishedFenceToken, AlreadyDone: true}
			return nil
		}
		if run.Status == SupplierDailyBatchStatusRunning && run.LockedUntil >= nowUnix {
			return ErrSupplierDailyBatchBusy
		}
		fence := run.FenceToken + 1
		result := tx.Model(&SupplierUsageDailyBatchRun{}).Where("id = ? AND fence_token = ?", run.Id, run.FenceToken).Updates(map[string]any{
			"day_start": dayStart, "day_end": dayEnd, "status": SupplierDailyBatchStatusRunning,
			"lease_owner": owner, "fence_token": fence, "locked_until": nowUnix + int64(leaseDuration/time.Second),
			"cursor_created_at": 0, "cursor_id": 0, "logs_scanned": 0, "snapshot_count": 0, "summary_count": 0, "error_message": "",
			"started_at": nowUnix, "completed_at": nil,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrSupplierDailyBatchFenceLost
		}
		if err = tx.Where("batch_date = ? AND batch_fence_token <> ?", batchDate, run.PublishedFenceToken).Delete(&SupplierUsageDailySummary{}).Error; err != nil {
			return err
		}
		lease = SupplierDailyBatchLease{RunId: run.Id, BatchDate: batchDate, Owner: owner, FenceToken: fence}
		return nil
	})
	return lease, err
}

func PersistSupplierDailyBatchPage(ctx context.Context, db *gorm.DB, lease SupplierDailyBatchLease, summaries []SupplierUsageDailySummary, nextCursorCreatedAt int64, nextCursorId int, logsScanned, snapshotCount int64, leaseDuration time.Duration) error {
	if db == nil || lease.RunId <= 0 || lease.FenceToken <= 0 || lease.Owner == "" {
		return ErrSupplierDailyBatchFenceLost
	}
	if nextCursorCreatedAt < lease.CursorCreatedAt || (nextCursorCreatedAt == lease.CursorCreatedAt && nextCursorId <= lease.CursorId) || logsScanned <= 0 || logsScanned > SupplierDailyLogPageSize || snapshotCount < 0 || snapshotCount > logsScanned || leaseDuration <= 0 {
		return ErrDatabase
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run SupplierUsageDailyBatchRun
		query := tx
		if tx.Dialector.Name() != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Where("id = ? AND status = ? AND lease_owner = ? AND fence_token = ? AND cursor_created_at = ? AND cursor_id = ?", lease.RunId, SupplierDailyBatchStatusRunning, lease.Owner, lease.FenceToken, lease.CursorCreatedAt, lease.CursorId).First(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSupplierDailyBatchFenceLost
			}
			return err
		}
		for index := range summaries {
			summaries[index].BatchDate = lease.BatchDate
			summaries[index].BatchFenceToken = lease.FenceToken
		}
		if err := upsertSupplierDailySummaries(tx, summaries); err != nil {
			return err
		}
		nowUnix, err := supplierDBUnix(ctx, tx)
		if err != nil {
			return err
		}
		result := tx.Model(&SupplierUsageDailyBatchRun{}).
			Where("id = ? AND status = ? AND lease_owner = ? AND fence_token = ? AND cursor_created_at = ? AND cursor_id = ?", lease.RunId, SupplierDailyBatchStatusRunning, lease.Owner, lease.FenceToken, lease.CursorCreatedAt, lease.CursorId).
			Updates(map[string]any{
				"cursor_created_at": nextCursorCreatedAt, "cursor_id": nextCursorId,
				"logs_scanned":   gorm.Expr("logs_scanned + ?", logsScanned),
				"snapshot_count": gorm.Expr("snapshot_count + ?", snapshotCount),
				"locked_until":   nowUnix + int64(leaseDuration/time.Second),
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
	numericColumns := []string{
		"request_count", "unattributed_request_count", "official_list_known_count", "official_list_micro_usd",
		"sales_known_count", "sales_micro_usd", "procurement_cost_known_count", "procurement_cost_micro_usd",
		"gross_profit_known_count", "gross_profit_micro_usd", "gross_margin_eligible_count", "gross_margin_eligible_sales_micro_usd",
	}
	assignments := make([]clause.Assignment, 0, len(numericColumns)+1)
	for _, column := range numericColumns {
		expression := supplierDailySummaryIncrementExpression(tx.Dialector.Name(), column)
		assignments = append(assignments, clause.Assignment{Column: clause.Column{Name: column}, Value: gorm.Expr(expression)})
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
	if db == nil || lease.RunId <= 0 || lease.FenceToken <= 0 || lease.Owner == "" {
		return ErrSupplierDailyBatchFenceLost
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run SupplierUsageDailyBatchRun
		query := tx
		if tx.Dialector.Name() != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Where("id = ? AND status = ? AND lease_owner = ? AND fence_token = ?", lease.RunId, SupplierDailyBatchStatusRunning, lease.Owner, lease.FenceToken).First(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSupplierDailyBatchFenceLost
			}
			return err
		}
		var summaryCount int64
		if err := tx.Model(&SupplierUsageDailySummary{}).Where("batch_date = ? AND batch_fence_token = ?", lease.BatchDate, lease.FenceToken).Count(&summaryCount).Error; err != nil {
			return err
		}
		completedUnix := completedAt.Unix()
		result := tx.Model(&SupplierUsageDailyBatchRun{}).
			Where("id = ? AND status = ? AND lease_owner = ? AND fence_token = ?", lease.RunId, SupplierDailyBatchStatusRunning, lease.Owner, lease.FenceToken).
			Updates(map[string]any{"status": SupplierDailyBatchStatusCompleted, "published_fence_token": lease.FenceToken, "locked_until": 0, "summary_count": summaryCount, "completed_at": completedUnix})
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
		Where("id = ? AND status = ? AND lease_owner = ? AND fence_token = ?", lease.RunId, SupplierDailyBatchStatusRunning, lease.Owner, lease.FenceToken).
		Update("locked_until", nowUnix+int64(leaseDuration/time.Second))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}
	// MySQL reports zero affected rows when locked_until already equals the
	// same-second renewal value. Confirm the fenced row still exists rather than
	// weakening lease ownership or depending on clientFoundRows DSN behavior.
	var matched int64
	err = db.WithContext(ctx).Model(&SupplierUsageDailyBatchRun{}).
		Where("id = ? AND status = ? AND lease_owner = ? AND fence_token = ?", lease.RunId, SupplierDailyBatchStatusRunning, lease.Owner, lease.FenceToken).
		Count(&matched).Error
	if err != nil {
		return err
	}
	if matched != 1 {
		return ErrSupplierDailyBatchFenceLost
	}
	return nil
}

func FailSupplierDailyBatch(ctx context.Context, db *gorm.DB, lease SupplierDailyBatchLease, cause error) error {
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run SupplierUsageDailyBatchRun
		query := tx
		if tx.Dialector.Name() != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Where("id = ? AND status = ? AND lease_owner = ? AND fence_token = ?", lease.RunId, SupplierDailyBatchStatusRunning, lease.Owner, lease.FenceToken).First(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSupplierDailyBatchFenceLost
			}
			return err
		}
		result := tx.Model(&SupplierUsageDailyBatchRun{}).
			Where("id = ? AND status = ? AND lease_owner = ? AND fence_token = ?", lease.RunId, SupplierDailyBatchStatusRunning, lease.Owner, lease.FenceToken).
			Updates(map[string]any{"status": SupplierDailyBatchStatusFailed, "locked_until": 0, "error_message": message})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrSupplierDailyBatchFenceLost
		}
		if lease.FenceToken == run.PublishedFenceToken {
			return nil
		}
		return tx.Where("batch_date = ? AND batch_fence_token = ?", lease.BatchDate, lease.FenceToken).Delete(&SupplierUsageDailySummary{}).Error
	})
}

func LatestCompletedSupplierDailyBatch(ctx context.Context, db *gorm.DB) (*SupplierUsageDailyBatchRun, error) {
	var run SupplierUsageDailyBatchRun
	err := db.WithContext(ctx).Where("status = ?", SupplierDailyBatchStatusCompleted).Order("batch_date DESC").First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &run, err
}

func EarliestIncompleteSupplierDailyBatch(ctx context.Context, db *gorm.DB) (*SupplierUsageDailyBatchRun, error) {
	var run SupplierUsageDailyBatchRun
	err := db.WithContext(ctx).Where("status <> ?", SupplierDailyBatchStatusCompleted).Order("batch_date ASC").First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &run, err
}

func SupplierAccountingCoverageStart(ctx context.Context, db *gorm.DB) (int64, error) {
	if db == nil {
		return 0, ErrDatabase
	}
	var option Option
	err := db.WithContext(ctx).Where(&Option{Key: SupplierAccountingCoverageStartOptionKey}).First(&option).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	coverageStartAt, err := strconv.ParseInt(option.Value, 10, 64)
	if err != nil || coverageStartAt <= 0 {
		return 0, ErrDatabase
	}
	return coverageStartAt, nil
}

type SupplierAccountingLogRow struct {
	Id        int
	CreatedAt int64
	ChannelId int
	ModelName string
	Other     string
}

func ScanSupplierAccountingLogPage(ctx context.Context, db *gorm.DB, startAt, endAt, cursorCreatedAt int64, cursorId, pageSize int) ([]SupplierAccountingLogRow, error) {
	if db == nil || startAt <= 0 || endAt <= startAt || cursorCreatedAt < 0 || cursorId < 0 {
		return nil, ErrDatabase
	}
	if pageSize <= 0 || pageSize > SupplierDailyLogPageSize {
		pageSize = SupplierDailyLogPageSize
	}
	rows := make([]SupplierAccountingLogRow, 0, pageSize)
	query := db.WithContext(ctx).Model(&Log{}).
		Select("id", "created_at", "channel_id", "model_name", "other").
		Where("type = ? AND created_at >= ? AND created_at < ?", LogTypeConsume, startAt, endAt)
	if cursorCreatedAt > 0 {
		query = query.Where("created_at > ? OR (created_at = ? AND id > ?)", cursorCreatedAt, cursorCreatedAt, cursorId)
	}
	if err := query.Order("created_at ASC").Order("id ASC").Limit(pageSize).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func ScanSupplierAccountingLogs(ctx context.Context, db *gorm.DB, startAt, endAt int64, pageSize int, consume func([]SupplierAccountingLogRow) error) (int64, error) {
	if db == nil || startAt <= 0 || endAt <= startAt || consume == nil {
		return 0, ErrDatabase
	}
	if pageSize <= 0 || pageSize > 5000 {
		pageSize = SupplierDailyLogPageSize
	}
	var scanned int64
	var cursorCreatedAt int64
	var cursorId int
	for {
		rows, err := ScanSupplierAccountingLogPage(ctx, db, startAt, endAt, cursorCreatedAt, cursorId, pageSize)
		if err != nil {
			return scanned, err
		}
		if len(rows) == 0 {
			return scanned, nil
		}
		if err := consume(rows); err != nil {
			return scanned, err
		}
		scanned += int64(len(rows))
		last := rows[len(rows)-1]
		cursorCreatedAt, cursorId = last.CreatedAt, last.Id
		if len(rows) < pageSize {
			return scanned, nil
		}
	}
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
		return 0, ErrDatabase
	}
	return timestamp, nil
}
