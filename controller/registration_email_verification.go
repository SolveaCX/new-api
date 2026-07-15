package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	registrationEmailGrantSessionKey = "registration_email_grant"
	maxRegistrationEmailTokenLength  = 512
	maxRegistrationEmailLength       = 320
)

type registrationEmailTokenRequest struct {
	Token string `json:"token"`
}

type registrationEmailStatusRequest struct {
	Email string `json:"email"`
}

type registrationEmailVerificationData struct {
	Verified  bool `json:"verified"`
	ExpiresIn int  `json:"expires_in,omitempty"`
}

func registrationEmailGrantFromSession(c *gin.Context) string {
	grant, _ := sessions.Default(c).Get(registrationEmailGrantSessionKey).(string)
	return strings.TrimSpace(grant)
}

func registrationEmailGrantMatches(c *gin.Context, email string) bool {
	return common.VerifyRegistrationEmailGrant(registrationEmailGrantFromSession(c), strings.TrimSpace(email))
}

func clearRegistrationEmailGrant(c *gin.Context) {
	session := sessions.Default(c)
	grant := registrationEmailGrantFromSession(c)
	session.Delete(registrationEmailGrantSessionKey)
	common.DeleteRegistrationEmailGrant(grant)
}

func ExchangeRegistrationEmailVerification(c *gin.Context) {
	var request registrationEmailTokenRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	request.Token = strings.TrimSpace(request.Token)
	if request.Token == "" || len(request.Token) > maxRegistrationEmailTokenLength {
		common.ApiErrorI18n(c, i18n.MsgEmailVerifyLinkInvalid)
		return
	}

	email, ok, err := common.ResolveRegistrationEmailLink(request.Token)
	if err != nil {
		common.SysError("failed to resolve registration email link: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgEmailVerifyUnavailable)
		return
	}
	if !ok {
		common.ApiErrorI18n(c, i18n.MsgEmailVerifyLinkInvalid)
		return
	}

	grant, err := common.RegisterRegistrationEmailGrant(email)
	if err != nil {
		common.SysError("failed to create registration email grant: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgEmailVerifyUnavailable)
		return
	}

	session := sessions.Default(c)
	previousGrant := registrationEmailGrantFromSession(c)
	session.Set(registrationEmailGrantSessionKey, grant)
	if err := session.Save(); err != nil {
		common.DeleteRegistrationEmailGrant(grant)
		common.ApiErrorI18n(c, i18n.MsgUserSessionSaveFailed)
		return
	}
	if previousGrant != "" && previousGrant != grant {
		common.DeleteRegistrationEmailGrant(previousGrant)
	}

	common.ApiSuccess(c, registrationEmailVerificationData{
		Verified:  true,
		ExpiresIn: common.VerificationValidMinutes * 60,
	})
}

func GetRegistrationEmailVerificationStatus(c *gin.Context) {
	var request registrationEmailStatusRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	request.Email = strings.TrimSpace(request.Email)
	if request.Email == "" || len(request.Email) > maxRegistrationEmailLength {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	common.ApiSuccess(c, registrationEmailVerificationData{
		Verified: registrationEmailGrantMatches(c, request.Email),
	})
}
