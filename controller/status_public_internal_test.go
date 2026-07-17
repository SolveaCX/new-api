package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestPublicStatusFreshnessUsesExactBoundaryIndependentOfGeneratedAtBucket(t *testing.T) {
	now := int64(1_700_000_029)
	require.Equal(t, now-now%statusPublicCacheSeconds, statusGeneratedAt(now))

	for _, testCase := range []struct {
		name             string
		age              int64
		expectedStatus   string
		expectedCoverage int64
	}{
		{name: "1199_seconds_is_fresh", age: statusPublicEvidenceMaxAge - 1, expectedStatus: model.StatusOperational, expectedCoverage: 1_000_000},
		{name: "1200_seconds_is_stale", age: statusPublicEvidenceMaxAge, expectedStatus: model.StatusUnknown, expectedCoverage: 0},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			components, honest := publicStatusComponents([]model.StatusComponent{{
				ID: 1, Slug: "router", Kind: model.StatusComponentKindRouter, Lifecycle: model.StatusLifecycleActive,
				EffectiveStatus: model.StatusOperational, LastTrustworthyUpdateAt: now - testCase.age, CoverageMicros: 1_000_000,
			}}, now)
			require.Equal(t, testCase.expectedStatus, components[0].Status)
			require.Equal(t, testCase.expectedCoverage, components[0].Coverage)
			require.Equal(t, testCase.expectedStatus, honest[0].EffectiveStatus)
			require.Equal(t, testCase.expectedCoverage, honest[0].CoverageMicros)
		})
	}
}
