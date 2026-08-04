package controller

import (
	"context"
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
		return acceptAssetTask(task, owner, leaseExpiresAt, &model.Channel{Id: 132}, &relay.TaskSubmitResult{
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
	err := acceptAssetTask(task, "node-a", 2000, &model.Channel{Id: 131}, &relay.TaskSubmitResult{
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

func useControllerAssetTaskDBForTest(t *testing.T) func() {
	t.Helper()
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.Asset{}, &model.User{}, &model.Token{}, &model.Channel{}, &model.Log{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	return func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		_ = sqlDB.Close()
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
		SourceExpiresAt: sourceExpiresAt,
		CreatedAt:       1,
		UpdatedAt:       1,
	}).Error)
}
