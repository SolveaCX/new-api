package controller

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v86"
	"gorm.io/gorm"
)

func setupStripeCardRiskControllerTest(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "stripe-card-risk-controller.db")), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.StripeBonusClaim{}, &model.StripePaymentCardObservation{}, &model.StripePaymentCardBackfillLease{}))
}

func TestStripeCardObservationFromChargeRecordsNonCardPayment(t *testing.T) {
	observation := stripeCardObservationFromCharge(stripeCardObservationSource{UserId: 1}, &stripe.Charge{
		ID:     "ch_bank",
		Paid:   true,
		Status: stripe.ChargeStatusSucceeded,
		PaymentMethodDetails: &stripe.ChargePaymentMethodDetails{
			Type: stripe.ChargePaymentMethodDetailsTypeACHDebit,
		},
	})
	require.NotNil(t, observation)
	require.Equal(t, "ach_debit", observation.PaymentMethodType)
	require.Empty(t, observation.Fingerprint)
}

func TestPersistStripeCheckoutCardObservationIsIdempotent(t *testing.T) {
	setupStripeCardRiskControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 401, Username: "stripe-risk", Email: "stripe-risk@example.com"}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{UserId: 401, TradeNo: "trade-risk"}).Error)

	originalFetcher := stripeCardPaymentIntentChargeFetcher
	stripeCardPaymentIntentChargeFetcher = func(context.Context, string) (*stripe.Charge, error) {
		return &stripe.Charge{
			ID:            "ch_risk",
			Paid:          true,
			Created:       1000,
			PaymentMethod: "pm_risk",
			PaymentMethodDetails: &stripe.ChargePaymentMethodDetails{Card: &stripe.ChargePaymentMethodDetailsCard{
				Fingerprint: "fp_risk",
				Brand:       stripe.PaymentMethodCardBrandVisa,
				Last4:       "4242",
				ExpMonth:    12,
				ExpYear:     2030,
			}},
		}, nil
	}
	t.Cleanup(func() { stripeCardPaymentIntentChargeFetcher = originalFetcher })

	source := stripeCardObservationSource{TradeNo: "trade-risk", CheckoutSessionId: "cs_risk", PaymentIntentId: "pi_risk"}
	created, err := persistStripeCheckoutCardObservation(context.Background(), source)
	require.NoError(t, err)
	require.True(t, created)
	created, err = persistStripeCheckoutCardObservation(context.Background(), source)
	require.NoError(t, err)
	require.False(t, created)

	var observation model.StripePaymentCardObservation
	require.NoError(t, model.DB.First(&observation, "charge_id = ?", "ch_risk").Error)
	require.Equal(t, 401, observation.UserId)
	require.Equal(t, "fp_risk", observation.Fingerprint)
}

func TestStripeCardFinalizeFailureReturnsRetryWithoutRollingBackSuccessfulTopUp(t *testing.T) {
	setupStripeCardRiskControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 501, Username: "retry-user"}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId:          501,
		TradeNo:         "trade-retry",
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusSuccess,
	}).Error)
	originalFetcher := stripeCardPaymentIntentChargeFetcher
	stripeCardPaymentIntentChargeFetcher = func(context.Context, string) (*stripe.Charge, error) {
		return nil, errors.New("stripe unavailable")
	}
	t.Cleanup(func() { stripeCardPaymentIntentChargeFetcher = originalFetcher })

	event := stripe.Event{
		Type: stripe.EventTypeCheckoutSessionCompleted,
		Data: &stripe.EventData{Raw: []byte(`{"id":"cs_retry","client_reference_id":"trade-retry","payment_intent":"pi_unavailable"}`)},
	}
	err := finalizeStripeCheckoutCardObservation(context.Background(), event)
	require.Error(t, err)

	// The caller returns this error to Stripe so the webhook is retried, but the already
	// committed payment remains successful and no purchased quota is rolled back.
	var topUp model.TopUp
	require.NoError(t, model.DB.First(&topUp, "trade_no = ?", "trade-retry").Error)
	require.Equal(t, common.TopUpStatusSuccess, topUp.Status)
	var count int64
	require.NoError(t, model.DB.Model(&model.StripePaymentCardObservation{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestBackfillStripeCardObservationsFetchesSessionsOnceWithoutChargeNPlusOne(t *testing.T) {
	setupStripeCardRiskControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 601, Username: "backfill-user", Email: "backfill@example.com", AffCode: "bf601", StripeCustomer: "cus_backfill"}).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 602, Username: "backfill-oldest", Email: "backfill-oldest@example.com", AffCode: "bf602", StripeCustomer: "cus_backfill_oldest"}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{UserId: 601, TradeNo: "trade-backfill"}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{UserId: 602, TradeNo: "trade-backfill-oldest"}).Error)

	var sessionRequests atomic.Int32
	var chargeRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/checkout/sessions":
			sessionRequests.Add(1)
			_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"cs_backfill","object":"checkout.session","client_reference_id":"trade-backfill","payment_intent":"pi_backfill","customer":"cus_backfill"},{"id":"cs_backfill_oldest","object":"checkout.session","client_reference_id":"trade-backfill-oldest","payment_intent":"pi_backfill_oldest","customer":"cus_backfill_oldest"}],"has_more":false,"url":"/v1/checkout/sessions"}`)
		case "/v1/charges":
			chargeRequests.Add(1)
			// Stripe lists newest first. Both charges intentionally use the same exact
			// fingerprint; the backfill must sort ascending before claiming the owner.
			_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"ch_backfill_newer","object":"charge","paid":true,"status":"succeeded","created":1787890001,"customer":"cus_backfill","payment_intent":"pi_backfill","payment_method":"pm_backfill_newer","payment_method_details":{"type":"card","card":{"fingerprint":"fp_backfill_shared","brand":"visa","last4":"4242","exp_month":12,"exp_year":2030}}},{"id":"ch_backfill_oldest","object":"charge","paid":true,"status":"succeeded","created":1787890000,"customer":"cus_backfill_oldest","payment_intent":"pi_backfill_oldest","payment_method":"pm_backfill_oldest","payment_method_details":{"type":"card","card":{"fingerprint":"fp_backfill_shared","brand":"visa","last4":"4242","exp_month":12,"exp_year":2030}}}],"has_more":false,"url":"/v1/charges"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	originalSecret := setting.StripeApiSecret
	originalBackend := stripe.GetBackend(stripe.APIBackend)
	setting.StripeApiSecret = "sk_test_backfill"
	stripe.SetBackend(stripe.APIBackend, stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{
		URL:               stripe.String(server.URL),
		HTTPClient:        server.Client(),
		MaxNetworkRetries: stripe.Int64(0),
		LeveledLogger:     &stripe.LeveledLogger{Level: stripe.LevelNull},
	}))
	t.Cleanup(func() {
		setting.StripeApiSecret = originalSecret
		stripe.SetBackend(stripe.APIBackend, originalBackend)
	})

	result, err := backfillStripeCardObservations(context.Background(), stripeCardBackfillRequest{Days: 30, MaxObjects: 10})
	require.NoError(t, err)
	require.Equal(t, int32(1), sessionRequests.Load())
	require.Equal(t, int32(1), chargeRequests.Load())
	require.Equal(t, 2, result.Inserted)
	require.Zero(t, result.Unmatched)

	var observations []model.StripePaymentCardObservation
	require.NoError(t, model.DB.Order("charge_id").Find(&observations).Error)
	require.Len(t, observations, 2)
	var claim model.StripeBonusClaim
	require.NoError(t, model.DB.First(&claim, "card_fingerprint = ?", "fp_backfill_shared").Error)
	require.Equal(t, 602, claim.UserId, "the chronologically earliest charge must own the exact fingerprint")
	for _, observation := range observations {
		if observation.UserId == 601 {
			require.Equal(t, model.StripeCardRewardDecisionDuplicateExactCard, observation.RewardDecision)
		}
	}
}

func TestBackfillStripeCardObservationsRejectsBusyLease(t *testing.T) {
	setupStripeCardRiskControllerTest(t)
	originalSecret := setting.StripeApiSecret
	setting.StripeApiSecret = "sk_test_backfill_busy"
	t.Cleanup(func() { setting.StripeApiSecret = originalSecret })

	now := common.GetTimestamp()
	acquired, err := model.AcquireStripeCardBackfillLease("stripe_card_observation_backfill", "other-node", now, now+300)
	require.NoError(t, err)
	require.True(t, acquired)

	_, err = backfillStripeCardObservations(context.Background(), stripeCardBackfillRequest{Days: 1, MaxObjects: 1})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "already running"))
}
