package service

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStripeCheckoutInitialRequestIDKeepsLegacyShortIDsAndBoundsLongTradeNos(t *testing.T) {
	shortTradeNo := "SUBSTRUSR1INT2NOabc123456789"
	require.Equal(t, "initial:recurring_subscription:"+shortTradeNo,
		StripeCheckoutInitialRequestID(StripeCheckoutPurchaseRecurringSubscription, shortTradeNo))

	longTradeNo := strings.Repeat("x", 200)
	got := StripeCheckoutInitialRequestID(StripeCheckoutPurchaseRecurringSubscription, longTradeNo)
	require.Len(t, got, 64)
	require.Equal(t, got, StripeCheckoutInitialRequestID(StripeCheckoutPurchaseRecurringSubscription, longTradeNo))
	require.NotEqual(t, got, StripeCheckoutInitialRequestID(StripeCheckoutPurchaseOneTimeSubscription, longTradeNo))
}

func TestStripeCheckoutContextRoundTripAndTamperFence(t *testing.T) {
	now := time.Unix(1_787_200_000, 0)
	claims := StripeCheckoutContextClaims{
		UserID:       19,
		PurchaseKind: StripeCheckoutPurchaseTopUp,
		TradeNo:      "topup-19",
		Revision:     3,
		ExpiresAt:    now.Add(15 * time.Minute).Unix(),
	}

	token, err := SignStripeCheckoutContext(claims)
	require.NoError(t, err)

	got, err := VerifyStripeCheckoutContext(token, now)
	require.NoError(t, err)
	require.Equal(t, claims, got)

	_, err = VerifyStripeCheckoutContext(token+"x", now)
	require.ErrorIs(t, err, ErrStripeCheckoutContextInvalid)
}

func TestStripeCheckoutContextRejectsExpired(t *testing.T) {
	now := time.Unix(1_787_200_000, 0)
	token, err := SignStripeCheckoutContext(StripeCheckoutContextClaims{
		UserID:       19,
		PurchaseKind: StripeCheckoutPurchaseOneTimeSubscription,
		TradeNo:      "sub-19",
		Revision:     1,
		ExpiresAt:    now.Add(-time.Second).Unix(),
	})
	require.NoError(t, err)

	_, err = VerifyStripeCheckoutContext(token, now)
	require.ErrorIs(t, err, ErrStripeCheckoutContextExpired)
}
