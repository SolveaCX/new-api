package service

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

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
