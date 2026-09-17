package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	stripe "github.com/stripe/stripe-go/v86"
)

const stripeWalletGatewayRequestTimeout = 15 * time.Second

type stripeWalletGateway interface {
	Scope(ctx context.Context) (scope string, live bool, err error)
	ListEvents(ctx context.Context, from int64, to int64, after string) (stripeWalletEventPage, error)
	GetSession(ctx context.Context, id string) (*stripe.CheckoutSession, error)
	GetRepairSession(ctx context.Context, id string) (*stripe.CheckoutSession, error)
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

// GetRepairSession verifies that current refund and dispute evidence is complete
// before allowing the caller to validate the order and settle its wallet credit.
func (g *stripeWalletGatewayClient) GetRepairSession(ctx context.Context, id string) (*stripe.CheckoutSession, error) {
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
	params.AddExpand("payment_intent.latest_charge")

	ctx, cancel := stripeWalletGatewayContext(ctx)
	defer cancel()
	session, err := g.client.V1CheckoutSessions.Retrieve(ctx, id, params)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("stripe reconciliation retrieve repair checkout session failed: %w", ctxErr)
		}
		return nil, g.safeError("retrieve repair checkout session", err)
	}
	if err := stripeWalletRepairFinancialEvidence(session); err != nil {
		return nil, err
	}

	refunds := g.client.V1Refunds.List(ctx, &stripe.RefundListParams{
		ListParams: stripe.ListParams{Limit: stripe.Int64(1), Single: true},
		Charge:     stripe.String(session.PaymentIntent.LatestCharge.ID),
	})
	if err := refunds.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("stripe reconciliation list repair refunds failed: %w", ctxErr)
		}
		return nil, g.safeError("list repair refunds", err)
	}
	response := refunds.LastResponse()
	if response == nil || len(response.RawJSON) == 0 {
		return nil, fmt.Errorf("stripe reconciliation repair refund evidence is missing")
	}
	var evidence struct {
		Object  string `json:"object"`
		Data    *[]any `json:"data"`
		HasMore *bool  `json:"has_more"`
	}
	if err := common.Unmarshal(response.RawJSON, &evidence); err != nil || evidence.Object != "list" || evidence.Data == nil || evidence.HasMore == nil {
		return nil, fmt.Errorf("stripe reconciliation repair refund evidence is incomplete")
	}
	// Any refund history is excluded, including pending, canceled or failed
	// refunds. An automatic wallet repair must not override a refund decision.
	if len(*evidence.Data) != 0 || *evidence.HasMore || len(refunds.Data()) != 0 || refunds.Meta().HasMore {
		return nil, fmt.Errorf("stripe reconciliation repair blocked by refund history")
	}
	return session, nil
}

func stripeWalletRepairFinancialEvidence(session *stripe.CheckoutSession) error {
	if session == nil || session.LastResponse == nil || len(session.LastResponse.RawJSON) == 0 {
		return fmt.Errorf("stripe reconciliation repair payment evidence is missing")
	}
	// Stripe's SDK uses scalar zero values for these fields. Decode pointers
	// from the live response so absent/null refund or dispute flags fail closed.
	var evidence struct {
		PaymentIntent *struct {
			ID           string `json:"id"`
			Object       string `json:"object"`
			LatestCharge *struct {
				ID             string `json:"id"`
				Object         string `json:"object"`
				AmountRefunded *int64 `json:"amount_refunded"`
				Refunded       *bool  `json:"refunded"`
				Disputed       *bool  `json:"disputed"`
			} `json:"latest_charge"`
		} `json:"payment_intent"`
	}
	if err := common.Unmarshal(session.LastResponse.RawJSON, &evidence); err != nil {
		return fmt.Errorf("stripe reconciliation repair payment evidence is malformed")
	}
	intent := evidence.PaymentIntent
	if intent == nil || intent.ID == "" || intent.Object != "payment_intent" || intent.LatestCharge == nil ||
		session.PaymentIntent == nil || session.PaymentIntent.LatestCharge == nil {
		return fmt.Errorf("stripe reconciliation repair payment evidence is incomplete")
	}
	charge := intent.LatestCharge
	if charge.ID == "" || charge.Object != "charge" || charge.AmountRefunded == nil || charge.Refunded == nil || charge.Disputed == nil {
		return fmt.Errorf("stripe reconciliation repair charge evidence is incomplete")
	}
	if *charge.AmountRefunded != 0 || *charge.Refunded || *charge.Disputed {
		return fmt.Errorf("stripe reconciliation repair blocked by refund or dispute")
	}
	return nil
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
