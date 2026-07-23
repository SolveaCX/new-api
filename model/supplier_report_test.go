package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSupplierReportFreshnessUsesCanonicalCutoverGapRange(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}, &SupplierAccountingCoverageGap{}, &SupplierUsageDailyBatchRun{}))
	now := time.Now().Unix()
	activation := activationState(3, SupplierAccountingActivationActive, now)
	cutoverAt := *activation.CutoverAt
	encoded, err := common.Marshal(activation)
	require.NoError(t, err)
	require.NoError(t, db.Create(&Option{Key: SupplierAccountingActivationOptionKey, Value: string(encoded)}).Error)

	createClosed := func(label string, startAt, endAt int64) SupplierAccountingCoverageGap {
		input := validSupplierCoverageGapInput("freshness-open-"+label, startAt)
		opened, openErr := OpenSupplierAccountingCoverageGap(db, input)
		require.NoError(t, openErr)
		closed, closeErr := CloseSupplierAccountingCoverageGap(db, CloseSupplierAccountingCoverageGapInput{
			ID: opened.Id, EndAt: endAt, CloseCommandID: "freshness-close-" + label,
			ClosedBy: 1, FinanceDisposition: SupplierCoverageGapFinanceNoImpact,
			ExpectedVersion: opened.RecordVersion,
		})
		require.NoError(t, closeErr)
		return *closed
	}

	createClosed("before", cutoverAt-20, cutoverAt-1)
	overlapping := createClosed("overlap", cutoverAt-10, cutoverAt+1)
	inside := createClosed("inside", cutoverAt+2, now-1)
	futureInput := validSupplierCoverageGapInput("freshness-open-future", now+3600)
	_, err = OpenSupplierAccountingCoverageGap(db, futureInput)
	require.NoError(t, err)

	freshness, err := NewSupplierReportStore(db).QueryFreshness(context.Background())
	require.NoError(t, err)
	require.Equal(t, cutoverAt, freshness.CoverageStartAt)
	require.Len(t, freshness.KnownCoverageGaps, 2)
	require.Equal(t, []int64{overlapping.Id, inside.Id}, []int64{freshness.KnownCoverageGaps[0].Id, freshness.KnownCoverageGaps[1].Id})
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
	require.NoError(t, db.Create(&SupplierUsageDailyBatchRun{
		BatchDate: "2026-03-23", DayStart: dayStart, DayEnd: dayEnd,
		Status: SupplierDailyBatchStatusRunning, FenceToken: runningFence,
		PublishedFenceToken: publishedFence,
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
	require.Equal(t, int64(240), requestCount, "all summary-backed reads must ignore the force-rerun generation")

	freshness, err := store.QueryFreshness(context.Background())
	require.NoError(t, err)
	require.Equal(t, SupplierDailyBatchStatusRunning, freshness.LatestStatus)
	require.NotNil(t, freshness.FreshThrough)
	require.Equal(t, dayEnd, *freshness.FreshThrough, "the previously published generation remains fresh while its force rerun is running")
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
