package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	stripe "github.com/stripe/stripe-go/v86"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type fakeStripeWalletGateway struct {
	scope       string
	live        bool
	scopeErr    error
	pages       []stripeWalletEventPage
	listErrAt   int
	listCalls   []stripeWalletListCall
	sessions    map[string]*stripe.CheckoutSession
	sessionErr  error
	sessionGets int
	repairGets  int
	repairErr   error
}

type stripeWalletListCall struct {
	from  int64
	to    int64
	after string
}

func (f *fakeStripeWalletGateway) Scope(context.Context) (string, bool, error) {
	return f.scope, f.live, f.scopeErr
}

func (f *fakeStripeWalletGateway) ListEvents(_ context.Context, from, to int64, after string) (stripeWalletEventPage, error) {
	f.listCalls = append(f.listCalls, stripeWalletListCall{from: from, to: to, after: after})
	if f.listErrAt > 0 && len(f.listCalls) == f.listErrAt {
		return stripeWalletEventPage{}, errors.New("stripe unavailable")
	}
	if len(f.pages) == 0 {
		return stripeWalletEventPage{}, nil
	}
	page := f.pages[0]
	f.pages = f.pages[1:]
	return page, nil
}

func (f *fakeStripeWalletGateway) GetSession(_ context.Context, id string) (*stripe.CheckoutSession, error) {
	f.sessionGets++
	if f.sessionErr != nil {
		return nil, f.sessionErr
	}
	return f.sessions[id], nil
}

func (f *fakeStripeWalletGateway) GetRepairSession(_ context.Context, id string) (*stripe.CheckoutSession, error) {
	f.repairGets++
	return f.sessions[id], f.repairErr
}

type stripeWalletHarness struct {
	t       *testing.T
	db      *gorm.DB
	now     int64
	gateway *fakeStripeWalletGateway
	notices []stripeWalletNotification
	notify  func(context.Context, stripeWalletNotification) error
	prices  map[string]int64
}

func newStripeWalletHarness(t *testing.T) *stripeWalletHarness {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "stripe-wallet.db")+"?_pragma=busy_timeout(5000)"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.TopUp{}, &model.StripeWalletScanState{}, &model.StripeWalletPaymentCheck{}))
	require.NoError(t, db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, quota INTEGER NOT NULL)").Error)
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = oldDB
		if sqlDB, sqlErr := db.DB(); sqlErr == nil {
			_ = sqlDB.Close()
		}
	})
	h := &stripeWalletHarness{
		t: t, db: db, now: 2_000_000_000,
		gateway: &fakeStripeWalletGateway{scope: "stripe:acct_test:test", sessions: map[string]*stripe.CheckoutSession{}},
		prices:  map[string]int64{"price_wallet": 10},
	}
	h.notify = func(_ context.Context, n stripeWalletNotification) error {
		h.notices = append(h.notices, n)
		return nil
	}
	return h
}

func (h *stripeWalletHarness) reconciler() *stripeWalletReconciler {
	return &stripeWalletReconciler{
		gateway: h.gateway,
		notify:  func(ctx context.Context, n stripeWalletNotification) error { return h.notify(ctx, n) },
		now:     func(context.Context) (int64, error) { return h.now, nil },
		prices:  func() map[string]int64 { return h.prices },
	}
}

func stripeWalletTrade(suffix byte) string {
	return "ref_" + fmt.Sprintf("%040x", suffix)
}

func stripeWalletTradeIndex(index int) string {
	return "ref_" + fmt.Sprintf("%040x", index)
}

func stripeWalletPaidSession(id, tradeNo, priceID string, paid bool) *stripe.CheckoutSession {
	status := stripe.CheckoutSessionPaymentStatusPaid
	if !paid {
		status = stripe.CheckoutSessionPaymentStatusUnpaid
	}
	return &stripe.CheckoutSession{
		ID: id, ClientReferenceID: tradeNo, Metadata: map[string]string{"trade_no": tradeNo},
		Mode: stripe.CheckoutSessionModePayment, PaymentStatus: status, Livemode: false,
		AmountTotal: 1000, Currency: stripe.CurrencyUSD,
		LineItems: &stripe.LineItemList{Data: []*stripe.LineItem{{Price: &stripe.Price{ID: priceID}, Quantity: 1}}},
	}
}

func stripeWalletEvent(t *testing.T, id string, created int64, session *stripe.CheckoutSession) *stripe.Event {
	t.Helper()
	raw, err := common.Marshal(session)
	require.NoError(t, err)
	return &stripe.Event{ID: id, Type: stripe.EventTypeCheckoutSessionCompleted, Created: created, Livemode: session.Livemode, Data: &stripe.EventData{Raw: raw}}
}

func (h *stripeWalletHarness) insertCheck(check model.StripeWalletPaymentCheck) model.StripeWalletPaymentCheck {
	h.t.Helper()
	if check.Scope == "" {
		check.Scope = h.gateway.scope
	}
	require.NoError(h.t, model.InsertStripeWalletPaymentCheck(context.Background(), &check))
	var stored model.StripeWalletPaymentCheck
	require.NoError(h.t, h.db.Where("scope = ? AND session_id = ?", check.Scope, check.SessionID).First(&stored).Error)
	return stored
}

func (h *stripeWalletHarness) claim(sessionID string) (model.StripeWalletPaymentCheck, string) {
	h.t.Helper()
	token := "lease-" + sessionID
	rows, err := model.ClaimStripeWalletPaymentChecks(context.Background(), h.gateway.scope, h.now, stripeWalletLeaseSeconds, 100, token)
	require.NoError(h.t, err)
	for _, row := range rows {
		if row.SessionID == sessionID {
			return row, token
		}
	}
	h.t.Fatalf("session %s was not claimable", sessionID)
	return model.StripeWalletPaymentCheck{}, ""
}

func (h *stripeWalletHarness) loadCheck(sessionID string) model.StripeWalletPaymentCheck {
	h.t.Helper()
	var row model.StripeWalletPaymentCheck
	require.NoError(h.t, h.db.Where("scope = ? AND session_id = ?", h.gateway.scope, sessionID).First(&row).Error)
	return row
}

func (h *stripeWalletHarness) createTopUp(tradeNo, sessionID, status string, amount int64) model.TopUp {
	h.t.Helper()
	row := model.TopUp{UserId: 7, Amount: amount, TradeNo: tradeNo, GatewayTradeNo: sessionID,
		PaymentMethod: model.PaymentMethodStripe, PaymentProvider: model.PaymentProviderStripe,
		PaymentPriceId: "price_wallet", Status: status, CreateTime: h.now - 3600}
	require.NoError(h.t, h.db.Create(&row).Error)
	return row
}

func TestStripeWalletPaidLocalAnomaliesSendInitialAlert(t *testing.T) {
	for i, status := range []string{"missing", common.TopUpStatusPending, common.TopUpStatusFailed, common.TopUpStatusExpired} {
		t.Run(status, func(t *testing.T) {
			h := newStripeWalletHarness(t)
			trade, sessionID := stripeWalletTrade(byte(i+1)), fmt.Sprintf("cs_%d", i)
			if status != "missing" {
				h.createTopUp(trade, sessionID, status, 10)
			}
			h.insertCheck(model.StripeWalletPaymentCheck{SessionID: sessionID, TradeNo: trade, PriceID: "price_wallet", WalletVerified: true, PaidAt: h.now, NextCheckAt: h.now})
			check, token := h.claim(sessionID)
			require.NoError(t, h.reconciler().processCheck(context.Background(), &check, token))
			require.Len(t, h.notices, 1)
			require.Equal(t, "initial", h.notices[0].Kind)
			require.Equal(t, status, h.notices[0].LocalStatus)
		})
	}
}

func TestStripeWalletRunDiscoversAndProcessesPaidMissingOrder(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, sessionID := stripeWalletTrade(22), "cs_run_missing"
	s := stripeWalletPaidSession(sessionID, trade, "price_wallet", true)
	h.gateway.sessions[sessionID] = s
	h.gateway.pages = []stripeWalletEventPage{{Events: []*stripe.Event{
		stripeWalletEvent(t, "evt_run_missing", h.now-stripeWalletOverlapSeconds, s),
	}}}

	processed, err := h.reconciler().run(context.Background())

	require.NoError(t, err)
	require.Equal(t, 1, processed)
	require.Len(t, h.notices, 1)
	require.Equal(t, "initial", h.notices[0].Kind)
	require.Equal(t, "missing", h.notices[0].LocalStatus)
	stored := h.loadCheck(sessionID)
	require.Equal(t, 1, stored.NotificationCount)
	require.NotZero(t, stored.FirstAnomalyAt)
}

func TestStripeWalletPoisonEventIsQuarantinedWithoutBlockingValidEventOrCursor(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, sessionID := stripeWalletTrade(23), "cs_after_poison"
	valid := stripeWalletPaidSession(sessionID, trade, "price_wallet", true)
	h.gateway.pages = []stripeWalletEventPage{{Events: []*stripe.Event{
		{ID: "evt_poison", Type: stripe.EventTypeCheckoutSessionCompleted, Created: h.now - 200, Data: &stripe.EventData{Raw: []byte(`{"broken"`)}},
		stripeWalletEvent(t, "evt_after_poison", h.now-100, valid),
	}}}
	state := &model.StripeWalletScanState{Scope: h.gateway.scope, InitialAt: h.now - 3600, RecentThrough: h.now - 600, RecentWindowEnd: h.now}
	token := "poison-page-lease"
	require.NoError(t, h.db.Create(state).Error)
	require.NoError(t, h.db.Model(state).Updates(map[string]any{"lease_token": token, "lease_until": h.now + 3600}).Error)

	require.NoError(t, h.reconciler().scan(context.Background(), state, token, false, h.now, false))

	require.Zero(t, h.gateway.sessionGets)
	var poison model.StripeWalletPaymentCheck
	require.NoError(t, h.db.Where("session_id = ?", "event:evt_poison").First(&poison).Error)
	require.NotEmpty(t, poison.DiscoveryError)
	validCheck := h.loadCheck(sessionID)
	require.Empty(t, validCheck.DiscoveryError)
	var stored model.StripeWalletScanState
	require.NoError(t, h.db.First(&stored, "scope = ?", h.gateway.scope).Error)
	require.Equal(t, h.now, stored.RecentThrough)
}

func TestStripeWalletDiscoveryOfOneHundredCandidatesPerformsNoSessionReads(t *testing.T) {
	h := newStripeWalletHarness(t)
	events := make([]*stripe.Event, 0, 100)
	for i := 1; i <= 100; i++ {
		s := stripeWalletPaidSession(fmt.Sprintf("cs_bulk_%03d", i), stripeWalletTradeIndex(1000+i), "price_wallet", true)
		events = append(events, stripeWalletEvent(t, fmt.Sprintf("evt_bulk_%03d", i), h.now-int64(100+i), s))
	}
	h.gateway.pages = []stripeWalletEventPage{{Events: events}}
	state := &model.StripeWalletScanState{Scope: h.gateway.scope, InitialAt: h.now - 3600, RecentThrough: h.now - 600, RecentWindowEnd: h.now}
	token := "bulk-page-lease"
	require.NoError(t, h.db.Create(state).Error)
	require.NoError(t, h.db.Model(state).Updates(map[string]any{"lease_token": token, "lease_until": h.now + 3600}).Error)

	require.NoError(t, h.reconciler().scan(context.Background(), state, token, false, h.now, false))

	require.Zero(t, h.gateway.sessionGets)
	var count int64
	require.NoError(t, h.db.Model(&model.StripeWalletPaymentCheck{}).Count(&count).Error)
	require.Equal(t, int64(100), count)
}

func TestStripeWalletHydrationFailureIsPersistedAndRetriedAfterCursorAdvances(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, sessionID := stripeWalletTrade(24), "cs_hydration_retry"
	s := stripeWalletPaidSession(sessionID, trade, "price_wallet", true)
	h.gateway.pages = []stripeWalletEventPage{{Events: []*stripe.Event{stripeWalletEvent(t, "evt_hydration_retry", h.now-600, s)}}}
	h.gateway.sessionErr = errors.New("session lookup unavailable")
	state := &model.StripeWalletScanState{Scope: h.gateway.scope, InitialAt: h.now - 3600, RecentThrough: h.now - 600, RecentWindowEnd: h.now}
	token := "hydration-page-lease"
	require.NoError(t, h.db.Create(state).Error)
	require.NoError(t, h.db.Model(state).Updates(map[string]any{"lease_token": token, "lease_until": h.now + 3600}).Error)
	require.NoError(t, h.reconciler().scan(context.Background(), state, token, false, h.now, false))
	require.Zero(t, h.gateway.sessionGets)

	processed, err := h.reconciler().processChecks(context.Background(), h.gateway.scope)
	require.ErrorContains(t, err, "session lookup unavailable")
	require.Equal(t, 1, processed)
	failed := h.loadCheck(sessionID)
	require.Contains(t, failed.VerificationError, "session lookup unavailable")
	require.Empty(t, failed.LocalStatus)
	require.Zero(t, failed.FirstAnomalyAt)
	require.Empty(t, h.notices)
	var cursor model.StripeWalletScanState
	require.NoError(t, h.db.First(&cursor, "scope = ?", h.gateway.scope).Error)
	require.Equal(t, h.now, cursor.RecentThrough)

	h.gateway.sessionErr = nil
	h.gateway.sessions[sessionID] = s
	h.now += 61
	processed, err = h.reconciler().processChecks(context.Background(), h.gateway.scope)
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	verified := h.loadCheck(sessionID)
	require.True(t, verified.SessionVerified)
	require.Empty(t, verified.VerificationError)
	require.Equal(t, "missing", verified.LocalStatus)
	require.Equal(t, 2, h.gateway.sessionGets)
	require.Len(t, h.notices, 1)
	require.Equal(t, "initial", h.notices[0].Kind)
}

func TestStripeWalletTransientHydrationFailureDoesNotAlertSuccessfulLocalOrder(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, sessionID := stripeWalletTrade(28), "cs_success_hydration_retry"
	h.createTopUp(trade, sessionID, common.TopUpStatusSuccess, 10)
	h.insertCheck(model.StripeWalletPaymentCheck{SessionID: sessionID, TradeNo: trade, PaidAt: h.now - 600, NextCheckAt: h.now})
	h.gateway.sessionErr = errors.New("temporary Stripe timeout")

	processed, err := h.reconciler().processChecks(context.Background(), h.gateway.scope)
	require.ErrorContains(t, err, "temporary Stripe timeout")
	require.Equal(t, 1, processed)
	failed := h.loadCheck(sessionID)
	require.Zero(t, failed.FirstAnomalyAt)
	require.Zero(t, failed.ClosedAt)
	require.Empty(t, h.notices)

	h.gateway.sessionErr = nil
	h.gateway.sessions[sessionID] = stripeWalletPaidSession(sessionID, trade, "price_wallet", true)
	h.now += 61
	processed, err = h.reconciler().processChecks(context.Background(), h.gateway.scope)
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	verified := h.loadCheck(sessionID)
	require.True(t, verified.SessionVerified)
	require.NotZero(t, verified.ClosedAt)
	require.Zero(t, verified.FirstAnomalyAt)
	require.Empty(t, h.notices)
}

func TestStripeWalletFailedInitialNotificationIsThrottledForFiveMinutes(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, sessionID := stripeWalletTrade(29), "cs_failed_notify_cooldown"
	h.createTopUp(trade, sessionID, common.TopUpStatusFailed, 10)
	h.insertCheck(model.StripeWalletPaymentCheck{SessionID: sessionID, TradeNo: trade, PriceID: "price_wallet", WalletVerified: true,
		PaidAt: h.now - 600, NextCheckAt: h.now})
	notifyCalls := 0
	h.notify = func(context.Context, stripeWalletNotification) error {
		notifyCalls++
		return errors.New("dingtalk unavailable")
	}

	processed, err := h.reconciler().processChecks(context.Background(), h.gateway.scope)
	require.ErrorContains(t, err, "dingtalk unavailable")
	require.Equal(t, 1, processed)
	require.Equal(t, 1, notifyCalls)
	firstAttempt := h.loadCheck(sessionID).LastAttemptAt
	require.Equal(t, h.now, firstAttempt)

	h.now += 61
	processed, err = h.reconciler().processChecks(context.Background(), h.gateway.scope)
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	require.Equal(t, 1, notifyCalls)
	require.Equal(t, firstAttempt, h.loadCheck(sessionID).LastAttemptAt)

	h.now = firstAttempt + stripeWalletNotifyRetry
	processed, err = h.reconciler().processChecks(context.Background(), h.gateway.scope)
	require.ErrorContains(t, err, "dingtalk unavailable")
	require.Equal(t, 1, processed)
	require.Equal(t, 2, notifyCalls)
}

func TestStripeWalletDiscoveryErrorRemainsAnomalyAndNeverHydrates(t *testing.T) {
	h := newStripeWalletHarness(t)
	event := &stripe.Event{ID: "evt_discovery_error", Type: stripe.EventTypeCheckoutSessionCompleted, Created: h.now - 600,
		Data: &stripe.EventData{Raw: []byte(`{"invalid"`)}}
	require.NoError(t, h.reconciler().ingest(context.Background(), h.gateway.scope, false, event, false))

	processed, err := h.reconciler().processChecks(context.Background(), h.gateway.scope)

	require.NoError(t, err)
	require.Equal(t, 1, processed)
	require.Zero(t, h.gateway.sessionGets)
	require.Len(t, h.notices, 1)
	require.Equal(t, "discovery_error", h.notices[0].LocalStatus)
	stored := h.loadCheck("event:evt_discovery_error")
	require.NotEmpty(t, stored.DiscoveryError)
	require.Zero(t, stored.RecoveredAt)
	require.Zero(t, stored.ClosedAt)
}

func TestStripeWalletBlockedNotifierDoesNotSuppressRecentOrHistoryDiscovery(t *testing.T) {
	h := newStripeWalletHarness(t)
	recent := stripeWalletPaidSession("cs_blocked_recent", stripeWalletTrade(25), "price_wallet", true)
	historical := stripeWalletPaidSession("cs_blocked_history", stripeWalletTrade(26), "price_wallet", true)
	h.gateway.pages = []stripeWalletEventPage{
		{Events: []*stripe.Event{stripeWalletEvent(t, "evt_blocked_recent", h.now-600, recent)}},
		{Events: []*stripe.Event{stripeWalletEvent(t, "evt_blocked_history", h.now-86400, historical)}},
	}
	notifyCalls := 0
	h.notify = func(ctx context.Context, _ stripeWalletNotification) error {
		notifyCalls++
		<-ctx.Done()
		return ctx.Err()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_, err := h.reconciler().run(ctx)

	require.Error(t, err)
	require.GreaterOrEqual(t, notifyCalls, 1)
	var count int64
	require.NoError(t, h.db.Model(&model.StripeWalletPaymentCheck{}).Where("scope = ?", h.gateway.scope).Count(&count).Error)
	require.Equal(t, int64(2), count)
	var state model.StripeWalletScanState
	require.NoError(t, h.db.First(&state, "scope = ?", h.gateway.scope).Error)
	require.Equal(t, h.now, state.RecentThrough)
	require.True(t, state.BackfillDone)
}

func TestStripeWalletScopeOutageStillSendsRecoveryForVerifiedCheck(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, sessionID := stripeWalletTrade(27), "cs_scope_outage_recovered"
	h.createTopUp(trade, sessionID, common.TopUpStatusSuccess, 10)
	h.insertCheck(model.StripeWalletPaymentCheck{
		SessionID: sessionID, TradeNo: trade, PriceID: "price_wallet", WalletVerified: true, SessionVerified: true,
		PaidAt: h.now - 7200, FirstAnomalyAt: h.now - 7000, LastNotifiedAt: h.now - 3600,
		NotificationCount: 1, NextCheckAt: h.now,
	})
	h.gateway.scopeErr = errors.New("account endpoint unavailable")

	processed, err := h.reconciler().run(context.Background())

	require.ErrorContains(t, err, "account endpoint unavailable")
	require.Equal(t, 1, processed)
	require.Zero(t, h.gateway.sessionGets)
	require.Len(t, h.notices, 1)
	require.Equal(t, "recovered", h.notices[0].Kind)
	require.NotZero(t, h.loadCheck(sessionID).ClosedAt)
}

func TestStripeWalletLargeHistoryBacklogStillProcessesRecentAndRecoveryLanes(t *testing.T) {
	h := newStripeWalletHarness(t)
	historical := make([]model.StripeWalletPaymentCheck, 0, 1001)
	for i := 0; i < 1001; i++ {
		historical = append(historical, model.StripeWalletPaymentCheck{
			Scope: h.gateway.scope, SessionID: fmt.Sprintf("cs_history_%04d", i), TradeNo: stripeWalletTradeIndex(10_000 + i),
			WalletVerified: true, Historical: true, PaidAt: h.now - 7200, NextCheckAt: h.now,
		})
	}
	require.NoError(t, h.db.CreateInBatches(&historical, 200).Error)
	recentSession := "cs_fair_recent"
	h.insertCheck(model.StripeWalletPaymentCheck{SessionID: recentSession, TradeNo: stripeWalletTrade(30), WalletVerified: true,
		PaidAt: h.now - 600, NextCheckAt: h.now})
	recoveryTrade, recoverySession := stripeWalletTrade(31), "cs_fair_recovery"
	h.createTopUp(recoveryTrade, recoverySession, common.TopUpStatusSuccess, 10)
	h.insertCheck(model.StripeWalletPaymentCheck{SessionID: recoverySession, TradeNo: recoveryTrade, WalletVerified: true, SessionVerified: true,
		PaidAt: h.now - 7200, FirstAnomalyAt: h.now - 7000, LastNotifiedAt: h.now - 3600,
		NotificationCount: 1, NextCheckAt: h.now})

	processed, err := h.reconciler().processChecks(context.Background(), h.gateway.scope)

	require.NoError(t, err)
	require.Equal(t, 35, processed)
	require.Equal(t, 1, h.loadCheck(recentSession).NotificationCount)
	require.NotZero(t, h.loadCheck(recoverySession).ClosedAt)
	var progressedHistory int64
	require.NoError(t, h.db.Model(&model.StripeWalletPaymentCheck{}).
		Where("scope = ? AND historical = ? AND notification_count = ?", h.gateway.scope, true, 1).Count(&progressedHistory).Error)
	require.Equal(t, int64(33), progressedHistory)
	require.Zero(t, h.gateway.sessionGets)
}

func TestStripeWalletOutstandingVerificationFailureKeepsTaskHealthUnhealthyWhenNotDue(t *testing.T) {
	h := newStripeWalletHarness(t)
	h.insertCheck(model.StripeWalletPaymentCheck{
		SessionID: "cs_health_not_due", TradeNo: stripeWalletTrade(32), PaidAt: h.now - 600,
		NextCheckAt: h.now + 3600, VerificationError: "temporary Stripe timeout", VerificationPending: true,
	})
	h.gateway.pages = []stripeWalletEventPage{{}, {}}

	processed, err := h.reconciler().run(context.Background())

	require.ErrorContains(t, err, "verification failures remain unresolved")
	require.Zero(t, processed)
	var state model.StripeWalletScanState
	require.NoError(t, h.db.First(&state, "scope = ?", h.gateway.scope).Error)
	require.Equal(t, h.now, state.FailureSince)
	require.Zero(t, state.LastSuccessAt)
}

func TestStripeWalletPermanentIdentityFailureDoesNotKeepTaskHealthUnhealthy(t *testing.T) {
	h := newStripeWalletHarness(t)
	h.insertCheck(model.StripeWalletPaymentCheck{
		SessionID: "cs_permanent_identity", TradeNo: stripeWalletTrade(33), PaidAt: h.now - 600,
		NextCheckAt: h.now + 3600, VerificationError: "Stripe wallet identity conflict", VerificationPending: false,
	})
	h.gateway.pages = []stripeWalletEventPage{{}, {}}

	processed, err := h.reconciler().run(context.Background())

	require.NoError(t, err)
	require.Zero(t, processed)
	var state model.StripeWalletScanState
	require.NoError(t, h.db.First(&state, "scope = ?", h.gateway.scope).Error)
	require.Zero(t, state.FailureSince)
	require.Equal(t, h.now, state.LastSuccessAt)
}

func TestStripeWalletRecoveredAccountLookupClearsConfigurationFailureChain(t *testing.T) {
	h := newStripeWalletHarness(t)
	oldFailure := h.now - 7200
	require.NoError(t, h.db.Create(&model.StripeWalletScanState{
		Scope: stripeWalletConfigurationScope, InitialAt: oldFailure, RecentThrough: oldFailure,
		FailureSince: oldFailure, LastFailureAlertAt: h.now - 3600,
	}).Error)

	require.NoError(t, h.reconciler().clearConfigurationFailure(context.Background(), h.now))

	var stored model.StripeWalletScanState
	require.NoError(t, h.db.First(&stored, "scope = ?", stripeWalletConfigurationScope).Error)
	require.Zero(t, stored.FailureSince)
	require.Equal(t, h.now, stored.LastSuccessAt)
}

func TestStripeWalletSuccessfulOrderClosesImmediatelyWithoutAlert(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, sessionID := stripeWalletTrade(10), "cs_success_grace"
	h.createTopUp(trade, sessionID, common.TopUpStatusSuccess, 10)
	h.insertCheck(model.StripeWalletPaymentCheck{SessionID: sessionID, TradeNo: trade, PriceID: "price_wallet", WalletVerified: true, PaidAt: h.now - 60, NextCheckAt: h.now})
	check, token := h.claim(sessionID)
	require.NoError(t, h.reconciler().processCheck(context.Background(), &check, token))
	require.Empty(t, h.notices)
	require.Equal(t, h.now, h.loadCheck(sessionID).ClosedAt)
}

func TestStripeWalletOlderSuccessfulOrderClosesWithoutAlert(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, sessionID := stripeWalletTrade(11), "cs_success"
	h.createTopUp(trade, sessionID, common.TopUpStatusSuccess, 10)
	h.insertCheck(model.StripeWalletPaymentCheck{SessionID: sessionID, TradeNo: trade, PriceID: "price_wallet", WalletVerified: true, PaidAt: h.now - 600, NextCheckAt: h.now})
	check, token := h.claim(sessionID)
	require.NoError(t, h.reconciler().processCheck(context.Background(), &check, token))
	require.Empty(t, h.notices)
	require.Equal(t, h.now, h.loadCheck(sessionID).ClosedAt)
}

func TestStripeWalletCompletedUnpaidEventIsIgnoredEvenWhenCurrentSessionIsPaid(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, sessionID := stripeWalletTrade(12), "cs_unpaid_snapshot"
	snapshot := stripeWalletPaidSession(sessionID, trade, "price_wallet", false)
	h.gateway.sessions[sessionID] = stripeWalletPaidSession(sessionID, trade, "price_wallet", true)
	require.NoError(t, h.reconciler().ingest(context.Background(), h.gateway.scope, false, stripeWalletEvent(t, "evt_unpaid", h.now-600, snapshot), false))
	var count int64
	require.NoError(t, h.db.Model(&model.StripeWalletPaymentCheck{}).Count(&count).Error)
	require.Zero(t, count)
	require.Zero(t, h.gateway.sessionGets)
}

func TestStripeWalletSubscriptionSessionsAreExcluded(t *testing.T) {
	for name, mutate := range map[string]func(*stripe.CheckoutSession){
		"subscription mode": func(s *stripe.CheckoutSession) { s.Mode = stripe.CheckoutSessionModeSubscription },
		"plan metadata":     func(s *stripe.CheckoutSession) { s.Metadata["plan_id"] = "12" },
	} {
		t.Run(name, func(t *testing.T) {
			h := newStripeWalletHarness(t)
			trade, sessionID := stripeWalletTrade(13), "cs_subscription"
			s := stripeWalletPaidSession(sessionID, trade, "price_wallet", true)
			mutate(s)
			require.NoError(t, h.reconciler().ingest(context.Background(), h.gateway.scope, false, stripeWalletEvent(t, "evt_subscription", h.now-600, s), false))
			var count int64
			require.NoError(t, h.db.Model(&model.StripeWalletPaymentCheck{}).Count(&count).Error)
			require.Zero(t, count)
		})
	}
}

func TestStripeWalletSubscriptionTopUpMirrorWithZeroAmountIsNotSuccess(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, sessionID := stripeWalletTrade(14), "cs_zero_amount"
	h.createTopUp(trade, sessionID, common.TopUpStatusSuccess, 0)
	check := model.StripeWalletPaymentCheck{TradeNo: trade, SessionID: sessionID, PriceID: "price_wallet", WalletVerified: true}
	status, err := h.reconciler().localStatus(context.Background(), &check)
	require.NoError(t, err)
	require.Equal(t, "identity_conflict", status)
}

func TestStripeWalletDuplicateEventsForOneSessionCreateOneCheck(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, sessionID := stripeWalletTrade(15), "cs_duplicate"
	s := stripeWalletPaidSession(sessionID, trade, "price_wallet", true)
	h.gateway.sessions[sessionID] = s
	require.NoError(t, h.reconciler().ingest(context.Background(), h.gateway.scope, false, stripeWalletEvent(t, "evt_first", h.now-600, s), false))
	require.NoError(t, h.reconciler().ingest(context.Background(), h.gateway.scope, false, stripeWalletEvent(t, "evt_second", h.now-590, s), false))
	var count int64
	require.NoError(t, h.db.Model(&model.StripeWalletPaymentCheck{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
	processed, err := h.reconciler().processChecks(context.Background(), h.gateway.scope)
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	require.Len(t, h.notices, 1)
}

func TestStripeWalletRetiredPriceMissingOrderAlertsIdentityUnknown(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, sessionID := stripeWalletTrade(16), "cs_retired_price"
	s := stripeWalletPaidSession(sessionID, trade, "price_retired", true)
	h.gateway.sessions[sessionID] = s
	require.NoError(t, h.reconciler().ingest(context.Background(), h.gateway.scope, false, stripeWalletEvent(t, "evt_retired", h.now-600, s), false))
	check := h.loadCheck(sessionID)
	require.False(t, check.WalletVerified)
	check.NextCheckAt = h.now
	require.NoError(t, h.db.Model(&check).Update("next_check_at", h.now).Error)
	claimed, token := h.claim(sessionID)
	require.NoError(t, h.reconciler().processCheck(context.Background(), &claimed, token))
	require.Len(t, h.notices, 1)
	require.Equal(t, "wallet_identity_unknown", h.notices[0].LocalStatus)
}

func TestStripeWalletInitialNotificationFailureStillAllowsRecoveryNotice(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, sessionID := stripeWalletTrade(17), "cs_notify_failure"
	h.createTopUp(trade, sessionID, common.TopUpStatusFailed, 10)
	h.insertCheck(model.StripeWalletPaymentCheck{SessionID: sessionID, TradeNo: trade, PriceID: "price_wallet", WalletVerified: true, PaidAt: h.now - 600, NextCheckAt: h.now})
	attempt := 0
	h.notify = func(_ context.Context, n stripeWalletNotification) error {
		h.notices = append(h.notices, n)
		attempt++
		if attempt == 1 {
			return errors.New("dingtalk down")
		}
		return nil
	}
	check, token := h.claim(sessionID)
	require.Error(t, h.reconciler().processCheck(context.Background(), &check, token))
	require.NotZero(t, h.loadCheck(sessionID).FirstAnomalyAt)
	require.NoError(t, h.db.Model(&model.TopUp{}).Where("trade_no = ?", trade).Update("status", common.TopUpStatusSuccess).Error)
	h.now += stripeWalletLeaseSeconds + 1
	require.NoError(t, h.db.Model(&model.StripeWalletPaymentCheck{}).Where("session_id = ?", sessionID).Updates(map[string]any{"next_check_at": h.now, "lease_until": 0, "lease_token": ""}).Error)
	check, token = h.claim(sessionID)
	require.NoError(t, h.reconciler().processCheck(context.Background(), &check, token))
	require.Len(t, h.notices, 2)
	require.Equal(t, "recovered", h.notices[1].Kind)
	require.NotZero(t, h.loadCheck(sessionID).ClosedAt)
}

func TestStripeWalletReminderOnlySendsWhenHourIsDue(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, sessionID := stripeWalletTrade(18), "cs_reminder"
	h.createTopUp(trade, sessionID, common.TopUpStatusFailed, 10)
	h.insertCheck(model.StripeWalletPaymentCheck{SessionID: sessionID, TradeNo: trade, PriceID: "price_wallet", WalletVerified: true,
		PaidAt: h.now - 7200, FirstAnomalyAt: h.now - 7200, LastNotifiedAt: h.now - stripeWalletReminder + 1, NotificationCount: 1, NextCheckAt: h.now})
	check, token := h.claim(sessionID)
	require.NoError(t, h.reconciler().processCheck(context.Background(), &check, token))
	require.Empty(t, h.notices)
	h.now += 61
	require.NoError(t, h.db.Model(&model.StripeWalletPaymentCheck{}).Where("session_id = ?", sessionID).Updates(map[string]any{"next_check_at": h.now, "lease_until": 0, "lease_token": ""}).Error)
	check, token = h.claim(sessionID)
	require.NoError(t, h.reconciler().processCheck(context.Background(), &check, token))
	require.Len(t, h.notices, 1)
	require.Equal(t, "reminder", h.notices[0].Kind)
}

func TestStripeWalletDatabaseReadFailureIsNotReportedAsMissing(t *testing.T) {
	h := newStripeWalletHarness(t)
	h.insertCheck(model.StripeWalletPaymentCheck{SessionID: "cs_db_error", TradeNo: stripeWalletTrade(19), PriceID: "price_wallet", WalletVerified: true, PaidAt: h.now - 600, NextCheckAt: h.now})
	check, token := h.claim("cs_db_error")
	sqlDB, err := h.db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	err = h.reconciler().processCheck(context.Background(), &check, token)
	require.Error(t, err)
	require.Empty(t, h.notices)
}

func TestStripeWalletProcessingNeverMutatesTopUpOrUserBusinessRows(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, sessionID := stripeWalletTrade(20), "cs_read_only"
	before := h.createTopUp(trade, sessionID, common.TopUpStatusFailed, 10)
	require.NoError(t, h.db.Exec("INSERT INTO users(id, quota) VALUES(?, ?)", 7, 123456).Error)
	h.insertCheck(model.StripeWalletPaymentCheck{SessionID: sessionID, TradeNo: trade, PriceID: "price_wallet", WalletVerified: true, PaidAt: h.now - 600, NextCheckAt: h.now})
	check, token := h.claim(sessionID)
	require.NoError(t, h.reconciler().processCheck(context.Background(), &check, token))
	var after model.TopUp
	require.NoError(t, h.db.First(&after, before.Id).Error)
	require.Equal(t, before, after)
	var quota int
	require.NoError(t, h.db.Raw("SELECT quota FROM users WHERE id = ?", 7).Scan(&quota).Error)
	require.Equal(t, 123456, quota)
}

func TestStripeWalletInitialBackfillStartsSevenDaysBeforeTaskStart(t *testing.T) {
	h := newStripeWalletHarness(t)
	state := &model.StripeWalletScanState{Scope: h.gateway.scope, InitialAt: h.now, RecentThrough: h.now}
	token := "history-lease"
	require.NoError(t, h.db.Create(state).Error)
	require.NoError(t, h.db.Model(state).Updates(map[string]any{"lease_token": token, "lease_until": h.now + 3600}).Error)
	require.NoError(t, h.reconciler().scan(context.Background(), state, token, false, h.now, true))
	require.Len(t, h.gateway.listCalls, 1)
	require.Equal(t, h.now-stripeWalletHistorySeconds, h.gateway.listCalls[0].from)
	require.Equal(t, h.now, h.gateway.listCalls[0].to)
}

func TestStripeWalletBackfillUsesPaidEventTimeNotSessionCreationTime(t *testing.T) {
	h := newStripeWalletHarness(t)
	trade, sessionID := stripeWalletTrade(21), "cs_old_created_recent_paid"
	s := stripeWalletPaidSession(sessionID, trade, "price_wallet", true)
	s.Created = h.now - 90*24*60*60
	h.gateway.sessions[sessionID] = s
	h.gateway.pages = []stripeWalletEventPage{{Events: []*stripe.Event{stripeWalletEvent(t, "evt_recent_paid", h.now-3600, s)}}}
	state := &model.StripeWalletScanState{Scope: h.gateway.scope, InitialAt: h.now, RecentThrough: h.now}
	token := "history-paid-lease"
	require.NoError(t, h.db.Create(state).Error)
	require.NoError(t, h.db.Model(state).Updates(map[string]any{"lease_token": token, "lease_until": h.now + 3600}).Error)
	require.NoError(t, h.reconciler().scan(context.Background(), state, token, false, h.now, true))
	require.Equal(t, "evt_recent_paid", h.loadCheck(sessionID).EventID)
}

func TestStripeWalletFailedPageDoesNotAdvanceRecentCursor(t *testing.T) {
	h := newStripeWalletHarness(t)
	state := &model.StripeWalletScanState{Scope: h.gateway.scope, InitialAt: h.now - 3600, RecentThrough: h.now - 600, RecentWindowEnd: h.now}
	token := "failed-page-lease"
	require.NoError(t, h.db.Create(state).Error)
	require.NoError(t, h.db.Model(state).Updates(map[string]any{"lease_token": token, "lease_until": h.now + 3600}).Error)
	h.gateway.listErrAt = 1
	require.Error(t, h.reconciler().scan(context.Background(), state, token, false, h.now, false))
	var stored model.StripeWalletScanState
	require.NoError(t, h.db.First(&stored, "scope = ?", h.gateway.scope).Error)
	require.Equal(t, h.now-600, stored.RecentThrough)
	require.Empty(t, stored.RecentAfter)
}

func TestStripeWalletPaginationRejectsNonAdvancingPage(t *testing.T) {
	h := newStripeWalletHarness(t)
	e := &stripe.Event{ID: "evt_same", Created: h.now - 100, Type: stripe.EventTypeChargeSucceeded, Data: &stripe.EventData{Raw: []byte(`{}`)}}
	h.gateway.pages = []stripeWalletEventPage{{Events: []*stripe.Event{e}, HasMore: true}}
	state := &model.StripeWalletScanState{Scope: h.gateway.scope, InitialAt: h.now - 3600, RecentThrough: h.now - 600, RecentWindowEnd: h.now, RecentAfter: "evt_same"}
	token := "duplicate-page-lease"
	require.NoError(t, h.db.Create(state).Error)
	require.NoError(t, h.db.Model(state).Updates(map[string]any{"lease_token": token, "lease_until": h.now + 3600}).Error)
	err := h.reconciler().scan(context.Background(), state, token, false, h.now, false)
	require.ErrorContains(t, err, "pagination did not advance")
}

func TestStripeWalletPageBudgetPersistsCursorForNextRun(t *testing.T) {
	h := newStripeWalletHarness(t)
	for i := 0; i < stripeWalletPageBudget; i++ {
		e := &stripe.Event{ID: fmt.Sprintf("evt_%d", i), Created: h.now - 100, Type: stripe.EventTypeChargeSucceeded, Data: &stripe.EventData{Raw: []byte(`{}`)}}
		h.gateway.pages = append(h.gateway.pages, stripeWalletEventPage{Events: []*stripe.Event{e}, HasMore: true})
	}
	state := &model.StripeWalletScanState{Scope: h.gateway.scope, InitialAt: h.now - 3600, RecentThrough: h.now - 600, RecentWindowEnd: h.now}
	token := "budget-lease"
	require.NoError(t, h.db.Create(state).Error)
	require.NoError(t, h.db.Model(state).Updates(map[string]any{"lease_token": token, "lease_until": h.now + 3600}).Error)
	require.NoError(t, h.reconciler().scan(context.Background(), state, token, false, h.now, false))
	var stored model.StripeWalletScanState
	require.NoError(t, h.db.First(&stored, "scope = ?", h.gateway.scope).Error)
	require.Equal(t, "evt_4", stored.RecentAfter)
	require.Equal(t, h.now-600, stored.RecentThrough)
}

func TestStripeWalletThirtyDayCoverageGapPersistsTaskHealthFailure(t *testing.T) {
	h := newStripeWalletHarness(t)
	state := &model.StripeWalletScanState{Scope: h.gateway.scope, InitialAt: h.now - stripeWalletRetention - 1, RecentThrough: h.now - stripeWalletRetention - 1}
	token := "retention-gap-lease"
	require.NoError(t, h.db.Create(state).Error)
	require.NoError(t, h.db.Model(state).Updates(map[string]any{"lease_token": token, "lease_until": h.now + 3600}).Error)
	scanErr := h.reconciler().scan(context.Background(), state, token, false, h.now, false)
	require.ErrorContains(t, scanErr, "retention exceeded")
	require.Error(t, h.reconciler().saveHealth(context.Background(), state, token, h.now, scanErr))
	var stored model.StripeWalletScanState
	require.NoError(t, h.db.First(&stored, "scope = ?", h.gateway.scope).Error)
	require.Equal(t, h.now, stored.FailureSince)
	require.Zero(t, stored.LastSuccessAt)
}
