package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHandleEmailUnsubscribe_InvalidParams(t *testing.T) {
	cases := []string{
		"/api/email/unsubscribe",                  // 无参数
		"/api/email/unsubscribe?uid=abc&token=x",  // uid 非数字
		"/api/email/unsubscribe?uid=0&token=x",    // uid<=0
		"/api/email/unsubscribe?uid=1",            // 缺 token
	}
	for _, url := range cases {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, url, nil)
		HandleEmailUnsubscribe(c)
		require.Equal(t, http.StatusBadRequest, w.Code, "url=%s 应返回 400", url)
	}
}

func TestHandleEmailUnsubscribe_InvalidToken(t *testing.T) {
	original := common.SessionSecret
	t.Cleanup(func() { common.SessionSecret = original })
	common.SessionSecret = "test-secret-key"

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/email/unsubscribe?uid=123&token=wrongtoken", nil)
	HandleEmailUnsubscribe(c)
	require.Equal(t, http.StatusBadRequest, w.Code, "无效 token 应返回 400,不触达 DB")
}
