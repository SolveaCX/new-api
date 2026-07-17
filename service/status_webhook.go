package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	statusWebhookConnectTimeout    = 2 * time.Second
	statusWebhookTotalTimeout      = 8 * time.Second
	statusWebhookTLSHandshake      = 3 * time.Second
	statusWebhookResponseHeader    = 4 * time.Second
	statusWebhookDNSLookupTimeout  = 2 * time.Second
	statusWebhookMaxResponseBytes  = int64(64 * 1024)
	statusWebhookMaxConcurrency    = 8
	statusWebhookChallengeEventID  = "status.webhook.challenge"
	statusWebhookChallengeBodyType = "challenge"
)

var (
	ErrStatusWebhookChallengeFailed  = errors.New("status webhook challenge failed")
	ErrStatusWebhookResponseTooLarge = errors.New("status webhook response is too large")
)

var statusWebhookDeniedPrefixes = mustStatusWebhookPrefixes(
	"0.0.0.0/8",
	"10.0.0.0/8",
	"100.64.0.0/10",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"172.16.0.0/12",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"192.88.99.0/24",
	"192.168.0.0/16",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"224.0.0.0/4",
	"240.0.0.0/4",
	"::/128",
	"::1/128",
	"64:ff9b:1::/48",
	"100::/64",
	"2001::/23",
	"2001:db8::/32",
	"2002::/16",
	"fc00::/7",
	"fe80::/10",
	"ff00::/8",
)

type StatusWebhookResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type StatusWebhookRequest struct {
	Endpoint      string
	EventID       string
	Timestamp     int64
	Body          []byte
	SigningSecret string
}

type StatusWebhookResponse struct {
	StatusCode int
	Body       []byte
}

type StatusWebhookSender interface {
	Send(ctx context.Context, request StatusWebhookRequest) (StatusWebhookResponse, error)
}

type statusWebhookDialer struct {
	resolver    StatusWebhookResolver
	dialContext func(context.Context, string, string) (net.Conn, error)
}

type statusWebhookLimiter struct {
	semaphore chan struct{}
}

type StatusSafeWebhookClient struct {
	httpClient       *http.Client
	resolver         StatusWebhookResolver
	allowedPorts     map[int]struct{}
	maxResponseBytes int64
	limiter          *statusWebhookLimiter
}

type StatusWebhookRegistrationService struct {
	Keyring *StatusSecretKeyring
	Sender  StatusWebhookSender
	Now     func() int64
}

type StatusWebhookRegistrationResult struct {
	SubscriberID  int64  `json:"subscriber_id"`
	SigningSecret string `json:"signing_secret"`
	ManageToken   string `json:"manage_token"`
	Status        string `json:"status"`
}

func StatusWebhookSignature(secret string, timestamp int64, rawBody []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(rawBody)
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}

func StatusWebhookHeaders(request StatusWebhookRequest) http.Header {
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("X-NewAPI-Event-ID", request.EventID)
	headers.Set("X-NewAPI-Timestamp", strconv.FormatInt(request.Timestamp, 10))
	if request.SigningSecret != "" {
		headers.Set("X-NewAPI-Signature", StatusWebhookSignature(request.SigningSecret, request.Timestamp, request.Body))
	}
	return headers
}

func NewStatusSafeWebhookClient() *StatusSafeWebhookClient {
	resolver := net.DefaultResolver
	netDialer := &net.Dialer{Timeout: statusWebhookConnectTimeout, KeepAlive: 30 * time.Second}
	dialer := statusWebhookDialer{resolver: resolver, dialContext: netDialer.DialContext}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   statusWebhookTLSHandshake,
		ResponseHeaderTimeout: statusWebhookResponseHeader,
		IdleConnTimeout:       30 * time.Second,
		MaxIdleConns:          statusWebhookMaxConcurrency,
		MaxIdleConnsPerHost:   2,
	}
	return &StatusSafeWebhookClient{
		httpClient: &http.Client{
			Transport:     transport,
			Timeout:       statusWebhookTotalTimeout,
			CheckRedirect: statusNoRedirect,
		},
		resolver:         resolver,
		allowedPorts:     defaultStatusWebhookPorts(),
		maxResponseBytes: statusWebhookMaxResponseBytes,
		limiter:          newStatusWebhookLimiter(statusWebhookMaxConcurrency),
	}
}

func (client *StatusSafeWebhookClient) Send(ctx context.Context, request StatusWebhookRequest) (StatusWebhookResponse, error) {
	if client == nil || client.httpClient == nil || client.resolver == nil || client.limiter == nil {
		return StatusWebhookResponse{}, errors.New("status webhook client is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(request.EventID) == "" || request.Timestamp <= 0 {
		return StatusWebhookResponse{}, errors.New("invalid status webhook request")
	}
	if err := ValidateStatusWebhookEndpoint(ctx, request.Endpoint, client.resolver, client.allowedPorts); err != nil {
		return StatusWebhookResponse{}, err
	}
	if err := client.limiter.acquire(ctx); err != nil {
		return StatusWebhookResponse{}, err
	}
	defer client.limiter.release()

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, request.Endpoint, bytes.NewReader(request.Body))
	if err != nil {
		return StatusWebhookResponse{}, err
	}
	httpRequest.Header = StatusWebhookHeaders(request)
	response, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return StatusWebhookResponse{}, err
	}
	body, readErr := readStatusWebhookResponse(response.Body, client.maxResponseBytes)
	if closeErr := response.Body.Close(); readErr == nil && closeErr != nil {
		readErr = closeErr
	}
	if readErr != nil {
		return StatusWebhookResponse{}, readErr
	}
	return StatusWebhookResponse{StatusCode: response.StatusCode, Body: body}, nil
}

func ValidateStatusWebhookEndpoint(ctx context.Context, endpoint string, resolver StatusWebhookResolver, allowedPorts map[int]struct{}) error {
	parsed, _, port, err := parseStatusWebhookEndpoint(endpoint, allowedPorts)
	if err != nil {
		return err
	}
	host := parsed.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if !statusWebhookIPIsPublic(ip) {
			return errors.New("status webhook endpoint resolves to a non-public address")
		}
		return nil
	}
	if resolver == nil {
		return errors.New("status webhook resolver is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	resolveContext, cancel := context.WithTimeout(ctx, statusWebhookDNSLookupTimeout)
	defer cancel()
	addresses, err := resolver.LookupIPAddr(resolveContext, host)
	if err != nil {
		return fmt.Errorf("resolve status webhook endpoint: %w", err)
	}
	if err := validateStatusWebhookAddresses(addresses); err != nil {
		return err
	}
	_ = port
	return nil
}

func (dialer statusWebhookDialer) DialContext(ctx context.Context, network string, address string) (net.Conn, error) {
	if dialer.resolver == nil || dialer.dialContext == nil {
		return nil, errors.New("status webhook dialer is not configured")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	resolveContext, cancel := context.WithTimeout(ctx, statusWebhookDNSLookupTimeout)
	defer cancel()
	addresses, err := dialer.resolver.LookupIPAddr(resolveContext, host)
	if err != nil {
		return nil, fmt.Errorf("resolve status webhook endpoint at dial time: %w", err)
	}
	if err := validateStatusWebhookAddresses(addresses); err != nil {
		return nil, err
	}
	return dialer.dialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
}

func (service StatusWebhookRegistrationService) Register(ctx context.Context, endpoint string, componentIDs []int64) (StatusWebhookRegistrationResult, error) {
	if service.Keyring == nil || !service.Keyring.Enabled() {
		return StatusWebhookRegistrationResult{}, ErrStatusSecretKeyringDisabled
	}
	if service.Sender == nil {
		return StatusWebhookRegistrationResult{}, errors.New("status webhook sender is not configured")
	}
	normalizedEndpoint, err := normalizeStatusWebhookEndpoint(endpoint)
	if err != nil {
		return StatusWebhookRegistrationResult{}, err
	}
	now := time.Now().Unix()
	if service.Now != nil {
		now = service.Now()
	}
	if now <= 0 {
		return StatusWebhookRegistrationResult{}, errors.New("invalid status webhook registration time")
	}
	signingSecret, err := GenerateStatusToken()
	if err != nil {
		return StatusWebhookRegistrationResult{}, err
	}
	manageToken, err := GenerateStatusToken()
	if err != nil {
		return StatusWebhookRegistrationResult{}, err
	}
	challenge, err := GenerateStatusToken()
	if err != nil {
		return StatusWebhookRegistrationResult{}, err
	}
	encryptedEndpoint, err := service.Keyring.Encrypt(normalizedEndpoint)
	if err != nil {
		return StatusWebhookRegistrationResult{}, err
	}
	encryptedSecret, err := service.Keyring.Encrypt(signingSecret)
	if err != nil {
		return StatusWebhookRegistrationResult{}, err
	}
	subscriber, _, err := model.CreateOrRefreshStatusSubscriber(model.StatusSubscriberMutation{
		Subscriber: model.StatusSubscriber{
			Kind:                   model.StatusSubscriberKindWebhook,
			IdentityHash:           HashStatusIdentity(model.StatusSubscriberKindWebhook, normalizedEndpoint),
			EncryptedEndpoint:      encryptedEndpoint,
			EncryptedSigningSecret: encryptedSecret,
			Status:                 model.StatusSubscriberPending,
			ManageTokenHash:        HashStatusToken(manageToken),
			CreatedAt:              now,
			UpdatedAt:              now,
		},
		ComponentIDs: componentIDs,
	})
	if err != nil {
		return StatusWebhookRegistrationResult{}, err
	}
	body, err := common.Marshal(struct {
		Type      string `json:"type"`
		Challenge string `json:"challenge"`
	}{Type: statusWebhookChallengeBodyType, Challenge: challenge})
	if err != nil {
		return StatusWebhookRegistrationResult{}, err
	}
	response, err := service.Sender.Send(ctx, StatusWebhookRequest{
		Endpoint: normalizedEndpoint, EventID: statusWebhookChallengeEventID, Timestamp: now, Body: body, SigningSecret: signingSecret,
	})
	if err != nil || response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return StatusWebhookRegistrationResult{}, ErrStatusWebhookChallengeFailed
	}
	var echo struct {
		Challenge string `json:"challenge"`
	}
	if err := common.Unmarshal(response.Body, &echo); err != nil || !VerifyStatusToken(HashStatusToken(challenge), echo.Challenge) {
		return StatusWebhookRegistrationResult{}, ErrStatusWebhookChallengeFailed
	}
	activated, err := model.ActivateStatusSubscriberChallenge(subscriber.ID, now)
	if err != nil {
		return StatusWebhookRegistrationResult{}, err
	}
	if !activated {
		return StatusWebhookRegistrationResult{}, ErrStatusWebhookChallengeFailed
	}
	return StatusWebhookRegistrationResult{
		SubscriberID:  subscriber.ID,
		SigningSecret: signingSecret,
		ManageToken:   manageToken,
		Status:        model.StatusSubscriberActive,
	}, nil
}

func statusNoRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

func readStatusWebhookResponse(body io.ReadCloser, limit int64) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	if limit <= 0 {
		return nil, ErrStatusWebhookResponseTooLarge
	}
	payload, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > limit {
		return nil, ErrStatusWebhookResponseTooLarge
	}
	return payload, nil
}

func newStatusWebhookLimiter(limit int) *statusWebhookLimiter {
	if limit <= 0 {
		limit = 1
	}
	return &statusWebhookLimiter{semaphore: make(chan struct{}, limit)}
}

func (limiter *statusWebhookLimiter) acquire(ctx context.Context) error {
	select {
	case limiter.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (limiter *statusWebhookLimiter) release() {
	<-limiter.semaphore
}

func statusWebhookIPIsPublic(ip net.IP) bool {
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() {
		return false
	}
	for _, prefix := range statusWebhookDeniedPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

func validateStatusWebhookAddresses(addresses []net.IPAddr) error {
	if len(addresses) == 0 {
		return errors.New("status webhook endpoint did not resolve")
	}
	for _, address := range addresses {
		if !statusWebhookIPIsPublic(address.IP) {
			return errors.New("status webhook endpoint resolves to a non-public address")
		}
	}
	return nil
}

func parseStatusWebhookEndpoint(endpoint string, allowedPorts map[int]struct{}) (*url.URL, string, int, error) {
	endpoint = strings.TrimSpace(endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Hostname() == "" {
		return nil, "", 0, errors.New("status webhook endpoint must be an absolute HTTPS URL")
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return nil, "", 0, errors.New("status webhook endpoint must not contain credentials or a fragment")
	}
	port := 443
	if parsed.Port() != "" {
		port, err = strconv.Atoi(parsed.Port())
		if err != nil || port < 1 || port > 65535 {
			return nil, "", 0, errors.New("status webhook endpoint has an invalid port")
		}
	}
	if allowedPorts == nil {
		allowedPorts = defaultStatusWebhookPorts()
	}
	if _, ok := allowedPorts[port]; !ok {
		return nil, "", 0, errors.New("status webhook endpoint port is not permitted")
	}
	return parsed, parsed.Hostname(), port, nil
}

func normalizeStatusWebhookEndpoint(endpoint string) (string, error) {
	parsed, _, _, err := parseStatusWebhookEndpoint(endpoint, nil)
	if err != nil {
		return "", err
	}
	parsed.Scheme = "https"
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed.String(), nil
}

func defaultStatusWebhookPorts() map[int]struct{} {
	return map[int]struct{}{443: {}, 8443: {}}
}

func mustStatusWebhookPrefixes(values ...string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefixes = append(prefixes, netip.MustParsePrefix(value))
	}
	return prefixes
}
