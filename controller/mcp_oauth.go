package controller

import (
	"errors"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type mcpOAuthAuthorizeRequest struct {
	Action              string `json:"action"`
	ClientID            string `json:"client_id"`
	Resource            string `json:"resource"`
	RedirectURI         string `json:"redirect_uri"`
	Scope               string `json:"scope"`
	ResponseType        string `json:"response_type"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	State               string `json:"state"`
}

func McpOAuthAuthorizationServerMetadata(c *gin.Context) {
	c.JSON(http.StatusOK, service.McpOAuthAuthorizationServerMetadata())
}

func McpOAuthJWKS(c *gin.Context) {
	lifecycle, err := mcpOAuthLifecycle()
	if err != nil {
		mcpOAuthError(c, http.StatusInternalServerError, "server_error", "OAuth signing key is unavailable")
		return
	}
	c.JSON(http.StatusOK, lifecycle.JWKS())
}

func McpOAuthRegisterClient(c *gin.Context) {
	if !mcpOAuthHasJSONContentType(c.GetHeader("Content-Type")) {
		mcpOAuthError(c, http.StatusBadRequest, "invalid_request", "Content-Type must be application/json")
		return
	}
	var req service.McpOAuthDCRRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		mcpOAuthError(c, http.StatusBadRequest, "invalid_client_metadata", "client metadata JSON is invalid")
		return
	}
	client, validated, err := service.RegisterMcpOAuthDCRClient(req, nil, time.Now().UTC().Unix())
	if err != nil {
		mcpOAuthServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"client_id":                  client.ClientID,
		"client_name":                client.ClientName,
		"redirect_uris":              client.RedirectURIs,
		"grant_types":                validated.GrantTypes,
		"response_types":             validated.ResponseTypes,
		"token_endpoint_auth_method": validated.TokenEndpointAuthMethod,
	})
}

func McpOAuthToken(c *gin.Context) {
	if !mcpOAuthHasFormContentType(c.GetHeader("Content-Type")) {
		mcpOAuthError(c, http.StatusBadRequest, "invalid_request", "Content-Type must be application/x-www-form-urlencoded")
		return
	}
	if err := c.Request.ParseForm(); err != nil {
		mcpOAuthError(c, http.StatusBadRequest, "invalid_request", "form body is invalid")
		return
	}
	lifecycle, err := mcpOAuthLifecycle()
	if err != nil {
		mcpOAuthError(c, http.StatusInternalServerError, "server_error", "OAuth signing key is unavailable")
		return
	}
	switch c.PostForm("grant_type") {
	case "authorization_code":
		token, err := lifecycle.ExchangeMcpOAuthAuthorizationCode(service.McpOAuthAuthorizationCodeExchangeRequest{
			Code:         c.PostForm("code"),
			ClientID:     c.PostForm("client_id"),
			Resource:     c.PostForm("resource"),
			RedirectURI:  c.PostForm("redirect_uri"),
			CodeVerifier: c.PostForm("code_verifier"),
		})
		if err != nil {
			mcpOAuthServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, token)
	case "refresh_token":
		token, err := lifecycle.RefreshMcpOAuthAccessToken(service.McpOAuthRefreshRequest{
			RefreshToken: c.PostForm("refresh_token"),
			ClientID:     c.PostForm("client_id"),
			Resource:     c.PostForm("resource"),
			Scope:        c.PostForm("scope"),
		})
		if err != nil {
			mcpOAuthServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, token)
	default:
		mcpOAuthError(c, http.StatusBadRequest, "unsupported_grant_type", "grant_type must be authorization_code or refresh_token")
	}
}

func McpOAuthRevoke(c *gin.Context) {
	if !mcpOAuthHasFormContentType(c.GetHeader("Content-Type")) {
		mcpOAuthError(c, http.StatusBadRequest, "invalid_request", "Content-Type must be application/x-www-form-urlencoded")
		return
	}
	if err := c.Request.ParseForm(); err != nil {
		mcpOAuthError(c, http.StatusBadRequest, "invalid_request", "form body is invalid")
		return
	}
	lifecycle, err := mcpOAuthLifecycle()
	if err != nil {
		mcpOAuthError(c, http.StatusInternalServerError, "server_error", "OAuth signing key is unavailable")
		return
	}
	if err := lifecycle.RevokeMcpOAuthCredential(service.McpOAuthRevokeRequest{
		Token:    c.PostForm("token"),
		ClientID: c.PostForm("client_id"),
	}); err != nil {
		mcpOAuthError(c, http.StatusInternalServerError, "server_error", "revocation failed")
		return
	}
	c.Status(http.StatusOK)
}

func McpOAuthAuthorizationDetails(c *gin.Context) {
	req := mcpOAuthAuthorizationRequestFromQuery(c)
	client, scopes, err := service.ValidateMcpOAuthAuthorizationRequest(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"client_id":     client.ClientID,
		"client_name":   client.ClientName,
		"redirect_uri":  req.RedirectURI,
		"resource":      req.Resource,
		"scopes":        scopes,
		"response_type": req.ResponseType,
		"state":         c.Query("state"),
	})
}

func McpOAuthAuthorize(c *gin.Context) {
	var req mcpOAuthAuthorizeRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	authReq := service.McpOAuthAuthorizationRequest{
		ClientID:            req.ClientID,
		Resource:            req.Resource,
		RedirectURI:         req.RedirectURI,
		Scope:               req.Scope,
		ResponseType:        req.ResponseType,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
	}
	client, _, err := service.ValidateMcpOAuthAuthorizationRequest(authReq)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Action == "deny" {
		common.ApiSuccess(c, gin.H{"redirect_url": mcpOAuthRedirect(req.RedirectURI, map[string]string{
			"error": "access_denied",
			"state": req.State,
		})})
		return
	}
	if req.Action != "approve" {
		common.ApiError(c, errors.New("action must be approve or deny"))
		return
	}
	lifecycle, err := mcpOAuthLifecycle()
	if err != nil {
		common.ApiError(c, errors.New("OAuth signing key is unavailable"))
		return
	}
	approved, err := lifecycle.ApproveMcpOAuthAuthorization(service.McpOAuthAuthorizationApprovalRequest{
		UserID:              c.GetInt("id"),
		Client:              client,
		Resource:            req.Resource,
		RedirectURI:         req.RedirectURI,
		Scope:               req.Scope,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"redirect_url": mcpOAuthRedirect(req.RedirectURI, map[string]string{
		"code":  approved.Code,
		"state": req.State,
	})})
}

func McpOAuthConnectedApps(c *gin.Context) {
	lifecycle, err := mcpOAuthLifecycle()
	if err != nil {
		common.ApiError(c, errors.New("OAuth signing key is unavailable"))
		return
	}
	apps, err := lifecycle.ListMcpOAuthConnectedApps(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, apps)
}

func McpOAuthRevokeConnectedApp(c *gin.Context) {
	lifecycle, err := mcpOAuthLifecycle()
	if err != nil {
		common.ApiError(c, errors.New("OAuth signing key is unavailable"))
		return
	}
	revoked, err := lifecycle.RevokeMcpOAuthConnectedApp(c.GetInt("id"), c.Param("grant_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !revoked {
		c.Status(http.StatusNotFound)
		return
	}
	common.ApiSuccess(c, gin.H{"revoked": true})
}

func mcpOAuthAuthorizationRequestFromQuery(c *gin.Context) service.McpOAuthAuthorizationRequest {
	return service.McpOAuthAuthorizationRequest{
		ClientID:            c.Query("client_id"),
		Resource:            c.Query("resource"),
		RedirectURI:         c.Query("redirect_uri"),
		Scope:               c.Query("scope"),
		ResponseType:        c.Query("response_type"),
		CodeChallenge:       c.Query("code_challenge"),
		CodeChallengeMethod: c.Query("code_challenge_method"),
	}
}

func mcpOAuthLifecycle() (*service.McpOAuthLifecycle, error) {
	signer, err := service.NewMcpOAuthSignerFromEnv(service.McpOAuthSigningConfig{})
	if err != nil {
		return nil, err
	}
	return service.NewMcpOAuthLifecycle(service.McpOAuthLifecycleConfig{Signer: signer})
}

func mcpOAuthServiceError(c *gin.Context, err error) {
	var oauthErr *service.McpOAuthError
	if errors.As(err, &oauthErr) {
		status := http.StatusBadRequest
		if oauthErr.Code == "server_error" {
			status = http.StatusInternalServerError
		}
		mcpOAuthError(c, status, oauthErr.Code, oauthErr.Description)
		return
	}
	switch {
	case errors.Is(err, model.ErrMcpOAuthCredentialConsumed),
		errors.Is(err, model.ErrMcpOAuthCredentialExpired),
		errors.Is(err, model.ErrMcpOAuthCredentialMismatch),
		errors.Is(err, model.ErrMcpOAuthPKCEMismatch),
		errors.Is(err, model.ErrMcpOAuthGrantRevoked),
		errors.Is(err, model.ErrMcpOAuthGrantUnavailable),
		errors.Is(err, model.ErrMcpOAuthRefreshReplay):
		mcpOAuthError(c, http.StatusBadRequest, "invalid_grant", "grant is invalid")
	default:
		mcpOAuthError(c, http.StatusInternalServerError, "server_error", "request could not be completed")
	}
}

func mcpOAuthError(c *gin.Context, status int, code, description string) {
	c.JSON(status, gin.H{
		"error":             code,
		"error_description": description,
	})
}

func mcpOAuthRedirect(rawRedirectURI string, params map[string]string) string {
	u, err := url.Parse(rawRedirectURI)
	if err != nil {
		return rawRedirectURI
	}
	q := u.Query()
	for key, value := range params {
		if value != "" {
			q.Set(key, value)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func mcpOAuthHasFormContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && strings.EqualFold(mediaType, "application/x-www-form-urlencoded")
}

func mcpOAuthHasJSONContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	mediaType = strings.ToLower(mediaType)
	return mediaType == "application/json" || strings.HasPrefix(mediaType, "application/") && strings.HasSuffix(mediaType, "+json")
}
