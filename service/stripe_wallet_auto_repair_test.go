package service

import (
	"context"
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v86"
)

func stripeWalletRepairSession(id, trade string) *stripe.CheckoutSession {
	s := stripeWalletPaidSession(id, trade, "price_wallet", true)
	s.Status = stripe.CheckoutSessionStatusComplete
	s.Customer = &stripe.Customer{ID: "cus_wallet"}
	s.PaymentIntent = &stripe.PaymentIntent{
		ID: "pi_wallet", Status: stripe.PaymentIntentStatusSucceeded, Amount: 1000, AmountReceived: 1000,
		Currency: stripe.CurrencyUSD, Customer: s.Customer,
		LatestCharge: &stripe.Charge{
			ID: "ch_wallet", Status: stripe.ChargeStatusSucceeded, Paid: true, Captured: true,
			Amount: 1000, AmountCaptured: 1000, Currency: stripe.CurrencyUSD,
			PaymentIntent: &stripe.PaymentIntent{ID: "pi_wallet"}, Customer: s.Customer,
		},
	}
	return s
}

func stripeWalletRepairCheck(h *stripeWalletHarness, sessionID, trade string) model.StripeWalletPaymentCheck {
	return model.StripeWalletPaymentCheck{Scope: h.gateway.scope, SessionID: sessionID, TradeNo: trade,
		UserID: 7, Amount: 1000, Currency: "USD", PriceID: "price_wallet", WalletVerified: true,
		SessionVerified: true, PaidAt: h.now, NextCheckAt: h.now}
}

// Model tests exercise the real wallet transaction. This seam isolates task
// scheduling, crash recovery and notification behavior from financial internals.
func fakeCommittedStripeWalletRepair(h *stripeWalletHarness, calls *int) func(context.Context, *model.StripeWalletPaymentCheck, model.StripeWalletRepairPayment) (bool, error) {
	return func(ctx context.Context, check *model.StripeWalletPaymentCheck, proof model.StripeWalletRepairPayment) (bool, error) {
		*calls++
		require.Equal(h.t, check.TradeNo, proof.TradeNo)
		require.Equal(h.t, int64(1000), proof.AmountMinor)
		check.AutoRepairedAt, check.RepairFromStatus, check.RepairCredit = h.now, check.LocalStatus, 5000000
		require.NoError(h.t, h.db.Model(&model.TopUp{}).Where("trade_no = ?", check.TradeNo).Update("status", common.TopUpStatusSuccess).Error)
		require.NoError(h.t, h.db.Model(&model.StripeWalletPaymentCheck{}).Where("id = ?", check.Id).Updates(map[string]any{
			"auto_repaired_at": check.AutoRepairedAt, "repair_from_status": check.RepairFromStatus, "repair_credit": check.RepairCredit,
		}).Error)
		return true, nil
	}
}

func TestStripeWalletAutoRepairImmediateAfterPaidEvent(t *testing.T) {
	for _, status := range []string{common.TopUpStatusPending, common.TopUpStatusFailed, common.TopUpStatusExpired, "cancelled", "canceled"} {
		t.Run(status, func(t *testing.T) {
			h := newStripeWalletHarness(t)
			trade, id := stripeWalletTrade(70), "cs_immediate"
			h.createTopUp(trade, id, status, 10)
			s := stripeWalletRepairSession(id, trade)
			h.gateway.sessions[id] = s
			r := h.reconciler()
			calls := 0
			r.repair = fakeCommittedStripeWalletRepair(h, &calls)
			require.NoError(t, r.ingest(context.Background(), h.gateway.scope, false, stripeWalletEvent(t, "evt_fresh", h.now, s), false))
			require.Equal(t, h.now, h.loadCheck(id).NextCheckAt)
			count, err := r.processChecks(context.Background(), h.gateway.scope)
			require.NoError(t, err)
			require.Equal(t, 1, count)
			require.Equal(t, 1, calls)
			require.Equal(t, 1, h.gateway.repairGets)
			require.Len(t, h.notices, 1)
			require.Equal(t, "auto_repaired", h.notices[0].Kind)
			require.Equal(t, status, h.notices[0].RepairFromStatus)
			require.Equal(t, common.TopUpStatusSuccess, h.notices[0].LocalStatus)
			require.Equal(t, h.now, h.loadCheck(id).ClosedAt)
		})
	}
}

func TestStripeWalletAutoRepairNotificationRetrySurvivesRestartWithoutRecredit(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, id := stripeWalletTrade(71), "cs_notify_retry"
	h.createTopUp(trade, id, common.TopUpStatusFailed, 10)
	h.gateway.sessions[id] = stripeWalletRepairSession(id, trade)
	h.insertCheck(stripeWalletRepairCheck(h, id, trade))
	calls := 0
	r := h.reconciler()
	r.repair = fakeCommittedStripeWalletRepair(h, &calls)
	h.notify = func(context.Context, stripeWalletNotification) error { return errors.New("DingTalk unavailable") }
	check, token := h.claim(id)
	require.ErrorContains(t, r.processCheck(context.Background(), &check, token), "DingTalk unavailable")
	require.Equal(t, 1, calls)
	require.NotZero(t, h.loadCheck(id).AutoRepairedAt)
	require.Zero(t, h.loadCheck(id).ClosedAt)
	h.notify = func(_ context.Context, notice stripeWalletNotification) error {
		h.notices = append(h.notices, notice)
		return nil
	}
	h.gateway.repairErr = errors.New("Stripe unavailable after restart")
	h.now += stripeWalletNotifyRetry + 1
	check, token = h.claim(id)
	require.NoError(t, h.reconciler().processCheck(context.Background(), &check, token))
	require.Equal(t, 1, h.gateway.repairGets)
	require.Len(t, h.notices, 1)
	require.Equal(t, "auto_repaired", h.notices[0].Kind)
	require.NotZero(t, h.loadCheck(id).ClosedAt)
}

func TestStripeWalletAutoRepairFailureAlertsAndRetries(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, id := stripeWalletTrade(72), "cs_repair_db_error"
	h.createTopUp(trade, id, common.TopUpStatusPending, 10)
	h.gateway.sessions[id] = stripeWalletRepairSession(id, trade)
	h.insertCheck(stripeWalletRepairCheck(h, id, trade))
	r := h.reconciler()
	r.repair = func(context.Context, *model.StripeWalletPaymentCheck, model.StripeWalletRepairPayment) (bool, error) {
		return false, errors.New("atomic credit rolled back")
	}
	check, token := h.claim(id)
	require.NoError(t, r.processCheck(context.Background(), &check, token))
	require.Equal(t, "initial", h.notices[0].Kind)
	require.Contains(t, h.notices[0].Detail, "atomic credit rolled back")
	require.Zero(t, h.loadCheck(id).AutoRepairedAt)
	h.now += 61
	calls := 0
	r.repair = fakeCommittedStripeWalletRepair(h, &calls)
	check, token = h.claim(id)
	require.NoError(t, r.processCheck(context.Background(), &check, token))
	require.Equal(t, 1, calls)
	require.Len(t, h.notices, 2)
	require.Equal(t, "auto_repaired", h.notices[1].Kind)
	require.Empty(t, h.loadCheck(id).RepairError)
}

func TestStripeWalletAutoRepairLostCommitAcknowledgementStillSendsRepairAudit(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, id := stripeWalletTrade(75), "cs_commit_ack_lost"
	h.createTopUp(trade, id, common.TopUpStatusFailed, 10)
	h.gateway.sessions[id] = stripeWalletRepairSession(id, trade)
	h.insertCheck(stripeWalletRepairCheck(h, id, trade))
	r := h.reconciler()
	calls := 0
	r.repair = func(ctx context.Context, check *model.StripeWalletPaymentCheck, proof model.StripeWalletRepairPayment) (bool, error) {
		// Simulate server-side commit with no result delivered to this worker.
		committed := *check
		_, err := fakeCommittedStripeWalletRepair(h, &calls)(ctx, &committed, proof)
		require.NoError(t, err)
		return false, errors.New("database commit acknowledgement lost")
	}
	check, token := h.claim(id)
	require.NoError(t, r.processCheck(context.Background(), &check, token))
	require.Equal(t, 1, calls)
	require.Len(t, h.notices, 1)
	require.Equal(t, "auto_repaired", h.notices[0].Kind)
	require.EqualValues(t, 5000000, h.notices[0].RepairCredit)
	require.Empty(t, h.notices[0].Detail)
	require.Positive(t, h.loadCheck(id).AutoRepairedAt)
	require.Positive(t, h.loadCheck(id).ClosedAt)
}

func TestStripeWalletAutoRepairNeverUsesCachedEvidenceDuringAccountOutage(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, id := stripeWalletTrade(73), "cs_local_only"
	h.createTopUp(trade, id, common.TopUpStatusFailed, 10)
	h.gateway.sessions[id] = stripeWalletRepairSession(id, trade)
	h.insertCheck(stripeWalletRepairCheck(h, id, trade))
	r := h.reconciler()
	r.localOnly = true
	r.repair = func(context.Context, *model.StripeWalletPaymentCheck, model.StripeWalletRepairPayment) (bool, error) {
		t.Fatal("must not repair during account outage")
		return false, nil
	}
	check, token := h.claim(id)
	require.NoError(t, r.processCheck(context.Background(), &check, token))
	require.Zero(t, h.gateway.repairGets)
	require.Equal(t, "initial", h.notices[0].Kind)
}

func TestStripeWalletAutoRepairRejectsUnsafePaymentEvidence(t *testing.T) {
	for name, mutate := range map[string]func(*stripe.CheckoutSession){
		"unpaid":                   func(s *stripe.CheckoutSession) { s.PaymentStatus = stripe.CheckoutSessionPaymentStatusUnpaid },
		"not complete":             func(s *stripe.CheckoutSession) { s.Status = stripe.CheckoutSessionStatusOpen },
		"subscription":             func(s *stripe.CheckoutSession) { s.Mode = stripe.CheckoutSessionModeSubscription },
		"wrong mode":               func(s *stripe.CheckoutSession) { s.Livemode = true },
		"different session":        func(s *stripe.CheckoutSession) { s.ID = "cs_other" },
		"different reference":      func(s *stripe.CheckoutSession) { s.ClientReferenceID = stripeWalletTrade(99) },
		"event amount mismatch":    func(s *stripe.CheckoutSession) { s.AmountTotal = 999 },
		"missing line":             func(s *stripe.CheckoutSession) { s.LineItems = nil },
		"hidden lines":             func(s *stripe.CheckoutSession) { s.LineItems.HasMore = true },
		"multiple quantity":        func(s *stripe.CheckoutSession) { s.LineItems.Data[0].Quantity = 2 },
		"missing intent":           func(s *stripe.CheckoutSession) { s.PaymentIntent = nil },
		"intent incomplete":        func(s *stripe.CheckoutSession) { s.PaymentIntent.Status = stripe.PaymentIntentStatusProcessing },
		"wrong intent amount":      func(s *stripe.CheckoutSession) { s.PaymentIntent.AmountReceived-- },
		"wrong intent currency":    func(s *stripe.CheckoutSession) { s.PaymentIntent.Currency = stripe.CurrencyCNY },
		"missing charge":           func(s *stripe.CheckoutSession) { s.PaymentIntent.LatestCharge = nil },
		"partial refund":           func(s *stripe.CheckoutSession) { s.PaymentIntent.LatestCharge.AmountRefunded = 1 },
		"refunded":                 func(s *stripe.CheckoutSession) { s.PaymentIntent.LatestCharge.Refunded = true },
		"disputed":                 func(s *stripe.CheckoutSession) { s.PaymentIntent.LatestCharge.Disputed = true },
		"authorization only":       func(s *stripe.CheckoutSession) { s.PaymentIntent.LatestCharge.Captured = false },
		"partial capture":          func(s *stripe.CheckoutSession) { s.PaymentIntent.LatestCharge.AmountCaptured-- },
		"wrong charge intent":      func(s *stripe.CheckoutSession) { s.PaymentIntent.LatestCharge.PaymentIntent.ID = "pi_other" },
		"missing session customer": func(s *stripe.CheckoutSession) { s.Customer = nil },
		"empty session customer":   func(s *stripe.CheckoutSession) { s.Customer = &stripe.Customer{ID: " "} },
		"missing intent customer":  func(s *stripe.CheckoutSession) { s.PaymentIntent.Customer = nil },
		"empty intent customer":    func(s *stripe.CheckoutSession) { s.PaymentIntent.Customer = &stripe.Customer{} },
		"missing charge customer":  func(s *stripe.CheckoutSession) { s.PaymentIntent.LatestCharge.Customer = nil },
		"empty charge customer":    func(s *stripe.CheckoutSession) { s.PaymentIntent.LatestCharge.Customer = &stripe.Customer{} },
		"wrong customer": func(s *stripe.CheckoutSession) {
			s.PaymentIntent.LatestCharge.Customer = &stripe.Customer{ID: "cus_other"}
		},
		"wrong user":         func(s *stripe.CheckoutSession) { s.Metadata["user_id"] = "8" },
		"missing revision":   func(s *stripe.CheckoutSession) { s.Metadata["discount_selection"] = "none" },
		"malformed revision": func(s *stripe.CheckoutSession) { s.Metadata["checkout_revision"] = "NaN" },
	} {
		t.Run(name, func(t *testing.T) {
			check := &model.StripeWalletPaymentCheck{Scope: "stripe:acct_test:test", SessionID: "cs_evidence", TradeNo: stripeWalletTrade(74), UserID: 7, Amount: 1000, Currency: "USD"}
			s := stripeWalletRepairSession(check.SessionID, check.TradeNo)
			mutate(s)
			_, err := stripeWalletRepairProof(check, s)
			require.Error(t, err)
		})
	}
}

func TestStripeWalletAutoRepairPaymentSnapshotCurrencyUnits(t *testing.T) {
	for currency, want := range map[string]float64{"USD": 10, "JPY": 1000, "KRW": 1000, "UGX": 10, "ISK": 10, "HUF": 10, "TWD": 10} {
		t.Run(currency, func(t *testing.T) { require.Equal(t, want, stripeWalletRepairPaymentSnapshot(1000, currency).Money) })
	}
}
