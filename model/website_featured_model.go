package model

import (
	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// WebsiteFeaturedModel stores the optional merchandising order used by the
// public website model directory. It deliberately does not affect API routing
// or the regular model catalog.
type WebsiteFeaturedModel struct {
	ID                      int    `json:"id"`
	ModelName               string `json:"model_name" gorm:"size:128;not null;uniqueIndex"`
	SortOrder               int    `json:"sort_order" gorm:"not null;index"`
	DisplayName             string `json:"display_name" gorm:"size:255"`
	Description             string `json:"description" gorm:"type:text"`
	Tags                    string `json:"tags" gorm:"size:255"`
	BackgroundImageURL      string `json:"background_image_url" gorm:"size:1024"`
	BackgroundImage         string `json:"background_image" gorm:"type:text"`
	FallbackBackgroundImage string `json:"fallback_background_image" gorm:"size:1024"`
	Video                   string `json:"video" gorm:"size:1024"`
	CreatedAt               int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt               int64  `json:"updated_at" gorm:"bigint"`
}

// WebsiteFeaturedModelInput is the admin payload used to persist banner
// merchandising. All presentation fields are optional and fall back to model
// metadata or the curated defaults on the public website.
type WebsiteFeaturedModelInput struct {
	ModelName               string
	DisplayName             string
	Description             string
	Tags                    string
	BackgroundImageURL      string
	BackgroundImage         string
	FallbackBackgroundImage string
	Video                   string
}

// WebsiteFeaturedModelConfig is embedded in the public pricing response for
// models selected for the top banner.
type WebsiteFeaturedModelConfig struct {
	DisplayName             string `json:"display_name,omitempty"`
	Description             string `json:"description,omitempty"`
	Tags                    string `json:"tags,omitempty"`
	BackgroundImageURL      string `json:"background_image_url,omitempty"`
	BackgroundImage         string `json:"background_image,omitempty"`
	FallbackBackgroundImage string `json:"fallback_background_image,omitempty"`
	Video                   string `json:"video,omitempty"`
}

// ListWebsiteFeaturedModels returns the configured order, with the ID as a
// deterministic tie-breaker for rows created by older versions.
func ListWebsiteFeaturedModels() ([]WebsiteFeaturedModel, error) {
	var rows []WebsiteFeaturedModel
	err := DB.Order("sort_order ASC").Order("id ASC").Find(&rows).Error
	return rows, err
}

// ReplaceWebsiteFeaturedModels atomically replaces the complete configured
// order. The caller validates public visibility and duplicate names before
// invoking this function.
func ReplaceWebsiteFeaturedModels(modelNames []string) error {
	items := make([]WebsiteFeaturedModelInput, len(modelNames))
	for i, modelName := range modelNames {
		items[i] = WebsiteFeaturedModelInput{ModelName: modelName}
	}
	return ReplaceWebsiteFeaturedModelsWithConfig(items)
}

// ReplaceWebsiteFeaturedModelsWithConfig atomically replaces the complete
// configured order and its optional banner presentation fields.
func ReplaceWebsiteFeaturedModelsWithConfig(items []WebsiteFeaturedModelInput) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&WebsiteFeaturedModel{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}

		now := common.GetTimestamp()
		rows := make([]WebsiteFeaturedModel, len(items))
		for i, item := range items {
			rows[i] = WebsiteFeaturedModel{
				ModelName:               item.ModelName,
				SortOrder:               i,
				DisplayName:             item.DisplayName,
				Description:             item.Description,
				Tags:                    item.Tags,
				BackgroundImageURL:      item.BackgroundImageURL,
				BackgroundImage:         item.BackgroundImage,
				FallbackBackgroundImage: item.FallbackBackgroundImage,
				Video:                   item.Video,
				CreatedAt:               now,
				UpdatedAt:               now,
			}
		}
		return tx.Create(&rows).Error
	})
}
