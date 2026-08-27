package controller

import (
	"context"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

const defaultPlaygroundAttachmentCleanupDelay = 5 * time.Minute

var (
	createPlaygroundAttachmentUploadSession = service.CreateAssetUploadSession
	completePlaygroundAttachmentUpload      = service.CompletePlaygroundAttachmentUpload
	getPlaygroundAttachmentPreview          = service.GetPlaygroundAttachmentPreview
	// The clear request is acknowledged before object deletion. A short grace
	// period lets an in-flight retry/regenerate reuse an attachment while the
	// durable record outbox catches up, and avoids making a just-cleared asset
	// nondeterministically unavailable.
	playgroundAttachmentCleanupDelay    = defaultPlaygroundAttachmentCleanupDelay
	schedulePlaygroundAttachmentCleanup = func(userID int, publicIDs []string) {
		if userID <= 0 || len(publicIDs) == 0 {
			return
		}
		ids := append([]string(nil), publicIDs...)
		cleanup := func() {
			gopool.Go(func() {
				if _, err := service.CleanupPlaygroundAssets(context.Background(), userID, ids); err != nil {
					common.SysError("playground attachment cleanup error: " + err.Error())
				}
			})
		}
		if playgroundAttachmentCleanupDelay <= 0 {
			cleanup()
			return
		}
		time.AfterFunc(playgroundAttachmentCleanupDelay, cleanup)
	}
)

func CreatePlaygroundAttachmentUpload(c *gin.Context) {
	var request dto.AssetUploadSessionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	userID := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	result, err := createPlaygroundAttachmentUploadSession(c.Request.Context(), service.AssetUploadSessionRequest{
		UserID:      userID,
		Owner:       assetUploadOwner(userID),
		AssetType:   strings.TrimSpace(request.AssetType),
		ContentType: strings.TrimSpace(request.ContentType),
		SizeBytes:   request.SizeBytes,
	})
	if err != nil {
		writeAssetServiceError(c, err)
		return
	}
	if result == nil {
		writeAssetServiceError(c, service.ErrAssetUploadValidation)
		return
	}
	common.ApiSuccess(c, dto.AssetUploadSessionResponse{
		UploadID:      result.UploadID,
		AssetID:       result.PublicID,
		Object:        "asset.upload",
		Status:        "pending",
		UploadURL:     result.SignedURL,
		UploadHeaders: result.UploadHeaders,
		ExpiresAt:     result.ExpiresAt,
	})
}

func CompletePlaygroundAttachmentUpload(c *gin.Context) {
	userID := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	result, err := completePlaygroundAttachmentUpload(c.Request.Context(), service.AssetCompleteUploadRequest{
		UploadID: strings.TrimSpace(c.Param("upload_id")),
		Owner:    assetUploadOwner(userID),
		UserID:   userID,
	})
	if err != nil {
		writeAssetServiceError(c, err)
		return
	}
	if result == nil {
		writeAssetServiceError(c, service.ErrAssetUploadValidation)
		return
	}
	common.ApiSuccess(c, playgroundAttachmentPreviewResponse(result))
}

func GetPlaygroundAttachmentPreview(c *gin.Context) {
	userID := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	result, err := getPlaygroundAttachmentPreview(
		c.Request.Context(),
		userID,
		strings.TrimSpace(c.Param("asset_id")),
	)
	if err != nil {
		writeAssetServiceError(c, err)
		return
	}
	if result == nil {
		writeAssetServiceError(c, service.ErrAssetUploadNotFound)
		return
	}
	common.ApiSuccess(c, playgroundAttachmentPreviewResponse(result))
}

func playgroundAttachmentPreviewResponse(result *service.PlaygroundAttachmentPreviewResult) dto.PlaygroundAttachmentPreviewResponse {
	return dto.PlaygroundAttachmentPreviewResponse{
		AssetID:     result.AssetID,
		AssetType:   result.AssetType,
		ContentType: result.ContentType,
		SizeBytes:   result.SizeBytes,
		PreviewURL:  result.PreviewURL,
		ExpiresAt:   result.ExpiresAt,
	}
}
