package controller

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCreatePlaygroundAttachmentUploadUsesAuthenticatedOwner(t *testing.T) {
	original := createPlaygroundAttachmentUploadSession
	t.Cleanup(func() { createPlaygroundAttachmentUploadSession = original })

	var got service.AssetUploadSessionRequest
	createPlaygroundAttachmentUploadSession = func(_ context.Context, request service.AssetUploadSessionRequest) (*service.AssetUploadSessionResult, error) {
		got = request
		return &service.AssetUploadSessionResult{
			UploadID:      "upl_playground",
			PublicID:      "ast_playground",
			SignedURL:     "https://signed.example/upload",
			UploadHeaders: map[string]string{"Content-Type": "image/png"},
			ExpiresAt:     1785682501,
		}, nil
	}

	c, recorder := newAssetJSONContext(http.MethodPost, "/api/playground/attachments/uploads", `{"asset_type":"Image","content_type":"image/png","size_bytes":17}`)
	setAssetTokenContext(c, 501)
	CreatePlaygroundAttachmentUpload(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, service.AssetUploadSessionRequest{
		UserID: 501, Owner: "user-501", AssetType: "Image", ContentType: "image/png", SizeBytes: 17,
	}, got)
	var response struct {
		Success bool                           `json:"success"`
		Data    dto.AssetUploadSessionResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, "ast_playground", response.Data.AssetID)
	require.Equal(t, "asset.upload", response.Data.Object)
}

func TestPlaygroundAttachmentCompletionAndPreviewForwardAuthenticatedUser(t *testing.T) {
	originalComplete := completePlaygroundAttachmentUpload
	originalPreview := getPlaygroundAttachmentPreview
	t.Cleanup(func() {
		completePlaygroundAttachmentUpload = originalComplete
		getPlaygroundAttachmentPreview = originalPreview
	})

	var completeRequest service.AssetCompleteUploadRequest
	completePlaygroundAttachmentUpload = func(_ context.Context, request service.AssetCompleteUploadRequest) (*service.PlaygroundAttachmentPreviewResult, error) {
		completeRequest = request
		return &service.PlaygroundAttachmentPreviewResult{
			AssetID: "ast_complete", AssetType: model.AssetTypeVideo, ContentType: "video/mp4",
			SizeBytes: 42, PreviewURL: "https://signed.example/video", ExpiresAt: 1785682501,
		}, nil
	}

	complete, completeRecorder := newAssetJSONContext(http.MethodPost, "/api/playground/attachments/uploads/upl_complete/complete", "")
	complete.Params = ginParams("upload_id", "upl_complete")
	setAssetTokenContext(complete, 502)
	CompletePlaygroundAttachmentUpload(complete)
	require.Equal(t, http.StatusOK, completeRecorder.Code, completeRecorder.Body.String())
	require.Equal(t, service.AssetCompleteUploadRequest{UploadID: "upl_complete", Owner: "user-502", UserID: 502}, completeRequest)
	require.Contains(t, completeRecorder.Body.String(), `"asset_id":"ast_complete"`)

	var previewUserID int
	var previewAssetID string
	getPlaygroundAttachmentPreview = func(_ context.Context, userID int, assetID string) (*service.PlaygroundAttachmentPreviewResult, error) {
		previewUserID, previewAssetID = userID, assetID
		return &service.PlaygroundAttachmentPreviewResult{
			AssetID: assetID, AssetType: model.AssetTypeImage, ContentType: "image/png",
			SizeBytes: 3, PreviewURL: "https://signed.example/image", ExpiresAt: 1785682501,
		}, nil
	}
	preview, previewRecorder := newAssetJSONContext(http.MethodGet, "/api/playground/attachments/ast_preview/preview", "")
	preview.Params = ginParams("asset_id", "ast_preview")
	setAssetTokenContext(preview, 503)
	GetPlaygroundAttachmentPreview(preview)
	require.Equal(t, http.StatusOK, previewRecorder.Code, previewRecorder.Body.String())
	require.Equal(t, 503, previewUserID)
	require.Equal(t, "ast_preview", previewAssetID)
}

func TestClearPlaygroundRecordSchedulesAssetCleanupAfterCommit(t *testing.T) {
	setupPlaygroundControllerDB(t, 504)
	now := time.Now().Unix()
	asset := model.Asset{
		PublicId: "ast_clear_cleanup", UserId: 504, AssetType: model.AssetTypeImage,
		Status: model.AssetStatusActive, SourceStatus: model.AssetSourceStatusAvailable,
		StorageBackend: "gcs", StorageBucket: "bucket", ObjectKey: "object.png", ObjectGeneration: 1,
		ContentType: "image/png", SizeBytes: 1, SourceExpiresAt: now + 3600, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, model.DB.Create(&asset).Error)
	record := &model.PlaygroundRecord{
		UserID: 504, RecordID: "record-cleanup", RecordType: model.PlaygroundRecordTypeTurn,
		ConversationID: playgroundConversationID, Status: model.PlaygroundStatusComplete,
		MessagesSnapshot: `[{"key":"user"}]`, ClientCompletedAt: 1000,
		AssetReferences: []model.PlaygroundAssetReference{{AssetID: asset.PublicId, AssetType: model.AssetTypeImage}},
	}
	require.NoError(t, model.SavePlaygroundRecord(record))

	originalScheduler := schedulePlaygroundAttachmentCleanup
	t.Cleanup(func() { schedulePlaygroundAttachmentCleanup = originalScheduler })
	var scheduledUserID int
	var scheduledIDs []string
	schedulePlaygroundAttachmentCleanup = func(userID int, ids []string) {
		scheduledUserID = userID
		scheduledIDs = append([]string(nil), ids...)
	}

	clear, recorder := playgroundRecordTestContext(t, 504, http.MethodPost, `{"record_id":"550e8400-e29b-41d4-a716-446655440002","conversation_id":"550e8400-e29b-41d4-a716-446655440001","client_completed_at":2000}`)
	ClearPlaygroundRecord(clear)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 504, scheduledUserID)
	require.Equal(t, []string{asset.PublicId}, scheduledIDs)
}

func ginParams(key, value string) gin.Params {
	return gin.Params{{Key: key, Value: value}}
}
