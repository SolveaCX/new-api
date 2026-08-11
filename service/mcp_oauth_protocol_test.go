package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestMcpOAuthProtocolMetadataIsCanonicalMCP20260728(t *testing.T) {
	metadata := McpOAuthAuthorizationServerMetadata()

	require.Equal(t, "https://console.flatkey.ai", metadata.Issuer)
	require.Equal(t, "https://console.flatkey.ai/oauth/authorize", metadata.AuthorizationEndpoint)
	require.Equal(t, "https://console.flatkey.ai/oauth/token", metadata.TokenEndpoint)
	require.Equal(t, "https://console.flatkey.ai/oauth/revoke", metadata.RevocationEndpoint)
	require.Equal(t, "https://console.flatkey.ai/oauth/jwks", metadata.JwksURI)
	require.Equal(t, []string{"S256"}, metadata.CodeChallengeMethodsSupported)
	require.Equal(t, []string{"authorization_code", "refresh_token"}, metadata.GrantTypesSupported)
	require.Equal(t, []string{"code"}, metadata.ResponseTypesSupported)
	require.Equal(t, []string{"none"}, metadata.TokenEndpointAuthMethodsSupported)
	require.Equal(t, []string{"tools:search", "tools:read", "tools:execute"}, metadata.ScopesSupported)
}

func TestMcpOAuthProtocolUsesConfiguredIssuerAndResource(t *testing.T) {
	t.Setenv("FLATKEY_MCP_OAUTH_ISSUER", " https://staging-console.flatkey.ai/ ")
	t.Setenv("FLATKEY_MCP_OAUTH_RESOURCE", " https://flatkey-mcp-staging.example/ ")

	metadata := McpOAuthAuthorizationServerMetadata()

	require.Equal(t, "https://staging-console.flatkey.ai", metadata.Issuer)
	require.Equal(t, "https://staging-console.flatkey.ai/oauth/authorize", metadata.AuthorizationEndpoint)
	require.Equal(t, "https://staging-console.flatkey.ai/oauth/token", metadata.TokenEndpoint)
	require.Equal(t, "https://staging-console.flatkey.ai/oauth/revoke", metadata.RevocationEndpoint)
	require.Equal(t, "https://staging-console.flatkey.ai/oauth/jwks", metadata.JwksURI)
	require.Equal(t, "https://staging-console.flatkey.ai/oauth/register", metadata.RegistrationEndpoint)

	require.NoError(t, ValidateMcpOAuthResource("https://flatkey-mcp-staging.example"))
	err := ValidateMcpOAuthResource(McpOAuthResource)
	var oauthErr *McpOAuthError
	require.ErrorAs(t, err, &oauthErr)
	require.Equal(t, "invalid_target", oauthErr.Code)
	require.Contains(t, oauthErr.Description, "https://flatkey-mcp-staging.example")
}

func TestMcpOAuthProtocolKeepsProductionFallbackForBlankConfiguredIssuerAndResource(t *testing.T) {
	t.Setenv("FLATKEY_MCP_OAUTH_ISSUER", " \t ")
	t.Setenv("FLATKEY_MCP_OAUTH_RESOURCE", " \t ")

	metadata := McpOAuthAuthorizationServerMetadata()

	require.Equal(t, McpOAuthIssuer, metadata.Issuer)
	require.NoError(t, ValidateMcpOAuthResource(McpOAuthResource))
}

func TestMcpOAuthProtocolRejectsUnsafeConfiguredIssuerAndResource(t *testing.T) {
	tests := []struct {
		name  string
		env   string
		value string
		call  func()
	}{
		{name: "issuer malformed", env: "FLATKEY_MCP_OAUTH_ISSUER", value: "https://", call: func() { _ = mcpOAuthIssuer() }},
		{name: "issuer invalid port", env: "FLATKEY_MCP_OAUTH_ISSUER", value: "https://staging-console.flatkey.ai:bad", call: func() { _ = mcpOAuthIssuer() }},
		{name: "issuer http", env: "FLATKEY_MCP_OAUTH_ISSUER", value: "http://staging-console.flatkey.ai", call: func() { _ = mcpOAuthIssuer() }},
		{name: "issuer userinfo", env: "FLATKEY_MCP_OAUTH_ISSUER", value: "https://user@staging-console.flatkey.ai", call: func() { _ = mcpOAuthIssuer() }},
		{name: "issuer query", env: "FLATKEY_MCP_OAUTH_ISSUER", value: "https://staging-console.flatkey.ai?x=1", call: func() { _ = mcpOAuthIssuer() }},
		{name: "issuer fragment", env: "FLATKEY_MCP_OAUTH_ISSUER", value: "https://staging-console.flatkey.ai#frag", call: func() { _ = mcpOAuthIssuer() }},
		{name: "resource malformed", env: "FLATKEY_MCP_OAUTH_RESOURCE", value: "https://", call: func() { _ = mcpOAuthResource() }},
		{name: "resource invalid port", env: "FLATKEY_MCP_OAUTH_RESOURCE", value: "https://flatkey-mcp-staging.example:bad", call: func() { _ = mcpOAuthResource() }},
		{name: "resource http", env: "FLATKEY_MCP_OAUTH_RESOURCE", value: "http://flatkey-mcp-staging.example", call: func() { _ = mcpOAuthResource() }},
		{name: "resource userinfo", env: "FLATKEY_MCP_OAUTH_RESOURCE", value: "https://user@flatkey-mcp-staging.example", call: func() { _ = mcpOAuthResource() }},
		{name: "resource query", env: "FLATKEY_MCP_OAUTH_RESOURCE", value: "https://flatkey-mcp-staging.example?x=1", call: func() { _ = mcpOAuthResource() }},
		{name: "resource fragment", env: "FLATKEY_MCP_OAUTH_RESOURCE", value: "https://flatkey-mcp-staging.example#frag", call: func() { _ = mcpOAuthResource() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.env, tt.value)
			require.Panics(t, tt.call)
		})
	}
}

func TestMcpOAuthPKCEVerifierRequiresRFC7636S256Challenge(t *testing.T) {
	verifier := strings.Repeat("a", 43) + "-._~"
	challenge := McpOAuthS256Challenge(verifier)

	require.NoError(t, ValidateMcpOAuthPKCE(verifier, challenge, "S256"))

	tests := []struct {
		name      string
		verifier  string
		challenge string
		method    string
	}{
		{name: "too short", verifier: strings.Repeat("a", 42), challenge: challenge, method: "S256"},
		{name: "too long", verifier: strings.Repeat("a", 129), challenge: challenge, method: "S256"},
		{name: "invalid character", verifier: strings.Repeat("a", 42) + "!", challenge: challenge, method: "S256"},
		{name: "plain rejected", verifier: verifier, challenge: verifier, method: "plain"},
		{name: "missing method", verifier: verifier, challenge: challenge, method: ""},
		{name: "wrong challenge", verifier: verifier, challenge: McpOAuthS256Challenge(verifier + "b"), method: "S256"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, ValidateMcpOAuthPKCE(tt.verifier, tt.challenge, tt.method))
		})
	}
}

func TestMcpOAuthValidationRequiresExactResourceAllowedScopesAndExactRedirect(t *testing.T) {
	client := McpOAuthClientMetadata{
		ClientID:     "https://client.example/app",
		ClientName:   "Client",
		RedirectURIs: []string{"https://client.example/callback", "http://127.0.0.1:49152/callback"},
	}

	scopes, err := NormalizeMcpOAuthScopes("tools:execute tools:read tools:read")
	require.NoError(t, err)
	require.Equal(t, []string{"tools:read", "tools:execute"}, scopes)

	scopes, err = NormalizeMcpOAuthScopes("tools:execute")
	require.NoError(t, err)
	require.Equal(t, []string{"tools:execute"}, scopes)

	require.NoError(t, ValidateMcpOAuthResource(McpOAuthResource))
	err = ValidateMcpOAuthResource("https://other.example")
	var oauthErr *McpOAuthError
	require.ErrorAs(t, err, &oauthErr)
	require.Equal(t, "invalid_target", oauthErr.Code)

	require.NoError(t, ValidateMcpOAuthRedirectURI(client, "https://client.example/callback"))
	require.NoError(t, ValidateMcpOAuthRedirectURI(client, "http://127.0.0.1:49152/callback"))
	require.Error(t, ValidateMcpOAuthRedirectURI(client, "https://client.example/callback/extra"))
	require.Error(t, ValidateMcpOAuthRedirectURI(client, "http://client.example/callback"))
	require.Error(t, ValidateMcpOAuthRedirectURI(client, "https://user@client.example/callback"))
	require.Error(t, ValidateMcpOAuthRedirectURI(client, "https://client.example/callback#fragment"))
	require.Error(t, ValidateMcpOAuthRedirectURI(client, "http://192.168.1.20/callback"))

	_, err = NormalizeMcpOAuthScopes("tools:read tools:admin")
	require.Error(t, err)
}

func TestMcpOAuthCIMDClientIDAndMetadataValidation(t *testing.T) {
	valid := McpOAuthClientMetadata{
		ClientID:     "https://client.example/oauth/client",
		ClientName:   "Client",
		RedirectURIs: []string{"https://client.example/callback"},
	}
	require.NoError(t, ValidateMcpOAuthCIMDClientID(valid.ClientID))
	require.NoError(t, ValidateMcpOAuthCIMDMetadata("https://client.example/oauth/client", valid))

	for _, clientID := range []string{
		"http://client.example/oauth/client",
		"https://client.example",
		"https://user@client.example/oauth/client",
		"https://client.example/oauth/client#fragment",
	} {
		require.Error(t, ValidateMcpOAuthCIMDClientID(clientID), clientID)
	}

	require.Error(t, ValidateMcpOAuthCIMDMetadata("https://client.example/oauth/client", McpOAuthClientMetadata{
		ClientID:     "https://client.example/other",
		ClientName:   "Client",
		RedirectURIs: []string{"https://client.example/callback"},
	}))
	require.Error(t, ValidateMcpOAuthCIMDMetadata(valid.ClientID, McpOAuthClientMetadata{ClientID: valid.ClientID, RedirectURIs: valid.RedirectURIs}))
	require.Error(t, ValidateMcpOAuthCIMDMetadata(valid.ClientID, McpOAuthClientMetadata{ClientID: valid.ClientID, ClientName: "Client"}))
}

func TestMcpOAuthDCRRequestAcceptsOnlyPublicAuthorizationCodeClients(t *testing.T) {
	out, err := ValidateMcpOAuthDCRRequest(McpOAuthDCRRequest{
		ClientName:              "Client",
		RedirectURIs:            []string{"https://client.example/callback"},
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "none",
	})
	require.NoError(t, err)
	require.Equal(t, "none", out.TokenEndpointAuthMethod)

	_, err = ValidateMcpOAuthDCRRequest(McpOAuthDCRRequest{ClientName: "Client", RedirectURIs: []string{"http://client.example/callback"}, GrantTypes: []string{"authorization_code"}, ResponseTypes: []string{"code"}, TokenEndpointAuthMethod: "none"})
	require.Error(t, err)
	_, err = ValidateMcpOAuthDCRRequest(McpOAuthDCRRequest{ClientName: "Client", RedirectURIs: []string{"https://client.example/callback"}, GrantTypes: []string{"client_credentials"}, ResponseTypes: []string{"code"}, TokenEndpointAuthMethod: "none"})
	require.Error(t, err)
	_, err = ValidateMcpOAuthDCRRequest(McpOAuthDCRRequest{ClientName: "Client", RedirectURIs: []string{"https://client.example/callback"}, GrantTypes: []string{"authorization_code"}, ResponseTypes: []string{"token"}, TokenEndpointAuthMethod: "none"})
	require.Error(t, err)
	_, err = ValidateMcpOAuthDCRRequest(McpOAuthDCRRequest{ClientName: "Client", RedirectURIs: []string{"https://client.example/callback"}, GrantTypes: []string{"authorization_code"}, ResponseTypes: []string{"code"}, TokenEndpointAuthMethod: "client_secret_basic"})
	require.Error(t, err)
}

func TestMcpOAuthCIMDFetcherRejectsUnsafeNetworkAndResponseShapes(t *testing.T) {
	fetcherType := reflect.TypeOf(McpOAuthCIMDFetcher{})
	_, hasResolver := fetcherType.FieldByName("Resolver")
	_, hasDoer := fetcherType.FieldByName("Doer")
	require.False(t, hasResolver)
	require.False(t, hasDoer)

	tests := []struct {
		name       string
		resolverIP string
		extraIP    string
		response   *http.Response
		err        error
		want       string
	}{
		{name: "private dns", resolverIP: "10.0.0.2", want: "unsafe"},
		{name: "loopback dns", resolverIP: "127.0.0.1", want: "unsafe"},
		{name: "cgnat dns", resolverIP: "100.64.0.1", want: "unsafe"},
		{name: "reserved dns", resolverIP: "192.0.2.1", want: "unsafe"},
		{name: "mixed unsafe dns", resolverIP: "93.184.216.34", extraIP: "172.16.0.10", want: "unsafe"},
		{name: "http status", resolverIP: "93.184.216.34", response: testMcpOAuthHTTPResponse(500, "application/json", `{}`), want: "status"},
		{name: "content type", resolverIP: "93.184.216.34", response: testMcpOAuthHTTPResponse(200, "text/plain", `{}`), want: "content-type"},
		{name: "json substring content type", resolverIP: "93.184.216.34", response: testMcpOAuthHTTPResponse(200, "text/application-json", `{}`), want: "content-type"},
		{name: "invalid json media type parameter", resolverIP: "93.184.216.34", response: testMcpOAuthHTTPResponse(200, "application/json; charset", `{}`), want: "content-type"},
		{name: "body limit", resolverIP: "93.184.216.34", response: testMcpOAuthHTTPResponse(200, "application/json", strings.Repeat(" ", 64*1024+1)), want: "64KiB"},
		{name: "invalid json", resolverIP: "93.184.216.34", response: testMcpOAuthHTTPResponse(200, "application/json", `{`), want: "json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetcher := newTestMcpOAuthCIMDFetcher(
				mcpOAuthResolverFunc(func(ctx context.Context, host string) ([]net.IP, error) {
					ips := []net.IP{net.ParseIP(tt.resolverIP)}
					if tt.extraIP != "" {
						ips = append(ips, net.ParseIP(tt.extraIP))
					}
					return ips, nil
				}),
				mcpOAuthDoerFunc(func(req *http.Request) (*http.Response, error) {
					if tt.err != nil {
						return nil, tt.err
					}
					require.Equal(t, "https://client.example/oauth/client", req.URL.String())
					require.Equal(t, tt.resolverIP, req.Context().Value(mcpOAuthVerifiedIPContextKey{}))
					return tt.response, nil
				}),
			)

			_, err := fetcher.Fetch(context.Background(), "https://client.example/oauth/client")
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestMcpOAuthCIMDFetcherDecodesValidJSONAndDoesNotExposeSecrets(t *testing.T) {
	fetcher := newTestMcpOAuthCIMDFetcher(
		mcpOAuthResolverFunc(func(ctx context.Context, host string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("93.184.216.34")}, nil
		}),
		mcpOAuthDoerFunc(func(req *http.Request) (*http.Response, error) {
			return testMcpOAuthHTTPResponse(200, "application/client-metadata+json", `{
				"client_id":"https://client.example/oauth/client",
				"client_name":"Client",
				"redirect_uris":["https://client.example/callback"],
				"client_secret":"must-not-survive"
			}`), nil
		}),
	)

	metadata, err := fetcher.Fetch(context.Background(), "https://client.example/oauth/client")
	require.NoError(t, err)
	require.Equal(t, "https://client.example/oauth/client", metadata.ClientID)
	require.Equal(t, []string{"https://client.example/callback"}, metadata.RedirectURIs)
	raw := testMcpOAuthMustMarshal(t, metadata)
	require.NotContains(t, string(raw), "client_secret")
}

func TestMcpOAuthDefaultFetcherDisablesRedirectsAndDialsVerifiedIP(t *testing.T) {
	resolver := mcpOAuthResolverFunc(func(ctx context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	})
	dialed := ""
	fetcher := NewMcpOAuthDefaultCIMDFetcher(resolver, func(ctx context.Context, network, address string) (net.Conn, error) {
		dialed = address
		return nil, errors.New("stop before network")
	})

	_, err := fetcher.Fetch(context.Background(), "https://client.example/oauth/client")
	require.ErrorContains(t, err, "stop before network")
	require.True(t, strings.HasPrefix(dialed, "93.184.216.34:"))

	req, _ := http.NewRequest(http.MethodGet, "https://client.example/next", nil)
	require.Error(t, fetcher.HTTPClient().CheckRedirect(req, []*http.Request{{}}))
}

type mcpOAuthResolverFunc func(context.Context, string) ([]net.IP, error)

func (f mcpOAuthResolverFunc) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	return f(ctx, host)
}

type mcpOAuthDoerFunc func(*http.Request) (*http.Response, error)

func (f mcpOAuthDoerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestMcpOAuthCIMDFetcher(resolver McpOAuthResolver, doer McpOAuthDoer) McpOAuthCIMDFetcher {
	return McpOAuthCIMDFetcher{resolver: resolver, doer: doer}
}

func testMcpOAuthHTTPResponse(status int, contentType string, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func testMcpOAuthMustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := common.Marshal(v)
	require.NoError(t, err)
	return bytes.TrimSpace(raw)
}
