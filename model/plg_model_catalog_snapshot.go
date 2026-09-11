package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PLGModelCatalogSnapshot stores the last observed set of models a group sees
// on the Console "Available Models" page. Change detection compares
// fingerprints and claims a change with a compare-and-swap update so exactly
// one node notifies when several master nodes poll concurrently.
type PLGModelCatalogSnapshot struct {
	GroupName   string `gorm:"primaryKey;type:varchar(64);autoIncrement:false"`
	Fingerprint string `gorm:"type:varchar(64);not null;default:''"`
	ModelNames  string `gorm:"type:text"`
	ModelCount  int    `gorm:"not null;default:0"`
	UpdatedAt   int64  `gorm:"bigint;not null;default:0"`
}

func (PLGModelCatalogSnapshot) TableName() string {
	return "plg_model_catalog_snapshots"
}

// ModelNameList decodes the stored JSON array. Corrupt data yields an empty
// list so a bad row degrades into "everything is new" rather than a crash.
func (s *PLGModelCatalogSnapshot) ModelNameList() []string {
	if s == nil || strings.TrimSpace(s.ModelNames) == "" {
		return []string{}
	}
	var names []string
	if err := common.Unmarshal([]byte(s.ModelNames), &names); err != nil {
		return []string{}
	}
	return names
}

func GetPLGModelCatalogSnapshot(groupName string) (*PLGModelCatalogSnapshot, error) {
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}
	var snapshot PLGModelCatalogSnapshot
	err := DB.Where("group_name = ?", groupName).First(&snapshot).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

// CreatePLGModelCatalogSnapshotIfMissing writes the baseline row. It returns
// false when another node already created it.
func CreatePLGModelCatalogSnapshotIfMissing(groupName string, fingerprint string, modelNames []string, now int64) (bool, error) {
	if DB == nil {
		return false, errors.New("database is not initialized")
	}
	encoded, err := encodePLGModelNames(modelNames)
	if err != nil {
		return false, err
	}
	result := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&PLGModelCatalogSnapshot{
		GroupName:   groupName,
		Fingerprint: fingerprint,
		ModelNames:  encoded,
		ModelCount:  len(modelNames),
		UpdatedAt:   now,
	})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// ClaimPLGModelCatalogSnapshotChange advances the snapshot only if it still
// carries previousFingerprint. Exactly one concurrent caller observes true.
// The new fingerprint always differs from the previous one, so the UPDATE
// changes at least one column and MySQL's changed-rows semantics stay safe.
func ClaimPLGModelCatalogSnapshotChange(groupName string, previousFingerprint string, nextFingerprint string, modelNames []string, now int64) (bool, error) {
	if DB == nil {
		return false, errors.New("database is not initialized")
	}
	if previousFingerprint == nextFingerprint {
		return false, nil
	}
	encoded, err := encodePLGModelNames(modelNames)
	if err != nil {
		return false, err
	}
	result := DB.Model(&PLGModelCatalogSnapshot{}).
		Where("group_name = ? AND fingerprint = ?", groupName, previousFingerprint).
		Updates(map[string]any{
			"fingerprint": nextFingerprint,
			"model_names": encoded,
			"model_count": len(modelNames),
			"updated_at":  now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func encodePLGModelNames(modelNames []string) (string, error) {
	if modelNames == nil {
		modelNames = []string{}
	}
	encoded, err := common.Marshal(modelNames)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
