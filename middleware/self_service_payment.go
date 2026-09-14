package middleware

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// RequireSelfServicePayment gates customer-initiated payments only. Keep
// settlement callbacks, refunds, and administrative operations outside this gate.
func RequireSelfServicePayment() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read the authoritative row on every request: session/token groups and
		// process-local caches can outlive an administrator's group change.
		user, err := model.GetUserById(c.GetInt("id"), false)
		if err != nil || user == nil || user.Status != common.UserStatusEnabled ||
			!strings.EqualFold(strings.TrimSpace(user.Group), plgGroup) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgForbidden),
			})
			return
		}
		c.Next()
	}
}
