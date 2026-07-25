package model

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"

	"github.com/stretchr/testify/require"
)

// TestClaimStripeAutoTopUpEpisodeConcurrentSingleWinner is the core Rule 11 test: many
// concurrent claimers (simulating relay nodes that all observed the same exhaustion
// episode) must produce exactly ONE claimed order — the trade_no unique index plus the
// single-statement snapshot guarantee it — so exactly one Stripe charge attempt happens.
func TestClaimStripeAutoTopUpEpisodeConcurrentSingleWinner(t *testing.T) {
	truncateTables(t)
	DB.Exec("DELETE FROM top_ups")

	const userId = 8801
	const day = "20260714"
	const dailyCap = 1
	const cooldown int64 = 120

	var wg sync.WaitGroup
	var mu sync.Mutex
	winners := 0
	var claimErrs []error
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, claimed, err := ClaimStripeAutoTopUpEpisode(userId, day, dailyCap, cooldown, 20, 20)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				claimErrs = append(claimErrs, err)
				return
			}
			if claimed {
				winners++
			}
		}()
	}
	wg.Wait()

	require.Empty(t, claimErrs, "claims must not error")
	require.Equal(t, 1, winners, "exactly one concurrent claimer may win the episode")

	var rows int64
	require.NoError(t, DB.Model(&TopUp{}).Where("user_id = ? AND payment_provider = ?", userId, PaymentProviderStripeAuto).Count(&rows).Error)
	require.Equal(t, int64(1), rows, "exactly one claim row may exist")
}

// TestClaimStripeAutoTopUpEpisodeDailyCapAndCooldown verifies the two episode brakes:
// consecutive claims inside the cooldown window are refused, and the daily cap bounds
// the number of slots even with cooldown disabled.
func TestClaimStripeAutoTopUpEpisodeDailyCapAndCooldown(t *testing.T) {
	truncateTables(t)
	DB.Exec("DELETE FROM top_ups")

	const userId = 8802
	const day = "20260714"

	// Cooldown: second claim right after the first is refused.
	first, claimed, err := ClaimStripeAutoTopUpEpisode(userId, day, 5, 120, 20, 20)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NotNil(t, first)
	_, claimed, err = ClaimStripeAutoTopUpEpisode(userId, day, 5, 120, 20, 20)
	require.NoError(t, err)
	require.False(t, claimed, "claim inside the cooldown window must be refused")

	// Daily cap: with cooldown off, slots are granted until the cap, then refused —
	// including after a failed charge (failed rows keep their slot: failure backoff).
	require.NoError(t, MarkStripeAutoTopUpOrderFailed(first.TradeNo, ""))
	second, claimed, err := ClaimStripeAutoTopUpEpisode(userId, day, 2, 0, 20, 20)
	require.NoError(t, err)
	require.True(t, claimed, "second slot must be claimable below the cap")
	require.NotEqual(t, first.TradeNo, second.TradeNo)
	_, claimed, err = ClaimStripeAutoTopUpEpisode(userId, day, 2, 0, 20, 20)
	require.NoError(t, err)
	require.False(t, claimed, "daily cap must refuse further slots")

	// A new day opens fresh slots.
	_, claimed, err = ClaimStripeAutoTopUpEpisode(userId, "20260715", 2, 0, 20, 20)
	require.NoError(t, err)
	require.True(t, claimed)
}

// TestCompleteStripeAutoTopUpOrderCreditsExactlyOnce verifies completing a claimed order
// credits quota once, records the gateway trade no, and is idempotent on replay.
func TestCompleteStripeAutoTopUpOrderCreditsExactlyOnce(t *testing.T) {
	truncateTables(t)
	DB.Exec("DELETE FROM top_ups")

	const userId = 8803
	require.NoError(t, DB.Create(&User{Id: userId, Username: "auto_topup_credit", Status: common.UserStatusEnabled, Quota: 100}).Error)

	order, claimed, err := ClaimStripeAutoTopUpEpisode(userId, "20260714", 2, 120, 20, 19.99)
	require.NoError(t, err)
	require.True(t, claimed)

	require.NoError(t, CompleteStripeAutoTopUpOrder(order.TradeNo, "pi_test_123", "127.0.0.1"))
	// Replay (e.g. a duplicate invocation) must not credit twice.
	require.NoError(t, CompleteStripeAutoTopUpOrder(order.TradeNo, "pi_test_123", "127.0.0.1"))

	var user User
	require.NoError(t, DB.Select("quota").Where("id = ?", userId).First(&user).Error)
	require.Equal(t, 100+20*int(common.QuotaPerUnit), user.Quota, "quota must be credited exactly once")

	reloaded := GetTopUpByTradeNo(order.TradeNo)
	require.NotNil(t, reloaded)
	require.Equal(t, common.TopUpStatusSuccess, reloaded.Status)
	require.Equal(t, "pi_test_123", reloaded.GatewayTradeNo)
	require.NotZero(t, reloaded.CompleteTime)
}

// TestCompleteStripeAutoTopUpOrderRejectsForeignOrders verifies the completion path only
// touches stripe_auto orders and refuses non-pending states other than success.
func TestCompleteStripeAutoTopUpOrderRejectsForeignOrders(t *testing.T) {
	truncateTables(t)
	DB.Exec("DELETE FROM top_ups")

	require.NoError(t, DB.Create(&TopUp{
		UserId:          8804,
		Amount:          20,
		TradeNo:         "manual_order_1",
		PaymentMethod:   PaymentMethodStripe,
		PaymentProvider: PaymentProviderStripe,
		Status:          common.TopUpStatusPending,
		CreateTime:      common.GetTimestamp(),
	}).Error)
	require.ErrorIs(t, CompleteStripeAutoTopUpOrder("manual_order_1", "pi_x", ""), ErrPaymentMethodMismatch)

	order, claimed, err := ClaimStripeAutoTopUpEpisode(8804, "20260714", 2, 0, 20, 20)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, MarkStripeAutoTopUpOrderFailed(order.TradeNo, ""))
	require.ErrorIs(t, CompleteStripeAutoTopUpOrder(order.TradeNo, "pi_x", ""), ErrTopUpStatusInvalid)
	require.ErrorIs(t, CompleteStripeAutoTopUpOrder("missing_order", "pi_x", ""), ErrTopUpNotFound)
}

// TestDisableUserAutoTopUpSetting verifies the decline path helper: the opt-in flag is
// cleared, unrelated settings survive, and repeat calls are no-ops.
func TestDisableUserAutoTopUpSetting(t *testing.T) {
	truncateTables(t)
	DB.Exec("DELETE FROM users")

	const userId = 8805
	user := &User{Id: userId, Username: "auto_topup_disable", Status: common.UserStatusEnabled}
	user.SetSetting(dto.UserSetting{
		NotifyType:            dto.NotifyTypeEmail,
		QuotaWarningThreshold: 3,
		Language:              "en",
		AutoTopUpEnabled:      true,
		AutoTopUpThresholdUSD: 5,
		AutoTopUpAmountUSD:    20,
	})
	require.NoError(t, DB.Create(user).Error)

	changed, err := DisableUserAutoTopUpSetting(userId)
	require.NoError(t, err)
	require.True(t, changed)

	reloaded, err := GetUserById(userId, true)
	require.NoError(t, err)
	reloadedSetting := reloaded.GetSetting()
	require.False(t, reloadedSetting.AutoTopUpEnabled, "opt-in flag must be cleared")
	require.Equal(t, "en", reloadedSetting.Language, "unrelated settings must survive")
	require.Equal(t, dto.NotifyTypeEmail, reloadedSetting.NotifyType)
	require.Equal(t, 5, reloadedSetting.AutoTopUpThresholdUSD, "stored threshold survives for re-enabling")

	changed, err = DisableUserAutoTopUpSetting(userId)
	require.NoError(t, err)
	require.False(t, changed, "second disable must be a no-op")
}
