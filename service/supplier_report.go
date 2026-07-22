package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
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
	LatestBatchDate     string `json:"latest_batch_date"`
	BatchStatus         string `json:"batch_status"`
	FreshThrough        *int64 `json:"fresh_through"`
	FreshnessLagSeconds *int64 `json:"freshness_lag_seconds"`
	ErrorMessage        string `json:"error_message"`
	SyncOnly            bool   `json:"sync_only"`
	CoverageStartAt     int64  `json:"coverage_start_at"`
}

type SupplierReportOverview struct {
	Range                        SupplierReportRange   `json:"range"`
	Business                     SupplierReportMetrics `json:"business"`
	Internal                     SupplierReportMetrics `json:"internal"`
	TotalProcurementCost         SupplierReportMoney   `json:"total_estimated_procurement_cost"`
	TotalInventoryMicroUsd       int64                 `json:"total_inventory_micro_usd"`
	OfficialListConsumedMicroUsd int64                 `json:"official_list_consumed_micro_usd"`
	RemainingInventoryMicroUsd   int64                 `json:"remaining_inventory_micro_usd"`
	InternalDimensionAvailable   bool                  `json:"internal_dimension_available"`
}

type SupplierReportTrendPoint struct {
	BucketStart                int64                 `json:"bucket_start"`
	Date                       string                `json:"date"`
	Business                   SupplierReportMetrics `json:"business"`
	Internal                   SupplierReportMetrics `json:"internal"`
	InternalDimensionAvailable bool                  `json:"internal_dimension_available"`
}

type SupplierReportTrend struct {
	Range  SupplierReportRange        `json:"range"`
	Points []SupplierReportTrendPoint `json:"points"`
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
	Range   SupplierReportRange         `json:"range"`
	Items   []SupplierReportContractRow `json:"items"`
	Limit   int                         `json:"limit"`
	Offset  int                         `json:"offset"`
	HasMore bool                        `json:"has_more"`
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
	Range   SupplierReportRange        `json:"range"`
	Items   []SupplierReportChannelRow `json:"items"`
	Limit   int                        `json:"limit"`
	Offset  int                        `json:"offset"`
	HasMore bool                       `json:"has_more"`
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
	Range                      SupplierReportRange           `json:"range"`
	Items                      []SupplierReportBreakdownItem `json:"items"`
	Limit                      int                           `json:"limit"`
	Offset                     int                           `json:"offset"`
	HasMore                    bool                          `json:"has_more"`
	BreakdownEligibleCount     int64                         `json:"breakdown_eligible_count"`
	TotalBusinessCount         int64                         `json:"total_business_count"`
	BreakdownCoverageRate      *string                       `json:"breakdown_coverage_rate"`
	BreakdownCoverageAvailable bool                          `json:"breakdown_coverage_available"`
}

type SupplierReportContractDetail struct {
	Range                SupplierReportRange                 `json:"range"`
	Summary              SupplierReportContractRow           `json:"summary"`
	RateVersions         []SupplierReportRateVersion         `json:"rate_versions"`
	InventoryAdjustments []SupplierReportInventoryAdjustment `json:"inventory_adjustments"`
	Channels             SupplierReportChannelList           `json:"channels"`
	InternalTrend        []SupplierReportTrendPoint          `json:"internal_trend"`
	Breakdown            SupplierReportBreakdownList         `json:"breakdown"`
}

type SupplierReportService struct {
	store *model.SupplierReportStore
}

func NewSupplierReportService(store *model.SupplierReportStore) *SupplierReportService {
	return &SupplierReportService{store: store}
}

func DefaultSupplierReportService() *SupplierReportService {
	return NewSupplierReportService(model.DefaultSupplierReportStore())
}

func (s *SupplierReportService) GetOverview(ctx context.Context, query SupplierReportQuery) (SupplierReportOverview, error) {
	reportRange, filter, catalog, _, err := s.resolveContracts(ctx, query, nil)
	if err != nil {
		return SupplierReportOverview{}, err
	}
	if len(catalog) == 0 {
		return SupplierReportOverview{Range: reportRange, InternalDimensionAvailable: len(filter.ChannelIds) == 0}, nil
	}
	usage, err := s.loadUsage(ctx, filter, false)
	if err != nil {
		return SupplierReportOverview{}, err
	}
	runtime, err := s.loadInventoryRuntime(ctx, catalog, filter.ChannelIds)
	if err != nil {
		return SupplierReportOverview{}, err
	}
	result := SupplierReportOverview{
		Range: reportRange, Business: usage.business.metrics(), Internal: usage.internal.metrics(),
		InternalDimensionAvailable: usage.internalDimensionAvailable,
	}
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
	reportRange, filter, catalog, _, err := s.resolveContracts(ctx, query, nil)
	if err != nil {
		return SupplierReportTrend{}, err
	}
	result := SupplierReportTrend{Range: reportRange, Points: []SupplierReportTrendPoint{}}
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
	reportRange, filter, catalog, hasMore, err := s.resolveContracts(ctx, query, &page)
	if err != nil {
		return SupplierReportContractList{}, err
	}
	page = page.Normalize()
	result := SupplierReportContractList{Range: reportRange, Limit: page.Limit, Offset: page.Offset, Items: []SupplierReportContractRow{}}
	result.HasMore = hasMore
	if len(catalog) == 0 {
		return result, nil
	}
	filter.ContractIds = catalogContractIds(catalog)
	usage, err := s.loadUsage(ctx, filter, false)
	if err != nil {
		return SupplierReportContractList{}, err
	}
	runtime, err := s.loadInventoryRuntime(ctx, catalog, filter.ChannelIds)
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
	list, err := s.ListContracts(ctx, query, model.SupplierReportPage{Limit: 1})
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	if len(list.Items) == 0 {
		return SupplierReportContractDetail{}, ErrSupplierReportContractNotFound
	}
	rates, err := s.store.ListRateVersions(ctx, contractId)
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	adjustments, err := s.store.ListInventoryAdjustments(ctx, []int{contractId})
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	channels, err := s.ListChannels(ctx, query, model.SupplierReportPage{Limit: model.SupplierReportMaxPageSize})
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	breakdown, err := s.ListBreakdown(ctx, query, page)
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	trend, err := s.GetTrend(ctx, query)
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	result := SupplierReportContractDetail{Range: list.Range, Summary: list.Items[0], Channels: channels, Breakdown: breakdown}
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
	reportRange, filter, catalog, _, err := s.resolveContracts(ctx, query, nil)
	if err != nil {
		return SupplierReportChannelList{}, err
	}
	page = page.Normalize()
	result := SupplierReportChannelList{Range: reportRange, Limit: page.Limit, Offset: page.Offset, Items: []SupplierReportChannelRow{}}
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
	reportRange, filter, catalog, _, err := s.resolveContracts(ctx, query, nil)
	if err != nil {
		return SupplierReportBreakdownList{}, err
	}
	page = page.Normalize()
	result := SupplierReportBreakdownList{Range: reportRange, Limit: page.Limit, Offset: page.Offset, Items: []SupplierReportBreakdownItem{}}
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
	if s == nil || s.store == nil {
		return SupplierReportFreshness{}, model.ErrDatabase
	}
	snapshot, err := s.store.QueryFreshness(ctx)
	if err != nil {
		return SupplierReportFreshness{}, err
	}
	return SupplierReportFreshness{
		LatestBatchDate: snapshot.LatestBatchDate, BatchStatus: snapshot.LatestStatus,
		FreshThrough: snapshot.FreshThrough, FreshnessLagSeconds: snapshot.FreshnessLagSeconds,
		ErrorMessage: snapshot.ErrorMessage, SyncOnly: snapshot.SyncOnly, CoverageStartAt: snapshot.CoverageStartAt,
	}, nil
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
	reportRange, filter, err := query.modelFilter()
	if err != nil {
		return SupplierReportRange{}, model.SupplierReportFilter{}, nil, false, err
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

func (s *SupplierReportService) loadInventoryRuntime(ctx context.Context, catalog []model.SupplierReportContractCatalogRow, channelIds []int) (map[int]contractRuntime, error) {
	contractIds := catalogContractIds(catalog)
	channelCounts, err := s.store.QueryLinkedChannelCounts(ctx, contractIds, channelIds)
	if err != nil {
		return nil, err
	}
	adjustments, err := s.store.ListInventoryAdjustments(ctx, contractIds)
	if err != nil {
		return nil, err
	}
	consumption, err := s.store.QueryInventoryConsumption(ctx, contractIds)
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
