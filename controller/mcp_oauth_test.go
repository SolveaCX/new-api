package controller

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMcpOAuthControllerTestDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/mcp-oauth-controller.db"), &gorm.Config{})
	require.NoError(t, err)
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
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
		model.DB = previousDB
	})
}

func TestMcpOAuthMetadataAndJWKSUseStandardShapes(t *testing.T) {
	setupMcpOAuthControllerTestDB(t)
	setupMcpOAuthControllerSignerEnv(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/.well-known/oauth-authorization-server", McpOAuthAuthorizationServerMetadata)
	router.GET("/oauth/jwks", McpOAuthJWKS)

	metadataRecorder := httptest.NewRecorder()
	router.ServeHTTP(metadataRecorder, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil))

	require.Equal(t, http.StatusOK, metadataRecorder.Code)
	var metadata map[string]any
	require.NoError(t, common.Unmarshal(metadataRecorder.Body.Bytes(), &metadata))
	require.Equal(t, "https://console.flatkey.ai", metadata["issuer"])
	require.Equal(t, "https://console.flatkey.ai/oauth/register", metadata["registration_endpoint"])
	require.Equal(t, true, metadata["client_id_metadata_document_supported"])
	require.ElementsMatch(t, []any{"none"}, metadata["revocation_endpoint_auth_methods_supported"].([]any))
	require.NotContains(t, metadata, "success")

	jwksRecorder := httptest.NewRecorder()
	router.ServeHTTP(jwksRecorder, httptest.NewRequest(http.MethodGet, "/oauth/jwks", nil))

	require.Equal(t, http.StatusOK, jwksRecorder.Code)
	var jwks map[string]any
	require.NoError(t, common.Unmarshal(jwksRecorder.Body.Bytes(), &jwks))
	require.NotEmpty(t, jwks["keys"])
	require.NotContains(t, jwks, "success")
}

func TestMcpOAuthDynamicClientRegistrationContract(t *testing.T) {
	setupMcpOAuthControllerTestDB(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/oauth/register", McpOAuthRegisterClient)

	valid := `{"client_name":"Test MCP Client","redirect_uris":["https://client.example/callback"],"grant_types":["authorization_code","refresh_token"],"response_types":["code"],"token_endpoint_auth_method":"none"}`
	validRecorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(valid))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(validRecorder, req)

	require.Equal(t, http.StatusCreated, validRecorder.Code)
	var registered map[string]any
	require.NoError(t, common.Unmarshal(validRecorder.Body.Bytes(), &registered))
	require.NotEmpty(t, registered["client_id"])
	require.Equal(t, "none", registered["token_endpoint_auth_method"])
	require.NotContains(t, registered, "success")

	invalidRecorder := httptest.NewRecorder()
	invalidReq := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(`{"client_name":"Bad","redirect_uris":["https://client.example/callback"],"token_endpoint_auth_method":"client_secret_basic"}`))
	invalidReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(invalidRecorder, invalidReq)

	require.Equal(t, http.StatusBadRequest, invalidRecorder.Code)
	assertMcpOAuthError(t, invalidRecorder.Body.Bytes(), "invalid_client_metadata")
}

func TestMcpOAuthRegisterRuntimeFailureUsesSafeServerError(t *testing.T) {
	setupMcpOAuthControllerTestDB(t)
	require.NoError(t, model.DB.Migrator().DropTable(&model.McpOAuthClient{}))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/oauth/register", McpOAuthRegisterClient)

	req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(`{"client_name":"Test MCP Client","redirect_uris":["https://client.example/callback"],"grant_types":["authorization_code"],"response_types":["code"],"token_endpoint_auth_method":"none"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	payload := assertMcpOAuthError(t, recorder.Body.Bytes(), "server_error")
	description := strings.ToLower(payload["error_description"].(string))
	require.NotContains(t, description, "database")
	require.NotContains(t, description, "table")
	require.NotContains(t, description, "constraint")
	require.NotContains(t, description, "no such")
	require.NotContains(t, description, "mcp_oauth_clients")
}

func TestMcpOAuthTokenAndRevokeRequireFormContentTypeAndStandardErrors(t *testing.T) {
	setupMcpOAuthControllerTestDB(t)
	setupMcpOAuthControllerSignerEnv(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/oauth/token", McpOAuthToken)
	router.POST("/oauth/revoke", McpOAuthRevoke)

	tokenRecorder := httptest.NewRecorder()
	tokenReq := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(`{"grant_type":"authorization_code"}`))
	tokenReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(tokenRecorder, tokenReq)

	require.Equal(t, http.StatusBadRequest, tokenRecorder.Code)
	assertMcpOAuthError(t, tokenRecorder.Body.Bytes(), "invalid_request")

	revokeRecorder := httptest.NewRecorder()
	revokeReq := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(`token=missing`))
	revokeReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(revokeRecorder, revokeReq)

	require.Equal(t, http.StatusBadRequest, revokeRecorder.Code)
	assertMcpOAuthError(t, revokeRecorder.Body.Bytes(), "invalid_request")

	unknownRecorder := httptest.NewRecorder()
	unknownReq := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(`token=missing`))
	unknownReq.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	router.ServeHTTP(unknownRecorder, unknownReq)

	require.Equal(t, http.StatusOK, unknownRecorder.Code)
	require.Empty(t, strings.TrimSpace(unknownRecorder.Body.String()))
}

func TestMcpOAuthTokenMapsOAuthFailuresToSpecificErrorCodes(t *testing.T) {
	setupMcpOAuthControllerTestDB(t)
	setupMcpOAuthControllerSignerEnv(t)
	user := model.User{Username: "mcp-token-user", Password: "password", AffCode: "mcp-token-user-aff"}
	require.NoError(t, model.DB.Create(&user).Error)
	client := seedMcpOAuthControllerClient(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/oauth/token", McpOAuthToken)

	unknownClient := postMcpOAuthTokenForm(t, router, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"unknown-code"},
		"client_id":     {"mcp_client_missing"},
		"resource":      {service.McpOAuthResource},
		"redirect_uri":  {"https://client.example/callback"},
		"code_verifier": {strings.Repeat("u", 50)},
	})
	require.Equal(t, http.StatusBadRequest, unknownClient.Code)
	assertMcpOAuthError(t, unknownClient.Body.Bytes(), "invalid_client")

	unknownCode := postMcpOAuthTokenForm(t, router, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"unknown-code"},
		"client_id":     {client.PublicID},
		"resource":      {service.McpOAuthResource},
		"redirect_uri":  {"https://client.example/callback"},
		"code_verifier": {strings.Repeat("u", 50)},
	})
	require.Equal(t, http.StatusBadRequest, unknownCode.Code)
	assertMcpOAuthError(t, unknownCode.Body.Bytes(), "invalid_grant")

	pkceVerifier := strings.Repeat("v", 50)
	codeForPKCE := approveMcpOAuthControllerCode(t, user.Id, client, "tools:read", service.McpOAuthS256Challenge(pkceVerifier))
	pkceMismatch := postMcpOAuthTokenForm(t, router, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {codeForPKCE},
		"client_id":     {client.PublicID},
		"resource":      {service.McpOAuthResource},
		"redirect_uri":  {"https://client.example/callback"},
		"code_verifier": {strings.Repeat("w", 50)},
	})
	require.Equal(t, http.StatusBadRequest, pkceMismatch.Code)
	assertMcpOAuthError(t, pkceMismatch.Body.Bytes(), "invalid_grant")

	expiryVerifier := strings.Repeat("x", 50)
	codeForExpiry := approveMcpOAuthControllerCode(t, user.Id, client, "tools:read", service.McpOAuthS256Challenge(expiryVerifier))
	require.NoError(t, model.DB.Model(&model.McpOAuthAuthorizationCode{}).Where("code_hash = ?", model.HashMcpOAuthCredential(codeForExpiry)).Update("expires_at", int64(1)).Error)
	expiredCode := postMcpOAuthTokenForm(t, router, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {codeForExpiry},
		"client_id":     {client.PublicID},
		"resource":      {service.McpOAuthResource},
		"redirect_uri":  {"https://client.example/callback"},
		"code_verifier": {expiryVerifier},
	})
	require.Equal(t, http.StatusBadRequest, expiredCode.Code)
	assertMcpOAuthError(t, expiredCode.Body.Bytes(), "invalid_grant")

	refreshVerifier := strings.Repeat("y", 50)
	codeForRefresh := approveMcpOAuthControllerCode(t, user.Id, client, "tools:read", service.McpOAuthS256Challenge(refreshVerifier))
	initial := postMcpOAuthTokenForm(t, router, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {codeForRefresh},
		"client_id":     {client.PublicID},
		"resource":      {service.McpOAuthResource},
		"redirect_uri":  {"https://client.example/callback"},
		"code_verifier": {refreshVerifier},
	})
	require.Equal(t, http.StatusOK, initial.Code)
	var tokenPayload map[string]any
	require.NoError(t, common.Unmarshal(initial.Body.Bytes(), &tokenPayload))
	refreshScope := postMcpOAuthTokenForm(t, router, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tokenPayload["refresh_token"].(string)},
		"client_id":     {client.PublicID},
		"resource":      {service.McpOAuthResource},
		"scope":         {"tools:execute"},
	})
	require.Equal(t, http.StatusBadRequest, refreshScope.Code)
	assertMcpOAuthError(t, refreshScope.Body.Bytes(), "invalid_scope")
}

func TestMcpOAuthAuthorizationDetailsAndAuthorizeGenerateRedirectsForCurrentUser(t *testing.T) {
	setupMcpOAuthControllerTestDB(t)
	setupMcpOAuthControllerSignerEnv(t)
	user := model.User{Username: "mcp-ui-user", Password: "password", AffCode: "mcp-ui-user-aff"}
	require.NoError(t, model.DB.Create(&user).Error)
	client := seedMcpOAuthControllerClient(t)
	verifier := strings.Repeat("a", 50)
	challenge := service.McpOAuthS256Challenge(verifier)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/oauth/authorization-details", func(c *gin.Context) {
		c.Set("id", user.Id)
		McpOAuthAuthorizationDetails(c)
	})
	router.POST("/api/oauth/authorize", func(c *gin.Context) {
		c.Set("id", user.Id)
		McpOAuthAuthorize(c)
	})

	detailsPath := "/api/oauth/authorization-details?client_id=" + url.QueryEscape(client.PublicID) +
		"&resource=https%3A%2F%2Fmcp.flatkey.ai&redirect_uri=https%3A%2F%2Fclient.example%2Fcallback&scope=tools%3Aread&response_type=code&code_challenge=" + challenge + "&code_challenge_method=S256&state=abc123"
	detailsRecorder := httptest.NewRecorder()
	router.ServeHTTP(detailsRecorder, httptest.NewRequest(http.MethodGet, detailsPath, nil))

	require.Equal(t, http.StatusOK, detailsRecorder.Code)
	var details apiResponse
	require.NoError(t, common.Unmarshal(detailsRecorder.Body.Bytes(), &details))
	require.True(t, details.Success)
	require.Equal(t, "abc123", details.Data.(map[string]any)["state"])
	require.Equal(t, []any{"tools:read"}, details.Data.(map[string]any)["scopes"])

	badResourceRecorder := httptest.NewRecorder()
	router.ServeHTTP(badResourceRecorder, httptest.NewRequest(http.MethodGet, strings.Replace(detailsPath, "https%3A%2F%2Fmcp.flatkey.ai", "https%3A%2F%2Fwrong.example", 1), nil))
	require.Equal(t, http.StatusOK, badResourceRecorder.Code)
	require.Contains(t, badResourceRecorder.Body.String(), "resource")

	approveRecorder := httptest.NewRecorder()
	approveReq := httptest.NewRequest(http.MethodPost, "/api/oauth/authorize", strings.NewReader(`{"action":"approve","client_id":"`+client.PublicID+`","resource":"https://mcp.flatkey.ai","redirect_uri":"https://client.example/callback","scope":"tools:read","response_type":"code","code_challenge":"`+challenge+`","code_challenge_method":"S256","state":"abc123"}`))
	approveReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(approveRecorder, approveReq)

	require.Equal(t, http.StatusOK, approveRecorder.Code)
	var approved apiResponse
	require.NoError(t, common.Unmarshal(approveRecorder.Body.Bytes(), &approved))
	redirectURL := approved.Data.(map[string]any)["redirect_url"].(string)
	require.Contains(t, redirectURL, "https://client.example/callback?")
	require.Contains(t, redirectURL, "code=")
	require.Contains(t, redirectURL, "state=abc123")
	parsedRedirect, err := url.Parse(redirectURL)
	require.NoError(t, err)
	code := parsedRedirect.Query().Get("code")
	require.NotEmpty(t, code)

	denyRecorder := httptest.NewRecorder()
	denyReq := httptest.NewRequest(http.MethodPost, "/api/oauth/authorize", strings.NewReader(`{"action":"deny","client_id":"`+client.PublicID+`","resource":"https://mcp.flatkey.ai","redirect_uri":"https://client.example/callback","scope":"tools:read","response_type":"code","code_challenge":"`+challenge+`","code_challenge_method":"S256","state":"denied-state"}`))
	denyReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(denyRecorder, denyReq)

	var denied apiResponse
	require.NoError(t, common.Unmarshal(denyRecorder.Body.Bytes(), &denied))
	denyURL := denied.Data.(map[string]any)["redirect_url"].(string)
	require.Contains(t, denyURL, "error=access_denied")
	require.Contains(t, denyURL, "state=denied-state")

	router.POST("/oauth/token", McpOAuthToken)
	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {client.PublicID},
		"resource":      {service.McpOAuthResource},
		"redirect_uri":  {"https://client.example/callback"},
		"code_verifier": {verifier},
	}
	tokenRecorder := httptest.NewRecorder()
	tokenReq := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(tokenRecorder, tokenReq)

	require.Equal(t, http.StatusOK, tokenRecorder.Code)
	var tokenPayload map[string]any
	require.NoError(t, common.Unmarshal(tokenRecorder.Body.Bytes(), &tokenPayload))
	require.NotEmpty(t, tokenPayload["access_token"])
	require.NotEmpty(t, tokenPayload["refresh_token"])
	require.Equal(t, "Bearer", tokenPayload["token_type"])
	require.NotContains(t, tokenPayload, "success")
}

func TestMcpOAuthConnectedAppsAreScopedToCurrentUser(t *testing.T) {
	setupMcpOAuthControllerTestDB(t)
	setupMcpOAuthControllerSignerEnv(t)
	user := model.User{Username: "mcp-owner", Password: "password", AffCode: "mcp-owner-aff"}
	other := model.User{Username: "mcp-other", Password: "password", AffCode: "mcp-other-aff"}
	require.NoError(t, model.DB.Create(&user).Error)
	require.NoError(t, model.DB.Create(&other).Error)
	grant := model.McpOAuthGrant{PublicID: "grant_owner", ClientID: "client-owner", UserID: user.Id, AccountID: user.Id, Resource: service.McpOAuthResource, DisplayName: "Owner App", Status: model.McpOAuthGrantStatusActive, Scope: "tools:read", CreatedTime: 100, UpdatedTime: 100}
	otherGrant := model.McpOAuthGrant{PublicID: "grant_other", ClientID: "client-other", UserID: other.Id, AccountID: other.Id, Resource: service.McpOAuthResource, DisplayName: "Other App", Status: model.McpOAuthGrantStatusActive, Scope: "tools:read", CreatedTime: 100, UpdatedTime: 100}
	require.NoError(t, model.DB.Create(&grant).Error)
	require.NoError(t, model.DB.Create(&otherGrant).Error)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/user/connected-apps", func(c *gin.Context) {
		c.Set("id", user.Id)
		McpOAuthConnectedApps(c)
	})
	router.POST("/api/user/connected-apps/:grant_id/revoke", func(c *gin.Context) {
		c.Set("id", user.Id)
		McpOAuthRevokeConnectedApp(c)
	})

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/api/user/connected-apps", nil))
	var listed apiResponse
	require.NoError(t, common.Unmarshal(listRecorder.Body.Bytes(), &listed))
	require.True(t, listed.Success)
	require.Len(t, listed.Data.([]any), 1)
	require.Equal(t, "grant_owner", listed.Data.([]any)[0].(map[string]any)["grant_public_id"])

	otherRevoke := httptest.NewRecorder()
	router.ServeHTTP(otherRevoke, httptest.NewRequest(http.MethodPost, "/api/user/connected-apps/grant_other/revoke", nil))
	require.Equal(t, http.StatusNotFound, otherRevoke.Code)

	ownerRevoke := httptest.NewRecorder()
	router.ServeHTTP(ownerRevoke, httptest.NewRequest(http.MethodPost, "/api/user/connected-apps/grant_owner/revoke", nil))
	require.Equal(t, http.StatusOK, ownerRevoke.Code)
}

type apiResponse struct {
	Success bool `json:"success"`
	Data    any  `json:"data"`
}

func assertMcpOAuthError(t *testing.T, body []byte, code string) map[string]any {
	t.Helper()
	var payload map[string]any
	require.NoError(t, common.Unmarshal(bytes.TrimSpace(body), &payload))
	require.Equal(t, code, payload["error"])
	require.NotEmpty(t, payload["error_description"])
	require.NotContains(t, payload, "success")
	require.NotContains(t, payload, "message")
	return payload
}

func seedMcpOAuthControllerClient(t *testing.T) model.McpOAuthClient {
	t.Helper()
	client := model.McpOAuthClient{
		PublicID:     "mcp_client_controller",
		Name:         "Controller Client",
		RedirectURIs: `["https://client.example/callback"]`,
		CreatedTime:  100,
		UpdatedTime:  100,
	}
	require.NoError(t, model.DB.Create(&client).Error)
	return client
}

func setupMcpOAuthControllerSignerEnv(t *testing.T) {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	t.Setenv("FLATKEY_MCP_OAUTH_ED25519_PRIVATE_KEY", base64.StdEncoding.EncodeToString(der))
}

func approveMcpOAuthControllerCode(t *testing.T, userID int, client model.McpOAuthClient, scope string, challenge string) string {
	t.Helper()
	signer, err := service.NewMcpOAuthSignerFromEnv(service.McpOAuthSigningConfig{})
	require.NoError(t, err)
	lifecycle, err := service.NewMcpOAuthLifecycle(service.McpOAuthLifecycleConfig{Signer: signer})
	require.NoError(t, err)
	approved, err := lifecycle.ApproveMcpOAuthAuthorization(service.McpOAuthAuthorizationApprovalRequest{
		UserID: userID,
		Client: service.McpOAuthClientMetadata{
			ClientID:     client.PublicID,
			ClientName:   client.Name,
			RedirectURIs: []string{"https://client.example/callback"},
		},
		Resource:            service.McpOAuthResource,
		RedirectURI:         "https://client.example/callback",
		Scope:               scope,
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	return approved.Code
}

func postMcpOAuthTokenForm(t *testing.T, router http.Handler, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(recorder, req)
	return recorder
}
