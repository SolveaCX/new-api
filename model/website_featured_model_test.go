package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplaceWebsiteFeaturedModelsStoresContinuousOrder(t *testing.T) {
	db, _ := setupModelAccessDB(t)
	require.NoError(t, db.AutoMigrate(&WebsiteFeaturedModel{}))

	require.NoError(t, ReplaceWebsiteFeaturedModels([]string{"gpt-5.5", "claude-opus-4.7"}))
	rows, err := ListWebsiteFeaturedModels()
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-5.5", "claude-opus-4.7"}, websiteFeaturedNames(rows))
	require.Equal(t, []int{0, 1}, websiteFeaturedOrders(rows))
}

func TestReplaceWebsiteFeaturedModelsEmptyClearsRows(t *testing.T) {
	db, _ := setupModelAccessDB(t)
	require.NoError(t, db.AutoMigrate(&WebsiteFeaturedModel{}))

	require.NoError(t, ReplaceWebsiteFeaturedModels([]string{"gpt-5.5"}))
	require.NoError(t, ReplaceWebsiteFeaturedModels(nil))
	rows, err := ListWebsiteFeaturedModels()
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestReplaceWebsiteFeaturedModelsWithConfigStoresBannerFields(t *testing.T) {
	db, _ := setupModelAccessDB(t)
	require.NoError(t, db.AutoMigrate(&WebsiteFeaturedModel{}))
	require.NoError(t, ReplaceWebsiteFeaturedModelsWithConfig([]WebsiteFeaturedModelInput{{
		ModelName:               "gpt-5.5",
		DisplayName:             "GPT launch",
		Description:             "Banner copy",
		Tags:                    "Coding, Agents",
		BackgroundImageURL:      "https://cdn.example/banner.png",
		BackgroundImage:         "data:image/png;base64,AA==",
		FallbackBackgroundImage: "/assets/fallback.png",
		Video:                   "/assets/banner.mp4",
	}}))
	rows, err := ListWebsiteFeaturedModels()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "GPT launch", rows[0].DisplayName)
	require.Equal(t, "data:image/png;base64,AA==", rows[0].BackgroundImage)
	require.Equal(t, "/assets/banner.mp4", rows[0].Video)
}

func TestSeedLegacyWebsiteFeaturedModelsIsOneTimeAndPreservesExplicitClear(t *testing.T) {
	db, _ := setupModelAccessDB(t)
	previousKeyCol := commonKeyCol
	t.Cleanup(func() { commonKeyCol = previousKeyCol })
	commonKeyCol = "`key`"
	require.NoError(t, db.AutoMigrate(&Option{}, &WebsiteFeaturedModel{}))

	require.NoError(t, SeedLegacyWebsiteFeaturedModels())
	rows, err := ListWebsiteFeaturedModels()
	require.NoError(t, err)
	require.Len(t, rows, len(legacyWebsiteFeaturedDefaults))
	require.Equal(t, legacyWebsiteFeaturedDefaults[0].ModelName, rows[0].ModelName)

	require.NoError(t, ReplaceWebsiteFeaturedModels(nil))
	require.NoError(t, SeedLegacyWebsiteFeaturedModels())
	rows, err = ListWebsiteFeaturedModels()
	require.NoError(t, err)
	require.Empty(t, rows)
}

func websiteFeaturedNames(rows []WebsiteFeaturedModel) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.ModelName)
	}
	return names
}

func websiteFeaturedOrders(rows []WebsiteFeaturedModel) []int {
	orders := make([]int, 0, len(rows))
	for _, row := range rows {
		orders = append(orders, row.SortOrder)
	}
	return orders
}
