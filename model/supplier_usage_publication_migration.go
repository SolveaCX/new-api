package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const supplierDailyPublicationMigrationMaxAttempts = 8

var errSupplierDailyPublicationMigrationConflict = errors.New("supplier daily publication migration concurrent conflict")

type supplierDailyPublicationMigrationPlan struct {
	run     SupplierUsageDailyBatchRun
	updates map[string]any
}

// MigrateSupplierUsageDailyBatchPublication adopts the pre-WP4 batch table
// without treating an existing committed publication as backlog. Each attempt
// validates a stable, all-row plan before making any repair and is rerunnable
// across concurrent startup, acquisition, and publication.
func MigrateSupplierUsageDailyBatchPublication(db *gorm.DB) error {
	if db == nil {
		return ErrDatabase
	}
	if !db.Migrator().HasTable(&SupplierUsageDailyBatchRun{}) {
		return nil
	}
	var lastConflict error
	for attempt := 0; attempt < supplierDailyPublicationMigrationMaxAttempts; attempt++ {
		err := migrateSupplierUsageDailyBatchPublicationAttempt(db)
		if err == nil {
			return nil
		}
		if !isSupplierDailyPublicationMigrationConflict(err) {
			return err
		}
		lastConflict = err
		if attempt+1 < supplierDailyPublicationMigrationMaxAttempts {
			time.Sleep(time.Duration(1<<attempt) * time.Millisecond)
		}
	}
	return fmt.Errorf("supplier daily publication migration did not converge: %w", lastConflict)
}

func migrateSupplierUsageDailyBatchPublicationAttempt(db *gorm.DB) error {
	ctx := context.Background()
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		nowUnix, err := supplierDBUnix(ctx, tx)
		if err != nil {
			return err
		}
		var runs []SupplierUsageDailyBatchRun
		query := tx.Order("id ASC")
		if tx.Dialector.Name() != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err = query.Find(&runs).Error; err != nil {
			return err
		}

		plans := make([]supplierDailyPublicationMigrationPlan, 0, len(runs))
		activeCount := 0
		for index := range runs {
			plan, active, planErr := planSupplierDailyPublicationMigration(tx, runs[index], nowUnix)
			if planErr != nil {
				return fmt.Errorf("validate supplier daily publication %q: %w", runs[index].BatchDate, planErr)
			}
			if active {
				activeCount++
				if activeCount > 1 {
					return fmt.Errorf("multiple active supplier daily leases: %w", ErrSupplierDailyBatchPublicationInvalid)
				}
			}
			if len(plan.updates) > 0 {
				plans = append(plans, plan)
			}
		}

		for index := range plans {
			if err = applySupplierDailyPublicationMigrationPlan(tx, plans[index]); err != nil {
				return err
			}
		}
		return nil
	})
}

func planSupplierDailyPublicationMigration(tx *gorm.DB, run SupplierUsageDailyBatchRun, nowUnix int64) (supplierDailyPublicationMigrationPlan, bool, error) {
	plan := supplierDailyPublicationMigrationPlan{run: run, updates: make(map[string]any)}
	if err := validateSupplierDailyBatchRunIdentity(run); err != nil {
		return plan, false, err
	}

	publishedMetadataCount := 0
	if run.PublishedFenceToken > 0 {
		publishedMetadataCount++
	}
	if run.PublishedAt != nil {
		publishedMetadataCount++
	}
	if run.PublishedEvidenceV1 != "" {
		publishedMetadataCount++
	}
	if run.PublishedPersistedLogSnapshotCompleteness != "" {
		publishedMetadataCount++
	}
	switch {
	case publishedMetadataCount == 4:
		if err := validateExistingSupplierDailyPublication(tx, run); err != nil {
			return plan, false, err
		}
	case publishedMetadataCount != 0:
		return plan, false, ErrSupplierDailyBatchPublicationInvalid
	case run.Status == SupplierDailyBatchStatusCompleted:
		updates, err := planLegacySupplierDailyPublication(tx, run)
		if err != nil {
			return plan, false, err
		}
		for key, value := range updates {
			plan.updates[key] = value
		}
	}

	active := run.Status == SupplierDailyBatchStatusRunning && run.LockedUntil >= nowUnix
	if active {
		if run.ActiveLeaseSlot == nil || *run.ActiveLeaseSlot != 1 {
			slot := 1
			plan.updates["active_lease_slot"] = &slot
		}
	} else if run.ActiveLeaseSlot != nil {
		plan.updates["active_lease_slot"] = nil
	}
	return plan, active, nil
}

func validateSupplierDailyBatchRunIdentity(run SupplierUsageDailyBatchRun) error {
	if run.Id <= 0 || run.FenceToken < 0 || run.PublishedFenceToken < 0 || run.PublishedFenceToken > run.FenceToken {
		return ErrSupplierDailyBatchPublicationInvalid
	}
	start, _, err := supplierDailyBatchDateRange(run.BatchDate, run.BatchDate)
	if err != nil || run.DayStart != start.Unix() || run.DayEnd != start.AddDate(0, 0, 1).Unix() {
		return ErrSupplierDailyBatchPublicationInvalid
	}
	return nil
}

func validateExistingSupplierDailyPublication(tx *gorm.DB, run SupplierUsageDailyBatchRun) error {
	if run.PublishedFenceToken <= 0 || run.PublishedFenceToken > run.FenceToken || run.PublishedAt == nil || *run.PublishedAt <= 0 {
		return ErrSupplierDailyBatchPublicationInvalid
	}
	evidence, err := types.ParseSupplierPublishedEvidenceV1(run.PublishedEvidenceV1)
	if err != nil || evidence.PersistedLogSnapshotCompleteness != run.PublishedPersistedLogSnapshotCompleteness ||
		evidence.PersistedLogSnapshotCompleteness == types.SupplierPersistedLogCompletenessNotScanned {
		return ErrSupplierDailyBatchPublicationInvalid
	}
	_, err = validateSupplierPublishedSummaryGeneration(tx, run.BatchDate, run.PublishedFenceToken, evidence.CapturedSnapshotCount)
	return err
}

func planLegacySupplierDailyPublication(tx *gorm.DB, run SupplierUsageDailyBatchRun) (map[string]any, error) {
	publishedFence := run.PublishedFenceToken
	if publishedFence == 0 {
		publishedFence = run.FenceToken
	}
	if publishedFence <= 0 || publishedFence > run.FenceToken || run.CompletedAt == nil || *run.CompletedAt <= 0 ||
		run.LogsScanned < 0 || run.SnapshotCount < 0 || run.LogsScanned != run.SnapshotCount {
		return nil, ErrSupplierDailyBatchPublicationInvalid
	}
	summaryCount, err := validateSupplierPublishedSummaryGeneration(tx, run.BatchDate, publishedFence, run.SnapshotCount)
	if err != nil {
		return nil, err
	}
	if run.SummaryCount != summaryCount {
		return nil, ErrSupplierDailyBatchPublicationInvalid
	}
	evidence, err := legacySupplierPublishedEvidence(run.LogsScanned, run.SnapshotCount)
	if err != nil {
		return nil, err
	}
	evidenceRaw, err := types.EncodeSupplierPublishedEvidenceV1(evidence)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"published_fence_token": publishedFence,
		"published_at":          *run.CompletedAt,
		"published_persisted_log_snapshot_completeness": evidence.PersistedLogSnapshotCompleteness,
		"published_evidence_v1":                         evidenceRaw,
	}, nil
}

func applySupplierDailyPublicationMigrationPlan(tx *gorm.DB, plan supplierDailyPublicationMigrationPlan) error {
	run := plan.run
	query := tx.Model(&SupplierUsageDailyBatchRun{}).
		Where("id = ? AND batch_date = ? AND day_start = ? AND day_end = ?", run.Id, run.BatchDate, run.DayStart, run.DayEnd).
		Where("status = ? AND fence_token = ? AND lease_owner = ? AND locked_until = ?", run.Status, run.FenceToken, run.LeaseOwner, run.LockedUntil)
	if run.ActiveLeaseSlot == nil {
		query = query.Where("active_lease_slot IS NULL")
	} else {
		query = query.Where("active_lease_slot = ?", *run.ActiveLeaseSlot)
	}
	if _, writesPublication := plan.updates["published_fence_token"]; writesPublication {
		query = query.
			Where("published_fence_token = ?", run.PublishedFenceToken).
			Where("published_at IS NULL").
			Where("published_persisted_log_snapshot_completeness = ?", run.PublishedPersistedLogSnapshotCompleteness).
			Where("published_evidence_v1 = ?", run.PublishedEvidenceV1).
			Where("completed_at = ? AND logs_scanned = ? AND snapshot_count = ? AND summary_count = ?", *run.CompletedAt, run.LogsScanned, run.SnapshotCount, run.SummaryCount)
	}
	result := query.UpdateColumns(plan.updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errSupplierDailyPublicationMigrationConflict
	}
	return nil
}

func isSupplierDailyPublicationMigrationConflict(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errSupplierDailyPublicationMigrationConflict) {
		return true
	}
	message := strings.ToLower(err.Error())
	for _, fragment := range []string{
		"unique constraint", "duplicate key", "duplicate entry", "serialization failure", "could not serialize",
		"deadlock", "database is locked", "database table is locked", "database busy", "lock wait timeout",
	} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}
