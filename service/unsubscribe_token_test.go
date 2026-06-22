package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func TestUnsubscribeTokenRoundTrip(t *testing.T) {
	original := common.SessionSecret
	t.Cleanup(func() { common.SessionSecret = original })
	common.SessionSecret = "test-secret-key"

	tok := GenerateUnsubscribeToken(12345)
	require.NotEmpty(t, tok)

	uid, ok := VerifyUnsubscribeToken(12345, tok)
	require.True(t, ok)
	require.Equal(t, 12345, uid)
}

func TestUnsubscribeTokenRejectsTamper(t *testing.T) {
	original := common.SessionSecret
	t.Cleanup(func() { common.SessionSecret = original })
	common.SessionSecret = "test-secret-key"

	tok := GenerateUnsubscribeToken(12345)

	// 换 userId 校验应失败
	_, ok := VerifyUnsubscribeToken(99999, tok)
	require.False(t, ok)

	// 篡改 token 应失败
	_, ok = VerifyUnsubscribeToken(12345, tok+"x")
	require.False(t, ok)

	// 空 token 应失败
	_, ok = VerifyUnsubscribeToken(12345, "")
	require.False(t, ok)
}

func TestBuildUnsubscribeLink(t *testing.T) {
	original := common.SessionSecret
	t.Cleanup(func() { common.SessionSecret = original })
	common.SessionSecret = "test-secret-key"

	link := BuildUnsubscribeLink("https://console.example.com", 42)
	require.Contains(t, link, "https://console.example.com/api/email/unsubscribe?uid=42&token=")
}
