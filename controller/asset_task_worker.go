package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

const (
	assetTaskWorkerLeaseSeconds = 120
	assetTaskWorkerBatchSize    = 10
)

var (
	assetTaskWorkerNowUnix           = func() int64 { return time.Now().Unix() }
	assetTaskPreparationLeaseSeconds = int64(assetTaskWorkerLeaseSeconds)
	runLeasedAssetTaskFunc           = runLeasedAssetTask
)

type AssetTaskWorkerConfig struct {
	Owner    string
	Interval time.Duration
	Limit    int
}

func hasQueuedAssetReferences(c *gin.Context) bool {
	references, ok := common.GetContextKeyType[service.AssetReferenceSet](c, constant.ContextKeyAssetReferenceSet)
	return ok && references.HasReferences()
}

func queueAssetTaskForPreparation(c *gin.Context, info *relaycommon.RelayInfo, preflight *relay.TaskPreflightResult) (*dto.OpenAIVideo, *dto.TaskError) {
	if info == nil || info.TaskRelayInfo == nil || preflight == nil {
		return nil, service.TaskErrorWrapperLocal(fmt.Errorf("missing task preflight"), "task_preflight_required", http.StatusInternalServerError)
	}
	payload, err := normalizedTaskPayload(c)
	if err != nil {
		return nil, service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	now := time.Now().Unix()
	taskID := info.PublicTaskID
	if strings.TrimSpace(taskID) == "" {
		taskID = model.GenerateTaskID()
		info.PublicTaskID = taskID
	}
	video := dto.NewOpenAIVideo()
	video.ID = taskID
	video.TaskID = taskID
	video.Model = info.OriginModelName
	video.Progress = 0
	video.CreatedAt = now
	responseData, err := common.Marshal(video)
	if err != nil {
		return nil, service.TaskErrorWrapper(err, "marshal_task_response_failed", http.StatusInternalServerError)
	}
	task := &model.Task{
		TaskID:                    taskID,
		UserId:                    info.UserId,
		Group:                     info.UsingGroup,
		ChannelId:                 common.GetContextKeyInt(c, constant.ContextKeyChannelId),
		Platform:                  preflight.Platform,
		Quota:                     preflight.Quota,
		Action:                    info.Action,
		Status:                    model.TaskStatusQueued,
		PreparationStatus:         model.TaskPreparationStatusPreparingAssets,
		NormalizedRequestPayload:  payload,
		Data:                      responseData,
		Progress:                  "0%",
		CreatedAt:                 now,
		UpdatedAt:                 now,
		PreparationLeaseExpiresAt: 0,
		Properties: model.Properties{
			OriginModelName:   info.OriginModelName,
			UpstreamModelName: taskUpstreamModelName(info),
		},
		PrivateData: model.TaskPrivateData{
			BillingSource:  info.BillingSource,
			SubscriptionId: info.SubscriptionId,
			TokenId:        info.TokenId,
			BillingContext: taskBillingContextSnapshot(info),
		},
	}
	if err := task.Insert(); err != nil {
		return nil, service.TaskErrorWrapper(err, "queue_task_failed", http.StatusInternalServerError)
	}
	return video, nil
}

func normalizedTaskPayload(c *gin.Context) ([]byte, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, err
	}
	raw, err := storage.Bytes()
	if err != nil {
		return nil, err
	}
	var payload any
	if err := common.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return common.Marshal(payload)
}

func taskUpstreamModelName(info *relaycommon.RelayInfo) string {
	if info == nil || info.ChannelMeta == nil {
		return ""
	}
	return info.UpstreamModelName
}

func taskBillingContextSnapshot(info *relaycommon.RelayInfo) *model.TaskBillingContext {
	return &model.TaskBillingContext{
		ModelPrice:           info.PriceData.ModelPrice,
		GroupRatio:           info.PriceData.GroupRatioInfo.GroupRatio,
		GroupModelRatio:      info.PriceData.GroupRatioInfo.GroupModelRatio,
		GroupModelRatioGroup: info.PriceData.GroupRatioInfo.GroupModelRatioGroup,
		GroupModelRatioModel: info.PriceData.GroupRatioInfo.GroupModelRatioModel,
		ModelRatio:           info.PriceData.ModelRatio,
		OtherRatios:          info.PriceData.OtherRatios,
		OriginModelName:      info.OriginModelName,
		PerCallBilling:       common.StringsContains(constant.TaskPricePatches, info.OriginModelName) || info.PriceData.UsePrice,
	}
}

func StartAssetTaskWorker() {
	StartAssetTaskWorkerWithConfig(context.Background(), AssetTaskWorkerConfig{})
}

func StartAssetTaskWorkerWithConfig(ctx context.Context, cfg AssetTaskWorkerConfig) {
	owner := assetTaskWorkerOwner(cfg.Owner)
	interval := cfg.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	limit := cfg.Limit
	if limit <= 0 {
		limit = assetTaskWorkerBatchSize
	}
	gopool.Go(func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if _, err := RunAssetTaskWorkerOnce(ctx, owner, limit); err != nil {
				common.SysError("asset task worker error: " + err.Error())
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})
}

func assetTaskWorkerOwner(configured string) string {
	owner := strings.TrimSpace(configured)
	if owner == "" {
		owner = strings.TrimSpace(os.Getenv("ASSET_TASK_WORKER_OWNER"))
	}
	if owner == "" {
		owner, _ = os.Hostname()
	}
	if owner == "" {
		owner = fmt.Sprintf("asset-worker-%d", time.Now().UnixNano())
	}
	return owner
}

func RunAssetTaskWorkerOnce(ctx context.Context, owner string, limit int) (int, error) {
	now := assetTaskWorkerNowUnix()
	tasks, err := model.GetQueuedAssetPreparationTasks(now, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, task := range tasks {
		now = assetTaskWorkerNowUnix()
		leaseExpiresAt := now + assetTaskPreparationLeaseSeconds
		won, err := model.ClaimTaskPreparationLease(task.TaskID, owner, now, leaseExpiresAt)
		if err != nil {
			return processed, err
		}
		if !won {
			continue
		}
		processed++
		if err := runLeasedAssetTaskFunc(ctx, task.TaskID, owner, leaseExpiresAt); err != nil {
			logger.LogError(ctx, fmt.Sprintf("asset task %s preparation failed: %s", task.TaskID, err.Error()))
		}
	}
	return processed, nil
}

func runLeasedAssetTask(ctx context.Context, taskID string, owner string, leaseExpiresAt int64) error {
	task, ok, err := model.GetByOnlyTaskId(taskID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("task not found")
	}
	c, info, err := rebuildAssetTaskContext(task)
	if err != nil {
		return failAssetTaskPreparation(ctx, task, owner, leaseExpiresAt, err)
	}
	retryParam := &service.RetryParam{
		Ctx:        c,
		TokenGroup: task.Group,
		ModelName:  task.Properties.OriginModelName,
		Retry:      common.GetPointer(0),
	}
	var lastErr error
	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		channel, channelErr := getChannel(c, info, retryParam)
		if channelErr != nil {
			lastErr = channelErr.Err
			break
		}
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			releaseChannelConcurrencyForRequest(c)
			lastErr = bodyErr
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)
		preflight := &relay.TaskPreflightResult{Platform: task.Platform, Quota: task.Quota}
		result, taskErr := relay.ExecutePreparedTaskSubmit(c, info, preflight)
		releaseChannelConcurrencyForRequest(c)
		if taskErr == nil {
			return acceptAssetTask(task, owner, leaseExpiresAt, channel, result)
		}
		lastErr = taskErr.Error
		if !shouldRetryTaskRelay(c, channel.Id, taskErr, common.RetryTimes-retryParam.GetRetry()) {
			break
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("asset preparation failed")
	}
	return failAssetTaskPreparation(ctx, task, owner, leaseExpiresAt, lastErr)
}

func rebuildAssetTaskContext(task *model.Task) (*gin.Context, *relaycommon.RelayInfo, error) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptestRequestFromPayload(task.NormalizedRequestPayload)
	common.SetContextKey(c, constant.ContextKeyUserId, task.UserId)
	common.SetContextKey(c, constant.ContextKeyUserGroup, task.Group)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, task.Group)
	common.SetContextKey(c, constant.ContextKeyAssetMaterializeEnabled, true)
	var seedanceReq dto.SeedanceVideoRequest
	if err := common.Unmarshal(task.NormalizedRequestPayload, &seedanceReq); err != nil {
		return nil, nil, err
	}
	refs, apiErr := service.ResolveAssetReferences(c, task.UserId, &seedanceReq)
	if apiErr != nil {
		return nil, nil, apiErr.Err
	}
	common.SetContextKey(c, constant.ContextKeyAssetReferenceSet, refs)
	info := &relaycommon.RelayInfo{
		UserId:          task.UserId,
		TokenId:         task.PrivateData.TokenId,
		UsingGroup:      task.Group,
		TokenGroup:      task.Group,
		OriginModelName: task.Properties.OriginModelName,
		BillingSource:   task.PrivateData.BillingSource,
		SubscriptionId:  task.PrivateData.SubscriptionId,
		ChannelMeta:     &relaycommon.ChannelMeta{},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			Action:       task.Action,
			PublicTaskID: task.TaskID,
		},
		PriceData: types.PriceData{Quota: task.Quota},
	}
	return c, info, nil
}

func httptestRequestFromPayload(payload []byte) *http.Request {
	req, _ := http.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func acceptAssetTask(task *model.Task, owner string, leaseExpiresAt int64, channel *model.Channel, result *relay.TaskSubmitResult) error {
	now := assetTaskWorkerNowUnix()
	publicIDs := extractStrictAssetPublicIDsFromPayload(task.NormalizedRequestPayload)
	retentionUntil := now + int64(service.CurrentAssetStorageConfig().SourceRetention.Seconds())
	won, err := model.MarkQueuedTaskAccepted(task.TaskID, owner, leaseExpiresAt, now, now, channel.Id, result.Platform, result.Quota, result.UpstreamTaskID, result.TaskData, publicIDs, now, retentionUntil)
	if err != nil {
		return err
	}
	if !won {
		return fmt.Errorf("task preparation lease lost")
	}
	return nil
}

func extractStrictAssetPublicIDsFromPayload(payload []byte) []string {
	var req dto.SeedanceVideoRequest
	if err := common.Unmarshal(payload, &req); err != nil {
		return nil
	}
	seen := make(map[string]struct{})
	ids := make([]string, 0)
	add := func(raw string) {
		if !service.IsStrictBytePlusAssetURI(raw) || !strings.HasPrefix(raw, "asset://") {
			return
		}
		publicID := strings.TrimPrefix(raw, "asset://")
		if _, ok := seen[publicID]; ok {
			return
		}
		seen[publicID] = struct{}{}
		ids = append(ids, publicID)
	}
	for _, item := range req.Content {
		if item.ImageURL != nil {
			add(item.ImageURL.URL)
		}
		if item.VideoURL != nil {
			add(item.VideoURL.URL)
		}
		if item.AudioURL != nil {
			add(item.AudioURL.URL)
		}
	}
	return ids
}

func failAssetTaskPreparation(ctx context.Context, task *model.Task, owner string, leaseExpiresAt int64, cause error) error {
	now := assetTaskWorkerNowUnix()
	reason := "asset preparation failed"
	if cause != nil && strings.TrimSpace(cause.Error()) != "" {
		reason = cause.Error()
	}
	won, err := model.MarkQueuedTaskFailed(task.TaskID, owner, leaseExpiresAt, reason, now)
	if err != nil {
		return err
	}
	if won {
		service.RefundTaskQuota(ctx, task, reason)
	}
	return cause
}
