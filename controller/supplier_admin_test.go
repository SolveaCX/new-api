package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	backendi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func performSupplyChainControllerRequest(method, path, body string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	parsed, err := url.Parse(path)
	if err != nil {
		panic(err)
	}
	return performSupplyChainControllerRequestAt(method, parsed.Path, path, body, handler)
}

func performSupplyChainControllerRequestAt(method, routePath, requestPath, body string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	return performSupplyChainControllerRequestWithHeaders(method, routePath, requestPath, body, nil, handler)
}

func performSupplyChainControllerRequestWithHeaders(method, routePath, requestPath, body string, headers map[string]string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	if err := backendi18n.Init(); err != nil {
		panic(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", 7)
		c.Next()
	})
	router.Handle(method, routePath, handler)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, requestPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestSupplyChainContractUpdateRejectsInvariantOnlyMassAssignment(t *testing.T) {
	recorder := performSupplyChainControllerRequestAt(http.MethodPatch, "/contracts/:id", "/contracts/12", `{
		"supplier_id":99,
		"status":"inactive",
		"current_rate_version_id":42
	}`, UpdateSupplyChainContract)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":false`)
}

func TestSupplierChannelBindingRequestHasCASAllowlistedFields(t *testing.T) {
	typeOf := reflect.TypeOf(dto.SupplierChannelBindingRequest{})
	require.Equal(t, 2, typeOf.NumField())
	require.Equal(t, "contract_id", strings.Split(typeOf.Field(0).Tag.Get("json"), ",")[0])
	require.Equal(t, "expected_contract_id", strings.Split(typeOf.Field(1).Tag.Get("json"), ",")[0])
}

func TestSupplyChainBindingWritesRequireObservedState(t *testing.T) {
	bind := performSupplyChainControllerRequestAt(http.MethodPut, "/channel-bindings/:channel_id", "/channel-bindings/4", `{"contract_id":9}`, BindSupplyChainChannel)
	require.Equal(t, http.StatusBadRequest, bind.Code)

	unbind := performSupplyChainControllerRequestAt(http.MethodDelete, "/channel-bindings/:channel_id", "/channel-bindings/4", "", UnbindSupplyChainChannel)
	require.Equal(t, http.StatusBadRequest, unbind.Code)
}

func TestSupplyChainAppendWritesRequireIdempotencyKey(t *testing.T) {
	supplier := performSupplyChainControllerRequest(http.MethodPost, "/suppliers", `{"name":"upstream"}`, CreateSupplyChainSupplier)
	require.Equal(t, http.StatusBadRequest, supplier.Code)
	require.Contains(t, supplier.Body.String(), "Idempotency-Key")

	contract := performSupplyChainControllerRequest(http.MethodPost, "/contracts", `{"supplier_id":1,"name":"contract","contract_no":"C-1"}`, CreateSupplyChainContract)
	require.Equal(t, http.StatusBadRequest, contract.Code)
	require.Contains(t, contract.Body.String(), "Idempotency-Key")

	rate := performSupplyChainControllerRequestAt(http.MethodPost, "/contracts/:id/rates", "/contracts/4/rates", `{"procurement_multiplier_ppm":650000}`, CreateSupplyChainRateVersion)
	require.Equal(t, http.StatusBadRequest, rate.Code)
	require.Contains(t, rate.Body.String(), "Idempotency-Key")

	inventory := performSupplyChainControllerRequestAt(http.MethodPost, "/contracts/:id/inventory", "/contracts/4/inventory", `{"delta_micro_usd":200000000000,"type":"replenishment"}`, CreateSupplyChainInventoryAdjustment)
	require.Equal(t, http.StatusBadRequest, inventory.Code)
	require.Contains(t, inventory.Body.String(), "Idempotency-Key")

	exclusion := performSupplyChainControllerRequest(http.MethodPost, "/exclusions", `{"user_id":9,"action":"exclude"}`, CreateSupplyChainExclusionRule)
	require.Equal(t, http.StatusBadRequest, exclusion.Code)
	require.Contains(t, exclusion.Body.String(), "Idempotency-Key")
}

func TestSupplyChainIntegerValidationRejectsInvalidRateAndZeroMoney(t *testing.T) {
	rate := performSupplyChainControllerRequestWithHeaders(http.MethodPost, "/contracts/:id/rates", "/contracts/4/rates", `{"procurement_multiplier_ppm":1000001}`, map[string]string{"Idempotency-Key": "invalid-rate"}, CreateSupplyChainRateVersion)
	require.Equal(t, http.StatusBadRequest, rate.Code)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("id", 7) })
	router.POST("/contracts/:id/inventory", CreateSupplyChainInventoryAdjustment)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/contracts/4/inventory", strings.NewReader(`{"delta_micro_usd":0,"type":"replenishment"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "inventory-zero")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestSupplyChainModelErrorUsesSemanticStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	conflict := httptest.NewRecorder()
	supplyChainModelError(testGinContext(conflict), model.ErrSupplierInactive)
	require.Equal(t, http.StatusConflict, conflict.Code)

	notFound := httptest.NewRecorder()
	supplyChainModelError(testGinContext(notFound), gorm.ErrRecordNotFound)
	require.Equal(t, http.StatusNotFound, notFound.Code)
}

func TestGetSupplyChainCommandResultContract(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:supply-chain-command-result-controller?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.UpstreamSupplier{}, &model.SupplierContract{}, &model.SupplierInventoryAdjustment{}, &model.SupplierStatisticsExclusionRule{}, &model.SupplierAdminCommand{}))
	require.NoError(t, model.MigrateSupplierAdminCommandLedger(db))
	require.NoError(t, model.FinalizeSupplierAdminCommandLedgerMigration(db))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	created, replayed, err := model.CreateUpstreamSupplierIdempotentForActor(&model.UpstreamSupplier{Name: "controller lookup"}, "controller-lookup-key", 7)
	require.NoError(t, err)
	require.False(t, replayed)

	found := performSupplyChainControllerRequestWithHeaders(http.MethodGet, "/commands/result", "/commands/result?scope=supplier.create", "", map[string]string{"Idempotency-Key": "controller-lookup-key"}, GetSupplyChainCommandResult)
	require.Equal(t, http.StatusOK, found.Code)
	require.Contains(t, found.Body.String(), `"scope":"supplier.create"`)
	require.Contains(t, found.Body.String(), `"resource_type":"supplier"`)
	require.Contains(t, found.Body.String(), `"resource_id":`+strconv.Itoa(created.Id))

	missing := performSupplyChainControllerRequestWithHeaders(http.MethodGet, "/commands/result", "/commands/result?scope=supplier.create", "", map[string]string{"Idempotency-Key": "missing-key"}, GetSupplyChainCommandResult)
	require.Equal(t, http.StatusNotFound, missing.Code)
	scopeMismatch := performSupplyChainControllerRequestWithHeaders(http.MethodGet, "/commands/result", "/commands/result?scope=supplier_contract.create", "", map[string]string{"Idempotency-Key": "controller-lookup-key"}, GetSupplyChainCommandResult)
	require.Equal(t, http.StatusNotFound, scopeMismatch.Code)

	_, _, err = model.CreateUpstreamSupplierIdempotentForActor(&model.UpstreamSupplier{Name: "other actor lookup"}, "other-actor-key", 8)
	require.NoError(t, err)
	actorMismatch := performSupplyChainControllerRequestWithHeaders(http.MethodGet, "/commands/result", "/commands/result?scope=supplier.create", "", map[string]string{"Idempotency-Key": "other-actor-key"}, GetSupplyChainCommandResult)
	require.Equal(t, http.StatusNotFound, actorMismatch.Code)

	contract := model.SupplierContract{SupplierId: created.Id, Name: "lookup contract", ContractNo: "lookup-contract"}
	require.NoError(t, db.Create(&contract).Error)
	adjustment := &model.SupplierInventoryAdjustment{ContractId: contract.Id, DeltaMicroUsd: 1, Type: model.SupplierInventoryAdjustmentTypeCorrection, IdempotencyKey: "inventory-controller-key", CreatedBy: 7}
	require.NoError(t, db.Create(adjustment).Error)
	inventory := performSupplyChainControllerRequestWithHeaders(http.MethodGet, "/commands/result", "/commands/result?scope="+model.SupplierInventoryCommandScope(contract.Id), "", map[string]string{"Idempotency-Key": "inventory-controller-key"}, GetSupplyChainCommandResult)
	require.Equal(t, http.StatusOK, inventory.Code)
	require.Contains(t, inventory.Body.String(), `"resource_id":`+strconv.Itoa(adjustment.Id))

	rule, err := model.CreateSupplierStatisticsExclusionRule(91, model.SupplierStatisticsActionExclude, 7, "company account", "exclusion-controller-key")
	require.NoError(t, err)
	exclusion := performSupplyChainControllerRequestWithHeaders(http.MethodGet, "/commands/result", "/commands/result?scope="+model.SupplierAdminCommandScopeCreateExclusion, "", map[string]string{"Idempotency-Key": "exclusion-controller-key"}, GetSupplyChainCommandResult)
	require.Equal(t, http.StatusOK, exclusion.Code)
	require.Contains(t, exclusion.Body.String(), `"resource_id":`+strconv.Itoa(rule.Id))

	badScope := performSupplyChainControllerRequestWithHeaders(http.MethodGet, "/commands/result", "/commands/result?scope=supplier.invalid", "", map[string]string{"Idempotency-Key": "controller-lookup-key"}, GetSupplyChainCommandResult)
	require.Equal(t, http.StatusBadRequest, badScope.Code)
	badKey := performSupplyChainControllerRequest(http.MethodGet, "/commands/result?scope=supplier.create", "", GetSupplyChainCommandResult)
	require.Equal(t, http.StatusBadRequest, badKey.Code)
	longKey := performSupplyChainControllerRequestWithHeaders(http.MethodGet, "/commands/result", "/commands/result?scope=supplier.create", "", map[string]string{"Idempotency-Key": strings.Repeat("k", 129)}, GetSupplyChainCommandResult)
	require.Equal(t, http.StatusBadRequest, longKey.Code)
}

func testGinContext(recorder *httptest.ResponseRecorder) *gin.Context {
	if err := backendi18n.Init(); err != nil {
		panic(err)
	}
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return context
}
