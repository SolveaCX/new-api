package model

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	AssetBindingStatusPending = "PENDING"
	AssetBindingStatusLeased  = "LEASED"

	AssetUploadStatusPending  = "PENDING"
	AssetUploadStatusComplete = "COMPLETE"
	AssetUploadStatusExpired  = "EXPIRED"
)

type Asset struct {
	Id              int64  `json:"id" gorm:"primaryKey"`
	PublicId        string `json:"public_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId          int    `json:"user_id" gorm:"index"`
	AssetType       string `json:"asset_type" gorm:"type:varchar(16);index"`
	Status          string `json:"status" gorm:"type:varchar(24);index"`
	SourceStatus    string `json:"source_status" gorm:"type:varchar(24);index"`
	StorageBackend  string `json:"storage_backend" gorm:"type:varchar(16)"`
	StorageBucket   string `json:"storage_bucket" gorm:"type:varchar(255)"`
	ObjectKey       string `json:"object_key" gorm:"type:varchar(512)"`
	ContentType     string `json:"content_type" gorm:"type:varchar(128)"`
	SizeBytes       int64  `json:"size_bytes"`
	SHA256          string `json:"sha256" gorm:"type:varchar(64)"`
	LastUsedAt      int64  `json:"last_used_at" gorm:"index"`
	SourceExpiresAt int64  `json:"source_expires_at" gorm:"index"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

type AssetBinding struct {
	Id              int64  `json:"id" gorm:"primaryKey"`
	AssetId         int64  `json:"asset_id" gorm:"uniqueIndex:idx_asset_binding_asset_channel"`
	ChannelId       int    `json:"channel_id" gorm:"uniqueIndex:idx_asset_binding_asset_channel;index"`
	UpstreamGroupId string `json:"-" gorm:"type:varchar(191)"`
	UpstreamAssetId string `json:"-" gorm:"type:varchar(191)"`
	Status          string `json:"status" gorm:"type:varchar(24);index"`
	ErrorCode       string `json:"error_code" gorm:"type:varchar(64)"`
	AttemptCount    int    `json:"attempt_count"`
	LeaseOwner      string `json:"-" gorm:"type:varchar(64);index"`
	LeaseExpiresAt  int64  `json:"-" gorm:"index"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

type AssetUpload struct {
	Id             int64  `json:"id" gorm:"primaryKey"`
	UploadId       string `json:"upload_id" gorm:"type:varchar(64);uniqueIndex"`
	Owner          string `json:"-" gorm:"type:varchar(64);index"`
	UserId         int    `json:"user_id" gorm:"index"`
	AssetId        int64  `json:"asset_id" gorm:"index"`
	PublicId       string `json:"public_id" gorm:"type:varchar(64);index"`
	AssetType      string `json:"asset_type" gorm:"type:varchar(16);index"`
	StorageBackend string `json:"storage_backend" gorm:"type:varchar(16)"`
	StorageBucket  string `json:"storage_bucket" gorm:"type:varchar(255)"`
	ObjectKey      string `json:"object_key" gorm:"type:varchar(512)"`
	ContentType    string `json:"content_type" gorm:"type:varchar(128)"`
	SizeBytes      int64  `json:"size_bytes"`
	SHA256         string `json:"sha256" gorm:"type:varchar(64)"`
	ExpiresAt      int64  `json:"expires_at" gorm:"index"`
	Status         string `json:"status" gorm:"type:varchar(24);index"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

func CreateAssetBindingIfAbsent(assetID int64, channelID int, now int64) (*AssetBinding, bool, error) {
	binding := &AssetBinding{
		AssetId:   assetID,
		ChannelId: channelID,
		Status:    AssetBindingStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	insert := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(binding)
	if insert.Error != nil {
		return nil, false, insert.Error
	}
	if insert.RowsAffected == 1 {
		return binding, true, nil
	}
	var stored AssetBinding
	if err := DB.Where("asset_id = ? AND channel_id = ?", assetID, channelID).First(&stored).Error; err != nil {
		return nil, false, err
	}
	return &stored, false, nil
}

func ClaimAssetBindingLease(assetID int64, channelID int, owner string, now int64, leaseExpiresAt int64) (bool, error) {
	result := DB.Model(&AssetBinding{}).
		Where("asset_id = ? AND channel_id = ?", assetID, channelID).
		Where("lease_owner = ? OR lease_expires_at <= ?", owner, now).
		Updates(map[string]any{
			"status":           AssetBindingStatusLeased,
			"lease_owner":      owner,
			"lease_expires_at": leaseExpiresAt,
			"attempt_count":    gorm.Expr("attempt_count + ?", 1),
			"updated_at":       now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func GetAssetUploadForOwner(uploadID string, owner string) (*AssetUpload, error) {
	var upload AssetUpload
	if err := DB.Where("upload_id = ? AND owner = ?", uploadID, owner).First(&upload).Error; err != nil {
		return nil, err
	}
	return &upload, nil
}
