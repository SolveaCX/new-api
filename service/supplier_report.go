package service

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/shopspring/decimal"
)

const (
	SupplierReportTimezone     = "Asia/Shanghai"
	SupplierReportMaxRangeDays = 366
)

var (
	ErrInvalidSupplierReportRange     = errors.New("invalid supplier report range")
	ErrSupplierReportOverflow         = errors.New("supplier report int64 overflow")
	ErrSupplierReportContractNotFound = errors.New("supplier report contract not found")
)

type SupplierReportQuery struct {
	Month       string
	StartDate   string
	EndDate     string
	SupplierIds []int
	ContractIds []int
	ChannelIds  []int

	eligibilityResolved bool
	hasEligibleDays     bool
	eligibleStartAt     int64
	eligibleEndAt       int64
	eligibleNow         time.Time
}

type SupplierReportRange struct {
	StartAt  int64  `json:"start_at"`
	EndAt    int64  `json:"end_at"`
	Timezone string `json:"timezone"`
	Month    string `json:"month,omitempty"`
}

func ParseSupplierReportRange(month, startDate, endDate string) (SupplierReportRange, error) {
	location, err := time.LoadLocation(SupplierReportTimezone)
	if err != nil {
		return SupplierReportRange{}, err
	}
	month = strings.TrimSpace(month)
	startDate = strings.TrimSpace(startDate)
	endDate = strings.TrimSpace(endDate)
	var start, end time.Time
	if month != "" {
		if startDate != "" || endDate != "" {
			return SupplierReportRange{}, ErrInvalidSupplierReportRange
		}
		start, err = time.ParseInLocation("2006-01", month, location)
		if err != nil {
			return SupplierReportRange{}, ErrInvalidSupplierReportRange
		}
		end = start.AddDate(0, 1, 0)
	} else {
		if startDate == "" || endDate == "" {
			return SupplierReportRange{}, ErrInvalidSupplierReportRange
		}
		start, err = time.ParseInLocation("2006-01-02", startDate, location)
		if err != nil {
			return SupplierReportRange{}, ErrInvalidSupplierReportRange
		}
		inclusiveEnd, parseErr := time.ParseInLocation("2006-01-02", endDate, location)
		if parseErr != nil || inclusiveEnd.Before(start) {
			return SupplierReportRange{}, ErrInvalidSupplierReportRange
		}
		end = inclusiveEnd.AddDate(0, 0, 1)
	}
	if !end.After(start) || end.Sub(start) > SupplierReportMaxRangeDays*24*time.Hour {
		return SupplierReportRange{}, ErrInvalidSupplierReportRange
	}
	return SupplierReportRange{StartAt: start.UTC().Unix(), EndAt: end.UTC().Unix(), Timezone: SupplierReportTimezone, Month: month}, nil
}

func (q SupplierReportQuery) modelFilter() (SupplierReportRange, model.SupplierReportFilter, error) {
	reportRange, err := ParseSupplierReportRange(q.Month, q.StartDate, q.EndDate)
	if err != nil {
		return SupplierReportRange{}, model.SupplierReportFilter{}, err
	}
	filter := model.SupplierReportFilter{
		StartAt: reportRange.StartAt, EndAt: reportRange.EndAt,
		SupplierIds: q.SupplierIds, ContractIds: q.ContractIds, ChannelIds: q.ChannelIds,
	}
	if err := filter.Validate(); err != nil {
		return SupplierReportRange{}, model.SupplierReportFilter{}, err
	}
	if q.eligibilityResolved {
		filter.StartAt = q.eligibleStartAt
		filter.EndAt = q.eligibleEndAt
	}
	return reportRange, filter, nil
}

type SupplierReportMoney struct {
	KnownCount int64 `json:"known_count"`
	MicroUsd   int64 `json:"micro_usd"`
}

type SupplierReportMetrics struct {
	RequestCount                     int64               `json:"request_count"`
	UnattributedRequestCount         int64               `json:"unattributed_request_count"`
	OfficialList                     SupplierReportMoney `json:"official_list"`
	Sales                            SupplierReportMoney `json:"sales"`
	ProcurementCost                  SupplierReportMoney `json:"procurement_cost"`
	GrossProfit                      SupplierReportMoney `json:"gross_profit"`
	GrossMarginEligibleCount         int64               `json:"gross_margin_eligible_count"`
	GrossMarginEligibleSalesMicroUsd int64               `json:"gross_margin_eligible_sales_micro_usd"`
	GrossMargin                      *string             `json:"gross_margin"`
	GrossMarginEligibleCoverage      *string             `json:"gross_margin_eligible_coverage"`
}

type SupplierReportFreshness struct {
	LatestBatchDate          string                                `json:"latest_batch_date"`
	BatchStatus              string                                `json:"batch_status"`
	FreshThrough             *int64                                `json:"fresh_through"`
	FreshnessLagSeconds      *int64                                `json:"freshness_lag_seconds"`
	ErrorMessage             string                                `json:"error_message"`
	SyncOnly                 bool                                  `json:"sync_only"`
	CoverageStartAt          int64                                 `json:"coverage_start_at"`
	KnownCoverageGaps        []model.SupplierAccountingCoverageGap `json:"known_coverage_gaps"`
	PublishedEvidence        SupplierReportPublishedEvidence       `json:"published_evidence"`
	FinanceAttentionRequired bool                                  `json:"finance_attention_required"`
}

type SupplierReportPublishedEvidence struct {
	PublishedDays                    int                                        `json:"published_days"`
	ExpectedDays                     int                                        `json:"expected_days"`
	LogsScanned                      int64                                      `json:"logs_scanned"`
	ProducerMarkersPresent           int64                                      `json:"producer_markers_present"`
	CapturedSnapshotCount            int64                                      `json:"captured_snapshot_count"`
	DispositionCounts                types.SupplierPublishedDispositionCountsV1 `json:"disposition_counts"`
	FailureCounts                    types.SupplierPublishedFailureCountsV1     `json:"failure_counts"`
	PersistedLogSnapshotCompleteness string                                     `json:"persisted_log_snapshot_completeness"`
	Warnings                         []types.SupplierPublishedWarningV1         `json:"warnings"`
}

type SupplierReportOverview struct {
	Range                        SupplierReportRange                   `json:"range"`
	KnownCoverageGaps            []model.SupplierAccountingCoverageGap `json:"known_coverage_gaps"`
	Business                     SupplierReportMetrics                 `json:"business"`
	Internal                     SupplierReportMetrics                 `json:"internal"`
	TotalProcurementCost         SupplierReportMoney                   `json:"total_estimated_procurement_cost"`
	TotalInventoryMicroUsd       int64                                 `json:"total_inventory_micro_usd"`
	OfficialListConsumedMicroUsd int64                                 `json:"official_list_consumed_micro_usd"`
	RemainingInventoryMicroUsd   int64                                 `json:"remaining_inventory_micro_usd"`
	InternalDimensionAvailable   bool                                  `json:"internal_dimension_available"`
	PublishedEvidence            SupplierReportPublishedEvidence       `json:"published_evidence"`
	FinanceAttentionRequired     bool                                  `json:"finance_attention_required"`
}

type SupplierReportTrendPoint struct {
	BucketStart                int64                 `json:"bucket_start"`
	Date                       string                `json:"date"`
	Business                   SupplierReportMetrics `json:"business"`
	Internal                   SupplierReportMetrics `json:"internal"`
	InternalDimensionAvailable bool                  `json:"internal_dimension_available"`
}

type SupplierReportTrend struct {
	Range                    SupplierReportRange                   `json:"range"`
	KnownCoverageGaps        []model.SupplierAccountingCoverageGap `json:"known_coverage_gaps"`
	Points                   []SupplierReportTrendPoint            `json:"points"`
	PublishedEvidence        SupplierReportPublishedEvidence       `json:"published_evidence"`
	FinanceAttentionRequired bool                                  `json:"finance_attention_required"`
}

type SupplierReportContractRow struct {
	ContractId                   int                   `json:"contract_id"`
	SupplierId                   int                   `json:"supplier_id"`
	SupplierName                 string                `json:"supplier_name"`
	SupplierStatus               string                `json:"supplier_status"`
	ContractName                 string                `json:"contract_name"`
	ContractNo                   string                `json:"contract_no"`
	ContractStatus               string                `json:"contract_status"`
	Remark                       string                `json:"remark"`
	CurrentRateVersionId         *int                  `json:"current_rate_version_id"`
	ProcurementMultiplierPpm     *int64                `json:"procurement_multiplier_ppm"`
	RpmLimit                     int64                 `json:"rpm_limit"`
	TpmLimit                     int64                 `json:"tpm_limit"`
	MaxConcurrency               int                   `json:"max_concurrency"`
	LinkedChannelCount           int                   `json:"linked_channel_count"`
	TotalInventoryMicroUsd       int64                 `json:"total_inventory_micro_usd"`
	OfficialListConsumedMicroUsd int64                 `json:"official_list_consumed_micro_usd"`
	RemainingInventoryMicroUsd   int64                 `json:"remaining_inventory_micro_usd"`
	UtilizationRate              *string               `json:"utilization_rate"`
	Oversold                     bool                  `json:"oversold"`
	Business                     SupplierReportMetrics `json:"business"`
	Internal                     SupplierReportMetrics `json:"internal"`
	TotalProcurementCost         SupplierReportMoney   `json:"total_estimated_procurement_cost"`
	InternalDimensionAvailable   bool                  `json:"internal_dimension_available"`
}

type SupplierReportContractList struct {
	Range                    SupplierReportRange                   `json:"range"`
	KnownCoverageGaps        []model.SupplierAccountingCoverageGap `json:"known_coverage_gaps"`
	Items                    []SupplierReportContractRow           `json:"items"`
	Limit                    int                                   `json:"limit"`
	Offset                   int                                   `json:"offset"`
	HasMore                  bool                                  `json:"has_more"`
	PublishedEvidence        SupplierReportPublishedEvidence       `json:"published_evidence"`
	FinanceAttentionRequired bool                                  `json:"finance_attention_required"`
}

type SupplierReportRateVersion struct {
	Id                       int    `json:"id"`
	ProcurementMultiplierPpm int64  `json:"procurement_multiplier_ppm"`
	EffectiveAt              int64  `json:"effective_at"`
	CreatedBy                int    `json:"created_by"`
	Reason                   string `json:"reason"`
	CreatedAt                int64  `json:"created_at"`
}

type SupplierReportInventoryAdjustment struct {
	Id             int    `json:"id"`
	DeltaMicroUsd  int64  `json:"delta_micro_usd"`
	Type           string `json:"type"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotency_key"`
	CreatedBy      int    `json:"created_by"`
	CreatedAt      int64  `json:"created_at"`
}

type SupplierReportChannelRow struct {
	ChannelId     int                   `json:"channel_id"`
	ChannelName   string                `json:"channel_name"`
	ChannelStatus int                   `json:"channel_status"`
	ContractId    int                   `json:"contract_id"`
	Business      SupplierReportMetrics `json:"business"`
}

type SupplierReportChannelList struct {
	Range                    SupplierReportRange                   `json:"range"`
	KnownCoverageGaps        []model.SupplierAccountingCoverageGap `json:"known_coverage_gaps"`
	Items                    []SupplierReportChannelRow            `json:"items"`
	Limit                    int                                   `json:"limit"`
	Offset                   int                                   `json:"offset"`
	HasMore                  bool                                  `json:"has_more"`
	PublishedEvidence        SupplierReportPublishedEvidence       `json:"published_evidence"`
	FinanceAttentionRequired bool                                  `json:"finance_attention_required"`
}

type SupplierReportBreakdownItem struct {
	ContractId         int                   `json:"contract_id"`
	ChannelId          int                   `json:"channel_id"`
	ModelName          string                `json:"model_name"`
	RateVersionId      int                   `json:"rate_version_id"`
	SalesMultiplierPpm *int64                `json:"sales_multiplier_ppm"`
	PricingMode        string                `json:"pricing_mode"`
	DataQuality        string                `json:"data_quality"`
	Metrics            SupplierReportMetrics `json:"metrics"`
}

type SupplierReportBreakdownList struct {
	Range                      SupplierReportRange                   `json:"range"`
	KnownCoverageGaps          []model.SupplierAccountingCoverageGap `json:"known_coverage_gaps"`
	Items                      []SupplierReportBreakdownItem         `json:"items"`
	Limit                      int                                   `json:"limit"`
	Offset                     int                                   `json:"offset"`
	HasMore                    bool                                  `json:"has_more"`
	BreakdownEligibleCount     int64                                 `json:"breakdown_eligible_count"`
	TotalBusinessCount         int64                                 `json:"total_business_count"`
	BreakdownCoverageRate      *string                               `json:"breakdown_coverage_rate"`
	BreakdownCoverageAvailable bool                                  `json:"breakdown_coverage_available"`
	PublishedEvidence          SupplierReportPublishedEvidence       `json:"published_evidence"`
	FinanceAttentionRequired   bool                                  `json:"finance_attention_required"`
}

type SupplierReportContractDetail struct {
	Range                    SupplierReportRange                   `json:"range"`
	KnownCoverageGaps        []model.SupplierAccountingCoverageGap `json:"known_coverage_gaps"`
	Summary                  SupplierReportContractRow             `json:"summary"`
	RateVersions             []SupplierReportRateVersion           `json:"rate_versions"`
	InventoryAdjustments     []SupplierReportInventoryAdjustment   `json:"inventory_adjustments"`
	Channels                 SupplierReportChannelList             `json:"channels"`
	InternalTrend            []SupplierReportTrendPoint            `json:"internal_trend"`
	Breakdown                SupplierReportBreakdownList           `json:"breakdown"`
	PublishedEvidence        SupplierReportPublishedEvidence       `json:"published_evidence"`
	FinanceAttentionRequired bool                                  `json:"finance_attention_required"`
}

type SupplierReportService struct {
	store *model.SupplierReportStore
	now   func() time.Time
}

func NewSupplierReportService(store *model.SupplierReportStore) *SupplierReportService {
	return &SupplierReportService{store: store, now: time.Now}
}

func DefaultSupplierReportService() *SupplierReportService {
	return NewSupplierReportService(model.DefaultSupplierReportStore())
}

func withSupplierReportSnapshot[T any](ctx context.Context, service *SupplierReportService, read func(*SupplierReportService) (T, error)) (T, error) {
	var result T
	if service == nil || service.store == nil || read == nil {
		return result, model.ErrDatabase
	}
	err := service.store.ReadSnapshot(ctx, func(store *model.SupplierReportStore) error {
		snapshotService := &SupplierReportService{store: store, now: service.now}
		var readErr error
		result, readErr = read(snapshotService)
		return readErr
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return result, nil
}

func (s *SupplierReportService) prepareQuery(query SupplierReportQuery) (SupplierReportQuery, error) {
	if query.eligibilityResolved {
		return query, nil
	}
	reportRange, err := ParseSupplierReportRange(query.Month, query.StartDate, query.EndDate)
	if err != nil {
		return SupplierReportQuery{}, err
	}
	now := time.Now()
	if s != nil && s.now != nil {
		now = s.now()
	}
	eligibleRange, hasEligibleDays, err := supplierReportEligibleRange(reportRange, now)
	if err != nil {
		return SupplierReportQuery{}, err
	}
	query.eligibilityResolved = true
	query.hasEligibleDays = hasEligibleDays
	query.eligibleStartAt = eligibleRange.StartAt
	query.eligibleEndAt = eligibleRange.EndAt
	query.eligibleNow = now
	return query, nil
}

func (s *SupplierReportService) GetOverview(ctx context.Context, query SupplierReportQuery) (SupplierReportOverview, error) {
	var err error
	query, err = s.prepareQuery(query)
	if err != nil {
		return SupplierReportOverview{}, err
	}
	return withSupplierReportSnapshot(ctx, s, func(snapshot *SupplierReportService) (SupplierReportOverview, error) {
		return snapshot.getOverview(ctx, query, nil)
	})
}

func (s *SupplierReportService) getOverview(ctx context.Context, query SupplierReportQuery, coverage *supplierReportCoverageProjection) (SupplierReportOverview, error) {
	reportRange, filter, catalog, _, err := s.resolveContracts(ctx, query, nil)
	if err != nil {
		return SupplierReportOverview{}, err
	}
	coverage, err = s.resolveCoverageProjectionForQuery(ctx, query, reportRange, coverage)
	if err != nil {
		return SupplierReportOverview{}, err
	}
	result := SupplierReportOverview{
		Range: reportRange, KnownCoverageGaps: coverage.gaps,
		InternalDimensionAvailable: len(filter.ChannelIds) == 0,
		PublishedEvidence:          coverage.evidence, FinanceAttentionRequired: coverage.financeAttentionRequired,
	}
	if len(catalog) == 0 {
		return result, nil
	}
	usage, err := s.loadUsage(ctx, filter, false)
	if err != nil {
		return SupplierReportOverview{}, err
	}
	runtime, err := s.loadInventoryRuntime(ctx, catalog, filter.ChannelIds, filter.EndAt)
	if err != nil {
		return SupplierReportOverview{}, err
	}
	result.Business = usage.business.metrics()
	result.Internal = usage.internal.metrics()
	result.InternalDimensionAvailable = usage.internalDimensionAvailable
	result.TotalProcurementCost, err = combinedSupplierReportMoney(result.Business.ProcurementCost, result.Internal.ProcurementCost)
	if err != nil {
		return SupplierReportOverview{}, err
	}
	for _, contract := range catalog {
		contractRuntime := runtime[contract.ContractId]
		if result.TotalInventoryMicroUsd, err = checkedAddInt64(result.TotalInventoryMicroUsd, contractRuntime.inventory); err != nil {
			return SupplierReportOverview{}, err
		}
		if result.OfficialListConsumedMicroUsd, err = checkedAddInt64(result.OfficialListConsumedMicroUsd, contractRuntime.consumed); err != nil {
			return SupplierReportOverview{}, err
		}
	}
	result.RemainingInventoryMicroUsd, err = checkedSubInt64(result.TotalInventoryMicroUsd, result.OfficialListConsumedMicroUsd)
	if err != nil {
		return SupplierReportOverview{}, err
	}
	return result, nil
}

func (s *SupplierReportService) GetTrend(ctx context.Context, query SupplierReportQuery) (SupplierReportTrend, error) {
	var err error
	query, err = s.prepareQuery(query)
	if err != nil {
		return SupplierReportTrend{}, err
	}
	return withSupplierReportSnapshot(ctx, s, func(snapshot *SupplierReportService) (SupplierReportTrend, error) {
		return snapshot.getTrend(ctx, query, nil)
	})
}

func (s *SupplierReportService) getTrend(ctx context.Context, query SupplierReportQuery, coverage *supplierReportCoverageProjection) (SupplierReportTrend, error) {
	reportRange, filter, catalog, _, err := s.resolveContracts(ctx, query, nil)
	if err != nil {
		return SupplierReportTrend{}, err
	}
	coverage, err = s.resolveCoverageProjectionForQuery(ctx, query, reportRange, coverage)
	if err != nil {
		return SupplierReportTrend{}, err
	}
	result := SupplierReportTrend{Range: reportRange, KnownCoverageGaps: coverage.gaps, Points: []SupplierReportTrendPoint{}, PublishedEvidence: coverage.evidence, FinanceAttentionRequired: coverage.financeAttentionRequired}
	if len(catalog) == 0 {
		return result, nil
	}
	usage, err := s.loadUsage(ctx, filter, true)
	if err != nil {
		return SupplierReportTrend{}, err
	}
	location, _ := time.LoadLocation(SupplierReportTimezone)
	byDay := make(map[int64]*usageAccumulator)
	for _, row := range usage.businessRows {
		day := byDay[row.BucketStart]
		if day == nil {
			day = newUsageAccumulator(usage.internalDimensionAvailable)
			byDay[row.BucketStart] = day
		}
		if err := day.addBusiness(row); err != nil {
			return SupplierReportTrend{}, err
		}
	}
	for _, row := range usage.internalRows {
		day := byDay[row.BucketStart]
		if day == nil {
			day = newUsageAccumulator(usage.internalDimensionAvailable)
			byDay[row.BucketStart] = day
		}
		if err := day.addInternal(row); err != nil {
			return SupplierReportTrend{}, err
		}
	}
	for bucket := reportRange.StartAt; bucket < reportRange.EndAt; {
		day := byDay[bucket]
		if day == nil {
			day = newUsageAccumulator(usage.internalDimensionAvailable)
		}
		result.Points = append(result.Points, SupplierReportTrendPoint{
			BucketStart: bucket, Date: time.Unix(bucket, 0).In(location).Format("2006-01-02"),
			Business: day.business.metrics(), Internal: day.internal.metrics(),
			InternalDimensionAvailable: day.internalDimensionAvailable,
		})
		next := time.Unix(bucket, 0).In(location).AddDate(0, 0, 1)
		bucket = next.UTC().Unix()
	}
	return result, nil
}

func (s *SupplierReportService) ListContracts(ctx context.Context, query SupplierReportQuery, page model.SupplierReportPage) (SupplierReportContractList, error) {
	var err error
	query, err = s.prepareQuery(query)
	if err != nil {
		return SupplierReportContractList{}, err
	}
	return withSupplierReportSnapshot(ctx, s, func(snapshot *SupplierReportService) (SupplierReportContractList, error) {
		return snapshot.listContracts(ctx, query, page, nil)
	})
}

func (s *SupplierReportService) listContracts(ctx context.Context, query SupplierReportQuery, page model.SupplierReportPage, coverage *supplierReportCoverageProjection) (SupplierReportContractList, error) {
	reportRange, filter, catalog, hasMore, err := s.resolveContracts(ctx, query, &page)
	if err != nil {
		return SupplierReportContractList{}, err
	}
	coverage, err = s.resolveCoverageProjectionForQuery(ctx, query, reportRange, coverage)
	if err != nil {
		return SupplierReportContractList{}, err
	}
	page = page.Normalize()
	result := SupplierReportContractList{Range: reportRange, KnownCoverageGaps: coverage.gaps, Limit: page.Limit, Offset: page.Offset, Items: []SupplierReportContractRow{}, PublishedEvidence: coverage.evidence, FinanceAttentionRequired: coverage.financeAttentionRequired}
	result.HasMore = hasMore
	if len(catalog) == 0 {
		return result, nil
	}
	filter.ContractIds = catalogContractIds(catalog)
	usage, err := s.loadUsage(ctx, filter, false)
	if err != nil {
		return SupplierReportContractList{}, err
	}
	runtime, err := s.loadInventoryRuntime(ctx, catalog, filter.ChannelIds, filter.EndAt)
	if err != nil {
		return SupplierReportContractList{}, err
	}
	byContract, err := usage.byContract()
	if err != nil {
		return SupplierReportContractList{}, err
	}
	for _, contract := range catalog {
		row, buildErr := buildContractRow(contract, runtime[contract.ContractId], byContract[contract.ContractId])
		if buildErr != nil {
			return SupplierReportContractList{}, buildErr
		}
		result.Items = append(result.Items, row)
	}
	return result, nil
}

func (s *SupplierReportService) GetContractDetail(ctx context.Context, contractId int, query SupplierReportQuery, page model.SupplierReportPage) (SupplierReportContractDetail, error) {
	if contractId <= 0 {
		return SupplierReportContractDetail{}, model.ErrInvalidSupplierReportFilter
	}
	query.ContractIds = []int{contractId}
	query, err := s.prepareQuery(query)
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	return withSupplierReportSnapshot(ctx, s, func(snapshot *SupplierReportService) (SupplierReportContractDetail, error) {
		return snapshot.getContractDetail(ctx, contractId, query, page)
	})
}

func (s *SupplierReportService) getContractDetail(ctx context.Context, contractId int, query SupplierReportQuery, page model.SupplierReportPage) (SupplierReportContractDetail, error) {
	reportRange, _, err := query.modelFilter()
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	coverage, err := s.resolveCoverageProjectionForQuery(ctx, query, reportRange, nil)
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	list, err := s.listContracts(ctx, query, model.SupplierReportPage{Limit: 1}, coverage)
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	if len(list.Items) == 0 {
		return SupplierReportContractDetail{}, ErrSupplierReportContractNotFound
	}
	rates, err := s.store.ListRateVersions(ctx, contractId, query.eligibleEndAt)
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	adjustments, err := s.store.ListInventoryAdjustments(ctx, []int{contractId}, query.eligibleEndAt)
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	channels, err := s.listChannels(ctx, query, model.SupplierReportPage{Limit: model.SupplierReportMaxPageSize}, coverage)
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	breakdown, err := s.listBreakdown(ctx, query, page, coverage)
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	trend, err := s.getTrend(ctx, query, coverage)
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	result := SupplierReportContractDetail{Range: list.Range, KnownCoverageGaps: coverage.gaps, Summary: list.Items[0], Channels: channels, Breakdown: breakdown, PublishedEvidence: coverage.evidence, FinanceAttentionRequired: coverage.financeAttentionRequired}
	for _, rate := range rates {
		result.RateVersions = append(result.RateVersions, SupplierReportRateVersion{Id: rate.Id, ProcurementMultiplierPpm: rate.ProcurementMultiplierPpm, EffectiveAt: rate.EffectiveAt, CreatedBy: rate.CreatedBy, Reason: rate.Reason, CreatedAt: rate.CreatedAt})
	}
	for _, adjustment := range adjustments {
		result.InventoryAdjustments = append(result.InventoryAdjustments, SupplierReportInventoryAdjustment{Id: adjustment.Id, DeltaMicroUsd: adjustment.DeltaMicroUsd, Type: adjustment.Type, Reason: adjustment.Reason, IdempotencyKey: adjustment.IdempotencyKey, CreatedBy: adjustment.CreatedBy, CreatedAt: adjustment.CreatedAt})
	}
	for _, point := range trend.Points {
		point.Business = SupplierReportMetrics{}
		result.InternalTrend = append(result.InternalTrend, point)
	}
	return result, nil
}

func (s *SupplierReportService) ListChannels(ctx context.Context, query SupplierReportQuery, page model.SupplierReportPage) (SupplierReportChannelList, error) {
	var err error
	query, err = s.prepareQuery(query)
	if err != nil {
		return SupplierReportChannelList{}, err
	}
	return withSupplierReportSnapshot(ctx, s, func(snapshot *SupplierReportService) (SupplierReportChannelList, error) {
		return snapshot.listChannels(ctx, query, page, nil)
	})
}

func (s *SupplierReportService) listChannels(ctx context.Context, query SupplierReportQuery, page model.SupplierReportPage, coverage *supplierReportCoverageProjection) (SupplierReportChannelList, error) {
	reportRange, filter, catalog, _, err := s.resolveContracts(ctx, query, nil)
	if err != nil {
		return SupplierReportChannelList{}, err
	}
	coverage, err = s.resolveCoverageProjectionForQuery(ctx, query, reportRange, coverage)
	if err != nil {
		return SupplierReportChannelList{}, err
	}
	page = page.Normalize()
	result := SupplierReportChannelList{Range: reportRange, KnownCoverageGaps: coverage.gaps, Limit: page.Limit, Offset: page.Offset, Items: []SupplierReportChannelRow{}, PublishedEvidence: coverage.evidence, FinanceAttentionRequired: coverage.financeAttentionRequired}
	if len(catalog) == 0 {
		return result, nil
	}
	filter.ContractIds = catalogContractIds(catalog)
	channels, hasMore, err := s.store.ListChannelCatalog(ctx, filter, &page)
	if err != nil {
		return SupplierReportChannelList{}, err
	}
	result.HasMore = hasMore
	filter.ChannelIds = make([]int, len(channels))
	for i := range channels {
		filter.ChannelIds[i] = channels[i].ChannelId
	}
	usageRows, err := s.store.QueryChannelUsage(ctx, filter)
	if err != nil {
		return SupplierReportChannelList{}, err
	}
	type channelContractKey struct {
		contractId int
		channelId  int
	}
	byChannel := make(map[channelContractKey]*usageAccumulator, len(channels))
	for _, row := range usageRows {
		key := channelContractKey{contractId: row.ContractId, channelId: row.ChannelId}
		accumulator := byChannel[key]
		if accumulator == nil {
			accumulator = newUsageAccumulator(false)
			byChannel[key] = accumulator
		}
		if err := accumulator.addChannel(row); err != nil {
			return SupplierReportChannelList{}, err
		}
	}
	for _, channel := range channels {
		usage := byChannel[channelContractKey{contractId: channel.SupplierContractId, channelId: channel.ChannelId}]
		if usage == nil {
			usage = newUsageAccumulator(false)
		}
		result.Items = append(result.Items, SupplierReportChannelRow{
			ChannelId: channel.ChannelId, ChannelName: channel.ChannelName, ChannelStatus: channel.ChannelStatus, ContractId: channel.SupplierContractId,
			Business: usage.business.metrics(),
		})
	}
	return result, nil
}

func (s *SupplierReportService) ListBreakdown(ctx context.Context, query SupplierReportQuery, page model.SupplierReportPage) (SupplierReportBreakdownList, error) {
	var err error
	query, err = s.prepareQuery(query)
	if err != nil {
		return SupplierReportBreakdownList{}, err
	}
	return withSupplierReportSnapshot(ctx, s, func(snapshot *SupplierReportService) (SupplierReportBreakdownList, error) {
		return snapshot.listBreakdown(ctx, query, page, nil)
	})
}

func (s *SupplierReportService) listBreakdown(ctx context.Context, query SupplierReportQuery, page model.SupplierReportPage, coverage *supplierReportCoverageProjection) (SupplierReportBreakdownList, error) {
	reportRange, filter, catalog, _, err := s.resolveContracts(ctx, query, nil)
	if err != nil {
		return SupplierReportBreakdownList{}, err
	}
	coverage, err = s.resolveCoverageProjectionForQuery(ctx, query, reportRange, coverage)
	if err != nil {
		return SupplierReportBreakdownList{}, err
	}
	page = page.Normalize()
	result := SupplierReportBreakdownList{Range: reportRange, KnownCoverageGaps: coverage.gaps, Limit: page.Limit, Offset: page.Offset, Items: []SupplierReportBreakdownItem{}, PublishedEvidence: coverage.evidence, FinanceAttentionRequired: coverage.financeAttentionRequired}
	if len(catalog) == 0 {
		return result, nil
	}
	filter.ContractIds = catalogContractIds(catalog)
	result.BreakdownCoverageAvailable = len(filter.ChannelIds) == 0
	if result.BreakdownCoverageAvailable {
		businessRows, usageErr := s.store.QueryBusinessUsage(ctx, filter, false)
		if usageErr != nil {
			return SupplierReportBreakdownList{}, usageErr
		}
		for _, row := range businessRows {
			result.TotalBusinessCount, err = checkedAddInt64(result.TotalBusinessCount, row.BusinessRequestCount)
			if err != nil {
				return SupplierReportBreakdownList{}, err
			}
		}
		result.BreakdownEligibleCount, err = s.store.QueryBreakdownEligibleCount(ctx, filter)
		if err != nil {
			return SupplierReportBreakdownList{}, err
		}
		result.BreakdownCoverageRate = ratioString(result.BreakdownEligibleCount, result.TotalBusinessCount)
	}
	rows, hasMore, err := s.store.QueryBreakdown(ctx, filter, page)
	if err != nil {
		return SupplierReportBreakdownList{}, err
	}
	result.HasMore = hasMore
	for _, row := range rows {
		metrics := metricAccumulator{}
		if err := metrics.addBusinessFields(row.BusinessRequestCount, row.UnattributedRequestCount, row.OfficialListKnownCount, row.OfficialListMicroUsd, row.SalesKnownCount, row.SalesMicroUsd, row.ProcurementCostKnownCount, row.ProcurementCostMicroUsd, row.GrossProfitKnownCount, row.GrossProfitMicroUsd, row.GrossMarginEligibleCount, row.GrossMarginEligibleSalesMicroUsd); err != nil {
			return SupplierReportBreakdownList{}, err
		}
		result.Items = append(result.Items, SupplierReportBreakdownItem{
			ContractId: row.ContractId, ChannelId: row.ChannelId, ModelName: row.ModelName, RateVersionId: row.RateVersionId,
			SalesMultiplierPpm: row.SalesMultiplierPpm, PricingMode: row.PricingMode, DataQuality: row.DataQuality,
			Metrics: metrics.metrics(),
		})
	}
	return result, nil
}

func (s *SupplierReportService) GetFreshness(ctx context.Context) (SupplierReportFreshness, error) {
	now := time.Now()
	return withSupplierReportSnapshot(ctx, s, func(snapshot *SupplierReportService) (SupplierReportFreshness, error) {
		return snapshot.getFreshnessAt(ctx, now)
	})
}

func (s *SupplierReportService) getFreshnessAt(ctx context.Context, now time.Time) (SupplierReportFreshness, error) {
	if s == nil || s.store == nil {
		return SupplierReportFreshness{}, model.ErrDatabase
	}
	snapshot, err := s.store.QueryFreshness(ctx)
	if err != nil {
		return SupplierReportFreshness{}, err
	}
	result := SupplierReportFreshness{SyncOnly: snapshot.SyncOnly, CoverageStartAt: snapshot.CoverageStartAt}
	if snapshot.CoverageStartAt <= 0 {
		result.PublishedEvidence.PersistedLogSnapshotCompleteness = types.SupplierPersistedLogCompletenessNotScanned
		result.PublishedEvidence.Warnings = []types.SupplierPublishedWarningV1{}
		result.KnownCoverageGaps = []model.SupplierAccountingCoverageGap{}
		result.FinanceAttentionRequired = false
		return result, nil
	}
	nowUnix := now.Unix()
	reportRange := SupplierReportRange{StartAt: snapshot.CoverageStartAt, EndAt: nowUnix, Timezone: SupplierReportTimezone}
	eligibleRange, hasEligibleDays, err := supplierReportEligibleRange(reportRange, now)
	if err != nil {
		return SupplierReportFreshness{}, err
	}
	var published []model.SupplierPublishedDailyBatch
	if hasEligibleDays {
		for _, gap := range snapshot.KnownCoverageGaps {
			if gap.StartAt < eligibleRange.EndAt && (gap.EndAt == nil || *gap.EndAt > eligibleRange.StartAt) {
				result.KnownCoverageGaps = append(result.KnownCoverageGaps, gap)
			}
		}
		published, err = s.store.QueryPublishedEvidence(ctx, eligibleRange.StartAt, eligibleRange.EndAt)
		if err != nil {
			return SupplierReportFreshness{}, err
		}
	}
	result.PublishedEvidence, err = aggregateSupplierReportPublishedEvidenceAt(reportRange, published, now)
	if err != nil {
		return SupplierReportFreshness{}, err
	}
	result.FinanceAttentionRequired = supplierReportNeedsFinanceAttention(result.PublishedEvidence, result.KnownCoverageGaps)
	if len(published) > 0 {
		latest := published[len(published)-1]
		result.LatestBatchDate = latest.BatchDate
		result.BatchStatus = model.SupplierDailyBatchStatusCompleted
		freshThrough := latest.DayEnd
		result.FreshThrough = &freshThrough
		lag := nowUnix - freshThrough
		if lag < 0 {
			lag = 0
		}
		result.FreshnessLagSeconds = &lag
	}
	return result, nil
}

type supplierReportCoverageProjection struct {
	gaps                     []model.SupplierAccountingCoverageGap
	evidence                 SupplierReportPublishedEvidence
	financeAttentionRequired bool
}

func (s *SupplierReportService) resolveCoverageProjection(ctx context.Context, reportRange SupplierReportRange, current *supplierReportCoverageProjection) (*supplierReportCoverageProjection, error) {
	return s.resolveCoverageProjectionAt(ctx, reportRange, current, time.Now())
}

func (s *SupplierReportService) resolveCoverageProjectionForQuery(ctx context.Context, query SupplierReportQuery, reportRange SupplierReportRange, current *supplierReportCoverageProjection) (*supplierReportCoverageProjection, error) {
	now := query.eligibleNow
	if now.IsZero() {
		now = time.Now()
	}
	return s.resolveCoverageProjectionAt(ctx, reportRange, current, now)
}

func (s *SupplierReportService) resolveCoverageProjectionAt(ctx context.Context, reportRange SupplierReportRange, current *supplierReportCoverageProjection, now time.Time) (*supplierReportCoverageProjection, error) {
	if current != nil {
		return current, nil
	}
	if s == nil || s.store == nil {
		return nil, model.ErrDatabase
	}
	eligibleRange, hasEligibleDays, err := supplierReportEligibleRange(reportRange, now)
	if err != nil {
		return nil, err
	}
	var gaps []model.SupplierAccountingCoverageGap
	var published []model.SupplierPublishedDailyBatch
	if hasEligibleDays {
		gaps, err = s.store.QueryCoverageGaps(ctx, eligibleRange.StartAt, eligibleRange.EndAt)
		if err != nil {
			return nil, err
		}
		published, err = s.store.QueryPublishedEvidence(ctx, eligibleRange.StartAt, eligibleRange.EndAt)
		if err != nil {
			return nil, err
		}
	}
	evidence, err := aggregateSupplierReportPublishedEvidenceAt(reportRange, published, now)
	if err != nil {
		return nil, err
	}
	return &supplierReportCoverageProjection{
		gaps: gaps, evidence: evidence,
		financeAttentionRequired: supplierReportNeedsFinanceAttention(evidence, gaps),
	}, nil
}

func aggregateSupplierReportPublishedEvidence(reportRange SupplierReportRange, batches []model.SupplierPublishedDailyBatch) (SupplierReportPublishedEvidence, error) {
	return aggregateSupplierReportPublishedEvidenceAt(reportRange, batches, time.Now())
}

func aggregateSupplierReportPublishedEvidenceAt(reportRange SupplierReportRange, batches []model.SupplierPublishedDailyBatch, now time.Time) (SupplierReportPublishedEvidence, error) {
	eligibleRange, hasEligibleDays, err := supplierReportEligibleRange(reportRange, now)
	if err != nil {
		return SupplierReportPublishedEvidence{}, err
	}
	expectedDays := 0
	if hasEligibleDays {
		location, loadErr := time.LoadLocation(SupplierReportTimezone)
		if loadErr != nil {
			return SupplierReportPublishedEvidence{}, loadErr
		}
		start := time.Unix(eligibleRange.StartAt, 0).In(location)
		end := time.Unix(eligibleRange.EndAt, 0).In(location)
		for day := start; day.Before(end); day = day.AddDate(0, 0, 1) {
			expectedDays++
		}
	}
	result := SupplierReportPublishedEvidence{
		PublishedDays: len(batches), ExpectedDays: expectedDays,
		PersistedLogSnapshotCompleteness: types.SupplierPersistedLogCompletenessNotScanned,
		Warnings:                         []types.SupplierPublishedWarningV1{},
	}
	warningCounts := make(map[string]types.SupplierPublishedWarningV1)
	allComplete := len(batches) == expectedDays && expectedDays > 0
	for _, batch := range batches {
		evidence := batch.Evidence
		if evidence.PersistedLogSnapshotCompleteness != types.SupplierPersistedLogCompletenessComplete {
			allComplete = false
		}
		for target, value := range map[*int64]int64{
			&result.LogsScanned: evidence.LogsScanned, &result.ProducerMarkersPresent: evidence.ProducerMarkersPresent, &result.CapturedSnapshotCount: evidence.CapturedSnapshotCount,
			&result.DispositionCounts.Captured: evidence.DispositionCounts.Captured, &result.DispositionCounts.UnsupportedPath: evidence.DispositionCounts.UnsupportedPath,
			&result.DispositionCounts.NotFinanciallyCommitted: evidence.DispositionCounts.NotFinanciallyCommitted, &result.DispositionCounts.ZeroUsage: evidence.DispositionCounts.ZeroUsage,
			&result.DispositionCounts.Unbound: evidence.DispositionCounts.Unbound, &result.DispositionCounts.ProducerError: evidence.DispositionCounts.ProducerError,
			&result.FailureCounts.UnknownProducerCapability:      evidence.FailureCounts.UnknownProducerCapability,
			&result.FailureCounts.IncompatibleProducerCapability: evidence.FailureCounts.IncompatibleProducerCapability,
			&result.FailureCounts.AbsentMarkerAfterCutover:       evidence.FailureCounts.AbsentMarkerAfterCutover,
			&result.FailureCounts.InvalidCapturedSnapshot:        evidence.FailureCounts.InvalidCapturedSnapshot,
			&result.FailureCounts.UnknownOfficialAmount:          evidence.FailureCounts.UnknownOfficialAmount,
		} {
			*target, err = checkedAddInt64(*target, value)
			if err != nil {
				return SupplierReportPublishedEvidence{}, err
			}
		}
		for _, warning := range evidence.Warnings {
			current := warningCounts[warning.Code]
			if current.Code == "" {
				current = warning
			} else {
				current.Count, err = checkedAddInt64(current.Count, warning.Count)
				if err != nil {
					return SupplierReportPublishedEvidence{}, err
				}
			}
			warningCounts[warning.Code] = current
		}
	}
	if len(batches) > 0 {
		result.PersistedLogSnapshotCompleteness = types.SupplierPersistedLogCompletenessIncomplete
		if allComplete {
			result.PersistedLogSnapshotCompleteness = types.SupplierPersistedLogCompletenessComplete
		}
	}
	codes := make([]string, 0, len(warningCounts))
	for code := range warningCounts {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		result.Warnings = append(result.Warnings, warningCounts[code])
	}
	return result, nil
}

func supplierReportEligibleRange(reportRange SupplierReportRange, now time.Time) (SupplierReportRange, bool, error) {
	location, err := time.LoadLocation(SupplierReportTimezone)
	if err != nil {
		return SupplierReportRange{}, false, err
	}
	start := beginningOfSupplierDay(time.Unix(reportRange.StartAt, 0).In(location))
	requestedEnd := time.Unix(reportRange.EndAt, 0).In(location)
	today := beginningOfSupplierDay(now.In(location))
	eligibleEnd := today
	if now.In(location).Before(today.Add(SupplierDailyCloseGrace)) {
		eligibleEnd = today.AddDate(0, 0, -1)
	}
	if requestedEnd.Before(eligibleEnd) {
		eligibleEnd = requestedEnd
	}
	result := SupplierReportRange{
		StartAt: start.Unix(), EndAt: eligibleEnd.Unix(), Timezone: SupplierReportTimezone, Month: reportRange.Month,
	}
	if !eligibleEnd.After(start) {
		result.EndAt = result.StartAt
		return result, false, nil
	}
	return result, true, nil
}

func supplierReportNeedsFinanceAttention(evidence SupplierReportPublishedEvidence, gaps []model.SupplierAccountingCoverageGap) bool {
	if len(gaps) > 0 {
		return true
	}
	if evidence.ExpectedDays == 0 && evidence.PublishedDays == 0 {
		return false
	}
	return evidence.PersistedLogSnapshotCompleteness != types.SupplierPersistedLogCompletenessComplete
}

type contractRuntime struct {
	inventory    int64
	consumed     int64
	channelCount int
}

func (s *SupplierReportService) resolveContracts(ctx context.Context, query SupplierReportQuery, page *model.SupplierReportPage) (SupplierReportRange, model.SupplierReportFilter, []model.SupplierReportContractCatalogRow, bool, error) {
	if s == nil || s.store == nil {
		return SupplierReportRange{}, model.SupplierReportFilter{}, nil, false, model.ErrDatabase
	}
	query, err := s.prepareQuery(query)
	if err != nil {
		return SupplierReportRange{}, model.SupplierReportFilter{}, nil, false, err
	}
	reportRange, filter, err := query.modelFilter()
	if err != nil {
		return SupplierReportRange{}, model.SupplierReportFilter{}, nil, false, err
	}
	if !query.hasEligibleDays {
		return reportRange, filter, []model.SupplierReportContractCatalogRow{}, false, nil
	}
	catalog, hasMore, err := s.store.ListContractCatalog(ctx, filter, page)
	if err != nil {
		return SupplierReportRange{}, model.SupplierReportFilter{}, nil, false, err
	}
	filter.ContractIds = catalogContractIds(catalog)
	return reportRange, filter, catalog, hasMore, nil
}

func (s *SupplierReportService) loadUsage(ctx context.Context, filter model.SupplierReportFilter, daily bool) (*usageAccumulator, error) {
	usage := newUsageAccumulator(len(filter.ChannelIds) == 0)
	businessRows, err := s.store.QueryBusinessUsage(ctx, filter, daily)
	if err != nil {
		return nil, err
	}
	usage.businessRows = businessRows
	for _, row := range businessRows {
		if err := usage.addBusiness(row); err != nil {
			return nil, err
		}
	}
	if usage.internalDimensionAvailable {
		internalRows, queryErr := s.store.QueryInternalUsage(ctx, filter, daily)
		if queryErr != nil {
			return nil, queryErr
		}
		usage.internalRows = internalRows
		for _, row := range internalRows {
			if err := usage.addInternal(row); err != nil {
				return nil, err
			}
		}
	}
	return usage, nil
}

func (s *SupplierReportService) loadInventoryRuntime(ctx context.Context, catalog []model.SupplierReportContractCatalogRow, channelIds []int, endAt int64) (map[int]contractRuntime, error) {
	contractIds := catalogContractIds(catalog)
	channelCounts, err := s.store.QueryLinkedChannelCounts(ctx, contractIds, channelIds)
	if err != nil {
		return nil, err
	}
	adjustments, err := s.store.ListInventoryAdjustments(ctx, contractIds, endAt)
	if err != nil {
		return nil, err
	}
	consumption, err := s.store.QueryInventoryConsumption(ctx, contractIds, endAt)
	if err != nil {
		return nil, err
	}
	result := make(map[int]contractRuntime, len(contractIds))
	for _, row := range channelCounts {
		if row.Count > int64(math.MaxInt) {
			return nil, ErrSupplierReportOverflow
		}
		runtime := result[row.ContractId]
		runtime.channelCount = int(row.Count)
		result[row.ContractId] = runtime
	}
	for _, adjustment := range adjustments {
		runtime := result[adjustment.ContractId]
		runtime.inventory, err = checkedAddInt64(runtime.inventory, adjustment.DeltaMicroUsd)
		if err != nil {
			return nil, err
		}
		result[adjustment.ContractId] = runtime
	}
	for _, row := range consumption {
		runtime := result[row.ContractId]
		runtime.consumed = row.InventoryAffectingOfficialListMicroUsd
		result[row.ContractId] = runtime
	}
	return result, nil
}

type metricAccumulator struct {
	requests, unattributed        int64
	officialKnown, official       int64
	salesKnown, sales             int64
	procurementKnown, procurement int64
	grossKnown, gross             int64
	eligibleCount, eligibleSales  int64
}

func (a *metricAccumulator) addBusinessFields(values ...int64) error {
	if len(values) != 12 {
		return ErrSupplierReportOverflow
	}
	targets := []*int64{&a.requests, &a.unattributed, &a.officialKnown, &a.official, &a.salesKnown, &a.sales, &a.procurementKnown, &a.procurement, &a.grossKnown, &a.gross, &a.eligibleCount, &a.eligibleSales}
	for i := range targets {
		value, err := checkedAddInt64(*targets[i], values[i])
		if err != nil {
			return err
		}
		*targets[i] = value
	}
	return nil
}

func (a *metricAccumulator) addInternalFields(requests, unattributed, officialKnown, official, procurementKnown, procurement int64) error {
	return a.addBusinessFields(requests, unattributed, officialKnown, official, 0, 0, procurementKnown, procurement, 0, 0, 0, 0)
}

func (a metricAccumulator) metrics() SupplierReportMetrics {
	return SupplierReportMetrics{
		RequestCount: a.requests, UnattributedRequestCount: a.unattributed,
		OfficialList:             SupplierReportMoney{KnownCount: a.officialKnown, MicroUsd: a.official},
		Sales:                    SupplierReportMoney{KnownCount: a.salesKnown, MicroUsd: a.sales},
		ProcurementCost:          SupplierReportMoney{KnownCount: a.procurementKnown, MicroUsd: a.procurement},
		GrossProfit:              SupplierReportMoney{KnownCount: a.grossKnown, MicroUsd: a.gross},
		GrossMarginEligibleCount: a.eligibleCount, GrossMarginEligibleSalesMicroUsd: a.eligibleSales,
		GrossMargin: ratioString(a.gross, a.eligibleSales), GrossMarginEligibleCoverage: ratioString(a.eligibleCount, a.requests),
	}
}

type usageAccumulator struct {
	business, internal         metricAccumulator
	internalDimensionAvailable bool
	businessRows               []model.SupplierReportBusinessUsageRow
	internalRows               []model.SupplierReportInternalUsageRow
}

func newUsageAccumulator(internalAvailable bool) *usageAccumulator {
	return &usageAccumulator{internalDimensionAvailable: internalAvailable}
}

func (a *usageAccumulator) addBusiness(row model.SupplierReportBusinessUsageRow) error {
	if err := a.business.addBusinessFields(row.BusinessRequestCount, row.UnattributedRequestCount, row.OfficialListKnownCount, row.OfficialListMicroUsd, row.SalesKnownCount, row.SalesMicroUsd, row.ProcurementCostKnownCount, row.ProcurementCostMicroUsd, row.GrossProfitKnownCount, row.GrossProfitMicroUsd, row.GrossMarginEligibleCount, row.GrossMarginEligibleSalesMicroUsd); err != nil {
		return err
	}
	return nil
}

func (a *usageAccumulator) addChannel(row model.SupplierReportChannelUsageRow) error {
	return a.addBusiness(model.SupplierReportBusinessUsageRow{
		ContractId: row.ContractId, DataQuality: row.DataQuality,
		BusinessRequestCount: row.BusinessRequestCount, UnattributedRequestCount: row.UnattributedRequestCount,
		OfficialListKnownCount: row.OfficialListKnownCount, OfficialListMicroUsd: row.OfficialListMicroUsd,
		SalesKnownCount: row.SalesKnownCount, SalesMicroUsd: row.SalesMicroUsd,
		ProcurementCostKnownCount: row.ProcurementCostKnownCount, ProcurementCostMicroUsd: row.ProcurementCostMicroUsd,
		GrossProfitKnownCount: row.GrossProfitKnownCount, GrossProfitMicroUsd: row.GrossProfitMicroUsd,
		GrossMarginEligibleCount: row.GrossMarginEligibleCount, GrossMarginEligibleSalesMicroUsd: row.GrossMarginEligibleSalesMicroUsd,
	})
}

func (a *usageAccumulator) addInternal(row model.SupplierReportInternalUsageRow) error {
	if err := a.internal.addInternalFields(row.InternalRequestCount, row.UnattributedRequestCount, row.OfficialListKnownCount, row.OfficialListMicroUsd, row.ProcurementCostKnownCount, row.ProcurementCostMicroUsd); err != nil {
		return err
	}
	return nil
}

func (a *usageAccumulator) byContract() (map[int]*usageAccumulator, error) {
	result := make(map[int]*usageAccumulator)
	for _, row := range a.businessRows {
		item := result[row.ContractId]
		if item == nil {
			item = newUsageAccumulator(a.internalDimensionAvailable)
			result[row.ContractId] = item
		}
		if err := item.addBusiness(row); err != nil {
			return nil, err
		}
	}
	for _, row := range a.internalRows {
		item := result[row.ContractId]
		if item == nil {
			item = newUsageAccumulator(a.internalDimensionAvailable)
			result[row.ContractId] = item
		}
		if err := item.addInternal(row); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func buildContractRow(catalog model.SupplierReportContractCatalogRow, runtime contractRuntime, usage *usageAccumulator) (SupplierReportContractRow, error) {
	if usage == nil {
		usage = newUsageAccumulator(true)
	}
	remaining, err := checkedSubInt64(runtime.inventory, runtime.consumed)
	if err != nil {
		return SupplierReportContractRow{}, err
	}
	row := SupplierReportContractRow{
		ContractId: catalog.ContractId, SupplierId: catalog.SupplierId, SupplierName: catalog.SupplierName, SupplierStatus: catalog.SupplierStatus,
		ContractName: catalog.ContractName, ContractNo: catalog.ContractNo, ContractStatus: catalog.ContractStatus, Remark: catalog.Remark,
		CurrentRateVersionId: catalog.CurrentRateVersionId, ProcurementMultiplierPpm: catalog.ProcurementMultiplierPpm,
		RpmLimit: catalog.RpmLimit, TpmLimit: catalog.TpmLimit, MaxConcurrency: catalog.MaxConcurrency, LinkedChannelCount: runtime.channelCount,
		TotalInventoryMicroUsd: runtime.inventory, OfficialListConsumedMicroUsd: runtime.consumed, RemainingInventoryMicroUsd: remaining,
		UtilizationRate: ratioString(runtime.consumed, runtime.inventory), Oversold: remaining < 0,
		Business:                   usage.business.metrics(),
		Internal:                   usage.internal.metrics(),
		InternalDimensionAvailable: usage.internalDimensionAvailable,
	}
	row.TotalProcurementCost, err = combinedSupplierReportMoney(row.Business.ProcurementCost, row.Internal.ProcurementCost)
	if err != nil {
		return SupplierReportContractRow{}, err
	}
	return row, nil
}

func catalogContractIds(catalog []model.SupplierReportContractCatalogRow) []int {
	ids := make([]int, len(catalog))
	for i := range catalog {
		ids[i] = catalog[i].ContractId
	}
	return ids
}

func combinedSupplierReportMoney(left, right SupplierReportMoney) (SupplierReportMoney, error) {
	knownCount, err := checkedAddInt64(left.KnownCount, right.KnownCount)
	if err != nil {
		return SupplierReportMoney{}, err
	}
	microUsd, err := checkedAddInt64(left.MicroUsd, right.MicroUsd)
	if err != nil {
		return SupplierReportMoney{}, err
	}
	return SupplierReportMoney{KnownCount: knownCount, MicroUsd: microUsd}, nil
}

func checkedAddInt64(left, right int64) (int64, error) {
	if (right > 0 && left > math.MaxInt64-right) || (right < 0 && left < math.MinInt64-right) {
		return 0, ErrSupplierReportOverflow
	}
	return left + right, nil
}

func checkedSubInt64(left, right int64) (int64, error) {
	if right == math.MinInt64 {
		if left >= 0 {
			return 0, ErrSupplierReportOverflow
		}
		return left - right, nil
	}
	return checkedAddInt64(left, -right)
}

func ratioString(numerator, denominator int64) *string {
	if denominator == 0 {
		return nil
	}
	value := decimal.NewFromInt(numerator).Div(decimal.NewFromInt(denominator)).Round(8).String()
	return &value
}
