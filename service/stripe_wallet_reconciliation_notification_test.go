package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/require"
)

func TestBuildStripeWalletReconciliationNotificationSanitizesAndUsesMinorAmount(t *testing.T) {
	t.Parallel()
	content := buildStripeWalletReconciliationNotification(stripeWalletNotification{
		Kind:              "reminder",
		Scope:             "stripe:acct_123:live",
		SessionID:         "cs_123",
		EventID:           "evt_123",
		TradeNo:           "trade_123",
		LocalStatus:       "failed",
		Detail:            "Authorization: Bearer secret-token api_key=sk_live_secret rk_test_restricted_secret",
		UserID:            42,
		Amount:            1234,
		Currency:          "jpy",
		PaidAt:            1700000000,
		FirstAnomalyAt:    1700000300,
		NotificationCount: 2,
	})
	require.Contains(t, content, "Stripe 钱包对账异常提醒")
	require.Contains(t, content, "支付金额（最小货币单位）：1234 JPY")
	require.Contains(t, content, "通知次数：2")
	require.Contains(t, content, "仅告警，请人工核查")
	require.NotContains(t, content, "secret-token")
	require.NotContains(t, content, "sk_live_secret")
	require.NotContains(t, content, "rk_test_restricted_secret")
}

func TestSendStripeWalletReconciliationNotificationDisabledIsError(t *testing.T) {
	setting := operation_setting.GetMonitorSetting()
	original := *setting
	t.Cleanup(func() { *setting = original })
	setting.DingTalkAlertEnabled = false
	setting.DingTalkAlertWebhookURL = ""

	err := sendStripeWalletReconciliationNotification(context.Background(), stripeWalletNotification{Kind: "task_error"})
	require.ErrorContains(t, err, "disabled")
}

func TestSendStripeWalletReconciliationNotificationEmptyWebhookIsError(t *testing.T) {
	setting := operation_setting.GetMonitorSetting()
	original := *setting
	t.Cleanup(func() { *setting = original })
	setting.DingTalkAlertEnabled = true
	setting.DingTalkAlertWebhookURL = ""

	err := sendStripeWalletReconciliationNotification(context.Background(), stripeWalletNotification{Kind: "task_error"})
	require.ErrorContains(t, err, "webhook url is empty")
}

func TestSendStripeWalletReconciliationDingTalkTextHonorsContext(t *testing.T) {
	originalClient := httpClient
	originalFetch := *system_setting.GetFetchSetting()
	t.Cleanup(func() {
		httpClient = originalClient
		*system_setting.GetFetchSetting() = originalFetch
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	httpClient = server.Client()
	system_setting.GetFetchSetting().EnableSSRFProtection = false

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := sendStripeWalletReconciliationDingTalkText(ctx, server.URL, "test-signing-secret", "test")
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "context canceled")
}

func TestSendStripeWalletReconciliationNotificationRequiresSigningSecret(t *testing.T) {
	setting := operation_setting.GetMonitorSetting()
	original := *setting
	t.Cleanup(func() { *setting = original })
	setting.DingTalkAlertEnabled = true
	setting.DingTalkAlertWebhookURL = "http://127.0.0.1:1"
	setting.DingTalkAlertSecret = ""
	err := sendStripeWalletReconciliationNotification(context.Background(), stripeWalletNotification{Kind: "initial"})
	require.ErrorContains(t, err, "signing secret is empty")
	err = sendStripeWalletReconciliationDingTalkText(context.Background(), setting.DingTalkAlertWebhookURL, "", "test")
	require.ErrorContains(t, err, "signing secret is empty")
}

func TestSendStripeWalletReconciliationNotificationPostsExpectedContent(t *testing.T) {
	originalClient := httpClient
	originalMonitor := *operation_setting.GetMonitorSetting()
	originalFetch := *system_setting.GetFetchSetting()
	t.Cleanup(func() {
		httpClient = originalClient
		*operation_setting.GetMonitorSetting() = originalMonitor
		*system_setting.GetFetchSetting() = originalFetch
	})

	contents := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		timestamp := r.URL.Query().Get("timestamp")
		require.NotEmpty(t, timestamp)
		mac := hmac.New(sha256.New, []byte("test-signing-secret"))
		_, _ = mac.Write([]byte(timestamp + "\n" + "test-signing-secret"))
		require.Equal(t, base64.StdEncoding.EncodeToString(mac.Sum(nil)), r.URL.Query().Get("sign"))
		var payload struct {
			Text struct {
				Content string `json:"content"`
			} `json:"text"`
		}
		require.NoError(t, common.DecodeJson(r.Body, &payload))
		contents <- payload.Text.Content
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()
	httpClient = server.Client()
	fetch := system_setting.GetFetchSetting()
	fetch.EnableSSRFProtection = false
	setting := operation_setting.GetMonitorSetting()
	setting.DingTalkAlertEnabled = true
	setting.DingTalkAlertWebhookURL = server.URL
	setting.DingTalkAlertSecret = "test-signing-secret"

	err := sendStripeWalletReconciliationNotification(context.Background(), stripeWalletNotification{
		Kind:      "initial",
		SessionID: "cs_send",
		Amount:    999,
		Currency:  "usd",
	})
	require.NoError(t, err)
	select {
	case content := <-contents:
		require.Contains(t, content, "cs_send")
		require.Contains(t, content, "999 USD")
	case <-time.After(time.Second):
		t.Fatal("notification request was not received")
	}
}
