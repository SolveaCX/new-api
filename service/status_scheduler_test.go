package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestStatusSchedulerCompetingWorkersOnlyLeaseOwnerCommits(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	var routerCalls atomic.Int64
	newScheduler := func(holder string) *StatusScheduler {
		return &StatusScheduler{
			Holder:       holder,
			Now:          func() int64 { return 10_000 },
			Pricing:      func() []model.Pricing { return nil },
			UsableGroups: func() map[string]string { return map[string]string{"public": "Public"} },
			Traffic:      func(_ int64, _ int64, _ []string) ([]model.PerfMetricSummary, error) { return nil, nil },
			RouterProbe: StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
				routerCalls.Add(1)
				return StatusProbeOutcome{Success: true, DiagnosticType: "ok"}
			}),
			ModelProbe: StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
				return StatusProbeOutcome{MonitoringFault: true, DiagnosticType: "unexpected_model_probe"}
			}),
		}
	}

	type result struct {
		ran bool
		err error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, holder := range []string{"node-a", "node-b"} {
		wg.Add(1)
		go func(holder string) {
			defer wg.Done()
			ran, runErr := newScheduler(holder).RunOnce(context.Background(), 10_000)
			results <- result{ran: ran, err: runErr}
		}(holder)
	}
	wg.Wait()
	close(results)

	acquired := 0
	for result := range results {
		require.NoError(t, result.err)
		if result.ran {
			acquired++
		}
	}
	require.Equal(t, 1, acquired)
	require.EqualValues(t, 1, routerCalls.Load())
	require.EqualValues(t, 1, countRows(t, db, &model.StatusComponent{}))
	require.EqualValues(t, 1, countRows(t, db, &model.StatusProbeResult{}))
	require.EqualValues(t, 1, countStatusPeriodsByGranularity(t, db, model.StatusGranularityFiveMinutes))
}

func TestStatusSchedulerProbeIsConsumedOnlyOnce(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	var modelProbeCalls atomic.Int64
	currentTime := int64(1_000)
	scheduler := &StatusScheduler{
		Holder: "node-a",
		Now:    func() int64 { return currentTime },
		Pricing: func() []model.Pricing {
			return []model.Pricing{{ModelName: "gpt-test", EnableGroup: []string{"public"}}}
		},
		UsableGroups: func() map[string]string { return map[string]string{"public": "Public"} },
		Traffic:      func(_ int64, _ int64, _ []string) ([]model.PerfMetricSummary, error) { return nil, nil },
		RouterProbe: StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
			return StatusProbeOutcome{Success: true, DiagnosticType: "ok"}
		}),
		ModelProbe: StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
			modelProbeCalls.Add(1)
			return StatusProbeOutcome{DiagnosticType: "upstream_failure"}
		}),
	}

	for _, now := range []int64{1_000, 1_060, 1_120} {
		currentTime = now
		ran, err := scheduler.RunOnce(context.Background(), now)
		require.NoError(t, err)
		require.True(t, ran)
	}

	var component model.StatusComponent
	require.NoError(t, db.Where("model_name = ?", "gpt-test").First(&component).Error)
	require.EqualValues(t, 1, modelProbeCalls.Load())
	require.EqualValues(t, 1, component.ConsecutiveProbeFailures)
	require.EqualValues(t, 0, component.ConsecutiveProbeSuccesses)
	require.Equal(t, model.StatusUnknown, component.ObservedStatus)
	var probeCount int64
	require.NoError(t, db.Model(&model.StatusProbeResult{}).Where("component_id = ?", component.ID).Count(&probeCount).Error)
	require.EqualValues(t, 1, probeCount)
}

func TestStatusSchedulerProjectsHighTrafficModelSuccess(t *testing.T) {
	setupStatusServiceTestDB(t)
	currentTime := int64(1_000)
	type projection struct {
		modelName string
		outcome   StatusProbeOutcome
	}
	projections := make([]projection, 0, 1)
	scheduler := &StatusScheduler{
		Holder: "node-a",
		Now:    func() int64 { return currentTime },
		Pricing: func() []model.Pricing {
			return []model.Pricing{{ModelName: "gpt-test", EnableGroup: []string{"public"}}}
		},
		UsableGroups: func() map[string]string { return map[string]string{"public": "Public"} },
		Traffic: func(_ int64, _ int64, _ []string) ([]model.PerfMetricSummary, error) {
			return []model.PerfMetricSummary{{ModelName: "gpt-test", AvailabilityEligibleCount: 20, AvailabilitySuccessCount: 20}}, nil
		},
		RouterProbe: StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
			return StatusProbeOutcome{Success: true, DiagnosticType: "ok"}
		}),
		ModelProbe: StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
			t.Fatal("high-traffic models must not run a synthetic probe")
			return StatusProbeOutcome{}
		}),
		Availability: func(jobName string, holder string, fencingToken int64, now int64, modelName string, outcome StatusProbeOutcome) error {
			require.Equal(t, statusSchedulerJobName, jobName)
			require.Equal(t, "node-a", holder)
			require.EqualValues(t, 1, fencingToken)
			require.EqualValues(t, currentTime, now)
			projections = append(projections, projection{modelName: modelName, outcome: outcome})
			return nil
		},
	}

	ran, err := scheduler.RunOnce(context.Background(), currentTime)
	require.NoError(t, err)
	require.True(t, ran)
	require.Equal(t, []projection{{modelName: "gpt-test", outcome: StatusProbeOutcome{Success: true, DiagnosticType: "ok"}}}, projections)
}

func TestStatusSchedulerProjectsHighTrafficModelFailureAsTemporaryEvidence(t *testing.T) {
	setupStatusServiceTestDB(t)
	currentTime := int64(1_000)
	var projected []StatusProbeOutcome
	scheduler := &StatusScheduler{
		Holder: "node-a",
		Now:    func() int64 { return currentTime },
		Pricing: func() []model.Pricing {
			return []model.Pricing{{ModelName: "gpt-test", EnableGroup: []string{"public"}}}
		},
		UsableGroups: func() map[string]string { return map[string]string{"public": "Public"} },
		Traffic: func(_ int64, _ int64, _ []string) ([]model.PerfMetricSummary, error) {
			return []model.PerfMetricSummary{{ModelName: "gpt-test", AvailabilityEligibleCount: 20, AvailabilitySuccessCount: 18}}, nil
		},
		RouterProbe: StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
			return StatusProbeOutcome{Success: true, DiagnosticType: "ok"}
		}),
		ModelProbe: StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
			t.Fatal("high-traffic models must not run a synthetic probe")
			return StatusProbeOutcome{}
		}),
		Availability: func(_ string, _ string, _ int64, _ int64, _ string, outcome StatusProbeOutcome) error {
			projected = append(projected, outcome)
			return nil
		},
	}

	ran, err := scheduler.RunOnce(context.Background(), currentTime)
	require.NoError(t, err)
	require.True(t, ran)
	require.Equal(t, []StatusProbeOutcome{{DiagnosticType: "traffic_failure"}}, projected)
}

func TestStatusSchedulerDoesNotProjectMonitoringFaults(t *testing.T) {
	setupStatusServiceTestDB(t)
	currentTime := int64(1_000)
	var projections atomic.Int64
	scheduler := &StatusScheduler{
		Holder: "node-a",
		Now:    func() int64 { return currentTime },
		Pricing: func() []model.Pricing {
			return []model.Pricing{{ModelName: "gpt-test", EnableGroup: []string{"public"}}}
		},
		UsableGroups: func() map[string]string { return map[string]string{"public": "Public"} },
		Traffic:      func(_ int64, _ int64, _ []string) ([]model.PerfMetricSummary, error) { return nil, nil },
		RouterProbe: StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
			return StatusProbeOutcome{Success: true, DiagnosticType: "ok"}
		}),
		ModelProbe: StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
			return StatusProbeOutcome{MonitoringFault: true, DiagnosticType: "probe_user_unavailable"}
		}),
		Availability: func(_ string, _ string, _ int64, _ int64, _ string, _ StatusProbeOutcome) error {
			projections.Add(1)
			return nil
		},
	}

	ran, err := scheduler.RunOnce(context.Background(), currentTime)
	require.NoError(t, err)
	require.True(t, ran)
	require.EqualValues(t, 0, projections.Load())
}

func TestStatusSchedulerProjectsTrustworthyFailureAfterPersistingProbe(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	currentTime := int64(1_000)
	var projected []StatusProbeOutcome
	scheduler := &StatusScheduler{
		Holder: "node-a",
		Now:    func() int64 { return currentTime },
		Pricing: func() []model.Pricing {
			return []model.Pricing{{ModelName: "gpt-test", EnableGroup: []string{"public"}}}
		},
		UsableGroups: func() map[string]string { return map[string]string{"public": "Public"} },
		Traffic:      func(_ int64, _ int64, _ []string) ([]model.PerfMetricSummary, error) { return nil, nil },
		RouterProbe: StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
			return StatusProbeOutcome{Success: true, DiagnosticType: "ok"}
		}),
		ModelProbe: StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
			return StatusProbeOutcome{DiagnosticType: "official_model_unsupported", TargetRef: "channel:7"}
		}),
		Availability: func(_ string, _ string, _ int64, _ int64, modelName string, outcome StatusProbeOutcome) error {
			var component model.StatusComponent
			require.NoError(t, db.Where("model_name = ?", modelName).First(&component).Error)
			var probeCount int64
			require.NoError(t, db.Model(&model.StatusProbeResult{}).Where("component_id = ?", component.ID).Count(&probeCount).Error)
			require.EqualValues(t, 1, probeCount)
			projected = append(projected, outcome)
			return nil
		},
	}

	ran, err := scheduler.RunOnce(context.Background(), currentTime)
	require.NoError(t, err)
	require.True(t, ran)
	require.Equal(t, []StatusProbeOutcome{{DiagnosticType: "official_model_unsupported", TargetRef: "channel:7"}}, projected)
}

func TestStatusSchedulerCompatibilityProbesNonPublicModelsWithoutDuplicatingPublicModels(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.ModelAvailabilityState{}))
	currentTime := int64(1_000)
	probeCalls := make(map[string]int)
	scheduler := &StatusScheduler{
		Holder: "node-a",
		Now:    func() int64 { return currentTime },
		Pricing: func() []model.Pricing {
			return []model.Pricing{{ModelName: "gpt-public", EnableGroup: []string{"public"}}}
		},
		UsableGroups: func() map[string]string { return map[string]string{"public": "Public"} },
		Traffic:      func(_ int64, _ int64, _ []string) ([]model.PerfMetricSummary, error) { return nil, nil },
		CompatibilityModels: func() ([]string, error) {
			return []string{"gpt-private", "gpt-public", "gpt-private"}, nil
		},
		RouterProbe: StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
			return StatusProbeOutcome{Success: true, DiagnosticType: "ok"}
		}),
		ModelProbe: StatusProbeAdapterFunc(func(_ context.Context, component model.StatusComponent) StatusProbeOutcome {
			probeCalls[component.ModelName]++
			return StatusProbeOutcome{DiagnosticType: "official_model_unsupported"}
		}),
		Availability: func(jobName string, holder string, fencingToken int64, now int64, modelName string, outcome StatusProbeOutcome) error {
			status := model.ModelAvailabilityOfficialUnsupported
			if outcome.Success {
				status = model.ModelAvailabilityAvailable
			}
			return model.SaveModelAvailabilityStateWithFence(jobName, holder, fencingToken, now, &model.ModelAvailabilityState{
				ModelName:     modelName,
				Status:        status,
				ReasonType:    outcome.DiagnosticType,
				LastCheckedAt: now,
			})
		},
	}

	for _, now := range []int64{1_000, 1_060} {
		currentTime = now
		ran, err := scheduler.RunOnce(context.Background(), now)
		require.NoError(t, err)
		require.True(t, ran)
	}

	models := make([]string, 0, len(probeCalls))
	for modelName := range probeCalls {
		models = append(models, modelName)
	}
	sort.Strings(models)
	require.Equal(t, []string{"gpt-private", "gpt-public"}, models)
	require.Equal(t, map[string]int{"gpt-private": 1, "gpt-public": 1}, probeCalls)
	privateState, err := model.GetModelAvailabilityState("gpt-private")
	require.NoError(t, err)
	require.Equal(t, model.ModelAvailabilityOfficialUnsupported, privateState.Status)
}

func TestStatusSchedulerSensitiveWritesRejectStaleFence(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	first := acquireStatusServiceLease(t, statusSchedulerJobName, "node-a", 100)
	component := model.StatusComponent{ComponentKey: "router", Slug: "router", Kind: model.StatusComponentKindRouter, DisplayName: "Router", Lifecycle: model.StatusLifecycleActive, ObservedStatus: model.StatusUnknown, EffectiveStatus: model.StatusUnknown, Version: 1}
	require.NoError(t, db.Create(&component).Error)
	_, acquired, err := model.AcquireStatusJobLease(statusSchedulerJobName, "node-b", 161, 60)
	require.NoError(t, err)
	require.True(t, acquired)

	t.Run("catalog", func(t *testing.T) {
		err := SyncStatusCatalog(statusSchedulerJobName, "node-a", first.FencingToken, 161, nil, map[string]string{"public": "Public"})
		require.Error(t, err)
	})
	t.Run("probe", func(t *testing.T) {
		err := persistStatusProbeWithFence(statusSchedulerJobName, "node-a", first.FencingToken, 161, &model.StatusProbeResult{ComponentID: component.ID, CreatedAt: 161})
		require.Error(t, err)
	})
	t.Run("evaluate", func(t *testing.T) {
		component.DisplayName = "stale"
		err := model.CommitStatusComponentWithFence(statusSchedulerJobName, "node-a", first.FencingToken, 161, &component)
		require.Error(t, err)
	})
	t.Run("period", func(t *testing.T) {
		err := writeStatusPeriodWithFence(statusSchedulerJobName, "node-a", first.FencingToken, 161, &model.StatusPeriod{ComponentID: component.ID, Granularity: model.StatusGranularityFiveMinutes, PeriodStart: 0})
		require.Error(t, err)
	})
	t.Run("rollup", func(t *testing.T) {
		err := rollupStatusPeriodsWithFence(statusSchedulerJobName, "node-a", first.FencingToken, 161)
		require.Error(t, err)
	})

	require.EqualValues(t, 0, countRows(t, db, &model.StatusProbeResult{}))
	require.EqualValues(t, 0, countRows(t, db, &model.StatusPeriod{}))
}

func TestStatusSchedulerLeaseExpiryRejectsLateWritesWithoutTakeover(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	currentTime := int64(2_000)
	scheduler := &StatusScheduler{
		Holder:       "node-a",
		Now:          func() int64 { return currentTime },
		Pricing:      func() []model.Pricing { return nil },
		UsableGroups: func() map[string]string { return map[string]string{"public": "Public"} },
		Traffic: func(_ int64, _ int64, _ []string) ([]model.PerfMetricSummary, error) {
			currentTime += statusSchedulerLeaseSeconds + 1
			return nil, nil
		},
		RouterProbe: StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
			return StatusProbeOutcome{Success: true, DiagnosticType: "ok"}
		}),
	}

	ran, err := scheduler.RunOnce(context.Background(), 2_000)
	require.True(t, ran)
	require.ErrorContains(t, err, "status job lease is no longer owned")
	require.EqualValues(t, 0, countRows(t, db, &model.StatusProbeResult{}))
	require.EqualValues(t, 0, countRows(t, db, &model.StatusPeriod{}))
	var lease model.StatusJobLease
	require.NoError(t, db.Where("name = ?", statusSchedulerJobName).First(&lease).Error)
	require.Equal(t, "node-a", lease.Holder)
	require.EqualValues(t, 1, lease.FencingToken)
}

func TestStatusSchedulerRenewsLeaseAcrossSlowProbesAndReachesLaterModelsAndRollups(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	currentTime := int64(10_000)
	var renewalTime atomic.Int64
	renewalTime.Store(currentTime)
	var laterProbeCalls atomic.Int64
	scheduler := &StatusScheduler{
		Holder:             "node-a",
		Now:                func() int64 { return currentTime },
		LeaseSeconds:       2,
		LeaseRenewInterval: 10 * time.Millisecond,
		RenewLease: func(name string, holder string, fencingToken int64, _ int64, leaseSeconds int64) (bool, error) {
			return model.RenewStatusJobLease(name, holder, fencingToken, renewalTime.Add(1), leaseSeconds)
		},
		Pricing: func() []model.Pricing {
			return []model.Pricing{
				{ModelName: "a-slow", EnableGroup: []string{"public"}},
				{ModelName: "z-later", EnableGroup: []string{"public"}},
			}
		},
		UsableGroups: func() map[string]string { return map[string]string{"public": "Public"} },
		Traffic:      func(_ int64, _ int64, _ []string) ([]model.PerfMetricSummary, error) { return nil, nil },
		RouterProbe: StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
			return StatusProbeOutcome{Success: true, DiagnosticType: "ok"}
		}),
		ModelProbe: StatusProbeAdapterFunc(func(_ context.Context, component model.StatusComponent) StatusProbeOutcome {
			if component.ModelName == "a-slow" {
				time.Sleep(80 * time.Millisecond)
			}
			if component.ModelName == "z-later" {
				laterProbeCalls.Add(1)
			}
			return StatusProbeOutcome{Success: true, DiagnosticType: "ok"}
		}),
	}

	ran, err := scheduler.RunOnce(context.Background(), currentTime)
	require.NoError(t, err)
	require.True(t, ran)
	require.EqualValues(t, 1, laterProbeCalls.Load())
	require.EqualValues(t, 3, countStatusPeriodsByGranularity(t, db, model.StatusGranularityFiveMinutes))
	require.EqualValues(t, 3, countStatusPeriodsByGranularity(t, db, model.StatusGranularityHour))
	require.EqualValues(t, 3, countStatusPeriodsByGranularity(t, db, model.StatusGranularityDay))
}

func TestStatusSchedulerLostLeaseRenewalStopsProbeWrites(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	scheduler := &StatusScheduler{
		Holder:             "node-a",
		Now:                func() int64 { return 10_000 },
		LeaseSeconds:       2,
		LeaseRenewInterval: 10 * time.Millisecond,
		RenewLease: func(_ string, _ string, _ int64, _ int64, _ int64) (bool, error) {
			return false, nil
		},
		Pricing:      func() []model.Pricing { return nil },
		UsableGroups: func() map[string]string { return map[string]string{"public": "Public"} },
		Traffic:      func(_ int64, _ int64, _ []string) ([]model.PerfMetricSummary, error) { return nil, nil },
		RouterProbe: StatusProbeAdapterFunc(func(ctx context.Context, _ model.StatusComponent) StatusProbeOutcome {
			<-ctx.Done()
			return StatusProbeOutcome{Success: true, DiagnosticType: "ok"}
		}),
	}

	ran, err := scheduler.RunOnce(context.Background(), 10_000)
	require.True(t, ran)
	require.ErrorContains(t, err, "status job lease renewal lost")
	require.EqualValues(t, 0, countRows(t, db, &model.StatusProbeResult{}))
	require.EqualValues(t, 0, countRows(t, db, &model.StatusPeriod{}))
}

func TestStatusSchedulerBucketUsesAlignedWindowAndKeepsWorstStatus(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	currentTime := int64(610)
	windows := make([][2]int64, 0, 3)
	trafficCall := 0
	scheduler := &StatusScheduler{
		Holder: "node-a",
		Now:    func() int64 { return currentTime },
		Pricing: func() []model.Pricing {
			return []model.Pricing{{ModelName: "gpt-test", EnableGroup: []string{"public"}}}
		},
		UsableGroups: func() map[string]string { return map[string]string{"public": "Public"} },
		Traffic: func(start int64, end int64, _ []string) ([]model.PerfMetricSummary, error) {
			windows = append(windows, [2]int64{start, end})
			trafficCall++
			successes := int64(99)
			if trafficCall > 1 {
				successes = 100
			}
			return []model.PerfMetricSummary{{ModelName: "gpt-test", AvailabilityEligibleCount: 100, AvailabilitySuccessCount: successes}}, nil
		},
		RouterProbe: StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
			return StatusProbeOutcome{Success: true, DiagnosticType: "ok"}
		}),
	}

	for _, now := range []int64{610, 670, 730} {
		currentTime = now
		ran, err := scheduler.RunOnce(context.Background(), now)
		require.NoError(t, err)
		require.True(t, ran)
	}

	var component model.StatusComponent
	require.NoError(t, db.Where("model_name = ?", "gpt-test").First(&component).Error)
	var period model.StatusPeriod
	require.NoError(t, db.Where("component_id = ? AND granularity = ?", component.ID, model.StatusGranularityFiveMinutes).First(&period).Error)
	t.Run("aligned window", func(t *testing.T) {
		require.Equal(t, [][2]int64{{300, 600}, {300, 600}, {300, 600}}, windows)
		require.EqualValues(t, 300, period.PeriodStart)
	})
	t.Run("worst status", func(t *testing.T) {
		require.Equal(t, model.StatusDegraded, period.WorstStatus)
	})
	t.Run("integer aggregates", func(t *testing.T) {
		require.EqualValues(t, 1, period.KnownBucketCount)
		require.EqualValues(t, 1_000_000, period.ScoreSumMicros)
		require.EqualValues(t, 100, period.EligibleCount)
		require.EqualValues(t, 100, period.SuccessCount)
	})
}

func TestStatusSchedulerConsumesTrafficRecoveryBucketOnlyOnce(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	require.NoError(t, db.Create(&model.StatusComponent{
		ComponentKey:    "model:gpt-test",
		Slug:            "model-gpt-test",
		Kind:            model.StatusComponentKindModel,
		ModelName:       "gpt-test",
		DisplayName:     "gpt-test",
		Lifecycle:       model.StatusLifecycleActive,
		ObservedStatus:  model.StatusDegraded,
		EffectiveStatus: model.StatusDegraded,
		Version:         1,
	}).Error)
	currentTime := int64(610)
	scheduler := &StatusScheduler{
		Holder: "node-a",
		Now:    func() int64 { return currentTime },
		Pricing: func() []model.Pricing {
			return []model.Pricing{{ModelName: "gpt-test", EnableGroup: []string{"public"}}}
		},
		UsableGroups: func() map[string]string { return map[string]string{"public": "Public"} },
		Traffic: func(_ int64, _ int64, _ []string) ([]model.PerfMetricSummary, error) {
			return []model.PerfMetricSummary{{ModelName: "gpt-test", AvailabilityEligibleCount: 100, AvailabilitySuccessCount: 100}}, nil
		},
		RouterProbe: StatusProbeAdapterFunc(func(_ context.Context, _ model.StatusComponent) StatusProbeOutcome {
			return StatusProbeOutcome{Success: true, DiagnosticType: "ok"}
		}),
	}

	for _, now := range []int64{610, 670, 730} {
		currentTime = now
		ran, err := scheduler.RunOnce(context.Background(), now)
		require.NoError(t, err)
		require.True(t, ran)
	}

	var component model.StatusComponent
	require.NoError(t, db.Where("model_name = ?", "gpt-test").First(&component).Error)
	require.Equal(t, model.StatusDegraded, component.ObservedStatus)
	require.EqualValues(t, 1, component.ConsecutiveTrafficRecovery)

	currentTime = 910
	ran, err := scheduler.RunOnce(context.Background(), currentTime)
	require.NoError(t, err)
	require.True(t, ran)
	require.NoError(t, db.Where("model_name = ?", "gpt-test").First(&component).Error)
	require.Equal(t, model.StatusOperational, component.ObservedStatus)
	require.EqualValues(t, 2, component.ConsecutiveTrafficRecovery)
}

func TestStatusSchedulerBucketReaderExcludesEndBoundary(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.PerfMetricAvailability{}))
	require.NoError(t, db.Create(&[]model.PerfMetricAvailability{
		{ModelName: "gpt-test", Group: "public", BucketTs: 300, EligibleCount: 10, SuccessCount: 10},
		{ModelName: "gpt-test", Group: "public", BucketTs: 599, EligibleCount: 20, SuccessCount: 19},
		{ModelName: "gpt-test", Group: "public", BucketTs: 600, EligibleCount: 100},
	}).Error)

	summaries, err := readStatusTraffic(300, 600, []string{"public"})
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	require.EqualValues(t, 30, summaries[0].AvailabilityEligibleCount)
	require.EqualValues(t, 29, summaries[0].AvailabilitySuccessCount)
}

func TestStatusSchedulerRollupsAreIdempotent(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	lease := acquireStatusServiceLease(t, statusSchedulerJobName, "node-a", 7_200)
	component := model.StatusComponent{ComponentKey: "model:gpt-test", Slug: "gpt-test", Kind: model.StatusComponentKindModel, ModelName: "gpt-test", DisplayName: "gpt-test", Lifecycle: model.StatusLifecycleActive, ObservedStatus: model.StatusOperational, EffectiveStatus: model.StatusOperational, Version: 1}
	require.NoError(t, db.Create(&component).Error)

	fiveMinute := &model.StatusPeriod{ComponentID: component.ID, Granularity: model.StatusGranularityFiveMinutes, PeriodStart: 3_600, ScoreSumMicros: 1_000_000, KnownBucketCount: 1, WorstStatus: model.StatusOperational}
	require.NoError(t, writeStatusPeriodWithFence(statusSchedulerJobName, "node-a", lease.FencingToken, 7_200, fiveMinute))
	fiveMinute.ScoreSumMicros = 900_000
	fiveMinute.WorstStatus = model.StatusDegraded
	require.NoError(t, writeStatusPeriodWithFence(statusSchedulerJobName, "node-a", lease.FencingToken, 7_200, fiveMinute))
	require.NoError(t, rollupStatusPeriodsWithFence(statusSchedulerJobName, "node-a", lease.FencingToken, 7_200))
	require.NoError(t, rollupStatusPeriodsWithFence(statusSchedulerJobName, "node-a", lease.FencingToken, 7_200))

	require.EqualValues(t, 1, countStatusPeriodsByGranularity(t, db, model.StatusGranularityFiveMinutes))
	require.EqualValues(t, 1, countStatusPeriodsByGranularity(t, db, model.StatusGranularityHour))
	require.EqualValues(t, 1, countStatusPeriodsByGranularity(t, db, model.StatusGranularityDay))
	var periods []model.StatusPeriod
	require.NoError(t, db.Find(&periods).Error)
	for _, period := range periods {
		require.EqualValues(t, 900_000, period.ScoreSumMicros)
		require.Equal(t, model.StatusDegraded, period.WorstStatus)
	}
}

func TestStatusSchedulerRollupFinalFiveMinutePeriodIntoOwningDay(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	now := int64(24*60*60 + 70)
	lease := acquireStatusServiceLease(t, statusSchedulerJobName, "node-a", now)
	component := model.StatusComponent{ComponentKey: "router", Slug: "router", Kind: model.StatusComponentKindRouter, DisplayName: "Router", Lifecycle: model.StatusLifecycleActive, Version: 1}
	require.NoError(t, db.Create(&component).Error)

	fiveMinuteStart := int64(23*60*60 + 55*60)
	require.NoError(t, writeStatusPeriodWithFence(statusSchedulerJobName, "node-a", lease.FencingToken, now, &model.StatusPeriod{
		ComponentID:      component.ID,
		Granularity:      model.StatusGranularityFiveMinutes,
		PeriodStart:      fiveMinuteStart,
		ScoreSumMicros:   900_000,
		KnownBucketCount: 1,
		WorstStatus:      model.StatusDegraded,
	}))

	require.NoError(t, rollupStatusPeriodsWithFence(statusSchedulerJobName, "node-a", lease.FencingToken, now))

	var hour model.StatusPeriod
	require.NoError(t, db.Where("component_id = ? AND granularity = ?", component.ID, model.StatusGranularityHour).First(&hour).Error)
	require.EqualValues(t, 23*60*60, hour.PeriodStart)
	var day model.StatusPeriod
	require.NoError(t, db.Where("component_id = ? AND granularity = ?", component.ID, model.StatusGranularityDay).First(&day).Error)
	require.EqualValues(t, 0, day.PeriodStart)
	require.EqualValues(t, 900_000, day.ScoreSumMicros)
}

func TestStatusSchedulerRetentionKeepsNinetyDayAggregates(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	now := int64(200 * 24 * 60 * 60)
	lease := acquireStatusServiceLease(t, statusSchedulerJobName, "node-a", now)
	component := model.StatusComponent{ComponentKey: "router", Slug: "router", Kind: model.StatusComponentKindRouter, DisplayName: "Router", Lifecycle: model.StatusLifecycleActive, Version: 1}
	require.NoError(t, db.Create(&component).Error)

	rawCutoff := now - statusRawRetentionSeconds
	aggregateCutoff := now - statusAggregateRetentionSeconds
	require.NoError(t, db.Create(&[]model.StatusProbeResult{
		{ComponentID: component.ID, CreatedAt: rawCutoff - 1},
		{ComponentID: component.ID, CreatedAt: rawCutoff},
	}).Error)
	require.NoError(t, db.Create(&[]model.StatusPeriod{
		{ComponentID: component.ID, Granularity: model.StatusGranularityFiveMinutes, PeriodStart: rawCutoff - 1},
		{ComponentID: component.ID, Granularity: model.StatusGranularityFiveMinutes, PeriodStart: rawCutoff},
		{ComponentID: component.ID, Granularity: model.StatusGranularityHour, PeriodStart: aggregateCutoff - 1},
		{ComponentID: component.ID, Granularity: model.StatusGranularityHour, PeriodStart: aggregateCutoff},
		{ComponentID: component.ID, Granularity: model.StatusGranularityDay, PeriodStart: aggregateCutoff - 1},
		{ComponentID: component.ID, Granularity: model.StatusGranularityDay, PeriodStart: aggregateCutoff},
	}).Error)

	require.NoError(t, applyStatusRetentionWithFence(statusSchedulerJobName, "node-a", lease.FencingToken, now))
	require.EqualValues(t, 1, countRows(t, db, &model.StatusProbeResult{}))
	require.EqualValues(t, 1, countStatusPeriodsByGranularity(t, db, model.StatusGranularityFiveMinutes))
	require.EqualValues(t, 1, countStatusPeriodsByGranularity(t, db, model.StatusGranularityHour))
	require.EqualValues(t, 1, countStatusPeriodsByGranularity(t, db, model.StatusGranularityDay))
}

func TestStatusSchedulerRouterCanaryUsesInjectedHTTPClient(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api/status", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	adapter := NewStatusRouterProbeAdapter(server.URL, server.Client())
	outcome := adapter.ProbeStatusComponent(context.Background(), model.StatusComponent{Kind: model.StatusComponentKindRouter})
	require.True(t, outcome.Success)
	require.False(t, outcome.MonitoringFault)
	require.Equal(t, "ok", outcome.DiagnosticType)
	require.EqualValues(t, 1, calls.Load())
}

func TestStatusSchedulerStartStatusCenterTasksRequiresMasterAndEnabled(t *testing.T) {
	originalMaster := common.IsMasterNode
	originalOnce := statusCenterTaskOnce
	originalLaunch := statusCenterTaskLaunch
	t.Cleanup(func() {
		common.IsMasterNode = originalMaster
		statusCenterTaskOnce = originalOnce
		statusCenterTaskLaunch = originalLaunch
	})
	statusCenterTaskOnce = &sync.Once{}
	var launches atomic.Int64
	statusCenterTaskLaunch = func(_ *StatusScheduler) { launches.Add(1) }
	t.Setenv("ROUTER_ORIGIN", "https://router.flatkey.ai")

	common.IsMasterNode = false
	t.Setenv("STATUS_CENTER_ENABLED", "true")
	require.False(t, StartStatusCenterTasks())
	common.IsMasterNode = true
	t.Setenv("STATUS_CENTER_ENABLED", "false")
	require.False(t, StartStatusCenterTasks())
	t.Setenv("STATUS_CENTER_ENABLED", "true")
	require.True(t, StartStatusCenterTasks())
	require.False(t, StartStatusCenterTasks())
	require.EqualValues(t, 1, launches.Load())
}

func TestStatusSchedulerStartRequiresValidRouterOriginBeforeConsumingOnce(t *testing.T) {
	originalMaster := common.IsMasterNode
	originalOnce := statusCenterTaskOnce
	originalLaunch := statusCenterTaskLaunch
	statusAvailabilityMu.RLock()
	originalAvailability := statusAvailabilityWriter
	statusAvailabilityMu.RUnlock()
	t.Cleanup(func() {
		common.IsMasterNode = originalMaster
		statusCenterTaskOnce = originalOnce
		statusCenterTaskLaunch = originalLaunch
		statusAvailabilityMu.Lock()
		statusAvailabilityWriter = originalAvailability
		statusAvailabilityMu.Unlock()
	})

	common.IsMasterNode = true
	statusCenterTaskOnce = &sync.Once{}
	var launched []*StatusScheduler
	statusCenterTaskLaunch = func(scheduler *StatusScheduler) { launched = append(launched, scheduler) }
	SetStatusModelAvailabilityWriter(func(_ string, _ string, _ int64, _ int64, _ string, _ StatusProbeOutcome) error { return nil })
	t.Setenv("STATUS_CENTER_ENABLED", "true")

	t.Setenv("ROUTER_ORIGIN", "")
	require.False(t, StartStatusCenterTasks())
	t.Setenv("ROUTER_ORIGIN", "ftp://router.flatkey.ai")
	require.False(t, StartStatusCenterTasks())
	require.Empty(t, launched)

	t.Setenv("ROUTER_ORIGIN", "https://router.flatkey.ai")
	require.True(t, StartStatusCenterTasks())
	require.Len(t, launched, 1)
	adapter, ok := launched[0].RouterProbe.(statusRouterProbeAdapter)
	require.True(t, ok)
	require.Equal(t, "https://router.flatkey.ai", adapter.origin)
	require.NotNil(t, launched[0].Availability)
}

func countRows(t *testing.T, db *gorm.DB, value any) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(value).Count(&count).Error)
	return count
}

func countStatusPeriodsByGranularity(t *testing.T, db *gorm.DB, granularity string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&model.StatusPeriod{}).Where("granularity = ?", granularity).Count(&count).Error)
	return count
}
