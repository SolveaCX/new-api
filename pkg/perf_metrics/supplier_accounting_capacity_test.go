package perfmetrics

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
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
	mysqlconfig "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	supplierAccountingCapacityExpectedRouterMax = 30
	supplierCapacityIsolationTokenEnv           = "SUPPLIER_CAPACITY_ISOLATION_TOKEN"
	supplierCapacityEvidenceDirEnv              = "SUPPLIER_CAPACITY_EVIDENCE_DIR"
	supplierCapacitySentinelTable               = "supplier_capacity_test_sentinel"
)

type supplierObserverDatabaseEvidence struct {
	Dialect                string  `json:"dialect"`
	Version                string  `json:"version"`
	BurstMicroseconds      int64   `json:"burst_us"`
	P50Microseconds        int64   `json:"p50_us"`
	P95Microseconds        int64   `json:"p95_us"`
	P99Microseconds        int64   `json:"p99_us"`
	MaxMicroseconds        int64   `json:"max_us"`
	DurationsMicroseconds  []int64 `json:"durations_us"`
	PoolOpenBefore         int     `json:"pool_open_before"`
	PoolOpenAfter          int     `json:"pool_open_after"`
	PoolWaitDelta          int64   `json:"pool_wait_delta"`
	PoolWaitDurationMicros int64   `json:"pool_wait_duration_us"`
	DBConnectionsBefore    int64   `json:"db_connections_before"`
	DBConnectionsAfter     int64   `json:"db_connections_after"`
	WaitingLocksBefore     int64   `json:"waiting_locks_before"`
	WaitingLocksAfter      int64   `json:"waiting_locks_after"`
}

type supplierObserverCapacityEvidence struct {
	SchemaVersion                    int                                `json:"schema_version"`
	EvidenceClass                    string                             `json:"evidence_class"`
	GeneratedAt                      string                             `json:"generated_at"`
	Commit                           string                             `json:"commit"`
	WorkingTreeDirty                 bool                               `json:"working_tree_dirty"`
	ConfiguredRouterMax              int                                `json:"configured_router_max"`
	ConfiguredRouterMaxSource        string                             `json:"configured_router_max_source"`
	ConfiguredRouterMaxSourceSHA256  string                             `json:"configured_router_max_source_sha256"`
	StressRouterCount                int                                `json:"stress_router_count"`
	StressMultiple                   int                                `json:"stress_multiple"`
	CoverageDays                     int                                `json:"coverage_days"`
	DBReadsPerColdRefresh            int                                `json:"db_reads_per_cold_refresh"`
	DBReadsTotal                     int                                `json:"db_reads_total"`
	PrometheusSeriesPerRouter        int                                `json:"prometheus_series_per_router"`
	PrometheusSeriesConfiguredMax    int                                `json:"prometheus_series_configured_max"`
	PrometheusSeriesStress           int                                `json:"prometheus_series_stress"`
	SharedPoolDBConcurrencyEmulation bool                               `json:"shared_pool_db_concurrency_emulation"`
	AggregationContract              string                             `json:"aggregation_contract"`
	HostOS                           string                             `json:"host_os"`
	HostArch                         string                             `json:"host_arch"`
	HostCPUCount                     int                                `json:"host_cpu_count"`
	HostGOMAXPROCS                   int                                `json:"host_gomaxprocs"`
	HostGoVersion                    string                             `json:"host_go_version"`
	Databases                        []supplierObserverDatabaseEvidence `json:"databases"`
	UnavailableFields                []string                           `json:"unavailable_fields"`
	ReleaseBlockers                  []string                           `json:"release_blockers"`
}

func TestSupplierAccountingConfiguredRouterMaxSource(t *testing.T) {
	value, source, digest := supplierAccountingConfiguredRouterMax(t)
	require.Equal(t, supplierAccountingCapacityExpectedRouterMax, value)
	require.Equal(t, "deploy/gcp/envs/prod/terraform.tfvars", source)
	require.Regexp(t, `^[0-9a-f]{64}$`, digest)
}

func TestSupplierObserverCapacityLoopbackDSNValidation(t *testing.T) {
	require.NoError(t, supplierCapacityRequireLoopbackDSN("mysql", "root:password@tcp(127.0.0.1:3306)/supplier_g009_mysql"))
	require.NoError(t, supplierCapacityRequireLoopbackDSN("postgres", "host=localhost port=5432 user=postgres dbname=supplier_g009_postgres"))
	require.Error(t, supplierCapacityRequireLoopbackDSN("mysql", "root:password@tcp(db.internal:3306)/supplier_g009_mysql"))
	require.Error(t, supplierCapacityRequireLoopbackDSN("postgres", "host=10.0.0.8 port=5432 user=postgres dbname=supplier_g009_postgres"))
}

func TestSupplierAccountingBacklogMaximumRouterBurstCapacity(t *testing.T) {
	if os.Getenv("RUN_SUPPLIER_CAPACITY_TEST") != "1" {
		t.Skip("set RUN_SUPPLIER_CAPACITY_TEST=1 with both isolated loopback DSNs and a provisioned sentinel token")
	}
	routerMax, routerMaxSource, routerMaxSourceSHA := supplierAccountingConfiguredRouterMax(t)
	require.Equal(t, supplierAccountingCapacityExpectedRouterMax, routerMax, "review the capacity contract when production Router max changes")
	stressRouterCount := 2 * routerMax
	isolationToken := strings.TrimSpace(os.Getenv(supplierCapacityIsolationTokenEnv))
	require.NotEmpty(t, isolationToken, "%s is required when the capacity gate is enabled", supplierCapacityIsolationTokenEnv)
	require.NotEmpty(t, strings.TrimSpace(os.Getenv(supplierCapacityEvidenceDirEnv)), "%s is required when the capacity gate is enabled", supplierCapacityEvidenceDirEnv)

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

	databaseEvidence := make([]supplierObserverDatabaseEvidence, 0, len(tests))
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(testCase.dsnEnv))
			db, err := gorm.Open(testCase.open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			requireSupplierAccountingCapacityDatabase(t, db, testCase.name, testCase.expectedDatabase, isolationToken)
			require.NoError(t, db.AutoMigrate(&model.Option{}, &model.SupplierUsageDailyBatchRun{}))
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(stressRouterCount)
			sqlDB.SetMaxIdleConns(stressRouterCount)

			location, err := time.LoadLocation("Asia/Shanghai")
			require.NoError(t, err)
			now := time.Now().In(location)
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
			coverageStart := today.AddDate(0, 0, -365)
			through := today.AddDate(0, 0, -1)
			startDate, throughDate := coverageStart.Format("2006-01-02"), through.Format("2006-01-02")
			cleanup := func() {
				require.NoError(t, db.Where("batch_date >= ? AND batch_date <= ?", startDate, throughDate).Delete(&model.SupplierUsageDailyBatchRun{}).Error)
				require.NoError(t, db.Delete(&model.Option{Key: model.SupplierAccountingActivationOptionKey}).Error)
			}
			cleanup()
			t.Cleanup(cleanup)
			runs := make([]model.SupplierUsageDailyBatchRun, 0, 365)
			for day := coverageStart; day.Before(today); day = day.AddDate(0, 0, 1) {
				runs = append(runs, model.SupplierUsageDailyBatchRun{BatchDate: day.Format("2006-01-02"), DayStart: day.Unix(), DayEnd: day.AddDate(0, 0, 1).Unix(), Status: model.SupplierDailyBatchStatusCompleted, FenceToken: 1, PublishedFenceToken: 1})
			}
			require.NoError(t, db.CreateInBatches(&runs, 200).Error)
			preparedAt, preparedBy, cutoverAt := coverageStart.Unix()-1, 7, coverageStart.Unix()
			activation := model.SupplierAccountingActivationState{SchemaVersion: 1, StateVersion: 3, Phase: model.SupplierAccountingActivationActive, CutoverAt: &cutoverAt, AcceptedCapabilityVersions: []int{1}, PreparedAt: &preparedAt, PreparedBy: &preparedBy, ActivatedAt: &cutoverAt, Reason: "capacity cold refresh"}
			encoded, err := common.Marshal(activation)
			require.NoError(t, err)
			require.NoError(t, db.Create(&model.Option{Key: model.SupplierAccountingActivationOptionKey, Value: string(encoded)}).Error)

			var queryCallbacks atomic.Int64
			countQuery := func(*gorm.DB) { queryCallbacks.Add(1) }
			require.NoError(t, db.Callback().Query().Before("gorm:query").Register("supplier_capacity_query", countQuery))
			require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register("supplier_capacity_raw", countQuery))
			require.NoError(t, db.Callback().Row().Before("gorm:row").Register("supplier_capacity_row", countQuery))
			beforePool := sqlDB.Stats()
			beforeConnections, beforeWaitingLocks := supplierAccountingCapacityDatabaseStats(t, db, testCase.name)
			queryCallbacks.Store(0)
			start := make(chan struct{})
			durations := make([]time.Duration, stressRouterCount)
			errs := make([]error, stressRouterCount)
			var wait sync.WaitGroup
			for index := range stressRouterCount {
				wait.Add(1)
				go func() {
					defer wait.Done()
					<-start
					beganAt := time.Now()
					_, errs[index] = refreshSupplierAccountingBacklogPrometheusSnapshot(context.Background(), db)
					durations[index] = time.Since(beganAt)
				}()
			}
			burstStartedAt := time.Now()
			close(start)
			wait.Wait()
			burstDuration := time.Since(burstStartedAt)
			for _, refreshErr := range errs {
				require.NoError(t, refreshErr)
			}
			observerQueries := queryCallbacks.Load()
			require.Equal(t, int64(stressRouterCount*3), observerQueries)
			afterPool := sqlDB.Stats()
			afterConnections, afterWaitingLocks := supplierAccountingCapacityDatabaseStats(t, db, testCase.name)
			require.Zero(t, afterPool.WaitCount-beforePool.WaitCount)
			require.Zero(t, beforeWaitingLocks)
			require.Zero(t, afterWaitingLocks)
			sort.Slice(durations, func(left, right int) bool { return durations[left] < durations[right] })
			durationsMicros := make([]int64, len(durations))
			for index, duration := range durations {
				durationsMicros[index] = duration.Microseconds()
			}
			databaseEvidence = append(databaseEvidence, supplierObserverDatabaseEvidence{Dialect: testCase.name, Version: supplierAccountingCapacityDatabaseVersion(t, db), BurstMicroseconds: burstDuration.Microseconds(), P50Microseconds: supplierAccountingCapacityPercentile(durations, 50).Microseconds(), P95Microseconds: supplierAccountingCapacityPercentile(durations, 95).Microseconds(), P99Microseconds: supplierAccountingCapacityPercentile(durations, 99).Microseconds(), MaxMicroseconds: durations[len(durations)-1].Microseconds(), DurationsMicroseconds: durationsMicros, PoolOpenBefore: beforePool.OpenConnections, PoolOpenAfter: afterPool.OpenConnections, PoolWaitDelta: afterPool.WaitCount - beforePool.WaitCount, PoolWaitDurationMicros: (afterPool.WaitDuration - beforePool.WaitDuration).Microseconds(), DBConnectionsBefore: beforeConnections, DBConnectionsAfter: afterConnections, WaitingLocksBefore: beforeWaitingLocks, WaitingLocksAfter: afterWaitingLocks})
		})
	}

	evidence := supplierObserverCapacityEvidence{SchemaVersion: 2, EvidenceClass: "generated_local_synthetic_not_production_equivalent", GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Commit: supplierCapacityGitOutput(t, "rev-parse", "HEAD"), WorkingTreeDirty: supplierCapacityGitOutput(t, "status", "--porcelain=v1") != "", ConfiguredRouterMax: routerMax, ConfiguredRouterMaxSource: routerMaxSource, ConfiguredRouterMaxSourceSHA256: routerMaxSourceSHA, StressRouterCount: stressRouterCount, StressMultiple: 2, CoverageDays: 365, DBReadsPerColdRefresh: 3, DBReadsTotal: stressRouterCount * 3, PrometheusSeriesPerRouter: 5, PrometheusSeriesConfiguredMax: routerMax * 5, PrometheusSeriesStress: stressRouterCount * 5, SharedPoolDBConcurrencyEmulation: true, AggregationContract: "service-level REDUCE_MAX per gauge; never sum instances", HostOS: runtime.GOOS, HostArch: runtime.GOARCH, HostCPUCount: runtime.NumCPU(), HostGOMAXPROCS: runtime.GOMAXPROCS(0), HostGoVersion: runtime.Version(), Databases: databaseEvidence, UnavailableFields: []string{"production_cloud_sql_cpu", "production_cloud_sql_connections", "production_cloud_sql_lock_distribution", "independent_one_connection_pool_per_router", "authenticated_http_get_metrics_latency", "prometheus_render_latency", "approved_latency_threshold", "cloud_monitoring_incident_fire_notification_resolution"}, ReleaseBlockers: []string{"production-equivalent database CPU/connections/locks evidence", "authenticated and rendered /metrics latency at production scale", "approved staging/live fire-and-resolve evidence for every supplier alert condition"}}
	writeSupplierObserverCapacityEvidence(t, evidence)
}

func supplierAccountingConfiguredRouterMax(t *testing.T) (int, string, string) {
	t.Helper()
	root := supplierCapacityGitOutput(t, "rev-parse", "--show-toplevel")
	source := "deploy/gcp/envs/prod/terraform.tfvars"
	content, err := os.ReadFile(filepath.Join(root, source))
	require.NoError(t, err, "capacity gate requires the production Router max source")
	matches := regexp.MustCompile(`(?m)^\s*router_max_instances\s*=\s*([0-9]+)\s*(?://.*)?$`).FindAllSubmatch(content, -1)
	require.Len(t, matches, 1, "capacity gate requires exactly one parseable router_max_instances")
	match := matches[0]
	value, err := strconv.Atoi(string(match[1]))
	require.NoError(t, err)
	digest := sha256.Sum256(content)
	return value, source, fmt.Sprintf("%x", digest)
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

func requireSupplierAccountingCapacityDatabase(t *testing.T, db *gorm.DB, dialect, expected, isolationToken string) {
	t.Helper()
	query := "SELECT DATABASE()"
	if dialect == "postgres" {
		query = "SELECT current_database()"
	}
	var databaseName string
	require.NoError(t, db.Raw(query).Scan(&databaseName).Error)
	require.Equal(t, expected, databaseName, "capacity test refuses a non-isolated database")
	var sentinelCount int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM "+supplierCapacitySentinelTable+" WHERE token = ?", isolationToken).Scan(&sentinelCount).Error, "externally provision the capacity sentinel before running destructive tests")
	require.Equal(t, int64(1), sentinelCount, "capacity sentinel token mismatch")
}

func supplierAccountingCapacityDatabaseStats(t *testing.T, db *gorm.DB, dialect string) (int64, int64) {
	t.Helper()
	var connections, waitingLocks int64
	if dialect == "postgres" {
		require.NoError(t, db.Raw("SELECT COUNT(*) FROM pg_stat_activity WHERE datname = current_database()").Scan(&connections).Error)
		require.NoError(t, db.Raw("SELECT COUNT(*) FROM pg_locks WHERE NOT granted").Scan(&waitingLocks).Error)
		return connections, waitingLocks
	}
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM information_schema.processlist WHERE db = DATABASE()").Scan(&connections).Error)
	type statusRow struct {
		VariableName string `gorm:"column:Variable_name"`
		Value        int64  `gorm:"column:Value"`
	}
	var row statusRow
	require.NoError(t, db.Raw("SHOW GLOBAL STATUS LIKE 'Innodb_row_lock_current_waits'").Scan(&row).Error)
	return connections, row.Value
}

func supplierAccountingCapacityDatabaseVersion(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var version string
	require.NoError(t, db.Raw("SELECT VERSION()").Scan(&version).Error)
	return version
}

func supplierAccountingCapacityPercentile(sorted []time.Duration, percentile int) time.Duration {
	index := (len(sorted)*percentile + 99) / 100
	if index < 1 {
		index = 1
	}
	return sorted[index-1]
}

func supplierCapacityGitOutput(t *testing.T, args ...string) string {
	t.Helper()
	output, err := exec.Command("git", args...).Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(output))
}

func writeSupplierObserverCapacityEvidence(t *testing.T, evidence supplierObserverCapacityEvidence) {
	t.Helper()
	directory := strings.TrimSpace(os.Getenv(supplierCapacityEvidenceDirEnv))
	require.NotEmpty(t, directory, "%s is required when the capacity gate is enabled", supplierCapacityEvidenceDirEnv)
	require.NoError(t, os.MkdirAll(directory, 0o755))
	payload, err := common.Marshal(evidence)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(directory, "supplier-observer.json"), append(payload, '\n'), 0o644))
}
