package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func setupPlaygroundAssetRelationTestDB(t *testing.T, userID int) {
	t.Helper()
	db := setupPlaygroundRecordTestDB(t)
	require.NoError(t, db.AutoMigrate(&User{}, &Asset{}, &AssetBinding{}, &PlaygroundRecord{}, &PlaygroundRecordAsset{}))
	require.NoError(t, db.Create(&User{Id: userID, Username: "playground-assets", AffCode: "playground-assets-aff"}).Error)
}

func playgroundAssetForTest(userID int, publicID, assetType string) Asset {
	now := time.Now().Unix()
	return Asset{
		PublicId:         publicID,
		UserId:           userID,
		AssetType:        assetType,
		Status:           AssetStatusActive,
		SourceStatus:     AssetSourceStatusAvailable,
		StorageBackend:   "gcs",
		StorageBucket:    "playground-test",
		ObjectKey:        "objects/" + publicID,
		ObjectGeneration: 1,
		ContentType:      map[string]string{AssetTypeImage: "image/png", AssetTypeVideo: "video/mp4", AssetTypeAudio: "audio/wav", AssetTypeDocument: "application/pdf"}[assetType],
		SizeBytes:        1,
		SourceExpiresAt:  now + 3600,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func playgroundTurnWithAssets(userID int, recordID, conversationID string, completedAt int64, refs []PlaygroundAssetReference) *PlaygroundRecord {
	return &PlaygroundRecord{
		UserID:            userID,
		RecordID:          recordID,
		RecordType:        PlaygroundRecordTypeTurn,
		ConversationID:    conversationID,
		Status:            PlaygroundStatusComplete,
		OutputText:        "output",
		MessagesSnapshot:  `[{"key":"assistant"}]`,
		ClientCompletedAt: completedAt,
		AssetReferences:   refs,
	}
}

func TestValidatePlaygroundAssetReferencesScopesTypeAndExpiry(t *testing.T) {
	setupPlaygroundAssetRelationTestDB(t, 401)
	image := playgroundAssetForTest(401, "ast_playground_image", AssetTypeImage)
	video := playgroundAssetForTest(401, "ast_playground_video", AssetTypeVideo)
	foreign := playgroundAssetForTest(402, "ast_playground_foreign", AssetTypeImage)
	require.NoError(t, DB.Create(&image).Error)
	require.NoError(t, DB.Create(&video).Error)
	require.NoError(t, DB.Create(&foreign).Error)

	require.NoError(t, ValidatePlaygroundAssetReferences(401, []PlaygroundAssetReference{
		{AssetID: image.PublicId, AssetType: AssetTypeImage},
		{AssetID: image.PublicId, AssetType: AssetTypeImage},
		{AssetID: video.PublicId, AssetType: AssetTypeVideo},
	}))
	require.ErrorIs(t, ValidatePlaygroundAssetReferences(401, []PlaygroundAssetReference{{AssetID: foreign.PublicId, AssetType: AssetTypeImage}}), ErrPlaygroundAssetNotFound)
	require.ErrorIs(t, ValidatePlaygroundAssetReferences(401, []PlaygroundAssetReference{{AssetID: image.PublicId, AssetType: AssetTypeVideo}}), ErrPlaygroundAssetTypeMismatch)
	require.ErrorIs(t, ValidatePlaygroundAssetReferences(401, []PlaygroundAssetReference{{AssetID: "bad-id", AssetType: AssetTypeImage}}), ErrPlaygroundAssetInvalidID)

	require.NoError(t, DB.Model(&Asset{}).Where("id = ?", image.Id).Updates(map[string]any{"source_expires_at": time.Now().Add(-time.Minute).Unix()}).Error)
	require.ErrorIs(t, ValidatePlaygroundAssetReferences(401, []PlaygroundAssetReference{{AssetID: image.PublicId, AssetType: AssetTypeImage}}), ErrPlaygroundAssetExpired)
}

func TestNormalizePlaygroundAssetReferencesAcceptsDocumentAndPdfTypes(t *testing.T) {
	refs, err := normalizePlaygroundAssetReferences([]PlaygroundAssetReference{
		{AssetID: "ast_playground_document", AssetType: "document"},
		{AssetID: "ast_playground_pdf", AssetType: "pdf"},
	})
	require.NoError(t, err)
	require.Len(t, refs, 2)
	require.Equal(t, "Document", refs[0].AssetType)
	require.Equal(t, "Document", refs[1].AssetType)
}

func TestNormalizePlaygroundAssetReferencesAcceptsAudioTypes(t *testing.T) {
	refs, err := normalizePlaygroundAssetReferences([]PlaygroundAssetReference{
		{AssetID: "ast_playground_audio", AssetType: "audio"},
		{AssetID: "ast_playground_audio_caps", AssetType: "Audio"},
	})
	require.NoError(t, err)
	require.Len(t, refs, 2)
	require.Equal(t, "Audio", refs[0].AssetType)
	require.Equal(t, "Audio", refs[1].AssetType)
}

func TestPlaygroundRecordAssetReferencesAreIdempotentAndClearKeepsSharedAssets(t *testing.T) {
	setupPlaygroundAssetRelationTestDB(t, 403)
	image := playgroundAssetForTest(403, "ast_playground_shared", AssetTypeImage)
	video := playgroundAssetForTest(403, "ast_playground_video2", AssetTypeVideo)
	require.NoError(t, DB.Create(&image).Error)
	require.NoError(t, DB.Create(&video).Error)

	refs := []PlaygroundAssetReference{
		{AssetID: image.PublicId, AssetType: AssetTypeImage},
		{AssetID: image.PublicId, AssetType: AssetTypeImage},
		{AssetID: video.PublicId, AssetType: AssetTypeVideo},
	}
	recordA := playgroundTurnWithAssets(403, "record-a", "conversation-a", 1000, refs)
	recordB := playgroundTurnWithAssets(403, "record-b", "conversation-b", 2000, []PlaygroundAssetReference{{AssetID: image.PublicId, AssetType: AssetTypeImage}})
	require.NoError(t, SavePlaygroundRecord(recordA))
	require.NoError(t, SavePlaygroundRecord(recordA))
	require.NoError(t, SavePlaygroundRecord(recordB))

	var count int64
	require.NoError(t, DB.Model(&PlaygroundRecordAsset{}).Where("record_id = ?", recordA.RecordID).Count(&count).Error)
	require.EqualValues(t, 2, count)

	removed, err := ClearPlaygroundConversationWithAssets(403, "clear-a", "conversation-a", 3000)
	require.NoError(t, err)
	require.Equal(t, []string{image.PublicId, video.PublicId}, removed)
	require.NoError(t, DB.Model(&PlaygroundRecordAsset{}).Where("conversation_id = ?", "conversation-a").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, DB.Model(&PlaygroundRecordAsset{}).Where("asset_id = ?", image.PublicId).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, DB.Model(&PlaygroundRecordAsset{}).Where("asset_id = ?", video.PublicId).Count(&count).Error)
	require.Zero(t, count)
}

func TestClaimPlaygroundAssetsForCleanupSkipsReferencedAndBoundAssets(t *testing.T) {
	setupPlaygroundAssetRelationTestDB(t, 404)
	orphan := playgroundAssetForTest(404, "ast_playground_orphan", AssetTypeImage)
	referenced := playgroundAssetForTest(404, "ast_playground_referenced", AssetTypeImage)
	bound := playgroundAssetForTest(404, "ast_playground_bound", AssetTypeVideo)
	require.NoError(t, DB.Create(&orphan).Error)
	require.NoError(t, DB.Create(&referenced).Error)
	require.NoError(t, DB.Create(&bound).Error)
	require.NoError(t, DB.Create(&PlaygroundRecordAsset{UserID: 404, ConversationID: "conversation", RecordID: "record", AssetID: referenced.PublicId}).Error)
	require.NoError(t, DB.Create(&AssetBinding{AssetId: bound.Id, ChannelId: 1, Status: AssetBindingStatusPending}).Error)

	claimed, err := ClaimPlaygroundAssetsForCleanup(404, []string{orphan.PublicId, referenced.PublicId, bound.PublicId}, "cleanup-test", time.Now().Unix(), time.Now().Add(time.Minute).Unix())
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, orphan.PublicId, claimed[0].PublicId)

	var stored Asset
	require.NoError(t, DB.Where("public_id = ?", orphan.PublicId).First(&stored).Error)
	require.Equal(t, AssetSourceStatusCleanupPending, stored.SourceStatus)
	stored = Asset{}
	require.NoError(t, DB.Where("public_id = ?", referenced.PublicId).First(&stored).Error)
	require.Equal(t, AssetSourceStatusAvailable, stored.SourceStatus)
}

func TestSavePlaygroundRecordRechecksAssetStateInsideRecordTransaction(t *testing.T) {
	setupPlaygroundAssetRelationTestDB(t, 405)
	asset := playgroundAssetForTest(405, "ast_playground_cleanup_pending", AssetTypeImage)
	require.NoError(t, DB.Create(&asset).Error)
	require.NoError(t, DB.Model(&Asset{}).Where("id = ?", asset.Id).Updates(map[string]any{
		"source_status":       AssetSourceStatusCleanupPending,
		"cleanup_lease_owner": "cleanup-worker",
		"cleanup_lease_until": time.Now().Add(time.Minute).Unix(),
	}).Error)

	record := playgroundTurnWithAssets(
		405,
		"record-cleanup-pending",
		"conversation-cleanup-pending",
		4000,
		[]PlaygroundAssetReference{{AssetID: asset.PublicId, AssetType: AssetTypeImage}},
	)
	err := SavePlaygroundRecord(record)
	require.ErrorIs(t, err, ErrPlaygroundAssetExpired)

	var count int64
	require.NoError(t, DB.Model(&PlaygroundRecord{}).Where("record_id = ?", record.RecordID).Count(&count).Error)
	require.Zero(t, count)
}

func TestRetryCannotMoveAttachmentEdgesAcrossConversationIdentity(t *testing.T) {
	setupPlaygroundAssetRelationTestDB(t, 406)
	asset := playgroundAssetForTest(406, "ast_playground_retry_identity", AssetTypeImage)
	require.NoError(t, DB.Create(&asset).Error)

	record := playgroundTurnWithAssets(
		406,
		"record-retry-identity",
		"conversation-original",
		5000,
		[]PlaygroundAssetReference{{AssetID: asset.PublicId, AssetType: AssetTypeImage}},
	)
	require.NoError(t, SavePlaygroundRecord(record))
	retry := playgroundTurnWithAssets(
		406,
		record.RecordID,
		"conversation-mismatched",
		5001,
		[]PlaygroundAssetReference{{AssetID: asset.PublicId, AssetType: AssetTypeImage}},
	)
	require.NoError(t, SavePlaygroundRecord(retry))

	var edge PlaygroundRecordAsset
	require.NoError(t, DB.Where("record_id = ?", record.RecordID).First(&edge).Error)
	require.Equal(t, "conversation-original", edge.ConversationID)
}
