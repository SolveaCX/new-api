package model

import (
	"errors"
	"strings"

	"gorm.io/gorm/clause"
)

var ErrBytePlusAssetBindingScopeRequired = errors.New("byteplus asset binding scope is required")

type BytePlusAssetBindingGroup struct {
	Id                int64  `json:"id"`
	UserId            int    `json:"user_id" gorm:"uniqueIndex:idx_byteplus_asset_binding_group_scope;index"`
	ChannelId         int    `json:"channel_id" gorm:"uniqueIndex:idx_byteplus_asset_binding_group_scope;index"`
	BindingScope      string `json:"-" gorm:"type:varchar(128);not null;uniqueIndex:idx_byteplus_asset_binding_group_scope"`
	UpstreamGroupId   string `json:"-" gorm:"type:varchar(128)"`
	UpstreamRequestId string `json:"-" gorm:"type:varchar(128)"`
	Status            string `json:"status" gorm:"type:varchar(32);index"`
	ErrorMessage      string `json:"-" gorm:"type:text"`
	LeaseUpdatedTime  int64  `json:"-" gorm:"bigint;index"`
	CreatedTime       int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime       int64  `json:"updated_time" gorm:"bigint"`
}

func (*BytePlusAssetBindingGroup) TableName() string {
	return "byte_plus_asset_binding_groups"
}

func ClaimBytePlusAssetBindingGroup(userID int, channelID int, bindingScope string, now int64, staleBefore int64) (*BytePlusAssetBindingGroup, bool, error) {
	bindingScope = strings.TrimSpace(bindingScope)
	if bindingScope == "" {
		return nil, false, ErrBytePlusAssetBindingScopeRequired
	}
	group := &BytePlusAssetBindingGroup{
		UserId:           userID,
		ChannelId:        channelID,
		BindingScope:     bindingScope,
		Status:           BytePlusAssetGroupStatusCreating,
		LeaseUpdatedTime: now,
		CreatedTime:      now,
		UpdatedTime:      now,
	}
	insert := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(group)
	if insert.Error != nil {
		return nil, false, insert.Error
	}
	if insert.RowsAffected == 1 {
		return group, true, nil
	}

	update := DB.Model(&BytePlusAssetBindingGroup{}).
		Where("user_id = ? AND channel_id = ? AND binding_scope = ? AND (status = ? OR (status = ? AND lease_updated_time < ?))",
			userID,
			channelID,
			bindingScope,
			BytePlusAssetGroupStatusFailed,
			BytePlusAssetGroupStatusCreating,
			staleBefore,
		).
		Updates(map[string]any{
			"status":              BytePlusAssetGroupStatusCreating,
			"error_message":       "",
			"upstream_group_id":   "",
			"upstream_request_id": "",
			"lease_updated_time":  now,
			"updated_time":        now,
		})
	if update.Error != nil {
		return nil, false, update.Error
	}

	stored, err := GetBytePlusAssetBindingGroup(userID, channelID, bindingScope)
	if err != nil {
		return nil, false, err
	}
	return stored, update.RowsAffected == 1, nil
}

func GetBytePlusAssetBindingGroup(userID int, channelID int, bindingScope string) (*BytePlusAssetBindingGroup, error) {
	var group BytePlusAssetBindingGroup
	if err := DB.Where("user_id = ? AND channel_id = ? AND binding_scope = ?", userID, channelID, strings.TrimSpace(bindingScope)).First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func ActivateBytePlusAssetBindingGroup(groupID int64, leaseUpdatedTime int64, upstreamGroupID string, upstreamRequestID string, now int64) (bool, error) {
	result := DB.Model(&BytePlusAssetBindingGroup{}).
		Where("id = ? AND status = ? AND lease_updated_time = ?", groupID, BytePlusAssetGroupStatusCreating, leaseUpdatedTime).
		Updates(map[string]any{
			"upstream_group_id":   upstreamGroupID,
			"upstream_request_id": upstreamRequestID,
			"status":              BytePlusAssetGroupStatusActive,
			"error_message":       "",
			"updated_time":        now,
		})
	return result.RowsAffected == 1, result.Error
}

func FailBytePlusAssetBindingGroup(groupID int64, leaseUpdatedTime int64, upstreamRequestID string, errorMessage string, now int64) (bool, error) {
	result := DB.Model(&BytePlusAssetBindingGroup{}).
		Where("id = ? AND status = ? AND lease_updated_time = ?", groupID, BytePlusAssetGroupStatusCreating, leaseUpdatedTime).
		Updates(map[string]any{
			"upstream_request_id": upstreamRequestID,
			"status":              BytePlusAssetGroupStatusFailed,
			"error_message":       errorMessage,
			"updated_time":        now,
		})
	return result.RowsAffected == 1, result.Error
}

func InvalidateActiveBytePlusAssetBindingGroup(groupID int64, expectedUpstreamGroupID string, upstreamRequestID string, errorMessage string, now int64) (bool, error) {
	result := DB.Model(&BytePlusAssetBindingGroup{}).
		Where("id = ? AND status = ? AND upstream_group_id = ?", groupID, BytePlusAssetGroupStatusActive, expectedUpstreamGroupID).
		Updates(map[string]any{
			"upstream_request_id": upstreamRequestID,
			"status":              BytePlusAssetGroupStatusFailed,
			"error_message":       errorMessage,
			"updated_time":        now,
		})
	return result.RowsAffected == 1, result.Error
}
