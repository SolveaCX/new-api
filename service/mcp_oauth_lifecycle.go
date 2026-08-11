package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

var ErrMcpOAuthAuthorizationAlreadyProcessed = errors.New("mcp oauth authorization already processed")

const mcpOAuthRefreshTokenLifetime = 30 * 24 * time.Hour

type McpOAuthLifecycleConfig struct {
	Signer       *McpOAuthSigner
	Clock        func() time.Time
	RandomString func(length int) (string, error)
}

type McpOAuthLifecycle struct {
	signer       *McpOAuthSigner
	clock        func() time.Time
	randomString func(length int) (string, error)
}

type McpOAuthAuthorizationApprovalRequest struct {
	UserID              int
	Client              McpOAuthClientMetadata
	Resource            string
	RedirectURI         string
	Scope               string
	CodeChallenge       string
	CodeChallengeMethod string
}

type McpOAuthAuthorizationApprovalResponse struct {
	Code   string `json:"code"`
	CodeID string `json:"code_id"`
	Scope  string `json:"scope"`
}

type McpOAuthAuthorizationCodeExchangeRequest struct {
	Code         string
	ClientID     string
	Resource     string
	RedirectURI  string
	CodeVerifier string
}

type McpOAuthRefreshRequest struct {
	RefreshToken string
	ClientID     string
	Resource     string
	Scope        string
}

type McpOAuthRevokeRequest struct {
	Token    string
	ClientID string
}

type McpOAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

type McpOAuthDataToolIdentity struct {
	UserID        int         `json:"user_id"`
	GrantPublicID string      `json:"grant_public_id"`
	ClientID      string      `json:"client_id"`
	Resource      string      `json:"resource"`
	Scopes        string      `json:"scopes"`
	Token         model.Token `json:"-"`
}

func NewMcpOAuthLifecycle(cfg McpOAuthLifecycleConfig) (*McpOAuthLifecycle, error) {
	if cfg.Signer == nil {
		return nil, errors.New("mcp oauth signer is required")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = time.Now
	}
	randomString := cfg.RandomString
	if randomString == nil {
		randomString = common.GenerateRandomCharsKey
	}
	return &McpOAuthLifecycle{signer: cfg.Signer, clock: clock, randomString: randomString}, nil
}

func (l *McpOAuthLifecycle) ApproveMcpOAuthAuthorization(req McpOAuthAuthorizationApprovalRequest) (McpOAuthAuthorizationApprovalResponse, error) {
	if req.UserID <= 0 {
		return McpOAuthAuthorizationApprovalResponse{}, errors.New("user_id is required")
	}
	if err := ValidateMcpOAuthResource(req.Resource); err != nil {
		return McpOAuthAuthorizationApprovalResponse{}, err
	}
	if err := ValidateMcpOAuthRedirectURI(req.Client, req.RedirectURI); err != nil {
		return McpOAuthAuthorizationApprovalResponse{}, err
	}
	if req.CodeChallengeMethod != "S256" || strings.TrimSpace(req.CodeChallenge) == "" {
		return McpOAuthAuthorizationApprovalResponse{}, errors.New("PKCE code_challenge_method must be S256")
	}
	scopes, err := NormalizeMcpOAuthScopes(req.Scope)
	if err != nil {
		return McpOAuthAuthorizationApprovalResponse{}, err
	}
	scopeText := strings.Join(scopes, " ")
	grantID, err := l.randomID("mcpgrant")
	if err != nil {
		return McpOAuthAuthorizationApprovalResponse{}, err
	}
	codeID, err := l.randomID("mcpcode")
	if err != nil {
		return McpOAuthAuthorizationApprovalResponse{}, err
	}
	codeSecret, err := l.randomID("mcpauth")
	if err != nil {
		return McpOAuthAuthorizationApprovalResponse{}, err
	}
	tokenKey, err := l.randomID("mcp-token")
	if err != nil {
		return McpOAuthAuthorizationApprovalResponse{}, err
	}
	now := l.clock().UTC()
	_, _, err = model.CreateMcpOAuthApproval(model.McpOAuthApprovalCreateParams{
		GrantPublicID:       grantID,
		CodePublicID:        codeID,
		CodeSecretHash:      model.HashMcpOAuthCredential(codeSecret),
		ApprovalFingerprint: model.FingerprintMcpOAuthApproval(req.UserID, req.Client.ClientID, req.Resource, req.RedirectURI, scopeText, req.CodeChallenge),
		ClientID:            req.Client.ClientID,
		UserID:              req.UserID,
		AccountID:           req.UserID,
		Resource:            req.Resource,
		DisplayName:         req.Client.ClientName,
		Scope:               scopeText,
		RedirectURI:         req.RedirectURI,
		CodeChallenge:       req.CodeChallenge,
		ChallengeMethod:     req.CodeChallengeMethod,
		TokenKey:            tokenKey,
		TokenRemainQuota:    common.GetEnvOrDefault("FLATKEY_MCP_OAUTH_TOKEN_REMAIN_QUOTA", 500000),
		TokenUnlimitedQuota: common.GetEnvOrDefaultBool("FLATKEY_MCP_OAUTH_TOKEN_UNLIMITED_QUOTA", true),
		Now:                 now.Unix(),
		CodeExpiresAt:       now.Add(5 * time.Minute).Unix(),
	})
	if errors.Is(err, model.ErrMcpOAuthApprovalAlreadyProcessed) {
		return McpOAuthAuthorizationApprovalResponse{}, ErrMcpOAuthAuthorizationAlreadyProcessed
	}
	if err != nil {
		return McpOAuthAuthorizationApprovalResponse{}, err
	}
	return McpOAuthAuthorizationApprovalResponse{Code: codeSecret, CodeID: codeID, Scope: scopeText}, nil
}

func (l *McpOAuthLifecycle) ExchangeMcpOAuthAuthorizationCode(req McpOAuthAuthorizationCodeExchangeRequest) (McpOAuthTokenResponse, error) {
	if err := ValidateMcpOAuthResource(req.Resource); err != nil {
		return McpOAuthTokenResponse{}, err
	}
	codeHash := model.HashMcpOAuthCredential(req.Code)
	snapshot, err := model.GetMcpOAuthAuthorizationCodeExchangeSnapshot(codeHash)
	if err != nil {
		return McpOAuthTokenResponse{}, err
	}
	if snapshot.Grant.ClientID != req.ClientID || snapshot.Grant.Resource != req.Resource || snapshot.Code.RedirectURI != req.RedirectURI {
		return McpOAuthTokenResponse{}, model.ErrMcpOAuthCredentialMismatch
	}
	if err := ValidateMcpOAuthPKCE(req.CodeVerifier, snapshot.Code.CodeChallenge, snapshot.Code.ChallengeMethod); err != nil {
		return McpOAuthTokenResponse{}, err
	}
	scopes, err := NormalizeMcpOAuthScopes(snapshot.Code.Scope)
	if err != nil {
		return McpOAuthTokenResponse{}, err
	}
	accessToken, err := l.signAccess(snapshot.Grant, scopes)
	if err != nil {
		return McpOAuthTokenResponse{}, err
	}
	refreshSecret, refreshID, familyID, err := l.refreshCredentialParts()
	if err != nil {
		return McpOAuthTokenResponse{}, err
	}
	now := l.clock().UTC()
	_, _, err = model.ExchangeMcpOAuthAuthorizationCode(model.ExchangeMcpOAuthAuthorizationCodeParams{
		CodeHash:           codeHash,
		ClientID:           req.ClientID,
		Resource:           req.Resource,
		RedirectURI:        req.RedirectURI,
		CodeVerifier:       req.CodeVerifier,
		RefreshPublicID:    refreshID,
		RefreshTokenHash:   model.HashMcpOAuthCredential(refreshSecret),
		RefreshTokenFamily: familyID,
		Now:                now.Unix(),
		RefreshExpiresAt:   now.Add(mcpOAuthRefreshTokenLifetime).Unix(),
	})
	if err != nil {
		return McpOAuthTokenResponse{}, err
	}
	return tokenResponse(accessToken, refreshSecret, strings.Join(scopes, " ")), nil
}

func (l *McpOAuthLifecycle) RefreshMcpOAuthAccessToken(req McpOAuthRefreshRequest) (McpOAuthTokenResponse, error) {
	if err := ValidateMcpOAuthResource(req.Resource); err != nil {
		return McpOAuthTokenResponse{}, err
	}
	refreshHash := model.HashMcpOAuthCredential(req.RefreshToken)
	snapshot, err := model.GetMcpOAuthRefreshSnapshotByHash(refreshHash)
	if err != nil {
		return McpOAuthTokenResponse{}, err
	}
	if snapshot.Grant.ClientID != req.ClientID || snapshot.Grant.Resource != req.Resource {
		return McpOAuthTokenResponse{}, model.ErrMcpOAuthCredentialMismatch
	}
	scopes, err := refreshScopes(snapshot.Grant.Scope, req.Scope)
	if err != nil {
		return McpOAuthTokenResponse{}, err
	}
	accessToken, err := l.signAccess(snapshot.Grant, scopes)
	if err != nil {
		return McpOAuthTokenResponse{}, err
	}
	nextSecret, nextID, _, err := l.refreshCredentialParts()
	if err != nil {
		return McpOAuthTokenResponse{}, err
	}
	now := l.clock().UTC()
	_, _, err = model.RotateMcpOAuthRefreshToken(refreshHash, model.McpOAuthRefreshToken{
		PublicID:    nextID,
		FamilyID:    snapshot.Refresh.FamilyID,
		TokenHash:   model.HashMcpOAuthCredential(nextSecret),
		CreatedTime: now.Unix(),
		ExpiresAt:   now.Add(mcpOAuthRefreshTokenLifetime).Unix(),
	}, now.Unix())
	if err != nil {
		return McpOAuthTokenResponse{}, err
	}
	return tokenResponse(accessToken, nextSecret, strings.Join(scopes, " ")), nil
}

func (l *McpOAuthLifecycle) RevokeMcpOAuthCredential(req McpOAuthRevokeRequest) error {
	if req.Token == "" {
		return nil
	}
	now := l.clock().UTC().Unix()
	if claims, err := l.signer.VerifyAccessToken(req.Token, ""); err == nil {
		if req.ClientID == "" || req.ClientID == claims.ClientID {
			_, err := model.RevokeMcpOAuthGrantIfExists(claims.GrantID, now)
			return err
		}
		return nil
	}
	_, err := model.RevokeMcpOAuthGrantByCredential(model.McpOAuthCredentialRevokeParams{
		CredentialHash: model.HashMcpOAuthCredential(req.Token),
		ClientID:       req.ClientID,
		Now:            now,
	})
	return err
}

func (l *McpOAuthLifecycle) JWKS() McpOAuthJWKS {
	return l.signer.JWKS()
}

func (l *McpOAuthLifecycle) ListMcpOAuthConnectedApps(userID int) ([]model.McpOAuthConnectedApp, error) {
	return model.ListMcpOAuthConnectedApps(userID)
}

func (l *McpOAuthLifecycle) RevokeMcpOAuthConnectedApp(userID int, grantPublicID string) (bool, error) {
	return model.RevokeMcpOAuthConnectedApp(userID, grantPublicID, l.clock().UTC().Unix())
}

func (l *McpOAuthLifecycle) ResolveMcpOAuthDataToolIdentity(claims McpOAuthVerifiedAccessClaims) (*McpOAuthDataToolIdentity, error) {
	resolved, err := model.ResolveMcpOAuthDataToolIdentity(model.McpOAuthDataToolClaims{
		Subject:  claims.Subject,
		GrantID:  claims.GrantID,
		ClientID: claims.ClientID,
		Resource: claims.Resource,
	}, l.clock().UTC().Unix())
	if err != nil {
		return nil, err
	}
	return &McpOAuthDataToolIdentity{
		UserID:        resolved.UserID,
		GrantPublicID: resolved.GrantPublicID,
		ClientID:      resolved.ClientID,
		Resource:      resolved.Resource,
		Scopes:        resolved.Scopes,
		Token:         resolved.Token,
	}, nil
}

func (l *McpOAuthLifecycle) signAccess(grant model.McpOAuthGrant, scopes []string) (string, error) {
	return l.signer.SignAccessToken(McpOAuthAccessTokenRequest{
		Subject:  fmt.Sprintf("user-%d", grant.UserID),
		GrantID:  grant.PublicID,
		ClientID: grant.ClientID,
		Scopes:   scopes,
		Resource: grant.Resource,
	})
}

func (l *McpOAuthLifecycle) refreshCredentialParts() (secret string, publicID string, familyID string, err error) {
	secret, err = l.randomID("mcprefresh")
	if err != nil {
		return "", "", "", err
	}
	publicID, err = l.randomID("mcprefreshid")
	if err != nil {
		return "", "", "", err
	}
	familyID, err = l.randomID("mcpfamily")
	if err != nil {
		return "", "", "", err
	}
	return secret, publicID, familyID, nil
}

func (l *McpOAuthLifecycle) randomID(prefix string) (string, error) {
	suffix, err := l.randomString(32)
	if err != nil {
		return "", err
	}
	return prefix + "_" + suffix, nil
}

func tokenResponse(accessToken, refreshToken, scope string) McpOAuthTokenResponse {
	return McpOAuthTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(mcpOAuthAccessTokenLifetime.Seconds()),
		Scope:        scope,
	}
}

func refreshScopes(originalScope, requestedScope string) ([]string, error) {
	original, err := NormalizeMcpOAuthScopes(originalScope)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(requestedScope) == "" {
		return original, nil
	}
	requested, err := NormalizeMcpOAuthScopes(requestedScope)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, scope := range original {
		allowed[scope] = true
	}
	for _, scope := range requested {
		if !allowed[scope] {
			return nil, errors.New("requested scope exceeds original grant")
		}
	}
	return requested, nil
}
