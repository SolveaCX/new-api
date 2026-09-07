package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// SendPhoneVerification sends an SMS code without creating or authenticating
// an account. A phone number that already exists is rejected before any SMS is
// sent, and the same check is repeated during registration to close races.
func SendPhoneVerification(c *gin.Context) {
	if !common.SMSVerificationEnabled {
		common.ApiErrorI18n(c, i18n.MsgFeatureDisabled)
		return
	}

	phone, err := common.NormalizePhoneNumber(c.Query("phone_number"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgUserPhoneInvalid)
		return
	}
	if model.IsPhoneAlreadyTaken(phone) {
		common.ApiErrorI18n(c, i18n.MsgUserPhoneAlreadyRegistered)
		return
	}

	code := common.GenerateVerificationCode(6)
	if err := service.SendSMSVerification(c.Request.Context(), phone, code); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("failed to send SMS verification to %s: %s", phone, err.Error()))
		common.ApiErrorI18n(c, i18n.MsgUserSMSVerificationUnavailable)
		return
	}
	common.RegisterVerificationCodeWithKey(phone, code, common.SMSVerificationPurpose)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

type bindPhoneRequest struct {
	PhoneNumber           string `json:"phone_number"`
	PhoneVerificationCode string `json:"phone_verification_code"`
}

// BindPhone verifies and binds a phone number for an authenticated user.
func BindPhone(c *gin.Context) {
	if !common.SMSVerificationEnabled {
		common.ApiErrorI18n(c, i18n.MsgFeatureDisabled)
		return
	}

	var req bindPhoneRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	phone, err := common.NormalizePhoneNumber(req.PhoneNumber)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgUserPhoneInvalid)
		return
	}
	if model.IsPhoneAlreadyTaken(phone) {
		common.ApiErrorI18n(c, i18n.MsgUserPhoneAlreadyRegistered)
		return
	}
	if strings.TrimSpace(req.PhoneVerificationCode) == "" ||
		!common.VerifyCodeWithKey(phone, req.PhoneVerificationCode, common.SMSVerificationPurpose) {
		common.ApiErrorI18n(c, i18n.MsgUserVerificationCodeError)
		return
	}
	common.DeleteKey(phone, common.SMSVerificationPurpose)

	user, err := model.GetUserById(c.GetInt("id"), false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	user.PhoneNumber = phone
	user.PhoneVerifiedAt = common.GetTimestamp()
	if err := user.Update(false); err != nil {
		common.ApiError(c, err)
		return
	}
	// Update(false) may refresh Redis from the pre-update user snapshot. Clear
	// the cache so the API phone gate observes the newly verified phone on all
	// nodes immediately.
	if err := model.InvalidateUserCache(user.Id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"phone_number":      user.PhoneNumber,
		"phone_verified_at": user.PhoneVerifiedAt,
	})
}

func normalizeRegistrationPhone(user *model.User) error {
	if !common.SMSVerificationEnabled {
		return nil
	}
	phone, err := common.NormalizePhoneNumber(user.PhoneNumber)
	if err != nil {
		return err
	}
	user.PhoneNumber = phone
	return nil
}

func verifyRegistrationPhone(user *model.User) bool {
	if !common.SMSVerificationEnabled {
		return true
	}
	if strings.TrimSpace(user.PhoneVerificationCode) == "" {
		return false
	}
	verified := common.VerifyCodeWithKey(user.PhoneNumber, user.PhoneVerificationCode, common.SMSVerificationPurpose)
	if verified {
		common.DeleteKey(user.PhoneNumber, common.SMSVerificationPurpose)
	}
	return verified
}
