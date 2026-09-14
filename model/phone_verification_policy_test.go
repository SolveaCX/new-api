package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// TestPhoneVerificationRolloutStartInstant pins the exact instant so an
// accidental edit of the date, hour, or zone fails loudly instead of silently
// moving the moment new PLG accounts start being gated.
func TestPhoneVerificationRolloutStartInstant(t *testing.T) {
	start := PhoneVerificationRolloutStart()
	require.Equal(t, time.Date(2026, time.September, 14, 9, 30, 0, 0, time.UTC).Unix(), start.Unix())
	require.Equal(t, "2026-09-14T02:30:00-07:00", start.Format(time.RFC3339))
}

func TestPhoneVerificationRequiredForAccountOnlyGatesNewUnverifiedPlgAccounts(t *testing.T) {
	original := common.SMSVerificationEnabled
	t.Cleanup(func() { common.SMSVerificationEnabled = original })
	common.SMSVerificationEnabled = true
	start := PhoneVerificationRolloutStart().Unix()

	tests := []struct {
		name            string
		group           string
		phoneVerifiedAt int64
		createdAt       int64
		required        bool
	}{
		{name: "new plg account, unverified", group: "plg", createdAt: start, required: true},
		{name: "new plg account created later, unverified", group: "plg", createdAt: start + 86400, required: true},
		{name: "new plg account, verified", group: "plg", phoneVerifiedAt: start + 60, createdAt: start, required: false},
		{name: "old plg account created one second before rollout", group: "plg", createdAt: start - 1, required: false},
		{name: "old plg account created months ago", group: "plg", createdAt: start - 90*86400, required: false},
		{name: "legacy plg account with no created_at (predates column)", group: "plg", createdAt: 0, required: false},
		{name: "new enterprise account", group: "enterprise", createdAt: start + 60, required: false},
		{name: "new admin account", group: "admin", createdAt: start + 60, required: false},
		{name: "new account with empty group", group: "", createdAt: start + 60, required: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.required, PhoneVerificationRequiredForAccount(tt.group, tt.phoneVerifiedAt, tt.createdAt))
		})
	}
}

func TestPhoneVerificationRequiredForAccountIsOffWhenFeatureDisabled(t *testing.T) {
	original := common.SMSVerificationEnabled
	t.Cleanup(func() { common.SMSVerificationEnabled = original })
	common.SMSVerificationEnabled = false
	require.False(t, PhoneVerificationRequiredForAccount("plg", 0, PhoneVerificationRolloutStart().Unix()+60))
}

func TestUserBasePhoneVerificationRequiredHandlesNilAndCarriesCreatedAt(t *testing.T) {
	original := common.SMSVerificationEnabled
	t.Cleanup(func() { common.SMSVerificationEnabled = original })
	common.SMSVerificationEnabled = true
	start := PhoneVerificationRolloutStart().Unix()

	var nilUser *UserBase
	require.False(t, nilUser.PhoneVerificationRequired())
	require.True(t, (&UserBase{Group: "plg", CreatedAt: start}).PhoneVerificationRequired())
	require.False(t, (&UserBase{Group: "plg", CreatedAt: start - 1}).PhoneVerificationRequired())

	// ToBaseUser must carry CreatedAt, otherwise every cached user would look
	// legacy and the gate would silently never fire.
	user := &User{Id: 7, Group: "plg", CreatedAt: start + 5}
	require.Equal(t, start+5, user.ToBaseUser().CreatedAt)
	require.True(t, user.ToBaseUser().PhoneVerificationRequired())
}
