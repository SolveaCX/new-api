package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func setupPlaygroundAttachmentServiceDB(t *testing.T) {
	t.Helper()
	newAssetServiceTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.PlaygroundRecord{}, &model.PlaygroundRecordAsset{}))
}

func createPlaygroundServiceAsset(t *testing.T, userID int, publicID, assetType string) *model.Asset {
	t.Helper()
	now := assetNow()
	asset := &model.Asset{
		PublicId:         publicID,
		UserId:           userID,
		AssetType:        assetType,
		Status:           model.AssetStatusActive,
		SourceStatus:     model.AssetSourceStatusAvailable,
		StorageBackend:   defaultAssetStorageBackend,
		StorageBucket:    "asset-test-bucket",
		ObjectKey:        "playground/" + publicID,
		ObjectGeneration: 7,
		ContentType:      map[string]string{model.AssetTypeImage: "image/png", model.AssetTypeVideo: "video/mp4"}[assetType],
		SizeBytes:        1,
		SourceExpiresAt:  now.Add(time.Hour).Unix(),
		CreatedAt:        now.Unix(),
		UpdatedAt:        now.Unix(),
	}
	require.NoError(t, model.DB.Create(asset).Error)
	return asset
}

func TestGetPlaygroundAttachmentPreviewEnforcesOwnerAndSourceState(t *testing.T) {
	setupPlaygroundAttachmentServiceDB(t)
	installAssetServiceTestDeps(t)
	t.Setenv("ASSET_SERVICE_ACCOUNT_EMAIL", "asset-signer@example.iam.gserviceaccount.com")
	asset := createPlaygroundServiceAsset(t, 411, "ast_playground_preview", model.AssetTypeImage)

	preview, err := GetPlaygroundAttachmentPreview(context.Background(), 411, asset.PublicId)
	require.NoError(t, err)
	require.Equal(t, asset.PublicId, preview.AssetID)
	require.Equal(t, model.AssetTypeImage, preview.AssetType)
	require.Equal(t, "image/png", preview.ContentType)
	require.NotEmpty(t, preview.PreviewURL)
	require.LessOrEqual(t, preview.ExpiresAt, asset.SourceExpiresAt)

	_, err = GetPlaygroundAttachmentPreview(context.Background(), 412, asset.PublicId)
	require.ErrorIs(t, err, ErrAssetUploadNotFound)

	require.NoError(t, model.DB.Model(&model.Asset{}).Where("id = ?", asset.Id).Updates(map[string]any{
		"source_status":     model.AssetSourceStatusExpired,
		"source_expires_at": assetNow().Add(-time.Minute).Unix(),
	}).Error)
	_, err = GetPlaygroundAttachmentPreview(context.Background(), 411, asset.PublicId)
	require.ErrorIs(t, err, ErrAssetExpired)
}

func TestCompletePlaygroundAttachmentUploadIsIdempotentAfterAssetActivation(t *testing.T) {
	setupPlaygroundAttachmentServiceDB(t)
	store := installAssetServiceTestDeps(t)
	t.Setenv("ASSET_SERVICE_ACCOUNT_EMAIL", "asset-signer@example.iam.gserviceaccount.com")
	png := tinyPNG()
	session, err := CreateAssetUploadSession(context.Background(), AssetUploadSessionRequest{
		UserID: 415, Owner: "user-415", AssetType: model.AssetTypeImage,
		ContentType: "image/png", SizeBytes: int64(len(png)),
	})
	require.NoError(t, err)
	store.objects["asset-test-bucket/"+session.ObjectKey] = png
	store.attrs["asset-test-bucket/"+session.ObjectKey] = AssetObjectAttrs{
		ContentType: "image/png", Size: int64(len(png)), Generation: 12,
	}

	request := AssetCompleteUploadRequest{UploadID: session.UploadID, Owner: "user-415", UserID: 415}
	first, err := CompletePlaygroundAttachmentUpload(context.Background(), request)
	require.NoError(t, err)
	second, err := CompletePlaygroundAttachmentUpload(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, first.AssetID, second.AssetID)
	require.Equal(t, first.ContentType, second.ContentType)
	// The retry recovers the activated row and only signs a fresh preview; it
	// must not re-open or re-validate the object as a new completion.
	require.Len(t, store.opens, 1)
}

func TestCleanupPlaygroundAssetsDeletesOnlyUnreferencedSources(t *testing.T) {
	setupPlaygroundAttachmentServiceDB(t)
	store := installAssetServiceTestDeps(t)
	orphan := createPlaygroundServiceAsset(t, 413, "ast_playground_orphan", model.AssetTypeImage)
	referenced := createPlaygroundServiceAsset(t, 413, "ast_playground_referenced", model.AssetTypeImage)
	bound := createPlaygroundServiceAsset(t, 413, "ast_playground_bound", model.AssetTypeVideo)
	store.objects[orphan.StorageBucket+"/"+orphan.ObjectKey] = []byte("orphan")
	store.objects[referenced.StorageBucket+"/"+referenced.ObjectKey] = []byte("referenced")
	store.objects[bound.StorageBucket+"/"+bound.ObjectKey] = []byte("bound")
	require.NoError(t, model.DB.Create(&model.PlaygroundRecordAsset{
		UserID: 413, ConversationID: "conversation", RecordID: "record", AssetID: referenced.PublicId,
	}).Error)
	require.NoError(t, model.DB.Create(&model.AssetBinding{
		AssetId: bound.Id, ChannelId: 1, Status: model.AssetBindingStatusPending,
	}).Error)

	result, err := CleanupPlaygroundAssets(context.Background(), 413, []string{orphan.PublicId, referenced.PublicId, bound.PublicId})
	require.NoError(t, err)
	require.Equal(t, 1, result.Claimed)
	require.Equal(t, 1, result.Deleted)
	require.Len(t, store.deletes, 1)
	require.Equal(t, orphan.StorageBucket+"/"+orphan.ObjectKey, store.deletes[0].key)

	var stored model.Asset
	require.NoError(t, model.DB.Where("public_id = ?", orphan.PublicId).First(&stored).Error)
	require.Equal(t, model.AssetSourceStatusExpired, stored.SourceStatus)
	require.Empty(t, stored.ObjectKey)
	stored = model.Asset{}
	require.NoError(t, model.DB.Where("public_id = ?", referenced.PublicId).First(&stored).Error)
	require.Equal(t, model.AssetSourceStatusAvailable, stored.SourceStatus)
	stored = model.Asset{}
	require.NoError(t, model.DB.Where("public_id = ?", bound.PublicId).First(&stored).Error)
	require.Equal(t, model.AssetSourceStatusAvailable, stored.SourceStatus)
}

func TestCleanupPlaygroundAssetsLeavesCleanupPendingOnStorageFailure(t *testing.T) {
	setupPlaygroundAttachmentServiceDB(t)
	store := installAssetServiceTestDeps(t)
	asset := createPlaygroundServiceAsset(t, 414, "ast_playground_delete_failure", model.AssetTypeImage)
	store.deleteErr = errFakeAssetStore

	result, err := CleanupPlaygroundAssets(context.Background(), 414, []string{asset.PublicId})
	require.NoError(t, err)
	require.Equal(t, 1, result.Claimed)
	require.Zero(t, result.Deleted)
	var stored model.Asset
	require.NoError(t, model.DB.Where("public_id = ?", asset.PublicId).First(&stored).Error)
	require.Equal(t, model.AssetSourceStatusCleanupPending, stored.SourceStatus)
	require.NotEmpty(t, stored.ObjectKey)
}
