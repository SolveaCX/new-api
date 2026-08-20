package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestStripeCheckoutRevisionPrepareIsIdempotentAndFenced(t *testing.T) {
	setupStripeCheckoutRevisionTestDB(t)
	topUp := TopUp{
		UserId:           7,
		TradeNo:          "t-7",
		GatewayTradeNo:   "cs_old",
		Status:           common.TopUpStatusPending,
		CheckoutRevision: 1,
	}
	require.NoError(t, DB.Create(&topUp).Error)

	first, replay, err := PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
		OrderType:        StripeCheckoutOrderTopUp,
		TradeNo:          "t-7",
		UserID:           7,
		ExpectedRevision: 1,
		RequestID:        "req-1",
		SelectionDigest:  "sha256:manual-promo-1",
	})
	require.NoError(t, err)
	require.False(t, replay)
	require.Equal(t, int64(2), first.Revision)
	require.Equal(t, StripeCheckoutRevisionStatePreparing, first.State)
	require.Nil(t, first.ProviderSessionId)

	second, replay, err := PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
		OrderType:        StripeCheckoutOrderTopUp,
		TradeNo:          "t-7",
		UserID:           7,
		ExpectedRevision: 1,
		RequestID:        "req-1",
		SelectionDigest:  "sha256:manual-promo-1",
	})
	require.NoError(t, err)
	require.True(t, replay)
	require.Equal(t, first.Id, second.Id)

	_, _, err = PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
		OrderType:        StripeCheckoutOrderTopUp,
		TradeNo:          "t-7",
		UserID:           7,
		ExpectedRevision: 1,
		RequestID:        "req-2",
		SelectionDigest:  "sha256:manual-promo-2",
	})
	require.ErrorIs(t, err, ErrStripeCheckoutRevisionConflict)
}

func TestStripeCheckoutRevisionRejectsRequestReplayWithDifferentDigest(t *testing.T) {
	setupStripeCheckoutRevisionTestDB(t)
	require.NoError(t, DB.Create(&TopUp{
		UserId:           8,
		TradeNo:          "t-digest",
		GatewayTradeNo:   "cs_digest_old",
		Status:           common.TopUpStatusPending,
		CheckoutRevision: 1,
	}).Error)

	_, _, err := PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
		OrderType:        StripeCheckoutOrderTopUp,
		TradeNo:          "t-digest",
		UserID:           8,
		ExpectedRevision: 1,
		RequestID:        "req-digest",
		SelectionDigest:  "sha256:first",
	})
	require.NoError(t, err)

	_, replay, err := PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
		OrderType:        StripeCheckoutOrderTopUp,
		TradeNo:          "t-digest",
		UserID:           8,
		ExpectedRevision: 1,
		RequestID:        "req-digest",
		SelectionDigest:  "sha256:second",
	})
	require.False(t, replay)
	require.ErrorIs(t, err, ErrStripeCheckoutRevisionConflict)
}

func TestStripeCheckoutRevisionActivateMovesPointerAndSupersedesExactlyOnce(t *testing.T) {
	setupStripeCheckoutRevisionTestDB(t)
	require.NoError(t, DB.Create(&TopUp{
		UserId:           9,
		TradeNo:          "t-activate",
		GatewayTradeNo:   "cs_old",
		Status:           common.TopUpStatusPending,
		CheckoutRevision: 1,
	}).Error)
	oldSessionID := "cs_old"
	require.NoError(t, DB.Create(&StripeCheckoutRevision{
		OrderType:         StripeCheckoutOrderTopUp,
		TradeNo:           "t-activate",
		Revision:          1,
		UserId:            9,
		RequestId:         "req-original",
		SelectionDigest:   "sha256:original",
		State:             StripeCheckoutRevisionStateActive,
		DiscountSource:    "recall",
		ProviderSessionId: &oldSessionID,
	}).Error)

	candidate, replay, err := PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
		OrderType:        StripeCheckoutOrderTopUp,
		TradeNo:          "t-activate",
		UserID:           9,
		ExpectedRevision: 1,
		RequestID:        "req-activate",
		SelectionDigest:  "sha256:manual",
		DiscountSource:   "manual",
	})
	require.NoError(t, err)
	require.False(t, replay)

	newSessionID := "cs_new"
	candidate, err = RecordStripeCheckoutCandidate(StripeCheckoutRevisionCandidate{
		RevisionID:         candidate.Id,
		ProviderSessionID:  &newSessionID,
		ProviderSessionURL: "https://checkout.example/new",
		SummaryPayload:     `{"total":900}`,
	})
	require.NoError(t, err)
	require.Equal(t, newSessionID, *candidate.ProviderSessionId)

	active, err := ActivateStripeCheckoutRevision(StripeCheckoutRevisionActivation{
		RevisionID:           candidate.Id,
		ExpectedRevision:     1,
		OldProviderSessionID: "cs_old",
	})
	require.NoError(t, err)
	require.Equal(t, StripeCheckoutRevisionStateActive, active.State)

	var stored TopUp
	require.NoError(t, DB.Where("trade_no = ?", "t-activate").First(&stored).Error)
	require.Equal(t, "cs_new", stored.GatewayTradeNo)
	require.Equal(t, int64(2), stored.CheckoutRevision)

	var original StripeCheckoutRevision
	require.NoError(t, DB.Where("order_type = ? AND trade_no = ? AND revision = ?", StripeCheckoutOrderTopUp, "t-activate", 1).First(&original).Error)
	require.Equal(t, StripeCheckoutRevisionStateSuperseded, original.State)

	_, err = ActivateStripeCheckoutRevision(StripeCheckoutRevisionActivation{
		RevisionID:           candidate.Id,
		ExpectedRevision:     1,
		OldProviderSessionID: "cs_old",
	})
	require.ErrorIs(t, err, ErrStripeCheckoutRevisionConflict)
}

func TestStripeCheckoutRevisionSupportsSubscriptionPointerCAS(t *testing.T) {
	setupStripeCheckoutRevisionTestDB(t)
	require.NoError(t, DB.Create(&SubscriptionOrder{
		UserId:            10,
		TradeNo:           "sub-activate",
		ProviderSessionId: "cs_sub_old",
		Status:            common.TopUpStatusPending,
		CheckoutRevision:  1,
	}).Error)

	candidate, _, err := PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
		OrderType:        StripeCheckoutOrderSubscription,
		TradeNo:          "sub-activate",
		UserID:           10,
		ExpectedRevision: 1,
		RequestID:        "req-sub",
		SelectionDigest:  "sha256:sub-manual",
	})
	require.NoError(t, err)
	newSessionID := "cs_sub_new"
	_, err = RecordStripeCheckoutCandidate(StripeCheckoutRevisionCandidate{
		RevisionID:         candidate.Id,
		ProviderSessionID:  &newSessionID,
		ProviderSessionURL: "https://checkout.example/sub-new",
	})
	require.NoError(t, err)
	_, err = ActivateStripeCheckoutRevision(StripeCheckoutRevisionActivation{
		RevisionID:           candidate.Id,
		ExpectedRevision:     1,
		OldProviderSessionID: "cs_sub_old",
	})
	require.NoError(t, err)

	var stored SubscriptionOrder
	require.NoError(t, DB.Where("trade_no = ?", "sub-activate").First(&stored).Error)
	require.Equal(t, "cs_sub_new", stored.ProviderSessionId)
	require.Equal(t, "https://checkout.example/sub-new", stored.ProviderSessionURL)
	require.Equal(t, int64(2), stored.CheckoutRevision)
}

func TestStripeCheckoutRevisionNullableProviderSessionIDsAndAbandonment(t *testing.T) {
	setupStripeCheckoutRevisionTestDB(t)
	for _, fixture := range []struct {
		userID  int
		tradeNo string
		reqID   string
	}{
		{userID: 11, tradeNo: "t-null-1", reqID: "req-null-1"},
		{userID: 12, tradeNo: "t-null-2", reqID: "req-null-2"},
	} {
		require.NoError(t, DB.Create(&TopUp{
			UserId:           fixture.userID,
			TradeNo:          fixture.tradeNo,
			GatewayTradeNo:   "cs_" + fixture.tradeNo,
			Status:           common.TopUpStatusPending,
			CheckoutRevision: 1,
		}).Error)
		prepared, replay, err := PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
			OrderType:        StripeCheckoutOrderTopUp,
			TradeNo:          fixture.tradeNo,
			UserID:           fixture.userID,
			ExpectedRevision: 1,
			RequestID:        fixture.reqID,
			SelectionDigest:  "sha256:none",
		})
		require.NoError(t, err)
		require.False(t, replay)
		require.Nil(t, prepared.ProviderSessionId)
		if fixture.userID == 11 {
			require.NoError(t, AbandonStripeCheckoutRevision(prepared.Id))
			stored, err := GetStripeCheckoutRevisionByRequestID(StripeCheckoutOrderTopUp, fixture.tradeNo, fixture.reqID)
			require.NoError(t, err)
			require.Equal(t, StripeCheckoutRevisionStateAbandoned, stored.State)
			require.ErrorIs(t, AbandonStripeCheckoutRevision(prepared.Id), ErrStripeCheckoutRevisionConflict)
		}
	}
}

func TestStripeCheckoutRevisionSkipsAbandonedRevision(t *testing.T) {
	setupStripeCheckoutRevisionTestDB(t)
	require.NoError(t, DB.Create(&TopUp{
		UserId:           13,
		TradeNo:          "t-abandoned-gap",
		GatewayTradeNo:   "cs_abandoned_old",
		Status:           common.TopUpStatusPending,
		CheckoutRevision: 1,
	}).Error)

	abandoned, replay, err := PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
		OrderType:        StripeCheckoutOrderTopUp,
		TradeNo:          "t-abandoned-gap",
		UserID:           13,
		ExpectedRevision: 1,
		RequestID:        "req-abandoned",
		SelectionDigest:  "sha256:abandoned",
	})
	require.NoError(t, err)
	require.False(t, replay)
	require.Equal(t, int64(2), abandoned.Revision)
	require.NoError(t, AbandonStripeCheckoutRevision(abandoned.Id))

	next, replay, err := PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
		OrderType:        StripeCheckoutOrderTopUp,
		TradeNo:          "t-abandoned-gap",
		UserID:           13,
		ExpectedRevision: 1,
		RequestID:        "req-after-abandoned",
		SelectionDigest:  "sha256:replacement",
	})
	require.NoError(t, err)
	require.False(t, replay)
	require.Equal(t, int64(3), next.Revision)

	immutable, err := GetStripeCheckoutRevisionByRequestID(StripeCheckoutOrderTopUp, "t-abandoned-gap", "req-abandoned")
	require.NoError(t, err)
	require.Equal(t, abandoned.Id, immutable.Id)
	require.Equal(t, int64(2), immutable.Revision)
	require.Equal(t, "sha256:abandoned", immutable.SelectionDigest)
	require.Equal(t, StripeCheckoutRevisionStateAbandoned, immutable.State)
}

func setupStripeCheckoutRevisionTestDB(t *testing.T) {
	t.Helper()
	setupTopUpLifecycleTestDB(t, 1)
	require.NoError(t, DB.AutoMigrate(&SubscriptionOrder{}, &StripeCheckoutRevision{}))
}
