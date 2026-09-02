package model

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	NameRuleExact = iota
	NameRulePrefix
	NameRuleContains
	NameRuleSuffix
)

type BoundChannel struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

type Model struct {
	Id            int            `json:"id"`
	ModelName     string         `json:"model_name" gorm:"size:128;not null;uniqueIndex:uk_model_name_delete_at,priority:1"`
	Description   string         `json:"description,omitempty" gorm:"type:text"`
	Icon          string         `json:"icon,omitempty" gorm:"type:varchar(128)"`
	Tags          string         `json:"tags,omitempty" gorm:"type:varchar(255)"`
	DisplayWeight int            `json:"display_weight" gorm:"not null;default:0;index"`
	VendorID      int            `json:"vendor_id,omitempty" gorm:"index"`
	Endpoints     string         `json:"endpoints,omitempty" gorm:"type:text"`
	Status        int            `json:"status" gorm:"default:1"`
	SyncOfficial  int            `json:"sync_official" gorm:"default:1"`
	CreatedTime   int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime   int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:uk_model_name_delete_at,priority:2"`

	BoundChannels []BoundChannel `json:"bound_channels,omitempty" gorm:"-"`
	EnableGroups  []string       `json:"enable_groups,omitempty" gorm:"-"`
	QuotaTypes    []int          `json:"quota_types,omitempty" gorm:"-"`
	NameRule      int            `json:"name_rule" gorm:"default:0"`

	MatchedModels []string `json:"matched_models,omitempty" gorm:"-"`
	MatchedCount  int      `json:"matched_count,omitempty" gorm:"-"`

	AvailabilityStatus              string `json:"availability_status,omitempty" gorm:"-"`
	AvailabilityReasonType          string `json:"availability_reason_type,omitempty" gorm:"-"`
	AvailabilityReason              string `json:"availability_reason,omitempty" gorm:"-"`
	AvailabilityLastError           string `json:"availability_last_error,omitempty" gorm:"-"`
	AvailabilityDetectedAt          int64  `json:"availability_detected_at,omitempty" gorm:"-"`
	AvailabilityCheckedAt           int64  `json:"availability_checked_at,omitempty" gorm:"-"`
	AvailabilityLastSuccessAt       int64  `json:"availability_last_success_at,omitempty" gorm:"-"`
	AvailabilityConsecutiveFailures int    `json:"availability_consecutive_failures,omitempty" gorm:"-"`
}

func ApplyAvailabilityStateToModel(m *Model, state ModelAvailabilityState) {
	if m == nil {
		return
	}
	m.AvailabilityStatus = state.Status
	m.AvailabilityReasonType = state.ReasonType
	m.AvailabilityReason = state.Reason
	m.AvailabilityLastError = state.LastError
	m.AvailabilityDetectedAt = state.FirstDetectedAt
	m.AvailabilityCheckedAt = state.LastCheckedAt
	m.AvailabilityLastSuccessAt = state.LastSuccessAt
	m.AvailabilityConsecutiveFailures = state.ConsecutiveFailures
}

func (mi *Model) Insert() error {
	now := common.GetTimestamp()
	mi.CreatedTime = now
	mi.UpdatedTime = now

	// 保存原始值（因为 Create 后可能被 GORM 的 default 标签覆盖为 1）
	originalStatus := mi.Status
	originalSyncOfficial := mi.SyncOfficial

	// 先创建记录（GORM 会对零值字段应用默认值）
	if err := DB.Create(mi).Error; err != nil {
		return err
	}

	// 使用保存的原始值进行更新，确保零值能正确保存
	return DB.Model(&Model{}).Where("id = ?", mi.Id).Updates(map[string]interface{}{
		"status":        originalStatus,
		"sync_official": originalSyncOfficial,
	}).Error
}

func IsModelNameDuplicated(id int, name string) (bool, error) {
	if name == "" {
		return false, nil
	}
	var cnt int64
	err := DB.Model(&Model{}).Where("model_name = ? AND id <> ?", name, id).Count(&cnt).Error
	return cnt > 0, err
}

func (mi *Model) Update() error {
	mi.UpdatedTime = common.GetTimestamp()
	// 使用 Select 强制更新所有字段，包括零值
	return DB.Model(&Model{}).Where("id = ?", mi.Id).
		Select("model_name", "description", "icon", "tags", "display_weight", "vendor_id", "endpoints", "status", "sync_official", "name_rule", "updated_time").
		Updates(mi).Error
}

func (mi *Model) Delete() error {
	return DB.Delete(mi).Error
}

func GetVendorModelCounts() (map[int64]int64, error) {
	var stats []struct {
		VendorID int64
		Count    int64
	}
	if err := DB.Model(&Model{}).
		Select("vendor_id as vendor_id, count(*) as count").
		Group("vendor_id").
		Scan(&stats).Error; err != nil {
		return nil, err
	}
	m := make(map[int64]int64, len(stats))
	for _, s := range stats {
		m[s.VendorID] = s.Count
	}
	return m, nil
}

func GetAllModels(offset int, limit int, hasTags bool) ([]*Model, error) {
	var models []*Model
	db := DB
	if hasTags {
		db = db.Where("tags IS NOT NULL AND tags <> ?", "")
	}
	err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&models).Error
	return models, err
}

// GetModelCreatedTimes returns creation timestamps for active model metadata
// records matching the supplied names. Missing names are omitted from the map.
func GetModelCreatedTimes(modelNames []string) (map[string]int64, error) {
	createdTimes := make(map[string]int64)
	if len(modelNames) == 0 {
		return createdTimes, nil
	}
	if DB == nil {
		return nil, errors.New("model database is not initialized")
	}

	var rows []struct {
		ModelName   string `gorm:"column:model_name"`
		CreatedTime int64  `gorm:"column:created_time"`
	}
	const queryBatchSize = 500
	for start := 0; start < len(modelNames); start += queryBatchSize {
		end := start + queryBatchSize
		if end > len(modelNames) {
			end = len(modelNames)
		}
		rows = rows[:0]
		if err := DB.Model(&Model{}).
			Select("model_name, created_time").
			Where("model_name IN ?", modelNames[start:end]).
			Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			createdTimes[row.ModelName] = row.CreatedTime
		}
	}
	return createdTimes, nil
}

func GetBoundChannelsByModelsMap(modelNames []string) (map[string][]BoundChannel, error) {
	result := make(map[string][]BoundChannel)
	if len(modelNames) == 0 {
		return result, nil
	}
	type row struct {
		Model string
		Name  string
		Type  int
	}
	var rows []row
	err := DB.Table("channels").
		Select("abilities.model as model, channels.name as name, channels.type as type").
		Joins("JOIN abilities ON abilities.channel_id = channels.id").
		Where("abilities.model IN ? AND abilities.enabled = ?", modelNames, true).
		Distinct().
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.Model] = append(result[r.Model], BoundChannel{Name: r.Name, Type: r.Type})
	}
	return result, nil
}

func normalizeLookupValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func GetPreferredModelOwnerChannelTypes(modelNames []string, groups []string) (map[string]int, error) {
	result := make(map[string]int)
	modelNames = normalizeLookupValues(modelNames)
	if len(modelNames) == 0 {
		return result, nil
	}

	type row struct {
		Model       string
		ChannelType int
	}
	var rows []row

	query := DB.Table("abilities").
		Select("abilities.model as model, channels.type as channel_type").
		Joins("JOIN channels ON abilities.channel_id = channels.id").
		Where("abilities.model IN ? AND abilities.enabled = ? AND channels.status = ?", modelNames, true, common.ChannelStatusEnabled).
		Order("COALESCE(abilities.priority, 0) DESC").
		Order("abilities.weight DESC").
		Order("abilities.channel_id ASC")

	groups = normalizeLookupValues(groups)
	if len(groups) > 0 {
		query = query.Where("abilities."+commonGroupCol+" IN ?", groups)
	}

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, r := range rows {
		if _, ok := result[r.Model]; ok {
			continue
		}
		result[r.Model] = r.ChannelType
	}
	return result, nil
}

func SearchModels(keyword string, vendor string, offset int, limit int, hasTags bool) ([]*Model, int64, error) {
	var models []*Model
	db := DB.Model(&Model{})
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("model_name LIKE ? OR description LIKE ? OR tags LIKE ?", like, like, like)
	}
	if vendor != "" {
		if vid, err := strconv.Atoi(vendor); err == nil {
			db = db.Where("models.vendor_id = ?", vid)
		} else {
			db = db.Joins("JOIN vendors ON vendors.id = models.vendor_id").Where("vendors.name LIKE ?", "%"+vendor+"%")
		}
	}
	if hasTags {
		db = db.Where("models.tags IS NOT NULL AND models.tags <> ?", "")
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("models.id DESC").Offset(offset).Limit(limit).Find(&models).Error; err != nil {
		return nil, 0, err
	}
	return models, total, nil
}
