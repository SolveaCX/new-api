package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

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
	statusCenterTaskLaunch = func() { launches.Add(1) }

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
