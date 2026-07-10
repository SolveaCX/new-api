package controller

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

const emailDomainWhitelistErrorMessage = "The administrator has enabled the email domain name whitelist, and your email address is not allowed due to special symbols or it's not in the whitelist."

func validateEmailDomainRestriction(email string) error {
	if !common.EmailDomainRestrictionEnabled {
		return nil
	}
	email = strings.TrimSpace(email)
	if err := common.Validate.Var(email, "required,email"); err != nil {
		return errors.New(emailDomainWhitelistErrorMessage)
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return errors.New(emailDomainWhitelistErrorMessage)
	}
	domainPart := strings.ToLower(strings.TrimSpace(parts[1]))
	for _, domain := range common.EmailDomainWhitelist {
		if domainPart == strings.ToLower(strings.TrimSpace(domain)) {
			return nil
		}
	}
	return errors.New(emailDomainWhitelistErrorMessage)
}

func evaluateRegistrationEmail(email string) (service.RegistrationEmailDecision, error) {
	if err := validateEmailDomainRestriction(email); err != nil {
		return service.RegistrationEmailDecision{}, err
	}
	return service.EvaluateRegistrationEmail(
		email,
		system_setting.GetRegistrationSecuritySettings(),
		model.IsRegistrationDomainBlocked,
	)
}

func registrationEmailErrorKey(err error) (string, bool) {
	switch {
	case errors.Is(err, common.ErrInvalidEmailDomain):
		return i18n.MsgRegistrationEmailDomainInvalid, true
	case errors.Is(err, service.ErrSubdomainEmailRegistrationRejected):
		return i18n.MsgRegistrationEmailSubdomainRejected, true
	case errors.Is(err, service.ErrRegistrationDomainUnavailable), errors.Is(err, model.ErrRegistrationDomainBlocked):
		return i18n.MsgRegistrationEmailDomainUnavailable, true
	default:
		return "", false
	}
}

func respondRegistrationEmailError(c *gin.Context, err error) {
	if key, ok := registrationEmailErrorKey(err); ok {
		common.ApiErrorI18n(c, key)
		return
	}
	common.ApiError(c, err)
}

func registrationEmailErrorMessage(c *gin.Context, err error) string {
	if key, ok := registrationEmailErrorKey(err); ok {
		return i18n.T(c, key)
	}
	return err.Error()
}

func registerLegacyOAuthUser(c *gin.Context, user *model.User, inviterID int) error {
	decision, err := evaluateRegistrationEmail(user.Email)
	if err != nil {
		return err
	}
	user.EmailDomain = decision.Domain
	if _, err := model.RegisterUserWithDomainRisk(user, inviterID, c.ClientIP(), decision.Policy, nil); err != nil {
		return err
	}
	user.FinalizeOAuthUserCreation(inviterID)
	return nil
}
