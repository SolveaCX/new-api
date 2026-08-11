package middleware

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DataToolsAuth is the single credential decision point for /api/data-tools.
// It deliberately does not chain TokenAuth before OAuth, because TokenAuth aborts
// before a data-tools-only OAuth bearer can be considered.
func DataToolsAuth(requiredScope string, allowSession bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		bearer := strings.TrimSpace(c.GetHeader("Authorization"))
		if bearer != "" {
			if strings.HasPrefix(bearer, "Bearer ") || strings.HasPrefix(bearer, "bearer ") {
				bearer = strings.TrimSpace(bearer[7:])
			}
			if bearer == "" {
				abortDataToolsAuth(c, common.TranslateMessage(c, i18n.MsgTokenNotProvided))
				return
			}
			if isDataToolsOAuthAccessJWT(bearer) {
				if authenticateDataToolsOAuth(c, bearer, requiredScope) {
					c.Next()
				}
				return
			}
			if authenticateDataToolsAPIKey(c, bearer) {
				c.Next()
			}
			return
		}

		if allowSession && authenticateDataToolsSession(c) {
			c.Next()
			return
		}
		abortDataToolsAuth(c, common.TranslateMessage(c, i18n.MsgTokenNotProvided))
	}
}

func authenticateDataToolsSession(c *gin.Context) bool {
	session := sessions.Default(c)
	id := session.Get("id")
	if id == nil {
		return false
	}
	status, ok := session.Get("status").(int)
	if !ok || status != common.UserStatusEnabled {
		return false
	}
	c.Set("id", id)
	return true
}

func authenticateDataToolsAPIKey(c *gin.Context, rawKey string) bool {
	key := strings.TrimPrefix(rawKey, "sk-")
	parts := strings.Split(key, "-")
	key = parts[0]
	token, err := model.ValidateUserToken(key)
	if token != nil && c.GetInt("id") == 0 {
		c.Set("id", token.UserId)
	}
	if err != nil {
		if errors.Is(err, model.ErrDatabase) {
			common.SysLog("DataToolsAuth ValidateUserToken database error: " + err.Error())
			abortWithOpenAiMessage(c, http.StatusInternalServerError, common.TranslateMessage(c, i18n.MsgDatabaseError))
		} else {
			logTokenAuthFailure(c, token, err)
			statusCode, message, code := tokenAuthErrorResponse(c, err)
			abortWithOpenAiMessage(c, statusCode, message, code...)
		}
		return false
	}

	return setupDataToolsTokenContext(c, token, parts)
}

func authenticateDataToolsOAuth(c *gin.Context, rawJWT string, requiredScope string) bool {
	signer, err := service.NewMcpOAuthSignerFromEnv(service.McpOAuthSigningConfig{})
	if err != nil {
		common.SysLog("DataToolsAuth OAuth signer unavailable: " + err.Error())
		abortDataToolsAuth(c, common.TranslateMessage(c, i18n.MsgTokenInvalid))
		return false
	}
	claims, err := signer.VerifyAccessToken(rawJWT, requiredScope)
	if err != nil {
		abortDataToolsAuth(c, common.TranslateMessage(c, i18n.MsgTokenInvalid))
		return false
	}
	identity, err := model.ResolveMcpOAuthDataToolIdentity(model.McpOAuthDataToolClaims{
		Subject:  claims.Subject,
		GrantID:  claims.GrantID,
		ClientID: claims.ClientID,
		Resource: claims.Resource,
	}, common.GetTimestamp())
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			common.SysLog("DataToolsAuth OAuth identity failed: " + err.Error())
		}
		abortDataToolsAuth(c, common.TranslateMessage(c, i18n.MsgTokenInvalid))
		return false
	}
	if !dataToolsOAuthGrantAllowsScope(identity.Scopes, requiredScope) {
		abortDataToolsAuth(c, common.TranslateMessage(c, i18n.MsgTokenInvalid))
		return false
	}
	if err := SetupContextForToken(c, &identity.Token); err != nil {
		return false
	}
	common.SetContextKey(c, constant.ContextKeyOAuthGrantId, identity.GrantPublicID)
	return true
}

func dataToolsOAuthGrantAllowsScope(grantScopes string, requiredScope string) bool {
	scopes, err := service.NormalizeMcpOAuthScopes(grantScopes)
	if err != nil {
		return false
	}
	for _, scope := range scopes {
		if scope == requiredScope {
			return true
		}
	}
	return false
}

func setupDataToolsTokenContext(c *gin.Context, token *model.Token, parts []string) bool {
	userCache, err := model.GetUserCache(token.UserId)
	if err != nil {
		common.SysLog(fmt.Sprintf("DataToolsAuth GetUserCache error for user %d: %v", token.UserId, err))
		abortWithOpenAiMessage(c, http.StatusInternalServerError, common.TranslateMessage(c, i18n.MsgDatabaseError))
		return false
	}
	if userCache.Status != common.UserStatusEnabled {
		abortWithOpenAiMessage(c, http.StatusForbidden, common.TranslateMessage(c, i18n.MsgAuthUserBanned))
		return false
	}
	userCache.WriteContext(c)
	userGroup, tokenGroup, contextToken := resolveTokenGroupsForUser(userCache, token)
	common.SetContextKey(c, constant.ContextKeyUserGroup, userGroup)
	if tokenGroup != "" {
		if _, ok := service.GetUserUsableGroups(userGroup)[tokenGroup]; !ok {
			abortWithOpenAiMessage(c, http.StatusForbidden, common.TranslateMessage(c, i18n.MsgTokenGroupNoPermission, map[string]any{"Group": tokenGroup}))
			return false
		}
		if !ratio_setting.ContainsGroupRatio(tokenGroup) && tokenGroup != "auto" {
			abortWithOpenAiMessage(c, http.StatusForbidden, common.TranslateMessage(c, i18n.MsgTokenGroupDeprecated, map[string]any{"Group": tokenGroup}))
			return false
		}
		userGroup = tokenGroup
	}
	common.SetContextKey(c, constant.ContextKeyUsingGroup, userGroup)
	return SetupContextForToken(c, &contextToken, parts...) == nil
}

func abortDataToolsAuth(c *gin.Context, message string) {
	abortWithOpenAiMessage(c, http.StatusUnauthorized, message, types.ErrorCodeInvalidRequest)
}

func isDataToolsOAuthAccessJWT(raw string) bool {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return false
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	var header struct {
		Type string `json:"typ"`
		Alg  string `json:"alg"`
		Kid  string `json:"kid"`
	}
	if err := common.Unmarshal(headerBytes, &header); err != nil {
		return false
	}
	return header.Type == "at+jwt" && header.Alg == "EdDSA" && strings.TrimSpace(header.Kid) != ""
}
