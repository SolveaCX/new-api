package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func supplierAccountingRetentionMainDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"-main?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SupplierUsageDailyBatchRun{}))
	return db
}

func TestSelectSupplierAccountingFactRetentionBatchRequiresPublishedCompletedBoundScopeAndOldDate(t *testing.T) {
	db := supplierAccountingRetentionMainDB(t)
	ctx := context.Background()
	now := time.Now().In(supplierAccountingFactLocation)
	day := func(daysAgo int) string { return now.AddDate(0, 0, -daysAgo).Format("2006-01-02") }
	scope := string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1)
	published := int64(9)
	runs := []SupplierUsageDailyBatchRun{
		{BatchDate: day(60), Status: SupplierDailyBatchStatusRunning, PublishedFenceToken: published, SourceMaxFactId: 1, CoverageScope: scope},
		{BatchDate: day(59), Status: SupplierDailyBatchStatusCompleted, PublishedFenceToken: 0, SourceMaxFactId: 2, CoverageScope: scope},
		{BatchDate: day(58), Status: SupplierDailyBatchStatusCompleted, PublishedFenceToken: published, SourceMaxFactId: 3, CoverageScope: "other_scope"},
		{BatchDate: day(56), Status: SupplierDailyBatchStatusFailed, PublishedFenceToken: published, SourceMaxFactId: 6, CoverageScope: scope},
		{BatchDate: day(57), Status: SupplierDailyBatchStatusCompleted, PublishedFenceToken: published, SourceMaxFactId: 4, CoverageScope: scope},
		{BatchDate: day(10), Status: SupplierDailyBatchStatusCompleted, PublishedFenceToken: published, SourceMaxFactId: 5, CoverageScope: scope},
	}
	require.NoError(t, db.Create(&runs).Error)

	batch, err := SelectSupplierAccountingFactRetentionBatch(ctx, db, 30, "")
	require.NoError(t, err)
	require.NotNil(t, batch)
	require.Equal(t, day(57), batch.PreparedDay)
	require.EqualValues(t, 4, batch.SourceMaxFactId)

	require.NoError(t, db.Delete(&SupplierUsageDailyBatchRun{}, runs[4].Id).Error)
	batch, err = SelectSupplierAccountingFactRetentionBatch(ctx, db, 30, "")
	require.NoError(t, err)
	require.Nil(t, batch)
}

func TestDeleteSupplierAccountingFactRetentionChunkUsesTerminalScopeDayAndWatermark(t *testing.T) {
	db := supplierAccountingFactTestDB(t)
	ctx := context.Background()
	preparedDay := "2026-01-01"
	scope := string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1)
	seed := func(attempt string, status string, day string, coverage string) SupplierAccountingFact {
		fact := SupplierAccountingFact{AttemptId: attempt, ParentRequestId: attempt, PreparedAt: 1, PreparedDay: day,
			CoverageScope: coverage, Status: status, SupplierId: 1, ContractId: 1, BindingVersionId: 1,
			RateVersionId: 1, ChannelId: 1, ModelName: "gpt-test"}
		require.NoError(t, db.Create(&fact).Error)
		return fact
	}

	pending := seed("pending", SupplierAccountingFactStatusPending, preparedDay, scope)
	nonTerminal := seed("non-terminal", "failed", preparedDay, scope)
	wrongScope := seed("wrong-scope", SupplierAccountingFactStatusCaptured, preparedDay, "other_scope")
	wrongDay := seed("wrong-day", SupplierAccountingFactStatusVoid, "2026-01-02", scope)

	facts := make([]SupplierAccountingFact, SupplierAccountingFactRetentionChunkSize+1)
	for index := range facts {
		attempt := fmt.Sprintf("eligible-%05d", index)
		facts[index] = SupplierAccountingFact{AttemptId: attempt, ParentRequestId: attempt, PreparedAt: 1, PreparedDay: preparedDay,
			CoverageScope: scope, Status: SupplierAccountingFactStatusCaptured, SupplierId: 1, ContractId: 1,
			BindingVersionId: 1, RateVersionId: 1, ChannelId: 1, ModelName: "gpt-test"}
	}
	facts[len(facts)-1].Status = SupplierAccountingFactStatusVoid
	require.NoError(t, db.CreateInBatches(&facts, 500).Error)
	watermark := facts[len(facts)-1].Id
	aboveWatermark := seed("above-watermark", SupplierAccountingFactStatusCaptured, preparedDay, scope)

	deleted, err := DeleteSupplierAccountingFactRetentionChunk(ctx, db, preparedDay, watermark)
	require.NoError(t, err)
	require.EqualValues(t, SupplierAccountingFactRetentionChunkSize, deleted.Selected)
	require.EqualValues(t, SupplierAccountingFactRetentionChunkSize, deleted.Deleted)
	deleted, err = DeleteSupplierAccountingFactRetentionChunk(ctx, db, preparedDay, watermark)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted.Selected)
	require.EqualValues(t, 1, deleted.Deleted)
	deleted, err = DeleteSupplierAccountingFactRetentionChunk(ctx, db, preparedDay, watermark)
	require.NoError(t, err)
	require.Zero(t, deleted.Selected)
	require.Zero(t, deleted.Deleted)

	for _, preserved := range []SupplierAccountingFact{pending, nonTerminal, wrongScope, wrongDay, aboveWatermark} {
		var count int64
		require.NoError(t, db.Model(&SupplierAccountingFact{}).Where("id = ?", preserved.Id).Count(&count).Error)
		require.EqualValues(t, 1, count, preserved.AttemptId)
	}
}
