package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
)

const statusMetricsProbeWindowSeconds = int64(time.Hour / time.Second)

type statusCenterMetricSnapshot struct {
	featureEnabled          map[string]int64
	leaseActive             int64
	leaseRemainingSeconds   int64
	componentInventoryReady int64
	evaluatorLag            int64
	probeQueueDepth         int64
	probeResults            map[string]int64
	probeDurationSeconds    float64
	probeRequests           int64
	unknownModels           int64
	coverageComponents      map[string]int64
	rollupReady             map[string]int64
	rollupLag               map[string]int64
	incidentDrafts          int64
	outboxDepth             map[string]int64
	outboxDead              int64
	outboxOldestAge         int64
	outboxRetryRatio        float64
	suspendedDestinations   int64
	keyringHealthy          int64
}

func BuildStatusCenterPrometheusText(ctx context.Context, now int64) (string, error) {
	if now <= 0 {
		now = time.Now().Unix()
	}
	fiveMinuteStart, fiveMinuteEnd := statusFiveMinuteBounds(now - statusAvailabilityFlushGrace)
	groups := sortedUsableGroupNames(WebsitePublicUsableGroups())
	rows, err := model.GetStatusCenterMetricRows(
		ctx,
		now-statusMetricsProbeWindowSeconds,
		now,
		now-statusSchedulerProbeFreshness,
		fiveMinuteStart,
		fiveMinuteEnd-1,
		now,
		groups,
	)
	if err != nil {
		return "", err
	}
	trafficByModel := make(map[string]model.PerfMetricSummary, len(rows.Traffic))
	for _, summary := range rows.Traffic {
		trafficByModel[summary.ModelName] = summary
	}

	snapshot := newStatusCenterMetricSnapshot(now)
	if keyring, keyringErr := LoadStatusSecretKeyringFromEnvironment(); keyringErr == nil && keyring.Enabled() {
		snapshot.keyringHealthy = 1
	}
	if rows.Lease != nil && rows.Lease.ExpiresAt > now {
		snapshot.leaseActive = 1
		snapshot.leaseRemainingSeconds = rows.Lease.ExpiresAt - now
	}
	if len(rows.Components) > 0 {
		snapshot.componentInventoryReady = 1
	}
	for _, component := range rows.Components {
		snapshot.evaluatorLag = maxStatusMetric(snapshot.evaluatorLag, statusMetricLag(now, component.LastEvaluatedAt))
		if component.Kind == model.StatusComponentKindModel && component.EffectiveStatus == model.StatusUnknown {
			snapshot.unknownModels++
		}
		switch {
		case component.CoverageMicros <= 0:
			snapshot.coverageComponents["zero"]++
		case component.CoverageMicros < statusSchedulerCoverageFullMicros:
			snapshot.coverageComponents["partial"]++
		default:
			snapshot.coverageComponents["full"]++
		}
	}
	snapshot.probeQueueDepth = statusMetricsProbeQueueDepth(rows.Components, trafficByModel, rows.LatestProbes, now)

	var totalProbeLatencyMs int64
	for _, aggregate := range rows.ProbeAggregates {
		snapshot.probeRequests += aggregate.Count
		totalProbeLatencyMs += aggregate.TotalLatencyMs
		switch {
		case aggregate.MonitoringFault:
			snapshot.probeResults["monitoring_fault"] += aggregate.Count
		case aggregate.Success:
			snapshot.probeResults["success"] += aggregate.Count
		default:
			snapshot.probeResults["failure"] += aggregate.Count
		}
	}
	if snapshot.probeRequests > 0 {
		snapshot.probeDurationSeconds = float64(totalProbeLatencyMs) / float64(snapshot.probeRequests) / 1_000
	}
	populateStatusRollupMetrics(&snapshot, rows.Components, rows.Rollups, now)

	snapshot.incidentDrafts = rows.IncidentDrafts
	var activeOutbox int64
	var retriedOutbox int64
	for _, row := range rows.ActiveOutbox {
		snapshot.outboxDepth[row.Status] = row.Count
		activeOutbox += row.Count
		retriedOutbox += row.Retried
		if row.OldestCreatedAt > 0 {
			snapshot.outboxOldestAge = maxStatusMetric(snapshot.outboxOldestAge, statusMetricLag(now, row.OldestCreatedAt))
		}
	}
	if activeOutbox > 0 {
		snapshot.outboxRetryRatio = float64(retriedOutbox) / float64(activeOutbox)
	}
	snapshot.outboxDead = rows.DeadOutbox
	snapshot.suspendedDestinations = rows.SuspendedDestinations
	return renderStatusCenterMetrics(snapshot), nil
}

func newStatusCenterMetricSnapshot(now int64) statusCenterMetricSnapshot {
	return statusCenterMetricSnapshot{
		featureEnabled: map[string]int64{
			"scheduler":     boolStatusMetric(IsStatusCenterEnabled()),
			"public":        boolStatusMetric(IsStatusCenterPublicEnabled()),
			"notifications": boolStatusMetric(IsStatusCenterNotificationsEnabled()),
			"shadow":        boolStatusMetric(IsStatusCenterShadowMode()),
		},
		probeResults:       map[string]int64{"success": 0, "failure": 0, "monitoring_fault": 0},
		coverageComponents: map[string]int64{"zero": 0, "partial": 0, "full": 0},
		rollupReady:        map[string]int64{model.StatusGranularityHour: 0, model.StatusGranularityDay: 0},
		rollupLag:          map[string]int64{model.StatusGranularityHour: now, model.StatusGranularityDay: now},
		outboxDepth: map[string]int64{
			model.StatusDeliveryPending:    0,
			model.StatusDeliveryProcessing: 0,
		},
	}
}

func statusMetricsProbeQueueDepth(components []model.StatusComponent, trafficByModel map[string]model.PerfMetricSummary, latestProbes map[int64]model.StatusProbeResult, now int64) int64 {
	selectedModels := selectDueStatusModelProbes(components, trafficByModel, latestProbes, now, statusDefaultModelProbeBudget)
	depth := int64(len(selectedModels))
	for _, component := range components {
		if component.Kind == model.StatusComponentKindModel || component.Lifecycle != model.StatusLifecycleActive {
			continue
		}
		latestProbe, hasProbe := latestProbes[component.ID]
		schedule := StatusProbeSchedule{
			Kind:            component.Kind,
			Lifecycle:       component.Lifecycle,
			EffectiveStatus: component.EffectiveStatus,
		}
		if hasProbe {
			schedule.LastProbeAt = latestProbe.CreatedAt
			schedule.MonitoringFault = latestProbe.MonitoringFault
		}
		if StatusProbeDue(schedule, now) {
			depth++
		}
	}
	return depth
}

func populateStatusRollupMetrics(snapshot *statusCenterMetricSnapshot, components []model.StatusComponent, rollups []model.StatusRollupMetricRow, now int64) {
	if snapshot == nil || len(components) == 0 {
		return
	}
	latest := make(map[string]map[int64]int64, 2)
	for _, granularity := range []string{model.StatusGranularityHour, model.StatusGranularityDay} {
		latest[granularity] = make(map[int64]int64, len(components))
	}
	for _, rollup := range rollups {
		if byComponent, ok := latest[rollup.Granularity]; ok {
			byComponent[rollup.ComponentID] = rollup.Latest
		}
	}
	for _, granularity := range []string{model.StatusGranularityHour, model.StatusGranularityDay} {
		ready := int64(1)
		lag := int64(0)
		for _, component := range components {
			periodStart, ok := latest[granularity][component.ID]
			if !ok || periodStart <= 0 || periodStart > now {
				ready = 0
				lag = now
				break
			}
			lag = maxStatusMetric(lag, statusMetricLag(now, periodStart))
		}
		snapshot.rollupReady[granularity] = ready
		snapshot.rollupLag[granularity] = lag
	}
}

func renderStatusCenterMetrics(snapshot statusCenterMetricSnapshot) string {
	var builder strings.Builder
	writeStatusMetricHeader(&builder, "newapi_status_center_metrics_up", "Whether status center metrics were collected successfully.")
	writeStatusMetric(&builder, "newapi_status_center_metrics_up", "", 1)

	writeStatusMetricHeader(&builder, "newapi_status_center_feature_enabled", "Whether each effective status center feature is enabled.")
	for _, feature := range []string{"scheduler", "public", "notifications", "shadow"} {
		writeStatusMetric(&builder, "newapi_status_center_feature_enabled", `feature="`+feature+`"`, snapshot.featureEnabled[feature])
	}
	writeStatusMetricHeader(&builder, "newapi_status_center_scheduler_lease_active", "Whether the scheduler lease is currently active.")
	writeStatusMetric(&builder, "newapi_status_center_scheduler_lease_active", "", snapshot.leaseActive)
	writeStatusMetricHeader(&builder, "newapi_status_center_scheduler_lease_remaining_seconds", "Remaining scheduler lease lifetime without exposing the holder identity.")
	writeStatusMetric(&builder, "newapi_status_center_scheduler_lease_remaining_seconds", "", snapshot.leaseRemainingSeconds)
	writeStatusMetricHeader(&builder, "newapi_status_center_component_inventory_ready", "Whether at least one active status component exists.")
	writeStatusMetric(&builder, "newapi_status_center_component_inventory_ready", "", snapshot.componentInventoryReady)
	writeStatusMetricHeader(&builder, "newapi_status_center_evaluator_lag_seconds", "Maximum age of active component evaluation data.")
	writeStatusMetric(&builder, "newapi_status_center_evaluator_lag_seconds", "", snapshot.evaluatorLag)
	writeStatusMetricHeader(&builder, "newapi_status_center_probe_queue_depth", "Number of active components selected for the next probe run.")
	writeStatusMetric(&builder, "newapi_status_center_probe_queue_depth", "", snapshot.probeQueueDepth)
	writeStatusMetricHeader(&builder, "newapi_status_center_probe_results", "Probe result counts observed during the last hour.")
	for _, result := range []string{"success", "failure", "monitoring_fault"} {
		writeStatusMetric(&builder, "newapi_status_center_probe_results", `result="`+result+`"`, snapshot.probeResults[result])
	}
	writeStatusMetricHeader(&builder, "newapi_status_center_probe_requests", "Probe request count observed during the last hour.")
	writeStatusMetric(&builder, "newapi_status_center_probe_requests", "", snapshot.probeRequests)
	writeStatusMetricHeader(&builder, "newapi_status_center_probe_duration_seconds", "Average probe duration during the last hour in seconds.")
	writeStatusFloatMetric(&builder, "newapi_status_center_probe_duration_seconds", "", snapshot.probeDurationSeconds)
	writeStatusMetricHeader(&builder, "newapi_status_center_unknown_models", "Number of active models whose effective state is unknown.")
	writeStatusMetric(&builder, "newapi_status_center_unknown_models", "", snapshot.unknownModels)
	writeStatusMetricHeader(&builder, "newapi_status_center_coverage_components", "Active component counts by aggregate coverage bucket.")
	for _, coverage := range []string{"zero", "partial", "full"} {
		writeStatusMetric(&builder, "newapi_status_center_coverage_components", `coverage="`+coverage+`"`, snapshot.coverageComponents[coverage])
	}
	writeStatusMetricHeader(&builder, "newapi_status_center_rollup_ready", "Whether every active component has produced each aggregate rollup.")
	writeStatusMetricHeader(&builder, "newapi_status_center_rollup_lag_seconds", "Maximum age of each active component's latest aggregate rollup.")
	for _, granularity := range []string{model.StatusGranularityHour, model.StatusGranularityDay} {
		labels := `granularity="` + granularity + `"`
		writeStatusMetric(&builder, "newapi_status_center_rollup_ready", labels, snapshot.rollupReady[granularity])
		writeStatusMetric(&builder, "newapi_status_center_rollup_lag_seconds", labels, snapshot.rollupLag[granularity])
	}
	writeStatusMetricHeader(&builder, "newapi_status_center_incident_drafts", "Number of unpublished incident or maintenance drafts.")
	writeStatusMetric(&builder, "newapi_status_center_incident_drafts", "", snapshot.incidentDrafts)
	writeStatusMetricHeader(&builder, "newapi_status_center_outbox_depth", "Pending and processing status notification outbox rows by state.")
	for _, status := range []string{model.StatusDeliveryPending, model.StatusDeliveryProcessing} {
		writeStatusMetric(&builder, "newapi_status_center_outbox_depth", `status="`+status+`"`, snapshot.outboxDepth[status])
	}
	writeStatusMetricHeader(&builder, "newapi_status_center_outbox_dead", "Number of dead status notification deliveries.")
	writeStatusMetric(&builder, "newapi_status_center_outbox_dead", "", snapshot.outboxDead)
	writeStatusMetricHeader(&builder, "newapi_status_center_outbox_oldest_age_seconds", "Age of the oldest pending or processing delivery.")
	writeStatusMetric(&builder, "newapi_status_center_outbox_oldest_age_seconds", "", snapshot.outboxOldestAge)
	writeStatusMetricHeader(&builder, "newapi_status_center_outbox_retry_ratio", "Fraction of the active delivery queue that has been retried.")
	writeStatusFloatMetric(&builder, "newapi_status_center_outbox_retry_ratio", "", snapshot.outboxRetryRatio)
	writeStatusMetricHeader(&builder, "newapi_status_center_suspended_destinations", "Number of suspended notification destinations.")
	writeStatusMetric(&builder, "newapi_status_center_suspended_destinations", "", snapshot.suspendedDestinations)
	writeStatusMetricHeader(&builder, "newapi_status_center_keyring_healthy", "Whether the status notification encryption keyring is configured and valid.")
	writeStatusMetric(&builder, "newapi_status_center_keyring_healthy", "", snapshot.keyringHealthy)
	return builder.String()
}

func writeStatusMetricHeader(builder *strings.Builder, name string, help string) {
	fmt.Fprintf(builder, "# HELP %s %s\n# TYPE %s gauge\n", name, help, name)
}

func writeStatusMetric(builder *strings.Builder, name string, labels string, value int64) {
	if labels != "" {
		fmt.Fprintf(builder, "%s{%s} %d\n", name, labels, value)
		return
	}
	fmt.Fprintf(builder, "%s %d\n", name, value)
}

func writeStatusFloatMetric(builder *strings.Builder, name string, labels string, value float64) {
	formatted := strconv.FormatFloat(value, 'f', 6, 64)
	formatted = strings.TrimRight(strings.TrimRight(formatted, "0"), ".")
	if formatted == "" {
		formatted = "0"
	}
	if labels != "" {
		fmt.Fprintf(builder, "%s{%s} %s\n", name, labels, formatted)
		return
	}
	fmt.Fprintf(builder, "%s %s\n", name, formatted)
}

func boolStatusMetric(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func statusMetricLag(now int64, timestamp int64) int64 {
	if timestamp <= 0 {
		return now
	}
	if timestamp >= now {
		return 0
	}
	return now - timestamp
}

func maxStatusMetric(left int64, right int64) int64 {
	if right > left {
		return right
	}
	return left
}
