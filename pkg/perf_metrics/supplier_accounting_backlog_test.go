package perfmetrics

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupSupplierAccountingBacklogPrometheusDB(t *testing.T) (*gorm.DB, int64) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=busy_timeout(5000)"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.SupplierUsageDailyBatchRun{}))
	var dbNow int64
	require.NoError(t, db.Raw("SELECT strftime('%s','now')").Scan(&dbNow).Error)
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Unix(dbNow, 0).In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	cutover := today.AddDate(0, 0, -2).Unix()
	preparedAt := cutover - 1
	preparedBy := 7
	activation := model.SupplierAccountingActivationState{
		SchemaVersion: 1, StateVersion: 3, Phase: model.SupplierAccountingActivationActive,
		CutoverAt: &cutover, AcceptedCapabilityVersions: []int{1}, PreparedAt: &preparedAt,
		PreparedBy: &preparedBy, ActivatedAt: &cutover, Reason: "backlog observer test",
	}
	encoded, err := common.Marshal(activation)
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Option{Key: model.SupplierAccountingActivationOptionKey, Value: string(encoded)}).Error)

	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		supplierAccountingBacklogCache.clear(nil)
		supplierAccountingBacklogSingleflight.Forget(supplierAccountingBacklogSingleflightKey)
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db, cutover
}

func createSupplierAccountingBacklogPrometheusRows(t *testing.T, db *gorm.DB, cutover int64) {
	t.Helper()
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	start := time.Unix(cutover, 0).In(location)
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, location)
	prior := start.AddDate(0, 0, 1)
	today := prior.AddDate(0, 0, 1)
	require.NoError(t, db.Create(&[]model.SupplierUsageDailyBatchRun{
		{BatchDate: start.Format("2006-01-02"), DayStart: start.Unix(), DayEnd: prior.Unix(), Status: model.SupplierDailyBatchStatusRunning, PublishedFenceToken: 0, LockedUntil: 1<<62 - 1},
		{BatchDate: prior.Format("2006-01-02"), DayStart: prior.Unix(), DayEnd: today.Unix(), Status: model.SupplierDailyBatchStatusCompleted, PublishedFenceToken: 0},
	}).Error)
}

func TestWriteSupplierAccountingBacklogPrometheusMetricsExactText(t *testing.T) {
	var b strings.Builder
	writeSupplierAccountingBacklogPrometheusMetrics(&b, supplierAccountingBacklogPrometheusSnapshot{
		state: supplierAccountingBacklogPrometheusUp,
		observation: model.SupplierAccountingBacklogObservation{
			ObservedAtUnix: 1_784_854_800, NeverPublishedDays: 2,
			OldestNeverPublishedAgeSeconds: 86_401, PriorDayUnpublishedAfter0800: true,
		},
	})
	require.Equal(t, "# HELP newapi_supplier_accounting_backlog_observer_up Whether the supplier accounting backlog observer completed successfully.\n"+
		"# TYPE newapi_supplier_accounting_backlog_observer_up gauge\n"+
		"newapi_supplier_accounting_backlog_observer_up 1\n"+
		"# HELP newapi_supplier_accounting_never_published_days Supplier accounting days in the bounded scheduler range with no published fence.\n"+
		"# TYPE newapi_supplier_accounting_never_published_days gauge\n"+
		"newapi_supplier_accounting_never_published_days 2\n"+
		"# HELP newapi_supplier_accounting_oldest_never_published_age_seconds Age of the oldest never-published supplier accounting day.\n"+
		"# TYPE newapi_supplier_accounting_oldest_never_published_age_seconds gauge\n"+
		"newapi_supplier_accounting_oldest_never_published_age_seconds 86401\n"+
		"# HELP newapi_supplier_accounting_prior_day_unpublished_after_0800 Whether the prior Shanghai accounting day remains unpublished at or after 08:00.\n"+
		"# TYPE newapi_supplier_accounting_prior_day_unpublished_after_0800 gauge\n"+
		"newapi_supplier_accounting_prior_day_unpublished_after_0800 1\n"+
		"# HELP newapi_supplier_accounting_backlog_observed_at_seconds Database unix time of the supplier accounting backlog observation.\n"+
		"# TYPE newapi_supplier_accounting_backlog_observed_at_seconds gauge\n"+
		"newapi_supplier_accounting_backlog_observed_at_seconds 1784854800\n", b.String())
	require.Equal(t, 5, (supplierAccountingBacklogPrometheusSnapshot{state: supplierAccountingBacklogPrometheusUp}).seriesCount())
	require.Equal(t, 1, (supplierAccountingBacklogPrometheusSnapshot{state: supplierAccountingBacklogPrometheusDown}).seriesCount())
	require.Zero(t, (supplierAccountingBacklogPrometheusSnapshot{state: supplierAccountingBacklogPrometheusOmitted}).seriesCount())
}

func TestBuildPrometheusTextEmitsSupplierAccountingBacklogFixedGauges(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	db, cutover := setupSupplierAccountingBacklogPrometheusDB(t)
	createSupplierAccountingBacklogPrometheusRows(t, db, cutover)
	t.Setenv(prometheusMaxSeriesPerScrapeEnv, "5")

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	requirePrometheusSampleLine(t, text, "newapi_perf_metrics_series 5")
	requirePrometheusSampleLine(t, text, "newapi_supplier_accounting_backlog_observer_up 1")
	requirePrometheusSampleLine(t, text, "newapi_supplier_accounting_never_published_days 2")
	require.Contains(t, text, "newapi_supplier_accounting_oldest_never_published_age_seconds ")
	require.Contains(t, text, "newapi_supplier_accounting_prior_day_unpublished_after_0800 ")
	require.Contains(t, text, "newapi_supplier_accounting_backlog_observed_at_seconds ")
	require.NotContains(t, text, "newapi_supplier_accounting_never_published_days{")
	requirePrometheusSeriesGaugeMatchesRenderedSamples(t, text)
}

func TestSupplierAccountingBacklogMetricsRespectFixedSeriesLimit(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	db, cutover := setupSupplierAccountingBacklogPrometheusDB(t)
	createSupplierAccountingBacklogPrometheusRows(t, db, cutover)
	t.Setenv(prometheusMaxSeriesPerScrapeEnv, "4")
	_, err := BuildPrometheusText(context.Background())
	require.ErrorContains(t, err, "prometheus series limit exceeded: 5 > 4")

	t.Setenv(prometheusMaxSeriesPerScrapeEnv, "5")
	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	requirePrometheusSampleLine(t, text, "newapi_perf_metrics_series 5")
}

func TestSupplierAccountingBacklogObserverFailureOmitsTruthAndDoesNotFailScrape(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	db, _ := setupSupplierAccountingBacklogPrometheusDB(t)
	require.NoError(t, db.Migrator().DropTable(&model.SupplierUsageDailyBatchRun{}))
	t.Setenv(prometheusMaxSeriesPerScrapeEnv, "1")

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	requirePrometheusSampleLine(t, text, "newapi_perf_metrics_series 1")
	requirePrometheusSampleLine(t, text, "newapi_supplier_accounting_backlog_observer_up 0")
	require.NotContains(t, text, "newapi_supplier_accounting_never_published_days")
	require.NotContains(t, text, "newapi_supplier_accounting_oldest_never_published_age_seconds")
	require.NotContains(t, text, "newapi_supplier_accounting_prior_day_unpublished_after_0800")
	require.NotContains(t, text, "newapi_supplier_accounting_backlog_observed_at_seconds")
	requirePrometheusSeriesGaugeMatchesRenderedSamples(t, text)
}

func TestSupplierAccountingBacklogObserverCancellationEmitsDown(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	_, _ = setupSupplierAccountingBacklogPrometheusDB(t)
	t.Setenv(prometheusMaxSeriesPerScrapeEnv, "1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	text, err := BuildPrometheusText(ctx)
	require.NoError(t, err)
	requirePrometheusSampleLine(t, text, "newapi_supplier_accounting_backlog_observer_up 0")
	require.NotContains(t, text, "newapi_supplier_accounting_backlog_observed_at_seconds")
	_, _, _ = supplierAccountingBacklogSingleflight.Do(supplierAccountingBacklogSingleflightKey, func() (any, error) { return nil, nil })
}

func TestSupplierAccountingBacklogObserverDoesNotServeExpiredSuccessAfterFailure(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	db, cutover := setupSupplierAccountingBacklogPrometheusDB(t)
	createSupplierAccountingBacklogPrometheusRows(t, db, cutover)
	t.Setenv(prometheusMaxSeriesPerScrapeEnv, "5")

	first, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	requirePrometheusSampleLine(t, first, "newapi_supplier_accounting_backlog_observer_up 1")
	require.NoError(t, db.Migrator().DropTable(&model.SupplierUsageDailyBatchRun{}))
	fresh, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	requirePrometheusSampleLine(t, fresh, "newapi_supplier_accounting_backlog_observer_up 1")

	supplierAccountingBacklogCache.mu.Lock()
	supplierAccountingBacklogCache.expiresAt = time.Now().Add(-time.Second)
	supplierAccountingBacklogCache.mu.Unlock()
	afterExpiry, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	requirePrometheusSampleLine(t, afterExpiry, "newapi_supplier_accounting_backlog_observer_up 0")
	require.NotContains(t, afterExpiry, "newapi_supplier_accounting_never_published_days")
	require.NotContains(t, afterExpiry, "newapi_supplier_accounting_backlog_observed_at_seconds")
}

func TestSupplierAccountingBacklogObserverUnconfiguredPreservesExistingMetricsOutput(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	originalDB := model.DB
	model.DB = nil
	t.Cleanup(func() { model.DB = originalDB })
	t.Setenv(prometheusMaxSeriesPerScrapeEnv, "1")

	text, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	requirePrometheusSampleLine(t, text, "newapi_perf_metrics_series 0")
	require.NotContains(t, text, "newapi_supplier_accounting_backlog_observer")
}

func TestSupplierAccountingBacklogObserverSingleflightsConcurrentScrapes(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	db, cutover := setupSupplierAccountingBacklogPrometheusDB(t)
	createSupplierAccountingBacklogPrometheusRows(t, db, cutover)
	t.Setenv(prometheusMaxSeriesPerScrapeEnv, "5")
	var queryCount atomic.Int64
	increment := func(*gorm.DB) { queryCount.Add(1) }
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("supplier_backlog_query_count", increment))
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register("supplier_backlog_raw_count", increment))
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register("supplier_backlog_row_count", increment))

	_, err := BuildPrometheusText(context.Background())
	require.NoError(t, err)
	singleScrapeQueries := queryCount.Load()
	require.Positive(t, singleScrapeQueries)
	supplierAccountingBacklogCache.clear(nil)
	supplierAccountingBacklogSingleflight.Forget(supplierAccountingBacklogSingleflightKey)
	queryCount.Store(0)

	const scrapes = 32
	start := make(chan struct{})
	texts := make([]string, scrapes)
	errs := make([]error, scrapes)
	var wait sync.WaitGroup
	for index := range scrapes {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			texts[index], errs[index] = BuildPrometheusText(context.Background())
		}()
	}
	close(start)
	wait.Wait()
	for index := range scrapes {
		require.NoError(t, errs[index])
		require.Equal(t, texts[0], texts[index])
	}
	require.Equal(t, singleScrapeQueries, queryCount.Load(), "concurrent scrapes must share one bounded DB observation")
}
