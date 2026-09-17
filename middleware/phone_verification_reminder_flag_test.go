package middleware_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupPhoneReminderFlagDB(t *testing.T) {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalIsMasterNode := common.IsMasterNode
	originalSQLitePath := common.SQLitePath
	originalRedisEnabled := common.RedisEnabled
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")

	common.IsMasterNode = false
	common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.RedisEnabled = false
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	require.NoError(t, os.Setenv("SQL_DSN", "local"))
	require.NoError(t, model.InitDB())
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.Token{}))

	t.Cleanup(func() {
		if model.DB != nil {
			if sqlDB, err := model.DB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.IsMasterNode = originalIsMasterNode
		common.SQLitePath = originalSQLitePath
		common.RedisEnabled = originalRedisEnabled
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	})
}

// TokenAuth must stamp the reminder flag for legacy PLG accounts only, and the
// existing gate must keep answering new PLG accounts with 403 before the flag
// is ever considered.
func TestTokenAuthStampsPhoneVerificationReminderFlag(t *testing.T) {
	setupPhoneReminderFlagDB(t)
	gin.SetMode(gin.TestMode)

	originalSMS := common.SMSVerificationEnabled
	originalToggle := operation_setting.GetPhoneVerificationSetting().APIReminderEnabled
	t.Cleanup(func() {
		common.SMSVerificationEnabled = originalSMS
		operation_setting.GetPhoneVerificationSetting().APIReminderEnabled = originalToggle
	})
	common.SMSVerificationEnabled = true
	operation_setting.GetPhoneVerificationSetting().APIReminderEnabled = true
	start := model.PhoneVerificationRolloutStart().Unix()

	users := []struct {
		id        int
		group     string
		createdAt int64
		verified  int64
		key       string
	}{
		{id: 26001, group: "plg", createdAt: start - 1, key: "legacyplgreminder"},
		{id: 26002, group: "plg", createdAt: start, key: "newplggated"},
		{id: 26003, group: "plg", createdAt: start - 1, verified: start - 1, key: "legacyplgverified"},
		{id: 26004, group: "enterprise", createdAt: start - 1, key: "legacyenterprise"},
	}
	for _, u := range users {
		require.NoError(t, model.DB.Create(&model.User{
			Id: u.id, Username: fmt.Sprintf("reminder-user-%d", u.id), Password: "password",
			Group: u.group, Status: common.UserStatusEnabled, CreatedAt: u.createdAt,
			PhoneVerifiedAt: u.verified, IsEnterprise: u.group == "enterprise",
			AffCode: fmt.Sprintf("aff%d", u.id),
		}).Error)
		require.NoError(t, model.DB.Create(&model.Token{
			Id: u.id, UserId: u.id, Key: u.key, Status: common.TokenStatusEnabled,
			RemainQuota: 1000, ExpiredTime: -1,
		}).Error)
	}

	engine := gin.New()
	engine.POST("/v1/chat/completions", middleware.TokenAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"reminder": common.GetContextKeyBool(c, constant.ContextKeyPhoneVerificationReminderEligible)})
	})

	call := func(key string) (int, string) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[]}`))
		request.Header.Set("Authorization", "Bearer sk-"+key)
		request.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(recorder, request)
		return recorder.Code, recorder.Body.String()
	}

	code, body := call("legacyplgreminder")
	require.Equal(t, http.StatusOK, code, body)
	require.JSONEq(t, `{"reminder":true}`, body)

	code, body = call("newplggated")
	require.Equal(t, http.StatusForbidden, code, body)
	require.Contains(t, body, `"notify"`)

	code, body = call("legacyplgverified")
	require.Equal(t, http.StatusOK, code, body)
	require.JSONEq(t, `{"reminder":false}`, body)

	code, body = call("legacyenterprise")
	require.Equal(t, http.StatusOK, code, body)
	require.JSONEq(t, `{"reminder":false}`, body)

	operation_setting.GetPhoneVerificationSetting().APIReminderEnabled = false
	code, body = call("legacyplgreminder")
	require.Equal(t, http.StatusOK, code, body)
	require.JSONEq(t, `{"reminder":false}`, body)
}
