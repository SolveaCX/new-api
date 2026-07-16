package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	statusSchedulerJobName            = "status-center-scheduler"
	statusSchedulerLeaseSeconds       = int64(55)
	statusFiveMinuteSeconds           = int64(5 * 60)
	statusHourSeconds                 = int64(60 * 60)
	statusDaySeconds                  = int64(24 * 60 * 60)
	statusRawRetentionSeconds         = int64(7 * 24 * 60 * 60)
	statusAggregateRetentionSeconds   = int64(100 * 24 * 60 * 60)
	statusRouterCanaryTimeout         = 10 * time.Second
	statusRouterCanaryPath            = "/api/status"
	statusSchedulerLoopInterval       = time.Minute
	statusSchedulerTrafficWindow      = int64(5 * 60)
	statusSchedulerProbeFreshness     = statusEvidenceMaxAgeSeconds
	statusSchedulerCoverageFullMicros = int64(1_000_000)
)

type StatusTrafficReader func(start int64, end int64, groups []string) ([]model.PerfMetricSummary, error)

type StatusScheduler struct {
	Holder       string
	Pricing      func() []model.Pricing
	UsableGroups func() map[string]string
	Traffic      StatusTrafficReader
	RouterProbe  StatusProbeAdapter
	ModelProbe   StatusProbeAdapter
}

type statusHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type statusRouterProbeAdapter struct {
	origin string
	client statusHTTPClient
}

var (
	statusCenterTaskOnce   = &sync.Once{}
	statusCenterTaskLaunch = launchStatusCenterTasks
	statusModelProbeMu     sync.RWMutex
	statusModelProbe       StatusProbeAdapter = StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
		return StatusProbeOutcome{MonitoringFault: true, DiagnosticType: "model_probe_not_configured"}
	})
)

func SetStatusModelProbeAdapter(adapter StatusProbeAdapter) {
	if adapter == nil {
		return
	}
	statusModelProbeMu.Lock()
	statusModelProbe = adapter
	statusModelProbeMu.Unlock()
}

func NewStatusRouterProbeAdapter(origin string, client statusHTTPClient) StatusProbeAdapter {
	return statusRouterProbeAdapter{origin: strings.TrimSpace(origin), client: client}
}

func (adapter statusRouterProbeAdapter) ProbeStatusComponent(ctx context.Context, component model.StatusComponent) StatusProbeOutcome {
	if component.Kind != model.StatusComponentKindRouter {
		return StatusProbeOutcome{MonitoringFault: true, DiagnosticType: "invalid_router_component"}
	}
	parsed, err := url.Parse(adapter.origin)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || adapter.client == nil {
		return StatusProbeOutcome{MonitoringFault: true, DiagnosticType: "router_probe_not_configured"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(ctx, statusRouterCanaryTimeout)
	defer cancel()
	probeURL := strings.TrimRight(adapter.origin, "/") + statusRouterCanaryPath
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, probeURL, nil)
	if err != nil {
		return StatusProbeOutcome{MonitoringFault: true, DiagnosticType: "router_probe_request_invalid"}
	}
	startedAt := time.Now()
	response, err := adapter.client.Do(req)
	latencyMs := time.Since(startedAt).Milliseconds()
	if err != nil {
		return StatusProbeOutcome{DiagnosticType: "router_network_failure", TargetRef: parsed.Host, LatencyMs: latencyMs}
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return StatusProbeOutcome{DiagnosticType: "router_http_failure", TargetRef: parsed.Host, LatencyMs: latencyMs}
	}
	return StatusProbeOutcome{Success: true, DiagnosticType: "ok", TargetRef: parsed.Host, LatencyMs: latencyMs}
}

func (scheduler *StatusScheduler) RunOnce(ctx context.Context, now int64) (bool, error) {
	if scheduler == nil || strings.TrimSpace(scheduler.Holder) == "" {
		return false, errors.New("status scheduler holder is required")
	}
	lease, acquired, err := model.AcquireStatusJobLease(statusSchedulerJobName, scheduler.Holder, now, statusSchedulerLeaseSeconds)
	if err != nil || !acquired {
		return false, err
	}

	pricing := []model.Pricing(nil)
	if scheduler.Pricing != nil {
		pricing = scheduler.Pricing()
	}
	usableGroups := map[string]string{}
	if scheduler.UsableGroups != nil {
		usableGroups = scheduler.UsableGroups()
	}
	if err := SyncStatusCatalog(statusSchedulerJobName, scheduler.Holder, lease.FencingToken, now, pricing, usableGroups); err != nil {
		return true, err
	}
	components, err := model.GetStatusComponents()
	if err != nil {
		return true, err
	}

	trafficByModel := make(map[string]model.PerfMetricSummary)
	if scheduler.Traffic != nil {
		groups := sortedUsableGroupNames(usableGroups)
		summaries, err := scheduler.Traffic(now-statusSchedulerTrafficWindow, now, groups)
		if err != nil {
			return true, err
		}
		for _, summary := range summaries {
			trafficByModel[summary.ModelName] = summary
		}
	}
	componentIDs := make([]int64, 0, len(components))
	for _, component := range components {
		if component.Lifecycle == model.StatusLifecycleActive {
			componentIDs = append(componentIDs, component.ID)
		}
	}
	latestProbes, err := model.GetLatestStatusProbeResults(componentIDs, now-statusSchedulerProbeFreshness)
	if err != nil {
		return true, err
	}

	for i := range components {
		component := components[i]
		if component.Lifecycle != model.StatusLifecycleActive {
			continue
		}
		summary := trafficByModel[component.ModelName]
		latestProbe, hasProbe := latestProbes[component.ID]
		conflict := statusSignalsConflict(summary, latestProbe, hasProbe)
		schedule := StatusProbeSchedule{
			Kind:            component.Kind,
			Lifecycle:       component.Lifecycle,
			EffectiveStatus: component.EffectiveStatus,
			EligibleCount:   summary.AvailabilityEligibleCount,
			SignalConflict:  conflict,
		}
		if hasProbe {
			schedule.LastProbeAt = latestProbe.CreatedAt
			schedule.MonitoringFault = latestProbe.MonitoringFault
		}
		probeRan := false
		if StatusProbeDue(schedule, now) {
			adapter := scheduler.ModelProbe
			if component.Kind == model.StatusComponentKindRouter {
				adapter = scheduler.RouterProbe
			}
			outcome := StatusProbeOutcome{MonitoringFault: true, DiagnosticType: "probe_not_configured"}
			if adapter != nil {
				outcome = adapter.ProbeStatusComponent(ctx, component)
			}
			latestProbe = model.StatusProbeResult{
				ComponentID:     component.ID,
				Success:         outcome.Success,
				MonitoringFault: outcome.MonitoringFault,
				DiagnosticType:  outcome.DiagnosticType,
				TargetRef:       outcome.TargetRef,
				LatencyMs:       outcome.LatencyMs,
				FencingToken:    lease.FencingToken,
				CreatedAt:       now,
			}
			if err := persistStatusProbeWithFence(statusSchedulerJobName, scheduler.Holder, lease.FencingToken, now, &latestProbe); err != nil {
				return true, err
			}
			hasProbe = true
			probeRan = true
			conflict = statusSignalsConflict(summary, latestProbe, true)
		}

		evidence := statusEvidenceForComponent(summary, latestProbe, hasProbe, conflict, now)
		transition := EvaluateStatus(component, evidence, now)
		component.ObservedStatus = transition.Observed
		component.EffectiveStatus = transition.Effective
		component.StatusSource = transition.Source
		component.LastTrustworthyUpdateAt = transition.TrustworthyAt
		component.LastEvaluatedAt = now
		component.ConsecutiveProbeFailures = transition.ConsecutiveProbeFailures
		component.ConsecutiveProbeSuccesses = transition.ConsecutiveProbeSuccesses
		component.ConsecutiveTrafficRecovery = transition.ConsecutiveTrafficRecovery
		component.CoverageMicros = 0
		if transition.ScoreMicros != nil {
			component.CoverageMicros = statusSchedulerCoverageFullMicros
		}
		if summary.AvailabilityEligibleCount > 0 || probeRan {
			component.LastEvidenceAt = now
		}
		component.UpdatedAt = now
		if err := model.CommitStatusEvaluationWithFence(statusSchedulerJobName, scheduler.Holder, lease.FencingToken, now, &component); err != nil {
			return true, err
		}

		period := statusFiveMinutePeriod(component, evidence, transition, now)
		if err := writeStatusPeriodWithFence(statusSchedulerJobName, scheduler.Holder, lease.FencingToken, now, &period); err != nil {
			return true, err
		}
	}
	if err := rollupStatusPeriodsWithFence(statusSchedulerJobName, scheduler.Holder, lease.FencingToken, now); err != nil {
		return true, err
	}
	if err := applyStatusRetentionWithFence(statusSchedulerJobName, scheduler.Holder, lease.FencingToken, now); err != nil {
		return true, err
	}
	return true, nil
}

func persistStatusProbeWithFence(jobName string, holder string, fencingToken int64, now int64, result *model.StatusProbeResult) error {
	return model.CreateStatusProbeResultWithFence(jobName, holder, fencingToken, now, result)
}

func writeStatusPeriodWithFence(jobName string, holder string, fencingToken int64, now int64, period *model.StatusPeriod) error {
	return model.UpsertStatusPeriodWithFence(jobName, holder, fencingToken, now, period)
}

func rollupStatusPeriodsWithFence(jobName string, holder string, fencingToken int64, now int64) error {
	if err := model.ValidateStatusJobFence(jobName, holder, fencingToken, now); err != nil {
		return err
	}
	hourStart := ((now - 1) / statusHourSeconds) * statusHourSeconds
	fiveMinutePeriods, err := model.GetStatusPeriodsInRange(model.StatusGranularityFiveMinutes, hourStart, hourStart+statusHourSeconds)
	if err != nil {
		return err
	}
	for _, period := range aggregateStatusPeriods(fiveMinutePeriods, model.StatusGranularityHour, hourStart, now) {
		period := period
		if err := writeStatusPeriodWithFence(jobName, holder, fencingToken, now, &period); err != nil {
			return err
		}
	}

	dayStart := ((now - 1) / statusDaySeconds) * statusDaySeconds
	hourPeriods, err := model.GetStatusPeriodsInRange(model.StatusGranularityHour, dayStart, dayStart+statusDaySeconds)
	if err != nil {
		return err
	}
	for _, period := range aggregateStatusPeriods(hourPeriods, model.StatusGranularityDay, dayStart, now) {
		period := period
		if err := writeStatusPeriodWithFence(jobName, holder, fencingToken, now, &period); err != nil {
			return err
		}
	}
	return nil
}

func applyStatusRetentionWithFence(jobName string, holder string, fencingToken int64, now int64) error {
	return model.DeleteStatusHistoryWithFence(
		jobName,
		holder,
		fencingToken,
		now,
		now-statusRawRetentionSeconds,
		now-statusAggregateRetentionSeconds,
	)
}

func statusEvidenceForComponent(summary model.PerfMetricSummary, probe model.StatusProbeResult, hasProbe bool, conflict bool, now int64) StatusEvidence {
	if summary.AvailabilityEligibleCount >= statusTrafficMinimumEligible {
		return StatusEvidence{
			Eligible:          summary.AvailabilityEligibleCount,
			Success:           summary.AvailabilitySuccessCount,
			LastTrustworthyAt: now,
		}
	}
	if !hasProbe || now-probe.CreatedAt >= statusSchedulerProbeFreshness {
		return StatusEvidence{}
	}
	evidence := StatusEvidenceFromProbe(StatusProbeOutcome{
		Success:         probe.Success,
		MonitoringFault: probe.MonitoringFault,
		DiagnosticType:  probe.DiagnosticType,
		TargetRef:       probe.TargetRef,
		LatencyMs:       probe.LatencyMs,
	}, probe.CreatedAt)
	evidence.SignalConflict = conflict
	return evidence
}

func statusSignalsConflict(summary model.PerfMetricSummary, probe model.StatusProbeResult, hasProbe bool) bool {
	eligible := summary.AvailabilityEligibleCount
	if eligible <= 0 || eligible >= statusTrafficMinimumEligible || !hasProbe || probe.MonitoringFault {
		return false
	}
	trafficHealthy := ratioMicros(summary.AvailabilitySuccessCount, eligible) >= statusOperationalMicros
	return trafficHealthy != probe.Success
}

func statusFiveMinutePeriod(component model.StatusComponent, evidence StatusEvidence, transition StatusTransition, now int64) model.StatusPeriod {
	period := model.StatusPeriod{
		ComponentID:       component.ID,
		Granularity:       model.StatusGranularityFiveMinutes,
		PeriodStart:       (now / statusFiveMinuteSeconds) * statusFiveMinuteSeconds,
		WorstStatus:       transition.Effective,
		EligibleCount:     evidence.Eligible,
		SuccessCount:      evidence.Success,
		ProbeSuccessCount: evidence.ProbeSuccess,
		ProbeFailureCount: evidence.ProbeFailure,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if transition.Effective == model.StatusMaintenance {
		period.MaintenanceBucketCount = 1
	} else if transition.ScoreMicros == nil {
		period.UnknownBucketCount = 1
	} else {
		period.ScoreSumMicros = *transition.ScoreMicros
		period.KnownBucketCount = 1
	}
	return period
}

func aggregateStatusPeriods(source []model.StatusPeriod, granularity string, periodStart int64, now int64) []model.StatusPeriod {
	byComponent := make(map[int64]*model.StatusPeriod)
	for _, item := range source {
		aggregate := byComponent[item.ComponentID]
		if aggregate == nil {
			aggregate = &model.StatusPeriod{
				ComponentID: item.ComponentID,
				Granularity: granularity,
				PeriodStart: periodStart,
				WorstStatus: item.WorstStatus,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			byComponent[item.ComponentID] = aggregate
		}
		aggregate.ScoreSumMicros += item.ScoreSumMicros
		aggregate.KnownBucketCount += item.KnownBucketCount
		aggregate.UnknownBucketCount += item.UnknownBucketCount
		aggregate.MaintenanceBucketCount += item.MaintenanceBucketCount
		aggregate.EligibleCount += item.EligibleCount
		aggregate.SuccessCount += item.SuccessCount
		aggregate.ProbeSuccessCount += item.ProbeSuccessCount
		aggregate.ProbeFailureCount += item.ProbeFailureCount
		aggregate.LatencySumMs += item.LatencySumMs
		aggregate.LatencyCount += item.LatencyCount
		aggregate.TtftSumMs += item.TtftSumMs
		aggregate.TtftCount += item.TtftCount
		aggregate.WorstStatus = worseStatus(aggregate.WorstStatus, item.WorstStatus)
	}
	componentIDs := make([]int64, 0, len(byComponent))
	for componentID := range byComponent {
		componentIDs = append(componentIDs, componentID)
	}
	sort.Slice(componentIDs, func(i, j int) bool { return componentIDs[i] < componentIDs[j] })
	result := make([]model.StatusPeriod, 0, len(componentIDs))
	for _, componentID := range componentIDs {
		result = append(result, *byComponent[componentID])
	}
	return result
}

func worseStatus(left string, right string) string {
	rank := map[string]int{
		model.StatusOperational: 1,
		model.StatusMaintenance: 2,
		model.StatusUnknown:     3,
		model.StatusDegraded:    4,
		model.StatusOutage:      5,
	}
	if rank[right] > rank[left] {
		return right
	}
	return left
}

func StartStatusCenterTasks() bool {
	if !common.IsMasterNode || !common.GetEnvOrDefaultBool("STATUS_CENTER_ENABLED", false) {
		return false
	}
	started := false
	statusCenterTaskOnce.Do(func() {
		started = true
		statusCenterTaskLaunch()
	})
	return started
}

func launchStatusCenterTasks() {
	gopool.Go(func() {
		scheduler := &StatusScheduler{
			Holder:       statusSchedulerHolder(),
			Pricing:      model.GetPricing,
			UsableGroups: func() map[string]string { return GetUserUsableGroups("") },
			Traffic:      model.GetPerfMetricsSummaryAll,
			RouterProbe:  NewStatusRouterProbeAdapter(common.GetEnvOrDefaultString("ROUTER_ORIGIN", ""), GetHttpClient()),
			ModelProbe:   configuredStatusModelProbe(),
		}
		run := func() {
			if _, err := scheduler.RunOnce(context.Background(), model.GetDBTimestamp()); err != nil {
				common.SysError("status center scheduler failed: " + err.Error())
			}
		}
		run()
		ticker := time.NewTicker(statusSchedulerLoopInterval)
		defer ticker.Stop()
		for range ticker.C {
			run()
		}
	})
}

func configuredStatusModelProbe() StatusProbeAdapter {
	statusModelProbeMu.RLock()
	defer statusModelProbeMu.RUnlock()
	return statusModelProbe
}

func statusSchedulerHolder() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "unknown-host"
	}
	return fmt.Sprintf("%s:%d", hostname, os.Getpid())
}
