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

func setupSupplyChainControllerAdminLedger(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(
		&model.SupplierInventoryAdjustment{},
	))
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

func TestSupplyChainSupplierUpdateRequiresVersionAndRejectsStaleWrite(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.UpstreamSupplier{}, &model.SupplierContract{}))
	setupSupplyChainControllerAdminLedger(t, db)
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })
	supplier := model.UpstreamSupplier{Name: "controller version supplier"}
	require.NoError(t, db.Create(&supplier).Error)
	list := performSupplyChainControllerRequest(http.MethodGet, "/suppliers", "", ListSupplyChainSuppliers)
	require.Equal(t, http.StatusOK, list.Code)
	require.Contains(t, list.Body.String(), `"row_version":1`, "supplier list projection exposes the CAS token required by clients")

	missingVersion := performSupplyChainControllerRequestWithHeaders(http.MethodPatch, "/suppliers/:id", "/suppliers/"+strconv.Itoa(supplier.Id), `{"name":"updated"}`, map[string]string{"Idempotency-Key": "update-command"}, UpdateSupplyChainSupplier)
	require.Equal(t, http.StatusBadRequest, missingVersion.Code)
	first := performSupplyChainControllerRequestAt(http.MethodPatch, "/suppliers/:id", "/suppliers/"+strconv.Itoa(supplier.Id), `{"name":"updated","expected_version":1}`, UpdateSupplyChainSupplier)
	require.Equal(t, http.StatusOK, first.Code)
	require.Contains(t, first.Body.String(), `"row_version":2`)
	stale := performSupplyChainControllerRequestAt(http.MethodPatch, "/suppliers/:id", "/suppliers/"+strconv.Itoa(supplier.Id), `{"name":"stale","expected_version":1}`, UpdateSupplyChainSupplier)
	require.Equal(t, http.StatusConflict, stale.Code)
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

func testGinContext(recorder *httptest.ResponseRecorder) *gin.Context {
	if err := backendi18n.Init(); err != nil {
		panic(err)
	}
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return context
}
