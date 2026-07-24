package service

import (
	"context"

	"github.com/QuantumNous/new-api/model"
)

const (
	statusIdleProbeIntervalSeconds = int64(15 * 60)
	statusFastProbeIntervalSeconds = int64(60)
)

type StatusProbeSchedule struct {
	Kind            string
	Lifecycle       string
	EffectiveStatus string
	EligibleCount   int64
	LastProbeAt     int64
	SignalConflict  bool
	MonitoringFault bool
}

type StatusProbeOutcome struct {
	Success         bool
	MonitoringFault bool
	DiagnosticType  string
	TargetRef       string
	LatencyMs       int64
}

type StatusProbeAdapter interface {
	ProbeStatusComponent(ctx context.Context, component model.StatusComponent) StatusProbeOutcome
}

type StatusProbeAdapterFunc func(ctx context.Context, component model.StatusComponent) StatusProbeOutcome

func (f StatusProbeAdapterFunc) ProbeStatusComponent(ctx context.Context, component model.StatusComponent) StatusProbeOutcome {
	return f(ctx, component)
}

func StatusProbeDue(schedule StatusProbeSchedule, now int64) bool {
	if schedule.Lifecycle != model.StatusLifecycleActive {
		return false
	}
	if schedule.Kind == model.StatusComponentKindModel && schedule.EligibleCount >= statusTrafficMinimumEligible {
		return false
	}

	interval := statusIdleProbeIntervalSeconds
	if schedule.Kind == model.StatusComponentKindRouter ||
		schedule.EffectiveStatus == model.StatusDegraded ||
		schedule.EffectiveStatus == model.StatusOutage ||
		schedule.SignalConflict ||
		schedule.MonitoringFault {
		interval = statusFastProbeIntervalSeconds
	}
	return schedule.LastProbeAt <= 0 || now-schedule.LastProbeAt >= interval
}

func StatusEvidenceFromProbe(outcome StatusProbeOutcome, observedAt int64) StatusEvidence {
	if outcome.MonitoringFault {
		return StatusEvidence{MonitoringFault: true}
	}
	if outcome.Success {
		return StatusEvidence{ProbeSuccess: 1, LastTrustworthyAt: observedAt}
	}
	return StatusEvidence{ProbeFailure: 1, LastTrustworthyAt: observedAt}
}
