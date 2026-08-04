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
