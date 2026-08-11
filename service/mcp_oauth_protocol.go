package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	McpOAuthIssuer   = "https://console.flatkey.ai"
	McpOAuthResource = "https://mcp.flatkey.ai"
)

var mcpOAuthAllowedScopes = []string{"tools:search", "tools:read", "tools:execute"}

type McpOAuthError struct {
	Code        string
	Description string
}

type McpOAuthAuthorizationServerMetadataDTO struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RevocationEndpoint                string   `json:"revocation_endpoint"`
	JwksURI                           string   `json:"jwks_uri"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ScopesSupported                   []string `json:"scopes_supported"`
}

type McpOAuthClientMetadata struct {
	ClientID     string   `json:"client_id"`
	ClientName   string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
}

type McpOAuthDCRRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

type McpOAuthValidatedDCRClient struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

type McpOAuthResolver interface {
	LookupIP(context.Context, string) ([]net.IP, error)
}

type McpOAuthDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type McpOAuthCIMDFetcher struct {
	Resolver McpOAuthResolver
	Doer     McpOAuthDoer
	client   *http.Client
}

type mcpOAuthVerifiedIPContextKey struct{}

type mcpOAuthNetResolver struct{}

func (mcpOAuthNetResolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	var resolver net.Resolver
	return resolver.LookupIP(ctx, "ip", host)
}

func (e *McpOAuthError) Error() string {
	if e.Description == "" {
		return e.Code
	}
	return e.Code + ": " + e.Description
}

func NewMcpOAuthError(code, description string) *McpOAuthError {
	return &McpOAuthError{Code: code, Description: description}
}

func McpOAuthAuthorizationServerMetadata() McpOAuthAuthorizationServerMetadataDTO {
	return McpOAuthAuthorizationServerMetadataDTO{
		Issuer:                            McpOAuthIssuer,
		AuthorizationEndpoint:             McpOAuthIssuer + "/oauth/authorize",
		TokenEndpoint:                     McpOAuthIssuer + "/oauth/token",
		RevocationEndpoint:                McpOAuthIssuer + "/oauth/revoke",
		JwksURI:                           McpOAuthIssuer + "/oauth/jwks",
		CodeChallengeMethodsSupported:     []string{"S256"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
		ResponseTypesSupported:            []string{"code"},
		TokenEndpointAuthMethodsSupported: []string{"none"},
		ScopesSupported:                   append([]string(nil), mcpOAuthAllowedScopes...),
	}
}

func McpOAuthS256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func ValidateMcpOAuthPKCE(verifier, challenge, method string) error {
	if method != "S256" {
		return errors.New("PKCE code_challenge_method must be S256")
	}
	if len(verifier) < 43 || len(verifier) > 128 {
		return errors.New("PKCE verifier length must be 43..128")
	}
	for _, r := range verifier {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.' || r == '_' || r == '~') {
			return errors.New("PKCE verifier contains non-unreserved characters")
		}
	}
	if challenge == "" || McpOAuthS256Challenge(verifier) != challenge {
		return errors.New("PKCE S256 challenge mismatch")
	}
	return nil
}

func ValidateMcpOAuthResource(resource string) error {
	if resource != McpOAuthResource {
		return NewMcpOAuthError("invalid_target", "resource must exactly match https://mcp.flatkey.ai")
	}
	return nil
}

func NormalizeMcpOAuthScopes(scopeText string) ([]string, error) {
	if strings.TrimSpace(scopeText) == "" {
		return nil, errors.New("scope is required")
	}
	return normalizeMcpOAuthScopeList(strings.Fields(scopeText))
}

func normalizeMcpOAuthScopeList(scopes []string) ([]string, error) {
	seen := map[string]bool{}
	for _, scope := range scopes {
		if !mcpOAuthScopeAllowed(scope) {
			return nil, fmt.Errorf("unknown scope %q", scope)
		}
		seen[scope] = true
	}
	normalized := make([]string, 0, len(seen))
	for _, scope := range mcpOAuthAllowedScopes {
		if seen[scope] {
			normalized = append(normalized, scope)
		}
	}
	if len(normalized) == 0 {
		return nil, errors.New("scope is required")
	}
	return normalized, nil
}

func mcpOAuthScopeAllowed(scope string) bool {
	for _, allowed := range mcpOAuthAllowedScopes {
		if scope == allowed {
			return true
		}
	}
	return false
}

func ValidateMcpOAuthRedirectURI(client McpOAuthClientMetadata, redirectURI string) error {
	if err := validateMcpOAuthRedirectURISafe(redirectURI); err != nil {
		return err
	}
	for _, registered := range client.RedirectURIs {
		if redirectURI == registered {
			return nil
		}
	}
	return errors.New("redirect_uri must exactly match a registered URI")
}

func ValidateMcpOAuthCIMDClientID(clientID string) error {
	u, err := url.Parse(clientID)
	if err != nil {
		return err
	}
	if u.Scheme != "https" {
		return errors.New("client_id must be an HTTPS URL")
	}
	if u.User != nil {
		return errors.New("client_id must not contain userinfo")
	}
	if u.Fragment != "" {
		return errors.New("client_id must not contain a fragment")
	}
	if u.Host == "" || strings.Trim(u.EscapedPath(), "/") == "" {
		return errors.New("client_id must include a host and real path")
	}
	return nil
}

func ValidateMcpOAuthCIMDMetadata(clientID string, metadata McpOAuthClientMetadata) error {
	if err := ValidateMcpOAuthCIMDClientID(clientID); err != nil {
		return err
	}
	if metadata.ClientID != clientID {
		return errors.New("metadata client_id must exactly match the client_id URL")
	}
	if strings.TrimSpace(metadata.ClientName) == "" {
		return errors.New("client_name is required")
	}
	if len(metadata.RedirectURIs) == 0 {
		return errors.New("redirect_uris is required")
	}
	for _, redirectURI := range metadata.RedirectURIs {
		if err := validateMcpOAuthRedirectURISafe(redirectURI); err != nil {
			return err
		}
	}
	return nil
}

func ValidateMcpOAuthDCRRequest(req McpOAuthDCRRequest) (McpOAuthValidatedDCRClient, error) {
	if strings.TrimSpace(req.ClientName) == "" {
		return McpOAuthValidatedDCRClient{}, errors.New("client_name is required")
	}
	if len(req.RedirectURIs) == 0 {
		return McpOAuthValidatedDCRClient{}, errors.New("redirect_uris is required")
	}
	for _, redirectURI := range req.RedirectURIs {
		if err := validateMcpOAuthRedirectURISafe(redirectURI); err != nil {
			return McpOAuthValidatedDCRClient{}, err
		}
	}
	if !stringSetEqual(defaultIfEmpty(req.GrantTypes, []string{"authorization_code"}), []string{"authorization_code", "refresh_token"}) &&
		!stringSetEqual(defaultIfEmpty(req.GrantTypes, []string{"authorization_code"}), []string{"authorization_code"}) {
		return McpOAuthValidatedDCRClient{}, errors.New("grant_types must only contain authorization_code and refresh_token")
	}
	if !stringSetEqual(defaultIfEmpty(req.ResponseTypes, []string{"code"}), []string{"code"}) {
		return McpOAuthValidatedDCRClient{}, errors.New("response_types must only contain code")
	}
	authMethod := req.TokenEndpointAuthMethod
	if authMethod == "" {
		authMethod = "none"
	}
	if authMethod != "none" {
		return McpOAuthValidatedDCRClient{}, errors.New("token_endpoint_auth_method must be none")
	}
	return McpOAuthValidatedDCRClient{
		ClientName:              strings.TrimSpace(req.ClientName),
		RedirectURIs:            append([]string(nil), req.RedirectURIs...),
		GrantTypes:              defaultIfEmpty(req.GrantTypes, []string{"authorization_code"}),
		ResponseTypes:           defaultIfEmpty(req.ResponseTypes, []string{"code"}),
		TokenEndpointAuthMethod: authMethod,
	}, nil
}

func NewMcpOAuthDefaultCIMDFetcher(resolver McpOAuthResolver, dialContext func(context.Context, string, string) (net.Conn, error)) McpOAuthCIMDFetcher {
	if resolver == nil {
		resolver = mcpOAuthNetResolver{}
	}
	if dialContext == nil {
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		dialContext = dialer.DialContext
	}
	fetcher := McpOAuthCIMDFetcher{Resolver: resolver}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := resolver.LookupIP(ctx, host)
			if err != nil {
				return nil, err
			}
			verifiedIP, err := firstSafeMcpOAuthIP(ips)
			if err != nil {
				return nil, err
			}
			return dialContext(ctx, network, net.JoinHostPort(verifiedIP.String(), port))
		},
	}
	fetcher.client = &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	fetcher.Doer = fetcher.client
	return fetcher
}

func (f McpOAuthCIMDFetcher) HTTPClient() *http.Client {
	return f.client
}

func (f McpOAuthCIMDFetcher) Fetch(ctx context.Context, clientID string) (McpOAuthClientMetadata, error) {
	if err := ValidateMcpOAuthCIMDClientID(clientID); err != nil {
		return McpOAuthClientMetadata{}, err
	}
	resolver := f.Resolver
	if resolver == nil {
		resolver = mcpOAuthNetResolver{}
	}
	u, err := url.Parse(clientID)
	if err != nil {
		return McpOAuthClientMetadata{}, err
	}
	ips, err := resolver.LookupIP(ctx, u.Hostname())
	if err != nil {
		return McpOAuthClientMetadata{}, err
	}
	verifiedIP, err := firstSafeMcpOAuthIP(ips)
	if err != nil {
		return McpOAuthClientMetadata{}, err
	}
	doer := f.Doer
	if doer == nil {
		defaultFetcher := NewMcpOAuthDefaultCIMDFetcher(resolver, nil)
		doer = defaultFetcher.Doer
	}
	req, err := http.NewRequestWithContext(context.WithValue(ctx, mcpOAuthVerifiedIPContextKey{}, verifiedIP.String()), http.MethodGet, clientID, nil)
	if err != nil {
		return McpOAuthClientMetadata{}, err
	}
	resp, err := doer.Do(req)
	if err != nil {
		return McpOAuthClientMetadata{}, err
	}
	if resp == nil {
		return McpOAuthClientMetadata{}, errors.New("metadata response is nil")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return McpOAuthClientMetadata{}, fmt.Errorf("metadata status must be 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "application/json") {
		return McpOAuthClientMetadata{}, errors.New("metadata content-type must be JSON")
	}
	limited := io.LimitReader(resp.Body, 64*1024+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return McpOAuthClientMetadata{}, err
	}
	if len(body) > 64*1024 {
		return McpOAuthClientMetadata{}, errors.New("metadata body must be <=64KiB")
	}
	var metadata McpOAuthClientMetadata
	if err := common.Unmarshal(bytes.TrimSpace(body), &metadata); err != nil {
		return McpOAuthClientMetadata{}, fmt.Errorf("decode metadata json: %w", err)
	}
	if err := ValidateMcpOAuthCIMDMetadata(clientID, metadata); err != nil {
		return McpOAuthClientMetadata{}, err
	}
	return metadata, nil
}

func firstSafeMcpOAuthIP(ips []net.IP) (net.IP, error) {
	if len(ips) == 0 {
		return nil, errors.New("DNS returned no addresses")
	}
	for _, ip := range ips {
		if !isSafeMcpOAuthDialIP(ip) {
			return nil, fmt.Errorf("unsafe resolved IP %s", ip.String())
		}
	}
	return ips[0], nil
}

func isSafeMcpOAuthDialIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsValid() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return false
	}
	if addr.Is4() {
		as4 := addr.As4()
		if as4[0] == 100 && as4[1] >= 64 && as4[1] <= 127 {
			return false
		}
		if as4[0] == 0 || as4[0] == 127 || as4[0] >= 224 {
			return false
		}
	}
	if isReservedMcpOAuthIP(addr) {
		return false
	}
	return true
}

func isReservedMcpOAuthIP(addr netip.Addr) bool {
	reserved := []string{
		"192.0.0.0/24",
		"192.0.2.0/24",
		"192.88.99.0/24",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"240.0.0.0/4",
		"2001:db8::/32",
	}
	for _, rawPrefix := range reserved {
		prefix := netip.MustParsePrefix(rawPrefix)
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func validateMcpOAuthRedirectURISafe(rawURI string) error {
	u, err := url.Parse(rawURI)
	if err != nil {
		return err
	}
	if u.User != nil {
		return errors.New("redirect_uri must not contain userinfo")
	}
	if u.Fragment != "" {
		return errors.New("redirect_uri must not contain a fragment")
	}
	if u.Scheme == "https" && u.Host != "" {
		return nil
	}
	if u.Scheme == "http" && isMcpOAuthLoopbackHost(u.Hostname()) && u.Port() != "" {
		return nil
	}
	return errors.New("redirect_uri must be https or RFC8252 loopback http")
}

func isMcpOAuthLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func stringSetEqual(a, b []string) bool {
	a = append([]string(nil), a...)
	b = append([]string(nil), b...)
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func defaultIfEmpty(values []string, defaults []string) []string {
	if len(values) == 0 {
		return append([]string(nil), defaults...)
	}
	return append([]string(nil), values...)
}
