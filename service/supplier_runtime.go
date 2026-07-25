package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

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
	if _, configured, err := configuredSupplierAccountingCutover(); err != nil {
		return result, err
	} else if !configured {
		return result, nil
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
	if err := RunSupplierDailyBatch(ctx, mainDB, logDB, next.Format("2006-01-02"), owner, now); err != nil {
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
	return target.AddDate(0, 0, 1), nil
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
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	if err != nil {
		return 0, false, err
	}
	local := time.Unix(value, 0).In(location)
	if !local.Equal(beginningOfSupplierDay(local)) {
		return 0, false, fmt.Errorf("invalid SUPPLIER_ACCOUNTING_CUTOVER_AT %q: must be Asia/Shanghai 00:00:00", raw)
	}
	return value, true, nil
}

func RunSupplierDailyBatch(ctx context.Context, mainDB, logDB *gorm.DB, batchDate, owner string, now time.Time) error {
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	if err != nil {
		return err
	}
	day, err := time.ParseInLocation("2006-01-02", batchDate, location)
	if err != nil || !day.Before(beginningOfSupplierDay(now.In(location))) {
		return fmt.Errorf("invalid supplier batch date %q", batchDate)
	}
	dayEnd := day.AddDate(0, 0, 1)
	lease, err := model.AcquireSupplierDailyBatch(ctx, mainDB, batchDate, day.Unix(), dayEnd.Unix(), owner, supplierDailyLeaseDuration)
	if err != nil || lease.AlreadyDone {
		return err
	}
	watermark, err := model.FreezeSupplierAccountingFactDay(ctx, logDB, batchDate)
	if err != nil {
		_ = model.FailSupplierDailyBatch(context.Background(), mainDB, lease, err)
		return err
	}
	if err := model.AttachSupplierDailyBatchSource(ctx, mainDB, &lease, watermark.SourceMaxFactId,
		string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1)); err != nil {
		_ = model.FailSupplierDailyBatch(context.Background(), mainDB, lease, err)
		return err
	}
	if err := scanSupplierDailyBatch(ctx, mainDB, logDB, lease, day); err != nil {
		_ = model.FailSupplierDailyBatch(context.Background(), mainDB, lease, err)
		return err
	}
	if err := model.VerifySupplierAccountingFactDayClosed(ctx, logDB, batchDate, lease.SourceMaxFactId); err != nil {
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
	if lease.SourceMaxFactId == 0 {
		return nil
	}
	for {
		rows, err := model.ScanCapturedSupplierAccountingFactPage(ctx, logDB, lease.BatchDate, lease.SourceMaxFactId, lease.CursorId, model.SupplierAccountingFactPageSize)
		if err != nil {
			return fmt.Errorf("scan supplier accounting facts: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}
		accumulators := make(map[string]*model.SupplierUsageDailySummary, len(rows))
		var snapshotCount int64
		for _, logRow := range rows {
			digest := sha256.Sum256([]byte(logRow.Payload))
			if logRow.PayloadHash != hex.EncodeToString(digest[:]) {
				return fmt.Errorf("supplier accounting fact %d payload hash mismatch", logRow.Id)
			}
			snapshot, err := parseSupplierAccountingFactPayload(logRow.Payload)
			if err != nil {
				return fmt.Errorf("parse supplier accounting fact %d: %w", logRow.Id, err)
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
		if err := model.PersistSupplierDailyBatchPage(ctx, mainDB, lease, summaries, last.Id, int64(len(rows)), snapshotCount, supplierDailyLeaseDuration); err != nil {
			return err
		}
		lease.CursorId = last.Id
		if last.Id == lease.SourceMaxFactId || len(rows) < model.SupplierAccountingFactPageSize {
			return nil
		}
	}
}

func parseSupplierAccountingFactPayload(payload string) (types.SupplierAccountingLogSnapshotV1, error) {
	if strings.TrimSpace(payload) == "" {
		return types.SupplierAccountingLogSnapshotV1{}, errors.New("empty captured supplier accounting fact payload")
	}
	parsed, err := types.ParseSupplierAccountingEnvelopeV1JSON([]byte(payload))
	if err != nil {
		return types.SupplierAccountingLogSnapshotV1{}, err
	}
	if err := ValidateSupplierAccountingEnvelopeV1(parsed); err != nil {
		return types.SupplierAccountingLogSnapshotV1{}, err
	}
	if parsed.Disposition != types.SupplierAccountingDispositionCaptured || parsed.Captured == nil {
		return types.SupplierAccountingLogSnapshotV1{}, errors.New("non-captured supplier accounting fact payload")
	}
	return *parsed.Captured, nil
}

func addSupplierDailySnapshot(accumulators map[string]*model.SupplierUsageDailySummary, batchDate string, bucketStart int64, logRow model.SupplierAccountingFactRow, snapshot types.SupplierAccountingLogSnapshotV1) error {
	quality := SupplierDataQualityAuthoritative
	if strings.TrimSpace(snapshot.QualityReason) != "" {
		quality = SupplierDataQualityUnattributed
	}
	modelName := logRow.ModelName
	bindingVersionID := snapshot.BindingVersionId
	rateVersionID := snapshot.RateVersionId
	channelID := logRow.ChannelId
	salesMultiplier := snapshot.SalesMultiplierPpm
	pricingMode := ""
	if snapshot.StatisticsScope == string(types.SupplierStatisticsScopeInternal) {
		bindingVersionID = 0
		rateVersionID = 0
		channelID = 0
		modelName = ""
		salesMultiplier = nil
	} else {
		var err error
		pricingMode, err = supplierPricingModeFromProvenance(snapshot.PricingProvenance)
		if err != nil {
			return err
		}
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
