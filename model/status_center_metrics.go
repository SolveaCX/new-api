package model

import (
	"context"
	"errors"
)

type StatusProbeMetricAggregate struct {
	Success         bool
	MonitoringFault bool
	Count           int64
	TotalLatencyMs  int64
}

type StatusRollupMetricRow struct {
	ComponentID int64
	Granularity string
	Latest      int64
}

type StatusOutboxMetricRow struct {
	Status          string
	Count           int64
	Retried         int64
	OldestCreatedAt int64
}

type StatusCenterMetricRows struct {
	Lease                 *StatusJobLease
	Components            []StatusComponent
	LatestProbes          map[int64]StatusProbeResult
	Traffic               []PerfMetricSummary
	ProbeAggregates       []StatusProbeMetricAggregate
	Rollups               []StatusRollupMetricRow
	IncidentDrafts        int64
	ActiveOutbox          []StatusOutboxMetricRow
	DeadOutbox            int64
	SuspendedDestinations int64
}

func GetStatusCenterMetricRows(ctx context.Context, probeWindowStart int64, probeWindowEnd int64, latestProbeSince int64, trafficStart int64, trafficEnd int64, rollupCutoff int64, groups []string) (StatusCenterMetricRows, error) {
	if DB == nil {
		return StatusCenterMetricRows{}, errors.New("database is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	db := DB.WithContext(ctx)
	rows := StatusCenterMetricRows{
		Components:      make([]StatusComponent, 0),
		LatestProbes:    make(map[int64]StatusProbeResult),
		Traffic:         make([]PerfMetricSummary, 0),
		ProbeAggregates: make([]StatusProbeMetricAggregate, 0),
		Rollups:         make([]StatusRollupMetricRow, 0),
		ActiveOutbox:    make([]StatusOutboxMetricRow, 0),
	}

	var lease StatusJobLease
	leaseResult := db.Where("name = ?", "status-center-scheduler").Limit(1).Find(&lease)
	if leaseResult.Error != nil {
		return StatusCenterMetricRows{}, leaseResult.Error
	}
	if leaseResult.RowsAffected > 0 {
		rows.Lease = &lease
	}

	if err := db.Select("id, kind, lifecycle, model_name, effective_status, last_evaluated_at, coverage_micros").
		Where("lifecycle = ?", StatusLifecycleActive).
		Find(&rows.Components).Error; err != nil {
		return StatusCenterMetricRows{}, err
	}
	componentIDs := make([]int64, 0, len(rows.Components))
	for _, component := range rows.Components {
		componentIDs = append(componentIDs, component.ID)
	}

	if len(componentIDs) > 0 {
		var probes []StatusProbeResult
		if err := db.Where("component_id IN ? AND created_at >= ?", componentIDs, latestProbeSince).
			Order("created_at DESC, id DESC").
			Find(&probes).Error; err != nil {
			return StatusCenterMetricRows{}, err
		}
		for _, probe := range probes {
			if _, ok := rows.LatestProbes[probe.ComponentID]; !ok {
				rows.LatestProbes[probe.ComponentID] = probe
			}
		}
	}

	ensurePerfMetricColumnsInitialized()
	trafficQuery := db.Model(&PerfMetricAvailability{}).
		Select("model_name, SUM(eligible_count) as availability_eligible_count, SUM(success_count) as availability_success_count").
		Where("bucket_ts >= ? AND bucket_ts <= ?", trafficStart, trafficEnd)
	if groups != nil {
		if len(groups) > 0 {
			trafficQuery = trafficQuery.Where(commonGroupCol+" IN ?", groups)
		}
	}
	if groups == nil || len(groups) > 0 {
		if err := trafficQuery.Group("model_name").Having("SUM(eligible_count) > 0").Find(&rows.Traffic).Error; err != nil {
			return StatusCenterMetricRows{}, err
		}
	}

	if err := db.Model(&StatusProbeResult{}).
		Select("success, monitoring_fault, COUNT(*) AS count, COALESCE(SUM(latency_ms), 0) AS total_latency_ms").
		Where("created_at >= ? AND created_at <= ?", probeWindowStart, probeWindowEnd).
		Group("success, monitoring_fault").
		Find(&rows.ProbeAggregates).Error; err != nil {
		return StatusCenterMetricRows{}, err
	}

	if len(componentIDs) > 0 {
		if err := db.Model(&StatusPeriod{}).
			Select("component_id, granularity, MAX(period_start) AS latest").
			Where("component_id IN ? AND granularity IN ? AND period_start <= ?", componentIDs, []string{StatusGranularityHour, StatusGranularityDay}, rollupCutoff).
			Group("component_id, granularity").
			Find(&rows.Rollups).Error; err != nil {
			return StatusCenterMetricRows{}, err
		}
	}

	if err := db.Model(&StatusIncident{}).Where("status = ?", "draft").Count(&rows.IncidentDrafts).Error; err != nil {
		return StatusCenterMetricRows{}, err
	}
	if err := db.Model(&StatusDeliveryOutbox{}).
		Select("status, COUNT(*) AS count, COALESCE(SUM(CASE WHEN attempts > 0 THEN 1 ELSE 0 END), 0) AS retried, COALESCE(MIN(created_at), 0) AS oldest_created_at").
		Where("status IN ?", []string{StatusDeliveryPending, StatusDeliveryProcessing}).
		Group("status").
		Find(&rows.ActiveOutbox).Error; err != nil {
		return StatusCenterMetricRows{}, err
	}
	if err := db.Model(&StatusDeliveryOutbox{}).Where("status = ?", StatusDeliveryDead).Count(&rows.DeadOutbox).Error; err != nil {
		return StatusCenterMetricRows{}, err
	}
	if err := db.Model(&StatusSubscriber{}).Where("status = ?", StatusSubscriberSuspended).Count(&rows.SuspendedDestinations).Error; err != nil {
		return StatusCenterMetricRows{}, err
	}
	return rows, nil
}
