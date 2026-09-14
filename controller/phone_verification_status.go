package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// maskPhoneNumber keeps the country code and the last two digits so a caller
// can recognise which number is on file without the response carrying a
// reusable identifier. "+14155550123" becomes "+1********23".
func maskPhoneNumber(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}
	digits := strings.TrimPrefix(phone, "+")
	// Too short to mask meaningfully: hide everything after the plus.
	if len(digits) <= 4 {
		return "+" + strings.Repeat("*", len(digits))
	}
	return "+" + digits[:1] + strings.Repeat("*", len(digits)-3) + digits[len(digits)-2:]
}

// GetSelfPhoneVerificationStatus reports the caller's own phone-verification
// state. It is intentionally lighter than GetSelf, which returns the full
// profile, and it is mounted under UserAuth (not TokenAuth) so an account that
// still has to bind a phone can always read why it is being asked.
func GetSelfPhoneVerificationStatus(c *gin.Context) {
	user, err := model.GetUserById(c.GetInt("id"), false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		// Whether this account has a verified phone on file.
		"phone_bound": user.PhoneVerifiedAt > 0,
		// Masked, never the full number.
		"phone_number": maskPhoneNumber(user.PhoneNumber),
		// Unix seconds, 0 when never verified.
		"phone_verified_at": user.PhoneVerifiedAt,
		// Whether THIS account must bind before using the console and the API.
		// False for accounts created before the rollout start, for non-PLG
		// accounts, and whenever the feature flag is off.
		"verification_required": model.PhoneVerificationRequiredForAccount(user.Group, user.PhoneVerifiedAt, user.CreatedAt),
		// Deployment-wide feature flag, so a client can tell "not required
		// because the feature is off" from "not required because this account
		// is exempt".
		"sms_verification_enabled": common.SMSVerificationEnabled,
		// The instant from which newly created PLG accounts are gated.
		"rollout_start_at": model.PhoneVerificationRolloutStart().Unix(),
	})
}
