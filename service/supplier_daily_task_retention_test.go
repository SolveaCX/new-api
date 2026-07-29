package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestSupplierDailyTaskRunsRetentionOnlyAfterSuccessfulAggregation(t *testing.T) {
	originalDB, originalLogDB := model.DB, model.LOG_DB
	t.Cleanup(func() {
		model.DB, model.LOG_DB = originalDB, originalLogDB
		supplierDailyTaskRunning.Store(false)
	})

	t.Run("successful aggregation", func(t *testing.T) {
		mainDB, logDB := supplierDailyTestDBs(t)
		require.NoError(t, mainDB.AutoMigrate(&model.SupplierHistoricalImport{}))
		fact := seedSupplierAccountingRetentionFixture(t, mainDB, logDB)
		model.DB, model.LOG_DB = mainDB, logDB
		supplierDailyTaskRunning.Store(false)
		supplierAccountingFactRetentionMu.Lock()
		supplierAccountingFactRetentionCursor = ""
		supplierAccountingFactRetentionMu.Unlock()
		t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", "")
		t.Setenv("SUPPLIER_ACCOUNTING_FACT_RETENTION_DAYS", "30")

		runSupplierDailyAggregationOnce()
		requireSupplierAccountingFactExists(t, logDB, fact.Id, false)
	})

	t.Run("failed aggregation", func(t *testing.T) {
		mainDB, logDB := supplierDailyTestDBs(t)
		require.NoError(t, mainDB.AutoMigrate(&model.SupplierHistoricalImport{}))
		fact := seedSupplierAccountingRetentionFixture(t, mainDB, logDB)
		model.DB, model.LOG_DB = mainDB, logDB
		supplierDailyTaskRunning.Store(false)
		supplierAccountingFactRetentionMu.Lock()
		supplierAccountingFactRetentionCursor = ""
		supplierAccountingFactRetentionMu.Unlock()
		t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", "invalid")
		t.Setenv("SUPPLIER_ACCOUNTING_FACT_RETENTION_DAYS", "30")

		runSupplierDailyAggregationOnce()
		requireSupplierAccountingFactExists(t, logDB, fact.Id, true)
	})

	require.False(t, supplierDailyTaskRunning.Load())
}
