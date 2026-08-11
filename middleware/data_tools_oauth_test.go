package middleware

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDataToolsAuthTestDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/data-tools-auth.db?_pragma=busy_timeout(10000)&_txlock=immediate"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.McpOAuthGrant{},
	))
	model.DB = db
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
		model.DB = previousDB
		common.RedisEnabled = previousRedisEnabled
	})
}

func setupDataToolsAuthSignerEnv(t *testing.T) *service.McpOAuthSigner {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	t.Setenv("FLATKEY_MCP_OAUTH_ED25519_PRIVATE_KEY", base64.StdEncoding.EncodeToString(der))
	signer, err := service.NewMcpOAuthSignerFromEnv(service.McpOAuthSigningConfig{})
	require.NoError(t, err)
	return signer
}

func seedDataToolsAuthUser(t *testing.T, suffix string) model.User {
	t.Helper()
	user := model.User{
		Username: fmt.Sprintf("data-tools-%s", suffix),
		Password: "password",
		Group:    "default",
		Quota:    100000,
		Status:   common.UserStatusEnabled,
		AffCode:  fmt.Sprintf("data-tools-%s-aff", suffix),
	}
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}

func seedDataToolsNormalToken(t *testing.T, userID int, key string) model.Token {
	t.Helper()
	token := model.Token{
		UserId:         userID,
		Key:            key,
		Name:           key,
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		RemainQuota:    5000,
		UnlimitedQuota: true,
		Group:          "default",
	}
	require.NoError(t, model.DB.Create(&token).Error)
	return token
}

func seedDataToolsOAuthGrant(t *testing.T, userID int, scope string, status int) (model.McpOAuthGrant, model.Token) {
	t.Helper()
	grant := model.McpOAuthGrant{
		PublicID:    fmt.Sprintf("grant_%d_%s", userID, strings.ReplaceAll(scope, ":", "_")),
		ClientID:    fmt.Sprintf("client_%d", userID),
		UserID:      userID,
		AccountID:   userID,
		Resource:    service.McpOAuthResource,
		DisplayName: "Data Tools Test",
		Status:      model.McpOAuthGrantStatusActive,
		Scope:       scope,
		CreatedTime: 100,
		UpdatedTime: 100,
		LastUsedAt:  100,
	}
	require.NoError(t, model.DB.Create(&grant).Error)
	tokenGrantID := grant.PublicID
	token := model.Token{
		UserId:         userID,
		Key:            fmt.Sprintf("oauth-dedicated-%d", userID),
		Name:           "oauth dedicated",
		Status:         status,
		ExpiredTime:    -1,
		RemainQuota:    9000,
		UnlimitedQuota: true,
		Group:          "default",
		Source:         model.TokenSourceMcpOAuth,
		OAuthGrantId:   &tokenGrantID,
	}
	require.NoError(t, model.DB.Create(&token).Error)
	require.NoError(t, model.DB.Model(&grant).Updates(map[string]any{"dedicated_token_id": token.Id}).Error)
	grant.DedicatedTokenId = &token.Id
	return grant, token
}

func signDataToolsOAuthJWT(t *testing.T, signer *service.McpOAuthSigner, grant model.McpOAuthGrant, scopes []string) string {
	t.Helper()
	raw, err := signer.SignAccessToken(service.McpOAuthAccessTokenRequest{
		Subject:  fmt.Sprintf("user-%d", grant.UserID),
		GrantID:  grant.PublicID,
		ClientID: grant.ClientID,
		Scopes:   scopes,
		Resource: service.McpOAuthResource,
	})
	require.NoError(t, err)
	return raw
}

func dataToolsAuthProbeRouter(t *testing.T, requiredScope string, allowSession bool, assertions func(*gin.Context)) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("data-tools-auth-test"))))
	engine.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", 4242)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	engine.Any("/probe", DataToolsAuth(requiredScope, allowSession), func(c *gin.Context) {
		if assertions != nil {
			assertions(c)
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	return engine
}

func performDataToolsAuthRequest(router http.Handler, method string, auth string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, "/probe", nil)
	if auth != "" {
		request.Header.Set("Authorization", auth)
	}
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func performDataToolsAuthRequestFromIP(router http.Handler, method string, auth string, remoteAddr string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, "/probe", nil)
	request.RemoteAddr = remoteAddr
	if auth != "" {
		request.Header.Set("Authorization", auth)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestDataToolAuthOAuthScopesAndBillingContext(t *testing.T) {
	setupDataToolsAuthTestDB(t)
	signer := setupDataToolsAuthSignerEnv(t)

	cases := []struct {
		name          string
		method        string
		requiredScope string
		scopes        []string
		allowSession  bool
	}{
		{name: "search requires tools search", method: http.MethodGet, requiredScope: "tools:search", scopes: []string{"tools:search"}, allowSession: true},
		{name: "inspect requires tools read", method: http.MethodGet, requiredScope: "tools:read", scopes: []string{"tools:read"}, allowSession: true},
		{name: "run requires tools execute", method: http.MethodPost, requiredScope: "tools:execute", scopes: []string{"tools:execute"}, allowSession: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user := seedDataToolsAuthUser(t, strings.ReplaceAll(tc.name, " ", "-"))
			grant, token := seedDataToolsOAuthGrant(t, user.Id, strings.Join(tc.scopes, " "), common.TokenStatusEnabled)
			rawJWT := signDataToolsOAuthJWT(t, signer, grant, tc.scopes)

			router := dataToolsAuthProbeRouter(t, tc.requiredScope, tc.allowSession, func(c *gin.Context) {
				require.Equal(t, user.Id, c.GetInt("id"))
				require.Equal(t, token.Id, c.GetInt("token_id"))
				require.Equal(t, token.Key, c.GetString("token_key"))
				require.True(t, c.GetBool("token_unlimited_quota"))
				require.Equal(t, grant.PublicID, common.GetContextKeyString(c, constant.ContextKeyOAuthGrantId))
			})

			response := performDataToolsAuthRequest(router, tc.method, "Bearer "+rawJWT)

			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var refreshed model.McpOAuthGrant
			require.NoError(t, model.DB.First(&refreshed, "public_id = ?", grant.PublicID).Error)
			require.Greater(t, refreshed.LastUsedAt, int64(100))
		})
	}
}

func TestDataToolAuthOAuthFailsClosed(t *testing.T) {
	setupDataToolsAuthTestDB(t)
	signer := setupDataToolsAuthSignerEnv(t)
	user := seedDataToolsAuthUser(t, "fail-closed")
	grant, token := seedDataToolsOAuthGrant(t, user.Id, "tools:read", common.TokenStatusEnabled)
	readJWT := signDataToolsOAuthJWT(t, signer, grant, []string{"tools:read"})

	t.Run("wrong scope", func(t *testing.T) {
		router := dataToolsAuthProbeRouter(t, "tools:execute", false, nil)
		response := performDataToolsAuthRequest(router, http.MethodPost, "Bearer "+readJWT)
		require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
	})

	t.Run("grant scope mismatch fails closed", func(t *testing.T) {
		require.NoError(t, model.DB.Model(&model.McpOAuthGrant{}).Where("public_id = ?", grant.PublicID).Updates(map[string]any{
			"status":       model.McpOAuthGrantStatusActive,
			"revoked_at":   int64(0),
			"last_used_at": int64(100),
		}).Error)
		require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).Update("status", common.TokenStatusEnabled).Error)
		executeJWT := signDataToolsOAuthJWT(t, signer, grant, []string{"tools:execute"})
		router := dataToolsAuthProbeRouter(t, "tools:execute", false, nil)
		response := performDataToolsAuthRequest(router, http.MethodPost, "Bearer "+executeJWT)
		require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
		var refreshed model.McpOAuthGrant
		require.NoError(t, model.DB.First(&refreshed, "public_id = ?", grant.PublicID).Error)
		require.Equal(t, int64(100), refreshed.LastUsedAt)
	})

	t.Run("missing signer env", func(t *testing.T) {
		t.Setenv("FLATKEY_MCP_OAUTH_ED25519_PRIVATE_KEY", "")
		router := dataToolsAuthProbeRouter(t, "tools:read", true, nil)
		response := performDataToolsAuthRequest(router, http.MethodGet, "Bearer "+readJWT)
		require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
	})

	t.Run("revoked grant does not update last used", func(t *testing.T) {
		require.NoError(t, model.DB.Model(&model.McpOAuthGrant{}).Where("public_id = ?", grant.PublicID).Updates(map[string]any{
			"status":       model.McpOAuthGrantStatusRevoked,
			"revoked_at":   int64(1234),
			"last_used_at": int64(100),
		}).Error)
		router := dataToolsAuthProbeRouter(t, "tools:read", true, nil)
		response := performDataToolsAuthRequest(router, http.MethodGet, "Bearer "+readJWT)
		require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
		var refreshed model.McpOAuthGrant
		require.NoError(t, model.DB.First(&refreshed, "public_id = ?", grant.PublicID).Error)
		require.Equal(t, int64(100), refreshed.LastUsedAt)
	})

	t.Run("disabled dedicated token does not update last used", func(t *testing.T) {
		require.NoError(t, model.DB.Model(&model.McpOAuthGrant{}).Where("public_id = ?", grant.PublicID).Updates(map[string]any{
			"status":       model.McpOAuthGrantStatusActive,
			"revoked_at":   int64(0),
			"last_used_at": int64(100),
		}).Error)
		require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).Update("status", common.TokenStatusDisabled).Error)
		router := dataToolsAuthProbeRouter(t, "tools:read", true, nil)
		response := performDataToolsAuthRequest(router, http.MethodGet, "Bearer "+readJWT)
		require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
		var refreshed model.McpOAuthGrant
		require.NoError(t, model.DB.First(&refreshed, "public_id = ?", grant.PublicID).Error)
		require.Equal(t, int64(100), refreshed.LastUsedAt)
	})

	t.Run("disabled user does not update last used", func(t *testing.T) {
		require.NoError(t, model.DB.Model(&model.McpOAuthGrant{}).Where("public_id = ?", grant.PublicID).Updates(map[string]any{
			"status":       model.McpOAuthGrantStatusActive,
			"revoked_at":   int64(0),
			"last_used_at": int64(100),
		}).Error)
		require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).Update("status", common.TokenStatusEnabled).Error)
		require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("status", common.UserStatusDisabled).Error)
		t.Cleanup(func() {
			_ = model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("status", common.UserStatusEnabled).Error
		})
		router := dataToolsAuthProbeRouter(t, "tools:read", true, nil)
		response := performDataToolsAuthRequest(router, http.MethodGet, "Bearer "+readJWT)
		require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
		var refreshed model.McpOAuthGrant
		require.NoError(t, model.DB.First(&refreshed, "public_id = ?", grant.PublicID).Error)
		require.Equal(t, int64(100), refreshed.LastUsedAt)
	})

	t.Run("group drift does not update last used", func(t *testing.T) {
		originalUsableGroups := setting.UserUsableGroups2JSONString()
		originalGroupRatio := ratio_setting.GroupRatio2JSONString()
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1}`))
		t.Cleanup(func() {
			_ = setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups)
			_ = ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio)
		})
		require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Updates(map[string]any{
			"status": common.UserStatusEnabled,
			"group":  "default",
		}).Error)
		require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).Updates(map[string]any{
			"status": common.TokenStatusEnabled,
			"group":  "retired",
		}).Error)
		require.NoError(t, model.DB.Model(&model.McpOAuthGrant{}).Where("public_id = ?", grant.PublicID).Updates(map[string]any{
			"status":       model.McpOAuthGrantStatusActive,
			"revoked_at":   int64(0),
			"last_used_at": int64(100),
		}).Error)
		router := dataToolsAuthProbeRouter(t, "tools:read", true, nil)
		response := performDataToolsAuthRequest(router, http.MethodGet, "Bearer "+readJWT)
		require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
		var refreshed model.McpOAuthGrant
		require.NoError(t, model.DB.First(&refreshed, "public_id = ?", grant.PublicID).Error)
		require.Equal(t, int64(100), refreshed.LastUsedAt)
	})
}

func TestDataToolAuthPreservesAPIKeyAndSessionBehavior(t *testing.T) {
	setupDataToolsAuthTestDB(t)
	user := seedDataToolsAuthUser(t, "regression")
	normalToken := seedDataToolsNormalToken(t, user.Id, "normaldatatoolskey")

	t.Run("ordinary api key reaches run context", func(t *testing.T) {
		router := dataToolsAuthProbeRouter(t, "tools:execute", false, func(c *gin.Context) {
			require.Equal(t, user.Id, c.GetInt("id"))
			require.Equal(t, normalToken.Id, c.GetInt("token_id"))
			require.Equal(t, normalToken.Key, c.GetString("token_key"))
			require.True(t, c.GetBool("token_unlimited_quota"))
			_, exists := common.GetContextKey(c, constant.ContextKeyOAuthGrantId)
			require.False(t, exists)
		})
		response := performDataToolsAuthRequest(router, http.MethodPost, "Bearer sk-"+normalToken.Key)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	})

	t.Run("ordinary api key rejects client ip outside token allowlist", func(t *testing.T) {
		allowIps := "203.0.113.0/24"
		require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", normalToken.Id).Update("allow_ips", allowIps).Error)
		router := dataToolsAuthProbeRouter(t, "tools:execute", false, nil)

		response := performDataToolsAuthRequestFromIP(router, http.MethodPost, "Bearer sk-"+normalToken.Key, "198.51.100.10:12345")

		require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
	})

	t.Run("list and inspect allow dashboard session", func(t *testing.T) {
		router := dataToolsAuthProbeRouter(t, "tools:search", true, func(c *gin.Context) {
			require.Equal(t, 4242, c.GetInt("id"))
			require.Zero(t, c.GetInt("token_id"))
		})
		login := httptest.NewRecorder()
		router.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/login", nil))
		require.Equal(t, http.StatusNoContent, login.Code)
		response := performDataToolsAuthRequest(router, http.MethodGet, "", login.Result().Cookies()...)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	})

	t.Run("run rejects session only", func(t *testing.T) {
		router := dataToolsAuthProbeRouter(t, "tools:execute", false, nil)
		login := httptest.NewRecorder()
		router.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/login", nil))
		require.Equal(t, http.StatusNoContent, login.Code)
		response := performDataToolsAuthRequest(router, http.MethodPost, "", login.Result().Cookies()...)
		require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
	})

	t.Run("invalid api key does not fall back to session", func(t *testing.T) {
		router := dataToolsAuthProbeRouter(t, "tools:search", true, nil)
		login := httptest.NewRecorder()
		router.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/login", nil))
		require.Equal(t, http.StatusNoContent, login.Code)
		response := performDataToolsAuthRequest(router, http.MethodGet, "Bearer sk-missing-normal-key", login.Result().Cookies()...)
		require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
	})
}
