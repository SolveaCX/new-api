package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/golang-jwt/jwt/v5"
)

const mcpOAuthAccessTokenLifetime = 15 * time.Minute

type McpOAuthSigningConfig struct {
	Clock      func() time.Time
	Randomness io.Reader
}

type McpOAuthSigner struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	kid        string
	clock      func() time.Time
	randomness io.Reader
}

type McpOAuthAccessTokenRequest struct {
	Subject  string
	GrantID  string
	ClientID string
	Scopes   []string
	Resource string
}

type McpOAuthVerifiedAccessClaims struct {
	Issuer    string           `json:"iss"`
	Audience  string           `json:"aud"`
	Subject   string           `json:"sub"`
	IssuedAt  *jwt.NumericDate `json:"iat,omitempty"`
	ExpiresAt *jwt.NumericDate `json:"exp,omitempty"`
	ID        string           `json:"jti"`
	GrantID   string           `json:"grant_id"`
	ClientID  string           `json:"client_id"`
	Scopes    []string         `json:"scope"`
	Resource  string           `json:"resource"`
}

type McpOAuthJWKS struct {
	Keys []McpOAuthJWK `json:"keys"`
}

type McpOAuthJWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	X   string `json:"x"`
}

func NewMcpOAuthSignerFromEnv(cfg McpOAuthSigningConfig) (*McpOAuthSigner, error) {
	raw := strings.TrimSpace(common.GetEnvOrDefaultString("FLATKEY_MCP_OAUTH_ED25519_PRIVATE_KEY", ""))
	if raw == "" {
		return nil, errors.New("FLATKEY_MCP_OAUTH_ED25519_PRIVATE_KEY is required")
	}
	der, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("decode ed25519 private key: %w", err)
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS#8 private key: %w", err)
	}
	privateKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("FLATKEY_MCP_OAUTH_ED25519_PRIVATE_KEY must contain an ed25519 private key")
	}
	return NewMcpOAuthSigner(privateKey, cfg)
}

func NewMcpOAuthSigner(privateKey ed25519.PrivateKey, cfg McpOAuthSigningConfig) (*McpOAuthSigner, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, errors.New("ed25519 private key is invalid")
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok || len(publicKey) != ed25519.PublicKeySize {
		return nil, errors.New("ed25519 public key is invalid")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = time.Now
	}
	randomness := cfg.Randomness
	if randomness == nil {
		randomness = rand.Reader
	}
	return &McpOAuthSigner{
		privateKey: privateKey,
		publicKey:  publicKey,
		kid:        deriveMcpOAuthKeyID(publicKey),
		clock:      clock,
		randomness: randomness,
	}, nil
}

func (s *McpOAuthSigner) KeyID() string {
	return s.kid
}

func (s *McpOAuthSigner) SignAccessToken(req McpOAuthAccessTokenRequest) (string, error) {
	if req.Resource != McpOAuthResource {
		return "", NewMcpOAuthError("invalid_target", "resource must match the MCP resource")
	}
	scopes, err := NormalizeMcpOAuthScopes(strings.Join(req.Scopes, " "))
	if err != nil {
		return "", err
	}
	jtiBytes := make([]byte, 16)
	if _, err := io.ReadFull(s.randomness, jtiBytes); err != nil {
		return "", fmt.Errorf("generate token id: %w", err)
	}
	now := s.clock().UTC()
	return s.signClaims(McpOAuthVerifiedAccessClaims{
		Issuer:    McpOAuthIssuer,
		Audience:  McpOAuthResource,
		Subject:   req.Subject,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(mcpOAuthAccessTokenLifetime)),
		ID:        base64.RawURLEncoding.EncodeToString(jtiBytes),
		GrantID:   req.GrantID,
		ClientID:  req.ClientID,
		Scopes:    scopes,
		Resource:  McpOAuthResource,
	})
}

func (s *McpOAuthSigner) VerifyAccessToken(rawToken string, requiredScope string) (McpOAuthVerifiedAccessClaims, error) {
	claims := McpOAuthVerifiedAccessClaims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(func() time.Time { return s.clock().UTC() }),
	)
	token, err := parser.ParseWithClaims(rawToken, &claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodEdDSA {
			return nil, errors.New("unexpected jwt alg")
		}
		return s.publicKey, nil
	})
	if err != nil {
		errText := err.Error()
		if strings.Contains(errText, "signing method") {
			return McpOAuthVerifiedAccessClaims{}, fmt.Errorf("invalid alg: %w", err)
		}
		if strings.Contains(errText, "before issued") {
			return McpOAuthVerifiedAccessClaims{}, fmt.Errorf("invalid iat: %w", err)
		}
		return McpOAuthVerifiedAccessClaims{}, err
	}
	if token == nil || !token.Valid {
		return McpOAuthVerifiedAccessClaims{}, errors.New("invalid token")
	}
	if err := claims.validate(s.clock().UTC(), requiredScope); err != nil {
		return McpOAuthVerifiedAccessClaims{}, err
	}
	return claims, nil
}

func (s *McpOAuthSigner) JWKS() McpOAuthJWKS {
	return McpOAuthJWKS{Keys: []McpOAuthJWK{{
		Kty: "OKP",
		Crv: "Ed25519",
		Alg: "EdDSA",
		Use: "sig",
		Kid: s.kid,
		X:   base64.RawURLEncoding.EncodeToString(s.publicKey),
	}}}
}

func (s *McpOAuthSigner) signClaims(claims McpOAuthVerifiedAccessClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = s.kid
	return token.SignedString(s.privateKey)
}

func (c McpOAuthVerifiedAccessClaims) GetExpirationTime() (*jwt.NumericDate, error) {
	return c.ExpiresAt, nil
}

func (c McpOAuthVerifiedAccessClaims) GetIssuedAt() (*jwt.NumericDate, error) {
	return c.IssuedAt, nil
}

func (c McpOAuthVerifiedAccessClaims) GetNotBefore() (*jwt.NumericDate, error) {
	return nil, nil
}

func (c McpOAuthVerifiedAccessClaims) GetIssuer() (string, error) {
	return c.Issuer, nil
}

func (c McpOAuthVerifiedAccessClaims) GetSubject() (string, error) {
	return c.Subject, nil
}

func (c McpOAuthVerifiedAccessClaims) GetAudience() (jwt.ClaimStrings, error) {
	if c.Audience == "" {
		return nil, nil
	}
	return jwt.ClaimStrings{c.Audience}, nil
}

func (c McpOAuthVerifiedAccessClaims) validate(now time.Time, requiredScope string) error {
	if c.Issuer != McpOAuthIssuer {
		return errors.New("invalid issuer")
	}
	if c.Audience != McpOAuthResource {
		return errors.New("invalid audience")
	}
	if c.Resource != McpOAuthResource {
		return NewMcpOAuthError("invalid_target", "invalid resource")
	}
	if c.ExpiresAt == nil {
		return errors.New("missing exp")
	}
	if !c.ExpiresAt.Time.After(now) {
		return errors.New("token expired")
	}
	if c.IssuedAt == nil {
		return errors.New("missing iat")
	}
	if c.IssuedAt.Time.After(now) {
		return errors.New("iat is in the future")
	}
	if strings.TrimSpace(c.GrantID) == "" {
		return errors.New("grant_id is required")
	}
	if strings.TrimSpace(c.ClientID) == "" {
		return errors.New("client_id is required")
	}
	if strings.TrimSpace(c.ID) == "" {
		return errors.New("jti is required")
	}
	normalized, err := normalizeMcpOAuthScopeList(c.Scopes)
	if err != nil {
		return err
	}
	if requiredScope != "" {
		if !mcpOAuthScopeAllowed(requiredScope) {
			return errors.New("unknown required scope")
		}
		found := false
		for _, scope := range normalized {
			if scope == requiredScope {
				found = true
				break
			}
		}
		if !found {
			return errors.New("required scope missing")
		}
	}
	c.Scopes = normalized
	return nil
}

func deriveMcpOAuthKeyID(publicKey ed25519.PublicKey) string {
	sum := sha256.Sum256(publicKey)
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
