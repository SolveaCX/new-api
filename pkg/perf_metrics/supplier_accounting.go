package perfmetrics

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

type SupplierAccountingBuildFailureReason string

const (
	SupplierAccountingBuildFailureNegative       SupplierAccountingBuildFailureReason = "negative"
	SupplierAccountingBuildFailureNonFinite      SupplierAccountingBuildFailureReason = "non_finite"
	SupplierAccountingBuildFailureInvalidDivisor SupplierAccountingBuildFailureReason = "invalid_divisor"
	SupplierAccountingBuildFailureOverflow       SupplierAccountingBuildFailureReason = "overflow"
	SupplierAccountingBuildFailureUnknown        SupplierAccountingBuildFailureReason = "unknown"
	SupplierAccountingBuildFailureMissing        SupplierAccountingBuildFailureReason = "missing"
	supplierAccountingBacklogObserverTimeout                                          = 2 * time.Second
	supplierAccountingBacklogCacheTTL                                                 = 15 * time.Second
	supplierAccountingBacklogSingleflightKey                                          = "supplier-accounting-backlog"
)

type supplierAccountingBacklogPrometheusState uint8

const (
	supplierAccountingBacklogPrometheusOmitted supplierAccountingBacklogPrometheusState = iota
	supplierAccountingBacklogPrometheusDown
	supplierAccountingBacklogPrometheusUp
)

type supplierAccountingBacklogPrometheusSnapshot struct {
	state       supplierAccountingBacklogPrometheusState
	observation model.SupplierAccountingBacklogObservation
}

func (snapshot supplierAccountingBacklogPrometheusSnapshot) seriesCount() int {
	switch snapshot.state {
	case supplierAccountingBacklogPrometheusDown:
		return 1
	case supplierAccountingBacklogPrometheusUp:
		return 5
	default:
		return 0
	}
}

type supplierAccountingBacklogCacheState struct {
	mu        sync.RWMutex
	source    *gorm.DB
	snapshot  supplierAccountingBacklogPrometheusSnapshot
	expiresAt time.Time
}

func (cache *supplierAccountingBacklogCacheState) load(source *gorm.DB, now time.Time) (supplierAccountingBacklogPrometheusSnapshot, bool) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	if cache.source != source || cache.expiresAt.IsZero() || !now.Before(cache.expiresAt) {
		return supplierAccountingBacklogPrometheusSnapshot{}, false
	}
	return cache.snapshot, true
}

func (cache *supplierAccountingBacklogCacheState) store(source *gorm.DB, snapshot supplierAccountingBacklogPrometheusSnapshot, expiresAt time.Time) {
	cache.mu.Lock()
	cache.source = source
	cache.snapshot = snapshot
	cache.expiresAt = expiresAt
	cache.mu.Unlock()
}

func (cache *supplierAccountingBacklogCacheState) clear(source *gorm.DB) {
	cache.mu.Lock()
	if source == nil || cache.source == source {
		cache.source = nil
		cache.snapshot = supplierAccountingBacklogPrometheusSnapshot{}
		cache.expiresAt = time.Time{}
	}
	cache.mu.Unlock()
}

var (
	supplierAccountingBacklogCache        supplierAccountingBacklogCacheState
	supplierAccountingBacklogSingleflight singleflight.Group
)

var supplierAccountingDispositions = [...]types.SupplierAccountingDisposition{
	types.SupplierAccountingDispositionUnsupportedPath,
	types.SupplierAccountingDispositionNotFinanciallyCommitted,
	types.SupplierAccountingDispositionZeroUsage,
	types.SupplierAccountingDispositionUnbound,
	types.SupplierAccountingDispositionCaptured,
	types.SupplierAccountingDispositionProducerError,
}

var supplierAccountingBuildFailureReasons = [...]SupplierAccountingBuildFailureReason{
	SupplierAccountingBuildFailureNegative,
	SupplierAccountingBuildFailureNonFinite,
	SupplierAccountingBuildFailureInvalidDivisor,
	SupplierAccountingBuildFailureOverflow,
	SupplierAccountingBuildFailureUnknown,
	SupplierAccountingBuildFailureMissing,
}

var supplierAccountingWriteOutcomes = [...]model.SupplierAccountingConsumeLogWriteOutcome{
	model.SupplierAccountingConsumeLogWriteSuccess,
	model.SupplierAccountingConsumeLogWriteFailure,
	model.SupplierAccountingConsumeLogWriteDisabled,
}

type supplierAccountingCounters struct {
	settlements   [len(supplierAccountingDispositions)]atomic.Uint64
	buildFailures [len(supplierAccountingBuildFailureReasons)]atomic.Uint64
	writes        [len(supplierAccountingDispositions)][len(supplierAccountingWriteOutcomes)]atomic.Uint64
}

var supplierAccountingMetricCounters supplierAccountingCounters

func RecordSupplierAccountingSettlement(disposition types.SupplierAccountingDisposition) bool {
	index := supplierAccountingDispositionIndex(disposition)
	if index < 0 {
		return false
	}
	supplierAccountingMetricCounters.settlements[index].Add(1)
	return true
}

func RecordSupplierAccountingSnapshotBuildFailure(reason SupplierAccountingBuildFailureReason) bool {
	index := supplierAccountingBuildFailureReasonIndex(reason)
	if index < 0 {
		return false
	}
	supplierAccountingMetricCounters.buildFailures[index].Add(1)
	return true
}

func RecordSupplierAccountingConsumeLogWrite(disposition types.SupplierAccountingDisposition, outcome model.SupplierAccountingConsumeLogWriteOutcome) bool {
	dispositionIndex := supplierAccountingDispositionIndex(disposition)
	outcomeIndex := supplierAccountingWriteOutcomeIndex(outcome)
	if dispositionIndex < 0 || outcomeIndex < 0 {
		return false
	}
	supplierAccountingMetricCounters.writes[dispositionIndex][outcomeIndex].Add(1)
	return true
}

func supplierAccountingDispositionIndex(disposition types.SupplierAccountingDisposition) int {
	for index, candidate := range supplierAccountingDispositions {
		if candidate == disposition {
			return index
		}
	}
	return -1
}

func supplierAccountingBuildFailureReasonIndex(reason SupplierAccountingBuildFailureReason) int {
	for index, candidate := range supplierAccountingBuildFailureReasons {
		if candidate == reason {
			return index
		}
	}
	return -1
}

func supplierAccountingWriteOutcomeIndex(outcome model.SupplierAccountingConsumeLogWriteOutcome) int {
	for index, candidate := range supplierAccountingWriteOutcomes {
		if candidate == outcome {
			return index
		}
	}
	return -1
}

func supplierAccountingPrometheusSeriesCount() int {
	count := 0
	for index := range supplierAccountingDispositions {
		if supplierAccountingMetricCounters.settlements[index].Load() > 0 {
			count++
		}
		for outcomeIndex := range supplierAccountingWriteOutcomes {
			if supplierAccountingMetricCounters.writes[index][outcomeIndex].Load() > 0 {
				count++
			}
		}
	}
	for index := range supplierAccountingBuildFailureReasons {
		if supplierAccountingMetricCounters.buildFailures[index].Load() > 0 {
			count++
		}
	}
	return count
}

func writeSupplierAccountingPrometheusMetrics(b *strings.Builder) {
	b.WriteString("# HELP newapi_supplier_accounting_settlements_total Supplier accounting settlements by fixed V1 disposition.\n")
	b.WriteString("# TYPE newapi_supplier_accounting_settlements_total counter\n")
	for index, disposition := range supplierAccountingDispositions {
		value := supplierAccountingMetricCounters.settlements[index].Load()
		if value == 0 {
			continue
		}
		b.WriteString(`newapi_supplier_accounting_settlements_total{disposition="`)
		b.WriteString(escapePrometheusLabelValue(string(disposition)))
		b.WriteString(`"} `)
		b.WriteString(strconv.FormatUint(value, 10))
		b.WriteByte('\n')
	}

	b.WriteString("# HELP newapi_supplier_accounting_snapshot_build_failures_total Otherwise-capturable supplier snapshot build failures by bounded reason.\n")
	b.WriteString("# TYPE newapi_supplier_accounting_snapshot_build_failures_total counter\n")
	for index, reason := range supplierAccountingBuildFailureReasons {
		value := supplierAccountingMetricCounters.buildFailures[index].Load()
		if value == 0 {
			continue
		}
		b.WriteString(`newapi_supplier_accounting_snapshot_build_failures_total{reason="`)
		b.WriteString(escapePrometheusLabelValue(string(reason)))
		b.WriteString(`"} `)
		b.WriteString(strconv.FormatUint(value, 10))
		b.WriteByte('\n')
	}

	b.WriteString("# HELP newapi_supplier_accounting_consume_log_writes_total Supplier consume-log write outcomes by fixed V1 disposition.\n")
	b.WriteString("# TYPE newapi_supplier_accounting_consume_log_writes_total counter\n")
	for dispositionIndex, disposition := range supplierAccountingDispositions {
		for outcomeIndex, outcome := range supplierAccountingWriteOutcomes {
			value := supplierAccountingMetricCounters.writes[dispositionIndex][outcomeIndex].Load()
			if value == 0 {
				continue
			}
			b.WriteString(`newapi_supplier_accounting_consume_log_writes_total{disposition="`)
			b.WriteString(escapePrometheusLabelValue(string(disposition)))
			b.WriteString(`",outcome="`)
			b.WriteString(escapePrometheusLabelValue(string(outcome)))
			b.WriteString(`"} `)
			b.WriteString(strconv.FormatUint(value, 10))
			b.WriteByte('\n')
		}
	}
}

func collectSupplierAccountingBacklogPrometheusSnapshot(ctx context.Context) supplierAccountingBacklogPrometheusSnapshot {
	db := model.DB
	if db == nil {
		return supplierAccountingBacklogPrometheusSnapshot{state: supplierAccountingBacklogPrometheusOmitted}
	}
	if cached, ok := supplierAccountingBacklogCache.load(db, time.Now()); ok {
		return cached
	}
	result := supplierAccountingBacklogSingleflight.DoChan(supplierAccountingBacklogSingleflightKey, func() (any, error) {
		if cached, ok := supplierAccountingBacklogCache.load(db, time.Now()); ok {
			return cached, nil
		}
		snapshot, err := refreshSupplierAccountingBacklogPrometheusSnapshot(ctx, db)
		if err != nil {
			supplierAccountingBacklogCache.clear(db)
			return supplierAccountingBacklogPrometheusSnapshot{state: supplierAccountingBacklogPrometheusDown}, err
		}
		supplierAccountingBacklogCache.store(db, snapshot, time.Now().Add(supplierAccountingBacklogCacheTTL))
		return snapshot, nil
	})
	select {
	case <-ctx.Done():
		return supplierAccountingBacklogPrometheusSnapshot{state: supplierAccountingBacklogPrometheusDown}
	case completed := <-result:
		if completed.Err != nil {
			return supplierAccountingBacklogPrometheusSnapshot{state: supplierAccountingBacklogPrometheusDown}
		}
		snapshot, ok := completed.Val.(supplierAccountingBacklogPrometheusSnapshot)
		if !ok {
			return supplierAccountingBacklogPrometheusSnapshot{state: supplierAccountingBacklogPrometheusDown}
		}
		return snapshot
	}
}

func refreshSupplierAccountingBacklogPrometheusSnapshot(ctx context.Context, db *gorm.DB) (supplierAccountingBacklogPrometheusSnapshot, error) {
	queryCtx, cancel := context.WithTimeout(ctx, supplierAccountingBacklogObserverTimeout)
	defer cancel()
	activation, err := model.ReadSupplierAccountingActivationState(db.WithContext(queryCtx))
	if err != nil {
		return supplierAccountingBacklogPrometheusSnapshot{}, err
	}
	if activation.Phase != model.SupplierAccountingActivationActive && activation.Phase != model.SupplierAccountingActivationDegraded {
		return supplierAccountingBacklogPrometheusSnapshot{state: supplierAccountingBacklogPrometheusOmitted}, nil
	}
	if activation.CutoverAt == nil || *activation.CutoverAt <= 0 {
		return supplierAccountingBacklogPrometheusSnapshot{}, model.ErrDatabase
	}
	observation, err := model.ObserveSupplierAccountingBacklog(queryCtx, db, *activation.CutoverAt)
	if err != nil {
		return supplierAccountingBacklogPrometheusSnapshot{}, err
	}
	return supplierAccountingBacklogPrometheusSnapshot{state: supplierAccountingBacklogPrometheusUp, observation: observation}, nil
}

func writeSupplierAccountingBacklogPrometheusMetrics(b *strings.Builder, snapshot supplierAccountingBacklogPrometheusSnapshot) {
	if snapshot.state == supplierAccountingBacklogPrometheusOmitted {
		return
	}
	b.WriteString("# HELP newapi_supplier_accounting_backlog_observer_up Whether the supplier accounting backlog observer completed successfully.\n")
	b.WriteString("# TYPE newapi_supplier_accounting_backlog_observer_up gauge\n")
	if snapshot.state != supplierAccountingBacklogPrometheusUp {
		b.WriteString("newapi_supplier_accounting_backlog_observer_up 0\n")
		return
	}
	b.WriteString("newapi_supplier_accounting_backlog_observer_up 1\n")
	b.WriteString("# HELP newapi_supplier_accounting_never_published_days Supplier accounting days in the bounded scheduler range with no published fence.\n")
	b.WriteString("# TYPE newapi_supplier_accounting_never_published_days gauge\n")
	b.WriteString("newapi_supplier_accounting_never_published_days ")
	b.WriteString(strconv.FormatInt(snapshot.observation.NeverPublishedDays, 10))
	b.WriteByte('\n')
	b.WriteString("# HELP newapi_supplier_accounting_oldest_never_published_age_seconds Age of the oldest never-published supplier accounting day.\n")
	b.WriteString("# TYPE newapi_supplier_accounting_oldest_never_published_age_seconds gauge\n")
	b.WriteString("newapi_supplier_accounting_oldest_never_published_age_seconds ")
	b.WriteString(strconv.FormatInt(snapshot.observation.OldestNeverPublishedAgeSeconds, 10))
	b.WriteByte('\n')
	b.WriteString("# HELP newapi_supplier_accounting_prior_day_unpublished_after_0800 Whether the prior Shanghai accounting day remains unpublished at or after 08:00.\n")
	b.WriteString("# TYPE newapi_supplier_accounting_prior_day_unpublished_after_0800 gauge\n")
	b.WriteString("newapi_supplier_accounting_prior_day_unpublished_after_0800 ")
	if snapshot.observation.PriorDayUnpublishedAfter0800 {
		b.WriteString("1\n")
	} else {
		b.WriteString("0\n")
	}
	b.WriteString("# HELP newapi_supplier_accounting_backlog_observed_at_seconds Database unix time of the supplier accounting backlog observation.\n")
	b.WriteString("# TYPE newapi_supplier_accounting_backlog_observed_at_seconds gauge\n")
	b.WriteString("newapi_supplier_accounting_backlog_observed_at_seconds ")
	b.WriteString(strconv.FormatInt(snapshot.observation.ObservedAtUnix, 10))
	b.WriteByte('\n')
}
