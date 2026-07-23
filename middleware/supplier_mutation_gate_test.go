package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	backendi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSupplierMutationGateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	require.NoError(t, backendi18n.Init())
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	previous := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previous
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	return db
}

func performSupplierMutationGateRequest(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/mutation", SupplierMutationGate(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/mutation", nil))
	return recorder
}

func TestSupplierMutationGateFailsClosedAndReadsMainDBEachRequest(t *testing.T) {
	db := setupSupplierMutationGateTestDB(t)

	missing := performSupplierMutationGateRequest(t)
	require.Equal(t, http.StatusLocked, missing.Code)
	require.Contains(t, missing.Body.String(), SupplierMutationGateDisabledCode)

	_, err := model.CASSupplierAccountingMutationState(db, 0, true, 7, "enable", 100)
	require.NoError(t, err)
	enabled := performSupplierMutationGateRequest(t)
	require.Equal(t, http.StatusOK, enabled.Code)

	_, err = model.CASSupplierAccountingMutationState(db, 1, false, 7, "disable", 101)
	require.NoError(t, err)
	disabled := performSupplierMutationGateRequest(t)
	require.Equal(t, http.StatusLocked, disabled.Code, "every request must observe the latest main-DB state")

	require.NoError(t, db.Model(&model.Option{}).
		Where("key = ?", model.SupplierAccountingMutationOptionKey).
		UpdateColumn("value", `{"schema_version":1,"state_version":3,"enabled":true,"unknown":1}`).Error)
	malformed := performSupplierMutationGateRequest(t)
	require.Equal(t, http.StatusServiceUnavailable, malformed.Code)
	require.Contains(t, malformed.Body.String(), SupplierMutationGateUnavailableCode)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	databaseFailure := performSupplierMutationGateRequest(t)
	require.Equal(t, http.StatusServiceUnavailable, databaseFailure.Code)
	require.Contains(t, databaseFailure.Body.String(), SupplierMutationGateUnavailableCode)
}
