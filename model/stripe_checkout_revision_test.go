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

func TestStripeCheckoutRevisionCandidateAttachmentExactReplaySucceeds(t *testing.T) {
	setupStripeCheckoutRevisionTestDB(t)
	require.NoError(t, DB.Create(&TopUp{
		UserId:           14,
		TradeNo:          "t-candidate-replay",
		GatewayTradeNo:   "cs_candidate_old",
		Status:           common.TopUpStatusPending,
		CheckoutRevision: 1,
	}).Error)
	prepared, _, err := PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
		OrderType:        StripeCheckoutOrderTopUp,
		TradeNo:          "t-candidate-replay",
		UserID:           14,
		ExpectedRevision: 1,
		RequestID:        "req-candidate-replay",
		SelectionDigest:  "sha256:candidate-replay",
	})
	require.NoError(t, err)

	sessionID := "cs_candidate_replay"
	input := StripeCheckoutRevisionCandidate{
		RevisionID:         prepared.Id,
		ProviderSessionID:  &sessionID,
		ProviderSessionURL: "https://checkout.example/candidate-replay",
		SummaryPayload:     `{"total":700}`,
	}
	first, err := RecordStripeCheckoutCandidate(input)
	require.NoError(t, err)
	second, err := RecordStripeCheckoutCandidate(input)
	require.NoError(t, err)
	require.Equal(t, first.Id, second.Id)
	require.Equal(t, "cs_candidate_replay", *second.ProviderSessionId)
	require.Equal(t, "https://checkout.example/candidate-replay", second.ProviderSessionURL)
	require.Equal(t, `{"total":700}`, second.SummaryPayload)
}

func TestStripeCheckoutRevisionCandidateAttachmentRejectsDifferentSession(t *testing.T) {
	setupStripeCheckoutRevisionTestDB(t)
	require.NoError(t, DB.Create(&TopUp{
		UserId:           15,
		TradeNo:          "t-candidate-conflict",
		GatewayTradeNo:   "cs_candidate_conflict_old",
		Status:           common.TopUpStatusPending,
		CheckoutRevision: 1,
	}).Error)
	prepared, _, err := PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
		OrderType:        StripeCheckoutOrderTopUp,
		TradeNo:          "t-candidate-conflict",
		UserID:           15,
		ExpectedRevision: 1,
		RequestID:        "req-candidate-conflict",
		SelectionDigest:  "sha256:candidate-conflict",
	})
	require.NoError(t, err)

	firstSessionID := "cs_candidate_first"
	_, err = RecordStripeCheckoutCandidate(StripeCheckoutRevisionCandidate{
		RevisionID:         prepared.Id,
		ProviderSessionID:  &firstSessionID,
		ProviderSessionURL: "https://checkout.example/candidate-first",
		SummaryPayload:     `{"total":800}`,
	})
	require.NoError(t, err)

	secondSessionID := "cs_candidate_second"
	_, err = RecordStripeCheckoutCandidate(StripeCheckoutRevisionCandidate{
		RevisionID:         prepared.Id,
		ProviderSessionID:  &secondSessionID,
		ProviderSessionURL: "https://checkout.example/candidate-second",
		SummaryPayload:     `{"total":600}`,
	})
	require.ErrorIs(t, err, ErrStripeCheckoutRevisionConflict)

	stored, err := GetStripeCheckoutRevisionByRequestID(StripeCheckoutOrderTopUp, "t-candidate-conflict", "req-candidate-conflict")
	require.NoError(t, err)
	require.Equal(t, "cs_candidate_first", *stored.ProviderSessionId)
	require.Equal(t, "https://checkout.example/candidate-first", stored.ProviderSessionURL)
	require.Equal(t, `{"total":800}`, stored.SummaryPayload)
}

func TestStripeCheckoutRevisionCandidateAttachmentRejectsSameSessionWithDifferentMetadata(t *testing.T) {
	setupStripeCheckoutRevisionTestDB(t)
	require.NoError(t, DB.Create(&TopUp{
		UserId:           18,
		TradeNo:          "t-candidate-metadata-conflict",
		GatewayTradeNo:   "cs_candidate_metadata_old",
		Status:           common.TopUpStatusPending,
		CheckoutRevision: 1,
	}).Error)
	prepared, _, err := PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
		OrderType:        StripeCheckoutOrderTopUp,
		TradeNo:          "t-candidate-metadata-conflict",
		UserID:           18,
		ExpectedRevision: 1,
		RequestID:        "req-candidate-metadata-conflict",
		SelectionDigest:  "sha256:candidate-metadata-conflict",
	})
	require.NoError(t, err)

	sessionID := "cs_candidate_metadata"
	_, err = RecordStripeCheckoutCandidate(StripeCheckoutRevisionCandidate{
		RevisionID:         prepared.Id,
		ProviderSessionID:  &sessionID,
		ProviderSessionURL: "https://checkout.example/candidate-metadata",
		SummaryPayload:     `{"total":500}`,
	})
	require.NoError(t, err)

	for _, fixture := range []struct {
		name           string
		url            string
		summaryPayload string
	}{
		{name: "url", url: "https://checkout.example/different", summaryPayload: `{"total":500}`},
		{name: "summary", url: "https://checkout.example/candidate-metadata", summaryPayload: `{"total":400}`},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			_, err := RecordStripeCheckoutCandidate(StripeCheckoutRevisionCandidate{
				RevisionID:         prepared.Id,
				ProviderSessionID:  &sessionID,
				ProviderSessionURL: fixture.url,
				SummaryPayload:     fixture.summaryPayload,
			})
			require.ErrorIs(t, err, ErrStripeCheckoutRevisionConflict)
		})
	}

	stored, err := GetStripeCheckoutRevisionByRequestID(StripeCheckoutOrderTopUp, "t-candidate-metadata-conflict", "req-candidate-metadata-conflict")
	require.NoError(t, err)
	require.Equal(t, "cs_candidate_metadata", *stored.ProviderSessionId)
	require.Equal(t, "https://checkout.example/candidate-metadata", stored.ProviderSessionURL)
	require.Equal(t, `{"total":500}`, stored.SummaryPayload)
}

func TestStripeCheckoutRevisionActivationRollsBackWithoutPriorActiveHistory(t *testing.T) {
	setupStripeCheckoutRevisionTestDB(t)
	require.NoError(t, DB.Create(&TopUp{
		UserId:           16,
		TradeNo:          "t-missing-active-history",
		GatewayTradeNo:   "cs_missing_history_old",
		Status:           common.TopUpStatusPending,
		CheckoutRevision: 1,
	}).Error)
	prepared, _, err := PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
		OrderType:        StripeCheckoutOrderTopUp,
		TradeNo:          "t-missing-active-history",
		UserID:           16,
		ExpectedRevision: 1,
		RequestID:        "req-missing-active-history",
		SelectionDigest:  "sha256:missing-active-history",
	})
	require.NoError(t, err)
	newSessionID := "cs_missing_history_new"
	_, err = RecordStripeCheckoutCandidate(StripeCheckoutRevisionCandidate{
		RevisionID:        prepared.Id,
		ProviderSessionID: &newSessionID,
	})
	require.NoError(t, err)

	_, err = ActivateStripeCheckoutRevision(StripeCheckoutRevisionActivation{
		RevisionID:           prepared.Id,
		ExpectedRevision:     1,
		OldProviderSessionID: "cs_missing_history_old",
	})
	require.ErrorIs(t, err, ErrStripeCheckoutRevisionConflict)

	var order TopUp
	require.NoError(t, DB.Where("trade_no = ?", "t-missing-active-history").First(&order).Error)
	require.Equal(t, "cs_missing_history_old", order.GatewayTradeNo)
	require.Equal(t, int64(1), order.CheckoutRevision)

	stored, err := GetStripeCheckoutRevisionByRequestID(StripeCheckoutOrderTopUp, "t-missing-active-history", "req-missing-active-history")
	require.NoError(t, err)
	require.Equal(t, StripeCheckoutRevisionStatePreparing, stored.State)
	require.Equal(t, "cs_missing_history_new", *stored.ProviderSessionId)
}

func TestStripeCheckoutRevisionActivationRollsBackWhenPriorActiveSessionMismatchesOrder(t *testing.T) {
	for _, fixture := range []struct {
		name      string
		orderType string
		tradeNo   string
		userID    int
	}{
		{name: "topup", orderType: StripeCheckoutOrderTopUp, tradeNo: "t-active-session-mismatch", userID: 19},
		{name: "subscription", orderType: StripeCheckoutOrderSubscription, tradeNo: "sub-active-session-mismatch", userID: 20},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			setupStripeCheckoutRevisionTestDB(t)
			const orderSessionID = "cs_actual"
			if fixture.orderType == StripeCheckoutOrderTopUp {
				require.NoError(t, DB.Create(&TopUp{
					UserId:           fixture.userID,
					TradeNo:          fixture.tradeNo,
					GatewayTradeNo:   orderSessionID,
					Status:           common.TopUpStatusPending,
					CheckoutRevision: 1,
				}).Error)
			} else {
				require.NoError(t, DB.Create(&SubscriptionOrder{
					UserId:             fixture.userID,
					TradeNo:            fixture.tradeNo,
					ProviderSessionId:  orderSessionID,
					ProviderSessionURL: "https://checkout.example/actual",
					Status:             common.TopUpStatusPending,
					CheckoutRevision:   1,
				}).Error)
			}

			ledgerSessionID := "cs_other"
			priorActive := StripeCheckoutRevision{
				OrderType:         fixture.orderType,
				TradeNo:           fixture.tradeNo,
				Revision:          1,
				UserId:            fixture.userID,
				RequestId:         "req-prior-" + fixture.name,
				SelectionDigest:   "sha256:prior-" + fixture.name,
				State:             StripeCheckoutRevisionStateActive,
				DiscountSource:    "none",
				ProviderSessionId: &ledgerSessionID,
			}
			require.NoError(t, DB.Create(&priorActive).Error)

			prepared, _, err := PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
				OrderType:        fixture.orderType,
				TradeNo:          fixture.tradeNo,
				UserID:           fixture.userID,
				ExpectedRevision: 1,
				RequestID:        "req-candidate-" + fixture.name,
				SelectionDigest:  "sha256:candidate-" + fixture.name,
			})
			require.NoError(t, err)
			candidateSessionID := "cs_candidate_" + fixture.name
			_, err = RecordStripeCheckoutCandidate(StripeCheckoutRevisionCandidate{
				RevisionID:         prepared.Id,
				ProviderSessionID:  &candidateSessionID,
				ProviderSessionURL: "https://checkout.example/candidate-" + fixture.name,
			})
			require.NoError(t, err)

			_, err = ActivateStripeCheckoutRevision(StripeCheckoutRevisionActivation{
				RevisionID:           prepared.Id,
				ExpectedRevision:     1,
				OldProviderSessionID: orderSessionID,
			})
			require.ErrorIs(t, err, ErrStripeCheckoutRevisionConflict)

			if fixture.orderType == StripeCheckoutOrderTopUp {
				var order TopUp
				require.NoError(t, DB.Where("trade_no = ?", fixture.tradeNo).First(&order).Error)
				require.Equal(t, orderSessionID, order.GatewayTradeNo)
				require.Equal(t, int64(1), order.CheckoutRevision)
			} else {
				var order SubscriptionOrder
				require.NoError(t, DB.Where("trade_no = ?", fixture.tradeNo).First(&order).Error)
				require.Equal(t, orderSessionID, order.ProviderSessionId)
				require.Equal(t, "https://checkout.example/actual", order.ProviderSessionURL)
				require.Equal(t, int64(1), order.CheckoutRevision)
			}

			var storedPrior StripeCheckoutRevision
			require.NoError(t, DB.First(&storedPrior, priorActive.Id).Error)
			require.Equal(t, StripeCheckoutRevisionStateActive, storedPrior.State)
			require.Equal(t, ledgerSessionID, *storedPrior.ProviderSessionId)

			storedCandidate, err := GetStripeCheckoutRevisionByRequestID(fixture.orderType, fixture.tradeNo, "req-candidate-"+fixture.name)
			require.NoError(t, err)
			require.Equal(t, StripeCheckoutRevisionStatePreparing, storedCandidate.State)
			require.Equal(t, candidateSessionID, *storedCandidate.ProviderSessionId)
		})
	}
}

func TestStripeCheckoutRevisionActivationAllowsLegacyRevisionZeroWithoutPriorHistory(t *testing.T) {
	setupStripeCheckoutRevisionTestDB(t)
	require.NoError(t, DB.Create(&TopUp{
		UserId:           17,
		TradeNo:          "t-legacy-no-history",
		GatewayTradeNo:   "cs_legacy_old",
		Status:           common.TopUpStatusPending,
		CheckoutRevision: 0,
	}).Error)
	prepared, _, err := PrepareStripeCheckoutRevision(StripeCheckoutRevisionPrepare{
		OrderType:        StripeCheckoutOrderTopUp,
		TradeNo:          "t-legacy-no-history",
		UserID:           17,
		ExpectedRevision: 0,
		RequestID:        "req-legacy-no-history",
		SelectionDigest:  "sha256:legacy-no-history",
	})
	require.NoError(t, err)
	newSessionID := "cs_legacy_new"
	_, err = RecordStripeCheckoutCandidate(StripeCheckoutRevisionCandidate{
		RevisionID:        prepared.Id,
		ProviderSessionID: &newSessionID,
	})
	require.NoError(t, err)

	_, err = ActivateStripeCheckoutRevision(StripeCheckoutRevisionActivation{
		RevisionID:           prepared.Id,
		ExpectedRevision:     0,
		OldProviderSessionID: "cs_legacy_old",
	})
	require.NoError(t, err)

	var order TopUp
	require.NoError(t, DB.Where("trade_no = ?", "t-legacy-no-history").First(&order).Error)
	require.Equal(t, "cs_legacy_new", order.GatewayTradeNo)
	require.Equal(t, int64(1), order.CheckoutRevision)
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
	oldSessionID := "cs_sub_old"
	require.NoError(t, DB.Create(&StripeCheckoutRevision{
		OrderType:         StripeCheckoutOrderSubscription,
		TradeNo:           "sub-activate",
		Revision:          1,
		UserId:            10,
		RequestId:         "req-sub-original",
		SelectionDigest:   "sha256:sub-original",
		State:             StripeCheckoutRevisionStateActive,
		DiscountSource:    "none",
		ProviderSessionId: &oldSessionID,
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
