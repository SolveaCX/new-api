package model

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func seedSubscriptionMediaFixture(t *testing.T, userID, planID, subscriptionID int, mediaTotal, mediaUsed int64) {
	t.Helper()
	setupSubscriptionEntitlementTestDB(t)
	createEntitlementTestUser(t, userID, "default")
	createEntitlementTestPlan(t, planID, 10_000, "")
	require.NoError(t, DB.Create(&UserSubscription{
		Id:                subscriptionID,
		UserId:            userID,
		PlanId:            planID,
		AmountTotal:       10_000,
		AmountUsed:        77,
		MediaCreditsTotal: mediaTotal,
		MediaCreditsUsed:  mediaUsed,
		StartTime:         time.Now().Add(-time.Hour).Unix(),
		EndTime:           time.Now().Add(30 * 24 * time.Hour).Unix(),
		Status:            SubscriptionEntitlementStatusActive,
		Source:            "test",
	}).Error)
}

func TestSubscriptionMediaPreConsumeSettleAndRefundAreIdempotent(t *testing.T) {
	const (
		userID         = 9401
		planID         = 9501
		subscriptionID = 9601
		requestID      = "media-idempotent-request"
	)
	seedSubscriptionMediaFixture(t, userID, planID, subscriptionID, 300, 0)

	result, err := PreConsumeUserSubscriptionMedia(requestID, userID, "gpt-image-2", 1)
	require.NoError(t, err)
	require.Equal(t, int64(1), result.PreConsumed)
	require.Equal(t, int64(300), result.AmountTotal)
	require.Equal(t, int64(1), result.AmountUsedAfter)

	replay, err := PreConsumeUserSubscriptionMedia(requestID, userID, "gpt-image-2", 1)
	require.NoError(t, err)
	require.Equal(t, result.UserSubscriptionId, replay.UserSubscriptionId)
	require.Equal(t, result.PreConsumed, replay.PreConsumed)
	require.Equal(t, result.AmountTotal, replay.AmountTotal)

	var sub UserSubscription
	require.NoError(t, DB.First(&sub, subscriptionID).Error)
	require.Equal(t, int64(77), sub.AmountUsed, "media requests must not consume the general pool")
	require.Equal(t, int64(1), sub.MediaCreditsUsed, "replayed pre-consume must not double charge")

	require.NoError(t, SetUserSubscriptionMediaPreConsume(requestID, 2))
	require.NoError(t, SetUserSubscriptionMediaPreConsume(requestID, 2))

	require.NoError(t, DB.First(&sub, subscriptionID).Error)
	require.Equal(t, int64(77), sub.AmountUsed, "media requests must not consume the general pool")
	require.Equal(t, int64(2), sub.MediaCreditsUsed)

	var record SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", requestID).First(&record).Error)
	require.Equal(t, SubscriptionPoolMedia, record.PoolType)
	require.Equal(t, int64(2), record.PreConsumed)

	require.NoError(t, RefundSubscriptionPreConsume(requestID))
	require.NoError(t, RefundSubscriptionPreConsume(requestID))
	require.NoError(t, DB.First(&sub, subscriptionID).Error)
	require.Equal(t, int64(77), sub.AmountUsed)
	require.Zero(t, sub.MediaCreditsUsed)
}

func TestSubscriptionMediaInsufficientUsesDedicatedSentinel(t *testing.T) {
	const (
		userID         = 9402
		planID         = 9502
		subscriptionID = 9602
	)
	seedSubscriptionMediaFixture(t, userID, planID, subscriptionID, 3, 2)

	_, err := PreConsumeUserSubscriptionMedia("media-insufficient-request", userID, "gpt-image-2", 2)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrSubscriptionMediaCreditsInsufficient))

	var sub UserSubscription
	require.NoError(t, DB.First(&sub, subscriptionID).Error)
	require.Equal(t, int64(77), sub.AmountUsed)
	require.Equal(t, int64(2), sub.MediaCreditsUsed)
}
