package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	stripe "github.com/stripe/stripe-go/v86"
)

const stripeWalletGatewayRequestTimeout = 15 * time.Second

type stripeWalletGateway interface {
	Scope(ctx context.Context) (scope string, live bool, err error)
	ListEvents(ctx context.Context, from int64, to int64, after string) (stripeWalletEventPage, error)
	GetSession(ctx context.Context, id string) (*stripe.CheckoutSession, error)
}

type stripeWalletEventPage struct {
	Events  []*stripe.Event
	HasMore bool
}

type stripeWalletGatewayClient struct {
	apiKey string
	client *stripe.Client
}

func newStripeWalletGateway(apiKey string) stripeWalletGateway {
	httpClient := GetHttpClient()
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return newStripeWalletGatewayWithBackends(apiKey, stripe.NewBackendsWithConfig(&stripe.BackendConfig{
		HTTPClient:        httpClient,
		LeveledLogger:     &stripe.LeveledLogger{Level: stripe.LevelNull},
		MaxNetworkRetries: stripe.Int64(0),
	}))
}

func newStripeWalletGatewayWithBackends(apiKey string, backends *stripe.Backends) stripeWalletGateway {
	apiKey = strings.TrimSpace(apiKey)
	return &stripeWalletGatewayClient{
		apiKey: apiKey,
		client: stripe.NewClient(apiKey, stripe.WithBackends(backends)),
	}
}

func (g *stripeWalletGatewayClient) Scope(ctx context.Context) (string, bool, error) {
	if g == nil || g.client == nil {
		return "", false, fmt.Errorf("stripe reconciliation configuration invalid: gateway is not initialized")
	}
	live, err := stripeWalletKeyMode(g.apiKey)
	if err != nil {
		return "", false, err
	}
	ctx, cancel := stripeWalletGatewayContext(ctx)
	defer cancel()

	account, err := g.client.V1Accounts.Retrieve(ctx, nil)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", false, fmt.Errorf("stripe reconciliation retrieve account scope failed: %w", ctxErr)
		}
		return "", false, g.safeError("retrieve account scope", err)
	}
	if account == nil || !strings.HasPrefix(strings.TrimSpace(account.ID), "acct_") {
		return "", false, fmt.Errorf("stripe reconciliation configuration invalid: account scope is missing")
	}
	mode := "test"
	if live {
		mode = "live"
	}
	return "stripe:" + strings.TrimSpace(account.ID) + ":" + mode, live, nil
}

func (g *stripeWalletGatewayClient) ListEvents(ctx context.Context, from int64, to int64, after string) (stripeWalletEventPage, error) {
	if g == nil || g.client == nil {
		return stripeWalletEventPage{}, fmt.Errorf("stripe reconciliation configuration invalid: gateway is not initialized")
	}
	if _, err := stripeWalletKeyMode(g.apiKey); err != nil {
		return stripeWalletEventPage{}, err
	}
	if from < 0 || to <= from {
		return stripeWalletEventPage{}, fmt.Errorf("stripe reconciliation event window is invalid")
	}

	params := &stripe.EventListParams{
		ListParams: stripe.ListParams{
			Limit:  stripe.Int64(100),
			Single: true,
		},
		CreatedRange: &stripe.RangeQueryParams{
			GreaterThanOrEqual: from,
			LesserThan:         to,
		},
		Types: []*string{
			stripe.String(string(stripe.EventTypeCheckoutSessionCompleted)),
			stripe.String(string(stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded)),
		},
	}
	if after = strings.TrimSpace(after); after != "" {
		params.StartingAfter = stripe.String(after)
	}

	ctx, cancel := stripeWalletGatewayContext(ctx)
	defer cancel()
	list := g.client.V1Events.List(ctx, params)
	if err := list.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return stripeWalletEventPage{}, fmt.Errorf("stripe reconciliation list payment events failed: %w", ctxErr)
		}
		return stripeWalletEventPage{}, g.safeError("list payment events", err)
	}
	events := append([]*stripe.Event(nil), list.Data()...)
	return stripeWalletEventPage{Events: events, HasMore: list.Meta().HasMore}, nil
}

func (g *stripeWalletGatewayClient) GetSession(ctx context.Context, id string) (*stripe.CheckoutSession, error) {
	if g == nil || g.client == nil {
		return nil, fmt.Errorf("stripe reconciliation configuration invalid: gateway is not initialized")
	}
	if _, err := stripeWalletKeyMode(g.apiKey); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("stripe reconciliation checkout session id is empty")
	}
	params := &stripe.CheckoutSessionRetrieveParams{}
	params.AddExpand("line_items.data.price")

	ctx, cancel := stripeWalletGatewayContext(ctx)
	defer cancel()
	session, err := g.client.V1CheckoutSessions.Retrieve(ctx, id, params)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("stripe reconciliation retrieve checkout session failed: %w", ctxErr)
		}
		return nil, g.safeError("retrieve checkout session", err)
	}
	return session, nil
}

func stripeWalletGatewayContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, stripeWalletGatewayRequestTimeout)
}

func stripeWalletKeyMode(apiKey string) (bool, error) {
	apiKey = strings.TrimSpace(apiKey)
	for _, prefix := range []string{"sk_live_", "rk_live_"} {
		if strings.HasPrefix(apiKey, prefix) && len(apiKey) > len(prefix) {
			return true, nil
		}
	}
	for _, prefix := range []string{"sk_test_", "rk_test_"} {
		if strings.HasPrefix(apiKey, prefix) && len(apiKey) > len(prefix) {
			return false, nil
		}
	}
	return false, fmt.Errorf("stripe reconciliation configuration invalid: unsupported api key type")
}

func (g *stripeWalletGatewayClient) safeError(action string, err error) error {
	message := "unknown error"
	if err != nil {
		message = err.Error()
	}
	if g != nil && g.apiKey != "" {
		message = strings.ReplaceAll(message, g.apiKey, "***")
	}
	message = sanitizeDingTalkAlertText(message)
	return fmt.Errorf("stripe reconciliation %s failed: %s", action, message)
}
