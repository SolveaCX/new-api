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

var ErrSupplierAccountingNotActive = errors.New("supplier accounting is not active")

type supplierAccountingLogEnvelope struct {
	SupplierAccountingV1 json.RawMessage `json:"supplier_accounting_v1"`
}

// InitializeSupplierAccountingCoverageStart is retained for compatibility with
// legacy readers. It is intentionally read-only; adoption is an explicit
// control-plane command.
func InitializeSupplierAccountingCoverageStart(ctx context.Context, db *gorm.DB) (int64, error) {
	return model.SupplierAccountingCoverageStart(ctx, db)
}

// CheckSupplierAccountingReadiness performs fail-closed, strongly consistent
// reads of both strict control documents. SUPPLIER_ACCOUNTING_CUTOVER_AT is an
// assertion only; it never creates or mutates activation or legacy state.
func CheckSupplierAccountingReadiness() error {
	activation, err := model.ReadSupplierAccountingActivationState(model.DB)
	if err != nil {
		return fmt.Errorf("supplier accounting readiness activation state: %w", err)
	}
	mutation, err := model.ReadSupplierAccountingMutationState(model.DB)
	if err != nil {
		return fmt.Errorf("supplier accounting readiness mutation state: %w", err)
	}
	if mutation.Enabled {
		if err := model.ValidateSupplierAdminCommandLedgerFinalized(model.DB); err != nil {
			return fmt.Errorf("supplier accounting readiness command ledger: %w", err)
		}
	}

	configuredCutover, err := configuredSupplierAccountingCoverageStart()
	if err != nil {
		return fmt.Errorf("supplier accounting readiness: %w", err)
	}
	if configuredCutover == 0 {
		return nil
	}
	if activation.CutoverAt == nil {
		return errors.New("supplier accounting readiness: SUPPLIER_ACCOUNTING_CUTOVER_AT requires a persisted activation cutover")
	}
	if *activation.CutoverAt != configuredCutover {
		return fmt.Errorf("supplier accounting readiness: SUPPLIER_ACCOUNTING_CUTOVER_AT mismatch: configured=%d persisted=%d", configuredCutover, *activation.CutoverAt)
	}
	return nil
}

type SupplierDailyBatchCatchUpResult struct {
	ProcessedDays int    `json:"processed_days"`
	RemainingWork bool   `json:"remaining_work"`
	NextBatchDate string `json:"next_batch_date"`
}

// CatchUpSupplierDailyBatches processes at most one missing Shanghai calendar
// day through D-1. Callers repeat while RemainingWork is true. The canonical
// activation state must already be active or degraded.
func CatchUpSupplierDailyBatches(ctx context.Context, mainDB, logDB *gorm.DB, owner string, now time.Time) (SupplierDailyBatchCatchUpResult, error) {
	return catchUpSupplierDailyBatches(ctx, mainDB, logDB, owner, now, RunSupplierDailyBatch)
}

type supplierDailyBatchRunner func(context.Context, *gorm.DB, *gorm.DB, string, string, time.Time, bool) error

func catchUpSupplierDailyBatches(ctx context.Context, mainDB, logDB *gorm.DB, owner string, now time.Time, run supplierDailyBatchRunner) (SupplierDailyBatchCatchUpResult, error) {
	result := SupplierDailyBatchCatchUpResult{}
	if run == nil {
		return result, model.ErrDatabase
	}
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	if err != nil {
		return result, err
	}
	today := beginningOfSupplierDay(now.In(location))
	if now.In(location).Before(today.Add(SupplierDailyCloseGrace)) {
		return result, nil
	}
	target := today.AddDate(0, 0, -1)
	cutoverAt, err := supplierAccountingBatchCutover(ctx, mainDB)
	if err != nil {
		return result, err
	}
	next, err := nextSupplierDailyBatchDate(ctx, mainDB, cutoverAt, location)
	if err != nil {
		return result, err
	}
	if next.After(target) {
		return result, nil
	}
	if err := run(ctx, mainDB, logDB, next.Format("2006-01-02"), owner, now, false); err != nil {
		return result, err
	}
	result.ProcessedDays = SupplierDailyCatchUpMaxDays
	next, err = nextSupplierDailyBatchDate(ctx, mainDB, cutoverAt, location)
	if err != nil {
		return result, err
	}
	if !next.After(target) {
		result.RemainingWork = true
		result.NextBatchDate = next.Format("2006-01-02")
	}
	return result, nil
}

func nextSupplierDailyBatchDate(ctx context.Context, mainDB *gorm.DB, coverageStartAt int64, location *time.Location) (time.Time, error) {
	earliestIncomplete, err := model.EarliestIncompleteSupplierDailyBatch(ctx, mainDB)
	if err != nil {
		return time.Time{}, err
	}
	next := beginningOfSupplierDay(time.Unix(coverageStartAt, 0).In(location))
	if earliestIncomplete != nil {
		return time.ParseInLocation("2006-01-02", earliestIncomplete.BatchDate, location)
	}
	latestCompleted, err := model.LatestCompletedSupplierDailyBatch(ctx, mainDB)
	if err != nil {
		return time.Time{}, err
	}
	if latestCompleted == nil {
		return next, nil
	}
	next, err = time.ParseInLocation("2006-01-02", latestCompleted.BatchDate, location)
	if err != nil {
		return time.Time{}, err
	}
	return next.AddDate(0, 0, 1), nil
}

func RunSupplierDailyBatch(ctx context.Context, mainDB, logDB *gorm.DB, batchDate, owner string, now time.Time, force bool) (runErr error) {
	location, err := time.LoadLocation(SupplierDailyBatchTimezone)
	if err != nil {
		return err
	}
	day, err := time.ParseInLocation("2006-01-02", batchDate, location)
	if err != nil || !day.Before(beginningOfSupplierDay(now.In(location))) {
		return fmt.Errorf("invalid supplier batch date %q", batchDate)
	}
	dayEnd := day.AddDate(0, 0, 1)
	cutoverAt, err := supplierAccountingBatchCutover(ctx, mainDB)
	if err != nil {
		return err
	}
	lease, err := model.AcquireSupplierDailyBatch(ctx, mainDB, batchDate, day.Unix(), dayEnd.Unix(), owner, now, supplierDailyLeaseDuration, force)
	if err != nil || lease.AlreadyDone {
		return err
	}
	defer func() {
		if runErr != nil {
			_ = model.FailSupplierDailyBatch(context.Background(), mainDB, lease, runErr)
		}
	}()

	scanStartAt := day.Unix()
	if cutoverAt > scanStartAt {
		scanStartAt = cutoverAt
	}
	if scanStartAt < dayEnd.Unix() {
		for {
			rows, pageErr := model.ScanSupplierAccountingLogPage(ctx, logDB, scanStartAt, dayEnd.Unix(), lease.CursorCreatedAt, lease.CursorId, model.SupplierDailyLogPageSize)
			if pageErr != nil {
				return pageErr
			}
			if len(rows) == 0 {
				break
			}
			accumulators := make(map[string]*model.SupplierUsageDailySummary, len(rows))
			var snapshotCount int64
			for _, logRow := range rows {
				snapshot, ok, parseErr := parseSupplierAccountingLog(logRow.Other)
				if parseErr != nil {
					return fmt.Errorf("parse supplier accounting log %d: %w", logRow.Id, parseErr)
				}
				if !ok {
					continue
				}
				if err := addSupplierDailySnapshot(accumulators, batchDate, day.Unix(), logRow, snapshot); err != nil {
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
				break
			}
		}
	}
	return model.CompleteSupplierDailyBatch(ctx, mainDB, lease, now)
}

func supplierAccountingBatchCutover(ctx context.Context, mainDB *gorm.DB) (int64, error) {
	if mainDB == nil {
		return 0, model.ErrDatabase
	}
	state, err := model.ReadSupplierAccountingActivationState(mainDB.WithContext(ctx))
	if err != nil {
		return 0, err
	}
	if state.Phase != model.SupplierAccountingActivationActive && state.Phase != model.SupplierAccountingActivationDegraded {
		return 0, fmt.Errorf("%w: phase %q", ErrSupplierAccountingNotActive, state.Phase)
	}
	if state.CutoverAt == nil || *state.CutoverAt <= 0 {
		return 0, fmt.Errorf("%w: canonical cutover is missing", ErrSupplierAccountingNotActive)
	}
	return *state.CutoverAt, nil
}

func configuredSupplierAccountingCoverageStart() (int64, error) {
	raw := strings.TrimSpace(os.Getenv("SUPPLIER_ACCOUNTING_CUTOVER_AT"))
	if raw == "" {
		return 0, nil
	}
	cutoverAt, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || cutoverAt <= 0 {
		return 0, fmt.Errorf("invalid SUPPLIER_ACCOUNTING_CUTOVER_AT %q", raw)
	}
	return cutoverAt, nil
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
	var snapshot types.SupplierAccountingLogSnapshotV1
	if err := common.Unmarshal(envelope.SupplierAccountingV1, &snapshot); err != nil {
		return types.SupplierAccountingLogSnapshotV1{}, false, err
	}
	if snapshot.SupplierId <= 0 || snapshot.ContractId <= 0 || snapshot.RateVersionId <= 0 || snapshot.FinanciallyCommittedAt <= 0 || (snapshot.StatisticsScope != string(types.SupplierStatisticsScopeBusiness) && snapshot.StatisticsScope != string(types.SupplierStatisticsScopeInternal)) {
		return types.SupplierAccountingLogSnapshotV1{}, false, errors.New("invalid supplier accounting snapshot")
	}
	return snapshot, true, nil
}

func addSupplierDailySnapshot(accumulators map[string]*model.SupplierUsageDailySummary, batchDate string, bucketStart int64, logRow model.SupplierAccountingLogRow, snapshot types.SupplierAccountingLogSnapshotV1) error {
	quality := SupplierDataQualityAuthoritative
	if strings.TrimSpace(snapshot.QualityReason) != "" {
		quality = SupplierDataQualityUnattributed
	}
	modelName := logRow.ModelName
	if snapshot.StatisticsScope == string(types.SupplierStatisticsScopeInternal) {
		modelName = ""
	}
	keyText := strings.Join([]string{
		batchDate, strconv.Itoa(snapshot.SupplierId), strconv.Itoa(snapshot.ContractId), strconv.Itoa(snapshot.BindingVersionId),
		strconv.Itoa(snapshot.RateVersionId), strconv.Itoa(logRow.ChannelId), modelName, pointerInt64String(snapshot.SalesMultiplierPpm), pointerString(snapshot.PricingMode), snapshot.StatisticsScope, quality,
	}, "|")
	digest := sha256.Sum256([]byte(keyText))
	key := hex.EncodeToString(digest[:])
	row := accumulators[key]
	if row == nil {
		row = &model.SupplierUsageDailySummary{
			BatchDate: batchDate, DimensionKey: key, BucketStart: bucketStart,
			SupplierId: snapshot.SupplierId, ContractId: snapshot.ContractId, BindingVersionId: snapshot.BindingVersionId,
			RateVersionId: snapshot.RateVersionId, ChannelId: logRow.ChannelId, ModelName: modelName,
			SalesMultiplierPpm: cloneSupplierInt64(snapshot.SalesMultiplierPpm), PricingMode: pointerString(snapshot.PricingMode), StatisticsScope: snapshot.StatisticsScope, DataQuality: quality,
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
	// Internal inventory consumption is retained, but internal traffic is never
	// included in business sales/profit metrics.
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

func pointerString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
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
