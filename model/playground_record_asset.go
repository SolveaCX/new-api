package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrPlaygroundAssetNotFound     = errors.New("playground attachment asset not found")
	ErrPlaygroundAssetExpired      = errors.New("playground attachment asset is expired")
	ErrPlaygroundAssetTypeMismatch = errors.New("playground attachment asset type mismatch")
	ErrPlaygroundAssetInvalidID    = errors.New("invalid playground attachment asset id")
)

type PlaygroundAssetReference struct {
	AssetID   string
	AssetType string
}

// PlaygroundRecordAsset stores the durable ownership edge separately from the
// large JSON record. Public asset IDs are used intentionally so records never
// expose internal database IDs or object-store keys.
type PlaygroundRecordAsset struct {
	ID             int64     `json:"id" gorm:"primaryKey"`
	UserID         int       `json:"user_id" gorm:"not null;index:idx_playground_asset_ref_user_conversation,priority:1;index:idx_playground_asset_ref_asset,priority:1"`
	ConversationID string    `json:"conversation_id" gorm:"type:varchar(64);not null;index:idx_playground_asset_ref_user_conversation,priority:2"`
	RecordID       string    `json:"record_id" gorm:"type:varchar(64);not null;index;uniqueIndex:idx_playground_asset_ref_record_asset,priority:1"`
	AssetID        string    `json:"asset_id" gorm:"type:varchar(64);not null;uniqueIndex:idx_playground_asset_ref_record_asset,priority:2;index:idx_playground_asset_ref_asset,priority:2"`
	CreatedAt      time.Time `json:"created_at"`
}

func normalizePlaygroundAssetType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "image":
		return AssetTypeImage
	case "video":
		return AssetTypeVideo
	default:
		return ""
	}
}

const (
	AssetTypeImage = "Image"
	AssetTypeVideo = "Video"
)

func normalizePlaygroundAssetReferences(refs []PlaygroundAssetReference) ([]PlaygroundAssetReference, error) {
	byID := make(map[string]PlaygroundAssetReference, len(refs))
	for _, ref := range refs {
		id := strings.TrimSpace(ref.AssetID)
		if id == "" {
			continue
		}
		if !strings.HasPrefix(id, "ast_") || len(id) <= len("ast_") {
			return nil, ErrPlaygroundAssetInvalidID
		}
		typ := normalizePlaygroundAssetType(ref.AssetType)
		if existing, ok := byID[id]; ok {
			if existing.AssetType == "" {
				existing.AssetType = typ
				byID[id] = existing
			} else if typ != "" && existing.AssetType != typ {
				return nil, ErrPlaygroundAssetTypeMismatch
			}
			continue
		}
		byID[id] = PlaygroundAssetReference{AssetID: id, AssetType: typ}
	}
	result := make([]PlaygroundAssetReference, 0, len(byID))
	for _, ref := range byID {
		result = append(result, ref)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].AssetID < result[j].AssetID })
	return result, nil
}

// validatePlaygroundAssetReferences enforces user ownership and source
// readiness before a record can retain an asset edge. When called inside the
// record transaction it locks the selected asset rows, closing the race where
// an orphan-cleanup worker could claim an asset between controller validation
// and reference insertion.
func validatePlaygroundAssetReferences(tx *gorm.DB, userID int, refs []PlaygroundAssetReference) error {
	normalized, err := normalizePlaygroundAssetReferences(refs)
	if err != nil {
		return err
	}
	if len(normalized) == 0 {
		return nil
	}
	if userID <= 0 || tx == nil || !tx.Migrator().HasTable(&Asset{}) {
		return ErrPlaygroundAssetNotFound
	}
	ids := make([]string, 0, len(normalized))
	for _, ref := range normalized {
		ids = append(ids, ref.AssetID)
	}
	var assets []Asset
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND public_id IN ?", userID, ids).Find(&assets).Error; err != nil {
		return err
	}
	byID := make(map[string]Asset, len(assets))
	for _, asset := range assets {
		byID[asset.PublicId] = asset
	}
	for _, ref := range normalized {
		asset, ok := byID[ref.AssetID]
		if !ok {
			return ErrPlaygroundAssetNotFound
		}
		if asset.Status != AssetStatusActive || asset.SourceStatus != AssetSourceStatusAvailable {
			return ErrPlaygroundAssetExpired
		}
		if asset.SourceExpiresAt > 0 && asset.SourceExpiresAt <= time.Now().Unix() {
			return ErrPlaygroundAssetExpired
		}
		if expected := normalizePlaygroundAssetType(ref.AssetType); expected != "" && asset.AssetType != expected {
			return ErrPlaygroundAssetTypeMismatch
		}
	}
	return nil
}

// ValidatePlaygroundAssetReferences checks references against the current
// database state for the controller's early, user-facing validation pass.
func ValidatePlaygroundAssetReferences(userID int, refs []PlaygroundAssetReference) error {
	return validatePlaygroundAssetReferences(DB, userID, refs)
}

func ReplacePlaygroundRecordAssets(tx *gorm.DB, record *PlaygroundRecord) error {
	if tx == nil || record == nil || !tx.Migrator().HasTable(&PlaygroundRecordAsset{}) {
		return nil
	}
	refs, err := normalizePlaygroundAssetReferences(record.AssetReferences)
	if err != nil {
		return err
	}
	if err := validatePlaygroundAssetReferences(tx, record.UserID, refs); err != nil {
		return err
	}
	if err := tx.Where("user_id = ? AND record_id = ?", record.UserID, record.RecordID).Delete(&PlaygroundRecordAsset{}).Error; err != nil {
		return err
	}
	for _, ref := range refs {
		row := &PlaygroundRecordAsset{
			UserID: record.UserID, ConversationID: record.ConversationID,
			RecordID: record.RecordID, AssetID: ref.AssetID, CreatedAt: time.Now(),
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error; err != nil {
			return err
		}
	}
	return nil
}

func DeletePlaygroundConversationAssetReferences(tx *gorm.DB, userID int, conversationID string) ([]string, error) {
	if tx == nil || !tx.Migrator().HasTable(&PlaygroundRecordAsset{}) {
		return nil, nil
	}
	var rows []PlaygroundRecordAsset
	if err := tx.Where("user_id = ? AND conversation_id = ?", userID, conversationID).Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if _, ok := seen[row.AssetID]; ok {
			continue
		}
		seen[row.AssetID] = struct{}{}
		ids = append(ids, row.AssetID)
	}
	if err := tx.Where("user_id = ? AND conversation_id = ?", userID, conversationID).Delete(&PlaygroundRecordAsset{}).Error; err != nil {
		return nil, err
	}
	sort.Strings(ids)
	return ids, nil
}

func CountPlaygroundAssetReferences(tx *gorm.DB, userID int, assetID string) (int64, error) {
	if tx == nil || !tx.Migrator().HasTable(&PlaygroundRecordAsset{}) {
		return 0, nil
	}
	var count int64
	err := tx.Model(&PlaygroundRecordAsset{}).Where("user_id = ? AND asset_id = ?", userID, assetID).Count(&count).Error
	return count, err
}

// ClaimPlaygroundAssetsForCleanup marks explicitly-cleared, unreferenced
// assets for object-store cleanup. The claim is serialized with record saves
// and clears through the user row lock, so a concurrent save cannot attach a
// new record after the orphan decision. Provider bindings are treated as
// independent owners and therefore keep their source object alive.
func ClaimPlaygroundAssetsForCleanup(userID int, publicIDs []string, owner string, now, leaseUntil int64) ([]Asset, error) {
	if DB == nil || userID <= 0 || !DB.Migrator().HasTable(&Asset{}) || !DB.Migrator().HasTable(&PlaygroundRecordAsset{}) {
		return nil, nil
	}
	if now <= 0 {
		now = time.Now().Unix()
	}
	if leaseUntil <= now {
		leaseUntil = now + int64((5 * time.Minute).Seconds())
	}
	owner = strings.TrimSpace(owner)
	if owner == "" {
		owner = fmt.Sprintf("playground-cleanup-%d", userID)
	}

	ids := make([]string, 0, len(publicIDs))
	seen := make(map[string]struct{}, len(publicIDs))
	for _, rawID := range publicIDs {
		id := strings.TrimSpace(rawID)
		if id == "" || !strings.HasPrefix(id, "ast_") {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	claimed := make([]Asset, 0, len(ids))
	err := DB.Transaction(func(tx *gorm.DB) error {
		// SavePlaygroundRecord and ClearPlaygroundConversationWithAssets both
		// acquire this lock. Older test databases may omit users, in which case
		// the asset row lock still provides the best available serialization.
		if tx.Migrator().HasTable(&User{}) {
			if err := lockPlaygroundUser(tx, userID); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		for _, publicID := range ids {
			var asset Asset
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("user_id = ? AND public_id = ?", userID, publicID).
				First(&asset).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			if asset.Status != AssetStatusActive || (asset.SourceStatus != AssetSourceStatusAvailable && asset.SourceStatus != AssetSourceStatusCleanupPending) {
				continue
			}
			if asset.CleanupLeaseOwner != "" && asset.CleanupLeaseOwner != owner && asset.CleanupLeaseUntil > now {
				continue
			}

			var references int64
			if err := tx.Model(&PlaygroundRecordAsset{}).
				Where("user_id = ? AND asset_id = ?", userID, publicID).
				Count(&references).Error; err != nil {
				return err
			}
			if references > 0 {
				continue
			}
			if tx.Migrator().HasTable(&AssetBinding{}) {
				var bindings int64
				if err := tx.Model(&AssetBinding{}).Where("asset_id = ?", asset.Id).Count(&bindings).Error; err != nil {
					return err
				}
				if bindings > 0 {
					continue
				}
			}

			nextGeneration := asset.CleanupGeneration + 1
			result := tx.Model(&Asset{}).
				Where("id = ? AND cleanup_generation = ?", asset.Id, asset.CleanupGeneration).
				Where("status = ? AND source_status IN ?", AssetStatusActive, []string{AssetSourceStatusAvailable, AssetSourceStatusCleanupPending}).
				Where("cleanup_lease_until <= ? OR cleanup_lease_owner = ? OR cleanup_lease_owner = ''", now, owner).
				Updates(map[string]any{
					"source_status":       AssetSourceStatusCleanupPending,
					"source_expires_at":   now,
					"cleanup_lease_owner": owner,
					"cleanup_lease_until": leaseUntil,
					"cleanup_generation":  nextGeneration,
					"updated_at":          now,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				continue
			}
			asset.SourceStatus = AssetSourceStatusCleanupPending
			asset.SourceExpiresAt = now
			asset.CleanupLeaseOwner = owner
			asset.CleanupLeaseUntil = leaseUntil
			asset.CleanupGeneration = nextGeneration
			claimed = append(claimed, asset)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}
