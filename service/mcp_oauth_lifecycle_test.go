package service

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMcpOAuthLifecycleTestDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/mcp-oauth-lifecycle.db?_pragma=busy_timeout(10000)&_txlock=immediate"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.McpOAuthClient{},
		&model.McpOAuthGrant{},
		&model.McpOAuthAuthorizationCode{},
		&model.McpOAuthRefreshToken{},
	))
	model.DB = db
	t.Cleanup(func() {
		sqlDB.Close()
		model.DB = previousDB
	})
}

func TestMcpOAuthLifecycleApprovalCreatesHiddenDedicatedTokenAndDeduplicatesDoubleClick(t *testing.T) {
	setupMcpOAuthLifecycleTestDB(t)
	t.Setenv("FLATKEY_MCP_OAUTH_TOKEN_REMAIN_QUOTA", "123456")
	t.Setenv("FLATKEY_MCP_OAUTH_TOKEN_UNLIMITED_QUOTA", "false")
	user := model.User{Username: "approval-user", Password: "password", AffCode: "approval-user-aff"}
	require.NoError(t, model.DB.Create(&user).Error)
	lifecycle := testMcpOAuthLifecycle(t)
	client := McpOAuthClientMetadata{
		ClientID:     "https://client.example/oauth/client",
		ClientName:   "Approval Client",
		RedirectURIs: []string{"https://client.example/callback"},
	}

	approved, err := lifecycle.ApproveMcpOAuthAuthorization(McpOAuthAuthorizationApprovalRequest{
		UserID:              user.Id,
		Client:              client,
		Resource:            McpOAuthResource,
		RedirectURI:         "https://client.example/callback",
		Scope:               "tools:read tools:search",
		CodeChallenge:       "pkce-challenge",
		CodeChallengeMethod: "S256",
	})

	require.NoError(t, err)
	require.NotEmpty(t, approved.Code)
	require.Equal(t, "tools:search tools:read", approved.Scope)
	require.NotContains(t, testMcpOAuthJSONString(t, approved), "mcp-token")

	var storedCode model.McpOAuthAuthorizationCode
	require.NoError(t, model.DB.First(&storedCode, "public_id = ?", approved.CodeID).Error)
	require.Equal(t, int64(300), storedCode.ExpiresAt-storedCode.CreatedTime)

	var tokens []model.Token
	require.NoError(t, model.DB.Find(&tokens).Error)
	require.Len(t, tokens, 1)
	require.Equal(t, model.TokenSourceMcpOAuth, tokens[0].Source)
	require.Equal(t, 123456, tokens[0].RemainQuota)
	require.False(t, tokens[0].UnlimitedQuota)
	require.Contains(t, tokens[0].Key, "mcp-token")

	_, err = lifecycle.ApproveMcpOAuthAuthorization(McpOAuthAuthorizationApprovalRequest{
		UserID:              user.Id,
		Client:              client,
		Resource:            McpOAuthResource,
		RedirectURI:         "https://client.example/callback",
		Scope:               "tools:search tools:read",
		CodeChallenge:       "pkce-challenge",
		CodeChallengeMethod: "S256",
	})
	require.ErrorIs(t, err, ErrMcpOAuthAuthorizationAlreadyProcessed)

	approvedSecondPKCE, err := lifecycle.ApproveMcpOAuthAuthorization(McpOAuthAuthorizationApprovalRequest{
		UserID:              user.Id,
		Client:              client,
		Resource:            McpOAuthResource,
		RedirectURI:         "https://client.example/callback",
		Scope:               "tools:read tools:search",
		CodeChallenge:       "pkce-challenge-next",
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	require.NotEqual(t, approved.Code, approvedSecondPKCE.Code)
}

func TestMcpOAuthLifecycleExchangeValidatesPKCEAndReturnsNarrowTokenShape(t *testing.T) {
	setupMcpOAuthLifecycleTestDB(t)
	user := model.User{Username: "exchange-user", Password: "password", AffCode: "exchange-user-aff"}
	require.NoError(t, model.DB.Create(&user).Error)
	lifecycle := testMcpOAuthLifecycle(t)
	verifier := strings.Repeat("a", 50)
	approved, err := lifecycle.ApproveMcpOAuthAuthorization(McpOAuthAuthorizationApprovalRequest{
		UserID:              user.Id,
		Client:              testMcpOAuthLifecycleClient(),
		Resource:            McpOAuthResource,
		RedirectURI:         "https://client.example/callback",
		Scope:               "tools:read tools:execute",
		CodeChallenge:       McpOAuthS256Challenge(verifier),
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)

	_, err = lifecycle.ExchangeMcpOAuthAuthorizationCode(McpOAuthAuthorizationCodeExchangeRequest{
		Code:         approved.Code,
		ClientID:     testMcpOAuthLifecycleClient().ClientID,
		Resource:     McpOAuthResource,
		RedirectURI:  "https://client.example/callback",
		CodeVerifier: strings.Repeat("b", 50),
	})
	require.Error(t, err)
	var storedCode model.McpOAuthAuthorizationCode
	require.NoError(t, model.DB.First(&storedCode, "public_id = ?", approved.CodeID).Error)
	require.Zero(t, storedCode.ConsumedAt)

	tokenResponse, err := lifecycle.ExchangeMcpOAuthAuthorizationCode(McpOAuthAuthorizationCodeExchangeRequest{
		Code:         approved.Code,
		ClientID:     testMcpOAuthLifecycleClient().ClientID,
		Resource:     McpOAuthResource,
		RedirectURI:  "https://client.example/callback",
		CodeVerifier: verifier,
	})
	require.NoError(t, err)
	require.NotEmpty(t, tokenResponse.AccessToken)
	require.NotEmpty(t, tokenResponse.RefreshToken)
	require.Equal(t, "Bearer", tokenResponse.TokenType)
	require.Equal(t, 900, tokenResponse.ExpiresIn)
	require.Equal(t, "tools:read tools:execute", tokenResponse.Scope)
	raw := testMcpOAuthJSONString(t, tokenResponse)
	require.Contains(t, raw, "access_token")
	require.Contains(t, raw, "refresh_token")
	require.NotContains(t, raw, "grant")
	require.NotContains(t, raw, "client_secret")
}

func TestMcpOAuthLifecycleRefreshAllowsScopeSubsetAndReplayRevokesRefreshFamily(t *testing.T) {
	setupMcpOAuthLifecycleTestDB(t)
	user := model.User{Username: "refresh-user", Password: "password", AffCode: "refresh-user-aff"}
	require.NoError(t, model.DB.Create(&user).Error)
	lifecycle := testMcpOAuthLifecycle(t)
	verifier := strings.Repeat("c", 50)
	approved, err := lifecycle.ApproveMcpOAuthAuthorization(McpOAuthAuthorizationApprovalRequest{
		UserID:              user.Id,
		Client:              testMcpOAuthLifecycleClient(),
		Resource:            McpOAuthResource,
		RedirectURI:         "https://client.example/callback",
		Scope:               "tools:read tools:execute",
		CodeChallenge:       McpOAuthS256Challenge(verifier),
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	initial, err := lifecycle.ExchangeMcpOAuthAuthorizationCode(McpOAuthAuthorizationCodeExchangeRequest{
		Code:         approved.Code,
		ClientID:     testMcpOAuthLifecycleClient().ClientID,
		Resource:     McpOAuthResource,
		RedirectURI:  "https://client.example/callback",
		CodeVerifier: verifier,
	})
	require.NoError(t, err)

	refreshed, err := lifecycle.RefreshMcpOAuthAccessToken(McpOAuthRefreshRequest{
		RefreshToken: initial.RefreshToken,
		ClientID:     testMcpOAuthLifecycleClient().ClientID,
		Resource:     McpOAuthResource,
		Scope:        "tools:read",
	})
	require.NoError(t, err)
	require.Equal(t, "tools:read", refreshed.Scope)
	require.NotEqual(t, initial.RefreshToken, refreshed.RefreshToken)

	_, err = lifecycle.RefreshMcpOAuthAccessToken(McpOAuthRefreshRequest{
		RefreshToken: initial.RefreshToken,
		ClientID:     testMcpOAuthLifecycleClient().ClientID,
		Resource:     McpOAuthResource,
	})
	require.ErrorIs(t, err, model.ErrMcpOAuthRefreshReplay)
	var activeCount int64
	require.NoError(t, model.DB.Model(&model.McpOAuthRefreshToken{}).Where("status = ?", model.McpOAuthRefreshTokenStatusActive).Count(&activeCount).Error)
	require.Zero(t, activeCount)
}

func TestMcpOAuthLifecycleRevokeConnectedAppsAndIdentityFailClosed(t *testing.T) {
	setupMcpOAuthLifecycleTestDB(t)
	user := model.User{Username: "identity-user", Password: "password", AffCode: "identity-user-aff"}
	require.NoError(t, model.DB.Create(&user).Error)
	lifecycle := testMcpOAuthLifecycle(t)
	verifier := strings.Repeat("d", 50)
	approved, err := lifecycle.ApproveMcpOAuthAuthorization(McpOAuthAuthorizationApprovalRequest{
		UserID:              user.Id,
		Client:              testMcpOAuthLifecycleClient(),
		Resource:            McpOAuthResource,
		RedirectURI:         "https://client.example/callback",
		Scope:               "tools:read",
		CodeChallenge:       McpOAuthS256Challenge(verifier),
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	tokenResponse, err := lifecycle.ExchangeMcpOAuthAuthorizationCode(McpOAuthAuthorizationCodeExchangeRequest{
		Code:         approved.Code,
		ClientID:     testMcpOAuthLifecycleClient().ClientID,
		Resource:     McpOAuthResource,
		RedirectURI:  "https://client.example/callback",
		CodeVerifier: verifier,
	})
	require.NoError(t, err)
	claims, err := lifecycle.signer.VerifyAccessToken(tokenResponse.AccessToken, "tools:read")
	require.NoError(t, err)

	identity, err := lifecycle.ResolveMcpOAuthDataToolIdentity(claims)
	require.NoError(t, err)
	require.NotEmpty(t, identity.Token.Key)
	require.NotContains(t, testMcpOAuthJSONString(t, identity), identity.Token.Key)

	apps, err := lifecycle.ListMcpOAuthConnectedApps(user.Id)
	require.NoError(t, err)
	require.Len(t, apps, 1)
	require.Equal(t, model.McpOAuthGrantStatusActive, apps[0].Status)

	require.NoError(t, lifecycle.RevokeMcpOAuthCredential(McpOAuthRevokeRequest{
		Token:    tokenResponse.AccessToken,
		ClientID: "wrong-client",
	}))
	_, err = lifecycle.ResolveMcpOAuthDataToolIdentity(claims)
	require.NoError(t, err)

	require.NoError(t, lifecycle.RevokeMcpOAuthCredential(McpOAuthRevokeRequest{
		Token:    tokenResponse.AccessToken,
		ClientID: testMcpOAuthLifecycleClient().ClientID,
	}))
	_, err = lifecycle.ResolveMcpOAuthDataToolIdentity(claims)
	require.ErrorIs(t, err, model.ErrMcpOAuthGrantRevoked)

	require.NoError(t, lifecycle.RevokeMcpOAuthCredential(McpOAuthRevokeRequest{Token: "unknown"}))
}

func testMcpOAuthLifecycle(t *testing.T) *McpOAuthLifecycle {
	t.Helper()
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	privateKey := testMcpOAuthPrivateKey(t)
	signer, err := NewMcpOAuthSigner(privateKey, McpOAuthSigningConfig{
		Clock:      func() time.Time { return now },
		Randomness: bytes.NewReader(bytes.Repeat([]byte{0x31}, 1024)),
	})
	require.NoError(t, err)
	counter := 0
	lifecycle, err := NewMcpOAuthLifecycle(McpOAuthLifecycleConfig{
		Signer: signer,
		Clock:  func() time.Time { return now },
		RandomString: func(length int) (string, error) {
			if length == 0 {
				return "", errors.New("bad length")
			}
			counter++
			suffix := fmt.Sprintf("%032d", counter)
			if length <= len(suffix) {
				return suffix[len(suffix)-length:], nil
			}
			return strings.Repeat("x", length-len(suffix)) + suffix, nil
		},
	})
	require.NoError(t, err)
	return lifecycle
}

func testMcpOAuthLifecycleClient() McpOAuthClientMetadata {
	return McpOAuthClientMetadata{
		ClientID:     "https://client.example/oauth/client",
		ClientName:   "Lifecycle Client",
		RedirectURIs: []string{"https://client.example/callback"},
	}
}

func testMcpOAuthJSONString(t *testing.T, v any) string {
	t.Helper()
	raw, err := common.Marshal(v)
	require.NoError(t, err)
	return string(bytes.TrimSpace(raw))
}
