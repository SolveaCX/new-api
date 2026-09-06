package model

import (
	"regexp"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Bump the marker after the first deployment could run before the model
// catalogue was fully populated. The v2 pass backfills those existing rows.
const modelTagDefaultsSeedKey = "model_tag_defaults_seeded_v2"

type modelTagDefault struct {
	pattern *regexp.Regexp
	tags    string
}

// These are the labels that were historically derived in the website from a
// model-name allowlist. Seed them into model metadata once so administrators
// can edit the same values from the console going forward.
var legacyModelTagDefaults = []modelTagDefault{
	{regexp.MustCompile(`(?i)(^|[/_.-])ling[-_.]?3\.0[-_.]?flash[-_.]?fin`), "Free"},
	{regexp.MustCompile(`(?i)(^|/)glm[-_.]?5[-_.]?3(?:[-_.]?flash)?$`), "Limited discount, New release"},
	{regexp.MustCompile(`(?i)(^|/)deepseek[-_.]?v4[-_.]?pro$`), "Limited discount"},
	{regexp.MustCompile(`(?i)(^|[/_-])seedance[-_.]?2[-_.]?5(?:[-_.]|$)`), "HOT"},
	{regexp.MustCompile(`(?i)(^|/)kimi[-_.]?k3(?:[-_.]|$)`), "HOT"},
	{regexp.MustCompile(`(?i)(^|/)gpt[-_.]?5[-_.]?6[-_.]?sol(?:[-_.]|$)`), "HOT"},
	{regexp.MustCompile(`(?i)(^|/)claude[-_.]?(?:opus[-_.]?(?:4[-_.]?7|4[-_.]?8|5)|sonnet[-_.]?(?:4[-_.]?6|5)|haiku[-_.]?4[-_.]?5(?:[-_.]?20251001)?)(?:[-_.]|$)`), "HOT"},
	{regexp.MustCompile(`(?i)(^|/)claude[-_.]?fable[-_.]?5[-_.]?1(?:[-_.]|$)`), "New release"},
}

// SeedLegacyModelTags migrates the website's former name-based promotion
// labels into Model.Tags. It is idempotent across nodes and never overwrites
// non-empty tags, including an intentional administrator edit.
func SeedLegacyModelTags() error {
	if DB == nil {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var models []Model
		if err := tx.Where("tags IS NULL OR tags = ?", "").Find(&models).Error; err != nil {
			return err
		}
		for _, item := range models {
			for _, rule := range legacyModelTagDefaults {
				if !rule.pattern.MatchString(strings.TrimSpace(item.ModelName)) {
					continue
				}
				if err := tx.Model(&Model{}).Where("id = ? AND (tags IS NULL OR tags = ?)", item.Id, "").Update("tags", rule.tags).Error; err != nil {
					return err
				}
				break
			}
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&Option{Key: modelTagDefaultsSeedKey, Value: "1"}).Error
	})
}
