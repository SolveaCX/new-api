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
	require.Equal(t, "success", stripeCheckoutMessage(t, recorder))
	require.Equal(t, "cs_new_secret", envelope.Data.ClientSecret)
	require.EqualValues(t, 2, envelope.Data.CheckoutRevision)
	require.Equal(t, service.StripeCheckoutDiscountManual, envelope.Data.DiscountState.Source)
	require.Equal(t, "SAVE20", envelope.Data.DiscountState.PromotionCodeMasked)
	require.Equal(t, []string{"create:2", "get:cs_new", "get:cs_old", "expire:cs_old"}, events)
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
	require.Equal(t, "success", stripeCheckoutMessage(t, second))
}

func TestUpdateStripeCheckoutDiscountExactReplayReturnsCompletedConflictForPaidCandidate(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	createCalls := 0
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			createCalls++
			return &stripeCheckoutSessionSnapshot{ID: "cs_paid_replay", ClientSecret: "cs_paid_replay_secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			if sessionID == "cs_paid_replay" {
				return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: "cs_paid_replay_secret", Status: "complete", PaymentStatus: "paid"}, nil
			}
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			require.Equal(t, "cs_old", sessionID)
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
	})
	t.Cleanup(restore)
	request := stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-paid-replay", Action: "restore"}

	first := fixture.post(t, request)
	require.Equal(t, http.StatusConflict, first.Code, first.Body.String())
	require.Equal(t, "checkout_already_completed", stripeCheckoutErrorCode(t, first))
	second := fixture.post(t, request)
	require.Equal(t, http.StatusConflict, second.Code, second.Body.String())
	require.Equal(t, "checkout_already_completed", stripeCheckoutErrorCode(t, second))
	require.Equal(t, 1, createCalls)
}

func TestUpdateStripeCheckoutDiscountExactApplyReplaySkipsResolverAndRejectsDifferentPayload(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountInvitation)
	resolveCalls := 0
	createCalls := 0
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		ResolvePromotion: func(context.Context, service.StripeCheckoutPromotionQuery) (service.StripeCheckoutResolvedPromotion, error) {
			resolveCalls++
			if resolveCalls > 1 {
				return service.StripeCheckoutResolvedPromotion{}, service.ErrStripePromotionUnavailable
			}
			return service.StripeCheckoutResolvedPromotion{PromotionCodeID: "promo_replay", MaskedCode: "SAVE20"}, nil
		},
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			createCalls++
			return &stripeCheckoutSessionSnapshot{ID: "cs_apply_replay", ClientSecret: "secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: "secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
	})
	t.Cleanup(restore)
	request := stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-apply-replay", Action: "apply", PromotionCode: " Save20 "}

	require.Equal(t, http.StatusOK, fixture.post(t, request).Code)
	require.Equal(t, http.StatusOK, fixture.post(t, request).Code)
	different := request
	different.PromotionCode = "OTHER"
	recorder := fixture.post(t, different)
	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Equal(t, "checkout_revision_conflict", stripeCheckoutErrorCode(t, recorder))
	require.Equal(t, 1, resolveCalls)
	require.Equal(t, 1, createCalls)
	require.NotContains(t, recorder.Body.String(), "OTHER")
}

func TestUpdateStripeCheckoutDiscountStaleNewApplyConflictsBeforeResolver(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	resolveCalls := 0
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		ResolvePromotion: func(context.Context, service.StripeCheckoutPromotionQuery) (service.StripeCheckoutResolvedPromotion, error) {
			resolveCalls++
			return service.StripeCheckoutResolvedPromotion{}, service.ErrStripePromotionUnavailable
		},
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: "cs_stale_winner", ClientSecret: "secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: "secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
	})
	t.Cleanup(restore)
	require.Equal(t, http.StatusOK, fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-stale-winner", Action: "restore"}).Code)

	recorder := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-stale-invalid", Action: "apply", PromotionCode: "INVALID-RAW-CODE"})
	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Equal(t, "checkout_revision_conflict", stripeCheckoutErrorCode(t, recorder))
	require.Zero(t, resolveCalls)
	require.NotContains(t, recorder.Body.String(), "INVALID-RAW-CODE")
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

func TestUpdateStripeCheckoutDiscountActivationConflictWithoutWinnerKeepsCandidateForRetry(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	createCalls := 0
	activateCalls := 0
	oldExpired := false
	candidateExpired := false
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			createCalls++
			return &stripeCheckoutSessionSnapshot{ID: "cs_cas_loser", ClientSecret: "secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			if sessionID == "cs_old" && oldExpired {
				return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
			}
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			if sessionID == "cs_old" {
				oldExpired = true
			} else {
				candidateExpired = true
			}
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
		ActivateRevision: func(input model.StripeCheckoutRevisionActivation) (*model.StripeCheckoutRevision, error) {
			activateCalls++
			if activateCalls == 1 {
				return nil, model.ErrStripeCheckoutRevisionConflict
			}
			return model.ActivateStripeCheckoutRevision(input)
		},
	})
	t.Cleanup(restore)
	request := stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-cas-no-winner", Action: "restore"}

	first := fixture.post(t, request)
	require.Equal(t, http.StatusInternalServerError, first.Code)
	revision, err := model.GetStripeCheckoutRevisionByRequestID(model.StripeCheckoutOrderTopUp, fixture.tradeNo, request.RequestID)
	require.NoError(t, err)
	require.Equal(t, model.StripeCheckoutRevisionStatePreparing, revision.State)
	require.False(t, candidateExpired)

	second := fixture.post(t, request)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	require.Equal(t, 1, createCalls)
	require.Equal(t, 2, activateCalls)
}

func TestUpdateStripeCheckoutDiscountActivationConflictCompletedOrderKeepsPaidCandidateAsWinner(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	candidateID := "cs_candidate_paid_before_activate"
	statusFlipped := false
	abandoned := false
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: candidateID, ClientSecret: "candidate_secret", Status: "complete", PaymentStatus: "paid"}, nil
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			if sessionID == "cs_old" {
				return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
			}
			return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: "candidate_secret", Status: "complete", PaymentStatus: "paid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			require.Equal(t, "cs_old", sessionID)
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
		ActivateRevision: func(input model.StripeCheckoutRevisionActivation) (*model.StripeCheckoutRevision, error) {
			require.NoError(t, model.DB.Model(&model.TopUp{}).Where("trade_no = ?", fixture.tradeNo).Update("status", common.TopUpStatusSuccess).Error)
			statusFlipped = true
			return nil, model.ErrStripeCheckoutRevisionConflict
		},
		AbandonRevision: func(int64) error {
			abandoned = true
			return nil
		},
	})
	t.Cleanup(restore)

	recorder := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-paid-before-activate", Action: "restore"})
	require.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())
	require.Equal(t, "checkout_already_completed", stripeCheckoutErrorCode(t, recorder))
	require.True(t, statusFlipped)
	require.False(t, abandoned, "paid candidate must not be abandoned just because the local order is already successful")
	row, err := model.GetStripeCheckoutRevisionByRequestID(model.StripeCheckoutOrderTopUp, fixture.tradeNo, "req-paid-before-activate")
	require.NoError(t, err)
	require.Equal(t, model.StripeCheckoutRevisionStateActive, row.State)
	require.Equal(t, candidateID, stripeCheckoutRevisionSessionID(row))
}

func TestUpdateStripeCheckoutDiscountActivationConflictRefetchesPaidCandidateAfterConflict(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	candidateID := "cs_candidate_refetch_paid"
	getCalls := 0
	activateCalls := 0
	candidateRow := model.StripeCheckoutRevision{
		OrderType:       model.StripeCheckoutOrderTopUp,
		TradeNo:         fixture.tradeNo,
		Revision:        2,
		UserId:          fixture.userID,
		RequestId:       "req-refetch-paid",
		SelectionDigest: "digest-refetch-paid",
		State:           model.StripeCheckoutRevisionStatePreparing,
		DiscountSource:  string(service.StripeCheckoutDiscountNone),
	}
	require.NoError(t, model.DB.Create(&candidateRow).Error)
	candidateRow.ProviderSessionId = &candidateID
	require.NoError(t, model.DB.Model(&model.StripeCheckoutRevision{}).Where("id = ?", candidateRow.Id).Update("provider_session_id", candidateID).Error)
	stored, err := model.GetStripeCheckoutRevisionByRequestID(model.StripeCheckoutOrderTopUp, fixture.tradeNo, "req-refetch-paid")
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			getCalls++
			if sessionID == "cs_old" {
				return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "open", PaymentStatus: "unpaid"}, nil
			}
			if getCalls == 1 {
				return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: "candidate_secret", Status: "open", PaymentStatus: "unpaid"}, nil
			}
			return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: "candidate_secret", Status: "complete", PaymentStatus: "paid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
		ActivateRevision: func(input model.StripeCheckoutRevisionActivation) (*model.StripeCheckoutRevision, error) {
			activateCalls++
			require.NoError(t, model.DB.Model(&model.TopUp{}).Where("trade_no = ?", fixture.tradeNo).Update("status", common.TopUpStatusSuccess).Error)
			return nil, model.ErrStripeCheckoutRevisionConflict
		},
	})
	t.Cleanup(restore)

	claims := service.StripeCheckoutContextClaims{UserID: fixture.userID, PurchaseKind: service.StripeCheckoutPurchaseTopUp, TradeNo: fixture.tradeNo, Revision: 1}
	purchase := stripeCheckoutPurchase{Kind: service.StripeCheckoutPurchaseTopUp, OrderType: model.StripeCheckoutOrderTopUp, TradeNo: fixture.tradeNo, UserID: fixture.userID, Revision: 1, OldSessionID: "cs_old"}
	candidate := &stripeCheckoutSessionSnapshot{ID: candidateID, ClientSecret: "candidate_secret", Status: "open", PaymentStatus: "unpaid"}
	finishStripeCheckoutDiscountTransition(ctx, claims, purchase, stored, candidate)
	require.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())
	require.Equal(t, "checkout_already_completed", stripeCheckoutErrorCode(t, recorder))
	require.Equal(t, 2, getCalls)
	require.Equal(t, 1, activateCalls)
	row, err := model.GetStripeCheckoutRevisionByRequestID(model.StripeCheckoutOrderTopUp, fixture.tradeNo, "req-refetch-paid")
	require.NoError(t, err)
	require.Equal(t, model.StripeCheckoutRevisionStateActive, row.State)
}

func TestUpdateStripeCheckoutDiscountActivationConflictWithRealWinnerCleansLoser(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	expired := make([]string, 0, 2)
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: "cs_real_loser", ClientSecret: "loser_secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: sessionID + "_secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			expired = append(expired, sessionID)
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
		ActivateRevision: func(model.StripeCheckoutRevisionActivation) (*model.StripeCheckoutRevision, error) {
			winnerID := "cs_real_winner"
			require.NoError(t, model.DB.Model(&model.StripeCheckoutRevision{}).Where("order_type = ? AND trade_no = ? AND revision = ?", model.StripeCheckoutOrderTopUp, fixture.tradeNo, 1).Update("state", model.StripeCheckoutRevisionStateSuperseded).Error)
			require.NoError(t, model.DB.Create(&model.StripeCheckoutRevision{
				OrderType: model.StripeCheckoutOrderTopUp, TradeNo: fixture.tradeNo, Revision: 3, UserId: fixture.userID,
				RequestId: "req-real-winner", SelectionDigest: "winner", State: model.StripeCheckoutRevisionStateActive,
				DiscountSource: string(service.StripeCheckoutDiscountNone), ProviderSessionId: &winnerID,
			}).Error)
			require.NoError(t, model.DB.Model(&model.TopUp{}).Where("trade_no = ?", fixture.tradeNo).Updates(map[string]any{"checkout_revision": 3, "gateway_trade_no": winnerID}).Error)
			return nil, model.ErrStripeCheckoutRevisionConflict
		},
	})
	t.Cleanup(restore)

	recorder := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-real-loser", Action: "restore"})
	require.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())
	require.Equal(t, "checkout_revision_conflict", stripeCheckoutErrorCode(t, recorder))
	require.Equal(t, []string{"cs_old", "cs_real_loser"}, expired)
	revision, err := model.GetStripeCheckoutRevisionByRequestID(model.StripeCheckoutOrderTopUp, fixture.tradeNo, "req-real-loser")
	require.NoError(t, err)
	require.Equal(t, model.StripeCheckoutRevisionStateAbandoned, revision.State)
}

func TestUpdateStripeCheckoutDiscountStaleReplayFinishesDifferentWinnerCleanup(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	request := stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-retry-winner-cleanup", Action: "restore"}
	createCalls := 0
	loserExpireCalls := 0
	loserExpired := false
	abandonCalls := 0
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			createCalls++
			return &stripeCheckoutSessionSnapshot{ID: "cs_retry_cleanup_loser", ClientSecret: "loser_secret"}, nil
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			switch sessionID {
			case "cs_retry_cleanup_loser":
				status := "open"
				if loserExpired {
					status = "expired"
				}
				return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: "loser_secret", Status: status, PaymentStatus: "unpaid"}, nil
			case "cs_old", "cs_retry_cleanup_winner":
				return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: sessionID + "_secret", Status: "open", PaymentStatus: "unpaid"}, nil
			default:
				return nil, errors.New("unexpected Session")
			}
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			if sessionID == "cs_retry_cleanup_loser" {
				loserExpireCalls++
				loserExpired = true
			}
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
		ActivateRevision: func(model.StripeCheckoutRevisionActivation) (*model.StripeCheckoutRevision, error) {
			winnerID := "cs_retry_cleanup_winner"
			require.NoError(t, model.DB.Model(&model.StripeCheckoutRevision{}).Where("order_type = ? AND trade_no = ? AND revision = ?", model.StripeCheckoutOrderTopUp, fixture.tradeNo, 1).Update("state", model.StripeCheckoutRevisionStateSuperseded).Error)
			require.NoError(t, model.DB.Create(&model.StripeCheckoutRevision{
				OrderType: model.StripeCheckoutOrderTopUp, TradeNo: fixture.tradeNo, Revision: 3, UserId: fixture.userID,
				RequestId: "req-retry-cleanup-winner", SelectionDigest: "winner", State: model.StripeCheckoutRevisionStateActive,
				DiscountSource: string(service.StripeCheckoutDiscountNone), ProviderSessionId: &winnerID,
			}).Error)
			require.NoError(t, model.DB.Model(&model.TopUp{}).Where("trade_no = ?", fixture.tradeNo).Updates(map[string]any{"checkout_revision": 3, "gateway_trade_no": winnerID}).Error)
			return nil, model.ErrStripeCheckoutRevisionConflict
		},
		AbandonRevision: func(revisionID int64) error {
			abandonCalls++
			if abandonCalls == 1 {
				return errors.New("abandon response lost")
			}
			return model.AbandonStripeCheckoutRevision(revisionID)
		},
	})
	t.Cleanup(restore)

	first := fixture.post(t, request)
	require.Equal(t, http.StatusInternalServerError, first.Code, first.Body.String())
	row, err := model.GetStripeCheckoutRevisionByRequestID(model.StripeCheckoutOrderTopUp, fixture.tradeNo, request.RequestID)
	require.NoError(t, err)
	require.Equal(t, model.StripeCheckoutRevisionStatePreparing, row.State)
	require.True(t, loserExpired)

	second := fixture.post(t, request)
	require.Equal(t, http.StatusConflict, second.Code, second.Body.String())
	require.Equal(t, "checkout_revision_conflict", stripeCheckoutErrorCode(t, second))
	row, err = model.GetStripeCheckoutRevisionByRequestID(model.StripeCheckoutOrderTopUp, fixture.tradeNo, request.RequestID)
	require.NoError(t, err)
	require.Equal(t, model.StripeCheckoutRevisionStateAbandoned, row.State)
	require.Equal(t, 1, createCalls)
	require.Equal(t, 1, loserExpireCalls)
	require.Equal(t, 2, abandonCalls)
}

func TestUpdateStripeCheckoutDiscountActivationConflictWithSameCandidateReturnsSuccess(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	expired := make([]string, 0, 1)
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: "cs_same_candidate", ClientSecret: "same_secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: "same_secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			expired = append(expired, sessionID)
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
		ActivateRevision: func(input model.StripeCheckoutRevisionActivation) (*model.StripeCheckoutRevision, error) {
			_, err := model.ActivateStripeCheckoutRevision(input)
			require.NoError(t, err)
			return nil, model.ErrStripeCheckoutRevisionConflict
		},
	})
	t.Cleanup(restore)

	recorder := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-same-candidate", Action: "restore"})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, []string{"cs_old"}, expired)
	active, err := model.GetActiveStripeCheckoutRevision(model.StripeCheckoutOrderTopUp, fixture.tradeNo)
	require.NoError(t, err)
	require.Equal(t, "cs_same_candidate", stripeCheckoutRevisionSessionID(active))
}

func TestStripeCheckoutDiscountRequestDigestIsKeyedAndBindsPredecessor(t *testing.T) {
	request := stripeCheckoutDiscountRequest{ExpectedRevision: 1, RequestID: "req-digest", Action: "apply", PromotionCode: " Save20 "}
	digest := stripeCheckoutDiscountRequestDigest(model.StripeCheckoutOrderTopUp, "trade-digest", request)
	require.Equal(t, digest, stripeCheckoutDiscountRequestDigest(model.StripeCheckoutOrderTopUp, "trade-digest", stripeCheckoutDiscountRequest{
		ExpectedRevision: 1, RequestID: "req-digest", Action: " APPLY ", PromotionCode: "save20",
	}))
	require.NotEqual(t, digest, stripeCheckoutDiscountRequestDigest(model.StripeCheckoutOrderTopUp, "trade-digest", stripeCheckoutDiscountRequest{
		ExpectedRevision: 3, RequestID: "req-digest", Action: "apply", PromotionCode: "save20",
	}))
	require.NotEqual(t, digest, stripeCheckoutDiscountRequestDigest(model.StripeCheckoutOrderTopUp, "trade-other", request))
	require.NotEqual(t, digest, stripeCheckoutDiscountRequestDigest(model.StripeCheckoutOrderTopUp, "trade-digest", stripeCheckoutDiscountRequest{
		ExpectedRevision: 1, RequestID: "req-other", Action: "apply", PromotionCode: "save20",
	}))
	require.NotContains(t, digest, "SAVE20")
	require.Equal(t, 64, len(digest))
}

func TestUpdateStripeCheckoutDiscountAmbiguousCreateRemainsPreparingAndExactRetryRecovers(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	createCalls := 0
	providerSessions := map[string]struct{}{}
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			createCalls++
			providerSessions["cs_ambiguous_create"] = struct{}{}
			if createCalls == 1 {
				return nil, errors.New("provider accepted but response timed out")
			}
			return &stripeCheckoutSessionSnapshot{ID: "cs_ambiguous_create", ClientSecret: "secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: "secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
	})
	t.Cleanup(restore)
	request := stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-ambiguous-create", Action: "restore"}

	first := fixture.post(t, request)
	require.Equal(t, http.StatusInternalServerError, first.Code)
	row, err := model.GetStripeCheckoutRevisionByRequestID(model.StripeCheckoutOrderTopUp, fixture.tradeNo, request.RequestID)
	require.NoError(t, err)
	require.Equal(t, model.StripeCheckoutRevisionStatePreparing, row.State)
	fresh := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-fenced-by-ambiguous", Action: "restore"})
	require.Equal(t, http.StatusConflict, fresh.Code)

	second := fixture.post(t, request)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	require.Equal(t, 2, createCalls)
	require.Len(t, providerSessions, 1)
}

func TestUpdateStripeCheckoutDiscountDefinitiveCreateRejectionAbandonsRevision(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		ResolvePromotion: func(context.Context, service.StripeCheckoutPromotionQuery) (service.StripeCheckoutResolvedPromotion, error) {
			return service.StripeCheckoutResolvedPromotion{PromotionCodeID: "promo_rejected", MaskedCode: "REJECTED"}, nil
		},
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			return nil, &stripe.Error{Type: stripe.ErrorTypeInvalidRequest, Param: "discounts", Msg: "promotion is not eligible"}
		},
	})
	t.Cleanup(restore)

	recorder := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-definitive-rejection", Action: "apply", PromotionCode: "REJECTED"})
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, "promotion_code_ineligible", stripeCheckoutErrorCode(t, recorder))
	row, err := model.GetStripeCheckoutRevisionByRequestID(model.StripeCheckoutOrderTopUp, fixture.tradeNo, "req-definitive-rejection")
	require.NoError(t, err)
	require.Equal(t, model.StripeCheckoutRevisionStateAbandoned, row.State)
	require.NotContains(t, recorder.Body.String(), "promotion is not eligible")
}

func TestStripeCheckoutDefinitiveSessionRejectionClassification(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "discounts param", err: &stripe.Error{Type: stripe.ErrorTypeInvalidRequest, Param: "discounts"}, want: true},
		{name: "promotion param", err: &stripe.Error{Type: stripe.ErrorTypeInvalidRequest, Param: "discounts[0][promotion_code]"}, want: true},
		{name: "coupon param", err: &stripe.Error{Type: stripe.ErrorTypeInvalidRequest, Param: "coupon"}, want: true},
		{name: "promotion code", err: &stripe.Error{Type: stripe.ErrorTypeInvalidRequest, Code: stripe.ErrorCode("promotion_code_inactive")}, want: true},
		{name: "coupon code", err: &stripe.Error{Type: stripe.ErrorTypeInvalidRequest, Code: stripe.ErrorCode("coupon_expired")}, want: true},
		{name: "price param", err: &stripe.Error{Type: stripe.ErrorTypeInvalidRequest, Param: "line_items[0][price]"}},
		{name: "success url", err: &stripe.Error{Type: stripe.ErrorTypeInvalidRequest, Param: "success_url"}},
		{name: "customer", err: &stripe.Error{Type: stripe.ErrorTypeInvalidRequest, Param: "customer"}},
		{name: "generic invalid request", err: &stripe.Error{Type: stripe.ErrorTypeInvalidRequest}},
		{name: "resource missing price", err: &stripe.Error{Type: stripe.ErrorTypeInvalidRequest, Code: stripe.ErrorCode("resource_missing"), Param: "line_items[0][price]"}},
		{name: "api error discount param", err: &stripe.Error{Type: stripe.ErrorTypeAPI, Param: "discounts"}},
		{name: "transport", err: errors.New("connection reset")},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, service.IsStripeCheckoutDefinitiveSessionRejection(test.err))
		})
	}
}

func TestUpdateStripeCheckoutDiscountNonDiscountInvalidRequestRemainsRecoverable(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		ResolvePromotion: func(context.Context, service.StripeCheckoutPromotionQuery) (service.StripeCheckoutResolvedPromotion, error) {
			return service.StripeCheckoutResolvedPromotion{PromotionCodeID: "promo_retry", MaskedCode: "RETRY"}, nil
		},
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			return nil, &stripe.Error{Type: stripe.ErrorTypeInvalidRequest, Param: "line_items[0][price]", Msg: "RAW PROVIDER INTERNAL DETAIL"}
		},
	})
	t.Cleanup(restore)

	recorder := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-nondiscount-invalid", Action: "apply", PromotionCode: "RAW-BUYER-CODE"})
	require.Equal(t, http.StatusInternalServerError, recorder.Code, recorder.Body.String())
	require.Equal(t, "checkout_replacement_failed", stripeCheckoutErrorCode(t, recorder))
	require.NotContains(t, recorder.Body.String(), "RAW-BUYER-CODE")
	require.NotContains(t, recorder.Body.String(), "RAW PROVIDER INTERNAL DETAIL")
	row, err := model.GetStripeCheckoutRevisionByRequestID(model.StripeCheckoutOrderTopUp, fixture.tradeNo, "req-nondiscount-invalid")
	require.NoError(t, err)
	require.Equal(t, model.StripeCheckoutRevisionStatePreparing, row.State)
}

func TestUpdateStripeCheckoutDiscountRecoveredTerminalCandidateExpiresOrConvergesPaid(t *testing.T) {
	for _, test := range []struct {
		name           string
		status         string
		payment        string
		wantStatus     int
		wantCode       string
		wantState      string
		wantOldTouched bool
		wantRevision   int64
		wantSessionID  string
		wantOldExpired bool
	}{
		{name: "expired", status: "expired", payment: "unpaid", wantStatus: http.StatusInternalServerError, wantCode: "checkout_replacement_failed", wantState: model.StripeCheckoutRevisionStateAbandoned, wantRevision: 1, wantSessionID: "cs_old"},
		{name: "complete", status: "complete", payment: "paid", wantStatus: http.StatusConflict, wantCode: "checkout_already_completed", wantState: model.StripeCheckoutRevisionStateActive, wantOldTouched: true, wantRevision: 2, wantSessionID: "cs_terminal_complete", wantOldExpired: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
			request := stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-terminal-" + test.name, Action: "restore"}
			candidateID := "cs_terminal_" + test.name
			require.NoError(t, model.DB.Create(&model.StripeCheckoutRevision{
				OrderType: model.StripeCheckoutOrderTopUp, TradeNo: fixture.tradeNo, Revision: 2, UserId: fixture.userID,
				RequestId: request.RequestID, SelectionDigest: stripeCheckoutDiscountRequestDigest(model.StripeCheckoutOrderTopUp, fixture.tradeNo, request),
				State: model.StripeCheckoutRevisionStatePreparing, DiscountSource: string(service.StripeCheckoutDiscountNone), ProviderSessionId: &candidateID,
			}).Error)
			oldTouched := false
			oldExpired := false
			restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
				GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
					if sessionID == "cs_old" {
						oldTouched = true
						return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "open", PaymentStatus: "unpaid"}, nil
					}
					return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: test.status, PaymentStatus: test.payment}, nil
				},
				ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
					require.Equal(t, "cs_old", sessionID)
					oldExpired = true
					return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
				},
			})
			t.Cleanup(restore)

			recorder := fixture.post(t, request)
			require.Equal(t, test.wantStatus, recorder.Code, recorder.Body.String())
			require.Equal(t, test.wantCode, stripeCheckoutErrorCode(t, recorder))
			require.Equal(t, test.wantOldTouched, oldTouched)
			require.Equal(t, test.wantOldExpired, oldExpired)
			row, err := model.GetStripeCheckoutRevisionByRequestID(model.StripeCheckoutOrderTopUp, fixture.tradeNo, request.RequestID)
			require.NoError(t, err)
			require.Equal(t, test.wantState, row.State)
			stored := model.GetTopUpByTradeNo(fixture.tradeNo)
			require.EqualValues(t, test.wantRevision, stored.CheckoutRevision)
			require.Equal(t, test.wantSessionID, stored.GatewayTradeNo)
		})
	}
}

func TestUpdateStripeCheckoutDiscountOldPaidWinsOverPaidCandidate(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	request := stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-both-paid", Action: "restore"}
	candidateID := "cs_candidate_paid"
	require.NoError(t, model.DB.Create(&model.StripeCheckoutRevision{
		OrderType: model.StripeCheckoutOrderTopUp, TradeNo: fixture.tradeNo, Revision: 2, UserId: fixture.userID,
		RequestId: request.RequestID, SelectionDigest: stripeCheckoutDiscountRequestDigest(model.StripeCheckoutOrderTopUp, fixture.tradeNo, request),
		State: model.StripeCheckoutRevisionStatePreparing, DiscountSource: string(service.StripeCheckoutDiscountNone), ProviderSessionId: &candidateID,
	}).Error)
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "complete", PaymentStatus: "paid"}, nil
		},
		ExpireSession: func(context.Context, service.StripeCheckoutPurchaseKind, string) (*stripeCheckoutSessionSnapshot, error) {
			return nil, errors.New("must not expire either paid Session")
		},
	})
	t.Cleanup(restore)

	recorder := fixture.post(t, request)
	require.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())
	require.Equal(t, "checkout_already_completed", stripeCheckoutErrorCode(t, recorder))
	row, err := model.GetStripeCheckoutRevisionByRequestID(model.StripeCheckoutOrderTopUp, fixture.tradeNo, request.RequestID)
	require.NoError(t, err)
	require.Equal(t, model.StripeCheckoutRevisionStateAbandoned, row.State)
	stored := model.GetTopUpByTradeNo(fixture.tradeNo)
	require.EqualValues(t, 1, stored.CheckoutRevision)
	require.Equal(t, "cs_old", stored.GatewayTradeNo)
}

func TestUpdateStripeCheckoutDiscountRefetchesCreatedCandidateForEveryPurchaseKind(t *testing.T) {
	for _, kind := range []service.StripeCheckoutPurchaseKind{
		service.StripeCheckoutPurchaseTopUp,
		service.StripeCheckoutPurchaseOneTimeSubscription,
		service.StripeCheckoutPurchaseRecurringSubscription,
	} {
		t.Run(string(kind), func(t *testing.T) {
			fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
			tradeNo := "trade-refetch-" + string(kind)
			orderType := stripeCheckoutOrderType(kind)
			oldID := "cs_old_refetch"
			require.NoError(t, model.DB.Create(&model.StripeCheckoutRevision{
				OrderType: orderType, TradeNo: tradeNo, Revision: 1, UserId: fixture.userID,
				RequestId: "initial:" + string(kind) + ":" + tradeNo, SelectionDigest: "initial-refetch", State: model.StripeCheckoutRevisionStateActive,
				DiscountSource: string(service.StripeCheckoutDiscountNone), ProviderSessionId: &oldID,
			}).Error)
			if kind == service.StripeCheckoutPurchaseTopUp {
				require.NoError(t, model.DB.Create(&model.TopUp{
					UserId: fixture.userID, TradeNo: tradeNo, GatewayTradeNo: oldID, CheckoutRevision: 1,
					Status: common.TopUpStatusPending, PaymentProvider: model.PaymentProviderStripe, PaymentMethod: model.PaymentMethodStripe,
				}).Error)
			} else {
				require.NoError(t, model.DB.Create(&model.SubscriptionOrder{
					UserId: fixture.userID, TradeNo: tradeNo, ProviderSessionId: oldID, CheckoutRevision: 1,
					Status: common.TopUpStatusPending, PaymentProvider: model.PaymentProviderStripe, PaymentMethod: model.PaymentMethodStripe,
				}).Error)
			}
			contextToken, err := service.SignStripeCheckoutContext(service.StripeCheckoutContextClaims{
				UserID: fixture.userID, PurchaseKind: kind, TradeNo: tradeNo, Revision: 1, ExpiresAt: time.Now().Add(time.Hour).Unix(),
			})
			require.NoError(t, err)
			getCalls := 0
			recordCalls := 0
			restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
				LoadPurchase: func(context.Context, service.StripeCheckoutContextClaims) (stripeCheckoutPurchase, error) {
					return stripeCheckoutPurchase{Kind: kind, OrderType: orderType, TradeNo: tradeNo, UserID: fixture.userID, Revision: 1, OldSessionID: oldID}, nil
				},
				PrepareRevision: func(input model.StripeCheckoutRevisionPrepare) (*model.StripeCheckoutRevision, bool, error) {
					row := &model.StripeCheckoutRevision{
						OrderType: input.OrderType, TradeNo: input.TradeNo, Revision: 2, UserId: input.UserID,
						RequestId: input.RequestID, SelectionDigest: input.SelectionDigest, State: model.StripeCheckoutRevisionStatePreparing,
						DiscountSource: input.DiscountSource, ReplacedSource: input.ReplacedSource, CouponId: input.CouponID,
						PromotionCodeId: input.PromotionCodeID, PromotionCodeMask: input.PromotionCodeMask,
					}
					require.NoError(t, model.DB.Create(row).Error)
					return row, false, nil
				},
				CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
					return &stripeCheckoutSessionSnapshot{ID: "cs_created_terminal", Status: "open", PaymentStatus: "unpaid"}, nil
				},
				GetSession: func(_ context.Context, gotKind service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
					require.Equal(t, kind, gotKind)
					require.Equal(t, "cs_created_terminal", sessionID)
					getCalls++
					return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
				},
				RecordCandidate: func(model.StripeCheckoutRevisionCandidate) (*model.StripeCheckoutRevision, error) {
					recordCalls++
					return nil, errors.New("must not record expired candidate")
				},
			})
			t.Cleanup(restore)

			recorder := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: contextToken, ExpectedRevision: 1, RequestID: "req-refetch-" + string(kind), Action: "restore"})
			require.Equal(t, http.StatusInternalServerError, recorder.Code, recorder.Body.String())
			require.Equal(t, 1, getCalls)
			require.Zero(t, recordCalls)
			row, err := model.GetStripeCheckoutRevisionByRequestID(orderType, tradeNo, "req-refetch-"+string(kind))
			require.NoError(t, err)
			require.Equal(t, model.StripeCheckoutRevisionStateAbandoned, row.State)
		})
	}
}

func TestUpdateStripeCheckoutDiscountInvoiceLookupFailureFailsClosed(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	require.NoError(t, model.DB.Migrator().DropTable(&model.PaymentInvoice{}))
	createCalls := 0
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			createCalls++
			return nil, errors.New("must not create")
		},
	})
	t.Cleanup(restore)

	recorder := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-invoice-db-failure", Action: "restore"})
	require.Equal(t, http.StatusInternalServerError, recorder.Code, recorder.Body.String())
	require.Equal(t, "checkout_replacement_failed", stripeCheckoutErrorCode(t, recorder))
	require.Zero(t, createCalls)
}

func TestCreateStripeCheckoutCandidateInvoiceLookupFailureStopsBeforeStripe(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	topUp := model.GetTopUpByTradeNo(fixture.tradeNo)
	user, err := model.GetUserById(fixture.userID, false)
	require.NoError(t, err)
	require.NoError(t, model.DB.Migrator().DropTable(&model.PaymentInvoice{}))
	originalCreator := stripeCheckoutSessionCreator
	createCalls := 0
	stripeCheckoutSessionCreator = func(*stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		createCalls++
		return nil, errors.New("must not call Stripe")
	}
	t.Cleanup(func() { stripeCheckoutSessionCreator = originalCreator })

	_, err = createStripeCheckoutCandidate(context.Background(), stripeCheckoutPurchase{
		Kind: service.StripeCheckoutPurchaseTopUp, OrderType: model.StripeCheckoutOrderTopUp, TradeNo: fixture.tradeNo,
		UserID: fixture.userID, CustomerID: user.StripeCustomer, TopUp: topUp, User: user,
	}, 2, service.StripeCheckoutDiscountSelection{Source: service.StripeCheckoutDiscountNone})
	require.Error(t, err)
	require.Zero(t, createCalls)
}

func TestUpdateStripeCheckoutDiscountMapsOnlyUserPromotionFailuresToBadRequest(t *testing.T) {
	for _, test := range []struct {
		name            string
		action          string
		resolverErr     error
		deleteCanonical bool
		priceErr        error
		wantStatus      int
		wantCode        string
	}{
		{name: "unavailable", action: "apply", resolverErr: service.ErrStripePromotionUnavailable, wantStatus: http.StatusBadRequest, wantCode: "promotion_code_invalid"},
		{name: "ambiguous", action: "apply", resolverErr: service.ErrStripePromotionAmbiguous, wantStatus: http.StatusBadRequest, wantCode: "promotion_code_ambiguous"},
		{name: "resolver lookup", action: "apply", resolverErr: service.ErrStripePromotionLookup, wantStatus: http.StatusInternalServerError, wantCode: "checkout_replacement_failed"},
		{name: "price lookup", action: "apply", priceErr: errors.New("price database unavailable"), wantStatus: http.StatusInternalServerError, wantCode: "checkout_replacement_failed"},
		{name: "canonical lookup", action: "restore", deleteCanonical: true, wantStatus: http.StatusInternalServerError, wantCode: "checkout_replacement_failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
			if test.deleteCanonical {
				require.NoError(t, model.DB.Where("order_type = ? AND trade_no = ? AND revision = ?", model.StripeCheckoutOrderTopUp, fixture.tradeNo, 1).Delete(&model.StripeCheckoutRevision{}).Error)
			}
			if test.priceErr != nil {
				stripePriceGetter = func(string, *stripe.PriceParams) (*stripe.Price, error) { return nil, test.priceErr }
			}
			restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
				ResolvePromotion: func(context.Context, service.StripeCheckoutPromotionQuery) (service.StripeCheckoutResolvedPromotion, error) {
					return service.StripeCheckoutResolvedPromotion{}, test.resolverErr
				},
			})
			t.Cleanup(restore)
			request := stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-error-map-" + test.name, Action: test.action}
			if test.action == "apply" {
				request.PromotionCode = "RAW-SENSITIVE-CODE"
			}
			recorder := fixture.post(t, request)
			require.Equal(t, test.wantStatus, recorder.Code, recorder.Body.String())
			require.Equal(t, test.wantCode, stripeCheckoutErrorCode(t, recorder))
			require.NotContains(t, recorder.Body.String(), "RAW-SENSITIVE-CODE")
		})
	}
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

func TestUpdateStripeCheckoutDiscountAbandonedGapActivatesFreshMonotonicRevisionAndReplays(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	createCalls := 0
	abandonedSession := "cs_abandoned"
	require.NoError(t, model.DB.Create(&model.StripeCheckoutRevision{
		OrderType: model.StripeCheckoutOrderTopUp, TradeNo: fixture.tradeNo, Revision: 2, UserId: fixture.userID,
		RequestId: "req-abandoned-gap", SelectionDigest: "gap", State: model.StripeCheckoutRevisionStateAbandoned,
		DiscountSource: string(service.StripeCheckoutDiscountNone), ProviderSessionId: &abandonedSession,
	}).Error)
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(_ context.Context, _ stripeCheckoutPurchase, revision int64, _ service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			createCalls++
			require.EqualValues(t, 3, revision)
			return &stripeCheckoutSessionSnapshot{ID: "cs_after_gap", ClientSecret: "gap_secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: "gap_secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
	})
	t.Cleanup(restore)
	request := stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-after-gap", Action: "restore"}

	first := fixture.post(t, request)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	require.Equal(t, http.StatusOK, fixture.post(t, request).Code)
	require.Equal(t, 1, createCalls)
	active, err := model.GetActiveStripeCheckoutRevision(model.StripeCheckoutOrderTopUp, fixture.tradeNo)
	require.NoError(t, err)
	require.EqualValues(t, 3, active.Revision)
	var preparing int64
	require.NoError(t, model.DB.Model(&model.StripeCheckoutRevision{}).Where("order_type = ? AND trade_no = ? AND state = ?", model.StripeCheckoutOrderTopUp, fixture.tradeNo, model.StripeCheckoutRevisionStatePreparing).Count(&preparing).Error)
	require.Zero(t, preparing)
}

func TestUpdateStripeCheckoutDiscountRecordInterruptionReplaysAttachedCandidate(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	createCalls := 0
	recordCalls := 0
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			createCalls++
			return &stripeCheckoutSessionSnapshot{ID: "cs_record_interrupted", ClientSecret: "secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		RecordCandidate: func(input model.StripeCheckoutRevisionCandidate) (*model.StripeCheckoutRevision, error) {
			recordCalls++
			stored, err := model.RecordStripeCheckoutCandidate(input)
			if recordCalls == 1 && err == nil {
				return nil, errors.New("response lost after durable record")
			}
			return stored, err
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: "secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
	})
	t.Cleanup(restore)
	request := stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-record-interrupted", Action: "restore"}

	first := fixture.post(t, request)
	require.Equal(t, http.StatusInternalServerError, first.Code)
	row, err := model.GetStripeCheckoutRevisionByRequestID(model.StripeCheckoutOrderTopUp, fixture.tradeNo, request.RequestID)
	require.NoError(t, err)
	require.Equal(t, model.StripeCheckoutRevisionStatePreparing, row.State)
	require.Equal(t, "cs_record_interrupted", stripeCheckoutRevisionSessionID(row))

	second := fixture.post(t, request)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	require.Equal(t, 1, createCalls)
	require.Equal(t, 1, recordCalls)
}

func TestUpdateStripeCheckoutDiscountRecordAndCleanupFailureLeavesRecoverablePreparing(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	createCalls := 0
	recordCalls := 0
	cleanupCalls := 0
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			createCalls++
			return &stripeCheckoutSessionSnapshot{ID: "cs_record_cleanup", ClientSecret: "secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		RecordCandidate: func(input model.StripeCheckoutRevisionCandidate) (*model.StripeCheckoutRevision, error) {
			recordCalls++
			if recordCalls == 1 {
				return nil, errors.New("record unavailable")
			}
			return model.RecordStripeCheckoutCandidate(input)
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: "secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			if sessionID == "cs_record_cleanup" && cleanupCalls == 0 {
				cleanupCalls++
				return nil, errors.New("expire unavailable")
			}
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
	})
	t.Cleanup(restore)
	request := stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-record-cleanup", Action: "restore"}

	first := fixture.post(t, request)
	require.Equal(t, http.StatusInternalServerError, first.Code)
	row, err := model.GetStripeCheckoutRevisionByRequestID(model.StripeCheckoutOrderTopUp, fixture.tradeNo, request.RequestID)
	require.NoError(t, err)
	require.Equal(t, model.StripeCheckoutRevisionStatePreparing, row.State)
	require.Empty(t, stripeCheckoutRevisionSessionID(row))

	second := fixture.post(t, request)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	require.Equal(t, 2, createCalls)
	require.Equal(t, 2, recordCalls)
}

func TestUpdateStripeCheckoutDiscountUsesInvoiceStripeCustomerForTopUp(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", fixture.userID).Update("stripe_customer", "").Error)
	require.NoError(t, model.DB.Create(&model.PaymentInvoice{
		TradeNo: fixture.tradeNo, UserId: fixture.userID, OrderType: model.PaymentOrderTypeTopUp,
		PaymentProvider: model.PaymentProviderStripe, InvoiceRequested: true, StripeCustomerId: "cus_invoice_checkout",
	}).Error)
	resolverCustomer := ""
	builderCustomer := ""
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		ResolvePromotion: func(_ context.Context, query service.StripeCheckoutPromotionQuery) (service.StripeCheckoutResolvedPromotion, error) {
			resolverCustomer = query.CustomerID
			return service.StripeCheckoutResolvedPromotion{PromotionCodeID: "promo_invoice", MaskedCode: "INVOICE"}, nil
		},
		CreateCandidate: func(_ context.Context, purchase stripeCheckoutPurchase, _ int64, _ service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			builderCustomer = purchase.CustomerID
			return &stripeCheckoutSessionSnapshot{ID: "cs_invoice_customer", ClientSecret: "secret", CustomerID: purchase.CustomerID, Status: "open", PaymentStatus: "unpaid"}, nil
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, CustomerID: "cus_invoice_checkout", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
	})
	t.Cleanup(restore)

	recorder := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-invoice-customer", Action: "apply", PromotionCode: "INVOICE"})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "cus_invoice_checkout", resolverCustomer)
	require.Equal(t, "cus_invoice_checkout", builderCustomer)
}

func TestUpdateStripeCheckoutDiscountRejectsInvoiceCustomerMismatch(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	require.NoError(t, model.DB.Create(&model.PaymentInvoice{
		TradeNo: fixture.tradeNo, UserId: fixture.userID, OrderType: model.PaymentOrderTypeTopUp,
		PaymentProvider: model.PaymentProviderStripe, InvoiceRequested: true, StripeCustomerId: "cus_invoice_expected",
	}).Error)
	expiredCandidate := false
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		CreateCandidate: func(context.Context, stripeCheckoutPurchase, int64, service.StripeCheckoutDiscountSelection) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: "cs_customer_mismatch", ClientSecret: "secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, CustomerID: "cus_different", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			if sessionID == "cs_customer_mismatch" {
				expiredCandidate = true
			}
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
		},
	})
	t.Cleanup(restore)

	recorder := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: fixture.context, ExpectedRevision: 1, RequestID: "req-customer-mismatch", Action: "restore"})
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, "checkout_context_invalid", stripeCheckoutErrorCode(t, recorder))
	require.True(t, expiredCandidate)
	active, err := model.GetActiveStripeCheckoutRevision(model.StripeCheckoutOrderTopUp, fixture.tradeNo)
	require.NoError(t, err)
	require.EqualValues(t, 1, active.Revision)
}

func TestUpdateStripeCheckoutDiscountCompletedOrdersReturnCompletedConflict(t *testing.T) {
	for _, kind := range []service.StripeCheckoutPurchaseKind{service.StripeCheckoutPurchaseTopUp, service.StripeCheckoutPurchaseOneTimeSubscription} {
		t.Run(string(kind), func(t *testing.T) {
			fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
			tradeNo := fixture.tradeNo
			if kind == service.StripeCheckoutPurchaseTopUp {
				require.NoError(t, model.DB.Model(&model.TopUp{}).Where("trade_no = ?", tradeNo).Update("status", common.TopUpStatusSuccess).Error)
			} else {
				tradeNo = "trade-completed-subscription"
				require.NoError(t, model.DB.Create(&model.SubscriptionOrder{
					UserId: fixture.userID, TradeNo: tradeNo, Status: common.TopUpStatusSuccess,
					PaymentProvider: model.PaymentProviderStripe, PaymentMethod: model.PaymentMethodStripe,
					CheckoutRevision: 1, ProviderSessionId: "cs_completed_subscription",
				}).Error)
			}
			contextToken, err := service.SignStripeCheckoutContext(service.StripeCheckoutContextClaims{
				UserID: fixture.userID, PurchaseKind: kind, TradeNo: tradeNo, Revision: 1, ExpiresAt: time.Now().Add(time.Hour).Unix(),
			})
			require.NoError(t, err)
			recorder := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: contextToken, ExpectedRevision: 1, RequestID: "req-completed-" + string(kind), Action: "restore"})
			require.Equal(t, http.StatusConflict, recorder.Code)
			require.Equal(t, "checkout_already_completed", stripeCheckoutErrorCode(t, recorder))
		})
	}
}

func TestUpdateStripeCheckoutDiscountActiveReplayReturnsCompletedConflictForPaidSession(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	activeID := "cs_active_paid_replay"
	require.NoError(t, model.DB.Model(&model.TopUp{}).Where("trade_no = ?", fixture.tradeNo).Updates(map[string]any{
		"checkout_revision": 2,
		"gateway_trade_no":  activeID,
	}).Error)
	require.NoError(t, model.DB.Create(&model.StripeCheckoutRevision{
		OrderType: model.StripeCheckoutOrderTopUp, TradeNo: fixture.tradeNo, Revision: 2, UserId: fixture.userID,
		RequestId: "req-active-replay", SelectionDigest: "digest-active-replay", State: model.StripeCheckoutRevisionStateActive,
		DiscountSource: string(service.StripeCheckoutDiscountNone), ProviderSessionId: &activeID,
	}).Error)
	getCalls := 0
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			getCalls++
			return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "complete", PaymentStatus: "paid"}, nil
		},
	})
	t.Cleanup(restore)

	contextToken, err := service.SignStripeCheckoutContext(service.StripeCheckoutContextClaims{
		UserID: fixture.userID, PurchaseKind: service.StripeCheckoutPurchaseTopUp, TradeNo: fixture.tradeNo, Revision: 2, ExpiresAt: time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)
	digest := stripeCheckoutDiscountRequestDigest(model.StripeCheckoutOrderTopUp, fixture.tradeNo, stripeCheckoutDiscountRequest{CheckoutContext: contextToken, ExpectedRevision: 2, RequestID: "req-active-replay", Action: "restore"})
	require.NoError(t, model.DB.Model(&model.StripeCheckoutRevision{}).Where("order_type = ? AND trade_no = ? AND request_id = ?", model.StripeCheckoutOrderTopUp, fixture.tradeNo, "req-active-replay").Update("selection_digest", digest).Error)

	recorder := fixture.post(t, stripeCheckoutDiscountRequest{CheckoutContext: contextToken, ExpectedRevision: 2, RequestID: "req-active-replay", Action: "restore"})
	require.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())
	require.Equal(t, "checkout_already_completed", stripeCheckoutErrorCode(t, recorder))
	require.GreaterOrEqual(t, getCalls, 1)
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

func TestCreateInitialStripeCheckoutRevisionRecoversEveryInterruptedStage(t *testing.T) {
	stages := []string{"prepare", "create", "record", "activate"}
	for _, kind := range []service.StripeCheckoutPurchaseKind{service.StripeCheckoutPurchaseTopUp, service.StripeCheckoutPurchaseOneTimeSubscription} {
		for _, stage := range stages {
			t.Run(string(kind)+"/"+stage, func(t *testing.T) {
				fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
				require.NoError(t, model.DB.Where("order_type = ? AND trade_no = ?", model.StripeCheckoutOrderTopUp, fixture.tradeNo).Delete(&model.StripeCheckoutRevision{}).Error)
				purchase := stripeCheckoutPurchase{Kind: kind, TradeNo: fixture.tradeNo, UserID: fixture.userID, Currency: "USD", SubtotalMinor: 2000}
				if kind == service.StripeCheckoutPurchaseTopUp {
					purchase.OrderType = model.StripeCheckoutOrderTopUp
					require.NoError(t, model.DB.Model(&model.TopUp{}).Where("trade_no = ?", fixture.tradeNo).Updates(map[string]any{"checkout_revision": 0, "gateway_trade_no": ""}).Error)
				} else {
					purchase.OrderType = model.StripeCheckoutOrderSubscription
					purchase.TradeNo = "initial-recovery-" + stage
					require.NoError(t, model.DB.Create(&model.SubscriptionOrder{
						UserId: fixture.userID, TradeNo: purchase.TradeNo, Status: common.TopUpStatusPending,
						PaymentProvider: model.PaymentProviderStripe, PaymentMethod: model.PaymentMethodStripe,
					}).Error)
				}
				failed := false
				expireCalls := 0
				restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
					PrepareRevision: func(input model.StripeCheckoutRevisionPrepare) (*model.StripeCheckoutRevision, bool, error) {
						row, replay, err := model.PrepareStripeCheckoutRevision(input)
						if stage == "prepare" && !failed && err == nil {
							failed = true
							return nil, false, errors.New("interrupted after prepare")
						}
						return row, replay, err
					},
					RecordCandidate: func(input model.StripeCheckoutRevisionCandidate) (*model.StripeCheckoutRevision, error) {
						row, err := model.RecordStripeCheckoutCandidate(input)
						if stage == "record" && !failed && err == nil {
							failed = true
							return nil, errors.New("interrupted after record")
						}
						return row, err
					},
					ActivateRevision: func(input model.StripeCheckoutRevisionActivation) (*model.StripeCheckoutRevision, error) {
						row, err := model.ActivateStripeCheckoutRevision(input)
						if stage == "activate" && !failed && err == nil {
							failed = true
							return nil, errors.New("interrupted after activate")
						}
						return row, err
					},
					GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
						return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: "initial_secret", Status: "open", PaymentStatus: "unpaid"}, nil
					},
					ExpireSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
						expireCalls++
						return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
					},
				})
				t.Cleanup(restore)
				providerSessions := map[string]struct{}{}
				create := func(int64) (*stripeCheckoutSessionSnapshot, error) {
					providerSessions["cs_initial_recovery"] = struct{}{}
					if stage == "create" && !failed {
						failed = true
						return nil, errors.New("interrupted after provider create")
					}
					return &stripeCheckoutSessionSnapshot{ID: "cs_initial_recovery", ClientSecret: "initial_secret", Status: "open", PaymentStatus: "unpaid"}, nil
				}

				_, _, firstErr := createInitialStripeCheckoutRevision(context.Background(), purchase, service.StripeCheckoutDiscountSelection{Source: service.StripeCheckoutDiscountNone}, nil, create)
				require.Error(t, firstErr)
				created, active, secondErr := createInitialStripeCheckoutRevision(context.Background(), purchase, service.StripeCheckoutDiscountSelection{Source: service.StripeCheckoutDiscountNone}, nil, create)
				require.NoError(t, secondErr)
				require.Equal(t, "cs_initial_recovery", created.ID)
				require.EqualValues(t, 1, active.Revision)
				require.Len(t, providerSessions, 1)
				require.Zero(t, expireCalls)
				var activeCount int64
				require.NoError(t, model.DB.Model(&model.StripeCheckoutRevision{}).Where("order_type = ? AND trade_no = ? AND state = ?", purchase.OrderType, purchase.TradeNo, model.StripeCheckoutRevisionStateActive).Count(&activeCount).Error)
				require.EqualValues(t, 1, activeCount)
			})
		}
	}
}

func TestCreateInitialStripeCheckoutRevisionRejectsRecoveredTerminalCandidate(t *testing.T) {
	for _, kind := range []service.StripeCheckoutPurchaseKind{service.StripeCheckoutPurchaseTopUp, service.StripeCheckoutPurchaseOneTimeSubscription} {
		for _, terminal := range []struct {
			name      string
			status    string
			payment   string
			wantState string
			completed bool
		}{
			{name: "expired", status: "expired", payment: "unpaid", wantState: model.StripeCheckoutRevisionStateAbandoned},
			{name: "complete", status: "complete", payment: "paid", wantState: model.StripeCheckoutRevisionStateActive, completed: true},
		} {
			t.Run(string(kind)+"/"+terminal.name, func(t *testing.T) {
				fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
				selection := service.StripeCheckoutDiscountSelection{Source: service.StripeCheckoutDiscountNone}
				purchase := stripeCheckoutPurchase{Kind: kind, UserID: fixture.userID, Currency: "USD", SubtotalMinor: 2000}
				if kind == service.StripeCheckoutPurchaseTopUp {
					purchase.OrderType = model.StripeCheckoutOrderTopUp
					purchase.TradeNo = fixture.tradeNo
					require.NoError(t, model.DB.Where("order_type = ? AND trade_no = ?", purchase.OrderType, purchase.TradeNo).Delete(&model.StripeCheckoutRevision{}).Error)
					require.NoError(t, model.DB.Model(&model.TopUp{}).Where("trade_no = ?", purchase.TradeNo).Updates(map[string]any{"checkout_revision": 0, "gateway_trade_no": ""}).Error)
				} else {
					purchase.OrderType = model.StripeCheckoutOrderSubscription
					purchase.TradeNo = "initial-terminal-" + terminal.name
					require.NoError(t, model.DB.Create(&model.SubscriptionOrder{
						UserId: fixture.userID, TradeNo: purchase.TradeNo, Status: common.TopUpStatusPending,
						PaymentProvider: model.PaymentProviderStripe, PaymentMethod: model.PaymentMethodStripe,
					}).Error)
				}
				digest, err := service.StripeCheckoutIdempotencyKey("stripe-checkout-initial:"+purchase.OrderType+":"+purchase.TradeNo, 1, selection)
				require.NoError(t, err)
				prepared, replay, err := model.PrepareStripeCheckoutRevision(model.StripeCheckoutRevisionPrepare{
					OrderType: purchase.OrderType, TradeNo: purchase.TradeNo, UserID: fixture.userID, ExpectedRevision: 0,
					RequestID: "initial:" + string(kind) + ":" + purchase.TradeNo, SelectionDigest: digest,
					DiscountSource: string(selection.Source), Currency: "USD", SubtotalMinor: 2000,
				})
				require.NoError(t, err)
				require.False(t, replay)
				candidateID := "cs_initial_terminal_" + terminal.name
				_, err = model.RecordStripeCheckoutCandidate(model.StripeCheckoutRevisionCandidate{RevisionID: prepared.Id, ProviderSessionID: &candidateID})
				require.NoError(t, err)
				restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
					GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
						return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: terminal.status, PaymentStatus: terminal.payment}, nil
					},
				})
				t.Cleanup(restore)

				_, _, err = createInitialStripeCheckoutRevision(context.Background(), purchase, selection, nil, func(int64) (*stripeCheckoutSessionSnapshot, error) {
					return nil, errors.New("must not create a replacement for a durable candidate")
				})
				require.Error(t, err)
				if terminal.completed {
					require.ErrorIs(t, err, errStripeCheckoutAlreadyCompleted)
				}
				row, lookupErr := model.GetStripeCheckoutRevision(purchase.OrderType, purchase.TradeNo, 1)
				require.NoError(t, lookupErr)
				require.Equal(t, terminal.wantState, row.State)
				if kind == service.StripeCheckoutPurchaseTopUp {
					stored := model.GetTopUpByTradeNo(purchase.TradeNo)
					if terminal.completed {
						require.EqualValues(t, 1, stored.CheckoutRevision)
						require.Equal(t, candidateID, stored.GatewayTradeNo)
					} else {
						require.EqualValues(t, 0, stored.CheckoutRevision)
						require.Empty(t, stored.GatewayTradeNo)
					}
				} else {
					stored := model.GetSubscriptionOrderByTradeNo(purchase.TradeNo)
					if terminal.completed {
						require.EqualValues(t, 1, stored.CheckoutRevision)
						require.Equal(t, candidateID, stored.ProviderSessionId)
					} else {
						require.EqualValues(t, 0, stored.CheckoutRevision)
						require.Empty(t, stored.ProviderSessionId)
					}
				}
			})
		}
	}
}

func TestCreateInitialStripeCheckoutRevisionRefetchesNewCandidateBeforeRecord(t *testing.T) {
	for _, kind := range []service.StripeCheckoutPurchaseKind{service.StripeCheckoutPurchaseTopUp, service.StripeCheckoutPurchaseOneTimeSubscription} {
		t.Run(string(kind), func(t *testing.T) {
			fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
			purchase := stripeCheckoutPurchase{Kind: kind, UserID: fixture.userID, Currency: "USD", SubtotalMinor: 2000}
			if kind == service.StripeCheckoutPurchaseTopUp {
				purchase.OrderType = model.StripeCheckoutOrderTopUp
				purchase.TradeNo = fixture.tradeNo
				require.NoError(t, model.DB.Where("order_type = ? AND trade_no = ?", purchase.OrderType, purchase.TradeNo).Delete(&model.StripeCheckoutRevision{}).Error)
				require.NoError(t, model.DB.Model(&model.TopUp{}).Where("trade_no = ?", purchase.TradeNo).Updates(map[string]any{"checkout_revision": 0, "gateway_trade_no": ""}).Error)
			} else {
				purchase.OrderType = model.StripeCheckoutOrderSubscription
				purchase.TradeNo = "initial-create-refetch-one-time"
				require.NoError(t, model.DB.Create(&model.SubscriptionOrder{
					UserId: fixture.userID, TradeNo: purchase.TradeNo, Status: common.TopUpStatusPending,
					PaymentProvider: model.PaymentProviderStripe, PaymentMethod: model.PaymentMethodStripe,
				}).Error)
			}
			getCalls := 0
			recordCalls := 0
			restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
				GetSession: func(_ context.Context, gotKind service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
					require.Equal(t, kind, gotKind)
					require.Equal(t, "cs_initial_created_terminal", sessionID)
					getCalls++
					return &stripeCheckoutSessionSnapshot{ID: sessionID, Status: "expired", PaymentStatus: "unpaid"}, nil
				},
				RecordCandidate: func(model.StripeCheckoutRevisionCandidate) (*model.StripeCheckoutRevision, error) {
					recordCalls++
					return nil, errors.New("must not record expired initial candidate")
				},
			})
			t.Cleanup(restore)

			_, _, err := createInitialStripeCheckoutRevision(context.Background(), purchase, service.StripeCheckoutDiscountSelection{Source: service.StripeCheckoutDiscountNone}, nil, func(int64) (*stripeCheckoutSessionSnapshot, error) {
				return &stripeCheckoutSessionSnapshot{ID: "cs_initial_created_terminal", Status: "open", PaymentStatus: "unpaid"}, nil
			})
			require.Error(t, err)
			require.Equal(t, 1, getCalls)
			require.Zero(t, recordCalls)
			row, lookupErr := model.GetStripeCheckoutRevision(purchase.OrderType, purchase.TradeNo, 1)
			require.NoError(t, lookupErr)
			require.Equal(t, model.StripeCheckoutRevisionStateAbandoned, row.State)
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
	couponCalls := 0
	sessionCalls := 0
	restore := service.ReplaceStripeSubscriptionCheckoutCreatorsForTest(
		func(context.Context, *stripe.CouponParams) (*stripe.Coupon, error) {
			couponCalls++
			return &stripe.Coupon{ID: "coupon_resolved_initial"}, nil
		},
		func(_ context.Context, params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
			sessionCalls++
			require.Equal(t, "1", params.Metadata["checkout_revision"])
			require.Len(t, params.Discounts, 1)
			require.Equal(t, "coupon_resolved_initial", *params.Discounts[0].Coupon)
			return &stripe.CheckoutSession{ID: "cs_recurring_initial", ClientSecret: "cs_recurring_initial_secret", Status: stripe.CheckoutSessionStatusOpen, PaymentStatus: stripe.CheckoutSessionPaymentStatusUnpaid}, nil
		},
	)
	t.Cleanup(restore)
	restoreAccessors := service.ReplaceStripeCheckoutSessionAccessorsForTest(
		func(context.Context, string) (*stripe.CheckoutSession, error) {
			return &stripe.CheckoutSession{ID: "cs_recurring_initial", ClientSecret: "cs_recurring_initial_secret", Status: stripe.CheckoutSessionStatusOpen, PaymentStatus: stripe.CheckoutSessionPaymentStatusUnpaid}, nil
		},
		nil,
	)
	t.Cleanup(restoreAccessors)

	input := service.StripeSubscriptionCheckoutInput{
		TradeNo: tradeNo, UserID: fixture.userID, PlanID: 77, PriceID: "price_recurring", Currency: "USD", SubtotalMinor: 2000,
		IdempotencyKey: "subscription-stripe:" + tradeNo, Presentation: service.StripeCheckoutPresentation{RequestedUIMode: "elements", Elements: true},
		DiscountKind: service.SubscriptionDiscountKindInvitation, DiscountAmountMinor: 500, DiscountCurrency: "USD", CheckoutRevision: 0,
	}
	created, err := service.CreateStripeSubscriptionCheckout(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "coupon_resolved_initial", created.DiscountSelection.CouponID)
	active, err := model.GetActiveStripeCheckoutRevision(model.StripeCheckoutOrderSubscription, tradeNo)
	require.NoError(t, err)
	require.EqualValues(t, 1, active.Revision)
	require.Equal(t, "coupon_resolved_initial", active.CouponId)
	require.Equal(t, "cs_recurring_initial", stripeCheckoutRevisionSessionID(active))
	replayed, err := service.CreateStripeSubscriptionCheckout(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, created.ID, replayed.ID)
	require.Equal(t, 1, couponCalls)
	require.Equal(t, 1, sessionCalls)
}

func TestStripeRecurringInitialRevisionRecoversEveryDurableStage(t *testing.T) {
	for _, stage := range []string{"prepare", "create", "record", "activate"} {
		t.Run(stage, func(t *testing.T) {
			fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
			tradeNo := "trade-recurring-recovery-" + stage
			order := &model.SubscriptionOrder{
				UserId: fixture.userID, TradeNo: tradeNo, PlanId: 77, Status: common.TopUpStatusPending,
				PaymentProvider: model.PaymentProviderStripe, PaymentMethod: model.PaymentMethodStripe,
				PaymentCurrency: "USD", PaymentAmountMinor: 2000,
			}
			require.NoError(t, model.DB.Create(order).Error)
			selection := service.StripeCheckoutDiscountSelection{Source: service.StripeCheckoutDiscountNone}
			digest, err := service.StripeCheckoutIdempotencyKey("stripe-checkout-initial:subscription:"+tradeNo, 1, selection)
			require.NoError(t, err)
			providerID := "cs_recurring_recovery"
			state := model.StripeCheckoutRevisionStatePreparing
			var attached *string
			if stage == "record" || stage == "activate" {
				attached = &providerID
			}
			if stage == "activate" {
				state = model.StripeCheckoutRevisionStateActive
				order.CheckoutRevision = 1
				order.ProviderSessionId = providerID
				require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Where("trade_no = ?", tradeNo).Updates(map[string]any{"checkout_revision": 1, "provider_session_id": providerID}).Error)
			}
			require.NoError(t, model.DB.Create(&model.StripeCheckoutRevision{
				OrderType: model.StripeCheckoutOrderSubscription, TradeNo: tradeNo, Revision: 1, UserId: fixture.userID,
				RequestId: "initial:recurring_subscription:" + tradeNo, SelectionDigest: digest, State: state,
				DiscountSource: string(service.StripeCheckoutDiscountNone), Currency: "USD", SubtotalMinor: 2000,
				ProviderSessionId: attached,
			}).Error)
			originalSecret := setting.StripeApiSecret
			setting.StripeApiSecret = "sk_test_recurring_recovery"
			t.Cleanup(func() { setting.StripeApiSecret = originalSecret })
			providerSessions := map[string]struct{}{}
			createCalls := 0
			restoreCreators := service.ReplaceStripeSubscriptionCheckoutCreatorsForTest(nil, func(context.Context, *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
				createCalls++
				providerSessions[providerID] = struct{}{}
				return &stripe.CheckoutSession{ID: providerID, ClientSecret: "recurring_secret", Status: stripe.CheckoutSessionStatusOpen, PaymentStatus: stripe.CheckoutSessionPaymentStatusUnpaid}, nil
			})
			t.Cleanup(restoreCreators)
			restoreAccessors := service.ReplaceStripeCheckoutSessionAccessorsForTest(
				func(context.Context, string) (*stripe.CheckoutSession, error) {
					providerSessions[providerID] = struct{}{}
					return &stripe.CheckoutSession{ID: providerID, ClientSecret: "recurring_secret", Status: stripe.CheckoutSessionStatusOpen, PaymentStatus: stripe.CheckoutSessionPaymentStatusUnpaid}, nil
				},
				func(context.Context, string) (*stripe.CheckoutSession, error) {
					return nil, errors.New("must not expire recoverable initial session")
				},
			)
			t.Cleanup(restoreAccessors)

			created, err := service.CreateStripeSubscriptionCheckout(context.Background(), service.StripeSubscriptionCheckoutInput{
				TradeNo: tradeNo, UserID: fixture.userID, PlanID: 77, PriceID: "price_recurring", Currency: "USD", SubtotalMinor: 2000,
				IdempotencyKey: "subscription-stripe:" + tradeNo, Presentation: service.StripeCheckoutPresentation{RequestedUIMode: "elements", Elements: true},
				CheckoutRevision: 0, DiscountSelection: selection,
			})
			require.NoError(t, err)
			require.Equal(t, providerID, created.ID)
			require.Len(t, providerSessions, 1)
			if stage == "record" || stage == "activate" {
				require.Zero(t, createCalls)
			} else {
				require.Equal(t, 1, createCalls)
			}
			active, err := model.GetActiveStripeCheckoutRevision(model.StripeCheckoutOrderSubscription, tradeNo)
			require.NoError(t, err)
			require.EqualValues(t, 1, active.Revision)
		})
	}
}

func TestStripeRecurringInitialRevisionRejectsRecoveredTerminalCandidate(t *testing.T) {
	for _, terminal := range []struct {
		name      string
		status    stripe.CheckoutSessionStatus
		payment   stripe.CheckoutSessionPaymentStatus
		wantState string
	}{
		{name: "expired", status: stripe.CheckoutSessionStatusExpired, payment: stripe.CheckoutSessionPaymentStatusUnpaid, wantState: model.StripeCheckoutRevisionStateAbandoned},
		{name: "complete", status: stripe.CheckoutSessionStatusComplete, payment: stripe.CheckoutSessionPaymentStatusPaid, wantState: model.StripeCheckoutRevisionStateActive},
	} {
		t.Run(terminal.name, func(t *testing.T) {
			fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
			tradeNo := "trade-recurring-terminal-" + terminal.name
			require.NoError(t, model.DB.Create(&model.SubscriptionOrder{
				UserId: fixture.userID, TradeNo: tradeNo, PlanId: 77, Status: common.TopUpStatusPending,
				PaymentProvider: model.PaymentProviderStripe, PaymentMethod: model.PaymentMethodStripe,
				PaymentCurrency: "USD", PaymentAmountMinor: 2000,
			}).Error)
			selection := service.StripeCheckoutDiscountSelection{Source: service.StripeCheckoutDiscountNone}
			digest, err := service.StripeCheckoutIdempotencyKey("stripe-checkout-initial:subscription:"+tradeNo, 1, selection)
			require.NoError(t, err)
			candidateID := "cs_recurring_terminal_" + terminal.name
			require.NoError(t, model.DB.Create(&model.StripeCheckoutRevision{
				OrderType: model.StripeCheckoutOrderSubscription, TradeNo: tradeNo, Revision: 1, UserId: fixture.userID,
				RequestId: "initial:recurring_subscription:" + tradeNo, SelectionDigest: digest, State: model.StripeCheckoutRevisionStatePreparing,
				DiscountSource: string(selection.Source), Currency: "USD", SubtotalMinor: 2000, ProviderSessionId: &candidateID,
			}).Error)
			originalSecret := setting.StripeApiSecret
			setting.StripeApiSecret = "sk_test_recurring_terminal"
			t.Cleanup(func() { setting.StripeApiSecret = originalSecret })
			createCalls := 0
			restoreCreators := service.ReplaceStripeSubscriptionCheckoutCreatorsForTest(nil, func(context.Context, *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
				createCalls++
				return nil, errors.New("must not create")
			})
			t.Cleanup(restoreCreators)
			restoreAccessors := service.ReplaceStripeCheckoutSessionAccessorsForTest(
				func(_ context.Context, sessionID string) (*stripe.CheckoutSession, error) {
					return &stripe.CheckoutSession{ID: sessionID, ClientSecret: "recurring_terminal_secret", Status: terminal.status, PaymentStatus: terminal.payment}, nil
				},
				nil,
			)
			t.Cleanup(restoreAccessors)

			_, err = service.CreateStripeSubscriptionCheckout(context.Background(), service.StripeSubscriptionCheckoutInput{
				TradeNo: tradeNo, UserID: fixture.userID, PlanID: 77, PriceID: "price_recurring", Currency: "USD", SubtotalMinor: 2000,
				IdempotencyKey: "subscription-stripe:" + tradeNo, Presentation: service.StripeCheckoutPresentation{RequestedUIMode: "elements", Elements: true},
				CheckoutRevision: 0, DiscountSelection: selection,
			})
			require.Error(t, err)
			require.Zero(t, createCalls)
			row, lookupErr := model.GetStripeCheckoutRevision(model.StripeCheckoutOrderSubscription, tradeNo, 1)
			require.NoError(t, lookupErr)
			require.Equal(t, terminal.wantState, row.State)
			stored := model.GetSubscriptionOrderByTradeNo(tradeNo)
			if terminal.name == "complete" {
				require.EqualValues(t, 1, stored.CheckoutRevision)
				require.Equal(t, candidateID, stored.ProviderSessionId)
			} else {
				require.EqualValues(t, 0, stored.CheckoutRevision)
				require.Empty(t, stored.ProviderSessionId)
			}
		})
	}
}

func TestStripeRecurringInitialRevisionRefetchesNewCandidateBeforeRecord(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	tradeNo := "trade-recurring-create-refetch"
	require.NoError(t, model.DB.Create(&model.SubscriptionOrder{
		UserId: fixture.userID, TradeNo: tradeNo, PlanId: 77, Status: common.TopUpStatusPending,
		PaymentProvider: model.PaymentProviderStripe, PaymentMethod: model.PaymentMethodStripe,
		PaymentCurrency: "USD", PaymentAmountMinor: 2000,
	}).Error)
	originalSecret := setting.StripeApiSecret
	setting.StripeApiSecret = "sk_test_recurring_create_refetch"
	t.Cleanup(func() { setting.StripeApiSecret = originalSecret })
	createCalls := 0
	getCalls := 0
	restoreCreators := service.ReplaceStripeSubscriptionCheckoutCreatorsForTest(nil, func(context.Context, *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		createCalls++
		return &stripe.CheckoutSession{ID: "cs_recurring_created_terminal", ClientSecret: "secret", Status: stripe.CheckoutSessionStatusOpen, PaymentStatus: stripe.CheckoutSessionPaymentStatusUnpaid}, nil
	})
	t.Cleanup(restoreCreators)
	restoreAccessors := service.ReplaceStripeCheckoutSessionAccessorsForTest(func(_ context.Context, sessionID string) (*stripe.CheckoutSession, error) {
		getCalls++
		return &stripe.CheckoutSession{ID: sessionID, ClientSecret: "secret", Status: stripe.CheckoutSessionStatusExpired, PaymentStatus: stripe.CheckoutSessionPaymentStatusUnpaid}, nil
	}, nil)
	t.Cleanup(restoreAccessors)

	_, err := service.CreateStripeSubscriptionCheckout(context.Background(), service.StripeSubscriptionCheckoutInput{
		TradeNo: tradeNo, UserID: fixture.userID, PlanID: 77, PriceID: "price_recurring", Currency: "USD", SubtotalMinor: 2000,
		IdempotencyKey: "subscription-stripe:" + tradeNo, Presentation: service.StripeCheckoutPresentation{RequestedUIMode: "elements", Elements: true},
		CheckoutRevision: 0, DiscountSelection: service.StripeCheckoutDiscountSelection{Source: service.StripeCheckoutDiscountNone},
	})
	require.Error(t, err)
	require.Equal(t, 1, createCalls)
	require.Equal(t, 1, getCalls)
	row, lookupErr := model.GetStripeCheckoutRevision(model.StripeCheckoutOrderSubscription, tradeNo, 1)
	require.NoError(t, lookupErr)
	require.Equal(t, model.StripeCheckoutRevisionStateAbandoned, row.State)
	stored := model.GetSubscriptionOrderByTradeNo(tradeNo)
	require.EqualValues(t, 0, stored.CheckoutRevision)
	require.Empty(t, stored.ProviderSessionId)
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

func TestCreateOneTimeStripeCheckoutSessionActiveReplayReturnsPersistedSession(t *testing.T) {
	fixture := newStripeCheckoutDiscountFixture(t, service.StripeCheckoutDiscountNone)
	tradeNo := "trade-one-time-active-replay"
	order := &model.SubscriptionOrder{
		UserId: fixture.userID, TradeNo: tradeNo, Status: common.TopUpStatusPending, PaymentProvider: model.PaymentProviderStripe,
		PaymentMethod: model.PaymentMethodStripe, PaymentCurrency: "USD", PaymentAmountMinor: 2000,
	}
	require.NoError(t, model.DB.Create(order).Error)
	user, err := model.GetUserById(fixture.userID, false)
	require.NoError(t, err)
	creatorCalls := 0
	originalCreator := stripeOneTimeCheckoutSessionForRevisionCreator
	stripeOneTimeCheckoutSessionForRevisionCreator = func(_ context.Context, _ *model.SubscriptionOrder, _ *model.User, revision int64, _ service.StripeCheckoutDiscountSelection, _ ...service.StripeCheckoutPresentation) (*oneTimeStripeCheckoutSession, error) {
		creatorCalls++
		require.EqualValues(t, 1, revision)
		return &oneTimeStripeCheckoutSession{ID: "cs_one_time_replay", ClientSecret: "one_time_replay_secret", Status: "open", PaymentStatus: "unpaid"}, nil
	}
	t.Cleanup(func() { stripeOneTimeCheckoutSessionForRevisionCreator = originalCreator })
	restore := replaceStripeCheckoutDiscountRuntimeForTest(stripeCheckoutDiscountRuntime{
		GetSession: func(_ context.Context, _ service.StripeCheckoutPurchaseKind, sessionID string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: sessionID, ClientSecret: "one_time_replay_secret", Status: "open", PaymentStatus: "unpaid"}, nil
		},
	})
	t.Cleanup(restore)
	presentation := service.StripeCheckoutPresentation{RequestedUIMode: "elements", Elements: true}

	first, err := createOneTimeStripeCheckoutSession(context.Background(), order, user, presentation)
	require.NoError(t, err)
	require.Equal(t, "cs_one_time_replay", first.ID)
	staleOrder := *order
	staleOrder.CheckoutRevision = 0
	staleOrder.ProviderSessionId = ""
	staleOrder.ProviderSessionURL = ""
	replayed, err := createOneTimeStripeCheckoutSession(context.Background(), &staleOrder, user, presentation)
	require.NoError(t, err)
	require.NotNil(t, replayed)
	require.Equal(t, first.ID, replayed.ID)
	require.Equal(t, first.ClientSecret, replayed.ClientSecret)
	require.Equal(t, 1, creatorCalls)
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
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.SubscriptionOrder{}, &model.StripeCheckoutRevision{}, &model.PaymentInvoice{}))
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

func stripeCheckoutMessage(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var envelope struct {
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &envelope))
	return envelope.Message
}
