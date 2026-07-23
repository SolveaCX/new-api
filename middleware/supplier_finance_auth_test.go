package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	backendi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestFinanceAuthAllowsOnlyRoot(t *testing.T) {
	require.NoError(t, backendi18n.Init())
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("finance-auth-test"))))
	engine.GET("/login/:role", func(c *gin.Context) {
		role, _ := strconv.Atoi(c.Param("role"))
		session := sessions.Default(c)
		session.Set("username", "finance-tester")
		session.Set("role", role)
		session.Set("id", 71)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	calls := 0
	engine.GET("/finance", FinanceAuth(), func(c *gin.Context) {
		calls++
		require.Equal(t, 71, c.GetInt("id"))
		c.Status(http.StatusNoContent)
	})

	request := func(role *int) *httptest.ResponseRecorder {
		var cookies []*http.Cookie
		if role != nil {
			login := httptest.NewRecorder()
			engine.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/login/"+strconv.Itoa(*role), nil))
			cookies = login.Result().Cookies()
		}
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/finance", nil)
		req.Header.Set("New-Api-User", "71")
		for _, value := range cookies {
			req.AddCookie(value)
		}
		engine.ServeHTTP(recorder, req)
		return recorder
	}

	require.Equal(t, http.StatusUnauthorized, request(nil).Code)
	admin := common.RoleAdminUser
	require.Equal(t, http.StatusOK, request(&admin).Code)
	require.Zero(t, calls)
	root := common.RoleRootUser
	require.Equal(t, http.StatusNoContent, request(&root).Code)
	require.Equal(t, 1, calls)
}
