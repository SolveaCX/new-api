package service

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestMcpOAuthSigningLoadsPKCS8EnvSignsVerifiesAccessJWTAndPublishesPublicJWKS(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	privateKey := testMcpOAuthPrivateKey(t)
	t.Setenv("FLATKEY_MCP_OAUTH_ED25519_PRIVATE_KEY", testMcpOAuthPKCS8Env(t, privateKey))

	signer, err := NewMcpOAuthSignerFromEnv(McpOAuthSigningConfig{
		Clock:      func() time.Time { return now },
		Randomness: bytes.NewReader(bytes.Repeat([]byte{0x42}, 32)),
	})
	require.NoError(t, err)

	token, err := signer.SignAccessToken(McpOAuthAccessTokenRequest{
		Subject:  "user-123",
		GrantID:  "grant-456",
		ClientID: "https://client.example/app",
		Scopes:   []string{"tools:read", "tools:search"},
		Resource: McpOAuthResource,
	})
	require.NoError(t, err)
	require.NotContains(t, token, base64.StdEncoding.EncodeToString(privateKey))

	parsed, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	require.NoError(t, err)
	require.Equal(t, "EdDSA", parsed.Header["alg"])
	require.Equal(t, signer.KeyID(), parsed.Header["kid"])

	claims, err := signer.VerifyAccessToken(token, "tools:read")
	require.NoError(t, err)
	require.Equal(t, McpOAuthIssuer, claims.Issuer)
	require.Equal(t, McpOAuthResource, claims.Audience)
	require.Equal(t, "user-123", claims.Subject)
	require.Equal(t, "grant-456", claims.GrantID)
	require.Equal(t, "https://client.example/app", claims.ClientID)
	require.Equal(t, []string{"tools:search", "tools:read"}, claims.Scopes)
	require.Equal(t, McpOAuthResource, claims.Resource)
	require.Equal(t, now.Unix(), claims.IssuedAt.Unix())
	require.Equal(t, now.Add(15*time.Minute).Unix(), claims.ExpiresAt.Unix())
	require.NotEmpty(t, claims.ID)

	jwks := signer.JWKS()
	require.Len(t, jwks.Keys, 1)
	require.Equal(t, "OKP", jwks.Keys[0].Kty)
	require.Equal(t, "Ed25519", jwks.Keys[0].Crv)
	require.Equal(t, "EdDSA", jwks.Keys[0].Alg)
	require.Equal(t, "sig", jwks.Keys[0].Use)
	require.Equal(t, signer.KeyID(), jwks.Keys[0].Kid)
	require.NotEmpty(t, jwks.Keys[0].X)
	rawJwks := testMcpOAuthMustMarshal(t, jwks)
	require.NotContains(t, string(rawJwks), `"d":`)
	require.NotContains(t, string(rawJwks), base64.StdEncoding.EncodeToString(privateKey))
}

func TestMcpOAuthSigningRejectsNonEd25519PKCS8Env(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	raw, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	require.NoError(t, err)
	t.Setenv("FLATKEY_MCP_OAUTH_ED25519_PRIVATE_KEY", base64.StdEncoding.EncodeToString(raw))

	_, err = NewMcpOAuthSignerFromEnv(McpOAuthSigningConfig{})
	require.ErrorContains(t, err, "ed25519")
}

func TestMcpOAuthSigningVerifyRejectsInvalidTokensAndScopes(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	signer := testMcpOAuthSigner(t, now)
	otherSigner := testMcpOAuthSigner(t, now)

	valid := McpOAuthVerifiedAccessClaims{
		Issuer:    McpOAuthIssuer,
		Audience:  McpOAuthResource,
		Subject:   "user-123",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		ID:        "jti-123",
		GrantID:   "grant-456",
		ClientID:  "https://client.example/app",
		Scopes:    []string{"tools:read"},
		Resource:  McpOAuthResource,
	}

	tests := []struct {
		name          string
		claims        McpOAuthVerifiedAccessClaims
		signWith      *McpOAuthSigner
		kid           string
		requiredScope string
		want          string
	}{
		{name: "wrong key", claims: valid, signWith: otherSigner, requiredScope: "tools:read", want: "signature"},
		{name: "wrong kid", claims: valid, kid: "wrong-kid", requiredScope: "tools:read", want: "kid"},
		{name: "missing kid", claims: valid, kid: "-", requiredScope: "tools:read", want: "kid"},
		{name: "wrong issuer", claims: withMcpOAuthIssuer(valid, "https://evil.example"), requiredScope: "tools:read", want: "issuer"},
		{name: "wrong audience", claims: withMcpOAuthAudience(valid, "https://other.example"), requiredScope: "tools:read", want: "audience"},
		{name: "empty subject", claims: withMcpOAuthSubject(valid, ""), requiredScope: "tools:read", want: "sub"},
		{name: "wrong resource", claims: withMcpOAuthResource(valid, "https://other.example"), requiredScope: "tools:read", want: "resource"},
		{name: "expired", claims: withMcpOAuthExpiry(valid, now.Add(-time.Minute)), requiredScope: "tools:read", want: "expired"},
		{name: "missing exp", claims: withMcpOAuthExpiry(valid, time.Time{}), requiredScope: "tools:read", want: "exp"},
		{name: "future iat", claims: withMcpOAuthIssuedAt(valid, now.Add(time.Minute)), requiredScope: "tools:read", want: "iat"},
		{name: "missing iat", claims: withMcpOAuthIssuedAt(valid, time.Time{}), requiredScope: "tools:read", want: "iat"},
		{name: "lifetime one second too long", claims: withMcpOAuthExpiry(valid, now.Add(15*time.Minute+time.Second)), requiredScope: "tools:read", want: "lifetime"},
		{name: "lifetime too long", claims: withMcpOAuthExpiry(valid, now.Add(15*time.Minute+2*time.Second)), requiredScope: "tools:read", want: "lifetime"},
		{name: "empty grant", claims: withMcpOAuthGrant(valid, ""), requiredScope: "tools:read", want: "grant_id"},
		{name: "empty client", claims: withMcpOAuthClient(valid, ""), requiredScope: "tools:read", want: "client_id"},
		{name: "empty jti", claims: withMcpOAuthJTI(valid, ""), requiredScope: "tools:read", want: "jti"},
		{name: "unknown scope", claims: withMcpOAuthScopes(valid, []string{"tools:admin"}), requiredScope: "tools:read", want: "scope"},
		{name: "required scope missing", claims: withMcpOAuthScopes(valid, []string{"tools:search"}), requiredScope: "tools:read", want: "scope"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signWith := tt.signWith
			if signWith == nil {
				signWith = signer
			}
			token := testMcpOAuthSignClaims(t, signWith, tt.claims)
			if tt.name == "wrong key" {
				token = testMcpOAuthSignClaimsWithKid(t, signWith, tt.claims, signer.KeyID())
			}
			if tt.kid == "wrong-kid" {
				token = testMcpOAuthSignClaimsWithKid(t, signer, tt.claims, "wrong-kid")
			}
			if tt.kid == "-" {
				token = testMcpOAuthSignClaimsWithKid(t, signer, tt.claims, "")
			}
			_, err := signer.VerifyAccessToken(token, tt.requiredScope)
			require.ErrorContains(t, err, tt.want)
		})
	}

	hsToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, valid).SignedString([]byte("secret"))
	require.NoError(t, err)
	_, err = signer.VerifyAccessToken(hsToken, "tools:read")
	require.ErrorContains(t, err, "alg")
}

func testMcpOAuthPrivateKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return privateKey
}

func testMcpOAuthPKCS8Env(t *testing.T, privateKey ed25519.PrivateKey) string {
	t.Helper()
	raw, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(raw)
}

func testMcpOAuthSigner(t *testing.T, now time.Time) *McpOAuthSigner {
	t.Helper()
	privateKey := testMcpOAuthPrivateKey(t)
	signer, err := NewMcpOAuthSigner(privateKey, McpOAuthSigningConfig{
		Clock:      func() time.Time { return now },
		Randomness: strings.NewReader("0123456789abcdef0123456789abcdef"),
	})
	require.NoError(t, err)
	return signer
}

func testMcpOAuthSignClaims(t *testing.T, signer *McpOAuthSigner, claims McpOAuthVerifiedAccessClaims) string {
	t.Helper()
	token, err := signer.signClaims(claims)
	require.NoError(t, err)
	return token
}

func testMcpOAuthSignClaimsWithKid(t *testing.T, signer *McpOAuthSigner, claims McpOAuthVerifiedAccessClaims, kid string) string {
	t.Helper()
	tokenWithClaims := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	if kid != "" {
		tokenWithClaims.Header["kid"] = kid
	} else {
		delete(tokenWithClaims.Header, "kid")
	}
	token, err := tokenWithClaims.SignedString(signer.privateKey)
	require.NoError(t, err)
	return token
}

func withMcpOAuthIssuer(c McpOAuthVerifiedAccessClaims, v string) McpOAuthVerifiedAccessClaims {
	c.Issuer = v
	return c
}

func withMcpOAuthSubject(c McpOAuthVerifiedAccessClaims, v string) McpOAuthVerifiedAccessClaims {
	c.Subject = v
	return c
}

func withMcpOAuthAudience(c McpOAuthVerifiedAccessClaims, v string) McpOAuthVerifiedAccessClaims {
	c.Audience = v
	return c
}

func withMcpOAuthResource(c McpOAuthVerifiedAccessClaims, v string) McpOAuthVerifiedAccessClaims {
	c.Resource = v
	return c
}

func withMcpOAuthExpiry(c McpOAuthVerifiedAccessClaims, v time.Time) McpOAuthVerifiedAccessClaims {
	if v.IsZero() {
		c.ExpiresAt = nil
	} else {
		c.ExpiresAt = jwt.NewNumericDate(v)
	}
	return c
}

func withMcpOAuthIssuedAt(c McpOAuthVerifiedAccessClaims, v time.Time) McpOAuthVerifiedAccessClaims {
	if v.IsZero() {
		c.IssuedAt = nil
	} else {
		c.IssuedAt = jwt.NewNumericDate(v)
	}
	return c
}

func withMcpOAuthGrant(c McpOAuthVerifiedAccessClaims, v string) McpOAuthVerifiedAccessClaims {
	c.GrantID = v
	return c
}

func withMcpOAuthClient(c McpOAuthVerifiedAccessClaims, v string) McpOAuthVerifiedAccessClaims {
	c.ClientID = v
	return c
}

func withMcpOAuthJTI(c McpOAuthVerifiedAccessClaims, v string) McpOAuthVerifiedAccessClaims {
	c.ID = v
	return c
}

func withMcpOAuthScopes(c McpOAuthVerifiedAccessClaims, v []string) McpOAuthVerifiedAccessClaims {
	c.Scopes = v
	return c
}
