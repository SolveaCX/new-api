package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestStatusWebhookSignatureMatchesExactHeaderVector(t *testing.T) {
	body := []byte(`{"status":"degraded"}`)
	request := StatusWebhookRequest{
		EventID:       "event-123",
		Timestamp:     1_700_000_000,
		Body:          body,
		SigningSecret: "topsecret",
	}
	require.Equal(
		t,
		"v1=ec5deca4f038ee49f232221b7bb6546a8f30241e702e58da4683152734234dd6",
		StatusWebhookSignature(request.SigningSecret, request.Timestamp, request.Body),
	)
	headers := StatusWebhookHeaders(request)
	require.Equal(t, "event-123", headers.Get("X-NewAPI-Event-ID"))
	require.Equal(t, "1700000000", headers.Get("X-NewAPI-Timestamp"))
	require.Equal(t, "v1=ec5deca4f038ee49f232221b7bb6546a8f30241e702e58da4683152734234dd6", headers.Get("X-NewAPI-Signature"))
}

type statusWebhookStaticResolver struct {
	mu        sync.Mutex
	responses [][]net.IPAddr
	calls     int
}

func (resolver *statusWebhookStaticResolver) LookupIPAddr(_ context.Context, _ string) ([]net.IPAddr, error) {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	if len(resolver.responses) == 0 {
		return nil, errors.New("no resolver response")
	}
	index := resolver.calls
	if index >= len(resolver.responses) {
		index = len(resolver.responses) - 1
	}
	resolver.calls++
	return resolver.responses[index], nil
}

func TestStatusWebhookRejectsUnsafeSchemesPortsAndAddresses(t *testing.T) {
	publicResolver := &statusWebhookStaticResolver{responses: [][]net.IPAddr{{{IP: net.ParseIP("8.8.8.8")}}}}
	require.NoError(t, ValidateStatusWebhookEndpoint(context.Background(), "https://example.com/hook", publicResolver, nil))

	rejectedURLs := []string{
		"http://example.com/hook",
		"https://user:pass@example.com/hook",
		"https://example.com:22/hook",
		"https://example.com:65536/hook",
		"https://127.0.0.1/hook",
		"https://[::1]/hook",
		"https://169.254.169.254/latest/meta-data",
		"https://10.0.0.1/hook",
		"https://172.16.0.1/hook",
		"https://192.168.0.1/hook",
		"https://100.64.0.1/hook",
		"https://0.0.0.0/hook",
		"https://224.0.0.1/hook",
		"https://192.0.2.1/hook",
		"https://198.51.100.1/hook",
		"https://203.0.113.1/hook",
		"https://[fe80::1]/hook",
		"https://[fc00::1]/hook",
		"https://[ff02::1]/hook",
		"https://[2001:db8::1]/hook",
	}
	for _, endpoint := range rejectedURLs {
		t.Run(endpoint, func(t *testing.T) {
			resolver := &statusWebhookStaticResolver{responses: [][]net.IPAddr{{{IP: net.ParseIP("8.8.8.8")}}}}
			require.Error(t, ValidateStatusWebhookEndpoint(context.Background(), endpoint, resolver, nil))
		})
	}

	resolver := &statusWebhookStaticResolver{responses: [][]net.IPAddr{{{IP: net.ParseIP("8.8.8.8")}}}}
	require.NoError(t, ValidateStatusWebhookEndpoint(context.Background(), "https://example.com:8443/hook", resolver, map[int]struct{}{8443: {}}))
}

func TestStatusWebhookRejectsEveryResolvedIPWhenAnyAddressIsPrivate(t *testing.T) {
	resolver := &statusWebhookStaticResolver{responses: [][]net.IPAddr{{
		{IP: net.ParseIP("8.8.8.8")},
		{IP: net.ParseIP("10.0.0.4")},
	}}}
	require.Error(t, ValidateStatusWebhookEndpoint(context.Background(), "https://example.com/hook", resolver, nil))
}

func TestStatusWebhookIPv6SiteLocalIsDeniedWithoutBlockingGlobalOrMappedPublic(t *testing.T) {
	require.False(t, statusWebhookIPIsPublic(net.ParseIP("fec0::1")))
	require.True(t, statusWebhookIPIsPublic(net.ParseIP("2606:4700:4700::1111")))
	require.True(t, statusWebhookIPIsPublic(net.ParseIP("::ffff:8.8.8.8")))
	require.False(t, statusWebhookIPIsPublic(net.ParseIP("::ffff:127.0.0.1")))
}

func TestStatusWebhookDialRevalidatesDNSAndBlocksRebinding(t *testing.T) {
	resolver := &statusWebhookStaticResolver{responses: [][]net.IPAddr{
		{{IP: net.ParseIP("8.8.8.8")}},
		{{IP: net.ParseIP("127.0.0.1")}},
	}}
	require.NoError(t, ValidateStatusWebhookEndpoint(context.Background(), "https://example.com/hook", resolver, nil))

	dialCalled := false
	dialer := statusWebhookDialer{
		resolver: resolver,
		dialContext: func(context.Context, string, string) (net.Conn, error) {
			dialCalled = true
			return nil, errors.New("unexpected dial")
		},
	}
	_, err := dialer.DialContext(context.Background(), "tcp", "example.com:443")
	require.Error(t, err)
	require.False(t, dialCalled)
}

func TestStatusWebhookClientForbidsRedirectsAndBoundsTimeoutsResponsesAndConcurrency(t *testing.T) {
	client := NewStatusSafeWebhookClient()
	require.Equal(t, statusWebhookTotalTimeout, client.httpClient.Timeout)
	require.ErrorIs(t, client.httpClient.CheckRedirect(nil, nil), http.ErrUseLastResponse)

	_, err := readStatusWebhookResponse(io.NopCloser(strings.NewReader("123456")), 5)
	require.ErrorIs(t, err, ErrStatusWebhookResponseTooLarge)
	body, err := readStatusWebhookResponse(io.NopCloser(strings.NewReader("12345")), 5)
	require.NoError(t, err)
	require.Equal(t, []byte("12345"), body)

	limiter := newStatusWebhookLimiter(1)
	require.NoError(t, limiter.acquire(context.Background()))
	blockedContext, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, limiter.acquire(blockedContext), context.DeadlineExceeded)
	limiter.release()
	require.NoError(t, limiter.acquire(context.Background()))
	limiter.release()
}

type statusChallengeSender struct {
	echo       bool
	requests   []StatusWebhookRequest
	statusCode int
}

func (sender *statusChallengeSender) Send(_ context.Context, request StatusWebhookRequest) (StatusWebhookResponse, error) {
	sender.requests = append(sender.requests, request)
	if sender.statusCode != 0 && sender.statusCode != http.StatusOK {
		return StatusWebhookResponse{StatusCode: sender.statusCode}, nil
	}
	var challenge struct {
		Challenge string `json:"challenge"`
	}
	if err := common.Unmarshal(request.Body, &challenge); err != nil {
		return StatusWebhookResponse{}, err
	}
	if !sender.echo {
		challenge.Challenge = "wrong"
	}
	body, err := common.Marshal(challenge)
	return StatusWebhookResponse{StatusCode: http.StatusOK, Body: body}, err
}

func TestStatusWebhookRegistrationReturnsSecretOnceAndActivatesOnlyAfterChallenge(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	keyring := statusSecretTestKeyring(t)
	sender := &statusChallengeSender{echo: true}
	service := StatusWebhookRegistrationService{
		Keyring: keyring,
		Sender:  sender,
		Now:     func() int64 { return 2_000 },
	}

	result, err := service.Register(context.Background(), "https://hooks.example.com/status", nil)
	require.NoError(t, err)
	require.NotEmpty(t, result.SigningSecret)
	require.NotEmpty(t, result.ManageToken)
	require.Equal(t, model.StatusSubscriberActive, result.Status)
	require.Len(t, sender.requests, 1)
	require.Equal(t, "status.webhook.challenge", sender.requests[0].EventID)
	require.Equal(t, result.SigningSecret, sender.requests[0].SigningSecret)

	var subscriber model.StatusSubscriber
	require.NoError(t, db.First(&subscriber, result.SubscriberID).Error)
	require.Equal(t, model.StatusSubscriberActive, subscriber.Status)
	require.NotContains(t, subscriber.EncryptedEndpoint, "hooks.example.com")
	require.NotContains(t, subscriber.EncryptedSigningSecret, result.SigningSecret)
	endpoint, err := keyring.Decrypt(subscriber.EncryptedEndpoint)
	require.NoError(t, err)
	require.Equal(t, "https://hooks.example.com/status", endpoint)
	secret, err := keyring.Decrypt(subscriber.EncryptedSigningSecret)
	require.NoError(t, err)
	require.Equal(t, result.SigningSecret, secret)
	require.True(t, VerifyStatusToken(subscriber.ManageTokenHash, result.ManageToken))

	payload, err := common.Marshal(subscriber)
	require.NoError(t, err)
	require.False(t, bytes.Contains(payload, []byte(result.SigningSecret)))
}

func TestStatusWebhookRegistrationLeavesDestinationPendingOnBadChallenge(t *testing.T) {
	db := setupStatusServiceTestDB(t)
	sender := &statusChallengeSender{echo: false}
	service := StatusWebhookRegistrationService{
		Keyring: statusSecretTestKeyring(t),
		Sender:  sender,
		Now:     func() int64 { return 3_000 },
	}

	_, err := service.Register(context.Background(), "https://hooks.example.com/bad", []int64{7})
	require.ErrorIs(t, err, ErrStatusWebhookChallengeFailed)
	var subscriber model.StatusSubscriber
	require.NoError(t, db.Where("identity_hash = ?", HashStatusIdentity(model.StatusSubscriberKindWebhook, "https://hooks.example.com/bad")).First(&subscriber).Error)
	require.Equal(t, model.StatusSubscriberPending, subscriber.Status)
}
