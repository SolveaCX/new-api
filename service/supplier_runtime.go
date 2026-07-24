package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"gorm.io/gorm"
)

const (
	SupplierDailyBatchTimezone       = "Asia/Shanghai"
	supplierDailyLeaseDuration       = 30 * time.Minute
	SupplierDailyCloseGrace          = 2 * time.Hour
	SupplierDailyCatchUpMaxDays      = 1
	SupplierDataQualityAuthoritative = "authoritative"
	SupplierDataQualityUnattributed  = "unattributed"
)

type supplierAccountingLogEnvelope struct {
	SupplierAccountingV1 json.RawMessage `json:"supplier_accounting_v1"`
}

type SupplierDailyBatchCatchUpResult struct {
	ProcessedDays int    `json:"processed_days"`
	RemainingWork bool   `json:"remaining_work"`
	NextBatchDate string `json:"next_batch_date"`
}

func CatchUpSupplierDailyBatches(ctx context.Context, mainDB, logDB *gorm.DB, owner string, now time.Time) (SupplierDailyBatchCatchUpResult, error) {
	result := SupplierDailyBatchCatchUpResult{}
	if mainDB == nil || logDB == nil || strings.TrimSpace(owner) == "" {
		return result, model.ErrDatabase
	}
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	if err != nil {
		return result, err
	}
	localNow := now.In(location)
	today := beginningOfSupplierDay(localNow)
	if localNow.Before(today.Add(SupplierDailyCloseGrace)) {
		return result, nil
	}
	target := today.AddDate(0, 0, -1)
	next, err := nextSupplierDailyBatchDate(ctx, mainDB, target, location)
	if err != nil {
		return result, err
	}
	if next.After(target) {
		return result, nil
	}
	if err := RunSupplierDailyBatch(ctx, mainDB, logDB, next.Format("2006-01-02"), owner, now, false); err != nil {
		if errors.Is(err, model.ErrSupplierDailyBatchBusy) {
			return result, nil
		}
		return result, err
	}
	result.ProcessedDays = 1
	next, err = nextSupplierDailyBatchDate(ctx, mainDB, target, location)
	if err != nil {
		return result, err
	}
	if !next.After(target) {
		result.RemainingWork = true
		result.NextBatchDate = next.Format("2006-01-02")
	}
	return result, nil
}

func nextSupplierDailyBatchDate(ctx context.Context, mainDB *gorm.DB, target time.Time, location *time.Location) (time.Time, error) {
	incomplete, err := model.EarliestIncompleteSupplierDailyBatch(ctx, mainDB)
	if err != nil {
		return time.Time{}, err
	}
	if incomplete != nil {
		return time.ParseInLocation("2006-01-02", incomplete.BatchDate, location)
	}
	completed, err := model.LatestCompletedSupplierDailyBatch(ctx, mainDB)
	if err != nil {
		return time.Time{}, err
	}
	if completed != nil {
		last, err := time.ParseInLocation("2006-01-02", completed.BatchDate, location)
		if err != nil {
			return time.Time{}, err
		}
		return last.AddDate(0, 0, 1), nil
	}
	if cutover, ok, err := configuredSupplierAccountingCutover(); err != nil {
		return time.Time{}, err
	} else if ok {
		return beginningOfSupplierDay(time.Unix(cutover, 0).In(location)), nil
	}
	return target, nil
}

func configuredSupplierAccountingCutover() (int64, bool, error) {
	raw := strings.TrimSpace(os.Getenv("SUPPLIER_ACCOUNTING_CUTOVER_AT"))
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, false, fmt.Errorf("invalid SUPPLIER_ACCOUNTING_CUTOVER_AT %q", raw)
	}
	return value, true, nil
}

func RunSupplierDailyBatch(ctx context.Context, mainDB, logDB *gorm.DB, batchDate, owner string, now time.Time, force bool) error {
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	if err != nil {
		return err
	}
	day, err := time.ParseInLocation("2006-01-02", batchDate, location)
	if err != nil || force || !day.Before(beginningOfSupplierDay(now.In(location))) {
		return fmt.Errorf("invalid supplier batch date %q", batchDate)
	}
	dayEnd := day.AddDate(0, 0, 1)
	lease, err := model.AcquireSupplierDailyBatch(ctx, mainDB, batchDate, day.Unix(), dayEnd.Unix(), owner, now, supplierDailyLeaseDuration, false)
	if err != nil || lease.AlreadyDone {
		return err
	}
	if err := scanSupplierDailyBatch(ctx, mainDB, logDB, lease, day); err != nil {
		_ = model.FailSupplierDailyBatch(context.Background(), mainDB, lease, err)
		return err
	}
	if err := model.CompleteSupplierDailyBatch(ctx, mainDB, lease, now); err != nil {
		_ = model.FailSupplierDailyBatch(context.Background(), mainDB, lease, err)
		return err
	}
	return nil
}

func scanSupplierDailyBatch(ctx context.Context, mainDB, logDB *gorm.DB, lease model.SupplierDailyBatchLease, day time.Time) error {
	dayEnd := day.AddDate(0, 0, 1)
	for {
		rows, err := model.ScanSupplierAccountingLogPage(ctx, logDB, day.Unix(), dayEnd.Unix(), lease.CursorCreatedAt, lease.CursorId, model.SupplierDailyLogPageSize)
		if err != nil {
			return fmt.Errorf("scan supplier accounting logs: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}
		accumulators := make(map[string]*model.SupplierUsageDailySummary, len(rows))
		var snapshotCount int64
		for _, logRow := range rows {
			snapshot, captured, err := parseSupplierAccountingLog(logRow.Other)
			if err != nil {
				return fmt.Errorf("parse supplier accounting log %d: %w", logRow.Id, err)
			}
			if !captured {
				continue
			}
			if err := addSupplierDailySnapshot(accumulators, lease.BatchDate, day.Unix(), logRow, snapshot); err != nil {
				return fmt.Errorf("aggregate supplier accounting log %d: %w", logRow.Id, err)
			}
			snapshotCount++
		}
		summaries := make([]model.SupplierUsageDailySummary, 0, len(accumulators))
		for _, summary := range accumulators {
			summaries = append(summaries, *summary)
		}
		last := rows[len(rows)-1]
		if err := model.PersistSupplierDailyBatchPage(ctx, mainDB, lease, summaries, last.CreatedAt, last.Id, int64(len(rows)), snapshotCount, supplierDailyLeaseDuration); err != nil {
			return err
		}
		lease.CursorCreatedAt = last.CreatedAt
		lease.CursorId = last.Id
		if len(rows) < model.SupplierDailyLogPageSize {
			return nil
		}
	}
}

func parseSupplierAccountingLog(other string) (types.SupplierAccountingLogSnapshotV1, bool, error) {
	if strings.TrimSpace(other) == "" || !strings.Contains(other, `"supplier_accounting_v1"`) {
		return types.SupplierAccountingLogSnapshotV1{}, false, nil
	}
	var envelope supplierAccountingLogEnvelope
	if err := common.Unmarshal([]byte(other), &envelope); err != nil {
		return types.SupplierAccountingLogSnapshotV1{}, false, err
	}
	if len(envelope.SupplierAccountingV1) == 0 || string(envelope.SupplierAccountingV1) == "null" {
		return types.SupplierAccountingLogSnapshotV1{}, false, nil
	}
	parsed, err := types.ParseSupplierAccountingEnvelopeV1JSON(envelope.SupplierAccountingV1)
	if err != nil {
		return types.SupplierAccountingLogSnapshotV1{}, false, err
	}
	if err := ValidateSupplierAccountingEnvelopeV1(parsed); err != nil {
		return types.SupplierAccountingLogSnapshotV1{}, false, err
	}
	if parsed.Disposition != types.SupplierAccountingDispositionCaptured || parsed.Captured == nil {
		return types.SupplierAccountingLogSnapshotV1{}, false, nil
	}
	return *parsed.Captured, true, nil
}

func addSupplierDailySnapshot(accumulators map[string]*model.SupplierUsageDailySummary, batchDate string, bucketStart int64, logRow model.SupplierAccountingLogRow, snapshot types.SupplierAccountingLogSnapshotV1) error {
	pricingMode, err := supplierPricingModeFromProvenance(snapshot.PricingProvenance)
	if err != nil {
		return err
	}
	quality := SupplierDataQualityAuthoritative
	if strings.TrimSpace(snapshot.QualityReason) != "" {
		quality = SupplierDataQualityUnattributed
	}
	modelName := logRow.ModelName
	bindingVersionID := snapshot.BindingVersionId
	rateVersionID := snapshot.RateVersionId
	channelID := logRow.ChannelId
	salesMultiplier := snapshot.SalesMultiplierPpm
	if snapshot.StatisticsScope == string(types.SupplierStatisticsScopeInternal) {
		bindingVersionID = 0
		rateVersionID = 0
		channelID = 0
		modelName = ""
		salesMultiplier = nil
		pricingMode = ""
	}
	keyText := strings.Join([]string{
		batchDate, strconv.Itoa(snapshot.SupplierId), strconv.Itoa(snapshot.ContractId), strconv.Itoa(bindingVersionID),
		strconv.Itoa(rateVersionID), strconv.Itoa(channelID), modelName, pointerInt64String(salesMultiplier), pricingMode, snapshot.StatisticsScope, quality,
	}, "|")
	digest := sha256.Sum256([]byte(keyText))
	key := hex.EncodeToString(digest[:])
	row := accumulators[key]
	if row == nil {
		row = &model.SupplierUsageDailySummary{
			BatchDate: batchDate, DimensionKey: key, BucketStart: bucketStart,
			SupplierId: snapshot.SupplierId, ContractId: snapshot.ContractId, BindingVersionId: bindingVersionID,
			RateVersionId: rateVersionID, ChannelId: channelID, ModelName: modelName,
			SalesMultiplierPpm: cloneSupplierInt64(salesMultiplier), PricingMode: pricingMode, StatisticsScope: snapshot.StatisticsScope, DataQuality: quality,
		}
		accumulators[key] = row
	}
	if err := addInt64(&row.RequestCount, 1); err != nil {
		return err
	}
	if quality == SupplierDataQualityUnattributed {
		if err := addInt64(&row.UnattributedRequestCount, 1); err != nil {
			return err
		}
	}
	if err := addKnownAmount(&row.OfficialListKnownCount, &row.OfficialListMicroUsd, snapshot.OfficialListMicroUsd); err != nil {
		return err
	}
	if err := addKnownAmount(&row.ProcurementCostKnownCount, &row.ProcurementCostMicroUsd, snapshot.ProcurementCostMicroUsd); err != nil {
		return err
	}
	if snapshot.StatisticsScope == string(types.SupplierStatisticsScopeInternal) {
		return nil
	}
	if err := addKnownAmount(&row.SalesKnownCount, &row.SalesMicroUsd, snapshot.SalesMicroUsd); err != nil {
		return err
	}
	if err := addKnownAmount(&row.GrossProfitKnownCount, &row.GrossProfitMicroUsd, snapshot.GrossProfitMicroUsd); err != nil {
		return err
	}
	if snapshot.SalesMicroUsd != nil && snapshot.GrossProfitMicroUsd != nil {
		if err := addInt64(&row.GrossMarginEligibleCount, 1); err != nil {
			return err
		}
		if err := addInt64(&row.GrossMarginEligibleSalesMicroUsd, *snapshot.SalesMicroUsd); err != nil {
			return err
		}
	}
	return nil
}

func supplierPricingModeFromProvenance(provenance *types.SupplierPricingProvenanceV1) (string, error) {
	if provenance == nil {
		return "", errors.New("missing supplier pricing provenance")
	}
	mode := ""
	if provenance.Ratio != nil {
		mode = string(types.SupplierPricingModeRatio)
	}
	if provenance.Fixed != nil {
		if mode != "" {
			return "", errors.New("ambiguous supplier pricing provenance")
		}
		mode = string(types.SupplierPricingModeFixed)
	}
	if provenance.Tiered != nil {
		if mode != "" {
			return "", errors.New("ambiguous supplier pricing provenance")
		}
		mode = string(types.SupplierPricingModeTiered)
	}
	if mode == "" {
		return "", errors.New("missing supplier pricing provenance mode")
	}
	return mode, nil
}

func addKnownAmount(count, total *int64, value *int64) error {
	if value == nil {
		return nil
	}
	if err := addInt64(count, 1); err != nil {
		return err
	}
	return addInt64(total, *value)
}

func addInt64(target *int64, value int64) error {
	if (value > 0 && *target > math.MaxInt64-value) || (value < 0 && *target < math.MinInt64-value) {
		return ErrSupplierReportOverflow
	}
	*target += value
	return nil
}

func pointerInt64String(value *int64) string {
	if value == nil {
		return "unknown"
	}
	return strconv.FormatInt(*value, 10)
}

func beginningOfSupplierDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}
