package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func withServerAddress(t *testing.T, addr string) {
	t.Helper()
	t.Setenv("APP_CONSOLE_ORIGIN", "")
	orig := system_setting.ServerAddress
	system_setting.ServerAddress = addr
	t.Cleanup(func() { system_setting.ServerAddress = orig })
}

func withAllowedHosts(t *testing.T, hosts []string) {
	t.Helper()
	settings := system_setting.GetTopupHintSettings()
	orig := settings.AllowedHosts
	settings.AllowedHosts = hosts
	t.Cleanup(func() { settings.AllowedHosts = orig })
}

// The regression this file guards against: the top-up hint URL is appended to
// quota-insufficient errors, but error messages pass through
// common.MaskSensitiveInfo before reaching the client. Without a preserved
// fragment the link renders as https://***.***.xx/***/*** — a payment hint
// nobody can follow. A domain-name console origin must survive sanitization
// out of the box, with no allowlist configuration required.
func TestTopUpHintSurvivesSanitizationForDomainOrigin(t *testing.T) {
	withServerAddress(t, "https://console.example-gateway.ai")
	withAllowedHosts(t, nil)

	topUp := topUpURL()
	require.NotEmpty(t, topUp)
	require.Contains(t, topUp, "console.example-gateway.ai")

	apiErr := types.NewErrorWithStatusCode(
		fmt.Errorf("quota is not enough. Add credits at %s to keep going.", topUp),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
		walletTopUpHintPreserveOption(),
	)
	msg := apiErr.ToOpenAIError().Message
	require.Contains(t, msg, topUp, "top-up URL must survive sanitization verbatim")
	require.NotContains(t, msg, "***", "no masked residue expected around the hint")
}

func TestTopUpHintStaysMaskedForIPLiteralOrigin(t *testing.T) {
	withServerAddress(t, "http://10.0.12.34:3000")
	withAllowedHosts(t, nil)

	topUp := topUpURL()
	require.NotEmpty(t, topUp)

	apiErr := types.NewErrorWithStatusCode(
		fmt.Errorf("quota is not enough. Add credits at %s to keep going.", topUp),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
		walletTopUpHintPreserveOption(),
	)
	msg := apiErr.ToOpenAIError().Message
	require.NotContains(t, msg, "10.0.12.34", "IP-literal origins must stay masked")
}

func TestTopUpHintAllowlistStillForceAllows(t *testing.T) {
	withServerAddress(t, "http://10.0.12.34:3000")
	withAllowedHosts(t, []string{"10.0.12.34"})

	topUp := topUpURL()
	require.NotEmpty(t, topUp)
	require.True(t, system_setting.TopupHintHostAllowed(topUp))

	apiErr := types.NewErrorWithStatusCode(
		fmt.Errorf("quota is not enough. Add credits at %s to keep going.", topUp),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
		walletTopUpHintPreserveOption(),
	)
	msg := apiErr.ToOpenAIError().Message
	require.Contains(t, msg, "10.0.12.34", "explicit allowlist entries win over the IP guard")
}

func TestTopUpHintHostIsDomainName(t *testing.T) {
	require.True(t, topUpHintHostIsDomainName("https://console.example.ai/console/topup"))
	require.True(t, topUpHintHostIsDomainName("console.example.ai"))
	require.False(t, topUpHintHostIsDomainName("https://192.168.1.80/topup"))
	require.False(t, topUpHintHostIsDomainName("https://[::1]/topup"))
	require.False(t, topUpHintHostIsDomainName(""))
}

func TestTopUpURLRefusesLoopback(t *testing.T) {
	withServerAddress(t, "http://localhost:3000")
	require.Empty(t, topUpURL())
	withServerAddress(t, "http://127.0.0.1:3000")
	require.Empty(t, topUpURL())
}

// Guards the hint text itself: the builder helper must carry the URL when a
// public origin is configured.
func TestQuotaMessagesCarryHint(t *testing.T) {
	if err := i18n.Init(); err != nil {
		t.Fatalf("i18n init failed: %v", err)
	}
	withServerAddress(t, "https://console.example-gateway.ai")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	msg := appendWalletTopUpHint(c, "quota is not enough.")
	require.True(t, strings.Contains(msg, "console.example-gateway.ai"), msg)
}
