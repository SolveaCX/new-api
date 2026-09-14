package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSendPhoneVerificationHidesAlreadyRegisteredPhone(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserPhoneBinding{}))
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
	router.POST("/api/phone-verification", SendPhoneVerification)
	recorder := httptest.NewRecorder()
	body := strings.NewReader(`{"phone_number":"+86 138-0013-8000"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/phone-verification", body)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Empty(t, payload.Message)
}

func TestRegisterRequiresPhoneVerificationCodeWhenSMSVerificationEnabled(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserPhoneBinding{}))
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
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserPhoneBinding{}))
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

func TestBindPhonePersistsVerifiedPhone(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserPhoneBinding{}))
	user := &model.User{
		Username: "phone-bind-user",
		Password: "hashed-password",
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)

	originalEnabled := common.SMSVerificationEnabled
	t.Cleanup(func() { common.SMSVerificationEnabled = originalEnabled })
	common.SMSVerificationEnabled = true

	phone := "+14155550123"
	common.RegisterVerificationCodeWithKey(phone, "654321", common.SMSVerificationPurpose)
	body, err := common.Marshal(map[string]string{
		"phone_number":            phone,
		"phone_verification_code": "654321",
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/user/self/phone", func(c *gin.Context) {
		c.Set("id", user.Id)
		BindPhone(c)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/user/self/phone", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			PhoneNumber     string `json:"phone_number"`
			PhoneVerifiedAt int64  `json:"phone_verified_at"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, phone, payload.Data.PhoneNumber)
	require.NotZero(t, payload.Data.PhoneVerifiedAt)

	var stored model.User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.Equal(t, phone, stored.PhoneNumber)
	require.NotZero(t, stored.PhoneVerifiedAt)
}

// GetSelf must expose the server-side decision so the console dialog follows
// the same "new PLG accounts only" rule as the API gate.
func TestGetSelfReportsPhoneVerificationRequiredOnlyForNewPlgAccounts(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserPhoneBinding{}))
	originalEnabled := common.SMSVerificationEnabled
	t.Cleanup(func() { common.SMSVerificationEnabled = originalEnabled })
	common.SMSVerificationEnabled = true
	start := model.PhoneVerificationRolloutStart().Unix()

	// aff_code carries a unique index, so each fixture needs its own value.
	oldUser := &model.User{Username: "old-plg", Password: "hashed-password", Status: common.UserStatusEnabled, Group: "plg", AffCode: "aff-old-plg"}
	require.NoError(t, db.Create(oldUser).Error)
	require.NoError(t, db.Model(oldUser).Update("created_at", start-3600).Error)
	newUser := &model.User{Username: "new-plg", Password: "hashed-password", Status: common.UserStatusEnabled, Group: "plg", AffCode: "aff-new-plg"}
	require.NoError(t, db.Create(newUser).Error)
	require.NoError(t, db.Model(newUser).Update("created_at", start+3600).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("phone-gate-test"))))
	// One route per fixture so the handler sees the right user id.
	for _, id := range []int{oldUser.Id, newUser.Id} {
		userID := id
		router.GET("/api/user/self/"+strconv.Itoa(userID), func(c *gin.Context) {
			c.Set("id", userID)
			c.Set("role", common.RoleCommonUser)
			GetSelf(c)
		})
	}
	fetch := func(userID int) bool {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/user/self/"+strconv.Itoa(userID), nil)
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		var payload struct {
			Success bool `json:"success"`
			Data    struct {
				PhoneVerificationRequired bool `json:"phone_verification_required"`
			} `json:"data"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
		require.True(t, payload.Success)
		return payload.Data.PhoneVerificationRequired
	}

	require.False(t, fetch(oldUser.Id), "account created before rollout must not be asked to bind")
	require.True(t, fetch(newUser.Id), "account created after rollout must bind")

	require.NoError(t, db.Model(newUser).Update("phone_verified_at", start+7200).Error)
	require.False(t, fetch(newUser.Id), "verified account is done")
}
