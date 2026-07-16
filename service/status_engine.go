package service

import "github.com/QuantumNous/new-api/model"

const (
	statusTrafficMinimumEligible = int64(20)
	statusOperationalMicros      = int64(995_000)
	statusOutageMicros           = int64(950_000)
	statusRecoveryMicros         = int64(999_000)
	statusEvidenceMaxAgeSeconds  = int64(20 * 60)

	OverallMajorOutage          = "major_outage"
	OverallSomeSystemsAffected  = "some_systems_affected"
	OverallDegradedPerformance  = "degraded_performance"
	OverallMonitoringIncomplete = "monitoring_incomplete"
	OverallMaintenance          = "maintenance"
	OverallOperational          = "all_systems_operational"
)

type StatusEvidence struct {
	Eligible          int64
	Success           int64
	ProbeSuccess      int64
	ProbeFailure      int64
	LastTrustworthyAt int64
	MonitoringFault   bool
	SignalConflict    bool
	MaintenanceActive bool
}

type StatusTransition struct {
	Observed                   string
	Effective                  string
	Source                     string
	ScoreMicros                *int64
	TrustworthyAt              int64
	ConsecutiveProbeFailures   int64
	ConsecutiveProbeSuccesses  int64
	ConsecutiveTrafficRecovery int64
}

type StatusAvailability struct {
	AvailabilityMicros     int64 `json:"availability_micros"`
	CoverageMicros         int64 `json:"coverage_micros"`
	KnownBucketCount       int64 `json:"known_bucket_count"`
	UnknownBucketCount     int64 `json:"unknown_bucket_count"`
	MaintenanceBucketCount int64 `json:"maintenance_bucket_count"`
}

func EvaluateStatus(previous model.StatusComponent, evidence StatusEvidence, now int64) StatusTransition {
	observed := previous.ObservedStatus
	if observed == "" {
		observed = model.StatusUnknown
	}
	transition := StatusTransition{
		Observed:                   observed,
		Effective:                  observed,
		Source:                     "observed",
		TrustworthyAt:              previous.LastTrustworthyUpdateAt,
		ConsecutiveProbeFailures:   previous.ConsecutiveProbeFailures,
		ConsecutiveProbeSuccesses:  previous.ConsecutiveProbeSuccesses,
		ConsecutiveTrafficRecovery: previous.ConsecutiveTrafficRecovery,
	}

	if evidence.Eligible >= statusTrafficMinimumEligible {
		rate := ratioMicros(evidence.Success, evidence.Eligible)
		transition.ScoreMicros = &rate
		transition.Source = "traffic"
		transition.TrustworthyAt = trustworthyTime(evidence.LastTrustworthyAt, now)
		transition.ConsecutiveProbeFailures = 0
		transition.ConsecutiveProbeSuccesses = 0

		switch {
		case rate < statusOutageMicros:
			transition.Observed = model.StatusOutage
			transition.ConsecutiveTrafficRecovery = 0
		case rate < statusOperationalMicros:
			transition.Observed = model.StatusDegraded
			transition.ConsecutiveTrafficRecovery = 0
		default:
			if rate >= statusRecoveryMicros {
				transition.ConsecutiveTrafficRecovery++
			} else {
				transition.ConsecutiveTrafficRecovery = 0
			}
			if isUnhealthyStatus(observed) && transition.ConsecutiveTrafficRecovery < 2 {
				transition.Observed = observed
			} else {
				transition.Observed = model.StatusOperational
			}
		}
	} else if evidence.SignalConflict {
		transition.Observed = model.StatusDegraded
		transition.Source = "conflict"
		transition.TrustworthyAt = trustworthyTime(evidence.LastTrustworthyAt, now)
		transition.ConsecutiveTrafficRecovery = 0
	} else if evidence.ProbeFailure > 0 {
		transition.Source = "probe"
		transition.TrustworthyAt = trustworthyTime(evidence.LastTrustworthyAt, now)
		transition.ConsecutiveProbeFailures += evidence.ProbeFailure
		transition.ConsecutiveProbeSuccesses = 0
		transition.ConsecutiveTrafficRecovery = 0
		score := ratioMicros(evidence.ProbeSuccess, evidence.ProbeSuccess+evidence.ProbeFailure)
		transition.ScoreMicros = &score
		switch {
		case transition.ConsecutiveProbeFailures >= 3:
			transition.Observed = model.StatusOutage
		case transition.ConsecutiveProbeFailures >= 2:
			transition.Observed = model.StatusDegraded
		}
	} else if evidence.ProbeSuccess > 0 {
		transition.Source = "probe"
		transition.TrustworthyAt = trustworthyTime(evidence.LastTrustworthyAt, now)
		transition.ConsecutiveProbeFailures = 0
		transition.ConsecutiveProbeSuccesses += evidence.ProbeSuccess
		transition.ConsecutiveTrafficRecovery = 0
		score := int64(1_000_000)
		transition.ScoreMicros = &score
		if !isUnhealthyStatus(observed) || transition.ConsecutiveProbeSuccesses >= 3 {
			transition.Observed = model.StatusOperational
		}
	} else {
		if evidence.MonitoringFault {
			transition.Source = "monitoring_fault"
		}
		lastTrustworthy := evidence.LastTrustworthyAt
		if lastTrustworthy == 0 {
			lastTrustworthy = previous.LastTrustworthyUpdateAt
		}
		transition.TrustworthyAt = lastTrustworthy
		if lastTrustworthy == 0 || now-lastTrustworthy >= statusEvidenceMaxAgeSeconds {
			transition.Observed = model.StatusUnknown
		}
	}

	transition.Effective = transition.Observed
	if evidence.MaintenanceActive {
		transition.Effective = model.StatusMaintenance
		transition.Source = "maintenance"
	} else if previous.OverrideStatus != "" && previous.OverrideExpiresAt > now {
		transition.Effective = previous.OverrideStatus
		transition.Source = "override"
	}
	return transition
}

func AggregateStatusPeriods(periods []model.StatusPeriod) StatusAvailability {
	var result StatusAvailability
	var scoreSum int64
	for _, period := range periods {
		scoreSum += period.ScoreSumMicros
		result.KnownBucketCount += period.KnownBucketCount
		result.UnknownBucketCount += period.UnknownBucketCount
		result.MaintenanceBucketCount += period.MaintenanceBucketCount
	}
	if result.KnownBucketCount > 0 {
		result.AvailabilityMicros = scoreSum / result.KnownBucketCount
	}
	coverageDenominator := result.KnownBucketCount + result.UnknownBucketCount
	if coverageDenominator > 0 {
		result.CoverageMicros = result.KnownBucketCount * 1_000_000 / coverageDenominator
	}
	return result
}

func OverallStatus(components []model.StatusComponent) string {
	if len(components) == 0 {
		return OverallMonitoringIncomplete
	}
	modelCount := 0
	affectedModels := 0
	hasDegraded := false
	hasUnknown := false
	hasMaintenance := false
	for _, component := range components {
		if component.Lifecycle == model.StatusLifecycleRetired {
			continue
		}
		if component.Kind == model.StatusComponentKindRouter && component.EffectiveStatus == model.StatusOutage {
			return OverallMajorOutage
		}
		if component.Kind == model.StatusComponentKindModel {
			modelCount++
			if component.EffectiveStatus == model.StatusOutage {
				return OverallSomeSystemsAffected
			}
			if component.EffectiveStatus == model.StatusDegraded {
				affectedModels++
			}
		}
		switch component.EffectiveStatus {
		case model.StatusDegraded:
			hasDegraded = true
		case model.StatusUnknown:
			hasUnknown = true
		case model.StatusMaintenance:
			hasMaintenance = true
		}
	}
	if modelCount > 0 && affectedModels*5 >= modelCount {
		return OverallSomeSystemsAffected
	}
	if hasDegraded {
		return OverallDegradedPerformance
	}
	if hasUnknown {
		return OverallMonitoringIncomplete
	}
	if hasMaintenance {
		return OverallMaintenance
	}
	return OverallOperational
}

func ratioMicros(numerator int64, denominator int64) int64 {
	if denominator <= 0 || numerator <= 0 {
		return 0
	}
	if numerator >= denominator {
		return 1_000_000
	}
	return numerator * 1_000_000 / denominator
}

func trustworthyTime(candidate int64, fallback int64) int64 {
	if candidate > 0 {
		return candidate
	}
	return fallback
}

func isUnhealthyStatus(status string) bool {
	return status == model.StatusDegraded || status == model.StatusOutage || status == model.StatusUnknown
}
