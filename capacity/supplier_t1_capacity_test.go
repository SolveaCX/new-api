package capacity_test

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	mysqlconfig "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	supplierProductionP99RowsEnv = "SUPPLIER_PRODUCTION_P99_DAILY_LOGS"
	supplierCapacityEvidenceDir  = "SUPPLIER_CAPACITY_EVIDENCE_DIR"
	supplierCapacitySmallSmoke   = "SUPPLIER_T1_CAPACITY_ALLOW_SMALL_SMOKE"
	supplierT1CapacityRunEnv     = "RUN_SUPPLIER_T1_CAPACITY_TEST"
	supplierIsolationTokenEnv    = "SUPPLIER_CAPACITY_ISOLATION_TOKEN"
	supplierMeasurementStartEnv  = "SUPPLIER_CAPACITY_MEASUREMENT_WINDOW_START"
	supplierMeasurementEndEnv    = "SUPPLIER_CAPACITY_MEASUREMENT_WINDOW_END"
	supplierSourceReferenceEnv   = "SUPPLIER_CAPACITY_SOURCE_REFERENCE"
	supplierSourceSHA256Env      = "SUPPLIER_CAPACITY_SOURCE_SHA256"
	supplierCapacitySentinel     = "supplier_capacity_test_sentinel"
	supplierCapacityHardTimeout  = 45 * time.Minute
	supplierCapacitySoftDeadline = 30 * time.Minute
	supplierCapacityMinimumRows  = int64(1_000_000)
	supplierMeasurementMaxAge    = 7 * 24 * time.Hour
	supplierMeasurementFutureMax = 5 * time.Minute
)

type supplierT1CapacityEvidence struct {
	SchemaVersion            int      `json:"schema_version"`
	EvidenceClass            string   `json:"evidence_class"`
	GeneratedAt              string   `json:"generated_at"`
	Commit                   string   `json:"commit"`
	WorkingTreeDirty         bool     `json:"working_tree_dirty"`
	Command                  string   `json:"command"`
	Database                 string   `json:"database"`
	DatabaseVersion          string   `json:"database_version"`
	ProductionP99DailyLogs   int64    `json:"production_p99_daily_logs"`
	TargetDayRows            int64    `json:"target_day_rows"`
	BackgroundRows           int64    `json:"background_rows"`
	InternalRows             int64    `json:"internal_rows"`
	BusinessRows             int64    `json:"business_rows"`
	InternalPercent          float64  `json:"internal_percent"`
	SeedDurationMilliseconds int64    `json:"seed_duration_ms"`
	BatchDurationMillis      int64    `json:"batch_duration_ms"`
	RowsPerSecond            float64  `json:"rows_per_second"`
	PageQueryDurationsMicros []int64  `json:"page_query_durations_us"`
	PageQueryP95Micros       int64    `json:"page_query_p95_us"`
	PageQueryMaxMicros       int64    `json:"page_query_max_us"`
	RetainedHeapDeltaBytes   int64    `json:"retained_heap_delta_bytes"`
	PeakRSSDeltaBytes        *int64   `json:"peak_rss_delta_bytes"`
	WaitingLocksBefore       int64    `json:"waiting_locks_before"`
	WaitingLocksAfter        int64    `json:"waiting_locks_after"`
	ExplainPlan              string   `json:"explain_plan"`
	FullScanDetected         bool     `json:"full_scan_detected"`
	PlanRepresentative       bool     `json:"plan_representative_of_production_distribution"`
	HostOS                   string   `json:"host_os"`
	HostArch                 string   `json:"host_arch"`
	HostCPUCount             int      `json:"host_cpu_count"`
	HostGOMAXPROCS           int      `json:"host_gomaxprocs"`
	HostGoVersion            string   `json:"host_go_version"`
	UnavailableFields        []string `json:"unavailable_fields"`
	ReleaseBlockers          []string `json:"release_blockers"`
	MeasurementWindowStart   string   `json:"measurement_window_start"`
	MeasurementWindowEnd     string   `json:"measurement_window_end"`
	SourceReference          string   `json:"source_reference"`
	SourceSHA256             string   `json:"source_sha256"`
}

type supplierCapacityProvenance struct {
	MeasurementWindowStart string
	MeasurementWindowEnd   string
	SourceReference        string
	SourceSHA256           string
}

func TestSupplierT1ProductionP99Capacity(t *testing.T) {
	if os.Getenv(supplierT1CapacityRunEnv) != "1" {
		t.Skipf("set %s=1 to run the destructive T+1 capacity release gate", supplierT1CapacityRunEnv)
	}
	rawP99 := strings.TrimSpace(os.Getenv(supplierProductionP99RowsEnv))
	require.NotEmpty(t, rawP99, "release-blocking capacity input absent: set %s", supplierProductionP99RowsEnv)
	productionP99, err := strconv.ParseInt(rawP99, 10, 64)
	require.NoError(t, err)
	require.Positive(t, productionP99)
	require.LessOrEqual(t, productionP99, (int64(^uint64(0)>>1)-19)/2, "rounded 2x p99 rows must fit in int64")
	totalRows, evidenceClass := supplierCapacityTargetRows(productionP99, os.Getenv(supplierCapacitySmallSmoke) == "1")
	provenance, err := supplierCapacityProvenanceFromEnvironment()
	require.NoError(t, err)
	isolationToken := strings.TrimSpace(os.Getenv(supplierIsolationTokenEnv))
	require.NotEmpty(t, isolationToken, "%s is required when the capacity gate is enabled", supplierIsolationTokenEnv)
	require.NotEmpty(t, strings.TrimSpace(os.Getenv(supplierCapacityEvidenceDir)), "%s is required when the capacity gate is enabled", supplierCapacityEvidenceDir)

	tests := []struct {
		name             string
		dsnEnv           string
		expectedDatabase string
		open             func(string) gorm.Dialector
	}{
		{name: "mysql", dsnEnv: "TEST_MYSQL_DSN", expectedDatabase: "supplier_g009_mysql", open: func(dsn string) gorm.Dialector { return mysql.Open(dsn) }},
		{name: "postgres", dsnEnv: "TEST_POSTGRES_DSN", expectedDatabase: "supplier_g009_postgres", open: func(dsn string) gorm.Dialector { return postgres.Open(dsn) }},
	}
	for _, testCase := range tests {
		dsn := strings.TrimSpace(os.Getenv(testCase.dsnEnv))
		require.NotEmpty(t, dsn, "%s is required when the capacity gate is enabled", testCase.dsnEnv)
		require.NoError(t, supplierCapacityRequireLoopbackDSN(testCase.name, dsn))
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(testCase.dsnEnv))
			db, err := gorm.Open(testCase.open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			requireSupplierCapacityDatabase(t, db, testCase.name, testCase.expectedDatabase, isolationToken)
			runSupplierT1CapacityCase(t, db, testCase.name, productionP99, totalRows, evidenceClass, provenance)
		})
	}
}

func TestSupplierT1CapacityMinimumAndSmokeClassification(t *testing.T) {
	tests := []struct {
		name      string
		p99       int64
		allowTiny bool
		wantRows  int64
		wantClass string
	}{
		{name: "manual minimum", p99: 1_000, wantRows: 1_000_000, wantClass: "local_capacity_not_production_equivalent"},
		{name: "explicit small smoke", p99: 1_000, allowTiny: true, wantRows: 2_000, wantClass: "smoke_not_release"},
		{name: "twice production p99", p99: 600_000, wantRows: 1_200_000, wantClass: "local_capacity_not_production_equivalent"},
		{name: "rounds minimum up", p99: 500_001, wantRows: 1_000_020, wantClass: "local_capacity_not_production_equivalent"},
		{name: "rounds smoke up", p99: 1_001, allowTiny: true, wantRows: 2_020, wantClass: "smoke_not_release"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			rows, class := supplierCapacityTargetRows(testCase.p99, testCase.allowTiny)
			require.Equal(t, testCase.wantRows, rows)
			require.Equal(t, testCase.wantClass, class)
		})
	}
}

func TestSupplierCapacityProvenanceValidation(t *testing.T) {
	for _, name := range []string{supplierMeasurementStartEnv, supplierMeasurementEndEnv, supplierSourceReferenceEnv, supplierSourceSHA256Env} {
		t.Setenv(name, "")
	}
	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	_, err := supplierCapacityProvenanceFromEnvironmentAt(now)
	require.Error(t, err)

	t.Setenv(supplierSourceReferenceEnv, "cloudsql-query:successful-consume-logs-p99-v1")
	t.Setenv(supplierSourceSHA256Env, strings.Repeat("a", 64))
	tests := []struct {
		name   string
		start  time.Time
		end    time.Time
		sha256 string
		valid  bool
	}{
		{name: "fresh", start: now.Add(-24 * time.Hour), end: now.Add(-time.Hour), sha256: strings.Repeat("a", 64), valid: true},
		{name: "oldest inclusive endpoint", start: now.Add(-8 * 24 * time.Hour), end: now.Add(-supplierMeasurementMaxAge), sha256: strings.Repeat("a", 64), valid: true},
		{name: "future inclusive endpoint", start: now, end: now.Add(supplierMeasurementFutureMax), sha256: strings.Repeat("a", 64), valid: true},
		{name: "stale", start: now.Add(-8 * 24 * time.Hour), end: now.Add(-supplierMeasurementMaxAge - time.Nanosecond), sha256: strings.Repeat("a", 64)},
		{name: "future", start: now, end: now.Add(supplierMeasurementFutureMax + time.Nanosecond), sha256: strings.Repeat("a", 64)},
		{name: "unordered", start: now, end: now.Add(-time.Hour), sha256: strings.Repeat("a", 64)},
		{name: "uppercase SHA", start: now.Add(-24 * time.Hour), end: now.Add(-time.Hour), sha256: strings.Repeat("A", 64)},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(supplierMeasurementStartEnv, testCase.start.Format(time.RFC3339Nano))
			t.Setenv(supplierMeasurementEndEnv, testCase.end.Format(time.RFC3339Nano))
			t.Setenv(supplierSourceSHA256Env, testCase.sha256)
			provenance, err := supplierCapacityProvenanceFromEnvironmentAt(now)
			if testCase.valid {
				require.NoError(t, err)
				require.Equal(t, "cloudsql-query:successful-consume-logs-p99-v1", provenance.SourceReference)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestSupplierCapacityLoopbackDSNValidation(t *testing.T) {
	require.NoError(t, supplierCapacityRequireLoopbackDSN("mysql", "root:password@tcp(127.0.0.1:3306)/supplier_g009_mysql"))
	require.NoError(t, supplierCapacityRequireLoopbackDSN("postgres", "host=localhost port=5432 user=postgres dbname=supplier_g009_postgres"))
	require.Error(t, supplierCapacityRequireLoopbackDSN("mysql", "root:password@tcp(db.internal:3306)/supplier_g009_mysql"))
	require.Error(t, supplierCapacityRequireLoopbackDSN("postgres", "host=10.0.0.8 port=5432 user=postgres dbname=supplier_g009_postgres"))
}

func supplierCapacityTargetRows(productionP99 int64, allowSmallSmoke bool) (int64, string) {
	totalRows := productionP99 * 2
	if totalRows < supplierCapacityMinimumRows && allowSmallSmoke {
		return supplierCapacityRoundRows(totalRows), "smoke_not_release"
	}
	if totalRows < supplierCapacityMinimumRows {
		totalRows = supplierCapacityMinimumRows
	}
	return supplierCapacityRoundRows(totalRows), "local_capacity_not_production_equivalent"
}

func supplierCapacityRoundRows(rows int64) int64 {
	return ((rows + 19) / 20) * 20
}

func runSupplierT1CapacityCase(t *testing.T, db *gorm.DB, dialect string, productionP99, totalRows int64, evidenceClass string, provenance supplierCapacityProvenance) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.Log{}, &model.SupplierUsageDailySummary{}, &model.SupplierUsageDailyBatchRun{}))
	require.NoError(t, model.EnsureSupplierUsageGenerationSchema(db))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(8)

	location, err := time.LoadLocation(service.SupplierDailyBatchTimezone)
	require.NoError(t, err)
	day := time.Date(1998, time.January, 2, 0, 0, 0, 0, location)
	dayEnd := day.AddDate(0, 0, 1)
	cleanupSupplierT1CapacityCase(t, db, day)
	t.Cleanup(func() { cleanupSupplierT1CapacityCase(t, db, day) })
	persistSupplierCapacityActivation(t, db, day)
	businessPayload, internalPayload := supplierCapacityPayloads(t, day)

	seedStarted := time.Now()
	internalRows, businessRows := insertSupplierCapacityLogs(t, sqlDB, dialect, day, totalRows, businessPayload, internalPayload)
	seedDuration := time.Since(seedStarted)
	require.Equal(t, totalRows, internalRows+businessRows)
	require.Equal(t, totalRows*95/100, internalRows, "fixture must be exactly 95% internal when total rows are divisible by 20")
	if dialect == "postgres" {
		require.NoError(t, db.Exec("ANALYZE logs").Error)
	} else {
		require.NoError(t, db.Exec("ANALYZE TABLE logs").Error)
	}

	plan := supplierCapacityExplainPlan(t, sqlDB, dialect, day.Unix(), dayEnd.Unix())
	fullScan := supplierCapacityPlanHasFullScan(dialect, plan)

	queryRecorder := &supplierCapacityQueryRecorder{}
	measuredDB := db.Session(&gorm.Session{Logger: queryRecorder})
	var slowPageStreak atomic.Int64
	ctx, cancel := context.WithTimeout(context.Background(), supplierCapacityHardTimeout)
	defer cancel()
	queryRecorder.onPage = func(duration time.Duration) {
		if duration > 10*time.Second {
			if slowPageStreak.Add(1) >= 3 {
				cancel()
			}
		} else {
			slowPageStreak.Store(0)
		}
	}

	beforeLocks := supplierCapacityWaitingLocks(t, db, dialect)
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	peakRSS, stopRSS := sampleSupplierCapacityRSS()
	startedAt := time.Now()
	runErr := service.RunSupplierDailyBatch(ctx, measuredDB, measuredDB, day.Format("2006-01-02"), "supplier-t1-capacity", dayEnd.AddDate(0, 0, 1), false)
	elapsed := time.Since(startedAt)
	stopRSS()
	require.NoError(t, runErr)
	require.LessOrEqual(t, elapsed, supplierCapacitySoftDeadline)
	require.NoError(t, ctx.Err())
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	afterLocks := supplierCapacityWaitingLocks(t, db, dialect)
	require.Zero(t, beforeLocks)
	require.Zero(t, afterLocks)

	var run model.SupplierUsageDailyBatchRun
	require.NoError(t, db.Where("batch_date = ?", day.Format("2006-01-02")).First(&run).Error)
	require.Equal(t, model.SupplierDailyBatchStatusCompleted, run.Status)
	require.Equal(t, totalRows, run.LogsScanned)
	require.Equal(t, totalRows, run.SnapshotCount)
	rowsPerSecond := float64(totalRows) / elapsed.Seconds()
	require.GreaterOrEqual(t, rowsPerSecond, 8_000.0)
	retainedHeapDelta := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	require.LessOrEqual(t, retainedHeapDelta, int64(96*1024*1024))

	sortedPages := queryRecorder.Durations()
	require.NotEmpty(t, sortedPages)
	sort.Slice(sortedPages, func(left, right int) bool { return sortedPages[left] < sortedPages[right] })
	pageP95 := supplierCapacityPercentile(sortedPages, 95)
	require.LessOrEqual(t, pageP95, 5*time.Second)
	require.Less(t, slowPageStreak.Load(), int64(3))

	var peakRSSDelta *int64
	if peakRSS != nil {
		delta := peakRSS.Peak() - peakRSS.Baseline()
		peakRSSDelta = &delta
		require.LessOrEqual(t, delta, int64(512*1024*1024))
	}
	evidence := supplierT1CapacityEvidence{
		SchemaVersion: 1, EvidenceClass: evidenceClass, GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Commit: supplierCapacityCommit(t),
		WorkingTreeDirty: supplierCapacityWorkingTreeDirty(t),
		Command:          supplierCapacityCommand(productionP99, evidenceClass),
		Database:         dialect, DatabaseVersion: supplierCapacityDatabaseVersion(t, db), ProductionP99DailyLogs: productionP99,
		TargetDayRows: totalRows, BackgroundRows: totalRows, InternalRows: internalRows, BusinessRows: businessRows, InternalPercent: float64(internalRows) * 100 / float64(totalRows),
		SeedDurationMilliseconds: seedDuration.Milliseconds(), BatchDurationMillis: elapsed.Milliseconds(), RowsPerSecond: rowsPerSecond,
		PageQueryDurationsMicros: supplierCapacityDurationsMicros(sortedPages), PageQueryP95Micros: pageP95.Microseconds(), PageQueryMaxMicros: sortedPages[len(sortedPages)-1].Microseconds(),
		RetainedHeapDeltaBytes: retainedHeapDelta, PeakRSSDeltaBytes: peakRSSDelta, WaitingLocksBefore: beforeLocks, WaitingLocksAfter: afterLocks,
		ExplainPlan: plan, FullScanDetected: fullScan, PlanRepresentative: false, HostOS: runtime.GOOS, HostArch: runtime.GOARCH, HostCPUCount: runtime.NumCPU(), HostGOMAXPROCS: runtime.GOMAXPROCS(0), HostGoVersion: runtime.Version(),
		UnavailableFields:      []string{"production_equivalent_explain_plan", "database_cpu_average_percent", "database_cpu_five_minute_peak_percent", "lock_wait_p95_seconds", "lock_wait_max_seconds", "replica_lag_seconds", "actual_rows_examined_or_read"},
		ReleaseBlockers:        []string{"production-distribution EXPLAIN and actual rows read <=1.2x", "production Cloud SQL CPU/lock/replica-lag evidence", "approved staging/live alert fire-and-resolve evidence"},
		MeasurementWindowStart: provenance.MeasurementWindowStart, MeasurementWindowEnd: provenance.MeasurementWindowEnd,
		SourceReference: provenance.SourceReference, SourceSHA256: provenance.SourceSHA256,
	}
	writeSupplierCapacityEvidence(t, evidence)
	t.Logf("database=%s evidence_class=%s p99_daily_logs=%d target_day_rows=%d background_rows=%d internal_rows=%d business_rows=%d seed=%s batch=%s rows_per_second=%.0f pages=%d page_p95=%s page_max=%s retained_heap_delta=%d peak_rss_delta=%v waiting_locks=%d/%d local_full_scan_detected=%t plan_representative_of_production=false unavailable=%v",
		dialect, evidenceClass, productionP99, totalRows, totalRows, internalRows, businessRows, seedDuration, elapsed, rowsPerSecond, len(sortedPages), pageP95, sortedPages[len(sortedPages)-1], retainedHeapDelta, peakRSSDelta, beforeLocks, afterLocks, fullScan, evidence.UnavailableFields)
}

func supplierCapacityCommand(productionP99 int64, evidenceClass string) string {
	prefix := fmt.Sprintf("%s=1 SUPPLIER_PRODUCTION_P99_DAILY_LOGS=%d ", supplierT1CapacityRunEnv, productionP99)
	if evidenceClass == "smoke_not_release" {
		prefix += supplierCapacitySmallSmoke + "=1 "
	}
	return prefix + "SUPPLIER_CAPACITY_ISOLATION_TOKEN='<redacted>' TEST_MYSQL_DSN='<redacted>' TEST_POSTGRES_DSN='<redacted>' go test ./capacity -run '^TestSupplierT1ProductionP99Capacity$' -count=1 -v -timeout=120m"
}

func supplierCapacityProvenanceFromEnvironment() (supplierCapacityProvenance, error) {
	return supplierCapacityProvenanceFromEnvironmentAt(time.Now())
}

func supplierCapacityProvenanceFromEnvironmentAt(now time.Time) (supplierCapacityProvenance, error) {
	provenance := supplierCapacityProvenance{MeasurementWindowStart: strings.TrimSpace(os.Getenv(supplierMeasurementStartEnv)), MeasurementWindowEnd: strings.TrimSpace(os.Getenv(supplierMeasurementEndEnv)), SourceReference: strings.TrimSpace(os.Getenv(supplierSourceReferenceEnv)), SourceSHA256: strings.TrimSpace(os.Getenv(supplierSourceSHA256Env))}
	if provenance.MeasurementWindowStart == "" || provenance.MeasurementWindowEnd == "" || provenance.SourceReference == "" || provenance.SourceSHA256 == "" {
		return provenance, fmt.Errorf("measurement window start/end, source reference, and source SHA-256 are required")
	}
	start, err := time.Parse(time.RFC3339, provenance.MeasurementWindowStart)
	if err != nil {
		return provenance, fmt.Errorf("parse measurement window start: %w", err)
	}
	end, err := time.Parse(time.RFC3339, provenance.MeasurementWindowEnd)
	if err != nil {
		return provenance, fmt.Errorf("parse measurement window end: %w", err)
	}
	if !end.After(start) {
		return provenance, fmt.Errorf("measurement window end must be after start")
	}
	if end.Before(now.Add(-supplierMeasurementMaxAge)) {
		return provenance, fmt.Errorf("measurement window end must be no older than %s", supplierMeasurementMaxAge)
	}
	if end.After(now.Add(supplierMeasurementFutureMax)) {
		return provenance, fmt.Errorf("measurement window end must not be more than %s in the future", supplierMeasurementFutureMax)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(provenance.SourceSHA256) {
		return provenance, fmt.Errorf("source SHA-256 must be 64 lowercase hexadecimal characters")
	}
	return provenance, nil
}

func supplierCapacityRequireLoopbackDSN(dialect, dsn string) error {
	var host string
	if dialect == "mysql" {
		config, err := mysqlconfig.ParseDSN(dsn)
		if err != nil {
			return fmt.Errorf("parse MySQL DSN: %w", err)
		}
		if !strings.HasPrefix(config.Net, "tcp") {
			return fmt.Errorf("MySQL DSN must use TCP loopback")
		}
		parsedHost, _, err := net.SplitHostPort(config.Addr)
		if err != nil {
			return fmt.Errorf("parse MySQL address: %w", err)
		}
		host = parsedHost
	} else {
		config, err := pgx.ParseConfig(dsn)
		if err != nil {
			return fmt.Errorf("parse PostgreSQL DSN: %w", err)
		}
		host = config.Host
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("%s DSN host %q is not loopback", dialect, host)
	}
	return nil
}

type supplierCapacityQueryRecorder struct {
	mu        sync.Mutex
	durations []time.Duration
	onPage    func(time.Duration)
}

func (recorder *supplierCapacityQueryRecorder) LogMode(logger.LogLevel) logger.Interface {
	return recorder
}
func (recorder *supplierCapacityQueryRecorder) Info(context.Context, string, ...interface{})  {}
func (recorder *supplierCapacityQueryRecorder) Warn(context.Context, string, ...interface{})  {}
func (recorder *supplierCapacityQueryRecorder) Error(context.Context, string, ...interface{}) {}
func (recorder *supplierCapacityQueryRecorder) Trace(_ context.Context, begin time.Time, fc func() (string, int64), _ error) {
	statement, _ := fc()
	lower := strings.ToLower(statement)
	if !strings.Contains(lower, "from") || !strings.Contains(lower, "logs") || !strings.Contains(lower, "order by") {
		return
	}
	duration := time.Since(begin)
	recorder.mu.Lock()
	recorder.durations = append(recorder.durations, duration)
	recorder.mu.Unlock()
	if recorder.onPage != nil {
		recorder.onPage(duration)
	}
}

func (recorder *supplierCapacityQueryRecorder) Durations() []time.Duration {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]time.Duration(nil), recorder.durations...)
}

func cleanupSupplierT1CapacityCase(t *testing.T, db *gorm.DB, day time.Time) {
	t.Helper()
	require.NoError(t, db.Where("created_at >= ? AND created_at < ?", day.AddDate(0, 0, -1).Unix(), day.AddDate(0, 0, 1).Unix()).Delete(&model.Log{}).Error)
	require.NoError(t, db.Where("batch_date = ?", day.Format("2006-01-02")).Delete(&model.SupplierUsageDailySummary{}).Error)
	require.NoError(t, db.Where("batch_date = ?", day.Format("2006-01-02")).Delete(&model.SupplierUsageDailyBatchRun{}).Error)
	require.NoError(t, db.Delete(&model.Option{Key: model.SupplierAccountingActivationOptionKey}).Error)
}

func persistSupplierCapacityActivation(t *testing.T, db *gorm.DB, day time.Time) {
	t.Helper()
	cutover := day.Unix()
	preparedAt := cutover - 1
	preparedBy := 7
	activation := model.SupplierAccountingActivationState{
		SchemaVersion: 1, StateVersion: 3, Phase: model.SupplierAccountingActivationActive,
		CutoverAt: &cutover, AcceptedCapabilityVersions: []int{1}, PreparedAt: &preparedAt,
		PreparedBy: &preparedBy, ActivatedAt: &cutover, Reason: "T+1 capacity",
	}
	encoded, err := common.Marshal(activation)
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Option{Key: model.SupplierAccountingActivationOptionKey, Value: string(encoded)}).Error)
}

func supplierCapacityPayloads(t *testing.T, day time.Time) (string, string) {
	t.Helper()
	official, sales, procurement, profit := int64(1_000), int64(700), int64(650), int64(50)
	salesMultiplier := int64(700_000)
	base := types.SupplierAccountingLogSnapshotV1{
		BindingVersionId: 1, SupplierId: 1, ContractId: 1, RateVersionId: 1, ProcurementMultiplierPpm: 650_000,
		SalesMultiplierPpm: &salesMultiplier, OfficialListMicroUsd: &official, SalesMicroUsd: &sales,
		ProcurementCostMicroUsd: &procurement, GrossProfitMicroUsd: &profit,
		StatisticsScope: string(types.SupplierStatisticsScopeBusiness), ExclusionDecision: "included", FinanciallyCommittedAt: day.Add(time.Hour).Unix(),
		PricingProvenance: &types.SupplierPricingProvenanceV1{Ratio: &types.SupplierRatioPricingProvenanceV1{ModelRatioPpm: 1_000_000, GroupRatioPpm: 700_000, ModelRatioVersion: 1, GroupRatioVersion: 1}},
	}
	encode := func(snapshot types.SupplierAccountingLogSnapshotV1) string {
		payload, err := common.Marshal(map[string]any{types.SupplierAccountingEnvelopeKeyV1: types.SupplierAccountingEnvelopeV1{
			EnvelopeSchemaVersion: types.SupplierAccountingEnvelopeSchemaVersionV1, ProducerCapabilityVersion: types.SupplierAccountingProducerCapabilityV1,
			ActivationStateVersion: 3, Disposition: types.SupplierAccountingDispositionCaptured, Captured: &snapshot,
		}})
		require.NoError(t, err)
		return string(payload)
	}
	business := encode(base)
	ruleID := 99
	base.StatisticsScope = string(types.SupplierStatisticsScopeInternal)
	base.ExclusionDecision = "excluded"
	base.ExclusionRuleId = &ruleID
	base.SalesMultiplierPpm = nil
	base.SalesMicroUsd = nil
	base.GrossProfitMicroUsd = nil
	return business, encode(base)
}

func insertSupplierCapacityLogs(t *testing.T, db *sql.DB, dialect string, day time.Time, totalRows int64, businessPayload, internalPayload string) (int64, int64) {
	t.Helper()
	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	query := "INSERT INTO logs(type, created_at, channel_id, model_name, other) VALUES (?, ?, ?, ?, ?)"
	if dialect == "postgres" {
		query = "INSERT INTO logs(type, created_at, channel_id, model_name, other) VALUES ($1, $2, $3, $4, $5)"
	}
	statement, err := tx.Prepare(query)
	require.NoError(t, err)
	defer func() { require.NoError(t, statement.Close()) }()
	var internalRows int64
	var businessRows int64
	for row := int64(0); row < totalRows; row++ {
		_, err = statement.Exec(model.LogTypeConsume, day.AddDate(0, 0, -1).Unix()+1+row/1000, 2, "", internalPayload)
		require.NoError(t, err)
	}
	for row := int64(0); row < totalRows; row++ {
		payload := internalPayload
		channelID := 2
		modelName := ""
		if row%20 == 0 {
			payload = businessPayload
			channelID = 1
			modelName = "capacity-business"
			businessRows++
		} else {
			internalRows++
		}
		_, err = statement.Exec(model.LogTypeConsume, day.Unix()+1+row/1000, channelID, modelName, payload)
		require.NoError(t, err)
	}
	require.NoError(t, tx.Commit())
	return internalRows, businessRows
}

func requireSupplierCapacityDatabase(t *testing.T, db *gorm.DB, dialect, expected, isolationToken string) {
	t.Helper()
	query := "SELECT DATABASE()"
	if dialect == "postgres" {
		query = "SELECT current_database()"
	}
	var databaseName string
	require.NoError(t, db.Raw(query).Scan(&databaseName).Error)
	require.Equal(t, expected, databaseName, "capacity test refuses a non-isolated database")
	var sentinelCount int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM "+supplierCapacitySentinel+" WHERE token = ?", isolationToken).Scan(&sentinelCount).Error, "externally provision the capacity sentinel before running destructive tests")
	require.Equal(t, int64(1), sentinelCount, "capacity sentinel token mismatch")
}

func supplierCapacityExplainPlan(t *testing.T, db *sql.DB, dialect string, startAt, endAt int64) string {
	t.Helper()
	query := "EXPLAIN FORMAT=JSON SELECT id, created_at, channel_id, model_name, other FROM logs WHERE type = ? AND created_at >= ? AND created_at < ? ORDER BY created_at ASC, id ASC LIMIT 5000"
	if dialect == "postgres" {
		query = "EXPLAIN (FORMAT JSON) SELECT id, created_at, channel_id, model_name, other FROM logs WHERE type = $1 AND created_at >= $2 AND created_at < $3 ORDER BY created_at ASC, id ASC LIMIT 5000"
	}
	var plan string
	require.NoError(t, db.QueryRow(query, model.LogTypeConsume, startAt, endAt).Scan(&plan))
	return plan
}

func supplierCapacityPlanHasFullScan(dialect, plan string) bool {
	lower := strings.ToLower(plan)
	if dialect == "postgres" {
		return strings.Contains(lower, `"node type": "seq scan"`)
	}
	return strings.Contains(lower, `"access_type": "all"`)
}

func supplierCapacityWaitingLocks(t *testing.T, db *gorm.DB, dialect string) int64 {
	t.Helper()
	if dialect == "postgres" {
		var waiting int64
		require.NoError(t, db.Raw("SELECT COUNT(*) FROM pg_locks WHERE NOT granted").Scan(&waiting).Error)
		return waiting
	}
	type statusRow struct {
		Value int64 `gorm:"column:Value"`
	}
	var row statusRow
	require.NoError(t, db.Raw("SHOW GLOBAL STATUS LIKE 'Innodb_row_lock_current_waits'").Scan(&row).Error)
	return row.Value
}

type supplierRSSSampler struct {
	baseline int64
	peak     atomic.Int64
}

func (sampler *supplierRSSSampler) Baseline() int64 { return sampler.baseline }
func (sampler *supplierRSSSampler) Peak() int64     { return sampler.peak.Load() }

func sampleSupplierCapacityRSS() (*supplierRSSSampler, func()) {
	baseline, ok := currentSupplierCapacityRSS()
	if !ok {
		return nil, func() {}
	}
	sampler := &supplierRSSSampler{baseline: baseline}
	sampler.peak.Store(baseline)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if rss, available := currentSupplierCapacityRSS(); available {
					for current := sampler.peak.Load(); rss > current && !sampler.peak.CompareAndSwap(current, rss); current = sampler.peak.Load() {
					}
				}
			}
		}
	}()
	return sampler, func() { close(stop); <-done }
}

func currentSupplierCapacityRSS() (int64, bool) {
	if runtime.GOOS != "linux" {
		return 0, false
	}
	content, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "VmRSS:" && fields[2] == "kB" {
			value, parseErr := strconv.ParseInt(fields[1], 10, 64)
			return value * 1024, parseErr == nil
		}
	}
	return 0, false
}

func supplierCapacityPercentile(sorted []time.Duration, percentile int) time.Duration {
	index := (len(sorted)*percentile + 99) / 100
	if index < 1 {
		index = 1
	}
	return sorted[index-1]
}

func supplierCapacityDurationsMicros(durations []time.Duration) []int64 {
	result := make([]int64, len(durations))
	for index, duration := range durations {
		result[index] = duration.Microseconds()
	}
	return result
}

func supplierCapacityDatabaseVersion(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var version string
	require.NoError(t, db.Raw("SELECT VERSION()").Scan(&version).Error)
	return version
}

func supplierCapacityCommit(t *testing.T) string {
	t.Helper()
	output, err := exec.Command("git", "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(output))
}

func supplierCapacityWorkingTreeDirty(t *testing.T) bool {
	t.Helper()
	output, err := exec.Command("git", "status", "--porcelain=v1").Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(output)) != ""
}

func writeSupplierCapacityEvidence(t *testing.T, evidence supplierT1CapacityEvidence) {
	t.Helper()
	directory := strings.TrimSpace(os.Getenv(supplierCapacityEvidenceDir))
	if directory == "" {
		return
	}
	require.NoError(t, os.MkdirAll(directory, 0o755))
	payload, err := common.Marshal(evidence)
	require.NoError(t, err)
	path := fmt.Sprintf("%s/supplier-t1-%s.json", strings.TrimRight(directory, "/"), evidence.Database)
	require.NoError(t, os.WriteFile(path, append(payload, '\n'), 0o644))
}
