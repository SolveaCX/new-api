package controller

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetRegistrationCaptcha(c *gin.Context) {
	captchaType := strings.TrimSpace(c.Query("type"))
	challenge, err := service.GenerateRegistrationCaptcha(captchaType)
	if err != nil {
		if errors.Is(err, service.ErrRegistrationCaptchaInvalid) {
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return
		}
		common.SysError("failed to generate registration captcha: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgRegistrationCaptchaUnavailable)
		return
	}
	common.ApiSuccess(c, challenge)
}

func VerifyRegistrationCaptcha(c *gin.Context) {
	var request service.RegistrationCaptchaAnswer
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	token, err := service.VerifyRegistrationCaptcha(request, c.ClientIP())
	if err != nil {
		if errors.Is(err, service.ErrRegistrationCaptchaInvalid) {
			common.ApiErrorI18n(c, i18n.MsgRegistrationCaptchaInvalid)
			return
		}
		common.SysError("failed to verify registration captcha: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgRegistrationCaptchaUnavailable)
		return
	}
	common.ApiSuccess(c, gin.H{"token": token})
}
