package service

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

type autoChargeHookRecorder struct {
	mu    sync.Mutex
	fired []int
	done  chan struct{}
}

func installAutoChargeHookRecorder(t *testing.T) *autoChargeHookRecorder {
	t.Helper()
	origHook := TriggerStripeAutoCharge
	t.Cleanup(func() { TriggerStripeAutoCharge = origHook })
	rec := &autoChargeHookRecorder{done: make(chan struct{}, 16)}
	TriggerStripeAutoCharge = func(userId int) {
		rec.mu.Lock()
		rec.fired = append(rec.fired, userId)
		rec.mu.Unlock()
		rec.done <- struct{}{}
	}
	return rec
}

func (rec *autoChargeHookRecorder) waitOne(t *testing.T) {
	t.Helper()
	select {
	case <-rec.done:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected auto-charge hook to fire")
	}
}

func (rec *autoChargeHookRecorder) firedUsers() []int {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return append([]int(nil), rec.fired...)
}

func autoChargeTestContext(userSetting *dto.UserSetting) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if userSetting != nil {
		common.SetContextKey(c, constant.ContextKeyUserSetting, *userSetting)
	}
	return c
}

func stashAutoChargeSettings(t *testing.T) {
	t.Helper()
	origEnabled := setting.StripeAutoChargeEnabled
	origThreshold := setting.StripeAutoChargeThreshold
	origDailyCap := setting.StripeAutoTopUpDailyMaxCharges
	t.Cleanup(func() {
		setting.StripeAutoChargeEnabled = origEnabled
		setting.StripeAutoChargeThreshold = origThreshold
		setting.StripeAutoTopUpDailyMaxCharges = origDailyCap
	})
}

// TestMaybeTriggerStripeAutoChargeGlobalGating verifies the legacy operator-level
// enabled/threshold gating still fires the hook only under the right conditions.
func TestMaybeTriggerStripeAutoChargeGlobalGating(t *testing.T) {
	stashAutoChargeSettings(t)
	rec := installAutoChargeHookRecorder(t)

	// Per-user path off so only the global path is exercised. Cached "not opted in"
	// setting on the context keeps the per-user fallback threshold out of the picture.
	setting.StripeAutoTopUpDailyMaxCharges = 0
	notOptedIn := &dto.UserSetting{}

	threshold := 2
	setting.StripeAutoChargeThreshold = threshold
	belowThreshold := threshold*int(common.QuotaPerUnit) - 1
	aboveThreshold := threshold * int(common.QuotaPerUnit)

	// Disabled => never fires.
	setting.StripeAutoChargeEnabled = false
	MaybeTriggerStripeAutoCharge(autoChargeTestContext(notOptedIn), 101, belowThreshold)

	// Enabled but balance above threshold => never fires.
	setting.StripeAutoChargeEnabled = true
	MaybeTriggerStripeAutoCharge(autoChargeTestContext(notOptedIn), 102, aboveThreshold)

	// Enabled and below threshold => fires (async).
	MaybeTriggerStripeAutoCharge(autoChargeTestContext(notOptedIn), 103, belowThreshold)
	rec.waitOne(t)

	if fired := rec.firedUsers(); len(fired) != 1 || fired[0] != 103 {
		t.Fatalf("expected only user 103 to trigger auto-charge, got %v", fired)
	}
}

// TestMaybeTriggerStripeAutoChargePerUserOptIn verifies the per-user opt-in path fires
// on the user's own threshold, and never fires for users who did not opt in.
func TestMaybeTriggerStripeAutoChargePerUserOptIn(t *testing.T) {
	stashAutoChargeSettings(t)
	rec := installAutoChargeHookRecorder(t)

	setting.StripeAutoChargeEnabled = false
	setting.StripeAutoTopUpDailyMaxCharges = 2

	optedIn := &dto.UserSetting{AutoTopUpEnabled: true, AutoTopUpThresholdUSD: 5, AutoTopUpAmountUSD: 20}
	notOptedIn := &dto.UserSetting{}

	below := 5*int(common.QuotaPerUnit) - 1
	above := 5 * int(common.QuotaPerUnit)

	// Not opted in (cached setting present) => never fires, regardless of balance.
	MaybeTriggerStripeAutoCharge(autoChargeTestContext(notOptedIn), 201, below)

	// Opted in but above own threshold => never fires.
	MaybeTriggerStripeAutoCharge(autoChargeTestContext(optedIn), 202, above)

	// Opted in and below own threshold => fires.
	MaybeTriggerStripeAutoCharge(autoChargeTestContext(optedIn), 203, below)
	rec.waitOne(t)

	// No cached setting on the context => conservative trigger below the max permitted
	// threshold (the async trigger re-checks the real opt-in).
	MaybeTriggerStripeAutoCharge(autoChargeTestContext(nil), 204, below)
	rec.waitOne(t)

	if fired := rec.firedUsers(); len(fired) != 2 || fired[0] != 203 || fired[1] != 204 {
		t.Fatalf("expected users 203 and 204 to trigger auto-charge, got %v", fired)
	}
}

// TestMaybeTriggerStripeAutoChargeEvalThrottle verifies the per-node evaluation throttle
// collapses rapid repeated triggers for the same user into one hook invocation.
func TestMaybeTriggerStripeAutoChargeEvalThrottle(t *testing.T) {
	stashAutoChargeSettings(t)
	rec := installAutoChargeHookRecorder(t)

	setting.StripeAutoChargeEnabled = true
	setting.StripeAutoChargeThreshold = 2
	setting.StripeAutoTopUpDailyMaxCharges = 0
	notOptedIn := &dto.UserSetting{}
	below := 2*int(common.QuotaPerUnit) - 1

	for i := 0; i < 5; i++ {
		MaybeTriggerStripeAutoCharge(autoChargeTestContext(notOptedIn), 301, below)
	}
	rec.waitOne(t)
	// Give any stray goroutines a moment to surface before asserting.
	time.Sleep(50 * time.Millisecond)

	if fired := rec.firedUsers(); len(fired) != 1 || fired[0] != 301 {
		t.Fatalf("expected the throttle to collapse repeats into one trigger, got %v", fired)
	}
}
