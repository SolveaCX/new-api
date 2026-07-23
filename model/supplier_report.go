package model

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	SupplierReportDefaultPageSize = 50
	SupplierReportMaxPageSize     = 200
	SupplierReportMaxContractRows = 5000
)

var ErrInvalidSupplierReportFilter = errors.New("invalid supplier report filter")

type SupplierReportFilter struct {
	StartAt, EndAt                       int64
	SupplierIds, ContractIds, ChannelIds []int
}

type SupplierReportPage struct{ Limit, Offset int }

func (p SupplierReportPage) Normalize() SupplierReportPage {
	if p.Limit <= 0 {
		p.Limit = SupplierReportDefaultPageSize
	}
	if p.Limit > SupplierReportMaxPageSize {
		p.Limit = SupplierReportMaxPageSize
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return p
}

func (f SupplierReportFilter) Validate() error {
	if f.StartAt <= 0 || f.EndAt <= f.StartAt || !validPositiveIds(f.SupplierIds) || !validPositiveIds(f.ContractIds) || !validPositiveIds(f.ChannelIds) {
		return ErrInvalidSupplierReportFilter
	}
	return nil
}

func validPositiveIds(ids []int) bool {
	for _, id := range ids {
		if id <= 0 {
			return false
		}
	}
	return true
}

type SupplierReportStore struct{ mainDB *gorm.DB }

func NewSupplierReportStore(mainDB *gorm.DB) *SupplierReportStore {
	return &SupplierReportStore{mainDB: mainDB}
}

func DefaultSupplierReportStore() *SupplierReportStore { return NewSupplierReportStore(DB) }

type SupplierReportContractCatalogRow struct {
	ContractId               int
	SupplierId               int
	SupplierName             string
	SupplierStatus           string
	ContractName             string
	ContractNo               string
	ContractStatus           string
	Remark                   string
	CurrentRateVersionId     *int
	ProcurementMultiplierPpm *int64
	RpmLimit                 int64
	TpmLimit                 int64
	MaxConcurrency           int
	CreatedAt                int64
	UpdatedAt                int64
}
type SupplierReportChannelCatalogRow struct {
	ChannelId          int
	ChannelName        string
	ChannelStatus      int
	SupplierContractId int
}
type SupplierReportChannelCountRow struct {
	ContractId int
	Count      int64
}
type SupplierReportInventoryAdjustmentRow struct {
	Id             int
	ContractId     int
	DeltaMicroUsd  int64
	Type           string
	Reason         string
	IdempotencyKey string
	CreatedBy      int
	CreatedAt      int64
}
type SupplierReportRateVersionRow struct {
	Id                       int
	ContractId               int
	ProcurementMultiplierPpm int64
	EffectiveAt              int64
	CreatedBy                int
	Reason                   string
	CreatedAt                int64
}

type SupplierReportBusinessUsageRow struct {
	BucketStart                      int64
	ContractId                       int
	DataQuality                      string
	BusinessRequestCount             int64
	UnattributedRequestCount         int64
	OfficialListKnownCount           int64
	OfficialListMicroUsd             int64
	SalesKnownCount                  int64
	SalesMicroUsd                    int64
	ProcurementCostKnownCount        int64
	ProcurementCostMicroUsd          int64
	GrossProfitKnownCount            int64
	GrossProfitMicroUsd              int64
	GrossMarginEligibleCount         int64
	GrossMarginEligibleSalesMicroUsd int64
}
type SupplierReportInternalUsageRow struct {
	BucketStart               int64
	ContractId                int
	DataQuality               string
	InternalRequestCount      int64
	UnattributedRequestCount  int64
	OfficialListKnownCount    int64
	OfficialListMicroUsd      int64
	ProcurementCostKnownCount int64
	ProcurementCostMicroUsd   int64
}
type SupplierReportInventoryConsumptionRow struct {
	ContractId                             int
	InventoryAffectingOfficialListMicroUsd int64
}
type SupplierReportBreakdownRow struct {
	ContractId                       int
	ChannelId                        int
	ModelName                        string
	RateVersionId                    int
	SalesMultiplierPpm               *int64
	PricingMode                      string
	DataQuality                      string
	BusinessRequestCount             int64
	UnattributedRequestCount         int64
	OfficialListKnownCount           int64
	OfficialListMicroUsd             int64
	SalesKnownCount                  int64
	SalesMicroUsd                    int64
	ProcurementCostKnownCount        int64
	ProcurementCostMicroUsd          int64
	GrossProfitKnownCount            int64
	GrossProfitMicroUsd              int64
	GrossMarginEligibleCount         int64
	GrossMarginEligibleSalesMicroUsd int64
}
type SupplierReportChannelUsageRow struct {
	ContractId                       int
	ChannelId                        int
	DataQuality                      string
	BusinessRequestCount             int64
	UnattributedRequestCount         int64
	OfficialListKnownCount           int64
	OfficialListMicroUsd             int64
	SalesKnownCount                  int64
	SalesMicroUsd                    int64
	ProcurementCostKnownCount        int64
	ProcurementCostMicroUsd          int64
	GrossProfitKnownCount            int64
	GrossProfitMicroUsd              int64
	GrossMarginEligibleCount         int64
	GrossMarginEligibleSalesMicroUsd int64
}
type SupplierReportFreshnessSnapshot struct {
	LatestBatchDate     string
	LatestStatus        string
	ErrorMessage        string
	FreshThrough        *int64
	FreshnessLagSeconds *int64
	SyncOnly            bool
	CoverageStartAt     int64
	KnownCoverageGaps   []SupplierAccountingCoverageGap
}

func (s *SupplierReportStore) ListContractCatalog(ctx context.Context, filter SupplierReportFilter, page *SupplierReportPage) ([]SupplierReportContractCatalogRow, bool, error) {
	query := s.mainDB.WithContext(ctx).Table("supplier_contracts AS c").
		Select("c.id AS contract_id, c.supplier_id, s.name AS supplier_name, s.status AS supplier_status, c.name AS contract_name, c.contract_no, c.status AS contract_status, c.remark, c.current_rate_version_id, rv.procurement_multiplier_ppm, c.rpm_limit, c.tpm_limit, c.max_concurrency, c.created_at, c.updated_at").
		Joins("JOIN upstream_suppliers AS s ON s.id = c.supplier_id").Joins("LEFT JOIN supplier_contract_rate_versions AS rv ON rv.id = c.current_rate_version_id")
	if len(filter.SupplierIds) > 0 {
		query = query.Where("c.supplier_id IN ?", filter.SupplierIds)
	}
	if len(filter.ContractIds) > 0 {
		query = query.Where("c.id IN ?", filter.ContractIds)
	}
	if len(filter.ChannelIds) > 0 {
		query = query.Where(
			"EXISTS (SELECT 1 FROM supplier_usage_daily_summaries uds JOIN supplier_usage_daily_batch_runs udr ON udr.batch_date = uds.batch_date AND udr.published_fence_token = uds.batch_fence_token WHERE uds.contract_id = c.id AND uds.bucket_start >= ? AND uds.bucket_start < ? AND uds.channel_id IN ?)",
			filter.StartAt, filter.EndAt, filter.ChannelIds,
		)
	}
	query = query.Order("c.id ASC")
	limit := SupplierReportMaxContractRows
	if page != nil {
		p := page.Normalize()
		limit = p.Limit
		query = query.Offset(p.Offset)
	}
	var rows []SupplierReportContractCatalogRow
	if err := query.Limit(limit + 1).Scan(&rows).Error; err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	return rows, hasMore, nil
}

func (s *SupplierReportStore) ListChannelCatalog(ctx context.Context, filter SupplierReportFilter, page *SupplierReportPage) ([]SupplierReportChannelCatalogRow, bool, error) {
	currentConditions := []string{"ch.supplier_contract_id IS NOT NULL"}
	currentArgs := make([]any, 0, 2)
	if len(filter.ContractIds) > 0 {
		currentConditions = append(currentConditions, "ch.supplier_contract_id IN ?")
		currentArgs = append(currentArgs, filter.ContractIds)
	}
	if len(filter.ChannelIds) > 0 {
		currentConditions = append(currentConditions, "ch.id IN ?")
		currentArgs = append(currentArgs, filter.ChannelIds)
	}

	historyConditions := []string{"uds.bucket_start >= ?", "uds.bucket_start < ?"}
	historyArgs := []any{filter.StartAt, filter.EndAt}
	if len(filter.SupplierIds) > 0 {
		historyConditions = append(historyConditions, "uds.supplier_id IN ?")
		historyArgs = append(historyArgs, filter.SupplierIds)
	}
	if len(filter.ContractIds) > 0 {
		historyConditions = append(historyConditions, "uds.contract_id IN ?")
		historyArgs = append(historyArgs, filter.ContractIds)
	}
	if len(filter.ChannelIds) > 0 {
		historyConditions = append(historyConditions, "uds.channel_id IN ?")
		historyArgs = append(historyArgs, filter.ChannelIds)
	}

	limit := SupplierReportMaxContractRows
	offset := 0
	if page != nil {
		p := page.Normalize()
		limit = p.Limit
		offset = p.Offset
	}

	// UNION (rather than UNION ALL) deduplicates an unchanged current binding
	// that also appears in the published history. The outer stable order and
	// limit keep pagination inside the database on every supported dialect.
	querySQL := `
SELECT catalog.channel_id, catalog.channel_name, catalog.channel_status, catalog.supplier_contract_id
FROM (
    SELECT ch.id AS channel_id, ch.name AS channel_name, ch.status AS channel_status, ch.supplier_contract_id
    FROM channels ch
    WHERE ` + strings.Join(currentConditions, " AND ") + `
    UNION
    SELECT DISTINCT uds.channel_id, COALESCE(ch.name, '') AS channel_name, COALESCE(ch.status, 0) AS channel_status, uds.contract_id AS supplier_contract_id
    FROM supplier_usage_daily_summaries uds
    JOIN supplier_usage_daily_batch_runs udr
      ON udr.batch_date = uds.batch_date
     AND udr.published_fence_token = uds.batch_fence_token
    LEFT JOIN channels ch ON ch.id = uds.channel_id
    WHERE ` + strings.Join(historyConditions, " AND ") + `
) catalog
ORDER BY catalog.channel_id ASC, catalog.supplier_contract_id ASC
LIMIT ? OFFSET ?`
	args := append(currentArgs, historyArgs...)
	args = append(args, limit+1, offset)
	var rows []SupplierReportChannelCatalogRow
	if err := s.mainDB.WithContext(ctx).Raw(querySQL, args...).Scan(&rows).Error; err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	return rows, hasMore, nil
}

func (s *SupplierReportStore) QueryLinkedChannelCounts(ctx context.Context, contractIds, channelIds []int) ([]SupplierReportChannelCountRow, error) {
	query := s.mainDB.WithContext(ctx).Table("channels").Select("supplier_contract_id AS contract_id, COUNT(*) AS count").Where("supplier_contract_id IN ?", contractIds)
	if len(channelIds) > 0 {
		query = query.Where("id IN ?", channelIds)
	}
	var rows []SupplierReportChannelCountRow
	err := query.Group("supplier_contract_id").Scan(&rows).Error
	return rows, err
}
func (s *SupplierReportStore) ListInventoryAdjustments(ctx context.Context, contractIds []int) ([]SupplierReportInventoryAdjustmentRow, error) {
	var rows []SupplierReportInventoryAdjustmentRow
	err := s.mainDB.WithContext(ctx).Model(&SupplierInventoryAdjustment{}).Where("contract_id IN ?", contractIds).Order("id ASC").Scan(&rows).Error
	return rows, err
}
func (s *SupplierReportStore) ListRateVersions(ctx context.Context, contractId int) ([]SupplierReportRateVersionRow, error) {
	var rows []SupplierReportRateVersionRow
	err := s.mainDB.WithContext(ctx).Model(&SupplierContractRateVersion{}).Where("contract_id = ?", contractId).Order("effective_at ASC, id ASC").Scan(&rows).Error
	return rows, err
}

func (s *SupplierReportStore) publishedSupplierSummaryQuery(ctx context.Context) *gorm.DB {
	return s.mainDB.WithContext(ctx).Table("supplier_usage_daily_summaries AS uds").
		Joins("JOIN supplier_usage_daily_batch_runs AS udr ON udr.batch_date = uds.batch_date AND udr.published_fence_token = uds.batch_fence_token")
}

func applySupplierSummaryFilter(query *gorm.DB, filter SupplierReportFilter) *gorm.DB {
	query = query.Where("uds.bucket_start >= ? AND uds.bucket_start < ?", filter.StartAt, filter.EndAt)
	if len(filter.SupplierIds) > 0 {
		query = query.Where("uds.supplier_id IN ?", filter.SupplierIds)
	}
	if len(filter.ContractIds) > 0 {
		query = query.Where("uds.contract_id IN ?", filter.ContractIds)
	}
	if len(filter.ChannelIds) > 0 {
		query = query.Where("uds.channel_id IN ?", filter.ChannelIds)
	}
	return query
}

const businessUsageSelect = "contract_id, data_quality, SUM(request_count) AS business_request_count, SUM(unattributed_request_count) AS unattributed_request_count, SUM(official_list_known_count) AS official_list_known_count, SUM(official_list_micro_usd) AS official_list_micro_usd, SUM(sales_known_count) AS sales_known_count, SUM(sales_micro_usd) AS sales_micro_usd, SUM(procurement_cost_known_count) AS procurement_cost_known_count, SUM(procurement_cost_micro_usd) AS procurement_cost_micro_usd, SUM(gross_profit_known_count) AS gross_profit_known_count, SUM(gross_profit_micro_usd) AS gross_profit_micro_usd, SUM(gross_margin_eligible_count) AS gross_margin_eligible_count, SUM(gross_margin_eligible_sales_micro_usd) AS gross_margin_eligible_sales_micro_usd"

func (s *SupplierReportStore) QueryBusinessUsage(ctx context.Context, filter SupplierReportFilter, daily bool) ([]SupplierReportBusinessUsageRow, error) {
	selectSQL := businessUsageSelect
	group := "contract_id, data_quality"
	if daily {
		selectSQL = "bucket_start, " + selectSQL
		group = "bucket_start, " + group
	}
	query := applySupplierSummaryFilter(s.publishedSupplierSummaryQuery(ctx), filter).Where("uds.statistics_scope = ?", "business")
	var rows []SupplierReportBusinessUsageRow
	err := query.Select(selectSQL).Group(group).Scan(&rows).Error
	return rows, err
}
func (s *SupplierReportStore) QueryInternalUsage(ctx context.Context, filter SupplierReportFilter, daily bool) ([]SupplierReportInternalUsageRow, error) {
	selectSQL := "contract_id, data_quality, SUM(request_count) AS internal_request_count, SUM(unattributed_request_count) AS unattributed_request_count, SUM(official_list_known_count) AS official_list_known_count, SUM(official_list_micro_usd) AS official_list_micro_usd, SUM(procurement_cost_known_count) AS procurement_cost_known_count, SUM(procurement_cost_micro_usd) AS procurement_cost_micro_usd"
	group := "contract_id, data_quality"
	if daily {
		selectSQL = "bucket_start, " + selectSQL
		group = "bucket_start, " + group
	}
	query := applySupplierSummaryFilter(s.publishedSupplierSummaryQuery(ctx), filter).Where("uds.statistics_scope = ?", "internal")
	var rows []SupplierReportInternalUsageRow
	err := query.Select(selectSQL).Group(group).Scan(&rows).Error
	return rows, err
}
func (s *SupplierReportStore) QueryInventoryConsumption(ctx context.Context, contractIds []int) ([]SupplierReportInventoryConsumptionRow, error) {
	query := s.publishedSupplierSummaryQuery(ctx).Select("uds.contract_id, SUM(uds.official_list_micro_usd) AS inventory_affecting_official_list_micro_usd")
	if len(contractIds) > 0 {
		query = query.Where("uds.contract_id IN ?", contractIds)
	}
	var rows []SupplierReportInventoryConsumptionRow
	err := query.Group("uds.contract_id").Scan(&rows).Error
	return rows, err
}
func (s *SupplierReportStore) QueryBreakdown(ctx context.Context, filter SupplierReportFilter, page SupplierReportPage) ([]SupplierReportBreakdownRow, bool, error) {
	p := page.Normalize()
	query := applySupplierSummaryFilter(s.publishedSupplierSummaryQuery(ctx), filter).Where("uds.statistics_scope = ?", "business")
	selectSQL := "contract_id, channel_id, model_name, rate_version_id, sales_multiplier_ppm, pricing_mode, data_quality, SUM(request_count) AS business_request_count, SUM(unattributed_request_count) AS unattributed_request_count, SUM(official_list_known_count) AS official_list_known_count, SUM(official_list_micro_usd) AS official_list_micro_usd, SUM(sales_known_count) AS sales_known_count, SUM(sales_micro_usd) AS sales_micro_usd, SUM(procurement_cost_known_count) AS procurement_cost_known_count, SUM(procurement_cost_micro_usd) AS procurement_cost_micro_usd, SUM(gross_profit_known_count) AS gross_profit_known_count, SUM(gross_profit_micro_usd) AS gross_profit_micro_usd, SUM(gross_margin_eligible_count) AS gross_margin_eligible_count, SUM(gross_margin_eligible_sales_micro_usd) AS gross_margin_eligible_sales_micro_usd"
	var rows []SupplierReportBreakdownRow
	err := query.Select(selectSQL).
		Group("contract_id, channel_id, model_name, rate_version_id, sales_multiplier_ppm, pricing_mode, data_quality").
		Order("uds.contract_id ASC, uds.channel_id ASC, uds.model_name ASC, uds.rate_version_id ASC, CASE WHEN uds.sales_multiplier_ppm IS NULL THEN 0 ELSE 1 END ASC, uds.sales_multiplier_ppm ASC, uds.pricing_mode ASC, uds.data_quality ASC").
		Offset(p.Offset).Limit(p.Limit + 1).Scan(&rows).Error
	if err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > p.Limit
	if hasMore {
		rows = rows[:p.Limit]
	}
	return rows, hasMore, nil
}
func (s *SupplierReportStore) QueryBreakdownEligibleCount(ctx context.Context, filter SupplierReportFilter) (int64, error) {
	var total int64
	err := applySupplierSummaryFilter(s.publishedSupplierSummaryQuery(ctx), filter).Where("uds.statistics_scope = ?", "business").Select("COALESCE(SUM(uds.request_count),0)").Scan(&total).Error
	return total, err
}
func (s *SupplierReportStore) QueryChannelUsage(ctx context.Context, filter SupplierReportFilter) ([]SupplierReportChannelUsageRow, error) {
	query := applySupplierSummaryFilter(s.publishedSupplierSummaryQuery(ctx), filter).Where("uds.statistics_scope = ?", "business")
	selectSQL := "contract_id, channel_id, data_quality, SUM(request_count) AS business_request_count, SUM(unattributed_request_count) AS unattributed_request_count, SUM(official_list_known_count) AS official_list_known_count, SUM(official_list_micro_usd) AS official_list_micro_usd, SUM(sales_known_count) AS sales_known_count, SUM(sales_micro_usd) AS sales_micro_usd, SUM(procurement_cost_known_count) AS procurement_cost_known_count, SUM(procurement_cost_micro_usd) AS procurement_cost_micro_usd, SUM(gross_profit_known_count) AS gross_profit_known_count, SUM(gross_profit_micro_usd) AS gross_profit_micro_usd, SUM(gross_margin_eligible_count) AS gross_margin_eligible_count, SUM(gross_margin_eligible_sales_micro_usd) AS gross_margin_eligible_sales_micro_usd"
	var rows []SupplierReportChannelUsageRow
	err := query.Select(selectSQL).Group("contract_id, channel_id, data_quality").Scan(&rows).Error
	return rows, err
}

func (s *SupplierReportStore) QueryCoverageGaps(ctx context.Context, startAt, endAt int64) ([]SupplierAccountingCoverageGap, error) {
	if s == nil || s.mainDB == nil {
		return nil, ErrDatabase
	}
	gaps, err := QuerySupplierAccountingCoverageGapsOverlapping(s.mainDB.WithContext(ctx), startAt, endAt)
	if err != nil {
		return nil, err
	}
	if gaps == nil {
		gaps = []SupplierAccountingCoverageGap{}
	}
	return gaps, nil
}

func (s *SupplierReportStore) QueryFreshness(ctx context.Context) (SupplierReportFreshnessSnapshot, error) {
	result := SupplierReportFreshnessSnapshot{SyncOnly: true, KnownCoverageGaps: []SupplierAccountingCoverageGap{}}
	if s == nil || s.mainDB == nil {
		return result, ErrDatabase
	}
	activation, err := ReadSupplierAccountingActivationState(s.mainDB.WithContext(ctx))
	if err != nil {
		return result, err
	}
	if activation.CutoverAt != nil {
		result.CoverageStartAt = *activation.CutoverAt
	}
	now := time.Now().Unix()
	if result.CoverageStartAt > 0 && result.CoverageStartAt < now {
		result.KnownCoverageGaps, err = s.QueryCoverageGaps(ctx, result.CoverageStartAt, now)
		if err != nil {
			return result, err
		}
	}
	var latest SupplierUsageDailyBatchRun
	err = s.mainDB.WithContext(ctx).Order("batch_date DESC").First(&latest).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return result, err
	}
	if err == nil {
		result.LatestBatchDate = latest.BatchDate
		result.LatestStatus = latest.Status
		result.ErrorMessage = latest.ErrorMessage
	}
	var completed SupplierUsageDailyBatchRun
	completedErr := s.mainDB.WithContext(ctx).
		Where("published_fence_token > 0 OR (published_fence_token = 0 AND status = ?)", SupplierDailyBatchStatusCompleted).
		Order("batch_date DESC").First(&completed).Error
	if completedErr != nil && !errors.Is(completedErr, gorm.ErrRecordNotFound) {
		return result, completedErr
	}
	if completedErr == nil {
		complete := completed.DayEnd
		lag := now - complete
		if lag < 0 {
			lag = 0
		}
		result.FreshThrough = &complete
		result.FreshnessLagSeconds = &lag
	}
	return result, nil
}
