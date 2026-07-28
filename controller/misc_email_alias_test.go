package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type emailVerificationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func TestSendEmailVerificationAllowsDottedAddressWhenAliasRestrictionEnabled(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	originalAliasRestrictionEnabled := common.EmailAliasRestrictionEnabled
	common.EmailAliasRestrictionEnabled = true
	t.Cleanup(func() {
		common.EmailAliasRestrictionEnabled = originalAliasRestrictionEnabled
	})

	email := "liqian.ma@zmo.ai"
	require.NoError(t, db.Create(&model.User{
		Username: "dotted-email-user",
		Email:    email,
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
	}).Error)

	payload := performEmailVerificationRequest(t, email)

	require.False(t, payload.Success)
	require.Equal(t, "邮箱地址已被占用", payload.Message)
}

func TestSendEmailVerificationRejectsPlusAliasWhenAliasRestrictionEnabled(t *testing.T) {
	setupModelListControllerTestDB(t)
	originalAliasRestrictionEnabled := common.EmailAliasRestrictionEnabled
	common.EmailAliasRestrictionEnabled = true
	t.Cleanup(func() {
		common.EmailAliasRestrictionEnabled = originalAliasRestrictionEnabled
	})

	payload := performEmailVerificationRequest(t, "user+tag@zmo.ai")

	require.False(t, payload.Success)
	require.Equal(t, "管理员已启用邮箱地址别名限制，您的邮箱地址由于包含特殊符号而被拒绝。", payload.Message)
}

func performEmailVerificationRequest(t *testing.T, email string) emailVerificationResponse {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/verification", SendEmailVerification)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/verification?email="+url.QueryEscape(email), nil)
	router.ServeHTTP(recorder, request)

	var payload emailVerificationResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	return payload
}
