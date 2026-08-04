package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	_ "unsafe"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

//go:linkname controllerAssetTaskModelCommonKeyCol github.com/QuantumNous/new-api/model.commonKeyCol
var controllerAssetTaskModelCommonKeyCol string

//go:linkname registerTaskAdaptorForTest github.com/QuantumNous/new-api/relay.registerTaskAdaptorForTest
func registerTaskAdaptorForTest(platform constant.TaskPlatform, adaptor channel.TaskAdaptor) func()

func TestAssetRelayTaskQueuesBeforeProviderOrMaterializer(t *testing.T) {
	restoreDB := useControllerAssetTaskDBForTest(t)
	defer restoreDB()
	restorePricing := useControllerAssetTaskPricingForTest(t)
	defer restorePricing()

	var materializerCalls atomic.Int32
	restoreMaterializer := service.RegisterAssetMaterializer(constant.ChannelTypeBytePlus, controllerAssetMaterializerWithCounter{calls: &materializerCalls})
	defer restoreMaterializer()
	adaptor := &controllerFakeTaskAdaptor{}
	restoreAdaptor := registerTaskAdaptorForTest(constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeBytePlus)), adaptor)
	defer restoreAdaptor()

	publicID := "ast_1234567890abcdefABCDEF1234567890"
	seedControllerRelayUserToken(t, 7, 11, 10000, 10000)
	seedControllerTaskChannel(t, 131, "sk-provider-secret-queue")
	seedControllerAsset(t, 7, publicID, time.Now().Add(time.Hour).Unix())
	model.InitChannelCache()

	body := seedanceTaskBody(publicID)
	c, recorder := newControllerRelayTaskContext(body)
	c.Request.Header.Set("Authorization", "Bearer sk-user-secret")
	common.SetContextKey(c, constant.ContextKeyUserId, 7)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyTokenId, 11)
	common.SetContextKey(c, constant.ContextKeyTokenKey, "sk-task-token-11")
	common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	c.Set("token_name", "task-token")
	c.Set("token_quota", 10000)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{"seedance-2.0": true})
	common.SetContextKey(c, constant.ContextKeyChannelId, 131)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeBytePlus)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-provider-secret-queue")
	common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: "wallet_only"})

	var req dto.SeedanceVideoRequest
	require.NoError(t, common.Unmarshal([]byte(body), &req))
	refs, apiErr := service.ResolveAssetReferences(c, 7, &req)
	require.Nil(t, apiErr)
	common.SetContextKey(c, constant.ContextKeyAssetReferenceSet, refs)

	RelayTask(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.EqualValues(t, 1, adaptor.validateCalls.Load())
	require.EqualValues(t, 1, adaptor.estimateCalls.Load())
	require.Zero(t, adaptor.providerCalls.Load(), "queued response must not submit upstream")
	require.Zero(t, materializerCalls.Load(), "external RelayTask must not materialize before queue response")

	var response dto.OpenAIVideo
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.NotEmpty(t, response.ID)
	require.Equal(t, response.ID, response.TaskID)
	require.Equal(t, "seedance-2.0", response.Model)
	require.Equal(t, dto.VideoStatusQueued, response.Status)
	require.Equal(t, 0, response.Progress)
	require.NotZero(t, response.CreatedAt)

	var task model.Task
	require.NoError(t, model.DB.Where("task_id = ?", response.TaskID).First(&task).Error)
	require.EqualValues(t, model.TaskStatusQueued, task.Status)
	require.Equal(t, model.TaskPreparationStatusPreparingAssets, task.PreparationStatus)
	require.Equal(t, 0, task.ChannelId, "queued asset task is not pinned before provider acceptance")
	require.Empty(t, task.PrivateData.UpstreamTaskID)
	require.Equal(t, response.TaskID, task.TaskID)
	require.Equal(t, 7, task.UserId)
	require.Equal(t, 11, task.PrivateData.TokenId)
	require.NotNil(t, task.PrivateData.BillingContext)
	require.Greater(t, task.Quota, 0)

	var queuedCount int64
	require.NoError(t, model.DB.Model(&model.Task{}).Where("task_id = ?", response.TaskID).Count(&queuedCount).Error)
	require.EqualValues(t, 1, queuedCount)
	var user model.User
	require.NoError(t, model.DB.First(&user, 7).Error)
	require.Equal(t, 10000-task.Quota, user.Quota, "reservation must happen exactly once")
	var token model.Token
	require.NoError(t, model.DB.First(&token, 11).Error)
	require.Equal(t, 10000-task.Quota, token.RemainQuota, "token reservation must happen exactly once")
}

func TestAssetRelayTaskPersistsNoSecretsBeforeAcceptance(t *testing.T) {
	restoreDB := useControllerAssetTaskDBForTest(t)
	defer restoreDB()
	restorePricing := useControllerAssetTaskPricingForTest(t)
	defer restorePricing()
	adaptor := &controllerFakeTaskAdaptor{}
	restoreAdaptor := registerTaskAdaptorForTest(constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeBytePlus)), adaptor)
	defer restoreAdaptor()

	publicID := "ast_2234567890abcdefABCDEF1234567890"
	seedControllerRelayUserToken(t, 7, 11, 10000, 10000)
	seedControllerTaskChannel(t, 131, `{"api_key":"sk-provider-secret-scan","signed":"https://signed.example/path?X-Goog-Signature=abc","credentials":{"private_key":"provider-secret-json"}}`)
	seedControllerAsset(t, 7, publicID, time.Now().Add(time.Hour).Unix())
	model.InitChannelCache()

	body := seedanceTaskBody(publicID)
	c, recorder := newControllerRelayTaskContext(body)
	c.Request.Header.Set("Authorization", "Bearer sk-user-authorization-secret")
	common.SetContextKey(c, constant.ContextKeyUserId, 7)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyTokenId, 11)
	common.SetContextKey(c, constant.ContextKeyTokenKey, "sk-task-token-11")
	common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	c.Set("token_name", "task-token")
	c.Set("token_quota", 10000)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{"seedance-2.0": true})
	common.SetContextKey(c, constant.ContextKeyChannelId, 131)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeBytePlus)
	common.SetContextKey(c, constant.ContextKeyChannelKey, `{"api_key":"sk-provider-secret-scan","signed":"https://signed.example/path?X-Goog-Signature=abc","credentials":{"private_key":"provider-secret-json"}}`)
	common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: "wallet_only"})
	var req dto.SeedanceVideoRequest
	require.NoError(t, common.Unmarshal([]byte(body), &req))
	refs, apiErr := service.ResolveAssetReferences(c, 7, &req)
	require.Nil(t, apiErr)
	common.SetContextKey(c, constant.ContextKeyAssetReferenceSet, refs)

	RelayTask(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response dto.OpenAIVideo
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	var task model.Task
	require.NoError(t, model.DB.Where("task_id = ?", response.TaskID).First(&task).Error)
	require.EqualValues(t, model.TaskStatusQueued, task.Status)
	require.Equal(t, model.TaskPreparationStatusPreparingAssets, task.PreparationStatus)
	require.Equal(t, 0, task.ChannelId)
	require.Empty(t, task.PrivateData.UpstreamTaskID)

	privateData, err := common.Marshal(task.PrivateData)
	require.NoError(t, err)
	scan := string(task.NormalizedRequestPayload) + "\n" + string(privateData) + "\n" + string(task.Data)
	for _, forbidden := range []string{
		"Authorization",
		"sk-user-authorization-secret",
		"sk-provider-secret-scan",
		"provider-secret-json",
		"signed.example",
		"X-Goog-Signature",
		"upstream-task-before-accept",
		"gin.Context",
		"ContextKeyChannelKey",
	} {
		require.NotContains(t, scan, forbidden)
	}
}

func TestNonAssetRelayTaskSubmitsSynchronously(t *testing.T) {
	restoreDB := useControllerAssetTaskDBForTest(t)
	defer restoreDB()
	restorePricing := useControllerAssetTaskPricingForTest(t)
	defer restorePricing()
	adaptor := &controllerFakeTaskAdaptor{upstreamTaskID: "upstream-sync"}
	restoreAdaptor := registerTaskAdaptorForTest(constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeBytePlus)), adaptor)
	defer restoreAdaptor()

	seedControllerRelayUserToken(t, 7, 11, 10000, 10000)
	seedControllerTaskChannel(t, 131, "sk-provider-sync")
	model.InitChannelCache()

	c, recorder := newControllerRelayTaskContext(`{"model":"seedance-2.0","content":[{"type":"text","text":"plain prompt"}]}`)
	common.SetContextKey(c, constant.ContextKeyUserId, 7)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyTokenId, 11)
	common.SetContextKey(c, constant.ContextKeyTokenKey, "sk-task-token-11")
	common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	c.Set("token_name", "task-token")
	c.Set("token_quota", 10000)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{"seedance-2.0": true})
	common.SetContextKey(c, constant.ContextKeyChannelId, 131)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeBytePlus)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-provider-sync")
	common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: "wallet_only"})

	RelayTask(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.EqualValues(t, 1, adaptor.providerCalls.Load())
	require.EqualValues(t, 2, adaptor.validateCalls.Load(), "sync path validates in preflight and execution")
	var response dto.OpenAIVideo
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "upstream-sync", response.ID)
	require.Equal(t, dto.VideoStatusQueued, response.Status)

	var queuedCount int64
	require.NoError(t, model.DB.Model(&model.Task{}).Where("status = ? AND preparation_status = ?", model.TaskStatusQueued, model.TaskPreparationStatusPreparingAssets).Count(&queuedCount).Error)
	require.Zero(t, queuedCount)
	var submitted model.Task
	require.NoError(t, model.DB.Where("json_extract(private_data, '$.upstream_task_id') = ?", "upstream-sync").First(&submitted).Error)
	require.NotEqual(t, model.TaskStatusQueued, submitted.Status)
	require.NotEqual(t, model.TaskPreparationStatusPreparingAssets, submitted.PreparationStatus)
	require.Equal(t, 131, submitted.ChannelId)
	require.Equal(t, "upstream-sync", submitted.PrivateData.UpstreamTaskID)
}

func TestAssetTaskWorkerFallbackBeforeAcceptancePinsWinningChannel(t *testing.T) {
	restoreDB := useControllerAssetTaskDBForTest(t)
	defer restoreDB()
	restorePricing := useControllerAssetTaskPricingForTest(t)
	defer restorePricing()
	restoreHooks := useAssetTaskWorkerHooksForTest(t, 100, func() int64 { return assetTaskWorkerTestNow })
	defer restoreHooks()
	oldRetryTimes := common.RetryTimes
	common.RetryTimes = 1
	defer func() { common.RetryTimes = oldRetryTimes }()

	var materializerCalls atomic.Int32
	restoreMaterializer := service.RegisterAssetMaterializer(constant.ChannelTypeBytePlus, controllerAssetMaterializerWithCounter{calls: &materializerCalls})
	defer restoreMaterializer()
	adaptor := &controllerFakeTaskAdaptor{
		upstreamTaskID:    "upstream-channel-b",
		failHTTPByChannel: map[int]int{131: http.StatusInternalServerError},
	}
	restoreAdaptor := registerTaskAdaptorForTest(constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeBytePlus)), adaptor)
	defer restoreAdaptor()

	publicID := "ast_3234567890abcdefABCDEF1234567890"
	seedControllerRelayUserToken(t, 7, 11, 10000, 10000)
	seedControllerTaskChannelWithPriority(t, 131, "sk-provider-a", 100, 1)
	seedControllerTaskChannelWithPriority(t, 132, "sk-provider-b", 90, 1)
	seedControllerAsset(t, 7, publicID, time.Now().Add(time.Hour).Unix())
	task := seedControllerQueuedAssetTask(t, "task_worker_fallback", model.TaskPreparationStatusPreparingAssets, "", 0)
	task.ChannelId = 0
	task.NormalizedRequestPayload = []byte(seedanceTaskBody(publicID))
	require.NoError(t, model.DB.Save(task).Error)
	model.InitChannelCache()

	assetTaskWorkerTestNow = 1000
	processed, err := RunAssetTaskWorkerOnce(context.Background(), "node-a", 10)
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	require.Equal(t, []int{131, 132}, adaptor.channelsSeen)
	require.EqualValues(t, 2, adaptor.providerCalls.Load())
	require.EqualValues(t, 2, materializerCalls.Load(), "each attempted channel may prepare its own binding before acceptance")
	var stored model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_worker_fallback").First(&stored).Error)
	require.EqualValues(t, model.TaskStatusSubmitted, stored.Status)
	require.Equal(t, 132, stored.ChannelId)
	require.Equal(t, "upstream-channel-b", stored.PrivateData.UpstreamTaskID)
	require.Contains(t, string(stored.Data), "upstream-channel-b")
	require.NotContains(t, string(stored.Data), "upstream-channel-a")

	var asset model.Asset
	require.NoError(t, model.DB.Where("public_id = ?", publicID).First(&asset).Error)
	winnerLastUsed := asset.LastUsedAt
	winnerExpires := asset.SourceExpiresAt

	assetTaskWorkerTestNow = 1100
	lateWon, err := model.MarkQueuedTaskAccepted("task_worker_fallback", "node-late", 1200, 1100, 1100, 131, constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeBytePlus)), stored.Quota, "late-upstream-a", []byte(`{"id":"late-upstream-a"}`), []string{publicID}, 1100, winnerExpires+1000)
	require.NoError(t, err)
	require.False(t, lateWon)
	var afterLate model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_worker_fallback").First(&afterLate).Error)
	require.Equal(t, 132, afterLate.ChannelId)
	require.Equal(t, "upstream-channel-b", afterLate.PrivateData.UpstreamTaskID)
	var assetAfterLate model.Asset
	require.NoError(t, model.DB.Where("public_id = ?", publicID).First(&assetAfterLate).Error)
	require.Equal(t, winnerLastUsed, assetAfterLate.LastUsedAt)
	require.Equal(t, winnerExpires, assetAfterLate.SourceExpiresAt)
}

func TestAssetTaskWorkerAcceptedWinnerSettlesAndLogsOnce(t *testing.T) {
	restoreDB := useControllerAssetTaskDBForTest(t)
	defer restoreDB()
	restorePricing := useControllerAssetTaskPricingForTest(t)
	defer restorePricing()
	restoreHooks := useAssetTaskWorkerHooksForTest(t, 100, func() int64 { return assetTaskWorkerTestNow })
	defer restoreHooks()

	var materializerCalls atomic.Int32
	restoreMaterializer := service.RegisterAssetMaterializer(constant.ChannelTypeBytePlus, controllerAssetMaterializerWithCounter{calls: &materializerCalls})
	defer restoreMaterializer()
	adaptor := &controllerFakeTaskAdaptor{
		upstreamTaskID: "upstream-accounted",
		adjustedRatios: map[string]float64{"actual": 2},
	}
	restoreAdaptor := registerTaskAdaptorForTest(constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeBytePlus)), adaptor)
	defer restoreAdaptor()

	publicID := "ast_4234567890abcdefABCDEF1234567890"
	seedControllerRelayUserToken(t, 7, 11, 10000, 10000)
	seedControllerTaskChannel(t, 131, "sk-provider-accounted")
	seedControllerAsset(t, 7, publicID, time.Now().Add(time.Hour).Unix())
	task := seedControllerQueuedAssetTask(t, "task_worker_accounting", model.TaskPreparationStatusPreparingAssets, "", 0)
	task.ChannelId = 0
	task.Quota = 123
	task.NormalizedRequestPayload = []byte(seedanceTaskBody(publicID))
	require.NoError(t, model.DB.Save(task).Error)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 7).Update("quota", 10000-task.Quota).Error)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", 11).Updates(map[string]any{
		"remain_quota": 10000 - task.Quota,
		"used_quota":   task.Quota,
	}).Error)
	model.InitChannelCache()

	assetTaskWorkerTestNow = 1000
	processed, err := RunAssetTaskWorkerOnce(context.Background(), "node-a", 10)
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	var stored model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_worker_accounting").First(&stored).Error)
	require.EqualValues(t, model.TaskStatusSubmitted, stored.Status)
	require.Equal(t, 246, stored.Quota)
	require.Equal(t, 131, stored.ChannelId)
	require.Equal(t, "upstream-accounted", stored.PrivateData.UpstreamTaskID)

	var user model.User
	require.NoError(t, model.DB.First(&user, 7).Error)
	require.Equal(t, 10000-246, user.Quota)
	require.EqualValues(t, 246, user.UsedQuota)
	require.Equal(t, 1, user.RequestCount)
	var token model.Token
	require.NoError(t, model.DB.First(&token, 11).Error)
	require.Equal(t, 10000-246, token.RemainQuota)
	require.Equal(t, 246, token.UsedQuota)
	var channel model.Channel
	require.NoError(t, model.DB.First(&channel, 131).Error)
	require.EqualValues(t, 246, channel.UsedQuota)
	var consumeLogs int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("type = ?", model.LogTypeConsume).Count(&consumeLogs).Error)
	require.EqualValues(t, 1, consumeLogs)

	err = acceptAssetTask(nil, nil, task, "node-late", 1200, &model.Channel{Id: 132}, &relay.TaskSubmitResult{
		UpstreamTaskID: "late-upstream",
		TaskData:       []byte(`{"id":"late-upstream"}`),
		Platform:       constant.TaskPlatform("107"),
		Quota:          999,
	})
	require.Error(t, err)
	require.NoError(t, model.DB.First(&user, 7).Error)
	require.Equal(t, 10000-246, user.Quota)
	require.EqualValues(t, 246, user.UsedQuota)
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("type = ?", model.LogTypeConsume).Count(&consumeLogs).Error)
	require.EqualValues(t, 1, consumeLogs, "late worker must not settle or log after losing acceptance CAS")
}

func TestAssetTaskWorkerAcceptedSubscriptionUsesSnapshotForSettlement(t *testing.T) {
	restoreDB := useControllerAssetTaskDBForTest(t)
	defer restoreDB()
	restorePricing := useControllerAssetTaskPricingForTest(t)
	defer restorePricing()
	restoreHooks := useAssetTaskWorkerHooksForTest(t, 100, func() int64 { return assetTaskWorkerTestNow })
	defer restoreHooks()

	var materializerCalls atomic.Int32
	restoreMaterializer := service.RegisterAssetMaterializer(constant.ChannelTypeBytePlus, controllerAssetMaterializerWithCounter{calls: &materializerCalls})
	defer restoreMaterializer()
	adaptor := &controllerFakeTaskAdaptor{
		upstreamTaskID: "upstream-sub-accounted",
		adjustedRatios: map[string]float64{"actual": 2},
	}
	restoreAdaptor := registerTaskAdaptorForTest(constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeBytePlus)), adaptor)
	defer restoreAdaptor()

	publicID := "ast_6234567890abcdefABCDEF1234567890"
	const userID = 17
	const tokenID = 18
	const subID = 19
	seedControllerRelayUserToken(t, userID, tokenID, 0, 10000)
	seedControllerTaskChannel(t, 131, "sk-provider-sub-accounted")
	seedControllerAsset(t, userID, publicID, time.Now().Add(time.Hour).Unix())
	require.NoError(t, model.DB.Create(&model.UserSubscription{
		Id:          subID,
		UserId:      userID,
		AmountTotal: 100000,
		AmountUsed:  150,
		Status:      "active",
		StartTime:   time.Now().Add(-time.Hour).Unix(),
		EndTime:     time.Now().Add(time.Hour).Unix(),
	}).Error)
	task := seedControllerQueuedAssetTask(t, "task_worker_subscription_accounting", model.TaskPreparationStatusPreparingAssets, "", 0)
	task.UserId = userID
	task.ChannelId = 0
	task.Quota = 100
	task.NormalizedRequestPayload = []byte(seedanceTaskBody(publicID))
	task.PrivateData = model.TaskPrivateData{
		BillingSource:  service.BillingSourceSubscription,
		SubscriptionId: subID,
		TokenId:        tokenID,
		BillingContext: &model.TaskBillingContext{
			OriginModelName:      "seedance-2.0",
			SubscriptionWeight:   1.5,
			SubscriptionWindow:   &model.TaskSubscriptionWindow{SubId: subID, SubStart: time.Now().Add(-time.Hour).Unix()},
			ModelPrice:           0.001,
			GroupRatio:           1,
			GroupModelRatioGroup: "",
		},
	}
	require.NoError(t, model.DB.Save(task).Error)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", tokenID).Updates(map[string]any{
		"remain_quota": 9900,
		"used_quota":   100,
	}).Error)
	model.InitChannelCache()

	assetTaskWorkerTestNow = 1000
	processed, err := RunAssetTaskWorkerOnce(context.Background(), "node-a", 10)
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	var stored model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_worker_subscription_accounting").First(&stored).Error)
	require.EqualValues(t, model.TaskStatusSubmitted, stored.Status)
	require.Equal(t, 200, stored.Quota)
	require.Equal(t, "upstream-sub-accounted", stored.PrivateData.UpstreamTaskID)
	require.Equal(t, int64(300), getControllerSubscriptionUsed(t, subID), "accepted settlement must use persisted subscription weight")
	var token model.Token
	require.NoError(t, model.DB.First(&token, tokenID).Error)
	require.Equal(t, 9800, token.RemainQuota)
	require.Equal(t, 200, token.UsedQuota)
	var consumeLogs int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("type = ?", model.LogTypeConsume).Count(&consumeLogs).Error)
	require.EqualValues(t, 1, consumeLogs)
}

func TestAssetTaskQueuePersistsSubscriptionBillingSnapshot(t *testing.T) {
	restoreDB := useControllerAssetTaskDBForTest(t)
	defer restoreDB()
	restorePricing := useControllerAssetTaskPricingForTest(t)
	defer restorePricing()
	restoreRedis := useControllerAssetTaskRedisForTest(t)
	defer restoreRedis()
	originalWeights := setting.SubscriptionModelWeights2JSONString()
	require.NoError(t, setting.UpdateSubscriptionModelWeightsByJSONString(`{"seedance-2.0":1.5}`))
	defer func() { require.NoError(t, setting.UpdateSubscriptionModelWeightsByJSONString(originalWeights)) }()

	const userID = 77
	const tokenID = 88
	const subID = 99
	window5h := int64(1000)
	windowWeek := int64(5000)
	now := time.Now().Unix()
	seedControllerRelayUserToken(t, userID, tokenID, 0, 10000)
	require.NoError(t, model.DB.Create(&model.SubscriptionPlan{
		Id:               501,
		Title:            "Task Snapshot Plan",
		DurationUnit:     "month",
		DurationValue:    1,
		TotalAmount:      100000,
		Window5hAmount:   window5h,
		WindowWeekAmount: windowWeek,
	}).Error)
	require.NoError(t, model.DB.Create(&model.UserSubscription{
		Id:               subID,
		UserId:           userID,
		PlanId:           501,
		AmountTotal:      100000,
		AmountUsed:       0,
		Window5hAmount:   &window5h,
		WindowWeekAmount: &windowWeek,
		Status:           "active",
		StartTime:        now - 60,
		EndTime:          now + 3600,
	}).Error)

	c := newControllerAssetTaskContext(seedanceTaskBody("ast_5234567890abcdefABCDEF1234567890"))
	common.SetContextKey(c, constant.ContextKeyUserId, userID)
	common.SetContextKey(c, constant.ContextKeyTokenId, tokenID)
	common.SetContextKey(c, constant.ContextKeyTokenKey, "sk-task-token-88")
	common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{})
	info := &relaycommon.RelayInfo{
		UserId:          userID,
		TokenId:         tokenID,
		TokenKey:        "sk-task-token-88",
		UsingGroup:      "default",
		TokenGroup:      "default",
		OriginModelName: "seedance-2.0",
		RequestId:       "asset-subscription-snapshot",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{Action: constant.TaskActionGenerate, PublicTaskID: "task_subscription_snapshot"},
		PriceData: types.PriceData{
			Quota:      100,
			ModelPrice: 0.001,
		},
	}
	apiErr := service.PreConsumeBilling(c, 100, info)
	require.Nil(t, apiErr)
	require.Equal(t, service.BillingSourceSubscription, info.BillingSource)

	video, taskErr := queueAssetTaskForPreparation(c, info, &relay.TaskPreflightResult{Platform: constant.TaskPlatform("107"), Quota: 100})
	require.Nil(t, taskErr)
	require.Equal(t, "task_subscription_snapshot", video.TaskID)

	var task model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_subscription_snapshot").First(&task).Error)
	require.NotNil(t, task.PrivateData.BillingContext)
	require.Equal(t, service.BillingSourceSubscription, task.PrivateData.BillingSource)
	require.Equal(t, subID, task.PrivateData.SubscriptionId)
	require.InDelta(t, 1.5, task.PrivateData.BillingContext.SubscriptionWeight, 0.0001)
	require.NotNil(t, task.PrivateData.BillingContext.SubscriptionWindow)
	require.Equal(t, subID, task.PrivateData.BillingContext.SubscriptionWindow.SubId)
	require.Equal(t, window5h, task.PrivateData.BillingContext.SubscriptionWindow.Limit5h)
	require.Equal(t, windowWeek, task.PrivateData.BillingContext.SubscriptionWindow.LimitWeek)

	beforeRefund := getControllerSubscriptionUsed(t, subID)
	service.RefundTaskQuota(context.Background(), &task, "restart refund")
	require.Equal(t, beforeRefund-150, getControllerSubscriptionUsed(t, subID), "refund must use persisted non-1 subscription weight after restart")
}

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

func TestAssetTaskPreparationStaleLeaseTakeoverAndLateResultFenced(t *testing.T) {
	restoreDB := useControllerAssetTaskDBForTest(t)
	defer restoreDB()
	restoreHooks := useAssetTaskWorkerHooksForTest(t, 100, func() int64 { return assetTaskWorkerTestNow })
	defer restoreHooks()

	task := seedControllerQueuedAssetTask(t, "task_stale_takeover", model.TaskPreparationStatusPreparingAssets, "", 0)
	calls := make([]string, 0, 2)
	runLeasedAssetTaskFunc = func(ctx context.Context, taskID string, owner string, leaseExpiresAt int64) error {
		calls = append(calls, owner)
		if owner == "node-a" {
			return nil
		}
		return acceptAssetTask(nil, nil, task, owner, leaseExpiresAt, &model.Channel{Id: 132}, &relay.TaskSubmitResult{
			UpstreamTaskID: "upstream-node-b",
			TaskData:       []byte(`{"id":"upstream-node-b"}`),
			Platform:       constant.TaskPlatform("107"),
			Quota:          123,
		})
	}

	assetTaskWorkerTestNow = 1000
	processed, err := RunAssetTaskWorkerOnce(context.Background(), "node-a", 10)
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	assetTaskWorkerTestNow = 1050
	processed, err = RunAssetTaskWorkerOnce(context.Background(), "node-b", 10)
	require.NoError(t, err)
	require.Equal(t, 0, processed, "fresh foreign lease must not be scanned for takeover")

	assetTaskWorkerTestNow = 1101
	processed, err = RunAssetTaskWorkerOnce(context.Background(), "node-b", 10)
	require.NoError(t, err)
	require.Equal(t, 1, processed, "expired foreign lease must be claimable by another node")
	require.Equal(t, []string{"node-a", "node-b"}, calls)

	lateWon, err := model.MarkQueuedTaskAccepted("task_stale_takeover", "node-a", 1100, 1102, 1102, 131, constant.TaskPlatform("107"), 123, "late-upstream", []byte(`{"id":"late"}`), nil, 1102, 1200)
	require.NoError(t, err)
	require.False(t, lateWon, "old worker result must be fenced after takeover acceptance")

	var stored model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_stale_takeover").First(&stored).Error)
	require.EqualValues(t, model.TaskStatusSubmitted, stored.Status)
	require.Equal(t, 132, stored.ChannelId)
	require.Equal(t, "upstream-node-b", stored.PrivateData.UpstreamTaskID)
}

func TestAssetTaskAcceptedWinnerExtendsRetentionAndLateWorkerDoesNot(t *testing.T) {
	restoreDB := useControllerAssetTaskDBForTest(t)
	defer restoreDB()
	restoreHooks := useAssetTaskWorkerHooksForTest(t, 100, func() int64 { return assetTaskWorkerTestNow })
	defer restoreHooks()

	publicID := "ast_1234567890abcdefABCDEF1234567890"
	task := seedControllerQueuedAssetTask(t, "task_retention", model.TaskPreparationStatusPreparing, "node-a", 2000)
	seedControllerAsset(t, 7, publicID, 100)
	task.NormalizedRequestPayload = []byte(`{"content":[{"type":"image_url","image_url":{"url":"asset://` + publicID + `"}}]}`)
	require.NoError(t, model.DB.Save(task).Error)

	assetTaskWorkerTestNow = 1900
	err := acceptAssetTask(nil, nil, task, "node-a", 2000, &model.Channel{Id: 131}, &relay.TaskSubmitResult{
		UpstreamTaskID: "upstream-accepted",
		TaskData:       []byte(`{"id":"upstream-accepted"}`),
		Platform:       constant.TaskPlatform("107"),
		Quota:          123,
	})
	require.NoError(t, err)

	var accepted model.Asset
	require.NoError(t, model.DB.Where("public_id = ?", publicID).First(&accepted).Error)
	require.GreaterOrEqual(t, accepted.LastUsedAt, int64(1))
	require.Greater(t, accepted.SourceExpiresAt, int64(100))
	winnerLastUsed := accepted.LastUsedAt
	winnerExpires := accepted.SourceExpiresAt

	lateWon, err := model.MarkQueuedTaskAccepted("task_retention", "node-b", 3000, winnerLastUsed+10, winnerLastUsed+10, 132, constant.TaskPlatform("107"), 123, "late-upstream", []byte(`{"id":"late"}`), []string{publicID}, winnerLastUsed+10, winnerExpires+1000)
	require.NoError(t, err)
	require.False(t, lateWon)

	var afterLate model.Asset
	require.NoError(t, model.DB.Where("public_id = ?", publicID).First(&afterLate).Error)
	require.Equal(t, winnerLastUsed, afterLate.LastUsedAt)
	require.Equal(t, winnerExpires, afterLate.SourceExpiresAt)
}

func TestAssetTaskWorkerPreparationFailureRefundsOnceForLateResult(t *testing.T) {
	restoreDB := useControllerAssetTaskDBForTest(t)
	defer restoreDB()
	restoreHooks := useAssetTaskWorkerHooksForTest(t, 100, func() int64 { return assetTaskWorkerTestNow })
	defer restoreHooks()

	require.NoError(t, model.DB.Create(&model.User{Id: 7, Username: "task_worker_refund", Quota: 1000, Status: common.UserStatusEnabled, AffCode: "task_worker_refund"}).Error)
	require.NoError(t, model.DB.Create(&model.Token{Id: 11, UserId: 7, Key: "sk-task-worker-refund", Name: "task-worker-refund", Status: common.TokenStatusEnabled, RemainQuota: 500}).Error)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 131, Name: "task-worker-refund", Key: "sk-channel", Status: common.ChannelStatusEnabled}).Error)

	task := seedControllerQueuedAssetTask(t, "task_worker_refund_once", model.TaskPreparationStatusPreparingAssets, "", 0)
	runLeasedAssetTaskFunc = func(ctx context.Context, taskID string, owner string, leaseExpiresAt int64) error {
		stored, ok, err := model.GetByOnlyTaskId(taskID)
		require.NoError(t, err)
		require.True(t, ok)
		return failAssetTaskPreparation(ctx, stored, owner, leaseExpiresAt, assertErr("materialize failed"))
	}

	assetTaskWorkerTestNow = 1000
	processed, err := RunAssetTaskWorkerOnce(context.Background(), "node-a", 10)
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	lateErr := failAssetTaskPreparation(context.Background(), task, "node-b", 1100, assertErr("late materialize failed"))
	require.Error(t, lateErr)

	var user model.User
	require.NoError(t, model.DB.Where("id = ?", 7).First(&user).Error)
	require.Equal(t, 1123, user.Quota)
	var token model.Token
	require.NoError(t, model.DB.Where("id = ?", 11).First(&token).Error)
	require.Equal(t, 623, token.RemainQuota)
	var refundLogs int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("type = ?", model.LogTypeRefund).Count(&refundLogs).Error)
	require.EqualValues(t, 1, refundLogs)
}

type assertErr string

func (e assertErr) Error() string {
	return string(e)
}

func newControllerAssetTaskContext(body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func newControllerRelayTaskContext(body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func useControllerAssetTaskDBForTest(t *testing.T) func() {
	t.Helper()
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldCommonKeyCol := controllerAssetTaskModelCommonKeyCol
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.Asset{}, &model.AssetBinding{}, &model.Ability{}, &model.User{}, &model.Token{}, &model.Channel{}, &model.Log{}, &model.SubscriptionPlan{}, &model.UserSubscription{}, &model.UserSubscriptionContract{}, &model.SubscriptionPreConsumeRecord{}, &model.SubscriptionDiscountAccount{}, &model.SubscriptionDiscountEntry{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	common.MemoryCacheEnabled = true
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	controllerAssetTaskModelCommonKeyCol = "`key`"
	return func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		controllerAssetTaskModelCommonKeyCol = oldCommonKeyCol
		model.InitChannelCache()
		_ = sqlDB.Close()
	}
}

func useControllerAssetTaskRedisForTest(t *testing.T) func() {
	t.Helper()
	mr := miniredis.RunT(t)
	oldRDB := common.RDB
	oldRedisEnabled := common.RedisEnabled
	common.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	common.RedisEnabled = true
	return func() {
		_ = common.RDB.Close()
		common.RDB = oldRDB
		common.RedisEnabled = oldRedisEnabled
		mr.Close()
	}
}

func useControllerAssetTaskPricingForTest(t *testing.T) func() {
	t.Helper()
	originalModelRatio := ratio_setting.ModelRatio2JSONString()
	originalModelPrice := ratio_setting.ModelPrice2JSONString()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{}`))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"seedance-2.0":0.001}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	return func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalModelRatio))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(originalModelPrice))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
	}
}

var assetTaskWorkerTestNow int64

func useAssetTaskWorkerHooksForTest(t *testing.T, leaseSeconds int64, now func() int64) func() {
	t.Helper()
	oldRunner := runLeasedAssetTaskFunc
	oldNow := assetTaskWorkerNowUnix
	oldLease := assetTaskPreparationLeaseSeconds
	assetTaskWorkerNowUnix = now
	assetTaskPreparationLeaseSeconds = leaseSeconds
	return func() {
		runLeasedAssetTaskFunc = oldRunner
		assetTaskWorkerNowUnix = oldNow
		assetTaskPreparationLeaseSeconds = oldLease
	}
}

func seedControllerQueuedAssetTask(t *testing.T, taskID string, prepStatus string, leaseOwner string, leaseExpiresAt int64) *model.Task {
	t.Helper()
	task := &model.Task{
		TaskID:                    taskID,
		UserId:                    7,
		Group:                     "default",
		ChannelId:                 131,
		Platform:                  constant.TaskPlatform("107"),
		Quota:                     123,
		Action:                    constant.TaskActionGenerate,
		Status:                    model.TaskStatusQueued,
		PreparationStatus:         prepStatus,
		PreparationLeaseOwner:     leaseOwner,
		PreparationLeaseExpiresAt: leaseExpiresAt,
		NormalizedRequestPayload:  []byte(`{"model":"seedance-2.0","content":[]}`),
		Data:                      []byte(`{}`),
		PrivateData: model.TaskPrivateData{
			BillingSource: service.BillingSourceWallet,
			TokenId:       11,
			BillingContext: &model.TaskBillingContext{
				OriginModelName: "seedance-2.0",
			},
		},
		Properties: model.Properties{OriginModelName: "seedance-2.0"},
	}
	require.NoError(t, model.DB.Create(task).Error)
	return task
}

func seedControllerAsset(t *testing.T, userID int, publicID string, sourceExpiresAt int64) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.Asset{
		PublicId:        publicID,
		UserId:          userID,
		AssetType:       "Image",
		Status:          model.AssetStatusActive,
		SourceStatus:    model.AssetSourceStatusAvailable,
		StorageBackend:  "gcs",
		StorageBucket:   "bucket",
		ObjectKey:       "assets/" + publicID,
		SourceExpiresAt: sourceExpiresAt,
		CreatedAt:       1,
		UpdatedAt:       1,
	}).Error)
}

func seedControllerRelayUserToken(t *testing.T, userID int, tokenID int, userQuota int, tokenQuota int) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{Id: userID, Username: fmt.Sprintf("task-user-%d", userID), Quota: userQuota, Status: common.UserStatusEnabled, AffCode: fmt.Sprintf("task-user-%d", userID)}).Error)
	require.NoError(t, model.DB.Create(&model.Token{Id: tokenID, UserId: userID, Key: fmt.Sprintf("sk-task-token-%d", tokenID), Name: "task-token", Status: common.TokenStatusEnabled, RemainQuota: tokenQuota}).Error)
}

func getControllerSubscriptionUsed(t *testing.T, subscriptionID int) int64 {
	t.Helper()
	var sub model.UserSubscription
	require.NoError(t, model.DB.Select("amount_used").Where("id = ?", subscriptionID).First(&sub).Error)
	return sub.AmountUsed
}

func seedControllerTaskChannel(t *testing.T, id int, key string) {
	t.Helper()
	seedControllerTaskChannelWithPriority(t, id, key, 100, 1)
}

func seedControllerTaskChannelWithPriority(t *testing.T, id int, key string, priority int64, weight uint) {
	t.Helper()
	channel := &model.Channel{
		Id:       id,
		Type:     constant.ChannelTypeBytePlus,
		Key:      key,
		Status:   common.ChannelStatusEnabled,
		Name:     fmt.Sprintf("byteplus-task-%d", id),
		Group:    "default",
		Models:   "seedance-2.0",
		Priority: &priority,
		Weight:   &weight,
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group:     "default",
		Model:     "seedance-2.0",
		ChannelId: id,
		Enabled:   true,
		Priority:  &priority,
		Weight:    weight,
	}).Error)
}

func seedanceTaskBody(publicID string) string {
	return fmt.Sprintf(`{"model":"seedance-2.0","content":[{"type":"image_url","image_url":{"url":"asset://%s"},"role":"reference_image"}]}`, publicID)
}

type controllerAssetMaterializerWithCounter struct {
	calls *atomic.Int32
}

func (m controllerAssetMaterializerWithCounter) CreateAsset(ctx context.Context, input service.AssetMaterializeInput) (service.AssetMaterializeResult, error) {
	if m.calls != nil {
		m.calls.Add(1)
	}
	return controllerAssetMaterializer{}.CreateAsset(ctx, input)
}

func (m controllerAssetMaterializerWithCounter) GetAsset(ctx context.Context, input service.AssetMaterializeInput, upstreamAssetID string) (service.AssetMaterializeResult, error) {
	return controllerAssetMaterializer{}.GetAsset(ctx, input, upstreamAssetID)
}

type controllerFakeTaskAdaptor struct {
	validateCalls     atomic.Int32
	estimateCalls     atomic.Int32
	providerCalls     atomic.Int32
	upstreamTaskID    string
	adjustedRatios    map[string]float64
	failByChannel     map[int]error
	failHTTPByChannel map[int]int
	channelsSeen      []int
}

var _ channel.TaskAdaptor = (*controllerFakeTaskAdaptor)(nil)

func (a *controllerFakeTaskAdaptor) Init(info *relaycommon.RelayInfo) {}

func (a *controllerFakeTaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	a.validateCalls.Add(1)
	var req dto.SeedanceVideoRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	info.Action = constant.TaskActionGenerate
	info.OriginModelName = req.Model
	return nil
}

func (a *controllerFakeTaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	a.estimateCalls.Add(1)
	return nil
}

func (a *controllerFakeTaskAdaptor) AdjustBillingOnSubmit(info *relaycommon.RelayInfo, taskData []byte) map[string]float64 {
	return a.adjustedRatios
}

func (a *controllerFakeTaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	return 0
}

func (a *controllerFakeTaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return "https://provider.example/tasks", nil
}

func (a *controllerFakeTaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	return nil
}

func (a *controllerFakeTaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, err
	}
	return storage, nil
}

func (a *controllerFakeTaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	a.providerCalls.Add(1)
	channelID := common.GetContextKeyInt(c, constant.ContextKeyChannelId)
	a.channelsSeen = append(a.channelsSeen, channelID)
	if status := a.failHTTPByChannel[channelID]; status > 0 {
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(`{"error":"retryable upstream failure"}`)),
			Header:     make(http.Header),
		}, nil
	}
	if err := a.failByChannel[channelID]; err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"accepted":true}`)),
		Header:     make(http.Header),
	}, nil
}

func (a *controllerFakeTaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, *dto.TaskError) {
	taskID := a.upstreamTaskID
	if taskID == "" {
		taskID = fmt.Sprintf("upstream-channel-%d", common.GetContextKeyInt(c, constant.ContextKeyChannelId))
	}
	video := dto.NewOpenAIVideo()
	video.ID = taskID
	video.TaskID = taskID
	video.Model = info.OriginModelName
	video.CreatedAt = time.Now().Unix()
	c.JSON(http.StatusOK, video)
	data, err := common.Marshal(video)
	if err != nil {
		return "", nil, service.TaskErrorWrapperLocal(err, "marshal_response_failed", http.StatusInternalServerError)
	}
	return taskID, data, nil
}

func (a *controllerFakeTaskAdaptor) GetModelList() []string {
	return []string{"seedance-2.0"}
}

func (a *controllerFakeTaskAdaptor) GetChannelName() string {
	return "fake-byteplus"
}

func (a *controllerFakeTaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	return nil, nil
}

func (a *controllerFakeTaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	return nil, nil
}
