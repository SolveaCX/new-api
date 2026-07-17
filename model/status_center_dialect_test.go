package model

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type statusDialectTestDB struct {
	db     *gorm.DB
	sqlDB  *sql.DB
	prefix string
}

func TestStatusCenterSQLiteDialect(t *testing.T) {
	harness := setupStatusDialectTestDB(t, "sqlite", "")
	runStatusDialectSuite(t, harness)
}

func TestStatusCenterMySQLDialect(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run MySQL status center dialect integration tests")
	}
	harness := setupStatusDialectTestDB(t, "mysql", dsn)
	runStatusDialectSuite(t, harness)
}

func TestStatusCenterPostgreSQLDialect(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set TEST_POSTGRES_DSN to run PostgreSQL status center dialect integration tests")
	}
	harness := setupStatusDialectTestDB(t, "postgres", dsn)
	runStatusDialectSuite(t, harness)
}

func setupStatusDialectTestDB(t *testing.T, dialect string, dsn string) *statusDialectTestDB {
	t.Helper()

	originalDB := DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL

	var (
		db  *gorm.DB
		err error
	)
	switch dialect {
	case "sqlite":
		path := filepath.ToSlash(filepath.Join(t.TempDir(), "status-center.db"))
		dsn = "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	case "mysql":
		db, err = gorm.Open(mysql.Open(ensureMySQLDSNDefaults(dsn)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	case "postgres":
		db, err = gorm.Open(postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true,
		}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	default:
		t.Fatalf("unsupported status center test dialect %q", dialect)
	}
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(8)
	require.NoError(t, sqlDB.Ping())

	DB = db
	common.UsingSQLite = dialect == "sqlite"
	common.UsingMySQL = dialect == "mysql"
	common.UsingPostgreSQL = dialect == "postgres"

	prefix := fmt.Sprintf("status_dialect_%s_%d", dialect, time.Now().UnixNano())
	harness := &statusDialectTestDB{db: db, sqlDB: sqlDB, prefix: prefix}
	t.Cleanup(func() {
		cleanupStatusDialectRows(harness)
		_ = sqlDB.Close()
		DB = originalDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
	})

	models := append(StatusCenterModels(), &ModelAvailabilityState{})
	require.NoError(t, db.AutoMigrate(models...))
	for _, current := range models {
		require.Truef(t, db.Migrator().HasTable(current), "expected %T migration for %s", current, dialect)
	}

	if dialect == "sqlite" {
		var journalMode string
		require.NoError(t, db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error)
		require.Equal(t, "wal", strings.ToLower(journalMode))
		var busyTimeout int
		require.NoError(t, db.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error)
		require.Equal(t, 5000, busyTimeout)
	}

	return harness
}

func cleanupStatusDialectRows(harness *statusDialectTestDB) {
	if harness == nil || harness.db == nil {
		return
	}

	likePrefix := harness.prefix + "%"
	var componentIDs []int64
	_ = harness.db.Model(&StatusComponent{}).
		Where("component_key LIKE ?", likePrefix).
		Pluck("id", &componentIDs).Error
	if len(componentIDs) > 0 {
		_ = harness.db.Where("component_id IN ?", componentIDs).Delete(&StatusProbeResult{}).Error
		_ = harness.db.Where("component_id IN ?", componentIDs).Delete(&StatusPeriod{}).Error
		_ = harness.db.Where("id IN ?", componentIDs).Delete(&StatusComponent{}).Error
	}
	_ = harness.db.Where("model_name LIKE ?", likePrefix).Delete(&ModelAvailabilityState{}).Error
	_ = harness.db.Where("name LIKE ?", likePrefix).Delete(&StatusJobLease{}).Error
}

func runStatusDialectSuite(t *testing.T, harness *statusDialectTestDB) {
	t.Helper()

	const (
		leaseNow     = int64(10_000)
		leaseSeconds = int64(30)
		workers      = 8
	)
	jobName := harness.prefix + "_lease"
	start := make(chan struct{})
	results := make(chan struct {
		lease    StatusJobLease
		acquired bool
		err      error
	}, workers)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			lease, acquired, err := AcquireStatusJobLease(jobName, fmt.Sprintf("%s_holder_%d", harness.prefix, worker), leaseNow, leaseSeconds)
			results <- struct {
				lease    StatusJobLease
				acquired bool
				err      error
			}{lease: lease, acquired: acquired, err: err}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)

	acquiredCount := 0
	for result := range results {
		require.NoError(t, result.err)
		if result.acquired {
			acquiredCount++
		}
	}
	require.Equal(t, 1, acquiredCount, "exactly one concurrent lease contender must become owner")
	require.GreaterOrEqual(t, harness.sqlDB.Stats().OpenConnections, 2, "lease contention must use a multi-connection pool")

	var lease StatusJobLease
	require.NoError(t, harness.db.First(&lease, "name = ?", jobName).Error)
	require.EqualValues(t, 1, lease.FencingToken)

	component := StatusComponent{
		ComponentKey:    harness.prefix + "_router",
		Slug:            harness.prefix + "-router",
		Kind:            StatusComponentKindRouter,
		DisplayName:     "Router",
		Lifecycle:       StatusLifecycleActive,
		ObservedStatus:  StatusUnknown,
		EffectiveStatus: StatusUnknown,
		Version:         1,
		CreatedAt:       leaseNow,
		UpdatedAt:       leaseNow,
	}
	require.NoError(t, harness.db.Create(&component).Error)

	component.DisplayName = "Active fenced component"
	require.NoError(t, CommitStatusComponentWithFence(jobName, lease.Holder, lease.FencingToken, leaseNow+1, &component))

	activeProbe := StatusProbeResult{
		ComponentID: component.ID,
		Success:     true,
		TargetRef:   harness.prefix + "_active_probe",
		CreatedAt:   leaseNow + 1,
	}
	require.NoError(t, CreateStatusProbeResultWithFence(jobName, lease.Holder, lease.FencingToken, leaseNow+1, &activeProbe))

	period := StatusPeriod{
		ComponentID:      component.ID,
		Granularity:      StatusGranularityHour,
		PeriodStart:      leaseNow,
		ScoreSumMicros:   900_000,
		KnownBucketCount: 1,
		WorstStatus:      StatusDegraded,
		CreatedAt:        leaseNow,
		UpdatedAt:        leaseNow,
	}
	require.NoError(t, UpsertStatusPeriodWithFence(jobName, lease.Holder, lease.FencingToken, leaseNow+1, &period))
	period.ScoreSumMicros = 1_900_000
	period.KnownBucketCount = 2
	period.WorstStatus = StatusOperational
	period.UpdatedAt = leaseNow + 1
	require.NoError(t, UpsertStatusPeriodWithFence(jobName, lease.Holder, lease.FencingToken, leaseNow+1, &period))

	availability := ModelAvailabilityState{
		ModelName:     harness.prefix + "_model",
		Status:        ModelAvailabilityAvailable,
		LastCheckedAt: leaseNow + 1,
	}
	require.NoError(t, SaveModelAvailabilityStateWithFence(jobName, lease.Holder, lease.FencingToken, leaseNow+1, &availability))

	var periodCount int64
	require.NoError(t, harness.db.Model(&StatusPeriod{}).
		Where("component_id = ? AND granularity = ? AND period_start = ?", component.ID, period.Granularity, period.PeriodStart).
		Count(&periodCount).Error)
	require.EqualValues(t, 1, periodCount)
	var storedPeriod StatusPeriod
	require.NoError(t, harness.db.First(&storedPeriod, "component_id = ? AND granularity = ? AND period_start = ?", component.ID, period.Granularity, period.PeriodStart).Error)
	require.EqualValues(t, 1_900_000, storedPeriod.ScoreSumMicros)
	require.EqualValues(t, 2, storedPeriod.KnownBucketCount)
	require.Equal(t, StatusOperational, storedPeriod.WorstStatus)

	expiredAt := leaseNow + leaseSeconds
	staleComponent := component
	staleComponent.DisplayName = "Expired fenced component"
	require.Error(t, CommitStatusComponentWithFence(jobName, lease.Holder, lease.FencingToken, expiredAt, &staleComponent))
	expiredProbe := StatusProbeResult{
		ComponentID: component.ID,
		Success:     false,
		TargetRef:   harness.prefix + "_expired_probe",
		CreatedAt:   expiredAt,
	}
	require.Error(t, CreateStatusProbeResultWithFence(jobName, lease.Holder, lease.FencingToken, expiredAt, &expiredProbe))
	expiredPeriod := period
	expiredPeriod.ScoreSumMicros = 0
	expiredPeriod.KnownBucketCount = 0
	expiredPeriod.WorstStatus = StatusOutage
	require.Error(t, UpsertStatusPeriodWithFence(jobName, lease.Holder, lease.FencingToken, expiredAt, &expiredPeriod))
	expiredAvailability := availability
	expiredAvailability.Status = ModelAvailabilityTemporaryFailure
	expiredAvailability.LastCheckedAt = expiredAt
	require.Error(t, SaveModelAvailabilityStateWithFence(jobName, lease.Holder, lease.FencingToken, expiredAt, &expiredAvailability))

	var storedComponent StatusComponent
	require.NoError(t, harness.db.First(&storedComponent, component.ID).Error)
	require.Equal(t, "Active fenced component", storedComponent.DisplayName)
	var expiredProbeCount int64
	require.NoError(t, harness.db.Model(&StatusProbeResult{}).Where("target_ref = ?", expiredProbe.TargetRef).Count(&expiredProbeCount).Error)
	require.Zero(t, expiredProbeCount)
	require.NoError(t, harness.db.First(&storedPeriod, "component_id = ? AND granularity = ? AND period_start = ?", component.ID, period.Granularity, period.PeriodStart).Error)
	require.EqualValues(t, 1_900_000, storedPeriod.ScoreSumMicros)
	require.Equal(t, StatusOperational, storedPeriod.WorstStatus)
	var storedAvailability ModelAvailabilityState
	require.NoError(t, harness.db.First(&storedAvailability, "model_name = ?", availability.ModelName).Error)
	require.Equal(t, ModelAvailabilityAvailable, storedAvailability.Status)
	require.EqualValues(t, leaseNow+1, storedAvailability.LastCheckedAt)
	var unchangedLease StatusJobLease
	require.NoError(t, harness.db.First(&unchangedLease, "name = ?", jobName).Error)
	require.Equal(t, lease.Holder, unchangedLease.Holder)
	require.Equal(t, lease.FencingToken, unchangedLease.FencingToken)
	require.Equal(t, lease.ExpiresAt, unchangedLease.ExpiresAt)

	testStatusRetentionWithFence(t, harness, component.ID)
}

func testStatusRetentionWithFence(t *testing.T, harness *statusDialectTestDB, componentID int64) {
	t.Helper()

	const (
		now             = int64(20_000)
		rawCutoff       = int64(-200)
		aggregateCutoff = int64(-400)
	)
	jobName := harness.prefix + "_retention"
	lease, acquired, err := AcquireStatusJobLease(jobName, harness.prefix+"_retention_holder", now, 100)
	require.NoError(t, err)
	require.True(t, acquired)

	oldProbe := StatusProbeResult{ComponentID: componentID, Success: false, TargetRef: harness.prefix + "_old_probe", CreatedAt: -300}
	keptProbe := StatusProbeResult{ComponentID: componentID, Success: true, TargetRef: harness.prefix + "_kept_probe", CreatedAt: -100}
	require.NoError(t, harness.db.Create(&oldProbe).Error)
	require.NoError(t, harness.db.Create(&keptProbe).Error)

	periods := []*StatusPeriod{
		{ComponentID: componentID, Granularity: StatusGranularityFiveMinutes, PeriodStart: -300, CreatedAt: -300},
		{ComponentID: componentID, Granularity: StatusGranularityFiveMinutes, PeriodStart: -100, CreatedAt: -100},
		{ComponentID: componentID, Granularity: StatusGranularityHour, PeriodStart: -500, CreatedAt: -500},
		{ComponentID: componentID, Granularity: StatusGranularityHour, PeriodStart: -100, CreatedAt: -100},
		{ComponentID: componentID, Granularity: StatusGranularityDay, PeriodStart: -500, CreatedAt: -500},
		{ComponentID: componentID, Granularity: StatusGranularityDay, PeriodStart: -100, CreatedAt: -100},
	}
	for _, period := range periods {
		require.NoError(t, harness.db.Create(period).Error)
	}

	assertNoUnrelatedRetentionRows(t, harness, componentID, rawCutoff, aggregateCutoff)
	require.NoError(t, DeleteStatusHistoryWithFence(jobName, lease.Holder, lease.FencingToken, now+1, rawCutoff, aggregateCutoff))

	requireStatusRowCount(t, harness.db, &StatusProbeResult{}, oldProbe.ID, 0)
	requireStatusRowCount(t, harness.db, &StatusProbeResult{}, keptProbe.ID, 1)
	for index, period := range periods {
		want := int64(1)
		if index == 0 || index == 2 || index == 4 {
			want = 0
		}
		requireStatusRowCount(t, harness.db, &StatusPeriod{}, period.ID, want)
	}
}

func assertNoUnrelatedRetentionRows(t *testing.T, harness *statusDialectTestDB, componentID int64, rawCutoff int64, aggregateCutoff int64) {
	t.Helper()

	var count int64
	require.NoError(t, harness.db.Model(&StatusProbeResult{}).
		Where("component_id <> ? AND created_at < ?", componentID, rawCutoff).
		Count(&count).Error)
	require.Zero(t, count, "refusing to run retention against unrelated probe rows")
	require.NoError(t, harness.db.Model(&StatusPeriod{}).
		Where("component_id <> ?", componentID).
		Where("(granularity = ? AND period_start < ?) OR (granularity IN ? AND period_start < ?)",
			StatusGranularityFiveMinutes, rawCutoff,
			[]string{StatusGranularityHour, StatusGranularityDay}, aggregateCutoff).
		Count(&count).Error)
	require.Zero(t, count, "refusing to run retention against unrelated period rows")
}

func requireStatusRowCount(t *testing.T, db *gorm.DB, model any, id int64, want int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(model).Where("id = ?", id).Count(&count).Error)
	require.Equal(t, want, count)
}
