package controller

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func createTestEncryptedCustomerInvite(t *testing.T, keyHex string, invite service.CustomerInvite) string {
	t.Helper()
	key, err := hex.DecodeString(keyHex)
	require.NoError(t, err)
	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	nonce := make([]byte, 12)
	for i := range nonce {
		nonce[i] = byte(i + 1)
	}
	plaintext, err := common.Marshal(invite)
	require.NoError(t, err)
	sealed := gcm.Seal(nil, nonce, plaintext, nil)
	envelope := append([]byte{1}, append(nonce, sealed...)...)
	return base64.RawURLEncoding.EncodeToString(envelope)
}

func performRegisterRequestWithQuery(t *testing.T, path string, body []byte, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("register-session-test"))))
	router.POST("/api/user/register", Register)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, request)

	return recorder
}

func TestRegisterRegularUser_DefaultsToPLGAndRegularQuota(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalQuotaForNewUser := common.QuotaForNewUser
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		common.QuotaForNewUser = originalQuotaForNewUser
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	common.QuotaPerUnit = 500000
	common.QuotaForNewUser = 500000 // $1.00 USD

	body, err := common.Marshal(map[string]any{
		"username": "regular-user-1",
		"password": "password123",
		"email":    "regular-user-1@example.com",
	})
	require.NoError(t, err)

	recorder := performRegisterRequest(t, body)
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload registerResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)

	var user model.User
	require.NoError(t, db.First(&user, "id = ?", payload.Data.ID).Error)
	require.Equal(t, plgGroup, user.Group, "regular user must default to plg group")
	require.Equal(t, 500000, user.Quota, "regular user must receive $1.00 quota (500,000)")

	// 验证钱包生命周期初始状态
	var walletState model.QuotaLifecycleState
	require.NoError(t, db.First(&walletState, "user_id = ? AND scope_type = ?", user.Id, model.QuotaLifecycleScopeWallet).Error)
	require.Equal(t, int64(500000), walletState.Balance)
}

func TestRegisterFluereUser_EnterpriseGroupAndFifteenDollars(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalQuotaForNewUser := common.QuotaForNewUser
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		common.QuotaForNewUser = originalQuotaForNewUser
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	common.QuotaPerUnit = 500000
	common.QuotaForNewUser = 500000 // regular is $1.00 USD

	expectedFluereQuota := int(15.0 * common.QuotaPerUnit) // 7,500,000 ($15.00 USD)

	// 1. 通过 Payload 携带 is_fluere = true
	body, err := common.Marshal(map[string]any{
		"username":        "fluere-user-payload",
		"password":        "password123",
		"email":           "fluere-payload@example.com",
		"is_fluere":       true,
		"source_platform": "fluere",
	})
	require.NoError(t, err)

	recorder := performRegisterRequest(t, body)
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload registerResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)

	var user model.User
	require.NoError(t, db.First(&user, "id = ?", payload.Data.ID).Error)
	require.Equal(t, "Enterprise", user.Group, "Fluere user must be assigned to Enterprise group")
	require.Equal(t, expectedFluereQuota, user.Quota, "Fluere user must receive $15.00 quota (7,500,000)")

	// 验证钱包生命周期初始状态
	var walletState model.QuotaLifecycleState
	require.NoError(t, db.First(&walletState, "user_id = ? AND scope_type = ?", user.Id, model.QuotaLifecycleScopeWallet).Error)
	require.Equal(t, int64(expectedFluereQuota), walletState.Balance)
}

func TestRegisterFluereUser_ViaQueryParam(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalQuotaForNewUser := common.QuotaForNewUser
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		common.QuotaForNewUser = originalQuotaForNewUser
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	common.QuotaPerUnit = 500000
	common.QuotaForNewUser = 500000

	expectedFluereQuota := int(15.0 * common.QuotaPerUnit)

	body, err := common.Marshal(map[string]any{
		"username": "fluere-user-query",
		"password": "password123",
		"email":    "fluere-query@example.com",
	})
	require.NoError(t, err)

	// URL Query 带有 isFluere=true
	recorder := performRegisterRequestWithQuery(t, "/api/user/register?isFluere=true", body)
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload registerResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)

	var user model.User
	require.NoError(t, db.First(&user, "id = ?", payload.Data.ID).Error)
	require.Equal(t, "Enterprise", user.Group)
	require.Equal(t, expectedFluereQuota, user.Quota)
}

func TestRegisterFluereUser_WithInvitePreservesAttribution(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}, &model.Log{}, &model.CustomerReferralOutbox{}))

	testKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	t.Setenv("FLATKEY_CUSTOMER_INVITE_ENCRYPTION_KEY", testKeyHex)

	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalQuotaForNewUser := common.QuotaForNewUser
	originalQuotaPerUnit := common.QuotaPerUnit
	constant.GenerateDefaultToken = true
	setting.DefaultUseAutoGroup = false
	t.Cleanup(func() {
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		common.QuotaForNewUser = originalQuotaForNewUser
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	common.QuotaPerUnit = 500000
	common.QuotaForNewUser = 500000

	encryptedInvite := createTestEncryptedCustomerInvite(t, testKeyHex, service.CustomerInvite{
		Code:     "fluere-partner-ref-123",
		Platform: "fluere",
	})

	decoded, err := service.DecodeCustomerInvite(encryptedInvite)
	require.NoError(t, err)
	require.Equal(t, "fluere-partner-ref-123", decoded.Code)
	require.Equal(t, "fluere", decoded.Platform)

	body, err := common.Marshal(map[string]any{
		"username":        "fluere-user-inv",
		"password":        "password123",
		"email":           "fluere-inv@example.com",
		"invite":          encryptedInvite,
		"is_fluere":       true,
		"source_platform": "fluere",
	})
	require.NoError(t, err)

	recorder := performRegisterRequest(t, body)
	require.Equal(t, http.StatusOK, recorder.Code, "response: "+recorder.Body.String())

	var payload registerResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success, "payload error: "+recorder.Body.String())

	var user model.User
	require.NoError(t, db.First(&user, "id = ?", payload.Data.ID).Error)
	require.Equal(t, "Enterprise", user.Group)
	require.Equal(t, int(15.0*common.QuotaPerUnit), user.Quota)

	// 验证 CustomerReferralOutbox 中成功创建了待投递的归因事件
	var outboxEvent model.CustomerReferralOutbox
	require.NoError(t, db.First(&outboxEvent, "user_id = ?", user.Id).Error)
	require.Equal(t, model.CustomerReferralOutboxPending, outboxEvent.Status)
	require.Contains(t, outboxEvent.Payload, "fluere-partner-ref-123")
	require.Contains(t, outboxEvent.Payload, "fluere")
}

func TestOAuthFluereUser_EnterpriseGroupAndFifteenDollars(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}, &model.Log{}))

	originalRegisterEnabled := common.RegisterEnabled
	originalQuotaForNewUser := common.QuotaForNewUser
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		common.RegisterEnabled = originalRegisterEnabled
		common.QuotaForNewUser = originalQuotaForNewUser
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	common.RegisterEnabled = true
	common.QuotaPerUnit = 500000
	common.QuotaForNewUser = 500000

	expectedFluereQuota := int(15.0 * common.QuotaPerUnit)

	// 1. 常规 OAuth 注册：未带 isFluere，分组为 plg，额度为 $1.00 USD
	regularUser := createOAuthLanguageTestUser(t, db, "regular-oauth-user", func(req *http.Request) {
		req.RemoteAddr = "192.0.2.1:1234"
	})
	require.Equal(t, plgGroup, regularUser.Group)
	require.Equal(t, 500000, regularUser.Quota)

	// 2. Fluere 渠道 OAuth 注册：携带 is_fluere=true query
	fluereUser := createOAuthLanguageTestUser(t, db, "fluere-oauth-user", func(req *http.Request) {
		req.RemoteAddr = "192.0.2.2:1234"
		req.URL.RawQuery = "is_fluere=true"
	})
	require.Equal(t, "Enterprise", fluereUser.Group)
	require.Equal(t, expectedFluereQuota, fluereUser.Quota)

	// 3. Fluere 渠道 OAuth 注册：通过 Cookie 携带 is_fluere=true
	fluereCookieUser := createOAuthLanguageTestUser(t, db, "fluere-cookie-user", func(req *http.Request) {
		req.RemoteAddr = "192.0.2.3:1234"
		req.AddCookie(&http.Cookie{Name: "is_fluere", Value: "true"})
	})
	require.Equal(t, "Enterprise", fluereCookieUser.Group)
	require.Equal(t, expectedFluereQuota, fluereCookieUser.Quota)
}

