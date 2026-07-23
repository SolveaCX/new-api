package middleware

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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

func TestSupplierBatchAuthTokenRotationUsesStableIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	current := supplierBatchTestToken(1)
	next := supplierBatchTestToken(2)
	t.Setenv(SupplierBatchCurrentVerifierHashEnv, supplierBatchTestVerifier(current))
	t.Setenv(SupplierBatchNextVerifierHashEnv, supplierBatchTestVerifier(next))
	t.Setenv(SupplierBatchTrustedIdentityEnv, "supplier-daily-runner")

	engine := gin.New()
	engine.GET("/batch", SupplierBatchAuth(), func(c *gin.Context) {
		principal, ok := SupplierBatchPrincipalFromContext(c)
		require.True(t, ok)
		c.JSON(http.StatusOK, gin.H{"identity": principal.TrustedJobIdentity, "slot": principal.AuditSlot})
	})

	for _, test := range []struct {
		name, token, slot string
	}{
		{name: "current", token: current, slot: "current"},
		{name: "next", token: next, slot: "next"},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/batch", nil)
			request.Header.Set("Authorization", "Bearer "+test.token)
			engine.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.JSONEq(t, `{"identity":"supplier-daily-runner","slot":"`+test.slot+`"}`, recorder.Body.String())
		})
	}
}

func TestSupplierBatchAuthFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := supplierBatchTestToken(7)
	verifier := supplierBatchTestVerifier(token)
	tests := []struct {
		name, currentVerifier, identity, bearer, code string
		wantStatus                                    int
	}{
		{name: "missing verifier", identity: "runner", bearer: token, wantStatus: http.StatusServiceUnavailable, code: "verifier_unavailable"},
		{name: "malformed verifier", currentVerifier: "not-a-sha256", identity: "runner", bearer: token, wantStatus: http.StatusServiceUnavailable, code: "verifier_unavailable"},
		{name: "missing identity", currentVerifier: verifier, bearer: token, wantStatus: http.StatusServiceUnavailable, code: "config_unavailable"},
		{name: "missing bearer", currentVerifier: verifier, identity: "runner", wantStatus: http.StatusUnauthorized, code: "unauthorized"},
		{name: "invalid bearer", currentVerifier: verifier, identity: "runner", bearer: supplierBatchTestToken(8), wantStatus: http.StatusUnauthorized, code: "unauthorized"},
		{name: "verifier hash as bearer", currentVerifier: verifier, identity: "runner", bearer: verifier, wantStatus: http.StatusUnauthorized, code: "unauthorized"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(SupplierBatchCurrentVerifierHashEnv, test.currentVerifier)
			t.Setenv(SupplierBatchNextVerifierHashEnv, "")
			t.Setenv(SupplierBatchTrustedIdentityEnv, test.identity)
			called := false
			engine := gin.New()
			engine.GET("/batch", SupplierBatchAuth(), func(c *gin.Context) { called = true })
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/batch", nil)
			if test.bearer != "" {
				request.Header.Set("Authorization", "Bearer "+test.bearer)
			}
			engine.ServeHTTP(recorder, request)
			require.Equal(t, test.wantStatus, recorder.Code)
			require.Contains(t, recorder.Body.String(), `"code":"`+test.code+`"`)
			require.False(t, called)
			require.NotContains(t, recorder.Body.String(), token)
			require.NotContains(t, recorder.Body.String(), verifier)
		})
	}
}

func supplierBatchTestToken(fill byte) string {
	return base64.RawURLEncoding.EncodeToString([]byte{
		fill, fill, fill, fill, fill, fill, fill, fill,
		fill, fill, fill, fill, fill, fill, fill, fill,
		fill, fill, fill, fill, fill, fill, fill, fill,
		fill, fill, fill, fill, fill, fill, fill, fill,
	})
}

func supplierBatchTestVerifier(token string) string {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}
