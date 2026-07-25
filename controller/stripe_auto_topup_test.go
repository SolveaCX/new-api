package controller

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
	"gorm.io/gorm"
)

// --- Test harness ---

func setupAutoTopUpTestDB(t *testing.T) {
	t.Helper()
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedisEnabled := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedisEnabled
	})
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.TopUp{},
		&model.Log{},
	))
}

func stashAutoTopUpSettings(t *testing.T) {
	t.Helper()
	origSecret := setting.StripeApiSecret
	origUnitPrice := setting.StripeUnitPrice
	origDailyCap := setting.StripeAutoTopUpDailyMaxCharges
	origGlobalEnabled := setting.StripeAutoChargeEnabled
	origGlobalThreshold := setting.StripeAutoChargeThreshold
	origGlobalAmount := setting.StripeAutoChargeAmount
	origDisplayType := operation_setting.GetQuotaDisplayType()
	paymentSetting := operation_setting.GetPaymentSetting()
	origAmountOptions := append([]int(nil), paymentSetting.AmountOptions...)
	t.Cleanup(func() {
		setting.StripeApiSecret = origSecret
		setting.StripeUnitPrice = origUnitPrice
		setting.StripeAutoTopUpDailyMaxCharges = origDailyCap
		setting.StripeAutoChargeEnabled = origGlobalEnabled
		setting.StripeAutoChargeThreshold = origGlobalThreshold
		setting.StripeAutoChargeAmount = origGlobalAmount
		operation_setting.GetGeneralSetting().QuotaDisplayType = origDisplayType
		paymentSetting.AmountOptions = origAmountOptions
	})
	setting.StripeApiSecret = "sk_test_auto_topup"
	setting.StripeUnitPrice = 1.0
	setting.StripeAutoTopUpDailyMaxCharges = 2
	setting.StripeAutoChargeEnabled = false
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	paymentSetting.AmountOptions = []int{10, 20, 200}
}

// stubAutoChargeStripe replaces every Stripe seam so no test touches the network, and
// returns a counter of PaymentIntent creations.
func stubAutoChargeStripe(t *testing.T, intent *stripe.PaymentIntent, intentErr error) *int32 {
	t.Helper()
	origCreate := stripeAutoChargeCreatePaymentIntent
	origFind := stripeAutoChargeFindPaymentMethod
	origCurrency := stripeAutoChargeResolveCurrency
	t.Cleanup(func() {
		stripeAutoChargeCreatePaymentIntent = origCreate
		stripeAutoChargeFindPaymentMethod = origFind
		stripeAutoChargeResolveCurrency = origCurrency
	})
	var calls int32
	var mu sync.Mutex
	stripeAutoChargeCreatePaymentIntent = func(params *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		return intent, intentErr
	}
	stripeAutoChargeFindPaymentMethod = func(customerId string) string { return "pm_test_1" }
	stripeAutoChargeResolveCurrency = func() string { return "usd" }
	return &calls
}

// resetAutoChargeNodeState clears the per-node in-memory guards, simulating a different
// relay node (or a restarted one) evaluating the same user.
func resetAutoChargeNodeState() {
	autoChargeInFlight = sync.Map{}
	autoChargeLastAt = sync.Map{}
}

func insertAutoTopUpUser(t *testing.T, id int, optIn bool, quota int) {
	t.Helper()
	user := &model.User{
		Id:             id,
		Username:       "auto_topup_ctl",
		Status:         common.UserStatusEnabled,
		Quota:          quota,
		StripeCustomer: "cus_test_1",
	}
	user.StripeCardBound = true
	if optIn {
		user.SetSetting(dto.UserSetting{AutoTopUpEnabled: true, AutoTopUpThresholdUSD: 5, AutoTopUpAmountUSD: 20})
	}
	require.NoError(t, model.DB.Create(user).Error)
}

func autoTopUpUserSetting(t *testing.T, id int) dto.UserSetting {
	t.Helper()
	user, err := model.GetUserById(id, true)
	require.NoError(t, err)
	return user.GetSetting()
}

// --- Settings validation ---

func TestValidateAutoTopUpParams(t *testing.T) {
	stashAutoTopUpSettings(t)

	// Valid: preset amount, threshold within bounds.
	require.NoError(t, validateAutoTopUpParams(5, 20))

	// Threshold out of bounds.
	require.Error(t, validateAutoTopUpParams(0, 20))
	require.Error(t, validateAutoTopUpParams(setting.StripeAutoTopUpThresholdMaxUSD+1, 20))

	// Amount out of hard bounds.
	require.Error(t, validateAutoTopUpParams(5, 1))
	require.Error(t, validateAutoTopUpParams(5, setting.StripeAutoTopUpAmountMaxUSD+1))

	// Amount not one of the configured presets.
	require.Error(t, validateAutoTopUpParams(5, 15))

	// Without presets, min/max bounds still apply.
	operation_setting.GetPaymentSetting().AmountOptions = nil
	require.NoError(t, validateAutoTopUpParams(5, 15))
	require.Error(t, validateAutoTopUpParams(5, 2))
}

// --- Setting API handler ---

func autoTopUpSettingRequest(t *testing.T, userId int, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("PUT", "/api/user/stripe/auto_topup", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", userId)
	UpdateStripeAutoTopUpSetting(c)
	return recorder
}

func requireApiSuccess(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var resp map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, true, resp["success"], "expected success response, got %s", recorder.Body.String())
	return resp
}

func requireApiFailure(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var resp map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, false, resp["success"], "expected failure response, got %s", recorder.Body.String())
	message, _ := resp["message"].(string)
	return message
}

func TestUpdateStripeAutoTopUpSettingValidatesAndPersists(t *testing.T) {
	setupAutoTopUpTestDB(t)
	stashAutoTopUpSettings(t)

	const userId = 9101
	// User without a bound card.
	require.NoError(t, model.DB.Create(&model.User{Id: userId, Username: "auto_topup_api", Status: common.UserStatusEnabled}).Error)

	// Enabling without a bound card is refused.
	msg := requireApiFailure(t, autoTopUpSettingRequest(t, userId, `{"enabled":true,"threshold_usd":5,"amount_usd":20}`))
	require.Contains(t, msg, "saved card")

	// Bind a card, then invalid params are refused.
	require.NoError(t, model.SetStripeCardBound(userId, "cus_api_1", "fp_api_1"))
	msg = requireApiFailure(t, autoTopUpSettingRequest(t, userId, `{"enabled":true,"threshold_usd":0,"amount_usd":20}`))
	require.Contains(t, msg, "threshold")
	msg = requireApiFailure(t, autoTopUpSettingRequest(t, userId, `{"enabled":true,"threshold_usd":5,"amount_usd":15}`))
	require.Contains(t, msg, "preset")

	// Nothing was persisted by the refused writes.
	require.False(t, autoTopUpUserSetting(t, userId).AutoTopUpEnabled)

	// A valid enable persists.
	requireApiSuccess(t, autoTopUpSettingRequest(t, userId, `{"enabled":true,"threshold_usd":5,"amount_usd":20}`))
	persisted := autoTopUpUserSetting(t, userId)
	require.True(t, persisted.AutoTopUpEnabled)
	require.Equal(t, 5, persisted.AutoTopUpThresholdUSD)
	require.Equal(t, 20, persisted.AutoTopUpAmountUSD)

	// Disable succeeds and keeps threshold/amount for re-enabling.
	requireApiSuccess(t, autoTopUpSettingRequest(t, userId, `{"enabled":false}`))
	persisted = autoTopUpUserSetting(t, userId)
	require.False(t, persisted.AutoTopUpEnabled)
	require.Equal(t, 5, persisted.AutoTopUpThresholdUSD)

	// Operator kill switch refuses enabling.
	setting.StripeAutoTopUpDailyMaxCharges = 0
	msg = requireApiFailure(t, autoTopUpSettingRequest(t, userId, `{"enabled":true,"threshold_usd":5,"amount_usd":20}`))
	require.Contains(t, msg, "not available")
}

// --- Charge path ---

// TestPerformStripeAutoChargeSuccessCreditsQuotaOnce covers the happy path and the
// cross-node idempotency story: a successful off-session charge credits quota exactly
// once, and a second node evaluating the same episode (fresh in-memory state) performs
// no additional Stripe call because the DB-side claim refuses it.
func TestPerformStripeAutoChargeSuccessCreditsQuotaOnce(t *testing.T) {
	setupAutoTopUpTestDB(t)
	stashAutoTopUpSettings(t)
	calls := stubAutoChargeStripe(t, &stripe.PaymentIntent{ID: "pi_ok_1", Status: stripe.PaymentIntentStatusSucceeded}, nil)

	const userId = 9201
	insertAutoTopUpUser(t, userId, true, 0)

	resetAutoChargeNodeState()
	performStripeAutoCharge(userId)

	require.EqualValues(t, 1, *calls, "exactly one PaymentIntent must be created")
	var user model.User
	require.NoError(t, model.DB.Select("quota").Where("id = ?", userId).First(&user).Error)
	require.Equal(t, 20*int(common.QuotaPerUnit), user.Quota, "quota must be credited with the configured amount")

	var order model.TopUp
	require.NoError(t, model.DB.Where("user_id = ? AND payment_provider = ?", userId, model.PaymentProviderStripeAuto).First(&order).Error)
	require.Equal(t, common.TopUpStatusSuccess, order.Status)
	require.Equal(t, "pi_ok_1", order.GatewayTradeNo)
	require.True(t, strings.HasPrefix(order.TradeNo, "sauto_"), "order must use the deterministic episode trade_no, got %s", order.TradeNo)

	// Simulate a second node (fresh in-memory guards) racing the same episode window:
	// quota is back above the threshold AND the DB-side cooldown holds; either way no
	// second charge may happen. Force the interesting path by draining quota again.
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", userId).Update("quota", 0).Error)
	resetAutoChargeNodeState()
	performStripeAutoCharge(userId)
	require.EqualValues(t, 1, *calls, "a second node in the same episode must not create another PaymentIntent")
}

// TestPerformStripeAutoChargeConcurrentTriggersSingleCharge fires many concurrent
// triggers for the same user on one node and asserts a single charge attempt.
func TestPerformStripeAutoChargeConcurrentTriggersSingleCharge(t *testing.T) {
	setupAutoTopUpTestDB(t)
	stashAutoTopUpSettings(t)
	calls := stubAutoChargeStripe(t, &stripe.PaymentIntent{ID: "pi_ok_2", Status: stripe.PaymentIntentStatusSucceeded}, nil)

	const userId = 9202
	insertAutoTopUpUser(t, userId, true, 0)

	resetAutoChargeNodeState()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			performStripeAutoCharge(userId)
		}()
	}
	wg.Wait()

	require.EqualValues(t, 1, *calls, "concurrent triggers must collapse into one charge attempt")
	var orders int64
	require.NoError(t, model.DB.Model(&model.TopUp{}).Where("user_id = ? AND payment_provider = ?", userId, model.PaymentProviderStripeAuto).Count(&orders).Error)
	require.Equal(t, int64(1), orders)
}

// TestPerformStripeAutoChargeDeclineDisablesOptIn covers the failure policy: a
// definitive card decline marks the claim failed, disables the user's opt-in, records a
// user-visible log, and never retries.
func TestPerformStripeAutoChargeDeclineDisablesOptIn(t *testing.T) {
	setupAutoTopUpTestDB(t)
	stashAutoTopUpSettings(t)
	declineErr := &stripe.Error{Type: stripe.ErrorTypeCard, Code: stripe.ErrorCodeCardDeclined}
	calls := stubAutoChargeStripe(t, nil, declineErr)

	const userId = 9203
	insertAutoTopUpUser(t, userId, true, 0)

	resetAutoChargeNodeState()
	performStripeAutoCharge(userId)

	require.EqualValues(t, 1, *calls)

	// The claim row is failed (slot consumed → failure backoff).
	var order model.TopUp
	require.NoError(t, model.DB.Where("user_id = ? AND payment_provider = ?", userId, model.PaymentProviderStripeAuto).First(&order).Error)
	require.Equal(t, common.TopUpStatusFailed, order.Status)

	// The opt-in was disabled and the user notified.
	require.False(t, autoTopUpUserSetting(t, userId).AutoTopUpEnabled, "a definitive decline must disable the opt-in")
	var logs []*model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ? AND type = ?", userId, model.LogTypeSystem).Find(&logs).Error)
	var sawDisabledNotice bool
	for _, entry := range logs {
		if strings.Contains(entry.Content, "自动充值已因扣款失败自动关闭") {
			sawDisabledNotice = true
		}
	}
	require.True(t, sawDisabledNotice, "user must get a visible notice that auto top-up was turned off")

	// Even a fresh node never retries: the opt-in is off.
	resetAutoChargeNodeState()
	performStripeAutoCharge(userId)
	require.EqualValues(t, 1, *calls, "no retry loop after a decline")
}

// TestPerformStripeAutoChargeGlobalPathDeclineKeepsUserSetting verifies a decline on the
// legacy operator-level path backs off via the failed claim slot but does not touch the
// (absent) per-user opt-in.
func TestPerformStripeAutoChargeGlobalPathDeclineKeepsUserSetting(t *testing.T) {
	setupAutoTopUpTestDB(t)
	stashAutoTopUpSettings(t)
	setting.StripeAutoChargeEnabled = true
	setting.StripeAutoChargeThreshold = 2
	setting.StripeAutoChargeAmount = 20
	declineErr := &stripe.Error{Type: stripe.ErrorTypeCard, Code: stripe.ErrorCodeCardDeclined}
	calls := stubAutoChargeStripe(t, nil, declineErr)

	const userId = 9204
	insertAutoTopUpUser(t, userId, false, 0)

	resetAutoChargeNodeState()
	performStripeAutoCharge(userId)
	require.EqualValues(t, 1, *calls)

	var order model.TopUp
	require.NoError(t, model.DB.Where("user_id = ? AND payment_provider = ?", userId, model.PaymentProviderStripeAuto).First(&order).Error)
	require.Equal(t, common.TopUpStatusFailed, order.Status)
	require.False(t, autoTopUpUserSetting(t, userId).AutoTopUpEnabled)

	// A fresh node inside the cooldown window must not charge again (failed slot armed it).
	resetAutoChargeNodeState()
	performStripeAutoCharge(userId)
	require.EqualValues(t, 1, *calls)
}

// TestPerformStripeAutoChargeSkipsUsersWithoutConfig verifies users with neither an
// opt-in nor the global flag never reach Stripe.
func TestPerformStripeAutoChargeSkipsUsersWithoutConfig(t *testing.T) {
	setupAutoTopUpTestDB(t)
	stashAutoTopUpSettings(t)
	calls := stubAutoChargeStripe(t, &stripe.PaymentIntent{ID: "pi_never", Status: stripe.PaymentIntentStatusSucceeded}, nil)

	const userId = 9205
	insertAutoTopUpUser(t, userId, false, 0)

	resetAutoChargeNodeState()
	performStripeAutoCharge(userId)
	require.EqualValues(t, 0, *calls, "non-opted-in user without global path must never be charged")
}
