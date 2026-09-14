package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMaskPhoneNumberKeepsCountryCodeAndLastTwoDigits(t *testing.T) {
	require.Equal(t, "+1********23", maskPhoneNumber("+14155550123"))
	require.Equal(t, "+8**********00", maskPhoneNumber("+8613800138000"))
	require.Equal(t, "", maskPhoneNumber(""))
	require.Equal(t, "", maskPhoneNumber("   "))
	// Never leak a full number, however short the input is.
	require.Equal(t, "+****", maskPhoneNumber("+1234"))
	require.NotContains(t, maskPhoneNumber("+14155550123"), "4155550")
}

type phoneStatusPayload struct {
	Success bool `json:"success"`
	Data    struct {
		PhoneBound             bool   `json:"phone_bound"`
		PhoneNumber            string `json:"phone_number"`
		PhoneVerifiedAt        int64  `json:"phone_verified_at"`
		VerificationRequired   bool   `json:"verification_required"`
		SMSVerificationEnabled bool   `json:"sms_verification_enabled"`
		RolloutStartAt         int64  `json:"rollout_start_at"`
	} `json:"data"`
}

// The product uses verification_required to decide whether to open the binding
// dialog, so each case below is a real product state.
func TestGetSelfPhoneVerificationStatusDrivesTheBindingDialog(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserPhoneBinding{}))
	originalEnabled := common.SMSVerificationEnabled
	t.Cleanup(func() { common.SMSVerificationEnabled = originalEnabled })
	common.SMSVerificationEnabled = true
	start := model.PhoneVerificationRolloutStart().Unix()

	create := func(name, group, aff string, createdAt, verifiedAt int64, phone string) *model.User {
		u := &model.User{Username: name, Password: "hashed-password", Status: common.UserStatusEnabled, Group: group, AffCode: aff}
		require.NoError(t, db.Create(u).Error)
		require.NoError(t, db.Model(u).Updates(map[string]any{
			"created_at": createdAt, "phone_verified_at": verifiedAt, "phone_number": phone,
		}).Error)
		return u
	}
	oldUser := create("status-old", "plg", "aff-status-old", start-3600, 0, "")
	newUser := create("status-new", "plg", "aff-status-new", start+3600, 0, "")
	boundUser := create("status-bound", "plg", "aff-status-bound", start+3600, start+7200, "+14155550123")
	entUser := create("status-ent", "enterprise", "aff-status-ent", start+3600, 0, "")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	for _, u := range []*model.User{oldUser, newUser, boundUser, entUser} {
		userID := u.Id
		router.GET("/api/user/self/phone-status/"+strconv.Itoa(userID), func(c *gin.Context) {
			c.Set("id", userID)
			GetSelfPhoneVerificationStatus(c)
		})
	}
	fetch := func(userID int) phoneStatusPayload {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/user/self/phone-status/"+strconv.Itoa(userID), nil))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		var payload phoneStatusPayload
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
		require.True(t, payload.Success)
		return payload
	}

	// Account created before the rollout: never prompted.
	got := fetch(oldUser.Id)
	require.False(t, got.Data.VerificationRequired)
	require.False(t, got.Data.PhoneBound)
	require.Equal(t, start, got.Data.RolloutStartAt)
	require.True(t, got.Data.SMSVerificationEnabled)

	// Account created after the rollout with no phone: open the dialog.
	got = fetch(newUser.Id)
	require.True(t, got.Data.VerificationRequired)
	require.False(t, got.Data.PhoneBound)
	require.Equal(t, "", got.Data.PhoneNumber)

	// Already verified: no dialog, masked number available for display.
	got = fetch(boundUser.Id)
	require.False(t, got.Data.VerificationRequired)
	require.True(t, got.Data.PhoneBound)
	require.Equal(t, "+1********23", got.Data.PhoneNumber)
	require.Equal(t, start+7200, got.Data.PhoneVerifiedAt)

	// Non-PLG account: exempt even though it is new.
	require.False(t, fetch(entUser.Id).Data.VerificationRequired)

	// Feature off: nothing is required, and the client can tell why.
	common.SMSVerificationEnabled = false
	got = fetch(newUser.Id)
	require.False(t, got.Data.VerificationRequired)
	require.False(t, got.Data.SMSVerificationEnabled)
}

// The gate lives in TokenAuth (the /v1 API), never in UserAuth, so an account
// that still has to bind can always read its own status and learn why.
func TestPhoneStatusRouteIsReachableWhileBindingIsStillRequired(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserPhoneBinding{}))
	originalEnabled := common.SMSVerificationEnabled
	t.Cleanup(func() { common.SMSVerificationEnabled = originalEnabled })
	common.SMSVerificationEnabled = true

	user := &model.User{Username: "status-gated", Password: "hashed-password", Status: common.UserStatusEnabled, Group: "plg", AffCode: "aff-status-gated"}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Model(user).Update("created_at", model.PhoneVerificationRolloutStart().Unix()+60).Error)
	require.True(t, (&model.UserBase{Group: user.Group, CreatedAt: model.PhoneVerificationRolloutStart().Unix() + 60}).PhoneVerificationRequired())

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/user/self/phone-status", func(c *gin.Context) {
		c.Set("id", user.Id)
		GetSelfPhoneVerificationStatus(c)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/user/self/phone-status", nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var payload phoneStatusPayload
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Data.VerificationRequired)
}
