package controller

import (
	"errors"
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

type sendPhoneVerificationRequest struct {
	PhoneNumber string `json:"phone_number"`
}

// SendPhoneVerification sends an SMS code without creating or authenticating
// an account. Existing numbers receive the same success response to prevent
// account enumeration.
func SendPhoneVerification(c *gin.Context) {
	if !common.SMSVerificationEnabled {
		common.ApiErrorI18n(c, i18n.MsgFeatureDisabled)
		return
	}

	var req sendPhoneVerificationRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	phone, err := common.NormalizePhoneNumber(req.PhoneNumber)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgUserPhoneInvalid)
		return
	}
	taken, err := model.IsPhoneAlreadyTaken(phone)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("failed to check SMS phone availability: %s", err.Error()))
		common.ApiErrorI18n(c, i18n.MsgUserSMSVerificationUnavailable)
		return
	}
	if taken {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
		return
	}

	code := common.GenerateNumericVerificationCode(6)
	if code == "" {
		common.ApiErrorI18n(c, i18n.MsgUserSMSVerificationUnavailable)
		return
	}
	if err := common.RegisterSMSVerificationCode(phone, code); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("failed to store SMS verification: %s", err.Error()))
		common.ApiErrorI18n(c, i18n.MsgUserSMSVerificationUnavailable)
		return
	}
	if err := service.SendSMSVerification(c.Request.Context(), phone, code); err != nil {
		_ = common.DeleteSMSVerificationCode(phone)
		logger.LogError(c.Request.Context(), fmt.Sprintf("failed to send SMS verification: %s", err.Error()))
		common.ApiErrorI18n(c, i18n.MsgUserSMSVerificationUnavailable)
		return
	}
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
	taken, err := model.IsPhoneAlreadyTaken(phone)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgUserSMSVerificationUnavailable)
		return
	}
	if taken {
		common.ApiErrorI18n(c, i18n.MsgUserPhoneAlreadyRegistered)
		return
	}
	verified, err := common.VerifySMSVerificationCode(phone, strings.TrimSpace(req.PhoneVerificationCode))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgUserSMSVerificationUnavailable)
		return
	}
	if strings.TrimSpace(req.PhoneVerificationCode) == "" || !verified {
		common.ApiErrorI18n(c, i18n.MsgUserVerificationCodeError)
		return
	}
	phoneVerifiedAt := common.GetTimestamp()
	if err := model.BindUserPhone(c.GetInt("id"), phone, phoneVerifiedAt); err != nil {
		if errors.Is(err, model.ErrPhoneAlreadyTaken) {
			common.ApiErrorI18n(c, i18n.MsgUserPhoneAlreadyRegistered)
			return
		}
		common.ApiError(c, err)
		return
	}
	if err := common.DeleteSMSVerificationCode(phone); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("failed to delete SMS verification: %s", err.Error()))
	}
	// Clear the user cache so the API phone gate observes the newly verified
	// phone on all nodes immediately.
	if err := model.InvalidateUserCache(c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"phone_number":      phone,
		"phone_verified_at": phoneVerifiedAt,
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

func verifyRegistrationPhone(user *model.User) (bool, error) {
	if !common.SMSVerificationEnabled {
		return true, nil
	}
	if strings.TrimSpace(user.PhoneVerificationCode) == "" {
		return false, nil
	}
	return common.VerifySMSVerificationCode(user.PhoneNumber, strings.TrimSpace(user.PhoneVerificationCode))
}
