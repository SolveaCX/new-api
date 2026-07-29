package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// FinanceAuth is the Root-only authentication boundary for supply-chain
// financial reads and mutations. It intentionally remains distinct from the
// generic RootAuth name so route-classification tests can enforce the finance
// surface contract.
func FinanceAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHelperWithInsufficientRoleStatus(c, common.RoleRootUser, http.StatusForbidden)
	}
}
