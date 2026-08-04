package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAssetTaskQueuePersistsQueuedTaskAndReturnsImmediately(t *testing.T) {
	restoreDB := useControllerAssetTaskDBForTest(t)
	defer restoreDB()

	c := newControllerAssetTaskContext(`{
		"model":"seedance-2.0",
		"content":[{"type":"image_url","image_url":{"url":"asset://ast_1234567890abcdefABCDEF1234567890"},"role":"reference_image"}]
	}`)
	c.Request.Header.Set("Authorization", "Bearer sk-user-secret")
	common.SetContextKey(c, constant.ContextKeyChannelKey, `{"api_key":"sk-provider-secret","signed":"https://signed.example/?X-Goog-Signature=abc"}`)
	common.SetContextKey(c, constant.ContextKeyChannelId, 131)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeBytePlus)

	info := &relaycommon.RelayInfo{
		UserId:          7,
		TokenId:         11,
		UsingGroup:      "default",
		TokenGroup:      "default",
		OriginModelName: "seedance-2.0",
		RelayMode:       20,
		BillingSource:   service.BillingSourceWallet,
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{Action: constant.TaskActionTextGenerate, PublicTaskID: "task_public_queued"},
		PriceData: types.PriceData{
			Quota:      123,
			ModelPrice: 0.02,
		},
	}
	preflight := &relay.TaskPreflightResult{Platform: constant.TaskPlatform("107"), Quota: 123}

	video, taskErr := queueAssetTaskForPreparation(c, info, preflight)
	require.Nil(t, taskErr)
	require.NotNil(t, video)
	require.Equal(t, "task_public_queued", video.ID)
	require.Equal(t, "task_public_queued", video.TaskID)
	require.Equal(t, dto.VideoStatusQueued, video.Status)
	require.Equal(t, 0, video.Progress)

	var task model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_public_queued").First(&task).Error)
	require.EqualValues(t, model.TaskStatusQueued, task.Status)
	require.Equal(t, model.TaskPreparationStatusPreparingAssets, task.PreparationStatus)
	require.Equal(t, 123, task.Quota)
	require.Equal(t, 7, task.UserId)
	require.Equal(t, 11, task.PrivateData.TokenId)
	require.Equal(t, service.BillingSourceWallet, task.PrivateData.BillingSource)
	require.NotNil(t, task.PrivateData.BillingContext)
	require.Empty(t, task.PrivateData.UpstreamTaskID)

	privateData, err := common.Marshal(task.PrivateData)
	require.NoError(t, err)
	persisted := string(task.NormalizedRequestPayload) + string(privateData) + string(task.Data)
	require.Contains(t, persisted, "asset://ast_1234567890abcdefABCDEF1234567890")
	require.NotContains(t, persisted, "sk-user-secret")
	require.NotContains(t, persisted, "sk-provider-secret")
	require.NotContains(t, persisted, "signed.example")
	require.NotContains(t, persisted, "X-Goog-Signature")
}

func newControllerAssetTaskContext(body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func useControllerAssetTaskDBForTest(t *testing.T) func() {
	t.Helper()
	oldDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Task{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	model.DB = db
	return func() {
		model.DB = oldDB
		_ = sqlDB.Close()
	}
}
