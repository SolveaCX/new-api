package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func seedMediaBillingSubscription(t *testing.T, userID, planID, subscriptionID, walletQuota int, mediaTotal, mediaUsed int64) {
	t.Helper()
	resetBillingStatusTables(t)
	require.NoError(t, model.DB.Migrator().DropTable(&model.SubscriptionPlan{}))
	require.NoError(t, model.DB.AutoMigrate(&model.SubscriptionPlan{}))
	require.NoError(t, model.DB.Exec("DELETE FROM subscription_plans").Error)
	seedUser(t, userID, walletQuota)
	require.NoError(t, model.DB.Create(&model.SubscriptionPlan{
		Id:                  planID,
		Title:               "Media Plan",
		PriceAmount:         10,
		Currency:            "USD",
		DurationUnit:        model.SubscriptionDurationMonth,
		DurationValue:       1,
		Enabled:             true,
		TotalAmount:         10_000,
		MediaCreditsMonthly: mediaTotal,
	}).Error)
	require.NoError(t, model.DB.Create(&model.UserSubscription{
		Id:                subscriptionID,
		UserId:            userID,
		PlanId:            planID,
		AmountTotal:       10_000,
		AmountUsed:        77,
		MediaCreditsTotal: mediaTotal,
		MediaCreditsUsed:  mediaUsed,
		StartTime:         time.Now().Add(-time.Hour).Unix(),
		EndTime:           time.Now().Add(30 * 24 * time.Hour).Unix(),
		Status:            model.SubscriptionEntitlementStatusActive,
		Source:            "test",
	}).Error)
}

func newImage2BillingRelayInfo(userID int, requestID string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RequestId:       requestID,
		UserId:          userID,
		OriginModelName: "gpt-image-2",
		UsingGroup:      "default",
		UserGroup:       "default",
		IsPlayground:    true,
	}
}

func TestMediaCreditsForQuotaRoundsUpByCent(t *testing.T) {
	require.Equal(t, int64(1), mediaCreditsForQuota(1))
	require.Equal(t, int64(1), mediaCreditsForQuota(3_161))
	require.Equal(t, int64(1), mediaCreditsForQuota(5_000))
	require.Equal(t, int64(2), mediaCreditsForQuota(5_001))
	require.Zero(t, mediaCreditsForQuota(0))
}

func TestMediaCreditsForQuotaAvoidsFloatCeilDrift(t *testing.T) {
	original := common.QuotaPerUnit
	common.QuotaPerUnit = 5
	defer func() {
		common.QuotaPerUnit = original
	}()

	require.Equal(t, int64(220), mediaCreditsForQuota(11))
}

func TestImage2SubscriptionMediaModelAliases(t *testing.T) {
	require.True(t, isSubscriptionMediaModel("gpt-image-2"))
	require.True(t, isSubscriptionMediaModel("openai/gpt-image-2"))
	require.True(t, isSubscriptionMediaModel("gpt-image-2-low"))
	require.False(t, isSubscriptionMediaModel("gpt-image-1"))
}

func TestImage2SubscriptionConsumesOnlyMediaCredits(t *testing.T) {
	const (
		userID         = 9701
		planID         = 9801
		subscriptionID = 9901
		walletQuota    = 25_000
	)
	seedMediaBillingSubscription(t, userID, planID, subscriptionID, walletQuota, 300, 0)

	info := newImage2BillingRelayInfo(userID, "image2-media-success")
	session, apiErr := NewBillingSession(newTestGinContext(), info, 3_161)
	require.Nil(t, apiErr)
	require.IsType(t, &SubscriptionFunding{}, session.funding)
	require.Equal(t, BillingSourceSubscription, info.BillingSource)
	require.Equal(t, model.SubscriptionPoolMedia, info.SubscriptionPoolType)

	var sub model.UserSubscription
	require.NoError(t, model.DB.First(&sub, subscriptionID).Error)
	require.Equal(t, int64(77), sub.AmountUsed)
	require.Equal(t, int64(1), sub.MediaCreditsUsed)
	userQuota, err := model.GetUserQuota(userID, false)
	require.NoError(t, err)
	require.Equal(t, walletQuota, userQuota)

	require.NoError(t, session.Settle(5_001))
	require.NoError(t, session.Settle(5_001), "settlement retry must be idempotent")
	require.NoError(t, model.DB.First(&sub, subscriptionID).Error)
	require.Equal(t, int64(77), sub.AmountUsed)
	require.Equal(t, int64(2), sub.MediaCreditsUsed)
	require.Equal(t, int64(1), info.SubscriptionPostDelta)

	other := map[string]interface{}{}
	appendBillingInfo(info, other)
	require.Equal(t, model.SubscriptionPoolMedia, other["subscription_pool_type"])
}

func TestImage2SubscriptionRefundRestoresMediaCredits(t *testing.T) {
	const (
		userID         = 9702
		planID         = 9802
		subscriptionID = 9902
	)
	seedMediaBillingSubscription(t, userID, planID, subscriptionID, 25_000, 300, 0)

	info := newImage2BillingRelayInfo(userID, "image2-media-refund")
	session, apiErr := NewBillingSession(newTestGinContext(), info, 5_001)
	require.Nil(t, apiErr)
	require.NoError(t, session.funding.Refund())
	require.NoError(t, session.funding.Refund())

	var sub model.UserSubscription
	require.NoError(t, model.DB.First(&sub, subscriptionID).Error)
	require.Equal(t, int64(77), sub.AmountUsed)
	require.Zero(t, sub.MediaCreditsUsed)
}

func TestImage2MediaExhaustionFallsBackToWallet(t *testing.T) {
	const (
		userID         = 9703
		planID         = 9803
		subscriptionID = 9903
		walletQuota    = 25_000
	)
	seedMediaBillingSubscription(t, userID, planID, subscriptionID, walletQuota, 0, 0)

	info := newImage2BillingRelayInfo(userID, "image2-media-wallet-fallback")
	session, apiErr := NewBillingSession(newTestGinContext(), info, 3_161)
	require.Nil(t, apiErr)
	require.IsType(t, &WalletFunding{}, session.funding)
	require.Equal(t, BillingSourceWallet, info.BillingSource)

	var sub model.UserSubscription
	require.NoError(t, model.DB.First(&sub, subscriptionID).Error)
	require.Equal(t, int64(77), sub.AmountUsed)
	require.Zero(t, sub.MediaCreditsUsed)
	userQuota, err := model.GetUserQuota(userID, false)
	require.NoError(t, err)
	require.Equal(t, walletQuota-3_161, userQuota)
}
