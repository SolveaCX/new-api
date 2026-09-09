package controller

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v86"
)

func TestStripeInvoicePaidPreviouslyRejectedCurrencyCanBeReplayedOnce(t *testing.T) {
	setupStripeFulfillmentTestDB(t)
	originalReconcile := reconcilePaidStripeInvoice
	t.Cleanup(func() { reconcilePaidStripeInvoice = originalReconcile })
	calls := 0
	reconcilePaidStripeInvoice = func(ctx context.Context, invoiceID string) (*service.PaidInvoiceReconcileResult, error) {
		calls++
		require.Equal(t, "in_adaptive_recovery", invoiceID)
		return &service.PaidInvoiceReconcileResult{}, nil
	}

	// Permanent validation errors were acknowledged with HTTP 200, but the
	// journal retained failed status. Replaying the original event after the
	// validator is fixed must reclaim it, not discard it as already handled.
	journal := &model.PaymentWebhookEvent{
		Provider:         model.PaymentProviderStripe,
		EventId:          "evt_adaptive_recovery",
		EventType:        string(stripe.EventTypeInvoicePaid),
		ProviderObjectId: "in_adaptive_recovery",
		Status:           model.PaymentWebhookEventStatusFailed,
		AttemptCount:     1,
		LastError:        "Stripe invoice currency mismatch",
	}
	require.NoError(t, model.DB.Create(journal).Error)
	event := stripe.Event{
		ID:   journal.EventId,
		Type: stripe.EventTypeInvoicePaid,
		Data: &stripe.EventData{Object: map[string]interface{}{
			"id": journal.ProviderObjectId,
		}},
	}

	require.NoError(t, handleStripeInvoicePaid(context.Background(), event))
	require.Equal(t, 1, calls)
	var recovered model.PaymentWebhookEvent
	require.NoError(t, model.DB.First(&recovered, journal.Id).Error)
	require.Equal(t, model.PaymentWebhookEventStatusProcessed, recovered.Status)
	require.Equal(t, 2, recovered.AttemptCount)
	require.Empty(t, recovered.LastError)
	require.Empty(t, recovered.ProcessingToken)
	require.Zero(t, recovered.ProcessingUntil)
	require.Positive(t, recovered.ProcessedAt)

	// The now-processed event must be idempotent on subsequent redelivery.
	require.NoError(t, handleStripeInvoicePaid(context.Background(), event))
	require.Equal(t, 1, calls)
	require.NoError(t, model.DB.First(&recovered, journal.Id).Error)
	require.Equal(t, 2, recovered.AttemptCount)
}
