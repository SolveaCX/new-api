package service

import (
	"context"
	"errors"
	"math"
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
	return SupplierReportRange{StartAt: start.Unix(), EndAt: end.Unix(), Timezone: SupplierReportTimezone, Month: month}, nil
}

func (q SupplierReportQuery) filter() (SupplierReportRange, model.SupplierReportFilter, error) {
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
	MicroUsd   int64 `json:"micro_usd,string"`
}

type SupplierReportMetrics struct {
	RequestCount                     int64               `json:"request_count"`
	UnattributedRequestCount         int64               `json:"unattributed_request_count"`
	OfficialList                     SupplierReportMoney `json:"official_list"`
	Sales                            SupplierReportMoney `json:"sales"`
	ProcurementCost                  SupplierReportMoney `json:"procurement_cost"`
	GrossProfit                      SupplierReportMoney `json:"gross_profit"`
	GrossMarginEligibleCount         int64               `json:"gross_margin_eligible_count"`
	GrossMarginEligibleSalesMicroUsd int64               `json:"gross_margin_eligible_sales_micro_usd,string"`
	GrossMargin                      *string             `json:"gross_margin"`
	GrossMarginEligibleCoverage      *string             `json:"gross_margin_eligible_coverage"`
}

type SupplierReportOverview struct {
	Range                        SupplierReportRange    `json:"range"`
	Business                     SupplierReportMetrics  `json:"business"`
	Internal                     *SupplierReportMetrics `json:"internal"`
	TotalProcurementCost         *SupplierReportMoney   `json:"total_estimated_procurement_cost"`
	TotalInventoryMicroUsd       int64                  `json:"total_inventory_micro_usd,string"`
	OfficialListConsumedMicroUsd int64                  `json:"official_list_consumed_micro_usd,string"`
	RemainingInventoryMicroUsd   int64                  `json:"remaining_inventory_micro_usd,string"`
	InternalDimensionAvailable   bool                   `json:"internal_dimension_available"`
}

type SupplierReportTrendPoint struct {
	BucketStart                int64                  `json:"bucket_start"`
	Date                       string                 `json:"date"`
	Business                   SupplierReportMetrics  `json:"business"`
	Internal                   *SupplierReportMetrics `json:"internal"`
	InternalDimensionAvailable bool                   `json:"internal_dimension_available"`
}
type SupplierReportDayStatus struct {
	Date   string `json:"date"`
	Status string `json:"status"`
}
type SupplierReportTrend struct {
	Range               SupplierReportRange        `json:"range"`
	Points              []SupplierReportTrendPoint `json:"points"`
	DayStatuses         []SupplierReportDayStatus  `json:"day_statuses"`
	LatestCompletedDate *string                    `json:"latest_completed_date"`
	HasIncompleteDays   bool                       `json:"has_incomplete_days"`
	IncompleteDayCount  int                        `json:"incomplete_day_count"`
}

type SupplierReportContractRow struct {
	ContractId                   int                    `json:"contract_id"`
	SupplierId                   int                    `json:"supplier_id"`
	SupplierName                 string                 `json:"supplier_name"`
	SupplierStatus               string                 `json:"supplier_status"`
	ContractName                 string                 `json:"contract_name"`
	ContractNo                   string                 `json:"contract_no"`
	ContractStatus               string                 `json:"contract_status"`
	Remark                       string                 `json:"remark"`
	CurrentRateVersionId         *int                   `json:"current_rate_version_id"`
	ProcurementMultiplierPpm     *int64                 `json:"procurement_multiplier_ppm"`
	RpmLimit                     int64                  `json:"rpm_limit"`
	TpmLimit                     int64                  `json:"tpm_limit"`
	MaxConcurrency               int                    `json:"max_concurrency"`
	LinkedChannelCount           int                    `json:"linked_channel_count"`
	TotalInventoryMicroUsd       int64                  `json:"total_inventory_micro_usd,string"`
	OfficialListConsumedMicroUsd int64                  `json:"official_list_consumed_micro_usd,string"`
	RemainingInventoryMicroUsd   int64                  `json:"remaining_inventory_micro_usd,string"`
	UtilizationRate              *string                `json:"utilization_rate"`
	Oversold                     bool                   `json:"oversold"`
	Business                     SupplierReportMetrics  `json:"business"`
	Internal                     *SupplierReportMetrics `json:"internal"`
	TotalProcurementCost         *SupplierReportMoney   `json:"total_estimated_procurement_cost"`
	InternalDimensionAvailable   bool                   `json:"internal_dimension_available"`
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
	DeltaMicroUsd  int64  `json:"delta_micro_usd,string"`
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
	Range                       SupplierReportRange                 `json:"range"`
	Summary                     SupplierReportContractRow           `json:"summary"`
	RateVersions                []SupplierReportRateVersion         `json:"rate_versions"`
	RateVersionsHasMore         bool                                `json:"rate_versions_has_more"`
	InventoryAdjustments        []SupplierReportInventoryAdjustment `json:"inventory_adjustments"`
	InventoryAdjustmentsHasMore bool                                `json:"inventory_adjustments_has_more"`
	Channels                    SupplierReportChannelList           `json:"channels"`
	Breakdown                   SupplierReportBreakdownList         `json:"breakdown"`
}

type SupplierReportService struct{ store *model.SupplierReportStore }

func NewSupplierReportService(store *model.SupplierReportStore) *SupplierReportService {
	return &SupplierReportService{store: store}
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
		var readErr error
		result, readErr = read(NewSupplierReportService(store))
		return readErr
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return result, nil
}

func (s *SupplierReportService) GetOverview(ctx context.Context, query SupplierReportQuery) (SupplierReportOverview, error) {
	return withSupplierReportSnapshot(ctx, s, func(snapshot *SupplierReportService) (SupplierReportOverview, error) {
		return snapshot.getOverview(ctx, query)
	})
}

func (s *SupplierReportService) getOverview(ctx context.Context, query SupplierReportQuery) (SupplierReportOverview, error) {
	reportRange, filter, err := query.filter()
	if err != nil {
		return SupplierReportOverview{}, err
	}
	usage, err := s.loadUsage(ctx, filter, false)
	if err != nil {
		return SupplierReportOverview{}, err
	}
	inventory, err := s.store.QueryOverviewInventory(ctx, filter)
	if err != nil {
		return SupplierReportOverview{}, err
	}
	result := SupplierReportOverview{Range: reportRange, Business: usage.business.metrics(), InternalDimensionAvailable: len(query.ChannelIds) == 0}
	if result.InternalDimensionAvailable {
		internal := usage.internal.metrics()
		result.Internal = &internal
		totalProcurementCost, combineErr := combineMoney(result.Business.ProcurementCost, internal.ProcurementCost)
		if combineErr != nil {
			return SupplierReportOverview{}, combineErr
		}
		result.TotalProcurementCost = &totalProcurementCost
	}
	result.TotalInventoryMicroUsd = inventory.TotalInventoryMicroUsd
	result.OfficialListConsumedMicroUsd = inventory.OfficialListConsumedMicroUsd
	result.RemainingInventoryMicroUsd, err = checkedSubInt64(result.TotalInventoryMicroUsd, result.OfficialListConsumedMicroUsd)
	return result, err
}

func (s *SupplierReportService) GetTrend(ctx context.Context, query SupplierReportQuery) (SupplierReportTrend, error) {
	return withSupplierReportSnapshot(ctx, s, func(snapshot *SupplierReportService) (SupplierReportTrend, error) {
		return snapshot.getTrend(ctx, query)
	})
}

func (s *SupplierReportService) getTrend(ctx context.Context, query SupplierReportQuery) (SupplierReportTrend, error) {
	reportRange, filter, err := query.filter()
	if err != nil {
		return SupplierReportTrend{}, err
	}
	statuses, err := s.store.ListDayStatuses(ctx, reportRange.StartAt, reportRange.EndAt)
	if err != nil {
		return SupplierReportTrend{}, err
	}
	statusByDay := make(map[int64]model.SupplierReportDayStatusRow, len(statuses))
	for _, status := range statuses {
		statusByDay[status.DayStart] = status
	}
	usage, err := s.loadUsage(ctx, filter, true)
	if err != nil {
		return SupplierReportTrend{}, err
	}
	points := map[int64]*usageAccumulator{}
	for _, row := range usage.businessRows {
		item := points[row.BucketStart]
		if item == nil {
			item = &usageAccumulator{}
			points[row.BucketStart] = item
		}
		if err := item.addBusiness(row); err != nil {
			return SupplierReportTrend{}, err
		}
	}
	for _, row := range usage.internalRows {
		item := points[row.BucketStart]
		if item == nil {
			item = &usageAccumulator{}
			points[row.BucketStart] = item
		}
		if err := item.addInternal(row); err != nil {
			return SupplierReportTrend{}, err
		}
	}
	result := SupplierReportTrend{Range: reportRange, Points: []SupplierReportTrendPoint{}, DayStatuses: []SupplierReportDayStatus{}}
	for day := reportRange.StartAt; day < reportRange.EndAt; {
		local := time.Unix(day, 0).In(mustSupplierReportLocation())
		date := local.Format("2006-01-02")
		status, found := statusByDay[day]
		dayStatus := "missing"
		published := found && status.PublishedFenceToken > 0
		if published {
			dayStatus = model.SupplierDailyBatchStatusCompleted
		} else if found && (status.Status == model.SupplierDailyBatchStatusRunning || status.Status == model.SupplierDailyBatchStatusFailed) {
			dayStatus = status.Status
		}
		result.DayStatuses = append(result.DayStatuses, SupplierReportDayStatus{Date: date, Status: dayStatus})
		if published {
			item := points[day]
			if item == nil {
				item = &usageAccumulator{}
			}
			point := SupplierReportTrendPoint{BucketStart: day, Date: date, Business: item.business.metrics(), InternalDimensionAvailable: len(query.ChannelIds) == 0}
			if point.InternalDimensionAvailable {
				internal := item.internal.metrics()
				point.Internal = &internal
			}
			result.Points = append(result.Points, point)
			completedDate := date
			result.LatestCompletedDate = &completedDate
		} else {
			result.IncompleteDayCount++
		}
		day = local.AddDate(0, 0, 1).Unix()
	}
	result.HasIncompleteDays = result.IncompleteDayCount > 0
	return result, nil
}

func (s *SupplierReportService) ListContracts(ctx context.Context, query SupplierReportQuery, page model.SupplierReportPage) (SupplierReportContractList, error) {
	return withSupplierReportSnapshot(ctx, s, func(snapshot *SupplierReportService) (SupplierReportContractList, error) {
		return snapshot.listContracts(ctx, query, page)
	})
}

func (s *SupplierReportService) listContracts(ctx context.Context, query SupplierReportQuery, page model.SupplierReportPage) (SupplierReportContractList, error) {
	reportRange, filter, err := query.filter()
	if err != nil {
		return SupplierReportContractList{}, err
	}
	catalog, hasMore, err := s.store.ListContractCatalog(ctx, filter, &page)
	if err != nil {
		return SupplierReportContractList{}, err
	}
	page = page.Normalize()
	result := SupplierReportContractList{Range: reportRange, Items: []SupplierReportContractRow{}, Limit: page.Limit, Offset: page.Offset, HasMore: hasMore}
	if len(catalog) == 0 {
		return result, nil
	}
	filter.ContractIds = catalogIDs(catalog)
	usage, err := s.loadUsageByContract(ctx, filter)
	if err != nil {
		return SupplierReportContractList{}, err
	}
	byContract, err := usage.byContract()
	if err != nil {
		return SupplierReportContractList{}, err
	}
	runtime, err := s.loadInventory(ctx, catalog, query.ChannelIds, reportRange.EndAt)
	if err != nil {
		return SupplierReportContractList{}, err
	}
	internalDimensionAvailable := len(query.ChannelIds) == 0
	for _, row := range catalog {
		item, err := buildContractRow(row, runtime[row.ContractId], byContract[row.ContractId], internalDimensionAvailable)
		if err != nil {
			return SupplierReportContractList{}, err
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func (s *SupplierReportService) GetContractDetail(ctx context.Context, contractId int, query SupplierReportQuery, page model.SupplierReportPage) (SupplierReportContractDetail, error) {
	return withSupplierReportSnapshot(ctx, s, func(snapshot *SupplierReportService) (SupplierReportContractDetail, error) {
		return snapshot.getContractDetail(ctx, contractId, query, page)
	})
}

func (s *SupplierReportService) getContractDetail(ctx context.Context, contractId int, query SupplierReportQuery, page model.SupplierReportPage) (SupplierReportContractDetail, error) {
	query.ContractIds = []int{contractId}
	list, err := s.listContracts(ctx, query, model.SupplierReportPage{Limit: 1})
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	if len(list.Items) == 0 {
		return SupplierReportContractDetail{}, ErrSupplierReportContractNotFound
	}
	rates, ratesHasMore, err := s.store.ListRateVersions(ctx, contractId, list.Range.EndAt, page)
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	adjustments, adjustmentsHasMore, err := s.store.ListInventoryAdjustments(ctx, []int{contractId}, list.Range.EndAt, page)
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	channels, err := s.listChannels(ctx, query, page)
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	breakdown, err := s.listBreakdown(ctx, query, page)
	if err != nil {
		return SupplierReportContractDetail{}, err
	}
	result := SupplierReportContractDetail{Range: list.Range, Summary: list.Items[0], Channels: channels, Breakdown: breakdown, RateVersions: []SupplierReportRateVersion{}, RateVersionsHasMore: ratesHasMore, InventoryAdjustments: []SupplierReportInventoryAdjustment{}, InventoryAdjustmentsHasMore: adjustmentsHasMore}
	for _, row := range rates {
		result.RateVersions = append(result.RateVersions, SupplierReportRateVersion{Id: row.Id, ProcurementMultiplierPpm: row.ProcurementMultiplierPpm, EffectiveAt: row.EffectiveAt, CreatedBy: row.CreatedBy, Reason: row.Reason, CreatedAt: row.CreatedAt})
	}
	for _, row := range adjustments {
		result.InventoryAdjustments = append(result.InventoryAdjustments, SupplierReportInventoryAdjustment{Id: row.Id, DeltaMicroUsd: row.DeltaMicroUsd, Type: row.Type, Reason: row.Reason, IdempotencyKey: row.IdempotencyKey, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt})
	}
	return result, nil
}

func (s *SupplierReportService) ListChannels(ctx context.Context, query SupplierReportQuery, page model.SupplierReportPage) (SupplierReportChannelList, error) {
	return withSupplierReportSnapshot(ctx, s, func(snapshot *SupplierReportService) (SupplierReportChannelList, error) {
		return snapshot.listChannels(ctx, query, page)
	})
}

func (s *SupplierReportService) listChannels(ctx context.Context, query SupplierReportQuery, page model.SupplierReportPage) (SupplierReportChannelList, error) {
	reportRange, filter, err := query.filter()
	if err != nil {
		return SupplierReportChannelList{}, err
	}
	catalog, hasMore, err := s.store.ListChannelCatalog(ctx, filter, &page)
	if err != nil {
		return SupplierReportChannelList{}, err
	}
	pairs := make([]model.SupplierReportChannelPair, 0, len(catalog))
	for _, row := range catalog {
		pairs = append(pairs, model.SupplierReportChannelPair{ContractId: row.SupplierContractId, ChannelId: row.ChannelId})
	}
	rows, err := s.store.QueryChannelUsage(ctx, filter, pairs)
	if err != nil {
		return SupplierReportChannelList{}, err
	}
	metrics := map[[2]int]*metricAccumulator{}
	for _, row := range rows {
		key := [2]int{row.ContractId, row.ChannelId}
		item := metrics[key]
		if item == nil {
			item = &metricAccumulator{}
			metrics[key] = item
		}
		if err := item.addChannel(row); err != nil {
			return SupplierReportChannelList{}, err
		}
	}
	page = page.Normalize()
	result := SupplierReportChannelList{Range: reportRange, Items: []SupplierReportChannelRow{}, Limit: page.Limit, Offset: page.Offset, HasMore: hasMore}
	for _, row := range catalog {
		item := metrics[[2]int{row.SupplierContractId, row.ChannelId}]
		if item == nil {
			item = &metricAccumulator{}
		}
		result.Items = append(result.Items, SupplierReportChannelRow{ChannelId: row.ChannelId, ChannelName: row.ChannelName, ChannelStatus: row.ChannelStatus, ContractId: row.SupplierContractId, Business: item.metrics()})
	}
	return result, nil
}

func (s *SupplierReportService) ListBreakdown(ctx context.Context, query SupplierReportQuery, page model.SupplierReportPage) (SupplierReportBreakdownList, error) {
	return withSupplierReportSnapshot(ctx, s, func(snapshot *SupplierReportService) (SupplierReportBreakdownList, error) {
		return snapshot.listBreakdown(ctx, query, page)
	})
}

func (s *SupplierReportService) listBreakdown(ctx context.Context, query SupplierReportQuery, page model.SupplierReportPage) (SupplierReportBreakdownList, error) {
	reportRange, filter, err := query.filter()
	if err != nil {
		return SupplierReportBreakdownList{}, err
	}
	rows, hasMore, err := s.store.QueryBreakdown(ctx, filter, page)
	if err != nil {
		return SupplierReportBreakdownList{}, err
	}
	eligible, err := s.store.QueryBreakdownEligibleCount(ctx, filter)
	if err != nil {
		return SupplierReportBreakdownList{}, err
	}
	business, err := s.store.QueryBusinessUsage(ctx, filter, false)
	if err != nil {
		return SupplierReportBreakdownList{}, err
	}
	var total int64
	for _, row := range business {
		total, err = checkedAddInt64(total, row.BusinessRequestCount)
		if err != nil {
			return SupplierReportBreakdownList{}, err
		}
	}
	page = page.Normalize()
	result := SupplierReportBreakdownList{Range: reportRange, Items: []SupplierReportBreakdownItem{}, Limit: page.Limit, Offset: page.Offset, HasMore: hasMore, BreakdownEligibleCount: eligible, TotalBusinessCount: total, BreakdownCoverageAvailable: len(query.ChannelIds) == 0}
	if result.BreakdownCoverageAvailable {
		result.BreakdownCoverageRate = ratioString(eligible, total)
	}
	for _, row := range rows {
		a := metricAccumulator{}
		if err := a.addBusinessRow(row); err != nil {
			return SupplierReportBreakdownList{}, err
		}
		result.Items = append(result.Items, SupplierReportBreakdownItem{ContractId: row.ContractId, ChannelId: row.ChannelId, ModelName: row.ModelName, RateVersionId: row.RateVersionId, SalesMultiplierPpm: row.SalesMultiplierPpm, PricingMode: row.PricingMode, DataQuality: row.DataQuality, Metrics: a.metrics()})
	}
	return result, nil
}

type contractRuntime struct {
	channelCount        int
	inventory, consumed int64
}

func (s *SupplierReportService) loadInventory(ctx context.Context, catalog []model.SupplierReportContractCatalogRow, channelIds []int, endAt int64) (map[int]contractRuntime, error) {
	ids := catalogIDs(catalog)
	counts, err := s.store.QueryLinkedChannelCounts(ctx, ids, channelIds)
	if err != nil {
		return nil, err
	}
	adjustments, err := s.store.QueryInventoryAdjustmentTotals(ctx, ids, endAt)
	if err != nil {
		return nil, err
	}
	consumption, err := s.store.QueryInventoryConsumption(ctx, ids, endAt)
	if err != nil {
		return nil, err
	}
	result := map[int]contractRuntime{}
	for _, row := range counts {
		if row.Count > math.MaxInt {
			return nil, ErrSupplierReportOverflow
		}
		r := result[row.ContractId]
		r.channelCount = int(row.Count)
		result[row.ContractId] = r
	}
	for _, row := range adjustments {
		r := result[row.ContractId]
		r.inventory = row.TotalInventoryMicroUsd
		result[row.ContractId] = r
	}
	for _, row := range consumption {
		r := result[row.ContractId]
		r.consumed = row.InventoryAffectingOfficialListMicroUsd
		result[row.ContractId] = r
	}
	return result, nil
}

type metricAccumulator struct{ requests, unattributed, officialKnown, official, salesKnown, sales, procurementKnown, procurement, grossKnown, gross, eligibleCount, eligibleSales int64 }

func (a *metricAccumulator) add(values ...int64) error {
	targets := []*int64{&a.requests, &a.unattributed, &a.officialKnown, &a.official, &a.salesKnown, &a.sales, &a.procurementKnown, &a.procurement, &a.grossKnown, &a.gross, &a.eligibleCount, &a.eligibleSales}
	if len(values) != len(targets) {
		return ErrSupplierReportOverflow
	}
	for i := range targets {
		v, err := checkedAddInt64(*targets[i], values[i])
		if err != nil {
			return err
		}
		*targets[i] = v
	}
	return nil
}
func (a *metricAccumulator) addBusiness(row model.SupplierReportBusinessUsageRow) error {
	return a.add(row.BusinessRequestCount, row.UnattributedRequestCount, row.OfficialListKnownCount, row.OfficialListMicroUsd, row.SalesKnownCount, row.SalesMicroUsd, row.ProcurementCostKnownCount, row.ProcurementCostMicroUsd, row.GrossProfitKnownCount, row.GrossProfitMicroUsd, row.GrossMarginEligibleCount, row.GrossMarginEligibleSalesMicroUsd)
}
func (a *metricAccumulator) addChannel(row model.SupplierReportChannelUsageRow) error {
	return a.add(row.BusinessRequestCount, row.UnattributedRequestCount, row.OfficialListKnownCount, row.OfficialListMicroUsd, row.SalesKnownCount, row.SalesMicroUsd, row.ProcurementCostKnownCount, row.ProcurementCostMicroUsd, row.GrossProfitKnownCount, row.GrossProfitMicroUsd, row.GrossMarginEligibleCount, row.GrossMarginEligibleSalesMicroUsd)
}
func (a *metricAccumulator) addBusinessRow(row model.SupplierReportBreakdownRow) error {
	return a.add(row.BusinessRequestCount, row.UnattributedRequestCount, row.OfficialListKnownCount, row.OfficialListMicroUsd, row.SalesKnownCount, row.SalesMicroUsd, row.ProcurementCostKnownCount, row.ProcurementCostMicroUsd, row.GrossProfitKnownCount, row.GrossProfitMicroUsd, row.GrossMarginEligibleCount, row.GrossMarginEligibleSalesMicroUsd)
}
func (a *metricAccumulator) addInternal(row model.SupplierReportInternalUsageRow) error {
	return a.add(row.InternalRequestCount, row.UnattributedRequestCount, row.OfficialListKnownCount, row.OfficialListMicroUsd, 0, 0, row.ProcurementCostKnownCount, row.ProcurementCostMicroUsd, 0, 0, 0, 0)
}
func (a metricAccumulator) metrics() SupplierReportMetrics {
	return SupplierReportMetrics{RequestCount: a.requests, UnattributedRequestCount: a.unattributed, OfficialList: SupplierReportMoney{a.officialKnown, a.official}, Sales: SupplierReportMoney{a.salesKnown, a.sales}, ProcurementCost: SupplierReportMoney{a.procurementKnown, a.procurement}, GrossProfit: SupplierReportMoney{a.grossKnown, a.gross}, GrossMarginEligibleCount: a.eligibleCount, GrossMarginEligibleSalesMicroUsd: a.eligibleSales, GrossMargin: ratioString(a.gross, a.eligibleSales), GrossMarginEligibleCoverage: ratioString(a.eligibleCount, a.requests)}
}

type usageAccumulator struct {
	business, internal metricAccumulator
	businessRows       []model.SupplierReportBusinessUsageRow
	internalRows       []model.SupplierReportInternalUsageRow
}

func (s *SupplierReportService) loadUsage(ctx context.Context, filter model.SupplierReportFilter, daily bool) (*usageAccumulator, error) {
	a := &usageAccumulator{}
	rows, err := s.store.QueryBusinessUsage(ctx, filter, daily)
	if err != nil {
		return nil, err
	}
	a.businessRows = rows
	for _, r := range rows {
		if err := a.business.addBusiness(r); err != nil {
			return nil, err
		}
	}
	if len(filter.ChannelIds) == 0 {
		internal, err := s.store.QueryInternalUsage(ctx, filter, daily)
		if err != nil {
			return nil, err
		}
		a.internalRows = internal
		for _, r := range internal {
			if err := a.internal.addInternal(r); err != nil {
				return nil, err
			}
		}
	}
	return a, nil
}
func (s *SupplierReportService) loadUsageByContract(ctx context.Context, filter model.SupplierReportFilter) (*usageAccumulator, error) {
	a := &usageAccumulator{}
	rows, err := s.store.QueryBusinessUsageByContract(ctx, filter)
	if err != nil {
		return nil, err
	}
	a.businessRows = rows
	for _, r := range rows {
		if err := a.business.addBusiness(r); err != nil {
			return nil, err
		}
	}
	if len(filter.ChannelIds) == 0 {
		internal, err := s.store.QueryInternalUsageByContract(ctx, filter)
		if err != nil {
			return nil, err
		}
		a.internalRows = internal
		for _, r := range internal {
			if err := a.internal.addInternal(r); err != nil {
				return nil, err
			}
		}
	}
	return a, nil
}
func (a *usageAccumulator) addBusiness(r model.SupplierReportBusinessUsageRow) error {
	return a.business.addBusiness(r)
}
func (a *usageAccumulator) addInternal(r model.SupplierReportInternalUsageRow) error {
	return a.internal.addInternal(r)
}
func (a *usageAccumulator) byContract() (map[int]*usageAccumulator, error) {
	result := map[int]*usageAccumulator{}
	for _, r := range a.businessRows {
		i := result[r.ContractId]
		if i == nil {
			i = &usageAccumulator{}
			result[r.ContractId] = i
		}
		if err := i.addBusiness(r); err != nil {
			return nil, err
		}
	}
	for _, r := range a.internalRows {
		i := result[r.ContractId]
		if i == nil {
			i = &usageAccumulator{}
			result[r.ContractId] = i
		}
		if err := i.addInternal(r); err != nil {
			return nil, err
		}
	}
	return result, nil
}
func buildContractRow(c model.SupplierReportContractCatalogRow, r contractRuntime, u *usageAccumulator, internalDimensionAvailable bool) (SupplierReportContractRow, error) {
	if u == nil {
		u = &usageAccumulator{}
	}
	remaining, err := checkedSubInt64(r.inventory, r.consumed)
	if err != nil {
		return SupplierReportContractRow{}, err
	}
	row := SupplierReportContractRow{ContractId: c.ContractId, SupplierId: c.SupplierId, SupplierName: c.SupplierName, SupplierStatus: c.SupplierStatus, ContractName: c.ContractName, ContractNo: c.ContractNo, ContractStatus: c.ContractStatus, Remark: c.Remark, CurrentRateVersionId: c.CurrentRateVersionId, ProcurementMultiplierPpm: c.ProcurementMultiplierPpm, RpmLimit: c.RpmLimit, TpmLimit: c.TpmLimit, MaxConcurrency: c.MaxConcurrency, LinkedChannelCount: r.channelCount, TotalInventoryMicroUsd: r.inventory, OfficialListConsumedMicroUsd: r.consumed, RemainingInventoryMicroUsd: remaining, UtilizationRate: ratioString(r.consumed, r.inventory), Oversold: remaining < 0, Business: u.business.metrics(), InternalDimensionAvailable: internalDimensionAvailable}
	if internalDimensionAvailable {
		internal := u.internal.metrics()
		row.Internal = &internal
		totalProcurementCost, combineErr := combineMoney(row.Business.ProcurementCost, internal.ProcurementCost)
		if combineErr != nil {
			return SupplierReportContractRow{}, combineErr
		}
		row.TotalProcurementCost = &totalProcurementCost
	}
	return row, nil
}
func catalogIDs(rows []model.SupplierReportContractCatalogRow) []int {
	ids := make([]int, len(rows))
	for i := range rows {
		ids[i] = rows[i].ContractId
	}
	return ids
}
func combineMoney(a, b SupplierReportMoney) (SupplierReportMoney, error) {
	count, err := checkedAddInt64(a.KnownCount, b.KnownCount)
	if err != nil {
		return SupplierReportMoney{}, err
	}
	amount, err := checkedAddInt64(a.MicroUsd, b.MicroUsd)
	return SupplierReportMoney{count, amount}, err
}
func checkedAddInt64(a, b int64) (int64, error) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0, ErrSupplierReportOverflow
	}
	return a + b, nil
}
func checkedSubInt64(a, b int64) (int64, error) {
	if b == math.MinInt64 {
		return 0, ErrSupplierReportOverflow
	}
	return checkedAddInt64(a, -b)
}
func ratioString(n, d int64) *string {
	if d == 0 {
		return nil
	}
	v := decimal.NewFromInt(n).Div(decimal.NewFromInt(d)).Round(8).String()
	return &v
}
func mustSupplierReportLocation() *time.Location {
	location, err := time.LoadLocation(SupplierReportTimezone)
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*3600)
	}
	return location
}
