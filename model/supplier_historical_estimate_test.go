package model

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func supplierHistoricalEstimateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SupplierHistoricalImport{}, &SupplierHistoricalDailySummary{}))
	return db
}

func TestSupplierHistoricalImportCommandIsIdempotentAndImmutable(t *testing.T) {
	db := supplierHistoricalEstimateTestDB(t)
	ctx := context.Background()
	input := SupplierHistoricalImportCreate{
		CommandHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CommandJSON:    `{"start_date":"2026-01-01"}`,
		IdempotencyKey: "history-1", CreatedBy: 7, Method: SupplierHistoricalMethodLogEstimateV1,
		Reason: "initial estimate", StartDate: "2026-01-01", EndDate: "2026-02-01",
		DayStart: 1767196800, DayEnd: 1769875200, QuotaPerUnit: "500000",
		ExcludedUserIdsJSON: `[1,2]`, ChannelMappingsJSON: `[]`,
	}
	created, err := CreateSupplierHistoricalImport(ctx, db, input)
	require.NoError(t, err)
	replayed, err := CreateSupplierHistoricalImport(ctx, db, input)
	require.NoError(t, err)
	require.Equal(t, created.Id, replayed.Id)

	conflict := input
	conflict.CommandHash = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	_, err = CreateSupplierHistoricalImport(ctx, db, conflict)
	require.ErrorIs(t, err, ErrSupplierHistoricalImportIdempotencyConflict)
	require.ErrorIs(t,
		db.Model(&SupplierHistoricalImport{}).Where("id = ?", created.Id).Update("quota_per_unit", "1").Error,
		ErrSupplierHistoricalImportImmutable,
	)
}

func TestCreateSupplierHistoricalImportSerializesConcurrentOverlappingRanges(t *testing.T) {
	db := supplierHistoricalEstimateTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	sqlDB.SetMaxOpenConns(2)

	inputs := []SupplierHistoricalImportCreate{
		{
			CommandHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CommandJSON: `{}`,
			IdempotencyKey: "concurrent-a", CreatedBy: 7, Method: SupplierHistoricalMethodLogEstimateV1, Reason: "concurrent a",
			StartDate: "2026-01-01", EndDate: "2026-02-01", DayStart: 1767196800, DayEnd: 1769875200,
			QuotaPerUnit: "500000", ExcludedUserIdsJSON: `[]`, ChannelMappingsJSON: `[]`,
		},
		{
			CommandHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", CommandJSON: `{}`,
			IdempotencyKey: "concurrent-b", CreatedBy: 8, Method: SupplierHistoricalMethodLogEstimateV1, Reason: "concurrent b",
			StartDate: "2026-01-15", EndDate: "2026-02-15", DayStart: 1768406400, DayEnd: 1771084800,
			QuotaPerUnit: "500000", ExcludedUserIdsJSON: `[]`, ChannelMappingsJSON: `[]`,
		},
	}

	start := make(chan struct{})
	errs := make(chan error, len(inputs))
	var workers sync.WaitGroup
	for _, input := range inputs {
		input := input
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, createErr := CreateSupplierHistoricalImport(context.Background(), db, input)
			errs <- createErr
		}()
	}
	close(start)
	workers.Wait()
	close(errs)

	var accepted, rejected int
	for createErr := range errs {
		switch {
		case createErr == nil:
			accepted++
		case errors.Is(createErr, ErrSupplierHistoricalImportOverlap):
			rejected++
		default:
			require.NoError(t, createErr)
		}
	}
	require.Equal(t, 1, accepted)
	require.Equal(t, 1, rejected)

	var imports []SupplierHistoricalImport
	require.NoError(t, db.Order("id ASC").Find(&imports).Error)
	require.Len(t, imports, 2)
	statuses := map[string]int{}
	var failed SupplierHistoricalImport
	for _, item := range imports {
		statuses[item.Status]++
		if item.Status == SupplierHistoricalImportStatusFailed {
			failed = item
			require.Equal(t, ErrSupplierHistoricalImportOverlap.Error(), item.ErrorMessage)
			require.Empty(t, item.LeaseOwner)
			require.Nil(t, item.ActiveLeaseSlot)
		}
	}
	require.Equal(t, 1, statuses[SupplierHistoricalImportStatusPending])
	require.Equal(t, 1, statuses[SupplierHistoricalImportStatusFailed])
	failedInput := inputs[0]
	if failed.IdempotencyKey == inputs[1].IdempotencyKey {
		failedInput = inputs[1]
	}
	replayed, err := CreateSupplierHistoricalImport(context.Background(), db, failedInput)
	require.ErrorIs(t, err, ErrSupplierHistoricalImportOverlap)
	require.Equal(t, failed.Id, replayed.Id)
	var importCount int64
	require.NoError(t, db.Model(&SupplierHistoricalImport{}).Count(&importCount).Error)
	require.Equal(t, int64(2), importCount)
}

func TestSupplierHistoricalImportLeaseFencesStaleWorkersAndRejectsOverlap(t *testing.T) {
	db := supplierHistoricalEstimateTestDB(t)
	ctx := context.Background()
	first, err := CreateSupplierHistoricalImport(ctx, db, SupplierHistoricalImportCreate{
		CommandHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CommandJSON: `{}`,
		IdempotencyKey: "first", CreatedBy: 7, Method: SupplierHistoricalMethodLogEstimateV1, Reason: "first",
		StartDate: "2026-01-01", EndDate: "2026-02-01", DayStart: 1767196800, DayEnd: 1769875200,
		QuotaPerUnit: "500000", ExcludedUserIdsJSON: `[]`, ChannelMappingsJSON: `[]`,
	})
	require.NoError(t, err)
	lease, err := AcquireSupplierHistoricalImport(ctx, db, first.Id, "node-a", time.Minute)
	require.NoError(t, err)
	require.NoError(t, FreezeSupplierHistoricalImport(ctx, db, lease, 90, 12))

	second := SupplierHistoricalImport{
		CommandHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", CommandJSON: `{}`,
		IdempotencyKey: "second", CreatedBy: 7, Method: SupplierHistoricalMethodLogEstimateV1, Reason: "second",
		StartDate: "2026-01-15", EndDate: "2026-02-15", DayStart: 1768406400, DayEnd: 1771084800,
		QuotaPerUnit: "500000", ExcludedUserIdsJSON: `[]`, ChannelMappingsJSON: `[]`, Status: SupplierHistoricalImportStatusPending,
	}
	require.NoError(t, db.Create(&second).Error, "simulate a concurrent insert that passed create preflight")
	_, err = AcquireSupplierHistoricalImport(ctx, db, second.Id, "node-b", time.Minute)
	require.ErrorIs(t, err, ErrSupplierHistoricalImportOverlap)
	var quarantined SupplierHistoricalImport
	require.NoError(t, db.First(&quarantined, second.Id).Error)
	require.Equal(t, SupplierHistoricalImportStatusFailed, quarantined.Status)
	require.Equal(t, ErrSupplierHistoricalImportOverlap.Error(), quarantined.ErrorMessage)
	require.Nil(t, quarantined.ActiveLeaseSlot)

	require.NoError(t, db.Model(&SupplierHistoricalImport{}).Where("id = ?", first.Id).UpdateColumn("locked_until", 0).Error)
	reclaimed, err := AcquireSupplierHistoricalImport(ctx, db, first.Id, "node-b", time.Minute)
	require.NoError(t, err)
	require.Greater(t, reclaimed.FenceToken, lease.FenceToken)
	require.ErrorIs(t, CommitSupplierHistoricalImportPage(ctx, db, lease, nil, 0, 0, 0), ErrSupplierHistoricalImportFenceLost)
}

func TestSupplierHistoricalDailySummaryUniqueDimensionAndIndexShape(t *testing.T) {
	db := supplierHistoricalEstimateTestDB(t)
	require.True(t, db.Migrator().HasIndex(&SupplierHistoricalDailySummary{}, "ux_supplier_historical_daily_dimension"))
	require.NoError(t, db.Create(&SupplierHistoricalDailySummary{ImportId: 1, Date: "2026-01-01", DimensionKey: "x", DataQuality: SupplierHistoricalDataQualityEstimated}).Error)
	require.Error(t, db.Create(&SupplierHistoricalDailySummary{ImportId: 1, Date: "2026-01-01", DimensionKey: "x", DataQuality: SupplierHistoricalDataQualityEstimated}).Error)
}

func TestSupplierHistoricalFailedImportDoesNotBlockReplacementRange(t *testing.T) {
	db := supplierHistoricalEstimateTestDB(t)
	ctx := context.Background()
	first, err := CreateSupplierHistoricalImport(ctx, db, SupplierHistoricalImportCreate{
		CommandHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CommandJSON: `{}`,
		IdempotencyKey: "failed", CreatedBy: 7, Method: SupplierHistoricalMethodLogEstimateV1, Reason: "failed",
		StartDate: "2026-01-01", EndDate: "2026-02-01", DayStart: 1767196800, DayEnd: 1769875200,
		QuotaPerUnit: "500000", ExcludedUserIdsJSON: `[]`, ChannelMappingsJSON: `[]`,
	})
	require.NoError(t, err)
	lease, err := AcquireSupplierHistoricalImport(ctx, db, first.Id, "node-a", time.Minute)
	require.NoError(t, err)
	require.NoError(t, FreezeSupplierHistoricalImport(ctx, db, lease, 1, 1))
	require.NoError(t, FailSupplierHistoricalImport(ctx, db, lease, ErrSupplierHistoricalImportSourceChanged))

	second, err := CreateSupplierHistoricalImport(ctx, db, SupplierHistoricalImportCreate{
		CommandHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", CommandJSON: `{}`,
		IdempotencyKey: "replacement", CreatedBy: 7, Method: SupplierHistoricalMethodLogEstimateV1, Reason: "replacement",
		StartDate: "2026-01-01", EndDate: "2026-02-01", DayStart: 1767196800, DayEnd: 1769875200,
		QuotaPerUnit: "500000", ExcludedUserIdsJSON: `[]`, ChannelMappingsJSON: `[]`,
	})
	require.NoError(t, err)
	_, err = AcquireSupplierHistoricalImport(ctx, db, second.Id, "node-b", time.Minute)
	require.NoError(t, err)
}

func TestSupplierHistoricalPageMergeRejectsCrossPageMoneyOverflow(t *testing.T) {
	db := supplierHistoricalEstimateTestDB(t)
	ctx := context.Background()
	item, err := CreateSupplierHistoricalImport(ctx, db, SupplierHistoricalImportCreate{
		CommandHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CommandJSON: `{}`,
		IdempotencyKey: "overflow", CreatedBy: 7, Method: SupplierHistoricalMethodLogEstimateV1, Reason: "overflow",
		StartDate: "2026-01-01", EndDate: "2026-02-01", DayStart: 1767196800, DayEnd: 1769875200,
		QuotaPerUnit: "500000", ExcludedUserIdsJSON: `[]`, ChannelMappingsJSON: `[]`,
	})
	require.NoError(t, err)
	lease, err := AcquireSupplierHistoricalImport(ctx, db, item.Id, "node-a", time.Minute)
	require.NoError(t, err)
	require.NoError(t, FreezeSupplierHistoricalImport(ctx, db, lease, 2, 2))
	require.NoError(t, db.Create(&SupplierHistoricalDailySummary{
		ImportId: item.Id, Date: "2026-01-01", DimensionKey: "same", DataQuality: SupplierHistoricalDataQualityEstimated,
		SourceRequestCount: 1, SalesKnownCount: 1, SalesMicroUsd: math.MaxInt64,
	}).Error)
	err = CommitSupplierHistoricalImportPage(ctx, db, lease, []SupplierHistoricalDailySummary{{
		ImportId: item.Id, Date: "2026-01-01", DimensionKey: "same", DataQuality: SupplierHistoricalDataQualityEstimated,
		SourceRequestCount: 1, SalesKnownCount: 1, SalesMicroUsd: 1,
	}}, 1767196801, 2, 1)
	require.ErrorIs(t, err, ErrSupplierHistoricalMoneyOverflow)
	refreshed, err := GetSupplierHistoricalImport(ctx, db, item.Id)
	require.NoError(t, err)
	require.Zero(t, refreshed.ProcessedCount, "overflow must roll back cursor and count with the summary write")
	var persisted SupplierHistoricalDailySummary
	require.NoError(t, db.Where("import_id = ? AND dimension_key = ?", item.Id, "same").First(&persisted).Error)
	require.Equal(t, int64(math.MaxInt64), persisted.SalesMicroUsd, "overflow must roll back the summary total")
}

func TestSupplierHistoricalSchemaIndexesAcrossSupportedDialects(t *testing.T) {
	sqliteDB, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	connection, err := sqliteDB.DB()
	require.NoError(t, err)
	dialectors := map[string]gorm.Dialector{
		"sqlite":     sqlite.Open("file:" + t.Name() + "-schema?mode=memory&cache=shared"),
		"mysql57":    mysql.New(mysql.Config{Conn: connection, SkipInitializeWithVersion: true}),
		"postgres96": postgres.New(postgres.Config{Conn: connection, WithoutReturning: true}),
	}
	for name, dialector := range dialectors {
		t.Run(name, func(t *testing.T) {
			db, openErr := gorm.Open(dialector, &gorm.Config{DryRun: true, DisableAutomaticPing: true})
			require.NoError(t, openErr)
			statement := &gorm.Statement{DB: db}
			require.NoError(t, statement.Parse(&SupplierHistoricalDailySummary{}))
			var uniqueColumns []string
			for _, index := range statement.Schema.ParseIndexes() {
				if index.Name == "ux_supplier_historical_daily_dimension" {
					for _, field := range index.Fields {
						uniqueColumns = append(uniqueColumns, field.DBName)
					}
				}
			}
			require.Equal(t, []string{"import_id", "date", "dimension_key"}, uniqueColumns)
		})
	}
}

func TestSupplierHistoricalSeriesAggregatesDimensionsAndEnforcesKeysetLimit(t *testing.T) {
	db := supplierHistoricalEstimateTestDB(t)
	rows := make([]SupplierHistoricalDailySummary, 0, 550)
	for index := 0; index < 550; index++ {
		rows = append(rows, SupplierHistoricalDailySummary{
			ImportId: 1, Date: "2026-01-01", DimensionKey: fmt.Sprintf("dimension-%03d", index),
			StatisticsScope: "business", SupplierId: index + 1, DataQuality: SupplierHistoricalDataQualityEstimated,
			SourceRequestCount: 1, SalesKnownCount: 1, SalesMicroUsd: 1,
		})
	}
	require.NoError(t, db.CreateInBatches(&rows, 100).Error)
	first, hasMore, err := ListSupplierHistoricalSeries(context.Background(), db, 1, "2026-01-01", "2026-01-02", SupplierHistoricalSeriesCursor{}, 500)
	require.NoError(t, err)
	require.True(t, hasMore)
	require.Len(t, first, 500)
	last := first[len(first)-1]
	second, hasMore, err := ListSupplierHistoricalSeries(context.Background(), db, 1, "2026-01-01", "2026-01-02", SupplierHistoricalSeriesCursor{
		Date: last.Date, StatisticsScope: last.StatisticsScope, SupplierId: last.SupplierId,
	}, 500)
	require.NoError(t, err)
	require.False(t, hasMore)
	require.Len(t, second, 50)
	_, _, err = ListSupplierHistoricalSeries(context.Background(), db, 1, "2026-01-01", "2026-01-02", SupplierHistoricalSeriesCursor{}, 501)
	require.ErrorIs(t, err, ErrSupplierHistoricalImportInvalid)
}

func TestSupplierHistoricalQuarantinesConflictingPendingAndClaimsNextImport(t *testing.T) {
	db := supplierHistoricalEstimateTestDB(t)
	completed := SupplierHistoricalImport{
		CommandHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CommandJSON: `{}`,
		IdempotencyKey: "completed", CreatedBy: 7, Method: SupplierHistoricalMethodLogEstimateV1, Reason: "completed",
		StartDate: "2026-01-01", EndDate: "2026-02-01", DayStart: 1767196800, DayEnd: 1769875200,
		QuotaPerUnit: "500000", ExcludedUserIdsJSON: `[]`, ChannelMappingsJSON: `[]`, Status: SupplierHistoricalImportStatusCompleted,
	}
	require.NoError(t, db.Create(&completed).Error)
	conflicting := SupplierHistoricalImport{
		CommandHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", CommandJSON: `{}`,
		IdempotencyKey: "conflict", CreatedBy: 7, Method: SupplierHistoricalMethodLogEstimateV1, Reason: "conflict",
		StartDate: "2026-01-15", EndDate: "2026-01-20", DayStart: 1768406400, DayEnd: 1768838400,
		QuotaPerUnit: "500000", ExcludedUserIdsJSON: `[]`, ChannelMappingsJSON: `[]`, Status: SupplierHistoricalImportStatusPending,
	}
	require.NoError(t, db.Create(&conflicting).Error)
	legal := SupplierHistoricalImport{
		CommandHash: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", CommandJSON: `{}`,
		IdempotencyKey: "legal", CreatedBy: 7, Method: SupplierHistoricalMethodLogEstimateV1, Reason: "legal",
		StartDate: "2026-02-01", EndDate: "2026-03-01", DayStart: 1769875200, DayEnd: 1772294400,
		QuotaPerUnit: "500000", ExcludedUserIdsJSON: `[]`, ChannelMappingsJSON: `[]`, Status: SupplierHistoricalImportStatusPending,
	}
	require.NoError(t, db.Create(&legal).Error)

	lease, err := AcquireSupplierHistoricalImport(context.Background(), db, 0, "node-a", time.Minute)
	require.NoError(t, err)
	require.Equal(t, legal.Id, lease.ImportId)
	var quarantined SupplierHistoricalImport
	require.NoError(t, db.First(&quarantined, conflicting.Id).Error)
	require.Equal(t, SupplierHistoricalImportStatusFailed, quarantined.Status)
	require.Contains(t, quarantined.ErrorMessage, "overlap")
}

func TestSupplierHistoricalQuarantinesOnlyConflictingPendingAndCommits(t *testing.T) {
	db := supplierHistoricalEstimateTestDB(t)
	completed := SupplierHistoricalImport{
		CommandHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CommandJSON: `{}`,
		IdempotencyKey: "completed-only", CreatedBy: 7, Method: SupplierHistoricalMethodLogEstimateV1, Reason: "completed",
		StartDate: "2026-01-01", EndDate: "2026-02-01", DayStart: 1767196800, DayEnd: 1769875200,
		QuotaPerUnit: "500000", ExcludedUserIdsJSON: `[]`, ChannelMappingsJSON: `[]`, Status: SupplierHistoricalImportStatusCompleted,
	}
	require.NoError(t, db.Create(&completed).Error)
	conflicting := SupplierHistoricalImport{
		CommandHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", CommandJSON: `{}`,
		IdempotencyKey: "conflict-only", CreatedBy: 7, Method: SupplierHistoricalMethodLogEstimateV1, Reason: "conflict",
		StartDate: "2026-01-15", EndDate: "2026-01-20", DayStart: 1768406400, DayEnd: 1768838400,
		QuotaPerUnit: "500000", ExcludedUserIdsJSON: `[]`, ChannelMappingsJSON: `[]`, Status: SupplierHistoricalImportStatusPending,
	}
	require.NoError(t, db.Create(&conflicting).Error)

	_, err := AcquireSupplierHistoricalImport(context.Background(), db, 0, "node-a", time.Minute)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	var quarantined SupplierHistoricalImport
	require.NoError(t, db.First(&quarantined, conflicting.Id).Error)
	require.Equal(t, SupplierHistoricalImportStatusFailed, quarantined.Status)
	require.Contains(t, quarantined.ErrorMessage, "overlap")
}
