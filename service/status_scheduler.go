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
	statusSchedulerLeaseRenewInterval = 20 * time.Second
	statusSchedulerProbeFreshness     = statusEvidenceMaxAgeSeconds
	statusSchedulerCoverageFullMicros = int64(1_000_000)
	statusAvailabilityFlushGrace      = int64(10)
	statusDefaultModelProbeBudget     = 10
)

type StatusTrafficReader func(start int64, end int64, groups []string) ([]model.PerfMetricSummary, error)

type StatusModelAvailabilityWriter func(jobName string, holder string, fencingToken int64, now int64, modelName string, outcome StatusProbeOutcome) error

type StatusLeaseRenewer func(name string, holder string, fencingToken int64, now int64, leaseSeconds int64) (bool, error)

func readStatusTraffic(start int64, end int64, groups []string) ([]model.PerfMetricSummary, error) {
	return model.GetPerfMetricAvailabilitySummaryAll(start, end-1, groups)
}

type StatusScheduler struct {
	Holder              string
	Now                 func() int64
	LeaseSeconds        int64
	LeaseRenewInterval  time.Duration
	RenewLease          StatusLeaseRenewer
	Pricing             func() []model.Pricing
	UsableGroups        func() map[string]string
	Traffic             StatusTrafficReader
	CompatibilityModels func() ([]string, error)
	RouterProbe         StatusProbeAdapter
	ModelProbe          StatusProbeAdapter
	ModelProbeBudget    int
	Availability        StatusModelAvailabilityWriter
}

type statusHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type statusRouterProbeAdapter struct {
	origin string
	client statusHTTPClient
}

type statusLeaseKeeper struct {
	cancel   context.CancelFunc
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	errMu    sync.RWMutex
	err      error
}

var (
	statusCenterTaskOnce        = &sync.Once{}
	statusDeliveryTaskOnce      = &sync.Once{}
	statusLeaseAcquiredOnce     = &sync.Once{}
	statusLeaseAcquiredObserver = func(holder string) {
		common.SysLog("status center scheduler lease acquired by " + holder)
	}
	statusCenterTaskLaunch   = launchStatusCenterTasks
	statusDeliveryTaskLaunch = launchStatusDeliveryTasks
	statusModelProbeMu       sync.RWMutex
	statusModelProbe         StatusProbeAdapter = StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
		return StatusProbeOutcome{MonitoringFault: true, DiagnosticType: "model_probe_not_configured"}
	})
	statusAvailabilityMu     sync.RWMutex
	statusAvailabilityWriter StatusModelAvailabilityWriter
)

func SetStatusModelProbeAdapter(adapter StatusProbeAdapter) {
	if adapter == nil {
		return
	}
	statusModelProbeMu.Lock()
	statusModelProbe = adapter
	statusModelProbeMu.Unlock()
}

func SetStatusModelAvailabilityWriter(writer StatusModelAvailabilityWriter) {
	if writer == nil {
		return
	}
	statusAvailabilityMu.Lock()
	statusAvailabilityWriter = writer
	statusAvailabilityMu.Unlock()
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

func (scheduler *StatusScheduler) RunOnce(ctx context.Context, now int64) (ran bool, runErr error) {
	if scheduler == nil || strings.TrimSpace(scheduler.Holder) == "" {
		return false, errors.New("status scheduler holder is required")
	}
	leaseSeconds := scheduler.statusLeaseSeconds()
	lease, acquired, err := model.AcquireStatusJobLease(statusSchedulerJobName, scheduler.Holder, now, leaseSeconds)
	if err != nil || !acquired {
		return false, err
	}
	statusLeaseAcquiredOnce.Do(func() {
		statusLeaseAcquiredObserver(scheduler.Holder)
	})
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, leaseKeeper := scheduler.startStatusLeaseKeeper(ctx, lease.FencingToken, leaseSeconds)
	defer func() {
		leaseKeeper.stopKeeping()
		_, err := model.ReleaseStatusJobLease(statusSchedulerJobName, scheduler.Holder, lease.FencingToken, scheduler.currentTime())
		if err != nil && runErr == nil {
			runErr = fmt.Errorf("status job lease release failed: %w", err)
		}
	}()

	pricing := []model.Pricing(nil)
	if scheduler.Pricing != nil {
		pricing = scheduler.Pricing()
	}
	usableGroups := map[string]string{}
	if scheduler.UsableGroups != nil {
		usableGroups = scheduler.UsableGroups()
	}
	if err := SyncStatusCatalog(statusSchedulerJobName, scheduler.Holder, lease.FencingToken, scheduler.currentTime(), pricing, usableGroups); err != nil {
		return true, err
	}
	if err := leaseKeeper.renewalError(); err != nil {
		return true, err
	}
	components, err := model.GetStatusComponents()
	if err != nil {
		return true, err
	}

	trafficByModel := make(map[string]model.PerfMetricSummary)
	fiveMinuteStart, fiveMinuteEnd := statusFiveMinuteBounds(now - statusAvailabilityFlushGrace)
	if scheduler.Traffic != nil {
		groups := sortedUsableGroupNames(usableGroups)
		summaries, err := scheduler.Traffic(fiveMinuteStart, fiveMinuteEnd, groups)
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
	selectedModelProbes := selectDueStatusModelProbes(components, trafficByModel, latestProbes, now, scheduler.modelProbeBudget())

	for i := range components {
		if err := leaseKeeper.renewalError(); err != nil {
			return true, err
		}
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
		var trustworthyProbe *StatusProbeOutcome
		probeDue := StatusProbeDue(schedule, now)
		if component.Kind == model.StatusComponentKindModel {
			_, probeDue = selectedModelProbes[component.ID]
		}
		if probeDue {
			adapter := scheduler.ModelProbe
			if component.Kind == model.StatusComponentKindRouter {
				adapter = scheduler.RouterProbe
			}
			outcome := StatusProbeOutcome{MonitoringFault: true, DiagnosticType: "probe_not_configured"}
			if adapter != nil {
				outcome = adapter.ProbeStatusComponent(runCtx, component)
			}
			if err := leaseKeeper.renewalError(); err != nil {
				return true, err
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
			if err := persistStatusProbeWithFence(statusSchedulerJobName, scheduler.Holder, lease.FencingToken, scheduler.currentTime(), &latestProbe); err != nil {
				return true, err
			}
			hasProbe = true
			probeRan = true
			if !outcome.MonitoringFault && component.Kind == model.StatusComponentKindModel {
				trustworthyProbe = &outcome
			}
			conflict = statusSignalsConflict(summary, latestProbe, true)
		}

		evidence := statusEvidenceForComponent(summary, latestProbe, hasProbe, probeRan, conflict, fiveMinuteStart, now)
		transition := EvaluateStatus(component, evidence, now)
		component.ObservedStatus = transition.Observed
		component.EffectiveStatus = transition.Effective
		component.StatusSource = transition.Source
		component.LastTrustworthyUpdateAt = transition.TrustworthyAt
		component.LastEvaluatedAt = now
		component.ConsecutiveProbeFailures = transition.ConsecutiveProbeFailures
		component.ConsecutiveProbeSuccesses = transition.ConsecutiveProbeSuccesses
		component.ConsecutiveTrafficRecovery = transition.ConsecutiveTrafficRecovery
		component.LastTrafficBucketStart = transition.LastTrafficBucketStart
		component.CoverageMicros = 0
		if transition.ScoreMicros != nil {
			component.CoverageMicros = statusSchedulerCoverageFullMicros
		}
		if summary.AvailabilityEligibleCount > 0 || probeRan {
			component.LastEvidenceAt = now
		}
		component.UpdatedAt = now
		if err := leaseKeeper.renewalError(); err != nil {
			return true, err
		}
		if err := model.CommitStatusEvaluationWithFence(statusSchedulerJobName, scheduler.Holder, lease.FencingToken, scheduler.currentTime(), &component); err != nil {
			return true, err
		}
		if component.Kind == model.StatusComponentKindModel && scheduler.Availability != nil {
			availability := trustworthyProbe
			if summary.AvailabilityEligibleCount >= statusTrafficMinimumEligible {
				if ratioMicros(summary.AvailabilitySuccessCount, summary.AvailabilityEligibleCount) >= statusOperationalMicros {
					availability = &StatusProbeOutcome{Success: true, DiagnosticType: "ok"}
				} else {
					availability = &StatusProbeOutcome{DiagnosticType: "traffic_failure"}
				}
			}
			if availability != nil {
				if err := scheduler.Availability(statusSchedulerJobName, scheduler.Holder, lease.FencingToken, scheduler.currentTime(), component.ModelName, *availability); err != nil {
					return true, err
				}
			}
		}

		period := statusFiveMinutePeriod(component, evidence, transition, fiveMinuteStart, now)
		if err := writeStatusFiveMinutePeriodWithFence(statusSchedulerJobName, scheduler.Holder, lease.FencingToken, scheduler.currentTime(), &period); err != nil {
			return true, err
		}
	}
	remainingModelProbeBudget := scheduler.modelProbeBudget() - len(selectedModelProbes)
	if err := scheduler.runCompatibilityProbes(runCtx, leaseKeeper, components, lease.FencingToken, now, remainingModelProbeBudget); err != nil {
		return true, err
	}
	if err := leaseKeeper.renewalError(); err != nil {
		return true, err
	}
	if err := rollupStatusPeriodsForFiveMinuteWithClock(statusSchedulerJobName, scheduler.Holder, lease.FencingToken, fiveMinuteStart, now, scheduler.currentTime); err != nil {
		return true, err
	}
	if err := leaseKeeper.renewalError(); err != nil {
		return true, err
	}
	if err := applyStatusRetentionWithFence(statusSchedulerJobName, scheduler.Holder, lease.FencingToken, scheduler.currentTime()); err != nil {
		return true, err
	}
	return true, nil
}

func selectDueStatusModelProbes(components []model.StatusComponent, trafficByModel map[string]model.PerfMetricSummary, latestProbes map[int64]model.StatusProbeResult, now int64, budget int) map[int64]struct{} {
	type candidate struct {
		componentID int64
		modelName   string
		lastProbeAt int64
	}
	candidates := make([]candidate, 0, len(components))
	for _, component := range components {
		if component.Kind != model.StatusComponentKindModel || component.Lifecycle != model.StatusLifecycleActive {
			continue
		}
		summary := trafficByModel[component.ModelName]
		latestProbe, hasProbe := latestProbes[component.ID]
		schedule := StatusProbeSchedule{
			Kind:            component.Kind,
			Lifecycle:       component.Lifecycle,
			EffectiveStatus: component.EffectiveStatus,
			EligibleCount:   summary.AvailabilityEligibleCount,
			SignalConflict:  statusSignalsConflict(summary, latestProbe, hasProbe),
		}
		if hasProbe {
			schedule.LastProbeAt = latestProbe.CreatedAt
			schedule.MonitoringFault = latestProbe.MonitoringFault
		}
		if StatusProbeDue(schedule, now) {
			candidates = append(candidates, candidate{componentID: component.ID, modelName: component.ModelName, lastProbeAt: schedule.LastProbeAt})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].lastProbeAt != candidates[j].lastProbeAt {
			return candidates[i].lastProbeAt < candidates[j].lastProbeAt
		}
		return candidates[i].modelName < candidates[j].modelName
	})
	if budget > len(candidates) {
		budget = len(candidates)
	}
	selected := make(map[int64]struct{}, budget)
	for i := 0; i < budget; i++ {
		selected[candidates[i].componentID] = struct{}{}
	}
	return selected
}

func (scheduler *StatusScheduler) runCompatibilityProbes(ctx context.Context, leaseKeeper *statusLeaseKeeper, components []model.StatusComponent, fencingToken int64, now int64, budget int) error {
	if budget <= 0 || scheduler.CompatibilityModels == nil || scheduler.ModelProbe == nil || scheduler.Availability == nil {
		return nil
	}
	modelNames, err := scheduler.CompatibilityModels()
	if err != nil {
		return err
	}
	publicModels := make(map[string]struct{}, len(components))
	for _, component := range components {
		if component.Kind == model.StatusComponentKindModel && component.Lifecycle == model.StatusLifecycleActive {
			publicModels[component.ModelName] = struct{}{}
		}
	}
	unique := make(map[string]struct{}, len(modelNames))
	compatibilityModels := make([]string, 0, len(modelNames))
	for _, modelName := range modelNames {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		if _, public := publicModels[modelName]; public {
			continue
		}
		if _, exists := unique[modelName]; exists {
			continue
		}
		unique[modelName] = struct{}{}
		compatibilityModels = append(compatibilityModels, modelName)
	}
	if len(compatibilityModels) == 0 {
		return nil
	}
	sort.Strings(compatibilityModels)
	states, err := model.GetModelAvailabilityStateMap(compatibilityModels)
	if err != nil {
		return err
	}
	probeSlotSeconds := int64(statusSchedulerLoopInterval / time.Second)
	start := int((now / probeSlotSeconds) % int64(len(compatibilityModels)))
	if start < 0 {
		start += len(compatibilityModels)
	}
	probesRun := 0
	for offset := 0; offset < len(compatibilityModels); offset++ {
		modelName := compatibilityModels[(start+offset)%len(compatibilityModels)]
		if err := leaseKeeper.renewalError(); err != nil {
			return err
		}
		if state, ok := states[modelName]; ok && now-state.LastCheckedAt < statusIdleProbeIntervalSeconds {
			continue
		}
		if probesRun >= budget {
			break
		}
		outcome := scheduler.ModelProbe.ProbeStatusComponent(ctx, model.StatusComponent{
			Kind:            model.StatusComponentKindModel,
			ModelName:       modelName,
			Lifecycle:       model.StatusLifecycleActive,
			LastEvaluatedAt: now,
		})
		probesRun++
		if err := leaseKeeper.renewalError(); err != nil {
			return err
		}
		if outcome.MonitoringFault {
			continue
		}
		if err := scheduler.Availability(statusSchedulerJobName, scheduler.Holder, fencingToken, scheduler.currentTime(), modelName, outcome); err != nil {
			return err
		}
	}
	return nil
}

func (scheduler *StatusScheduler) modelProbeBudget() int {
	if scheduler.ModelProbeBudget > 0 {
		return scheduler.ModelProbeBudget
	}
	return statusDefaultModelProbeBudget
}

func (scheduler *StatusScheduler) statusLeaseSeconds() int64 {
	if scheduler.LeaseSeconds > 0 {
		return scheduler.LeaseSeconds
	}
	return statusSchedulerLeaseSeconds
}

func (scheduler *StatusScheduler) statusLeaseRenewInterval() time.Duration {
	if scheduler.LeaseRenewInterval > 0 {
		return scheduler.LeaseRenewInterval
	}
	return statusSchedulerLeaseRenewInterval
}

func (scheduler *StatusScheduler) statusLeaseRenewer() StatusLeaseRenewer {
	if scheduler.RenewLease != nil {
		return scheduler.RenewLease
	}
	return model.RenewStatusJobLease
}

func (scheduler *StatusScheduler) startStatusLeaseKeeper(ctx context.Context, fencingToken int64, leaseSeconds int64) (context.Context, *statusLeaseKeeper) {
	runCtx, cancel := context.WithCancel(ctx)
	keeper := &statusLeaseKeeper{
		cancel: cancel,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	go func() {
		defer close(keeper.done)
		ticker := time.NewTicker(scheduler.statusLeaseRenewInterval())
		defer ticker.Stop()
		for {
			select {
			case <-keeper.stop:
				return
			case <-runCtx.Done():
				return
			case <-ticker.C:
				renewed, err := scheduler.statusLeaseRenewer()(statusSchedulerJobName, scheduler.Holder, fencingToken, scheduler.currentTime(), leaseSeconds)
				if err != nil || !renewed {
					keeper.errMu.Lock()
					if err != nil {
						keeper.err = fmt.Errorf("status job lease renewal lost: %w", err)
					} else {
						keeper.err = errors.New("status job lease renewal lost")
					}
					keeper.errMu.Unlock()
					cancel()
					return
				}
			}
		}
	}()
	return runCtx, keeper
}

func (keeper *statusLeaseKeeper) renewalError() error {
	keeper.errMu.RLock()
	defer keeper.errMu.RUnlock()
	return keeper.err
}

func (keeper *statusLeaseKeeper) stopKeeping() {
	keeper.stopOnce.Do(func() {
		close(keeper.stop)
		keeper.cancel()
	})
	<-keeper.done
}

func persistStatusProbeWithFence(jobName string, holder string, fencingToken int64, now int64, result *model.StatusProbeResult) error {
	return model.CreateStatusProbeResultWithFence(jobName, holder, fencingToken, now, result)
}

func writeStatusPeriodWithFence(jobName string, holder string, fencingToken int64, now int64, period *model.StatusPeriod) error {
	return model.UpsertStatusPeriodWithFence(jobName, holder, fencingToken, now, period)
}

func writeStatusFiveMinutePeriodWithFence(jobName string, holder string, fencingToken int64, now int64, period *model.StatusPeriod) error {
	existing, err := model.GetStatusPeriodsInRange(model.StatusGranularityFiveMinutes, period.PeriodStart, period.PeriodStart+1)
	if err != nil {
		return err
	}
	for _, current := range existing {
		if current.ComponentID == period.ComponentID {
			*period = mergeStatusFiveMinutePeriod(current, *period)
			break
		}
	}
	return writeStatusPeriodWithFence(jobName, holder, fencingToken, now, period)
}

func mergeStatusFiveMinutePeriod(current model.StatusPeriod, next model.StatusPeriod) model.StatusPeriod {
	current.UpdatedAt = next.UpdatedAt
	if next.EligibleCount == 0 && next.ProbeSuccessCount == 0 && next.ProbeFailureCount == 0 && next.MaintenanceBucketCount == 0 {
		return current
	}
	current.WorstStatus = worseStatus(current.WorstStatus, next.WorstStatus)

	if next.EligibleCount > 0 {
		// The traffic reader returns cumulative totals for the fixed bucket.
		current.EligibleCount = next.EligibleCount
		current.SuccessCount = next.SuccessCount
	}
	current.ProbeSuccessCount += next.ProbeSuccessCount
	current.ProbeFailureCount += next.ProbeFailureCount

	if current.MaintenanceBucketCount > 0 || next.MaintenanceBucketCount > 0 {
		current.ScoreSumMicros = 0
		current.KnownBucketCount = 0
		current.UnknownBucketCount = 0
		current.MaintenanceBucketCount = 1
		return current
	}

	current.MaintenanceBucketCount = 0
	current.UnknownBucketCount = 0
	current.KnownBucketCount = 1
	if current.EligibleCount > 0 {
		current.ScoreSumMicros = ratioMicros(current.SuccessCount, current.EligibleCount)
		return current
	}
	probeCount := current.ProbeSuccessCount + current.ProbeFailureCount
	current.ScoreSumMicros = ratioMicros(current.ProbeSuccessCount, probeCount)
	return current
}

func rollupStatusPeriodsWithFence(jobName string, holder string, fencingToken int64, now int64) error {
	return rollupStatusPeriodsWithClock(jobName, holder, fencingToken, now, func() int64 { return now })
}

func rollupStatusPeriodsWithClock(jobName string, holder string, fencingToken int64, now int64, currentTime func() int64) error {
	fiveMinuteStart, _ := statusFiveMinuteBounds(now - statusAvailabilityFlushGrace)
	return rollupStatusPeriodsForFiveMinuteWithClock(jobName, holder, fencingToken, fiveMinuteStart, now, currentTime)
}

func rollupStatusPeriodsForFiveMinuteWithClock(jobName string, holder string, fencingToken int64, fiveMinuteStart int64, now int64, currentTime func() int64) error {
	if err := model.ValidateStatusJobFence(jobName, holder, fencingToken, currentTime()); err != nil {
		return err
	}
	hourStart := (fiveMinuteStart / statusHourSeconds) * statusHourSeconds
	fiveMinutePeriods, err := model.GetStatusPeriodsInRange(model.StatusGranularityFiveMinutes, hourStart, hourStart+statusHourSeconds)
	if err != nil {
		return err
	}
	for _, period := range aggregateStatusPeriods(fiveMinutePeriods, model.StatusGranularityHour, hourStart, now) {
		period := period
		if err := writeStatusPeriodWithFence(jobName, holder, fencingToken, currentTime(), &period); err != nil {
			return err
		}
	}

	dayStart := (fiveMinuteStart / statusDaySeconds) * statusDaySeconds
	hourPeriods, err := model.GetStatusPeriodsInRange(model.StatusGranularityHour, dayStart, dayStart+statusDaySeconds)
	if err != nil {
		return err
	}
	for _, period := range aggregateStatusPeriods(hourPeriods, model.StatusGranularityDay, dayStart, now) {
		period := period
		if err := writeStatusPeriodWithFence(jobName, holder, fencingToken, currentTime(), &period); err != nil {
			return err
		}
	}
	return nil
}

func (scheduler *StatusScheduler) currentTime() int64 {
	if scheduler.Now != nil {
		return scheduler.Now()
	}
	return model.GetDBTimestamp()
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

func statusEvidenceForComponent(summary model.PerfMetricSummary, probe model.StatusProbeResult, hasProbe bool, consumeProbe bool, conflict bool, trafficBucketStart int64, now int64) StatusEvidence {
	if summary.AvailabilityEligibleCount >= statusTrafficMinimumEligible {
		return StatusEvidence{
			Eligible:           summary.AvailabilityEligibleCount,
			Success:            summary.AvailabilitySuccessCount,
			LastTrustworthyAt:  now,
			TrafficBucketStart: trafficBucketStart,
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
	if !consumeProbe {
		evidence.ProbeSuccess = 0
		evidence.ProbeFailure = 0
	}
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

func statusFiveMinutePeriod(component model.StatusComponent, evidence StatusEvidence, transition StatusTransition, periodStart int64, now int64) model.StatusPeriod {
	period := model.StatusPeriod{
		ComponentID:       component.ID,
		Granularity:       model.StatusGranularityFiveMinutes,
		PeriodStart:       periodStart,
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

func statusFiveMinuteBounds(now int64) (int64, int64) {
	end := (now / statusFiveMinuteSeconds) * statusFiveMinuteSeconds
	return end - statusFiveMinuteSeconds, end
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
	if !common.IsMasterNode {
		return false
	}
	started := false
	if IsStatusCenterEnabled() {
		routerOrigin, err := validateStatusRouterOrigin(common.GetEnvOrDefaultString("ROUTER_ORIGIN", ""))
		if err != nil {
			common.SysError("status center scheduler not started: " + err.Error())
		} else {
			scheduler := newStatusCenterScheduler(routerOrigin)
			statusCenterTaskOnce.Do(func() {
				started = true
				statusCenterTaskLaunch(scheduler)
			})
		}
	}
	if IsStatusCenterNotificationsEnabled() {
		statusDeliveryTaskOnce.Do(func() {
			started = true
			keyring, keyringErr := LoadStatusSecretKeyringFromEnvironment()
			if keyringErr != nil {
				common.SysError("status notification keyring is invalid; webhook and Discord delivery disabled: " + keyringErr.Error())
				keyring, _ = ParseStatusSecretKeyring("", "")
			}
			statusDeliveryTaskLaunch(StatusDeliveryWorker{
				Keyring: keyring,
				Webhook: NewStatusSafeWebhookClient(),
				Now:     model.GetDBTimestamp,
			})
		})
	}
	return started
}

func launchStatusDeliveryTasks(worker StatusDeliveryWorker) {
	gopool.Go(func() {
		workerID := statusSchedulerHolder() + ":delivery"
		run := func() {
			if _, err := worker.RunOnce(context.Background(), workerID, 20); err != nil {
				common.SysError("status delivery worker failed: " + err.Error())
			}
		}
		run()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			run()
		}
	})
}

func validateStatusRouterOrigin(origin string) (string, error) {
	origin = strings.TrimSpace(origin)
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("ROUTER_ORIGIN must be an absolute http(s) origin")
	}
	if parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("ROUTER_ORIGIN must not include credentials, a path, query, or fragment")
	}
	return strings.TrimRight(origin, "/"), nil
}

func newStatusCenterScheduler(routerOrigin string) *StatusScheduler {
	return &StatusScheduler{
		Holder:              statusSchedulerHolder(),
		Pricing:             GetWebsiteVisiblePricing,
		UsableGroups:        WebsitePublicUsableGroups,
		Traffic:             readStatusTraffic,
		CompatibilityModels: model.GetModelAvailabilityProbeModelNames,
		RouterProbe:         NewStatusRouterProbeAdapter(routerOrigin, GetHttpClient()),
		ModelProbe:          configuredStatusModelProbe(),
		Availability:        configuredStatusModelAvailabilityWriter(),
	}
}

func launchStatusCenterTasks(scheduler *StatusScheduler) {
	gopool.Go(func() {
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

func configuredStatusModelAvailabilityWriter() StatusModelAvailabilityWriter {
	statusAvailabilityMu.RLock()
	defer statusAvailabilityMu.RUnlock()
	return statusAvailabilityWriter
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
