package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelTagDefaultsTestDB(t *testing.T) {
	t.Helper()

	originalDB := DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalKeyCol := commonKeyCol

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Model{}, &Option{}))

	DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	commonKeyCol = "`key`"

	t.Cleanup(func() {
		DB = originalDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		commonKeyCol = originalKeyCol
	})
}

func TestSeedLegacyModelTagsBackfillsOnlyEmptyTags(t *testing.T) {
	setupModelTagDefaultsTestDB(t)

	models := []Model{
		{ModelName: "deepseek-v4-flash"},
		{ModelName: "deepseek-v4-pro", Tags: "custom"},
		{ModelName: "unrelated-model"},
	}
	for index := range models {
		require.NoError(t, models[index].Insert())
	}

	require.NoError(t, SeedLegacyModelTags())

	var got []Model
	require.NoError(t, DB.Order("model_name").Find(&got).Error)
	require.Len(t, got, 3)
	// DeepSeek V4 Flash was a retired free promotion. An empty tag must stay
	// empty so removing the badge in the console is not undone on restart.
	require.Empty(t, got[0].Tags)
	require.Equal(t, "custom", got[1].Tags)
	require.Empty(t, got[2].Tags)
}

func TestSeedLegacyModelTagsIsIdempotent(t *testing.T) {
	setupModelTagDefaultsTestDB(t)
	require.NoError(t, (&Model{ModelName: "gpt-5.6-sol"}).Insert())

	require.NoError(t, SeedLegacyModelTags())
	require.NoError(t, DB.Model(&Model{}).Where("model_name = ?", "gpt-5.6-sol").Update("tags", "administrator-edit").Error)
	require.NoError(t, SeedLegacyModelTags())

	var item Model
	require.NoError(t, DB.Where("model_name = ?", "gpt-5.6-sol").First(&item).Error)
	require.Equal(t, "administrator-edit", item.Tags)

	var marker Option
	require.NoError(t, DB.Where("key = ?", modelTagDefaultsSeedKey).First(&marker).Error)
	require.Equal(t, "1", marker.Value)
}

func TestSeedLegacyModelTagsRepairsCatalogueAfterPreviousMarker(t *testing.T) {
	setupModelTagDefaultsTestDB(t)
	require.NoError(t, DB.Create(&Option{Key: "model_tag_defaults_seeded_v1", Value: "1"}).Error)
	require.NoError(t, (&Model{ModelName: "claude-fable-5.1"}).Insert())

	require.NoError(t, SeedLegacyModelTags())

	var item Model
	require.NoError(t, DB.Where("model_name = ?", "claude-fable-5.1").First(&item).Error)
	require.Equal(t, "New release", item.Tags)
}
