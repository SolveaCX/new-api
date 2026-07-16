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
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "component_id"}, {Name: "granularity"}, {Name: "period_start"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"score_sum_micros", "known_bucket_count", "unknown_bucket_count", "maintenance_bucket_count",
			"worst_status", "eligible_count", "success_count", "probe_success_count", "probe_failure_count",
			"latency_sum_ms", "latency_count", "ttft_sum_ms", "ttft_count", "updated_at",
		}),
	}).Create(period).Error
}
