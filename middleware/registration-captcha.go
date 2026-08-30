package middleware

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func RegistrationCaptchaCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(c.Query("captcha_token"))
		if token == "" {
			common.ApiErrorI18n(c, i18n.MsgRegistrationCaptchaRequired)
			c.Abort()
			return
		}
		valid, err := service.ConsumeRegistrationCaptchaToken(token, c.ClientIP())
		if err != nil {
			common.ApiErrorI18n(c, i18n.MsgRegistrationCaptchaUnavailable)
			c.Abort()
			return
		}
		if !valid {
			common.ApiErrorI18n(c, i18n.MsgRegistrationCaptchaInvalid)
			c.Abort()
			return
		}
		c.Next()
	}
}
