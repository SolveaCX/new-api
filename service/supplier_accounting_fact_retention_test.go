package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedSupplierAccountingRetentionFixture(t *testing.T, mainDB, logDB *gorm.DB) model.SupplierAccountingFact {
	t.Helper()
	preparedDay := time.Now().In(time.FixedZone("Asia/Shanghai", 8*60*60)).AddDate(0, 0, -60).Format("2006-01-02")
	fact := model.SupplierAccountingFact{AttemptId: "retention-fixture", ParentRequestId: "retention-fixture", PreparedAt: 1,
		PreparedDay: preparedDay, CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
		Status: model.SupplierAccountingFactStatusCaptured, SupplierId: 1, ContractId: 1, BindingVersionId: 1,
		RateVersionId: 1, ChannelId: 1, ModelName: "gpt-test"}
	require.NoError(t, logDB.Create(&fact).Error)
	require.NoError(t, mainDB.Create(&model.SupplierUsageDailyBatchRun{BatchDate: preparedDay,
		Status: model.SupplierDailyBatchStatusCompleted, PublishedFenceToken: 1, SourceMaxFactId: fact.Id,
		CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1)}).Error)
	return fact
}

func requireSupplierAccountingFactExists(t *testing.T, db *gorm.DB, id int64, expected bool) {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&model.SupplierAccountingFact{}).Where("id = ?", id).Count(&count).Error)
	if expected {
		require.EqualValues(t, 1, count)
	} else {
		require.Zero(t, count)
	}
}

func TestRunSupplierAccountingFactRetentionOnceDefaultsDisabledAndRejectsInvalidConfiguration(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	fact := seedSupplierAccountingRetentionFixture(t, mainDB, logDB)

	for _, value := range []string{"", "0"} {
		t.Run("disabled_"+value, func(t *testing.T) {
			t.Setenv("SUPPLIER_ACCOUNTING_FACT_RETENTION_DAYS", value)
			deleted, err := RunSupplierAccountingFactRetentionOnce(context.Background(), mainDB, logDB)
			require.NoError(t, err)
			require.Zero(t, deleted)
			requireSupplierAccountingFactExists(t, logDB, fact.Id, true)
		})
	}
	for _, value := range []string{"-1", "invalid"} {
		t.Run("invalid_"+value, func(t *testing.T) {
			t.Setenv("SUPPLIER_ACCOUNTING_FACT_RETENTION_DAYS", value)
			deleted, err := RunSupplierAccountingFactRetentionOnce(context.Background(), mainDB, logDB)
			require.Error(t, err)
			require.Zero(t, deleted)
			requireSupplierAccountingFactExists(t, logDB, fact.Id, true)
		})
	}
}

func TestRunSupplierAccountingFactRetentionOnceUsesIndependentDatabases(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	fact := seedSupplierAccountingRetentionFixture(t, mainDB, logDB)
	t.Setenv("SUPPLIER_ACCOUNTING_FACT_RETENTION_DAYS", "30")

	deleted, err := RunSupplierAccountingFactRetentionOnce(context.Background(), mainDB, logDB)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)
	requireSupplierAccountingFactExists(t, logDB, fact.Id, false)

	deleted, err = RunSupplierAccountingFactRetentionOnce(context.Background(), mainDB, logDB)
	require.NoError(t, err)
	require.Zero(t, deleted)
}

func TestRunSupplierAccountingFactRetentionOnceAdvancesPastEmptyPublishedDay(t *testing.T) {
	mainDB, logDB := supplierDailyTestDBs(t)
	scope := string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1)
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	emptyDay := time.Now().In(location).AddDate(0, 0, -61).Format("2006-01-02")
	factDay := time.Now().In(location).AddDate(0, 0, -60).Format("2006-01-02")
	fact := model.SupplierAccountingFact{AttemptId: "retention-after-empty", ParentRequestId: "retention-after-empty", PreparedAt: 1,
		PreparedDay: factDay, CoverageScope: scope, Status: model.SupplierAccountingFactStatusVoid,
		SupplierId: 1, ContractId: 1, BindingVersionId: 1, RateVersionId: 1, ChannelId: 1, ModelName: "gpt-test"}
	require.NoError(t, logDB.Create(&fact).Error)
	require.NoError(t, mainDB.Create(&[]model.SupplierUsageDailyBatchRun{
		{BatchDate: emptyDay, Status: model.SupplierDailyBatchStatusCompleted, PublishedFenceToken: 1, SourceMaxFactId: 0, CoverageScope: scope},
		{BatchDate: factDay, Status: model.SupplierDailyBatchStatusCompleted, PublishedFenceToken: 2, SourceMaxFactId: fact.Id, CoverageScope: scope},
	}).Error)
	t.Setenv("SUPPLIER_ACCOUNTING_FACT_RETENTION_DAYS", "30")
	supplierAccountingFactRetentionMu.Lock()
	supplierAccountingFactRetentionCursor = ""
	supplierAccountingFactRetentionMu.Unlock()

	deleted, err := RunSupplierAccountingFactRetentionOnce(context.Background(), mainDB, logDB)
	require.NoError(t, err)
	require.Zero(t, deleted)
	requireSupplierAccountingFactExists(t, logDB, fact.Id, true)

	deleted, err = RunSupplierAccountingFactRetentionOnce(context.Background(), mainDB, logDB)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)
	requireSupplierAccountingFactExists(t, logDB, fact.Id, false)
}
