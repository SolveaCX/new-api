package model

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrStatusVersionConflict = errors.New("status object version conflict")

func AcquireStatusJobLease(name string, holder string, now int64, leaseSeconds int64) (StatusJobLease, bool, error) {
	if DB == nil {
		return StatusJobLease{}, false, errors.New("database is not initialized")
	}
	if name == "" || holder == "" || leaseSeconds <= 0 {
		return StatusJobLease{}, false, errors.New("invalid status job lease request")
	}

	lease := StatusJobLease{
		Name:         name,
		Holder:       holder,
		ExpiresAt:    now + leaseSeconds,
		FencingToken: 1,
		UpdatedAt:    now,
	}
	created := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&lease)
	if created.Error != nil {
		return StatusJobLease{}, false, created.Error
	}
	if created.RowsAffected == 1 {
		return lease, true, nil
	}

	result := DB.Model(&StatusJobLease{}).
		Where("name = ? AND expires_at <= ?", name, now).
		Updates(map[string]any{
			"holder":        holder,
			"expires_at":    now + leaseSeconds,
			"fencing_token": gorm.Expr("fencing_token + 1"),
			"updated_at":    now,
		})
	if result.Error != nil {
		return StatusJobLease{}, false, result.Error
	}
	var current StatusJobLease
	if err := DB.Where("name = ?", name).First(&current).Error; err != nil {
		return StatusJobLease{}, false, err
	}
	return current, result.RowsAffected == 1, nil
}

func RenewStatusJobLease(name string, holder string, fencingToken int64, now int64, leaseSeconds int64) (bool, error) {
	if DB == nil {
		return false, errors.New("database is not initialized")
	}
	if name == "" || holder == "" || fencingToken <= 0 || leaseSeconds <= 0 {
		return false, errors.New("invalid status job lease renewal")
	}
	result := DB.Model(&StatusJobLease{}).
		Where("name = ? AND holder = ? AND fencing_token = ? AND expires_at > ?", name, holder, fencingToken, now).
		Updates(map[string]any{
			"expires_at": now + leaseSeconds,
			"updated_at": now,
		})
	return result.RowsAffected == 1, result.Error
}

func CommitStatusComponentWithFence(jobName string, holder string, fencingToken int64, now int64, component *StatusComponent) error {
	if component == nil {
		return errors.New("status component is nil")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var lease StatusJobLease
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("name = ?", jobName).First(&lease).Error; err != nil {
			return err
		}
		if lease.Holder != holder || lease.FencingToken != fencingToken || lease.ExpiresAt <= now {
			return fmt.Errorf("status job lease is no longer owned")
		}
		return tx.Save(component).Error
	})
}

func CommitStatusEvaluationWithFence(jobName string, holder string, fencingToken int64, now int64, component *StatusComponent) error {
	if component == nil || component.ID == 0 {
		return errors.New("status component is nil")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := validateStatusJobFence(tx, jobName, holder, fencingToken, now); err != nil {
			return err
		}
		return tx.Model(&StatusComponent{}).Where("id = ?", component.ID).Updates(map[string]any{
			"observed_status":              component.ObservedStatus,
			"effective_status":             component.EffectiveStatus,
			"status_source":                component.StatusSource,
			"last_evidence_at":             component.LastEvidenceAt,
			"last_trustworthy_update_at":   component.LastTrustworthyUpdateAt,
			"last_evaluated_at":            component.LastEvaluatedAt,
			"coverage_micros":              component.CoverageMicros,
			"consecutive_probe_failures":   component.ConsecutiveProbeFailures,
			"consecutive_probe_successes":  component.ConsecutiveProbeSuccesses,
			"consecutive_traffic_recovery": component.ConsecutiveTrafficRecovery,
			"last_traffic_bucket_start":    component.LastTrafficBucketStart,
			"updated_at":                   component.UpdatedAt,
		}).Error
	})
}

func SyncStatusCatalogWithFence(jobName string, holder string, fencingToken int64, now int64, desired []StatusComponent) error {
	if DB == nil {
		return errors.New("database is not initialized")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var lease StatusJobLease
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("name = ?", jobName).First(&lease).Error; err != nil {
			return err
		}
		if lease.Holder != holder || lease.FencingToken != fencingToken || lease.ExpiresAt <= now {
			return fmt.Errorf("status job lease is no longer owned")
		}

		activeModelKeys := make([]string, 0, len(desired))
		for i := range desired {
			next := desired[i]
			if next.ComponentKey == "" || next.Slug == "" || next.Kind == "" {
				return errors.New("invalid status catalog component")
			}
			if next.Kind == StatusComponentKindModel {
				activeModelKeys = append(activeModelKeys, next.ComponentKey)
			}

			var existing StatusComponent
			err := tx.Where("component_key = ?", next.ComponentKey).First(&existing).Error
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound):
				if err := tx.Create(&next).Error; err != nil {
					return err
				}
			case err != nil:
				return err
			default:
				updates := map[string]any{
					"display_name": next.DisplayName,
					"model_name":   next.ModelName,
					"capability":   next.Capability,
					"lifecycle":    StatusLifecycleActive,
					"updated_at":   now,
				}
				if err := tx.Model(&StatusComponent{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
					return err
				}
			}
		}

		retired := tx.Model(&StatusComponent{}).Where("kind = ?", StatusComponentKindModel)
		if len(activeModelKeys) > 0 {
			retired = retired.Where("component_key NOT IN ?", activeModelKeys)
		}
		return retired.Updates(map[string]any{
			"lifecycle":  StatusLifecycleRetired,
			"updated_at": now,
		}).Error
	})
}

func UpdateStatusComponentVersion(id int64, expectedVersion int64, updates map[string]any) (StatusComponent, error) {
	if len(updates) == 0 {
		return StatusComponent{}, errors.New("status component update is empty")
	}
	clean := make(map[string]any, len(updates)+1)
	for key, value := range updates {
		if key == "id" || key == "version" || key == "component_key" {
			continue
		}
		clean[key] = value
	}
	clean["version"] = gorm.Expr("version + 1")
	result := DB.Model(&StatusComponent{}).Where("id = ? AND version = ?", id, expectedVersion).Updates(clean)
	if result.Error != nil {
		return StatusComponent{}, result.Error
	}
	if result.RowsAffected == 0 {
		return StatusComponent{}, ErrStatusVersionConflict
	}
	var component StatusComponent
	if err := DB.First(&component, id).Error; err != nil {
		return StatusComponent{}, err
	}
	return component, nil
}

func UpsertStatusPeriod(period *StatusPeriod) error {
	if period == nil || period.ComponentID == 0 || period.Granularity == "" {
		return errors.New("invalid status period")
	}
	return upsertStatusPeriod(DB, period)
}

func UpsertStatusPeriodWithFence(jobName string, holder string, fencingToken int64, now int64, period *StatusPeriod) error {
	if period == nil || period.ComponentID == 0 || period.Granularity == "" {
		return errors.New("invalid status period")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := validateStatusJobFence(tx, jobName, holder, fencingToken, now); err != nil {
			return err
		}
		return upsertStatusPeriod(tx, period)
	})
}

func CreateStatusProbeResultWithFence(jobName string, holder string, fencingToken int64, now int64, result *StatusProbeResult) error {
	if result == nil || result.ComponentID == 0 {
		return errors.New("invalid status probe result")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := validateStatusJobFence(tx, jobName, holder, fencingToken, now); err != nil {
			return err
		}
		return tx.Create(result).Error
	})
}

func ValidateStatusJobFence(jobName string, holder string, fencingToken int64, now int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return validateStatusJobFence(tx, jobName, holder, fencingToken, now)
	})
}

func GetStatusComponents() ([]StatusComponent, error) {
	var components []StatusComponent
	err := DB.Order("kind DESC, model_name ASC").Find(&components).Error
	return components, err
}

func GetLatestStatusProbeResults(componentIDs []int64, since int64) (map[int64]StatusProbeResult, error) {
	results := make(map[int64]StatusProbeResult, len(componentIDs))
	if len(componentIDs) == 0 {
		return results, nil
	}
	var probes []StatusProbeResult
	if err := DB.Where("component_id IN ? AND created_at >= ?", componentIDs, since).Order("created_at DESC, id DESC").Find(&probes).Error; err != nil {
		return nil, err
	}
	for _, probe := range probes {
		if _, ok := results[probe.ComponentID]; !ok {
			results[probe.ComponentID] = probe
		}
	}
	return results, nil
}

func GetStatusPeriodsInRange(granularity string, start int64, end int64) ([]StatusPeriod, error) {
	var periods []StatusPeriod
	err := DB.Where("granularity = ? AND period_start >= ? AND period_start < ?", granularity, start, end).
		Order("component_id ASC, period_start ASC").Find(&periods).Error
	return periods, err
}

func DeleteStatusHistoryWithFence(jobName string, holder string, fencingToken int64, now int64, rawCutoff int64, aggregateCutoff int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := validateStatusJobFence(tx, jobName, holder, fencingToken, now); err != nil {
			return err
		}
		if err := tx.Where("created_at < ?", rawCutoff).Delete(&StatusProbeResult{}).Error; err != nil {
			return err
		}
		if err := tx.Where("granularity = ? AND period_start < ?", StatusGranularityFiveMinutes, rawCutoff).Delete(&StatusPeriod{}).Error; err != nil {
			return err
		}
		return tx.Where("granularity IN ? AND period_start < ?", []string{StatusGranularityHour, StatusGranularityDay}, aggregateCutoff).
			Delete(&StatusPeriod{}).Error
	})
}

func validateStatusJobFence(tx *gorm.DB, jobName string, holder string, fencingToken int64, now int64) error {
	var lease StatusJobLease
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("name = ?", jobName).First(&lease).Error; err != nil {
		return err
	}
	if lease.Holder != holder || lease.FencingToken != fencingToken || lease.ExpiresAt <= now {
		return fmt.Errorf("status job lease is no longer owned")
	}
	return nil
}

func upsertStatusPeriod(db *gorm.DB, period *StatusPeriod) error {
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "component_id"}, {Name: "granularity"}, {Name: "period_start"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"score_sum_micros", "known_bucket_count", "unknown_bucket_count", "maintenance_bucket_count",
			"worst_status", "eligible_count", "success_count", "probe_success_count", "probe_failure_count",
			"latency_sum_ms", "latency_count", "ttft_sum_ms", "ttft_count", "updated_at",
		}),
	}).Create(period).Error
}
