package middleware

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSelfServicePaymentUsesAuthoritativeIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	i18n.Init()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "payments.db")), &gorm.Config{})
	require.NoError(t, err)
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB; sqlDB, _ := db.DB(); _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.User{}))
	user := model.User{Id: 1, Username: "payment-test", Group: "plg", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	engine := gin.New()
	id := 1
	engine.Use(func(c *gin.Context) { c.Set("id", id); c.Set("group", "plg") })
	reached := false
	engine.POST("/pay", RequireSelfServicePayment(), func(c *gin.Context) { reached = true; c.Status(http.StatusNoContent) })
	request := func(want int) {
		t.Helper()
		reached = false
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/pay?group=plg", nil))
		require.Equal(t, want, response.Code)
		require.Equal(t, want == http.StatusNoContent, reached)
	}
	request(http.StatusNoContent)
	for _, group := range []string{"default", "slg", "enterprise", "", " PLG "} {
		require.NoError(t, db.Model(&model.User{}).Where("id = ?", 1).Update("group", group).Error)
		want := http.StatusForbidden
		if group == " PLG " {
			want = http.StatusNoContent
		}
		request(want)
	}
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 1).Update("status", common.UserStatusDisabled).Error)
	request(http.StatusForbidden)
	id = 0
	request(http.StatusForbidden)
	id = 999
	request(http.StatusForbidden)
	id = 1
	require.NoError(t, db.Migrator().DropTable(&model.User{}))
	request(http.StatusForbidden)
}
