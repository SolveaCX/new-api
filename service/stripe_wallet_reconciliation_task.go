package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/stripe/stripe-go/v86"
)

const (
	stripeWalletGraceSeconds       = int64(5 * 60)
	stripeWalletHistorySeconds     = int64(7 * 24 * 60 * 60)
	stripeWalletOverlapSeconds     = int64(10 * 60)
	stripeWalletRetention          = int64(30 * 24 * 60 * 60)
	stripeWalletLeaseSeconds       = int64(120)
	stripeWalletReminder           = int64(60 * 60)
	stripeWalletNotifyRetry        = int64(5 * 60)
	stripeWalletPageBudget         = 5
	stripeWalletConfigurationScope = "stripe-wallet-configuration"
)

var (
	stripeWalletTaskOnce    sync.Once
	stripeWalletTradeNo     = regexp.MustCompile(`^ref_[a-fA-F0-9]{40}$`)
	errStripeWalletIdentity = errors.New("Stripe wallet identity conflict")
)

// This task observes payments. It deliberately never calls payment fulfillment,
// wallet recharge, invoice mutation or business order status update functions.
func StartStripeWalletReconciliationTask() {
	if !common.IsMasterNode || strings.EqualFold(strings.TrimSpace(os.Getenv("STRIPE_WALLET_RECONCILIATION_ENABLED")), "false") {
		return
	}
	stripeWalletTaskOnce.Do(func() {
		gopool.Go(func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				_, err := RunStripeWalletReconciliationOnce(ctx)
				cancel()
				if err != nil {
					logger.LogWarn(context.Background(), "Stripe wallet reconciliation: "+sanitizeDingTalkAlertText(err.Error()))
				}
				<-ticker.C
			}
		})
	})
}

func RunStripeWalletReconciliationOnce(ctx context.Context) (int, error) {
	if !common.IsMasterNode || strings.EqualFold(strings.TrimSpace(os.Getenv("STRIPE_WALLET_RECONCILIATION_ENABLED")), "false") {
		return 0, nil
	}
	key := strings.TrimSpace(setting.StripeApiSecret)
	if key == "" { // An installation without Stripe is not an outage.
		return 0, nil
	}
	r := &stripeWalletReconciler{
		gateway: newStripeWalletGateway(key),
		notify:  sendStripeWalletReconciliationNotification,
		now:     model.GetDBTimestampWithContext,
		prices:  setting.StripeTopUpAmountsByPriceID,
	}
	return r.run(ctx)
}

type stripeWalletReconciler struct {
	gateway   stripeWalletGateway
	notify    func(context.Context, stripeWalletNotification) error
	now       func(context.Context) (int64, error)
	prices    func() map[string]int64
	localOnly bool
}

func (r *stripeWalletReconciler) run(ctx context.Context) (int, error) {
	now, err := r.now(ctx)
	if err != nil {
		return 0, err
	}
	scopeCtx, scopeCancel := context.WithTimeout(ctx, 5*time.Second)
	scope, live, scopeErr := r.gateway.Scope(scopeCtx)
	scopeCancel()
	if scopeErr != nil {
		// Stable, non-secret identity permits persistent configuration-failure
		// throttling even when account lookup is unavailable.
		scope = stripeWalletConfigurationScope
	}
	token := common.GetUUID()
	state, claimed, err := model.AcquireStripeWalletScan(ctx, scope, now, stripeWalletLeaseSeconds, token)
	if err != nil || !claimed {
		return 0, err
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = model.ReleaseStripeWalletScan(releaseCtx, scope, token)
	}()
	if scopeErr != nil {
		// Already verified observations need only our database and DingTalk, not
		// a working Stripe account endpoint. Never hydrate using an unknown scope.
		scopes, listErr := model.ListActiveStripeWalletCheckScopes(ctx)
		local := *r
		local.localOnly = true
		followCtx, followCancel := context.WithTimeout(ctx, 18*time.Second)
		processed := 0
		for _, knownScope := range scopes {
			n, followErr := local.processChecks(followCtx, knownScope)
			processed += n
			listErr = errors.Join(listErr, followErr)
			if followCtx.Err() != nil {
				break
			}
		}
		followCancel()
		return processed, r.saveHealth(ctx, state, token, now, errors.Join(scopeErr, listErr))
	}
	configurationErr := r.clearConfigurationFailure(ctx, now)

	// Separate time budgets keep notification and hydration outages from
	// starving either discovery cursor. Ingest itself performs no Stripe reads.
	recentCtx, recentCancel := context.WithTimeout(ctx, 8*time.Second)
	recentErr := r.scan(recentCtx, state, token, live, now, false)
	recentCancel()
	historyCtx, historyCancel := context.WithTimeout(ctx, 8*time.Second)
	historyErr := r.scan(historyCtx, state, token, live, now, true)
	historyCancel()
	followCtx, followCancel := context.WithTimeout(ctx, 18*time.Second)
	processed, followErr := r.processChecks(followCtx, scope)
	followCancel()
	runErr := errors.Join(configurationErr, followErr, recentErr, historyErr)
	// Another replica may run between retries. No due work does not prove a
	// persisted verification outage recovered, so preserve its failure streak.
	verificationPending, verificationErr := model.HasStripeWalletVerificationFailures(ctx, scope)
	runErr = errors.Join(runErr, verificationErr)
	if verificationPending {
		runErr = errors.Join(runErr, errors.New("Stripe session verification failures remain unresolved"))
	}
	if state.RecentThrough < now-stripeWalletOverlapSeconds {
		runErr = errors.Join(runErr, errors.New("new payment scan is more than 10 minutes behind"))
	}
	return processed, r.saveHealth(ctx, state, token, now, runErr)
}

// A recovered account lookup ends its previous failure streak. Otherwise an
// unrelated later outage could inherit an old FailureSince and alert at once.
func (r *stripeWalletReconciler) clearConfigurationFailure(ctx context.Context, now int64) error {
	token := common.GetUUID()
	state, claimed, err := model.AcquireStripeWalletScan(ctx, stripeWalletConfigurationScope, now, stripeWalletLeaseSeconds, token)
	if err != nil || !claimed {
		return err
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = model.ReleaseStripeWalletScan(releaseCtx, state.Scope, token)
	}()
	state.FailureSince = 0
	state.LastFailureAttemptAt = 0
	state.LastSuccessAt = now
	return model.SaveStripeWalletScan(ctx, state, token)
}

func (r *stripeWalletReconciler) saveHealth(ctx context.Context, state *model.StripeWalletScanState, token string, now int64, runErr error) error {
	// Preserve failure evidence even if the run exhausted its request budget.
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		runErr = errors.Join(runErr, errors.New("reconciliation run timed out"))
	}
	if runErr == nil {
		state.FailureSince = 0
		state.LastSuccessAt = now
	} else {
		if state.FailureSince == 0 {
			state.FailureSince = now
		}
		if now-state.FailureSince >= stripeWalletOverlapSeconds &&
			(state.LastFailureAlertAt == 0 || now-state.LastFailureAlertAt >= stripeWalletReminder) &&
			(state.LastFailureAttemptAt == 0 || now-state.LastFailureAttemptAt >= stripeWalletNotifyRetry) {
			state.LastFailureAttemptAt = now
			if err := model.SaveStripeWalletScan(ctx, state, token); err != nil {
				return errors.Join(runErr, err)
			}
			err := r.notify(ctx, stripeWalletNotification{Kind: "task_error", Scope: state.Scope,
				FirstAnomalyAt: state.FailureSince, Detail: sanitizeDingTalkAlertText(runErr.Error())})
			if err == nil {
				state.LastFailureAlertAt = now
			} else {
				runErr = errors.Join(runErr, err)
			}
		}
	}
	return errors.Join(runErr, model.SaveStripeWalletScan(ctx, state, token))
}

func (r *stripeWalletReconciler) scan(ctx context.Context, state *model.StripeWalletScanState, token string, live bool, now int64, history bool) error {
	if history && state.BackfillDone {
		return nil
	}
	floor := state.InitialAt - stripeWalletHistorySeconds
	from, to, after := floor, state.InitialAt, state.BackfillAfter
	if !history {
		from = max(floor, state.RecentThrough-stripeWalletOverlapSeconds)
		if state.RecentWindowEnd == 0 {
			state.RecentWindowEnd = now
			if err := model.SaveStripeWalletScan(ctx, state, token); err != nil {
				return err
			}
		}
		to, after = state.RecentWindowEnd, state.RecentAfter
	}
	if from < now-stripeWalletRetention {
		return errors.New("Stripe event retention exceeded: uncovered payment interval requires manual backfill")
	}
	if to <= from {
		return nil
	}
	for pageNumber := 0; pageNumber < stripeWalletPageBudget; pageNumber++ {
		page, err := r.gateway.ListEvents(ctx, from, to, after)
		if err != nil {
			return fmt.Errorf("list Stripe payment events: %w", err)
		}
		for _, event := range page.Events {
			if event == nil || event.ID == "" || event.Created < from || event.Created >= to {
				return errors.New("Stripe event page contains an invalid event or timestamp")
			}
			if err := r.ingest(ctx, state.Scope, live, event, event.Created < state.InitialAt); err != nil {
				return err // No cursor advance until the entire page is durable.
			}
		}
		if page.HasMore {
			if len(page.Events) == 0 || page.Events[len(page.Events)-1].ID == after {
				return errors.New("Stripe event pagination did not advance")
			}
			after = page.Events[len(page.Events)-1].ID
		}
		if history {
			state.BackfillAfter = after
			state.BackfillDone = !page.HasMore
		} else if page.HasMore {
			state.RecentAfter = after
		} else {
			state.RecentThrough = to
			state.RecentWindowEnd, state.RecentAfter = 0, ""
		}
		if err := model.SaveStripeWalletScan(ctx, state, token); err != nil {
			return err
		}
		if !page.HasMore {
			return nil
		}
	}
	return nil // Durable page cursor resumes next minute; follow-ups stay fair.
}

func stripeWalletSubscriptionSession(s *stripe.CheckoutSession) bool {
	if s.Mode != stripe.CheckoutSessionModePayment || s.Subscription != nil {
		return true
	}
	for _, key := range []string{"plan_id", "newapi_plan_id", "purchase_intent", "purchase_months"} {
		if s.Metadata[key] != "" {
			return true
		}
	}
	return false
}

func stripeWalletReference(s *stripe.CheckoutSession) (string, bool) {
	client, metadata := strings.TrimSpace(s.ClientReferenceID), strings.TrimSpace(s.Metadata["trade_no"])
	if client != "" && metadata != "" && client != metadata {
		return "", false
	}
	if client == "" {
		client = metadata
	}
	return client, stripeWalletTradeNo.MatchString(client)
}

func (r *stripeWalletReconciler) ingest(ctx context.Context, scope string, live bool, event *stripe.Event, historical bool) error {
	if event.Type != stripe.EventTypeCheckoutSessionCompleted && event.Type != stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded {
		return nil
	}
	if event.Livemode != live {
		return r.quarantineEvent(ctx, scope, event, historical, "Stripe event livemode does not match configured account mode")
	}
	if event.Data == nil || len(event.Data.Raw) == 0 {
		return r.quarantineEvent(ctx, scope, event, historical, "Stripe payment event has no object payload")
	}
	// Raw decoding deliberately tolerates historical event API versions. Reading
	// the CURRENT Session must not turn an OLD unpaid completed event into paid.
	var snapshot stripe.CheckoutSession
	if err := common.Unmarshal(event.Data.Raw, &snapshot); err != nil {
		return r.quarantineEvent(ctx, scope, event, historical, "cannot decode historical Stripe Checkout Session")
	}
	if stripeWalletSubscriptionSession(&snapshot) || snapshot.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
		return nil
	}
	tradeNo, plausible := stripeWalletReference(&snapshot)
	if !plausible {
		if stripeWalletTradeNo.MatchString(snapshot.ClientReferenceID) || stripeWalletTradeNo.MatchString(snapshot.Metadata["trade_no"]) {
			return r.quarantineEvent(ctx, scope, event, historical, "Stripe wallet order reference conflict requires manual inspection")
		}
		return nil // Not an order emitted by this application's wallet checkout.
	}
	if snapshot.ID == "" || snapshot.Livemode != live {
		return r.quarantineEvent(ctx, scope, event, historical, "Stripe wallet session identity is invalid")
	}
	check := model.StripeWalletPaymentCheck{
		Scope: scope, SessionID: snapshot.ID, EventID: event.ID, TradeNo: tradeNo,
		PaidAt: event.Created, Amount: snapshot.AmountTotal, Currency: strings.ToUpper(string(snapshot.Currency)),
		Historical:  historical,
		NextCheckAt: event.Created + stripeWalletGraceSeconds,
	}
	return model.InsertStripeWalletPaymentCheck(ctx, &check)
}

func (r *stripeWalletReconciler) quarantineEvent(ctx context.Context, scope string, event *stripe.Event, historical bool, detail string) error {
	// Durable keyed evidence lets the cursor advance without losing or repeatedly
	// reparsing a poison event. No untrusted order association can mark it recovered.
	return model.InsertStripeWalletPaymentCheck(ctx, &model.StripeWalletPaymentCheck{
		Scope: scope, SessionID: "event:" + event.ID, EventID: event.ID,
		PaidAt: event.Created, NextCheckAt: event.Created + stripeWalletGraceSeconds,
		Historical:     historical,
		DiscoveryError: detail,
	})
}

func (r *stripeWalletReconciler) verifySession(ctx context.Context, check *model.StripeWalletPaymentCheck) error {
	if check.SessionVerified || check.WalletVerified || check.DiscoveryError != "" {
		return nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	current, err := r.gateway.GetSession(lookupCtx, check.SessionID)
	if err != nil {
		return fmt.Errorf("retrieve Stripe wallet session: %w", err)
	}
	if current == nil || current.ID != check.SessionID || current.Livemode != strings.HasSuffix(check.Scope, ":live") || stripeWalletSubscriptionSession(current) {
		return fmt.Errorf("%w: session snapshot identity changed", errStripeWalletIdentity)
	}
	tradeNo, ok := stripeWalletReference(current)
	if !ok || tradeNo != check.TradeNo || current.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
		return fmt.Errorf("%w: paid session association changed", errStripeWalletIdentity)
	}
	check.SessionVerified = true
	if current.LineItems != nil && !current.LineItems.HasMore && len(current.LineItems.Data) == 1 && current.LineItems.Data[0] != nil && current.LineItems.Data[0].Price != nil {
		check.PriceID = current.LineItems.Data[0].Price.ID
		_, check.WalletVerified = r.prices()[check.PriceID]
	}
	return nil
}

func (r *stripeWalletReconciler) localStatus(ctx context.Context, check *model.StripeWalletPaymentCheck) (string, error) {
	if check.DiscoveryError != "" {
		return "discovery_error", nil
	}
	if check.VerificationError != "" {
		return "stripe_check_failed", nil
	}
	topUp, err := model.GetStripeWalletTopUpForReconciliation(ctx, check.TradeNo)
	if errors.Is(err, model.ErrTopUpNotFound) {
		if check.WalletVerified {
			return "missing", nil
		}
		return "wallet_identity_unknown", nil
	}
	if err != nil {
		return "", err
	}
	provider := topUp.PaymentProvider
	if provider == "" {
		provider = topUp.PaymentMethod
	}
	if provider != model.PaymentProviderStripe || topUp.Amount <= 0 ||
		(topUp.GatewayTradeNo != "" && topUp.GatewayTradeNo != check.SessionID) ||
		(topUp.PaymentPriceId != "" && check.PriceID != "" && topUp.PaymentPriceId != check.PriceID) {
		return "identity_conflict", nil
	}
	check.WalletVerified, check.UserID = true, topUp.UserId
	return topUp.Status, nil
}

func (r *stripeWalletReconciler) processChecks(ctx context.Context, scope string) (int, error) {
	processed := 0
	var failures []error
	// Historical first checks cannot preempt new alerts or existing recoveries.
	// Each lane gets reserved time and work, including under a large backfill.
	for _, lane := range []string{"recent", "active", "history"} {
		laneCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
		n, err := r.processCheckLane(laneCtx, scope, lane)
		processed += n
		if err != nil {
			failures = append(failures, err)
		}
		cancel()
	}
	return processed, errors.Join(failures...)
}

func (r *stripeWalletReconciler) processCheckLane(ctx context.Context, scope, lane string) (int, error) {
	now, err := r.now(ctx)
	if err != nil {
		return 0, err
	}
	var failures []error
	processed := 0
	for processed < 33 {
		if err := ctx.Err(); err != nil {
			return processed, errors.Join(append(failures, err)...)
		}
		token := common.GetUUID()
		checks, err := model.ClaimStripeWalletPaymentChecksForLane(ctx, scope, now, stripeWalletLeaseSeconds, 1, token, lane)
		if err != nil {
			return processed, errors.Join(append(failures, err)...)
		}
		if len(checks) == 0 {
			break
		}
		check := &checks[0]
		itemCtx, itemCancel := context.WithTimeout(ctx, 10*time.Second)
		if err := r.processCheck(itemCtx, check, token); err != nil {
			failures = append(failures, err)
		}
		if itemCtx.Err() != nil {
			// Release only this owned item; no large batch of unprocessed leases
			// waits two minutes after a slow network call consumes its budget.
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			check.NextCheckAt = max(check.NextCheckAt, now+60)
			err := model.SaveStripeWalletPaymentCheck(cleanupCtx, check, token)
			if err != nil && !errors.Is(err, model.ErrStripeWalletReconciliationLeaseLost) {
				failures = append(failures, err)
			}
			cancel()
		}
		itemCancel()
		processed++
	}
	return processed, errors.Join(failures...)
}

func (r *stripeWalletReconciler) processCheck(ctx context.Context, check *model.StripeWalletPaymentCheck, token string) error {
	now, err := r.now(ctx)
	if err != nil {
		return err
	}
	check.NextCheckAt = max(now+60, check.PaidAt+stripeWalletGraceSeconds)
	if r.localOnly && !check.SessionVerified && !check.WalletVerified && check.DiscoveryError == "" {
		return model.SaveStripeWalletPaymentCheck(ctx, check, token)
	}
	if err := r.verifySession(ctx, check); err != nil {
		check.VerificationError = sanitizeDingTalkAlertText(err.Error())
		check.VerificationPending = !errors.Is(err, errStripeWalletIdentity)
		if check.VerificationPending {
			// An upstream read failure does not establish a local order anomaly.
			// Keep it retryable and feed the sustained task-health alarm instead.
			return errors.Join(err, model.SaveStripeWalletPaymentCheck(ctx, check, token))
		}
	} else {
		check.VerificationError = ""
		check.VerificationPending = false
	}
	status, statusErr := r.localStatus(ctx, check)
	if statusErr != nil {
		// A failed SELECT is not evidence that a local order is missing.
		return errors.Join(statusErr, model.SaveStripeWalletPaymentCheck(ctx, check, token))
	}
	check.LocalStatus = status
	if now < check.PaidAt+stripeWalletGraceSeconds {
		return model.SaveStripeWalletPaymentCheck(ctx, check, token)
	}
	if status == common.TopUpStatusSuccess && check.FirstAnomalyAt == 0 {
		check.ClosedAt = now // Normal payment, never an observed anomaly.
		return model.SaveStripeWalletPaymentCheck(ctx, check, token)
	}
	kind := ""
	if status == common.TopUpStatusSuccess || check.RecoveredAt != 0 {
		if check.RecoveredAt == 0 {
			check.RecoveredAt = now
		}
		kind = "recovered"
	} else {
		if check.FirstAnomalyAt == 0 {
			check.FirstAnomalyAt = now
		}
		if check.LastNotifiedAt == 0 {
			kind = "initial"
		} else if now-check.LastNotifiedAt >= stripeWalletReminder {
			kind = "reminder"
		}
	}
	if kind == "" {
		return model.SaveStripeWalletPaymentCheck(ctx, check, token)
	}
	// Persist the anomaly BEFORE delivery without releasing the lease. An initial
	// delivery failure followed by recovery must still produce a recovery notice.
	if err := model.CheckpointStripeWalletPaymentCheck(ctx, check, token); err != nil {
		return err
	}
	return r.deliverCheck(ctx, check, kind)
}

func (r *stripeWalletReconciler) deliverCheck(ctx context.Context, check *model.StripeWalletPaymentCheck, kind string) error {
	token := check.LeaseToken
	// Re-read immediately before sending so a callback that completed after the
	// initial observation cancels the stale failure message.
	status, err := r.localStatus(ctx, check)
	if err != nil {
		return errors.Join(err, model.SaveStripeWalletPaymentCheck(ctx, check, token))
	}
	now, err := r.now(ctx)
	if err != nil {
		return err
	}
	check.LocalStatus = status
	if status == common.TopUpStatusSuccess || check.RecoveredAt != 0 {
		kind = "recovered"
		if check.RecoveredAt == 0 {
			check.RecoveredAt = now
		}
	}
	if check.LastAttemptKind == kind && check.LastAttemptAt != 0 && now-check.LastAttemptAt < stripeWalletNotifyRetry {
		return model.SaveStripeWalletPaymentCheck(ctx, check, token)
	}
	check.LastAttemptAt, check.LastAttemptKind = now, kind
	if err := model.CheckpointStripeWalletPaymentCheck(ctx, check, token); err != nil {
		return err
	}
	sendErr := r.notify(ctx, stripeWalletNotification{
		Kind: kind, Scope: check.Scope, SessionID: check.SessionID, EventID: check.EventID,
		TradeNo: check.TradeNo, UserID: check.UserID, Amount: check.Amount, Currency: check.Currency,
		PaidAt: check.PaidAt, FirstAnomalyAt: check.FirstAnomalyAt, LocalStatus: check.LocalStatus,
		NotificationCount: check.NotificationCount,
		Detail:            strings.TrimSpace(check.DiscoveryError + " " + check.VerificationError),
	})
	if sendErr == nil {
		// Use the successful send time, not when the batch was first claimed.
		sentAt, clockErr := r.now(ctx)
		if clockErr != nil {
			return clockErr // Lease expires and retries; never pretend delivery was committed.
		}
		check.LastNotifiedAt = sentAt
		check.NotificationCount++
		if kind == "recovered" {
			check.ClosedAt = sentAt
		}
	}
	return errors.Join(sendErr, model.SaveStripeWalletPaymentCheck(ctx, check, token))
}
