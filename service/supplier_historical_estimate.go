package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const SupplierHistoricalPageSize = 5000

var (
	ErrSupplierHistoricalCommandInvalid = errors.New("invalid supplier historical estimate command")
	ErrSupplierHistoricalMappingInvalid = errors.New("invalid supplier historical channel mapping")
	ErrSupplierHistoricalMoneyOverflow  = model.ErrSupplierHistoricalMoneyOverflow
	ErrSupplierHistoricalImportNotReady = errors.New("supplier historical import series is not ready")
)

type SupplierHistoricalChannelMapping struct {
	ChannelId                int   `json:"channel_id"`
	SupplierId               int   `json:"supplier_id"`
	ContractId               int   `json:"contract_id"`
	RateVersionId            int   `json:"rate_version_id"`
	ProcurementMultiplierPpm int64 `json:"procurement_multiplier_ppm"`
}

type SupplierHistoricalImportCommand struct {
	StartDate       string                             `json:"start_date"`
	EndDate         string                             `json:"end_date"`
	QuotaPerUnit    string                             `json:"quota_per_unit"`
	ExcludedUserIds []int                              `json:"excluded_user_ids"`
	ChannelMappings []SupplierHistoricalChannelMapping `json:"channel_mappings"`
	Reason          string                             `json:"reason"`
	Method          string                             `json:"method"`
}

type SupplierHistoricalRunResult struct {
	ImportId  int64 `json:"import_id"`
	Processed int64 `json:"processed"`
	Completed bool  `json:"completed"`
	NoWork    bool  `json:"no_work"`
}

type SupplierHistoricalImportView struct {
	model.SupplierHistoricalImport
	Command       SupplierHistoricalImportCommand `json:"command"`
	EstimateOnly  bool                            `json:"estimate_only"`
	CoverageScope string                          `json:"coverage_scope"`
	Assumptions   []string                        `json:"assumptions"`
}

func BuildSupplierHistoricalImportView(item model.SupplierHistoricalImport) (SupplierHistoricalImportView, error) {
	command, err := decodeSupplierHistoricalCommand(item.CommandJSON)
	if err != nil {
		return SupplierHistoricalImportView{}, err
	}
	return SupplierHistoricalImportView{
		SupplierHistoricalImport: item,
		Command:                  command,
		EstimateOnly:             true,
		CoverageScope:            "historical_consume_logs_v1",
		Assumptions: []string{
			"sales_equals_quota_divided_by_frozen_quota_per_unit",
			"official_list_requires_valid_logged_group_ratio",
			"procurement_cost_requires_explicit_channel_mapping",
			"authoritative_reports_and_inventory_are_unchanged",
		},
	}, nil
}

func ListCompletedSupplierHistoricalSeries(ctx context.Context, mainDB *gorm.DB, importId int64, startDate, endDate string, cursor model.SupplierHistoricalSeriesCursor, limit int) ([]model.SupplierHistoricalSeriesPoint, bool, error) {
	item, err := model.GetSupplierHistoricalImport(ctx, mainDB, importId)
	if err != nil {
		return nil, false, err
	}
	if item.Status != model.SupplierHistoricalImportStatusCompleted {
		return nil, false, ErrSupplierHistoricalImportNotReady
	}
	return model.ListSupplierHistoricalSeries(ctx, mainDB, importId, startDate, endDate, cursor, limit)
}

func CreateSupplierHistoricalEstimate(ctx context.Context, mainDB *gorm.DB, command SupplierHistoricalImportCommand, actor int, idempotencyKey string) (model.SupplierHistoricalImport, error) {
	canonical, dayStart, dayEnd, err := canonicalizeSupplierHistoricalCommand(command)
	if err != nil || actor <= 0 || strings.TrimSpace(idempotencyKey) == "" || len(strings.TrimSpace(idempotencyKey)) > 128 {
		return model.SupplierHistoricalImport{}, ErrSupplierHistoricalCommandInvalid
	}
	if err := validateSupplierHistoricalMappings(ctx, mainDB, canonical.ChannelMappings); err != nil {
		return model.SupplierHistoricalImport{}, err
	}
	commandJSON, err := common.Marshal(canonical)
	if err != nil {
		return model.SupplierHistoricalImport{}, err
	}
	digest := sha256.Sum256(commandJSON)
	excludedJSON, err := common.Marshal(canonical.ExcludedUserIds)
	if err != nil {
		return model.SupplierHistoricalImport{}, err
	}
	mappingsJSON, err := common.Marshal(canonical.ChannelMappings)
	if err != nil {
		return model.SupplierHistoricalImport{}, err
	}
	return model.CreateSupplierHistoricalImport(ctx, mainDB, model.SupplierHistoricalImportCreate{
		CommandHash: hex.EncodeToString(digest[:]), CommandJSON: string(commandJSON), IdempotencyKey: strings.TrimSpace(idempotencyKey),
		CreatedBy: actor, Method: canonical.Method, Reason: canonical.Reason, StartDate: canonical.StartDate, EndDate: canonical.EndDate,
		DayStart: dayStart, DayEnd: dayEnd, QuotaPerUnit: canonical.QuotaPerUnit,
		ExcludedUserIdsJSON: string(excludedJSON), ChannelMappingsJSON: string(mappingsJSON),
	})
}

func canonicalizeSupplierHistoricalCommand(command SupplierHistoricalImportCommand) (SupplierHistoricalImportCommand, int64, int64, error) {
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	if err != nil {
		return command, 0, 0, err
	}
	start, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(command.StartDate), location)
	if err != nil || start.Format("2006-01-02") != strings.TrimSpace(command.StartDate) {
		return command, 0, 0, ErrSupplierHistoricalCommandInvalid
	}
	end, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(command.EndDate), location)
	if err != nil || end.Format("2006-01-02") != strings.TrimSpace(command.EndDate) || !end.After(start) || end.Sub(start) > 366*24*time.Hour {
		return command, 0, 0, ErrSupplierHistoricalCommandInvalid
	}
	qpu, err := decimal.NewFromString(strings.TrimSpace(command.QuotaPerUnit))
	if err != nil || !qpu.IsPositive() {
		return command, 0, 0, ErrSupplierHistoricalCommandInvalid
	}
	reason := strings.TrimSpace(command.Reason)
	if reason == "" {
		return command, 0, 0, ErrSupplierHistoricalCommandInvalid
	}
	excludedSet := make(map[int]struct{}, len(command.ExcludedUserIds))
	for _, id := range command.ExcludedUserIds {
		if id <= 0 {
			return command, 0, 0, ErrSupplierHistoricalCommandInvalid
		}
		excludedSet[id] = struct{}{}
	}
	excluded := make([]int, 0, len(excludedSet))
	for id := range excludedSet {
		excluded = append(excluded, id)
	}
	sort.Ints(excluded)
	mappings := append([]SupplierHistoricalChannelMapping(nil), command.ChannelMappings...)
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].ChannelId < mappings[j].ChannelId })
	for index, mapping := range mappings {
		if mapping.ChannelId <= 0 || mapping.SupplierId <= 0 || mapping.ContractId <= 0 || mapping.RateVersionId <= 0 ||
			mapping.ProcurementMultiplierPpm < 0 || mapping.ProcurementMultiplierPpm > 1_000_000 ||
			(index > 0 && mappings[index-1].ChannelId == mapping.ChannelId) {
			return command, 0, 0, ErrSupplierHistoricalMappingInvalid
		}
	}
	return SupplierHistoricalImportCommand{
		StartDate: start.Format("2006-01-02"), EndDate: end.Format("2006-01-02"), QuotaPerUnit: qpu.String(),
		ExcludedUserIds: excluded, ChannelMappings: mappings, Reason: reason, Method: model.SupplierHistoricalMethodLogEstimateV1,
	}, start.Unix(), end.Unix(), nil
}

func validateSupplierHistoricalMappings(ctx context.Context, mainDB *gorm.DB, mappings []SupplierHistoricalChannelMapping) error {
	if mainDB == nil {
		return model.ErrDatabase
	}
	rateIds := make([]int, len(mappings))
	for index := range mappings {
		rateIds[index] = mappings[index].RateVersionId
	}
	chains, err := model.ListSupplierHistoricalRateChains(ctx, mainDB, rateIds)
	if err != nil {
		return err
	}
	byRate := make(map[int]model.SupplierHistoricalRateChain, len(chains))
	for _, chain := range chains {
		byRate[chain.RateVersionId] = chain
	}
	for _, mapping := range mappings {
		chain, ok := byRate[mapping.RateVersionId]
		if !ok || chain.SupplierId != mapping.SupplierId || chain.ContractId != mapping.ContractId || chain.ProcurementMultiplierPpm != mapping.ProcurementMultiplierPpm {
			return ErrSupplierHistoricalMappingInvalid
		}
	}
	return nil
}

func RunSupplierHistoricalEstimatePage(ctx context.Context, mainDB, logDB *gorm.DB, owner string, leaseDuration time.Duration) (SupplierHistoricalRunResult, error) {
	lease, err := model.AcquireSupplierHistoricalImport(ctx, mainDB, 0, owner, leaseDuration)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SupplierHistoricalRunResult{NoWork: true}, nil
	}
	if err != nil {
		return SupplierHistoricalRunResult{}, err
	}
	if lease.AlreadyDone {
		return SupplierHistoricalRunResult{ImportId: lease.ImportId, Completed: true}, nil
	}
	item, err := model.GetSupplierHistoricalImport(ctx, mainDB, lease.ImportId)
	if err != nil {
		return SupplierHistoricalRunResult{}, err
	}
	if !lease.Started {
		stats, statsErr := model.FreezeSupplierHistoricalSourceStats(ctx, logDB, item.DayStart, item.DayEnd)
		if statsErr != nil {
			return SupplierHistoricalRunResult{}, statsErr
		}
		if err := model.FreezeSupplierHistoricalImport(ctx, mainDB, lease, stats.SourceMaxLogId, stats.CandidateCount); err != nil {
			return SupplierHistoricalRunResult{}, err
		}
		lease.Started = true
		lease.SourceMaxLogId = stats.SourceMaxLogId
		lease.CandidateCount = stats.CandidateCount
	}
	command, err := decodeSupplierHistoricalCommand(item.CommandJSON)
	if err != nil {
		_ = model.FailSupplierHistoricalImport(ctx, mainDB, lease, err)
		return SupplierHistoricalRunResult{}, err
	}
	rows, err := model.ListSupplierHistoricalSourcePage(ctx, logDB, item.DayStart, item.DayEnd, lease.SourceMaxLogId, lease.CursorCreatedAt, lease.CursorId, SupplierHistoricalPageSize)
	if err != nil {
		return SupplierHistoricalRunResult{}, err
	}
	summaries, err := aggregateSupplierHistoricalPage(lease.ImportId, rows, command)
	if err != nil {
		_ = model.FailSupplierHistoricalImport(ctx, mainDB, lease, err)
		return SupplierHistoricalRunResult{}, err
	}
	cursorCreatedAt, cursorId := lease.CursorCreatedAt, lease.CursorId
	if len(rows) > 0 {
		cursorCreatedAt = rows[len(rows)-1].CreatedAt
		cursorId = rows[len(rows)-1].Id
	}
	if err := model.CommitSupplierHistoricalImportPage(ctx, mainDB, lease, summaries, cursorCreatedAt, cursorId, int64(len(rows))); err != nil {
		return SupplierHistoricalRunResult{}, err
	}
	result := SupplierHistoricalRunResult{ImportId: lease.ImportId, Processed: int64(len(rows))}
	if len(rows) == SupplierHistoricalPageSize {
		return result, nil
	}
	verified, err := model.CountSupplierHistoricalFrozenSource(ctx, logDB, item.DayStart, item.DayEnd, lease.SourceMaxLogId)
	if err != nil {
		return SupplierHistoricalRunResult{}, err
	}
	if err := model.CompleteSupplierHistoricalImport(ctx, mainDB, lease, verified); err != nil {
		if errors.Is(err, model.ErrSupplierHistoricalImportSourceChanged) {
			_ = model.FailSupplierHistoricalImport(ctx, mainDB, lease, err)
		}
		return SupplierHistoricalRunResult{}, err
	}
	result.Completed = true
	return result, nil
}

func decodeSupplierHistoricalCommand(value string) (SupplierHistoricalImportCommand, error) {
	var command SupplierHistoricalImportCommand
	if err := common.UnmarshalJsonStr(value, &command); err != nil {
		return command, err
	}
	return command, nil
}

func aggregateSupplierHistoricalPage(importId int64, rows []model.SupplierHistoricalSourceLog, command SupplierHistoricalImportCommand) ([]model.SupplierHistoricalDailySummary, error) {
	qpu, err := decimal.NewFromString(command.QuotaPerUnit)
	if err != nil || !qpu.IsPositive() {
		return nil, ErrSupplierHistoricalCommandInvalid
	}
	excluded := make(map[int]struct{}, len(command.ExcludedUserIds))
	for _, userId := range command.ExcludedUserIds {
		excluded[userId] = struct{}{}
	}
	mappings := make(map[int]SupplierHistoricalChannelMapping, len(command.ChannelMappings))
	for _, mapping := range command.ChannelMappings {
		mappings[mapping.ChannelId] = mapping
	}
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	if err != nil {
		return nil, err
	}
	byDimension := make(map[string]*model.SupplierHistoricalDailySummary)
	for _, row := range rows {
		_, internal := excluded[row.UserId]
		mapping, assigned := mappings[row.ChannelId]
		dateTime := time.Unix(row.CreatedAt, 0).In(location)
		date := dateTime.Format("2006-01-02")
		bucketStart := time.Date(dateTime.Year(), dateTime.Month(), dateTime.Day(), 0, 0, 0, 0, location).Unix()
		dimension := supplierHistoricalDimension(importId, date, row, mapping, assigned, internal)
		summary := byDimension[dimension]
		if summary == nil {
			summary = newSupplierHistoricalSummary(importId, date, bucketStart, dimension, row, mapping, assigned, internal)
			byDimension[dimension] = summary
		}
		summary.SourceRequestCount++
		if !assigned {
			summary.UnassignedRequestCount++
		}
		if err := accumulateSupplierHistoricalMoney(summary, row, qpu, mapping, assigned, internal); err != nil {
			return nil, err
		}
	}
	keys := make([]string, 0, len(byDimension))
	for key := range byDimension {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]model.SupplierHistoricalDailySummary, 0, len(keys))
	for _, key := range keys {
		result = append(result, *byDimension[key])
	}
	return result, nil
}

func supplierHistoricalDimension(importId int64, date string, row model.SupplierHistoricalSourceLog, mapping SupplierHistoricalChannelMapping, assigned, internal bool) string {
	value := "business|unassigned|" + strconv.Itoa(row.ChannelId) + "|" + strings.TrimSpace(row.ModelName)
	if assigned {
		value = fmt.Sprintf("business|%d|%d|%d|%d|%s", mapping.SupplierId, mapping.ContractId, mapping.RateVersionId, row.ChannelId, strings.TrimSpace(row.ModelName))
	}
	if internal {
		value = "internal|unassigned"
		if assigned {
			value = fmt.Sprintf("internal|%d|%d", mapping.SupplierId, mapping.ContractId)
		}
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d|%s|%s", importId, date, value)))
	return hex.EncodeToString(digest[:])
}

func newSupplierHistoricalSummary(importId int64, date string, bucketStart int64, dimension string, row model.SupplierHistoricalSourceLog, mapping SupplierHistoricalChannelMapping, assigned, internal bool) *model.SupplierHistoricalDailySummary {
	summary := &model.SupplierHistoricalDailySummary{
		ImportId: importId, Date: date, BucketStart: bucketStart, DimensionKey: dimension,
		StatisticsScope: "business", ChannelId: row.ChannelId, ModelName: strings.TrimSpace(row.ModelName),
		DataQuality: model.SupplierHistoricalDataQualityEstimated,
	}
	if assigned {
		multiplier := mapping.ProcurementMultiplierPpm
		summary.SupplierId = mapping.SupplierId
		summary.ContractId = mapping.ContractId
		summary.RateVersionId = mapping.RateVersionId
		summary.ProcurementMultiplierPpm = &multiplier
	}
	if internal {
		summary.StatisticsScope = "internal"
		summary.ChannelId = 0
		summary.ModelName = ""
		summary.RateVersionId = 0
		summary.ProcurementMultiplierPpm = nil
	}
	return summary
}

func accumulateSupplierHistoricalMoney(summary *model.SupplierHistoricalDailySummary, row model.SupplierHistoricalSourceLog, qpu decimal.Decimal, mapping SupplierHistoricalChannelMapping, assigned, internal bool) error {
	salesExact, salesKnown := supplierHistoricalSalesExact(row.Quota, qpu)
	sales, salesSafe := supplierHistoricalMicroUSD(salesExact)
	if salesKnown && !salesSafe {
		return ErrSupplierHistoricalMoneyOverflow
	}
	groupRatio, groupKnown := supplierHistoricalGroupRatio(row.Other)
	official, officialKnown := int64(0), salesKnown && groupKnown
	var officialExact decimal.Decimal
	if officialKnown {
		officialExact = salesExact.Div(groupRatio)
		official, officialKnown = supplierHistoricalMicroUSD(officialExact)
		if !officialKnown {
			return ErrSupplierHistoricalMoneyOverflow
		}
	}
	cost, costKnown := int64(0), officialKnown && assigned
	if costKnown {
		costExact := officialExact.Mul(decimal.NewFromInt(mapping.ProcurementMultiplierPpm)).Div(decimal.NewFromInt(1_000_000))
		cost, costKnown = supplierHistoricalMicroUSD(costExact)
		if !costKnown {
			return ErrSupplierHistoricalMoneyOverflow
		}
	}
	if officialKnown {
		summary.OfficialListKnownCount++
		if value, ok := supplierHistoricalCheckedAdd(summary.OfficialListMicroUsd, official); ok {
			summary.OfficialListMicroUsd = value
		} else {
			return ErrSupplierHistoricalMoneyOverflow
		}
	} else {
		summary.OfficialListUnknownCount++
	}
	if costKnown {
		summary.ProcurementCostKnownCount++
		if value, ok := supplierHistoricalCheckedAdd(summary.ProcurementCostMicroUsd, cost); ok {
			summary.ProcurementCostMicroUsd = value
		} else {
			return ErrSupplierHistoricalMoneyOverflow
		}
	} else {
		summary.ProcurementCostUnknownCount++
	}
	if !internal && salesKnown {
		summary.SalesKnownCount++
		if value, ok := supplierHistoricalCheckedAdd(summary.SalesMicroUsd, sales); ok {
			summary.SalesMicroUsd = value
		} else {
			return ErrSupplierHistoricalMoneyOverflow
		}
	} else {
		summary.SalesUnknownCount++
	}
	if !internal && salesKnown && costKnown {
		summary.GrossProfitKnownCount++
		gross, ok := supplierHistoricalCheckedAdd(sales, -cost)
		if !ok {
			return ErrSupplierHistoricalMoneyOverflow
		}
		if value, ok := supplierHistoricalCheckedAdd(summary.GrossProfitMicroUsd, gross); ok {
			summary.GrossProfitMicroUsd = value
		} else {
			return ErrSupplierHistoricalMoneyOverflow
		}
	} else {
		summary.GrossProfitUnknownCount++
	}
	return nil
}

func supplierHistoricalSalesExact(quota int64, qpu decimal.Decimal) (decimal.Decimal, bool) {
	if quota < 0 || !qpu.IsPositive() {
		return decimal.Zero, false
	}
	return decimal.NewFromInt(quota).Mul(decimal.NewFromInt(1_000_000)).Div(qpu), true
}

func supplierHistoricalMicroUSD(value decimal.Decimal) (int64, bool) {
	rounded := value.Round(0)
	max := decimal.NewFromInt(math.MaxInt64)
	min := decimal.NewFromInt(math.MinInt64)
	if rounded.GreaterThan(max) || rounded.LessThan(min) {
		return 0, false
	}
	return rounded.IntPart(), true
}

func supplierHistoricalGroupRatio(other string) (decimal.Decimal, bool) {
	var payload struct {
		GroupRatio any `json:"group_ratio"`
	}
	if err := common.UnmarshalJsonStr(other, &payload); err != nil {
		return decimal.Zero, false
	}
	var ratio decimal.Decimal
	var err error
	switch value := payload.GroupRatio.(type) {
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return decimal.Zero, false
		}
		ratio = decimal.NewFromFloat(value)
	case string:
		ratio, err = decimal.NewFromString(strings.TrimSpace(value))
	default:
		return decimal.Zero, false
	}
	return ratio, err == nil && ratio.IsPositive()
}

func supplierHistoricalCheckedAdd(left, right int64) (int64, bool) {
	if (right > 0 && left > math.MaxInt64-right) || (right < 0 && left < math.MinInt64-right) {
		return 0, false
	}
	return left + right, true
}
