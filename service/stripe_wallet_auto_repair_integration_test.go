package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/require"
)

func TestStripeWalletAutoRepairHTTPToRealWalletAndDingTalk(t *testing.T) {
	h := newStripeWalletHarness(t)
	oldLogDB, oldHook := model.LOG_DB, model.PaymentSuccessHook
	oldRedis, oldBatch, oldQuota := common.RedisEnabled, common.BatchUpdateEnabled, common.QuotaPerUnit
	oldClient := httpClient
	oldMonitor, oldFetch := *operation_setting.GetMonitorSetting(), *system_setting.GetFetchSetting()
	oldBonus := operation_setting.GetPaymentSetting().AmountBonusLimit
	model.LOG_DB, model.PaymentSuccessHook = h.db, nil
	common.RedisEnabled, common.BatchUpdateEnabled, common.QuotaPerUnit = false, false, 1000
	operation_setting.GetPaymentSetting().AmountBonusLimit = map[int]int{}
	system_setting.GetFetchSetting().EnableSSRFProtection = false
	t.Cleanup(func() {
		model.LOG_DB, model.PaymentSuccessHook = oldLogDB, oldHook
		common.RedisEnabled, common.BatchUpdateEnabled, common.QuotaPerUnit = oldRedis, oldBatch, oldQuota
		httpClient = oldClient
		*operation_setting.GetMonitorSetting(), *system_setting.GetFetchSetting() = oldMonitor, oldFetch
		operation_setting.GetPaymentSetting().AmountBonusLimit = oldBonus
	})
	// Replace this harness's deliberately minimal users table only in its fresh,
	// isolated temporary SQLite database, then exercise production model methods.
	require.NoError(t, h.db.Migrator().DropTable(&model.User{}))
	require.NoError(t, h.db.AutoMigrate(&model.User{}, &model.SubscriptionOrder{}, &model.PaymentInvoice{},
		&model.RecallLifecycleEvent{}, &model.QuotaLifecycleState{}, &model.TopUpBonusClaim{},
		&model.PaymentAnalyticsOutbox{}, &model.PaymentAnalyticsEventReceipt{}, &model.Log{}, &model.StripeCheckoutRevision{}))
	sqlDB, err := h.db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	h.now, err = model.GetDBTimestampWithContext(context.Background())
	require.NoError(t, err)
	require.NoError(t, h.db.Create(&model.User{Id: 7, Username: "wallet_http_test", Status: common.UserStatusEnabled, Quota: 100}).Error)
	trade, id := stripeWalletTrade(80), "cs_http_repair"
	topUp := h.createTopUp(trade, id, common.TopUpStatusFailed, 10)
	require.NoError(t, h.db.Model(&topUp).Updates(map[string]any{"bonus_amount": 2, "bonus_tier": 10, "money": 12}).Error)
	require.NoError(t, h.db.Create(&model.PaymentInvoice{TradeNo: trade, UserId: 7,
		OrderType: model.PaymentOrderTypeTopUp, PaymentProvider: model.PaymentProviderStripe,
		InvoiceStatus: model.PaymentInvoiceStatusFailed}).Error)
	session := stripeWalletRepairSession(id, trade)
	// The production hosted/default checkout builder emits these metadata
	// fields even when it does not create a StripeCheckoutRevision row.
	session.Metadata["checkout_revision"] = "0"
	session.Metadata["discount_selection"] = "none"
	session.Object, session.PaymentIntent.Object, session.PaymentIntent.LatestCharge.Object = "checkout.session", "payment_intent", "charge"
	event := stripeWalletEvent(t, "evt_http_repair", h.now-1, session)
	notices := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.URL.Path == "/robot/send" {
			require.Equal(t, http.MethodPost, req.Method)
			require.NotEmpty(t, req.URL.Query().Get("sign"))
			var payload struct {
				Text struct {
					Content string `json:"content"`
				} `json:"text"`
			}
			require.NoError(t, common.DecodeJson(req.Body, &payload))
			notices <- payload.Text.Content
			_, _ = w.Write([]byte(`{"errcode":0}`))
			return
		}
		require.Equal(t, http.MethodGet, req.Method)
		var body any
		switch req.URL.Path {
		case "/v1/account":
			body = map[string]any{"id": "acct_test", "object": "account"}
		case "/v1/events":
			body = map[string]any{"object": "list", "data": []any{event}, "has_more": false}
		case "/v1/checkout/sessions/" + id:
			body = session
		case "/v1/refunds":
			require.Equal(t, "ch_wallet", req.URL.Query().Get("charge"))
			body = map[string]any{"object": "list", "data": []any{}, "has_more": false}
		default:
			http.NotFound(w, req)
			return
		}
		data, err := common.Marshal(body)
		require.NoError(t, err)
		_, _ = w.Write(data)
	}))
	defer server.Close()
	httpClient = server.Client()
	monitor := operation_setting.GetMonitorSetting()
	monitor.DingTalkAlertEnabled, monitor.DingTalkAlertWebhookURL, monitor.DingTalkAlertSecret = true, server.URL+"/robot/send", "integration-test-signing-secret"
	r := h.reconciler()
	r.gateway = newStripeWalletGatewayWithBackends("rk_test_mock_only", stripeWalletTestBackends(server))
	r.notify = sendStripeWalletReconciliationNotification

	processed, err := r.run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	var user model.User
	require.NoError(t, h.db.First(&user, 7).Error)
	require.Equal(t, 12100, user.Quota)
	require.NoError(t, h.db.First(&topUp, topUp.Id).Error)
	require.Equal(t, common.TopUpStatusSuccess, topUp.Status)
	require.Equal(t, 10.0, topUp.Money)
	check := h.loadCheck(id)
	require.Positive(t, check.AutoRepairedAt)
	require.EqualValues(t, 12000, check.RepairCredit)
	require.Positive(t, check.ClosedAt)
	var invoice model.PaymentInvoice
	require.NoError(t, h.db.Where("trade_no = ?", trade).First(&invoice).Error)
	require.Equal(t, model.PaymentInvoiceStatusPaid, invoice.InvoiceStatus)
	select {
	case message := <-notices:
		require.Contains(t, message, "漏单已自动补账")
		require.Contains(t, message, trade)
		require.Contains(t, message, "12000")
	default:
		t.Fatal("expected signed HTTP DingTalk repair warning after real wallet commit")
	}
	credited, err := model.RechargeWithPaymentSnapshot(trade, "cus_wallet", "", model.PaymentSnapshot{Money: 10, Currency: "USD"})
	require.NoError(t, err)
	require.False(t, credited, "later webhook must not credit the repaired order again")
	require.NoError(t, h.db.First(&user, 7).Error)
	require.Equal(t, 12100, user.Quota)
	var successEvents int64
	require.NoError(t, h.db.Model(&model.RecallLifecycleEvent{}).Where("event_type = ?", model.RecallLifecycleTriggerPaymentSucceeded).Count(&successEvents).Error)
	require.EqualValues(t, 1, successEvents)
	t.Logf("Mock preview %s: paid event -> failed order repaired, wallet +12000, invoice paid, signed warning; webhook replay +0", server.URL)
}
