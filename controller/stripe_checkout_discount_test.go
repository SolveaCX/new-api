package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v86"
	"gorm.io/gorm"
)

func TestUpdateStripeCheckoutDiscountApplyTransitionsCandidateInOrder(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountInvitation)
	events := make([]string, 0, 4)
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		ResolvePromotion: func(_ context.Context, query service.StripeCheckoutPromotionQuery) (service.StripeCheckoutResolvedPromotion, error) {
			require.Equal(t, "SAVE20", query.Code)
			return service.StripeCheckoutResolvedPromotion{PromotionCodeID: "promo_manual", CouponID: "coupon_manual", MaskedCode: "SAVE20"}, nil
		},
		CreateCandidate: func(_ context.Context, purchase stripeCheckoutPurchase, revision int64, selection service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			events = append(events, fmt.Sprintf("create:%d", revision))
			require.Equal(t, service.StripeCheckoutDiscountManual, selection.Source)
			require.Equal(t, service.StripeCheckoutDiscountInvitation, selection.ReplacedSource)
			return &stripeCheckoutSessionSnapshot{ID: "cs_new", ClientSecret: "cs_new_secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		GetSession: func(_ context.Context, purchaseKind service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			events = append(events, "get:"+sessionID)
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			events = append(events, "expire:"+sessionID)
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
	})
	t.Cleanup(restore)

	recorder := fixture.post(t, stripeCheckoutDiscountRequest{
		CheckoutContext:  fixture.context,
		ExpectedRevision: 1,
		RequestID:        "  req-apply-2  ",
		Action:           "apply",
		PromotionCode:    "  SAVE20  ",
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	var envelope struct {
		Success bool                           `json:"success"`
		Data    StripeCheckoutRevisionResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)
	require.Equal(t, "cs_new_secret", envelope.Data.ClientSecret)
	require.EqualValues(t, 2, envelope.Data.CheckoutRevision)
	require.Equal(t, service.StripeCheckoutDiscountManual, envelope.Data.DiscountState.Source)
	require.Equal(t, "SAVE20", envelope.Data.DiscountState.PromotionCodeMasked)
	require.Equal(t, []string{"create:2", "get:cs_old", "expire:cs_old"}, events)
	require.NotContains(t, recorder.Body.String(), "  SAVE20  ")

	var order model.TopUp
	require.NoError(t, model.DB.Where("trade_no = ?", fixture.tradeNo).First(&order).Error)
	require.EqualValues(t, 2, order.CheckoutRevision)
	require.Equal(t, "cs_new", order.GatewayTradeNo)
	active, err := model.GetActiveStripeCheckoutRevision(model.StripeCheckoutOrderTopUp, fixture.tradeNo)
	require.NoError(t, err)
	require.EqualValues(t, 2, active.Revision)
	require.Equal(t, "req-apply-2", active.RequestId)
}

func TestUpdateStripeCheckoutDiscountRejectsInvalidApplyWithoutTouchingOldSession(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	touched := false
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		ResolvePromotion: func(context.Context, service.StripeCheckoutPromotionQuery) (service.StripeCheckoutResolvedPromotion, error) {
			return service.StripeCheckoutResolvedPromotion{}, service.ErrStripePromotionUnavailable
		},
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			touched = true
			return nil, errors.New("must not create")
		},
		GetSession: func(context.Context, service.StripeCheckoutPurchaseKind, string) (*stripeCheckoutSessionSnapshot, error) {
			touched = true
			return nil, errors.New("must not get")
		},
		ExpireSession: func(context.Context, service.StripeCheckoutPurchaseKind, string) (*stripeCheckoutSessionSnapshot, error) {
			touched = true
			return nil, errors.New("must not expire")
		},
	})
	t.Cleanup(restore)

	recorder := fixture.post(t, stripeCheckoutDiscountRequest{
		CheckoutContext:  fixture.context,
		ExpectedRevision: 1,
		RequestID:        "req-invalid",
		Action:           "apply",
		PromotionCode:    "NOPE",
	})

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.False(t, touched)
	require.Equal(t, "promotion_code_invalid", stripeCheckoutErrorCode(t, recorder))
	active, err := model.GetActiveStripeCheckoutRevision(model.StripeCheckoutOrderTopUp, fixture.tradeNo)
	require.NoError(t, err)
	require.EqualValues(t, 1, active.Revision)
}

func TestUpdateStripeCheckoutDiscountRestoreUsesCanonicalRevisionOne(t *testing.T) {
	for _, source := range []service.StripeCheckoutDiscountSource{
		service.StripeCheckoutDiscountNone,
		service.StripeCheckoutDiscountInvitation,
		service.StripeCheckoutDiscountRecall,
	} {
		t.Run(string(source), func(t *testing.T) {
			fixture := newStripeCheckoutDiscountFixture(t, source)
			var got service.StripeCheckoutDiscountSelection
			restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
				CreateCandidate: func(_ context.Context, _ stripeCheckoutPurchase, _ int64, selection service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
					got = selection
					return &stripeCheckoutSessionSnapshot{ID: "cs_restore", ClientSecret: "cs_restore_secret", Status: "open", PaymentStatus: "unpaid"}, nil
				},
				GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
					return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "open", PaymentStatus: "unpaid"}, nil
				},
				ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
					return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
				},
			})
			t.Cleanup(restore)

			recorder := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-restore", Action: "restore"})
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, source, got.Source)
			switch source {
			case service.StripeCheckoutDiscountInvitation:
				require.Equal(t, "coupon_original", got.CouponID)
				require.Empty(t, got.PromotionCodeID)
			case service.StripeCheckoutDiscountRecall:
				require.Equal(t, "promo_original", got.PromotionCodeID)
				require.Empty(t, got.CouponID)
			default:
				require.Empty(t, got.CouponID)
				require.Empty(t, got.PromotionCodeID)
			}
		})
	}
}

func TestUpdateStripeCheckoutDiscountValidatesFeatureRequestAndContext(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)

	setting.StripePromotionCodeEnabled = false
	recorder := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-disabled", Action: "restore"})
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Equal(t, "stripe_promotion_disabled", stripeCheckoutErrorCode(t, recorder))

	setting.StripePromotionCodeEnabled = true
	tests := []struct {
		name       string
		context    string
		userID     int
		expected   int64
		requestID  string
		wantStatus int
		wantCode   string
	}{
		{name: "tampered", context: fixture.context + "x", userID: fixture.userID, expected: 1, requestID: "req-tampered", wantStatus: http.StatusBadRequest, wantCode: "checkout_context_invalid"},
		{name: "wrong user", context: fixture.context, userID: fixture.userID + 1, expected: 1, requestID: "req-wrong-user", wantStatus: http.StatusBadRequest, wantCode: "checkout_context_invalid"},
		{name: "revision mismatch with context", context: fixture.context, userID: fixture.userID, expected: 2, requestID: "req-context-revision", wantStatus: http.StatusConflict, wantCode: "checkout_revision_conflict"},
		{name: "request id too long", context: fixture.context, userID: fixture.userID, expected: 1, requestID: string(bytes.Repeat([]byte("x"), 65)), wantStatus: http.StatusBadRequest, wantCode: "checkout_request_invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := fixture.postAs(t, test.userID, stripeCheckoutDiscountRequest{CheckoutContext: test.context, ExpectedRevision: test.expected, RequestID: test.requestID, Action: "restore"})
			require.Equal(t, test.wantStatus, recorder.Code)
			require.Equal(t, test.wantCode, stripeCheckoutErrorCode(t, recorder))
		})
	}

	expired, err := service.SignStripeCheckoutContext(service.StripeCheckoutContextClaims{UserID: fixture.userID, PurchaseKind: service.StripeCheckoutPurchaseTopUp, TradeNo: fixture.tradeNo, Revision: 1, ExpiresAt: time.Now().Add(-time.Minute).Unix()})
	require.NoError(t, err)
	recorder = fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: expired, ExpectedRevision: 1, RequestID: "req-expired", Action: "restore"})
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, "checkout_context_expired", stripeCheckoutErrorCode(t, recorder))
}

func TestUpdateStripeCheckoutDiscountExactReplayReturnsPersistedCandidateWithoutSecondCreate(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	createCalls := 0
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			createCalls++
			return &stripeCheckoutSessionSnapshot{ID: "cs_replay", ClientSecret: "cs_replay_secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			secret := ""
			if sessionID == "cs_replay" {
				secret = "cs_replay_secret"
			}
			return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: secret, Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
	})
	t.Cleanup(restore)
	request := stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-replay", Action: "restore"}

	first := fixture.post(t, request)
	require.Equal(t, http.StatusOK, first.Code)
	second := fixture.post(t, request)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	require.Equal(t, 1, createCalls)
	var envelope struct {
		Data StripeCheckoutRevisionResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(second.Body.Bytes(), &envelope))
	require.EqualValues(t, 2, envelope.Data.CheckoutRevision)
	require.Equal(t, "cs_replay_secret", envelope.Data.ClientSecret)
}

func TestUpdateStripeCheckoutDiscountNewRequestAtStaleRevisionReturnsLatest(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	createCalls := 0
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			createCalls++
			return &stripeCheckoutSessionSnapshot{ID: "cs_winner", ClientSecret: "cs_winner_secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			secret := ""
			if sessionID == "cs_winner" {
				secret = "cs_winner_secret"
			}
			return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: secret, Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
	})
	t.Cleanup(restore)
	require.Equal(t, http.StatusOK, fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-winner", Action: "restore"}).Code)

	loser := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-loser", Action: "restore"})
	require.Equal(t, http.StatusConflict, loser.Code)
	require.Equal(t, "checkout_revision_conflict", stripeCheckoutErrorCode(t, loser))
	var envelope struct {
		Data StripeCheckoutRevisionResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(loser.Body.Bytes(), &envelope))
	require.EqualValues(t, 2, envelope.Data.CheckoutRevision)
	require.Equal(t, "cs_winner_secret", envelope.Data.ClientSecret)
	require.Equal(t, 1, createCalls)
}

func TestUpdateStripeCheckoutDiscountPaymentWinsExpirationRace(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	expired := make([]string, 0, 2)
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: "cs_payment_race", ClientSecret: "secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			expired = append(expired, sessionID)
			if sessionID == "cs_old" {
				return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "complete", PaymentStatus: "paid"}, nil
			}
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
	})
	t.Cleanup(restore)

	recorder := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-payment-race", Action: "restore"})
	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Equal(t, "checkout_already_completed", stripeCheckoutErrorCode(t, recorder))
	require.Equal(t, []string{"cs_old", "cs_payment_race"}, expired)
	revision, err := model.GetStripeCheckoutRevisionByRequestID(model.StripeCheckoutOrderTopUp, fixture.tradeNo, "req-payment-race")
	require.NoError(t, err)
	require.Equal(t, model.StripeCheckoutRevisionStateAbandoned, revision.State)
}

func TestUpdateStripeCheckoutDiscountActivationCASLossExpiresAndAbandonsCandidate(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	expired := make([]string, 0, 2)
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: "cs_cas_loser", ClientSecret: "secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			expired = append(expired, sessionID)
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
		ActivateRevision: func(model.StripeCheckoutRevisionActivation) (*model.StripeCheckoutRevision, error) {
			return nil, model.ErrStripeCheckoutRevisionConflict
		},
	})
	t.Cleanup(restore)

	recorder := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-cas-loss", Action: "restore"})
	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Equal(t, "checkout_revision_conflict", stripeCheckoutErrorCode(t, recorder))
	require.Equal(t, []string{"cs_old", "cs_cas_loser"}, expired)
	revision, err := model.GetStripeCheckoutRevisionByRequestID(model.StripeCheckoutOrderTopUp, fixture.tradeNo, "req-cas-loss")
	require.NoError(t, err)
	require.Equal(t, model.StripeCheckoutRevisionStateAbandoned, revision.State)
}

func TestUpdateStripeCheckoutDiscountTransientActivationRetryPromotesRecordedCandidate(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	createCalls := 0
	activateCalls := 0
	oldExpired := false
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			createCalls++
			return &stripeCheckoutSessionSnapshot{ID: "cs_transient", ClientSecret: "cs_transient_secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			if sessionID == "cs_old" && oldExpired {
				return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
			}
			secret := ""
			if sessionID == "cs_transient" {
				secret = "cs_transient_secret"
			}
			return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: secret, Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			if sessionID == "cs_old" {
				oldExpired = true
			}
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
		ActivateRevision: func(input model.StripeCheckoutRevisionActivation) (*model.StripeCheckoutRevision, error) {
			activateCalls++
			if activateCalls == 1 {
				return nil, errors.New("transient database failure")
			}
			return model.ActivateStripeCheckoutRevision(input)
		},
	})
	t.Cleanup(restore)
	request := stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-transient", Action: "restore"}

	first := fixture.post(t, request)
	require.Equal(t, http.StatusInternalServerError, first.Code)
	require.Equal(t, "checkout_replacement_failed", stripeCheckoutErrorCode(t, first))
	preparing, err := model.GetStripeCheckoutRevisionByRequestID(model.StripeCheckoutOrderTopUp, fixture.tradeNo, "req-transient")
	require.NoError(t, err)
	require.Equal(t, model.StripeCheckoutRevisionStatePreparing, preparing.State)

	second := fixture.post(t, request)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	require.Equal(t, 1, createCalls)
	require.Equal(t, 2, activateCalls)
	active, err := model.GetActiveStripeCheckoutRevision(model.StripeCheckoutOrderTopUp, fixture.tradeNo)
	require.NoError(t, err)
	require.EqualValues(t, 2, active.Revision)
}

func TestUpdateStripeCheckoutDiscountRejectsRevisionGapBeforeCreatingCandidate(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	createCalls := 0
	abandonedSession := "cs_abandoned"
	require.NoError(t, model.DB.Create(&model.StripeCheckoutRevision{
		OrderType: model.StripeCheckoutOrderTopUp, TradeNo: fixture.tradeNo, Revision: 2, UserId: fixture.userID,
		RequestId: "req-abandoned-gap", SelectionDigest: "gap", State: model.StripeCheckoutRevisionStateAbandoned,
		DiscountSource: string(service.StripeCheckoutDiscountNone), ProviderSessionId: &abandonedSession,
	}).Error)
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			createCalls++
			return nil, errors.New("must not create")
		},
	})
	t.Cleanup(restore)

	recorder := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-after-gap", Action: "restore"})
	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Equal(t, "checkout_revision_conflict", stripeCheckoutErrorCode(t, recorder))
	require.Zero(t, createCalls)
}

func TestCreateInitialStripeCheckoutRevisionActivatesTopUpAndOneTime(t *testing.T) {
	for _, kind := range []service.StripeCheckoutPurchaseKind{
		service.StripeCheckoutPurchaseTopUp,
		service.StripeCheckoutPurchaseOneTimeSubscription,
	} {
		t.Run(string(kind), func(t *testing.T) {
			fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
			require.NoError(t, model.DB.Where("order_type = ? AND trade_no = ?", model.StripeCheckoutOrderTopUp, fixture.tradeNo).Delete(&model.StripeCheckoutRevision{}).Error)
			purchase := stripeCheckoutPurchase{Kind: kind, TradeNo: fixture.tradeNo, UserID: fixture.userID, Currency: "USD", SubtotalMinor: 2000}
			if kind == service.StripeCheckoutPurchaseTopUp {
				purchase.OrderType = model.StripeCheckoutOrderTopUp
				require.NoError(t, model.DB.Model(&model.TopUp{}).Where("trade_no = ?", fixture.tradeNo).Updates(map[string]any{"checkout_revision": 0, "gateway_trade_no": ""}).Error)
			} else {
				purchase.OrderType = model.StripeCheckoutOrderSubscription
				purchase.TradeNo = "trade-one-time-initial"
				require.NoError(t, model.DB.Create(&model.SubscriptionOrder{
					UserId: fixture.userID, TradeNo: purchase.TradeNo, Status: common.TopUpStatusPending,
					PaymentProvider: model.PaymentProviderStripe, PaymentMethod: model.PaymentMethodStripe,
					PaymentCurrency: "USD", PaymentAmountMinor: 2000,
				}).Error)
			}
			created, active, err := createInitialStripeCheckoutRevision(context.Background(), purchase, service.StripeCheckoutDiscountSelection{Source: service.StripeCheckoutDiscountNone}, nil, func(revision int64) (*stripeCheckoutSessionSnapshot, error) {
				require.EqualValues(t, 1, revision)
				prepared, lookupErr := model.GetStripeCheckoutRevision(purchase.OrderType, purchase.TradeNo, 1)
				require.NoError(t, lookupErr)
				require.Equal(t, model.StripeCheckoutRevisionStatePreparing, prepared.State)
				return &stripeCheckoutSessionSnapshot{ID: "cs_initial_" + string(kind), ClientSecret: "secret", Status: "open", PaymentStatus: "unpaid"}, nil
			})
			require.NoError(t, err)
			require.Equal(t, created.ID, stripeCheckoutRevisionSessionID(active))
			require.EqualValues(t, 1, active.Revision)
			require.Equal(t, model.StripeCheckoutRevisionStateActive, active.State)
		})
	}
}

func TestStripeRecurringInitialRevisionPersistsResolvedInvitationCoupon(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	tradeNo := "trade-recurring-initial"
	require.NoError(t, model.DB.Create(&model.SubscriptionOrder{
		UserId: fixture.userID, TradeNo: tradeNo, PlanId: 77, Status: common.TopUpStatusPending,
		PaymentProvider: model.PaymentProviderStripe, PaymentMethod: model.PaymentMethodStripe,
		PaymentCurrency: "USD", PaymentAmountMinor: 1500, DiscountKind: service.SubscriptionDiscountKindInvitation,
		SubscriptionDiscountAmountMinor: 500,
	}).Error)
	originalSecret := setting.StripeApiSecret
	originalPublishable := setting.StripePublishableKey
	setting.StripeApiSecret = "sk_test_initial_revision"
	setting.StripePublishableKey = "pk_test_initial_revision"
	t.Cleanup(func() {
		setting.StripeApiSecret = originalSecret
		setting.StripePublishableKey = originalPublishable
	})
	restore := service.ReplaceStripeSubscriptionCheckoutCreatorsForTest(
		func(context.Context, *stripe.CouponParams) (*stripe.Coupon, error) {
			return &stripe.Coupon{ID: "coupon_resolved_initial"}, nil
		},
		func(_ context.Context, params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
			require.Equal(t, "1", params.Metadata["checkout_revision"])
			require.Len(t, params.Discounts, 1)
			require.Equal(t, "coupon_resolved_initial", *params.Discounts[0].Coupon)
			return &stripe.CheckoutSession{ID: "cs_recurring_initial", ClientSecret: "cs_recurring_initial_secret", Status: stripe.CheckoutSessionStatusOpen, PaymentStatus: stripe.CheckoutSessionPaymentStatusUnpaid}, nil
		},
	)
	t.Cleanup(restore)

	created, err := service.CreateStripeSubscriptionCheckout(context.Background(), service.StripeSubscriptionCheckoutInput{
		TradeNo: tradeNo, UserID: fixture.userID, PlanID: 77, PriceID: "price_recurring", Currency: "USD", SubtotalMinor: 2000,
		IdempotencyKey: "subscription-stripe:" + tradeNo, Presentation: service.StripeCheckoutPresentation{RequestedUIMode: "elements", Elements: true},
		DiscountKind: service.SubscriptionDiscountKindInvitation, DiscountAmountMinor: 500, DiscountCurrency: "USD", CheckoutRevision: 0,
	})
	require.NoError(t, err)
	require.Equal(t, "coupon_resolved_initial", created.DiscountSelection.CouponID)
	active, err := model.GetActiveStripeCheckoutRevision(model.StripeCheckoutOrderSubscription, tradeNo)
	require.NoError(t, err)
	require.EqualValues(t, 1, active.Revision)
	require.Equal(t, "coupon_resolved_initial", active.CouponId)
	require.Equal(t, "cs_recurring_initial", stripeCheckoutRevisionSessionID(active))
}

func TestCreateStripeTopUpCheckoutSessionInitializesRevisionOneWhenFeatureEnabled(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	require.NoError(t, model.DB.Where("order_type = ? AND trade_no = ?", model.StripeCheckoutOrderTopUp, fixture.tradeNo).Delete(&model.StripeCheckoutRevision{}).Error)
	require.NoError(t, model.DB.Model(&model.TopUp{}).Where("trade_no = ?", fixture.tradeNo).Updates(map[string]any{"checkout_revision": 0, "gateway_trade_no": ""}).Error)
	topUp := model.GetTopUpByTradeNo(fixture.tradeNo)
	require.NotNil(t, topUp)
	user, err := model.GetUserById(fixture.userID, false)
	require.NoError(t, err)
	originalSecret := setting.StripeApiSecret
	originalPublishable := setting.StripePublishableKey
	originalCreator := stripeCheckoutSessionCreator
	setting.StripeApiSecret = "sk_test_topup_initial"
	setting.StripePublishableKey = "pk_test_topup_initial"
	stripeCheckoutSessionCreator = func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		require.Equal(t, "1", params.Metadata["checkout_revision"])
		return &stripe.CheckoutSession{ID: "cs_topup_initial", ClientSecret: "cs_topup_initial_secret", Status: stripe.CheckoutSessionStatusOpen, PaymentStatus: stripe.CheckoutSessionPaymentStatusUnpaid}, nil
	}
	t.Cleanup(func() {
		setting.StripeApiSecret = originalSecret
		setting.StripePublishableKey = originalPublishable
		stripeCheckoutSessionCreator = originalCreator
	})

	created, active, err := createStripeTopUpCheckoutSession(
		context.Background(), topUp, user, user.StripeCustomer, user.Email,
		&stripeTopUpCheckout{PriceId: topUp.PaymentPriceId, Quantity: 1, Money: topUp.Money, PaymentCurrency: topUp.PaymentCurrency, AmountMinor: topUp.PaymentAmountMinor},
		"", "", false, service.StripeCheckoutPresentation{RequestedUIMode: "elements", Elements: true},
		service.StripeCheckoutDiscountSelection{Source: service.StripeCheckoutDiscountNone}, nil,
	)
	require.NoError(t, err)
	require.Equal(t, "cs_topup_initial", created.ID)
	require.EqualValues(t, 1, active.Revision)
	stored := model.GetTopUpByTradeNo(fixture.tradeNo)
	require.EqualValues(t, 1, stored.CheckoutRevision)
	require.Equal(t, "cs_topup_initial", stored.GatewayTradeNo)
}

func TestCreateOneTimeStripeCheckoutSessionInitializesRevisionOneWhenFeatureEnabled(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	tradeNo := "trade-one-time-wrapper"
	order := &model.SubscriptionOrder{
		UserId: fixture.userID, TradeNo: tradeNo, Status: common.TopUpStatusPending, PaymentProvider: model.PaymentProviderStripe,
		PaymentMethod: model.PaymentMethodStripe, PaymentCurrency: "USD", PaymentAmountMinor: 2000,
	}
	require.NoError(t, model.DB.Create(order).Error)
	user, err := model.GetUserById(fixture.userID, false)
	require.NoError(t, err)
	originalCreator := stripeOneTimeCheckoutSessionForRevisionCreator
	stripeOneTimeCheckoutSessionForRevisionCreator = func(_ context.Context, _ *model.SubscriptionOrder, _ *model.User, revision int64, selection service.StripeCheckoutDiscountSelection, _ ...service.StripeCheckoutPresentation) (*oneTimeStripeCheckoutSession, error) {
		require.EqualValues(t, 1, revision)
		require.Equal(t, service.StripeCheckoutDiscountNone, selection.Source)
		return &oneTimeStripeCheckoutSession{ID: "cs_one_time_initial", ClientSecret: "cs_one_time_initial_secret", Status: "open", PaymentStatus: "unpaid"}, nil
	}
	t.Cleanup(func() { stripeOneTimeCheckoutSessionForRevisionCreator = originalCreator })

	created, err := createOneTimeStripeCheckoutSession(context.Background(), order, user, service.StripeCheckoutPresentation{RequestedUIMode: "elements", Elements: true})
	require.NoError(t, err)
	require.Equal(t, "cs_one_time_initial", created.ID)
	active, err := model.GetActiveStripeCheckoutRevision(model.StripeCheckoutOrderSubscription, tradeNo)
	require.NoError(t, err)
	require.EqualValues(t, 1, active.Revision)
	stored := model.GetSubscriptionOrderByTradeNo(tradeNo)
	require.EqualValues(t, 1, stored.CheckoutRevision)
	require.Equal(t, "cs_one_time_initial", stored.ProviderSessionId)
}

func TestSubscriptionSelfPurchaseInitialResponsesExposeRevisionContract(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	tradeNo := "trade-response-contract"
	order := &model.SubscriptionOrder{
		UserId: fixture.userID, TradeNo: tradeNo, Status: common.TopUpStatusPending, PaymentProvider: model.PaymentProviderStripe,
		PaymentMethod: model.PaymentMethodStripe, PaymentCurrency: "USD", PaymentAmountMinor: 2000,
		CheckoutRevision: 1, ProviderSessionId: "cs_response_contract",
	}
	require.NoError(t, model.DB.Create(order).Error)
	sessionID := order.ProviderSessionId
	require.NoError(t, model.DB.Create(&model.StripeCheckoutRevision{
		OrderType: model.StripeCheckoutOrderSubscription, TradeNo: tradeNo, Revision: 1, UserId: fixture.userID,
		RequestId: "initial:recurring_subscription:" + tradeNo, SelectionDigest: "initial-response", State: model.StripeCheckoutRevisionStateActive,
		DiscountSource: string(service.StripeCheckoutDiscountNone), ProviderSessionId: &sessionID,
	}).Error)
	originalPublishable := setting.StripePublishableKey
	setting.StripePublishableKey = "pk_test_response_contract"
	t.Cleanup(func() { setting.StripePublishableKey = originalPublishable })

	recurring := subscriptionStripePayResponseData(&service.PurchaseSubscriptionResult{Order: order, ClientSecret: "cs_response_secret"})
	require.EqualValues(t, 1, recurring["checkout_revision"])
	require.NotEmpty(t, recurring["checkout_context"])
	require.Equal(t, service.StripeCheckoutDiscountNone, recurring["discount_state"].(StripeCheckoutDiscountState).Source)

	oneTime := subscriptionSelfPurchaseResponse(&service.PurchaseSubscriptionResult{Order: order, ClientSecret: "cs_response_secret"}, "")
	require.EqualValues(t, 1, oneTime.CheckoutRevision)
	require.NotEmpty(t, oneTime.CheckoutContext)
	require.Equal(t, service.StripeCheckoutDiscountNone, oneTime.DiscountState.Source)
}

func TestCreateStripeTopUpCheckoutSessionKeepsLegacyPathWhenFeatureDisabled(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	require.NoError(t, model.DB.Where("order_type = ? AND trade_no = ?", model.StripeCheckoutOrderTopUp, fixture.tradeNo).Delete(&model.StripeCheckoutRevision{}).Error)
	require.NoError(t, model.DB.Model(&model.TopUp{}).Where("trade_no = ?", fixture.tradeNo).Updates(map[string]any{"checkout_revision": 0, "gateway_trade_no": ""}).Error)
	topUp := model.GetTopUpByTradeNo(fixture.tradeNo)
	user, err := model.GetUserById(fixture.userID, false)
	require.NoError(t, err)
	originalFlag := setting.StripePromotionCodeEnabled
	originalSecret := setting.StripeApiSecret
	originalCreator := stripeCheckoutSessionCreator
	setting.StripePromotionCodeEnabled = false
	setting.StripeApiSecret = "sk_test_legacy"
	stripeCheckoutSessionCreator = func(*stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		return &stripe.CheckoutSession{ID: "cs_legacy", ClientSecret: "legacy_secret"}, nil
	}
	t.Cleanup(func() {
		setting.StripePromotionCodeEnabled = originalFlag
		setting.StripeApiSecret = originalSecret
		stripeCheckoutSessionCreator = originalCreator
	})
	created, active, err := createStripeTopUpCheckoutSession(
		context.Background(), topUp, user, user.StripeCustomer, user.Email,
		&stripeTopUpCheckout{PriceId: topUp.PaymentPriceId, Quantity: 1, PaymentCurrency: topUp.PaymentCurrency, AmountMinor: topUp.PaymentAmountMinor},
		"", "", false, service.StripeCheckoutPresentation{RequestedUIMode: "elements", Elements: true},
		service.StripeCheckoutDiscountSelection{Source: service.StripeCheckoutDiscountNone}, nil,
	)
	require.NoError(t, err)
	require.Equal(t, "cs_legacy", created.ID)
	require.Nil(t, active)
	_, err = model.GetStripeCheckoutRevision(model.StripeCheckoutOrderTopUp, fixture.tradeNo, 1)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

type stripeCheckoutDiscountFixture struct {
	userID  int
	tradeNo string
	context string
}

func newStripeCheckoutDiscountFixture(t *testing.T, source service.StripeCheckoutDiscountSource) stripeCheckoutDiscountFixture {
	t.Helper()
	originalDB := model.DB
	originalFlag := setting.StripePromotionCodeEnabled
	originalPriceGetter := stripePriceGetter
	dsn := fmt.Sprintf("file:stripe-checkout-discount-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.SubscriptionOrder{}, &model.StripeCheckoutRevision{}))
	setting.StripePromotionCodeEnabled = true
	stripePriceGetter = func(string, *stripe.PriceParams) (*stripe.Price, error) {
		return &stripe.Price{ID: "price_topup", Product: &stripe.Product{ID: "prod_topup"}}, nil
	}
	restoreRuntime := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "open", PaymentStatus: "unpaid"}, nil
		},
	})
	t.Cleanup(func() {
		restoreRuntime()
		model.DB = originalDB
		setting.StripePromotionCodeEnabled = originalFlag
		stripePriceGetter = originalPriceGetter
	})

	fixture := stripeCheckoutDiscountFixture{userID: 321, tradeNo: "trade-discount"}
	require.NoError(t, db.Create(&model.User{Id: fixture.userID, Username: "stripe-discount-user", StripeCustomer: "cus_fixture"}).Error)
	require.NoError(t, db.Create(&model.TopUp{
		UserId: fixture.userID, TradeNo: fixture.tradeNo, GatewayTradeNo: "cs_old", CheckoutRevision: 1,
		Status: common.TopUpStatusPending, PaymentProvider: model.PaymentProviderStripe, PaymentMethod: model.PaymentMethodStripe,
		PaymentCurrency: "USD", PaymentPriceId: "price_topup", PaymentAmountMinor: 2000, Amount: 20,
	}).Error)
	couponID, promotionID, mask := "", "", ""
	if source == service.StripeCheckoutDiscountInvitation {
		couponID = "coupon_original"
	}
	if source == service.StripeCheckoutDiscountRecall {
		promotionID, mask = "promo_original", "ORIGINAL"
	}
	oldSessionID := "cs_old"
	require.NoError(t, db.Create(&model.StripeCheckoutRevision{
		OrderType: model.StripeCheckoutOrderTopUp, TradeNo: fixture.tradeNo, Revision: 1, UserId: fixture.userID,
		RequestId: "initial:" + fixture.tradeNo, SelectionDigest: "initial-digest", State: model.StripeCheckoutRevisionStateActive,
		DiscountSource: string(source), CouponId: couponID, PromotionCodeId: promotionID, PromotionCodeMask: mask,
		Currency: "USD", SubtotalMinor: 2000, TotalMinor: 2000, ProviderSessionId: &oldSessionID,
	}).Error)
	fixture.context, err = service.SignStripeCheckoutContext(service.StripeCheckoutContextClaims{
		UserID: fixture.userID, PurchaseKind: service.StripeCheckoutPurchaseTopUp, TradeNo: fixture.tradeNo, Revision: 1, ExpiresAt: time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)
	return fixture
}

func (fixture stripeCheckoutDiscountFixture) post(t *testing.T, request stripeCheckoutDiscountRequest) *httptest.ResponseRecorder {
	t.Helper()
	return fixture.postAs(t, fixture.userID, request)
}

func (fixture stripeCheckoutDiscountFixture) postAs(t *testing.T, userID int, request stripeCheckoutDiscountRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := common.Marshal(request)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", userID)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/stripe/checkout/discount", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	UpdateStripeCheckoutDiscount(c)
	return recorder
}

func stripeCheckoutErrorCode(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var envelope struct {
		ErrorCode string `json:"error_code"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &envelope))
	return envelope.ErrorCode
}
