package model

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newSupplierAccountingBacklogObserverTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "supplier-accounting-backlog.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SupplierUsageDailyBatchRun{}))
	return db
}

func supplierAccountingBacklogTestTime(t *testing.T, hour, minute, second int) time.Time {
	t.Helper()
	location, err := time.LoadLocation(supplierDailyBatchTimezone)
	require.NoError(t, err)
	return time.Date(2026, time.July, 24, hour, minute, second, 0, location)
}

func TestObserveSupplierAccountingBacklogUsesExactBoundedZeroFenceSet(t *testing.T) {
	db := newSupplierAccountingBacklogObserverTestDB(t)
	var queryCount atomic.Int64
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register("test:count_backlog_queries", func(*gorm.DB) {
		queryCount.Add(1)
	}))
	now := supplierAccountingBacklogTestTime(t, 8, 0, 0)
	activeSlot := 1
	runs := []SupplierUsageDailyBatchRun{
		{BatchDate: "2026-07-19", DayStart: now.AddDate(0, 0, -5).Unix(), DayEnd: now.AddDate(0, 0, -4).Unix(), Status: SupplierDailyBatchStatusFailed, PublishedFenceToken: 0},
		{BatchDate: "2026-07-20", DayStart: now.AddDate(0, 0, -4).Unix(), DayEnd: now.AddDate(0, 0, -3).Unix(), Status: SupplierDailyBatchStatusCompleted, PublishedFenceToken: 0},
		{BatchDate: "2026-07-21", DayStart: now.AddDate(0, 0, -3).Unix(), DayEnd: now.AddDate(0, 0, -2).Unix(), Status: SupplierDailyBatchStatusRunning, FenceToken: 7, PublishedFenceToken: 7, ActiveLeaseSlot: &activeSlot, LockedUntil: 1<<62 - 1},
		{BatchDate: "2026-07-22", DayStart: now.AddDate(0, 0, -2).Unix(), DayEnd: now.AddDate(0, 0, -1).Unix(), Status: SupplierDailyBatchStatusCompleted, PublishedFenceToken: 0},
		{BatchDate: "2026-07-23", DayStart: now.AddDate(0, 0, -1).Unix(), DayEnd: time.Date(2026, time.July, 24, 0, 0, 0, 0, now.Location()).Unix(), Status: SupplierDailyBatchStatusRunning, FenceToken: 9, PublishedFenceToken: 0, LockedUntil: 1<<62 - 1},
		{BatchDate: "2026-07-24", DayStart: now.Unix(), DayEnd: now.AddDate(0, 0, 1).Unix(), Status: SupplierDailyBatchStatusFailed, PublishedFenceToken: 0},
	}
	require.NoError(t, db.Create(&runs).Error)

	observation, err := observeSupplierAccountingBacklogAt(context.Background(), db, time.Date(2026, time.July, 20, 12, 0, 0, 0, now.Location()).Unix(), now.Unix())
	require.NoError(t, err)
	require.Equal(t, int64(1), queryCount.Load(), "backlog and prior-day publication truth must share one database snapshot")
	require.Equal(t, now.Unix(), observation.ObservedAtUnix)
	require.Equal(t, int64(3), observation.NeverPublishedDays)
	expectedOldestDayEnd := time.Date(2026, time.July, 21, 0, 0, 0, 0, now.Location()).Unix()
	require.Equal(t, expectedOldestDayEnd, observation.OldestNeverPublishedDayEnd)
	require.Equal(t, now.Unix()-expectedOldestDayEnd, observation.OldestNeverPublishedAgeSeconds)
	require.False(t, observation.PriorDayPublished)
	require.True(t, observation.PriorDayUnpublishedAfter0800)

	require.NoError(t, db.Model(&SupplierUsageDailyBatchRun{}).Where("batch_date = ?", "2026-07-23").Updates(map[string]any{
		"status": SupplierDailyBatchStatusFailed, "active_lease_slot": nil, "locked_until": 0, "published_fence_token": 9,
	}).Error)
	observation, err = observeSupplierAccountingBacklogAt(context.Background(), db, runs[1].DayStart, now.Unix())
	require.NoError(t, err)
	require.Equal(t, int64(2), queryCount.Load())
	require.Equal(t, int64(2), observation.NeverPublishedDays)
	require.True(t, observation.PriorDayPublished)
	require.False(t, observation.PriorDayUnpublishedAfter0800)
}

func TestObserveSupplierAccountingBacklogShanghaiBoundaryAndMissingPriorDay(t *testing.T) {
	db := newSupplierAccountingBacklogObserverTestDB(t)
	before := supplierAccountingBacklogTestTime(t, 7, 59, 59)
	atBoundary := supplierAccountingBacklogTestTime(t, 8, 0, 0)
	cutover := time.Date(2026, time.July, 23, 0, 0, 0, 0, before.Location()).Unix()

	observation, err := observeSupplierAccountingBacklogAt(context.Background(), db, cutover, before.Unix())
	require.NoError(t, err)
	require.Equal(t, int64(1), observation.NeverPublishedDays)
	require.Equal(t, atBoundary.Add(-8*time.Hour).Unix(), observation.OldestNeverPublishedDayEnd)
	require.False(t, observation.PriorDayPublished, "a missing prior-day row is not published")
	require.False(t, observation.PriorDayUnpublishedAfter0800)

	observation, err = observeSupplierAccountingBacklogAt(context.Background(), db, cutover, atBoundary.Unix())
	require.NoError(t, err)
	require.False(t, observation.PriorDayPublished)
	require.True(t, observation.PriorDayUnpublishedAfter0800)
}

func TestObserveSupplierAccountingBacklogUsesCalendarDayEndForMissingAndUnpublishedDates(t *testing.T) {
	db := newSupplierAccountingBacklogObserverTestDB(t)
	now := supplierAccountingBacklogTestTime(t, 8, 0, 0)
	day := time.Date(2026, time.July, 20, 0, 0, 0, 0, now.Location())
	run := SupplierUsageDailyBatchRun{BatchDate: "2026-07-20", DayStart: day.Unix(), DayEnd: now.Unix() + 1, Status: SupplierDailyBatchStatusFailed, PublishedFenceToken: 0}
	require.NoError(t, db.Create(&run).Error)

	observation, err := observeSupplierAccountingBacklogAt(context.Background(), db, day.Unix(), now.Unix())
	require.NoError(t, err)
	require.Equal(t, int64(4), observation.NeverPublishedDays, "the unpublished row plus three absent calendar dates are all backlog")
	require.Equal(t, day.AddDate(0, 0, 1).Unix(), observation.OldestNeverPublishedDayEnd)
	require.Equal(t, now.Unix()-day.AddDate(0, 0, 1).Unix(), observation.OldestNeverPublishedAgeSeconds)

	require.Equal(t, int64(86_400), supplierAccountingBacklogAgeSeconds(now.Unix(), now.Unix()-86_400))
	require.Equal(t, int64(86_401), supplierAccountingBacklogAgeSeconds(now.Unix(), now.Unix()-86_401))
	require.Zero(t, supplierAccountingBacklogAgeSeconds(now.Unix(), now.Unix()+1))
}

func TestObserveSupplierAccountingBacklogRejectsInteriorCalendarHole(t *testing.T) {
	db := newSupplierAccountingBacklogObserverTestDB(t)
	now := supplierAccountingBacklogTestTime(t, 8, 0, 0)
	start := time.Date(2026, time.July, 20, 0, 0, 0, 0, now.Location())
	runs := []SupplierUsageDailyBatchRun{
		{BatchDate: start.Format("2006-01-02"), DayStart: start.Unix(), DayEnd: start.AddDate(0, 0, 1).Unix(), Status: SupplierDailyBatchStatusCompleted, FenceToken: 1, PublishedFenceToken: 1},
		{BatchDate: start.AddDate(0, 0, 2).Format("2006-01-02"), DayStart: start.AddDate(0, 0, 2).Unix(), DayEnd: start.AddDate(0, 0, 3).Unix(), Status: SupplierDailyBatchStatusCompleted, FenceToken: 2, PublishedFenceToken: 2},
	}
	require.NoError(t, db.Create(&runs).Error)

	_, err := observeSupplierAccountingBacklogAt(context.Background(), db, start.Unix(), now.Unix())
	require.ErrorIs(t, err, ErrDatabase, "an interior hole is corruption, not a valid missing suffix")
}

func TestObserveSupplierAccountingBacklogFailsOnCancellationAndQueryError(t *testing.T) {
	db := newSupplierAccountingBacklogObserverTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ObserveSupplierAccountingBacklog(ctx, db, time.Now().AddDate(0, 0, -1).Unix())
	require.Error(t, err)

	require.NoError(t, db.Migrator().DropTable(&SupplierUsageDailyBatchRun{}))
	_, err = observeSupplierAccountingBacklogAt(context.Background(), db, time.Now().AddDate(0, 0, -1).Unix(), time.Now().Unix())
	require.Error(t, err)
}

func TestSupplierAccountingBacklogObserverSharesSchedulerPredicate(t *testing.T) {
	source, err := os.ReadFile("supplier_usage.go")
	require.NoError(t, err)
	text := string(source)
	sharedStart := strings.Index(text, "func neverPublishedSupplierDailyBatchRangeQuery")
	require.NotEqual(t, -1, sharedStart)
	sharedEnd := strings.Index(text[sharedStart:], "\nfunc ObserveSupplierAccountingBacklog")
	require.NotEqual(t, -1, sharedEnd)
	shared := text[sharedStart : sharedStart+sharedEnd]
	require.Contains(t, shared, `Where("batch_date >= ? AND batch_date <= ?", startDate, throughDate)`)
	require.Contains(t, shared, `Where("published_fence_token = ?", 0)`)
	for _, mutablePredicate := range []string{"status", "locked_until", "active_lease_slot", "lease_owner"} {
		require.NotContains(t, shared, mutablePredicate)
	}

	oldestStart := strings.Index(text, "func oldestNeverPublishedSupplierDailyBatchQuery")
	require.NotEqual(t, -1, oldestStart)
	oldest := text[oldestStart:sharedStart]
	require.Contains(t, oldest, "neverPublishedSupplierDailyBatchRangeQuery")
	observerStart := strings.Index(text, "func observeSupplierAccountingBacklogAt")
	require.NotEqual(t, -1, observerStart)
	observerEnd := strings.Index(text[observerStart:], "\nfunc supplierAccountingBacklogAgeSeconds")
	require.NotEqual(t, -1, observerEnd)
	observer := text[observerStart : observerStart+observerEnd]
	require.Contains(t, observer, "supplierAccountingBacklogAggregateQuery")
	require.NotContains(t, observer, "published_fence_token >")
	for _, mutablePredicate := range []string{`Where("status`, "locked_until", "active_lease_slot", "lease_owner"} {
		require.NotContains(t, observer, mutablePredicate)
	}
}

func TestSupplierAccountingBacklogAggregateSQLIsCrossDatabaseReady(t *testing.T) {
	sqliteDB := newSupplierAccountingBacklogObserverTestDB(t)
	connection, err := sqliteDB.DB()
	require.NoError(t, err)
	dialectors := map[string]gorm.Dialector{
		"sqlite":   sqlite.Open("file:" + t.Name() + "-dry-run?mode=memory&cache=shared"),
		"mysql":    mysql.New(mysql.Config{Conn: connection, SkipInitializeWithVersion: true}),
		"postgres": postgres.New(postgres.Config{Conn: connection, WithoutReturning: true}),
	}
	for name, dialector := range dialectors {
		t.Run(name, func(t *testing.T) {
			dryRun, openErr := gorm.Open(dialector, &gorm.Config{DryRun: true, DisableAutomaticPing: true})
			require.NoError(t, openErr)
			var aggregate struct {
				ExistingDays               int64
				NeverPublishedExistingDays int64
				OldestExistingDate         *string
				NewestExistingDate         *string
				OldestNeverPublishedDate   *string
				PriorDayRows               int64
				PriorDayNeverPublishedRows int64
			}
			statement := supplierAccountingBacklogAggregateQuery(dryRun, "2026-07-20", "2026-07-23").Scan(&aggregate).Statement
			normalized := strings.NewReplacer("`", "", `"`, "").Replace(strings.ToLower(statement.SQL.String()))
			require.Contains(t, normalized, "batch_date >=")
			require.Contains(t, normalized, "batch_date <=")
			require.Contains(t, normalized, "count(*) as existing_days")
			require.Contains(t, normalized, "never_published_existing_days")
			require.Contains(t, normalized, "oldest_existing_date")
			require.Contains(t, normalized, "newest_existing_date")
			require.Contains(t, normalized, "oldest_never_published_date")
			require.Contains(t, normalized, "prior_day_rows")
			require.Contains(t, normalized, "prior_day_never_published_rows")
			require.NotContains(t, normalized, "not exists")
			require.NotContains(t, normalized, "day_end")
			require.NotContains(t, normalized, "status")
			require.NotContains(t, normalized, "locked_until")
			require.NotContains(t, normalized, "active_lease_slot")
		})
	}
}

func TestSupplierAccountingBacklogObservationCrossDB(t *testing.T) {
	tests := []struct {
		name             string
		dsnEnv           string
		expectedDatabase string
		open             func(string) gorm.Dialector
	}{
		{name: "sqlite", open: func(string) gorm.Dialector { return sqlite.Open(filepath.Join(t.TempDir(), "backlog-crossdb.db")) }},
		{name: "mysql", dsnEnv: "TEST_MYSQL_DSN", expectedDatabase: "supplier_g009_mysql", open: func(dsn string) gorm.Dialector { return mysql.Open(dsn) }},
		{name: "postgres", dsnEnv: "TEST_POSTGRES_DSN", expectedDatabase: "supplier_g009_postgres", open: func(dsn string) gorm.Dialector { return postgres.Open(dsn) }},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(testCase.dsnEnv))
			if testCase.dsnEnv != "" && dsn == "" {
				t.Skipf("set %s to run the supplier backlog observer integration test", testCase.dsnEnv)
			}
			db, err := gorm.Open(testCase.open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			if testCase.expectedDatabase != "" {
				query := "SELECT DATABASE()"
				if testCase.name == "postgres" {
					query = "SELECT current_database()"
				}
				var databaseName string
				require.NoError(t, db.Raw(query).Scan(&databaseName).Error)
				require.Equal(t, testCase.expectedDatabase, databaseName, "integration test refuses a non-isolated database")
			}
			require.NoError(t, db.AutoMigrate(&SupplierUsageDailyBatchRun{}))
			dbNow, err := supplierDBUnix(context.Background(), db)
			require.NoError(t, err)
			location, err := time.LoadLocation(supplierDailyBatchTimezone)
			require.NoError(t, err)
			now := time.Unix(dbNow, 0).In(location)
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
			start := today.AddDate(0, 0, -6)
			through := today.AddDate(0, 0, -1)
			startDate := start.Format("2006-01-02")
			throughDate := through.Format("2006-01-02")
			cleanup := func() {
				require.NoError(t, db.Where("batch_date >= ? AND batch_date <= ?", startDate, throughDate).Delete(&SupplierUsageDailyBatchRun{}).Error)
			}
			cleanup()
			t.Cleanup(cleanup)
			runs := []SupplierUsageDailyBatchRun{
				{BatchDate: startDate, DayStart: start.Unix(), DayEnd: start.AddDate(0, 0, 1).Unix(), Status: SupplierDailyBatchStatusCompleted, FenceToken: 1, PublishedFenceToken: 1},
				{BatchDate: start.AddDate(0, 0, 1).Format("2006-01-02"), DayStart: start.AddDate(0, 0, 1).Unix(), DayEnd: start.AddDate(0, 0, 2).Unix(), Status: SupplierDailyBatchStatusRunning, PublishedFenceToken: 0, LockedUntil: 1<<62 - 1},
				{BatchDate: start.AddDate(0, 0, 2).Format("2006-01-02"), DayStart: start.AddDate(0, 0, 2).Unix(), DayEnd: start.AddDate(0, 0, 3).Unix(), Status: SupplierDailyBatchStatusCompleted, FenceToken: 3, PublishedFenceToken: 3},
				{BatchDate: start.AddDate(0, 0, 3).Format("2006-01-02"), DayStart: start.AddDate(0, 0, 3).Unix(), DayEnd: start.AddDate(0, 0, 4).Unix(), Status: SupplierDailyBatchStatusFailed, PublishedFenceToken: 0},
			}
			require.NoError(t, db.Create(&runs).Error)
			observation, err := ObserveSupplierAccountingBacklog(context.Background(), db, start.Unix())
			require.NoError(t, err)
			require.Equal(t, int64(4), observation.NeverPublishedDays, "two zero-fence rows and two consecutive missing suffix dates must all count")
			expectedOldestDayEnd := start.AddDate(0, 0, 2).Unix()
			require.Equal(t, expectedOldestDayEnd, observation.OldestNeverPublishedDayEnd)
			require.Equal(t, supplierAccountingBacklogAgeSeconds(observation.ObservedAtUnix, expectedOldestDayEnd), observation.OldestNeverPublishedAgeSeconds)
			require.False(t, observation.PriorDayPublished)

			require.NoError(t, db.Create([]SupplierUsageDailyBatchRun{
				{BatchDate: start.AddDate(0, 0, 4).Format("2006-01-02"), DayStart: start.AddDate(0, 0, 4).Unix(), DayEnd: through.Unix(), Status: SupplierDailyBatchStatusCompleted, FenceToken: 10, PublishedFenceToken: 10},
				{BatchDate: throughDate, DayStart: through.Unix(), DayEnd: today.Unix(), Status: SupplierDailyBatchStatusCompleted, FenceToken: 11, PublishedFenceToken: 11},
			}).Error)
			observation, err = observeSupplierAccountingBacklogAt(context.Background(), db, start.Unix(), observation.ObservedAtUnix)
			require.NoError(t, err)
			require.Equal(t, int64(2), observation.NeverPublishedDays)
			require.True(t, observation.PriorDayPublished)

			observation, err = observeSupplierAccountingBacklogAt(context.Background(), db, start.AddDate(0, 0, 2).Add(12*time.Hour).Unix(), observation.ObservedAtUnix)
			require.NoError(t, err)
			require.Equal(t, int64(1), observation.NeverPublishedDays, "the cutover calendar day is included while an earlier zero-fence day is excluded")
			require.Equal(t, start.AddDate(0, 0, 4).Unix(), observation.OldestNeverPublishedDayEnd)
			require.True(t, observation.PriorDayPublished)
		})
	}
}
