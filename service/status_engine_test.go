package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func statusComponent(status string) model.StatusComponent {
	return model.StatusComponent{
		ObservedStatus:          status,
		EffectiveStatus:         status,
		LastTrustworthyUpdateAt: 1_000,
	}
}

func TestStatusEngineTrafficThresholds(t *testing.T) {
	tests := []struct {
		name            string
		eligible        int64
		success         int64
		want            string
		wantScoreMicros int64
	}{
		{name: "operational boundary", eligible: 200, success: 199, want: model.StatusOperational, wantScoreMicros: 995_000},
		{name: "degraded", eligible: 200, success: 198, want: model.StatusDegraded, wantScoreMicros: 990_000},
		{name: "outage", eligible: 20, success: 18, want: model.StatusOutage, wantScoreMicros: 900_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transition := EvaluateStatus(statusComponent(model.StatusOperational), StatusEvidence{Eligible: tt.eligible, Success: tt.success, LastTrustworthyAt: 2_000}, 2_000)
			require.Equal(t, tt.want, transition.Observed)
			require.Equal(t, tt.want, transition.Effective)
			require.NotNil(t, transition.ScoreMicros)
			require.Equal(t, tt.wantScoreMicros, *transition.ScoreMicros)
			require.Equal(t, "traffic", transition.Source)
		})
	}
}

func TestStatusEngineProbeFailureAndRecoveryHysteresis(t *testing.T) {
	first := EvaluateStatus(statusComponent(model.StatusOperational), StatusEvidence{ProbeFailure: 1, LastTrustworthyAt: 2_000}, 2_000)
	require.Equal(t, model.StatusOperational, first.Observed)
	require.EqualValues(t, 1, first.ConsecutiveProbeFailures)

	previous := statusComponent(first.Observed)
	previous.ConsecutiveProbeFailures = first.ConsecutiveProbeFailures
	second := EvaluateStatus(previous, StatusEvidence{ProbeFailure: 1, LastTrustworthyAt: 2_060}, 2_060)
	require.Equal(t, model.StatusDegraded, second.Observed)
	require.EqualValues(t, 2, second.ConsecutiveProbeFailures)

	previous = statusComponent(second.Observed)
	previous.ConsecutiveProbeFailures = second.ConsecutiveProbeFailures
	third := EvaluateStatus(previous, StatusEvidence{ProbeFailure: 1, LastTrustworthyAt: 2_120}, 2_120)
	require.Equal(t, model.StatusOutage, third.Observed)
	require.EqualValues(t, 3, third.ConsecutiveProbeFailures)

	previous = statusComponent(model.StatusOutage)
	previous.ConsecutiveProbeSuccesses = 2
	recovered := EvaluateStatus(previous, StatusEvidence{ProbeSuccess: 1, LastTrustworthyAt: 2_180}, 2_180)
	require.Equal(t, model.StatusOperational, recovered.Observed)
	require.EqualValues(t, 3, recovered.ConsecutiveProbeSuccesses)
}

func TestStatusEngineTrafficRecoveryRequiresTwoHealthyBuckets(t *testing.T) {
	previous := statusComponent(model.StatusOutage)
	first := EvaluateStatus(previous, StatusEvidence{Eligible: 1_000, Success: 999, LastTrustworthyAt: 2_000, TrafficBucketStart: 1_800}, 2_000)
	require.Equal(t, model.StatusOutage, first.Observed)
	require.EqualValues(t, 1, first.ConsecutiveTrafficRecovery)

	previous.ConsecutiveTrafficRecovery = first.ConsecutiveTrafficRecovery
	previous.LastTrafficBucketStart = first.LastTrafficBucketStart
	second := EvaluateStatus(previous, StatusEvidence{Eligible: 1_000, Success: 1_000, LastTrustworthyAt: 2_300, TrafficBucketStart: 2_100}, 2_300)
	require.Equal(t, model.StatusOperational, second.Observed)
	require.EqualValues(t, 2, second.ConsecutiveTrafficRecovery)
}

func TestStatusEngineUnknownConflictAndMonitoringFault(t *testing.T) {
	previous := statusComponent(model.StatusOperational)
	previous.LastTrustworthyUpdateAt = 700
	stale := EvaluateStatus(previous, StatusEvidence{}, 2_000)
	require.Equal(t, model.StatusUnknown, stale.Observed)

	previous.LastTrustworthyUpdateAt = 1_900
	fault := EvaluateStatus(previous, StatusEvidence{MonitoringFault: true}, 2_000)
	require.Equal(t, model.StatusOperational, fault.Observed)
	require.Zero(t, fault.ConsecutiveProbeFailures)

	conflict := EvaluateStatus(previous, StatusEvidence{SignalConflict: true, LastTrustworthyAt: 2_000}, 2_000)
	require.Equal(t, model.StatusDegraded, conflict.Observed)
}

func TestStatusEngineEffectivePrecedence(t *testing.T) {
	previous := statusComponent(model.StatusOutage)
	previous.OverrideStatus = model.StatusUnknown
	previous.OverrideExpiresAt = 3_000

	overridden := EvaluateStatus(previous, StatusEvidence{LastTrustworthyAt: 2_000}, 2_000)
	require.Equal(t, model.StatusUnknown, overridden.Effective)
	require.Equal(t, "override", overridden.Source)

	maintenance := EvaluateStatus(previous, StatusEvidence{MaintenanceActive: true, LastTrustworthyAt: 2_000}, 2_000)
	require.Equal(t, model.StatusMaintenance, maintenance.Effective)
	require.Equal(t, "maintenance", maintenance.Source)

	expired := EvaluateStatus(previous, StatusEvidence{LastTrustworthyAt: 3_100}, 3_100)
	require.Equal(t, model.StatusOutage, expired.Effective)
}

func TestStatusAvailabilityExcludesUnknownAndMaintenance(t *testing.T) {
	got := AggregateStatusPeriods([]model.StatusPeriod{
		{ScoreSumMicros: 1_800_000, KnownBucketCount: 2, UnknownBucketCount: 1, MaintenanceBucketCount: 1},
		{ScoreSumMicros: 1_000_000, KnownBucketCount: 1, UnknownBucketCount: 1},
	})
	require.EqualValues(t, 933_333, got.AvailabilityMicros)
	require.EqualValues(t, 600_000, got.CoverageMicros)
	require.EqualValues(t, 3, got.KnownBucketCount)
	require.EqualValues(t, 2, got.UnknownBucketCount)
	require.EqualValues(t, 1, got.MaintenanceBucketCount)
}

func TestOverallStatusPriority(t *testing.T) {
	require.Equal(t, OverallMajorOutage, OverallStatus([]model.StatusComponent{{Kind: model.StatusComponentKindRouter, EffectiveStatus: model.StatusOutage}}))
	require.Equal(t, OverallSomeSystemsAffected, OverallStatus([]model.StatusComponent{{Kind: model.StatusComponentKindModel, EffectiveStatus: model.StatusOutage}}))
	require.Equal(t, OverallDegradedPerformance, OverallStatus([]model.StatusComponent{
		{Kind: model.StatusComponentKindModel, EffectiveStatus: model.StatusDegraded},
		{Kind: model.StatusComponentKindModel, EffectiveStatus: model.StatusOperational},
		{Kind: model.StatusComponentKindModel, EffectiveStatus: model.StatusOperational},
		{Kind: model.StatusComponentKindModel, EffectiveStatus: model.StatusOperational},
		{Kind: model.StatusComponentKindModel, EffectiveStatus: model.StatusOperational},
		{Kind: model.StatusComponentKindModel, EffectiveStatus: model.StatusOperational},
	}))
	require.Equal(t, OverallMonitoringIncomplete, OverallStatus([]model.StatusComponent{{Kind: model.StatusComponentKindModel, EffectiveStatus: model.StatusUnknown}}))
	require.Equal(t, OverallMaintenance, OverallStatus([]model.StatusComponent{{Kind: model.StatusComponentKindModel, EffectiveStatus: model.StatusMaintenance}}))
	require.Equal(t, OverallOperational, OverallStatus([]model.StatusComponent{{Kind: model.StatusComponentKindModel, EffectiveStatus: model.StatusOperational}}))
}
