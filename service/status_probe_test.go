package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestStatusProbeUsesAdaptiveIntervals(t *testing.T) {
	now := int64(10_000)
	tests := []struct {
		name  string
		input StatusProbeSchedule
		want  bool
	}{
		{name: "idle model waits fifteen minutes", input: StatusProbeSchedule{Kind: model.StatusComponentKindModel, Lifecycle: model.StatusLifecycleActive, LastProbeAt: now - 14*60}, want: false},
		{name: "idle model probes at fifteen minutes", input: StatusProbeSchedule{Kind: model.StatusComponentKindModel, Lifecycle: model.StatusLifecycleActive, LastProbeAt: now - 15*60}, want: true},
		{name: "router canary probes every minute", input: StatusProbeSchedule{Kind: model.StatusComponentKindRouter, Lifecycle: model.StatusLifecycleActive, LastProbeAt: now - 60}, want: true},
		{name: "degraded model retries every minute", input: StatusProbeSchedule{Kind: model.StatusComponentKindModel, Lifecycle: model.StatusLifecycleActive, EffectiveStatus: model.StatusDegraded, LastProbeAt: now - 60}, want: true},
		{name: "outage model retries every minute", input: StatusProbeSchedule{Kind: model.StatusComponentKindModel, Lifecycle: model.StatusLifecycleActive, EffectiveStatus: model.StatusOutage, LastProbeAt: now - 60}, want: true},
		{name: "conflict retries every minute", input: StatusProbeSchedule{Kind: model.StatusComponentKindModel, Lifecycle: model.StatusLifecycleActive, LastProbeAt: now - 60, SignalConflict: true}, want: true},
		{name: "monitoring fault retries every minute", input: StatusProbeSchedule{Kind: model.StatusComponentKindModel, Lifecycle: model.StatusLifecycleActive, LastProbeAt: now - 60, MonitoringFault: true}, want: true},
		{name: "high traffic skips model probes", input: StatusProbeSchedule{Kind: model.StatusComponentKindModel, Lifecycle: model.StatusLifecycleActive, EligibleCount: statusTrafficMinimumEligible}, want: false},
		{name: "retired model never probes", input: StatusProbeSchedule{Kind: model.StatusComponentKindModel, Lifecycle: model.StatusLifecycleRetired}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, StatusProbeDue(tt.input, now))
		})
	}
}

func TestStatusProbeAdapterIsInjectable(t *testing.T) {
	component := model.StatusComponent{ID: 7, Kind: model.StatusComponentKindModel, ModelName: "gpt-test"}
	called := false
	adapter := StatusProbeAdapterFunc(func(_ context.Context, got model.StatusComponent) StatusProbeOutcome {
		called = true
		require.Equal(t, component.ModelName, got.ModelName)
		return StatusProbeOutcome{Success: true, DiagnosticType: "ok", TargetRef: "test-target", LatencyMs: 12}
	})

	outcome := adapter.ProbeStatusComponent(context.Background(), component)
	require.True(t, called)
	require.True(t, outcome.Success)
	require.False(t, outcome.MonitoringFault)
}

func TestStatusProbeMonitoringFaultBecomesUnknownInsteadOfOutage(t *testing.T) {
	previous := statusComponent(model.StatusOperational)
	previous.LastTrustworthyUpdateAt = 700
	outcome := StatusProbeOutcome{MonitoringFault: true, DiagnosticType: "probe_credentials_unavailable"}
	evidence := StatusEvidenceFromProbe(outcome, 2_000)

	require.True(t, evidence.MonitoringFault)
	require.Zero(t, evidence.ProbeFailure)
	require.Zero(t, evidence.LastTrustworthyAt)
	transition := EvaluateStatus(previous, evidence, 2_000)
	require.Equal(t, model.StatusUnknown, transition.Observed)
	require.NotEqual(t, model.StatusOutage, transition.Observed)
	require.Equal(t, "monitoring_fault", transition.Source)
}
