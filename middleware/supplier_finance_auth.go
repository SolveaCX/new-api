package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
)

const (
	SupplierBatchCurrentVerifierHashEnv = "SUPPLIER_BATCH_TOKEN_CURRENT_SHA256"
	SupplierBatchNextVerifierHashEnv    = "SUPPLIER_BATCH_TOKEN_NEXT_SHA256"
	SupplierBatchTrustedIdentityEnv     = "SUPPLIER_BATCH_TRUSTED_JOB_IDENTITY"
)

type supplierBatchPrincipalContextKey struct{}

// FinanceAuth is the Root-only authentication boundary for supply-chain
// financial reads and mutations. It intentionally remains distinct from the
// generic RootAuth name so route-classification tests can enforce the finance
// surface contract.
func FinanceAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHelper(c, common.RoleRootUser)
	}
}

// SupplierBatchAuth accepts only the scheduler current/next bearer slots. The
// configured values are SHA-256 verifier hashes; raw bearer secrets never
// reside in the Console configuration.
func SupplierBatchAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentHash, currentConfigured, currentValid := supplierBatchVerifierHash(os.Getenv(SupplierBatchCurrentVerifierHashEnv))
		nextHash, nextConfigured, nextValid := supplierBatchVerifierHash(os.Getenv(SupplierBatchNextVerifierHashEnv))
		if (!currentConfigured && !nextConfigured) || !currentValid || !nextValid {
			supplierBatchAuthError(c, http.StatusServiceUnavailable, "verifier_unavailable")
			return
		}

		trustedIdentity := strings.TrimSpace(os.Getenv(SupplierBatchTrustedIdentityEnv))
		if trustedIdentity == "" || len(trustedIdentity) > 256 || strings.ContainsAny(trustedIdentity, "\r\n\x00") {
			supplierBatchAuthError(c, http.StatusServiceUnavailable, "config_unavailable")
			return
		}

		token, ok := supplierBatchBearer(c.GetHeader("Authorization"))
		if !ok {
			supplierBatchAuthError(c, http.StatusUnauthorized, "unauthorized")
			return
		}
		digest := sha256.Sum256(token)
		auditSlot := ""
		if currentConfigured && subtle.ConstantTimeCompare(digest[:], currentHash) == 1 {
			auditSlot = dto.SupplierBatchAuditSlotCurrent
		} else if nextConfigured && subtle.ConstantTimeCompare(digest[:], nextHash) == 1 {
			auditSlot = dto.SupplierBatchAuditSlotNext
		}
		if auditSlot == "" {
			supplierBatchAuthError(c, http.StatusUnauthorized, "unauthorized")
			return
		}

		principal := dto.SupplierBatchSchedulerPrincipal{TrustedJobIdentity: trustedIdentity, AuditSlot: auditSlot}
		requestContext := context.WithValue(c.Request.Context(), supplierBatchPrincipalContextKey{}, principal)
		c.Request = c.Request.WithContext(requestContext)
		c.Next()
	}
}

func SupplierBatchPrincipalFromContext(c *gin.Context) (dto.SupplierBatchSchedulerPrincipal, bool) {
	if c == nil || c.Request == nil {
		return dto.SupplierBatchSchedulerPrincipal{}, false
	}
	principal, ok := c.Request.Context().Value(supplierBatchPrincipalContextKey{}).(dto.SupplierBatchSchedulerPrincipal)
	return principal, ok && principal.TrustedJobIdentity != "" && (principal.AuditSlot == dto.SupplierBatchAuditSlotCurrent || principal.AuditSlot == dto.SupplierBatchAuditSlotNext)
}

func supplierBatchVerifierHash(raw string) ([]byte, bool, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false, true
	}
	decoded, err := hex.DecodeString(raw)
	if err != nil || len(decoded) != sha256.Size {
		return nil, true, false
	}
	return decoded, true, true
}

func supplierBatchBearer(authorization string) ([]byte, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(authorization, prefix) || strings.ContainsAny(authorization, "\r\n\t") {
		return nil, false
	}
	raw := strings.TrimPrefix(authorization, prefix)
	if raw == "" || strings.TrimSpace(raw) != raw || strings.Contains(raw, " ") {
		return nil, false
	}
	token, err := base64.RawURLEncoding.DecodeString(raw)
	return token, err == nil && len(token) == 32
}

func supplierBatchAuthError(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"success": false, "message": "supplier batch authentication failed", "code": code})
}
