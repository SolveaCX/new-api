package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func usePhoneReminderMiniredis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	oldEnabled := common.RedisEnabled
	oldRDB := common.RDB
	mini := miniredis.RunT(t)
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RedisEnabled = oldEnabled
		common.RDB = oldRDB
	})
	return mini
}

func resetPhoneReminderMemoryClaims(t *testing.T) {
	t.Helper()
	oldNow := phoneVerificationReminderNow
	phoneVerificationReminderMemory.Lock()
	oldClaims := phoneVerificationReminderMemory.claims
	phoneVerificationReminderMemory.claims = map[int]time.Time{}
	phoneVerificationReminderMemory.Unlock()
	t.Cleanup(func() {
		phoneVerificationReminderNow = oldNow
		phoneVerificationReminderMemory.Lock()
		phoneVerificationReminderMemory.claims = oldClaims
		phoneVerificationReminderMemory.Unlock()
	})
}

func TestClaimPhoneVerificationReminderWinsOncePerWindowOnRedis(t *testing.T) {
	mini := usePhoneReminderMiniredis(t)

	first, err := ClaimPhoneVerificationReminder(42)
	require.NoError(t, err)
	require.True(t, first)

	second, err := ClaimPhoneVerificationReminder(42)
	require.NoError(t, err)
	require.False(t, second)

	other, err := ClaimPhoneVerificationReminder(43)
	require.NoError(t, err)
	require.True(t, other)

	require.Equal(t, PhoneVerificationReminderWindow, mini.TTL("phone_reminder:api:42"))

	mini.FastForward(PhoneVerificationReminderWindow + time.Second)
	again, err := ClaimPhoneVerificationReminder(42)
	require.NoError(t, err)
	require.True(t, again)
}

func TestClaimPhoneVerificationReminderReportsRedisErrors(t *testing.T) {
	mini := usePhoneReminderMiniredis(t)
	mini.Close()

	claimed, err := ClaimPhoneVerificationReminder(42)
	require.Error(t, err)
	require.False(t, claimed)
}

func TestClaimPhoneVerificationReminderRejectsInvalidUser(t *testing.T) {
	usePhoneReminderMiniredis(t)
	claimed, err := ClaimPhoneVerificationReminder(0)
	require.Error(t, err)
	require.False(t, claimed)
}

func TestClaimPhoneVerificationReminderMemoryFallbackWinsOncePerWindow(t *testing.T) {
	oldEnabled := common.RedisEnabled
	oldRDB := common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = oldEnabled
		common.RDB = oldRDB
	})
	resetPhoneReminderMemoryClaims(t)
	base := time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC)
	phoneVerificationReminderNow = func() time.Time { return base }

	first, err := ClaimPhoneVerificationReminder(7)
	require.NoError(t, err)
	require.True(t, first)

	second, err := ClaimPhoneVerificationReminder(7)
	require.NoError(t, err)
	require.False(t, second)

	phoneVerificationReminderNow = func() time.Time { return base.Add(PhoneVerificationReminderWindow - time.Minute) }
	stillHeld, err := ClaimPhoneVerificationReminder(7)
	require.NoError(t, err)
	require.False(t, stillHeld)

	phoneVerificationReminderNow = func() time.Time { return base.Add(PhoneVerificationReminderWindow + time.Second) }
	expired, err := ClaimPhoneVerificationReminder(7)
	require.NoError(t, err)
	require.True(t, expired)
}
