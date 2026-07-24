package model

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSupplierReportReadTxOptions(t *testing.T) {
	require.Nil(t, supplierReportReadTxOptions("sqlite"))
	for _, dialect := range []string{"mysql", "postgres"} {
		options := supplierReportReadTxOptions(dialect)
		require.NotNil(t, options)
		require.Equal(t, sql.LevelRepeatableRead, options.Isolation)
		require.True(t, options.ReadOnly)
	}
}

func TestListChannelCatalogDatabasePaginationAndPublishedGeneration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}, &Channel{}, &SupplierUsageDailySummary{}, &SupplierUsageDailyBatchRun{}))

	const (
		publishedFence = int64(7)
		runningFence   = int64(8)
		dayStart       = int64(1_774_281_600)
		dayEnd         = dayStart + 86_400
	)
	publishedAt := dayEnd
	require.NoError(t, db.Create(&SupplierUsageDailyBatchRun{
		BatchDate: "2026-03-23", DayStart: dayStart, DayEnd: dayEnd,
		Status: SupplierDailyBatchStatusRunning, FenceToken: runningFence,
		PublishedFenceToken: publishedFence, PublishedAt: &publishedAt,
	}).Error)

	channels := make([]Channel, 0, 300)
	for channelID := 1; channelID <= 300; channelID++ {
		channel := Channel{Id: channelID, Key: fmt.Sprintf("key-%d", channelID), Name: fmt.Sprintf("channel-%03d", channelID), Status: 1}
		if channelID <= 180 {
			contractID := 10
			channel.SupplierContractId = &contractID
		}
		channels = append(channels, channel)
	}
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).CreateInBatches(channels, 100).Error)

	summaries := make([]SupplierUsageDailySummary, 0, 300)
	appendSummary := func(channelID, contractID int, fence int64, key string, requestCount int64) {
		summaries = append(summaries, SupplierUsageDailySummary{
			BatchDate: "2026-03-23", BatchFenceToken: fence, DimensionKey: key,
			BucketStart: dayStart, SupplierId: 1, ContractId: contractID,
			ChannelId: channelID, ModelName: "model", StatisticsScope: "business",
			DataQuality: "authoritative", RequestCount: requestCount,
		})
	}
	for channelID := 81; channelID <= 180; channelID++ {
		appendSummary(channelID, 10, publishedFence, fmt.Sprintf("overlap-%d", channelID), 1)
	}
	for channelID := 121; channelID <= 180; channelID++ {
		appendSummary(channelID, 20, publishedFence, fmt.Sprintf("rebound-%d", channelID), 1)
	}
	for channelID := 181; channelID <= 260; channelID++ {
		appendSummary(channelID, 10, publishedFence, fmt.Sprintf("historical-%d", channelID), 1)
	}
	for channelID := 261; channelID <= 300; channelID++ {
		appendSummary(channelID, 10, runningFence, fmt.Sprintf("unpublished-%d", channelID), 100)
	}
	require.NoError(t, db.CreateInBatches(summaries, 100).Error)

	store := NewSupplierReportStore(db)
	filter := SupplierReportFilter{StartAt: dayStart, EndAt: dayEnd, ContractIds: []int{10, 20}}
	const pageSize = 73
	var allRows []SupplierReportChannelCatalogRow
	for offset := 0; ; offset += pageSize {
		page := SupplierReportPage{Limit: pageSize, Offset: offset}
		rows, hasMore, queryErr := store.ListChannelCatalog(context.Background(), filter, &page)
		require.NoError(t, queryErr)
		allRows = append(allRows, rows...)
		if !hasMore {
			break
		}
	}

	require.Len(t, allRows, 320, "current-only, historical-only, and rebound pairs must form one deduplicated catalog")
	seen := make(map[[2]int]struct{}, len(allRows))
	for i, row := range allRows {
		key := [2]int{row.ChannelId, row.SupplierContractId}
		_, duplicate := seen[key]
		require.False(t, duplicate, "duplicate catalog pair at row %d: %v", i, key)
		seen[key] = struct{}{}
		if i > 0 {
			previous := allRows[i-1]
			require.True(t,
				previous.ChannelId < row.ChannelId ||
					(previous.ChannelId == row.ChannelId && previous.SupplierContractId < row.SupplierContractId),
				"catalog order must be stable by channel then contract",
			)
		}
	}
	require.Contains(t, seen, [2]int{1, 10}, "current-only binding must remain visible")
	require.Contains(t, seen, [2]int{200, 10}, "historical-only binding must remain visible")
	require.Contains(t, seen, [2]int{150, 10}, "current/history overlap must be deduplicated")
	require.Contains(t, seen, [2]int{150, 20}, "historical ownership must survive a rebind")
	require.NotContains(t, seen, [2]int{261, 10}, "the in-progress generation must remain invisible")

	page := SupplierReportPage{Limit: pageSize, Offset: pageSize}
	firstRead, _, err := store.ListChannelCatalog(context.Background(), filter, &page)
	require.NoError(t, err)
	secondRead, _, err := store.ListChannelCatalog(context.Background(), filter, &page)
	require.NoError(t, err)
	require.Equal(t, firstRead, secondRead, "re-reading a page must not reorder or duplicate rows")

	usageRows, err := store.QueryBusinessUsage(context.Background(), filter, false)
	require.NoError(t, err)
	var requestCount int64
	for _, row := range usageRows {
		requestCount += row.BusinessRequestCount
	}
	require.Equal(t, int64(240), requestCount, "all summary-backed reads must ignore the unpublished generation")

}

func TestQueryBreakdownPaginationOrdersEveryGroupedDimension(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SupplierUsageDailySummary{}, &SupplierUsageDailyBatchRun{}))
	const (
		dayStart = int64(1_774_281_600)
		dayEnd   = dayStart + 86_400
		fence    = int64(3)
	)
	completedAt := dayEnd
	require.NoError(t, db.Create(&SupplierUsageDailyBatchRun{
		BatchDate: "2026-03-23", DayStart: dayStart, DayEnd: dayEnd,
		Status: SupplierDailyBatchStatusCompleted, FenceToken: fence,
		PublishedFenceToken: fence, CompletedAt: &completedAt,
	}).Error)

	zero := int64(0)
	fiveHundred := int64(500)
	rows := []SupplierUsageDailySummary{
		{BatchDate: "2026-03-23", BatchFenceToken: fence, DimensionKey: "1", BucketStart: dayStart, SupplierId: 1, ContractId: 10, ChannelId: 20, ModelName: "same", RateVersionId: 1, SalesMultiplierPpm: nil, PricingMode: "a", StatisticsScope: "business", DataQuality: "a", RequestCount: 1},
		{BatchDate: "2026-03-23", BatchFenceToken: fence, DimensionKey: "2", BucketStart: dayStart, SupplierId: 1, ContractId: 10, ChannelId: 20, ModelName: "same", RateVersionId: 1, SalesMultiplierPpm: nil, PricingMode: "b", StatisticsScope: "business", DataQuality: "a", RequestCount: 1},
		{BatchDate: "2026-03-23", BatchFenceToken: fence, DimensionKey: "3", BucketStart: dayStart, SupplierId: 1, ContractId: 10, ChannelId: 20, ModelName: "same", RateVersionId: 1, SalesMultiplierPpm: &zero, PricingMode: "a", StatisticsScope: "business", DataQuality: "a", RequestCount: 1},
		{BatchDate: "2026-03-23", BatchFenceToken: fence, DimensionKey: "4", BucketStart: dayStart, SupplierId: 1, ContractId: 10, ChannelId: 20, ModelName: "same", RateVersionId: 1, SalesMultiplierPpm: &fiveHundred, PricingMode: "a", StatisticsScope: "business", DataQuality: "a", RequestCount: 1},
		{BatchDate: "2026-03-23", BatchFenceToken: fence, DimensionKey: "5", BucketStart: dayStart, SupplierId: 1, ContractId: 10, ChannelId: 20, ModelName: "same", RateVersionId: 1, SalesMultiplierPpm: &fiveHundred, PricingMode: "a", StatisticsScope: "business", DataQuality: "b", RequestCount: 1},
		{BatchDate: "2026-03-23", BatchFenceToken: fence, DimensionKey: "6", BucketStart: dayStart, SupplierId: 1, ContractId: 10, ChannelId: 20, ModelName: "same", RateVersionId: 2, SalesMultiplierPpm: nil, PricingMode: "a", StatisticsScope: "business", DataQuality: "a", RequestCount: 1},
	}
	require.NoError(t, db.Create(&rows).Error)

	store := NewSupplierReportStore(db)
	filter := SupplierReportFilter{StartAt: dayStart, EndAt: dayEnd, ContractIds: []int{10}}
	var got []string
	for offset := 0; ; offset += 2 {
		pageRows, hasMore, queryErr := store.QueryBreakdown(context.Background(), filter, SupplierReportPage{Limit: 2, Offset: offset})
		require.NoError(t, queryErr)
		for _, row := range pageRows {
			multiplier := "null"
			if row.SalesMultiplierPpm != nil {
				multiplier = fmt.Sprint(*row.SalesMultiplierPpm)
			}
			got = append(got, fmt.Sprintf("%d/%s/%s/%s", row.RateVersionId, multiplier, row.PricingMode, row.DataQuality))
		}
		if !hasMore {
			break
		}
	}
	require.Equal(t, []string{
		"1/null/a/a", "1/null/b/a", "1/0/a/a",
		"1/500/a/a", "1/500/a/b", "2/null/a/a",
	}, got)

	first, _, err := store.QueryBreakdown(context.Background(), filter, SupplierReportPage{Limit: 2, Offset: 2})
	require.NoError(t, err)
	second, _, err := store.QueryBreakdown(context.Background(), filter, SupplierReportPage{Limit: 2, Offset: 2})
	require.NoError(t, err)
	require.Equal(t, first, second)
}

func TestQueryChannelUsageBoundsAggregationToCatalogPairs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SupplierUsageDailySummary{}, &SupplierUsageDailyBatchRun{}))
	const (
		dayStart = int64(1_774_281_600)
		dayEnd   = dayStart + 86_400
		fence    = int64(9)
	)
	publishedAt := dayEnd
	require.NoError(t, db.Create(&SupplierUsageDailyBatchRun{
		BatchDate: "2026-03-23", DayStart: dayStart, DayEnd: dayEnd,
		Status: SupplierDailyBatchStatusCompleted, FenceToken: fence,
		PublishedFenceToken: fence, PublishedAt: &publishedAt,
	}).Error)
	require.NoError(t, db.Create(&[]SupplierUsageDailySummary{
		{BatchDate: "2026-03-23", BatchFenceToken: fence, DimensionKey: "page-1", BucketStart: dayStart, SupplierId: 1, ContractId: 10, ChannelId: 20, StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 2},
		{BatchDate: "2026-03-23", BatchFenceToken: fence, DimensionKey: "page-2", BucketStart: dayStart, SupplierId: 1, ContractId: 30, ChannelId: 40, StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 3},
		{BatchDate: "2026-03-23", BatchFenceToken: fence, DimensionKey: "outside-page", BucketStart: dayStart, SupplierId: 1, ContractId: 10, ChannelId: 40, StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 100},
	}).Error)

	store := NewSupplierReportStore(db)
	filter := SupplierReportFilter{StartAt: dayStart, EndAt: dayEnd}
	rows, err := store.QueryChannelUsage(context.Background(), filter, []SupplierReportChannelPair{
		{ContractId: 10, ChannelId: 20},
		{ContractId: 30, ChannelId: 40},
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	requestCounts := map[[2]int]int64{}
	for _, row := range rows {
		requestCounts[[2]int{row.ContractId, row.ChannelId}] += row.BusinessRequestCount
	}
	require.Equal(t, map[[2]int]int64{{10, 20}: 2, {30, 40}: 3}, requestCounts)

	emptyRows, err := store.QueryChannelUsage(context.Background(), filter, nil)
	require.NoError(t, err)
	require.Empty(t, emptyRows)
}

func TestSupplierReportStoreBoundsCatalogHistoryAndInventoryAtClosedEnd(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&UpstreamSupplier{}, &SupplierContract{}, &SupplierContractRateVersion{}, &SupplierInventoryAdjustment{},
		&Channel{}, &SupplierUsageDailySummary{}, &SupplierUsageDailyBatchRun{},
	))
	const (
		closedStart = int64(1_784_044_800) // 2026-07-15 00:00:00 Asia/Shanghai
		closedEnd   = closedStart + 86_400
		futureEnd   = closedEnd + 86_400
	)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&UpstreamSupplier{Id: 1, Name: "supplier", Status: SupplierStatusActive}).Error)
	futureRateID := 2
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&SupplierContract{
		Id: 1, SupplierId: 1, Name: "contract", ContractNo: "C-1", Status: SupplierContractStatusActive, CurrentRateVersionId: &futureRateID,
	}).Error)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&[]SupplierContractRateVersion{
		{Id: 1, ContractId: 1, ProcurementMultiplierPpm: 600_000, EffectiveAt: closedStart + 1, CreatedBy: 1, CreatedAt: closedStart + 1},
		{Id: futureRateID, ContractId: 1, ProcurementMultiplierPpm: 900_000, EffectiveAt: closedEnd + 1, CreatedBy: 1, CreatedAt: closedEnd + 1},
	}).Error)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&[]SupplierInventoryAdjustment{
		{ContractId: 1, DeltaMicroUsd: 5_000, Type: SupplierInventoryAdjustmentTypeInitial, IdempotencyKey: "closed", CreatedBy: 1, CreatedAt: closedStart + 1},
		{ContractId: 1, DeltaMicroUsd: 8_000, Type: SupplierInventoryAdjustmentTypeReplenishment, IdempotencyKey: "future", CreatedBy: 1, CreatedAt: closedEnd + 1},
	}).Error)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&[]Channel{
		{Id: 10, Name: "closed-history"}, {Id: 20, Name: "future-history"},
	}).Error)
	publishedAt := futureEnd
	require.NoError(t, db.Create(&[]SupplierUsageDailyBatchRun{
		{BatchDate: "2026-07-15", DayStart: closedStart, DayEnd: closedEnd, Status: SupplierDailyBatchStatusCompleted, FenceToken: 1, PublishedFenceToken: 1, PublishedAt: &publishedAt},
		{BatchDate: "2026-07-16", DayStart: closedEnd, DayEnd: futureEnd, Status: SupplierDailyBatchStatusCompleted, FenceToken: 2, PublishedFenceToken: 2, PublishedAt: &publishedAt},
	}).Error)
	require.NoError(t, db.Create(&[]SupplierUsageDailySummary{
		{BatchDate: "2026-07-15", BatchFenceToken: 1, DimensionKey: "closed", BucketStart: closedStart, SupplierId: 1, ContractId: 1, ChannelId: 10, ModelName: "closed", StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 1, OfficialListMicroUsd: 1_000},
		{BatchDate: "2026-07-16", BatchFenceToken: 2, DimensionKey: "future", BucketStart: closedEnd, SupplierId: 1, ContractId: 1, ChannelId: 20, ModelName: "future", StatisticsScope: "business", DataQuality: "authoritative", RequestCount: 10, OfficialListMicroUsd: 10_000},
	}).Error)

	store := NewSupplierReportStore(db)
	filter := SupplierReportFilter{StartAt: closedStart, EndAt: closedEnd, ContractIds: []int{1}}
	catalog, _, err := store.ListContractCatalog(context.Background(), filter, nil)
	require.NoError(t, err)
	require.Len(t, catalog, 1)
	require.NotNil(t, catalog[0].CurrentRateVersionId)
	require.Equal(t, 1, *catalog[0].CurrentRateVersionId, "catalog projects the latest rate effective before the closed end")

	channels, _, err := store.ListChannelCatalog(context.Background(), filter, nil)
	require.NoError(t, err)
	require.Len(t, channels, 1)
	require.Equal(t, 10, channels[0].ChannelId)

	rates, ratesHasMore, err := store.ListRateVersions(context.Background(), 1, closedEnd, SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.False(t, ratesHasMore)
	require.Len(t, rates, 1)
	require.Equal(t, 1, rates[0].Id)

	adjustments, adjustmentsHasMore, err := store.ListInventoryAdjustments(context.Background(), []int{1}, closedEnd, SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.False(t, adjustmentsHasMore)
	require.Len(t, adjustments, 1)
	require.Equal(t, int64(5_000), adjustments[0].DeltaMicroUsd)

	consumption, err := store.QueryInventoryConsumption(context.Background(), []int{1}, closedEnd)
	require.NoError(t, err)
	require.Len(t, consumption, 1)
	require.Equal(t, int64(1_000), consumption[0].InventoryAffectingOfficialListMicroUsd)
}

func TestSupplierReportUsageAggregationDropsContractAndQualityDimensions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SupplierUsageDailySummary{}, &SupplierUsageDailyBatchRun{}))
	const dayStart = int64(1_784_044_800)
	for dayOffset := 0; dayOffset < 2; dayOffset++ {
		start := dayStart + int64(dayOffset)*86_400
		fence := int64(dayOffset + 1)
		publishedAt := start + 86_400
		require.NoError(t, db.Create(&SupplierUsageDailyBatchRun{
			BatchDate: fmt.Sprintf("2026-07-%02d", 15+dayOffset), DayStart: start, DayEnd: publishedAt,
			Status: SupplierDailyBatchStatusCompleted, FenceToken: fence, PublishedFenceToken: fence, PublishedAt: &publishedAt,
		}).Error)
		rows := make([]SupplierUsageDailySummary, 0, 60)
		for contractID := 1; contractID <= 10; contractID++ {
			for qualityID, quality := range []string{"authoritative", "estimated", "unpriced"} {
				for scopeID, scope := range []string{"business", "internal"} {
					rows = append(rows, SupplierUsageDailySummary{
						BatchDate: fmt.Sprintf("2026-07-%02d", 15+dayOffset), BatchFenceToken: fence,
						DimensionKey: fmt.Sprintf("%d-%d-%d-%d", dayOffset, contractID, qualityID, scopeID), BucketStart: start,
						SupplierId: 1, ContractId: contractID, ChannelId: 1, StatisticsScope: scope, DataQuality: quality, RequestCount: 1,
					})
				}
			}
		}
		require.NoError(t, db.Create(&rows).Error)
	}
	store := NewSupplierReportStore(db)
	filter := SupplierReportFilter{StartAt: dayStart, EndAt: dayStart + 2*86_400}

	overview, err := store.QueryBusinessUsage(context.Background(), filter, false)
	require.NoError(t, err)
	require.Len(t, overview, 1, "overview materializes one SQL aggregate row")
	require.Equal(t, int64(60), overview[0].BusinessRequestCount)

	trend, err := store.QueryBusinessUsage(context.Background(), filter, true)
	require.NoError(t, err)
	require.Len(t, trend, 2, "trend materializes at most one row per requested day")
	require.Equal(t, int64(30), trend[0].BusinessRequestCount)
	require.Equal(t, int64(30), trend[1].BusinessRequestCount)

	byContract, err := store.QueryBusinessUsageByContract(context.Background(), filter)
	require.NoError(t, err)
	require.Len(t, byContract, 10, "contract reports materialize only the requested contract dimension")
}

func TestQueryOverviewInventoryIncludesContractsBeyondCatalogHardLimit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SupplierContract{}, &SupplierInventoryAdjustment{}, &SupplierUsageDailySummary{}, &SupplierUsageDailyBatchRun{}))
	const contractCount = SupplierReportMaxContractRows + 1
	contracts := make([]SupplierContract, 0, contractCount)
	adjustments := make([]SupplierInventoryAdjustment, 0, contractCount)
	for id := 1; id <= contractCount; id++ {
		contracts = append(contracts, SupplierContract{Id: id, SupplierId: 1, Name: fmt.Sprintf("contract-%d", id), ContractNo: fmt.Sprintf("C-%d", id), Status: SupplierContractStatusActive})
		adjustments = append(adjustments, SupplierInventoryAdjustment{ContractId: id, DeltaMicroUsd: 1, Type: SupplierInventoryAdjustmentTypeInitial, IdempotencyKey: fmt.Sprintf("inventory-%d", id), CreatedBy: 1, CreatedAt: 10})
	}
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).CreateInBatches(&contracts, 500).Error)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).CreateInBatches(&adjustments, 500).Error)

	totals, err := NewSupplierReportStore(db).QueryOverviewInventory(context.Background(), SupplierReportFilter{StartAt: 1, EndAt: 20})
	require.NoError(t, err)
	require.Equal(t, int64(contractCount), totals.TotalInventoryMicroUsd, "overview must not inherit the 5,000-row catalog cap")
}

func TestListChannelCatalogCurrentBindingsHonorSupplierFilter(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SupplierContract{}, &Channel{}, &SupplierUsageDailySummary{}, &SupplierUsageDailyBatchRun{}))
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&[]SupplierContract{
		{Id: 1, SupplierId: 1, Name: "wanted", ContractNo: "C-1", Status: SupplierContractStatusActive},
		{Id: 2, SupplierId: 2, Name: "other", ContractNo: "C-2", Status: SupplierContractStatusActive},
	}).Error)
	contract1, contract2 := 1, 2
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&[]Channel{
		{Id: 11, Name: "wanted-no-usage", SupplierContractId: &contract1},
		{Id: 22, Name: "other-no-usage", SupplierContractId: &contract2},
	}).Error)

	rows, hasMore, err := NewSupplierReportStore(db).ListChannelCatalog(context.Background(), SupplierReportFilter{StartAt: 1, EndAt: 2, SupplierIds: []int{1}}, &SupplierReportPage{Limit: 10})
	require.NoError(t, err)
	require.False(t, hasMore)
	require.Equal(t, []SupplierReportChannelCatalogRow{{ChannelId: 11, ChannelName: "wanted-no-usage", ChannelStatus: 1, SupplierContractId: 1}}, rows)
}
