package model

import (
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAssetModelsAutoMigrateAndUniqueness(t *testing.T) {
	db := newAssetTestDB(t, &Asset{}, &AssetBinding{}, &AssetUpload{})

	asset := Asset{
		PublicId:       "asset_public_unique",
		UserId:         10,
		AssetType:      "image",
		Status:         "READY",
		SourceStatus:   "READY",
		StorageBackend: "gcs",
		StorageBucket:  "flatkey-assets",
		ObjectKey:      "users/10/asset_public_unique.png",
		ContentType:    "image/png",
		SizeBytes:      1234,
		SHA256:         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CreatedAt:      100,
		UpdatedAt:      100,
	}
	require.NoError(t, db.Create(&asset).Error)

	duplicatePublicID := asset
	duplicatePublicID.Id = 0
	duplicatePublicID.UserId = 11
	require.Error(t, db.Create(&duplicatePublicID).Error, "public asset IDs must be globally unique")

	binding := AssetBinding{
		AssetId:         asset.Id,
		ChannelId:       131,
		UpstreamGroupId: "group-a",
		UpstreamAssetId: "upstream-a",
		Status:          AssetBindingStatusLeased,
		CreatedAt:       110,
		UpdatedAt:       110,
	}
	require.NoError(t, db.Create(&binding).Error)

	duplicateBinding := binding
	duplicateBinding.Id = 0
	duplicateBinding.UpstreamAssetId = "upstream-b"
	require.Error(t, db.Create(&duplicateBinding).Error, "asset/channel binding must be unique")
}

func TestAssetBindingCreateDoesNothingOnDuplicate(t *testing.T) {
	newAssetTestDB(t, &Asset{}, &AssetBinding{})
	asset := insertAssetForAssetTest(t, "asset_binding_duplicate")

	first, created, err := CreateAssetBindingIfAbsent(asset.Id, 131, 100)
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, asset.Id, first.AssetId)

	second, created, err := CreateAssetBindingIfAbsent(asset.Id, 131, 200)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, first.Id, second.Id)

	var count int64
	require.NoError(t, DB.Model(&AssetBinding{}).Where("asset_id = ? AND channel_id = ?", asset.Id, 131).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestAssetBindingLeaseTakeover(t *testing.T) {
	newAssetTestDB(t, &Asset{}, &AssetBinding{})
	asset := insertAssetForAssetTest(t, "asset_binding_lease")
	_, created, err := CreateAssetBindingIfAbsent(asset.Id, 131, 100)
	require.NoError(t, err)
	require.True(t, created)

	claimed, err := ClaimAssetBindingLease(asset.Id, 131, "node-a", 100, 160)
	require.NoError(t, err)
	require.True(t, claimed)

	claimed, err = ClaimAssetBindingLease(asset.Id, 131, "node-b", 120, 180)
	require.NoError(t, err)
	require.False(t, claimed, "fresh foreign lease must not be stolen")

	claimed, err = ClaimAssetBindingLease(asset.Id, 131, "node-a", 130, 190)
	require.NoError(t, err)
	require.True(t, claimed, "same owner may renew before expiry")

	claimed, err = ClaimAssetBindingLease(asset.Id, 131, "node-b", 191, 250)
	require.NoError(t, err)
	require.True(t, claimed, "expired lease can be taken over")

	var stored AssetBinding
	require.NoError(t, DB.Where("asset_id = ? AND channel_id = ?", asset.Id, 131).First(&stored).Error)
	require.Equal(t, "node-b", stored.LeaseOwner)
	require.EqualValues(t, 250, stored.LeaseExpiresAt)
	require.EqualValues(t, 3, stored.AttemptCount)
}

func TestMigrateLegacyBytePlusAssetsPreservesPublicIDsAndBindingsIdempotently(t *testing.T) {
	newAssetTestDB(t, &Asset{}, &AssetBinding{}, &BytePlusAssetGroup{}, &BytePlusAsset{})

	group := BytePlusAssetGroup{
		UserId:          10,
		ChannelId:       131,
		UpstreamGroupId: "upstream-group",
		Status:          BytePlusAssetGroupStatusActive,
		CreatedTime:     90,
		UpdatedTime:     100,
	}
	require.NoError(t, DB.Create(&group).Error)

	legacy := BytePlusAsset{
		PublicId:        "ast_1234567890abcdefABCDEF1234567890",
		UserId:          10,
		AssetGroupId:    group.Id,
		ChannelId:       131,
		UpstreamAssetId: "upstream-asset",
		AssetType:       "Image",
		Status:          BytePlusAssetStatusActive,
		CreatedTime:     110,
		UpdatedTime:     120,
	}
	require.NoError(t, DB.Create(&legacy).Error)

	require.NoError(t, MigrateLegacyBytePlusAssets())
	require.NoError(t, MigrateLegacyBytePlusAssets())

	var assetCount int64
	require.NoError(t, DB.Model(&Asset{}).Where("public_id = ?", legacy.PublicId).Count(&assetCount).Error)
	require.EqualValues(t, 1, assetCount)

	var asset Asset
	require.NoError(t, DB.Where("public_id = ?", legacy.PublicId).First(&asset).Error)
	require.Equal(t, legacy.PublicId, asset.PublicId)
	require.Equal(t, legacy.UserId, asset.UserId)
	require.Equal(t, legacy.AssetType, asset.AssetType)
	require.Equal(t, legacy.Status, asset.Status)
	require.Equal(t, AssetSourceStatusUnavailable, asset.SourceStatus)
	require.Empty(t, asset.StorageBackend)
	require.Empty(t, asset.StorageBucket)
	require.Empty(t, asset.ObjectKey)
	require.EqualValues(t, legacy.CreatedTime, asset.CreatedAt)
	require.EqualValues(t, legacy.UpdatedTime, asset.UpdatedAt)

	var bindings []AssetBinding
	require.NoError(t, DB.Where("asset_id = ?", asset.Id).Find(&bindings).Error)
	require.Len(t, bindings, 1)
	require.Equal(t, legacy.ChannelId, bindings[0].ChannelId)
	require.Equal(t, group.UpstreamGroupId, bindings[0].UpstreamGroupId)
	require.Equal(t, legacy.UpstreamAssetId, bindings[0].UpstreamAssetId)
	require.Equal(t, legacy.Status, bindings[0].Status)
	require.EqualValues(t, legacy.CreatedTime, bindings[0].CreatedAt)
	require.EqualValues(t, legacy.UpdatedTime, bindings[0].UpdatedAt)
}

func TestMigrateLegacyBytePlusAssetsDoesNotOverwriteExistingGeneralizedSource(t *testing.T) {
	newAssetTestDB(t, &Asset{}, &AssetBinding{}, &BytePlusAssetGroup{}, &BytePlusAsset{})

	group := BytePlusAssetGroup{
		UserId:          10,
		ChannelId:       131,
		UpstreamGroupId: "upstream-group",
		Status:          BytePlusAssetGroupStatusActive,
		CreatedTime:     90,
		UpdatedTime:     100,
	}
	require.NoError(t, DB.Create(&group).Error)

	legacy := BytePlusAsset{
		PublicId:        "ast_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		UserId:          10,
		AssetGroupId:    group.Id,
		ChannelId:       131,
		UpstreamAssetId: "upstream-asset",
		AssetType:       "Image",
		Status:          BytePlusAssetStatusActive,
		CreatedTime:     110,
		UpdatedTime:     120,
	}
	require.NoError(t, DB.Create(&legacy).Error)

	existing := Asset{
		PublicId:       legacy.PublicId,
		UserId:         legacy.UserId,
		AssetType:      legacy.AssetType,
		Status:         AssetStatusActive,
		SourceStatus:   AssetSourceStatusAvailable,
		StorageBackend: "gcs",
		StorageBucket:  "flatkey-assets",
		ObjectKey:      "objects/existing",
		CreatedAt:      1,
		UpdatedAt:      2,
	}
	require.NoError(t, DB.Create(&existing).Error)

	require.NoError(t, MigrateLegacyBytePlusAssets())

	var asset Asset
	require.NoError(t, DB.Where("public_id = ?", legacy.PublicId).First(&asset).Error)
	require.Equal(t, AssetSourceStatusAvailable, asset.SourceStatus)
	require.Equal(t, "gcs", asset.StorageBackend)
	require.Equal(t, "flatkey-assets", asset.StorageBucket)
	require.Equal(t, "objects/existing", asset.ObjectKey)
	require.EqualValues(t, existing.CreatedAt, asset.CreatedAt)
	require.EqualValues(t, existing.UpdatedAt, asset.UpdatedAt)

	var binding AssetBinding
	require.NoError(t, DB.Where("asset_id = ? AND channel_id = ?", asset.Id, legacy.ChannelId).First(&binding).Error)
	require.Equal(t, group.UpstreamGroupId, binding.UpstreamGroupId)
	require.Equal(t, legacy.UpstreamAssetId, binding.UpstreamAssetId)
	require.Equal(t, legacy.Status, binding.Status)
}

func TestAssetUploadOwnership(t *testing.T) {
	newAssetTestDB(t, &AssetUpload{})

	upload := AssetUpload{
		UploadId:       "upload_unique",
		Owner:          "user-10",
		UserId:         10,
		PublicId:       "asset_upload_target",
		AssetType:      "image",
		StorageBackend: "gcs",
		StorageBucket:  "flatkey-assets",
		ObjectKey:      "uploads/upload_unique",
		ContentType:    "image/png",
		SizeBytes:      1024,
		SHA256:         "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ExpiresAt:      500,
		Status:         AssetUploadStatusPending,
		CreatedAt:      100,
		UpdatedAt:      100,
	}
	require.NoError(t, DB.Create(&upload).Error)

	duplicate := upload
	duplicate.Id = 0
	duplicate.Owner = "user-11"
	duplicate.UserId = 11
	require.Error(t, DB.Create(&duplicate).Error, "upload IDs must be globally unique")

	got, err := GetAssetUploadForOwner("upload_unique", "user-10")
	require.NoError(t, err)
	require.Equal(t, "asset_upload_target", got.PublicId)

	_, err = GetAssetUploadForOwner("upload_unique", "user-11")
	require.True(t, errors.Is(err, gorm.ErrRecordNotFound), "different owner must not read the upload session")
}

func TestAssetCreateAvailableAndPendingUploadThenCompleteWithOwnerCAS(t *testing.T) {
	newAssetTestDB(t, &Asset{}, &AssetUpload{})

	asset, err := CreateAssetWithUploadSession(Asset{
		PublicId:       "ast_pending",
		UserId:         10,
		AssetType:      "Image",
		Status:         AssetStatusCreating,
		SourceStatus:   AssetSourceStatusUnavailable,
		StorageBackend: "gcs",
		StorageBucket:  "bucket",
		ObjectKey:      "objects/pending.png",
		ContentType:    "image/png",
		SizeBytes:      123,
		CreatedAt:      100,
		UpdatedAt:      100,
	}, AssetUpload{
		UploadId:    "upl_pending",
		Owner:       "owner-a",
		UserId:      10,
		PublicId:    "ast_pending",
		AssetType:   "Image",
		ContentType: "image/png",
		SizeBytes:   123,
		ExpiresAt:   200,
		Status:      AssetUploadStatusPending,
		CreatedAt:   100,
		UpdatedAt:   100,
	})
	require.NoError(t, err)
	require.NotZero(t, asset.Id)

	completed, err := CompleteAssetUploadCAS("upl_pending", "owner-b", AssetUploadCompletion{
		ContentType:     "image/png",
		SizeBytes:       123,
		SHA256:          "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		SourceExpiresAt: 300,
		Now:             150,
	})
	require.NoError(t, err)
	require.False(t, completed)

	completed, err = CompleteAssetUploadCAS("upl_pending", "owner-a", AssetUploadCompletion{
		ContentType:     "image/png",
		SizeBytes:       123,
		SHA256:          "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		SourceExpiresAt: 300,
		Now:             150,
	})
	require.NoError(t, err)
	require.True(t, completed)

	var stored Asset
	require.NoError(t, DB.First(&stored, asset.Id).Error)
	require.Equal(t, AssetStatusActive, stored.Status)
	require.Equal(t, AssetSourceStatusAvailable, stored.SourceStatus)
	require.Equal(t, "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", stored.SHA256)
	require.EqualValues(t, 300, stored.SourceExpiresAt)

	var upload AssetUpload
	require.NoError(t, DB.First(&upload, "upload_id = ?", "upl_pending").Error)
	require.Equal(t, AssetUploadStatusComplete, upload.Status)
}

func TestAssetUploadCompletionRejectsExpiredAndLoserDoesNotMutateAsset(t *testing.T) {
	newAssetTestDB(t, &Asset{}, &AssetUpload{})
	asset, err := CreateAssetWithUploadSession(Asset{
		PublicId:       "ast_pending_expired",
		UserId:         10,
		AssetType:      "Image",
		Status:         AssetStatusCreating,
		SourceStatus:   AssetSourceStatusUnavailable,
		StorageBackend: "gcs",
		StorageBucket:  "bucket",
		ObjectKey:      "objects/pending.png",
		ContentType:    "image/png",
		SizeBytes:      123,
		CreatedAt:      100,
		UpdatedAt:      100,
	}, AssetUpload{
		UploadId:    "upl_pending_expired",
		Owner:       "owner-a",
		UserId:      10,
		PublicId:    "ast_pending_expired",
		AssetType:   "Image",
		ContentType: "image/png",
		SizeBytes:   123,
		ExpiresAt:   150,
		Status:      AssetUploadStatusPending,
		CreatedAt:   100,
		UpdatedAt:   100,
	})
	require.NoError(t, err)

	completed, err := CompleteAssetUploadCAS("upl_pending_expired", "owner-a", AssetUploadCompletion{
		ContentType:      "image/png",
		SizeBytes:        123,
		SHA256:           "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		ObjectGeneration: 77,
		SourceExpiresAt:  300,
		Now:              150,
	})
	require.NoError(t, err)
	require.False(t, completed)

	var stored Asset
	require.NoError(t, DB.First(&stored, asset.Id).Error)
	require.Equal(t, AssetStatusCreating, stored.Status)
	require.Empty(t, stored.SHA256)
	require.Zero(t, stored.ObjectGeneration)

	var upload AssetUpload
	require.NoError(t, DB.First(&upload, "upload_id = ?", "upl_pending_expired").Error)
	require.Equal(t, AssetUploadStatusExpired, upload.Status)

	require.NoError(t, DB.Model(&AssetUpload{}).Where("upload_id = ?", "upl_pending_expired").Updates(map[string]any{
		"status":     AssetUploadStatusPending,
		"expires_at": int64(250),
	}).Error)
	completed, err = CompleteAssetUploadCAS("upl_pending_expired", "owner-a", AssetUploadCompletion{
		ContentType:      "image/png",
		SizeBytes:        123,
		SHA256:           "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		ObjectGeneration: 88,
		SourceExpiresAt:  400,
		Now:              200,
	})
	require.NoError(t, err)
	require.True(t, completed)
	completed, err = CompleteAssetUploadCAS("upl_pending_expired", "owner-a", AssetUploadCompletion{
		ContentType:      "image/png",
		SizeBytes:        123,
		SHA256:           "9999999999999999999999999999999999999999999999999999999999999999",
		ObjectGeneration: 99,
		SourceExpiresAt:  500,
		Now:              201,
	})
	require.NoError(t, err)
	require.False(t, completed)

	require.NoError(t, DB.First(&stored, asset.Id).Error)
	require.Equal(t, "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", stored.SHA256)
	require.EqualValues(t, 88, stored.ObjectGeneration)
	require.EqualValues(t, 400, stored.SourceExpiresAt)
}

func TestAssetCleanupLeaseClaimAndGenerationFencing(t *testing.T) {
	newAssetTestDB(t, &Asset{}, &AssetBinding{})
	asset := insertAssetForAssetTest(t, "asset_cleanup_lease")
	require.NoError(t, DB.Model(&Asset{}).Where("id = ?", asset.Id).Updates(map[string]any{
		"status":              AssetStatusActive,
		"source_status":       AssetSourceStatusAvailable,
		"source_expires_at":   int64(100),
		"cleanup_lease_owner": "node-stale",
		"cleanup_lease_until": int64(90),
		"cleanup_generation":  int64(2),
	}).Error)

	claimed, err := ClaimExpiredAssetSources("node-a", 150, 210, 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.EqualValues(t, 3, claimed[0].CleanupGeneration)

	ok, err := MarkAssetSourceExpiredIfCleanupLease(claimed[0].Id, "node-b", claimed[0].CleanupGeneration, 200)
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = MarkAssetSourceExpiredIfCleanupLease(claimed[0].Id, "node-a", claimed[0].CleanupGeneration, 200)
	require.NoError(t, err)
	require.True(t, ok)
	var stored Asset
	require.NoError(t, DB.First(&stored, asset.Id).Error)
	require.Equal(t, AssetSourceStatusExpired, stored.SourceStatus)
	require.Equal(t, AssetStatusExpired, stored.Status)
}

func newAssetTestDB(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(models...))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	oldDB := DB
	DB = db
	t.Cleanup(func() {
		DB = oldDB
		_ = sqlDB.Close()
	})
	return db
}

func insertAssetForAssetTest(t *testing.T, publicID string) Asset {
	t.Helper()
	asset := Asset{
		PublicId:       publicID,
		UserId:         10,
		AssetType:      "image",
		Status:         "READY",
		SourceStatus:   "READY",
		StorageBackend: "gcs",
		StorageBucket:  "flatkey-assets",
		ObjectKey:      "objects/" + publicID,
		ContentType:    "image/png",
		SizeBytes:      123,
		SHA256:         "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		CreatedAt:      100,
		UpdatedAt:      100,
	}
	require.NoError(t, DB.Create(&asset).Error)
	return asset
}
