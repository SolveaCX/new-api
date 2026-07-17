package model

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PerfMetricAvailability stores the fixed five-minute availability signal used
// by the status scheduler independently of configurable performance buckets.
type PerfMetricAvailability struct {
	ID            int    `json:"id" gorm:"primaryKey"`
	ModelName     string `json:"model_name" gorm:"size:128;uniqueIndex:idx_perf_availability_bucket,priority:1"`
	Group         string `json:"group" gorm:"column:group;size:64;uniqueIndex:idx_perf_availability_bucket,priority:2"`
	BucketTs      int64  `json:"bucket_ts" gorm:"uniqueIndex:idx_perf_availability_bucket,priority:3;index:idx_perf_availability_bucket_ts"`
	EligibleCount int64  `json:"eligible_count" gorm:"default:0"`
	SuccessCount  int64  `json:"success_count" gorm:"default:0"`
}

func (PerfMetricAvailability) TableName() string {
	return "perf_metric_availability"
}

func UpsertPerfMetricAvailability(metric *PerfMetricAvailability) error {
	if metric == nil || metric.EligibleCount <= 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "model_name"},
			{Name: "group"},
			{Name: "bucket_ts"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"eligible_count": gorm.Expr("perf_metric_availability.eligible_count + ?", metric.EligibleCount),
			"success_count":  gorm.Expr("perf_metric_availability.success_count + ?", metric.SuccessCount),
		}),
	}).Create(metric).Error
}

func GetPerfMetricAvailabilitySummaryAll(startTs int64, endTs int64, groups []string) ([]PerfMetricSummary, error) {
	ensurePerfMetricColumnsInitialized()

	var summaries []PerfMetricSummary
	query := DB.Model(&PerfMetricAvailability{}).
		Select("model_name, SUM(eligible_count) as availability_eligible_count, SUM(success_count) as availability_success_count").
		Where("bucket_ts >= ? AND bucket_ts <= ?", startTs, endTs)
	if groups != nil {
		if len(groups) == 0 {
			return summaries, nil
		}
		query = query.Where(commonGroupCol+" IN ?", groups)
	}
	err := query.
		Group("model_name").
		Having("SUM(eligible_count) > 0").
		Find(&summaries).Error
	return summaries, err
}

func DeletePerfMetricAvailabilityBefore(cutoffTs int64) error {
	if cutoffTs <= 0 {
		return nil
	}
	return DB.Where("bucket_ts < ?", cutoffTs).Delete(&PerfMetricAvailability{}).Error
}
