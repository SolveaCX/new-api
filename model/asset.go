package model

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	AssetStatusCreating   = "Creating"
	AssetStatusProcessing = "Processing"
	AssetStatusActive     = "Active"
	AssetStatusFailed     = "Failed"

	AssetSourceStatusAvailable   = "Available"
	AssetSourceStatusUnavailable = "Unavailable"

	AssetBindingStatusPending = "PENDING"
	AssetBindingStatusLeased  = "LEASED"

	AssetUploadStatusPending  = "PENDING"
	AssetUploadStatusComplete = "COMPLETE"
	AssetUploadStatusExpired  = "EXPIRED"
)

const legacyBytePlusAssetMigrationPageSize = 500

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

type AssetWithBinding struct {
	Asset    Asset
	Binding  *AssetBinding
	Bindings []AssetBinding
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

func GetAssetsWithBindingsByPublicIDsForUser(userID int, publicIDs []string) (map[string]AssetWithBinding, error) {
	if len(publicIDs) == 0 || DB == nil || !DB.Migrator().HasTable(&Asset{}) || !DB.Migrator().HasTable(&AssetBinding{}) {
		return nil, nil
	}
	unique := make([]string, 0, len(publicIDs))
	seen := make(map[string]struct{}, len(publicIDs))
	for _, id := range publicIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}

	var assets []Asset
	if err := DB.Where("user_id = ? AND public_id IN ?", userID, unique).Find(&assets).Error; err != nil {
		return nil, err
	}
	if len(assets) == 0 {
		return map[string]AssetWithBinding{}, nil
	}

	assetIDs := make([]int64, 0, len(assets))
	for _, asset := range assets {
		assetIDs = append(assetIDs, asset.Id)
	}
	var bindings []AssetBinding
	if err := DB.Where("asset_id IN ?", assetIDs).Order("asset_id ASC, channel_id ASC, id ASC").Find(&bindings).Error; err != nil {
		return nil, err
	}
	bindingsByAssetID := make(map[int64][]AssetBinding, len(bindings))
	for _, binding := range bindings {
		bindingsByAssetID[binding.AssetId] = append(bindingsByAssetID[binding.AssetId], binding)
	}

	byPublicID := make(map[string]AssetWithBinding, len(assets))
	for _, asset := range assets {
		item := AssetWithBinding{Asset: asset}
		if assetBindings := bindingsByAssetID[asset.Id]; len(assetBindings) > 0 {
			item.Bindings = append([]AssetBinding(nil), assetBindings...)
			bindingCopy := assetBindings[0]
			item.Binding = &bindingCopy
		}
		byPublicID[asset.PublicId] = item
	}
	return byPublicID, nil
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

func MigrateLegacyBytePlusAssets() error {
	if DB == nil ||
		!DB.Migrator().HasTable(&Asset{}) ||
		!DB.Migrator().HasTable(&AssetBinding{}) ||
		!DB.Migrator().HasTable(&BytePlusAssetGroup{}) ||
		!DB.Migrator().HasTable(&BytePlusAsset{}) {
		return nil
	}

	for lastID := int64(0); ; {
		var legacyAssets []BytePlusAsset
		if err := DB.Where("id > ?", lastID).
			Order("id ASC").
			Limit(legacyBytePlusAssetMigrationPageSize).
			Find(&legacyAssets).Error; err != nil {
			return fmt.Errorf("failed to load legacy byteplus assets: %w", err)
		}
		if len(legacyAssets) == 0 {
			break
		}

		groups, err := loadLegacyBytePlusAssetGroups(legacyAssets)
		if err != nil {
			return err
		}
		for _, legacy := range legacyAssets {
			if err := migrateLegacyBytePlusAsset(legacy, groups[legacy.AssetGroupId]); err != nil {
				return err
			}
			lastID = legacy.Id
		}
	}
	return nil
}

func loadLegacyBytePlusAssetGroups(legacyAssets []BytePlusAsset) (map[int64]BytePlusAssetGroup, error) {
	groupIDs := make([]int64, 0, len(legacyAssets))
	seen := make(map[int64]struct{}, len(legacyAssets))
	for _, legacy := range legacyAssets {
		if legacy.AssetGroupId <= 0 {
			continue
		}
		if _, ok := seen[legacy.AssetGroupId]; ok {
			continue
		}
		seen[legacy.AssetGroupId] = struct{}{}
		groupIDs = append(groupIDs, legacy.AssetGroupId)
	}
	if len(groupIDs) == 0 {
		return nil, nil
	}

	var groups []BytePlusAssetGroup
	if err := DB.Where("id IN ?", groupIDs).Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("failed to load legacy byteplus asset groups: %w", err)
	}
	byID := make(map[int64]BytePlusAssetGroup, len(groups))
	for _, group := range groups {
		byID[group.Id] = group
	}
	return byID, nil
}

func migrateLegacyBytePlusAsset(legacy BytePlusAsset, group BytePlusAssetGroup) error {
	insertAsset := Asset{
		PublicId:     legacy.PublicId,
		UserId:       legacy.UserId,
		AssetType:    legacy.AssetType,
		Status:       legacy.Status,
		SourceStatus: AssetSourceStatusUnavailable,
		CreatedAt:    legacy.CreatedTime,
		UpdatedAt:    legacy.UpdatedTime,
	}
	if err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&insertAsset).Error; err != nil {
		return fmt.Errorf("failed to insert generalized asset for legacy byteplus asset %d: %w", legacy.Id, err)
	}

	var asset Asset
	if err := DB.Where("public_id = ?", legacy.PublicId).First(&asset).Error; err != nil {
		return fmt.Errorf("failed to load generalized asset for legacy byteplus asset %d: %w", legacy.Id, err)
	}

	insertBinding := AssetBinding{
		AssetId:         asset.Id,
		ChannelId:       legacy.ChannelId,
		UpstreamGroupId: group.UpstreamGroupId,
		UpstreamAssetId: legacy.UpstreamAssetId,
		Status:          legacy.Status,
		CreatedAt:       legacy.CreatedTime,
		UpdatedAt:       legacy.UpdatedTime,
	}
	if err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&insertBinding).Error; err != nil {
		return fmt.Errorf("failed to insert asset binding for legacy byteplus asset %d: %w", legacy.Id, err)
	}
	return nil
}

func GetAssetUploadForOwner(uploadID string, owner string) (*AssetUpload, error) {
	var upload AssetUpload
	if err := DB.Where("upload_id = ? AND owner = ?", uploadID, owner).First(&upload).Error; err != nil {
		return nil, err
	}
	return &upload, nil
}
