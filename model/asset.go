package model

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	AssetStatusCreating   = "Creating"
	AssetStatusProcessing = "Processing"
	AssetStatusActive     = "Active"
	AssetStatusFailed     = "Failed"
	AssetStatusExpired    = "Expired"

	AssetSourceStatusAvailable      = "Available"
	AssetSourceStatusUnavailable    = "Unavailable"
	AssetSourceStatusCleanupPending = "CleanupPending"
	AssetSourceStatusExpired        = "Expired"

	AssetBindingStatusPending = "PENDING"
	AssetBindingStatusLeased  = "LEASED"

	AssetUploadStatusPending  = "PENDING"
	AssetUploadStatusComplete = "COMPLETE"
	AssetUploadStatusExpired  = "EXPIRED"
	AssetUploadStatusFailed   = "FAILED"
)

const legacyBytePlusAssetMigrationPageSize = 500

type Asset struct {
	Id                int64  `json:"id" gorm:"primaryKey"`
	PublicId          string `json:"public_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId            int    `json:"user_id" gorm:"index"`
	AssetType         string `json:"asset_type" gorm:"type:varchar(16);index"`
	Status            string `json:"status" gorm:"type:varchar(24);index"`
	SourceStatus      string `json:"source_status" gorm:"type:varchar(24);index"`
	StorageBackend    string `json:"storage_backend" gorm:"type:varchar(16)"`
	StorageBucket     string `json:"storage_bucket" gorm:"type:varchar(255)"`
	ObjectKey         string `json:"object_key" gorm:"type:varchar(512)"`
	ObjectGeneration  int64  `json:"-" gorm:"index"`
	ContentType       string `json:"content_type" gorm:"type:varchar(128)"`
	SizeBytes         int64  `json:"size_bytes"`
	SHA256            string `json:"sha256" gorm:"type:varchar(64)"`
	LastUsedAt        int64  `json:"last_used_at" gorm:"index"`
	SourceExpiresAt   int64  `json:"source_expires_at" gorm:"index"`
	CleanupLeaseOwner string `json:"-" gorm:"type:varchar(64);index"`
	CleanupLeaseUntil int64  `json:"-" gorm:"index"`
	CleanupGeneration int64  `json:"-" gorm:"index"`
	CreatedAt         int64  `json:"created_at"`
	UpdatedAt         int64  `json:"updated_at"`
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
	Id               int64  `json:"id" gorm:"primaryKey"`
	UploadId         string `json:"upload_id" gorm:"type:varchar(64);uniqueIndex"`
	Owner            string `json:"-" gorm:"type:varchar(64);index"`
	UserId           int    `json:"user_id" gorm:"index"`
	AssetId          int64  `json:"asset_id" gorm:"index"`
	PublicId         string `json:"public_id" gorm:"type:varchar(64);index"`
	AssetType        string `json:"asset_type" gorm:"type:varchar(16);index"`
	StorageBackend   string `json:"storage_backend" gorm:"type:varchar(16)"`
	StorageBucket    string `json:"storage_bucket" gorm:"type:varchar(255)"`
	ObjectKey        string `json:"object_key" gorm:"type:varchar(512)"`
	ObjectGeneration int64  `json:"-" gorm:"index"`
	ContentType      string `json:"content_type" gorm:"type:varchar(128)"`
	SizeBytes        int64  `json:"size_bytes"`
	SHA256           string `json:"sha256" gorm:"type:varchar(64)"`
	ExpiresAt        int64  `json:"expires_at" gorm:"index"`
	Status           string `json:"status" gorm:"type:varchar(24);index"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

type AssetWithBinding struct {
	Asset    Asset
	Binding  *AssetBinding
	Bindings []AssetBinding
}

type AssetUploadCompletion struct {
	ContentType      string
	SizeBytes        int64
	SHA256           string
	ObjectGeneration int64
	SourceExpiresAt  int64
	Now              int64
}

type AssetUploadFailure struct {
	SourceStatus     string
	ObjectGeneration int64
	ClearStorage     bool
	Now              int64
}

type AssetBindingActivation struct {
	AssetID         int64
	ChannelID       int
	LeaseOwner      string
	UpstreamGroupID string
	UpstreamAssetID string
	Now             int64
}

type ExpiredAssetUploadCleanupCandidate struct {
	Asset  Asset
	Upload AssetUpload
}

func CreateAsset(asset Asset) (*Asset, error) {
	if err := DB.Create(&asset).Error; err != nil {
		return nil, err
	}
	return &asset, nil
}

func CreateAssetWithUploadSession(asset Asset, upload AssetUpload) (*Asset, error) {
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&asset).Error; err != nil {
			return err
		}
		upload.AssetId = asset.Id
		upload.UserId = asset.UserId
		upload.PublicId = asset.PublicId
		upload.AssetType = asset.AssetType
		upload.StorageBackend = asset.StorageBackend
		upload.StorageBucket = asset.StorageBucket
		upload.ObjectKey = asset.ObjectKey
		if err := tx.Create(&upload).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

func CompleteAssetUploadCAS(uploadID string, owner string, completion AssetUploadCompletion) (bool, error) {
	completed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var upload AssetUpload
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("upload_id = ? AND owner = ? AND status = ?", uploadID, owner, AssetUploadStatusPending).First(&upload).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		var asset Asset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", upload.AssetId, upload.UserId).First(&asset).Error; err != nil {
			return err
		}
		if upload.ExpiresAt <= completion.Now {
			result := tx.Model(&AssetUpload{}).
				Where("id = ? AND owner = ? AND status = ? AND expires_at <= ?", upload.Id, owner, AssetUploadStatusPending, completion.Now).
				Updates(map[string]any{
					"status":     AssetUploadStatusExpired,
					"updated_at": completion.Now,
				})
			return result.Error
		}
		uploadUpdates := map[string]any{
			"status":            AssetUploadStatusComplete,
			"content_type":      completion.ContentType,
			"size_bytes":        completion.SizeBytes,
			"sha256":            completion.SHA256,
			"object_generation": completion.ObjectGeneration,
			"updated_at":        completion.Now,
		}
		result := tx.Model(&AssetUpload{}).
			Where("id = ? AND owner = ? AND status = ? AND expires_at > ?", upload.Id, owner, AssetUploadStatusPending, completion.Now).
			Updates(uploadUpdates)
		if result.Error != nil {
			return result.Error
		}
		completed = result.RowsAffected == 1
		if !completed {
			return nil
		}
		assetUpdates := map[string]any{
			"status":            AssetStatusActive,
			"source_status":     AssetSourceStatusAvailable,
			"content_type":      completion.ContentType,
			"size_bytes":        completion.SizeBytes,
			"sha256":            completion.SHA256,
			"object_generation": completion.ObjectGeneration,
			"source_expires_at": completion.SourceExpiresAt,
			"updated_at":        completion.Now,
		}
		if err := tx.Model(&Asset{}).
			Where("id = ? AND user_id = ? AND status = ?", asset.Id, upload.UserId, AssetStatusCreating).
			Updates(assetUpdates).Error; err != nil {
			return err
		}
		return nil
	})
	return completed, err
}

func FailAssetUploadCAS(uploadID string, owner string, failure AssetUploadFailure) (bool, error) {
	failed := false
	sourceStatus := failure.SourceStatus
	if sourceStatus == "" {
		sourceStatus = AssetSourceStatusUnavailable
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var upload AssetUpload
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("upload_id = ? AND owner = ? AND status = ?", uploadID, owner, AssetUploadStatusPending).
			First(&upload).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		var asset Asset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", upload.AssetId, upload.UserId).
			First(&asset).Error; err != nil {
			return err
		}
		uploadUpdates := map[string]any{
			"status":            AssetUploadStatusFailed,
			"object_generation": failure.ObjectGeneration,
			"updated_at":        failure.Now,
		}
		result := tx.Model(&AssetUpload{}).
			Where("id = ? AND owner = ? AND status = ?", upload.Id, owner, AssetUploadStatusPending).
			Updates(uploadUpdates)
		if result.Error != nil {
			return result.Error
		}
		failed = result.RowsAffected == 1
		if !failed {
			return nil
		}
		assetUpdates := map[string]any{
			"status":            AssetStatusFailed,
			"source_status":     sourceStatus,
			"object_generation": failure.ObjectGeneration,
			"updated_at":        failure.Now,
		}
		if failure.ClearStorage {
			assetUpdates["storage_backend"] = ""
			assetUpdates["storage_bucket"] = ""
			assetUpdates["object_key"] = ""
			assetUpdates["object_generation"] = int64(0)
		}
		return tx.Model(&Asset{}).
			Where("id = ? AND user_id = ? AND status = ?", asset.Id, upload.UserId, AssetStatusCreating).
			Updates(assetUpdates).Error
	})
	return failed, err
}

func ActivateAssetBindingWithAssetCAS(activation AssetBindingActivation) (bool, error) {
	activated := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var asset Asset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", activation.AssetID).
			First(&asset).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		query := tx.Model(&AssetBinding{}).
			Where("asset_id = ? AND channel_id = ?", activation.AssetID, activation.ChannelID).
			Where("status IN ?", []string{AssetBindingStatusPending, AssetBindingStatusLeased})
		if activation.LeaseOwner != "" {
			query = query.Where("lease_owner = ?", activation.LeaseOwner)
		}
		result := query.Updates(map[string]any{
			"status":            AssetStatusActive,
			"upstream_group_id": activation.UpstreamGroupID,
			"upstream_asset_id": activation.UpstreamAssetID,
			"lease_owner":       "",
			"lease_expires_at":  int64(0),
			"updated_at":        activation.Now,
		})
		if result.Error != nil {
			return result.Error
		}
		activated = result.RowsAffected == 1
		if !activated {
			return nil
		}
		return tx.Model(&Asset{}).
			Where("id = ?", asset.Id).
			Update("status", AssetStatusActive).Error
	})
	return activated, err
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

func GetAssetByIDForUser(assetID int64, userID int) (*Asset, error) {
	var asset Asset
	if err := DB.Where("id = ? AND user_id = ?", assetID, userID).First(&asset).Error; err != nil {
		return nil, err
	}
	return &asset, nil
}

func ClaimExpiredAssetSources(owner string, now int64, leaseUntil int64, limit int) ([]Asset, error) {
	if limit <= 0 {
		limit = 100
	}
	var candidates []Asset
	if err := DB.Where("(source_status = ? AND source_expires_at > 0 AND source_expires_at <= ?) OR source_status = ?", AssetSourceStatusAvailable, now, AssetSourceStatusCleanupPending).
		Where("cleanup_lease_owner = ? OR cleanup_lease_until <= ? OR cleanup_lease_owner = ''", owner, now).
		Order("source_expires_at ASC, id ASC").
		Limit(limit).
		Find(&candidates).Error; err != nil {
		return nil, err
	}
	claimed := make([]Asset, 0, len(candidates))
	for _, candidate := range candidates {
		nextGeneration := candidate.CleanupGeneration + 1
		result := DB.Model(&Asset{}).
			Where("id = ? AND cleanup_generation = ?", candidate.Id, candidate.CleanupGeneration).
			Where("(source_status = ? AND source_expires_at > 0 AND source_expires_at <= ?) OR source_status = ?", AssetSourceStatusAvailable, now, AssetSourceStatusCleanupPending).
			Where("cleanup_lease_owner = ? OR cleanup_lease_until <= ? OR cleanup_lease_owner = ''", owner, now).
			Updates(map[string]any{
				"cleanup_lease_owner": owner,
				"cleanup_lease_until": leaseUntil,
				"cleanup_generation":  nextGeneration,
				"updated_at":          now,
			})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 1 {
			candidate.CleanupLeaseOwner = owner
			candidate.CleanupLeaseUntil = leaseUntil
			candidate.CleanupGeneration = nextGeneration
			claimed = append(claimed, candidate)
		}
	}
	return claimed, nil
}

func ClaimExpiredPendingAssetUploads(owner string, now int64, leaseUntil int64, limit int) ([]ExpiredAssetUploadCleanupCandidate, error) {
	if limit <= 0 {
		limit = 100
	}
	var uploads []AssetUpload
	if err := DB.Where("status = ? AND expires_at > 0 AND expires_at <= ?", AssetUploadStatusPending, now).
		Order("expires_at ASC, id ASC").
		Limit(limit).
		Find(&uploads).Error; err != nil {
		return nil, err
	}
	claimed := make([]ExpiredAssetUploadCleanupCandidate, 0, len(uploads))
	for _, upload := range uploads {
		err := DB.Transaction(func(tx *gorm.DB) error {
			var asset Asset
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND user_id = ? AND status = ? AND source_status = ?", upload.AssetId, upload.UserId, AssetStatusCreating, AssetSourceStatusUnavailable).
				Where("cleanup_lease_owner = ? OR cleanup_lease_until <= ? OR cleanup_lease_owner = ''", owner, now).
				First(&asset).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil
				}
				return err
			}
			nextGeneration := asset.CleanupGeneration + 1
			result := tx.Model(&Asset{}).
				Where("id = ? AND cleanup_generation = ?", asset.Id, asset.CleanupGeneration).
				Where("status = ? AND source_status = ?", AssetStatusCreating, AssetSourceStatusUnavailable).
				Where("cleanup_lease_owner = ? OR cleanup_lease_until <= ? OR cleanup_lease_owner = ''", owner, now).
				Updates(map[string]any{
					"cleanup_lease_owner": owner,
					"cleanup_lease_until": leaseUntil,
					"cleanup_generation":  nextGeneration,
					"updated_at":          now,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return nil
			}
			asset.CleanupLeaseOwner = owner
			asset.CleanupLeaseUntil = leaseUntil
			asset.CleanupGeneration = nextGeneration
			claimed = append(claimed, ExpiredAssetUploadCleanupCandidate{Asset: asset, Upload: upload})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return claimed, nil
}

func MarkAssetSourceExpiredIfCleanupLease(assetID int64, owner string, generation int64, now int64) (bool, error) {
	expired := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var asset Asset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND cleanup_lease_owner = ? AND cleanup_generation = ?", assetID, owner, generation).
			Where("source_status IN ?", []string{AssetSourceStatusAvailable, AssetSourceStatusCleanupPending}).
			First(&asset).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		nextStatus := AssetStatusExpired
		var activeBindings int64
		if err := tx.Model(&AssetBinding{}).
			Where("asset_id = ? AND status = ?", assetID, AssetStatusActive).
			Count(&activeBindings).Error; err != nil {
			return err
		}
		if activeBindings > 0 {
			nextStatus = AssetStatusActive
		}
		result := tx.Model(&Asset{}).
			Where("id = ? AND cleanup_lease_owner = ? AND cleanup_generation = ?", asset.Id, owner, generation).
			Where("source_status IN ?", []string{AssetSourceStatusAvailable, AssetSourceStatusCleanupPending}).
			Updates(map[string]any{
				"source_status":       AssetSourceStatusExpired,
				"status":              nextStatus,
				"storage_backend":     "",
				"storage_bucket":      "",
				"object_key":          "",
				"object_generation":   int64(0),
				"cleanup_lease_owner": "",
				"cleanup_lease_until": int64(0),
				"updated_at":          now,
			})
		if result.Error != nil {
			return result.Error
		}
		expired = result.RowsAffected == 1
		return nil
	})
	return expired, err
}

func MarkExpiredPendingAssetUploadIfCleanupLease(uploadID string, owner string, cleanupGeneration int64, objectGeneration int64, now int64) (bool, error) {
	expired := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var upload AssetUpload
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("upload_id = ? AND status = ?", uploadID, AssetUploadStatusPending).
			First(&upload).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		var asset Asset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ? AND cleanup_lease_owner = ? AND cleanup_generation = ?", upload.AssetId, upload.UserId, owner, cleanupGeneration).
			Where("status = ? AND source_status = ?", AssetStatusCreating, AssetSourceStatusUnavailable).
			First(&asset).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if err := tx.Model(&AssetUpload{}).
			Where("id = ? AND status = ?", upload.Id, AssetUploadStatusPending).
			Updates(map[string]any{
				"status":            AssetUploadStatusExpired,
				"object_generation": objectGeneration,
				"updated_at":        now,
			}).Error; err != nil {
			return err
		}
		result := tx.Model(&Asset{}).
			Where("id = ? AND cleanup_lease_owner = ? AND cleanup_generation = ?", asset.Id, owner, cleanupGeneration).
			Where("status = ? AND source_status = ?", AssetStatusCreating, AssetSourceStatusUnavailable).
			Updates(map[string]any{
				"status":              AssetStatusExpired,
				"source_status":       AssetSourceStatusExpired,
				"storage_backend":     "",
				"storage_bucket":      "",
				"object_key":          "",
				"object_generation":   int64(0),
				"cleanup_lease_owner": "",
				"cleanup_lease_until": int64(0),
				"updated_at":          now,
			})
		if result.Error != nil {
			return result.Error
		}
		expired = result.RowsAffected == 1
		return nil
	})
	return expired, err
}
