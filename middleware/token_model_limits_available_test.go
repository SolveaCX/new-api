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
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupTokenAvailableModelsContractDB(t *testing.T) {
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
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))

	priority := int64(0)
	weight := uint(100)
	require.NoError(t, model.DB.Create(&model.Channel{
		Id: 94001, Type: constant.ChannelTypeOpenAI, Key: "contract-test-key",
		Status: common.ChannelStatusEnabled, Models: "scope-model", Group: "default",
		Priority: &priority, Weight: &weight,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group: "default", Model: "scope-model", ChannelId: 94001,
		Enabled: true, Priority: &priority, Weight: weight,
	}).Error)

	t.Cleanup(func() {
		if model.DB != nil {
			sqlDB, err := model.DB.DB()
			if err == nil {
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

func TestEnabledEmptyTokenAllowlistRemainsEnabledAndReturnsZeroAvailableModels(t *testing.T) {
	setupTokenAvailableModelsContractDB(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/v1/available_models", nil)
	token := model.Token{
		Id: 1, UserId: 1, Key: "contract-token", Group: "default",
		ModelLimitsEnabled: true, ModelLimits: "", UnlimitedQuota: true,
	}

	require.NoError(t, middleware.SetupContextForToken(context, &token))
	require.True(t, common.GetContextKeyBool(context, constant.ContextKeyTokenModelLimitEnabled))
	value, exists := common.GetContextKey(context, constant.ContextKeyTokenModelLimit)
	require.True(t, exists)
	require.Empty(t, value.(map[string]bool))

	controller.AvailableModels(context)

	var payload struct {
		Success bool               `json:"success"`
		Object  string             `json:"object"`
		Data    []dto.OpenAIModels `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, "list", payload.Object)
	require.Empty(t, payload.Data)
}
