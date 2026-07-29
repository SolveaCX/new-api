package controller

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSupplierHistoricalControllerDB(t *testing.T) (*gorm.DB, int, int, int) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.UpstreamSupplier{}, &model.SupplierContract{}, &model.SupplierContractRateVersion{},
		&model.SupplierHistoricalImport{}, &model.SupplierHistoricalDailySummary{}, &model.SupplierHistoricalPublishedDay{},
	))
	supplier := model.UpstreamSupplier{Name: "controller historical supplier"}
	require.NoError(t, db.Create(&supplier).Error)
	contract := model.SupplierContract{SupplierId: supplier.Id, Name: "controller historical contract", ContractNo: "HC-1"}
	require.NoError(t, db.Create(&contract).Error)
	rate := model.SupplierContractRateVersion{ContractId: contract.Id, ProcurementMultiplierPpm: 650_000, CreatedBy: 7}
	require.NoError(t, db.Create(&rate).Error)
	previous := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previous })
	return db, supplier.Id, contract.Id, rate.Id
}

func TestCreateSupplierHistoricalEstimateImportRequiresIdempotencyAndValidCommand(t *testing.T) {
	_, supplierID, contractID, rateID := setupSupplierHistoricalControllerDB(t)
	body := `{"start_date":"2026-01-01","end_date":"2026-01-02","quota_per_unit":"500000","excluded_user_ids":[1],"channel_mappings":[{"channel_id":11,"supplier_id":` + strconv.Itoa(supplierID) + `,"contract_id":` + strconv.Itoa(contractID) + `,"rate_version_id":` + strconv.Itoa(rateID) + `,"procurement_multiplier_ppm":650000}],"reason":"legacy estimate"}`

	missingKey := performSupplyChainControllerRequestAt(http.MethodPost, "/historical-imports", "/historical-imports", body, CreateSupplierHistoricalEstimateImport)
	require.Equal(t, http.StatusBadRequest, missingKey.Code)
	created := performSupplyChainControllerRequestWithHeaders(http.MethodPost, "/historical-imports", "/historical-imports", body, map[string]string{"Idempotency-Key": "history-command"}, CreateSupplierHistoricalEstimateImport)
	require.Equal(t, http.StatusOK, created.Code)
	require.Contains(t, created.Body.String(), `"estimate_only":true`)
	require.Contains(t, created.Body.String(), `"coverage_scope":"historical_consume_logs_v1"`)
	require.Contains(t, created.Body.String(), `"status":"pending"`)

	replayed := performSupplyChainControllerRequestWithHeaders(http.MethodPost, "/historical-imports", "/historical-imports", body, map[string]string{"Idempotency-Key": "history-command"}, CreateSupplierHistoricalEstimateImport)
	require.Equal(t, http.StatusOK, replayed.Code)
}

func TestSupplierHistoricalEstimateImportListGetAndSeries(t *testing.T) {
	db, supplierID, contractID, rateID := setupSupplierHistoricalControllerDB(t)
	body := `{"start_date":"2026-01-01","end_date":"2026-01-02","quota_per_unit":"500000","excluded_user_ids":[],"channel_mappings":[{"channel_id":11,"supplier_id":` + strconv.Itoa(supplierID) + `,"contract_id":` + strconv.Itoa(contractID) + `,"rate_version_id":` + strconv.Itoa(rateID) + `,"procurement_multiplier_ppm":650000}],"reason":"legacy estimate"}`
	created := performSupplyChainControllerRequestWithHeaders(http.MethodPost, "/historical-imports", "/historical-imports", body, map[string]string{"Idempotency-Key": "history-list"}, CreateSupplierHistoricalEstimateImport)
	require.Equal(t, http.StatusOK, created.Code)
	var item model.SupplierHistoricalImport
	require.NoError(t, db.First(&item).Error)
	require.NoError(t, db.Create(&model.SupplierHistoricalDailySummary{ImportId: item.Id, Date: "2026-01-01", DimensionKey: "series", DataQuality: model.SupplierHistoricalDataQualityEstimated, SourceRequestCount: 1}).Error)

	list := performSupplyChainControllerRequest(http.MethodGet, "/historical-imports?p=1&page_size=20", "", ListSupplierHistoricalEstimateImports)
	require.Equal(t, http.StatusOK, list.Code)
	require.Contains(t, list.Body.String(), `"total":1`)
	get := performSupplyChainControllerRequestAt(http.MethodGet, "/historical-imports/:id", "/historical-imports/"+strconv.FormatInt(item.Id, 10), "", GetSupplierHistoricalEstimateImport)
	require.Equal(t, http.StatusOK, get.Code)
	require.Contains(t, get.Body.String(), `"reason":"legacy estimate"`)
	for _, status := range []string{model.SupplierHistoricalImportStatusPending, model.SupplierHistoricalImportStatusRunning, model.SupplierHistoricalImportStatusFailed} {
		require.NoError(t, db.Model(&model.SupplierHistoricalImport{}).Where("id = ?", item.Id).UpdateColumn("status", status).Error)
		notReady := performSupplyChainControllerRequestAt(http.MethodGet, "/historical-imports/:id/summaries", "/historical-imports/"+strconv.FormatInt(item.Id, 10)+"/summaries", "", ListSupplierHistoricalEstimateSummaries)
		require.Equal(t, http.StatusConflict, notReady.Code, status)
	}
	require.NoError(t, db.Model(&model.SupplierHistoricalImport{}).Where("id = ?", item.Id).UpdateColumn("status", model.SupplierHistoricalImportStatusCompleted).Error)
	series := performSupplyChainControllerRequestAt(http.MethodGet, "/historical-imports/:id/summaries", "/historical-imports/"+strconv.FormatInt(item.Id, 10)+"/summaries", "", ListSupplierHistoricalEstimateSummaries)
	require.Equal(t, http.StatusOK, series.Code)
	require.Contains(t, series.Body.String(), `"data_quality":"estimated"`)
	published := performSupplyChainControllerRequestAt(http.MethodPost, "/historical-imports/:id/publish", "/historical-imports/"+strconv.FormatInt(item.Id, 10)+"/publish", "", PublishSupplierHistoricalEstimateImport)
	require.Equal(t, http.StatusOK, published.Code)
	require.Contains(t, published.Body.String(), `"publication_status":"published"`)
	require.Contains(t, published.Body.String(), `"affects_inventory":false`)
	var publishedDays int64
	require.NoError(t, db.Model(&model.SupplierHistoricalPublishedDay{}).Where("import_id = ?", item.Id).Count(&publishedDays).Error)
	require.Equal(t, int64(1), publishedDays)
}
