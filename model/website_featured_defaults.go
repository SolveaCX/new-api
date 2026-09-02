package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const websiteFeaturedDefaultsSeedKey = "website_featured_defaults_seeded_v1"

var legacyWebsiteFeaturedDefaults = []WebsiteFeaturedModelInput{
	{ModelName: "claude-fable-5.1", DisplayName: "Fable 5.1", Description: "Anthropic's strongest coding model yet: 55.8% on Terminal-Bench 4.0, scientific research more than doubled, and up to 45% cheaper on long agentic runs.", Tags: "Coding, Agents, Computer Use", BackgroundImageURL: "/assets/models-featured/claude-fable-5.1.png"},
	{ModelName: "glm-5.3-flash", DisplayName: "GLM-5.3 Flash", Description: "Ox Alpha, unmasked. GLM-5.3 Flash ran 62 trillion tokens anonymously before Zhipu claimed it - 1M-token multimodal context, open MIT weights, Chinese silicon.", Tags: "Coding, Multimodal, Long Context", BackgroundImageURL: "https://cdn.shulex-voc.com/flatkey/models-featured/glm-5.3-flash.png", FallbackBackgroundImage: "/assets/models-featured/glm-5.3-flash.png"},
	{ModelName: "deepseek-v4-pro", DisplayName: "DeepSeek V4 Pro", Description: "A frontier reasoning model with a 1M-token window, built for multi-step analysis, repository-scale code review and research workflows — at a fraction of the cost of comparable frontier models.", Tags: "Chat, Reasoning, Long Context", BackgroundImageURL: "/assets/models-featured/deepseek.jpg"},
	{ModelName: "kimi-k3", DisplayName: "Kimi K3", Description: "Tuned for long-horizon agent work. It holds a full 1M-token context, chains tool calls without losing the thread, and keeps an entire codebase in view across a session.", Tags: "Chat, Agents, Long Context", BackgroundImageURL: "/assets/models-featured/moonshot.jpg"},
	{ModelName: "MiniMax-H3", DisplayName: "MiniMax H3", Description: "An omni-modal generation model that understands text, images, video and audio in one context, producing up to 15 seconds of 2K video with native stereo sound generated alongside the picture.", Tags: "Text to Video, Image to Video, Native Audio", BackgroundImageURL: "/assets/models-featured/minimax.jpg", Video: "/assets/models-featured/minimax.mp4"},
	{ModelName: "seedance-2.5", DisplayName: "Seedance 2.5", Description: "Raises the bar for controllable video: longer takes, multimodal references, precise shot editing and far stronger consistency across cuts for production-ready creative work.", Tags: "Image to Video, Text to Video, Video to Video", BackgroundImageURL: "/assets/models-featured/bytedance.jpg", Video: "/assets/models-featured/bytedance.mp4"},
	{ModelName: "gpt-5.6-sol", DisplayName: "GPT-5.6 Sol", Description: "The dependable workhorse of the GPT line — strong general reasoning, reliable structured output and first-class tool calling, priced so you can put it on the hot path of a production app.", Tags: "Chat, Reasoning, Tool Use", BackgroundImageURL: "/assets/models-featured/gpt-5.6-sol.png"},
	{ModelName: "glm-5.3", DisplayName: "GLM-5.3", Description: "The model teams are switching to for agentic coding: sharp instruction following, dependable function calling and open weights, at a price that makes long autonomous runs actually affordable.", Tags: "Chat, Coding, Agents", BackgroundImageURL: "/assets/models-featured/zhipu.jpg"},
}

// SeedLegacyWebsiteFeaturedModels imports the old bundled banner slate once.
// The marker is separate so an intentional admin clear is preserved.
func SeedLegacyWebsiteFeaturedModels() error {
	if DB == nil {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var marker Option
		err := tx.Where(commonKeyCol+" = ?", websiteFeaturedDefaultsSeedKey).First(&marker).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var count int64
		if err := tx.Model(&WebsiteFeaturedModel{}).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			now := common.GetTimestamp()
			rows := make([]WebsiteFeaturedModel, len(legacyWebsiteFeaturedDefaults))
			for index, item := range legacyWebsiteFeaturedDefaults {
				rows[index] = WebsiteFeaturedModel{ModelName: item.ModelName, SortOrder: index, DisplayName: item.DisplayName, Description: item.Description, Tags: item.Tags, BackgroundImageURL: item.BackgroundImageURL, BackgroundImage: item.BackgroundImage, FallbackBackgroundImage: item.FallbackBackgroundImage, Video: item.Video, CreatedAt: now, UpdatedAt: now}
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
				return err
			}
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&Option{Key: websiteFeaturedDefaultsSeedKey, Value: "1"}).Error
	})
}
