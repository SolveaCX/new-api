package controller

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCloseStripeCheckoutTopUpExpiresProviderAndFailsInvoice(t *testing.T) {
	setupStripeCheckoutCloseTestDB(t)
	const userID = 6101
	const tradeNo = "close-topup-6101"
	require.NoError(t, model.DB.Create(&model.User{Id: userID, Username: "close-topup"}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId: userID, TradeNo: tradeNo, GatewayTradeNo: "cs_close_topup",
		PaymentProvider: model.PaymentProviderStripe, PaymentMethod: model.PaymentMethodStripe,
		Status: common.TopUpStatusPending, Amount: 20, Money: 20,
	}).Error)
	require.NoError(t, model.DB.Create(&model.PaymentInvoice{
		TradeNo: tradeNo, UserId: userID, PaymentProvider: model.PaymentProviderStripe,
		InvoiceRequested: true, InvoiceStatus: model.PaymentInvoiceStatusRequested,
	}).Error)

	expired := 0
	failed := 0
	invoiceFailed := 0
	restore := replaceStripeCheckoutCloseRuntimeForTest(stripeCheckoutCloseRuntime{
		GetSession: func(context.Context, service.StripeCheckoutPurchaseKind, string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: "cs_close_topup", Status: "open", PaymentStatus: "unpaid"}, nil
		},
		ExpireSession: func(context.Context, service.StripeCheckoutPurchaseKind, string) (*stripeCheckoutSessionSnapshot, error) {
			expired++
			return &stripeCheckoutSessionSnapshot{ID: "cs_close_topup", Status: "expired", PaymentStatus: "unpaid"}, nil
		},
		FailTopUp:   func(string) error { failed++; return nil },
		FailInvoice: func(string) error { invoiceFailed++; return nil },
	})
	t.Cleanup(restore)

	recorder := postCloseStripeCheckout(t, userID, tradeNo)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.Equal(t, 1, expired)
	require.Equal(t, 1, failed)
	require.Equal(t, 1, invoiceFailed)
}

func TestCloseStripeCheckoutDoesNotFailPaidSession(t *testing.T) {
	setupStripeCheckoutCloseTestDB(t)
	const userID = 6102
	const tradeNo = "close-paid-6102"
	require.NoError(t, model.DB.Create(&model.User{Id: userID, Username: "close-paid"}).Error)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId: userID, TradeNo: tradeNo, GatewayTradeNo: "cs_close_paid",
		PaymentProvider: model.PaymentProviderStripe, PaymentMethod: model.PaymentMethodStripe,
		Status: common.TopUpStatusPending, Amount: 20, Money: 20,
	}).Error)

	failed := 0
	restore := replaceStripeCheckoutCloseRuntimeForTest(stripeCheckoutCloseRuntime{
		GetSession: func(context.Context, service.StripeCheckoutPurchaseKind, string) (*stripeCheckoutSessionSnapshot, error) {
			return &stripeCheckoutSessionSnapshot{ID: "cs_close_paid", Status: "complete", PaymentStatus: "paid"}, nil
		},
		FailTopUp: func(string) error { failed++; return nil },
	})
	t.Cleanup(restore)

	recorder := postCloseStripeCheckout(t, userID, tradeNo)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"status":"paid"`)
	require.Zero(t, failed)
}

func TestCloseStripeCheckoutRejectsForeignOrder(t *testing.T) {
	setupStripeCheckoutCloseTestDB(t)
	require.NoError(t, model.DB.Create(&model.TopUp{
		UserId: 6103, TradeNo: "close-foreign", GatewayTradeNo: "cs_foreign",
		PaymentProvider: model.PaymentProviderStripe, Status: common.TopUpStatusPending,
	}).Error)
	recorder := postCloseStripeCheckout(t, 6104, "close-foreign")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":false`)
}

func setupStripeCheckoutCloseTestDB(t *testing.T) {
	t.Helper()
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:stripe-checkout-close-"+time.Now().Format("150405.000000000")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.PaymentInvoice{}, &model.SubscriptionOrder{}))
	t.Cleanup(func() { model.DB = originalDB })
}

func postCloseStripeCheckout(t *testing.T, userID int, tradeNo string) *httptest.ResponseRecorder {
	t.Helper()
	body := bytes.NewBufferString(`{"trade_no":"` + tradeNo + `"}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", userID)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/stripe/checkout/close", body)
	c.Request.Header.Set("Content-Type", "application/json")
	CloseStripeCheckout(c)
	return recorder
}
