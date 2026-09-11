package service

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestSendSMSVerificationUsesTeleSignMessagingAPI(t *testing.T) {
	var gotAuth string
	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		require.NoError(t, r.ParseForm())
		gotForm = r.PostForm
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	originalURL := common.TeleSignAPIURL
	originalCustomerID := common.TeleSignCustomerID
	originalAPIKey := common.TeleSignAPIKey
	originalOriginator := common.TeleSignOriginator
	t.Cleanup(func() {
		common.TeleSignAPIURL = originalURL
		common.TeleSignCustomerID = originalCustomerID
		common.TeleSignAPIKey = originalAPIKey
		common.TeleSignOriginator = originalOriginator
	})
	common.TeleSignAPIURL = server.URL
	common.TeleSignCustomerID = "customer"
	common.TeleSignAPIKey = "secret"
	common.TeleSignOriginator = "Flatkey"

	require.NoError(t, SendSMSVerification(context.Background(), "+8613800138000", "123456"))
	require.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte("customer:secret")), gotAuth)
	require.Equal(t, "+8613800138000", gotForm.Get("phone_number"))
	require.Equal(t, "Your Flatkey verification code is 123456.", gotForm.Get("message"))
	require.Equal(t, "OTP", gotForm.Get("message_type"))
	require.Equal(t, "Flatkey", gotForm.Get("sender_id"))
	require.False(t, strings.Contains(gotForm.Get("message"), "<"))
}

func TestSendSMSVerificationUsesITNIOForInternationalPhone(t *testing.T) {
	var gotBody map[string]any
	var gotSign, gotTimestamp, gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSign = r.Header.Get("Sign")
		gotTimestamp = r.Header.Get("Timestamp")
		gotAPIKey = r.Header.Get("Api-Key")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, common.Unmarshal(body, &gotBody))
		_, _ = w.Write([]byte(`{"status":"0"}`))
	}))
	defer server.Close()

	originalMock := common.TeleSignMockEnabled
	originalTeleSignURL := common.TeleSignAPIURL
	originalCustomerID := common.TeleSignCustomerID
	originalTeleSignKey := common.TeleSignAPIKey
	originalOriginator := common.TeleSignOriginator
	originalITNIOURL := common.ITNIOAPIURL
	originalITNIOKey := common.ITNIOAPIKey
	originalITNIOSecret := common.ITNIOAPISecret
	originalITNIOAppID := common.ITNIOAppID
	originalITNIOSenderID := common.ITNIOSenderID
	t.Cleanup(func() {
		common.TeleSignMockEnabled = originalMock
		common.TeleSignAPIURL = originalTeleSignURL
		common.TeleSignCustomerID = originalCustomerID
		common.TeleSignAPIKey = originalTeleSignKey
		common.TeleSignOriginator = originalOriginator
		common.ITNIOAPIURL = originalITNIOURL
		common.ITNIOAPIKey = originalITNIOKey
		common.ITNIOAPISecret = originalITNIOSecret
		common.ITNIOAppID = originalITNIOAppID
		common.ITNIOSenderID = originalITNIOSenderID
	})
	common.TeleSignMockEnabled = false
	common.ITNIOAPIURL = server.URL
	common.ITNIOAPIKey = "api-key"
	common.ITNIOAPISecret = "api-secret"
	common.ITNIOAppID = "app-id"
	common.ITNIOSenderID = "Flatkey"

	require.NoError(t, SendSMSVerification(context.Background(), "+14155552671", "123456"))
	require.Equal(t, "api-key", gotAPIKey)
	require.Equal(t, "14155552671", gotBody["numbers"])
	require.Equal(t, "app-id", gotBody["appId"])
	require.Equal(t, "Your Flatkey verification code is 123456.", gotBody["content"])
	require.Equal(t, "Flatkey", gotBody["senderId"])
	require.Equal(t, float64(0), gotBody["trackClicks"])
	timestamp, err := strconv.ParseInt(gotTimestamp, 10, 64)
	require.NoError(t, err)
	digest := md5.Sum([]byte("api-keyapi-secret" + gotTimestamp))
	require.Equal(t, hex.EncodeToString(digest[:]), gotSign)
	require.InDelta(t, time.Now().Unix(), timestamp, float64(5))
}
