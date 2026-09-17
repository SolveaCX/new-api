package model

import (
	"time"

	"github.com/QuantumNous/new-api/common"
)

// PhoneVerificationRolloutStart is the instant the phone-verification
// requirement starts applying to NEW accounts. Accounts created before it
// are never asked to bind a phone; accounts created at or after it must
// bind before using the console or the API.
//
// 2026-09-14 02:30 America/Los_Angeles = 09:30 UTC = 17:30 Asia/Shanghai.
// The location is loaded at runtime so the date remains correct across DST
// rules; the fixed-zone fallback only matters if tzdata is missing.
func PhoneVerificationRolloutStart() time.Time {
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		location = time.FixedZone("PDT", -7*60*60)
	}
	return time.Date(2026, time.September, 14, 2, 30, 0, 0, location)
}

// PhoneVerificationRequiredForAccount is the single rule shared by the API
// gate (middleware), the console gate (GetSelf) and, through GetSelf, the
// frontend dialog. It answers "must THIS account bind a phone?":
//
//   - feature flag SMS_VERIFICATION_ENABLED must be on;
//   - only PLG accounts are subject to it;
//   - an account with a verified phone is done;
//   - only accounts created at or after PhoneVerificationRolloutStart are
//     subject to it. createdAt == 0 means the row predates the created_at
//     column (added 2026-04-25, never backfilled), so it is an old account
//     and is exempt.
func PhoneVerificationRequiredForAccount(group string, phoneVerifiedAt int64, createdAt int64) bool {
	if !common.SMSVerificationEnabled {
		return false
	}
	if group != plgUserGroup {
		return false
	}
	if phoneVerifiedAt != 0 {
		return false
	}
	if createdAt <= 0 {
		return false
	}
	return createdAt >= PhoneVerificationRolloutStart().Unix()
}

// PhoneVerificationRequired is the UserBase convenience wrapper used by the
// token-auth middleware, which only has the cached user at hand.
func (user *UserBase) PhoneVerificationRequired() bool {
	if user == nil {
		return false
	}
	return PhoneVerificationRequiredForAccount(user.Group, user.PhoneVerifiedAt, user.CreatedAt)
}

// PhoneVerificationReminderEligibleForAccount answers "should THIS account be
// reminded, through the API, to bind a phone?". It is the complement of the
// gate for accounts that can bind but are never forced to:
//
//   - feature flag SMS_VERIFICATION_ENABLED must be on, otherwise there is
//     nothing to bind;
//   - only PLG accounts are subject to it;
//   - an account with a verified phone is done;
//   - accounts the gate already blocks (created at or after the rollout
//     start) are never reminded: they get the 403 instead.
//
// Whether the reminder is actually served is decided by the operator toggle
// and the 24h claim, both outside this package.
func PhoneVerificationReminderEligibleForAccount(group string, phoneVerifiedAt int64, createdAt int64) bool {
	if !common.SMSVerificationEnabled {
		return false
	}
	if group != plgUserGroup {
		return false
	}
	if phoneVerifiedAt != 0 {
		return false
	}
	return !PhoneVerificationRequiredForAccount(group, phoneVerifiedAt, createdAt)
}

// PhoneVerificationReminderEligible is the UserBase convenience wrapper used by
// the token-auth middleware.
func (user *UserBase) PhoneVerificationReminderEligible() bool {
	if user == nil {
		return false
	}
	return PhoneVerificationReminderEligibleForAccount(user.Group, user.PhoneVerifiedAt, user.CreatedAt)
}
