package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	stripe "github.com/stripe/stripe-go/v86"
)

func TestStripeWalletGatewayUsesReadOnlyScopedRequests(t *testing.T) {
	t.Parallel()
	const apiKey = "rk_test_reconciliation_secret"
	var mu sync.Mutex
	requests := make([]*http.Request, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Clone(r.Context()))
		mu.Unlock()
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "Bearer "+apiKey, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/account":
			_, _ = w.Write([]byte(`{"id":"acct_stable","object":"account"}`))
		case "/v1/events":
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"evt_1","object":"event","created":1700000001,"livemode":false,"type":"checkout.session.completed","data":{"object":{"id":"cs_1","object":"checkout.session"}}}],"has_more":true,"url":"/v1/events"}`))
		case "/v1/checkout/sessions/cs_1":
			_, _ = w.Write([]byte(`{"id":"cs_1","object":"checkout.session","payment_status":"paid","line_items":{"object":"list","data":[],"has_more":true,"url":"/v1/checkout/sessions/cs_1/line_items"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	gateway := newStripeWalletGatewayWithBackends(apiKey, stripeWalletTestBackends(server))
	scope, live, err := gateway.Scope(context.Background())
	require.NoError(t, err)
	require.Equal(t, "stripe:acct_stable:test", scope)
	require.False(t, live)

	page, err := gateway.ListEvents(context.Background(), 1700000000, 1700000100, "evt_previous")
	require.NoError(t, err)
	require.True(t, page.HasMore)
	require.Len(t, page.Events, 1)
	require.Equal(t, "evt_1", page.Events[0].ID)
	require.JSONEq(t, `{"id":"cs_1","object":"checkout.session"}`, string(page.Events[0].Data.Raw))

	session, err := gateway.GetSession(context.Background(), "cs_1")
	require.NoError(t, err)
	require.NotNil(t, session.LineItems)
	require.True(t, session.LineItems.HasMore)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, requests, 3)
	query := requests[1].URL.Query()
	require.Equal(t, "100", query.Get("limit"))
	require.Equal(t, "evt_previous", query.Get("starting_after"))
	require.Equal(t, "1700000000", query.Get("created[gte]"))
	require.Equal(t, "1700000100", query.Get("created[lt]"))
	require.ElementsMatch(t, []string{"checkout.session.completed", "checkout.session.async_payment_succeeded"}, stripeWalletTestQueryArray(query, "types"))
	require.Empty(t, query.Get("delivery_success"))
	require.Equal(t, []string{"line_items.data.price"}, stripeWalletTestQueryArray(requests[2].URL.Query(), "expand"))
}

func TestStripeWalletGatewayRejectsUnsupportedKeyWithoutRequest(t *testing.T) {
	t.Parallel()
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer server.Close()
	gateway := newStripeWalletGatewayWithBackends("whsec_not_an_api_key", stripeWalletTestBackends(server))

	_, _, err := gateway.Scope(context.Background())
	require.ErrorContains(t, err, "unsupported api key type")
	require.Equal(t, 0, requests)
}

func TestStripeWalletGatewayHonorsContextAndRedactsKey(t *testing.T) {
	t.Parallel()
	const apiKey = "sk_live_do_not_leak_this"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/account" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprintf(w, `{"error":{"message":"rejected %s","type":"invalid_request_error"}}`, apiKey)
			return
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	gateway := newStripeWalletGatewayWithBackends(apiKey, stripeWalletTestBackends(server))

	_, _, err := gateway.Scope(context.Background())
	require.Error(t, err)
	require.NotContains(t, err.Error(), apiKey)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = gateway.GetSession(ctx, "cs_cancelled")
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func stripeWalletTestBackends(server *httptest.Server) *stripe.Backends {
	baseURL, err := url.Parse(server.URL)
	if err != nil {
		panic(err)
	}
	urlString := strings.TrimRight(baseURL.String(), "/")
	return stripe.NewBackendsWithConfig(&stripe.BackendConfig{
		HTTPClient:        server.Client(),
		LeveledLogger:     &stripe.LeveledLogger{Level: stripe.LevelNull},
		MaxNetworkRetries: stripe.Int64(0),
		URL:               stripe.String(urlString),
	})
}

func stripeWalletTestQueryArray(query url.Values, name string) []string {
	var values []string
	for key, current := range query {
		if key == name+"[]" || strings.HasPrefix(key, name+"[") {
			values = append(values, current...)
		}
	}
	return values
}

func TestStripeWalletGatewayContextKeepsEarlierDeadline(t *testing.T) {
	t.Parallel()
	deadline := time.Now().Add(100 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	derived, derivedCancel := stripeWalletGatewayContext(ctx)
	defer derivedCancel()
	actual, ok := derived.Deadline()
	require.True(t, ok)
	require.WithinDuration(t, deadline, actual, 10*time.Millisecond)
}
