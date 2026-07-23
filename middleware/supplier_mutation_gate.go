package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const (
	SupplierMutationGateDisabledCode    = "supplier_mutation_gate_disabled"
	SupplierMutationGateUnavailableCode = "supplier_mutation_gate_unavailable"
)

// SupplierMutationGate performs a strongly consistent main-DB read for every
// low-frequency finance/master-data mutation. Missing state is synthetic
// disabled; malformed state and database failures fail closed.
func SupplierMutationGate() gin.HandlerFunc {
	return func(c *gin.Context) {
		state, err := model.ReadSupplierAccountingMutationState(model.DB)
		if err != nil {
			abortSupplierMutationGate(c, http.StatusServiceUnavailable, SupplierMutationGateUnavailableCode, i18n.MsgSupplierAccountingMutationGateUnavailable)
			return
		}
		if !state.Enabled {
			abortSupplierMutationGate(c, http.StatusLocked, SupplierMutationGateDisabledCode, i18n.MsgSupplierAccountingMutationsDisabled)
			return
		}
		c.Next()
	}
}

func abortSupplierMutationGate(c *gin.Context, status int, code, messageKey string) {
	c.AbortWithStatusJSON(status, gin.H{
		"success": false,
		"message": i18n.T(c, messageKey),
		"code":    code,
	})
}
