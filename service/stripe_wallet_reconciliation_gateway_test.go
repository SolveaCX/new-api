package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
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

const stripeWalletRepairSessionJSON = `{"id":"cs_repair","object":"checkout.session","mode":"payment","status":"complete","payment_status":"paid","livemode":false,"currency":"usd","amount_total":1000,"line_items":{"object":"list","data":[{"id":"li_repair","object":"item","quantity":10,"price":{"id":"price_wallet","object":"price","currency":"usd","unit_amount":100}}],"has_more":false},"payment_intent":{"id":"pi_repair","object":"payment_intent","status":"succeeded","livemode":false,"currency":"usd","amount":1000,"amount_received":1000,"latest_charge":{"id":"ch_repair","object":"charge","payment_intent":"pi_repair","status":"succeeded","livemode":false,"currency":"usd","amount":1000,"amount_captured":1000,"paid":true,"captured":true,"amount_refunded":0,"refunded":false,"disputed":false}}}`

const stripeWalletNoRefundsJSON = `{"object":"list","data":[],"has_more":false,"url":"/v1/refunds"}`

func TestStripeWalletGatewayRepairUsesExpandedReadOnlyRequests(t *testing.T) {
	t.Parallel()
	const apiKey = "rk_test_repair_secret"
	var mu sync.Mutex
	var requests []*http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Clone(r.Context()))
		mu.Unlock()
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "Bearer "+apiKey, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/checkout/sessions/cs_repair":
			_, _ = w.Write([]byte(stripeWalletRepairSessionJSON))
		case "/v1/refunds":
			_, _ = w.Write([]byte(stripeWalletNoRefundsJSON))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := newStripeWalletGatewayWithBackends(apiKey, stripeWalletTestBackends(server))

	session, err := client.GetRepairSession(context.Background(), " cs_repair ")
	require.NoError(t, err)
	require.Equal(t, "cs_repair", session.ID)
	require.Equal(t, "price_wallet", session.LineItems.Data[0].Price.ID)
	require.Equal(t, "ch_repair", session.PaymentIntent.LatestCharge.ID)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, requests, 2)
	require.ElementsMatch(t, []string{"line_items.data.price", "payment_intent.latest_charge"}, stripeWalletTestQueryArray(requests[0].URL.Query(), "expand"))
	require.Equal(t, url.Values{"charge": {"ch_repair"}, "limit": {"1"}}, requests[1].URL.Query())
}

func TestStripeWalletGatewayRepairRejectsUnsafePaymentEvidence(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		old  string
		new  string
	}{
		{"partial refund", `"amount_refunded":0`, `"amount_refunded":1`},
		{"negative refund amount", `"amount_refunded":0`, `"amount_refunded":-1`},
		{"full refund", `"refunded":false`, `"refunded":true`},
		{"disputed", `"disputed":false`, `"disputed":true`},
		{"missing refund amount", `"amount_refunded":0,`, ``},
		{"null refund amount", `"amount_refunded":0`, `"amount_refunded":null`},
		{"missing refunded", `"refunded":false,`, ``},
		{"null refunded", `"refunded":false`, `"refunded":null`},
		{"missing disputed", `,"disputed":false`, ``},
		{"null disputed", `"disputed":false`, `"disputed":null`},
		{"invalid disputed type", `"disputed":false`, `"disputed":"false"`},
		{"missing charge id", `"id":"ch_repair",`, ``},
		{"missing charge object", `"object":"charge",`, ``},
		{"missing intent id", `"id":"pi_repair",`, ``},
		{"missing intent object", `"object":"payment_intent",`, ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "/v1/checkout/sessions/cs_repair", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(strings.Replace(stripeWalletRepairSessionJSON, tc.old, tc.new, 1)))
			}))
			defer server.Close()
			client := newStripeWalletGatewayWithBackends("rk_test_repair_secret", stripeWalletTestBackends(server))
			session, err := client.GetRepairSession(context.Background(), "cs_repair")
			require.Error(t, err)
			require.Nil(t, session)
			require.EqualValues(t, 1, requests.Load(), "unsafe payment evidence must not reach the refunds request")
		})
	}
}

func TestStripeWalletGatewayRepairRejectsMissingExpandedPayment(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"missing intent":    `{"id":"cs_repair","object":"checkout.session"}`,
		"null intent":       `{"id":"cs_repair","payment_intent":null}`,
		"unexpanded intent": `{"id":"cs_repair","payment_intent":"pi_repair"}`,
		"missing charge":    `{"id":"cs_repair","payment_intent":{"id":"pi_repair","object":"payment_intent"}}`,
		"null charge":       `{"id":"cs_repair","payment_intent":{"id":"pi_repair","object":"payment_intent","latest_charge":null}}`,
		"unexpanded charge": `{"id":"cs_repair","payment_intent":{"id":"pi_repair","object":"payment_intent","latest_charge":"ch_repair"}}`,
		"malformed":         `{"id":"cs_repair"`,
	} {
		t.Run(name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			client := newStripeWalletGatewayWithBackends("rk_test_repair_secret", stripeWalletTestBackends(server))
			session, err := client.GetRepairSession(context.Background(), "cs_repair")
			require.Error(t, err)
			require.Nil(t, session)
			require.EqualValues(t, 1, requests.Load())
		})
	}
	for _, session := range []*stripe.CheckoutSession{nil, {}, {APIResource: stripe.APIResource{LastResponse: &stripe.APIResponse{}}}} {
		require.ErrorContains(t, stripeWalletRepairFinancialEvidence(session), "evidence is missing")
	}
}

func TestStripeWalletGatewayRepairRejectsRefundHistoryAndIncompleteLists(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"missing data":         `{"object":"list","has_more":false}`,
		"null data":            `{"object":"list","data":null,"has_more":false}`,
		"missing has more":     `{"object":"list","data":[]}`,
		"null has more":        `{"object":"list","data":[],"has_more":null}`,
		"missing object":       `{"data":[],"has_more":false}`,
		"has more":             `{"object":"list","data":[],"has_more":true}`,
		"refund page has more": `{"object":"list","data":[{"id":"re_repair","object":"refund","status":"pending"}],"has_more":true}`,
		"malformed data":       `{"object":"list","data":{},"has_more":false}`,
	}
	for _, status := range []string{"succeeded", "pending", "requires_action", "failed", "canceled"} {
		cases[status] = fmt.Sprintf(`{"object":"list","data":[{"id":"re_repair","object":"refund","charge":"ch_repair","status":%q}],"has_more":false}`, status)
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				require.Equal(t, http.MethodGet, r.Method)
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/v1/checkout/sessions/cs_repair" {
					_, _ = w.Write([]byte(stripeWalletRepairSessionJSON))
					return
				}
				require.Equal(t, "/v1/refunds", r.URL.Path)
				require.Equal(t, "ch_repair", r.URL.Query().Get("charge"))
				require.Equal(t, "1", r.URL.Query().Get("limit"))
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			client := newStripeWalletGatewayWithBackends("rk_test_repair_secret", stripeWalletTestBackends(server))
			session, err := client.GetRepairSession(context.Background(), "cs_repair")
			require.Error(t, err)
			require.Nil(t, session)
			require.EqualValues(t, 2, requests.Load(), "refund inspection must not paginate")
		})
	}
}

func TestStripeWalletGatewayRepairRedactsErrorsAndHonorsContext(t *testing.T) {
	t.Parallel()
	const apiKey = "sk_live_repair_do_not_leak"
	for _, failAt := range []string{"session", "refunds"} {
		t.Run(failAt, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if failAt == "refunds" && r.URL.Path != "/v1/refunds" {
					_, _ = w.Write([]byte(stripeWalletRepairSessionJSON))
					return
				}
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = fmt.Fprintf(w, `{"error":{"message":"rejected %s","type":"invalid_request_error"}}`, apiKey)
			}))
			defer server.Close()
			client := newStripeWalletGatewayWithBackends(apiKey, stripeWalletTestBackends(server))
			_, err := client.GetRepairSession(context.Background(), "cs_repair")
			require.Error(t, err)
			require.NotContains(t, err.Error(), apiKey)
		})
	}
	t.Run("canceled before request", func(t *testing.T) {
		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
		defer server.Close()
		client := newStripeWalletGatewayWithBackends(apiKey, stripeWalletTestBackends(server))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := client.GetRepairSession(ctx, "cs_repair")
		require.ErrorIs(t, err, context.Canceled)
		require.Zero(t, requests.Load())
	})
	t.Run("deadline during refunds", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1/refunds" {
				<-r.Context().Done()
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(stripeWalletRepairSessionJSON))
		}))
		defer server.Close()
		client := newStripeWalletGatewayWithBackends(apiKey, stripeWalletTestBackends(server))
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		_, err := client.GetRepairSession(ctx, "cs_repair")
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}
