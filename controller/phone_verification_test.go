package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSendPhoneVerificationRejectsAlreadyRegisteredPhone(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	require.NoError(t, db.Create(&model.User{
		Username:    "existing-phone-user",
		Password:    "hashed-password",
		PhoneNumber: "+8613800138000",
	}).Error)

	originalEnabled := common.SMSVerificationEnabled
	originalMock := common.TeleSignMockEnabled
	t.Cleanup(func() {
		common.SMSVerificationEnabled = originalEnabled
		common.TeleSignMockEnabled = originalMock
	})
	common.SMSVerificationEnabled = true
	common.TeleSignMockEnabled = true

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/phone-verification", SendPhoneVerification)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/phone-verification?phone_number=%2B86%20138-0013-8000", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.False(t, payload.Success)
	require.Contains(t, payload.Message, "registered")
}

func TestRegisterRequiresPhoneVerificationCodeWhenSMSVerificationEnabled(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalSMSVerificationEnabled := common.SMSVerificationEnabled
	originalGenerateDefaultToken := constant.GenerateDefaultToken
	t.Cleanup(func() {
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		common.SMSVerificationEnabled = originalSMSVerificationEnabled
		constant.GenerateDefaultToken = originalGenerateDefaultToken
	})
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	common.SMSVerificationEnabled = true
	constant.GenerateDefaultToken = false

	body, err := common.Marshal(map[string]any{
		"username":                "phone-verification-required",
		"password":                "password123",
		"email":                   "phone-verification@example.com",
		"phone_number":            "+8613800138000",
		"phone_verification_code": "",
	})
	require.NoError(t, err)
	recorder := performRegisterRequest(t, body)
	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.False(t, payload.Success)
	var count int64
	require.NoError(t, db.Model(&model.User{}).Where("username = ?", "phone-verification-required").Count(&count).Error)
	require.Zero(t, count)
}

func TestRegisterWithPhoneVerificationPersistsVerifiedPhone(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalSMSVerificationEnabled := common.SMSVerificationEnabled
	originalGenerateDefaultToken := constant.GenerateDefaultToken
	t.Cleanup(func() {
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		common.SMSVerificationEnabled = originalSMSVerificationEnabled
		constant.GenerateDefaultToken = originalGenerateDefaultToken
	})
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	common.SMSVerificationEnabled = true
	constant.GenerateDefaultToken = false

	phone := "+8613800138000"
	common.RegisterVerificationCodeWithKey(phone, "654321", common.SMSVerificationPurpose)
	body, err := common.Marshal(map[string]any{
		"username":                "phone-verified-user",
		"password":                "password123",
		"email":                   "phone-verified@example.com",
		"phone_number":            phone,
		"phone_verification_code": "654321",
	})
	require.NoError(t, err)
	recorder := performRegisterRequest(t, body)
	var payload registerResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)

	var user model.User
	require.NoError(t, db.First(&user, "username = ?", "phone-verified-user").Error)
	require.Equal(t, phone, user.PhoneNumber)
	require.NotZero(t, user.PhoneVerifiedAt)
}
